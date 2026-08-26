package core

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWorkflowTimelineCountsSubmittedAndSkippedSteps(t *testing.T) {
	run := Record{"nodes": []Record{
		{
			"id": "completed-node", "name": "Khảo sát", "status": "completed",
			"steps": []Record{{"status": "submitted"}, {"status": "completed"}},
		},
		{
			"id": "skipped-node", "name": "BOQ không thiết kế", "status": "skipped",
			"steps": []Record{{"status": "skipped"}, {"status": "cancelled"}},
		},
	}}

	timeline := workflowTimeline(run)
	if len(timeline) != 2 {
		t.Fatalf("workflowTimeline returned %d nodes, want 2", len(timeline))
	}
	if number(timeline[0]["completed_steps"]) != 2 || number(timeline[0]["progress_percent"]) != 100 {
		t.Fatalf("completed node summary = %#v, want 2/2 and 100%%", timeline[0])
	}
	if number(timeline[1]["completed_steps"]) != 0 || number(timeline[1]["skipped_steps"]) != 2 || number(timeline[1]["progress_percent"]) != 0 {
		t.Fatalf("skipped node summary = %#v, want 0 completed, 2 skipped and 0%%", timeline[1])
	}
}

func TestWorkItemCompletionKeepsLinkedRunStepTerminalStateAtomic(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	service := NewService(NewRepository(db), "test-secret")
	fixture := setupWorkItemFlowFixture(t, db, false)

	runID := newID()
	nodeID := newID()
	stepID := newID()
	mustExec(t, db, `
		INSERT INTO pipeline_runs (id,project_id,template_id,status,engine_version)
		VALUES ($1,$2,'40000000-0000-0000-0000-000000000021','in_progress',2)
	`, runID, fixture.projectID)
	mustExec(t, db, `
		INSERT INTO pipeline_run_nodes (id,run_id,project_id,name,type,order_index,status,started_at)
		VALUES ($1,$2,$3,'Node đồng bộ vòng đời','task',1,'in_progress',now())
	`, nodeID, runID, fixture.projectID)
	mustExec(t, db, `
		INSERT INTO pipeline_run_steps (id,run_node_id,project_id,name,order_index,status)
		VALUES ($1,$2,$3,'Bước đồng bộ vòng đời',1,'pending')
	`, stepID, nodeID, fixture.projectID)
	mustExec(t, db, `UPDATE pipeline_runs SET current_node_id=$1 WHERE id=$2`, nodeID, runID)
	mustExec(t, db, `UPDATE projects SET current_pipeline_run_id=$1 WHERE id=$2`, runID, fixture.projectID)
	mustExec(t, db, `
		UPDATE work_items
		SET pipeline_run_id=$2,run_node_id=$3,run_step_id=$4,status='ready'
		WHERE id=$1
	`, fixture.itemID, runID, nodeID, stepID)

	completed, err := service.WorkItemAction(ctx, fixture.itemID, "complete", WorkItemActionRequest{Comment: "Hoàn tất đúng vòng đời"}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(completed["status"]) != "completed" {
		t.Fatalf("work item status = %v, want completed", completed["status"])
	}
	assertCount(t, db, `
		SELECT COUNT(*)
		FROM pipeline_run_steps rs
		JOIN work_items wi ON wi.run_step_id=rs.id
		WHERE rs.id=$1 AND rs.status='completed' AND wi.status='completed'
	`, 1, stepID)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE pipeline_run_steps SET status='submitted' WHERE id=$1`, stepID); err != nil {
		t.Fatal(err)
	}
	if err := ensureRunStepWorkItemCompletionSyncTx(ctx, tx, stepID); err == nil || !strings.Contains(err.Error(), "workflow completion state mismatch") {
		t.Fatalf("completion postcondition error = %v, want fail-closed mismatch", err)
	}
}

func TestWorkItemCanStartOutOfSequenceWithoutMovingRunPointer(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	service := NewService(NewRepository(db), "test-secret")
	fixture := setupWorkItemFlowFixture(t, db, false)

	templateID := newID()
	runID := newID()
	currentNodeID := newID()
	futureNodeID := newID()
	currentStepID := newID()
	futureStepID := newID()
	futureWorkItemID := newID()
	defer func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM projects WHERE id=$1`, fixture.projectID)
		_, _ = db.ExecContext(ctx, `DELETE FROM pipeline_templates WHERE id=$1`, templateID)
	}()

	var workerStaffID string
	if err := db.QueryRowContext(ctx, `SELECT id::text FROM staff WHERE user_id=$1`, fixture.workerID).Scan(&workerStaffID); err != nil {
		t.Fatal(err)
	}
	mustExec(t, db, `INSERT INTO pipeline_templates (id,template_family_id,name,status) VALUES ($1,$1,'Early start test','published')`, templateID)
	mustExec(t, db, `UPDATE projects SET pipeline_template_id=$2 WHERE id=$1`, fixture.projectID, templateID)
	mustExec(t, db, `INSERT INTO pipeline_runs (id,project_id,template_id,engine_version,status) VALUES ($1,$2,$3,4,'active')`, runID, fixture.projectID, templateID)
	mustExec(t, db, `
		INSERT INTO pipeline_run_nodes (id,run_id,project_id,name,type,order_index,status)
		VALUES ($1,$3,$4,'Node hiện tại','task',1,'in_progress'),($2,$3,$4,'Node tương lai','task',2,'pending')
	`, currentNodeID, futureNodeID, runID, fixture.projectID)
	mustExec(t, db, `UPDATE pipeline_runs SET current_node_id=$1 WHERE id=$2`, currentNodeID, runID)
	mustExec(t, db, `UPDATE projects SET current_pipeline_run_id=$1 WHERE id=$2`, runID, fixture.projectID)
	mustExec(t, db, `
		INSERT INTO pipeline_run_steps (id,run_node_id,project_id,name,order_index,status)
		VALUES ($1,$3,$4,'Bước hiện tại',1,'in_progress'),($2,$5,$4,'Bước tương lai',1,'pending')
	`, currentStepID, futureStepID, currentNodeID, fixture.projectID, futureNodeID)
	mustExec(t, db, `
		INSERT INTO pipeline_run_edges (run_id,project_id,from_run_node_id,to_run_node_id,condition_type,priority)
		VALUES ($1,$2,$3,$4,'always',1)
	`, runID, fixture.projectID, currentNodeID, futureNodeID)
	mustExec(t, db, `
		UPDATE work_items
		SET pipeline_run_id=$2,run_node_id=$3,run_step_id=$4,status='ready'
		WHERE id=$1
	`, fixture.itemID, runID, currentNodeID, currentStepID)
	mustExec(t, db, `
		INSERT INTO work_items (
			id,project_id,pipeline_run_id,run_node_id,run_step_id,source_type,source_id,title,work_type,status,
			role_ids,assignee_staff_id,evidence_requirement_json,created_by,updated_by
		)
		VALUES ($1,$2,$3,$4,$5,'pipeline_run_step',$5,'Công việc tương lai','task','pending',
			'["Đội thi công"]'::jsonb,$6,'{"required_permission":"operation.report"}'::jsonb,$7,$7)
	`, futureWorkItemID, fixture.projectID, runID, futureNodeID, futureStepID, workerStaffID, fixture.workerID)

	detail, err := service.WorkItemDetail(ctx, futureWorkItemID, fixture.workerID)
	if err != nil {
		t.Fatal(err)
	}
	startReadiness := recordFromAny(detail["start_readiness"])
	if fmt.Sprint(startReadiness["state"]) != workItemStartRequirementsUnmet || !boolValue(startReadiness["can_bypass"]) {
		t.Fatalf("start readiness = %#v", startReadiness)
	}
	requirements, ok := startReadiness["requirements"].([]Record)
	if !ok || len(requirements) != 2 {
		t.Fatalf("requirements = %#v, want transition plus current work item", startReadiness["requirements"])
	}
	if !containsMyWorkString(stringSlice(detail["available_actions"]), "start") {
		t.Fatalf("available actions = %#v, want start", detail["available_actions"])
	}

	if _, err := service.WorkItemAction(ctx, futureWorkItemID, "start", WorkItemActionRequest{}, fixture.workerID, "Worker"); err == nil {
		t.Fatal("future start without explicit bypass confirmation must fail")
	}
	if _, err := service.WorkItemAction(ctx, futureWorkItemID, "start", WorkItemActionRequest{
		BypassStartRequirements: true,
		StartRequirementsToken:  "stale-token",
		Reason:                  "Triển khai song song",
	}, fixture.workerID, "Worker"); err == nil {
		t.Fatal("future start with stale snapshot must fail")
	}

	updated, err := service.WorkItemAction(ctx, futureWorkItemID, "start", WorkItemActionRequest{
		BypassStartRequirements: true,
		StartRequirementsToken:  fmt.Sprint(startReadiness["snapshot_token"]),
		Reason:                  "Triển khai song song để kịp tiến độ",
	}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(updated["status"]) != "in_progress" {
		t.Fatalf("updated status = %v", updated["status"])
	}
	var currentNodeAfter string
	if err := db.QueryRowContext(ctx, `SELECT current_node_id::text FROM pipeline_runs WHERE id=$1`, runID).Scan(&currentNodeAfter); err != nil {
		t.Fatal(err)
	}
	if currentNodeAfter != currentNodeID {
		t.Fatalf("run pointer moved to %s, want %s", currentNodeAfter, currentNodeID)
	}

	after, err := service.WorkItemDetail(ctx, futureWorkItemID, fixture.workerID)
	if err != nil {
		t.Fatal(err)
	}
	if !boolValue(after["is_early_execution"]) || fmt.Sprint(after["readiness"]) != "actionable" {
		t.Fatalf("early execution detail = %#v", after)
	}
	var activityOverrideMode, activityReason string
	if err := db.QueryRowContext(ctx, `
		SELECT metadata_json->'start_override'->>'mode', metadata_json->'start_override'->>'reason'
		FROM work_item_activities
		WHERE work_item_id=$1 AND activity_type='started'
		ORDER BY created_at DESC LIMIT 1
	`, futureWorkItemID).Scan(&activityOverrideMode, &activityReason); err != nil {
		t.Fatal(err)
	}
	if activityOverrideMode != "out_of_sequence" || activityReason != "Triển khai song song để kịp tiến độ" {
		t.Fatalf("activity override = %q/%q", activityOverrideMode, activityReason)
	}
	var auditActorID, auditOverrideMode, auditReason, auditDescription string
	if err := db.QueryRowContext(ctx, `
		SELECT actor_id::text, metadata_json->'start_override'->>'mode',
			metadata_json->'start_override'->>'reason', description
		FROM audit_logs
		WHERE target_type='work_item' AND target_id=$1 AND event_type='work_item_started'
		ORDER BY created_at DESC LIMIT 1
	`, futureWorkItemID).Scan(&auditActorID, &auditOverrideMode, &auditReason, &auditDescription); err != nil {
		t.Fatal(err)
	}
	if auditActorID != fixture.workerID || auditOverrideMode != "out_of_sequence" || auditReason != activityReason || !strings.Contains(auditDescription, "Bắt đầu sớm") {
		t.Fatalf("audit = actor %q, mode %q, reason %q, description %q", auditActorID, auditOverrideMode, auditReason, auditDescription)
	}
}

func TestWorkflowFormFirstWriteStartsCurrentWorkItemAtomicallyOnce(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	service := NewService(NewRepository(db), "test-secret")
	fixture := setupWorkItemFlowFixture(t, db, false)

	templateID := newID()
	runID := newID()
	runNodeID := newID()
	runStepID := newID()
	instanceID := newID()
	defer func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM projects WHERE id=$1`, fixture.projectID)
		_, _ = db.ExecContext(ctx, `DELETE FROM pipeline_templates WHERE id=$1`, templateID)
	}()

	mustExec(t, db, `INSERT INTO pipeline_templates (id,template_family_id,name,status) VALUES ($1,$1,'Workflow form start test','published')`, templateID)
	mustExec(t, db, `UPDATE projects SET pipeline_template_id=$2 WHERE id=$1`, fixture.projectID, templateID)
	mustExec(t, db, `INSERT INTO pipeline_runs (id,project_id,template_id,engine_version,status) VALUES ($1,$2,$3,4,'active')`, runID, fixture.projectID, templateID)
	mustExec(t, db, `
		INSERT INTO pipeline_run_nodes (id,run_id,project_id,name,type,order_index,status,started_at)
		VALUES ($1,$2,$3,'Node biểu mẫu','construction_planning',1,'in_progress',now())
	`, runNodeID, runID, fixture.projectID)
	mustExec(t, db, `UPDATE pipeline_runs SET current_node_id=$1 WHERE id=$2`, runNodeID, runID)
	mustExec(t, db, `UPDATE projects SET current_pipeline_run_id=$1 WHERE id=$2`, runID, fixture.projectID)
	mustExec(t, db, `
		INSERT INTO pipeline_run_steps (id,run_node_id,project_id,name,step_type,order_index,status)
		VALUES ($1,$2,$3,'Bước biểu mẫu','checklist',1,'pending')
	`, runStepID, runNodeID, fixture.projectID)
	mustExec(t, db, `
		UPDATE work_items
		SET pipeline_run_id=$2,run_node_id=$3,run_step_id=$4,status='ready',started_at=NULL
		WHERE id=$1
	`, fixture.itemID, runID, runNodeID, runStepID)
	mustExec(t, db, `
		INSERT INTO workflow_form_instances (
			id,project_id,work_item_id,pipeline_run_id,run_node_id,run_step_id,
			form_template_id,form_version_id,form_type,title,created_by
		)
		SELECT $1,$2,$3,$4,$5,$6,t.id,v.id,t.form_type,'Checklist bắt đầu đầu việc',$7
		FROM workflow_form_templates t
		JOIN workflow_form_versions v ON v.template_id=t.id
		WHERE t.code='builtin_checklist' AND v.version_no=1 AND v.status='published'
	`, instanceID, fixture.projectID, fixture.itemID, runID, runNodeID, runStepID, fixture.workerID)

	if _, err := service.SubmitWorkflowForm(ctx, instanceID, WorkflowFormSaveRequest{Answers: Record{}}, fixture.workerID, "Worker"); err == nil {
		t.Fatal("invalid submit must fail before starting the work item")
	}
	assertCount(t, db, `SELECT COUNT(*) FROM work_items WHERE id=$1 AND status='ready' AND started_at IS NULL`, 1, fixture.itemID)
	assertCount(t, db, `SELECT COUNT(*) FROM pipeline_run_steps WHERE id=$1 AND status='pending'`, 1, runStepID)
	assertCount(t, db, `SELECT COUNT(*) FROM work_item_activities WHERE work_item_id=$1 AND activity_type='started'`, 0, fixture.itemID)

	if _, err := service.SaveWorkflowFormDraft(ctx, instanceID, WorkflowFormSaveRequest{Answers: Record{}}, fixture.workerID, "Worker"); err != nil {
		t.Fatal(err)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM work_items WHERE id=$1 AND status='in_progress' AND started_at IS NOT NULL`, 1, fixture.itemID)
	assertCount(t, db, `SELECT COUNT(*) FROM pipeline_run_steps WHERE id=$1 AND status='in_progress'`, 1, runStepID)
	assertCount(t, db, `
		SELECT COUNT(*) FROM work_item_activities
		WHERE work_item_id=$1 AND activity_type='started'
		  AND from_status='ready' AND to_status='in_progress'
		  AND metadata_json->>'source'='workflow_form_first_write'
		  AND metadata_json->>'workflow_form_instance_id'=$2
	`, 1, fixture.itemID, instanceID)
	assertCount(t, db, `
		SELECT COUNT(*) FROM audit_logs
		WHERE target_type='work_item' AND target_id=$1 AND event_type='work_item_started'
		  AND metadata_json->>'source'='workflow_form_first_write'
	`, 1, fixture.itemID)

	if _, err := service.SaveWorkflowFormDraft(ctx, instanceID, WorkflowFormSaveRequest{Answers: Record{"exceptions": "Không có"}}, fixture.workerID, "Worker"); err != nil {
		t.Fatal(err)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM work_item_activities WHERE work_item_id=$1 AND activity_type='started'`, 1, fixture.itemID)
	assertCount(t, db, `SELECT COUNT(*) FROM audit_logs WHERE target_type='work_item' AND target_id=$1 AND event_type='work_item_started'`, 1, fixture.itemID)
}

func TestSkipUnselectedBranchesSkipsOnlyExclusiveSubgraph(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()

	exec := func(query string, args ...any) {
		t.Helper()
		if _, execErr := tx.ExecContext(ctx, query, args...); execErr != nil {
			t.Fatal(execErr)
		}
	}
	count := func(query string, args ...any) int {
		t.Helper()
		var total int
		if queryErr := tx.QueryRowContext(ctx, query, args...).Scan(&total); queryErr != nil {
			t.Fatal(queryErr)
		}
		return total
	}

	templateID := newID()
	projectID := newID()
	runID := newID()
	sourceID := newID()
	surveyID := newID()
	meetingID := newID()
	designID := newID()
	boqNoDesignID := newID()
	boqDesignID := newID()
	mergeID := newID()

	exec(`INSERT INTO pipeline_templates (id, template_family_id, name, status) VALUES ($1,$1,'Branch test','published')`, templateID)
	exec(`INSERT INTO projects (id, name, code, client_name, pipeline_template_id) VALUES ($1,'Branch test project',$2,'Client',$3)`, projectID, "BRANCH-"+projectID[:8], templateID)
	exec(`INSERT INTO pipeline_runs (id, project_id, template_id, engine_version) VALUES ($1,$2,$3,4)`, runID, projectID, templateID)

	nodes := []struct {
		id     string
		name   string
		order  int
		status string
	}{
		{sourceID, "Tiếp nhận dự án", 1, "completed"},
		{surveyID, "Khảo sát", 2, "pending"},
		{meetingID, "Họp báo cáo khảo sát", 3, "pending"},
		{designID, "Thiết kế", 4, "pending"},
		{boqNoDesignID, "BOQ (Không thiết kế)", 5, "pending"},
		{boqDesignID, "BOQ (Có thiết kế)", 6, "pending"},
		{mergeID, "Chốt hợp đồng", 7, "pending"},
	}
	for _, node := range nodes {
		exec(`
			INSERT INTO pipeline_run_nodes (id, run_id, project_id, name, order_index, status)
			VALUES ($1,$2,$3,$4,$5,$6)
		`, node.id, runID, projectID, node.name, node.order, node.status)
	}
	exec(`UPDATE pipeline_runs SET current_node_id=$1 WHERE id=$2`, sourceID, runID)
	exec(`UPDATE projects SET current_pipeline_run_id=$1 WHERE id=$2`, runID, projectID)

	edges := [][2]string{
		{sourceID, surveyID},
		{sourceID, boqNoDesignID},
		{surveyID, meetingID},
		{meetingID, designID},
		{meetingID, boqNoDesignID},
		{designID, boqDesignID},
		{boqNoDesignID, mergeID},
		{boqDesignID, mergeID},
	}
	for index, edge := range edges {
		exec(`
			INSERT INTO pipeline_run_edges (run_id, project_id, from_run_node_id, to_run_node_id, priority)
			VALUES ($1,$2,$3,$4,$5)
		`, runID, projectID, edge[0], edge[1], index+1)
	}

	var formTemplateID string
	var formVersionID string
	var formType string
	if err := tx.QueryRowContext(ctx, `
		SELECT template.id, version.id, template.form_type
		FROM workflow_form_templates template
		JOIN workflow_form_versions version ON version.template_id=template.id
		WHERE template.code='builtin_checklist' AND version.status='published'
		ORDER BY version.version_no DESC
		LIMIT 1
	`).Scan(&formTemplateID, &formVersionID, &formType); err != nil {
		t.Fatal(err)
	}
	for _, node := range nodes[1:] {
		stepID := newID()
		workItemID := newID()
		exec(`
			INSERT INTO pipeline_run_steps (id, run_node_id, project_id, name, order_index, status)
			VALUES ($1,$2,$3,$4,1,'pending')
		`, stepID, node.id, projectID, node.name+" step")
		exec(`
			INSERT INTO work_items (id, project_id, pipeline_run_id, run_node_id, run_step_id, source_id, title, status)
			VALUES ($1,$2,$3,$4,$5,$5,$6,'pending')
		`, workItemID, projectID, runID, node.id, stepID, node.name+" work")
		exec(`
			INSERT INTO workflow_form_instances (
				project_id, work_item_id, pipeline_run_id, run_node_id, run_step_id,
				form_template_id, form_version_id, form_type, title, status
			)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,'draft')
		`, projectID, workItemID, runID, node.id, stepID, formTemplateID, formVersionID, formType, node.name+" form")
	}

	fromNode := Record{"id": sourceID, "run_id": runID}
	if err := skipUnselectedBranchExclusiveNodesTx(ctx, tx, fromNode, surveyID); err != nil {
		t.Fatal(err)
	}
	if got := count(`SELECT COUNT(*) FROM pipeline_run_nodes WHERE run_id=$1 AND status='skipped'`, runID); got != 0 {
		t.Fatalf("survey route skipped nodes = %d, want 0 because BOQ remains reachable later", got)
	}

	if err := skipUnselectedBranchExclusiveNodesTx(ctx, tx, fromNode, boqNoDesignID); err != nil {
		t.Fatal(err)
	}

	exclusiveArgs := []any{surveyID, meetingID, designID, boqDesignID}
	if got := count(`SELECT COUNT(*) FROM pipeline_run_nodes WHERE id IN ($1,$2,$3,$4) AND status='skipped'`, exclusiveArgs...); got != 4 {
		t.Fatalf("skipped exclusive nodes = %d, want 4", got)
	}
	if got := count(`SELECT COUNT(*) FROM pipeline_run_nodes WHERE id IN ($1,$2) AND status='pending'`, boqNoDesignID, mergeID); got != 2 {
		t.Fatalf("pending selected/shared nodes = %d, want 2", got)
	}
	if got := count(`SELECT COUNT(*) FROM pipeline_run_steps WHERE run_node_id IN ($1,$2,$3,$4) AND status='skipped'`, exclusiveArgs...); got != 4 {
		t.Fatalf("skipped exclusive steps = %d, want 4", got)
	}
	if got := count(`SELECT COUNT(*) FROM work_items WHERE run_node_id IN ($1,$2,$3,$4) AND status='cancelled'`, exclusiveArgs...); got != 4 {
		t.Fatalf("cancelled exclusive work items = %d, want 4", got)
	}
	if got := count(`SELECT COUNT(*) FROM workflow_form_instances WHERE run_node_id IN ($1,$2,$3,$4) AND status='cancelled'`, exclusiveArgs...); got != 4 {
		t.Fatalf("cancelled exclusive forms = %d, want 4", got)
	}
	if got := count(`
		SELECT COUNT(*)
		FROM pipeline_run_steps
		WHERE run_node_id IN ($1,$2) AND status='pending'
	`, boqNoDesignID, mergeID); got != 2 {
		t.Fatalf("pending selected/shared steps = %d, want 2", got)
	}

	if err := skipUnselectedBranchExclusiveNodesTx(ctx, tx, fromNode, boqNoDesignID); err != nil {
		t.Fatal(err)
	}
	if got := count(`SELECT COUNT(*) FROM pipeline_run_nodes WHERE id IN ($1,$2,$3,$4) AND status='skipped'`, exclusiveArgs...); got != 4 {
		t.Fatalf("idempotent skipped nodes = %d, want 4", got)
	}
}

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

func TestEngineV2StepSubmitDoesNotSynthesizeLegacyChecklist(t *testing.T) {
	item := Record{
		"pipeline_engine_version":   2,
		"evidence_requirement_json": map[string]any{"checklist_required": true},
	}
	req := withLegacyChecklistCompletion(item, WorkItemActionRequest{})
	if hasChecklistPayload(req.CompletionPayload) {
		t.Fatalf("engine v2 must require its exact workflow form instead of a synthetic checklist: %#v", req.CompletionPayload)
	}
}

func TestWorkflowFormValidationNormalizesAndRejectsBadInput(t *testing.T) {
	service := NewService(&Repository{}, "test-secret")
	fields := []Record{
		{"field_key": "summary", "label": "Kết quả", "field_type": "textarea", "required": true, "validation_json": map[string]any{"min_length": 3}},
		{"field_key": "confirmed", "label": "Xác nhận", "field_type": "checkbox", "required": true, "validation_json": map[string]any{}},
		{"field_key": "quantity", "label": "Khối lượng", "field_type": "number", "required": false, "validation_json": map[string]any{"min": float64(0)}},
		{"field_key": "budget", "label": "Ngân sách", "field_type": "currency", "required": false, "validation_json": map[string]any{"min": float64(0), "currency_code": "VND"}},
	}

	_, err := service.validateWorkflowFormAnswers(context.Background(), Record{}, fields, Record{"summary": "ok", "extra": "bad"}, true)
	assertValidationError(t, err)

	_, err = service.validateWorkflowFormAnswers(context.Background(), Record{}, fields, Record{"summary": "ok", "confirmed": false}, true)
	assertValidationError(t, err)

	answers, err := service.validateWorkflowFormAnswers(context.Background(), Record{}, fields, Record{"summary": "  đã xong  ", "confirmed": true, "quantity": "12.5", "budget": "3100000000.50"}, true)
	if err != nil {
		t.Fatal(err)
	}
	if answers["summary"] != "đã xong" || answers["confirmed"] != true || number(answers["quantity"]) != 12.5 || number(answers["budget"]) != 3100000000.50 {
		t.Fatalf("normalized answers = %#v", answers)
	}
}

func TestWorkflowSelectOtherTextRegistryIsScoped(t *testing.T) {
	service := NewService(&Repository{}, "test-secret")
	tests := []struct {
		formCode string
		fieldKey string
	}{
		{formCode: "client_design_style_brief", fieldKey: "style_direction"},
		{formCode: "client_survey_work_package", fieldKey: "style_direction"},
		{formCode: "client_boq_owner_submission", fieldKey: "sent_channel"},
		{formCode: "client_hardcopy_dispatch", fieldKey: "delivery_method"},
		{formCode: "builtin_construction_daily_report", fieldKey: "construction_scope"},
		{formCode: "builtin_construction_daily_report", fieldKey: "weather"},
		{formCode: "builtin_subcontractor_checklist", fieldKey: "work_package"},
		{formCode: "client_subcontractor_acceptance_package", fieldKey: "work_package"},
		{formCode: "builtin_daily_report", fieldKey: "planning_scope"},
		{formCode: "builtin_checklist", fieldKey: "planning_scope"},
		{formCode: "client_owner_soft_file_submission", fieldKey: "delivery_channel"},
		{formCode: "client_procurement_takeoff_review", fieldKey: "takeoff_source"},
	}
	selectField := func(fieldKey string) Record {
		return Record{
			"field_key":  fieldKey,
			"label":      fieldKey,
			"field_type": "select",
			"required":   true,
			"options_json": []any{
				map[string]any{"value": "catalog", "label": "Trong danh mục"},
				map[string]any{"value": "other", "label": "Khác"},
			},
			"validation_json": map[string]any{},
		}
	}

	for _, tt := range tests {
		t.Run(tt.formCode+"."+tt.fieldKey, func(t *testing.T) {
			instance := Record{"form_code": tt.formCode}
			field := selectField(tt.fieldKey)
			draft, err := service.validateWorkflowFormAnswers(context.Background(), instance, []Record{field}, Record{tt.fieldKey: "other"}, false)
			if err != nil || draft[tt.fieldKey] != "other" {
				t.Fatalf("draft other = %#v, err=%v", draft, err)
			}
			_, err = service.validateWorkflowFormAnswers(context.Background(), instance, []Record{field}, Record{tt.fieldKey: "other"}, true)
			assertValidationError(t, err)
			answers, err := service.validateWorkflowFormAnswers(context.Background(), instance, []Record{field}, Record{tt.fieldKey: "  Giá trị tùy chỉnh  "}, true)
			if err != nil {
				t.Fatal(err)
			}
			if answers[tt.fieldKey] != "Giá trị tùy chỉnh" {
				t.Fatalf("custom answer = %#v", answers[tt.fieldKey])
			}
		})
	}

	styleField := selectField("style_direction")
	_, err := service.validateWorkflowFormAnswers(context.Background(), Record{"form_code": "client_survey_work_package"}, []Record{styleField}, Record{"style_direction": strings.Repeat("x", 301)}, true)
	assertValidationError(t, err)
	_, err = service.validateWorkflowFormAnswers(context.Background(), Record{"form_code": "unrelated_template"}, []Record{styleField}, Record{"style_direction": "custom"}, true)
	assertValidationError(t, err)
	unsupportedField := selectField("unsupported_field")
	_, err = service.validateWorkflowFormAnswers(context.Background(), Record{"form_code": "client_survey_work_package"}, []Record{unsupportedField}, Record{"unsupported_field": "custom"}, true)
	assertValidationError(t, err)
}

func TestWorkflowOtherDetailRulesRequireExistingFieldsOnSubmit(t *testing.T) {
	service := NewService(&Repository{}, "test-secret")
	ownerFields := []Record{
		{
			"field_key": "handover_scope", "label": "Phạm vi kiểm tra", "field_type": "select", "required": true,
			"options_json": []any{map[string]any{"value": "area"}, map[string]any{"value": "other"}}, "validation_json": map[string]any{},
		},
		{"field_key": "scope_detail", "label": "Chi tiết phạm vi", "field_type": "text", "required": false, "validation_json": map[string]any{"max_length": 240}},
	}
	ownerInstance := Record{"form_code": "builtin_owner_client_checklist"}
	draft, err := service.validateWorkflowFormAnswers(context.Background(), ownerInstance, ownerFields, Record{"handover_scope": "other"}, false)
	if err != nil || draft["handover_scope"] != "other" {
		t.Fatalf("owner draft = %#v, err=%v", draft, err)
	}
	_, err = service.validateWorkflowFormAnswers(context.Background(), ownerInstance, ownerFields, Record{"handover_scope": "other"}, true)
	assertValidationError(t, err)
	ownerAnswers, err := service.validateWorkflowFormAnswers(context.Background(), ownerInstance, ownerFields, Record{"handover_scope": "other", "scope_detail": "  Khu vực sân thượng  "}, true)
	if err != nil || ownerAnswers["scope_detail"] != "Khu vực sân thượng" {
		t.Fatalf("owner answers = %#v, err=%v", ownerAnswers, err)
	}

	materialField := Record{
		"field_key": "material_usage_items", "label": "Vật tư sử dụng", "field_type": "structured_list", "required": false,
		"validation_json": map[string]any{"item_fields": []any{
			map[string]any{"key": "source", "label": "Nguồn", "type": "select", "required": true, "options": []any{map[string]any{"value": "site_balance"}, map[string]any{"value": "other"}}},
			map[string]any{"key": "note", "label": "Ghi chú", "type": "textarea", "required": false},
		}},
	}
	constructionInstance := Record{"form_code": "builtin_construction_daily_report"}
	materialOther := Record{"material_usage_items": []any{map[string]any{"source": "other", "note": ""}}}
	if _, err = service.validateWorkflowFormAnswers(context.Background(), constructionInstance, []Record{materialField}, materialOther, false); err != nil {
		t.Fatalf("material draft err=%v", err)
	}
	_, err = service.validateWorkflowFormAnswers(context.Background(), constructionInstance, []Record{materialField}, materialOther, true)
	assertValidationError(t, err)
	materialAnswers, err := service.validateWorkflowFormAnswers(context.Background(), constructionInstance, []Record{materialField}, Record{
		"material_usage_items": []any{map[string]any{"source": "other", "note": "  Nhà cung cấp giao trực tiếp  "}},
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	materialRows := materialAnswers["material_usage_items"].([]Record)
	if materialRows[0]["note"] != "Nhà cung cấp giao trực tiếp" {
		t.Fatalf("material answers = %#v", materialAnswers)
	}

	blockersField := Record{
		"field_key": "blockers", "label": "Vướng mắc", "field_type": "structured_list", "required": false,
		"validation_json": map[string]any{"item_fields": []any{
			map[string]any{"key": "content", "label": "Nội dung", "type": "text", "required": true},
			map[string]any{"key": "impact", "label": "Ảnh hưởng", "type": "select", "required": true, "options": []any{map[string]any{"value": "other"}}},
		}},
	}
	if _, err = service.validateWorkflowFormAnswers(context.Background(), constructionInstance, []Record{blockersField}, Record{
		"blockers": []any{map[string]any{"content": "Giới hạn giờ thi công", "impact": "other"}},
	}, true); err != nil {
		t.Fatalf("blocker impact other must remain catalog-only without a detail requirement: %v", err)
	}
}

func TestWorkflowPDFOptionLabelPreservesCustomSelectText(t *testing.T) {
	options := []any{
		map[string]any{"value": "modern", "label": "Hiện đại"},
		map[string]any{"value": "other", "label": "Khác"},
	}
	if got := workflowPDFOptionLabel(options, "Japandi tông gỗ sáng"); got != "Japandi tông gỗ sáng" {
		t.Fatalf("custom style PDF label = %q", got)
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
					map[string]any{"key": "amount", "label": "Giá trị", "type": "currency", "required": false, "min": float64(0)},
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
		"action_items": []any{map[string]any{"title": "  Chốt bản vẽ  ", "quantity": "2.5", "amount": "1250000.25", "due_date": "2026-06-30", "priority": "high"}},
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
	if number(items[0]["amount"]) != 1250000.25 {
		t.Fatalf("normalized currency amount = %#v", items[0]["amount"])
	}
}

func TestSurveyWorkPackageDraftAndDimensionAreaCompatibility(t *testing.T) {
	service := NewService(&Repository{}, "test-secret")
	fields := []Record{
		{
			"field_key": "summary", "label": "Yêu cầu", "field_type": "text", "required": true,
			"validation_json": map[string]any{"min_length": float64(3), "max_length": float64(160)},
		},
		{
			"field_key": "dimension_items", "label": "Kích thước chính", "field_type": "structured_list", "required": true,
			"validation_json": map[string]any{
				"min_items": float64(1),
				"item_fields": []any{
					map[string]any{"key": "area", "label": "Khu vực", "type": "text", "required": true, "min_length": float64(2), "max_length": float64(160)},
					map[string]any{"key": "dimension", "label": "Kích thước", "type": "text", "required": true},
					map[string]any{"key": "unit", "label": "ĐVT", "type": "text", "required": true},
				},
			},
		},
		{
			"field_key": "risk_items", "label": "Danh sách rủi ro", "field_type": "structured_list", "required": true,
			"validation_json": map[string]any{
				"min_items": float64(1),
				"item_fields": []any{
					map[string]any{"key": "risk", "label": "Rủi ro", "type": "textarea", "required": true, "min_length": float64(3)},
				},
			},
		},
	}
	instance := Record{"form_code": "client_survey_work_package"}
	draftAnswers := Record{
		"summary":         "x",
		"dimension_items": []any{map[string]any{"area": "1", "dimension": "1", "unit": "m"}},
		"risk_items":      []any{map[string]any{"risk": "1"}},
	}
	draft, err := service.validateWorkflowFormAnswers(context.Background(), instance, fields, draftAnswers, false)
	if err != nil {
		t.Fatalf("short but structurally valid draft must save: %v", err)
	}
	if draft["summary"] != "x" {
		t.Fatalf("draft summary = %#v, want x", draft["summary"])
	}

	validSubmit := Record{
		"summary":         "Yêu cầu khảo sát",
		"dimension_items": []any{map[string]any{"dimension": "1", "unit": "m"}},
		"risk_items":      []any{map[string]any{"risk": "Rủi ro kết cấu"}},
	}
	answers, err := service.validateWorkflowFormAnswers(context.Background(), instance, fields, validSubmit, true)
	if err != nil {
		t.Fatalf("survey submit must allow omitted area: %v", err)
	}
	dimensions := answers["dimension_items"].([]Record)
	if _, exists := dimensions[0]["area"]; exists {
		t.Fatalf("omitted area must remain omitted, got %#v", dimensions[0])
	}

	validSubmit["dimension_items"] = []any{map[string]any{"area": "1", "dimension": "1", "unit": "m"}}
	if _, err = service.validateWorkflowFormAnswers(context.Background(), instance, fields, validSubmit, true); err != nil {
		t.Fatalf("survey submit must allow one-character area: %v", err)
	}

	invalidRisk := Record{
		"summary":         "Yêu cầu khảo sát",
		"dimension_items": []any{map[string]any{"area": "1", "dimension": "1", "unit": "m"}},
		"risk_items":      []any{map[string]any{"risk": "1"}},
	}
	_, err = service.validateWorkflowFormAnswers(context.Background(), instance, fields, invalidRisk, true)
	assertValidationError(t, err)
	var statusErr StatusError
	if !errors.As(err, &statusErr) {
		t.Fatalf("submit error = %#v", err)
	}
	details := recordFromAny(statusErr.Details)
	fieldErrors := recordFromAny(details["fields"])
	if got := fmt.Sprint(fieldErrors["risk_items"]); !strings.Contains(got, "Rủi ro phải có ít nhất 3 ký tự") {
		t.Fatalf("risk error = %q", got)
	}

	_, err = service.validateWorkflowFormAnswers(context.Background(), Record{"form_code": "another_form"}, fields, validSubmit, true)
	assertValidationError(t, err)
}

func TestSiteRiskAssessmentDefaultsMissingCategoryAndPreservesExistingValue(t *testing.T) {
	service := NewService(&Repository{}, "test-secret")
	fields := []Record{{
		"field_key":  clientSiteRiskItemsFieldKey,
		"label":      "Danh sách rủi ro",
		"field_type": "structured_list",
		"required":   true,
		"validation_json": map[string]any{
			"min_items": float64(1),
			"item_fields": []any{
				map[string]any{"key": "risk", "label": "Rủi ro", "type": "textarea", "required": true},
				map[string]any{"key": clientSiteRiskCategoryFieldKey, "label": "Nhóm", "type": "select", "required": true, "options": []any{
					map[string]any{"value": "mep"},
					map[string]any{"value": clientSiteRiskDefaultCategory},
				}},
				map[string]any{"key": "severity", "label": "Mức độ", "type": "select", "required": true, "options": []any{map[string]any{"value": "high"}}},
			},
		},
	}}
	instance := Record{"form_code": clientSiteRiskAssessmentFormCode}

	answers, err := service.validateWorkflowFormAnswers(context.Background(), instance, fields, Record{
		clientSiteRiskItemsFieldKey: []any{map[string]any{"risk": "Thiếu tải điện", "severity": "high"}},
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	items, ok := answers[clientSiteRiskItemsFieldKey].([]Record)
	if !ok || len(items) != 1 || items[0][clientSiteRiskCategoryFieldKey] != clientSiteRiskDefaultCategory {
		t.Fatalf("defaulted site risks = %#v, want category other", answers[clientSiteRiskItemsFieldKey])
	}

	answers, err = service.validateWorkflowFormAnswers(context.Background(), instance, fields, Record{
		clientSiteRiskItemsFieldKey: []any{map[string]any{"risk": "Xung đột đường ống", clientSiteRiskCategoryFieldKey: "mep", "severity": "high"}},
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	items = answers[clientSiteRiskItemsFieldKey].([]Record)
	if items[0][clientSiteRiskCategoryFieldKey] != "mep" {
		t.Fatalf("existing category = %#v, want mep", items[0][clientSiteRiskCategoryFieldKey])
	}

	_, err = service.validateWorkflowFormAnswers(context.Background(), Record{"form_code": "another_form"}, fields, Record{
		clientSiteRiskItemsFieldKey: []any{map[string]any{"risk": "Thiếu tải điện", "severity": "high"}},
	}, true)
	assertValidationError(t, err)
}

func TestSiteRiskAssessmentPersistsDefaultCategory(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")
	fixture := setupWorkItemFlowFixture(t, db, false)

	var formVersionID string
	var formTemplateID string
	var formType string
	if err := db.QueryRowContext(ctx, `
		SELECT v.id::text,t.id::text,t.form_type
		FROM workflow_form_versions v
		JOIN workflow_form_templates t ON t.id=v.template_id
		WHERE t.code=$1 AND v.status='published'
		ORDER BY v.version_no DESC
		LIMIT 1
	`, clientSiteRiskAssessmentFormCode).Scan(&formVersionID, &formTemplateID, &formType); err != nil {
		t.Fatal(err)
	}
	instanceID := newID()
	mustExec(t, db, `
		INSERT INTO workflow_form_instances (
			id,project_id,work_item_id,form_template_id,form_version_id,form_type,title,created_by
		) VALUES ($1,$2,$3,$4,$5,$6,'Đánh giá rủi ro mặt bằng',$7)
	`, instanceID, fixture.projectID, fixture.itemID, formTemplateID, formVersionID, formType, fixture.workerID)

	if _, err := service.SaveWorkflowFormDraft(ctx, instanceID, WorkflowFormSaveRequest{Answers: Record{
		clientSiteRiskItemsFieldKey: []any{map[string]any{"risk": "Thiếu tải điện", "severity": "high", "mitigation": "Kiểm tra lại công suất"}},
	}}, fixture.workerID, "Worker"); err != nil {
		t.Fatal(err)
	}
	assertCount(t, db, `
		SELECT COUNT(*)
		FROM workflow_form_answers
		WHERE instance_id=$1
		  AND field_key=$2
		  AND value_json->0->>$3=$4
	`, 1, instanceID, clientSiteRiskItemsFieldKey, clientSiteRiskCategoryFieldKey, clientSiteRiskDefaultCategory)

	if _, err := service.SaveWorkflowFormDraft(ctx, instanceID, WorkflowFormSaveRequest{Answers: Record{
		clientSiteRiskItemsFieldKey: []any{map[string]any{"risk": "Xung đột đường ống", clientSiteRiskCategoryFieldKey: "mep", "severity": "high"}},
	}}, fixture.workerID, "Worker"); err != nil {
		t.Fatal(err)
	}
	assertCount(t, db, `
		SELECT COUNT(*)
		FROM workflow_form_answers
		WHERE instance_id=$1
		  AND field_key=$2
		  AND value_json->0->>$3='mep'
	`, 1, instanceID, clientSiteRiskItemsFieldKey, clientSiteRiskCategoryFieldKey)
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

func TestWorkflowFormTemplateAndFieldValidation(t *testing.T) {
	if _, err := normalizeWorkflowFormTemplateRequest(WorkflowFormTemplateRequest{Code: "Bad Code", Name: "Form", FormType: "checklist"}, true); err == nil {
		t.Fatal("expected invalid code to fail")
	}
	if _, err := normalizeWorkflowFormTemplateRequest(WorkflowFormTemplateRequest{Code: "valid_code", Name: "Form", FormType: "unknown"}, true); err == nil {
		t.Fatal("expected invalid form type to fail")
	}
	fields, err := normalizeWorkflowFormFieldRequests([]WorkflowFormFieldRequest{{
		FieldKey:   "action_items",
		Label:      "Action items",
		FieldType:  "structured_list",
		Required:   true,
		OrderIndex: 10,
	}}, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(fields) != 1 || fields[0]["field_key"] != "action_items" || fields[0]["field_type"] != "structured_list" {
		t.Fatalf("unexpected normalized fields: %#v", fields)
	}
	if !validWorkflowFieldType("currency") {
		t.Fatal("currency must be a supported workflow field type")
	}
	if !validWorkflowCurrencyCode("VND") || validWorkflowCurrencyCode("1AA") || validWorkflowCurrencyCode("VNĐ") {
		t.Fatal("currency codes must contain exactly three uppercase ASCII letters")
	}
	if _, err := normalizeWorkflowFormFieldRequests([]WorkflowFormFieldRequest{{
		FieldKey:  "amount",
		Label:     "Amount",
		FieldType: "currency",
		Validation: Record{
			"currency_code":           "vnd",
			"maximum_fraction_digits": float64(7),
		},
	}}, true); err == nil {
		t.Fatal("expected invalid currency configuration to fail")
	}
	if _, err := normalizeWorkflowFormFieldRequests([]WorkflowFormFieldRequest{{
		FieldKey:  "vnd_amount",
		Label:     "VND amount",
		FieldType: "currency",
		Validation: Record{
			"currency_code":           "VND",
			"maximum_fraction_digits": float64(2),
		},
	}}, true); err == nil {
		t.Fatal("expected VND fractional configuration to fail")
	}
	if _, err := normalizeWorkflowFormFieldRequests([]WorkflowFormFieldRequest{{
		FieldKey:  "usd_amount",
		Label:     "USD amount",
		FieldType: "currency",
		Validation: Record{
			"currency_code":           "USD",
			"maximum_fraction_digits": float64(2),
		},
	}}, true); err != nil {
		t.Fatalf("expected valid USD fractional configuration: %v", err)
	}
	if _, err := normalizeWorkflowFormFieldRequests([]WorkflowFormFieldRequest{
		{FieldKey: "dup", Label: "Dup", FieldType: "text"},
		{FieldKey: "dup", Label: "Dup 2", FieldType: "text"},
	}, true); err == nil {
		t.Fatal("expected duplicate field key to fail")
	}
}

func TestStoreSealAssetUploadNormalizesSupportedImagesAndRejectsSVG(t *testing.T) {
	service := NewService(nil, "test-secret")
	storageRoot := t.TempDir()
	service.ConfigureUploads(storageRoot, 10*1024*1024)

	imageData := image.NewRGBA(image.Rect(0, 0, 2, 2))
	imageData.Set(0, 0, color.RGBA{R: 180, G: 20, B: 20, A: 255})
	var pngBuffer bytes.Buffer
	if err := png.Encode(&pngBuffer, imageData); err != nil {
		t.Fatal(err)
	}
	var jpegBuffer bytes.Buffer
	if err := jpeg.Encode(&jpegBuffer, imageData, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
	webpBytes, err := base64.StdEncoding.DecodeString("UklGRiIAAABXRUJQVlA4IBYAAAAwAQCdASoBAAEADsD+JaQAA3AAAAAA")
	if err != nil {
		t.Fatal(err)
	}

	for _, testCase := range []struct {
		name     string
		filename string
		content  []byte
		mimeType string
	}{
		{name: "png", filename: "seal.png", content: pngBuffer.Bytes(), mimeType: "image/png"},
		{name: "jpeg", filename: "seal.jpg", content: jpegBuffer.Bytes(), mimeType: "image/jpeg"},
		{name: "webp", filename: "seal.webp", content: webpBytes, mimeType: "image/webp"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			stored, _, originalName, sourceMIME, width, height, storeErr := service.storeSealAssetUpload(
				context.Background(),
				testMultipartFileHeader(t, "file", testCase.filename, testCase.content),
			)
			if storeErr != nil {
				t.Fatal(storeErr)
			}
			if originalName != testCase.filename || sourceMIME != testCase.mimeType {
				t.Fatalf("upload metadata original=%q mime=%q, want %q %q", originalName, sourceMIME, testCase.filename, testCase.mimeType)
			}
			if stored.MIMEType != "image/png" || stored.StorageProvider != "local" || stored.StorageKey == "" || stored.FileSize <= 0 || len(stored.ChecksumSHA256) != 64 {
				t.Fatalf("normalized seal metadata = %#v", stored)
			}
			if width <= 0 || height <= 0 {
				t.Fatalf("normalized dimensions = %dx%d", width, height)
			}
			normalized, readErr := os.ReadFile(filepath.Join(storageRoot, filepath.FromSlash(stored.StorageKey)))
			if readErr != nil {
				t.Fatal(readErr)
			}
			if !bytes.HasPrefix(normalized, []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}) {
				t.Fatalf("normalized content is not PNG: %v", normalized[:min(8, len(normalized))])
			}
		})
	}

	_, _, _, _, _, _, err = service.storeSealAssetUpload(
		context.Background(),
		testMultipartFileHeader(t, "file", "seal.svg", []byte(`<svg xmlns="http://www.w3.org/2000/svg"></svg>`)),
	)
	assertValidationError(t, err)
}

func TestWorkflowFormPDFGeneratorProducesPDFWithSignatureStamp(t *testing.T) {
	sealApplicationID := newID()
	sealImage, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+/p9sAAAAASUVORK5CYII=")
	if err != nil {
		t.Fatal(err)
	}
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
			{"field_key": "summary", "label": "Nội dung", "field_type": "textarea", "order_index": 1},
			{"field_key": "style", "label": "Phong cách mong muốn", "field_type": "select", "options_json": []any{map[string]any{"value": "minimal", "label": "Tối giản"}}, "order_index": 2},
			{"field_key": "meeting_date", "label": "Ngày họp", "field_type": "date", "order_index": 3},
			{"field_key": "budget", "label": "Ngân sách", "field_type": "currency", "validation_json": Record{"currency_code": "VND"}, "order_index": 4},
			{"field_key": "risks", "label": "Danh sách rủi ro", "field_type": "structured_list", "validation_json": Record{"item_fields": []any{
				map[string]any{"key": "risk", "label": "Rủi ro", "type": "textarea"},
				map[string]any{"key": "severity", "label": "Mức độ", "type": "select", "options": []any{map[string]any{"value": "high", "label": "Cao"}}},
			}}, "order_index": 5},
			{"field_key": "confirmed", "label": "Xác nhận", "field_type": "checkbox", "order_index": 6},
		},
		"answers": Record{
			"summary": "Đã thống nhất kế hoạch tuần", "style": "minimal", "meeting_date": "2026-06-21", "budget": 12500000, "confirmed": true,
			"risks": []any{map[string]any{"risk": "Chậm phê duyệt vật liệu", "severity": "high"}},
		},
		"signatures": []Record{{
			"id":                     newID(),
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
		"seal_applications": []Record{{
			"id":                sealApplicationID,
			"seal_asset_name":   "Dấu tròn TTCons",
			"purpose":           "internal_ack",
			"applied_by_name":   "Giám đốc",
			"applied_at":        "2026-06-21T10:05:00Z",
			"payload_hash":      strings.Repeat("c", 64),
			"checksum_sha256":   strings.Repeat("d", 64),
			"width_px":          1,
			"height_px":         1,
			"form_revision_no":  2,
			"seal_asset_status": "active",
			"storage_provider":  "local",
			"storage_key":       "seal-assets/test.png",
			"mime_type":         "image/png",
			"file_size":         len(sealImage),
		}},
	}
	context := WorkflowFormPDFContext{
		GeneratedAt: time.Date(2026, 6, 21, 17, 30, 0, 0, time.FixedZone("ICT", 7*60*60)),
		GeneratedBy: "Nguyễn Văn A", ProjectCode: "DA-001", ProjectName: "Văn phòng An Phát",
		NodeName: "Khảo sát", NodeOrder: 2, StepName: "Nắm phong cách thiết kế", StepOrder: 3,
		SealImages: map[string][]byte{sealApplicationID: sealImage},
	}
	pdfBytes, payloadHash, err := GenerateWorkflowFormPDFWithContext(form, context)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(pdfBytes, []byte("%PDF-")) {
		t.Fatalf("expected PDF header, got %q", pdfBytes[:8])
	}
	if len(pdfBytes) < 10000 {
		t.Fatalf("expected embedded-font PDF, got only %d bytes", len(pdfBytes))
	}
	if len(payloadHash) != 64 {
		t.Fatalf("payload hash = %q, want 64 chars", payloadHash)
	}
	document, err := buildWorkflowFormPDFDocument(form, context, payloadHash)
	if err != nil {
		t.Fatal(err)
	}
	if document.Title != "Biên bản họp kế hoạch" || document.Project != "DA-001 · Văn phòng An Phát" || document.Workflow != "Node 2: Khảo sát › Bước 3: Nắm phong cách thiết kế" {
		t.Fatalf("unexpected PDF business context: %#v", document)
	}
	if len(document.Fields) != 5 || document.Fields[1].Display != "Tối giản" || document.Fields[2].Display != "21/06/2026" || document.Fields[3].Display != "12.500.000 VND" || document.Fields[4].Display != "1 mục" {
		t.Fatalf("unexpected localized PDF fields: %#v", document.Fields)
	}
	structuredSpecs := workflowPDFStructuredSpecs(document.Fields[4].Definition)
	structuredRows := workflowPDFRecords(document.Fields[4].Value)
	if len(structuredSpecs) != 2 || len(structuredRows) != 1 || workflowPDFDisplayStructuredValue(structuredSpecs[1], structuredRows[0]["severity"]) != "Cao" {
		t.Fatalf("unexpected localized structured-list presentation: specs=%#v rows=%#v", structuredSpecs, structuredRows)
	}
	if len(document.Confirmations) != 1 || document.Confirmations[0].Display != "Đã xác nhận" {
		t.Fatalf("unexpected PDF confirmations: %#v", document.Confirmations)
	}
	if workflowPDFPurposeLabel("internal_ack") != "Xác nhận nội bộ" || len(signatureStrokes(document.Signatures[0])) != 1 {
		t.Fatalf("signature presentation was not preserved: %#v", document.Signatures)
	}
	if len(document.SealApplications) != 1 || cleanID(document.SealApplications[0]["id"]) != sealApplicationID || len(document.SealImages[sealApplicationID]) == 0 {
		t.Fatalf("seal presentation was not preserved: applications=%#v images=%d", document.SealApplications, len(document.SealImages[sealApplicationID]))
	}
	_, secondPayloadHash, err := GenerateWorkflowFormPDFWithContext(form, WorkflowFormPDFContext{GeneratedAt: context.GeneratedAt.Add(time.Hour), GeneratedBy: "Người khác"})
	if err != nil {
		t.Fatal(err)
	}
	if secondPayloadHash != payloadHash {
		t.Fatalf("presentation metadata changed immutable payload hash: %s != %s", secondPayloadHash, payloadHash)
	}
	form["seal_applications"].([]Record)[0]["seal_asset_status"] = "archived"
	form["seal_applications"].([]Record)[0]["storage_provider"] = "s3"
	_, archivedPayloadHash, err := GenerateWorkflowFormPDFWithContext(form, context)
	if err != nil {
		t.Fatal(err)
	}
	if archivedPayloadHash != payloadHash {
		t.Fatalf("mutable seal catalog/storage presentation changed immutable payload hash: %s != %s", archivedPayloadHash, payloadHash)
	}
	form["pdf_confirmation_preferences"] = []Record{{
		"purpose":            "internal_ack",
		"include_signatures": false,
		"include_seals":      true,
	}}
	selectedForm, confirmationSelection := selectWorkflowFormPDFConfirmations(form)
	if len(workflowPDFRecords(selectedForm["signatures"])) != 0 || len(workflowPDFRecords(selectedForm["seal_applications"])) != 1 {
		t.Fatalf("selected PDF confirmations = signatures %#v seals %#v", selectedForm["signatures"], selectedForm["seal_applications"])
	}
	if number(confirmationSelection["signature_count"]) != 0 || number(confirmationSelection["seal_application_count"]) != 1 {
		t.Fatalf("confirmation selection metadata = %#v", confirmationSelection)
	}
	_, selectedPayloadHash, err := GenerateWorkflowFormPDFWithContext(selectedForm, context)
	if err != nil {
		t.Fatal(err)
	}
	if selectedPayloadHash == payloadHash {
		t.Fatal("excluding a signature must change the rendered confirmation snapshot hash")
	}
}

func TestWorkflowFormPDFSelectsConfirmationsFromCurrentRevisionOnly(t *testing.T) {
	form := Record{
		"instance": Record{"current_revision": 2},
		"signatures": []Record{
			{"id": "signature-v1", "purpose": "internal_ack", "form_revision_no": 1},
			{"id": "signature-v2", "purpose": "internal_ack", "form_revision_no": 2},
		},
		"seal_applications": []Record{
			{"id": "seal-v1", "purpose": "internal_ack", "form_revision_no": 1},
			{"id": "seal-v2", "purpose": "internal_ack", "form_revision_no": 2},
		},
		"pdf_confirmation_preferences": []Record{{
			"purpose":            "internal_ack",
			"include_signatures": true,
			"include_seals":      true,
		}},
	}
	selected, selection := selectWorkflowFormPDFConfirmations(form)
	signatures := workflowPDFRecords(selected["signatures"])
	seals := workflowPDFRecords(selected["seal_applications"])
	if len(signatures) != 1 || cleanID(signatures[0]["id"]) != "signature-v2" {
		t.Fatalf("selected signatures = %#v, want current revision only", signatures)
	}
	if len(seals) != 1 || cleanID(seals[0]["id"]) != "seal-v2" {
		t.Fatalf("selected seals = %#v, want current revision only", seals)
	}
	if number(selection["signature_count"]) != 1 || number(selection["seal_application_count"]) != 1 {
		t.Fatalf("selection counts = %#v", selection)
	}
}

func TestWorkflowFormPDFOmitsSiteRiskCategoryFromNewPresentation(t *testing.T) {
	field := Record{
		"field_key": clientSiteRiskItemsFieldKey, "label": "Danh sách rủi ro", "field_type": "structured_list",
		"validation_json": Record{"item_fields": []any{
			map[string]any{"key": "risk", "label": "Rủi ro", "type": "textarea"},
			map[string]any{"key": clientSiteRiskCategoryFieldKey, "label": "Nhóm", "type": "select", "options": []any{map[string]any{"value": clientSiteRiskDefaultCategory, "label": "Khác"}}},
			map[string]any{"key": "severity", "label": "Mức độ", "type": "select"},
			map[string]any{"key": "mitigation", "label": "Hướng xử lý", "type": "textarea"},
		}},
	}
	form := Record{
		"instance": Record{"title": "Đánh giá rủi ro mặt bằng", "form_type": "checklist", "status": "submitted"},
		"template": Record{"code": clientSiteRiskAssessmentFormCode, "name": "Đánh giá rủi ro mặt bằng"},
		"version":  Record{"version_no": 1},
		"fields":   []Record{field},
		"answers": Record{clientSiteRiskItemsFieldKey: []any{map[string]any{
			"risk": "Thiếu tải điện", clientSiteRiskCategoryFieldKey: clientSiteRiskDefaultCategory, "severity": "high", "mitigation": "Kiểm tra lại công suất",
		}}},
	}
	pdfBytes, payloadHash, err := GenerateWorkflowFormPDFWithContext(form, WorkflowFormPDFContext{})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(pdfBytes, []byte("%PDF-")) {
		t.Fatalf("expected site-risk PDF header, got %q", pdfBytes[:min(8, len(pdfBytes))])
	}
	document, err := buildWorkflowFormPDFDocument(form, WorkflowFormPDFContext{}, payloadHash)
	if err != nil {
		t.Fatal(err)
	}
	if len(document.Fields) != 1 {
		t.Fatalf("PDF fields = %#v, want one risk list", document.Fields)
	}
	specs := workflowPDFStructuredSpecs(document.Fields[0].Definition)
	keys := make([]string, 0, len(specs))
	for _, spec := range specs {
		keys = append(keys, fmt.Sprint(spec["key"]))
	}
	if strings.Join(keys, ",") != "risk,severity,mitigation" {
		t.Fatalf("site-risk PDF specs = %v, want category omitted", keys)
	}
	storedSpecs := workflowPDFStructuredSpecs(field)
	if len(storedSpecs) != 4 {
		t.Fatalf("published field definition was mutated: %#v", storedSpecs)
	}

	form["template"] = Record{"code": "another_form", "name": "Biểu mẫu khác"}
	otherDocument, err := buildWorkflowFormPDFDocument(form, WorkflowFormPDFContext{}, strings.Repeat("b", 64))
	if err != nil {
		t.Fatal(err)
	}
	if len(workflowPDFStructuredSpecs(otherDocument.Fields[0].Definition)) != 4 {
		t.Fatalf("unrelated PDF field was changed: %#v", otherDocument.Fields[0].Definition)
	}
}

func TestWorkflowFormPDFOmitsConstructionHandoffEvidenceToggle(t *testing.T) {
	field := Record{
		"field_key": clientConstructionInspectionItemsFieldKey, "label": "Hạng mục giám sát", "field_type": "structured_list",
		"validation_json": Record{"item_fields": []any{
			map[string]any{"key": "title", "label": "Nội dung kiểm tra", "type": "text"},
			map[string]any{"key": "criteria", "label": "Tiêu chí đạt", "type": "textarea"},
			map[string]any{"key": clientConstructionEvidenceRequiredFieldKey, "label": "Bắt buộc minh chứng", "type": "checkbox"},
		}},
	}
	form := Record{
		"instance": Record{"title": "Kích hoạt bàn giao thi công", "form_type": "checklist", "status": "submitted"},
		"template": Record{"code": clientConstructionHandoffPlanFormCode, "name": "Kế hoạch giám sát và bàn giao thi công"},
		"version":  Record{"version_no": 1},
		"fields":   []Record{field},
		"answers": Record{clientConstructionInspectionItemsFieldKey: []any{map[string]any{
			"title": "Kiểm tra trần", "criteria": "Sai số không quá 3mm", clientConstructionEvidenceRequiredFieldKey: true,
		}}},
	}
	document, err := buildWorkflowFormPDFDocument(form, WorkflowFormPDFContext{}, strings.Repeat("c", 64))
	if err != nil {
		t.Fatal(err)
	}
	if len(document.Fields) != 1 {
		t.Fatalf("PDF fields = %#v, want one handoff item list", document.Fields)
	}
	specs := workflowPDFStructuredSpecs(document.Fields[0].Definition)
	keys := make([]string, 0, len(specs))
	for _, spec := range specs {
		keys = append(keys, fmt.Sprint(spec["key"]))
	}
	if strings.Join(keys, ",") != "title,criteria" {
		t.Fatalf("handoff PDF specs = %v, want evidence toggle omitted", keys)
	}
	if len(workflowPDFStructuredSpecs(field)) != 3 {
		t.Fatalf("published handoff field definition was mutated: %#v", field)
	}

	form["template"] = Record{"code": "another_form", "name": "Biểu mẫu khác"}
	otherDocument, err := buildWorkflowFormPDFDocument(form, WorkflowFormPDFContext{}, strings.Repeat("d", 64))
	if err != nil {
		t.Fatal(err)
	}
	if len(workflowPDFStructuredSpecs(otherDocument.Fields[0].Definition)) != 3 {
		t.Fatalf("unrelated handoff-like PDF field was changed: %#v", otherDocument.Fields[0].Definition)
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

	handover, err := service.CreateHandover(ctx, fixture.projectID, HandoverCreateRequest{ClientRepresentative: "Client", CompanyRepresentative: "PM"}, fixture.userID, "Scoped User")
	if err != nil {
		t.Fatal(err)
	}
	if cleanID(handover["created_by"]) != fixture.userID {
		t.Fatalf("handover created_by = %v, want %s", handover["created_by"], fixture.userID)
	}

	budget, err := service.CreateBudget(ctx, fixture.projectID, BudgetCreateRequest{BudgetCode: "BUD-CORE-" + suffix, Name: "Core budget", PlannedAmount: 1200}, fixture.userID, "Scoped User")
	if err != nil {
		t.Fatal(err)
	}
	if budget["status"] != "draft" || number(budget["approved_amount"]) != 0 || number(budget["used_amount"]) != 0 || number(budget["remaining_amount"]) != 0 {
		t.Fatalf("budget derived state = status %v approved %v used %v remaining %v, want draft/0/0/0", budget["status"], budget["approved_amount"], budget["used_amount"], budget["remaining_amount"])
	}
	budget, err = service.ApproveBudget(ctx, cleanID(budget["id"]), fixture.userID, "Scoped User")
	if err != nil {
		t.Fatal(err)
	}
	if budget["status"] != "approved" || cleanID(budget["approved_by"]) != fixture.userID {
		t.Fatalf("budget approval = status %v approved_by %v", budget["status"], budget["approved_by"])
	}
	if number(budget["approved_amount"]) != 1200 || number(budget["remaining_amount"]) != 1200 {
		t.Fatalf("budget approved amounts = approved %v remaining %v, want 1200/1200", budget["approved_amount"], budget["remaining_amount"])
	}

	payment, err := service.CreatePayment(ctx, PaymentCreateRequest{
		ProjectID: fixture.projectID, PaymentCode: "PMT-CORE-" + suffix,
		PaymentType: "outbound", TargetType: "issue", TargetID: cleanID(issue["id"]),
		BudgetID: cleanID(budget["id"]), Amount: 450, PaymentMethod: "bank_transfer",
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

func TestStartPipelineGeneratesWorkItemsAndIsIdempotent(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")

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
	operationViewID := ensurePermission(t, db, "operation.view", "Xem thi công", "operation")
	pmRoleID := ensureRole(t, db, "pm", "PM")
	mustExec(t, db, `INSERT INTO role_permissions (role_id, permission_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`, pmRoleID, operationViewID)
	mustExec(t, db, `INSERT INTO user_roles (user_id, role_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`, actorID, pmRoleID)
	mustExec(t, db, `INSERT INTO pipeline_templates (id, template_family_id, revision_no, name, status, created_by) VALUES ($1,$1,1,'Ops Template','published',$2)`, templateID, actorID)
	mustExec(t, db, `
		INSERT INTO pipeline_template_nodes (id, template_id, name, type, order_index, responsible_role_ids)
		VALUES ($1,$2,'Survey','survey',1,'["PM"]'::jsonb)
	`, nodeID, templateID)
	mustExec(t, db, `
		INSERT INTO pipeline_template_steps (id, node_id, name, step_type, required_role_ids, checklist_required, document_required, photo_required, form_version_id, form_required, order_index)
		VALUES ($1,$2,'Upload survey','document_upload','["PM"]'::jsonb,false,false,false,$4,true,1),
		       ($3,$2,'Confirm survey','checklist','["PM"]'::jsonb,true,false,false,$4,true,2)
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
	mustExec(t, db, `
		UPDATE work_items
		SET evidence_requirement_json=jsonb_set(
			COALESCE(evidence_requirement_json, '{}'::jsonb),
			'{required_permission}',
			to_jsonb('planning.edit'::text),
			true
		)
		WHERE id=$1
	`, workItemID)
	detail, err := repo.GetWorkItemDetail(ctx, workItemID)
	if err != nil {
		t.Fatal(err)
	}
	if cleanID(detail["workflow_form_instance_id"]) == "" || fmt.Sprint(detail["workflow_form_template_name"]) == "" || !boolValue(detail["workflow_form_required"]) {
		t.Fatalf("expected workflow form summary on work item detail, got %#v", detail)
	}
	if fmt.Sprint(detail["required_permission_name"]) != "Cập nhật kế hoạch" || fmt.Sprint(detail["required_permission_description"]) != "Cập nhật nội dung kế hoạch được phân công" {
		t.Fatalf("expected source-backed permission metadata on work item detail, got name=%v description=%v", detail["required_permission_name"], detail["required_permission_description"])
	}
	secondWorkItemID := cleanID(steps[1]["work_item_id"])
	beforeRequirements, err := service.WorkItemDetail(ctx, secondWorkItemID, actorID)
	if err != nil {
		t.Fatal(err)
	}
	beforeRows, _ := beforeRequirements["completion_requirements"].([]Record)
	beforeChecklist := recordByID(beforeRows, "key", "checklist")
	if beforeChecklist == nil || fmt.Sprint(beforeChecklist["status"]) != "missing" {
		t.Fatalf("checklist before form submit = %#v", beforeRows)
	}
	mustExec(t, db, `UPDATE workflow_form_instances SET status='submitted',submitted_by=$2,submitted_at=now() WHERE work_item_id=$1`, secondWorkItemID, actorID)
	afterRequirements, err := service.WorkItemDetail(ctx, secondWorkItemID, actorID)
	if err != nil {
		t.Fatal(err)
	}
	afterRows, _ := afterRequirements["completion_requirements"].([]Record)
	afterChecklist := recordByID(afterRows, "key", "checklist")
	if afterChecklist == nil || fmt.Sprint(afterChecklist["status"]) != "satisfied" || fmt.Sprint(afterChecklist["satisfied_by"]) != "workflow_form" {
		t.Fatalf("checklist after form submit = %#v", afterRows)
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
	lightOperations, err := repo.ProjectOperations(ctx, projectID, actorID, ProjectOperationsFilter{View: projectOperationsViewWorkflowSidebar})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := lightOperations["latest_activity"]; ok {
		t.Fatalf("expected lightweight operations to omit latest_activity, got %#v", lightOperations["latest_activity"])
	}
	if _, ok := lightOperations["latest_evidence"]; ok {
		t.Fatalf("expected lightweight operations to omit latest_evidence, got %#v", lightOperations["latest_evidence"])
	}
	if _, ok := lightOperations["audit_summary"]; ok {
		t.Fatalf("expected lightweight operations to omit audit_summary, got %#v", lightOperations["audit_summary"])
	}
	lightRun, _ := lightOperations["pipeline_run"].(Record)
	lightNodes, _ := lightRun["nodes"].([]Record)
	if len(lightNodes) != 1 {
		t.Fatalf("expected lightweight operations to include one runtime node, got %#v", lightOperations["pipeline_run"])
	}
	lightSteps, _ := lightNodes[0]["steps"].([]Record)
	if len(lightSteps) != 2 {
		t.Fatalf("expected lightweight operations to include runtime steps, got %#v", lightNodes[0]["steps"])
	}
	lightItems, _ := lightOperations["work_items"].([]Record)
	if len(lightItems) != 2 {
		t.Fatalf("expected lightweight operations to include sidebar work items, got %#v", lightOperations["work_items"])
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
	mustExec(t, db, `INSERT INTO pipeline_templates (id, template_family_id, revision_no, name, status, created_by) VALUES ($1,$1,1,'Invalid Ops Template','published',$2)`, templateID, actorID)
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

func TestClientOfficialWorkflowTemplateMigrationShape(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	templateID := "40000000-0000-0000-0000-000000000020"

	assertCount(t, db, `SELECT COUNT(*) FROM pipeline_templates WHERE id=$1 AND status='published'`, 1, templateID)
	assertCount(t, db, `SELECT COUNT(*) FROM pipeline_template_nodes WHERE template_id=$1`, 12, templateID)
	assertCount(t, db, `
		SELECT COUNT(*)
		FROM pipeline_template_steps pts
		JOIN pipeline_template_nodes ptn ON ptn.id=pts.node_id
		WHERE ptn.template_id=$1
	`, 52, templateID)
	assertCount(t, db, `
		SELECT COUNT(*)
		FROM pipeline_template_edges pte
		JOIN pipeline_template_nodes from_node ON from_node.id=pte.from_node_id
		JOIN pipeline_template_nodes to_node ON to_node.id=pte.to_node_id
		WHERE pte.template_id=$1
			AND from_node.name IN ('BOQ (Không thiết kế)','BOQ (Có thiết kế)')
			AND to_node.name='Đầu Vào: Tổ Đội / Thầu Phụ / Cung cấp vật tư'
	`, 2, templateID)
	assertCount(t, db, `
		SELECT COUNT(*)
		FROM pipeline_template_edges
		WHERE template_id=$1
			AND condition_type='workflow_form_answer_equals'
			AND condition_config ? 'field_key'
	`, 2, templateID)
	assertCount(t, db, `
		SELECT COUNT(*)
		FROM pipeline_template_nodes
		WHERE template_id=$1
			AND name='Đầu Vào: Tổ Đội / Thầu Phụ / Cung cấp vật tư'
			AND exit_conditions->>'mandatory_gate'='true'
	`, 1, templateID)
	assertCount(t, db, `
		SELECT COUNT(*)
		FROM pipeline_template_edges
		WHERE template_id=$1
			AND id='41600000-0000-0000-0000-000000000007'
			AND condition_type='procurement_approved'
	`, 1, templateID)
}

func TestOfficialWorkflowRevisionTwoWordingAndRoles(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	templateID := "40000000-0000-0000-0000-000000000021"

	assertCount(t, db, `SELECT COUNT(*) FROM pipeline_templates WHERE id=$1 AND status='published'`, 1, templateID)
	assertCount(t, db, `SELECT COUNT(*) FROM pipeline_template_nodes WHERE template_id=$1`, 13, templateID)
	assertCount(t, db, `
		SELECT COUNT(*)
		FROM pipeline_template_steps step
		JOIN pipeline_template_nodes node ON node.id=step.node_id
		WHERE node.template_id=$1
	`, 53, templateID)
	assertCount(t, db, `
		SELECT COUNT(*)
		FROM pipeline_template_nodes
		WHERE template_id=$1
			AND name='Đầu Vào'
			AND exit_conditions->>'mandatory_gate'='true'
	`, 1, templateID)
	assertCount(t, db, `
		SELECT COUNT(*)
		FROM pipeline_template_steps step
		JOIN pipeline_template_nodes node ON node.id=step.node_id
		WHERE node.template_id=$1
			AND (
				position('[' in step.name)>0 OR position(']' in step.name)>0 OR
				position('(' in step.name)>0 OR position(')' in step.name)>0 OR
				position('/' in step.name)>0
			)
	`, 0, templateID)

	wordingCases := []struct {
		stepID string
		name   string
		roles  string
	}{
		{"42500000-0000-0000-0000-000000000003", "Nắm phong cách thiết kế", `["Thiết kế"]`},
		{"42500000-0000-0000-0000-000000000005", "Xác định rủi ro mặt bằng", `["QS","Đội thi công"]`},
		{"42500000-0000-0000-0000-000000000009", "Lên bản vẽ 3D", `["Thiết kế"]`},
		{"42500000-0000-0000-0000-000000000019", "Review định mức quản lý", `["Giám đốc"]`},
		{"42500000-0000-0000-0000-000000000021", "Gửi BOQ cho chủ đầu tư", `["QS","Sale","PM"]`},
		{"42500000-0000-0000-0000-000000000030", "Kiểm tra hợp đồng", `["Kế toán","Pháp lý"]`},
		{"42500000-0000-0000-0000-000000000034", "Họp và phân công thi công", `["QS","PM","Thiết kế","QA/QC"]`},
	}
	for _, tc := range wordingCases {
		assertCount(t, db, `
			SELECT COUNT(*)
			FROM pipeline_template_steps step
			JOIN pipeline_template_nodes node ON node.id=step.node_id
			WHERE node.template_id=$1
				AND step.id=$2
				AND step.name=$3
				AND step.required_role_ids=$4::jsonb
		`, 1, templateID, tc.stepID, tc.name, tc.roles)
	}

	inputTerminologyCases := []struct {
		stepID      string
		name        string
		description string
	}{
		{"42500000-0000-0000-0000-000000000017", "Lấy giá đầu vào", "Thu thập giá đầu vào."},
		{"42500000-0000-0000-0000-000000000023", "Mua hàng đầu vào", "Thực hiện mua hàng đầu vào."},
		{"42500000-0000-0000-0000-000000000024", "Ký hợp đồng và lập hồ sơ tạm ứng đầu vào", "Ký hợp đồng và chuẩn bị hồ sơ tạm ứng đầu vào."},
	}
	for _, tc := range inputTerminologyCases {
		assertCount(t, db, `
			SELECT COUNT(*)
			FROM pipeline_template_steps step
			JOIN pipeline_template_nodes node ON node.id=step.node_id
			WHERE node.template_id=$1
				AND step.id=$2
				AND step.name=$3
				AND step.description=$4
		`, 1, templateID, tc.stepID, tc.name, tc.description)
	}
	assertCount(t, db, `
		SELECT COUNT(*)
		FROM pipeline_template_nodes
		WHERE template_id=$1
			AND id='42400000-0000-0000-0000-000000000006'
			AND description='Bước bắt buộc để chốt khối lượng, mua hàng, hợp đồng/tạm ứng và rà soát đầu vào.'
			AND exit_conditions->>'text'='Đầu vào đã được rà soát và kiểm tra'
	`, 1, templateID)
	assertCount(t, db, `
		SELECT COUNT(*)
		FROM pipeline_template_edges
		WHERE template_id=$1
			AND id='42600000-0000-0000-0000-000000000007'
			AND label='Đầu vào đã được kiểm tra'
	`, 1, templateID)
	assertCount(t, db, `
		SELECT COUNT(*)
		FROM workflow_form_templates
		WHERE (code='client_boq_supplier_price_collection' AND name='Thu thập giá đầu vào')
			OR (
				code='client_procurement_sourcing'
				AND name='Mua hàng đầu vào'
				AND description='Ghi nhận nguồn đầu vào, phạm vi mua hàng, so sánh báo giá, lead time và phương án lựa chọn.'
			)
	`, 2)

	explicitEntityCases := []struct {
		stepID string
		name   string
	}{
		{"42500000-0000-0000-0000-000000000026", "Đàm phán với thầu phụ để chốt giá hợp lý"},
		{"42500000-0000-0000-0000-000000000038", "Nghiệm thu thầu phụ và tổ đội"},
		{"42500000-0000-0000-0000-000000000039", "Nghiệm thu vật tư đầu vào và đầu ra"},
		{"42500000-0000-0000-0000-000000000040", "Lập hồ sơ kiểm tra khối lượng thầu phụ"},
		{"42500000-0000-0000-0000-000000000042", "Lập hồ sơ đề nghị chi thầu phụ"},
		{"42500000-0000-0000-0000-000000000045", "Bàn giao hồ sơ chi cho Kế toán"},
		{"42500000-0000-0000-0000-000000000051", "Ghi nhận chi phí mua vật tư công trình"},
	}
	projectFinanceSteps := []struct {
		stepID     string
		name       string
		permission string
	}{
		{"42500000-0000-0000-0000-000000000043", "Duyệt đề nghị chi", "project_finance.request_approve"},
		{"42500000-0000-0000-0000-000000000044", "Kiểm tra hồ sơ đề nghị chi", "project_finance.view"},
		{"42500000-0000-0000-0000-000000000048", "Lập hồ sơ đề nghị thu từ chủ đầu tư", "project_finance.request_create"},
		{"42500000-0000-0000-0000-000000000049", "Bàn giao hồ sơ phát hành hóa đơn đầu ra", "project_finance.process"},
		{"42500000-0000-0000-0000-000000000050", "Theo dõi hồ sơ thu theo kỳ", "project_finance.view"},
		{"42500000-0000-0000-0000-000000000052", "Xác nhận hoàn tất hồ sơ tài chính quản trị", "project_finance.report_view"},
	}
	for _, tc := range projectFinanceSteps {
		assertCount(t, db, `
			SELECT COUNT(*)
			FROM pipeline_template_steps step
			JOIN pipeline_template_nodes node ON node.id=step.node_id
			WHERE node.template_id=$1 AND step.id=$2 AND step.name=$3 AND step.required_permission=$4
		`, 1, templateID, tc.stepID, tc.name, tc.permission)
	}
	for _, tc := range explicitEntityCases {
		assertCount(t, db, `
			SELECT COUNT(*)
			FROM pipeline_template_steps step
			JOIN pipeline_template_nodes node ON node.id=step.node_id
			WHERE node.template_id=$1 AND step.id=$2 AND step.name=$3
		`, 1, templateID, tc.stepID, tc.name)
	}

	roleCases := []struct {
		name        string
		displayName string
		permissions []string
	}{
		{"design", "Thiết kế", []string{"project.view", "pipeline.view", "planning.view", "planning.edit", "operation.view", "operation.report"}},
		{"sales", "Sale", []string{"project.view", "pipeline.view", "planning.view", "planning.edit", "boq.view", "boq.update", "boq.approve", "contract.view", "contract.review"}},
		{"legal", "Pháp lý", []string{"project.view", "pipeline.view", "contract.view", "contract.review"}},
	}
	for _, tc := range roleCases {
		assertCount(t, db, `SELECT COUNT(*) FROM roles WHERE name=$1 AND display_name=$2 AND system_role=true`, 1, tc.name, tc.displayName)
		assertCount(t, db, `
			SELECT COUNT(*)
			FROM role_permissions role_permission
			JOIN roles role ON role.id=role_permission.role_id
			JOIN permissions permission ON permission.id=role_permission.permission_id
			WHERE role.name=$1 AND permission.key=ANY($2)
		`, len(tc.permissions), tc.name, tc.permissions)
	}
}

func TestClientOfficialWorkflowRoutesByDesignDecisionFormAnswer(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)

	actorID := newID()
	actorStaffID := newID()
	projectID := newID()
	templateID := "40000000-0000-0000-0000-000000000020"
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM projects WHERE id=$1`, projectID)
		_, _ = db.ExecContext(ctx, `DELETE FROM staff WHERE id=$1`, actorStaffID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id=$1`, actorID)
	})

	mustExec(t, db, `INSERT INTO users (id, email, password_hash, full_name, status) VALUES ($1,$2,'demo:pw','Test Actor','active')`, actorID, "client-flow-"+actorID+"@test.local")
	mustExec(t, db, `INSERT INTO staff (id, user_id, full_name, email, department, status) VALUES ($1,$2,'Test Actor',$3,'operations','active')`, actorStaffID, actorID, "client-flow-staff-"+actorID+"@test.local")
	mustExec(t, db, `
		INSERT INTO projects (id, name, code, client_name, status, pipeline_template_id)
		VALUES ($1,'Client Official Flow Project',$2,'Client','draft',$3)
	`, projectID, "COF-"+projectID[:8], templateID)
	mustExec(t, db, `INSERT INTO project_members (project_id, staff_id, role_ids, active) VALUES ($1,$2,'["PM"]'::jsonb,true)`, projectID, actorStaffID)

	run, err := repo.StartPipeline(ctx, projectID, templateID, actorID, "Test Actor")
	if err != nil {
		t.Fatal(err)
	}
	surveyNodeID := runNodeIDByName(t, run, "Khảo sát")
	meetingNodeID := runNodeIDByName(t, run, "Họp báo cáo khảo sát")

	satisfyRunNodeForAdvance(t, db, surveyNodeID, actorID)
	if _, err := repo.AdvanceRunNode(ctx, surveyNodeID, actorID, "Test Actor"); err != nil {
		t.Fatal(err)
	}
	mustExec(t, db, `
		UPDATE workflow_form_instances
		SET status='submitted',
			current_revision=1,
			answers_snapshot_json='{"needs_design":"no","decision_note":"Khách chốt không thiết kế"}'::jsonb,
			submitted_by=$3,
			submitted_at=now(),
			updated_at=now()
		WHERE project_id=$1
			AND run_node_id=$2
			AND form_template_id=(SELECT id FROM workflow_form_templates WHERE code='client_design_branch_decision')
		`, projectID, meetingNodeID, actorID)

	satisfyRunNodeForAdvance(t, db, meetingNodeID, actorID)
	if _, err := repo.AdvanceRunNode(ctx, meetingNodeID, actorID, "Test Actor"); err != nil {
		t.Fatal(err)
	}
	assertCount(t, db, `
		SELECT COUNT(*)
		FROM pipeline_runs pr
		JOIN pipeline_run_nodes rn ON rn.id=pr.current_node_id
		WHERE pr.project_id=$1 AND rn.name='BOQ (Không thiết kế)'
	`, 1, projectID)
	assertCount(t, db, `
		SELECT COUNT(*)
		FROM pipeline_run_nodes
		WHERE project_id=$1 AND name='Thiết kế' AND status='in_progress'
	`, 0, projectID)
}

func TestClientWorkflowAutomationCreatesIncomingScheduleAndInvoiceBlocker(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	fixture := setupClientOfficialWorkflowFixture(t, db, 150000000)
	workItemID := clientOfficialWorkItemIDByAction(t, db, fixture.projectID, "finance.scaffold_incoming_acceptance")
	procurementID := newID()
	mustExec(t, db, `
		INSERT INTO procurement_requests (id, project_id, request_code, title, requested_by, status, total_estimated_amount)
		VALUES ($1,$2,$3,'Đầu vào đã duyệt',$4,'approved',75000000)
	`, procurementID, fixture.projectID, "PR-IN-"+procurementID[:8], fixture.actorID)
	mustExec(t, db, `
		INSERT INTO procurement_request_items (request_id, quantity_planned, quantity_requested, unit, estimated_unit_price, supplier_name, approval_status)
		VALUES ($1,1,1,'gói',75000000,'Nhà cung cấp kiểm thử','approved')
	`, procurementID)
	mustExec(t, db, `UPDATE work_items SET status='completed' WHERE id=$1`, workItemID)
	assertValidationError(t, repo.RunRequiredWorkflowAutomationForWorkItem(ctx, workItemID, fixture.actorID, "Client Actor"))
	assertValidationError(t, repo.RunRequiredWorkflowAutomationForWorkItem(ctx, workItemID, fixture.actorID, "Client Actor"))
	assertCount(t, db, `
		SELECT COUNT(*)
		FROM incoming_acceptance_records
		WHERE project_id=$1
			AND procurement_request_id=$2
			AND source_type='procurement'
			AND status='draft'
	`, 1, fixture.projectID, procurementID)
	assertCount(t, db, `
		SELECT COUNT(*)
		FROM payables
		WHERE project_id=$1
			AND source_type='procurement'
			AND source_id=$2
			AND status='draft'
			AND gross_amount=75000000
			AND remaining_amount=75000000
	`, 1, fixture.projectID, procurementID)
	assertCount(t, db, `
		SELECT COUNT(*)
		FROM payable_payment_schedules pps
		JOIN payables pay ON pay.id=pps.payable_id
		WHERE pay.project_id=$1
			AND pay.source_id=$2
			AND pps.schedule_code='LICH-THANH-TOAN-001'
			AND pps.milestone_name='Thanh toán đầu vào sau nghiệm thu'
			AND pps.status='draft'
			AND pps.planned_amount=75000000
			AND pps.remaining_amount=75000000
	`, 1, fixture.projectID, procurementID)
	assertCount(t, db, `
		SELECT COUNT(*)
		FROM input_invoices ii
		JOIN payables pay ON pay.id=ii.payable_id
		WHERE pay.project_id=$1 AND pay.source_id=$2
	`, 0, fixture.projectID, procurementID)
	assertCount(t, db, `
		SELECT COUNT(*)
		FROM workflow_automation_events
		WHERE source_type='pipeline_run_node'
			AND source_id=(SELECT run_node_id FROM work_items WHERE id=$1)
			AND action_key='finance.scaffold_incoming_acceptance'
			AND status='blocked'
			AND target_type='payable'
			AND blocker_reason='Cần tải lên và xác minh hóa đơn đầu vào thật'
			AND run_count=$2
	`, 1, workItemID, 2)
	assertCount(t, db, `
		SELECT COUNT(*)
		FROM work_item_activities wia
		JOIN work_items wi ON wi.id=wia.work_item_id
		WHERE wi.run_node_id=(SELECT run_node_id FROM work_items WHERE id=$1)
			AND activity_type='blocker'
			AND comment='Cần tải lên và xác minh hóa đơn đầu vào thật'
			AND metadata_json->>'automation_action'='finance.scaffold_incoming_acceptance'
	`, 1, workItemID)
}

func TestWorkflowFormAnswerValueMatches(t *testing.T) {
	tests := []struct {
		name     string
		actual   any
		expected any
		want     bool
	}{
		{name: "string ignores case and spaces", actual: " No ", expected: "no", want: true},
		{name: "bool parses expected string", actual: true, expected: "true", want: true},
		{name: "number string matches numeric", actual: "12.5", expected: 12.5, want: true},
		{name: "expected list matches one value", actual: "no", expected: []any{"yes", "no"}, want: true},
		{name: "actual list matches expected", actual: []any{"draft", "approved"}, expected: "approved", want: true},
		{name: "mismatch", actual: "yes", expected: "no", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := workflowFormAnswerValueMatches(tt.actual, tt.expected); got != tt.want {
				t.Fatalf("workflowFormAnswerValueMatches(%#v, %#v) = %v, want %v", tt.actual, tt.expected, got, tt.want)
			}
		})
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
		if row["template_code"] == "builtin_checklist" && boolValue(row["latest_published"]) {
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

func TestWorkflowFormVersionGovernancePublishesAndKeepsOldCatalogVersions(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")
	actorID := newID()
	mustExec(t, db, `INSERT INTO users (id, email, password_hash, full_name, status) VALUES ($1,$2,'demo:pw','Workflow Form Admin','active')`, actorID, "workflow-form-governance-"+actorID+"@test.local")
	code := "custom_governance_" + strings.ReplaceAll(newID()[:8], "-", "_")

	template, err := service.CreateWorkflowFormTemplate(ctx, WorkflowFormTemplateRequest{
		Code:     code,
		Name:     "Custom Governance Form",
		FormType: "generic_submission",
	}, actorID, "Workflow Form Admin")
	if err != nil {
		t.Fatal(err)
	}
	templateID := cleanID(template["template_id"])
	draft, err := service.CreateWorkflowFormVersion(ctx, templateID, WorkflowFormVersionRequest{
		Fields: []WorkflowFormFieldRequest{{FieldKey: "summary", Label: "Summary", FieldType: "textarea", Required: true}},
	}, actorID, "Workflow Form Admin")
	if err != nil {
		t.Fatal(err)
	}
	published, err := service.PublishWorkflowFormVersion(ctx, cleanID(draft["version_id"]), actorID, "Workflow Form Admin")
	if err != nil {
		t.Fatal(err)
	}
	if published["version_status"] != "published" || number(published["version_no"]) != 1 {
		t.Fatalf("unexpected first published version: %#v", published)
	}
	nextDraft, err := service.CreateWorkflowFormVersion(ctx, templateID, WorkflowFormVersionRequest{
		SourceVersionID: cleanID(published["version_id"]),
	}, actorID, "Workflow Form Admin")
	if err != nil {
		t.Fatal(err)
	}
	updatedDraft, err := service.UpdateWorkflowFormVersion(ctx, cleanID(nextDraft["version_id"]), WorkflowFormVersionRequest{
		Fields: []WorkflowFormFieldRequest{
			{FieldKey: "summary", Label: "Summary", FieldType: "textarea", Required: true},
			{FieldKey: "next_action", Label: "Next action", FieldType: "textarea", Required: false},
		},
	}, actorID, "Workflow Form Admin")
	if err != nil {
		t.Fatal(err)
	}
	if number(updatedDraft["field_count"]) != 2 {
		t.Fatalf("expected updated draft fields, got %#v", updatedDraft)
	}
	published2, err := service.PublishWorkflowFormVersion(ctx, cleanID(nextDraft["version_id"]), actorID, "Workflow Form Admin")
	if err != nil {
		t.Fatal(err)
	}
	if published2["version_status"] != "published" || !boolValue(published2["latest_published"]) {
		t.Fatalf("expected second version to be latest published, got %#v", published2)
	}

	catalog, err := repo.WorkflowFormCatalog(ctx)
	if err != nil {
		t.Fatal(err)
	}
	publishedRows := 0
	latestVersion := 0.0
	for _, row := range catalog {
		if row["template_code"] != code {
			continue
		}
		publishedRows++
		if boolValue(row["latest_published"]) {
			latestVersion = number(row["version_no"])
		}
	}
	if publishedRows != 2 || latestVersion != 2 {
		t.Fatalf("expected both published versions with v2 latest, rows=%d latest=%v", publishedRows, latestVersion)
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
	mustExec(t, db, `INSERT INTO pipeline_templates (id, template_family_id, revision_no, name, status, created_by) VALUES ($1,$1,1,'Invalid Form Template','draft',$2)`, templateID, actorID)
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

func TestPublishTemplateRejectsExternalOnlyNodeStepAndApprovalRoles(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")
	actorID := newID()
	templateID := newID()
	nodeID := newID()
	stepID := newID()
	externalRoleID := newID()
	suffix := templateID[:8]
	externalLabel := "Đối tác ngoài App " + suffix
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM pipeline_templates WHERE id=$1`, templateID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id=$1`, actorID)
		_, _ = db.ExecContext(ctx, `DELETE FROM roles WHERE id=$1`, externalRoleID)
	})
	mustExec(t, db, `INSERT INTO users (id,email,password_hash,full_name,status) VALUES ($1,$2,'demo:pw','External Publish Actor','active')`, actorID, "external-publish-"+suffix+"@test.local")
	mustExec(t, db, `INSERT INTO roles (id,name,display_name,system_role,external_only) VALUES ($1,$2,$3,true,true)`, externalRoleID, "external_publish_"+suffix, externalLabel)
	mustExec(t, db, `INSERT INTO pipeline_templates (id,template_family_id,revision_no,name,status,created_by) VALUES ($1,$1,1,'External Role Template','draft',$2)`, templateID, actorID)
	mustExec(t, db, `
		INSERT INTO pipeline_template_nodes (id,template_id,name,type,order_index,responsible_role_ids,approval_policy_json)
		VALUES ($1,$2,'External Node','custom',1,jsonb_build_array($3::text),jsonb_build_object('mode','any','roles',jsonb_build_array($3::text),'quorum',1,'requester_cannot_approve',true,'comment_required',true,'signature_required',false))
	`, nodeID, templateID, externalLabel)
	mustExec(t, db, `INSERT INTO pipeline_template_steps (id,node_id,name,step_type,required_role_ids,form_required,order_index) VALUES ($1,$2,'External Step','task',jsonb_build_array($3::text),false,1)`, stepID, nodeID, externalLabel)

	issues, err := repo.TemplatePublishIssues(ctx, templateID)
	if err != nil {
		t.Fatal(err)
	}
	externalIssueCount := 0
	for _, issue := range issues {
		if issue["code"] == "EXTERNAL_ROLE_NOT_ASSIGNABLE" {
			externalIssueCount++
		}
	}
	if externalIssueCount != 3 {
		t.Fatalf("external-only publish issues = %d, want node + step + approval policy; issues=%#v", externalIssueCount, issues)
	}
	if _, err := service.PublishTemplate(ctx, templateID, actorID, "External Publish Actor"); err == nil {
		t.Fatal("expected template publish with external-only roles to fail")
	}
	template, err := repo.GetByID(ctx, "pipeline_templates", templateID)
	if err != nil {
		t.Fatal(err)
	}
	if template["status"] != "draft" {
		t.Fatalf("template status = %v, want draft", template["status"])
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
	var formVersionID string
	err := db.QueryRowContext(ctx, `
		SELECT v.id FROM workflow_form_versions v
		JOIN workflow_form_templates t ON t.id=v.template_id
		WHERE t.code='builtin_checklist' AND t.status='active' AND v.status='published'
		ORDER BY v.version_no DESC LIMIT 1
	`).Scan(&formVersionID)
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
			"check_date":     "2026-07-10",
			"checked_by":     "Worker",
			"planning_scope": "design",
			"checklist_items": []any{
				map[string]any{"title": "Đã kiểm tra checklist", "assignee_role": "PM", "status": "done", "comment": "Đạt"},
			},
			"confirmed": true,
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
	answers := recordFromAny(form["answers"])
	if got, ok := answers["checked_by"].(string); !ok || got != "Worker" {
		t.Fatalf("checked_by answer = %#v, want native string Worker", answers["checked_by"])
	}
	if got, ok := answers["confirmed"].(bool); !ok || !got {
		t.Fatalf("confirmed answer = %#v, want native boolean true", answers["confirmed"])
	}
	if _, ok := answers["checklist_items"].([]any); !ok {
		t.Fatalf("checklist_items answer = %#v, want native array", answers["checklist_items"])
	}
	assertCount(t, db, `SELECT COUNT(*) FROM workflow_form_answers WHERE instance_id=$1`, 5, instanceID)
	assertCount(t, db, `SELECT COUNT(*) FROM workflow_form_events WHERE instance_id=$1 AND event_type='submitted'`, 1, instanceID)
	assertCount(t, db, `SELECT COUNT(*) FROM audit_logs WHERE target_type='workflow_form_instance' AND target_id=$1`, 1, instanceID)

	evidenceReferenceID := newID()
	mustExec(t, db, `
		INSERT INTO workflow_form_answers (instance_id, field_key, field_type, value_json, revision_no, updated_by)
		VALUES
			($1,'scalar_number','number','12.5'::jsonb,1,$2),
			($1,'scalar_null','text','null'::jsonb,1,$2),
			($1,'scalar_object','text','{"key":"value"}'::jsonb,1,$2),
			($1,'scalar_array','structured_list','["one","two"]'::jsonb,1,$2),
			($1,'scalar_evidence','evidence_reference',to_jsonb($3::text),1,$2)
	`, instanceID, fixture.workerID, evidenceReferenceID)
	detail, err := repo.GetWorkflowFormInstanceDetail(ctx, instanceID)
	if err != nil {
		t.Fatal(err)
	}
	detailAnswers := recordFromAny(detail["answers"])
	if got, ok := detailAnswers["scalar_number"].(float64); !ok || got != 12.5 {
		t.Fatalf("scalar_number answer = %#v, want native number 12.5", detailAnswers["scalar_number"])
	}
	if got, exists := detailAnswers["scalar_null"]; !exists || got != nil {
		t.Fatalf("scalar_null answer = %#v (exists=%t), want native null", got, exists)
	}
	if got := fmt.Sprint(recordFromAny(detailAnswers["scalar_object"])["key"]); got != "value" {
		t.Fatalf("scalar_object answer key = %q, want value", got)
	}
	if got, ok := detailAnswers["scalar_array"].([]any); !ok || len(got) != 2 {
		t.Fatalf("scalar_array answer = %#v, want native two-item array", detailAnswers["scalar_array"])
	}
	if got, ok := detailAnswers["scalar_evidence"].(string); !ok || got != evidenceReferenceID {
		t.Fatalf("scalar_evidence answer = %#v, want native evidence ID %s", detailAnswers["scalar_evidence"], evidenceReferenceID)
	}

	completed, err := service.WorkItemAction(ctx, fixture.itemID, "complete", WorkItemActionRequest{Comment: "done"}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	if completed["status"] != "completed" {
		t.Fatalf("work item status = %v, want completed", completed["status"])
	}
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

func TestWorkflowFormSealAssetsAreReusableArchivedAndSeparateFromSignatures(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")
	service.ConfigureUploads(t.TempDir(), 10*1024*1024)
	fixture := setupWorkItemFlowFixture(t, db, false)

	formVersionID, err := repo.DefaultWorkflowFormVersionID(ctx, "task", false)
	if err != nil {
		t.Fatal(err)
	}
	instanceRows, err := repo.Query(ctx, `
		INSERT INTO workflow_form_instances (project_id, work_item_id, form_template_id, form_version_id, form_type, title, created_by)
		SELECT $1, $2, v.template_id, v.id, t.form_type, 'Seal confirmation form', $3
		FROM workflow_form_versions v
		JOIN workflow_form_templates t ON t.id=v.template_id
		WHERE v.id=$4
		RETURNING id
	`, fixture.projectID, fixture.itemID, fixture.workerID, formVersionID)
	if err != nil {
		t.Fatal(err)
	}
	instanceID := cleanID(instanceRows[0]["id"])
	sealImage := image.NewRGBA(image.Rect(0, 0, 2, 2))
	sealImage.Set(0, 0, color.RGBA{R: 180, G: 20, B: 20, A: 255})
	var sealBuffer bytes.Buffer
	if err := png.Encode(&sealBuffer, sealImage); err != nil {
		t.Fatal(err)
	}
	pngBytes := sealBuffer.Bytes()

	if _, err := service.SealAssets(ctx, false, fixture.workerID); err == nil {
		t.Fatal("expected seal catalog permission to be required")
	} else {
		var statusErr StatusError
		if !errors.As(err, &statusErr) || statusErr.Code != "FORBIDDEN" {
			t.Fatalf("seal catalog permission error = %#v", err)
		}
	}
	if _, err := service.CreateSealAsset(ctx, "Dấu chưa cấp quyền", testMultipartFileHeader(t, "file", "seal.png", pngBytes), fixture.workerID, "Worker"); err == nil {
		t.Fatal("expected seal.manage permission to be required")
	}

	// seal.manage is intentionally reserved for Admin/Developer. Keep the
	// ordinary worker on seal.use and use the PM fixture as the catalog manager.
	mustExec(t, db, `
		INSERT INTO user_roles (user_id, role_id)
		SELECT $1, id FROM roles WHERE name='admin'
		ON CONFLICT DO NOTHING
	`, fixture.pmID)
	mustExec(t, db, `
		INSERT INTO role_permissions (role_id, permission_id)
		SELECT assignment.role_id, permission.id
		FROM user_roles assignment
		JOIN permissions permission ON permission.key='seal.use'
		WHERE assignment.user_id=$1
		ON CONFLICT DO NOTHING
	`, fixture.workerID)
	asset, err := service.CreateSealAsset(ctx, "Dấu tròn TTCons", testMultipartFileHeader(t, "file", "ttcons-seal.png", pngBytes), fixture.pmID, "PM")
	if err != nil {
		t.Fatal(err)
	}
	assetID := cleanID(asset["id"])
	if assetID == "" || fmt.Sprint(asset["status"]) != "active" || fmt.Sprint(asset["mime_type"]) != "image/png" || fmt.Sprint(asset["source_mime_type"]) != "image/png" {
		t.Fatalf("created seal asset = %#v", asset)
	}
	if hash := fmt.Sprint(asset["checksum_sha256"]); len(hash) != 64 {
		t.Fatalf("seal checksum = %q, want 64 hex chars", hash)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM audit_logs WHERE target_type='seal_asset' AND target_id=$1 AND event_type='seal_asset_uploaded'`, 1, assetID)

	_, err = service.ApplyWorkflowFormSeal(ctx, instanceID, WorkflowFormSealApplyRequest{SealAssetID: assetID, Purpose: "internal_ack"}, fixture.workerID, "Worker")
	assertValidationError(t, err)
	if _, err := service.SubmitWorkflowForm(ctx, instanceID, WorkflowFormSaveRequest{
		Answers: Record{"summary": "Đã hoàn tất nội dung để xác nhận bằng con dấu"},
	}, fixture.workerID, "Worker"); err != nil {
		t.Fatal(err)
	}

	form, err := service.ApplyWorkflowFormSeal(ctx, instanceID, WorkflowFormSealApplyRequest{
		SealAssetID: assetID,
		Purpose:     "internal_ack",
		Comment:     "Đóng dấu xác nhận nội bộ",
	}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	form, err = service.ApplyWorkflowFormSeal(ctx, instanceID, WorkflowFormSealApplyRequest{
		SealAssetID: assetID,
		Purpose:     "reviewed_by",
		Comment:     "Tái sử dụng cùng tài nguyên cho mục đích rà soát",
	}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	applications, ok := form["seal_applications"].([]Record)
	if !ok || len(applications) != 2 {
		t.Fatalf("expected two reusable seal applications, got %#v", form["seal_applications"])
	}
	if signatures, ok := form["signatures"].([]Record); !ok || len(signatures) != 0 {
		t.Fatalf("seal applications must remain separate from human signatures, got %#v", form["signatures"])
	}
	applicationID := cleanID(applications[0]["id"])
	if applicationID == "" || cleanID(applications[0]["seal_asset_id"]) != assetID {
		t.Fatalf("seal application missing immutable links: %#v", applications[0])
	}
	assertCount(t, db, `SELECT COUNT(*) FROM workflow_form_events WHERE instance_id=$1 AND event_type='seal_applied'`, 2, instanceID)
	assertCount(t, db, `SELECT COUNT(*) FROM audit_logs WHERE target_type='workflow_form_seal_application' AND event_type='workflow_form_seal_applied' AND metadata_json->>'seal_asset_id'=$1`, 2, assetID)

	archived, err := service.ArchiveSealAsset(ctx, assetID, fixture.pmID, "PM")
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(archived["status"]) != "archived" {
		t.Fatalf("archived seal status = %#v", archived["status"])
	}
	activeAssets, err := service.SealAssets(ctx, false, fixture.workerID)
	if err != nil {
		t.Fatal(err)
	}
	if recordByID(activeAssets, "id", assetID) != nil {
		t.Fatalf("archived seal remained in active catalog: %#v", activeAssets)
	}
	allAssets, err := service.SealAssets(ctx, true, fixture.pmID)
	if err != nil {
		t.Fatal(err)
	}
	if archivedAsset := recordByID(allAssets, "id", assetID); archivedAsset == nil || fmt.Sprint(archivedAsset["status"]) != "archived" {
		t.Fatalf("archived seal missing from management catalog: %#v", allAssets)
	}
	if _, err := service.ApplyWorkflowFormSeal(ctx, instanceID, WorkflowFormSealApplyRequest{SealAssetID: assetID, Purpose: "approved_by"}, fixture.workerID, "Worker"); err == nil {
		t.Fatal("expected archived seal to be unavailable for new applications")
	} else {
		assertValidationError(t, err)
	}

	visualFile, err := service.WorkflowFormSealApplicationVisual(ctx, applicationID, fixture.workerID)
	if err != nil {
		t.Fatal(err)
	}
	visualBytes, err := io.ReadAll(visualFile.Reader)
	if err != nil {
		t.Fatal(err)
	}
	if err := visualFile.Reader.Close(); err != nil {
		t.Fatal(err)
	}
	if visualFile.MIMEType != "image/png" || !bytes.HasPrefix(visualBytes, []byte{0x89, 'P', 'N', 'G'}) {
		t.Fatalf("historical seal visual = mime %q bytes %v", visualFile.MIMEType, visualBytes[:min(8, len(visualBytes))])
	}

	form, err = service.SignWorkflowForm(ctx, instanceID, testSignatureRequest("Ký cùng mục đích để chọn thành phần PDF"), fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	form, err = service.SaveWorkflowFormPDFConfirmationPreference(ctx, instanceID, WorkflowFormPDFConfirmationPreferenceRequest{
		Purpose:           "internal_ack",
		IncludeSignatures: false,
		IncludeSeals:      true,
	}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	preferences, ok := form["pdf_confirmation_preferences"].([]Record)
	if !ok || len(preferences) != 1 || boolValue(preferences[0]["include_signatures"]) || !boolValue(preferences[0]["include_seals"]) {
		t.Fatalf("unexpected PDF confirmation preference: %#v", form["pdf_confirmation_preferences"])
	}
	if _, err := service.SaveWorkflowFormPDFConfirmationPreference(ctx, instanceID, WorkflowFormPDFConfirmationPreferenceRequest{
		Purpose:           "internal_ack",
		IncludeSignatures: false,
		IncludeSeals:      false,
	}, fixture.workerID, "Worker"); err == nil {
		t.Fatal("expected at least one PDF confirmation kind to remain selected")
	} else {
		assertValidationError(t, err)
	}

	pdfForm, err := service.GenerateWorkflowFormPDFExport(ctx, instanceID, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	pdfExports, ok := pdfForm["pdf_exports"].([]Record)
	if !ok || len(pdfExports) != 1 {
		t.Fatalf("expected PDF with seal confirmations, got %#v", pdfForm["pdf_exports"])
	}
	exportMetadata := recordFromAny(pdfExports[0]["metadata_json"])
	confirmationSelection := recordFromAny(exportMetadata["confirmation_selection"])
	if number(confirmationSelection["signature_count"]) != 0 || number(confirmationSelection["seal_application_count"]) != 2 {
		t.Fatalf("PDF confirmation selection = %#v, want 0 signatures and 2 seals", confirmationSelection)
	}
	if ids, ok := confirmationSelection["signature_ids"].([]any); !ok || len(ids) != 0 {
		t.Fatalf("selected signature ids = %#v, want empty", confirmationSelection["signature_ids"])
	}
	pdfFile, err := service.WorkflowFormPDFExportFile(ctx, cleanID(pdfExports[0]["id"]), fixture.workerID)
	if err != nil {
		t.Fatal(err)
	}
	pdfBytes, err := io.ReadAll(pdfFile.Reader)
	if err != nil {
		t.Fatal(err)
	}
	if err := pdfFile.Reader.Close(); err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(pdfBytes, []byte("%PDF-")) {
		t.Fatalf("expected PDF bytes, got %q", pdfBytes[:min(8, len(pdfBytes))])
	}

	if _, err := db.ExecContext(ctx, `UPDATE workflow_form_seal_applications SET comment='mutated' WHERE id=$1`, applicationID); err == nil {
		t.Fatal("expected immutable seal application update to fail")
	}
	if _, err := db.ExecContext(ctx, `UPDATE seal_assets SET name='mutated' WHERE id=$1`, assetID); err == nil {
		t.Fatal("expected immutable seal asset content update to fail")
	}
}

func TestWorkflowFormAmendmentCreatesAuditedRevisionWithoutChangingWorkItemState(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")
	service.ConfigureUploads(t.TempDir(), 1024*1024)
	fixture := setupWorkItemFlowFixture(t, db, false)

	formVersionID, err := repo.DefaultWorkflowFormVersionID(ctx, "task", false)
	if err != nil {
		t.Fatal(err)
	}
	instanceRows, err := repo.Query(ctx, `
		INSERT INTO workflow_form_instances (project_id, work_item_id, form_template_id, form_version_id, form_type, title, created_by)
		SELECT $1,$2,v.template_id,v.id,t.form_type,'Amendment form',$3
		FROM workflow_form_versions v
		JOIN workflow_form_templates t ON t.id=v.template_id
		WHERE v.id=$4
		RETURNING id
	`, fixture.projectID, fixture.itemID, fixture.workerID, formVersionID)
	if err != nil {
		t.Fatal(err)
	}
	instanceID := cleanID(instanceRows[0]["id"])
	if _, err := service.SubmitWorkflowForm(ctx, instanceID, WorkflowFormSaveRequest{
		Answers: Record{"summary": "Nội dung ban đầu"},
		Comment: "Chốt lần đầu",
	}, fixture.workerID, "Worker"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.SignWorkflowForm(ctx, instanceID, testSignatureRequest("Ký phiên bản 1"), fixture.workerID, "Worker"); err != nil {
		t.Fatal(err)
	}
	versionOnePDF, err := service.GenerateWorkflowFormPDFExport(ctx, instanceID, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	versionOneExports := workflowAnswerRecordList(versionOnePDF["pdf_exports"])
	if len(versionOneExports) != 1 {
		t.Fatalf("version-one PDF exports = %#v", versionOnePDF["pdf_exports"])
	}
	versionOnePDFID := cleanID(versionOneExports[0]["id"])
	mustExec(t, db, `UPDATE work_items SET status='completed',completed_at=now() WHERE id=$1`, fixture.itemID)

	workItemForm, err := service.WorkItemForm(ctx, fixture.itemID, fixture.workerID)
	if err != nil {
		t.Fatal(err)
	}
	if !boolValue(recordFromAny(workItemForm["capabilities"])["can_amend"]) {
		t.Fatalf("expected server amendment capability, got %#v", workItemForm["capabilities"])
	}

	amended, err := service.AmendWorkflowForm(ctx, instanceID, WorkflowFormAmendRequest{
		Answers:          Record{"summary": "Nội dung đã chỉnh sửa"},
		Reason:           "Bổ sung theo biên bản khảo sát",
		ExpectedRevision: 1,
	}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	instance := recordFromAny(amended["instance"])
	if fmt.Sprint(instance["status"]) != "submitted" || int(number(instance["current_revision"])) != 2 {
		t.Fatalf("amended instance = %#v, want submitted revision 2", instance)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM work_items WHERE id=$1 AND status='completed'`, 1, fixture.itemID)
	assertCount(t, db, `SELECT COUNT(*) FROM workflow_form_answers WHERE instance_id=$1 AND field_key='summary' AND value_json='"Nội dung đã chỉnh sửa"'::jsonb AND revision_no=2`, 1, instanceID)
	assertCount(t, db, `
		SELECT COUNT(*)
		FROM workflow_form_events
		WHERE instance_id=$1
		  AND event_type='amended'
		  AND metadata_json->>'from_revision'='1'
		  AND metadata_json->>'to_revision'='2'
		  AND metadata_json->'changed_fields'->0->>'field_key'='summary'
		  AND metadata_json->'changed_fields'->0->>'before'='Nội dung ban đầu'
		  AND metadata_json->'changed_fields'->0->>'after'='Nội dung đã chỉnh sửa'
	`, 1, instanceID)
	assertCount(t, db, `SELECT COUNT(*) FROM work_item_activities WHERE work_item_id=$1 AND activity_type='system_event' AND comment LIKE 'Đã chỉnh sửa hồ sơ nghiệp vụ v1 → v2:%'`, 1, fixture.itemID)
	assertCount(t, db, `SELECT COUNT(*) FROM audit_logs WHERE target_type='workflow_form_instance' AND target_id=$1 AND event_type='workflow_form_amended'`, 1, instanceID)
	assertCount(t, db, `SELECT COUNT(*) FROM work_items WHERE project_id=$1 AND source_type='workflow_form_action_item'`, 0, fixture.projectID)

	if _, err := service.SignWorkflowForm(ctx, instanceID, testSignatureRequest("Ký phiên bản 2"), fixture.workerID, "Worker"); err != nil {
		t.Fatal(err)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM workflow_form_signatures WHERE form_instance_id=$1 AND signer_user_id=$2 AND purpose='internal_ack'`, 2, instanceID, fixture.workerID)
	if _, err := service.DeleteWorkflowFormPDFExport(ctx, versionOnePDFID, fixture.workerID, "Worker"); err == nil {
		t.Fatal("expected prior-revision PDF deletion to be rejected")
	}
	versionTwoPDF, err := service.GenerateWorkflowFormPDFExport(ctx, instanceID, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	versionTwoExports := workflowAnswerRecordList(versionTwoPDF["pdf_exports"])
	currentExports := 0
	for _, export := range versionTwoExports {
		if int(number(export["form_revision_no"])) != 2 {
			continue
		}
		currentExports++
		selection := recordFromAny(recordFromAny(export["metadata_json"])["confirmation_selection"])
		if int(number(selection["signature_count"])) != 1 {
			t.Fatalf("revision-two PDF confirmation selection = %#v, want one current signature", selection)
		}
	}
	if currentExports != 1 {
		t.Fatalf("current PDF exports = %d, want 1; all=%#v", currentExports, versionTwoExports)
	}

	_, err = service.AmendWorkflowForm(ctx, instanceID, WorkflowFormAmendRequest{
		Answers:          Record{"summary": "Nội dung stale"},
		Reason:           "Ghi đè phiên cũ",
		ExpectedRevision: 1,
	}, fixture.workerID, "Worker")
	var statusErr StatusError
	if !errors.As(err, &statusErr) || statusErr.Code != "CONFLICT" {
		t.Fatalf("stale amendment error = %#v, want conflict", err)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM workflow_form_instances WHERE id=$1 AND current_revision=2`, 1, instanceID)

	_, err = service.AmendWorkflowForm(ctx, instanceID, WorkflowFormAmendRequest{
		Answers:          Record{"summary": "Nội dung đã chỉnh sửa"},
		Reason:           "Không có thay đổi",
		ExpectedRevision: 2,
	}, fixture.workerID, "Worker")
	assertValidationError(t, err)
}

func TestWorkflowFormSignatureRequiresSubmittedFormAndIsImmutable(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	service := NewService(repo, "test-secret")
	storageRoot := t.TempDir()
	service.ConfigureUploads(storageRoot, 1024*1024)
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
	visualBytes, err := io.ReadAll(visualFile.Reader)
	if err != nil {
		t.Fatal(err)
	}
	if err := visualFile.Reader.Close(); err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(visualBytes, []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}) {
		t.Fatalf("expected PNG signature bytes, got %v", visualBytes)
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
	pdfBytes, err := io.ReadAll(pdfFile.Reader)
	if err != nil {
		t.Fatal(err)
	}
	if err := pdfFile.Reader.Close(); err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(pdfBytes, []byte("%PDF-")) {
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
	unconfiguredService := NewService(repo, "test-secret")
	if _, err := unconfiguredService.DeleteWorkflowFormPDFExport(ctx, pdfExportID, fixture.workerID, "Worker"); err == nil {
		t.Fatal("expected PDF deletion with unavailable storage to fail")
	}
	assertCount(t, db, `SELECT COUNT(*) FROM workflow_form_pdf_export_deletions WHERE pdf_export_id=$1 AND status='failed' AND attempt_count=1`, 1, pdfExportID)
	failedDeletionForm, err := repo.GetWorkflowFormInstanceDetail(ctx, instanceID)
	if err != nil {
		t.Fatal(err)
	}
	failedExports, ok := failedDeletionForm["pdf_exports"].([]Record)
	if !ok || len(failedExports) != 1 || fmt.Sprint(failedExports[0]["deletion_status"]) != "failed" {
		t.Fatalf("failed deletion should remain retryable, got %#v", failedDeletionForm["pdf_exports"])
	}

	deletedForm, err := service.DeleteWorkflowFormPDFExport(ctx, pdfExportID, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	remainingExports, ok := deletedForm["pdf_exports"].([]Record)
	if !ok || len(remainingExports) != 0 {
		t.Fatalf("deleted PDF export should be hidden from the form, got %#v", deletedForm["pdf_exports"])
	}
	if _, err := service.WorkflowFormPDFExportFile(ctx, pdfExportID, fixture.workerID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("deleted PDF file lookup error = %#v, want sql.ErrNoRows", err)
	}
	storageKey := fmt.Sprint(pdfExports[0]["storage_key"])
	if _, err := os.Stat(filepath.Join(storageRoot, filepath.FromSlash(storageKey))); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("deleted PDF storage object stat error = %#v, want os.ErrNotExist", err)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM workflow_form_pdf_exports WHERE id=$1`, 1, pdfExportID)
	assertCount(t, db, `SELECT COUNT(*) FROM workflow_form_pdf_export_deletions WHERE pdf_export_id=$1 AND status='deleted'`, 1, pdfExportID)
	assertCount(t, db, `SELECT COUNT(*) FROM workflow_form_events WHERE instance_id=$1 AND event_type='pdf_export_deleted'`, 1, instanceID)
	assertCount(t, db, `SELECT COUNT(*) FROM audit_logs WHERE target_type='workflow_form_pdf_export' AND target_id=$1 AND event_type='workflow_form_pdf_export_deleted'`, 1, pdfExportID)

	if _, err := service.DeleteWorkflowFormPDFExport(ctx, pdfExportID, fixture.workerID, "Worker"); err != nil {
		t.Fatalf("idempotent PDF deletion failed: %v", err)
	}
	assertCount(t, db, `SELECT COUNT(*) FROM workflow_form_events WHERE instance_id=$1 AND event_type='pdf_export_deleted'`, 1, instanceID)

	linkedForm, err := service.GenerateWorkflowFormPDFExport(ctx, instanceID, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	linkedExports, ok := linkedForm["pdf_exports"].([]Record)
	if !ok || len(linkedExports) != 1 {
		t.Fatalf("expected one active replacement PDF export, got %#v", linkedForm["pdf_exports"])
	}
	linkedExportID := cleanID(linkedExports[0]["id"])
	if _, err := db.ExecContext(ctx, `
		INSERT INTO acceptance_records (project_id, work_item_id, form_instance_id, pdf_export_id, title, requested_by)
		VALUES ($1, $2, $3, $4, 'Referenced PDF acceptance', $5)
	`, fixture.projectID, fixture.itemID, instanceID, linkedExportID, fixture.workerID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.DeleteWorkflowFormPDFExport(ctx, linkedExportID, fixture.workerID, "Worker"); err == nil {
		t.Fatal("expected referenced PDF deletion to fail")
	} else {
		var statusErr StatusError
		if !errors.As(err, &statusErr) || statusErr.Code != "CONFLICT" {
			t.Fatalf("referenced PDF deletion error = %#v, want CONFLICT", err)
		}
	}
	assertCount(t, db, `SELECT COUNT(*) FROM workflow_form_pdf_export_deletions WHERE pdf_export_id=$1`, 0, linkedExportID)
	linkedFile, err := service.WorkflowFormPDFExportFile(ctx, linkedExportID, fixture.workerID)
	if err != nil {
		t.Fatalf("referenced PDF should remain readable: %v", err)
	}
	if err := linkedFile.Reader.Close(); err != nil {
		t.Fatal(err)
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
	mustDispatchNotifications(t, service)
	assertCount(t, db, `SELECT COUNT(*) FROM user_notifications WHERE recipient_user_id=$1 AND event_key='work_item.approval_requested' AND source_id=$2 AND resolved_at IS NULL`, 1, fixture.pmID, fixture.itemID)
	assertCount(t, db, `SELECT COUNT(*) FROM user_notifications WHERE recipient_user_id=$1 AND event_key='work_item.approval_requested' AND source_id=$2`, 0, fixture.workerID, fixture.itemID)
	var approvalNotificationID string
	if err := db.QueryRowContext(ctx, `SELECT id::text FROM user_notifications WHERE recipient_user_id=$1 AND event_key='work_item.approval_requested' AND source_id=$2`, fixture.pmID, fixture.itemID).Scan(&approvalNotificationID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.MarkUserNotificationRead(ctx, approvalNotificationID, fixture.pmID); err != nil {
		t.Fatal(err)
	}
	pmSummary, err := service.UserNotificationSummary(ctx, fixture.pmID)
	if err != nil {
		t.Fatal(err)
	}
	if pmSummary.UnreadTotal != 0 || pmSummary.ActionRequired != 1 {
		t.Fatalf("notification summary after read = %#v, want unread=0 action_required=1", pmSummary)
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
	mustDispatchNotifications(t, service)
	assertCount(t, db, `SELECT COUNT(*) FROM user_notifications WHERE recipient_user_id=$1 AND event_key='work_item.approved' AND source_id=$2`, 1, fixture.workerID, fixture.itemID)
	assertCount(t, db, `SELECT COUNT(*) FROM user_notifications WHERE recipient_user_id=$1 AND event_key='work_item.approval_requested' AND source_id=$2 AND resolved_at IS NOT NULL`, 1, fixture.pmID, fixture.itemID)
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
