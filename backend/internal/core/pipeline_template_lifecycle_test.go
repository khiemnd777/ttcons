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

func TestTemplateDefinitionHashIgnoresReportPresentationConfig(t *testing.T) {
	first := Record{
		"id": "template-1", "name": "Workflow",
		"report_config_json": Record{"node_reference_title": "Client -> Rev4"},
		"nodes":              []Record{{"id": "node-1", "business_code": "survey"}},
	}
	second := Record{
		"id": "template-1", "name": "Workflow",
		"report_config_json": Record{"node_reference_title": "Đối chiếu node client"},
		"nodes":              []Record{{"id": "node-1", "business_code": "survey"}},
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
		t.Fatalf("expected report-only presentation config to preserve definition hash: %s != %s", firstHash, secondHash)
	}
}

func TestTemplateDefinitionHashIgnoresDiagramPresentationMetadata(t *testing.T) {
	first := Record{
		"id":    "template-1",
		"nodes": []Record{{"id": "node-1", "name": "Start", "position_x": 100, "position_y": 200}},
		"edges": []Record{{
			"id": "edge-1", "from_node_id": "node-1", "to_node_id": "node-2",
			"condition_type": "workflow_form_answer_equals",
			"condition_config": Record{
				"field_key": "execution_route",
				"value":     "subcontractor",
				"diagram": Record{
					"portMode": "manual",
					"source":   Record{"nodeId": "node-1", "side": "right"},
					"target":   Record{"nodeId": "node-2", "side": "left"},
				},
			},
		}},
	}
	second := Record{
		"id":    "template-1",
		"nodes": []Record{{"id": "node-1", "name": "Start", "position_x": 900, "position_y": -400}},
		"edges": []Record{{
			"id": "edge-1", "from_node_id": "node-1", "to_node_id": "node-2",
			"condition_type": "workflow_form_answer_equals",
			"condition_config": Record{
				"field_key": "execution_route",
				"value":     "subcontractor",
				"diagram": Record{
					"portMode": "manual",
					"source":   Record{"nodeId": "node-1", "side": "bottom"},
					"target":   Record{"nodeId": "node-2", "side": "top"},
				},
			},
		}},
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
		t.Fatalf("expected layout-only changes to preserve definition hash: %s != %s", firstHash, secondHash)
	}

	secondEdges := second["edges"].([]Record)
	secondConfig := secondEdges[0]["condition_config"].(Record)
	secondConfig["value"] = "direct"
	changedHash, err := templateDefinitionHash(second)
	if err != nil {
		t.Fatal(err)
	}
	if firstHash == changedHash {
		t.Fatal("expected workflow condition changes to change definition hash")
	}

	secondConfig["value"] = "subcontractor"
	secondConfig["position_x"] = 99
	changedHash, err = templateDefinitionHash(second)
	if err != nil {
		t.Fatal(err)
	}
	if firstHash == changedHash {
		t.Fatal("expected a business condition key named position_x to remain in the definition hash")
	}
}

func TestNormalizePipelineTemplateLayoutRequest(t *testing.T) {
	req := PipelineTemplateLayoutRequest{
		Nodes: []PipelineTemplateNodeLayoutRequest{{
			ID:        "41400000-0000-0000-0000-000000000001",
			PositionX: 120,
			PositionY: -48,
		}},
		Edges: []PipelineTemplateEdgeLayoutRequest{{
			ID:         "41600000-0000-0000-0000-000000000001",
			SourceSide: " Bottom ",
			TargetSide: "TOP",
		}},
	}
	normalized, err := normalizePipelineTemplateLayoutRequest(req)
	if err != nil {
		t.Fatal(err)
	}
	if normalized.Edges[0].SourceSide != "bottom" || normalized.Edges[0].TargetSide != "top" {
		t.Fatalf("expected normalized port sides, got %#v", normalized.Edges[0])
	}
}

func TestNormalizePipelineTemplateLayoutRequestRejectsInvalidPresentationData(t *testing.T) {
	tests := []PipelineTemplateLayoutRequest{
		{},
		{Nodes: []PipelineTemplateNodeLayoutRequest{
			{ID: "41400000-0000-0000-0000-000000000001"},
			{ID: "41400000-0000-0000-0000-000000000001"},
		}},
		{Nodes: []PipelineTemplateNodeLayoutRequest{{
			ID:        "41400000-0000-0000-0000-000000000001",
			PositionX: maxPipelineDiagramCoordinate + 1,
		}}},
		{Edges: []PipelineTemplateEdgeLayoutRequest{{
			ID:         "41600000-0000-0000-0000-000000000001",
			SourceSide: "center",
			TargetSide: "left",
		}}},
	}
	for index, req := range tests {
		if _, err := normalizePipelineTemplateLayoutRequest(req); err == nil {
			t.Fatalf("case %d: expected invalid layout request to be rejected", index)
		}
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

func TestOfficialWorkflowAutomationRecognizesRevisionFamily(t *testing.T) {
	if !isClientOfficialWorkflowContext(Record{
		"template_id":        "40000000-0000-0000-0000-000000000026",
		"template_family_id": clientOfficialWorkflowTemplateID,
	}) {
		t.Fatal("published revision in the official family must keep hard automation gates")
	}
	if isClientOfficialWorkflowContext(Record{
		"template_id":        "40000000-0000-0000-0000-000000000026",
		"template_family_id": "40000000-0000-0000-0000-000000000099",
	}) {
		t.Fatal("unrelated template family must not run official workflow automation")
	}
	contextRow := Record{"engine_version": 4, "domain_action_key": "source.boq_release_ready"}
	if action := clientWorkflowAutomationAction(contextRow); action != "source.boq_release_ready" {
		t.Fatalf("expected Rev7 source gate, got %q", action)
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
