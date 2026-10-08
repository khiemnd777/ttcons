package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

func TestCreateDraftSampleDataProjectIntegration(t *testing.T) {
	if os.Getenv("CONSFLOW_TEST_DATABASE_URL") == "" {
		t.Skip("set CONSFLOW_TEST_DATABASE_URL to run DB-backed sample data tests")
	}
	database := openProjectOperationsTestDB(t)
	ctx := context.Background()
	service := NewService(NewRepository(database), "sample-data-test-secret")
	actorID, actorName := setupSampleDataDeveloperActor(t, database)
	pipeline := sampleDataIntegrationSources(t, ctx, service, actorID, actorName, false)
	key := uuid.NewString()
	t.Cleanup(func() { cancelSampleDataIntegration(t, database, key) })
	req := SampleDataCreateRequest{Profile: "draft", PipelineTemplateID: pipeline.ID, IdempotencyKey: key}
	result, err := service.CreateSampleDataProject(ctx, req, actorID, actorName)
	if err != nil || cleanID(result["id"]) == "" || result["status"] != "draft" || number(result["progress_percent"]) != 0 || cleanID(result["current_pipeline_run_id"]) != "" {
		t.Fatalf("book-free draft must not create runtime work: result=%#v err=%v", result, err)
	}
	replayed, err := service.CreateSampleDataProject(ctx, req, actorID, actorName)
	if err != nil || cleanID(replayed["id"]) != cleanID(result["id"]) || !boolValue(replayed["replayed"]) {
		t.Fatalf("draft retry must replay original project: result=%#v err=%v", replayed, err)
	}
	req.Profile, req.ProgressPercent = "planning", 5
	if _, err := service.CreateSampleDataProject(ctx, req, actorID, actorName); err == nil || !strings.Contains(err.Error(), "idempotency") {
		t.Fatalf("changed-profile idempotency retry must conflict: %v", err)
	}
}

func TestCreateCompletedSampleDataProjectIntegration(t *testing.T) {
	if os.Getenv("CONSFLOW_TEST_DATABASE_URL") == "" {
		t.Skip("set CONSFLOW_TEST_DATABASE_URL to run DB-backed sample data tests")
	}
	database := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(database)
	service := NewService(repo, "sample-data-test-secret")
	service.ConfigureUploads(t.TempDir(), defaultMaxUploadBytes)

	actorID, actorName := setupSampleDataDeveloperActor(t, database)
	pipeline := sampleDataIntegrationSources(t, ctx, service, actorID, actorName, true)
	reprovisioned, err := service.CreateSampleDataUsers(ctx, SampleDataUserCreateRequest{PipelineTemplateID: pipeline.ID}, actorID, actorName)
	if err != nil {
		t.Fatal(err)
	}
	if int(number(reprovisioned["created_count"])) != 0 || int(number(reprovisioned["existing_count"])) != len(pipeline.RoleCoverage) {
		t.Fatalf("sample user provisioning must be idempotent: %#v", reprovisioned)
	}
	var sampleUserCount, sampleRoleCount, globalRoleCount, mustChangeCount int
	var passwordHash string
	if err := database.QueryRowContext(ctx, `
		SELECT COUNT(*),COUNT(DISTINCT sample_role_id),
			COUNT(*) FILTER (WHERE must_change_password),
			COALESCE((SELECT password_hash FROM users WHERE is_sample ORDER BY created_at,id LIMIT 1),'')
		FROM users WHERE is_sample
	`).Scan(&sampleUserCount, &sampleRoleCount, &mustChangeCount, &passwordHash); err != nil {
		t.Fatal(err)
	}
	if err := database.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM user_roles assignment JOIN users account ON account.id=assignment.user_id
		WHERE account.is_sample AND assignment.project_id IS NULL
	`).Scan(&globalRoleCount); err != nil {
		t.Fatal(err)
	}
	if sampleUserCount == 0 || sampleUserCount != sampleRoleCount || mustChangeCount != 0 || globalRoleCount != 0 ||
		bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(sampleDataUserPassword)) != nil {
		t.Fatalf("sample user provisioning mismatch users=%d roles=%d must_change=%d global_roles=%d", sampleUserCount, sampleRoleCount, mustChangeCount, globalRoleCount)
	}
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
		IdempotencyKey: idempotencyKey,
	}, actorID, actorName)
	if err != nil {
		t.Fatal(err)
	}
	projectID := cleanID(result["id"])
	if projectID == "" || !boolValue(result["is_sample"]) || int(number(result["progress_percent"])) != 100 ||
		cleanID(result["pipeline_template_id"]) != pipeline.ID {
		t.Fatalf("unexpected completed sample result: %#v", result)
	}
	assertSampleRunStepWorkItemCompletionSync(t, database, projectID)
	replayed, err := service.CreateSampleDataProject(ctx, SampleDataCreateRequest{
		Profile: "completed", ProgressPercent: 100, PipelineTemplateID: pipeline.ID,
		IdempotencyKey: idempotencyKey,
	}, actorID, actorName)
	if err != nil || cleanID(replayed["id"]) != projectID || !boolValue(replayed["replayed"]) {
		t.Fatalf("completed idempotency replay mismatch: result=%#v err=%v", replayed, err)
	}
	for table, minimum := range map[string]int{
		"pipeline_runs": 1, "work_items": totalSteps, "workflow_form_instances": 1,
		"evidence_records": pipeline.SelectedStepCount, "approval_requests": 1, "handover_records": 1,
		"accounting_handoffs": 2, "boq_releases": 1, "construction_handoffs": 1,
		"subcontractor_checklist_reconciliations": 1, "owner_client_checklist_reconciliations": 1,
	} {
		var count int
		if err := database.QueryRowContext(ctx, `SELECT count(*) FROM `+table+` WHERE project_id=$1`, projectID).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count < minimum {
			t.Fatalf("%s count=%d, want at least %d", table, count, minimum)
		}
	}
	assertSampleCurrentDomainGates(t, ctx, repo, projectID)
	for _, table := range []string{"payables", "receivables", "payments", "project_material_costs", "project_cash_requests", "journal_entries", "accounting_misa_records"} {
		var exists bool
		if err := database.QueryRowContext(ctx, `SELECT to_regclass($1) IS NOT NULL`, "public."+table).Scan(&exists); err != nil {
			t.Fatal(err)
		}
		if !exists {
			continue
		}
		var count int
		if err := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+table+` WHERE project_id=$1`, projectID).Scan(&count); err != nil || count != 0 {
			t.Fatalf("sample must not fabricate %s rows: count=%d err=%v", table, count, err)
		}
	}
}

func assertSampleCurrentDomainGates(t *testing.T, ctx context.Context, repo *Repository, projectID string) {
	t.Helper()
	steps, err := repo.Query(ctx, `SELECT work.id,work.project_id,step.domain_action_key FROM work_items work JOIN pipeline_run_steps step ON step.id=work.run_step_id WHERE work.project_id=$1 AND work.status='completed' AND step.domain_action_key IS NOT NULL`, projectID)
	if err != nil {
		t.Fatal(err)
	}
	for _, step := range steps {
		action := cleanOptionalText(step["domain_action_key"])
		var outcome workflowAutomationOutcome
		switch action {
		case "contract.require_execution_ready":
			outcome, err = repo.automateClientContractExecutionGate(ctx, step)
		case "procurement.generate_from_boq":
			outcome, err = repo.automateClientBOQProcurement(ctx, step, "", "")
		default:
			outcome, err = repo.automateClientOperationalSourceGate(ctx, step, action)
		}
		if err != nil || outcome.Status != "succeeded" {
			t.Fatalf("canonical source gate %s did not pass: outcome=%#v err=%v", action, outcome, err)
		}
	}
}

func TestCompletedSampleDataSourceRollbackIntegration(t *testing.T) {
	if os.Getenv("CONSFLOW_TEST_DATABASE_URL") == "" {
		t.Skip("set CONSFLOW_TEST_DATABASE_URL to run DB-backed sample data tests")
	}
	database := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(database)
	service := NewService(repo, "sample-data-test-secret")
	service.ConfigureUploads(t.TempDir(), defaultMaxUploadBytes)
	actorID, actorName := setupSampleDataDeveloperActor(t, database)
	pipeline := sampleDataIntegrationSources(t, ctx, service, actorID, actorName, true)
	blueprint, err := repo.LoadSampleDataBlueprint(ctx, pipeline.ID)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := buildSampleDataPlan(blueprint, SampleDataCreateRequest{Profile: "completed", ProgressPercent: 100, PipelineTemplateID: pipeline.ID}, actorID, actorName, uuid.NewString(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.storeSampleDataArtifacts(ctx, &plan); err != nil {
		t.Fatal(err)
	}
	// Inject a late unsupported gate after valid domain sources are generated.
	// The transaction must roll back the project, memberships and every source.
	injected := false
	for nodeIndex := range plan.Nodes {
		for stepIndex := range plan.Nodes[nodeIndex].Steps {
			step := &plan.Nodes[nodeIndex].Steps[stepIndex]
			if !injected && step.Selected && cleanOptionalText(step.Definition.Record["domain_action_key"]) == "" {
				step.Definition.Record["domain_action_key"] = "source.unimplemented_sample_gate"
				injected = true
			}
		}
	}
	if !injected {
		t.Fatal("fixture has no form-only step to inject source gate")
	}
	if _, err := repo.CreateSampleData(ctx, plan); err == nil || !strings.Contains(err.Error(), "bộ kiểm chứng") {
		t.Fatalf("unknown source gate must fail closed: %v", err)
	}
	for _, table := range []string{"projects", "pipeline_runs", "work_items", "accounting_handoffs", "contracts", "boq_releases", "construction_handoffs", "handover_records"} {
		key := "project_id"
		if table == "projects" {
			key = "id"
		}
		var count int
		if err := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+table+` WHERE `+key+`=$1`, plan.ProjectID).Scan(&count); err != nil || count != 0 {
			t.Fatalf("failed sample leaked %s rows: count=%d err=%v", table, count, err)
		}
	}
	var generationCount int
	if err := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM sample_data_generations WHERE idempotency_key=$1`, plan.IdempotencyKey).Scan(&generationCount); err != nil || generationCount != 0 {
		t.Fatalf("failed sample leaked generation: count=%d err=%v", generationCount, err)
	}
}

func TestSampleDataContractHasNoAccountingPrerequisites(t *testing.T) {
	for _, value := range []any{SampleDataCreateRequest{Profile: "draft"}, SampleDataOptions{}} {
		payload, err := json.Marshal(value)
		if err != nil || strings.Contains(string(payload), "accounting_book") {
			t.Fatalf("sample API must not require or expose accounting books: %s err=%v", payload, err)
		}
	}
}

func TestSampleStructuredListPreservesSourceFieldTypes(t *testing.T) {
	value := sampleFormFieldValue(Record{
		"field_type": "structured_list",
		"validation_json": map[string]any{"item_fields": []any{
			map[string]any{"key": "responsible_role", "type": "select", "options": []any{map[string]any{"value": "QA/QC", "label": "QA/QC"}}},
			map[string]any{"key": "evidence_required", "type": "checkbox"},
			map[string]any{"key": "scheduled_date", "type": "date"},
		}},
	}, "DEMO", time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC))
	items := recordsFromAny(value)
	if len(items) != 1 || items[0]["responsible_role"] != "QA/QC" || items[0]["evidence_required"] != true || items[0]["scheduled_date"] != "2026-10-05" {
		t.Fatalf("structured source rows must retain selectable role, boolean and date values: %#v", value)
	}
}

func TestPlanningSampleDataConcurrentRetryIntegration(t *testing.T) {
	if os.Getenv("CONSFLOW_TEST_DATABASE_URL") == "" {
		t.Skip("set CONSFLOW_TEST_DATABASE_URL to run DB-backed sample data tests")
	}
	database := openProjectOperationsTestDB(t)
	ctx := context.Background()
	service := NewService(NewRepository(database), "sample-data-test-secret")
	uploads := t.TempDir()
	service.ConfigureUploads(uploads, defaultMaxUploadBytes)
	actorID, actorName := setupSampleDataDeveloperActor(t, database)
	pipeline := sampleDataIntegrationSources(t, ctx, service, actorID, actorName, false)
	key := uuid.NewString()
	t.Cleanup(func() { cancelSampleDataIntegration(t, database, key) })
	type result struct {
		record Record
		err    error
	}
	results := make(chan result, 2)
	start := make(chan struct{})
	for attempt := 0; attempt < 2; attempt++ {
		go func() {
			<-start
			record, err := service.CreateSampleDataProject(ctx, SampleDataCreateRequest{Profile: "planning", ProgressPercent: 25, PipelineTemplateID: pipeline.ID, IdempotencyKey: key}, actorID, actorName)
			results <- result{record, err}
		}()
	}
	close(start)
	first, second := <-results, <-results
	if first.err != nil || second.err != nil || cleanID(first.record["id"]) == "" || cleanID(first.record["id"]) != cleanID(second.record["id"]) {
		t.Fatalf("concurrent retries must yield one project: first=%#v second=%#v", first, second)
	}
	var count int
	if err := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM sample_data_generations WHERE idempotency_key=$1`, key).Scan(&count); err != nil || count != 1 {
		t.Fatalf("concurrent generation count=%d err=%v", count, err)
	}
	projectID := cleanID(first.record["id"])
	if err := filepath.Walk(uploads, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && !strings.Contains(path, projectID) {
			t.Errorf("concurrent retry left an unreferenced artifact: %s", path)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
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

	actorID, actorName := setupSampleDataDeveloperActor(t, database)
	pipeline := sampleDataIntegrationSources(t, ctx, service, actorID, actorName, false)
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
	assertSampleRunStepWorkItemCompletionSync(t, database, projectID)
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
		SELECT work.id::text,role_assignment.role_name,staff.user_id::text
		FROM work_items work
		JOIN work_item_assignments role_assignment ON role_assignment.work_item_id=work.id AND role_assignment.assignment_type='role'
		JOIN work_item_assignments assignee_assignment ON assignee_assignment.work_item_id=work.id AND assignee_assignment.assignment_type='assignee'
		JOIN staff ON staff.id=assignee_assignment.staff_id AND staff.status='active'
		JOIN users actor ON actor.id=staff.user_id AND actor.status='active' AND actor.is_sample
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
	var realMembers, unscopedRoles, scopedRoles, realBusinessActors int
	if err := database.QueryRowContext(ctx, `
		SELECT
			(SELECT COUNT(*) FROM project_members member JOIN staff profile ON profile.id=member.staff_id JOIN users account ON account.id=profile.user_id WHERE member.project_id=$1 AND NOT account.is_sample),
			(SELECT COUNT(*) FROM project_members member JOIN staff profile ON profile.id=member.staff_id JOIN user_roles assignment ON assignment.user_id=profile.user_id WHERE member.project_id=$1 AND assignment.project_id IS NULL),
			(SELECT COUNT(*) FROM project_members member JOIN staff profile ON profile.id=member.staff_id JOIN user_roles assignment ON assignment.user_id=profile.user_id WHERE member.project_id=$1 AND assignment.project_id=$1),
			(SELECT COUNT(*) FROM work_items work JOIN users business_actor ON business_actor.id=work.created_by WHERE work.project_id=$1 AND NOT business_actor.is_sample)
	`, projectID).Scan(&realMembers, &unscopedRoles, &scopedRoles, &realBusinessActors); err != nil {
		t.Fatal(err)
	}
	if realMembers != 0 || unscopedRoles != 0 || scopedRoles == 0 || realBusinessActors != 0 {
		t.Fatalf("sample isolation mismatch real_members=%d unscoped_roles=%d scoped_roles=%d real_business_actors=%d", realMembers, unscopedRoles, scopedRoles, realBusinessActors)
	}
}

func assertSampleRunStepWorkItemCompletionSync(t *testing.T, database *sql.DB, projectID string) {
	t.Helper()
	var mismatched int
	if err := database.QueryRowContext(context.Background(), `
		SELECT COUNT(*)
		FROM pipeline_run_steps rs
		JOIN work_items wi ON wi.run_step_id=rs.id AND wi.project_id=rs.project_id
		WHERE rs.project_id=$1
		  AND ((rs.status='completed') <> (wi.status='completed'))
	`, projectID).Scan(&mismatched); err != nil {
		t.Fatal(err)
	}
	if mismatched != 0 {
		t.Fatalf("sample project %s has %d Work Item/Run Step terminal completion mismatches", projectID, mismatched)
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
		Roles: []sampleDataRoleAccount{{
			RoleID: actor.RoleID, RoleKey: actor.RoleKey, RoleName: actor.RoleName,
			UserID: actor.UserID, StaffID: actor.StaffID, Email: sampleRoleEmail(actor.RoleKey),
			UserStatus: "active", StaffStatus: "active",
		}},
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
	if option.CompletedCompatible || !hasSampleDataBlocker(option.CompletedBlockers, "COMPLETED_SOURCE_BINDING_MISSING") {
		t.Fatalf("planning compatibility must not imply completed source coverage: %#v", option)
	}
	blueprint.Template["definition_hash"] = nil
	missingHash := sampleDataPipelineCompatibility(blueprint)
	if missingHash.Compatible || len(missingHash.Blockers) != 1 || missingHash.Blockers[0].Code != "PIPELINE_HASH_MISSING" {
		t.Fatalf("nil definition hash must fail closed: %#v", missingHash)
	}
	blueprint.Template["definition_hash"] = option.DefinitionHash
	readyRole := blueprint.Roles[0]
	blueprint.Roles[0].UserID = ""
	blueprint.Roles[0].StaffID = ""
	blueprint.Roles[0].Email = ""
	blueprint.Roles[0].EmailConflict = true
	blueprint.Actors = map[string][]sampleDataStaffActor{}
	conflictingEmail := sampleDataPipelineCompatibility(blueprint)
	if !conflictingEmail.StructureCompatible || conflictingEmail.SampleUsersProvisionable || conflictingEmail.SampleUsersReady ||
		!hasSampleDataBlocker(conflictingEmail.Blockers, "SAMPLE_EMAIL_CONFLICT") {
		t.Fatalf("sample email conflicts must block provisioning without changing structural compatibility: %#v", conflictingEmail)
	}
	blueprint.Roles[0] = readyRole
	blueprint.Actors = map[string][]sampleDataStaffActor{roleName: {actor}}

	plan, err := buildSampleDataPlan(blueprint, SampleDataCreateRequest{
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
				if plannedStep.Status != "completed" || plannedStep.WorkStatus != "completed" {
					t.Fatalf("completed sample step lifecycle mismatch: run_step=%s work_item=%s step=%s", plannedStep.Status, plannedStep.WorkStatus, plannedStep.Definition.Record["name"])
				}
			}
		}
	}
	if plan.SelectedStepCount != 5 || completed != 3 {
		t.Fatalf("dynamic progress should floor 65%% of 5 steps to 3, got selected=%d completed=%d", plan.SelectedStepCount, completed)
	}
	if plan.ActorID != actor.UserID || plan.GeneratorID != actor.UserID {
		t.Fatalf("sample actor/generator identity mismatch: %#v", plan)
	}
}

func hasSampleDataBlocker(blockers []SampleDataBlocker, code string) bool {
	for _, blocker := range blockers {
		if blocker.Code == code {
			return true
		}
	}
	return false
}

func sampleDataIntegrationSources(t *testing.T, ctx context.Context, service *Service, actorID, actorName string, requireCompleted bool) SampleDataPipelineOption {
	t.Helper()
	options, err := service.SampleDataOptions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var selectedPipeline SampleDataPipelineOption
	for _, pipeline := range options.Pipelines {
		if (pipeline.Compatible || pipeline.SampleUsersProvisionable) && (!requireCompleted || len(pipeline.CompletedBlockers) == 0) {
			selectedPipeline = pipeline
			break
		}
	}
	if selectedPipeline.ID == "" {
		t.Fatalf("test environment has no provisionable published Pipeline: %#v", options.Pipelines)
	}
	if _, err := service.CreateSampleDataUsers(ctx, SampleDataUserCreateRequest{PipelineTemplateID: selectedPipeline.ID}, actorID, actorName); err != nil {
		t.Fatalf("provision sample users: %v", err)
	}
	options, err = service.SampleDataOptions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	selectedPipeline = SampleDataPipelineOption{}
	for _, pipeline := range options.Pipelines {
		if pipeline.Compatible && (!requireCompleted || pipeline.CompletedCompatible) {
			selectedPipeline = pipeline
			break
		}
	}
	if selectedPipeline.ID == "" {
		t.Fatalf("test environment has no compatible Pipeline after provisioning: %#v", options.Pipelines)
	}
	return selectedPipeline
}

func setupSampleDataDeveloperActor(t *testing.T, database *sql.DB) (string, string) {
	t.Helper()
	ctx := context.Background()
	actorID := newID()
	actorName := "Sample Data Test Developer"
	roleID := ensureRole(t, database, "developer", "Nhà phát triển hệ thống")
	permissionID := ensurePermission(t, database, "sample_data.create", "Tạo dữ liệu công trình mẫu", "settings")
	mustExec(t, database, `INSERT INTO role_permissions (role_id,permission_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`, roleID, permissionID)
	mustExec(t, database, `
		INSERT INTO users (id,email,password_hash,full_name,status)
		VALUES ($1,$2,'demo:pw',$3,'active')
	`, actorID, "sample-data-developer-"+actorID+"@test.local", actorName)
	mustExec(t, database, `INSERT INTO user_roles (user_id,role_id) VALUES ($1,$2)`, actorID, roleID)
	t.Cleanup(func() {
		_, _ = database.ExecContext(ctx, `DELETE FROM user_roles WHERE user_id=$1`, actorID)
		_, _ = database.ExecContext(ctx, `DELETE FROM users WHERE id=$1`, actorID)
	})
	return actorID, actorName
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
