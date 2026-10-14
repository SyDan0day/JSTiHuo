package order

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"unicode/utf8"

	"jstihuo/internal/app/identity"
	"jstihuo/internal/app/redemption"
	rules "jstihuo/internal/domain/voucher"
)

// validReason 用途：校验版本和不可空原因。
// 参数：in 操作版本与原因。
// 返回值：合法时 true。
func validReason(in VersionReason) bool {
	validVersion := in.Version >= 1 && in.Version <= 9007199254740991
	hasReason := strings.TrimSpace(in.Reason) != ""
	return validVersion && hasReason && utf8.RuneCountInString(in.Reason) <= 200
}

// checkPending 用途：检查订单版本与待发配送状态。
// 参数：locked 已锁定订单快照；version 期望版本。
// 返回值：nil 或版本、状态错误。
// 约束：不改变资源，不要求新兑换商品仍启用。
func checkPending(locked Locked, version int64) error {
	if locked.Version != version {
		return fault(409, "VERSION_CONFLICT")
	}
	if locked.Status != "PENDING" || locked.Mode != "DELIVERY" || locked.Deleted {
		return fault(409, "ILLEGAL_TRANSITION")
	}
	return nil
}

// Cancel 用途：原子取消待发配送订单并释放资源。
// 参数：ctx 请求上下文；a 已认证操作主体。
// 参数：id 目标对象标识；in 操作版本与原因。
// 参数：key 当前动作的幂等键；rid 审计请求标识。
// 返回值：订单或权限、版本、任务、资源及 SQL 错误。
// 事务：预占、流水、日额度和取消状态一次提交。
// 约束：只释放一次资源，不返还已消费卡券。
func (s *Service) Cancel(
	ctx context.Context, a identity.Actor, id string, in VersionReason, key, rid string,
) (redemption.Order, error) {
	if !rules.ValidID(id) || !validReason(in) {
		return redemption.Order{}, fault(422, "VALIDATION_FAILED")
	}
	err := s.mutate(ctx, a, "orderCancel", "order.cancel", id, in, key, rid, func(tx *sql.Tx) error {
		return s.cancelTx(ctx, tx, id, in)
	})
	if err != nil {
		return redemption.Order{}, err
	}
	return s.Redemption.GetOrder(ctx, id, true)
}

// cancelTx 用途：协调取消的资源锁定、释放和状态发布。
// 参数：ctx 请求上下文；tx 当前业务事务。
// 参数：id 目标对象标识；in 操作版本与原因。
// 返回值：nil 或状态、未决任务、资源、SQL 错误。
// 约束：调用方提交，所有失败整体回滚。
func (s *Service) cancelTx(ctx context.Context, tx *sql.Tx, id string, in VersionReason) error {
	locked, err := s.lock(ctx, tx, id)
	if err != nil {
		return err
	}
	if err = checkPending(locked, in.Version); err != nil {
		return err
	}
	if err = checkCancelableResources(ctx, tx, locked); err != nil {
		return err
	}
	items, err := orderItems(ctx, tx, id)
	if err != nil {
		return err
	}
	if err = lockReservedStock(ctx, tx, items); err != nil {
		return err
	}
	for index, item := range items {
		if err = releaseItem(ctx, tx, id, item, in.Reason); err != nil {
			return err
		}
		if err = s.inject(ctx, tx, fmt.Sprintf("release:%d", index+1)); err != nil {
			return err
		}
	}
	_, err = tx.ExecContext(ctx,
		"UPDATE shipping_calendars SET used_slots=used_slots-1,version=version+1 WHERE id=?", locked.CalendarID,
	)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx,
		"UPDATE orders SET status='CANCELLED',cancelled_at=?,cancel_reason=?,version=version+1 WHERE id=?",
		s.Redemption.Now().UTC(), in.Reason, id,
	)
	if err != nil {
		return err
	}
	return s.event(ctx, tx, "order.cancelled", id, locked.Version+1)
}

// checkCancelableResources 用途：拒绝未决运单作业并锁定当前预约日。
// 参数：ctx 请求上下文；tx 当前业务事务。
// 参数：locked 已锁定订单快照。
// 返回值：nil 或任务、额度一致性、SQL 错误。
// 约束：不释放任何资源，后续取消保持此日历锁。
func checkCancelableResources(ctx context.Context, tx *sql.Tx, locked Locked) error {
	var jobs int
	err := tx.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM integration_jobs WHERE order_id=? AND kind='LABEL' AND state IN ('PENDING','RUNNING','READY','UNKNOWN')",
		locked.ID,
	).Scan(&jobs)
	if err != nil {
		return err
	}
	if jobs > 0 {
		return fault(409, "LABEL_JOB_PENDING")
	}
	var slots int
	err = tx.QueryRowContext(ctx,
		"SELECT used_slots FROM shipping_calendars WHERE id=? FOR UPDATE", locked.CalendarID,
	).Scan(&slots)
	if err != nil {
		return err
	}
	if slots < 1 {
		return fault(409, "CONSISTENCY_CONFLICT")
	}
	return nil
}

// Visibility 用途：把终态订单移入回收站或恢复显示。
// 参数：ctx 请求上下文；a 已认证操作主体。
// 参数：id 目标对象标识；in 操作版本与原因。
// 参数：restore 是否恢复显示；key 当前动作的幂等键。
// 参数：rid 审计请求标识。
// 返回值：订单或权限、版本、状态、SQL 错误。
// 约束：只改显示标记，不改变状态、库存和卡消费。
func (s *Service) Visibility(
	ctx context.Context, a identity.Actor, id string, in VersionReason,
	restore bool, key, rid string,
) (redemption.Order, error) {
	if !rules.ValidID(id) || !validReason(in) {
		return redemption.Order{}, fault(422, "VALIDATION_FAILED")
	}
	operation, permission := "orderTrash", "order.trash"
	if restore {
		operation, permission = "orderRestore", "order.restore"
	}
	err := s.mutate(ctx, a, operation, permission, id, in, key, rid, func(tx *sql.Tx) error {
		locked, err := s.lock(ctx, tx, id)
		if err != nil {
			return err
		}
		if locked.Version != in.Version {
			return fault(409, "VERSION_CONFLICT")
		}
		terminal := locked.Status == "CANCELLED" || locked.Status == "COMPLETED"
		if !terminal || locked.Deleted != restore {
			return fault(409, "ILLEGAL_TRANSITION")
		}
		var deleted any
		if !restore {
			deleted = s.Redemption.Now().UTC()
		}
		_, err = tx.ExecContext(ctx,
			"UPDATE orders SET deleted_at=?,version=version+1 WHERE id=?", deleted, id,
		)
		if err != nil {
			return err
		}
		return s.event(ctx, tx, "order.visibility", id, locked.Version+1)
	})
	if err != nil {
		return redemption.Order{}, err
	}
	return s.Redemption.GetOrder(ctx, id, true)
}
