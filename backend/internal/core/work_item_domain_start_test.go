package core

import "testing"

func TestValidateWorkItemMutationContextRequiresExplicitStartIntent(t *testing.T) {
	tests := []struct {
		name    string
		input   WorkItemMutationContext
		wantID  string
		wantErr bool
	}{
		{name: "empty context keeps ordinary module mutation unchanged"},
		{name: "explicit start context", input: WorkItemMutationContext{WorkItemID: " work-1 ", Intent: workItemMutationIntentStartOnSuccess}, wantID: "work-1"},
		{name: "missing work item", input: WorkItemMutationContext{Intent: workItemMutationIntentStartOnSuccess}, wantErr: true},
		{name: "unknown intent", input: WorkItemMutationContext{WorkItemID: "work-1", Intent: "start"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := validateWorkItemMutationContext(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateWorkItemMutationContext() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got.WorkItemID != tt.wantID {
				t.Fatalf("work item id = %q, want %q", got.WorkItemID, tt.wantID)
			}
		})
	}
}

func TestWorkItemDomainMutationPolicyAllowsExactActionBeforeWorkspaceFallback(t *testing.T) {
	policy := workItemDomainMutationPolicy{
		Mutation:   "supplier_quotation_qualified",
		ActionKeys: []string{"source.supplier_quotation_ready"},
		Workspaces: []string{executionWorkspaceBOQ},
	}

	if !workItemDomainMutationPolicyAllows(Record{
		"domain_action_key": "source.supplier_quotation_ready",
		"surface_kind":      "boq",
	}, policy) {
		t.Fatal("exact supplier quotation action should be allowed")
	}
	if workItemDomainMutationPolicyAllows(Record{
		"domain_action_key": "source.boq_approved",
		"surface_kind":      "boq",
	}, policy) {
		t.Fatal("a different non-empty action key must not pass through the shared workspace")
	}
	if !workItemDomainMutationPolicyAllows(Record{
		"domain_action_key":      "",
		"run_node_business_code": "boq_without_design",
	}, policy) {
		t.Fatal("legacy work without an action key may use the explicitly allowed workspace fallback")
	}
}

func TestLinkedWorkItemMutationContextIsExplicit(t *testing.T) {
	if got := linkedWorkItemMutationContext(""); got != (WorkItemMutationContext{}) {
		t.Fatalf("empty linked id should not opt into a transition: %#v", got)
	}
	got := linkedWorkItemMutationContext("work-2")
	if got.WorkItemID != "work-2" || got.Intent != workItemMutationIntentStartOnSuccess {
		t.Fatalf("linked context = %#v", got)
	}
}
