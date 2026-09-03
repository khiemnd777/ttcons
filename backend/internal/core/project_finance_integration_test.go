package core

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestProjectFinanceValidationRejectsIncompleteManagementRecords(t *testing.T) {
	service := NewService(nil, "test-secret")
	_, err := service.CreateProjectMaterialCost(context.Background(), newID(), ProjectMaterialCostRequest{
		VendorName:      "Nhà cung cấp",
		Description:     "Mua vật tư",
		AmountBeforeVAT: 100,
		VATAmount:       10,
		TotalAmount:     111,
	}, newID(), "Actor")
	assertValidationError(t, err)

	_, err = service.CreateProjectCashRequest(context.Background(), newID(), ProjectCashRequestRequest{
		Direction:        "payment",
		Title:            "Đề nghị",
		CounterpartyName: "Đối tác",
		Amount:           100,
	}, newID(), "Actor")
	assertValidationError(t, err)
}

func TestProjectFinanceProjectOptionsRespectScopePermissionAndCursor(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	fixture := setupProjectScopeFixture(t, db, []string{"project_finance.view"})
	service := NewService(NewRepository(db), "test-secret")
	extraProjectID := newID()
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM project_members WHERE project_id=$1`, extraProjectID)
		_, _ = db.ExecContext(ctx, `DELETE FROM projects WHERE id=$1`, extraProjectID)
	})
	mustExec(t, db, `UPDATE user_roles SET project_id=$1 WHERE user_id=$2 AND role_id=$3`, fixture.projectID, fixture.userID, fixture.roleID)
	mustExec(t, db, `INSERT INTO projects (id,name,code,client_name,status) VALUES ($1,'Batch4 Scoped Z',$2,'Client','planning')`, extraProjectID, "PFZ-"+extraProjectID[:8])
	mustExec(t, db, `INSERT INTO project_members (project_id,staff_id,role_ids,active) VALUES ($1,$2,'["Member"]'::jsonb,true)`, extraProjectID, fixture.staffID)

	filtered, err := service.ProjectFinanceProjects(ctx, ReportOptionsRequest{
		Search: "batch4 scoped", SelectedID: extraProjectID,
	}, fixture.userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(filtered.Projects.Items) != 1 || filtered.Projects.Items[0].ID != fixture.projectID {
		t.Fatalf("finance options permission filter = %#v", filtered.Projects.Items)
	}
	if filtered.Projects.Selected != nil {
		t.Fatalf("unauthorized selected project leaked: %#v", filtered.Projects.Selected)
	}

	mustExec(t, db, `INSERT INTO user_roles (user_id,role_id,project_id) VALUES ($1,$2,$3)`, fixture.userID, fixture.roleID, extraProjectID)
	first, err := service.ProjectFinanceProjects(ctx, ReportOptionsRequest{
		Search: "batch4 scoped", Limit: 1, SelectedID: fixture.projectID,
	}, fixture.userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Projects.Items) != 1 || first.Projects.Items[0].ID != fixture.projectID || !first.Projects.HasMore || first.Projects.NextCursor == "" {
		t.Fatalf("finance options first page = %#v", first.Projects)
	}
	second, err := service.ProjectFinanceProjects(ctx, ReportOptionsRequest{
		Search: "batch4 scoped", Limit: 1, Cursor: first.Projects.NextCursor,
	}, fixture.userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Projects.Items) != 1 || second.Projects.Items[0].ID != extraProjectID || second.Projects.HasMore {
		t.Fatalf("finance options second page = %#v", second.Projects)
	}

	handler := NewHandler(service)
	app := batch4TestApp(fixture.userID, func(app *fiber.App, h *Handler) {
		app.Get("/project-finance/projects", h.ProjectFinanceProjects)
	}, handler)
	response, err := app.Test(httptest.NewRequest(http.MethodGet, "/project-finance/projects?search=batch4+scoped&limit=1&selected_id="+fixture.projectID, nil))
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusOK)
	}
	var body struct {
		Success bool                  `json:"success"`
		Data    ReportOptionsResponse `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if !body.Success || len(body.Data.Projects.Items) != 1 || body.Data.Projects.Selected == nil {
		t.Fatalf("finance project options response = %#v", body)
	}
}

func TestProjectFinanceCostAndCashRequestLifecycle(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	fixture := setupProjectScopeFixture(t, db, []string{
		"project_finance.view", "project_finance.cost_create", "project_finance.cost_review",
		"project_finance.request_create", "project_finance.request_approve", "project_finance.process", "project_finance.report_view",
	})
	service := NewService(NewRepository(db), "test-secret")
	procurementID := newID()
	mustExec(t, db, `
		INSERT INTO procurement_requests (id,project_id,request_code,title,requested_by,status,total_estimated_amount)
		VALUES ($1,$2,$3,'Mua vật tư kiểm thử',$4,'ordered',110)
	`, procurementID, fixture.projectID, "PF-"+procurementID[:8], fixture.userID)

	cost, err := service.CreateProjectMaterialCost(ctx, fixture.projectID, ProjectMaterialCostRequest{
		ProcurementRequestID: procurementID,
		VendorName:           "Nhà cung cấp kiểm thử",
		InvoiceNumber:        "PF-INV-" + procurementID[:8],
		InvoiceDate:          "2026-07-30",
		Description:          "Mua vật tư kiểm thử",
		AmountBeforeVAT:      100,
		VATAmount:            10,
		TotalAmount:          110,
	}, fixture.userID, "Finance Actor")
	if err != nil {
		t.Fatal(err)
	}
	if cost["status"] != "draft" {
		t.Fatalf("cost status = %v, want draft", cost["status"])
	}
	costID := cleanID(cost["id"])
	if _, err := service.ProjectMaterialCostAction(ctx, costID, "submit", ProjectFinanceActionRequest{}, fixture.userID, "Finance Actor"); err != nil {
		t.Fatal(err)
	}
	confirmed, err := service.ProjectMaterialCostAction(ctx, costID, "confirm", ProjectFinanceActionRequest{}, fixture.userID, "Finance Actor")
	if err != nil {
		t.Fatal(err)
	}
	if confirmed["status"] != "confirmed" || number(confirmed["recognized_cost_amount"]) != 110 {
		t.Fatalf("confirmed cost = %#v", confirmed)
	}

	request, err := service.CreateProjectCashRequest(ctx, fixture.projectID, ProjectCashRequestRequest{
		Direction: "outbound", Title: "Đề nghị chi vật tư", CounterpartyName: "Nhà cung cấp kiểm thử", Amount: 110,
		SourceType: "procurement_request", SourceID: procurementID,
	}, fixture.userID, "Finance Actor")
	if err != nil {
		t.Fatal(err)
	}
	requestID := cleanID(request["id"])
	for _, action := range []string{"submit", "approve", "handoff", "process"} {
		request, err = service.ProjectCashRequestAction(ctx, requestID, action, ProjectFinanceActionRequest{}, fixture.userID, "Finance Actor")
		if err != nil {
			t.Fatalf("action %s: %v", action, err)
		}
	}
	if request["status"] != "processed" {
		t.Fatalf("request status = %v, want processed", request["status"])
	}

	summary, err := service.repo.ProjectFinanceSummary(ctx, fixture.projectID)
	if err != nil {
		t.Fatal(err)
	}
	if number(summary["confirmed_material_cost"]) != 110 || number(summary["provisional_profit"]) != -110 {
		t.Fatalf("summary = %#v", summary)
	}
	redacted, err := service.ProjectFinanceWorkspace(ctx, fixture.projectID, false)
	if err != nil {
		t.Fatal(err)
	}
	if redacted["can_view_report"] != false || redacted["summary"].(Record)["provisional_profit"] != nil {
		t.Fatalf("management report was not redacted: %#v", redacted)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM audit_logs WHERE project_id=$1 AND target_id IN ($2,$3)`, 8, fixture.projectID, costID, requestID)
}

func TestProjectFinanceRejectsSourceFromAnotherProject(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	fixture := setupProjectScopeFixture(t, db, []string{"project_finance.cost_create"})
	procurementID := newID()
	mustExec(t, db, `
		INSERT INTO procurement_requests (id,project_id,request_code,title,requested_by,status,total_estimated_amount)
		VALUES ($1,$2,$3,'Nguồn khác công trình',$4,'ordered',110)
	`, procurementID, fixture.otherProjectID, "PF-X-"+procurementID[:8], fixture.userID)
	_, err := NewService(NewRepository(db), "test-secret").CreateProjectMaterialCost(ctx, fixture.projectID, ProjectMaterialCostRequest{
		ProcurementRequestID: procurementID, VendorName: "Nhà cung cấp", Description: "Mua vật tư",
		AmountBeforeVAT: 100, VATAmount: 10, TotalAmount: 110,
	}, fixture.userID, "Finance Actor")
	assertValidationError(t, err)

	_, err = NewService(NewRepository(db), "test-secret").CreateProjectCashRequest(ctx, fixture.projectID, ProjectCashRequestRequest{
		Direction: "outbound", Title: "Đề nghị chi", CounterpartyName: "Nhà cung cấp", Amount: 110,
		SourceType: "procurement_request", SourceID: procurementID,
	}, fixture.userID, "Finance Actor")
	assertValidationError(t, err)
}

func TestAccountingHandoffUsesSourceReadinessAndAuditedLifecycle(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	fixture := setupProjectScopeFixture(t, db, []string{
		"accounting_handoff.view", "accounting_handoff.submit", "accounting_handoff.acknowledge",
	})
	service := NewService(NewRepository(db), "test-secret")
	partnerID := newID()
	procurementID := newID()
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM partners WHERE id=$1`, partnerID)
	})
	mustExec(t, db, `
		INSERT INTO partners (id,partner_code,name,partner_types,active)
		VALUES ($1,$2,'Nhà cung cấp bàn giao','["supplier"]'::jsonb,true)
	`, partnerID, "AH-"+partnerID[:8])
	mustExec(t, db, `
		INSERT INTO procurement_requests (
			id,project_id,request_code,title,requested_by,status,total_estimated_amount,
			supplier_partner_id,supplier_name,order_reference
		) VALUES ($1,$2,$3,'Mua vật tư bàn giao',$4,'ordered',2500000,$5,'Nhà cung cấp bàn giao','PO-001')
	`, procurementID, fixture.projectID, "AH-PR-"+procurementID[:8], fixture.userID, partnerID)

	workspace, err := service.ProjectFinanceWorkspace(ctx, fixture.projectID, false)
	if err != nil {
		t.Fatal(err)
	}
	if workspace["mode"] != "handoff" {
		t.Fatalf("workspace mode = %v, want handoff", workspace["mode"])
	}
	items := workspace["items"].([]Record)
	if len(items) != 1 || items[0]["status"] != "ready" || cleanID(items[0]["source_id"]) != procurementID {
		t.Fatalf("handoff candidates = %#v", items)
	}

	created, err := service.CreateAccountingHandoff(ctx, fixture.projectID, AccountingHandoffRequest{
		SourceType: "procurement_request", SourceID: procurementID, Note: "Đủ PO",
	}, fixture.userID, "Handoff Actor")
	if err != nil {
		t.Fatal(err)
	}
	handoffID := cleanID(created["id"])
	assertQueue := func(bucket string) {
		t.Helper()
		page, err := service.MyWorkItems(ctx, MyWorkItemsFilter{Unified: true, ProjectID: fixture.projectID, EntryKind: "accounting_handoff", Bucket: bucket}, fixture.userID)
		if err != nil {
			t.Fatal(err)
		}
		entries := page["entries"].([]Record)
		if len(entries) != 1 || cleanID(recordValue(entries[0]["item"])["handoff_id"]) != handoffID {
			t.Fatalf("canonical queue after transition: %#v", page)
		}
	}
	assertQueue("need_action")
	if created["status"] != "handed_off" {
		t.Fatalf("created handoff = %#v", created)
	}
	readiness, err := service.repo.AccountingHandoffCloseReadiness(ctx, fixture.projectID)
	if err != nil || boolValue(readiness["ready"]) || int(number(readiness["pending_handoffs"])) != 1 {
		t.Fatalf("pending close readiness = %#v, err=%v", readiness, err)
	}
	_, err = service.AccountingHandoffAction(ctx, handoffID, "return", AccountingHandoffActionRequest{}, fixture.userID, "Accounting Actor")
	assertValidationError(t, err)
	returned, err := service.AccountingHandoffAction(ctx, handoffID, "return", AccountingHandoffActionRequest{ReturnReason: "Thiếu biên bản giao nhận"}, fixture.userID, "Accounting Actor")
	if err != nil || returned["status"] != "returned" {
		t.Fatalf("returned handoff = %#v, err=%v", returned, err)
	}
	assertQueue("waiting_supplement")
	resent, err := service.AccountingHandoffAction(ctx, handoffID, "handoff", AccountingHandoffActionRequest{Note: "Đã bổ sung"}, fixture.userID, "Handoff Actor")
	if err != nil || resent["status"] != "handed_off" || resent["return_reason"] != nil {
		t.Fatalf("resent handoff = %#v, err=%v", resent, err)
	}
	assertQueue("need_action")
	acknowledged, err := service.AccountingHandoffAction(ctx, handoffID, "acknowledge", AccountingHandoffActionRequest{ExternalReference: "MISA-001"}, fixture.userID, "Accounting Actor")
	if err != nil || acknowledged["status"] != "acknowledged" || acknowledged["external_reference"] != "MISA-001" {
		t.Fatalf("acknowledged handoff = %#v, err=%v", acknowledged, err)
	}
	assertQueue("completed")
	handoverID := newID()
	mustExec(t, db, `
		INSERT INTO handover_records (id,project_id,handover_code,status,completed_at,created_by)
		VALUES ($1,$2,$3,'handed_over',now(),$4)
	`, handoverID, fixture.projectID, "AH-HO-"+handoverID[:8], fixture.userID)
	readiness, err = service.repo.AccountingHandoffCloseReadiness(ctx, fixture.projectID)
	if err != nil || !boolValue(readiness["ready"]) || int(number(readiness["acknowledged_count"])) != 1 {
		t.Fatalf("acknowledged close readiness = %#v, err=%v", readiness, err)
	}
	if _, err := service.CreateAccountingHandoff(ctx, fixture.projectID, AccountingHandoffRequest{SourceType: "procurement_request", SourceID: procurementID}, fixture.userID, "Handoff Actor"); err == nil {
		t.Fatal("duplicate source handoff was accepted")
	}
	assertCount(t, db, `SELECT COUNT(*) FROM audit_logs WHERE project_id=$1 AND target_type='accounting_handoff' AND target_id=$2`, 4, fixture.projectID, handoffID)
}

func TestAccountingHandoffRejectsProcurementBeforeOrderReadiness(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	fixture := setupProjectScopeFixture(t, db, []string{"accounting_handoff.submit"})
	procurementID := newID()
	mustExec(t, db, `
		INSERT INTO procurement_requests (id,project_id,request_code,title,requested_by,status,total_estimated_amount)
		VALUES ($1,$2,$3,'Yêu cầu chưa đặt',$4,'approved',100)
	`, procurementID, fixture.projectID, "AH-WAIT-"+procurementID[:8], fixture.userID)
	_, err := NewService(NewRepository(db), "test-secret").CreateAccountingHandoff(ctx, fixture.projectID, AccountingHandoffRequest{
		SourceType: "procurement_request", SourceID: procurementID,
	}, fixture.userID, "Handoff Actor")
	assertValidationError(t, err)
}

func TestAccountingReconciliationObserverSeesCompanyScopeButKeepsActionBoundaries(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	fixture := setupProjectScopeFixture(t, db, []string{
		"project.reconciliation.view",
		"project.view",
		"operation.view",
		"project_finance.view",
		"accounting_handoff.view",
		"accounting_handoff.acknowledge",
		"audit.view",
		"procurement.approve",
		"project.update",
	})
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")

	canViewOther, err := repo.CanAccessProjectPermission(ctx, fixture.userID, fixture.otherProjectID, []string{"project.view"})
	if err != nil || !canViewOther {
		t.Fatalf("observer project read access = %v, err=%v", canViewOther, err)
	}
	canAcknowledgeOther, err := repo.CanAccessProjectPermission(ctx, fixture.userID, fixture.otherProjectID, []string{"accounting_handoff.acknowledge"})
	if err != nil || !canAcknowledgeOther {
		t.Fatalf("observer Accounting action access = %v, err=%v", canAcknowledgeOther, err)
	}
	for permission, label := range map[string]string{
		"project.update":      "project mutation",
		"procurement.approve": "procurement approval",
	} {
		allowed, permissionErr := repo.CanAccessProjectPermission(ctx, fixture.userID, fixture.otherProjectID, []string{permission})
		if permissionErr != nil {
			t.Fatal(permissionErr)
		}
		if allowed {
			t.Fatalf("observer unexpectedly received %s outside project membership", label)
		}
	}
	handler := NewHandler(service)
	app := batch4TestApp(fixture.userID, func(app *fiber.App, h *Handler) {
		app.Get("/audit-logs", h.AuditLogs)
		app.Get("/projects/:id/audit-logs", h.ProjectAuditLogs)
	}, handler)
	for _, path := range []string{"/audit-logs", "/projects/" + fixture.projectID + "/audit-logs"} {
		response, requestErr := app.Test(httptest.NewRequest(http.MethodGet, path, nil))
		if requestErr != nil {
			t.Fatal(requestErr)
		}
		_ = response.Body.Close()
		if response.StatusCode != http.StatusForbidden {
			t.Fatalf("Accounting reconciliation Audit status for %s = %d, want %d", path, response.StatusCode, http.StatusForbidden)
		}
	}
	canPostActivity, err := repo.CanMutateProjectActivity(ctx, fixture.userID, fixture.otherProjectID)
	if err != nil || canPostActivity {
		t.Fatalf("observer activity mutation outside membership = %v, err=%v", canPostActivity, err)
	}
	canPostMemberActivity, err := repo.CanMutateProjectActivity(ctx, fixture.userID, fixture.projectID)
	if err != nil || !canPostMemberActivity {
		t.Fatalf("observer activity mutation inside membership = %v, err=%v", canPostMemberActivity, err)
	}

	handoffID := newID()
	sourceID := newID()
	mustExec(t, db, `
		INSERT INTO accounting_handoffs (
			id,project_id,direction,source_type,source_id,source_snapshot_json,status,handed_off_by
		) VALUES (
			$1,$2,'payable','procurement_request',$3,
			jsonb_build_object(
				'source_code',$4::text,'source_title','Hồ sơ đối soát ngoài phạm vi thành viên',
				'counterparty_name','Nhà cung cấp kiểm thử','reference_amount',2500000,'currency','VND'
			),
			'handed_off',$5
		)
	`, handoffID, fixture.otherProjectID, sourceID, "REC-"+sourceID[:8], fixture.userID)

	page, err := service.MyAccountingWork(ctx, MyAccountingWorkFilter{Status: "handed_off", Limit: 20}, fixture.userID)
	if err != nil {
		t.Fatal(err)
	}
	items, ok := page["items"].([]Record)
	if !ok || len(items) != 1 || cleanID(items[0]["project_id"]) != fixture.otherProjectID || cleanID(items[0]["handoff_id"]) != handoffID {
		t.Fatalf("company Accounting queue = %#v", page)
	}
	if int(number(page["total"])) != 1 || int(number(recordValue(page["summary"])["handed_off_count"])) != 1 {
		t.Fatalf("company Accounting queue summary = %#v", page)
	}
}
