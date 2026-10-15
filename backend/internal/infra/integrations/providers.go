package integrations

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"
)

var ErrUnknown = errors.New("RESULT_UNKNOWN")
var ErrTemporary = errors.New("PROVIDER_UNAVAILABLE")
var ErrRejected = errors.New("PROVIDER_REJECTED")

type LabelRequest struct {
	OrderID, CarrierID string
	Version            int64
}
type Label struct{ CarrierID, Tracking string }
type Notice struct{ Template, Recipient, OrderID string }

type TrackingEvent struct {
	ID, Type string
	Occurred time.Time
}
type Tracker interface {
	Track(context.Context, string, string) ([]TrackingEvent, error)
}

type CarrierProvider interface {
	CreateLabel(context.Context, string, LabelRequest) (Label, error)
	GetLabel(context.Context, string) (Label, bool, error)
	Idempotent() bool
}
type Notifier interface {
	Send(context.Context, string, Notice) (string, error)
	Lookup(context.Context, string) (string, bool, error)
	Idempotent() bool
}

// Token 用途：从稳定请求键生成不含客户信息的模拟结果标识。
// 参数：key 已持久化请求键。
// 返回值：固定长度十六进制摘要。
func Token(key string) string { sum := sha256.Sum256([]byte(key)); return hex.EncodeToString(sum[:16]) }

type MockCarrier struct{}

// CreateLabel 用途：生成明确标记 MOCK 的确定性模拟面单，不访问网络。
// 参数：ctx 取消上下文；key 幂等请求键；in 订单引用。
// 返回值：模拟承运信息或取消错误。
func (MockCarrier) CreateLabel(ctx context.Context, key string, in LabelRequest) (Label, error) {
	if err := ctx.Err(); err != nil {
		return Label{}, err
	}
	return Label{CarrierID: in.CarrierID, Tracking: "MOCK-" + Token(key)}, nil
}

// GetLabel 用途：报告无外部存储的模拟适配器可用同键安全重建。
// 参数：ctx 上下文；key 请求键。
// 返回值：未查到结果；调用方可根据幂等能力同键调用。
func (MockCarrier) GetLabel(ctx context.Context, key string) (Label, bool, error) {
	return Label{}, false, ctx.Err()
}

// Idempotent 用途：声明模拟面单支持稳定键重复调用。
// 参数：无。
// 返回值：true。
func (MockCarrier) Idempotent() bool { return true }

type MockNotifier struct{}

// Send 用途：返回模拟回执，不向任何真实联系人发送消息。
// 参数：ctx 上下文；key 事件键；in 模板及虚拟收件引用。
// 返回值：MOCK 回执或取消错误。
func (MockNotifier) Send(ctx context.Context, key string, in Notice) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return "MOCK-" + Token(key), nil
}

// Lookup 用途：按事件键重建模拟通知回执。
// 参数：ctx 上下文；key 事件键。
// 返回值：模拟回执、存在标记或取消错误。
func (MockNotifier) Lookup(ctx context.Context, key string) (string, bool, error) {
	return "MOCK-" + Token(key), true, ctx.Err()
}

// Idempotent 用途：声明模拟通知支持同键去重。
// 参数：无。
// 返回值：true。
func (MockNotifier) Idempotent() bool { return true }
