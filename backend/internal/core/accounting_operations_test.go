package core

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/gofiber/fiber/v2"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestAccountingOperationsMoneyAndCSV(t *testing.T) {
	for _, v := range []struct {
		value, currency string
		valid           bool
	}{{"100", "VND", true}, {"1.00", "VND", false}, {"1.01", "USD", true}, {"1.001", "USD", false}, {"1.001", "KWD", true}, {"NaN", "VND", false}, {"-1", "USD", false}, {"1e4", "USD", false}, {"1", "ZZZ", false}} {
		err := accountingMoney(v.value, v.currency, false)
		if (err == nil) != v.valid {
			t.Fatalf("%+v: %v", v, err)
		}
	}
	line := "PC-1,1,transaction,outbound," + newID() + ",VND,100,2026-09-17,\n"
	rows, err := parseAccountingMISA(accountingMISAHeader + "\n" + line)
	if err != nil || len(rows) != 1 {
		t.Fatal(rows, err)
	}
	if _, err = parseAccountingMISA(accountingMISAHeader + "\n" + line + line); err == nil {
		t.Fatal("duplicate rows accepted")
	}
}

type accountingOperationsFixture struct {
	db                               *sql.DB
	s                                *Service
	f                                projectScopeFixture
	basis, partner, source, evidence string
}

func setupAccountingOperations(t *testing.T) accountingOperationsFixture {
	t.Helper()
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	f := setupProjectScopeFixture(t, db, []string{"accounting_request.view", "accounting_request.submit", "accounting_request.review", "accounting_misa.reconcile", "procurement.view", "warehouse.view"})
	s := NewService(NewRepository(db), "test")
	partner, source, evidence, work := newID(), newID(), newID(), newID()
	mustExec(t, db, `INSERT INTO partners(id,partner_code,name) VALUES($1,$2,'Đối tác kiểm thử')`, partner, "AO-"+partner)
	mustExec(t, db, `INSERT INTO procurement_requests(id,project_id,request_code,title,status,supplier_partner_id,total_estimated_amount) VALUES($1,$2,$3,'Nguồn mua','ordered',$4,1)`, source, f.projectID, "AO-"+source, partner)
	mustExec(t, db, `INSERT INTO work_items(id,project_id,source_type,source_id,title,assignee_staff_id,status) VALUES($1,$2,'acceptance',$3,'Chứng từ căn cứ',$4,'ready')`, work, f.projectID, source, f.staffID)
	mustExec(t, db, `INSERT INTO evidence_records(id,work_item_id,project_id,evidence_type,name,status) VALUES($1,$2,$3,'document','Căn cứ đã ký','submitted')`, evidence, work, f.projectID)
	b, err := s.CreateAccountingBasis(ctx, f.projectID, AccountingBasisInput{SourceType: "procurement_request", SourceID: source, Amount: "100", Note: "Giá trị theo chứng từ, không lấy giá dự kiến", EvidenceIDs: []string{evidence}}, f.userID, "Tester")
	if err != nil {
		t.Fatal(err)
	}
	return accountingOperationsFixture{db, s, f, cleanID(b["id"]), partner, source, evidence}
}
func (x accountingOperationsFixture) draft(t *testing.T, kind, gross, recovery, retention, advance, release string) AccountingRequestInput {
	t.Helper()
	in := AccountingRequestInput{ID: newID(), BasisID: x.basis, Title: "Đợt kiểm thử", Kind: kind, DueDate: "2026-09-30", Gross: gross, Recovery: recovery, Retention: retention, AdvanceID: advance, ReleaseID: release, EvidenceIDs: []string{x.evidence}, StockIDs: []string{}}
	_, err := x.s.SaveAccountingRequest(context.Background(), x.f.projectID, in, x.f.userID, "Tester")
	if err != nil {
		t.Fatal(err)
	}
	return in
}
func (x accountingOperationsFixture) action(t *testing.T, id, action string, revision int) {
	t.Helper()
	_, err := x.s.AccountingRequestAction(context.Background(), x.f.projectID, id, action, AccountingRequestActionInput{ExpectedRevision: revision, Reason: "Test reason"}, x.f.userID, "Tester")
	if err != nil {
		t.Fatal(err)
	}
}
func (x accountingOperationsFixture) importRow(t *testing.T, external, amount, replace string) string {
	t.Helper()
	ctx := context.Background()
	csv := accountingMISAHeader + "\n" + fmt.Sprintf("%s,1,transaction,outbound,%s,VND,%s,2026-09-17,%s\n", external, x.partner, amount, replace)
	b, err := x.s.PreviewAccountingMISA(ctx, x.f.projectID, AccountingMISAImportInput{Entity: "TEST", Filename: "misa.csv", Content: csv}, x.f.userID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = x.s.ConfirmAccountingMISA(ctx, x.f.projectID, cleanID(b["id"]), x.f.userID, "Tester")
	if err != nil {
		t.Fatal(err)
	}
	var id string
	if err = x.db.QueryRow(`SELECT id FROM accounting_misa_records WHERE batch_id=$1`, b["id"]).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func TestAccountingOperationsRequestLifecycleAndCaps(t *testing.T) {
	x := setupAccountingOperations(t)
	ctx := context.Background()
	advance := x.draft(t, "advance", "30", "0", "0", "", "")
	x.action(t, advance.ID, "submit", 1)
	x.action(t, advance.ID, "acknowledge", 2)
	progress := x.draft(t, "progress", "100", "30", "10", advance.ID, "")
	x.action(t, progress.ID, "submit", 1)
	x.action(t, progress.ID, "acknowledge", 2)
	release := x.draft(t, "retention_release", "10", "0", "0", "", progress.ID)
	x.action(t, release.ID, "submit", 1)
	x.action(t, release.ID, "acknowledge", 2)
	over := x.draft(t, "retention_release", "1", "0", "0", "", progress.ID)
	if _, err := x.s.AccountingRequestAction(ctx, x.f.projectID, over.ID, "submit", AccountingRequestActionInput{ExpectedRevision: 1}, x.f.userID, "Tester"); err == nil {
		t.Fatal("over-release accepted")
	}
	over = x.draft(t, "progress", "1", "0", "0", "", "")
	if _, err := x.s.AccountingRequestAction(ctx, x.f.projectID, over.ID, "submit", AccountingRequestActionInput{ExpectedRevision: 1}, x.f.userID, "Tester"); err == nil {
		t.Fatal("over-basis accepted")
	}
	advance.ExpectedRevision = 3
	if _, err := x.s.SaveAccountingRequest(ctx, x.f.projectID, advance, x.f.userID, "Tester"); err == nil {
		t.Fatal("acknowledged request edited")
	}
	if _, err := x.db.Exec(`UPDATE accounting_request_events SET action='fake' WHERE request_id=$1`, advance.ID); err == nil {
		t.Fatal("event mutated")
	}
	if _, err := x.s.AccountingOperationsWorkspace(ctx, x.f.otherProjectID, x.f.userID); err == nil {
		t.Fatal("cross-project read allowed")
	}
	data, err := x.s.AccountingOperationsWorkspace(ctx, x.f.projectID, x.f.userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(data["requests"].([]Record)) != 5 {
		t.Fatal(data)
	}
	assertCount(t, x.db, `SELECT COUNT(*) FROM payables WHERE project_id=$1`, 0, x.f.projectID)
	assertCount(t, x.db, `SELECT COUNT(*) FROM payments WHERE project_id=$1`, 0, x.f.projectID)
	assertCount(t, x.db, `SELECT COUNT(*) FROM stock_movements WHERE project_id=$1`, 0, x.f.projectID)
}

func TestAccountingOperationsMISAAllocationRaceAndReplacement(t *testing.T) {
	x := setupAccountingOperations(t)
	ctx := context.Background()
	req := x.draft(t, "progress", "100", "0", "0", "", "")
	x.action(t, req.ID, "submit", 1)
	x.action(t, req.ID, "acknowledge", 2)
	record := x.importRow(t, "PC-1", "100", "")
	// Same import is idempotent and retains the exact observation ID.
	if second := x.importRow(t, "PC-1", "100", ""); second != record {
		t.Fatal("duplicate import")
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := x.s.AllocateAccountingMISA(ctx, x.f.projectID, AccountingMISAAllocationInput{ID: newID(), RequestID: req.ID, RecordID: record, Amount: "60"}, x.f.userID, "Tester")
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	successes := 0
	for err := range results {
		if err == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("allocation successes=%d", successes)
	}
	data, err := x.s.AccountingOperationsWorkspace(ctx, x.f.projectID, x.f.userID)
	if err != nil {
		t.Fatal(err)
	}
	r := data["requests"].([]Record)[0]
	_, err = x.s.ReviewAccountingMISA(ctx, x.f.projectID, AccountingMISAAllocationInput{RequestID: req.ID, Note: "Đã kiểm tra, còn 40", Fingerprint: fmt.Sprint(r["fingerprint"])}, x.f.userID, "Tester")
	if err != nil {
		t.Fatal(err)
	}
	alloc := data["allocations"].([]Record)[0]
	content := accountingMISAHeader + "\n" + fmt.Sprintf("PC-1,1,transaction,outbound,%s,VND,90,2026-09-17,%s\n", x.partner, record)
	b, err := x.s.PreviewAccountingMISA(ctx, x.f.projectID, AccountingMISAImportInput{Entity: "TEST", Filename: "correction.csv", Content: content}, x.f.userID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = x.s.ConfirmAccountingMISA(ctx, x.f.projectID, cleanID(b["id"]), x.f.userID, "Tester"); err == nil {
		t.Fatal("replaced allocated record")
	}
	if _, err = x.s.RevokeAccountingAllocation(ctx, x.f.projectID, cleanID(alloc["id"]), "MISA điều chỉnh", x.f.userID, "Tester"); err != nil {
		t.Fatal(err)
	}
	data, err = x.s.AccountingOperationsWorkspace(ctx, x.f.projectID, x.f.userID)
	if err != nil {
		t.Fatal(err)
	}
	if boolValue(data["requests"].([]Record)[0]["reviewed"]) {
		t.Fatal("stale reconciliation remains accepted")
	}
	if _, err = x.s.ConfirmAccountingMISA(ctx, x.f.projectID, cleanID(b["id"]), x.f.userID, "Tester"); err != nil {
		t.Fatal(err)
	}
	assertCount(t, x.db, `SELECT COUNT(*) FROM accounting_misa_records WHERE project_id=$1 AND active`, 1, x.f.projectID)
	assertCount(t, x.db, `SELECT COUNT(*) FROM accounting_misa_records WHERE project_id=$1`, 2, x.f.projectID)
	if _, err = x.db.Exec(`UPDATE accounting_misa_batches SET file_content='tampered' WHERE id=$1`, b["id"]); err == nil {
		t.Fatal("import file mutated")
	}
}

func TestAccountingOperationsReturnAndPermissions(t *testing.T) {
	x := setupAccountingOperations(t)
	ctx := context.Background()
	req := x.draft(t, "progress", "40", "0", "0", "", "")
	x.action(t, req.ID, "submit", 1)
	x.action(t, req.ID, "return", 2)
	req.ExpectedRevision = 3
	req.Gross = "50"
	if _, err := x.s.SaveAccountingRequest(ctx, x.f.projectID, req, x.f.userID, "Tester"); err != nil {
		t.Fatal(err)
	}
	if _, err := x.s.SaveAccountingRequest(ctx, x.f.projectID, req, x.f.userID, "Tester"); err == nil {
		t.Fatal("stale save allowed")
	}
	x.action(t, req.ID, "submit", 4)
	q, err := x.s.AccountingRequestQueue(ctx, x.f.userID)
	if err != nil || len(q) != 1 {
		t.Fatal(q, err)
	}
	mustExec(t, x.db, `DELETE FROM role_permissions WHERE role_id=$1 AND permission_id IN(SELECT id FROM permissions WHERE key IN ('accounting_request.review','accounting_misa.reconcile'))`, x.f.roleID)
	if _, err := x.s.AccountingRequestAction(ctx, x.f.projectID, req.ID, "acknowledge", AccountingRequestActionInput{ExpectedRevision: 5}, x.f.userID, "Tester"); err == nil {
		t.Fatal("unauthorized acknowledge")
	}
	if _, err := x.s.PreviewAccountingMISA(ctx, x.f.projectID, AccountingMISAImportInput{}, x.f.userID); err == nil {
		t.Fatal("unauthorized import")
	}
}

func TestAccountingOperationsManyToManyAndBalanceSeparation(t *testing.T) {
	x := setupAccountingOperations(t)
	ctx := context.Background()
	first := x.draft(t, "progress", "40", "0", "0", "", "")
	second := x.draft(t, "progress", "60", "0", "0", "", "")
	for _, r := range []AccountingRequestInput{first, second} {
		x.action(t, r.ID, "submit", 1)
		x.action(t, r.ID, "acknowledge", 2)
	}
	one := x.importRow(t, "PC-A", "70", "")
	two := x.importRow(t, "PC-B", "30", "")
	in := AccountingMISAAllocationInput{ID: newID(), RequestID: first.ID, RecordID: one, Amount: "40"}
	for i := 0; i < 2; i++ {
		if _, err := x.s.AllocateAccountingMISA(ctx, x.f.projectID, in, x.f.userID, "Tester"); err != nil {
			t.Fatal(err)
		}
	}
	assertCount(t, x.db, `SELECT COUNT(*) FROM accounting_misa_allocations WHERE request_id=$1`, 1, first.ID)
	for _, id := range []string{one, two} {
		if _, err := x.s.AllocateAccountingMISA(ctx, x.f.projectID, AccountingMISAAllocationInput{ID: newID(), RequestID: second.ID, RecordID: id, Amount: "30"}, x.f.userID, "Tester"); err != nil {
			t.Fatal(err)
		}
	}
	content := accountingMISAHeader + "\n" + fmt.Sprintf("BAL-1,1,balance,outbound,%s,VND,100,2026-09-30,\n", x.partner)
	b, err := x.s.PreviewAccountingMISA(ctx, x.f.projectID, AccountingMISAImportInput{Entity: "TEST", Filename: "balance.csv", Content: content}, x.f.userID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = x.s.ConfirmAccountingMISA(ctx, x.f.projectID, cleanID(b["id"]), x.f.userID, "Tester"); err != nil {
		t.Fatal(err)
	}
	var balance string
	if err = x.db.QueryRow(`SELECT id FROM accounting_misa_records WHERE batch_id=$1`, b["id"]).Scan(&balance); err != nil {
		t.Fatal(err)
	}
	if _, err = x.s.AllocateAccountingMISA(ctx, x.f.projectID, AccountingMISAAllocationInput{ID: newID(), RequestID: first.ID, RecordID: balance, Amount: "1"}, x.f.userID, "Tester"); err == nil {
		t.Fatal("balance used as cash")
	}
	data, err := x.s.AccountingOperationsWorkspace(ctx, x.f.projectID, x.f.userID)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range data["requests"].([]Record) {
		if number(r["uncovered_amount"]) != 0 {
			t.Fatal(r)
		}
	}
	assertCount(t, x.db, `SELECT COUNT(*) FROM payments WHERE project_id=$1`, 0, x.f.projectID)
	assertCount(t, x.db, `SELECT COUNT(*) FROM journal_entries WHERE project_id=$1`, 0, x.f.projectID)
}

func TestAccountingOperationsContractSourcesAndStockLinks(t *testing.T) {
	x := setupAccountingOperations(t)
	ctx := context.Background()
	permission := ensurePermission(t, x.db, "contract.view", "View contract", "contract")
	mustExec(t, x.db, `INSERT INTO role_permissions(role_id,permission_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, x.f.roleID, permission)
	for _, c := range []struct{ kind, direction, want string }{{"main_client", "receivable", "inbound"}, {"subcontractor", "payable", "outbound"}} {
		contract, version, milestone := newID(), newID(), newID()
		mustExec(t, x.db, `INSERT INTO contracts(id,project_id,contract_code,title,contract_type,direction,counterparty_partner_id,counterparty_name,currency,status) VALUES($1,$2,$3,'Hợp đồng',$4,$5,$6,'Đối tác','USD','active')`, contract, x.f.projectID, "HD-"+contract, c.kind, c.direction, x.partner)
		mustExec(t, x.db, `INSERT INTO contract_versions(id,contract_id,version_no,version_label,amount,scope_summary,definition_hash) VALUES($1,$2,1,'V1',100,'Scope','hash')`, version, contract)
		mustExec(t, x.db, `INSERT INTO contract_milestones(id,contract_id,contract_version_id,milestone_code,milestone_name,sequence_no,milestone_type,gross_amount,net_amount,status) VALUES($1,$2,$3,'M1','Đợt 1',1,'progress',100,100,'active')`, milestone, contract, version)
		in := AccountingBasisInput{SourceType: "contract_milestone", SourceID: milestone, Amount: "100.01", Note: "Theo hợp đồng", EvidenceIDs: []string{x.evidence}}
		if _, err := x.s.CreateAccountingBasis(ctx, x.f.projectID, in, x.f.userID, "Tester"); err == nil {
			t.Fatal("basis above milestone accepted")
		}
		in.Amount = "100.00"
		b, err := x.s.CreateAccountingBasis(ctx, x.f.projectID, in, x.f.userID, "Tester")
		if err != nil {
			t.Fatal(err)
		}
		var direction, currency string
		if err = x.db.QueryRow(`SELECT direction,currency FROM accounting_request_bases WHERE id=$1`, b["id"]).Scan(&direction, &currency); err != nil || direction != c.want || currency != "USD" {
			t.Fatal(direction, currency, err)
		}
	}
	req := x.draft(t, "progress", "20", "0", "0", "", "")
	stock, other := newID(), newID()
	mustExec(t, x.db, `INSERT INTO stock_documents(id,project_id,document_code,movement_type,status) VALUES($1,$2,$3,'issue','posted'),($4,$5,$6,'issue','posted')`, stock, x.f.projectID, "PX-"+stock, other, x.f.otherProjectID, "PX-"+other)
	req.ExpectedRevision = 1
	req.StockIDs = []string{other}
	if _, err := x.s.SaveAccountingRequest(ctx, x.f.projectID, req, x.f.userID, "Tester"); err == nil {
		t.Fatal("cross-project stock link")
	}
	req.StockIDs = []string{stock}
	if _, err := x.s.SaveAccountingRequest(ctx, x.f.projectID, req, x.f.userID, "Tester"); err != nil {
		t.Fatal(err)
	}
	assertCount(t, x.db, `SELECT COUNT(*) FROM stock_movements WHERE project_id=$1`, 0, x.f.projectID)
}

func TestAccountingOperationsObserverAndStaleEvidence(t *testing.T) {
	x := setupAccountingOperations(t)
	ctx := context.Background()
	req := x.draft(t, "progress", "20", "0", "0", "", "")
	mustExec(t, x.db, `UPDATE evidence_records SET status='rejected' WHERE id=$1`, x.evidence)
	if _, err := x.s.AccountingRequestAction(ctx, x.f.projectID, req.ID, "submit", AccountingRequestActionInput{ExpectedRevision: 1}, x.f.userID, "Tester"); err == nil {
		t.Fatal("submitted rejected evidence")
	}
	mustExec(t, x.db, `UPDATE evidence_records SET status='submitted' WHERE id=$1`, x.evidence)
	x.action(t, req.ID, "submit", 1)
	marker := ensurePermission(t, x.db, "project.reconciliation.view", "Observer", "project")
	mustExec(t, x.db, `INSERT INTO role_permissions(role_id,permission_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, x.f.roleID, marker)
	mustExec(t, x.db, `DELETE FROM project_members WHERE project_id=$1 AND staff_id=$2`, x.f.projectID, x.f.staffID)
	if _, err := x.s.AccountingOperationsWorkspace(ctx, x.f.projectID, x.f.userID); err != nil {
		t.Fatal(err)
	}
	x.action(t, req.ID, "acknowledge", 2)
	record := x.importRow(t, "GLOBAL-PC", "20", "")
	if _, err := x.s.AllocateAccountingMISA(ctx, x.f.projectID, AccountingMISAAllocationInput{ID: newID(), RequestID: req.ID, RecordID: record, Amount: "20"}, x.f.userID, "Tester"); err != nil {
		t.Fatal(err)
	}
	// The observation marker still does not grant unrelated project mutations.
	allowed, err := x.s.repo.CanAccessProjectPermission(ctx, x.f.userID, x.f.projectID, []string{"warehouse.issue"})
	if err != nil || allowed {
		t.Fatal(allowed, err)
	}
}

func TestAccountingOperationsHTTPContractsAndProtectedOriginal(t *testing.T) {
	x := setupAccountingOperations(t)
	ctx := context.Background()
	app := batch4TestApp(x.f.userID, func(app *fiber.App, h *Handler) { registerFinanceRoutes(app, h) }, NewHandler(x.s))
	req := x.draft(t, "progress", "20", "0", "0", "", "")
	// Optional reference lists omitted in JSON remain arrays, never JSON null.
	req.ExpectedRevision = 1
	req.EvidenceIDs = nil
	req.StockIDs = nil
	if _, err := x.s.SaveAccountingRequest(ctx, x.f.projectID, req, x.f.userID, "Tester"); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/projects/"+x.f.projectID+"/accounting-requests/"+req.ID+"/submit", strings.NewReader(`{"expected_revision":2}`))
	request.Header.Set("Content-Type", "application/json")
	res, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("%d %s", res.StatusCode, body)
	}
	content := accountingMISAHeader + "\n" + fmt.Sprintf("PC-HTTP,1,transaction,outbound,%s,VND,20,2026-09-17,\n", x.partner)
	batch, err := x.s.PreviewAccountingMISA(ctx, x.f.projectID, AccountingMISAImportInput{Entity: "TEST", Filename: "original.csv", Content: content}, x.f.userID)
	if err != nil {
		t.Fatal(err)
	}
	res, err = app.Test(httptest.NewRequest(http.MethodGet, "/projects/"+x.f.projectID+"/misa-imports/"+cleanID(batch["id"])+"/file", nil))
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != 200 || string(body) != content || res.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal(res.StatusCode, string(body))
	}
	res, err = app.Test(httptest.NewRequest(http.MethodGet, "/projects/"+x.f.otherProjectID+"/misa-imports/"+cleanID(batch["id"])+"/file", nil))
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 403 {
		t.Fatal("cross-project original file", res.StatusCode)
	}
}
