package order

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"sort"
	"time"
	"unicode/utf8"

	"jstihuo/internal/app/identity"
	"jstihuo/internal/app/redemption"
	"jstihuo/internal/domain/calendar"
	rules "jstihuo/internal/domain/voucher"
)

type preparedAddress struct {
	Cipher      []byte
	KeyVersion  int
	PhoneLookup []byte
}

// Edit 用途：以完整新地址、日期、备注更新待发配送订单。
// 参数：ctx 请求上下文；a 已认证操作主体。
// 参数：id 目标对象标识；in 完整地址、日期、备注及期望版本。
// 参数：key 当前动作的幂等键；rid 审计请求标识。
// 返回值：脱敏订单或权限、截止、版本、容量、禁配及 SQL 错误。
// 事务：按日期顺序锁日历，原子迁移名额和更新地址密文。
// 约束：保留冻结商品与库存预占；截止后或非待发状态禁止编辑。
func (s *Service) Edit(
	ctx context.Context, a identity.Actor, id string, in EditInput, key, rid string,
) (redemption.Order, error) {
	if err := s.validateEdit(id, &in); err != nil {
		return redemption.Order{}, err
	}
	config, err := s.Redemption.Configuration(ctx)
	if err != nil {
		return redemption.Order{}, err
	}
	address, err := s.prepareAddress(ctx, id, in.Address)
	if err != nil {
		return redemption.Order{}, err
	}
	operation, permission := "customerOrderEdit", ""
	if a.Kind == "ADMIN" {
		operation, permission = "adminOrderEdit", "order.write"
	}
	err = s.mutate(ctx, a, operation, permission, id, in, key, rid, func(tx *sql.Tx) error {
		return s.editTx(ctx, tx, id, in, config, address)
	})
	if err != nil {
		return redemption.Order{}, err
	}
	return s.Redemption.GetOrder(ctx, id, a.Kind == "ADMIN")
}

// validateEdit 用途：验证完整订单编辑命令并规范地址。
// 参数：id 目标对象标识；in 完整地址、日期、备注及期望版本。
// 返回值：nil 或字段、地址、日期错误。
// 约束：只修改输入地址，不执行 SQL。
func (s *Service) validateEdit(id string, in *EditInput) error {
	validVersion := in.Version >= 1 && in.Version <= 9007199254740991
	if !rules.ValidID(id) || !validVersion || utf8.RuneCountInString(in.Note) > 200 {
		return fault(422, "VALIDATION_FAILED")
	}
	if err := s.Redemption.ValidateAddress(&in.Address); err != nil {
		return err
	}
	if _, err := time.ParseInLocation("2006-01-02", in.Date, calendar.Shanghai); err != nil {
		return fault(422, "DATE_UNAVAILABLE")
	}
	return nil
}

// prepareAddress 用途：在业务锁之前准备绑定订单号的地址密文和手机 HMAC。
// 参数：ctx 请求上下文；id 目标对象标识。
// 参数：address 已规范化的完整地址。
// 返回值：加密材料或不存在、序列化、加密、SQL 错误。
// 约束：明文缓冲立即清零，不写文件。
func (s *Service) prepareAddress(
	ctx context.Context, id string, address redemption.Address,
) (preparedAddress, error) {
	var number string
	err := s.DB.QueryRowContext(ctx, "SELECT order_no FROM orders WHERE id=?", id).Scan(&number)
	if err == sql.ErrNoRows {
		return preparedAddress{}, fault(404, "NOT_FOUND")
	}
	if err != nil {
		return preparedAddress{}, err
	}
	data := redemption.RecipientData{
		Address: address, Region: redemption.RegionNames(s.Redemption.Regions, address),
	}
	plain, err := json.Marshal(data)
	if err != nil {
		return preparedAddress{}, err
	}
	cipher, version, err := s.Redemption.PII.Encrypt(plain, redemption.OrderAAD(number))
	clear(plain)
	if err != nil {
		return preparedAddress{}, err
	}
	phoneMAC := hmac.New(sha256.New, s.Redemption.LookupKey)
	phoneMAC.Write([]byte(address.Phone))
	return preparedAddress{cipher, version, phoneMAC.Sum(nil)}, nil
}

// editTx 用途：协调已授权订单编辑的同一事务。
// 参数：ctx 请求上下文；tx 当前业务事务。
// 参数：id 目标对象标识；in 完整地址、日期、备注及期望版本。
// 参数：config 日历配置快照；address 已规范化的完整地址。
// 返回值：nil 或状态、日期、库存地区及 SQL 错误。
// 约束：调用方提交，失败不能保留部分名额迁移。
func (s *Service) editTx(
	ctx context.Context, tx *sql.Tx, id string, in EditInput,
	config calendar.Config, address preparedAddress,
) error {
	locked, err := s.lock(ctx, tx, id)
	if err != nil {
		return err
	}
	if err = s.checkEditable(locked, in.Version); err != nil {
		return err
	}
	oldDate := locked.Date.Format("2006-01-02")
	if in.Date != oldDate && !config.Allowed(s.Redemption.Now(), in.Date) {
		return fault(422, "DATE_UNAVAILABLE")
	}
	newCalendar, err := s.lockEditDates(ctx, tx, locked, in.Date, config.Capacity)
	if err != nil {
		return err
	}
	if err = s.validateDestination(ctx, tx, locked, in); err != nil {
		return err
	}
	if err = s.moveSlots(ctx, tx, locked.CalendarID, newCalendar); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx,
		"UPDATE orders SET calendar_id=?,recipient_cipher=?,pii_key_version=?,phone_lookup=?,customer_note=?,version=version+1 WHERE id=?",
		newCalendar, address.Cipher, address.KeyVersion, address.PhoneLookup, in.Note, id,
	)
	if err != nil {
		return err
	}
	return s.event(ctx, tx, "order.updated", id, locked.Version+1)
}

// checkEditable 用途：检查待发状态、版本及上海修改截止时间。
// 参数：locked 已锁定订单快照；version 期望版本。
// 返回值：nil 或版本、状态错误。
// 约束：不改订单，允许当前商品已下架的既有订单继续履约。
func (s *Service) checkEditable(locked Locked, version int64) error {
	if err := checkPending(locked, version); err != nil {
		return err
	}
	date, err := time.ParseInLocation(
		"2006-01-02", locked.Date.Format("2006-01-02"), calendar.Shanghai,
	)
	if err != nil {
		return err
	}
	cutoff := date.AddDate(0, 0, -1).Add(17 * time.Hour)
	if !s.Redemption.Now().Before(cutoff) {
		return fault(409, "ILLEGAL_TRANSITION")
	}
	return nil
}

// lockEditDates 用途：按业务日期升序锁定旧、新预约日。
// 参数：ctx 请求上下文；tx 当前业务事务。
// 参数：locked 已锁定订单快照；newDate 目标业务日期。
// 参数：capacity 缺失日历的默认容量。
// 返回值：目标日历 ID 或日期、容量、SQL 错误。
// 约束：保留原日名额不受新配置禁约影响，不自行占位。
func (s *Service) lockEditDates(
	ctx context.Context, tx *sql.Tx, locked Locked, newDate string, capacity int,
) (string, error) {
	oldDate := locked.Date.Format("2006-01-02")
	dates := []string{oldDate}
	if newDate != oldDate {
		dates = append(dates, newDate)
	}
	sort.Strings(dates)
	target := locked.CalendarID
	for _, date := range dates {
		if date != oldDate {
			_, err := tx.ExecContext(ctx,
				"INSERT INTO shipping_calendars(ship_date,capacity) VALUES(?,?) ON DUPLICATE KEY UPDATE ship_date=ship_date",
				date, capacity,
			)
			if err != nil {
				return "", err
			}
		}
		var id string
		var maximum, used int
		var closed bool
		err := tx.QueryRowContext(ctx,
			"SELECT id,capacity,used_slots,closed FROM shipping_calendars WHERE ship_date=? FOR UPDATE", date,
		).Scan(&id, &maximum, &used, &closed)
		if err != nil {
			return "", err
		}
		if date != newDate {
			continue
		}
		if date != oldDate && closed {
			return "", fault(422, "DATE_UNAVAILABLE")
		}
		if date != oldDate && used >= maximum {
			return "", fault(422, "CAPACITY_EXCEEDED")
		}
		target = id
	}
	return target, nil
}

// validateDestination 用途：校验既有订单的最新禁配地区和新预约期限。
// 参数：ctx 请求上下文；tx 当前业务事务。
// 参数：locked 已锁定订单快照；in 完整地址、日期、备注及期望版本。
// 返回值：nil 或禁配、商品日期、SQL、JSON 错误。
// 约束：按商品升序共享锁，不以当前下架撤销既有履约。
func (s *Service) validateDestination(
	ctx context.Context, tx *sql.Tx, locked Locked, in EditInput,
) error {
	items, err := orderItems(ctx, tx, locked.ID)
	if err != nil {
		return err
	}
	newDay, err := time.ParseInLocation("2006-01-02", in.Date, calendar.Shanghai)
	if err != nil {
		return err
	}
	changedDate := in.Date != locked.Date.Format("2006-01-02")
	for _, item := range items {
		var excluded []byte
		var from, until sql.NullTime
		err = tx.QueryRowContext(ctx,
			"SELECT excluded_region_codes,redeem_from,redeem_until FROM products WHERE id=? FOR SHARE", item.Product,
		).Scan(&excluded, &from, &until)
		if err != nil {
			return err
		}
		var codes []string
		if err = json.Unmarshal(excluded, &codes); err != nil {
			return err
		}
		if redemption.Restricted(codes, in.Address) {
			return fault(422, "REGION_RESTRICTED")
		}
		outsidePeriod := (from.Valid && newDay.Before(from.Time)) ||
			(until.Valid && !newDay.Before(until.Time))
		if changedDate && outsidePeriod {
			return fault(422, "DATE_UNAVAILABLE")
		}
	}
	return nil
}

// moveSlots 用途：原子迁移单个既有预约名额。
// 参数：ctx 请求上下文；tx 当前业务事务。
// 参数：oldID 原日历标识；newID 目标日历标识。
// 返回值：nil 或一致性、容量、SQL、测试故障错误。
// 约束：同一天不改额度，失败由外层整体回滚。
func (s *Service) moveSlots(ctx context.Context, tx *sql.Tx, oldID, newID string) error {
	if oldID == newID {
		return nil
	}
	result, err := tx.ExecContext(ctx,
		"UPDATE shipping_calendars SET used_slots=used_slots-1,version=version+1 WHERE id=? AND used_slots>0", oldID,
	)
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed != 1 {
		return fault(409, "CONSISTENCY_CONFLICT")
	}
	result, err = tx.ExecContext(ctx,
		"UPDATE shipping_calendars SET used_slots=used_slots+1,version=version+1 WHERE id=? AND closed=0 AND used_slots<capacity", newID,
	)
	if err != nil {
		return err
	}
	changed, err = result.RowsAffected()
	if err != nil {
		return err
	}
	if changed != 1 {
		return fault(422, "CAPACITY_EXCEEDED")
	}
	return s.inject(ctx, tx, "calendar")
}

// PrivateView 用途：带原因审计读取完整订单地址。
// 参数：ctx 请求上下文；a 已认证操作主体。
// 参数：id 目标对象标识；in 完整地址、日期、备注及期望版本。
// 参数：key 当前动作的幂等键；rid 审计请求标识。
// 返回值：地址或权限、版本、解密、SQL 错误。
// 约束：每次成功读取都审计，包括重放；幂等记录不保存 PII。
func (s *Service) PrivateView(
	ctx context.Context, a identity.Actor, id string, in VersionReason, key, rid string,
) (Private, error) {
	if !rules.ValidID(id) || !validReason(in) {
		return Private{}, fault(422, "VALIDATION_FAILED")
	}
	out := Private{OrderID: id}
	err := s.mutate(ctx, a, "privateOrder", "order.pii", id, in, key, rid, func(tx *sql.Tx) error {
		locked, err := s.lock(ctx, tx, id)
		if err != nil {
			return err
		}
		if locked.Version != in.Version {
			return fault(409, "VERSION_CONFLICT")
		}
		if locked.Mode != "DELIVERY" {
			return fault(409, "ILLEGAL_TRANSITION")
		}
		plain, err := s.Redemption.PII.Decrypt(
			locked.Cipher, redemption.OrderAAD(locked.Number), locked.KeyVersion,
		)
		if err != nil {
			return err
		}
		defer clear(plain)
		var data redemption.RecipientData
		if err = json.Unmarshal(plain, &data); err != nil {
			return err
		}
		out.Address = data.Address
		return nil
	})
	if err != nil {
		return Private{}, err
	}
	return out, nil
}
