package core

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestClientWorkflowAutomationCreatesOutgoingAcceptanceAndReceivableDrafts(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	fixture := setupClientOfficialWorkflowFixture(t, db, 180000000)
	workItemID := clientOfficialWorkItemIDByAction(t, db, fixture.projectID, "finance.scaffold_outgoing_acceptance")
	mustExec(t, db, `UPDATE work_items SET status='completed' WHERE id=$1`, workItemID)
	assertValidationError(t, repo.RunRequiredWorkflowAutomationForWorkItem(ctx, workItemID, fixture.actorID, "Client Actor"))
	assertCount(t, db, `
		SELECT COUNT(*)
		FROM acceptance_records
		WHERE project_id=$1
			AND node_id=(SELECT run_node_id FROM work_items WHERE id=$2)
			AND type='final'
			AND status='draft'
			AND summary_json->>'automation_scaffold'='true'
	`, 1, fixture.projectID, workItemID)
	assertCount(t, db, `
		SELECT COUNT(*)
		FROM receivables rec
		JOIN acceptance_records acc ON acc.id=rec.source_id
		WHERE acc.project_id=$1
			AND acc.node_id=(SELECT run_node_id FROM work_items WHERE id=$2)
			AND rec.source_type='acceptance'
			AND rec.status='draft'
			AND rec.gross_amount=180000000
			AND rec.remaining_amount=180000000
	`, 1, fixture.projectID, workItemID)
	assertCount(t, db, `
		SELECT COUNT(*)
		FROM receivable_payment_schedules rps
		JOIN receivables rec ON rec.id=rps.receivable_id
		JOIN acceptance_records acc ON acc.id=rec.source_id
		WHERE acc.project_id=$1
			AND acc.node_id=(SELECT run_node_id FROM work_items WHERE id=$2)
			AND rps.schedule_code='LICH-THU-TIEN-001'
			AND rps.milestone_name='Thu tiền đầu ra sau nghiệm thu'
			AND rps.status='draft'
			AND rps.planned_amount=180000000
			AND rps.remaining_amount=180000000
	`, 1, fixture.projectID, workItemID)
	assertCount(t, db, `
		SELECT COUNT(*)
		FROM output_invoices oi
		JOIN receivables rec ON rec.id=oi.receivable_id
		JOIN acceptance_records acc ON acc.id=rec.source_id
		WHERE acc.project_id=$1
			AND acc.node_id=(SELECT run_node_id FROM work_items WHERE id=$2)
	`, 0, fixture.projectID, workItemID)
	assertCount(t, db, `
		SELECT COUNT(*)
		FROM workflow_automation_events
		WHERE source_type='pipeline_run_node'
			AND source_id=(SELECT run_node_id FROM work_items WHERE id=$1)
			AND action_key='finance.scaffold_outgoing_acceptance'
			AND status='blocked'
			AND target_type='receivable'
			AND blocker_reason='Cần tải lên và phát hành hóa đơn đầu ra thật'
			AND run_count=$2
	`, 1, workItemID, 1)
	assertCount(t, db, `
		SELECT COUNT(*)
		FROM work_item_activities wia
		JOIN work_items wi ON wi.id=wia.work_item_id
		WHERE wi.run_node_id=(SELECT run_node_id FROM work_items WHERE id=$1)
			AND activity_type='blocker'
			AND comment='Cần tải lên và phát hành hóa đơn đầu ra thật'
			AND metadata_json->>'automation_action'='finance.scaffold_outgoing_acceptance'
	`, 1, workItemID)
}

func TestQualityITPRFINCRReinspectionAndRecurringLifecycle(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")
	fixture := setupWorkItemFlowFixture(t, db, false)

	requestEvidence, err := service.AddWorkItemEvidence(ctx, fixture.itemID, WorkItemEvidenceRequest{
		EvidenceType: "photo", Name: "Ảnh trước kiểm tra", FileURL: "/evidence/quality-request.jpg", Visibility: "project",
	}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	inspectionEvidence, err := service.AddWorkItemEvidence(ctx, fixture.itemID, WorkItemEvidenceRequest{
		EvidenceType: "photo", Name: "Ảnh inspection fail", FileURL: "/evidence/quality-fail.jpg", Visibility: "project",
	}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	correctiveEvidence, err := service.AddWorkItemEvidence(ctx, fixture.itemID, WorkItemEvidenceRequest{
		EvidenceType: "photo", Name: "Ảnh corrective", FileURL: "/evidence/quality-corrective.jpg", Visibility: "project",
	}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	passEvidence, err := service.AddWorkItemEvidence(ctx, fixture.itemID, WorkItemEvidenceRequest{
		EvidenceType: "photo", Name: "Ảnh tái kiểm tra đạt", FileURL: "/evidence/quality-pass.jpg", Visibility: "project",
	}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}

	workspace, err := service.CreateQualityITP(ctx, fixture.projectID, QualityITPRequest{
		ITPCode: "ITP-MEP-01", Title: "ITP lắp đặt MEP", Discipline: "MEP", WorkPackage: "Điện âm trần",
		Items: []QualityITPItemRequest{{SequenceNo: 1, ActivityName: "Kiểm tra tuyến ống", InspectionStage: "Trước khi đóng trần", AcceptanceCriteria: "Đúng bản vẽ và cao độ", InspectionMethod: "Đo và kiểm tra trực quan", ControlPoint: "hold", FrequencyType: "each_lot", FrequencyValue: 1, ResponsibleRole: "QA/QC", EvidenceRequired: true, ClientWitnessRequired: true}},
	}, fixture.pmID, "PM")
	if err != nil {
		t.Fatal(err)
	}
	itps := workspace["itp_plans"].([]Record)
	itpID := cleanID(itps[0]["id"])
	itpItems := itps[0]["items"].([]Record)
	itpItemID := cleanID(itpItems[0]["id"])
	if _, err = service.QualityITPAction(ctx, itpID, "submit", "Gửi review", fixture.pmID, "PM"); err != nil {
		t.Fatal(err)
	}
	if _, err = service.QualityITPAction(ctx, itpID, "approve", "Đã review tiêu chí", fixture.pmID, "PM"); err != nil {
		t.Fatal(err)
	}
	if _, err = service.QualityITPAction(ctx, itpID, "activate", "Áp dụng tại hiện trường", fixture.pmID, "PM"); err != nil {
		t.Fatal(err)
	}

	workspace, err = service.CreateQualityInspectionRequest(ctx, fixture.projectID, QualityInspectionRequest{
		RFICode: "RFI-001", ITPItemID: itpItemID, WorkItemID: fixture.itemID, Location: "Tầng 2 trục A", LotReference: "LOT-MEP-01",
		RequestedInspectionAt: time.Now().Add(time.Hour).UTC().Format(time.RFC3339), RequestComment: "Mời kiểm tra trước đóng trần", RequestEvidenceID: cleanID(requestEvidence["id"]),
	}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	rfis := workspace["inspection_requests"].([]Record)
	rfiID := cleanID(rfis[0]["id"])
	if _, err = service.QualityInspectionAction(ctx, rfiID, "submit", QualityInspectionDecisionRequest{}, fixture.workerID, "Worker"); err != nil {
		t.Fatal(err)
	}
	workspace, err = service.QualityInspectionAction(ctx, rfiID, "inspect", QualityInspectionDecisionRequest{Result: "fail", Comment: "Sai cao độ tuyến ống", InspectionEvidenceID: cleanID(inspectionEvidence["id"])}, fixture.pmID, "PM")
	if err != nil {
		t.Fatal(err)
	}
	ncrs := workspace["ncrs"].([]Record)
	if len(ncrs) != 1 || ncrs[0]["status"] != "open" || cleanID(ncrs[0]["rework_request_id"]) == "" {
		t.Fatalf("auto NCR = %#v", ncrs)
	}
	ncrID := cleanID(ncrs[0]["id"])
	reworkID := cleanID(ncrs[0]["rework_request_id"])
	if _, err = service.QualityNCRAction(ctx, ncrID, "root-cause", QualityNCRActionRequest{RootCause: "Sai mốc chuẩn", PreventiveAction: "Bổ sung checklist cao độ"}, fixture.pmID, "PM"); err != nil {
		t.Fatal(err)
	}
	if _, err = service.QualityNCRAction(ctx, ncrID, "corrective", QualityNCRActionRequest{CorrectiveAction: "Tháo và lắp lại tuyến ống", CorrectiveEvidenceID: cleanID(correctiveEvidence["id"])}, fixture.workerID, "Worker"); err != nil {
		t.Fatal(err)
	}
	workspace, err = service.CreateQualityInspectionRequest(ctx, fixture.projectID, QualityInspectionRequest{
		RFICode: "RFI-002", ITPItemID: itpItemID, WorkItemID: fixture.itemID, Location: "Tầng 2 trục A", LotReference: "LOT-MEP-01-RE",
		RequestedInspectionAt: time.Now().Add(2 * time.Hour).UTC().Format(time.RFC3339), RequestComment: "Tái kiểm tra sau rework", RequestEvidenceID: cleanID(correctiveEvidence["id"]),
	}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	rfis = workspace["inspection_requests"].([]Record)
	var reinspectionID string
	for _, row := range rfis {
		if row["rfi_code"] == "RFI-002" {
			reinspectionID = cleanID(row["id"])
		}
	}
	if _, err = service.QualityInspectionAction(ctx, reinspectionID, "submit", QualityInspectionDecisionRequest{}, fixture.workerID, "Worker"); err != nil {
		t.Fatal(err)
	}
	if _, err = service.QualityInspectionAction(ctx, reinspectionID, "inspect", QualityInspectionDecisionRequest{Result: "pass", Comment: "Đạt sau rework", InspectionEvidenceID: cleanID(passEvidence["id"])}, fixture.pmID, "PM"); err != nil {
		t.Fatal(err)
	}
	if _, err = service.QualityNCRAction(ctx, ncrID, "reinspection", QualityNCRActionRequest{ReinspectionRFIID: reinspectionID}, fixture.pmID, "PM"); err != nil {
		t.Fatal(err)
	}
	if _, err = service.UpdateReworkStatus(ctx, reworkID, "approved", fixture.pmID, "PM"); err != nil {
		t.Fatal(err)
	}
	if _, err = service.UpdateReworkStatus(ctx, reworkID, "done", fixture.pmID, "PM"); err != nil {
		t.Fatal(err)
	}
	if _, err = service.QualityNCRAction(ctx, ncrID, "close", QualityNCRActionRequest{Comment: "Đã tái kiểm tra đạt và đóng NCR"}, fixture.pmID, "PM"); err != nil {
		t.Fatal(err)
	}

	workspace, err = service.CreateRecurringPlan(ctx, fixture.projectID, ConstructionRecurringPlanRequest{
		PlanCode: "QA-DAILY-01", Title: "QA walkdown hằng ngày", Description: "Kiểm tra hiện trường cuối ngày", RecurrenceType: "daily", IntervalValue: 1,
		StartDate: time.Now().Format("2006-01-02"), EndDate: time.Now().AddDate(0, 0, 1).Format("2006-01-02"), ResponsibleRole: "QA/QC", AssignedUserID: fixture.pmID, EvidenceRequired: true, SourceWorkItemID: fixture.itemID,
	}, fixture.pmID, "PM")
	if err != nil {
		t.Fatal(err)
	}
	plans := workspace["recurring_plans"].([]Record)
	planID := cleanID(plans[0]["id"])
	workspace, err = service.GenerateRecurringOccurrences(ctx, planID, time.Now().AddDate(0, 0, 1).Format("2006-01-02"), fixture.pmID, "PM")
	if err != nil {
		t.Fatal(err)
	}
	occurrences := workspace["recurring_occurrences"].([]Record)
	if len(occurrences) != 2 {
		t.Fatalf("occurrences = %d, want 2", len(occurrences))
	}
	if _, err = service.RecurringOccurrenceAction(ctx, cleanID(occurrences[0]["id"]), "complete", ConstructionRecurringOccurrenceRequest{Comment: "Đã walkdown", EvidenceID: cleanID(passEvidence["id"])}, fixture.pmID, "PM"); err != nil {
		t.Fatal(err)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM quality_ncrs WHERE id=$1 AND status='closed'`, 1, ncrID)
	assertCount(t, db, `SELECT COUNT(*) FROM rework_requests WHERE id=$1 AND status='done'`, 1, reworkID)
	assertCount(t, db, `SELECT COUNT(*) FROM construction_recurring_occurrences WHERE plan_id=$1 AND status='completed'`, 1, planID)
	assertCount(t, db, `SELECT COUNT(*) FROM audit_logs WHERE project_id=$1 AND event_type IN ('quality_itp_activated','quality_rfi_inspected','quality_ncr_created','quality_ncr_closed','recurring_occurrences_generated','recurring_occurrence_completed')`, 7, fixture.projectID)
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

func TestBatch7HandoverBlockedAuditIncludesProjectFinanceBlockers(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)

	actorID := newID()
	projectID := newID()
	acceptanceID := newID()
	handoverID := newID()
	materialCostID := newID()
	cashRequestID := newID()
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM project_cash_requests WHERE id=$1`, cashRequestID)
		_, _ = db.ExecContext(ctx, `DELETE FROM project_material_costs WHERE id=$1`, materialCostID)
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
			(id, project_id, handover_code, status, final_acceptance_id, documents_ready, warehouse_closed, payables_recorded, receivables_recorded, created_by)
		VALUES ($1,$2,$3,'submitted',$4,true,true,true,true,$5)
	`, handoverID, projectID, "HO-B7-"+handoverID[:8], acceptanceID, actorID)
	mustExec(t, db, `
		INSERT INTO project_material_costs (id,project_id,vendor_name,description,amount_before_vat,vat_amount,total_amount,status,created_by)
		VALUES ($1,$2,'Batch7 Supplier','Vật tư chưa xác nhận',100,10,110,'submitted',$3)
	`, materialCostID, projectID, actorID)
	mustExec(t, db, `
		INSERT INTO project_cash_requests (id,project_id,request_code,direction,title,counterparty_name,amount,status,created_by)
		VALUES ($1,$2,$3,'outbound','Đề nghị chi chưa xử lý','Batch7 Supplier',110,'submitted',$4)
	`, cashRequestID, projectID, "B7R-"+cashRequestID[:8], actorID)

	blocked, err := repo.CompleteHandover(ctx, handoverID, ActionRequest{Reason: "Kiểm tra bàn giao"}, actorID, "Batch7 Actor")
	if err != nil {
		t.Fatal(err)
	}
	if blocked["status"] != "blocked" || blocked["ready"] != false {
		t.Fatalf("handover = status %v ready %v, want blocked/false", blocked["status"], blocked["ready"])
	}
	blockers, _ := blocked["blockers"].([]string)
	if !containsString(blockers, "Còn chi phí vật tư chưa được xác nhận") || !containsString(blockers, "Còn đề nghị thu/chi nội bộ chưa được xử lý") {
		t.Fatalf("blockers = %#v, want project-finance blockers", blockers)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM audit_logs WHERE project_id=$1 AND event_type='handover_blocked'`, 1, projectID)
}

func TestRev8PhysicalHandoverCompletesBeforeProjectFinanceClose(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)

	actorID := newID()
	staffID := newID()
	projectID := newID()
	acceptanceID := newID()
	handoverID := newID()
	materialCostID := newID()
	cashRequestID := newID()
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM projects WHERE id=$1`, projectID)
		_, _ = db.ExecContext(ctx, `DELETE FROM staff WHERE id=$1`, staffID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id=$1`, actorID)
	})
	mustExec(t, db, `INSERT INTO users (id,email,password_hash,full_name,status) VALUES ($1,$2,'demo:pw','Rev8 Handover Actor','active')`, actorID, "rev8-handover-"+actorID+"@test.local")
	mustExec(t, db, `INSERT INTO staff (id,user_id,full_name,email,department,status) VALUES ($1,$2,'Rev8 Handover Actor',$3,'operations','active')`, staffID, actorID, "rev8-handover-staff-"+actorID+"@test.local")
	mustExec(t, db, `INSERT INTO projects (id,name,code,client_name,status,pipeline_template_id) VALUES ($1,'Rev8 Handover',$2,'Client','handover','40000000-0000-0000-0000-000000000027')`, projectID, "R8H-"+projectID[:8])
	mustExec(t, db, `INSERT INTO project_members (project_id,staff_id,role_ids,active) VALUES ($1,$2,'["PM"]'::jsonb,true)`, projectID, staffID)
	if _, err := repo.StartPipeline(ctx, projectID, "40000000-0000-0000-0000-000000000027", actorID, "Rev8 Handover Actor"); err != nil {
		t.Fatal(err)
	}
	mustExec(t, db, `
		INSERT INTO acceptance_records (id,project_id,type,title,requested_by,approved_by,status,result)
		VALUES ($1,$2,'final','Rev8 final acceptance',$3,$3,'completed','pass')
	`, acceptanceID, projectID, actorID)
	mustExec(t, db, `
		INSERT INTO handover_records
			(id,project_id,handover_code,status,final_acceptance_id,documents_ready,warehouse_closed,payables_recorded,receivables_recorded,created_by)
		VALUES ($1,$2,$3,'submitted',$4,true,true,false,false,$5)
	`, handoverID, projectID, "HO-R8-"+handoverID[:8], acceptanceID, actorID)
	mustExec(t, db, `
		INSERT INTO project_material_costs (id,project_id,vendor_name,description,amount_before_vat,vat_amount,total_amount,status,created_by)
		VALUES ($1,$2,'Rev8 Supplier','Chi phí chờ xác nhận',100,10,110,'submitted',$3)
	`, materialCostID, projectID, actorID)
	mustExec(t, db, `
		INSERT INTO project_cash_requests (id,project_id,request_code,direction,title,counterparty_name,amount,status,created_by)
		VALUES ($1,$2,$3,'outbound','Đề nghị chi chờ xử lý','Rev8 Supplier',110,'submitted',$4)
	`, cashRequestID, projectID, "R8R-"+cashRequestID[:8], actorID)

	completed, err := repo.CompleteHandover(ctx, handoverID, ActionRequest{Comment: "Bàn giao vật lý"}, actorID, "Rev8 Handover Actor")
	if err != nil {
		t.Fatal(err)
	}
	if completed["status"] != "handed_over" {
		t.Fatalf("Rev8 handover status=%v, want handed_over despite pending finance", completed["status"])
	}

	financeWorkItemID := clientOfficialWorkItemIDByAction(t, db, projectID, "source.project_finance_close_ready")
	source, err := repo.WorkflowBusinessSourceForWorkItem(ctx, financeWorkItemID)
	if err != nil {
		t.Fatal(err)
	}
	if boolValue(source["ready"]) || source["status"] != "blocked" {
		t.Fatalf("Rev8 finance source=%#v, want blocked after physical handover", source)
	}
	checks, ok := source["checks"].([]Record)
	if !ok || len(checks) != 3 {
		t.Fatalf("Rev8 finance checks=%#v, want 3 composite checks", source["checks"])
	}
	if !boolValue(checks[0]["ready"]) || boolValue(checks[1]["ready"]) || boolValue(checks[2]["ready"]) {
		t.Fatalf("Rev8 finance check states=%#v, want handover ready and finance pending", checks)
	}
}

func TestOutgoingAcceptanceSubmitLocksAndFeedsReworkAndHandover(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")
	fixture := setupWorkItemFlowFixture(t, db, false)

	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM handover_records WHERE project_id=$1`, fixture.projectID)
		_, _ = db.ExecContext(ctx, `DELETE FROM documents WHERE project_id=$1`, fixture.projectID)
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
	secondEvidence, err := service.AddWorkItemEvidence(ctx, fixture.itemID, WorkItemEvidenceRequest{
		EvidenceType: "document",
		Name:         "Outgoing inspection record",
		FileURL:      "/evidence/outgoing-inspection.pdf",
		Visibility:   "project",
	}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	secondEvidenceID := fmt.Sprint(secondEvidence["id"])

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
			{ItemIndex: 1, Title: "Vệ sinh khu vực", Decision: "fail", Severity: "high", EvidenceID: evidenceID, EvidenceIDs: []string{evidenceID, secondEvidenceID}, RequiredAction: "Dọn lại khu vực nghiệm thu"},
		},
	}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	if draft["status"] != "draft" {
		t.Fatalf("draft status = %v, want draft", draft["status"])
	}
	assertCount(t, db, `
		SELECT COUNT(*)
		FROM acceptance_checklist_item_evidence link
		JOIN acceptance_checklist_items item ON item.id=link.checklist_item_id
		WHERE item.acceptance_id=$1 AND item.item_index=1
	`, 2, draft["id"])
	checklist, ok := draft["checklist"].([]Record)
	if !ok || len(checklist) != 2 {
		t.Fatalf("draft checklist = %#v, want two items", draft["checklist"])
	}
	linkedEvidence, ok := checklist[1]["evidence_records"].([]Record)
	if !ok || len(linkedEvidence) != 2 {
		t.Fatalf("linked evidence = %#v, want two records", checklist[1]["evidence_records"])
	}
	if cleanID(checklist[1]["evidence_id"]) != evidenceID {
		t.Fatalf("compatibility evidence_id = %v, want %s", checklist[1]["evidence_id"], evidenceID)
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
	if _, err := service.UpdateReworkStatus(ctx, cleanID(rework["id"]), "done", fixture.workerID, "Worker"); err != nil {
		t.Fatal(err)
	}
	acceptedEvidence, err := service.EvidenceAction(ctx, evidenceID, "accept", ActionRequest{Comment: "Dùng cho hồ sơ bàn giao"}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	handover, err := service.CreateHandover(ctx, fixture.projectID, HandoverCreateRequest{
		FinalAcceptanceID: cleanID(finalPassed["id"]), ClientRepresentative: "Owner rep", CompanyRepresentative: "Contractor rep",
	}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	handoverID := cleanID(handover["id"])
	linkedDocumentID := newID()
	mustExec(t, db, `INSERT INTO documents (id, project_id, name, file_url, document_type, status, uploaded_by) VALUES ($1,$2,'Approved as-built drawing','/documents/as-built.pdf','drawing','active',$3)`, linkedDocumentID, fixture.projectID, fixture.workerID)
	manualItemIndex := 0
	for _, raw := range handover["checklist_items"].([]Record) {
		if raw["source_type"] == nil || strings.TrimSpace(fmt.Sprint(raw["source_type"])) == "" {
			values := Record{"status": "verified", "evidence_id": acceptedEvidence["id"]}
			if manualItemIndex == 0 {
				values = Record{"status": "verified", "document_id": linkedDocumentID}
			}
			if _, err := repo.UpdateHandoverChecklistItem(ctx, cleanID(raw["id"]), values, fixture.workerID, "Worker"); err != nil {
				t.Fatal(err)
			}
			manualItemIndex++
		}
	}
	linkedDetail, err := repo.GetHandoverDetail(ctx, handoverID)
	if err != nil {
		t.Fatal(err)
	}
	linkedSourceFound := false
	for _, raw := range linkedDetail["checklist_items"].([]Record) {
		if cleanID(raw["evidence_id"]) != evidenceID {
			continue
		}
		linkedSourceFound = true
		if raw["evidence_name"] != "Outgoing defect photo" || raw["evidence_type"] != "photo" || raw["evidence_status"] != "accepted" {
			t.Fatalf("handover linked source metadata = name %v type %v status %v, want accepted photo evidence", raw["evidence_name"], raw["evidence_type"], raw["evidence_status"])
		}
		break
	}
	if !linkedSourceFound {
		t.Fatal("handover detail did not expose the exact linked evidence source")
	}
	linkedDocumentFound := false
	for _, raw := range linkedDetail["checklist_items"].([]Record) {
		if cleanID(raw["document_id"]) != linkedDocumentID {
			continue
		}
		linkedDocumentFound = true
		if raw["document_name"] != "Approved as-built drawing" || raw["document_type"] != "drawing" || raw["document_status"] != "active" || raw["document_file_url"] != "/documents/as-built.pdf" {
			t.Fatalf("handover linked document metadata = name %v type %v status %v url %v, want active drawing source", raw["document_name"], raw["document_type"], raw["document_status"], raw["document_file_url"])
		}
		break
	}
	if !linkedDocumentFound {
		t.Fatal("handover detail did not expose the exact linked document source")
	}
	if _, err := repo.SubmitHandover(ctx, handoverID, ActionRequest{Comment: "Gửi kiểm tra"}, fixture.workerID, "Worker"); err != nil {
		t.Fatal(err)
	}
	check, err := repo.HandoverCheck(ctx, handoverID, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	if ready, ok := check["ready"].(bool); !ok || !ready {
		t.Fatalf("handover ready = %#v, blockers = %#v, want ready", check["ready"], check["blockers"])
	}
	completed, err := repo.CompleteHandover(ctx, handoverID, ActionRequest{Comment: "Hai bên xác nhận bàn giao"}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	if completed["status"] != "handed_over" || completed["completed_at"] == nil {
		t.Fatalf("handover completion = status %v completed_at %v, want handed_over timestamp", completed["status"], completed["completed_at"])
	}
	assertCount(t, db, `SELECT COUNT(*) FROM handover_checklist_items WHERE handover_id=$1 AND required AND status='verified'`, 8, handoverID)
	assertCount(t, db, `SELECT COUNT(*) FROM audit_logs WHERE project_id=$1 AND event_type IN ('handover_submitted','handover_ready','handover_completed')`, 4, fixture.projectID)
	assertCount(t, db, `SELECT COUNT(*) FROM acceptance_records WHERE project_id=$1 AND type='final' AND status='completed' AND result='pass'`, 1, fixture.projectID)
}
