package core

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestBatch4AccountingListUsesProjectScope(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	repo := NewRepository(db)
	handler := NewHandler(NewService(repo, "test-secret"))
	fixture := setupProjectScopeFixture(t, db, []string{"accounting.view", "accounting.payable_view"})
	mustExec(t, db, `
		INSERT INTO payables (project_id, payable_code, vendor_type, vendor_name, source_type, gross_amount, remaining_amount, due_date, status, created_by)
		VALUES ($1,$2,'supplier','Scoped Vendor','manual',100,100,'2000-01-01','approved',$3),
		       ($4,$5,'supplier','Other Vendor','manual',200,200,'2000-01-01','approved',$3)
	`, fixture.projectID, "PAY-S-"+fixture.projectID[:8], fixture.userID, fixture.otherProjectID, "PAY-O-"+fixture.otherProjectID[:8])
	app := batch4TestApp(fixture.userID, func(app *fiber.App, h *Handler) {
		app.Get("/payables", h.ListPayables)
	}, handler)

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/payables?project_id="+fixture.projectID, nil))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var body struct {
		Success bool     `json:"success"`
		Data    []Record `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if !body.Success {
		t.Fatal("expected success response")
	}
	if len(body.Data) != 1 {
		t.Fatalf("expected one scoped payable, got %d", len(body.Data))
	}
	if body.Data[0]["project_id"] != fixture.projectID {
		t.Fatalf("project_id = %v, want %s", body.Data[0]["project_id"], fixture.projectID)
	}
	if body.Data[0]["aging_bucket"] != "90_plus" {
		t.Fatalf("aging_bucket = %v, want 90_plus", body.Data[0]["aging_bucket"])
	}
	if number(body.Data[0]["overdue_days"]) <= 0 {
		t.Fatalf("overdue_days = %v, want positive", body.Data[0]["overdue_days"])
	}
	if body.Data[0]["reconciliation_status"] != "needs_review" {
		t.Fatalf("reconciliation_status = %v, want needs_review", body.Data[0]["reconciliation_status"])
	}
}

func TestReceivableListUsesProjectScopeAndAging(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	repo := NewRepository(db)
	handler := NewHandler(NewService(repo, "test-secret"))
	fixture := setupProjectScopeFixture(t, db, []string{"accounting.view", "accounting.receivable_view"})
	// Give the actor visibility to both projects so this test proves the explicit
	// project_id query filter instead of passing only because of membership scope.
	mustExec(t, db, `INSERT INTO project_members (project_id, staff_id, role_ids, active) VALUES ($1,$2,'["Member"]'::jsonb,true)`, fixture.otherProjectID, fixture.staffID)
	mustExec(t, db, `
		INSERT INTO receivables (project_id, receivable_code, client_name, source_type, gross_amount, received_amount, remaining_amount, due_date, status, created_by)
		VALUES ($1,$2,'Scoped Client','manual',100,0,100,'2000-01-01','approved',$3),
		       ($4,$5,'Other Client','manual',200,0,200,'2000-01-01','approved',$3)
	`, fixture.projectID, "REC-S-"+fixture.projectID[:8], fixture.userID, fixture.otherProjectID, "REC-O-"+fixture.otherProjectID[:8])
	app := batch4TestApp(fixture.userID, func(app *fiber.App, h *Handler) {
		app.Get("/receivables", h.ListReceivables)
	}, handler)

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/receivables?project_id="+fixture.projectID, nil))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var body struct {
		Success bool     `json:"success"`
		Data    []Record `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if !body.Success {
		t.Fatal("expected success response")
	}
	if len(body.Data) != 1 {
		t.Fatalf("expected one scoped receivable, got %d", len(body.Data))
	}
	if body.Data[0]["project_id"] != fixture.projectID {
		t.Fatalf("project_id = %v, want %s", body.Data[0]["project_id"], fixture.projectID)
	}
	if body.Data[0]["aging_bucket"] != "90_plus" {
		t.Fatalf("aging_bucket = %v, want 90_plus", body.Data[0]["aging_bucket"])
	}
	if number(body.Data[0]["overdue_days"]) <= 0 {
		t.Fatalf("overdue_days = %v, want positive", body.Data[0]["overdue_days"])
	}
	if body.Data[0]["reconciliation_status"] != "needs_review" {
		t.Fatalf("reconciliation_status = %v, want needs_review", body.Data[0]["reconciliation_status"])
	}
}

func TestInputInvoicePDFUploadLinksPayableDocumentAndStatus(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	service := NewService(NewRepository(db), "test-secret")
	uploadDir := t.TempDir()
	service.ConfigureUploads(uploadDir, 1024*1024)
	fixture := setupProjectScopeFixture(t, db, []string{"accounting.view", "accounting.payable_view", "accounting.payable_approve"})
	payableID := newID()
	otherProcurementID := newID()
	mustExec(t, db, `
		INSERT INTO payables (id, project_id, payable_code, vendor_type, vendor_name, source_type, gross_amount, remaining_amount, status, created_by)
		VALUES ($1,$2,$3,'supplier','VAT Vendor','manual',1100000,1100000,'approved',$4)
	`, payableID, fixture.projectID, "PAY-IN-"+payableID[:8], fixture.userID)
	mustExec(t, db, `
		INSERT INTO procurement_requests (id, project_id, request_code, title, requested_by, status, total_estimated_amount)
		VALUES ($1,$2,$3,'Other procurement',$4,'approved',1100000)
	`, otherProcurementID, fixture.otherProjectID, "PR-OTHER-"+otherProcurementID[:8], fixture.userID)

	pdfContent := []byte("%PDF-1.4\n% test invoice\n1 0 obj\n<<>>\nendobj\ntrailer\n<<>>\n%%EOF\n")
	invoice, err := service.CreateInputInvoice(ctx, payableID, InputInvoiceRequest{
		InvoiceNumber:   "VAT-001",
		InvoiceSymbol:   "C25TAA",
		InvoiceDate:     "2026-06-22",
		VendorName:      "VAT Vendor",
		VendorTaxCode:   "0312345678",
		AmountBeforeVAT: 1000000,
		VATAmount:       100000,
		TotalAmount:     1100000,
		Notes:           "Hoá đơn mua vật tư",
	}, testMultipartFileHeader(t, "file", "vat-invoice.pdf", pdfContent), fixture.userID, "Scoped User")
	if err != nil {
		t.Fatal(err)
	}
	if invoice["status"] != "draft" {
		t.Fatalf("status = %v, want draft", invoice["status"])
	}
	invoiceID := cleanID(invoice["id"])
	documentID := cleanID(invoice["document_id"])
	if invoiceID == "" || documentID == "" {
		t.Fatalf("expected invoice and document ids, got invoice=%q document=%q", invoiceID, documentID)
	}
	storageKey := fmt.Sprint(invoice["storage_key"])
	if !strings.HasPrefix(storageKey, fmt.Sprintf("input-invoices/%s/%s/", fixture.projectID, payableID)) || strings.Contains(storageKey, "..") {
		t.Fatalf("invalid storage_key = %q", storageKey)
	}
	sum := sha256.Sum256(pdfContent)
	if got := fmt.Sprint(invoice["checksum_sha256"]); got != hex.EncodeToString(sum[:]) {
		t.Fatalf("checksum = %s, want %s", got, hex.EncodeToString(sum[:]))
	}
	assertCount(t, db, `SELECT COUNT(*) FROM documents WHERE id=$1 AND project_id=$2 AND target_type='input_invoice' AND target_id=$3 AND status='active'`, 1, documentID, fixture.projectID, invoiceID)
	assertCount(t, db, `SELECT COUNT(*) FROM audit_logs WHERE project_id=$1 AND event_type='input_invoice_uploaded' AND target_id=$2`, 1, fixture.projectID, invoiceID)

	file, err := service.InputInvoiceFile(ctx, invoiceID, fixture.userID)
	if err != nil {
		t.Fatal(err)
	}
	if file.MIMEType != "application/pdf" || file.Filename != "vat-invoice.pdf" {
		t.Fatalf("file result = %#v", file)
	}
	storedBytes, err := io.ReadAll(file.Reader)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Reader.Close(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(storedBytes, pdfContent) {
		t.Fatal("stored invoice PDF bytes do not match upload")
	}
	if file.Size != int64(len(pdfContent)) {
		t.Fatalf("file size = %d, want %d", file.Size, len(pdfContent))
	}

	payableInvoices, err := service.PayableInputInvoices(ctx, payableID, fixture.userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(payableInvoices) != 1 || cleanID(payableInvoices[0]["id"]) != invoiceID {
		t.Fatalf("payable invoices = %#v", payableInvoices)
	}
	projectInvoices, err := service.ProjectInputInvoices(ctx, fixture.projectID, fixture.userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(projectInvoices) != 1 || cleanID(projectInvoices[0]["id"]) != invoiceID {
		t.Fatalf("project invoices = %#v", projectInvoices)
	}

	verified, err := service.InputInvoiceAction(ctx, invoiceID, "verify", ActionRequest{}, fixture.userID, "Scoped User")
	if err != nil {
		t.Fatal(err)
	}
	if verified["status"] != "verified" || cleanID(verified["verified_by"]) != fixture.userID {
		t.Fatalf("verified invoice = %#v", verified)
	}
	rejected, err := service.InputInvoiceAction(ctx, invoiceID, "reject", ActionRequest{}, fixture.userID, "Scoped User")
	if err != nil {
		t.Fatal(err)
	}
	if rejected["status"] != "rejected" || cleanID(rejected["rejected_by"]) != fixture.userID || cleanID(rejected["verified_by"]) != "" {
		t.Fatalf("rejected invoice = %#v", rejected)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM audit_logs WHERE project_id=$1 AND target_id=$2 AND event_type IN ('input_invoice_verified','input_invoice_rejected')`, 2, fixture.projectID, invoiceID)

	_, err = service.CreateInputInvoice(ctx, payableID, InputInvoiceRequest{
		InvoiceNumber:        "VAT-OTHER-PROC",
		InvoiceDate:          "2026-06-22",
		VendorName:           "VAT Vendor",
		AmountBeforeVAT:      1000000,
		VATAmount:            100000,
		TotalAmount:          1100000,
		ProcurementRequestID: otherProcurementID,
	}, testMultipartFileHeader(t, "file", "other-proc.pdf", pdfContent), fixture.userID, "Scoped User")
	assertValidationError(t, err)
}

func TestOutputInvoicePDFUploadLinksReceivablePaymentDocumentAndStatus(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	service := NewService(NewRepository(db), "test-secret")
	uploadDir := t.TempDir()
	service.ConfigureUploads(uploadDir, 1024*1024)
	fixture := setupProjectScopeFixture(t, db, []string{"accounting.view", "accounting.receivable_view", "accounting.receivable_approve"})
	receivableID := newID()
	paymentID := newID()
	otherPaymentID := newID()
	mustExec(t, db, `
		INSERT INTO receivables (id, project_id, receivable_code, client_name, source_type, contract_amount, milestone_name, gross_amount, received_amount, remaining_amount, status, created_by)
		VALUES ($1,$2,$3,'VAT Client','manual',2200000,'Đợt 1',2200000,0,2200000,'approved',$4)
	`, receivableID, fixture.projectID, "REC-OUT-"+receivableID[:8], fixture.userID)
	mustExec(t, db, `
		INSERT INTO payments (id, project_id, payment_code, payment_type, target_type, target_id, amount, payment_method, status, created_by)
		VALUES ($1,$2,$3,'client_receipt','receivable',$4,2200000,'bank_transfer','received',$5)
	`, paymentID, fixture.projectID, "RCV-OUT-"+paymentID[:8], receivableID, fixture.userID)
	mustExec(t, db, `
		INSERT INTO payments (id, project_id, payment_code, payment_type, target_type, target_id, amount, payment_method, status, created_by)
		VALUES ($1,$2,$3,'client_receipt','receivable',$4,2200000,'bank_transfer','received',$5)
	`, otherPaymentID, fixture.otherProjectID, "RCV-OTH-"+otherPaymentID[:8], receivableID, fixture.userID)

	pdfContent := []byte("%PDF-1.4\n% test output invoice\n1 0 obj\n<<>>\nendobj\ntrailer\n<<>>\n%%EOF\n")
	invoice, err := service.CreateOutputInvoice(ctx, receivableID, OutputInvoiceRequest{
		InvoiceNumber:   "OUT-001",
		InvoiceSymbol:   "C25TAA",
		InvoiceDate:     "2026-06-22",
		ClientName:      "VAT Client",
		ClientTaxCode:   "0311111111",
		AmountBeforeVAT: 2000000,
		VATAmount:       200000,
		TotalAmount:     2200000,
		PaymentID:       paymentID,
		Notes:           "Hoá đơn đầu ra đợt 1",
	}, testMultipartFileHeader(t, "file", "output-invoice.pdf", pdfContent), fixture.userID, "Scoped User")
	if err != nil {
		t.Fatal(err)
	}
	if invoice["status"] != "draft" {
		t.Fatalf("status = %v, want draft", invoice["status"])
	}
	invoiceID := cleanID(invoice["id"])
	documentID := cleanID(invoice["document_id"])
	if invoiceID == "" || documentID == "" {
		t.Fatalf("expected invoice and document ids, got invoice=%q document=%q", invoiceID, documentID)
	}
	if cleanID(invoice["payment_id"]) != paymentID {
		t.Fatalf("payment_id = %v, want %s", invoice["payment_id"], paymentID)
	}
	storageKey := fmt.Sprint(invoice["storage_key"])
	if !strings.HasPrefix(storageKey, fmt.Sprintf("output-invoices/%s/%s/", fixture.projectID, receivableID)) || strings.Contains(storageKey, "..") {
		t.Fatalf("invalid storage_key = %q", storageKey)
	}
	sum := sha256.Sum256(pdfContent)
	if got := fmt.Sprint(invoice["checksum_sha256"]); got != hex.EncodeToString(sum[:]) {
		t.Fatalf("checksum = %s, want %s", got, hex.EncodeToString(sum[:]))
	}
	assertCount(t, db, `SELECT COUNT(*) FROM documents WHERE id=$1 AND project_id=$2 AND target_type='output_invoice' AND target_id=$3 AND status='active'`, 1, documentID, fixture.projectID, invoiceID)
	assertCount(t, db, `SELECT COUNT(*) FROM audit_logs WHERE project_id=$1 AND event_type='output_invoice_uploaded' AND target_id=$2`, 1, fixture.projectID, invoiceID)

	file, err := service.OutputInvoiceFile(ctx, invoiceID, fixture.userID)
	if err != nil {
		t.Fatal(err)
	}
	if file.MIMEType != "application/pdf" || file.Filename != "output-invoice.pdf" {
		t.Fatalf("file result = %#v", file)
	}
	storedBytes, err := io.ReadAll(file.Reader)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Reader.Close(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(storedBytes, pdfContent) {
		t.Fatal("stored output invoice PDF bytes do not match upload")
	}
	if file.Size != int64(len(pdfContent)) {
		t.Fatalf("file size = %d, want %d", file.Size, len(pdfContent))
	}

	receivableInvoices, err := service.ReceivableOutputInvoices(ctx, receivableID, fixture.userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(receivableInvoices) != 1 || cleanID(receivableInvoices[0]["id"]) != invoiceID {
		t.Fatalf("receivable invoices = %#v", receivableInvoices)
	}
	projectInvoices, err := service.ProjectOutputInvoices(ctx, fixture.projectID, fixture.userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(projectInvoices) != 1 || cleanID(projectInvoices[0]["id"]) != invoiceID {
		t.Fatalf("project invoices = %#v", projectInvoices)
	}

	issued, err := service.OutputInvoiceAction(ctx, invoiceID, "issue", ActionRequest{}, fixture.userID, "Scoped User")
	if err != nil {
		t.Fatal(err)
	}
	if issued["status"] != "issued" || cleanID(issued["issued_by"]) != fixture.userID {
		t.Fatalf("issued invoice = %#v", issued)
	}
	voided, err := service.OutputInvoiceAction(ctx, invoiceID, "void", ActionRequest{}, fixture.userID, "Scoped User")
	if err != nil {
		t.Fatal(err)
	}
	if voided["status"] != "voided" || cleanID(voided["voided_by"]) != fixture.userID || cleanID(voided["issued_by"]) != fixture.userID {
		t.Fatalf("voided invoice = %#v", voided)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM audit_logs WHERE project_id=$1 AND target_id=$2 AND event_type IN ('output_invoice_issued','output_invoice_voided')`, 2, fixture.projectID, invoiceID)

	_, err = service.CreateOutputInvoice(ctx, receivableID, OutputInvoiceRequest{
		InvoiceNumber:   "OUT-OTHER-PAYMENT",
		InvoiceDate:     "2026-06-22",
		ClientName:      "VAT Client",
		AmountBeforeVAT: 2000000,
		VATAmount:       200000,
		TotalAmount:     2200000,
		PaymentID:       otherPaymentID,
	}, testMultipartFileHeader(t, "file", "other-payment.pdf", pdfContent), fixture.userID, "Scoped User")
	assertValidationError(t, err)
}

func TestBatch7AccountingActionsRequireApprovalBeforeSettlement(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)

	actorID := newID()
	projectID := newID()
	payableID := newID()
	receivableID := newID()
	paymentID := newID()
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM payments WHERE id=$1`, paymentID)
		_, _ = db.ExecContext(ctx, `DELETE FROM receivables WHERE id=$1`, receivableID)
		_, _ = db.ExecContext(ctx, `DELETE FROM payables WHERE id=$1`, payableID)
		_, _ = db.ExecContext(ctx, `DELETE FROM projects WHERE id=$1`, projectID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id=$1`, actorID)
	})
	mustExec(t, db, `INSERT INTO users (id, email, password_hash, full_name, status) VALUES ($1,$2,'demo:pw','Batch7 Actor','active')`, actorID, "batch7-accounting-"+actorID+"@test.local")
	mustExec(t, db, `INSERT INTO projects (id, name, code, client_name, status) VALUES ($1,'Batch7 Accounting',$2,'Client','in_progress')`, projectID, "B7C-"+projectID[:8])
	mustExec(t, db, `
		INSERT INTO payables (id, project_id, payable_code, vendor_type, vendor_name, source_type, gross_amount, paid_amount, remaining_amount, status, created_by)
		VALUES ($1,$2,$3,'supplier','Batch7 Supplier','manual',100,0,100,'draft',$4)
	`, payableID, projectID, "B7PAY-"+payableID[:8], actorID)
	mustExec(t, db, `
		INSERT INTO receivables (id, project_id, receivable_code, client_name, source_type, gross_amount, received_amount, remaining_amount, status, created_by)
		VALUES ($1,$2,$3,'Batch7 Client','manual',120,0,120,'draft',$4)
	`, receivableID, projectID, "B7REC-"+receivableID[:8], actorID)
	mustExec(t, db, `
		INSERT INTO payments (id, project_id, payment_code, payment_type, target_type, target_id, amount, payment_method, status, created_by)
		VALUES ($1,$2,$3,'outbound','payable',$4,100,'bank_transfer','draft',$5)
	`, paymentID, projectID, "B7PMT-"+paymentID[:8], payableID, actorID)

	_, err := repo.PayableAction(ctx, payableID, "pay", ActionRequest{}, actorID, "Batch7 Actor")
	assertValidationError(t, err)
	payable, err := repo.PayableAction(ctx, payableID, "approve", ActionRequest{Comment: "Duyệt phải trả"}, actorID, "Batch7 Actor")
	if err != nil {
		t.Fatal(err)
	}
	if payable["status"] != "approved" {
		t.Fatalf("payable status = %v, want approved", payable["status"])
	}
	payable, err = repo.PayableAction(ctx, payableID, "pay", ActionRequest{Comment: "Tất toán"}, actorID, "Batch7 Actor")
	if err != nil {
		t.Fatal(err)
	}
	if payable["status"] != "paid" || number(payable["remaining_amount"]) != 0 || number(payable["paid_amount"]) != 100 {
		t.Fatalf("payable settlement = status %v paid %v remaining %v, want paid/100/0", payable["status"], payable["paid_amount"], payable["remaining_amount"])
	}

	_, err = repo.ReceivableAction(ctx, receivableID, "receive", ActionRequest{}, actorID, "Batch7 Actor")
	assertValidationError(t, err)
	receivable, err := repo.ReceivableAction(ctx, receivableID, "approve", ActionRequest{Comment: "Duyệt phải thu"}, actorID, "Batch7 Actor")
	if err != nil {
		t.Fatal(err)
	}
	if receivable["status"] != "approved" {
		t.Fatalf("receivable status = %v, want approved", receivable["status"])
	}
	receivable, err = repo.ReceivableAction(ctx, receivableID, "receive", ActionRequest{Comment: "Đã thu"}, actorID, "Batch7 Actor")
	if err != nil {
		t.Fatal(err)
	}
	if receivable["status"] != "received" || number(receivable["remaining_amount"]) != 0 || number(receivable["received_amount"]) != 120 {
		t.Fatalf("receivable settlement = status %v received %v remaining %v, want received/120/0", receivable["status"], receivable["received_amount"], receivable["remaining_amount"])
	}

	payment, err := repo.PaymentAction(ctx, paymentID, "approve", ActionRequest{Comment: "Duyệt phiếu chi"}, actorID, "Batch7 Actor")
	if err != nil {
		t.Fatal(err)
	}
	if payment["status"] != "approved" || cleanID(payment["approved_by"]) != actorID || payment["paid_at"] == nil {
		t.Fatalf("payment = status %v approved_by %v paid_at %v, want approved actor paid_at", payment["status"], payment["approved_by"], payment["paid_at"])
	}
	assertCount(t, db, `SELECT COUNT(*) FROM audit_logs WHERE project_id=$1 AND event_type IN ('payable_approved','payable_paid','receivable_approved','receivable_received','payment_approved')`, 5, projectID)
}

func TestFinancialCreationIgnoresClientDerivedStateAndAuditsActor(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")
	fixture := setupProjectScopeFixture(t, db, []string{"accounting.payable_approve", "accounting.receivable_approve", "accounting.payment_create"})
	suffix := strings.ToUpper(strings.ReplaceAll(newID(), "-", ""))[:8]

	payable, err := service.CreatePayable(ctx, PayableCreateRequest{
		ProjectID: fixture.projectID, PayableCode: "PAY-HARD-" + suffix, VendorType: "supplier", VendorName: "Nhà cung cấp hardening",
		GrossAmount: 1000, AdvanceAmount: 100, DeductionAmount: 50, RetentionAmount: 25,
	}, fixture.userID, "Scoped User")
	if err != nil {
		t.Fatal(err)
	}
	if payable["status"] != "draft" || number(payable["paid_amount"]) != 0 || number(payable["remaining_amount"]) != 850 {
		t.Fatalf("payable derived state = status %v paid %v remaining %v, want draft/0/850", payable["status"], payable["paid_amount"], payable["remaining_amount"])
	}

	receivable, err := service.CreateReceivable(ctx, ReceivableCreateRequest{
		ProjectID: fixture.projectID, ReceivableCode: "REC-HARD-" + suffix, ClientName: "Chủ đầu tư hardening",
		ContractAmount: 2000, MilestoneName: "Đợt 1", GrossAmount: 600,
	}, fixture.userID, "Scoped User")
	if err != nil {
		t.Fatal(err)
	}
	if receivable["status"] != "draft" || number(receivable["received_amount"]) != 0 || number(receivable["remaining_amount"]) != 600 {
		t.Fatalf("receivable derived state = status %v received %v remaining %v, want draft/0/600", receivable["status"], receivable["received_amount"], receivable["remaining_amount"])
	}

	payment, err := service.CreatePayment(ctx, PaymentCreateRequest{
		ProjectID: fixture.projectID, PaymentCode: "PMT-HARD-" + suffix, PaymentType: "other",
		Amount: 200, PaymentMethod: "bank_transfer",
	}, fixture.userID, "Scoped User")
	if err != nil {
		t.Fatal(err)
	}
	if payment["status"] != "draft" || payment["approved_by"] != nil {
		t.Fatalf("payment client state leaked = status %v approved_by %v, want draft/nil", payment["status"], payment["approved_by"])
	}
	if _, err := service.CreatePayment(ctx, PaymentCreateRequest{
		ProjectID: fixture.projectID, PaymentCode: "PMT-AR-" + suffix,
		PaymentType: "client_receipt", Amount: 100, PaymentMethod: "bank_transfer",
	}, fixture.userID, "Scoped User"); err == nil {
		t.Fatal("expected generic AP/AR-linked payment type to be rejected")
	}
	assertCount(t, db, `SELECT COUNT(*) FROM audit_logs WHERE project_id=$1 AND actor_id=$2 AND event_type IN ('payable_created','receivable_created','payment_created')`, 3, fixture.projectID, fixture.userID)
}

func TestInputPaymentScheduleAdvanceAndPartialPaymentReconcilePayable(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")
	fixture := setupProjectScopeFixture(t, db, []string{
		"accounting.view",
		"accounting.payable_view",
		"accounting.payable_approve",
		"accounting.payment_create",
		"accounting.payment_approve",
	})
	payableID := newID()
	mustExec(t, db, `
		INSERT INTO payables (id, project_id, payable_code, vendor_type, vendor_name, source_type, gross_amount, paid_amount, remaining_amount, status, created_by)
		VALUES ($1,$2,$3,'supplier','Input Payment Vendor','manual',1000,0,1000,'approved',$4)
	`, payableID, fixture.projectID, "PAY-IP-"+payableID[:8], fixture.userID)

	schedule, err := service.CreatePayablePaymentSchedule(ctx, payableID, PayablePaymentScheduleRequest{
		ScheduleCode:  "SCH-IP-001",
		MilestoneName: "Kỳ vật tư 1",
		ScheduleType:  "progress",
		PlannedAmount: 600,
		DueDate:       "2026-07-15",
	}, fixture.userID, "Scoped User")
	if err != nil {
		t.Fatal(err)
	}
	scheduleID := cleanID(schedule["id"])
	schedule, err = service.PayablePaymentScheduleAction(ctx, scheduleID, "approve", ActionRequest{Comment: "Duyệt kỳ"}, fixture.userID, "Scoped User")
	if err != nil {
		t.Fatal(err)
	}
	if schedule["status"] != "approved" || number(schedule["remaining_amount"]) != 600 {
		t.Fatalf("schedule = status %v remaining %v, want approved/600", schedule["status"], schedule["remaining_amount"])
	}

	advance, err := service.CreatePayableAdvanceRequest(ctx, payableID, AdvanceRequestCreateRequest{
		RequestCode: "ADV-IP-001",
		Amount:      200,
		DueDate:     "2026-07-01",
		Reason:      "Ứng trước cho vendor",
	}, fixture.userID, "Scoped User")
	if err != nil {
		t.Fatal(err)
	}
	advanceID := cleanID(advance["id"])
	advance, err = service.PayableAdvanceRequestAction(ctx, advanceID, "approve", ActionRequest{Comment: "Duyệt ứng"}, fixture.userID, "Scoped User")
	if err != nil {
		t.Fatal(err)
	}
	if advance["status"] != "approved" {
		t.Fatalf("advance status = %v, want approved", advance["status"])
	}
	payable, err := repo.GetPayableDetail(ctx, payableID)
	if err != nil {
		t.Fatal(err)
	}
	if payable["status"] != "partially_paid" || number(payable["advance_amount"]) != 200 || number(payable["remaining_amount"]) != 800 {
		t.Fatalf("payable after advance = status %v advance %v remaining %v, want partially_paid/200/800", payable["status"], payable["advance_amount"], payable["remaining_amount"])
	}

	advance, err = service.SettlePayableAdvanceRequest(ctx, advanceID, AdvanceSettlementRequest{Amount: 50, ScheduleID: scheduleID, Comment: "Đối soát một phần"}, fixture.userID, "Scoped User")
	if err != nil {
		t.Fatal(err)
	}
	if advance["status"] != "partially_settled" || number(advance["settled_amount"]) != 50 {
		t.Fatalf("advance settlement = status %v settled %v, want partially_settled/50", advance["status"], advance["settled_amount"])
	}

	payment, err := service.CreatePayablePayment(ctx, payableID, PayablePaymentCreateRequest{
		PaymentCode:   "PMT-IP-001",
		PaymentType:   "supplier_payment",
		PaymentMethod: "bank_transfer",
		Amount:        300,
		ScheduleID:    scheduleID,
		PaidAt:        "2026-07-20",
		Notes:         "Thanh toán một phần kỳ 1",
	}, fixture.userID, "Scoped User")
	if err != nil {
		t.Fatal(err)
	}
	paymentID := cleanID(payment["id"])
	if payment["status"] != "draft" || payment["allocation_type"] != "payable_payment" {
		t.Fatalf("payment = %#v, want draft payable allocation", payment)
	}
	payment, err = service.PaymentAction(ctx, paymentID, "approve", ActionRequest{Comment: "Duyệt thanh toán"}, fixture.userID, "Scoped User")
	if err != nil {
		t.Fatal(err)
	}
	if payment["status"] != "approved" {
		t.Fatalf("payment status = %v, want approved", payment["status"])
	}
	payable, err = repo.GetPayableDetail(ctx, payableID)
	if err != nil {
		t.Fatal(err)
	}
	if payable["status"] != "partially_paid" || number(payable["paid_amount"]) != 300 || number(payable["remaining_amount"]) != 500 {
		t.Fatalf("payable after payment = status %v paid %v remaining %v, want partially_paid/300/500", payable["status"], payable["paid_amount"], payable["remaining_amount"])
	}
	schedule, err = repo.GetPayablePaymentScheduleDetail(ctx, scheduleID)
	if err != nil {
		t.Fatal(err)
	}
	if schedule["status"] != "partially_paid" || number(schedule["paid_amount"]) != 300 || number(schedule["remaining_amount"]) != 300 {
		t.Fatalf("schedule after payment = status %v paid %v remaining %v, want partially_paid/300/300", schedule["status"], schedule["paid_amount"], schedule["remaining_amount"])
	}

	advancePayment, err := service.CreatePayablePayment(ctx, payableID, PayablePaymentCreateRequest{
		PaymentCode:      "PMT-IP-ADV",
		PaymentType:      "advance",
		PaymentMethod:    "cash",
		Amount:           200,
		AdvanceRequestID: advanceID,
		PaidAt:           "2026-07-02",
	}, fixture.userID, "Scoped User")
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.PaymentAction(ctx, cleanID(advancePayment["id"]), "approve", ActionRequest{Comment: "Chi ứng trước"}, fixture.userID, "Scoped User")
	if err != nil {
		t.Fatal(err)
	}
	advance, err = repo.GetPayableAdvanceRequestDetail(ctx, advanceID)
	if err != nil {
		t.Fatal(err)
	}
	if number(advance["paid_amount"]) != 200 {
		t.Fatalf("advance paid_amount = %v, want 200", advance["paid_amount"])
	}
	payable, err = repo.GetPayableDetail(ctx, payableID)
	if err != nil {
		t.Fatal(err)
	}
	if number(payable["remaining_amount"]) != 500 {
		t.Fatalf("advance payment should not reduce payable twice, remaining = %v, want 500", payable["remaining_amount"])
	}

	_, err = service.CreatePayablePayment(ctx, payableID, PayablePaymentCreateRequest{
		PaymentCode:   "PMT-IP-OVER",
		PaymentMethod: "bank_transfer",
		Amount:        1000,
	}, fixture.userID, "Scoped User")
	assertValidationError(t, err)
	assertCount(t, db, `
		SELECT COUNT(*)
		FROM audit_logs
		WHERE project_id=$1
		  AND event_type IN (
		    'payable_payment_schedule_created',
		    'payable_payment_schedule_approved',
		    'payable_advance_requested',
		    'payable_advance_approved',
		    'payable_advance_settled',
		    'payable_payment_created',
		    'payable_payment_allocated',
		    'payable_advance_payment_recorded'
		  )
	`, 9, fixture.projectID)
}

func TestPayableReconciliationSnapshotIncludesAgingWarningsAndLedger(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")
	fixture := setupProjectScopeFixture(t, db, []string{
		"accounting.view",
		"accounting.payable_view",
		"accounting.payable_approve",
		"accounting.payment_create",
		"accounting.payment_approve",
	})
	payableID := newID()
	invoiceID := newID()
	mustExec(t, db, `
		INSERT INTO payables (id, project_id, payable_code, vendor_type, vendor_name, source_type, gross_amount, paid_amount, remaining_amount, due_date, status, created_by)
		VALUES ($1,$2,$3,'supplier','Ledger Vendor','manual',1000,0,1000,'2000-01-01','approved',$4)
	`, payableID, fixture.projectID, "PAY-LED-"+payableID[:8], fixture.userID)
	mustExec(t, db, `
		INSERT INTO input_invoices (
			id, project_id, payable_id, invoice_number, invoice_date, vendor_name,
			amount_before_vat, vat_amount, total_amount, status,
			storage_provider, storage_key, mime_type, file_name, file_size, checksum_sha256,
			uploaded_by, verified_by, verified_at
		)
		VALUES ($1,$2,$3,'INV-LED-001','2026-06-01','Ledger Vendor',
			300,0,300,'verified',
			'local','input-invoices/test.pdf','application/pdf','test.pdf',128,$4,
			$5,$5,now())
	`, invoiceID, fixture.projectID, payableID, strings.Repeat("a", 64), fixture.userID)

	schedule, err := service.CreatePayablePaymentSchedule(ctx, payableID, PayablePaymentScheduleRequest{
		ScheduleCode:  "SCH-LED-001",
		MilestoneName: "Kỳ ledger",
		ScheduleType:  "progress",
		PlannedAmount: 500,
		DueDate:       "2000-02-01",
	}, fixture.userID, "Scoped User")
	if err != nil {
		t.Fatal(err)
	}
	scheduleID := cleanID(schedule["id"])
	if _, err := service.PayablePaymentScheduleAction(ctx, scheduleID, "approve", ActionRequest{}, fixture.userID, "Scoped User"); err != nil {
		t.Fatal(err)
	}

	advance, err := service.CreatePayableAdvanceRequest(ctx, payableID, AdvanceRequestCreateRequest{
		RequestCode: "ADV-LED-001",
		Amount:      200,
		DueDate:     "2000-01-15",
		Reason:      "Ứng trước ledger",
	}, fixture.userID, "Scoped User")
	if err != nil {
		t.Fatal(err)
	}
	advanceID := cleanID(advance["id"])
	if _, err := service.PayableAdvanceRequestAction(ctx, advanceID, "approve", ActionRequest{}, fixture.userID, "Scoped User"); err != nil {
		t.Fatal(err)
	}

	payment, err := service.CreatePayablePayment(ctx, payableID, PayablePaymentCreateRequest{
		PaymentCode:    "PMT-LED-001",
		PaymentType:    "supplier_payment",
		PaymentMethod:  "bank_transfer",
		Amount:         100,
		ScheduleID:     scheduleID,
		InputInvoiceID: invoiceID,
		PaidAt:         "2026-06-20",
	}, fixture.userID, "Scoped User")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.PaymentAction(ctx, cleanID(payment["id"]), "approve", ActionRequest{}, fixture.userID, "Scoped User"); err != nil {
		t.Fatal(err)
	}

	snapshot, err := service.PayableReconciliation(ctx, payableID, fixture.userID)
	if err != nil {
		t.Fatal(err)
	}
	summary, _ := snapshot["summary"].(Record)
	warnings, _ := snapshot["warnings"].([]Record)
	ledger, _ := snapshot["ledger"].([]Record)
	if summary["aging_bucket"] != "90_plus" {
		t.Fatalf("aging_bucket = %v, want 90_plus", summary["aging_bucket"])
	}
	if summary["reconciliation_status"] != "needs_review" {
		t.Fatalf("reconciliation_status = %v, want needs_review", summary["reconciliation_status"])
	}
	if !hasWarningCode(warnings, "overdue") || !hasWarningCode(warnings, "advance_unpaid") {
		t.Fatalf("warnings = %#v, want overdue and advance_unpaid", warnings)
	}
	if len(ledger) < 4 {
		t.Fatalf("ledger rows = %d, want at least 4", len(ledger))
	}
	if number(summary["remaining_amount"]) != 700 {
		t.Fatalf("remaining_amount = %v, want 700", summary["remaining_amount"])
	}
}

func TestOutputBillingScheduleAdvanceAndPartialReceiptReconcileReceivable(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")
	fixture := setupProjectScopeFixture(t, db, []string{
		"accounting.view",
		"accounting.receivable_view",
		"accounting.receivable_approve",
		"accounting.payment_create",
		"accounting.payment_approve",
	})
	receivableID := newID()
	mustExec(t, db, `
		INSERT INTO receivables (id, project_id, receivable_code, client_name, source_type, gross_amount, received_amount, remaining_amount, status, created_by)
		VALUES ($1,$2,$3,'Output Billing Client','manual',1000,0,1000,'approved',$4)
	`, receivableID, fixture.projectID, "REC-OB-"+receivableID[:8], fixture.userID)

	schedule, err := service.CreateReceivablePaymentSchedule(ctx, receivableID, PayablePaymentScheduleRequest{
		ScheduleCode:  "SCH-OB-001",
		MilestoneName: "Kỳ thu 1",
		ScheduleType:  "progress",
		PlannedAmount: 600,
		DueDate:       "2026-08-15",
	}, fixture.userID, "Scoped User")
	if err != nil {
		t.Fatal(err)
	}
	scheduleID := cleanID(schedule["id"])
	schedule, err = service.ReceivablePaymentScheduleAction(ctx, scheduleID, "approve", ActionRequest{Comment: "Duyệt kỳ"}, fixture.userID, "Scoped User")
	if err != nil {
		t.Fatal(err)
	}
	if schedule["status"] != "approved" || number(schedule["remaining_amount"]) != 600 {
		t.Fatalf("schedule = status %v remaining %v, want approved/600", schedule["status"], schedule["remaining_amount"])
	}

	advance, err := service.CreateReceivableAdvanceReceipt(ctx, receivableID, ReceivableAdvanceReceiptCreateRequest{
		ReceiptCode: "ADV-OB-001",
		Amount:      200,
		DueDate:     "2026-08-01",
		Reason:      "Khách hàng ứng trước",
	}, fixture.userID, "Scoped User")
	if err != nil {
		t.Fatal(err)
	}
	advanceID := cleanID(advance["id"])
	advance, err = service.ReceivableAdvanceReceiptAction(ctx, advanceID, "approve", ActionRequest{Comment: "Duyệt ứng"}, fixture.userID, "Scoped User")
	if err != nil {
		t.Fatal(err)
	}
	if advance["status"] != "approved" {
		t.Fatalf("advance status = %v, want approved", advance["status"])
	}
	receivable, err := repo.GetReceivableDetail(ctx, receivableID)
	if err != nil {
		t.Fatal(err)
	}
	if receivable["status"] != "partially_received" || number(receivable["received_amount"]) != 200 || number(receivable["remaining_amount"]) != 800 {
		t.Fatalf("receivable after advance = status %v received %v remaining %v, want partially_received/200/800", receivable["status"], receivable["received_amount"], receivable["remaining_amount"])
	}

	advance, err = service.SettleReceivableAdvanceReceipt(ctx, advanceID, ReceivableAdvanceSettlementRequest{Amount: 50, ScheduleID: scheduleID, Comment: "Đối soát một phần"}, fixture.userID, "Scoped User")
	if err != nil {
		t.Fatal(err)
	}
	if advance["status"] != "partially_settled" || number(advance["settled_amount"]) != 50 {
		t.Fatalf("advance settlement = status %v settled %v, want partially_settled/50", advance["status"], advance["settled_amount"])
	}

	receipt, err := service.CreateReceivablePayment(ctx, receivableID, ReceivableReceiptCreateRequest{
		PaymentCode:   "RCV-OB-001",
		PaymentType:   "client_receipt",
		PaymentMethod: "bank_transfer",
		Amount:        300,
		ScheduleID:    scheduleID,
		PaidAt:        "2026-08-20",
		Notes:         "Thu một phần kỳ 1",
	}, fixture.userID, "Scoped User")
	if err != nil {
		t.Fatal(err)
	}
	if receipt["status"] != "draft" || receipt["allocation_type"] != "receivable_receipt" {
		t.Fatalf("receipt = %#v, want draft receivable allocation", receipt)
	}
	receipt, err = service.PaymentAction(ctx, cleanID(receipt["id"]), "approve", ActionRequest{Comment: "Duyệt phiếu thu"}, fixture.userID, "Scoped User")
	if err != nil {
		t.Fatal(err)
	}
	if receipt["status"] != "approved" {
		t.Fatalf("receipt status = %v, want approved", receipt["status"])
	}
	receivable, err = repo.GetReceivableDetail(ctx, receivableID)
	if err != nil {
		t.Fatal(err)
	}
	if receivable["status"] != "partially_received" || number(receivable["received_amount"]) != 500 || number(receivable["remaining_amount"]) != 500 {
		t.Fatalf("receivable after receipt = status %v received %v remaining %v, want partially_received/500/500", receivable["status"], receivable["received_amount"], receivable["remaining_amount"])
	}
	schedule, err = repo.GetReceivablePaymentScheduleDetail(ctx, scheduleID)
	if err != nil {
		t.Fatal(err)
	}
	if schedule["status"] != "partially_received" || number(schedule["received_amount"]) != 300 || number(schedule["remaining_amount"]) != 300 {
		t.Fatalf("schedule after receipt = status %v received %v remaining %v, want partially_received/300/300", schedule["status"], schedule["received_amount"], schedule["remaining_amount"])
	}

	advanceReceipt, err := service.CreateReceivablePayment(ctx, receivableID, ReceivableReceiptCreateRequest{
		PaymentCode:      "RCV-OB-ADV",
		PaymentType:      "advance",
		PaymentMethod:    "cash",
		Amount:           200,
		AdvanceReceiptID: advanceID,
		PaidAt:           "2026-08-02",
	}, fixture.userID, "Scoped User")
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.PaymentAction(ctx, cleanID(advanceReceipt["id"]), "approve", ActionRequest{Comment: "Thu tiền ứng trước"}, fixture.userID, "Scoped User")
	if err != nil {
		t.Fatal(err)
	}
	advance, err = repo.GetReceivableAdvanceReceiptDetail(ctx, advanceID)
	if err != nil {
		t.Fatal(err)
	}
	if number(advance["received_amount"]) != 200 {
		t.Fatalf("advance received_amount = %v, want 200", advance["received_amount"])
	}
	receivable, err = repo.GetReceivableDetail(ctx, receivableID)
	if err != nil {
		t.Fatal(err)
	}
	if number(receivable["remaining_amount"]) != 500 {
		t.Fatalf("advance receipt payment should not reduce receivable twice, remaining = %v, want 500", receivable["remaining_amount"])
	}

	_, err = service.CreateReceivablePayment(ctx, receivableID, ReceivableReceiptCreateRequest{
		PaymentCode:   "RCV-OB-OVER",
		PaymentMethod: "bank_transfer",
		Amount:        1000,
	}, fixture.userID, "Scoped User")
	assertValidationError(t, err)
	assertCount(t, db, `
		SELECT COUNT(*)
		FROM audit_logs
		WHERE project_id=$1
		  AND event_type IN (
		    'receivable_payment_schedule_created',
		    'receivable_payment_schedule_approved',
		    'receivable_advance_receipt_created',
		    'receivable_advance_receipt_approved',
		    'receivable_advance_receipt_settled',
		    'receivable_payment_created',
		    'receivable_payment_allocated',
		    'receivable_advance_payment_recorded'
		  )
	`, 9, fixture.projectID)
}

func TestReceivableReconciliationSnapshotIncludesAgingWarningsAndLedger(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")
	fixture := setupProjectScopeFixture(t, db, []string{
		"accounting.view",
		"accounting.receivable_view",
		"accounting.receivable_approve",
		"accounting.payment_create",
		"accounting.payment_approve",
	})
	receivableID := newID()
	invoiceID := newID()
	mustExec(t, db, `
		INSERT INTO receivables (id, project_id, receivable_code, client_name, source_type, gross_amount, received_amount, remaining_amount, due_date, status, created_by)
		VALUES ($1,$2,$3,'Ledger Client','manual',1000,0,1000,'2000-01-01','approved',$4)
	`, receivableID, fixture.projectID, "REC-LED-"+receivableID[:8], fixture.userID)
	mustExec(t, db, `
		INSERT INTO output_invoices (
			id, project_id, receivable_id, invoice_number, invoice_date, client_name,
			amount_before_vat, vat_amount, total_amount, status,
			storage_provider, storage_key, mime_type, file_name, file_size, checksum_sha256,
			uploaded_by, issued_by, issued_at
		)
		VALUES ($1,$2,$3,'OUT-LED-001','2026-06-01','Ledger Client',
			300,0,300,'issued',
			'local','output-invoices/test.pdf','application/pdf','test.pdf',128,$4,
			$5,$5,now())
	`, invoiceID, fixture.projectID, receivableID, strings.Repeat("b", 64), fixture.userID)

	schedule, err := service.CreateReceivablePaymentSchedule(ctx, receivableID, PayablePaymentScheduleRequest{
		ScheduleCode:  "SCH-REC-LED-001",
		MilestoneName: "Kỳ thu ledger",
		ScheduleType:  "progress",
		PlannedAmount: 500,
		DueDate:       "2000-02-01",
	}, fixture.userID, "Scoped User")
	if err != nil {
		t.Fatal(err)
	}
	scheduleID := cleanID(schedule["id"])
	if _, err := service.ReceivablePaymentScheduleAction(ctx, scheduleID, "approve", ActionRequest{}, fixture.userID, "Scoped User"); err != nil {
		t.Fatal(err)
	}

	advance, err := service.CreateReceivableAdvanceReceipt(ctx, receivableID, ReceivableAdvanceReceiptCreateRequest{
		ReceiptCode: "ADV-REC-LED-001",
		Amount:      200,
		DueDate:     "2000-01-15",
		Reason:      "Ứng trước ledger",
	}, fixture.userID, "Scoped User")
	if err != nil {
		t.Fatal(err)
	}
	advanceID := cleanID(advance["id"])
	if _, err := service.ReceivableAdvanceReceiptAction(ctx, advanceID, "approve", ActionRequest{}, fixture.userID, "Scoped User"); err != nil {
		t.Fatal(err)
	}

	receipt, err := service.CreateReceivablePayment(ctx, receivableID, ReceivableReceiptCreateRequest{
		PaymentCode:     "RCV-LED-001",
		PaymentType:     "client_receipt",
		PaymentMethod:   "bank_transfer",
		Amount:          100,
		ScheduleID:      scheduleID,
		OutputInvoiceID: invoiceID,
		PaidAt:          "2026-06-20",
	}, fixture.userID, "Scoped User")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.PaymentAction(ctx, cleanID(receipt["id"]), "approve", ActionRequest{}, fixture.userID, "Scoped User"); err != nil {
		t.Fatal(err)
	}

	snapshot, err := service.ReceivableReconciliation(ctx, receivableID, fixture.userID)
	if err != nil {
		t.Fatal(err)
	}
	summary, _ := snapshot["summary"].(Record)
	warnings, _ := snapshot["warnings"].([]Record)
	ledger, _ := snapshot["ledger"].([]Record)
	if summary["aging_bucket"] != "90_plus" {
		t.Fatalf("aging_bucket = %v, want 90_plus", summary["aging_bucket"])
	}
	if summary["reconciliation_status"] != "needs_review" {
		t.Fatalf("reconciliation_status = %v, want needs_review", summary["reconciliation_status"])
	}
	if !hasWarningCode(warnings, "overdue") || !hasWarningCode(warnings, "advance_unreceived") {
		t.Fatalf("warnings = %#v, want overdue and advance_unreceived", warnings)
	}
	if len(ledger) < 4 {
		t.Fatalf("ledger rows = %d, want at least 4", len(ledger))
	}
	if number(summary["remaining_amount"]) != 700 {
		t.Fatalf("remaining_amount = %v, want 700", summary["remaining_amount"])
	}
}
