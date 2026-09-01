package core

import (
	"context"
	"database/sql"
	"math"
	"strconv"
	"strings"
	"time"
)

func payableSettlementStatus(applied, remaining float64) string {
	if remaining <= 0.000001 {
		return "paid"
	}
	if applied > 0 {
		return "partially_paid"
	}
	return "approved"
}

func scheduleSettlementStatus(remaining float64) string {
	if remaining <= 0.000001 {
		return "paid"
	}
	return "partially_paid"
}

func receivableSettlementStatus(received, remaining float64) string {
	if remaining <= 0.000001 {
		return "received"
	}
	if received > 0 {
		return "partially_received"
	}
	return "approved"
}

func receiptScheduleSettlementStatus(remaining float64) string {
	if remaining <= 0.000001 {
		return "received"
	}
	return "partially_received"
}

func clientAdvanceReceiptStatusAfterReceipt(total, received, settled float64) string {
	if settled >= total && total > 0 {
		return "settled"
	}
	if settled > 0 {
		return "partially_settled"
	}
	if received >= total && total > 0 {
		return "received"
	}
	return "partially_received"
}

func queryTx(ctx context.Context, tx *sql.Tx, query string, args ...any) ([]Record, error) {
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRows(rows)
}

func number(value any) float64 {
	switch v := value.(type) {
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case float64:
		return v
	case float32:
		return float64(v)
	case string:
		out, _ := strconv.ParseFloat(v, 64)
		if math.IsNaN(out) {
			return 0
		}
		return out
	default:
		return 0
	}
}

func boolValue(value any) bool {
	switch v := value.(type) {
	case bool:
		return v
	case string:
		out, _ := strconv.ParseBool(v)
		return out
	default:
		return false
	}
}

func stringsJoin(values []string) string {
	return strings.Join(values, "; ")
}

func timeNowUnix() int64 {
	return time.Now().Unix()
}
