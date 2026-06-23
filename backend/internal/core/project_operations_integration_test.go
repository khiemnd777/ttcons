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

	"consflow/backend/internal/shared/middleware"
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

func TestCashflowCSVIncludesLedgerRows(t *testing.T) {
	payload, err := cashflowCSV(Record{"ledger": []Record{{
		"occurred_at":    "2026-06-03T10:00:00Z",
		"payment_code":   "PMT-CSV-001",
		"project_name":   "CSV Project",
		"category":       "Thu khách hàng",
		"direction":      "inflow",
		"payment_type":   "client_receipt",
		"payment_method": "bank_transfer",
		"status":         "approved",
		"inflow_amount":  100,
		"outflow_amount": 0,
		"net_amount":     100,
		"target_type":    "receivable",
	}}})
	if err != nil {
		t.Fatal(err)
	}
	text := string(payload)
	if !strings.Contains(text, "payment_code") || !strings.Contains(text, "PMT-CSV-001") {
		t.Fatalf("csv payload missing header or row: %s", text)
	}
}

func TestTaxCSVIncludesLedgerRows(t *testing.T) {
	payload, err := taxCSV(Record{"ledger": []Record{{
		"invoice_date":      "2026-06-05",
		"invoice_number":    "VAT-CSV-001",
		"direction":         "output",
		"project_name":      "CSV Project",
		"counterparty_name": "CSV Client",
		"tax_code":          "0311111111",
		"amount_before_vat": 1000,
		"vat_amount":        100,
		"total_amount":      1100,
		"status":            "issued",
		"source_type":       "receivable",
		"source_code":       "REC-CSV-001",
		"document_name":     "VAT PDF",
		"warning_flags":     "",
	}}})
	if err != nil {
		t.Fatal(err)
	}
	text := string(payload)
	if !strings.Contains(text, "invoice_number") || !strings.Contains(text, "VAT-CSV-001") {
		t.Fatalf("csv payload missing header or row: %s", text)
	}
}

func TestProfitCSVIncludesLedgerRows(t *testing.T) {
	payload, err := profitCSV(Record{"ledger": []Record{{
		"invoice_date":          "2026-06-06",
		"invoice_number":        "PNL-CSV-001",
		"direction":             "revenue",
		"cost_type":             "revenue",
		"project_name":          "CSV Project",
		"counterparty_name":     "CSV Client",
		"amount_before_vat":     1000,
		"revenue_amount":        1000,
		"direct_cost_amount":    0,
		"indirect_cost_amount":  0,
		"status":                "issued",
		"source_type":           "receivable",
		"source_code":           "REC-CSV-001",
		"classification_reason": "issued_output_invoice",
		"warning_flags":         "",
	}}})
	if err != nil {
		t.Fatal(err)
	}
	text := string(payload)
	if !strings.Contains(text, "invoice_number") || !strings.Contains(text, "PNL-CSV-001") {
		t.Fatalf("csv payload missing header or row: %s", text)
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
					map[string]any{"key": "quantity", "label": "Khối lượng", "type": "number", "required": true, "min": float64(0)},
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

	_, err = service.validateWorkflowFormAnswers(context.Background(), Record{}, fields, Record{
		"action_items": []any{map[string]any{"title": "Chốt bản vẽ", "quantity": "-1", "due_date": "2026-06-30", "priority": "normal"}},
	}, true)
	assertValidationError(t, err)

	answers, err := service.validateWorkflowFormAnswers(context.Background(), Record{}, fields, Record{
		"action_items": []any{map[string]any{"title": "  Chốt bản vẽ  ", "quantity": "2.5", "due_date": "2026-06-30", "priority": "high"}},
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
	if number(items[0]["quantity"]) != 2.5 {
		t.Fatalf("normalized quantity = %#v", items[0]["quantity"])
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

func TestDefaultWorkflowFormCodePrefersSpecificStepTypes(t *testing.T) {
	if got := defaultWorkflowFormCode("daily_report", true, "Checklist thầu phụ"); got != "builtin_daily_report" {
		t.Fatalf("daily report default form = %s", got)
	}
	if got := defaultWorkflowFormCode("daily_report", false, "Daily report thi công"); got != "builtin_construction_daily_report" {
		t.Fatalf("construction daily report default form = %s", got)
	}
	if got := defaultWorkflowFormCode("daily_report", false, "Nhật ký công trường"); got != "builtin_construction_daily_report" {
		t.Fatalf("site daily report default form = %s", got)
	}
	if got := defaultWorkflowFormCode("checklist", true, "Checklist chủ đầu tư"); got != "builtin_owner_client_checklist" {
		t.Fatalf("owner/client checklist default form = %s", got)
	}
	if got := defaultWorkflowFormCode("checklist", true, "Client checklist"); got != "builtin_owner_client_checklist" {
		t.Fatalf("client checklist default form = %s", got)
	}
	if got := defaultWorkflowFormCode("checklist", true, "Checklist thầu phụ"); got != "builtin_subcontractor_checklist" {
		t.Fatalf("subcontractor checklist default form = %s", got)
	}
	if got := defaultWorkflowFormCode("checklist", true, "Checklist nghiệm thu"); got != "builtin_checklist" {
		t.Fatalf("generic checklist default form = %s", got)
	}
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

func TestAuthSessionRefreshAndLogoutRevokesAccess(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")
	userID := newID()
	suffix := userID[:8]
	email := "auth-session-" + suffix + "@test.local"
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM auth_sessions WHERE user_id=$1`, userID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id=$1`, userID)
	})
	mustExec(t, db, `INSERT INTO users (id, email, password_hash, full_name, status) VALUES ($1,$2,'demo:pw','Auth Session','active')`, userID, email)

	login, err := service.Login(ctx, LoginRequest{Email: email, Password: "pw"}, "test-agent", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	accessToken := fmt.Sprint(login["token"])
	refreshToken := fmt.Sprint(login["refresh_token"])
	if accessToken == "" || refreshToken == "" {
		t.Fatalf("login tokens missing: %#v", login)
	}

	handler := NewHandler(service)
	app := fiber.New(fiber.Config{ErrorHandler: ErrorHandler})
	app.Use(middleware.AuthRequired("test-secret", repo))
	app.Get("/me", handler.Me)
	assertBearerStatus(t, app, "/me", accessToken, http.StatusOK)

	refreshed, err := service.RefreshSession(ctx, refreshToken, "test-agent", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	nextAccessToken := fmt.Sprint(refreshed["token"])
	nextRefreshToken := fmt.Sprint(refreshed["refresh_token"])
	assertBearerStatus(t, app, "/me", accessToken, http.StatusUnauthorized)
	assertBearerStatus(t, app, "/me", nextAccessToken, http.StatusOK)

	if err := service.LogoutByRefreshToken(ctx, nextRefreshToken); err != nil {
		t.Fatal(err)
	}
	assertBearerStatus(t, app, "/me", nextAccessToken, http.StatusUnauthorized)
}

func TestRBACAssignmentsReplaceSnapshots(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	suffix := newID()
	roleID := ensureRole(t, db, "rbac_replace_"+suffix[:8], "RBAC Replace")
	firstPermissionID := ensurePermission(t, db, "rbac.replace.first."+suffix[:8], "RBAC Replace First", "rbac")
	secondPermissionID := ensurePermission(t, db, "rbac.replace.second."+suffix[:8], "RBAC Replace Second", "rbac")
	userID := newID()
	projectID := newID()
	staffID := newID()
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM project_members WHERE project_id=$1`, projectID)
		_, _ = db.ExecContext(ctx, `DELETE FROM projects WHERE id=$1`, projectID)
		_, _ = db.ExecContext(ctx, `DELETE FROM user_roles WHERE user_id=$1`, userID)
		_, _ = db.ExecContext(ctx, `DELETE FROM role_permissions WHERE role_id=$1`, roleID)
		_, _ = db.ExecContext(ctx, `DELETE FROM roles WHERE id=$1`, roleID)
		_, _ = db.ExecContext(ctx, `DELETE FROM staff WHERE id=$1`, staffID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id=$1`, userID)
	})
	mustExec(t, db, `INSERT INTO users (id, email, password_hash, full_name, status) VALUES ($1,$2,'demo:pw','RBAC User','active')`, userID, "rbac-user-"+suffix+"@test.local")
	mustExec(t, db, `INSERT INTO staff (id, user_id, full_name, email, department, position, status) VALUES ($1,$2,'RBAC User',$3,'Dự án','Member','active')`, staffID, userID, "rbac-staff-"+suffix+"@test.local")
	mustExec(t, db, `INSERT INTO projects (id, name, code, client_name, status) VALUES ($1,'RBAC Scoped',$2,'Client','planning')`, projectID, "RBA-"+suffix[:8])

	if _, err := repo.ReplaceRolePermissions(ctx, roleID, []string{firstPermissionID, secondPermissionID}); err != nil {
		t.Fatal(err)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM role_permissions WHERE role_id=$1`, 2, roleID)
	if _, err := repo.ReplaceRolePermissions(ctx, roleID, []string{secondPermissionID}); err != nil {
		t.Fatal(err)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM role_permissions WHERE role_id=$1 AND permission_id=$2`, 0, roleID, firstPermissionID)
	assertCount(t, db, `SELECT COUNT(*) FROM role_permissions WHERE role_id=$1 AND permission_id=$2`, 1, roleID, secondPermissionID)

	if _, err := repo.ReplaceUserRoles(ctx, userID, []string{roleID}, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.ReplaceUserRoles(ctx, userID, []string{roleID}, projectID); err != nil {
		t.Fatal(err)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM user_roles WHERE user_id=$1 AND project_id IS NULL`, 1, userID)
	assertCount(t, db, `SELECT COUNT(*) FROM user_roles WHERE user_id=$1 AND project_id=$2`, 1, userID, projectID)
	if _, err := repo.ReplaceUserRoles(ctx, userID, []string{}, ""); err != nil {
		t.Fatal(err)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM user_roles WHERE user_id=$1 AND project_id IS NULL`, 0, userID)
	assertCount(t, db, `SELECT COUNT(*) FROM user_roles WHERE user_id=$1 AND project_id=$2`, 1, userID, projectID)
}

func TestBatch4AccountingListUsesProjectScope(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	repo := NewRepository(db)
	handler := NewHandler(NewService(repo, "test-secret"))
	fixture := setupProjectScopeFixture(t, db, []string{"accounting.view", "accounting.payable_view"})
	mustExec(t, db, `
		INSERT INTO payables (project_id, payable_code, vendor_type, vendor_name, source_type, gross_amount, remaining_amount, due_date, status, created_by)
		VALUES ($1,$2,'supplier','Scoped Vendor','manual',100,100,'2000-01-01','approved',$3),
		       ($4,$5,'supplier','Other Vendor','manual',200,200,'2000-01-01','approved',$3)
	`, fixture.projectID, "PAY-S-"+fixture.projectID[:8], fixture.userID, fixture.otherProjectID, "PAY-O-"+fixture.otherProjectID[:8])
	app := batch4TestApp(fixture.userID, func(app *fiber.App, h *Handler) {
		app.Get("/payables", h.ListPayables)
	}, handler)

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/payables?project_id="+fixture.projectID, nil))
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
	if body.Data[0]["aging_bucket"] != "90_plus" {
		t.Fatalf("aging_bucket = %v, want 90_plus", body.Data[0]["aging_bucket"])
	}
	if number(body.Data[0]["overdue_days"]) <= 0 {
		t.Fatalf("overdue_days = %v, want positive", body.Data[0]["overdue_days"])
	}
	if body.Data[0]["reconciliation_status"] != "needs_review" {
		t.Fatalf("reconciliation_status = %v, want needs_review", body.Data[0]["reconciliation_status"])
	}
}

func TestReceivableListUsesProjectScopeAndAging(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	repo := NewRepository(db)
	handler := NewHandler(NewService(repo, "test-secret"))
	fixture := setupProjectScopeFixture(t, db, []string{"accounting.view", "accounting.receivable_view"})
	mustExec(t, db, `
		INSERT INTO receivables (project_id, receivable_code, client_name, source_type, gross_amount, received_amount, remaining_amount, due_date, status, created_by)
		VALUES ($1,$2,'Scoped Client','manual',100,0,100,'2000-01-01','approved',$3),
		       ($4,$5,'Other Client','manual',200,0,200,'2000-01-01','approved',$3)
	`, fixture.projectID, "REC-S-"+fixture.projectID[:8], fixture.userID, fixture.otherProjectID, "REC-O-"+fixture.otherProjectID[:8])
	app := batch4TestApp(fixture.userID, func(app *fiber.App, h *Handler) {
		app.Get("/receivables", h.ListReceivables)
	}, handler)

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/receivables?project_id="+fixture.projectID, nil))
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
		t.Fatalf("expected one scoped receivable, got %d", len(body.Data))
	}
	if body.Data[0]["project_id"] != fixture.projectID {
		t.Fatalf("project_id = %v, want %s", body.Data[0]["project_id"], fixture.projectID)
	}
	if body.Data[0]["aging_bucket"] != "90_plus" {
		t.Fatalf("aging_bucket = %v, want 90_plus", body.Data[0]["aging_bucket"])
	}
	if number(body.Data[0]["overdue_days"]) <= 0 {
		t.Fatalf("overdue_days = %v, want positive", body.Data[0]["overdue_days"])
	}
	if body.Data[0]["reconciliation_status"] != "needs_review" {
		t.Fatalf("reconciliation_status = %v, want needs_review", body.Data[0]["reconciliation_status"])
	}
}

func TestCashflowReportSummarizesScopedPayments(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	service := NewService(NewRepository(db), "test-secret")
	fixture := setupProjectScopeFixture(t, db, []string{"report.finance.view", "accounting.view"})
	mustExec(t, db, `
		INSERT INTO payments (id, project_id, payment_code, payment_type, target_type, target_id, amount, payment_method, status, paid_at, created_by)
		VALUES
			($1,$2,'PMT-CF-OPEN','client_receipt','receivable',NULL,20,'bank_transfer','approved','2026-05-20T10:00:00Z',$3),
			($4,$2,'PMT-CF-IN','client_receipt','receivable',NULL,100,'bank_transfer','approved','2026-06-03T10:00:00Z',$3),
			($5,$2,'PMT-CF-OUT','supplier_payment','payable',NULL,40,'bank_transfer','approved','2026-06-03T11:00:00Z',$3),
			($6,$2,'PMT-CF-OTHER','other','',NULL,10,'cash','approved','2026-06-04T09:00:00Z',$3),
			($7,$2,'PMT-CF-PENDING','client_receipt','receivable',NULL,30,'bank_transfer','pending','2026-06-05T09:00:00Z',$3),
			($8,$9,'PMT-CF-OTHER-PROJECT','client_receipt','receivable',NULL,999,'bank_transfer','approved','2026-06-03T10:00:00Z',$3)
	`, newID(), fixture.projectID, fixture.userID, newID(), newID(), newID(), newID(), newID(), fixture.otherProjectID)

	report, err := service.CashflowReport(ctx, CashflowReportRequest{
		ProjectID: fixture.projectID,
		FromDate:  "2026-06-01",
		ToDate:    "2026-06-30",
		GroupBy:   "day",
		Status:    "approved",
	}, fixture.userID)
	if err != nil {
		t.Fatal(err)
	}
	summary, _ := report["summary"].(Record)
	series, _ := report["series"].([]Record)
	categories, _ := report["categories"].([]Record)
	ledger, _ := report["ledger"].([]Record)
	if number(summary["opening_balance"]) != 20 {
		t.Fatalf("opening_balance = %v, want 20", summary["opening_balance"])
	}
	if number(summary["total_inflow"]) != 100 || number(summary["total_outflow"]) != 40 || number(summary["net_flow"]) != 60 {
		t.Fatalf("cashflow summary = inflow %v outflow %v net %v, want 100/40/60", summary["total_inflow"], summary["total_outflow"], summary["net_flow"])
	}
	if number(summary["closing_balance"]) != 80 {
		t.Fatalf("closing_balance = %v, want 80", summary["closing_balance"])
	}
	if number(summary["payment_count"]) != 3 || number(summary["unclassified_count"]) != 1 {
		t.Fatalf("counts = payment %v unclassified %v, want 3/1", summary["payment_count"], summary["unclassified_count"])
	}
	if number(summary["pending_payment_count"]) != 1 || number(summary["pending_payment_total"]) != 30 {
		t.Fatalf("pending = count %v total %v, want 1/30", summary["pending_payment_count"], summary["pending_payment_total"])
	}
	if len(series) < 1 {
		t.Fatal("expected at least one cashflow series row")
	}
	if len(categories) < 3 {
		t.Fatalf("categories = %#v, want at least inflow/outflow/unclassified", categories)
	}
	if len(ledger) != 3 {
		t.Fatalf("ledger rows = %d, want 3 approved scoped rows", len(ledger))
	}
}

func TestTaxReportSummarizesScopedInvoices(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	service := NewService(NewRepository(db), "test-secret")
	fixture := setupProjectScopeFixture(t, db, []string{"report.finance.view", "accounting.view"})
	payableID := newID()
	draftPayableID := newID()
	otherPayableID := newID()
	receivableID := newID()
	otherReceivableID := newID()
	mustExec(t, db, `
		INSERT INTO payables (id, project_id, payable_code, vendor_type, vendor_name, source_type, gross_amount, remaining_amount, status, created_by)
		VALUES
			($1,$2,$3,'supplier','Tax Vendor','manual',1100,1100,'approved',$4),
			($5,$2,$6,'supplier','Draft Vendor','manual',550,550,'approved',$4),
			($7,$8,$9,'supplier','Other Vendor','manual',999,999,'approved',$4)
	`, payableID, fixture.projectID, "PAY-TAX-"+payableID[:8], fixture.userID, draftPayableID, "PAY-TAX-"+draftPayableID[:8], otherPayableID, fixture.otherProjectID, "PAY-TAX-"+otherPayableID[:8])
	mustExec(t, db, `
		INSERT INTO receivables (id, project_id, receivable_code, client_name, source_type, gross_amount, received_amount, remaining_amount, status, created_by)
		VALUES
			($1,$2,$3,'Tax Client','manual',2210,0,2210,'approved',$4),
			($5,$6,$7,'Other Client','manual',999,0,999,'approved',$4)
	`, receivableID, fixture.projectID, "REC-TAX-"+receivableID[:8], fixture.userID, otherReceivableID, fixture.otherProjectID, "REC-TAX-"+otherReceivableID[:8])
	mustExec(t, db, `
		INSERT INTO input_invoices (
			id, project_id, payable_id, invoice_number, invoice_date, vendor_name, vendor_tax_code,
			amount_before_vat, vat_amount, total_amount, status,
			storage_provider, storage_key, mime_type, file_name, file_size, checksum_sha256,
			uploaded_by, verified_by, verified_at
		)
		VALUES
			($1,$2,$3,'IN-TAX-001','2026-06-03','Tax Vendor','0101111111',
				1000,100,1100,'verified',
				'local','input-invoices/tax-1.pdf','application/pdf','tax-1.pdf',128,$4,
				$5,$5,now()),
			($6,$2,$7,'IN-TAX-DRAFT','2026-06-05','Draft Vendor','0102222222',
				500,50,550,'draft',
				'local','input-invoices/tax-draft.pdf','application/pdf','tax-draft.pdf',128,$4,
				$5,NULL,NULL),
			($8,$9,$10,'IN-TAX-OTHER','2026-06-03','Other Vendor','0109999999',
				900,90,990,'verified',
				'local','input-invoices/tax-other.pdf','application/pdf','tax-other.pdf',128,$4,
				$5,$5,now())
	`, newID(), fixture.projectID, payableID, strings.Repeat("c", 64), fixture.userID, newID(), draftPayableID, newID(), fixture.otherProjectID, otherPayableID)
	mustExec(t, db, `
		INSERT INTO output_invoices (
			id, project_id, receivable_id, invoice_number, invoice_date, client_name, client_tax_code,
			amount_before_vat, vat_amount, total_amount, status,
			storage_provider, storage_key, mime_type, file_name, file_size, checksum_sha256,
			uploaded_by, issued_by, issued_at
		)
		VALUES
			($1,$2,$3,'OUT-TAX-001','2026-06-04','Tax Client','',
				2000,200,2210,'issued',
				'local','output-invoices/tax-1.pdf','application/pdf','tax-1.pdf',128,$4,
				$5,$5,now()),
			($6,$7,$8,'OUT-TAX-OTHER','2026-06-04','Other Client','0319999999',
				900,90,990,'issued',
				'local','output-invoices/tax-other.pdf','application/pdf','tax-other.pdf',128,$4,
				$5,$5,now())
	`, newID(), fixture.projectID, receivableID, strings.Repeat("d", 64), fixture.userID, newID(), fixture.otherProjectID, otherReceivableID)

	report, err := service.TaxReport(ctx, TaxReportRequest{
		ProjectID: fixture.projectID,
		FromDate:  "2026-06-01",
		ToDate:    "2026-06-30",
		GroupBy:   "month",
		Source:    "all",
		Status:    "posted",
	}, fixture.userID)
	if err != nil {
		t.Fatal(err)
	}
	summary, _ := report["summary"].(Record)
	periods, _ := report["periods"].([]Record)
	counterparties, _ := report["counterparties"].([]Record)
	ledger, _ := report["ledger"].([]Record)
	warnings, _ := report["warnings"].([]Record)
	if number(summary["input_vat_amount"]) != 100 || number(summary["output_vat_amount"]) != 200 || number(summary["net_vat_amount"]) != 100 {
		t.Fatalf("tax summary = input %v output %v net %v, want 100/200/100", summary["input_vat_amount"], summary["output_vat_amount"], summary["net_vat_amount"])
	}
	if number(summary["input_invoice_count"]) != 1 || number(summary["output_invoice_count"]) != 1 {
		t.Fatalf("invoice counts = input %v output %v, want 1/1", summary["input_invoice_count"], summary["output_invoice_count"])
	}
	if summary["net_vat_status"] != "payable" {
		t.Fatalf("net_vat_status = %v, want payable", summary["net_vat_status"])
	}
	if number(summary["excluded_unposted_count"]) != 1 {
		t.Fatalf("excluded_unposted_count = %v, want 1", summary["excluded_unposted_count"])
	}
	if number(summary["missing_tax_code_count"]) != 1 || number(summary["vat_math_mismatch_count"]) != 1 {
		t.Fatalf("warning counts = missing %v mismatch %v, want 1/1", summary["missing_tax_code_count"], summary["vat_math_mismatch_count"])
	}
	if len(warnings) != 3 || number(summary["warning_count"]) != 3 {
		t.Fatalf("warnings = %d summary=%v, want 3", len(warnings), summary["warning_count"])
	}
	if len(periods) != 1 || number(periods[0]["net_vat_amount"]) != 100 {
		t.Fatalf("periods = %#v, want one net 100 row", periods)
	}
	if len(counterparties) != 2 {
		t.Fatalf("counterparties = %d, want 2 scoped posted counterparties", len(counterparties))
	}
	if len(ledger) != 2 {
		t.Fatalf("ledger rows = %d, want 2 scoped posted invoices", len(ledger))
	}
}

func TestProfitReportSummarizesScopedInvoices(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	service := NewService(NewRepository(db), "test-secret")
	fixture := setupProjectScopeFixture(t, db, []string{"report.finance.view", "accounting.view"})
	directPayableID := newID()
	indirectPayableID := newID()
	draftPayableID := newID()
	otherPayableID := newID()
	receivableID := newID()
	draftReceivableID := newID()
	otherReceivableID := newID()
	mustExec(t, db, `
		INSERT INTO payables (id, project_id, payable_code, vendor_type, vendor_name, source_type, gross_amount, remaining_amount, status, created_by)
		VALUES
			($1,$2,$3,'supplier','Direct Vendor','procurement',330,330,'approved',$4),
			($5,$2,$6,'consultant','Indirect Vendor','manual',88,88,'approved',$4),
			($7,$2,$8,'supplier','Draft Vendor','procurement',55,55,'approved',$4),
			($9,$10,$11,'supplier','Other Vendor','procurement',999,999,'approved',$4)
	`, directPayableID, fixture.projectID, "PAY-PNL-"+directPayableID[:8], fixture.userID, indirectPayableID, "PAY-PNL-"+indirectPayableID[:8], draftPayableID, "PAY-PNL-"+draftPayableID[:8], otherPayableID, fixture.otherProjectID, "PAY-PNL-"+otherPayableID[:8])
	mustExec(t, db, `
		INSERT INTO receivables (id, project_id, receivable_code, client_name, source_type, gross_amount, received_amount, remaining_amount, status, created_by)
		VALUES
			($1,$2,$3,'P&L Client','manual',1100,0,1100,'approved',$4),
			($5,$2,$6,'Draft Client','manual',220,0,220,'approved',$4),
			($7,$8,$9,'Other Client','manual',999,0,999,'approved',$4)
	`, receivableID, fixture.projectID, "REC-PNL-"+receivableID[:8], fixture.userID, draftReceivableID, "REC-PNL-"+draftReceivableID[:8], otherReceivableID, fixture.otherProjectID, "REC-PNL-"+otherReceivableID[:8])
	mustExec(t, db, `
		INSERT INTO input_invoices (
			id, project_id, payable_id, invoice_number, invoice_date, vendor_name, vendor_tax_code,
			amount_before_vat, vat_amount, total_amount, status,
			storage_provider, storage_key, mime_type, file_name, file_size, checksum_sha256,
			uploaded_by, verified_by, verified_at
		)
		VALUES
			($1,$2,$3,'IN-PNL-DIRECT','2026-06-03','Direct Vendor','0101111111',
				300,30,330,'verified',
				'local','input-invoices/pnl-direct.pdf','application/pdf','pnl-direct.pdf',128,$4,
				$5,$5,now()),
			($6,$2,$7,'IN-PNL-INDIRECT','2026-06-04','Indirect Vendor','0102222222',
				80,8,88,'verified',
				'local','input-invoices/pnl-indirect.pdf','application/pdf','pnl-indirect.pdf',128,$4,
				$5,$5,now()),
			($8,$2,$9,'IN-PNL-DRAFT','2026-06-05','Draft Vendor','0103333333',
				50,5,55,'draft',
				'local','input-invoices/pnl-draft.pdf','application/pdf','pnl-draft.pdf',128,$4,
				$5,NULL,NULL),
			($10,$11,$12,'IN-PNL-OTHER','2026-06-04','Other Vendor','0109999999',
				900,90,990,'verified',
				'local','input-invoices/pnl-other.pdf','application/pdf','pnl-other.pdf',128,$4,
				$5,$5,now())
	`, newID(), fixture.projectID, directPayableID, strings.Repeat("e", 64), fixture.userID, newID(), indirectPayableID, newID(), draftPayableID, newID(), fixture.otherProjectID, otherPayableID)
	mustExec(t, db, `
		INSERT INTO output_invoices (
			id, project_id, receivable_id, invoice_number, invoice_date, client_name, client_tax_code,
			amount_before_vat, vat_amount, total_amount, status,
			storage_provider, storage_key, mime_type, file_name, file_size, checksum_sha256,
			uploaded_by, issued_by, issued_at
		)
		VALUES
			($1,$2,$3,'OUT-PNL-001','2026-06-02','P&L Client','0311111111',
				1000,100,1100,'issued',
				'local','output-invoices/pnl-1.pdf','application/pdf','pnl-1.pdf',128,$4,
				$5,$5,now()),
			($6,$2,$7,'OUT-PNL-DRAFT','2026-06-05','Draft Client','0312222222',
				200,20,220,'draft',
				'local','output-invoices/pnl-draft.pdf','application/pdf','pnl-draft.pdf',128,$4,
				$5,NULL,NULL),
			($8,$9,$10,'OUT-PNL-OTHER','2026-06-02','Other Client','0319999999',
				900,90,990,'issued',
				'local','output-invoices/pnl-other.pdf','application/pdf','pnl-other.pdf',128,$4,
				$5,$5,now())
	`, newID(), fixture.projectID, receivableID, strings.Repeat("f", 64), fixture.userID, newID(), draftReceivableID, newID(), fixture.otherProjectID, otherReceivableID)

	report, err := service.ProfitReport(ctx, ProfitReportRequest{
		ProjectID: fixture.projectID,
		FromDate:  "2026-06-01",
		ToDate:    "2026-06-30",
		GroupBy:   "month",
		Status:    "posted",
	}, fixture.userID)
	if err != nil {
		t.Fatal(err)
	}
	summary, _ := report["summary"].(Record)
	periods, _ := report["periods"].([]Record)
	projects, _ := report["projects"].([]Record)
	breakdown, _ := report["cost_breakdown"].([]Record)
	ledger, _ := report["ledger"].([]Record)
	warnings, _ := report["warnings"].([]Record)
	if number(summary["revenue_amount"]) != 1000 || number(summary["direct_cost_amount"]) != 300 || number(summary["indirect_cost_amount"]) != 80 {
		t.Fatalf("pnl summary = revenue %v direct %v indirect %v, want 1000/300/80", summary["revenue_amount"], summary["direct_cost_amount"], summary["indirect_cost_amount"])
	}
	if number(summary["gross_profit"]) != 700 || number(summary["net_profit"]) != 620 {
		t.Fatalf("profit = gross %v net %v, want 700/620", summary["gross_profit"], summary["net_profit"])
	}
	if number(summary["gross_margin_percent"]) != 70 || number(summary["net_margin_percent"]) != 62 {
		t.Fatalf("margins = gross %v net %v, want 70/62", summary["gross_margin_percent"], summary["net_margin_percent"])
	}
	if number(summary["excluded_unposted_count"]) != 2 || number(summary["excluded_unposted_total"]) != 250 {
		t.Fatalf("excluded = count %v total %v, want 2/250", summary["excluded_unposted_count"], summary["excluded_unposted_total"])
	}
	if number(summary["unclassified_cost_count"]) != 1 {
		t.Fatalf("unclassified_cost_count = %v, want 1", summary["unclassified_cost_count"])
	}
	if len(warnings) != 2 || number(summary["warning_count"]) != 2 {
		t.Fatalf("warnings = %d summary=%v, want 2", len(warnings), summary["warning_count"])
	}
	if len(periods) != 1 || number(periods[0]["net_profit"]) != 620 {
		t.Fatalf("periods = %#v, want one net 620 row", periods)
	}
	if len(projects) != 1 || number(projects[0]["net_profit"]) != 620 {
		t.Fatalf("projects = %#v, want one scoped project net 620", projects)
	}
	if len(breakdown) != 2 {
		t.Fatalf("cost breakdown rows = %d, want 2 direct/indirect groups", len(breakdown))
	}
	if len(ledger) != 3 {
		t.Fatalf("ledger rows = %d, want 3 scoped posted P&L rows", len(ledger))
	}
}

func TestInputInvoicePDFUploadLinksPayableDocumentAndStatus(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	service := NewService(NewRepository(db), "test-secret")
	uploadDir := t.TempDir()
	service.ConfigureUploads(uploadDir, 1024*1024)
	fixture := setupProjectScopeFixture(t, db, []string{"accounting.view", "accounting.payable_view", "accounting.payable_approve"})
	payableID := newID()
	otherProcurementID := newID()
	mustExec(t, db, `
		INSERT INTO payables (id, project_id, payable_code, vendor_type, vendor_name, source_type, gross_amount, remaining_amount, status, created_by)
		VALUES ($1,$2,$3,'supplier','VAT Vendor','manual',1100000,1100000,'approved',$4)
	`, payableID, fixture.projectID, "PAY-IN-"+payableID[:8], fixture.userID)
	mustExec(t, db, `
		INSERT INTO procurement_requests (id, project_id, request_code, title, requested_by, status, total_estimated_amount)
		VALUES ($1,$2,$3,'Other procurement',$4,'approved',1100000)
	`, otherProcurementID, fixture.otherProjectID, "PR-OTHER-"+otherProcurementID[:8], fixture.userID)

	pdfContent := []byte("%PDF-1.4\n% test invoice\n1 0 obj\n<<>>\nendobj\ntrailer\n<<>>\n%%EOF\n")
	invoice, err := service.CreateInputInvoice(ctx, payableID, InputInvoiceRequest{
		InvoiceNumber:   "VAT-001",
		InvoiceSymbol:   "C25TAA",
		InvoiceDate:     "2026-06-22",
		VendorName:      "VAT Vendor",
		VendorTaxCode:   "0312345678",
		AmountBeforeVAT: 1000000,
		VATAmount:       100000,
		TotalAmount:     1100000,
		Notes:           "Hoá đơn mua vật tư",
	}, testMultipartFileHeader(t, "file", "vat-invoice.pdf", pdfContent), fixture.userID, "Scoped User")
	if err != nil {
		t.Fatal(err)
	}
	if invoice["status"] != "draft" {
		t.Fatalf("status = %v, want draft", invoice["status"])
	}
	invoiceID := cleanID(invoice["id"])
	documentID := cleanID(invoice["document_id"])
	if invoiceID == "" || documentID == "" {
		t.Fatalf("expected invoice and document ids, got invoice=%q document=%q", invoiceID, documentID)
	}
	storageKey := fmt.Sprint(invoice["storage_key"])
	if !strings.HasPrefix(storageKey, fmt.Sprintf("input-invoices/%s/%s/", fixture.projectID, payableID)) || strings.Contains(storageKey, "..") {
		t.Fatalf("invalid storage_key = %q", storageKey)
	}
	sum := sha256.Sum256(pdfContent)
	if got := fmt.Sprint(invoice["checksum_sha256"]); got != hex.EncodeToString(sum[:]) {
		t.Fatalf("checksum = %s, want %s", got, hex.EncodeToString(sum[:]))
	}
	assertCount(t, db, `SELECT COUNT(*) FROM documents WHERE id=$1 AND project_id=$2 AND target_type='input_invoice' AND target_id=$3 AND status='active'`, 1, documentID, fixture.projectID, invoiceID)
	assertCount(t, db, `SELECT COUNT(*) FROM audit_logs WHERE project_id=$1 AND event_type='input_invoice_uploaded' AND target_id=$2`, 1, fixture.projectID, invoiceID)

	file, err := service.InputInvoiceFile(ctx, invoiceID, fixture.userID)
	if err != nil {
		t.Fatal(err)
	}
	if file.MIMEType != "application/pdf" || file.Filename != "vat-invoice.pdf" {
		t.Fatalf("file result = %#v", file)
	}
	storedBytes, err := os.ReadFile(file.Path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(storedBytes, pdfContent) {
		t.Fatal("stored invoice PDF bytes do not match upload")
	}
	if !strings.HasPrefix(file.Path, uploadDir) {
		t.Fatalf("file path = %q, want under %q", file.Path, uploadDir)
	}

	payableInvoices, err := service.PayableInputInvoices(ctx, payableID, fixture.userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(payableInvoices) != 1 || cleanID(payableInvoices[0]["id"]) != invoiceID {
		t.Fatalf("payable invoices = %#v", payableInvoices)
	}
	projectInvoices, err := service.ProjectInputInvoices(ctx, fixture.projectID, fixture.userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(projectInvoices) != 1 || cleanID(projectInvoices[0]["id"]) != invoiceID {
		t.Fatalf("project invoices = %#v", projectInvoices)
	}

	verified, err := service.InputInvoiceAction(ctx, invoiceID, "verify", ActionRequest{}, fixture.userID, "Scoped User")
	if err != nil {
		t.Fatal(err)
	}
	if verified["status"] != "verified" || cleanID(verified["verified_by"]) != fixture.userID {
		t.Fatalf("verified invoice = %#v", verified)
	}
	rejected, err := service.InputInvoiceAction(ctx, invoiceID, "reject", ActionRequest{}, fixture.userID, "Scoped User")
	if err != nil {
		t.Fatal(err)
	}
	if rejected["status"] != "rejected" || cleanID(rejected["rejected_by"]) != fixture.userID || cleanID(rejected["verified_by"]) != "" {
		t.Fatalf("rejected invoice = %#v", rejected)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM audit_logs WHERE project_id=$1 AND target_id=$2 AND event_type IN ('input_invoice_verified','input_invoice_rejected')`, 2, fixture.projectID, invoiceID)

	_, err = service.CreateInputInvoice(ctx, payableID, InputInvoiceRequest{
		InvoiceNumber:        "VAT-OTHER-PROC",
		InvoiceDate:          "2026-06-22",
		VendorName:           "VAT Vendor",
		AmountBeforeVAT:      1000000,
		VATAmount:            100000,
		TotalAmount:          1100000,
		ProcurementRequestID: otherProcurementID,
	}, testMultipartFileHeader(t, "file", "other-proc.pdf", pdfContent), fixture.userID, "Scoped User")
	assertValidationError(t, err)
}

func TestOutputInvoicePDFUploadLinksReceivablePaymentDocumentAndStatus(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	service := NewService(NewRepository(db), "test-secret")
	uploadDir := t.TempDir()
	service.ConfigureUploads(uploadDir, 1024*1024)
	fixture := setupProjectScopeFixture(t, db, []string{"accounting.view", "accounting.receivable_view", "accounting.receivable_approve"})
	receivableID := newID()
	paymentID := newID()
	otherPaymentID := newID()
	mustExec(t, db, `
		INSERT INTO receivables (id, project_id, receivable_code, client_name, source_type, contract_amount, milestone_name, gross_amount, received_amount, remaining_amount, status, created_by)
		VALUES ($1,$2,$3,'VAT Client','manual',2200000,'Đợt 1',2200000,0,2200000,'approved',$4)
	`, receivableID, fixture.projectID, "REC-OUT-"+receivableID[:8], fixture.userID)
	mustExec(t, db, `
		INSERT INTO payments (id, project_id, payment_code, payment_type, target_type, target_id, amount, payment_method, status, created_by)
		VALUES ($1,$2,$3,'client_receipt','receivable',$4,2200000,'bank_transfer','received',$5)
	`, paymentID, fixture.projectID, "RCV-OUT-"+paymentID[:8], receivableID, fixture.userID)
	mustExec(t, db, `
		INSERT INTO payments (id, project_id, payment_code, payment_type, target_type, target_id, amount, payment_method, status, created_by)
		VALUES ($1,$2,$3,'client_receipt','receivable',$4,2200000,'bank_transfer','received',$5)
	`, otherPaymentID, fixture.otherProjectID, "RCV-OTH-"+otherPaymentID[:8], receivableID, fixture.userID)

	pdfContent := []byte("%PDF-1.4\n% test output invoice\n1 0 obj\n<<>>\nendobj\ntrailer\n<<>>\n%%EOF\n")
	invoice, err := service.CreateOutputInvoice(ctx, receivableID, OutputInvoiceRequest{
		InvoiceNumber:   "OUT-001",
		InvoiceSymbol:   "C25TAA",
		InvoiceDate:     "2026-06-22",
		ClientName:      "VAT Client",
		ClientTaxCode:   "0311111111",
		AmountBeforeVAT: 2000000,
		VATAmount:       200000,
		TotalAmount:     2200000,
		PaymentID:       paymentID,
		Notes:           "Hoá đơn đầu ra đợt 1",
	}, testMultipartFileHeader(t, "file", "output-invoice.pdf", pdfContent), fixture.userID, "Scoped User")
	if err != nil {
		t.Fatal(err)
	}
	if invoice["status"] != "draft" {
		t.Fatalf("status = %v, want draft", invoice["status"])
	}
	invoiceID := cleanID(invoice["id"])
	documentID := cleanID(invoice["document_id"])
	if invoiceID == "" || documentID == "" {
		t.Fatalf("expected invoice and document ids, got invoice=%q document=%q", invoiceID, documentID)
	}
	if cleanID(invoice["payment_id"]) != paymentID {
		t.Fatalf("payment_id = %v, want %s", invoice["payment_id"], paymentID)
	}
	storageKey := fmt.Sprint(invoice["storage_key"])
	if !strings.HasPrefix(storageKey, fmt.Sprintf("output-invoices/%s/%s/", fixture.projectID, receivableID)) || strings.Contains(storageKey, "..") {
		t.Fatalf("invalid storage_key = %q", storageKey)
	}
	sum := sha256.Sum256(pdfContent)
	if got := fmt.Sprint(invoice["checksum_sha256"]); got != hex.EncodeToString(sum[:]) {
		t.Fatalf("checksum = %s, want %s", got, hex.EncodeToString(sum[:]))
	}
	assertCount(t, db, `SELECT COUNT(*) FROM documents WHERE id=$1 AND project_id=$2 AND target_type='output_invoice' AND target_id=$3 AND status='active'`, 1, documentID, fixture.projectID, invoiceID)
	assertCount(t, db, `SELECT COUNT(*) FROM audit_logs WHERE project_id=$1 AND event_type='output_invoice_uploaded' AND target_id=$2`, 1, fixture.projectID, invoiceID)

	file, err := service.OutputInvoiceFile(ctx, invoiceID, fixture.userID)
	if err != nil {
		t.Fatal(err)
	}
	if file.MIMEType != "application/pdf" || file.Filename != "output-invoice.pdf" {
		t.Fatalf("file result = %#v", file)
	}
	storedBytes, err := os.ReadFile(file.Path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(storedBytes, pdfContent) {
		t.Fatal("stored output invoice PDF bytes do not match upload")
	}
	if !strings.HasPrefix(file.Path, uploadDir) {
		t.Fatalf("file path = %q, want under %q", file.Path, uploadDir)
	}

	receivableInvoices, err := service.ReceivableOutputInvoices(ctx, receivableID, fixture.userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(receivableInvoices) != 1 || cleanID(receivableInvoices[0]["id"]) != invoiceID {
		t.Fatalf("receivable invoices = %#v", receivableInvoices)
	}
	projectInvoices, err := service.ProjectOutputInvoices(ctx, fixture.projectID, fixture.userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(projectInvoices) != 1 || cleanID(projectInvoices[0]["id"]) != invoiceID {
		t.Fatalf("project invoices = %#v", projectInvoices)
	}

	issued, err := service.OutputInvoiceAction(ctx, invoiceID, "issue", ActionRequest{}, fixture.userID, "Scoped User")
	if err != nil {
		t.Fatal(err)
	}
	if issued["status"] != "issued" || cleanID(issued["issued_by"]) != fixture.userID {
		t.Fatalf("issued invoice = %#v", issued)
	}
	voided, err := service.OutputInvoiceAction(ctx, invoiceID, "void", ActionRequest{}, fixture.userID, "Scoped User")
	if err != nil {
		t.Fatal(err)
	}
	if voided["status"] != "voided" || cleanID(voided["voided_by"]) != fixture.userID || cleanID(voided["issued_by"]) != fixture.userID {
		t.Fatalf("voided invoice = %#v", voided)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM audit_logs WHERE project_id=$1 AND target_id=$2 AND event_type IN ('output_invoice_issued','output_invoice_voided')`, 2, fixture.projectID, invoiceID)

	_, err = service.CreateOutputInvoice(ctx, receivableID, OutputInvoiceRequest{
		InvoiceNumber:   "OUT-OTHER-PAYMENT",
		InvoiceDate:     "2026-06-22",
		ClientName:      "VAT Client",
		AmountBeforeVAT: 2000000,
		VATAmount:       200000,
		TotalAmount:     2200000,
		PaymentID:       otherPaymentID,
	}, testMultipartFileHeader(t, "file", "other-payment.pdf", pdfContent), fixture.userID, "Scoped User")
	assertValidationError(t, err)
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

func TestBusinessCoreOperationsServiceRecordsActorAndScope(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")
	fixture := setupProjectScopeFixture(t, db, []string{"operation.view", "operation.report", "operation.issue_create", "operation.issue_update"})

	report, err := service.CreateDailyReport(ctx, fixture.projectID, Record{
		"report_date":            "2026-06-23",
		"weather":                "Nắng",
		"work_summary":           "Đổ bê tông sàn",
		"manpower_summary":       "8 công nhân",
		"material_usage_summary": "Bê tông M300",
		"issues_summary":         "Không",
		"progress_percent":       42,
	}, fixture.userID, "Scoped User")
	if err != nil {
		t.Fatal(err)
	}
	if cleanID(report["project_id"]) != fixture.projectID || cleanID(report["reported_by"]) != fixture.userID {
		t.Fatalf("daily report project/reporter = %v/%v, want %s/%s", report["project_id"], report["reported_by"], fixture.projectID, fixture.userID)
	}
	reports, err := service.ProjectDailyReports(ctx, fixture.projectID)
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 1 || cleanID(reports[0]["id"]) != cleanID(report["id"]) {
		t.Fatalf("project reports = %#v, want created report", reports)
	}

	issue, err := service.CreateIssue(ctx, fixture.projectID, Record{
		"title":               "Sai cao độ",
		"description":         "Kiểm tra lại cote hoàn thiện",
		"severity":            "medium",
		"requires_rework":     true,
		"requires_replanning": false,
	}, fixture.userID, "Scoped User")
	if err != nil {
		t.Fatal(err)
	}
	if cleanID(issue["project_id"]) != fixture.projectID || cleanID(issue["reported_by"]) != fixture.userID {
		t.Fatalf("issue project/reporter = %v/%v, want %s/%s", issue["project_id"], issue["reported_by"], fixture.projectID, fixture.userID)
	}
	updated, err := service.UpdateIssue(ctx, cleanID(issue["id"]), Record{"project_id": fixture.otherProjectID, "status": "closed", "severity": "high"}, fixture.userID, "Scoped User")
	if err != nil {
		t.Fatal(err)
	}
	if cleanID(updated["project_id"]) != fixture.projectID {
		t.Fatalf("issue project_id moved to %v, want fixed %s", updated["project_id"], fixture.projectID)
	}
	if updated["status"] != "closed" || updated["severity"] != "high" {
		t.Fatalf("updated issue status/severity = %v/%v, want closed/high", updated["status"], updated["severity"])
	}
	issues, err := service.ProjectIssues(ctx, fixture.projectID)
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 1 || cleanID(issues[0]["id"]) != cleanID(issue["id"]) {
		t.Fatalf("project issues = %#v, want created issue", issues)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM audit_logs WHERE project_id=$1 AND event_type IN ('daily_report_created','issue_created','issue_updated')`, 3, fixture.projectID)
}

func TestBusinessCoreLifecycleServiceRecordsActors(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")
	fixture := setupProjectScopeFixture(t, db, []string{
		"rework.view",
		"rework.create",
		"rework.approve",
		"replanning.view",
		"replanning.create",
		"replanning.approve",
		"handover.view",
		"handover.create",
		"accounting.budget_view",
		"accounting.budget_approve",
		"accounting.payment_create",
	})
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM audit_logs WHERE actor_id=$1 AND project_id IS NULL`, fixture.userID)
	})

	suffix := fixture.projectID[:8]
	warehouse, err := service.CreateWarehouse(ctx, Record{"name": "Core WH " + suffix, "location": "Site", "type": "site"}, fixture.userID, "Scoped User")
	if err != nil {
		t.Fatal(err)
	}
	warehouseID := cleanID(warehouse["id"])
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM warehouses WHERE id=$1`, warehouseID)
	})
	if warehouse["active"] != true {
		t.Fatalf("warehouse active = %v, want DB default true", warehouse["active"])
	}

	issue, err := service.CreateIssue(ctx, fixture.projectID, Record{"title": "Cần xử lý lại", "severity": "high"}, fixture.userID, "Scoped User")
	if err != nil {
		t.Fatal(err)
	}
	rework, err := service.CreateRework(ctx, fixture.projectID, Record{"issue_id": cleanID(issue["id"]), "title": "Làm lại lớp chống thấm"}, fixture.userID, "Scoped User")
	if err != nil {
		t.Fatal(err)
	}
	if cleanID(rework["created_by"]) != fixture.userID {
		t.Fatalf("rework created_by = %v, want %s", rework["created_by"], fixture.userID)
	}
	rework, err = service.UpdateReworkStatus(ctx, cleanID(rework["id"]), "approved", fixture.userID, "Scoped User")
	if err != nil {
		t.Fatal(err)
	}
	if rework["status"] != "approved" || cleanID(rework["approved_by"]) != fixture.userID {
		t.Fatalf("rework approval = status %v approved_by %v", rework["status"], rework["approved_by"])
	}
	rework, err = service.UpdateReworkStatus(ctx, cleanID(rework["id"]), "done", fixture.userID, "Scoped User")
	if err != nil {
		t.Fatal(err)
	}
	if rework["status"] != "done" || cleanID(rework["approved_by"]) != fixture.userID {
		t.Fatalf("rework completion = status %v approved_by %v", rework["status"], rework["approved_by"])
	}

	replanning, err := service.CreateReplanning(ctx, fixture.projectID, Record{
		"issue_id":        cleanID(issue["id"]),
		"reason":          "Đổi biện pháp thi công",
		"impact_scope":    "Tiến độ tầng 2",
		"budget_impact":   1000,
		"timeline_impact": 2,
	}, fixture.userID, "Scoped User")
	if err != nil {
		t.Fatal(err)
	}
	if cleanID(replanning["created_by"]) != fixture.userID {
		t.Fatalf("replanning created_by = %v, want %s", replanning["created_by"], fixture.userID)
	}
	replanning, err = service.ApproveReplanning(ctx, cleanID(replanning["id"]), fixture.userID, "Scoped User")
	if err != nil {
		t.Fatal(err)
	}
	if replanning["status"] != "approved" || cleanID(replanning["approved_by"]) != fixture.userID {
		t.Fatalf("replanning approval = status %v approved_by %v", replanning["status"], replanning["approved_by"])
	}

	handover, err := service.CreateHandover(ctx, fixture.projectID, Record{"client_representative": "Client", "company_representative": "PM"}, fixture.userID, "Scoped User")
	if err != nil {
		t.Fatal(err)
	}
	if cleanID(handover["created_by"]) != fixture.userID {
		t.Fatalf("handover created_by = %v, want %s", handover["created_by"], fixture.userID)
	}

	budget, err := service.CreateBudget(ctx, fixture.projectID, Record{"budget_code": "BUD-CORE-" + suffix, "name": "Core budget", "planned_amount": 1200}, fixture.userID, "Scoped User")
	if err != nil {
		t.Fatal(err)
	}
	budget, err = service.ApproveBudget(ctx, cleanID(budget["id"]), fixture.userID, "Scoped User")
	if err != nil {
		t.Fatal(err)
	}
	if budget["status"] != "approved" || cleanID(budget["approved_by"]) != fixture.userID {
		t.Fatalf("budget approval = status %v approved_by %v", budget["status"], budget["approved_by"])
	}

	payment, err := service.CreatePayment(ctx, Record{
		"project_id":     fixture.projectID,
		"payment_code":   "PMT-CORE-" + suffix,
		"payment_type":   "outbound",
		"target_type":    "issue",
		"target_id":      cleanID(issue["id"]),
		"amount":         450,
		"payment_method": "bank_transfer",
	}, fixture.userID, "Scoped User")
	if err != nil {
		t.Fatal(err)
	}
	if cleanID(payment["created_by"]) != fixture.userID {
		t.Fatalf("payment created_by = %v, want %s", payment["created_by"], fixture.userID)
	}

	assertCount(t, db, `
		SELECT COUNT(*)
		FROM audit_logs
		WHERE project_id=$1
			AND event_type IN (
				'issue_created',
				'rework_created',
				'rework_approved',
				'rework_completed',
				'replanning_created',
				'replanning_approved',
				'handover_created',
				'budget_created',
				'budget_approved',
				'payment_created'
			)
	`, 10, fixture.projectID)
	assertCount(t, db, `SELECT COUNT(*) FROM audit_logs WHERE target_id=$1 AND event_type='warehouse_created'`, 1, warehouseID)
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
	actorStaffID := newID()
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
		_, _ = db.ExecContext(ctx, `DELETE FROM staff WHERE id=$1`, actorStaffID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id=$1`, actorID)
	})

	mustExec(t, db, `INSERT INTO users (id, email, password_hash, full_name, status) VALUES ($1,$2,'demo:pw','Test Actor','active')`, actorID, "ops-start-"+actorID+"@test.local")
	mustExec(t, db, `INSERT INTO staff (id, user_id, full_name, email, department, status) VALUES ($1,$2,'Test Actor',$3,'operations','active')`, actorStaffID, actorID, "ops-start-staff-"+actorID+"@test.local")
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
	mustExec(t, db, `INSERT INTO project_members (project_id, staff_id, role_ids, active) VALUES ($1,$2,'["PM"]'::jsonb,true)`, projectID, actorStaffID)

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
		if cleanID(step["workflow_form_version_id"]) != formVersionID || fmt.Sprint(step["workflow_form_template_code"]) != "builtin_checklist" || number(step["workflow_form_version_no"]) == 0 {
			t.Fatalf("expected typed workflow form summary, got %#v", step)
		}
	}
	firstStep := steps[0]
	workItemID := cleanID(firstStep["work_item_id"])
	detail, err := repo.GetWorkItemDetail(ctx, workItemID)
	if err != nil {
		t.Fatal(err)
	}
	if cleanID(detail["workflow_form_instance_id"]) == "" || fmt.Sprint(detail["workflow_form_template_name"]) == "" || !boolValue(detail["workflow_form_required"]) {
		t.Fatalf("expected workflow form summary on work item detail, got %#v", detail)
	}
	projectItems, err := repo.ListProjectWorkItems(ctx, projectID, actorID)
	if err != nil {
		t.Fatal(err)
	}
	foundListSummary := false
	for _, item := range projectItems {
		if cleanID(item["id"]) == workItemID {
			foundListSummary = cleanID(item["workflow_form_instance_id"]) != "" &&
				fmt.Sprint(item["workflow_form_template_code"]) == "builtin_checklist" &&
				boolValue(item["workflow_form_required"])
		}
	}
	if !foundListSummary {
		t.Fatalf("expected workflow form summary in project work item list, got %#v", projectItems)
	}
}

func TestStartPipelineRejectsRequiredStepWithoutPublishedWorkflowForm(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)

	actorID := newID()
	templateID := newID()
	nodeID := newID()
	projectID := newID()
	stepID := newID()
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM projects WHERE id=$1`, projectID)
		_, _ = db.ExecContext(ctx, `DELETE FROM pipeline_templates WHERE id=$1`, templateID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id=$1`, actorID)
	})

	mustExec(t, db, `INSERT INTO users (id, email, password_hash, full_name, status) VALUES ($1,$2,'demo:pw','Test Actor','active')`, actorID, "ops-start-invalid-"+actorID+"@test.local")
	mustExec(t, db, `INSERT INTO pipeline_templates (id, name, status, created_by) VALUES ($1,'Invalid Ops Template','published',$2)`, templateID, actorID)
	mustExec(t, db, `
		INSERT INTO pipeline_template_nodes (id, template_id, name, type, order_index, responsible_role_ids)
		VALUES ($1,$2,'Survey','survey',1,'["PM"]'::jsonb)
	`, nodeID, templateID)
	mustExec(t, db, `
		INSERT INTO pipeline_template_steps (id, node_id, name, step_type, required_role_ids, form_required, order_index)
		VALUES ($1,$2,'Required business form','checklist','["PM"]'::jsonb,true,1)
	`, stepID, nodeID)
	mustExec(t, db, `
		INSERT INTO projects (id, name, code, client_name, status, pipeline_template_id)
		VALUES ($1,'Invalid Ops Project',$2,'Client','draft',$3)
	`, projectID, "OPSI-"+projectID[:8], templateID)

	_, err := repo.StartPipeline(ctx, projectID, templateID, actorID, "actor@test.local")
	assertValidationError(t, err)
	assertCount(t, db, `SELECT COUNT(*) FROM pipeline_runs WHERE project_id=$1`, 0, projectID)
	assertCount(t, db, `SELECT COUNT(*) FROM work_items WHERE project_id=$1`, 0, projectID)
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

func TestInputPaymentScheduleAdvanceAndPartialPaymentReconcilePayable(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")
	fixture := setupProjectScopeFixture(t, db, []string{
		"accounting.view",
		"accounting.payable_view",
		"accounting.payable_approve",
		"accounting.payment_create",
		"accounting.payment_approve",
	})
	payableID := newID()
	mustExec(t, db, `
		INSERT INTO payables (id, project_id, payable_code, vendor_type, vendor_name, source_type, gross_amount, paid_amount, remaining_amount, status, created_by)
		VALUES ($1,$2,$3,'supplier','Input Payment Vendor','manual',1000,0,1000,'approved',$4)
	`, payableID, fixture.projectID, "PAY-IP-"+payableID[:8], fixture.userID)

	schedule, err := service.CreatePayablePaymentSchedule(ctx, payableID, PayablePaymentScheduleRequest{
		ScheduleCode:  "SCH-IP-001",
		MilestoneName: "Kỳ vật tư 1",
		ScheduleType:  "progress",
		PlannedAmount: 600,
		DueDate:       "2026-07-15",
	}, fixture.userID, "Scoped User")
	if err != nil {
		t.Fatal(err)
	}
	scheduleID := cleanID(schedule["id"])
	schedule, err = service.PayablePaymentScheduleAction(ctx, scheduleID, "approve", ActionRequest{Comment: "Duyệt kỳ"}, fixture.userID, "Scoped User")
	if err != nil {
		t.Fatal(err)
	}
	if schedule["status"] != "approved" || number(schedule["remaining_amount"]) != 600 {
		t.Fatalf("schedule = status %v remaining %v, want approved/600", schedule["status"], schedule["remaining_amount"])
	}

	advance, err := service.CreatePayableAdvanceRequest(ctx, payableID, AdvanceRequestCreateRequest{
		RequestCode: "ADV-IP-001",
		Amount:      200,
		DueDate:     "2026-07-01",
		Reason:      "Ứng trước cho vendor",
	}, fixture.userID, "Scoped User")
	if err != nil {
		t.Fatal(err)
	}
	advanceID := cleanID(advance["id"])
	advance, err = service.PayableAdvanceRequestAction(ctx, advanceID, "approve", ActionRequest{Comment: "Duyệt ứng"}, fixture.userID, "Scoped User")
	if err != nil {
		t.Fatal(err)
	}
	if advance["status"] != "approved" {
		t.Fatalf("advance status = %v, want approved", advance["status"])
	}
	payable, err := repo.GetPayableDetail(ctx, payableID)
	if err != nil {
		t.Fatal(err)
	}
	if payable["status"] != "partially_paid" || number(payable["advance_amount"]) != 200 || number(payable["remaining_amount"]) != 800 {
		t.Fatalf("payable after advance = status %v advance %v remaining %v, want partially_paid/200/800", payable["status"], payable["advance_amount"], payable["remaining_amount"])
	}

	advance, err = service.SettlePayableAdvanceRequest(ctx, advanceID, AdvanceSettlementRequest{Amount: 50, ScheduleID: scheduleID, Comment: "Đối soát một phần"}, fixture.userID, "Scoped User")
	if err != nil {
		t.Fatal(err)
	}
	if advance["status"] != "partially_settled" || number(advance["settled_amount"]) != 50 {
		t.Fatalf("advance settlement = status %v settled %v, want partially_settled/50", advance["status"], advance["settled_amount"])
	}

	payment, err := service.CreatePayablePayment(ctx, payableID, PayablePaymentCreateRequest{
		PaymentCode:   "PMT-IP-001",
		PaymentType:   "supplier_payment",
		PaymentMethod: "bank_transfer",
		Amount:        300,
		ScheduleID:    scheduleID,
		PaidAt:        "2026-07-20",
		Notes:         "Thanh toán một phần kỳ 1",
	}, fixture.userID, "Scoped User")
	if err != nil {
		t.Fatal(err)
	}
	paymentID := cleanID(payment["id"])
	if payment["status"] != "draft" || payment["allocation_type"] != "payable_payment" {
		t.Fatalf("payment = %#v, want draft payable allocation", payment)
	}
	payment, err = service.PaymentAction(ctx, paymentID, "approve", ActionRequest{Comment: "Duyệt thanh toán"}, fixture.userID, "Scoped User")
	if err != nil {
		t.Fatal(err)
	}
	if payment["status"] != "approved" {
		t.Fatalf("payment status = %v, want approved", payment["status"])
	}
	payable, err = repo.GetPayableDetail(ctx, payableID)
	if err != nil {
		t.Fatal(err)
	}
	if payable["status"] != "partially_paid" || number(payable["paid_amount"]) != 300 || number(payable["remaining_amount"]) != 500 {
		t.Fatalf("payable after payment = status %v paid %v remaining %v, want partially_paid/300/500", payable["status"], payable["paid_amount"], payable["remaining_amount"])
	}
	schedule, err = repo.GetPayablePaymentScheduleDetail(ctx, scheduleID)
	if err != nil {
		t.Fatal(err)
	}
	if schedule["status"] != "partially_paid" || number(schedule["paid_amount"]) != 300 || number(schedule["remaining_amount"]) != 300 {
		t.Fatalf("schedule after payment = status %v paid %v remaining %v, want partially_paid/300/300", schedule["status"], schedule["paid_amount"], schedule["remaining_amount"])
	}

	advancePayment, err := service.CreatePayablePayment(ctx, payableID, PayablePaymentCreateRequest{
		PaymentCode:      "PMT-IP-ADV",
		PaymentType:      "advance",
		PaymentMethod:    "cash",
		Amount:           200,
		AdvanceRequestID: advanceID,
		PaidAt:           "2026-07-02",
	}, fixture.userID, "Scoped User")
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.PaymentAction(ctx, cleanID(advancePayment["id"]), "approve", ActionRequest{Comment: "Chi ứng trước"}, fixture.userID, "Scoped User")
	if err != nil {
		t.Fatal(err)
	}
	advance, err = repo.GetPayableAdvanceRequestDetail(ctx, advanceID)
	if err != nil {
		t.Fatal(err)
	}
	if number(advance["paid_amount"]) != 200 {
		t.Fatalf("advance paid_amount = %v, want 200", advance["paid_amount"])
	}
	payable, err = repo.GetPayableDetail(ctx, payableID)
	if err != nil {
		t.Fatal(err)
	}
	if number(payable["remaining_amount"]) != 500 {
		t.Fatalf("advance payment should not reduce payable twice, remaining = %v, want 500", payable["remaining_amount"])
	}

	_, err = service.CreatePayablePayment(ctx, payableID, PayablePaymentCreateRequest{
		PaymentCode:   "PMT-IP-OVER",
		PaymentMethod: "bank_transfer",
		Amount:        1000,
	}, fixture.userID, "Scoped User")
	assertValidationError(t, err)
	assertCount(t, db, `
		SELECT COUNT(*)
		FROM audit_logs
		WHERE project_id=$1
		  AND event_type IN (
		    'payable_payment_schedule_created',
		    'payable_payment_schedule_approved',
		    'payable_advance_requested',
		    'payable_advance_approved',
		    'payable_advance_settled',
		    'payable_payment_created',
		    'payable_payment_allocated',
		    'payable_advance_payment_recorded'
		  )
	`, 9, fixture.projectID)
}

func TestPayableReconciliationSnapshotIncludesAgingWarningsAndLedger(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")
	fixture := setupProjectScopeFixture(t, db, []string{
		"accounting.view",
		"accounting.payable_view",
		"accounting.payable_approve",
		"accounting.payment_create",
		"accounting.payment_approve",
	})
	payableID := newID()
	invoiceID := newID()
	mustExec(t, db, `
		INSERT INTO payables (id, project_id, payable_code, vendor_type, vendor_name, source_type, gross_amount, paid_amount, remaining_amount, due_date, status, created_by)
		VALUES ($1,$2,$3,'supplier','Ledger Vendor','manual',1000,0,1000,'2000-01-01','approved',$4)
	`, payableID, fixture.projectID, "PAY-LED-"+payableID[:8], fixture.userID)
	mustExec(t, db, `
		INSERT INTO input_invoices (
			id, project_id, payable_id, invoice_number, invoice_date, vendor_name,
			amount_before_vat, vat_amount, total_amount, status,
			storage_provider, storage_key, mime_type, file_name, file_size, checksum_sha256,
			uploaded_by, verified_by, verified_at
		)
		VALUES ($1,$2,$3,'INV-LED-001','2026-06-01','Ledger Vendor',
			300,0,300,'verified',
			'local','input-invoices/test.pdf','application/pdf','test.pdf',128,$4,
			$5,$5,now())
	`, invoiceID, fixture.projectID, payableID, strings.Repeat("a", 64), fixture.userID)

	schedule, err := service.CreatePayablePaymentSchedule(ctx, payableID, PayablePaymentScheduleRequest{
		ScheduleCode:  "SCH-LED-001",
		MilestoneName: "Kỳ ledger",
		ScheduleType:  "progress",
		PlannedAmount: 500,
		DueDate:       "2000-02-01",
	}, fixture.userID, "Scoped User")
	if err != nil {
		t.Fatal(err)
	}
	scheduleID := cleanID(schedule["id"])
	if _, err := service.PayablePaymentScheduleAction(ctx, scheduleID, "approve", ActionRequest{}, fixture.userID, "Scoped User"); err != nil {
		t.Fatal(err)
	}

	advance, err := service.CreatePayableAdvanceRequest(ctx, payableID, AdvanceRequestCreateRequest{
		RequestCode: "ADV-LED-001",
		Amount:      200,
		DueDate:     "2000-01-15",
		Reason:      "Ứng trước ledger",
	}, fixture.userID, "Scoped User")
	if err != nil {
		t.Fatal(err)
	}
	advanceID := cleanID(advance["id"])
	if _, err := service.PayableAdvanceRequestAction(ctx, advanceID, "approve", ActionRequest{}, fixture.userID, "Scoped User"); err != nil {
		t.Fatal(err)
	}

	payment, err := service.CreatePayablePayment(ctx, payableID, PayablePaymentCreateRequest{
		PaymentCode:    "PMT-LED-001",
		PaymentType:    "supplier_payment",
		PaymentMethod:  "bank_transfer",
		Amount:         100,
		ScheduleID:     scheduleID,
		InputInvoiceID: invoiceID,
		PaidAt:         "2026-06-20",
	}, fixture.userID, "Scoped User")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.PaymentAction(ctx, cleanID(payment["id"]), "approve", ActionRequest{}, fixture.userID, "Scoped User"); err != nil {
		t.Fatal(err)
	}

	snapshot, err := service.PayableReconciliation(ctx, payableID, fixture.userID)
	if err != nil {
		t.Fatal(err)
	}
	summary, _ := snapshot["summary"].(Record)
	warnings, _ := snapshot["warnings"].([]Record)
	ledger, _ := snapshot["ledger"].([]Record)
	if summary["aging_bucket"] != "90_plus" {
		t.Fatalf("aging_bucket = %v, want 90_plus", summary["aging_bucket"])
	}
	if summary["reconciliation_status"] != "needs_review" {
		t.Fatalf("reconciliation_status = %v, want needs_review", summary["reconciliation_status"])
	}
	if !hasWarningCode(warnings, "overdue") || !hasWarningCode(warnings, "advance_unpaid") {
		t.Fatalf("warnings = %#v, want overdue and advance_unpaid", warnings)
	}
	if len(ledger) < 4 {
		t.Fatalf("ledger rows = %d, want at least 4", len(ledger))
	}
	if number(summary["remaining_amount"]) != 700 {
		t.Fatalf("remaining_amount = %v, want 700", summary["remaining_amount"])
	}
}

func TestOutputBillingScheduleAdvanceAndPartialReceiptReconcileReceivable(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")
	fixture := setupProjectScopeFixture(t, db, []string{
		"accounting.view",
		"accounting.receivable_view",
		"accounting.receivable_approve",
		"accounting.payment_create",
		"accounting.payment_approve",
	})
	receivableID := newID()
	mustExec(t, db, `
		INSERT INTO receivables (id, project_id, receivable_code, client_name, source_type, gross_amount, received_amount, remaining_amount, status, created_by)
		VALUES ($1,$2,$3,'Output Billing Client','manual',1000,0,1000,'approved',$4)
	`, receivableID, fixture.projectID, "REC-OB-"+receivableID[:8], fixture.userID)

	schedule, err := service.CreateReceivablePaymentSchedule(ctx, receivableID, PayablePaymentScheduleRequest{
		ScheduleCode:  "SCH-OB-001",
		MilestoneName: "Kỳ thu 1",
		ScheduleType:  "progress",
		PlannedAmount: 600,
		DueDate:       "2026-08-15",
	}, fixture.userID, "Scoped User")
	if err != nil {
		t.Fatal(err)
	}
	scheduleID := cleanID(schedule["id"])
	schedule, err = service.ReceivablePaymentScheduleAction(ctx, scheduleID, "approve", ActionRequest{Comment: "Duyệt kỳ"}, fixture.userID, "Scoped User")
	if err != nil {
		t.Fatal(err)
	}
	if schedule["status"] != "approved" || number(schedule["remaining_amount"]) != 600 {
		t.Fatalf("schedule = status %v remaining %v, want approved/600", schedule["status"], schedule["remaining_amount"])
	}

	advance, err := service.CreateReceivableAdvanceReceipt(ctx, receivableID, ReceivableAdvanceReceiptCreateRequest{
		ReceiptCode: "ADV-OB-001",
		Amount:      200,
		DueDate:     "2026-08-01",
		Reason:      "Khách hàng ứng trước",
	}, fixture.userID, "Scoped User")
	if err != nil {
		t.Fatal(err)
	}
	advanceID := cleanID(advance["id"])
	advance, err = service.ReceivableAdvanceReceiptAction(ctx, advanceID, "approve", ActionRequest{Comment: "Duyệt ứng"}, fixture.userID, "Scoped User")
	if err != nil {
		t.Fatal(err)
	}
	if advance["status"] != "approved" {
		t.Fatalf("advance status = %v, want approved", advance["status"])
	}
	receivable, err := repo.GetReceivableDetail(ctx, receivableID)
	if err != nil {
		t.Fatal(err)
	}
	if receivable["status"] != "partially_received" || number(receivable["received_amount"]) != 200 || number(receivable["remaining_amount"]) != 800 {
		t.Fatalf("receivable after advance = status %v received %v remaining %v, want partially_received/200/800", receivable["status"], receivable["received_amount"], receivable["remaining_amount"])
	}

	advance, err = service.SettleReceivableAdvanceReceipt(ctx, advanceID, ReceivableAdvanceSettlementRequest{Amount: 50, ScheduleID: scheduleID, Comment: "Đối soát một phần"}, fixture.userID, "Scoped User")
	if err != nil {
		t.Fatal(err)
	}
	if advance["status"] != "partially_settled" || number(advance["settled_amount"]) != 50 {
		t.Fatalf("advance settlement = status %v settled %v, want partially_settled/50", advance["status"], advance["settled_amount"])
	}

	receipt, err := service.CreateReceivablePayment(ctx, receivableID, ReceivableReceiptCreateRequest{
		PaymentCode:   "RCV-OB-001",
		PaymentType:   "client_receipt",
		PaymentMethod: "bank_transfer",
		Amount:        300,
		ScheduleID:    scheduleID,
		PaidAt:        "2026-08-20",
		Notes:         "Thu một phần kỳ 1",
	}, fixture.userID, "Scoped User")
	if err != nil {
		t.Fatal(err)
	}
	if receipt["status"] != "draft" || receipt["allocation_type"] != "receivable_receipt" {
		t.Fatalf("receipt = %#v, want draft receivable allocation", receipt)
	}
	receipt, err = service.PaymentAction(ctx, cleanID(receipt["id"]), "approve", ActionRequest{Comment: "Duyệt phiếu thu"}, fixture.userID, "Scoped User")
	if err != nil {
		t.Fatal(err)
	}
	if receipt["status"] != "approved" {
		t.Fatalf("receipt status = %v, want approved", receipt["status"])
	}
	receivable, err = repo.GetReceivableDetail(ctx, receivableID)
	if err != nil {
		t.Fatal(err)
	}
	if receivable["status"] != "partially_received" || number(receivable["received_amount"]) != 500 || number(receivable["remaining_amount"]) != 500 {
		t.Fatalf("receivable after receipt = status %v received %v remaining %v, want partially_received/500/500", receivable["status"], receivable["received_amount"], receivable["remaining_amount"])
	}
	schedule, err = repo.GetReceivablePaymentScheduleDetail(ctx, scheduleID)
	if err != nil {
		t.Fatal(err)
	}
	if schedule["status"] != "partially_received" || number(schedule["received_amount"]) != 300 || number(schedule["remaining_amount"]) != 300 {
		t.Fatalf("schedule after receipt = status %v received %v remaining %v, want partially_received/300/300", schedule["status"], schedule["received_amount"], schedule["remaining_amount"])
	}

	advanceReceipt, err := service.CreateReceivablePayment(ctx, receivableID, ReceivableReceiptCreateRequest{
		PaymentCode:      "RCV-OB-ADV",
		PaymentType:      "advance",
		PaymentMethod:    "cash",
		Amount:           200,
		AdvanceReceiptID: advanceID,
		PaidAt:           "2026-08-02",
	}, fixture.userID, "Scoped User")
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.PaymentAction(ctx, cleanID(advanceReceipt["id"]), "approve", ActionRequest{Comment: "Thu tiền ứng trước"}, fixture.userID, "Scoped User")
	if err != nil {
		t.Fatal(err)
	}
	advance, err = repo.GetReceivableAdvanceReceiptDetail(ctx, advanceID)
	if err != nil {
		t.Fatal(err)
	}
	if number(advance["received_amount"]) != 200 {
		t.Fatalf("advance received_amount = %v, want 200", advance["received_amount"])
	}
	receivable, err = repo.GetReceivableDetail(ctx, receivableID)
	if err != nil {
		t.Fatal(err)
	}
	if number(receivable["remaining_amount"]) != 500 {
		t.Fatalf("advance receipt payment should not reduce receivable twice, remaining = %v, want 500", receivable["remaining_amount"])
	}

	_, err = service.CreateReceivablePayment(ctx, receivableID, ReceivableReceiptCreateRequest{
		PaymentCode:   "RCV-OB-OVER",
		PaymentMethod: "bank_transfer",
		Amount:        1000,
	}, fixture.userID, "Scoped User")
	assertValidationError(t, err)
	assertCount(t, db, `
		SELECT COUNT(*)
		FROM audit_logs
		WHERE project_id=$1
		  AND event_type IN (
		    'receivable_payment_schedule_created',
		    'receivable_payment_schedule_approved',
		    'receivable_advance_receipt_created',
		    'receivable_advance_receipt_approved',
		    'receivable_advance_receipt_settled',
		    'receivable_payment_created',
		    'receivable_payment_allocated',
		    'receivable_advance_payment_recorded'
		  )
	`, 9, fixture.projectID)
}

func TestReceivableReconciliationSnapshotIncludesAgingWarningsAndLedger(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")
	fixture := setupProjectScopeFixture(t, db, []string{
		"accounting.view",
		"accounting.receivable_view",
		"accounting.receivable_approve",
		"accounting.payment_create",
		"accounting.payment_approve",
	})
	receivableID := newID()
	invoiceID := newID()
	mustExec(t, db, `
		INSERT INTO receivables (id, project_id, receivable_code, client_name, source_type, gross_amount, received_amount, remaining_amount, due_date, status, created_by)
		VALUES ($1,$2,$3,'Ledger Client','manual',1000,0,1000,'2000-01-01','approved',$4)
	`, receivableID, fixture.projectID, "REC-LED-"+receivableID[:8], fixture.userID)
	mustExec(t, db, `
		INSERT INTO output_invoices (
			id, project_id, receivable_id, invoice_number, invoice_date, client_name,
			amount_before_vat, vat_amount, total_amount, status,
			storage_provider, storage_key, mime_type, file_name, file_size, checksum_sha256,
			uploaded_by, issued_by, issued_at
		)
		VALUES ($1,$2,$3,'OUT-LED-001','2026-06-01','Ledger Client',
			300,0,300,'issued',
			'local','output-invoices/test.pdf','application/pdf','test.pdf',128,$4,
			$5,$5,now())
	`, invoiceID, fixture.projectID, receivableID, strings.Repeat("b", 64), fixture.userID)

	schedule, err := service.CreateReceivablePaymentSchedule(ctx, receivableID, PayablePaymentScheduleRequest{
		ScheduleCode:  "SCH-REC-LED-001",
		MilestoneName: "Kỳ thu ledger",
		ScheduleType:  "progress",
		PlannedAmount: 500,
		DueDate:       "2000-02-01",
	}, fixture.userID, "Scoped User")
	if err != nil {
		t.Fatal(err)
	}
	scheduleID := cleanID(schedule["id"])
	if _, err := service.ReceivablePaymentScheduleAction(ctx, scheduleID, "approve", ActionRequest{}, fixture.userID, "Scoped User"); err != nil {
		t.Fatal(err)
	}

	advance, err := service.CreateReceivableAdvanceReceipt(ctx, receivableID, ReceivableAdvanceReceiptCreateRequest{
		ReceiptCode: "ADV-REC-LED-001",
		Amount:      200,
		DueDate:     "2000-01-15",
		Reason:      "Ứng trước ledger",
	}, fixture.userID, "Scoped User")
	if err != nil {
		t.Fatal(err)
	}
	advanceID := cleanID(advance["id"])
	if _, err := service.ReceivableAdvanceReceiptAction(ctx, advanceID, "approve", ActionRequest{}, fixture.userID, "Scoped User"); err != nil {
		t.Fatal(err)
	}

	receipt, err := service.CreateReceivablePayment(ctx, receivableID, ReceivableReceiptCreateRequest{
		PaymentCode:     "RCV-LED-001",
		PaymentType:     "client_receipt",
		PaymentMethod:   "bank_transfer",
		Amount:          100,
		ScheduleID:      scheduleID,
		OutputInvoiceID: invoiceID,
		PaidAt:          "2026-06-20",
	}, fixture.userID, "Scoped User")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.PaymentAction(ctx, cleanID(receipt["id"]), "approve", ActionRequest{}, fixture.userID, "Scoped User"); err != nil {
		t.Fatal(err)
	}

	snapshot, err := service.ReceivableReconciliation(ctx, receivableID, fixture.userID)
	if err != nil {
		t.Fatal(err)
	}
	summary, _ := snapshot["summary"].(Record)
	warnings, _ := snapshot["warnings"].([]Record)
	ledger, _ := snapshot["ledger"].([]Record)
	if summary["aging_bucket"] != "90_plus" {
		t.Fatalf("aging_bucket = %v, want 90_plus", summary["aging_bucket"])
	}
	if summary["reconciliation_status"] != "needs_review" {
		t.Fatalf("reconciliation_status = %v, want needs_review", summary["reconciliation_status"])
	}
	if !hasWarningCode(warnings, "overdue") || !hasWarningCode(warnings, "advance_unreceived") {
		t.Fatalf("warnings = %#v, want overdue and advance_unreceived", warnings)
	}
	if len(ledger) < 4 {
		t.Fatalf("ledger rows = %d, want at least 4", len(ledger))
	}
	if number(summary["remaining_amount"]) != 700 {
		t.Fatalf("remaining_amount = %v, want 700", summary["remaining_amount"])
	}
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

func TestConstructionDailyReportFormValidatesEvidenceAndCreatesNextPlanWorkItems(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")
	fixture := setupWorkItemFlowFixture(t, db, false)

	rows, err := repo.WorkflowFormCatalog(ctx)
	if err != nil {
		t.Fatal(err)
	}
	foundCatalog := false
	for _, row := range rows {
		if row["template_code"] == "builtin_construction_daily_report" {
			foundCatalog = true
			if row["form_type"] != "daily_report" || number(row["version_no"]) != 1 {
				t.Fatalf("unexpected construction daily report catalog row: %#v", row)
			}
			requiredKeys := stringSlice(row["required_field_keys"])
			for _, key := range []string{"report_date", "reported_by", "construction_scope", "weather", "completed_items", "confirmed"} {
				if !containsString(requiredKeys, key) {
					t.Fatalf("expected %s to be required, got %#v", key, row["required_field_keys"])
				}
			}
		}
	}
	if !foundCatalog {
		t.Fatal("expected construction daily report in workflow form catalog")
	}

	var formVersionID string
	if err := db.QueryRowContext(ctx, `
		SELECT v.id
		FROM workflow_form_templates t
		JOIN workflow_form_versions v ON v.template_id=t.id
		WHERE t.code='builtin_construction_daily_report'
		  AND t.status='active'
		  AND v.status='published'
		ORDER BY v.version_no DESC
		LIMIT 1
	`).Scan(&formVersionID); err != nil {
		t.Fatal(err)
	}
	defaultFormVersionID, err := repo.DefaultWorkflowFormVersionID(ctx, "daily_report", false, "Daily report thi công")
	if err != nil {
		t.Fatal(err)
	}
	if defaultFormVersionID != formVersionID {
		t.Fatalf("default construction daily report form version = %q, want %q", defaultFormVersionID, formVersionID)
	}

	evidence, err := service.AddWorkItemEvidence(ctx, fixture.itemID, WorkItemEvidenceRequest{
		EvidenceType: "photo",
		Name:         "Ảnh hiện trường ngày 12",
		FileURL:      "https://example.com/site-photo.jpg",
		Visibility:   "project",
	}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	evidenceID := fmt.Sprint(evidence["id"])
	instanceRows, err := repo.Query(ctx, `
		INSERT INTO workflow_form_instances (project_id, work_item_id, form_template_id, form_version_id, form_type, title, created_by)
		SELECT $1, $2, v.template_id, v.id, t.form_type, 'Construction daily report form', $3
		FROM workflow_form_versions v
		JOIN workflow_form_templates t ON t.id=v.template_id
		WHERE v.id=$4
		RETURNING id
	`, fixture.projectID, fixture.itemID, fixture.workerID, formVersionID)
	if err != nil {
		t.Fatal(err)
	}
	instanceID := fmt.Sprint(instanceRows[0]["id"])

	invalidEvidenceAnswers := Record{
		"report_date":            "2026-07-12",
		"reported_by":            "Supervisor",
		"construction_scope":     "civil",
		"weather":                "normal",
		"site_photo_evidence_id": newID(),
		"completed_items": []any{
			map[string]any{"title": "Xây tường trục A", "actual_quantity": "24.5", "unit": "m2", "result": "done"},
		},
		"confirmed": true,
	}
	_, err = service.SubmitWorkflowForm(ctx, instanceID, WorkflowFormSaveRequest{Answers: invalidEvidenceAnswers}, fixture.workerID, "Worker")
	assertValidationError(t, err)

	invalidQuantityAnswers := Record{
		"report_date":        "2026-07-12",
		"reported_by":        "Supervisor",
		"construction_scope": "civil",
		"weather":            "normal",
		"completed_items": []any{
			map[string]any{"title": "Xây tường trục A", "actual_quantity": "-1", "unit": "m2", "result": "done"},
		},
		"confirmed": true,
	}
	_, err = service.SubmitWorkflowForm(ctx, instanceID, WorkflowFormSaveRequest{Answers: invalidQuantityAnswers}, fixture.workerID, "Worker")
	assertValidationError(t, err)

	readyAnswers := Record{
		"report_date":            "2026-07-12",
		"reported_by":            "Supervisor",
		"construction_scope":     "civil",
		"weather":                "normal",
		"progress_percent":       42,
		"site_photo_evidence_id": evidenceID,
		"completed_items": []any{
			map[string]any{
				"title":           "Xây tường trục A",
				"work_area":       "Tầng 2",
				"crew_name":       "Tổ nề A",
				"actual_quantity": "24.5",
				"unit":            "m2",
				"material_used":   "Gạch AAC",
				"result":          "done",
				"evidence_id":     evidenceID,
				"note":            "Đạt theo kiểm tra cuối ngày",
			},
		},
		"crew_items": []any{
			map[string]any{"crew_name": "Tổ nề A", "trade": "Masonry", "worker_count": "8", "work_hours": "8", "note": "Đủ quân số"},
		},
		"material_usage_items": []any{
			map[string]any{"material_name": "Xi măng", "material_code": "XM-01", "quantity": "12", "unit": "bao", "source": "site_balance", "note": "Ghi nhận report, không trừ kho"},
		},
		"next_plan_items": []any{
			map[string]any{"title": "Tô tường khu B", "owner_role": "Giám sát", "owner_name": "Supervisor", "due_date": "2026-07-13", "priority": "high", "description": "Bám theo tiến độ tuần"},
		},
		"blockers": []any{
			map[string]any{"content": "Cần bổ sung xi măng trước ca chiều", "impact": "material", "owner_role": "Thủ kho", "target_date": "2026-07-13", "severity": "medium"},
		},
		"confirmed": true,
	}
	form, err := service.SubmitWorkflowForm(ctx, instanceID, WorkflowFormSaveRequest{Answers: readyAnswers}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	instance, _ := form["instance"].(Record)
	if instance["status"] != "submitted" {
		t.Fatalf("form status = %#v, want submitted", instance["status"])
	}
	assertCount(t, db, `SELECT COUNT(*) FROM workflow_form_answers WHERE instance_id=$1 AND field_key='completed_items' AND value_json->0->>'actual_quantity'='24.5'`, 1, instanceID)
	assertCount(t, db, `SELECT COUNT(*) FROM workflow_form_answers WHERE instance_id=$1 AND field_key='material_usage_items' AND value_json->0->>'source'='site_balance'`, 1, instanceID)
	assertCount(t, db, `SELECT COUNT(*) FROM work_items WHERE project_id=$1 AND source_type='workflow_form_action_item' AND source_id=$2`, 1, fixture.projectID, instanceID)
	assertCount(t, db, `
		SELECT COUNT(*)
		FROM work_item_activities wia
		JOIN work_items wi ON wi.id=wia.work_item_id
		WHERE wi.source_type='workflow_form_action_item'
		  AND wi.source_id=$1
		  AND wia.activity_type='created'
		  AND wia.comment='Next plan item generated from construction daily report'
		  AND wia.metadata_json->>'source'='construction_daily_report'
		  AND wia.metadata_json->>'construction_scope'='civil'
	`, 1, instanceID)
	assertCount(t, db, `
		SELECT COUNT(*)
		FROM work_item_activities
		WHERE work_item_id=$1
		  AND activity_type='system_event'
		  AND comment='Construction daily report generated 1 next plan work items'
		  AND metadata_json->>'source'='construction_daily_report'
	`, 1, fixture.itemID)
	assertCount(t, db, `SELECT COUNT(*) FROM work_items WHERE id=$1 AND completion_payload_json->'workflow_form_answers'->'material_usage_items'->0->>'source'='site_balance'`, 1, fixture.itemID)
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

func TestSubcontractorChecklistFormValidatesEvidenceAndQuantities(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")
	fixture := setupWorkItemFlowFixture(t, db, false)

	rows, err := repo.WorkflowFormCatalog(ctx)
	if err != nil {
		t.Fatal(err)
	}
	foundCatalog := false
	for _, row := range rows {
		if row["template_code"] == "builtin_subcontractor_checklist" {
			foundCatalog = true
			if row["form_type"] != "checklist" || number(row["version_no"]) != 1 {
				t.Fatalf("unexpected subcontractor checklist catalog row: %#v", row)
			}
			if !containsString(stringSlice(row["required_field_keys"]), "checklist_items") {
				t.Fatalf("expected checklist_items to be required, got %#v", row["required_field_keys"])
			}
		}
	}
	if !foundCatalog {
		t.Fatal("expected subcontractor checklist in workflow form catalog")
	}

	var formVersionID string
	if err := db.QueryRowContext(ctx, `
		SELECT v.id
		FROM workflow_form_templates t
		JOIN workflow_form_versions v ON v.template_id=t.id
		WHERE t.code='builtin_subcontractor_checklist'
		  AND t.status='active'
		  AND v.status='published'
		ORDER BY v.version_no DESC
		LIMIT 1
	`).Scan(&formVersionID); err != nil {
		t.Fatal(err)
	}
	defaultFormVersionID, err := repo.DefaultWorkflowFormVersionID(ctx, "checklist", true, "Checklist thầu phụ")
	if err != nil {
		t.Fatal(err)
	}
	if defaultFormVersionID != formVersionID {
		t.Fatalf("default subcontractor checklist form version = %q, want %q", defaultFormVersionID, formVersionID)
	}

	evidence, err := service.AddWorkItemEvidence(ctx, fixture.itemID, WorkItemEvidenceRequest{
		EvidenceType: "checklist",
		Name:         "Subcontractor checklist evidence",
		Visibility:   "project",
	}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	evidenceID := fmt.Sprint(evidence["id"])
	instanceRows, err := repo.Query(ctx, `
		INSERT INTO workflow_form_instances (project_id, work_item_id, form_template_id, form_version_id, form_type, title, created_by)
		SELECT $1, $2, v.template_id, v.id, t.form_type, 'Subcontractor checklist form', $3
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
		"check_date":                   "2026-07-10",
		"subcontractor_name":           "Đội trần A",
		"subcontractor_representative": "Anh B",
		"work_package":                 "ceiling",
		"construction_area":            "Tầng 2",
		"checklist_items": []any{
			map[string]any{
				"title":              "Lắp khung xương trần",
				"boq_item_ref":       "BOQ-C-01",
				"claimed_quantity":   "12.5",
				"confirmed_quantity": "10",
				"unit":               "m2",
				"status":             "needs_follow_up",
				"evidence_id":        evidenceID,
				"owner_response":     "adjust_quantity",
				"due_date":           "2026-07-11",
				"comment":            "Cần kiểm tra lại mép biên",
			},
		},
		"overall_comment": "Chưa đủ điều kiện submit",
		"confirmed":       true,
	}
	_, err = service.SubmitWorkflowForm(ctx, instanceID, WorkflowFormSaveRequest{Answers: blockedAnswers}, fixture.workerID, "Worker")
	assertValidationError(t, err)

	invalidQuantityAnswers := Record{
		"check_date":         "2026-07-10",
		"subcontractor_name": "Đội trần A",
		"work_package":       "ceiling",
		"checklist_items": []any{
			map[string]any{
				"title":              "Lắp khung xương trần",
				"claimed_quantity":   "-1",
				"confirmed_quantity": "10",
				"status":             "done",
			},
		},
		"confirmed": true,
	}
	_, err = service.SubmitWorkflowForm(ctx, instanceID, WorkflowFormSaveRequest{Answers: invalidQuantityAnswers}, fixture.workerID, "Worker")
	assertValidationError(t, err)

	invalidEvidenceAnswers := Record{
		"check_date":         "2026-07-10",
		"subcontractor_name": "Đội trần A",
		"work_package":       "ceiling",
		"checklist_items": []any{
			map[string]any{
				"title":       "Lắp khung xương trần",
				"status":      "done",
				"evidence_id": newID(),
			},
		},
		"confirmed": true,
	}
	_, err = service.SubmitWorkflowForm(ctx, instanceID, WorkflowFormSaveRequest{Answers: invalidEvidenceAnswers}, fixture.workerID, "Worker")
	assertValidationError(t, err)

	readyAnswers := Record{
		"check_date":                   "2026-07-10",
		"subcontractor_name":           "Đội trần A",
		"subcontractor_representative": "Anh B",
		"work_package":                 "ceiling",
		"construction_area":            "Tầng 2",
		"checklist_items": []any{
			map[string]any{
				"title":              "Lắp khung xương trần",
				"boq_item_ref":       "BOQ-C-01",
				"claimed_quantity":   "12.5",
				"confirmed_quantity": "12",
				"unit":               "m2",
				"status":             "done",
				"evidence_id":        evidenceID,
				"owner_response":     "accepted",
				"comment":            "Đã đối chiếu evidence",
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
	assertCount(t, db, `SELECT COUNT(*) FROM workflow_form_answers WHERE instance_id=$1 AND field_key='checklist_items' AND value_json->0->>'claimed_quantity'='12.5'`, 1, instanceID)
	assertCount(t, db, `SELECT COUNT(*) FROM workflow_form_events WHERE instance_id=$1 AND event_type='submitted'`, 1, instanceID)
	assertCount(t, db, `SELECT COUNT(*) FROM work_items WHERE id=$1 AND completion_payload_json->'checklist_items'->0->>'confirmed_quantity'='12'`, 1, fixture.itemID)
}

func TestSubcontractorChecklistReconciliationPersistsAuditAndValidatesReferences(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")
	fixture := setupWorkItemFlowFixture(t, db, false)

	boqItem, err := service.CreateBOQ(ctx, fixture.projectID, Record{
		"category":            "Trần",
		"work_item_name":      "BOQ-C-01 Lắp khung xương trần",
		"unit":                "m2",
		"quantity":            "12.5",
		"expected_unit_price": "100000",
		"status":              "approved",
	}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := service.AddWorkItemEvidence(ctx, fixture.itemID, WorkItemEvidenceRequest{
		EvidenceType: "checklist",
		Name:         "Subcontractor reconciliation evidence",
		Visibility:   "project",
	}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}

	var formVersionID string
	if err := db.QueryRowContext(ctx, `
		SELECT v.id
		FROM workflow_form_templates t
		JOIN workflow_form_versions v ON v.template_id=t.id
		WHERE t.code='builtin_subcontractor_checklist'
		  AND t.status='active'
		  AND v.status='published'
		ORDER BY v.version_no DESC
		LIMIT 1
	`).Scan(&formVersionID); err != nil {
		t.Fatal(err)
	}
	instanceRows, err := repo.Query(ctx, `
		INSERT INTO workflow_form_instances (project_id, work_item_id, form_template_id, form_version_id, form_type, title, created_by)
		SELECT $1, $2, v.template_id, v.id, t.form_type, 'Subcontractor checklist form', $3
		FROM workflow_form_versions v
		JOIN workflow_form_templates t ON t.id=v.template_id
		WHERE v.id=$4
		RETURNING id
	`, fixture.projectID, fixture.itemID, fixture.workerID, formVersionID)
	if err != nil {
		t.Fatal(err)
	}
	instanceID := fmt.Sprint(instanceRows[0]["id"])
	answers := Record{
		"check_date":                   "2026-07-10",
		"subcontractor_name":           "Đội trần A",
		"subcontractor_representative": "Anh B",
		"work_package":                 "ceiling",
		"construction_area":            "Tầng 2",
		"checklist_items": []any{
			map[string]any{
				"title":              "Lắp khung xương trần",
				"boq_item_ref":       "BOQ-C-01",
				"claimed_quantity":   "12.5",
				"confirmed_quantity": "10",
				"unit":               "m2",
				"status":             "done",
				"evidence_id":        fmt.Sprint(evidence["id"]),
				"owner_response":     "adjust_quantity",
				"comment":            "Đội thầu phụ claim vượt KL nội bộ",
			},
		},
		"overall_comment": "Chuyển đối soát nội bộ",
		"confirmed":       true,
	}
	if _, err := service.SubmitWorkflowForm(ctx, instanceID, WorkflowFormSaveRequest{Answers: answers}, fixture.workerID, "Worker"); err != nil {
		t.Fatal(err)
	}

	draft, err := service.SubcontractorReconciliation(ctx, fixture.itemID, fixture.workerID)
	if err != nil {
		t.Fatal(err)
	}
	if draft["available"] != true {
		t.Fatalf("reconciliation available = %#v, want true", draft["available"])
	}
	reconciliation, _ := draft["reconciliation"].(Record)
	sourceItems, _ := reconciliation["items"].([]Record)
	if len(sourceItems) != 1 || sourceItems[0]["decision"] != "adjust_quantity" {
		t.Fatalf("unexpected default reconciliation items: %#v", sourceItems)
	}

	confirmed := 9.0
	_, err = service.SaveSubcontractorReconciliation(ctx, fixture.itemID, SubcontractorReconciliationRequest{
		Items: []SubcontractorReconciliationItemRequest{{
			ItemIndex:         0,
			BOQItemID:         newID(),
			ConfirmedQuantity: &confirmed,
			Decision:          "adjust_quantity",
		}},
	}, fixture.workerID, "Worker")
	assertValidationError(t, err)

	submitted, err := service.SubmitSubcontractorReconciliation(ctx, fixture.itemID, SubcontractorReconciliationRequest{
		Comment: "Chốt KL nội bộ 9m2",
		Items: []SubcontractorReconciliationItemRequest{{
			ItemIndex:         0,
			BOQItemID:         fmt.Sprint(boqItem["id"]),
			ConfirmedQuantity: &confirmed,
			Decision:          "adjust_quantity",
			Comment:           "Cắt phần claim chưa đủ evidence",
		}},
	}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	submittedRecord, _ := submitted["reconciliation"].(Record)
	if submittedRecord["status"] != "submitted" {
		t.Fatalf("reconciliation status = %#v, want submitted", submittedRecord["status"])
	}

	var status, decision, mappedBOQID string
	var claimedQuantity, confirmedQuantity, varianceQuantity float64
	if err := db.QueryRowContext(ctx, `
		SELECT scr.status, scri.claimed_quantity, scri.confirmed_quantity, scri.variance_quantity, scri.decision, scri.boq_item_id::text
		FROM subcontractor_checklist_reconciliations scr
		JOIN subcontractor_checklist_reconciliation_items scri ON scri.reconciliation_id=scr.id
		WHERE scr.work_item_id=$1
	`, fixture.itemID).Scan(&status, &claimedQuantity, &confirmedQuantity, &varianceQuantity, &decision, &mappedBOQID); err != nil {
		t.Fatal(err)
	}
	if status != "submitted" || decision != "adjust_quantity" || mappedBOQID != fmt.Sprint(boqItem["id"]) {
		t.Fatalf("unexpected reconciliation row: status=%s decision=%s boq=%s", status, decision, mappedBOQID)
	}
	if claimedQuantity != 12.5 || confirmedQuantity != 9 || varianceQuantity != -3.5 {
		t.Fatalf("unexpected quantities: claimed=%v confirmed=%v variance=%v", claimedQuantity, confirmedQuantity, varianceQuantity)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM workflow_form_answers WHERE instance_id=$1 AND field_key='checklist_items' AND value_json->0->>'confirmed_quantity'='10'`, 1, instanceID)
	assertCount(t, db, `SELECT COUNT(*) FROM work_item_activities WHERE work_item_id=$1 AND activity_type='system_event' AND metadata_json->>'status'='submitted'`, 1, fixture.itemID)
	assertCount(t, db, `SELECT COUNT(*) FROM audit_logs WHERE project_id=$1 AND event_type='subcontractor_reconciliation_submitted'`, 1, fixture.projectID)

	_, err = service.SaveSubcontractorReconciliation(ctx, fixture.itemID, SubcontractorReconciliationRequest{
		Items: []SubcontractorReconciliationItemRequest{{
			ItemIndex: 0,
			Decision:  "accepted",
		}},
	}, fixture.workerID, "Worker")
	assertValidationError(t, err)
}

func TestOwnerClientChecklistFormValidatesEvidenceAndSubmitGate(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")
	fixture := setupWorkItemFlowFixture(t, db, false)

	rows, err := repo.WorkflowFormCatalog(ctx)
	if err != nil {
		t.Fatal(err)
	}
	foundCatalog := false
	for _, row := range rows {
		if row["template_code"] == "builtin_owner_client_checklist" {
			foundCatalog = true
			if row["form_type"] != "checklist" || number(row["version_no"]) != 1 {
				t.Fatalf("unexpected owner/client checklist catalog row: %#v", row)
			}
			if !containsString(stringSlice(row["required_field_keys"]), "checklist_items") {
				t.Fatalf("expected checklist_items to be required, got %#v", row["required_field_keys"])
			}
		}
	}
	if !foundCatalog {
		t.Fatal("expected owner/client checklist in workflow form catalog")
	}

	var formVersionID string
	if err := db.QueryRowContext(ctx, `
		SELECT v.id
		FROM workflow_form_templates t
		JOIN workflow_form_versions v ON v.template_id=t.id
		WHERE t.code='builtin_owner_client_checklist'
		  AND t.status='active'
		  AND v.status='published'
		ORDER BY v.version_no DESC
		LIMIT 1
	`).Scan(&formVersionID); err != nil {
		t.Fatal(err)
	}
	defaultFormVersionID, err := repo.DefaultWorkflowFormVersionID(ctx, "checklist", true, "Checklist chủ đầu tư")
	if err != nil {
		t.Fatal(err)
	}
	if defaultFormVersionID != formVersionID {
		t.Fatalf("default owner/client checklist form version = %q, want %q", defaultFormVersionID, formVersionID)
	}

	evidence, err := service.AddWorkItemEvidence(ctx, fixture.itemID, WorkItemEvidenceRequest{
		EvidenceType: "checklist",
		Name:         "Owner checklist evidence",
		Visibility:   "client",
	}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	evidenceID := fmt.Sprint(evidence["id"])
	instanceRows, err := repo.Query(ctx, `
		INSERT INTO workflow_form_instances (project_id, work_item_id, form_template_id, form_version_id, form_type, title, created_by)
		SELECT $1, $2, v.template_id, v.id, t.form_type, 'Owner client checklist form', $3
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
		"check_date":             "2026-07-12",
		"client_representative":  "Client A",
		"company_representative": "Supervisor B",
		"handover_scope":         "area",
		"scope_detail":           "Tầng 2",
		"checklist_items": []any{
			map[string]any{
				"title":           "Kiểm tra hoàn thiện sơn",
				"output_ref":      "ACC-01",
				"status":          "needs_follow_up",
				"client_decision": "conditional_pass",
				"evidence_id":     evidenceID,
				"severity":        "medium",
				"required_action": "Bổ sung vá lỗi mép tường",
				"due_date":        "2026-07-13",
				"comment":         "Cần xử lý trước bàn giao",
			},
		},
		"final_client_note": "Chưa đủ điều kiện submit",
		"confirmed":         true,
	}
	_, err = service.SubmitWorkflowForm(ctx, instanceID, WorkflowFormSaveRequest{Answers: blockedAnswers}, fixture.workerID, "Worker")
	assertValidationError(t, err)

	invalidEvidenceAnswers := Record{
		"check_date":            "2026-07-12",
		"client_representative": "Client A",
		"handover_scope":        "area",
		"checklist_items": []any{
			map[string]any{
				"title":           "Kiểm tra hoàn thiện sơn",
				"status":          "done",
				"client_decision": "pass",
				"evidence_id":     newID(),
			},
		},
		"confirmed": true,
	}
	_, err = service.SubmitWorkflowForm(ctx, instanceID, WorkflowFormSaveRequest{Answers: invalidEvidenceAnswers}, fixture.workerID, "Worker")
	assertValidationError(t, err)

	readyAnswers := Record{
		"check_date":             "2026-07-12",
		"client_representative":  "Client A",
		"company_representative": "Supervisor B",
		"handover_scope":         "area",
		"scope_detail":           "Tầng 2",
		"checklist_items": []any{
			map[string]any{
				"title":           "Kiểm tra hoàn thiện sơn",
				"output_ref":      "ACC-01",
				"status":          "done",
				"client_decision": "pass",
				"evidence_id":     evidenceID,
				"comment":         "Đạt theo biên bản",
			},
		},
		"final_client_note": "Đủ điều kiện xác nhận",
		"confirmed":         true,
	}
	form, err := service.SubmitWorkflowForm(ctx, instanceID, WorkflowFormSaveRequest{Answers: readyAnswers}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	instance, _ := form["instance"].(Record)
	if instance["status"] != "submitted" {
		t.Fatalf("form status = %#v, want submitted", instance["status"])
	}
	assertCount(t, db, `SELECT COUNT(*) FROM workflow_form_answers WHERE instance_id=$1 AND field_key='checklist_items' AND value_json->0->>'client_decision'='pass'`, 1, instanceID)
	assertCount(t, db, `SELECT COUNT(*) FROM workflow_form_events WHERE instance_id=$1 AND event_type='submitted'`, 1, instanceID)
	assertCount(t, db, `SELECT COUNT(*) FROM work_items WHERE id=$1 AND completion_payload_json->'checklist_items'->0->>'client_decision'='pass'`, 1, fixture.itemID)
}

func TestOwnerClientChecklistReconciliationPersistsAuditAndValidatesAcceptance(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")
	fixture := setupWorkItemFlowFixture(t, db, false)

	acceptanceID := newID()
	mustExec(t, db, `
		INSERT INTO acceptance_records (id, project_id, type, title, requested_by, approved_by, status, result, notes)
		VALUES ($1,$2,'final','Final acceptance tầng 2',$3,$3,'completed','pass','Nội bộ nghiệm thu đạt')
	`, acceptanceID, fixture.projectID, fixture.workerID)
	evidence, err := service.AddWorkItemEvidence(ctx, fixture.itemID, WorkItemEvidenceRequest{
		EvidenceType: "checklist",
		Name:         "Owner reconciliation evidence",
		Visibility:   "client",
	}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}

	var formVersionID string
	if err := db.QueryRowContext(ctx, `
		SELECT v.id
		FROM workflow_form_templates t
		JOIN workflow_form_versions v ON v.template_id=t.id
		WHERE t.code='builtin_owner_client_checklist'
		  AND t.status='active'
		  AND v.status='published'
		ORDER BY v.version_no DESC
		LIMIT 1
	`).Scan(&formVersionID); err != nil {
		t.Fatal(err)
	}
	instanceRows, err := repo.Query(ctx, `
		INSERT INTO workflow_form_instances (project_id, work_item_id, form_template_id, form_version_id, form_type, title, created_by)
		SELECT $1, $2, v.template_id, v.id, t.form_type, 'Owner client checklist form', $3
		FROM workflow_form_versions v
		JOIN workflow_form_templates t ON t.id=v.template_id
		WHERE v.id=$4
		RETURNING id
	`, fixture.projectID, fixture.itemID, fixture.workerID, formVersionID)
	if err != nil {
		t.Fatal(err)
	}
	instanceID := fmt.Sprint(instanceRows[0]["id"])
	answers := Record{
		"check_date":             "2026-07-12",
		"client_representative":  "Client A",
		"company_representative": "Supervisor B",
		"handover_scope":         "area",
		"scope_detail":           "Tầng 2",
		"checklist_items": []any{
			map[string]any{
				"title":           "Kiểm tra hoàn thiện sơn",
				"output_ref":      "ACC-01",
				"status":          "done",
				"client_decision": "pass",
				"evidence_id":     fmt.Sprint(evidence["id"]),
				"comment":         "Đạt theo biên bản",
			},
		},
		"final_client_note": "Đủ điều kiện xác nhận",
		"confirmed":         true,
	}
	if _, err := service.SubmitWorkflowForm(ctx, instanceID, WorkflowFormSaveRequest{Answers: answers}, fixture.workerID, "Worker"); err != nil {
		t.Fatal(err)
	}

	draft, err := service.OwnerClientReconciliation(ctx, fixture.itemID, fixture.workerID)
	if err != nil {
		t.Fatal(err)
	}
	if draft["available"] != true {
		t.Fatalf("reconciliation available = %#v, want true", draft["available"])
	}
	reconciliation, _ := draft["reconciliation"].(Record)
	sourceItems, _ := reconciliation["items"].([]Record)
	if len(sourceItems) != 1 || sourceItems[0]["decision"] != "accepted" {
		t.Fatalf("unexpected default owner reconciliation items: %#v", sourceItems)
	}

	saved, err := service.SaveOwnerClientReconciliation(ctx, fixture.itemID, OwnerClientReconciliationRequest{
		Comment: "Draft trước khi link nghiệm thu",
		Items: []OwnerClientReconciliationItemRequest{{
			ItemIndex: 0,
			Decision:  "accepted",
		}},
	}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	savedRecord, _ := saved["reconciliation"].(Record)
	if savedRecord["status"] != "draft" {
		t.Fatalf("draft reconciliation status = %#v, want draft", savedRecord["status"])
	}

	_, err = service.SubmitOwnerClientReconciliation(ctx, fixture.itemID, OwnerClientReconciliationRequest{
		Items: []OwnerClientReconciliationItemRequest{{
			ItemIndex: 0,
			Decision:  "accepted",
		}},
	}, fixture.workerID, "Worker")
	assertValidationError(t, err)

	_, err = service.SubmitOwnerClientReconciliation(ctx, fixture.itemID, OwnerClientReconciliationRequest{
		Items: []OwnerClientReconciliationItemRequest{{
			ItemIndex:    0,
			AcceptanceID: newID(),
			Decision:     "accepted",
		}},
	}, fixture.workerID, "Worker")
	assertValidationError(t, err)

	submitted, err := service.SubmitOwnerClientReconciliation(ctx, fixture.itemID, OwnerClientReconciliationRequest{
		Comment: "Chốt theo final acceptance",
		Items: []OwnerClientReconciliationItemRequest{{
			ItemIndex:    0,
			AcceptanceID: acceptanceID,
			Decision:     "accepted",
			Comment:      "Đạt theo nghiệm thu nội bộ",
		}},
	}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	submittedRecord, _ := submitted["reconciliation"].(Record)
	if submittedRecord["status"] != "submitted" {
		t.Fatalf("reconciliation status = %#v, want submitted", submittedRecord["status"])
	}

	var status, decision, mappedAcceptanceID, clientDecision, acceptanceResult string
	if err := db.QueryRowContext(ctx, `
		SELECT ocr.status, ocri.decision, ocri.acceptance_id::text, ocri.client_decision, ocri.acceptance_result
		FROM owner_client_checklist_reconciliations ocr
		JOIN owner_client_checklist_reconciliation_items ocri ON ocri.reconciliation_id=ocr.id
		WHERE ocr.work_item_id=$1
	`, fixture.itemID).Scan(&status, &decision, &mappedAcceptanceID, &clientDecision, &acceptanceResult); err != nil {
		t.Fatal(err)
	}
	if status != "submitted" || decision != "accepted" || mappedAcceptanceID != acceptanceID || clientDecision != "pass" || acceptanceResult != "pass" {
		t.Fatalf("unexpected owner reconciliation row: status=%s decision=%s acceptance=%s client=%s result=%s", status, decision, mappedAcceptanceID, clientDecision, acceptanceResult)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM workflow_form_answers WHERE instance_id=$1 AND field_key='checklist_items' AND value_json->0->>'client_decision'='pass'`, 1, instanceID)
	assertCount(t, db, `SELECT COUNT(*) FROM work_item_activities WHERE work_item_id=$1 AND activity_type='system_event' AND metadata_json->>'status'='submitted'`, 1, fixture.itemID)
	assertCount(t, db, `SELECT COUNT(*) FROM audit_logs WHERE project_id=$1 AND event_type='owner_client_reconciliation_submitted'`, 1, fixture.projectID)

	_, err = service.SaveOwnerClientReconciliation(ctx, fixture.itemID, OwnerClientReconciliationRequest{
		Items: []OwnerClientReconciliationItemRequest{{
			ItemIndex: 0,
			Decision:  "not_applicable",
		}},
	}, fixture.workerID, "Worker")
	assertValidationError(t, err)
}

func TestMaterialBeforeAfterSnapshotAppliesSiteBalanceWithoutStockMovement(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")
	fixture := setupWorkItemFlowFixture(t, db, false)

	warehouseID := newID()
	materialID := newID()
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM audit_logs WHERE project_id=$1 AND target_type IN ('stock_movement','material_before_after_snapshot')`, fixture.projectID)
		_, _ = db.ExecContext(ctx, `DELETE FROM material_before_after_snapshot_items WHERE material_id=$1`, materialID)
		_, _ = db.ExecContext(ctx, `DELETE FROM material_before_after_snapshots WHERE project_id=$1`, fixture.projectID)
		_, _ = db.ExecContext(ctx, `DELETE FROM stock_movements WHERE material_id=$1`, materialID)
		_, _ = db.ExecContext(ctx, `DELETE FROM site_material_balances WHERE project_id=$1 OR material_id=$2`, fixture.projectID, materialID)
		_, _ = db.ExecContext(ctx, `DELETE FROM materials WHERE id=$1`, materialID)
		_, _ = db.ExecContext(ctx, `DELETE FROM warehouses WHERE id=$1`, warehouseID)
	})
	mustExec(t, db, `INSERT INTO warehouses (id, name, location, type, active) VALUES ($1,'Snapshot Warehouse','Test','central',true)`, warehouseID)
	mustExec(t, db, `INSERT INTO materials (id, code, name, unit, category, active) VALUES ($1,$2,'Snapshot Cable','m','Electrical',true)`, materialID, "SNAP-"+materialID[:8])

	if _, err := repo.CreateStockMovement(ctx, StockActionRequest{
		WarehouseID:  warehouseID,
		MaterialID:   materialID,
		MovementType: "receive",
		Quantity:     20,
		Unit:         "m",
		Reason:       "receive for snapshot test",
	}, fixture.workerID, "Worker"); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CreateStockMovement(ctx, StockActionRequest{
		ProjectID:    fixture.projectID,
		WarehouseID:  warehouseID,
		MaterialID:   materialID,
		MovementType: "issue",
		Quantity:     12,
		Unit:         "m",
		Reason:       "issue to site for snapshot test",
	}, fixture.workerID, "Worker"); err != nil {
		t.Fatal(err)
	}

	beforeEvidence, err := service.AddWorkItemEvidence(ctx, fixture.itemID, WorkItemEvidenceRequest{
		EvidenceType: "photo",
		Name:         "Before material count",
		FileURL:      "/evidence/before-material.jpg",
		Visibility:   "project",
	}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	afterEvidence, err := service.AddWorkItemEvidence(ctx, fixture.itemID, WorkItemEvidenceRequest{
		EvidenceType: "photo",
		Name:         "After material count",
		FileURL:      "/evidence/after-material.jpg",
		Visibility:   "project",
	}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}

	beforeQty := 12.0
	addedQty := 0.0
	afterQty := 7.0
	wasteQty := 1.0
	affects := true
	req := MaterialSnapshotRequest{
		SnapshotDate:     "2026-07-14",
		ConstructionArea: "Tầng 3",
		Comment:          "Nháp kiểm kê vật tư trước/sau",
		Items: []MaterialSnapshotItemRequest{{
			ItemIndex:          0,
			MaterialID:         materialID,
			Source:             "site_balance",
			BeforeQuantity:     &beforeQty,
			AddedQuantity:      &addedQty,
			AfterQuantity:      &afterQty,
			WasteQuantity:      &wasteQty,
			AffectsSiteBalance: &affects,
			BeforeEvidenceID:   fmt.Sprint(beforeEvidence["id"]),
			AfterEvidenceID:    fmt.Sprint(afterEvidence["id"]),
			ConstructionArea:   "Tầng 3 - trục A",
			Condition:          "mixed",
			Comment:            "Đã dùng 4m, hư hao 1m",
		}},
	}

	saved, err := service.SaveMaterialSnapshot(ctx, fixture.itemID, req, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	savedRecord, _ := saved["snapshot"].(Record)
	if savedRecord["status"] != "draft" {
		t.Fatalf("draft snapshot status = %#v, want draft", savedRecord["status"])
	}
	var used, surplus float64
	if err := db.QueryRowContext(ctx, `
		SELECT used_quantity::float8, surplus_quantity::float8
		FROM site_material_balances
		WHERE project_id=$1 AND material_id=$2
	`, fixture.projectID, materialID).Scan(&used, &surplus); err != nil {
		t.Fatal(err)
	}
	if used != 0 || surplus != 12 {
		t.Fatalf("draft changed balance used=%.3f surplus=%.3f, want 0/12", used, surplus)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM stock_movements WHERE material_id=$1`, 2, materialID)

	req.Comment = "Chốt snapshot vật tư"
	submitted, err := service.SubmitMaterialSnapshot(ctx, fixture.itemID, req, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	submittedRecord, _ := submitted["snapshot"].(Record)
	if submittedRecord["status"] != "submitted" || submittedRecord["applied_to_site_balance"] != true {
		t.Fatalf("submitted snapshot = %#v, want submitted/applied", submittedRecord)
	}
	if err := db.QueryRowContext(ctx, `
		SELECT used_quantity::float8, surplus_quantity::float8
		FROM site_material_balances
		WHERE project_id=$1 AND material_id=$2
	`, fixture.projectID, materialID).Scan(&used, &surplus); err != nil {
		t.Fatal(err)
	}
	if used != 4 || surplus != 8 {
		t.Fatalf("submitted balance used=%.3f surplus=%.3f, want 4/8", used, surplus)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM stock_movements WHERE material_id=$1`, 2, materialID)
	assertCount(t, db, `SELECT COUNT(*) FROM material_before_after_snapshot_items WHERE material_id=$1 AND used_quantity=4 AND before_evidence_id=$2 AND after_evidence_id=$3`, 1, materialID, beforeEvidence["id"], afterEvidence["id"])
	assertCount(t, db, `SELECT COUNT(*) FROM work_item_activities WHERE work_item_id=$1 AND activity_type='system_event' AND metadata_json->>'status'='submitted' AND metadata_json ? 'material_snapshot_id'`, 1, fixture.itemID)
	assertCount(t, db, `SELECT COUNT(*) FROM audit_logs WHERE project_id=$1 AND event_type='material_snapshot_submitted'`, 1, fixture.projectID)

	_, err = service.SaveMaterialSnapshot(ctx, fixture.itemID, req, fixture.workerID, "Worker")
	assertValidationError(t, err)
}

func TestSiteSurplusHandlingCreatesStockMovementAndPartialStatus(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")
	fixture := setupWorkItemFlowFixture(t, db, false)

	suffix := newID()
	surplusPermissionID := ensurePermission(t, db, "warehouse.surplus_handle", "Xử lý vật tư dư", "warehouse")
	handlerRoleID := ensureRole(t, db, "surplus_handler_"+suffix[:8], "Surplus Handler")
	mustExec(t, db, `INSERT INTO role_permissions (role_id, permission_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`, handlerRoleID, surplusPermissionID)
	mustExec(t, db, `INSERT INTO user_roles (user_id, role_id) VALUES ($1,$2)`, fixture.workerID, handlerRoleID)
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM roles WHERE id=$1`, handlerRoleID)
	})

	warehouseID := newID()
	materialID := newID()
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM audit_logs WHERE project_id=$1 AND target_type IN ('stock_movement','site_surplus_handling')`, fixture.projectID)
		_, _ = db.ExecContext(ctx, `DELETE FROM site_surplus_handlings WHERE project_id=$1`, fixture.projectID)
		_, _ = db.ExecContext(ctx, `DELETE FROM stock_movements WHERE material_id=$1`, materialID)
		_, _ = db.ExecContext(ctx, `DELETE FROM site_material_balances WHERE project_id=$1 OR material_id=$2`, fixture.projectID, materialID)
		_, _ = db.ExecContext(ctx, `DELETE FROM materials WHERE id=$1`, materialID)
		_, _ = db.ExecContext(ctx, `DELETE FROM warehouses WHERE id=$1`, warehouseID)
	})
	mustExec(t, db, `INSERT INTO warehouses (id, name, location, type, active) VALUES ($1,'Surplus Warehouse','Yard','central',true)`, warehouseID)
	mustExec(t, db, `INSERT INTO materials (id, code, name, unit, category, active) VALUES ($1,$2,'Surplus Cable','m','Electrical',true)`, materialID, "SUR-"+materialID[:8])

	if _, err := repo.CreateStockMovement(ctx, StockActionRequest{
		WarehouseID:  warehouseID,
		MaterialID:   materialID,
		MovementType: "receive",
		Quantity:     20,
		Unit:         "m",
		Reason:       "receive for surplus test",
	}, fixture.workerID, "Worker"); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CreateStockMovement(ctx, StockActionRequest{
		ProjectID:    fixture.projectID,
		WarehouseID:  warehouseID,
		MaterialID:   materialID,
		MovementType: "issue",
		Quantity:     12,
		Unit:         "m",
		Reason:       "issue to site for surplus test",
	}, fixture.workerID, "Worker"); err != nil {
		t.Fatal(err)
	}
	mustExec(t, db, `
		UPDATE site_material_balances
		SET used_quantity=4,
			surplus_quantity=8,
			surplus_status='pending'
		WHERE project_id=$1 AND material_id=$2
	`, fixture.projectID, materialID)

	evidence, err := service.AddWorkItemEvidence(ctx, fixture.itemID, WorkItemEvidenceRequest{
		EvidenceType: "photo",
		Name:         "Surplus material photo",
		FileURL:      "/evidence/surplus-material.jpg",
		Visibility:   "project",
	}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	var balanceID string
	if err := db.QueryRowContext(ctx, `SELECT id::text FROM site_material_balances WHERE project_id=$1 AND material_id=$2`, fixture.projectID, materialID).Scan(&balanceID); err != nil {
		t.Fatal(err)
	}

	firstQty := 3.0
	draft, err := service.CreateSurplusHandling(ctx, fixture.itemID, SurplusHandlingRequest{
		SiteBalanceID:   balanceID,
		HandlingType:    "return_to_warehouse",
		Quantity:        &firstQty,
		Unit:            "m",
		WarehouseID:     warehouseID,
		CustodyLocation: "Kệ A3",
		Condition:       "usable",
		EvidenceID:      fmt.Sprint(evidence["id"]),
		Comment:         "Lưu lại vật tư dư",
	}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	var issued, used, surplus float64
	var surplusStatus string
	if err := db.QueryRowContext(ctx, `
		SELECT issued_quantity::float8, used_quantity::float8, surplus_quantity::float8, surplus_status
		FROM site_material_balances
		WHERE id=$1
	`, balanceID).Scan(&issued, &used, &surplus, &surplusStatus); err != nil {
		t.Fatal(err)
	}
	if issued != 12 || used != 4 || surplus != 8 || surplusStatus != "pending" {
		t.Fatalf("draft changed balance issued=%.3f used=%.3f surplus=%.3f status=%s", issued, used, surplus, surplusStatus)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM stock_movements WHERE material_id=$1`, 2, materialID)

	submitted, err := service.SubmitSurplusHandling(ctx, fmt.Sprint(draft["id"]), fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	if submitted["status"] != "submitted" || cleanID(submitted["stock_movement_id"]) == "" {
		t.Fatalf("submitted surplus handling = %#v, want submitted with stock movement", submitted)
	}
	if err := db.QueryRowContext(ctx, `
		SELECT issued_quantity::float8, used_quantity::float8, surplus_quantity::float8, surplus_status
		FROM site_material_balances
		WHERE id=$1
	`, balanceID).Scan(&issued, &used, &surplus, &surplusStatus); err != nil {
		t.Fatal(err)
	}
	if issued != 9 || used != 4 || surplus != 5 || surplusStatus != "partial" {
		t.Fatalf("partial balance issued=%.3f used=%.3f surplus=%.3f status=%s, want 9/4/5/partial", issued, used, surplus, surplusStatus)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM stock_movements WHERE material_id=$1 AND movement_type='return_to_warehouse' AND reference_type='site_surplus_handling'`, 1, materialID)

	secondQty := 5.0
	second, err := service.CreateSurplusHandling(ctx, fixture.itemID, SurplusHandlingRequest{
		SiteBalanceID:   balanceID,
		HandlingType:    "return_to_subcontractor",
		Quantity:        &secondQty,
		Unit:            "m",
		WarehouseID:     warehouseID,
		CustodyLocation: "Bàn giao đội thầu phụ",
		Condition:       "mixed",
		EvidenceID:      fmt.Sprint(evidence["id"]),
		Comment:         "Trả lại phần còn dư",
	}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.SubmitSurplusHandling(ctx, fmt.Sprint(second["id"]), fixture.workerID, "Worker"); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `
		SELECT issued_quantity::float8, used_quantity::float8, surplus_quantity::float8, surplus_status
		FROM site_material_balances
		WHERE id=$1
	`, balanceID).Scan(&issued, &used, &surplus, &surplusStatus); err != nil {
		t.Fatal(err)
	}
	if issued != 4 || used != 4 || surplus != 0 || surplusStatus != "handled" {
		t.Fatalf("handled balance issued=%.3f used=%.3f surplus=%.3f status=%s, want 4/4/0/handled", issued, used, surplus, surplusStatus)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM stock_movements WHERE material_id=$1 AND movement_type='return_to_subcontractor' AND reference_type='site_surplus_handling'`, 1, materialID)
	assertCount(t, db, `SELECT COUNT(*) FROM work_item_activities WHERE work_item_id=$1 AND activity_type='system_event' AND metadata_json->>'status'='submitted' AND metadata_json ? 'surplus_handling_id'`, 2, fixture.itemID)
	assertCount(t, db, `SELECT COUNT(*) FROM audit_logs WHERE project_id=$1 AND event_type='site_surplus_handling_submitted'`, 2, fixture.projectID)

	_, err = service.UpdateSurplusHandling(ctx, fmt.Sprint(submitted["id"]), SurplusHandlingRequest{
		SiteBalanceID: balanceID,
		HandlingType:  "return_to_warehouse",
		Quantity:      &firstQty,
		Unit:          "m",
		WarehouseID:   warehouseID,
		Condition:     "usable",
	}, fixture.workerID, "Worker")
	assertValidationError(t, err)
}

func TestIncomingAcceptanceSubmitDoesNotCreateStockMovement(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")
	fixture := setupWorkItemFlowFixture(t, db, false)

	warehouseID := newID()
	materialID := newID()
	procurementRequestID := newID()
	procurementItemID := newID()
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM audit_logs WHERE project_id=$1 AND target_type='incoming_acceptance'`, fixture.projectID)
		_, _ = db.ExecContext(ctx, `DELETE FROM incoming_acceptance_records WHERE project_id=$1`, fixture.projectID)
		_, _ = db.ExecContext(ctx, `DELETE FROM procurement_requests WHERE id=$1`, procurementRequestID)
		_, _ = db.ExecContext(ctx, `DELETE FROM stock_movements WHERE material_id=$1`, materialID)
		_, _ = db.ExecContext(ctx, `DELETE FROM materials WHERE id=$1`, materialID)
		_, _ = db.ExecContext(ctx, `DELETE FROM warehouses WHERE id=$1`, warehouseID)
	})
	mustExec(t, db, `INSERT INTO warehouses (id, name, location, type, active) VALUES ($1,'Incoming Warehouse','Gate A','central',true)`, warehouseID)
	mustExec(t, db, `INSERT INTO materials (id, code, name, unit, category, active) VALUES ($1,$2,'Incoming Cable','m','Electrical',true)`, materialID, "IN-"+materialID[:8])
	mustExec(t, db, `
		INSERT INTO procurement_requests (id, project_id, request_code, title, requested_by, status)
		VALUES ($1,$2,$3,'Incoming acceptance procurement',$4,'ordered')
	`, procurementRequestID, fixture.projectID, "IAR-"+procurementRequestID[:8], fixture.workerID)
	mustExec(t, db, `
		INSERT INTO procurement_request_items (id, request_id, material_id, quantity_planned, quantity_requested, unit, estimated_unit_price, delivery_status, approval_status)
		VALUES ($1,$2,$3,10,10,'m',100,'pending','approved')
	`, procurementItemID, procurementRequestID, materialID)

	evidence, err := service.AddWorkItemEvidence(ctx, fixture.itemID, WorkItemEvidenceRequest{
		EvidenceType: "photo",
		Name:         "Incoming material defect",
		FileURL:      "/evidence/incoming-material.jpg",
		Visibility:   "project",
	}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}

	delivered := 10.0
	accepted := 8.0
	rejected := 2.0
	draft, err := service.CreateIncomingAcceptance(ctx, fixture.projectID, IncomingAcceptanceRequest{
		WorkItemID:           fixture.itemID,
		ProcurementRequestID: procurementRequestID,
		WarehouseID:          warehouseID,
		SourceType:           "procurement",
		SupplierName:         "Test Supplier",
		DeliveryReference:    "DN-001",
		ReceivedAt:           "2026-06-20",
		Comment:              "Nghiệm thu đầu vào có vật tư lỗi",
		Items: []IncomingAcceptanceItemRequest{{
			ItemIndex:                0,
			MaterialID:               materialID,
			ProcurementRequestItemID: procurementItemID,
			DeliveredQuantity:        &delivered,
			AcceptedQuantity:         &accepted,
			RejectedQuantity:         &rejected,
			Unit:                     "m",
			Condition:                "mixed",
			DefectNote:               "Một phần vật tư móp méo",
			EvidenceID:               fmt.Sprint(evidence["id"]),
			Decision:                 "conditional_acceptance",
		}},
	}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	if draft["status"] != "draft" {
		t.Fatalf("draft status = %v, want draft", draft["status"])
	}
	assertCount(t, db, `SELECT COUNT(*) FROM stock_movements WHERE material_id=$1`, 0, materialID)

	submitted, err := service.SubmitIncomingAcceptance(ctx, fmt.Sprint(draft["id"]), fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	if submitted["status"] != "submitted" || submitted["result"] != "conditional_pass" {
		t.Fatalf("submitted incoming acceptance = %#v, want submitted conditional_pass", submitted)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM stock_movements WHERE material_id=$1`, 0, materialID)
	assertCount(t, db, `SELECT COUNT(*) FROM incoming_acceptance_items WHERE incoming_acceptance_id=$1 AND accepted_quantity=8 AND rejected_quantity=2`, 1, draft["id"])
	assertCount(t, db, `SELECT COUNT(*) FROM work_item_activities WHERE work_item_id=$1 AND activity_type='system_event' AND metadata_json->>'status'='submitted' AND metadata_json ? 'incoming_acceptance_id'`, 1, fixture.itemID)
	assertCount(t, db, `SELECT COUNT(*) FROM audit_logs WHERE project_id=$1 AND event_type='incoming_acceptance_submitted'`, 1, fixture.projectID)

	_, err = service.UpdateIncomingAcceptance(ctx, fmt.Sprint(submitted["id"]), IncomingAcceptanceRequest{
		WorkItemID: fixture.itemID,
		SourceType: "supplier",
	}, fixture.workerID, "Worker")
	assertValidationError(t, err)
}

func TestOutgoingAcceptanceSubmitLocksAndFeedsReworkAndHandover(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")
	fixture := setupWorkItemFlowFixture(t, db, false)

	handoverID := newID()
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM handover_records WHERE project_id=$1`, fixture.projectID)
		_, _ = db.ExecContext(ctx, `DELETE FROM rework_requests WHERE project_id=$1`, fixture.projectID)
		_, _ = db.ExecContext(ctx, `DELETE FROM issues WHERE project_id=$1`, fixture.projectID)
		_, _ = db.ExecContext(ctx, `DELETE FROM acceptance_records WHERE project_id=$1`, fixture.projectID)
		_, _ = db.ExecContext(ctx, `DELETE FROM evidence_records WHERE project_id=$1`, fixture.projectID)
		_, _ = db.ExecContext(ctx, `DELETE FROM work_item_activities WHERE project_id=$1`, fixture.projectID)
		_, _ = db.ExecContext(ctx, `DELETE FROM audit_logs WHERE project_id=$1`, fixture.projectID)
	})

	evidence, err := service.AddWorkItemEvidence(ctx, fixture.itemID, WorkItemEvidenceRequest{
		EvidenceType: "photo",
		Name:         "Outgoing defect photo",
		FileURL:      "/evidence/outgoing-defect.jpg",
		Visibility:   "project",
	}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	evidenceID := fmt.Sprint(evidence["id"])

	draft, err := service.CreateAcceptance(ctx, fixture.projectID, AcceptanceRequest{
		WorkItemID:               fixture.itemID,
		Type:                     "partial",
		Title:                    "Nghiệm thu đầu ra có punch list",
		AcceptanceDate:           "2026-06-20",
		ClientRepresentative:     "Owner rep",
		ContractorRepresentative: "Contractor rep",
		SignatureStatus:          "waived",
		DocumentStatus:           "waived",
		Notes:                    "Có lỗi cần sửa trước khi bàn giao",
		Checklist: []AcceptanceChecklistItemRequest{
			{ItemIndex: 0, Title: "Bề mặt hoàn thiện", Decision: "pass"},
			{ItemIndex: 1, Title: "Vệ sinh khu vực", Decision: "fail", Severity: "high", EvidenceID: evidenceID, RequiredAction: "Dọn lại khu vực nghiệm thu"},
		},
	}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	if draft["status"] != "draft" {
		t.Fatalf("draft status = %v, want draft", draft["status"])
	}

	submitted, err := service.SubmitAcceptance(ctx, fmt.Sprint(draft["id"]), fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	if submitted["status"] != "submitted" {
		t.Fatalf("submitted status = %v, want submitted", submitted["status"])
	}
	assertCount(t, db, `SELECT COUNT(*) FROM work_item_activities WHERE work_item_id=$1 AND activity_type='system_event' AND metadata_json->>'status'='submitted' AND metadata_json ? 'acceptance_id'`, 1, fixture.itemID)
	assertCount(t, db, `SELECT COUNT(*) FROM audit_logs WHERE project_id=$1 AND event_type='outgoing_acceptance_submitted'`, 1, fixture.projectID)

	_, err = service.UpdateAcceptance(ctx, fmt.Sprint(submitted["id"]), AcceptanceRequest{
		WorkItemID: fixture.itemID,
		Title:      "Không được sửa sau submit",
	}, fixture.workerID, "Worker")
	assertValidationError(t, err)

	_, err = service.AcceptanceAction(ctx, fmt.Sprint(submitted["id"]), "pass", ActionRequest{Comment: "Không hợp lệ vì còn fail"}, fixture.workerID, "Worker")
	assertValidationError(t, err)

	failed, err := service.AcceptanceAction(ctx, fmt.Sprint(submitted["id"]), "fail", ActionRequest{Comment: "Không đạt"}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	if failed["status"] != "completed" || failed["result"] != "fail" {
		t.Fatalf("failed acceptance = %#v, want completed/fail", failed)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM issues WHERE project_id=$1 AND requires_rework`, 1, fixture.projectID)

	rework, err := service.CreateReworkFromAcceptance(ctx, fmt.Sprint(failed["id"]), fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	if rework["status"] != "approved" {
		t.Fatalf("rework status = %v, want approved", rework["status"])
	}
	_, err = service.CreateReworkFromAcceptance(ctx, fmt.Sprint(failed["id"]), fixture.workerID, "Worker")
	assertValidationError(t, err)

	finalDraft, err := service.CreateAcceptance(ctx, fixture.projectID, AcceptanceRequest{
		WorkItemID:               fixture.itemID,
		Type:                     "final",
		Title:                    "Nghiệm thu toàn phần đạt",
		AcceptanceDate:           "2026-06-21",
		ClientRepresentative:     "Owner rep",
		ContractorRepresentative: "Contractor rep",
		SignatureStatus:          "waived",
		DocumentStatus:           "waived",
		Checklist: []AcceptanceChecklistItemRequest{{
			ItemIndex: 0,
			Title:     "Toàn bộ hạng mục đạt",
			Decision:  "pass",
		}},
	}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	finalSubmitted, err := service.SubmitAcceptance(ctx, fmt.Sprint(finalDraft["id"]), fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	finalPassed, err := service.AcceptanceAction(ctx, fmt.Sprint(finalSubmitted["id"]), "pass", ActionRequest{Comment: "Đạt bàn giao"}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	if finalPassed["type"] != "final" || finalPassed["result"] != "pass" {
		t.Fatalf("final acceptance = %#v, want final/pass", finalPassed)
	}

	mustExec(t, db, `UPDATE issues SET status='closed' WHERE project_id=$1`, fixture.projectID)
	mustExec(t, db, `
		INSERT INTO handover_records
			(id, project_id, status, final_acceptance_id, documents_ready, warehouse_closed, payables_recorded, receivables_recorded, created_by)
		VALUES ($1,$2,'draft',$3,true,true,true,true,$4)
	`, handoverID, fixture.projectID, finalPassed["id"], fixture.workerID)
	check, err := repo.HandoverCheck(ctx, handoverID)
	if err != nil {
		t.Fatal(err)
	}
	if ready, ok := check["ready"].(bool); !ok || !ready {
		t.Fatalf("handover ready = %#v, blockers = %#v, want ready", check["ready"], check["blockers"])
	}
	assertCount(t, db, `SELECT COUNT(*) FROM acceptance_records WHERE project_id=$1 AND type='final' AND status='completed' AND result='pass'`, 1, fixture.projectID)
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

func assertBearerStatus(t *testing.T, app *fiber.App, path string, token string, want int) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != want {
		t.Fatalf("status = %d, want %d", resp.StatusCode, want)
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

func hasWarningCode(warnings []Record, code string) bool {
	for _, warning := range warnings {
		if warning["code"] == code {
			return true
		}
	}
	return false
}
