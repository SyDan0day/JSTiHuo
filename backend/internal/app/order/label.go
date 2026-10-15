package order

import (
	"context"
	"database/sql"
	"encoding/json"
)

// checkShippingLabel 用途：阻止未核实外部面单被人工发货绕过。
// 参数：ctx 上下文；tx 已锁订单的事务；id 订单；in 实际发货命令。
// 返回值：待确认任务 ID、空值或状态、版本、数据库错误。
// 约束：READY 只能使用同版本、同承运商和同单号，UNKNOWN 必须先人工核对。
func checkShippingLabel(ctx context.Context, tx *sql.Tx, id string, in ShipInput) (string, error) {
	rows, err := tx.QueryContext(ctx, `SELECT id,state,options,result_summary FROM integration_jobs WHERE order_id=? AND kind='LABEL' AND state IN ('PENDING','RUNNING','READY','UNKNOWN') ORDER BY id FOR UPDATE`, id)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	jobID := ""
	for rows.Next() {
		var current, state string
		var options, summary []byte
		if err = rows.Scan(&current, &state, &options, &summary); err != nil {
			return "", err
		}
		var opts struct {
			Version   int64 `json:"expected_order_version"`
			Confirmed bool  `json:"confirmed"`
		}
		var result struct {
			CarrierID string `json:"carrier_id"`
			Tracking  string `json:"tracking_no"`
		}
		if json.Unmarshal(options, &opts) != nil || json.Unmarshal(summary, &result) != nil {
			return "", fault(409, "CONSISTENCY_CONFLICT")
		}
		if state != "READY" || opts.Confirmed || opts.Version != in.Version || result.CarrierID != in.CarrierID || result.Tracking != in.Tracking || jobID != "" {
			return "", fault(409, "INTEGRATION_PENDING")
		}
		jobID = current
	}
	return jobID, rows.Err()
}
