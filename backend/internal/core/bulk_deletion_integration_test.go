package core

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestPartnerBulkDeletionIsGlobalAuditedAtomicAndIdempotent(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	service := NewService(NewRepository(db), "test-secret")
	suffix := strings.ReplaceAll(newID(), "-", "")[:10]
	actorID, actorStaffID := newID(), newID()
	partnerID, stalePartnerID := newID(), newID()
	projectID := newID()
	actorEmail := "partner-delete-actor-" + suffix + "@test.local"

	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM audit_logs WHERE actor_id=$1 OR target_id IN ($2,$3,$4)`, actorID, partnerID, stalePartnerID, projectID)
		_, _ = db.ExecContext(ctx, `DELETE FROM bulk_deletion_batches WHERE actor_id=$1`, actorID)
		_, _ = db.ExecContext(ctx, `DELETE FROM projects WHERE id=$1`, projectID)
		_, _ = db.ExecContext(ctx, `DELETE FROM partners WHERE id IN ($1,$2)`, partnerID, stalePartnerID)
		_, _ = db.ExecContext(ctx, `DELETE FROM user_roles WHERE user_id=$1`, actorID)
		_, _ = db.ExecContext(ctx, `DELETE FROM staff WHERE id=$1`, actorStaffID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id=$1`, actorID)
	})

	var adminRoleID string
	if err := db.QueryRowContext(ctx, `SELECT id::text FROM roles WHERE name='admin'`).Scan(&adminRoleID); err != nil {
		t.Fatal(err)
	}
	mustExec(t, db, `INSERT INTO users (id,email,password_hash,full_name,status) VALUES ($1,$2,'demo:actor','Partner Deletion Actor','active')`, actorID, actorEmail)
	mustExec(t, db, `INSERT INTO staff (id,user_id,full_name,email,status) VALUES ($1,$2,'Partner Deletion Actor',$3,'active')`, actorStaffID, actorID, actorEmail)
	mustExec(t, db, `INSERT INTO user_roles (user_id,role_id) VALUES ($1,$2)`, actorID, adminRoleID)
	mustExec(t, db, `
		INSERT INTO partners (id,partner_code,name,partner_types,active) VALUES
			($1,$2,'Mixed Deletion Partner','["client","supplier"]'::jsonb,true),
			($3,$4,'Stale Deletion Partner','["supplier"]'::jsonb,true)
	`, partnerID, "MIXDEL-"+suffix[:6], stalePartnerID, "STALE-"+suffix[:6])
	mustExec(t, db, `INSERT INTO projects (id,name,code,client_name,partner_id,status) VALUES ($1,'Partner Reference Project',$2,'Mixed Deletion Partner',$3,'planning')`, projectID, "PDEL-"+suffix[:6], partnerID)

	preview, err := service.PreviewPartnerDeletion(ctx, actorID, PartnerDeletionPreviewRequest{PartnerIDs: []string{partnerID}})
	if err != nil {
		t.Fatal(err)
	}
	if number(preview["total_count"]) != 1 || number(preview["blocked_count"]) != 0 || number(preview["warning_count"]) < 2 {
		t.Fatalf("unexpected partner deletion preview: %#v", preview)
	}
	row := recordSlice(preview["rows"])[0]
	if groups, ok := row["catalog_groups"].([]string); !ok || len(groups) != 2 {
		t.Fatalf("mixed partner catalog scope missing: %#v", row)
	}
	request := PartnerDeletionExecuteRequest{
		PartnerDeletionPreviewRequest: PartnerDeletionPreviewRequest{PartnerIDs: []string{partnerID}},
		PreviewHash:                   fmt.Sprint(preview["preview_hash"]),
		IdempotencyKey:                newID(),
	}
	result, err := service.ExecutePartnerDeletion(ctx, actorID, actorEmail, request)
	if err != nil {
		t.Fatal(err)
	}
	if number(result["deleted_count"]) != 1 || result["idempotent_replay"] != false {
		t.Fatalf("unexpected partner deletion result: %#v", result)
	}
	replay, err := service.ExecutePartnerDeletion(ctx, actorID, actorEmail, request)
	if err != nil {
		t.Fatal(err)
	}
	if replay["idempotent_replay"] != true || number(replay["deleted_count"]) != 1 {
		t.Fatalf("unexpected partner deletion replay: %#v", replay)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM partners WHERE id=$1 AND NOT active`, 1, partnerID)
	assertCount(t, db, `SELECT COUNT(*) FROM projects WHERE id=$1 AND partner_id=$2`, 1, projectID, partnerID)
	assertCount(t, db, `SELECT COUNT(*) FROM audit_logs WHERE actor_id=$1 AND target_id=$2 AND event_type='partner_deactivated'`, 1, actorID, partnerID)
	inputRows, err := service.ListPartners(ctx, PartnerFilter{Group: "input", Status: "active"})
	if err != nil {
		t.Fatal(err)
	}
	outputRows, err := service.ListPartners(ctx, PartnerFilter{Group: "output", Status: "active"})
	if err != nil {
		t.Fatal(err)
	}
	if recordByID(inputRows, "id", partnerID) != nil || recordByID(outputRows, "id", partnerID) != nil {
		t.Fatal("globally deactivated mixed partner remained in an active catalog")
	}

	stalePreview, err := service.PreviewPartnerDeletion(ctx, actorID, PartnerDeletionPreviewRequest{PartnerIDs: []string{stalePartnerID}})
	if err != nil {
		t.Fatal(err)
	}
	mustExec(t, db, `UPDATE partners SET notes='Changed after preview',updated_at=now()+interval '1 second' WHERE id=$1`, stalePartnerID)
	_, err = service.ExecutePartnerDeletion(ctx, actorID, actorEmail, PartnerDeletionExecuteRequest{
		PartnerDeletionPreviewRequest: PartnerDeletionPreviewRequest{PartnerIDs: []string{stalePartnerID}},
		PreviewHash:                   fmt.Sprint(stalePreview["preview_hash"]),
		IdempotencyKey:                newID(),
	})
	var statusErr StatusError
	if !errors.As(err, &statusErr) || statusErr.Code != "CONFLICT" {
		t.Fatalf("stale partner preview error=%#v, want conflict", err)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM partners WHERE id=$1 AND active`, 1, stalePartnerID)
}

func TestProjectBulkDeletionPreservesHistoryChecksPermissionAndAuditsSingleDelete(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	service := NewService(NewRepository(db), "test-secret")
	suffix := strings.ReplaceAll(newID(), "-", "")[:10]
	actorID, actorStaffID := newID(), newID()
	unauthorizedID, unauthorizedStaffID := newID(), newID()
	projectID, singleProjectID := newID(), newID()
	workItemID, sourceID := newID(), newID()
	actorEmail := "project-delete-actor-" + suffix + "@test.local"

	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM audit_logs WHERE actor_id IN ($1,$2) OR project_id IN ($3,$4)`, actorID, unauthorizedID, projectID, singleProjectID)
		_, _ = db.ExecContext(ctx, `DELETE FROM bulk_deletion_batches WHERE actor_id=$1`, actorID)
		_, _ = db.ExecContext(ctx, `DELETE FROM projects WHERE id IN ($1,$2)`, projectID, singleProjectID)
		_, _ = db.ExecContext(ctx, `DELETE FROM user_roles WHERE user_id=$1`, actorID)
		_, _ = db.ExecContext(ctx, `DELETE FROM staff WHERE id IN ($1,$2)`, actorStaffID, unauthorizedStaffID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id IN ($1,$2)`, actorID, unauthorizedID)
	})

	var adminRoleID string
	if err := db.QueryRowContext(ctx, `SELECT id::text FROM roles WHERE name='admin'`).Scan(&adminRoleID); err != nil {
		t.Fatal(err)
	}
	mustExec(t, db, `INSERT INTO users (id,email,password_hash,full_name,status) VALUES ($1,$2,'demo:actor','Project Deletion Actor','active'),($3,$4,'demo:unauthorized','Unauthorized Actor','active')`, actorID, actorEmail, unauthorizedID, "unauthorized-"+suffix+"@test.local")
	mustExec(t, db, `INSERT INTO staff (id,user_id,full_name,email,status) VALUES ($1,$2,'Project Deletion Actor',$3,'active'),($4,$5,'Unauthorized Actor',$6,'active')`, actorStaffID, actorID, actorEmail, unauthorizedStaffID, unauthorizedID, "unauthorized-"+suffix+"@test.local")
	mustExec(t, db, `INSERT INTO user_roles (user_id,role_id) VALUES ($1,$2)`, actorID, adminRoleID)
	mustExec(t, db, `INSERT INTO projects (id,name,code,client_name,status) VALUES ($1,'Bulk Deletion Project',$2,'Client','construction'),($3,'Single Deletion Project',$4,'Client','planning')`, projectID, "BULKDEL-"+suffix[:5], singleProjectID, "SINGLE-"+suffix[:5])
	mustExec(t, db, `INSERT INTO work_items (id,project_id,source_type,source_id,title,status,created_by,updated_by) VALUES ($1,$2,'issue',$3,'Open deletion work item','in_progress',$4,$4)`, workItemID, projectID, sourceID, actorID)

	_, err := service.PreviewProjectDeletion(ctx, unauthorizedID, ProjectDeletionPreviewRequest{ProjectIDs: []string{projectID}})
	var forbidden StatusError
	if !errors.As(err, &forbidden) || forbidden.Code != "FORBIDDEN" {
		t.Fatalf("unauthorized project deletion error=%#v, want forbidden", err)
	}

	preview, err := service.PreviewProjectDeletion(ctx, actorID, ProjectDeletionPreviewRequest{ProjectIDs: []string{projectID}})
	if err != nil {
		t.Fatal(err)
	}
	if number(preview["total_count"]) != 1 || number(preview["warning_count"]) < 1 {
		t.Fatalf("unexpected project deletion preview: %#v", preview)
	}
	request := ProjectDeletionExecuteRequest{
		ProjectDeletionPreviewRequest: ProjectDeletionPreviewRequest{ProjectIDs: []string{projectID}},
		PreviewHash:                   fmt.Sprint(preview["preview_hash"]),
		IdempotencyKey:                newID(),
	}
	result, err := service.ExecuteProjectDeletion(ctx, actorID, actorEmail, request)
	if err != nil {
		t.Fatal(err)
	}
	if number(result["deleted_count"]) != 1 {
		t.Fatalf("unexpected project deletion result: %#v", result)
	}
	replay, err := service.ExecuteProjectDeletion(ctx, actorID, actorEmail, request)
	if err != nil {
		t.Fatal(err)
	}
	if replay["idempotent_replay"] != true {
		t.Fatalf("project deletion replay was not idempotent: %#v", replay)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM projects WHERE id=$1 AND status='cancelled'`, 1, projectID)
	assertCount(t, db, `SELECT COUNT(*) FROM work_items WHERE id=$1 AND project_id=$2 AND status='in_progress'`, 1, workItemID, projectID)
	assertCount(t, db, `SELECT COUNT(*) FROM audit_logs WHERE project_id=$1 AND target_id=$1 AND event_type='project_cancelled'`, 1, projectID)

	if err := service.DeleteProject(ctx, singleProjectID, actorID, actorEmail); err != nil {
		t.Fatal(err)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM projects WHERE id=$1 AND status='cancelled'`, 1, singleProjectID)
	assertCount(t, db, `SELECT COUNT(*) FROM audit_logs WHERE project_id=$1 AND event_type='project_cancelled' AND metadata_json->>'source'='single_delete_api'`, 1, singleProjectID)
}
