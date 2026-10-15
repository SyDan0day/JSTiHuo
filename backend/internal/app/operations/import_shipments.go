package operations

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"jstihuo/internal/app/identity"
	"jstihuo/internal/app/order"
	imports "jstihuo/internal/domain/imports"
)

type importedShipment struct {
	Row       int
	OrderID   string
	OrderNo   string
	CarrierID string
	Tracking  string
	Version   int64
	Hash      string
}

// shipmentRow 用途：预检一行运单的字段、订单版本和承运商约束。
// 参数：ctx 上下文；row CSV 行；restrictedCarrier 可选限定承运商 ID。
// 返回值：规范化发货命令、可选脱敏错误或数据库错误。
func (s *Service) shipmentRow(ctx context.Context, row imports.Row, restrictedCarrier string) (importedShipment, *RowError, error) {
	in := importedShipment{Row: row.Number, OrderNo: strings.ToUpper(strings.TrimSpace(row.Values[0])), Tracking: strings.TrimSpace(row.Values[2])}
	bad := func(code, field string) (importedShipment, *RowError, error) {
		return in, &RowError{Row: row.Number, Code: code, Field: field}, nil
	}
	version, err := strconv.ParseInt(row.Values[3], 10, 64)
	if err != nil || version < 1 || version > 9007199254740991 || strconv.FormatInt(version, 10) != row.Values[3] {
		return bad("INVALID_VERSION", "expected_version")
	}
	in.Version = version
	if !regexp.MustCompile(`^[A-Za-z0-9-]{3,80}$`).MatchString(in.Tracking) {
		return bad("INVALID_TRACKING", "tracking_no")
	}
	var state, mode string
	var storedVersion int64
	err = s.DB.QueryRowContext(ctx, "SELECT id,version,status,fulfillment_mode FROM orders WHERE order_no=? AND deleted_at IS NULL", in.OrderNo).Scan(&in.OrderID, &storedVersion, &state, &mode)
	if err == sql.ErrNoRows {
		return bad("ORDER_NOT_FOUND", "order_no")
	}
	if err != nil {
		return in, nil, err
	}
	if storedVersion != version {
		return bad("VERSION_CONFLICT", "expected_version")
	}
	if state != "PENDING" || mode != "DELIVERY" {
		return bad("ILLEGAL_TRANSITION", "order_no")
	}
	err = s.DB.QueryRowContext(ctx, "SELECT id FROM carriers WHERE code=? AND enabled=1", strings.ToUpper(strings.TrimSpace(row.Values[1]))).Scan(&in.CarrierID)
	if err == sql.ErrNoRows {
		return bad("CARRIER_NOT_FOUND", "carrier_code")
	}
	if err != nil {
		return in, nil, err
	}
	if restrictedCarrier != "" && restrictedCarrier != in.CarrierID {
		return bad("CARRIER_MISMATCH", "carrier_code")
	}
	var count int
	if err = s.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM shipments WHERE carrier_id=? AND tracking_no=?", in.CarrierID, in.Tracking).Scan(&count); err != nil {
		return in, nil, err
	}
	if count != 0 {
		return bad("TRACKING_CONFLICT", "tracking_no")
	}
	hash := sha256.Sum256([]byte(in.OrderNo + "\x00" + in.CarrierID + "\x00" + in.Tracking + "\x00" + strconv.FormatInt(in.Version, 10)))
	in.Hash = hex.EncodeToString(hash[:])[:20]
	return in, nil, nil
}

// processShipmentImport 用途：预检运单或按订单 ID 升序逐行执行真实发货。
// 参数：ctx 上下文；claim 领取凭证；actor 来源管理员；rows CSV 逻辑行。
// 返回值：保留每行失败的任务结果或数据库、租约错误。
// 约束：行幂等键由任务、行号和规范化摘要固定，恢复任务不重复发货。
func (s *Service) processShipmentImport(ctx context.Context, claim JobClaim, actor identity.Actor, rows []imports.Row) error {
	items, errs := []importedShipment{}, []RowError{}
	orders, tracking := map[string]bool{}, map[string]bool{}
	for _, row := range rows {
		if err := s.RenewJob(ctx, claim); err != nil {
			return err
		}
		// APPLY 恢复先重放已成功的行，不能让旧版本预检误报为重复失败。
		if claim.job.Phase == "APPLY" {
			if item, ok, err := s.replayedShipmentRow(ctx, claim, row); err != nil {
				return err
			} else if ok {
				items = append(items, item)
				continue
			}
		}
		item, invalid, err := s.shipmentRow(ctx, row, claim.job.Options.CarrierID)
		if err != nil {
			return err
		}
		if invalid != nil {
			errs = append(errs, *invalid)
			continue
		}
		key := item.CarrierID + ":" + item.Tracking
		if orders[item.OrderID] || tracking[key] {
			errs = append(errs, RowError{Row: row.Number, Code: "DUPLICATE_SHIPMENT", Field: "row"})
			continue
		}
		orders[item.OrderID], tracking[key] = true, true
		items = append(items, item)
	}
	if claim.job.Phase == "PREVIEW" {
		return s.finishImport(ctx, claim, jobSummary{Processed: len(rows), Failed: len(errs), Errors: errs})
	}
	sort.Slice(items, func(i, j int) bool {
		if len(items[i].OrderID) != len(items[j].OrderID) {
			return len(items[i].OrderID) < len(items[j].OrderID)
		}
		return items[i].OrderID < items[j].OrderID
	})
	for _, item := range items {
		if err := s.RenewJob(ctx, claim); err != nil {
			return err
		}
		input := order.ShipInput{Version: item.Version, CarrierID: item.CarrierID, Tracking: item.Tracking}
		_, err := s.Orders.Ship(ctx, actor, item.OrderID, input, shipmentRowKey(claim.ID, item), "import-"+claim.ID)
		if err != nil {
			var f *identity.Fault
			if !errors.As(err, &f) || f.Status >= 500 {
				return err
			}
			errs = append(errs, RowError{Row: item.Row, Code: f.Code, Field: "row"})
		}
	}
	return s.finishImport(ctx, claim, jobSummary{Processed: len(rows), Failed: len(errs), Errors: errs})
}

// shipmentRowKey 用途：生成逐行发货的稳定幂等键。
// 参数：jobID 任务 ID；item 规范化运单行。
// 返回值：不包含订单号或收件信息的固定键。
func shipmentRowKey(jobID string, item importedShipment) string {
	return fmt.Sprintf("job-%s-row-%d-%s", jobID, item.Row, item.Hash)
}

// replayedShipmentRow 用途：恢复已成功行，绕过已发状态导致的旧版本预检失败。
// 参数：ctx 上下文；claim 当前任务；row 原始 CSV 行。
// 返回值：可重放命令、是否已经发货或数据库错误。
// 约束：必须核对主运单与原输入一致；真正业务重放仍由 Ship 的摘要验证执行。
func (s *Service) replayedShipmentRow(ctx context.Context, claim JobClaim, row imports.Row) (importedShipment, bool, error) {
	var in importedShipment
	var version int64
	version, err := strconv.ParseInt(row.Values[3], 10, 64)
	if err != nil {
		return in, false, nil
	}
	err = s.DB.QueryRowContext(ctx, `SELECT o.id,s.carrier_id FROM orders o
		JOIN shipments s ON s.order_id=o.id AND s.kind='PRIMARY'
		JOIN carriers c ON c.id=s.carrier_id WHERE o.order_no=? AND c.code=? AND s.tracking_no=?`,
		strings.ToUpper(strings.TrimSpace(row.Values[0])), strings.ToUpper(strings.TrimSpace(row.Values[1])), strings.TrimSpace(row.Values[2])).Scan(&in.OrderID, &in.CarrierID)
	if err == sql.ErrNoRows {
		return in, false, nil
	}
	if err != nil {
		return in, false, err
	}
	in.Row, in.Version, in.OrderNo, in.Tracking = row.Number, version, strings.ToUpper(strings.TrimSpace(row.Values[0])), strings.TrimSpace(row.Values[2])
	hash := sha256.Sum256([]byte(in.OrderNo + "\x00" + in.CarrierID + "\x00" + in.Tracking + "\x00" + strconv.FormatInt(in.Version, 10)))
	in.Hash = hex.EncodeToString(hash[:])[:20]
	var count int
	scope := fmt.Sprintf("admin:%d:shipOrder", claim.job.Options.Owner)
	err = s.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM idempotency_keys WHERE scope=? AND request_key=? AND expires_at>?", scope, shipmentRowKey(claim.ID, in), s.Now().UTC()).Scan(&count)
	return in, count == 1, err
}
