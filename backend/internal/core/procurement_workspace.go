package core

import (
	"context"
	"database/sql"
	"fmt"
)

func procurementVersionConflict() error {
	return conflictError("Yêu cầu mua đã được người khác cập nhật. Nội dung đang nhập vẫn được giữ; hãy xem bản mới trước khi lưu hoặc thao tác.")
}

// Capabilities describe this actor in this project, not global role names or
// Work Item can_act. The mutation handlers recheck the same source permissions.
func (s *Service) ProcurementWorkspace(ctx context.Context, id, actorID string) (Record, error) {
	row, err := s.repo.GetProcurementDetail(ctx, id)
	if err != nil {
		return nil, err
	}
	if row["status"] == "cancelled" {
		return nil, sql.ErrNoRows
	}
	approvalReady := true
	if approvalID := cleanID(row["approval_request_id"]); approvalID != "" {
		approval, err := s.repo.QueryOne(ctx, `SELECT status FROM approval_requests WHERE id=$1 AND project_id=$2`, approvalID, row["project_id"])
		if err != nil {
			return nil, err
		}
		approvalReady = approval["status"] == "approved"
		if !approvalReady {
			row["approval_message"] = "Yêu cầu phê duyệt liên quan chưa hoàn tất. Hoàn tất phê duyệt trước khi duyệt yêu cầu mua."
		}
	}
	capabilities := Record{}
	for action, permission := range map[string]string{"edit": "procurement.create", "submit": "procurement.create", "approve": "procurement.approve", "order": "procurement.order"} {
		allowed, err := s.repo.CanAccessProjectPermission(ctx, actorID, cleanID(row["project_id"]), []string{permission})
		if err != nil {
			return nil, err
		}
		status := fmt.Sprint(row["status"])
		if action == "edit" {
			allowed = allowed && !statusIn(status, "ordered", "partially_received", "received", "cancelled")
		} else {
			_, _, transitionErr := procurementTransitionForAction(action, status)
			allowed = allowed && transitionErr == nil
		}
		if action == "approve" {
			allowed = allowed && approvalReady
		}
		capabilities[action] = allowed
	}
	row["capabilities"] = capabilities
	return row, nil
}

func (r *Repository) UpdateProcurementHeader(ctx context.Context, id string, values Record, expectedVersion int, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := queryTx(ctx, tx, `SELECT * FROM procurement_requests WHERE id=$1 AND status <> 'cancelled' FOR UPDATE`, id)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	current := rows[0]
	if int(number(current["edit_version"])) != expectedVersion {
		return nil, procurementVersionConflict()
	}
	if statusIn(fmt.Sprint(current["status"]), "ordered", "partially_received", "received") {
		return nil, validationError("Yêu cầu mua đã đặt/nhận hàng, không thể sửa thông tin thương mại")
	}
	// A header supplier must not contradict snapshotted BOQ/quotation lines.
	var mismatched bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM procurement_request_items WHERE request_id=$1 AND supplier_partner_id IS NOT NULL AND supplier_partner_id IS DISTINCT FROM NULLIF($2,'')::uuid)`, id, cleanID(values["supplier_partner_id"])).Scan(&mismatched); err != nil {
		return nil, err
	}
	if mismatched {
		return nil, validationError("Nhà cung cấp phải khớp với các dòng mua từ BOQ và báo giá đã chọn", Record{"field": "supplier_partner_id"})
	}
	updated, err := queryTx(ctx, tx, `UPDATE procurement_requests SET title=$2,
		supplier_partner_id=NULLIF($3,'')::uuid, supplier_name=$4, order_reference=$5,
		expected_delivery_date=NULLIF($6,'')::date, edit_version=edit_version+1, updated_at=now()
		WHERE id=$1 RETURNING *`, id, values["title"], cleanID(values["supplier_partner_id"]), values["supplier_name"], values["order_reference"], values["expected_delivery_date"])
	if err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, cleanID(current["project_id"]), actorID, actorName, "procurement_updated", "procurement_request", id, "Cập nhật thông tin yêu cầu mua", Record{"request_code": current["request_code"], "edit_version": updated[0]["edit_version"]}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return updated[0], nil
}

func insertProcurementDiscussionUpdateTx(ctx context.Context, tx *sql.Tx, request Record, workItemID, event, actorID string) error {
	id := cleanID(request["id"])
	// Snapshot only persisted purchase values; do not resolve live BOQ prices or
	// warehouse/acceptance totals when rendering this historical record later.
	items, err := queryTx(ctx, tx, `SELECT pri.id,pri.boq_item_id,pri.supplier_quotation_item_id,
		COALESCE(m.name,b.work_item_name,'Vật tư') AS material_name,
		pri.quantity_requested,pri.unit,pri.estimated_unit_price,pri.supplier_name,
		pri.boq_price_revision_no,pri.boq_price_event_id
		FROM procurement_request_items pri LEFT JOIN materials m ON m.id=pri.material_id
		LEFT JOIN boq_items b ON b.id=pri.boq_item_id WHERE pri.request_id=$1 ORDER BY pri.created_at,pri.id`, id)
	if err != nil {
		return err
	}
	snapshot := Record{"items": items}
	for _, key := range []string{"id", "project_id", "request_code", "title", "status", "supplier_partner_id", "supplier_name", "order_reference", "expected_delivery_date", "total_estimated_amount", "edit_version"} {
		snapshot[key] = request[key]
	}
	snapshot["revision_label"] = request["request_code"]
	label := map[string]string{"generated": "Đã tạo yêu cầu mua từ BOQ · Chờ duyệt", "submit": "Đã gửi yêu cầu mua để duyệt", "approve": "Đã duyệt yêu cầu mua", "order": "Đã ghi nhận đặt hàng"}[event]
	return insertDiscussionWorkUpdateTx(ctx, tx, cleanID(request["project_id"]), workItemID, actorID, "procurement_request", id,
		fmt.Sprintf("%s:%d", event, int(number(request["edit_version"]))), label+" · "+fmt.Sprint(request["request_code"]), snapshot)
}
