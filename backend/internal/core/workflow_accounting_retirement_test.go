package core

import (
	"context"
	"strings"
	"testing"
)

func TestRetiredAccountingAutomationFailsClosed(t *testing.T) {
	repo := NewRepository(nil)
	ctx := context.Background()
	for _, key := range []string{
		"finance.scaffold_incoming_acceptance", "finance.scaffold_outgoing_acceptance",
		"source.payable_ready", "source.payable_approved", "source.input_invoice_verified",
		"source.receivable_ready", "source.receivable_collected", "source.project_finance_close_ready",
		"payable.verify_input_invoice", "receivable.issue_invoice", "source.unrecognized", "unrecognized.action",
	} {
		t.Run(key, func(t *testing.T) {
			contextRow := Record{"engine_version": 4, "domain_action_key": key, "project_id": newID()}
			if got := clientWorkflowAutomationAction(contextRow); got != key {
				t.Fatalf("unsupported action disappeared before the required gate: %q", got)
			}
			outcome, err := repo.executeClientWorkflowAutomation(ctx, contextRow, key, "", "")
			if err != nil || outcome.ActionKey != key || outcome.Status != "blocked" || outcome.BlockerReason == "" {
				t.Fatalf("unsupported action must fail closed without reading retired tables: %#v, %v", outcome, err)
			}
		})
	}
}

func TestRetiredFinancialEdgeFailsClosedAndCannotBeAuthored(t *testing.T) {
	for _, definition := range PipelineEdgeConditionCatalog() {
		if definition.Type == "financial_ready" {
			t.Fatal("retired debt condition remains in authoring catalog")
		}
	}
	matched, reason, err := evaluateRunEdgeCondition(NewRepository(nil), context.Background(), nil, Record{"condition_type": "financial_ready"}, Record{})
	if err != nil || matched || !strings.Contains(reason, "financial_ready") {
		t.Fatalf("retired debt edge must fail closed without querying old tables: %v, %q, %v", matched, reason, err)
	}
}

func TestConstructionStartIgnoresReceiptsAndKeepsMaterialBlocker(t *testing.T) {
	db := openIsolatedAccountingTestDB(t)
	ctx := context.Background()
	projectID, materialID := newID(), newID()
	mustAccountingExec(t, ctx, db, `INSERT INTO projects (id,name,code,client_name,contract_amount) VALUES ($1,'Project construction readiness',$2,'Client',900000000)`, projectID, "READY-"+projectID[:8])
	mustAccountingExec(t, ctx, db, `INSERT INTO materials (id,code,name,unit,category) VALUES ($1,$2,'Required construction material','m','Construction')`, materialID, "MAT-"+materialID[:8])
	mustAccountingExec(t, ctx, db, `INSERT INTO site_material_balances (project_id,material_id,planned_quantity,issued_quantity) VALUES ($1,$2,10,9)`, projectID, materialID)
	repo := NewRepository(db)
	node := Record{"type": "construction", "project_id": projectID}
	blockers := repo.nodeBlockers(ctx, node)
	if len(blockers) != 1 || blockers[0] != "Vật tư bắt buộc chưa được xuất đủ ra công trình" {
		t.Fatalf("only missing material should block construction: %#v", blockers)
	}
	mustAccountingExec(t, ctx, db, `UPDATE site_material_balances SET issued_quantity=10 WHERE project_id=$1`, projectID)
	if blockers := repo.nodeBlockers(ctx, node); len(blockers) != 0 {
		t.Fatalf("construction with sufficient material must not require collected money: %#v", blockers)
	}
}
