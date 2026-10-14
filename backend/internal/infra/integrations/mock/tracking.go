package mock

import (
	"context"
	"database/sql"
	"time"

	"jstihuo/internal/app/order"
)

type Tracking struct {
	DB      *sql.DB
	Now     func() time.Time
	Publish func(context.Context, order.TrackingInput) error
}

// RunOnce 用途：推进最多二十个 MOCK 运单的本地模拟轨迹。
// 参数：ctx worker 上下文。
// 返回值：本轮是否处理事件及数据库、发布错误。
// 约束：只处理显式 MOCK 承运商，三十秒运输、六十秒模拟签收，不调用真实供应商。
func (s Tracking) RunOnce(ctx context.Context) (bool, error) {
	rows, err := s.DB.QueryContext(ctx,
		"SELECT s.id,s.status,s.shipped_at FROM shipments s JOIN carriers c ON c.id=s.carrier_id WHERE c.provider='MOCK' AND s.status IN ('SHIPPED','IN_TRANSIT') AND s.shipped_at<=? ORDER BY s.id LIMIT 20",
		s.Now().UTC().Add(-30*time.Second),
	)
	if err != nil {
		return false, err
	}
	events := []order.TrackingInput{}
	for rows.Next() {
		var id, status string
		var shipped time.Time
		if err = rows.Scan(&id, &status, &shipped); err != nil {
			rows.Close()
			return false, err
		}
		typeName, delay, description := "IN_TRANSIT", 30*time.Second, "MOCK 模拟运输"
		if !s.Now().UTC().Before(shipped.Add(time.Minute)) {
			typeName, delay, description = "DELIVERED", time.Minute, "MOCK 模拟签收"
		}
		events = append(events, order.TrackingInput{ShipmentID: id, ProviderID: "mock:" + id + ":" + typeName, Type: typeName, Description: description, Occurred: shipped.Add(delay)})
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return false, err
	}
	for _, event := range events {
		if err = s.Publish(ctx, event); err != nil {
			return len(events) > 0, err
		}
	}
	return len(events) > 0, nil
}
