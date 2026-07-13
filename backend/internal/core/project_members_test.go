package core

import (
	"context"
	"strings"
	"testing"
)

func TestNormalizeProjectMemberMutationRequest(t *testing.T) {
	projectID := newID()
	staffID := newID()
	roleID := newID()
	req, err := normalizeProjectMemberMutationRequest(projectID, "", ProjectMemberMutationRequest{
		StaffID:        staffID,
		RoleIDs:        []string{" PM ", "PM", "QS"},
		ProjectRoleIDs: []string{roleID, roleID},
		Notes:          "  Phụ trách điều hành  ",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(req.RoleIDs) != 2 || req.RoleIDs[0] != "PM" || len(req.ProjectRoleIDs) != 1 || req.Notes != "Phụ trách điều hành" {
		t.Fatalf("normalized project member request = %#v", req)
	}
	if _, err := normalizeProjectMemberMutationRequest(projectID, "", ProjectMemberMutationRequest{StaffID: staffID}); err == nil {
		t.Fatal("expected empty operational roles to fail")
	}
}

func TestProjectMemberLifecycleAndProjectManagerProtection(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	projectID := newID()
	workerUserID := newID()
	workerStaffID := newID()
	pmUserID := newID()
	pmStaffID := newID()
	roleID := newID()
	roleName := "project-member-test-" + strings.ReplaceAll(roleID[:8], "-", "")
	permissionID := newID()
	projectViewPermissionSeedID := newID()

	mustExec(t, db, `INSERT INTO permissions (id,key,name,module) VALUES ($1,$2,'Project member test view','project')`, permissionID, "project.member.test.view."+roleID[:8])
	mustExec(t, db, `INSERT INTO roles (id,name,display_name,system_role) VALUES ($1,$2,'Project Member Test',false)`, roleID, roleName)
	mustExec(t, db, `INSERT INTO role_permissions (role_id,permission_id) VALUES ($1,$2)`, roleID, permissionID)
	var projectViewPermissionExisted bool
	if err := db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM permissions WHERE key='project.view')`).Scan(&projectViewPermissionExisted); err != nil {
		t.Fatal(err)
	}
	mustExec(t, db, `INSERT INTO permissions (id,key,name,module) VALUES ($1,'project.view','Xem công trình','project') ON CONFLICT (key) DO NOTHING`, projectViewPermissionSeedID)
	var projectViewPermissionID string
	if err := db.QueryRowContext(ctx, `SELECT id::text FROM permissions WHERE key='project.view'`).Scan(&projectViewPermissionID); err != nil {
		t.Fatal(err)
	}
	mustExec(t, db, `INSERT INTO role_permissions (role_id,permission_id) VALUES ($1,$2)`, roleID, projectViewPermissionID)
	mustExec(t, db, `INSERT INTO users (id,email,password_hash,full_name,status) VALUES ($1,$2,'x','Worker Member','active'),($3,$4,'x','PM Member','active')`, workerUserID, workerUserID+"@test.local", pmUserID, pmUserID+"@test.local")
	mustExec(t, db, `INSERT INTO staff (id,user_id,full_name,email,department,position,status) VALUES ($1,$2,'Worker Member',$3,'Site','Worker','active'),($4,$5,'PM Member',$6,'Project','PM','active')`, workerStaffID, workerUserID, workerUserID+"@test.local", pmStaffID, pmUserID, pmUserID+"@test.local")
	mustExec(t, db, `INSERT INTO projects (id,name,code,client_name,status,pm_staff_id) VALUES ($1,'Project Member Test',$2,'Client','planning',$3)`, projectID, "PMT-"+projectID[:8], pmStaffID)

	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM user_roles WHERE project_id=$1 OR user_id IN ($2,$3)`, projectID, workerUserID, pmUserID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM audit_logs WHERE project_id=$1`, projectID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM projects WHERE id=$1`, projectID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM staff WHERE id IN ($1,$2)`, workerStaffID, pmStaffID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM users WHERE id IN ($1,$2)`, workerUserID, pmUserID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM roles WHERE id=$1`, roleID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM permissions WHERE id=$1`, permissionID)
		if !projectViewPermissionExisted {
			_, _ = db.ExecContext(context.Background(), `DELETE FROM permissions WHERE id=$1`, projectViewPermissionSeedID)
		}
	})

	worker, err := repo.SaveProjectMember(ctx, projectID, "", ProjectMemberMutationRequest{
		StaffID:        workerStaffID,
		RoleIDs:        []string{"Đội thi công"},
		ProjectRoleIDs: []string{roleID},
		Notes:          "Ca ngày",
	}, "", "Integration Test")
	if err != nil {
		t.Fatal(err)
	}
	workerMemberID := cleanID(worker["id"])
	if worker["active"] != true || cleanID(worker["staff_id"]) != workerStaffID {
		t.Fatalf("created worker member = %#v", worker)
	}
	if _, err := repo.SaveProjectMember(ctx, projectID, "", ProjectMemberMutationRequest{StaffID: workerStaffID, RoleIDs: []string{"QS"}, ProjectRoleIDs: []string{roleID}}, "", "Integration Test"); err == nil {
		t.Fatal("expected duplicate active project member to fail")
	}

	pmMember, err := repo.SaveProjectMember(ctx, projectID, "", ProjectMemberMutationRequest{
		StaffID:        pmStaffID,
		RoleIDs:        []string{"PM"},
		ProjectRoleIDs: []string{roleID},
	}, "", "Integration Test")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.DeactivateProjectMember(ctx, projectID, cleanID(pmMember["id"]), "", "Integration Test"); err == nil {
		t.Fatal("expected current project manager deactivation to fail")
	}

	options, err := repo.ProjectMemberOptions(ctx, projectID)
	if err != nil {
		t.Fatalf("ProjectMemberOptions returned error: %v", err)
	}
	roleLabels := stringSlice(options["role_labels"])
	if len(roleLabels) == 0 {
		t.Fatal("ProjectMemberOptions returned no role labels")
	}
	seenRoleLabels := map[string]bool{}
	foundOperationalRole := false
	for _, label := range roleLabels {
		if seenRoleLabels[label] {
			t.Fatalf("ProjectMemberOptions returned duplicate role label %q: %#v", label, roleLabels)
		}
		seenRoleLabels[label] = true
		if label == "Đội thi công" {
			foundOperationalRole = true
		}
	}
	if !foundOperationalRole {
		t.Fatalf("ProjectMemberOptions role labels missing project operational role: %#v", roleLabels)
	}

	deactivated, err := repo.DeactivateProjectMember(ctx, projectID, workerMemberID, "", "Integration Test")
	if err != nil {
		t.Fatal(err)
	}
	if deactivated["active"] != false {
		t.Fatalf("deactivated worker member = %#v", deactivated)
	}
	assertCount(t, db, `SELECT count(*) FROM user_roles WHERE user_id=$1 AND project_id=$2`, 0, workerUserID, projectID)

	reactivated, err := repo.SaveProjectMember(ctx, projectID, "", ProjectMemberMutationRequest{
		StaffID:        workerStaffID,
		RoleIDs:        []string{"Giám sát"},
		ProjectRoleIDs: []string{roleID},
	}, "", "Integration Test")
	if err != nil {
		t.Fatal(err)
	}
	if cleanID(reactivated["id"]) != workerMemberID || reactivated["active"] != true {
		t.Fatalf("reactivated worker member = %#v", reactivated)
	}
	assertCount(t, db, `SELECT count(*) FROM audit_logs WHERE project_id=$1 AND target_type='project_member'`, 4, projectID)
}
