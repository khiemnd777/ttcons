package core

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

func (s *Service) SubcontractorReconciliation(ctx context.Context, workItemID, actorID string, revision ...int) (Record, error) {
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
	form, err := s.repo.GetWorkflowFormForWorkItem(ctx, workItemID)
	form = subcontractorReconciliationSourceForm(form)
	if errors.Is(err, sql.ErrNoRows) {
		return Record{"available": false, "reason": "Đầu việc chưa có biểu mẫu quy trình"}, nil
	}
	if err != nil {
		return nil, err
	}
	if !isSubcontractorChecklistForm(form) {
		return Record{"available": false, "reason": "Biểu mẫu quy trình không phải checklist thầu phụ", "form": form}, nil
	}
	instance, _ := form["instance"].(Record)
	if fmt.Sprint(instance["status"]) != "submitted" && !isClientSubcontractorChecklistForm(form) {
		return Record{"available": false, "reason": "Gửi checklist thầu phụ trước khi đối soát", "form": form}, nil
	}
	projectID := cleanID(item["project_id"])
	allowed, err := s.repo.CanAccessProjectPermission(ctx, actorID, projectID, []string{"boq.view", "boq.update", "operation.report"})
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền xem đối soát thầu phụ", Record{"permissions": []string{"boq.view", "operation.report"}})
	}
	if len(revision) > 0 {
		return s.subcontractorReconciliationHistoryResponse(ctx, item, form, revision[0])
	}
	result, err := s.subcontractorReconciliationResponse(ctx, item, form)
	if err == nil && isClientSubcontractorChecklistForm(form) {
		canAct, actionErr := s.repo.CanActOnWorkItem(ctx, workItemID, actorID)
		if actionErr != nil {
			return nil, actionErr
		}
		canReconcile, permissionErr := s.repo.CanAccessProjectPermission(ctx, actorID, projectID, []string{"boq.update", "operation.report"})
		if permissionErr != nil {
			return nil, permissionErr
		}
		canWrite := canAct && canReconcile && s.requireActivePipelineWorkItem(ctx, item) == nil && s.requireWorkItemPermission(ctx, item, actorID) == nil && clientReconciliationWorkItemWritable(item) && fmt.Sprint(instance["status"]) == "submitted"
		ledger, _ := result["reconciliation"].(Record)
		canWrite = canWrite && cleanOptionalText(ledger["status"]) != "submitted"
		result["capabilities"] = Record{"can_write": canWrite}
		result["readonly"] = !canWrite
	}
	return result, err
}

func (s *Service) SaveSubcontractorReconciliation(ctx context.Context, workItemID string, req SubcontractorReconciliationRequest, actorID, actorName string, precondition ...string) (Record, error) {
	return s.saveSubcontractorReconciliation(ctx, workItemID, req, false, actorID, actorName, precondition...)
}

func (s *Service) SubmitSubcontractorReconciliation(ctx context.Context, workItemID string, req SubcontractorReconciliationRequest, actorID, actorName string, precondition ...string) (Record, error) {
	return s.saveSubcontractorReconciliation(ctx, workItemID, req, true, actorID, actorName, precondition...)
}

func (s *Service) saveSubcontractorReconciliation(ctx context.Context, workItemID string, req SubcontractorReconciliationRequest, submit bool, actorID, actorName string, precondition ...string) (Record, error) {
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
	canReconcile, err := s.repo.CanAccessProjectPermission(ctx, actorID, projectID, []string{"boq.update", "operation.report"})
	if err != nil {
		return nil, err
	}
	if !canReconcile {
		return nil, forbiddenError("Bạn không có quyền đối soát checklist thầu phụ", Record{"permissions": []string{"boq.update", "operation.report"}})
	}
	form, err := s.repo.GetWorkflowFormForWorkItem(ctx, workItemID)
	form = subcontractorReconciliationSourceForm(form)
	if err != nil {
		return nil, err
	}
	if !isSubcontractorChecklistForm(form) {
		return nil, validationError("Biểu mẫu quy trình không phải checklist thầu phụ")
	}
	if isClientSubcontractorChecklistForm(form) && !clientReconciliationWorkItemWritable(item) {
		return nil, conflictError("Đầu việc đã đóng, không thể sửa đối soát")
	}
	instance, _ := form["instance"].(Record)
	if fmt.Sprint(instance["status"]) != "submitted" {
		return nil, validationError("Gửi checklist thầu phụ trước khi đối soát", Record{"status": instance["status"]})
	}
	boqItems, err := s.repo.ListProjectBOQ(ctx, projectID)
	if err != nil {
		return nil, err
	}
	normalizedItems, summary, err := s.normalizeSubcontractorReconciliationRequest(ctx, item, form, boqItems, req)
	if err != nil {
		return nil, err
	}
	if err := s.repo.SaveSubcontractorReconciliation(ctx, item, form, strings.TrimSpace(req.Comment), normalizedItems, summary, submit, actorID, actorName, precondition...); err != nil {
		return nil, err
	}
	result, err := s.subcontractorReconciliationResponse(ctx, item, form)
	transition, _ := item["work_item_transition"].(Record)
	if isClientSubcontractorChecklistForm(form) {
		result, err = s.SubcontractorReconciliation(ctx, workItemID, actorID)
	}
	return attachWorkItemTransition(result, transition), err
}

func (s *Service) subcontractorReconciliationResponse(ctx context.Context, item Record, form Record) (Record, error) {
	projectID := cleanID(item["project_id"])
	boqItems, err := s.repo.ListProjectBOQ(ctx, projectID)
	if err != nil {
		return nil, err
	}
	instance, _ := form["instance"].(Record)
	var reconciliation Record
	if isClientSubcontractorChecklistForm(form) {
		reconciliation, err = s.repo.getSubcontractorReconciliationForSource(ctx, projectID, cleanID(item["id"]), cleanID(instance["id"]), int(number(instance["current_revision"])))
	} else {
		reconciliation, err = s.repo.GetSubcontractorReconciliation(ctx, cleanID(item["id"]))
	}
	if errors.Is(err, sql.ErrNoRows) {
		if isClientSubcontractorChecklistForm(form) && fmt.Sprint(instance["status"]) != "submitted" {
			return s.clientSubcontractorResponseMetadata(ctx, item, form, nil, Record{"available": false, "reason": "Gửi checklist thầu phụ trước khi đối soát", "form": form})
		}
		sourceItems, err := subcontractorChecklistSourceItems(form)
		if err != nil {
			return nil, err
		}
		items := make([]Record, 0, len(sourceItems))
		for index, source := range sourceItems {
			items = append(items, defaultSubcontractorReconciliationItem(index, source))
		}
		instance, _ := form["instance"].(Record)
		answers, _ := form["answers"].(Record)
		reconciliation = Record{
			"id":                 "",
			"project_id":         projectID,
			"work_item_id":       item["id"],
			"form_instance_id":   instance["id"],
			"form_revision_no":   instance["current_revision"],
			"status":             "draft",
			"subcontractor_name": answers["subcontractor_name"],
			"work_package":       answers["work_package"],
			"construction_area":  answers["construction_area"],
			"summary_json":       subcontractorReconciliationSummary(items),
			"comment":            "",
			"items":              items,
		}
	} else if err != nil {
		return nil, err
	}
	result := Record{
		"available":      true,
		"work_item_id":   item["id"],
		"project_id":     projectID,
		"form":           form,
		"reconciliation": reconciliation,
		"boq_items":      boqItems,
	}
	if isClientSubcontractorChecklistForm(form) {
		return s.clientSubcontractorResponseMetadata(ctx, item, form, reconciliation, result)
	}
	return result, nil
}

func (s *Service) normalizeSubcontractorReconciliationRequest(ctx context.Context, item Record, form Record, boqItems []Record, req SubcontractorReconciliationRequest) ([]Record, Record, error) {
	if len([]rune(strings.TrimSpace(req.Comment))) > 2000 {
		return nil, nil, validationError("Ghi chú đối soát không được vượt quá 2000 ký tự")
	}
	sourceItems, err := subcontractorChecklistSourceItems(form)
	if err != nil {
		return nil, nil, err
	}
	requestByIndex := map[int]SubcontractorReconciliationItemRequest{}
	for _, itemReq := range req.Items {
		if itemReq.ItemIndex < 0 || itemReq.ItemIndex >= len(sourceItems) {
			return nil, nil, validationError("Dòng đối soát không thuộc checklist", Record{"item_index": itemReq.ItemIndex})
		}
		if _, exists := requestByIndex[itemReq.ItemIndex]; exists {
			return nil, nil, validationError("Dòng đối soát bị trùng", Record{"item_index": itemReq.ItemIndex})
		}
		requestByIndex[itemReq.ItemIndex] = itemReq
	}
	boqByID := map[string]Record{}
	for _, row := range boqItems {
		boqByID[cleanID(row["id"])] = row
	}
	projectID := cleanID(item["project_id"])
	workItemID := cleanID(item["id"])
	normalizedItems := make([]Record, 0, len(sourceItems))
	for index, source := range sourceItems {
		itemReq, hasRequest := requestByIndex[index]
		if !hasRequest {
			itemReq = SubcontractorReconciliationItemRequest{ItemIndex: index}
		}
		normalized, err := s.normalizeSubcontractorReconciliationItem(ctx, projectID, workItemID, index, source, itemReq, boqByID)
		if err != nil {
			return nil, nil, err
		}
		normalizedItems = append(normalizedItems, normalized)
	}
	return normalizedItems, subcontractorReconciliationSummary(normalizedItems), nil
}

func (s *Service) normalizeSubcontractorReconciliationItem(ctx context.Context, projectID, workItemID string, itemIndex int, source Record, req SubcontractorReconciliationItemRequest, boqByID map[string]Record) (Record, error) {
	title := strings.TrimSpace(fmt.Sprint(source["title"]))
	if title == "" || title == "<nil>" {
		return nil, validationError("Hạng mục checklist thiếu tên", Record{"item_index": itemIndex})
	}
	claimed, err := optionalWorkflowNumber(source["claimed_quantity"])
	if err != nil {
		return nil, validationError("KL claim không hợp lệ", Record{"item_index": itemIndex})
	}
	confirmed, err := optionalWorkflowNumber(source["confirmed_quantity"])
	if err != nil {
		return nil, validationError("KL xác nhận không hợp lệ", Record{"item_index": itemIndex})
	}
	if req.ConfirmedQuantity != nil {
		if *req.ConfirmedQuantity < 0 {
			return nil, validationError("KL xác nhận không được âm", Record{"item_index": itemIndex})
		}
		value := *req.ConfirmedQuantity
		confirmed = &value
	}
	decision := strings.TrimSpace(req.Decision)
	if decision == "" {
		decision = defaultSubcontractorDecision(source)
	}
	if !statusIn(decision, "accepted", "adjust_quantity", "rework_required", "waiting_evidence", "rejected") {
		return nil, validationError("Quyết định đối soát không hợp lệ", Record{"item_index": itemIndex, "decision": decision})
	}
	if decision == "accepted" && confirmed == nil && claimed != nil {
		value := *claimed
		confirmed = &value
	}
	if decision == "adjust_quantity" && confirmed == nil {
		return nil, validationError("Điều chỉnh KL cần KL xác nhận", Record{"item_index": itemIndex})
	}
	boqItemID := cleanID(req.BOQItemID)
	var boqItem Record
	if boqItemID != "" {
		var ok bool
		boqItem, ok = boqByID[boqItemID]
		if !ok {
			return nil, validationError("Dòng BOQ không thuộc công trình", Record{"item_index": itemIndex, "boq_item_id": boqItemID})
		}
	}
	evidenceID := cleanID(source["evidence_id"])
	if evidenceID != "" {
		evidence, err := s.repo.GetEvidenceDetail(ctx, evidenceID)
		if err != nil {
			return nil, validationError("Minh chứng đối soát không hợp lệ", Record{"item_index": itemIndex, "evidence_id": evidenceID})
		}
		if cleanID(evidence["project_id"]) != projectID || cleanID(evidence["work_item_id"]) != workItemID || fmt.Sprint(evidence["status"]) == "rejected" {
			return nil, validationError("Minh chứng đối soát không thuộc đầu việc", Record{"item_index": itemIndex, "evidence_id": evidenceID})
		}
	}
	comment := strings.TrimSpace(req.Comment)
	if len([]rune(comment)) > 1000 {
		return nil, validationError("Ghi chú dòng đối soát không được vượt quá 1000 ký tự", Record{"item_index": itemIndex})
	}
	var variance any
	if claimed != nil && confirmed != nil {
		variance = *confirmed - *claimed
	}
	row := Record{
		"item_index":         itemIndex,
		"checklist_title":    title,
		"boq_item_id":        boqItemID,
		"boq_item_ref":       cleanOptionalText(source["boq_item_ref"]),
		"claimed_quantity":   numberOrNil(claimed),
		"confirmed_quantity": numberOrNil(confirmed),
		"variance_quantity":  variance,
		"unit":               cleanOptionalText(source["unit"]),
		"status":             cleanOptionalText(source["status"]),
		"evidence_id":        evidenceID,
		"owner_response":     cleanOptionalText(source["owner_response"]),
		"decision":           decision,
		"due_date":           cleanOptionalText(source["due_date"]),
		"comment":            comment,
		"payload_json":       source,
	}
	if boqItem != nil {
		row["boq_work_item_name"] = boqItem["work_item_name"]
		row["boq_quantity"] = boqItem["quantity"]
		row["boq_unit"] = boqItem["unit"]
	}
	return row, nil
}

func isSubcontractorChecklistForm(form Record) bool {
	template, _ := form["template"].(Record)
	return fmt.Sprint(template["code"]) == "builtin_subcontractor_checklist" || isClientSubcontractorChecklistForm(form)
}

func subcontractorChecklistSourceItems(form Record) ([]Record, error) {
	answers, _ := form["answers"].(Record)
	rawItems, ok := answers["checklist_items"]
	if !ok || workflowFormValueEmpty(Record{"field_type": "structured_list"}, rawItems) {
		return nil, validationError("Checklist thầu phụ chưa có hạng mục")
	}
	return workflowRecordList(rawItems)
}

func defaultSubcontractorReconciliationItem(index int, source Record) Record {
	decision := defaultSubcontractorDecision(source)
	claimed, _ := optionalWorkflowNumber(source["claimed_quantity"])
	confirmed, _ := optionalWorkflowNumber(source["confirmed_quantity"])
	if decision == "accepted" && confirmed == nil && claimed != nil {
		value := *claimed
		confirmed = &value
	}
	var variance any
	if claimed != nil && confirmed != nil {
		variance = *confirmed - *claimed
	}
	return Record{
		"item_index":         index,
		"checklist_title":    cleanOptionalText(source["title"]),
		"boq_item_id":        "",
		"boq_item_ref":       cleanOptionalText(source["boq_item_ref"]),
		"claimed_quantity":   numberOrNil(claimed),
		"confirmed_quantity": numberOrNil(confirmed),
		"variance_quantity":  variance,
		"unit":               cleanOptionalText(source["unit"]),
		"status":             cleanOptionalText(source["status"]),
		"evidence_id":        cleanID(source["evidence_id"]),
		"owner_response":     cleanOptionalText(source["owner_response"]),
		"decision":           decision,
		"due_date":           cleanOptionalText(source["due_date"]),
		"comment":            cleanOptionalText(source["comment"]),
		"payload_json":       source,
	}
}

func defaultSubcontractorDecision(source Record) string {
	ownerResponse := cleanOptionalText(source["owner_response"])
	if statusIn(ownerResponse, "accepted", "adjust_quantity", "rework_required", "waiting_evidence") {
		return ownerResponse
	}
	status := cleanOptionalText(source["status"])
	if statusIn(status, "done", "not_applicable") {
		return "accepted"
	}
	return "waiting_evidence"
}

func subcontractorReconciliationSummary(items []Record) Record {
	summary := Record{
		"total":            len(items),
		"accepted":         0,
		"adjust_quantity":  0,
		"rework_required":  0,
		"waiting_evidence": 0,
		"rejected":         0,
	}
	for _, item := range items {
		decision := cleanOptionalText(item["decision"])
		if _, ok := summary[decision]; ok {
			summary[decision] = int(number(summary[decision])) + 1
		}
	}
	return summary
}

func optionalWorkflowNumber(value any) (*float64, error) {
	if workflowFormValueEmpty(Record{"field_type": "number"}, value) {
		return nil, nil
	}
	numeric, err := workflowNumberValue(value)
	if err != nil {
		return nil, err
	}
	if numeric < 0 {
		return nil, errors.New("Số lượng không được âm")
	}
	return &numeric, nil
}

func numberOrNil(value *float64) any {
	if value == nil {
		return nil
	}
	return *value
}

func (s *Service) OwnerClientReconciliation(ctx context.Context, workItemID, actorID string) (Record, error) {
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
	form, err := s.repo.GetWorkflowFormForWorkItem(ctx, workItemID)
	if errors.Is(err, sql.ErrNoRows) {
		return Record{"available": false, "reason": "Đầu việc chưa có biểu mẫu quy trình"}, nil
	}
	if err != nil {
		return nil, err
	}
	if !isOwnerClientChecklistForm(form) {
		return Record{"available": false, "reason": "Biểu mẫu quy trình không phải checklist chủ đầu tư", "form": form}, nil
	}
	instance, _ := form["instance"].(Record)
	if fmt.Sprint(instance["status"]) != "submitted" {
		return Record{"available": false, "reason": "Gửi checklist chủ đầu tư trước khi đối soát", "form": form}, nil
	}
	projectID := cleanID(item["project_id"])
	allowed, err := s.repo.CanAccessProjectPermission(ctx, actorID, projectID, []string{"acceptance.view", "acceptance.approve", "acceptance.reject", "operation.report"})
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền xem đối soát chủ đầu tư", Record{"permissions": []string{"acceptance.view", "operation.report"}})
	}
	return s.ownerClientReconciliationResponse(ctx, item, form)
}

func (s *Service) SaveOwnerClientReconciliation(ctx context.Context, workItemID string, req OwnerClientReconciliationRequest, actorID, actorName string) (Record, error) {
	return s.saveOwnerClientReconciliation(ctx, workItemID, req, false, actorID, actorName)
}

func (s *Service) SubmitOwnerClientReconciliation(ctx context.Context, workItemID string, req OwnerClientReconciliationRequest, actorID, actorName string) (Record, error) {
	return s.saveOwnerClientReconciliation(ctx, workItemID, req, true, actorID, actorName)
}

func (s *Service) saveOwnerClientReconciliation(ctx context.Context, workItemID string, req OwnerClientReconciliationRequest, submit bool, actorID, actorName string) (Record, error) {
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
	canReconcile, err := s.repo.CanAccessProjectPermission(ctx, actorID, projectID, []string{"acceptance.approve", "acceptance.reject", "operation.report"})
	if err != nil {
		return nil, err
	}
	if !canReconcile {
		return nil, forbiddenError("Bạn không có quyền đối soát checklist chủ đầu tư", Record{"permissions": []string{"acceptance.approve", "acceptance.reject", "operation.report"}})
	}
	form, err := s.repo.GetWorkflowFormForWorkItem(ctx, workItemID)
	if err != nil {
		return nil, err
	}
	if !isOwnerClientChecklistForm(form) {
		return nil, validationError("Biểu mẫu quy trình không phải checklist chủ đầu tư")
	}
	instance, _ := form["instance"].(Record)
	if fmt.Sprint(instance["status"]) != "submitted" {
		return nil, validationError("Gửi checklist chủ đầu tư trước khi đối soát", Record{"status": instance["status"]})
	}
	acceptances, err := s.repo.ListProjectAcceptanceRecords(ctx, projectID)
	if err != nil {
		return nil, err
	}
	normalizedItems, summary, err := s.normalizeOwnerClientReconciliationRequest(ctx, item, form, acceptances, req, submit)
	if err != nil {
		return nil, err
	}
	if err := s.repo.SaveOwnerClientReconciliation(ctx, item, form, strings.TrimSpace(req.Comment), normalizedItems, summary, submit, actorID, actorName); err != nil {
		return nil, err
	}
	result, err := s.ownerClientReconciliationResponse(ctx, item, form)
	transition, _ := item["work_item_transition"].(Record)
	return attachWorkItemTransition(result, transition), err
}

func (s *Service) ownerClientReconciliationResponse(ctx context.Context, item Record, form Record) (Record, error) {
	projectID := cleanID(item["project_id"])
	acceptances, err := s.repo.ListProjectAcceptanceRecords(ctx, projectID)
	if err != nil {
		return nil, err
	}
	reconciliation, err := s.repo.GetOwnerClientReconciliation(ctx, cleanID(item["id"]))
	if errors.Is(err, sql.ErrNoRows) {
		sourceItems, err := ownerClientChecklistSourceItems(form)
		if err != nil {
			return nil, err
		}
		items := make([]Record, 0, len(sourceItems))
		for index, source := range sourceItems {
			items = append(items, defaultOwnerClientReconciliationItem(index, source))
		}
		instance, _ := form["instance"].(Record)
		answers, _ := form["answers"].(Record)
		reconciliation = Record{
			"id":                     "",
			"project_id":             projectID,
			"work_item_id":           item["id"],
			"form_instance_id":       instance["id"],
			"form_revision_no":       instance["current_revision"],
			"status":                 "draft",
			"client_representative":  answers["client_representative"],
			"company_representative": answers["company_representative"],
			"handover_scope":         answers["handover_scope"],
			"scope_detail":           answers["scope_detail"],
			"summary_json":           ownerClientReconciliationSummary(items),
			"comment":                "",
			"items":                  items,
		}
	} else if err != nil {
		return nil, err
	}
	return Record{
		"available":      true,
		"work_item_id":   item["id"],
		"project_id":     projectID,
		"form":           form,
		"reconciliation": reconciliation,
		"acceptances":    acceptances,
	}, nil
}

func (s *Service) normalizeOwnerClientReconciliationRequest(ctx context.Context, item Record, form Record, acceptances []Record, req OwnerClientReconciliationRequest, submit bool) ([]Record, Record, error) {
	if len([]rune(strings.TrimSpace(req.Comment))) > 2000 {
		return nil, nil, validationError("Ghi chú đối soát không được vượt quá 2000 ký tự")
	}
	sourceItems, err := ownerClientChecklistSourceItems(form)
	if err != nil {
		return nil, nil, err
	}
	requestByIndex := map[int]OwnerClientReconciliationItemRequest{}
	for _, itemReq := range req.Items {
		if itemReq.ItemIndex < 0 || itemReq.ItemIndex >= len(sourceItems) {
			return nil, nil, validationError("Dòng đối soát không thuộc checklist", Record{"item_index": itemReq.ItemIndex})
		}
		if _, exists := requestByIndex[itemReq.ItemIndex]; exists {
			return nil, nil, validationError("Dòng đối soát bị trùng", Record{"item_index": itemReq.ItemIndex})
		}
		requestByIndex[itemReq.ItemIndex] = itemReq
	}
	acceptanceByID := map[string]Record{}
	for _, row := range acceptances {
		acceptanceByID[cleanID(row["id"])] = row
	}
	projectID := cleanID(item["project_id"])
	workItemID := cleanID(item["id"])
	normalizedItems := make([]Record, 0, len(sourceItems))
	for index, source := range sourceItems {
		itemReq, hasRequest := requestByIndex[index]
		if !hasRequest {
			itemReq = OwnerClientReconciliationItemRequest{ItemIndex: index}
		}
		normalized, err := s.normalizeOwnerClientReconciliationItem(ctx, projectID, workItemID, index, source, itemReq, acceptanceByID, submit)
		if err != nil {
			return nil, nil, err
		}
		normalizedItems = append(normalizedItems, normalized)
	}
	return normalizedItems, ownerClientReconciliationSummary(normalizedItems), nil
}

func (s *Service) normalizeOwnerClientReconciliationItem(ctx context.Context, projectID, workItemID string, itemIndex int, source Record, req OwnerClientReconciliationItemRequest, acceptanceByID map[string]Record, submit bool) (Record, error) {
	title := strings.TrimSpace(fmt.Sprint(source["title"]))
	if title == "" || title == "<nil>" {
		return nil, validationError("Hạng mục checklist thiếu tên", Record{"item_index": itemIndex})
	}
	clientDecision := cleanOptionalText(source["client_decision"])
	if clientDecision != "" && !statusIn(clientDecision, "pass", "conditional_pass", "fail", "not_applicable") {
		return nil, validationError("Quyết định chủ đầu tư không hợp lệ", Record{"item_index": itemIndex, "client_decision": clientDecision})
	}
	severity := cleanOptionalText(source["severity"])
	if severity != "" && !statusIn(severity, "low", "medium", "high", "critical") {
		return nil, validationError("Mức độ checklist chủ đầu tư không hợp lệ", Record{"item_index": itemIndex, "severity": severity})
	}
	decision := strings.TrimSpace(req.Decision)
	if decision == "" {
		decision = defaultOwnerClientDecision(source)
	}
	if !statusIn(decision, "accepted", "conditional_acceptance", "rework_required", "waiting_internal_acceptance", "rejected", "not_applicable") {
		return nil, validationError("Quyết định đối soát không hợp lệ", Record{"item_index": itemIndex, "decision": decision})
	}
	if clientDecision == "fail" && decision == "accepted" {
		return nil, validationError("Không thể chấp nhận khi chủ đầu tư đánh không đạt", Record{"item_index": itemIndex})
	}
	acceptanceID := cleanID(req.AcceptanceID)
	var acceptance Record
	if acceptanceID != "" {
		var ok bool
		acceptance, ok = acceptanceByID[acceptanceID]
		if !ok {
			return nil, validationError("Nghiệm thu không thuộc công trình", Record{"item_index": itemIndex, "acceptance_id": acceptanceID})
		}
	}
	if submit && statusIn(decision, "accepted", "conditional_acceptance") {
		if acceptance == nil {
			return nil, validationError("Cần gắn nghiệm thu nội bộ cho dòng đã chấp nhận", Record{"item_index": itemIndex})
		}
		if fmt.Sprint(acceptance["status"]) != "completed" {
			return nil, validationError("Nghiệm thu nội bộ phải ở trạng thái hoàn tất", Record{"item_index": itemIndex, "acceptance_id": acceptanceID})
		}
		result := cleanOptionalText(acceptance["result"])
		if decision == "accepted" && result != "pass" {
			return nil, validationError("Dòng đã chấp nhận cần nghiệm thu nội bộ đạt", Record{"item_index": itemIndex, "acceptance_id": acceptanceID, "result": result})
		}
		if decision == "conditional_acceptance" && !statusIn(result, "pass", "conditional_pass") {
			return nil, validationError("Dòng chấp nhận có điều kiện cần nghiệm thu nội bộ đạt hoặc đạt có điều kiện", Record{"item_index": itemIndex, "acceptance_id": acceptanceID, "result": result})
		}
	}
	evidenceID := cleanID(source["evidence_id"])
	if evidenceID != "" {
		evidence, err := s.repo.GetEvidenceDetail(ctx, evidenceID)
		if err != nil {
			return nil, validationError("Minh chứng đối soát không hợp lệ", Record{"item_index": itemIndex, "evidence_id": evidenceID})
		}
		if cleanID(evidence["project_id"]) != projectID || cleanID(evidence["work_item_id"]) != workItemID || fmt.Sprint(evidence["status"]) == "rejected" {
			return nil, validationError("Minh chứng đối soát không thuộc đầu việc", Record{"item_index": itemIndex, "evidence_id": evidenceID})
		}
	}
	comment := strings.TrimSpace(req.Comment)
	if len([]rune(comment)) > 1000 {
		return nil, validationError("Ghi chú dòng đối soát không được vượt quá 1000 ký tự", Record{"item_index": itemIndex})
	}
	row := Record{
		"item_index":      itemIndex,
		"checklist_title": title,
		"output_ref":      cleanOptionalText(source["output_ref"]),
		"status":          cleanOptionalText(source["status"]),
		"client_decision": clientDecision,
		"severity":        severity,
		"required_action": cleanOptionalText(source["required_action"]),
		"due_date":        cleanOptionalText(source["due_date"]),
		"evidence_id":     evidenceID,
		"acceptance_id":   acceptanceID,
		"decision":        decision,
		"comment":         comment,
		"payload_json":    source,
	}
	if acceptance != nil {
		row["acceptance_title"] = acceptance["title"]
		row["acceptance_type"] = acceptance["type"]
		row["acceptance_status"] = acceptance["status"]
		row["acceptance_result"] = acceptance["result"]
	}
	return row, nil
}

func isOwnerClientChecklistForm(form Record) bool {
	template, _ := form["template"].(Record)
	return fmt.Sprint(template["code"]) == "builtin_owner_client_checklist"
}

func ownerClientChecklistSourceItems(form Record) ([]Record, error) {
	answers, _ := form["answers"].(Record)
	rawItems, ok := answers["checklist_items"]
	if !ok || workflowFormValueEmpty(Record{"field_type": "structured_list"}, rawItems) {
		return nil, validationError("Checklist chủ đầu tư chưa có hạng mục")
	}
	return workflowRecordList(rawItems)
}

func defaultOwnerClientReconciliationItem(index int, source Record) Record {
	return Record{
		"item_index":      index,
		"checklist_title": cleanOptionalText(source["title"]),
		"output_ref":      cleanOptionalText(source["output_ref"]),
		"status":          cleanOptionalText(source["status"]),
		"client_decision": cleanOptionalText(source["client_decision"]),
		"severity":        cleanOptionalText(source["severity"]),
		"required_action": cleanOptionalText(source["required_action"]),
		"due_date":        cleanOptionalText(source["due_date"]),
		"evidence_id":     cleanID(source["evidence_id"]),
		"acceptance_id":   "",
		"decision":        defaultOwnerClientDecision(source),
		"comment":         cleanOptionalText(source["comment"]),
		"payload_json":    source,
	}
}

func defaultOwnerClientDecision(source Record) string {
	clientDecision := cleanOptionalText(source["client_decision"])
	if clientDecision == "not_applicable" || cleanOptionalText(source["status"]) == "not_applicable" {
		return "not_applicable"
	}
	switch clientDecision {
	case "pass":
		return "accepted"
	case "conditional_pass":
		return "conditional_acceptance"
	case "fail":
		return "rework_required"
	default:
		return "waiting_internal_acceptance"
	}
}

func ownerClientReconciliationSummary(items []Record) Record {
	summary := Record{
		"total":                       len(items),
		"accepted":                    0,
		"conditional_acceptance":      0,
		"rework_required":             0,
		"waiting_internal_acceptance": 0,
		"rejected":                    0,
		"not_applicable":              0,
	}
	for _, item := range items {
		decision := cleanOptionalText(item["decision"])
		if _, ok := summary[decision]; ok {
			summary[decision] = int(number(summary[decision])) + 1
		}
	}
	return summary
}
