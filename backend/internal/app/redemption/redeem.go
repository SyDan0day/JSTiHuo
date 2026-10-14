package redemption

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-sql-driver/mysql"
	"jstihuo/internal/app/identity"
	"jstihuo/internal/domain/calendar"
	"regexp"
	"sort"
	"strconv"
	"time"
	"unicode/utf8"
)

type RedeemResult struct {
	Order   Order
	Session identity.LoginResult
}

var keyPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{16,64}$`)

// numericLess 用途：比较正十进制字符串的数值顺序。
// 参数：a 第一个规范十进制标识；b 第二个规范十进制标识。
// 返回值：a 小于 b 时为 true。
// 约束：避免大整数精度损失。
func numericLess(a, b string) bool {
	if len(a) != len(b) {
		return len(a) < len(b)
	}
	return a < b
}

// validate 用途：验证并规范化兑换命令。
// 参数：in 选品、地址和预约命令。
// 返回值：nil 或字段、选品、日期错误。
// 约束：商品按数值标识排序供摘要和锁序复用。
func (s *Service) validate(in *Input) error {
	if in.Policy != "selection-v1" || len(in.Items) < 1 || len(in.Items) > 99 || utf8.RuneCountInString(in.Note) > 200 {
		return fault(422, "SELECTION_INVALID")
	}
	if e := s.ValidateAddress(&in.Address); e != nil {
		return e
	}
	if _, e := time.Parse("2006-01-02", in.Date); e != nil {
		return fault(422, "DATE_UNAVAILABLE")
	}
	seen := map[string]bool{}
	for _, item := range in.Items {
		if _, e := parseID(item.ProductID); e != nil || item.Quantity != 1 || seen[item.ProductID] {
			return fault(422, "SELECTION_INVALID")
		}
		seen[item.ProductID] = true
	}
	sort.Slice(in.Items, func(i, j int) bool { return numericLess(in.Items[i].ProductID, in.Items[j].ProductID) })
	return nil
}

// Redeem 用途：幂等核销并创建配送订单，再补发对应 WRITE 会话。
// 参数：ctx 请求上下文；a 已认证操作主体。
// 参数：in 选品、地址和预约命令；key 当前动作的幂等键。
// 参数：rid 审计请求标识。
// 返回值：脱敏订单、内部会话或校验、冲突、库存、容量、数据库错误。
// 事务：一次发布核销、订单、预占、日额度、审计和 Outbox；同键可恢复成功。
// 并发：死锁或锁等待最多重试三次；消费检查在成功重放之后。
func (s *Service) Redeem(ctx context.Context, a identity.Actor, in Input, key, rid string) (RedeemResult, error) {
	if a.Kind != "VOUCHER" || a.Scope != "WRITE" {
		return RedeemResult{}, fault(403, "SCOPE_REQUIRED")
	}
	if !keyPattern.MatchString(key) {
		return RedeemResult{}, fault(400, "INVALID_ARGUMENT")
	}
	in.Items = append([]Selection(nil), in.Items...)
	if e := s.validate(&in); e != nil {
		return RedeemResult{}, e
	}
	config, e := s.Configuration(ctx)
	if e != nil {
		return RedeemResult{}, e
	}
	raw, _ := json.Marshal(in)
	mac := hmac.New(sha256.New, s.RequestKey)
	mac.Write(raw)
	hash := mac.Sum(nil)
	// Create the business number and encrypted address before locks. A different
	// candidate on replay is ignored; only the committed reference is returned.
	number, e := s.Number(s.Now())
	if e != nil {
		return RedeemResult{}, e
	}
	address, _ := json.Marshal(RecipientData{in.Address, RegionNames(s.Regions, in.Address)})
	encrypted, keyVersion, e := s.PII.Encrypt(address, OrderAAD(number))
	clear(address)
	if e != nil {
		return RedeemResult{}, e
	}
	phoneMAC := hmac.New(sha256.New, s.LookupKey)
	phoneMAC.Write([]byte(in.Address.Phone))
	phoneHash := phoneMAC.Sum(nil)
	var id string
	for attempt := 0; attempt < 3; attempt++ {
		id, e = s.redeemTx(ctx, a, in, key, rid, hash, number, encrypted, keyVersion, phoneHash, config)
		if e == nil {
			break
		}
		var sqlError *mysql.MySQLError
		if errors.As(e, &sqlError) && sqlError.Number == 1062 {
			number, e = s.Number(s.Now())
			if e != nil {
				return RedeemResult{}, e
			}
			address, e = json.Marshal(RecipientData{in.Address, RegionNames(s.Regions, in.Address)})
			if e != nil {
				return RedeemResult{}, e
			}
			encrypted, keyVersion, e = s.PII.Encrypt(address, OrderAAD(number))
			clear(address)
			if e != nil {
				return RedeemResult{}, e
			}
			e = fault(409, "CONFLICT")
			continue
		}
		if !errors.As(e, &sqlError) || (sqlError.Number != 1213 && sqlError.Number != 1205) {
			return RedeemResult{}, e
		}
		select {
		case <-ctx.Done():
			return RedeemResult{}, ctx.Err()
		case <-time.After(time.Duration(attempt+1) * 20 * time.Millisecond):
		}
	}
	if e != nil {
		return RedeemResult{}, e
	}
	o, e := s.GetOrder(ctx, id, false)
	if e != nil {
		return RedeemResult{}, e
	}
	session, e := s.IssueOrder(ctx, a, id, "WRITE")
	if e != nil {
		return RedeemResult{}, e
	}
	return RedeemResult{o, session}, nil
}

// IssueOrder 用途：为刚成交的同卡订单签发作用域会话。
// 参数：ctx 请求上下文；a 已认证操作主体。
// 参数：id 目标对象标识；scope 会话权限范围。
// 返回值：内部会话或归属、凭证版本、数据库错误。
// 约束：独立事务失败不撤销已成交订单。
func (s *Service) IssueOrder(ctx context.Context, a identity.Actor, id, scope string) (identity.LoginResult, error) {
	result, e := s.Auth.PrepareCustomer("ORDER")
	if e != nil {
		return result, e
	}
	tx, e := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if e != nil {
		return result, e
	}
	defer tx.Rollback()
	var voucherID, cred int64
	e = tx.QueryRowContext(ctx, "SELECT v.id,v.credential_version FROM orders o JOIN voucher_redemptions r ON r.id=o.redemption_id JOIN vouchers v ON v.id=r.voucher_id WHERE o.id=? FOR SHARE", id).Scan(&voucherID, &cred)
	if e != nil {
		return result, e
	}
	if a.Kind != "VOUCHER" || voucherID != a.VoucherID || cred != a.AuthVersion {
		return result, fault(403, "FORBIDDEN")
	}
	oid, e := strconv.ParseInt(id, 10, 64)
	if e != nil {
		return result, e
	}
	if e = s.Auth.InsertCustomer(ctx, tx, result, nil, "ORDER", scope, voucherID, oid, cred); e != nil {
		return result, e
	}
	return result, tx.Commit()
}

// inject 用途：运行仅由测试装配的事务故障钩子。
// 参数：ctx 请求上下文；tx 当前业务事务。
// 参数：step 测试故障边界名称。
// 返回值：钩子错误或 nil。
// 约束：生产装配不设置钩子，不执行故障操作。
func (s *Service) inject(ctx context.Context, tx *sql.Tx, step string) error {
	if s.FaultTx != nil {
		if e := s.FaultTx(ctx, tx, step); e != nil {
			return e
		}
	}
	if s.Fault != nil {
		return s.Fault(step)
	}
	return nil
}

// redeemTx 用途：在单个事务中完成核销、订单、预占、日额度、审计、Outbox 和幂等记录。
// 参数：ctx 请求上下文；a 已认证操作主体。
// 参数：in 选品、地址和预约命令；key 当前动作的幂等键。
// 参数：rid 审计请求标识；hash 请求 HMAC 摘要。
// 参数：number 服务器生成的订单号；encrypted 锁前准备的地址密文。
// 参数：keyVersion 密文密钥版本；phoneHash 独立手机号 HMAC。
// 参数：config 日历配置快照。
// 返回值：订单标识或业务、SQL、提交错误。
// 事务：固定锁序，全部业务与成功引用同提交；失败整体回滚。
func (s *Service) redeemTx(ctx context.Context, a identity.Actor, in Input, key, rid string, hash []byte, number string, encrypted []byte, keyVersion int, phoneHash []byte, config calendar.Config) (string, error) {
	tx, e := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if e != nil {
		return "", e
	}
	defer tx.Rollback()
	now := s.Now().UTC()
	scope := fmt.Sprintf("voucher:%d:redeem", a.VoucherID)
	if _, e = tx.ExecContext(ctx, "INSERT INTO idempotency_keys(scope,request_key,request_hash,result_ref,expires_at) VALUES(?,?,?,JSON_OBJECT(),?) ON DUPLICATE KEY UPDATE scope=scope", scope, key, hash, now.Add(24*time.Hour)); e != nil {
		return "", e
	}
	var stored, ref []byte
	var expiry time.Time
	if e = tx.QueryRowContext(ctx, "SELECT request_hash,result_ref,expires_at FROM idempotency_keys WHERE scope=? AND request_key=? FOR UPDATE", scope, key).Scan(&stored, &ref, &expiry); e != nil {
		return "", e
	}
	// Persisted session/credential checks still precede a replay; redeemability does not.
	var batchID string
	if e = tx.QueryRowContext(ctx, "SELECT batch_id FROM vouchers WHERE id=?", a.VoucherID).Scan(&batchID); e != nil {
		return "", e
	}
	var batchState string
	if e = tx.QueryRowContext(ctx, "SELECT state FROM voucher_batches WHERE id=? FOR UPDATE", batchID).Scan(&batchState); e != nil {
		return "", e
	}
	c, e := s.card(ctx, tx, fmt.Sprint(a.VoucherID), "", true)
	if e != nil {
		return "", e
	}
	var sessionExpiry time.Time
	var subjectVersion int64
	var sessionScope string
	if e = tx.QueryRowContext(ctx, "SELECT expires_at,subject_version,scope FROM sessions WHERE id=? AND voucher_id=? AND kind='VOUCHER' FOR SHARE", a.SessionID, a.VoucherID).Scan(&sessionExpiry, &subjectVersion, &sessionScope); e != nil {
		return "", fault(401, "AUTH_REQUIRED")
	}
	if subjectVersion != c.CredentialVersion || a.AuthVersion != c.CredentialVersion || !s.Now().Before(sessionExpiry) || sessionScope != "WRITE" {
		return "", fault(401, "AUTH_REQUIRED")
	}
	if expiry.After(now) {
		if !hmac.Equal(stored, hash) {
			return "", fault(409, "IDEMPOTENCY_CONFLICT")
		}
		var previous struct {
			ID string `json:"id"`
		}
		if e = json.Unmarshal(ref, &previous); e != nil {
			return "", e
		}
		if previous.ID != "" {
			return previous.ID, tx.Commit()
		}
	}
	if c.Redeemed {
		return "", fault(409, "ALREADY_REDEEMED")
	}
	if !s.eligible(c) {
		return "", fault(422, "VOUCHER_UNAVAILABLE")
	}
	if len(in.Items) != c.Policy.Count {
		return "", fault(422, "SELECTION_INVALID")
	}
	allowed := map[string]bool{}
	for _, id := range c.Policy.Products {
		allowed[id] = true
	}
	for _, item := range in.Items {
		if !allowed[item.ProductID] {
			return "", fault(422, "SELECTION_INVALID")
		}
	}
	if !config.Allowed(s.Now(), in.Date) {
		return "", fault(422, "DATE_UNAVAILABLE")
	}
	if _, e = tx.ExecContext(ctx, "INSERT INTO shipping_calendars(ship_date,capacity) VALUES(?,?) ON DUPLICATE KEY UPDATE ship_date=ship_date", in.Date, config.Capacity); e != nil {
		return "", e
	}
	var calendarID string
	var capacity, used int
	var closed bool
	if e = tx.QueryRowContext(ctx, "SELECT id,capacity,used_slots,closed FROM shipping_calendars WHERE ship_date=? FOR UPDATE", in.Date).Scan(&calendarID, &capacity, &used, &closed); e != nil {
		return "", e
	}
	if closed {
		return "", fault(422, "DATE_UNAVAILABLE")
	}
	if used >= capacity {
		return "", fault(422, "CAPACITY_EXCEEDED")
	}
	snapshots := make([][]byte, len(in.Items))
	for i, item := range in.Items {
		var name, code, status string
		var attrs, excluded []byte
		var from, until, deleted sql.NullTime
		if e = tx.QueryRowContext(ctx, "SELECT name,code,status,attributes,excluded_region_codes,redeem_from,redeem_until,deleted_at FROM products WHERE id=? FOR UPDATE", item.ProductID).Scan(&name, &code, &status, &attrs, &excluded, &from, &until, &deleted); e != nil {
			return "", e
		}
		date, _ := time.ParseInLocation("2006-01-02", in.Date, calendar.Shanghai)
		if deleted.Valid || status != "ENABLED" || (from.Valid && (s.Now().Before(from.Time) || date.Before(from.Time))) || (until.Valid && (!s.Now().Before(until.Time) || !date.Before(until.Time))) {
			return "", fault(422, "DATE_UNAVAILABLE")
		}
		var exclusions []string
		if e = json.Unmarshal(excluded, &exclusions); e != nil {
			return "", e
		}
		if Restricted(exclusions, in.Address) {
			return "", fault(422, "REGION_RESTRICTED")
		}
		snapshots[i], e = json.Marshal(map[string]any{"name": name, "code": code, "attributes": json.RawMessage(attrs)})
		if e != nil {
			return "", e
		}
	}
	for _, item := range in.Items {
		var onhand, reserved int64
		if e = tx.QueryRowContext(ctx, "SELECT on_hand,reserved FROM inventory_balances WHERE product_id=? FOR UPDATE", item.ProductID).Scan(&onhand, &reserved); e != nil {
			return "", e
		}
		if onhand-reserved < 1 {
			return "", fault(422, "OUT_OF_STOCK")
		}
	}
	policy, _ := json.Marshal(c.Policy)
	r, e := tx.ExecContext(ctx, "INSERT INTO voucher_redemptions(voucher_id,policy_snapshot,redeemed_at) VALUES(?,?,?)", c.ID, policy, now)
	if e != nil {
		return "", e
	}
	redemptionID, e := r.LastInsertId()
	if e != nil {
		return "", e
	}
	r, e = tx.ExecContext(ctx, "INSERT INTO orders(order_no,redemption_id,calendar_id,recipient_cipher,pii_key_version,phone_lookup,customer_note,admin_note) VALUES(?,?,?,?,?,?,?,'')", number, redemptionID, calendarID, encrypted, keyVersion, phoneHash, in.Note)
	if e != nil {
		return "", e
	}
	oid, e := r.LastInsertId()
	if e != nil {
		return "", e
	}
	id := fmt.Sprint(oid)
	if e = s.inject(ctx, tx, "order"); e != nil {
		return "", e
	}
	for i, item := range in.Items {
		r, e = tx.ExecContext(ctx, "INSERT INTO order_items(order_id,product_id,product_snapshot) VALUES(?,?,?)", id, item.ProductID, snapshots[i])
		if e != nil {
			return "", e
		}
		itemID, e := r.LastInsertId()
		if e != nil {
			return "", e
		}
		r, e = tx.ExecContext(ctx, "UPDATE inventory_balances SET reserved=reserved+1,version=version+1 WHERE product_id=? AND on_hand-reserved>=1", item.ProductID)
		if e != nil {
			return "", e
		}
		n, e := r.RowsAffected()
		if e != nil {
			return "", e
		}
		if n != 1 {
			return "", fault(422, "OUT_OF_STOCK")
		}
		if _, e = tx.ExecContext(ctx, "INSERT INTO inventory_reservations(order_item_id,product_id,quantity) VALUES(?,?,1)", itemID, item.ProductID); e != nil {
			return "", e
		}
		if _, e = tx.ExecContext(ctx, "INSERT INTO inventory_transactions(product_id,reserved_delta,event_key,order_id,reason) VALUES(?,1,?,?,'redemption')", item.ProductID, "reserve:"+id+":"+item.ProductID, id); e != nil {
			return "", e
		}
		if e = s.inject(ctx, tx, fmt.Sprintf("reserve:%d", i+1)); e != nil {
			return "", e
		}
	}
	r, e = tx.ExecContext(ctx, "UPDATE shipping_calendars SET used_slots=used_slots+1,version=version+1 WHERE id=? AND used_slots<capacity AND closed=0", calendarID)
	if e != nil {
		return "", e
	}
	n, e := r.RowsAffected()
	if e != nil {
		return "", e
	}
	if n != 1 {
		return "", fault(422, "CAPACITY_EXCEEDED")
	}
	if e = s.inject(ctx, tx, "calendar"); e != nil {
		return "", e
	}
	if _, e = tx.ExecContext(ctx, "UPDATE vouchers SET redeemed_at=?,version=version+1 WHERE id=?", now, c.ID); e != nil {
		return "", e
	}
	if _, e = tx.ExecContext(ctx, "INSERT INTO outbox_events(event_key,event_type,payload,next_attempt_at) VALUES(?,'order.created',JSON_OBJECT('order_id',?),?)", "order.created:"+id, id, now); e != nil {
		return "", e
	}
	if e = s.inject(ctx, tx, "outbox"); e != nil {
		return "", e
	}
	if _, e = tx.ExecContext(ctx, "INSERT INTO audit_logs(actor_kind,action,entity_type,entity_id,request_id,diff,result) VALUES('CUSTOMER','redeem','order',?,?,JSON_OBJECT('voucher_id',?),'SUCCESS')", id, rid, c.ID); e != nil {
		return "", e
	}
	encoded, _ := json.Marshal(map[string]string{"id": id})
	if _, e = tx.ExecContext(ctx, "UPDATE idempotency_keys SET request_hash=?,result_ref=?,response_status=201,expires_at=? WHERE scope=? AND request_key=?", hash, encoded, now.Add(24*time.Hour), scope, key); e != nil {
		return "", e
	}
	if e = s.inject(ctx, tx, "commit"); e != nil {
		return "", e
	}
	return id, tx.Commit()
}
