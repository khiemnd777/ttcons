package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"consflow/backend/internal/shared/migrations"

	"github.com/gofiber/fiber/v2"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestHasChecklistPayloadRequiresChecklistContent(t *testing.T) {
	tests := []struct {
		name    string
		payload Record
		want    bool
	}{
		{name: "empty payload", payload: Record{}, want: false},
		{name: "unrelated payload", payload: Record{"note": "done"}, want: false},
		{name: "false checklist flag", payload: Record{"checklist": false}, want: false},
		{name: "empty checklist list", payload: Record{"checklist": []any{}}, want: false},
		{name: "checklist list", payload: Record{"checklist": []any{Record{"label": "done", "checked": true}}}, want: true},
		{name: "checklist items", payload: Record{"checklist_items": []string{"done"}}, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasChecklistPayload(tt.payload); got != tt.want {
				t.Fatalf("hasChecklistPayload() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLegacyStepSubmitFillsChecklistPayload(t *testing.T) {
	item := Record{"evidence_requirement_json": map[string]any{"checklist_required": true}}
	req := withLegacyChecklistCompletion(item, WorkItemActionRequest{})

	if !hasChecklistPayload(req.CompletionPayload) {
		t.Fatal("expected legacy submit to satisfy checklist payload")
	}
	if _, ok := req.CompletionPayload["checklist"]; !ok {
		t.Fatalf("expected checklist key in payload, got %#v", req.CompletionPayload)
	}
}

func TestCanCompleteWorkItemStatus(t *testing.T) {
	allowed := []string{"ready", "in_progress", "needs_changes", "waiting_evidence"}
	for _, status := range allowed {
		t.Run("allows_"+status, func(t *testing.T) {
			if !canCompleteWorkItemStatus(status) {
				t.Fatalf("expected %s to be completable", status)
			}
		})
	}
	blocked := []string{"pending", "blocked", "waiting_approval", "waiting_review", "completed", "cancelled"}
	for _, status := range blocked {
		t.Run("blocks_"+status, func(t *testing.T) {
			if canCompleteWorkItemStatus(status) {
				t.Fatalf("expected %s to be blocked", status)
			}
		})
	}
}

func TestWorkItemTransitionForAction(t *testing.T) {
	tests := []struct {
		name         string
		action       string
		status       string
		startedAt    string
		wantStatus   string
		wantActivity string
		wantEvidence bool
		wantErr      bool
	}{
		{name: "request review", action: "request-review", status: "in_progress", wantStatus: "waiting_review", wantActivity: "review_requested", wantEvidence: true},
		{name: "request approval", action: "request-approval", status: "waiting_review", wantStatus: "waiting_approval", wantActivity: "approval_requested", wantEvidence: true},
		{name: "approve approval", action: "approve", status: "waiting_approval", wantStatus: "completed", wantActivity: "approved", wantEvidence: true},
		{name: "reject review", action: "reject", status: "waiting_review", wantStatus: "needs_changes", wantActivity: "rejected"},
		{name: "request changes", action: "request-change", status: "waiting_approval", wantStatus: "needs_changes", wantActivity: "changes_requested"},
		{name: "resolve blocker resumes started", action: "resolve-blocker", status: "blocked", startedAt: "2026-06-12T00:00:00Z", wantStatus: "in_progress", wantActivity: "blocker_resolved"},
		{name: "blocks invalid approve", action: "approve", status: "ready", wantErr: true},
		{name: "blocks invalid review", action: "request-review", status: "pending", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item := Record{"status": tt.status}
			if tt.startedAt != "" {
				item["started_at"] = tt.startedAt
			}
			got, err := workItemTransitionForAction(tt.action, item)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got.ToStatus != tt.wantStatus || got.ActivityType != tt.wantActivity {
				t.Fatalf("transition = %s/%s, want %s/%s", got.ToStatus, got.ActivityType, tt.wantStatus, tt.wantActivity)
			}
			if got.RequiresEvidence != tt.wantEvidence {
				t.Fatalf("RequiresEvidence = %v, want %v", got.RequiresEvidence, tt.wantEvidence)
			}
		})
	}
}

func TestProjectOperationsAccessPolicy(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)

	suffix := newID()
	projectID := newID()
	cancelledProjectID := newID()
	directorUserID := newID()
	pmUserID := newID()
	pmStaffID := newID()
	noScopeUserID := newID()
	projectRoleNoPermissionUserID := newID()

	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM projects WHERE id IN ($1,$2)`, projectID, cancelledProjectID)
		_, _ = db.ExecContext(ctx, `DELETE FROM staff WHERE id=$1`, pmStaffID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id IN ($1,$2,$3,$4)`, directorUserID, pmUserID, noScopeUserID, projectRoleNoPermissionUserID)
	})

	operationViewID := ensurePermission(t, db, "operation.view", "Xem thi công", "operation")
	directorRoleID := ensureRole(t, db, "director", "Giám đốc")
	pmRoleID := ensureRole(t, db, "pm", "PM")
	noPermissionRoleID := ensureRole(t, db, "batch2_viewer_"+suffix[:8], "Batch2 Viewer")
	mustExec(t, db, `INSERT INTO role_permissions (role_id, permission_id) VALUES ($1,$2),($3,$2) ON CONFLICT DO NOTHING`, directorRoleID, operationViewID, pmRoleID)

	mustExec(t, db, `INSERT INTO users (id, email, password_hash, full_name, status) VALUES ($1,$2,'demo:pw','Director','active')`, directorUserID, "director-"+suffix+"@test.local")
	mustExec(t, db, `INSERT INTO users (id, email, password_hash, full_name, status) VALUES ($1,$2,'demo:pw','PM','active')`, pmUserID, "pm-"+suffix+"@test.local")
	mustExec(t, db, `INSERT INTO users (id, email, password_hash, full_name, status) VALUES ($1,$2,'demo:pw','No Scope','active')`, noScopeUserID, "noscope-"+suffix+"@test.local")
	mustExec(t, db, `INSERT INTO users (id, email, password_hash, full_name, status) VALUES ($1,$2,'demo:pw','Project Role No Permission','active')`, projectRoleNoPermissionUserID, "project-role-no-perm-"+suffix+"@test.local")
	mustExec(t, db, `INSERT INTO staff (id, user_id, full_name, email, department, position, status) VALUES ($1,$2,'PM',$3,'Dự án','PM','active')`, pmStaffID, pmUserID, "pm-staff-"+suffix+"@test.local")
	mustExec(t, db, `INSERT INTO projects (id, name, code, client_name, status) VALUES ($1,'Ops Access',$2,'Client','planning'),($3,'Ops Cancelled',$4,'Client','cancelled')`, projectID, "OPA-"+suffix[:8], cancelledProjectID, "OPC-"+suffix[:8])
	mustExec(t, db, `INSERT INTO project_members (project_id, staff_id, role_ids, active) VALUES ($1,$2,'["PM"]'::jsonb,true)`, projectID, pmStaffID)
	mustExec(t, db, `INSERT INTO user_roles (user_id, role_id) VALUES ($1,$2),($3,$4),($5,$4)`, directorUserID, directorRoleID, pmUserID, pmRoleID, noScopeUserID)
	mustExec(t, db, `INSERT INTO user_roles (user_id, role_id, project_id) VALUES ($1,$2,$3)`, projectRoleNoPermissionUserID, noPermissionRoleID, projectID)

	assertCanViewProjectOperations(t, repo, ctx, projectID, directorUserID, true)
	assertCanViewProjectOperations(t, repo, ctx, projectID, pmUserID, true)
	assertCanViewProjectOperations(t, repo, ctx, projectID, noScopeUserID, false)
	assertCanViewProjectOperations(t, repo, ctx, projectID, projectRoleNoPermissionUserID, false)
	assertCanViewProjectOperations(t, repo, ctx, cancelledProjectID, directorUserID, false)
}

func TestBatch4ProjectListIsScopedByMembership(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	fixture := setupProjectScopeFixture(t, db, []string{"project.view"})

	rows, err := repo.ListProjects(ctx, fixture.userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected exactly one scoped project, got %d", len(rows))
	}
	if rows[0]["id"] != fixture.projectID {
		t.Fatalf("project id = %v, want %s", rows[0]["id"], fixture.projectID)
	}
}

func TestBatch4ProjectCreateRequiresModulePermission(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	handler := NewHandler(NewService(repo, "test-secret"))
	fixture := setupProjectScopeFixture(t, db, []string{"project.view", "boq.view"})
	allowed, err := repo.CanAccessProjectPermission(ctx, fixture.userID, fixture.projectID, []string{"boq.create"})
	if err != nil {
		t.Fatal(err)
	}
	if allowed {
		t.Fatal("expected direct RBAC helper to deny boq.create")
	}
	app := batch4TestApp(fixture.userID, func(app *fiber.App, h *Handler) {
		app.Post("/projects/:id/boq", h.CreateBOQ)
	}, handler)

	req := httptest.NewRequest(http.MethodPost, "/projects/"+fixture.projectID+"/boq", strings.NewReader(`{"category":"A","work_item_name":"Denied","unit":"m2","quantity":1,"unit_price":10}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

func TestBatch4AccountingListUsesProjectScope(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	repo := NewRepository(db)
	handler := NewHandler(NewService(repo, "test-secret"))
	fixture := setupProjectScopeFixture(t, db, []string{"accounting.view", "accounting.payable_view"})
	mustExec(t, db, `
		INSERT INTO payables (project_id, payable_code, vendor_type, vendor_name, source_type, gross_amount, remaining_amount, status, created_by)
		VALUES ($1,$2,'supplier','Scoped Vendor','manual',100,100,'approved',$3),
		       ($4,$5,'supplier','Other Vendor','manual',200,200,'approved',$3)
	`, fixture.projectID, "PAY-S-"+fixture.projectID[:8], fixture.userID, fixture.otherProjectID, "PAY-O-"+fixture.otherProjectID[:8])
	app := batch4TestApp(fixture.userID, func(app *fiber.App, h *Handler) {
		app.Get("/payables", h.ListPayables)
	}, handler)

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/payables", nil))
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

func openProjectOperationsTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("CONSFLOW_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set CONSFLOW_TEST_DATABASE_URL to run DB-backed project operations tests")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := migrations.Run(ctx, db, filepath.Join("..", "..", "migrations")); err != nil {
		t.Fatal(err)
	}
	return db
}

func ensureRole(t *testing.T, db *sql.DB, name, displayName string) string {
	t.Helper()
	var id string
	if err := db.QueryRowContext(context.Background(), `
		INSERT INTO roles (name, display_name, system_role)
		VALUES ($1,$2,true)
		ON CONFLICT (name) DO UPDATE SET name=EXCLUDED.name
		RETURNING id
	`, name, displayName).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func ensurePermission(t *testing.T, db *sql.DB, key, name, module string) string {
	t.Helper()
	var id string
	if err := db.QueryRowContext(context.Background(), `
		INSERT INTO permissions (key, name, module)
		VALUES ($1,$2,$3)
		ON CONFLICT (key) DO UPDATE SET key=EXCLUDED.key
		RETURNING id
	`, key, name, module).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func assertCanViewProjectOperations(t *testing.T, repo *Repository, ctx context.Context, projectID, userID string, want bool) {
	t.Helper()
	got, err := repo.CanViewProjectOperations(ctx, projectID, userID)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("CanViewProjectOperations(%s, %s) = %v, want %v", projectID, userID, got, want)
	}
}

type projectScopeFixture struct {
	userID         string
	staffID        string
	roleID         string
	projectID      string
	otherProjectID string
}

func setupProjectScopeFixture(t *testing.T, db *sql.DB, permissions []string) projectScopeFixture {
	t.Helper()
	ctx := context.Background()
	suffix := newID()
	userID := newID()
	staffID := newID()
	roleID := ensureRole(t, db, "batch4_scope_"+suffix[:8], "Batch4 Scope")
	projectID := newID()
	otherProjectID := newID()
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM project_members WHERE project_id IN ($1,$2)`, projectID, otherProjectID)
		_, _ = db.ExecContext(ctx, `DELETE FROM projects WHERE id IN ($1,$2)`, projectID, otherProjectID)
		_, _ = db.ExecContext(ctx, `DELETE FROM user_roles WHERE user_id=$1`, userID)
		_, _ = db.ExecContext(ctx, `DELETE FROM role_permissions WHERE role_id=$1`, roleID)
		_, _ = db.ExecContext(ctx, `DELETE FROM roles WHERE id=$1`, roleID)
		_, _ = db.ExecContext(ctx, `DELETE FROM staff WHERE id=$1`, staffID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id=$1`, userID)
	})
	mustExec(t, db, `INSERT INTO users (id, email, password_hash, full_name, status) VALUES ($1,$2,'demo:pw','Scoped User','active')`, userID, "batch4-"+suffix+"@test.local")
	mustExec(t, db, `INSERT INTO staff (id, user_id, full_name, email, department, position, status) VALUES ($1,$2,'Scoped User',$3,'Dự án','Member','active')`, staffID, userID, "batch4-staff-"+suffix+"@test.local")
	mustExec(t, db, `INSERT INTO projects (id, name, code, client_name, status) VALUES ($1,'Batch4 Scoped',$2,'Client','planning'),($3,'Batch4 Other',$4,'Client','planning')`, projectID, "B4S-"+suffix[:8], otherProjectID, "B4O-"+suffix[:8])
	mustExec(t, db, `INSERT INTO project_members (project_id, staff_id, role_ids, active) VALUES ($1,$2,'["Member"]'::jsonb,true)`, projectID, staffID)
	mustExec(t, db, `INSERT INTO user_roles (user_id, role_id) VALUES ($1,$2)`, userID, roleID)
	for _, permission := range permissions {
		permissionID := ensurePermission(t, db, permission, permission, "batch4")
		mustExec(t, db, `INSERT INTO role_permissions (role_id, permission_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`, roleID, permissionID)
	}
	return projectScopeFixture{userID: userID, staffID: staffID, roleID: roleID, projectID: projectID, otherProjectID: otherProjectID}
}

func batch4TestApp(userID string, register func(*fiber.App, *Handler), handler *Handler) *fiber.App {
	app := fiber.New(fiber.Config{ErrorHandler: ErrorHandler})
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user_id", userID)
		c.Locals("email", "batch4@test.local")
		return c.Next()
	})
	register(app, handler)
	return app
}

type workItemFlowFixture struct {
	projectID string
	workerID  string
	pmID      string
	itemID    string
}

func setupWorkItemFlowFixture(t *testing.T, db *sql.DB, documentRequired bool) workItemFlowFixture {
	t.Helper()
	ctx := context.Background()
	suffix := newID()
	projectID := newID()
	workerID := newID()
	workerStaffID := newID()
	pmID := newID()
	pmStaffID := newID()
	itemID := newID()
	sourceID := newID()

	operationViewID := ensurePermission(t, db, "operation.view", "Xem thi công", "operation")
	operationReportID := ensurePermission(t, db, "operation.report", "Tạo daily report", "operation")
	workerRoleID := ensureRole(t, db, "batch3_worker_"+suffix[:8], "Đội thi công")
	pmRoleID := ensureRole(t, db, "pm", "PM")
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM projects WHERE id=$1`, projectID)
		_, _ = db.ExecContext(ctx, `DELETE FROM staff WHERE id IN ($1,$2)`, workerStaffID, pmStaffID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id IN ($1,$2)`, workerID, pmID)
		_, _ = db.ExecContext(ctx, `DELETE FROM roles WHERE id=$1`, workerRoleID)
	})
	mustExec(t, db, `INSERT INTO role_permissions (role_id, permission_id) VALUES ($1,$2),($1,$3) ON CONFLICT DO NOTHING`, workerRoleID, operationViewID, operationReportID)

	mustExec(t, db, `INSERT INTO users (id, email, password_hash, full_name, status) VALUES ($1,$2,'demo:pw','Worker','active')`, workerID, "worker-"+suffix+"@test.local")
	mustExec(t, db, `INSERT INTO users (id, email, password_hash, full_name, status) VALUES ($1,$2,'demo:pw','PM','active')`, pmID, "pm-batch3-"+suffix+"@test.local")
	mustExec(t, db, `INSERT INTO staff (id, user_id, full_name, email, department, position, status) VALUES ($1,$2,'Worker',$3,'Thi công','Worker','active')`, workerStaffID, workerID, "worker-staff-"+suffix+"@test.local")
	mustExec(t, db, `INSERT INTO staff (id, user_id, full_name, email, department, position, status) VALUES ($1,$2,'PM',$3,'Dự án','PM','active')`, pmStaffID, pmID, "pm-staff-batch3-"+suffix+"@test.local")
	mustExec(t, db, `INSERT INTO projects (id, name, code, client_name, status) VALUES ($1,'Batch3 Ops',$2,'Client','planning')`, projectID, "B3-"+suffix[:8])
	mustExec(t, db, `INSERT INTO project_members (project_id, staff_id, role_ids, active) VALUES ($1,$2,'["Đội thi công"]'::jsonb,true),($1,$3,'["PM"]'::jsonb,true)`, projectID, workerStaffID, pmStaffID)
	mustExec(t, db, `INSERT INTO user_roles (user_id, role_id) VALUES ($1,$2),($3,$4)`, workerID, workerRoleID, pmID, pmRoleID)

	requirement := `{"required_permission":"operation.report"}`
	if documentRequired {
		requirement = `{"required_permission":"operation.report","document_required":true}`
	}
	mustExec(t, db, `
		INSERT INTO work_items
			(id, project_id, source_type, source_id, title, work_type, status, role_ids, evidence_requirement_json, created_by, updated_by)
		VALUES
			($1,$2,'issue',$3,'Batch3 work item','task','ready','["Đội thi công"]'::jsonb,$4::jsonb,$5,$5)
	`, itemID, projectID, sourceID, requirement, workerID)

	return workItemFlowFixture{projectID: projectID, workerID: workerID, pmID: pmID, itemID: itemID}
}

func TestStartPipelineGeneratesWorkItemsAndIsIdempotent(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)

	actorID := newID()
	templateID := newID()
	nodeID := newID()
	projectID := newID()
	stepOneID := newID()
	stepTwoID := newID()
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM projects WHERE id=$1`, projectID)
		_, _ = db.ExecContext(ctx, `DELETE FROM pipeline_templates WHERE id=$1`, templateID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id=$1`, actorID)
	})

	mustExec(t, db, `INSERT INTO users (id, email, password_hash, full_name, status) VALUES ($1,$2,'demo:pw','Test Actor','active')`, actorID, "ops-start-"+actorID+"@test.local")
	mustExec(t, db, `INSERT INTO pipeline_templates (id, name, status, created_by) VALUES ($1,'Ops Template','published',$2)`, templateID, actorID)
	mustExec(t, db, `
		INSERT INTO pipeline_template_nodes (id, template_id, name, type, order_index, responsible_role_ids)
		VALUES ($1,$2,'Survey','survey',1,'["PM"]'::jsonb)
	`, nodeID, templateID)
	mustExec(t, db, `
		INSERT INTO pipeline_template_steps (id, node_id, name, step_type, required_role_ids, document_required, photo_required, order_index)
		VALUES ($1,$2,'Upload survey','document_upload','["PM"]'::jsonb,false,false,1),
		       ($3,$2,'Confirm survey','checklist','["PM"]'::jsonb,false,false,2)
	`, stepOneID, nodeID, stepTwoID)
	mustExec(t, db, `
		INSERT INTO projects (id, name, code, client_name, status, pipeline_template_id)
		VALUES ($1,'Ops Project',$2,'Client','draft',$3)
	`, projectID, "OPS-"+projectID[:8], templateID)

	firstRun, err := repo.StartPipeline(ctx, projectID, templateID, actorID, "actor@test.local")
	if err != nil {
		t.Fatal(err)
	}
	secondRun, err := repo.StartPipeline(ctx, projectID, templateID, actorID, "actor@test.local")
	if err != nil {
		t.Fatal(err)
	}
	if firstRun["id"] != secondRun["id"] {
		t.Fatalf("expected idempotent StartPipeline to return same run, got %v and %v", firstRun["id"], secondRun["id"])
	}
	assertCount(t, db, `SELECT COUNT(*) FROM pipeline_runs WHERE project_id=$1`, 1, projectID)
	assertCount(t, db, `SELECT COUNT(*) FROM pipeline_run_steps WHERE project_id=$1`, 2, projectID)
	assertCount(t, db, `SELECT COUNT(*) FROM work_items WHERE project_id=$1 AND source_type='pipeline_run_step'`, 2, projectID)
}

func TestGenerateProcurementFromBOQUsesApprovedItemsOnly(t *testing.T) {
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
		VALUES ($1,$2,'A','Approved item','m2',2,10,'approved',$3),
		       ($4,$2,'A','Submitted item','m2',100,10,'submitted',$3)
	`, approvedBOQID, projectID, actorID, submittedBOQID)

	procurement, err := repo.GenerateProcurementFromBOQ(ctx, projectID, actorID, "actor@test.local")
	if err != nil {
		t.Fatal(err)
	}
	requestID := procurement["id"]
	assertCount(t, db, `SELECT COUNT(*) FROM procurement_request_items WHERE request_id=$1`, 1, requestID)
	assertCount(t, db, `SELECT COUNT(*) FROM procurement_request_items WHERE request_id=$1 AND boq_item_id=$2`, 1, requestID, approvedBOQID)
	assertCount(t, db, `SELECT COUNT(*) FROM procurement_request_items WHERE request_id=$1 AND boq_item_id=$2`, 0, requestID, submittedBOQID)
}

func TestWorkItemEvidenceRequiredBeforeReviewApprovalAndCompletion(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")
	fixture := setupWorkItemFlowFixture(t, db, true)

	_, err := service.WorkItemAction(ctx, fixture.itemID, "request-approval", WorkItemActionRequest{
		Comment:       "PM duyệt giúp",
		RequiredRoles: []string{"PM"},
	}, fixture.workerID, "Worker")
	if err == nil {
		t.Fatal("expected approval request to require evidence")
	}
	var statusErr StatusError
	if !errors.As(err, &statusErr) || statusErr.Code != "VALIDATION_ERROR" {
		t.Fatalf("expected validation error, got %#v", err)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM approval_requests WHERE target_type='work_item' AND target_id=$1`, 0, fixture.itemID)

	_, err = service.WorkItemAction(ctx, fixture.itemID, "request-review", WorkItemActionRequest{Comment: "review giúp"}, fixture.workerID, "Worker")
	if err == nil {
		t.Fatal("expected review request to require evidence")
	}
	if !errors.As(err, &statusErr) || statusErr.Code != "VALIDATION_ERROR" {
		t.Fatalf("expected validation error, got %#v", err)
	}

	evidence, err := service.AddWorkItemEvidence(ctx, fixture.itemID, WorkItemEvidenceRequest{
		EvidenceType: "document",
		Name:         "Biên bản gửi duyệt",
		FileURL:      "/evidence/bien-ban-gui-duyet.pdf",
		Visibility:   "project",
	}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	if evidence["status"] != "submitted" {
		t.Fatalf("evidence status = %v, want submitted", evidence["status"])
	}
	waiting, err := service.WorkItemAction(ctx, fixture.itemID, "request-approval", WorkItemActionRequest{
		Comment:       "PM duyệt giúp",
		RequiredRoles: []string{"PM"},
	}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	if waiting["status"] != "waiting_approval" {
		t.Fatalf("work item status = %v, want waiting_approval", waiting["status"])
	}

	completionFixture := setupWorkItemFlowFixture(t, db, true)
	_, err = service.WorkItemAction(ctx, completionFixture.itemID, "complete", WorkItemActionRequest{Comment: "done"}, completionFixture.workerID, "Worker")
	if err == nil {
		t.Fatal("expected completion to require evidence")
	}
	if !errors.As(err, &statusErr) || statusErr.Code != "VALIDATION_ERROR" {
		t.Fatalf("expected validation error, got %#v", err)
	}

	evidence, err = service.AddWorkItemEvidence(ctx, completionFixture.itemID, WorkItemEvidenceRequest{
		EvidenceType: "document",
		Name:         "Biên bản hoàn tất",
		FileURL:      "/evidence/bien-ban.pdf",
		Visibility:   "project",
	}, completionFixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	if evidence["status"] != "submitted" {
		t.Fatalf("evidence status = %v, want submitted", evidence["status"])
	}

	completed, err := service.WorkItemAction(ctx, completionFixture.itemID, "complete", WorkItemActionRequest{Comment: "done"}, completionFixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	if completed["status"] != "completed" {
		t.Fatalf("work item status = %v, want completed", completed["status"])
	}
	assertCount(t, db, `SELECT COUNT(*) FROM evidence_records WHERE work_item_id=$1 AND evidence_type='document'`, 1, completionFixture.itemID)
	assertCount(t, db, `SELECT COUNT(*) FROM work_item_activities WHERE work_item_id=$1 AND activity_type='evidence_added'`, 1, completionFixture.itemID)
	assertCount(t, db, `SELECT COUNT(*) FROM work_item_activities WHERE work_item_id=$1 AND activity_type='completed'`, 1, completionFixture.itemID)
	assertCount(t, db, `SELECT COUNT(*) FROM audit_logs WHERE project_id=$1 AND event_type IN ('work_item_evidence_added','work_item_completed')`, 2, completionFixture.projectID)
}

func TestWorkItemApprovalRequiresDecisionScope(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")
	fixture := setupWorkItemFlowFixture(t, db, false)

	waiting, err := service.WorkItemAction(ctx, fixture.itemID, "request-approval", WorkItemActionRequest{
		Comment:       "PM duyệt giúp",
		RequiredRoles: []string{"PM"},
	}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	if waiting["status"] != "waiting_approval" {
		t.Fatalf("work item status = %v, want waiting_approval", waiting["status"])
	}
	approvalID := cleanID(waiting["approval_request_id"])
	if approvalID == "" {
		t.Fatal("expected approval_request_id")
	}

	_, err = service.ApprovalAction(ctx, approvalID, "approve", "self approve", fixture.workerID, "Worker")
	if err == nil {
		t.Fatal("expected worker self approval to be forbidden")
	}
	var statusErr StatusError
	if !errors.As(err, &statusErr) || statusErr.Code != "FORBIDDEN" {
		t.Fatalf("expected forbidden error, got %#v", err)
	}

	approval, err := service.ApprovalAction(ctx, approvalID, "approve", "approved by PM", fixture.pmID, "PM")
	if err != nil {
		t.Fatal(err)
	}
	if approval["status"] != "approved" {
		t.Fatalf("approval status = %v, want approved", approval["status"])
	}
	approved, err := repo.GetWorkItemDetail(ctx, fixture.itemID)
	if err != nil {
		t.Fatal(err)
	}
	if approved["status"] != "completed" {
		t.Fatalf("work item status = %v, want completed", approved["status"])
	}
	assertCount(t, db, `SELECT COUNT(*) FROM approval_requests WHERE id=$1 AND status='approved'`, 1, approvalID)
	assertCount(t, db, `SELECT COUNT(*) FROM approval_actions WHERE approval_request_id=$1 AND action='approve'`, 1, approvalID)
	assertCount(t, db, `SELECT COUNT(*) FROM work_item_activities WHERE work_item_id=$1 AND activity_type='approval_requested'`, 1, fixture.itemID)
	assertCount(t, db, `SELECT COUNT(*) FROM work_item_activities WHERE work_item_id=$1 AND activity_type='approved'`, 1, fixture.itemID)
	assertCount(t, db, `SELECT COUNT(*) FROM audit_logs WHERE project_id=$1 AND event_type IN ('work_item_approval_requested','work_item_approved')`, 2, fixture.projectID)
}

func TestWorkItemReviewSelfApprovalForbidden(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")
	fixture := setupWorkItemFlowFixture(t, db, false)

	waiting, err := service.WorkItemAction(ctx, fixture.itemID, "request-review", WorkItemActionRequest{Comment: "review giúp"}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	if waiting["status"] != "waiting_review" {
		t.Fatalf("work item status = %v, want waiting_review", waiting["status"])
	}

	_, err = service.WorkItemAction(ctx, fixture.itemID, "approve", WorkItemActionRequest{Comment: "self review"}, fixture.workerID, "Worker")
	if err == nil {
		t.Fatal("expected requester review approval to be forbidden")
	}
	var statusErr StatusError
	if !errors.As(err, &statusErr) || statusErr.Code != "FORBIDDEN" {
		t.Fatalf("expected forbidden error, got %#v", err)
	}

	approved, err := service.WorkItemAction(ctx, fixture.itemID, "approve", WorkItemActionRequest{Comment: "PM reviewed"}, fixture.pmID, "PM")
	if err != nil {
		t.Fatal(err)
	}
	if approved["status"] != "completed" {
		t.Fatalf("work item status = %v, want completed", approved["status"])
	}
	assertCount(t, db, `SELECT COUNT(*) FROM work_item_activities WHERE work_item_id=$1 AND activity_type='review_requested'`, 1, fixture.itemID)
	assertCount(t, db, `SELECT COUNT(*) FROM work_item_activities WHERE work_item_id=$1 AND activity_type='approved'`, 1, fixture.itemID)
}

func TestWorkItemGenericApprovalRequestChangeKeepsItemInSync(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")
	fixture := setupWorkItemFlowFixture(t, db, false)

	waiting, err := service.WorkItemAction(ctx, fixture.itemID, "request-approval", WorkItemActionRequest{
		Comment:       "PM xem lại",
		RequiredRoles: []string{"PM"},
	}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	approvalID := cleanID(waiting["approval_request_id"])
	if approvalID == "" {
		t.Fatal("expected approval_request_id")
	}

	approval, err := service.ApprovalAction(ctx, approvalID, "request_change", "Bổ sung biên bản", fixture.pmID, "PM")
	if err != nil {
		t.Fatal(err)
	}
	if approval["status"] != "changes_requested" {
		t.Fatalf("approval status = %v, want changes_requested", approval["status"])
	}
	item, err := repo.GetWorkItemDetail(ctx, fixture.itemID)
	if err != nil {
		t.Fatal(err)
	}
	if item["status"] != "needs_changes" {
		t.Fatalf("work item status = %v, want needs_changes", item["status"])
	}
	assertCount(t, db, `SELECT COUNT(*) FROM approval_actions WHERE approval_request_id=$1 AND action='request-change'`, 1, approvalID)
	assertCount(t, db, `SELECT COUNT(*) FROM work_item_activities WHERE work_item_id=$1 AND activity_type='changes_requested'`, 1, fixture.itemID)
}

func TestGenericApprovalActionRequiresDecisionScope(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")
	fixture := setupWorkItemFlowFixture(t, db, false)
	approvalID := newID()
	mustExec(t, db, `
		INSERT INTO approval_requests (id, target_type, target_id, project_id, requested_by, status, reason, required_roles)
		VALUES ($1,'procurement_request',NULL,$2,$3,'pending','Duyệt mua sắm','["PM"]'::jsonb)
	`, approvalID, fixture.projectID, fixture.workerID)

	_, err := service.ApprovalAction(ctx, approvalID, "approve", "self approve", fixture.workerID, "Worker")
	if err == nil {
		t.Fatal("expected generic approval requester to be forbidden")
	}
	var statusErr StatusError
	if !errors.As(err, &statusErr) || statusErr.Code != "FORBIDDEN" {
		t.Fatalf("expected forbidden error, got %#v", err)
	}

	approved, err := service.ApprovalAction(ctx, approvalID, "approve", "approved by PM", fixture.pmID, "PM")
	if err != nil {
		t.Fatal(err)
	}
	if approved["status"] != "approved" {
		t.Fatalf("approval status = %v, want approved", approved["status"])
	}
	assertCount(t, db, `SELECT COUNT(*) FROM approval_actions WHERE approval_request_id=$1 AND action='approve'`, 1, approvalID)
	assertCount(t, db, `SELECT COUNT(*) FROM audit_logs WHERE project_id=$1 AND event_type='approval_granted'`, 1, fixture.projectID)
}

func mustExec(t *testing.T, db *sql.DB, query string, args ...any) {
	t.Helper()
	if _, err := db.ExecContext(context.Background(), query, args...); err != nil {
		t.Fatal(err)
	}
}

func assertCount(t *testing.T, db *sql.DB, query string, want int, args ...any) {
	t.Helper()
	var got int
	if err := db.QueryRowContext(context.Background(), query, args...).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("count mismatch for %q: got %d want %d", query, got, want)
	}
}
