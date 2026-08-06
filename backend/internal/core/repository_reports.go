package core

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strings"
)

func (r *Repository) DashboardReport(ctx context.Context, userID string) (Record, error) {
	scopeSQL, scopeArgs, err := r.ProjectAccessFilter(ctx, userID, "p", 1)
	if err != nil {
		return nil, err
	}
	data, err := r.QueryOne(ctx, fmt.Sprintf(`
		WITH scoped_projects AS MATERIALIZED (
			SELECT p.id, p.status
			FROM projects p
			WHERE p.status <> 'cancelled' AND %s
		)
		SELECT
			(SELECT COUNT(*) FROM scoped_projects) AS total_projects,
			(SELECT COUNT(*) FROM scoped_projects WHERE status IN ('planning','in_progress','acceptance','handover')) AS active_projects,
			(SELECT COUNT(*) FROM scoped_projects WHERE status='blocked') AS blocked_projects,
			(SELECT COUNT(*) FROM approval_requests ar JOIN scoped_projects sp ON sp.id=ar.project_id WHERE ar.status='pending') AS pending_approvals,
			(SELECT COALESCE(SUM(rec.remaining_amount),0) FROM receivables rec JOIN scoped_projects sp ON sp.id=rec.project_id WHERE rec.status <> 'cancelled') AS receivable_remaining,
			(SELECT COALESCE(SUM(pay.remaining_amount),0) FROM payables pay JOIN scoped_projects sp ON sp.id=pay.project_id WHERE pay.status <> 'cancelled') AS payable_remaining,
			(SELECT COUNT(*) FROM site_material_balances smb JOIN materials m ON m.id=smb.material_id JOIN scoped_projects sp ON sp.id=smb.project_id WHERE smb.issued_quantity < smb.planned_quantity AND m.active) AS missing_materials,
			(SELECT COUNT(*) FROM acceptance_records ar JOIN scoped_projects sp ON sp.id=ar.project_id WHERE ar.result='fail') AS failed_acceptances,
			(SELECT COUNT(*) FROM rework_requests rw JOIN scoped_projects sp ON sp.id=rw.project_id WHERE rw.status IN ('draft','approved','in_progress')) AS pending_reworks,
			(SELECT COALESCE(SUM(CASE WHEN pay.payment_type='client_receipt' THEN pay.amount ELSE -pay.amount END),0)
			 FROM payments pay
			 JOIN scoped_projects sp ON sp.id=pay.project_id
			 WHERE pay.status <> 'cancelled'
			   AND date_trunc('month', pay.created_at)=date_trunc('month', now())) AS cashflow_month
	`, scopeSQL), scopeArgs...)
	if err != nil {
		return nil, err
	}
	charts := Record{}
	statusRows, err := r.Query(ctx, fmt.Sprintf(`SELECT p.status, COUNT(*) AS total FROM projects p WHERE p.status <> 'cancelled' AND %s GROUP BY p.status ORDER BY p.status`, scopeSQL), scopeArgs...)
	if err != nil {
		return nil, err
	}
	debtRows, err := r.Query(ctx, fmt.Sprintf(`
		SELECT 'Phải thu' AS type, COALESCE(SUM(rec.remaining_amount),0) AS amount
		FROM receivables rec
		JOIN projects p ON p.id=rec.project_id
		WHERE rec.status <> 'cancelled' AND p.status <> 'cancelled' AND %s
		UNION ALL
		SELECT 'Phải trả' AS type, COALESCE(SUM(pay.remaining_amount),0) AS amount
		FROM payables pay
		JOIN projects p ON p.id=pay.project_id
		WHERE pay.status <> 'cancelled' AND p.status <> 'cancelled' AND %s
	`, scopeSQL, scopeSQL), scopeArgs...)
	if err != nil {
		return nil, err
	}
	costRows, err := r.Query(ctx, fmt.Sprintf(`
		SELECT p.name, COALESCE(SUM(b.actual_amount),0) AS used_amount, COALESCE(SUM(b.remaining_amount),0) AS remaining_amount
		FROM projects p
		LEFT JOIN accounting_budget_balances b ON b.project_id=p.id AND b.status <> 'cancelled'
		WHERE p.status <> 'cancelled' AND %s
		GROUP BY p.id
		ORDER BY p.created_at
	`, scopeSQL), scopeArgs...)
	if err != nil {
		return nil, err
	}
	approvalRows, err := r.Query(ctx, fmt.Sprintf(`
		SELECT ar.target_type, COUNT(*) AS total
		FROM approval_requests ar
		JOIN projects p ON p.id=ar.project_id
		WHERE p.status <> 'cancelled' AND %s
		GROUP BY ar.target_type
		ORDER BY ar.target_type
	`, scopeSQL), scopeArgs...)
	if err != nil {
		return nil, err
	}
	charts["project_status"] = statusRows
	charts["debt"] = debtRows
	charts["cost_by_project"] = costRows
	charts["approvals_by_type"] = approvalRows
	data["charts"] = charts
	return data, nil
}

func (r *Repository) ReportProjectOptions(ctx context.Context, userID string, req ReportOptionsRequest, cursor *reportProjectOptionsCursor) ([]Record, Record, error) {
	scopeSQL, scopeArgs, err := r.ProjectAccessFilter(ctx, userID, "p", 1)
	if err != nil {
		return nil, nil, err
	}
	args := append([]any{}, scopeArgs...)
	conditions := []string{"p.status <> 'cancelled'", scopeSQL}
	if req.Search != "" {
		args = append(args, "%"+req.Search+"%")
		conditions = append(conditions, fmt.Sprintf("lower(btrim(p.code) || ' ' || btrim(p.name)) LIKE lower($%d)", len(args)))
	}
	if cursor != nil {
		args = append(args, cursor.NameKey, cursor.CodeKey, cursor.ID)
		conditions = append(conditions, fmt.Sprintf(`
			(lower(btrim(p.name)), lower(btrim(p.code)), p.id) >
			($%d, $%d, NULLIF($%d, '')::uuid)
		`, len(args)-2, len(args)-1, len(args)))
	}
	args = append(args, req.Limit+1)
	rows, err := r.Query(ctx, fmt.Sprintf(`
		SELECT p.id, p.code, p.name,
			lower(btrim(p.name)) AS name_key,
			lower(btrim(p.code)) AS code_key
		FROM projects p
		WHERE %s
		ORDER BY lower(btrim(p.name)), lower(btrim(p.code)), p.id
		LIMIT $%d
	`, strings.Join(conditions, " AND "), len(args)), args...)
	if err != nil {
		return nil, nil, err
	}
	var selected Record
	if req.SelectedID != "" {
		selectedArgs := append(append([]any{}, scopeArgs...), req.SelectedID)
		selected, err = r.QueryOne(ctx, fmt.Sprintf(`
			SELECT p.id, p.code, p.name
			FROM projects p
			WHERE p.status <> 'cancelled'
			  AND %s
			  AND p.id=NULLIF($%d, '')::uuid
		`, scopeSQL, len(selectedArgs)), selectedArgs...)
		if errors.Is(err, sql.ErrNoRows) {
			selected = nil
			err = nil
		}
		if err != nil {
			return nil, nil, err
		}
	}
	return rows, selected, nil
}

func (r *Repository) Report(ctx context.Context, kind, userID, projectID string) ([]Record, error) {
	scopeSQL, scopeArgs, err := r.ProjectAccessFilter(ctx, userID, "p", 1)
	if err != nil {
		return nil, err
	}
	args := append([]any{}, scopeArgs...)
	projectFilter := ""
	if projectID != "" {
		args = append(args, projectID)
		projectFilter = fmt.Sprintf(" AND p.id=$%d", len(args))
	}
	switch kind {
	case "projects":
		return r.Query(ctx, fmt.Sprintf(`
			SELECT p.id AS project_id, p.name, p.status, p.progress_percent,
				(SELECT COUNT(*) FROM issues i WHERE i.project_id=p.id AND i.status IN ('open','in_progress')) AS open_issues,
				(SELECT COUNT(*) FROM approval_requests ar WHERE ar.project_id=p.id AND ar.status='pending') AS pending_approvals
			FROM projects p
			WHERE p.status <> 'cancelled' AND %s%s
			ORDER BY p.created_at DESC
		`, scopeSQL, projectFilter), args...)
	case "warehouse":
		movementScope := fmt.Sprintf(`(
			sm.project_id IS NULL
			OR EXISTS (
				SELECT 1 FROM projects p
				WHERE p.id=sm.project_id AND p.status <> 'cancelled' AND %s
			)
		)`, scopeSQL)
		balanceProjectFilter := ""
		if projectID != "" {
			projectPlaceholder := fmt.Sprintf("$%d", len(args))
			movementScope = fmt.Sprintf(`EXISTS (
				SELECT 1 FROM projects p
				WHERE p.id=sm.project_id AND p.id=%s AND p.status <> 'cancelled' AND %s
			)`, projectPlaceholder, scopeSQL)
			balanceProjectFilter = fmt.Sprintf(" AND p.id=%s", projectPlaceholder)
		}
		return r.Query(ctx, fmt.Sprintf(`
			WITH stock_totals AS (
				SELECT sm.material_id, COALESCE(SUM(CASE
					WHEN sm.movement_type IN ('receive','return_to_warehouse') THEN sm.quantity
					WHEN sm.movement_type IN ('issue','loss') THEN -sm.quantity
					WHEN sm.movement_type='transfer' AND sm.target_warehouse_id IS NULL THEN -sm.quantity
					ELSE 0
				END),0) AS stock_quantity
				FROM stock_movements sm
				WHERE %s
				GROUP BY sm.material_id
			), site_totals AS (
				SELECT smb.material_id, COALESCE(SUM(smb.surplus_quantity),0) AS surplus_quantity
				FROM site_material_balances smb
				JOIN projects p ON p.id=smb.project_id
				WHERE p.status <> 'cancelled' AND %s%s
				GROUP BY smb.material_id
			)
			SELECT m.id AS material_id, m.name, m.category,
				COALESCE(st.stock_quantity,0) AS stock_quantity,
				COALESCE(site.surplus_quantity,0) AS surplus_quantity
			FROM materials m
			LEFT JOIN stock_totals st ON st.material_id=m.id
			LEFT JOIN site_totals site ON site.material_id=m.id
			WHERE m.active
			ORDER BY m.category, m.name
		`, movementScope, scopeSQL, balanceProjectFilter), args...)
	case "procurement":
		return r.Query(ctx, fmt.Sprintf(`
			SELECT pr.status, pr.budget_status, COUNT(*) AS total, COALESCE(SUM(pr.total_estimated_amount),0) AS amount
			FROM procurement_requests pr
			JOIN projects p ON p.id=pr.project_id
			WHERE pr.status <> 'cancelled' AND p.status <> 'cancelled' AND %s%s
			GROUP BY pr.status, pr.budget_status
			ORDER BY pr.status
		`, scopeSQL, projectFilter), args...)
	case "accounting":
		return r.Query(ctx, fmt.Sprintf(`
			WITH payable_totals AS (
				SELECT project_id, COALESCE(SUM(remaining_amount),0) AS payable_remaining
				FROM payables WHERE status <> 'cancelled' GROUP BY project_id
			), receivable_totals AS (
				SELECT project_id, COALESCE(SUM(remaining_amount),0) AS receivable_remaining
				FROM receivables WHERE status <> 'cancelled' GROUP BY project_id
			), budget_totals AS (
				SELECT project_id,
					COALESCE(SUM(committed_amount),0) AS committed_budget,
					COALESCE(SUM(actual_amount),0) AS used_budget,
					COALESCE(SUM(paid_amount),0) AS paid_budget,
					COALESCE(SUM(available_amount),0) AS available_budget
				FROM accounting_budget_balances WHERE status <> 'cancelled' GROUP BY project_id
			)
			SELECT p.id AS project_id, p.name,
				COALESCE(pay.payable_remaining,0) AS payable_remaining,
				COALESCE(rec.receivable_remaining,0) AS receivable_remaining,
				COALESCE(b.committed_budget,0) AS committed_budget,
				COALESCE(b.used_budget,0) AS used_budget,
				COALESCE(b.paid_budget,0) AS paid_budget,
				COALESCE(b.available_budget,0) AS available_budget
			FROM projects p
			LEFT JOIN payable_totals pay ON pay.project_id=p.id
			LEFT JOIN receivable_totals rec ON rec.project_id=p.id
			LEFT JOIN budget_totals b ON b.project_id=p.id
			WHERE p.status <> 'cancelled' AND %s%s
			ORDER BY p.name
		`, scopeSQL, projectFilter), args...)
	case "staff":
		return r.Query(ctx, fmt.Sprintf(`
			SELECT s.id AS staff_id, s.full_name, s.department, s.position, COUNT(DISTINCT p.id) AS project_count
			FROM staff s
			JOIN project_members pm ON pm.staff_id=s.id AND pm.active
			JOIN projects p ON p.id=pm.project_id
			WHERE s.status <> 'deleted' AND p.status <> 'cancelled' AND %s%s
			GROUP BY s.id
			ORDER BY s.full_name
		`, scopeSQL, projectFilter), args...)
	default:
		return nil, errors.New("Báo cáo không hợp lệ")
	}
}

func (r *Repository) CashflowReport(ctx context.Context, req CashflowReportRequest, userID string) (Record, error) {
	scopeSQL, scopeArgs, err := r.ProjectAccessFilter(ctx, userID, "p", 1)
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
		"ledger":     ledgerRows,
	}, nil
}

func (r *Repository) TaxReport(ctx context.Context, req TaxReportRequest, userID string) (Record, error) {
	scopeSQL, scopeArgs, err := r.ProjectAccessFilter(ctx, userID, "p", 1)
	if err != nil {
		return nil, err
	}
	args := append([]any{}, scopeArgs...)
	fromIdx := len(args) + 1
	args = append(args, req.FromDate)
	toIdx := len(args) + 1
	args = append(args, req.ToDate)
	projectFilter := taxProjectFilter(&args, req.ProjectID)
	inputDateFilter := fmt.Sprintf(" AND ii.invoice_date >= $%d::date AND ii.invoice_date <= $%d::date", fromIdx, toIdx)
	outputDateFilter := fmt.Sprintf(" AND oi.invoice_date >= $%d::date AND oi.invoice_date <= $%d::date", fromIdx, toIdx)
	adjustmentDateFilter := fmt.Sprintf(" AND adjustment.entry_date >= $%d::date AND adjustment.entry_date <= $%d::date", fromIdx, toIdx)
	sourceFilter := taxSourceFilter(req.Source)
	statusFilter := taxStatusFilter(req.Status)
	baseCTE := taxRowsCTE(scopeSQL, projectFilter, inputDateFilter, outputDateFilter, adjustmentDateFilter)
	filteredCTE := baseCTE + fmt.Sprintf(`
		, filtered_tax_rows AS (
			SELECT * FROM tax_rows
			WHERE 1=1
			%s
			%s
		)
	`, sourceFilter, statusFilter)

	summaryRows, err := r.Query(ctx, filteredCTE+`
		SELECT
			COUNT(*) AS invoice_count,
			COUNT(*) FILTER (WHERE direction='input') AS input_invoice_count,
			COUNT(*) FILTER (WHERE direction='output') AS output_invoice_count,
			COALESCE(SUM(amount_before_vat) FILTER (WHERE direction='input'), 0) AS input_taxable_amount,
			COALESCE(SUM(vat_amount) FILTER (WHERE direction='input'), 0) AS input_vat_amount,
			COALESCE(SUM(total_amount) FILTER (WHERE direction='input'), 0) AS input_total_amount,
			COALESCE(SUM(amount_before_vat) FILTER (WHERE direction='output'), 0) AS output_taxable_amount,
			COALESCE(SUM(vat_amount) FILTER (WHERE direction='output'), 0) AS output_vat_amount,
			COALESCE(SUM(total_amount) FILTER (WHERE direction='output'), 0) AS output_total_amount,
			COALESCE(SUM(vat_amount) FILTER (WHERE direction='output'), 0) - COALESCE(SUM(vat_amount) FILTER (WHERE direction='input'), 0) AS net_vat_amount,
			COUNT(*) FILTER (WHERE missing_tax_code) AS missing_tax_code_count,
			COUNT(*) FILTER (WHERE vat_math_mismatch) AS vat_math_mismatch_count
		FROM filtered_tax_rows
	`, args...)
	if err != nil {
		return nil, err
	}
	summary := Record{
		"from":                    req.FromDate,
		"to":                      req.ToDate,
		"group_by":                req.GroupBy,
		"source":                  req.Source,
		"status":                  req.Status,
		"project_id":              req.ProjectID,
		"invoice_count":           0,
		"input_invoice_count":     0,
		"output_invoice_count":    0,
		"input_taxable_amount":    0,
		"input_vat_amount":        0,
		"input_total_amount":      0,
		"output_taxable_amount":   0,
		"output_vat_amount":       0,
		"output_total_amount":     0,
		"net_vat_amount":          0,
		"net_vat_status":          "neutral",
		"missing_tax_code_count":  0,
		"vat_math_mismatch_count": 0,
		"excluded_unposted_count": 0,
		"excluded_unposted_total": 0,
		"warning_count":           0,
	}
	if len(summaryRows) > 0 {
		for key, value := range summaryRows[0] {
			summary[key] = value
		}
	}

	excludedRows, err := r.Query(ctx, baseCTE+fmt.Sprintf(`
		SELECT COUNT(*) AS excluded_unposted_count, COALESCE(SUM(total_amount), 0) AS excluded_unposted_total
		FROM tax_rows
		WHERE 1=1
		%s
		AND NOT posted
	`, sourceFilter), args...)
	if err != nil {
		return nil, err
	}
	if len(excludedRows) > 0 {
		summary["excluded_unposted_count"] = excludedRows[0]["excluded_unposted_count"]
		summary["excluded_unposted_total"] = excludedRows[0]["excluded_unposted_total"]
	}

	netVAT := number(summary["net_vat_amount"])
	if netVAT > 1 {
		summary["net_vat_status"] = "payable"
	} else if netVAT < -1 {
		summary["net_vat_status"] = "credit"
	}
	warnings := taxReportWarnings(summary, req.Status)
	summary["warning_count"] = len(warnings)

	periodRows, err := r.Query(ctx, filteredCTE+fmt.Sprintf(`
		SELECT
			%s AS period_start,
			COUNT(*) AS invoice_count,
			COUNT(*) FILTER (WHERE direction='input') AS input_invoice_count,
			COUNT(*) FILTER (WHERE direction='output') AS output_invoice_count,
			COALESCE(SUM(amount_before_vat) FILTER (WHERE direction='input'), 0) AS input_taxable_amount,
			COALESCE(SUM(vat_amount) FILTER (WHERE direction='input'), 0) AS input_vat_amount,
			COALESCE(SUM(total_amount) FILTER (WHERE direction='input'), 0) AS input_total_amount,
			COALESCE(SUM(amount_before_vat) FILTER (WHERE direction='output'), 0) AS output_taxable_amount,
			COALESCE(SUM(vat_amount) FILTER (WHERE direction='output'), 0) AS output_vat_amount,
			COALESCE(SUM(total_amount) FILTER (WHERE direction='output'), 0) AS output_total_amount,
			COALESCE(SUM(vat_amount) FILTER (WHERE direction='output'), 0) - COALESCE(SUM(vat_amount) FILTER (WHERE direction='input'), 0) AS net_vat_amount
		FROM filtered_tax_rows
		GROUP BY period_start
		ORDER BY period_start
	`, taxPeriodExpr(req.GroupBy)), args...)
	if err != nil {
		return nil, err
	}

	counterpartyRows, err := r.Query(ctx, filteredCTE+`
		SELECT
			direction,
			tax_code,
			counterparty_name,
			COUNT(*) AS invoice_count,
			COALESCE(SUM(amount_before_vat), 0) AS taxable_amount,
			COALESCE(SUM(vat_amount), 0) AS vat_amount,
			COALESCE(SUM(total_amount), 0) AS total_amount,
			COUNT(*) FILTER (WHERE missing_tax_code OR vat_math_mismatch) AS warning_count
		FROM filtered_tax_rows
		GROUP BY direction, tax_code, counterparty_name
		ORDER BY direction, vat_amount DESC, counterparty_name
	`, args...)
	if err != nil {
		return nil, err
	}

	ledgerRows, err := r.Query(ctx, filteredCTE+`
		SELECT
			invoice_date,
			id,
			direction,
			project_id,
			project_name,
			invoice_number,
			invoice_symbol,
			counterparty_name,
			tax_code,
			amount_before_vat,
			vat_amount,
			total_amount,
			currency,
			status,
			source_type,
			source_id,
			source_code,
			secondary_source_type,
			secondary_source_id,
			secondary_source_code,
			document_id,
			document_name,
			missing_tax_code,
			vat_math_mismatch,
			CONCAT_WS('; ',
				CASE WHEN missing_tax_code THEN 'missing_tax_code' END,
				CASE WHEN vat_math_mismatch THEN 'vat_math_mismatch' END
			) AS warning_flags
		FROM filtered_tax_rows
		ORDER BY invoice_date DESC, created_at DESC, id DESC
	`, args...)
	if err != nil {
		return nil, err
	}

	return Record{
		"summary":        summary,
		"periods":        periodRows,
		"counterparties": counterpartyRows,
		"ledger":         ledgerRows,
		"warnings":       warnings,
	}, nil
}

func (r *Repository) ProfitReport(ctx context.Context, req ProfitReportRequest, userID string) (Record, error) {
	scopeSQL, scopeArgs, err := r.ProjectAccessFilter(ctx, userID, "p", 1)
	if err != nil {
		return nil, err
	}
	args := append([]any{}, scopeArgs...)
	fromIdx := len(args) + 1
	args = append(args, req.FromDate)
	toIdx := len(args) + 1
	args = append(args, req.ToDate)
	projectFilter := profitProjectFilter(&args, req.ProjectID)
	inputDateFilter := fmt.Sprintf(" AND ii.invoice_date >= $%d::date AND ii.invoice_date <= $%d::date", fromIdx, toIdx)
	outputDateFilter := fmt.Sprintf(" AND oi.invoice_date >= $%d::date AND oi.invoice_date <= $%d::date", fromIdx, toIdx)
	statusFilter := profitStatusFilter(req.Status)
	baseCTE := profitRowsCTE(scopeSQL, projectFilter, inputDateFilter, outputDateFilter)
	filteredCTE := baseCTE + fmt.Sprintf(`
		, filtered_profit_rows AS (
			SELECT * FROM profit_rows
			WHERE 1=1
			%s
		)
	`, statusFilter)

	summaryRows, err := r.Query(ctx, filteredCTE+`
		SELECT
			COUNT(*) AS ledger_count,
			COUNT(*) FILTER (WHERE direction='revenue') AS revenue_invoice_count,
			COUNT(*) FILTER (WHERE direction='cost') AS cost_invoice_count,
			COALESCE(SUM(revenue_amount), 0) AS revenue_amount,
			COALESCE(SUM(direct_cost_amount), 0) AS direct_cost_amount,
			COALESCE(SUM(indirect_cost_amount), 0) AS indirect_cost_amount,
			COALESCE(SUM(direct_cost_amount + indirect_cost_amount), 0) AS total_cost_amount,
			COALESCE(SUM(revenue_amount - direct_cost_amount), 0) AS gross_profit,
			COALESCE(SUM(revenue_amount - direct_cost_amount - indirect_cost_amount), 0) AS net_profit,
			COUNT(*) FILTER (WHERE unclassified_cost) AS unclassified_cost_count
		FROM filtered_profit_rows
	`, args...)
	if err != nil {
		return nil, err
	}
	summary := Record{
		"from":                    req.FromDate,
		"to":                      req.ToDate,
		"group_by":                req.GroupBy,
		"status":                  req.Status,
		"project_id":              req.ProjectID,
		"ledger_count":            0,
		"revenue_invoice_count":   0,
		"cost_invoice_count":      0,
		"revenue_amount":          0,
		"direct_cost_amount":      0,
		"indirect_cost_amount":    0,
		"total_cost_amount":       0,
		"gross_profit":            0,
		"net_profit":              0,
		"gross_margin_percent":    0,
		"net_margin_percent":      0,
		"unclassified_cost_count": 0,
		"excluded_unposted_count": 0,
		"excluded_unposted_total": 0,
		"warning_count":           0,
	}
	if len(summaryRows) > 0 {
		for key, value := range summaryRows[0] {
			summary[key] = value
		}
	}
	profitApplyMargins(summary)

	excludedRows, err := r.Query(ctx, baseCTE+`
		SELECT COUNT(*) AS excluded_unposted_count, COALESCE(SUM(amount_before_vat), 0) AS excluded_unposted_total
		FROM profit_rows
		WHERE NOT posted
	`, args...)
	if err != nil {
		return nil, err
	}
	if len(excludedRows) > 0 {
		summary["excluded_unposted_count"] = excludedRows[0]["excluded_unposted_count"]
		summary["excluded_unposted_total"] = excludedRows[0]["excluded_unposted_total"]
	}
	warnings := profitReportWarnings(summary, req.Status)
	summary["warning_count"] = len(warnings)

	periodRows, err := r.Query(ctx, filteredCTE+fmt.Sprintf(`
		SELECT
			%s AS period_start,
			COUNT(*) AS ledger_count,
			COUNT(*) FILTER (WHERE direction='revenue') AS revenue_invoice_count,
			COUNT(*) FILTER (WHERE direction='cost') AS cost_invoice_count,
			COALESCE(SUM(revenue_amount), 0) AS revenue_amount,
			COALESCE(SUM(direct_cost_amount), 0) AS direct_cost_amount,
			COALESCE(SUM(indirect_cost_amount), 0) AS indirect_cost_amount,
			COALESCE(SUM(direct_cost_amount + indirect_cost_amount), 0) AS total_cost_amount,
			COALESCE(SUM(revenue_amount - direct_cost_amount), 0) AS gross_profit,
			COALESCE(SUM(revenue_amount - direct_cost_amount - indirect_cost_amount), 0) AS net_profit
		FROM filtered_profit_rows
		GROUP BY period_start
		ORDER BY period_start
	`, profitPeriodExpr(req.GroupBy)), args...)
	if err != nil {
		return nil, err
	}
	for _, row := range periodRows {
		profitApplyMargins(row)
	}

	projectRows, err := r.Query(ctx, filteredCTE+`
		SELECT
			project_id,
			project_name,
			COUNT(*) AS ledger_count,
			COUNT(*) FILTER (WHERE direction='revenue') AS revenue_invoice_count,
			COUNT(*) FILTER (WHERE direction='cost') AS cost_invoice_count,
			COALESCE(SUM(revenue_amount), 0) AS revenue_amount,
			COALESCE(SUM(direct_cost_amount), 0) AS direct_cost_amount,
			COALESCE(SUM(indirect_cost_amount), 0) AS indirect_cost_amount,
			COALESCE(SUM(direct_cost_amount + indirect_cost_amount), 0) AS total_cost_amount,
			COALESCE(SUM(revenue_amount - direct_cost_amount), 0) AS gross_profit,
			COALESCE(SUM(revenue_amount - direct_cost_amount - indirect_cost_amount), 0) AS net_profit,
			COUNT(*) FILTER (WHERE unclassified_cost) AS warning_count
		FROM filtered_profit_rows
		GROUP BY project_id, project_name
		ORDER BY net_profit DESC, project_name
	`, args...)
	if err != nil {
		return nil, err
	}
	for _, row := range projectRows {
		profitApplyMargins(row)
	}

	breakdownRows, err := r.Query(ctx, filteredCTE+`
		SELECT
			cost_type,
			cost_group,
			classification_reason,
			COUNT(*) AS invoice_count,
			COALESCE(SUM(amount_before_vat), 0) AS cost_amount,
			COALESCE(SUM(vat_amount), 0) AS vat_amount,
			COALESCE(SUM(total_amount), 0) AS total_amount,
			COUNT(*) FILTER (WHERE unclassified_cost) AS warning_count
		FROM filtered_profit_rows
		WHERE direction='cost'
		GROUP BY cost_type, cost_group, classification_reason
		ORDER BY cost_type, cost_amount DESC, cost_group
	`, args...)
	if err != nil {
		return nil, err
	}

	ledgerRows, err := r.Query(ctx, filteredCTE+`
		SELECT
			invoice_date,
			id,
			direction,
			cost_type,
			cost_group,
			classification_reason,
			project_id,
			project_name,
			invoice_number,
			invoice_symbol,
			counterparty_name,
			tax_code,
			amount_before_vat,
			vat_amount,
			total_amount,
			revenue_amount,
			direct_cost_amount,
			indirect_cost_amount,
			currency,
			status,
			source_type,
			source_id,
			source_code,
			secondary_source_type,
			secondary_source_id,
			secondary_source_code,
			document_id,
			document_name,
			unclassified_cost,
			CASE WHEN unclassified_cost THEN 'indirect_or_unclassified_cost' ELSE '' END AS warning_flags
		FROM filtered_profit_rows
		ORDER BY invoice_date DESC, created_at DESC, id DESC
	`, args...)
	if err != nil {
		return nil, err
	}

	return Record{
		"summary":        summary,
		"periods":        periodRows,
		"projects":       projectRows,
		"cost_breakdown": breakdownRows,
		"ledger":         ledgerRows,
		"warnings":       warnings,
	}, nil
}

func profitApplyMargins(row Record) {
	revenue := number(row["revenue_amount"])
	row["gross_margin_percent"] = profitMarginPercent(revenue, number(row["gross_profit"]))
	row["net_margin_percent"] = profitMarginPercent(revenue, number(row["net_profit"]))
}

func profitMarginPercent(revenue, profit float64) float64 {
	if math.Abs(revenue) <= 1 {
		return 0
	}
	return profit / revenue * 100
}

func profitReportWarnings(summary Record, status string) []Record {
	warnings := []Record{}
	if count := number(summary["unclassified_cost_count"]); count > 0 {
		warnings = append(warnings, Record{
			"code":     "indirect_or_unclassified_cost",
			"severity": "warning",
			"message":  "Có chi phí chưa đủ nguồn phân loại trực tiếp nên được đưa vào chi phí gián tiếp",
			"count":    count,
		})
	}
	if status == "posted" {
		if count := number(summary["excluded_unposted_count"]); count > 0 {
			warnings = append(warnings, Record{
				"code":     "unposted_excluded",
				"severity": "info",
				"message":  "Có chứng từ chưa ghi nhận bị loại khỏi báo cáo mặc định",
				"count":    count,
				"amount":   summary["excluded_unposted_total"],
			})
		}
	}
	if number(summary["revenue_amount"]) > 0 && number(summary["net_profit"]) < 0 {
		warnings = append(warnings, Record{
			"code":     "negative_net_profit",
			"severity": "warning",
			"message":  "Lợi nhuận ròng đang âm trong khoảng lọc",
			"amount":   summary["net_profit"],
		})
	}
	return warnings
}

func taxReportWarnings(summary Record, status string) []Record {
	warnings := []Record{}
	if count := number(summary["missing_tax_code_count"]); count > 0 {
		warnings = append(warnings, Record{
			"code":     "missing_tax_code",
			"severity": "warning",
			"message":  "Có hoá đơn thiếu mã số thuế đối tác",
			"count":    count,
		})
	}
	if count := number(summary["vat_math_mismatch_count"]); count > 0 {
		warnings = append(warnings, Record{
			"code":     "vat_math_mismatch",
			"severity": "warning",
			"message":  "Có hoá đơn lệch phép tính trước VAT + VAT = tổng tiền",
			"count":    count,
		})
	}
	if status == "posted" {
		if count := number(summary["excluded_unposted_count"]); count > 0 {
			warnings = append(warnings, Record{
				"code":     "unposted_excluded",
				"severity": "info",
				"message":  "Có hóa đơn chưa ghi nhận bị loại khỏi báo cáo mặc định",
				"count":    count,
				"amount":   summary["excluded_unposted_total"],
			})
		}
	}
	return warnings
}

func taxRowsCTE(scopeSQL, projectFilter, inputDateFilter, outputDateFilter, adjustmentDateFilter string) string {
	return fmt.Sprintf(`
		WITH tax_rows AS (
			SELECT
				'input'::text AS direction,
				line.id::text AS id,
				ii.project_id::text AS project_id,
				p.name AS project_name,
				ii.invoice_number,
				COALESCE(ii.invoice_symbol, '') AS invoice_symbol,
				ii.invoice_date,
				ii.vendor_name AS counterparty_name,
				COALESCE(ii.vendor_tax_code,'') AS tax_code,
				line.taxable_amount AS amount_before_vat,
				line.vat_amount,
				line.taxable_amount+line.vat_amount AS total_amount,
				ii.currency,
				ii.status,
				(original.id IS NOT NULL AND reversal.id IS NULL) AS posted,
				'payable'::text AS source_type,
				ii.payable_id::text AS source_id,
				COALESCE(payable.payable_code, '') AS source_code,
				'procurement_request'::text AS secondary_source_type,
				ii.procurement_request_id::text AS secondary_source_id,
				COALESCE(procurement.request_code, '') AS secondary_source_code,
				ii.document_id::text AS document_id,
				COALESCE(doc.name, '') AS document_name,
				(NULLIF(btrim(ii.vendor_tax_code),'') IS NULL) AS missing_tax_code,
				(ABS((COALESCE(line.taxable_amount, 0) + COALESCE(line.vat_amount, 0)) - COALESCE(ii.total_amount, 0)) > 1) AS vat_math_mismatch,
				ii.created_at
			FROM invoice_tax_lines line
			JOIN input_invoices ii ON ii.id=line.input_invoice_id
			JOIN projects p ON p.id=ii.project_id
			JOIN tax_codes tax ON tax.id=line.tax_code_id
			LEFT JOIN journal_entries original ON original.source_type='input_invoice' AND original.source_id=ii.id
				AND original.source_event='input_invoice_verified' AND original.status='posted'
			LEFT JOIN journal_entries reversal ON reversal.reversal_of_id=original.id AND reversal.status='posted'
			LEFT JOIN payables payable ON payable.id=ii.payable_id
			LEFT JOIN procurement_requests procurement ON procurement.id=ii.procurement_request_id
			LEFT JOIN documents doc ON doc.id=ii.document_id
			WHERE ii.status <> 'deleted'
			  AND p.status <> 'cancelled'
			  AND %s
			  %s
			  %s
			UNION ALL
			SELECT
				'output'::text AS direction,
				line.id::text AS id,
				oi.project_id::text AS project_id,
				p.name AS project_name,
				oi.invoice_number,
				COALESCE(oi.invoice_symbol, '') AS invoice_symbol,
				oi.invoice_date,
				oi.client_name AS counterparty_name,
				COALESCE(oi.client_tax_code,'') AS tax_code,
				line.taxable_amount AS amount_before_vat,
				line.vat_amount,
				line.taxable_amount+line.vat_amount AS total_amount,
				oi.currency,
				oi.status,
				(original.id IS NOT NULL AND reversal.id IS NULL) AS posted,
				'receivable'::text AS source_type,
				oi.receivable_id::text AS source_id,
				COALESCE(receivable.receivable_code, '') AS source_code,
				'payment'::text AS secondary_source_type,
				oi.payment_id::text AS secondary_source_id,
				COALESCE(payment.payment_code, '') AS secondary_source_code,
				oi.document_id::text AS document_id,
				COALESCE(doc.name, '') AS document_name,
				(NULLIF(btrim(oi.client_tax_code),'') IS NULL) AS missing_tax_code,
				(ABS((COALESCE(line.taxable_amount, 0) + COALESCE(line.vat_amount, 0)) - COALESCE(oi.total_amount, 0)) > 1) AS vat_math_mismatch,
				oi.created_at
			FROM invoice_tax_lines line
			JOIN output_invoices oi ON oi.id=line.output_invoice_id
			JOIN projects p ON p.id=oi.project_id
			JOIN tax_codes tax ON tax.id=line.tax_code_id
			LEFT JOIN journal_entries original ON original.source_type='output_invoice' AND original.source_id=oi.id
				AND original.source_event='output_invoice_issued' AND original.status='posted'
			LEFT JOIN journal_entries reversal ON reversal.reversal_of_id=original.id AND reversal.status='posted'
			LEFT JOIN receivables receivable ON receivable.id=oi.receivable_id
			LEFT JOIN payments payment ON payment.id=oi.payment_id
			LEFT JOIN documents doc ON doc.id=oi.document_id
			WHERE oi.status <> 'deleted'
			  AND p.status <> 'cancelled'
			  AND %s
			  %s
			  %s
			UNION ALL
			SELECT
				adjustment.direction,
				adjustment.id::text,
				adjustment.project_id::text,
				p.name,
				COALESCE(adjustment.source_document_no,'ĐCT-' || left(adjustment.id::text,8)),
				''::text,
				adjustment.entry_date,
				adjustment.reason,
				tax.tax_code,
				adjustment.taxable_delta,
				adjustment.vat_delta,
				adjustment.taxable_delta+adjustment.vat_delta,
				'VND'::text,
				'posted'::text,
				true,
				'tax_adjustment'::text,
				adjustment.id::text,
				COALESCE(adjustment.source_document_no,''),
				''::text,
				NULL::text,
				''::text,
				NULL::text,
				''::text,
				false,
				false,
				adjustment.created_at
			FROM tax_adjustments adjustment
			JOIN projects p ON p.id=adjustment.project_id
			JOIN tax_codes tax ON tax.id=adjustment.tax_code_id
			JOIN journal_entries adjustment_journal ON adjustment_journal.id=adjustment.journal_entry_id AND adjustment_journal.status='posted'
			WHERE p.status <> 'cancelled'
			  AND %s
			  %s
			  %s
		)
	`, scopeSQL, projectFilter, inputDateFilter, scopeSQL, projectFilter, outputDateFilter,
		scopeSQL, projectFilter, adjustmentDateFilter)
}

func taxSourceFilter(source string) string {
	switch source {
	case "input":
		return " AND direction='input'"
	case "output":
		return " AND direction='output'"
	default:
		return ""
	}
}

func taxStatusFilter(status string) string {
	if status == "posted" {
		return " AND posted"
	}
	return ""
}

func taxProjectFilter(args *[]any, projectID string) string {
	projectID = cleanID(projectID)
	if projectID == "" {
		return ""
	}
	*args = append(*args, projectID)
	return fmt.Sprintf(" AND p.id=$%d::uuid", len(*args))
}

func taxPeriodExpr(groupBy string) string {
	if groupBy == "quarter" {
		return "date_trunc('quarter', invoice_date)::date"
	}
	return "date_trunc('month', invoice_date)::date"
}

func profitRowsCTE(scopeSQL, projectFilter, inputDateFilter, outputDateFilter string) string {
	directExpr := `
		ii.procurement_request_id IS NOT NULL
		OR lower(COALESCE(payable.source_type, '')) IN ('procurement','boq','boq_item','work_item','acceptance','incoming_acceptance','outgoing_acceptance','operation','rework')
		OR lower(COALESCE(payable.vendor_type, '')) IN ('supplier','subcontractor')
	`
	return fmt.Sprintf(`
		WITH input_rows AS (
			SELECT
				ii.id::text AS id,
				ii.project_id::text AS project_id,
				p.name AS project_name,
				ii.invoice_number,
				COALESCE(ii.invoice_symbol, '') AS invoice_symbol,
				ii.invoice_date,
				ii.vendor_name AS counterparty_name,
				COALESCE(ii.vendor_tax_code, '') AS tax_code,
				ii.amount_before_vat,
				ii.vat_amount,
				ii.total_amount,
				ii.currency,
				ii.status,
				(ii.status='verified') AS posted,
				'payable'::text AS source_type,
				ii.payable_id::text AS source_id,
				COALESCE(payable.payable_code, '') AS source_code,
				'procurement_request'::text AS secondary_source_type,
				ii.procurement_request_id::text AS secondary_source_id,
				COALESCE(procurement.request_code, '') AS secondary_source_code,
				ii.document_id::text AS document_id,
				COALESCE(doc.name, '') AS document_name,
				(%s) AS direct_cost,
				CASE
					WHEN ii.procurement_request_id IS NOT NULL OR lower(COALESCE(payable.source_type, '')) IN ('procurement','boq','boq_item') THEN 'Procurement / BOQ'
					WHEN lower(COALESCE(payable.vendor_type, ''))='subcontractor' THEN 'Thầu phụ'
					WHEN lower(COALESCE(payable.vendor_type, ''))='supplier' THEN 'Nhà cung cấp'
					WHEN COALESCE(payable.source_type, '') <> '' THEN 'Source khác'
					ELSE 'Chi phí gián tiếp'
				END AS cost_group,
				CASE
					WHEN ii.procurement_request_id IS NOT NULL THEN 'linked_procurement_invoice'
					WHEN lower(COALESCE(payable.source_type, '')) IN ('procurement','boq','boq_item','work_item','acceptance','incoming_acceptance','outgoing_acceptance','operation','rework') THEN 'payable_source_direct'
					WHEN lower(COALESCE(payable.vendor_type, '')) IN ('supplier','subcontractor') THEN 'vendor_type_direct'
					WHEN COALESCE(payable.source_type, '') <> '' THEN 'payable_source_indirect'
					ELSE 'indirect_fallback'
				END AS classification_reason,
				ii.created_at
			FROM input_invoices ii
			JOIN projects p ON p.id=ii.project_id
			LEFT JOIN payables payable ON payable.id=ii.payable_id
			LEFT JOIN procurement_requests procurement ON procurement.id=ii.procurement_request_id
			LEFT JOIN documents doc ON doc.id=ii.document_id
			WHERE ii.status <> 'deleted'
			  AND p.status <> 'cancelled'
			  AND %s
			  %s
			  %s
		),
		output_rows AS (
			SELECT
				oi.id::text AS id,
				oi.project_id::text AS project_id,
				p.name AS project_name,
				oi.invoice_number,
				COALESCE(oi.invoice_symbol, '') AS invoice_symbol,
				oi.invoice_date,
				oi.client_name AS counterparty_name,
				COALESCE(oi.client_tax_code, '') AS tax_code,
				oi.amount_before_vat,
				oi.vat_amount,
				oi.total_amount,
				oi.currency,
				oi.status,
				(oi.status='issued') AS posted,
				'receivable'::text AS source_type,
				oi.receivable_id::text AS source_id,
				COALESCE(receivable.receivable_code, '') AS source_code,
				'payment'::text AS secondary_source_type,
				oi.payment_id::text AS secondary_source_id,
				COALESCE(payment.payment_code, '') AS secondary_source_code,
				oi.document_id::text AS document_id,
				COALESCE(doc.name, '') AS document_name,
				oi.created_at
			FROM output_invoices oi
			JOIN projects p ON p.id=oi.project_id
			LEFT JOIN receivables receivable ON receivable.id=oi.receivable_id
			LEFT JOIN payments payment ON payment.id=oi.payment_id
			LEFT JOIN documents doc ON doc.id=oi.document_id
			WHERE oi.status <> 'deleted'
			  AND p.status <> 'cancelled'
			  AND %s
			  %s
			  %s
		),
		profit_rows AS (
			SELECT
				'revenue'::text AS direction,
				'revenue'::text AS cost_type,
				'Revenue'::text AS cost_group,
				'issued_output_invoice'::text AS classification_reason,
				id,
				project_id,
				project_name,
				invoice_number,
				invoice_symbol,
				invoice_date,
				counterparty_name,
				tax_code,
				amount_before_vat,
				vat_amount,
				total_amount,
				amount_before_vat AS revenue_amount,
				0::numeric AS direct_cost_amount,
				0::numeric AS indirect_cost_amount,
				currency,
				status,
				posted,
				source_type,
				source_id,
				source_code,
				secondary_source_type,
				secondary_source_id,
				secondary_source_code,
				document_id,
				document_name,
				false AS unclassified_cost,
				created_at
			FROM output_rows
			UNION ALL
			SELECT
				'cost'::text AS direction,
				CASE WHEN direct_cost THEN 'direct' ELSE 'indirect' END AS cost_type,
				cost_group,
				classification_reason,
				id,
				project_id,
				project_name,
				invoice_number,
				invoice_symbol,
				invoice_date,
				counterparty_name,
				tax_code,
				amount_before_vat,
				vat_amount,
				total_amount,
				0::numeric AS revenue_amount,
				CASE WHEN direct_cost THEN amount_before_vat ELSE 0 END AS direct_cost_amount,
				CASE WHEN direct_cost THEN 0 ELSE amount_before_vat END AS indirect_cost_amount,
				currency,
				status,
				posted,
				source_type,
				source_id,
				source_code,
				secondary_source_type,
				secondary_source_id,
				secondary_source_code,
				document_id,
				document_name,
				NOT direct_cost AS unclassified_cost,
				created_at
			FROM input_rows
		)
	`, directExpr, scopeSQL, projectFilter, inputDateFilter, scopeSQL, projectFilter, outputDateFilter)
}

func profitStatusFilter(status string) string {
	if status == "posted" {
		return " AND posted"
	}
	return ""
}

func profitProjectFilter(args *[]any, projectID string) string {
	projectID = cleanID(projectID)
	if projectID == "" {
		return ""
	}
	*args = append(*args, projectID)
	return fmt.Sprintf(" AND p.id=$%d::uuid", len(*args))
}

func profitPeriodExpr(groupBy string) string {
	if groupBy == "quarter" {
		return "date_trunc('quarter', invoice_date)::date"
	}
	return "date_trunc('month', invoice_date)::date"
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
				CASE
					WHEN pay.payment_type IN ('client_receipt','inbound') OR (pay.payment_type='advance' AND pay.target_type='receivable') THEN 'inflow'
					WHEN pay.payment_type IN ('supplier_payment','subcontractor_payment','outbound','tax_payment') OR (pay.payment_type='advance' AND pay.target_type='payable') THEN 'outflow'
					WHEN pay.payment_type='advance_settlement' THEN 'non_cash'
					ELSE 'unclassified'
				END AS direction,
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
	`, cashflowOccurredAtSQL(), scopeSQL, projectFilter, dateFilter, statusFilter)
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
