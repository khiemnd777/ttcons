package core

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestPipelineEdgeConditionCatalogUsesSchemaAndHidesLegacyProjectRouting(t *testing.T) {
	var workflowFormCondition *EdgeConditionDefinition
	for index := range PipelineEdgeConditionCatalog() {
		definition := PipelineEdgeConditionCatalog()[index]
		if definition.Type == "project_entry_mode" {
			t.Fatal("legacy project_entry_mode must not be exposed for new template authoring")
		}
		if definition.Type == "workflow_form_answer_equals" {
			workflowFormCondition = &definition
		}
	}
	if workflowFormCondition == nil {
		t.Fatal("workflow_form_answer_equals condition is missing")
	}
	if len(workflowFormCondition.ConfigSchema) != 2 {
		t.Fatalf("expected two schema-driven config fields, got %d", len(workflowFormCondition.ConfigSchema))
	}
	if workflowFormCondition.ConfigSchema[0].Key != "field_key" || workflowFormCondition.ConfigSchema[1].DependsOn != "field_key" {
		t.Fatalf("unexpected connector config schema: %#v", workflowFormCondition.ConfigSchema)
	}
}

func TestWorkflowFormRulesUseArbitraryFieldKeysAndValues(t *testing.T) {
	fields := []Record{
		{
			"field_key": "route_choice", "required": true,
			"validation_json": Record{}, "visibility_condition_json": Record{},
		},
		{
			"field_key": "source_document", "required": false,
			"validation_json":           Record{"required_when": Record{"operator": "equals", "field_key": "route_choice", "value": "direct"}},
			"visibility_condition_json": Record{"operator": "equals", "field_key": "route_choice", "value": "direct"},
		},
	}
	if err := validateWorkflowFormFieldRules(fields); err != nil {
		t.Fatalf("generic form rule must validate: %v", err)
	}

	visible, err := workflowFormFieldVisible(fields[1], Record{"route_choice": "direct"})
	if err != nil || !visible {
		t.Fatalf("dependent field should be visible: visible=%v err=%v", visible, err)
	}
	required, err := workflowFormFieldRequired(fields[1], Record{"route_choice": "direct"})
	if err != nil || !required {
		t.Fatalf("dependent field should be required: required=%v err=%v", required, err)
	}

	visible, err = workflowFormFieldVisible(fields[1], Record{"route_choice": "survey"})
	if err != nil || visible {
		t.Fatalf("dependent field should be hidden: visible=%v err=%v", visible, err)
	}
}

func TestLegacyEdgeEvaluatorIsIsolatedFromAuthoringRegistry(t *testing.T) {
	if _, exists := runEdgeConditionEvaluators["project_entry_mode"]; exists {
		t.Fatal("legacy routing must not be in the normal runtime registry")
	}
	if _, exists := legacyRunEdgeConditionEvaluators["project_entry_mode"]; !exists {
		t.Fatal("published legacy snapshots still need their compatibility evaluator")
	}
}

func TestWorkflowFormAnswerEdgeUsesSubmittedNodeAnswer(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()

	projectCode := fmt.Sprintf("RULE-%d", time.Now().UnixNano())
	var projectID string
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO projects (name,code,client_name,pipeline_template_id)
		VALUES ('Dynamic rule test',$1,'Test client','40000000-0000-0000-0000-000000000024')
		RETURNING id
	`, projectCode).Scan(&projectID); err != nil {
		t.Fatal(err)
	}
	var runID string
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO pipeline_runs (project_id,template_id,engine_version,status)
		VALUES ($1,'40000000-0000-0000-0000-000000000024',4,'in_progress')
		RETURNING id
	`, projectID).Scan(&runID); err != nil {
		t.Fatal(err)
	}
	var nodeID string
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO pipeline_run_nodes (run_id,project_id,name,business_code,status,exit_conditions)
		VALUES ($1,$2,'Intake','test_dynamic_intake','in_progress','{"auto_advance_when_ready":true}'::jsonb)
		RETURNING id
	`, runID, projectID).Scan(&nodeID); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO workflow_form_instances (
			project_id,pipeline_run_id,run_node_id,form_template_id,form_version_id,form_type,title,status,
			answers_snapshot_json,submitted_at
		) VALUES (
			$1,$2,$3,'45700000-0000-0000-0000-000000000001','45700000-0000-0000-0000-000000000002',
			'generic_submission','Dynamic intake','submitted','{"entry_route":"client_boq"}'::jsonb,now()
		)
	`, projectID, runID, nodeID); err != nil {
		t.Fatal(err)
	}

	repo := NewRepository(db)
	fromNode := Record{"project_id": projectID, "id": nodeID, "order_index": 1}
	clientBOQEdge := Record{"condition_type": "workflow_form_answer_equals", "condition_config": Record{"field_key": "entry_route", "value": "client_boq"}}
	matched, _, err := repo.runEdgeConditionMet(ctx, tx, clientBOQEdge, fromNode)
	if err != nil || !matched {
		t.Fatalf("submitted form answer must select client BOQ edge: matched=%v err=%v", matched, err)
	}
	surveyEdge := Record{"condition_type": "workflow_form_answer_equals", "condition_config": Record{"field_key": "entry_route", "value": "survey_required"}}
	matched, _, err = repo.runEdgeConditionMet(ctx, tx, surveyEdge, fromNode)
	if err != nil || matched {
		t.Fatalf("non-matching connector must remain false: matched=%v err=%v", matched, err)
	}
}
