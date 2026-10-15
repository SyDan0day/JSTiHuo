package main

import (
	"context"
	"database/sql"
	"log/slog"
	"strconv"
	"syscall"
)

type metricWindow struct{ Deadlocks, Transactions int64 }

// reportMetrics 用途：周期记录队列、数据库池、磁盘与死锁指标，并输出固定阈值告警。
// 参数：ctx worker 上下文；db 数据库；root 私有卷；log 日志器；previous 上次累计计数。
// 返回值：本次累计计数；只输出结构化计数，不查询正文或发送真实通知。
func reportMetrics(ctx context.Context, db *sql.DB, root string, log *slog.Logger, previous metricWindow) metricWindow {
	var pending, dead, age, expired int64
	err := db.QueryRowContext(ctx, `SELECT
		(SELECT COUNT(*) FROM outbox_events WHERE state IN ('PENDING','PROCESSING')),
		(SELECT COUNT(*) FROM outbox_events WHERE state='DEAD'),
		(SELECT COALESCE(MAX(TIMESTAMPDIFF(SECOND,created_at,UTC_TIMESTAMP(6))),0) FROM outbox_events WHERE state IN ('PENDING','PROCESSING') AND next_attempt_at<=UTC_TIMESTAMP(6)),
		(SELECT COUNT(*) FROM integration_jobs WHERE state='RUNNING' AND lease_until<=UTC_TIMESTAMP(6))`).Scan(&pending, &dead, &age, &expired)
	if err != nil {
		if ctx.Err() == nil {
			log.Error("operational metrics unavailable")
		}
		return previous
	}
	stats := db.Stats()
	log.Info("operational metrics", "outbox_pending", pending, "outbox_dead", dead, "outbox_oldest_seconds", age, "expired_job_leases", expired, "db_in_use", stats.InUse, "db_idle", stats.Idle, "db_wait_count", stats.WaitCount, "db_wait_ms", stats.WaitDuration.Milliseconds())
	if age > 15*60 {
		log.Warn("outbox backlog alert", "oldest_seconds", age)
	}
	var disk syscall.Statfs_t
	if syscall.Statfs(root, &disk) == nil && disk.Blocks > 0 {
		used := 100 * (1 - float64(disk.Bavail)/float64(disk.Blocks))
		log.Info("private volume metrics", "used_percent", used)
		if used > 80 {
			log.Warn("private volume capacity alert", "used_percent", used)
		}
	} else {
		log.Error("private volume metrics unavailable")
	}
	current, err := transactionMetrics(ctx, db)
	if err != nil {
		if ctx.Err() == nil {
			log.Error("database transaction metrics unavailable")
		}
		return previous
	}
	delta := current.Transactions - previous.Transactions
	deadlocks := current.Deadlocks - previous.Deadlocks
	if delta >= 100 && deadlocks > 0 && float64(deadlocks)/float64(delta) > 0.01 {
		log.Warn("database deadlock ratio alert", "transactions", delta, "deadlocks", deadlocks)
	}
	log.Info("database transaction metrics", "transactions", current.Transactions, "deadlocks", current.Deadlocks)
	return current
}

// transactionMetrics 用途：读取 MySQL 累计死锁与显式事务计数。
// 参数：ctx 上下文；db 应用数据库连接。
// 返回值：累计计数或查询、解析错误，不读取 SQL 文本或会话输入。
func transactionMetrics(ctx context.Context, db *sql.DB) (metricWindow, error) {
	out := metricWindow{}
	rows, err := db.QueryContext(ctx, "SHOW GLOBAL STATUS WHERE Variable_name IN ('Innodb_deadlocks','Com_commit','Com_rollback')")
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var name, value string
		if err = rows.Scan(&name, &value); err != nil {
			return out, err
		}
		count, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return out, err
		}
		if name == "Innodb_deadlocks" {
			out.Deadlocks = count
		} else {
			out.Transactions += count
		}
	}
	return out, rows.Err()
}
