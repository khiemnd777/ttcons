package core

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestBusinessCoreOperationsServiceRecordsActorAndScope(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")
	fixture := setupProjectScopeFixture(t, db, []string{"operation.view", "operation.report", "operation.issue_create", "operation.issue_update"})

	report, err := service.CreateDailyReport(ctx, fixture.projectID, ConstructionDailyReportCreateRequest{
		ReportDate:      "2026-06-23",
		Weather:         "Nắng",
		ProgressPercent: 42,
		DoneItems:       []ConstructionDailyReportWorkItemRequest{{Title: "Đổ bê tông sàn"}},
		FieldNotes:      "Không",
		CrewItems:       []ConstructionDailyReportCrewRequest{{CrewName: "Tổ bê tông", WorkerCount: 8}},
		MaterialItems:   []ConstructionDailyReportMaterialRequest{{MaterialName: "Bê tông M300", Quantity: 24, Unit: "m3"}},
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

func TestCreateDailyReportPersistsStructuredEvidenceAndRejectsMismatchedEvidence(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")
	fixture := setupWorkItemFlowFixture(t, db, false)

	evidence, err := service.AddWorkItemEvidence(ctx, fixture.itemID, WorkItemEvidenceRequest{
		EvidenceType: "photo",
		Name:         "Ảnh báo cáo thi công",
		FileURL:      "https://example.com/report-photo.jpg",
		Visibility:   "project",
	}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	evidenceID := cleanID(evidence["id"])
	req := ConstructionDailyReportCreateRequest{
		ReportDate:        "2026-07-13",
		Weather:           "Nắng",
		ProgressPercent:   68,
		WorkItemID:        fixture.itemID,
		DoneItems:         []ConstructionDailyReportWorkItemRequest{{Title: "Hoàn thiện trần khu A"}},
		CurrentItems:      []ConstructionDailyReportWorkItemRequest{{Title: "Sơn lớp hai khu B"}},
		NextItems:         []ConstructionDailyReportWorkItemRequest{{Title: "Vệ sinh và nghiệm thu nội bộ"}},
		Blockers:          []ConstructionDailyReportBlockerRequest{{Content: "Chờ bổ sung mẫu màu"}},
		FieldNotes:        "Không có sự cố an toàn.",
		CrewItems:         []ConstructionDailyReportCrewRequest{{CrewName: "Tổ hoàn thiện", WorkerCount: 8, WorkHours: 8}},
		MaterialItems:     []ConstructionDailyReportMaterialRequest{{MaterialName: "Sơn nội thất", Quantity: 12, Unit: "thùng"}},
		EvidenceIDs:       []string{evidenceID},
		PrimaryEvidenceID: evidenceID,
	}
	report, err := service.CreateDailyReport(ctx, fixture.projectID, req, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	if cleanID(report["work_item_id"]) != fixture.itemID || cleanID(report["primary_evidence_id"]) != evidenceID {
		t.Fatalf("daily report source/evidence = %v/%v, want %s/%s", report["work_item_id"], report["primary_evidence_id"], fixture.itemID, evidenceID)
	}
	structured, ok := report["structured_json"].(map[string]any)
	if !ok {
		t.Fatalf("structured_json = %#v, want object", report["structured_json"])
	}
	if len(workflowAnswerRecordList(structured["completed_items"])) != 2 || len(workflowAnswerRecordList(structured["blockers"])) != 1 {
		t.Fatalf("structured report = %#v, want completed and blocker rows", structured)
	}
	if fmt.Sprint(report["issues_summary"]) != "Không có sự cố an toàn." {
		t.Fatalf("field note = %v, want neutral note", report["issues_summary"])
	}

	updatedReq := req
	updatedReq.ProgressPercent = 74
	updatedReq.DoneItems = []ConstructionDailyReportWorkItemRequest{{Title: "Hoàn thiện trần khu A và B"}}
	updatedReq.ManpowerSummary = "Dữ liệu tóm tắt không được dùng khi đã có tổ đội"
	updated, err := service.UpdateDailyReport(ctx, cleanID(report["id"]), updatedReq, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	if number(updated["progress_percent"]) != 74 || cleanID(updated["reported_by"]) != fixture.workerID {
		t.Fatalf("updated report progress/reporter = %v/%v, want 74/%s", updated["progress_percent"], updated["reported_by"], fixture.workerID)
	}
	if !strings.Contains(fmt.Sprint(updated["work_summary"]), "Hoàn thiện trần khu A và B") || strings.Contains(fmt.Sprint(updated["manpower_summary"]), "Dữ liệu tóm tắt") {
		t.Fatalf("updated report summaries = work %q, manpower %q", updated["work_summary"], updated["manpower_summary"])
	}

	uploadDir := t.TempDir()
	service.ConfigureUploads(uploadDir, 1024*1024)
	content := []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a, 0, 0, 0, 0, 'I', 'E', 'N', 'D'}
	linked, err := service.AddDailyReportEvidenceUpload(ctx, cleanID(report["id"]), fixture.itemID, WorkItemEvidenceRequest{
		EvidenceType: "photo",
		Name:         "Ảnh chụp từ báo cáo",
		Caption:      "Khu B",
		Visibility:   "project",
	}, testMultipartFileHeader(t, "file", "report-camera.png", content), fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	linkedPhotos := constructionReportStringList(linked["photos"])
	if len(linkedPhotos) != 2 || cleanID(linked["primary_evidence_id"]) != evidenceID {
		t.Fatalf("linked report photos/primary = %#v/%v, want two photos and primary %s", linkedPhotos, linked["primary_evidence_id"], evidenceID)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM audit_logs WHERE project_id=$1 AND target_id=$2 AND event_type='daily_report_updated'`, 1, fixture.projectID, report["id"])
	assertCount(t, db, `SELECT COUNT(*) FROM audit_logs WHERE project_id=$1 AND target_id=$2 AND event_type='daily_report_evidence_added'`, 1, fixture.projectID, report["id"])

	otherItemID := newID()
	mustExec(t, db, `
		INSERT INTO work_items
			(id, project_id, source_type, source_id, title, work_type, status, role_ids, evidence_requirement_json, created_by, updated_by)
		VALUES
			($1,$2,'issue',$3,'Other report source','task','ready','["Đội thi công"]'::jsonb,'{"required_permission":"operation.report"}'::jsonb,$4,$4)
	`, otherItemID, fixture.projectID, newID(), fixture.workerID)
	otherEvidence, err := service.AddWorkItemEvidence(ctx, otherItemID, WorkItemEvidenceRequest{
		EvidenceType: "photo",
		Name:         "Ảnh thuộc đầu việc khác",
		FileURL:      "https://example.com/other-report-photo.jpg",
		Visibility:   "project",
	}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}

	invalid := req
	invalid.ReportDate = "2026-07-14"
	invalid.EvidenceIDs = []string{cleanID(otherEvidence["id"])}
	invalid.PrimaryEvidenceID = ""
	_, err = service.CreateDailyReport(ctx, fixture.projectID, invalid, fixture.workerID, "Worker")
	assertValidationError(t, err)

	_, err = service.AddDailyReportEvidenceUpload(ctx, cleanID(report["id"]), otherItemID, WorkItemEvidenceRequest{
		EvidenceType: "photo",
		Name:         "Sai đầu việc",
		Visibility:   "project",
	}, testMultipartFileHeader(t, "file", "wrong-source.png", content), fixture.workerID, "Worker")
	assertValidationError(t, err)

	var workflowFormInstanceID string
	if err := db.QueryRowContext(ctx, `
		INSERT INTO workflow_form_instances (project_id, work_item_id, form_template_id, form_version_id, form_type, title, created_by)
		SELECT $1, $2, v.template_id, v.id, t.form_type, 'Read-only report source', $3
		FROM workflow_form_versions v
		JOIN workflow_form_templates t ON t.id=v.template_id
		WHERE t.code='builtin_construction_daily_report' AND v.status='published'
		ORDER BY v.version_no DESC
		LIMIT 1
		RETURNING id
	`, fixture.projectID, fixture.itemID, fixture.workerID).Scan(&workflowFormInstanceID); err != nil {
		t.Fatal(err)
	}
	mustExec(t, db, `UPDATE operation_daily_reports SET workflow_form_instance_id=$2 WHERE id=$1`, report["id"], workflowFormInstanceID)
	_, err = service.UpdateDailyReport(ctx, cleanID(report["id"]), updatedReq, fixture.workerID, "Worker")
	assertValidationError(t, err)
	_, err = service.AddDailyReportEvidenceUpload(ctx, cleanID(report["id"]), fixture.itemID, WorkItemEvidenceRequest{
		EvidenceType: "photo",
		Name:         "Workflow read-only",
		Visibility:   "project",
	}, testMultipartFileHeader(t, "file", "workflow-read-only.png", content), fixture.workerID, "Worker")
	assertValidationError(t, err)
}

func TestConstructionWorkspaceLinksDailyReportThroughChecksActionsAndResolution(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	service := NewService(NewRepository(db), "test-secret")
	fixture := setupWorkItemFlowFixture(t, db, false)

	evidence, err := service.AddWorkItemEvidence(ctx, fixture.itemID, WorkItemEvidenceRequest{
		EvidenceType: "photo",
		Name:         "Hiện trạng từ báo cáo",
		FileURL:      "https://example.com/construction-report-source.jpg",
		Visibility:   "project",
	}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	evidenceID := cleanID(evidence["id"])
	report, err := service.CreateDailyReport(ctx, fixture.projectID, ConstructionDailyReportCreateRequest{
		ReportDate:        "2026-07-14",
		Weather:           "Nắng",
		ProgressPercent:   70,
		WorkItemID:        fixture.itemID,
		DoneItems:         []ConstructionDailyReportWorkItemRequest{{Title: "Hoàn thiện trần"}},
		Blockers:          []ConstructionDailyReportBlockerRequest{{Content: "Cao độ cần kiểm tra"}},
		EvidenceIDs:       []string{evidenceID},
		PrimaryEvidenceID: evidenceID,
	}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	reportID := cleanID(report["id"])

	check, err := service.CreateConstructionCheck(ctx, fixture.projectID, ConstructionCheckRequest{
		WorkItemID:          fixture.itemID,
		SourceDailyReportID: reportID,
		Title:               "Kiểm tra cao độ từ báo cáo",
		Location:            "Khu A",
		ContextEvidenceIDs:  []string{evidenceID},
	}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	if cleanID(check["source_daily_report_id"]) != reportID {
		t.Fatalf("check source report = %v, want %s", check["source_daily_report_id"], reportID)
	}

	directAction, err := service.CreateConstructionAction(ctx, fixture.projectID, ConstructionActionRequest{
		WorkItemID:          fixture.itemID,
		SourceDailyReportID: reportID,
		Title:               "Xử lý blocker trực tiếp từ báo cáo",
		Description:         "Cân chỉnh khu vực hiện trường",
		AssignedUserID:      fixture.workerID,
		DueAt:               "2026-07-16T17:00:00+07:00",
		ImpactScope:         "none",
		EvidenceID:          evidenceID,
	}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	if cleanID(directAction["source_daily_report_id"]) != reportID {
		t.Fatalf("direct action source report = %v, want %s", directAction["source_daily_report_id"], reportID)
	}

	completed, err := service.CompleteConstructionCheck(ctx, cleanID(check["id"]), ConstructionCheckCompleteRequest{
		Result:         "fail",
		Note:           "Sai cao độ 8mm",
		EvidenceIDs:    []string{evidenceID},
		ActionTitle:    "Khắc phục cao độ từ kiểm tra",
		AssignedUserID: fixture.workerID,
		DueAt:          "2026-07-17T17:00:00+07:00",
		ImpactScope:    "none",
		Priority:       "urgent",
	}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	checkAction, ok := completed["action"].(Record)
	if !ok {
		t.Fatalf("completed check action = %#v, want action record", completed["action"])
	}
	if cleanID(checkAction["source_check_id"]) != cleanID(check["id"]) || cleanID(checkAction["source_daily_report_id"]) != "" {
		t.Fatalf("check action source = check %v/report %v, want inherited check only", checkAction["source_check_id"], checkAction["source_daily_report_id"])
	}

	workspace, err := service.ProjectConstructionWorkspace(ctx, fixture.projectID)
	if err != nil {
		t.Fatal(err)
	}
	checks := workflowAnswerRecordList(workspace["checks"])
	actions := workflowAnswerRecordList(workspace["actions"])
	if len(checks) != 1 || len(actions) != 2 {
		t.Fatalf("workspace linked rows = %d checks/%d actions, want 1/2", len(checks), len(actions))
	}
	linkKinds := map[string]bool{}
	for _, action := range actions {
		if cleanID(action["source_daily_report_id"]) != reportID {
			t.Fatalf("workspace action source report = %v, want %s", action["source_daily_report_id"], reportID)
		}
		linkKinds[fmt.Sprint(action["source_report_link_kind"])] = true
	}
	if !linkKinds["direct"] || !linkKinds["check"] {
		t.Fatalf("workspace action link kinds = %#v, want direct and check", linkKinds)
	}

	otherItemID := newID()
	mustExec(t, db, `
		INSERT INTO work_items
			(id, project_id, source_type, source_id, title, work_type, status, role_ids, evidence_requirement_json, created_by, updated_by)
		VALUES
			($1,$2,'issue',$3,'Other construction source','task','ready','["Đội thi công"]'::jsonb,'{}'::jsonb,$4,$4)
	`, otherItemID, fixture.projectID, newID(), fixture.workerID)
	_, err = service.CreateConstructionCheck(ctx, fixture.projectID, ConstructionCheckRequest{
		WorkItemID:          otherItemID,
		SourceDailyReportID: reportID,
		Title:               "Sai nguồn công việc",
	}, fixture.workerID, "Worker")
	assertValidationError(t, err)
}

func TestPlanningConstructionHandoffIsIdempotentAndTracksCheckResolution(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")
	fixture := setupWorkItemFlowFixture(t, db, false)

	runID := newID()
	planningNodeID := newID()
	constructionNodeID := newID()
	mustExec(t, db, `
		INSERT INTO pipeline_runs (id,project_id,template_id,status,engine_version)
		VALUES ($1,$2,'40000000-0000-0000-0000-000000000021','in_progress',2)
	`, runID, fixture.projectID)
	mustExec(t, db, `
		INSERT INTO pipeline_run_nodes (id,run_id,project_id,name,type,order_index,responsible_role_ids,status,started_at)
		VALUES
			($1,$3,$4,'Kế hoạch thi công','construction_planning',9,'["PM","QA/QC"]'::jsonb,'in_progress',now()),
			($2,$3,$4,'Thi công','construction',10,'["QA/QC","Giám sát"]'::jsonb,'pending',NULL)
	`, planningNodeID, constructionNodeID, runID, fixture.projectID)
	mustExec(t, db, `UPDATE pipeline_runs SET current_node_id=$2 WHERE id=$1`, runID, planningNodeID)
	mustExec(t, db, `UPDATE projects SET current_pipeline_run_id=$2 WHERE id=$1`, fixture.projectID, runID)

	beforeIssues, err := repo.NodeCompletionIssues(ctx, Record{"id": planningNodeID, "run_id": runID, "type": "construction_planning", "status": "in_progress"})
	if err != nil {
		t.Fatal(err)
	}
	if !recordListHasCode(beforeIssues, "CONSTRUCTION_HANDOFF_REQUIRED") {
		t.Fatalf("planning completion issues before handoff = %#v, want CONSTRUCTION_HANDOFF_REQUIRED", beforeIssues)
	}

	req := ConstructionHandoffActivateRequest{
		Title:           "Bàn giao hoàn thiện khu A",
		Notes:           "Giám sát theo hồ sơ đã duyệt",
		SourceRunNodeID: planningNodeID,
		Items: []ConstructionHandoffItemRequest{
			{
				SourceFieldKey: "inspection_items", SourceItemIndex: 0,
				Title: "Kiểm tra cao độ trần", Location: "Khu A", ResponsibleRole: "QA/QC", AssignedUserID: fixture.workerID,
				EvidenceRequired: true,
				Criteria:         []ConstructionCheckCriterion{{Title: "Cao độ", Criteria: "Sai số không quá 3mm", EvidenceRequired: true}},
			},
			{
				SourceFieldKey: "inspection_items", SourceItemIndex: 1,
				Title: "Kiểm tra bề mặt sơn", Location: "Khu A", ResponsibleRole: "Giám sát", AssignedUserID: fixture.workerID,
				EvidenceRequired: true,
				Criteria:         []ConstructionCheckCriterion{{Title: "Bề mặt", Criteria: "Không nứt, phồng hoặc lệch màu", EvidenceRequired: true}},
			},
		},
	}

	first, err := service.ActivateConstructionHandoff(ctx, fixture.projectID, req, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.ActivateConstructionHandoff(ctx, fixture.projectID, req, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	if cleanID(first["id"]) != cleanID(second["id"]) || !boolValue(second["idempotent_replay"]) {
		t.Fatalf("handoff replay = first %v/second %v/idempotent %v", first["id"], second["id"], second["idempotent_replay"])
	}
	assertCount(t, db, `SELECT COUNT(*) FROM construction_handoffs WHERE project_id=$1`, 1, fixture.projectID)
	assertCount(t, db, `SELECT COUNT(*) FROM construction_handoff_items WHERE project_id=$1`, 2, fixture.projectID)
	assertCount(t, db, `SELECT COUNT(*) FROM work_items WHERE project_id=$1 AND source_type='construction_handoff_item'`, 2, fixture.projectID)
	assertCount(t, db, `SELECT COUNT(*) FROM construction_checks WHERE project_id=$1 AND source_handoff_item_id IS NOT NULL`, 2, fixture.projectID)
	afterIssues, err := repo.NodeCompletionIssues(ctx, Record{"id": planningNodeID, "run_id": runID, "type": "construction_planning", "status": "in_progress"})
	if err != nil {
		t.Fatal(err)
	}
	if recordListHasCode(afterIssues, "CONSTRUCTION_HANDOFF_REQUIRED") {
		t.Fatalf("planning completion issues after handoff = %#v, handoff blocker should be cleared", afterIssues)
	}
	mustExec(t, db, `UPDATE pipeline_run_nodes SET status='completed',completed_at=now() WHERE id=$1`, planningNodeID)
	mustExec(t, db, `UPDATE pipeline_run_nodes SET status='in_progress',started_at=now() WHERE id=$1`, constructionNodeID)
	mustExec(t, db, `UPDATE pipeline_runs SET current_node_id=$2 WHERE id=$1`, runID, constructionNodeID)
	mustExec(t, db, `UPDATE work_items SET status='ready' WHERE project_id=$1 AND source_type='construction_handoff_item' AND status='pending'`, fixture.projectID)

	items := workflowAnswerRecordList(first["items"])
	if len(items) != 2 {
		t.Fatalf("handoff items = %#v, want 2", first["items"])
	}
	checkIDs := []string{cleanID(items[0]["generated_check_id"]), cleanID(items[1]["generated_check_id"])}
	workIDs := []string{cleanID(items[0]["generated_work_item_id"]), cleanID(items[1]["generated_work_item_id"])}
	evidenceIDs := make([]string, 0, 3)
	for index, workID := range workIDs {
		evidence, addErr := service.AddWorkItemEvidence(ctx, workID, WorkItemEvidenceRequest{
			EvidenceType: "photo", Name: fmt.Sprintf("Handoff result %d", index+1), FileURL: fmt.Sprintf("https://example.com/handoff-%d.jpg", index+1), Visibility: "project",
		}, fixture.workerID, "Worker")
		if addErr != nil {
			t.Fatal(addErr)
		}
		evidenceIDs = append(evidenceIDs, cleanID(evidence["id"]))
	}
	manualAction, err := service.CreateConstructionAction(ctx, fixture.projectID, ConstructionActionRequest{
		WorkItemID: workIDs[0], Title: "Dọn vật tư trước kiểm tra", Description: "Việc hiện trường độc lập với kết quả kiểm tra",
		AssignedUserID: fixture.workerID, DueAt: "2026-07-19T17:00:00+07:00", ImpactScope: "none", Priority: "normal", EvidenceID: evidenceIDs[0],
	}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.SubmitConstructionActionResolution(ctx, cleanID(manualAction["id"]), ConstructionActionResolutionRequest{
		EvidenceID: evidenceIDs[0], Note: "Đã dọn vật tư",
	}, fixture.workerID, "Worker"); err != nil {
		t.Fatal(err)
	}
	if _, err = service.VerifyConstructionAction(ctx, cleanID(manualAction["id"]), ConstructionActionVerifyRequest{
		Result: "pass", Comment: "Đạt",
	}, fixture.workerID, "Worker"); err != nil {
		t.Fatal(err)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM work_items WHERE id=$1 AND status='ready'`, 1, workIDs[0])

	if _, err = service.CompleteConstructionCheck(ctx, checkIDs[0], ConstructionCheckCompleteRequest{
		Result: "pass", Note: "Đạt", EvidenceIDs: []string{evidenceIDs[0]},
	}, fixture.workerID, "Worker"); err != nil {
		t.Fatal(err)
	}
	failed, err := service.CompleteConstructionCheck(ctx, checkIDs[1], ConstructionCheckCompleteRequest{
		Result: "fail", Note: "Lệch màu", EvidenceIDs: []string{evidenceIDs[1]}, ActionTitle: "Sơn dặm lại khu A",
		AssignedUserID: fixture.workerID, DueAt: "2026-07-20T17:00:00+07:00", ImpactScope: "none", Priority: "normal",
	}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	action := recordFromAny(failed["action"])
	if cleanID(action["id"]) == "" {
		t.Fatalf("failed check did not create action: %#v", failed)
	}
	resolutionEvidence, err := service.AddWorkItemEvidence(ctx, workIDs[1], WorkItemEvidenceRequest{
		EvidenceType: "photo", Name: "Handoff resolution", FileURL: "https://example.com/handoff-resolution.jpg", Visibility: "project",
	}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.SubmitConstructionActionResolution(ctx, cleanID(action["id"]), ConstructionActionResolutionRequest{
		EvidenceID: cleanID(resolutionEvidence["id"]), Note: "Đã sơn dặm",
	}, fixture.workerID, "Worker"); err != nil {
		t.Fatal(err)
	}
	if _, err = service.VerifyConstructionAction(ctx, cleanID(action["id"]), ConstructionActionVerifyRequest{
		Result: "pass", Comment: "Đạt sau khắc phục",
	}, fixture.workerID, "Worker"); err != nil {
		t.Fatal(err)
	}

	assertCount(t, db, `SELECT COUNT(*) FROM work_items WHERE project_id=$1 AND source_type='construction_handoff_item' AND status='completed'`, 2, fixture.projectID)
	assertCount(t, db, `SELECT COUNT(*) FROM construction_handoff_items WHERE project_id=$1 AND status='completed'`, 2, fixture.projectID)
	assertCount(t, db, `SELECT COUNT(*) FROM construction_action_items WHERE id=$1 AND status='done'`, 1, action["id"])
}

func TestPlanningConstructionHandoffSupersedesOnlyUnstartedFormRevision(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	service := NewService(NewRepository(db), "test-secret")
	fixture := setupWorkItemFlowFixture(t, db, false)

	runID := newID()
	planningNodeID := newID()
	constructionNodeID := newID()
	formID := newID()
	mustExec(t, db, `
		INSERT INTO pipeline_runs (id,project_id,template_id,status,engine_version)
		VALUES ($1,$2,'40000000-0000-0000-0000-000000000021','in_progress',2)
	`, runID, fixture.projectID)
	mustExec(t, db, `
		INSERT INTO pipeline_run_nodes (id,run_id,project_id,name,type,order_index,responsible_role_ids,status,started_at)
		VALUES
			($1,$3,$4,'Kế hoạch thi công','construction_planning',9,'["PM","QA/QC"]'::jsonb,'in_progress',now()),
			($2,$3,$4,'Thi công','construction',10,'["QA/QC","Giám sát"]'::jsonb,'pending',NULL)
	`, planningNodeID, constructionNodeID, runID, fixture.projectID)
	mustExec(t, db, `UPDATE pipeline_runs SET current_node_id=$2 WHERE id=$1`, runID, planningNodeID)
	mustExec(t, db, `UPDATE projects SET current_pipeline_run_id=$2 WHERE id=$1`, fixture.projectID, runID)
	mustExec(t, db, `
		INSERT INTO workflow_form_instances (
			id,project_id,pipeline_run_id,run_node_id,form_template_id,form_version_id,form_type,title,status,
			current_revision,answers_snapshot_json,created_by,submitted_by,submitted_at
		)
		SELECT $1,$2,$3,$4,t.id,v.id,'checklist','Kế hoạch bàn giao','submitted',1,
			'{"handoff_title":"Đợt 1","inspection_items":[{"title":"Kiểm tra trần","criteria":"Sai số không quá 3mm"}]}'::jsonb,
			$5,$5,now()
		FROM workflow_form_templates t
		JOIN workflow_form_versions v ON v.template_id=t.id AND v.version_no=1
		WHERE t.code='client_construction_handoff_plan'
	`, formID, fixture.projectID, runID, planningNodeID, fixture.workerID)

	revisionOne := ConstructionHandoffActivateRequest{
		Title:                "Bàn giao revision 1",
		SourceRunNodeID:      planningNodeID,
		SourceFormInstanceID: formID,
		SourceFormRevisionNo: 1,
		Items: []ConstructionHandoffItemRequest{{
			SourceFieldKey: "inspection_items", SourceItemIndex: 0, Title: "Kiểm tra trần",
			ResponsibleRole: "QA/QC", EvidenceRequired: true,
			Criteria: []ConstructionCheckCriterion{{Title: "Cao độ", Criteria: "Sai số không quá 3mm", EvidenceRequired: true}},
		}},
	}
	first, err := service.ActivateConstructionHandoff(ctx, fixture.projectID, revisionOne, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	replayed, err := service.ActivateConstructionHandoff(ctx, fixture.projectID, ConstructionHandoffActivateRequest{
		Title:                "Nội dung client thay đổi nhưng cùng form revision",
		SourceRunNodeID:      planningNodeID,
		SourceFormInstanceID: formID,
		SourceFormRevisionNo: 1,
		Items:                revisionOne.Items,
	}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	if cleanID(replayed["id"]) != cleanID(first["id"]) || !boolValue(replayed["idempotent_replay"]) {
		t.Fatalf("same form revision replay = first %v/replayed %v/idempotent %v", first["id"], replayed["id"], replayed["idempotent_replay"])
	}

	mustExec(t, db, `
		UPDATE workflow_form_instances
		SET current_revision=2,
			answers_snapshot_json='{"handoff_title":"Đợt 2","inspection_items":[{"title":"Kiểm tra sơn","criteria":"Không lệch màu"}]}'::jsonb,
			updated_at=now()
		WHERE id=$1
	`, formID)
	_, err = service.ActivateConstructionHandoff(ctx, fixture.projectID, revisionOne, fixture.workerID, "Worker")
	assertValidationError(t, err)
	assertCount(t, db, `SELECT COUNT(*) FROM construction_handoffs WHERE project_id=$1`, 1, fixture.projectID)
	assertCount(t, db, `SELECT COUNT(*) FROM construction_handoff_items WHERE handoff_id=$1 AND status='activated'`, 1, first["id"])

	revisionTwo := revisionOne
	revisionTwo.Title = "Bàn giao revision 2"
	revisionTwo.SourceFormRevisionNo = 2
	revisionTwo.Items = []ConstructionHandoffItemRequest{{
		SourceFieldKey: "inspection_items", SourceItemIndex: 0, Title: "Kiểm tra sơn",
		ResponsibleRole: "Giám sát", EvidenceRequired: true,
		Criteria: []ConstructionCheckCriterion{{Title: "Bề mặt", Criteria: "Không lệch màu", EvidenceRequired: true}},
	}}
	second, err := service.ActivateConstructionHandoff(ctx, fixture.projectID, revisionTwo, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	if cleanID(second["id"]) == cleanID(first["id"]) || int(number(second["source_form_revision_no"])) != 2 {
		t.Fatalf("revision two handoff = id %v/form revision %v, want new id/revision 2", second["id"], second["source_form_revision_no"])
	}
	assertCount(t, db, `SELECT COUNT(*) FROM construction_handoffs WHERE project_id=$1`, 2, fixture.projectID)
	assertCount(t, db, `SELECT COUNT(*) FROM construction_handoff_items WHERE handoff_id=$1 AND status='cancelled'`, 1, first["id"])
	assertCount(t, db, `SELECT COUNT(*) FROM construction_checks c JOIN construction_handoff_items i ON i.id=c.source_handoff_item_id WHERE i.handoff_id=$1 AND c.status='cancelled'`, 1, first["id"])
	assertCount(t, db, `SELECT COUNT(*) FROM work_items w JOIN construction_handoff_items i ON i.generated_work_item_id=w.id WHERE i.handoff_id=$1 AND w.status='cancelled'`, 1, first["id"])

	secondItems := workflowAnswerRecordList(second["items"])
	if len(secondItems) != 1 {
		t.Fatalf("revision two items = %#v, want one item", second["items"])
	}
	mustExec(t, db, `UPDATE work_items SET status='in_progress',started_at=now() WHERE id=$1`, secondItems[0]["generated_work_item_id"])
	mustExec(t, db, `UPDATE workflow_form_instances SET current_revision=3,updated_at=now() WHERE id=$1`, formID)
	revisionThree := revisionTwo
	revisionThree.Title = "Bàn giao revision 3"
	revisionThree.SourceFormRevisionNo = 3
	_, err = service.ActivateConstructionHandoff(ctx, fixture.projectID, revisionThree, fixture.workerID, "Worker")
	assertValidationError(t, err)
	assertCount(t, db, `SELECT COUNT(*) FROM construction_handoffs WHERE project_id=$1`, 2, fixture.projectID)
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
		  AND wia.comment='Đầu việc kế hoạch tiếp theo được sinh từ nhật ký kế hoạch'
	`, 1, instanceID)
	assertCount(t, db, `
		SELECT COUNT(*)
		FROM work_item_activities
		WHERE work_item_id=$1
		  AND activity_type='system_event'
		  AND comment='Nhật ký kế hoạch đã sinh 1 đầu việc kế hoạch tiếp theo'
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
	assertCount(t, db, `
		SELECT COUNT(*)
		FROM operation_daily_reports
		WHERE project_id=$1
		  AND workflow_form_instance_id=$2
		  AND work_item_id=$3
		  AND report_date='2026-07-12'
		  AND reported_by=$4
		  AND progress_percent=42
		  AND primary_evidence_id=$5
		  AND work_summary='Xây tường trục A'
		  AND manpower_summary='8 nhân sự / 1 đội'
		  AND material_usage_summary='Xi măng'
		  AND issues_summary='Cần bổ sung xi măng trước ca chiều'
		  AND structured_json->'completed_items'->0->>'actual_quantity'='24.5'
	`, 1, fixture.projectID, instanceID, fixture.itemID, fixture.workerID, evidenceID)
	assertCount(t, db, `SELECT COUNT(*) FROM work_items WHERE project_id=$1 AND source_type='workflow_form_action_item' AND source_id=$2`, 1, fixture.projectID, instanceID)
	assertCount(t, db, `
		SELECT COUNT(*)
		FROM work_item_activities wia
		JOIN work_items wi ON wi.id=wia.work_item_id
		WHERE wi.source_type='workflow_form_action_item'
		  AND wi.source_id=$1
		  AND wia.activity_type='created'
		  AND wia.comment='Đầu việc kế hoạch tiếp theo được sinh từ nhật ký thi công'
		  AND wia.metadata_json->>'source'='construction_daily_report'
		  AND wia.metadata_json->>'construction_scope'='civil'
	`, 1, instanceID)
	assertCount(t, db, `
		SELECT COUNT(*)
		FROM work_item_activities
		WHERE work_item_id=$1
		  AND activity_type='system_event'
		  AND comment='Nhật ký thi công đã sinh 1 đầu việc kế hoạch tiếp theo'
		  AND metadata_json->>'source'='construction_daily_report'
	`, 1, fixture.itemID)
	assertCount(t, db, `SELECT COUNT(*) FROM work_items WHERE id=$1 AND completion_payload_json->'workflow_form_answers'->'material_usage_items'->0->>'source'='site_balance'`, 1, fixture.itemID)
}
