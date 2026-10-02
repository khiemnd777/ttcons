package core

import (
	"context"
	"fmt"
	"testing"
)

func TestNormalizeSupervisionReportFilter(t *testing.T) {
	filter, err := normalizeWorkReportFilter(WorkReportFilter{Scope: "supervision", Limit: 100, ReadStatus: "unread"})
	if err != nil || filter.Limit != 50 || filter.ReadStatus != "unread" {
		t.Fatalf("normalized filter = %#v, %v", filter, err)
	}
	for _, filter := range []WorkReportFilter{{Scope: "supervision", SupervisorID: "invalid"}, {Scope: "supervision", Offset: -1}, {Scope: "supervision", ReadStatus: "seen"}} {
		if _, err := normalizeWorkReportFilter(filter); err == nil {
			t.Fatalf("invalid filter accepted: %#v", filter)
		}
	}
}

func TestDeputyReportReceiptsAreIndependentRevisionScopedAndReauthorized(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")
	author := setupProjectScopeFixture(t, db, []string{"work_report.view", "work_report.submit", "work_report.export"})
	var supervisorRole, deputyRole string
	if err := db.QueryRowContext(ctx, `SELECT id::text FROM roles WHERE name='supervisor'`).Scan(&supervisorRole); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT id::text FROM roles WHERE name='deputy_manager'`).Scan(&deputyRole); err != nil {
		t.Fatal(err)
	}
	mustExec(t, db, `INSERT INTO user_roles(user_id,role_id) VALUES($1,$2)`, author.userID, supervisorRole)
	deputyID, reviewerID, reviewerStaffID := newID(), newID(), newID()
	suffix := newID()
	reviewerRole := ensureRole(t, db, "receipt_reviewer_"+suffix[:8], "Report receipt reviewer")
	for _, permission := range []string{"work_report.view", "work_report.review", "work_report.export"} {
		permissionID := ensurePermission(t, db, permission, permission, "work_report")
		mustExec(t, db, `INSERT INTO role_permissions(role_id,permission_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, reviewerRole, permissionID)
	}
	mustExec(t, db, `INSERT INTO users(id,email,password_hash,full_name,status) VALUES($1,$2,'demo:pw','Deputy','active'),($3,$4,'demo:pw','Reviewer','active')`, deputyID, "deputy-"+suffix+"@test.local", reviewerID, "reviewer-"+suffix+"@test.local")
	mustExec(t, db, `INSERT INTO staff(id,user_id,full_name,email,department,position,status) VALUES($1,$2,'Reviewer',$3,'Project','PM','active')`, reviewerStaffID, reviewerID, "staff-"+suffix+"@test.local")
	mustExec(t, db, `INSERT INTO project_members(project_id,staff_id,role_ids,active) VALUES($1,$2,'["PM"]'::jsonb,true)`, author.projectID, reviewerStaffID)
	mustExec(t, db, `INSERT INTO user_roles(user_id,role_id) VALUES($1,$2),($3,$4)`, reviewerID, reviewerRole, deputyID, deputyRole)
	sourceID := newID()
	mustExec(t, db, `INSERT INTO operation_daily_reports(id,project_id,report_date,reported_by,work_summary,report_delivery_eligible) VALUES($1,$2,'2026-10-02',$3,'Site progress',true)`, sourceID, author.projectID, author.userID)
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM projects WHERE id=$1`, author.projectID)
		_, _ = db.ExecContext(ctx, `DELETE FROM user_roles WHERE user_id IN ($1,$2) OR (user_id=$3 AND role_id=$4)`, deputyID, reviewerID, author.userID, supervisorRole)
		_, _ = db.ExecContext(ctx, `DELETE FROM role_permissions WHERE role_id=$1`, reviewerRole)
		_, _ = db.ExecContext(ctx, `DELETE FROM roles WHERE id=$1`, reviewerRole)
		_, _ = db.ExecContext(ctx, `DELETE FROM staff WHERE id=$1`, reviewerStaffID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id IN ($1,$2)`, deputyID, reviewerID)
	})
	submitted, err := service.SubmitWorkReport(ctx, sourceID, WorkReportSubmitRequest{RecipientID: reviewerID}, author.userID, "Supervisor")
	if err != nil {
		t.Fatal(err)
	}
	reportID := cleanID(submitted["work_report_id"])
	if !boolValue(submitted["is_supervisor_report"]) {
		t.Fatalf("report classification = %#v", submitted)
	}
	mustDispatchNotifications(t, service)
	assertCount(t, db, `SELECT COUNT(*) FROM user_notifications WHERE recipient_user_id=$1 AND event_key='work_report.submitted' AND source_id=$2`, 1, reviewerID, sourceID)
	assertCount(t, db, `SELECT COUNT(*) FROM user_notifications WHERE recipient_user_id=$1 AND event_key='work_report.submitted' AND source_id=$2`, 1, deputyID, sourceID)
	assertCount(t, db, `SELECT COUNT(*) FROM project_members member JOIN staff ON staff.id=member.staff_id WHERE member.project_id=$1 AND staff.user_id=$2`, 0, author.projectID, deputyID)
	rows, err := service.WorkReports(ctx, WorkReportFilter{Scope: "supervision", ReadStatus: "unread", SupervisorID: author.userID}, deputyID)
	if err != nil || len(rows) != 1 {
		t.Fatalf("supervision rows = %#v, %v", rows, err)
	}
	if _, err := service.ReviewWorkReport(ctx, reportID, "acknowledge", WorkReportReviewRequest{RevisionNo: 1}, reviewerID, "Reviewer"); err != nil {
		t.Fatal(err)
	}
	detail, err := service.WorkReportDetail(ctx, sourceID, deputyID)
	if err != nil || !boolValue(recordFromAny(detail["capabilities"])["can_acknowledge"]) || boolValue(detail["viewer_acknowledged"]) {
		t.Fatalf("deputy after recipient ack = %#v, %v", detail, err)
	}
	start, results := make(chan struct{}), make(chan error, 2)
	for attempt := 0; attempt < 2; attempt++ {
		go func() {
			<-start
			_, err := service.ReviewWorkReport(ctx, reportID, "acknowledge", WorkReportReviewRequest{RevisionNo: 1, Comment: "Viewed"}, deputyID, "Deputy")
			results <- err
		}()
	}
	close(start)
	for attempt := 0; attempt < 2; attempt++ {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
	assertCount(t, db, `SELECT COUNT(*) FROM work_report_read_receipts WHERE report_id=$1 AND revision_no=1`, 2, reportID)
	assertCount(t, db, `SELECT COUNT(*) FROM work_report_events WHERE report_id=$1 AND event_type='acknowledged' AND actor_id=$2`, 1, reportID, deputyID)
	if _, err := service.ReviewWorkReport(ctx, reportID, "request-changes", WorkReportReviewRequest{RevisionNo: 1, Comment: "Add progress"}, deputyID, "Deputy"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ReviewWorkReport(ctx, reportID, "acknowledge", WorkReportReviewRequest{RevisionNo: 1}, reviewerID, "Reviewer"); err == nil {
		t.Fatal("ack must not overwrite needs_changes")
	}
	mustExec(t, db, `UPDATE operation_daily_reports SET work_summary='Updated site progress' WHERE id=$1`, sourceID)
	if _, err := service.SubmitWorkReport(ctx, sourceID, WorkReportSubmitRequest{RecipientID: reviewerID}, author.userID, "Supervisor"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ReviewWorkReport(ctx, reportID, "acknowledge", WorkReportReviewRequest{RevisionNo: 1}, deputyID, "Deputy"); err == nil {
		t.Fatal("stale revision must be rejected")
	}
	detail, err = service.WorkReportDetail(ctx, sourceID, deputyID)
	if err != nil || boolValue(detail["viewer_acknowledged"]) {
		t.Fatalf("new revision viewed = %#v, %v", detail, err)
	}
	if _, err := service.ReviewWorkReport(ctx, reportID, "acknowledge", WorkReportReviewRequest{RevisionNo: 2}, deputyID, "Deputy"); err != nil {
		t.Fatal(err)
	}
	pmRows, err := service.WorkReports(ctx, WorkReportFilter{Scope: "review"}, reviewerID)
	if err != nil || len(pmRows) != 1 {
		t.Fatalf("recipient pending after deputy ack = %#v, %v", pmRows, err)
	}
	pmSummary, err := service.WorkReportSummary(ctx, WorkReportFilter{}, reviewerID)
	if err != nil || pmSummary.Scopes["review"] != 1 {
		t.Fatalf("recipient pending count = %#v, %v", pmSummary, err)
	}
	mustExec(t, db, `DELETE FROM user_roles WHERE user_id=$1 AND role_id=$2`, author.userID, supervisorRole)
	otherSourceID := newID()
	mustExec(t, db, `INSERT INTO operation_daily_reports(id,project_id,report_date,reported_by,work_summary,report_delivery_eligible) VALUES($1,$2,'2026-10-03',$3,'Non-supervisor report',true)`, otherSourceID, author.projectID, author.userID)
	other, err := service.SubmitWorkReport(ctx, otherSourceID, WorkReportSubmitRequest{RecipientID: reviewerID}, author.userID, "Author")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.WorkReportDetail(ctx, otherSourceID, deputyID); err == nil {
		t.Fatal("deputy must not read unrelated reports")
	}
	if _, err := service.ExportWorkReportPDF(ctx, cleanID(other["work_report_id"]), 1, deputyID, "Deputy"); err == nil {
		t.Fatal("deputy must not export unrelated reports")
	}
	if _, err := service.ReviewWorkReport(ctx, cleanID(other["work_report_id"]), "acknowledge", WorkReportReviewRequest{RevisionNo: 1}, deputyID, "Deputy"); err == nil {
		t.Fatal("deputy must not review unrelated reports")
	}
	mustExec(t, db, `DELETE FROM user_roles WHERE user_id=$1 AND role_id=$2`, deputyID, deputyRole)
	if _, err := repo.ReviewWorkReport(ctx, reportID, "acknowledge", "", deputyID, "Deputy", true, 2); err == nil {
		t.Fatal("repository must reauthorize instead of trusting manager flag")
	}
	assertCount(t, db, `SELECT COUNT(*) FROM work_report_revisions WHERE report_id=$1`, 2, reportID)
	if fmt.Sprint(detail["status"]) != "submitted" {
		t.Fatalf("resubmitted status = %#v", detail["status"])
	}
}
