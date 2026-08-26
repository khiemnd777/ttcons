package core

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestNormalizeMyWorkItemsFilterAndCursor(t *testing.T) {
	defaultFilter, _, err := normalizeMyWorkItemsFilter(MyWorkItemsFilter{})
	if err != nil || defaultFilter.Queue != "responsible" {
		t.Fatalf("default queue = %q, error = %v, want responsible", defaultFilter.Queue, err)
	}
	filter, offset, err := normalizeMyWorkItemsFilter(MyWorkItemsFilter{
		Queue: " Pending_Approval ", Sort: " Due_Asc ", Bucket: " Waiting_Decision ", Limit: 25,
		Cursor: encodeMyWorkItemsCursor(50),
	})
	if err != nil {
		t.Fatal(err)
	}
	if filter.Queue != "pending_approval" || filter.Sort != "due_asc" || filter.Bucket != "waiting_decision" || offset != 50 {
		t.Fatalf("normalized filter = %#v, offset = %d", filter, offset)
	}
	if _, _, err := normalizeMyWorkItemsFilter(MyWorkItemsFilter{Queue: "invalid"}); err == nil {
		t.Fatal("expected invalid queue error")
	}
	if _, _, err := normalizeMyWorkItemsFilter(MyWorkItemsFilter{Cursor: "not-base64"}); err == nil {
		t.Fatal("expected invalid cursor error")
	}
	if _, _, err := normalizeMyWorkItemsFilter(MyWorkItemsFilter{ProjectID: "not-a-uuid"}); err == nil {
		t.Fatal("expected invalid project id error")
	}
	if _, _, err := normalizeMyWorkItemsFilter(MyWorkItemsFilter{Limit: 101}); err == nil {
		t.Fatal("expected invalid limit error")
	}
	for _, bucket := range []string{"all", "in_progress", "upcoming", "completed"} {
		if normalized, _, err := normalizeMyWorkItemsFilter(MyWorkItemsFilter{Bucket: bucket}); err != nil || normalized.Bucket != bucket {
			t.Fatalf("bucket %q normalized to %#v with error %v", bucket, normalized, err)
		}
	}
}

func TestMyWorkCompletedBucketIncludesFullHistoryAndKeepsDoneToday(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	service := NewService(NewRepository(db), "test-secret")
	fixture := setupWorkItemFlowFixture(t, db, false)
	completedID := newID()

	mustExec(t, db, `
		INSERT INTO work_items (
			id,project_id,source_type,source_id,title,work_type,status,role_ids,
			completed_at,created_by,updated_by
		) VALUES (
			$1,$2,'issue',$3,'Completed history','task','completed','["Đội thi công"]'::jsonb,
			now()-interval '2 days',$4,$4
		)
	`, completedID, fixture.projectID, newID(), fixture.workerID)

	allPage, err := service.MyWorkItems(ctx, MyWorkItemsFilter{
		Queue: "responsible", Bucket: "all", ProjectID: fixture.projectID, Limit: 20,
	}, fixture.workerID)
	if err != nil {
		t.Fatal(err)
	}
	summary, _ := allPage["summary"].(Record)
	if number(summary["total"]) != 2 || number(summary["need_action"]) != 1 || number(summary["completed"]) != 1 || number(summary["done_today"]) != 0 {
		t.Fatalf("completed history summary = %#v, want total=2 need_action=1 completed=1 done_today=0", summary)
	}

	completedPage, err := service.MyWorkItems(ctx, MyWorkItemsFilter{
		Queue: "responsible", Bucket: "completed", ProjectID: fixture.projectID, Limit: 20,
	}, fixture.workerID)
	if err != nil {
		t.Fatal(err)
	}
	completedItems, _ := completedPage["items"].([]Record)
	if number(completedPage["total"]) != 1 || len(completedItems) != 1 || cleanID(completedItems[0]["id"]) != completedID {
		t.Fatalf("completed history page = %#v", completedPage)
	}

	doneTodayPage, err := service.MyWorkItems(ctx, MyWorkItemsFilter{
		Queue: "responsible", Bucket: "done_today", ProjectID: fixture.projectID, Limit: 20,
	}, fixture.workerID)
	if err != nil {
		t.Fatal(err)
	}
	if number(doneTodayPage["total"]) != 0 {
		t.Fatalf("done-today compatibility bucket = %#v, want no old history", doneTodayPage)
	}
}

func TestNormalizeMyWorkflowActionsFilter(t *testing.T) {
	filter, err := normalizeMyWorkflowActionsFilter(MyWorkflowActionsFilter{
		Queue: " Responsible ", Search: "  Thi công  ",
	})
	if err != nil {
		t.Fatal(err)
	}
	if filter.Queue != "responsible" || filter.Search != "Thi công" {
		t.Fatalf("normalized workflow action filter = %#v", filter)
	}
	if _, err := normalizeMyWorkflowActionsFilter(MyWorkflowActionsFilter{Queue: "invalid"}); err == nil {
		t.Fatal("expected invalid workflow-action queue error")
	}
	if _, err := normalizeMyWorkflowActionsFilter(MyWorkflowActionsFilter{ProjectID: "not-a-uuid"}); err == nil {
		t.Fatal("expected invalid workflow-action project id error")
	}
}

func TestMyWorkflowAdvanceLabelsUseBusinessLanguage(t *testing.T) {
	tests := []struct {
		name      string
		nextName  string
		nextType  string
		terminal  bool
		wantTitle string
		wantLabel string
	}{
		{name: "construction", nextName: "Thi công", nextType: "construction_execution", wantTitle: "Bắt đầu Thi công", wantLabel: "Chuyển sang Thi công"},
		{name: "handover", nextName: "Kết thúc: Bàn giao", nextType: "handover_finance", wantTitle: "Mở giai đoạn Bàn giao", wantLabel: "Chuyển sang Bàn giao"},
		{name: "generic", nextName: "Khảo sát", nextType: "survey", wantTitle: "Chuyển sang Khảo sát", wantLabel: "Chuyển sang Khảo sát"},
		{name: "terminal", terminal: true, wantTitle: "Hoàn tất quy trình", wantLabel: "Hoàn tất quy trình"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			title, label := myWorkflowAdvanceLabels(test.nextName, test.nextType, test.terminal)
			if title != test.wantTitle || label != test.wantLabel {
				t.Fatalf("labels = %q / %q, want %q / %q", title, label, test.wantTitle, test.wantLabel)
			}
		})
	}
}

func TestMyWorkflowActionsExposeOnlyReadyAuthorizedCurrentNode(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	service := NewService(NewRepository(db), "test-secret")
	fixture := setupWorkItemFlowFixture(t, db, false)

	templateID := newID()
	runID := newID()
	currentNodeID := newID()
	nextNodeID := newID()
	stepID := newID()
	workItemID := newID()
	advancePermissionID := ensurePermission(t, db, "pipeline.advance_node", "Chuyển node", "pipeline")
	operationPermissionID := ensurePermission(t, db, "operation.report", "Tạo daily report", "operation")
	pmRoleID := ensureRole(t, db, "pm", "PM")
	mustExec(t, db, `INSERT INTO role_permissions (role_id,permission_id) VALUES ($1,$2),($1,$3) ON CONFLICT DO NOTHING`, pmRoleID, advancePermissionID, operationPermissionID)

	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `UPDATE projects SET current_pipeline_run_id=NULL WHERE id=$1`, fixture.projectID)
		_, _ = db.ExecContext(ctx, `DELETE FROM pipeline_runs WHERE id=$1`, runID)
		_, _ = db.ExecContext(ctx, `DELETE FROM pipeline_templates WHERE id=$1`, templateID)
	})
	mustExec(t, db, `
		INSERT INTO pipeline_templates (id,name,status,template_family_id,revision_no,engine_version)
		VALUES ($1,'My Work transition','published',$1,1,2)
	`, templateID)
	mustExec(t, db, `
		INSERT INTO pipeline_runs (id,project_id,template_id,status,engine_version)
		VALUES ($1,$2,$3,'in_progress',2)
	`, runID, fixture.projectID, templateID)
	mustExec(t, db, `
		INSERT INTO pipeline_run_nodes (
			id,run_id,project_id,name,type,business_code,order_index,responsible_role_ids,
			required_permission,permissions,status,started_at
		) VALUES
			($1,$3,$4,'Kế hoạch thi công','planning','planning',1,'["PM"]'::jsonb,'pipeline.advance_node','["pipeline.advance_node"]'::jsonb,'in_progress',now()),
			($2,$3,$4,'Thi công','construction_execution','construction_execution',2,'["Giám sát"]'::jsonb,'operation.report','["operation.report"]'::jsonb,'pending',NULL)
	`, currentNodeID, nextNodeID, runID, fixture.projectID)
	mustExec(t, db, `UPDATE pipeline_runs SET current_node_id=$2 WHERE id=$1`, runID, currentNodeID)
	mustExec(t, db, `UPDATE projects SET current_pipeline_run_id=$2 WHERE id=$1`, fixture.projectID, runID)
	mustExec(t, db, `
		INSERT INTO pipeline_run_edges (run_id,project_id,from_run_node_id,to_run_node_id,condition_type,label)
		VALUES ($1,$2,$3,$4,'always','Bắt đầu thi công')
	`, runID, fixture.projectID, currentNodeID, nextNodeID)
	mustExec(t, db, `
		INSERT INTO pipeline_run_steps (
			id,run_node_id,project_id,name,step_type,required_role_ids,required_permission,
			checklist_required,order_index,status,submitted_by,submitted_at
		) VALUES ($1,$2,$3,'Kích hoạt bàn giao thi công','task','["PM"]'::jsonb,'pipeline.advance_node',false,1,'completed',$4,now())
	`, stepID, currentNodeID, fixture.projectID, fixture.pmID)
	mustExec(t, db, `
		INSERT INTO work_items (
			id,project_id,pipeline_run_id,run_node_id,run_step_id,source_type,source_id,title,
			work_type,status,role_ids,completed_at,created_by,updated_by
		) VALUES ($1,$2,$3,$4,$5,'pipeline_run_step',$5,'Kích hoạt bàn giao thi công','task','completed','["PM"]'::jsonb,now(),$6,$6)
	`, workItemID, fixture.projectID, runID, currentNodeID, stepID, fixture.pmID)

	page, err := service.MyWorkflowActions(ctx, MyWorkflowActionsFilter{
		Queue: "responsible", ProjectID: fixture.projectID,
	}, fixture.pmID)
	if err != nil {
		t.Fatal(err)
	}
	actions, _ := page["items"].([]Record)
	if len(actions) != 1 || number(page["total"]) != 1 {
		t.Fatalf("ready PM workflow actions = %#v", page)
	}
	if fmt.Sprint(actions[0]["title"]) != "Bắt đầu Thi công" || cleanID(actions[0]["run_node_id"]) != currentNodeID {
		t.Fatalf("ready workflow action = %#v", actions[0])
	}

	workerPage, err := service.MyWorkflowActions(ctx, MyWorkflowActionsFilter{
		Queue: "all", ProjectID: fixture.projectID,
	}, fixture.workerID)
	if err != nil {
		t.Fatal(err)
	}
	if number(workerPage["total"]) != 0 {
		t.Fatalf("unauthorized worker workflow actions = %#v", workerPage)
	}

	mustExec(t, db, `UPDATE pipeline_run_nodes SET responsible_role_ids='[]'::jsonb WHERE id=$1`, currentNodeID)
	unassignedPage, err := service.MyWorkflowActions(ctx, MyWorkflowActionsFilter{
		Queue: "responsible", ProjectID: fixture.projectID,
	}, fixture.pmID)
	if err != nil {
		t.Fatal(err)
	}
	if number(unassignedPage["total"]) != 1 {
		t.Fatalf("authorized unassigned workflow actions = %#v", unassignedPage)
	}
	mustExec(t, db, `UPDATE pipeline_run_nodes SET status='blocked',block_reason='Tạm dừng' WHERE id=$1`, currentNodeID)
	blockedNodePage, err := service.MyWorkflowActions(ctx, MyWorkflowActionsFilter{
		Queue: "responsible", ProjectID: fixture.projectID,
	}, fixture.pmID)
	if err != nil {
		t.Fatal(err)
	}
	if number(blockedNodePage["total"]) != 0 {
		t.Fatalf("blocked node workflow actions = %#v", blockedNodePage)
	}
	mustExec(t, db, `UPDATE pipeline_run_nodes SET status='in_progress',block_reason=NULL,responsible_role_ids='["PM"]'::jsonb WHERE id=$1`, currentNodeID)

	mustExec(t, db, `UPDATE pipeline_run_steps SET status='pending',submitted_by=NULL,submitted_at=NULL WHERE id=$1`, stepID)
	mustExec(t, db, `UPDATE work_items SET status='ready',completed_at=NULL WHERE id=$1`, workItemID)
	blockedPage, err := service.MyWorkflowActions(ctx, MyWorkflowActionsFilter{
		Queue: "responsible", ProjectID: fixture.projectID,
	}, fixture.pmID)
	if err != nil {
		t.Fatal(err)
	}
	if number(blockedPage["total"]) != 0 {
		t.Fatalf("incomplete node workflow actions = %#v", blockedPage)
	}

	mustExec(t, db, `UPDATE pipeline_run_steps SET status='completed',submitted_by=$2,submitted_at=now() WHERE id=$1`, stepID, fixture.pmID)
	mustExec(t, db, `UPDATE work_items SET status='completed',completed_at=now() WHERE id=$1`, workItemID)
	if _, err := service.AdvanceRunNode(ctx, currentNodeID, fixture.pmID, "PM"); err != nil {
		t.Fatal(err)
	}
	afterAdvance, err := service.MyWorkflowActions(ctx, MyWorkflowActionsFilter{
		Queue: "responsible", ProjectID: fixture.projectID,
	}, fixture.pmID)
	if err != nil {
		t.Fatal(err)
	}
	if number(afterAdvance["total"]) != 0 {
		t.Fatalf("advanced node workflow actions = %#v", afterAdvance)
	}
}

func TestMyWorkItemReadinessSeparatesCurrentFutureAndHistory(t *testing.T) {
	tests := []struct {
		name string
		item Record
		want string
	}{
		{name: "actionable", item: Record{"status": "ready", "can_act": true, "is_future": false}, want: "actionable"},
		{name: "decision", item: Record{"status": "waiting_approval", "can_decide": true}, want: "decision"},
		{name: "upcoming", item: Record{"status": "pending", "can_act": false, "is_future": true}, want: "upcoming"},
		{name: "read only", item: Record{"status": "pending", "can_act": false, "is_future": false}, want: "read_only"},
		{name: "history wins", item: Record{"status": "completed", "can_act": true, "can_decide": true}, want: "history"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := myWorkItemReadiness(test.item); got != test.want {
				t.Fatalf("readiness = %q, want %q", got, test.want)
			}
		})
	}
}

func TestMyWorkFutureNodeCannotActNow(t *testing.T) {
	service := &Service{}
	allowed, err := service.myWorkItemCanActNow(context.Background(), Record{
		"status":                   "pending",
		"pipeline_engine_version":  2,
		"pipeline_run_id":          "run-1",
		"pipeline_current_node_id": "node-current",
		"run_node_id":              "node-future",
		"run_node_status":          "pending",
	}, "actor-1", true)
	if err != nil || allowed {
		t.Fatalf("future node allowed = %v, error = %v", allowed, err)
	}
}

func TestMyWorkEarlyStartedFutureNodeCanActNow(t *testing.T) {
	service := &Service{}
	allowed, err := service.myWorkItemCanActNow(context.Background(), Record{
		"status":                   "in_progress",
		"started_at":               time.Now().Add(-time.Minute),
		"planned_start_at":         time.Now().Add(time.Hour),
		"pipeline_engine_version":  2,
		"pipeline_run_id":          "run-1",
		"pipeline_current_node_id": "node-current",
		"run_node_id":              "node-future",
		"run_node_status":          "pending",
	}, "actor-1", true)
	if err != nil || !allowed {
		t.Fatalf("early-started future node allowed = %v, error = %v", allowed, err)
	}
}

func TestMyWorkPlannedFutureCannotActNow(t *testing.T) {
	service := &Service{}
	planned := time.Now().Add(time.Hour)
	allowed, err := service.myWorkItemCanActNow(context.Background(), Record{
		"status":           "ready",
		"planned_start_at": planned,
	}, "actor-1", true)
	if err != nil || allowed {
		t.Fatalf("future planned start allowed = %v, error = %v", allowed, err)
	}
	if !myWorkPlannedStartInFuture(planned.Format(time.RFC3339Nano), time.Now()) {
		t.Fatal("RFC3339 planned start should be recognized as future")
	}
	if err := service.requireActivePipelineWorkItem(context.Background(), Record{
		"id":               "work-1",
		"planned_start_at": planned,
	}); err == nil {
		t.Fatal("future planned start must fail the shared mutation guard")
	}
}

func TestMyWorkSQLExposesWorkflowContextAndFutureState(t *testing.T) {
	query := myWorkItemsBaseSQL()
	for _, fragment := range []string{
		"rn.order_index AS run_node_order_index",
		"rs.order_index AS run_step_order_index",
		") AS is_future",
	} {
		if !strings.Contains(query, fragment) {
			t.Fatalf("My Work SQL missing %q", fragment)
		}
	}
	if !strings.Contains(myWorkNeedActionSQL(), "AND NOT is_future") {
		t.Fatal("need-action bucket must exclude future work")
	}
}

func TestMyWorkDecisionSQLPreservesAuthoritativeScopeParity(t *testing.T) {
	query := myWorkItemsBaseSQL()
	if strings.Contains(query, "status IN ('waiting_review','waiting_approval') AND NOT is_watching") {
		t.Fatal("decision SQL must not deny an otherwise-authorized actor only because they watch the item")
	}
	for _, fragment := range []string{
		"NULLIF(ar.required_roles, '[]'::jsonb)",
		"scoped_role.project_id=sw.project_id",
		"actor_staff.status='active' AND pm.project_id=sw.project_id",
	} {
		if !strings.Contains(query, fragment) {
			t.Fatalf("decision SQL missing parity fragment %q", fragment)
		}
	}
}

func TestReadTransactionDisablesJITLocally(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	ctx := context.Background()

	var before string
	if err := db.QueryRowContext(ctx, `SELECT current_setting('jit')`).Scan(&before); err != nil {
		t.Fatal(err)
	}

	for _, finish := range []string{"commit", "rollback"} {
		t.Run(finish, func(t *testing.T) {
			tx, err := NewRepository(db).beginReadTransactionWithJITDisabled(ctx)
			if err != nil {
				t.Fatal(err)
			}
			var inside, readOnly string
			if err := tx.QueryRowContext(ctx, `SELECT current_setting('jit'), current_setting('transaction_read_only')`).Scan(&inside, &readOnly); err != nil {
				_ = tx.Rollback()
				t.Fatal(err)
			}
			if inside != "off" {
				_ = tx.Rollback()
				t.Fatalf("transaction jit = %q, want off", inside)
			}
			if readOnly != "on" {
				_ = tx.Rollback()
				t.Fatalf("transaction_read_only = %q, want on", readOnly)
			}
			if finish == "commit" {
				err = tx.Commit()
			} else {
				err = tx.Rollback()
			}
			if err != nil {
				t.Fatal(err)
			}

			var after string
			if err := db.QueryRowContext(ctx, `SELECT current_setting('jit')`).Scan(&after); err != nil {
				t.Fatal(err)
			}
			if after != before {
				t.Fatalf("connection jit after transaction = %q, want original %q", after, before)
			}
		})
	}
}

func TestActivityAttentionQuerySelectsCandidatesBeforeEnrichment(t *testing.T) {
	query, args := activityAttentionCandidatesSQL("10000000-0000-0000-0000-000000000001", workItemAccessProfile{
		UserID:  "10000000-0000-0000-0000-000000000001",
		StaffID: "20000000-0000-0000-0000-000000000002",
	}, ActivityAttentionFilter{Queue: "all", Limit: 120})
	if len(args) == 0 || args[len(args)-1] != 120 {
		t.Fatalf("candidate args = %#v", args)
	}
	for _, fragment := range []string{
		"WITH attention_candidates AS MATERIALIZED",
		"selected_attention AS MATERIALIZED",
		"candidate.mentioned_me OR candidate.is_question OR candidate.is_blocker OR candidate.is_reminder",
		"LIMIT $",
	} {
		if !strings.Contains(query, fragment) {
			t.Fatalf("candidate query missing %q", fragment)
		}
	}
	for _, enrichment := range []string{"attachment_state", "conversation_state", "activity_reactions"} {
		if strings.Contains(query, enrichment) {
			t.Fatalf("candidate query must not enrich %q", enrichment)
		}
	}
}

func TestMyWorkItemAvailableActionsAreCapabilityAndStatusBound(t *testing.T) {
	ready := Record{"status": "ready", "can_act": true, "can_decide": false}
	actions := myWorkItemAvailableActions(ready)
	if !containsMyWorkString(actions, "start") || !containsMyWorkString(actions, "complete") || containsMyWorkString(actions, "approve") {
		t.Fatalf("ready actions = %#v", actions)
	}
	waiting := Record{"status": "waiting_review", "can_act": false, "can_decide": true}
	actions = myWorkItemAvailableActions(waiting)
	if !containsMyWorkString(actions, "approve") || !containsMyWorkString(actions, "reject") || containsMyWorkString(actions, "start") {
		t.Fatalf("waiting actions = %#v", actions)
	}
	terminal := Record{"status": "completed", "can_act": true, "can_decide": true}
	if actions := myWorkItemAvailableActions(terminal); len(actions) != 0 {
		t.Fatalf("terminal actions = %#v, want none", actions)
	}
}

func TestMyActivityAttentionStreamMapsWorkItemInvalidation(t *testing.T) {
	eventName, output := myActivityAttentionStreamEvent(ActivityStreamEvent{
		Type: "work_item_changed", ProjectID: "project-1", WorkItemID: "work-item-1",
	})
	if eventName != "work_queue_changed" || output.Type != "work_queue_changed" || output.ProjectID != "project-1" {
		t.Fatalf("mapped event = %q %#v", eventName, output)
	}
	if output.WorkItemID != "" {
		t.Fatalf("work queue invalidation leaked unverified item id: %#v", output)
	}
	eventName, output = myActivityAttentionStreamEvent(ActivityStreamEvent{Type: "work_queue_changed", ProjectID: "project-1"})
	if eventName != "work_queue_changed" || output.Type != "work_queue_changed" || output.ProjectID != "project-1" {
		t.Fatalf("direct work queue invalidation = %q %#v", eventName, output)
	}
	eventName, output = myActivityAttentionStreamEvent(ActivityStreamEvent{Type: "activity_changed", ProjectID: "project-1"})
	if eventName != "attention_changed" || output.Type != "attention_changed" {
		t.Fatalf("activity mapping = %q %#v", eventName, output)
	}
}

func TestMyWorkItemsPageContractPaginationAndDecisionCapability(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	service := NewService(NewRepository(db), "test-secret")
	fixture := setupWorkItemFlowFixture(t, db, false)

	for index, status := range []string{"ready", "blocked"} {
		itemID := newID()
		mustExec(t, db, `
			INSERT INTO work_items
				(id, project_id, source_type, source_id, title, work_type, status, priority, role_ids, evidence_requirement_json, created_by, updated_by)
			VALUES
				($1,$2,'issue',$3,$4,'task',$5,'high','["Đội thi công"]'::jsonb,'{"required_permission":"operation.report"}'::jsonb,$6,$6)
		`, itemID, fixture.projectID, newID(), "My Work page item "+string(rune('A'+index)), status, fixture.workerID)
	}

	page, err := service.MyWorkItems(ctx, MyWorkItemsFilter{ProjectID: fixture.projectID, Limit: 1}, fixture.workerID)
	if err != nil {
		t.Fatal(err)
	}
	items, ok := page["items"].([]Record)
	if !ok || len(items) != 1 {
		t.Fatalf("items = %#v", page["items"])
	}
	if number(page["total"]) < 3 || !boolValue(page["has_more"]) || strings.TrimSpace(cleanID(page["next_cursor"])) == "" {
		t.Fatalf("pagination contract = %#v", page)
	}
	if _, ok := items[0]["capabilities"].(Record); !ok {
		t.Fatalf("capabilities = %#v", items[0]["capabilities"])
	}
	if _, ok := page["summary"].(Record); !ok {
		t.Fatalf("summary = %#v", page["summary"])
	}
	if _, ok := page["facets"].(Record); !ok {
		t.Fatalf("facets = %#v", page["facets"])
	}
	if _, err := service.WorkItemAction(ctx, fixture.itemID, "request-review", WorkItemActionRequest{}, fixture.workerID, "Worker"); err != nil {
		t.Fatal(err)
	}
	personalDecisionPage, err := service.MyWorkItems(ctx, MyWorkItemsFilter{
		ProjectID: fixture.projectID, Queue: "responsible", Bucket: "waiting_decision", Limit: 20,
	}, fixture.pmID)
	if err != nil {
		t.Fatal(err)
	}
	if number(personalDecisionPage["total"]) != 1 {
		t.Fatalf("waiting-decision bucket must ignore the ordinary responsibility queue: %#v", personalDecisionPage)
	}
	personalDecisionSummary, _ := personalDecisionPage["summary"].(Record)
	if number(personalDecisionSummary["total"]) != 0 || number(personalDecisionSummary["waiting_decision"]) != 1 {
		t.Fatalf("decision summary must remain actor-decidable beside responsible totals: %#v", personalDecisionSummary)
	}
	var pmStaffID, workerRoleID string
	if err := db.QueryRowContext(ctx, `SELECT id::text FROM staff WHERE user_id=$1`, fixture.pmID).Scan(&pmStaffID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `SELECT role_id::text FROM user_roles WHERE user_id=$1 AND project_id IS NULL LIMIT 1`, fixture.workerID).Scan(&workerRoleID); err != nil {
		t.Fatal(err)
	}
	outsideUserID := newID()
	outsideStaffID := newID()
	suffix := newID()
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `UPDATE work_items SET assignee_staff_id=NULL WHERE id=$1`, fixture.itemID)
		_, _ = db.ExecContext(ctx, `DELETE FROM user_roles WHERE user_id=$1`, outsideUserID)
		_, _ = db.ExecContext(ctx, `DELETE FROM staff WHERE id=$1`, outsideStaffID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id=$1`, outsideUserID)
	})
	mustExec(t, db, `INSERT INTO users (id,email,password_hash,full_name,status) VALUES ($1,$2,'demo:pw','Outside role','active')`, outsideUserID, "my-work-outside-"+suffix+"@test.local")
	mustExec(t, db, `INSERT INTO staff (id,user_id,full_name,email,department,position,status) VALUES ($1,$2,'Outside role',$3,'Thi công','Worker','active')`, outsideStaffID, outsideUserID, "my-work-outside-staff-"+suffix+"@test.local")
	mustExec(t, db, `INSERT INTO user_roles (user_id,role_id) VALUES ($1,$2)`, outsideUserID, workerRoleID)
	mustExec(t, db, `UPDATE work_items SET assignee_staff_id=$2, watcher_staff_ids=jsonb_build_array($3::text) WHERE id=$1`, fixture.itemID, outsideStaffID, pmStaffID)

	decisionPage, err := service.MyWorkItems(ctx, MyWorkItemsFilter{
		ProjectID: fixture.projectID, Queue: "pending_approval", Limit: 20,
	}, fixture.pmID)
	if err != nil {
		t.Fatal(err)
	}
	decisionItems, ok := decisionPage["items"].([]Record)
	if !ok || len(decisionItems) != 1 {
		t.Fatalf("decision items = %#v", decisionPage["items"])
	}
	capabilities, _ := decisionItems[0]["capabilities"].(Record)
	if !boolValue(capabilities["can_decide"]) || !containsMyWorkString(stringSlice(decisionItems[0]["available_actions"]), "approve") {
		t.Fatalf("decision item = %#v", decisionItems[0])
	}
	outsidePage, err := service.MyWorkItems(ctx, MyWorkItemsFilter{
		ProjectID: fixture.projectID, Queue: "pending_approval", Limit: 20,
	}, outsideUserID)
	if err != nil {
		t.Fatal(err)
	}
	if number(outsidePage["total"]) != 0 {
		t.Fatalf("global-role actor without project scope pending total = %v, want 0", outsidePage["total"])
	}

	requesterPage, err := service.MyWorkItems(ctx, MyWorkItemsFilter{
		ProjectID: fixture.projectID, Queue: "pending_approval", Limit: 20,
	}, fixture.workerID)
	if err != nil {
		t.Fatal(err)
	}
	if number(requesterPage["total"]) != 0 {
		t.Fatalf("requester pending approval total = %v, want 0", requesterPage["total"])
	}
	navigationSummary, err := service.MyWorkNavigationSummary(ctx, fixture.workerID)
	if err != nil {
		t.Fatal(err)
	}
	workItemsNeedAction := int(number(navigationSummary["work_items_need_action"]))
	workflowActionsReady := int(number(navigationSummary["workflow_actions_ready"]))
	workTotal := int(number(navigationSummary["work_total"]))
	approvalTotal := int(number(navigationSummary["approval_total"]))
	if workItemsNeedAction < 1 || workTotal != workItemsNeedAction+workflowActionsReady || int(number(navigationSummary["total"])) != workTotal+approvalTotal {
		t.Fatalf("my work navigation summary = %#v", navigationSummary)
	}
}

func TestMyWorkNavigationSummaryTotalsOnlyWorkAndApprovalLanes(t *testing.T) {
	summary := myWorkNavigationSummaryRecord(myWorkNavigationCounts{
		NeedAction:      2,
		WaitingDecision: 3,
	}, 4, 5, 6)

	if number(summary["work_total"]) != 6 || number(summary["approval_total"]) != 14 || number(summary["total"]) != 20 {
		t.Fatalf("my work navigation totals = %#v, want work 6 + approvals 14 = 20", summary)
	}
	if _, leakedAttention := summary["attention_total"]; leakedAttention {
		t.Fatalf("my work navigation summary must exclude attention: %#v", summary)
	}
}

func TestMyWorkNavigationNeedActionExcludesDecisionLane(t *testing.T) {
	query := myWorkNavigationNeedActionSQL()
	if !strings.Contains(query, "status NOT IN ('waiting_review','waiting_approval','completed','cancelled')") {
		t.Fatalf("navigation predicate = %q", query)
	}
	if strings.Contains(query, "can_decide") {
		t.Fatalf("navigation predicate leaked the separate decision lane: %q", query)
	}
}

func TestMyWorkResponsibleScopeExcludesAnotherAssigneeFromSameDepartment(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	service := NewService(NewRepository(db), "test-secret")
	suffix := newID()
	projectID := newID()
	actorID := newID()
	actorStaffID := newID()
	colleagueID := newID()
	colleagueStaffID := newID()
	actorRoleID := ensureRole(t, db, "scope_actor_"+suffix[:8], "Scope Actor "+suffix[:8])
	colleagueRoleID := ensureRole(t, db, "scope_colleague_"+suffix[:8], "Scope Colleague "+suffix[:8])
	department := "Shared Scope " + suffix[:8]

	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM projects WHERE id=$1`, projectID)
		_, _ = db.ExecContext(ctx, `DELETE FROM user_roles WHERE user_id IN ($1,$2)`, actorID, colleagueID)
		_, _ = db.ExecContext(ctx, `DELETE FROM staff WHERE id IN ($1,$2)`, actorStaffID, colleagueStaffID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id IN ($1,$2)`, actorID, colleagueID)
		_, _ = db.ExecContext(ctx, `DELETE FROM roles WHERE id IN ($1,$2)`, actorRoleID, colleagueRoleID)
	})

	mustExec(t, db, `INSERT INTO users (id,email,password_hash,full_name,status) VALUES ($1,$2,'demo:pw','Scope Actor','active'),($3,$4,'demo:pw','Scope Colleague','active')`, actorID, "scope-actor-"+suffix+"@test.local", colleagueID, "scope-colleague-"+suffix+"@test.local")
	mustExec(t, db, `INSERT INTO staff (id,user_id,full_name,email,department,status) VALUES ($1,$2,'Scope Actor',$3,$4,'active'),($5,$6,'Scope Colleague',$7,$4,'active')`, actorStaffID, actorID, "scope-actor-staff-"+suffix+"@test.local", department, colleagueStaffID, colleagueID, "scope-colleague-staff-"+suffix+"@test.local")
	mustExec(t, db, `INSERT INTO projects (id,name,code,client_name,status) VALUES ($1,'My Work Scope',$2,'Client','planning')`, projectID, "MWS-"+suffix[:8])
	mustExec(t, db, `INSERT INTO project_members (project_id,staff_id,role_ids,active) VALUES ($1,$2,jsonb_build_array($3::text),true),($1,$4,jsonb_build_array($5::text),true)`, projectID, actorStaffID, "Scope Actor "+suffix[:8], colleagueStaffID, "Scope Colleague "+suffix[:8])
	mustExec(t, db, `INSERT INTO user_roles (user_id,role_id,project_id) VALUES ($1,$2,$3),($4,$5,$3)`, actorID, actorRoleID, projectID, colleagueID, colleagueRoleID)

	insertFutureItem := func(title, workDepartment, roleName, assigneeStaffID string) string {
		t.Helper()
		itemID := newID()
		mustExec(t, db, `
			INSERT INTO work_items (
				id,project_id,source_type,source_id,title,work_type,status,priority,department,role_ids,
				assignee_staff_id,planned_start_at,created_by,updated_by
			) VALUES (
				$1,$2,'issue',$3,$4,'task','pending','normal',NULLIF($5,''),
				CASE WHEN NULLIF($6,'') IS NULL THEN '[]'::jsonb ELSE jsonb_build_array($6) END,
				NULLIF($7,'')::uuid,now()+interval '1 day',$8,$8
			)
		`, itemID, projectID, newID(), title, workDepartment, roleName, assigneeStaffID, actorID)
		return itemID
	}
	actorRoleName := "Scope Actor " + suffix[:8]
	colleagueRoleName := "Scope Colleague " + suffix[:8]
	directID := insertFutureItem("Direct responsibility", department, actorRoleName, actorStaffID)
	roleID := insertFutureItem("Role responsibility", "", actorRoleName, "")
	departmentPoolID := insertFutureItem("Department pool responsibility", department, "", "")
	colleagueItemID := insertFutureItem("Other assignee same department", department, colleagueRoleName, colleagueStaffID)
	mustExec(t, db, `
		INSERT INTO work_item_assignments (work_item_id,assignment_type,staff_id,role_name,department,created_by) VALUES
			($1,'assignee',$2,NULL,NULL,$3),
			($1,'role',NULL,$4,NULL,$3),
			($5,'role',NULL,$4,NULL,$3),
			($6,'department',NULL,NULL,$7,$3),
			($8,'assignee',$9,NULL,NULL,$3),
			($8,'role',NULL,$10,NULL,$3)
	`, directID, actorStaffID, actorID, actorRoleName, roleID, departmentPoolID, department, colleagueItemID, colleagueStaffID, colleagueRoleName)

	page, err := service.MyWorkItems(ctx, MyWorkItemsFilter{
		Queue: "responsible", Bucket: "all", ProjectID: projectID, Limit: 20,
	}, actorID)
	if err != nil {
		t.Fatal(err)
	}
	items, _ := page["items"].([]Record)
	for _, expectedID := range []string{directID, roleID, departmentPoolID} {
		item := recordByID(items, "id", expectedID)
		if item == nil {
			t.Fatalf("responsible queue missing %s: %#v", expectedID, page)
		}
		if !stringSliceContains(stringSlice(item["queue_scopes"]), "responsible") {
			t.Fatalf("responsible queue scope missing for %s: %#v", expectedID, item["queue_scopes"])
		}
	}
	if recordByID(items, "id", colleagueItemID) != nil {
		t.Fatalf("another assignee's same-department item leaked into responsible queue: %#v", items)
	}
	if number(page["total"]) != 3 {
		t.Fatalf("responsible total = %v, want 3", page["total"])
	}
	summary, _ := page["summary"].(Record)
	if number(summary["total"]) != 3 || number(summary["upcoming"]) != 3 {
		t.Fatalf("responsible summary = %#v, want total=3 upcoming=3", summary)
	}

	departmentPage, err := service.MyWorkItems(ctx, MyWorkItemsFilter{
		Queue: "department", Bucket: "all", ProjectID: projectID, Limit: 20,
	}, actorID)
	if err != nil {
		t.Fatal(err)
	}
	departmentItems, _ := departmentPage["items"].([]Record)
	if recordByID(departmentItems, "id", colleagueItemID) == nil || recordByID(departmentItems, "id", departmentPoolID) == nil {
		t.Fatalf("department queue must expose same-department colleague and pool work: %#v", departmentPage)
	}
	if recordByID(departmentItems, "id", directID) != nil || recordByID(departmentItems, "id", roleID) != nil {
		t.Fatalf("department queue must not duplicate direct or role-only responsibility: %#v", departmentItems)
	}
	if number(departmentPage["total"]) != 2 {
		t.Fatalf("department total = %v, want 2", departmentPage["total"])
	}

	allPage, err := service.MyWorkItems(ctx, MyWorkItemsFilter{
		Queue: "all", Bucket: "all", ProjectID: projectID, Limit: 20,
	}, actorID)
	if err != nil {
		t.Fatal(err)
	}
	if number(allPage["total"]) != 4 {
		t.Fatalf("all-scope total = %v, want 4", allPage["total"])
	}
}

func TestPendingApprovalsExposeActorCapabilities(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	service := NewService(NewRepository(db), "test-secret")
	fixture := setupWorkItemFlowFixture(t, db, false)

	waiting, err := service.WorkItemAction(ctx, fixture.itemID, "request-approval", WorkItemActionRequest{
		Comment:       "PM duyệt giúp",
		RequiredRoles: []string{"PM"},
	}, fixture.workerID, "Worker")
	if err != nil {
		t.Fatal(err)
	}
	approvalID := cleanID(waiting["approval_request_id"])
	if approvalID == "" {
		t.Fatal("expected approval_request_id")
	}

	pmRows, err := service.PendingApprovals(ctx, PendingApprovalFilter{
		ProjectID: fixture.projectID, DecidableOnly: true,
	}, fixture.pmID)
	if err != nil {
		t.Fatal(err)
	}
	if len(pmRows) != 1 || cleanID(pmRows[0]["id"]) != approvalID {
		t.Fatalf("PM pending approvals = %#v", pmRows)
	}
	capabilities, _ := pmRows[0]["capabilities"].(Record)
	if !boolValue(capabilities["can_decide"]) {
		t.Fatalf("PM capabilities = %#v", pmRows[0]["capabilities"])
	}
	actions := stringSlice(pmRows[0]["available_actions"])
	for _, expected := range []string{"approve", "request-change", "reject"} {
		if !containsMyWorkString(actions, expected) {
			t.Fatalf("PM available actions = %#v, missing %s", actions, expected)
		}
	}
	decisionContext, _ := pmRows[0]["decision_context"].(Record)
	if !stringSliceContains(stringSlice(decisionContext["required_roles"]), "PM") {
		t.Fatalf("decision context = %#v", decisionContext)
	}
	if fmt.Sprint(pmRows[0]["target_name"]) != "Batch3 work item" {
		t.Fatalf("target name = %#v", pmRows[0]["target_name"])
	}

	requesterRows, err := service.PendingApprovals(ctx, PendingApprovalFilter{
		ProjectID: fixture.projectID, DecidableOnly: true,
	}, fixture.workerID)
	if err != nil {
		t.Fatal(err)
	}
	if len(requesterRows) != 0 {
		t.Fatalf("requester decidable approvals = %#v, want none", requesterRows)
	}
	visibleRows, err := service.PendingApprovals(ctx, PendingApprovalFilter{ProjectID: fixture.projectID}, fixture.workerID)
	if err != nil {
		t.Fatal(err)
	}
	if len(visibleRows) != 1 {
		t.Fatalf("requester visible approvals = %#v", visibleRows)
	}
	requesterCapabilities, _ := visibleRows[0]["capabilities"].(Record)
	if boolValue(requesterCapabilities["can_decide"]) || len(stringSlice(visibleRows[0]["available_actions"])) != 0 {
		t.Fatalf("requester approval contract = %#v", visibleRows[0])
	}
}

func containsMyWorkString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
