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
