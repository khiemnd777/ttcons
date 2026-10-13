package core

import (
	"context"
	"database/sql"
	"testing"
)

func contractSignatureSourceFixture(t *testing.T, db *sql.DB, stepCode string, overrideCodes ...string) (*Repository, workItemFlowFixture, string, string, string) {
	t.Helper()
	fixture := setupWorkItemFlowFixture(t, db, false)
	bindWorkItemDomainExecutionContext(t, db, fixture, "contract", "outbound_contract", "contract", "contract.require_execution_ready")
	var formTemplate, formVersion, formType, formCode string
	err := db.QueryRow(`SELECT wft.id::text,wfv.id::text,wft.form_type,wft.code FROM pipeline_template_steps step JOIN pipeline_template_nodes node ON node.id=step.node_id JOIN workflow_form_versions wfv ON wfv.id=step.form_version_id JOIN workflow_form_templates wft ON wft.id=wfv.template_id WHERE step.business_code=$1 ORDER BY (node.template_id='40000000-0000-0000-0000-000000000029') DESC,wfv.version_no DESC LIMIT 1`, stepCode).Scan(&formTemplate, &formVersion, &formType, &formCode)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("pinned %s source=%s", stepCode, formCode)
	if len(overrideCodes) > 0 {
		if err = db.QueryRow(`SELECT template.id::text,version.id::text,template.form_type FROM workflow_form_templates template JOIN workflow_form_versions version ON version.template_id=template.id WHERE template.code=$1 AND version.status='published' ORDER BY version.version_no DESC LIMIT 1`, overrideCodes[0]).Scan(&formTemplate, &formVersion, &formType); err != nil {
			t.Fatal(err)
		}
	}
	mustExec(t, db, `UPDATE pipeline_run_steps SET business_code=$2,form_version_id=$3,form_required=true WHERE id=(SELECT run_step_id FROM work_items WHERE id=$1)`, fixture.itemID, stepCode, formVersion)
	formID, signatureID, contractID, versionID := newID(), newID(), newID(), newID()
	mustExec(t, db, `INSERT INTO workflow_form_instances(id,project_id,work_item_id,pipeline_run_id,run_node_id,run_step_id,form_template_id,form_version_id,form_type,title,status,current_revision,created_by) SELECT $1,project_id,id,pipeline_run_id,run_node_id,run_step_id,$2,$3,$4,'Contract sign source','submitted',1,$5 FROM work_items WHERE id=$6`, formID, formTemplate, formVersion, formType, fixture.workerID, fixture.itemID)
	mustExec(t, db, `INSERT INTO workflow_form_signatures(id,project_id,work_item_id,form_instance_id,form_version_id,form_revision_no,purpose,signer_user_id,signer_name,signer_roles_json,comment,payload_json,payload_hash) VALUES($1,$2,$3,$4,$5,1,'approved_by',$6,'Contract test','[]','Test signature','{}',repeat('a',64))`, signatureID, fixture.projectID, fixture.itemID, formID, formVersion, fixture.workerID)
	mustExec(t, db, `INSERT INTO contracts(id,project_id,contract_code,title,contract_type,direction,counterparty_name,original_amount,current_amount,status,created_by) VALUES($1,$2,$3,'Contract source test','main_client','receivable','Client',1000,1000,'approved',$4)`, contractID, fixture.projectID, "SIGN-"+contractID[:8], fixture.workerID)
	mustExec(t, db, `INSERT INTO contract_versions(id,contract_id,version_no,version_label,amount,scope_summary,status,created_by,definition_hash) VALUES($1,$2,1,'V1',1000,'Test scope','approved',$3,repeat('a',64))`, versionID, contractID, fixture.workerID)
	mustExec(t, db, `UPDATE contracts SET current_version_id=$2 WHERE id=$1`, contractID, versionID)
	return NewRepository(db), fixture, contractID, versionID, signatureID
}

func TestContractSignatureRev10SourceDB(t *testing.T) {
	db := openIsolatedCoreTestDB(t, "contract_source_", []string{"contracts", "contract_versions", "workflow_form_instances"})
	repo, fixture, contractID, versionID, signatureID := contractSignatureSourceFixture(t, db, "outbound_contract.execution_package")
	detail, err := repo.GetContractDetail(context.Background(), contractID)
	if err != nil {
		t.Fatal(err)
	}
	forms := detail["signing_forms"].([]Record)
	if len(forms) != 1 {
		t.Errorf("canonical signing_forms count=%d want1", len(forms))
	}
	if _, err = repo.SignContractVersionWithWorkflowSignature(context.Background(), versionID, signatureID, "Sign current source", WorkItemMutationContext{}, fixture.workerID, "Worker"); err != nil {
		t.Fatalf("canonical sign rejected: %v", err)
	}
	assertCount(t, db, `SELECT count(*) FROM contract_version_signature_links WHERE contract_version_id=$1`, 1, versionID)
}

func TestContractSignatureSourceGuardDB(t *testing.T) {
	db := openIsolatedCoreTestDB(t, "contract_guards_", []string{"contracts", "contract_versions", "workflow_form_instances"})
	for _, step := range []string{"outbound_contract.app_sign", "outbound_contract.execution_package"} {
		t.Run(step, func(t *testing.T) {
			repo, fixture, contractID, versionID, signatureID := contractSignatureSourceFixture(t, db, step)
			ctx := context.Background()
			detail, err := repo.GetContractDetail(ctx, contractID)
			if err != nil {
				t.Fatal(err)
			}
			if len(detail["signing_forms"].([]Record)) != 1 {
				t.Fatal("valid source not offered")
			}
			var signatureHash string
			if err = db.QueryRow(`SELECT md5(to_jsonb(signature)::text) FROM workflow_form_signatures signature WHERE id=$1`, signatureID).Scan(&signatureHash); err != nil {
				t.Fatal(err)
			}
			if _, err = repo.SignContractVersionWithWorkflowSignature(ctx, versionID, signatureID, "Sign", WorkItemMutationContext{}, fixture.pmID, "Other"); err == nil {
				t.Fatal("different signer actor accepted")
			}
			if _, err = repo.SignContractVersionWithWorkflowSignature(ctx, versionID, signatureID, "Sign", WorkItemMutationContext{}, fixture.workerID, "Worker"); err != nil {
				t.Fatal(err)
			}
			if _, err = repo.SignContractVersionWithWorkflowSignature(ctx, versionID, signatureID, "Retry", WorkItemMutationContext{}, fixture.workerID, "Worker"); err == nil {
				t.Fatal("duplicate retry accepted")
			}
			assertCount(t, db, `SELECT count(*) FROM contract_version_signature_links WHERE contract_version_id=$1`, 1, versionID)
			assertCount(t, db, `SELECT count(*) FROM audit_logs WHERE project_id=$1 AND event_type='contract_version_signed_on_app'`, 1, fixture.projectID)
			var same bool
			if err = db.QueryRow(`SELECT md5(to_jsonb(signature)::text)=$2 FROM workflow_form_signatures signature WHERE id=$1`, signatureID, signatureHash).Scan(&same); err != nil || !same {
				t.Fatal("signature history changed")
			}
		})
	}
	for _, failure := range []string{"stale_revision", "wrong_project", "wrong_step", "wrong_pin", "unsubmitted_form", "wrong_code", "wrong_purpose"} {
		t.Run(failure, func(t *testing.T) {
			var overrides []string
			if failure == "wrong_code" {
				overrides = []string{"builtin_subcontractor_checklist"}
			}
			repo, fixture, contractID, versionID, signatureID := contractSignatureSourceFixture(t, db, "outbound_contract.execution_package", overrides...)
			ctx := context.Background()
			switch failure {
			case "stale_revision":
				mustExec(t, db, `UPDATE workflow_form_instances SET current_revision=2 WHERE work_item_id=$1`, fixture.itemID)
			case "unsubmitted_form":
				mustExec(t, db, `UPDATE workflow_form_instances SET status='reopened' WHERE work_item_id=$1`, fixture.itemID)
			case "wrong_step":
				mustExec(t, db, `UPDATE pipeline_run_steps SET business_code='outbound_contract.paper_dispatch' WHERE id=(SELECT run_step_id FROM work_items WHERE id=$1)`, fixture.itemID)
			case "wrong_pin":
				mustExec(t, db, `UPDATE pipeline_run_steps SET form_version_id=NULL WHERE id=(SELECT run_step_id FROM work_items WHERE id=$1)`, fixture.itemID)
			case "wrong_project":
				other := setupProjectScopeFixture(t, db, []string{"contract.sign"})
				mustExec(t, db, `UPDATE contracts SET project_id=$2 WHERE id=$1`, contractID, other.projectID)
			case "wrong_purpose":
				oldID := signatureID
				signatureID = newID()
				mustExec(t, db, `INSERT INTO workflow_form_signatures(id,project_id,work_item_id,form_instance_id,form_version_id,form_revision_no,purpose,signer_user_id,signer_name,signer_roles_json,comment,payload_json,payload_hash) SELECT $1,project_id,work_item_id,form_instance_id,form_version_id,form_revision_no,'internal_ack',signer_user_id,signer_name,signer_roles_json,comment,payload_json,payload_hash FROM workflow_form_signatures WHERE id=$2`, signatureID, oldID)
			}
			if _, err := repo.SignContractVersionWithWorkflowSignature(ctx, versionID, signatureID, "Sign", WorkItemMutationContext{}, fixture.workerID, "Worker"); err == nil {
				t.Fatalf("%s accepted", failure)
			}
			assertCount(t, db, `SELECT count(*) FROM contract_version_signature_links WHERE contract_version_id=$1`, 0, versionID)
			assertCount(t, db, `SELECT count(*) FROM contract_versions WHERE id=$1 AND status='approved'`, 1, versionID)
		})
	}
}
