package core

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
)

func isClientSubcontractorChecklistForm(form Record) bool {
	template, _ := form["template"].(Record)
	return cleanOptionalText(template["code"]) == "client_subcontractor_acceptance_package"
}

func subcontractorReconciliationSourceForm(form Record) Record {
	if !isClientSubcontractorChecklistForm(form) {
		return form
	}
	instance, _ := form["instance"].(Record)
	if cleanOptionalText(instance["status"]) != "submitted" {
		return form
	}
	// The instance row contains the submitted revision and its saved answers in
	// one read. Separate live answer reads may race with an amendment.
	form["answers"] = recordFromAny(instance["answers_snapshot_json"])
	return form
}

func clientReconciliationWorkItemWritable(item Record) bool {
	return statusIn(cleanOptionalText(item["status"]), "pending", "ready", "needs_changes", "in_progress", "blocked")
}

// The transport token binds the existing source and ledger, including absence.
// It is not a captured form field or a persisted business revision.
func subcontractorReconciliationPrecondition(instance, ledger Record) string {
	if cleanID(ledger["id"]) == "" {
		ledger = nil
	}
	identity := []any{instance["id"], instance["project_id"], instance["work_item_id"], instance["current_revision"], instance["status"], instance["updated_at"], ledger["id"], ledger["status"], ledger["updated_at"]}
	raw, _ := json.Marshal(identity)
	return fmt.Sprintf("\"%x\"", sha256.Sum256(raw))
}

func (s *Service) clientSubcontractorResponseMetadata(ctx context.Context, item, form, ledger, result Record) (Record, error) {
	instance, _ := form["instance"].(Record)
	history, err := s.repo.Query(ctx, `SELECT id,form_instance_id,form_revision_no,status,submitted_at,updated_at
		FROM subcontractor_checklist_reconciliations WHERE project_id=$1 AND work_item_id=$2
		ORDER BY form_revision_no DESC,created_at DESC,id`, cleanID(item["project_id"]), cleanID(item["id"]))
	if err != nil {
		return nil, err
	}
	if history == nil {
		history = []Record{}
	}
	result["history"] = history
	result["precondition"] = subcontractorReconciliationPrecondition(instance, ledger)
	result["capabilities"] = Record{"can_write": false}
	result["readonly"] = true
	return result, nil
}

func (s *Service) subcontractorReconciliationHistoryResponse(ctx context.Context, item, form Record, revision int) (Record, error) {
	if !isClientSubcontractorChecklistForm(form) || revision < 0 {
		return nil, validationError("Phiên bản đối soát không hợp lệ")
	}
	instance, _ := form["instance"].(Record)
	ledger, err := s.repo.getSubcontractorReconciliationForSource(ctx, cleanID(item["project_id"]), cleanID(item["id"]), cleanID(instance["id"]), revision)
	if err != nil {
		return nil, err
	}
	result, err := s.clientSubcontractorResponseMetadata(ctx, item, form, ledger, Record{"available": true, "work_item_id": item["id"], "project_id": item["project_id"], "form": nil, "reconciliation": ledger, "boq_items": []Record{}})
	delete(result, "precondition")
	return result, err
}

func (r *Repository) getSubcontractorReconciliationForSource(ctx context.Context, projectID, workItemID, formID string, revision int) (Record, error) {
	ledger, err := r.QueryOne(ctx, `SELECT scr.*,submitted.full_name AS submitted_by_name,updated.full_name AS updated_by_name
		FROM subcontractor_checklist_reconciliations scr
		LEFT JOIN users submitted ON submitted.id=scr.submitted_by LEFT JOIN users updated ON updated.id=scr.updated_by
		WHERE scr.project_id=$1 AND scr.work_item_id=$2 AND scr.form_instance_id=$3 AND scr.form_revision_no=$4`, projectID, workItemID, formID, revision)
	if err != nil {
		return nil, err
	}
	// Historical payloads and quantities belong to this ledger. Do not replace
	// them with the current form answers or current BOQ values.
	items, err := r.Query(ctx, `SELECT scri.*,er.name AS evidence_name,er.status AS evidence_status
		FROM subcontractor_checklist_reconciliation_items scri LEFT JOIN evidence_records er ON er.id=scri.evidence_id
		WHERE scri.reconciliation_id=$1 ORDER BY scri.item_index,scri.id`, ledger["id"])
	if err != nil {
		return nil, err
	}
	ledger["items"] = items
	return ledger, nil
}

func lockClientSubcontractorSource(ctx context.Context, tx *sql.Tx, item, instance Record) (Record, error) {
	rows, err := queryTx(ctx, tx, `SELECT * FROM workflow_form_instances WHERE id=$1 AND project_id=$2 AND work_item_id=$3 FOR UPDATE`, cleanID(instance["id"]), cleanID(item["project_id"]), cleanID(item["id"]))
	if err != nil {
		return nil, err
	}
	if len(rows) != 1 {
		return nil, sql.ErrNoRows
	}
	current := rows[0]
	if int(number(current["current_revision"])) != int(number(instance["current_revision"])) || cleanOptionalText(current["status"]) != "submitted" {
		return nil, conflictError("Phiên bản biểu mẫu đã thay đổi, hãy tải lại đối soát")
	}
	workRows, err := queryTx(ctx, tx, `SELECT * FROM work_items WHERE id=$1 AND project_id=$2 FOR UPDATE`, cleanID(item["id"]), cleanID(item["project_id"]))
	if err != nil {
		return nil, err
	}
	if len(workRows) != 1 || !clientReconciliationWorkItemWritable(workRows[0]) {
		return nil, conflictError("Đầu việc đã đóng, không thể sửa đối soát")
	}
	work := workRows[0]
	var sourceID string
	if err := tx.QueryRowContext(ctx, `SELECT id::text FROM workflow_form_instances WHERE work_item_id=$1 AND project_id=$2 AND status<>'cancelled' ORDER BY created_at DESC LIMIT 1`, cleanID(item["id"]), cleanID(item["project_id"])).Scan(&sourceID); err != nil {
		return nil, err
	}
	if sourceID != cleanID(current["id"]) {
		return nil, conflictError("Biểu mẫu của đầu việc đã thay đổi")
	}
	if cleanID(work["pipeline_run_id"]) != "" {
		runs, err := queryTx(ctx, tx, `SELECT pr.current_node_id,pr.status AS run_status,rn.status AS node_status FROM pipeline_runs pr JOIN pipeline_run_nodes rn ON rn.id=pr.current_node_id WHERE pr.id=$1 FOR UPDATE OF pr,rn`, cleanID(work["pipeline_run_id"]))
		if err != nil {
			return nil, err
		}
		if len(runs) != 1 || cleanID(runs[0]["current_node_id"]) != cleanID(work["run_node_id"]) || statusIn(cleanOptionalText(runs[0]["run_status"]), "completed", "cancelled") || !statusIn(cleanOptionalText(runs[0]["node_status"]), "in_progress", "waiting_approval", "blocked") {
			return nil, conflictError("Đầu việc không còn thuộc giai đoạn đang thực hiện")
		}
	}
	return current, nil
}
