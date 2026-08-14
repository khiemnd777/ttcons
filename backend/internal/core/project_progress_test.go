package core

import (
	"strings"
	"testing"
	"time"
)

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

func TestNormalizeProjectProgressStageRequest(t *testing.T) {
	pmStaffID := "30000000-0000-0000-0000-000000000002"
	normalized, cursor, err := normalizeProjectProgressStageRequest(ProjectProgressStageRequest{
		ProjectProgressFilter: ProjectProgressFilter{
			Search:    "  Nhà phố  ",
			PMStaffID: pmStaffID,
			Status:    " planning ",
		},
		StageKey: "construction",
	})
	if err != nil {
		t.Fatal(err)
	}
	if normalized.Search != "Nhà phố" || normalized.PMStaffID != pmStaffID || normalized.Status != "planning" || normalized.Limit != 25 || cursor != nil {
		t.Fatalf("normalized request=%+v cursor=%+v", normalized, cursor)
	}

	for _, request := range []ProjectProgressStageRequest{
		{StageKey: "unknown"},
		{StageKey: "survey", Limit: 51},
		{ProjectProgressFilter: ProjectProgressFilter{PMStaffID: "not-a-uuid"}, StageKey: "survey"},
		{ProjectProgressFilter: ProjectProgressFilter{Search: strings.Repeat("x", 121)}, StageKey: "survey"},
	} {
		if _, _, err := normalizeProjectProgressStageRequest(request); err == nil {
			t.Fatalf("expected validation error for %+v", request)
		}
	}
}

func TestProjectProgressCursorBindsStageAndFilter(t *testing.T) {
	filter := ProjectProgressFilter{Search: "Thủ Đức", Status: "planning"}
	cursor := projectProgressCursor{
		Version:    1,
		StageKey:   "not_started",
		CreatedAt:  time.Date(2026, time.August, 14, 3, 0, 0, 0, time.UTC).Format(time.RFC3339Nano),
		ID:         "30000000-0000-0000-0000-000000000002",
		FilterHash: filter.hash(),
	}
	encoded := encodeProjectProgressCursor(cursor)
	parsed, err := parseProjectProgressCursor(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if parsed != cursor {
		t.Fatalf("parsed cursor=%+v want=%+v", parsed, cursor)
	}
	if _, _, err := normalizeProjectProgressStageRequest(ProjectProgressStageRequest{
		ProjectProgressFilter: filter,
		StageKey:              "not_started",
		Cursor:                encoded,
	}); err != nil {
		t.Fatalf("matching cursor rejected: %v", err)
	}
	if _, _, err := normalizeProjectProgressStageRequest(ProjectProgressStageRequest{
		ProjectProgressFilter: ProjectProgressFilter{Search: "Quận 1", Status: "planning"},
		StageKey:              "not_started",
		Cursor:                encoded,
	}); err == nil {
		t.Fatal("cursor reused with a different filter must be rejected")
	}
	if _, _, err := normalizeProjectProgressStageRequest(ProjectProgressStageRequest{
		ProjectProgressFilter: filter,
		StageKey:              "survey",
		Cursor:                encoded,
	}); err == nil {
		t.Fatal("cursor reused with a different stage must be rejected")
	}
}
