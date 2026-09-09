package core

import (
	"context"
	"testing"
)

func TestDiscussionWorkUpdateCannotBeForged(t *testing.T) {
	for _, req := range []WorkItemActivityRequest{
		{ActivityType: "work_update", Comment: "fake release"},
		{ActivityType: "comment", Comment: "fake source", Metadata: Record{"work_update": Record{"source_id": newID()}}},
	} {
		if _, err := validateActivityRequest(req); err == nil {
			t.Fatal("chat accepted server-owned update data")
		}
	}
}

func TestDiscussionWorkUpdateEvidenceVisibilityReplyAndUnread(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")
	fixture := setupWorkItemFlowFixture(t, db, false)
	outsider := setupWorkItemFlowFixture(t, db, false)
	evidence, err := service.AddWorkItemEvidence(ctx, fixture.itemID, WorkItemEvidenceRequest{
		EvidenceType: "document", Name: "Bản vẽ gửi lần đầu", FileURL: "https://example.com/design.pdf", Caption: "Chưa rà soát",
	}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	update, err := repo.QueryOne(ctx, `SELECT a.* FROM work_item_activities a JOIN activity_work_updates u ON u.activity_id=a.id WHERE u.source_id=$1`, evidence["id"])
	if err != nil {
		t.Fatal(err)
	}
	activityID := cleanID(update["id"])
	detail, err := service.DiscussionWorkUpdate(ctx, activityID, fixture.pmID)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := recordFromAny(detail["snapshot"])
	if cleanID(snapshot["evidence_id"]) != cleanID(evidence["id"]) || snapshot["status"] != "submitted" {
		t.Fatalf("wrong evidence snapshot: %#v", snapshot)
	}
	if _, err := service.DiscussionWorkUpdate(ctx, activityID, outsider.workerID); err == nil {
		t.Fatal("outsider read source snapshot")
	}
	rows, err := service.ProjectActivity(ctx, fixture.projectID, ProjectActivityFilter{Kind: "discussion", Limit: 30}, fixture.pmID)
	if err != nil {
		t.Fatal(err)
	}
	row := recordByID(rows, "id", activityID)
	if row == nil || !boolValue(row["is_discussion_unread"]) {
		t.Fatalf("update missing from unread discussion: %#v", rows)
	}
	if boolValue(row["can_edit"]) || boolValue(row["can_withdraw"]) {
		t.Fatal("source event exposed message editing")
	}
	reply, err := service.AddProjectActivity(ctx, fixture.projectID, WorkItemActivityRequest{Comment: "Cần chỉnh mặt bằng", ParentActivityID: activityID}, fixture.pmID, "PM")
	if err != nil {
		t.Fatal(err)
	}
	if cleanID(reply["work_item_id"]) != fixture.itemID || cleanID(reply["parent_activity_id"]) != activityID || reply["visibility"] != "internal" {
		t.Fatalf("reply lost source scope: %#v", reply)
	}
	if _, err := service.SetActivityReaction(ctx, activityID, ActivityReactionRequest{Reaction: "👀", Active: true}, fixture.pmID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.MarkProjectDiscussionReadThrough(ctx, fixture.projectID, ProjectDiscussionReadRequest{ActivityID: activityID}, fixture.pmID); err != nil {
		t.Fatal(err)
	}
	rows, err = service.ProjectActivity(ctx, fixture.projectID, ProjectActivityFilter{Kind: "discussion", Limit: 30}, fixture.pmID)
	if err != nil {
		t.Fatal(err)
	}
	if boolValue(recordByID(rows, "id", activityID)["is_discussion_unread"]) {
		t.Fatal("update remains unread after read-through")
	}
	assertCount(t, db, `SELECT COUNT(*) FROM evidence_records WHERE id=$1 AND status='submitted'`, 1, evidence["id"])
	assertCount(t, db, `SELECT COUNT(*) FROM work_items WHERE id=$1 AND status='ready'`, 1, fixture.itemID)
	if _, err := db.ExecContext(ctx, `UPDATE activity_work_updates SET source_snapshot_json='{}' WHERE activity_id=$1`, activityID); err == nil {
		t.Fatal("snapshot is mutable")
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if err := insertDiscussionWorkUpdateTx(ctx, tx, fixture.projectID, fixture.itemID, fixture.workerID, "evidence", cleanID(evidence["id"]), "uploaded", "duplicate", Record{}); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM activity_work_updates WHERE source_id=$1`, 1, evidence["id"])
	// The source and its discussion event roll back together.
	tx, err = db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	fakeID := newID()
	if err := insertDiscussionWorkUpdateTx(ctx, tx, fixture.projectID, fixture.itemID, fixture.workerID, "evidence", fakeID, "uploaded", "rollback", Record{}); err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM activity_work_updates WHERE source_id=$1`, 0, fakeID)
	assertCount(t, db, `SELECT COUNT(*) FROM work_item_activities WHERE project_id=$1 AND comment='rollback'`, 0, fixture.projectID)
}

func TestDiscussionWorkflowSnapshotAndConcurrentDraft(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	fixture := setupWorkItemFlowFixture(t, db, false)
	instanceID := newID()
	mustExec(t, db, `INSERT INTO workflow_form_instances (id,project_id,work_item_id,form_template_id,form_version_id,form_type,title,created_by)
  SELECT $1,$2,$3,t.id,v.id,t.form_type,'Checklist thảo luận',$4 FROM workflow_form_templates t JOIN workflow_form_versions v ON v.template_id=t.id
  WHERE t.code='builtin_checklist' AND v.version_no=1`, instanceID, fixture.projectID, fixture.itemID, fixture.workerID)
	initial := Record{"exceptions": "Nháp đầu"}
	if _, err := repo.SaveWorkflowForm(ctx, instanceID, initial, "", false, fixture.workerID, "Worker", Record{}); err != nil {
		t.Fatal(err)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM activity_work_updates WHERE source_id=$1`, 0, instanceID)
	if _, err := repo.SaveWorkflowForm(ctx, instanceID, Record{"exceptions": "stale"}, "", false, fixture.workerID, "Worker", Record{}); err == nil {
		t.Fatal("stale draft silently overwrote saved answers")
	}
	assertCount(t, db, `SELECT COUNT(*) FROM workflow_form_instances WHERE id=$1 AND answers_snapshot_json->>'exceptions'='Nháp đầu'`, 1, instanceID)
	if _, err := repo.SaveWorkflowForm(ctx, instanceID, initial, "", true, fixture.workerID, "Worker", initial); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.AmendWorkflowForm(ctx, instanceID, Record{"exceptions": "Chỉnh lần hai"}, "Bổ sung ghi chú", 1, fixture.workerID, "Worker"); err != nil {
		t.Fatal(err)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM activity_work_updates WHERE source_id=$1`, 2, instanceID)
	assertCount(t, db, `SELECT COUNT(*) FROM activity_work_updates WHERE source_id=$1 AND source_snapshot_json->>'revision_no'='1' AND source_snapshot_json->'answers'->>'exceptions'='Nháp đầu' AND jsonb_array_length(source_snapshot_json->'fields')>0`, 1, instanceID)
	assertCount(t, db, `SELECT COUNT(*) FROM activity_work_updates WHERE source_id=$1 AND source_snapshot_json->>'revision_no'='2' AND source_snapshot_json->'answers'->>'exceptions'='Chỉnh lần hai'`, 1, instanceID)
	assertCount(t, db, `SELECT COUNT(*) FROM work_items WHERE id=$1 AND status='in_progress'`, 1, fixture.itemID)
}
