package core

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

func (s *Service) AccountingTrialBalance(ctx context.Context, userID string, filter AccountingReportFilter) (Record, error) {
	filter, err := normalizeAccountingReportFilter(filter)
	if err != nil {
		return nil, err
	}
	return s.repo.AccountingTrialBalance(ctx, userID, filter)
}

func (s *Service) AccountingGeneralLedger(ctx context.Context, userID string, filter AccountingReportFilter) (Record, error) {
	filter, err := normalizeAccountingReportFilter(filter)
	if err != nil {
		return nil, err
	}
	return s.repo.AccountingGeneralLedger(ctx, userID, filter)
}

func (s *Service) AccountingProfitLoss(ctx context.Context, userID string, filter AccountingReportFilter) (Record, error) {
	filter, err := normalizeAccountingReportFilter(filter)
	if err != nil {
		return nil, err
	}
	return s.repo.AccountingProfitLoss(ctx, userID, filter)
}

func (s *Service) AccountingVATLedger(ctx context.Context, userID string, filter AccountingReportFilter) (Record, error) {
	filter, err := normalizeAccountingReportFilter(filter)
	if err != nil {
		return nil, err
	}
	return s.repo.AccountingVATLedger(ctx, userID, filter)
}

func (s *Service) AccountingBackfillIssues(ctx context.Context, userID, projectID string) ([]Record, error) {
	projectID = cleanID(projectID)
	if projectID != "" && !validActivityUUID(projectID) {
		return nil, validationError("Công trình không hợp lệ", Record{"project_id": projectID})
	}
	return s.repo.AccountingBackfillIssues(ctx, userID, projectID)
}

func (s *Service) ResolveAccountingBackfillIssue(ctx context.Context, id string, req AccountingReconciliationResolveRequest, actorID, actorName string) (Record, error) {
	id = cleanID(id)
	if id == "" {
		return nil, validationError("Đối chiếu chuyển đổi không hợp lệ")
	}
	action := strings.TrimSpace(req.Action)
	if action != "resolve" && action != "accept" {
		return nil, validationError("Cách xử lý đối chiếu không hợp lệ")
	}
	note := strings.TrimSpace(req.ResolutionNote)
	if note == "" {
		return nil, validationError("Căn cứ xử lý là bắt buộc")
	}
	journalEntryID := cleanID(req.JournalEntryID)
	if action == "resolve" && journalEntryID == "" {
		return nil, validationError("Chứng từ ghi sổ bổ sung là bắt buộc")
	}
	if action == "accept" && journalEntryID != "" {
		return nil, validationError("Không chọn Chứng từ ghi sổ khi chấp nhận không chuyển sổ")
	}
	return s.repo.ResolveAccountingBackfillIssue(ctx, id, action, note, journalEntryID, actorID, actorName)
}

func normalizeAccountingReportFilter(filter AccountingReportFilter) (AccountingReportFilter, error) {
	filter.ProjectID = cleanID(filter.ProjectID)
	if filter.ProjectID != "" && !validActivityUUID(filter.ProjectID) {
		return filter, validationError("Công trình không hợp lệ", Record{"project_id": filter.ProjectID})
	}
	now := time.Now()
	if strings.TrimSpace(filter.DateFrom) == "" {
		filter.DateFrom = fmt.Sprintf("%04d-01-01", now.Year())
	}
	if strings.TrimSpace(filter.DateTo) == "" {
		filter.DateTo = now.Format("2006-01-02")
	}
	if _, err := time.Parse("2006-01-02", filter.DateFrom); err != nil {
		return filter, validationError("Từ ngày không hợp lệ", Record{"date_from": filter.DateFrom})
	}
	if _, err := time.Parse("2006-01-02", filter.DateTo); err != nil {
		return filter, validationError("Đến ngày không hợp lệ", Record{"date_to": filter.DateTo})
	}
	if filter.DateTo < filter.DateFrom {
		return filter, validationError("Đến ngày phải từ hoặc sau Từ ngày")
	}
	return filter, nil
}

func (r *Repository) accountingReportScope(ctx context.Context, userID string, filter AccountingReportFilter) (string, []any, error) {
	scopeSQL, args, err := r.ProjectAccessFilter(ctx, userID, "project", 1)
	if err != nil {
		return "", nil, err
	}
	where := []string{"project.status <> 'cancelled'", scopeSQL}
	if filter.ProjectID != "" {
		args = append(args, filter.ProjectID)
		where = append(where, fmt.Sprintf("project.id=$%d::uuid", len(args)))
	}
	return strings.Join(where, " AND "), args, nil
}

func (r *Repository) AccountingTrialBalance(ctx context.Context, userID string, filter AccountingReportFilter) (Record, error) {
	where, args, err := r.accountingReportScope(ctx, userID, filter)
	if err != nil {
		return nil, err
	}
	args = append(args, filter.DateFrom, filter.DateTo)
	fromIdx := len(args) - 1
	toIdx := len(args)
	rows, err := r.Query(ctx, fmt.Sprintf(`
		WITH balances AS (
			SELECT account.id,account.account_code,account.name,account.account_type,account.normal_balance,
				COALESCE(SUM(line.debit_amount) FILTER (WHERE entry.entry_date < $%d::date),0) AS opening_debit_turnover,
				COALESCE(SUM(line.credit_amount) FILTER (WHERE entry.entry_date < $%d::date),0) AS opening_credit_turnover,
				COALESCE(SUM(line.debit_amount) FILTER (WHERE entry.entry_date BETWEEN $%d::date AND $%d::date),0) AS period_debit,
				COALESCE(SUM(line.credit_amount) FILTER (WHERE entry.entry_date BETWEEN $%d::date AND $%d::date),0) AS period_credit,
				COALESCE(SUM(line.debit_amount-line.credit_amount) FILTER (WHERE entry.entry_date <= $%d::date),0) AS closing_net
			FROM accounting_accounts account
			JOIN journal_entry_lines line ON line.account_id=account.id
			JOIN journal_entries entry ON entry.id=line.entry_id AND entry.status='posted'
			JOIN projects project ON project.id=entry.project_id
			WHERE %s
			GROUP BY account.id
		)
		SELECT *,
			GREATEST(opening_debit_turnover-opening_credit_turnover,0) AS opening_debit,
			GREATEST(opening_credit_turnover-opening_debit_turnover,0) AS opening_credit,
			GREATEST(closing_net,0) AS closing_debit,
			GREATEST(-closing_net,0) AS closing_credit
		FROM balances
		WHERE opening_debit_turnover <> 0 OR opening_credit_turnover <> 0 OR period_debit <> 0 OR period_credit <> 0
		ORDER BY account_code
	`, fromIdx, fromIdx, fromIdx, toIdx, fromIdx, toIdx, toIdx, where), args...)
	if err != nil {
		return nil, err
	}
	summary := Record{"date_from": filter.DateFrom, "date_to": filter.DateTo, "project_id": filter.ProjectID, "account_count": len(rows)}
	for _, key := range []string{"opening_debit", "opening_credit", "period_debit", "period_credit", "closing_debit", "closing_credit"} {
		var total float64
		for _, row := range rows {
			total += number(row[key])
		}
		summary[key] = roundAccountingAmount(total)
	}
	return Record{"summary": summary, "accounts": rows}, nil
}

func (r *Repository) AccountingGeneralLedger(ctx context.Context, userID string, filter AccountingReportFilter) (Record, error) {
	where, args, err := r.accountingReportScope(ctx, userID, filter)
	if err != nil {
		return nil, err
	}
	args = append(args, filter.DateFrom, filter.DateTo)
	fromIdx := len(args) - 1
	toIdx := len(args)
	rows, err := r.Query(ctx, fmt.Sprintf(`
		SELECT entry.entry_date,entry.entry_no,entry.description AS entry_description,
			entry.source_type,entry.source_id,entry.source_event,project.id AS project_id,
			project.code AS project_code,project.name AS project_name,
			account.id AS account_id,account.account_code,account.name AS account_name,
			line.line_no,line.description,line.debit_amount,line.credit_amount,
			SUM(line.debit_amount-line.credit_amount) OVER (
				PARTITION BY account.id ORDER BY entry.entry_date,entry.entry_no,line.line_no
			) AS period_running_net
		FROM journal_entries entry
		JOIN journal_entry_lines line ON line.entry_id=entry.id
		JOIN accounting_accounts account ON account.id=line.account_id
		JOIN projects project ON project.id=entry.project_id
		WHERE entry.status='posted' AND entry.entry_date BETWEEN $%d::date AND $%d::date AND %s
		ORDER BY account.account_code,entry.entry_date,entry.entry_no,line.line_no
	`, fromIdx, toIdx, where), args...)
	if err != nil {
		return nil, err
	}
	return Record{"summary": Record{"date_from": filter.DateFrom, "date_to": filter.DateTo, "project_id": filter.ProjectID, "line_count": len(rows)}, "lines": rows}, nil
}

func (r *Repository) AccountingProfitLoss(ctx context.Context, userID string, filter AccountingReportFilter) (Record, error) {
	where, args, err := r.accountingReportScope(ctx, userID, filter)
	if err != nil {
		return nil, err
	}
	args = append(args, filter.DateFrom, filter.DateTo)
	fromIdx := len(args) - 1
	toIdx := len(args)
	rows, err := r.Query(ctx, fmt.Sprintf(`
		SELECT account.account_code,account.name,account.account_type,
			COALESCE(SUM(CASE WHEN account.account_type='revenue' THEN line.credit_amount-line.debit_amount ELSE 0 END),0) AS revenue_amount,
			COALESCE(SUM(CASE WHEN account.account_type='expense' THEN line.debit_amount-line.credit_amount ELSE 0 END),0) AS expense_amount
		FROM journal_entries entry
		JOIN journal_entry_lines line ON line.entry_id=entry.id
		JOIN accounting_accounts account ON account.id=line.account_id
		JOIN projects project ON project.id=entry.project_id
		WHERE entry.status='posted' AND account.account_type IN ('revenue','expense')
		  AND entry.entry_date BETWEEN $%d::date AND $%d::date AND %s
		GROUP BY account.id
		ORDER BY account.account_type DESC,account.account_code
	`, fromIdx, toIdx, where), args...)
	if err != nil {
		return nil, err
	}
	var revenue float64
	var expense float64
	for _, row := range rows {
		revenue += number(row["revenue_amount"])
		expense += number(row["expense_amount"])
	}
	profit := revenue - expense
	margin := float64(0)
	if revenue != 0 {
		margin = profit * 100 / revenue
	}
	return Record{
		"summary": Record{"date_from": filter.DateFrom, "date_to": filter.DateTo, "project_id": filter.ProjectID,
			"revenue_amount": roundAccountingAmount(revenue), "expense_amount": roundAccountingAmount(expense),
			"profit_amount": roundAccountingAmount(profit), "profit_margin_percent": roundAccountingAmount(margin)},
		"accounts": rows,
	}, nil
}

func (r *Repository) AccountingVATLedger(ctx context.Context, userID string, filter AccountingReportFilter) (Record, error) {
	where, args, err := r.accountingReportScope(ctx, userID, filter)
	if err != nil {
		return nil, err
	}
	args = append(args, filter.DateFrom, filter.DateTo)
	fromIdx := len(args) - 1
	toIdx := len(args)
	rows, err := r.Query(ctx, fmt.Sprintf(`
		WITH tax_rows AS (
			SELECT tax_line.id,tax_line.project_id,project.code AS project_code,project.name AS project_name,
				'input'::text AS direction,input.invoice_date,input.invoice_number,input.vendor_name AS counterparty_name,
				tax.tax_code,tax.name AS tax_name,tax.vat_rate,tax_line.taxable_amount,tax_line.vat_amount,
				tax_line.deductible_amount,tax_line.nondeductible_amount,input.status,
				original.entry_no AS journal_entry_no,reversal.entry_no AS reversal_entry_no
			FROM invoice_tax_lines tax_line
			JOIN input_invoices input ON input.id=tax_line.input_invoice_id
			JOIN projects project ON project.id=tax_line.project_id
			JOIN tax_codes tax ON tax.id=tax_line.tax_code_id
			JOIN journal_entries original ON original.source_type='input_invoice' AND original.source_id=input.id AND original.source_event='input_invoice_verified' AND original.status='posted'
			LEFT JOIN journal_entries reversal ON reversal.reversal_of_id=original.id AND reversal.status='posted'
			WHERE input.invoice_date BETWEEN $%d::date AND $%d::date AND reversal.id IS NULL AND %s
			UNION ALL
			SELECT tax_line.id,tax_line.project_id,project.code,project.name,
				'output'::text,output.invoice_date,output.invoice_number,output.client_name,
				tax.tax_code,tax.name,tax.vat_rate,tax_line.taxable_amount,tax_line.vat_amount,
				0::numeric,tax_line.vat_amount,output.status,
				original.entry_no,reversal.entry_no
			FROM invoice_tax_lines tax_line
			JOIN output_invoices output ON output.id=tax_line.output_invoice_id
			JOIN projects project ON project.id=tax_line.project_id
			JOIN tax_codes tax ON tax.id=tax_line.tax_code_id
			JOIN journal_entries original ON original.source_type='output_invoice' AND original.source_id=output.id AND original.source_event='output_invoice_issued' AND original.status='posted'
			LEFT JOIN journal_entries reversal ON reversal.reversal_of_id=original.id AND reversal.status='posted'
			WHERE output.invoice_date BETWEEN $%d::date AND $%d::date AND reversal.id IS NULL AND %s
		)
		SELECT * FROM tax_rows ORDER BY invoice_date DESC,invoice_number
	`, fromIdx, toIdx, where, fromIdx, toIdx, where), args...)
	if err != nil {
		return nil, err
	}
	var inputVAT float64
	var deductibleVAT float64
	var outputVAT float64
	for _, row := range rows {
		if fmt.Sprint(row["direction"]) == "input" {
			inputVAT += number(row["vat_amount"])
			deductibleVAT += number(row["deductible_amount"])
		} else {
			outputVAT += number(row["vat_amount"])
		}
	}
	return Record{
		"summary": Record{"date_from": filter.DateFrom, "date_to": filter.DateTo, "project_id": filter.ProjectID,
			"invoice_count": len(rows), "input_vat_amount": roundAccountingAmount(inputVAT),
			"deductible_input_vat_amount": roundAccountingAmount(deductibleVAT), "output_vat_amount": roundAccountingAmount(outputVAT),
			"net_vat_payable": roundAccountingAmount(outputVAT - deductibleVAT)},
		"ledger": rows,
	}, nil
}

func (r *Repository) AccountingBackfillIssues(ctx context.Context, userID, projectID string) ([]Record, error) {
	scopeSQL, args, err := r.ProjectAccessFilter(ctx, userID, "project", 1)
	if err != nil {
		return nil, err
	}
	projectFilter := ""
	if projectID != "" {
		args = append(args, projectID)
		projectFilter = fmt.Sprintf(" AND issue.project_id=$%d::uuid", len(args))
	}
	return r.Query(ctx, fmt.Sprintf(`
		SELECT issue.*,project.code AS project_code,project.name AS project_name
		FROM accounting_backfill_issues issue
		JOIN projects project ON project.id=issue.project_id
		WHERE issue.status='open' AND project.status <> 'cancelled' AND %s%s
		ORDER BY issue.created_at DESC
	`, scopeSQL, projectFilter), args...)
}

func (r *Repository) ResolveAccountingBackfillIssue(ctx context.Context, id, action, note, journalEntryID, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	issue, err := queryOneRecord(ctx, tx, `
		SELECT issue.*,project.name AS project_name
		FROM accounting_backfill_issues issue
		JOIN projects project ON project.id=issue.project_id
		WHERE issue.id=$1
		FOR UPDATE OF issue
	`, id)
	if err != nil {
		return nil, err
	}
	if fmt.Sprint(issue["status"]) != "open" {
		return nil, conflictError("Đối chiếu này đã được xử lý", Record{"status": issue["status"]})
	}

	status := "accepted"
	var journalID any
	if action == "resolve" {
		status = "resolved"
		var journalProjectID, journalStatus string
		if err := tx.QueryRowContext(ctx, `SELECT project_id::text,status FROM journal_entries WHERE id=$1`, journalEntryID).Scan(&journalProjectID, &journalStatus); err != nil {
			if err == sql.ErrNoRows {
				return nil, validationError("Không tìm thấy Chứng từ ghi sổ bổ sung")
			}
			return nil, err
		}
		if journalProjectID != cleanID(issue["project_id"]) || journalStatus != "posted" {
			return nil, validationError("Chứng từ ghi sổ bổ sung phải thuộc cùng công trình và đã ghi sổ")
		}
		journalID = journalEntryID
	}

	row, err := queryOneRecord(ctx, tx, `
		UPDATE accounting_backfill_issues
		SET status=$2,resolved_by=$3,resolved_at=now(),resolution_note=$4,resolution_journal_entry_id=$5
		WHERE id=$1
		RETURNING *
	`, id, status, actorID, note, journalID)
	if err != nil {
		return nil, err
	}
	projectID := cleanID(issue["project_id"])
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "accounting_reconciliation_"+status, "accounting_backfill_issue", id, "Đã xử lý đối chiếu chuyển đổi sổ kế toán", Record{
		"source_type": issue["source_type"], "source_id": issue["source_id"], "source_code": issue["source_code"],
		"resolution_note": note, "journal_entry_id": journalEntryID,
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return row, nil
}
