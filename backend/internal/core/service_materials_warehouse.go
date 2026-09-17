package core

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
)

func (s *Service) CreateMaterial(ctx context.Context, body Record, actorID, actorName string) (Record, error) {
	if err := normalizeMaterialBody(body, true); err != nil {
		return nil, err
	}
	if err := s.applyPartnerSnapshot(ctx, body, "default_supplier_partner_id", "default_supplier", "", []string{"supplier", "service_provider", "other"}); err != nil {
		return nil, err
	}
	return s.repo.CreateMaterial(ctx, body, actorID, actorName)
}

func (s *Service) UpdateMaterial(ctx context.Context, materialID string, body Record, actorID, actorName string) (Record, error) {
	if err := normalizeMaterialBody(body, false); err != nil {
		return nil, err
	}
	if err := s.applyPartnerSnapshot(ctx, body, "default_supplier_partner_id", "default_supplier", "", []string{"supplier", "service_provider", "other"}); err != nil {
		return nil, err
	}
	return s.repo.UpdateMaterial(ctx, cleanID(materialID), body, actorID, actorName)
}

func normalizeMaterialBody(body Record, creating bool) error {
	if body == nil {
		return validationError("Dữ liệu vật tư không hợp lệ")
	}
	for _, key := range []string{"code", "name", "unit", "category", "default_supplier", "tracking_mode", "form_factor"} {
		if value, exists := body[key]; exists {
			body[key] = cleanOptionalText(value)
		}
	}
	for _, key := range []string{"code", "name", "unit", "category"} {
		if cleanOptionalText(body[key]) == "" {
			return validationError("Thông tin vật tư bắt buộc còn thiếu", Record{"field": key})
		}
	}
	if creating {
		body["active"] = true
	} else {
		delete(body, "active")
	}
	trackingMode := cleanOptionalText(body["tracking_mode"])
	if trackingMode == "" {
		trackingMode = "aggregate"
	}
	if !statusIn(trackingMode, "aggregate", "physical_unit") {
		return validationError("Chế độ theo dõi vật tư không hợp lệ", Record{"tracking_mode": trackingMode})
	}
	formFactor := cleanOptionalText(body["form_factor"])
	if formFactor != "" && !statusIn(formFactor, "segment", "roll", "bar", "sheet") {
		return validationError("Dạng vật tư không hợp lệ", Record{"form_factor": formFactor})
	}
	if trackingMode == "physical_unit" && formFactor == "" {
		return validationError("Dạng vật tư là bắt buộc khi theo dõi từng đơn vị vật lý", Record{"field": "form_factor"})
	}
	if trackingMode == "aggregate" {
		formFactor = ""
	}
	body["tracking_mode"] = trackingMode
	if formFactor == "" {
		body["form_factor"] = nil
	} else {
		body["form_factor"] = formFactor
	}
	delete(body, "standard_price")
	delete(body, "physical_tracking_initialized_at")
	delete(body, "physical_tracking_initialized_by")
	return nil
}

func (s *Service) CreateWarehouse(ctx context.Context, body Record, actorID, actorName string) (Record, error) {
	row, err := s.repo.CreateWarehouse(ctx, body)
	if err != nil {
		return nil, err
	}
	s.repo.AddAuditBestEffort(ctx, "", actorID, actorName, "warehouse_created", "warehouse", cleanID(row["id"]), fmt.Sprintf("Tạo kho %s", row["name"]), Record{"type": row["type"]})
	return row, nil
}

func (s *Service) MaterialSnapshot(ctx context.Context, workItemID, actorID string) (Record, error) {
	canView, err := s.repo.CanViewWorkItem(ctx, workItemID, actorID)
	if err != nil {
		return nil, err
	}
	if !canView {
		return nil, forbiddenError("Bạn không có quyền xem đầu việc này")
	}
	item, err := s.repo.GetWorkItemDetail(ctx, workItemID)
	if err != nil {
		return nil, err
	}
	projectID := cleanID(item["project_id"])
	allowed, err := s.repo.CanAccessProjectPermission(ctx, actorID, projectID, []string{"warehouse.view", "operation.view", "operation.report", "warehouse.issue"})
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền xem ghi nhận vật tư", Record{"permissions": []string{"warehouse.view", "operation.view"}})
	}
	return s.materialSnapshotResponse(ctx, item)
}

func (s *Service) SaveMaterialSnapshot(ctx context.Context, workItemID string, req MaterialSnapshotRequest, actorID, actorName string) (Record, error) {
	return s.saveMaterialSnapshot(ctx, workItemID, req, false, actorID, actorName)
}

func (s *Service) SubmitMaterialSnapshot(ctx context.Context, workItemID string, req MaterialSnapshotRequest, actorID, actorName string) (Record, error) {
	return s.saveMaterialSnapshot(ctx, workItemID, req, true, actorID, actorName)
}

func (s *Service) saveMaterialSnapshot(ctx context.Context, workItemID string, req MaterialSnapshotRequest, submit bool, actorID, actorName string) (Record, error) {
	allowed, err := s.repo.CanActOnWorkItem(ctx, workItemID, actorID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền thao tác đầu việc này")
	}
	item, err := s.repo.GetWorkItemDetail(ctx, workItemID)
	if err != nil {
		return nil, err
	}
	if err := s.requireActivePipelineWorkItem(ctx, item); err != nil {
		return nil, err
	}
	if err := s.requireWorkItemPermission(ctx, item, actorID); err != nil {
		return nil, err
	}
	projectID := cleanID(item["project_id"])
	canUpdate, err := s.repo.CanAccessProjectPermission(ctx, actorID, projectID, []string{"operation.report", "warehouse.issue"})
	if err != nil {
		return nil, err
	}
	if !canUpdate {
		return nil, forbiddenError("Bạn không có quyền ghi nhận vật tư", Record{"permissions": []string{"operation.report", "warehouse.issue"}})
	}
	materials, err := s.repo.ListActiveMaterials(ctx)
	if err != nil {
		return nil, err
	}
	balances, err := s.repo.ProjectMaterialBalances(ctx, projectID)
	if err != nil {
		return nil, err
	}
	normalizedItems, summary, header, err := s.normalizeMaterialSnapshotRequest(ctx, item, materials, balances, req, submit)
	if err != nil {
		return nil, err
	}
	if err := s.repo.SaveMaterialSnapshot(ctx, item, header, normalizedItems, summary, submit, actorID, actorName); err != nil {
		return nil, err
	}
	result, err := s.materialSnapshotResponse(ctx, item)
	transition, _ := item["work_item_transition"].(Record)
	return attachWorkItemTransition(result, transition), err
}

func (s *Service) materialSnapshotResponse(ctx context.Context, item Record) (Record, error) {
	projectID := cleanID(item["project_id"])
	materials, err := s.repo.ListActiveMaterials(ctx)
	if err != nil {
		return nil, err
	}
	balances, err := s.repo.ProjectMaterialBalances(ctx, projectID)
	if err != nil {
		return nil, err
	}
	snapshot, err := s.repo.GetMaterialSnapshot(ctx, cleanID(item["id"]))
	if errors.Is(err, sql.ErrNoRows) {
		snapshot = Record{
			"id":                      "",
			"project_id":              projectID,
			"work_item_id":            item["id"],
			"status":                  "draft",
			"snapshot_date":           time.Now().Format("2006-01-02"),
			"construction_area":       "",
			"summary_json":            materialSnapshotSummary(nil),
			"comment":                 "",
			"applied_to_site_balance": false,
			"items":                   []Record{},
		}
	} else if err != nil {
		return nil, err
	}
	return Record{
		"available":         true,
		"work_item_id":      item["id"],
		"project_id":        projectID,
		"snapshot":          snapshot,
		"materials":         materials,
		"material_balances": balances,
	}, nil
}

func (s *Service) normalizeMaterialSnapshotRequest(ctx context.Context, item Record, materials []Record, balances []Record, req MaterialSnapshotRequest, submit bool) ([]Record, Record, Record, error) {
	comment := strings.TrimSpace(req.Comment)
	if len([]rune(comment)) > 2000 {
		return nil, nil, nil, validationError("Ghi chú ghi nhận vật tư không được vượt quá 2000 ký tự")
	}
	constructionArea := strings.TrimSpace(req.ConstructionArea)
	if len([]rune(constructionArea)) > 240 {
		return nil, nil, nil, validationError("Khu vực thi công không được vượt quá 240 ký tự")
	}
	snapshotDate := strings.TrimSpace(req.SnapshotDate)
	if snapshotDate != "" {
		if _, err := time.Parse("2006-01-02", snapshotDate); err != nil {
			return nil, nil, nil, validationError("Ngày ghi nhận vật tư không hợp lệ", Record{"snapshot_date": snapshotDate})
		}
	} else if submit {
		return nil, nil, nil, validationError("Ngày ghi nhận vật tư là bắt buộc khi gửi")
	}
	if submit && len(req.Items) == 0 {
		return nil, nil, nil, validationError("Cần ít nhất một dòng vật tư để gửi ghi nhận")
	}

	materialByID := map[string]Record{}
	for _, material := range materials {
		materialByID[cleanID(material["id"])] = material
	}
	balanceByMaterialID := map[string]Record{}
	for _, balance := range balances {
		balanceByMaterialID[cleanID(balance["material_id"])] = balance
	}
	seen := map[int]bool{}
	projectID := cleanID(item["project_id"])
	workItemID := cleanID(item["id"])
	normalized := make([]Record, 0, len(req.Items))
	for index, itemReq := range req.Items {
		itemIndex := itemReq.ItemIndex
		if itemIndex < 0 {
			itemIndex = index
		}
		if itemIndex < 0 {
			return nil, nil, nil, validationError("Dòng ghi nhận vật tư không hợp lệ", Record{"item_index": itemReq.ItemIndex})
		}
		if seen[itemIndex] {
			return nil, nil, nil, validationError("Dòng ghi nhận vật tư bị trùng", Record{"item_index": itemIndex})
		}
		seen[itemIndex] = true
		row, err := s.normalizeMaterialSnapshotItem(ctx, projectID, workItemID, itemIndex, materialByID, balanceByMaterialID, itemReq, submit)
		if err != nil {
			return nil, nil, nil, err
		}
		normalized = append(normalized, row)
	}
	summary := materialSnapshotSummary(normalized)
	header := Record{
		"snapshot_date":     snapshotDate,
		"construction_area": constructionArea,
		"comment":           comment,
	}
	return normalized, summary, header, nil
}

func (s *Service) normalizeMaterialSnapshotItem(ctx context.Context, projectID, workItemID string, itemIndex int, materialByID map[string]Record, balanceByMaterialID map[string]Record, req MaterialSnapshotItemRequest, submit bool) (Record, error) {
	source := strings.TrimSpace(req.Source)
	if source == "" {
		source = "site_balance"
	}
	if !statusIn(source, "site_balance", "warehouse_issue", "subcontractor", "other") {
		return nil, validationError("Nguồn vật tư không hợp lệ", Record{"item_index": itemIndex, "source": source})
	}
	affectsSiteBalance := source == "site_balance" || source == "warehouse_issue"
	if req.AffectsSiteBalance != nil {
		affectsSiteBalance = *req.AffectsSiteBalance
	}

	materialID := cleanID(req.MaterialID)
	var material Record
	if materialID != "" {
		var ok bool
		material, ok = materialByID[materialID]
		if !ok {
			return nil, validationError("Vật tư không tồn tại hoặc đã ngưng hoạt động", Record{"item_index": itemIndex, "material_id": materialID})
		}
	}
	if affectsSiteBalance && materialID == "" {
		return nil, validationError("Dòng ảnh hưởng số dư vật tư công trình cần chọn vật tư", Record{"item_index": itemIndex})
	}

	materialName := strings.TrimSpace(req.MaterialName)
	materialCode := ""
	if material != nil {
		materialName = strings.TrimSpace(fmt.Sprint(material["name"]))
		materialCode = strings.TrimSpace(fmt.Sprint(material["code"]))
	}
	if materialName == "" {
		return nil, validationError("Tên vật tư là bắt buộc", Record{"item_index": itemIndex})
	}
	if len([]rune(materialName)) > 240 {
		return nil, validationError("Tên vật tư không được vượt quá 240 ký tự", Record{"item_index": itemIndex})
	}

	unit := strings.TrimSpace(req.Unit)
	if unit == "" && material != nil {
		unit = strings.TrimSpace(fmt.Sprint(material["unit"]))
	}
	if unit == "" {
		return nil, validationError("Đơn vị vật tư là bắt buộc", Record{"item_index": itemIndex})
	}
	if len([]rune(unit)) > 40 {
		return nil, validationError("Đơn vị vật tư không được vượt quá 40 ký tự", Record{"item_index": itemIndex})
	}

	beforeQuantity, err := materialSnapshotQuantity(req.BeforeQuantity, "Số lượng trước không hợp lệ", itemIndex)
	if err != nil {
		return nil, err
	}
	addedQuantity, err := materialSnapshotQuantity(req.AddedQuantity, "Số lượng bổ sung không hợp lệ", itemIndex)
	if err != nil {
		return nil, err
	}
	afterQuantity, err := materialSnapshotQuantity(req.AfterQuantity, "Số lượng sau không hợp lệ", itemIndex)
	if err != nil {
		return nil, err
	}
	wasteQuantity, err := materialSnapshotQuantity(req.WasteQuantity, "Số lượng hao hụt/hư hỏng không hợp lệ", itemIndex)
	if err != nil {
		return nil, err
	}
	usedQuantity := materialSnapshotRound(beforeQuantity + addedQuantity - afterQuantity - wasteQuantity)
	if usedQuantity < -0.000001 {
		return nil, validationError("Số lượng sử dụng tính ra không được âm", Record{
			"item_index":      itemIndex,
			"before_quantity": beforeQuantity,
			"added_quantity":  addedQuantity,
			"after_quantity":  afterQuantity,
			"waste_quantity":  wasteQuantity,
		})
	}
	if usedQuantity < 0 {
		usedQuantity = 0
	}

	var balance Record
	siteBalanceID := ""
	if affectsSiteBalance {
		balance = balanceByMaterialID[materialID]
		if balance == nil {
			return nil, validationError("Chưa có số dư vật tư tại công trình cho dòng snapshot", Record{"item_index": itemIndex, "material_id": materialID})
		}
		siteBalanceID = cleanID(balance["id"])
		if submit && materialSnapshotQuantityExceeds(usedQuantity+wasteQuantity, materialBalanceAvailable(balance)) {
			return nil, validationError("Số lượng sử dụng và hao hụt vượt vật tư còn tại công trình", Record{
				"item_index":  itemIndex,
				"material_id": materialID,
				"available":   materialBalanceAvailable(balance),
				"quantity":    usedQuantity + wasteQuantity,
			})
		}
	}
	stockUnitUsage := req.StockUnitUsage
	if material != nil && cleanOptionalText(material["tracking_mode"]) == "physical_unit" && affectsSiteBalance {
		usedAllocated := 0.0
		wasteAllocated := 0.0
		seenUnits := map[string]bool{}
		for usageIndex := range stockUnitUsage {
			usage := &stockUnitUsage[usageIndex]
			usage.StockUnitID = cleanID(usage.StockUnitID)
			usage.RemainingCode = strings.TrimSpace(usage.RemainingCode)
			usage.Condition = strings.TrimSpace(usage.Condition)
			if usage.StockUnitID == "" || usage.UsedQuantity < 0 || usage.WasteQuantity < 0 || math.IsNaN(usage.UsedQuantity) || math.IsNaN(usage.WasteQuantity) || math.IsInf(usage.UsedQuantity, 0) || math.IsInf(usage.WasteQuantity, 0) || usage.UsedQuantity+usage.WasteQuantity <= 0 {
				return nil, validationError("Phân bổ sử dụng đơn vị vật lý không hợp lệ", Record{"item_index": itemIndex, "usage_index": usageIndex})
			}
			if seenUnits[usage.StockUnitID] {
				return nil, validationError("Một đơn vị vật lý không được lặp trong cùng dòng ghi nhận", Record{"item_index": itemIndex, "stock_unit_id": usage.StockUnitID})
			}
			seenUnits[usage.StockUnitID] = true
			if err := validateStockUnitDimensions(usage.RemainingLengthMM, usage.RemainingWidthMM, usage.RemainingThicknessMM, usageIndex); err != nil {
				return nil, err
			}
			if usage.Condition != "" && !statusIn(usage.Condition, "usable", "damaged", "mixed", "unknown") {
				return nil, validationError("Tình trạng phần còn lại không hợp lệ", Record{"item_index": itemIndex, "usage_index": usageIndex})
			}
			usedAllocated += usage.UsedQuantity
			wasteAllocated += usage.WasteQuantity
		}
		if submit && usedQuantity+wasteQuantity > stockUnitQuantityTolerance && len(stockUnitUsage) == 0 {
			return nil, validationError("Vật tư theo dõi từng đơn vị cần chọn đúng Đoạn/Cuộn/Cây/Tấm đã sử dụng", Record{"item_index": itemIndex})
		}
		if (submit || len(stockUnitUsage) > 0) && (math.Abs(usedAllocated-usedQuantity) > stockUnitQuantityTolerance || math.Abs(wasteAllocated-wasteQuantity) > stockUnitQuantityTolerance) {
			return nil, validationError("Tổng sử dụng/hao hụt theo đơn vị vật lý phải khớp dòng ghi nhận", Record{"item_index": itemIndex, "used_quantity": usedQuantity, "allocated_used_quantity": usedAllocated, "waste_quantity": wasteQuantity, "allocated_waste_quantity": wasteAllocated})
		}
	} else if len(stockUnitUsage) > 0 {
		return nil, validationError("Dòng vật tư theo dõi tổng hợp không nhận phân bổ đơn vị vật lý", Record{"item_index": itemIndex})
	}

	beforeEvidenceID, err := s.validateMaterialSnapshotEvidence(ctx, projectID, workItemID, req.BeforeEvidenceID, "before_evidence_id", itemIndex)
	if err != nil {
		return nil, err
	}
	afterEvidenceID, err := s.validateMaterialSnapshotEvidence(ctx, projectID, workItemID, req.AfterEvidenceID, "after_evidence_id", itemIndex)
	if err != nil {
		return nil, err
	}
	area := strings.TrimSpace(req.ConstructionArea)
	if len([]rune(area)) > 240 {
		return nil, validationError("Khu vực dòng snapshot không được vượt quá 240 ký tự", Record{"item_index": itemIndex})
	}
	condition := strings.TrimSpace(req.Condition)
	if condition == "" {
		condition = "usable"
	}
	if !statusIn(condition, "usable", "damaged", "mixed", "unknown") {
		return nil, validationError("Tình trạng vật tư không hợp lệ", Record{"item_index": itemIndex, "condition": condition})
	}
	comment := strings.TrimSpace(req.Comment)
	if len([]rune(comment)) > 1000 {
		return nil, validationError("Ghi chú dòng snapshot không được vượt quá 1000 ký tự", Record{"item_index": itemIndex})
	}

	return Record{
		"item_index":           itemIndex,
		"material_id":          materialID,
		"material_code":        materialCode,
		"material_name":        materialName,
		"unit":                 unit,
		"source":               source,
		"before_quantity":      beforeQuantity,
		"added_quantity":       addedQuantity,
		"after_quantity":       afterQuantity,
		"waste_quantity":       wasteQuantity,
		"used_quantity":        usedQuantity,
		"affects_site_balance": affectsSiteBalance,
		"site_balance_id":      siteBalanceID,
		"before_evidence_id":   beforeEvidenceID,
		"after_evidence_id":    afterEvidenceID,
		"construction_area":    area,
		"condition":            condition,
		"comment":              comment,
		"tracking_mode":        cleanOptionalText(material["tracking_mode"]),
		"stock_unit_usage":     stockUnitUsage,
		"payload_json": Record{
			"source":                source,
			"affects_site_balance":  affectsSiteBalance,
			"computed_used_formula": "before_quantity + added_quantity - after_quantity - waste_quantity",
		},
	}, nil
}

func (s *Service) validateMaterialSnapshotEvidence(ctx context.Context, projectID, workItemID, evidenceID, field string, itemIndex int) (string, error) {
	evidenceID = cleanID(evidenceID)
	if evidenceID == "" {
		return "", nil
	}
	evidence, err := s.repo.GetEvidenceDetail(ctx, evidenceID)
	if err != nil {
		return "", validationError("Minh chứng ghi nhận vật tư không hợp lệ", Record{"item_index": itemIndex, field: evidenceID})
	}
	if cleanID(evidence["project_id"]) != projectID || cleanID(evidence["work_item_id"]) != workItemID || fmt.Sprint(evidence["status"]) == "rejected" {
		return "", validationError("Minh chứng ghi nhận vật tư không thuộc đầu việc", Record{"item_index": itemIndex, field: evidenceID})
	}
	return evidenceID, nil
}

func materialSnapshotQuantity(value *float64, message string, itemIndex int) (float64, error) {
	if value == nil {
		return 0, nil
	}
	if math.IsNaN(*value) || math.IsInf(*value, 0) || *value < 0 {
		return 0, validationError(message, Record{"item_index": itemIndex, "quantity": *value})
	}
	return materialSnapshotRound(*value), nil
}

func materialSnapshotRound(value float64) float64 {
	return math.Round(value*1000) / 1000
}

func materialSnapshotQuantityExceeds(quantity, available float64) bool {
	return quantity > available+0.000001
}

func materialBalanceAvailable(balance Record) float64 {
	issuedMinusUsed := number(balance["issued_quantity"]) - number(balance["used_quantity"]) - number(balance["waste_quantity"])
	surplus := number(balance["surplus_quantity"])
	available := math.Max(issuedMinusUsed, surplus)
	if available < 0 {
		return 0
	}
	return available
}

func materialSnapshotSummary(items []Record) Record {
	summary := Record{
		"total":                   len(items),
		"affects_site_balance":    0,
		"complete_evidence":       0,
		"with_waste_or_damage":    0,
		"site_balance_used_total": 0.0,
		"non_balance_item_count":  0,
		"source_site_balance":     0,
		"source_warehouse_issue":  0,
		"source_subcontractor":    0,
		"source_other":            0,
	}
	for _, item := range items {
		source := cleanOptionalText(item["source"])
		sourceKey := "source_" + source
		if _, ok := summary[sourceKey]; ok {
			summary[sourceKey] = int(number(summary[sourceKey])) + 1
		}
		if boolValue(item["affects_site_balance"]) {
			summary["affects_site_balance"] = int(number(summary["affects_site_balance"])) + 1
			summary["site_balance_used_total"] = materialSnapshotRound(number(summary["site_balance_used_total"]) + number(item["used_quantity"]))
		} else {
			summary["non_balance_item_count"] = int(number(summary["non_balance_item_count"])) + 1
		}
		if cleanID(item["before_evidence_id"]) != "" && cleanID(item["after_evidence_id"]) != "" {
			summary["complete_evidence"] = int(number(summary["complete_evidence"])) + 1
		}
		if number(item["waste_quantity"]) > 0 || cleanOptionalText(item["condition"]) == "damaged" {
			summary["with_waste_or_damage"] = int(number(summary["with_waste_or_damage"])) + 1
		}
	}
	return summary
}

func (s *Service) SurplusHandlings(ctx context.Context, workItemID, actorID string) (Record, error) {
	canView, err := s.repo.CanViewWorkItem(ctx, workItemID, actorID)
	if err != nil {
		return nil, err
	}
	if !canView {
		return nil, forbiddenError("Bạn không có quyền xem đầu việc này")
	}
	item, err := s.repo.GetWorkItemDetail(ctx, workItemID)
	if err != nil {
		return nil, err
	}
	projectID := cleanID(item["project_id"])
	allowed, err := s.repo.CanAccessProjectPermission(ctx, actorID, projectID, []string{"warehouse.view", "operation.view", "operation.report", "warehouse.surplus_handle"})
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền xem xử lý vật tư dư", Record{"permissions": []string{"warehouse.view", "operation.view"}})
	}
	return s.surplusHandlingResponse(ctx, item)
}

func (s *Service) CreateSurplusHandling(ctx context.Context, workItemID string, req SurplusHandlingRequest, actorID, actorName string) (Record, error) {
	item, err := s.requireSurplusHandlingWorkItemAccess(ctx, workItemID, actorID, []string{"operation.report", "warehouse.surplus_handle"}, "Bạn không có quyền lưu xử lý vật tư dư")
	if err != nil {
		return nil, err
	}
	normalized, err := s.normalizeSurplusHandlingRequest(ctx, item, req, false)
	if err != nil {
		return nil, err
	}
	return s.repo.SaveSurplusHandling(ctx, item, "", normalized, actorID, actorName)
}

func (s *Service) UpdateSurplusHandling(ctx context.Context, handlingID string, req SurplusHandlingRequest, actorID, actorName string) (Record, error) {
	handling, err := s.repo.GetSurplusHandling(ctx, handlingID)
	if err != nil {
		return nil, err
	}
	if fmt.Sprint(handling["status"]) == "submitted" {
		return nil, validationError("Hồ sơ xử lý vật tư dư đã gửi, không thể sửa", Record{"surplus_handling_id": handlingID})
	}
	item, err := s.requireSurplusHandlingWorkItemAccess(ctx, cleanID(handling["work_item_id"]), actorID, []string{"operation.report", "warehouse.surplus_handle"}, "Bạn không có quyền lưu xử lý vật tư dư")
	if err != nil {
		return nil, err
	}
	normalized, err := s.normalizeSurplusHandlingRequest(ctx, item, req, false)
	if err != nil {
		return nil, err
	}
	return s.repo.SaveSurplusHandling(ctx, item, cleanID(handling["id"]), normalized, actorID, actorName)
}

func (s *Service) SubmitSurplusHandling(ctx context.Context, handlingID string, actorID, actorName string) (Record, error) {
	handling, err := s.repo.GetSurplusHandling(ctx, handlingID)
	if err != nil {
		return nil, err
	}
	if fmt.Sprint(handling["status"]) == "submitted" {
		return nil, validationError("Hồ sơ xử lý vật tư dư đã gửi", Record{"surplus_handling_id": handlingID})
	}
	item, err := s.requireSurplusHandlingWorkItemAccess(ctx, cleanID(handling["work_item_id"]), actorID, []string{"warehouse.surplus_handle"}, "Bạn không có quyền gửi hồ sơ xử lý vật tư dư")
	if err != nil {
		return nil, err
	}
	req := SurplusHandlingRequest{
		SiteBalanceID:          cleanID(handling["site_balance_id"]),
		MaterialSnapshotID:     cleanID(handling["material_snapshot_id"]),
		MaterialSnapshotItemID: cleanID(handling["material_snapshot_item_id"]),
		HandlingType:           cleanOptionalText(handling["handling_type"]),
		Unit:                   cleanOptionalText(handling["unit"]),
		WarehouseID:            cleanID(handling["warehouse_id"]),
		CustodyLocation:        cleanOptionalText(handling["custody_location"]),
		Condition:              cleanOptionalText(handling["condition"]),
		EvidenceID:             cleanID(handling["evidence_id"]),
		Comment:                cleanOptionalText(handling["comment"]),
	}
	req.StockUnitAllocations, err = decodeStockUnitAllocations(handling["stock_unit_allocations_json"])
	if err != nil {
		return nil, validationError("Phân bổ đơn vị vật lý đã lưu không hợp lệ", Record{"surplus_handling_id": handlingID})
	}
	quantity := number(handling["quantity"])
	req.Quantity = &quantity
	if _, err := s.normalizeSurplusHandlingRequest(ctx, item, req, true); err != nil {
		return nil, err
	}
	return s.repo.SubmitSurplusHandling(ctx, cleanID(handling["id"]), actorID, actorName)
}

func (s *Service) requireSurplusHandlingWorkItemAccess(ctx context.Context, workItemID, actorID string, permissions []string, message string) (Record, error) {
	allowed, err := s.repo.CanActOnWorkItem(ctx, workItemID, actorID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền thao tác đầu việc này")
	}
	item, err := s.repo.GetWorkItemDetail(ctx, workItemID)
	if err != nil {
		return nil, err
	}
	if err := s.requireActivePipelineWorkItem(ctx, item); err != nil {
		return nil, err
	}
	if err := s.requireWorkItemPermission(ctx, item, actorID); err != nil {
		return nil, err
	}
	projectID := cleanID(item["project_id"])
	canUpdate, err := s.repo.CanAccessProjectPermission(ctx, actorID, projectID, permissions)
	if err != nil {
		return nil, err
	}
	if !canUpdate {
		return nil, forbiddenError(message, Record{"permissions": permissions})
	}
	return item, nil
}

func (s *Service) surplusHandlingResponse(ctx context.Context, item Record) (Record, error) {
	projectID := cleanID(item["project_id"])
	balances, err := s.repo.ProjectMaterialBalances(ctx, projectID)
	if err != nil {
		return nil, err
	}
	warehouses, err := s.repo.ListActiveWarehouses(ctx)
	if err != nil {
		return nil, err
	}
	handlings, err := s.repo.ListSurplusHandlingsForWorkItem(ctx, cleanID(item["id"]))
	if err != nil {
		return nil, err
	}
	return Record{
		"available":         true,
		"work_item_id":      item["id"],
		"project_id":        projectID,
		"material_balances": balances,
		"warehouses":        warehouses,
		"handlings":         handlings,
		"handling_options":  s.surplusHandlingOptions(ctx),
	}, nil
}

func (s *Service) surplusHandlingOptions(ctx context.Context) []string {
	if s.repo.systemSettings == nil {
		return []string{"return_to_warehouse", "return_to_subcontractor"}
	}
	return s.repo.systemSettings.SurplusHandlingOptions(ctx)
}

func (s *Service) normalizeSurplusHandlingRequest(ctx context.Context, item Record, req SurplusHandlingRequest, submit bool) (Record, error) {
	projectID := cleanID(item["project_id"])
	workItemID := cleanID(item["id"])
	balances, err := s.repo.ProjectMaterialBalances(ctx, projectID)
	if err != nil {
		return nil, err
	}
	balanceByID := map[string]Record{}
	for _, balance := range balances {
		balanceByID[cleanID(balance["id"])] = balance
	}
	siteBalanceID := cleanID(req.SiteBalanceID)
	if siteBalanceID == "" {
		return nil, validationError("Số dư vật tư công trình là bắt buộc")
	}
	balance := balanceByID[siteBalanceID]
	if balance == nil {
		return nil, validationError("Số dư vật tư không thuộc công trình", Record{"site_balance_id": siteBalanceID})
	}
	surplusQuantity := number(balance["surplus_quantity"])
	if surplusQuantity <= 0 {
		return nil, validationError("Vật tư này không còn dư để xử lý", Record{"site_balance_id": siteBalanceID})
	}
	quantity, err := surplusHandlingQuantity(req.Quantity)
	if err != nil {
		return nil, err
	}
	if quantityExceedsAvailable(quantity, surplusQuantity) {
		return nil, validationError("Số lượng xử lý vượt vật tư dư", Record{"available": surplusQuantity, "quantity": quantity})
	}
	handlingType := strings.TrimSpace(req.HandlingType)
	if handlingType == "" {
		handlingType = "return_to_warehouse"
	}
	if !statusIn(handlingType, "return_to_warehouse", "return_to_subcontractor") {
		return nil, validationError("Loại xử lý vật tư dư không hợp lệ", Record{"handling_type": handlingType})
	}
	if !statusIn(handlingType, s.surplusHandlingOptions(ctx)...) {
		return nil, validationError("Loại xử lý vật tư dư đang bị tắt trong thiết lập hệ thống", Record{"handling_type": handlingType})
	}
	unit := strings.TrimSpace(req.Unit)
	if unit == "" {
		unit = cleanOptionalText(balance["unit"])
	}
	if unit == "" {
		return nil, validationError("Đơn vị xử lý vật tư dư là bắt buộc")
	}
	if len([]rune(unit)) > 40 {
		return nil, validationError("Đơn vị xử lý vật tư dư không được vượt quá 40 ký tự")
	}
	warehouseID := cleanID(req.WarehouseID)
	if warehouseID == "" {
		return nil, validationError("Kho/kho tạm là bắt buộc")
	}
	if err := s.requireActiveWarehouse(ctx, warehouseID); err != nil {
		return nil, err
	}
	custodyLocation := strings.TrimSpace(req.CustodyLocation)
	if len([]rune(custodyLocation)) > 240 {
		return nil, validationError("Vị trí lưu/trả vật tư không được vượt quá 240 ký tự")
	}
	if submit && custodyLocation == "" {
		return nil, validationError("Vị trí lưu/trả vật tư là bắt buộc khi gửi")
	}
	condition := strings.TrimSpace(req.Condition)
	if condition == "" {
		condition = "usable"
	}
	if !statusIn(condition, "usable", "damaged", "mixed", "unknown") {
		return nil, validationError("Tình trạng vật tư dư không hợp lệ", Record{"condition": condition})
	}
	stockUnitAllocations := req.StockUnitAllocations
	if cleanOptionalText(balance["tracking_mode"]) == "physical_unit" {
		if submit && len(stockUnitAllocations) == 0 {
			return nil, validationError("Vật tư theo dõi từng đơn vị cần chọn đúng Đoạn/Cuộn/Cây/Tấm còn tại công trình", Record{"site_balance_id": siteBalanceID})
		}
		allocatedQuantity := 0.0
		seenUnits := map[string]bool{}
		for index := range stockUnitAllocations {
			allocation := &stockUnitAllocations[index]
			allocation.StockUnitID = cleanID(allocation.StockUnitID)
			allocation.ResultUnitCode = strings.TrimSpace(allocation.ResultUnitCode)
			allocation.Condition = strings.TrimSpace(allocation.Condition)
			allocation.CustodyLocation = strings.TrimSpace(allocation.CustodyLocation)
			if allocation.Condition == "" {
				allocation.Condition = condition
			}
			if allocation.CustodyLocation == "" {
				allocation.CustodyLocation = custodyLocation
			}
			if allocation.StockUnitID == "" || allocation.Quantity <= 0 || math.IsNaN(allocation.Quantity) || math.IsInf(allocation.Quantity, 0) {
				return nil, validationError("Phân bổ đơn vị vật lý không hợp lệ", Record{"allocation_index": index})
			}
			if err := validateStockUnitDimensions(allocation.LengthMM, allocation.WidthMM, allocation.ThicknessMM, index); err != nil {
				return nil, err
			}
			if err := validateStockUnitDimensions(allocation.RemainingLengthMM, allocation.RemainingWidthMM, allocation.RemainingThicknessMM, index); err != nil {
				return nil, err
			}
			if seenUnits[allocation.StockUnitID] {
				return nil, validationError("Một đơn vị vật lý không được chọn lặp", Record{"allocation_index": index, "stock_unit_id": allocation.StockUnitID})
			}
			seenUnits[allocation.StockUnitID] = true
			allocatedQuantity += allocation.Quantity
		}
		if (submit || len(stockUnitAllocations) > 0) && math.Abs(allocatedQuantity-quantity) > stockUnitQuantityTolerance {
			return nil, validationError("Tổng phân bổ đơn vị vật lý phải bằng số lượng xử lý", Record{"quantity": quantity, "allocated_quantity": allocatedQuantity})
		}
	} else if len(stockUnitAllocations) > 0 {
		return nil, validationError("Vật tư theo dõi tổng hợp không nhận phân bổ đơn vị vật lý", Record{"site_balance_id": siteBalanceID})
	}
	evidenceID := cleanID(req.EvidenceID)
	if submit && evidenceID == "" {
		return nil, validationError("Minh chứng/ảnh vật tư dư là bắt buộc khi gửi")
	}
	if evidenceID != "" {
		evidence, err := s.repo.GetEvidenceDetail(ctx, evidenceID)
		if err != nil {
			return nil, validationError("Minh chứng xử lý vật tư dư không hợp lệ", Record{"evidence_id": evidenceID})
		}
		if cleanID(evidence["project_id"]) != projectID || cleanID(evidence["work_item_id"]) != workItemID || fmt.Sprint(evidence["status"]) == "rejected" {
			return nil, validationError("Minh chứng xử lý vật tư dư không thuộc đầu việc", Record{"evidence_id": evidenceID})
		}
	}
	materialSnapshotID, materialSnapshotItemID, err := s.validateSurplusHandlingSnapshotLink(ctx, workItemID, req.MaterialSnapshotID, req.MaterialSnapshotItemID)
	if err != nil {
		return nil, err
	}
	comment := strings.TrimSpace(req.Comment)
	if len([]rune(comment)) > 1000 {
		return nil, validationError("Ghi chú xử lý vật tư dư không được vượt quá 1000 ký tự")
	}
	return Record{
		"project_id":                projectID,
		"work_item_id":              workItemID,
		"site_balance_id":           siteBalanceID,
		"material_id":               cleanID(balance["material_id"]),
		"material_snapshot_id":      materialSnapshotID,
		"material_snapshot_item_id": materialSnapshotItemID,
		"handling_type":             handlingType,
		"quantity":                  materialSnapshotRound(quantity),
		"unit":                      unit,
		"warehouse_id":              warehouseID,
		"custody_location":          custodyLocation,
		"condition":                 condition,
		"evidence_id":               evidenceID,
		"comment":                   comment,
		"material_name":             cleanOptionalText(balance["material_name"]),
		"material_code":             cleanOptionalText(balance["code"]),
		"surplus_before_handling":   surplusQuantity,
		"stock_unit_allocations":    stockUnitAllocations,
	}, nil
}

func surplusHandlingQuantity(value *float64) (float64, error) {
	if value == nil || math.IsNaN(*value) || math.IsInf(*value, 0) || *value <= 0 {
		return 0, validationError("Số lượng xử lý vật tư dư phải lớn hơn 0")
	}
	return materialSnapshotRound(*value), nil
}

func (s *Service) requireActiveWarehouse(ctx context.Context, warehouseID string) error {
	warehouses, err := s.repo.ListActiveWarehouses(ctx)
	if err != nil {
		return err
	}
	for _, warehouse := range warehouses {
		if cleanID(warehouse["id"]) == warehouseID {
			return nil
		}
	}
	return validationError("Kho không tồn tại hoặc đã ngưng hoạt động", Record{"warehouse_id": warehouseID})
}

func (s *Service) validateSurplusHandlingSnapshotLink(ctx context.Context, workItemID, snapshotID, snapshotItemID string) (string, string, error) {
	snapshotID = cleanID(snapshotID)
	snapshotItemID = cleanID(snapshotItemID)
	if snapshotID == "" && snapshotItemID == "" {
		return "", "", nil
	}
	snapshot, err := s.repo.GetMaterialSnapshot(ctx, workItemID)
	if err != nil {
		return "", "", validationError("Ghi nhận vật tư liên kết không hợp lệ", Record{"material_snapshot_id": snapshotID})
	}
	if snapshotID != "" && cleanID(snapshot["id"]) != snapshotID {
		return "", "", validationError("Ghi nhận vật tư không thuộc đầu việc", Record{"material_snapshot_id": snapshotID})
	}
	if snapshotID == "" {
		snapshotID = cleanID(snapshot["id"])
	}
	if snapshotItemID != "" {
		items, _ := snapshot["items"].([]Record)
		found := false
		for _, item := range items {
			if cleanID(item["id"]) == snapshotItemID {
				found = true
				break
			}
		}
		if !found {
			return "", "", validationError("Dòng ghi nhận vật tư không thuộc đầu việc", Record{"material_snapshot_item_id": snapshotItemID})
		}
	}
	return snapshotID, snapshotItemID, nil
}

func (s *Service) CreateStockMovement(ctx context.Context, req StockActionRequest, actorID, actorName string) (Record, error) {
	return s.repo.CreateStockMovement(ctx, req, actorID, actorName)
}

func (s *Service) InitializeMaterialStockUnits(ctx context.Context, materialID string, req InitializeMaterialStockUnitsRequest, actorID, actorName string) (Record, error) {
	return s.repo.InitializeMaterialStockUnits(ctx, cleanID(materialID), req, actorID, actorName)
}

func (s *Service) CreateStockDocument(ctx context.Context, req StockDocumentRequest, actorID, actorName string) (Record, error) {
	return s.repo.CreateStockDocument(ctx, req, actorID, actorName)
}

func (s *Service) UpdateStockDocument(ctx context.Context, documentID string, req StockDocumentRequest, actorID, actorName string) (Record, error) {
	return s.repo.UpdateStockDocument(ctx, cleanID(documentID), req, actorID, actorName)
}

func (s *Service) CreateStockAdjustment(ctx context.Context, req StockAdjustmentRequest, actorID, actorName string) (Record, error) {
	return s.repo.CreateStockAdjustment(ctx, req, actorID, actorName)
}

func (s *Service) AdjustSiteMaterialUsage(ctx context.Context, projectID, materialID string, req SiteMaterialUsageAdjustmentRequest, actorID, actorName string) (Record, error) {
	return s.repo.AdjustSiteMaterialUsage(ctx, cleanID(projectID), cleanID(materialID), req, actorID, actorName)
}
