package core

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"strings"
	"time"
)

func (s *Service) AccountingTaxPeriod(ctx context.Context, id string) (Record, error) {
	return s.repo.AccountingTaxPeriod(ctx, cleanID(id))
}

func (s *Service) AccountingTaxPeriodAction(ctx context.Context, id, action string, req AccountingTaxPeriodActionRequest, actorID, actorName string) (Record, error) {
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		return nil, validationError("Căn cứ xử lý kỳ thuế GTGT là bắt buộc")
	}
	if !statusIn(action, "prepare", "file", "lock", "start_amendment") {
		return nil, validationError("Hành động kỳ thuế GTGT không hợp lệ", Record{"action": action})
	}
	filingReference := strings.TrimSpace(req.FilingReference)
	if action == "file" && filingReference == "" {
		return nil, validationError("Mã giao dịch nộp tờ khai là bắt buộc")
	}
	return s.repo.AccountingTaxPeriodAction(ctx, cleanID(id), action, reason, filingReference, actorID, actorName)
}

func (s *Service) CreateAccountingTaxAdjustment(ctx context.Context, periodID string, req AccountingTaxAdjustmentCreateRequest, actorID, actorName string) (Record, error) {
	req.ProjectID = cleanID(req.ProjectID)
	req.TaxCodeID = cleanID(req.TaxCodeID)
	req.OffsetAccountID = cleanID(req.OffsetAccountID)
	req.Direction = strings.TrimSpace(req.Direction)
	req.Reason = strings.TrimSpace(req.Reason)
	req.SourceDocumentNo = strings.TrimSpace(req.SourceDocumentNo)
	if req.ProjectID == "" || req.TaxCodeID == "" || req.OffsetAccountID == "" {
		return nil, validationError("Công trình, mã thuế và tài khoản đối ứng là bắt buộc")
	}
	if !statusIn(req.Direction, "input", "output") {
		return nil, validationError("Nhóm điều chỉnh thuế phải là đầu vào hoặc đầu ra")
	}
	if req.Reason == "" {
		return nil, validationError("Lý do điều chỉnh thuế là bắt buộc")
	}
	if math.Abs(req.TaxableDelta) < 0.005 && math.Abs(req.VATDelta) < 0.005 {
		return nil, validationError("Số điều chỉnh giá trị chưa thuế hoặc tiền thuế phải khác 0")
	}
	entryDate, err := accountingDate(req.EntryDate, time.Now())
	if err != nil {
		return nil, validationError("Ngày hạch toán điều chỉnh thuế không hợp lệ")
	}
	return s.repo.CreateAccountingTaxAdjustment(ctx, cleanID(periodID), req, entryDate, actorID, actorName)
}

func (s *Service) ReverseAccountingTaxAdjustment(ctx context.Context, id string, req AccountingTaxAdjustmentReverseRequest, actorID, actorName string) (Record, error) {
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		return nil, validationError("Lý do đảo điều chỉnh thuế là bắt buộc")
	}
	entryDate, err := accountingDate(req.EntryDate, time.Now())
	if err != nil {
		return nil, validationError("Ngày hạch toán bút toán đảo không hợp lệ")
	}
	return s.repo.ReverseAccountingTaxAdjustment(ctx, cleanID(id), entryDate, reason, actorID, actorName)
}

func (s *Service) CreateAccountingTaxPayment(ctx context.Context, periodID string, req AccountingTaxPaymentCreateRequest, actorID, actorName string) (Record, error) {
	projectID := cleanID(req.ProjectID)
	paymentCode := strings.TrimSpace(req.PaymentCode)
	method := strings.TrimSpace(req.PaymentMethod)
	if projectID == "" || paymentCode == "" || req.Amount <= 0 {
		return nil, validationError("Phiếu nộp thuế cần công trình hạch toán, số phiếu và số tiền lớn hơn 0")
	}
	if method == "" {
		method = "bank_transfer"
	}
	if !statusIn(method, "bank_transfer", "cash") {
		return nil, validationError("Phương thức nộp thuế không hợp lệ")
	}
	defaultCashAccount := "1121"
	if method == "cash" {
		defaultCashAccount = "1111"
	}
	bookID, cashAccountID, err := s.resolveAccountingAccount(ctx, projectID, req.CashAccountID, defaultCashAccount)
	if err != nil {
		return nil, err
	}
	body := Record{
		"project_id": projectID, "accounting_book_id": bookID, "cash_account_id": cashAccountID,
		"payment_code": paymentCode, "payment_type": "tax_payment", "tax_period_id": cleanID(periodID),
		"target_type": "tax_period", "target_id": cleanID(periodID), "amount": roundAccountingAmount(req.Amount),
		"payment_method": method, "status": "draft", "created_by": actorID,
	}
	return s.repo.CreateAccountingTaxPayment(ctx, cleanID(periodID), body, actorID, actorName)
}

func (r *Repository) AccountingTaxPeriod(ctx context.Context, periodID string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	period, err := accountingTaxPeriodTx(ctx, tx, periodID, false)
	if err != nil {
		return nil, err
	}
	summary, issues, err := vatPeriodSummaryTx(ctx, tx, period)
	if err != nil {
		return nil, err
	}
	creditBroughtForward, err := previousVATCreditTx(ctx, tx, period)
	if err != nil {
		return nil, err
	}
	inputCredit := number(summary["deductible_input_vat_amount"]) + number(summary["input_adjustment_vat_amount"]) + creditBroughtForward
	outputLiability := number(summary["output_vat_amount"]) + number(summary["output_adjustment_vat_amount"])
	summary["credit_brought_forward"] = roundAccountingAmount(creditBroughtForward)
	summary["vat_payable"] = roundAccountingAmount(math.Max(outputLiability-inputCredit, 0))
	summary["credit_carried_forward"] = roundAccountingAmount(math.Max(inputCredit-outputLiability, 0))
	purchases, err := vatInvoiceRowsTx(ctx, tx, period, "input")
	if err != nil {
		return nil, err
	}
	sales, err := vatInvoiceRowsTx(ctx, tx, period, "output")
	if err != nil {
		return nil, err
	}
	adjustments, err := queryTx(ctx, tx, `
		SELECT adjustment.*,project.code AS project_code,project.name AS project_name,
			tax.tax_code,tax.name AS tax_name,offset_account.account_code AS offset_account_code,
			offset_account.name AS offset_account_name,journal.entry_no AS journal_entry_no,
		reversal_journal.entry_no AS reversal_journal_entry_no,creator.full_name AS created_by_name,
		COALESCE(creator.deleted_at IS NOT NULL,false) AS created_by_account_deleted
		FROM tax_adjustments adjustment
		LEFT JOIN projects project ON project.id=adjustment.project_id
		JOIN tax_codes tax ON tax.id=adjustment.tax_code_id
		JOIN accounting_accounts offset_account ON offset_account.id=adjustment.offset_account_id
		LEFT JOIN journal_entries journal ON journal.id=adjustment.journal_entry_id
		LEFT JOIN journal_entries reversal_journal ON reversal_journal.reversal_of_id=journal.id AND reversal_journal.status='posted'
		LEFT JOIN users creator ON creator.id=adjustment.created_by
		WHERE adjustment.tax_period_id=$1
		ORDER BY adjustment.created_at DESC,adjustment.id DESC
	`, periodID)
	if err != nil {
		return nil, err
	}
	returns, err := queryTx(ctx, tx, `
		SELECT snapshot.*,prepared.full_name AS prepared_by_name,
			COALESCE(prepared.deleted_at IS NOT NULL,false) AS prepared_by_account_deleted
		FROM vat_return_snapshots snapshot
		LEFT JOIN users prepared ON prepared.id=snapshot.prepared_by
		WHERE snapshot.tax_period_id=$1
		ORDER BY snapshot.amendment_no DESC,snapshot.created_at DESC
	`, periodID)
	if err != nil {
		return nil, err
	}
	events, err := queryTx(ctx, tx, `
		SELECT event.*,COALESCE(actor.deleted_at IS NOT NULL,false) AS actor_account_deleted
		FROM tax_period_events event
		LEFT JOIN users actor ON actor.id=event.actor_id
		WHERE event.tax_period_id=$1 ORDER BY event.created_at DESC,event.id DESC
	`, periodID)
	if err != nil {
		return nil, err
	}
	payments, err := queryTx(ctx, tx, `
		SELECT payment.*,project.code AS project_code,project.name AS project_name,
			journal.id AS journal_entry_id,journal.entry_no AS journal_entry_no,
			creator.full_name AS created_by_name,COALESCE(creator.deleted_at IS NOT NULL,false) AS created_by_account_deleted,
			approver.full_name AS approved_by_name,COALESCE(approver.deleted_at IS NOT NULL,false) AS approved_by_account_deleted
		FROM payments payment
		JOIN projects project ON project.id=payment.project_id
		LEFT JOIN journal_entries journal ON journal.source_type='payment' AND journal.source_id=payment.id
			AND journal.status='posted' AND journal.reversal_of_id IS NULL
		LEFT JOIN users creator ON creator.id=payment.created_by
		LEFT JOIN users approver ON approver.id=payment.approved_by
		WHERE payment.tax_period_id=$1 AND payment.status <> 'deleted'
		ORDER BY payment.created_at DESC,payment.id DESC
	`, periodID)
	if err != nil {
		return nil, err
	}
	period["summary"] = summary
	period["issues"] = issues
	period["purchases"] = purchases
	period["sales"] = sales
	period["adjustments"] = adjustments
	period["returns"] = returns
	period["events"] = events
	period["payments"] = payments
	if len(returns) > 0 {
		period["latest_return"] = returns[0]
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return period, nil
}

func (r *Repository) AccountingTaxPeriodAction(ctx context.Context, periodID, action, reason, filingReference, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	period, err := accountingTaxPeriodTx(ctx, tx, periodID, true)
	if err != nil {
		return nil, err
	}
	fromStatus := fmt.Sprint(period["status"])
	amendmentNo := int(number(period["amendment_no"]))
	toStatus := ""
	eventType := ""
	var snapshotID any
	metadata := Record{"reason": reason, "from_status": fromStatus}

	switch action {
	case "prepare":
		if !statusIn(fromStatus, "open", "amended") {
			return nil, validationError("Chỉ được chốt số liệu khi kỳ thuế đang tập hợp hoặc đang lập bổ sung", Record{"status": fromStatus})
		}
		summary, issues, err := vatPeriodSummaryTx(ctx, tx, period)
		if err != nil {
			return nil, err
		}
		if len(issues) > 0 {
			return nil, validationError("Kỳ thuế còn sai lệch phải xử lý trước khi chốt số liệu", Record{"issues": issues})
		}
		creditBroughtForward, err := previousVATCreditTx(ctx, tx, period)
		if err != nil {
			return nil, err
		}
		inputCredit := number(summary["deductible_input_vat_amount"]) + number(summary["input_adjustment_vat_amount"]) + creditBroughtForward
		outputLiability := number(summary["output_vat_amount"]) + number(summary["output_adjustment_vat_amount"])
		vatPayable := math.Max(outputLiability-inputCredit, 0)
		creditCarried := math.Max(inputCredit-outputLiability, 0)
		snapshot, err := queryOneRecord(ctx, tx, `
			INSERT INTO vat_return_snapshots (
				tax_period_id,book_id,amendment_no,input_invoice_count,output_invoice_count,
				input_vat_amount,deductible_input_vat_amount,output_vat_amount,
				input_adjustment_vat_amount,output_adjustment_vat_amount,adjustment_vat_amount,
				credit_brought_forward,vat_payable,credit_carried_forward,prepared_by
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,NULLIF($15,'')::uuid)
			RETURNING *
		`, periodID, period["book_id"], amendmentNo, summary["input_invoice_count"], summary["output_invoice_count"],
			summary["input_vat_amount"], summary["deductible_input_vat_amount"], summary["output_vat_amount"],
			summary["input_adjustment_vat_amount"], summary["output_adjustment_vat_amount"], summary["adjustment_vat_amount"],
			creditBroughtForward, roundAccountingAmount(vatPayable), roundAccountingAmount(creditCarried), actorID)
		if err != nil {
			return nil, err
		}
		snapshotID = snapshot["id"]
		toStatus, eventType = "prepared", "prepared"
		metadata["snapshot"] = snapshot
	case "file":
		if fromStatus != "prepared" {
			return nil, validationError("Chỉ được nộp tờ khai sau khi đã chốt số liệu", Record{"status": fromStatus})
		}
		if err := tx.QueryRowContext(ctx, `SELECT id FROM vat_return_snapshots WHERE tax_period_id=$1 AND amendment_no=$2`, periodID, amendmentNo).Scan(&snapshotID); err != nil {
			return nil, err
		}
		toStatus, eventType = "filed", "filed"
		metadata["filing_reference"] = filingReference
	case "lock":
		if fromStatus != "filed" {
			return nil, validationError("Chỉ được khóa kỳ sau khi tờ khai đã nộp", Record{"status": fromStatus})
		}
		if err := tx.QueryRowContext(ctx, `SELECT id FROM vat_return_snapshots WHERE tax_period_id=$1 AND amendment_no=$2`, periodID, amendmentNo).Scan(&snapshotID); err != nil {
			return nil, err
		}
		toStatus, eventType = "locked", "locked"
	case "start_amendment":
		if !statusIn(fromStatus, "filed", "locked") {
			return nil, validationError("Chỉ được lập tờ khai bổ sung cho kỳ đã nộp hoặc đã khóa", Record{"status": fromStatus})
		}
		toStatus, eventType = "amended", "amendment_started"
		amendmentNo++
	default:
		return nil, validationError("Hành động kỳ thuế GTGT không hợp lệ")
	}

	updated, err := queryOneRecord(ctx, tx, `
		UPDATE tax_periods
		SET status=$2,amendment_no=$3,
			prepared_by=CASE WHEN $2='prepared' THEN NULLIF($4,'')::uuid WHEN $2='amended' THEN NULL ELSE prepared_by END,
			prepared_at=CASE WHEN $2='prepared' THEN now() WHEN $2='amended' THEN NULL ELSE prepared_at END,
			filed_by=CASE WHEN $2='filed' THEN NULLIF($4,'')::uuid WHEN $2='amended' THEN NULL ELSE filed_by END,
			filed_at=CASE WHEN $2='filed' THEN now() WHEN $2='amended' THEN NULL ELSE filed_at END,
			locked_by=CASE WHEN $2='locked' THEN NULLIF($4,'')::uuid WHEN $2='amended' THEN NULL ELSE locked_by END,
			locked_at=CASE WHEN $2='locked' THEN now() WHEN $2='amended' THEN NULL ELSE locked_at END,
			notes=$5,updated_at=now()
		WHERE id=$1 AND status=$6
		RETURNING *
	`, periodID, toStatus, amendmentNo, actorID, reason, fromStatus)
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO tax_period_events (
			tax_period_id,vat_return_snapshot_id,event_type,from_status,to_status,amendment_no,
			filing_reference,reason,actor_id,actor_name
		) VALUES ($1,$2,$3,$4,$5,$6,NULLIF($7,''),$8,NULLIF($9,'')::uuid,NULLIF($10,''))
	`, periodID, snapshotID, eventType, fromStatus, toStatus, amendmentNo, filingReference, reason, actorID, actorName); err != nil {
		return nil, err
	}
	metadata["to_status"] = toStatus
	metadata["amendment_no"] = amendmentNo
	if err := addAuditTx(ctx, tx, "", actorID, actorName, "tax_period_"+eventType, "tax_period", periodID, taxPeriodEventDescription(eventType), metadata); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	updated["snapshot_id"] = snapshotID
	return updated, nil
}

func (r *Repository) CreateAccountingTaxAdjustment(ctx context.Context, periodID string, req AccountingTaxAdjustmentCreateRequest, entryDate, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	period, err := accountingTaxPeriodTx(ctx, tx, periodID, true)
	if err != nil {
		return nil, err
	}
	if !statusIn(fmt.Sprint(period["status"]), "open", "amended") {
		return nil, validationError("Kỳ thuế đã chốt số liệu; hãy lập tờ khai bổ sung trước khi điều chỉnh", Record{"status": period["status"]})
	}
	if entryDate < fmt.Sprint(period["date_from"])[0:10] || entryDate > fmt.Sprint(period["date_to"])[0:10] {
		return nil, validationError("Ngày hạch toán điều chỉnh phải thuộc kỳ thuế đã chọn")
	}
	dimensions, err := queryOneRecord(ctx, tx, `
		SELECT project.accounting_book_id,tax.input_account_id,tax.output_account_id,
			tax.tax_code,tax.name AS tax_name,offset_account.account_code AS offset_account_code
		FROM projects project
		JOIN tax_codes tax ON tax.id=$2 AND tax.book_id=project.accounting_book_id AND tax.active
		JOIN accounting_accounts offset_account ON offset_account.id=$3 AND offset_account.book_id=project.accounting_book_id
			AND offset_account.active AND offset_account.is_postable
		WHERE project.id=$1 AND project.status <> 'cancelled'
	`, req.ProjectID, req.TaxCodeID, req.OffsetAccountID)
	if err != nil {
		return nil, validationError("Mã thuế hoặc tài khoản đối ứng không thuộc sổ kế toán của công trình")
	}
	if cleanID(dimensions["accounting_book_id"]) != cleanID(period["book_id"]) {
		return nil, validationError("Công trình không thuộc sổ kế toán của kỳ thuế đã chọn")
	}
	taxAccountID := cleanID(dimensions["output_account_id"])
	if req.Direction == "input" {
		taxAccountID = cleanID(dimensions["input_account_id"])
	}
	if taxAccountID == "" || taxAccountID == req.OffsetAccountID {
		return nil, validationError("Mã thuế chưa có tài khoản thuế hợp lệ hoặc trùng tài khoản đối ứng")
	}
	adjustmentID := newID()
	amount := roundAccountingAmount(math.Abs(req.VATDelta))
	description := "Điều chỉnh thuế GTGT " + taxDirectionLabel(req.Direction) + ": " + req.Reason
	lines := taxAdjustmentLines(req.Direction, req.VATDelta, taxAccountID, req.OffsetAccountID, description)
	journal, err := r.postSourceJournalTx(ctx, tx, sourceJournalSpec{
		ProjectID: req.ProjectID, EntryDate: entryDate, DocumentDate: entryDate,
		Description: description, SourceType: "tax_adjustment", SourceID: adjustmentID,
		SourceEvent: "tax_adjustment_posted", Lines: lines,
	}, actorID)
	if err != nil {
		return nil, err
	}
	adjustmentType := "increase"
	if req.VATDelta < 0 {
		adjustmentType = "decrease"
	}
	row, err := queryOneRecord(ctx, tx, `
		INSERT INTO tax_adjustments (
			id,tax_period_id,book_id,project_id,tax_code_id,direction,offset_account_id,entry_date,
			adjustment_type,taxable_delta,vat_delta,reason,source_document_no,journal_entry_id,created_by
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8::date,$9,$10,$11,$12,NULLIF($13,''),$14,NULLIF($15,'')::uuid)
		RETURNING *
	`, adjustmentID, periodID, period["book_id"], req.ProjectID, req.TaxCodeID, req.Direction,
		req.OffsetAccountID, entryDate, adjustmentType, roundAccountingAmount(req.TaxableDelta),
		roundAccountingAmount(req.VATDelta), req.Reason, req.SourceDocumentNo, journal["id"], actorID)
	if err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, req.ProjectID, actorID, actorName, "tax_adjustment_posted", "tax_adjustment", adjustmentID, "Ghi sổ điều chỉnh thuế GTGT", Record{
		"tax_period_id": periodID, "direction": req.Direction, "vat_delta": req.VATDelta,
		"amount_posted": amount, "journal_entry_id": journal["id"], "reason": req.Reason,
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	row["journal_entry_no"] = journal["entry_no"]
	return row, nil
}

func (r *Repository) ReverseAccountingTaxAdjustment(ctx context.Context, adjustmentID, entryDate, reason, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	adjustment, err := queryOneRecord(ctx, tx, `SELECT * FROM tax_adjustments WHERE id=$1 FOR SHARE`, adjustmentID)
	if err != nil {
		return nil, err
	}
	period, err := accountingTaxPeriodTx(ctx, tx, cleanID(adjustment["tax_period_id"]), true)
	if err != nil {
		return nil, err
	}
	if !statusIn(fmt.Sprint(period["status"]), "open", "amended") {
		return nil, validationError("Kỳ thuế đã chốt số liệu; hãy lập tờ khai bổ sung trước khi đảo điều chỉnh")
	}
	dateFrom := fmt.Sprint(period["date_from"])[0:10]
	dateTo := fmt.Sprint(period["date_to"])[0:10]
	if entryDate < dateFrom || entryDate > dateTo {
		return nil, validationError("Ngày hạch toán bút toán đảo phải thuộc kỳ thuế đã chọn")
	}
	var existing string
	err = tx.QueryRowContext(ctx, `SELECT id::text FROM tax_adjustments WHERE reversal_of_id=$1`, adjustmentID).Scan(&existing)
	if err == nil {
		return nil, validationError("Điều chỉnh thuế đã được đảo", Record{"reversal_adjustment_id": existing})
	}
	if err != sql.ErrNoRows {
		return nil, err
	}
	reversalJournal, err := r.reverseSourceJournalAtTx(ctx, tx, cleanID(adjustment["project_id"]), "tax_adjustment", adjustmentID,
		"tax_adjustment_posted", "tax_adjustment_reversed", entryDate, reason, actorID)
	if err != nil {
		return nil, err
	}
	reversalID := newID()
	row, err := queryOneRecord(ctx, tx, `
		INSERT INTO tax_adjustments (
			id,tax_period_id,book_id,project_id,tax_code_id,direction,offset_account_id,entry_date,
			adjustment_type,taxable_delta,vat_delta,reason,source_document_no,journal_entry_id,reversal_of_id,created_by
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8::date,'reversal',$9,$10,$11,$12,$13,$14,NULLIF($15,'')::uuid)
		RETURNING *
	`, reversalID, adjustment["tax_period_id"], adjustment["book_id"], adjustment["project_id"], adjustment["tax_code_id"],
		adjustment["direction"], adjustment["offset_account_id"], entryDate, -number(adjustment["taxable_delta"]),
		-number(adjustment["vat_delta"]), reason, adjustment["source_document_no"], reversalJournal["id"], adjustmentID, actorID)
	if err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, cleanID(adjustment["project_id"]), actorID, actorName, "tax_adjustment_reversed", "tax_adjustment", reversalID, "Đảo điều chỉnh thuế GTGT", Record{
		"original_adjustment_id": adjustmentID, "journal_entry_id": reversalJournal["id"], "reason": reason,
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	row["journal_entry_no"] = reversalJournal["entry_no"]
	return row, nil
}

func (r *Repository) CreateAccountingTaxPayment(ctx context.Context, periodID string, body Record, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	period, err := accountingTaxPeriodTx(ctx, tx, periodID, true)
	if err != nil {
		return nil, err
	}
	if !statusIn(fmt.Sprint(period["status"]), "filed", "locked") {
		return nil, validationError("Chỉ được lập Phiếu nộp thuế sau khi tờ khai đã nộp")
	}
	project, err := queryOneRecord(ctx, tx, `SELECT accounting_book_id FROM projects WHERE id=$1 AND status <> 'cancelled'`, body["project_id"])
	if err != nil || cleanID(project["accounting_book_id"]) != cleanID(period["book_id"]) {
		return nil, validationError("Công trình hạch toán không thuộc sổ kế toán của kỳ thuế")
	}
	var payable, reserved float64
	if err := tx.QueryRowContext(ctx, `
		SELECT snapshot.vat_payable,
			COALESCE((SELECT SUM(payment.amount) FROM payments payment
				WHERE payment.tax_period_id=$1 AND payment.status NOT IN ('cancelled','deleted')),0)
		FROM vat_return_snapshots snapshot
		WHERE snapshot.tax_period_id=$1
		ORDER BY snapshot.amendment_no DESC LIMIT 1
	`, periodID).Scan(&payable, &reserved); err != nil {
		return nil, err
	}
	amount := number(body["amount"])
	if amount > payable-reserved+0.005 {
		return nil, validationError("Số tiền nộp vượt số thuế GTGT còn phải nộp", Record{"vat_payable": payable, "reserved_or_paid": reserved})
	}
	row, err := queryOneRecord(ctx, tx, `
		INSERT INTO payments (
			project_id,accounting_book_id,budget_id,cash_account_id,payment_code,payment_type,
			target_type,target_id,tax_period_id,amount,payment_method,status,created_by
		) VALUES ($1,$2,NULL,$3,$4,'tax_payment','tax_period',$5,$5,$6,$7,'draft',NULLIF($8,'')::uuid)
		RETURNING *
	`, body["project_id"], body["accounting_book_id"], body["cash_account_id"], body["payment_code"],
		periodID, body["amount"], body["payment_method"], actorID)
	if err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, cleanID(body["project_id"]), actorID, actorName, "tax_payment_created", "payment", cleanID(row["id"]), "Lập Phiếu nộp thuế GTGT", Record{
		"tax_period_id": periodID, "amount": amount, "status": row["status"],
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return row, nil
}

func accountingTaxPeriodTx(ctx context.Context, tx *sql.Tx, periodID string, forUpdate bool) (Record, error) {
	lock := ""
	if forUpdate {
		lock = " FOR UPDATE OF period"
	}
	return queryOneRecord(ctx, tx, `
		SELECT period.*,book.book_code,book.name AS book_name
		FROM tax_periods period
		JOIN accounting_books book ON book.id=period.book_id
		WHERE period.id=$1`+lock, periodID)
}

func vatPeriodSummaryTx(ctx context.Context, tx *sql.Tx, period Record) (Record, []Record, error) {
	row, err := queryOneRecord(ctx, tx, `
		WITH input_rows AS (
			SELECT line.vat_amount,line.deductible_amount
			FROM invoice_tax_lines line
			JOIN input_invoices invoice ON invoice.id=line.input_invoice_id
			JOIN journal_entries journal ON journal.source_type='input_invoice' AND journal.source_id=invoice.id
				AND journal.source_event='input_invoice_verified' AND journal.status='posted'
			LEFT JOIN journal_entries reversal ON reversal.reversal_of_id=journal.id AND reversal.status='posted'
			WHERE line.book_id=$1 AND invoice.invoice_date BETWEEN $2::date AND $3::date AND reversal.id IS NULL
		), output_rows AS (
			SELECT line.vat_amount
			FROM invoice_tax_lines line
			JOIN output_invoices invoice ON invoice.id=line.output_invoice_id
			JOIN journal_entries journal ON journal.source_type='output_invoice' AND journal.source_id=invoice.id
				AND journal.source_event='output_invoice_issued' AND journal.status='posted'
			LEFT JOIN journal_entries reversal ON reversal.reversal_of_id=journal.id AND reversal.status='posted'
			WHERE line.book_id=$1 AND invoice.invoice_date BETWEEN $2::date AND $3::date AND reversal.id IS NULL
		), adjustment_rows AS (
			SELECT direction,vat_delta FROM tax_adjustments adjustment
			JOIN journal_entries journal ON journal.id=adjustment.journal_entry_id AND journal.status='posted'
			WHERE adjustment.book_id=$1 AND adjustment.entry_date BETWEEN $2::date AND $3::date
		), gl AS (
			SELECT
				COALESCE(SUM(line.debit_amount-line.credit_amount) FILTER (WHERE account.account_code='1331'),0) AS input_gl,
				COALESCE(SUM(line.credit_amount-line.debit_amount) FILTER (WHERE account.account_code='3331'),0) AS output_gl
			FROM journal_entries journal
			JOIN journal_entry_lines line ON line.entry_id=journal.id
			JOIN accounting_accounts account ON account.id=line.account_id
			WHERE journal.book_id=$1 AND journal.status='posted'
			  AND journal.source_type IN ('input_invoice','output_invoice','tax_adjustment')
			  AND journal.entry_date BETWEEN $2::date AND $3::date
		), gaps AS (
			SELECT
				(SELECT COUNT(*) FROM input_invoices invoice WHERE invoice.accounting_book_id=$1 AND invoice.invoice_date BETWEEN $2::date AND $3::date AND invoice.status='draft')
				+ (SELECT COUNT(*) FROM output_invoices invoice WHERE invoice.accounting_book_id=$1 AND invoice.invoice_date BETWEEN $2::date AND $3::date AND invoice.status='draft') AS unposted_count,
				(SELECT COUNT(*) FROM input_invoices invoice LEFT JOIN invoice_tax_lines line ON line.input_invoice_id=invoice.id
					WHERE invoice.accounting_book_id=$1 AND invoice.invoice_date BETWEEN $2::date AND $3::date AND invoice.status <> 'deleted' AND line.id IS NULL)
				+ (SELECT COUNT(*) FROM output_invoices invoice LEFT JOIN invoice_tax_lines line ON line.output_invoice_id=invoice.id
					WHERE invoice.accounting_book_id=$1 AND invoice.invoice_date BETWEEN $2::date AND $3::date AND invoice.status <> 'deleted' AND line.id IS NULL) AS tax_line_gap_count,
				(SELECT COUNT(*) FROM input_invoices invoice WHERE invoice.accounting_book_id=$1 AND invoice.invoice_date BETWEEN $2::date AND $3::date
					AND invoice.status <> 'deleted' AND abs(invoice.amount_before_vat+invoice.vat_amount-invoice.total_amount)>0.01)
				+ (SELECT COUNT(*) FROM output_invoices invoice WHERE invoice.accounting_book_id=$1 AND invoice.invoice_date BETWEEN $2::date AND $3::date
					AND invoice.status <> 'deleted' AND abs(invoice.amount_before_vat+invoice.vat_amount-invoice.total_amount)>0.01) AS vat_math_mismatch_count
		)
		SELECT
			(SELECT COUNT(*) FROM input_rows) AS input_invoice_count,
			(SELECT COUNT(*) FROM output_rows) AS output_invoice_count,
			COALESCE((SELECT SUM(vat_amount) FROM input_rows),0) AS input_vat_amount,
			COALESCE((SELECT SUM(deductible_amount) FROM input_rows),0) AS deductible_input_vat_amount,
			COALESCE((SELECT SUM(vat_amount) FROM output_rows),0) AS output_vat_amount,
			COALESCE((SELECT SUM(vat_delta) FROM adjustment_rows WHERE direction='input'),0) AS input_adjustment_vat_amount,
			COALESCE((SELECT SUM(vat_delta) FROM adjustment_rows WHERE direction='output'),0) AS output_adjustment_vat_amount,
			COALESCE((SELECT SUM(CASE WHEN direction='output' THEN vat_delta ELSE -vat_delta END) FROM adjustment_rows),0) AS adjustment_vat_amount,
			gl.input_gl,gl.output_gl,gaps.unposted_count,gaps.tax_line_gap_count,gaps.vat_math_mismatch_count
		FROM gl CROSS JOIN gaps
	`, period["book_id"], period["date_from"], period["date_to"])
	if err != nil {
		return nil, nil, err
	}
	expectedInput := number(row["deductible_input_vat_amount"]) + number(row["input_adjustment_vat_amount"])
	expectedOutput := number(row["output_vat_amount"]) + number(row["output_adjustment_vat_amount"])
	row["input_gl_difference"] = roundAccountingAmount(number(row["input_gl"]) - expectedInput)
	row["output_gl_difference"] = roundAccountingAmount(number(row["output_gl"]) - expectedOutput)
	row["net_vat_payable_before_credit"] = roundAccountingAmount(expectedOutput - expectedInput)
	issues := make([]Record, 0, 3)
	if number(row["unposted_count"]) > 0 {
		issues = append(issues, Record{"code": "unposted_invoices", "severity": "blocker", "title": "Còn hóa đơn chưa ghi sổ", "count": row["unposted_count"]})
	}
	if number(row["tax_line_gap_count"]) > 0 || number(row["vat_math_mismatch_count"]) > 0 {
		issues = append(issues, Record{"code": "tax_dimension_gap", "severity": "blocker", "title": "Thiếu mã thuế hoặc sai phép tính thuế", "tax_line_gap_count": row["tax_line_gap_count"], "vat_math_mismatch_count": row["vat_math_mismatch_count"]})
	}
	if math.Abs(number(row["input_gl_difference"])) > 0.01 || math.Abs(number(row["output_gl_difference"])) > 0.01 {
		issues = append(issues, Record{"code": "vat_gl_mismatch", "severity": "blocker", "title": "Sổ thuế lệch TK 1331/3331", "input_difference": row["input_gl_difference"], "output_difference": row["output_gl_difference"]})
	}
	row["blocker_count"] = len(issues)
	return row, issues, nil
}

func vatInvoiceRowsTx(ctx context.Context, tx *sql.Tx, period Record, direction string) ([]Record, error) {
	if direction == "input" {
		return queryTx(ctx, tx, `
			SELECT line.id,line.project_id,project.code AS project_code,project.name AS project_name,
				'input'::text AS direction,invoice.invoice_date,invoice.invoice_number,invoice.invoice_symbol,
				invoice.vendor_name AS counterparty_name,invoice.vendor_tax_code AS counterparty_tax_code,
				tax.tax_code,tax.name AS tax_name,tax.vat_rate,line.taxable_amount,line.vat_amount,
				line.deductible_amount,line.nondeductible_amount,journal.id AS journal_entry_id,journal.entry_no AS journal_entry_no
			FROM invoice_tax_lines line
			JOIN input_invoices invoice ON invoice.id=line.input_invoice_id
			JOIN projects project ON project.id=line.project_id
			JOIN tax_codes tax ON tax.id=line.tax_code_id
			JOIN journal_entries journal ON journal.source_type='input_invoice' AND journal.source_id=invoice.id
				AND journal.source_event='input_invoice_verified' AND journal.status='posted'
			LEFT JOIN journal_entries reversal ON reversal.reversal_of_id=journal.id AND reversal.status='posted'
			WHERE line.book_id=$1 AND invoice.invoice_date BETWEEN $2::date AND $3::date AND reversal.id IS NULL
			ORDER BY invoice.invoice_date DESC,invoice.invoice_number,line.line_no
		`, period["book_id"], period["date_from"], period["date_to"])
	}
	return queryTx(ctx, tx, `
		SELECT line.id,line.project_id,project.code AS project_code,project.name AS project_name,
			'output'::text AS direction,invoice.invoice_date,invoice.invoice_number,invoice.invoice_symbol,
			invoice.client_name AS counterparty_name,invoice.client_tax_code AS counterparty_tax_code,
			tax.tax_code,tax.name AS tax_name,tax.vat_rate,line.taxable_amount,line.vat_amount,
			0::numeric AS deductible_amount,line.nondeductible_amount,journal.id AS journal_entry_id,journal.entry_no AS journal_entry_no
		FROM invoice_tax_lines line
		JOIN output_invoices invoice ON invoice.id=line.output_invoice_id
		JOIN projects project ON project.id=line.project_id
		JOIN tax_codes tax ON tax.id=line.tax_code_id
		JOIN journal_entries journal ON journal.source_type='output_invoice' AND journal.source_id=invoice.id
			AND journal.source_event='output_invoice_issued' AND journal.status='posted'
		LEFT JOIN journal_entries reversal ON reversal.reversal_of_id=journal.id AND reversal.status='posted'
		WHERE line.book_id=$1 AND invoice.invoice_date BETWEEN $2::date AND $3::date AND reversal.id IS NULL
		ORDER BY invoice.invoice_date DESC,invoice.invoice_number,line.line_no
	`, period["book_id"], period["date_from"], period["date_to"])
}

func previousVATCreditTx(ctx context.Context, tx *sql.Tx, period Record) (float64, error) {
	var credit float64
	err := tx.QueryRowContext(ctx, `
		SELECT COALESCE(snapshot.credit_carried_forward,0)
		FROM vat_return_snapshots snapshot
		JOIN tax_periods previous ON previous.id=snapshot.tax_period_id
		WHERE previous.book_id=$1 AND previous.period_type=$2 AND previous.date_to < $3::date
		ORDER BY previous.date_to DESC,snapshot.amendment_no DESC LIMIT 1
	`, period["book_id"], period["period_type"], period["date_from"]).Scan(&credit)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	return credit, err
}

func ensureVATSourcePeriodMutableTx(ctx context.Context, tx *sql.Tx, bookID string, sourceDate any) error {
	var periodLabel string
	err := tx.QueryRowContext(ctx, `
		SELECT CASE WHEN period.period_type='quarter' THEN 'Quý ' ELSE 'Tháng ' END || period.period_no || '/' || period.fiscal_year
		FROM tax_periods period
		WHERE period.book_id=$1 AND $2::date BETWEEN period.date_from AND period.date_to
		  AND period.status IN ('prepared','filed','locked')
		  AND EXISTS (SELECT 1 FROM vat_return_snapshots snapshot WHERE snapshot.tax_period_id=period.id)
		ORDER BY period.date_to-period.date_from LIMIT 1
	`, bookID, sourceDate).Scan(&periodLabel)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	return validationError("Hóa đơn thuộc kỳ thuế đã chốt số liệu; hãy lập tờ khai bổ sung trước khi sửa", Record{"tax_period": periodLabel})
}

func insertInputInvoiceTaxLineTx(ctx context.Context, tx *sql.Tx, invoice Record, actorID string) error {
	var deductibleRate float64
	if err := tx.QueryRowContext(ctx, `SELECT deductible_rate FROM tax_codes WHERE id=$1 AND book_id=$2`, invoice["tax_code_id"], invoice["accounting_book_id"]).Scan(&deductibleRate); err != nil {
		return err
	}
	vatAmount := roundAccountingAmount(number(invoice["vat_amount"]))
	deductible := roundAccountingAmount(vatAmount * deductibleRate / 100)
	_, err := tx.ExecContext(ctx, `
		INSERT INTO invoice_tax_lines (
			book_id,project_id,input_invoice_id,line_no,tax_code_id,taxable_amount,vat_amount,
			deductible_amount,nondeductible_amount,description,created_by
		) VALUES ($1,$2,$3,1,$4,$5,$6,$7,$8,'Thuế GTGT theo hóa đơn đầu vào',NULLIF($9,'')::uuid)
		ON CONFLICT (input_invoice_id,line_no) WHERE input_invoice_id IS NOT NULL DO NOTHING
	`, invoice["accounting_book_id"], invoice["project_id"], invoice["id"], invoice["tax_code_id"],
		invoice["amount_before_vat"], vatAmount, deductible, roundAccountingAmount(vatAmount-deductible), actorID)
	return err
}

func insertOutputInvoiceTaxLineTx(ctx context.Context, tx *sql.Tx, invoice Record, actorID string) error {
	vatAmount := roundAccountingAmount(number(invoice["vat_amount"]))
	_, err := tx.ExecContext(ctx, `
		INSERT INTO invoice_tax_lines (
			book_id,project_id,output_invoice_id,line_no,tax_code_id,taxable_amount,vat_amount,
			deductible_amount,nondeductible_amount,description,created_by
		) VALUES ($1,$2,$3,1,$4,$5,$6,0,$6,'Thuế GTGT theo hóa đơn đầu ra',NULLIF($7,'')::uuid)
		ON CONFLICT (output_invoice_id,line_no) WHERE output_invoice_id IS NOT NULL DO NOTHING
	`, invoice["accounting_book_id"], invoice["project_id"], invoice["id"], invoice["tax_code_id"],
		invoice["amount_before_vat"], vatAmount, actorID)
	return err
}

func taxAdjustmentLines(direction string, vatDelta float64, taxAccountID, offsetAccountID, description string) []accountingJournalLine {
	amount := roundAccountingAmount(math.Abs(vatDelta))
	if (direction == "input" && vatDelta > 0) || (direction == "output" && vatDelta < 0) {
		return []accountingJournalLine{
			{AccountID: taxAccountID, Description: description, DebitAmount: amount},
			{AccountID: offsetAccountID, Description: description, CreditAmount: amount},
		}
	}
	return []accountingJournalLine{
		{AccountID: offsetAccountID, Description: description, DebitAmount: amount},
		{AccountID: taxAccountID, Description: description, CreditAmount: amount},
	}
}

func taxPeriodEventDescription(eventType string) string {
	return map[string]string{
		"prepared": "Chốt số liệu tờ khai thuế GTGT", "filed": "Ghi nhận đã nộp tờ khai thuế GTGT",
		"locked": "Khóa kỳ thuế GTGT", "amendment_started": "Mở tờ khai bổ sung thuế GTGT",
	}[eventType]
}

func taxDirectionLabel(direction string) string {
	if direction == "input" {
		return "đầu vào"
	}
	return "đầu ra"
}
