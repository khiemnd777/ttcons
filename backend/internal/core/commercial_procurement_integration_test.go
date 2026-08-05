package core

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
)

func seedContractExecutionReadiness(t *testing.T, db *sql.DB, projectID, contractID, versionID, actorID string) {
	t.Helper()
	ctx := context.Background()
	var signatureID string
	err := db.QueryRowContext(ctx, `
		WITH form_ref AS (
			SELECT version.id AS form_version_id,template.id AS form_template_id,template.form_type
			FROM workflow_form_versions version
			JOIN workflow_form_templates template ON template.id=version.template_id
			WHERE version.status='published'
			ORDER BY version.published_at NULLS LAST,version.created_at
			LIMIT 1
		), work_row AS (
			INSERT INTO work_items (
				project_id,source_type,source_id,title,work_type,status,priority,role_ids,evidence_requirement_json,created_by,updated_by
			)
			VALUES ($1,'workflow_form_action_item',gen_random_uuid(),'Ký hợp đồng trên App','approval','completed','normal','[]'::jsonb,'{}'::jsonb,$2,$2)
			RETURNING id
		), instance_row AS (
			INSERT INTO workflow_form_instances (
				project_id,work_item_id,form_template_id,form_version_id,form_type,title,status,current_revision,
				answers_snapshot_json,required_role_ids,created_by,submitted_by,submitted_at
			)
			SELECT $1,work_row.id,form_ref.form_template_id,form_ref.form_version_id,form_ref.form_type,
				'Ký hợp đồng trên App','submitted',1,'{}'::jsonb,'[]'::jsonb,$2,$2,now()
			FROM form_ref,work_row
			RETURNING id,work_item_id,form_version_id
		)
		INSERT INTO workflow_form_signatures (
			project_id,work_item_id,form_instance_id,form_version_id,form_revision_no,purpose,
			signer_user_id,signer_name,signer_roles_json,comment,payload_json,payload_hash
		)
		SELECT $1,instance_row.work_item_id,instance_row.id,instance_row.form_version_id,1,'approved_by',
			$2,'Contract Actor','[]'::jsonb,'Ký trong integration test','{}'::jsonb,repeat('a',64)
		FROM instance_row
		RETURNING id
	`, projectID, actorID).Scan(&signatureID)
	if err != nil {
		t.Fatal(err)
	}
	mustExec(t, db, `
		INSERT INTO contract_version_signature_links (contract_version_id,workflow_form_signature_id,purpose,linked_by)
		VALUES ($1,$2,'approved_by',$3)
	`, versionID, signatureID, actorID)
	var documentID string
	if err := db.QueryRowContext(ctx, `
		INSERT INTO documents (
			project_id,target_type,target_id,name,file_url,document_type,status,uploaded_by,
			mime_type,file_size,storage_provider
		)
		VALUES ($1,'contract_returned_scan',$2,'signed-contract.pdf','/test/signed-contract.pdf',
			'returned_signed_contract_scan','active',$3,'application/pdf',0,'external')
		RETURNING id
	`, projectID, contractID, actorID).Scan(&documentID); err != nil {
		t.Fatal(err)
	}
	mustExec(t, db, `
		INSERT INTO contract_dispatches (
			contract_id,contract_version_id,dispatch_no,recipient_name,delivery_method,
			returned_scan_document_id,status,sent_by,received_by,received_at,note
		)
		SELECT $1,$2,COALESCE(MAX(dispatch_no),0)+1,'Chủ đầu tư','direct',$3,'received',$4,$4,now(),
			'Đã nhận scan giấy ký trong integration test'
		FROM contract_dispatches WHERE contract_id=$1
	`, contractID, versionID, documentID, actorID)
}

func TestBOQUnitPriceValidation(t *testing.T) {
	expectedPrice, comment, err := normalizeBOQUnitPriceRequest(BOQUnitPriceRequest{UnitPrice: 0, Comment: "  draft expected  "}, false)
	if err != nil {
		t.Fatal(err)
	}
	if expectedPrice != 0 || comment != "draft expected" {
		t.Fatalf("expected normalized expected price/comment, got %v %q", expectedPrice, comment)
	}
	_, _, err = normalizeBOQUnitPriceRequest(BOQUnitPriceRequest{UnitPrice: 0}, true)
	assertValidationError(t, err)
	normalized, err := normalizeBOQItemRequest("project-id", Record{"category": "A", "work_item_name": "Item", "unit": "m2", "quantity": "2.5", "unit_price": "100", "status": "approved", "approved_by": "client-owned"}, true)
	if err != nil {
		t.Fatal(err)
	}
	if normalized["status"] != "draft" || normalized["approved_by"] != nil {
		t.Fatalf("create BOQ server-owned fields = %#v, want draft status without approved_by", normalized)
	}
	normalized, err = normalizeBOQItemRequest("", Record{"work_item_name": "Updated", "status": "approved", "approved_by": "client-owned"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if normalized["status"] != nil || normalized["approved_by"] != nil {
		t.Fatalf("update BOQ accepted server-owned fields: %#v", normalized)
	}
	_, err = normalizeBOQItemRequest("project-id", Record{"category": "A", "work_item_name": "Item", "unit": "m2", "quantity": -1}, true)
	assertValidationError(t, err)
}

func TestBatch7ProcurementTransitionsAreExplicit(t *testing.T) {
	tests := []struct {
		name       string
		action     string
		status     string
		wantStatus string
		wantEvent  string
		wantErr    bool
	}{
		{name: "submit draft", action: "submit", status: "draft", wantStatus: "waiting_approval", wantEvent: "review_requested"},
		{name: "approve waiting", action: "approve", status: "waiting_approval", wantStatus: "approved", wantEvent: "material_approved"},
		{name: "order approved", action: "order", status: "approved", wantStatus: "ordered", wantEvent: "material_ordered"},
		{name: "receive ordered", action: "receive", status: "ordered", wantStatus: "received", wantEvent: "procurement_acceptance_reconciled"},
		{name: "block skipped order", action: "order", status: "draft", wantErr: true},
		{name: "block skipped receive", action: "receive", status: "approved", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotStatus, gotEvent, err := procurementTransitionForAction(tt.action, tt.status)
			if tt.wantErr {
				assertValidationError(t, err)
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if gotStatus != tt.wantStatus || gotEvent != tt.wantEvent {
				t.Fatalf("transition = %s/%s, want %s/%s", gotStatus, gotEvent, tt.wantStatus, tt.wantEvent)
			}
		})
	}
}

func TestClientWorkflowAutomationRecordsBlockedBOQOnce(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	fixture := setupClientOfficialWorkflowFixture(t, db, 150000000)
	workItemID := clientOfficialWorkItemIDByNodeName(t, db, fixture.projectID, "BOQ (Có thiết kế)")
	mustExec(t, db, `UPDATE work_items SET status='completed' WHERE id=$1`, workItemID)

	assertValidationError(t, repo.RunRequiredWorkflowAutomationForWorkItem(ctx, workItemID, fixture.actorID, "Client Actor"))
	assertValidationError(t, repo.RunRequiredWorkflowAutomationForWorkItem(ctx, workItemID, fixture.actorID, "Client Actor"))
	assertCount(t, db, `
		SELECT COUNT(*)
		FROM workflow_automation_events
		WHERE work_item_id=$1
			AND action_key='procurement.generate_from_boq'
			AND status='blocked'
			AND blocker_reason='Chưa có dòng BOQ để sinh yêu cầu mua'
			AND run_count=2
	`, 1, workItemID)
	assertCount(t, db, `
		SELECT COUNT(*)
		FROM work_item_activities
		WHERE work_item_id=$1
			AND activity_type='blocker'
			AND metadata_json->>'automation_action'='procurement.generate_from_boq'
	`, 1, workItemID)
}

func TestClientWorkflowAutomationGeneratesProcurementIdempotently(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	fixture := setupClientOfficialWorkflowFixture(t, db, 150000000)
	workItemID := clientOfficialWorkItemIDByNodeName(t, db, fixture.projectID, "BOQ (Có thiết kế)")
	boqID := newID()
	mustExec(t, db, `
		INSERT INTO boq_items (
			id, project_id, category, work_item_name, unit, quantity, unit_price,
			status, created_by, approved_by, expected_unit_price, norm_unit_price,
			final_unit_price, unit_price_source, norm_price_locked, norm_price_locked_by,
			norm_price_locked_at, final_price_locked, final_price_locked_by,
			final_price_locked_at, price_revision_no, price_waiver_reason
		)
		VALUES (
			$1,$2,'Vật tư','Gạch lát','m2',10,100,
			'approved',$3,$3,100,100,
			120,'final',true,$3,
			now(),true,$3,
			now(),1,'UAT không áp dụng báo giá nhà cung cấp'
		)
	`, boqID, fixture.projectID, fixture.actorID)
	mustExec(t, db, `
		INSERT INTO boq_unit_price_events (project_id, boq_item_id, event_type, price_source, to_unit_price, revision_no, actor_id, actor_name)
		VALUES ($1,$2,'final_locked','final',120,1,$3,'Client Actor')
	`, fixture.projectID, boqID, fixture.actorID)
	mustExec(t, db, `UPDATE work_items SET status='completed' WHERE id=$1`, workItemID)

	if err := repo.RunRequiredWorkflowAutomationForWorkItem(ctx, workItemID, fixture.actorID, "Client Actor"); err != nil {
		t.Fatal(err)
	}
	if err := repo.RunRequiredWorkflowAutomationForWorkItem(ctx, workItemID, fixture.actorID, "Client Actor"); err != nil {
		t.Fatal(err)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM procurement_requests WHERE project_id=$1 AND status='waiting_approval'`, 1, fixture.projectID)
	assertCount(t, db, `SELECT COUNT(*) FROM procurement_request_items WHERE boq_item_id=$1`, 1, boqID)
	assertCount(t, db, `
		SELECT COUNT(*)
		FROM workflow_automation_events
		WHERE work_item_id=$1
			AND action_key='procurement.generate_from_boq'
			AND status='succeeded'
			AND target_type='procurement_request'
			AND run_count=2
	`, 1, workItemID)
	assertCount(t, db, `
		SELECT COUNT(*)
		FROM work_item_activities
		WHERE work_item_id=$1
			AND activity_type='system_event'
			AND metadata_json->>'automation_action'='procurement.generate_from_boq'
	`, 1, workItemID)
}

func TestBOQLifecycleHardeningAndProcurementLink(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")

	actorID := newID()
	projectID := newID()
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM projects WHERE id=$1`, projectID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id=$1`, actorID)
	})
	mustExec(t, db, `INSERT INTO users (id, email, password_hash, full_name, status) VALUES ($1,$2,'demo:pw','BOQ Actor','active')`, actorID, "boq-lifecycle-"+actorID+"@test.local")
	mustExec(t, db, `INSERT INTO projects (id, name, code, client_name, status) VALUES ($1,'BOQ Lifecycle',$2,'Client','planning')`, projectID, "BQL-"+projectID[:8])

	draft, err := service.CreateBOQ(ctx, projectID, Record{
		"category": "A", "work_item_name": "Server draft", "unit": "m2", "quantity": 2, "unit_price": 10,
		"status": "approved", "approved_by": actorID,
	}, actorID, "boq@test.local")
	if err != nil {
		t.Fatal(err)
	}
	if draft["status"] != "draft" || draft["approved_by"] != nil {
		t.Fatalf("created BOQ = %#v, want server-owned draft", draft)
	}
	draftID := cleanID(draft["id"])
	second, err := service.CreateBOQ(ctx, projectID, Record{
		"category": "A", "work_item_name": "Revised row", "unit": "m", "quantity": 3, "unit_price": 20,
	}, actorID, "boq@test.local")
	if err != nil {
		t.Fatal(err)
	}
	secondID := cleanID(second["id"])
	mustExec(t, db, `UPDATE boq_items SET status='revised' WHERE id=$1`, secondID)
	_, err = service.LockBOQNormUnitPrice(ctx, draftID, BOQUnitPriceRequest{UnitPrice: 11, Comment: "must be submitted"}, actorID, "boq@test.local")
	assertValidationError(t, err)

	updated, err := service.UpdateBOQ(ctx, draftID, Record{"work_item_name": "Updated draft", "status": "approved"}, actorID, "boq@test.local")
	if err != nil {
		t.Fatal(err)
	}
	if updated["status"] != "draft" || updated["work_item_name"] != "Updated draft" {
		t.Fatalf("updated draft = %#v", updated)
	}
	submitted, err := service.BOQProjectAction(ctx, projectID, "submit", actorID, "boq@test.local")
	if err != nil {
		t.Fatal(err)
	}
	if number(submitted["item_count"]) != 2 {
		t.Fatalf("submit result = %#v, want both draft and revised rows", submitted)
	}
	_, err = service.UpdateBOQ(ctx, draftID, Record{"work_item_name": "Bypass submitted"}, actorID, "boq@test.local")
	assertValidationError(t, err)
	_, err = service.BOQProjectAction(ctx, projectID, "approve", actorID, "boq@test.local")
	assertValidationError(t, err)
	if _, err := service.LockBOQNormUnitPrice(ctx, draftID, BOQUnitPriceRequest{UnitPrice: 11, Comment: "norm"}, actorID, "boq@test.local"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.LockBOQNormUnitPrice(ctx, secondID, BOQUnitPriceRequest{UnitPrice: 21, Comment: "norm"}, actorID, "boq@test.local"); err != nil {
		t.Fatal(err)
	}
	_, err = service.LockBOQFinalUnitPrice(ctx, draftID, BOQUnitPriceRequest{UnitPrice: 12, Comment: "must still be approved"}, actorID, "boq@test.local")
	assertValidationError(t, err)
	approved, err := service.BOQProjectAction(ctx, projectID, "approve", actorID, "boq@test.local")
	if err != nil {
		t.Fatal(err)
	}
	if number(approved["item_count"]) != 2 {
		t.Fatalf("approve result = %#v, want two norm-locked submitted rows", approved)
	}
	_, err = service.ReviseBOQ(ctx, draftID, BOQReviseRequest{}, actorID, "boq@test.local")
	assertValidationError(t, err)
	revised, err := service.ReviseBOQ(ctx, draftID, BOQReviseRequest{Reason: "  Bổ sung diễn giải theo hiện trường  "}, actorID, "boq@test.local")
	if err != nil {
		t.Fatal(err)
	}
	if revised["status"] != "revised" || revised["approved_by"] != nil || !boolValue(revised["norm_price_locked"]) {
		t.Fatalf("revised approved BOQ = %#v, want revised, cleared approver and retained norm lock", revised)
	}
	revised, err = service.UpdateBOQ(ctx, draftID, Record{"description": "Bổ sung diễn giải sau khi yêu cầu chỉnh sửa"}, actorID, "boq@test.local")
	if err != nil {
		t.Fatal(err)
	}
	if revised["description"] != "Bổ sung diễn giải sau khi yêu cầu chỉnh sửa" || number(revised["price_revision_no"]) != 2 {
		t.Fatalf("metadata-only revision = %#v, want description update without price revision change", revised)
	}
	_, err = service.UpdateBOQ(ctx, draftID, Record{"unit": "kg"}, actorID, "boq@test.local")
	assertValidationError(t, err)
	_, err = service.UpdateBOQ(ctx, draftID, Record{"quantity": 4}, actorID, "boq@test.local")
	assertValidationError(t, err)
	assertCount(t, db, `SELECT COUNT(*) FROM audit_logs WHERE project_id=$1 AND event_type='boq_revision_requested' AND target_id=$2 AND metadata_json->>'from_status'='approved' AND metadata_json->>'reason'='Bổ sung diễn giải theo hiện trường'`, 1, projectID, draftID)

	if _, err := service.BOQProjectAction(ctx, projectID, "submit", actorID, "boq@test.local"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.BOQProjectAction(ctx, projectID, "approve", actorID, "boq@test.local"); err != nil {
		t.Fatal(err)
	}
	requestID := newID()
	mustExec(t, db, `INSERT INTO procurement_requests (id, project_id, request_code, title, status) VALUES ($1,$2,$3,'Linked request','waiting_approval')`, requestID, projectID, "PR-LINK-"+requestID[:8])
	mustExec(t, db, `INSERT INTO procurement_request_items (request_id, boq_item_id, quantity_planned, quantity_requested, unit, estimated_unit_price) VALUES ($1,$2,2,2,'m2',11)`, requestID, draftID)
	detail, err := service.GetBOQ(ctx, draftID)
	if err != nil {
		t.Fatal(err)
	}
	if cleanID(detail["procurement_request_id"]) != requestID || detail["procurement_request_code"] != "PR-LINK-"+requestID[:8] || detail["procurement_request_status"] != "waiting_approval" || number(detail["active_procurement_count"]) != 1 || !boolValue(detail["has_active_procurement"]) {
		t.Fatalf("BOQ procurement link = %#v", detail)
	}
	list, err := service.ProjectBOQ(ctx, projectID)
	if err != nil {
		t.Fatal(err)
	}
	var linked Record
	for _, row := range list {
		if cleanID(row["id"]) == draftID {
			linked = row
		}
	}
	if linked == nil || cleanID(linked["procurement_request_id"]) != requestID || number(linked["active_procurement_count"]) != 1 {
		t.Fatalf("BOQ list missing active procurement link: %#v", linked)
	}
	_, err = service.ReviseBOQ(ctx, draftID, BOQReviseRequest{Reason: "Should be blocked by procurement"}, actorID, "boq@test.local")
	assertValidationError(t, err)

	if _, err := service.LockBOQFinalUnitPrice(ctx, secondID, BOQUnitPriceRequest{UnitPrice: 22, Comment: "manual waiver"}, actorID, "boq@test.local"); err != nil {
		t.Fatal(err)
	}
	_, err = service.ReviseBOQ(ctx, secondID, BOQReviseRequest{Reason: "Should be blocked by final lock"}, actorID, "boq@test.local")
	assertValidationError(t, err)
}

func TestBOQRecordRoutesEnforceScopedPermissions(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	repo := NewRepository(db)
	handler := NewHandler(NewService(repo, "test-secret"))
	fixture := setupProjectScopeFixture(t, db, []string{"boq.view"})
	boqID := newID()
	mustExec(t, db, `INSERT INTO boq_items (id, project_id, category, work_item_name, unit, quantity, unit_price, status, created_by) VALUES ($1,$2,'A','Scoped BOQ','m2',1,10,'submitted',$3)`, boqID, fixture.projectID, fixture.userID)
	app := batch4TestApp(fixture.userID, func(app *fiber.App, h *Handler) {
		app.Get("/boq-items/:id", h.GetBOQ)
		app.Post("/boq-items/:id/revise", h.ReviseBOQ)
	}, handler)

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/boq-items/"+boqID, nil))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET BOQ status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	req := httptest.NewRequest(http.MethodPost, "/boq-items/"+boqID+"/revise", strings.NewReader(`{"reason":"Need correction"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err = app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("revise BOQ status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

func TestGenerateProcurementFromBOQRouteEnforcesScopedPermissions(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")
	handler := NewHandler(service)
	fixture := setupProjectScopeFixture(t, db, []string{"boq.view"})
	app := batch4TestApp(fixture.userID, func(app *fiber.App, h *Handler) {
		app.Post("/projects/:id/boq/generate-procurement", h.GenerateProcurementFromBOQ)
	}, handler)

	resp, err := app.Test(httptest.NewRequest(http.MethodPost, "/projects/"+fixture.projectID+"/boq/generate-procurement", nil))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("generate procurement status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}

	authorized := setupProjectScopeFixture(t, db, []string{"procurement.create"})
	app = batch4TestApp(authorized.userID, func(app *fiber.App, h *Handler) {
		app.Post("/projects/:id/boq/generate-procurement", h.GenerateProcurementFromBOQ)
	}, handler)
	resp, err = app.Test(httptest.NewRequest(http.MethodPost, "/projects/"+authorized.projectID+"/boq/generate-procurement", nil))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode == http.StatusForbidden {
		t.Fatal("generate procurement rejected a project-scoped procurement.create actor")
	}
}

func TestGenerateProcurementFromBOQRequiresFinalLockedApprovedItems(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)

	actorID := newID()
	projectID := newID()
	approvedBOQID := newID()
	submittedBOQID := newID()
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM projects WHERE id=$1`, projectID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id=$1`, actorID)
	})

	mustExec(t, db, `INSERT INTO users (id, email, password_hash, full_name, status) VALUES ($1,$2,'demo:pw','Test Actor','active')`, actorID, "ops-boq-"+actorID+"@test.local")
	mustExec(t, db, `
		INSERT INTO projects (id, name, code, client_name, status)
		VALUES ($1,'BOQ Project',$2,'Client','planning')
	`, projectID, "BOQ-"+projectID[:8])
	mustExec(t, db, `
		INSERT INTO boq_items (id, project_id, category, work_item_name, unit, quantity, unit_price, status, created_by)
		VALUES ($1,$2,'A','Approved item','m2',2,10,'submitted',$3),
		       ($4,$2,'A','Submitted item','m2',100,10,'submitted',$3)
	`, approvedBOQID, projectID, actorID, submittedBOQID)

	if _, err := repo.LockBOQNormUnitPrice(ctx, approvedBOQID, 11, "norm locked", actorID, "actor@test.local"); err != nil {
		t.Fatal(err)
	}
	mustExec(t, db, `UPDATE boq_items SET status='approved', approved_by=$2 WHERE id=$1`, approvedBOQID, actorID)
	_, err := repo.GenerateProcurementFromBOQ(ctx, projectID, actorID, "actor@test.local")
	assertValidationError(t, err)

	if _, err := repo.LockBOQFinalUnitPrice(ctx, approvedBOQID, 12, "final locked", actorID, "actor@test.local"); err != nil {
		t.Fatal(err)
	}

	procurement, err := repo.GenerateProcurementFromBOQ(ctx, projectID, actorID, "actor@test.local")
	if err != nil {
		t.Fatal(err)
	}
	requestID := procurement["id"]
	assertCount(t, db, `SELECT COUNT(*) FROM procurement_request_items WHERE request_id=$1`, 1, requestID)
	assertCount(t, db, `SELECT COUNT(*) FROM procurement_request_items WHERE request_id=$1 AND boq_item_id=$2`, 1, requestID, approvedBOQID)
	assertCount(t, db, `SELECT COUNT(*) FROM procurement_request_items WHERE request_id=$1 AND boq_item_id=$2`, 0, requestID, submittedBOQID)
	assertCount(t, db, `SELECT COUNT(*) FROM procurement_request_items WHERE request_id=$1 AND boq_item_id=$2 AND estimated_unit_price=12 AND boq_price_source='final' AND boq_price_revision_no=2 AND boq_price_event_id IS NOT NULL`, 1, requestID, approvedBOQID)
	detail, err := repo.GetBOQItemDetail(ctx, approvedBOQID)
	if err != nil {
		t.Fatal(err)
	}
	if cleanID(detail["procurement_request_id"]) != cleanID(requestID) || detail["procurement_request_status"] != "waiting_approval" || number(detail["active_procurement_count"]) != 1 || !boolValue(detail["has_active_procurement"]) {
		t.Fatalf("generated procurement link = %#v", detail)
	}
	if _, err := repo.GenerateProcurementFromBOQ(ctx, projectID, actorID, "actor@test.local"); err == nil {
		t.Fatal("expected duplicate procurement generation to report no eligible BOQ rows")
	}
	assertCount(t, db, `SELECT COUNT(*) FROM procurement_requests WHERE project_id=$1`, 1, projectID)
	assertCount(t, db, `SELECT COUNT(*) FROM procurement_request_items WHERE boq_item_id=$1`, 1, approvedBOQID)
	assertCount(t, db, `SELECT COUNT(*) FROM stock_movements WHERE project_id=$1`, 0, projectID)
	assertCount(t, db, `SELECT COUNT(*) FROM payables WHERE project_id=$1`, 0, projectID)
	assertCount(t, db, `SELECT COUNT(*) FROM receivables WHERE project_id=$1`, 0, projectID)
}

func TestSupplierQuotationSelectionLocksBOQAndSnapshotsProcurement(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")

	actorID := newID()
	projectID := newID()
	partnerID := newID()
	boqID := newID()
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM projects WHERE id=$1`, projectID)
		_, _ = db.ExecContext(ctx, `DELETE FROM partners WHERE id=$1`, partnerID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id=$1`, actorID)
	})

	mustExec(t, db, `INSERT INTO users (id, email, password_hash, full_name, status) VALUES ($1,$2,'demo:pw','Commercial Actor','active')`, actorID, "quote-"+actorID+"@test.local")
	mustExec(t, db, `INSERT INTO projects (id, name, code, client_name, status) VALUES ($1,'Quotation Project',$2,'Client','planning')`, projectID, "QT-"+projectID[:8])
	mustExec(t, db, `INSERT INTO partners (id, partner_code, name, partner_types, active) VALUES ($1,$2,'Qualified Supplier','["supplier"]'::jsonb,true)`, partnerID, "SUP-"+partnerID[:8])
	mustExec(t, db, `
		INSERT INTO boq_items (id, project_id, category, work_item_name, unit, quantity, unit_price, expected_unit_price, status, created_by)
		VALUES ($1,$2,'A','Concrete supply','m3',10,100,100,'submitted',$3)
	`, boqID, projectID, actorID)
	if _, err := repo.LockBOQNormUnitPrice(ctx, boqID, 105, "Approved norm", actorID, "commercial@test.local"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.BOQProjectAction(ctx, projectID, "approve", actorID, "commercial@test.local"); err != nil {
		t.Fatal(err)
	}

	quotation, err := service.SaveSupplierQuotation(ctx, projectID, "", SupplierQuotationRequest{
		QuotationCode:     "BG-TEST-001",
		SupplierPartnerID: partnerID,
		QuotationDate:     time.Now().Format("2006-01-02"),
		ValidUntil:        time.Now().AddDate(0, 1, 0).Format("2006-01-02"),
		Currency:          "VND",
		PaymentTermsDays:  30,
		Items: []SupplierQuotationItemRequest{{
			BOQItemID: boqID, Quantity: 10, UnitPrice: 102, ComplianceStatus: "compliant",
		}},
	}, actorID, "commercial@test.local")
	if err != nil {
		t.Fatal(err)
	}
	quotationID := cleanID(quotation["id"])
	quotation, err = service.SubmitSupplierQuotation(ctx, quotationID, actorID, "commercial@test.local")
	if err != nil {
		t.Fatal(err)
	}
	quotation, err = service.QualifySupplierQuotation(ctx, quotationID, SupplierQuotationQualificationRequest{
		TechnicalStatus: "compliant", CommercialStatus: "compliant", Comment: "Đạt yêu cầu kỹ thuật và thương mại",
	}, actorID, "commercial@test.local")
	if err != nil {
		t.Fatal(err)
	}
	items := quotation["items"].([]Record)
	if len(items) != 1 {
		t.Fatalf("quotation items = %d, want 1", len(items))
	}
	quotationItemID := cleanID(items[0]["id"])
	boq, err := service.SelectSupplierQuote(ctx, quotationItemID, SupplierQuoteSelectionRequest{DecisionReason: "Giá tốt nhất trong các hồ sơ đạt kỹ thuật"}, actorID, "commercial@test.local")
	if err != nil {
		t.Fatal(err)
	}
	if !boolValue(boq["final_price_locked"]) || number(boq["final_unit_price"]) != 102 || cleanID(boq["selected_quotation_item_id"]) != quotationItemID {
		t.Fatalf("selected BOQ snapshot = %#v", boq)
	}

	procurement, err := repo.GenerateProcurementFromBOQ(ctx, projectID, actorID, "commercial@test.local")
	if err != nil {
		t.Fatal(err)
	}
	requestID := cleanID(procurement["id"])
	assertCount(t, db, `SELECT COUNT(*) FROM procurement_requests WHERE id=$1 AND supplier_partner_id=$2 AND status='waiting_approval'`, 1, requestID, partnerID)
	assertCount(t, db, `SELECT COUNT(*) FROM procurement_request_items WHERE request_id=$1 AND supplier_quotation_item_id=$2 AND quoted_unit_price=102 AND supplier_partner_id=$3`, 1, requestID, quotationItemID, partnerID)
	assertCount(t, db, `SELECT COUNT(*) FROM audit_logs WHERE project_id=$1 AND event_type='supplier_quote_selected' AND target_id=$2`, 1, projectID, boqID)
}

func TestContractLifecycleCreatesVersionedObligationsVariationAndRetention(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")
	actorID := newID()
	projectID := newID()
	partnerID := newID()
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM projects WHERE id=$1`, projectID)
		_, _ = db.ExecContext(ctx, `DELETE FROM partners WHERE id=$1`, partnerID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id=$1`, actorID)
	})
	mustExec(t, db, `INSERT INTO users (id,email,password_hash,full_name,status) VALUES ($1,$2,'demo:pw','Contract Actor','active')`, actorID, "contract-"+actorID+"@test.local")
	mustExec(t, db, `INSERT INTO projects (id,name,code,client_name,status) VALUES ($1,'Contract Project',$2,'Client','planning')`, projectID, "CT-"+projectID[:8])
	mustExec(t, db, `INSERT INTO partners (id,partner_code,name,partner_types,active) VALUES ($1,$2,'Contract Client','["client"]'::jsonb,true)`, partnerID, "CLI-"+partnerID[:8])

	contract, err := service.SaveContract(ctx, projectID, "", ContractRequest{
		ContractCode: "HD-001", Title: "Hợp đồng thi công chính", ContractType: "main_client", Direction: "receivable",
		CounterpartyPartnerID: partnerID, Currency: "VND", OriginalAmount: 1000000, AdvancePercent: 10,
		RetentionPercent: 5, WarrantyDays: 0, EffectiveDate: time.Now().Format("2006-01-02"),
	}, actorID, "contract@test.local")
	if err != nil {
		t.Fatal(err)
	}
	contractID := cleanID(contract["id"])
	contract, err = service.CreateContractVersion(ctx, contractID, ContractVersionRequest{
		VersionLabel: "Hợp đồng gốc", Amount: 1000000, ScopeSummary: "Thi công hoàn thiện", CommercialTerms: Record{"currency": "VND"}, PaymentTerms: Record{"advance_percent": 10},
	}, actorID, "contract@test.local")
	if err != nil {
		t.Fatal(err)
	}
	versions := contract["versions"].([]Record)
	versionID := cleanID(versions[0]["id"])
	if _, err = service.ContractVersionAction(ctx, versionID, "submit", "Gửi review", actorID, "contract@test.local"); err != nil {
		t.Fatal(err)
	}
	if _, err = service.ContractVersionAction(ctx, versionID, "approve", "Đã kiểm tra pháp lý và thương mại", actorID, "contract@test.local"); err != nil {
		t.Fatal(err)
	}
	if _, err = service.ContractVersionAction(ctx, versionID, "sign", "Hai bên đã ký", actorID, "contract@test.local"); err != nil {
		t.Fatal(err)
	}
	seedContractExecutionReadiness(t, db, projectID, contractID, versionID, actorID)
	contract, err = service.ActivateContract(ctx, contractID, "Kích hoạt hợp đồng", actorID, "contract@test.local")
	if err != nil {
		t.Fatal(err)
	}
	if contract["status"] != "active" || number(contract["current_amount"]) != 1000000 {
		t.Fatalf("active contract = %#v", contract)
	}

	contract, err = service.CreateContractMilestone(ctx, contractID, ContractMilestoneRequest{
		MilestoneCode: "M01", MilestoneName: "Thanh toán đợt 1", SequenceNo: 1, MilestoneType: "progress",
		AmountBasis: "percentage", Percentage: 10, RetentionPercent: 5, PlannedDate: time.Now().Format("2006-01-02"), DueDays: 7,
	}, actorID, "contract@test.local")
	if err != nil {
		t.Fatal(err)
	}
	milestones := contract["milestones"].([]Record)
	milestoneID := cleanID(milestones[0]["id"])
	if _, err = service.ContractMilestoneAction(ctx, milestoneID, "eligible", "Đủ điều kiện theo tiến độ", actorID, "contract@test.local"); err != nil {
		t.Fatal(err)
	}
	contract, err = service.ContractMilestoneAction(ctx, milestoneID, "open-obligation", "Mở phải thu", actorID, "contract@test.local")
	if err != nil {
		t.Fatal(err)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM receivables WHERE project_id=$1 AND source_type='contract_milestone' AND source_id=$2 AND gross_amount=95000 AND remaining_amount=95000`, 1, projectID, milestoneID)
	assertCount(t, db, `SELECT COUNT(*) FROM contract_retentions WHERE contract_id=$1 AND source_milestone_id=$2 AND held_amount=5000 AND outstanding_amount=5000`, 1, contractID, milestoneID)

	contract, err = service.CreateContractVariation(ctx, contractID, ContractVariationRequest{
		VariationCode: "PL-01", Title: "Bổ sung phạm vi", VariationType: "mixed", Reason: "Chủ đầu tư yêu cầu bổ sung",
		AmountDelta: 100000, DaysDelta: 5, ScopeDelta: "Bổ sung khu vực sảnh",
	}, actorID, "contract@test.local")
	if err != nil {
		t.Fatal(err)
	}
	variations := contract["variations"].([]Record)
	variationID := cleanID(variations[0]["id"])
	if _, err = service.ContractVariationAction(ctx, variationID, "submit", "Gửi duyệt phụ lục", actorID, "contract@test.local"); err != nil {
		t.Fatal(err)
	}
	if _, err = service.ContractVariationAction(ctx, variationID, "qs-review", "QS xác nhận khối lượng", actorID, "contract@test.local"); err != nil {
		t.Fatal(err)
	}
	if _, err = service.ContractVariationAction(ctx, variationID, "purchase", "Mua hàng đã lấy giá", actorID, "contract@test.local"); err != nil {
		t.Fatal(err)
	}
	if _, err = service.ContractVariationAction(ctx, variationID, "approve", "Duyệt giá trị và tiến độ", actorID, "contract@test.local"); err != nil {
		t.Fatal(err)
	}
	contract, err = service.ContractVariationAction(ctx, variationID, "incorporate", "Tạo version phụ lục", actorID, "contract@test.local")
	if err != nil {
		t.Fatal(err)
	}
	versions = contract["versions"].([]Record)
	if number(versions[0]["amount"]) != 1100000 || versions[0]["status"] != "draft" || cleanID(versions[0]["source_variation_id"]) != variationID {
		t.Fatalf("variation version = %#v", versions[0])
	}
	newVersionID := cleanID(versions[0]["id"])
	if _, err = service.ContractVersionAction(ctx, newVersionID, "submit", "Gửi review V2", actorID, "contract@test.local"); err != nil {
		t.Fatal(err)
	}
	if _, err = service.ContractVersionAction(ctx, newVersionID, "approve", "Duyệt V2", actorID, "contract@test.local"); err != nil {
		t.Fatal(err)
	}
	if _, err = service.ContractVersionAction(ctx, newVersionID, "sign", "Ký V2", actorID, "contract@test.local"); err != nil {
		t.Fatal(err)
	}
	seedContractExecutionReadiness(t, db, projectID, contractID, newVersionID, actorID)
	if _, err = service.ActivateContract(ctx, contractID, "Kích hoạt V2", actorID, "contract@test.local"); err != nil {
		t.Fatal(err)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM projects WHERE id=$1 AND contract_amount=1100000`, 1, projectID)

	// Historical signatures/dispatches must not make a newer current version appear execution-ready.
	unreadyVersionID := newID()
	mustExec(t, db, `
		INSERT INTO contract_versions (
			id,contract_id,version_no,version_label,amount,scope_summary,definition_hash,status,created_by
		) VALUES ($1,$2,3,'V3 chưa đủ hồ sơ',1100000,'Kiểm thử readiness theo version',repeat('b',64),'signed',$3)
	`, unreadyVersionID, contractID, actorID)
	mustExec(t, db, `UPDATE contracts SET current_version_id=$2 WHERE id=$1`, contractID, unreadyVersionID)
	contract, err = repo.GetContractDetail(ctx, contractID)
	if err != nil {
		t.Fatal(err)
	}
	readiness := contract["execution_readiness"].(Record)
	if boolValue(readiness["app_signed"]) || boolValue(readiness["paper_dispatched"]) || boolValue(readiness["returned_scan_received"]) || !boolValue(readiness["contract_active"]) {
		t.Fatalf("current-version execution readiness = %#v", readiness)
	}
	mustExec(t, db, `UPDATE contracts SET current_version_id=$2 WHERE id=$1`, contractID, newVersionID)

	retentions := contract["retentions"].([]Record)
	retentionID := cleanID(retentions[0]["id"])
	if _, err = service.ReleaseContractRetention(ctx, retentionID, ContractRetentionReleaseRequest{Amount: 5000, DueDate: time.Now().Format("2006-01-02"), Comment: "Hết bảo hành"}, actorID, "contract@test.local"); err != nil {
		t.Fatal(err)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM contract_retentions WHERE id=$1 AND status='released' AND outstanding_amount=0`, 1, retentionID)
	assertCount(t, db, `SELECT COUNT(*) FROM receivables WHERE project_id=$1 AND source_type='contract_retention' AND source_id=$2 AND gross_amount=5000`, 1, projectID, retentionID)
	assertCount(t, db, `SELECT COUNT(*) FROM audit_logs WHERE project_id=$1 AND event_type='contract_variation_incorporated'`, 1, projectID)
}

func TestConstructionVariationProposalStartsAtQSAndTracesAction(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	service := NewService(NewRepository(db), "test-secret")
	fixture := setupWorkItemFlowFixture(t, db, false)
	contractID := newID()
	mustExec(t, db, `
		INSERT INTO contracts (
			id,project_id,contract_code,title,contract_type,direction,counterparty_name,
			currency,original_amount,current_amount,status,created_by,activated_by,activated_at
		) VALUES ($1,$2,'HD-THI-CONG','Hợp đồng chủ đầu tư','main_client','receivable','Chủ đầu tư','VND',1000000,1000000,'active',$3,$3,now())
	`, contractID, fixture.projectID, fixture.workerID)
	evidence, err := service.AddWorkItemEvidence(ctx, fixture.itemID, WorkItemEvidenceRequest{
		EvidenceType: "photo", Name: "Hiện trạng phát sinh", FileURL: "https://example.com/variation-source.jpg", Visibility: "project",
	}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	action, err := service.CreateConstructionAction(ctx, fixture.projectID, ConstructionActionRequest{
		WorkItemID: fixture.itemID, Title: "Thay đổi vật liệu hoàn thiện", Description: "Chủ đầu tư yêu cầu đổi chủng loại",
		Priority: "urgent", AssignedUserID: fixture.workerID, DueAt: "2026-08-10T17:00:00+07:00",
		ImpactScope: "budget", EvidenceID: cleanID(evidence["id"]),
	}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	proposal, err := service.ProposeConstructionVariation(ctx, fixture.projectID, ContractVariationRequest{
		VariationCode: "PS-001", Title: "Đổi vật liệu hoàn thiện", VariationType: "mixed",
		Reason: "Yêu cầu thay đổi tại hiện trường", AmountDelta: 250000, DaysDelta: 2,
		ScopeDelta: "Thay chủng loại vật liệu khu A", Priority: "urgent",
		SourceWorkItemID: fixture.itemID, SourceConstructionActionID: cleanID(action["id"]),
	}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(proposal["status"]) != "qs_review" || cleanID(proposal["source_construction_action_id"]) != cleanID(action["id"]) {
		t.Fatalf("construction variation proposal = %#v, want qs_review linked to action", proposal)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM contract_variation_events WHERE variation_id=$1 AND event_type IN ('contract_variation_created','contract_variation_submitted')`, 2, proposal["id"])
	assertCount(t, db, `SELECT COUNT(*) FROM audit_logs WHERE project_id=$1 AND target_id=$2 AND event_type='contract_variation_proposed'`, 1, fixture.projectID, proposal["id"])

	contextRow, err := service.ProjectConstructionVariationContext(ctx, fixture.projectID)
	if err != nil {
		t.Fatal(err)
	}
	if !boolValue(contextRow["available"]) || cleanID(recordFromAny(contextRow["contract"])["id"]) != contractID {
		t.Fatalf("construction variation context = %#v, want active client contract", contextRow)
	}
	variations := workflowAnswerRecordList(contextRow["variations"])
	if len(variations) != 1 || cleanID(variations[0]["source_construction_action_id"]) != cleanID(action["id"]) {
		t.Fatalf("construction variation context rows = %#v", variations)
	}
	if _, err := service.ProposeConstructionVariation(ctx, fixture.projectID, ContractVariationRequest{
		VariationCode: "PS-002", Title: "Đề xuất trùng", VariationType: "price", Reason: "Trùng nguồn",
		Priority: "normal", SourceWorkItemID: fixture.itemID, SourceConstructionActionID: cleanID(action["id"]),
	}, fixture.workerID, "Worker"); err == nil || !strings.Contains(err.Error(), "đã có một đề xuất") {
		t.Fatalf("duplicate action proposal error = %v, want duplicate validation", err)
	}
}

func TestSubcontractorChecklistFormValidatesEvidenceAndQuantities(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")
	fixture := setupWorkItemFlowFixture(t, db, false)

	rows, err := repo.WorkflowFormCatalog(ctx)
	if err != nil {
		t.Fatal(err)
	}
	foundCatalog := false
	for _, row := range rows {
		if row["template_code"] == "builtin_subcontractor_checklist" {
			foundCatalog = true
			if row["form_type"] != "checklist" || number(row["version_no"]) != 1 {
				t.Fatalf("unexpected subcontractor checklist catalog row: %#v", row)
			}
			if !containsString(stringSlice(row["required_field_keys"]), "checklist_items") {
				t.Fatalf("expected checklist_items to be required, got %#v", row["required_field_keys"])
			}
		}
	}
	if !foundCatalog {
		t.Fatal("expected subcontractor checklist in workflow form catalog")
	}

	var formVersionID string
	if err := db.QueryRowContext(ctx, `
		SELECT v.id
		FROM workflow_form_templates t
		JOIN workflow_form_versions v ON v.template_id=t.id
		WHERE t.code='builtin_subcontractor_checklist'
		  AND t.status='active'
		  AND v.status='published'
		ORDER BY v.version_no DESC
		LIMIT 1
	`).Scan(&formVersionID); err != nil {
		t.Fatal(err)
	}
	defaultFormVersionID, err := repo.DefaultWorkflowFormVersionID(ctx, "checklist", true, "Checklist thầu phụ")
	if err != nil {
		t.Fatal(err)
	}
	if defaultFormVersionID != formVersionID {
		t.Fatalf("default subcontractor checklist form version = %q, want %q", defaultFormVersionID, formVersionID)
	}

	evidence, err := service.AddWorkItemEvidence(ctx, fixture.itemID, WorkItemEvidenceRequest{
		EvidenceType: "checklist",
		Name:         "Subcontractor checklist evidence",
		Visibility:   "project",
	}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	evidenceID := fmt.Sprint(evidence["id"])
	instanceRows, err := repo.Query(ctx, `
		INSERT INTO workflow_form_instances (project_id, work_item_id, form_template_id, form_version_id, form_type, title, created_by)
		SELECT $1, $2, v.template_id, v.id, t.form_type, 'Subcontractor checklist form', $3
		FROM workflow_form_versions v
		JOIN workflow_form_templates t ON t.id=v.template_id
		WHERE v.id=$4
		RETURNING id
	`, fixture.projectID, fixture.itemID, fixture.workerID, formVersionID)
	if err != nil {
		t.Fatal(err)
	}
	instanceID := fmt.Sprint(instanceRows[0]["id"])

	blockedAnswers := Record{
		"check_date":                   "2026-07-10",
		"subcontractor_name":           "Đội trần A",
		"subcontractor_representative": "Anh B",
		"work_package":                 "ceiling",
		"construction_area":            "Tầng 2",
		"checklist_items": []any{
			map[string]any{
				"title":              "Lắp khung xương trần",
				"boq_item_ref":       "BOQ-C-01",
				"claimed_quantity":   "12.5",
				"confirmed_quantity": "10",
				"unit":               "m2",
				"status":             "needs_follow_up",
				"evidence_id":        evidenceID,
				"owner_response":     "adjust_quantity",
				"due_date":           "2026-07-11",
				"comment":            "Cần kiểm tra lại mép biên",
			},
		},
		"overall_comment": "Chưa đủ điều kiện submit",
		"confirmed":       true,
	}
	_, err = service.SubmitWorkflowForm(ctx, instanceID, WorkflowFormSaveRequest{Answers: blockedAnswers}, fixture.workerID, "Worker")
	assertValidationError(t, err)

	invalidQuantityAnswers := Record{
		"check_date":         "2026-07-10",
		"subcontractor_name": "Đội trần A",
		"work_package":       "ceiling",
		"checklist_items": []any{
			map[string]any{
				"title":              "Lắp khung xương trần",
				"claimed_quantity":   "-1",
				"confirmed_quantity": "10",
				"status":             "done",
			},
		},
		"confirmed": true,
	}
	_, err = service.SubmitWorkflowForm(ctx, instanceID, WorkflowFormSaveRequest{Answers: invalidQuantityAnswers}, fixture.workerID, "Worker")
	assertValidationError(t, err)

	invalidEvidenceAnswers := Record{
		"check_date":         "2026-07-10",
		"subcontractor_name": "Đội trần A",
		"work_package":       "ceiling",
		"checklist_items": []any{
			map[string]any{
				"title":       "Lắp khung xương trần",
				"status":      "done",
				"evidence_id": newID(),
			},
		},
		"confirmed": true,
	}
	_, err = service.SubmitWorkflowForm(ctx, instanceID, WorkflowFormSaveRequest{Answers: invalidEvidenceAnswers}, fixture.workerID, "Worker")
	assertValidationError(t, err)

	readyAnswers := Record{
		"check_date":                   "2026-07-10",
		"subcontractor_name":           "Đội trần A",
		"subcontractor_representative": "Anh B",
		"work_package":                 "ceiling",
		"construction_area":            "Tầng 2",
		"checklist_items": []any{
			map[string]any{
				"title":              "Lắp khung xương trần",
				"boq_item_ref":       "BOQ-C-01",
				"claimed_quantity":   "12.5",
				"confirmed_quantity": "12",
				"unit":               "m2",
				"status":             "done",
				"evidence_id":        evidenceID,
				"owner_response":     "accepted",
				"comment":            "Đã đối chiếu evidence",
			},
		},
		"overall_comment": "Đủ điều kiện submit",
		"confirmed":       true,
	}
	form, err := service.SubmitWorkflowForm(ctx, instanceID, WorkflowFormSaveRequest{Answers: readyAnswers}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	instance, _ := form["instance"].(Record)
	if instance["status"] != "submitted" {
		t.Fatalf("form status = %#v, want submitted", instance["status"])
	}
	assertCount(t, db, `SELECT COUNT(*) FROM workflow_form_answers WHERE instance_id=$1 AND field_key='checklist_items' AND value_json->0->>'claimed_quantity'='12.5'`, 1, instanceID)
	assertCount(t, db, `SELECT COUNT(*) FROM workflow_form_events WHERE instance_id=$1 AND event_type='submitted'`, 1, instanceID)
	assertCount(t, db, `SELECT COUNT(*) FROM work_items WHERE id=$1 AND completion_payload_json->'checklist_items'->0->>'confirmed_quantity'='12'`, 1, fixture.itemID)
}

func TestSubcontractorChecklistReconciliationPersistsAuditAndValidatesReferences(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")
	fixture := setupWorkItemFlowFixture(t, db, false)

	boqItem, err := service.CreateBOQ(ctx, fixture.projectID, Record{
		"category":            "Trần",
		"work_item_name":      "BOQ-C-01 Lắp khung xương trần",
		"unit":                "m2",
		"quantity":            "12.5",
		"expected_unit_price": "100000",
		"status":              "approved",
	}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := service.AddWorkItemEvidence(ctx, fixture.itemID, WorkItemEvidenceRequest{
		EvidenceType: "checklist",
		Name:         "Subcontractor reconciliation evidence",
		Visibility:   "project",
	}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}

	var formVersionID string
	if err := db.QueryRowContext(ctx, `
		SELECT v.id
		FROM workflow_form_templates t
		JOIN workflow_form_versions v ON v.template_id=t.id
		WHERE t.code='builtin_subcontractor_checklist'
		  AND t.status='active'
		  AND v.status='published'
		ORDER BY v.version_no DESC
		LIMIT 1
	`).Scan(&formVersionID); err != nil {
		t.Fatal(err)
	}
	instanceRows, err := repo.Query(ctx, `
		INSERT INTO workflow_form_instances (project_id, work_item_id, form_template_id, form_version_id, form_type, title, created_by)
		SELECT $1, $2, v.template_id, v.id, t.form_type, 'Subcontractor checklist form', $3
		FROM workflow_form_versions v
		JOIN workflow_form_templates t ON t.id=v.template_id
		WHERE v.id=$4
		RETURNING id
	`, fixture.projectID, fixture.itemID, fixture.workerID, formVersionID)
	if err != nil {
		t.Fatal(err)
	}
	instanceID := fmt.Sprint(instanceRows[0]["id"])
	answers := Record{
		"check_date":                   "2026-07-10",
		"subcontractor_name":           "Đội trần A",
		"subcontractor_representative": "Anh B",
		"work_package":                 "ceiling",
		"construction_area":            "Tầng 2",
		"checklist_items": []any{
			map[string]any{
				"title":              "Lắp khung xương trần",
				"boq_item_ref":       "BOQ-C-01",
				"claimed_quantity":   "12.5",
				"confirmed_quantity": "10",
				"unit":               "m2",
				"status":             "done",
				"evidence_id":        fmt.Sprint(evidence["id"]),
				"owner_response":     "adjust_quantity",
				"comment":            "Đội thầu phụ claim vượt KL nội bộ",
			},
		},
		"overall_comment": "Chuyển đối soát nội bộ",
		"confirmed":       true,
	}
	if _, err := service.SubmitWorkflowForm(ctx, instanceID, WorkflowFormSaveRequest{Answers: answers}, fixture.workerID, "Worker"); err != nil {
		t.Fatal(err)
	}

	draft, err := service.SubcontractorReconciliation(ctx, fixture.itemID, fixture.workerID)
	if err != nil {
		t.Fatal(err)
	}
	if draft["available"] != true {
		t.Fatalf("reconciliation available = %#v, want true", draft["available"])
	}
	reconciliation, _ := draft["reconciliation"].(Record)
	sourceItems, _ := reconciliation["items"].([]Record)
	if len(sourceItems) != 1 || sourceItems[0]["decision"] != "adjust_quantity" {
		t.Fatalf("unexpected default reconciliation items: %#v", sourceItems)
	}

	confirmed := 9.0
	_, err = service.SaveSubcontractorReconciliation(ctx, fixture.itemID, SubcontractorReconciliationRequest{
		Items: []SubcontractorReconciliationItemRequest{{
			ItemIndex:         0,
			BOQItemID:         newID(),
			ConfirmedQuantity: &confirmed,
			Decision:          "adjust_quantity",
		}},
	}, fixture.workerID, "Worker")
	assertValidationError(t, err)

	submitted, err := service.SubmitSubcontractorReconciliation(ctx, fixture.itemID, SubcontractorReconciliationRequest{
		Comment: "Chốt KL nội bộ 9m2",
		Items: []SubcontractorReconciliationItemRequest{{
			ItemIndex:         0,
			BOQItemID:         fmt.Sprint(boqItem["id"]),
			ConfirmedQuantity: &confirmed,
			Decision:          "adjust_quantity",
			Comment:           "Cắt phần claim chưa đủ evidence",
		}},
	}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	submittedRecord, _ := submitted["reconciliation"].(Record)
	if submittedRecord["status"] != "submitted" {
		t.Fatalf("reconciliation status = %#v, want submitted", submittedRecord["status"])
	}

	var status, decision, mappedBOQID string
	var claimedQuantity, confirmedQuantity, varianceQuantity float64
	if err := db.QueryRowContext(ctx, `
		SELECT scr.status, scri.claimed_quantity, scri.confirmed_quantity, scri.variance_quantity, scri.decision, scri.boq_item_id::text
		FROM subcontractor_checklist_reconciliations scr
		JOIN subcontractor_checklist_reconciliation_items scri ON scri.reconciliation_id=scr.id
		WHERE scr.work_item_id=$1
	`, fixture.itemID).Scan(&status, &claimedQuantity, &confirmedQuantity, &varianceQuantity, &decision, &mappedBOQID); err != nil {
		t.Fatal(err)
	}
	if status != "submitted" || decision != "adjust_quantity" || mappedBOQID != fmt.Sprint(boqItem["id"]) {
		t.Fatalf("unexpected reconciliation row: status=%s decision=%s boq=%s", status, decision, mappedBOQID)
	}
	if claimedQuantity != 12.5 || confirmedQuantity != 9 || varianceQuantity != -3.5 {
		t.Fatalf("unexpected quantities: claimed=%v confirmed=%v variance=%v", claimedQuantity, confirmedQuantity, varianceQuantity)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM workflow_form_answers WHERE instance_id=$1 AND field_key='checklist_items' AND value_json->0->>'confirmed_quantity'='10'`, 1, instanceID)
	assertCount(t, db, `SELECT COUNT(*) FROM work_item_activities WHERE work_item_id=$1 AND activity_type='system_event' AND metadata_json->>'status'='submitted'`, 1, fixture.itemID)
	assertCount(t, db, `SELECT COUNT(*) FROM audit_logs WHERE project_id=$1 AND event_type='subcontractor_reconciliation_submitted'`, 1, fixture.projectID)

	_, err = service.SaveSubcontractorReconciliation(ctx, fixture.itemID, SubcontractorReconciliationRequest{
		Items: []SubcontractorReconciliationItemRequest{{
			ItemIndex: 0,
			Decision:  "accepted",
		}},
	}, fixture.workerID, "Worker")
	assertValidationError(t, err)
}

func TestOwnerClientChecklistFormValidatesEvidenceAndSubmitGate(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")
	fixture := setupWorkItemFlowFixture(t, db, false)

	rows, err := repo.WorkflowFormCatalog(ctx)
	if err != nil {
		t.Fatal(err)
	}
	foundCatalog := false
	for _, row := range rows {
		if row["template_code"] == "builtin_owner_client_checklist" {
			foundCatalog = true
			if row["form_type"] != "checklist" || number(row["version_no"]) != 1 {
				t.Fatalf("unexpected owner/client checklist catalog row: %#v", row)
			}
			if !containsString(stringSlice(row["required_field_keys"]), "checklist_items") {
				t.Fatalf("expected checklist_items to be required, got %#v", row["required_field_keys"])
			}
		}
	}
	if !foundCatalog {
		t.Fatal("expected owner/client checklist in workflow form catalog")
	}

	var formVersionID string
	if err := db.QueryRowContext(ctx, `
		SELECT v.id
		FROM workflow_form_templates t
		JOIN workflow_form_versions v ON v.template_id=t.id
		WHERE t.code='builtin_owner_client_checklist'
		  AND t.status='active'
		  AND v.status='published'
		ORDER BY v.version_no DESC
		LIMIT 1
	`).Scan(&formVersionID); err != nil {
		t.Fatal(err)
	}
	defaultFormVersionID, err := repo.DefaultWorkflowFormVersionID(ctx, "checklist", true, "Checklist chủ đầu tư")
	if err != nil {
		t.Fatal(err)
	}
	if defaultFormVersionID != formVersionID {
		t.Fatalf("default owner/client checklist form version = %q, want %q", defaultFormVersionID, formVersionID)
	}

	evidence, err := service.AddWorkItemEvidence(ctx, fixture.itemID, WorkItemEvidenceRequest{
		EvidenceType: "checklist",
		Name:         "Owner checklist evidence",
		Visibility:   "client",
	}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	evidenceID := fmt.Sprint(evidence["id"])
	instanceRows, err := repo.Query(ctx, `
		INSERT INTO workflow_form_instances (project_id, work_item_id, form_template_id, form_version_id, form_type, title, created_by)
		SELECT $1, $2, v.template_id, v.id, t.form_type, 'Owner client checklist form', $3
		FROM workflow_form_versions v
		JOIN workflow_form_templates t ON t.id=v.template_id
		WHERE v.id=$4
		RETURNING id
	`, fixture.projectID, fixture.itemID, fixture.workerID, formVersionID)
	if err != nil {
		t.Fatal(err)
	}
	instanceID := fmt.Sprint(instanceRows[0]["id"])

	blockedAnswers := Record{
		"check_date":             "2026-07-12",
		"client_representative":  "Client A",
		"company_representative": "Supervisor B",
		"handover_scope":         "area",
		"scope_detail":           "Tầng 2",
		"checklist_items": []any{
			map[string]any{
				"title":           "Kiểm tra hoàn thiện sơn",
				"output_ref":      "ACC-01",
				"status":          "needs_follow_up",
				"client_decision": "conditional_pass",
				"evidence_id":     evidenceID,
				"severity":        "medium",
				"required_action": "Bổ sung vá lỗi mép tường",
				"due_date":        "2026-07-13",
				"comment":         "Cần xử lý trước bàn giao",
			},
		},
		"final_client_note": "Chưa đủ điều kiện submit",
		"confirmed":         true,
	}
	_, err = service.SubmitWorkflowForm(ctx, instanceID, WorkflowFormSaveRequest{Answers: blockedAnswers}, fixture.workerID, "Worker")
	assertValidationError(t, err)

	invalidEvidenceAnswers := Record{
		"check_date":            "2026-07-12",
		"client_representative": "Client A",
		"handover_scope":        "area",
		"checklist_items": []any{
			map[string]any{
				"title":           "Kiểm tra hoàn thiện sơn",
				"status":          "done",
				"client_decision": "pass",
				"evidence_id":     newID(),
			},
		},
		"confirmed": true,
	}
	_, err = service.SubmitWorkflowForm(ctx, instanceID, WorkflowFormSaveRequest{Answers: invalidEvidenceAnswers}, fixture.workerID, "Worker")
	assertValidationError(t, err)

	readyAnswers := Record{
		"check_date":             "2026-07-12",
		"client_representative":  "Client A",
		"company_representative": "Supervisor B",
		"handover_scope":         "area",
		"scope_detail":           "Tầng 2",
		"checklist_items": []any{
			map[string]any{
				"title":           "Kiểm tra hoàn thiện sơn",
				"output_ref":      "ACC-01",
				"status":          "done",
				"client_decision": "pass",
				"evidence_id":     evidenceID,
				"comment":         "Đạt theo biên bản",
			},
		},
		"final_client_note": "Đủ điều kiện xác nhận",
		"confirmed":         true,
	}
	form, err := service.SubmitWorkflowForm(ctx, instanceID, WorkflowFormSaveRequest{Answers: readyAnswers}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	instance, _ := form["instance"].(Record)
	if instance["status"] != "submitted" {
		t.Fatalf("form status = %#v, want submitted", instance["status"])
	}
	assertCount(t, db, `SELECT COUNT(*) FROM workflow_form_answers WHERE instance_id=$1 AND field_key='checklist_items' AND value_json->0->>'client_decision'='pass'`, 1, instanceID)
	assertCount(t, db, `SELECT COUNT(*) FROM workflow_form_events WHERE instance_id=$1 AND event_type='submitted'`, 1, instanceID)
	assertCount(t, db, `SELECT COUNT(*) FROM work_items WHERE id=$1 AND completion_payload_json->'checklist_items'->0->>'client_decision'='pass'`, 1, fixture.itemID)
}

func TestOwnerClientChecklistReconciliationPersistsAuditAndValidatesAcceptance(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")
	fixture := setupWorkItemFlowFixture(t, db, false)

	acceptanceID := newID()
	mustExec(t, db, `
		INSERT INTO acceptance_records (id, project_id, type, title, requested_by, approved_by, status, result, notes)
		VALUES ($1,$2,'final','Final acceptance tầng 2',$3,$3,'completed','pass','Nội bộ nghiệm thu đạt')
	`, acceptanceID, fixture.projectID, fixture.workerID)
	evidence, err := service.AddWorkItemEvidence(ctx, fixture.itemID, WorkItemEvidenceRequest{
		EvidenceType: "checklist",
		Name:         "Owner reconciliation evidence",
		Visibility:   "client",
	}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}

	var formVersionID string
	if err := db.QueryRowContext(ctx, `
		SELECT v.id
		FROM workflow_form_templates t
		JOIN workflow_form_versions v ON v.template_id=t.id
		WHERE t.code='builtin_owner_client_checklist'
		  AND t.status='active'
		  AND v.status='published'
		ORDER BY v.version_no DESC
		LIMIT 1
	`).Scan(&formVersionID); err != nil {
		t.Fatal(err)
	}
	instanceRows, err := repo.Query(ctx, `
		INSERT INTO workflow_form_instances (project_id, work_item_id, form_template_id, form_version_id, form_type, title, created_by)
		SELECT $1, $2, v.template_id, v.id, t.form_type, 'Owner client checklist form', $3
		FROM workflow_form_versions v
		JOIN workflow_form_templates t ON t.id=v.template_id
		WHERE v.id=$4
		RETURNING id
	`, fixture.projectID, fixture.itemID, fixture.workerID, formVersionID)
	if err != nil {
		t.Fatal(err)
	}
	instanceID := fmt.Sprint(instanceRows[0]["id"])
	answers := Record{
		"check_date":             "2026-07-12",
		"client_representative":  "Client A",
		"company_representative": "Supervisor B",
		"handover_scope":         "area",
		"scope_detail":           "Tầng 2",
		"checklist_items": []any{
			map[string]any{
				"title":           "Kiểm tra hoàn thiện sơn",
				"output_ref":      "ACC-01",
				"status":          "done",
				"client_decision": "pass",
				"evidence_id":     fmt.Sprint(evidence["id"]),
				"comment":         "Đạt theo biên bản",
			},
		},
		"final_client_note": "Đủ điều kiện xác nhận",
		"confirmed":         true,
	}
	if _, err := service.SubmitWorkflowForm(ctx, instanceID, WorkflowFormSaveRequest{Answers: answers}, fixture.workerID, "Worker"); err != nil {
		t.Fatal(err)
	}

	draft, err := service.OwnerClientReconciliation(ctx, fixture.itemID, fixture.workerID)
	if err != nil {
		t.Fatal(err)
	}
	if draft["available"] != true {
		t.Fatalf("reconciliation available = %#v, want true", draft["available"])
	}
	reconciliation, _ := draft["reconciliation"].(Record)
	sourceItems, _ := reconciliation["items"].([]Record)
	if len(sourceItems) != 1 || sourceItems[0]["decision"] != "accepted" {
		t.Fatalf("unexpected default owner reconciliation items: %#v", sourceItems)
	}

	saved, err := service.SaveOwnerClientReconciliation(ctx, fixture.itemID, OwnerClientReconciliationRequest{
		Comment: "Draft trước khi link nghiệm thu",
		Items: []OwnerClientReconciliationItemRequest{{
			ItemIndex: 0,
			Decision:  "accepted",
		}},
	}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	savedRecord, _ := saved["reconciliation"].(Record)
	if savedRecord["status"] != "draft" {
		t.Fatalf("draft reconciliation status = %#v, want draft", savedRecord["status"])
	}

	_, err = service.SubmitOwnerClientReconciliation(ctx, fixture.itemID, OwnerClientReconciliationRequest{
		Items: []OwnerClientReconciliationItemRequest{{
			ItemIndex: 0,
			Decision:  "accepted",
		}},
	}, fixture.workerID, "Worker")
	assertValidationError(t, err)

	_, err = service.SubmitOwnerClientReconciliation(ctx, fixture.itemID, OwnerClientReconciliationRequest{
		Items: []OwnerClientReconciliationItemRequest{{
			ItemIndex:    0,
			AcceptanceID: newID(),
			Decision:     "accepted",
		}},
	}, fixture.workerID, "Worker")
	assertValidationError(t, err)

	submitted, err := service.SubmitOwnerClientReconciliation(ctx, fixture.itemID, OwnerClientReconciliationRequest{
		Comment: "Chốt theo final acceptance",
		Items: []OwnerClientReconciliationItemRequest{{
			ItemIndex:    0,
			AcceptanceID: acceptanceID,
			Decision:     "accepted",
			Comment:      "Đạt theo nghiệm thu nội bộ",
		}},
	}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	submittedRecord, _ := submitted["reconciliation"].(Record)
	if submittedRecord["status"] != "submitted" {
		t.Fatalf("reconciliation status = %#v, want submitted", submittedRecord["status"])
	}

	var status, decision, mappedAcceptanceID, clientDecision, acceptanceResult string
	if err := db.QueryRowContext(ctx, `
		SELECT ocr.status, ocri.decision, ocri.acceptance_id::text, ocri.client_decision, ocri.acceptance_result
		FROM owner_client_checklist_reconciliations ocr
		JOIN owner_client_checklist_reconciliation_items ocri ON ocri.reconciliation_id=ocr.id
		WHERE ocr.work_item_id=$1
	`, fixture.itemID).Scan(&status, &decision, &mappedAcceptanceID, &clientDecision, &acceptanceResult); err != nil {
		t.Fatal(err)
	}
	if status != "submitted" || decision != "accepted" || mappedAcceptanceID != acceptanceID || clientDecision != "pass" || acceptanceResult != "pass" {
		t.Fatalf("unexpected owner reconciliation row: status=%s decision=%s acceptance=%s client=%s result=%s", status, decision, mappedAcceptanceID, clientDecision, acceptanceResult)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM workflow_form_answers WHERE instance_id=$1 AND field_key='checklist_items' AND value_json->0->>'client_decision'='pass'`, 1, instanceID)
	assertCount(t, db, `SELECT COUNT(*) FROM work_item_activities WHERE work_item_id=$1 AND activity_type='system_event' AND metadata_json->>'status'='submitted'`, 1, fixture.itemID)
	assertCount(t, db, `SELECT COUNT(*) FROM audit_logs WHERE project_id=$1 AND event_type='owner_client_reconciliation_submitted'`, 1, fixture.projectID)

	_, err = service.SaveOwnerClientReconciliation(ctx, fixture.itemID, OwnerClientReconciliationRequest{
		Items: []OwnerClientReconciliationItemRequest{{
			ItemIndex: 0,
			Decision:  "not_applicable",
		}},
	}, fixture.workerID, "Worker")
	assertValidationError(t, err)
}
