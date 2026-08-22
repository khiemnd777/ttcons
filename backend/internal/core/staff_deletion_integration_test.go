package core

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestStaffDeletionPreservesReferencesRevokesAccessAndLabelsDeletedIdentity(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")
	suffix := strings.ReplaceAll(newID(), "-", "")[:10]
	actorID := newID()
	actorStaffID := newID()
	targetUserID := newID()
	targetStaffID := newID()
	projectID := newID()
	workItemID := newID()
	workSourceID := newID()
	actionID := newID()
	recurringPlanID := newID()
	recurringOccurrenceID := newID()
	actorEmail := "staff-delete-actor-" + suffix + "@test.local"
	targetEmail := "staff-delete-target-" + suffix + "@test.local"

	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM audit_logs WHERE actor_id=$1 OR target_id IN ($2,$3)`, actorID, targetStaffID, targetUserID)
		_, _ = db.ExecContext(ctx, `DELETE FROM staff_deletion_batches WHERE actor_id=$1`, actorID)
		_, _ = db.ExecContext(ctx, `DELETE FROM projects WHERE id=$1`, projectID)
		_, _ = db.ExecContext(ctx, `DELETE FROM auth_email_tokens WHERE user_id IN ($1,$2)`, actorID, targetUserID)
		_, _ = db.ExecContext(ctx, `DELETE FROM auth_sessions WHERE user_id IN ($1,$2)`, actorID, targetUserID)
		_, _ = db.ExecContext(ctx, `DELETE FROM user_roles WHERE user_id IN ($1,$2)`, actorID, targetUserID)
		_, _ = db.ExecContext(ctx, `DELETE FROM staff WHERE id IN ($1,$2)`, actorStaffID, targetStaffID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id IN ($1,$2)`, actorID, targetUserID)
	})

	var adminRoleID, pmRoleID string
	if err := db.QueryRowContext(ctx, `SELECT id::text FROM roles WHERE name='admin'`).Scan(&adminRoleID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT id::text FROM roles WHERE name='pm'`).Scan(&pmRoleID); err != nil {
		t.Fatal(err)
	}
	mustExec(t, db, `INSERT INTO users (id,email,password_hash,full_name,status) VALUES ($1,$2,'demo:actor','Deletion Actor','active')`, actorID, actorEmail)
	mustExec(t, db, `INSERT INTO users (id,email,password_hash,full_name,status,recovery_email,recovery_email_verified_at,must_change_password) VALUES ($1,$2,'demo:target','Deleted Project Manager','active',$3,now(),true)`, targetUserID, targetEmail, "recovery-"+suffix+"@test.local")
	mustExec(t, db, `INSERT INTO staff (id,user_id,full_name,email,department,position,status) VALUES ($1,$2,'Deletion Actor',$3,'Công nghệ','Admin','active')`, actorStaffID, actorID, actorEmail)
	mustExec(t, db, `INSERT INTO staff (id,user_id,full_name,email,department,position,status) VALUES ($1,$2,'Deleted Project Manager',$3,'Dự án','PM','active')`, targetStaffID, targetUserID, targetEmail)
	mustExec(t, db, `INSERT INTO user_roles (user_id,role_id) VALUES ($1,$2),($3,$4)`, actorID, adminRoleID, targetUserID, pmRoleID)
	mustExec(t, db, `INSERT INTO projects (id,name,code,client_name,status,pm_staff_id) VALUES ($1,'Deletion Project',$2,'Client','construction',$3)`, projectID, "DEL-"+suffix[:7], targetStaffID)
	mustExec(t, db, `INSERT INTO project_members (project_id,staff_id,role_ids,active) VALUES ($1,$2,'["PM"]'::jsonb,true),($1,$3,'["Admin"]'::jsonb,true)`, projectID, targetStaffID, actorStaffID)
	mustExec(t, db, `INSERT INTO work_items (id,project_id,source_type,source_id,title,status,assignee_staff_id,created_by,updated_by) VALUES ($1,$2,'issue',$3,'Deletion work item','in_progress',$4,$5,$5)`, workItemID, projectID, workSourceID, targetStaffID, actorID)
	mustExec(t, db, `INSERT INTO construction_action_items (id,project_id,work_item_id,title,assigned_user_id,status,created_by) VALUES ($1,$2,$3,'Deletion action',$4,'open',$5)`, actionID, projectID, workItemID, targetUserID, actorID)
	mustExec(t, db, `INSERT INTO construction_recurring_plans (id,project_id,plan_code,title,recurrence_type,start_date,next_due_date,responsible_role,assigned_user_id,status,created_by) VALUES ($1,$2,$3,'Deletion recurring plan','daily',current_date,current_date,'Giám sát',$4,'active',$5)`, recurringPlanID, projectID, "DEL-QA-"+suffix[:5], targetUserID, actorID)
	mustExec(t, db, `INSERT INTO construction_recurring_occurrences (id,plan_id,project_id,occurrence_date,assigned_user_id,status) VALUES ($1,$2,$3,current_date,$4,'pending')`, recurringOccurrenceID, recurringPlanID, projectID, targetUserID)
	mustExec(t, db, `INSERT INTO auth_sessions (user_id,refresh_token_hash,access_token_id,expires_at) VALUES ($1,$2,$3,now()+interval '1 day')`, targetUserID, strings.Repeat("a", 64), "staff-delete-access-"+suffix)
	mustExec(t, db, `INSERT INTO auth_email_tokens (user_id,purpose,token_hash,target_email,expires_at) VALUES ($1,'reset_password',$2,$3,now()+interval '1 hour')`, targetUserID, strings.Repeat("b", 64), targetEmail)

	preview, err := service.PreviewStaffDeletion(ctx, actorID, StaffDeletionPreviewRequest{StaffIDs: []string{targetStaffID}})
	if err != nil {
		t.Fatal(err)
	}
	if number(preview["total_count"]) != 1 || number(preview["blocked_count"]) != 0 || number(preview["warning_count"]) < 3 {
		t.Fatalf("unexpected deletion preview: %#v", preview)
	}
	previewRow := recordSlice(preview["rows"])[0]
	if len(recordSlice(nestedRecordValue(previewRow, "impact", "other_responsibilities"))) < 3 {
		t.Fatalf("live user-linked responsibilities missing from preview: %#v", previewRow)
	}
	previewHash := fmt.Sprint(preview["preview_hash"])
	idempotencyKey := newID()
	executeRequest := StaffDeletionExecuteRequest{
		StaffDeletionPreviewRequest: StaffDeletionPreviewRequest{StaffIDs: []string{targetStaffID}},
		PreviewHash:                 previewHash,
		IdempotencyKey:              idempotencyKey,
	}
	result, err := service.ExecuteStaffDeletion(ctx, actorID, actorEmail, executeRequest)
	if err != nil {
		t.Fatal(err)
	}
	if number(result["deleted_count"]) != 1 || number(result["memberships_deactivated"]) != 1 || number(result["sessions_revoked"]) != 1 || number(result["roles_removed"]) != 1 {
		t.Fatalf("unexpected deletion result: %#v", result)
	}
	replay, err := service.ExecuteStaffDeletion(ctx, actorID, actorEmail, executeRequest)
	if err != nil {
		t.Fatal(err)
	}
	if replay["idempotent_replay"] != true || number(replay["deleted_count"]) != 1 {
		t.Fatalf("unexpected deletion replay: %#v", replay)
	}

	assertCount(t, db, `SELECT COUNT(*) FROM projects WHERE id=$1 AND pm_staff_id=$2`, 1, projectID, targetStaffID)
	assertCount(t, db, `SELECT COUNT(*) FROM work_items WHERE id=$1 AND assignee_staff_id=$2`, 1, workItemID, targetStaffID)
	assertCount(t, db, `SELECT COUNT(*) FROM project_members WHERE project_id=$1 AND staff_id=$2 AND NOT active`, 1, projectID, targetStaffID)
	assertCount(t, db, `SELECT COUNT(*) FROM auth_sessions WHERE user_id=$1 AND revoked_at IS NOT NULL`, 1, targetUserID)
	assertCount(t, db, `SELECT COUNT(*) FROM auth_email_tokens WHERE user_id=$1 AND invalidated_at IS NOT NULL`, 1, targetUserID)
	assertCount(t, db, `SELECT COUNT(*) FROM user_roles WHERE user_id=$1`, 0, targetUserID)
	assertCount(t, db, `SELECT COUNT(*) FROM staff WHERE id=$1 AND status='deleted' AND deleted_at IS NOT NULL AND deleted_by=$2`, 1, targetStaffID, actorID)
	assertCount(t, db, `SELECT COUNT(*) FROM users WHERE id=$1 AND status='inactive' AND deleted_at IS NOT NULL AND deleted_by=$2 AND recovery_email IS NULL AND NOT must_change_password`, 1, targetUserID, actorID)
	if _, err := db.ExecContext(ctx, `INSERT INTO users (email,password_hash,full_name,status) VALUES ($1,'demo:duplicate','Duplicate','active')`, targetEmail); err == nil {
		t.Fatal("deleted login email was unexpectedly reusable")
	}

	projects, err := repo.ListProjects(ctx, actorID)
	if err != nil {
		t.Fatal(err)
	}
	project := recordByID(projects, "id", projectID)
	if project == nil || project["pm_account_deleted"] != true {
		t.Fatalf("project manager deleted marker missing: %#v", project)
	}
	explorerRows, err := repo.ProjectExplorerRows(ctx, []string{projectID})
	if err != nil {
		t.Fatal(err)
	}
	if explorerRows[projectID]["pm_account_deleted"] != true {
		t.Fatalf("project explorer deleted marker missing: %#v", explorerRows[projectID])
	}
	item, err := repo.GetWorkItemDetail(ctx, workItemID)
	if err != nil {
		t.Fatal(err)
	}
	if item["assignee_account_deleted"] != true {
		t.Fatalf("work item assignee deleted marker missing: %#v", item)
	}
	constructionWorkspace, err := repo.ProjectConstructionWorkspace(ctx, projectID)
	if err != nil {
		t.Fatal(err)
	}
	action := recordByID(recordSlice(constructionWorkspace["actions"]), "id", actionID)
	if action == nil || action["assigned_user_account_deleted"] != true {
		t.Fatalf("construction action assignee deleted marker missing: %#v", action)
	}
	qualityWorkspace, err := repo.ProjectQualityWorkspace(ctx, projectID)
	if err != nil {
		t.Fatal(err)
	}
	occurrence := recordByID(recordSlice(qualityWorkspace["recurring_occurrences"]), "id", recurringOccurrenceID)
	if occurrence == nil || occurrence["assigned_user_account_deleted"] != true {
		t.Fatalf("recurring occurrence assignee deleted marker missing: %#v", occurrence)
	}
	members, err := repo.ListProjectMembers(ctx, projectID, true)
	if err != nil {
		t.Fatal(err)
	}
	deletedMember := recordByID(members, "staff_id", targetStaffID)
	if deletedMember == nil || deletedMember["account_deleted"] != true || deletedMember["active"] != false {
		t.Fatalf("deleted project member history missing: %#v", deletedMember)
	}
	staffRows, err := service.ListStaff(ctx, actorID)
	if err != nil {
		t.Fatal(err)
	}
	if recordByID(staffRows, "id", targetStaffID) != nil {
		t.Fatal("deleted staff remained visible in active staff list")
	}

	selfPreview, err := service.PreviewStaffDeletion(ctx, actorID, StaffDeletionPreviewRequest{StaffIDs: []string{actorStaffID}})
	if err != nil {
		t.Fatal(err)
	}
	if number(selfPreview["blocked_count"]) != 1 {
		t.Fatalf("self deletion was not blocked: %#v", selfPreview)
	}
}

func TestStaffDeletionKeepsSharedAccountUntilEveryProfileIsDeleted(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")
	suffix := strings.ReplaceAll(newID(), "-", "")[:10]
	actorID, actorStaffID := newID(), newID()
	targetUserID, firstStaffID, secondStaffID := newID(), newID(), newID()
	actorEmail := "staff-delete-shared-actor-" + suffix + "@test.local"
	targetEmail := "staff-delete-shared-target-" + suffix + "@test.local"

	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM audit_logs WHERE actor_id=$1`, actorID)
		_, _ = db.ExecContext(ctx, `DELETE FROM staff_deletion_batches WHERE actor_id=$1`, actorID)
		_, _ = db.ExecContext(ctx, `DELETE FROM auth_sessions WHERE user_id=$1`, targetUserID)
		_, _ = db.ExecContext(ctx, `DELETE FROM user_roles WHERE user_id IN ($1,$2)`, actorID, targetUserID)
		_, _ = db.ExecContext(ctx, `DELETE FROM staff WHERE id IN ($1,$2,$3)`, actorStaffID, firstStaffID, secondStaffID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id IN ($1,$2)`, actorID, targetUserID)
	})

	var adminRoleID, pmRoleID string
	if err := db.QueryRowContext(ctx, `SELECT id::text FROM roles WHERE name='admin'`).Scan(&adminRoleID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT id::text FROM roles WHERE name='pm'`).Scan(&pmRoleID); err != nil {
		t.Fatal(err)
	}
	mustExec(t, db, `INSERT INTO users (id,email,password_hash,full_name,status) VALUES ($1,$2,'demo:actor','Shared Actor','active'),($3,$4,'demo:target','Shared Target','active')`, actorID, actorEmail, targetUserID, targetEmail)
	mustExec(t, db, `INSERT INTO staff (id,user_id,full_name,email,status) VALUES ($1,$2,'Shared Actor',$3,'active'),($4,$5,'Shared Target A',$6,'active'),($7,$5,'Shared Target B',$8,'active')`, actorStaffID, actorID, actorEmail, firstStaffID, targetUserID, targetEmail, secondStaffID, "second-"+targetEmail)
	mustExec(t, db, `INSERT INTO user_roles (user_id,role_id) VALUES ($1,$2),($3,$4)`, actorID, adminRoleID, targetUserID, pmRoleID)
	mustExec(t, db, `INSERT INTO auth_sessions (user_id,refresh_token_hash,access_token_id,expires_at) VALUES ($1,$2,$3,now()+interval '1 day')`, targetUserID, strings.Repeat("c", 64), "staff-delete-shared-"+suffix)

	preview, err := service.PreviewStaffDeletion(ctx, actorID, StaffDeletionPreviewRequest{StaffIDs: []string{firstStaffID}})
	if err != nil {
		t.Fatal(err)
	}
	row := recordSlice(preview["rows"])[0]
	if row["will_delete_account"] != false {
		t.Fatalf("shared account should remain until its last active profile is selected: %#v", row)
	}
	_, err = service.ExecuteStaffDeletion(ctx, actorID, actorEmail, StaffDeletionExecuteRequest{
		StaffDeletionPreviewRequest: StaffDeletionPreviewRequest{StaffIDs: []string{firstStaffID}},
		PreviewHash:                 fmt.Sprint(preview["preview_hash"]),
		IdempotencyKey:              newID(),
	})
	if err != nil {
		t.Fatal(err)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM staff WHERE id=$1 AND status='deleted'`, 1, firstStaffID)
	assertCount(t, db, `SELECT COUNT(*) FROM staff WHERE id=$1 AND status='active'`, 1, secondStaffID)
	assertCount(t, db, `SELECT COUNT(*) FROM users WHERE id=$1 AND status='active' AND deleted_at IS NULL`, 1, targetUserID)
	assertCount(t, db, `SELECT COUNT(*) FROM auth_sessions WHERE user_id=$1 AND revoked_at IS NULL`, 1, targetUserID)
	assertCount(t, db, `SELECT COUNT(*) FROM user_roles WHERE user_id=$1`, 1, targetUserID)
}

func TestStaffDeletionBlockedBatchIsAtomic(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")
	suffix := strings.ReplaceAll(newID(), "-", "")[:10]
	actorID, actorStaffID := newID(), newID()
	targetUserID, targetStaffID := newID(), newID()
	actorEmail := "staff-delete-atomic-actor-" + suffix + "@test.local"
	targetEmail := "staff-delete-atomic-target-" + suffix + "@test.local"

	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM user_roles WHERE user_id=$1`, actorID)
		_, _ = db.ExecContext(ctx, `DELETE FROM staff WHERE id IN ($1,$2)`, actorStaffID, targetStaffID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id IN ($1,$2)`, actorID, targetUserID)
	})
	var adminRoleID string
	if err := db.QueryRowContext(ctx, `SELECT id::text FROM roles WHERE name='admin'`).Scan(&adminRoleID); err != nil {
		t.Fatal(err)
	}
	mustExec(t, db, `INSERT INTO users (id,email,password_hash,full_name,status) VALUES ($1,$2,'demo:actor','Atomic Actor','active'),($3,$4,'demo:target','Atomic Target','active')`, actorID, actorEmail, targetUserID, targetEmail)
	mustExec(t, db, `INSERT INTO staff (id,user_id,full_name,email,status) VALUES ($1,$2,'Atomic Actor',$3,'active'),($4,$5,'Atomic Target',$6,'active')`, actorStaffID, actorID, actorEmail, targetStaffID, targetUserID, targetEmail)
	mustExec(t, db, `INSERT INTO user_roles (user_id,role_id) VALUES ($1,$2)`, actorID, adminRoleID)

	request := StaffDeletionPreviewRequest{StaffIDs: []string{actorStaffID, targetStaffID}}
	preview, err := service.PreviewStaffDeletion(ctx, actorID, request)
	if err != nil {
		t.Fatal(err)
	}
	if number(preview["blocked_count"]) != 1 {
		t.Fatalf("self-delete batch should be blocked before execution: %#v", preview)
	}
	_, err = service.ExecuteStaffDeletion(ctx, actorID, actorEmail, StaffDeletionExecuteRequest{
		StaffDeletionPreviewRequest: request,
		PreviewHash:                 fmt.Sprint(preview["preview_hash"]),
		IdempotencyKey:              newID(),
	})
	if err == nil {
		t.Fatal("blocked deletion batch unexpectedly executed")
	}
	assertCount(t, db, `SELECT COUNT(*) FROM staff WHERE id IN ($1,$2) AND status='active'`, 2, actorStaffID, targetStaffID)
	assertCount(t, db, `SELECT COUNT(*) FROM users WHERE id=$1 AND status='active' AND deleted_at IS NULL`, 1, targetUserID)
}

func nestedRecordValue(record Record, keys ...string) any {
	var current any = record
	for _, key := range keys {
		next, ok := current.(Record)
		if !ok {
			return nil
		}
		current = next[key]
	}
	return current
}
