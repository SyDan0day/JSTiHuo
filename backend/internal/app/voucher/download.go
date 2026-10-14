package voucher

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"io"
	"jstihuo/internal/app/identity"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// authorizeDownload 用途：重查下载的持久化身份和权限。
// 参数：ctx 请求上下文；tx 当前业务事务。
// 参数：a 已认证操作主体。
// 返回值：nil 或身份、权限、复核、SQL 错误。
// 约束：共享锁保护管理员和会话，要求三项权限及五分钟复核。
func (s *Service) authorizeDownload(ctx context.Context, tx *sql.Tx, a identity.Actor) error {
	var enabled, force bool
	var authVersion int64
	if e := tx.QueryRowContext(ctx, "SELECT enabled,force_password_change,auth_version FROM admin_users WHERE id=? FOR SHARE", a.AdminID).Scan(&enabled, &force, &authVersion); e != nil {
		return e
	}
	if !enabled || authVersion != a.AuthVersion {
		return fault(401, "AUTH_REQUIRED")
	}
	if force {
		return fault(403, "PASSWORD_CHANGE_REQUIRED")
	}
	for _, permission := range []string{"file.read", "voucher.export", "voucher.generate"} {
		var count int
		if e := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM admin_user_roles ur JOIN roles r ON r.id=ur.role_id AND r.enabled=1 JOIN role_permissions rp ON rp.role_id=r.id JOIN permissions p ON p.id=rp.permission_id WHERE ur.admin_id=? AND p.permission_key=?", a.AdminID, permission).Scan(&count); e != nil {
			return e
		}
		if count == 0 {
			return fault(403, "FORBIDDEN")
		}
	}
	var recent sql.NullTime
	if e := tx.QueryRowContext(ctx, "SELECT reauth_at FROM sessions WHERE id=? AND admin_id=? AND expires_at>? FOR SHARE", a.SessionID, a.AdminID, s.Now().UTC()).Scan(&recent); e != nil {
		if e == sql.ErrNoRows {
			return fault(401, "AUTH_REQUIRED")
		}
		return e
	}
	if !recent.Valid || recent.Time.After(s.Now()) || s.Now().Sub(recent.Time) >= 5*time.Minute {
		return fault(403, "REAUTH_REQUIRED")
	}
	return nil
}

// Download 用途：授权后在文件锁内提供凭证流。
// 参数：ctx 请求上下文；a 已认证操作主体。
// 参数：id 目标对象标识；rid 审计请求标识。
// 参数：stream 持有授权锁期间的输出回调。
// 返回值：nil 或归属、过期、密钥、文件完整性、SQL、输出错误。
// 约束：摘要及 GCM 认证通过且审计持久化后才输出，明文不写磁盘。
// 并发：授权锁保持至输出完成，调用方限制最长六十秒。
func (s *Service) Download(ctx context.Context, a identity.Actor, id, rid string, stream func(io.Reader, string) error) error {
	tx, e := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if e = s.authorizeDownload(ctx, tx, a); e != nil {
		return e
	}
	var batchID, voucherID string
	var subject string
	e = tx.QueryRowContext(ctx, "SELECT id FROM voucher_batches WHERE export_file_id=?", id).Scan(&batchID)
	if e == nil {
		var state string
		if e = tx.QueryRowContext(ctx, "SELECT state FROM voucher_batches WHERE id=? FOR UPDATE", batchID).Scan(&state); e != nil {
			return e
		}
		if state != "GENERATED" {
			return fault(410, "EXPORT_EXPIRED")
		}
		subject = batchID
	} else if e == sql.ErrNoRows {
		if e = tx.QueryRowContext(ctx, "SELECT id,batch_id FROM vouchers WHERE credential_file_id=? LIMIT 1", id).Scan(&voucherID, &batchID); e != nil {
			if e == sql.ErrNoRows {
				return fault(404, "NOT_FOUND")
			}
			return e
		}
		v, file, _, e := s.lockVoucher(ctx, tx, voucherID)
		if e != nil {
			return e
		}
		if v.State != "EXPORTABLE" || file != id || v.Active || v.Redeemed {
			return fault(410, "EXPORT_EXPIRED")
		}
		subject = "voucher:" + voucherID
	} else {
		return e
	}
	var storage, state, kind string
	var owner int64
	var version int
	var expiry time.Time
	var digest []byte
	var size int64
	if e = tx.QueryRowContext(ctx, "SELECT storage_key,state,kind,owner_admin_id,key_version,expires_at,sha256,size_bytes FROM files WHERE id=? FOR UPDATE", id).Scan(&storage, &state, &kind, &owner, &version, &expiry, &digest, &size); e != nil {
		return e
	}
	if owner != a.AdminID {
		return fault(403, "FORBIDDEN")
	}
	if kind != "CREDENTIAL_EXPORT" || state != "READY" || !s.Now().Before(expiry) {
		return fault(410, "EXPORT_EXPIRED")
	}
	if !strings.HasPrefix(storage, "credentials/") || filepath.Clean(storage) != storage || strings.Contains(storage, "..") || size < 28 || size > 512*1024 {
		return fault(500, "FILE_UNAVAILABLE")
	}
	f, e := os.Open(filepath.Join(s.Root, storage))
	if e != nil {
		return fault(503, "FILE_UNAVAILABLE")
	}
	defer f.Close()
	sealed, e := io.ReadAll(io.LimitReader(f, 512*1024+1))
	if e != nil || int64(len(sealed)) != size {
		return fault(500, "FILE_UNAVAILABLE")
	}
	got := sha256.Sum256(sealed)
	if !hmac.Equal(got[:], digest) {
		return fault(500, "FILE_UNAVAILABLE")
	}
	key := s.ExportKeys[version]
	if len(key) != 32 {
		return fault(503, "FILE_UNAVAILABLE")
	}
	block, e := aes.NewCipher(key)
	if e != nil {
		return e
	}
	gcm, e := cipher.NewGCM(block)
	if e != nil {
		return e
	}
	plain, e := gcm.Open(nil, sealed[:gcm.NonceSize()], sealed[gcm.NonceSize():], ExportAAD(subject, version))
	if e != nil {
		return fault(500, "FILE_UNAVAILABLE")
	}
	defer clear(plain)
	// Persist access before sending the first credential byte, independently of
	// stream cancellation or an uncertain read-only commit acknowledgment.
	if _, e = s.DB.ExecContext(ctx, "INSERT INTO audit_logs(actor_admin_id,actor_kind,action,entity_type,entity_id,request_id,diff,result) VALUES(?,'ADMIN','downloadFile','file',?,?,JSON_OBJECT(),'AUTHORIZED')", a.AdminID, id, rid); e != nil {
		return e
	}
	e = stream(bytes.NewReader(plain), fmt.Sprintf("credentials-%s.csv", id))
	if e != nil {
		return e
	}
	return tx.Commit()
}

// removeUnreferenced 用途：保守清理失败写入的密文。
// 参数：storage 内部随机文件存储键。
// 返回值：无。
// 约束：独立五秒查询确认无引用才删除；无法确认时保留供孤儿维护重试。
func (s *Service) removeUnreferenced(storage string) {
	ctx, stop := context.WithTimeout(context.Background(), 5*time.Second)
	defer stop()
	var count int
	if s.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM files WHERE storage_key=?", storage).Scan(&count) == nil && count == 0 {
		os.Remove(filepath.Join(s.Root, storage))
	}
}
