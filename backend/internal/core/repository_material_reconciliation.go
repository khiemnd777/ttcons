package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

func (r *Repository) GetMaterialSnapshot(ctx context.Context, workItemID string) (Record, error) {
	snapshot, err := r.QueryOne(ctx, `
		SELECT mbs.*,
			submitted.full_name AS submitted_by_name,COALESCE(submitted.deleted_at IS NOT NULL,false) AS submitted_by_account_deleted,
			updated.full_name AS updated_by_name,COALESCE(updated.deleted_at IS NOT NULL,false) AS updated_by_account_deleted
		FROM material_before_after_snapshots mbs
		LEFT JOIN users submitted ON submitted.id=mbs.submitted_by
		LEFT JOIN users updated ON updated.id=mbs.updated_by
		WHERE mbs.work_item_id=$1
	`, workItemID)
	if err != nil {
		return nil, err
	}
	items, err := r.Query(ctx, `
		SELECT
			mbsi.*,
			m.code AS current_material_code,
			m.name AS current_material_name,
			m.unit AS current_material_unit,
			m.category AS current_material_category,
			m.active AS current_material_active,
			smb.issued_quantity AS current_issued_quantity,
			smb.used_quantity AS current_used_quantity,
			smb.surplus_quantity AS current_surplus_quantity,
			before_ev.name AS before_evidence_name,
			before_ev.status AS before_evidence_status,
			after_ev.name AS after_evidence_name,
			after_ev.status AS after_evidence_status
		FROM material_before_after_snapshot_items mbsi
		LEFT JOIN materials m ON m.id=mbsi.material_id
		LEFT JOIN site_material_balances smb ON smb.id=mbsi.site_balance_id
		LEFT JOIN evidence_records before_ev ON before_ev.id=mbsi.before_evidence_id
		LEFT JOIN evidence_records after_ev ON after_ev.id=mbsi.after_evidence_id
		WHERE mbsi.snapshot_id=$1
		ORDER BY mbsi.item_index, mbsi.created_at
	`, snapshot["id"])
	if err != nil {
		return nil, err
	}
	snapshot["items"] = items
	return snapshot, nil
}

func (r *Repository) SaveMaterialSnapshot(ctx context.Context, item Record, header Record, items []Record, summary Record, submit bool, actorID, actorName string) error {
	workItemID := cleanID(item["id"])
	projectID := cleanID(item["project_id"])
	snapshotDate := cleanOptionalText(header["snapshot_date"])
	constructionArea := cleanOptionalText(header["construction_area"])
	comment := cleanOptionalText(header["comment"])
	status := "draft"
	activityComment := "Đã lưu nháp ghi nhận vật tư trước/sau"
	eventType := "material_snapshot_saved"
	if submit {
		status = "submitted"
		activityComment = "Đã gửi ghi nhận vật tư trước/sau"
		eventType = "material_snapshot_submitted"
	}
	if comment != "" {
		activityComment = comment
	}
	rawSummary, _ := json.Marshal(summary)

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	existingRows, err := queryTx(ctx, tx, `
		SELECT *
		FROM material_before_after_snapshots
		WHERE work_item_id=$1
		FOR UPDATE
	`, workItemID)
	if err != nil {
		return err
	}
	snapshotID := ""
	if len(existingRows) > 0 {
		existing := existingRows[0]
		if fmt.Sprint(existing["status"]) == "submitted" || boolValue(existing["applied_to_site_balance"]) {
			return validationError("Ghi nhận vật tư đã gửi, không thể sửa", Record{"work_item_id": workItemID})
		}
		rows, err := queryTx(ctx, tx, `
			UPDATE material_before_after_snapshots
			SET status=$2,
				snapshot_date=NULLIF($3,'')::date,
				construction_area=NULLIF($4,''),
				summary_json=$5::jsonb,
				comment=NULLIF($6,''),
				submitted_by=CASE WHEN $7 THEN NULLIF($8,'')::uuid ELSE submitted_by END,
				submitted_at=CASE WHEN $7 THEN now() ELSE submitted_at END,
				updated_by=NULLIF($8,'')::uuid,
				updated_at=now()
			WHERE id=$1
			RETURNING id
		`, existing["id"], status, snapshotDate, constructionArea, string(rawSummary), comment, submit, actorID)
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			return sql.ErrNoRows
		}
		snapshotID = cleanID(rows[0]["id"])
	} else {
		rows, err := queryTx(ctx, tx, `
			INSERT INTO material_before_after_snapshots (
				project_id, work_item_id, status, snapshot_date, construction_area,
				summary_json, comment, created_by, submitted_by, submitted_at, updated_by
			)
			VALUES (
				$1, $2, $3, NULLIF($4,'')::date, NULLIF($5,''),
				$6::jsonb, NULLIF($7,''), NULLIF($8,'')::uuid,
				CASE WHEN $9 THEN NULLIF($8,'')::uuid ELSE NULL END,
				CASE WHEN $9 THEN now() ELSE NULL END,
				NULLIF($8,'')::uuid
			)
			RETURNING id
		`, projectID, workItemID, status, snapshotDate, constructionArea, string(rawSummary), comment, actorID, submit)
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			return sql.ErrNoRows
		}
		snapshotID = cleanID(rows[0]["id"])
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM material_before_after_snapshot_items WHERE snapshot_id=$1`, snapshotID); err != nil {
		return err
	}
	for _, row := range items {
		rawPayload, _ := json.Marshal(row["payload_json"])
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO material_before_after_snapshot_items (
				snapshot_id, project_id, work_item_id, item_index,
				material_id, material_code, material_name, unit, source,
				before_quantity, added_quantity, after_quantity, waste_quantity, used_quantity,
				affects_site_balance, site_balance_id, before_evidence_id, after_evidence_id,
				construction_area, condition, comment, payload_json
			)
			VALUES (
				$1, $2, $3, $4,
				NULLIF($5,'')::uuid, NULLIF($6,''), $7, $8, $9,
				$10, $11, $12, $13, $14,
				$15, NULLIF($16,'')::uuid, NULLIF($17,'')::uuid, NULLIF($18,'')::uuid,
				NULLIF($19,''), $20, NULLIF($21,''), $22::jsonb
			)
		`, snapshotID, projectID, workItemID, row["item_index"],
			cleanID(row["material_id"]), row["material_code"], row["material_name"], row["unit"], row["source"],
			row["before_quantity"], row["added_quantity"], row["after_quantity"], row["waste_quantity"], row["used_quantity"],
			row["affects_site_balance"], cleanID(row["site_balance_id"]), cleanID(row["before_evidence_id"]), cleanID(row["after_evidence_id"]),
			row["construction_area"], row["condition"], row["comment"], string(rawPayload)); err != nil {
			return err
		}
	}
	if submit {
		if err := r.applyMaterialSnapshotToSiteBalanceTx(ctx, tx, projectID, items); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE material_before_after_snapshots
			SET applied_to_site_balance=true,
				updated_at=now()
			WHERE id=$1
		`, snapshotID); err != nil {
			return err
		}
	}
	rawMetadata, _ := json.Marshal(Record{
		"material_snapshot_id":    snapshotID,
		"status":                  status,
		"summary":                 summary,
		"applied_to_site_balance": submit,
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
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, eventType, "material_before_after_snapshot", snapshotID, activityComment, Record{
		"work_item_id":            workItemID,
		"status":                  status,
		"summary":                 summary,
		"applied_to_site_balance": submit,
	}); err != nil {
		return err
	}
	transition, err := r.startWorkItemForDomainMutationTx(ctx, tx, projectID, linkedWorkItemMutationContext(workItemID), warehouseWorkItemPolicy, "material_snapshot", snapshotID, actorID, actorName)
	if err != nil {
		return err
	}
	item["work_item_transition"] = transition
	return tx.Commit()
}

func (r *Repository) applyMaterialSnapshotToSiteBalanceTx(ctx context.Context, tx *sql.Tx, projectID string, items []Record) error {
	for _, item := range items {
		if !boolValue(item["affects_site_balance"]) {
			continue
		}
		materialID := cleanID(item["material_id"])
		balanceID := cleanID(item["site_balance_id"])
		if materialID == "" || balanceID == "" {
			return validationError("Dòng snapshot thiếu vật tư hoặc số dư công trình", Record{"item_index": item["item_index"]})
		}
		balanceRows, err := queryTx(ctx, tx, `
			SELECT *
			FROM site_material_balances
			WHERE id=$1 AND project_id=$2 AND material_id=$3
			FOR UPDATE
		`, balanceID, projectID, materialID)
		if err != nil {
			return err
		}
		if len(balanceRows) == 0 {
			return validationError("Không tìm thấy số dư vật tư tại công trình", Record{"item_index": item["item_index"], "material_id": materialID})
		}
		usedQuantity := number(item["used_quantity"])
		if materialSnapshotQuantityExceeds(usedQuantity, materialBalanceAvailable(balanceRows[0])) {
			return validationError("Số lượng sử dụng vượt vật tư còn tại công trình", Record{
				"item_index":  item["item_index"],
				"material_id": materialID,
				"available":   materialBalanceAvailable(balanceRows[0]),
				"quantity":    usedQuantity,
			})
		}
		if usedQuantity <= 0 {
			continue
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE site_material_balances
			SET used_quantity = used_quantity + $2,
				surplus_quantity = GREATEST(issued_quantity - (used_quantity + $2), 0),
				updated_at = now()
			WHERE id=$1
		`, balanceID, usedQuantity); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) ListSurplusHandlingsForWorkItem(ctx context.Context, workItemID string) ([]Record, error) {
	return r.Query(ctx, `
		SELECT
			ssh.*,
			m.code AS material_code,
			m.name AS material_name,
			m.unit AS material_unit,
			w.name AS warehouse_name,
			er.name AS evidence_name,
			er.status AS evidence_status,
			sm.movement_code AS stock_movement_code,
			sm.movement_type AS stock_movement_type
		FROM site_surplus_handlings ssh
		JOIN materials m ON m.id=ssh.material_id
		LEFT JOIN warehouses w ON w.id=ssh.warehouse_id
		LEFT JOIN evidence_records er ON er.id=ssh.evidence_id
		LEFT JOIN stock_movements sm ON sm.id=ssh.stock_movement_id
		WHERE ssh.work_item_id=$1
		  AND ssh.status <> 'cancelled'
		ORDER BY ssh.created_at DESC, ssh.id
	`, workItemID)
}

func (r *Repository) GetSurplusHandling(ctx context.Context, handlingID string) (Record, error) {
	return r.QueryOne(ctx, `
		SELECT
			ssh.*,
			m.code AS material_code,
			m.name AS material_name,
			m.unit AS material_unit,
			w.name AS warehouse_name,
			er.name AS evidence_name,
			er.status AS evidence_status,
			sm.movement_code AS stock_movement_code,
			sm.movement_type AS stock_movement_type
		FROM site_surplus_handlings ssh
		JOIN materials m ON m.id=ssh.material_id
		LEFT JOIN warehouses w ON w.id=ssh.warehouse_id
		LEFT JOIN evidence_records er ON er.id=ssh.evidence_id
		LEFT JOIN stock_movements sm ON sm.id=ssh.stock_movement_id
		WHERE ssh.id=$1
		  AND ssh.status <> 'cancelled'
	`, handlingID)
}

func (r *Repository) SaveSurplusHandling(ctx context.Context, item Record, handlingID string, row Record, actorID, actorName string) (Record, error) {
	workItemID := cleanID(item["id"])
	projectID := cleanID(item["project_id"])
	handlingID = cleanID(handlingID)
	activityComment := cleanOptionalText(row["comment"])
	if activityComment == "" {
		activityComment = "Đã lưu nháp xử lý vật tư dư"
	}
	rawPayload, _ := json.Marshal(Record{
		"material_name":           row["material_name"],
		"material_code":           row["material_code"],
		"surplus_before_handling": row["surplus_before_handling"],
	})

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if handlingID != "" {
		existingRows, err := queryTx(ctx, tx, `
			SELECT *
			FROM site_surplus_handlings
			WHERE id=$1
			FOR UPDATE
		`, handlingID)
		if err != nil {
			return nil, err
		}
		if len(existingRows) == 0 {
			return nil, sql.ErrNoRows
		}
		if fmt.Sprint(existingRows[0]["status"]) == "submitted" {
			return nil, validationError("Hồ sơ xử lý vật tư dư đã gửi, không thể sửa", Record{"surplus_handling_id": handlingID})
		}
		if cleanID(existingRows[0]["work_item_id"]) != workItemID {
			return nil, validationError("Hồ sơ xử lý vật tư dư không thuộc đầu việc", Record{"surplus_handling_id": handlingID})
		}
		rows, err := queryTx(ctx, tx, `
			UPDATE site_surplus_handlings
			SET site_balance_id=$2,
				material_id=$3,
				material_snapshot_id=NULLIF($4,'')::uuid,
				material_snapshot_item_id=NULLIF($5,'')::uuid,
				handling_type=$6,
				quantity=$7,
				unit=$8,
				warehouse_id=NULLIF($9,'')::uuid,
				custody_location=NULLIF($10,''),
				condition=$11,
				evidence_id=NULLIF($12,'')::uuid,
				comment=NULLIF($13,''),
				payload_json=$14::jsonb,
				updated_by=NULLIF($15,'')::uuid,
				updated_at=now()
			WHERE id=$1
			RETURNING id
		`, handlingID, row["site_balance_id"], row["material_id"], cleanID(row["material_snapshot_id"]), cleanID(row["material_snapshot_item_id"]),
			row["handling_type"], row["quantity"], row["unit"], cleanID(row["warehouse_id"]), row["custody_location"], row["condition"],
			cleanID(row["evidence_id"]), row["comment"], string(rawPayload), actorID)
		if err != nil {
			return nil, err
		}
		if len(rows) == 0 {
			return nil, sql.ErrNoRows
		}
	} else {
		code := fmt.Sprintf("SH-%s-%d-%s", workItemID[:8], timeNowUnix(), newID()[:8])
		rows, err := queryTx(ctx, tx, `
			INSERT INTO site_surplus_handlings (
				handling_code, project_id, work_item_id, site_balance_id, material_id,
				material_snapshot_id, material_snapshot_item_id, status, handling_type,
				quantity, unit, warehouse_id, custody_location, condition, evidence_id,
				comment, payload_json, created_by, updated_by
			)
			VALUES (
				$1, $2, $3, $4, $5,
				NULLIF($6,'')::uuid, NULLIF($7,'')::uuid, 'draft', $8,
				$9, $10, NULLIF($11,'')::uuid, NULLIF($12,''), $13, NULLIF($14,'')::uuid,
				NULLIF($15,''), $16::jsonb, NULLIF($17,'')::uuid, NULLIF($17,'')::uuid
			)
			RETURNING id
		`, code, projectID, workItemID, row["site_balance_id"], row["material_id"],
			cleanID(row["material_snapshot_id"]), cleanID(row["material_snapshot_item_id"]), row["handling_type"],
			row["quantity"], row["unit"], cleanID(row["warehouse_id"]), row["custody_location"], row["condition"], cleanID(row["evidence_id"]),
			row["comment"], string(rawPayload), actorID)
		if err != nil {
			return nil, err
		}
		if len(rows) == 0 {
			return nil, sql.ErrNoRows
		}
		handlingID = cleanID(rows[0]["id"])
	}
	rawMetadata, _ := json.Marshal(Record{
		"surplus_handling_id": handlingID,
		"status":              "draft",
		"handling_type":       row["handling_type"],
		"material_id":         row["material_id"],
		"quantity":            row["quantity"],
	})
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
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "site_surplus_handling_saved", "site_surplus_handling", handlingID, activityComment, Record{
		"work_item_id":  workItemID,
		"handling_type": row["handling_type"],
		"material_id":   row["material_id"],
		"quantity":      row["quantity"],
	}); err != nil {
		return nil, err
	}
	transition, err := r.startWorkItemForDomainMutationTx(ctx, tx, projectID, linkedWorkItemMutationContext(workItemID), warehouseWorkItemPolicy, "site_surplus_handling", handlingID, actorID, actorName)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	result, err := r.GetSurplusHandling(ctx, handlingID)
	return attachWorkItemTransition(result, transition), err
}

func (r *Repository) SubmitSurplusHandling(ctx context.Context, handlingID string, actorID, actorName string) (Record, error) {
	handlingID = cleanID(handlingID)
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	rows, err := queryTx(ctx, tx, `
		SELECT *
		FROM site_surplus_handlings
		WHERE id=$1
		FOR UPDATE
	`, handlingID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	handling := rows[0]
	if fmt.Sprint(handling["status"]) == "submitted" {
		return nil, validationError("Hồ sơ xử lý vật tư dư đã gửi", Record{"surplus_handling_id": handlingID})
	}
	projectID := cleanID(handling["project_id"])
	workItemID := cleanID(handling["work_item_id"])
	materialID := cleanID(handling["material_id"])
	warehouseID := cleanID(handling["warehouse_id"])
	handlingType := cleanOptionalText(handling["handling_type"])
	quantity := number(handling["quantity"])
	unit := cleanOptionalText(handling["unit"])
	if warehouseID == "" || cleanID(handling["evidence_id"]) == "" || cleanOptionalText(handling["custody_location"]) == "" {
		return nil, validationError("Hồ sơ xử lý vật tư dư thiếu kho, minh chứng hoặc vị trí lưu/trả")
	}
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1)::bigint)`, "stock:"+warehouseID+":"+materialID); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1)::bigint)`, "site-material:"+projectID+":"+materialID); err != nil {
		return nil, err
	}
	warehouseRows, err := queryTx(ctx, tx, `SELECT id FROM warehouses WHERE id=$1 AND active`, warehouseID)
	if err != nil {
		return nil, err
	}
	if len(warehouseRows) == 0 {
		return nil, validationError("Kho không tồn tại hoặc đã ngưng hoạt động", Record{"warehouse_id": warehouseID})
	}
	materialRows, err := queryTx(ctx, tx, `SELECT id FROM materials WHERE id=$1 AND active`, materialID)
	if err != nil {
		return nil, err
	}
	if len(materialRows) == 0 {
		return nil, validationError("Vật tư không tồn tại hoặc đã ngưng hoạt động", Record{"material_id": materialID})
	}
	balanceRows, err := queryTx(ctx, tx, `
		SELECT *
		FROM site_material_balances
		WHERE id=$1 AND project_id=$2 AND material_id=$3
		FOR UPDATE
	`, handling["site_balance_id"], projectID, materialID)
	if err != nil {
		return nil, err
	}
	if len(balanceRows) == 0 {
		return nil, validationError("Không tìm thấy số dư vật tư tại công trình", Record{"site_balance_id": handling["site_balance_id"]})
	}
	if quantityExceedsAvailable(quantity, number(balanceRows[0]["surplus_quantity"])) {
		return nil, validationError("Số lượng xử lý vượt vật tư dư", Record{"available": balanceRows[0]["surplus_quantity"], "quantity": quantity})
	}

	code := fmt.Sprintf("SM-%s-%d-%s", handlingType, timeNowUnix(), newID()[:8])
	reason := cleanOptionalText(handling["comment"])
	if reason == "" {
		reason = fmt.Sprintf("Xử lý vật tư dư %s %.3f %s", handlingType, quantity, unit)
	}
	movementRows, err := queryTx(ctx, tx, `
		INSERT INTO stock_movements
			(movement_code, project_id, warehouse_id, material_id, movement_type, quantity, unit, reason, reference_type, reference_id, created_by, approved_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,'site_surplus_handling',$9,NULLIF($10,'')::uuid,NULLIF($10,'')::uuid)
		RETURNING *
	`, code, projectID, warehouseID, materialID, handlingType, quantity, unit, reason, handlingID, actorID)
	if err != nil {
		return nil, err
	}
	if len(movementRows) == 0 {
		return nil, sql.ErrNoRows
	}
	movement := movementRows[0]
	updatedBalance, err := queryTx(ctx, tx, `
		UPDATE site_material_balances
		SET issued_quantity = GREATEST(issued_quantity - $3, 0),
			surplus_quantity = GREATEST(GREATEST(issued_quantity - $3, 0) - used_quantity, 0),
			surplus_status = CASE
				WHEN GREATEST(GREATEST(issued_quantity - $3, 0) - used_quantity, 0) > 0.000001 THEN 'partial'
				ELSE 'handled'
			END,
			updated_at = now()
		WHERE id=$1 AND project_id=$2
		RETURNING *
	`, handling["site_balance_id"], projectID, quantity)
	if err != nil {
		return nil, err
	}
	if len(updatedBalance) == 0 {
		return nil, validationError("Không cập nhật được số dư vật tư tại công trình", Record{"site_balance_id": handling["site_balance_id"]})
	}
	updated, err := queryTx(ctx, tx, `
		UPDATE site_surplus_handlings
		SET status='submitted',
			stock_movement_id=$2,
			submitted_by=NULLIF($3,'')::uuid,
			submitted_at=now(),
			updated_by=NULLIF($3,'')::uuid,
			updated_at=now()
		WHERE id=$1
		RETURNING *
	`, handlingID, movement["id"], actorID)
	if err != nil {
		return nil, err
	}
	if len(updated) == 0 {
		return nil, sql.ErrNoRows
	}
	activityComment := reason
	rawMetadata, _ := json.Marshal(Record{
		"surplus_handling_id": handlingID,
		"stock_movement_id":   movement["id"],
		"stock_movement_code": movement["movement_code"],
		"status":              "submitted",
		"handling_type":       handlingType,
		"material_id":         materialID,
		"quantity":            quantity,
		"surplus_status":      updatedBalance[0]["surplus_status"],
	})
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
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, stockMovementEvent(handlingType), "stock_movement", cleanID(movement["id"]), stockMovementDescription(StockActionRequest{
		ProjectID:     projectID,
		WarehouseID:   warehouseID,
		MaterialID:    materialID,
		MovementType:  handlingType,
		Quantity:      quantity,
		Unit:          unit,
		Reason:        reason,
		ReferenceType: "site_surplus_handling",
		ReferenceID:   handlingID,
	}), Record{
		"movement_type":       handlingType,
		"quantity":            quantity,
		"material_id":         materialID,
		"warehouse_id":        warehouseID,
		"reference_type":      "site_surplus_handling",
		"reference_id":        handlingID,
		"surplus_handling_id": handlingID,
	}); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "site_surplus_handling_submitted", "site_surplus_handling", handlingID, activityComment, Record{
		"work_item_id":        workItemID,
		"stock_movement_id":   movement["id"],
		"stock_movement_code": movement["movement_code"],
		"handling_type":       handlingType,
		"material_id":         materialID,
		"quantity":            quantity,
		"surplus_status":      updatedBalance[0]["surplus_status"],
	}); err != nil {
		return nil, err
	}
	transition, err := r.startWorkItemForDomainMutationTx(ctx, tx, projectID, linkedWorkItemMutationContext(workItemID), warehouseWorkItemPolicy, "site_surplus_handling", handlingID, actorID, actorName)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	result, err := r.GetSurplusHandling(ctx, handlingID)
	return attachWorkItemTransition(result, transition), err
}
