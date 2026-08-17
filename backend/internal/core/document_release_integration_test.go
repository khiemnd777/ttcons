package core

import (
	"context"
	"testing"
)

func TestCreateProjectDocumentPublishesMonotonicRevision(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	actorID := newID()
	projectID := newID()
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM projects WHERE id=$1`, projectID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id=$1`, actorID)
	})
	mustExec(t, db, `INSERT INTO users (id,email,password_hash,full_name,status) VALUES ($1,$2,'demo:pw','Document Actor','active')`, actorID, "document-"+actorID+"@test.local")
	mustExec(t, db, `INSERT INTO projects (id,name,code,client_name,status) VALUES ($1,'Document Revision Project',$2,'Client','planning')`, projectID, "DOC-"+projectID[:8])

	first, err := repo.CreateProjectDocument(ctx, projectID, "project_intake", projectID, "brief", "Brief", "brief-v1.pdf", StoredFileMetadata{
		StorageProvider: "local", StorageKey: "tests/brief-v1.pdf", MIMEType: "application/pdf", FileSize: 100, ChecksumSHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}, actorID, "Document Actor")
	if err != nil {
		t.Fatal(err)
	}
	second, err := repo.CreateProjectDocument(ctx, projectID, "project_intake", projectID, "brief", "Brief", "brief-v2.pdf", StoredFileMetadata{
		StorageProvider: "local", StorageKey: "tests/brief-v2.pdf", MIMEType: "application/pdf", FileSize: 120, ChecksumSHA256: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	}, actorID, "Document Actor")
	if err != nil {
		t.Fatal(err)
	}
	if int(number(first["revision_no"])) != 1 || int(number(second["revision_no"])) != 2 {
		t.Fatalf("document revisions=%v/%v, want 1/2", first["revision_no"], second["revision_no"])
	}
	assertCount(t, db, `SELECT COUNT(*) FROM documents WHERE id=$1 AND release_status='superseded'`, 1, cleanID(first["id"]))
	assertCount(t, db, `SELECT COUNT(*) FROM documents WHERE id=$1 AND release_status='released' AND released_at IS NOT NULL`, 1, cleanID(second["id"]))
}
