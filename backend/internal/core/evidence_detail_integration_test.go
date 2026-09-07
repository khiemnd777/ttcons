package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestEvidenceDetailRejectsInvalidIDBeforeRepository(t *testing.T) {
	service := NewService(nil, "test-secret")
	_, err := service.EvidenceDetail(context.Background(), "not-an-id", "actor")
	assertValidationError(t, err)
}

func TestEvidenceDetailScopeAndMetadata(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	fixture := setupWorkItemFlowFixture(t, db, false)
	outside := setupProjectScopeFixture(t, db, []string{"operation.view"})
	service := NewService(NewRepository(db), "test-secret")
	evidenceID := newID()
	mustExec(t, db, `INSERT INTO evidence_records
		(id,work_item_id,project_id,target_type,target_id,evidence_type,name,mime_type,file_size,storage_provider,storage_key,visibility,status,submitted_by,uploaded_by,caption)
		VALUES ($1,$2,$3,'work_item',$2,'document','Evidence metadata','application/pdf',3,'local','evidence/metadata.pdf','internal','submitted',$4,$4,'Original caption')`, evidenceID, fixture.itemID, fixture.projectID, fixture.workerID)

	detail, err := service.EvidenceDetail(ctx, evidenceID, fixture.workerID)
	if err != nil {
		t.Fatal(err)
	}
	if cleanID(detail["id"]) != evidenceID || cleanID(detail["work_item_id"]) != fixture.itemID || cleanID(detail["project_id"]) != fixture.projectID {
		t.Fatalf("unexpected evidence identity: %#v", detail)
	}
	if detail["caption"] != "Original caption" || cleanID(detail["submitted_by"]) != fixture.workerID || detail["submitted_by_name"] == nil || detail["created_at"] == nil || detail["work_item_title"] == nil {
		t.Fatalf("missing original submission/source metadata: %#v", detail)
	}
	if _, exists := detail["run_node_name"]; !exists {
		t.Fatal("stage metadata column is missing")
	}
	handler := NewHandler(service)
	app := fiber.New(fiber.Config{ErrorHandler: ErrorHandler})
	app.Get("/evidence/:id", func(c *fiber.Ctx) error {
		c.Locals("user_id", fixture.workerID)
		return handler.GetEvidenceDetail(c)
	})
	response, err := app.Test(httptest.NewRequest("GET", "/evidence/"+evidenceID, nil))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var envelope struct {
		Success bool   `json:"success"`
		Data    Record `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != 200 || !envelope.Success || cleanID(envelope.Data["id"]) != evidenceID || response.Header.Get("Cache-Control") != "private, no-store" {
		t.Fatalf("unexpected metadata response: status=%d envelope=%#v", response.StatusCode, envelope)
	}
	// Metadata access does not open the storage backend or invent a file result.
	if _, err = service.EvidenceDetail(ctx, evidenceID, outside.userID); err == nil {
		t.Fatal("outside-project actor received evidence metadata")
	} else {
		var forbidden StatusError
		if !errors.As(err, &forbidden) || forbidden.Status != fiber.StatusForbidden {
			t.Fatalf("want forbidden, got %v", err)
		}
	}
	if _, err = service.EvidenceDetail(ctx, newID(), fixture.workerID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("missing evidence: want no rows, got %v", err)
	}
	mustExec(t, db, `UPDATE work_items SET status='cancelled' WHERE id=$1`, fixture.itemID)
	if _, err = service.EvidenceDetail(ctx, evidenceID, fixture.workerID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("cancelled work item exposed metadata: %v", err)
	}
}
