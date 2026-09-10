package core

import (
	"context"
	"testing"
)

func TestDesignReleaseRequiresExplicitPMRehandoffForEveryPublishedRevision(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	actorID := newID()
	projectID := newID()
	workItemID := newID()
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM projects WHERE id=$1`, projectID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id=$1`, actorID)
	})
	mustExec(t, db, `INSERT INTO users (id,email,password_hash,full_name,status) VALUES ($1,$2,'demo:pw','Design Release Actor','active')`, actorID, "design-release-"+actorID+"@test.local")
	mustExec(t, db, `INSERT INTO projects (id,name,code,client_name,status) VALUES ($1,'Design Revision Project',$2,'Client','planning')`, projectID, "DES-"+projectID[:8])
	mustExec(t, db, `INSERT INTO work_items (id,project_id,source_type,source_id,title,work_type,status,created_by,updated_by) VALUES ($1,$2,'issue',$3,'Nộp hồ sơ thiết kế','document_upload','ready',$4,$4)`, workItemID, projectID, newID(), actorID)

	first, err := repo.CreateDesignRelease(ctx, projectID, workItemID, "R01", "Phát hành thiết kế lần đầu", "", actorID, "Design Release Actor")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.UpdateDesignReleaseDraft(ctx, cleanID(first["id"]), "R01", "Overwrite", "", actorID, "Actor", Record{"revision_label": "R01", "change_reason": "stale", "notes": ""}); err == nil {
		t.Fatal("stale design draft overwrote the current package")
	}
	firstEvidence, err := repo.AddDesignReleaseFile(ctx, cleanID(first["id"]), "layout", "layout-r01.pdf", WorkItemEvidenceRequest{
		EvidenceType: "document", DocumentType: "layout", Name: "Layout R01", Title: "Layout R01", Visibility: "internal",
		StorageProvider: "local", StorageKey: "tests/design/" + cleanID(first["id"]) + "/layout-r01.pdf", MIMEType: "application/pdf", FileSize: 100, ChecksumSHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}, actorID, "Design Release Actor")
	if err != nil {
		t.Fatal(err)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM design_release_items WHERE design_release_id=$1 AND evidence_id=$2`, 1, first["id"], firstEvidence["id"])
	assertCount(t, db, `SELECT COUNT(*) FROM documents WHERE target_type='design_release' AND target_id=$1`, 0, first["id"])
	if _, err := db.ExecContext(ctx, `INSERT INTO documents (project_id,target_type,target_id,name,document_type,status) VALUES ($1,'design_release',$2,'Legacy write','layout','active')`, projectID, first["id"]); err == nil {
		t.Fatal("legacy design document insert remained writable")
	}
	draftSummary, err := repo.ProjectEvidenceSummary(ctx, projectID, workItemAccessProfile{}, true)
	if err != nil {
		t.Fatal(err)
	}
	if number(draftSummary["total"]) != 1 || number(draftSummary["waiting"]) != 0 {
		t.Fatalf("draft design evidence summary = %#v, want visible but not waiting", draftSummary)
	}
	if _, err := NewService(repo, "test-secret").EvidenceAction(ctx, cleanID(firstEvidence["id"]), "accept", ActionRequest{}, actorID, "Design Release Actor"); err == nil {
		t.Fatal("revision-managed evidence allowed a generic per-file review")
	}
	first, err = repo.ReleaseDesignRelease(ctx, cleanID(first["id"]), actorID, "Design Release Actor")
	if err != nil {
		t.Fatal(err)
	}
	releasedSummary, err := repo.ProjectEvidenceSummary(ctx, projectID, workItemAccessProfile{}, true)
	if err != nil {
		t.Fatal(err)
	}
	if number(releasedSummary["waiting"]) != 1 {
		t.Fatalf("released design evidence summary = %#v, want waiting", releasedSummary)
	}

	outcome, err := repo.automateClientOperationalSourceGate(ctx, Record{"project_id": projectID, "id": workItemID}, "source.design_documents_ready")
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Status != "blocked" || outcome.TargetType != "design_release" {
		t.Fatalf("gate before handoff = %#v, want blocked design_release", outcome)
	}
	if _, err := repo.HandoffDesignRelease(ctx, cleanID(first["id"]), "", "Bàn giao R01", actorID, "Design Release Actor"); err != nil {
		t.Fatal(err)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM evidence_records WHERE id=$1 AND status='accepted' AND reviewed_by=$2`, 1, firstEvidence["id"], actorID)
	handedOffSummary, err := repo.ProjectEvidenceSummary(ctx, projectID, workItemAccessProfile{}, true)
	if err != nil {
		t.Fatal(err)
	}
	if number(handedOffSummary["waiting"]) != 0 || number(handedOffSummary["accepted"]) != 1 {
		t.Fatalf("handed-off design evidence summary = %#v", handedOffSummary)
	}
	outcome, err = repo.automateClientOperationalSourceGate(ctx, Record{"project_id": projectID, "id": workItemID}, "source.design_documents_ready")
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Status != "succeeded" || outcome.TargetType != "design_release" || outcome.TargetID != cleanID(first["id"]) {
		t.Fatalf("gate after R01 handoff = %#v, want succeeded pinned R01", outcome)
	}

	second, err := repo.CreateDesignRelease(ctx, projectID, workItemID, "R02", "Điều chỉnh layout theo phản hồi client", "Không thay đổi BOQ tự động", actorID, "Design Release Actor")
	if err != nil {
		t.Fatal(err)
	}
	secondEvidence, err := repo.AddDesignReleaseFile(ctx, cleanID(second["id"]), "construction_drawing", "drawing-r02.pdf", WorkItemEvidenceRequest{
		EvidenceType: "document", DocumentType: "construction_drawing", Name: "Bản vẽ R02", Title: "Bản vẽ R02", Visibility: "internal",
		StorageProvider: "local", StorageKey: "tests/design/" + cleanID(second["id"]) + "/drawing-r02.pdf", MIMEType: "application/pdf", FileSize: 120, ChecksumSHA256: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	}, actorID, "Design Release Actor")
	if err != nil {
		t.Fatal(err)
	}
	second, err = repo.ReleaseDesignRelease(ctx, cleanID(second["id"]), actorID, "Design Release Actor")
	if err != nil {
		t.Fatal(err)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM design_releases WHERE id=$1 AND status='superseded'`, 1, cleanID(first["id"]))
	assertCount(t, db, `SELECT COUNT(*) FROM design_releases WHERE id=$1 AND status='released'`, 1, cleanID(second["id"]))
	assertCount(t, db, `SELECT COUNT(*) FROM evidence_records WHERE id=$1 AND status='submitted'`, 1, secondEvidence["id"])

	outcome, err = repo.automateClientOperationalSourceGate(ctx, Record{"project_id": projectID, "id": workItemID}, "source.design_documents_ready")
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Status != "blocked" || outcome.TargetID != cleanID(second["id"]) {
		t.Fatalf("gate after R02 release = %#v, want blocked pending re-handoff", outcome)
	}
	if _, err := repo.HandoffDesignRelease(ctx, cleanID(second["id"]), "", "Bàn giao lại R02", actorID, "Design Release Actor"); err != nil {
		t.Fatal(err)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM evidence_records WHERE id=$1 AND status='accepted' AND reviewed_by=$2`, 1, secondEvidence["id"], actorID)
	outcome, err = repo.automateClientOperationalSourceGate(ctx, Record{"project_id": projectID, "id": workItemID}, "source.design_documents_ready")
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Status != "succeeded" || outcome.TargetID != cleanID(second["id"]) {
		t.Fatalf("gate after R02 handoff = %#v, want succeeded pinned R02", outcome)
	}
	if _, err := db.ExecContext(ctx, `UPDATE design_releases SET notes='mutated after release' WHERE id=$1`, cleanID(second["id"])); err == nil {
		t.Fatal("released design revision allowed a content mutation")
	}
	assertCount(t, db, `SELECT COUNT(*) FROM audit_logs WHERE project_id=$1 AND event_type IN ('design_release_created','design_release_file_uploaded','design_release_released','design_release_handed_off')`, 8, projectID)
	assertCount(t, db, `SELECT COUNT(*) FROM activity_work_updates WHERE source_id IN ($1,$2)`, 6, first["id"], second["id"])
	assertCount(t, db, `SELECT COUNT(*) FROM activity_work_updates WHERE source_id=$1 AND source_snapshot_json->>'status'='released' AND source_snapshot_json->>'revision_label'='R01' AND source_snapshot_json->'files'->0->>'name'='Layout R01'`, 2, first["id"])
	assertCount(t, db, `SELECT COUNT(*) FROM activity_work_updates WHERE source_id=$1 AND source_snapshot_json->'files'->0->>'reference_type'='evidence'`, 3, second["id"])
	if _, err := db.ExecContext(ctx, `DELETE FROM projects WHERE id=$1`, projectID); err != nil {
		t.Fatalf("design release item immutability blocked owner cascade: %v", err)
	}
}

func TestValidateDesignReleaseDraftRequiresReason(t *testing.T) {
	if _, _, _, err := validateDesignReleaseDraft(Record{"revision_label": "R03"}); err == nil {
		t.Fatal("missing design revision reason was accepted")
	}
	label, reason, notes, err := validateDesignReleaseDraft(Record{
		"revision_label": " R03 ",
		"change_reason":  " Cập nhật phối cảnh ",
		"notes":          " Client đã phản hồi ",
	})
	if err != nil {
		t.Fatal(err)
	}
	if label != "R03" || reason != "Cập nhật phối cảnh" || notes != "Client đã phản hồi" {
		t.Fatalf("normalized draft=%q/%q/%q", label, reason, notes)
	}
}
