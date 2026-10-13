package core

import (
	"context"
	"encoding/json"
	"testing"
)

func TestNativeDailyNextPlanExactIdentity(t *testing.T) {
	a, b := newID(), newID()
	task := func(id, state string, index int, row Record) Record {
		return Record{"id": id, "status": state, "metadata_json": Record{"next_plan_item_index": index, "next_plan_item": row}}
	}
	rows := []Record{{"row_id": b, "title": "Same title", "progress_percent": 0}, {"row_id": a, "title": "Same title", "state": ""}}
	got := nativeDailyNextPlanStates(rows, []Record{task(newID(), "completed", 0, Record{"row_id": a, "title": "Same title"}), task(newID(), "in_progress", 1, Record{"row_id": b, "title": "Same title"})})
	if got[0]["state"] != "Đang làm" || got[0]["progress_percent"] != 0 || got[1]["state"] != "" {
		t.Fatal(got)
	}
	if rows[0]["state"] != nil || got[0]["issued_date"] != nil {
		t.Fatal("source mutated or issue date fabricated")
	}
	duplicate := nativeDailyNextPlanStates([]Record{{"row_id": b}}, []Record{task(newID(), "completed", 0, Record{"row_id": b}), task(newID(), "ready", 0, Record{"row_id": b})})
	if duplicate[0]["state"] != nil {
		t.Fatal("ambiguous generated relation reused")
	}
	legacy := Record{"title": "Legacy", "priority": "normal"}
	if got := nativeDailyNextPlanStates([]Record{legacy}, []Record{task(newID(), "completed", 0, legacy)}); got[0]["state"] != "Hoàn thành" {
		t.Fatal(got)
	}
	if got := nativeDailyNextPlanStates([]Record{Record{"title": "Changed"}, legacy}, []Record{task(newID(), "completed", 0, legacy)}); got[1]["state"] != nil {
		t.Fatal("legacy index followed row reorder")
	}
}

func TestNativeDailySourceReuseExactReceivingScope(t *testing.T) {
	db := openIsolatedCoreTestDB(t, "native_daily_reuse_", []string{"incoming_acceptance_records", "incoming_acceptance_items", "work_items", "work_item_activities"})
	ctx := context.Background()
	f := setupWorkItemFlowFixture(t, db, false)
	other := setupWorkItemFlowFixture(t, db, false)
	otherWork, receivingWork := newID(), newID()
	mustExec(t, db, `INSERT INTO work_items(id,project_id,source_type,source_id,title,work_type,status,role_ids) VALUES($1,$3,'issue',$1,'Hidden receipt','task','ready','["Restricted"]'),($2,$3,'issue',$2,'Receiving task','task','ready','["Đội thi công"]')`, otherWork, receivingWork, f.projectID)
	insert := func(project, work, date, status string, delivered, accepted float64) string {
		id, item := newID(), newID()
		mustExec(t, db, `INSERT INTO incoming_acceptance_records(id,acceptance_code,project_id,work_item_id,status,received_at,supplier_name) VALUES($1::uuid,'reuse-'||$1::text,$2,$3,$4,$5,'Supplier is not origin')`, id, project, work, status, date)
		mustExec(t, db, `INSERT INTO incoming_acceptance_items(id,incoming_acceptance_id,project_id,item_index,material_name,unit,delivered_quantity,accepted_quantity,comment) VALUES($1,$2,$3,0,'Sand','m3',$4,$5,'Receiving observation')`, item, id, project, delivered, accepted)
		return item
	}
	id := insert(f.projectID, receivingWork, "2026-10-09", "submitted", 10, 3)
	hiddenItem := insert(f.projectID, otherWork, "2026-10-09", "submitted", 20, 5)
	mustExec(t, db, `INSERT INTO incoming_acceptance_items(incoming_acceptance_id,project_id,item_index,material_name,unit,delivered_quantity) SELECT incoming_acceptance_id,project_id,position,'Hidden row','m3',1 FROM incoming_acceptance_items CROSS JOIN generate_series(1,250) position WHERE id=$1`, hiddenItem)
	hiddenEffective := insert(f.projectID, receivingWork, "2026-10-09", "submitted", 22, 5)
	mustExec(t, db, `UPDATE incoming_acceptance_items SET work_item_id=$2 WHERE id=$1`, hiddenEffective, otherWork)
	insert(other.projectID, other.itemID, "2026-10-09", "submitted", 30, 5)
	insert(f.projectID, f.itemID, "2026-10-08", "submitted", 40, 5)
	insert(f.projectID, f.itemID, "2026-10-09", "cancelled", 50, 5)
	insert(f.projectID, f.itemID, "2026-10-09", "draft", 60, 5)
	source := Record{"project_id": f.projectID, "work_item_id": f.itemID, "report_date": "2026-10-09", "structured_json": Record{"material_usage_items": []Record{{"material_name": "Unrelated usage", "quantity": 99}}}}
	if err := enrichNativeDailySourceForActor(ctx, db, source, false, f.workerID); err != nil {
		t.Fatal(err)
	}
	arrivals := workflowAnswerRecordList(recordFromAny(source["structured_json"])["material_arrival_items"])
	if len(arrivals) != 1 || cleanID(arrivals[0]["source_item_id"]) != id || cleanID(arrivals[0]["source_work_item_id"]) != receivingWork || number(arrivals[0]["quantity"]) != 10 || arrivals[0]["origin"] != nil || arrivals[0]["note"] != "Receiving observation" {
		t.Fatal(arrivals)
	}
	for _, explicit := range []any{[]Record{{"material_name": "Manual", "quantity": 0, "origin": ""}}} {
		manual := bmgsCopy(source)
		manual["structured_json"] = Record{"material_arrival_items": explicit}
		before, _ := json.Marshal(manual)
		if err := enrichNativeDailySourceForActor(ctx, db, manual, false, f.workerID); err != nil {
			t.Fatal(err)
		}
		after, _ := json.Marshal(manual)
		if string(before) != string(after) {
			t.Fatal("explicit manual rows/clear overridden", string(after))
		}
	}
	for _, empty := range []any{[]Record{}, nil} {
		defaultEmpty := bmgsCopy(source)
		defaultEmpty["structured_json"] = Record{"material_arrival_items": empty}
		if err := enrichNativeDailySourceForActor(ctx, db, defaultEmpty, false, f.workerID); err != nil {
			t.Fatal(err)
		}
		if len(workflowAnswerRecordList(recordFromAny(defaultEmpty["structured_json"])["material_arrival_items"])) != 1 {
			t.Fatal("default empty list blocked exact receiving reuse")
		}
	}
	denied := bmgsCopy(source)
	denied["structured_json"] = Record{}
	if err := enrichNativeDailySourceForActor(ctx, db, denied, false, other.workerID); err != nil {
		t.Fatal(err)
	}
	if recordFromAny(denied["structured_json"])["material_arrival_items"] != nil {
		t.Fatal("cross-project actor received source facts")
	}
	mustExec(t, db, `DELETE FROM role_permissions USING user_roles,permissions WHERE role_permissions.role_id=user_roles.role_id AND role_permissions.permission_id=permissions.id AND user_roles.user_id=$1 AND permissions.key='operation.view'`, f.workerID)
	if err := enrichNativeDailySourceForActor(ctx, db, denied, false, f.workerID); err != nil {
		t.Fatal(err)
	}
	if recordFromAny(denied["structured_json"])["material_arrival_items"] != nil {
		t.Fatal("receipt reused without source view permission")
	}
	mustExec(t, db, `INSERT INTO role_permissions(role_id,permission_id) SELECT assignment.role_id,permission.id FROM user_roles assignment CROSS JOIN permissions permission WHERE assignment.user_id=$1 AND permission.key='operation.view' ON CONFLICT DO NOTHING`, f.workerID)
	narrowless := Record{"project_id": f.projectID, "report_date": "2026-10-09", "structured_json": Record{}}
	if err := enrichNativeDailySourceForActor(ctx, db, narrowless, false, f.workerID); err != nil {
		t.Fatal(err)
	}
	if recordFromAny(narrowless["structured_json"])["material_arrival_items"] != nil {
		t.Fatal("project/date-only receiving reuse")
	}
	overflowID := newID()
	mustExec(t, db, `INSERT INTO incoming_acceptance_records(id,acceptance_code,project_id,work_item_id,status,received_at) VALUES($1::uuid,'overflow-'||$1::text,$2,$3,'submitted','2026-10-09')`, overflowID, f.projectID, f.itemID)
	mustExec(t, db, `INSERT INTO incoming_acceptance_items(incoming_acceptance_id,project_id,item_index,material_name,unit,delivered_quantity) SELECT $1,$2,position,'Bounded row','m3',1 FROM generate_series(0,199) AS position`, overflowID, f.projectID)
	bounded := bmgsCopy(source)
	bounded["structured_json"] = Record{}
	if err := enrichNativeDailySourceForActor(ctx, db, bounded, false, f.workerID); err == nil {
		t.Fatal("source overflow silently truncated")
	}
	mustExec(t, db, `DELETE FROM incoming_acceptance_items WHERE incoming_acceptance_id=$1 AND item_index=199`, overflowID)
	if err := enrichNativeDailySourceForActor(ctx, db, bounded, false, f.workerID); err != nil {
		t.Fatal("200 visible rows rejected because hidden rows were counted", err)
	}
	if rows := workflowAnswerRecordList(recordFromAny(bounded["structured_json"])["material_arrival_items"]); len(rows) != 200 {
		t.Fatalf("visible boundary returned %d rows", len(rows))
	}
}

func TestNativeDailySourceReuseGeneratedStateAndDeliveryFreeze(t *testing.T) {
	db := openIsolatedCoreTestDB(t, "native_daily_state_", []string{"work_items", "work_item_activities", "operation_daily_reports", "work_report_revisions"})
	ctx := context.Background()
	f := setupWorkItemFlowFixture(t, db, false)
	instanceID, a, b := newID(), newID(), newID()
	answers := Record{"report_date": "2026-10-09", "next_plan_items": []Record{{"row_id": a, "title": "Same title", "due_date": "2026-10-10", "owner_role": "Đội thi công", "owner_name": "", "description": ""}, {"row_id": b, "title": "Same title", "due_date": "2026-10-10", "owner_role": "Đội thi công", "owner_name": "", "description": "", "progress_percent": 0}}}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	ids, err := createDailyReportNextItemsTx(ctx, tx, Record{"id": instanceID, "project_id": f.projectID, "work_item_id": f.itemID, "form_code": "builtin_construction_daily_report"}, answers, f.workerID)
	if err != nil {
		tx.Rollback()
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 {
		t.Fatal(ids)
	}
	mustExec(t, db, `UPDATE work_items SET status='completed' WHERE id=$1`, ids[0])
	mustExec(t, db, `UPDATE work_items SET status='in_progress' WHERE id=$1`, ids[1])
	// Current order differs from the creation index: stable row identity wins.
	source := Record{"id": newID(), "project_id": f.projectID, "work_item_id": f.itemID, "reported_by": f.workerID, "workflow_form_instance_id": instanceID, "report_date": "2026-10-09", "structured_json": Record{"next_plan_items": []Record{{"row_id": b, "title": "Same title", "progress_percent": 0}, {"row_id": a, "title": "Same title"}}}}
	if err := enrichNativeDailySourceForActor(ctx, db, source, false, f.workerID); err != nil {
		t.Fatal(err)
	}
	got := workflowAnswerRecordList(recordFromAny(source["structured_json"])["next_plan_items"])
	if got[0]["state"] != "Đang làm" || got[1]["state"] != "Hoàn thành" || got[0]["progress_percent"] != 0 || got[0]["issued_date"] != nil {
		t.Fatal(got)
	}
	// Freeze executes the actual transaction helper, including the source queries.
	source["structured_json"] = answers
	tx, err = db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := freezeNativeWorkReportEvidence(ctx, tx, source, true); err != nil {
		tx.Rollback()
		t.Fatal(err)
	}
	snapshot := workReportSnapshot(source)
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	comparison := bmgsCopy(source)
	comparison["structured_json"] = answers
	comparison["status"] = "needs_changes"
	comparison["delivery_snapshot_json"] = snapshot
	if err := enrichNativeDailySourceForActor(ctx, db, comparison, false, f.workerID); err != nil {
		t.Fatal(err)
	}
	if err := annotateWorkReportDeliverySourceChanged(comparison); err != nil {
		t.Fatal(err)
	}
	if boolValue(comparison["delivery_source_changed"]) {
		t.Fatal("unchanged derived facts marked source changed")
	}
	raw, _ := json.Marshal(snapshot)
	reportID := newID()
	mustExec(t, db, `INSERT INTO operation_daily_reports(id,project_id,work_item_id,report_date,reported_by,work_summary) VALUES($1,$2,$3,'2026-10-09',$4,'Frozen task state')`, cleanID(source["id"]), f.projectID, f.itemID, f.workerID)
	mustExec(t, db, `INSERT INTO work_reports(id,source_kind,source_id,project_id,report_type,report_date,author_id,recipient_id,current_revision_no,status) VALUES($1,'operation_daily_report',$2,$3,'construction_daily','2026-10-09',$4,$4,1,'submitted')`, reportID, cleanID(source["id"]), f.projectID, f.workerID)
	mustExec(t, db, `INSERT INTO work_report_revisions(report_id,revision_no,snapshot_json,snapshot_sha256,submitted_by) VALUES($1,1,$2::jsonb,$3,$4)`, reportID, string(raw), sha256Hex(raw), f.workerID)
	mustExec(t, db, `UPDATE work_items SET status='cancelled' WHERE id=$1`, ids[1])
	comparison = bmgsCopy(source)
	comparison["structured_json"] = answers
	comparison["status"] = "needs_changes"
	comparison["delivery_snapshot_json"] = snapshot
	if err := enrichNativeDailySourceForActor(ctx, db, comparison, false, f.workerID); err != nil {
		t.Fatal(err)
	}
	if err := annotateWorkReportDeliverySourceChanged(comparison); err != nil {
		t.Fatal(err)
	}
	if !boolValue(comparison["delivery_source_changed"]) {
		t.Fatal("changed source lifecycle lost material-change signal")
	}
	revision, err := NewRepository(db).WorkReportRevision(ctx, reportID, 1)
	if err != nil {
		t.Fatal(err)
	}
	locked := Record{"project_id": f.projectID, "work_item_id": f.itemID, "workflow_form_instance_id": instanceID, "status": "submitted", "delivery_snapshot_json": recordFromAny(revision["snapshot_json"]), "structured_json": answers}
	if err := enrichNativeDailySource(ctx, db, locked, false); err != nil {
		t.Fatal(err)
	}
	got = workflowAnswerRecordList(recordFromAny(locked["structured_json"])["next_plan_items"])
	if got[1]["state"] != "Đang làm" {
		t.Fatal("delivered snapshot followed live lifecycle", got)
	}
	after, _ := json.Marshal(snapshot)
	if string(raw) != string(after) {
		t.Fatal("historical source mutated")
	}
	old := bmgsCopy(locked)
	old["delivery_snapshot_json"] = Record{"structured_json": answers}
	if err := enrichNativeDailySourceForActor(ctx, db, old, false, f.workerID); err != nil {
		t.Fatal(err)
	}
	if rows := workflowAnswerRecordList(recordFromAny(old["structured_json"])["next_plan_items"]); rows[0]["state"] != nil {
		t.Fatal("legacy snapshot silently supplemented")
	}
	mustExec(t, db, `UPDATE work_items SET role_ids='["Restricted"]'::jsonb WHERE id=$1`, ids[0])
	mustExec(t, db, `UPDATE work_item_assignments SET role_name='Restricted' WHERE work_item_id=$1`, ids[0])
	hidden := bmgsCopy(source)
	hidden["structured_json"] = answers
	if err := enrichNativeDailySourceForActor(ctx, db, hidden, false, f.workerID); err != nil {
		t.Fatal(err)
	}
	if rows := workflowAnswerRecordList(recordFromAny(hidden["structured_json"])["next_plan_items"]); rows[0]["state"] != nil {
		t.Fatal("generated task source view scope bypassed", rows)
	}
	mustExec(t, db, `UPDATE work_items SET role_ids='["Đội thi công"]'::jsonb WHERE id=$1`, ids[0])
	mustExec(t, db, `UPDATE work_item_assignments SET role_name='Đội thi công' WHERE work_item_id=$1`, ids[0])
	// A wrong parent relation and a cancelled exact task never reuse a status.
	mustExec(t, db, `UPDATE work_item_activities SET metadata_json=jsonb_set(metadata_json,'{parent_work_item_id}',to_jsonb($2::text)) WHERE work_item_id=$1 AND activity_type='created'`, ids[0], newID())
	unlinked := bmgsCopy(source)
	unlinked["structured_json"] = answers
	if err := enrichNativeDailySourceForActor(ctx, db, unlinked, false, f.workerID); err != nil {
		t.Fatal(err)
	}
	if rows := workflowAnswerRecordList(recordFromAny(unlinked["structured_json"])["next_plan_items"]); rows[0]["state"] != nil || rows[1]["state"] != nil {
		t.Fatal(rows)
	}
}
