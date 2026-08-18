package core

import (
	"bytes"
	"context"
	"fmt"
	"testing"
)

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
