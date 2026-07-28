package core

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"strings"
	"time"
)

type accountingJournalLine struct {
	AccountID    string
	PartnerID    string
	CostCode     string
	Description  string
	DebitAmount  float64
	CreditAmount float64
}

func (s *Service) AccountingAccounts(ctx context.Context, projectID string) ([]Record, error) {
	projectID = cleanID(projectID)
	if projectID == "" {
		return nil, validationError("Công trình là bắt buộc")
	}
	return s.repo.AccountingAccounts(ctx, projectID)
}

func (s *Service) AccountingPeriods(ctx context.Context) ([]Record, error) {
	return s.repo.AccountingPeriods(ctx)
}

func (s *Service) AccountingTaxCodes(ctx context.Context, projectID string) ([]Record, error) {
	projectID = cleanID(projectID)
	if projectID == "" {
		return nil, validationError("Công trình là bắt buộc")
	}
	return s.repo.AccountingTaxCodes(ctx, projectID)
}

func (s *Service) AccountingTaxPeriods(ctx context.Context) ([]Record, error) {
	return s.repo.AccountingTaxPeriods(ctx)
}

func (s *Service) AccountingJournalEntries(ctx context.Context, userID string, filter AccountingJournalFilter) ([]Record, error) {
	filter.ProjectID = cleanID(filter.ProjectID)
	filter.AccountID = cleanID(filter.AccountID)
	if filter.DateFrom != "" {
		if _, err := time.Parse("2006-01-02", filter.DateFrom); err != nil {
			return nil, validationError("Từ ngày không hợp lệ", Record{"date_from": filter.DateFrom})
		}
	}
	if filter.DateTo != "" {
		if _, err := time.Parse("2006-01-02", filter.DateTo); err != nil {
			return nil, validationError("Đến ngày không hợp lệ", Record{"date_to": filter.DateTo})
		}
	}
	if filter.DateFrom != "" && filter.DateTo != "" && filter.DateTo < filter.DateFrom {
		return nil, validationError("Đến ngày phải từ hoặc sau Từ ngày")
	}
	return s.repo.AccountingJournalEntries(ctx, userID, filter)
}

func (s *Service) AccountingJournalEntry(ctx context.Context, id string) (Record, error) {
	return s.repo.AccountingJournalEntry(ctx, cleanID(id))
}

func (s *Service) CreateAccountingJournal(ctx context.Context, projectID string, req AccountingJournalCreateRequest, actorID, actorName string) (Record, error) {
	projectID = cleanID(projectID)
	if projectID == "" {
		return nil, validationError("Công trình là bắt buộc")
	}
	entryDate, err := accountingDate(req.EntryDate, time.Now())
	if err != nil {
		return nil, validationError("Ngày hạch toán không hợp lệ", Record{"entry_date": req.EntryDate})
	}
	documentDate := ""
	if strings.TrimSpace(req.DocumentDate) != "" {
		parsed, err := time.Parse("2006-01-02", strings.TrimSpace(req.DocumentDate))
		if err != nil {
			return nil, validationError("Ngày chứng từ không hợp lệ", Record{"document_date": req.DocumentDate})
		}
		documentDate = parsed.Format("2006-01-02")
	}
	description := strings.TrimSpace(req.Description)
	if description == "" {
		return nil, validationError("Diễn giải là bắt buộc")
	}
	if len(req.Lines) < 2 {
		return nil, validationError("Bút toán phải có ít nhất hai dòng định khoản")
	}
	lines := make([]accountingJournalLine, 0, len(req.Lines))
	var debitTotal float64
	var creditTotal float64
	for index, line := range req.Lines {
		accountID := cleanID(line.AccountID)
		if accountID == "" {
			return nil, validationError("Tài khoản là bắt buộc", Record{"line": index + 1})
		}
		debit := roundAccountingAmount(line.DebitAmount)
		credit := roundAccountingAmount(line.CreditAmount)
		if (debit <= 0 && credit <= 0) || (debit > 0 && credit > 0) {
			return nil, validationError("Mỗi dòng chỉ được ghi bên Nợ hoặc bên Có", Record{"line": index + 1})
		}
		debitTotal += debit
		creditTotal += credit
		lines = append(lines, accountingJournalLine{
			AccountID: accountID, PartnerID: cleanID(line.PartnerID), CostCode: strings.TrimSpace(line.CostCode),
			Description: strings.TrimSpace(line.Description), DebitAmount: debit, CreditAmount: credit,
		})
	}
	if math.Abs(debitTotal-creditTotal) > 0.005 || debitTotal <= 0 {
		return nil, validationError("Bút toán chưa cân", Record{"debit_total": roundAccountingAmount(debitTotal), "credit_total": roundAccountingAmount(creditTotal)})
	}
	return s.repo.CreateAccountingJournal(ctx, projectID, entryDate, documentDate, strings.TrimSpace(req.SourceDocumentNo), description, lines, actorID, actorName)
}

func (s *Service) ReverseAccountingJournal(ctx context.Context, id string, req AccountingJournalReverseRequest, actorID, actorName string) (Record, error) {
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		return nil, validationError("Lý do lập bút toán đảo là bắt buộc")
	}
	entryDate, err := accountingDate(req.EntryDate, time.Now())
	if err != nil {
		return nil, validationError("Ngày hạch toán không hợp lệ", Record{"entry_date": req.EntryDate})
	}
	return s.repo.ReverseAccountingJournal(ctx, cleanID(id), entryDate, reason, actorID, actorName)
}

func (s *Service) AccountingPeriodAction(ctx context.Context, id, action string, req AccountingPeriodActionRequest, actorID, actorName string) (Record, error) {
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		return nil, validationError("Lý do khóa hoặc mở lại kỳ kế toán là bắt buộc")
	}
	if action != "close" && action != "reopen" {
		return nil, validationError("Hành động kỳ kế toán không hợp lệ", Record{"action": action})
	}
	return s.repo.AccountingPeriodAction(ctx, cleanID(id), action, reason, actorID, actorName)
}

func accountingDate(value string, fallback time.Time) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback.Format("2006-01-02"), nil
	}
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return "", err
	}
	return parsed.Format("2006-01-02"), nil
}

func roundAccountingAmount(value float64) float64 {
	return math.Round(value*100) / 100
}

func (r *Repository) AccountingAccounts(ctx context.Context, projectID string) ([]Record, error) {
	return r.Query(ctx, `
		SELECT a.id,a.book_id,a.account_code,a.name,a.account_type,a.normal_balance,a.is_postable
		FROM projects p
		JOIN accounting_accounts a ON a.book_id=p.accounting_book_id
		WHERE p.id=$1 AND p.status <> 'cancelled' AND a.active
		ORDER BY a.account_code
	`, projectID)
}

func (r *Repository) AccountingPeriods(ctx context.Context) ([]Record, error) {
	return r.Query(ctx, `
		SELECT period.*,book.book_code,book.name AS book_name
		FROM accounting_periods period
		JOIN accounting_books book ON book.id=period.book_id
		WHERE book.active
		ORDER BY period.fiscal_year DESC,period.period_no DESC
	`)
}

func (r *Repository) AccountingTaxCodes(ctx context.Context, projectID string) ([]Record, error) {
	return r.Query(ctx, `
		SELECT tax.id,tax.tax_code,tax.name,tax.direction,tax.vat_rate,tax.deductible_rate,
			input_account.account_code AS input_account_code,
			output_account.account_code AS output_account_code
		FROM projects project
		JOIN tax_codes tax ON tax.book_id=project.accounting_book_id
		LEFT JOIN accounting_accounts input_account ON input_account.id=tax.input_account_id
		LEFT JOIN accounting_accounts output_account ON output_account.id=tax.output_account_id
		WHERE project.id=$1 AND project.status <> 'cancelled' AND tax.active
		  AND tax.effective_from <= CURRENT_DATE
		  AND (tax.effective_to IS NULL OR tax.effective_to >= CURRENT_DATE)
		ORDER BY tax.vat_rate,tax.tax_code
	`, projectID)
}

func (r *Repository) AccountingTaxPeriods(ctx context.Context) ([]Record, error) {
	return r.Query(ctx, `
		SELECT period.*,book.book_code,book.name AS book_name
		FROM tax_periods period
		JOIN accounting_books book ON book.id=period.book_id
		WHERE book.active
		ORDER BY period.fiscal_year DESC,period.period_no DESC
	`)
}

func (r *Repository) AccountingJournalEntries(ctx context.Context, userID string, filter AccountingJournalFilter) ([]Record, error) {
	scopeSQL, args, err := r.ProjectAccessFilter(ctx, userID, "project", 1)
	if err != nil {
		return nil, err
	}
	where := []string{"entry.status='posted'", "project.status <> 'cancelled'", scopeSQL}
	if filter.ProjectID != "" {
		args = append(args, filter.ProjectID)
		where = append(where, fmt.Sprintf("entry.project_id=$%d::uuid", len(args)))
	}
	if filter.DateFrom != "" {
		args = append(args, filter.DateFrom)
		where = append(where, fmt.Sprintf("entry.entry_date >= $%d::date", len(args)))
	}
	if filter.DateTo != "" {
		args = append(args, filter.DateTo)
		where = append(where, fmt.Sprintf("entry.entry_date <= $%d::date", len(args)))
	}
	if filter.AccountID != "" {
		args = append(args, filter.AccountID)
		where = append(where, fmt.Sprintf("EXISTS (SELECT 1 FROM journal_entry_lines filter_line WHERE filter_line.entry_id=entry.id AND filter_line.account_id=$%d::uuid)", len(args)))
	}
	return r.Query(ctx, fmt.Sprintf(`
		SELECT entry.*,project.code AS project_code,project.name AS project_name,period.period_no,period.fiscal_year,
			COALESCE(total.debit_total,0) AS debit_total,
			COALESCE(total.credit_total,0) AS credit_total,
			(reversal.id IS NOT NULL) AS reversed,
			reversal.id AS reversal_entry_id,reversal.entry_no AS reversal_entry_no
		FROM journal_entries entry
		JOIN projects project ON project.id=entry.project_id
		JOIN accounting_periods period ON period.id=entry.period_id
		LEFT JOIN LATERAL (
			SELECT SUM(debit_amount) AS debit_total,SUM(credit_amount) AS credit_total
			FROM journal_entry_lines line WHERE line.entry_id=entry.id
		) total ON true
		LEFT JOIN journal_entries reversal ON reversal.reversal_of_id=entry.id AND reversal.status='posted'
		WHERE %s
		ORDER BY entry.entry_date DESC,entry.entry_no DESC
		LIMIT 500
	`, strings.Join(where, " AND ")), args...)
}

func (r *Repository) AccountingJournalEntry(ctx context.Context, id string) (Record, error) {
	entry, err := r.QueryOne(ctx, `
		SELECT entry.*,project.code AS project_code,project.name AS project_name,period.period_no,period.fiscal_year,
			(reversal.id IS NOT NULL) AS reversed,reversal.id AS reversal_entry_id,reversal.entry_no AS reversal_entry_no,
			original.entry_no AS original_entry_no
		FROM journal_entries entry
		JOIN projects project ON project.id=entry.project_id
		JOIN accounting_periods period ON period.id=entry.period_id
		LEFT JOIN journal_entries reversal ON reversal.reversal_of_id=entry.id AND reversal.status='posted'
		LEFT JOIN journal_entries original ON original.id=entry.reversal_of_id
		WHERE entry.id=$1 AND entry.status='posted'
	`, id)
	if err != nil {
		return nil, err
	}
	lines, err := r.Query(ctx, `
		SELECT line.*,account.account_code,account.name AS account_name,partner.name AS partner_name
		FROM journal_entry_lines line
		JOIN accounting_accounts account ON account.id=line.account_id
		LEFT JOIN partners partner ON partner.id=line.partner_id
		WHERE line.entry_id=$1
		ORDER BY line.line_no
	`, id)
	if err != nil {
		return nil, err
	}
	entry["lines"] = lines
	return entry, nil
}

func (r *Repository) CreateAccountingJournal(
	ctx context.Context,
	projectID, entryDate, documentDate, sourceDocumentNo, description string,
	lines []accountingJournalLine,
	actorID, actorName string,
) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	project, period, err := accountingPostingContextTx(ctx, tx, projectID, entryDate)
	if err != nil {
		return nil, err
	}
	bookID := cleanID(project["accounting_book_id"])
	if err := validateAccountingAccountsTx(ctx, tx, bookID, lines); err != nil {
		return nil, err
	}
	entryID := newID()
	entryNo, err := nextAccountingEntryNoTx(ctx, tx, bookID, entryDate)
	if err != nil {
		return nil, err
	}
	sourceID := newID()
	rows, err := queryTx(ctx, tx, `
		INSERT INTO journal_entries (
			id,book_id,project_id,period_id,entry_no,entry_date,document_date,description,source_document_no,
			source_type,source_id,source_event,status,created_by
		) VALUES ($1,$2,$3,$4,$5,$6::date,NULLIF($7,'')::date,$8,NULLIF($9,''),'manual_adjustment',$10,'manual_post','draft',NULLIF($11,'')::uuid)
		RETURNING *
	`, entryID, bookID, projectID, period["id"], entryNo, entryDate, documentDate, description, sourceDocumentNo, sourceID, actorID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	for index, line := range lines {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO journal_entry_lines (
				entry_id,book_id,line_no,account_id,partner_id,cost_code,description,debit_amount,credit_amount
			) VALUES ($1,$2,$3,$4,NULLIF($5,'')::uuid,NULLIF($6,''),NULLIF($7,''),$8,$9)
		`, entryID, bookID, index+1, line.AccountID, line.PartnerID, line.CostCode, line.Description, line.DebitAmount, line.CreditAmount); err != nil {
			return nil, err
		}
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE journal_entries SET status='posted',posted_by=NULLIF($2,'')::uuid,posted_at=now(),updated_at=now()
		WHERE id=$1 AND status='draft'
	`, entryID, actorID); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "journal_posted", "journal_entry", entryID, "Ghi sổ chứng từ kế toán", Record{
		"entry_no": entryNo, "entry_date": entryDate, "source_document_no": sourceDocumentNo, "description": description,
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.AccountingJournalEntry(ctx, entryID)
}

func (r *Repository) ReverseAccountingJournal(ctx context.Context, entryID, entryDate, reason, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	entries, err := queryTx(ctx, tx, `SELECT * FROM journal_entries WHERE id=$1 AND status='posted' FOR UPDATE`, entryID)
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return nil, sql.ErrNoRows
	}
	original := entries[0]
	existing, err := queryTx(ctx, tx, `SELECT id FROM journal_entries WHERE reversal_of_id=$1 AND status='posted'`, entryID)
	if err != nil {
		return nil, err
	}
	if len(existing) > 0 {
		return nil, validationError("Bút toán đã được đảo", Record{"reversal_entry_id": existing[0]["id"]})
	}
	_, period, err := accountingPostingContextTx(ctx, tx, cleanID(original["project_id"]), entryDate)
	if err != nil {
		return nil, err
	}
	bookID := cleanID(original["book_id"])
	reversalID := newID()
	reversalNo, err := nextAccountingEntryNoTx(ctx, tx, bookID, entryDate)
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO journal_entries (
			id,book_id,project_id,period_id,entry_no,entry_date,document_date,description,
			source_type,source_id,source_event,status,reversal_of_id,created_by
		) VALUES ($1,$2,$3,$4,$5,$6::date,$7::date,$8,'journal_entry',$9,'reversal','draft',$9,NULLIF($10,'')::uuid)
	`, reversalID, bookID, original["project_id"], period["id"], reversalNo, entryDate, original["document_date"], "Đảo "+fmt.Sprint(original["entry_no"])+": "+reason, entryID, actorID); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO journal_entry_lines (
			entry_id,book_id,line_no,account_id,partner_id,cost_code,description,debit_amount,credit_amount
		)
		SELECT $1,book_id,line_no,account_id,partner_id,cost_code,
			COALESCE(description,'') || CASE WHEN description IS NULL OR description='' THEN '' ELSE ' · ' END || 'Bút toán đảo',
			credit_amount,debit_amount
		FROM journal_entry_lines WHERE entry_id=$2 ORDER BY line_no
	`, reversalID, entryID); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE journal_entries SET status='posted',posted_by=NULLIF($2,'')::uuid,posted_at=now(),updated_at=now()
		WHERE id=$1 AND status='draft'
	`, reversalID, actorID); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO budget_movements (
			budget_id,project_id,movement_type,commitment_delta,actual_delta,paid_delta,
			source_type,source_id,source_event,journal_entry_id,reversal_of_id,description,occurred_at,created_by
		)
		SELECT movement.budget_id,movement.project_id,'reversal',-movement.commitment_delta,-movement.actual_delta,-movement.paid_delta,
			'journal_entry',$1,'reversal:' || movement.id::text,$2,movement.id,'Đảo biến động ngân sách theo ' || $3,now(),NULLIF($4,'')::uuid
		FROM budget_movements movement
		WHERE movement.journal_entry_id=$1
		  AND NOT EXISTS (SELECT 1 FROM budget_movements reversed WHERE reversed.reversal_of_id=movement.id)
	`, entryID, reversalID, fmt.Sprint(original["entry_no"]), actorID); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, cleanID(original["project_id"]), actorID, actorName, "journal_reversed", "journal_entry", reversalID, "Lập bút toán đảo", Record{
		"original_entry_id": entryID, "original_entry_no": original["entry_no"], "reversal_entry_no": reversalNo, "reason": reason,
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.AccountingJournalEntry(ctx, reversalID)
}

func (r *Repository) AccountingPeriodAction(ctx context.Context, periodID, action, reason, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	periods, err := queryTx(ctx, tx, `SELECT * FROM accounting_periods WHERE id=$1 FOR UPDATE`, periodID)
	if err != nil {
		return nil, err
	}
	if len(periods) == 0 {
		return nil, sql.ErrNoRows
	}
	period := periods[0]
	fromStatus := fmt.Sprint(period["status"])
	var rows []Record
	if action == "close" {
		if fromStatus != "open" {
			return nil, validationError("Kỳ kế toán đã khóa", Record{"status": fromStatus})
		}
		var draftCount int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM journal_entries WHERE period_id=$1 AND status='draft'`, periodID).Scan(&draftCount); err != nil {
			return nil, err
		}
		if draftCount > 0 {
			return nil, validationError("Kỳ kế toán còn chứng từ chưa ghi sổ", Record{"draft_journal_count": draftCount})
		}
		rows, err = queryTx(ctx, tx, `
			UPDATE accounting_periods
			SET status='closed',closed_by=NULLIF($2,'')::uuid,closed_at=now(),close_reason=$3,updated_at=now()
			WHERE id=$1 AND status='open' RETURNING *
		`, periodID, actorID, reason)
	} else {
		if fromStatus != "closed" {
			return nil, validationError("Kỳ kế toán đang mở", Record{"status": fromStatus})
		}
		rows, err = queryTx(ctx, tx, `
			UPDATE accounting_periods
			SET status='open',reopened_by=NULLIF($2,'')::uuid,reopened_at=now(),reopen_reason=$3,updated_at=now()
			WHERE id=$1 AND status='closed' RETURNING *
		`, periodID, actorID, reason)
	}
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, validationError("Trạng thái kỳ kế toán đã thay đổi, vui lòng tải lại")
	}
	event := "accounting_period_closed"
	description := "Khóa sổ kỳ kế toán"
	if action == "reopen" {
		event = "accounting_period_reopened"
		description = "Mở lại kỳ kế toán"
	}
	if err := addAuditTx(ctx, tx, "", actorID, actorName, event, "accounting_period", periodID, description, Record{
		"fiscal_year": period["fiscal_year"], "period_no": period["period_no"], "reason": reason,
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return rows[0], nil
}

func accountingPostingContextTx(ctx context.Context, tx *sql.Tx, projectID, entryDate string) (Record, Record, error) {
	projects, err := queryTx(ctx, tx, `
		SELECT id,accounting_book_id FROM projects WHERE id=$1 AND status <> 'cancelled' FOR SHARE
	`, projectID)
	if err != nil {
		return nil, nil, err
	}
	if len(projects) == 0 {
		return nil, nil, sql.ErrNoRows
	}
	periods, err := queryTx(ctx, tx, `
		SELECT * FROM accounting_periods
		WHERE book_id=$1 AND $2::date BETWEEN date_from AND date_to
		FOR SHARE
	`, projects[0]["accounting_book_id"], entryDate)
	if err != nil {
		return nil, nil, err
	}
	if len(periods) == 0 {
		return nil, nil, validationError("Chưa thiết lập kỳ kế toán cho ngày hạch toán", Record{"entry_date": entryDate})
	}
	if fmt.Sprint(periods[0]["status"]) != "open" {
		return nil, nil, validationError("Kỳ kế toán đã khóa", Record{"period_id": periods[0]["id"], "entry_date": entryDate})
	}
	return projects[0], periods[0], nil
}

func validateAccountingAccountsTx(ctx context.Context, tx *sql.Tx, bookID string, lines []accountingJournalLine) error {
	for index, line := range lines {
		var valid bool
		if err := tx.QueryRowContext(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM accounting_accounts
				WHERE id=$1 AND book_id=$2 AND active AND is_postable
			)
		`, line.AccountID, bookID).Scan(&valid); err != nil {
			return err
		}
		if !valid {
			return validationError("Tài khoản không hợp lệ hoặc không được phép hạch toán", Record{"line": index + 1, "account_id": line.AccountID})
		}
	}
	return nil
}

func nextAccountingEntryNoTx(ctx context.Context, tx *sql.Tx, bookID, entryDate string) (string, error) {
	parsed, err := time.Parse("2006-01-02", entryDate)
	if err != nil {
		return "", err
	}
	prefix := "CTGS-" + parsed.Format("200601")
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "journal:"+bookID+":"+prefix); err != nil {
		return "", err
	}
	var sequence int
	if err := tx.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(NULLIF(regexp_replace(entry_no,'^.*-','','g'),'')::integer),0)+1
		FROM journal_entries
		WHERE book_id=$1 AND entry_no LIKE $2
	`, bookID, prefix+"-%").Scan(&sequence); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s-%06d", prefix, sequence), nil
}
