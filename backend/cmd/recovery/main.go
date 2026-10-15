package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"jstihuo/internal/app/operations"
	"jstihuo/internal/app/redemption"
	"jstihuo/internal/bootstrap"
	"jstihuo/internal/infra/backup"
	crypt "jstihuo/internal/infra/crypto"
	"jstihuo/migrations"
)

var imageFile = regexp.MustCompile(`^[a-f0-9]{32}\.png$`)

// main 用途：提供离线认证备份、恢复解包与恢复后安全收敛命令。
// 参数：进程参数选择 keygen、pack、unpack、unpack-sql、sanitize、verify-db。
// 返回值：无；成功返回零，失败只输出固定错误，不打印秘密或 SQL。
func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "recovery failed:", err)
		os.Exit(1)
	}
}

// run 用途：解析受控运维子命令，不接受任意数据库 SQL。
// 参数：args 进程参数。
// 返回值：nil 或固定配置、认证、目标、数据库错误。
func run(args []string) error {
	if len(args) == 0 {
		return errors.New("COMMAND_REQUIRED")
	}
	flags := flag.NewFlagSet("recovery", flag.ContinueOnError)
	keyPath := flags.String("key-file", "", "独立 Base64 备份密钥文件")
	root := flags.String("files-root", "", "私有文件根目录，仅备份公开图片")
	target := flags.String("target-dir", "", "必须不存在的恢复目录")
	dsn := flags.String("dsn-file", "", "数据库 DSN 文件，不接受命令行明文")
	restored := flags.String("restored-database", "", "sanitize 必须明确给出已恢复目标库名")
	piiPath := flags.String("pii-keyring-file", "", "恢复验收使用分开保管的 PII 密钥环校验全部密文")
	if flags.Parse(args[1:]) != nil || flags.NArg() != 0 {
		return errors.New("INVALID_ARGUMENT")
	}
	switch args[0] {
	case "keygen":
		return generateKey(*keyPath)
	case "sanitize", "verify-db":
		return databaseCommand(args[0], *dsn, *restored, *piiPath)
	case "pack", "unpack", "unpack-sql":
		key, err := readKey(*keyPath)
		if err != nil {
			return err
		}
		defer clear(key)
		if args[0] == "pack" {
			return packArchive(key, *root)
		}
		if args[0] == "unpack-sql" {
			archive, err := readArchive(key)
			if err != nil {
				return err
			}
			defer archive.Clear()
			_, err = os.Stdout.Write(archive.SQL)
			return err
		}
		return unpackArchive(key, *target)
	default:
		return errors.New("UNKNOWN_COMMAND")
	}
}

// generateKey 用途：一次性生成独立备份密钥，拒绝覆盖现有文件。
// 参数：path 新密钥文件路径，其父目录由运维预建并独立保管。
// 返回值：nil 或目标、随机源、文件写入错误，不输出密钥。
func generateKey(path string) error {
	if path == "" {
		return errors.New("KEY_FILE_REQUIRED")
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return errors.New("RANDOM_UNAVAILABLE")
	}
	defer clear(key)
	return writeExclusive(path, []byte(base64.StdEncoding.EncodeToString(key)+"\n"))
}

// readKey 用途：读取独立 32 字节备份密钥。
// 参数：path 受限密钥文件。
// 返回值：密钥或固定错误，拒绝宽松权限与符号链接。
func readKey(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 || info.Size() > 1024 {
		return nil, errors.New("KEY_FILE_INVALID")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.New("KEY_FILE_UNREADABLE")
	}
	defer clear(raw)
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(raw)))
	if err != nil || len(key) != 32 {
		return nil, errors.New("KEY_INVALID")
	}
	return key, nil
}

// packArchive 用途：从 stdin SQL 和白名单图片生成认证密文到 stdout。
// 参数：key 独立备份密钥；root 私有文件根目录。
// 返回值：nil 或有界读取、目录、加密、写入错误；不产生明文备份文件。
func packArchive(key []byte, root string) error {
	sqlBytes, err := io.ReadAll(io.LimitReader(os.Stdin, backup.MaxBytes+1))
	if err != nil || len(sqlBytes) > backup.MaxBytes {
		return errors.New("SQL_TOO_LARGE")
	}
	archive := backup.Archive{SQL: sqlBytes, Images: map[string][]byte{}}
	defer archive.Clear()
	if root != "" {
		directory, err := os.OpenRoot(root)
		if err != nil {
			return errors.New("FILES_ROOT_INVALID")
		}
		defer directory.Close()
		entries, err := os.ReadDir(root)
		if err != nil {
			return errors.New("FILES_ROOT_UNREADABLE")
		}
		total := len(sqlBytes)
		for _, entry := range entries {
			if !imageFile.MatchString(entry.Name()) {
				continue
			}
			info, err := directory.Lstat(entry.Name())
			if err != nil || !info.Mode().IsRegular() || info.Size() > 10000000 {
				return errors.New("IMAGE_INVALID")
			}
			data, err := directory.ReadFile(entry.Name())
			if err != nil {
				return errors.New("IMAGE_UNREADABLE")
			}
			total += len(data)
			if total > backup.MaxBytes {
				clear(data)
				return errors.New("BACKUP_TOO_LARGE")
			}
			archive.Images["images/"+entry.Name()] = data
		}
	}
	sealed, err := backup.Pack(key, archive)
	if err != nil {
		return err
	}
	_, err = os.Stdout.Write(sealed)
	return err
}

// unpackArchive 用途：完整认证后在新建目录中解包，不覆盖任何既有目标。
// 参数：key 对应备份密钥；target 必须不存在的恢复目录。
// 返回值：nil 或认证、路径、已存在、写入错误；失败清理本次暂存目录。
func unpackArchive(key []byte, target string) error {
	if target == "" {
		return errors.New("TARGET_REQUIRED")
	}
	if _, err := os.Lstat(target); !os.IsNotExist(err) {
		return errors.New("TARGET_EXISTS")
	}
	archive, err := readArchive(key)
	if err != nil {
		return err
	}
	defer archive.Clear()
	staging, err := os.MkdirTemp(filepath.Dir(target), ".jst-restore-")
	if err != nil {
		return errors.New("STAGING_UNAVAILABLE")
	}
	defer os.RemoveAll(staging)
	if err = writeExclusive(filepath.Join(staging, "db.sql"), archive.SQL); err != nil {
		return err
	}
	if err = os.Mkdir(filepath.Join(staging, "images"), 0700); err != nil {
		return errors.New("IMAGES_DIRECTORY_FAILED")
	}
	for name, data := range archive.Images {
		if err = writeExclusive(filepath.Join(staging, filepath.FromSlash(name)), data); err != nil {
			return err
		}
	}
	if _, err = os.Lstat(target); !os.IsNotExist(err) {
		return errors.New("TARGET_EXISTS")
	}
	if err = os.Rename(staging, target); err != nil {
		return errors.New("TARGET_PUBLISH_FAILED")
	}
	return nil
}

// readArchive 用途：有界读取 stdin 并完整认证归档，供目录或管道恢复共用。
// 参数：key 对应备份密钥。
// 返回值：已认证内存归档或读取、认证、路径错误。
func readArchive(key []byte) (backup.Archive, error) {
	sealed, err := io.ReadAll(io.LimitReader(os.Stdin, backup.MaxBytes+129))
	if err != nil {
		return backup.Archive{}, errors.New("BACKUP_UNREADABLE")
	}
	return backup.Unpack(key, sealed)
}

// writeExclusive 用途：以最小权限独占创建并刷盘运维文件。
// 参数：path 新文件；data 文件内容。
// 返回值：nil 或固定已存在、写入、同步错误。
func writeExclusive(path string, data []byte) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return errors.New("FILE_EXISTS_OR_UNWRITABLE")
	}
	if _, err = file.Write(data); err != nil {
		file.Close()
		return errors.New("FILE_WRITE_FAILED")
	}
	if err = file.Sync(); err != nil {
		file.Close()
		return errors.New("FILE_SYNC_FAILED")
	}
	if err = file.Close(); err != nil {
		return errors.New("FILE_CLOSE_FAILED")
	}
	return nil
}

// databaseCommand 用途：检查迁移和账本，或在恢复后撤销会话并阻止不确定外部重发。
// 参数：command 为 sanitize 或 verify-db；path 为目标 DSN 文件；restored 为明确恢复库名。
// 参数：piiPath 为可选的独立 PII 密钥环，提供时校验全部收件人与反馈密文。
// 返回值：nil 或固定数据库、迁移、漂移错误，不自动修复账本。
func databaseCommand(command, path, restored, piiPath string) error {
	if command == "sanitize" && !regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]{0,63}$`).MatchString(restored) {
		return errors.New("RESTORED_DATABASE_REQUIRED")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return errors.New("DSN_FILE_UNREADABLE")
	}
	defer clear(raw)
	db, err := bootstrap.OpenDB(strings.TrimSpace(string(raw)))
	if err != nil {
		return errors.New("DB_UNAVAILABLE")
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if command == "sanitize" {
		var database string
		if db.QueryRowContext(ctx, "SELECT DATABASE()").Scan(&database) != nil || database != restored {
			return errors.New("RESTORED_DATABASE_MISMATCH")
		}
	}
	if err = migrations.Check(ctx, db); err != nil {
		return errors.New("MIGRATION_MISMATCH")
	}
	if command == "sanitize" {
		if err = sanitize(ctx, db); err != nil {
			return errors.New("SANITIZE_FAILED")
		}
	}
	service := &operations.Service{DB: db, Now: time.Now}
	result, err := service.Reconcile(ctx)
	if err != nil {
		return errors.New("RECONCILIATION_FAILED")
	}
	var checked *int
	if piiPath != "" {
		count, err := verifyPII(ctx, db, piiPath)
		if err != nil {
			return err
		}
		checked = &count
	}
	output := struct {
		operations.Reconcile
		PIIChecked *int `json:"pii_records_checked,omitempty"`
	}{result, checked}
	if err = json.NewEncoder(os.Stdout).Encode(output); err != nil {
		return err
	}
	if !result.Balanced {
		return errors.New("INVENTORY_DRIFT")
	}
	return nil
}

// verifyPII 用途：使用独立保管的版本密钥环验证恢复数据可解密且为 JSON。
// 参数：ctx 上下文；db 恢复数据库；path PII 密钥环文件。
// 返回值：已校验条数或固定密钥、查询、认证错误；不输出解密内容。
func verifyPII(ctx context.Context, db *sql.DB, path string) (int, error) {
	raw, err := os.ReadFile(path)
	if err != nil || len(raw) > 128*1024 {
		return 0, errors.New("PII_KEYRING_INVALID")
	}
	defer clear(raw)
	keys, err := crypt.ParseKeyring(raw)
	if err != nil {
		return 0, errors.New("PII_KEYRING_INVALID")
	}
	rows, err := db.QueryContext(ctx, `SELECT 'ORDER',CAST(order_no AS BINARY),recipient_cipher,pii_key_version FROM orders UNION ALL SELECT 'FEEDBACK',CAST(id AS BINARY),contact_cipher,pii_key_version FROM feedback`)
	if err != nil {
		return 0, errors.New("PII_QUERY_FAILED")
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var kind, reference string
		var encrypted []byte
		var version int
		if rows.Scan(&kind, &reference, &encrypted, &version) != nil {
			return count, errors.New("PII_QUERY_FAILED")
		}
		aad := []byte("feedback:" + reference)
		if kind == "ORDER" {
			aad = redemption.OrderAAD(reference)
		}
		plain, err := keys.Decrypt(encrypted, aad, version)
		if err != nil {
			return count, errors.New("PII_VERIFICATION_FAILED")
		}
		valid := json.Valid(plain)
		clear(plain)
		if !valid {
			return count, errors.New("PII_VERIFICATION_FAILED")
		}
		count++
	}
	if rows.Err() != nil {
		return count, errors.New("PII_QUERY_FAILED")
	}
	return count, nil
}

// sanitize 用途：在恢复目标事务中撤销会话、关闭全部私有导出并隔离不确定集成。
// 参数：ctx 运维上下文；db 已通过迁移检查的隔离恢复库。
// 返回值：nil 或数据库错误，失败整体回滚，不改库存与订单状态。
func sanitize(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	statements := []string{
		"DELETE FROM challenges", "DELETE FROM sessions",
		"UPDATE files SET state='EXPIRED',expires_at=UTC_TIMESTAMP(6),version=version+1 WHERE kind<>'IMAGE' AND state IN ('STAGED','READY')",
		"UPDATE voucher_batches SET state='SEALED',lease_until=NULL,version=version+1 WHERE state='GENERATED'",
		`UPDATE integration_jobs SET state='UNKNOWN',lease_until=NULL,version=version+1 WHERE kind='LABEL' AND state IN ('PENDING','RUNNING','READY') AND COALESCE(JSON_UNQUOTE(JSON_EXTRACT(options,'$.confirmed')),'false')<>'true'`,
		`INSERT INTO notifications(outbox_id,order_id,channel,template_key,state,last_error_code)
		 SELECT id,JSON_UNQUOTE(JSON_EXTRACT(payload,'$.order_id')),'MOCK',event_type,'UNKNOWN','RESTORE_RECONCILIATION_REQUIRED' FROM outbox_events
		 WHERE state IN ('PENDING','PROCESSING') AND event_type IN ('order.created','order.updated','order.cancelled','shipment.created','order.completed','after_sale.created','after_sale.decided','replacement.shipped')
		 ON DUPLICATE KEY UPDATE outbox_id=notifications.outbox_id`,
		`UPDATE notifications n JOIN outbox_events o ON o.id=n.outbox_id SET n.state='UNKNOWN',n.last_error_code='RESTORE_RECONCILIATION_REQUIRED',n.version=n.version+1 WHERE o.state IN ('PENDING','PROCESSING') AND n.state IN ('PENDING','FAILED','UNKNOWN')`,
		`UPDATE outbox_events o JOIN notifications n ON n.outbox_id=o.id SET o.state='DEAD',o.lease_until=NULL,o.last_error_code='RESTORE_RECONCILIATION_REQUIRED',o.version=o.version+1 WHERE n.state='UNKNOWN' AND o.state IN ('PENDING','PROCESSING')`,
		`INSERT INTO audit_logs(actor_kind,action,entity_type,request_id,diff,result) VALUES('SYSTEM','recovery.sanitized','config','offline-recovery',JSON_OBJECT(),'SUCCESS')`,
	}
	for _, statement := range statements {
		if _, err = tx.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	return tx.Commit()
}
