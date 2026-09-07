package core

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
)

func (r *Repository) ListProjectBOQ(ctx context.Context, projectID string) ([]Record, error) {
	return r.Query(ctx, `
		SELECT b.*, m.name AS material_name, selected_quote.quotation_code AS selected_quotation_code,
			selected_quote.quotation_id AS selected_quotation_id,
			selected_quote.supplier_partner_id AS selected_supplier_partner_id,
			selected_quote.supplier_name AS selected_supplier_name,
			selected_quote.quoted_unit_price,
			active_procurement.request_id AS procurement_request_id,
			active_procurement.request_code AS procurement_request_code,
			active_procurement.status AS procurement_request_status,
			COALESCE(active_procurement.request_count, 0) AS active_procurement_count,
			(active_procurement.request_id IS NOT NULL) AS has_active_procurement,
			COALESCE(b.final_unit_price, b.norm_unit_price, b.expected_unit_price, b.unit_price, 0) AS effective_unit_price,
			(b.quantity * COALESCE(b.final_unit_price, b.norm_unit_price, b.expected_unit_price, b.unit_price, 0))::numeric(18,2) AS effective_total_amount,
			CASE
				WHEN b.final_price_locked THEN 'final_locked'
				WHEN b.norm_price_locked THEN 'norm_locked'
				ELSE 'expected'
			END AS price_lifecycle_status,
			COALESCE((
				SELECT jsonb_agg(to_jsonb(price_events) ORDER BY price_events.created_at DESC)
				FROM (
					SELECT event.id,event.event_type,event.price_source,event.from_unit_price,event.to_unit_price,
						event.revision_no,event.comment,event.actor_name,
						COALESCE(actor.deleted_at IS NOT NULL,false) AS actor_account_deleted,event.created_at
					FROM boq_unit_price_events event
					LEFT JOIN users actor ON actor.id=event.actor_id
					WHERE event.boq_item_id=b.id
					ORDER BY event.created_at DESC
					LIMIT 5
				) price_events
			), '[]'::jsonb) AS price_events,
			(SELECT COUNT(*) FROM boq_unit_price_events WHERE boq_item_id=b.id) AS price_event_count,
			((SELECT COUNT(*) FROM boq_unit_price_events WHERE boq_item_id=b.id) > 5) AS price_events_truncated
		FROM boq_items b
		LEFT JOIN materials m ON m.id=b.material_id
		LEFT JOIN LATERAL (
			SELECT q.id AS quotation_id, q.quotation_code, q.supplier_partner_id, q.supplier_name, qi.unit_price AS quoted_unit_price
			FROM supplier_quotation_items qi
			JOIN supplier_quotations q ON q.id=qi.quotation_id
			WHERE qi.id=b.selected_quotation_item_id
		) selected_quote ON true
		LEFT JOIN LATERAL (
			SELECT linked.request_id, linked.request_code, linked.status, COUNT(*) OVER() AS request_count
			FROM (
				SELECT DISTINCT pr.id AS request_id, pr.request_code, pr.status, pr.created_at
				FROM procurement_request_items pri
				JOIN procurement_requests pr ON pr.id=pri.request_id
				WHERE pri.boq_item_id=b.id AND pr.status <> 'cancelled'
			) linked
			ORDER BY linked.created_at DESC, linked.request_id DESC
			LIMIT 1
		) active_procurement ON true
		WHERE b.project_id=$1 AND b.status <> 'deleted'
		ORDER BY b.category, b.created_at
	`, projectID)
}

func (r *Repository) GetBOQItemDetail(ctx context.Context, itemID string) (Record, error) {
	return r.QueryOne(ctx, `
		SELECT b.*, m.name AS material_name, selected_quote.quotation_code AS selected_quotation_code,
			selected_quote.quotation_id AS selected_quotation_id,
			selected_quote.supplier_partner_id AS selected_supplier_partner_id,
			selected_quote.supplier_name AS selected_supplier_name,
			selected_quote.quoted_unit_price,
			active_procurement.request_id AS procurement_request_id,
			active_procurement.request_code AS procurement_request_code,
			active_procurement.status AS procurement_request_status,
			COALESCE(active_procurement.request_count, 0) AS active_procurement_count,
			(active_procurement.request_id IS NOT NULL) AS has_active_procurement,
			COALESCE(b.final_unit_price, b.norm_unit_price, b.expected_unit_price, b.unit_price, 0) AS effective_unit_price,
			(b.quantity * COALESCE(b.final_unit_price, b.norm_unit_price, b.expected_unit_price, b.unit_price, 0))::numeric(18,2) AS effective_total_amount,
			CASE
				WHEN b.final_price_locked THEN 'final_locked'
				WHEN b.norm_price_locked THEN 'norm_locked'
				ELSE 'expected'
			END AS price_lifecycle_status,
			COALESCE((
				SELECT jsonb_agg(to_jsonb(price_events) ORDER BY price_events.created_at DESC)
				FROM (
					SELECT event.id,event.event_type,event.price_source,event.from_unit_price,event.to_unit_price,
						event.revision_no,event.comment,event.actor_name,
						COALESCE(actor.deleted_at IS NOT NULL,false) AS actor_account_deleted,event.created_at
					FROM boq_unit_price_events event
					LEFT JOIN users actor ON actor.id=event.actor_id
					WHERE event.boq_item_id=b.id
					ORDER BY event.created_at DESC
					LIMIT 20
				) price_events
			), '[]'::jsonb) AS price_events,
			(SELECT COUNT(*) FROM boq_unit_price_events WHERE boq_item_id=b.id) AS price_event_count,
			((SELECT COUNT(*) FROM boq_unit_price_events WHERE boq_item_id=b.id) > 20) AS price_events_truncated
		FROM boq_items b
		LEFT JOIN materials m ON m.id=b.material_id
		LEFT JOIN LATERAL (
			SELECT q.id AS quotation_id, q.quotation_code, q.supplier_partner_id, q.supplier_name, qi.unit_price AS quoted_unit_price
			FROM supplier_quotation_items qi
			JOIN supplier_quotations q ON q.id=qi.quotation_id
			WHERE qi.id=b.selected_quotation_item_id
		) selected_quote ON true
		LEFT JOIN LATERAL (
			SELECT linked.request_id, linked.request_code, linked.status, COUNT(*) OVER() AS request_count
			FROM (
				SELECT DISTINCT pr.id AS request_id, pr.request_code, pr.status, pr.created_at
				FROM procurement_request_items pri
				JOIN procurement_requests pr ON pr.id=pri.request_id
				WHERE pri.boq_item_id=b.id AND pr.status <> 'cancelled'
			) linked
			ORDER BY linked.created_at DESC, linked.request_id DESC
			LIMIT 1
		) active_procurement ON true
		WHERE b.id=$1 AND b.status <> 'deleted'
	`, itemID)
}

func (r *Repository) BOQProjectAction(ctx context.Context, projectID, action string, mutationContext WorkItemMutationContext, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1)::bigint)`, "boq_lifecycle:"+projectID); err != nil {
		return nil, err
	}
	fromStatuses := []string{}
	toStatus := ""
	eventType := ""
	switch action {
	case "submit":
		fromStatuses, toStatus, eventType = []string{"draft", "revised"}, "submitted", "boq_submitted"
	case "approve":
		fromStatuses, toStatus, eventType = []string{"submitted"}, "approved", "boq_approved"
	default:
		return nil, validationError("Hành động BOQ không hợp lệ", Record{"action": action})
	}
	if action == "approve" {
		blockers, err := queryTx(ctx, tx, `
			SELECT id, work_item_name, status, norm_price_locked
			FROM boq_items
			WHERE project_id=$1 AND status='submitted' AND NOT norm_price_locked
			ORDER BY category, created_at
		`, projectID)
		if err != nil {
			return nil, err
		}
		if len(blockers) > 0 {
			return nil, validationError("Chỉ phê duyệt BOQ sau khi mọi dòng đã khóa đơn giá định mức", Record{"items": blockers})
		}
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE boq_items SET status=$3,
			approved_by=CASE WHEN $4='approve' THEN NULLIF($5,'')::uuid ELSE approved_by END,
			updated_at=now()
		WHERE project_id=$1
		  AND (($2='submit' AND status IN ('draft','revised')) OR ($2='approve' AND status='submitted'))
	`, projectID, action, toStatus, action, actorID)
	if err != nil {
		return nil, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if count == 0 {
		return nil, validationError("Không có dòng BOQ ở trạng thái phù hợp để thực hiện", Record{"action": action, "expected_statuses": fromStatuses})
	}
	if action == "approve" {
		if _, err := tx.ExecContext(ctx, `UPDATE boq_releases SET status='superseded' WHERE project_id=$1 AND status='released'`, projectID); err != nil {
			return nil, err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO boq_releases (
				project_id,release_no,status,item_count,total_amount,definition_hash,
				items_snapshot_json,released_by,released_at
			)
			SELECT
				$1,
				COALESCE((SELECT MAX(release_no)+1 FROM boq_releases WHERE project_id=$1),1),
				'released',COUNT(*),
				COALESCE(SUM(quantity*COALESCE(final_unit_price,norm_unit_price,expected_unit_price,unit_price,0)),0),
				encode(digest(COALESCE(jsonb_agg(to_jsonb(item) ORDER BY item.category,item.created_at,item.id)::text,'[]'),'sha256'),'hex'),
				COALESCE(jsonb_agg(to_jsonb(item) ORDER BY item.category,item.created_at,item.id),'[]'),
				NULLIF($2,'')::uuid,now()
			FROM boq_items item
			WHERE item.project_id=$1 AND item.status='approved'
		`, projectID, actorID); err != nil {
			return nil, err
		}
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, eventType, "project_boq", projectID, fmt.Sprintf("Chuyển %d dòng BOQ sang %s", count, toStatus), Record{"action": action, "from_statuses": fromStatuses, "to_status": toStatus, "item_count": count}); err != nil {
		return nil, err
	}
	transition, err := r.startWorkItemForDomainMutationTx(ctx, tx, projectID, mutationContext, boqReleaseWorkItemPolicy, "project_boq", projectID, actorID, actorName)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return attachWorkItemTransition(Record{"action": action, "from_status": fromStatuses[0], "from_statuses": fromStatuses, "to_status": toStatus, "item_count": count}, transition), nil
}

func (r *Repository) CreateBOQItem(ctx context.Context, body Record, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	rows, err := queryTx(ctx, tx, `
		INSERT INTO boq_items (
			project_id, category, work_item_name, description, unit, quantity, expected_unit_price,
			material_id, node_id, status, created_by, price_revision_no
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,NULLIF($8,'')::uuid,NULLIF($9,'')::uuid,$10,NULLIF($11,'')::uuid,1)
		RETURNING *
	`, body["project_id"], body["category"], body["work_item_name"], body["description"], body["unit"], body["quantity"], body["expected_unit_price"], cleanID(body["material_id"]), cleanID(body["node_id"]), body["status"], actorID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	item := rows[0]
	if err := insertBOQUnitPriceEventTx(ctx, tx, item, "expected_updated", "expected", nil, number(item["expected_unit_price"]), "Thiết lập đơn giá dự kiến ban đầu", actorID, actorName); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, fmt.Sprint(item["project_id"]), actorID, actorName, "boq_created", "boq_item", fmt.Sprint(item["id"]), "Tạo dòng BOQ", Record{"price_revision_no": item["price_revision_no"]}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetBOQItemDetail(ctx, fmt.Sprint(item["id"]))
}

func (r *Repository) UpdateBOQItem(ctx context.Context, itemID string, body Record, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	currentRows, err := queryTx(ctx, tx, `SELECT * FROM boq_items WHERE id=$1 AND status <> 'deleted' FOR UPDATE`, itemID)
	if err != nil {
		return nil, err
	}
	if len(currentRows) == 0 {
		return nil, sql.ErrNoRows
	}
	current := currentRows[0]
	if !statusIn(fmt.Sprint(current["status"]), "draft", "revised") {
		return nil, validationError("Chỉ có thể cập nhật nội dung BOQ ở trạng thái nháp hoặc cần chỉnh sửa", Record{"boq_item_id": itemID, "status": current["status"]})
	}
	if boolValue(current["final_price_locked"]) && boqBodyChangesAny(current, body, finalLockedBOQProtectedFields...) {
		return nil, validationError("Dòng BOQ đã khóa đơn giá cuối, không thể sửa khối lượng/vật tư/đơn giá", Record{"boq_item_id": itemID})
	}
	if boolValue(current["norm_price_locked"]) && boqBodyChangesAny(current, body, "expected_unit_price", "unit", "quantity", "material_id") {
		return nil, validationError("Dòng BOQ đã khóa đơn giá định mức, không thể sửa đơn giá dự kiến, đơn vị, khối lượng hoặc vật tư", Record{"boq_item_id": itemID})
	}

	specs := []string{"category", "work_item_name", "description", "unit", "quantity", "expected_unit_price", "material_id", "node_id"}
	sets := make([]string, 0, len(specs)+2)
	args := make([]any, 0, len(specs)+1)
	for _, key := range specs {
		value, ok := body[key]
		if !ok {
			continue
		}
		args = append(args, value)
		if key == "material_id" || key == "node_id" || key == "approved_by" {
			sets = append(sets, fmt.Sprintf("%s=NULLIF($%d,'')::uuid", key, len(args)))
		} else {
			sets = append(sets, fmt.Sprintf("%s=$%d", key, len(args)))
		}
	}
	expectedChanged := boqBodyChangesAny(current, body, "expected_unit_price")
	if expectedChanged {
		sets = append(sets, "price_revision_no=price_revision_no+1")
	}
	if len(sets) == 0 {
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return r.GetBOQItemDetail(ctx, itemID)
	}
	sets = append(sets, "updated_at=now()")
	args = append(args, itemID)
	updatedRows, err := queryTx(ctx, tx, fmt.Sprintf(`UPDATE boq_items SET %s WHERE id=$%d RETURNING *`, strings.Join(sets, ","), len(args)), args...)
	if err != nil {
		return nil, err
	}
	if len(updatedRows) == 0 {
		return nil, sql.ErrNoRows
	}
	updated := updatedRows[0]
	if expectedChanged {
		if err := insertBOQUnitPriceEventTx(ctx, tx, updated, "expected_updated", "expected", numberPtr(current["expected_unit_price"]), number(updated["expected_unit_price"]), "Cập nhật đơn giá dự kiến", actorID, actorName); err != nil {
			return nil, err
		}
	}
	if err := addAuditTx(ctx, tx, fmt.Sprint(updated["project_id"]), actorID, actorName, "boq_updated", "boq_item", itemID, "Cập nhật dòng BOQ", Record{"price_revision_no": updated["price_revision_no"]}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetBOQItemDetail(ctx, itemID)
}

func (r *Repository) ReviseBOQItem(ctx context.Context, itemID, reason, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var projectID string
	if err := tx.QueryRowContext(ctx, `SELECT project_id::text FROM boq_items WHERE id=$1 AND status <> 'deleted'`, itemID).Scan(&projectID); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1)::bigint)`, "boq_lifecycle:"+projectID); err != nil {
		return nil, err
	}
	rows, err := queryTx(ctx, tx, `SELECT * FROM boq_items WHERE id=$1 AND status <> 'deleted' FOR UPDATE`, itemID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	current := rows[0]
	fromStatus := fmt.Sprint(current["status"])
	if !statusIn(fromStatus, "submitted", "approved") {
		return nil, validationError("Chỉ BOQ đã gửi hoặc đã phê duyệt mới có thể yêu cầu chỉnh sửa", Record{"boq_item_id": itemID, "status": fromStatus})
	}
	if boolValue(current["final_price_locked"]) {
		return nil, validationError("Dòng BOQ đã khóa đơn giá cuối, không thể yêu cầu chỉnh sửa", Record{"boq_item_id": itemID})
	}
	var hasActiveProcurement bool
	if err := tx.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM procurement_request_items pri
			JOIN procurement_requests pr ON pr.id=pri.request_id
			WHERE pri.boq_item_id=$1 AND pr.status <> 'cancelled'
		)
	`, itemID).Scan(&hasActiveProcurement); err != nil {
		return nil, err
	}
	if hasActiveProcurement {
		return nil, validationError("Dòng BOQ đã sinh yêu cầu mua, không thể yêu cầu chỉnh sửa", Record{"boq_item_id": itemID})
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE boq_items
		SET status='revised', approved_by=NULL, updated_at=now()
		WHERE id=$1
	`, itemID); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "boq_revision_requested", "boq_item", itemID, "Yêu cầu chỉnh sửa dòng BOQ", Record{"from_status": fromStatus, "to_status": "revised", "reason": reason}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetBOQItemDetail(ctx, itemID)
}

func (r *Repository) UpdateBOQExpectedUnitPrice(ctx context.Context, itemID string, unitPrice float64, comment, actorID, actorName string) (Record, error) {
	return r.changeBOQUnitPrice(ctx, itemID, "expected_updated", unitPrice, comment, actorID, actorName)
}

func (r *Repository) LockBOQNormUnitPrice(ctx context.Context, itemID string, unitPrice float64, comment, actorID, actorName string) (Record, error) {
	return r.changeBOQUnitPrice(ctx, itemID, "norm_locked", unitPrice, comment, actorID, actorName)
}

func (r *Repository) LockBOQFinalUnitPrice(ctx context.Context, itemID string, unitPrice float64, comment, actorID, actorName string) (Record, error) {
	return r.changeBOQUnitPrice(ctx, itemID, "final_locked", unitPrice, comment, actorID, actorName)
}

func (r *Repository) changeBOQUnitPrice(ctx context.Context, itemID, eventType string, unitPrice float64, comment, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	currentRows, err := queryTx(ctx, tx, `SELECT * FROM boq_items WHERE id=$1 AND status <> 'deleted' FOR UPDATE`, itemID)
	if err != nil {
		return nil, err
	}
	if len(currentRows) == 0 {
		return nil, sql.ErrNoRows
	}
	current := currentRows[0]
	if eventType == "expected_updated" && !statusIn(fmt.Sprint(current["status"]), "draft", "revised") {
		return nil, validationError("Chỉ có thể cập nhật đơn giá dự kiến ở trạng thái nháp hoặc cần chỉnh sửa", Record{"boq_item_id": itemID, "status": current["status"]})
	}
	if eventType == "norm_locked" && fmt.Sprint(current["status"]) != "submitted" {
		return nil, validationError("Chỉ có thể khóa đơn giá định mức khi dòng BOQ đã được gửi duyệt", Record{"boq_item_id": itemID, "status": current["status"]})
	}
	if eventType == "final_locked" && fmt.Sprint(current["status"]) != "approved" {
		return nil, validationError("Chỉ có thể khóa đơn giá cuối khi dòng BOQ đã được phê duyệt", Record{"boq_item_id": itemID, "status": current["status"]})
	}
	var updatedRows []Record
	var source string
	var fromPrice *float64
	switch eventType {
	case "expected_updated":
		if boolValue(current["norm_price_locked"]) || boolValue(current["final_price_locked"]) {
			return nil, validationError("Không thể sửa đơn giá dự kiến sau khi đã khóa đơn giá định mức/cuối", Record{"boq_item_id": itemID})
		}
		fromPrice = numberPtr(current["expected_unit_price"])
		updatedRows, err = queryTx(ctx, tx, `
			UPDATE boq_items
			SET expected_unit_price=$2, price_revision_no=price_revision_no+1, updated_at=now()
			WHERE id=$1
			RETURNING *
		`, itemID, unitPrice)
		source = "expected"
	case "norm_locked":
		if boolValue(current["norm_price_locked"]) {
			return nil, validationError("Dòng BOQ đã khóa đơn giá định mức", Record{"boq_item_id": itemID})
		}
		if boolValue(current["final_price_locked"]) {
			return nil, validationError("Dòng BOQ đã khóa đơn giá cuối", Record{"boq_item_id": itemID})
		}
		fromPrice = numberPtr(current["expected_unit_price"])
		updatedRows, err = queryTx(ctx, tx, `
			UPDATE boq_items
			SET norm_unit_price=$2,
			    norm_price_locked=true,
			    norm_price_locked_by=NULLIF($3,'')::uuid,
			    norm_price_locked_at=now(),
			    norm_price_comment=$4,
			    price_revision_no=price_revision_no+1,
			    updated_at=now()
			WHERE id=$1
			RETURNING *
		`, itemID, unitPrice, actorID, comment)
		source = "norm"
	case "final_locked":
		if !boolValue(current["norm_price_locked"]) {
			return nil, validationError("Cần khóa đơn giá định mức trước khi khóa đơn giá cuối", Record{"boq_item_id": itemID})
		}
		if boolValue(current["final_price_locked"]) {
			return nil, validationError("Dòng BOQ đã khóa đơn giá cuối", Record{"boq_item_id": itemID})
		}
		fromPrice = numberPtr(current["norm_unit_price"])
		updatedRows, err = queryTx(ctx, tx, `
			UPDATE boq_items
			SET final_unit_price=$2,
			    selected_quotation_item_id=NULL,
			    price_waiver_reason=$4,
			    final_price_locked=true,
			    final_price_locked_by=NULLIF($3,'')::uuid,
			    final_price_locked_at=now(),
			    final_price_comment=$4,
			    price_revision_no=price_revision_no+1,
			    updated_at=now()
			WHERE id=$1
			RETURNING *
		`, itemID, unitPrice, actorID, comment)
		source = "final"
	default:
		return nil, validationError("Hành động đơn giá không hợp lệ", Record{"event_type": eventType})
	}
	if err != nil {
		return nil, err
	}
	if len(updatedRows) == 0 {
		return nil, sql.ErrNoRows
	}
	updated := updatedRows[0]
	if err := insertBOQUnitPriceEventTx(ctx, tx, updated, eventType, source, fromPrice, unitPrice, comment, actorID, actorName); err != nil {
		return nil, err
	}
	eventName := "boq_unit_price_" + eventType
	if err := addAuditTx(ctx, tx, fmt.Sprint(updated["project_id"]), actorID, actorName, eventName, "boq_item", itemID, "Cập nhật lifecycle đơn giá BOQ", Record{
		"price_source":      source,
		"unit_price":        unitPrice,
		"price_revision_no": updated["price_revision_no"],
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetBOQItemDetail(ctx, itemID)
}

func insertBOQUnitPriceEventTx(ctx context.Context, tx *sql.Tx, item Record, eventType, source string, fromPrice *float64, toPrice float64, comment, actorID, actorName string) error {
	snapshot := Record{
		"boq_item_id":           item["id"],
		"project_id":            item["project_id"],
		"quantity":              item["quantity"],
		"unit":                  item["unit"],
		"expected_unit_price":   item["expected_unit_price"],
		"norm_unit_price":       item["norm_unit_price"],
		"final_unit_price":      item["final_unit_price"],
		"effective_unit_price":  item["unit_price"],
		"unit_price_source":     item["unit_price_source"],
		"price_revision_no":     item["price_revision_no"],
		"norm_price_locked":     item["norm_price_locked"],
		"norm_price_locked_at":  item["norm_price_locked_at"],
		"final_price_locked":    item["final_price_locked"],
		"final_price_locked_at": item["final_price_locked_at"],
		"work_item_name":        item["work_item_name"],
		"category":              item["category"],
	}
	rawSnapshot, _ := json.Marshal(snapshot)
	var from any
	if fromPrice != nil {
		from = *fromPrice
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO boq_unit_price_events (
			project_id, boq_item_id, event_type, price_source, from_unit_price, to_unit_price,
			revision_no, comment, snapshot_json, actor_id, actor_name
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9::jsonb,NULLIF($10,'')::uuid,$11)
	`, item["project_id"], item["id"], eventType, source, from, toPrice, item["price_revision_no"], strings.TrimSpace(comment), string(rawSnapshot), actorID, actorName)
	return err
}

var finalLockedBOQProtectedFields = []string{
	"category",
	"work_item_name",
	"description",
	"unit",
	"quantity",
	"material_id",
	"node_id",
	"expected_unit_price",
}

func boqBodyChangesAny(current, body Record, fields ...string) bool {
	for _, field := range fields {
		next, ok := body[field]
		if !ok {
			continue
		}
		if boqFieldChanged(current[field], next) {
			return true
		}
	}
	return false
}

func boqFieldChanged(current any, next any) bool {
	switch next.(type) {
	case int, int64, float64, float32:
		return number(current) != number(next)
	}
	if _, ok := current.(float64); ok {
		return number(current) != number(next)
	}
	return strings.TrimSpace(fmt.Sprint(current)) != strings.TrimSpace(fmt.Sprint(next))
}

func numberPtr(value any) *float64 {
	out := number(value)
	return &out
}

func (r *Repository) ListProjectSupplierQuotations(ctx context.Context, projectID string) ([]Record, error) {
	return r.Query(ctx, `
		SELECT q.*, p.tax_code AS supplier_tax_code,
			COALESCE(summary.item_count, 0) AS item_count,
			COALESCE(summary.amount_before_tax, 0) AS amount_before_tax,
			COALESCE(summary.total_amount, 0) AS total_amount
		FROM supplier_quotations q
		JOIN partners p ON p.id=q.supplier_partner_id
		LEFT JOIN (
			SELECT quotation_id, COUNT(*) AS item_count, SUM(amount_before_tax) AS amount_before_tax, SUM(total_amount) AS total_amount
			FROM supplier_quotation_items
			GROUP BY quotation_id
		) summary ON summary.quotation_id=q.id
		WHERE q.project_id=$1 AND q.status <> 'cancelled'
		ORDER BY q.quotation_date DESC, q.created_at DESC
	`, projectID)
}

func (r *Repository) GetSupplierQuotationDetail(ctx context.Context, quotationID string) (Record, error) {
	quotation, err := r.QueryOne(ctx, `
		SELECT q.*, p.tax_code AS supplier_tax_code,
			COALESCE(summary.item_count, 0) AS item_count,
			COALESCE(summary.amount_before_tax, 0) AS amount_before_tax,
			COALESCE(summary.total_amount, 0) AS total_amount
		FROM supplier_quotations q
		JOIN partners p ON p.id=q.supplier_partner_id
		LEFT JOIN (
			SELECT quotation_id, COUNT(*) AS item_count, SUM(amount_before_tax) AS amount_before_tax, SUM(total_amount) AS total_amount
			FROM supplier_quotation_items WHERE quotation_id=$1 GROUP BY quotation_id
		) summary ON summary.quotation_id=q.id
		WHERE q.id=$1 AND q.status <> 'cancelled'
	`, quotationID)
	if err != nil {
		return nil, err
	}
	items, err := r.Query(ctx, `
		SELECT qi.*, b.category, b.work_item_name, b.norm_unit_price, b.final_unit_price,
			m.code AS material_code, m.name AS material_name,
			(qi.unit_price - COALESCE(b.norm_unit_price, b.expected_unit_price, 0)) AS variance_to_norm
		FROM supplier_quotation_items qi
		JOIN boq_items b ON b.id=qi.boq_item_id
		LEFT JOIN materials m ON m.id=qi.material_id
		WHERE qi.quotation_id=$1
		ORDER BY b.category, b.created_at, qi.created_at
	`, quotationID)
	if err != nil {
		return nil, err
	}
	quotation["items"] = items
	return quotation, nil
}

func (r *Repository) SaveSupplierQuotation(ctx context.Context, projectID, quotationID string, header Record, items []Record, mutationContext WorkItemMutationContext, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	isUpdate := quotationID != ""
	if quotationID == "" {
		rows, err := queryTx(ctx, tx, `
			INSERT INTO supplier_quotations (
				project_id, quotation_code, supplier_partner_id, supplier_name, quotation_date, valid_until, currency,
				payment_terms_days, lead_time_days, delivery_terms, warranty_terms, notes, status, created_by
			) VALUES ($1,$2,$3,$4,$5,NULLIF($6,'')::date,$7,$8,$9,$10,$11,$12,'draft',NULLIF($13,'')::uuid)
			RETURNING *
		`, projectID, header["quotation_code"], header["supplier_partner_id"], header["supplier_name"], header["quotation_date"], header["valid_until"], header["currency"], header["payment_terms_days"], header["lead_time_days"], header["delivery_terms"], header["warranty_terms"], header["notes"], actorID)
		if err != nil {
			return nil, err
		}
		if len(rows) == 0 {
			return nil, sql.ErrNoRows
		}
		quotationID = cleanID(rows[0]["id"])
	} else {
		current, err := queryTx(ctx, tx, `SELECT * FROM supplier_quotations WHERE id=$1 AND project_id=$2 FOR UPDATE`, quotationID, projectID)
		if err != nil {
			return nil, err
		}
		if len(current) == 0 {
			return nil, sql.ErrNoRows
		}
		if fmt.Sprint(current[0]["status"]) != "draft" {
			return nil, validationError("Chỉ báo giá nháp mới được chỉnh sửa", Record{"quotation_id": quotationID, "status": current[0]["status"]})
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE supplier_quotations SET
				quotation_code=$3, supplier_partner_id=$4, supplier_name=$5, quotation_date=$6,
				valid_until=NULLIF($7,'')::date, currency=$8, payment_terms_days=$9, lead_time_days=$10,
				delivery_terms=$11, warranty_terms=$12, notes=$13, updated_at=now()
			WHERE id=$1 AND project_id=$2
		`, quotationID, projectID, header["quotation_code"], header["supplier_partner_id"], header["supplier_name"], header["quotation_date"], header["valid_until"], header["currency"], header["payment_terms_days"], header["lead_time_days"], header["delivery_terms"], header["warranty_terms"], header["notes"]); err != nil {
			return nil, err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM supplier_quotation_items WHERE quotation_id=$1`, quotationID); err != nil {
			return nil, err
		}
	}
	for _, item := range items {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO supplier_quotation_items (
				quotation_id, boq_item_id, material_id, description, unit, quantity, unit_price, discount_amount,
				tax_rate, lead_time_days, compliance_status, deviation_note
			) VALUES ($1,$2,NULLIF($3,'')::uuid,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		`, quotationID, item["boq_item_id"], cleanID(item["material_id"]), item["description"], item["unit"], item["quantity"], item["unit_price"], item["discount_amount"], item["tax_rate"], item["lead_time_days"], item["compliance_status"], item["deviation_note"]); err != nil {
			return nil, err
		}
	}
	eventType := "supplier_quotation_created"
	if isUpdate {
		eventType = "supplier_quotation_updated"
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, eventType, "supplier_quotation", quotationID, "Lưu báo giá nhà cung cấp", Record{"quotation_code": header["quotation_code"], "item_count": len(items), "supplier_partner_id": header["supplier_partner_id"]}); err != nil {
		return nil, err
	}
	transition, err := r.startWorkItemForDomainMutationTx(ctx, tx, projectID, mutationContext, supplierQuotationWorkItemPolicy, "supplier_quotation", quotationID, actorID, actorName)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	result, err := r.GetSupplierQuotationDetail(ctx, quotationID)
	return attachWorkItemTransition(result, transition), err
}

func (r *Repository) SupplierQuotationAction(ctx context.Context, quotationID, action, technicalStatus, commercialStatus, comment string, mutationContext WorkItemMutationContext, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := queryTx(ctx, tx, `SELECT * FROM supplier_quotations WHERE id=$1 AND status <> 'cancelled' FOR UPDATE`, quotationID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	current := rows[0]
	projectID := cleanID(current["project_id"])
	fromStatus := fmt.Sprint(current["status"])
	toStatus := ""
	eventType := ""
	switch action {
	case "submit":
		if fromStatus != "draft" {
			return nil, validationError("Chỉ báo giá nháp mới được gửi", Record{"status": fromStatus})
		}
		var count int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM supplier_quotation_items WHERE quotation_id=$1`, quotationID).Scan(&count); err != nil {
			return nil, err
		}
		if count == 0 {
			return nil, validationError("Báo giá cần ít nhất một dòng trước khi gửi")
		}
		if validUntil := dateOnly(current["valid_until"]); validUntil != "" && validUntil < time.Now().Format("2006-01-02") {
			return nil, validationError("Báo giá đã hết hiệu lực", Record{"valid_until": validUntil})
		}
		toStatus = "submitted"
		eventType = "supplier_quotation_submitted"
		_, err = tx.ExecContext(ctx, `UPDATE supplier_quotations SET status=$2, submitted_by=NULLIF($3,'')::uuid, submitted_at=now(), updated_at=now() WHERE id=$1`, quotationID, toStatus, actorID)
	case "qualify":
		if fromStatus != "submitted" {
			return nil, validationError("Chỉ báo giá đã gửi mới được đánh giá", Record{"status": fromStatus})
		}
		var invalidItems int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM supplier_quotation_items WHERE quotation_id=$1 AND compliance_status IN ('pending','non_compliant')`, quotationID).Scan(&invalidItems); err != nil {
			return nil, err
		}
		qualified := statusIn(technicalStatus, "compliant", "conditional") && commercialStatus == "compliant" && invalidItems == 0
		if qualified {
			toStatus = "qualified"
		} else {
			toStatus = "rejected"
		}
		eventType = "supplier_quotation_qualified"
		_, err = tx.ExecContext(ctx, `
			UPDATE supplier_quotations SET status=$2, technical_status=$3, commercial_status=$4,
				qualified_by=NULLIF($5,'')::uuid, qualified_at=now(), notes=concat_ws(E'\n', NULLIF(notes,''), NULLIF($6,'')), updated_at=now()
			WHERE id=$1
		`, quotationID, toStatus, technicalStatus, commercialStatus, actorID, comment)
	default:
		return nil, validationError("Hành động báo giá không hợp lệ", Record{"action": action})
	}
	if err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, eventType, "supplier_quotation", quotationID, "Cập nhật trạng thái báo giá", Record{"from_status": fromStatus, "to_status": toStatus, "technical_status": technicalStatus, "commercial_status": commercialStatus, "comment": comment}); err != nil {
		return nil, err
	}
	transition, err := r.startWorkItemForDomainMutationTx(ctx, tx, projectID, mutationContext, supplierQuotationWorkItemPolicy, "supplier_quotation", quotationID, actorID, actorName)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	result, err := r.GetSupplierQuotationDetail(ctx, quotationID)
	return attachWorkItemTransition(result, transition), err
}

func (r *Repository) SelectSupplierQuote(ctx context.Context, quotationItemID, reason string, mutationContext WorkItemMutationContext, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := queryTx(ctx, tx, `
		SELECT qi.*, q.project_id, q.status AS quotation_status, q.valid_until, q.supplier_partner_id, q.supplier_name,
			q.technical_status, q.commercial_status, b.status AS boq_status, b.norm_price_locked, b.final_price_locked,
			b.norm_unit_price, b.price_revision_no, b.work_item_name
		FROM supplier_quotation_items qi
		JOIN supplier_quotations q ON q.id=qi.quotation_id
		JOIN boq_items b ON b.id=qi.boq_item_id
		WHERE qi.id=$1
		FOR UPDATE OF qi, q, b
	`, quotationItemID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	item := rows[0]
	if !statusIn(fmt.Sprint(item["quotation_status"]), "qualified", "selected") || !statusIn(fmt.Sprint(item["technical_status"]), "compliant", "conditional") || fmt.Sprint(item["commercial_status"]) != "compliant" || !statusIn(fmt.Sprint(item["compliance_status"]), "compliant", "conditional") {
		return nil, validationError("Chỉ được chọn dòng báo giá đã đạt đánh giá kỹ thuật và thương mại")
	}
	if validUntil := dateOnly(item["valid_until"]); validUntil != "" && validUntil < time.Now().Format("2006-01-02") {
		return nil, validationError("Báo giá đã hết hiệu lực", Record{"valid_until": validUntil})
	}
	if fmt.Sprint(item["boq_status"]) != "approved" || !boolValue(item["norm_price_locked"]) || boolValue(item["final_price_locked"]) {
		return nil, validationError("Dòng BOQ phải được duyệt, khóa giá định mức và chưa khóa giá cuối")
	}
	var procurementCount int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM procurement_request_items pri
		JOIN procurement_requests pr ON pr.id=pri.request_id
		WHERE pri.boq_item_id=$1 AND pr.status <> 'cancelled'
	`, item["boq_item_id"]).Scan(&procurementCount); err != nil {
		return nil, err
	}
	if procurementCount > 0 {
		return nil, validationError("Dòng BOQ đã phát sinh yêu cầu mua, không thể đổi lựa chọn báo giá")
	}
	if _, err := tx.ExecContext(ctx, `UPDATE supplier_quote_selections SET status='superseded', superseded_at=now() WHERE boq_item_id=$1 AND status='active'`, item["boq_item_id"]); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO supplier_quote_selections (project_id, boq_item_id, quotation_item_id, decision_reason, selected_by)
		VALUES ($1,$2,$3,$4,NULLIF($5,'')::uuid)
	`, item["project_id"], item["boq_item_id"], quotationItemID, reason, actorID); err != nil {
		return nil, err
	}
	updatedRows, err := queryTx(ctx, tx, `
		UPDATE boq_items SET selected_quotation_item_id=$2, price_waiver_reason=NULL, final_unit_price=$3,
			final_price_locked=true, final_price_locked_by=NULLIF($4,'')::uuid, final_price_locked_at=now(),
			final_price_comment=$5, price_revision_no=price_revision_no+1, updated_at=now()
		WHERE id=$1 RETURNING *
	`, item["boq_item_id"], quotationItemID, item["unit_price"], actorID, reason)
	if err != nil {
		return nil, err
	}
	if len(updatedRows) == 0 {
		return nil, sql.ErrNoRows
	}
	updated := updatedRows[0]
	if err := insertBOQUnitPriceEventTx(ctx, tx, updated, "final_locked", "final", numberPtr(item["norm_unit_price"]), number(item["unit_price"]), reason, actorID, actorName); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE supplier_quotations SET status='selected', selected_at=COALESCE(selected_at,now()), updated_at=now() WHERE id=$1`, item["quotation_id"]); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, cleanID(item["project_id"]), actorID, actorName, "supplier_quote_selected", "boq_item", cleanID(item["boq_item_id"]), "Chọn báo giá và khóa đơn giá cuối BOQ", Record{"quotation_item_id": quotationItemID, "quotation_id": item["quotation_id"], "supplier_partner_id": item["supplier_partner_id"], "unit_price": item["unit_price"], "decision_reason": reason}); err != nil {
		return nil, err
	}
	transition, err := r.startWorkItemForDomainMutationTx(ctx, tx, cleanID(item["project_id"]), mutationContext, supplierSelectionWorkItemPolicy, "supplier_quote_selection", cleanID(item["boq_item_id"]), actorID, actorName)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	result, err := r.GetBOQItemDetail(ctx, cleanID(item["boq_item_id"]))
	return attachWorkItemTransition(result, transition), err
}

func (r *Repository) SupplierQuoteComparison(ctx context.Context, projectID string) ([]Record, error) {
	return r.Query(ctx, `
		SELECT b.id AS boq_item_id, b.category, b.work_item_name, b.unit, b.quantity, b.status, b.expected_unit_price,
			b.norm_unit_price, b.final_unit_price, b.norm_price_locked, b.final_price_locked,
			b.selected_quotation_item_id, b.price_waiver_reason,
			COALESCE((
				SELECT jsonb_agg(jsonb_build_object(
					'quotation_item_id', qi.id, 'quotation_id', q.id, 'quotation_code', q.quotation_code,
					'supplier_partner_id', q.supplier_partner_id, 'supplier_name', q.supplier_name,
					'quotation_status', q.status, 'valid_until', q.valid_until, 'unit_price', qi.unit_price,
					'discount_amount', qi.discount_amount, 'tax_rate', qi.tax_rate, 'total_amount', qi.total_amount,
					'lead_time_days', qi.lead_time_days, 'compliance_status', qi.compliance_status,
					'variance_to_norm', qi.unit_price - COALESCE(b.norm_unit_price, b.expected_unit_price, 0),
					'is_selected', qi.id=b.selected_quotation_item_id
				) ORDER BY qi.unit_price, q.supplier_name)
				FROM supplier_quotation_items qi
				JOIN supplier_quotations q ON q.id=qi.quotation_id
				WHERE qi.boq_item_id=b.id AND q.status <> 'cancelled'
			), '[]'::jsonb) AS quotations
		FROM boq_items b
		WHERE b.project_id=$1 AND b.status <> 'deleted'
		ORDER BY b.category, b.created_at
	`, projectID)
}

func (r *Repository) GenerateProcurementFromBOQ(ctx context.Context, projectID string, mutationContext WorkItemMutationContext, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1)::bigint)`, "boq_lifecycle:"+projectID); err != nil {
		return nil, err
	}

	blockers, err := queryTx(ctx, tx, `
		SELECT id, work_item_name, status, unit_price_source, expected_unit_price, norm_unit_price, final_unit_price,
			price_revision_no, selected_quotation_item_id, price_waiver_reason
		FROM boq_items b
		WHERE b.project_id=$1 AND b.status='approved'
		  AND (NOT b.final_price_locked OR (b.selected_quotation_item_id IS NULL AND NULLIF(b.price_waiver_reason, '') IS NULL))
		  AND NOT EXISTS (
			SELECT 1
			FROM procurement_request_items pri
			JOIN procurement_requests pr ON pr.id=pri.request_id
			WHERE pri.boq_item_id=b.id
			  AND pr.status <> 'cancelled'
		  )
		ORDER BY b.category, b.created_at
	`, projectID)
	if err != nil {
		return nil, err
	}
	if len(blockers) > 0 {
		return nil, validationError("Chỉ sinh yêu cầu mua khi mọi dòng BOQ đã duyệt, khóa giá cuối và có báo giá được chọn hoặc lý do miễn báo giá", Record{"items": blockers})
	}

	rows, err := queryTx(ctx, tx, `
		SELECT b.*, m.name AS material_name, m.standard_price,
			COALESCE(q.supplier_name, m.default_supplier) AS selected_supplier_name,
			COALESCE(q.supplier_partner_id, m.default_supplier_partner_id) AS selected_supplier_partner_id,
			qi.id AS supplier_quotation_item_id, qi.unit_price AS quoted_unit_price,
			COALESCE(b.final_unit_price, b.unit_price) AS effective_unit_price,
			(
				SELECT e.id
				FROM boq_unit_price_events e
				WHERE e.boq_item_id=b.id AND e.event_type='final_locked'
				ORDER BY e.created_at DESC
				LIMIT 1
			) AS final_price_event_id
		FROM boq_items b
		LEFT JOIN materials m ON m.id=b.material_id
		LEFT JOIN supplier_quotation_items qi ON qi.id=b.selected_quotation_item_id
		LEFT JOIN supplier_quotations q ON q.id=qi.quotation_id
		WHERE b.project_id=$1 AND b.status='approved' AND b.final_price_locked
		  AND (b.selected_quotation_item_id IS NOT NULL OR NULLIF(b.price_waiver_reason, '') IS NOT NULL)
		  AND NOT EXISTS (
			SELECT 1
			FROM procurement_request_items pri
			JOIN procurement_requests pr ON pr.id=pri.request_id
			WHERE pri.boq_item_id=b.id
			  AND pr.status <> 'cancelled'
		  )
	`, projectID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, errors.New("Không có dòng BOQ đủ điều kiện để sinh yêu cầu mua")
	}
	groups := map[string][]Record{}
	groupOrder := []string{}
	for _, row := range rows {
		key := cleanID(row["selected_supplier_partner_id"])
		if key == "" {
			key = "unassigned"
		}
		if _, ok := groups[key]; !ok {
			groupOrder = append(groupOrder, key)
		}
		groups[key] = append(groups[key], row)
	}
	requestIDs := make([]string, 0, len(groupOrder))
	for groupIndex, key := range groupOrder {
		groupRows := groups[key]
		total := 0.0
		for _, row := range groupRows {
			total += number(row["quantity"]) * number(row["effective_unit_price"])
		}
		supplierName := cleanOptionalText(groupRows[0]["selected_supplier_name"])
		title := "Yêu cầu mua sinh từ BOQ"
		if supplierName != "" {
			title += " - " + supplierName
		}
		requestCode := fmt.Sprintf("PR-%s-%d-%02d", projectID[:8], timeNowUnix(), groupIndex+1)
		reqRows, err := queryTx(ctx, tx, `
			INSERT INTO procurement_requests
				(project_id, request_code, title, requested_by, status, total_estimated_amount, budget_status, supplier_partner_id, supplier_name)
			VALUES ($1,$2,$3,NULLIF($4,'')::uuid,'waiting_approval',$5,'within_budget',NULLIF($6,'')::uuid,$7)
			RETURNING *
		`, projectID, requestCode, title, actorID, total, func() string {
			if key == "unassigned" {
				return ""
			}
			return key
		}(), supplierName)
		if err != nil {
			return nil, err
		}
		if len(reqRows) == 0 {
			return nil, sql.ErrNoRows
		}
		requestID := cleanID(reqRows[0]["id"])
		requestIDs = append(requestIDs, requestID)
		for _, row := range groupRows {
			price := number(row["effective_unit_price"])
			quotedPrice := row["quoted_unit_price"]
			var variance any
			if row["supplier_quotation_item_id"] != nil {
				variance = number(quotedPrice) - price
			}
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO procurement_request_items
					(request_id, material_id, boq_item_id, quantity_planned, quantity_requested, unit, estimated_unit_price,
					 supplier_partner_id, supplier_name, delivery_status, approval_status, boq_price_source, boq_price_revision_no,
					 boq_price_event_id, supplier_quotation_item_id, quoted_unit_price, price_variance_amount)
				VALUES ($1,$2,$3,$4,$5,$6,$7,NULLIF($8,'')::uuid,$9,'pending','pending','final',$10,NULLIF($11,'')::uuid,NULLIF($12,'')::uuid,$13,$14)
			`, requestID, row["material_id"], row["id"], row["quantity"], row["quantity"], row["unit"], price,
				cleanID(row["selected_supplier_partner_id"]), row["selected_supplier_name"], row["price_revision_no"], cleanID(row["final_price_event_id"]), cleanID(row["supplier_quotation_item_id"]), quotedPrice, variance); err != nil {
				return nil, err
			}
		}
		if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "material_requested", "procurement_request", requestID, "Sinh yêu cầu mua từ BOQ theo nhà cung cấp", Record{"total": total, "boq_price_source": "final", "supplier_partner_id": key, "item_count": len(groupRows)}); err != nil {
			return nil, err
		}
	}
	transition, err := r.startWorkItemForDomainMutationTx(ctx, tx, projectID, mutationContext, boqProcurementWorkItemPolicy, "procurement_request", requestIDs[0], actorID, actorName)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	requests := make([]Record, 0, len(requestIDs))
	for _, requestID := range requestIDs {
		request, err := r.GetProcurementDetail(ctx, requestID)
		if err != nil {
			return nil, err
		}
		requests = append(requests, request)
	}
	result := requests[0]
	result["requests"] = requests
	result["request_count"] = len(requests)
	return attachWorkItemTransition(result, transition), nil
}

func (r *Repository) GetProcurementDetail(ctx context.Context, requestID string) (Record, error) {
	req, err := r.GetByID(ctx, "procurement_requests", requestID)
	if err != nil {
		return nil, err
	}
	items, _ := r.Query(ctx, `
		SELECT pri.*, m.name AS material_name, b.work_item_name, b.price_waiver_reason,
			q.id AS quotation_id, q.quotation_code, q.quotation_date, q.valid_until, q.currency,
			COALESCE(received.accepted_quantity,0) AS accepted_quantity,
			LEAST(COALESCE(received.accepted_quantity,0), COALESCE(received.stock_received_quantity,0)+COALESCE(legacy_stock.received_quantity,0)) AS stock_received_quantity,
			GREATEST(pri.quantity_requested - COALESCE(received.accepted_quantity,0),0) AS remaining_quantity,
			GREATEST(COALESCE(received.accepted_quantity,0)-COALESCE(received.stock_received_quantity,0)-COALESCE(legacy_stock.received_quantity,0),0) AS awaiting_stock_quantity
		FROM procurement_request_items pri
		LEFT JOIN materials m ON m.id=pri.material_id
		LEFT JOIN boq_items b ON b.id=pri.boq_item_id
		LEFT JOIN supplier_quotation_items qi ON qi.id=pri.supplier_quotation_item_id
		LEFT JOIN supplier_quotations q ON q.id=qi.quotation_id
		LEFT JOIN LATERAL (
			SELECT SUM(iai.accepted_quantity) AS accepted_quantity,
				SUM(COALESCE(stock.received_quantity,0)) AS stock_received_quantity
			FROM incoming_acceptance_items iai
			JOIN incoming_acceptance_records iar ON iar.id=iai.incoming_acceptance_id AND iar.status='submitted'
			LEFT JOIN LATERAL (
				SELECT SUM(CASE WHEN sm.movement_type='receive' THEN sm.quantity WHEN sm.movement_type='adjustment_out' THEN -sm.quantity ELSE 0 END) AS received_quantity
				FROM stock_movements sm
				WHERE sm.movement_type IN ('receive','adjustment_out')
				  AND sm.reference_type='incoming_acceptance_item'
				  AND sm.reference_id=iai.id
			) stock ON true
			WHERE iai.procurement_request_item_id=pri.id
		) received ON true
		LEFT JOIN LATERAL (
			SELECT SUM(CASE WHEN sm.movement_type='receive' THEN sm.quantity WHEN sm.movement_type='adjustment_out' THEN -sm.quantity ELSE 0 END) AS received_quantity
			FROM stock_movements sm
			WHERE sm.movement_type IN ('receive','adjustment_out')
			  AND sm.reference_type='boq_item'
			  AND sm.reference_id=pri.boq_item_id
		) legacy_stock ON true
		WHERE pri.request_id=$1
		ORDER BY pri.created_at
	`, requestID)
	req["items"] = items
	return req, nil
}

func (r *Repository) ListProjectProcurementRequests(ctx context.Context, projectID string) ([]Record, error) {
	return r.Query(ctx, `
		SELECT
			pr.*,
			u.full_name AS requested_by_name,
			COALESCE(u.deleted_at IS NOT NULL,false) AS requested_by_account_deleted,
			COALESCE(item_counts.item_count, 0) AS item_count
		FROM procurement_requests pr
		LEFT JOIN users u ON u.id=pr.requested_by
		LEFT JOIN (
			SELECT request_id, COUNT(*) AS item_count
			FROM procurement_request_items
			GROUP BY request_id
		) item_counts ON item_counts.request_id=pr.id
		WHERE pr.project_id=$1
		  AND pr.status <> 'cancelled'
		ORDER BY pr.created_at DESC, pr.id
	`, projectID)
}

func (r *Repository) ListProjectProcurementRequestItems(ctx context.Context, projectID string) ([]Record, error) {
	return r.Query(ctx, `
		SELECT
			pri.*,
			pr.project_id,
			pr.request_code,
			pr.title AS procurement_request_title,
			m.code AS material_code,
			m.name AS material_name,
			b.work_item_name
		FROM procurement_request_items pri
		JOIN procurement_requests pr ON pr.id=pri.request_id
		LEFT JOIN materials m ON m.id=pri.material_id
		LEFT JOIN boq_items b ON b.id=pri.boq_item_id
		WHERE pr.project_id=$1
		  AND pr.status <> 'cancelled'
		ORDER BY pr.created_at DESC, pri.created_at, pri.id
	`, projectID)
}

func (r *Repository) ProcurementAction(ctx context.Context, id, action string, req ActionRequest, actorID, actorName string) (Record, error) {
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
	fromStatus := fmt.Sprint(current["status"])
	toStatus := ""
	event := ""
	if action == "submit" {
		var itemCount, incompleteCount int
		if err := tx.QueryRowContext(ctx, `
			SELECT COUNT(*), COUNT(*) FILTER (
				WHERE pri.supplier_partner_id IS NULL
				   OR (pri.supplier_quotation_item_id IS NULL AND NULLIF(b.price_waiver_reason,'') IS NULL)
			)
			FROM procurement_request_items pri
			LEFT JOIN boq_items b ON b.id=pri.boq_item_id
			WHERE pri.request_id=$1
		`, id).Scan(&itemCount, &incompleteCount); err != nil {
			return nil, err
		}
		if itemCount == 0 || incompleteCount > 0 {
			return nil, validationError("Yêu cầu mua cần có dòng hàng, nhà cung cấp và nguồn giá đã duyệt trước khi gửi", Record{"item_count": itemCount, "incomplete_item_count": incompleteCount})
		}
	}
	if action == "order" {
		if cleanID(current["supplier_partner_id"]) == "" || strings.TrimSpace(fmt.Sprint(current["order_reference"])) == "" || dateOnly(current["expected_delivery_date"]) == "" {
			return nil, validationError("Đặt hàng cần nhà cung cấp, số đơn đặt hàng và ngày giao dự kiến")
		}
	}
	if action == "approve" && cleanID(current["approval_request_id"]) != "" {
		var approvalStatus string
		if err := tx.QueryRowContext(ctx, `SELECT status FROM approval_requests WHERE id=$1 AND project_id=$2`, current["approval_request_id"], current["project_id"]).Scan(&approvalStatus); err != nil {
			return nil, err
		}
		if approvalStatus != "approved" {
			return nil, validationError("Yêu cầu phê duyệt cấu trúc chưa hoàn tất", Record{"approval_request_id": current["approval_request_id"], "approval_status": approvalStatus})
		}
	}
	if action == "receive" {
		if !statusIn(fromStatus, "ordered", "partially_received") {
			return nil, validationError("Chỉ yêu cầu mua đã đặt hàng mới được đối soát nhận hàng", Record{"status": fromStatus})
		}
		receipts, err := queryTx(ctx, tx, `
			SELECT pri.id, pri.quantity_requested,
				COALESCE(SUM(iai.accepted_quantity) FILTER (WHERE iar.id IS NOT NULL),0) AS accepted_quantity
			FROM procurement_request_items pri
			LEFT JOIN incoming_acceptance_items iai ON iai.procurement_request_item_id=pri.id
			LEFT JOIN incoming_acceptance_records iar ON iar.id=iai.incoming_acceptance_id AND iar.status='submitted'
			WHERE pri.request_id=$1
			GROUP BY pri.id, pri.quantity_requested
		`, id)
		if err != nil {
			return nil, err
		}
		acceptedAny := false
		allReceived := len(receipts) > 0
		for _, receipt := range receipts {
			accepted := number(receipt["accepted_quantity"])
			requested := number(receipt["quantity_requested"])
			acceptedAny = acceptedAny || accepted > 0
			allReceived = allReceived && accepted+0.000001 >= requested
			deliveryStatus := "pending"
			if accepted > 0 && accepted+0.000001 < requested {
				deliveryStatus = "partial"
			} else if accepted+0.000001 >= requested {
				deliveryStatus = "received"
			}
			if _, err := tx.ExecContext(ctx, `UPDATE procurement_request_items SET delivery_status=$2 WHERE id=$1`, receipt["id"], deliveryStatus); err != nil {
				return nil, err
			}
		}
		if !acceptedAny {
			return nil, validationError("Chưa có khối lượng đạt nghiệm thu đầu vào để ghi nhận nhận hàng")
		}
		toStatus = "partially_received"
		event = "procurement_acceptance_partially_reconciled"
		if allReceived {
			toStatus = "received"
			event = "procurement_acceptance_reconciled"
		}
	} else {
		toStatus, event, err = procurementTransitionForAction(action, fromStatus)
		if err != nil {
			return nil, err
		}
	}
	updated, err := queryTx(ctx, tx, `
		UPDATE procurement_requests
		SET status=$2, ordered_at=CASE WHEN $4='order' THEN COALESCE(ordered_at,now()) ELSE ordered_at END, updated_at=now()
		WHERE id=$1 AND status=$3
		RETURNING *
	`, id, toStatus, fromStatus, action)
	if err != nil {
		return nil, err
	}
	if len(updated) == 0 {
		return nil, validationError("Trạng thái yêu cầu mua đã thay đổi, vui lòng tải lại trước khi thao tác", Record{"expected_status": fromStatus})
	}
	comment := actionComment(req)
	metadata := Record{"action": action, "from_status": fromStatus, "to_status": toStatus}
	if comment != "" {
		metadata["comment"] = comment
	}
	if err := addAuditTx(ctx, tx, fmt.Sprint(current["project_id"]), actorID, actorName, event, "procurement_request", id, fmt.Sprintf("Cập nhật yêu cầu mua từ %s sang %s", fromStatus, toStatus), metadata); err != nil {
		return nil, err
	}
	transition, err := r.startWorkItemForDomainMutationTx(ctx, tx, cleanID(current["project_id"]), req.WorkItemContext, inputContractProcurementWorkItemPolicy, "procurement_request", id, actorID, actorName)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	result, err := r.GetProcurementDetail(ctx, id)
	return attachWorkItemTransition(result, transition), err
}

func procurementTransitionForAction(action, fromStatus string) (string, string, error) {
	switch action {
	case "submit":
		if statusIn(fromStatus, "draft", "submitted", "changes_requested") {
			return "waiting_approval", "review_requested", nil
		}
	case "approve":
		if statusIn(fromStatus, "waiting_approval", "submitted") {
			return "approved", "material_approved", nil
		}
	case "order":
		if fromStatus == "approved" {
			return "ordered", "material_ordered", nil
		}
	case "receive":
		if fromStatus == "ordered" {
			return "received", "procurement_acceptance_reconciled", nil
		}
	default:
		return "", "", validationError("Hành động yêu cầu mua không hợp lệ", Record{"action": action})
	}
	return "", "", validationError("Trạng thái yêu cầu mua không cho phép thao tác này", Record{"action": action, "status": fromStatus})
}

func actionComment(req ActionRequest) string {
	comment := strings.TrimSpace(req.Comment)
	if comment == "" {
		comment = strings.TrimSpace(req.Reason)
	}
	return comment
}

func (r *Repository) ListProjectContracts(ctx context.Context, projectID string) ([]Record, error) {
	return r.Query(ctx, `
		SELECT c.*, p.tax_code AS counterparty_tax_code,
			cv.version_no AS current_version_no, cv.version_label AS current_version_label, cv.status AS current_version_status,
			COALESCE(summary.milestone_count,0) AS milestone_count,
			COALESCE(summary.variation_count,0) AS variation_count,
			COALESCE(summary.retention_outstanding,0) AS retention_outstanding,
			COALESCE(summary.signed_scan_received,false) AS signed_scan_received
		FROM contracts c
		LEFT JOIN partners p ON p.id=c.counterparty_partner_id
		LEFT JOIN contract_versions cv ON cv.id=c.current_version_id
		LEFT JOIN LATERAL (
			SELECT
				(SELECT COUNT(*) FROM contract_milestones cm WHERE cm.contract_id=c.id AND cm.status <> 'cancelled') AS milestone_count,
				(SELECT COUNT(*) FROM contract_variations v WHERE v.contract_id=c.id AND v.status <> 'rejected') AS variation_count,
				(SELECT COALESCE(SUM(cr.outstanding_amount),0) FROM contract_retentions cr WHERE cr.contract_id=c.id AND cr.status <> 'forfeited') AS retention_outstanding,
				EXISTS (
					SELECT 1
					FROM contract_dispatches dispatch
					WHERE dispatch.contract_id=c.id
					  AND dispatch.contract_version_id=c.current_version_id
					  AND dispatch.status='received'
					  AND dispatch.returned_scan_document_id IS NOT NULL
				) AS signed_scan_received
		) summary ON true
		WHERE c.project_id=$1 AND c.status <> 'terminated'
		ORDER BY c.created_at DESC
	`, projectID)
}

func (r *Repository) GetContractDetail(ctx context.Context, contractID string) (Record, error) {
	contract, err := r.QueryOne(ctx, `
		SELECT c.*, p.tax_code AS counterparty_tax_code, p.partner_code AS counterparty_code,
			cv.version_no AS current_version_no, cv.version_label AS current_version_label, cv.status AS current_version_status
		FROM contracts c
		LEFT JOIN partners p ON p.id=c.counterparty_partner_id
		LEFT JOIN contract_versions cv ON cv.id=c.current_version_id
		WHERE c.id=$1 AND c.status <> 'terminated'
	`, contractID)
	if err != nil {
		return nil, err
	}
	versions, err := r.Query(ctx, `SELECT * FROM contract_versions WHERE contract_id=$1 ORDER BY version_no DESC`, contractID)
	if err != nil {
		return nil, err
	}
	signatures, err := r.Query(ctx, `
		SELECT link.id AS link_id,link.contract_version_id,link.purpose,link.linked_by,link.linked_at,
			signature.id AS workflow_form_signature_id,signature.form_instance_id,signature.signer_user_id,
			signature.signer_name,signature.signer_roles_json,signature.comment,signature.payload_hash,
			signature.signed_at,signature.visual_mime_type,signature.visual_file_size,
			wfi.title AS workflow_form_title
		FROM contract_version_signature_links link
		JOIN workflow_form_signatures signature ON signature.id=link.workflow_form_signature_id
		JOIN workflow_form_instances wfi ON wfi.id=signature.form_instance_id
		JOIN contract_versions version ON version.id=link.contract_version_id
		WHERE version.contract_id=$1
		ORDER BY signature.signed_at DESC,signature.id
	`, contractID)
	if err != nil {
		return nil, err
	}
	signingForms, err := r.Query(ctx, `
		SELECT wfi.id,wfi.title,wfi.status,wfi.current_revision,wfi.submitted_at,
			wi.id AS work_item_id,wi.title AS work_item_title,rs.business_code AS step_business_code,
			COALESCE((
				SELECT jsonb_agg(jsonb_build_object(
					'id',signature.id,
					'purpose',signature.purpose,
					'signer_user_id',signature.signer_user_id,
					'signer_name',signature.signer_name,
					'signed_at',signature.signed_at,
					'linked_contract_version_id',link.contract_version_id
				) ORDER BY signature.signed_at DESC)
				FROM workflow_form_signatures signature
				LEFT JOIN contract_version_signature_links link ON link.workflow_form_signature_id=signature.id
				WHERE signature.form_instance_id=wfi.id
			),'[]'::jsonb) AS signatures
		FROM contracts contract
		JOIN workflow_form_instances wfi ON wfi.project_id=contract.project_id AND wfi.status='submitted'
		JOIN work_items wi ON wi.id=wfi.work_item_id
		JOIN pipeline_run_steps rs ON rs.id=wfi.run_step_id
		WHERE contract.id=$1 AND rs.business_code='outbound_contract.app_sign'
		ORDER BY wfi.submitted_at DESC NULLS LAST,wfi.updated_at DESC,wfi.id
	`, contractID)
	if err != nil {
		return nil, err
	}
	dispatches, err := r.Query(ctx, `
		SELECT dispatch.*,
			sent.name AS sent_document_name,sent.mime_type AS sent_document_mime_type,
			returned.name AS returned_scan_document_name,returned.mime_type AS returned_scan_document_mime_type
		FROM contract_dispatches dispatch
		LEFT JOIN documents sent ON sent.id=dispatch.sent_document_id
		LEFT JOIN documents returned ON returned.id=dispatch.returned_scan_document_id
		WHERE dispatch.contract_id=$1
		ORDER BY dispatch.dispatch_no DESC
	`, contractID)
	if err != nil {
		return nil, err
	}
	milestones, err := r.Query(ctx, `
		SELECT cm.*, ar.title AS acceptance_title, ar.status AS acceptance_status
		FROM contract_milestones cm
		LEFT JOIN acceptance_records ar ON ar.id=cm.acceptance_id
		WHERE cm.contract_id=$1 AND cm.status <> 'cancelled'
		ORDER BY cm.sequence_no
	`, contractID)
	if err != nil {
		return nil, err
	}
	variations, err := r.Query(ctx, `
		SELECT variation.*,
			COALESCE((
				SELECT jsonb_agg(to_jsonb(event_row) ORDER BY event_row.created_at,event_row.id)
				FROM (
					SELECT event.*,COALESCE(actor.deleted_at IS NOT NULL,false) AS actor_account_deleted
					FROM contract_variation_events event
					LEFT JOIN users actor ON actor.id=event.actor_id
					WHERE event.variation_id=variation.id
				) event_row
			),'[]'::jsonb) AS events
		FROM contract_variations variation
		WHERE variation.contract_id=$1
		ORDER BY variation.created_at DESC
	`, contractID)
	if err != nil {
		return nil, err
	}
	retentions, err := r.Query(ctx, `
		SELECT cr.*, cm.milestone_code, cm.milestone_name,
			COALESCE((SELECT jsonb_agg(to_jsonb(rr) ORDER BY rr.release_no) FROM contract_retention_releases rr WHERE rr.retention_id=cr.id),'[]'::jsonb) AS releases
		FROM contract_retentions cr
		JOIN contract_milestones cm ON cm.id=cr.source_milestone_id
		WHERE cr.contract_id=$1
		ORDER BY cr.created_at
	`, contractID)
	if err != nil {
		return nil, err
	}
	contract["versions"] = versions
	contract["workflow_signatures"] = signatures
	contract["signing_forms"] = signingForms
	contract["dispatches"] = dispatches
	currentVersionID := cleanID(contract["current_version_id"])
	appSigned := false
	for _, signature := range signatures {
		if cleanID(signature["contract_version_id"]) == currentVersionID {
			appSigned = true
			break
		}
	}
	paperDispatched := false
	returnedScanReceived := false
	for _, dispatch := range dispatches {
		if cleanID(dispatch["contract_version_id"]) != currentVersionID {
			continue
		}
		paperDispatched = true
		if fmt.Sprint(dispatch["status"]) == "received" && cleanID(dispatch["returned_scan_document_id"]) != "" {
			returnedScanReceived = true
		}
	}
	contract["execution_readiness"] = Record{
		"app_signed":             appSigned,
		"paper_dispatched":       paperDispatched,
		"returned_scan_received": returnedScanReceived,
		"contract_active":        fmt.Sprint(contract["status"]) == "active",
	}
	contract["milestones"] = milestones
	contract["variations"] = variations
	contract["retentions"] = retentions
	return contract, nil
}

func (r *Repository) CreateProjectDocument(ctx context.Context, projectID, targetType, targetID, documentType, name, originalName string, file StoredFileMetadata, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	seriesLock := strings.Join([]string{"document_release", projectID, targetType, targetID, documentType}, ":")
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1)::bigint)`, seriesLock); err != nil {
		return nil, err
	}
	var revisionNo int
	if err := tx.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(revision_no),0)+1
		FROM documents
		WHERE project_id=$1 AND target_type=$2
		  AND target_id IS NOT DISTINCT FROM NULLIF($3,'')::uuid
		  AND document_type IS NOT DISTINCT FROM NULLIF($4,'')
	`, projectID, targetType, targetID, documentType).Scan(&revisionNo); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE documents
		SET release_status='superseded'
		WHERE project_id=$1 AND target_type=$2
		  AND target_id IS NOT DISTINCT FROM NULLIF($3,'')::uuid
		  AND document_type IS NOT DISTINCT FROM NULLIF($4,'')
		  AND release_status='released'
	`, projectID, targetType, targetID, documentType); err != nil {
		return nil, err
	}
	rows, err := queryTx(ctx, tx, `
		INSERT INTO documents (
			project_id,target_type,target_id,name,document_type,status,uploaded_by,
			mime_type,file_size,storage_provider,storage_key,checksum_sha256,
			revision_no,release_status,released_at,released_by
		)
		VALUES ($1,$2,NULLIF($3,'')::uuid,$4,NULLIF($5,''),'active',NULLIF($6,'')::uuid,$7,$8,$9,$10,$11,$12,'released',now(),NULLIF($6,'')::uuid)
		RETURNING *
	`, projectID, targetType, targetID, name, documentType, actorID, file.MIMEType, file.FileSize, file.StorageProvider, file.StorageKey, file.ChecksumSHA256, revisionNo)
	if err != nil {
		return nil, err
	}
	documentID := cleanID(rows[0]["id"])
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "project_document_uploaded", "document", documentID, "Tải hồ sơ công trình lên kho bảo vệ", Record{
		"target_type": targetType, "target_id": targetID, "document_type": documentType,
		"name": name, "original_filename": originalName, "checksum_sha256": file.ChecksumSHA256,
		"revision_no": revisionNo, "release_status": "released",
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return rows[0], nil
}

func (r *Repository) SaveContract(ctx context.Context, contractID string, body Record, mutationContext WorkItemMutationContext, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	projectID := cleanID(body["project_id"])
	eventType := "contract_created"
	if contractID == "" {
		rows, err := queryTx(ctx, tx, `
			INSERT INTO contracts (
				project_id, contract_code, title, contract_type, direction, counterparty_partner_id, counterparty_name,
				currency, original_amount, current_amount, advance_percent, retention_percent, warranty_days,
				signed_date, effective_date, expiry_date, status, created_by
			) VALUES ($1,$2,$3,$4,$5,NULLIF($6,'')::uuid,$7,$8,$9,$9,$10,$11,$12,NULLIF($13,'')::date,NULLIF($14,'')::date,NULLIF($15,'')::date,'draft',NULLIF($16,'')::uuid)
			RETURNING *
		`, projectID, body["contract_code"], body["title"], body["contract_type"], body["direction"], cleanID(body["counterparty_partner_id"]), body["counterparty_name"], body["currency"], body["original_amount"], body["advance_percent"], body["retention_percent"], body["warranty_days"], body["signed_date"], body["effective_date"], body["expiry_date"], actorID)
		if err != nil {
			return nil, err
		}
		if len(rows) == 0 {
			return nil, sql.ErrNoRows
		}
		contractID = cleanID(rows[0]["id"])
	} else {
		currentRows, err := queryTx(ctx, tx, `SELECT * FROM contracts WHERE id=$1 AND project_id=$2 FOR UPDATE`, contractID, projectID)
		if err != nil {
			return nil, err
		}
		if len(currentRows) == 0 {
			return nil, sql.ErrNoRows
		}
		if !statusIn(fmt.Sprint(currentRows[0]["status"]), "draft", "review") {
			return nil, validationError("Chỉ hợp đồng nháp/review mới được sửa thông tin gốc", Record{"status": currentRows[0]["status"]})
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE contracts SET contract_code=$3,title=$4,contract_type=$5,direction=$6,
				counterparty_partner_id=NULLIF($7,'')::uuid,counterparty_name=$8,currency=$9,
				original_amount=$10,current_amount=CASE WHEN current_version_id IS NULL THEN $10 ELSE current_amount END,
				advance_percent=$11,retention_percent=$12,warranty_days=$13,signed_date=NULLIF($14,'')::date,
				effective_date=NULLIF($15,'')::date,expiry_date=NULLIF($16,'')::date,updated_at=now()
			WHERE id=$1 AND project_id=$2
		`, contractID, projectID, body["contract_code"], body["title"], body["contract_type"], body["direction"], cleanID(body["counterparty_partner_id"]), body["counterparty_name"], body["currency"], body["original_amount"], body["advance_percent"], body["retention_percent"], body["warranty_days"], body["signed_date"], body["effective_date"], body["expiry_date"]); err != nil {
			return nil, err
		}
		eventType = "contract_updated"
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, eventType, "contract", contractID, "Lưu hồ sơ hợp đồng", Record{"contract_code": body["contract_code"], "contract_type": body["contract_type"], "direction": body["direction"]}); err != nil {
		return nil, err
	}
	transition, err := r.startWorkItemForDomainMutationTx(ctx, tx, projectID, mutationContext, contractWorkItemPolicyFor(body, "contract_saved"), "contract", contractID, actorID, actorName)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	result, err := r.GetContractDetail(ctx, contractID)
	return attachWorkItemTransition(result, transition), err
}

func (r *Repository) CreateContractVersion(ctx context.Context, contractID string, body Record, mutationContext WorkItemMutationContext, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	contracts, err := queryTx(ctx, tx, `SELECT * FROM contracts WHERE id=$1 AND status <> 'terminated' FOR UPDATE`, contractID)
	if err != nil {
		return nil, err
	}
	if len(contracts) == 0 {
		return nil, sql.ErrNoRows
	}
	contract := contracts[0]
	if statusIn(fmt.Sprint(contract["status"]), "active", "completed", "suspended") {
		return nil, validationError("Hợp đồng đang hiệu lực chỉ tạo version mới thông qua variation đã duyệt")
	}
	var nextVersion int
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(version_no),0)+1 FROM contract_versions WHERE contract_id=$1`, contractID).Scan(&nextVersion); err != nil {
		return nil, err
	}
	label := strings.TrimSpace(fmt.Sprint(body["version_label"]))
	if label == "" {
		label = fmt.Sprintf("V%d", nextVersion)
	}
	hashPayload, _ := json.Marshal(Record{"contract_id": contractID, "version_no": nextVersion, "amount": body["amount"], "scope_summary": body["scope_summary"], "commercial_terms": body["commercial_terms_json"], "payment_terms": body["payment_terms_json"], "planned_start_date": body["planned_start_date"], "planned_end_date": body["planned_end_date"], "document_id": body["document_id"]})
	hash := sha256.Sum256(hashPayload)
	rows, err := queryTx(ctx, tx, `
		INSERT INTO contract_versions (
			contract_id,version_no,version_label,amount,scope_summary,commercial_terms_json,payment_terms_json,
			planned_start_date,planned_end_date,document_id,definition_hash,status,review_comment,created_by
		) VALUES ($1,$2,$3,$4,$5,$6::jsonb,$7::jsonb,NULLIF($8,'')::date,NULLIF($9,'')::date,NULLIF($10,'')::uuid,$11,'draft',$12,NULLIF($13,'')::uuid)
		RETURNING *
	`, contractID, nextVersion, label, body["amount"], body["scope_summary"], mustJSON(body["commercial_terms_json"]), mustJSON(body["payment_terms_json"]), body["planned_start_date"], body["planned_end_date"], cleanID(body["document_id"]), hex.EncodeToString(hash[:]), body["comment"], actorID)
	if err != nil {
		return nil, err
	}
	versionID := cleanID(rows[0]["id"])
	if err := addAuditTx(ctx, tx, cleanID(contract["project_id"]), actorID, actorName, "contract_version_created", "contract_version", versionID, "Tạo phiên bản hợp đồng", Record{"contract_id": contractID, "version_no": nextVersion, "definition_hash": hex.EncodeToString(hash[:])}); err != nil {
		return nil, err
	}
	transition, err := r.startWorkItemForDomainMutationTx(ctx, tx, cleanID(contract["project_id"]), mutationContext, contractWorkItemPolicyFor(contract, "contract_version_created"), "contract_version", versionID, actorID, actorName)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	result, err := r.GetContractDetail(ctx, contractID)
	return attachWorkItemTransition(result, transition), err
}

func mustJSON(value any) string {
	raw, _ := json.Marshal(value)
	if string(raw) == "null" {
		return "{}"
	}
	return string(raw)
}

func (r *Repository) ContractVersionAction(ctx context.Context, versionID, action, comment string, mutationContext WorkItemMutationContext, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := queryTx(ctx, tx, `
		SELECT cv.*, c.project_id, c.status AS contract_status, c.approval_request_id, c.contract_type, c.direction
		FROM contract_versions cv JOIN contracts c ON c.id=cv.contract_id
		WHERE cv.id=$1 FOR UPDATE OF cv,c
	`, versionID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	version := rows[0]
	fromStatus := fmt.Sprint(version["status"])
	toStatus := ""
	eventType := ""
	switch action {
	case "submit":
		if fromStatus != "draft" {
			return nil, validationError("Chỉ version nháp mới được gửi review")
		}
		toStatus, eventType = "review", "contract_version_submitted"
	case "approve":
		if fromStatus != "review" {
			return nil, validationError("Chỉ version đang review mới được duyệt")
		}
		if cleanID(version["approval_request_id"]) != "" {
			var approvalStatus string
			if err := tx.QueryRowContext(ctx, `SELECT status FROM approval_requests WHERE id=$1`, version["approval_request_id"]).Scan(&approvalStatus); err != nil {
				return nil, err
			}
			if approvalStatus != "approved" {
				return nil, validationError("Approval cấu trúc của hợp đồng chưa hoàn tất", Record{"approval_status": approvalStatus})
			}
		}
		toStatus, eventType = "approved", "contract_version_approved"
	case "sign":
		if fromStatus != "approved" {
			return nil, validationError("Chỉ version đã duyệt mới được xác nhận ký")
		}
		toStatus, eventType = "signed", "contract_version_signed"
	default:
		return nil, validationError("Hành động version hợp đồng không hợp lệ", Record{"action": action})
	}
	_, err = tx.ExecContext(ctx, `
		UPDATE contract_versions SET status=$2,review_comment=CASE WHEN $3 IN ('submit','approve') THEN concat_ws(E'\n',NULLIF(review_comment,''),NULLIF($4,'')) ELSE review_comment END,
			signature_comment=CASE WHEN $3='sign' THEN $4 ELSE signature_comment END,
			submitted_by=CASE WHEN $3='submit' THEN NULLIF($5,'')::uuid ELSE submitted_by END,submitted_at=CASE WHEN $3='submit' THEN now() ELSE submitted_at END,
			approved_by=CASE WHEN $3='approve' THEN NULLIF($5,'')::uuid ELSE approved_by END,approved_at=CASE WHEN $3='approve' THEN now() ELSE approved_at END,
			signed_by=CASE WHEN $3='sign' THEN NULLIF($5,'')::uuid ELSE signed_by END,signed_at=CASE WHEN $3='sign' THEN now() ELSE signed_at END,updated_at=now()
		WHERE id=$1
	`, versionID, toStatus, action, comment, actorID)
	if err != nil {
		return nil, err
	}
	contractStatus := map[string]string{"submit": "review", "approve": "approved", "sign": "signed"}[action]
	_, err = tx.ExecContext(ctx, `
		UPDATE contracts SET status=$2,
			current_version_id=CASE WHEN $3='sign' THEN $4::uuid ELSE current_version_id END,
			current_amount=CASE WHEN $3='sign' THEN $5 ELSE current_amount END,
			approved_by=CASE WHEN $3='approve' THEN NULLIF($6,'')::uuid ELSE approved_by END,
			signed_by=CASE WHEN $3='sign' THEN NULLIF($6,'')::uuid ELSE signed_by END,
			signed_date=CASE WHEN $3='sign' THEN COALESCE(signed_date,CURRENT_DATE) ELSE signed_date END,
			updated_at=now()
		WHERE id=$1
	`, version["contract_id"], contractStatus, action, versionID, version["amount"], actorID)
	if err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, cleanID(version["project_id"]), actorID, actorName, eventType, "contract_version", versionID, fmt.Sprintf("Chuyển version hợp đồng từ %s sang %s", fromStatus, toStatus), Record{"contract_id": version["contract_id"], "version_no": version["version_no"], "comment": comment}); err != nil {
		return nil, err
	}
	transition, err := r.startWorkItemForDomainMutationTx(ctx, tx, cleanID(version["project_id"]), mutationContext, contractWorkItemPolicyFor(version, "contract_version_"+action), "contract_version", versionID, actorID, actorName)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	result, err := r.GetContractDetail(ctx, cleanID(version["contract_id"]))
	return attachWorkItemTransition(result, transition), err
}

func (r *Repository) SignContractVersionWithWorkflowSignature(ctx context.Context, versionID, signatureID, comment string, mutationContext WorkItemMutationContext, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	versions, err := queryTx(ctx, tx, `
		SELECT cv.*,c.project_id,c.status AS contract_status,c.contract_type,c.direction
		FROM contract_versions cv
		JOIN contracts c ON c.id=cv.contract_id
		WHERE cv.id=$1
		FOR UPDATE OF cv,c
	`, versionID)
	if err != nil {
		return nil, err
	}
	if len(versions) == 0 {
		return nil, sql.ErrNoRows
	}
	version := versions[0]
	if fmt.Sprint(version["status"]) != "approved" {
		return nil, validationError("Chỉ version đã duyệt mới được ký trên App", Record{"status": version["status"]})
	}
	signatures, err := queryTx(ctx, tx, `
		SELECT signature.*,wfi.project_id AS form_project_id,rs.business_code AS step_business_code
		FROM workflow_form_signatures signature
		JOIN workflow_form_instances wfi ON wfi.id=signature.form_instance_id
		JOIN pipeline_run_steps rs ON rs.id=wfi.run_step_id
		WHERE signature.id=$1
		FOR SHARE OF signature,wfi,rs
	`, signatureID)
	if err != nil {
		return nil, err
	}
	if len(signatures) == 0 {
		return nil, validationError("Không tìm thấy chữ ký workflow đã chọn")
	}
	signature := signatures[0]
	if cleanID(signature["form_project_id"]) != cleanID(version["project_id"]) {
		return nil, validationError("Chữ ký workflow không thuộc cùng công trình với hợp đồng")
	}
	if fmt.Sprint(signature["purpose"]) != "approved_by" || fmt.Sprint(signature["step_business_code"]) != "outbound_contract.app_sign" {
		return nil, validationError("Chữ ký không thuộc bước ký hợp đồng trên App", Record{"purpose": signature["purpose"], "step_business_code": signature["step_business_code"]})
	}
	if cleanID(signature["signer_user_id"]) != actorID {
		return nil, forbiddenError("Chỉ người vừa ký workflow mới được xác nhận ký version hợp đồng")
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO contract_version_signature_links (contract_version_id,workflow_form_signature_id,purpose,linked_by)
		VALUES ($1,$2,'approved_by',NULLIF($3,'')::uuid)
	`, versionID, signatureID, actorID); err != nil {
		return nil, validationError("Chữ ký workflow đã được liên kết hoặc version đã có chữ ký", Record{"cause": err.Error()})
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE contract_versions
		SET status='signed',signature_comment=$2,signed_by=NULLIF($3,'')::uuid,signed_at=now(),updated_at=now()
		WHERE id=$1
	`, versionID, comment, actorID); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE contracts
		SET status='signed',current_version_id=$2,current_amount=$3,signed_by=NULLIF($4,'')::uuid,
			signed_date=COALESCE(signed_date,CURRENT_DATE),updated_at=now()
		WHERE id=$1
	`, version["contract_id"], versionID, version["amount"], actorID); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, cleanID(version["project_id"]), actorID, actorName, "contract_version_signed_on_app", "contract_version", versionID, "Ký version hợp đồng trên App", Record{
		"contract_id": version["contract_id"], "workflow_form_signature_id": signatureID, "signature_payload_hash": signature["payload_hash"], "comment": comment,
	}); err != nil {
		return nil, err
	}
	transition, err := r.startWorkItemForDomainMutationTx(ctx, tx, cleanID(version["project_id"]), mutationContext, contractWorkItemPolicyFor(version, "contract_version_signed"), "contract_version", versionID, actorID, actorName)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	result, err := r.GetContractDetail(ctx, cleanID(version["contract_id"]))
	return attachWorkItemTransition(result, transition), err
}

func (r *Repository) CreateContractDispatch(ctx context.Context, contractID string, body Record, mutationContext WorkItemMutationContext, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	contracts, err := queryTx(ctx, tx, `
		SELECT c.*,cv.status AS version_status,
			EXISTS (SELECT 1 FROM contract_version_signature_links link WHERE link.contract_version_id=cv.id) AS app_signed
		FROM contracts c
		JOIN contract_versions cv ON cv.id=c.current_version_id
		WHERE c.id=$1 FOR UPDATE OF c
	`, contractID)
	if err != nil {
		return nil, err
	}
	if len(contracts) == 0 {
		return nil, sql.ErrNoRows
	}
	contract := contracts[0]
	versionID := cleanID(body["contract_version_id"])
	if versionID == "" {
		versionID = cleanID(contract["current_version_id"])
	}
	if versionID != cleanID(contract["current_version_id"]) || fmt.Sprint(contract["version_status"]) != "signed" || !boolValue(contract["app_signed"]) {
		return nil, validationError("Chỉ version hiện hành đã ký trên App mới được gửi hồ sơ giấy")
	}
	if sentDocumentID := cleanID(body["sent_document_id"]); sentDocumentID != "" {
		if err := validateProjectDocumentTx(ctx, tx, sentDocumentID, cleanID(contract["project_id"])); err != nil {
			return nil, err
		}
	}
	var dispatchNo int
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(dispatch_no),0)+1 FROM contract_dispatches WHERE contract_id=$1`, contractID).Scan(&dispatchNo); err != nil {
		return nil, err
	}
	rows, err := queryTx(ctx, tx, `
		INSERT INTO contract_dispatches (
			contract_id,contract_version_id,dispatch_no,recipient_name,delivery_method,
			tracking_reference,sent_document_id,status,sent_by,note
		)
		VALUES ($1,$2,$3,$4,$5,NULLIF($6,''),NULLIF($7,'')::uuid,'sent',NULLIF($8,'')::uuid,NULLIF($9,''))
		RETURNING *
	`, contractID, versionID, dispatchNo, body["recipient_name"], body["delivery_method"], body["tracking_reference"], cleanID(body["sent_document_id"]), actorID, body["note"])
	if err != nil {
		return nil, err
	}
	dispatchID := cleanID(rows[0]["id"])
	if err := addAuditTx(ctx, tx, cleanID(contract["project_id"]), actorID, actorName, "contract_paper_dispatched", "contract_dispatch", dispatchID, "Gửi hồ sơ hợp đồng giấy cho chủ đầu tư", Record{
		"contract_id": contractID, "contract_version_id": versionID, "dispatch_no": dispatchNo, "recipient_name": body["recipient_name"], "tracking_reference": body["tracking_reference"],
	}); err != nil {
		return nil, err
	}
	transition, err := r.startWorkItemForDomainMutationTx(ctx, tx, cleanID(contract["project_id"]), mutationContext, contractWorkItemPolicyFor(contract, "contract_paper_dispatched"), "contract_dispatch", dispatchID, actorID, actorName)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	result, err := r.GetContractDetail(ctx, contractID)
	return attachWorkItemTransition(result, transition), err
}

func validateProjectDocumentTx(ctx context.Context, tx *sql.Tx, documentID, projectID string) error {
	var total int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM documents WHERE id=$1 AND project_id=$2 AND status='active'`, documentID, projectID).Scan(&total); err != nil {
		return err
	}
	if total == 0 {
		return validationError("Hồ sơ đính kèm không thuộc công trình hoặc không còn hiệu lực", Record{"document_id": documentID})
	}
	return nil
}

func (r *Repository) ReceiveContractDispatch(ctx context.Context, dispatchID, scanDocumentID, note string, mutationContext WorkItemMutationContext, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := queryTx(ctx, tx, `
		SELECT dispatch.*,contract.project_id,contract.contract_type,contract.direction
		FROM contract_dispatches dispatch
		JOIN contracts contract ON contract.id=dispatch.contract_id
		WHERE dispatch.id=$1 FOR UPDATE OF dispatch
	`, dispatchID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	dispatch := rows[0]
	if fmt.Sprint(dispatch["status"]) != "sent" {
		return nil, validationError("Hồ sơ giấy đã được ghi nhận nhận lại", Record{"status": dispatch["status"]})
	}
	if err := validateProjectDocumentTx(ctx, tx, scanDocumentID, cleanID(dispatch["project_id"])); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE contract_dispatches
		SET status='received',returned_scan_document_id=$2,received_by=NULLIF($3,'')::uuid,
			received_at=now(),note=concat_ws(E'\n',NULLIF(note,''),NULLIF($4,'')),updated_at=now()
		WHERE id=$1
	`, dispatchID, scanDocumentID, actorID, note); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, cleanID(dispatch["project_id"]), actorID, actorName, "contract_signed_scan_received", "contract_dispatch", dispatchID, "Nhận lại và lưu bản scan hợp đồng giấy đã ký", Record{
		"contract_id": dispatch["contract_id"], "contract_version_id": dispatch["contract_version_id"], "returned_scan_document_id": scanDocumentID, "note": note,
	}); err != nil {
		return nil, err
	}
	transition, err := r.startWorkItemForDomainMutationTx(ctx, tx, cleanID(dispatch["project_id"]), mutationContext, contractWorkItemPolicyFor(dispatch, "contract_signed_scan_received"), "contract_dispatch", dispatchID, actorID, actorName)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	result, err := r.GetContractDetail(ctx, cleanID(dispatch["contract_id"]))
	return attachWorkItemTransition(result, transition), err
}

func (r *Repository) ActivateContract(ctx context.Context, contractID, comment string, mutationContext WorkItemMutationContext, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := queryTx(ctx, tx, `
		SELECT c.*, cv.status AS version_status, cv.amount AS version_amount,
			EXISTS (
				SELECT 1 FROM contract_version_signature_links link
				WHERE link.contract_version_id=cv.id
			) AS app_signed,
			EXISTS (
				SELECT 1 FROM contract_dispatches dispatch
				WHERE dispatch.contract_id=c.id AND dispatch.contract_version_id=cv.id
				  AND dispatch.status='received' AND dispatch.returned_scan_document_id IS NOT NULL
			) AS returned_scan_received
		FROM contracts c LEFT JOIN contract_versions cv ON cv.id=c.current_version_id
		WHERE c.id=$1 FOR UPDATE OF c
	`, contractID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	contract := rows[0]
	if fmt.Sprint(contract["status"]) != "signed" || fmt.Sprint(contract["version_status"]) != "signed" {
		return nil, validationError("Chỉ hợp đồng có version đã ký mới được kích hoạt")
	}
	if !boolValue(contract["app_signed"]) {
		return nil, validationError("Cần chữ ký approved_by trên App trước khi kích hoạt hợp đồng")
	}
	if !boolValue(contract["returned_scan_received"]) {
		return nil, validationError("Cần nhận lại và lưu bản scan hợp đồng giấy đã ký trước khi kích hoạt")
	}
	if cleanID(contract["counterparty_partner_id"]) == "" || dateOnly(contract["effective_date"]) == "" {
		return nil, validationError("Kích hoạt hợp đồng cần đối tác catalog và ngày hiệu lực")
	}
	if _, err := tx.ExecContext(ctx, `UPDATE contracts SET status='active',activated_by=NULLIF($2,'')::uuid,activated_at=now(),updated_at=now() WHERE id=$1`, contractID, actorID); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE contract_milestones SET status='active',updated_at=now() WHERE contract_id=$1 AND status='draft'`, contractID); err != nil {
		return nil, err
	}
	if fmt.Sprint(contract["contract_type"]) == "main_client" {
		if _, err := tx.ExecContext(ctx, `UPDATE projects SET contract_amount=$2,updated_at=now() WHERE id=$1`, contract["project_id"], contract["version_amount"]); err != nil {
			return nil, err
		}
	}
	if err := addAuditTx(ctx, tx, cleanID(contract["project_id"]), actorID, actorName, "contract_activated", "contract", contractID, "Kích hoạt hợp đồng đã ký", Record{"current_version_id": contract["current_version_id"], "current_amount": contract["version_amount"], "comment": comment}); err != nil {
		return nil, err
	}
	transition, err := r.startWorkItemForDomainMutationTx(ctx, tx, cleanID(contract["project_id"]), mutationContext, contractWorkItemPolicyFor(contract, "contract_activated"), "contract", contractID, actorID, actorName)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	result, err := r.GetContractDetail(ctx, contractID)
	return attachWorkItemTransition(result, transition), err
}

func (r *Repository) CreateContractMilestone(ctx context.Context, contractID string, body Record, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := queryTx(ctx, tx, `SELECT * FROM contracts WHERE id=$1 AND status IN ('signed','active') FOR UPDATE`, contractID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, validationError("Cần ký hợp đồng trước khi lập milestone")
	}
	contract := rows[0]
	if cleanID(contract["current_version_id"]) == "" {
		return nil, validationError("Hợp đồng chưa có version hiện hành")
	}
	gross := number(body["gross_amount"])
	percentage := number(body["percentage"])
	if fmt.Sprint(body["amount_basis"]) == "percentage" {
		if percentage <= 0 {
			return nil, validationError("Milestone theo tỷ lệ cần phần trăm lớn hơn 0")
		}
		gross = math.Round(number(contract["current_amount"])*percentage) / 100
	}
	if gross <= 0 {
		return nil, validationError("Giá trị milestone phải lớn hơn 0")
	}
	retentionPercent := number(body["retention_percent"])
	retentionAmount := math.Round(gross*retentionPercent) / 100
	netAmount := gross - retentionAmount
	status := "draft"
	if fmt.Sprint(contract["status"]) == "active" {
		status = "active"
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO contract_milestones (
			contract_id,contract_version_id,milestone_code,milestone_name,sequence_no,milestone_type,amount_basis,
			percentage,gross_amount,retention_percent,retention_amount,net_amount,planned_date,due_days,
			acceptance_required,acceptance_id,status,created_by
		) VALUES ($1,$2,$3,$4,$5,$6,$7,NULLIF($8,0),$9,$10,$11,$12,NULLIF($13,'')::date,$14,$15,NULLIF($16,'')::uuid,$17,NULLIF($18,'')::uuid)
	`, contractID, contract["current_version_id"], body["milestone_code"], body["milestone_name"], body["sequence_no"], body["milestone_type"], body["amount_basis"], percentage, gross, retentionPercent, retentionAmount, netAmount, body["planned_date"], body["due_days"], body["acceptance_required"], cleanID(body["acceptance_id"]), status, actorID)
	if err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, cleanID(contract["project_id"]), actorID, actorName, "contract_milestone_created", "contract", contractID, "Tạo milestone hợp đồng", Record{"milestone_code": body["milestone_code"], "gross_amount": gross, "retention_amount": retentionAmount}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetContractDetail(ctx, contractID)
}

func (r *Repository) ContractMilestoneAction(ctx context.Context, milestoneID, action, comment, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := queryTx(ctx, tx, `
		SELECT cm.*,c.project_id,c.status AS contract_status,c.direction,c.contract_code,c.contract_type,
			c.counterparty_partner_id,c.counterparty_name,c.current_amount,c.warranty_days
		FROM contract_milestones cm JOIN contracts c ON c.id=cm.contract_id
		WHERE cm.id=$1 FOR UPDATE OF cm,c
	`, milestoneID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	milestone := rows[0]
	if fmt.Sprint(milestone["contract_status"]) != "active" {
		return nil, validationError("Milestone chỉ thao tác khi hợp đồng đang hiệu lực")
	}
	if action == "eligible" {
		if fmt.Sprint(milestone["status"]) != "active" {
			return nil, validationError("Chỉ milestone active mới được xác nhận đủ điều kiện")
		}
		if boolValue(milestone["acceptance_required"]) {
			if cleanID(milestone["acceptance_id"]) == "" {
				return nil, validationError("Milestone yêu cầu nghiệm thu nhưng chưa liên kết hồ sơ nghiệm thu")
			}
			var acceptanceStatus string
			var result any
			if err := tx.QueryRowContext(ctx, `SELECT status,result FROM acceptance_records WHERE id=$1 AND project_id=$2`, milestone["acceptance_id"], milestone["project_id"]).Scan(&acceptanceStatus, &result); err != nil {
				return nil, err
			}
			if acceptanceStatus != "approved" || !statusIn(fmt.Sprint(result), "pass", "conditional_pass") {
				return nil, validationError("Hồ sơ nghiệm thu chưa đạt điều kiện milestone", Record{"acceptance_status": acceptanceStatus, "result": result})
			}
		}
		if _, err := tx.ExecContext(ctx, `UPDATE contract_milestones SET status='eligible',eligible_by=NULLIF($2,'')::uuid,eligible_at=now(),updated_at=now() WHERE id=$1`, milestoneID, actorID); err != nil {
			return nil, err
		}
		if err := addAuditTx(ctx, tx, cleanID(milestone["project_id"]), actorID, actorName, "contract_milestone_eligible", "contract_milestone", milestoneID, "Xác nhận milestone đủ điều kiện", Record{"contract_id": milestone["contract_id"], "comment": comment}); err != nil {
			return nil, err
		}
	} else if action == "open-obligation" {
		if fmt.Sprint(milestone["status"]) == "obligation_opened" && cleanID(milestone["obligation_id"]) != "" {
			if err := tx.Commit(); err != nil {
				return nil, err
			}
			return r.GetContractDetail(ctx, cleanID(milestone["contract_id"]))
		}
		if fmt.Sprint(milestone["status"]) != "eligible" {
			return nil, validationError("Milestone phải đủ điều kiện trước khi mở công nợ")
		}
		dueDate := time.Now().AddDate(0, 0, int(number(milestone["due_days"]))).Format("2006-01-02")
		if planned := dateOnly(milestone["planned_date"]); planned != "" {
			base, _ := time.Parse("2006-01-02", planned)
			dueDate = base.AddDate(0, 0, int(number(milestone["due_days"]))).Format("2006-01-02")
		}
		obligationType := fmt.Sprint(milestone["direction"])
		obligationID := ""
		codeSuffix := strings.ToUpper(strings.ReplaceAll(fmt.Sprint(milestone["milestone_code"]), " ", "-"))
		if obligationType == "receivable" {
			obligationRows, err := queryTx(ctx, tx, `
				INSERT INTO receivables (project_id,receivable_code,client_partner_id,client_name,source_type,source_id,contract_amount,milestone_name,gross_amount,received_amount,remaining_amount,due_date,status,created_by)
				VALUES ($1,$2,NULLIF($3,'')::uuid,$4,'contract_milestone',$5,$6,$7,$8,0,$8,$9,'draft',NULLIF($10,'')::uuid)
				RETURNING id
			`, milestone["project_id"], fmt.Sprintf("AR-%s-%s", cleanID(milestone["contract_id"])[:8], codeSuffix), cleanID(milestone["counterparty_partner_id"]), milestone["counterparty_name"], milestoneID, milestone["current_amount"], milestone["milestone_name"], milestone["net_amount"], dueDate, actorID)
			if err != nil {
				return nil, err
			}
			obligationID = cleanID(obligationRows[0]["id"])
		} else {
			vendorType := fmt.Sprint(milestone["contract_type"])
			if vendorType == "main_client" {
				vendorType = "other"
			}
			obligationRows, err := queryTx(ctx, tx, `
				INSERT INTO payables (project_id,payable_code,vendor_type,vendor_partner_id,vendor_name,source_type,source_id,gross_amount,advance_amount,retention_amount,deduction_amount,paid_amount,remaining_amount,due_date,status,created_by)
				VALUES ($1,$2,$3,NULLIF($4,'')::uuid,$5,'contract_milestone',$6,$7,0,$8,0,0,$9,$10,'draft',NULLIF($11,'')::uuid)
				RETURNING id
			`, milestone["project_id"], fmt.Sprintf("AP-%s-%s", cleanID(milestone["contract_id"])[:8], codeSuffix), vendorType, cleanID(milestone["counterparty_partner_id"]), milestone["counterparty_name"], milestoneID, milestone["gross_amount"], milestone["retention_amount"], milestone["net_amount"], dueDate, actorID)
			if err != nil {
				return nil, err
			}
			obligationID = cleanID(obligationRows[0]["id"])
		}
		if _, err := tx.ExecContext(ctx, `UPDATE contract_milestones SET status='obligation_opened',obligation_type=$2,obligation_id=$3,updated_at=now() WHERE id=$1`, milestoneID, obligationType, obligationID); err != nil {
			return nil, err
		}
		if number(milestone["retention_amount"]) > 0 {
			warrantyEnd := ""
			if dateOnly(milestone["planned_date"]) != "" {
				base, _ := time.Parse("2006-01-02", dateOnly(milestone["planned_date"]))
				warrantyEnd = base.AddDate(0, 0, int(number(milestone["warranty_days"]))).Format("2006-01-02")
			}
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO contract_retentions (contract_id,source_milestone_id,direction,held_amount,warranty_end_date,status)
				VALUES ($1,$2,$3,$4,NULLIF($5,'')::date,'held') ON CONFLICT (source_milestone_id) DO NOTHING
			`, milestone["contract_id"], milestoneID, obligationType, milestone["retention_amount"], warrantyEnd); err != nil {
				return nil, err
			}
		}
		if err := addAuditTx(ctx, tx, cleanID(milestone["project_id"]), actorID, actorName, "contract_milestone_obligation_opened", "contract_milestone", milestoneID, "Mở công nợ từ milestone hợp đồng", Record{"contract_id": milestone["contract_id"], "obligation_type": obligationType, "obligation_id": obligationID, "net_amount": milestone["net_amount"], "retention_amount": milestone["retention_amount"], "comment": comment}); err != nil {
			return nil, err
		}
	} else {
		return nil, validationError("Hành động milestone không hợp lệ", Record{"action": action})
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetContractDetail(ctx, cleanID(milestone["contract_id"]))
}

func (r *Repository) CreateContractVariation(ctx context.Context, contractID string, body Record, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := queryTx(ctx, tx, `SELECT * FROM contracts WHERE id=$1 AND status IN ('active','suspended') FOR UPDATE`, contractID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, validationError("Chỉ hợp đồng đang hiệu lực/tạm dừng mới được lập variation")
	}
	contract := rows[0]
	if sourceWorkItemID := cleanID(body["source_work_item_id"]); sourceWorkItemID != "" {
		var validSource bool
		if err := tx.QueryRowContext(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM work_items item
				LEFT JOIN pipeline_run_steps step ON step.id=item.run_step_id
				LEFT JOIN pipeline_run_nodes node ON node.id=item.run_node_id
				WHERE item.id=$1 AND item.project_id=$2
				  AND (step.business_code='construction.item_reporting' OR node.business_code='construction_execution')
			)
		`, sourceWorkItemID, contract["project_id"]).Scan(&validSource); err != nil {
			return nil, err
		}
		if !validSource {
			return nil, validationError("Đầu việc nguồn không thuộc bước thi công của công trình")
		}
	}
	variationRows, err := queryTx(ctx, tx, `
		INSERT INTO contract_variations (
			contract_id,variation_code,title,variation_type,reason,amount_delta,days_delta,scope_delta,
			document_id,status,priority,source_work_item_id,created_by
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,NULLIF($9,'')::uuid,'draft',$10,NULLIF($11,'')::uuid,NULLIF($12,'')::uuid)
		RETURNING *
	`, contractID, body["variation_code"], body["title"], body["variation_type"], body["reason"], body["amount_delta"], body["days_delta"], body["scope_delta"], cleanID(body["document_id"]), body["priority"], cleanID(body["source_work_item_id"]), actorID)
	if err != nil {
		return nil, err
	}
	if err := r.recordContractVariationEventTx(ctx, tx, variationRows[0], "contract_variation_created", "", "draft", body["comment"], []string{"QS"}, actorID, actorName); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, cleanID(contract["project_id"]), actorID, actorName, "contract_variation_created", "contract", contractID, "Tạo phụ lục/variation", Record{"variation_code": body["variation_code"], "amount_delta": body["amount_delta"], "days_delta": body["days_delta"]}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetContractDetail(ctx, contractID)
}

func (r *Repository) ProjectConstructionVariationContext(ctx context.Context, projectID string) (Record, error) {
	contracts, err := r.Query(ctx, `
		SELECT id,contract_code,title,contract_type,direction,counterparty_name,current_amount,status
		FROM contracts
		WHERE project_id=$1 AND contract_type='main_client' AND direction='receivable' AND status='active'
		ORDER BY updated_at DESC,id DESC
		LIMIT 1
	`, projectID)
	if err != nil {
		return nil, err
	}
	if len(contracts) == 0 {
		return Record{"available": false, "contract": nil, "variations": []Record{}}, nil
	}
	contract := contracts[0]
	variations, err := r.Query(ctx, `
		SELECT variation.id,variation.variation_code,variation.title,variation.variation_type,
			variation.reason,variation.amount_delta,variation.days_delta,variation.scope_delta,
			variation.status,variation.priority,variation.source_work_item_id,
			variation.source_construction_action_id,variation.created_at,variation.updated_at,
			item.title AS source_work_item_title,action.title AS source_construction_action_title
		FROM contract_variations variation
		LEFT JOIN work_items item ON item.id=variation.source_work_item_id
		LEFT JOIN construction_action_items action ON action.id=variation.source_construction_action_id
		WHERE variation.contract_id=$1
		ORDER BY variation.created_at DESC,variation.id DESC
	`, contract["id"])
	if err != nil {
		return nil, err
	}
	return Record{"available": true, "contract": contract, "variations": variations}, nil
}

func (r *Repository) ProposeConstructionVariation(ctx context.Context, projectID string, body Record, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	contracts, err := queryTx(ctx, tx, `
		SELECT *
		FROM contracts
		WHERE project_id=$1 AND contract_type='main_client' AND direction='receivable' AND status='active'
		ORDER BY updated_at DESC,id DESC
		LIMIT 1
		FOR UPDATE
	`, projectID)
	if err != nil {
		return nil, err
	}
	if len(contracts) == 0 {
		return nil, validationError("Công trình chưa có hợp đồng chủ đầu tư đang hiệu lực")
	}
	contract := contracts[0]
	workItemID := cleanID(body["source_work_item_id"])
	var validWorkItem bool
	if err := tx.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM work_items
			WHERE id=$1 AND project_id=$2 AND status <> 'cancelled'
		)
	`, workItemID, projectID).Scan(&validWorkItem); err != nil {
		return nil, err
	}
	if !validWorkItem {
		return nil, validationError("Hạng mục thi công nguồn không thuộc công trình")
	}
	actionID := cleanID(body["source_construction_action_id"])
	if actionID != "" {
		actions, err := queryTx(ctx, tx, `
			SELECT id,work_item_id,impact_scope
			FROM construction_action_items
			WHERE id=$1 AND project_id=$2 AND status <> 'cancelled'
			FOR UPDATE
		`, actionID, projectID)
		if err != nil {
			return nil, err
		}
		if len(actions) == 0 || cleanID(actions[0]["work_item_id"]) != workItemID {
			return nil, validationError("Việc cần xử lý nguồn không thuộc hạng mục thi công đã chọn")
		}
		if fmt.Sprint(actions[0]["impact_scope"]) == "none" {
			return nil, validationError("Chỉ việc có ảnh hưởng tiến độ, chi phí hoặc phạm vi mới cần đề xuất phát sinh")
		}
		var duplicate bool
		if err := tx.QueryRowContext(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM contract_variations
				WHERE source_construction_action_id=$1
				  AND status NOT IN ('rejected','incorporated')
			)
		`, actionID).Scan(&duplicate); err != nil {
			return nil, err
		}
		if duplicate {
			return nil, validationError("Việc cần xử lý đã có một đề xuất phát sinh đang được xử lý")
		}
	}
	variationRows, err := queryTx(ctx, tx, `
		INSERT INTO contract_variations (
			contract_id,variation_code,title,variation_type,reason,amount_delta,days_delta,scope_delta,
			document_id,status,priority,source_work_item_id,source_construction_action_id,created_by
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,NULLIF($9,'')::uuid,'draft',$10,$11,NULLIF($12,'')::uuid,NULLIF($13,'')::uuid)
		RETURNING *
	`, contract["id"], body["variation_code"], body["title"], body["variation_type"], body["reason"], body["amount_delta"], body["days_delta"], body["scope_delta"], cleanID(body["document_id"]), body["priority"], workItemID, actionID, actorID)
	if err != nil {
		return nil, err
	}
	variation := variationRows[0]
	variation["project_id"] = projectID
	if err := r.recordContractVariationEventTx(ctx, tx, variation, "contract_variation_created", "", "draft", body["comment"], nil, actorID, actorName); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE contract_variations
		SET status='qs_review',submitted_by=NULLIF($2,'')::uuid,submitted_at=now(),updated_at=now()
		WHERE id=$1
	`, variation["id"], actorID); err != nil {
		return nil, err
	}
	variation["status"] = "qs_review"
	if err := r.recordContractVariationEventTx(ctx, tx, variation, "contract_variation_submitted", "draft", "qs_review", body["comment"], []string{"QS"}, actorID, actorName); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "contract_variation_proposed", "contract_variation", cleanID(variation["id"]), "Đề xuất phát sinh từ hiện trường và gửi QS", Record{
		"contract_id": contract["id"], "source_work_item_id": workItemID, "source_construction_action_id": actionID,
		"priority": body["priority"], "amount_delta": body["amount_delta"], "days_delta": body["days_delta"],
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.QueryOne(ctx, `
		SELECT variation.*,item.title AS source_work_item_title,action.title AS source_construction_action_title
		FROM contract_variations variation
		LEFT JOIN work_items item ON item.id=variation.source_work_item_id
		LEFT JOIN construction_action_items action ON action.id=variation.source_construction_action_id
		WHERE variation.id=$1
	`, variation["id"])
}

func (r *Repository) ContractVariationAction(ctx context.Context, variationID, action, comment, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := queryTx(ctx, tx, `
		SELECT v.*,c.project_id,c.current_version_id,c.current_amount,c.status AS contract_status,
			cv.scope_summary,cv.commercial_terms_json,cv.payment_terms_json,cv.planned_start_date,cv.planned_end_date,cv.document_id
		FROM contract_variations v JOIN contracts c ON c.id=v.contract_id
		LEFT JOIN contract_versions cv ON cv.id=c.current_version_id
		WHERE v.id=$1 FOR UPDATE OF v,c
	`, variationID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	variation := rows[0]
	fromStatus := fmt.Sprint(variation["status"])
	toStatus := ""
	eventType := ""
	switch action {
	case "submit":
		if fromStatus != "draft" {
			return nil, validationError("Chỉ phát sinh nháp mới được gửi QS rà soát")
		}
		toStatus, eventType = "qs_review", "contract_variation_submitted"
		_, err = tx.ExecContext(ctx, `UPDATE contract_variations SET status='qs_review',submitted_by=NULLIF($2,'')::uuid,submitted_at=now(),updated_at=now() WHERE id=$1`, variationID, actorID)
	case "qs-review":
		if fromStatus != "qs_review" {
			return nil, validationError("Phát sinh chưa ở bước QS rà soát")
		}
		toStatus, eventType = "purchasing_sourcing", "contract_variation_qs_reviewed"
		_, err = tx.ExecContext(ctx, `UPDATE contract_variations SET status='purchasing_sourcing',qs_reviewed_by=NULLIF($2,'')::uuid,qs_reviewed_at=now(),qs_comment=$3,updated_at=now() WHERE id=$1`, variationID, actorID, comment)
	case "purchase":
		if fromStatus != "purchasing_sourcing" {
			return nil, validationError("Phát sinh chưa ở bước Mua hàng lấy giá")
		}
		toStatus, eventType = "pending_approval", "contract_variation_purchasing_completed"
		_, err = tx.ExecContext(ctx, `UPDATE contract_variations SET status='pending_approval',purchasing_reviewed_by=NULLIF($2,'')::uuid,purchasing_reviewed_at=now(),purchasing_comment=$3,updated_at=now() WHERE id=$1`, variationID, actorID, comment)
	case "approve":
		if !statusIn(fromStatus, "pending_approval", "review") {
			return nil, validationError("Phát sinh phải qua QS và Mua hàng trước khi duyệt")
		}
		if cleanID(variation["approval_request_id"]) != "" {
			var status string
			if err := tx.QueryRowContext(ctx, `SELECT status FROM approval_requests WHERE id=$1`, variation["approval_request_id"]).Scan(&status); err != nil {
				return nil, err
			}
			if status != "approved" {
				return nil, validationError("Approval cấu trúc của variation chưa hoàn tất", Record{"approval_status": status})
			}
		}
		toStatus, eventType = "approved", "contract_variation_approved"
		_, err = tx.ExecContext(ctx, `UPDATE contract_variations SET status='approved',approved_by=NULLIF($2,'')::uuid,approved_at=now(),updated_at=now() WHERE id=$1`, variationID, actorID)
	case "reject":
		if !statusIn(fromStatus, "qs_review", "purchasing_sourcing", "pending_approval", "review") {
			return nil, validationError("Trạng thái phát sinh không cho phép từ chối")
		}
		toStatus, eventType = "rejected", "contract_variation_rejected"
		_, err = tx.ExecContext(ctx, `UPDATE contract_variations SET status='rejected',rejected_by=NULLIF($2,'')::uuid,rejected_at=now(),updated_at=now() WHERE id=$1`, variationID, actorID)
	case "incorporate":
		if !statusIn(fromStatus, "approved", "procurement_created") || cleanID(variation["current_version_id"]) == "" {
			return nil, validationError("Chỉ variation đã duyệt của hợp đồng có version hiện hành mới được tạo revision")
		}
		var nextVersion int
		if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(version_no),0)+1 FROM contract_versions WHERE contract_id=$1`, variation["contract_id"]).Scan(&nextVersion); err != nil {
			return nil, err
		}
		newAmount := number(variation["current_amount"]) + number(variation["amount_delta"])
		if newAmount < 0 {
			return nil, validationError("Variation làm giá trị hợp đồng âm")
		}
		endDate := dateOnly(variation["planned_end_date"])
		if endDate != "" && int(number(variation["days_delta"])) != 0 {
			parsed, _ := time.Parse("2006-01-02", endDate)
			endDate = parsed.AddDate(0, 0, int(number(variation["days_delta"]))).Format("2006-01-02")
		}
		scopeSummary := strings.TrimSpace(fmt.Sprint(variation["scope_summary"]))
		if delta := strings.TrimSpace(fmt.Sprint(variation["scope_delta"])); delta != "" {
			scopeSummary = strings.TrimSpace(scopeSummary + "\nPhụ lục " + fmt.Sprint(variation["variation_code"]) + ": " + delta)
		}
		hashPayload, _ := json.Marshal(Record{"contract_id": variation["contract_id"], "version_no": nextVersion, "source_variation_id": variationID, "amount": newAmount, "scope_summary": scopeSummary, "commercial_terms": variation["commercial_terms_json"], "payment_terms": variation["payment_terms_json"], "planned_start_date": variation["planned_start_date"], "planned_end_date": endDate})
		hash := sha256.Sum256(hashPayload)
		versionRows, err := queryTx(ctx, tx, `
			INSERT INTO contract_versions (contract_id,version_no,version_label,source_variation_id,amount,scope_summary,commercial_terms_json,payment_terms_json,planned_start_date,planned_end_date,document_id,definition_hash,status,review_comment,created_by)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,NULLIF($9,'')::date,NULLIF($10,'')::date,NULLIF($11,'')::uuid,$12,'draft',$13,NULLIF($14,'')::uuid)
			RETURNING id
		`, variation["contract_id"], nextVersion, fmt.Sprintf("V%d", nextVersion), variationID, newAmount, scopeSummary, variation["commercial_terms_json"], variation["payment_terms_json"], dateOnly(variation["planned_start_date"]), endDate, cleanID(variation["document_id"]), hex.EncodeToString(hash[:]), comment, actorID)
		if err != nil {
			return nil, err
		}
		toStatus, eventType = "incorporated", "contract_variation_incorporated"
		_, err = tx.ExecContext(ctx, `UPDATE contract_variations SET status='incorporated',resulting_version_id=$2,incorporated_by=NULLIF($3,'')::uuid,incorporated_at=now(),updated_at=now() WHERE id=$1`, variationID, versionRows[0]["id"], actorID)
	default:
		return nil, validationError("Hành động variation không hợp lệ", Record{"action": action})
	}
	if err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, cleanID(variation["project_id"]), actorID, actorName, eventType, "contract_variation", variationID, fmt.Sprintf("Chuyển variation từ %s sang %s", fromStatus, toStatus), Record{"contract_id": variation["contract_id"], "amount_delta": variation["amount_delta"], "days_delta": variation["days_delta"], "comment": comment}); err != nil {
		return nil, err
	}
	nextRoles := map[string][]string{
		"qs_review":           {"QS"},
		"purchasing_sourcing": {"Mua hàng"},
		"pending_approval":    {"PM", "Giám đốc"},
	}[toStatus]
	variation["status"] = toStatus
	if err := r.recordContractVariationEventTx(ctx, tx, variation, eventType, fromStatus, toStatus, comment, nextRoles, actorID, actorName); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetContractDetail(ctx, cleanID(variation["contract_id"]))
}

func (r *Repository) recordContractVariationEventTx(ctx context.Context, tx *sql.Tx, variation Record, eventType, fromStatus, toStatus string, comment any, nextRoles []string, actorID, actorName string) error {
	projectID := cleanID(variation["project_id"])
	if projectID == "" {
		var err error
		projectID, err = projectIDForContractTx(ctx, tx, cleanID(variation["contract_id"]))
		if err != nil {
			return err
		}
	}
	metadata := Record{"contract_id": variation["contract_id"], "priority": variation["priority"], "next_roles": nextRoles}
	rawMetadata, _ := json.Marshal(metadata)
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO contract_variation_events (
			variation_id,project_id,event_type,from_status,to_status,actor_id,actor_name,comment,metadata_json
		)
		VALUES ($1,$2,$3,NULLIF($4,''),NULLIF($5,''),NULLIF($6,'')::uuid,NULLIF($7,''),NULLIF($8,''),$9::jsonb)
	`, variation["id"], projectID, eventType, fromStatus, toStatus, actorID, actorName, strings.TrimSpace(fmt.Sprint(comment)), string(rawMetadata)); err != nil {
		return err
	}
	workItemID := cleanID(variation["source_work_item_id"])
	if workItemID == "" || len(nextRoles) == 0 {
		return nil
	}
	activityRows, err := queryTx(ctx, tx, `
		INSERT INTO work_item_activities (
			work_item_id,project_id,scope_type,scope_id,actor_id,activity_type,comment,visibility,metadata_json
		)
		VALUES ($1,$2,'work_item',$1,NULLIF($3,'')::uuid,'system_event',$4,'project',$5::jsonb)
		RETURNING *
	`, workItemID, projectID, actorID, fmt.Sprintf("Phát sinh %s chuyển sang %s", variation["variation_code"], toStatus), string(rawMetadata))
	if err != nil {
		return err
	}
	mentions := make([]ActivityMentionRequest, 0, len(nextRoles))
	for _, role := range nextRoles {
		mentions = append(mentions, ActivityMentionRequest{Type: "role", ID: role, Label: role})
	}
	return r.insertActivityAttentionTx(ctx, tx, activityRows[0], WorkItemActivityRequest{Mentions: mentions}, actorID, actorName)
}

func projectIDForContractTx(ctx context.Context, tx *sql.Tx, contractID string) (string, error) {
	var projectID string
	if err := tx.QueryRowContext(ctx, `SELECT project_id FROM contracts WHERE id=$1`, contractID).Scan(&projectID); err != nil {
		return "", err
	}
	return projectID, nil
}

func (r *Repository) GenerateProcurementFromContractVariation(ctx context.Context, variationID, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := queryTx(ctx, tx, `
		SELECT variation.*,contract.project_id
		FROM contract_variations variation
		JOIN contracts contract ON contract.id=variation.contract_id
		WHERE variation.id=$1 FOR UPDATE OF variation
	`, variationID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	variation := rows[0]
	if cleanID(variation["procurement_request_id"]) != "" {
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return r.GetContractDetail(ctx, cleanID(variation["contract_id"]))
	}
	if fmt.Sprint(variation["status"]) != "approved" {
		return nil, validationError("Chỉ phát sinh đã duyệt mới được tạo yêu cầu mua")
	}
	requestCode := fmt.Sprintf("VAR-%s-%d", strings.ToUpper(strings.ReplaceAll(fmt.Sprint(variation["variation_code"]), " ", "-")), time.Now().Unix())
	amount := math.Max(number(variation["amount_delta"]), 0)
	procurementRows, err := queryTx(ctx, tx, `
		INSERT INTO procurement_requests (project_id,request_code,title,requested_by,status,total_estimated_amount,budget_status)
		VALUES ($1,$2,$3,NULLIF($4,'')::uuid,'draft',$5,'within_budget')
		RETURNING id
	`, variation["project_id"], requestCode, fmt.Sprintf("Mua hàng cho phát sinh %s - %s", variation["variation_code"], variation["title"]), actorID, amount)
	if err != nil {
		return nil, err
	}
	procurementID := cleanID(procurementRows[0]["id"])
	if _, err := tx.ExecContext(ctx, `UPDATE contract_variations SET status='procurement_created',procurement_request_id=$2,updated_at=now() WHERE id=$1`, variationID, procurementID); err != nil {
		return nil, err
	}
	variation["status"] = "procurement_created"
	if err := r.recordContractVariationEventTx(ctx, tx, variation, "contract_variation_procurement_created", "approved", "procurement_created", "Tạo yêu cầu mua từ phát sinh", []string{"Mua hàng"}, actorID, actorName); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, cleanID(variation["project_id"]), actorID, actorName, "contract_variation_procurement_created", "contract_variation", variationID, "Tạo yêu cầu mua từ phát sinh đã duyệt", Record{"procurement_request_id": procurementID, "request_code": requestCode}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetContractDetail(ctx, cleanID(variation["contract_id"]))
}

func (r *Repository) ReleaseContractRetention(ctx context.Context, retentionID string, amount float64, dueDate, comment, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := queryTx(ctx, tx, `
		SELECT cr.*,c.project_id,c.status AS contract_status,c.counterparty_partner_id,c.counterparty_name,c.contract_type
		FROM contract_retentions cr JOIN contracts c ON c.id=cr.contract_id
		WHERE cr.id=$1 FOR UPDATE OF cr,c
	`, retentionID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	retention := rows[0]
	if !statusIn(fmt.Sprint(retention["contract_status"]), "active", "completed") || fmt.Sprint(retention["status"]) == "forfeited" {
		return nil, validationError("Khoản giữ lại không ở trạng thái cho phép giải phóng")
	}
	if warrantyEnd := dateOnly(retention["warranty_end_date"]); warrantyEnd != "" && warrantyEnd > time.Now().Format("2006-01-02") {
		return nil, validationError("Chưa hết thời hạn bảo hành để giải phóng giữ lại", Record{"warranty_end_date": warrantyEnd})
	}
	if amount > number(retention["outstanding_amount"])+0.000001 {
		return nil, validationError("Số tiền giải phóng vượt số dư giữ lại", Record{"outstanding_amount": retention["outstanding_amount"]})
	}
	if dueDate == "" {
		dueDate = time.Now().Format("2006-01-02")
	}
	var releaseNo int
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(release_no),0)+1 FROM contract_retention_releases WHERE retention_id=$1`, retentionID).Scan(&releaseNo); err != nil {
		return nil, err
	}
	obligationType := fmt.Sprint(retention["direction"])
	obligationID := ""
	if obligationType == "receivable" {
		obligationRows, err := queryTx(ctx, tx, `
			INSERT INTO receivables (project_id,receivable_code,client_partner_id,client_name,source_type,source_id,contract_amount,milestone_name,gross_amount,received_amount,remaining_amount,due_date,status,created_by)
			VALUES ($1,$2,NULLIF($3,'')::uuid,$4,'contract_retention',$5,$6,'Giải phóng giữ lại',$7,0,$7,$8,'draft',NULLIF($9,'')::uuid) RETURNING id
		`, retention["project_id"], fmt.Sprintf("AR-RET-%s-%02d", retentionID[:8], releaseNo), cleanID(retention["counterparty_partner_id"]), retention["counterparty_name"], retentionID, retention["held_amount"], amount, dueDate, actorID)
		if err != nil {
			return nil, err
		}
		obligationID = cleanID(obligationRows[0]["id"])
	} else {
		vendorType := fmt.Sprint(retention["contract_type"])
		if vendorType == "service" {
			vendorType = "service_provider"
		}
		obligationRows, err := queryTx(ctx, tx, `
			INSERT INTO payables (project_id,payable_code,vendor_type,vendor_partner_id,vendor_name,source_type,source_id,gross_amount,advance_amount,retention_amount,deduction_amount,paid_amount,remaining_amount,due_date,status,created_by)
			VALUES ($1,$2,$3,NULLIF($4,'')::uuid,$5,'contract_retention',$6,$7,0,0,0,0,$7,$8,'draft',NULLIF($9,'')::uuid) RETURNING id
		`, retention["project_id"], fmt.Sprintf("AP-RET-%s-%02d", retentionID[:8], releaseNo), vendorType, cleanID(retention["counterparty_partner_id"]), retention["counterparty_name"], retentionID, amount, dueDate, actorID)
		if err != nil {
			return nil, err
		}
		obligationID = cleanID(obligationRows[0]["id"])
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO contract_retention_releases (retention_id,release_no,amount,due_date,comment,obligation_type,obligation_id,released_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,NULLIF($8,'')::uuid)
	`, retentionID, releaseNo, amount, dueDate, comment, obligationType, obligationID, actorID); err != nil {
		return nil, err
	}
	newReleased := number(retention["released_amount"]) + amount
	newStatus := "partially_released"
	if newReleased+0.000001 >= number(retention["held_amount"]) {
		newStatus = "released"
	}
	if _, err := tx.ExecContext(ctx, `UPDATE contract_retentions SET released_amount=$2,status=$3,release_obligation_type=$4,release_obligation_id=$5,released_by=NULLIF($6,'')::uuid,released_at=now(),release_comment=$7,updated_at=now() WHERE id=$1`, retentionID, newReleased, newStatus, obligationType, obligationID, actorID, comment); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, cleanID(retention["project_id"]), actorID, actorName, "contract_retention_released", "contract_retention", retentionID, "Giải phóng khoản giữ lại", Record{"contract_id": retention["contract_id"], "amount": amount, "released_amount": newReleased, "outstanding_amount": number(retention["held_amount"]) - newReleased, "obligation_type": obligationType, "obligation_id": obligationID, "comment": comment}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetContractDetail(ctx, cleanID(retention["contract_id"]))
}
