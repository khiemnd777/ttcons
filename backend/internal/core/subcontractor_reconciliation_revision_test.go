package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"path/filepath"
	"sync"
	"testing"

	"consflow/backend/internal/shared/migrations"
)

func TestClientSubcontractorChecklistQuantityAuthority(t *testing.T) {
	source := Record{"title": "Wall", "claimed_quantity": 23.0, "confirmed_quantity": 20.0, "design_quantity": 25.0, "actual_quantity": 24.8, "status": "done"}
	form := Record{"template": Record{"code": "client_subcontractor_acceptance_package"}, "answers": Record{"checklist_items": []Record{source}, "quantity_items": []Record{{"title": "Independent", "claimed_quantity": 999.0}}}}
	if !isSubcontractorChecklistForm(form) {
		t.Fatal("client source excluded")
	}
	items, err := subcontractorChecklistSourceItems(form)
	if err != nil || len(items) != 1 {
		t.Fatalf("source: %v %v", items, err)
	}
	row := defaultSubcontractorReconciliationItem(0, items[0])
	if number(row["claimed_quantity"]) != 23 || number(row["confirmed_quantity"]) != 20 {
		t.Fatalf("quantity authority changed: %v", row)
	}
	payload := row["payload_json"].(Record)
	if number(payload["design_quantity"]) != 25 || number(payload["actual_quantity"]) != 24.8 {
		t.Fatal("physical assessment lost")
	}
	unknown := Record{"template": Record{"code": "client_subcontractor_other"}}
	if isSubcontractorChecklistForm(unknown) {
		t.Fatal("nonexact form admitted")
	}
	form["instance"] = Record{"status": "submitted", "current_revision": 1, "answers_snapshot_json": Record{"checklist_items": []Record{source}}}
	form["answers"] = Record{"checklist_items": []Record{{"title": "New live R2", "claimed_quantity": 999.0}}}
	pinned := subcontractorReconciliationSourceForm(form)
	items, err = subcontractorChecklistSourceItems(pinned)
	if err != nil || cleanOptionalText(items[0]["title"]) != "Wall" {
		t.Fatal("submitted snapshot mixed with live answer rows")
	}
}

func clientReconciliationFixture(t *testing.T, db *sql.DB, codes ...string) (*Service, workItemFlowFixture, string) {
	t.Helper()
	fixture := setupWorkItemFlowFixture(t, db, false)
	bindWorkItemDomainExecutionContext(t, db, fixture, "acceptance", "subcontractor_acceptance", "subcontractor_reconciliation", "source.subcontractor_acceptance_package_ready")
	formID := newID()
	var templateID, versionID, formType string
	code := "client_subcontractor_acceptance_package"
	if len(codes) > 0 {
		code = codes[0]
	}
	if err := db.QueryRow(`SELECT wft.id::text,wfv.id::text,wft.form_type FROM workflow_form_templates wft JOIN workflow_form_versions wfv ON wfv.template_id=wft.id WHERE wft.code=$1 AND wfv.status='published' ORDER BY wfv.version_no DESC LIMIT 1`, code).Scan(&templateID, &versionID, &formType); err != nil {
		t.Fatal(err)
	}
	answers := Record{"subcontractor_name": "Test subcontractor", "checklist_items": []Record{{"title": "Wall", "status": "done", "claimed_quantity": 23.0, "confirmed_quantity": 20.0, "design_quantity": 25.0, "actual_quantity": 24.8}}}
	raw, _ := json.Marshal(answers)
	mustExec(t, db, `INSERT INTO workflow_form_instances(id,project_id,work_item_id,form_template_id,form_version_id,form_type,title,status,current_revision,answers_snapshot_json,created_by) VALUES($1,$2,$3,$4,$5,$6,'Client test','submitted',1,$7::jsonb,$8)`, formID, fixture.projectID, fixture.itemID, templateID, versionID, formType, string(raw), fixture.workerID)
	for key, value := range answers {
		body, _ := json.Marshal(value)
		mustExec(t, db, `INSERT INTO workflow_form_answers(instance_id,field_key,field_type,value_json,revision_no) VALUES($1,$2,'structured_list',$3::jsonb,1)`, formID, key, string(body))
	}
	return NewService(NewRepository(db), "test-secret"), fixture, formID
}

func requireClientStatus(t *testing.T, err error, status int) {
	t.Helper()
	var statusErr StatusError
	if !errors.As(err, &statusErr) || statusErr.Status != status {
		t.Fatalf("error %v; want status %d", err, status)
	}
}

func TestClientSubcontractorReconciliationRevisionDB(t *testing.T) {
	db := openIsolatedCoreTestDB(t, "bm03_revision_", []string{"subcontractor_checklist_reconciliations", "workflow_form_instances", "work_items"})
	s, fixture, formID := clientReconciliationFixture(t, db)
	ctx := context.Background()
	get := func() Record {
		t.Helper()
		result, err := s.SubcontractorReconciliation(ctx, fixture.itemID, fixture.workerID)
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	initial := get()
	if !boolValue(initial["available"]) {
		t.Fatalf("client unavailable: %v", initial)
	}
	if !boolValue(initial["capabilities"].(Record)["can_write"]) {
		t.Fatal("worker cannot write")
	}
	req := SubcontractorReconciliationRequest{Items: []SubcontractorReconciliationItemRequest{{ItemIndex: 0, Decision: "accepted"}}}
	_, err := s.SaveSubcontractorReconciliation(ctx, fixture.itemID, req, fixture.workerID, "Worker")
	requireClientStatus(t, err, 428)
	token := cleanOptionalText(initial["precondition"])
	if _, err = s.SaveSubcontractorReconciliation(ctx, fixture.itemID, req, fixture.workerID, "Worker", token); err != nil {
		t.Fatal(err)
	}
	_, err = s.SaveSubcontractorReconciliation(ctx, fixture.itemID, req, fixture.workerID, "Worker", token)
	requireClientStatus(t, err, 412)
	if _, err = s.SubmitSubcontractorReconciliation(ctx, fixture.itemID, req, fixture.workerID, "Worker", cleanOptionalText(get()["precondition"])); err != nil {
		t.Fatal(err)
	}
	historical, err := s.SubcontractorReconciliation(ctx, fixture.itemID, fixture.workerID, 1)
	if err != nil {
		t.Fatal(err)
	}
	ledger := historical["reconciliation"].(Record)
	oldID := cleanID(ledger["id"])
	var oldHash, oldItemHash string
	if err = db.QueryRow(`SELECT md5(row_to_json(scr)::text) FROM subcontractor_checklist_reconciliations scr WHERE id=$1`, oldID).Scan(&oldHash); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow(`SELECT md5(jsonb_agg(to_jsonb(scri) ORDER BY item_index)::text) FROM subcontractor_checklist_reconciliation_items scri WHERE reconciliation_id=$1`, oldID).Scan(&oldItemHash); err != nil {
		t.Fatal(err)
	}
	form, err := s.repo.GetWorkflowFormInstanceDetail(ctx, formID)
	if err != nil {
		t.Fatal(err)
	}
	staleForm := subcontractorReconciliationSourceForm(form)
	staleItem, err := s.repo.GetWorkItemDetail(ctx, fixture.itemID)
	if err != nil {
		t.Fatal(err)
	}
	staleRows, staleSummary, err := s.normalizeSubcontractorReconciliationRequest(ctx, staleItem, staleForm, []Record{}, req)
	if err != nil {
		t.Fatal(err)
	}
	staleSourceToken := cleanOptionalText(get()["precondition"])
	answers := form["answers"].(Record)
	answers["checklist_items"] = []Record{{"title": "Amended wall", "status": "done", "claimed_quantity": 30.0, "confirmed_quantity": 28.0}}
	if _, err = s.repo.AmendWorkflowForm(ctx, formID, answers, "New assessment", 1, fixture.workerID, "Worker"); err != nil {
		t.Fatal(err)
	}
	// Deterministic race interleaving: normalization reads R1, the actual
	// amendment commits R2, then stale R1 persistence reaches its source lock.
	err = s.repo.SaveSubcontractorReconciliation(ctx, staleItem, staleForm, "", staleRows, staleSummary, false, fixture.workerID, "Worker", staleSourceToken)
	requireClientStatus(t, err, 409)
	current := get()
	_, err = s.SaveSubcontractorReconciliation(ctx, fixture.itemID, req, fixture.workerID, "Worker", token)
	requireClientStatus(t, err, 412)
	if number(current["reconciliation"].(Record)["form_revision_no"]) != 2 || cleanID(current["reconciliation"].(Record)["id"]) != "" {
		t.Fatal("R1 reused for R2")
	}
	var readyBeforeR2 bool
	if err = db.QueryRow(sampleCompletedGateQuery("source.subcontractor_acceptance_package_ready"), fixture.projectID, fixture.itemID).Scan(&readyBeforeR2); err != nil || readyBeforeR2 {
		t.Fatalf("old R1 unlocked R2: %v %v", readyBeforeR2, err)
	}
	item, err := s.repo.GetWorkItemDetail(ctx, fixture.itemID)
	if err != nil {
		t.Fatal(err)
	}
	outcome, err := s.repo.automateClientOperationalSourceGate(ctx, item, "source.subcontractor_acceptance_package_ready")
	if err != nil || outcome.Status != "blocked" {
		t.Fatalf("runtime old source unlocked R2: %v %v", outcome, err)
	}
	if _, err = s.SubmitSubcontractorReconciliation(ctx, fixture.itemID, req, fixture.workerID, "Worker", cleanOptionalText(current["precondition"])); err != nil {
		t.Fatal(err)
	}
	var count int
	var unchanged bool
	if err = db.QueryRow(`SELECT count(*) FROM subcontractor_checklist_reconciliations WHERE work_item_id=$1`, fixture.itemID).Scan(&count); err != nil || count != 2 {
		t.Fatalf("ledger count %d %v", count, err)
	}
	if err = db.QueryRow(`SELECT md5(row_to_json(scr)::text)=$2 FROM subcontractor_checklist_reconciliations scr WHERE id=$1`, oldID, oldHash).Scan(&unchanged); err != nil || !unchanged {
		t.Fatal("R1 rewritten")
	}
	if err = db.QueryRow(`SELECT md5(jsonb_agg(to_jsonb(scri) ORDER BY item_index)::text)=$2 FROM subcontractor_checklist_reconciliation_items scri WHERE reconciliation_id=$1`, oldID, oldItemHash).Scan(&unchanged); err != nil || !unchanged {
		t.Fatal("R1 item payload rewritten")
	}
	historical, err = s.SubcontractorReconciliation(ctx, fixture.itemID, fixture.workerID, 1)
	if err != nil {
		t.Fatal(err)
	}
	rows := historical["reconciliation"].(Record)["items"].([]Record)
	if len(rows) != 1 || cleanOptionalText(rows[0]["checklist_title"]) != "Wall" || number(rows[0]["claimed_quantity"]) != 23 || historical["form"] != nil {
		t.Fatal("historical source enriched from current")
	}
	_, err = s.SubcontractorReconciliation(ctx, fixture.itemID, fixture.workerID, 99)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("missing history: %v", err)
	}
	outsider := setupProjectScopeFixture(t, db, []string{"operation.view", "operation.report"})
	_, err = s.SubcontractorReconciliation(ctx, fixture.itemID, outsider.userID)
	requireClientStatus(t, err, 403)
	_, err = s.SaveSubcontractorReconciliation(ctx, fixture.itemID, req, outsider.userID, "Other", cleanOptionalText(get()["precondition"]))
	requireClientStatus(t, err, 403)
	var sourceReady bool
	if err = db.QueryRow(sampleCompletedGateQuery("source.subcontractor_acceptance_package_ready"), fixture.projectID, fixture.itemID).Scan(&sourceReady); err != nil || !sourceReady {
		t.Fatalf("current source readiness %v %v", sourceReady, err)
	}
	outcome, err = s.repo.automateClientOperationalSourceGate(ctx, item, "source.subcontractor_acceptance_package_ready")
	if err != nil || outcome.Status != "succeeded" {
		t.Fatalf("runtime current source blocked: %v %v", outcome, err)
	}
	mustExec(t, db, `UPDATE work_items SET status='completed' WHERE id=$1`, fixture.itemID)
	_, err = s.SaveSubcontractorReconciliation(ctx, fixture.itemID, req, fixture.workerID, "Worker", cleanOptionalText(get()["precondition"]))
	if err == nil {
		t.Fatal("completed write permitted")
	}
}

func TestClientSubcontractorBuiltinLegacyAndWrongSourceDB(t *testing.T) {
	db := openIsolatedCoreTestDB(t, "bm03_legacy_", []string{"subcontractor_checklist_reconciliations", "workflow_form_instances", "work_items"})
	s, fixture, formID := clientReconciliationFixture(t, db, "builtin_subcontractor_checklist")
	ctx := context.Background()
	mustExec(t, db, `UPDATE workflow_form_instances SET current_revision=0 WHERE id=$1`, formID)
	if _, err := s.SaveSubcontractorReconciliation(ctx, fixture.itemID, SubcontractorReconciliationRequest{}, fixture.workerID, "Worker"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SubmitSubcontractorReconciliation(ctx, fixture.itemID, SubcontractorReconciliationRequest{}, fixture.workerID, "Worker"); err != nil {
		t.Fatal(err)
	}
	var ready bool
	if err := db.QueryRow(sampleCompletedGateQuery("source.subcontractor_acceptance_package_ready"), fixture.projectID, fixture.itemID).Scan(&ready); err != nil || ready {
		t.Fatalf("builtin unlocked client source: %v %v", ready, err)
	}
	ledger, err := s.repo.GetSubcontractorReconciliation(ctx, fixture.itemID)
	if err != nil || number(ledger["form_revision_no"]) != 0 {
		t.Fatalf("legacy revision0: %v %v", ledger, err)
	}
	if _, err = s.SaveSubcontractorReconciliation(ctx, fixture.itemID, SubcontractorReconciliationRequest{}, fixture.workerID, "Worker"); err == nil {
		t.Fatal("legacy submitted ledger mutable")
	}
	otherItem := newID()
	mustExec(t, db, `INSERT INTO work_items(id,project_id,source_type,source_id,title,work_type,status,role_ids) VALUES($1,$2,'issue',$3,'Other work','task','ready','["Đội thi công"]')`, otherItem, fixture.projectID, newID())
	if err = db.QueryRow(sampleCompletedGateQuery("source.subcontractor_acceptance_package_ready"), fixture.projectID, otherItem).Scan(&ready); err != nil || ready {
		t.Fatalf("unrelated source unlocked client: %v %v", ready, err)
	}
}

func TestClientSubcontractorReconciliationConcurrentCreateDB(t *testing.T) {
	db := openIsolatedCoreTestDB(t, "bm03_concurrent_", []string{"subcontractor_checklist_reconciliations", "workflow_form_instances", "work_items"})
	s, fixture, _ := clientReconciliationFixture(t, db)
	ctx := context.Background()
	initial, err := s.SubcontractorReconciliation(ctx, fixture.itemID, fixture.workerID)
	if err != nil {
		t.Fatal(err)
	}
	token := cleanOptionalText(initial["precondition"])
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := s.SubmitSubcontractorReconciliation(ctx, fixture.itemID, SubcontractorReconciliationRequest{}, fixture.workerID, "Worker", token)
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	success := 0
	for err := range results {
		if err == nil {
			success++
		} else {
			requireClientStatus(t, err, 412)
		}
	}
	if success != 1 {
		t.Fatalf("success count %d", success)
	}
	assertCount(t, db, `SELECT count(*) FROM subcontractor_checklist_reconciliations WHERE work_item_id=$1 AND status='submitted'`, 1, fixture.itemID)
	assertCount(t, db, `SELECT count(*) FROM subcontractor_checklist_reconciliation_items WHERE work_item_id=$1`, 1, fixture.itemID)
	assertCount(t, db, `SELECT count(*) FROM audit_logs WHERE project_id=$1 AND event_type='subcontractor_reconciliation_submitted'`, 1, fixture.projectID)
	assertCount(t, db, `SELECT count(*) FROM work_item_activities WHERE work_item_id=$1 AND metadata_json ? 'subcontractor_reconciliation_id'`, 1, fixture.itemID)
}

func TestClientSubcontractorInactiveSourceDB(t *testing.T) {
	db := openIsolatedCoreTestDB(t, "bm03_inactive_", []string{"subcontractor_checklist_reconciliations", "workflow_form_instances", "work_items"})
	s, fixture, _ := clientReconciliationFixture(t, db)
	ctx := context.Background()
	result, err := s.SubcontractorReconciliation(ctx, fixture.itemID, fixture.workerID)
	if err != nil {
		t.Fatal(err)
	}
	item, err := s.repo.GetWorkItemDetail(ctx, fixture.itemID)
	if err != nil {
		t.Fatal(err)
	}
	form, err := s.repo.GetWorkflowFormForWorkItem(ctx, fixture.itemID)
	if err != nil {
		t.Fatal(err)
	}
	form = subcontractorReconciliationSourceForm(form)
	rows, summary, err := s.normalizeSubcontractorReconciliationRequest(ctx, item, form, []Record{}, SubcontractorReconciliationRequest{})
	if err != nil {
		t.Fatal(err)
	}
	mustExec(t, db, `UPDATE pipeline_run_nodes SET status='completed' WHERE id=$1`, cleanID(item["run_node_id"]))
	err = s.repo.SaveSubcontractorReconciliation(ctx, item, form, "", rows, summary, true, fixture.workerID, "Worker", cleanOptionalText(result["precondition"]))
	requireClientStatus(t, err, 409)
	assertCount(t, db, `SELECT count(*) FROM subcontractor_checklist_reconciliations WHERE work_item_id=$1`, 0, fixture.itemID)
	assertCount(t, db, `SELECT count(*) FROM audit_logs WHERE project_id=$1`, 0, fixture.projectID)
	assertCount(t, db, `SELECT count(*) FROM work_item_activities WHERE work_item_id=$1`, 0, fixture.itemID)
}

func TestClientSubcontractorMigrationPreservesLegacyRevisionZeroDB(t *testing.T) {
	db := openIsolatedCoreTestDB(t, "bm03_migration_", []string{"subcontractor_checklist_reconciliations", "workflow_form_instances", "work_items"})
	s, fixture, _ := clientReconciliationFixture(t, db)
	ctx := context.Background()
	result, err := s.SubcontractorReconciliation(ctx, fixture.itemID, fixture.workerID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.SubmitSubcontractorReconciliation(ctx, fixture.itemID, SubcontractorReconciliationRequest{}, fixture.workerID, "Worker", cleanOptionalText(result["precondition"])); err != nil {
		t.Fatal(err)
	}
	// Reconstruct the V21 constraint state with a retained legacy revision0 row
	// inside this isolated schema, then run V158 through the custom runner.
	mustExec(t, db, `UPDATE subcontractor_checklist_reconciliations SET form_revision_no=0 WHERE work_item_id=$1`, fixture.itemID)
	mustExec(t, db, `ALTER TABLE subcontractor_checklist_reconciliations DROP CONSTRAINT subcontractor_reconciliations_source_revision_unique,DROP CONSTRAINT subcontractor_reconciliations_form_revision_unique,ADD CONSTRAINT subcontractor_reconciliations_work_item_unique UNIQUE(work_item_id),ADD CONSTRAINT subcontractor_reconciliations_form_unique UNIQUE(form_instance_id)`)
	mustExec(t, db, `DELETE FROM schema_migrations WHERE version='158'`)
	var headerHash, itemHash string
	if err = db.QueryRow(`SELECT md5(to_jsonb(scr)::text) FROM subcontractor_checklist_reconciliations scr WHERE work_item_id=$1`, fixture.itemID).Scan(&headerHash); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow(`SELECT md5(jsonb_agg(to_jsonb(scri) ORDER BY item_index)::text) FROM subcontractor_checklist_reconciliation_items scri WHERE work_item_id=$1`, fixture.itemID).Scan(&itemHash); err != nil {
		t.Fatal(err)
	}
	if _, err = migrations.Run(ctx, db, filepath.Join("..", "..", "migrations")); err != nil {
		t.Fatal(err)
	}
	var same bool
	if err = db.QueryRow(`SELECT md5(to_jsonb(scr)::text)=$2 AND form_revision_no=0 FROM subcontractor_checklist_reconciliations scr WHERE work_item_id=$1`, fixture.itemID, headerHash).Scan(&same); err != nil || !same {
		t.Fatal("migration rewrote legacy header")
	}
	if err = db.QueryRow(`SELECT md5(jsonb_agg(to_jsonb(scri) ORDER BY item_index)::text)=$2 FROM subcontractor_checklist_reconciliation_items scri WHERE work_item_id=$1`, fixture.itemID, itemHash).Scan(&same); err != nil || !same {
		t.Fatal("migration rewrote legacy items")
	}
	history, err := s.SubcontractorReconciliation(ctx, fixture.itemID, fixture.workerID, 0)
	if err != nil || !boolValue(history["readonly"]) {
		t.Fatalf("legacy history unreadable %v %v", history, err)
	}
}
