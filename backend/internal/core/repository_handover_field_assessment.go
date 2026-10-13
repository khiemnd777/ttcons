package core

import (
	"context"
	"database/sql"
	"fmt"
)

// Lock the parent before any child observation write. Completion also updates
// this row, so observations cannot commit after a concurrent terminal transition.
func (r *Repository) lockHandoverFieldAssessment(ctx context.Context, tx *sql.Tx, id, actorID string) (Record, error) {
	rows, err := queryTx(ctx, tx, `SELECT * FROM handover_records WHERE id=$1 FOR UPDATE`, id)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	handover := rows[0]
	allowed, err := r.withReadTransaction(tx).CanAccessProjectPermission(ctx, actorID, cleanID(handover["project_id"]), []string{"handover.verify"})
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền xác minh checklist bàn giao")
	}
	if statusIn(fmt.Sprint(handover["status"]), "handed_over", "cancelled", "deleted") {
		return nil, validationError("Hồ sơ bàn giao đã khóa", Record{"status": handover["status"]})
	}
	return handover, nil
}

func handoverFieldChanges(before, after, body Record) Record {
	old, current := Record{}, Record{}
	for key := range body {
		old[key], current[key] = before[key], after[key]
	}
	return Record{"before": old, "after": current}
}

func (r *Repository) UpdateHandoverFieldAssessment(ctx context.Context, id string, body Record, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	before, err := r.lockHandoverFieldAssessment(ctx, tx, id, actorID)
	if err != nil {
		return nil, err
	}
	rows, err := queryTx(ctx, tx, `UPDATE handover_records SET
	 handover_stage=CASE WHEN $2::text IS NULL THEN handover_stage ELSE NULLIF($2,'') END,
	 handover_conclusion=CASE WHEN $3::text IS NULL THEN handover_conclusion ELSE NULLIF($3,'') END,
	 handover_conclusion_by=CASE WHEN $3::text IS NULL THEN handover_conclusion_by WHEN $3='' THEN NULL ELSE $4::uuid END,
	 handover_conclusion_at=CASE WHEN $3::text IS NULL THEN handover_conclusion_at WHEN $3='' THEN NULL ELSE now() END,
	 updated_at=now() WHERE id=$1 RETURNING *`, id, body["handover_stage"], body["handover_conclusion"], actorID)
	if err != nil {
		return nil, err
	}
	metadata := handoverFieldChanges(before, rows[0], body)
	metadata["decision_by"], metadata["decision_at"] = rows[0]["handover_conclusion_by"], rows[0]["handover_conclusion_at"]
	if err := addAuditTx(ctx, tx, cleanID(before["project_id"]), actorID, actorName, "handover_field_assessment_updated", "handover", id, "Cập nhật đánh giá thực tế bàn giao", metadata); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return rows[0], nil
}

func (r *Repository) UpdateHandoverItemFieldAssessment(ctx context.Context, id string, body Record, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	parents, err := queryTx(ctx, tx, `SELECT handover_id FROM handover_checklist_items WHERE id=$1`, id)
	if err != nil {
		return nil, err
	}
	if len(parents) == 0 {
		return nil, sql.ErrNoRows
	}
	parent, err := r.lockHandoverFieldAssessment(ctx, tx, cleanID(parents[0]["handover_id"]), actorID)
	if err != nil {
		return nil, err
	}
	items, err := queryTx(ctx, tx, `SELECT * FROM handover_checklist_items WHERE id=$1 AND handover_id=$2 AND project_id=$3 FOR UPDATE`, id, parent["id"], parent["project_id"])
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, sql.ErrNoRows
	}
	rows, err := queryTx(ctx, tx, `UPDATE handover_checklist_items SET
	 requirement=CASE WHEN $2::text IS NULL THEN requirement ELSE NULLIF($2,'') END,
	 physical_result=CASE WHEN $3::text IS NULL THEN physical_result ELSE NULLIF($3,'') END,
	 defect=CASE WHEN $4::text IS NULL THEN defect ELSE NULLIF($4,'') END,
	 due_date=CASE WHEN $5::text IS NULL THEN due_date ELSE NULLIF($5,'')::date END,
	 updated_at=now() WHERE id=$1 RETURNING *`, id, body["requirement"], body["physical_result"], body["defect"], body["due_date"])
	if err != nil {
		return nil, err
	}
	metadata := handoverFieldChanges(items[0], rows[0], body)
	metadata["handover_id"] = parent["id"]
	if err := addAuditTx(ctx, tx, cleanID(parent["project_id"]), actorID, actorName, "handover_item_field_assessment_updated", "handover_checklist_item", id, "Cập nhật đánh giá hạng mục bàn giao", metadata); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return rows[0], nil
}

func (r *Repository) CreateHandoverPhysicalItem(ctx context.Context, id string, body Record, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	parent, err := r.lockHandoverFieldAssessment(ctx, tx, id, actorID)
	if err != nil {
		return nil, err
	}
	itemID := newID()
	rows, err := queryTx(ctx, tx, `INSERT INTO handover_checklist_items
	 (id,handover_id,project_id,item_code,category,title,required,status,requirement,physical_result,defect,due_date)
	 VALUES($1,$2,$3,$4,'other',$5,false,'pending',NULLIF($6,''),NULLIF($7,''),NULLIF($8,''),NULLIF($9,'')::date) RETURNING *`,
		itemID, id, parent["project_id"], "physical_"+itemID, body["title"], body["requirement"], body["physical_result"], body["defect"], body["due_date"])
	if err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, cleanID(parent["project_id"]), actorID, actorName, "handover_physical_item_created", "handover_checklist_item", itemID, "Thêm hạng mục bàn giao thực tế", Record{"handover_id": id, "after": rows[0]}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return rows[0], nil
}
