package core

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

func (r *Repository) CreateStockMovement(ctx context.Context, req StockActionRequest, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	movement, err := createStockMovementTx(ctx, tx, req, actorID, actorName)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return movement, nil
}

func createStockMovementTx(ctx context.Context, tx *sql.Tx, req StockActionRequest, actorID, actorName string) (Record, error) {
	req.MovementType = strings.TrimSpace(req.MovementType)
	if req.MovementType == "" {
		req.MovementType = "receive"
	}
	if !statusIn(req.MovementType, "receive", "issue", "transfer", "loss", "return_to_warehouse", "return_to_subcontractor", "adjustment_in", "adjustment_out") {
		return nil, validationError("Loại nhập xuất kho không hợp lệ", Record{"movement_type": req.MovementType})
	}
	req.ProjectID = cleanID(req.ProjectID)
	req.WarehouseID = cleanID(req.WarehouseID)
	req.TargetWarehouseID = cleanID(req.TargetWarehouseID)
	req.MaterialID = cleanID(req.MaterialID)
	req.ReferenceID = cleanID(req.ReferenceID)
	req.StockDocumentID = cleanID(req.StockDocumentID)
	req.ReferenceType = strings.TrimSpace(req.ReferenceType)
	req.Unit = strings.TrimSpace(req.Unit)
	req.Reason = strings.TrimSpace(req.Reason)
	if req.WarehouseID == "" {
		return nil, validationError("Kho là bắt buộc")
	}
	if req.MovementType == "transfer" && (req.TargetWarehouseID == "" || req.TargetWarehouseID == req.WarehouseID) {
		return nil, validationError("Điều chuyển cần kho đích khác kho nguồn")
	}
	if req.MaterialID == "" {
		return nil, validationError("Vật tư là bắt buộc")
	}
	if req.Quantity <= 0 {
		return nil, validationError("Số lượng nhập xuất kho phải lớn hơn 0", Record{"quantity": req.Quantity})
	}
	if statusIn(req.MovementType, "issue", "return_to_warehouse", "return_to_subcontractor") && req.ProjectID == "" {
		return nil, validationError("Công trình là bắt buộc cho thao tác xuất/hoàn vật tư")
	}

	lockKeys := []string{"material-tracking:" + req.MaterialID, "stock:" + req.WarehouseID + ":" + req.MaterialID}
	if req.TargetWarehouseID != "" {
		lockKeys = append(lockKeys, "stock:"+req.TargetWarehouseID+":"+req.MaterialID)
	}
	if req.ProjectID != "" {
		lockKeys = append(lockKeys, "site-material:"+req.ProjectID+":"+req.MaterialID)
	}
	sort.Strings(lockKeys)
	for _, lockKey := range lockKeys {
		if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1)::bigint)`, lockKey); err != nil {
			return nil, err
		}
	}

	warehouseIDs := []string{req.WarehouseID}
	if req.TargetWarehouseID != "" {
		warehouseIDs = append(warehouseIDs, req.TargetWarehouseID)
	}
	for _, warehouseID := range warehouseIDs {
		warehouseRows, err := queryTx(ctx, tx, `SELECT id FROM warehouses WHERE id=$1 AND active`, warehouseID)
		if err != nil {
			return nil, err
		}
		if len(warehouseRows) == 0 {
			return nil, validationError("Kho không tồn tại hoặc đã ngưng hoạt động", Record{"warehouse_id": warehouseID})
		}
	}
	materialRows, err := queryTx(ctx, tx, `SELECT id, unit, tracking_mode FROM materials WHERE id=$1 AND active`, req.MaterialID)
	if err != nil {
		return nil, err
	}
	if len(materialRows) == 0 {
		return nil, validationError("Vật tư không tồn tại hoặc đã ngưng hoạt động", Record{"material_id": req.MaterialID})
	}
	if req.Unit == "" {
		req.Unit = strings.TrimSpace(fmt.Sprint(materialRows[0]["unit"]))
	}
	if req.Unit == "" {
		return nil, validationError("Đơn vị vật tư là bắt buộc")
	}
	materialUnit := strings.TrimSpace(fmt.Sprint(materialRows[0]["unit"]))
	if materialUnit != "" && req.Unit != materialUnit {
		return nil, validationError("ĐVT phiếu kho phải trùng ĐVT của vật tư", Record{"material_id": req.MaterialID, "expected_unit": materialUnit, "unit": req.Unit})
	}
	if cleanOptionalText(materialRows[0]["tracking_mode"]) == "physical_unit" && req.StockDocumentID == "" {
		return nil, validationError("Vật tư theo dõi từng đơn vị phải ghi nhận qua phiếu kho có phân bổ Đoạn/Cuộn/Cây/Tấm", Record{"material_id": req.MaterialID})
	}
	if req.MovementType == "receive" && req.ReferenceType == "incoming_acceptance_item" && req.ReferenceID != "" {
		sourceRows, err := queryTx(ctx, tx, `
			SELECT iai.material_id, iai.project_id, iai.accepted_quantity,
				COALESCE(received.received_quantity,0)+COALESCE(legacy_stock.received_quantity,0) AS received_quantity
			FROM incoming_acceptance_items iai
			JOIN incoming_acceptance_records iar ON iar.id=iai.incoming_acceptance_id AND iar.status='submitted'
			LEFT JOIN procurement_request_items pri ON pri.id=iai.procurement_request_item_id
			LEFT JOIN LATERAL (
				SELECT SUM(CASE WHEN sm.movement_type='receive' THEN sm.quantity WHEN sm.movement_type='adjustment_out' THEN -sm.quantity ELSE 0 END) AS received_quantity
				FROM stock_movements sm
				WHERE sm.movement_type IN ('receive','adjustment_out')
				  AND sm.reference_type='incoming_acceptance_item'
				  AND sm.reference_id=iai.id
			) received ON true
			LEFT JOIN LATERAL (
				SELECT SUM(CASE WHEN sm.movement_type='receive' THEN sm.quantity WHEN sm.movement_type='adjustment_out' THEN -sm.quantity ELSE 0 END) AS received_quantity
				FROM stock_movements sm
				WHERE sm.movement_type IN ('receive','adjustment_out')
				  AND sm.reference_type='boq_item'
				  AND sm.reference_id=pri.boq_item_id
			) legacy_stock ON true
			WHERE iai.id=$1
			FOR UPDATE OF iai
		`, req.ReferenceID)
		if err != nil {
			return nil, err
		}
		if len(sourceRows) == 0 || cleanID(sourceRows[0]["material_id"]) != req.MaterialID {
			return nil, validationError("Dòng nghiệm thu đầu vào không tồn tại hoặc không khớp vật tư")
		}
		if req.ProjectID != "" && cleanID(sourceRows[0]["project_id"]) != req.ProjectID {
			return nil, validationError("Dòng nghiệm thu đầu vào không thuộc công trình của phiếu nhập")
		}
		remaining := number(sourceRows[0]["accepted_quantity"]) - number(sourceRows[0]["received_quantity"])
		if quantityExceedsAvailable(req.Quantity, remaining) {
			return nil, validationError("Số lượng nhập vượt khối lượng đã đạt nghiệm thu còn lại", Record{"remaining": remaining, "quantity": req.Quantity})
		}
	}

	if statusIn(req.MovementType, "issue", "transfer", "loss", "adjustment_out") {
		available, err := warehouseMaterialAvailableTx(ctx, tx, req.WarehouseID, req.MaterialID)
		if err != nil {
			return nil, err
		}
		if quantityExceedsAvailable(req.Quantity, available) {
			return nil, validationError("Tồn kho không đủ cho thao tác này", Record{"available": available, "quantity": req.Quantity})
		}
	}
	if statusIn(req.MovementType, "return_to_warehouse", "return_to_subcontractor") {
		available, err := projectMaterialAvailableTx(ctx, tx, req.ProjectID, req.MaterialID)
		if err != nil {
			return nil, err
		}
		if quantityExceedsAvailable(req.Quantity, available) {
			return nil, validationError("Vật tư tại công trình không đủ để hoàn/trả", Record{"available": available, "quantity": req.Quantity})
		}
	}

	code := fmt.Sprintf("SM-%s-%d-%s", req.MovementType, timeNowUnix(), newID()[:8])
	movements, err := queryTx(ctx, tx, `
		INSERT INTO stock_movements
			(movement_code, project_id, warehouse_id, target_warehouse_id, material_id, movement_type, quantity, unit, reason, reference_type, reference_id, stock_document_id, created_by, approved_by)
		VALUES ($1,NULLIF($2,'')::uuid,$3,NULLIF($4,'')::uuid,$5,$6,$7,$8,$9,$10,NULLIF($11,'')::uuid,NULLIF($12,'')::uuid,NULLIF($13,'')::uuid,NULLIF($13,'')::uuid)
		RETURNING *
	`, code, req.ProjectID, req.WarehouseID, req.TargetWarehouseID, req.MaterialID, req.MovementType, req.Quantity, req.Unit, req.Reason, req.ReferenceType, req.ReferenceID, req.StockDocumentID, actorID)
	if err != nil {
		return nil, err
	}
	if len(movements) == 0 {
		return nil, sql.ErrNoRows
	}
	movement := movements[0]

	if req.ProjectID != "" && statusIn(req.MovementType, "issue", "return_to_warehouse", "return_to_subcontractor") {
		if req.MovementType == "issue" {
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO site_material_balances (project_id, material_id, issued_quantity, surplus_quantity, surplus_status)
				VALUES ($1,$2,$3::numeric,GREATEST($3::numeric,0::numeric),'pending')
				ON CONFLICT (project_id, material_id)
				DO UPDATE SET issued_quantity = site_material_balances.issued_quantity + EXCLUDED.issued_quantity,
					surplus_quantity = GREATEST(site_material_balances.issued_quantity + EXCLUDED.issued_quantity - site_material_balances.used_quantity - site_material_balances.waste_quantity, 0),
					surplus_status = CASE
						WHEN GREATEST(site_material_balances.issued_quantity + EXCLUDED.issued_quantity - site_material_balances.used_quantity - site_material_balances.waste_quantity, 0) > 0.000001 THEN 'pending'
						ELSE 'handled'
					END,
					updated_at=now()
			`, req.ProjectID, req.MaterialID, req.Quantity); err != nil {
				return nil, err
			}
		} else {
			updated, err := queryTx(ctx, tx, `
				UPDATE site_material_balances
				SET issued_quantity = GREATEST(issued_quantity - $3::numeric, 0::numeric),
					surplus_quantity = GREATEST(GREATEST(issued_quantity - $3::numeric, 0::numeric) - used_quantity - waste_quantity, 0::numeric),
					surplus_status = CASE
						WHEN GREATEST(GREATEST(issued_quantity - $3::numeric, 0::numeric) - used_quantity - waste_quantity, 0::numeric) > 0.000001 THEN 'partial'
						ELSE 'handled'
					END,
					updated_at = now()
				WHERE project_id=$1 AND material_id=$2
				RETURNING *
			`, req.ProjectID, req.MaterialID, req.Quantity)
			if err != nil {
				return nil, err
			}
			if len(updated) == 0 {
				return nil, validationError("Không tìm thấy số dư vật tư tại công trình", Record{"project_id": req.ProjectID, "material_id": req.MaterialID})
			}
		}
	}

	metadata := Record{"movement_type": req.MovementType, "quantity": req.Quantity, "material_id": req.MaterialID, "warehouse_id": req.WarehouseID}
	if req.TargetWarehouseID != "" {
		metadata["target_warehouse_id"] = req.TargetWarehouseID
	}
	if req.ReferenceType != "" {
		metadata["reference_type"] = req.ReferenceType
	}
	if req.ReferenceID != "" {
		metadata["reference_id"] = req.ReferenceID
	}
	if req.StockDocumentID != "" {
		metadata["stock_document_id"] = req.StockDocumentID
	}
	if err := addAuditTx(ctx, tx, req.ProjectID, actorID, actorName, stockMovementEvent(req.MovementType), "stock_movement", fmt.Sprint(movement["id"]), stockMovementDescription(req), metadata); err != nil {
		return nil, err
	}
	return movement, nil
}

func warehouseMaterialAvailableTx(ctx context.Context, tx *sql.Tx, warehouseID, materialID string) (float64, error) {
	rows, err := queryTx(ctx, tx, `
		SELECT COALESCE(SUM(CASE
			WHEN warehouse_id=$1 AND movement_type IN ('receive','return_to_warehouse','adjustment_in') THEN quantity
			WHEN warehouse_id=$1 AND movement_type IN ('issue','transfer','loss','adjustment_out') THEN -quantity
			WHEN target_warehouse_id=$1 AND movement_type='transfer' THEN quantity
			ELSE 0
		END),0) AS available
		FROM stock_movements
		WHERE material_id=$2 AND (warehouse_id=$1 OR target_warehouse_id=$1)
	`, warehouseID, materialID)
	if err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, nil
	}
	return number(rows[0]["available"]), nil
}

func projectMaterialAvailableTx(ctx context.Context, tx *sql.Tx, projectID, materialID string) (float64, error) {
	rows, err := queryTx(ctx, tx, `
		SELECT GREATEST(issued_quantity - used_quantity - waste_quantity, surplus_quantity, 0) AS available
		FROM site_material_balances
		WHERE project_id=$1 AND material_id=$2
		FOR UPDATE
	`, projectID, materialID)
	if err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, nil
	}
	return number(rows[0]["available"]), nil
}

func quantityExceedsAvailable(quantity, available float64) bool {
	return quantity > available+0.000001
}

func stockMovementEvent(movementType string) string {
	switch movementType {
	case "receive":
		return "warehouse_received"
	case "issue":
		return "warehouse_issued"
	case "transfer":
		return "warehouse_transferred"
	case "loss":
		return "warehouse_loss_recorded"
	case "return_to_warehouse":
		return "surplus_stored"
	case "return_to_subcontractor":
		return "surplus_returned_to_subcontractor"
	case "adjustment_in":
		return "warehouse_adjusted_in"
	case "adjustment_out":
		return "warehouse_adjusted_out"
	default:
		return "stock_movement_created"
	}
}

func stockMovementDescription(req StockActionRequest) string {
	if req.Reason != "" {
		return req.Reason
	}
	return fmt.Sprintf("Ghi nhận %s %.3f %s", req.MovementType, req.Quantity, req.Unit)
}

func (r *Repository) CreateStockDocument(ctx context.Context, req StockDocumentRequest, actorID, actorName string) (Record, error) {
	if err := normalizeStockDocumentRequest(&req, false); err != nil {
		return nil, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	document, err := r.createStockDocumentTx(ctx, tx, req, actorID, actorName, "")
	if err != nil {
		return nil, err
	}
	documentID := cleanID(document["id"])
	transition, err := r.startWorkItemForDomainMutationTx(ctx, tx, req.ProjectID, req.WorkItemContext, warehouseWorkItemPolicy, "stock_document", documentID, actorID, actorName)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	created, err := r.GetStockDocument(ctx, documentID)
	return attachWorkItemTransition(created, transition), err
}

func normalizeStockDocumentRequest(req *StockDocumentRequest, allowAdjustment bool) error {
	req.ProjectID = cleanID(req.ProjectID)
	req.MovementType = strings.TrimSpace(req.MovementType)
	req.SourceWarehouseID = cleanID(req.SourceWarehouseID)
	req.TargetWarehouseID = cleanID(req.TargetWarehouseID)
	req.ReferenceType = strings.TrimSpace(req.ReferenceType)
	req.ReferenceID = cleanID(req.ReferenceID)
	req.DocumentDate = strings.TrimSpace(req.DocumentDate)
	req.Note = strings.TrimSpace(req.Note)
	req.CorrectionReason = strings.TrimSpace(req.CorrectionReason)
	allowedTypes := []string{"receive", "issue", "transfer", "return_to_warehouse", "return_to_subcontractor"}
	if allowAdjustment {
		allowedTypes = append(allowedTypes, "adjustment_in", "adjustment_out")
	}
	if !statusIn(req.MovementType, allowedTypes...) {
		return validationError("Loại phiếu kho không hợp lệ", Record{"movement_type": req.MovementType})
	}
	if len(req.Items) == 0 {
		return validationError("Phiếu kho cần ít nhất một dòng vật tư")
	}
	if req.DocumentDate == "" {
		req.DocumentDate = time.Now().Format("2006-01-02")
	}
	if _, err := time.Parse("2006-01-02", req.DocumentDate); err != nil {
		return validationError("Ngày phiếu kho không hợp lệ")
	}
	if statusIn(req.MovementType, "issue", "transfer") && req.SourceWarehouseID == "" {
		return validationError("Kho nguồn là bắt buộc")
	}
	if statusIn(req.MovementType, "receive", "return_to_warehouse", "adjustment_in") && req.TargetWarehouseID == "" {
		return validationError("Kho nhận là bắt buộc")
	}
	if req.MovementType == "adjustment_out" && req.SourceWarehouseID == "" {
		return validationError("Kho điều chỉnh là bắt buộc")
	}
	if req.MovementType == "transfer" && (req.TargetWarehouseID == "" || req.TargetWarehouseID == req.SourceWarehouseID) {
		return validationError("Điều chuyển cần kho đích khác kho nguồn")
	}
	if statusIn(req.MovementType, "issue", "return_to_warehouse", "return_to_subcontractor") && req.ProjectID == "" {
		return validationError("Công trình là bắt buộc cho phiếu xuất/hoàn trả")
	}
	if req.MovementType == "return_to_subcontractor" && req.SourceWarehouseID == "" && req.TargetWarehouseID == "" {
		return validationError("Kho quản lý vật tư hoàn trả là bắt buộc")
	}
	for index := range req.Items {
		item := &req.Items[index]
		item.MaterialID = cleanID(item.MaterialID)
		item.Unit = strings.TrimSpace(item.Unit)
		item.Reason = strings.TrimSpace(item.Reason)
		item.SourceLineType = strings.TrimSpace(item.SourceLineType)
		item.SourceLineID = cleanID(item.SourceLineID)
		if item.MaterialID == "" || item.Quantity <= 0 {
			return validationError("Dòng vật tư phiếu kho không hợp lệ", Record{"item_index": index})
		}
	}
	return nil
}

func (r *Repository) createStockDocumentTx(ctx context.Context, tx *sql.Tx, req StockDocumentRequest, actorID, actorName, correctionOfDocumentID string) (Record, error) {
	if req.ReferenceType == "incoming_acceptance" {
		rows, err := queryTx(ctx, tx, `
			SELECT id, project_id, status
			FROM incoming_acceptance_records
			WHERE id=$1
			FOR UPDATE
		`, req.ReferenceID)
		if err != nil {
			return nil, err
		}
		if len(rows) == 0 || fmt.Sprint(rows[0]["status"]) != "submitted" {
			return nil, validationError("Nghiệm thu đầu vào chưa được gửi hoặc không tồn tại")
		}
		if req.ProjectID == "" {
			req.ProjectID = cleanID(rows[0]["project_id"])
		}
		if req.ProjectID != cleanID(rows[0]["project_id"]) {
			return nil, validationError("Nghiệm thu đầu vào không thuộc công trình của phiếu kho")
		}
	}

	prefix := map[string]string{"receive": "PNK", "issue": "PXK", "transfer": "PDC", "return_to_warehouse": "PHT", "return_to_subcontractor": "PTN", "adjustment_in": "PDT", "adjustment_out": "PDG"}[req.MovementType]
	documentCode := fmt.Sprintf("%s-%d-%s", prefix, timeNowUnix(), newID()[:6])
	documents, err := queryTx(ctx, tx, `
		INSERT INTO stock_documents (
			document_code, project_id, movement_type, source_warehouse_id, target_warehouse_id,
			reference_type, reference_id, document_date, note, status, created_by, posted_by, correction_of_document_id
		) VALUES (
			$1,NULLIF($2,'')::uuid,$3,NULLIF($4,'')::uuid,NULLIF($5,'')::uuid,
			$6,NULLIF($7,'')::uuid,$8::date,$9,'posted',NULLIF($10,'')::uuid,NULLIF($10,'')::uuid,NULLIF($11,'')::uuid
		)
		RETURNING *
	`, documentCode, req.ProjectID, req.MovementType, req.SourceWarehouseID, req.TargetWarehouseID, req.ReferenceType, req.ReferenceID, req.DocumentDate, req.Note, actorID, correctionOfDocumentID)
	if err != nil {
		return nil, err
	}
	if len(documents) == 0 {
		return nil, sql.ErrNoRows
	}
	documentID := cleanID(documents[0]["id"])

	for itemIndex, item := range req.Items {
		if req.ReferenceType == "incoming_acceptance" {
			if item.SourceLineType != "incoming_acceptance_item" || item.SourceLineID == "" {
				return nil, validationError("Dòng nhập kho cần liên kết dòng nghiệm thu đầu vào", Record{"item_index": itemIndex})
			}
			rows, err := queryTx(ctx, tx, `
				SELECT iai.id, iai.incoming_acceptance_id, iai.material_id, iai.accepted_quantity,
					COALESCE(received.received_quantity,0)+COALESCE(legacy_stock.received_quantity,0) AS received_quantity
				FROM incoming_acceptance_items iai
				LEFT JOIN procurement_request_items pri ON pri.id=iai.procurement_request_item_id
				LEFT JOIN LATERAL (
					SELECT SUM(CASE WHEN sm.movement_type='receive' THEN sm.quantity WHEN sm.movement_type='adjustment_out' THEN -sm.quantity ELSE 0 END) AS received_quantity
					FROM stock_movements sm
					WHERE sm.movement_type IN ('receive','adjustment_out')
					  AND sm.reference_type='incoming_acceptance_item'
					  AND sm.reference_id=iai.id
				) received ON true
				LEFT JOIN LATERAL (
					SELECT SUM(CASE WHEN sm.movement_type='receive' THEN sm.quantity WHEN sm.movement_type='adjustment_out' THEN -sm.quantity ELSE 0 END) AS received_quantity
					FROM stock_movements sm
					WHERE sm.movement_type IN ('receive','adjustment_out')
					  AND sm.reference_type='boq_item'
					  AND sm.reference_id=pri.boq_item_id
				) legacy_stock ON true
				WHERE iai.id=$1
				FOR UPDATE OF iai
			`, item.SourceLineID)
			if err != nil {
				return nil, err
			}
			if len(rows) == 0 || cleanID(rows[0]["incoming_acceptance_id"]) != req.ReferenceID || cleanID(rows[0]["material_id"]) != item.MaterialID {
				return nil, validationError("Dòng nghiệm thu đầu vào không khớp vật tư hoặc chứng từ nguồn", Record{"item_index": itemIndex})
			}
			remaining := number(rows[0]["accepted_quantity"]) - number(rows[0]["received_quantity"])
			if quantityExceedsAvailable(item.Quantity, remaining) {
				return nil, validationError("Số lượng nhập vượt khối lượng đã đạt nghiệm thu còn lại", Record{"item_index": itemIndex, "remaining": remaining, "quantity": item.Quantity})
			}
		}
		itemRows, err := queryTx(ctx, tx, `
			INSERT INTO stock_document_items (document_id, material_id, quantity, unit, reason, source_line_type, source_line_id)
			VALUES ($1,$2,$3,$4,$5,$6,NULLIF($7,'')::uuid)
			RETURNING id
		`, documentID, item.MaterialID, item.Quantity, item.Unit, item.Reason, item.SourceLineType, item.SourceLineID)
		if err != nil {
			return nil, err
		}
		if len(itemRows) == 0 {
			return nil, sql.ErrNoRows
		}
		warehouseID := req.SourceWarehouseID
		if statusIn(req.MovementType, "receive", "return_to_warehouse", "adjustment_in") {
			warehouseID = req.TargetWarehouseID
		}
		if req.MovementType == "return_to_subcontractor" && warehouseID == "" {
			warehouseID = req.TargetWarehouseID
		}
		movementReferenceType := item.SourceLineType
		movementReferenceID := item.SourceLineID
		if movementReferenceType == "" {
			movementReferenceType = "stock_document_item"
			movementReferenceID = cleanID(itemRows[0]["id"])
		}
		movementTargetWarehouseID := ""
		if req.MovementType == "transfer" {
			movementTargetWarehouseID = req.TargetWarehouseID
		}
		movement, err := createStockMovementTx(ctx, tx, StockActionRequest{
			ProjectID: req.ProjectID, WarehouseID: warehouseID, TargetWarehouseID: movementTargetWarehouseID,
			MaterialID: item.MaterialID, MovementType: req.MovementType, Quantity: item.Quantity,
			Unit: item.Unit, Reason: item.Reason, ReferenceType: movementReferenceType,
			ReferenceID: movementReferenceID, StockDocumentID: documentID,
		}, actorID, actorName)
		if err != nil {
			return nil, err
		}
		if err := applyStockUnitMovementTx(ctx, tx, req, item, cleanID(itemRows[0]["id"]), movement, actorID); err != nil {
			return nil, err
		}
	}

	if err := addAuditTx(ctx, tx, req.ProjectID, actorID, actorName, "stock_document_posted", "stock_document", documentID, "Ghi nhận phiếu kho "+documentCode, Record{
		"document_code": documentCode, "movement_type": req.MovementType, "item_count": len(req.Items),
		"source_warehouse_id": req.SourceWarehouseID, "target_warehouse_id": req.TargetWarehouseID,
		"reference_type": req.ReferenceType, "reference_id": req.ReferenceID,
		"correction_of_document_id": correctionOfDocumentID,
	}); err != nil {
		return nil, err
	}
	return documents[0], nil
}

func (r *Repository) CreateStockAdjustment(ctx context.Context, req StockAdjustmentRequest, actorID, actorName string) (Record, error) {
	req.WarehouseID = cleanID(req.WarehouseID)
	req.MaterialID = cleanID(req.MaterialID)
	req.DocumentDate = strings.TrimSpace(req.DocumentDate)
	req.Reason = strings.TrimSpace(req.Reason)
	if req.WarehouseID == "" || req.MaterialID == "" {
		return nil, validationError("Kho và vật tư là bắt buộc")
	}
	if math.IsNaN(req.TargetQuantity) || math.IsInf(req.TargetQuantity, 0) || req.TargetQuantity < 0 {
		return nil, validationError("Tồn sau điều chỉnh phải lớn hơn hoặc bằng 0")
	}
	if req.Reason == "" {
		return nil, validationError("Lý do điều chỉnh tồn kho là bắt buộc")
	}
	if len([]rune(req.Reason)) > 2000 {
		return nil, validationError("Lý do điều chỉnh không được vượt quá 2000 ký tự")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	for _, lockKey := range []string{"material-tracking:" + req.MaterialID, "stock:" + req.WarehouseID + ":" + req.MaterialID} {
		if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1)::bigint)`, lockKey); err != nil {
			return nil, err
		}
	}
	warehouse, err := queryOneRecord(ctx, tx, `SELECT id,name FROM warehouses WHERE id=$1 AND active`, req.WarehouseID)
	if err != nil {
		return nil, validationError("Kho không tồn tại hoặc đã ngưng hoạt động", Record{"warehouse_id": req.WarehouseID})
	}
	material, err := queryOneRecord(ctx, tx, `SELECT id,code,name,unit,tracking_mode FROM materials WHERE id=$1 AND active`, req.MaterialID)
	if err != nil {
		return nil, validationError("Vật tư không tồn tại hoặc đã ngưng hoạt động", Record{"material_id": req.MaterialID})
	}
	if cleanOptionalText(material["tracking_mode"]) == "physical_unit" {
		return nil, validationError("Vật tư theo dõi từng Đoạn/Cuộn/Cây/Tấm phải điều chỉnh bằng phiếu kho có chọn đúng đơn vị vật lý", Record{"material_id": req.MaterialID})
	}
	currentQuantity, err := warehouseMaterialAvailableTx(ctx, tx, req.WarehouseID, req.MaterialID)
	if err != nil {
		return nil, err
	}
	delta := req.TargetQuantity - currentQuantity
	if math.Abs(delta) <= 0.000001 {
		return nil, validationError("Tồn sau điều chỉnh không thay đổi", Record{"current_quantity": currentQuantity})
	}
	movementType := "adjustment_in"
	sourceWarehouseID := ""
	targetWarehouseID := req.WarehouseID
	if delta < 0 {
		movementType = "adjustment_out"
		sourceWarehouseID = req.WarehouseID
		targetWarehouseID = ""
		delta = -delta
	}
	documentReq := StockDocumentRequest{
		MovementType: movementType, SourceWarehouseID: sourceWarehouseID, TargetWarehouseID: targetWarehouseID,
		DocumentDate: req.DocumentDate, Note: req.Reason,
		Items: []StockDocumentItemRequest{{MaterialID: req.MaterialID, Quantity: delta, Unit: cleanOptionalText(material["unit"]), Reason: req.Reason}},
	}
	if err := normalizeStockDocumentRequest(&documentReq, true); err != nil {
		return nil, err
	}
	document, err := r.createStockDocumentTx(ctx, tx, documentReq, actorID, actorName, "")
	if err != nil {
		return nil, err
	}
	documentID := cleanID(document["id"])
	if err := addAuditTx(ctx, tx, "", actorID, actorName, "stock_position_adjusted", "stock_document", documentID, fmt.Sprintf("Điều chỉnh tồn %s tại %s", material["name"], warehouse["name"]), Record{
		"warehouse_id": req.WarehouseID, "material_id": req.MaterialID, "before_quantity": currentQuantity,
		"target_quantity": req.TargetQuantity, "movement_type": movementType, "adjustment_quantity": delta, "reason": req.Reason,
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetStockDocument(ctx, documentID)
}

func (r *Repository) UpdateStockDocument(ctx context.Context, documentID string, req StockDocumentRequest, actorID, actorName string) (Record, error) {
	if err := normalizeStockDocumentRequest(&req, false); err != nil {
		return nil, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	current, err := queryOneRecord(ctx, tx, `SELECT * FROM stock_documents WHERE id=$1 FOR UPDATE`, documentID)
	if err != nil {
		return nil, err
	}
	if cleanOptionalText(current["status"]) != "posted" {
		return nil, validationError("Chỉ phiếu đang ghi sổ mới được chỉnh sửa", Record{"document_id": documentID, "status": current["status"]})
	}
	if statusIn(cleanOptionalText(current["movement_type"]), "adjustment_in", "adjustment_out") {
		return nil, validationError("Phiếu điều chỉnh tồn không được sửa; hãy lập điều chỉnh mới")
	}
	items, err := queryTx(ctx, tx, `SELECT * FROM stock_document_items WHERE document_id=$1 ORDER BY created_at,id`, documentID)
	if err != nil {
		return nil, err
	}
	if !stockDocumentCoreChanged(current, items, req) {
		updated, err := queryOneRecord(ctx, tx, `
			UPDATE stock_documents SET document_date=$2::date,note=$3,updated_at=now()
			WHERE id=$1 RETURNING *
		`, documentID, req.DocumentDate, req.Note)
		if err != nil {
			return nil, err
		}
		if err := addAuditTx(ctx, tx, cleanID(current["project_id"]), actorID, actorName, "stock_document_metadata_updated", "stock_document", documentID, "Cập nhật ngày/ghi chú phiếu kho "+cleanOptionalText(current["document_code"]), Record{
			"before": Record{"document_date": current["document_date"], "note": current["note"]},
			"after":  Record{"document_date": updated["document_date"], "note": updated["note"]},
		}); err != nil {
			return nil, err
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return r.GetStockDocument(ctx, documentID)
	}
	if req.CorrectionReason == "" {
		return nil, validationError("Lý do điều chỉnh phiếu là bắt buộc khi thay đổi dữ liệu tồn kho")
	}
	var trackedAllocationCount int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM stock_movement_unit_allocations allocation
		JOIN stock_movements movement ON movement.id=allocation.stock_movement_id
		WHERE movement.stock_document_id=$1
	`, documentID).Scan(&trackedAllocationCount); err != nil {
		return nil, err
	}
	if trackedAllocationCount > 0 {
		return nil, validationError("Phiếu có đơn vị vật lý chỉ cho sửa ngày/ghi chú; để đổi tồn cần lập phiếu hoàn/điều chuyển mới nhằm giữ nguyên lịch sử Đoạn/Cuộn/Cây/Tấm", Record{"document_id": documentID})
	}
	if len([]rune(req.CorrectionReason)) > 2000 {
		return nil, validationError("Lý do điều chỉnh phiếu không được vượt quá 2000 ký tự")
	}
	if err := r.reverseStockDocumentTx(ctx, tx, current, actorID, actorName, req.CorrectionReason); err != nil {
		return nil, err
	}
	replacement, err := r.createStockDocumentTx(ctx, tx, req, actorID, actorName, documentID)
	if err != nil {
		return nil, err
	}
	if err := validateStockDocumentCorrectionSiteBalancesTx(ctx, tx, current, req); err != nil {
		return nil, err
	}
	replacementID := cleanID(replacement["id"])
	if _, err := tx.ExecContext(ctx, `
		UPDATE stock_documents
		SET status='corrected',corrected_by_document_id=$2,correction_reason=$3,
			corrected_by=NULLIF($4,'')::uuid,corrected_at=now(),updated_at=now()
		WHERE id=$1
	`, documentID, replacementID, req.CorrectionReason, actorID); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE project_material_costs SET stock_document_id=$2,updated_at=now() WHERE stock_document_id=$1`, documentID, replacementID); err != nil {
		return nil, err
	}
	transition, err := r.startWorkItemForDomainMutationTx(ctx, tx, req.ProjectID, req.WorkItemContext, warehouseWorkItemPolicy, "stock_document", replacementID, actorID, actorName)
	if err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, cleanID(current["project_id"]), actorID, actorName, "stock_document_corrected", "stock_document", documentID, "Điều chỉnh phiếu kho "+cleanOptionalText(current["document_code"]), Record{
		"replacement_document_id": replacementID, "replacement_document_code": replacement["document_code"],
		"reason": req.CorrectionReason, "before_movement_type": current["movement_type"], "after_movement_type": req.MovementType,
		"before_item_count": len(items), "after_item_count": len(req.Items),
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	result, err := r.GetStockDocument(ctx, replacementID)
	if err != nil {
		return nil, err
	}
	result["corrected_document_id"] = documentID
	return attachWorkItemTransition(result, transition), nil
}

func stockDocumentCoreChanged(current Record, currentItems []Record, req StockDocumentRequest) bool {
	if cleanID(current["project_id"]) != req.ProjectID || cleanOptionalText(current["movement_type"]) != req.MovementType ||
		cleanID(current["source_warehouse_id"]) != req.SourceWarehouseID || cleanID(current["target_warehouse_id"]) != req.TargetWarehouseID ||
		cleanOptionalText(current["reference_type"]) != req.ReferenceType || cleanID(current["reference_id"]) != req.ReferenceID || len(currentItems) != len(req.Items) {
		return true
	}
	for index, currentItem := range currentItems {
		requestedItem := req.Items[index]
		if len(requestedItem.StockUnits) > 0 || len(requestedItem.StockUnitAllocations) > 0 {
			return true
		}
		if cleanID(currentItem["material_id"]) != requestedItem.MaterialID || math.Abs(number(currentItem["quantity"])-requestedItem.Quantity) > 0.000001 ||
			cleanOptionalText(currentItem["unit"]) != requestedItem.Unit || cleanOptionalText(currentItem["reason"]) != requestedItem.Reason ||
			cleanOptionalText(currentItem["source_line_type"]) != requestedItem.SourceLineType || cleanID(currentItem["source_line_id"]) != requestedItem.SourceLineID {
			return true
		}
	}
	return false
}

func (r *Repository) reverseStockDocumentTx(ctx context.Context, tx *sql.Tx, document Record, actorID, actorName, reason string) error {
	documentID := cleanID(document["id"])
	movements, err := queryTx(ctx, tx, `SELECT * FROM stock_movements WHERE stock_document_id=$1 ORDER BY created_at,id FOR UPDATE`, documentID)
	if err != nil {
		return err
	}
	if len(movements) == 0 {
		return validationError("Phiếu kho không có biến động để điều chỉnh", Record{"document_id": documentID})
	}
	for _, movement := range movements {
		movementType := cleanOptionalText(movement["movement_type"])
		projectID := cleanID(movement["project_id"])
		warehouseID := cleanID(movement["warehouse_id"])
		targetWarehouseID := cleanID(movement["target_warehouse_id"])
		materialID := cleanID(movement["material_id"])
		quantity := number(movement["quantity"])
		unit := cleanOptionalText(movement["unit"])
		correctionReason := fmt.Sprintf("Đảo phiếu %s: %s", document["document_code"], reason)
		referenceType := cleanOptionalText(movement["reference_type"])
		referenceID := cleanID(movement["reference_id"])
		switch movementType {
		case "receive":
			_, err = createStockMovementTx(ctx, tx, StockActionRequest{ProjectID: projectID, WarehouseID: warehouseID, MaterialID: materialID, MovementType: "adjustment_out", Quantity: quantity, Unit: unit, Reason: correctionReason, ReferenceType: referenceType, ReferenceID: referenceID, StockDocumentID: documentID}, actorID, actorName)
		case "issue":
			_, err = createStockMovementTx(ctx, tx, StockActionRequest{ProjectID: projectID, WarehouseID: warehouseID, MaterialID: materialID, MovementType: "adjustment_in", Quantity: quantity, Unit: unit, Reason: correctionReason, ReferenceType: "stock_document_correction", ReferenceID: documentID, StockDocumentID: documentID}, actorID, actorName)
			if err == nil {
				err = adjustSiteIssuedQuantityTx(ctx, tx, projectID, materialID, -quantity, unit, correctionReason, documentID, actorID, actorName)
			}
		case "transfer":
			_, err = createStockMovementTx(ctx, tx, StockActionRequest{ProjectID: projectID, WarehouseID: targetWarehouseID, TargetWarehouseID: warehouseID, MaterialID: materialID, MovementType: "transfer", Quantity: quantity, Unit: unit, Reason: correctionReason, ReferenceType: "stock_document_correction", ReferenceID: documentID, StockDocumentID: documentID}, actorID, actorName)
		case "return_to_warehouse":
			_, err = createStockMovementTx(ctx, tx, StockActionRequest{ProjectID: projectID, WarehouseID: warehouseID, MaterialID: materialID, MovementType: "adjustment_out", Quantity: quantity, Unit: unit, Reason: correctionReason, ReferenceType: "stock_document_correction", ReferenceID: documentID, StockDocumentID: documentID}, actorID, actorName)
			if err == nil {
				err = adjustSiteIssuedQuantityTx(ctx, tx, projectID, materialID, quantity, unit, correctionReason, documentID, actorID, actorName)
			}
		case "return_to_subcontractor":
			err = adjustSiteIssuedQuantityTx(ctx, tx, projectID, materialID, quantity, unit, correctionReason, documentID, actorID, actorName)
		default:
			err = validationError("Loại biến động không hỗ trợ điều chỉnh phiếu", Record{"movement_type": movementType})
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func adjustSiteIssuedQuantityTx(ctx context.Context, tx *sql.Tx, projectID, materialID string, delta float64, unit, reason, sourceID, actorID, actorName string) error {
	if projectID == "" {
		return validationError("Phiếu điều chỉnh thiếu công trình", Record{"material_id": materialID})
	}
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1)::bigint)`, "site-material:"+projectID+":"+materialID); err != nil {
		return err
	}
	current, err := queryOneRecord(ctx, tx, `SELECT * FROM site_material_balances WHERE project_id=$1 AND material_id=$2 FOR UPDATE`, projectID, materialID)
	if err != nil {
		return validationError("Không tìm thấy số dư vật tư tại công trình", Record{"project_id": projectID, "material_id": materialID})
	}
	previousIssued := number(current["issued_quantity"])
	targetIssued := previousIssued + delta
	usedQuantity := number(current["used_quantity"])
	wasteQuantity := number(current["waste_quantity"])
	if targetIssued < -0.000001 {
		return validationError("Không thể điều chỉnh phiếu vì số dư công trình không đủ", Record{"issued_quantity": previousIssued, "target_issued_quantity": targetIssued})
	}
	if targetIssued < 0 {
		targetIssued = 0
	}
	updated, err := queryOneRecord(ctx, tx, `
		UPDATE site_material_balances
		SET issued_quantity=$3::numeric,
			surplus_quantity=GREATEST($3::numeric-used_quantity-waste_quantity,0),
			surplus_status=CASE
				WHEN GREATEST($3::numeric-used_quantity-waste_quantity,0)>0.000001 THEN CASE WHEN surplus_status='partial' THEN 'partial' ELSE 'pending' END
				ELSE 'handled'
			END,
			updated_at=now()
		WHERE project_id=$1 AND material_id=$2
		RETURNING *
	`, projectID, materialID, targetIssued)
	if err != nil {
		return err
	}
	adjustmentType := "issued_increase_correction"
	if delta < 0 {
		adjustmentType = "issued_decrease_correction"
	}
	quantity := math.Abs(delta)
	adjustment, err := queryOneRecord(ctx, tx, `
		INSERT INTO site_material_adjustments (
			project_id,material_id,adjustment_type,quantity,previous_issued_quantity,target_issued_quantity,
			unit,reason,source_type,source_id,created_by
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,'stock_document_correction',$9,NULLIF($10,'')::uuid)
		RETURNING *
	`, projectID, materialID, adjustmentType, quantity, previousIssued, targetIssued, unit, reason, sourceID, actorID)
	if err != nil {
		return err
	}
	return addAuditTx(ctx, tx, projectID, actorID, actorName, "site_material_issued_corrected", "site_material_adjustment", cleanID(adjustment["id"]), reason, Record{
		"material_id": materialID, "before_issued_quantity": previousIssued, "target_issued_quantity": targetIssued,
		"used_quantity": usedQuantity, "waste_quantity": wasteQuantity, "surplus_quantity": updated["surplus_quantity"], "source_id": sourceID,
	})
}

func validateStockDocumentCorrectionSiteBalancesTx(ctx context.Context, tx *sql.Tx, current Record, req StockDocumentRequest) error {
	pairs := map[string][2]string{}
	currentProjectID := cleanID(current["project_id"])
	if currentProjectID != "" {
		rows, err := queryTx(ctx, tx, `SELECT DISTINCT material_id::text AS material_id FROM stock_movements WHERE stock_document_id=$1`, cleanID(current["id"]))
		if err != nil {
			return err
		}
		for _, row := range rows {
			materialID := cleanID(row["material_id"])
			pairs[currentProjectID+":"+materialID] = [2]string{currentProjectID, materialID}
		}
	}
	if req.ProjectID != "" {
		for _, item := range req.Items {
			pairs[req.ProjectID+":"+item.MaterialID] = [2]string{req.ProjectID, item.MaterialID}
		}
	}
	for _, pair := range pairs {
		rows, err := queryTx(ctx, tx, `SELECT issued_quantity,used_quantity,waste_quantity FROM site_material_balances WHERE project_id=$1 AND material_id=$2 FOR UPDATE`, pair[0], pair[1])
		if err != nil {
			return err
		}
		if len(rows) > 0 && number(rows[0]["issued_quantity"])+0.000001 < number(rows[0]["used_quantity"])+number(rows[0]["waste_quantity"]) {
			return validationError("Không thể điều chỉnh phiếu vì khối lượng đã dùng/hao hụt vượt khối lượng xuất sau điều chỉnh", Record{"project_id": pair[0], "material_id": pair[1], "issued_quantity": rows[0]["issued_quantity"], "used_quantity": rows[0]["used_quantity"], "waste_quantity": rows[0]["waste_quantity"]})
		}
	}
	return nil
}

func (r *Repository) AdjustSiteMaterialUsage(ctx context.Context, projectID, materialID string, req SiteMaterialUsageAdjustmentRequest, actorID, actorName string) (Record, error) {
	req.Reason = strings.TrimSpace(req.Reason)
	if projectID == "" || materialID == "" {
		return nil, validationError("Công trình và vật tư là bắt buộc")
	}
	if math.IsNaN(req.TargetUsedQuantity) || math.IsInf(req.TargetUsedQuantity, 0) || req.TargetUsedQuantity < 0 {
		return nil, validationError("Khối lượng đã dùng phải lớn hơn hoặc bằng 0")
	}
	if req.Reason == "" {
		return nil, validationError("Lý do điều chỉnh lượng đã dùng là bắt buộc")
	}
	if len([]rune(req.Reason)) > 2000 {
		return nil, validationError("Lý do điều chỉnh không được vượt quá 2000 ký tự")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1)::bigint)`, "site-material:"+projectID+":"+materialID); err != nil {
		return nil, err
	}
	current, err := queryOneRecord(ctx, tx, `
		SELECT smb.*,m.code,m.name AS material_name,m.unit,m.category,m.tracking_mode
		FROM site_material_balances smb JOIN materials m ON m.id=smb.material_id AND m.active
		WHERE smb.project_id=$1 AND smb.material_id=$2 FOR UPDATE OF smb
	`, projectID, materialID)
	if err != nil {
		return nil, validationError("Không tìm thấy số dư vật tư tại công trình", Record{"project_id": projectID, "material_id": materialID})
	}
	if cleanOptionalText(current["tracking_mode"]) == "physical_unit" {
		return nil, validationError("Vật tư theo dõi từng Đoạn/Cuộn/Cây/Tấm phải ghi nhận sử dụng từ đầu việc và chọn đúng đơn vị vật lý", Record{"project_id": projectID, "material_id": materialID})
	}
	issuedQuantity := number(current["issued_quantity"])
	previousUsed := number(current["used_quantity"])
	wasteQuantity := number(current["waste_quantity"])
	if req.TargetUsedQuantity+wasteQuantity > issuedQuantity+0.000001 {
		return nil, validationError("Khối lượng đã dùng và hao hụt không được vượt khối lượng đã xuất công trình", Record{"issued_quantity": issuedQuantity, "waste_quantity": wasteQuantity, "target_used_quantity": req.TargetUsedQuantity})
	}
	delta := req.TargetUsedQuantity - previousUsed
	if math.Abs(delta) <= 0.000001 {
		return nil, validationError("Khối lượng đã dùng không thay đổi", Record{"used_quantity": previousUsed})
	}
	updated, err := queryOneRecord(ctx, tx, `
		UPDATE site_material_balances
		SET used_quantity=$3::numeric,
			surplus_quantity=GREATEST(issued_quantity-$3::numeric-waste_quantity,0),
			surplus_status=CASE
				WHEN GREATEST(issued_quantity-$3::numeric-waste_quantity,0)>0.000001 THEN CASE WHEN surplus_status='partial' THEN 'partial' ELSE 'pending' END
				ELSE 'handled'
			END,
			updated_at=now()
		WHERE project_id=$1 AND material_id=$2
		RETURNING *
	`, projectID, materialID, req.TargetUsedQuantity)
	if err != nil {
		return nil, err
	}
	adjustmentType := "usage_increase"
	if delta < 0 {
		adjustmentType = "usage_decrease"
	}
	adjustment, err := queryOneRecord(ctx, tx, `
		INSERT INTO site_material_adjustments (
			project_id,material_id,adjustment_type,quantity,previous_used_quantity,target_used_quantity,unit,reason,created_by
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,NULLIF($9,'')::uuid)
		RETURNING *
	`, projectID, materialID, adjustmentType, math.Abs(delta), previousUsed, req.TargetUsedQuantity, current["unit"], req.Reason, actorID)
	if err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "site_material_usage_adjusted", "site_material_adjustment", cleanID(adjustment["id"]), fmt.Sprintf("Điều chỉnh lượng đã dùng của %s", current["material_name"]), Record{
		"material_id": materialID, "before_used_quantity": previousUsed, "target_used_quantity": req.TargetUsedQuantity,
		"issued_quantity": issuedQuantity, "surplus_quantity": updated["surplus_quantity"], "reason": req.Reason,
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.QueryOne(ctx, `
		SELECT smb.*,m.code,m.name AS material_name,m.unit,m.category
		FROM site_material_balances smb JOIN materials m ON m.id=smb.material_id
		WHERE smb.project_id=$1 AND smb.material_id=$2
	`, projectID, materialID)
}

func (r *Repository) StockPositions(ctx context.Context, userID, warehouseID string) ([]Record, error) {
	scopeSQL, scopeArgs, err := r.ProjectAccessFilter(ctx, userID, "p", 2)
	if err != nil {
		return nil, err
	}
	args := append([]any{cleanID(warehouseID)}, scopeArgs...)
	return r.Query(ctx, fmt.Sprintf(`
		WITH visible_movements AS (
			SELECT sm.*
			FROM stock_movements sm
			WHERE sm.project_id IS NULL OR EXISTS (
				SELECT 1 FROM projects p
				WHERE p.id=sm.project_id AND p.status <> 'cancelled' AND %s
			)
		), ledger AS (
			SELECT warehouse_id, material_id,
				CASE
					WHEN movement_type IN ('receive','return_to_warehouse','adjustment_in') THEN quantity
					WHEN movement_type IN ('issue','transfer','loss','adjustment_out') THEN -quantity
					ELSE 0
				END AS signed_quantity
			FROM visible_movements
			UNION ALL
			SELECT target_warehouse_id, material_id, quantity
			FROM visible_movements
			WHERE movement_type='transfer' AND target_warehouse_id IS NOT NULL
		)
		SELECT w.id AS warehouse_id, w.name AS warehouse_name, w.type AS warehouse_type, w.location,
			m.id AS material_id, m.code, m.name, m.unit, m.category, m.tracking_mode, m.form_factor,
			COALESCE(SUM(ledger.signed_quantity),0) AS stock_quantity,
			COALESCE(physical.available_quantity,0) AS physical_available_quantity,
			COALESCE(physical.quarantined_quantity,0) AS physical_quarantined_quantity,
			COALESCE(physical.unit_count,0) AS physical_unit_count
		FROM ledger
		JOIN warehouses w ON w.id=ledger.warehouse_id AND w.active
		JOIN materials m ON m.id=ledger.material_id AND m.active
		LEFT JOIN LATERAL (
			SELECT SUM(quantity) FILTER (WHERE status='available' AND condition='usable') AS available_quantity,
				SUM(quantity) FILTER (WHERE status='quarantined' OR condition<>'usable') AS quarantined_quantity,
				COUNT(*) FILTER (WHERE status IN ('available','quarantined')) AS unit_count
			FROM material_stock_units unit
			WHERE unit.material_id=m.id AND unit.warehouse_id=w.id AND unit.location_type='warehouse'
		) physical ON true
		WHERE ($1::text='' OR w.id=$1::uuid)
		GROUP BY w.id, m.id, physical.available_quantity, physical.quarantined_quantity, physical.unit_count
		ORDER BY w.type, w.name, m.category, m.name
	`, scopeSQL), args...)
}

func (r *Repository) StockMovements(ctx context.Context, userID, projectID, warehouseID string) ([]Record, error) {
	scopeSQL, scopeArgs, err := r.ProjectAccessFilter(ctx, userID, "p", 3)
	if err != nil {
		return nil, err
	}
	args := append([]any{cleanID(projectID), cleanID(warehouseID)}, scopeArgs...)
	return r.Query(ctx, fmt.Sprintf(`
		SELECT sm.*, m.code AS material_code, m.name AS material_name,
			w.name AS warehouse_name, tw.name AS target_warehouse_name,
			p.name AS project_name,u.full_name AS created_by_name,
			COALESCE(u.deleted_at IS NOT NULL,false) AS created_by_account_deleted,
			sd.document_code
		FROM stock_movements sm
		JOIN materials m ON m.id=sm.material_id AND m.active
		JOIN warehouses w ON w.id=sm.warehouse_id
		LEFT JOIN warehouses tw ON tw.id=sm.target_warehouse_id
		LEFT JOIN projects p ON p.id=sm.project_id
		LEFT JOIN users u ON u.id=sm.created_by
		LEFT JOIN stock_documents sd ON sd.id=sm.stock_document_id
		WHERE ($1::text='' OR sm.project_id=$1::uuid)
		  AND ($2::text='' OR sm.warehouse_id=$2::uuid OR sm.target_warehouse_id=$2::uuid)
		  AND (sm.project_id IS NULL OR (p.status <> 'cancelled' AND %s))
		ORDER BY sm.created_at DESC, sm.id
	`, scopeSQL), args...)
}

func (r *Repository) ListStockDocuments(ctx context.Context, userID, projectID string) ([]Record, error) {
	scopeSQL, scopeArgs, err := r.ProjectAccessFilter(ctx, userID, "p", 2)
	if err != nil {
		return nil, err
	}
	args := append([]any{cleanID(projectID)}, scopeArgs...)
	return r.Query(ctx, fmt.Sprintf(`
		SELECT sd.*, sw.name AS source_warehouse_name, tw.name AS target_warehouse_name,
			p.name AS project_name,u.full_name AS created_by_name,
			COALESCE(u.deleted_at IS NOT NULL,false) AS created_by_account_deleted,
			COALESCE(items.item_count,0) AS item_count,
			COALESCE(items.total_quantity,0) AS total_quantity
		FROM stock_documents sd
		LEFT JOIN warehouses sw ON sw.id=sd.source_warehouse_id
		LEFT JOIN warehouses tw ON tw.id=sd.target_warehouse_id
		LEFT JOIN projects p ON p.id=sd.project_id
		LEFT JOIN users u ON u.id=sd.created_by
		LEFT JOIN LATERAL (
			SELECT COUNT(*) AS item_count, SUM(quantity) AS total_quantity
			FROM stock_document_items sdi WHERE sdi.document_id=sd.id
		) items ON true
		WHERE sd.status <> 'cancelled'
		  AND ($1::text='' OR sd.project_id=$1::uuid)
		  AND (sd.project_id IS NULL OR (p.status <> 'cancelled' AND %s))
		ORDER BY sd.document_date DESC, sd.created_at DESC, sd.id
	`, scopeSQL), args...)
}

func (r *Repository) GetStockDocument(ctx context.Context, documentID string) (Record, error) {
	document, err := r.QueryOne(ctx, `
		SELECT sd.*, sw.name AS source_warehouse_name, tw.name AS target_warehouse_name,
			p.name AS project_name,u.full_name AS created_by_name,
			COALESCE(u.deleted_at IS NOT NULL,false) AS created_by_account_deleted
		FROM stock_documents sd
		LEFT JOIN warehouses sw ON sw.id=sd.source_warehouse_id
		LEFT JOIN warehouses tw ON tw.id=sd.target_warehouse_id
		LEFT JOIN projects p ON p.id=sd.project_id
		LEFT JOIN users u ON u.id=sd.created_by
		WHERE sd.id=$1 AND sd.status <> 'cancelled'
	`, documentID)
	if err != nil {
		return nil, err
	}
	items, err := r.Query(ctx, `
		SELECT sdi.*, m.code AS material_code, m.name AS material_name, m.tracking_mode, m.form_factor,
			sm.id AS movement_id, sm.movement_code,
			COALESCE((
				SELECT jsonb_agg(jsonb_build_object(
					'stock_unit_id', allocation.stock_unit_id,
					'unit_code', unit.unit_code,
					'quantity', allocation.quantity,
					'condition', unit.condition,
					'status', unit.status
				) ORDER BY allocation.created_at, allocation.id)
				FROM stock_movement_unit_allocations allocation
				JOIN material_stock_units unit ON unit.id=allocation.stock_unit_id
				WHERE allocation.stock_document_item_id=sdi.id
			), '[]'::jsonb) AS stock_unit_allocations
		FROM stock_document_items sdi
		JOIN materials m ON m.id=sdi.material_id
		LEFT JOIN LATERAL (
			SELECT candidate.id, candidate.movement_code
			FROM stock_movements candidate
			WHERE candidate.stock_document_id=sdi.document_id
			  AND candidate.material_id=sdi.material_id
			  AND (
				(candidate.reference_type='stock_document_item' AND candidate.reference_id=sdi.id)
				OR (sdi.source_line_id IS NOT NULL AND candidate.reference_id=sdi.source_line_id)
			  )
			ORDER BY candidate.created_at, candidate.id
			LIMIT 1
		) sm ON true
		WHERE sdi.document_id=$1
		ORDER BY sdi.created_at, sdi.id
	`, documentID)
	if err != nil {
		return nil, err
	}
	document["items"] = items
	return document, nil
}

func (r *Repository) WarehouseReceiptOptions(ctx context.Context, projectID string) ([]Record, error) {
	return r.Query(ctx, `
		SELECT iar.id AS incoming_acceptance_id, iar.acceptance_code, iar.procurement_request_id,
			pr.request_code, iar.supplier_name, iar.received_at, iar.result, iar.warehouse_id,
			iai.id AS incoming_acceptance_item_id, iai.material_id, iai.material_code, iai.material_name,
			iai.accepted_quantity, iai.unit,
			LEAST(iai.accepted_quantity,COALESCE(received.received_quantity,0)+COALESCE(legacy_stock.received_quantity,0)) AS stock_received_quantity,
			GREATEST(iai.accepted_quantity-COALESCE(received.received_quantity,0)-COALESCE(legacy_stock.received_quantity,0),0) AS remaining_quantity
		FROM incoming_acceptance_records iar
		JOIN incoming_acceptance_items iai ON iai.incoming_acceptance_id=iar.id
		LEFT JOIN procurement_request_items pri ON pri.id=iai.procurement_request_item_id
		LEFT JOIN procurement_requests pr ON pr.id=iar.procurement_request_id
		LEFT JOIN LATERAL (
			SELECT SUM(CASE WHEN sm.movement_type='receive' THEN sm.quantity WHEN sm.movement_type='adjustment_out' THEN -sm.quantity ELSE 0 END) AS received_quantity
			FROM stock_movements sm
			WHERE sm.movement_type IN ('receive','adjustment_out')
			  AND sm.reference_type='incoming_acceptance_item'
			  AND sm.reference_id=iai.id
		) received ON true
		LEFT JOIN LATERAL (
			SELECT SUM(CASE WHEN sm.movement_type='receive' THEN sm.quantity WHEN sm.movement_type='adjustment_out' THEN -sm.quantity ELSE 0 END) AS received_quantity
			FROM stock_movements sm
			WHERE sm.movement_type IN ('receive','adjustment_out')
			  AND sm.reference_type='boq_item'
			  AND sm.reference_id=pri.boq_item_id
		) legacy_stock ON true
		WHERE iar.project_id=$1 AND iar.status='submitted' AND iai.material_id IS NOT NULL
		  AND iai.accepted_quantity-COALESCE(received.received_quantity,0)-COALESCE(legacy_stock.received_quantity,0) > 0.000001
		ORDER BY iar.received_at DESC NULLS LAST, iar.created_at DESC, iai.item_index
	`, projectID)
}

func (r *Repository) ProjectMaterialBalances(ctx context.Context, projectID string) ([]Record, error) {
	return r.Query(ctx, `
		SELECT smb.*, m.code, m.name AS material_name, m.unit, m.category, m.tracking_mode, m.form_factor,
			COALESCE(physical.available_quantity,0) AS physical_available_quantity,
			COALESCE(physical.unit_count,0) AS physical_unit_count
		FROM site_material_balances smb
		JOIN materials m ON m.id=smb.material_id
		LEFT JOIN LATERAL (
			SELECT SUM(quantity) FILTER (WHERE status='issued') AS available_quantity,
				COUNT(*) FILTER (WHERE status='issued') AS unit_count
			FROM material_stock_units unit
			WHERE unit.material_id=m.id AND unit.project_id=smb.project_id AND unit.location_type='project'
		) physical ON true
		WHERE smb.project_id=$1 AND m.active
		ORDER BY m.category, m.name
	`, projectID)
}

func (r *Repository) ListActiveMaterials(ctx context.Context) ([]Record, error) {
	return r.Query(ctx, `
		SELECT m.id, m.code, m.name, m.unit, m.category, m.default_supplier_partner_id, m.default_supplier,
			p.name AS default_supplier_partner_name, p.tax_code AS default_supplier_tax_code,
			m.standard_price, m.active, m.tracking_mode, m.form_factor,
			m.physical_tracking_initialized_at, m.physical_tracking_initialized_by, m.created_at,
			(
				EXISTS (SELECT 1 FROM stock_movements row WHERE row.material_id=m.id)
				OR EXISTS (SELECT 1 FROM stock_document_items row WHERE row.material_id=m.id)
				OR EXISTS (SELECT 1 FROM site_material_balances row WHERE row.material_id=m.id)
				OR EXISTS (SELECT 1 FROM boq_items row WHERE row.material_id=m.id)
				OR EXISTS (SELECT 1 FROM procurement_request_items row WHERE row.material_id=m.id)
				OR EXISTS (SELECT 1 FROM supplier_quotation_items row WHERE row.material_id=m.id)
				OR EXISTS (SELECT 1 FROM incoming_acceptance_items row WHERE row.material_id=m.id)
				OR EXISTS (SELECT 1 FROM material_before_after_snapshot_items row WHERE row.material_id=m.id)
				OR EXISTS (SELECT 1 FROM site_surplus_handlings row WHERE row.material_id=m.id)
			) AS identity_locked
		FROM materials m
		LEFT JOIN partners p ON p.id=m.default_supplier_partner_id
		WHERE m.active
		ORDER BY m.category, m.name, m.code
	`)
}

func (r *Repository) CreateMaterial(ctx context.Context, body Record, actorID, actorName string) (Record, error) {
	return r.insertWithAudit(ctx, "materials", materialColumns(), body, func(row Record) auditEvent {
		return auditEvent{
			ActorID: actorID, ActorName: actorName, EventType: "material_created", TargetType: "material",
			Description: fmt.Sprintf("Tạo vật tư %s", row["name"]),
			Metadata:    Record{"code": row["code"], "unit": row["unit"], "category": row["category"]},
		}
	})
}

func (r *Repository) UpdateMaterial(ctx context.Context, materialID string, body Record, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	current, err := queryOneRecord(ctx, tx, `SELECT * FROM materials WHERE id=$1 AND active FOR UPDATE`, materialID)
	if err != nil {
		return nil, err
	}
	identityLocked, err := materialOperationalUsageExistsTx(ctx, tx, materialID)
	if err != nil {
		return nil, err
	}
	if identityLocked && (cleanOptionalText(body["code"]) != cleanOptionalText(current["code"]) || cleanOptionalText(body["unit"]) != cleanOptionalText(current["unit"]) || cleanOptionalText(body["tracking_mode"]) != cleanOptionalText(current["tracking_mode"]) || cleanOptionalText(body["form_factor"]) != cleanOptionalText(current["form_factor"])) {
		return nil, validationError("Mã, ĐVT và chế độ theo dõi vật tư đã khóa vì vật tư đã phát sinh nghiệp vụ", Record{"material_id": materialID, "identity_locked": true})
	}
	updated, err := updateRecord(ctx, tx, "materials", materialColumns(), materialID, body)
	if err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, "", actorID, actorName, "material_updated", "material", materialID, fmt.Sprintf("Cập nhật vật tư %s", updated["name"]), Record{
		"before":          Record{"code": current["code"], "name": current["name"], "unit": current["unit"], "category": current["category"], "default_supplier_partner_id": current["default_supplier_partner_id"], "default_supplier": current["default_supplier"], "tracking_mode": current["tracking_mode"], "form_factor": current["form_factor"]},
		"after":           Record{"code": updated["code"], "name": updated["name"], "unit": updated["unit"], "category": updated["category"], "default_supplier_partner_id": updated["default_supplier_partner_id"], "default_supplier": updated["default_supplier"], "tracking_mode": updated["tracking_mode"], "form_factor": updated["form_factor"]},
		"identity_locked": identityLocked,
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	updated["identity_locked"] = identityLocked
	return updated, nil
}

func materialOperationalUsageExistsTx(ctx context.Context, tx *sql.Tx, materialID string) (bool, error) {
	var exists bool
	err := tx.QueryRowContext(ctx, `
		SELECT
			EXISTS (SELECT 1 FROM stock_movements row WHERE row.material_id=$1)
			OR EXISTS (SELECT 1 FROM stock_document_items row WHERE row.material_id=$1)
			OR EXISTS (SELECT 1 FROM site_material_balances row WHERE row.material_id=$1)
			OR EXISTS (SELECT 1 FROM boq_items row WHERE row.material_id=$1)
			OR EXISTS (SELECT 1 FROM procurement_request_items row WHERE row.material_id=$1)
			OR EXISTS (SELECT 1 FROM supplier_quotation_items row WHERE row.material_id=$1)
			OR EXISTS (SELECT 1 FROM incoming_acceptance_items row WHERE row.material_id=$1)
			OR EXISTS (SELECT 1 FROM material_before_after_snapshot_items row WHERE row.material_id=$1)
			OR EXISTS (SELECT 1 FROM site_surplus_handlings row WHERE row.material_id=$1)
	`, materialID).Scan(&exists)
	return exists, err
}

func (r *Repository) ListActiveWarehouses(ctx context.Context) ([]Record, error) {
	return r.Query(ctx, `
		SELECT id, name, location, type, active, created_at
		FROM warehouses
		WHERE active
		ORDER BY type, name
	`)
}

func (r *Repository) CreateWarehouse(ctx context.Context, body Record) (Record, error) {
	return r.Insert(ctx, "warehouses", warehouseColumns(), body)
}
