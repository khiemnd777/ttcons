package core

import "testing"

func TestDeterministicStartPathFollowsAlwaysEdges(t *testing.T) {
	nodes := []Record{
		{"id": "node-1", "order_index": 1, "status": "in_progress"},
		{"id": "node-2", "order_index": 2, "status": "pending"},
		{"id": "node-3", "order_index": 3, "status": "pending"},
	}
	edges := []Record{
		{"from_run_node_id": "node-1", "to_run_node_id": "node-2", "condition_type": "always"},
		{"from_run_node_id": "node-2", "to_run_node_id": "node-3", "condition_type": "always"},
	}

	path, resolved, reason := deterministicStartPath(nodes, edges, "node-1", "node-3")
	if !resolved || reason != "" {
		t.Fatalf("resolved = %v, reason = %q", resolved, reason)
	}
	if len(path) != 2 || path[0] != "node-1" || path[1] != "node-2" {
		t.Fatalf("path = %#v, want predecessor nodes", path)
	}
}

func TestDeterministicStartPathRejectsUnresolvedBranch(t *testing.T) {
	nodes := []Record{
		{"id": "node-1", "order_index": 1, "status": "in_progress"},
		{"id": "node-2a", "order_index": 2, "status": "pending"},
		{"id": "node-2b", "order_index": 2, "status": "pending"},
	}
	edges := []Record{
		{"from_run_node_id": "node-1", "to_run_node_id": "node-2a", "condition_type": "workflow_form_answer_equals"},
		{"from_run_node_id": "node-1", "to_run_node_id": "node-2b", "condition_type": "workflow_form_answer_equals"},
	}

	_, resolved, reason := deterministicStartPath(nodes, edges, "node-1", "node-2a")
	if resolved || reason == "" {
		t.Fatalf("resolved = %v, reason = %q, want unresolved branch", resolved, reason)
	}
}

func TestDeterministicStartPathUsesLegacyOrderFallback(t *testing.T) {
	nodes := []Record{
		{"id": "node-1", "order_index": 1, "status": "in_progress"},
		{"id": "node-skipped", "order_index": 2, "status": "skipped"},
		{"id": "node-3", "order_index": 3, "status": "pending"},
	}

	path, resolved, _ := deterministicStartPath(nodes, nil, "node-1", "node-3")
	if !resolved || len(path) != 1 || path[0] != "node-1" {
		t.Fatalf("path = %#v, resolved = %v", path, resolved)
	}
}

func TestFutureStartActionUsesStartReadinessInsteadOfCurrentActionCapability(t *testing.T) {
	item := Record{
		"status":          "pending",
		"can_act":         false,
		"can_decide":      false,
		"is_future":       true,
		"start_readiness": Record{"can_start": true},
	}
	actions := myWorkItemAvailableActions(item)
	if !containsMyWorkString(actions, "start") || containsMyWorkString(actions, "block") {
		t.Fatalf("future actions = %#v, want audited start only", actions)
	}
}
