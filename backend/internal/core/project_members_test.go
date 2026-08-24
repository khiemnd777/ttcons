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

func TestProjectMemberOptionsAndSaveExcludeExternalOnlyRoles(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	suffix := newID()
	userID := newID()
	staffID := newID()
	projectID := newID()
	internalRoleID := newID()
	externalRoleID := newID()
	externalLabel := "Đối tác ngoài App " + suffix[:8]
	projectViewPermissionID := ensurePermission(t, db, "project.view", "Xem công trình", "project")
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM project_members WHERE project_id=$1`, projectID)
		_, _ = db.ExecContext(ctx, `DELETE FROM projects WHERE id=$1`, projectID)
		_, _ = db.ExecContext(ctx, `DELETE FROM user_roles WHERE user_id=$1`, userID)
		_, _ = db.ExecContext(ctx, `DELETE FROM role_permissions WHERE role_id IN ($1,$2)`, internalRoleID, externalRoleID)
		_, _ = db.ExecContext(ctx, `DELETE FROM roles WHERE id IN ($1,$2)`, internalRoleID, externalRoleID)
		_, _ = db.ExecContext(ctx, `DELETE FROM staff WHERE id=$1`, staffID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id=$1`, userID)
	})
	mustExec(t, db, `INSERT INTO roles (id,name,display_name,system_role,external_only) VALUES ($1,$2,$3,false,false),($4,$5,$6,true,true)`, internalRoleID, "member_internal_"+suffix[:8], "Nội bộ "+suffix[:8], externalRoleID, "member_external_"+suffix[:8], externalLabel)
	mustExec(t, db, `INSERT INTO role_permissions (role_id,permission_id) VALUES ($1,$2)`, internalRoleID, projectViewPermissionID)
	mustExec(t, db, `INSERT INTO users (id,email,password_hash,full_name,status) VALUES ($1,$2,'demo:pw','Member External Guard','active')`, userID, "member-external-guard-"+suffix[:8]+"@test.local")
	mustExec(t, db, `INSERT INTO staff (id,user_id,full_name,email,department,position,status) VALUES ($1,$2,'Member External Guard',$3,'Dự án','Member','active')`, staffID, userID, "member-external-staff-"+suffix[:8]+"@test.local")
	mustExec(t, db, `INSERT INTO projects (id,name,code,client_name,status) VALUES ($1,'External Member Guard',$2,'Client','planning')`, projectID, "EMG-"+suffix[:8])

	options, err := repo.ProjectMemberOptions(ctx, projectID)
	if err != nil {
		t.Fatal(err)
	}
	for _, role := range options["access_roles"].([]Record) {
		if cleanID(role["id"]) == externalRoleID {
			t.Fatalf("external-only access role leaked into options: %#v", role)
		}
	}
	for _, label := range stringSlice(options["role_labels"]) {
		if label == externalLabel {
			t.Fatalf("external-only operational label leaked into options: %#v", options["role_labels"])
		}
	}
	if _, err := repo.SaveProjectMember(ctx, projectID, "", ProjectMemberMutationRequest{StaffID: staffID, RoleIDs: []string{"Nội bộ"}, ProjectRoleIDs: []string{externalRoleID}}, "", "Integration Test"); err == nil {
		t.Fatal("expected external-only access role assignment to fail")
	}
	if _, err := repo.SaveProjectMember(ctx, projectID, "", ProjectMemberMutationRequest{StaffID: staffID, RoleIDs: []string{externalLabel}, ProjectRoleIDs: []string{internalRoleID}}, "", "Integration Test"); err == nil {
		t.Fatal("expected external-only operational role assignment to fail")
	}
	assertCount(t, db, `SELECT COUNT(*) FROM project_members WHERE project_id=$1`, 0, projectID)
}

func TestProjectMemberSampleAccountIsolation(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	suffix := strings.ReplaceAll(newID()[:8], "-", "")
	roleID := newID()
	realUserID, realStaffID := newID(), newID()
	sampleUserID, sampleStaffID := newID(), newID()
	realProjectID, sampleProjectID := newID(), newID()
	roleKey := "sample_scope_" + suffix
	roleName := "Vai trò cách ly " + suffix
	projectViewPermissionID := ensurePermission(t, db, "project.view", "Xem công trình", "project")

	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM user_roles WHERE user_id IN ($1,$2)`, realUserID, sampleUserID)
		_, _ = db.ExecContext(ctx, `DELETE FROM project_members WHERE project_id IN ($1,$2)`, realProjectID, sampleProjectID)
		_, _ = db.ExecContext(ctx, `DELETE FROM audit_logs WHERE project_id IN ($1,$2)`, realProjectID, sampleProjectID)
		_, _ = db.ExecContext(ctx, `DELETE FROM projects WHERE id IN ($1,$2)`, realProjectID, sampleProjectID)
		_, _ = db.ExecContext(ctx, `DELETE FROM staff WHERE id IN ($1,$2)`, realStaffID, sampleStaffID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id IN ($1,$2)`, realUserID, sampleUserID)
		_, _ = db.ExecContext(ctx, `DELETE FROM roles WHERE id=$1`, roleID)
	})

	mustExec(t, db, `INSERT INTO roles (id,name,display_name,system_role,external_only) VALUES ($1,$2,$3,false,false)`, roleID, roleKey, roleName)
	mustExec(t, db, `INSERT INTO role_permissions (role_id,permission_id) VALUES ($1,$2)`, roleID, projectViewPermissionID)
	mustExec(t, db, `INSERT INTO users (id,email,password_hash,full_name,status) VALUES ($1,$2,'x','Người dùng thật','active')`, realUserID, "real-"+suffix+"@test.local")
	mustExec(t, db, `INSERT INTO users (id,email,password_hash,full_name,status,is_sample,sample_role_id) VALUES ($1,$2,'x',$3,'active',true,$4)`, sampleUserID, roleKey+"@dev.local", roleName, roleID)
	mustExec(t, db, `INSERT INTO staff (id,user_id,full_name,email,status) VALUES ($1,$2,'Người dùng thật',$3,'active'),($4,$5,$6,$7,'active')`, realStaffID, realUserID, "real-staff-"+suffix+"@test.local", sampleStaffID, sampleUserID, roleName, roleKey+"@dev.local")
	mustExec(t, db, `INSERT INTO projects (id,name,code,client_name,status) VALUES ($1,'Công trình thật',$2,'Client','planning')`, realProjectID, "REAL-"+suffix)
	mustExec(t, db, `INSERT INTO projects (id,name,code,client_name,status,is_sample,sample_profile,sample_blueprint_version) VALUES ($1,'Công trình mẫu',$2,'Client','draft',true,'draft',3)`, sampleProjectID, "SAMPLE-"+suffix)

	realOptions, err := repo.ProjectMemberOptions(ctx, realProjectID)
	if err != nil {
		t.Fatal(err)
	}
	sampleOptions, err := repo.ProjectMemberOptions(ctx, sampleProjectID)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range realOptions["staff"].([]Record) {
		if cleanID(row["id"]) == sampleStaffID {
			t.Fatalf("sample staff leaked into real project options: %#v", row)
		}
	}
	foundSample := false
	for _, row := range sampleOptions["staff"].([]Record) {
		if cleanID(row["id"]) == realStaffID {
			t.Fatalf("real staff leaked into sample project options: %#v", row)
		}
		foundSample = foundSample || cleanID(row["id"]) == sampleStaffID
	}
	if !foundSample {
		t.Fatalf("sample staff missing from sample project options: %#v", sampleOptions["staff"])
	}

	request := ProjectMemberMutationRequest{RoleIDs: []string{roleName}, ProjectRoleIDs: []string{roleID}}
	request.StaffID = realStaffID
	if _, err := repo.SaveProjectMember(ctx, sampleProjectID, "", request, "", "Integration Test"); err == nil {
		t.Fatal("expected real staff assignment to sample project to fail")
	}
	request.StaffID = sampleStaffID
	if _, err := repo.SaveProjectMember(ctx, realProjectID, "", request, "", "Integration Test"); err == nil {
		t.Fatal("expected sample staff assignment to real project to fail")
	}
	member, err := repo.SaveProjectMember(ctx, sampleProjectID, "", request, "", "Integration Test")
	if err != nil || !boolValue(member["is_sample"]) {
		t.Fatalf("sample staff assignment to sample project failed: member=%#v err=%v", member, err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO user_roles (user_id,role_id,project_id) VALUES ($1,$2,NULL)`, sampleUserID, roleID); err == nil {
		t.Fatal("database allowed a global role for a sample user")
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO project_members (project_id,staff_id,role_ids) VALUES ($1,$2,'[]'::jsonb)`, realProjectID, sampleStaffID); err == nil {
		t.Fatal("database allowed a sample member in a real project")
	}
}
