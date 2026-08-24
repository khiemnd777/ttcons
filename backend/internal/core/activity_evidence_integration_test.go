package core

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

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

func TestProjectActivityFilterNormalizationAndCursor(t *testing.T) {
	cursor := encodeActivityCursor(Record{
		"id":         "10000000-0000-0000-0000-000000000001",
		"created_at": "2026-07-11T08:30:00Z",
	})
	if cursor == "" {
		t.Fatal("expected activity cursor")
	}
	filter, err := normalizeProjectActivityFilter(ProjectActivityFilter{
		Kind:         " Discussion ",
		Attention:    " Questions ",
		ActivityType: " Question ",
		Search:       "  định mức  ",
		Cursor:       cursor,
		Limit:        500,
	})
	if err != nil {
		t.Fatal(err)
	}
	if filter.Kind != "discussion" || filter.Attention != "questions" || filter.ActivityType != "question" || filter.Search != "định mức" {
		t.Fatalf("normalized filter = %#v", filter)
	}
	if filter.Limit != 30 || filter.CursorCreatedAt != "2026-07-11T08:30:00Z" || filter.CursorID != "10000000-0000-0000-0000-000000000001" {
		t.Fatalf("normalized paging = %#v", filter)
	}
	if _, err := normalizeProjectActivityFilter(ProjectActivityFilter{Kind: "chat"}); err == nil {
		t.Fatal("expected invalid kind to fail")
	}
	if _, err := normalizeProjectActivityFilter(ProjectActivityFilter{Cursor: "not-a-cursor"}); err == nil {
		t.Fatal("expected invalid cursor to fail")
	}
}

func TestProjectEvidenceFilterNormalizationAndCursor(t *testing.T) {
	cursor := encodeEvidenceCursor(Record{
		"id":              "10000000-0000-0000-0000-000000000001",
		"created_at":      "2026-07-11T08:30:00Z",
		"review_priority": 0,
	})
	if cursor == "" {
		t.Fatal("expected evidence cursor")
	}
	filter, err := normalizeProjectEvidenceFilter(ProjectEvidenceFilter{
		EvidenceType: " Document ",
		Status:       " Pending ",
		DateFrom:     "2026-07-01",
		DateTo:       "2026-07-11",
		Search:       "  biên bản  ",
		Cursor:       cursor,
		Limit:        500,
	})
	if err != nil {
		t.Fatal(err)
	}
	if filter.EvidenceType != "document" || filter.Status != "waiting" || filter.Search != "biên bản" {
		t.Fatalf("normalized evidence filter = %#v", filter)
	}
	if filter.Limit != 25 || filter.CursorPriority != 0 || filter.CursorCreatedAt != "2026-07-11T08:30:00Z" || filter.CursorID != "10000000-0000-0000-0000-000000000001" {
		t.Fatalf("normalized evidence paging = %#v", filter)
	}
	if _, err := normalizeProjectEvidenceFilter(ProjectEvidenceFilter{Status: "archived"}); err == nil {
		t.Fatal("expected invalid evidence status to fail")
	}
	if _, err := normalizeProjectEvidenceFilter(ProjectEvidenceFilter{DateFrom: "2026-07-12", DateTo: "2026-07-11"}); err == nil {
		t.Fatal("expected invalid evidence date range to fail")
	}
	if _, err := normalizeProjectEvidenceFilter(ProjectEvidenceFilter{Cursor: "not-a-cursor"}); err == nil {
		t.Fatal("expected invalid evidence cursor to fail")
	}
}

func TestProjectEvidencePageScopeHistoryAndReviewCapability(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")
	fixture := setupWorkItemFlowFixture(t, db, false)
	operationViewID := ensurePermission(t, db, "operation.view", "Xem thi công", "operation")
	pmRoleID := ensureRole(t, db, "pm", "PM")
	mustExec(t, db, `INSERT INTO role_permissions (role_id, permission_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`, pmRoleID, operationViewID)

	submittedID := newID()
	acceptedID := newID()
	rejectedID := newID()
	cancelledItemID := newID()
	mustExec(t, db, `
		INSERT INTO evidence_records
			(id, work_item_id, project_id, evidence_type, name, mime_type, visibility, status, submitted_by, created_at)
		VALUES
			($1,$4,$5,'document','Biên bản kiểm tra','application/pdf','project','submitted',$6,now() - INTERVAL '1 hour'),
			($2,$4,$5,'photo','Ảnh hoàn thiện','image/jpeg','project','accepted',$6,now() - INTERVAL '3 hours'),
			($3,$4,$5,'document','Hồ sơ bị từ chối','application/pdf','project','rejected',$6,now() - INTERVAL '2 hours')
	`, submittedID, acceptedID, rejectedID, fixture.itemID, fixture.projectID, fixture.workerID)
	mustExec(t, db, `
		INSERT INTO work_items
			(id, project_id, source_type, source_id, title, work_type, status, role_ids, created_by, updated_by)
		VALUES ($1,$2,'issue',$3,'Công việc đã hủy','task','cancelled','["PM"]'::jsonb,$4,$4)
	`, cancelledItemID, fixture.projectID, newID(), fixture.pmID)
	mustExec(t, db, `
		INSERT INTO evidence_records
			(work_item_id, project_id, evidence_type, name, mime_type, visibility, status, submitted_by)
		VALUES ($1,$2,'document','Không được hiển thị','application/pdf','project','accepted',$3)
	`, cancelledItemID, fixture.projectID, fixture.workerID)

	page, err := service.ProjectEvidencePage(ctx, fixture.projectID, ProjectEvidenceFilter{Limit: 2}, fixture.pmID)
	if err != nil {
		t.Fatal(err)
	}
	items, _ := page["items"].([]Record)
	if len(items) != 2 || !boolValue(page["has_more"]) || cleanID(page["next_cursor"]) == "" {
		t.Fatalf("evidence first page = %#v", page)
	}
	if cleanID(items[0]["id"]) != submittedID || !boolValue(items[0]["can_review"]) {
		t.Fatalf("waiting evidence priority/capability = %#v", items[0])
	}
	summary := recordValue(page["summary"])
	if number(summary["total"]) != 3 || number(summary["waiting"]) != 1 || number(summary["accepted"]) != 1 || number(summary["rejected"]) != 1 {
		t.Fatalf("evidence summary = %#v", summary)
	}
	if number(page["filtered_total"]) != 3 {
		t.Fatalf("filtered total = %#v", page["filtered_total"])
	}

	next, err := service.ProjectEvidencePage(ctx, fixture.projectID, ProjectEvidenceFilter{Limit: 2, Cursor: fmt.Sprint(page["next_cursor"])}, fixture.pmID)
	if err != nil {
		t.Fatal(err)
	}
	nextItems, _ := next["items"].([]Record)
	if len(nextItems) != 1 || cleanID(nextItems[0]["id"]) != acceptedID {
		t.Fatalf("evidence next page = %#v", next)
	}

	rejected, err := service.ProjectEvidencePage(ctx, fixture.projectID, ProjectEvidenceFilter{Status: "rejected", Search: "bị từ chối"}, fixture.pmID)
	if err != nil {
		t.Fatal(err)
	}
	rejectedItems, _ := rejected["items"].([]Record)
	if len(rejectedItems) != 1 || cleanID(rejectedItems[0]["id"]) != rejectedID || boolValue(rejectedItems[0]["can_review"]) {
		t.Fatalf("rejected evidence history = %#v", rejected)
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
	mustExec(t, db, `INSERT INTO project_members (project_id, staff_id, role_ids, active) VALUES ($1,$2,'["Batch3 Reporter","Batch3 Reporter"]'::jsonb,true),($1,$3,'["Batch3 PM"]'::jsonb,true)`, projectID, actorStaffID, pmStaffID)
	mustExec(t, db, `INSERT INTO user_roles (user_id, role_id) VALUES ($1,$2),($3,$4),($5,$4)`, actorUserID, reporterRoleID, pmUserID, pmRoleID, outsideUserID)

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
	attentionSummary, err := service.MyActivityAttentionSummary(ctx, "", pmUserID)
	if err != nil {
		t.Fatal(err)
	}
	if attentionSummary.Count != 1 || attentionSummary.HasMore {
		t.Fatalf("attention summary = %#v", attentionSummary)
	}
	projectAttentionSummary, err := service.MyActivityAttentionSummary(ctx, projectID, pmUserID)
	if err != nil {
		t.Fatal(err)
	}
	if projectAttentionSummary != attentionSummary {
		t.Fatalf("project attention summary = %#v, want %#v", projectAttentionSummary, attentionSummary)
	}
	outsideAttention, err := service.MyActivityAttention(ctx, ActivityAttentionFilter{Queue: "mentions"}, outsideUserID)
	if err != nil {
		t.Fatal(err)
	}
	if len(outsideAttention) != 0 {
		t.Fatalf("outside attention = %#v", outsideAttention)
	}
	page, err := service.ProjectActivityPage(ctx, projectID, ProjectActivityFilter{
		Attention: "questions",
		Search:    "PM xác nhận",
		Limit:     10,
	}, pmUserID)
	if err != nil {
		t.Fatal(err)
	}
	pageItems, _ := page["items"].([]Record)
	if len(pageItems) != 1 || pageItems[0]["id"] != activityID {
		t.Fatalf("activity page = %#v", page)
	}
	summary, err := repo.ProjectActivitySummary(ctx, projectID, workItemAccessProfile{UserID: pmUserID, StaffID: pmStaffID})
	if err != nil {
		t.Fatal(err)
	}
	if number(summary["questions"]) != 1 || number(summary["unread"]) != 1 {
		t.Fatalf("activity summary = %#v", summary)
	}
	options, err := service.ProjectActivityOptions(ctx, projectID, actorUserID)
	if err != nil {
		t.Fatal(err)
	}
	if len(anySlice(options["members"])) != 2 {
		t.Fatalf("activity options = %#v", options)
	}
	roles, ok := options["roles"].([]Record)
	if !ok || len(roles) != 2 {
		t.Fatalf("activity option roles = %#v", options["roles"])
	}
	for i, expected := range []string{"Batch3 PM", "Batch3 Reporter"} {
		if len(roles[i]) != 2 || roles[i]["id"] != expected || roles[i]["label"] != expected {
			t.Fatalf("activity option role %d = %#v", i, roles[i])
		}
	}
	if _, err := service.ResolveActivity(ctx, activityID, ActivityResolutionRequest{}, pmUserID, "Mention PM"); err != nil {
		t.Fatal(err)
	}
	questionAttention, err := service.MyActivityAttention(ctx, ActivityAttentionFilter{Queue: "questions"}, pmUserID)
	if err != nil {
		t.Fatal(err)
	}
	if len(questionAttention) != 0 {
		t.Fatalf("questions after resolve = %#v", questionAttention)
	}

	readResult, err := service.MarkProjectActivityRead(ctx, projectID, pmUserID)
	if err != nil {
		t.Fatal(err)
	}
	if number(readResult["marked_count"]) != 1 {
		t.Fatalf("mark all result = %#v", readResult)
	}
	unread, err := service.MyActivityAttention(ctx, ActivityAttentionFilter{Queue: "unread"}, pmUserID)
	if err != nil {
		t.Fatal(err)
	}
	if len(unread) != 0 {
		t.Fatalf("unread after mark read = %#v", unread)
	}

	mustExec(t, db, `
		INSERT INTO work_item_activities(project_id,scope_type,scope_id,actor_id,activity_type,comment,visibility,metadata_json,created_at)
		SELECT $1,'project',$1,$2,'question','Câu hỏi giới hạn ' || sequence_no,'project','{}'::jsonb,now() + (sequence_no * INTERVAL '1 millisecond')
		FROM generate_series(1,121) sequence_no
	`, projectID, actorUserID)
	cappedSummary, err := service.MyActivityAttentionSummary(ctx, projectID, pmUserID)
	if err != nil {
		t.Fatal(err)
	}
	if cappedSummary.Count != 120 || !cappedSummary.HasMore {
		t.Fatalf("capped attention summary = %#v", cappedSummary)
	}
	cappedItems, err := service.MyActivityAttention(ctx, ActivityAttentionFilter{ProjectID: projectID, Limit: 120}, pmUserID)
	if err != nil {
		t.Fatal(err)
	}
	if len(cappedItems) != 120 {
		t.Fatalf("capped attention items = %d, want 120", len(cappedItems))
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
	defer file.Reader.Close()
	downloadedBytes, err := io.ReadAll(file.Reader)
	if err != nil {
		t.Fatal(err)
	}
	if file.MIMEType != "image/png" || file.Size != int64(len(content)) || !bytes.Equal(downloadedBytes, content) {
		t.Fatalf("file result metadata/bytes mismatch: mime=%q size=%d", file.MIMEType, file.Size)
	}
	if !strings.HasSuffix(file.Filename, ".png") {
		t.Fatalf("filename = %q, want .png suffix", file.Filename)
	}

	for _, officeUpload := range []struct {
		filename string
		content  []byte
		mimeType string
	}{
		{filename: "survey.docx", content: openXMLTestFile(t, "word/document.xml", false), mimeType: "application/vnd.openxmlformats-officedocument.wordprocessingml.document"},
		{filename: "measurements.xlsx", content: openXMLTestFile(t, "xl/workbook.xml", false), mimeType: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"},
	} {
		officeEvidence, uploadErr := service.AddWorkItemEvidenceUpload(ctx, fixture.itemID, WorkItemEvidenceRequest{
			EvidenceType: "document",
			Name:         officeUpload.filename,
			Visibility:   "project",
		}, testMultipartFileHeader(t, "file", officeUpload.filename, officeUpload.content), fixture.workerID, "Worker")
		if uploadErr != nil {
			t.Fatalf("upload %s: %v", officeUpload.filename, uploadErr)
		}
		if got := fmt.Sprint(officeEvidence["mime_type"]); got != officeUpload.mimeType {
			t.Fatalf("%s MIME = %q, want %q", officeUpload.filename, got, officeUpload.mimeType)
		}
		officeSum := sha256.Sum256(officeUpload.content)
		if got := fmt.Sprint(officeEvidence["checksum_sha256"]); got != hex.EncodeToString(officeSum[:]) {
			t.Fatalf("%s checksum = %s, want %s", officeUpload.filename, got, hex.EncodeToString(officeSum[:]))
		}
		officeFile, fileErr := service.EvidenceFile(ctx, fmt.Sprint(officeEvidence["id"]), fixture.workerID)
		if fileErr != nil {
			t.Fatalf("open %s: %v", officeUpload.filename, fileErr)
		}
		officeBytes, readErr := io.ReadAll(officeFile.Reader)
		_ = officeFile.Reader.Close()
		if readErr != nil || officeFile.MIMEType != officeUpload.mimeType || !bytes.Equal(officeBytes, officeUpload.content) {
			t.Fatalf("download %s mismatch: mime=%q read_error=%v", officeUpload.filename, officeFile.MIMEType, readErr)
		}
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

func TestActivityReplyAndReactionScope(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	service := NewService(NewRepository(db), "test-secret")
	fixture := setupWorkItemFlowFixture(t, db, false)
	operationViewID := ensurePermission(t, db, "operation.view", "Xem thi công", "operation")
	pmRoleID := ensureRole(t, db, "pm", "PM")
	mustExec(t, db, `INSERT INTO role_permissions (role_id, permission_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`, pmRoleID, operationViewID)

	root, err := service.AddProjectActivity(ctx, fixture.projectID, WorkItemActivityRequest{
		ActivityType: "question",
		Comment:      "Cần kiểm tra lại khối lượng",
		Visibility:   "project",
		ScopeType:    "project",
		ScopeID:      fixture.projectID,
	}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	rootID := cleanID(root["id"])
	otherProjectID := newID()
	var workerStaffID string
	if err := db.QueryRowContext(ctx, `SELECT id::text FROM staff WHERE user_id=$1`, fixture.workerID).Scan(&workerStaffID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = db.ExecContext(ctx, `DELETE FROM projects WHERE id=$1`, otherProjectID) })
	mustExec(t, db, `INSERT INTO projects (id, name, code, client_name, status) VALUES ($1,'Reply Other',$2,'Client','planning')`, otherProjectID, "RPL-"+otherProjectID[:8])
	mustExec(t, db, `INSERT INTO project_members (project_id, staff_id, role_ids, active) VALUES ($1,$2,'["Đội thi công"]'::jsonb,true)`, otherProjectID, workerStaffID)
	otherRoot, err := service.AddProjectActivity(ctx, otherProjectID, WorkItemActivityRequest{ActivityType: "comment", Comment: "Khác công trình"}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.AddProjectActivity(ctx, fixture.projectID, WorkItemActivityRequest{Comment: "Không được nối chéo", ParentActivityID: cleanID(otherRoot["id"])}, fixture.workerID, "Worker")
	assertValidationError(t, err)

	reply, err := service.AddProjectActivity(ctx, fixture.projectID, WorkItemActivityRequest{
		ActivityType:     "blocker",
		Comment:          "Đã cập nhật BOQ để kiểm tra",
		Visibility:       "client",
		ScopeType:        "work_item",
		ScopeID:          fixture.itemID,
		WorkItemID:       fixture.itemID,
		ParentActivityID: rootID,
	}, fixture.pmID, "PM")
	if err != nil {
		t.Fatal(err)
	}
	if reply["activity_type"] != "comment" || reply["visibility"] != "project" || reply["scope_type"] != "project" || cleanID(reply["scope_id"]) != fixture.projectID || cleanID(reply["parent_activity_id"]) != rootID {
		t.Fatalf("reply inheritance = %#v", reply)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM activity_mentions WHERE activity_id=$1 AND recipient_user_id=$2`, 1, reply["id"], fixture.workerID)
	assertCount(t, db, `SELECT COUNT(*) FROM activity_read_states WHERE activity_id=$1 AND user_id=$2 AND read_at IS NULL`, 1, reply["id"], fixture.workerID)

	if _, err := service.SetActivityReaction(ctx, rootID, ActivityReactionRequest{Reaction: "👍", Active: true}, fixture.workerID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.SetActivityReaction(ctx, rootID, ActivityReactionRequest{Reaction: "👍", Active: true}, fixture.workerID); err != nil {
		t.Fatal(err)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM activity_reactions WHERE activity_id=$1 AND user_id=$2 AND reaction='👍' AND active`, 1, rootID, fixture.workerID)
	if _, err := service.SetActivityReaction(ctx, rootID, ActivityReactionRequest{Reaction: "✅", Active: true}, fixture.workerID); err == nil {
		t.Fatal("expected invalid reaction to fail")
	}
	systemActivityID := newID()
	mustExec(t, db, `
		INSERT INTO work_item_activities (id, project_id, scope_type, scope_id, actor_id, activity_type, comment, visibility)
		VALUES ($1,$2,'project',$2,$3,'system_event','Sự kiện kiểm thử','project')
	`, systemActivityID, fixture.projectID, fixture.workerID)
	if _, err := service.SetActivityReaction(ctx, systemActivityID, ActivityReactionRequest{Reaction: "👍", Active: true}, fixture.workerID); err == nil {
		t.Fatal("expected system event reaction to fail")
	}

	page, err := service.ProjectActivityPage(ctx, fixture.projectID, ProjectActivityFilter{Kind: "discussion", Limit: 20}, fixture.workerID)
	if err != nil {
		t.Fatal(err)
	}
	items, _ := page["items"].([]Record)
	var rootRow, replyRow Record
	for _, item := range items {
		switch cleanID(item["id"]) {
		case rootID:
			rootRow = item
		case cleanID(reply["id"]):
			replyRow = item
		}
	}
	if rootRow == nil || number(rootRow["reply_count"]) != 1 || len(anySlice(rootRow["reactions"])) != 1 {
		t.Fatalf("root conversation fields = %#v", rootRow)
	}
	if replyRow == nil || cleanID(replyRow["parent_activity_id"]) != rootID || replyRow["reply_to"] == nil {
		t.Fatalf("reply preview = %#v", replyRow)
	}

	if _, err := service.SetActivityReaction(ctx, rootID, ActivityReactionRequest{Reaction: "👍", Active: false}, fixture.workerID); err != nil {
		t.Fatal(err)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM activity_reactions WHERE activity_id=$1 AND user_id=$2 AND reaction='👍' AND active`, 0, rootID, fixture.workerID)
}

func TestActivityMessageEditAndWithdrawScope(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	service := NewService(NewRepository(db), "test-secret")
	fixture := setupWorkItemFlowFixture(t, db, false)
	operationViewID := ensurePermission(t, db, "operation.view", "Xem thi công", "operation")
	pmRoleID := ensureRole(t, db, "pm", "PM")
	mustExec(t, db, `INSERT INTO role_permissions (role_id, permission_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`, pmRoleID, operationViewID)

	message, err := service.AddProjectActivity(ctx, fixture.projectID, WorkItemActivityRequest{
		ActivityType: "comment",
		Comment:      "Khối lượng ban đầu",
		Visibility:   "project",
		ScopeType:    "project",
		ScopeID:      fixture.projectID,
		Mentions:     []ActivityMentionRequest{{Type: "user", ID: fixture.pmID}},
	}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	messageID := cleanID(message["id"])
	if _, err := service.UpdateActivityMessage(ctx, messageID, ActivityMessageUpdateRequest{Comment: "Không được sửa thay"}, fixture.pmID, "PM"); err == nil {
		t.Fatal("expected non-author edit to fail")
	}
	updated, err := service.UpdateActivityMessage(ctx, messageID, ActivityMessageUpdateRequest{Comment: "Khối lượng đã cập nhật"}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	if updated["comment"] != "Khối lượng đã cập nhật" || number(updated["message_revision_no"]) != 1 || cleanID(updated["edited_at"]) == "" {
		t.Fatalf("updated message = %#v", updated)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM activity_message_revisions WHERE activity_id=$1 AND revision_no=1 AND action='edited' AND previous_comment='Khối lượng ban đầu' AND new_comment='Khối lượng đã cập nhật'`, 1, messageID)

	typed, err := service.AddProjectActivity(ctx, fixture.projectID, WorkItemActivityRequest{ActivityType: "question", Comment: "Câu hỏi bất biến"}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.UpdateActivityMessage(ctx, cleanID(typed["id"]), ActivityMessageUpdateRequest{Comment: "Không được sửa"}, fixture.workerID, "Worker"); err == nil {
		t.Fatal("expected typed operational activity edit to fail")
	}
	if _, err := service.AddProjectActivity(ctx, fixture.projectID, WorkItemActivityRequest{ActivityType: "decision", Comment: "Quyết định kiểm thử"}, fixture.workerID, "Worker"); err != nil {
		t.Fatal(err)
	}

	expired, err := service.AddProjectActivity(ctx, fixture.projectID, WorkItemActivityRequest{ActivityType: "comment", Comment: "Tin đã hết hạn"}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	mustExec(t, db, `UPDATE work_item_activities SET created_at=now() - INTERVAL '16 minutes' WHERE id=$1`, expired["id"])
	if _, err := service.UpdateActivityMessage(ctx, cleanID(expired["id"]), ActivityMessageUpdateRequest{Comment: "Không được sửa trễ"}, fixture.workerID, "Worker"); err == nil {
		t.Fatal("expected expired message edit to fail")
	}

	reply, err := service.AddProjectActivity(ctx, fixture.projectID, WorkItemActivityRequest{Comment: "Phản hồi vẫn được giữ", ParentActivityID: messageID}, fixture.pmID, "PM")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.SetActivityReaction(ctx, messageID, ActivityReactionRequest{Reaction: "👍", Active: true}, fixture.pmID); err != nil {
		t.Fatal(err)
	}
	withdrawn, err := service.WithdrawActivityMessage(ctx, messageID, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	if withdrawn["comment"] != nil || cleanID(withdrawn["withdrawn_at"]) == "" || number(withdrawn["message_revision_no"]) != 2 {
		t.Fatalf("withdrawn message = %#v", withdrawn)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM activity_message_revisions WHERE activity_id=$1 AND revision_no=2 AND action='withdrawn' AND previous_comment='Khối lượng đã cập nhật' AND new_comment IS NULL`, 1, messageID)
	assertCount(t, db, `SELECT COUNT(*) FROM audit_logs WHERE target_id=$1 AND event_type IN ('project_activity_comment_edited','project_activity_comment_withdrawn')`, 2, messageID)
	assertCount(t, db, `SELECT COUNT(*) FROM activity_reactions WHERE activity_id=$1 AND active`, 0, messageID)
	assertCount(t, db, `SELECT COUNT(*) FROM activity_read_states WHERE activity_id=$1 AND user_id=$2 AND read_at IS NULL`, 0, messageID, fixture.pmID)
	assertCount(t, db, `SELECT COUNT(*) FROM work_item_activities WHERE parent_activity_id=$1`, 1, messageID)

	if _, err := service.SetActivityReaction(ctx, messageID, ActivityReactionRequest{Reaction: "👍", Active: true}, fixture.pmID); err == nil {
		t.Fatal("expected reaction on withdrawn message to fail")
	}
	if _, err := service.AddProjectActivity(ctx, fixture.projectID, WorkItemActivityRequest{Comment: "Không được phản hồi thêm", ParentActivityID: messageID}, fixture.pmID, "PM"); err == nil {
		t.Fatal("expected reply to withdrawn message to fail")
	}

	page, err := service.ProjectActivityPage(ctx, fixture.projectID, ProjectActivityFilter{Kind: "discussion", Limit: 20}, fixture.pmID)
	if err != nil {
		t.Fatal(err)
	}
	items, _ := page["items"].([]Record)
	summary := recordValue(page["summary"])
	var expectedDecisionCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM work_item_activities WHERE project_id=$1 AND activity_type='decision'`, fixture.projectID).Scan(&expectedDecisionCount); err != nil {
		t.Fatal(err)
	}
	if number(summary["decisions"]) != float64(expectedDecisionCount) {
		t.Fatalf("decision summary = %#v, want %d", summary["decisions"], expectedDecisionCount)
	}
	var messageRow, replyRow Record
	for _, item := range items {
		switch cleanID(item["id"]) {
		case messageID:
			messageRow = item
		case cleanID(reply["id"]):
			replyRow = item
		}
	}
	if messageRow == nil || cleanID(messageRow["withdrawn_at"]) == "" || boolValue(messageRow["can_edit"]) || boolValue(messageRow["can_withdraw"]) || len(anySlice(messageRow["reactions"])) != 0 {
		t.Fatalf("withdrawn conversation row = %#v", messageRow)
	}
	replyPreview := recordValue(replyRow["reply_to"])
	if !boolValue(replyPreview["withdrawn"]) || cleanID(replyPreview["comment"]) != "" {
		t.Fatalf("withdrawn reply preview = %#v", replyPreview)
	}
	attention, err := service.MyActivityAttention(ctx, ActivityAttentionFilter{ProjectID: fixture.projectID, Queue: "mentions", Limit: 20}, fixture.pmID)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range attention {
		if cleanID(item["id"]) == messageID {
			t.Fatal("withdrawn message must not remain in attention queue")
		}
	}
}
