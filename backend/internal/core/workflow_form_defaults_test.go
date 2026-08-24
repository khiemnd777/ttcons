package core

import (
	"testing"
	"time"
)

func TestWorkflowFormSuggestedAnswersUsesRev7Metadata(t *testing.T) {
	now := time.Date(2026, time.August, 17, 23, 59, 0, 0, time.FixedZone("ICT", 7*60*60))
	instance := Record{"work_item_title": "Họp phân công thi công"}
	fields := []Record{
		{"field_key": "meeting_date", "validation_json": Record{"default_source": "current_date"}},
		{"field_key": "reported_by", "validation_json": Record{"default_source": "actor_name"}},
		{"field_key": "meeting_title", "validation_json": Record{"default_source": "step_title"}},
		{"field_key": "notes", "validation_json": Record{"capture_mode": "optional"}},
	}

	got := workflowFormSuggestedAnswers(instance, fields, "Nguyễn Minh PM", now)
	if got["meeting_date"] != "2026-08-17" {
		t.Fatalf("expected local business date, got %#v", got["meeting_date"])
	}
	if got["reported_by"] != "Nguyễn Minh PM" {
		t.Fatalf("expected actor name, got %#v", got["reported_by"])
	}
	if got["meeting_title"] != "Họp phân công thi công" {
		t.Fatalf("expected work item title, got %#v", got["meeting_title"])
	}
	if _, exists := got["notes"]; exists {
		t.Fatalf("optional field without default must not be suggested")
	}
}

func TestApplyWorkflowFormSuggestedAnswersPreservesEnteredValues(t *testing.T) {
	now := time.Date(2026, time.August, 17, 9, 0, 0, 0, time.UTC)
	fields := []Record{
		{"field_key": "check_date", "validation_json": Record{"default_source": "current_date"}},
		{"field_key": "company_representative", "validation_json": Record{"default_source": "actor_name"}},
	}

	got := applyWorkflowFormSuggestedAnswers(
		Record{},
		fields,
		Record{"check_date": "2026-08-20"},
		"Admin hệ thống",
		now,
	)
	if got["check_date"] != "2026-08-20" {
		t.Fatalf("suggestion overwrote entered value: %#v", got["check_date"])
	}
	if got["company_representative"] != "Admin hệ thống" {
		t.Fatalf("expected missing actor field to be defaulted, got %#v", got["company_representative"])
	}
}

func TestApplyWorkflowFormAnswerDefaultsForcesConstructionHandoffEvidence(t *testing.T) {
	answers := Record{
		clientConstructionInspectionItemsFieldKey: []any{
			map[string]any{"title": "Kiểm tra trần", clientConstructionEvidenceRequiredFieldKey: false},
			map[string]any{"title": "Kiểm tra sơn"},
			map[string]any{"title": "Kiểm tra sàn", clientConstructionEvidenceRequiredFieldKey: true},
		},
	}

	normalized := applyWorkflowFormAnswerDefaults(Record{"form_code": clientConstructionHandoffPlanFormCode}, answers)
	items, err := workflowRecordList(normalized[clientConstructionInspectionItemsFieldKey])
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 {
		t.Fatalf("handoff item count = %d, want 3", len(items))
	}
	for index, item := range items {
		if !boolValue(item[clientConstructionEvidenceRequiredFieldKey]) {
			t.Fatalf("handoff item %d evidence_required = %#v, want true", index, item[clientConstructionEvidenceRequiredFieldKey])
		}
	}
	originalItems, err := workflowRecordList(answers[clientConstructionInspectionItemsFieldKey])
	if err != nil {
		t.Fatal(err)
	}
	if boolValue(originalItems[0][clientConstructionEvidenceRequiredFieldKey]) {
		t.Fatal("published answer input was mutated in place")
	}

	unrelated := applyWorkflowFormAnswerDefaults(Record{"form_code": "another_form"}, answers)
	unrelatedItems, err := workflowRecordList(unrelated[clientConstructionInspectionItemsFieldKey])
	if err != nil {
		t.Fatal(err)
	}
	if boolValue(unrelatedItems[0][clientConstructionEvidenceRequiredFieldKey]) {
		t.Fatal("unrelated form evidence policy was changed")
	}
}
