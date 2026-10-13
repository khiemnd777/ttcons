package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
)

func (r *Repository) GetSubcontractorReconciliation(ctx context.Context, workItemID string) (Record, error) {
	reconciliation, err := r.QueryOne(ctx, `
		SELECT scr.*,
			submitted.full_name AS submitted_by_name,COALESCE(submitted.deleted_at IS NOT NULL,false) AS submitted_by_account_deleted,
			updated.full_name AS updated_by_name,COALESCE(updated.deleted_at IS NOT NULL,false) AS updated_by_account_deleted
		FROM subcontractor_checklist_reconciliations scr
		LEFT JOIN users submitted ON submitted.id=scr.submitted_by
		LEFT JOIN users updated ON updated.id=scr.updated_by
		WHERE scr.work_item_id=$1
		ORDER BY scr.created_at DESC,scr.id DESC LIMIT 1
	`, workItemID)
	if err != nil {
		return nil, err
	}
	items, err := r.Query(ctx, `
		SELECT
			scri.*,
			b.status AS boq_status,
			b.category AS boq_category,
			b.work_item_name AS current_boq_work_item_name,
			b.quantity AS current_boq_quantity,
			b.unit AS current_boq_unit,
			er.name AS evidence_name,
			er.status AS evidence_status
		FROM subcontractor_checklist_reconciliation_items scri
		LEFT JOIN boq_items b ON b.id=scri.boq_item_id
		LEFT JOIN evidence_records er ON er.id=scri.evidence_id
		WHERE scri.reconciliation_id=$1
		ORDER BY scri.item_index, scri.created_at
	`, reconciliation["id"])
	if err != nil {
		return nil, err
	}
	reconciliation["items"] = items
	return reconciliation, nil
}

func (r *Repository) SaveSubcontractorReconciliation(ctx context.Context, item Record, form Record, comment string, items []Record, summary Record, submit bool, actorID, actorName string, precondition ...string) error {
	workItemID := cleanID(item["id"])
	projectID := cleanID(item["project_id"])
	instance, _ := form["instance"].(Record)
	answers, _ := form["answers"].(Record)
	formInstanceID := cleanID(instance["id"])
	formRevisionNo := int(number(instance["current_revision"]))
	status := "draft"
	activityComment := "Đã lưu nháp đối soát checklist thầu phụ"
	eventType := "subcontractor_reconciliation_saved"
	if submit {
		status = "submitted"
		activityComment = "Đã gửi đối soát checklist thầu phụ"
		eventType = "subcontractor_reconciliation_submitted"
	}
	if strings.TrimSpace(comment) != "" {
		activityComment = strings.TrimSpace(comment)
	}
	rawSummary, _ := json.Marshal(summary)

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	client := isClientSubcontractorChecklistForm(form)
	if client {
		instance, err = lockClientSubcontractorSource(ctx, tx, item, instance)
		if err != nil {
			return err
		}
	}

	existingRows, err := queryTx(ctx, tx, `
		SELECT *
		FROM subcontractor_checklist_reconciliations
		WHERE work_item_id=$1 AND ($2=false OR (project_id=$3 AND form_instance_id=$4 AND form_revision_no=$5))
		FOR UPDATE
	`, workItemID, client, projectID, formInstanceID, formRevisionNo)
	if err != nil {
		return err
	}
	if client {
		var ledger Record
		if len(existingRows) > 0 {
			ledger = existingRows[0]
		}
		if len(precondition) == 0 || precondition[0] == "" {
			return StatusError{Status: 428, Code: "PRECONDITION_REQUIRED", Message: "Tải lại đối soát trước khi lưu"}
		}
		if precondition[0] != subcontractorReconciliationPrecondition(instance, ledger) {
			return StatusError{Status: 412, Code: "PRECONDITION_FAILED", Message: "Đối soát đã thay đổi. Nội dung đang nhập vẫn được giữ; hãy tải lại trước khi lưu."}
		}
	}
	reconciliationID := ""
	if len(existingRows) > 0 {
		if fmt.Sprint(existingRows[0]["status"]) == "submitted" {
			return validationError("Đối soát checklist thầu phụ đã gửi, không thể sửa", Record{"work_item_id": workItemID})
		}
		rows, err := queryTx(ctx, tx, `
			UPDATE subcontractor_checklist_reconciliations
			SET form_instance_id=$2,
				form_revision_no=$3,
				status=$4,
				subcontractor_name=NULLIF($5,''),
				work_package=NULLIF($6,''),
				construction_area=NULLIF($7,''),
				summary_json=$8::jsonb,
				comment=NULLIF($9,''),
				submitted_by=CASE WHEN $10 THEN NULLIF($11,'')::uuid ELSE submitted_by END,
				submitted_at=CASE WHEN $10 THEN now() ELSE submitted_at END,
				updated_by=NULLIF($11,'')::uuid,
				updated_at=now()
			WHERE id=$1
			RETURNING id
		`, existingRows[0]["id"], formInstanceID, formRevisionNo, status, cleanOptionalText(answers["subcontractor_name"]), cleanOptionalText(answers["work_package"]), cleanOptionalText(answers["construction_area"]), string(rawSummary), comment, submit, actorID)
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			return sql.ErrNoRows
		}
		reconciliationID = cleanID(rows[0]["id"])
	} else {
		rows, err := queryTx(ctx, tx, `
			INSERT INTO subcontractor_checklist_reconciliations (
				project_id, work_item_id, form_instance_id, form_revision_no, status,
				subcontractor_name, work_package, construction_area, summary_json, comment,
				created_by, submitted_by, submitted_at, updated_by
			)
			VALUES (
				$1, $2, $3, $4, $5,
				NULLIF($6,''), NULLIF($7,''), NULLIF($8,''), $9::jsonb, NULLIF($10,''),
				NULLIF($11,'')::uuid,
				CASE WHEN $12 THEN NULLIF($11,'')::uuid ELSE NULL END,
				CASE WHEN $12 THEN now() ELSE NULL END,
				NULLIF($11,'')::uuid
			)
			RETURNING id
		`, projectID, workItemID, formInstanceID, formRevisionNo, status, cleanOptionalText(answers["subcontractor_name"]), cleanOptionalText(answers["work_package"]), cleanOptionalText(answers["construction_area"]), string(rawSummary), comment, actorID, submit)
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			return sql.ErrNoRows
		}
		reconciliationID = cleanID(rows[0]["id"])
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM subcontractor_checklist_reconciliation_items WHERE reconciliation_id=$1`, reconciliationID); err != nil {
		return err
	}
	for _, row := range items {
		rawPayload, _ := json.Marshal(row["payload_json"])
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO subcontractor_checklist_reconciliation_items (
				reconciliation_id, project_id, work_item_id, form_instance_id, item_index,
				checklist_title, boq_item_id, boq_item_ref, boq_work_item_name, boq_quantity, boq_unit,
				claimed_quantity, confirmed_quantity, variance_quantity, unit, status, evidence_id,
				owner_response, decision, due_date, comment, payload_json
			)
			VALUES (
				$1, $2, $3, $4, $5,
				$6, NULLIF($7,'')::uuid, NULLIF($8,''), NULLIF($9,''), $10, NULLIF($11,''),
				$12, $13, $14, NULLIF($15,''), NULLIF($16,''), NULLIF($17,'')::uuid,
				NULLIF($18,''), $19, NULLIF($20,'')::date, NULLIF($21,''), $22::jsonb
			)
		`, reconciliationID, projectID, workItemID, formInstanceID, row["item_index"],
			row["checklist_title"], cleanID(row["boq_item_id"]), row["boq_item_ref"], row["boq_work_item_name"], row["boq_quantity"], row["boq_unit"],
			row["claimed_quantity"], row["confirmed_quantity"], row["variance_quantity"], row["unit"], row["status"], cleanID(row["evidence_id"]),
			row["owner_response"], row["decision"], row["due_date"], row["comment"], string(rawPayload)); err != nil {
			return err
		}
	}
	rawMetadata, _ := json.Marshal(Record{
		"subcontractor_reconciliation_id": reconciliationID,
		"workflow_form_instance_id":       formInstanceID,
		"status":                          status,
		"summary":                         summary,
	})
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO work_item_activities (work_item_id, project_id, scope_type, scope_id, actor_id, activity_type, comment, visibility, metadata_json)
		VALUES ($1, $2, 'work_item', $1, NULLIF($3,'')::uuid, 'system_event', $4, 'project', $5::jsonb)
	`, workItemID, projectID, actorID, activityComment, string(rawMetadata)); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE work_items
		SET updated_by=NULLIF($2,'')::uuid,
			updated_at=now()
		WHERE id=$1
	`, workItemID, actorID); err != nil {
		return err
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, eventType, "subcontractor_checklist_reconciliation", reconciliationID, activityComment, Record{
		"work_item_id":              workItemID,
		"workflow_form_instance_id": formInstanceID,
		"status":                    status,
		"summary":                   summary,
	}); err != nil {
		return err
	}
	transition, err := r.startWorkItemForDomainMutationTx(ctx, tx, projectID, linkedWorkItemMutationContext(workItemID), subcontractorAcceptanceWorkItemPolicy, "subcontractor_reconciliation", reconciliationID, actorID, actorName)
	if err != nil {
		return err
	}
	item["work_item_transition"] = transition
	return tx.Commit()
}

func (r *Repository) ListProjectAcceptanceRecords(ctx context.Context, projectID string) ([]Record, error) {
	return r.Query(ctx, `
		SELECT
			ar.*,
			requested.full_name AS requested_by_name,COALESCE(requested.deleted_at IS NOT NULL,false) AS requested_by_account_deleted,
			submitted.full_name AS submitted_by_name,COALESCE(submitted.deleted_at IS NOT NULL,false) AS submitted_by_account_deleted,
			approved.full_name AS approved_by_name,COALESCE(approved.deleted_at IS NOT NULL,false) AS approved_by_account_deleted,
			wi.title AS work_item_title,
			wfi.title AS workflow_form_title,
			wfpe.file_name AS pdf_file_name,
			COALESCE(item_counts.item_count, 0) AS checklist_count
		FROM acceptance_records ar
		LEFT JOIN users requested ON requested.id=ar.requested_by
		LEFT JOIN users submitted ON submitted.id=ar.submitted_by
		LEFT JOIN users approved ON approved.id=ar.approved_by
		LEFT JOIN work_items wi ON wi.id=ar.work_item_id
		LEFT JOIN workflow_form_instances wfi ON wfi.id=ar.form_instance_id
		LEFT JOIN workflow_form_pdf_exports wfpe ON wfpe.id=ar.pdf_export_id
		LEFT JOIN (
			SELECT acceptance_id, COUNT(*) AS item_count
			FROM acceptance_checklist_items
			GROUP BY acceptance_id
		) item_counts ON item_counts.acceptance_id=ar.id
		WHERE ar.project_id=$1
		  AND ar.status <> 'deleted'
		  AND ar.status <> 'cancelled'
		ORDER BY ar.updated_at DESC, ar.created_at DESC, ar.id
	`, projectID)
}

func (r *Repository) ListAcceptanceRecordsForWorkItem(ctx context.Context, workItemID string) ([]Record, error) {
	return r.Query(ctx, `
		SELECT
			ar.*,
			requested.full_name AS requested_by_name,COALESCE(requested.deleted_at IS NOT NULL,false) AS requested_by_account_deleted,
			submitted.full_name AS submitted_by_name,COALESCE(submitted.deleted_at IS NOT NULL,false) AS submitted_by_account_deleted,
			approved.full_name AS approved_by_name,COALESCE(approved.deleted_at IS NOT NULL,false) AS approved_by_account_deleted,
			wi.title AS work_item_title,
			wfi.title AS workflow_form_title,
			wfpe.file_name AS pdf_file_name,
			COALESCE(item_counts.item_count, 0) AS checklist_count
		FROM acceptance_records ar
		LEFT JOIN users requested ON requested.id=ar.requested_by
		LEFT JOIN users submitted ON submitted.id=ar.submitted_by
		LEFT JOIN users approved ON approved.id=ar.approved_by
		LEFT JOIN work_items wi ON wi.id=ar.work_item_id
		LEFT JOIN workflow_form_instances wfi ON wfi.id=ar.form_instance_id
		LEFT JOIN workflow_form_pdf_exports wfpe ON wfpe.id=ar.pdf_export_id
		LEFT JOIN (
			SELECT acceptance_id, COUNT(*) AS item_count
			FROM acceptance_checklist_items
			GROUP BY acceptance_id
		) item_counts ON item_counts.acceptance_id=ar.id
		WHERE ar.work_item_id=$1
		  AND ar.status <> 'deleted'
		  AND ar.status <> 'cancelled'
		ORDER BY ar.updated_at DESC, ar.created_at DESC, ar.id
	`, workItemID)
}

func (r *Repository) GetAcceptanceDetail(ctx context.Context, acceptanceID string) (Record, error) {
	record, err := r.QueryOne(ctx, `
		SELECT
			ar.*,
			requested.full_name AS requested_by_name,COALESCE(requested.deleted_at IS NOT NULL,false) AS requested_by_account_deleted,
			submitted.full_name AS submitted_by_name,COALESCE(submitted.deleted_at IS NOT NULL,false) AS submitted_by_account_deleted,
			approved.full_name AS approved_by_name,COALESCE(approved.deleted_at IS NOT NULL,false) AS approved_by_account_deleted,
			wi.title AS work_item_title,
			wfi.title AS workflow_form_title,
			wfpe.file_name AS pdf_file_name
		FROM acceptance_records ar
		LEFT JOIN users requested ON requested.id=ar.requested_by
		LEFT JOIN users submitted ON submitted.id=ar.submitted_by
		LEFT JOIN users approved ON approved.id=ar.approved_by
		LEFT JOIN work_items wi ON wi.id=ar.work_item_id
		LEFT JOIN workflow_form_instances wfi ON wfi.id=ar.form_instance_id
		LEFT JOIN workflow_form_pdf_exports wfpe ON wfpe.id=ar.pdf_export_id
		WHERE ar.id=$1
		  AND ar.status <> 'deleted'
	`, acceptanceID)
	if err != nil {
		return nil, err
	}
	items, err := r.Query(ctx, `
		SELECT
			aci.*,
			er.name AS evidence_name,
			er.status AS evidence_status
		FROM acceptance_checklist_items aci
		LEFT JOIN evidence_records er ON er.id=aci.evidence_id
		WHERE aci.acceptance_id=$1
		ORDER BY aci.item_index, aci.created_at, aci.id
	`, acceptanceID)
	if err != nil {
		return nil, err
	}
	evidenceRows, err := r.Query(ctx, `
		SELECT
			link.checklist_item_id,
			link.sort_order,
			er.id,
			er.work_item_id,
			er.evidence_type,
			er.name,
			er.caption,
			er.file_url,
			er.mime_type,
			er.file_size,
			er.visibility,
			er.status,
			er.created_at,
			wi.title AS work_item_title
		FROM acceptance_checklist_item_evidence link
		JOIN acceptance_checklist_items item ON item.id=link.checklist_item_id
		JOIN evidence_records er ON er.id=link.evidence_id
		LEFT JOIN work_items wi ON wi.id=er.work_item_id
		WHERE item.acceptance_id=$1
		ORDER BY link.checklist_item_id, link.sort_order, link.created_at, er.id
	`, acceptanceID)
	if err != nil {
		return nil, err
	}
	evidenceByItem := map[string][]Record{}
	for _, evidence := range evidenceRows {
		itemID := cleanID(evidence["checklist_item_id"])
		delete(evidence, "checklist_item_id")
		evidenceByItem[itemID] = append(evidenceByItem[itemID], evidence)
	}
	for _, item := range items {
		records := evidenceByItem[cleanID(item["id"])]
		ids := make([]string, 0, len(records))
		for _, evidence := range records {
			ids = append(ids, cleanID(evidence["id"]))
		}
		item["evidence_ids"] = ids
		item["evidence_records"] = records
	}
	record["checklist"] = items
	record["checklist_count"] = len(items)
	return record, nil
}

func (r *Repository) SaveAcceptance(ctx context.Context, acceptanceID string, header Record, items []Record, actorID, actorName string) (Record, error) {
	acceptanceID = cleanID(acceptanceID)
	projectID := cleanID(header["project_id"])
	workItemID := cleanID(header["work_item_id"])
	rawSummary, _ := json.Marshal(header["summary_json"])
	activityComment := cleanOptionalText(header["notes"])
	if activityComment == "" {
		activityComment = "Đã lưu nháp nghiệm thu đầu ra"
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if acceptanceID != "" {
		existingRows, err := queryTx(ctx, tx, `
			SELECT *
			FROM acceptance_records
			WHERE id=$1
			FOR UPDATE
		`, acceptanceID)
		if err != nil {
			return nil, err
		}
		if len(existingRows) == 0 {
			return nil, sql.ErrNoRows
		}
		if !statusIn(fmt.Sprint(existingRows[0]["status"]), "draft") {
			return nil, validationError("Hồ sơ nghiệm thu đầu ra đã gửi/chốt, không thể sửa", Record{"acceptance_id": acceptanceID, "status": existingRows[0]["status"]})
		}
		if cleanID(existingRows[0]["project_id"]) != projectID {
			return nil, validationError("Hồ sơ nghiệm thu đầu ra không thuộc công trình", Record{"acceptance_id": acceptanceID})
		}
		rows, err := queryTx(ctx, tx, `
			UPDATE acceptance_records
			SET work_item_id=NULLIF($2,'')::uuid,
				form_instance_id=NULLIF($3,'')::uuid,
				pdf_export_id=NULLIF($4,'')::uuid,
				type=$5,
				title=$6,
				acceptance_date=NULLIF($7,'')::date,
				location=NULLIF($8,''),
				client_representative=NULLIF($9,''),
				contractor_representative=NULLIF($10,''),
				scope_summary=NULLIF($11,''),
				signature_status=$12,
				document_status=$13,
				notes=NULLIF($14,''),
				summary_json=$15::jsonb,
				updated_at=now()
			WHERE id=$1
			RETURNING id
		`, acceptanceID, workItemID, cleanID(header["form_instance_id"]), cleanID(header["pdf_export_id"]), header["type"],
			header["title"], header["acceptance_date"], header["location"], header["client_representative"], header["contractor_representative"],
			header["scope_summary"], header["signature_status"], header["document_status"], header["notes"], string(rawSummary))
		if err != nil {
			return nil, err
		}
		if len(rows) == 0 {
			return nil, sql.ErrNoRows
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM acceptance_checklist_items WHERE acceptance_id=$1`, acceptanceID); err != nil {
			return nil, err
		}
	} else {
		code := fmt.Sprintf("OA-%s-%d-%s", projectID[:8], timeNowUnix(), newID()[:8])
		rows, err := queryTx(ctx, tx, `
			INSERT INTO acceptance_records (
				acceptance_code, project_id, work_item_id, form_instance_id, pdf_export_id,
				type, title, requested_by, status, acceptance_date, location,
				client_representative, contractor_representative, scope_summary,
				signature_status, document_status, notes, summary_json
			)
			VALUES (
				$1, $2, NULLIF($3,'')::uuid, NULLIF($4,'')::uuid, NULLIF($5,'')::uuid,
				$6, $7, NULLIF($8,'')::uuid, 'draft', NULLIF($9,'')::date, NULLIF($10,''),
				NULLIF($11,''), NULLIF($12,''), NULLIF($13,''),
				$14, $15, NULLIF($16,''), $17::jsonb
			)
			RETURNING id
		`, code, projectID, workItemID, cleanID(header["form_instance_id"]), cleanID(header["pdf_export_id"]),
			header["type"], header["title"], actorID, header["acceptance_date"], header["location"], header["client_representative"],
			header["contractor_representative"], header["scope_summary"], header["signature_status"], header["document_status"],
			header["notes"], string(rawSummary))
		if err != nil {
			return nil, err
		}
		if len(rows) == 0 {
			return nil, sql.ErrNoRows
		}
		acceptanceID = cleanID(rows[0]["id"])
	}

	for _, item := range items {
		rawPayload, _ := json.Marshal(item["payload_json"])
		isPassed := cleanOptionalText(item["decision"]) == "pass"
		rows, err := queryTx(ctx, tx, `
			INSERT INTO acceptance_checklist_items (
				acceptance_id, item_index, title, description, is_passed, note,
				photo_required, photo_url, evidence_id, decision, severity, required_action, due_date, payload_json
			)
			VALUES (
				$1, $2, $3, NULLIF($4,''), $5, NULLIF($6,''),
				$7, NULLIF($8,''), NULLIF($9,'')::uuid, $10, $11, NULLIF($12,''), NULLIF($13,'')::date, $14::jsonb
			)
			RETURNING id
		`, acceptanceID, item["item_index"], item["title"], item["description"], isPassed, item["note"],
			item["photo_required"], item["photo_url"], cleanID(item["evidence_id"]), item["decision"], item["severity"],
			item["required_action"], item["due_date"], string(rawPayload))
		if err != nil {
			return nil, err
		}
		if len(rows) == 0 {
			return nil, sql.ErrNoRows
		}
		checklistItemID := cleanID(rows[0]["id"])
		evidenceIDs := stringSlice(item["evidence_ids"])
		if len(evidenceIDs) == 0 && cleanID(item["evidence_id"]) != "" {
			evidenceIDs = []string{cleanID(item["evidence_id"])}
		}
		for sortOrder, evidenceID := range evidenceIDs {
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO acceptance_checklist_item_evidence (
					checklist_item_id, evidence_id, sort_order, created_by
				)
				VALUES ($1, $2, $3, NULLIF($4,'')::uuid)
			`, checklistItemID, evidenceID, sortOrder, actorID); err != nil {
				return nil, err
			}
		}
	}
	rawMetadata, _ := json.Marshal(Record{
		"acceptance_id": acceptanceID,
		"status":        "draft",
		"type":          header["type"],
		"summary":       header["summary_json"],
	})
	if workItemID != "" {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO work_item_activities (work_item_id, project_id, scope_type, scope_id, actor_id, activity_type, comment, visibility, metadata_json)
			VALUES ($1, $2, 'work_item', $1, NULLIF($3,'')::uuid, 'system_event', $4, 'project', $5::jsonb)
		`, workItemID, projectID, actorID, activityComment, string(rawMetadata)); err != nil {
			return nil, err
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE work_items
			SET updated_by=NULLIF($2,'')::uuid,
				updated_at=now()
			WHERE id=$1
		`, workItemID, actorID); err != nil {
			return nil, err
		}
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "outgoing_acceptance_saved", "acceptance", acceptanceID, activityComment, Record{
		"work_item_id":     workItemID,
		"form_instance_id": cleanID(header["form_instance_id"]),
		"pdf_export_id":    cleanID(header["pdf_export_id"]),
		"type":             header["type"],
		"signature_status": header["signature_status"],
		"document_status":  header["document_status"],
		"summary":          header["summary_json"],
	}); err != nil {
		return nil, err
	}
	transition, err := r.startWorkItemForDomainMutationTx(ctx, tx, projectID, linkedWorkItemMutationContext(workItemID), finalAcceptanceWorkItemPolicy, "acceptance", acceptanceID, actorID, actorName)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	result, err := r.GetAcceptanceDetail(ctx, acceptanceID)
	return attachWorkItemTransition(result, transition), err
}

func (r *Repository) SubmitAcceptance(ctx context.Context, acceptanceID, actorID, actorName string) (Record, error) {
	acceptanceID = cleanID(acceptanceID)
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	rows, err := queryTx(ctx, tx, `
		SELECT *
		FROM acceptance_records
		WHERE id=$1
		FOR UPDATE
	`, acceptanceID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	record := rows[0]
	if fmt.Sprint(record["status"]) != "draft" {
		return nil, validationError("Chỉ hồ sơ nghiệm thu nháp mới được gửi", Record{"acceptance_id": acceptanceID, "status": record["status"]})
	}
	itemRows, err := queryTx(ctx, tx, `SELECT COUNT(*) AS total FROM acceptance_checklist_items WHERE acceptance_id=$1`, acceptanceID)
	if err != nil {
		return nil, err
	}
	if len(itemRows) == 0 || number(itemRows[0]["total"]) == 0 {
		return nil, validationError("Cần ít nhất một dòng checklist để gửi nghiệm thu đầu ra")
	}
	updated, err := queryTx(ctx, tx, `
		UPDATE acceptance_records
		SET status='submitted',
			submitted_by=NULLIF($2,'')::uuid,
			submitted_at=now(),
			updated_at=now()
		WHERE id=$1
		RETURNING *
	`, acceptanceID, actorID)
	if err != nil {
		return nil, err
	}
	if len(updated) == 0 {
		return nil, sql.ErrNoRows
	}
	projectID := cleanID(record["project_id"])
	workItemID := cleanID(record["work_item_id"])
	comment := cleanOptionalText(record["notes"])
	if comment == "" {
		comment = "Đã gửi nghiệm thu đầu ra"
	}
	rawMetadata, _ := json.Marshal(Record{
		"acceptance_id": acceptanceID,
		"status":        "submitted",
		"type":          record["type"],
		"summary":       record["summary_json"],
	})
	if workItemID != "" {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO work_item_activities (work_item_id, project_id, scope_type, scope_id, actor_id, activity_type, comment, visibility, metadata_json)
			VALUES ($1, $2, 'work_item', $1, NULLIF($3,'')::uuid, 'system_event', $4, 'project', $5::jsonb)
		`, workItemID, projectID, actorID, comment, string(rawMetadata)); err != nil {
			return nil, err
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE work_items
			SET updated_by=NULLIF($2,'')::uuid,
				updated_at=now()
			WHERE id=$1
		`, workItemID, actorID); err != nil {
			return nil, err
		}
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "outgoing_acceptance_submitted", "acceptance", acceptanceID, comment, Record{
		"work_item_id":     workItemID,
		"form_instance_id": cleanID(record["form_instance_id"]),
		"pdf_export_id":    cleanID(record["pdf_export_id"]),
		"type":             record["type"],
		"summary":          record["summary_json"],
	}); err != nil {
		return nil, err
	}
	transition, err := r.startWorkItemForDomainMutationTx(ctx, tx, projectID, linkedWorkItemMutationContext(workItemID), finalAcceptanceWorkItemPolicy, "acceptance", acceptanceID, actorID, actorName)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	result, err := r.GetAcceptanceDetail(ctx, acceptanceID)
	return attachWorkItemTransition(result, transition), err
}

func (r *Repository) GetOwnerClientReconciliation(ctx context.Context, workItemID string) (Record, error) {
	reconciliation, err := r.QueryOne(ctx, `
		SELECT ocr.*,
			submitted.full_name AS submitted_by_name,COALESCE(submitted.deleted_at IS NOT NULL,false) AS submitted_by_account_deleted,
			updated.full_name AS updated_by_name,COALESCE(updated.deleted_at IS NOT NULL,false) AS updated_by_account_deleted
		FROM owner_client_checklist_reconciliations ocr
		LEFT JOIN users submitted ON submitted.id=ocr.submitted_by
		LEFT JOIN users updated ON updated.id=ocr.updated_by
		WHERE ocr.work_item_id=$1
	`, workItemID)
	if err != nil {
		return nil, err
	}
	items, err := r.Query(ctx, `
		SELECT
			ocri.*,
			ar.title AS current_acceptance_title,
			ar.type AS current_acceptance_type,
			ar.status AS current_acceptance_status,
			ar.result AS current_acceptance_result,
			er.name AS evidence_name,
			er.status AS evidence_status
		FROM owner_client_checklist_reconciliation_items ocri
		LEFT JOIN acceptance_records ar ON ar.id=ocri.acceptance_id
		LEFT JOIN evidence_records er ON er.id=ocri.evidence_id
		WHERE ocri.reconciliation_id=$1
		ORDER BY ocri.item_index, ocri.created_at
	`, reconciliation["id"])
	if err != nil {
		return nil, err
	}
	reconciliation["items"] = items
	return reconciliation, nil
}

func (r *Repository) SaveOwnerClientReconciliation(ctx context.Context, item Record, form Record, comment string, items []Record, summary Record, submit bool, actorID, actorName string) error {
	workItemID := cleanID(item["id"])
	projectID := cleanID(item["project_id"])
	instance, _ := form["instance"].(Record)
	answers, _ := form["answers"].(Record)
	formInstanceID := cleanID(instance["id"])
	formRevisionNo := int(number(instance["current_revision"]))
	status := "draft"
	activityComment := "Đã lưu nháp đối soát checklist chủ đầu tư"
	eventType := "owner_client_reconciliation_saved"
	if submit {
		status = "submitted"
		activityComment = "Đã gửi đối soát checklist chủ đầu tư"
		eventType = "owner_client_reconciliation_submitted"
	}
	if strings.TrimSpace(comment) != "" {
		activityComment = strings.TrimSpace(comment)
	}
	rawSummary, _ := json.Marshal(summary)

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	existingRows, err := queryTx(ctx, tx, `
		SELECT *
		FROM owner_client_checklist_reconciliations
		WHERE work_item_id=$1
		FOR UPDATE
	`, workItemID)
	if err != nil {
		return err
	}
	reconciliationID := ""
	if len(existingRows) > 0 {
		if fmt.Sprint(existingRows[0]["status"]) == "submitted" {
			return validationError("Đối soát checklist chủ đầu tư đã gửi, không thể sửa", Record{"work_item_id": workItemID})
		}
		rows, err := queryTx(ctx, tx, `
			UPDATE owner_client_checklist_reconciliations
			SET form_instance_id=$2,
				form_revision_no=$3,
				status=$4,
				client_representative=NULLIF($5,''),
				company_representative=NULLIF($6,''),
				handover_scope=NULLIF($7,''),
				scope_detail=NULLIF($8,''),
				summary_json=$9::jsonb,
				comment=NULLIF($10,''),
				submitted_by=CASE WHEN $11 THEN NULLIF($12,'')::uuid ELSE submitted_by END,
				submitted_at=CASE WHEN $11 THEN now() ELSE submitted_at END,
				updated_by=NULLIF($12,'')::uuid,
				updated_at=now()
			WHERE id=$1
			RETURNING id
		`, existingRows[0]["id"], formInstanceID, formRevisionNo, status, cleanOptionalText(answers["client_representative"]), cleanOptionalText(answers["company_representative"]), cleanOptionalText(answers["handover_scope"]), cleanOptionalText(answers["scope_detail"]), string(rawSummary), comment, submit, actorID)
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			return sql.ErrNoRows
		}
		reconciliationID = cleanID(rows[0]["id"])
	} else {
		rows, err := queryTx(ctx, tx, `
			INSERT INTO owner_client_checklist_reconciliations (
				project_id, work_item_id, form_instance_id, form_revision_no, status,
				client_representative, company_representative, handover_scope, scope_detail,
				summary_json, comment, created_by, submitted_by, submitted_at, updated_by
			)
			VALUES (
				$1, $2, $3, $4, $5,
				NULLIF($6,''), NULLIF($7,''), NULLIF($8,''), NULLIF($9,''),
				$10::jsonb, NULLIF($11,''), NULLIF($12,'')::uuid,
				CASE WHEN $13 THEN NULLIF($12,'')::uuid ELSE NULL END,
				CASE WHEN $13 THEN now() ELSE NULL END,
				NULLIF($12,'')::uuid
			)
			RETURNING id
		`, projectID, workItemID, formInstanceID, formRevisionNo, status, cleanOptionalText(answers["client_representative"]), cleanOptionalText(answers["company_representative"]), cleanOptionalText(answers["handover_scope"]), cleanOptionalText(answers["scope_detail"]), string(rawSummary), comment, actorID, submit)
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			return sql.ErrNoRows
		}
		reconciliationID = cleanID(rows[0]["id"])
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM owner_client_checklist_reconciliation_items WHERE reconciliation_id=$1`, reconciliationID); err != nil {
		return err
	}
	for _, row := range items {
		rawPayload, _ := json.Marshal(row["payload_json"])
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO owner_client_checklist_reconciliation_items (
				reconciliation_id, project_id, work_item_id, form_instance_id, item_index,
				checklist_title, output_ref, status, client_decision, severity, required_action, due_date,
				evidence_id, acceptance_id, acceptance_title, acceptance_type, acceptance_status, acceptance_result,
				decision, comment, payload_json
			)
			VALUES (
				$1, $2, $3, $4, $5,
				$6, NULLIF($7,''), NULLIF($8,''), NULLIF($9,''), NULLIF($10,''), NULLIF($11,''), NULLIF($12,'')::date,
				NULLIF($13,'')::uuid, NULLIF($14,'')::uuid, NULLIF($15,''), NULLIF($16,''), NULLIF($17,''), NULLIF($18,''),
				$19, NULLIF($20,''), $21::jsonb
			)
		`, reconciliationID, projectID, workItemID, formInstanceID, row["item_index"],
			row["checklist_title"], row["output_ref"], row["status"], row["client_decision"], row["severity"], row["required_action"], row["due_date"],
			cleanID(row["evidence_id"]), cleanID(row["acceptance_id"]), row["acceptance_title"], row["acceptance_type"], row["acceptance_status"], row["acceptance_result"],
			row["decision"], row["comment"], string(rawPayload)); err != nil {
			return err
		}
	}
	rawMetadata, _ := json.Marshal(Record{
		"owner_client_reconciliation_id": reconciliationID,
		"workflow_form_instance_id":      formInstanceID,
		"status":                         status,
		"summary":                        summary,
	})
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO work_item_activities (work_item_id, project_id, scope_type, scope_id, actor_id, activity_type, comment, visibility, metadata_json)
		VALUES ($1, $2, 'work_item', $1, NULLIF($3,'')::uuid, 'system_event', $4, 'project', $5::jsonb)
	`, workItemID, projectID, actorID, activityComment, string(rawMetadata)); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE work_items
		SET updated_by=NULLIF($2,'')::uuid,
			updated_at=now()
		WHERE id=$1
	`, workItemID, actorID); err != nil {
		return err
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, eventType, "owner_client_checklist_reconciliation", reconciliationID, activityComment, Record{
		"work_item_id":              workItemID,
		"workflow_form_instance_id": formInstanceID,
		"status":                    status,
		"summary":                   summary,
	}); err != nil {
		return err
	}
	transition, err := r.startWorkItemForDomainMutationTx(ctx, tx, projectID, linkedWorkItemMutationContext(workItemID), finalAcceptanceWorkItemPolicy, "owner_client_reconciliation", reconciliationID, actorID, actorName)
	if err != nil {
		return err
	}
	item["work_item_transition"] = transition
	return tx.Commit()
}
