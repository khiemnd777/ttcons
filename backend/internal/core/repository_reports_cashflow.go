package core

import (
	"context"
	"fmt"
)

func (r *Repository) CashflowReport(ctx context.Context, req CashflowReportRequest, userID string) (Record, error) {
	scopeSQL, scopeArgs, err := r.reportFinanceAccessFilter(ctx, userID, "p", 1)
	if err != nil {
		return nil, err
	}
	statusFilter := cashflowStatusFilter(req.Status)

	openingArgs := append([]any{}, scopeArgs...)
	openingFromIdx := len(openingArgs) + 1
	openingArgs = append(openingArgs, req.FromDate)
	openingProjectFilter := cashflowProjectFilter(&openingArgs, req.ProjectID)
	openingRows, err := r.Query(ctx, cashflowRowsCTE(
		scopeSQL,
		openingProjectFilter,
		fmt.Sprintf(" AND %s < $%d::date", cashflowOccurredAtSQL(), openingFromIdx),
		statusFilter,
	)+`
		SELECT COALESCE(SUM(net_amount), 0) AS opening_balance
		FROM cashflow_rows
	`, openingArgs...)
	if err != nil {
		return nil, err
	}
	openingBalance := 0.0
	if len(openingRows) > 0 {
		openingBalance = number(openingRows[0]["opening_balance"])
	}

	rangeArgs := append([]any{}, scopeArgs...)
	fromIdx := len(rangeArgs) + 1
	rangeArgs = append(rangeArgs, req.FromDate)
	toIdx := len(rangeArgs) + 1
	rangeArgs = append(rangeArgs, req.ToDate)
	rangeProjectFilter := cashflowProjectFilter(&rangeArgs, req.ProjectID)
	rangeDateFilter := fmt.Sprintf(" AND %s >= $%d::date AND %s < ($%d::date + INTERVAL '1 day')", cashflowOccurredAtSQL(), fromIdx, cashflowOccurredAtSQL(), toIdx)
	rangeCTE := cashflowRowsCTE(scopeSQL, rangeProjectFilter, rangeDateFilter, statusFilter)

	summaryRows, err := r.Query(ctx, rangeCTE+`
		SELECT
			COUNT(*) AS payment_count,
			COUNT(*) FILTER (WHERE direction='unclassified') AS unclassified_count,
			COUNT(*) FILTER (WHERE direction='non_cash') AS non_cash_count,
			COALESCE(SUM(inflow_amount), 0) AS total_inflow,
			COALESCE(SUM(outflow_amount), 0) AS total_outflow,
			COALESCE(SUM(net_amount), 0) AS net_flow
		FROM cashflow_rows
	`, rangeArgs...)
	if err != nil {
		return nil, err
	}
	summary := Record{
		"from":                  req.FromDate,
		"to":                    req.ToDate,
		"group_by":              req.GroupBy,
		"status":                req.Status,
		"project_id":            req.ProjectID,
		"opening_balance":       openingBalance,
		"payment_count":         0,
		"unclassified_count":    0,
		"non_cash_count":        0,
		"total_inflow":          0,
		"total_outflow":         0,
		"net_flow":              0,
		"closing_balance":       openingBalance,
		"pending_payment_count": 0,
		"pending_payment_total": 0,
	}
	if len(summaryRows) > 0 {
		row := summaryRows[0]
		netFlow := number(row["net_flow"])
		summary["payment_count"] = row["payment_count"]
		summary["unclassified_count"] = row["unclassified_count"]
		summary["non_cash_count"] = row["non_cash_count"]
		summary["total_inflow"] = row["total_inflow"]
		summary["total_outflow"] = row["total_outflow"]
		summary["net_flow"] = row["net_flow"]
		summary["closing_balance"] = openingBalance + netFlow
	}

	pendingArgs := append([]any{}, scopeArgs...)
	pendingFromIdx := len(pendingArgs) + 1
	pendingArgs = append(pendingArgs, req.FromDate)
	pendingToIdx := len(pendingArgs) + 1
	pendingArgs = append(pendingArgs, req.ToDate)
	pendingProjectFilter := cashflowProjectFilter(&pendingArgs, req.ProjectID)
	pendingDateFilter := fmt.Sprintf(" AND %s >= $%d::date AND %s < ($%d::date + INTERVAL '1 day')", cashflowOccurredAtSQL(), pendingFromIdx, cashflowOccurredAtSQL(), pendingToIdx)
	pendingRows, err := r.Query(ctx, cashflowRowsCTE(scopeSQL, pendingProjectFilter, pendingDateFilter, " AND pay.status NOT IN ('approved','cancelled')")+`
		SELECT COUNT(*) AS pending_payment_count, COALESCE(SUM(amount), 0) AS pending_payment_total
		FROM cashflow_rows
	`, pendingArgs...)
	if err != nil {
		return nil, err
	}
	if len(pendingRows) > 0 {
		summary["pending_payment_count"] = pendingRows[0]["pending_payment_count"]
		summary["pending_payment_total"] = pendingRows[0]["pending_payment_total"]
	}

	periodExpr := cashflowPeriodExpr(req.GroupBy)
	seriesRows, err := r.Query(ctx, rangeCTE+fmt.Sprintf(`
		SELECT
			%s AS period_start,
			COALESCE(SUM(inflow_amount), 0) AS inflow_amount,
			COALESCE(SUM(outflow_amount), 0) AS outflow_amount,
			COALESCE(SUM(net_amount), 0) AS net_amount,
			COUNT(*) AS payment_count
		FROM cashflow_rows
		GROUP BY period_start
		ORDER BY period_start
	`, periodExpr), rangeArgs...)
	if err != nil {
		return nil, err
	}
	runningBalance := openingBalance
	for _, row := range seriesRows {
		runningBalance += number(row["net_amount"])
		row["running_balance"] = runningBalance
	}

	categoryRows, err := r.Query(ctx, rangeCTE+`
		SELECT
			category,
			direction,
			COUNT(*) AS payment_count,
			COALESCE(SUM(inflow_amount), 0) AS inflow_amount,
			COALESCE(SUM(outflow_amount), 0) AS outflow_amount,
			COALESCE(SUM(net_amount), 0) AS net_amount
		FROM cashflow_rows
		GROUP BY category, direction
		ORDER BY direction, category
	`, rangeArgs...)
	if err != nil {
		return nil, err
	}

	projectRows, err := r.Query(ctx, rangeCTE+`
		SELECT
			project_id,
			project_name,
			COUNT(*) AS payment_count,
			COALESCE(SUM(inflow_amount), 0) AS inflow_amount,
			COALESCE(SUM(outflow_amount), 0) AS outflow_amount,
			COALESCE(SUM(net_amount), 0) AS net_amount
		FROM cashflow_rows
		GROUP BY project_id, project_name
		ORDER BY GREATEST(COALESCE(SUM(inflow_amount), 0), COALESCE(SUM(outflow_amount), 0)) DESC, project_name
	`, rangeArgs...)
	if err != nil {
		return nil, err
	}

	ledgerRows, err := r.Query(ctx, rangeCTE+`
		SELECT
			occurred_at,
			id,
			project_id,
			project_name,
			payment_code,
			payment_type,
			payment_method,
			status,
			direction,
			category,
			amount,
			inflow_amount,
			outflow_amount,
			net_amount,
			target_type,
			target_id,
			created_by_name,
			approved_by_name
		FROM cashflow_rows
		ORDER BY occurred_at DESC, created_at DESC, id DESC
	`, rangeArgs...)
	if err != nil {
		return nil, err
	}

	return Record{
		"summary":    summary,
		"series":     seriesRows,
		"categories": categoryRows,
		"projects":   projectRows,
		"ledger":     ledgerRows,
	}, nil
}

func (r *Repository) reportFinanceAccessFilter(ctx context.Context, userID, projectAlias string, start int) (string, []any, error) {
	scopeSQL, scopeArgs, err := r.ProjectAccessFilter(ctx, userID, projectAlias, start)
	if err != nil {
		return "", nil, err
	}
	scopeArgs = append(scopeArgs, userID)
	return fmt.Sprintf("(%s AND %s)", scopeSQL, reportFinancePermissionSQL(projectAlias, start+len(scopeArgs)-1)), scopeArgs, nil
}

func reportFinancePermissionSQL(projectAlias string, userIDPlaceholder int) string {
	return fmt.Sprintf(`EXISTS (
		SELECT 1
		FROM user_roles finance_role
		JOIN role_permissions finance_role_permission ON finance_role_permission.role_id=finance_role.role_id
		JOIN permissions finance_permission ON finance_permission.id=finance_role_permission.permission_id
		WHERE finance_role.user_id=NULLIF($%d, '')::uuid
		  AND (finance_role.project_id IS NULL OR finance_role.project_id=%s.id)
		  AND finance_permission.key IN ('report.finance.view','accounting.view','project.reconciliation.view')
	)`, userIDPlaceholder, projectAlias)
}

func cashflowDirectionSQL(alias string) string {
	return fmt.Sprintf(`CASE
		WHEN %[1]s.payment_type IN ('client_receipt','inbound') OR (%[1]s.payment_type='advance' AND %[1]s.target_type='receivable') THEN 'inflow'
		WHEN %[1]s.payment_type IN ('supplier_payment','subcontractor_payment','outbound','tax_payment') OR (%[1]s.payment_type='advance' AND %[1]s.target_type='payable') THEN 'outflow'
		WHEN %[1]s.payment_type='advance_settlement' THEN 'non_cash'
		ELSE 'unclassified'
	END`, alias)
}

func cashflowRowsCTE(scopeSQL, projectFilter, dateFilter, statusFilter string) string {
	return fmt.Sprintf(`
		WITH classified AS (
			SELECT
				pay.*,
				p.name AS project_name,
				COALESCE(created.full_name, '') AS created_by_name,
				COALESCE(approved.full_name, '') AS approved_by_name,
				%s AS occurred_at,
				%s AS direction,
				CASE
					WHEN pay.payment_type='client_receipt' THEN 'Thu khách hàng'
					WHEN pay.payment_type='inbound' THEN 'Thu khác'
					WHEN pay.payment_type='advance' AND pay.target_type='receivable' THEN 'Thu ứng trước khách hàng'
					WHEN pay.payment_type='supplier_payment' THEN 'Thanh toán nhà cung cấp'
					WHEN pay.payment_type='subcontractor_payment' THEN 'Thanh toán thầu phụ'
					WHEN pay.payment_type='outbound' THEN 'Phiếu chi'
					WHEN pay.payment_type='tax_payment' THEN 'Nộp thuế GTGT'
					WHEN pay.payment_type='advance' AND pay.target_type='payable' THEN 'Chi ứng trước'
					WHEN pay.payment_type='advance_settlement' THEN 'Quyết toán tạm ứng'
					ELSE 'Unclassified'
				END AS category
			FROM payments pay
			JOIN projects p ON p.id=pay.project_id
			LEFT JOIN users created ON created.id=pay.created_by
			LEFT JOIN users approved ON approved.id=pay.approved_by
			WHERE pay.status <> 'cancelled'
			  AND p.status <> 'cancelled'
			  AND %s
			  %s
			  %s
			  %s
		),
		cashflow_rows AS (
			SELECT
				classified.*,
				CASE WHEN direction='inflow' THEN COALESCE(amount,0) ELSE 0 END AS inflow_amount,
				CASE WHEN direction='outflow' THEN COALESCE(amount,0) ELSE 0 END AS outflow_amount,
				CASE
					WHEN direction='inflow' THEN COALESCE(amount,0)
					WHEN direction='outflow' THEN -COALESCE(amount,0)
					ELSE 0
				END AS net_amount
			FROM classified
		)
	`, cashflowOccurredAtSQL(), cashflowDirectionSQL("pay"), scopeSQL, projectFilter, dateFilter, statusFilter)
}

func cashflowOccurredAtSQL() string {
	return "COALESCE(pay.paid_at, pay.updated_at, pay.created_at)"
}

func cashflowStatusFilter(status string) string {
	if status == "approved" {
		return " AND pay.status='approved'"
	}
	return ""
}

func cashflowProjectFilter(args *[]any, projectID string) string {
	projectID = cleanID(projectID)
	if projectID == "" {
		return ""
	}
	*args = append(*args, projectID)
	return fmt.Sprintf(" AND p.id=$%d::uuid", len(*args))
}

func cashflowPeriodExpr(groupBy string) string {
	switch groupBy {
	case "week":
		return "date_trunc('week', occurred_at)::date"
	case "month":
		return "date_trunc('month', occurred_at)::date"
	default:
		return "date_trunc('day', occurred_at)::date"
	}
}
