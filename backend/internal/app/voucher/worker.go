package voucher

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/csv"
	"encoding/hex"
	"errors"
	"fmt"
	rules "jstihuo/internal/domain/voucher"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

type Claim struct {
	Batch Batch
	Epoch int64
	Owner int64
}

var ErrLeaseLost = errors.New("GENERATION_LEASE_LOST")

// Claim 用途：领取待生成或租约到期的批次。
// 参数：ctx 请求上下文。
// 返回值：领取快照、是否有任务或 SQL 错误。
// 约束：SKIP LOCKED 防重复领取，短事务递增 epoch 并设置十分钟租约。
func (s *Service) Claim(ctx context.Context) (Claim, bool, error) {
	tx, e := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if e != nil {
		return Claim{}, false, e
	}
	defer tx.Rollback()
	var c Claim
	now := s.Now().UTC()
	e = tx.QueryRowContext(ctx, "SELECT id,generation_epoch,created_by FROM voucher_batches WHERE state='GENERATING' AND (lease_until IS NULL OR lease_until<=?) ORDER BY id LIMIT 1 FOR UPDATE SKIP LOCKED", now).Scan(&c.Batch.ID, &c.Epoch, &c.Owner)
	if e == sql.ErrNoRows {
		return c, false, nil
	}
	if e != nil {
		return c, false, e
	}
	c.Epoch++
	_, e = tx.ExecContext(ctx, "UPDATE voucher_batches SET generation_epoch=?,lease_until=?,version=version+1 WHERE id=?", c.Epoch, now.Add(10*time.Minute), c.Batch.ID)
	if e != nil {
		return c, false, e
	}
	c.Batch, e = s.batch(ctx, tx, c.Batch.ID)
	if e != nil {
		return c, false, e
	}
	return c, true, tx.Commit()
}

// renew 用途：续租当前 epoch 的生成任务。
// 参数：ctx 请求上下文；c 任务领取凭证。
// 返回值：nil 或租约失效、SQL 错误。
// 约束：过期租约不能复活，只更新匹配的 GENERATING 对象。
func (s *Service) renew(ctx context.Context, c Claim) error {
	now := s.Now().UTC()
	r, e := s.DB.ExecContext(ctx, "UPDATE voucher_batches SET lease_until=? WHERE id=? AND state='GENERATING' AND generation_epoch=? AND lease_until>?", now.Add(10*time.Minute), c.Batch.ID, c.Epoch, now)
	if e != nil {
		return e
	}
	n, e := r.RowsAffected()
	if e != nil {
		return e
	}
	if n != 1 {
		return ErrLeaseLost
	}
	return nil
}

type credential struct{ Code, Secret, Hash string }

// credentials 用途：在内存生成整批凭证及哈希。
// 参数：ctx 请求上下文；b 冻结批次配置。
// 返回值：凭证切片或随机源、碰撞、哈希错误。
// 约束：批内碰撞重采样有上限，不写明文文件。
func (s *Service) credentials(ctx context.Context, b Batch) ([]credential, error) {
	out := make([]credential, b.Quantity)
	seen := map[string]bool{}
	for i := range out {
		if e := ctx.Err(); e != nil {
			return nil, e
		}
		var code string
		if b.CodeMode == "SEQUENTIAL" {
			var ok bool
			code, ok = rules.Serial(b.Prefix, b.Start, b.CodeLength, i)
			if !ok {
				return nil, fault(422, "VALIDATION_FAILED")
			}
		} else {
			for attempt := 0; attempt < 5; attempt++ {
				tail, e := rules.Random(s.Random, rules.CodeAlphabet, b.CodeLength-len(b.Prefix))
				if e != nil {
					return nil, e
				}
				code = b.Prefix + tail
				if !seen[code] {
					break
				}
				code = ""
			}
			if code == "" {
				return nil, fault(409, "CODE_COLLISION")
			}
		}
		seen[code] = true
		secret, e := rules.Random(s.Random, rules.Alphabet(b.SecretFormat), b.SecretLength)
		if e != nil {
			return nil, e
		}
		hash, e := s.Hash(secret)
		if e != nil {
			return nil, e
		}
		out[i] = credential{code, secret, hash}
	}
	return out, nil
}

// ExportAAD 用途：构造绑定主体和密钥版本的导出 AAD。
// 参数：batch 批次标识或 voucher:卡券标识；version 导出密钥版本。
// 返回值：AAD 字节。
// 约束：绑定格式 v1，防止密文跨主体替换。
func ExportAAD(batch string, version int) []byte {
	return []byte(fmt.Sprintf("jst:credential:v1:batch:%s:key:%d", batch, version))
}

// writeExport 用途：将内存凭证写成私有 GCM 密文。
// 参数：b 密文主体与批次号；creds 内存中的生成凭证。
// 返回值：存储键、SHA256、字节数或加密、文件错误。
// 约束：先 fsync 再原子 rename，不生成明文磁盘文件，关闭文件资源。
func (s *Service) writeExport(b Batch, creds []credential) (string, []byte, int64, error) {
	if len(s.ExportKey) != 32 || s.KeyVersion < 1 {
		return "", nil, 0, errors.New("invalid export key")
	}
	var plain bytes.Buffer
	w := csv.NewWriter(&plain)
	w.Write([]string{"batch_no", "code", "secret"})
	for _, c := range creds {
		if e := w.Write([]string{b.Number, c.Code, c.Secret}); e != nil {
			return "", nil, 0, e
		}
	}
	w.Flush()
	if e := w.Error(); e != nil {
		return "", nil, 0, e
	}
	block, e := aes.NewCipher(s.ExportKey)
	if e != nil {
		return "", nil, 0, e
	}
	gcm, e := cipher.NewGCM(block)
	if e != nil {
		return "", nil, 0, e
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, e = rand.Read(nonce); e != nil {
		return "", nil, 0, e
	}
	sealed := gcm.Seal(nonce, nonce, plain.Bytes(), ExportAAD(b.ID, s.KeyVersion))
	clear(plain.Bytes())
	dir := filepath.Join(s.Root, "credentials")
	if e = os.MkdirAll(dir, 0700); e != nil {
		return "", nil, 0, e
	}
	f, e := os.CreateTemp(dir, ".generation-")
	if e != nil {
		return "", nil, 0, e
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, e = f.Write(sealed); e != nil {
		f.Close()
		return "", nil, 0, e
	}
	if e = f.Sync(); e != nil {
		f.Close()
		return "", nil, 0, e
	}
	if e = f.Close(); e != nil {
		return "", nil, 0, e
	}
	random := make([]byte, 16)
	if _, e = rand.Read(random); e != nil {
		return "", nil, 0, e
	}
	key := "credentials/" + hex.EncodeToString(random) + ".gcm"
	full := filepath.Join(s.Root, key)
	if e = os.Rename(tmp, full); e != nil {
		return "", nil, 0, e
	}
	d, e := os.Open(dir)
	if e == nil {
		e = d.Sync()
		d.Close()
	}
	if e != nil {
		os.Remove(full)
		return "", nil, 0, e
	}
	digest := sha256.Sum256(sealed)
	return key, digest[:], int64(len(sealed)), nil
}

// commit 用途：原子发布整批卡券及文件引用。
// 参数：ctx 请求上下文；c 任务领取凭证。
// 参数：creds 内存中的生成凭证；key 已写入的随机密文存储键。
// 参数：digest 密文 SHA256 摘要；size 密文文件字节数。
// 返回值：nil 或租约失效、唯一冲突、SQL 错误。
// 约束：锁批次校验 epoch 和 lease，全部权益、审计与 GENERATED 一起提交。
func (s *Service) commit(ctx context.Context, c Claim, creds []credential, key string, digest []byte, size int64) error {
	tx, e := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var epoch int64
	var lease sql.NullTime
	var state string
	if e = tx.QueryRowContext(ctx, "SELECT generation_epoch,lease_until,state FROM voucher_batches WHERE id=? FOR UPDATE", c.Batch.ID).Scan(&epoch, &lease, &state); e != nil {
		return e
	}
	now := s.Now().UTC()
	if epoch != c.Epoch || state != "GENERATING" || !lease.Valid || !lease.Time.After(now) {
		return ErrLeaseLost
	}
	b := c.Batch
	r, e := tx.ExecContext(ctx, "INSERT INTO files(storage_key,kind,mime_type,size_bytes,sha256,state,owner_admin_id,key_version,expires_at) VALUES(?,'CREDENTIAL_EXPORT','text/csv',?,?,'READY',?,?,?)", key, size, digest, c.Owner, s.KeyVersion, now.Add(24*time.Hour))
	if e != nil {
		return e
	}
	fileID, e := r.LastInsertId()
	if e != nil {
		return e
	}
	for _, v := range creds {
		r, e := tx.ExecContext(ctx, "INSERT INTO vouchers(batch_id,credential_file_id,code,secret_hash,valid_from,valid_until) VALUES(?,?,?,?,?,?)", b.ID, fileID, v.Code, v.Hash, b.From, b.Until)
		if e != nil {
			return e
		}
		id, e := r.LastInsertId()
		if e != nil {
			return e
		}
		for _, p := range b.Products {
			if _, e = tx.ExecContext(ctx, "INSERT INTO voucher_products(voucher_id,product_id) VALUES(?,?)", id, p); e != nil {
				return e
			}
		}
	}
	// Time can advance during a long transaction; recheck before publishing.
	if !lease.Time.After(s.Now().UTC()) {
		return ErrLeaseLost
	}
	if _, e = tx.ExecContext(ctx, "UPDATE voucher_batches SET state='GENERATED',export_file_id=?,lease_until=NULL,last_error_code=NULL,version=version+1 WHERE id=?", fileID, b.ID); e != nil {
		return e
	}
	if _, e = tx.ExecContext(ctx, "INSERT INTO audit_logs(actor_kind,action,entity_type,entity_id,request_id,diff,result) VALUES('SYSTEM','batch.generated','voucher_batch',?,?,JSON_OBJECT('quantity',?,'epoch',?),'SUCCESS')", b.ID, "generation-"+b.ID+"-"+strconv.FormatInt(c.Epoch, 10), b.Quantity, c.Epoch); e != nil {
		return e
	}
	return tx.Commit()
}

// failClaim 用途：标记仍拥有有效租约的任务失败。
// 参数：ctx 请求上下文；c 任务领取凭证。
// 参数：code 固定业务错误码。
// 返回值：nil 或 SQL 错误。
// 约束：旧 epoch 不覆盖新 worker 或已完成任务。
func (s *Service) failClaim(ctx context.Context, c Claim, code string) error {
	_, e := s.DB.ExecContext(ctx, "UPDATE voucher_batches SET state='FAILED',lease_until=NULL,last_error_code=?,version=version+1 WHERE id=? AND state='GENERATING' AND generation_epoch=? AND lease_until>?", code, c.Batch.ID, c.Epoch, s.Now().UTC())
	return e
}

// Process 用途：处理一份生成任务并维护心跳。
// 参数：ctx 请求上下文；c 任务领取凭证。
// 返回值：nil 或随机、文件、租约、SQL 错误。
// 并发：结束时取消并回收续租协程；旧 epoch 不得发布。
// 约束：提交结果不确定时先核对文件引用，避免误删已提交密文。
func (s *Service) Process(ctx context.Context, c Claim) error {
	work, cancel := context.WithCancel(ctx)
	defer cancel()
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-work.Done():
				return
			case <-ticker.C:
				if s.renew(work, c) != nil {
					cancel()
					return
				}
			}
		}
	}()
	defer func() { cancel(); wg.Wait() }()
	var last error
	for attempt := 0; attempt < 3; attempt++ {
		creds, e := s.credentials(work, c.Batch)
		if e != nil {
			last = e
			break
		}
		key, digest, size, e := s.writeExport(c.Batch, creds)
		if e != nil {
			last = e
			break
		}
		e = s.commit(work, c, creds, key, digest, size)
		if e == nil {
			return nil
		}
		// Commit acknowledgments can be lost. Only remove a file proven unreferenced.
		var count int
		check, stop := context.WithTimeout(context.Background(), 5*time.Second)
		checkErr := s.DB.QueryRowContext(check, "SELECT COUNT(*) FROM files WHERE storage_key=?", key).Scan(&count)
		stop()
		if checkErr == nil && count == 0 {
			os.Remove(filepath.Join(s.Root, key))
		}
		if checkErr == nil && count == 1 {
			return nil
		}
		last = e
		if !duplicate(e) || c.Batch.CodeMode == "SEQUENTIAL" {
			break
		}
	}
	if ctx.Err() == nil && !errors.Is(last, ErrLeaseLost) {
		code := "GENERATION_FAILED"
		if duplicate(last) {
			code = "CODE_COLLISION"
		}
		cleanup, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		if e := s.failClaim(cleanup, c, code); e != nil {
			return e
		}
	}
	return last
}

// RunOnce 用途：领取并执行一次生成任务。
// 参数：ctx 请求上下文。
// 返回值：是否领取以及处理错误。
// 约束：没有任务即返回，不启动空转协程。
func (s *Service) RunOnce(ctx context.Context) (bool, error) {
	c, ok, e := s.Claim(ctx)
	if e != nil || !ok {
		return ok, e
	}
	return true, s.Process(ctx, c)
}

// CleanupOrphans 用途：删除崩溃后无引用的过期密文。
// 参数：ctx 请求上下文。
// 返回值：nil 或查询、文件错误。
// 约束：只处理常规文件，保留 24 小时宽限及所有数据库引用。
func (s *Service) CleanupOrphans(ctx context.Context) error {
	dir := filepath.Join(s.Root, "credentials")
	entries, e := os.ReadDir(dir)
	if os.IsNotExist(e) {
		return nil
	}
	if e != nil {
		return e
	}
	for _, entry := range entries {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		info, e := entry.Info()
		if e != nil {
			return e
		}
		if !info.Mode().IsRegular() || !info.ModTime().Before(s.Now().Add(-24*time.Hour)) {
			continue
		}
		key := "credentials/" + entry.Name()
		var count int
		if e = s.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM files WHERE storage_key=?", key).Scan(&count); e != nil {
			return e
		}
		if count == 0 {
			if e = os.Remove(filepath.Join(dir, entry.Name())); e != nil && !os.IsNotExist(e) {
				return e
			}
		}
	}
	return nil
}
