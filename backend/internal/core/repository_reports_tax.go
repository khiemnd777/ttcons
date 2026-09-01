package core

import (
	"context"
	"fmt"
)

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
