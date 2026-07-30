package core

import (
	"context"
	"testing"
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
