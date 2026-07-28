package core

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"strings"
	"time"
)

type sourceJournalSpec struct {
	ProjectID    string
	EntryDate    string
	DocumentDate string
	Description  string
	SourceType   string
	SourceID     string
	SourceEvent  string
	ReversalOfID string
	Lines        []accountingJournalLine
}

func (r *Repository) postSourceJournalTx(ctx context.Context, tx *sql.Tx, spec sourceJournalSpec, actorID string) (Record, error) {
	existing, err := queryTx(ctx, tx, `
		SELECT * FROM journal_entries
		WHERE project_id=$1 AND source_type=$2 AND source_id=$3 AND source_event=$4 AND status='posted'
	`, spec.ProjectID, spec.SourceType, spec.SourceID, spec.SourceEvent)
	if err != nil {
		return nil, err
	}
	if len(existing) > 0 {
		return existing[0], nil
	}
	if len(spec.Lines) < 2 {
		return nil, validationError("Bút toán nguồn phải có ít nhất hai dòng định khoản")
	}
	var debitTotal float64
	var creditTotal float64
	for _, line := range spec.Lines {
		debitTotal += roundAccountingAmount(line.DebitAmount)
		creditTotal += roundAccountingAmount(line.CreditAmount)
	}
	if debitTotal <= 0 || math.Abs(debitTotal-creditTotal) > 0.005 {
		return nil, validationError("Bút toán nguồn chưa cân", Record{
			"source_type": spec.SourceType, "source_id": spec.SourceID,
			"debit_total": roundAccountingAmount(debitTotal), "credit_total": roundAccountingAmount(creditTotal),
		})
	}
	project, period, err := accountingPostingContextTx(ctx, tx, spec.ProjectID, spec.EntryDate)
	if err != nil {
		return nil, err
	}
	bookID := cleanID(project["accounting_book_id"])
	if err := validateAccountingAccountsTx(ctx, tx, bookID, spec.Lines); err != nil {
		return nil, err
	}
	entryNo, err := nextAccountingEntryNoTx(ctx, tx, bookID, spec.EntryDate)
	if err != nil {
		return nil, err
	}
	entryID := newID()
	rows, err := queryTx(ctx, tx, `
		INSERT INTO journal_entries (
			id,book_id,project_id,period_id,entry_no,entry_date,document_date,description,
			source_type,source_id,source_event,status,reversal_of_id,created_by
		) VALUES (
			$1,$2,$3,$4,$5,$6::date,NULLIF($7,'')::date,$8,$9,$10,$11,'draft',NULLIF($12,'')::uuid,NULLIF($13,'')::uuid
		)
		RETURNING *
	`, entryID, bookID, spec.ProjectID, period["id"], entryNo, spec.EntryDate, spec.DocumentDate,
		spec.Description, spec.SourceType, spec.SourceID, spec.SourceEvent, spec.ReversalOfID, actorID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	for index, line := range spec.Lines {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO journal_entry_lines (
				entry_id,book_id,line_no,account_id,partner_id,cost_code,description,debit_amount,credit_amount
			) VALUES ($1,$2,$3,$4,NULLIF($5,'')::uuid,NULLIF($6,''),NULLIF($7,''),$8,$9)
		`, entryID, bookID, index+1, line.AccountID, line.PartnerID, line.CostCode,
			line.Description, roundAccountingAmount(line.DebitAmount), roundAccountingAmount(line.CreditAmount)); err != nil {
			return nil, err
		}
	}
	posted, err := queryTx(ctx, tx, `
		UPDATE journal_entries
		SET status='posted',posted_by=NULLIF($2,'')::uuid,posted_at=now(),updated_at=now()
		WHERE id=$1 AND status='draft'
		RETURNING *
	`, entryID, actorID)
	if err != nil {
		return nil, err
	}
	if len(posted) == 0 {
		return nil, validationError("Không thể ghi sổ bút toán nguồn")
	}
	return posted[0], nil
}

func (r *Repository) reverseSourceJournalTx(
	ctx context.Context,
	tx *sql.Tx,
	projectID, sourceType, sourceID, originalEvent, reversalEvent, reason, actorID string,
) (Record, error) {
	return r.reverseSourceJournalAtTx(
		ctx, tx, projectID, sourceType, sourceID, originalEvent, reversalEvent,
		time.Now().Format("2006-01-02"), reason, actorID,
	)
}

func (r *Repository) reverseSourceJournalAtTx(
	ctx context.Context,
	tx *sql.Tx,
	projectID, sourceType, sourceID, originalEvent, reversalEvent, entryDate, reason, actorID string,
) (Record, error) {
	entries, err := queryTx(ctx, tx, `
		SELECT * FROM journal_entries
		WHERE project_id=$1 AND source_type=$2 AND source_id=$3 AND source_event=$4 AND status='posted'
		FOR SHARE
	`, projectID, sourceType, sourceID, originalEvent)
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return nil, validationError("Chứng từ đã ghi nhận nhưng chưa có bút toán nguồn để đảo", Record{
			"source_type": sourceType, "source_id": sourceID, "source_event": originalEvent,
		})
	}
	original := entries[0]
	existing, err := queryTx(ctx, tx, `SELECT id FROM journal_entries WHERE reversal_of_id=$1 AND status='posted'`, original["id"])
	if err != nil {
		return nil, err
	}
	if len(existing) > 0 {
		return nil, validationError("Bút toán nguồn đã được đảo", Record{"reversal_entry_id": existing[0]["id"]})
	}
	rows, err := queryTx(ctx, tx, `
		SELECT account_id,partner_id,cost_code,description,debit_amount,credit_amount
		FROM journal_entry_lines WHERE entry_id=$1 ORDER BY line_no
	`, original["id"])
	if err != nil {
		return nil, err
	}
	lines := make([]accountingJournalLine, 0, len(rows))
	for _, line := range rows {
		lines = append(lines, accountingJournalLine{
			AccountID: cleanID(line["account_id"]), PartnerID: cleanID(line["partner_id"]),
			CostCode: fmt.Sprint(line["cost_code"]), Description: "Bút toán đảo · " + fmt.Sprint(line["description"]),
			DebitAmount: number(line["credit_amount"]), CreditAmount: number(line["debit_amount"]),
		})
	}
	reversal, err := r.postSourceJournalTx(ctx, tx, sourceJournalSpec{
		ProjectID: projectID, EntryDate: entryDate, DocumentDate: accountingRecordDate(original["document_date"]),
		Description: "Đảo " + fmt.Sprint(original["entry_no"]) + ": " + strings.TrimSpace(reason),
		SourceType:  sourceType, SourceID: sourceID, SourceEvent: reversalEvent,
		ReversalOfID: cleanID(original["id"]), Lines: lines,
	}, actorID)
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO budget_movements (
			budget_id,project_id,movement_type,commitment_delta,actual_delta,paid_delta,
			source_type,source_id,source_event,journal_entry_id,reversal_of_id,description,occurred_at,created_by
		)
		SELECT movement.budget_id,movement.project_id,'reversal',-movement.commitment_delta,-movement.actual_delta,-movement.paid_delta,
			$2,$3,$4,$5,movement.id,'Đảo biến động ngân sách: ' || $6,now(),NULLIF($7,'')::uuid
		FROM budget_movements movement
		WHERE movement.journal_entry_id=$1
		  AND NOT EXISTS (SELECT 1 FROM budget_movements reversed WHERE reversed.reversal_of_id=movement.id)
	`, original["id"], sourceType, sourceID, reversalEvent, reversal["id"], reason, actorID); err != nil {
		return nil, err
	}
	return reversal, nil
}

func (r *Repository) recordPayableCommitmentTx(ctx context.Context, tx *sql.Tx, payable Record, actorID string) error {
	budgetID := cleanID(payable["budget_id"])
	if budgetID == "" {
		return nil
	}
	amount := roundAccountingAmount(number(payable["gross_amount"]))
	if amount <= 0 {
		return nil
	}
	budgets, err := queryTx(ctx, tx, `
		SELECT status FROM accounting_budgets
		WHERE id=$1 AND project_id=$2
		FOR UPDATE
	`, budgetID, payable["project_id"])
	if err != nil {
		return err
	}
	if len(budgets) == 0 || fmt.Sprint(budgets[0]["status"]) != "approved" {
		return validationError("Khoản phải trả chỉ được cam kết vào ngân sách đã duyệt", Record{"budget_id": budgetID})
	}
	balances, err := queryTx(ctx, tx, `SELECT available_amount FROM accounting_budget_balances WHERE id=$1`, budgetID)
	if err != nil {
		return err
	}
	if len(balances) == 0 {
		return sql.ErrNoRows
	}
	if number(balances[0]["available_amount"])+0.005 < amount {
		return validationError("Giá trị cam kết chi vượt ngân sách khả dụng", Record{
			"budget_id": budgetID, "commitment_amount": amount, "available_amount": balances[0]["available_amount"],
		})
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO budget_movements (
			budget_id,project_id,movement_type,commitment_delta,actual_delta,paid_delta,
			source_type,source_id,source_event,description,created_by
		) VALUES ($1,$2,'commitment',$3,0,0,'payable',$4,'payable_approved','Cam kết chi theo khoản phải trả',NULLIF($5,'')::uuid)
		ON CONFLICT (budget_id,source_type,source_id,source_event) DO NOTHING
	`, budgetID, payable["project_id"], amount, payable["id"], actorID)
	return err
}

func (r *Repository) postInputInvoiceTx(ctx context.Context, tx *sql.Tx, invoice Record, actorID string) (Record, error) {
	taxRows, err := queryTx(ctx, tx, `
		SELECT COALESCE(SUM(deductible_amount),0) AS deductible_amount
		FROM invoice_tax_lines WHERE input_invoice_id=$1
	`, invoice["id"])
	if err != nil {
		return nil, err
	}
	deductibleVAT := number(taxRows[0]["deductible_amount"])
	totalAmount := roundAccountingAmount(number(invoice["total_amount"]))
	expenseAmount := roundAccountingAmount(totalAmount - deductibleVAT)
	creditAccountID, err := accountingAccountIDTx(ctx, tx, cleanID(invoice["accounting_book_id"]), "331")
	if err != nil {
		return nil, err
	}
	lines := []accountingJournalLine{{
		AccountID: cleanID(invoice["expense_account_id"]), PartnerID: cleanID(invoice["vendor_partner_id"]),
		Description: "Giá trị mua vào và thuế GTGT không được khấu trừ", DebitAmount: expenseAmount,
	}}
	if deductibleVAT > 0 {
		inputVATAccountID, err := accountingAccountIDTx(ctx, tx, cleanID(invoice["accounting_book_id"]), "1331")
		if err != nil {
			return nil, err
		}
		lines = append(lines, accountingJournalLine{
			AccountID: inputVATAccountID, PartnerID: cleanID(invoice["vendor_partner_id"]),
			Description: "Thuế GTGT đầu vào được khấu trừ", DebitAmount: deductibleVAT,
		})
	}
	lines = append(lines, accountingJournalLine{
		AccountID: creditAccountID, PartnerID: cleanID(invoice["vendor_partner_id"]),
		Description: "Công nợ phải trả theo hóa đơn đầu vào", CreditAmount: totalAmount,
	})
	entry, err := r.postSourceJournalTx(ctx, tx, sourceJournalSpec{
		ProjectID: cleanID(invoice["project_id"]), EntryDate: accountingRecordDate(invoice["invoice_date"]),
		DocumentDate: accountingRecordDate(invoice["invoice_date"]),
		Description:  "Ghi nhận hóa đơn đầu vào " + fmt.Sprint(invoice["invoice_number"]),
		SourceType:   "input_invoice", SourceID: cleanID(invoice["id"]), SourceEvent: "input_invoice_verified", Lines: lines,
	}, actorID)
	if err != nil {
		return nil, err
	}
	payables, err := queryTx(ctx, tx, `SELECT budget_id FROM payables WHERE id=$1`, invoice["payable_id"])
	if err != nil {
		return nil, err
	}
	if len(payables) > 0 && cleanID(payables[0]["budget_id"]) != "" {
		budgetID := cleanID(payables[0]["budget_id"])
		var outstandingCommitment float64
		if err := tx.QueryRowContext(ctx, `
			SELECT GREATEST(COALESCE(SUM(commitment_delta),0),0)
			FROM budget_movements WHERE budget_id=$1
		`, budgetID).Scan(&outstandingCommitment); err != nil {
			return nil, err
		}
		releaseAmount := math.Min(outstandingCommitment, totalAmount)
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO budget_movements (
				budget_id,project_id,movement_type,commitment_delta,actual_delta,paid_delta,
				source_type,source_id,source_event,journal_entry_id,description,occurred_at,created_by
			) VALUES ($1,$2,'actual',$3,$4,0,'input_invoice',$5,'input_invoice_verified',$6,
				'Ghi nhận thực chi theo hóa đơn đầu vào',$7::date,NULLIF($8,'')::uuid)
			ON CONFLICT (budget_id,source_type,source_id,source_event) DO NOTHING
		`, budgetID, invoice["project_id"], -releaseAmount, expenseAmount, invoice["id"], entry["id"],
			accountingRecordDate(invoice["invoice_date"]), actorID); err != nil {
			return nil, err
		}
	}
	return entry, nil
}

func (r *Repository) postOutputInvoiceTx(ctx context.Context, tx *sql.Tx, invoice Record, actorID string) (Record, error) {
	totalAmount := roundAccountingAmount(number(invoice["total_amount"]))
	vatAmount := roundAccountingAmount(number(invoice["vat_amount"]))
	revenueAmount := roundAccountingAmount(totalAmount - vatAmount)
	receivableAccountID, err := accountingAccountIDTx(ctx, tx, cleanID(invoice["accounting_book_id"]), "131")
	if err != nil {
		return nil, err
	}
	lines := []accountingJournalLine{
		{AccountID: receivableAccountID, PartnerID: cleanID(invoice["client_partner_id"]), Description: "Công nợ phải thu theo hóa đơn đầu ra", DebitAmount: totalAmount},
		{AccountID: cleanID(invoice["revenue_account_id"]), PartnerID: cleanID(invoice["client_partner_id"]), Description: "Doanh thu chưa có thuế GTGT", CreditAmount: revenueAmount},
	}
	if vatAmount > 0 {
		outputVATAccountID, err := accountingAccountIDTx(ctx, tx, cleanID(invoice["accounting_book_id"]), "3331")
		if err != nil {
			return nil, err
		}
		lines = append(lines, accountingJournalLine{
			AccountID: outputVATAccountID, PartnerID: cleanID(invoice["client_partner_id"]), Description: "Thuế GTGT đầu ra phải nộp", CreditAmount: vatAmount,
		})
	}
	return r.postSourceJournalTx(ctx, tx, sourceJournalSpec{
		ProjectID: cleanID(invoice["project_id"]), EntryDate: accountingRecordDate(invoice["invoice_date"]),
		DocumentDate: accountingRecordDate(invoice["invoice_date"]),
		Description:  "Ghi nhận hóa đơn đầu ra " + fmt.Sprint(invoice["invoice_number"]),
		SourceType:   "output_invoice", SourceID: cleanID(invoice["id"]), SourceEvent: "output_invoice_issued", Lines: lines,
	}, actorID)
}

func (r *Repository) postPaymentTx(ctx context.Context, tx *sql.Tx, payment Record, actorID string) (Record, error) {
	paymentType := fmt.Sprint(payment["payment_type"])
	if paymentType == "advance_settlement" {
		return nil, nil
	}
	amount := roundAccountingAmount(number(payment["amount"]))
	bookID := cleanID(payment["accounting_book_id"])
	partnerID := ""
	debitAccountID := ""
	creditAccountID := ""
	sourceEvent := ""
	description := ""

	paymentAllocations, err := queryTx(ctx, tx, `
		SELECT COALESCE(SUM(allocation.amount),0) AS allocated_amount,
			MIN(pay.vendor_partner_id::text) AS partner_id
		FROM payment_allocations allocation
		JOIN payables pay ON pay.id=allocation.payable_id
		WHERE allocation.payment_id=$1 AND allocation.allocation_type <> 'advance_settlement'
	`, payment["id"])
	if err != nil {
		return nil, err
	}
	receiptAllocations, err := queryTx(ctx, tx, `
		SELECT COALESCE(SUM(allocation.amount),0) AS allocated_amount,
			MIN(rec.client_partner_id::text) AS partner_id
		FROM receipt_allocations allocation
		JOIN receivables rec ON rec.id=allocation.receivable_id
		WHERE allocation.payment_id=$1 AND allocation.allocation_type <> 'advance_settlement'
	`, payment["id"])
	if err != nil {
		return nil, err
	}
	paymentAllocated := number(paymentAllocations[0]["allocated_amount"])
	receiptAllocated := number(receiptAllocations[0]["allocated_amount"])
	if paymentAllocated > 0 && receiptAllocated > 0 {
		return nil, validationError("Một phiếu không được đồng thời phân bổ cho phải trả và phải thu")
	}
	if paymentAllocated > 0 {
		if math.Abs(paymentAllocated-amount) > 0.005 {
			return nil, validationError("Tổng phân bổ phải trả không bằng số tiền phiếu chi", Record{"allocated_amount": paymentAllocated, "payment_amount": amount})
		}
		debitAccountID, err = accountingAccountIDTx(ctx, tx, bookID, "331")
		partnerID = cleanID(paymentAllocations[0]["partner_id"])
		sourceEvent = "payable_payment_approved"
		description = "Thanh toán công nợ phải trả"
	} else if receiptAllocated > 0 {
		if math.Abs(receiptAllocated-amount) > 0.005 {
			return nil, validationError("Tổng phân bổ phải thu không bằng số tiền phiếu thu", Record{"allocated_amount": receiptAllocated, "payment_amount": amount})
		}
		creditAccountID, err = accountingAccountIDTx(ctx, tx, bookID, "131")
		partnerID = cleanID(receiptAllocations[0]["partner_id"])
		sourceEvent = "receivable_receipt_approved"
		description = "Thu tiền công nợ phải thu"
	} else {
		switch paymentType {
		case "outbound":
			debitAccountID, err = accountingAccountIDTx(ctx, tx, bookID, "642")
			sourceEvent = "generic_outbound_approved"
			description = "Phiếu chi không qua công nợ"
		case "inbound":
			creditAccountID, err = accountingAccountIDTx(ctx, tx, bookID, "711")
			sourceEvent = "generic_inbound_approved"
			description = "Phiếu thu khác"
		case "tax_payment":
			debitAccountID, err = accountingAccountIDTx(ctx, tx, bookID, "3331")
			sourceEvent = "vat_payment_approved"
			description = "Nộp thuế GTGT theo tờ khai"
		default:
			return nil, validationError("Loại phiếu chưa đủ căn cứ định khoản Nợ/Có", Record{
				"payment_type": paymentType, "allowed_types": []string{"outbound", "inbound", "tax_payment"},
			})
		}
	}
	if err != nil {
		return nil, err
	}
	cashAccountID := cleanID(payment["cash_account_id"])
	if debitAccountID == "" {
		debitAccountID = cashAccountID
	}
	if creditAccountID == "" {
		creditAccountID = cashAccountID
	}
	entry, err := r.postSourceJournalTx(ctx, tx, sourceJournalSpec{
		ProjectID: cleanID(payment["project_id"]), EntryDate: accountingRecordDate(payment["paid_at"]),
		DocumentDate: accountingRecordDate(payment["paid_at"]), Description: description + " · " + fmt.Sprint(payment["payment_code"]),
		SourceType: "payment", SourceID: cleanID(payment["id"]), SourceEvent: sourceEvent,
		Lines: []accountingJournalLine{
			{AccountID: debitAccountID, PartnerID: partnerID, Description: description, DebitAmount: amount},
			{AccountID: creditAccountID, PartnerID: partnerID, Description: description, CreditAmount: amount},
		},
	}, actorID)
	if err != nil {
		return nil, err
	}
	budgetID := cleanID(payment["budget_id"])
	if budgetID != "" && (paymentAllocated > 0 || paymentType == "outbound") {
		actualDelta := float64(0)
		if paymentType == "outbound" && paymentAllocated == 0 {
			actualDelta = amount
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO budget_movements (
				budget_id,project_id,movement_type,commitment_delta,actual_delta,paid_delta,
				source_type,source_id,source_event,journal_entry_id,description,occurred_at,created_by
			) VALUES ($1,$2,'payment',0,$3,$4,'payment',$5,$6,$7,'Ghi nhận số đã chi theo phiếu chi',$8::date,NULLIF($9,'')::uuid)
			ON CONFLICT (budget_id,source_type,source_id,source_event) DO NOTHING
		`, budgetID, payment["project_id"], actualDelta, amount, payment["id"], sourceEvent, entry["id"],
			accountingRecordDate(payment["paid_at"]), actorID); err != nil {
			return nil, err
		}
	}
	return entry, nil
}

func accountingAccountIDTx(ctx context.Context, tx *sql.Tx, bookID, accountCode string) (string, error) {
	var accountID string
	if err := tx.QueryRowContext(ctx, `
		SELECT id::text FROM accounting_accounts
		WHERE book_id=$1 AND account_code=$2 AND active AND is_postable
	`, bookID, accountCode).Scan(&accountID); err != nil {
		if err == sql.ErrNoRows {
			return "", validationError("Chưa cấu hình tài khoản hạch toán", Record{"account_code": accountCode})
		}
		return "", err
	}
	return accountID, nil
}

func accountingRecordDate(value any) string {
	switch typed := value.(type) {
	case time.Time:
		return typed.Format("2006-01-02")
	case string:
		valueText := strings.TrimSpace(typed)
		if len(valueText) >= 10 {
			if _, err := time.Parse("2006-01-02", valueText[:10]); err == nil {
				return valueText[:10]
			}
		}
	}
	return time.Now().Format("2006-01-02")
}
