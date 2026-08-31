package core

import "testing"

func TestReconciliationCompanyScopeSeparatesReadAndAccountingActions(t *testing.T) {
	profile := workItemAccessProfile{CanReconcileAllProjects: true}
	granted := []string{
		"project.view",
		"procurement.approve",
		"accounting_handoff.acknowledge",
	}

	if !reconciliationCompanyScopeAllowed(profile, granted, []string{"project.view"}) {
		t.Fatal("company reconciliation observer must receive project read scope")
	}
	if !reconciliationCompanyScopeAllowed(profile, granted, []string{"accounting_handoff.acknowledge"}) {
		t.Fatal("company reconciliation observer must receive the granted Accounting action scope")
	}
	if reconciliationCompanyScopeAllowed(profile, granted, []string{"procurement.approve"}) {
		t.Fatal("company reconciliation observer must not receive procurement approval scope")
	}
	if reconciliationCompanyScopeAllowed(profile, granted, []string{"contract.review"}) {
		t.Fatal("company reconciliation observer must not receive an action that was not granted")
	}
	if reconciliationCompanyScopeAllowed(workItemAccessProfile{}, granted, []string{"project.view"}) {
		t.Fatal("ordinary actors must not receive the company reconciliation bypass")
	}
}

func TestAccountingObserverDoesNotGainWorkItemDecisionOverride(t *testing.T) {
	observer := workItemAccessProfile{CanReconcileAllProjects: true}
	if !profileCanObserveAllProjectWork(observer) {
		t.Fatal("Accounting observer must be able to inspect project work")
	}
	if observer.IsAdmin {
		t.Fatal("Accounting observer must not be treated as an admin decision actor")
	}
}
