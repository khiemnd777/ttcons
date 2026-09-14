package core

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestProjectOptionsRespectScopePermissionAndCursor(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	fixture := setupProjectScopeFixture(t, db, []string{"project.view"})
	service := NewService(NewRepository(db), "test-secret")
	extraProjectID := newID()
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM project_members WHERE project_id=$1`, extraProjectID)
		_, _ = db.ExecContext(ctx, `DELETE FROM projects WHERE id=$1`, extraProjectID)
	})
	mustExec(t, db, `INSERT INTO projects (id,name,code,client_name,status) VALUES ($1,'Batch4 Scoped Z',$2,'Client','planning')`, extraProjectID, "POZ-"+extraProjectID[:8])
	mustExec(t, db, `INSERT INTO project_members (project_id,staff_id,role_ids,active) VALUES ($1,$2,'["Member"]'::jsonb,true)`, extraProjectID, fixture.staffID)

	first, err := service.ProjectOptions(ctx, ReportOptionsRequest{
		Search: "batch4 scoped", Limit: 1, SelectedID: fixture.projectID,
	}, fixture.userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Projects.Items) != 1 || first.Projects.Items[0].ID != fixture.projectID || !first.Projects.HasMore || first.Projects.NextCursor == "" {
		t.Fatalf("project options first page = %#v", first.Projects)
	}
	if first.Projects.Selected == nil || first.Projects.Selected.ID != fixture.projectID {
		t.Fatalf("project options selected = %#v", first.Projects.Selected)
	}
	second, err := service.ProjectOptions(ctx, ReportOptionsRequest{Search: "batch4 scoped", Limit: 1, Cursor: first.Projects.NextCursor}, fixture.userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Projects.Items) != 1 || second.Projects.Items[0].ID != extraProjectID || second.Projects.HasMore {
		t.Fatalf("project options second page = %#v", second.Projects)
	}

	handler := NewHandler(service)
	app := batch4TestApp(fixture.userID, func(app *fiber.App, h *Handler) {
		registerProjectsRoutes(app, h)
	}, handler)
	response, err := app.Test(httptest.NewRequest(http.MethodGet, "/projects/options?search=batch4+scoped&limit=1&selected_id="+fixture.projectID, nil))
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
	if !body.Success || len(body.Data.Projects.Items) != 1 || body.Data.Projects.Selected == nil || body.Data.Projects.Selected.ID != fixture.projectID {
		t.Fatalf("project options response = %#v", body)
	}
}
