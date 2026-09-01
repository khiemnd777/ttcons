package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

func (r *Repository) ListIncomingAcceptancesForProject(ctx context.Context, projectID string) ([]Record, error) {
	return r.Query(ctx, `
		SELECT
			iar.*,
			w.name AS warehouse_name,
			pr.request_code AS procurement_request_code,
			pr.title AS procurement_request_title,
			cb.full_name AS created_by_name,COALESCE(cb.deleted_at IS NOT NULL,false) AS created_by_account_deleted,
			sb.full_name AS submitted_by_name,COALESCE(sb.deleted_at IS NOT NULL,false) AS submitted_by_account_deleted,
			COALESCE(item_counts.item_count, 0) AS item_count
		FROM incoming_acceptance_records iar
		LEFT JOIN warehouses w ON w.id=iar.warehouse_id
		LEFT JOIN procurement_requests pr ON pr.id=iar.procurement_request_id
		LEFT JOIN users cb ON cb.id=iar.created_by
		LEFT JOIN users sb ON sb.id=iar.submitted_by
		LEFT JOIN (
			SELECT incoming_acceptance_id, COUNT(*) AS item_count
			FROM incoming_acceptance_items
			GROUP BY incoming_acceptance_id
		) item_counts ON item_counts.incoming_acceptance_id=iar.id
		WHERE iar.project_id=$1
		  AND iar.status <> 'cancelled'
		ORDER BY iar.updated_at DESC, iar.created_at DESC, iar.id
	`, projectID)
}

func (r *Repository) ListIncomingAcceptancesForWorkItem(ctx context.Context, workItemID string) ([]Record, error) {
	return r.Query(ctx, `
		SELECT
			iar.*,
			w.name AS warehouse_name,
			pr.request_code AS procurement_request_code,
			pr.title AS procurement_request_title,
			cb.full_name AS created_by_name,COALESCE(cb.deleted_at IS NOT NULL,false) AS created_by_account_deleted,
			sb.full_name AS submitted_by_name,COALESCE(sb.deleted_at IS NOT NULL,false) AS submitted_by_account_deleted,
			COALESCE(item_counts.item_count, 0) AS item_count
		FROM incoming_acceptance_records iar
		LEFT JOIN warehouses w ON w.id=iar.warehouse_id
		LEFT JOIN procurement_requests pr ON pr.id=iar.procurement_request_id
		LEFT JOIN users cb ON cb.id=iar.created_by
		LEFT JOIN users sb ON sb.id=iar.submitted_by
		LEFT JOIN (
			SELECT incoming_acceptance_id, COUNT(*) AS item_count
			FROM incoming_acceptance_items
			GROUP BY incoming_acceptance_id
		) item_counts ON item_counts.incoming_acceptance_id=iar.id
		WHERE iar.work_item_id=$1
		  AND iar.status <> 'cancelled'
		ORDER BY iar.updated_at DESC, iar.created_at DESC, iar.id
	`, workItemID)
}

func (r *Repository) GetIncomingAcceptance(ctx context.Context, acceptanceID string) (Record, error) {
	record, err := r.QueryOne(ctx, `
		SELECT
			iar.*,
			w.name AS warehouse_name,
			pr.request_code AS procurement_request_code,
			pr.title AS procurement_request_title,
			cb.full_name AS created_by_name,COALESCE(cb.deleted_at IS NOT NULL,false) AS created_by_account_deleted,
			sb.full_name AS submitted_by_name,COALESCE(sb.deleted_at IS NOT NULL,false) AS submitted_by_account_deleted
		FROM incoming_acceptance_records iar
		LEFT JOIN warehouses w ON w.id=iar.warehouse_id
		LEFT JOIN procurement_requests pr ON pr.id=iar.procurement_request_id
		LEFT JOIN users cb ON cb.id=iar.created_by
		LEFT JOIN users sb ON sb.id=iar.submitted_by
		WHERE iar.id=$1
		  AND iar.status <> 'cancelled'
	`, acceptanceID)
	if err != nil {
		return nil, err
	}
	items, err := r.Query(ctx, `
		SELECT
			iai.*,
			er.name AS evidence_name,
			er.status AS evidence_status,
			pri.quantity_requested AS procurement_quantity_requested,
			pri.delivery_status AS procurement_delivery_status,
			pri.approval_status AS procurement_approval_status
		FROM incoming_acceptance_items iai
		LEFT JOIN evidence_records er ON er.id=iai.evidence_id
		LEFT JOIN procurement_request_items pri ON pri.id=iai.procurement_request_item_id
		WHERE iai.incoming_acceptance_id=$1
		ORDER BY iai.item_index, iai.created_at
	`, acceptanceID)
	if err != nil {
		return nil, err
	}
	record["items"] = items
	record["item_count"] = len(items)
	return record, nil
}

func (r *Repository) SaveIncomingAcceptance(ctx context.Context, acceptanceID string, header Record, items []Record, actorID, actorName string) (Record, error) {
	acceptanceID = cleanID(acceptanceID)
	projectID := cleanID(header["project_id"])
	workItemID := cleanID(header["work_item_id"])
	rawSummary, _ := json.Marshal(header["summary_json"])
	activityComment := cleanOptionalText(header["comment"])
	if activityComment == "" {
		activityComment = "Đã lưu nháp nghiệm thu đầu vào"
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if acceptanceID != "" {
		existingRows, err := queryTx(ctx, tx, `
			SELECT *
			FROM incoming_acceptance_records
			WHERE id=$1
			FOR UPDATE
		`, acceptanceID)
		if err != nil {
			return nil, err
		}
		if len(existingRows) == 0 {
			return nil, sql.ErrNoRows
		}
		if fmt.Sprint(existingRows[0]["status"]) == "submitted" {
			return nil, validationError("Hồ sơ nghiệm thu đầu vào đã gửi, không thể sửa", Record{"incoming_acceptance_id": acceptanceID})
		}
		if cleanID(existingRows[0]["project_id"]) != projectID {
			return nil, validationError("Hồ sơ nghiệm thu đầu vào không thuộc công trình", Record{"incoming_acceptance_id": acceptanceID})
		}
		rows, err := queryTx(ctx, tx, `
			UPDATE incoming_acceptance_records
			SET work_item_id=NULLIF($2,'')::uuid,
				procurement_request_id=NULLIF($3,'')::uuid,
				warehouse_id=NULLIF($4,'')::uuid,
				source_type=$5,
				supplier_partner_id=NULLIF($6,'')::uuid,
				supplier_name=NULLIF($7,''),
				subcontractor_partner_id=NULLIF($8,'')::uuid,
				subcontractor_name=NULLIF($9,''),
				delivery_reference=NULLIF($10,''),
				result=NULLIF($11,''),
				received_at=NULLIF($12,'')::date,
				summary_json=$13::jsonb,
				comment=NULLIF($14,''),
				updated_by=NULLIF($15,'')::uuid,
				updated_at=now()
			WHERE id=$1
			RETURNING id
		`, acceptanceID, workItemID, cleanID(header["procurement_request_id"]), cleanID(header["warehouse_id"]), header["source_type"],
			cleanID(header["supplier_partner_id"]), header["supplier_name"], cleanID(header["subcontractor_partner_id"]), header["subcontractor_name"],
			header["delivery_reference"], header["result"], header["received_at"],
			string(rawSummary), header["comment"], actorID)
		if err != nil {
			return nil, err
		}
		if len(rows) == 0 {
			return nil, sql.ErrNoRows
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM incoming_acceptance_items WHERE incoming_acceptance_id=$1`, acceptanceID); err != nil {
			return nil, err
		}
	} else {
		code := fmt.Sprintf("IA-%s-%d-%s", projectID[:8], timeNowUnix(), newID()[:8])
		rows, err := queryTx(ctx, tx, `
			INSERT INTO incoming_acceptance_records (
				acceptance_code, project_id, work_item_id, procurement_request_id, warehouse_id,
				source_type, supplier_partner_id, supplier_name, subcontractor_partner_id, subcontractor_name, delivery_reference,
				status, result, received_at, summary_json, comment, created_by, updated_by
			)
			VALUES (
				$1, $2, NULLIF($3,'')::uuid, NULLIF($4,'')::uuid, NULLIF($5,'')::uuid,
				$6, NULLIF($7,'')::uuid, NULLIF($8,''), NULLIF($9,'')::uuid, NULLIF($10,''), NULLIF($11,''),
				'draft', NULLIF($12,''), NULLIF($13,'')::date, $14::jsonb, NULLIF($15,''), NULLIF($16,'')::uuid, NULLIF($16,'')::uuid
			)
			RETURNING id
		`, code, projectID, workItemID, cleanID(header["procurement_request_id"]), cleanID(header["warehouse_id"]), header["source_type"],
			cleanID(header["supplier_partner_id"]), header["supplier_name"], cleanID(header["subcontractor_partner_id"]), header["subcontractor_name"],
			header["delivery_reference"], header["result"], header["received_at"],
			string(rawSummary), header["comment"], actorID)
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
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO incoming_acceptance_items (
				incoming_acceptance_id, project_id, work_item_id, item_index,
				material_id, material_code, material_name, procurement_request_item_id,
				delivered_quantity, accepted_quantity, rejected_quantity, unit,
				condition, defect_note, evidence_id, decision, comment, payload_json
			)
			VALUES (
				$1, $2, NULLIF($3,'')::uuid, $4,
				NULLIF($5,'')::uuid, NULLIF($6,''), $7, NULLIF($8,'')::uuid,
				$9, $10, $11, $12,
				$13, NULLIF($14,''), NULLIF($15,'')::uuid, $16, NULLIF($17,''), $18::jsonb
			)
		`, acceptanceID, projectID, workItemID, item["item_index"], cleanID(item["material_id"]), item["material_code"], item["material_name"],
			cleanID(item["procurement_request_item_id"]), item["delivered_quantity"], item["accepted_quantity"], item["rejected_quantity"],
			item["unit"], item["condition"], item["defect_note"], cleanID(item["evidence_id"]), item["decision"], item["comment"], string(rawPayload)); err != nil {
			return nil, err
		}
	}
	rawMetadata, _ := json.Marshal(Record{
		"incoming_acceptance_id": acceptanceID,
		"status":                 "draft",
		"source_type":            header["source_type"],
		"result":                 header["result"],
		"summary":                header["summary_json"],
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
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "incoming_acceptance_saved", "incoming_acceptance", acceptanceID, activityComment, Record{
		"work_item_id":           workItemID,
		"procurement_request_id": cleanID(header["procurement_request_id"]),
		"warehouse_id":           cleanID(header["warehouse_id"]),
		"source_type":            header["source_type"],
		"result":                 header["result"],
		"summary":                header["summary_json"],
	}); err != nil {
		return nil, err
	}
	transition, err := r.startWorkItemForDomainMutationTx(ctx, tx, projectID, linkedWorkItemMutationContext(workItemID), incomingAcceptanceWorkItemPolicy, "incoming_acceptance", acceptanceID, actorID, actorName)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	response, err := r.GetIncomingAcceptance(ctx, acceptanceID)
	return attachWorkItemTransition(response, transition), err
}

func (r *Repository) SubmitIncomingAcceptance(ctx context.Context, acceptanceID string, result string, actorID, actorName string) (Record, error) {
	acceptanceID = cleanID(acceptanceID)
	result = cleanOptionalText(result)
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	rows, err := queryTx(ctx, tx, `
		SELECT *
		FROM incoming_acceptance_records
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
	if fmt.Sprint(record["status"]) == "submitted" {
		return nil, validationError("Hồ sơ nghiệm thu đầu vào đã gửi", Record{"incoming_acceptance_id": acceptanceID})
	}
	if fmt.Sprint(record["status"]) == "cancelled" {
		return nil, validationError("Hồ sơ nghiệm thu đầu vào đã hủy", Record{"incoming_acceptance_id": acceptanceID})
	}
	itemRows, err := queryTx(ctx, tx, `
		SELECT COUNT(*) AS total
		FROM incoming_acceptance_items
		WHERE incoming_acceptance_id=$1
	`, acceptanceID)
	if err != nil {
		return nil, err
	}
	if len(itemRows) == 0 || number(itemRows[0]["total"]) == 0 {
		return nil, validationError("Cần ít nhất một dòng vật tư để gửi nghiệm thu đầu vào")
	}
	updated, err := queryTx(ctx, tx, `
		UPDATE incoming_acceptance_records
		SET status='submitted',
			submitted_by=NULLIF($2,'')::uuid,
			submitted_at=now(),
			result=NULLIF($3,''),
			updated_by=NULLIF($2,'')::uuid,
			updated_at=now()
		WHERE id=$1
		RETURNING *
	`, acceptanceID, actorID, result)
	if err != nil {
		return nil, err
	}
	if len(updated) == 0 {
		return nil, sql.ErrNoRows
	}
	projectID := cleanID(record["project_id"])
	workItemID := cleanID(record["work_item_id"])
	if result == "" {
		result = cleanOptionalText(record["result"])
	}
	comment := cleanOptionalText(record["comment"])
	if comment == "" {
		comment = fmt.Sprintf("Gửi nghiệm thu đầu vào: %s", result)
	}
	rawMetadata, _ := json.Marshal(Record{
		"incoming_acceptance_id": acceptanceID,
		"status":                 "submitted",
		"result":                 result,
		"summary":                record["summary_json"],
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
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "incoming_acceptance_submitted", "incoming_acceptance", acceptanceID, comment, Record{
		"work_item_id":           workItemID,
		"procurement_request_id": cleanID(record["procurement_request_id"]),
		"warehouse_id":           cleanID(record["warehouse_id"]),
		"source_type":            record["source_type"],
		"result":                 result,
		"summary":                record["summary_json"],
	}); err != nil {
		return nil, err
	}
	transition, err := r.startWorkItemForDomainMutationTx(ctx, tx, projectID, linkedWorkItemMutationContext(workItemID), incomingAcceptanceWorkItemPolicy, "incoming_acceptance", acceptanceID, actorID, actorName)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	response, err := r.GetIncomingAcceptance(ctx, acceptanceID)
	return attachWorkItemTransition(response, transition), err
}
