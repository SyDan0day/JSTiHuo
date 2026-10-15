package operations

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"jstihuo/internal/app/identity"
	"jstihuo/internal/domain/calendar"
	rules "jstihuo/internal/domain/voucher"
)

type Config struct {
	Version int64 `json:"version"`
	calendar.Config
	NotificationMode string `json:"notification_mode"`
	CarrierID        string `json:"default_carrier_id,omitempty"`
}

// loadConfig 用途：读取固定系统配置，缺省值只供读取，不覆盖已保存规则。
// 参数：ctx 上下文；q 数据源。
// 返回值：当前配置或数据库、配置不可用错误，不包含供应商秘密。
func loadConfig(ctx context.Context, q queryer) (Config, error) {
	out := Config{Version: 1, Config: calendar.Default(), NotificationMode: "DISABLED"}
	var raw []byte
	var version int64
	err := q.QueryRowContext(ctx, "SELECT version,value FROM system_configs WHERE config_key='shipping_calendar'").Scan(&version, &raw)
	if err == sql.ErrNoRows {
		return out, nil
	}
	if err != nil {
		return out, err
	}
	if err = json.Unmarshal(raw, &out); err != nil {
		return out, err
	}
	out.Version = version
	if !out.Config.Valid() {
		return out, fault(503, "CONFIGURATION_UNAVAILABLE")
	}
	return out, nil
}

// GetConfig 用途：读取当前固定参数。
// 参数：ctx 上下文。
// 返回值：日历、通知模式与默认承运商或读取错误。
func (s *Service) GetConfig(ctx context.Context) (Config, error) { return loadConfig(ctx, s.DB) }

// UpdateConfig 用途：版本化更新固定参数并原子调整未来日容量与黑名单。
// 参数：ctx 上下文；a 管理员；in 完整配置；key 幂等键；rid 请求标识。
// 返回值：配置 JSON 或字段、容量、版本、集成不可用错误。
// 事务：先锁配置，再按日期锁未来三十一天；不改变已有订单日期和占用数量。
func (s *Service) UpdateConfig(ctx context.Context, a identity.Actor, in Config, key, rid string) (json.RawMessage, error) {
	if in.Version < 1 || in.Version > 9007199254740991 || in.Blackouts == nil || !in.Config.Valid() || in.CarrierID != "" && !rules.ValidID(in.CarrierID) {
		return nil, fault(422, "VALIDATION_FAILED")
	}
	if in.NotificationMode != "DISABLED" && in.NotificationMode != "MOCK" {
		if in.NotificationMode == "LIVE" {
			return nil, fault(503, "INTEGRATION_DISABLED")
		}
		return nil, fault(422, "VALIDATION_FAILED")
	}
	command := mutation{Operation: "updateConfig", Permission: "config.write", Entity: "config", Key: key, RequestID: rid, Body: in}
	return s.write(ctx, a, command, func(tx *sql.Tx) (any, string, error) {
		defaults, _ := json.Marshal(Config{Version: 1, Config: calendar.Default(), NotificationMode: "DISABLED"})
		if _, err := tx.ExecContext(ctx, "INSERT INTO system_configs(config_key,value) VALUES('shipping_calendar',?) ON DUPLICATE KEY UPDATE config_key=config_key", defaults); err != nil {
			return nil, "", err
		}
		var id string
		var version int64
		if err := tx.QueryRowContext(ctx, "SELECT id,version FROM system_configs WHERE config_key='shipping_calendar' FOR UPDATE").Scan(&id, &version); err != nil {
			return nil, "", err
		}
		if version != in.Version {
			return nil, "", fault(409, "VERSION_CONFLICT")
		}
		if in.CarrierID != "" {
			var enabled bool
			err := tx.QueryRowContext(ctx, "SELECT enabled FROM carriers WHERE id=? FOR SHARE", in.CarrierID).Scan(&enabled)
			if err == sql.ErrNoRows || err == nil && !enabled {
				return nil, "", fault(422, "VALIDATION_FAILED")
			}
			if err != nil {
				return nil, "", err
			}
		}
		if err := s.configureCalendar(ctx, tx, in.Config); err != nil {
			return nil, "", err
		}
		in.Version++
		raw, err := json.Marshal(in)
		if err != nil {
			return nil, "", err
		}
		if _, err = tx.ExecContext(ctx, "UPDATE system_configs SET value=?,updated_by=?,version=version+1 WHERE id=?", raw, a.AdminID, id); err != nil {
			return nil, "", err
		}
		return in, id, nil
	})
}

// configureCalendar 用途：按日期调整未来三十一天容量，已有占用不能被压低或释放。
// 参数：ctx 上下文；tx 已锁配置的事务；config 新日历规则。
// 返回值：nil 或 CAPACITY_EXCEEDED、数据库错误。
func (s *Service) configureCalendar(ctx context.Context, tx *sql.Tx, config calendar.Config) error {
	blackouts := map[string]bool{}
	for _, date := range config.Blackouts {
		blackouts[date] = true
	}
	today := calendar.Day(s.Now())
	for day := 0; day <= 30; day++ {
		date := today.AddDate(0, 0, day).Format("2006-01-02")
		if _, err := tx.ExecContext(ctx, "INSERT INTO shipping_calendars(ship_date,capacity) VALUES(?,?) ON DUPLICATE KEY UPDATE ship_date=ship_date", date, config.Capacity); err != nil {
			return err
		}
		var used int
		if err := tx.QueryRowContext(ctx, "SELECT used_slots FROM shipping_calendars WHERE ship_date=? FOR UPDATE", date).Scan(&used); err != nil {
			return err
		}
		if used > config.Capacity {
			return fault(409, "CAPACITY_EXCEEDED")
		}
		if _, err := tx.ExecContext(ctx, "UPDATE shipping_calendars SET capacity=?,closed=?,version=version+1 WHERE ship_date=?", config.Capacity, blackouts[date], date); err != nil {
			return err
		}
		if err := s.inject(ctx, tx, fmt.Sprintf("calendar:%d", day)); err != nil {
			return err
		}
	}
	return nil
}
