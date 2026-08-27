package core

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"consflow/backend/internal/shared/migrations"

	"github.com/gofiber/fiber/v2"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func recordByID(rows []Record, key, id string) Record {
	for _, row := range rows {
		if cleanID(row[key]) == id {
			return row
		}
	}
	return nil
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

func testMultipartFileHeader(t *testing.T, fieldName, filename string, content []byte) *multipart.FileHeader {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile(fieldName, filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/upload", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if err := req.ParseMultipartForm(int64(body.Len()) + 1024); err != nil {
		t.Fatal(err)
	}
	files := req.MultipartForm.File[fieldName]
	if len(files) != 1 {
		t.Fatalf("expected one multipart file for %s, got %d", fieldName, len(files))
	}
	return files[0]
}

func testSignatureRequest(comment string) WorkflowFormSignRequest {
	return WorkflowFormSignRequest{
		Purpose:               "internal_ack",
		Comment:               comment,
		SignatureImageDataURL: "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+/p9sAAAAASUVORK5CYII=",
		SignatureStrokes: []WorkflowFormSignatureStroke{{
			Points: []WorkflowFormSignaturePoint{
				{X: 0.1, Y: 0.7},
				{X: 0.35, Y: 0.25},
				{X: 0.7, Y: 0.55},
				{X: 0.9, Y: 0.3},
			},
		}},
	}
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

func recordListHasCode(rows []Record, code string) bool {
	for _, row := range rows {
		if fmt.Sprint(row["code"]) == code {
			return true
		}
	}
	return false
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

func bindWorkItemDomainExecutionContext(t *testing.T, db *sql.DB, fixture workItemFlowFixture, nodeType, nodeBusinessCode, surfaceKind, actionKey string) {
	t.Helper()
	runID := newID()
	nodeID := newID()
	stepID := newID()
	mustExec(t, db, `
		INSERT INTO pipeline_runs (id,project_id,template_id,status,engine_version)
		VALUES ($1,$2,'40000000-0000-0000-0000-000000000021','in_progress',2)
	`, runID, fixture.projectID)
	mustExec(t, db, `
		INSERT INTO pipeline_run_nodes (
			id,run_id,project_id,name,type,business_code,order_index,responsible_role_ids,status,started_at
		) VALUES ($1,$2,$3,'Domain mutation test node',$4,NULLIF($5,''),1,'["Đội thi công"]'::jsonb,'in_progress',now())
	`, nodeID, runID, fixture.projectID, nodeType, nodeBusinessCode)
	mustExec(t, db, `UPDATE pipeline_runs SET current_node_id=$2 WHERE id=$1`, runID, nodeID)
	mustExec(t, db, `
		INSERT INTO pipeline_run_steps (
			id,run_node_id,project_id,name,step_type,required_role_ids,required_permission,
			checklist_required,order_index,status,surface_kind,domain_action_key,business_code
		) VALUES (
			$1,$2,$3,'Domain mutation test step','task','["Đội thi công"]'::jsonb,'operation.report',
			false,1,'ready',NULLIF($4,''),NULLIF($5,''),$6
		)
	`, stepID, nodeID, fixture.projectID, surfaceKind, actionKey, "test."+stepID)
	mustExec(t, db, `
		UPDATE work_items
		SET pipeline_run_id=$2,run_node_id=$3,run_step_id=$4
		WHERE id=$1
	`, fixture.itemID, runID, nodeID, stepID)
}

type clientOfficialWorkflowFixture struct {
	projectID    string
	actorID      string
	actorStaffID string
}

func setupClientOfficialWorkflowFixture(t *testing.T, db *sql.DB, contractAmount float64) clientOfficialWorkflowFixture {
	t.Helper()
	ctx := context.Background()
	repo := NewRepository(db)
	templateID := clientOfficialWorkflowTemplateID
	actorID := newID()
	actorStaffID := newID()
	projectID := newID()
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM projects WHERE id=$1`, projectID)
		_, _ = db.ExecContext(ctx, `DELETE FROM staff WHERE id=$1`, actorStaffID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id=$1`, actorID)
	})
	mustExec(t, db, `INSERT INTO users (id, email, password_hash, full_name, status) VALUES ($1,$2,'demo:pw','Client Actor','active')`, actorID, "client-auto-"+actorID+"@test.local")
	mustExec(t, db, `INSERT INTO staff (id, user_id, full_name, email, department, status) VALUES ($1,$2,'Client Actor',$3,'operations','active')`, actorStaffID, actorID, "client-auto-staff-"+actorID+"@test.local")
	mustExec(t, db, `
		INSERT INTO projects (id, name, code, client_name, status, pipeline_template_id, contract_amount)
		VALUES ($1,'Client Automation Project',$2,'Client','draft',$3,$4)
	`, projectID, "CA-"+projectID[:8], templateID, contractAmount)
	mustExec(t, db, `INSERT INTO project_members (project_id, staff_id, role_ids, active) VALUES ($1,$2,'["PM"]'::jsonb,true)`, projectID, actorStaffID)
	if _, err := repo.StartPipeline(ctx, projectID, templateID, actorID, "Client Actor"); err != nil {
		t.Fatal(err)
	}
	return clientOfficialWorkflowFixture{projectID: projectID, actorID: actorID, actorStaffID: actorStaffID}
}

func clientOfficialWorkItemIDByNodeName(t *testing.T, db *sql.DB, projectID, nodeName string) string {
	t.Helper()
	var workItemID string
	if err := db.QueryRowContext(context.Background(), `
		SELECT wi.id::text
		FROM work_items wi
		JOIN pipeline_run_nodes rn ON rn.id=wi.run_node_id
		WHERE wi.project_id=$1 AND rn.name=$2
		ORDER BY wi.created_at, wi.id
		LIMIT 1
	`, projectID, nodeName).Scan(&workItemID); err != nil {
		t.Fatal(err)
	}
	return workItemID
}

func clientOfficialWorkItemIDByAction(t *testing.T, db *sql.DB, projectID, actionKey string) string {
	t.Helper()
	var workItemID string
	if err := db.QueryRowContext(context.Background(), `
		SELECT wi.id::text
		FROM work_items wi
		JOIN pipeline_run_steps rs ON rs.id=wi.run_step_id
		WHERE wi.project_id=$1 AND rs.domain_action_key=$2
		ORDER BY wi.created_at,wi.id
		LIMIT 1
	`, projectID, actionKey).Scan(&workItemID); err != nil {
		t.Fatal(err)
	}
	return workItemID
}

func satisfyRunNodeForAdvance(t *testing.T, db *sql.DB, runNodeID, actorID string) {
	t.Helper()
	ctx := context.Background()
	mustExec(t, db, `
		UPDATE workflow_form_instances
		SET status='submitted',current_revision=GREATEST(current_revision,1),submitted_by=$2,submitted_at=COALESCE(submitted_at,now()),updated_at=now()
		WHERE run_node_id=$1 AND status <> 'cancelled'
	`, runNodeID, actorID)
	mustExec(t, db, `UPDATE pipeline_run_steps SET status='completed',submitted_by=$2,submitted_at=COALESCE(submitted_at,now()) WHERE run_node_id=$1`, runNodeID, actorID)
	mustExec(t, db, `UPDATE work_items SET status='completed',completed_at=COALESCE(completed_at,now()),updated_at=now() WHERE run_node_id=$1`, runNodeID)
	var projectID string
	if err := db.QueryRowContext(ctx, `SELECT project_id::text FROM pipeline_run_nodes WHERE id=$1`, runNodeID).Scan(&projectID); err != nil {
		t.Fatal(err)
	}
	mustExec(t, db, `
		INSERT INTO approval_requests (target_type,target_id,project_id,requested_by,status,reason,required_roles,resolved_at)
		VALUES ('pipeline_run_node',$1,$2,$3,'approved','UAT fixture đã duyệt node','[]'::jsonb,now())
	`, runNodeID, projectID, actorID)
}

func clientOfficialWorkItemIDsByNodeName(t *testing.T, db *sql.DB, projectID, nodeName string) []string {
	t.Helper()
	rows, err := db.QueryContext(context.Background(), `
		SELECT wi.id::text
		FROM work_items wi
		JOIN pipeline_run_nodes rn ON rn.id=wi.run_node_id
		WHERE wi.project_id=$1 AND rn.name=$2
		ORDER BY wi.created_at, wi.id
	`, projectID, nodeName)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	var workItemIDs []string
	for rows.Next() {
		var workItemID string
		if err := rows.Scan(&workItemID); err != nil {
			t.Fatal(err)
		}
		workItemIDs = append(workItemIDs, workItemID)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(workItemIDs) == 0 {
		t.Fatalf("work items for node %q not found", nodeName)
	}
	return workItemIDs
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

func runNodeIDByName(t *testing.T, run Record, name string) string {
	t.Helper()
	nodes, _ := run["nodes"].([]Record)
	for _, node := range nodes {
		if fmt.Sprint(node["name"]) == name {
			return cleanID(node["id"])
		}
	}
	t.Fatalf("run node %q not found in %#v", name, nodes)
	return ""
}

func assertBearerStatus(t *testing.T, app *fiber.App, path string, token string, want int) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != want {
		t.Fatalf("status = %d, want %d", resp.StatusCode, want)
	}
}

func assertValidationError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected validation error")
	}
	var statusErr StatusError
	if !errors.As(err, &statusErr) || statusErr.Code != "VALIDATION_ERROR" {
		t.Fatalf("expected validation error, got %#v", err)
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func hasWarningCode(warnings []Record, code string) bool {
	for _, warning := range warnings {
		if warning["code"] == code {
			return true
		}
	}
	return false
}
