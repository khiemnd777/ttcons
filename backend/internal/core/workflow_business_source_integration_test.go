package core

import (
	"context"
	"testing"
)

func TestRev7BusinessSourceIsFailClosedAndUsesCanonicalBOQ(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	actorID := newID()
	staffID := newID()
	projectID := newID()
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM projects WHERE id=$1`, projectID)
		_, _ = db.ExecContext(ctx, `DELETE FROM staff WHERE id=$1`, staffID)
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id=$1`, actorID)
	})
	mustExec(t, db, `INSERT INTO users (id,email,password_hash,full_name,status) VALUES ($1,$2,'demo:pw','Rev7 Actor','active')`, actorID, "rev7-source-"+actorID+"@test.local")
	mustExec(t, db, `INSERT INTO staff (id,user_id,full_name,email,department,status) VALUES ($1,$2,'Rev7 Actor',$3,'operations','active')`, staffID, actorID, "rev7-source-staff-"+actorID+"@test.local")
	mustExec(t, db, `INSERT INTO projects (id,name,code,client_name,status,pipeline_template_id) VALUES ($1,'Rev7 Source Project',$2,'Client','planning','40000000-0000-0000-0000-000000000026')`, projectID, "R7S-"+projectID[:8])
	mustExec(t, db, `INSERT INTO project_members (project_id,staff_id,role_ids,active) VALUES ($1,$2,'["QS"]'::jsonb,true)`, projectID, staffID)
	if _, err := repo.StartPipeline(ctx, projectID, "40000000-0000-0000-0000-000000000026", actorID, "Rev7 Actor"); err != nil {
		t.Fatal(err)
	}
	workItemID := clientOfficialWorkItemIDByAction(t, db, projectID, "source.boq_items_ready")
	source, err := repo.WorkflowBusinessSourceForWorkItem(ctx, workItemID)
	if err != nil {
		t.Fatal(err)
	}
	if boolValue(source["ready"]) || source["status"] != "blocked" {
		t.Fatalf("empty BOQ source=%#v, want fail-closed", source)
	}

	mustExec(t, db, `
		INSERT INTO boq_items (project_id,category,work_item_name,unit,quantity,unit_price,status,created_by)
		VALUES ($1,'A','Canonical BOQ row','m',1,100,'draft',$2)
	`, projectID, actorID)
	source, err = repo.WorkflowBusinessSourceForWorkItem(ctx, workItemID)
	if err != nil {
		t.Fatal(err)
	}
	if !boolValue(source["ready"]) || source["status"] != "succeeded" {
		t.Fatalf("BOQ-backed source=%#v, want ready", source)
	}
	if source["route"] != "/projects/"+projectID+"/boq" {
		t.Fatalf("source route=%#v", source["route"])
	}
}
