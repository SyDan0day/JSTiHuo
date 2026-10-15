package operations

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"jstihuo/internal/app/order"
	"jstihuo/internal/infra/integrations"
)

// ScheduleTracking 用途：为已配置跟踪适配器的未签收运单建立六小时去重任务。
// 参数：ctx worker 上下文。
// 返回值：nil 或数据库、事件错误；超过三十天只标记人工异常，不自动完成。
// 约束：MANUAL 不伪造轨迹，MOCK 由显式模拟调度处理。
// 并发：单服务串行推进游标，每轮最多一千件，完成一轮后从头扫描。
func (s *Service) ScheduleTracking(ctx context.Context) error {
	s.trackingMu.Lock()
	defer s.trackingMu.Unlock()
	after := s.trackingAfter
	if after == "" {
		after = "0"
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT s.id,c.provider,s.shipped_at FROM shipments s JOIN carriers c ON c.id=s.carrier_id
		WHERE s.status IN ('SHIPPED','IN_TRANSIT','EXCEPTION') AND c.provider<>'MOCK' AND s.id>? ORDER BY s.id LIMIT 1000`, after)
	if err != nil {
		return err
	}
	type target struct {
		ID, Provider string
		Shipped      time.Time
	}
	targets := []target{}
	for rows.Next() {
		var out target
		if err = rows.Scan(&out.ID, &out.Provider, &out.Shipped); err != nil {
			break
		}
		targets = append(targets, out)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return err
	}
	for _, item := range targets {
		if !s.Now().Before(item.Shipped.Add(30 * 24 * time.Hour)) {
			err = s.Orders.RecordTracking(ctx, order.TrackingInput{ShipmentID: item.ID, ProviderID: "manual-timeout:" + item.ID, Type: "EXCEPTION", Description: "发货已满三十天，请人工核对签收结果", Occurred: item.Shipped.Add(30 * 24 * time.Hour)})
			if err != nil {
				return err
			}
			continue
		}
		if _, ok := s.Carriers[item.Provider].(integrations.Tracker); !ok {
			continue
		}
		window := s.Now().UTC().Unix() / int64((6 * time.Hour).Seconds())
		_, err = s.DB.ExecContext(ctx, `INSERT INTO outbox_events(event_key,event_type,payload,next_attempt_at) VALUES(?,'tracking.poll',JSON_OBJECT('shipment_id',?),?) ON DUPLICATE KEY UPDATE event_key=event_key`, fmt.Sprintf("tracking:%s:%d", item.ID, window), item.ID, s.Now().UTC())
		if err != nil {
			return err
		}
	}
	if len(targets) < 1000 {
		s.trackingAfter = ""
	} else {
		s.trackingAfter = targets[len(targets)-1].ID
	}
	return nil
}

// processTrackingPoll 用途：在事务外拉取已配置供应商轨迹并应用单调状态。
// 参数：ctx 上下文；claim 六小时窗口任务凭证。
// 返回值：nil 或租约、数据库错误，供应商失败有界重试，不伪造签收。
func (s *Service) processTrackingPoll(ctx context.Context, claim OutboxClaim) error {
	var raw []byte
	if err := s.DB.QueryRowContext(ctx, "SELECT payload FROM outbox_events WHERE id=?", claim.ID).Scan(&raw); err != nil {
		return err
	}
	var payload struct {
		ShipmentID string `json:"shipment_id"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return err
	}
	var status, provider, code, tracking string
	err := s.DB.QueryRowContext(ctx, "SELECT s.status,c.provider,c.code,s.tracking_no FROM shipments s JOIN carriers c ON c.id=s.carrier_id WHERE s.id=?", payload.ShipmentID).Scan(&status, &provider, &code, &tracking)
	if err == sql.ErrNoRows || err == nil && status == "DELIVERED" {
		return s.finishOutbox(ctx, claim, "", "", "")
	}
	if err != nil {
		return err
	}
	adapter, ok := s.Carriers[provider].(integrations.Tracker)
	if !ok {
		return s.finishOutbox(ctx, claim, "FAILED", "", "INTEGRATION_DISABLED")
	}
	if claim.Attempts > 5 {
		return s.finishOutbox(ctx, claim, "FAILED", "", "RETRY_EXHAUSTED")
	}
	if err = s.markOutboxCall(ctx, claim); err != nil {
		return err
	}
	work, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	events, err := adapter.Track(work, code, tracking)
	if err != nil {
		return s.finishOutbox(ctx, claim, "FAILED", "", "PROVIDER_UNAVAILABLE")
	}
	if len(events) > 1000 {
		return s.finishOutbox(ctx, claim, "FAILED", "", "INVALID_PROVIDER_RESULT")
	}
	for _, event := range events {
		if err = s.RecordProviderEvent(ctx, provider, ProviderEvent{EventID: event.ID, CarrierCode: code, Tracking: tracking, Type: event.Type, Occurred: event.Occurred}); err != nil {
			return err
		}
	}
	return s.finishOutbox(ctx, claim, "", "", "")
}
