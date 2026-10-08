package core

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/gofiber/fiber/v2"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
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
	db := openIsolatedAccountingTestDB(t)
	ctx := context.Background()
	f := setupProjectScopeFixture(t, db, []string{"accounting_reconciliation.view", "accounting_reconciliation.manage", "contract.view", "accounting_misa.reconcile", "procurement.view", "warehouse.view"})
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

func (x accountingOperationsFixture) basisRow(t *testing.T) Record {
	t.Helper()
	data, err := x.s.AccountingOperationsWorkspace(context.Background(), x.f.projectID, x.f.userID)
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range data["bases"].([]Record) {
		if cleanID(b["id"]) == x.basis {
			return b
		}
	}
	t.Fatal("basis missing")
	return nil
}
func (x accountingOperationsFixture) allocate(t *testing.T, record, amount string) string {
	t.Helper()
	id := newID()
	if _, err := x.s.AllocateAccountingMISA(context.Background(), x.f.projectID, AccountingMISAAllocationInput{ID: id, BasisID: x.basis, RecordID: record, Amount: amount}, x.f.userID, "Tester"); err != nil {
		t.Fatal(err)
	}
	return id
}
func TestAccountingOperationsBasisEvidenceAndImmutableSnapshot(t *testing.T) {
	x := setupAccountingOperations(t)
	ctx := context.Background()
	// Expected evidence amount is independent of the procurement estimate (1).
	b := x.basisRow(t)
	if number(b["amount"]) != 100 || number(b["difference_amount"]) != -100 {
		t.Fatal(b)
	}
	if _, err := x.db.Exec(`UPDATE accounting_reconciliation_bases SET amount=101 WHERE id=$1`, x.basis); err == nil {
		t.Fatal("immutable basis edited")
	}
	input := AccountingBasisInput{SourceType: "procurement_request", SourceID: x.source, Amount: "100", Note: "Repeat", EvidenceIDs: []string{x.evidence}}
	if _, err := x.s.CreateAccountingBasis(ctx, x.f.projectID, input, x.f.userID, "Tester"); err == nil {
		t.Fatal("duplicate source confirmed")
	}
	input.SourceID = newID()
	mustExec(t, x.db, `INSERT INTO procurement_requests(id,project_id,request_code,title,status,supplier_partner_id) VALUES($1,$2,$3,'Unapproved','draft',$4)`, input.SourceID, x.f.projectID, "AO-"+input.SourceID, x.partner)
	if _, err := x.s.CreateAccountingBasis(ctx, x.f.projectID, input, x.f.userID, "Tester"); err == nil {
		t.Fatal("ineligible source confirmed")
	}
	mustExec(t, x.db, `UPDATE procurement_requests SET status='ordered' WHERE id=$1`, input.SourceID)
	input.EvidenceIDs = nil
	if _, err := x.s.CreateAccountingBasis(ctx, x.f.projectID, input, x.f.userID, "Tester"); err == nil {
		t.Fatal("missing evidence confirmed")
	}
	input.EvidenceIDs = []string{x.evidence}
	mustExec(t, x.db, `UPDATE evidence_records SET status='rejected' WHERE id=$1`, x.evidence)
	if _, err := x.s.CreateAccountingBasis(ctx, x.f.projectID, input, x.f.userID, "Tester"); err == nil {
		t.Fatal("rejected evidence confirmed")
	}
	assertCount(t, x.db, `SELECT COUNT(*) FROM accounting_reconciliation_bases WHERE project_id=$1`, 1, x.f.projectID)
	assertCount(t, x.db, `SELECT COUNT(*) FROM audit_logs WHERE project_id=$1 AND event_type='accounting_basis_confirmed'`, 1, x.f.projectID)
}
func TestAccountingOperationsPartialExcessAndIdempotency(t *testing.T) {
	x := setupAccountingOperations(t)
	ctx := context.Background()
	one := x.importRow(t, "PC-A", "200", "")
	in := AccountingMISAAllocationInput{ID: newID(), BasisID: x.basis, RecordID: one, Amount: "40"}
	for i := 0; i < 2; i++ {
		if _, err := x.s.AllocateAccountingMISA(ctx, x.f.projectID, in, x.f.userID, "Tester"); err != nil {
			t.Fatal(err)
		}
	}
	assertCount(t, x.db, `SELECT COUNT(*) FROM accounting_misa_allocations WHERE basis_id=$1`, 1, x.basis)
	b := x.basisRow(t)
	if number(b["allocated_amount"]) != 40 || number(b["difference_amount"]) != -60 {
		t.Fatal(b)
	}
	in.Amount = "41"
	if _, err := x.s.AllocateAccountingMISA(ctx, x.f.projectID, in, x.f.userID, "Tester"); err == nil {
		t.Fatal("id reused with changed amount")
	}
	x.allocate(t, one, "110")
	b = x.basisRow(t)
	if number(b["allocated_amount"]) != 150 || number(b["difference_amount"]) != 50 {
		t.Fatal(b)
	}
	// Excess over expected amount is visible, but record amount remains a hard cap.
	if _, err := x.s.AllocateAccountingMISA(ctx, x.f.projectID, AccountingMISAAllocationInput{ID: newID(), BasisID: x.basis, RecordID: one, Amount: "51"}, x.f.userID, "Tester"); err == nil {
		t.Fatal("record over-allocation")
	}
	assertAccountingAmount(t, ctx, x.db, `SELECT SUM(amount) FROM accounting_misa_allocations WHERE basis_id=$1 AND active`, 150, x.basis)
	assertCount(t, x.db, `SELECT COUNT(*) FROM audit_logs WHERE project_id=$1 AND event_type='accounting_misa_allocated'`, 2, x.f.projectID)
}
func TestAccountingOperationsRaceReviewInvalidationAndReplacement(t *testing.T) {
	x := setupAccountingOperations(t)
	ctx := context.Background()
	record := x.importRow(t, "PC-RACE", "100", "")
	if x.importRow(t, "PC-RACE", "100", "") != record {
		t.Fatal("import retry duplicated record")
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := x.s.AllocateAccountingMISA(ctx, x.f.projectID, AccountingMISAAllocationInput{ID: newID(), BasisID: x.basis, RecordID: record, Amount: "60"}, x.f.userID, "Tester")
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	success := 0
	for err := range results {
		if err == nil {
			success++
		}
	}
	if success != 1 {
		t.Fatalf("race successes=%d", success)
	}
	b := x.basisRow(t)
	old := fmt.Sprint(b["fingerprint"])
	if _, err := x.s.ReviewAccountingMISA(ctx, x.f.projectID, AccountingMISAReviewInput{BasisID: x.basis, Fingerprint: old, Note: "Còn thiếu 40"}, x.f.userID, "Tester"); err != nil {
		t.Fatal(err)
	}
	if !boolValue(x.basisRow(t)["reviewed"]) {
		t.Fatal("partial review not retained")
	}
	extra := x.allocate(t, record, "10")
	if boolValue(x.basisRow(t)["reviewed"]) {
		t.Fatal("allocation failed to invalidate review")
	}
	if _, err := x.s.ReviewAccountingMISA(ctx, x.f.projectID, AccountingMISAReviewInput{BasisID: x.basis, Fingerprint: old, Note: "Stale"}, x.f.userID, "Tester"); err == nil {
		t.Fatal("stale review accepted")
	}
	content := accountingMISAHeader + "\n" + fmt.Sprintf("PC-RACE,1,transaction,outbound,%s,VND,90,2026-09-17,%s\n", x.partner, record)
	batch, err := x.s.PreviewAccountingMISA(ctx, x.f.projectID, AccountingMISAImportInput{Entity: "TEST", Filename: "correction.csv", Content: content}, x.f.userID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = x.s.ConfirmAccountingMISA(ctx, x.f.projectID, cleanID(batch["id"]), x.f.userID, "Tester"); err == nil {
		t.Fatal("replaced allocated record")
	}
	assertCount(t, x.db, `SELECT COUNT(*) FROM accounting_misa_records WHERE project_id=$1`, 1, x.f.projectID)
	if _, err = x.s.RevokeAccountingAllocation(ctx, x.f.projectID, extra, "Correction", x.f.userID, "Tester"); err != nil {
		t.Fatal(err)
	}
	// Active allocation identity is immutable; revocation leaves its audit history.
	var initial string
	err = x.db.QueryRow(`SELECT id FROM accounting_misa_allocations WHERE record_id=$1 AND active`, record).Scan(&initial)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = x.db.Exec(`UPDATE accounting_misa_allocations SET amount=1 WHERE id=$1`, initial); err == nil {
		t.Fatal("allocation edited")
	}
	if _, err = x.s.RevokeAccountingAllocation(ctx, x.f.projectID, initial, "MISA điều chỉnh", x.f.userID, "Tester"); err != nil {
		t.Fatal(err)
	}
	if _, err = x.s.ConfirmAccountingMISA(ctx, x.f.projectID, cleanID(batch["id"]), x.f.userID, "Tester"); err != nil {
		t.Fatal(err)
	}
	assertCount(t, x.db, `SELECT COUNT(*) FROM accounting_misa_records WHERE project_id=$1 AND active`, 1, x.f.projectID)
	assertCount(t, x.db, `SELECT COUNT(*) FROM accounting_misa_records WHERE project_id=$1`, 2, x.f.projectID)
	if _, err = x.db.Exec(`UPDATE accounting_misa_batches SET file_content='tampered' WHERE id=$1`, batch["id"]); err == nil {
		t.Fatal("file edited")
	}
	if boolValue(x.basisRow(t)["reviewed"]) {
		t.Fatal("revocation left current review")
	}
}
func TestAccountingOperationsAllocationScopeAndStaleEvidence(t *testing.T) {
	x := setupAccountingOperations(t)
	ctx := context.Background()
	for i, c := range []struct{ kind, direction, currency, partner string }{{"balance", "outbound", "VND", x.partner}, {"transaction", "inbound", "VND", x.partner}, {"transaction", "outbound", "USD", x.partner}, {"transaction", "outbound", "VND", newID()}} {
		if c.partner != x.partner {
			mustExec(t, x.db, `INSERT INTO partners(id,partner_code,name) VALUES($1,$2,'Other')`, c.partner, "OTHER-"+c.partner)
		}
		content := accountingMISAHeader + "\n" + fmt.Sprintf("PC-SCOPE-%d,1,%s,%s,%s,%s,100,2026-09-17,\n", i, c.kind, c.direction, c.partner, c.currency)
		batch, err := x.s.PreviewAccountingMISA(ctx, x.f.projectID, AccountingMISAImportInput{Entity: "TEST", Filename: "scope.csv", Content: content}, x.f.userID)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = x.s.ConfirmAccountingMISA(ctx, x.f.projectID, cleanID(batch["id"]), x.f.userID, "Tester"); err != nil {
			t.Fatal(err)
		}
		var record string
		if err = x.db.QueryRow(`SELECT id FROM accounting_misa_records WHERE batch_id=$1`, batch["id"]).Scan(&record); err != nil {
			t.Fatal(err)
		}
		if _, err = x.s.AllocateAccountingMISA(ctx, x.f.projectID, AccountingMISAAllocationInput{ID: newID(), BasisID: x.basis, RecordID: record, Amount: "1"}, x.f.userID, "Tester"); err == nil {
			t.Fatalf("scope mismatch %d accepted", i)
		}
	}
	record := x.importRow(t, "PC-STALE", "100", "")
	mustExec(t, x.db, `UPDATE evidence_records SET status='rejected' WHERE id=$1`, x.evidence)
	if _, err := x.s.AllocateAccountingMISA(ctx, x.f.projectID, AccountingMISAAllocationInput{ID: newID(), BasisID: x.basis, RecordID: record, Amount: "1"}, x.f.userID, "Tester"); err == nil {
		t.Fatal("stale evidence allocated")
	}
	mustExec(t, x.db, `UPDATE evidence_records SET status='submitted' WHERE id=$1`, x.evidence)
	mustExec(t, x.db, `UPDATE procurement_requests SET status='cancelled' WHERE id=$1`, x.source)
	if _, err := x.s.AllocateAccountingMISA(ctx, x.f.projectID, AccountingMISAAllocationInput{ID: newID(), BasisID: x.basis, RecordID: record, Amount: "1"}, x.f.userID, "Tester"); err == nil {
		t.Fatal("cancelled source allocated")
	}
	if _, err := x.s.AccountingOperationsWorkspace(ctx, x.f.otherProjectID, x.f.userID); err == nil {
		t.Fatal("cross project read")
	}
	if _, err := x.s.AllocateAccountingMISA(ctx, x.f.otherProjectID, AccountingMISAAllocationInput{ID: newID(), BasisID: x.basis, RecordID: record, Amount: "1"}, x.f.userID, "Tester"); err == nil {
		t.Fatal("cross project write")
	}
	assertCount(t, x.db, `SELECT COUNT(*) FROM accounting_misa_allocations WHERE project_id=$1`, 0, x.f.projectID)
}
func TestAccountingOperationsContractExpectedAmountAndSourceSnapshot(t *testing.T) {
	x := setupAccountingOperations(t)
	ctx := context.Background()
	for _, c := range []struct{ kind, direction, want string }{{"main_client", "receivable", "inbound"}, {"subcontractor", "payable", "outbound"}} {
		contract, version, milestone := newID(), newID(), newID()
		mustExec(t, x.db, `INSERT INTO contracts(id,project_id,contract_code,title,contract_type,direction,counterparty_partner_id,counterparty_name,currency,status) VALUES($1,$2,$3,'Hợp đồng',$4,$5,$6,'Đối tác','USD','active')`, contract, x.f.projectID, "HD-"+contract, c.kind, c.direction, x.partner)
		mustExec(t, x.db, `INSERT INTO contract_versions(id,contract_id,version_no,version_label,amount,scope_summary,definition_hash) VALUES($1,$2,1,'V1',100,'Scope','hash')`, version, contract)
		mustExec(t, x.db, `INSERT INTO contract_milestones(id,contract_id,contract_version_id,milestone_code,milestone_name,sequence_no,milestone_type,gross_amount,net_amount,status) VALUES($1,$2,$3,'M1','Đợt 1',1,'progress',100,100,'active')`, milestone, contract, version)
		in := AccountingBasisInput{SourceType: "contract_milestone", SourceID: milestone, Amount: "100.01", Note: "Giá trị xác nhận trên chứng từ", EvidenceIDs: []string{x.evidence}}
		b, err := x.s.CreateAccountingBasis(ctx, x.f.projectID, in, x.f.userID, "Tester")
		if err != nil {
			t.Fatal(err)
		}
		var direction, currency, expected, reference string
		if err = x.db.QueryRow(`SELECT direction,currency,amount::text,snapshot->>'reference_amount' FROM accounting_reconciliation_bases WHERE id=$1`, b["id"]).Scan(&direction, &currency, &expected, &reference); err != nil {
			t.Fatal(err)
		}
		if direction != c.want || currency != "USD" || number(expected) != 100.01 || number(reference) != 100 {
			t.Fatal(direction, currency, expected, reference)
		}
		in.Amount = "100.001"
		in.SourceID = newID()
		if err := accountingMoney(in.Amount, "USD", false); err == nil {
			t.Fatal("invalid currency precision")
		}
	}
	data, err := x.s.AccountingOperationsWorkspace(ctx, x.f.projectID, x.f.userID)
	if err != nil {
		t.Fatal(err)
	}
	for _, source := range data["sources"].([]Record) {
		if source["reference_amount"] == nil || source["direction"] == nil {
			t.Fatal("source context missing", source)
		}
	}
}
func TestAccountingOperationsObserverAndPermissionBoundary(t *testing.T) {
	x := setupAccountingOperations(t)
	ctx := context.Background()
	marker := ensurePermission(t, x.db, "project.reconciliation.view", "Observer", "project")
	mustExec(t, x.db, `INSERT INTO role_permissions(role_id,permission_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, x.f.roleID, marker)
	mustExec(t, x.db, `DELETE FROM project_members WHERE project_id=$1 AND staff_id=$2`, x.f.projectID, x.f.staffID)
	if _, err := x.s.AccountingOperationsWorkspace(ctx, x.f.projectID, x.f.userID); err != nil {
		t.Fatal(err)
	}
	record := x.importRow(t, "GLOBAL-PC", "20", "")
	x.allocate(t, record, "20")
	allowed, err := x.s.repo.CanAccessProjectPermission(ctx, x.f.userID, x.f.projectID, []string{"warehouse.issue"})
	if err != nil || allowed {
		t.Fatal(allowed, err)
	}
	mustExec(t, x.db, `DELETE FROM role_permissions WHERE role_id=$1 AND permission_id IN (SELECT id FROM permissions WHERE key IN ('accounting_reconciliation.manage','accounting_misa.reconcile'))`, x.f.roleID)
	if _, err := x.s.PreviewAccountingMISA(ctx, x.f.projectID, AccountingMISAImportInput{}, x.f.userID); err == nil {
		t.Fatal("unauthorized import")
	}
	if _, err := x.s.AllocateAccountingMISA(ctx, x.f.projectID, AccountingMISAAllocationInput{ID: newID(), BasisID: x.basis, RecordID: record, Amount: "1"}, x.f.userID, "Tester"); err == nil {
		t.Fatal("unauthorized allocation")
	}
	b := x.basisRow(t)
	if _, err := x.s.ReviewAccountingMISA(ctx, x.f.projectID, AccountingMISAReviewInput{BasisID: x.basis, Fingerprint: fmt.Sprint(b["fingerprint"]), Note: "Review"}, x.f.userID, "Tester"); err == nil {
		t.Fatal("unauthorized review")
	}
}
func TestAccountingOperationsHTTPContractsAndProtectedOriginal(t *testing.T) {
	x := setupAccountingOperations(t)
	ctx := context.Background()
	app := batch4TestApp(x.f.userID, func(app *fiber.App, h *Handler) { registerFinanceRoutes(app, h) }, NewHandler(x.s))
	content := accountingMISAHeader + "\n" + fmt.Sprintf("PC-HTTP,1,transaction,outbound,%s,VND,20,2026-09-17,\n", x.partner)
	batch, err := x.s.PreviewAccountingMISA(ctx, x.f.projectID, AccountingMISAImportInput{Entity: "TEST", Filename: "original.csv", Content: content}, x.f.userID)
	if err != nil {
		t.Fatal(err)
	}
	res, err := app.Test(httptest.NewRequest(http.MethodGet, "/projects/"+x.f.projectID+"/misa-imports/"+cleanID(batch["id"])+"/file", nil))
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != 200 || string(body) != content || res.Header.Get("X-Content-Type-Options") != "nosniff" || !strings.Contains(res.Header.Get("Content-Disposition"), "original.csv") {
		t.Fatal(res.StatusCode, string(body), res.Header)
	}
	res, err = app.Test(httptest.NewRequest(http.MethodGet, "/projects/"+x.f.otherProjectID+"/misa-imports/"+cleanID(batch["id"])+"/file", nil))
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 403 {
		t.Fatal("cross project file", res.StatusCode)
	}
	if _, err = x.s.ConfirmAccountingMISA(ctx, x.f.projectID, cleanID(batch["id"]), x.f.userID, "Tester"); err != nil {
		t.Fatal(err)
	}
	var record string
	if err = x.db.QueryRow(`SELECT id FROM accounting_misa_records WHERE batch_id=$1`, batch["id"]).Scan(&record); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/projects/"+x.f.projectID+"/misa-allocations", strings.NewReader(fmt.Sprintf(`{"id":%q,"basis_id":%q,"record_id":%q,"amount":"10"}`, newID(), x.basis, record)))
	req.Header.Set("Content-Type", "application/json")
	res, err = app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ = io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		t.Fatal(res.StatusCode, string(body))
	}
	for _, route := range []string{"/me/accounting-requests", "/projects/" + x.f.projectID + "/accounting-requests"} {
		res, err = app.Test(httptest.NewRequest(http.MethodGet, route, nil))
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != 404 {
			t.Fatal("retired route", route, res.StatusCode)
		}
	}
	data, err := x.s.AccountingOperationsWorkspace(ctx, x.f.projectID, x.f.userID)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := data["requests"]; ok {
		t.Fatal("proposal list leaked")
	}
	var eventAmount string
	for _, event := range data["events"].([]Record) {
		if fmt.Sprint(event["action"]) == "accounting_misa_allocated" {
			snapshot, ok := event["snapshot"].(map[string]any)
			if !ok {
				t.Fatal("history snapshot is not an object", event)
			}
			eventAmount = fmt.Sprint(snapshot["amount"])
		}
	}
	if eventAmount != "10" {
		t.Fatal("allocation history lost amount", eventAmount)
	}
}

func TestAccountingOperationsManyToManyExactMoneyAndAuthorizedCrossProject(t *testing.T) {
	x := setupAccountingOperations(t)
	ctx := context.Background()
	// Allow both projects so the mismatch exercises record ownership, not just membership.
	mustExec(t, x.db, `INSERT INTO project_members(project_id,staff_id,role_ids,active) VALUES($1,$2,'["Member"]',true)`, x.f.otherProjectID, x.f.staffID)
	secondSource := newID()
	mustExec(t, x.db, `INSERT INTO procurement_requests(id,project_id,request_code,title,status,supplier_partner_id,total_estimated_amount) VALUES($1,$2,$3,'Other basis','ordered',$4,1)`, secondSource, x.f.projectID, "SECOND-"+secondSource, x.partner)
	basis, err := x.s.CreateAccountingBasis(ctx, x.f.projectID, AccountingBasisInput{SourceType: "procurement_request", SourceID: secondSource, Amount: "100", Note: "Evidence amount", EvidenceIDs: []string{x.evidence}}, x.f.userID, "Tester")
	if err != nil {
		t.Fatal(err)
	}
	second := x
	second.basis = cleanID(basis["id"])
	one := x.importRow(t, "SHARED", "900719925474099", "")
	two := x.importRow(t, "SPLIT", "30", "")
	x.allocate(t, one, "900719925474039")
	second.allocate(t, one, "60")
	second.allocate(t, two, "30")
	var total string
	if err = x.db.QueryRow(`SELECT SUM(amount)::text FROM accounting_misa_allocations WHERE record_id=$1 AND active`, one).Scan(&total); err != nil {
		t.Fatal(err)
	}
	if total != "900719925474099.000" && total != "900719925474099" {
		t.Fatal("money lost precision", total)
	}
	if _, err = x.s.AllocateAccountingMISA(ctx, x.f.projectID, AccountingMISAAllocationInput{ID: newID(), BasisID: x.basis, RecordID: one, Amount: "1"}, x.f.userID, "Tester"); err == nil {
		t.Fatal("record cap shared across bases exceeded")
	}
	content := accountingMISAHeader + "\n" + fmt.Sprintf("OTHER-PROJECT,1,transaction,outbound,%s,VND,100,2026-09-17,\n", x.partner)
	b, err := x.s.PreviewAccountingMISA(ctx, x.f.otherProjectID, AccountingMISAImportInput{Entity: "TEST", Filename: "other.csv", Content: content}, x.f.userID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = x.s.ConfirmAccountingMISA(ctx, x.f.otherProjectID, cleanID(b["id"]), x.f.userID, "Tester"); err != nil {
		t.Fatal(err)
	}
	var otherRecord string
	if err = x.db.QueryRow(`SELECT id FROM accounting_misa_records WHERE batch_id=$1`, b["id"]).Scan(&otherRecord); err != nil {
		t.Fatal(err)
	}
	if _, err = x.s.AllocateAccountingMISA(ctx, x.f.projectID, AccountingMISAAllocationInput{ID: newID(), BasisID: x.basis, RecordID: otherRecord, Amount: "1"}, x.f.userID, "Tester"); err == nil {
		t.Fatal("cross-project record matched")
	}
	// Source identity staleness cannot be bypassed by keeping a former immutable basis.
	otherPartner := newID()
	mustExec(t, x.db, `INSERT INTO partners(id,partner_code,name) VALUES($1,$2,'New supplier')`, otherPartner, "NEW-"+otherPartner)
	mustExec(t, x.db, `UPDATE procurement_requests SET supplier_partner_id=$2 WHERE id=$1`, x.source, otherPartner)
	before := x.basisRow(t)
	if _, err = x.s.ReviewAccountingMISA(ctx, x.f.projectID, AccountingMISAReviewInput{BasisID: x.basis, Fingerprint: fmt.Sprint(before["fingerprint"]), Note: "Stale supplier"}, x.f.userID, "Tester"); err == nil {
		t.Fatal("changed source identity reviewed")
	}
	assertCount(t, x.db, `SELECT COUNT(*) FROM accounting_misa_allocations WHERE project_id=$1`, 3, x.f.projectID)
}
func TestAccountingOperationsFailedImportRollsBackAllRowsAndAudit(t *testing.T) {
	x := setupAccountingOperations(t)
	ctx := context.Background()
	existing := x.importRow(t, "EXISTS", "100", "")
	content := accountingMISAHeader + "\n" + fmt.Sprintf("NEW,1,transaction,outbound,%s,VND,100,2026-09-17,\nEXISTS,1,transaction,outbound,%s,VND,90,2026-09-17,\n", x.partner, x.partner)
	b, err := x.s.PreviewAccountingMISA(ctx, x.f.projectID, AccountingMISAImportInput{Entity: "TEST", Filename: "bad-change.csv", Content: content}, x.f.userID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = x.s.ConfirmAccountingMISA(ctx, x.f.projectID, cleanID(b["id"]), x.f.userID, "Tester"); err == nil {
		t.Fatal("unidentified replacement allowed")
	}
	assertCount(t, x.db, `SELECT COUNT(*) FROM accounting_misa_records WHERE project_id=$1`, 1, x.f.projectID)
	assertCount(t, x.db, `SELECT COUNT(*) FROM accounting_misa_records WHERE id=$1 AND active`, 1, existing)
	assertCount(t, x.db, `SELECT COUNT(*) FROM audit_logs WHERE project_id=$1 AND event_type='accounting_misa_import_confirmed'`, 1, x.f.projectID)
	var status string
	if err = x.db.QueryRow(`SELECT status FROM accounting_misa_batches WHERE id=$1`, b["id"]).Scan(&status); err != nil || status != "preview" {
		t.Fatal(status, err)
	}
}

func TestAccountingOperationsCurrentReviewTracksSourceEvidenceEligibility(t *testing.T) {
	x := setupAccountingOperations(t)
	ctx := context.Background()
	b := x.basisRow(t)
	fingerprint := fmt.Sprint(b["fingerprint"])
	// A review of an unmatched basis records its difference; it does not manufacture payment.
	if _, err := x.s.ReviewAccountingMISA(ctx, x.f.projectID, AccountingMISAReviewInput{BasisID: x.basis, Fingerprint: fingerprint, Note: "Đã kiểm tra, chưa có giao dịch"}, x.f.userID, "Tester"); err != nil {
		t.Fatal(err)
	}
	for _, step := range []struct {
		change, restore string
		args            []any
	}{
		{`UPDATE evidence_records SET status='rejected' WHERE id=$1`, `UPDATE evidence_records SET status='submitted' WHERE id=$1`, []any{x.evidence}},
		{`UPDATE procurement_requests SET status='cancelled' WHERE id=$1`, `UPDATE procurement_requests SET status='ordered' WHERE id=$1`, []any{x.source}},
		{`UPDATE procurement_requests SET supplier_partner_id=NULL WHERE id=$1`, `UPDATE procurement_requests SET supplier_partner_id=$2 WHERE id=$1`, []any{x.source, x.partner}},
	} {
		// The first two statements use one bind; supplier restoration uses two.
		args := step.args
		if !strings.Contains(step.change, "$2") {
			args = args[:1]
		}
		mustExec(t, x.db, step.change, args...)
		current := x.basisRow(t)
		if boolValue(current["source_available"]) || boolValue(current["reviewed"]) || fmt.Sprint(current["source_unavailable_reason"]) == "" {
			t.Fatal("stale source remains checked", current)
		}
		assertCount(t, x.db, `SELECT COUNT(*) FROM accounting_misa_reviews WHERE basis_id=$1`, 1, x.basis)
		args = step.args
		if !strings.Contains(step.restore, "$2") {
			args = args[:1]
		}
		mustExec(t, x.db, step.restore, args...)
		current = x.basisRow(t)
		if !boolValue(current["source_available"]) || !boolValue(current["reviewed"]) || fmt.Sprint(current["fingerprint"]) != fingerprint {
			t.Fatal("exact restored source not checked", current)
		}
	}
	permission := ensurePermission(t, x.db, "procurement.view", "View procurement", "procurement")
	mustExec(t, x.db, `DELETE FROM role_permissions WHERE role_id=$1 AND permission_id=$2`, x.f.roleID, permission)
	current := x.basisRow(t)
	if boolValue(current["source_available"]) || boolValue(current["reviewed"]) {
		t.Fatal("missing source permission remains checked", current)
	}
	// Historical immutable reference and review stay readable in the reconciliation domain.
	assertCount(t, x.db, `SELECT COUNT(*) FROM accounting_misa_reviews WHERE basis_id=$1`, 1, x.basis)
	mustExec(t, x.db, `INSERT INTO role_permissions(role_id,permission_id) VALUES($1,$2)`, x.f.roleID, permission)
	current = x.basisRow(t)
	if !boolValue(current["source_available"]) || !boolValue(current["reviewed"]) {
		t.Fatal(current)
	}
	assertCount(t, x.db, `SELECT COUNT(*) FROM accounting_misa_allocations WHERE basis_id=$1`, 0, x.basis)
}

func accountingWaitForBlockedWriter(t *testing.T, db *sql.DB, blockerPID int, completed <-chan error) {
	t.Helper()
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()
	for {
		var waiting bool
		err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM pg_locks waiting JOIN pg_locks held
   ON waiting.locktype=held.locktype AND waiting.database=held.database AND waiting.classid=held.classid AND waiting.objid=held.objid AND waiting.objsubid=held.objsubid
   WHERE held.pid=$1 AND held.locktype='advisory' AND held.granted AND NOT waiting.granted)`, blockerPID).Scan(&waiting)
		if err != nil {
			t.Fatal(err)
		}
		if waiting {
			return
		}
		select {
		case err := <-completed:
			t.Fatalf("writer never reached held project lock: %v", err)
		case <-deadline.C:
			t.Fatal("writer did not reach held project lock")
		case <-tick.C:
		}
	}
}
func TestAccountingOperationsReauthorizesBlockedWriters(t *testing.T) {
	cases := []struct{ name, operation, revocation string }{
		{"basis_manage", "basis", "accounting_reconciliation.manage"},
		{"basis_source", "basis", "procurement.view"},
		{"basis_evidence_visibility", "basis", "evidence"},
		{"allocation_permission", "allocate", "accounting_misa.reconcile"},
		{"allocation_source", "allocate", "procurement.view"},
		{"allocation_evidence_visibility", "allocate", "evidence"},
		{"review_permission", "review", "accounting_misa.reconcile"},
		{"review_evidence_visibility", "review", "evidence"},
		{"review_design_evidence_visibility", "review", "project.view"},
		{"review_actor_deactivated", "review", "actor"},
		{"revoke_permission", "revoke", "accounting_misa.reconcile"},
		{"confirm_permission", "confirm", "accounting_misa.reconcile"},
		{"preview_permission", "preview", "accounting_misa.reconcile"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			x := setupAccountingOperations(t)
			if c.name == "review_design_evidence_visibility" {
				accountingAttachDesignEvidence(t, x)
			}
			record := x.importRow(t, "WAIT-PC", "100", "")
			fingerprint := fmt.Sprint(x.basisRow(t)["fingerprint"])
			source := newID()
			mustExec(t, x.db, `INSERT INTO procurement_requests(id,project_id,request_code,title,status,supplier_partner_id) VALUES($1,$2,$3,'Wait source','ordered',$4)`, source, x.f.projectID, "WAIT-"+source, x.partner)
			input := AccountingBasisInput{SourceType: "procurement_request", SourceID: source, Amount: "100", Note: "Signed source", EvidenceIDs: []string{x.evidence}}
			allocation := ""
			if c.operation == "revoke" {
				allocation = x.allocate(t, record, "10")
			}
			batchID := ""
			if c.operation == "confirm" {
				content := accountingMISAHeader + "\n" + fmt.Sprintf("WAIT-CONFIRM,1,transaction,outbound,%s,VND,100,2026-09-17,\n", x.partner)
				batch, err := x.s.PreviewAccountingMISA(context.Background(), x.f.projectID, AccountingMISAImportInput{Entity: "TEST", Filename: "wait.csv", Content: content}, x.f.userID)
				if err != nil {
					t.Fatal(err)
				}
				batchID = cleanID(batch["id"])
			}
			var audits, bases, allocations, reviews, records, batches int
			if err := x.db.QueryRow(`SELECT (SELECT COUNT(*) FROM audit_logs WHERE project_id=$1),(SELECT COUNT(*) FROM accounting_reconciliation_bases WHERE project_id=$1),(SELECT COUNT(*) FROM accounting_misa_allocations WHERE project_id=$1),(SELECT COUNT(*) FROM accounting_misa_reviews WHERE project_id=$1),(SELECT COUNT(*) FROM accounting_misa_records WHERE project_id=$1),(SELECT COUNT(*) FROM accounting_misa_batches WHERE project_id=$1)`, x.f.projectID).Scan(&audits, &bases, &allocations, &reviews, &records, &batches); err != nil {
				t.Fatal(err)
			}
			blocker, err := x.db.Begin()
			if err != nil {
				t.Fatal(err)
			}
			defer blocker.Rollback()
			var blockerPID int
			if err = blocker.QueryRow(`SELECT pg_backend_pid()`).Scan(&blockerPID); err != nil {
				t.Fatal(err)
			}
			if _, err = blocker.Exec(`SELECT pg_advisory_xact_lock(hashtextextended('accounting_operations:' || $1,0))`, x.f.projectID); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			completed := make(chan error, 1)
			go func() {
				var err error
				switch c.operation {
				case "basis":
					_, err = x.s.CreateAccountingBasis(ctx, x.f.projectID, input, x.f.userID, "Tester")
				case "allocate":
					_, err = x.s.AllocateAccountingMISA(ctx, x.f.projectID, AccountingMISAAllocationInput{ID: newID(), BasisID: x.basis, RecordID: record, Amount: "10"}, x.f.userID, "Tester")
				case "review":
					_, err = x.s.ReviewAccountingMISA(ctx, x.f.projectID, AccountingMISAReviewInput{BasisID: x.basis, Fingerprint: fingerprint, Note: "Checked"}, x.f.userID, "Tester")
				case "revoke":
					_, err = x.s.RevokeAccountingAllocation(ctx, x.f.projectID, allocation, "Recovery", x.f.userID, "Tester")
				case "confirm":
					_, err = x.s.ConfirmAccountingMISA(ctx, x.f.projectID, batchID, x.f.userID, "Tester")
				case "preview":
					content := accountingMISAHeader + "\n" + fmt.Sprintf("WAIT-PREVIEW,1,transaction,outbound,%s,VND,100,2026-09-17,\n", x.partner)
					_, err = x.s.PreviewAccountingMISA(ctx, x.f.projectID, AccountingMISAImportInput{Entity: "TEST", Filename: "wait-preview.csv", Content: content}, x.f.userID)
				}
				completed <- err
			}()
			accountingWaitForBlockedWriter(t, x.db, blockerPID, completed)
			switch c.revocation {
			case "actor":
				mustExec(t, x.db, `UPDATE users SET status='inactive' WHERE id=$1`, x.f.userID)
			case "evidence":
				mustExec(t, x.db, `UPDATE work_items SET assignee_staff_id=NULL,watcher_staff_ids='[]' WHERE id=(SELECT work_item_id FROM evidence_records WHERE id=$1)`, x.evidence)
			default:
				mustExec(t, x.db, `DELETE FROM role_permissions WHERE role_id=$1 AND permission_id=(SELECT id FROM permissions WHERE key=$2)`, x.f.roleID, c.revocation)
			}
			if err = blocker.Commit(); err != nil {
				t.Fatal(err)
			}
			var denied StatusError
			select {
			case err = <-completed:
				if !errors.As(err, &denied) || denied.Status != http.StatusForbidden {
					t.Fatalf("post-wait authorization=%v", err)
				}
			case <-ctx.Done():
				t.Fatal("writer failed to leave lock wait")
			}
			assertCount(t, x.db, `SELECT COUNT(*) FROM audit_logs WHERE project_id=$1`, audits, x.f.projectID)
			assertCount(t, x.db, `SELECT COUNT(*) FROM accounting_reconciliation_bases WHERE project_id=$1`, bases, x.f.projectID)
			assertCount(t, x.db, `SELECT COUNT(*) FROM accounting_misa_allocations WHERE project_id=$1`, allocations, x.f.projectID)
			assertCount(t, x.db, `SELECT COUNT(*) FROM accounting_misa_reviews WHERE project_id=$1`, reviews, x.f.projectID)
			assertCount(t, x.db, `SELECT COUNT(*) FROM accounting_misa_records WHERE project_id=$1`, records, x.f.projectID)
			assertCount(t, x.db, `SELECT COUNT(*) FROM accounting_misa_batches WHERE project_id=$1`, batches, x.f.projectID)
			if c.operation == "revoke" {
				assertCount(t, x.db, `SELECT COUNT(*) FROM accounting_misa_allocations WHERE id=$1 AND active`, 1, allocation)
			}
		})
	}
}
func TestAccountingOperationsSingleConnectionAndUnavailableBasisRecovery(t *testing.T) {
	x := setupAccountingOperations(t)
	accountingAttachDesignEvidence(t, x)
	x.db.SetMaxOpenConns(1)
	x.db.SetMaxIdleConns(1)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	source := newID()
	mustExec(t, x.db, `INSERT INTO procurement_requests(id,project_id,request_code,title,status,supplier_partner_id) VALUES($1,$2,$3,'Single connection','ordered',$4)`, source, x.f.projectID, "SINGLE-"+source, x.partner)
	if _, err := x.s.CreateAccountingBasis(ctx, x.f.projectID, AccountingBasisInput{SourceType: "procurement_request", SourceID: source, Amount: "100", Note: "Signed", EvidenceIDs: []string{x.evidence}}, x.f.userID, "Tester"); err != nil {
		t.Fatal(err)
	}
	content := accountingMISAHeader + "\n" + fmt.Sprintf("SINGLE,1,transaction,outbound,%s,VND,100,2026-09-17,\n", x.partner)
	batch, err := x.s.PreviewAccountingMISA(ctx, x.f.projectID, AccountingMISAImportInput{Entity: "TEST", Filename: "single.csv", Content: content}, x.f.userID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = x.s.ConfirmAccountingMISA(ctx, x.f.projectID, cleanID(batch["id"]), x.f.userID, "Tester"); err != nil {
		t.Fatal(err)
	}
	var record string
	if err = x.db.QueryRowContext(ctx, `SELECT id FROM accounting_misa_records WHERE batch_id=$1`, batch["id"]).Scan(&record); err != nil {
		t.Fatal(err)
	}
	allocation := newID()
	if _, err = x.s.AllocateAccountingMISA(ctx, x.f.projectID, AccountingMISAAllocationInput{ID: allocation, BasisID: x.basis, RecordID: record, Amount: "10"}, x.f.userID, "Tester"); err != nil {
		t.Fatal(err)
	}
	b := x.basisRow(t)
	if _, err = x.s.ReviewAccountingMISA(ctx, x.f.projectID, AccountingMISAReviewInput{BasisID: x.basis, Fingerprint: fmt.Sprint(b["fingerprint"]), Note: "Checked partial"}, x.f.userID, "Tester"); err != nil {
		t.Fatal(err)
	}
	mustExec(t, x.db, `UPDATE evidence_records SET status='rejected' WHERE id=$1`, x.evidence)
	mustExec(t, x.db, `DELETE FROM role_permissions WHERE role_id=$1 AND permission_id=(SELECT id FROM permissions WHERE key='procurement.view')`, x.f.roleID)
	b = x.basisRow(t)
	if boolValue(b["source_available"]) || boolValue(b["reviewed"]) {
		t.Fatal(b)
	}
	// Recovery is authorized by exact project/MISA scope even when the source is unavailable.
	if _, err = x.s.RevokeAccountingAllocation(ctx, x.f.projectID, allocation, "Nguồn không còn đủ điều kiện", x.f.userID, "Tester"); err != nil {
		t.Fatal(err)
	}
	assertCount(t, x.db, `SELECT COUNT(*) FROM accounting_misa_allocations WHERE id=$1 AND NOT active AND revoked_by=$2 AND revoke_reason<>''`, 1, allocation, x.f.userID)
	assertCount(t, x.db, `SELECT COUNT(*) FROM audit_logs WHERE project_id=$1 AND event_type='accounting_misa_allocation_revoked' AND actor_id=$2`, 1, x.f.projectID, x.f.userID)
}

func accountingAttachDesignEvidence(t *testing.T, x accountingOperationsFixture) {
	t.Helper()
	permission := ensurePermission(t, x.db, "project.view", "View project", "project")
	mustExec(t, x.db, `INSERT INTO role_permissions(role_id,permission_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, x.f.roleID, permission)
	release := newID()
	mustExec(t, x.db, `INSERT INTO design_releases(id,project_id,revision_no,revision_label,change_reason,status) VALUES($1,$2,1,'AUTH-1','Visibility regression','draft')`, release, x.f.projectID)
	mustExec(t, x.db, `INSERT INTO design_release_items(project_id,design_release_id,evidence_id,document_type) VALUES($1,$2,$3,'other')`, x.f.projectID, release, x.evidence)
}
func TestAccountingOperationsLargeFractionalHistoryPreservesDecimalStrings(t *testing.T) {
	x := setupAccountingOperations(t)
	ctx := context.Background()
	contract, version, milestone := newID(), newID(), newID()
	mustExec(t, x.db, `INSERT INTO contracts(id,project_id,contract_code,title,contract_type,direction,counterparty_partner_id,counterparty_name,currency,status) VALUES($1,$2,$3,'Large source','main_client','receivable',$4,'Partner','USD','active')`, contract, x.f.projectID, "LARGE-"+contract, x.partner)
	mustExec(t, x.db, `INSERT INTO contract_versions(id,contract_id,version_no,version_label,amount,scope_summary,definition_hash) VALUES($1,$2,1,'V1',100,'Scope','hash')`, version, contract)
	mustExec(t, x.db, `INSERT INTO contract_milestones(id,contract_id,contract_version_id,milestone_code,milestone_name,sequence_no,milestone_type,gross_amount,net_amount,status) VALUES($1,$2,$3,'M1','Large',1,'progress',999999999999999.99,999999999999999.99,'active')`, milestone, contract, version)
	b, err := x.s.CreateAccountingBasis(ctx, x.f.projectID, AccountingBasisInput{SourceType: "contract_milestone", SourceID: milestone, Amount: "999999999999999.99", Note: "Signed large amount", EvidenceIDs: []string{x.evidence}}, x.f.userID, "Tester")
	if err != nil {
		t.Fatal(err)
	}
	data, err := x.s.AccountingOperationsWorkspace(ctx, x.f.projectID, x.f.userID)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, event := range data["events"].([]Record) {
		if cleanID(event["basis_id"]) != cleanID(b["id"]) {
			continue
		}
		found = true
		snapshot, ok := event["snapshot"].(map[string]any)
		if !ok {
			t.Fatal(event)
		}
		amount, ok := snapshot["amount"].(string)
		if !ok || amount != "999999999999999.990" {
			t.Fatal("history amount lost decimal precision", snapshot["amount"])
		}
		source, ok := snapshot["snapshot"].(map[string]any)
		if !ok {
			t.Fatal(snapshot)
		}
		reference, ok := source["reference_amount"].(string)
		if !ok || reference != "999999999999999.99" {
			t.Fatal("source reference lost precision", source)
		}
	}
	if !found {
		t.Fatal("basis history missing")
	}
}

func TestAccountingOperationsReauthorizesAfterEligibilityRowWait(t *testing.T) {
	for _, c := range []struct{ name, operation, lock, revocation string }{
		{"basis_manage_after_evidence_wait", "basis", "evidence", "accounting_reconciliation.manage"},
		{"allocation_evidence_after_work_wait", "allocate", "evidence", "evidence"},
		{"allocation_permission_after_work_wait", "allocate", "evidence", "accounting_misa.reconcile"},
		{"review_permission_after_work_wait", "review", "evidence", "accounting_misa.reconcile"},
		{"allocation_source_after_record_wait", "allocate", "record", "procurement.view"},
		{"allocation_design_visibility_after_record_wait", "allocate", "record", "project.view"},
		{"revoke_permission_after_allocation_wait", "revoke", "allocation", "accounting_misa.reconcile"},
	} {
		t.Run(c.name, func(t *testing.T) {
			x := setupAccountingOperations(t)
			if c.revocation == "project.view" {
				accountingAttachDesignEvidence(t, x)
			}
			record := x.importRow(t, "ROW-WAIT", "100", "")
			fingerprint := fmt.Sprint(x.basisRow(t)["fingerprint"])
			allocation := ""
			if c.operation == "revoke" {
				allocation = x.allocate(t, record, "10")
			}
			source := newID()
			mustExec(t, x.db, `INSERT INTO procurement_requests(id,project_id,request_code,title,status,supplier_partner_id) VALUES($1,$2,$3,'Row source','ordered',$4)`, source, x.f.projectID, "ROW-"+source, x.partner)
			blocker, err := x.db.Begin()
			if err != nil {
				t.Fatal(err)
			}
			defer blocker.Rollback()
			var blockerPID int
			if err = blocker.QueryRow(`SELECT pg_backend_pid()`).Scan(&blockerPID); err != nil {
				t.Fatal(err)
			}
			var id string
			switch c.lock {
			case "evidence":
				err = blocker.QueryRow(`SELECT w.id FROM work_items w JOIN evidence_records e ON e.work_item_id=w.id WHERE e.id=$1 FOR UPDATE OF w`, x.evidence).Scan(&id)
			case "record":
				err = blocker.QueryRow(`SELECT id FROM accounting_misa_records WHERE id=$1 FOR UPDATE`, record).Scan(&id)
			case "allocation":
				err = blocker.QueryRow(`SELECT id FROM accounting_misa_allocations WHERE id=$1 FOR UPDATE`, allocation).Scan(&id)
			}
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			completed := make(chan error, 1)
			go func() {
				var err error
				switch c.operation {
				case "basis":
					_, err = x.s.CreateAccountingBasis(ctx, x.f.projectID, AccountingBasisInput{SourceType: "procurement_request", SourceID: source, Amount: "100", Note: "Signed row source", EvidenceIDs: []string{x.evidence}}, x.f.userID, "Tester")
				case "allocate":
					_, err = x.s.AllocateAccountingMISA(ctx, x.f.projectID, AccountingMISAAllocationInput{ID: newID(), BasisID: x.basis, RecordID: record, Amount: "10"}, x.f.userID, "Tester")
				case "review":
					_, err = x.s.ReviewAccountingMISA(ctx, x.f.projectID, AccountingMISAReviewInput{BasisID: x.basis, Fingerprint: fingerprint, Note: "Checked row source"}, x.f.userID, "Tester")
				case "revoke":
					_, err = x.s.RevokeAccountingAllocation(ctx, x.f.projectID, allocation, "Recovery", x.f.userID, "Tester")
				}
				completed <- err
			}()
			deadline := time.NewTimer(5 * time.Second)
			defer deadline.Stop()
			tick := time.NewTicker(10 * time.Millisecond)
			defer tick.Stop()
			waiting := false
			for !waiting {
				if err = x.db.QueryRow(`SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE $1=ANY(pg_blocking_pids(pid)) AND wait_event_type='Lock')`, blockerPID).Scan(&waiting); err != nil {
					t.Fatal(err)
				}
				if waiting {
					break
				}
				select {
				case err := <-completed:
					t.Fatalf("writer never reached eligibility row lock: %v", err)
				case <-deadline.C:
					t.Fatal("no eligibility row wait")
				case <-tick.C:
				}
			}
			if c.revocation == "evidence" {
				if _, err = blocker.Exec(`UPDATE work_items SET assignee_staff_id=NULL,watcher_staff_ids='[]' WHERE id=$1`, id); err != nil {
					t.Fatal(err)
				}
			} else {
				mustExec(t, x.db, `DELETE FROM role_permissions WHERE role_id=$1 AND permission_id=(SELECT id FROM permissions WHERE key=$2)`, x.f.roleID, c.revocation)
			}
			if err = blocker.Commit(); err != nil {
				t.Fatal(err)
			}
			var denied StatusError
			select {
			case err = <-completed:
				if !errors.As(err, &denied) || denied.Status != http.StatusForbidden {
					t.Fatalf("after-row-wait authorization=%v", err)
				}
			case <-ctx.Done():
				t.Fatal("writer failed to leave row lock wait")
			}
			assertCount(t, x.db, `SELECT COUNT(*) FROM accounting_reconciliation_bases WHERE project_id=$1`, 1, x.f.projectID)
			assertCount(t, x.db, `SELECT COUNT(*) FROM accounting_misa_reviews WHERE project_id=$1`, 0, x.f.projectID)
			if c.operation == "revoke" {
				assertCount(t, x.db, `SELECT COUNT(*) FROM accounting_misa_allocations WHERE id=$1 AND active`, 1, allocation)
				assertCount(t, x.db, `SELECT COUNT(*) FROM audit_logs WHERE project_id=$1 AND event_type='accounting_misa_allocation_revoked'`, 0, x.f.projectID)
			} else {
				assertCount(t, x.db, `SELECT COUNT(*) FROM accounting_misa_allocations WHERE project_id=$1`, 0, x.f.projectID)
				assertCount(t, x.db, `SELECT COUNT(*) FROM audit_logs WHERE project_id=$1 AND event_type IN ('accounting_misa_allocated','accounting_misa_reviewed')`, 0, x.f.projectID)
				assertCount(t, x.db, `SELECT COUNT(*) FROM audit_logs WHERE project_id=$1 AND event_type='accounting_basis_confirmed'`, 1, x.f.projectID)
			}
		})
	}
}
