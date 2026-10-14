package order

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-sql-driver/mysql"
	"jstihuo/internal/app/identity"
	"jstihuo/internal/app/redemption"
	"jstihuo/internal/app/voucher"
	rules "jstihuo/internal/domain/voucher"
	crypt "jstihuo/internal/infra/crypto"
)

type PickupInput struct {
	Code   string                 `json:"code"`
	Secret string                 `json:"secret"`
	Items  []redemption.Selection `json:"items"`
	Reason string                 `json:"reason"`
}
type pickupCard struct {
	ID, BatchID, Hash         string
	Credential                int64
	Active, Redeemed, Deleted bool
	State, BatchState         string
	From, Until               time.Time
	Policy                    voucher.Policy
}

// Pickup 用途：现场验密后幂等创建已完成的到店订单。
// 参数：ctx 请求上下文；actor 管理员；input 卡号、原卡密、商品集合和原因。
// 参数：key 幂等键；requestID 审计请求标识。
// 返回值：脱敏到店订单或权限、凭证、权益、库存、唯一冲突及 SQL 错误。
// 事务：核销和真实库存扣减一次提交，无配送地址、预约名额、运单或预占。
func (s *Service) Pickup(
	ctx context.Context, actor identity.Actor, input PickupInput, key, requestID string,
) (redemption.Order, error) {
	if actor.Kind != "ADMIN" || !actor.Can("order.pickup") {
		return redemption.Order{}, fault(403, "FORBIDDEN")
	}
	if err := validatePickup(&input); err != nil {
		return redemption.Order{}, err
	}
	if !keyPattern.MatchString(key) {
		return redemption.Order{}, fault(400, "INVALID_ARGUMENT")
	}
	if err := s.Auth.Limit(ctx, fmt.Sprintf("pickup-admin:%d", actor.AdminID), 60, 15*time.Minute); err != nil {
		return redemption.Order{}, err
	}
	card, err := s.findPickupCard(ctx, input.Code)
	hash := s.Redemption.Dummy
	if err == nil {
		hash = card.Hash
	}
	valid := crypt.Verify(input.Secret, hash)
	if err != nil && err != sql.ErrNoRows {
		return redemption.Order{}, err
	}
	if !valid || err != nil || card.Deleted {
		return redemption.Order{}, fault(401, "INVALID_CREDENTIALS")
	}
	raw, err := json.Marshal(input)
	if err != nil {
		return redemption.Order{}, err
	}
	mac := hmac.New(sha256.New, s.Redemption.RequestKey)
	mac.Write(raw)
	var orderID string
	for attempt := 0; attempt < 3; attempt++ {
		orderID, err = s.pickupTx(ctx, actor, input, card, key, requestID, mac.Sum(nil))
		if err == nil {
			return s.Redemption.GetOrder(ctx, orderID, true)
		}
		var sqlError *mysql.MySQLError
		if !errors.As(err, &sqlError) || (sqlError.Number != 1062 && sqlError.Number != 1213 && sqlError.Number != 1205) {
			return redemption.Order{}, err
		}
		select {
		case <-ctx.Done():
			return redemption.Order{}, ctx.Err()
		case <-time.After(time.Duration(attempt+1) * 20 * time.Millisecond):
		}
	}
	return redemption.Order{}, err
}

// validatePickup 用途：规范卡号并验证固定一件的去重商品集合。
// 参数：input 可规范化的到店命令，原卡密保持字节不变。
// 返回值：nil 或字段、数量、重复商品错误；排序用于摘要与全局锁序。
func validatePickup(input *PickupInput) error {
	input.Code = strings.ToUpper(strings.TrimSpace(input.Code))
	validCode := regexp.MustCompile(`^[A-Z0-9]{6,32}$`).MatchString(input.Code)
	validReason := strings.TrimSpace(input.Reason) != "" && utf8.RuneCountInString(input.Reason) <= 200
	if !validCode || len(input.Secret) < 6 || len(input.Secret) > 32 || !validReason || len(input.Items) < 1 || len(input.Items) > 99 {
		return fault(422, "VALIDATION_FAILED")
	}
	input.Items = append([]redemption.Selection(nil), input.Items...)
	seen := map[string]bool{}
	for _, item := range input.Items {
		if !rules.ValidID(item.ProductID) || item.Quantity != 1 || seen[item.ProductID] {
			return fault(422, "SELECTION_INVALID")
		}
		seen[item.ProductID] = true
	}
	sort.Slice(input.Items, func(i, j int) bool {
		left, right := input.Items[i].ProductID, input.Items[j].ProductID
		if len(left) != len(right) {
			return len(left) < len(right)
		}
		return left < right
	})
	return nil
}

// findPickupCard 用途：读取验密所需卡券与冻结策略。
// 参数：ctx 请求上下文；code 规范化卡号。
// 返回值：卡券快照或不存在、SQL、策略 JSON 错误，不修改消费状态。
func (s *Service) findPickupCard(ctx context.Context, code string) (pickupCard, error) {
	var card pickupCard
	var policy []byte
	err := s.DB.QueryRowContext(ctx, "SELECT v.id,v.batch_id,v.secret_hash,v.credential_version,v.active,v.redeemed_at IS NOT NULL,v.deleted_at IS NOT NULL,v.credential_state,v.valid_from,v.valid_until,b.state,b.policy_snapshot FROM vouchers v JOIN voucher_batches b ON b.id=v.batch_id WHERE v.code=?", code).Scan(&card.ID, &card.BatchID, &card.Hash, &card.Credential, &card.Active, &card.Redeemed, &card.Deleted, &card.State, &card.From, &card.Until, &card.BatchState, &policy)
	if err == nil {
		err = json.Unmarshal(policy, &card.Policy)
	}
	return card, err
}

// pickupTx 用途：执行一轮到店核销事务并保存安全重放引用。
// 参数：ctx 请求上下文；actor 管理员；input 到店命令；expected 锁前验密快照。
// 参数：key 幂等键；requestID 请求标识；hash 请求 HMAC 摘要。
// 返回值：订单 ID 或权限、摘要冲突、卡状态、权益、库存及 SQL 错误。
// 并发：幂等键、批次、卡券、产品、余额按序锁定，重放在消费状态检查之前。
func (s *Service) pickupTx(
	ctx context.Context, actor identity.Actor, input PickupInput, expected pickupCard,
	key, requestID string, hash []byte,
) (string, error) {
	number, err := s.Redemption.Number(s.Redemption.Now())
	if err != nil {
		return "", err
	}
	encrypted, version, err := s.Redemption.PII.Encrypt([]byte(`{}`), redemption.OrderAAD(number))
	if err != nil {
		return "", err
	}
	phoneLookup := make([]byte, 32)
	if _, err = rand.Read(phoneLookup); err != nil {
		return "", err
	}
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	scope := fmt.Sprintf("admin:%d:pickup", actor.AdminID)
	now := s.Redemption.Now().UTC()
	_, err = tx.ExecContext(ctx, "INSERT INTO idempotency_keys(scope,request_key,request_hash,result_ref,expires_at) VALUES(?,?,?,JSON_OBJECT(),?) ON DUPLICATE KEY UPDATE scope=scope", scope, key, hash, now.Add(24*time.Hour))
	if err != nil {
		return "", err
	}
	var stored, reference []byte
	var expires time.Time
	err = tx.QueryRowContext(ctx, "SELECT request_hash,result_ref,expires_at FROM idempotency_keys WHERE scope=? AND request_key=? FOR UPDATE", scope, key).Scan(&stored, &reference, &expires)
	if err != nil {
		return "", err
	}
	if err = identity.AuthorizeAdmin(ctx, tx, actor, "order.pickup"); err != nil {
		return "", err
	}
	current, err := lockPickupCard(ctx, tx, expected)
	if err != nil {
		return "", err
	}
	if expires.After(now) {
		if !hmac.Equal(stored, hash) {
			return "", fault(409, "IDEMPOTENCY_CONFLICT")
		}
		var ref struct {
			ID string `json:"id"`
		}
		if err = json.Unmarshal(reference, &ref); err != nil {
			return "", err
		}
		if ref.ID != "" {
			return ref.ID, tx.Commit()
		}
	}
	available := current.Active && !current.Deleted && current.State == "SEALED" && current.BatchState == "SEALED" && rules.ValidAt(s.Redemption.Now(), current.From, current.Until)
	if current.Redeemed {
		return "", fault(409, "ALREADY_REDEEMED")
	}
	if !available {
		return "", fault(422, "VOUCHER_UNAVAILABLE")
	}
	snapshots, err := lockPickupProducts(ctx, tx, current.Policy, input.Items, s.Redemption.Now())
	if err != nil {
		return "", err
	}
	for _, item := range input.Items {
		var quantity int
		err = tx.QueryRowContext(ctx, "SELECT on_hand-reserved FROM inventory_balances WHERE product_id=? FOR UPDATE", item.ProductID).Scan(&quantity)
		if err != nil {
			return "", err
		}
		if quantity < 1 {
			return "", fault(422, "OUT_OF_STOCK")
		}
	}
	orderID, err := s.createPickupOrder(ctx, tx, current, input, number, encrypted, version, phoneLookup, snapshots)
	if err != nil {
		return "", err
	}
	if err = s.event(ctx, tx, "order.completed", orderID, 1); err != nil {
		return "", err
	}
	diff, _ := json.Marshal(map[string]string{"reason": input.Reason, "voucher_id": current.ID})
	if err = s.inject(ctx, tx, "audit"); err != nil {
		return "", err
	}
	_, err = tx.ExecContext(ctx, "INSERT INTO audit_logs(actor_admin_id,actor_kind,action,entity_type,entity_id,request_id,diff,result) VALUES(?,'ADMIN','pickup','order',?,?,?,'SUCCESS')", actor.AdminID, orderID, requestID, diff)
	if err != nil {
		return "", err
	}
	encoded, _ := json.Marshal(map[string]string{"id": orderID})
	_, err = tx.ExecContext(ctx, "UPDATE idempotency_keys SET request_hash=?,result_ref=?,response_status=200,expires_at=? WHERE scope=? AND request_key=?", hash, encoded, now.Add(24*time.Hour), scope, key)
	if err != nil {
		return "", err
	}
	if err = s.inject(ctx, tx, "commit"); err != nil {
		return "", err
	}
	return orderID, tx.Commit()
}

// lockPickupCard 用途：按批次、卡券锁序复核原凭证。
// 参数：ctx 请求上下文；tx 当前事务；expected 已通过锁前哈希校验的快照。
// 返回值：当前卡券或凭证轮换、SQL 错误，禁止验密后用旧版本完成交易。
func lockPickupCard(ctx context.Context, tx *sql.Tx, expected pickupCard) (pickupCard, error) {
	current := expected
	err := tx.QueryRowContext(ctx, "SELECT state FROM voucher_batches WHERE id=? FOR UPDATE", expected.BatchID).Scan(&current.BatchState)
	if err != nil {
		return current, err
	}
	err = tx.QueryRowContext(ctx, "SELECT secret_hash,credential_version,active,redeemed_at IS NOT NULL,deleted_at IS NOT NULL,credential_state,valid_from,valid_until FROM vouchers WHERE id=? FOR UPDATE", expected.ID).Scan(&current.Hash, &current.Credential, &current.Active, &current.Redeemed, &current.Deleted, &current.State, &current.From, &current.Until)
	if err != nil {
		return current, err
	}
	if current.Hash != expected.Hash || current.Credential != expected.Credential {
		return current, fault(401, "INVALID_CREDENTIALS")
	}
	return current, nil
}

// lockPickupProducts 用途：校验冻结权益并锁定现场商品快照。
// 参数：ctx 请求上下文；tx 当前事务；policy 冻结选品规则；items 去重升序商品；now 当前时刻。
// 返回值：商品快照或选品、启用、有效期及 SQL 错误，无配送日期或地区要求。
func lockPickupProducts(ctx context.Context, tx *sql.Tx, policy voucher.Policy, items []redemption.Selection, now time.Time) ([][]byte, error) {
	if len(items) != policy.Count {
		return nil, fault(422, "SELECTION_INVALID")
	}
	allowed := map[string]bool{}
	for _, id := range policy.Products {
		allowed[id] = true
	}
	snapshots := make([][]byte, len(items))
	for index, item := range items {
		if !allowed[item.ProductID] {
			return nil, fault(422, "SELECTION_INVALID")
		}
		var name, code, status string
		var attrs []byte
		var from, until, deleted sql.NullTime
		err := tx.QueryRowContext(ctx, "SELECT name,code,status,attributes,redeem_from,redeem_until,deleted_at FROM products WHERE id=? FOR UPDATE", item.ProductID).Scan(&name, &code, &status, &attrs, &from, &until, &deleted)
		if err != nil {
			return nil, err
		}
		active := status == "ENABLED" && !deleted.Valid && (!from.Valid || !now.Before(from.Time)) && (!until.Valid || now.Before(until.Time))
		if !active {
			return nil, fault(422, "VOUCHER_UNAVAILABLE")
		}
		snapshots[index], err = json.Marshal(map[string]any{"name": name, "code": code, "attributes": json.RawMessage(attrs)})
		if err != nil {
			return nil, err
		}
	}
	return snapshots, nil
}

// createPickupOrder 用途：创建现场完成订单并只扣可用实存。
// 参数：ctx 请求上下文；tx 当前事务；card 锁定卡；input 到店命令；number 业务号。
// 参数：encrypted 空地址密文；version 密钥版本；phoneLookup 随机不可查询值；snapshots 冻结商品信息。
// 返回值：订单 ID 或库存、SQL、故障错误，调用方负责全批回滚和提交。
func (s *Service) createPickupOrder(
	ctx context.Context, tx *sql.Tx, card pickupCard, input PickupInput, number string,
	encrypted []byte, version int, phoneLookup []byte, snapshots [][]byte,
) (string, error) {
	policy, err := json.Marshal(card.Policy)
	if err != nil {
		return "", err
	}
	result, err := tx.ExecContext(ctx, "INSERT INTO voucher_redemptions(voucher_id,policy_snapshot,redeemed_at) VALUES(?,?,?)", card.ID, policy, s.Redemption.Now().UTC())
	if err != nil {
		return "", err
	}
	redemptionID, err := result.LastInsertId()
	if err != nil {
		return "", err
	}
	result, err = tx.ExecContext(ctx, "INSERT INTO orders(order_no,redemption_id,status,fulfillment_mode,recipient_cipher,pii_key_version,phone_lookup,customer_note,admin_note,completed_at) VALUES(?,?,'COMPLETED','PICKUP',?,?,?,'',?,?)", number, redemptionID, encrypted, version, phoneLookup, input.Reason, s.Redemption.Now().UTC())
	if err != nil {
		return "", err
	}
	created, err := result.LastInsertId()
	if err != nil {
		return "", err
	}
	orderID := fmt.Sprint(created)
	for index, item := range input.Items {
		_, err = tx.ExecContext(ctx, "INSERT INTO order_items(order_id,product_id,product_snapshot) VALUES(?,?,?)", orderID, item.ProductID, snapshots[index])
		if err != nil {
			return "", err
		}
		result, err = tx.ExecContext(ctx, "UPDATE inventory_balances SET on_hand=on_hand-1,version=version+1 WHERE product_id=? AND on_hand-reserved>=1", item.ProductID)
		if err != nil {
			return "", err
		}
		changed, err := result.RowsAffected()
		if err != nil {
			return "", err
		}
		if changed != 1 {
			return "", fault(422, "OUT_OF_STOCK")
		}
		_, err = tx.ExecContext(ctx, "INSERT INTO inventory_transactions(product_id,on_hand_delta,event_key,order_id,reason) VALUES(?,-1,?,?,?)", item.ProductID, "pickup:"+orderID+":"+item.ProductID, orderID, input.Reason)
		if err != nil {
			return "", err
		}
		if err = s.inject(ctx, tx, fmt.Sprintf("pickup:%d", index+1)); err != nil {
			return "", err
		}
	}
	_, err = tx.ExecContext(ctx, "UPDATE vouchers SET redeemed_at=?,version=version+1 WHERE id=?", s.Redemption.Now().UTC(), card.ID)
	return orderID, err
}
