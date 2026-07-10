package core

import "testing"

func TestValidateTemplateGraphAcceptsReachableDAG(t *testing.T) {
	nodes := []Record{{"id": "a"}, {"id": "b"}, {"id": "c"}}
	edges := []Record{
		{"id": "ab", "from_node_id": "a", "to_node_id": "b"},
		{"id": "bc", "from_node_id": "b", "to_node_id": "c"},
	}
	if issues := validateTemplateGraph(nodes, edges); len(issues) != 0 {
		t.Fatalf("expected valid graph, got %#v", issues)
	}
}

func TestValidateTemplateGraphRejectsCycleAndMissingSingleRoot(t *testing.T) {
	nodes := []Record{{"id": "a"}, {"id": "b"}}
	edges := []Record{
		{"id": "ab", "from_node_id": "a", "to_node_id": "b"},
		{"id": "ba", "from_node_id": "b", "to_node_id": "a"},
	}
	issues := validateTemplateGraph(nodes, edges)
	assertLifecycleIssueCode(t, issues, "GRAPH_ROOT_COUNT")
	assertLifecycleIssueCode(t, issues, "GRAPH_SINK_COUNT")
	assertLifecycleIssueCode(t, issues, "GRAPH_CYCLE")
}

func TestPrepareTemplateEdgeBodyRequiresAnswerConditionConfig(t *testing.T) {
	body := Record{
		"from_node_id":     "a",
		"to_node_id":       "b",
		"condition_type":   "workflow_form_answer_equals",
		"condition_config": Record{},
	}
	if err := prepareTemplateEdgeBody(body, false); err == nil {
		t.Fatal("expected missing field_key/value to be rejected")
	}
	body["condition_config"] = Record{"field_key": "execution_route", "value": "subcontractor"}
	if err := prepareTemplateEdgeBody(body, false); err != nil {
		t.Fatalf("expected complete answer condition config, got %v", err)
	}
}

func TestTemplateDefinitionHashIgnoresLifecycleTimestamps(t *testing.T) {
	first := Record{
		"id": "template-1", "name": "Workflow", "status": "draft", "updated_at": "2026-01-01",
		"nodes": []Record{{"id": "node-1", "name": "Start", "updated_at": "2026-01-01"}},
	}
	second := Record{
		"id": "template-1", "name": "Workflow", "status": "published", "updated_at": "2026-02-01", "published_at": "2026-02-01",
		"nodes": []Record{{"id": "node-1", "name": "Start", "updated_at": "2026-02-01"}},
	}
	firstHash, err := templateDefinitionHash(first)
	if err != nil {
		t.Fatal(err)
	}
	secondHash, err := templateDefinitionHash(second)
	if err != nil {
		t.Fatal(err)
	}
	if firstHash != secondHash {
		t.Fatalf("expected lifecycle-only changes to preserve definition hash: %s != %s", firstHash, secondHash)
	}
}

func TestEngineV2AutomationUsesStableDomainActionKey(t *testing.T) {
	contextRow := Record{
		"engine_version":    2,
		"run_node_name":     "Tên node có thể thay đổi",
		"domain_action_key": "procurement.generate_from_boq",
	}
	if action := clientWorkflowAutomationAction(contextRow); action != "procurement.generate_from_boq" {
		t.Fatalf("expected stable domain action key, got %q", action)
	}
	contextRow["domain_action_key"] = ""
	contextRow["run_node_name"] = "BOQ (Có thiết kế)"
	if action := clientWorkflowAutomationAction(contextRow); action != "" {
		t.Fatalf("engine v2 must not fall back to display-name dispatch, got %q", action)
	}
}

func TestNormalizeOrderedApprovalPolicyBuildsRoleStages(t *testing.T) {
	normalized, err := normalizeApprovalPolicyValue(Record{
		"mode":                     "ordered",
		"roles":                    []any{"QS", "PM", "Giám đốc"},
		"requester_cannot_approve": true,
		"comment_required":         true,
	})
	if err != nil {
		t.Fatal(err)
	}
	policy := approvalPolicyFromValue(normalized)
	if len(policy.Stages) != 3 {
		t.Fatalf("expected one stage per ordered role, got %#v", policy.Stages)
	}
	if got := policy.rolesForStage(1); len(got) != 1 || got[0] != "PM" {
		t.Fatalf("expected PM at stage 2, got %#v", got)
	}
}

func assertLifecycleIssueCode(t *testing.T, issues []Record, expected string) {
	t.Helper()
	for _, issue := range issues {
		if issue["code"] == expected {
			return
		}
	}
	t.Fatalf("expected issue %s in %#v", expected, issues)
}
