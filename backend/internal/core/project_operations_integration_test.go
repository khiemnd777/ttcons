package core

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"consflow/backend/internal/shared/migrations"

	"github.com/gofiber/fiber/v2"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestHasChecklistPayloadRequiresChecklistContent(t *testing.T) {
	tests := []struct {
		name    string
		payload Record
		want    bool
	}{
		{name: "empty payload", payload: Record{}, want: false},
		{name: "unrelated payload", payload: Record{"note": "done"}, want: false},
		{name: "false checklist flag", payload: Record{"checklist": false}, want: false},
		{name: "empty checklist list", payload: Record{"checklist": []any{}}, want: false},
		{name: "checklist list", payload: Record{"checklist": []any{Record{"label": "done", "checked": true}}}, want: true},
		{name: "checklist items", payload: Record{"checklist_items": []string{"done"}}, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasChecklistPayload(tt.payload); got != tt.want {
				t.Fatalf("hasChecklistPayload() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLegacyStepSubmitFillsChecklistPayload(t *testing.T) {
	item := Record{"evidence_requirement_json": map[string]any{"checklist_required": true}}
	req := withLegacyChecklistCompletion(item, WorkItemActionRequest{})

	if !hasChecklistPayload(req.CompletionPayload) {
		t.Fatal("expected legacy submit to satisfy checklist payload")
	}
	if _, ok := req.CompletionPayload["checklist"]; !ok {
		t.Fatalf("expected checklist key in payload, got %#v", req.CompletionPayload)
	}
}

func TestWorkflowFormValidationNormalizesAndRejectsBadInput(t *testing.T) {
	service := NewService(&Repository{}, "test-secret")
	fields := []Record{
		{"field_key": "summary", "label": "Kết quả", "field_type": "textarea", "required": true, "validation_json": map[string]any{"min_length": 3}},
		{"field_key": "confirmed", "label": "Xác nhận", "field_type": "checkbox", "required": true, "validation_json": map[string]any{}},
		{"field_key": "quantity", "label": "Khối lượng", "field_type": "number", "required": false, "validation_json": map[string]any{"min": float64(0)}},
	}

	_, err := service.validateWorkflowFormAnswers(context.Background(), Record{}, fields, Record{"summary": "ok", "extra": "bad"}, true)
	assertValidationError(t, err)

	_, err = service.validateWorkflowFormAnswers(context.Background(), Record{}, fields, Record{"summary": "ok", "confirmed": false}, true)
	assertValidationError(t, err)

	answers, err := service.validateWorkflowFormAnswers(context.Background(), Record{}, fields, Record{"summary": "  đã xong  ", "confirmed": true, "quantity": "12.5"}, true)
	if err != nil {
		t.Fatal(err)
	}
	if answers["summary"] != "đã xong" || answers["confirmed"] != true || number(answers["quantity"]) != 12.5 {
		t.Fatalf("normalized answers = %#v", answers)
	}
}

func TestWorkflowFormStructuredListValidation(t *testing.T) {
	service := NewService(&Repository{}, "test-secret")
	fields := []Record{
		{
			"field_key":  "action_items",
			"label":      "Action items",
			"field_type": "structured_list",
			"required":   true,
			"validation_json": map[string]any{
				"min_items": float64(1),
				"item_fields": []any{
					map[string]any{"key": "title", "label": "Việc cần làm", "type": "text", "required": true, "min_length": float64(3)},
					map[string]any{"key": "due_date", "label": "Due date", "type": "date", "required": true},
					map[string]any{"key": "priority", "label": "Priority", "type": "select", "required": true, "options": []any{map[string]any{"value": "normal"}, map[string]any{"value": "high"}}},
				},
			},
		},
	}

	_, err := service.validateWorkflowFormAnswers(context.Background(), Record{}, fields, Record{"action_items": []any{}}, true)
	assertValidationError(t, err)

	_, err = service.validateWorkflowFormAnswers(context.Background(), Record{}, fields, Record{"action_items": []any{map[string]any{"title": "ok", "due_date": "bad-date", "priority": "urgent"}}}, true)
	assertValidationError(t, err)

	answers, err := service.validateWorkflowFormAnswers(context.Background(), Record{}, fields, Record{
		"action_items": []any{map[string]any{"title": "  Chốt bản vẽ  ", "due_date": "2026-06-30", "priority": "high"}},
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	items, ok := answers["action_items"].([]Record)
	if !ok || len(items) != 1 {
		t.Fatalf("normalized action_items = %#v", answers["action_items"])
	}
	if items[0]["title"] != "Chốt bản vẽ" || items[0]["priority"] != "high" {
		t.Fatalf("normalized structured list item = %#v", items[0])
	}
}

func TestBOQUnitPriceValidation(t *testing.T) {
	expectedPrice, comment, err := normalizeBOQUnitPriceRequest(BOQUnitPriceRequest{UnitPrice: 0, Comment: "  draft expected  "}, false)
	if err != nil {
		t.Fatal(err)
	}
	if expectedPrice != 0 || comment != "draft expected" {
		t.Fatalf("expected normalized expected price/comment, got %v %q", expectedPrice, comment)
	}
	_, _, err = normalizeBOQUnitPriceRequest(BOQUnitPriceRequest{UnitPrice: 0}, true)
	assertValidationError(t, err)
	_, err = normalizeBOQItemRequest("project-id", Record{"category": "A", "work_item_name": "Item", "unit": "m2", "quantity": "2.5", "unit_price": "100"}, true)
	if err != nil {
		t.Fatal(err)
	}
	_, err = normalizeBOQItemRequest("project-id", Record{"category": "A", "work_item_name": "Item", "unit": "m2", "quantity": -1}, true)
	assertValidationError(t, err)
}

func TestWorkflowFormPDFGeneratorProducesPDFWithSignatureStamp(t *testing.T) {
	form := Record{
		"instance": Record{
			"id":               newID(),
			"project_id":       newID(),
			"work_item_id":     newID(),
			"title":            "Biên bản họp kế hoạch",
			"form_type":        "meeting_minutes",
			"status":           "submitted",
			"current_revision": 2,
		},
		"template": Record{"name": "Biên bản họp", "form_type": "meeting_minutes"},
		"version":  Record{"version_no": 2},
		"fields": []Record{
			{"field_key": "summary", "label": "Nội dung", "order_index": 1},
			{"field_key": "confirmed", "label": "Xác nhận", "order_index": 2},
		},
		"answers": Record{"summary": "Đã thống nhất kế hoạch tuần", "confirmed": true},
		"signatures": []Record{{
			"signer_name":            "Worker",
			"purpose":                "internal_ack",
			"signed_at":              "2026-06-21T10:00:00Z",
			"payload_hash":           strings.Repeat("a", 64),
			"visual_checksum_sha256": strings.Repeat("b", 64),
			"payload_json": Record{
				"visual_signature": Record{
					"strokes": []any{
						map[string]any{"points": []any{
							map[string]any{"x": 0.1, "y": 0.7},
							map[string]any{"x": 0.45, "y": 0.25},
							map[string]any{"x": 0.9, "y": 0.6},
						}},
					},
				},
			},
		}},
	}
	pdfBytes, payloadHash, err := GenerateWorkflowFormPDF(form)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(pdfBytes, []byte("%PDF-1.4")) {
		t.Fatalf("expected PDF header, got %q", pdfBytes[:8])
	}
	if len(payloadHash) != 64 {
		t.Fatalf("payload hash = %q, want 64 chars", payloadHash)
	}
	if !bytes.Contains(pdfBytes, []byte("SIGNED BY Worker")) {
		t.Fatal("expected generated PDF to include visual signature stamp text")
	}
}

func TestValidateActivityRequest(t *testing.T) {
	t.Run("defaults legacy comment", func(t *testing.T) {
		req, err := validateActivityRequest(WorkItemActivityRequest{Comment: "  cập nhật hiện trường  "})
		if err != nil {
			t.Fatal(err)
		}
		if req.ActivityType != "comment" || req.Visibility != "project" || req.Comment != "cập nhật hiện trường" {
			t.Fatalf("normalized request = %#v", req)
		}
	})

	t.Run("rejects system event", func(t *testing.T) {
		_, err := validateActivityRequest(WorkItemActivityRequest{ActivityType: "system_event", Comment: "generated"})
		assertValidationError(t, err)
	})

	t.Run("requires reminder due date", func(t *testing.T) {
		_, err := validateActivityRequest(WorkItemActivityRequest{ActivityType: "reminder", Comment: "nhắc kiểm tra lại"})
		assertValidationError(t, err)
	})

	t.Run("accepts reminder due date", func(t *testing.T) {
		req, err := validateActivityRequest(WorkItemActivityRequest{
			ActivityType: "reminder",
			Comment:      "nhắc kiểm tra lại",
			Metadata:     Record{"due_at": "2026-06-30"},
		})
		if err != nil {
			t.Fatal(err)
		}
		if req.Metadata["due_at"] != "2026-06-30" {
			t.Fatalf("due_at = %#v", req.Metadata["due_at"])
		}
	})

	t.Run("requires file note reference", func(t *testing.T) {
		_, err := validateActivityRequest(WorkItemActivityRequest{ActivityType: "file_note", Comment: "ghi chú file"})
		assertValidationError(t, err)
	})

	t.Run("accepts structured file note attachment", func(t *testing.T) {
		evidenceID := newID()
		req, err := validateActivityRequest(WorkItemActivityRequest{
			ActivityType: "file_note",
			Comment:      "ghi chú file",
			Attachments:  []ActivityAttachmentRequest{{Type: "evidence", ID: evidenceID}},
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(req.Attachments) != 1 || req.Metadata["evidence_id"] != evidenceID {
			t.Fatalf("normalized attachment request = %#v", req)
		}
	})

	t.Run("normalizes structured mentions", func(t *testing.T) {
		req, err := validateActivityRequest(WorkItemActivityRequest{
			ActivityType: "question",
			Comment:      "cần phản hồi",
			Mentions: []ActivityMentionRequest{
				{Type: "role", ID: "PM", Label: "PM"},
				{Type: "role", ID: "PM", Label: "PM"},
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(req.Mentions) != 1 || req.Mentions[0].Type != "role" || req.Mentions[0].ID != "PM" {
			t.Fatalf("mentions = %#v", req.Mentions)
		}
	})

	t.Run("rejects invalid user mention id", func(t *testing.T) {
		_, err := validateActivityRequest(WorkItemActivityRequest{
			ActivityType: "question",
			Comment:      "cần phản hồi",
			Mentions:     []ActivityMentionRequest{{Type: "user", ID: "not-a-uuid"}},
		})
		assertValidationError(t, err)
	})
}

func TestActivityStreamHubPublishesEvents(t *testing.T) {
	hub := NewActivityStreamHub()
	subID, events := hub.Subscribe()
	defer hub.Unsubscribe(subID)

	hub.Publish(ActivityStreamEvent{Type: "activity_changed", ProjectID: "project-1", ActivityID: "activity-1"})

	select {
	case event := <-events:
		if event.Type != "activity_changed" || event.ProjectID != "project-1" || event.ActivityID != "activity-1" {
			t.Fatalf("event = %#v", event)
		}
		if event.At == "" {
			t.Fatal("expected published event timestamp")
		}
	case <-time.After(time.Second):
		t.Fatal("expected published stream event")
	}
}

func TestActivityMentionsUnreadAndRoleScope(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")
	suffix := newID()
	projectID := newID()
	actorUserID := newID()
	actorStaffID := newID()
	pmUserID := newID()
	pmStaffID := newID()
	outsideUserID := newID()
	outsideStaffID := newID()
	reporterRoleID := ensureRole(t, db, "batch3_reporter_"+suffix[:8], "Batch3 Reporter")
	pmRoleID := ensureRole(t, db, "batch3_pm_"+suffix[:8], "Batch3 PM")
	operationViewID := ensurePermission(t, db, "operation.view", "Xem thi công", "operation")
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM projects WHERE id=$1`, projectID)
		_, _ = db.ExecContext(ctx, `DELETE FROM staff WHERE id IN ($1,$2,$3)`, actorStaffID, pmStaffID, outsideStaffID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id IN ($1,$2,$3)`, actorUserID, pmUserID, outsideUserID)
		_, _ = db.ExecContext(ctx, `DELETE FROM roles WHERE id IN ($1,$2)`, reporterRoleID, pmRoleID)
	})
	mustExec(t, db, `INSERT INTO role_permissions (role_id, permission_id) VALUES ($1,$3),($2,$3) ON CONFLICT DO NOTHING`, reporterRoleID, pmRoleID, operationViewID)
	mustExec(t, db, `INSERT INTO users (id, email, password_hash, full_name, status) VALUES ($1,$2,'demo:pw','Mention Actor','active'),($3,$4,'demo:pw','Mention PM','active'),($5,$6,'demo:pw','Outside PM','active')`,
		actorUserID, "mention-actor-"+suffix+"@test.local", pmUserID, "mention-pm-"+suffix+"@test.local", outsideUserID, "mention-outside-"+suffix+"@test.local")
	mustExec(t, db, `INSERT INTO staff (id, user_id, full_name, email, department, position, status) VALUES ($1,$2,'Mention Actor',$3,'Site','Reporter','active'),($4,$5,'Mention PM',$6,'Site','PM','active'),($7,$8,'Outside PM',$9,'Site','PM','active')`,
		actorStaffID, actorUserID, "mention-actor-staff-"+suffix+"@test.local", pmStaffID, pmUserID, "mention-pm-staff-"+suffix+"@test.local", outsideStaffID, outsideUserID, "mention-outside-staff-"+suffix+"@test.local")
	mustExec(t, db, `INSERT INTO projects (id, name, code, client_name, status) VALUES ($1,'Mention Project',$2,'Client','planning')`, projectID, "MEN-"+suffix[:8])
	mustExec(t, db, `INSERT INTO project_members (project_id, staff_id, role_ids, active) VALUES ($1,$2,'["Batch3 Reporter"]'::jsonb,true),($1,$3,'["Batch3 PM"]'::jsonb,true)`, projectID, actorStaffID, pmStaffID)
	mustExec(t, db, `INSERT INTO user_roles (user_id, role_id) VALUES ($1,$2),($3,$4),($5,$4)`, actorUserID, reporterRoleID, pmUserID, pmRoleID, outsideUserID, pmRoleID)

	activity, err := service.AddProjectActivity(ctx, projectID, WorkItemActivityRequest{
		ActivityType: "question",
		Comment:      "Cần PM xác nhận biện pháp thi công",
		Mentions:     []ActivityMentionRequest{{Type: "role", ID: "Batch3 PM", Label: "Batch3 PM"}},
	}, actorUserID, "Mention Actor")
	if err != nil {
		t.Fatal(err)
	}
	activityID := fmt.Sprint(activity["id"])
	assertCount(t, db, `SELECT COUNT(*) FROM activity_mentions WHERE activity_id=$1 AND recipient_user_id=$2`, 1, activityID, pmUserID)
	assertCount(t, db, `SELECT COUNT(*) FROM activity_mentions WHERE activity_id=$1 AND recipient_user_id=$2`, 0, activityID, outsideUserID)
	assertCount(t, db, `SELECT COUNT(*) FROM activity_read_states WHERE activity_id=$1 AND user_id=$2 AND read_at IS NULL`, 1, activityID, pmUserID)

	attention, err := service.MyActivityAttention(ctx, ActivityAttentionFilter{Queue: "mentions"}, pmUserID)
	if err != nil {
		t.Fatal(err)
	}
	if len(attention) != 1 || attention[0]["id"] != activityID || !boolValue(attention[0]["is_unread"]) || !boolValue(attention[0]["mentioned_me"]) {
		t.Fatalf("attention = %#v", attention)
	}
	outsideAttention, err := service.MyActivityAttention(ctx, ActivityAttentionFilter{Queue: "mentions"}, outsideUserID)
	if err != nil {
		t.Fatal(err)
	}
	if len(outsideAttention) != 0 {
		t.Fatalf("outside attention = %#v", outsideAttention)
	}

	if _, err := service.MarkActivityRead(ctx, activityID, pmUserID); err != nil {
		t.Fatal(err)
	}
	unread, err := service.MyActivityAttention(ctx, ActivityAttentionFilter{Queue: "unread"}, pmUserID)
	if err != nil {
		t.Fatal(err)
	}
	if len(unread) != 0 {
		t.Fatalf("unread after mark read = %#v", unread)
	}
}

func TestActivityAttachmentReferencesAreScopedAndVisible(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	service := NewService(NewRepository(db), "test-secret")
	fixture := setupWorkItemFlowFixture(t, db, false)
	otherProjectID := newID()
	otherWorkItemID := newID()
	otherEvidenceID := newID()
	documentID := newID()
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM projects WHERE id=$1`, otherProjectID)
	})
	evidence, err := service.AddWorkItemEvidence(ctx, fixture.itemID, WorkItemEvidenceRequest{
		EvidenceType: "document",
		Name:         "Biên bản nghiệm thu",
		FileURL:      "/evidence/bien-ban.pdf",
		Visibility:   "project",
	}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	evidenceID := fmt.Sprint(evidence["id"])
	activity, err := service.AddWorkItemActivity(ctx, fixture.itemID, WorkItemActivityRequest{
		ActivityType: "file_note",
		Comment:      "Cần kiểm tra evidence",
		Attachments:  []ActivityAttachmentRequest{{Type: "evidence", ID: evidenceID}},
	}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM work_item_activities WHERE id=$1 AND metadata_json->'attachments' @> $2::jsonb`, 1, activity["id"], fmt.Sprintf(`[{"type":"evidence","id":"%s"}]`, evidenceID))

	_, err = service.AddWorkItemActivity(ctx, fixture.itemID, WorkItemActivityRequest{
		ActivityType: "file_note",
		Comment:      "Không được leak client",
		Visibility:   "client",
		Attachments:  []ActivityAttachmentRequest{{Type: "evidence", ID: evidenceID}},
	}, fixture.workerID, "Worker")
	assertValidationError(t, err)

	mustExec(t, db, `INSERT INTO projects (id, name, code, client_name, status) VALUES ($1,'Attachment Other',$2,'Client','planning')`, otherProjectID, "ATT-"+otherProjectID[:8])
	mustExec(t, db, `
		INSERT INTO work_items
			(id, project_id, source_type, source_id, title, work_type, status, evidence_requirement_json, created_by, updated_by)
		VALUES
			($1,$2,'issue',$3,'Other evidence source','task','ready','{}'::jsonb,$4,$4)
	`, otherWorkItemID, otherProjectID, newID(), fixture.workerID)
	mustExec(t, db, `
		INSERT INTO evidence_records
			(id, work_item_id, project_id, target_type, target_id, evidence_type, name, file_url, visibility, status, submitted_by)
		VALUES
			($1,$2,$3,'work_item',$2,'document','Other project evidence','/evidence/other.pdf','project','submitted',$4)
	`, otherEvidenceID, otherWorkItemID, otherProjectID, fixture.workerID)
	_, err = service.AddProjectActivity(ctx, fixture.projectID, WorkItemActivityRequest{
		ActivityType: "file_note",
		Comment:      "Sai project",
		Attachments:  []ActivityAttachmentRequest{{Type: "evidence", ID: otherEvidenceID}},
	}, fixture.workerID, "Worker")
	assertValidationError(t, err)

	mustExec(t, db, `INSERT INTO documents (id, project_id, name, file_url, document_type, status, uploaded_by) VALUES ($1,$2,'Project document','/docs/project.pdf','drawing','active',$3)`, documentID, fixture.projectID, fixture.workerID)
	if _, err := service.AddProjectActivity(ctx, fixture.projectID, WorkItemActivityRequest{
		ActivityType: "file_note",
		Comment:      "Gắn document nội bộ",
		Attachments:  []ActivityAttachmentRequest{{Type: "document", ID: documentID}},
	}, fixture.workerID, "Worker"); err != nil {
		t.Fatal(err)
	}
	_, err = service.AddProjectActivity(ctx, fixture.projectID, WorkItemActivityRequest{
		ActivityType: "file_note",
		Comment:      "Document chưa có visibility nên không client-visible",
		Visibility:   "client",
		Attachments:  []ActivityAttachmentRequest{{Type: "document", ID: documentID}},
	}, fixture.workerID, "Worker")
	assertValidationError(t, err)
}

func TestWorkItemEvidenceUploadStoresProtectedFile(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	service := NewService(NewRepository(db), "test-secret")
	uploadDir := t.TempDir()
	service.ConfigureUploads(uploadDir, 1024*1024)
	fixture := setupWorkItemFlowFixture(t, db, false)
	content := []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a, 0, 0, 0, 0, 'I', 'E', 'N', 'D'}
	fileHeader := testMultipartFileHeader(t, "file", "site-photo.png", content)

	evidence, err := service.AddWorkItemEvidenceUpload(ctx, fixture.itemID, WorkItemEvidenceRequest{
		EvidenceType: "photo",
		Name:         "Ảnh hiện trường",
		Caption:      "Trước khi nghiệm thu",
		Visibility:   "project",
	}, fileHeader, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	if evidence["storage_provider"] != "local" {
		t.Fatalf("storage_provider = %v, want local", evidence["storage_provider"])
	}
	storageKey := fmt.Sprint(evidence["storage_key"])
	if storageKey == "" || strings.Contains(storageKey, "..") {
		t.Fatalf("invalid storage_key = %q", storageKey)
	}
	sum := sha256.Sum256(content)
	if got := fmt.Sprint(evidence["checksum_sha256"]); got != hex.EncodeToString(sum[:]) {
		t.Fatalf("checksum = %s, want %s", got, hex.EncodeToString(sum[:]))
	}
	storedPath := filepath.Join(uploadDir, filepath.FromSlash(storageKey))
	storedBytes, err := os.ReadFile(storedPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(storedBytes, content) {
		t.Fatalf("stored bytes = %v, want %v", storedBytes, content)
	}

	file, err := service.EvidenceFile(ctx, fmt.Sprint(evidence["id"]), fixture.workerID)
	if err != nil {
		t.Fatal(err)
	}
	if file.Path != storedPath || file.MIMEType != "image/png" {
		t.Fatalf("file result = %#v, want path %s and image/png", file, storedPath)
	}
	if !strings.HasSuffix(file.Filename, ".png") {
		t.Fatalf("filename = %q, want .png suffix", file.Filename)
	}

	invalidHeader := testMultipartFileHeader(t, "file", "notes.txt", []byte("not an allowed evidence file"))
	_, err = service.AddWorkItemEvidenceUpload(ctx, fixture.itemID, WorkItemEvidenceRequest{EvidenceType: "photo", Name: "Bad file"}, invalidHeader, fixture.workerID, "Worker")
	assertValidationError(t, err)
}

func TestActivityTargetTableFirstPassScopes(t *testing.T) {
	tests := []struct {
		scope string
		table string
		ok    bool
	}{
		{scope: "boq_item", table: "boq_items", ok: true},
		{scope: "procurement_request", table: "procurement_requests", ok: true},
		{scope: "stock_movement", table: "stock_movements", ok: true},
		{scope: "acceptance", table: "acceptance_records", ok: true},
		{scope: "rework", table: "rework_requests", ok: true},
		{scope: "handover", table: "handover_records", ok: true},
		{scope: "payable", table: "payables", ok: true},
		{scope: "receivable", table: "receivables", ok: true},
		{scope: "payment", table: "payments", ok: true},
		{scope: "daily_report", ok: false},
	}
	for _, tt := range tests {
		t.Run(tt.scope, func(t *testing.T) {
			table, _, ok := activityTargetTable(tt.scope)
			if ok != tt.ok || table != tt.table {
				t.Fatalf("activityTargetTable(%q) = %q/%v, want %q/%v", tt.scope, table, ok, tt.table, tt.ok)
			}
		})
	}
}

func TestCanCompleteWorkItemStatus(t *testing.T) {
	allowed := []string{"ready", "in_progress", "needs_changes", "waiting_evidence"}
	for _, status := range allowed {
		t.Run("allows_"+status, func(t *testing.T) {
			if !canCompleteWorkItemStatus(status) {
				t.Fatalf("expected %s to be completable", status)
			}
		})
	}
	blocked := []string{"pending", "blocked", "waiting_approval", "waiting_review", "completed", "cancelled"}
	for _, status := range blocked {
		t.Run("blocks_"+status, func(t *testing.T) {
			if canCompleteWorkItemStatus(status) {
				t.Fatalf("expected %s to be blocked", status)
			}
		})
	}
}

func TestWorkItemTransitionForAction(t *testing.T) {
	tests := []struct {
		name         string
		action       string
		status       string
		startedAt    string
		wantStatus   string
		wantActivity string
		wantEvidence bool
		wantErr      bool
	}{
		{name: "request review", action: "request-review", status: "in_progress", wantStatus: "waiting_review", wantActivity: "review_requested", wantEvidence: true},
		{name: "request approval", action: "request-approval", status: "waiting_review", wantStatus: "waiting_approval", wantActivity: "approval_requested", wantEvidence: true},
		{name: "approve approval", action: "approve", status: "waiting_approval", wantStatus: "completed", wantActivity: "approved", wantEvidence: true},
		{name: "reject review", action: "reject", status: "waiting_review", wantStatus: "needs_changes", wantActivity: "rejected"},
		{name: "request changes", action: "request-change", status: "waiting_approval", wantStatus: "needs_changes", wantActivity: "changes_requested"},
		{name: "resolve blocker resumes started", action: "resolve-blocker", status: "blocked", startedAt: "2026-06-12T00:00:00Z", wantStatus: "in_progress", wantActivity: "blocker_resolved"},
		{name: "blocks invalid approve", action: "approve", status: "ready", wantErr: true},
		{name: "blocks invalid review", action: "request-review", status: "pending", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item := Record{"status": tt.status}
			if tt.startedAt != "" {
				item["started_at"] = tt.startedAt
			}
			got, err := workItemTransitionForAction(tt.action, item)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got.ToStatus != tt.wantStatus || got.ActivityType != tt.wantActivity {
				t.Fatalf("transition = %s/%s, want %s/%s", got.ToStatus, got.ActivityType, tt.wantStatus, tt.wantActivity)
			}
			if got.RequiresEvidence != tt.wantEvidence {
				t.Fatalf("RequiresEvidence = %v, want %v", got.RequiresEvidence, tt.wantEvidence)
			}
		})
	}
}

func TestBatch7ProcurementTransitionsAreExplicit(t *testing.T) {
	tests := []struct {
		name       string
		action     string
		status     string
		wantStatus string
		wantEvent  string
		wantErr    bool
	}{
		{name: "submit draft", action: "submit", status: "draft", wantStatus: "waiting_approval", wantEvent: "review_requested"},
		{name: "approve waiting", action: "approve", status: "waiting_approval", wantStatus: "approved", wantEvent: "material_approved"},
		{name: "order approved", action: "order", status: "approved", wantStatus: "ordered", wantEvent: "material_ordered"},
		{name: "receive ordered", action: "receive", status: "ordered", wantStatus: "received", wantEvent: "warehouse_received"},
		{name: "block skipped order", action: "order", status: "draft", wantErr: true},
		{name: "block skipped receive", action: "receive", status: "approved", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotStatus, gotEvent, err := procurementTransitionForAction(tt.action, tt.status)
			if tt.wantErr {
				assertValidationError(t, err)
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if gotStatus != tt.wantStatus || gotEvent != tt.wantEvent {
				t.Fatalf("transition = %s/%s, want %s/%s", gotStatus, gotEvent, tt.wantStatus, tt.wantEvent)
			}
		})
	}
}

func TestProjectOperationsAccessPolicy(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)

	suffix := newID()
	projectID := newID()
	cancelledProjectID := newID()
	directorUserID := newID()
	pmUserID := newID()
	pmStaffID := newID()
	noScopeUserID := newID()
	projectRoleNoPermissionUserID := newID()

	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM projects WHERE id IN ($1,$2)`, projectID, cancelledProjectID)
		_, _ = db.ExecContext(ctx, `DELETE FROM staff WHERE id=$1`, pmStaffID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id IN ($1,$2,$3,$4)`, directorUserID, pmUserID, noScopeUserID, projectRoleNoPermissionUserID)
	})

	operationViewID := ensurePermission(t, db, "operation.view", "Xem thi công", "operation")
	directorRoleID := ensureRole(t, db, "director", "Giám đốc")
	pmRoleID := ensureRole(t, db, "pm", "PM")
	noPermissionRoleID := ensureRole(t, db, "batch2_viewer_"+suffix[:8], "Batch2 Viewer")
	mustExec(t, db, `INSERT INTO role_permissions (role_id, permission_id) VALUES ($1,$2),($3,$2) ON CONFLICT DO NOTHING`, directorRoleID, operationViewID, pmRoleID)

	mustExec(t, db, `INSERT INTO users (id, email, password_hash, full_name, status) VALUES ($1,$2,'demo:pw','Director','active')`, directorUserID, "director-"+suffix+"@test.local")
	mustExec(t, db, `INSERT INTO users (id, email, password_hash, full_name, status) VALUES ($1,$2,'demo:pw','PM','active')`, pmUserID, "pm-"+suffix+"@test.local")
	mustExec(t, db, `INSERT INTO users (id, email, password_hash, full_name, status) VALUES ($1,$2,'demo:pw','No Scope','active')`, noScopeUserID, "noscope-"+suffix+"@test.local")
	mustExec(t, db, `INSERT INTO users (id, email, password_hash, full_name, status) VALUES ($1,$2,'demo:pw','Project Role No Permission','active')`, projectRoleNoPermissionUserID, "project-role-no-perm-"+suffix+"@test.local")
	mustExec(t, db, `INSERT INTO staff (id, user_id, full_name, email, department, position, status) VALUES ($1,$2,'PM',$3,'Dự án','PM','active')`, pmStaffID, pmUserID, "pm-staff-"+suffix+"@test.local")
	mustExec(t, db, `INSERT INTO projects (id, name, code, client_name, status) VALUES ($1,'Ops Access',$2,'Client','planning'),($3,'Ops Cancelled',$4,'Client','cancelled')`, projectID, "OPA-"+suffix[:8], cancelledProjectID, "OPC-"+suffix[:8])
	mustExec(t, db, `INSERT INTO project_members (project_id, staff_id, role_ids, active) VALUES ($1,$2,'["PM"]'::jsonb,true)`, projectID, pmStaffID)
	mustExec(t, db, `INSERT INTO user_roles (user_id, role_id) VALUES ($1,$2),($3,$4),($5,$4)`, directorUserID, directorRoleID, pmUserID, pmRoleID, noScopeUserID)
	mustExec(t, db, `INSERT INTO user_roles (user_id, role_id, project_id) VALUES ($1,$2,$3)`, projectRoleNoPermissionUserID, noPermissionRoleID, projectID)

	assertCanViewProjectOperations(t, repo, ctx, projectID, directorUserID, true)
	assertCanViewProjectOperations(t, repo, ctx, projectID, pmUserID, true)
	assertCanViewProjectOperations(t, repo, ctx, projectID, noScopeUserID, false)
	assertCanViewProjectOperations(t, repo, ctx, projectID, projectRoleNoPermissionUserID, false)
	assertCanViewProjectOperations(t, repo, ctx, cancelledProjectID, directorUserID, false)
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

func TestBatch4AccountingListUsesProjectScope(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	repo := NewRepository(db)
	handler := NewHandler(NewService(repo, "test-secret"))
	fixture := setupProjectScopeFixture(t, db, []string{"accounting.view", "accounting.payable_view"})
	mustExec(t, db, `
		INSERT INTO payables (project_id, payable_code, vendor_type, vendor_name, source_type, gross_amount, remaining_amount, status, created_by)
		VALUES ($1,$2,'supplier','Scoped Vendor','manual',100,100,'approved',$3),
		       ($4,$5,'supplier','Other Vendor','manual',200,200,'approved',$3)
	`, fixture.projectID, "PAY-S-"+fixture.projectID[:8], fixture.userID, fixture.otherProjectID, "PAY-O-"+fixture.otherProjectID[:8])
	app := batch4TestApp(fixture.userID, func(app *fiber.App, h *Handler) {
		app.Get("/payables", h.ListPayables)
	}, handler)

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/payables", nil))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var body struct {
		Success bool     `json:"success"`
		Data    []Record `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if !body.Success {
		t.Fatal("expected success response")
	}
	if len(body.Data) != 1 {
		t.Fatalf("expected one scoped payable, got %d", len(body.Data))
	}
	if body.Data[0]["project_id"] != fixture.projectID {
		t.Fatalf("project_id = %v, want %s", body.Data[0]["project_id"], fixture.projectID)
	}
}

func TestBatch4StockAggregateExcludesOutOfScopeProjectMovements(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	repo := NewRepository(db)
	handler := NewHandler(NewService(repo, "test-secret"))
	fixture := setupProjectScopeFixture(t, db, []string{"warehouse.view", "report.view"})
	materialID := newID()
	warehouseID := newID()
	mustExec(t, db, `INSERT INTO materials (id, code, name, unit, category, active) VALUES ($1,$2,'Batch4 Material','m','Test',true)`, materialID, "MAT-"+materialID[:8])
	mustExec(t, db, `INSERT INTO warehouses (id, name, location, type, active) VALUES ($1,'Batch4 Warehouse','Test','central',true)`, warehouseID)
	mustExec(t, db, `
		INSERT INTO stock_movements (movement_code, project_id, warehouse_id, material_id, movement_type, quantity, unit, reason, created_by, approved_by)
		VALUES ($1,NULL,$2,$3,'receive',100,'m','Central stock',$4,$4),
		       ($5,$6,$2,$3,'issue',10,'m','Scoped issue',$4,$4),
		       ($7,$8,$2,$3,'issue',70,'m','Out of scope issue',$4,$4)
	`, "SM-C-"+materialID[:6], warehouseID, materialID, fixture.userID, "SM-S-"+materialID[:6], fixture.projectID, "SM-O-"+materialID[:6], fixture.otherProjectID)
	app := batch4TestApp(fixture.userID, func(app *fiber.App, h *Handler) {
		app.Get("/stock", h.Stock)
	}, handler)

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/stock", nil))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var body struct {
		Success bool     `json:"success"`
		Data    []Record `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	var found Record
	for _, row := range body.Data {
		if row["material_id"] == materialID {
			found = row
			break
		}
	}
	if found == nil {
		t.Fatal("expected batch4 material in stock response")
	}
	if got := number(found["stock_quantity"]); got != 90 {
		t.Fatalf("stock_quantity = %v, want 90", got)
	}
}

func openProjectOperationsTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("CONSFLOW_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set CONSFLOW_TEST_DATABASE_URL to run DB-backed project operations tests")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := migrations.Run(ctx, db, filepath.Join("..", "..", "migrations")); err != nil {
		t.Fatal(err)
	}
	return db
}

func testMultipartFileHeader(t *testing.T, fieldName, filename string, content []byte) *multipart.FileHeader {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile(fieldName, filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/upload", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if err := req.ParseMultipartForm(int64(body.Len()) + 1024); err != nil {
		t.Fatal(err)
	}
	files := req.MultipartForm.File[fieldName]
	if len(files) != 1 {
		t.Fatalf("expected one multipart file for %s, got %d", fieldName, len(files))
	}
	return files[0]
}

func testSignatureRequest(comment string) WorkflowFormSignRequest {
	return WorkflowFormSignRequest{
		Purpose:               "internal_ack",
		Comment:               comment,
		SignatureImageDataURL: "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+/p9sAAAAASUVORK5CYII=",
		SignatureStrokes: []WorkflowFormSignatureStroke{{
			Points: []WorkflowFormSignaturePoint{
				{X: 0.1, Y: 0.7},
				{X: 0.35, Y: 0.25},
				{X: 0.7, Y: 0.55},
				{X: 0.9, Y: 0.3},
			},
		}},
	}
}

func ensureRole(t *testing.T, db *sql.DB, name, displayName string) string {
	t.Helper()
	var id string
	if err := db.QueryRowContext(context.Background(), `
		INSERT INTO roles (name, display_name, system_role)
		VALUES ($1,$2,true)
		ON CONFLICT (name) DO UPDATE SET name=EXCLUDED.name
		RETURNING id
	`, name, displayName).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func ensurePermission(t *testing.T, db *sql.DB, key, name, module string) string {
	t.Helper()
	var id string
	if err := db.QueryRowContext(context.Background(), `
		INSERT INTO permissions (key, name, module)
		VALUES ($1,$2,$3)
		ON CONFLICT (key) DO UPDATE SET key=EXCLUDED.key
		RETURNING id
	`, key, name, module).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func assertCanViewProjectOperations(t *testing.T, repo *Repository, ctx context.Context, projectID, userID string, want bool) {
	t.Helper()
	got, err := repo.CanViewProjectOperations(ctx, projectID, userID)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("CanViewProjectOperations(%s, %s) = %v, want %v", projectID, userID, got, want)
	}
}

type projectScopeFixture struct {
	userID         string
	staffID        string
	roleID         string
	projectID      string
	otherProjectID string
}

func setupProjectScopeFixture(t *testing.T, db *sql.DB, permissions []string) projectScopeFixture {
	t.Helper()
	ctx := context.Background()
	suffix := newID()
	userID := newID()
	staffID := newID()
	roleID := ensureRole(t, db, "batch4_scope_"+suffix[:8], "Batch4 Scope")
	projectID := newID()
	otherProjectID := newID()
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM project_members WHERE project_id IN ($1,$2)`, projectID, otherProjectID)
		_, _ = db.ExecContext(ctx, `DELETE FROM projects WHERE id IN ($1,$2)`, projectID, otherProjectID)
		_, _ = db.ExecContext(ctx, `DELETE FROM user_roles WHERE user_id=$1`, userID)
		_, _ = db.ExecContext(ctx, `DELETE FROM role_permissions WHERE role_id=$1`, roleID)
		_, _ = db.ExecContext(ctx, `DELETE FROM roles WHERE id=$1`, roleID)
		_, _ = db.ExecContext(ctx, `DELETE FROM staff WHERE id=$1`, staffID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id=$1`, userID)
	})
	mustExec(t, db, `INSERT INTO users (id, email, password_hash, full_name, status) VALUES ($1,$2,'demo:pw','Scoped User','active')`, userID, "batch4-"+suffix+"@test.local")
	mustExec(t, db, `INSERT INTO staff (id, user_id, full_name, email, department, position, status) VALUES ($1,$2,'Scoped User',$3,'Dự án','Member','active')`, staffID, userID, "batch4-staff-"+suffix+"@test.local")
	mustExec(t, db, `INSERT INTO projects (id, name, code, client_name, status) VALUES ($1,'Batch4 Scoped',$2,'Client','planning'),($3,'Batch4 Other',$4,'Client','planning')`, projectID, "B4S-"+suffix[:8], otherProjectID, "B4O-"+suffix[:8])
	mustExec(t, db, `INSERT INTO project_members (project_id, staff_id, role_ids, active) VALUES ($1,$2,'["Member"]'::jsonb,true)`, projectID, staffID)
	mustExec(t, db, `INSERT INTO user_roles (user_id, role_id) VALUES ($1,$2)`, userID, roleID)
	for _, permission := range permissions {
		permissionID := ensurePermission(t, db, permission, permission, "batch4")
		mustExec(t, db, `INSERT INTO role_permissions (role_id, permission_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`, roleID, permissionID)
	}
	return projectScopeFixture{userID: userID, staffID: staffID, roleID: roleID, projectID: projectID, otherProjectID: otherProjectID}
}

func batch4TestApp(userID string, register func(*fiber.App, *Handler), handler *Handler) *fiber.App {
	app := fiber.New(fiber.Config{ErrorHandler: ErrorHandler})
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user_id", userID)
		c.Locals("email", "batch4@test.local")
		return c.Next()
	})
	register(app, handler)
	return app
}

type workItemFlowFixture struct {
	projectID string
	workerID  string
	pmID      string
	itemID    string
}

func setupWorkItemFlowFixture(t *testing.T, db *sql.DB, documentRequired bool) workItemFlowFixture {
	t.Helper()
	ctx := context.Background()
	suffix := newID()
	projectID := newID()
	workerID := newID()
	workerStaffID := newID()
	pmID := newID()
	pmStaffID := newID()
	itemID := newID()
	sourceID := newID()

	operationViewID := ensurePermission(t, db, "operation.view", "Xem thi công", "operation")
	operationReportID := ensurePermission(t, db, "operation.report", "Tạo daily report", "operation")
	workerRoleID := ensureRole(t, db, "batch3_worker_"+suffix[:8], "Đội thi công")
	pmRoleID := ensureRole(t, db, "pm", "PM")
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM projects WHERE id=$1`, projectID)
		_, _ = db.ExecContext(ctx, `DELETE FROM staff WHERE id IN ($1,$2)`, workerStaffID, pmStaffID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id IN ($1,$2)`, workerID, pmID)
		_, _ = db.ExecContext(ctx, `DELETE FROM roles WHERE id=$1`, workerRoleID)
	})
	mustExec(t, db, `INSERT INTO role_permissions (role_id, permission_id) VALUES ($1,$2),($1,$3) ON CONFLICT DO NOTHING`, workerRoleID, operationViewID, operationReportID)

	mustExec(t, db, `INSERT INTO users (id, email, password_hash, full_name, status) VALUES ($1,$2,'demo:pw','Worker','active')`, workerID, "worker-"+suffix+"@test.local")
	mustExec(t, db, `INSERT INTO users (id, email, password_hash, full_name, status) VALUES ($1,$2,'demo:pw','PM','active')`, pmID, "pm-batch3-"+suffix+"@test.local")
	mustExec(t, db, `INSERT INTO staff (id, user_id, full_name, email, department, position, status) VALUES ($1,$2,'Worker',$3,'Thi công','Worker','active')`, workerStaffID, workerID, "worker-staff-"+suffix+"@test.local")
	mustExec(t, db, `INSERT INTO staff (id, user_id, full_name, email, department, position, status) VALUES ($1,$2,'PM',$3,'Dự án','PM','active')`, pmStaffID, pmID, "pm-staff-batch3-"+suffix+"@test.local")
	mustExec(t, db, `INSERT INTO projects (id, name, code, client_name, status) VALUES ($1,'Batch3 Ops',$2,'Client','planning')`, projectID, "B3-"+suffix[:8])
	mustExec(t, db, `INSERT INTO project_members (project_id, staff_id, role_ids, active) VALUES ($1,$2,'["Đội thi công"]'::jsonb,true),($1,$3,'["PM"]'::jsonb,true)`, projectID, workerStaffID, pmStaffID)
	mustExec(t, db, `INSERT INTO user_roles (user_id, role_id) VALUES ($1,$2),($3,$4)`, workerID, workerRoleID, pmID, pmRoleID)

	requirement := `{"required_permission":"operation.report"}`
	if documentRequired {
		requirement = `{"required_permission":"operation.report","document_required":true}`
	}
	mustExec(t, db, `
		INSERT INTO work_items
			(id, project_id, source_type, source_id, title, work_type, status, role_ids, evidence_requirement_json, created_by, updated_by)
		VALUES
			($1,$2,'issue',$3,'Batch3 work item','task','ready','["Đội thi công"]'::jsonb,$4::jsonb,$5,$5)
	`, itemID, projectID, sourceID, requirement, workerID)

	return workItemFlowFixture{projectID: projectID, workerID: workerID, pmID: pmID, itemID: itemID}
}

func TestStartPipelineGeneratesWorkItemsAndIsIdempotent(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)

	actorID := newID()
	templateID := newID()
	nodeID := newID()
	projectID := newID()
	stepOneID := newID()
	stepTwoID := newID()
	formVersionID, err := repo.DefaultWorkflowFormVersionID(ctx, "checklist", true)
	if err != nil {
		t.Fatal(err)
	}
	if formVersionID == "" {
		t.Fatal("expected default checklist form version")
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM projects WHERE id=$1`, projectID)
		_, _ = db.ExecContext(ctx, `DELETE FROM pipeline_templates WHERE id=$1`, templateID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id=$1`, actorID)
	})

	mustExec(t, db, `INSERT INTO users (id, email, password_hash, full_name, status) VALUES ($1,$2,'demo:pw','Test Actor','active')`, actorID, "ops-start-"+actorID+"@test.local")
	mustExec(t, db, `INSERT INTO pipeline_templates (id, name, status, created_by) VALUES ($1,'Ops Template','published',$2)`, templateID, actorID)
	mustExec(t, db, `
		INSERT INTO pipeline_template_nodes (id, template_id, name, type, order_index, responsible_role_ids)
		VALUES ($1,$2,'Survey','survey',1,'["PM"]'::jsonb)
	`, nodeID, templateID)
	mustExec(t, db, `
		INSERT INTO pipeline_template_steps (id, node_id, name, step_type, required_role_ids, document_required, photo_required, form_version_id, form_required, order_index)
		VALUES ($1,$2,'Upload survey','document_upload','["PM"]'::jsonb,false,false,$4,true,1),
		       ($3,$2,'Confirm survey','checklist','["PM"]'::jsonb,false,false,$4,true,2)
	`, stepOneID, nodeID, stepTwoID, formVersionID)
	mustExec(t, db, `
		INSERT INTO projects (id, name, code, client_name, status, pipeline_template_id)
		VALUES ($1,'Ops Project',$2,'Client','draft',$3)
	`, projectID, "OPS-"+projectID[:8], templateID)

	firstRun, err := repo.StartPipeline(ctx, projectID, templateID, actorID, "actor@test.local")
	if err != nil {
		t.Fatal(err)
	}
	secondRun, err := repo.StartPipeline(ctx, projectID, templateID, actorID, "actor@test.local")
	if err != nil {
		t.Fatal(err)
	}
	if firstRun["id"] != secondRun["id"] {
		t.Fatalf("expected idempotent StartPipeline to return same run, got %v and %v", firstRun["id"], secondRun["id"])
	}
	assertCount(t, db, `SELECT COUNT(*) FROM pipeline_runs WHERE project_id=$1`, 1, projectID)
	assertCount(t, db, `SELECT COUNT(*) FROM pipeline_run_steps WHERE project_id=$1`, 2, projectID)
	assertCount(t, db, `SELECT COUNT(*) FROM work_items WHERE project_id=$1 AND source_type='pipeline_run_step'`, 2, projectID)
	assertCount(t, db, `SELECT COUNT(*) FROM workflow_form_instances WHERE project_id=$1`, 2, projectID)
	nodes, _ := secondRun["nodes"].([]Record)
	if len(nodes) == 0 {
		t.Fatal("expected pipeline run nodes")
	}
	steps, _ := nodes[0]["steps"].([]Record)
	if len(steps) != 2 {
		t.Fatalf("steps = %#v", steps)
	}
	for _, step := range steps {
		if cleanID(step["work_item_id"]) == "" || cleanID(step["workflow_form_instance_id"]) == "" {
			t.Fatalf("expected enriched step contract, got %#v", step)
		}
		if step["workflow_form_status"] != "draft" || !boolValue(step["workflow_form_required"]) {
			t.Fatalf("unexpected workflow form summary = %#v", step)
		}
	}
}

func TestWorkflowFormCatalogReturnsPublishedTemplates(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)

	rows, err := repo.WorkflowFormCatalog(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) == 0 {
		t.Fatal("expected workflow form catalog rows")
	}

	foundChecklist := false
	for _, row := range rows {
		if cleanID(row["version_id"]) == "" {
			t.Fatalf("catalog row missing version_id: %#v", row)
		}
		if number(row["field_count"]) <= 0 {
			t.Fatalf("catalog row missing fields: %#v", row)
		}
		if row["template_code"] == "builtin_checklist" {
			foundChecklist = true
			if number(row["version_no"]) != 2 {
				t.Fatalf("expected checklist v2 in catalog, got %#v", row["version_no"])
			}
			if !containsString(stringSlice(row["required_field_keys"]), "checklist_items") {
				t.Fatalf("expected checklist required fields, got %#v", row["required_field_keys"])
			}
		}
	}
	if !foundChecklist {
		t.Fatal("expected builtin checklist in workflow form catalog")
	}
}

func TestPublishTemplateRejectsRequiredStepWithoutPublishedForm(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")

	actorID := newID()
	templateID := newID()
	nodeID := newID()
	stepID := newID()
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM pipeline_templates WHERE id=$1`, templateID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id=$1`, actorID)
	})

	mustExec(t, db, `INSERT INTO users (id, email, password_hash, full_name, status) VALUES ($1,$2,'demo:pw','Test Actor','active')`, actorID, "ops-publish-"+actorID+"@test.local")
	mustExec(t, db, `INSERT INTO pipeline_templates (id, name, status, created_by) VALUES ($1,'Invalid Form Template','draft',$2)`, templateID, actorID)
	mustExec(t, db, `
		INSERT INTO pipeline_template_nodes (id, template_id, name, type, order_index, responsible_role_ids)
		VALUES ($1,$2,'Survey','survey',1,'["PM"]'::jsonb)
	`, nodeID, templateID)
	mustExec(t, db, `
		INSERT INTO pipeline_template_steps (id, node_id, name, step_type, required_role_ids, form_required, order_index)
		VALUES ($1,$2,'Required form step','task','["PM"]'::jsonb,true,1)
	`, stepID, nodeID)

	_, err := service.PublishTemplate(ctx, templateID, actorID, "actor@test.local")
	assertValidationError(t, err)

	row, err := repo.GetByID(ctx, "pipeline_templates", templateID)
	if err != nil {
		t.Fatal(err)
	}
	if row["status"] != "draft" {
		t.Fatalf("expected publish validation to keep draft status, got %#v", row["status"])
	}
}

func TestGenerateProcurementFromBOQRequiresFinalLockedApprovedItems(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)

	actorID := newID()
	projectID := newID()
	approvedBOQID := newID()
	submittedBOQID := newID()
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM projects WHERE id=$1`, projectID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id=$1`, actorID)
	})

	mustExec(t, db, `INSERT INTO users (id, email, password_hash, full_name, status) VALUES ($1,$2,'demo:pw','Test Actor','active')`, actorID, "ops-boq-"+actorID+"@test.local")
	mustExec(t, db, `
		INSERT INTO projects (id, name, code, client_name, status)
		VALUES ($1,'BOQ Project',$2,'Client','planning')
	`, projectID, "BOQ-"+projectID[:8])
	mustExec(t, db, `
		INSERT INTO boq_items (id, project_id, category, work_item_name, unit, quantity, unit_price, status, created_by)
		VALUES ($1,$2,'A','Approved item','m2',2,10,'approved',$3),
		       ($4,$2,'A','Submitted item','m2',100,10,'submitted',$3)
	`, approvedBOQID, projectID, actorID, submittedBOQID)

	_, err := repo.GenerateProcurementFromBOQ(ctx, projectID, actorID, "actor@test.local")
	assertValidationError(t, err)

	if _, err := repo.LockBOQNormUnitPrice(ctx, approvedBOQID, 11, "norm locked", actorID, "actor@test.local"); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.LockBOQFinalUnitPrice(ctx, approvedBOQID, 12, "final locked", actorID, "actor@test.local"); err != nil {
		t.Fatal(err)
	}

	procurement, err := repo.GenerateProcurementFromBOQ(ctx, projectID, actorID, "actor@test.local")
	if err != nil {
		t.Fatal(err)
	}
	requestID := procurement["id"]
	assertCount(t, db, `SELECT COUNT(*) FROM procurement_request_items WHERE request_id=$1`, 1, requestID)
	assertCount(t, db, `SELECT COUNT(*) FROM procurement_request_items WHERE request_id=$1 AND boq_item_id=$2`, 1, requestID, approvedBOQID)
	assertCount(t, db, `SELECT COUNT(*) FROM procurement_request_items WHERE request_id=$1 AND boq_item_id=$2`, 0, requestID, submittedBOQID)
	assertCount(t, db, `SELECT COUNT(*) FROM procurement_request_items WHERE request_id=$1 AND boq_item_id=$2 AND estimated_unit_price=12 AND boq_price_source='final' AND boq_price_revision_no=2 AND boq_price_event_id IS NOT NULL`, 1, requestID, approvedBOQID)
}

func TestBatch7StockMovementGuardsWarehouseAndSiteBalances(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)

	actorID := newID()
	projectID := newID()
	warehouseID := newID()
	materialID := newID()
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM audit_logs WHERE target_type='stock_movement' AND target_id IN (SELECT id FROM stock_movements WHERE material_id=$1)`, materialID)
		_, _ = db.ExecContext(ctx, `DELETE FROM stock_movements WHERE material_id=$1`, materialID)
		_, _ = db.ExecContext(ctx, `DELETE FROM site_material_balances WHERE project_id=$1 OR material_id=$2`, projectID, materialID)
		_, _ = db.ExecContext(ctx, `DELETE FROM materials WHERE id=$1`, materialID)
		_, _ = db.ExecContext(ctx, `DELETE FROM warehouses WHERE id=$1`, warehouseID)
		_, _ = db.ExecContext(ctx, `DELETE FROM projects WHERE id=$1`, projectID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id=$1`, actorID)
	})
	mustExec(t, db, `INSERT INTO users (id, email, password_hash, full_name, status) VALUES ($1,$2,'demo:pw','Batch7 Actor','active')`, actorID, "batch7-stock-"+actorID+"@test.local")
	mustExec(t, db, `INSERT INTO projects (id, name, code, client_name, status) VALUES ($1,'Batch7 Stock',$2,'Client','planning')`, projectID, "B7S-"+projectID[:8])
	mustExec(t, db, `INSERT INTO warehouses (id, name, location, type, active) VALUES ($1,'Batch7 Warehouse','Test','central',true)`, warehouseID)
	mustExec(t, db, `INSERT INTO materials (id, code, name, unit, category, active) VALUES ($1,$2,'Batch7 Material','m','Test',true)`, materialID, "B7M-"+materialID[:8])

	_, err := repo.CreateStockMovement(ctx, StockActionRequest{
		ProjectID:    projectID,
		WarehouseID:  warehouseID,
		MaterialID:   materialID,
		MovementType: "issue",
		Quantity:     10,
		Unit:         "m",
		Reason:       "issue before receive",
	}, actorID, "Batch7 Actor")
	assertValidationError(t, err)

	if _, err := repo.CreateStockMovement(ctx, StockActionRequest{
		WarehouseID:  warehouseID,
		MaterialID:   materialID,
		MovementType: "receive",
		Quantity:     5,
		Unit:         "m",
		Reason:       "receive stock",
	}, actorID, "Batch7 Actor"); err != nil {
		t.Fatal(err)
	}
	_, err = repo.CreateStockMovement(ctx, StockActionRequest{
		ProjectID:    projectID,
		WarehouseID:  warehouseID,
		MaterialID:   materialID,
		MovementType: "issue",
		Quantity:     10,
		Unit:         "m",
		Reason:       "over issue",
	}, actorID, "Batch7 Actor")
	assertValidationError(t, err)

	if _, err := repo.CreateStockMovement(ctx, StockActionRequest{
		ProjectID:    projectID,
		WarehouseID:  warehouseID,
		MaterialID:   materialID,
		MovementType: "issue",
		Quantity:     3,
		Unit:         "m",
		Reason:       "valid issue",
	}, actorID, "Batch7 Actor"); err != nil {
		t.Fatal(err)
	}
	_, err = repo.CreateStockMovement(ctx, StockActionRequest{
		ProjectID:    projectID,
		WarehouseID:  warehouseID,
		MaterialID:   materialID,
		MovementType: "return_to_warehouse",
		Quantity:     4,
		Unit:         "m",
		Reason:       "over return",
	}, actorID, "Batch7 Actor")
	assertValidationError(t, err)

	if _, err := repo.CreateStockMovement(ctx, StockActionRequest{
		ProjectID:    projectID,
		WarehouseID:  warehouseID,
		MaterialID:   materialID,
		MovementType: "return_to_warehouse",
		Quantity:     2,
		Unit:         "m",
		Reason:       "valid return",
	}, actorID, "Batch7 Actor"); err != nil {
		t.Fatal(err)
	}
	var issued, surplus, stock float64
	var surplusStatus string
	if err := db.QueryRowContext(ctx, `SELECT issued_quantity::float8, surplus_quantity::float8, surplus_status FROM site_material_balances WHERE project_id=$1 AND material_id=$2`, projectID, materialID).Scan(&issued, &surplus, &surplusStatus); err != nil {
		t.Fatal(err)
	}
	if issued != 1 || surplus != 1 || surplusStatus != "handled" {
		t.Fatalf("site balance = issued %.3f surplus %.3f status %s, want 1/1/handled", issued, surplus, surplusStatus)
	}
	if err := db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(CASE
			WHEN movement_type IN ('receive','return_to_warehouse') THEN quantity
			WHEN movement_type IN ('issue','transfer','loss') THEN -quantity
			ELSE 0
		END),0)::float8
		FROM stock_movements
		WHERE warehouse_id=$1 AND material_id=$2
	`, warehouseID, materialID).Scan(&stock); err != nil {
		t.Fatal(err)
	}
	if stock != 4 {
		t.Fatalf("warehouse stock = %.3f, want 4", stock)
	}
}

func TestBatch7FailedAcceptanceCreatesSingleOpenRework(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)

	actorID := newID()
	projectID := newID()
	acceptanceID := newID()
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM rework_requests WHERE acceptance_id=$1`, acceptanceID)
		_, _ = db.ExecContext(ctx, `DELETE FROM issues WHERE project_id=$1`, projectID)
		_, _ = db.ExecContext(ctx, `DELETE FROM acceptance_records WHERE id=$1`, acceptanceID)
		_, _ = db.ExecContext(ctx, `DELETE FROM projects WHERE id=$1`, projectID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id=$1`, actorID)
	})
	mustExec(t, db, `INSERT INTO users (id, email, password_hash, full_name, status) VALUES ($1,$2,'demo:pw','Batch7 Actor','active')`, actorID, "batch7-acceptance-"+actorID+"@test.local")
	mustExec(t, db, `INSERT INTO projects (id, name, code, client_name, status) VALUES ($1,'Batch7 Acceptance',$2,'Client','acceptance')`, projectID, "B7A-"+projectID[:8])
	mustExec(t, db, `
		INSERT INTO acceptance_records (id, project_id, type, title, requested_by, status, notes)
		VALUES ($1,$2,'partial','Batch7 nghiệm thu',$3,'submitted','Có lỗi cần sửa')
	`, acceptanceID, projectID, actorID)

	failed, err := repo.AcceptanceAction(ctx, acceptanceID, "fail", ActionRequest{Comment: "Không đạt"}, actorID, "Batch7 Actor")
	if err != nil {
		t.Fatal(err)
	}
	if failed["status"] != "completed" || failed["result"] != "fail" {
		t.Fatalf("acceptance = %v/%v, want completed/fail", failed["status"], failed["result"])
	}
	assertCount(t, db, `SELECT COUNT(*) FROM issues WHERE project_id=$1 AND requires_rework`, 1, projectID)

	rework, err := repo.CreateReworkFromAcceptance(ctx, acceptanceID, actorID, "Batch7 Actor")
	if err != nil {
		t.Fatal(err)
	}
	if rework["status"] != "approved" {
		t.Fatalf("rework status = %v, want approved", rework["status"])
	}
	_, err = repo.CreateReworkFromAcceptance(ctx, acceptanceID, actorID, "Batch7 Actor")
	assertValidationError(t, err)
	assertCount(t, db, `SELECT COUNT(*) FROM rework_requests WHERE acceptance_id=$1 AND status NOT IN ('cancelled','done','deleted')`, 1, acceptanceID)
	assertCount(t, db, `SELECT COUNT(*) FROM audit_logs WHERE project_id=$1 AND event_type IN ('acceptance_failed','rework_requested')`, 3, projectID)
}

func TestBatch7HandoverBlockedAuditIncludesAccountingBlockers(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)

	actorID := newID()
	projectID := newID()
	acceptanceID := newID()
	handoverID := newID()
	payableID := newID()
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM payables WHERE id=$1`, payableID)
		_, _ = db.ExecContext(ctx, `DELETE FROM handover_records WHERE id=$1`, handoverID)
		_, _ = db.ExecContext(ctx, `DELETE FROM acceptance_records WHERE id=$1`, acceptanceID)
		_, _ = db.ExecContext(ctx, `DELETE FROM projects WHERE id=$1`, projectID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id=$1`, actorID)
	})
	mustExec(t, db, `INSERT INTO users (id, email, password_hash, full_name, status) VALUES ($1,$2,'demo:pw','Batch7 Actor','active')`, actorID, "batch7-handover-"+actorID+"@test.local")
	mustExec(t, db, `INSERT INTO projects (id, name, code, client_name, status) VALUES ($1,'Batch7 Handover',$2,'Client','handover')`, projectID, "B7H-"+projectID[:8])
	mustExec(t, db, `
		INSERT INTO acceptance_records (id, project_id, type, title, requested_by, approved_by, status, result)
		VALUES ($1,$2,'final','Final acceptance',$3,$3,'completed','pass')
	`, acceptanceID, projectID, actorID)
	mustExec(t, db, `
		INSERT INTO handover_records
			(id, project_id, status, final_acceptance_id, documents_ready, warehouse_closed, payables_recorded, receivables_recorded, created_by)
		VALUES ($1,$2,'draft',$3,true,true,true,true,$4)
	`, handoverID, projectID, acceptanceID, actorID)
	mustExec(t, db, `
		INSERT INTO payables (id, project_id, payable_code, vendor_type, vendor_name, source_type, gross_amount, paid_amount, remaining_amount, status, created_by)
		VALUES ($1,$2,$3,'supplier','Batch7 Supplier','manual',100,0,100,'approved',$4)
	`, payableID, projectID, "B7P-"+payableID[:8], actorID)

	blocked, err := repo.CompleteHandover(ctx, handoverID, ActionRequest{Reason: "Kiểm tra bàn giao"}, actorID, "Batch7 Actor")
	if err != nil {
		t.Fatal(err)
	}
	if blocked["status"] != "blocked" || blocked["ready"] != false {
		t.Fatalf("handover = status %v ready %v, want blocked/false", blocked["status"], blocked["ready"])
	}
	blockers, _ := blocked["blockers"].([]string)
	if !containsString(blockers, "Còn công nợ đầu vào chưa tất toán") {
		t.Fatalf("blockers = %#v, want payable blocker", blockers)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM audit_logs WHERE project_id=$1 AND event_type='handover_blocked'`, 1, projectID)
}

func TestBatch7AccountingActionsRequireApprovalBeforeSettlement(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)

	actorID := newID()
	projectID := newID()
	payableID := newID()
	receivableID := newID()
	paymentID := newID()
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM payments WHERE id=$1`, paymentID)
		_, _ = db.ExecContext(ctx, `DELETE FROM receivables WHERE id=$1`, receivableID)
		_, _ = db.ExecContext(ctx, `DELETE FROM payables WHERE id=$1`, payableID)
		_, _ = db.ExecContext(ctx, `DELETE FROM projects WHERE id=$1`, projectID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id=$1`, actorID)
	})
	mustExec(t, db, `INSERT INTO users (id, email, password_hash, full_name, status) VALUES ($1,$2,'demo:pw','Batch7 Actor','active')`, actorID, "batch7-accounting-"+actorID+"@test.local")
	mustExec(t, db, `INSERT INTO projects (id, name, code, client_name, status) VALUES ($1,'Batch7 Accounting',$2,'Client','in_progress')`, projectID, "B7C-"+projectID[:8])
	mustExec(t, db, `
		INSERT INTO payables (id, project_id, payable_code, vendor_type, vendor_name, source_type, gross_amount, paid_amount, remaining_amount, status, created_by)
		VALUES ($1,$2,$3,'supplier','Batch7 Supplier','manual',100,0,100,'draft',$4)
	`, payableID, projectID, "B7PAY-"+payableID[:8], actorID)
	mustExec(t, db, `
		INSERT INTO receivables (id, project_id, receivable_code, client_name, source_type, gross_amount, received_amount, remaining_amount, status, created_by)
		VALUES ($1,$2,$3,'Batch7 Client','manual',120,0,120,'draft',$4)
	`, receivableID, projectID, "B7REC-"+receivableID[:8], actorID)
	mustExec(t, db, `
		INSERT INTO payments (id, project_id, payment_code, payment_type, target_type, target_id, amount, payment_method, status, created_by)
		VALUES ($1,$2,$3,'outbound','payable',$4,100,'bank_transfer','draft',$5)
	`, paymentID, projectID, "B7PMT-"+paymentID[:8], payableID, actorID)

	_, err := repo.PayableAction(ctx, payableID, "pay", ActionRequest{}, actorID, "Batch7 Actor")
	assertValidationError(t, err)
	payable, err := repo.PayableAction(ctx, payableID, "approve", ActionRequest{Comment: "Duyệt phải trả"}, actorID, "Batch7 Actor")
	if err != nil {
		t.Fatal(err)
	}
	if payable["status"] != "approved" {
		t.Fatalf("payable status = %v, want approved", payable["status"])
	}
	payable, err = repo.PayableAction(ctx, payableID, "pay", ActionRequest{Comment: "Tất toán"}, actorID, "Batch7 Actor")
	if err != nil {
		t.Fatal(err)
	}
	if payable["status"] != "paid" || number(payable["remaining_amount"]) != 0 || number(payable["paid_amount"]) != 100 {
		t.Fatalf("payable settlement = status %v paid %v remaining %v, want paid/100/0", payable["status"], payable["paid_amount"], payable["remaining_amount"])
	}

	_, err = repo.ReceivableAction(ctx, receivableID, "receive", ActionRequest{}, actorID, "Batch7 Actor")
	assertValidationError(t, err)
	receivable, err := repo.ReceivableAction(ctx, receivableID, "approve", ActionRequest{Comment: "Duyệt phải thu"}, actorID, "Batch7 Actor")
	if err != nil {
		t.Fatal(err)
	}
	if receivable["status"] != "approved" {
		t.Fatalf("receivable status = %v, want approved", receivable["status"])
	}
	receivable, err = repo.ReceivableAction(ctx, receivableID, "receive", ActionRequest{Comment: "Đã thu"}, actorID, "Batch7 Actor")
	if err != nil {
		t.Fatal(err)
	}
	if receivable["status"] != "received" || number(receivable["remaining_amount"]) != 0 || number(receivable["received_amount"]) != 120 {
		t.Fatalf("receivable settlement = status %v received %v remaining %v, want received/120/0", receivable["status"], receivable["received_amount"], receivable["remaining_amount"])
	}

	payment, err := repo.PaymentAction(ctx, paymentID, "approve", ActionRequest{Comment: "Duyệt phiếu chi"}, actorID, "Batch7 Actor")
	if err != nil {
		t.Fatal(err)
	}
	if payment["status"] != "approved" || cleanID(payment["approved_by"]) != actorID || payment["paid_at"] == nil {
		t.Fatalf("payment = status %v approved_by %v paid_at %v, want approved actor paid_at", payment["status"], payment["approved_by"], payment["paid_at"])
	}
	assertCount(t, db, `SELECT COUNT(*) FROM audit_logs WHERE project_id=$1 AND event_type IN ('payable_approved','payable_paid','receivable_approved','receivable_received','payment_approved')`, 5, projectID)
}

func TestWorkItemEvidenceRequiredBeforeReviewApprovalAndCompletion(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")
	fixture := setupWorkItemFlowFixture(t, db, true)

	_, err := service.WorkItemAction(ctx, fixture.itemID, "request-approval", WorkItemActionRequest{
		Comment:       "PM duyệt giúp",
		RequiredRoles: []string{"PM"},
	}, fixture.workerID, "Worker")
	if err == nil {
		t.Fatal("expected approval request to require evidence")
	}
	var statusErr StatusError
	if !errors.As(err, &statusErr) || statusErr.Code != "VALIDATION_ERROR" {
		t.Fatalf("expected validation error, got %#v", err)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM approval_requests WHERE target_type='work_item' AND target_id=$1`, 0, fixture.itemID)

	_, err = service.WorkItemAction(ctx, fixture.itemID, "request-review", WorkItemActionRequest{Comment: "review giúp"}, fixture.workerID, "Worker")
	if err == nil {
		t.Fatal("expected review request to require evidence")
	}
	if !errors.As(err, &statusErr) || statusErr.Code != "VALIDATION_ERROR" {
		t.Fatalf("expected validation error, got %#v", err)
	}

	evidence, err := service.AddWorkItemEvidence(ctx, fixture.itemID, WorkItemEvidenceRequest{
		EvidenceType: "document",
		Name:         "Biên bản gửi duyệt",
		FileURL:      "/evidence/bien-ban-gui-duyet.pdf",
		Visibility:   "project",
	}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	if evidence["status"] != "submitted" {
		t.Fatalf("evidence status = %v, want submitted", evidence["status"])
	}
	waiting, err := service.WorkItemAction(ctx, fixture.itemID, "request-approval", WorkItemActionRequest{
		Comment:       "PM duyệt giúp",
		RequiredRoles: []string{"PM"},
	}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	if waiting["status"] != "waiting_approval" {
		t.Fatalf("work item status = %v, want waiting_approval", waiting["status"])
	}

	completionFixture := setupWorkItemFlowFixture(t, db, true)
	_, err = service.WorkItemAction(ctx, completionFixture.itemID, "complete", WorkItemActionRequest{Comment: "done"}, completionFixture.workerID, "Worker")
	if err == nil {
		t.Fatal("expected completion to require evidence")
	}
	if !errors.As(err, &statusErr) || statusErr.Code != "VALIDATION_ERROR" {
		t.Fatalf("expected validation error, got %#v", err)
	}

	evidence, err = service.AddWorkItemEvidence(ctx, completionFixture.itemID, WorkItemEvidenceRequest{
		EvidenceType: "document",
		Name:         "Biên bản hoàn tất",
		FileURL:      "/evidence/bien-ban.pdf",
		Visibility:   "project",
	}, completionFixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	if evidence["status"] != "submitted" {
		t.Fatalf("evidence status = %v, want submitted", evidence["status"])
	}

	completed, err := service.WorkItemAction(ctx, completionFixture.itemID, "complete", WorkItemActionRequest{Comment: "done"}, completionFixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	if completed["status"] != "completed" {
		t.Fatalf("work item status = %v, want completed", completed["status"])
	}
	assertCount(t, db, `SELECT COUNT(*) FROM evidence_records WHERE work_item_id=$1 AND evidence_type='document'`, 1, completionFixture.itemID)
	assertCount(t, db, `SELECT COUNT(*) FROM work_item_activities WHERE work_item_id=$1 AND activity_type='evidence_added'`, 1, completionFixture.itemID)
	assertCount(t, db, `SELECT COUNT(*) FROM work_item_activities WHERE work_item_id=$1 AND activity_type='completed'`, 1, completionFixture.itemID)
	assertCount(t, db, `SELECT COUNT(*) FROM audit_logs WHERE project_id=$1 AND event_type IN ('work_item_evidence_added','work_item_completed')`, 2, completionFixture.projectID)
}

func TestWorkflowFormSubmitSatisfiesChecklistGate(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")
	service.ConfigureUploads(t.TempDir(), 1024*1024)
	fixture := setupWorkItemFlowFixture(t, db, false)

	mustExec(t, db, `
		UPDATE work_items
		SET evidence_requirement_json='{"required_permission":"operation.report","checklist_required":true}'::jsonb
		WHERE id=$1
	`, fixture.itemID)
	formVersionID, err := repo.DefaultWorkflowFormVersionID(ctx, "checklist", true)
	if err != nil {
		t.Fatal(err)
	}
	if formVersionID == "" {
		t.Fatal("expected default checklist form version")
	}
	instanceRows, err := repo.Query(ctx, `
		INSERT INTO workflow_form_instances (project_id, work_item_id, form_template_id, form_version_id, form_type, title, created_by)
		SELECT $1, $2, v.template_id, v.id, t.form_type, 'Checklist form', $3
		FROM workflow_form_versions v
		JOIN workflow_form_templates t ON t.id=v.template_id
		WHERE v.id=$4
		RETURNING id
	`, fixture.projectID, fixture.itemID, fixture.workerID, formVersionID)
	if err != nil {
		t.Fatal(err)
	}
	instanceID := fmt.Sprint(instanceRows[0]["id"])

	form, err := service.SubmitWorkflowForm(ctx, instanceID, WorkflowFormSaveRequest{
		Answers: Record{
			"items_done": "Đã kiểm tra checklist",
			"confirmed":  true,
		},
		Comment: "Hoàn tất checklist form",
	}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	instance, _ := form["instance"].(Record)
	if instance["status"] != "submitted" {
		t.Fatalf("form status = %#v", instance["status"])
	}
	assertCount(t, db, `SELECT COUNT(*) FROM workflow_form_answers WHERE instance_id=$1`, 2, instanceID)
	assertCount(t, db, `SELECT COUNT(*) FROM workflow_form_events WHERE instance_id=$1 AND event_type='submitted'`, 1, instanceID)
	assertCount(t, db, `SELECT COUNT(*) FROM audit_logs WHERE target_type='workflow_form_instance' AND target_id=$1`, 1, instanceID)

	completed, err := service.WorkItemAction(ctx, fixture.itemID, "complete", WorkItemActionRequest{Comment: "done"}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	if completed["status"] != "completed" {
		t.Fatalf("work item status = %v, want completed", completed["status"])
	}
}

func TestMeetingMinutesSubmitCreatesActionItemWorkItems(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")
	fixture := setupWorkItemFlowFixture(t, db, false)

	formVersionID, err := repo.DefaultWorkflowFormVersionID(ctx, "meeting_minutes", false)
	if err != nil {
		t.Fatal(err)
	}
	if formVersionID == "" {
		t.Fatal("expected default meeting minutes form version")
	}
	instanceRows, err := repo.Query(ctx, `
		INSERT INTO workflow_form_instances (project_id, work_item_id, form_template_id, form_version_id, form_type, title, created_by)
		SELECT $1, $2, v.template_id, v.id, t.form_type, 'Meeting minutes form', $3
		FROM workflow_form_versions v
		JOIN workflow_form_templates t ON t.id=v.template_id
		WHERE v.id=$4
		RETURNING id
	`, fixture.projectID, fixture.itemID, fixture.workerID, formVersionID)
	if err != nil {
		t.Fatal(err)
	}
	instanceID := fmt.Sprint(instanceRows[0]["id"])

	_, err = service.SubmitWorkflowForm(ctx, instanceID, WorkflowFormSaveRequest{
		Answers: Record{
			"meeting_title": "Họp kickoff thiết kế",
			"meeting_date":  "2026-06-30",
			"chair":         "PM",
			"attendees": []any{
				map[string]any{"name": "Nguyễn A", "role": "PM", "attendance": "present"},
			},
			"agenda": "Chốt phạm vi thiết kế",
			"decisions": []any{
				map[string]any{"content": "Dùng phương án layout A", "owner": "Thiết kế", "target": "Bản vẽ"},
			},
			"action_items": []any{
				map[string]any{"title": "Cập nhật bản vẽ layout A", "owner_role": "Thiết kế", "owner_name": "Nguyễn B", "due_date": "2026-07-02", "priority": "high", "description": "Gửi lại cho PM review"},
			},
			"confirmed": true,
		},
		Comment: "Submit biên bản họp",
	}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}

	assertCount(t, db, `SELECT COUNT(*) FROM workflow_form_answers WHERE instance_id=$1 AND field_key='action_items'`, 1, instanceID)
	assertCount(t, db, `SELECT COUNT(*) FROM work_items WHERE project_id=$1 AND source_type='workflow_form_action_item' AND source_id=$2`, 1, fixture.projectID, instanceID)
	assertCount(t, db, `
		SELECT COUNT(*)
		FROM work_item_assignments wia
		JOIN work_items wi ON wi.id=wia.work_item_id
		WHERE wi.source_type='workflow_form_action_item'
		  AND wi.source_id=$1
		  AND wia.assignment_type='role'
		  AND wia.role_name='Thiết kế'
	`, 1, instanceID)
	assertCount(t, db, `
		SELECT COUNT(*)
		FROM work_item_activities wia
		JOIN work_items wi ON wi.id=wia.work_item_id
		WHERE wi.source_type='workflow_form_action_item'
		  AND wi.source_id=$1
		  AND wia.activity_type='created'
	`, 1, instanceID)
}

func TestPlanningDailyReportSubmitCreatesNextPlanWorkItems(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")
	fixture := setupWorkItemFlowFixture(t, db, false)

	formVersionID, err := repo.DefaultWorkflowFormVersionID(ctx, "daily_report", false)
	if err != nil {
		t.Fatal(err)
	}
	if formVersionID == "" {
		t.Fatal("expected default daily report form version")
	}
	var versionNo int
	if err := db.QueryRowContext(ctx, `SELECT version_no FROM workflow_form_versions WHERE id=$1`, formVersionID).Scan(&versionNo); err != nil {
		t.Fatal(err)
	}
	if versionNo != 2 {
		t.Fatalf("daily report form version = %d, want 2", versionNo)
	}
	instanceRows, err := repo.Query(ctx, `
		INSERT INTO workflow_form_instances (project_id, work_item_id, form_template_id, form_version_id, form_type, title, created_by)
		SELECT $1, $2, v.template_id, v.id, t.form_type, 'Planning daily report form', $3
		FROM workflow_form_versions v
		JOIN workflow_form_templates t ON t.id=v.template_id
		WHERE v.id=$4
		RETURNING id
	`, fixture.projectID, fixture.itemID, fixture.workerID, formVersionID)
	if err != nil {
		t.Fatal(err)
	}
	instanceID := fmt.Sprint(instanceRows[0]["id"])

	_, err = service.SubmitWorkflowForm(ctx, instanceID, WorkflowFormSaveRequest{
		Answers: Record{
			"report_date":      "2026-07-01",
			"reported_by":      "PM",
			"planning_scope":   "design",
			"progress_percent": 35,
			"completed_items": []any{
				map[string]any{"title": "Chốt scope thiết kế", "owner_role": "PM", "result": "done", "note": "Đã xong"},
			},
			"next_plan_items": []any{
				map[string]any{"title": "Chuẩn bị bản vẽ concept", "owner_role": "Thiết kế", "owner_name": "Nguyễn B", "due_date": "2026-07-03", "priority": "high", "description": "Review cùng PM"},
			},
			"blockers": []any{
				map[string]any{"content": "Chờ khách xác nhận vật liệu", "impact": "schedule", "owner_role": "PM", "target_date": "2026-07-02", "severity": "medium"},
			},
			"confirmed": true,
		},
		Comment: "Submit planning daily report",
	}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}

	assertCount(t, db, `SELECT COUNT(*) FROM workflow_form_answers WHERE instance_id=$1 AND field_key='next_plan_items'`, 1, instanceID)
	assertCount(t, db, `SELECT COUNT(*) FROM workflow_form_answers WHERE instance_id=$1 AND field_key='blockers'`, 1, instanceID)
	assertCount(t, db, `SELECT COUNT(*) FROM work_items WHERE project_id=$1 AND source_type='workflow_form_action_item' AND source_id=$2`, 1, fixture.projectID, instanceID)
	assertCount(t, db, `
		SELECT COUNT(*)
		FROM work_item_assignments wia
		JOIN work_items wi ON wi.id=wia.work_item_id
		WHERE wi.source_type='workflow_form_action_item'
		  AND wi.source_id=$1
		  AND wia.assignment_type='role'
		  AND wia.role_name='Thiết kế'
	`, 1, instanceID)
	assertCount(t, db, `
		SELECT COUNT(*)
		FROM work_item_activities wia
		JOIN work_items wi ON wi.id=wia.work_item_id
		WHERE wi.source_type='workflow_form_action_item'
		  AND wi.source_id=$1
		  AND wia.activity_type='created'
		  AND wia.comment='Next plan item generated from planning daily report'
	`, 1, instanceID)
	assertCount(t, db, `
		SELECT COUNT(*)
		FROM work_item_activities
		WHERE work_item_id=$1
		  AND activity_type='system_event'
		  AND comment='Planning daily report generated 1 next plan work items'
	`, 1, fixture.itemID)
}

func TestPlanningChecklistSubmitRequiresClearItemsAndValidEvidence(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")
	fixture := setupWorkItemFlowFixture(t, db, false)

	formVersionID, err := repo.DefaultWorkflowFormVersionID(ctx, "checklist", true)
	if err != nil {
		t.Fatal(err)
	}
	if formVersionID == "" {
		t.Fatal("expected default checklist form version")
	}
	var versionNo int
	if err := db.QueryRowContext(ctx, `SELECT version_no FROM workflow_form_versions WHERE id=$1`, formVersionID).Scan(&versionNo); err != nil {
		t.Fatal(err)
	}
	if versionNo != 2 {
		t.Fatalf("checklist form version = %d, want 2", versionNo)
	}
	evidence, err := service.AddWorkItemEvidence(ctx, fixture.itemID, WorkItemEvidenceRequest{
		EvidenceType: "checklist",
		Name:         "Checklist planning evidence",
		Visibility:   "project",
	}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	evidenceID := fmt.Sprint(evidence["id"])
	instanceRows, err := repo.Query(ctx, `
		INSERT INTO workflow_form_instances (project_id, work_item_id, form_template_id, form_version_id, form_type, title, created_by)
		SELECT $1, $2, v.template_id, v.id, t.form_type, 'Planning checklist form', $3
		FROM workflow_form_versions v
		JOIN workflow_form_templates t ON t.id=v.template_id
		WHERE v.id=$4
		RETURNING id
	`, fixture.projectID, fixture.itemID, fixture.workerID, formVersionID)
	if err != nil {
		t.Fatal(err)
	}
	instanceID := fmt.Sprint(instanceRows[0]["id"])

	blockedAnswers := Record{
		"check_date":     "2026-07-04",
		"checked_by":     "PM",
		"planning_scope": "design",
		"checklist_items": []any{
			map[string]any{
				"title":         "Chốt layout concept",
				"assignee_role": "Thiết kế",
				"assignee_name": "Nguyễn B",
				"status":        "needs_follow_up",
				"due_date":      "2026-07-05",
				"evidence_id":   evidenceID,
				"comment":       "Cần cập nhật thêm",
			},
		},
		"overall_comment": "Chưa đủ điều kiện submit",
		"confirmed":       true,
	}
	_, err = service.SubmitWorkflowForm(ctx, instanceID, WorkflowFormSaveRequest{Answers: blockedAnswers}, fixture.workerID, "Worker")
	assertValidationError(t, err)

	invalidEvidenceAnswers := Record{
		"check_date":     "2026-07-04",
		"checked_by":     "PM",
		"planning_scope": "design",
		"checklist_items": []any{
			map[string]any{
				"title":         "Chốt layout concept",
				"assignee_role": "Thiết kế",
				"status":        "done",
				"evidence_id":   newID(),
			},
		},
		"confirmed": true,
	}
	_, err = service.SubmitWorkflowForm(ctx, instanceID, WorkflowFormSaveRequest{Answers: invalidEvidenceAnswers}, fixture.workerID, "Worker")
	assertValidationError(t, err)

	readyAnswers := Record{
		"check_date":     "2026-07-04",
		"checked_by":     "PM",
		"planning_scope": "design",
		"checklist_items": []any{
			map[string]any{
				"title":         "Chốt layout concept",
				"assignee_role": "Thiết kế",
				"assignee_name": "Nguyễn B",
				"status":        "done",
				"due_date":      "2026-07-05",
				"evidence_id":   evidenceID,
				"comment":       "Đã đối chiếu evidence",
			},
		},
		"overall_comment": "Đủ điều kiện submit",
		"confirmed":       true,
	}
	form, err := service.SubmitWorkflowForm(ctx, instanceID, WorkflowFormSaveRequest{Answers: readyAnswers}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	instance, _ := form["instance"].(Record)
	if instance["status"] != "submitted" {
		t.Fatalf("form status = %#v, want submitted", instance["status"])
	}
	assertCount(t, db, `SELECT COUNT(*) FROM workflow_form_answers WHERE instance_id=$1 AND field_key='checklist_items'`, 1, instanceID)
	assertCount(t, db, `SELECT COUNT(*) FROM workflow_form_events WHERE instance_id=$1 AND event_type='submitted'`, 1, instanceID)
	assertCount(t, db, `SELECT COUNT(*) FROM work_items WHERE id=$1 AND completion_payload_json ? 'checklist_items'`, 1, fixture.itemID)
}

func TestWorkflowFormSignatureRequiresSubmittedFormAndIsImmutable(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")
	fixture := setupWorkItemFlowFixture(t, db, false)

	formVersionID, err := repo.DefaultWorkflowFormVersionID(ctx, "task", false)
	if err != nil {
		t.Fatal(err)
	}
	if formVersionID == "" {
		t.Fatal("expected default generic workflow form version")
	}
	instanceRows, err := repo.Query(ctx, `
		INSERT INTO workflow_form_instances (project_id, work_item_id, form_template_id, form_version_id, form_type, title, created_by)
		SELECT $1, $2, v.template_id, v.id, t.form_type, 'Generic submission form', $3
		FROM workflow_form_versions v
		JOIN workflow_form_templates t ON t.id=v.template_id
		WHERE v.id=$4
		RETURNING id
	`, fixture.projectID, fixture.itemID, fixture.workerID, formVersionID)
	if err != nil {
		t.Fatal(err)
	}
	instanceID := fmt.Sprint(instanceRows[0]["id"])

	_, err = service.SignWorkflowForm(ctx, instanceID, WorkflowFormSignRequest{Comment: "Ký khi chưa submit"}, fixture.workerID, "Worker")
	assertValidationError(t, err)

	_, err = service.SubmitWorkflowForm(ctx, instanceID, WorkflowFormSaveRequest{
		Answers: Record{"summary": "Đã hoàn tất nội dung cần submit"},
		Comment: "Submit để ký nội bộ",
	}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	form, err := service.SignWorkflowForm(ctx, instanceID, testSignatureRequest("Đã kiểm tra và xác nhận nội bộ"), fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	signatures, ok := form["signatures"].([]Record)
	if !ok || len(signatures) != 1 {
		t.Fatalf("expected one signature, got %#v", form["signatures"])
	}
	signatureID := cleanID(signatures[0]["id"])
	if signatureID == "" {
		t.Fatalf("signature missing id: %#v", signatures[0])
	}
	if signatures[0]["purpose"] != "internal_ack" {
		t.Fatalf("signature purpose = %#v, want internal_ack", signatures[0]["purpose"])
	}
	if hash := fmt.Sprint(signatures[0]["payload_hash"]); len(hash) != 64 {
		t.Fatalf("payload_hash = %q, want 64 hex chars", hash)
	}
	if signatures[0]["visual_storage_provider"] != "local" || cleanID(signatures[0]["visual_storage_key"]) == "" {
		t.Fatalf("visual storage missing: %#v", signatures[0])
	}
	if hash := fmt.Sprint(signatures[0]["visual_checksum_sha256"]); len(hash) != 64 {
		t.Fatalf("visual_checksum_sha256 = %q, want 64 hex chars", hash)
	}
	visualFile, err := service.WorkflowFormSignatureVisual(ctx, signatureID, fixture.workerID)
	if err != nil {
		t.Fatal(err)
	}
	if visualFile.MIMEType != "image/png" {
		t.Fatalf("visual mime = %q, want image/png", visualFile.MIMEType)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM workflow_form_events WHERE instance_id=$1 AND event_type='signed'`, 1, instanceID)
	assertCount(t, db, `SELECT COUNT(*) FROM work_item_activities WHERE work_item_id=$1 AND activity_type='system_event' AND metadata_json->>'signature_id'=$2`, 1, fixture.itemID, signatureID)
	assertCount(t, db, `SELECT COUNT(*) FROM audit_logs WHERE target_type='workflow_form_signature' AND target_id=$1 AND event_type='workflow_form_signed'`, 1, signatureID)

	pdfForm, err := service.GenerateWorkflowFormPDFExport(ctx, instanceID, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	pdfExports, ok := pdfForm["pdf_exports"].([]Record)
	if !ok || len(pdfExports) != 1 {
		t.Fatalf("expected one PDF export, got %#v", pdfForm["pdf_exports"])
	}
	pdfExportID := cleanID(pdfExports[0]["id"])
	if pdfExportID == "" {
		t.Fatalf("PDF export missing id: %#v", pdfExports[0])
	}
	if hash := fmt.Sprint(pdfExports[0]["checksum_sha256"]); len(hash) != 64 {
		t.Fatalf("PDF checksum = %q, want 64 hex chars", hash)
	}
	pdfFile, err := service.WorkflowFormPDFExportFile(ctx, pdfExportID, fixture.workerID)
	if err != nil {
		t.Fatal(err)
	}
	pdfBytes, err := os.ReadFile(pdfFile.Path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(pdfBytes, []byte("%PDF-1.4")) {
		t.Fatalf("expected PDF bytes, got %q", pdfBytes[:8])
	}
	assertCount(t, db, `SELECT COUNT(*) FROM workflow_form_events WHERE instance_id=$1 AND event_type='pdf_exported'`, 1, instanceID)
	assertCount(t, db, `SELECT COUNT(*) FROM audit_logs WHERE target_type='workflow_form_pdf_export' AND target_id=$1 AND event_type='workflow_form_pdf_exported'`, 1, pdfExportID)

	_, err = service.SignWorkflowForm(ctx, instanceID, testSignatureRequest("Duplicate"), fixture.workerID, "Worker")
	assertValidationError(t, err)

	if _, err := db.ExecContext(ctx, `UPDATE workflow_form_signatures SET comment='mutated' WHERE id=$1`, signatureID); err == nil {
		t.Fatal("expected immutable workflow form signature update to fail")
	}
	if _, err := db.ExecContext(ctx, `UPDATE workflow_form_pdf_exports SET title='mutated' WHERE id=$1`, pdfExportID); err == nil {
		t.Fatal("expected immutable workflow form PDF export update to fail")
	}
}

func TestWorkItemApprovalRequiresDecisionScope(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")
	fixture := setupWorkItemFlowFixture(t, db, false)

	waiting, err := service.WorkItemAction(ctx, fixture.itemID, "request-approval", WorkItemActionRequest{
		Comment:       "PM duyệt giúp",
		RequiredRoles: []string{"PM"},
	}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	if waiting["status"] != "waiting_approval" {
		t.Fatalf("work item status = %v, want waiting_approval", waiting["status"])
	}
	approvalID := cleanID(waiting["approval_request_id"])
	if approvalID == "" {
		t.Fatal("expected approval_request_id")
	}

	_, err = service.ApprovalAction(ctx, approvalID, "approve", "self approve", fixture.workerID, "Worker")
	if err == nil {
		t.Fatal("expected worker self approval to be forbidden")
	}
	var statusErr StatusError
	if !errors.As(err, &statusErr) || statusErr.Code != "FORBIDDEN" {
		t.Fatalf("expected forbidden error, got %#v", err)
	}

	approval, err := service.ApprovalAction(ctx, approvalID, "approve", "approved by PM", fixture.pmID, "PM")
	if err != nil {
		t.Fatal(err)
	}
	if approval["status"] != "approved" {
		t.Fatalf("approval status = %v, want approved", approval["status"])
	}
	approved, err := repo.GetWorkItemDetail(ctx, fixture.itemID)
	if err != nil {
		t.Fatal(err)
	}
	if approved["status"] != "completed" {
		t.Fatalf("work item status = %v, want completed", approved["status"])
	}
	assertCount(t, db, `SELECT COUNT(*) FROM approval_requests WHERE id=$1 AND status='approved'`, 1, approvalID)
	assertCount(t, db, `SELECT COUNT(*) FROM approval_actions WHERE approval_request_id=$1 AND action='approve'`, 1, approvalID)
	assertCount(t, db, `SELECT COUNT(*) FROM work_item_activities WHERE work_item_id=$1 AND activity_type='approval_requested'`, 1, fixture.itemID)
	assertCount(t, db, `SELECT COUNT(*) FROM work_item_activities WHERE work_item_id=$1 AND activity_type='approved'`, 1, fixture.itemID)
	assertCount(t, db, `SELECT COUNT(*) FROM audit_logs WHERE project_id=$1 AND event_type IN ('work_item_approval_requested','work_item_approved')`, 2, fixture.projectID)
}

func TestWorkItemReviewSelfApprovalForbidden(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")
	fixture := setupWorkItemFlowFixture(t, db, false)

	waiting, err := service.WorkItemAction(ctx, fixture.itemID, "request-review", WorkItemActionRequest{Comment: "review giúp"}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	if waiting["status"] != "waiting_review" {
		t.Fatalf("work item status = %v, want waiting_review", waiting["status"])
	}

	_, err = service.WorkItemAction(ctx, fixture.itemID, "approve", WorkItemActionRequest{Comment: "self review"}, fixture.workerID, "Worker")
	if err == nil {
		t.Fatal("expected requester review approval to be forbidden")
	}
	var statusErr StatusError
	if !errors.As(err, &statusErr) || statusErr.Code != "FORBIDDEN" {
		t.Fatalf("expected forbidden error, got %#v", err)
	}

	approved, err := service.WorkItemAction(ctx, fixture.itemID, "approve", WorkItemActionRequest{Comment: "PM reviewed"}, fixture.pmID, "PM")
	if err != nil {
		t.Fatal(err)
	}
	if approved["status"] != "completed" {
		t.Fatalf("work item status = %v, want completed", approved["status"])
	}
	assertCount(t, db, `SELECT COUNT(*) FROM work_item_activities WHERE work_item_id=$1 AND activity_type='review_requested'`, 1, fixture.itemID)
	assertCount(t, db, `SELECT COUNT(*) FROM work_item_activities WHERE work_item_id=$1 AND activity_type='approved'`, 1, fixture.itemID)
}

func TestWorkItemGenericApprovalRequestChangeKeepsItemInSync(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")
	fixture := setupWorkItemFlowFixture(t, db, false)

	waiting, err := service.WorkItemAction(ctx, fixture.itemID, "request-approval", WorkItemActionRequest{
		Comment:       "PM xem lại",
		RequiredRoles: []string{"PM"},
	}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	approvalID := cleanID(waiting["approval_request_id"])
	if approvalID == "" {
		t.Fatal("expected approval_request_id")
	}

	approval, err := service.ApprovalAction(ctx, approvalID, "request_change", "Bổ sung biên bản", fixture.pmID, "PM")
	if err != nil {
		t.Fatal(err)
	}
	if approval["status"] != "changes_requested" {
		t.Fatalf("approval status = %v, want changes_requested", approval["status"])
	}
	item, err := repo.GetWorkItemDetail(ctx, fixture.itemID)
	if err != nil {
		t.Fatal(err)
	}
	if item["status"] != "needs_changes" {
		t.Fatalf("work item status = %v, want needs_changes", item["status"])
	}
	assertCount(t, db, `SELECT COUNT(*) FROM approval_actions WHERE approval_request_id=$1 AND action='request-change'`, 1, approvalID)
	assertCount(t, db, `SELECT COUNT(*) FROM work_item_activities WHERE work_item_id=$1 AND activity_type='changes_requested'`, 1, fixture.itemID)
}

func TestGenericApprovalActionRequiresDecisionScope(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")
	fixture := setupWorkItemFlowFixture(t, db, false)
	approvalID := newID()
	mustExec(t, db, `
		INSERT INTO approval_requests (id, target_type, target_id, project_id, requested_by, status, reason, required_roles)
		VALUES ($1,'procurement_request',NULL,$2,$3,'pending','Duyệt mua sắm','["PM"]'::jsonb)
	`, approvalID, fixture.projectID, fixture.workerID)

	_, err := service.ApprovalAction(ctx, approvalID, "approve", "self approve", fixture.workerID, "Worker")
	if err == nil {
		t.Fatal("expected generic approval requester to be forbidden")
	}
	var statusErr StatusError
	if !errors.As(err, &statusErr) || statusErr.Code != "FORBIDDEN" {
		t.Fatalf("expected forbidden error, got %#v", err)
	}

	approved, err := service.ApprovalAction(ctx, approvalID, "approve", "approved by PM", fixture.pmID, "PM")
	if err != nil {
		t.Fatal(err)
	}
	if approved["status"] != "approved" {
		t.Fatalf("approval status = %v, want approved", approved["status"])
	}
	assertCount(t, db, `SELECT COUNT(*) FROM approval_actions WHERE approval_request_id=$1 AND action='approve'`, 1, approvalID)
	assertCount(t, db, `SELECT COUNT(*) FROM audit_logs WHERE project_id=$1 AND event_type='approval_granted'`, 1, fixture.projectID)
}

func mustExec(t *testing.T, db *sql.DB, query string, args ...any) {
	t.Helper()
	if _, err := db.ExecContext(context.Background(), query, args...); err != nil {
		t.Fatal(err)
	}
}

func assertCount(t *testing.T, db *sql.DB, query string, want int, args ...any) {
	t.Helper()
	var got int
	if err := db.QueryRowContext(context.Background(), query, args...).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("count mismatch for %q: got %d want %d", query, got, want)
	}
}

func assertValidationError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected validation error")
	}
	var statusErr StatusError
	if !errors.As(err, &statusErr) || statusErr.Code != "VALIDATION_ERROR" {
		t.Fatalf("expected validation error, got %#v", err)
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
