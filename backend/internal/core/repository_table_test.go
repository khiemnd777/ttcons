package core

import (
	"fmt"
	"testing"
)

func TestSafeTableAllowsContractsForProjectRecordPermission(t *testing.T) {
	if got := safeTable("contracts"); got != "contracts" {
		t.Fatalf("safeTable(contracts) = %q, want contracts", got)
	}
	if got := safeTable("contracts; DROP TABLE projects"); got != "" {
		t.Fatalf("safeTable accepted unsafe contract table name %q", got)
	}
}

func TestSafeTableAllowsSupplierQuotationProjectScope(t *testing.T) {
	if got := safeTable("supplier_quotations"); got != "supplier_quotations" {
		t.Fatalf("quotation detail project scope rejects its persisted table: %q", got)
	}
	if got := safeTable("supplier_quotations; DROP TABLE projects"); got != "" {
		t.Fatalf("accepted unsafe quotation table name: %q", got)
	}
}

func TestSafeTableAllowsSimplifiedConstructionRecords(t *testing.T) {
	for _, table := range []string{"construction_checks", "construction_action_items", "accounting_handoffs"} {
		if got := safeTable(table); got != table {
			t.Fatalf("safeTable(%s) = %q, want %s", table, got, table)
		}
	}
}

func TestConstructionActionEditableProtectsTechnicalBackfills(t *testing.T) {
	tests := []struct {
		name   string
		action Record
		want   bool
	}{
		{name: "manual canonical action", action: Record{"id": "action-1", "technical_issue_id": "issue-1", "technical_rework_id": "rework-1"}, want: true},
		{name: "legacy technical issue backfill", action: Record{"id": "issue-1", "technical_issue_id": "issue-1"}, want: false},
		{name: "technical NCR backfill", action: Record{"id": "action-2", "technical_ncr_id": "ncr-1"}, want: false},
		{name: "standalone manual action", action: Record{"id": "action-3"}, want: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := constructionActionEditable(test.action); got != test.want {
				t.Fatalf("constructionActionEditable(%v) = %v, want %v", test.action, got, test.want)
			}
		})
	}
}

func TestNormalizeConstructionCheckCriteriaSupportsDirectEntry(t *testing.T) {
	criteria, err := normalizeConstructionCheckCriteria([]ConstructionCheckCriterion{
		{Title: "  Cao độ và độ phẳng  ", Criteria: "", EvidenceRequired: true},
		{Title: "", Criteria: " Không nứt hoặc hở mối nối "},
		{},
	})
	if err != nil {
		t.Fatalf("normalizeConstructionCheckCriteria returned error: %v", err)
	}
	if len(criteria) != 2 {
		t.Fatalf("criteria count = %d, want 2", len(criteria))
	}
	if criteria[0].Title != "Cao độ và độ phẳng" || criteria[0].Criteria != "Cao độ và độ phẳng" {
		t.Fatalf("first criterion = %#v, want normalized direct text", criteria[0])
	}
	if criteria[1].Title != "Không nứt hoặc hở mối nối" || criteria[1].Criteria != "Không nứt hoặc hở mối nối" {
		t.Fatalf("second criterion = %#v, want criteria promoted to title", criteria[1])
	}
	tooMany := make([]ConstructionCheckCriterion, 31)
	for index := range tooMany {
		tooMany[index].Title = "Tiêu chí"
	}
	if _, err := normalizeConstructionCheckCriteria(tooMany); err == nil {
		t.Fatal("accepted more than 30 criteria")
	}
}

func TestNormalizeConstructionCheckEvidenceIDsKeepsOrderedUniqueItems(t *testing.T) {
	first := "30000000-0000-0000-0000-000000000001"
	second := "30000000-0000-0000-0000-000000000002"
	ids, err := normalizeConstructionCheckEvidenceIDs([]string{second, first, second}, first, true)
	if err != nil {
		t.Fatalf("normalizeConstructionCheckEvidenceIDs returned error: %v", err)
	}
	if len(ids) != 2 || ids[0] != first || ids[1] != second {
		t.Fatalf("evidence IDs = %#v, want [%s %s]", ids, first, second)
	}
	if _, err := normalizeConstructionCheckEvidenceIDs(nil, "", true); err == nil {
		t.Fatal("required result evidence accepted an empty list")
	}
	tooMany := make([]string, 11)
	for index := range tooMany {
		tooMany[index] = fmt.Sprintf("30000000-0000-0000-0000-0000000000%02d", index)
	}
	if _, err := normalizeConstructionCheckEvidenceIDs(tooMany, "", false); err == nil {
		t.Fatal("accepted more than 10 evidence records")
	}
}
