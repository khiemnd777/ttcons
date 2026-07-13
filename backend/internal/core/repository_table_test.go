package core

import "testing"

func TestSafeTableAllowsContractsForProjectRecordPermission(t *testing.T) {
	if got := safeTable("contracts"); got != "contracts" {
		t.Fatalf("safeTable(contracts) = %q, want contracts", got)
	}
	if got := safeTable("contracts; DROP TABLE projects"); got != "" {
		t.Fatalf("safeTable accepted unsafe contract table name %q", got)
	}
}
