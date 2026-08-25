package core

import (
	"bytes"
	"context"
	"fmt"
	"testing"
)

func TestConstructionReportCreateAndSubmitIsAtomicAndRecommendsProjectPM(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	service := NewService(NewRepository(db), "test-secret")
	fixture := setupWorkItemFlowFixture(t, db, false)

	submitPermissionID := ensurePermission(t, db, "work_report.submit", "Gửi báo cáo", "work_report")
	reviewPermissionID := ensurePermission(t, db, "work_report.review", "Xem xét báo cáo", "work_report")
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
	mustExec(t, db, `INSERT INTO role_permissions (role_id,permission_id) VALUES ($1,$2),($3,$4) ON CONFLICT DO NOTHING`, workerRoleID, submitPermissionID, pmRoleID, reviewPermissionID)
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
	delivery := recordFromAny(created["delivery_transition"])
	if !boolValue(delivery["submitted"]) || delivery["status"] != "submitted" || cleanID(delivery["recipient_id"]) != fixture.pmID || int(number(delivery["revision_no"])) != 1 {
		t.Fatalf("delivery transition = %#v, want submitted revision 1", delivery)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM work_reports WHERE source_id=$1 AND status='submitted' AND recipient_id=$2`, 1, created["id"], fixture.pmID)
	assertCount(t, db, `SELECT COUNT(*) FROM work_report_revisions revision JOIN work_reports report ON report.id=revision.report_id WHERE report.source_id=$1 AND revision.revision_no=1`, 1, created["id"])
	assertCount(t, db, `SELECT COUNT(*) FROM work_report_events event JOIN work_reports report ON report.id=event.report_id WHERE report.source_id=$1 AND event.event_type='submitted'`, 1, created["id"])
	assertCount(t, db, `SELECT COUNT(*) FROM audit_logs WHERE project_id=$1 AND target_id=$2 AND event_type='daily_report_created'`, 1, fixture.projectID, created["id"])
	assertCount(t, db, `SELECT COUNT(*) FROM audit_logs WHERE project_id=$1 AND event_type='work_report_submitted'`, 1, fixture.projectID)
	assertCount(t, db, `SELECT COUNT(*) FROM work_items WHERE id=$1 AND status='in_progress'`, 1, fixture.itemID)

	reportID := cleanID(delivery["work_report_id"])
	if _, err := service.ReviewWorkReport(ctx, reportID, "request-changes", WorkReportReviewRequest{Comment: "Bổ sung ghi chú hiện trường"}, fixture.pmID, "PM"); err != nil {
		t.Fatal(err)
	}
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
	assertCount(t, db, `SELECT COUNT(*) FROM work_report_revisions WHERE report_id=$1`, 2, reportID)
	assertCount(t, db, `SELECT COUNT(*) FROM work_report_events WHERE report_id=$1 AND event_type='resubmitted'`, 1, reportID)
	assertCount(t, db, `SELECT COUNT(*) FROM work_items WHERE id=$1 AND status='in_progress'`, 1, fixture.itemID)
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
