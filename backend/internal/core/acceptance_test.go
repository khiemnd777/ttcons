package core

import "testing"

func TestNormalizeAcceptanceEvidenceIDsKeepsPrimaryAndDeduplicates(t *testing.T) {
	ids := normalizeAcceptanceEvidenceIDs([]string{
		"20000000-0000-0000-0000-000000000002",
		"20000000-0000-0000-0000-000000000001",
		"20000000-0000-0000-0000-000000000002",
	}, "20000000-0000-0000-0000-000000000001")

	if len(ids) != 2 {
		t.Fatalf("expected 2 unique evidence ids, got %d: %#v", len(ids), ids)
	}
	if ids[0] != "20000000-0000-0000-0000-000000000001" {
		t.Fatalf("expected the compatibility evidence_id first, got %q", ids[0])
	}
	if ids[1] != "20000000-0000-0000-0000-000000000002" {
		t.Fatalf("expected the second evidence id to be retained, got %q", ids[1])
	}
}

func TestValidateAcceptanceEvidenceLimit(t *testing.T) {
	ids := make([]string, 11)
	for index := range ids {
		ids[index] = "evidence"
	}

	if err := validateAcceptanceEvidenceLimit(ids[:10], 0); err != nil {
		t.Fatalf("expected 10 evidence records to be accepted: %v", err)
	}
	if err := validateAcceptanceEvidenceLimit(ids, 0); err == nil {
		t.Fatal("expected more than 10 evidence records to be rejected")
	}
}
