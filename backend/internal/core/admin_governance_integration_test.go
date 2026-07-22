package core

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"consflow/backend/internal/shared/middleware"

	"github.com/gofiber/fiber/v2"
)

func TestProjectOperationsAccessPolicy(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)

	suffix := newID()
	projectID := newID()
	cancelledProjectID := newID()
	adminUserID := newID()
	directorUserID := newID()
	pmUserID := newID()
	pmStaffID := newID()
	noScopeUserID := newID()
	projectRoleNoPermissionUserID := newID()
	projectRolePermissionUserID := newID()
	var projectPermissionRoleID string

	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM user_roles WHERE user_id IN ($1,$2,$3,$4,$5,$6)`, adminUserID, directorUserID, pmUserID, noScopeUserID, projectRoleNoPermissionUserID, projectRolePermissionUserID)
		_, _ = db.ExecContext(ctx, `DELETE FROM projects WHERE id IN ($1,$2)`, projectID, cancelledProjectID)
		_, _ = db.ExecContext(ctx, `DELETE FROM staff WHERE id=$1`, pmStaffID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id IN ($1,$2,$3,$4,$5,$6)`, adminUserID, directorUserID, pmUserID, noScopeUserID, projectRoleNoPermissionUserID, projectRolePermissionUserID)
		if projectPermissionRoleID != "" {
			_, _ = db.ExecContext(ctx, `DELETE FROM role_permissions WHERE role_id=$1`, projectPermissionRoleID)
			_, _ = db.ExecContext(ctx, `DELETE FROM roles WHERE id=$1`, projectPermissionRoleID)
		}
	})

	operationViewID := ensurePermission(t, db, "operation.view", "Xem thi công", "operation")
	adminRoleID := ensureRole(t, db, "admin", "Quản trị")
	directorRoleID := ensureRole(t, db, "director", "Giám đốc")
	pmRoleID := ensureRole(t, db, "pm", "PM")
	noPermissionRoleID := ensureRole(t, db, "batch2_viewer_"+suffix[:8], "Batch2 Viewer")
	projectPermissionRoleID = ensureRole(t, db, "batch2_project_viewer_"+suffix[:8], "Batch2 Project Viewer")
	mustExec(t, db, `INSERT INTO role_permissions (role_id, permission_id) VALUES ($1,$2),($3,$2),($4,$2) ON CONFLICT DO NOTHING`, directorRoleID, operationViewID, pmRoleID, projectPermissionRoleID)

	mustExec(t, db, `INSERT INTO users (id, email, password_hash, full_name, status) VALUES ($1,$2,'demo:pw','Admin','active')`, adminUserID, "admin-"+suffix+"@test.local")
	mustExec(t, db, `INSERT INTO users (id, email, password_hash, full_name, status) VALUES ($1,$2,'demo:pw','Director','active')`, directorUserID, "director-"+suffix+"@test.local")
	mustExec(t, db, `INSERT INTO users (id, email, password_hash, full_name, status) VALUES ($1,$2,'demo:pw','PM','active')`, pmUserID, "pm-"+suffix+"@test.local")
	mustExec(t, db, `INSERT INTO users (id, email, password_hash, full_name, status) VALUES ($1,$2,'demo:pw','No Scope','active')`, noScopeUserID, "noscope-"+suffix+"@test.local")
	mustExec(t, db, `INSERT INTO users (id, email, password_hash, full_name, status) VALUES ($1,$2,'demo:pw','Project Role No Permission','active')`, projectRoleNoPermissionUserID, "project-role-no-perm-"+suffix+"@test.local")
	mustExec(t, db, `INSERT INTO users (id, email, password_hash, full_name, status) VALUES ($1,$2,'demo:pw','Project Role Permission','active')`, projectRolePermissionUserID, "project-role-perm-"+suffix+"@test.local")
	mustExec(t, db, `INSERT INTO staff (id, user_id, full_name, email, department, position, status) VALUES ($1,$2,'PM',$3,'Dự án','PM','active')`, pmStaffID, pmUserID, "pm-staff-"+suffix+"@test.local")
	mustExec(t, db, `INSERT INTO projects (id, name, code, client_name, status) VALUES ($1,'Ops Access',$2,'Client','planning'),($3,'Ops Cancelled',$4,'Client','cancelled')`, projectID, "OPA-"+suffix[:8], cancelledProjectID, "OPC-"+suffix[:8])
	mustExec(t, db, `INSERT INTO project_members (project_id, staff_id, role_ids, active) VALUES ($1,$2,'["PM"]'::jsonb,true)`, projectID, pmStaffID)
	mustExec(t, db, `INSERT INTO user_roles (user_id, role_id) VALUES ($1,$2),($3,$4),($5,$6),($7,$6)`, adminUserID, adminRoleID, directorUserID, directorRoleID, pmUserID, pmRoleID, noScopeUserID)
	mustExec(t, db, `INSERT INTO user_roles (user_id, role_id, project_id) VALUES ($1,$2,$3)`, projectRoleNoPermissionUserID, noPermissionRoleID, projectID)
	mustExec(t, db, `INSERT INTO user_roles (user_id, role_id, project_id) VALUES ($1,$2,$3)`, projectRolePermissionUserID, projectPermissionRoleID, projectID)

	assertCanViewProjectOperations(t, repo, ctx, projectID, adminUserID, true)
	assertCanViewProjectOperations(t, repo, ctx, projectID, directorUserID, true)
	assertCanViewProjectOperations(t, repo, ctx, projectID, pmUserID, true)
	assertCanViewProjectOperations(t, repo, ctx, projectID, noScopeUserID, false)
	assertCanViewProjectOperations(t, repo, ctx, projectID, projectRoleNoPermissionUserID, false)
	assertCanViewProjectOperations(t, repo, ctx, projectID, projectRolePermissionUserID, true)
	assertCanViewProjectOperations(t, repo, ctx, cancelledProjectID, directorUserID, false)

	visibilityCases := []struct {
		name   string
		userID string
		want   bool
	}{
		{name: "admin", userID: adminUserID, want: true},
		{name: "director", userID: directorUserID, want: true},
		{name: "member", userID: pmUserID, want: true},
		{name: "project role", userID: projectRolePermissionUserID, want: true},
		{name: "no project scope", userID: noScopeUserID, want: false},
		{name: "project role without permission", userID: projectRoleNoPermissionUserID, want: false},
	}
	for _, test := range visibilityCases {
		t.Run("snapshot parity "+test.name, func(t *testing.T) {
			visibleProjects, err := repo.ProjectOperationsVisibleProjectIDs(ctx, test.userID)
			if err != nil {
				t.Fatal(err)
			}
			_, visible := visibleProjects[projectID]
			if visible != test.want {
				t.Fatalf("active project snapshot visibility=%v, want %v", visible, test.want)
			}
			if _, visible := visibleProjects[cancelledProjectID]; visible {
				t.Fatal("cancelled project leaked into activity visibility snapshot")
			}
			canView, err := repo.CanViewProjectOperations(ctx, projectID, test.userID)
			if err != nil {
				t.Fatal(err)
			}
			if visible != canView {
				t.Fatalf("snapshot visibility=%v differs from CanViewProjectOperations=%v", visible, canView)
			}
		})
	}
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

func TestAuthSessionRefreshAndLogoutRevokesAccess(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")
	userID := newID()
	suffix := userID[:8]
	email := "auth-session-" + suffix + "@test.local"
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM auth_sessions WHERE user_id=$1`, userID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id=$1`, userID)
	})
	mustExec(t, db, `INSERT INTO users (id, email, password_hash, full_name, status) VALUES ($1,$2,'demo:pw','Auth Session','active')`, userID, email)

	login, err := service.Login(ctx, LoginRequest{Email: email, Password: "pw"}, "test-agent", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	accessToken := fmt.Sprint(login["token"])
	refreshToken := fmt.Sprint(login["refresh_token"])
	if accessToken == "" || refreshToken == "" {
		t.Fatalf("login tokens missing: %#v", login)
	}

	handler := NewHandler(service)
	app := fiber.New(fiber.Config{ErrorHandler: ErrorHandler})
	app.Use(middleware.AuthRequired("test-secret", repo))
	app.Get("/me", handler.Me)
	assertBearerStatus(t, app, "/me", accessToken, http.StatusOK)

	refreshed, err := service.RefreshSession(ctx, refreshToken, "test-agent", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	nextAccessToken := fmt.Sprint(refreshed["token"])
	nextRefreshToken := fmt.Sprint(refreshed["refresh_token"])
	assertBearerStatus(t, app, "/me", accessToken, http.StatusUnauthorized)
	assertBearerStatus(t, app, "/me", nextAccessToken, http.StatusOK)

	if err := service.LogoutByRefreshToken(ctx, nextRefreshToken); err != nil {
		t.Fatal(err)
	}
	assertBearerStatus(t, app, "/me", nextAccessToken, http.StatusUnauthorized)
}

func TestRBACAssignmentsReplaceSnapshots(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	suffix := newID()
	roleID := ensureRole(t, db, "rbac_replace_"+suffix[:8], "RBAC Replace")
	firstPermissionID := ensurePermission(t, db, "rbac.replace.first."+suffix[:8], "RBAC Replace First", "rbac")
	secondPermissionID := ensurePermission(t, db, "rbac.replace.second."+suffix[:8], "RBAC Replace Second", "rbac")
	userID := newID()
	projectID := newID()
	staffID := newID()
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM project_members WHERE project_id=$1`, projectID)
		_, _ = db.ExecContext(ctx, `DELETE FROM projects WHERE id=$1`, projectID)
		_, _ = db.ExecContext(ctx, `DELETE FROM user_roles WHERE user_id=$1`, userID)
		_, _ = db.ExecContext(ctx, `DELETE FROM role_permissions WHERE role_id=$1`, roleID)
		_, _ = db.ExecContext(ctx, `DELETE FROM roles WHERE id=$1`, roleID)
		_, _ = db.ExecContext(ctx, `DELETE FROM staff WHERE id=$1`, staffID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id=$1`, userID)
	})
	mustExec(t, db, `INSERT INTO users (id, email, password_hash, full_name, status) VALUES ($1,$2,'demo:pw','RBAC User','active')`, userID, "rbac-user-"+suffix+"@test.local")
	mustExec(t, db, `INSERT INTO staff (id, user_id, full_name, email, department, position, status) VALUES ($1,$2,'RBAC User',$3,'Dự án','Member','active')`, staffID, userID, "rbac-staff-"+suffix+"@test.local")
	mustExec(t, db, `INSERT INTO projects (id, name, code, client_name, status) VALUES ($1,'RBAC Scoped',$2,'Client','planning')`, projectID, "RBA-"+suffix[:8])

	if _, err := repo.ReplaceRolePermissions(ctx, roleID, []string{firstPermissionID, secondPermissionID}); err != nil {
		t.Fatal(err)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM role_permissions WHERE role_id=$1`, 2, roleID)
	if _, err := repo.ReplaceRolePermissions(ctx, roleID, []string{secondPermissionID}); err != nil {
		t.Fatal(err)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM role_permissions WHERE role_id=$1 AND permission_id=$2`, 0, roleID, firstPermissionID)
	assertCount(t, db, `SELECT COUNT(*) FROM role_permissions WHERE role_id=$1 AND permission_id=$2`, 1, roleID, secondPermissionID)

	if _, err := repo.ReplaceUserRoles(ctx, userID, []string{roleID}, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.ReplaceUserRoles(ctx, userID, []string{roleID}, projectID); err != nil {
		t.Fatal(err)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM user_roles WHERE user_id=$1 AND project_id IS NULL`, 1, userID)
	assertCount(t, db, `SELECT COUNT(*) FROM user_roles WHERE user_id=$1 AND project_id=$2`, 1, userID, projectID)
	if _, err := repo.ReplaceUserRoles(ctx, userID, []string{}, ""); err != nil {
		t.Fatal(err)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM user_roles WHERE user_id=$1 AND project_id IS NULL`, 0, userID)
	assertCount(t, db, `SELECT COUNT(*) FROM user_roles WHERE user_id=$1 AND project_id=$2`, 1, userID, projectID)
}
