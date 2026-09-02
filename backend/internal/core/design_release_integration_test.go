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

	first, err := repo.CreateDesignRelease(ctx, projectID, "", "R01", "Phát hành thiết kế lần đầu", "", actorID, "Design Release Actor")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.AddDesignReleaseFile(ctx, cleanID(first["id"]), "layout", "Layout R01", "layout-r01.pdf", StoredFileMetadata{
		StorageProvider: "local", StorageKey: "tests/design/layout-r01.pdf", MIMEType: "application/pdf", FileSize: 100, ChecksumSHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}, actorID, "Design Release Actor"); err != nil {
		t.Fatal(err)
	}
	first, err = repo.ReleaseDesignRelease(ctx, cleanID(first["id"]), actorID, "Design Release Actor")
	if err != nil {
		t.Fatal(err)
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
	outcome, err = repo.automateClientOperationalSourceGate(ctx, Record{"project_id": projectID, "id": workItemID}, "source.design_documents_ready")
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Status != "succeeded" || outcome.TargetType != "design_release" || outcome.TargetID != cleanID(first["id"]) {
		t.Fatalf("gate after R01 handoff = %#v, want succeeded pinned R01", outcome)
	}

	second, err := repo.CreateDesignRelease(ctx, projectID, "", "R02", "Điều chỉnh layout theo phản hồi client", "Không thay đổi BOQ tự động", actorID, "Design Release Actor")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.AddDesignReleaseFile(ctx, cleanID(second["id"]), "construction_drawing", "Bản vẽ R02", "drawing-r02.pdf", StoredFileMetadata{
		StorageProvider: "local", StorageKey: "tests/design/drawing-r02.pdf", MIMEType: "application/pdf", FileSize: 120, ChecksumSHA256: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	}, actorID, "Design Release Actor"); err != nil {
		t.Fatal(err)
	}
	second, err = repo.ReleaseDesignRelease(ctx, cleanID(second["id"]), actorID, "Design Release Actor")
	if err != nil {
		t.Fatal(err)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM design_releases WHERE id=$1 AND status='superseded'`, 1, cleanID(first["id"]))
	assertCount(t, db, `SELECT COUNT(*) FROM design_releases WHERE id=$1 AND status='released'`, 1, cleanID(second["id"]))

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
