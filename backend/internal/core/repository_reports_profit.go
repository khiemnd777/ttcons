package core

import (
	"context"
	"fmt"
	"math"
)

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
