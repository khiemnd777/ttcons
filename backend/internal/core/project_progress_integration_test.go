package core

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
)

func TestProjectProgressBoardSummaryAndStageCursor(t *testing.T) {
	database := openIsolatedAccountingTestDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	userID := newID()
	staffID := newID()
	otherProjectID := newID()
	mustAccountingExec(t, ctx, database,
		`INSERT INTO users(id,email,password_hash,full_name) VALUES ($1,$2,'demo:test','Progress User')`,
		userID, userID+"@example.com",
	)
	mustAccountingExec(t, ctx, database,
		`INSERT INTO staff(id,user_id,full_name,email,department,status) VALUES ($1,$2,'Progress PM',$3,'Dự án','active')`,
		staffID, userID, staffID+"@example.com",
	)
	roleID := ensureRole(t, database, "progress_view_"+userID[:8], "Progress View")
	permissionID := ensurePermission(t, database, "project.view", "Xem công trình", "project")
	mustAccountingExec(t, ctx, database, `INSERT INTO role_permissions(role_id,permission_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`, roleID, permissionID)
	mustAccountingExec(t, ctx, database, `INSERT INTO user_roles(user_id,role_id) VALUES ($1,$2)`, userID, roleID)

	visibleProjectIDs := make([]string, 0, 27)
	for index := 0; index < 27; index++ {
		projectID := newID()
		visibleProjectIDs = append(visibleProjectIDs, projectID)
		createdAt := time.Date(2026, time.August, 14, 0, index, 0, 0, time.UTC)
		mustAccountingExec(t, ctx, database, `
			INSERT INTO projects(id,name,code,client_name,status,pm_staff_id,created_at)
			VALUES ($1,$2,$3,'Progress Client','planning',$4,$5)
		`, projectID, fmt.Sprintf("Progress Scoped %02d", index), fmt.Sprintf("PROGRESS-%02d", index), staffID, createdAt)
		mustAccountingExec(t, ctx, database,
			`INSERT INTO project_members(project_id,staff_id,role_ids,active) VALUES ($1,$2,'["PM"]'::jsonb,true)`,
			projectID, staffID,
		)
	}
	completedID := newID()
	mustAccountingExec(t, ctx, database, `
		INSERT INTO projects(id,name,code,client_name,status,pm_staff_id,created_at)
		VALUES ($1,'Progress Completed','PROGRESS-DONE','Progress Client','completed',$2,$3)
	`, completedID, staffID, time.Date(2026, time.August, 13, 0, 0, 0, 0, time.UTC))
	mustAccountingExec(t, ctx, database,
		`INSERT INTO project_members(project_id,staff_id,role_ids,active) VALUES ($1,$2,'["PM"]'::jsonb,true)`,
		completedID, staffID,
	)
	mustAccountingExec(t, ctx, database, `
		INSERT INTO projects(id,name,code,client_name,status,pm_staff_id)
		VALUES ($1,'Progress Invisible','PROGRESS-HIDDEN','Progress Client','planning',$2)
	`, otherProjectID, staffID)
	cancelledID := newID()
	mustAccountingExec(t, ctx, database, `
		INSERT INTO projects(id,name,code,client_name,status,pm_staff_id)
		VALUES ($1,'Progress Cancelled','PROGRESS-CANCELLED','Progress Client','cancelled',$2)
	`, cancelledID, staffID)
	mustAccountingExec(t, ctx, database,
		`INSERT INTO project_members(project_id,staff_id,role_ids,active) VALUES ($1,$2,'["PM"]'::jsonb,true)`,
		cancelledID, staffID,
	)

	service := NewService(NewRepository(database), "test-secret")
	summary, err := service.ProjectProgressBoardSummary(ctx, userID, ProjectProgressFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if summary.FilteredTotal != 28 || summary.StageCounts["not_started"] != 27 || summary.StageCounts["completed"] != 1 {
		t.Fatalf("summary=%+v", summary)
	}
	if len(summary.PMOptions) != 1 || summary.PMOptions[0].ID != staffID || len(summary.StatusOptions) != 2 {
		t.Fatalf("summary options=%+v statuses=%+v", summary.PMOptions, summary.StatusOptions)
	}

	filtered, err := service.ProjectProgressBoardSummary(ctx, userID, ProjectProgressFilter{
		Search:    "Scoped 2",
		PMStaffID: staffID,
		Status:    "planning",
	})
	if err != nil {
		t.Fatal(err)
	}
	if filtered.FilteredTotal != 7 || filtered.StageCounts["not_started"] != 7 {
		t.Fatalf("filtered summary=%+v", filtered)
	}

	request := ProjectProgressStageRequest{
		StageKey: "not_started",
		Limit:    5,
	}
	first, err := service.ProjectProgressBoardStage(ctx, userID, request)
	if err != nil {
		t.Fatal(err)
	}
	if first.Total != 27 || len(first.Items) != 5 || !first.HasMore || first.NextCursor == "" {
		t.Fatalf("first page=%+v", first)
	}
	seen := map[string]bool{}
	for _, item := range first.Items {
		seen[cleanID(item["id"])] = true
		if item["stage_key"] != "not_started" {
			t.Fatalf("unexpected stage item=%+v", item)
		}
		if _, leaked := item["cursor_created_at"]; leaked {
			t.Fatalf("cursor field leaked in item=%+v", item)
		}
	}
	request.Cursor = first.NextCursor
	second, err := service.ProjectProgressBoardStage(ctx, userID, request)
	if err != nil {
		t.Fatal(err)
	}
	if second.Total != 27 || len(second.Items) != 5 {
		t.Fatalf("second page=%+v", second)
	}
	for _, item := range second.Items {
		if seen[cleanID(item["id"])] {
			t.Fatalf("cursor page duplicated project %s", item["id"])
		}
	}

	if _, err := service.ProjectProgressBoardStage(ctx, userID, ProjectProgressStageRequest{
		ProjectProgressFilter: ProjectProgressFilter{Status: "completed"},
		StageKey:              "not_started",
		Cursor:                first.NextCursor,
		Limit:                 5,
	}); err == nil {
		t.Fatal("cursor must fail closed after filters change")
	}

	handler := NewHandler(service)
	app := batch4TestApp(userID, func(app *fiber.App, handler *Handler) {
		app.Get("/projects/progress-board", handler.ProjectProgressBoard)
		app.Get("/projects/progress-board/summary", handler.ProjectProgressBoardSummary)
		app.Get("/projects/progress-board/stages/:stageKey", handler.ProjectProgressBoardStage)
	}, handler)
	for _, route := range []string{
		"/projects/progress-board",
		"/projects/progress-board/summary",
		"/projects/progress-board/stages/not_started?limit=5",
	} {
		httpResponse, err := app.Test(httptest.NewRequest(http.MethodGet, route, nil))
		if err != nil {
			t.Fatal(err)
		}
		if httpResponse.StatusCode != http.StatusOK {
			t.Fatalf("route %s status=%d want=%d", route, httpResponse.StatusCode, http.StatusOK)
		}
		var envelope struct {
			Success bool            `json:"success"`
			Data    json.RawMessage `json:"data"`
		}
		if err := json.NewDecoder(httpResponse.Body).Decode(&envelope); err != nil {
			t.Fatal(err)
		}
		if !envelope.Success || len(envelope.Data) == 0 {
			t.Fatalf("route %s envelope=%+v", route, envelope)
		}
	}
	invalidResponse, err := app.Test(httptest.NewRequest(
		http.MethodGet,
		"/projects/progress-board/stages/not_started?limit=5&status=completed&cursor="+first.NextCursor,
		nil,
	))
	if err != nil {
		t.Fatal(err)
	}
	if invalidResponse.StatusCode != http.StatusBadRequest {
		t.Fatalf("mismatched cursor status=%d want=%d", invalidResponse.StatusCode, http.StatusBadRequest)
	}
}
