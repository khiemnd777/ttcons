package core

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
)

func (s *Service) ProjectIncomingAcceptances(ctx context.Context, projectID, actorID string) ([]Record, error) {
	projectID = cleanID(projectID)
	allowed, err := s.repo.CanAccessProjectPermission(ctx, actorID, projectID, []string{"acceptance.view", "operation.view", "warehouse.view"})
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền xem nghiệm thu đầu vào", Record{"permissions": []string{"acceptance.view", "operation.view", "warehouse.view"}})
	}
	return s.repo.ListIncomingAcceptancesForProject(ctx, projectID)
}

func (s *Service) WorkItemIncomingAcceptances(ctx context.Context, workItemID, actorID string) (Record, error) {
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
	allowed, err := s.repo.CanAccessProjectPermission(ctx, actorID, projectID, []string{"acceptance.view", "operation.view", "warehouse.view", "operation.report", "acceptance.create"})
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền xem nghiệm thu đầu vào", Record{"permissions": []string{"acceptance.view", "operation.view", "warehouse.view"}})
	}
	return s.incomingAcceptanceResponse(ctx, item)
}

func (s *Service) GetIncomingAcceptance(ctx context.Context, acceptanceID, actorID string) (Record, error) {
	record, err := s.repo.GetIncomingAcceptance(ctx, acceptanceID)
	if err != nil {
		return nil, err
	}
	projectID := cleanID(record["project_id"])
	allowed, err := s.repo.CanAccessProjectPermission(ctx, actorID, projectID, []string{"acceptance.view", "operation.view", "warehouse.view"})
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền xem nghiệm thu đầu vào", Record{"permissions": []string{"acceptance.view", "operation.view", "warehouse.view"}})
	}
	workItemID := cleanID(record["work_item_id"])
	if workItemID != "" {
		canView, err := s.repo.CanViewWorkItem(ctx, workItemID, actorID)
		if err != nil {
			return nil, err
		}
		if !canView {
			return nil, forbiddenError("Bạn không có quyền xem đầu việc này")
		}
	}
	return record, nil
}

func (s *Service) CreateIncomingAcceptance(ctx context.Context, projectID string, req IncomingAcceptanceRequest, actorID, actorName string) (Record, error) {
	projectID = cleanID(projectID)
	item, err := s.requireIncomingAcceptanceAccess(ctx, projectID, req.WorkItemID, actorID, []string{"acceptance.create", "operation.report"}, "Bạn không có quyền lưu nghiệm thu đầu vào")
	if err != nil {
		return nil, err
	}
	header, items, err := s.normalizeIncomingAcceptanceRequest(ctx, projectID, item, nil, req, false)
	if err != nil {
		return nil, err
	}
	return s.repo.SaveIncomingAcceptance(ctx, "", header, items, actorID, actorName)
}

func (s *Service) UpdateIncomingAcceptance(ctx context.Context, acceptanceID string, req IncomingAcceptanceRequest, actorID, actorName string) (Record, error) {
	existing, err := s.repo.GetIncomingAcceptance(ctx, acceptanceID)
	if err != nil {
		return nil, err
	}
	if fmt.Sprint(existing["status"]) == "submitted" {
		return nil, validationError("Hồ sơ nghiệm thu đầu vào đã gửi, không thể sửa", Record{"incoming_acceptance_id": acceptanceID})
	}
	projectID := cleanID(existing["project_id"])
	workItemID := req.WorkItemID
	if cleanID(workItemID) == "" {
		workItemID = cleanID(existing["work_item_id"])
	}
	item, err := s.requireIncomingAcceptanceAccess(ctx, projectID, workItemID, actorID, []string{"acceptance.create", "operation.report"}, "Bạn không có quyền lưu nghiệm thu đầu vào")
	if err != nil {
		return nil, err
	}
	header, items, err := s.normalizeIncomingAcceptanceRequest(ctx, projectID, item, existing, req, false)
	if err != nil {
		return nil, err
	}
	return s.repo.SaveIncomingAcceptance(ctx, cleanID(existing["id"]), header, items, actorID, actorName)
}

func (s *Service) SubmitIncomingAcceptance(ctx context.Context, acceptanceID string, actorID, actorName string) (Record, error) {
	existing, err := s.repo.GetIncomingAcceptance(ctx, acceptanceID)
	if err != nil {
		return nil, err
	}
	if fmt.Sprint(existing["status"]) == "submitted" {
		return nil, validationError("Hồ sơ nghiệm thu đầu vào đã gửi", Record{"incoming_acceptance_id": acceptanceID})
	}
	projectID := cleanID(existing["project_id"])
	workItemID := cleanID(existing["work_item_id"])
	item, err := s.requireIncomingAcceptanceAccess(ctx, projectID, workItemID, actorID, []string{"acceptance.approve", "acceptance.create", "operation.report"}, "Bạn không có quyền gửi nghiệm thu đầu vào")
	if err != nil {
		return nil, err
	}
	req := incomingAcceptanceRequestFromRecord(existing)
	header, _, err := s.normalizeIncomingAcceptanceRequest(ctx, projectID, item, existing, req, true)
	if err != nil {
		return nil, err
	}
	return s.repo.SubmitIncomingAcceptance(ctx, cleanID(existing["id"]), cleanOptionalText(header["result"]), actorID, actorName)
}

func (s *Service) requireIncomingAcceptanceAccess(ctx context.Context, projectID, workItemID, actorID string, permissions []string, message string) (Record, error) {
	projectID = cleanID(projectID)
	workItemID = cleanID(workItemID)
	allowed, err := s.repo.CanAccessProjectPermission(ctx, actorID, projectID, permissions)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError(message, Record{"permissions": permissions})
	}
	if workItemID == "" {
		return nil, nil
	}
	canAct, err := s.repo.CanActOnWorkItem(ctx, workItemID, actorID)
	if err != nil {
		return nil, err
	}
	if !canAct {
		return nil, forbiddenError("Bạn không có quyền thao tác đầu việc này")
	}
	item, err := s.repo.GetWorkItemDetail(ctx, workItemID)
	if err != nil {
		return nil, err
	}
	if err := s.requireActivePipelineWorkItem(ctx, item); err != nil {
		return nil, err
	}
	if cleanID(item["project_id"]) != projectID {
		return nil, validationError("Đầu việc không thuộc công trình nghiệm thu đầu vào", Record{"work_item_id": workItemID, "project_id": projectID})
	}
	if err := s.requireWorkItemPermission(ctx, item, actorID); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Service) incomingAcceptanceResponse(ctx context.Context, item Record) (Record, error) {
	projectID := cleanID(item["project_id"])
	materials, err := s.repo.ListActiveMaterials(ctx)
	if err != nil {
		return nil, err
	}
	warehouses, err := s.repo.ListActiveWarehouses(ctx)
	if err != nil {
		return nil, err
	}
	partners, err := s.repo.ListPartners(ctx, PartnerFilter{Status: "active"})
	if err != nil {
		return nil, err
	}
	acceptances, err := s.repo.ListIncomingAcceptancesForWorkItem(ctx, cleanID(item["id"]))
	if err != nil {
		return nil, err
	}
	procurementRequests, err := s.repo.ListProjectProcurementRequests(ctx, projectID)
	if err != nil {
		return nil, err
	}
	procurementItems, err := s.repo.ListProjectProcurementRequestItems(ctx, projectID)
	if err != nil {
		return nil, err
	}
	return Record{
		"available":                 true,
		"work_item_id":              item["id"],
		"project_id":                projectID,
		"records":                   acceptances,
		"materials":                 materials,
		"warehouses":                warehouses,
		"partners":                  partners,
		"procurement_requests":      procurementRequests,
		"procurement_request_items": procurementItems,
	}, nil
}

func (s *Service) normalizeIncomingAcceptanceRequest(ctx context.Context, projectID string, item Record, existing Record, req IncomingAcceptanceRequest, submit bool) (Record, []Record, error) {
	projectID = cleanID(projectID)
	workItemID := cleanID(req.WorkItemID)
	if workItemID == "" && item != nil {
		workItemID = cleanID(item["id"])
	}
	if workItemID == "" && existing != nil {
		workItemID = cleanID(existing["work_item_id"])
	}
	if workItemID != "" {
		if item == nil || cleanID(item["id"]) != workItemID {
			workItem, err := s.repo.GetWorkItemDetail(ctx, workItemID)
			if err != nil {
				return nil, nil, err
			}
			item = workItem
		}
		if cleanID(item["project_id"]) != projectID {
			return nil, nil, validationError("Đầu việc không thuộc công trình nghiệm thu đầu vào", Record{"work_item_id": workItemID, "project_id": projectID})
		}
	}

	sourceType := strings.TrimSpace(req.SourceType)
	procurementRequestID := cleanID(req.ProcurementRequestID)
	if sourceType == "" && procurementRequestID != "" {
		sourceType = "procurement"
	}
	if sourceType == "" {
		sourceType = "supplier"
	}
	if !statusIn(sourceType, "supplier", "subcontractor", "procurement", "other") {
		return nil, nil, validationError("Nguồn nghiệm thu đầu vào không hợp lệ", Record{"source_type": sourceType})
	}
	supplierPartnerID := cleanID(req.SupplierPartnerID)
	subcontractorPartnerID := cleanID(req.SubcontractorPartnerID)
	supplierName := strings.TrimSpace(req.SupplierName)
	subcontractorName := strings.TrimSpace(req.SubcontractorName)
	if supplierPartnerID != "" {
		partner, err := s.requireActivePartner(ctx, supplierPartnerID, []string{"supplier", "service_provider", "other"})
		if err != nil {
			return nil, nil, err
		}
		supplierName = cleanOptionalText(partner["name"])
	}
	if subcontractorPartnerID != "" {
		partner, err := s.requireActivePartner(ctx, subcontractorPartnerID, []string{"subcontractor", "labor_team"})
		if err != nil {
			return nil, nil, err
		}
		subcontractorName = cleanOptionalText(partner["name"])
	}
	deliveryReference := strings.TrimSpace(req.DeliveryReference)
	for field, value := range map[string]string{
		"supplier_name":      supplierName,
		"subcontractor_name": subcontractorName,
		"delivery_reference": deliveryReference,
	} {
		if len([]rune(value)) > 240 {
			return nil, nil, validationError("Thông tin nghiệm thu đầu vào không được vượt quá 240 ký tự", Record{"field": field})
		}
	}
	comment := strings.TrimSpace(req.Comment)
	if len([]rune(comment)) > 2000 {
		return nil, nil, validationError("Ghi chú nghiệm thu đầu vào không được vượt quá 2000 ký tự")
	}
	receivedAt := strings.TrimSpace(req.ReceivedAt)
	if receivedAt != "" {
		if _, err := time.Parse("2006-01-02", receivedAt); err != nil {
			return nil, nil, validationError("Ngày nhận vật tư không hợp lệ", Record{"received_at": receivedAt})
		}
	} else if submit {
		return nil, nil, validationError("Ngày nhận vật tư là bắt buộc khi gửi")
	}
	warehouseID := cleanID(req.WarehouseID)
	if warehouseID != "" {
		if err := s.requireActiveWarehouse(ctx, warehouseID); err != nil {
			return nil, nil, err
		}
	}

	procurementRequests, err := s.repo.ListProjectProcurementRequests(ctx, projectID)
	if err != nil {
		return nil, nil, err
	}
	if procurementRequestID != "" {
		found := false
		for _, request := range procurementRequests {
			if cleanID(request["id"]) == procurementRequestID {
				found = true
				break
			}
		}
		if !found {
			return nil, nil, validationError("Yêu cầu mua không thuộc công trình nghiệm thu đầu vào", Record{"procurement_request_id": procurementRequestID})
		}
	}
	materials, err := s.repo.ListActiveMaterials(ctx)
	if err != nil {
		return nil, nil, err
	}
	materialByID := map[string]Record{}
	for _, material := range materials {
		materialByID[cleanID(material["id"])] = material
	}
	procurementItems, err := s.repo.ListProjectProcurementRequestItems(ctx, projectID)
	if err != nil {
		return nil, nil, err
	}
	procurementItemByID := map[string]Record{}
	for _, item := range procurementItems {
		procurementItemByID[cleanID(item["id"])] = item
	}

	if submit && len(req.Items) == 0 {
		return nil, nil, validationError("Cần ít nhất một dòng vật tư để gửi nghiệm thu đầu vào")
	}
	seen := map[int]bool{}
	normalized := make([]Record, 0, len(req.Items))
	for index, itemReq := range req.Items {
		itemIndex := itemReq.ItemIndex
		if itemIndex < 0 {
			itemIndex = index
		}
		if itemIndex < 0 {
			return nil, nil, validationError("Dòng nghiệm thu đầu vào không hợp lệ", Record{"item_index": itemReq.ItemIndex})
		}
		if seen[itemIndex] {
			return nil, nil, validationError("Dòng nghiệm thu đầu vào bị trùng", Record{"item_index": itemIndex})
		}
		seen[itemIndex] = true
		row, err := s.normalizeIncomingAcceptanceItem(ctx, projectID, workItemID, procurementRequestID, itemIndex, materialByID, procurementItemByID, itemReq, submit)
		if err != nil {
			return nil, nil, err
		}
		normalized = append(normalized, row)
	}
	summary := incomingAcceptanceSummary(normalized)
	result := strings.TrimSpace(req.Result)
	if result == "" && submit {
		result = incomingAcceptanceResultFromSummary(summary)
	}
	if result != "" && !statusIn(result, "pass", "conditional_pass", "fail") {
		return nil, nil, validationError("Kết quả nghiệm thu đầu vào không hợp lệ", Record{"result": result})
	}
	if submit {
		if result == "" {
			return nil, nil, validationError("Kết quả nghiệm thu đầu vào là bắt buộc khi gửi")
		}
		if err := validateIncomingAcceptanceResult(result, summary); err != nil {
			return nil, nil, err
		}
	}
	header := Record{
		"project_id":               projectID,
		"work_item_id":             workItemID,
		"procurement_request_id":   procurementRequestID,
		"warehouse_id":             warehouseID,
		"source_type":              sourceType,
		"supplier_partner_id":      supplierPartnerID,
		"supplier_name":            supplierName,
		"subcontractor_partner_id": subcontractorPartnerID,
		"subcontractor_name":       subcontractorName,
		"delivery_reference":       deliveryReference,
		"received_at":              receivedAt,
		"result":                   result,
		"comment":                  comment,
		"summary_json":             summary,
		"incoming_material_count":  summary["total"],
	}
	return header, normalized, nil
}

func (s *Service) normalizeIncomingAcceptanceItem(ctx context.Context, projectID, workItemID, procurementRequestID string, itemIndex int, materialByID, procurementItemByID map[string]Record, req IncomingAcceptanceItemRequest, submit bool) (Record, error) {
	materialID := cleanID(req.MaterialID)
	procurementRequestItemID := cleanID(req.ProcurementRequestItemID)
	var procurementItem Record
	if procurementRequestItemID != "" {
		var ok bool
		procurementItem, ok = procurementItemByID[procurementRequestItemID]
		if !ok {
			return nil, validationError("Dòng yêu cầu mua không thuộc công trình nghiệm thu đầu vào", Record{"item_index": itemIndex, "procurement_request_item_id": procurementRequestItemID})
		}
		itemRequestID := cleanID(procurementItem["request_id"])
		if procurementRequestID != "" && itemRequestID != procurementRequestID {
			return nil, validationError("Dòng yêu cầu mua không thuộc yêu cầu mua đã chọn", Record{"item_index": itemIndex, "procurement_request_item_id": procurementRequestItemID})
		}
		procurementMaterialID := cleanID(procurementItem["material_id"])
		if materialID == "" {
			materialID = procurementMaterialID
		}
		if materialID != "" && procurementMaterialID != "" && materialID != procurementMaterialID {
			return nil, validationError("Vật tư không khớp dòng yêu cầu mua", Record{"item_index": itemIndex, "material_id": materialID, "procurement_request_item_id": procurementRequestItemID})
		}
	}

	var material Record
	if materialID != "" {
		var ok bool
		material, ok = materialByID[materialID]
		if !ok {
			return nil, validationError("Vật tư không tồn tại hoặc đã ngưng hoạt động", Record{"item_index": itemIndex, "material_id": materialID})
		}
	}
	materialName := strings.TrimSpace(req.MaterialName)
	materialCode := ""
	if material != nil {
		materialName = cleanOptionalText(material["name"])
		materialCode = cleanOptionalText(material["code"])
	} else if procurementItem != nil {
		materialName = cleanOptionalText(procurementItem["material_name"])
	}
	if materialName == "" {
		return nil, validationError("Tên vật tư nghiệm thu đầu vào là bắt buộc", Record{"item_index": itemIndex})
	}
	if len([]rune(materialName)) > 240 {
		return nil, validationError("Tên vật tư nghiệm thu đầu vào không được vượt quá 240 ký tự", Record{"item_index": itemIndex})
	}

	unit := strings.TrimSpace(req.Unit)
	if unit == "" && material != nil {
		unit = cleanOptionalText(material["unit"])
	}
	if unit == "" && procurementItem != nil {
		unit = cleanOptionalText(procurementItem["unit"])
	}
	if unit == "" {
		return nil, validationError("Đơn vị nghiệm thu đầu vào là bắt buộc", Record{"item_index": itemIndex})
	}
	if len([]rune(unit)) > 40 {
		return nil, validationError("Đơn vị nghiệm thu đầu vào không được vượt quá 40 ký tự", Record{"item_index": itemIndex})
	}

	deliveredQuantity, err := incomingAcceptanceQuantity(req.DeliveredQuantity, "Số lượng giao không hợp lệ", itemIndex)
	if err != nil {
		return nil, err
	}
	acceptedQuantity, err := incomingAcceptanceQuantity(req.AcceptedQuantity, "Số lượng nhận không hợp lệ", itemIndex)
	if err != nil {
		return nil, err
	}
	rejectedQuantity, err := incomingAcceptanceQuantity(req.RejectedQuantity, "Số lượng từ chối không hợp lệ", itemIndex)
	if err != nil {
		return nil, err
	}
	if submit && deliveredQuantity <= 0 {
		return nil, validationError("Số lượng giao phải lớn hơn 0 khi gửi", Record{"item_index": itemIndex})
	}
	if materialSnapshotQuantityExceeds(acceptedQuantity+rejectedQuantity, deliveredQuantity) {
		return nil, validationError("Số lượng nhận và từ chối vượt số lượng giao", Record{"item_index": itemIndex, "delivered_quantity": deliveredQuantity})
	}
	if submit && procurementItem != nil {
		alreadyAccepted, err := s.repo.QueryOne(ctx, `
			SELECT COALESCE(SUM(iai.accepted_quantity),0) AS accepted_quantity
			FROM incoming_acceptance_items iai
			JOIN incoming_acceptance_records iar ON iar.id=iai.incoming_acceptance_id
			WHERE iai.procurement_request_item_id=$1 AND iar.status='submitted'
		`, procurementRequestItemID)
		if err != nil {
			return nil, err
		}
		acceptedBefore := number(alreadyAccepted["accepted_quantity"])
		requested := number(procurementItem["quantity_requested"])
		if materialSnapshotQuantityExceeds(acceptedBefore+acceptedQuantity, requested) {
			return nil, validationError("Khối lượng nghiệm thu lũy kế vượt khối lượng yêu cầu mua", Record{
				"item_index": itemIndex, "quantity_requested": requested, "accepted_before": acceptedBefore, "accepted_quantity": acceptedQuantity,
			})
		}
	}

	condition := strings.TrimSpace(req.Condition)
	if condition == "" {
		condition = "usable"
	}
	if !statusIn(condition, "usable", "damaged", "mixed", "unknown") {
		return nil, validationError("Tình trạng vật tư nghiệm thu đầu vào không hợp lệ", Record{"item_index": itemIndex, "condition": condition})
	}
	decision := strings.TrimSpace(req.Decision)
	if decision == "" {
		decision = incomingAcceptanceDefaultDecision(deliveredQuantity, acceptedQuantity, rejectedQuantity)
	}
	if !statusIn(decision, "pending", "accepted", "conditional_acceptance", "rejected") {
		return nil, validationError("Quyết định dòng nghiệm thu đầu vào không hợp lệ", Record{"item_index": itemIndex, "decision": decision})
	}
	if submit {
		if decision == "pending" {
			return nil, validationError("Dòng nghiệm thu đầu vào chưa có quyết định", Record{"item_index": itemIndex})
		}
		if err := validateIncomingAcceptanceDecision(decision, deliveredQuantity, acceptedQuantity, rejectedQuantity); err != nil {
			return nil, validationError(err.Error(), Record{"item_index": itemIndex, "decision": decision})
		}
	}

	defectNote := strings.TrimSpace(req.DefectNote)
	if len([]rune(defectNote)) > 1000 {
		return nil, validationError("Mô tả lỗi nghiệm thu đầu vào không được vượt quá 1000 ký tự", Record{"item_index": itemIndex})
	}
	comment := strings.TrimSpace(req.Comment)
	if len([]rune(comment)) > 1000 {
		return nil, validationError("Ghi chú dòng nghiệm thu đầu vào không được vượt quá 1000 ký tự", Record{"item_index": itemIndex})
	}
	evidenceID := cleanID(req.EvidenceID)
	evidenceRequired := decision != "accepted" || condition != "usable" || rejectedQuantity > 0
	if submit && evidenceRequired && evidenceID == "" {
		return nil, validationError("Dòng nghiệm thu lỗi/có điều kiện cần minh chứng", Record{"item_index": itemIndex})
	}
	if submit && evidenceRequired && defectNote == "" {
		return nil, validationError("Dòng nghiệm thu lỗi/có điều kiện cần mô tả lỗi", Record{"item_index": itemIndex})
	}
	if evidenceID != "" {
		evidence, err := s.repo.GetEvidenceDetail(ctx, evidenceID)
		if err != nil {
			return nil, validationError("Minh chứng nghiệm thu đầu vào không hợp lệ", Record{"item_index": itemIndex, "evidence_id": evidenceID})
		}
		if cleanID(evidence["project_id"]) != projectID || fmt.Sprint(evidence["status"]) == "rejected" {
			return nil, validationError("Minh chứng nghiệm thu đầu vào không thuộc công trình", Record{"item_index": itemIndex, "evidence_id": evidenceID})
		}
		if workItemID != "" && cleanID(evidence["work_item_id"]) != workItemID {
			return nil, validationError("Minh chứng nghiệm thu đầu vào không thuộc đầu việc", Record{"item_index": itemIndex, "evidence_id": evidenceID})
		}
	}

	return Record{
		"item_index":                  itemIndex,
		"material_id":                 materialID,
		"material_code":               materialCode,
		"material_name":               materialName,
		"procurement_request_item_id": procurementRequestItemID,
		"delivered_quantity":          deliveredQuantity,
		"accepted_quantity":           acceptedQuantity,
		"rejected_quantity":           rejectedQuantity,
		"unit":                        unit,
		"condition":                   condition,
		"defect_note":                 defectNote,
		"evidence_id":                 evidenceID,
		"decision":                    decision,
		"comment":                     comment,
		"payload_json": Record{
			"procurement_request_item_id": procurementRequestItemID,
			"accepted_plus_rejected":      materialSnapshotRound(acceptedQuantity + rejectedQuantity),
		},
	}, nil
}

func incomingAcceptanceRequestFromRecord(record Record) IncomingAcceptanceRequest {
	req := IncomingAcceptanceRequest{
		WorkItemID:             cleanID(record["work_item_id"]),
		ProcurementRequestID:   cleanID(record["procurement_request_id"]),
		WarehouseID:            cleanID(record["warehouse_id"]),
		SourceType:             cleanOptionalText(record["source_type"]),
		SupplierPartnerID:      cleanID(record["supplier_partner_id"]),
		SupplierName:           cleanOptionalText(record["supplier_name"]),
		SubcontractorPartnerID: cleanID(record["subcontractor_partner_id"]),
		SubcontractorName:      cleanOptionalText(record["subcontractor_name"]),
		DeliveryReference:      cleanOptionalText(record["delivery_reference"]),
		ReceivedAt:             dateOnly(record["received_at"]),
		Result:                 cleanOptionalText(record["result"]),
		Comment:                cleanOptionalText(record["comment"]),
		Items:                  []IncomingAcceptanceItemRequest{},
	}
	if items, ok := record["items"].([]Record); ok {
		req.Items = make([]IncomingAcceptanceItemRequest, 0, len(items))
		for _, item := range items {
			delivered := number(item["delivered_quantity"])
			accepted := number(item["accepted_quantity"])
			rejected := number(item["rejected_quantity"])
			req.Items = append(req.Items, IncomingAcceptanceItemRequest{
				ItemIndex:                int(number(item["item_index"])),
				MaterialID:               cleanID(item["material_id"]),
				MaterialName:             cleanOptionalText(item["material_name"]),
				ProcurementRequestItemID: cleanID(item["procurement_request_item_id"]),
				DeliveredQuantity:        &delivered,
				AcceptedQuantity:         &accepted,
				RejectedQuantity:         &rejected,
				Unit:                     cleanOptionalText(item["unit"]),
				Condition:                cleanOptionalText(item["condition"]),
				DefectNote:               cleanOptionalText(item["defect_note"]),
				EvidenceID:               cleanID(item["evidence_id"]),
				Decision:                 cleanOptionalText(item["decision"]),
				Comment:                  cleanOptionalText(item["comment"]),
			})
		}
	}
	return req
}

func incomingAcceptanceQuantity(value *float64, message string, itemIndex int) (float64, error) {
	if value == nil {
		return 0, nil
	}
	if math.IsNaN(*value) || math.IsInf(*value, 0) || *value < 0 {
		return 0, validationError(message, Record{"item_index": itemIndex, "quantity": *value})
	}
	return materialSnapshotRound(*value), nil
}

func incomingAcceptanceDefaultDecision(delivered, accepted, rejected float64) string {
	if delivered <= 0 && accepted <= 0 && rejected <= 0 {
		return "pending"
	}
	if rejected > 0 && accepted > 0 {
		return "conditional_acceptance"
	}
	if rejected > 0 {
		return "rejected"
	}
	if accepted > 0 && !materialSnapshotQuantityExceeds(delivered, accepted) && !materialSnapshotQuantityExceeds(accepted, delivered) {
		return "accepted"
	}
	return "pending"
}

func validateIncomingAcceptanceDecision(decision string, delivered, accepted, rejected float64) error {
	switch decision {
	case "accepted":
		if rejected > 0 || accepted <= 0 || materialSnapshotQuantityExceeds(accepted, delivered) || materialSnapshotQuantityExceeds(delivered, accepted) {
			return errors.New("Dòng được chấp nhận phải nhận đủ và không có số lượng từ chối")
		}
	case "conditional_acceptance":
		if accepted <= 0 || rejected <= 0 {
			return errors.New("Dòng có điều kiện cần có cả số lượng nhận và từ chối")
		}
	case "rejected":
		if rejected <= 0 || accepted > 0 {
			return errors.New("Dòng bị từ chối phải có số lượng từ chối và không có số lượng nhận")
		}
	}
	return nil
}

func incomingAcceptanceSummary(items []Record) Record {
	summary := Record{
		"total":                len(items),
		"accepted":             0,
		"conditional":          0,
		"rejected":             0,
		"pending":              0,
		"defect_count":         0,
		"evidence_count":       0,
		"delivered_quantity":   0.0,
		"accepted_quantity":    0.0,
		"rejected_quantity":    0.0,
		"with_procurement_ref": 0,
	}
	for _, item := range items {
		switch cleanOptionalText(item["decision"]) {
		case "accepted":
			summary["accepted"] = int(number(summary["accepted"])) + 1
		case "conditional_acceptance":
			summary["conditional"] = int(number(summary["conditional"])) + 1
		case "rejected":
			summary["rejected"] = int(number(summary["rejected"])) + 1
		default:
			summary["pending"] = int(number(summary["pending"])) + 1
		}
		if cleanOptionalText(item["defect_note"]) != "" || number(item["rejected_quantity"]) > 0 || cleanOptionalText(item["condition"]) != "usable" {
			summary["defect_count"] = int(number(summary["defect_count"])) + 1
		}
		if cleanID(item["evidence_id"]) != "" {
			summary["evidence_count"] = int(number(summary["evidence_count"])) + 1
		}
		if cleanID(item["procurement_request_item_id"]) != "" {
			summary["with_procurement_ref"] = int(number(summary["with_procurement_ref"])) + 1
		}
		summary["delivered_quantity"] = materialSnapshotRound(number(summary["delivered_quantity"]) + number(item["delivered_quantity"]))
		summary["accepted_quantity"] = materialSnapshotRound(number(summary["accepted_quantity"]) + number(item["accepted_quantity"]))
		summary["rejected_quantity"] = materialSnapshotRound(number(summary["rejected_quantity"]) + number(item["rejected_quantity"]))
	}
	return summary
}

func incomingAcceptanceResultFromSummary(summary Record) string {
	total := int(number(summary["total"]))
	accepted := int(number(summary["accepted"]))
	conditional := int(number(summary["conditional"]))
	rejected := int(number(summary["rejected"]))
	if total == 0 {
		return ""
	}
	if accepted == total {
		return "pass"
	}
	if rejected == total {
		return "fail"
	}
	if conditional > 0 || rejected > 0 {
		return "conditional_pass"
	}
	return ""
}

func validateIncomingAcceptanceResult(result string, summary Record) error {
	total := int(number(summary["total"]))
	accepted := int(number(summary["accepted"]))
	conditional := int(number(summary["conditional"]))
	rejected := int(number(summary["rejected"]))
	pending := int(number(summary["pending"]))
	if total == 0 {
		return validationError("Cần ít nhất một dòng vật tư để gửi nghiệm thu đầu vào")
	}
	if pending > 0 {
		return validationError("Không thể gửi nghiệm thu đầu vào khi còn dòng chờ xử lý", Record{"pending": pending})
	}
	switch result {
	case "pass":
		if accepted != total {
			return validationError("Kết quả đạt chỉ hợp lệ khi mọi dòng đều được chấp nhận")
		}
	case "conditional_pass":
		if conditional+rejected == 0 {
			return validationError("Kết quả đạt có điều kiện cần có dòng có điều kiện hoặc bị từ chối")
		}
	case "fail":
		if rejected == 0 || accepted > 0 || conditional > 0 {
			return validationError("Kết quả không đạt chỉ hợp lệ khi có dòng bị từ chối và không có dòng được chấp nhận/có điều kiện")
		}
	}
	return nil
}

func dateOnly(value any) string {
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "" || text == "<nil>" {
		return ""
	}
	if len(text) >= 10 {
		return text[:10]
	}
	return text
}
