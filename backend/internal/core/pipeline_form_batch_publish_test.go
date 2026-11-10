package core

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"testing"
)

func TestPublishPipelineFormsIsAtomicAndUsesDistinctVersions(t *testing.T) {
	db := openProjectOperationsTestDB(t)
	ctx := context.Background()
	repo := NewRepository(db)
	actorID, templateID, nodeID := newID(), newID(), newID()
	formTemplateIDs := []string{newID(), newID()}
	versionIDs := []string{newID(), newID()}
	sort.Strings(versionIDs)
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM pipeline_templates WHERE id=$1`, templateID)
		for _, id := range formTemplateIDs {
			_, _ = db.ExecContext(ctx, `DELETE FROM workflow_form_templates WHERE id=$1`, id)
		}
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id=$1`, actorID)
	})
	mustExec(t, db, `INSERT INTO users(id,email,password_hash,full_name,status) VALUES($1,$2,'demo:pw','Batch Form Publisher','active')`, actorID, "batch-form-"+actorID+"@test.local")
	mustExec(t, db, `INSERT INTO pipeline_templates(id,template_family_id,name,status) VALUES($1,$1,'Batch Form Pipeline','draft')`, templateID)
	mustExec(t, db, `INSERT INTO pipeline_template_nodes(id,template_id,name) VALUES($1,$2,'Giai đoạn thử nghiệm')`, nodeID, templateID)
	for i := range formTemplateIDs {
		mustExec(t, db, `INSERT INTO workflow_form_templates(id,code,name,form_type,status) VALUES($1,$2,$3,'generic_submission','active')`, formTemplateIDs[i], "batch_form_"+strings.ReplaceAll(formTemplateIDs[i], "-", ""), fmt.Sprintf("Biểu mẫu %d", i+1))
		mustExec(t, db, `INSERT INTO workflow_form_versions(id,template_id,version_no,status) VALUES($1,$2,1,'draft')`, versionIDs[i], formTemplateIDs[i])
		mustExec(t, db, `INSERT INTO pipeline_template_steps(node_id,name,step_type,form_required,form_version_id) VALUES($1,$2,'task',true,$3)`, nodeID, fmt.Sprintf("Gói %d", i+1), versionIDs[i])
	}
	mustExec(t, db, `INSERT INTO pipeline_template_steps(node_id,name,step_type,form_required,form_version_id) VALUES($1,'Gói dùng lại','task',true,$2)`, nodeID, versionIDs[0])
	addBatchFormTestField(t, db, versionIDs[0])
	for _, id := range versionIDs {
		refreshBatchFormTestHash(t, db, id)
	}

	preview, err := repo.PipelineFormPublishPreview(ctx, templateID)
	if err != nil {
		t.Fatal(err)
	}
	if number(preview["publishable_count"]) != 1 || number(preview["step_occurrences"]) != 3 {
		t.Fatalf("preview counts = %#v, want one eligible version and three uses", preview)
	}
	request := batchFormRequest(preview)
	if _, err := repo.PublishPipelineForms(ctx, templateID, request, actorID, "Batch Form Publisher"); err == nil {
		t.Fatal("expected zero-field Form to block entire batch")
	}
	assertCount(t, db, `SELECT COUNT(*) FROM workflow_form_versions WHERE id=ANY($1::uuid[]) AND status='published'`, 0, versionIDs)
	assertCount(t, db, `SELECT COUNT(*) FROM audit_logs WHERE target_type='workflow_form_version' AND target_id=ANY($1::uuid[]) AND event_type='workflow_form_version_published'`, 0, versionIDs)

	addBatchFormTestField(t, db, versionIDs[1])
	refreshBatchFormTestHash(t, db, versionIDs[1])
	preview, err = repo.PipelineFormPublishPreview(ctx, templateID)
	if err != nil {
		t.Fatal(err)
	}
	if number(preview["publishable_count"]) != 2 || len(preview["forms"].([]Record)) != 2 {
		t.Fatalf("preview did not deduplicate referenced Forms: %#v", preview)
	}
	request = batchFormRequest(preview)
	result, err := repo.PublishPipelineForms(ctx, templateID, request, actorID, "Batch Form Publisher")
	if err != nil {
		t.Fatal(err)
	}
	if number(result["published_count"]) != 2 {
		t.Fatalf("published count = %v, want 2", result["published_count"])
	}
	assertCount(t, db, `SELECT COUNT(*) FROM workflow_form_versions WHERE id=ANY($1::uuid[]) AND status='published'`, 2, versionIDs)
	assertCount(t, db, `SELECT COUNT(*) FROM audit_logs WHERE target_type='workflow_form_version' AND target_id=ANY($1::uuid[]) AND event_type='workflow_form_version_published'`, 2, versionIDs)
	if _, err := repo.PublishPipelineForms(ctx, templateID, request, actorID, "Batch Form Publisher"); err == nil {
		t.Fatal("stale preview must not publish again")
	}
}

func batchFormRequest(preview Record) PipelineFormPublishRequest {
	request := PipelineFormPublishRequest{}
	for _, form := range preview["forms"].([]Record) {
		request.Versions = append(request.Versions, PipelineFormPublishExpectation{
			VersionID: fmt.Sprint(form["version_id"]), DefinitionHash: fmt.Sprint(form["definition_hash"]), VersionStatus: fmt.Sprint(form["version_status"]),
		})
	}
	return request
}

func addBatchFormTestField(t *testing.T, db *sql.DB, versionID string) {
	t.Helper()
	mustExec(t, db, `INSERT INTO workflow_form_fields(form_version_id,field_key,label,field_type,required) VALUES($1,'summary','Nội dung','text',true)`, versionID)
}

func refreshBatchFormTestHash(t *testing.T, db *sql.DB, versionID string) {
	t.Helper()
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	hash, err := workflowFormDefinitionHashTx(context.Background(), tx, versionID)
	_ = tx.Rollback()
	if err != nil {
		t.Fatal(err)
	}
	mustExec(t, db, `UPDATE workflow_form_versions SET definition_hash=$2 WHERE id=$1`, versionID, hash)
}
