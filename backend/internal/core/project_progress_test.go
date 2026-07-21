package core

import "testing"

func TestProjectProgressStageKey(t *testing.T) {
	tests := []struct {
		name          string
		projectStatus string
		runID         string
		runStatus     string
		nodeType      string
		want          string
	}{
		{name: "not started", projectStatus: "planning", want: "not_started"},
		{name: "completed project", projectStatus: "completed", want: "completed"},
		{name: "completed run", projectStatus: "planning", runID: "run-1", runStatus: "completed", want: "completed"},
		{name: "survey", runID: "run-1", runStatus: "in_progress", nodeType: "survey", want: "survey"},
		{name: "design", runID: "run-1", runStatus: "in_progress", nodeType: "design", want: "design"},
		{name: "commercial boq", runID: "run-1", runStatus: "in_progress", nodeType: "boq", want: "commercial"},
		{name: "commercial contract", runID: "run-1", runStatus: "in_progress", nodeType: "contract", want: "commercial"},
		{name: "planning", runID: "run-1", runStatus: "in_progress", nodeType: "construction_planning", want: "planning"},
		{name: "construction", runID: "run-1", runStatus: "in_progress", nodeType: "construction", want: "construction"},
		{name: "acceptance and rework", runID: "run-1", runStatus: "in_progress", nodeType: "rework", want: "acceptance"},
		{name: "handover finance", runID: "run-1", runStatus: "in_progress", nodeType: "financial_closure", want: "handover_finance"},
		{name: "custom fallback", runID: "run-1", runStatus: "in_progress", nodeType: "custom", want: "other"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := projectProgressStageKey(tt.projectStatus, tt.runID, tt.runStatus, tt.nodeType); got != tt.want {
				t.Fatalf("projectProgressStageKey() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestProjectProgressBoardStreamEvent(t *testing.T) {
	visible := map[string]struct{}{"project-1": {}}

	eventName, output, ok := projectProgressBoardStreamEvent(ActivityStreamEvent{
		Type:      "work_item_changed",
		ProjectID: "project-1",
		At:        "2026-07-21T00:00:00Z",
	}, visible)
	if !ok || eventName != "project_progress_changed" || output.ProjectID != "project-1" {
		t.Fatalf("visible work item event was not normalized: ok=%v name=%q output=%+v", ok, eventName, output)
	}

	if _, _, ok := projectProgressBoardStreamEvent(ActivityStreamEvent{
		Type:      "project_progress_changed",
		ProjectID: "project-2",
	}, visible); ok {
		t.Fatal("event for an invisible project must be filtered")
	}

	if _, _, ok := projectProgressBoardStreamEvent(ActivityStreamEvent{
		Type: "activity_changed",
	}, visible); ok {
		t.Fatal("unrelated activity event must be filtered")
	}

	if _, _, ok := projectProgressBoardStreamEvent(ActivityStreamEvent{
		Type: "project_progress_changed",
	}, visible); !ok {
		t.Fatal("project-agnostic invalidation must stay visible for newly created projects")
	}
}
