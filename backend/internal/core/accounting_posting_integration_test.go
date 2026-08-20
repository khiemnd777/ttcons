package core

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"consflow/backend/internal/shared/migrations"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestAccountingPostingSourceFlowsAndSafeCorrection(t *testing.T) {
	database := openIsolatedAccountingTestDB(t)
	repo := NewRepository(database)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	const (
		actorID      = "75900000-0000-0000-0000-000000000001"
		projectID    = "75900000-0000-0000-0000-000000000002"
		budgetID     = "75900000-0000-0000-0000-000000000003"
		payableID    = "75900000-0000-0000-0000-000000000004"
		inputID      = "75900000-0000-0000-0000-000000000005"
		receivableID = "75900000-0000-0000-0000-000000000006"
		outputID     = "75900000-0000-0000-0000-000000000007"
		paymentID    = "75900000-0000-0000-0000-000000000008"
	)

	mustAccountingExec(t, ctx, database, `
		INSERT INTO users (id,email,password_hash,full_name,status)
		VALUES ($1,'accounting-posting@test.local','test','Kế toán kiểm thử','active')
	`, actorID)
	mustAccountingExec(t, ctx, database, `
		INSERT INTO user_roles (user_id,role_id)
		SELECT $1,id FROM roles WHERE name='admin'
	`, actorID)
	mustAccountingExec(t, ctx, database, `
		INSERT INTO projects (id,name,code,client_name,status)
		VALUES ($1,'Công trình kiểm thử hạch toán','ACC-POSTING-TEST','Khách hàng kiểm thử','planning')
	`, projectID)
	mustAccountingExec(t, ctx, database, `
		INSERT INTO accounting_budgets (id,project_id,budget_code,name,planned_amount,status)
		VALUES ($1,$2,'NS-TEST-01','Ngân sách kiểm thử',1000000,'draft')
	`, budgetID, projectID)
	if _, err := repo.ApproveBudget(ctx, budgetID, actorID, "Kế toán kiểm thử"); err != nil {
		t.Fatal(err)
	}
	mustAccountingExec(t, ctx, database, `
		INSERT INTO payables (
			id,project_id,payable_code,vendor_type,vendor_name,source_type,gross_amount,remaining_amount,status,budget_id,created_by
		) VALUES ($1,$2,'PT-TEST-01','supplier','Nhà cung cấp kiểm thử','manual',110000,110000,'draft',$3,$4)
	`, payableID, projectID, budgetID, actorID)
	if _, err := repo.PayableAction(ctx, payableID, "approve", ActionRequest{}, actorID, "Kế toán kiểm thử"); err != nil {
		t.Fatal(err)
	}
	assertAccountingAmount(t, ctx, database, `
		SELECT COALESCE(SUM(commitment_delta),0) FROM budget_movements WHERE budget_id=$1
	`, 110000, budgetID)

	mustAccountingExec(t, ctx, database, `
		INSERT INTO input_invoices (
			id,project_id,payable_id,invoice_number,invoice_date,vendor_name,
			amount_before_vat,vat_amount,total_amount,status,storage_key,file_name,file_size,checksum_sha256,uploaded_by
		) VALUES (
			$1,$2,$3,'HDV-TEST-01',CURRENT_DATE,'Nhà cung cấp kiểm thử',
			100000,10000,110000,'draft','accounting/test/input.pdf','input.pdf',1,repeat('a',64),$4
		)
	`, inputID, projectID, payableID, actorID)
	assertAccountingAmount(t, ctx, database, `SELECT COUNT(*) FROM invoice_tax_lines WHERE input_invoice_id=$1`, 1, inputID)
	if _, err := repo.InputInvoiceAction(ctx, inputID, "verify", ActionRequest{}, actorID, "Kế toán kiểm thử"); err != nil {
		t.Fatal(err)
	}
	assertBalancedSourceJournal(t, ctx, database, "input_invoice", inputID, "input_invoice_verified", 110000)
	assertAccountingAmount(t, ctx, database, `
		SELECT COALESCE(SUM(actual_delta),0) FROM budget_movements WHERE budget_id=$1
	`, 100000, budgetID)
	assertAccountingAmount(t, ctx, database, `
		SELECT COALESCE(SUM(commitment_delta),0) FROM budget_movements WHERE budget_id=$1
	`, 0, budgetID)

	if _, err := repo.InputInvoiceAction(ctx, inputID, "reject", ActionRequest{}, actorID, "Kế toán kiểm thử"); err == nil || !strings.Contains(err.Error(), "Lý do") {
		t.Fatalf("reject posted invoice without reason error=%v, want mandatory reason", err)
	}
	if _, err := repo.InputInvoiceAction(ctx, inputID, "reject", ActionRequest{Reason: "Sai mã số thuế, lập hóa đơn thay thế"}, actorID, "Kế toán kiểm thử"); err != nil {
		t.Fatal(err)
	}
	assertBalancedSourceJournal(t, ctx, database, "input_invoice", inputID, "input_invoice_rejected", 110000)
	assertAccountingAmount(t, ctx, database, `
		SELECT COALESCE(SUM(actual_delta),0) FROM budget_movements WHERE budget_id=$1
	`, 0, budgetID)

	mustAccountingExec(t, ctx, database, `
		INSERT INTO receivables (
			id,project_id,receivable_code,client_name,source_type,contract_amount,gross_amount,remaining_amount,status,created_by
		) VALUES ($1,$2,'PTHU-TEST-01','Khách hàng kiểm thử','manual',220000,220000,220000,'approved',$3)
	`, receivableID, projectID, actorID)
	mustAccountingExec(t, ctx, database, `
		INSERT INTO output_invoices (
			id,project_id,receivable_id,invoice_number,invoice_date,client_name,
			amount_before_vat,vat_amount,total_amount,status,storage_key,file_name,file_size,checksum_sha256,uploaded_by
		) VALUES (
			$1,$2,$3,'HDR-TEST-01',CURRENT_DATE,'Khách hàng kiểm thử',
			200000,20000,220000,'draft','accounting/test/output.pdf','output.pdf',1,repeat('b',64),$4
		)
	`, outputID, projectID, receivableID, actorID)
	if _, err := repo.OutputInvoiceAction(ctx, outputID, "issue", ActionRequest{}, actorID, "Kế toán kiểm thử"); err != nil {
		t.Fatal(err)
	}
	assertBalancedSourceJournal(t, ctx, database, "output_invoice", outputID, "output_invoice_issued", 220000)
	reportFilter := AccountingReportFilter{ProjectID: projectID, DateFrom: time.Now().Format("2006-01-02"), DateTo: time.Now().Format("2006-01-02")}
	trialBalance, err := repo.AccountingTrialBalance(ctx, actorID, reportFilter)
	if err != nil {
		t.Fatal(err)
	}
	if number(trialBalance["summary"].(Record)["period_debit"]) != 440000 || number(trialBalance["summary"].(Record)["period_credit"]) != 440000 {
		t.Fatalf("trial balance summary=%v, want balanced period turnover 440000", trialBalance["summary"])
	}
	profitLoss, err := repo.AccountingProfitLoss(ctx, actorID, reportFilter)
	if err != nil {
		t.Fatal(err)
	}
	if number(profitLoss["summary"].(Record)["revenue_amount"]) != 200000 || number(profitLoss["summary"].(Record)["expense_amount"]) != 0 {
		t.Fatalf("profit loss summary=%v, want revenue 200000 and reversed expense 0", profitLoss["summary"])
	}
	vatLedger, err := repo.AccountingVATLedger(ctx, actorID, reportFilter)
	if err != nil {
		t.Fatal(err)
	}
	if number(vatLedger["summary"].(Record)["output_vat_amount"]) != 20000 || number(vatLedger["summary"].(Record)["deductible_input_vat_amount"]) != 0 {
		t.Fatalf("VAT summary=%v, want output VAT 20000 and reversed input VAT excluded", vatLedger["summary"])
	}
	generalLedger, err := repo.AccountingGeneralLedger(ctx, actorID, reportFilter)
	if err != nil {
		t.Fatal(err)
	}
	if int(number(generalLedger["summary"].(Record)["line_count"])) < 7 {
		t.Fatalf("general ledger summary=%v, want source and reversal lines", generalLedger["summary"])
	}
	if _, err := repo.OutputInvoiceAction(ctx, outputID, "void", ActionRequest{Reason: "Hóa đơn lập sai, phát hành hóa đơn thay thế"}, actorID, "Kế toán kiểm thử"); err != nil {
		t.Fatal(err)
	}
	assertBalancedSourceJournal(t, ctx, database, "output_invoice", outputID, "output_invoice_voided", 220000)

	if _, err := repo.PayableAction(ctx, payableID, "pay", ActionRequest{}, actorID, "Kế toán kiểm thử"); err == nil || !strings.Contains(err.Error(), "Phiếu chi") {
		t.Fatalf("direct payable settlement error=%v, want voucher requirement", err)
	}
	if _, err := repo.ReceivableAction(ctx, receivableID, "receive", ActionRequest{}, actorID, "Kế toán kiểm thử"); err == nil || !strings.Contains(err.Error(), "Phiếu thu") {
		t.Fatalf("direct receivable settlement error=%v, want voucher requirement", err)
	}

	mustAccountingExec(t, ctx, database, `
		INSERT INTO payments (
			id,project_id,payment_code,payment_type,amount,payment_method,status,budget_id,created_by
		) VALUES ($1,$2,'PC-TEST-01','outbound',50000,'bank_transfer','draft',$3,$4)
	`, paymentID, projectID, budgetID, actorID)
	if _, err := repo.PaymentAction(ctx, paymentID, "approve", ActionRequest{}, actorID, "Kế toán kiểm thử"); err != nil {
		t.Fatal(err)
	}
	assertBalancedSourceJournal(t, ctx, database, "payment", paymentID, "generic_outbound_approved", 50000)
	assertAccountingAmount(t, ctx, database, `
		SELECT COALESCE(SUM(paid_delta),0) FROM budget_movements WHERE budget_id=$1
	`, 50000, budgetID)
	if _, err := repo.PaymentAction(ctx, paymentID, "reverse", ActionRequest{}, actorID, "Kế toán kiểm thử"); err == nil || !strings.Contains(err.Error(), "Lý do") {
		t.Fatalf("payment reversal without reason error=%v, want mandatory reason", err)
	}
	if _, err := repo.PaymentAction(ctx, paymentID, "reverse", ActionRequest{Reason: "Phiếu chi hạch toán nhầm đối tượng"}, actorID, "Kế toán kiểm thử"); err != nil {
		t.Fatal(err)
	}
	assertBalancedSourceJournal(t, ctx, database, "payment", paymentID, "payment_reversed", 50000)
	assertAccountingAmount(t, ctx, database, `
		SELECT COALESCE(SUM(paid_delta),0) FROM budget_movements WHERE budget_id=$1
	`, 0, budgetID)
	var paymentStatus string
	if err := database.QueryRowContext(ctx, `SELECT status FROM payments WHERE id=$1`, paymentID).Scan(&paymentStatus); err != nil {
		t.Fatal(err)
	}
	if paymentStatus != "cancelled" {
		t.Fatalf("reversed payment status=%s, want cancelled", paymentStatus)
	}

	const reconciliationIssueID = "75900000-0000-0000-0000-000000000009"
	mustAccountingExec(t, ctx, database, `
		INSERT INTO accounting_backfill_issues (id,project_id,source_type,source_id,source_code,issue_code,description)
		VALUES ($1,$2,'payment',$3,'PC-TEST-01','manual_review','Cần đối chiếu chứng từ chuyển đổi')
	`, reconciliationIssueID, projectID, paymentID)
	var resolutionJournalID string
	if err := database.QueryRowContext(ctx, `
		SELECT id::text FROM journal_entries
		WHERE project_id=$1 AND source_type='payment' AND source_event='generic_outbound_approved'
	`, projectID).Scan(&resolutionJournalID); err != nil {
		t.Fatal(err)
	}
	resolved, err := repo.ResolveAccountingBackfillIssue(ctx, reconciliationIssueID, "resolve", "Đã lập CTGS bổ sung và đối chiếu số dư", resolutionJournalID, actorID, "Kế toán kiểm thử")
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(resolved["status"]) != "resolved" || cleanID(resolved["resolution_journal_entry_id"]) != resolutionJournalID {
		t.Fatalf("resolved reconciliation=%v, want linked posted journal", resolved)
	}
	assertAccountingAmount(t, ctx, database, `SELECT COUNT(*) FROM audit_logs WHERE target_type='accounting_backfill_issue' AND target_id=$1`, 1, reconciliationIssueID)
	if _, err := repo.ResolveAccountingBackfillIssue(ctx, reconciliationIssueID, "accept", "Không phát sinh nghĩa vụ ghi sổ", "", actorID, "Kế toán kiểm thử"); err == nil || !strings.Contains(err.Error(), "đã được xử lý") {
		t.Fatalf("second reconciliation resolution error=%v, want immutable resolution conflict", err)
	}
}

func TestManualJournalPreservesSourceDocumentNumber(t *testing.T) {
	database := openIsolatedAccountingTestDB(t)
	repo := NewRepository(database)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	const (
		actorID   = "75910000-0000-0000-0000-000000000001"
		projectID = "75910000-0000-0000-0000-000000000002"
	)
	mustAccountingExec(t, ctx, database, `
		INSERT INTO users (id,email,password_hash,full_name,status)
		VALUES ($1,'manual-journal@test.local','test','Kế toán kiểm thử CTGS','active')
	`, actorID)
	mustAccountingExec(t, ctx, database, `
		INSERT INTO projects (id,name,code,client_name,status)
		VALUES ($1,'Công trình kiểm thử CTGS','ACC-MANUAL-TEST','Khách hàng kiểm thử','planning')
	`, projectID)

	record, err := repo.CreateAccountingJournal(
		ctx,
		projectID,
		time.Now().Format("2006-01-02"),
		time.Now().Format("2006-01-02"),
		"BB-DC-2026-001",
		"Điều chỉnh theo biên bản đối chiếu",
		[]accountingJournalLine{
			{AccountID: "73100000-0000-0000-0000-000000001111", Description: "Ghi Nợ tiền mặt", DebitAmount: 1000},
			{AccountID: "73100000-0000-0000-0000-000000000411", Description: "Ghi Có vốn", CreditAmount: 1000},
		},
		actorID,
		"Kế toán kiểm thử CTGS",
	)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(fmt.Sprint(record["source_document_no"])); got != "BB-DC-2026-001" {
		t.Fatalf("source_document_no=%q, want BB-DC-2026-001", got)
	}
}

func TestAccountingLedgerBackfillIsBalancedAndIdempotent(t *testing.T) {
	database := openIsolatedAccountingTestDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	const (
		actorID      = "75910000-0000-0000-0000-000000000001"
		projectID    = "75910000-0000-0000-0000-000000000002"
		budgetID     = "75910000-0000-0000-0000-000000000003"
		payableID    = "75910000-0000-0000-0000-000000000004"
		inputID      = "75910000-0000-0000-0000-000000000005"
		receivableID = "75910000-0000-0000-0000-000000000006"
		outputID     = "75910000-0000-0000-0000-000000000007"
		paymentID    = "75910000-0000-0000-0000-000000000008"
	)
	mustAccountingExec(t, ctx, database, `INSERT INTO users (id,email,password_hash,full_name) VALUES ($1,'backfill@test.local','test','Kế toán backfill')`, actorID)
	mustAccountingExec(t, ctx, database, `INSERT INTO projects (id,name,code,client_name,status) VALUES ($1,'Công trình backfill','ACC-BACKFILL','Khách hàng','planning')`, projectID)
	mustAccountingExec(t, ctx, database, `
		INSERT INTO accounting_budgets (id,project_id,budget_code,name,planned_amount,approved_amount,used_amount,remaining_amount,status,approved_by)
		VALUES ($1,$2,'NS-BF-01','Ngân sách cũ',500000,500000,300000,200000,'approved',$3)
	`, budgetID, projectID, actorID)
	mustAccountingExec(t, ctx, database, `
		INSERT INTO payables (id,project_id,payable_code,vendor_type,vendor_name,gross_amount,remaining_amount,status,budget_id,approved_by,created_by)
		VALUES ($1,$2,'PT-BF-01','supplier','Nhà cung cấp',110000,110000,'approved',$3,$4,$4)
	`, payableID, projectID, budgetID, actorID)
	mustAccountingExec(t, ctx, database, `
		INSERT INTO input_invoices (
			id,project_id,payable_id,invoice_number,invoice_date,vendor_name,amount_before_vat,vat_amount,total_amount,
			status,storage_key,file_name,file_size,checksum_sha256,uploaded_by,verified_by,verified_at
		) VALUES ($1,$2,$3,'HD-BF-01',CURRENT_DATE,'Nhà cung cấp',100000,10000,110000,
			'verified','bf/input.pdf','input.pdf',1,repeat('c',64),$4,$4,now())
	`, inputID, projectID, payableID, actorID)
	mustAccountingExec(t, ctx, database, `
		INSERT INTO receivables (id,project_id,receivable_code,client_name,gross_amount,remaining_amount,status,approved_by,created_by)
		VALUES ($1,$2,'PTHU-BF-01','Khách hàng',220000,220000,'approved',$3,$3)
	`, receivableID, projectID, actorID)
	mustAccountingExec(t, ctx, database, `
		INSERT INTO output_invoices (
			id,project_id,receivable_id,invoice_number,invoice_date,client_name,amount_before_vat,vat_amount,total_amount,
			status,storage_key,file_name,file_size,checksum_sha256,uploaded_by,issued_by,issued_at
		) VALUES ($1,$2,$3,'HDR-BF-01',CURRENT_DATE,'Khách hàng',200000,20000,220000,
			'issued','bf/output.pdf','output.pdf',1,repeat('d',64),$4,$4,now())
	`, outputID, projectID, receivableID, actorID)
	mustAccountingExec(t, ctx, database, `
		INSERT INTO payments (id,project_id,payment_code,payment_type,amount,payment_method,status,budget_id,paid_at,created_by,approved_by)
		VALUES ($1,$2,'PC-BF-01','outbound',50000,'bank_transfer','approved',$3,now(),$4,$4)
	`, paymentID, projectID, budgetID, actorID)

	mustAccountingExec(t, ctx, database, `SELECT backfill_accounting_ledger()`)
	assertAccountingAmount(t, ctx, database, `SELECT COUNT(*) FROM journal_entries WHERE project_id=$1 AND status='posted'`, 3, projectID)
	assertAccountingAmount(t, ctx, database, `
		SELECT COALESCE(SUM(line.debit_amount),0)-COALESCE(SUM(line.credit_amount),0)
		FROM journal_entry_lines line JOIN journal_entries entry ON entry.id=line.entry_id WHERE entry.project_id=$1
	`, 0, projectID)
	assertAccountingAmount(t, ctx, database, `SELECT actual_amount FROM accounting_budget_balances WHERE id=$1`, 300000, budgetID)
	assertAccountingAmount(t, ctx, database, `SELECT paid_amount FROM accounting_budget_balances WHERE id=$1`, 50000, budgetID)
	assertAccountingAmount(t, ctx, database, `SELECT COUNT(*) FROM accounting_backfill_issues WHERE project_id=$1`, 0, projectID)

	mustAccountingExec(t, ctx, database, `SELECT backfill_accounting_ledger()`)
	assertAccountingAmount(t, ctx, database, `SELECT COUNT(*) FROM journal_entries WHERE project_id=$1 AND status='posted'`, 3, projectID)
	assertAccountingAmount(t, ctx, database, `SELECT COUNT(*) FROM budget_movements WHERE budget_id=$1`, 3, budgetID)
}

func TestVATFilingAdjustmentSupplementAndTaxPayment(t *testing.T) {
	database := openIsolatedAccountingTestDB(t)
	repo := NewRepository(database)
	service := NewService(repo, "test-secret")
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	const (
		actorID      = "75920000-0000-0000-0000-000000000001"
		projectID    = "75920000-0000-0000-0000-000000000002"
		payableID    = "75920000-0000-0000-0000-000000000003"
		inputID      = "75920000-0000-0000-0000-000000000004"
		receivableID = "75920000-0000-0000-0000-000000000005"
		outputID     = "75920000-0000-0000-0000-000000000006"
	)
	mustAccountingExec(t, ctx, database, `INSERT INTO users (id,email,password_hash,full_name,status) VALUES ($1,'vat-filing@test.local','test','Kế toán thuế kiểm thử','active')`, actorID)
	mustAccountingExec(t, ctx, database, `INSERT INTO projects (id,name,code,client_name,status) VALUES ($1,'Công trình kiểm thử thuế','VAT-FILING-TEST','Khách hàng kiểm thử','planning')`, projectID)
	mustAccountingExec(t, ctx, database, `
		INSERT INTO payables (id,project_id,payable_code,vendor_type,vendor_name,gross_amount,remaining_amount,status,created_by)
		VALUES ($1,$2,'PT-VAT-01','supplier','Nhà cung cấp VAT',110000,110000,'approved',$3)
	`, payableID, projectID, actorID)
	mustAccountingExec(t, ctx, database, `
		INSERT INTO input_invoices (
			id,project_id,payable_id,invoice_number,invoice_date,vendor_name,amount_before_vat,vat_amount,total_amount,
			status,storage_key,file_name,file_size,checksum_sha256,uploaded_by
		) VALUES ($1,$2,$3,'HDV-VAT-01',CURRENT_DATE,'Nhà cung cấp VAT',100000,10000,110000,
			'draft','vat/input.pdf','input.pdf',1,repeat('a',64),$4)
	`, inputID, projectID, payableID, actorID)
	if _, err := repo.InputInvoiceAction(ctx, inputID, "verify", ActionRequest{}, actorID, "Kế toán thuế kiểm thử"); err != nil {
		t.Fatal(err)
	}
	mustAccountingExec(t, ctx, database, `
		INSERT INTO receivables (id,project_id,receivable_code,client_name,contract_amount,gross_amount,remaining_amount,status,created_by)
		VALUES ($1,$2,'PTHU-VAT-01','Khách hàng VAT',220000,220000,220000,'approved',$3)
	`, receivableID, projectID, actorID)
	mustAccountingExec(t, ctx, database, `
		INSERT INTO output_invoices (
			id,project_id,receivable_id,invoice_number,invoice_date,client_name,amount_before_vat,vat_amount,total_amount,
			status,storage_key,file_name,file_size,checksum_sha256,uploaded_by
		) VALUES ($1,$2,$3,'HDR-VAT-01',CURRENT_DATE,'Khách hàng VAT',200000,20000,220000,
			'draft','vat/output.pdf','output.pdf',1,repeat('b',64),$4)
	`, outputID, projectID, receivableID, actorID)
	if _, err := repo.OutputInvoiceAction(ctx, outputID, "issue", ActionRequest{}, actorID, "Kế toán thuế kiểm thử"); err != nil {
		t.Fatal(err)
	}

	var periodID, taxCodeID, offsetAccountID string
	if err := database.QueryRowContext(ctx, `
		SELECT id::text FROM tax_periods
		WHERE period_type='month' AND CURRENT_DATE BETWEEN date_from AND date_to
	`).Scan(&periodID); err != nil {
		t.Fatal(err)
	}
	if err := database.QueryRowContext(ctx, `SELECT id::text FROM tax_codes WHERE tax_code='VAT-10' LIMIT 1`).Scan(&taxCodeID); err != nil {
		t.Fatal(err)
	}
	if err := database.QueryRowContext(ctx, `SELECT id::text FROM accounting_accounts WHERE account_code='642' LIMIT 1`).Scan(&offsetAccountID); err != nil {
		t.Fatal(err)
	}

	detail, err := repo.AccountingTaxPeriod(ctx, periodID)
	if err != nil {
		t.Fatal(err)
	}
	summary := detail["summary"].(Record)
	if number(summary["deductible_input_vat_amount"]) != 10000 || number(summary["output_vat_amount"]) != 20000 || number(summary["vat_payable"]) != 10000 || number(summary["blocker_count"]) != 0 {
		t.Fatalf("initial VAT summary=%v, want deductible 10000, output 20000, payable preview 10000, no blockers", summary)
	}
	adjustment, err := service.CreateAccountingTaxAdjustment(ctx, periodID, AccountingTaxAdjustmentCreateRequest{
		ProjectID: projectID, Direction: "input", TaxCodeID: taxCodeID, OffsetAccountID: offsetAccountID,
		EntryDate: time.Now().Format("2006-01-02"), VATDelta: 1000, Reason: "Bổ sung thuế đầu vào đủ điều kiện khấu trừ", SourceDocumentNo: "ĐC-VAT-01",
	}, actorID, "Kế toán thuế kiểm thử")
	if err != nil {
		t.Fatal(err)
	}
	assertBalancedSourceJournal(t, ctx, database, "tax_adjustment", cleanID(adjustment["id"]), "tax_adjustment_posted", 1000)

	prepared, err := service.AccountingTaxPeriodAction(ctx, periodID, "prepare", AccountingTaxPeriodActionRequest{Reason: "Đã đối chiếu hóa đơn với TK 1331 và 3331"}, actorID, "Kế toán thuế kiểm thử")
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(prepared["status"]) != "prepared" {
		t.Fatalf("prepared period=%v", prepared)
	}
	assertAccountingAmount(t, ctx, database, `SELECT vat_payable FROM vat_return_snapshots WHERE tax_period_id=$1 AND amendment_no=0`, 9000, periodID)
	if _, err := service.CreateAccountingTaxAdjustment(ctx, periodID, AccountingTaxAdjustmentCreateRequest{
		ProjectID: projectID, Direction: "input", TaxCodeID: taxCodeID, OffsetAccountID: offsetAccountID,
		VATDelta: 1, Reason: "Không được sửa sau chốt",
	}, actorID, "Kế toán thuế kiểm thử"); err == nil || !strings.Contains(err.Error(), "tờ khai bổ sung") {
		t.Fatalf("adjustment after prepare error=%v, want supplemental return gate", err)
	}
	if _, err := service.AccountingTaxPeriodAction(ctx, periodID, "file", AccountingTaxPeriodActionRequest{Reason: "Đã nhận thông báo tiếp nhận", FilingReference: "GDT-2026-0001"}, actorID, "Kế toán thuế kiểm thử"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.AccountingTaxPeriodAction(ctx, periodID, "lock", AccountingTaxPeriodActionRequest{Reason: "Đã hoàn tất hồ sơ khai thuế kỳ"}, actorID, "Kế toán thuế kiểm thử"); err != nil {
		t.Fatal(err)
	}

	taxPayment, err := service.CreateAccountingTaxPayment(ctx, periodID, AccountingTaxPaymentCreateRequest{
		ProjectID: projectID, PaymentCode: "NT-VAT-01", Amount: 9000, PaymentMethod: "bank_transfer",
	}, actorID, "Kế toán thuế kiểm thử")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.PaymentAction(ctx, cleanID(taxPayment["id"]), "approve", ActionRequest{}, actorID, "Kế toán thuế kiểm thử"); err != nil {
		t.Fatal(err)
	}
	assertBalancedSourceJournal(t, ctx, database, "payment", cleanID(taxPayment["id"]), "vat_payment_approved", 9000)
	assertAccountingAmount(t, ctx, database, `SELECT COUNT(*) FROM budget_movements WHERE source_type='payment' AND source_id=$1`, 0, taxPayment["id"])

	if _, err := service.AccountingTaxPeriodAction(ctx, periodID, "start_amendment", AccountingTaxPeriodActionRequest{Reason: "Phát hiện chứng từ điều chỉnh cần đảo"}, actorID, "Kế toán thuế kiểm thử"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ReverseAccountingTaxAdjustment(ctx, cleanID(adjustment["id"]), AccountingTaxAdjustmentReverseRequest{
		EntryDate: time.Now().Format("2006-01-02"), Reason: "Chứng từ điều chỉnh không đủ điều kiện khấu trừ",
	}, actorID, "Kế toán thuế kiểm thử"); err != nil {
		t.Fatal(err)
	}
	if _, err := database.ExecContext(ctx, `UPDATE tax_adjustments SET reason='Không được sửa' WHERE id=$1`, adjustment["id"]); err == nil || !strings.Contains(err.Error(), "bất biến") {
		t.Fatalf("tax adjustment mutation error=%v, want immutable rejection", err)
	}
	if _, err := service.AccountingTaxPeriodAction(ctx, periodID, "prepare", AccountingTaxPeriodActionRequest{Reason: "Chốt số liệu tờ khai bổ sung lần 1"}, actorID, "Kế toán thuế kiểm thử"); err != nil {
		t.Fatal(err)
	}
	assertAccountingAmount(t, ctx, database, `SELECT vat_payable FROM vat_return_snapshots WHERE tax_period_id=$1 AND amendment_no=1`, 10000, periodID)
	assertAccountingAmount(t, ctx, database, `SELECT COUNT(*) FROM vat_return_snapshots WHERE tax_period_id=$1`, 2, periodID)
}

func openIsolatedAccountingTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("CONSFLOW_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set CONSFLOW_TEST_DATABASE_URL to run DB-backed accounting tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	adminDB, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	schema := "accounting_test_" + strings.ReplaceAll(newID(), "-", "")
	if _, err := adminDB.ExecContext(ctx, `CREATE SCHEMA `+schema); err != nil {
		_ = adminDB.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = adminDB.ExecContext(context.Background(), `DROP SCHEMA `+schema+` CASCADE`)
		_ = adminDB.Close()
	})
	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	query := parsed.Query()
	// Keep the test schema first while retaining extension functions installed in public
	// (for example pgcrypto.digest used by immutable migration definitions).
	query.Set("search_path", schema+",public")
	parsed.RawQuery = query.Encode()
	database, err := sql.Open("pgx", parsed.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	if _, err := migrations.Run(ctx, database, filepath.Join("..", "..", "migrations")); err != nil {
		t.Fatal(err)
	}
	return database
}

func mustAccountingExec(t *testing.T, ctx context.Context, database *sql.DB, query string, args ...any) {
	t.Helper()
	if _, err := database.ExecContext(ctx, query, args...); err != nil {
		t.Fatal(err)
	}
}

func assertAccountingAmount(t *testing.T, ctx context.Context, database *sql.DB, query string, want float64, args ...any) {
	t.Helper()
	var got float64
	if err := database.QueryRowContext(ctx, query, args...).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if mathAbs(got-want) > 0.005 {
		t.Fatalf("query %q amount=%v, want %v", query, got, want)
	}
}

func assertBalancedSourceJournal(t *testing.T, ctx context.Context, database *sql.DB, sourceType, sourceID, sourceEvent string, wantTotal float64) {
	t.Helper()
	var debitTotal float64
	var creditTotal float64
	if err := database.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(line.debit_amount),0),COALESCE(SUM(line.credit_amount),0)
		FROM journal_entries entry
		JOIN journal_entry_lines line ON line.entry_id=entry.id
		WHERE entry.source_type=$1 AND entry.source_id=$2 AND entry.source_event=$3 AND entry.status='posted'
	`, sourceType, sourceID, sourceEvent).Scan(&debitTotal, &creditTotal); err != nil {
		t.Fatal(err)
	}
	if mathAbs(debitTotal-wantTotal) > 0.005 || mathAbs(creditTotal-wantTotal) > 0.005 {
		t.Fatalf("%s/%s journal Nợ=%v Có=%v, want %v", sourceType, sourceEvent, debitTotal, creditTotal, wantTotal)
	}
}

func mathAbs(value float64) float64 {
	if value < 0 {
		return -value
	}
	return value
}

func TestAccountingPostingTestSchemaNameIsSafe(t *testing.T) {
	name := "accounting_test_" + strings.ReplaceAll(newID(), "-", "")
	if strings.ContainsAny(name, `;\"' `) {
		t.Fatalf("unsafe generated schema name %q", name)
	}
	_ = fmt.Sprint(name)
}
