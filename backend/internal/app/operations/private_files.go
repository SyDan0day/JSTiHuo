package operations

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"jstihuo/internal/app/identity"
	rules "jstihuo/internal/domain/voucher"
)

type File struct {
	ID        string     `json:"id"`
	Kind      string     `json:"kind"`
	State     string     `json:"state"`
	MIME      string     `json:"mime_type"`
	Size      int64      `json:"size_bytes"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

type storedFile struct {
	File
	Storage    string
	Owner      int64
	KeyVersion int
	Expires    time.Time
	SHA        []byte
}

var privateStorage = regexp.MustCompile(`^(imports|reports)/[a-f0-9]{32}\.gcm$`)

// privateAAD 用途：把私有文件的用途、存储键和密钥版本绑定到认证密文。
// 参数：kind 文件用途；storage 服务器生成存储键；version 密钥版本。
// 返回值：确定性 AAD，不包含文件正文。
func privateAAD(kind, storage string, version int) []byte {
	return []byte(fmt.Sprintf("jst:private:%s:%s:%d", kind, storage, version))
}

// writePrivate 用途：原子写入私有加密文件，不产生明文临时文件。
// 参数：kind IMPORT 或 REPORT；plain 有界内存正文。
// 返回值：存储元数据或加密、文件系统错误，调用方负责登记数据库。
func (s *Service) writePrivate(kind string, plain []byte) (storedFile, error) {
	directory := "reports"
	if kind == "IMPORT" {
		directory = "imports"
	}
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return storedFile{}, err
	}
	storage := directory + "/" + hex.EncodeToString(random[:]) + ".gcm"
	sealed, version, err := s.FileKeys.Encrypt(plain, privateAAD(kind, storage, s.FileKeys.Active))
	if err != nil {
		return storedFile{}, err
	}
	root := filepath.Join(s.FileRoot, directory)
	if err = os.MkdirAll(root, 0700); err != nil {
		return storedFile{}, err
	}
	file, err := os.CreateTemp(root, ".staged-")
	if err != nil {
		return storedFile{}, err
	}
	defer os.Remove(file.Name())
	if err = file.Chmod(0600); err == nil {
		_, err = file.Write(sealed)
	}
	if err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return storedFile{}, err
	}
	if closeErr != nil {
		return storedFile{}, closeErr
	}
	if err = os.Rename(file.Name(), filepath.Join(s.FileRoot, storage)); err != nil {
		return storedFile{}, err
	}
	dir, err := os.Open(root)
	if err != nil {
		return storedFile{}, err
	}
	err = dir.Sync()
	closeErr = dir.Close()
	if err != nil {
		return storedFile{}, err
	}
	if closeErr != nil {
		return storedFile{}, closeErr
	}
	hash := sha256.Sum256(plain)
	return storedFile{File: File{Kind: kind, State: "READY", MIME: "text/csv", Size: int64(len(plain))}, Storage: storage, KeyVersion: version, SHA: hash[:], Expires: s.Now().UTC().Add(24 * time.Hour)}, nil
}

// insertPrivate 用途：在业务事务中登记已落盘密文。
// 参数：ctx 上下文；tx 事务；file 私有文件元数据；owner 创建管理员。
// 返回值：文件回执或数据库错误。
func insertPrivate(ctx context.Context, tx *sql.Tx, file storedFile, owner int64) (File, error) {
	r, err := tx.ExecContext(ctx, `INSERT INTO files(storage_key,kind,mime_type,size_bytes,sha256,state,owner_admin_id,key_version,expires_at)
		VALUES(?,?,?,?,?,?,?,?,?)`, file.Storage, file.Kind, file.MIME, file.Size, file.SHA, file.State, owner, file.KeyVersion, file.Expires)
	if err != nil {
		return File{}, err
	}
	id, err := r.LastInsertId()
	file.File.ID = fmt.Sprint(id)
	return file.File, err
}

// loadPrivate 用途：按 ID 读取私有文件元数据。
// 参数：ctx 上下文；q 数据库或事务；id 文件 ID；lock 是否排他锁定。
// 返回值：私有文件或不存在、数据库错误。
func loadPrivate(ctx context.Context, q queryer, id string, lock bool) (storedFile, error) {
	var out storedFile
	var owner sql.NullInt64
	query := "SELECT id,kind,state,mime_type,size_bytes,storage_key,owner_admin_id,key_version,expires_at,sha256 FROM files WHERE id=? AND deleted_at IS NULL AND kind IN ('IMPORT','REPORT')"
	if lock {
		query += " FOR UPDATE"
	}
	err := q.QueryRowContext(ctx, query, id).Scan(&out.ID, &out.Kind, &out.State, &out.MIME, &out.Size, &out.Storage, &owner, &out.KeyVersion, &out.Expires, &out.SHA)
	if err == sql.ErrNoRows {
		err = fault(404, "NOT_FOUND")
	}
	if err == nil && !owner.Valid {
		err = fault(403, "FORBIDDEN")
	}
	out.Owner = owner.Int64
	return out, err
}

// readPrivate 用途：校验用途、期限与存储键后在内存解密文件。
// 参数：file 数据库元数据；kind 期望文件用途。
// 返回值：明文缓冲；过期、完整性、密钥或文件错误，调用方须清零缓冲。
func (s *Service) readPrivate(file storedFile, kind string) ([]byte, error) {
	if file.Kind != kind || (file.State != "READY" && file.State != "STAGED") || !s.Now().Before(file.Expires) {
		return nil, fault(410, "EXPORT_EXPIRED")
	}
	prefix := "reports/"
	if kind == "IMPORT" {
		prefix = "imports/"
	}
	if !privateStorage.MatchString(file.Storage) || !strings.HasPrefix(file.Storage, prefix) || file.Size < 1 || file.Size > 10*1024*1024 {
		return nil, fault(503, "FILE_UNAVAILABLE")
	}
	reader, err := os.Open(filepath.Join(s.FileRoot, file.Storage))
	if err != nil {
		return nil, fault(503, "FILE_UNAVAILABLE")
	}
	defer reader.Close()
	data, err := io.ReadAll(io.LimitReader(reader, 10*1024*1024+65))
	if err != nil || len(data) > 10*1024*1024+64 {
		return nil, fault(503, "FILE_UNAVAILABLE")
	}
	plain, err := s.FileKeys.Decrypt(data, privateAAD(kind, file.Storage, file.KeyVersion), file.KeyVersion)
	if err != nil {
		return nil, fault(503, "FILE_UNAVAILABLE")
	}
	hash := sha256.Sum256(plain)
	if int64(len(plain)) != file.Size || !hmac.Equal(hash[:], file.SHA) {
		clear(plain)
		return nil, fault(503, "FILE_UNAVAILABLE")
	}
	return plain, nil
}

// UploadImport 用途：把 UTF-8 CSV 作为短期 AEAD 密文暂存。
// 参数：ctx 上下文；a 管理员；raw 内存文件；key 幂等键；rid 请求标识。
// 返回值：不含原文件名与正文的文件 JSON；非法输入或写入错误。
// 约束：没有创建导入任务前不会生成卡券、发货或暴露原始文件下载。
func (s *Service) UploadImport(ctx context.Context, a identity.Actor, raw []byte, key, rid string) (json.RawMessage, error) {
	if !a.Can("file.write") {
		return nil, fault(403, "FORBIDDEN")
	}
	if !requestKeyPattern.MatchString(key) || len(raw) == 0 || len(raw) > 5*1024*1024 || !utf8.Valid(raw) || bytes.IndexByte(raw, 0) >= 0 {
		return nil, fault(422, "INVALID_ARGUMENT")
	}
	file, err := s.writePrivate("IMPORT", raw)
	if err != nil {
		return nil, err
	}
	file.State = "STAGED"
	command := mutation{Operation: "uploadFile", Permission: "file.write", Entity: "file", Key: key, RequestID: rid, Status: 201, Body: "IMPORT:" + hex.EncodeToString(file.SHA)}
	result, err := s.write(ctx, a, command, func(tx *sql.Tx) (any, string, error) {
		out, err := insertPrivate(ctx, tx, file, a.AdminID)
		return out, out.ID, err
	})
	s.cleanupUnreferenced(ctx, file.Storage)
	return result, err
}

// cleanupUnreferenced 用途：仅在数据库证实没有引用时清理本次密文。
// 参数：ctx 上下文；storage 本次写入的服务器存储键。
// 返回值：无；提交结果不确定或数据库不可用时保留文件待维护任务核对。
func (s *Service) cleanupUnreferenced(ctx context.Context, storage string) {
	var count int
	if s.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM files WHERE storage_key=?", storage).Scan(&count) == nil && count == 0 {
		os.Remove(filepath.Join(s.FileRoot, storage))
	}
}

// DownloadReport 用途：授权下载脱敏报表，持有文件锁防止过期清理竞争。
// 参数：ctx 上下文；a 管理员；id 文件 ID；rid 请求标识；stream 有界输出回调。
// 返回值：权限、期限、文件、审计或输出错误，不允许下载原始导入文件。
func (s *Service) DownloadReport(ctx context.Context, a identity.Actor, id, rid string, stream func(io.Reader, string) error) error {
	if !rules.ValidID(id) {
		return fault(422, "INVALID_ARGUMENT")
	}
	tx, err := s.DB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = identity.AuthorizeAdmin(ctx, tx, a, "file.read"); err != nil {
		return err
	}
	file, err := loadPrivate(ctx, tx, id, true)
	if err != nil {
		return err
	}
	if file.Kind != "REPORT" || (file.Owner != a.AdminID && !a.Can("file.export")) {
		return fault(403, "FORBIDDEN")
	}
	if file.Owner != a.AdminID {
		if err = identity.AuthorizeAdmin(ctx, tx, a, "file.export"); err != nil {
			return err
		}
	}
	plain, err := s.readPrivate(file, "REPORT")
	if err != nil {
		return err
	}
	defer clear(plain)
	_, err = s.DB.ExecContext(ctx, `INSERT INTO audit_logs(actor_admin_id,actor_kind,action,entity_type,entity_id,request_id,diff,result)
		VALUES(?,'ADMIN','downloadFile','file',?,?,JSON_OBJECT(),'AUTHORIZED')`, a.AdminID, id, rid)
	if err != nil {
		return err
	}
	if err = stream(bytes.NewReader(plain), "report-"+id+".csv"); err != nil {
		return err
	}
	return tx.Commit()
}

type queryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}
