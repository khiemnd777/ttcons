package core

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestConstructionReportCreateAndSubmitIsAtomicAndRecommendsProjectPM(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	service := NewService(NewRepository(db), "test-secret")
	fixture := setupWorkItemFlowFixture(t, db, false)

	submitPermissionID := ensurePermission(t, db, "work_report.submit", "Gửi báo cáo", "work_report")
	reviewPermissionID := ensurePermission(t, db, "work_report.review", "Xem xét báo cáo", "work_report")
	viewPermissionID := ensurePermission(t, db, "work_report.view", "Xem báo cáo", "work_report")
	var workerRoleID, pmRoleID, pmStaffID string
	if err := db.QueryRowContext(ctx, `SELECT role_id::text FROM user_roles WHERE user_id=$1 LIMIT 1`, fixture.workerID).Scan(&workerRoleID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT role_id::text FROM user_roles WHERE user_id=$1 LIMIT 1`, fixture.pmID).Scan(&pmRoleID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT id::text FROM staff WHERE user_id=$1 AND status <> 'deleted'`, fixture.pmID).Scan(&pmStaffID); err != nil {
		t.Fatal(err)
	}
	mustExec(t, db, `INSERT INTO role_permissions (role_id,permission_id) VALUES ($1,$2),($3,$4),($3,$5) ON CONFLICT DO NOTHING`, workerRoleID, submitPermissionID, pmRoleID, reviewPermissionID, viewPermissionID)
	mustExec(t, db, `UPDATE projects SET pm_staff_id=$2 WHERE id=$1`, fixture.projectID, pmStaffID)

	recipients, err := service.WorkReportRecipients(ctx, fixture.projectID, fixture.workerID)
	if err != nil {
		t.Fatal(err)
	}
	if len(recipients) == 0 || cleanID(recipients[0]["id"]) != fixture.pmID || !boolValue(recipients[0]["is_recommended"]) || fmt.Sprint(recipients[0]["recommendation_reason"]) != "project_pm" {
		t.Fatalf("recommended recipients = %#v, want project PM first", recipients)
	}

	evidence, err := service.AddWorkItemEvidence(ctx, fixture.itemID, WorkItemEvidenceRequest{
		EvidenceType: "photo", Name: "Ảnh gửi báo cáo", FileURL: "https://example.com/report-submit.jpg", Visibility: "project",
	}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	req := ConstructionDailyReportCreateRequest{
		ReportDate: "2026-08-25", Weather: "Nắng", ProgressPercent: 45,
		WorkItemID: fixture.itemID, DoneItems: []ConstructionDailyReportWorkItemRequest{{Title: "Hoàn thiện khu A"}},
		EvidenceIDs: []string{cleanID(evidence["id"])}, PrimaryEvidenceID: cleanID(evidence["id"]),
		Delivery: WorkReportDeliveryRequest{Mode: "submit", RecipientID: fixture.workerID},
	}
	if _, err := service.CreateDailyReport(ctx, fixture.projectID, req, fixture.workerID, "Worker"); err == nil {
		t.Fatal("self-recipient must fail the atomic create-and-submit mutation")
	}
	assertCount(t, db, `SELECT COUNT(*) FROM operation_daily_reports WHERE project_id=$1 AND report_date='2026-08-25'`, 0, fixture.projectID)
	assertCount(t, db, `SELECT COUNT(*) FROM work_items WHERE id=$1 AND status='ready' AND started_at IS NULL`, 1, fixture.itemID)
	assertCount(t, db, `SELECT COUNT(*) FROM work_item_activities WHERE work_item_id=$1 AND activity_type='started'`, 0, fixture.itemID)

	req.Delivery = WorkReportDeliveryRequest{Mode: "submit", RecipientID: fixture.pmID, Comment: "Báo cáo cuối ngày"}
	created, err := service.CreateDailyReport(ctx, fixture.projectID, req, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	if !boolValue(recordFromAny(created["work_item_transition"])["auto_started"]) {
		t.Fatalf("work item transition = %#v, want atomic auto start", created["work_item_transition"])
	}
	mustDispatchNotifications(t, service)
	delivery := recordFromAny(created["delivery_transition"])
	if !boolValue(delivery["submitted"]) || delivery["status"] != "submitted" || cleanID(delivery["recipient_id"]) != fixture.pmID || int(number(delivery["revision_no"])) != 1 {
		t.Fatalf("delivery transition = %#v, want submitted revision 1", delivery)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM work_reports WHERE source_id=$1 AND status='submitted' AND recipient_id=$2`, 1, created["id"], fixture.pmID)
	assertCount(t, db, `SELECT COUNT(*) FROM work_report_revisions revision JOIN work_reports report ON report.id=revision.report_id WHERE report.source_id=$1 AND revision.revision_no=1`, 1, created["id"])
	assertCount(t, db, `SELECT COUNT(*) FROM work_report_events event JOIN work_reports report ON report.id=event.report_id WHERE report.source_id=$1 AND event.event_type='submitted'`, 1, created["id"])
	assertCount(t, db, `SELECT COUNT(*) FROM user_notifications WHERE recipient_user_id=$1 AND event_key='work_report.submitted' AND source_id=$2`, 1, fixture.pmID, created["id"])
	assertCount(t, db, `SELECT COUNT(*) FROM audit_logs WHERE project_id=$1 AND target_id=$2 AND event_type='daily_report_created'`, 1, fixture.projectID, created["id"])
	assertCount(t, db, `SELECT COUNT(*) FROM audit_logs WHERE project_id=$1 AND event_type='work_report_submitted'`, 1, fixture.projectID)
	assertCount(t, db, `SELECT COUNT(*) FROM work_items WHERE id=$1 AND status='in_progress'`, 1, fixture.itemID)

	reportID := cleanID(delivery["work_report_id"])
	detail, err := service.WorkReportDetail(ctx, cleanID(created["id"]), fixture.pmID)
	if err != nil {
		t.Fatal(err)
	}
	revisions, _ := detail["revisions"].([]Record)
	if len(revisions) != 1 {
		t.Fatalf("detail revisions = %#v, want one submitted revision", detail["revisions"])
	}
	revisionEvidence, _ := revisions[0]["evidence_records"].([]Record)
	if len(revisionEvidence) != 1 || cleanID(revisionEvidence[0]["id"]) != cleanID(evidence["id"]) {
		t.Fatalf("revision evidence = %#v, want submitted evidence %s", revisions[0]["evidence_records"], cleanID(evidence["id"]))
	}
	unlinkedEvidence, err := service.AddWorkItemEvidence(ctx, fixture.itemID, WorkItemEvidenceRequest{
		EvidenceType: "photo", Name: "Ảnh thêm sau khi gửi", FileURL: "https://example.com/unlinked.jpg", Visibility: "project",
	}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.WorkReportEvidenceFile(ctx, reportID, 1, cleanID(unlinkedEvidence["id"]), fixture.pmID); err == nil {
		t.Fatal("report evidence endpoint must reject an evidence record outside the immutable revision snapshot")
	} else {
		var forbidden StatusError
		if !errors.As(err, &forbidden) || forbidden.Status != fiber.StatusForbidden {
			t.Fatalf("unlinked report evidence error = %T %v, want forbidden", err, err)
		}
	}
	if _, err := service.ReviewWorkReport(ctx, reportID, "request-changes", WorkReportReviewRequest{Comment: "Bổ sung ghi chú hiện trường"}, fixture.pmID, "PM"); err != nil {
		t.Fatal(err)
	}
	mustDispatchNotifications(t, service)
	assertCount(t, db, `SELECT COUNT(*) FROM user_notifications WHERE recipient_user_id=$1 AND event_key='work_report.needs_changes' AND source_id=$2`, 1, fixture.workerID, created["id"])
	req.DoneItems = []ConstructionDailyReportWorkItemRequest{{Title: "Hoàn thiện khu A và vệ sinh cuối ca"}}
	req.FieldNotes = "Đã bổ sung theo phản hồi"
	updated, err := service.UpdateDailyReport(ctx, cleanID(created["id"]), req, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	resubmission := recordFromAny(updated["delivery_transition"])
	if !boolValue(resubmission["submitted"]) || int(number(resubmission["revision_no"])) != 2 || resubmission["event_type"] != "resubmitted" {
		t.Fatalf("resubmission transition = %#v, want atomic revision 2", resubmission)
	}
	mustDispatchNotifications(t, service)
	assertCount(t, db, `SELECT COUNT(*) FROM work_report_revisions WHERE report_id=$1`, 2, reportID)
	assertCount(t, db, `SELECT COUNT(*) FROM work_report_events WHERE report_id=$1 AND event_type='resubmitted'`, 1, reportID)
	assertCount(t, db, `SELECT COUNT(*) FROM user_notifications WHERE recipient_user_id=$1 AND event_key='work_report.resubmitted' AND source_id=$2`, 1, fixture.pmID, created["id"])
	assertCount(t, db, `SELECT COUNT(*) FROM work_items WHERE id=$1 AND status='in_progress'`, 1, fixture.itemID)
}

func TestWorkReportSnapshotEvidenceIDsAreOrderedAndUnique(t *testing.T) {
	primary := "10000000-0000-4000-8000-000000000001"
	photo := "10000000-0000-4000-8000-000000000002"
	sitePhoto := "10000000-0000-4000-8000-000000000003"
	completed := "10000000-0000-4000-8000-000000000004"
	snapshot := Record{
		"primary_evidence_id": primary,
		"photos":              []any{photo, primary, "not-a-uuid"},
		"structured_json": Record{
			"site_photo_evidence_id": sitePhoto,
			"completed_items": []any{
				map[string]any{"evidence_id": completed},
				map[string]any{"evidence_id": photo},
			},
		},
	}

	got := workReportSnapshotEvidenceIDs(snapshot)
	want := []string{primary, photo, sitePhoto, completed}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("workReportSnapshotEvidenceIDs() = %#v, want %#v", got, want)
	}
}

func TestWorkReportLifecycleKeepsImmutableSubmittedRevisions(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")
	author := setupProjectScopeFixture(t, db, []string{"work_report.view", "work_report.submit", "work_report.export"})

	suffix := newID()
	reviewerID := newID()
	reviewerStaffID := newID()
	reviewerRoleID := ensureRole(t, db, "work_report_reviewer_"+suffix[:8], "Work report reviewer")
	for _, permission := range []string{"work_report.view", "work_report.review", "work_report.export"} {
		permissionID := ensurePermission(t, db, permission, permission, "work_report")
		mustExec(t, db, `INSERT INTO role_permissions (role_id,permission_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`, reviewerRoleID, permissionID)
	}
	mustExec(t, db, `INSERT INTO users (id,email,password_hash,full_name,status) VALUES ($1,$2,'demo:pw','Người quản lý','active')`, reviewerID, "work-report-reviewer-"+suffix+"@test.local")
	mustExec(t, db, `INSERT INTO staff (id,user_id,full_name,email,department,position,status) VALUES ($1,$2,'Người quản lý',$3,'Dự án','Chỉ huy trưởng','active')`, reviewerStaffID, reviewerID, "work-report-reviewer-staff-"+suffix+"@test.local")
	mustExec(t, db, `INSERT INTO project_members (project_id,staff_id,role_ids,active) VALUES ($1,$2,'["Reviewer"]'::jsonb,true)`, author.projectID, reviewerStaffID)
	mustExec(t, db, `INSERT INTO user_roles (user_id,role_id) VALUES ($1,$2)`, reviewerID, reviewerRoleID)

	sourceID := newID()
	mustExec(t, db, `
		INSERT INTO operation_daily_reports
			(id,project_id,report_date,reported_by,weather,work_summary,manpower_summary,material_usage_summary,issues_summary,progress_percent,photos,report_delivery_eligible)
		VALUES ($1,$2,'2026-08-18',$3,'Nắng','Đã làm: Hoàn thiện cốp pha','8 công nhân','Thép 200 kg','Không có',35,'[]'::jsonb,true)
	`, sourceID, author.projectID, author.userID)
	t.Cleanup(func() {
		// Deleting the project cascades through the report parent and intentionally
		// removes its immutable child history as one aggregate.
		_, _ = db.ExecContext(ctx, `DELETE FROM projects WHERE id=$1`, author.projectID)
		_, _ = db.ExecContext(ctx, `DELETE FROM user_roles WHERE user_id=$1`, reviewerID)
		_, _ = db.ExecContext(ctx, `DELETE FROM role_permissions WHERE role_id=$1`, reviewerRoleID)
		_, _ = db.ExecContext(ctx, `DELETE FROM roles WHERE id=$1`, reviewerRoleID)
		_, _ = db.ExecContext(ctx, `DELETE FROM staff WHERE id=$1`, reviewerStaffID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id=$1`, reviewerID)
	})

	formalBeforeSubmit, err := service.WorkReports(ctx, WorkReportFilter{Scope: "mine"}, author.userID)
	if err != nil {
		t.Fatal(err)
	}
	sourceDrafts, err := service.WorkReports(ctx, WorkReportFilter{Scope: "source"}, author.userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(formalBeforeSubmit) != 0 || len(sourceDrafts) != 1 || fmt.Sprint(sourceDrafts[0]["status"]) != "draft" {
		t.Fatalf("report register split before submit = formal %#v source %#v", formalBeforeSubmit, sourceDrafts)
	}
	beforeSummary, err := service.WorkReportSummary(ctx, WorkReportFilter{}, author.userID)
	if err != nil {
		t.Fatal(err)
	}
	if beforeSummary.Scopes["source"] != 1 || beforeSummary.TotalVisible != 1 {
		t.Fatalf("summary before submit = %#v, want one distinct source draft", beforeSummary)
	}
	if _, err := service.SubmitWorkReport(ctx, sourceID, WorkReportSubmitRequest{RecipientID: author.userID}, author.userID, "Scoped User"); err == nil {
		t.Fatal("saved-source quick submit must reject the author as recipient")
	}
	if _, err := service.SubmitWorkReport(ctx, sourceID, WorkReportSubmitRequest{RecipientID: reviewerID}, reviewerID, "Người quản lý"); err == nil {
		t.Fatal("saved-source quick submit must reject an actor without submit permission and source authorship")
	}
	assertCount(t, db, `SELECT COUNT(*) FROM work_reports WHERE source_id=$1`, 0, sourceID)

	submitted, err := service.SubmitWorkReport(ctx, sourceID, WorkReportSubmitRequest{RecipientID: reviewerID, Comment: "Báo cáo cuối ngày"}, author.userID, "Scoped User")
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(submitted["status"]) != "submitted" || int(number(submitted["current_revision_no"])) != 1 {
		t.Fatalf("submitted report = %#v", submitted)
	}
	reportID := cleanID(submitted["work_report_id"])
	if reportID == "" {
		t.Fatal("submitted report must receive a lifecycle id")
	}
	if _, err := service.SubmitWorkReport(ctx, sourceID, WorkReportSubmitRequest{RecipientID: reviewerID}, author.userID, "Scoped User"); err == nil {
		t.Fatal("a repeated or stale quick submit must not create a duplicate revision")
	}
	assertCount(t, db, `SELECT COUNT(*) FROM work_report_revisions WHERE report_id=$1`, 1, reportID)
	formalAfterSubmit, err := service.WorkReports(ctx, WorkReportFilter{Scope: "mine"}, author.userID)
	if err != nil {
		t.Fatal(err)
	}
	sourceAfterSubmit, err := service.WorkReports(ctx, WorkReportFilter{Scope: "source"}, author.userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(formalAfterSubmit) != 1 || len(sourceAfterSubmit) != 0 {
		t.Fatalf("report register split after submit = formal %#v source %#v", formalAfterSubmit, sourceAfterSubmit)
	}
	authorSummary, err := service.WorkReportSummary(ctx, WorkReportFilter{}, author.userID)
	if err != nil {
		t.Fatal(err)
	}
	reviewerSummary, err := service.WorkReportSummary(ctx, WorkReportFilter{}, reviewerID)
	if err != nil {
		t.Fatal(err)
	}
	if authorSummary.Scopes["mine"] != 1 || authorSummary.TotalVisible != 1 || reviewerSummary.Scopes["review"] != 1 || reviewerSummary.TotalVisible != 1 {
		t.Fatalf("summaries after submit = author %#v reviewer %#v", authorSummary, reviewerSummary)
	}
	reviewerNavigationSummary, err := service.MyWorkNavigationSummary(ctx, reviewerID)
	if err != nil {
		t.Fatal(err)
	}
	if number(reviewerNavigationSummary["work_total"]) != 0 ||
		number(reviewerNavigationSummary["work_reports_review_pending"]) != 1 ||
		number(reviewerNavigationSummary["approval_total"]) != 1 ||
		number(reviewerNavigationSummary["total"]) != 1 {
		t.Fatalf("reviewer navigation summary = %#v, want 0 work + 1 report approval", reviewerNavigationSummary)
	}

	if _, err := service.UpdateDailyReport(ctx, sourceID, ConstructionDailyReportCreateRequest{}, author.userID, "Scoped User"); err == nil {
		t.Fatal("source must be locked while its submitted revision awaits review")
	}

	detail, err := service.WorkReportDetail(ctx, sourceID, reviewerID)
	if err != nil {
		t.Fatal(err)
	}
	revisions, _ := detail["revisions"].([]Record)
	events, _ := detail["events"].([]Record)
	if len(revisions) != 1 || len(events) != 1 {
		t.Fatalf("detail history = revisions %#v events %#v", detail["revisions"], detail["events"])
	}

	changes, err := service.ReviewWorkReport(ctx, reportID, "request-changes", WorkReportReviewRequest{Comment: "Bổ sung khối lượng nghiệm thu"}, reviewerID, "Người quản lý")
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(changes["status"]) != "needs_changes" {
		t.Fatalf("request changes status = %#v", changes["status"])
	}

	unchangedRows, err := repo.ListProjectDailyReports(ctx, author.projectID)
	if err != nil {
		t.Fatal(err)
	}
	if len(unchangedRows) != 1 || boolValue(unchangedRows[0]["delivery_source_changed"]) {
		t.Fatalf("unchanged delivery state = %#v, want source_changed=false", unchangedRows)
	}
	if _, err := service.SubmitWorkReport(ctx, sourceID, WorkReportSubmitRequest{RecipientID: reviewerID, Comment: "Đã bổ sung"}, author.userID, "Scoped User"); err == nil {
		t.Fatal("unchanged report must not create another submitted revision")
	}
	assertCount(t, db, `SELECT COUNT(*) FROM work_report_revisions WHERE report_id=$1`, 1, reportID)
	assertCount(t, db, `SELECT COUNT(*) FROM work_report_events WHERE report_id=$1 AND event_type='resubmitted'`, 0, reportID)
	assertCount(t, db, `SELECT COUNT(*) FROM work_reports WHERE id=$1 AND status='needs_changes' AND current_revision_no=1`, 1, reportID)

	mustExec(t, db, `UPDATE operation_daily_reports SET work_summary='Đã làm: Hoàn thiện cốp pha và bổ sung khối lượng nghiệm thu' WHERE id=$1`, sourceID)
	changedRows, err := repo.ListProjectDailyReports(ctx, author.projectID)
	if err != nil {
		t.Fatal(err)
	}
	if len(changedRows) != 1 || !boolValue(changedRows[0]["delivery_source_changed"]) {
		t.Fatalf("changed delivery state = %#v, want source_changed=true", changedRows)
	}

	resubmitted, err := service.SubmitWorkReport(ctx, sourceID, WorkReportSubmitRequest{RecipientID: reviewerID, Comment: "Đã bổ sung"}, author.userID, "Scoped User")
	if err != nil {
		t.Fatal(err)
	}
	if int(number(resubmitted["current_revision_no"])) != 2 {
		t.Fatalf("resubmitted revision = %#v", resubmitted["current_revision_no"])
	}

	acknowledged, err := service.ReviewWorkReport(ctx, reportID, "acknowledge", WorkReportReviewRequest{Comment: "Đã xem"}, reviewerID, "Người quản lý")
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(acknowledged["status"]) != "acknowledged" {
		t.Fatalf("acknowledged status = %#v", acknowledged["status"])
	}
	mustDispatchNotifications(t, service)

	reviewerNotifications, err := service.UserNotifications(ctx, NotificationFilter{Limit: 20}, reviewerID)
	if err != nil {
		t.Fatal(err)
	}
	authorNotifications, err := service.UserNotifications(ctx, NotificationFilter{Limit: 20}, author.userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(reviewerNotifications.Items) != 2 || len(authorNotifications.Items) != 2 {
		t.Fatalf("notification lifecycle = reviewer %#v author %#v, want two each", reviewerNotifications.Items, authorNotifications.Items)
	}
	reviewerUnread, err := service.UserNotificationSummary(ctx, reviewerID)
	if err != nil {
		t.Fatal(err)
	}
	if reviewerUnread.UnreadTotal != 2 || reviewerUnread.ByCategory["reports"] != 2 || reviewerUnread.ActionRequired != 0 {
		t.Fatalf("reviewer unread summary = %#v", reviewerUnread)
	}
	if _, err := service.MarkUserNotificationRead(ctx, cleanID(reviewerNotifications.Items[0]["id"]), reviewerID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.MarkUserNotificationRead(ctx, cleanID(reviewerNotifications.Items[0]["id"]), author.userID); err == nil {
		t.Fatal("another actor must not mark a notification as read")
	}
	if _, err := service.MarkAllUserNotificationsRead(ctx, reviewerID); err != nil {
		t.Fatal(err)
	}
	reviewerUnread, err = service.UserNotificationSummary(ctx, reviewerID)
	if err != nil {
		t.Fatal(err)
	}
	if reviewerUnread.UnreadTotal != 0 {
		t.Fatalf("reviewer unread after mark-all = %#v", reviewerUnread)
	}
	mustExec(t, db, `
		INSERT INTO user_notifications (
			recipient_user_id,actor_user_id,project_id,notification_type,source_kind,source_id,source_event_id,
			title_snapshot,message_snapshot,action_key,action_payload_json,idempotency_key,created_at
		)
		SELECT $1::uuid,$2::uuid,$3::uuid,'work_report_submitted','operation_daily_report',$4::uuid,gen_random_uuid(),
			'Kiểm thử phân trang','Thông báo kiểm thử','open_work_report_review','{}'::jsonb,
			'pagination-test-' || $5 || '-' || sequence,
			TIMESTAMPTZ '2026-08-26 03:00:00+00' + sequence * INTERVAL '1 microsecond'
		FROM generate_series(1,25) sequence
	`, reviewerID, author.userID, author.projectID, sourceID, suffix)
	firstNotificationPage, err := service.UserNotifications(ctx, NotificationFilter{State: "unread", Limit: 20}, reviewerID)
	if err != nil {
		t.Fatal(err)
	}
	secondNotificationPage, err := service.UserNotifications(ctx, NotificationFilter{State: "unread", Limit: 20, Cursor: firstNotificationPage.NextCursor}, reviewerID)
	if err != nil {
		t.Fatal(err)
	}
	if len(firstNotificationPage.Items) != 20 || !firstNotificationPage.HasMore || firstNotificationPage.NextCursor == "" || len(secondNotificationPage.Items) != 5 || secondNotificationPage.HasMore {
		t.Fatalf("notification pagination = first %#v second %#v", firstNotificationPage, secondNotificationPage)
	}
	seenNotificationIDs := map[string]bool{}
	for _, notification := range append(firstNotificationPage.Items, secondNotificationPage.Items...) {
		notificationID := cleanID(notification["id"])
		if notificationID == "" || seenNotificationIDs[notificationID] {
			t.Fatalf("notification cursor duplicated or lost identity: %#v", notification)
		}
		seenNotificationIDs[notificationID] = true
	}
	if len(seenNotificationIDs) != 25 {
		t.Fatalf("notification cursor returned %d unique rows, want 25", len(seenNotificationIDs))
	}

	pdf, err := service.ExportWorkReportPDF(ctx, reportID, 1, reviewerID, "Người quản lý")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(pdf.Data, []byte("%PDF-")) {
		t.Fatal("revision export must be a PDF generated from the immutable snapshot")
	}

	if _, err := db.ExecContext(ctx, `UPDATE work_report_revisions SET comment='tampered' WHERE report_id=$1 AND revision_no=1`, reportID); err == nil {
		t.Fatal("revision history update must be rejected by the database")
	}
	if _, err := db.ExecContext(ctx, `DELETE FROM work_report_events WHERE report_id=$1`, reportID); err == nil {
		t.Fatal("event history delete must be rejected by the database")
	}
}

func mustDispatchNotifications(t *testing.T, service *Service) {
	t.Helper()
	if err := service.dispatchNotificationEvents(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestProjectScopedStatisticsExcludeCentralWarehouseInventory(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	service := NewService(NewRepository(db), "test-secret")
	fixture := setupProjectScopeFixture(t, db, []string{"statistics.view"})

	// Convert the fixture role from a global grant into an exact project grant.
	mustExec(t, db, `UPDATE user_roles SET project_id=$1 WHERE user_id=$2 AND role_id=$3`, fixture.projectID, fixture.userID, fixture.roleID)

	scopedMaterialID := newID()
	centralMaterialID := newID()
	warehouseID := newID()
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM stock_movements WHERE material_id IN ($1,$2)`, scopedMaterialID, centralMaterialID)
		_, _ = db.ExecContext(ctx, `DELETE FROM warehouses WHERE id=$1`, warehouseID)
		_, _ = db.ExecContext(ctx, `DELETE FROM materials WHERE id IN ($1,$2)`, scopedMaterialID, centralMaterialID)
	})
	mustExec(t, db, `
		INSERT INTO materials (id,code,name,unit,category,active)
		VALUES ($1,$2,'Vật tư thuộc công trình','m','Test',true),($3,$4,'Vật tư kho trung tâm','m','Test',true)
	`, scopedMaterialID, "STAT-S-"+scopedMaterialID[:8], centralMaterialID, "STAT-C-"+centralMaterialID[:8])
	mustExec(t, db, `INSERT INTO warehouses (id,name,location,type,active) VALUES ($1,'Kho kiểm thử thống kê','Test','central',true)`, warehouseID)
	mustExec(t, db, `
		INSERT INTO stock_movements (movement_code,project_id,warehouse_id,material_id,movement_type,quantity,unit,reason,created_by,approved_by)
		VALUES ($1,$2,$3,$4,'receive',10,'m','Scoped statistics',$5,$5),
		       ($6,NULL,$3,$7,'receive',100,'m','Central statistics',$5,$5)
	`, "STAT-SM-S-"+scopedMaterialID[:6], fixture.projectID, warehouseID, scopedMaterialID, fixture.userID, "STAT-SM-C-"+centralMaterialID[:6], centralMaterialID)

	rows, err := service.Statistic(ctx, "warehouse", OperationalReportRequest{}, fixture.userID)
	if err != nil {
		t.Fatal(err)
	}
	if scoped := recordByID(rows, "material_id", scopedMaterialID); scoped == nil || number(scoped["stock_quantity"]) != 10 {
		t.Fatalf("project-scoped material = %#v, want quantity 10", scoped)
	}
	if central := recordByID(rows, "material_id", centralMaterialID); central != nil {
		t.Fatalf("project-scoped statistics leaked central inventory: %#v", central)
	}
}

func TestProcurementStatisticGroupsByProjectAndKeepsLegacyAggregate(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	service := NewService(NewRepository(db), "test-secret")
	fixture := setupProjectScopeFixture(t, db, []string{"statistics.view"})

	// The actor can see both project sources; the Statistics response must still keep them separate.
	mustExec(t, db, `INSERT INTO project_members (project_id, staff_id, role_ids, active) VALUES ($1,$2,'["Member"]'::jsonb,true)`, fixture.otherProjectID, fixture.staffID)
	firstRequestID, secondRequestID := newID(), newID()
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM procurement_requests WHERE id IN ($1,$2)`, firstRequestID, secondRequestID)
	})
	mustExec(t, db, `
		INSERT INTO procurement_requests (id, project_id, request_code, title, status, budget_status, total_estimated_amount)
		VALUES ($1,$2,$3,'Nguồn thống kê công trình một','draft','within_budget',100),
		       ($4,$5,$6,'Nguồn thống kê công trình hai','draft','within_budget',200)
	`, firstRequestID, fixture.projectID, "STAT-PROC-"+firstRequestID[:8], secondRequestID, fixture.otherProjectID, "STAT-PROC-"+secondRequestID[:8])

	rows, err := service.Statistic(ctx, "procurement", OperationalReportRequest{}, fixture.userID)
	if err != nil {
		t.Fatal(err)
	}
	for projectID, wantAmount := range map[string]float64{fixture.projectID: 100, fixture.otherProjectID: 200} {
		row := recordByID(rows, "project_id", projectID)
		if row == nil || cleanOptionalText(row["project_name"]) == "" || cleanOptionalText(row["project_code"]) == "" || number(row["total"]) != 1 || number(row["amount"]) != wantAmount {
			t.Fatalf("procurement statistic for project %s = %#v", projectID, row)
		}
	}
	exported, err := service.ExportStatistic(ctx, "procurement", "csv", OperationalReportRequest{}, fixture.userID, "Scoped User")
	if err != nil || !bytes.Contains(exported.Data, []byte("Mã công trình")) || !bytes.Contains(exported.Data, []byte("Batch4 Scoped")) || !bytes.Contains(exported.Data, []byte("Batch4 Other")) {
		t.Fatalf("procurement statistics export = %q, err=%v", exported.Data, err)
	}

	legacyRows, err := service.repo.Report(ctx, "procurement", fixture.userID, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range legacyRows {
		if cleanOptionalText(row["status"]) == "draft" && cleanOptionalText(row["budget_status"]) == "within_budget" && number(row["amount"]) == 300 {
			if _, includesProjectID := row["project_id"]; includesProjectID {
				t.Fatalf("legacy procurement aggregate unexpectedly changed grain: %#v", row)
			}
			return
		}
	}
	t.Fatalf("legacy procurement aggregate did not preserve combined row: %#v", legacyRows)
}
