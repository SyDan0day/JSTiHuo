package main

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"os"
	"strconv"
	"strings"

	"jstihuo/internal/app/catalog"
	"jstihuo/internal/app/identity"
	"jstihuo/internal/app/operations"
	"jstihuo/internal/app/order"
	"jstihuo/internal/app/redemption"
	"jstihuo/internal/app/voucher"
	"jstihuo/internal/bootstrap"
	"jstihuo/internal/domain/calendar"
	crypt "jstihuo/internal/infra/crypto"
)

// maintainCalendar 用途：启动或上海跨日后补建预约窗口，不改变既有占用。
// 参数：ctx 工作上下文；redeem 日历服务；lastDay 最近成功维护的上海日期。
// 返回值：本次成功日期或原日期与错误；失败允许下次分钟维护重试。
func maintainCalendar(ctx context.Context, redeem *redemption.Service, lastDay string) (string, error) {
	today := calendar.Day(redeem.Now()).Format("2006-01-02")
	if today == lastDay {
		return lastDay, nil
	}
	if _, err := redeem.CalendarDays(ctx, ""); err != nil {
		return lastDay, err
	}
	return today, nil
}

// retentionDays 用途：读取订单隐私保留天数，拒绝低于既定政策的配置。
// 参数：无，使用 ORDER_PII_RETENTION_DAYS，默认 180 天。
// 返回值：保留天数或固定配置错误。
func retentionDays() (int, error) {
	raw := os.Getenv("ORDER_PII_RETENTION_DAYS")
	if raw == "" {
		return 180, nil
	}
	days, err := strconv.Atoi(raw)
	if err != nil || days < 180 || days > 36500 {
		return 0, errors.New("RETENTION_CONFIGURATION_INVALID")
	}
	return days, nil
}

// workerKey 用途：读取启动阶段已校验的 HMAC 密钥。
// 参数：name 对应密钥文件的环境变量名称。
// 返回值：解码密钥或文件、编码错误，不输出密钥。
func workerKey(name string) ([]byte, error) {
	raw, err := bootstrap.ReadSecret(name)
	if err != nil {
		return nil, err
	}
	return base64.StdEncoding.DecodeString(strings.TrimSpace(string(raw)))
}

// loadOperations 用途：为文件任务装配与 API 相同的业务服务和密钥环。
// 参数：db 数据库；root 私有文件根目录；requestKey 请求密钥；vouchers 凭证服务。
// 返回值：运营任务服务或依赖初始化错误，不发送真实通知。
func loadOperations(db *sql.DB, root string, requestKey []byte, vouchers *voucher.Service) (*operations.Service, error) {
	lookup, err := workerKey("LOOKUP_HMAC_KEY_FILE")
	if err != nil {
		return nil, err
	}
	challenge, err := workerKey("CHALLENGE_HMAC_KEY_FILE")
	if err != nil {
		return nil, err
	}
	auth, err := identity.New(db, lookup, challenge, requestKey)
	if err != nil {
		return nil, err
	}
	raw, err := bootstrap.ReadSecret("PII_KEYRING_FILE")
	if err != nil {
		return nil, err
	}
	pii, err := crypt.ParseKeyring(raw)
	if err != nil {
		return nil, err
	}
	cat := catalog.New(db, root, requestKey)
	redeem, err := redemption.New(db, auth, cat, pii, requestKey, lookup)
	if err != nil {
		return nil, err
	}
	redeem.Test = os.Getenv("APP_ENV") == "test"
	svc := operations.New(db, auth, pii, requestKey)
	svc.FileRoot = root
	svc.FileKeys = crypt.Keyring{Active: vouchers.KeyVersion, Keys: vouchers.ExportKeys}
	svc.Orders = order.New(db, auth, redeem)
	svc.Vouchers = vouchers
	return svc, nil
}
