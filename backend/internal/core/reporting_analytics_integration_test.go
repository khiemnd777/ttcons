package core

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
)

func TestNormalizeReportOptionsRequest(t *testing.T) {
	selectedID := "30000000-0000-0000-0000-000000000002"
	normalized, cursor, err := normalizeReportOptionsRequest(ReportOptionsRequest{
		Search:     "  Thủ Đức  ",
		SelectedID: selectedID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if normalized.Search != "Thủ Đức" || normalized.Limit != 20 || normalized.SelectedID != selectedID || cursor != nil {
		t.Fatalf("normalized report options = %#v cursor=%#v", normalized, cursor)
	}

	if _, _, err := normalizeReportOptionsRequest(ReportOptionsRequest{Limit: 51}); err == nil {
		t.Fatal("expected limit validation error")
	}
	if _, _, err := normalizeReportOptionsRequest(ReportOptionsRequest{SelectedID: "not-a-uuid"}); err == nil {
		t.Fatal("expected selected_id validation error")
	}
}

func TestReportProjectOptionsCursorRoundTrip(t *testing.T) {
	want := reportProjectOptionsCursor{
		NameKey: "cải tạo nhà phố thủ đức",
		CodeKey: "ct-td-001",
		ID:      "30000000-0000-0000-0000-000000000002",
	}
	encoded := encodeReportProjectOptionsCursor(want)
	got, err := parseReportProjectOptionsCursor(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("cursor = %#v, want %#v", got, want)
	}
	if _, err := parseReportProjectOptionsCursor("invalid"); err == nil {
		t.Fatal("expected invalid cursor error")
	}
}

func TestNormalizeDashboardPeriodRequest(t *testing.T) {
	now := time.Date(2026, time.August, 18, 9, 30, 0, 0, time.FixedZone("ICT", 7*60*60))

	month, monthMeta, err := normalizeDashboardPeriodRequestAt(DashboardPeriodRequest{}, now)
	if err != nil {
		t.Fatal(err)
	}
	if month.PeriodType != "month" || month.Year != 2026 || month.PeriodNo != 8 || monthMeta.FromDate != "2026-08-01" || monthMeta.ToDate != "2026-08-18" || monthMeta.GroupBy != "day" || !monthMeta.IsCurrent || monthMeta.CanMoveForward {
		t.Fatalf("current month normalization = request %#v meta %#v", month, monthMeta)
	}

	quarter, quarterMeta, err := normalizeDashboardPeriodRequestAt(DashboardPeriodRequest{PeriodType: "quarter", Year: 2026, PeriodNo: 2}, now)
	if err != nil {
		t.Fatal(err)
	}
	if quarter.PeriodNo != 2 || quarterMeta.FromDate != "2026-04-01" || quarterMeta.ToDate != "2026-06-30" || quarterMeta.GroupBy != "month" || quarterMeta.IsCurrent || !quarterMeta.CanMoveForward {
		t.Fatalf("quarter normalization = request %#v meta %#v", quarter, quarterMeta)
	}

	_, leapMeta, err := normalizeDashboardPeriodRequestAt(DashboardPeriodRequest{PeriodType: "month", Year: 2024, PeriodNo: 2}, now)
	if err != nil {
		t.Fatal(err)
	}
	if leapMeta.ToDate != "2024-02-29" {
		t.Fatalf("leap month to = %s, want 2024-02-29", leapMeta.ToDate)
	}

	if _, _, err := normalizeDashboardPeriodRequestAt(DashboardPeriodRequest{PeriodType: "month", Year: 2026, PeriodNo: 9}, now); err == nil {
		t.Fatal("expected future dashboard period validation error")
	}
}

func TestDashboardProjectsCursorBindsFilters(t *testing.T) {
	request := DashboardProjectsRequest{
		DashboardPeriodRequest: DashboardPeriodRequest{PeriodType: "month", Year: 2026, PeriodNo: 8},
		Search:                 "  Nhà phố  ",
		Sort:                   "progress",
		Direction:              "desc",
		Limit:                  25,
	}
	meta := DashboardPeriodMeta{PeriodType: "month", Year: 2026, PeriodNo: 8, FromDate: "2026-08-01", ToDate: "2026-08-18"}
	normalized, offset, filterHash, err := normalizeDashboardProjectsRequest(request, meta)
	if err != nil {
		t.Fatal(err)
	}
	if normalized.Search != "Nhà phố" || offset != 0 || len(filterHash) != sha256.Size*2 {
		t.Fatalf("normalized dashboard projects = %#v offset=%d hash=%q", normalized, offset, filterHash)
	}
	normalized.Cursor = encodeDashboardProjectsCursor(dashboardProjectsCursor{Version: 1, Offset: 25, FilterHash: filterHash})
	_, offset, _, err = normalizeDashboardProjectsRequest(normalized, meta)
	if err != nil || offset != 25 {
		t.Fatalf("dashboard cursor offset=%d err=%v", offset, err)
	}
	normalized.Search = "Khác"
	if _, _, _, err := normalizeDashboardProjectsRequest(normalized, meta); err == nil {
		t.Fatal("expected dashboard cursor to fail after filters change")
	}
}

func TestOperationalReportCSVIncludesExactlyReturnedRows(t *testing.T) {
	payload, err := operationalReportCSV("projects", []Record{{
		"project_id":        "project-1",
		"name":              "CSV Project",
		"is_sample":         false,
		"status":            "in_progress",
		"progress_percent":  42,
		"open_issues":       2,
		"pending_approvals": 1,
	}})
	if err != nil {
		t.Fatal(err)
	}
	text := string(payload)
	if !strings.Contains(text, "project_id,name,is_sample,status") || !strings.Contains(text, "project-1,CSV Project,false,in_progress,42,2,1") {
		t.Fatalf("operational csv payload missing header or row: %s", text)
	}
}

func TestDashboardExposesOperationalSnapshotOnly(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	service := NewService(NewRepository(db), "test-secret")
	fixture := setupProjectScopeFixture(t, db, []string{"report.view"})

	dashboard, err := service.Dashboard(ctx, DashboardPeriodRequest{PeriodType: "month", Year: 2026, PeriodNo: 6}, fixture.userID)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(dashboard)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(payload), "cashflow") || strings.Contains(string(payload), "finance_visible") {
		t.Fatalf("dashboard leaked retired finance fields: %s", payload)
	}
	if dashboard.ProjectSnapshot.TotalProjects != 1 || dashboard.Meta.FromDate != "2026-06-01" || dashboard.Meta.ToDate != "2026-06-30" {
		t.Fatalf("dashboard snapshot/meta = %#v", dashboard)
	}

	page, err := service.DashboardProjects(ctx, DashboardProjectsRequest{
		DashboardPeriodRequest: DashboardPeriodRequest{PeriodType: "month", Year: 2026, PeriodNo: 6},
	}, fixture.userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || page.Items[0].ID != fixture.projectID {
		t.Fatalf("dashboard project scope mismatch: %#v", page.Items)
	}
	payload, err = json.Marshal(page)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"finance_visible", "inflow_amount", "outflow_amount", "net_amount"} {
		if strings.Contains(string(payload), key) {
			t.Fatalf("dashboard project response leaked %s: %s", key, payload)
		}
	}
}

func TestDashboardProjectsPreservesScopeAndCursor(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	service := NewService(NewRepository(db), "test-secret")
	fixture := setupProjectScopeFixture(t, db, []string{"report.view"})
	extraProjectID := newID()
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM project_members WHERE project_id=$1`, extraProjectID)
		_, _ = db.ExecContext(ctx, `DELETE FROM projects WHERE id=$1`, extraProjectID)
	})
	mustExec(t, db, `UPDATE projects SET name='Dashboard Alpha', progress_percent=42 WHERE id=$1`, fixture.projectID)
	mustExec(t, db, `INSERT INTO projects (id,name,code,client_name,status,progress_percent) VALUES ($1,'Dashboard Beta',$2,'Client','in_progress',75)`, extraProjectID, "DBB-"+extraProjectID[:8])
	mustExec(t, db, `INSERT INTO project_members (project_id,staff_id,role_ids,active) VALUES ($1,$2,'["Member"]'::jsonb,true)`, extraProjectID, fixture.staffID)

	request := DashboardProjectsRequest{
		DashboardPeriodRequest: DashboardPeriodRequest{PeriodType: "month", Year: 2026, PeriodNo: 6},
		Search:                 "Dashboard",
		Sort:                   "name",
		Direction:              "asc",
		Limit:                  1,
	}
	first, err := service.DashboardProjects(ctx, request, fixture.userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Items) != 1 || first.Items[0].ID != fixture.projectID || first.Items[0].ProgressPercent != 42 || !first.HasMore || first.NextCursor == "" || first.Total != 2 {
		t.Fatalf("dashboard projects first page = %#v", first)
	}
	request.Cursor = first.NextCursor
	second, err := service.DashboardProjects(ctx, request, fixture.userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Items) != 1 || second.Items[0].ID != extraProjectID || second.HasMore || second.Total != 2 {
		t.Fatalf("dashboard projects second page = %#v", second)
	}
}

func TestOperationalReportsAggregateWithoutJoinMultiplicationAndRespectScope(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")
	fixture := setupProjectScopeFixture(t, db, []string{"report.view"})
	suffix := newID()
	materialID := newID()
	warehouseID := newID()
	outsideStaffID := newID()
	outsideUserID := newID()

	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM stock_movements WHERE material_id=$1`, materialID)
		_, _ = db.ExecContext(ctx, `DELETE FROM site_material_balances WHERE material_id=$1`, materialID)
		_, _ = db.ExecContext(ctx, `DELETE FROM materials WHERE id=$1`, materialID)
		_, _ = db.ExecContext(ctx, `DELETE FROM warehouses WHERE id=$1`, warehouseID)
		_, _ = db.ExecContext(ctx, `DELETE FROM project_members WHERE staff_id=$1`, outsideStaffID)
		_, _ = db.ExecContext(ctx, `DELETE FROM staff WHERE id=$1`, outsideStaffID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id=$1`, outsideUserID)
	})

	mustExec(t, db, `
		INSERT INTO issues (id, project_id, title, status, reported_by)
		VALUES ($1,$2,'Issue one','open',$3),($4,$2,'Issue two','in_progress',$3)
	`, newID(), fixture.projectID, fixture.userID, newID())
	mustExec(t, db, `
		INSERT INTO approval_requests (id, target_type, project_id, requested_by, status, reason)
		VALUES ($1,'work_item',$2,$3,'pending','Approval one'),($4,'work_item',$2,$3,'pending','Approval two')
	`, newID(), fixture.projectID, fixture.userID, newID())

	projectRows, err := service.Report(ctx, "projects", OperationalReportRequest{}, fixture.userID)
	if err != nil {
		t.Fatal(err)
	}
	projectRow := recordByID(projectRows, "project_id", fixture.projectID)
	if projectRow == nil || number(projectRow["open_issues"]) != 2 || number(projectRow["pending_approvals"]) != 2 {
		t.Fatalf("project report row = %#v, want two issues and two approvals", projectRow)
	}

	mustExec(t, db, `INSERT INTO materials (id, code, name, unit, category, active) VALUES ($1,$2,'Report Material','m','Test',true)`, materialID, "MAT-RPT-"+suffix[:8])
	mustExec(t, db, `INSERT INTO warehouses (id, name, location, type, active) VALUES ($1,'Report Warehouse','Test','central',true)`, warehouseID)
	mustExec(t, db, `
		INSERT INTO stock_movements (id, movement_code, project_id, warehouse_id, material_id, movement_type, quantity, unit, created_by)
		VALUES ($1,$2,$3,$4,$5,'receive',10,'m',$6),($7,$8,$3,$4,$5,'receive',20,'m',$6)
	`, newID(), "SM-RPT-1-"+suffix[:8], fixture.projectID, warehouseID, materialID, fixture.userID, newID(), "SM-RPT-2-"+suffix[:8])
	mustExec(t, db, `
		INSERT INTO site_material_balances (id, project_id, material_id, surplus_quantity)
		VALUES ($1,$2,$3,7)
	`, newID(), fixture.projectID, materialID)

	warehouseRows, err := service.Report(ctx, "warehouse", OperationalReportRequest{ProjectID: fixture.projectID}, fixture.userID)
	if err != nil {
		t.Fatal(err)
	}
	warehouseRow := recordByID(warehouseRows, "material_id", materialID)
	if warehouseRow == nil || number(warehouseRow["stock_quantity"]) != 30 || number(warehouseRow["surplus_quantity"]) != 7 {
		t.Fatalf("warehouse report row = %#v, want stock/surplus 30/7", warehouseRow)
	}

	mustExec(t, db, `INSERT INTO users (id, email, password_hash, full_name, status) VALUES ($1,$2,'demo:pw','Outside Staff','active')`, outsideUserID, "outside-report-"+suffix+"@test.local")
	mustExec(t, db, `INSERT INTO staff (id, user_id, full_name, email, department, position, status) VALUES ($1,$2,'Outside Staff',$3,'Other','Worker','active')`, outsideStaffID, outsideUserID, "outside-report-staff-"+suffix+"@test.local")
	mustExec(t, db, `INSERT INTO project_members (project_id, staff_id, role_ids, active) VALUES ($1,$2,'["Worker"]'::jsonb,true)`, fixture.otherProjectID, outsideStaffID)

	staffRows, err := service.Report(ctx, "staff", OperationalReportRequest{}, fixture.userID)
	if err != nil {
		t.Fatal(err)
	}
	if recordByID(staffRows, "staff_id", outsideStaffID) != nil || recordByID(staffRows, "staff_id", fixture.staffID) == nil {
		t.Fatalf("staff report leaked out-of-scope staff or omitted scoped staff: %#v", staffRows)
	}

	extraScopedProjectID := newID()
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM project_members WHERE project_id=$1`, extraScopedProjectID)
		_, _ = db.ExecContext(ctx, `DELETE FROM projects WHERE id=$1`, extraScopedProjectID)
	})
	mustExec(t, db, `INSERT INTO projects (id, name, code, client_name, status) VALUES ($1,'Batch4 Scoped Z',$2,'Client','planning')`, extraScopedProjectID, "B4SZ-"+extraScopedProjectID[:8])
	mustExec(t, db, `INSERT INTO project_members (project_id, staff_id, role_ids, active) VALUES ($1,$2,'["Member"]'::jsonb,true)`, extraScopedProjectID, fixture.staffID)

	options, err := service.ReportOptions(ctx, ReportOptionsRequest{
		Search:     "batch4 scoped",
		Limit:      1,
		SelectedID: fixture.projectID,
	}, fixture.userID)
	if err != nil {
		t.Fatal(err)
	}
	projects := options.Projects.Items
	if len(projects) != 1 || projects[0].ID != fixture.projectID || projects[0].Code == "" || !options.Projects.HasMore || options.Projects.NextCursor == "" {
		t.Fatalf("report options scope/search = %#v", projects)
	}
	if options.Projects.Selected == nil || options.Projects.Selected.ID != fixture.projectID {
		t.Fatalf("report options selected = %#v", options.Projects.Selected)
	}
	nextOptions, err := service.ReportOptions(ctx, ReportOptionsRequest{
		Search: "batch4 scoped",
		Limit:  1,
		Cursor: options.Projects.NextCursor,
	}, fixture.userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(nextOptions.Projects.Items) != 1 || nextOptions.Projects.Items[0].ID != extraScopedProjectID || nextOptions.Projects.HasMore {
		t.Fatalf("report options next page = %#v", nextOptions.Projects)
	}

	outsideOptions, err := service.ReportOptions(ctx, ReportOptionsRequest{Search: "batch4 other"}, fixture.userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(outsideOptions.Projects.Items) != 0 {
		t.Fatalf("report options leaked out-of-scope project: %#v", outsideOptions.Projects.Items)
	}
}

func TestBatch4StockAggregateExcludesOutOfScopeProjectMovements(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	repo := NewRepository(db)
	handler := NewHandler(NewService(repo, "test-secret"))
	fixture := setupProjectScopeFixture(t, db, []string{"warehouse.view", "report.view"})
	materialID := newID()
	warehouseID := newID()
	mustExec(t, db, `INSERT INTO materials (id, code, name, unit, category, active) VALUES ($1,$2,'Batch4 Material','m','Test',true)`, materialID, "MAT-"+materialID[:8])
	mustExec(t, db, `INSERT INTO warehouses (id, name, location, type, active) VALUES ($1,'Batch4 Warehouse','Test','central',true)`, warehouseID)
	mustExec(t, db, `
		INSERT INTO stock_movements (movement_code, project_id, warehouse_id, material_id, movement_type, quantity, unit, reason, created_by, approved_by)
		VALUES ($1,NULL,$2,$3,'receive',100,'m','Central stock',$4,$4),
		       ($5,$6,$2,$3,'issue',10,'m','Scoped issue',$4,$4),
		       ($7,$8,$2,$3,'issue',70,'m','Out of scope issue',$4,$4)
	`, "SM-C-"+materialID[:6], warehouseID, materialID, fixture.userID, "SM-S-"+materialID[:6], fixture.projectID, "SM-O-"+materialID[:6], fixture.otherProjectID)
	app := batch4TestApp(fixture.userID, func(app *fiber.App, h *Handler) {
		app.Get("/stock", h.Stock)
	}, handler)

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/stock", nil))
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
	var found Record
	for _, row := range body.Data {
		if row["material_id"] == materialID {
			found = row
			break
		}
	}
	if found == nil {
		t.Fatal("expected batch4 material in stock response")
	}
	if got := number(found["stock_quantity"]); got != 90 {
		t.Fatalf("stock_quantity = %v, want 90", got)
	}
}

func TestDashboardRejectsRetiredFinanceSort(t *testing.T) {
	meta := DashboardPeriodMeta{PeriodType: "month", Year: 2026, PeriodNo: 8}
	if _, _, _, err := normalizeDashboardProjectsRequest(DashboardProjectsRequest{Sort: "net"}, meta); err == nil {
		t.Fatal("retired finance sort must be rejected")
	}
}

func TestRetiredFinancialReportRoutesAreUnavailable(t *testing.T) {
	app := fiber.New()
	registerReportingRoutes(app, &Handler{})
	for _, path := range []string{
		"/reports/cashflow", "/reports/cashflow/export",
		"/reports/tax", "/reports/tax/export",
		"/reports/profit", "/reports/profit/export",
		"/reports/accounting", "/reports/accounting/export",
	} {
		resp, err := app.Test(httptest.NewRequest(http.MethodGet, path, nil))
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("%s status=%d, want 404", path, resp.StatusCode)
		}
	}
}
