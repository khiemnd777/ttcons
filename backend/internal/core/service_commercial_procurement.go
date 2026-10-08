package core

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"
)

func (s *Service) ProjectBOQ(ctx context.Context, projectID string) ([]Record, error) {
	if cleanID(projectID) == "" {
		return nil, validationError("project_id không hợp lệ", Record{"project_id": projectID})
	}
	return s.repo.ListProjectBOQ(ctx, projectID)
}

func (s *Service) GetBOQ(ctx context.Context, itemID string) (Record, error) {
	itemID = cleanID(itemID)
	if itemID == "" {
		return nil, validationError("Mã dòng BOQ không hợp lệ", Record{"boq_item_id": itemID})
	}
	return s.repo.GetBOQItemDetail(ctx, itemID)
}

func (s *Service) BOQProjectAction(ctx context.Context, projectID, action string, mutationContext WorkItemMutationContext, actorID, actorName string) (Record, error) {
	projectID = cleanID(projectID)
	if projectID == "" {
		return nil, validationError("project_id không hợp lệ")
	}
	return s.repo.BOQProjectAction(ctx, projectID, action, mutationContext, actorID, actorName)
}

func (s *Service) CreateBOQ(ctx context.Context, projectID string, body Record, actorID, actorName string) (Record, error) {
	normalized, err := normalizeBOQItemRequest(projectID, body, true)
	if err != nil {
		return nil, err
	}
	return s.repo.CreateBOQItem(ctx, normalized, actorID, actorName)
}

func (s *Service) UpdateBOQ(ctx context.Context, itemID string, body Record, actorID, actorName string) (Record, error) {
	if cleanID(itemID) == "" {
		return nil, validationError("Mã dòng BOQ không hợp lệ", Record{"boq_item_id": itemID})
	}
	normalized, err := normalizeBOQItemRequest("", body, false)
	if err != nil {
		return nil, err
	}
	return s.repo.UpdateBOQItem(ctx, itemID, normalized, actorID, actorName)
}

func (s *Service) ReviseBOQ(ctx context.Context, itemID string, req BOQReviseRequest, actorID, actorName string) (Record, error) {
	itemID = cleanID(itemID)
	if itemID == "" {
		return nil, validationError("Mã dòng BOQ không hợp lệ", Record{"boq_item_id": itemID})
	}
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		return nil, validationError("Yêu cầu chỉnh sửa BOQ cần nêu lý do")
	}
	if len([]rune(reason)) > 2000 {
		return nil, validationError("Lý do chỉnh sửa BOQ không được vượt quá 2000 ký tự")
	}
	return s.repo.ReviseBOQItem(ctx, itemID, reason, actorID, actorName)
}

func (s *Service) UpdateBOQExpectedUnitPrice(ctx context.Context, itemID string, req BOQUnitPriceRequest, actorID, actorName string) (Record, error) {
	unitPrice, comment, err := normalizeBOQUnitPriceRequest(req, false)
	if err != nil {
		return nil, err
	}
	return s.repo.UpdateBOQExpectedUnitPrice(ctx, itemID, unitPrice, comment, actorID, actorName)
}

func (s *Service) LockBOQNormUnitPrice(ctx context.Context, itemID string, req BOQUnitPriceRequest, actorID, actorName string) (Record, error) {
	unitPrice, comment, err := normalizeBOQUnitPriceRequest(req, true)
	if err != nil {
		return nil, err
	}
	return s.repo.LockBOQNormUnitPrice(ctx, itemID, unitPrice, comment, actorID, actorName)
}

func (s *Service) LockBOQFinalUnitPrice(ctx context.Context, itemID string, req BOQUnitPriceRequest, actorID, actorName string) (Record, error) {
	unitPrice, comment, err := normalizeBOQUnitPriceRequest(req, true)
	if err != nil {
		return nil, err
	}
	if comment == "" {
		return nil, validationError("Khóa giá cuối thủ công cần nêu lý do miễn so sánh báo giá")
	}
	return s.repo.LockBOQFinalUnitPrice(ctx, itemID, unitPrice, comment, actorID, actorName)
}

func (s *Service) ProjectSupplierQuotations(ctx context.Context, projectID string) ([]Record, error) {
	return s.repo.ListProjectSupplierQuotations(ctx, cleanID(projectID))
}

func (s *Service) SupplierQuotationDetail(ctx context.Context, quotationID string) (Record, error) {
	return s.repo.GetSupplierQuotationDetail(ctx, cleanID(quotationID))
}

func (s *Service) SaveSupplierQuotation(ctx context.Context, projectID, quotationID string, req SupplierQuotationRequest, actorID, actorName string) (Record, error) {
	projectID = cleanID(projectID)
	if projectID == "" {
		return nil, validationError("project_id không hợp lệ")
	}
	partner, err := s.requireActivePartner(ctx, cleanID(req.SupplierPartnerID), []string{"supplier", "service_provider", "other"})
	if err != nil {
		return nil, err
	}
	header, items, err := s.normalizeSupplierQuotation(ctx, projectID, req)
	if err != nil {
		return nil, err
	}
	header["supplier_partner_id"] = cleanID(partner["id"])
	header["supplier_name"] = cleanOptionalText(partner["name"])
	if cleanOptionalText(header["quotation_code"]) == "" {
		header["quotation_code"] = fmt.Sprintf("BG-%s-%d", projectID[:8], timeNowUnix())
	}
	return s.repo.SaveSupplierQuotation(ctx, projectID, cleanID(quotationID), header, items, req.WorkItemContext, actorID, actorName)
}

func (s *Service) SubmitSupplierQuotation(ctx context.Context, quotationID string, mutationContext WorkItemMutationContext, actorID, actorName string) (Record, error) {
	return s.repo.SupplierQuotationAction(ctx, cleanID(quotationID), "submit", "", "", "", mutationContext, actorID, actorName)
}

func (s *Service) QualifySupplierQuotation(ctx context.Context, quotationID string, req SupplierQuotationQualificationRequest, actorID, actorName string) (Record, error) {
	technicalStatus := strings.TrimSpace(req.TechnicalStatus)
	commercialStatus := strings.TrimSpace(req.CommercialStatus)
	comment := strings.TrimSpace(req.Comment)
	if !statusIn(technicalStatus, "compliant", "conditional", "non_compliant") {
		return nil, validationError("Kết quả đánh giá kỹ thuật không hợp lệ", Record{"technical_status": technicalStatus})
	}
	if !statusIn(commercialStatus, "compliant", "non_compliant") {
		return nil, validationError("Kết quả đánh giá thương mại không hợp lệ", Record{"commercial_status": commercialStatus})
	}
	if comment == "" {
		return nil, validationError("Đánh giá báo giá cần có nhận xét")
	}
	return s.repo.SupplierQuotationAction(ctx, cleanID(quotationID), "qualify", technicalStatus, commercialStatus, comment, req.WorkItemContext, actorID, actorName)
}

func (s *Service) SelectSupplierQuote(ctx context.Context, quotationItemID string, req SupplierQuoteSelectionRequest, actorID, actorName string) (Record, error) {
	reason := strings.TrimSpace(req.DecisionReason)
	if reason == "" {
		return nil, validationError("Quyết định chọn báo giá cần nêu lý do")
	}
	if len([]rune(reason)) > 2000 {
		return nil, validationError("Lý do chọn báo giá không được vượt quá 2000 ký tự")
	}
	return s.repo.SelectSupplierQuote(ctx, cleanID(quotationItemID), reason, req.WorkItemContext, actorID, actorName)
}

func (s *Service) SupplierQuoteComparison(ctx context.Context, projectID string) ([]Record, error) {
	return s.repo.SupplierQuoteComparison(ctx, cleanID(projectID))
}

func (s *Service) normalizeSupplierQuotation(ctx context.Context, projectID string, req SupplierQuotationRequest) (Record, []Record, error) {
	quotationDate := strings.TrimSpace(req.QuotationDate)
	if quotationDate == "" {
		quotationDate = time.Now().Format("2006-01-02")
	}
	quotedAt, err := time.Parse("2006-01-02", quotationDate)
	if err != nil {
		return nil, nil, validationError("Ngày báo giá không hợp lệ")
	}
	validUntil := strings.TrimSpace(req.ValidUntil)
	if validUntil != "" {
		validAt, err := time.Parse("2006-01-02", validUntil)
		if err != nil || validAt.Before(quotedAt) {
			return nil, nil, validationError("Hạn hiệu lực báo giá không hợp lệ")
		}
	}
	currency := strings.ToUpper(strings.TrimSpace(req.Currency))
	if currency == "" {
		currency = "VND"
	}
	if len(currency) != 3 {
		return nil, nil, validationError("Mã tiền tệ phải gồm 3 ký tự")
	}
	if req.PaymentTermsDays < 0 || req.LeadTimeDays < 0 {
		return nil, nil, validationError("Số ngày thanh toán/giao hàng không được âm")
	}
	if len(req.Items) == 0 {
		return nil, nil, validationError("Báo giá cần ít nhất một dòng BOQ")
	}
	items := make([]Record, 0, len(req.Items))
	seen := map[string]bool{}
	for index, itemReq := range req.Items {
		boqID := cleanID(itemReq.BOQItemID)
		if boqID == "" || seen[boqID] {
			return nil, nil, validationError("Dòng BOQ trong báo giá không hợp lệ hoặc bị trùng", Record{"item_index": index})
		}
		seen[boqID] = true
		boq, err := s.repo.GetBOQItemDetail(ctx, boqID)
		if err != nil || cleanID(boq["project_id"]) != projectID {
			return nil, nil, validationError("Dòng BOQ không thuộc công trình", Record{"item_index": index, "boq_item_id": boqID})
		}
		if fmt.Sprint(boq["status"]) == "deleted" || boolValue(boq["final_price_locked"]) {
			return nil, nil, validationError("Dòng BOQ đã khóa giá hoặc không còn hiệu lực", Record{"item_index": index, "boq_item_id": boqID})
		}
		quantity := itemReq.Quantity
		if quantity <= 0 || math.IsNaN(quantity) || math.IsInf(quantity, 0) {
			quantity = number(boq["quantity"])
		}
		if quantity <= 0 || itemReq.UnitPrice <= 0 || math.IsNaN(itemReq.UnitPrice) || math.IsInf(itemReq.UnitPrice, 0) {
			return nil, nil, validationError("Khối lượng và đơn giá báo giá phải lớn hơn 0", Record{"item_index": index})
		}
		if itemReq.DiscountAmount < 0 || itemReq.DiscountAmount > quantity*itemReq.UnitPrice || itemReq.TaxRate < 0 || itemReq.TaxRate > 100 || itemReq.LeadTimeDays < 0 {
			return nil, nil, validationError("Chiết khấu, thuế hoặc thời gian giao hàng không hợp lệ", Record{"item_index": index})
		}
		compliance := strings.TrimSpace(itemReq.ComplianceStatus)
		if compliance == "" {
			compliance = "pending"
		}
		if !statusIn(compliance, "pending", "compliant", "conditional", "non_compliant") {
			return nil, nil, validationError("Mức đáp ứng của dòng báo giá không hợp lệ", Record{"item_index": index})
		}
		description := strings.TrimSpace(itemReq.Description)
		if description == "" {
			description = cleanOptionalText(boq["work_item_name"])
		}
		unit := strings.TrimSpace(itemReq.Unit)
		if unit == "" {
			unit = cleanOptionalText(boq["unit"])
		}
		items = append(items, Record{
			"boq_item_id": boqID, "material_id": cleanID(boq["material_id"]), "description": description,
			"unit": unit, "quantity": quantity, "unit_price": itemReq.UnitPrice, "discount_amount": itemReq.DiscountAmount,
			"tax_rate": itemReq.TaxRate, "lead_time_days": itemReq.LeadTimeDays, "compliance_status": compliance,
			"deviation_note": strings.TrimSpace(itemReq.DeviationNote),
		})
	}
	header := Record{
		"quotation_code": strings.TrimSpace(req.QuotationCode), "quotation_date": quotationDate, "valid_until": validUntil,
		"currency": currency, "payment_terms_days": req.PaymentTermsDays, "lead_time_days": req.LeadTimeDays,
		"delivery_terms": strings.TrimSpace(req.DeliveryTerms), "warranty_terms": strings.TrimSpace(req.WarrantyTerms), "notes": strings.TrimSpace(req.Notes),
	}
	return header, items, nil
}

func normalizeBOQUnitPriceRequest(req BOQUnitPriceRequest, lock bool) (float64, string, error) {
	if math.IsNaN(req.UnitPrice) || math.IsInf(req.UnitPrice, 0) {
		return 0, "", validationError("Đơn giá không hợp lệ")
	}
	if lock && req.UnitPrice <= 0 {
		return 0, "", validationError("Đơn giá lock phải lớn hơn 0")
	}
	if !lock && req.UnitPrice < 0 {
		return 0, "", validationError("Expected unit price không được âm")
	}
	comment := strings.TrimSpace(req.Comment)
	if len([]rune(comment)) > 1000 {
		return 0, "", validationError("Ghi chú đơn giá không được vượt quá 1000 ký tự")
	}
	return req.UnitPrice, comment, nil
}

func normalizeBOQItemRequest(projectID string, body Record, create bool) (Record, error) {
	out := Record{}
	if projectID != "" {
		if cleanID(projectID) == "" {
			return nil, validationError("project_id không hợp lệ", Record{"project_id": projectID})
		}
		out["project_id"] = projectID
	}
	textFields := map[string]int{
		"category":       120,
		"work_item_name": 180,
		"description":    2000,
		"unit":           40,
	}
	for field, maxLen := range textFields {
		value, ok := body[field]
		if !ok {
			continue
		}
		text := strings.TrimSpace(fmt.Sprint(value))
		if len([]rune(text)) > maxLen {
			return nil, validationError("Dữ liệu BOQ vượt quá độ dài cho phép", Record{"field": field, "max_length": maxLen})
		}
		out[field] = text
	}
	if create {
		for _, field := range []string{"category", "work_item_name", "unit"} {
			if strings.TrimSpace(fmt.Sprint(out[field])) == "" {
				return nil, validationError("Dòng BOQ thiếu dữ liệu bắt buộc", Record{"field": field})
			}
		}
	}
	if value, ok := body["quantity"]; ok {
		quantity, err := workflowNumberValue(value)
		if err != nil {
			return nil, validationError("Khối lượng BOQ phải là số")
		}
		if quantity < 0 {
			return nil, validationError("Khối lượng BOQ không được âm")
		}
		out["quantity"] = quantity
	} else if create {
		out["quantity"] = float64(0)
	}
	priceValue, hasExpected := body["expected_unit_price"]
	if !hasExpected {
		priceValue, hasExpected = body["unit_price"]
	}
	if hasExpected {
		price, err := workflowNumberValue(priceValue)
		if err != nil {
			return nil, validationError("Expected unit price phải là số")
		}
		if price < 0 {
			return nil, validationError("Expected unit price không được âm")
		}
		out["expected_unit_price"] = price
	} else if create {
		out["expected_unit_price"] = float64(0)
	}
	for _, field := range []string{"material_id", "node_id"} {
		if value, ok := body[field]; ok {
			out[field] = cleanID(value)
		}
	}
	if create {
		out["status"] = "draft"
	}
	return out, nil
}

func (s *Service) GenerateProcurementFromBOQ(ctx context.Context, projectID string, mutationContext WorkItemMutationContext, actorID, actorName string) (Record, error) {
	projectID = cleanID(projectID)
	canGenerate, err := s.repo.CanAccessProjectPermission(ctx, actorID, projectID, []string{"procurement.create", "procurement.approve", "boq.approve"})
	if err != nil {
		return nil, err
	}
	if !canGenerate {
		return nil, forbiddenError("Bạn không có quyền sinh yêu cầu mua từ BOQ", Record{"permissions": []string{"procurement.create", "procurement.approve", "boq.approve"}})
	}
	result, err := s.repo.GenerateProcurementFromBOQ(ctx, projectID, mutationContext, actorID, actorName)
	if err == nil {
		s.publishDiscussionWorkUpdate(projectID, cleanID(mutationContext.WorkItemID), actorID)
	}
	return result, err
}

func (s *Service) ProcurementProjects(ctx context.Context, req ReportOptionsRequest, actorID string) (ReportOptionsResponse, error) {
	return s.projectOptions(ctx, req, actorID, "procurement.view")
}

func (s *Service) CreateProcurement(ctx context.Context, body Record, actorID, actorName string) (Record, error) {
	normalized, err := s.normalizeProcurementHeader(ctx, body, true)
	if err != nil {
		return nil, err
	}
	normalized["requested_by"] = actorID
	normalized["status"] = "draft"
	row, err := s.repo.Insert(ctx, "procurement_requests", procurementColumns(), normalized)
	if err != nil {
		return nil, err
	}
	s.repo.AddAuditBestEffort(ctx, cleanID(row["project_id"]), actorID, actorName, "procurement_created", "procurement_request", cleanID(row["id"]), "Tạo yêu cầu mua", Record{"request_code": row["request_code"]})
	return row, nil
}

func (s *Service) UpdateProcurement(ctx context.Context, id string, body Record, actorID, actorName string) (Record, error) {
	current, err := s.repo.GetByID(ctx, "procurement_requests", cleanID(id))
	if err != nil {
		return nil, err
	}
	if statusIn(fmt.Sprint(current["status"]), "ordered", "partially_received", "received", "cancelled") {
		return nil, validationError("Yêu cầu mua đã đặt/nhận hàng, không thể sửa thông tin thương mại", Record{"status": current["status"]})
	}
	version := int(number(current["edit_version"]))
	if expected, present := body["expected_version"]; present && number(expected) != float64(version) {
		return nil, procurementVersionConflict()
	}
	// Only header fields are editable. Lifecycle, approvals and amounts remain
	// the source record's values, even for older callers.
	merged := Record{}
	for key, value := range current {
		merged[key] = value
	}
	for _, key := range []string{"title", "supplier_partner_id", "order_reference", "expected_delivery_date"} {
		if value, present := body[key]; present {
			merged[key] = value
		}
	}
	normalized, err := s.normalizeProcurementHeader(ctx, merged, false)
	if err != nil {
		return nil, err
	}
	row, err := s.repo.UpdateProcurementHeader(ctx, id, normalized, version, actorID, actorName)
	if err != nil {
		return nil, err
	}
	s.publishDiscussionWorkUpdate(cleanID(row["project_id"]), "", actorID)
	return row, nil
}

func (s *Service) normalizeProcurementHeader(ctx context.Context, body Record, create bool) (Record, error) {
	out := Record{}
	projectID := cleanID(body["project_id"])
	if projectID == "" {
		return nil, validationError("Công trình là bắt buộc")
	}
	out["project_id"] = projectID
	requestCode := strings.TrimSpace(fmt.Sprint(body["request_code"]))
	if requestCode == "" && create {
		requestCode = fmt.Sprintf("PR-%d", timeNowUnix())
	}
	if requestCode == "" {
		return nil, validationError("Mã yêu cầu mua là bắt buộc")
	}
	title := strings.TrimSpace(fmt.Sprint(body["title"]))
	if title == "" {
		return nil, validationError("Tiêu đề yêu cầu mua là bắt buộc")
	}
	out["request_code"] = requestCode
	out["title"] = title
	out["requested_by"] = cleanID(body["requested_by"])
	out["status"] = strings.TrimSpace(fmt.Sprint(body["status"]))
	if out["status"] == "" {
		out["status"] = "draft"
	}
	total := number(body["total_estimated_amount"])
	if total < 0 {
		return nil, validationError("Giá trị dự kiến không được âm")
	}
	out["total_estimated_amount"] = total
	budgetStatus := strings.TrimSpace(fmt.Sprint(body["budget_status"]))
	if budgetStatus == "" {
		budgetStatus = "within_budget"
	}
	if !statusIn(budgetStatus, "within_budget", "budget_exceeded") {
		return nil, validationError("Trạng thái ngân sách không hợp lệ")
	}
	out["budget_status"] = budgetStatus
	out["approval_request_id"] = cleanID(body["approval_request_id"])
	partnerID := cleanID(body["supplier_partner_id"])
	if partnerID != "" {
		partner, err := s.requireActivePartner(ctx, partnerID, []string{"supplier", "service_provider", "other"})
		if err != nil {
			return nil, err
		}
		out["supplier_partner_id"] = partnerID
		out["supplier_name"] = cleanOptionalText(partner["name"])
	} else {
		out["supplier_partner_id"] = ""
		out["supplier_name"] = cleanOptionalText(body["supplier_name"])
	}
	out["order_reference"] = cleanOptionalText(body["order_reference"])
	expectedDeliveryDate := dateOnly(body["expected_delivery_date"])
	if expectedDeliveryDate != "" {
		if _, err := time.Parse("2006-01-02", expectedDeliveryDate); err != nil {
			return nil, validationError("Ngày giao dự kiến không hợp lệ")
		}
	}
	out["expected_delivery_date"] = expectedDeliveryDate
	return out, nil
}

func (s *Service) ProcurementAction(ctx context.Context, id, action string, req ActionRequest, actorID, actorName string) (Record, error) {
	result, err := s.repo.ProcurementAction(ctx, id, action, req, actorID, actorName)
	if err == nil {
		s.publishDiscussionWorkUpdate(cleanID(result["project_id"]), cleanID(req.WorkItemContext.WorkItemID), actorID)
	}
	return result, err
}

func (s *Service) ProjectContracts(ctx context.Context, projectID string) ([]Record, error) {
	return s.repo.ListProjectContracts(ctx, cleanID(projectID))
}

func (s *Service) ContractDetail(ctx context.Context, contractID string) (Record, error) {
	return s.repo.GetContractDetail(ctx, cleanID(contractID))
}

func (s *Service) SaveContract(ctx context.Context, projectID, contractID string, req ContractRequest, actorID, actorName string) (Record, error) {
	projectID = cleanID(projectID)
	if projectID == "" {
		return nil, validationError("project_id không hợp lệ")
	}
	contractType := strings.TrimSpace(req.ContractType)
	direction := strings.TrimSpace(req.Direction)
	if !statusIn(contractType, "main_client", "subcontractor", "supplier", "service", "other") {
		return nil, validationError("Loại hợp đồng không hợp lệ")
	}
	if !statusIn(direction, "receivable", "payable") || (contractType == "main_client" && direction != "receivable") {
		return nil, validationError("Chiều tài chính của hợp đồng không hợp lệ")
	}
	partnerID := cleanID(req.CounterpartyPartnerID)
	partnerName := strings.TrimSpace(req.CounterpartyName)
	if partnerID != "" {
		allowedTypes := vendorPartnerTypes()
		if direction == "receivable" {
			allowedTypes = []string{"client"}
		}
		partner, err := s.requireActivePartner(ctx, partnerID, allowedTypes)
		if err != nil {
			return nil, err
		}
		partnerName = cleanOptionalText(partner["name"])
	}
	if partnerName == "" {
		return nil, validationError("Đối tác hợp đồng là bắt buộc")
	}
	code := strings.TrimSpace(req.ContractCode)
	title := strings.TrimSpace(req.Title)
	if code == "" || title == "" {
		return nil, validationError("Mã và tên hợp đồng là bắt buộc")
	}
	if req.OriginalAmount < 0 || req.AdvancePercent < 0 || req.AdvancePercent > 100 || req.RetentionPercent < 0 || req.RetentionPercent > 100 || req.WarrantyDays < 0 {
		return nil, validationError("Giá trị, tạm ứng, giữ lại hoặc bảo hành không hợp lệ")
	}
	currency := strings.ToUpper(strings.TrimSpace(req.Currency))
	if currency == "" {
		currency = "VND"
	}
	if len(currency) != 3 {
		return nil, validationError("Mã tiền tệ phải gồm 3 ký tự")
	}
	signedDate, effectiveDate, expiryDate, err := normalizeContractDates(req.SignedDate, req.EffectiveDate, req.ExpiryDate)
	if err != nil {
		return nil, err
	}
	body := Record{
		"project_id": projectID, "contract_code": code, "title": title, "contract_type": contractType, "direction": direction,
		"counterparty_partner_id": partnerID, "counterparty_name": partnerName, "currency": currency,
		"original_amount": req.OriginalAmount, "advance_percent": req.AdvancePercent, "retention_percent": req.RetentionPercent,
		"warranty_days": req.WarrantyDays, "signed_date": signedDate, "effective_date": effectiveDate, "expiry_date": expiryDate,
	}
	return s.repo.SaveContract(ctx, contractID, body, req.WorkItemContext, actorID, actorName)
}

func normalizeContractDates(signedDate, effectiveDate, expiryDate string) (string, string, string, error) {
	values := []string{strings.TrimSpace(signedDate), strings.TrimSpace(effectiveDate), strings.TrimSpace(expiryDate)}
	for _, value := range values {
		if value != "" {
			if _, err := time.Parse("2006-01-02", value); err != nil {
				return "", "", "", validationError("Ngày hợp đồng không hợp lệ")
			}
		}
	}
	if values[1] != "" && values[2] != "" && values[2] < values[1] {
		return "", "", "", validationError("Ngày hết hạn phải sau ngày hiệu lực")
	}
	return values[0], values[1], values[2], nil
}

func (s *Service) CreateContractVersion(ctx context.Context, contractID string, req ContractVersionRequest, actorID, actorName string) (Record, error) {
	if req.Amount < 0 || strings.TrimSpace(req.ScopeSummary) == "" {
		return nil, validationError("Giá trị và phạm vi phiên bản hợp đồng không hợp lệ")
	}
	startDate, endDate, _, err := normalizeContractDates(req.PlannedStartDate, req.PlannedEndDate, "")
	if err != nil {
		return nil, err
	}
	if startDate != "" && endDate != "" && endDate < startDate {
		return nil, validationError("Ngày kết thúc phiên bản phải sau ngày bắt đầu")
	}
	body := Record{
		"version_label": strings.TrimSpace(req.VersionLabel), "amount": req.Amount, "scope_summary": strings.TrimSpace(req.ScopeSummary),
		"commercial_terms_json": req.CommercialTerms, "payment_terms_json": req.PaymentTerms,
		"planned_start_date": startDate, "planned_end_date": endDate, "document_id": cleanID(req.DocumentID), "comment": strings.TrimSpace(req.Comment),
	}
	return s.repo.CreateContractVersion(ctx, cleanID(contractID), body, req.WorkItemContext, actorID, actorName)
}

func (s *Service) ContractVersionAction(ctx context.Context, versionID, action, comment string, mutationContext WorkItemMutationContext, actorID, actorName string) (Record, error) {
	comment = strings.TrimSpace(comment)
	if statusIn(action, "approve", "sign") && comment == "" {
		return nil, validationError("Duyệt/ký phiên bản hợp đồng cần có nhận xét")
	}
	return s.repo.ContractVersionAction(ctx, cleanID(versionID), action, comment, mutationContext, actorID, actorName)
}

func (s *Service) SignContractVersion(ctx context.Context, versionID string, req ContractVersionSignRequest, actorID, actorName string) (Record, error) {
	comment := strings.TrimSpace(req.Comment)
	if comment == "" {
		return nil, validationError("Ký phiên bản hợp đồng cần có nhận xét")
	}
	signatureID := cleanID(req.WorkflowFormSignatureID)
	if signatureID == "" {
		return nil, validationError("Chữ ký trên App là bắt buộc")
	}
	return s.repo.SignContractVersionWithWorkflowSignature(ctx, cleanID(versionID), signatureID, comment, req.WorkItemContext, actorID, actorName)
}

func (s *Service) CreateContractDispatch(ctx context.Context, contractID string, req ContractDispatchRequest, actorID, actorName string) (Record, error) {
	req.RecipientName = strings.TrimSpace(req.RecipientName)
	req.DeliveryMethod = strings.TrimSpace(req.DeliveryMethod)
	if req.DeliveryMethod == "" {
		req.DeliveryMethod = "courier"
	}
	if req.RecipientName == "" || !statusIn(req.DeliveryMethod, "courier", "direct", "postal", "other") {
		return nil, validationError("Người nhận hoặc phương thức gửi hồ sơ không hợp lệ")
	}
	return s.repo.CreateContractDispatch(ctx, cleanID(contractID), Record{
		"contract_version_id": cleanID(req.ContractVersionID),
		"recipient_name":      req.RecipientName,
		"delivery_method":     req.DeliveryMethod,
		"tracking_reference":  strings.TrimSpace(req.TrackingReference),
		"sent_document_id":    cleanID(req.SentDocumentID),
		"note":                strings.TrimSpace(req.Note),
	}, req.WorkItemContext, actorID, actorName)
}

func (s *Service) ReceiveContractDispatch(ctx context.Context, dispatchID string, req ContractDispatchReceiveRequest, actorID, actorName string) (Record, error) {
	scanID := cleanID(req.ReturnedScanDocumentID)
	if scanID == "" {
		return nil, validationError("Bản scan hợp đồng đã ký nhận là bắt buộc")
	}
	return s.repo.ReceiveContractDispatch(ctx, cleanID(dispatchID), scanID, strings.TrimSpace(req.Note), req.WorkItemContext, actorID, actorName)
}

func (s *Service) ActivateContract(ctx context.Context, contractID, comment string, mutationContext WorkItemMutationContext, actorID, actorName string) (Record, error) {
	return s.repo.ActivateContract(ctx, cleanID(contractID), strings.TrimSpace(comment), mutationContext, actorID, actorName)
}

func (s *Service) CreateContractMilestone(ctx context.Context, contractID string, req ContractMilestoneRequest, actorID, actorName string) (Record, error) {
	if strings.TrimSpace(req.MilestoneCode) == "" || strings.TrimSpace(req.MilestoneName) == "" || req.SequenceNo <= 0 {
		return nil, validationError("Mã, tên và thứ tự milestone là bắt buộc")
	}
	if !statusIn(req.MilestoneType, "advance", "progress", "final", "retention_release", "other") || !statusIn(req.AmountBasis, "fixed", "percentage", "quantity_certificate") {
		return nil, validationError("Loại milestone hoặc cơ sở giá trị không hợp lệ")
	}
	if req.GrossAmount < 0 || req.Percentage < 0 || req.Percentage > 100 || req.RetentionPercent < 0 || req.RetentionPercent > 100 || req.DueDays < 0 {
		return nil, validationError("Giá trị milestone không hợp lệ")
	}
	plannedDate := strings.TrimSpace(req.PlannedDate)
	if plannedDate != "" {
		if _, err := time.Parse("2006-01-02", plannedDate); err != nil {
			return nil, validationError("Ngày milestone không hợp lệ")
		}
	}
	body := Record{
		"milestone_code": strings.TrimSpace(req.MilestoneCode), "milestone_name": strings.TrimSpace(req.MilestoneName), "sequence_no": req.SequenceNo,
		"milestone_type": req.MilestoneType, "amount_basis": req.AmountBasis, "percentage": req.Percentage, "gross_amount": req.GrossAmount,
		"retention_percent": req.RetentionPercent, "planned_date": plannedDate, "due_days": req.DueDays,
		"acceptance_required": req.AcceptanceRequired, "acceptance_id": cleanID(req.AcceptanceID),
	}
	return s.repo.CreateContractMilestone(ctx, cleanID(contractID), body, actorID, actorName)
}

func (s *Service) ContractMilestoneAction(ctx context.Context, milestoneID, action, comment, actorID, actorName string) (Record, error) {
	return s.repo.ContractMilestoneAction(ctx, cleanID(milestoneID), action, strings.TrimSpace(comment), actorID, actorName)
}

func (s *Service) CreateContractVariation(ctx context.Context, contractID string, req ContractVariationRequest, actorID, actorName string) (Record, error) {
	body, err := normalizeContractVariationRequest(req)
	if err != nil {
		return nil, err
	}
	return s.repo.CreateContractVariation(ctx, cleanID(contractID), body, actorID, actorName)
}

func (s *Service) ProjectConstructionVariationContext(ctx context.Context, projectID string) (Record, error) {
	projectID = cleanID(projectID)
	if !validActivityUUID(projectID) {
		return nil, validationError("Mã công trình không hợp lệ")
	}
	return s.repo.ProjectConstructionVariationContext(ctx, projectID)
}

func (s *Service) ProposeConstructionVariation(ctx context.Context, projectID string, req ContractVariationRequest, actorID, actorName string) (Record, error) {
	projectID = cleanID(projectID)
	body, err := normalizeContractVariationRequest(req)
	if err != nil {
		return nil, err
	}
	workItemID := cleanID(req.SourceWorkItemID)
	if workItemID == "" {
		return nil, validationError("Đề xuất phát sinh cần hạng mục thi công nguồn", Record{"source_work_item_id": "required"})
	}
	if _, err := s.requireActiveConstructionWorkItem(ctx, projectID, workItemID); err != nil {
		return nil, err
	}
	actionID := cleanID(req.SourceConstructionActionID)
	if actionID != "" && !validActivityUUID(actionID) {
		return nil, validationError("Việc cần xử lý nguồn không hợp lệ", Record{"source_construction_action_id": actionID})
	}
	body["source_construction_action_id"] = actionID
	return s.repo.ProposeConstructionVariation(ctx, projectID, body, actorID, actorName)
}

func normalizeContractVariationRequest(req ContractVariationRequest) (Record, error) {
	if strings.TrimSpace(req.VariationCode) == "" || strings.TrimSpace(req.Title) == "" || strings.TrimSpace(req.Reason) == "" {
		return nil, validationError("Mã, tên và lý do phụ lục/variation là bắt buộc")
	}
	if !statusIn(req.VariationType, "scope", "price", "time", "mixed") {
		return nil, validationError("Loại variation không hợp lệ")
	}
	priority := strings.TrimSpace(req.Priority)
	if priority == "" {
		priority = "normal"
	}
	if !statusIn(priority, "normal", "urgent") {
		return nil, validationError("Mức độ phát sinh phải là Gấp hoặc Không gấp")
	}
	body := Record{"variation_code": strings.TrimSpace(req.VariationCode), "title": strings.TrimSpace(req.Title), "variation_type": req.VariationType,
		"reason": strings.TrimSpace(req.Reason), "amount_delta": req.AmountDelta, "days_delta": req.DaysDelta,
		"scope_delta": strings.TrimSpace(req.ScopeDelta), "document_id": cleanID(req.DocumentID), "comment": strings.TrimSpace(req.Comment),
		"priority": priority, "source_work_item_id": cleanID(req.SourceWorkItemID)}
	return body, nil
}

func (s *Service) ContractVariationAction(ctx context.Context, variationID, action, comment, actorID, actorName string) (Record, error) {
	comment = strings.TrimSpace(comment)
	if statusIn(action, "qs-review", "purchase", "approve", "reject") && comment == "" {
		return nil, validationError("Rà soát, mua hàng, duyệt hoặc từ chối phát sinh cần có nhận xét")
	}
	return s.repo.ContractVariationAction(ctx, cleanID(variationID), action, comment, actorID, actorName)
}

func (s *Service) GenerateProcurementFromContractVariation(ctx context.Context, variationID, actorID, actorName string) (Record, error) {
	return s.repo.GenerateProcurementFromContractVariation(ctx, cleanID(variationID), actorID, actorName)
}
