package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

const stockUnitQuantityTolerance = 0.000001

func (r *Repository) ListMaterialStockUnits(ctx context.Context, userID, materialID, warehouseID, projectID, status, query string) ([]Record, error) {
	scopeSQL, scopeArgs, err := r.ProjectAccessFilter(ctx, userID, "p", 6)
	if err != nil {
		return nil, err
	}
	args := append([]any{cleanID(materialID), cleanID(warehouseID), cleanID(projectID), strings.TrimSpace(status), strings.TrimSpace(query)}, scopeArgs...)
	return r.Query(ctx, fmt.Sprintf(`
		SELECT msu.*, m.code AS material_code, m.name AS material_name, m.tracking_mode, m.form_factor,
			w.name AS warehouse_name, p.name AS project_name,
			parent.unit_code AS parent_unit_code, root.unit_code AS root_unit_code,
			creator.full_name AS created_by_name, updater.full_name AS updated_by_name
		FROM material_stock_units msu
		JOIN materials m ON m.id=msu.material_id AND m.active
		LEFT JOIN warehouses w ON w.id=msu.warehouse_id
		LEFT JOIN projects p ON p.id=msu.project_id
		LEFT JOIN material_stock_units parent ON parent.id=msu.parent_unit_id
		LEFT JOIN material_stock_units root ON root.id=msu.root_unit_id
		LEFT JOIN users creator ON creator.id=msu.created_by
		LEFT JOIN users updater ON updater.id=msu.updated_by
		WHERE ($1::text='' OR msu.material_id=$1::uuid)
		  AND ($2::text='' OR msu.warehouse_id=$2::uuid)
		  AND ($3::text='' OR msu.project_id=$3::uuid)
		  AND ($4::text='' OR msu.status=$4)
		  AND ($5::text='' OR msu.unit_code ILIKE '%%'||$5||'%%' OR m.code ILIKE '%%'||$5||'%%' OR m.name ILIKE '%%'||$5||'%%')
		  AND (msu.project_id IS NULL OR (p.status <> 'cancelled' AND %s))
		ORDER BY CASE msu.status WHEN 'available' THEN 0 WHEN 'issued' THEN 1 WHEN 'quarantined' THEN 2 ELSE 3 END,
			msu.updated_at DESC, msu.id
	`, scopeSQL), args...)
}

func (r *Repository) GetMaterialStockUnit(ctx context.Context, stockUnitID string) (Record, error) {
	row, err := r.QueryOne(ctx, `
		SELECT msu.*, m.code AS material_code, m.name AS material_name, m.tracking_mode, m.form_factor,
			w.name AS warehouse_name, p.name AS project_name,
			parent.unit_code AS parent_unit_code, root.unit_code AS root_unit_code,
			creator.full_name AS created_by_name, updater.full_name AS updated_by_name
		FROM material_stock_units msu
		JOIN materials m ON m.id=msu.material_id
		LEFT JOIN warehouses w ON w.id=msu.warehouse_id
		LEFT JOIN projects p ON p.id=msu.project_id
		LEFT JOIN material_stock_units parent ON parent.id=msu.parent_unit_id
		LEFT JOIN material_stock_units root ON root.id=msu.root_unit_id
		LEFT JOIN users creator ON creator.id=msu.created_by
		LEFT JOIN users updater ON updater.id=msu.updated_by
		WHERE msu.id=$1
	`, cleanID(stockUnitID))
	if err != nil {
		return nil, err
	}
	events, err := r.Query(ctx, `
		SELECT event.*, actor.full_name AS actor_name,
			fw.name AS from_warehouse_name, tw.name AS to_warehouse_name,
			fp.name AS from_project_name, tp.name AS to_project_name
		FROM material_stock_unit_events event
		LEFT JOIN users actor ON actor.id=event.actor_id
		LEFT JOIN warehouses fw ON fw.id=event.from_warehouse_id
		LEFT JOIN warehouses tw ON tw.id=event.to_warehouse_id
		LEFT JOIN projects fp ON fp.id=event.from_project_id
		LEFT JOIN projects tp ON tp.id=event.to_project_id
		WHERE event.stock_unit_id=$1
		ORDER BY event.created_at DESC, event.id DESC
	`, cleanID(stockUnitID))
	if err != nil {
		return nil, err
	}
	row["events"] = events
	return row, nil
}

func (r *Repository) InitializeMaterialStockUnits(ctx context.Context, materialID string, req InitializeMaterialStockUnitsRequest, actorID, actorName string) (Record, error) {
	materialID = cleanID(materialID)
	req.FormFactor = strings.TrimSpace(req.FormFactor)
	if materialID == "" || !statusIn(req.FormFactor, "segment", "roll", "bar", "sheet") {
		return nil, validationError("Vật tư và dạng vật tư hợp lệ là bắt buộc")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1)::bigint)`, "material-tracking:"+materialID); err != nil {
		return nil, err
	}
	material, err := queryOneRecord(ctx, tx, `SELECT * FROM materials WHERE id=$1 AND active FOR UPDATE`, materialID)
	if err != nil {
		return nil, validationError("Vật tư không tồn tại hoặc đã ngưng hoạt động", Record{"material_id": materialID})
	}
	var existingCount int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM material_stock_units WHERE material_id=$1`, materialID).Scan(&existingCount); err != nil {
		return nil, err
	}
	if existingCount > 0 || cleanOptionalText(material["tracking_mode"]) == "physical_unit" {
		return nil, validationError("Vật tư đã được khởi tạo theo dõi đơn vị vật lý", Record{"material_id": materialID})
	}
	var siteQuantity float64
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(GREATEST(issued_quantity-used_quantity-waste_quantity,surplus_quantity,0)),0) FROM site_material_balances WHERE material_id=$1`, materialID).Scan(&siteQuantity); err != nil {
		return nil, err
	}
	if siteQuantity > stockUnitQuantityTolerance {
		return nil, validationError("Không thể khởi tạo khi vật tư còn tại công trình; cần đối soát và hoàn kho trước", Record{"site_quantity": siteQuantity})
	}
	positions, err := queryTx(ctx, tx, `
		WITH ledger AS (
			SELECT warehouse_id,
				CASE WHEN movement_type IN ('receive','return_to_warehouse','adjustment_in') THEN quantity
					WHEN movement_type IN ('issue','transfer','loss','adjustment_out') THEN -quantity ELSE 0 END AS signed_quantity
			FROM stock_movements WHERE material_id=$1
			UNION ALL
			SELECT target_warehouse_id, quantity FROM stock_movements
			WHERE material_id=$1 AND movement_type='transfer' AND target_warehouse_id IS NOT NULL
		)
		SELECT warehouse_id, COALESCE(SUM(signed_quantity),0) AS quantity
		FROM ledger WHERE warehouse_id IS NOT NULL GROUP BY warehouse_id HAVING ABS(SUM(signed_quantity)) > 0.000001
	`, materialID)
	if err != nil {
		return nil, err
	}
	expectedByWarehouse := map[string]float64{}
	for _, row := range positions {
		quantity := number(row["quantity"])
		if quantity < -stockUnitQuantityTolerance {
			return nil, validationError("Tồn kho tổng hợp đang âm; cần xử lý trước khi khởi tạo", Record{"warehouse_id": row["warehouse_id"], "quantity": quantity})
		}
		expectedByWarehouse[cleanID(row["warehouse_id"])] = quantity
	}
	actualByWarehouse := map[string]float64{}
	seenCodes := map[string]bool{}
	for index := range req.Units {
		unit := &req.Units[index]
		unit.WarehouseID = cleanID(unit.WarehouseID)
		if unit.WarehouseID == "" {
			return nil, validationError("Kho là bắt buộc cho đơn vị khởi tạo", Record{"unit_index": index})
		}
		if _, ok := expectedByWarehouse[unit.WarehouseID]; !ok {
			return nil, validationError("Kho khởi tạo không có tồn sổ của vật tư", Record{"unit_index": index, "warehouse_id": unit.WarehouseID})
		}
		if err := normalizeStockUnitInput(unit, cleanOptionalText(material["unit"]), index, seenCodes); err != nil {
			return nil, err
		}
		actualByWarehouse[unit.WarehouseID] += unit.Quantity
	}
	for warehouseID, expected := range expectedByWarehouse {
		if math.Abs(actualByWarehouse[warehouseID]-expected) > stockUnitQuantityTolerance {
			return nil, validationError("Tổng đơn vị vật lý phải khớp tồn sổ theo từng kho", Record{"warehouse_id": warehouseID, "stock_quantity": expected, "physical_quantity": actualByWarehouse[warehouseID]})
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE materials SET tracking_mode='physical_unit',form_factor=$2,physical_tracking_initialized_at=now(),physical_tracking_initialized_by=NULLIF($3,'')::uuid WHERE id=$1`, materialID, req.FormFactor, actorID); err != nil {
		return nil, err
	}
	for index, input := range req.Units {
		unit, err := createMaterialStockUnitTx(ctx, tx, materialID, "", "", "", "warehouse", input.WarehouseID, "", "", input, statusForStockUnitCondition(input.Condition), actorID)
		if err != nil {
			return nil, err
		}
		if _, err := createMaterialStockUnitEventTx(ctx, tx, cleanID(unit["id"]), "initialize", input.Quantity, 0, input.Quantity, "", "warehouse", "", input.WarehouseID, "", "", "material_tracking_initialization", materialID, actorID, Record{"unit_index": index}); err != nil {
			return nil, err
		}
	}
	if err := addAuditTx(ctx, tx, "", actorID, actorName, "material_physical_tracking_initialized", "material", materialID, fmt.Sprintf("Khởi tạo theo dõi đơn vị vật lý cho %s", material["name"]), Record{"form_factor": req.FormFactor, "unit_count": len(req.Units), "warehouse_count": len(expectedByWarehouse)}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.QueryOne(ctx, `SELECT * FROM materials WHERE id=$1`, materialID)
}

func materialTrackingTx(ctx context.Context, tx *sql.Tx, materialID string) (Record, error) {
	return queryOneRecord(ctx, tx, `SELECT id,code,name,unit,tracking_mode,form_factor FROM materials WHERE id=$1 AND active`, materialID)
}

func applyStockUnitMovementTx(ctx context.Context, tx *sql.Tx, req StockDocumentRequest, item StockDocumentItemRequest, documentItemID string, movement Record, actorID string) error {
	material, err := materialTrackingTx(ctx, tx, item.MaterialID)
	if err != nil {
		return err
	}
	if cleanOptionalText(material["tracking_mode"]) != "physical_unit" {
		if len(item.StockUnits) > 0 || len(item.StockUnitAllocations) > 0 {
			return validationError("Vật tư theo dõi tổng hợp không nhận danh sách đơn vị vật lý", Record{"material_id": item.MaterialID})
		}
		return nil
	}
	if req.MovementType == "receive" || req.MovementType == "adjustment_in" {
		return receiveMaterialStockUnitsTx(ctx, tx, req, item, documentItemID, movement, material, actorID)
	}
	return allocateMaterialStockUnitsTx(ctx, tx, req, item, documentItemID, movement, material, actorID)
}

func receiveMaterialStockUnitsTx(ctx context.Context, tx *sql.Tx, req StockDocumentRequest, item StockDocumentItemRequest, documentItemID string, movement, material Record, actorID string) error {
	if len(item.StockUnits) == 0 {
		return validationError("Vật tư theo dõi từng đơn vị cần khai báo Đoạn/Cuộn/Cây/Tấm khi nhập kho", Record{"material_id": item.MaterialID})
	}
	if len(item.StockUnitAllocations) > 0 {
		return validationError("Phiếu nhập mới không nhận phân bổ đơn vị đã tồn tại", Record{"material_id": item.MaterialID})
	}
	seenCodes := map[string]bool{}
	total := 0.0
	for index := range item.StockUnits {
		input := &item.StockUnits[index]
		if err := normalizeStockUnitInput(input, item.Unit, index, seenCodes); err != nil {
			return err
		}
		total += input.Quantity
	}
	if math.Abs(total-item.Quantity) > stockUnitQuantityTolerance {
		return validationError("Tổng số lượng các đơn vị vật lý phải bằng số lượng dòng phiếu", Record{"material_id": item.MaterialID, "line_quantity": item.Quantity, "physical_quantity": total})
	}
	warehouseID := req.TargetWarehouseID
	for index, input := range item.StockUnits {
		unit, err := createMaterialStockUnitTx(ctx, tx, item.MaterialID, "", "", documentItemID, "warehouse", warehouseID, "", "", input, statusForStockUnitCondition(input.Condition), actorID)
		if err != nil {
			return err
		}
		event, err := createMaterialStockUnitEventTx(ctx, tx, cleanID(unit["id"]), "receive", input.Quantity, 0, input.Quantity, "", "warehouse", "", warehouseID, "", "", "stock_movement", cleanID(movement["id"]), actorID, Record{"stock_document_item_id": documentItemID, "unit_index": index})
		if err != nil {
			return err
		}
		if err := createStockMovementUnitAllocationTx(ctx, tx, cleanID(movement["id"]), documentItemID, cleanID(unit["id"]), cleanID(event["id"]), input.Quantity); err != nil {
			return err
		}
	}
	_ = material
	return nil
}

func allocateMaterialStockUnitsTx(ctx context.Context, tx *sql.Tx, req StockDocumentRequest, item StockDocumentItemRequest, documentItemID string, movement, material Record, actorID string) error {
	if len(item.StockUnits) > 0 {
		return validationError("Phiếu xuất/chuyển/hoàn chỉ nhận phân bổ từ đơn vị vật lý đã tồn tại", Record{"material_id": item.MaterialID})
	}
	if len(item.StockUnitAllocations) == 0 {
		return validationError("Cần chọn đúng đơn vị vật lý cho dòng phiếu", Record{"material_id": item.MaterialID})
	}
	total := 0.0
	seen := map[string]bool{}
	for index := range item.StockUnitAllocations {
		allocation := &item.StockUnitAllocations[index]
		allocation.StockUnitID = cleanID(allocation.StockUnitID)
		allocation.ResultUnitCode = strings.TrimSpace(allocation.ResultUnitCode)
		allocation.Condition = strings.TrimSpace(allocation.Condition)
		allocation.CustodyLocation = strings.TrimSpace(allocation.CustodyLocation)
		if allocation.StockUnitID == "" || allocation.Quantity <= 0 || math.IsNaN(allocation.Quantity) || math.IsInf(allocation.Quantity, 0) {
			return validationError("Phân bổ đơn vị vật lý không hợp lệ", Record{"allocation_index": index})
		}
		if seen[allocation.StockUnitID] {
			return validationError("Một đơn vị vật lý không được lặp trên cùng dòng phiếu", Record{"allocation_index": index, "stock_unit_id": allocation.StockUnitID})
		}
		seen[allocation.StockUnitID] = true
		if err := validateStockUnitDimensions(allocation.LengthMM, allocation.WidthMM, allocation.ThicknessMM, index); err != nil {
			return err
		}
		if err := validateStockUnitDimensions(allocation.RemainingLengthMM, allocation.RemainingWidthMM, allocation.RemainingThicknessMM, index); err != nil {
			return err
		}
		if allocation.Condition != "" && !statusIn(allocation.Condition, "usable", "damaged", "mixed", "unknown") {
			return validationError("Tình trạng đơn vị vật lý không hợp lệ", Record{"allocation_index": index, "condition": allocation.Condition})
		}
		total += allocation.Quantity
	}
	if math.Abs(total-item.Quantity) > stockUnitQuantityTolerance {
		return validationError("Tổng phân bổ đơn vị vật lý phải bằng số lượng dòng phiếu", Record{"material_id": item.MaterialID, "line_quantity": item.Quantity, "allocated_quantity": total})
	}
	for index, allocation := range item.StockUnitAllocations {
		if err := moveMaterialStockUnitTx(ctx, tx, req, item, documentItemID, movement, material, allocation, index, actorID); err != nil {
			return err
		}
	}
	return nil
}

func moveMaterialStockUnitTx(ctx context.Context, tx *sql.Tx, req StockDocumentRequest, item StockDocumentItemRequest, documentItemID string, movement, material Record, allocation StockUnitAllocationRequest, index int, actorID string) error {
	unit, err := queryOneRecord(ctx, tx, `SELECT * FROM material_stock_units WHERE id=$1 FOR UPDATE`, allocation.StockUnitID)
	if err != nil {
		return validationError("Không tìm thấy đơn vị vật tư vật lý", Record{"allocation_index": index, "stock_unit_id": allocation.StockUnitID})
	}
	if cleanID(unit["material_id"]) != item.MaterialID || cleanOptionalText(unit["unit"]) != item.Unit {
		return validationError("Đơn vị vật lý không khớp vật tư hoặc ĐVT của dòng phiếu", Record{"allocation_index": index, "stock_unit_id": allocation.StockUnitID})
	}
	beforeQuantity := number(unit["quantity"])
	if allocation.Quantity > beforeQuantity+stockUnitQuantityTolerance {
		return validationError("Số lượng phân bổ vượt phần còn lại của đơn vị vật lý", Record{"allocation_index": index, "stock_unit_id": allocation.StockUnitID, "available": beforeQuantity, "quantity": allocation.Quantity})
	}
	if statusIn(req.MovementType, "issue", "transfer", "adjustment_out") {
		if cleanOptionalText(unit["location_type"]) != "warehouse" || cleanID(unit["warehouse_id"]) != req.SourceWarehouseID || cleanOptionalText(unit["status"]) != "available" || cleanOptionalText(unit["condition"]) != "usable" {
			return validationError("Chỉ đơn vị dùng được tại đúng kho nguồn mới được xuất/chuyển", Record{"allocation_index": index, "stock_unit_id": allocation.StockUnitID})
		}
	} else if statusIn(req.MovementType, "return_to_warehouse", "return_to_subcontractor") {
		if cleanOptionalText(unit["location_type"]) != "project" || cleanID(unit["project_id"]) != req.ProjectID || cleanOptionalText(unit["status"]) != "issued" {
			return validationError("Chỉ đơn vị đang ở đúng công trình mới được hoàn/trả", Record{"allocation_index": index, "stock_unit_id": allocation.StockUnitID})
		}
	} else {
		return validationError("Loại biến động chưa hỗ trợ đơn vị vật lý", Record{"movement_type": req.MovementType})
	}

	toLocation, toWarehouseID, toProjectID, toStatus := "", "", "", ""
	eventType := req.MovementType
	switch req.MovementType {
	case "issue":
		toLocation, toProjectID, toStatus = "project", req.ProjectID, "issued"
	case "transfer":
		toLocation, toWarehouseID, toStatus = "warehouse", req.TargetWarehouseID, "available"
	case "return_to_warehouse":
		toLocation, toWarehouseID = "warehouse", req.TargetWarehouseID
	case "return_to_subcontractor":
		toLocation, toStatus = "external", "external"
	case "adjustment_out":
		toLocation, toStatus, eventType = "external", "external", "reclassify"
	}
	condition := cleanOptionalText(unit["condition"])
	if allocation.Condition != "" {
		if statusIn(req.MovementType, "issue", "transfer", "adjustment_out") && allocation.Condition != condition {
			return validationError("Không được đổi tình trạng đơn vị vật lý trong phiếu xuất/chuyển", Record{"allocation_index": index, "stock_unit_id": allocation.StockUnitID})
		}
		condition = allocation.Condition
	}
	if req.MovementType == "return_to_warehouse" {
		toStatus = statusForStockUnitCondition(condition)
	}
	custodyLocation := allocation.CustodyLocation
	if custodyLocation == "" {
		custodyLocation = cleanOptionalText(unit["custody_location"])
	}
	partial := beforeQuantity-allocation.Quantity > stockUnitQuantityTolerance
	targetUnit := unit
	if partial {
		if _, err := tx.ExecContext(ctx, `
			UPDATE material_stock_units
			SET quantity=$2,length_mm=$3,width_mm=$4,thickness_mm=$5,
				version=version+1,updated_by=NULLIF($6,'')::uuid,updated_at=now()
			WHERE id=$1
		`, unit["id"], beforeQuantity-allocation.Quantity,
			coalesceDimension(allocation.RemainingLengthMM, unit["length_mm"]), coalesceDimension(allocation.RemainingWidthMM, unit["width_mm"]), coalesceDimension(allocation.RemainingThicknessMM, unit["thickness_mm"]), actorID); err != nil {
			return err
		}
		if _, err := createMaterialStockUnitEventTx(ctx, tx, cleanID(unit["id"]), "split", allocation.Quantity, beforeQuantity, beforeQuantity-allocation.Quantity, cleanOptionalText(unit["location_type"]), cleanOptionalText(unit["location_type"]), cleanID(unit["warehouse_id"]), cleanID(unit["warehouse_id"]), cleanID(unit["project_id"]), cleanID(unit["project_id"]), "stock_movement", cleanID(movement["id"]), actorID, Record{"stock_document_item_id": documentItemID}); err != nil {
			return err
		}
		input := MaterialStockUnitInput{
			UnitCode: allocation.ResultUnitCode, Quantity: allocation.Quantity, Unit: item.Unit,
			LengthMM: allocation.LengthMM, WidthMM: allocation.WidthMM, ThicknessMM: allocation.ThicknessMM,
			Condition: condition, CustodyLocation: custodyLocation,
		}
		if input.UnitCode == "" {
			input.UnitCode = derivedStockUnitCode(cleanOptionalText(unit["unit_code"]))
		}
		if input.LengthMM == nil {
			input.LengthMM = nullableFloat64(unit["length_mm"])
		}
		if input.WidthMM == nil {
			input.WidthMM = nullableFloat64(unit["width_mm"])
		}
		if input.ThicknessMM == nil {
			input.ThicknessMM = nullableFloat64(unit["thickness_mm"])
		}
		rootID := cleanID(unit["root_unit_id"])
		if rootID == "" {
			rootID = cleanID(unit["id"])
		}
		targetUnit, err = createMaterialStockUnitTx(ctx, tx, item.MaterialID, rootID, cleanID(unit["id"]), documentItemID, toLocation, toWarehouseID, toProjectID, "", input, toStatus, actorID)
		if err != nil {
			return err
		}
	} else {
		_, err := tx.ExecContext(ctx, `
			UPDATE material_stock_units
			SET location_type=$2,warehouse_id=NULLIF($3,'')::uuid,project_id=NULLIF($4,'')::uuid,
				work_item_id=NULL,status=$5,condition=$6,custody_location=NULLIF($7,''),
				length_mm=$8,width_mm=$9,thickness_mm=$10,version=version+1,
				updated_by=NULLIF($11,'')::uuid,updated_at=now()
			WHERE id=$1
		`, unit["id"], toLocation, toWarehouseID, toProjectID, toStatus, condition, custodyLocation,
			coalesceDimension(allocation.LengthMM, unit["length_mm"]), coalesceDimension(allocation.WidthMM, unit["width_mm"]), coalesceDimension(allocation.ThicknessMM, unit["thickness_mm"]), actorID)
		if err != nil {
			return err
		}
	}
	event, err := createMaterialStockUnitEventTx(ctx, tx, cleanID(targetUnit["id"]), eventType, allocation.Quantity, map[bool]float64{true: 0, false: beforeQuantity}[partial], allocation.Quantity, cleanOptionalText(unit["location_type"]), toLocation, cleanID(unit["warehouse_id"]), toWarehouseID, cleanID(unit["project_id"]), toProjectID, "stock_movement", cleanID(movement["id"]), actorID, Record{"stock_document_item_id": documentItemID, "source_stock_unit_id": unit["id"], "partial": partial})
	if err != nil {
		return err
	}
	return createStockMovementUnitAllocationTx(ctx, tx, cleanID(movement["id"]), documentItemID, cleanID(targetUnit["id"]), cleanID(event["id"]), allocation.Quantity)
}

func normalizeStockUnitInput(input *MaterialStockUnitInput, materialUnit string, index int, seenCodes map[string]bool) error {
	input.UnitCode = strings.TrimSpace(input.UnitCode)
	input.Unit = strings.TrimSpace(input.Unit)
	input.Condition = strings.TrimSpace(input.Condition)
	input.CustodyLocation = strings.TrimSpace(input.CustodyLocation)
	if input.UnitCode == "" {
		input.UnitCode = fmt.Sprintf("VT-%d-%s", timeNowUnix(), newID()[:6])
	}
	key := strings.ToUpper(input.UnitCode)
	if seenCodes[key] {
		return validationError("Mã đơn vị vật lý bị trùng trong yêu cầu", Record{"unit_index": index, "unit_code": input.UnitCode})
	}
	seenCodes[key] = true
	if input.Quantity <= 0 || math.IsNaN(input.Quantity) || math.IsInf(input.Quantity, 0) {
		return validationError("Số lượng đơn vị vật lý phải lớn hơn 0", Record{"unit_index": index})
	}
	if input.Unit == "" {
		input.Unit = materialUnit
	}
	if input.Unit == "" || input.Unit != materialUnit {
		return validationError("ĐVT đơn vị vật lý phải trùng ĐVT vật tư", Record{"unit_index": index, "expected_unit": materialUnit, "unit": input.Unit})
	}
	if input.Condition == "" {
		input.Condition = "usable"
	}
	if !statusIn(input.Condition, "usable", "damaged", "mixed", "unknown") {
		return validationError("Tình trạng đơn vị vật lý không hợp lệ", Record{"unit_index": index, "condition": input.Condition})
	}
	if len([]rune(input.UnitCode)) > 120 || len([]rune(input.CustodyLocation)) > 240 {
		return validationError("Mã hoặc vị trí đơn vị vật lý vượt độ dài cho phép", Record{"unit_index": index})
	}
	return validateStockUnitDimensions(input.LengthMM, input.WidthMM, input.ThicknessMM, index)
}

func validateStockUnitDimensions(lengthMM, widthMM, thicknessMM *float64, index int) error {
	for field, value := range map[string]*float64{"length_mm": lengthMM, "width_mm": widthMM, "thickness_mm": thicknessMM} {
		if value != nil && (*value <= 0 || math.IsNaN(*value) || math.IsInf(*value, 0)) {
			return validationError("Kích thước đơn vị vật lý phải lớn hơn 0", Record{"unit_index": index, "field": field})
		}
	}
	return nil
}

func createMaterialStockUnitTx(ctx context.Context, tx *sql.Tx, materialID, rootID, parentID, originItemID, locationType, warehouseID, projectID, workItemID string, input MaterialStockUnitInput, status, actorID string) (Record, error) {
	row, err := queryOneRecord(ctx, tx, `
		INSERT INTO material_stock_units (
			unit_code,material_id,root_unit_id,parent_unit_id,origin_stock_document_item_id,
			location_type,warehouse_id,project_id,work_item_id,quantity,unit,
			length_mm,width_mm,thickness_mm,condition,status,custody_location,created_by,updated_by
		) VALUES (
			$1,$2,NULLIF($3,'')::uuid,NULLIF($4,'')::uuid,NULLIF($5,'')::uuid,
			$6,NULLIF($7,'')::uuid,NULLIF($8,'')::uuid,NULLIF($9,'')::uuid,$10,$11,
			$12,$13,$14,$15,$16,NULLIF($17,''),NULLIF($18,'')::uuid,NULLIF($18,'')::uuid
		) RETURNING *
	`, input.UnitCode, materialID, rootID, parentID, originItemID, locationType, warehouseID, projectID, workItemID, input.Quantity, input.Unit,
		input.LengthMM, input.WidthMM, input.ThicknessMM, input.Condition, status, input.CustodyLocation, actorID)
	if err != nil {
		return nil, err
	}
	if rootID == "" {
		if _, err := tx.ExecContext(ctx, `UPDATE material_stock_units SET root_unit_id=id WHERE id=$1`, row["id"]); err != nil {
			return nil, err
		}
		row["root_unit_id"] = row["id"]
	}
	return row, nil
}

func createMaterialStockUnitEventTx(ctx context.Context, tx *sql.Tx, stockUnitID, eventType string, quantity, beforeQuantity, afterQuantity float64, fromLocation, toLocation, fromWarehouseID, toWarehouseID, fromProjectID, toProjectID, sourceType, sourceID, actorID string, metadata Record) (Record, error) {
	rawMetadata, _ := json.Marshal(metadata)
	return queryOneRecord(ctx, tx, `
		INSERT INTO material_stock_unit_events (
			stock_unit_id,event_type,quantity,before_quantity,after_quantity,
			from_location_type,to_location_type,from_warehouse_id,to_warehouse_id,
			from_project_id,to_project_id,source_type,source_id,actor_id,metadata_json
		) VALUES (
			$1,$2,$3,$4,$5,NULLIF($6,''),NULLIF($7,''),NULLIF($8,'')::uuid,NULLIF($9,'')::uuid,
			NULLIF($10,'')::uuid,NULLIF($11,'')::uuid,$12,NULLIF($13,'')::uuid,NULLIF($14,'')::uuid,$15::jsonb
		) RETURNING *
	`, stockUnitID, eventType, quantity, beforeQuantity, afterQuantity, fromLocation, toLocation, fromWarehouseID, toWarehouseID, fromProjectID, toProjectID, sourceType, sourceID, actorID, string(rawMetadata))
}

func createStockMovementUnitAllocationTx(ctx context.Context, tx *sql.Tx, movementID, documentItemID, stockUnitID, eventID string, quantity float64) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO stock_movement_unit_allocations (stock_movement_id,stock_document_item_id,stock_unit_id,stock_unit_event_id,quantity)
		VALUES ($1,NULLIF($2,'')::uuid,$3,$4,$5)
	`, movementID, documentItemID, stockUnitID, eventID, quantity)
	return err
}

func statusForStockUnitCondition(condition string) string {
	if condition == "usable" {
		return "available"
	}
	return "quarantined"
}

func derivedStockUnitCode(source string) string {
	return fmt.Sprintf("%s-%s", source, strings.ToUpper(newID()[:6]))
}

func nullableFloat64(value any) *float64 {
	if value == nil {
		return nil
	}
	n := number(value)
	return &n
}

func coalesceDimension(requested *float64, current any) any {
	if requested != nil {
		return *requested
	}
	return current
}

func decodeStockUnitAllocations(value any) ([]StockUnitAllocationRequest, error) {
	if value == nil {
		return nil, nil
	}
	// PostgreSQL JSONB null is normalized by scanRows as the string "null".
	// A draft aggregate-material surplus handling intentionally persists no
	// physical-unit allocations, which is equivalent to an empty list.
	switch value := value.(type) {
	case string:
		if strings.TrimSpace(value) == "null" {
			return nil, nil
		}
	case []byte:
		if strings.TrimSpace(string(value)) == "null" {
			return nil, nil
		}
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var allocations []StockUnitAllocationRequest
	if err := json.Unmarshal(raw, &allocations); err != nil {
		return nil, err
	}
	return allocations, nil
}

func decodeStockUnitUsage(value any) ([]StockUnitUsageRequest, error) {
	if value == nil {
		return nil, nil
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var usage []StockUnitUsageRequest
	if err := json.Unmarshal(raw, &usage); err != nil {
		return nil, err
	}
	return usage, nil
}

func applyMaterialStockUnitUsageTx(ctx context.Context, tx *sql.Tx, projectID, materialID, snapshotItemID string, usage []StockUnitUsageRequest, actorID string) error {
	for index, item := range usage {
		item.StockUnitID = cleanID(item.StockUnitID)
		if item.StockUnitID == "" || item.UsedQuantity < 0 || item.WasteQuantity < 0 || item.UsedQuantity+item.WasteQuantity <= 0 {
			return validationError("Phân bổ sử dụng đơn vị vật lý không hợp lệ", Record{"usage_index": index})
		}
		if err := validateStockUnitDimensions(item.RemainingLengthMM, item.RemainingWidthMM, item.RemainingThicknessMM, index); err != nil {
			return err
		}
		unit, err := queryOneRecord(ctx, tx, `SELECT * FROM material_stock_units WHERE id=$1 FOR UPDATE`, item.StockUnitID)
		if err != nil {
			return validationError("Không tìm thấy đơn vị vật lý được sử dụng", Record{"usage_index": index, "stock_unit_id": item.StockUnitID})
		}
		if cleanID(unit["material_id"]) != materialID || cleanOptionalText(unit["location_type"]) != "project" || cleanID(unit["project_id"]) != projectID || cleanOptionalText(unit["status"]) != "issued" {
			return validationError("Đơn vị vật lý không còn ở đúng công trình", Record{"usage_index": index, "stock_unit_id": item.StockUnitID})
		}
		before := number(unit["quantity"])
		consumed := item.UsedQuantity + item.WasteQuantity
		if consumed > before+stockUnitQuantityTolerance {
			return validationError("Số lượng sử dụng/hao hụt vượt phần còn lại của đơn vị vật lý", Record{"usage_index": index, "stock_unit_id": item.StockUnitID, "available": before, "quantity": consumed})
		}
		after := before - consumed
		if after < stockUnitQuantityTolerance {
			after = 0
		}
		condition := strings.TrimSpace(item.Condition)
		if condition == "" {
			condition = cleanOptionalText(unit["condition"])
		}
		remainingCode := strings.TrimSpace(item.RemainingCode)
		if remainingCode == "" {
			remainingCode = cleanOptionalText(unit["unit_code"])
		}
		locationType := "project"
		status := "issued"
		updatedProjectID := projectID
		if after == 0 {
			locationType = "consumed"
			status = "consumed"
			updatedProjectID = ""
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE material_stock_units
			SET unit_code=$2,quantity=$3,length_mm=$4,width_mm=$5,thickness_mm=$6,condition=$7,
				location_type=$8,project_id=NULLIF($9,'')::uuid,warehouse_id=NULL,status=$10,
				version=version+1,updated_by=NULLIF($11,'')::uuid,updated_at=now()
			WHERE id=$1
		`, item.StockUnitID, remainingCode, after,
			coalesceDimension(item.RemainingLengthMM, unit["length_mm"]),
			coalesceDimension(item.RemainingWidthMM, unit["width_mm"]),
			coalesceDimension(item.RemainingThicknessMM, unit["thickness_mm"]),
			condition, locationType, updatedProjectID, status, actorID); err != nil {
			return err
		}
		cursor := before
		if item.UsedQuantity > 0 {
			next := cursor - item.UsedQuantity
			toLocation := "project"
			toProjectID := projectID
			if next <= stockUnitQuantityTolerance && item.WasteQuantity <= stockUnitQuantityTolerance {
				next, toLocation, toProjectID = 0, "consumed", ""
			}
			if _, err := createMaterialStockUnitEventTx(ctx, tx, item.StockUnitID, "use", item.UsedQuantity, cursor, next, "project", toLocation, "", "", projectID, toProjectID, "material_snapshot_item", snapshotItemID, actorID, Record{"usage_index": index}); err != nil {
				return err
			}
			cursor = next
		}
		if item.WasteQuantity > 0 {
			next := cursor - item.WasteQuantity
			toLocation := "project"
			toProjectID := projectID
			if next <= stockUnitQuantityTolerance {
				next, toLocation, toProjectID = 0, "consumed", ""
			}
			if _, err := createMaterialStockUnitEventTx(ctx, tx, item.StockUnitID, "waste", item.WasteQuantity, cursor, next, "project", toLocation, "", "", projectID, toProjectID, "material_snapshot_item", snapshotItemID, actorID, Record{"usage_index": index, "condition": condition}); err != nil {
				return err
			}
		}
	}
	return nil
}
