package core

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestCreateCompletedSampleDataProjectIntegration(t *testing.T) {
	if os.Getenv("CONSFLOW_TEST_DATABASE_URL") == "" {
		t.Skip("set CONSFLOW_TEST_DATABASE_URL to run DB-backed sample data tests")
	}
	database := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(database)
	service := NewService(repo, "sample-data-test-secret")
	service.ConfigureUploads(t.TempDir(), defaultMaxUploadBytes)

	var actorID, actorName string
	if err := database.QueryRowContext(ctx, `
		SELECT id::text,full_name FROM users WHERE email='developer@consflow.local' AND status='active'
	`).Scan(&actorID, &actorName); err != nil {
		t.Fatal(err)
	}
	pipeline, accountingBook := sampleDataIntegrationSources(t, ctx, service, true)
	blueprint, err := repo.LoadSampleDataBlueprint(ctx, pipeline.ID)
	if err != nil {
		t.Fatal(err)
	}
	totalSteps := 0
	for _, node := range blueprint.Nodes {
		totalSteps += len(node.Steps)
	}
	idempotencyKey := uuid.NewString()
	t.Cleanup(func() { cancelSampleDataIntegration(t, database, idempotencyKey) })

	result, err := service.CreateSampleDataProject(ctx, SampleDataCreateRequest{
		Profile: "completed", ProgressPercent: 100, PipelineTemplateID: pipeline.ID,
		AccountingBookID: accountingBook.ID, IdempotencyKey: idempotencyKey,
	}, actorID, actorName)
	if err != nil {
		t.Fatal(err)
	}
	projectID := cleanID(result["id"])
	if projectID == "" || !boolValue(result["is_sample"]) || int(number(result["progress_percent"])) != 100 ||
		cleanID(result["pipeline_template_id"]) != pipeline.ID || cleanID(result["accounting_book_id"]) != accountingBook.ID {
		t.Fatalf("unexpected completed sample result: %#v", result)
	}
	replayed, err := service.CreateSampleDataProject(ctx, SampleDataCreateRequest{
		Profile: "completed", ProgressPercent: 100, PipelineTemplateID: pipeline.ID,
		AccountingBookID: accountingBook.ID, IdempotencyKey: idempotencyKey,
	}, actorID, actorName)
	if err != nil || cleanID(replayed["id"]) != projectID || !boolValue(replayed["replayed"]) {
		t.Fatalf("completed idempotency replay mismatch: result=%#v err=%v", replayed, err)
	}
	for table, minimum := range map[string]int{
		"pipeline_runs": 1, "work_items": totalSteps, "workflow_form_instances": 1,
		"evidence_records": pipeline.SelectedStepCount, "approval_requests": 1, "project_material_costs": 1,
		"payables": 1, "receivables": 1, "journal_entries": 2, "handover_records": 1,
	} {
		var count int
		if err := database.QueryRowContext(ctx, `SELECT count(*) FROM `+table+` WHERE project_id=$1`, projectID).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count < minimum {
			t.Fatalf("%s count=%d, want at least %d", table, count, minimum)
		}
	}
}

func TestCreatePlanningSampleDataAssignmentsIntegration(t *testing.T) {
	if os.Getenv("CONSFLOW_TEST_DATABASE_URL") == "" {
		t.Skip("set CONSFLOW_TEST_DATABASE_URL to run DB-backed sample data tests")
	}
	database := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(database)
	service := NewService(repo, "sample-data-test-secret")
	service.ConfigureUploads(t.TempDir(), defaultMaxUploadBytes)

	var actorID, actorName string
	if err := database.QueryRowContext(ctx, `
		SELECT id::text,full_name FROM users WHERE email='developer@consflow.local' AND status='active'
	`).Scan(&actorID, &actorName); err != nil {
		t.Fatal(err)
	}
	pipeline, _ := sampleDataIntegrationSources(t, ctx, service, false)
	blueprint, err := repo.LoadSampleDataBlueprint(ctx, pipeline.ID)
	if err != nil {
		t.Fatal(err)
	}
	totalSteps := 0
	for _, node := range blueprint.Nodes {
		totalSteps += len(node.Steps)
	}
	idempotencyKey := uuid.NewString()
	t.Cleanup(func() { cancelSampleDataIntegration(t, database, idempotencyKey) })
	result, err := service.CreateSampleDataProject(ctx, SampleDataCreateRequest{
		Profile: "planning", ProgressPercent: 65, PipelineTemplateID: pipeline.ID, IdempotencyKey: idempotencyKey,
	}, actorID, actorName)
	if err != nil {
		t.Fatal(err)
	}
	projectID := cleanID(result["id"])
	replayed, err := service.CreateSampleDataProject(ctx, SampleDataCreateRequest{
		Profile: "planning", ProgressPercent: 65, PipelineTemplateID: pipeline.ID, IdempotencyKey: idempotencyKey,
	}, actorID, actorName)
	if err != nil || cleanID(replayed["id"]) != projectID || !boolValue(replayed["replayed"]) {
		t.Fatalf("planning idempotency replay mismatch: result=%#v err=%v", replayed, err)
	}
	var completed, ready, pending, cancelled int
	if err := database.QueryRowContext(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE status='completed'),
			COUNT(*) FILTER (WHERE status='ready'),
			COUNT(*) FILTER (WHERE status='pending'),
			COUNT(*) FILTER (WHERE status='cancelled')
		FROM work_items WHERE project_id=$1
	`, projectID).Scan(&completed, &ready, &pending, &cancelled); err != nil {
		t.Fatal(err)
	}
	expectedCompleted := 65 * pipeline.SelectedStepCount / 100
	expectedPending := pipeline.SelectedStepCount - expectedCompleted - 1
	expectedCancelled := totalSteps - pipeline.SelectedStepCount
	if completed != expectedCompleted || ready != 1 || pending != expectedPending || cancelled != expectedCancelled {
		t.Fatalf("planning 65%% status counts completed=%d ready=%d pending=%d cancelled=%d", completed, ready, pending, cancelled)
	}

	var workItemID, roleName, assigneeUserID string
	if err := database.QueryRowContext(ctx, `
		SELECT work.id::text,assignment.role_name,staff.user_id::text
		FROM work_items work
		JOIN work_item_assignments assignment ON assignment.work_item_id=work.id AND assignment.assignment_type='role'
		JOIN roles role ON role.display_name=assignment.role_name
		JOIN user_roles user_role ON user_role.role_id=role.id AND user_role.project_id IS NULL
		JOIN staff ON staff.user_id=user_role.user_id AND staff.status='active'
		JOIN users actor ON actor.id=staff.user_id AND actor.status='active'
		WHERE work.project_id=$1 AND work.status='ready'
		ORDER BY staff.id
		LIMIT 1
	`, projectID).Scan(&workItemID, &roleName, &assigneeUserID); err != nil {
		t.Fatal(err)
	}
	myWork, err := service.MyWorkItems(ctx, MyWorkItemsFilter{
		Queue: "all", Bucket: "all", ProjectID: projectID, Limit: 100,
	}, assigneeUserID)
	if err != nil {
		t.Fatal(err)
	}
	items, _ := myWork["items"].([]Record)
	if recordByID(items, "id", workItemID) == nil {
		t.Fatalf("ready work item %s for role %s is missing from assignee My Work: %#v", workItemID, roleName, myWork)
	}
}

func TestSampleDataPipelineCompatibilityDoesNotDependOnEnvironmentIDsOrRevision(t *testing.T) {
	roleName := "Vai trò môi trường kiểm thử"
	actor := sampleDataStaffActor{
		StaffID: uuid.NewString(), UserID: uuid.NewString(), FullName: "Nhân sự kiểm thử",
		RoleID: uuid.NewString(), RoleName: roleName, RoleKey: "environment_role",
	}
	step := func(name, surface, action string) sampleDataStepDefinition {
		return sampleDataStepDefinition{Record: Record{
			"id": uuid.NewString(), "name": name, "required_role_ids": []string{roleName},
			"surface_kind": surface, "domain_action_key": action, "form_required": false,
		}}
	}
	blueprint := sampleDataBlueprint{
		Template: Record{
			"id": uuid.NewString(), "name": "Pipeline imported in another environment", "status": "published",
			"revision_no": 3, "engine_version": 1, "definition_hash": "environment-specific-hash",
		},
		Actors: map[string][]sampleDataStaffActor{roleName: {actor}},
		Nodes: []sampleDataNodeDefinition{
			{Record: Record{"id": uuid.NewString(), "business_code": "boq_without_design"}},
			{Record: Record{"id": uuid.NewString(), "business_code": "construction_execution"}, Steps: []sampleDataStepDefinition{
				step("Thi công 1", "", ""), step("Thi công 2", "", ""),
			}},
			{Record: Record{"id": uuid.NewString(), "business_code": "incoming_material"}, Steps: []sampleDataStepDefinition{
				step("Nghiệm thu đầu vào", "incoming_acceptance", "acceptance.incoming_material"),
			}},
			{Record: Record{"id": uuid.NewString(), "business_code": "outgoing_acceptance"}, Steps: []sampleDataStepDefinition{
				step("Nghiệm thu đầu ra", "", ""),
			}},
			{Record: Record{"id": uuid.NewString(), "business_code": "handover"}, Steps: []sampleDataStepDefinition{
				step("Xác nhận bàn giao", "handover", "source.handover_completed"),
			}},
		},
	}

	option := sampleDataPipelineCompatibility(blueprint)
	if !option.Compatible {
		t.Fatalf("portable pipeline should be compatible, blockers=%#v", option.Blockers)
	}
	if option.ID != cleanID(blueprint.Template["id"]) || option.Revision != 3 || option.SelectedStepCount != 5 {
		t.Fatalf("unexpected dynamic pipeline option: %#v", option)
	}
	blueprint.Template["definition_hash"] = nil
	missingHash := sampleDataPipelineCompatibility(blueprint)
	if missingHash.Compatible || len(missingHash.Blockers) != 1 || missingHash.Blockers[0].Code != "PIPELINE_HASH_MISSING" {
		t.Fatalf("nil definition hash must fail closed: %#v", missingHash)
	}
	blueprint.Template["definition_hash"] = option.DefinitionHash

	accounting := &sampleDataAccountingContext{
		ID: uuid.NewString(), BankAccountID: uuid.NewString(), CostAccountID: uuid.NewString(), RevenueAccountID: uuid.NewString(),
	}
	plan, err := buildSampleDataPlan(blueprint, accounting, SampleDataCreateRequest{
		Profile: "planning", ProgressPercent: 65, PipelineTemplateID: option.ID,
	}, actor.UserID, actor.FullName, uuid.NewString(), time.Date(2026, 8, 23, 10, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	completed := 0
	for _, node := range plan.Nodes {
		for _, plannedStep := range node.Steps {
			if plannedStep.Completed {
				completed++
			}
		}
	}
	if plan.SelectedStepCount != 5 || completed != 3 {
		t.Fatalf("dynamic progress should floor 65%% of 5 steps to 3, got selected=%d completed=%d", plan.SelectedStepCount, completed)
	}
}

func sampleDataIntegrationSources(t *testing.T, ctx context.Context, service *Service, requireAccounting bool) (SampleDataPipelineOption, SampleDataAccountingBookOption) {
	t.Helper()
	options, err := service.SampleDataOptions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var selectedPipeline SampleDataPipelineOption
	for _, pipeline := range options.Pipelines {
		if pipeline.Compatible {
			selectedPipeline = pipeline
			break
		}
	}
	if selectedPipeline.ID == "" {
		t.Fatalf("test environment has no compatible published Pipeline: %#v", options.Pipelines)
	}
	var selectedBook SampleDataAccountingBookOption
	for _, book := range options.AccountingBooks {
		if book.Compatible {
			selectedBook = book
			break
		}
	}
	if requireAccounting && selectedBook.ID == "" {
		t.Fatalf("test environment has no compatible accounting book: %#v", options.AccountingBooks)
	}
	return selectedPipeline, selectedBook
}

func cancelSampleDataIntegration(t *testing.T, database *sql.DB, idempotencyKey string) {
	t.Helper()
	ctx := context.Background()
	var projectID string
	if err := database.QueryRowContext(ctx, `
		SELECT COALESCE(project_id::text,'') FROM sample_data_generations WHERE idempotency_key=$1
	`, idempotencyKey).Scan(&projectID); err != nil || projectID == "" {
		return
	}
	suffix := sampleCodeSuffix(projectID)
	if _, err := database.ExecContext(ctx, `UPDATE projects SET status='cancelled',updated_at=now() WHERE id=$1`, projectID); err != nil {
		t.Errorf("cancel sample integration project: %v", err)
	}
	if _, err := database.ExecContext(ctx, `DELETE FROM sample_data_generations WHERE idempotency_key=$1`, idempotencyKey); err != nil {
		t.Errorf("delete sample integration generation: %v", err)
	}
	_, _ = database.ExecContext(ctx, `UPDATE materials SET active=false WHERE code=$1`, "DEMO-MAT-"+suffix)
	_, _ = database.ExecContext(ctx, `UPDATE warehouses SET active=false WHERE name=$1`, "Kho công trình mẫu "+suffix)
	_, _ = database.ExecContext(ctx, `UPDATE partners SET active=false WHERE partner_code IN ($1,$2)`, "DEMO-CL-"+suffix, "DEMO-SP-"+suffix)
}
