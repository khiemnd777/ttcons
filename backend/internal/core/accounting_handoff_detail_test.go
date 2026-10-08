package core

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestAccountingHandoffDetailValidatesIdentity(t *testing.T) {
	s := NewService(nil, "test")
	for _, pair := range [][2]string{{"procurement_request", "bad"}, {"payable", newID()}} {
		_, err := s.AccountingHandoffDetail(context.Background(), newID(), pair[0], pair[1], newID())
		assertValidationError(t, err)
	}
}

func TestAccountingHandoffDetailScopeSnapshotAndStockIdentity(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	f := setupProjectScopeFixture(t, db, []string{"accounting_handoff.view", "accounting_handoff.submit", "accounting_handoff.acknowledge", "procurement.view", "warehouse.view"})
	s := NewService(NewRepository(db), "test")
	procurement, acceptance, material, item := newID(), newID(), newID(), newID()
	stock, wrongType, crossProject, cancelled := newID(), newID(), newID(), newID()
	t.Cleanup(func() {
		_, _ = db.Exec(`DELETE FROM stock_document_items WHERE document_id IN ($1,$2,$3,$4)`, stock, wrongType, crossProject, cancelled)
		_, _ = db.Exec(`DELETE FROM stock_documents WHERE id IN ($1,$2,$3,$4)`, stock, wrongType, crossProject, cancelled)
		_, _ = db.Exec(`DELETE FROM incoming_acceptance_items WHERE id=$1`, item)
		_, _ = db.Exec(`DELETE FROM materials WHERE id=$1`, material)
	})
	mustExec(t, db, `INSERT INTO procurement_requests(id,project_id,request_code,title,status,total_estimated_amount,supplier_name,order_reference) VALUES($1,$2,$3,'Nguồn ban đầu','ordered',100,'NCC','PO')`, procurement, f.projectID, "PR-"+procurement)
	handoff, err := s.CreateAccountingHandoff(ctx, f.projectID, AccountingHandoffRequest{SourceType: "procurement_request", SourceID: procurement}, f.userID, "Sender")
	if err != nil {
		t.Fatal(err)
	}
	mustExec(t, db, `UPDATE procurement_requests SET title='Nguồn đã đổi',total_estimated_amount=150 WHERE id=$1`, procurement)
	mustExec(t, db, `INSERT INTO incoming_acceptance_records(id,project_id,acceptance_code,procurement_request_id,status) VALUES($1,$2,$3,$4,'submitted')`, acceptance, f.projectID, "NT-"+acceptance, procurement)
	mustExec(t, db, `INSERT INTO materials(id,code,name,unit) VALUES($1,$2,'Vật tư','cái')`, material, "MAT-"+material)
	mustExec(t, db, `INSERT INTO incoming_acceptance_items(id,incoming_acceptance_id,project_id,item_index,material_id,material_name,unit,delivered_quantity,accepted_quantity) VALUES($1,$2,$3,0,$4,'Vật tư','cái',2,2)`, item, acceptance, f.projectID, material)
	for _, row := range []struct{ id, project, kind, status string }{{stock, f.projectID, "incoming_acceptance", "posted"}, {wrongType, f.projectID, "other", "posted"}, {crossProject, f.otherProjectID, "incoming_acceptance", "posted"}, {cancelled, f.projectID, "incoming_acceptance", "cancelled"}} {
		mustExec(t, db, `INSERT INTO stock_documents(id,document_code,project_id,movement_type,reference_type,reference_id,status) VALUES($1,$2,$3,'receive',$4,$5,$6)`, row.id, "PNK-"+row.id, row.project, row.kind, acceptance, row.status)
	}
	// The same stock document has two paths/lines. It must appear once.
	for i := 0; i < 2; i++ {
		mustExec(t, db, `INSERT INTO stock_document_items(document_id,material_id,quantity,unit,source_line_type,source_line_id) VALUES($1,$2,1,'cái','incoming_acceptance_item',$3)`, stock, material, item)
	}
	workItem, evidence := newID(), newID()
	mustExec(t, db, `INSERT INTO work_items(id,project_id,source_type,source_id,title,assignee_staff_id,status) VALUES($1,$2,'acceptance',$3,'Kiểm nhận',$4,'ready')`, workItem, f.projectID, acceptance, f.staffID)
	mustExec(t, db, `INSERT INTO evidence_records(id,work_item_id,project_id,evidence_type,name,mime_type,status) VALUES($1,$2,$3,'photo','Giao nhận.png','image/png','submitted')`, evidence, workItem, f.projectID)
	mustExec(t, db, `UPDATE incoming_acceptance_items SET evidence_id=$2 WHERE id=$1`, item, evidence)
	detail, err := s.AccountingHandoffDetail(ctx, f.projectID, "procurement_request", procurement, f.userID)
	if err != nil {
		t.Fatal(err)
	}
	if detail["current"].(Record)["source_title"] != "Nguồn đã đổi" {
		t.Fatalf("current=%#v", detail)
	}
	snapshot := recordValue(detail["handoff"].(Record)["source_snapshot_json"])
	if snapshot["source_title"] != "Nguồn ban đầu" || number(snapshot["reference_amount"]) != 100 {
		t.Fatalf("snapshot overwritten: %#v", snapshot)
	}
	refs := detail["related"].([]Record)
	if len(refs) != 3 || recordByID(refs, "id", stock) == nil || recordByID(refs, "id", wrongType) != nil || recordByID(refs, "id", crossProject) != nil {
		t.Fatalf("wrong related identities: %#v", refs)
	}
	files := detail["files"].([]Record)
	if len(files) != 1 || cleanID(files[0]["id"]) != evidence || files[0]["kind"] != "evidence" {
		t.Fatalf("wrong evidence identity: %#v", files)
	}
	if !boolValue(detail["capabilities"].(Record)["acknowledge"]) {
		t.Fatalf("missing action: %#v", detail)
	}
	if _, err = s.AccountingHandoffDetail(ctx, f.otherProjectID, "procurement_request", procurement, f.userID); err == nil {
		t.Fatal("cross-project detail allowed")
	}
	if _, err = s.AccountingHandoffDetail(ctx, f.projectID, "procurement_request", newID(), f.userID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("unknown source err=%v", err)
	}
	// Removing source permissions preserves only the explicitly authorized dossier snapshot.
	mustExec(t, db, `DELETE FROM role_permissions WHERE role_id=$1 AND permission_id IN (SELECT id FROM permissions WHERE key IN ('procurement.view','warehouse.view'))`, f.roleID)
	detail, err = s.AccountingHandoffDetail(ctx, f.projectID, "procurement_request", procurement, f.userID)
	if err != nil {
		t.Fatal(err)
	}
	if detail["current"] != nil && detail["current"].(Record) != nil {
		t.Fatal("current source leaked without permission")
	}
	if len(detail["related"].([]Record)) != 0 || len(detail["files"].([]Record)) != 0 || !boolValue(detail["restricted"]) {
		t.Fatalf("restricted source leaked: %#v", detail)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM audit_logs WHERE target_id=$1 AND target_type='accounting_handoff'`, 1, handoff["id"])
	app := batch4TestApp(f.userID, func(app *fiber.App, h *Handler) {
		app.Get("/projects/:id/accounting-handoff-detail", h.AccountingHandoffDetail)
	}, NewHandler(s))
	response, err := app.Test(httptest.NewRequest(http.MethodGet, "/projects/"+f.projectID+"/accounting-handoff-detail?source_type=procurement_request&source_id="+procurement, nil))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("detail endpoint: %d", response.StatusCode)
	}
}

func TestAccountingHandoffDetailMilestoneExactContractAndFiles(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	f := setupProjectScopeFixture(t, db, []string{"accounting_handoff.view", "contract.view"})
	s := NewService(NewRepository(db), "test")
	contract, version, milestone, document, unrelated := newID(), newID(), newID(), newID(), newID()
	mustExec(t, db, `INSERT INTO contracts(id,project_id,contract_code,title,contract_type,direction,counterparty_name,currency,status) VALUES($1,$2,$3,'Hợp đồng CĐT','main_client','receivable','CĐT','USD','active')`, contract, f.projectID, "HD-"+contract)
	mustExec(t, db, `INSERT INTO contract_versions(id,contract_id,version_no,version_label,amount,scope_summary,definition_hash) VALUES($1,$2,1,'V1',100,'Scope','hash')`, version, contract)
	mustExec(t, db, `INSERT INTO contract_milestones(id,contract_id,contract_version_id,milestone_code,milestone_name,sequence_no,milestone_type,gross_amount,net_amount,status) VALUES($1,$2,$3,'M1','Đợt 1',1,'progress',100,100,'active')`, milestone, contract, version)
	for _, row := range []struct{ id, target string }{{document, contract}, {unrelated, newID()}} {
		mustExec(t, db, `INSERT INTO documents(id,project_id,target_type,target_id,name,status,storage_provider,storage_key,mime_type) VALUES($1,$2,'contract',$3,'Hợp đồng.pdf','active','local',$4,'application/pdf')`, row.id, f.projectID, row.target, "test/"+row.id)
	}
	detail, err := s.AccountingHandoffDetail(ctx, f.projectID, "contract_milestone", milestone, f.userID)
	if err != nil {
		t.Fatal(err)
	}
	if detail["current"].(Record)["currency"] != "USD" {
		t.Fatal("lost source currency")
	}
	files := detail["files"].([]Record)
	if len(files) != 1 || cleanID(files[0]["id"]) != document || files[0]["kind"] != "document" {
		t.Fatalf("wrong document identity: %#v", files)
	}
	refs := detail["related"].([]Record)
	if len(refs) != 1 || cleanID(refs[0]["id"]) != contract {
		t.Fatalf("wrong contract: %#v", refs)
	}
	if boolValue(detail["capabilities"].(Record)["handoff"]) {
		t.Fatal("view permission granted mutation")
	}
}

func TestReconciliationSourceDetailDoesNotGrantHandoffAccess(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	f := setupProjectScopeFixture(t, db, []string{"accounting_reconciliation.view", "procurement.view"})
	s := NewService(NewRepository(db), "test")
	sourceID := newID()
	mustExec(t, db, `INSERT INTO procurement_requests(id,project_id,request_code,title,status,total_estimated_amount,supplier_name,order_reference)
		VALUES($1,$2,$3,'Reconciliation source','ordered',100,'NCC','PO')`, sourceID, f.projectID, "PR-"+sourceID)
	mustExec(t, db, `INSERT INTO accounting_handoffs(project_id,direction,source_type,source_id,status,source_snapshot_json,handed_off_by)
		VALUES($1,'payable','procurement_request',$2,'handed_off','{"private_snapshot":"hidden"}'::jsonb,$3)`, f.projectID, sourceID, f.userID)
	detail, err := s.AccountingHandoffDetail(ctx, f.projectID, "procurement_request", sourceID, f.userID)
	if err != nil {
		t.Fatal(err)
	}
	handoff, _ := detail["handoff"].(Record)
	if detail["current"] == nil || len(handoff) != 0 || boolValue(detail["handoff_access"]) {
		t.Fatalf("source reader leaked handoff snapshot: %#v", detail)
	}
	for _, allowed := range detail["capabilities"].(Record) {
		if boolValue(allowed) {
			t.Fatal("source reader received handoff mutation capability")
		}
	}
	mustExec(t, db, `DELETE FROM role_permissions WHERE role_id=$1 AND permission_id IN (SELECT id FROM permissions WHERE key='procurement.view')`, f.roleID)
	if _, err := s.AccountingHandoffDetail(ctx, f.projectID, "procurement_request", sourceID, f.userID); err == nil {
		t.Fatal("reconciliation permission alone must not disclose source or snapshot")
	}
}

func TestAccountingHandoffDetailConcurrentAcknowledgement(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	f := setupProjectScopeFixture(t, db, []string{"accounting_handoff.view", "accounting_handoff.acknowledge", "procurement.view"})
	s := NewService(NewRepository(db), "test")
	id := newID()
	mustExec(t, db, `INSERT INTO procurement_requests(id,project_id,request_code,title,status,total_estimated_amount,supplier_name,order_reference) VALUES($1,$2,$3,'Concurrent','ordered',100,'NCC','PO')`, id, f.projectID, "PR-"+id)
	h, err := s.CreateAccountingHandoff(ctx, f.projectID, AccountingHandoffRequest{SourceType: "procurement_request", SourceID: id}, f.userID, "Sender")
	if err != nil {
		t.Fatal(err)
	}
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() {
			_, err := s.AccountingHandoffAction(ctx, cleanID(h["id"]), "acknowledge", AccountingHandoffActionRequest{}, f.userID, "Receiver")
			results <- err
		}()
	}
	successes := 0
	for i := 0; i < 2; i++ {
		if <-results == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("acknowledgements=%d, want one", successes)
	}
	detail, err := s.AccountingHandoffDetail(ctx, f.projectID, "procurement_request", id, f.userID)
	if err != nil {
		t.Fatal(err)
	}
	if detail["status"] != "acknowledged" || boolValue(detail["capabilities"].(Record)["acknowledge"]) {
		t.Fatalf("stale detail after acknowledgement: %#v", detail)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM audit_logs WHERE target_id=$1 AND event_type='accounting_handoff_acknowledge'`, 1, h["id"])
}
