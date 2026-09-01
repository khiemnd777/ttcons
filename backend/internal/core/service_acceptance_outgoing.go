package core

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

func (s *Service) ProjectAcceptances(ctx context.Context, projectID, actorID string) ([]Record, error) {
	projectID = cleanID(projectID)
	allowed, err := s.repo.CanAccessProjectPermission(ctx, actorID, projectID, []string{"acceptance.view", "operation.view"})
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền xem nghiệm thu", Record{"permissions": []string{"acceptance.view", "operation.view"}})
	}
	return s.repo.ListProjectAcceptanceRecords(ctx, projectID)
}

func (s *Service) WorkItemOutgoingAcceptance(ctx context.Context, workItemID, actorID string) (Record, error) {
	canView, err := s.repo.CanViewWorkItem(ctx, workItemID, actorID)
	if err != nil {
		return nil, err
	}
	if !canView {
		return nil, forbiddenError("Bạn không có quyền xem công việc liên quan này")
	}
	item, err := s.repo.GetWorkItemDetail(ctx, workItemID)
	if err != nil {
		return nil, err
	}
	projectID := cleanID(item["project_id"])
	allowed, err := s.repo.CanAccessProjectPermission(ctx, actorID, projectID, []string{"acceptance.view", "operation.view", "acceptance.create", "operation.report"})
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền xem nghiệm thu đầu ra", Record{"permissions": []string{"acceptance.view", "operation.view"}})
	}
	records, err := s.repo.ListAcceptanceRecordsForWorkItem(ctx, workItemID)
	if err != nil {
		return nil, err
	}
	form, err := s.repo.GetWorkflowFormForWorkItem(ctx, workItemID)
	if errors.Is(err, sql.ErrNoRows) {
		form = Record{"available": false}
	} else if err != nil {
		return nil, err
	} else {
		form["available"] = true
	}
	return Record{
		"available":    true,
		"work_item_id": item["id"],
		"project_id":   projectID,
		"records":      records,
		"form":         form,
	}, nil
}

func (s *Service) GetAcceptance(ctx context.Context, acceptanceID, actorID string) (Record, error) {
	record, err := s.repo.GetAcceptanceDetail(ctx, acceptanceID)
	if err != nil {
		return nil, err
	}
	projectID := cleanID(record["project_id"])
	allowed, err := s.repo.CanAccessProjectPermission(ctx, actorID, projectID, []string{"acceptance.view", "operation.view"})
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền xem nghiệm thu", Record{"permissions": []string{"acceptance.view", "operation.view"}})
	}
	workItemID := cleanID(record["work_item_id"])
	if workItemID != "" {
		canView, err := s.repo.CanViewWorkItem(ctx, workItemID, actorID)
		if err != nil {
			return nil, err
		}
		if !canView {
			return nil, forbiddenError("Bạn không có quyền xem công việc liên quan này")
		}
	}
	return record, nil
}

func (s *Service) CreateAcceptance(ctx context.Context, projectID string, req AcceptanceRequest, actorID, actorName string) (Record, error) {
	projectID = cleanID(projectID)
	item, err := s.requireAcceptanceWriteAccess(ctx, projectID, req.WorkItemID, actorID, []string{"acceptance.create", "operation.report"}, "Bạn không có quyền lưu nghiệm thu đầu ra")
	if err != nil {
		return nil, err
	}
	header, items, err := s.normalizeAcceptanceRequest(ctx, projectID, item, nil, req, false)
	if err != nil {
		return nil, err
	}
	return s.repo.SaveAcceptance(ctx, "", header, items, actorID, actorName)
}

func (s *Service) UpdateAcceptance(ctx context.Context, acceptanceID string, req AcceptanceRequest, actorID, actorName string) (Record, error) {
	existing, err := s.repo.GetAcceptanceDetail(ctx, acceptanceID)
	if err != nil {
		return nil, err
	}
	if fmt.Sprint(existing["status"]) != "draft" {
		return nil, validationError("Hồ sơ nghiệm thu đầu ra đã gửi/chốt, không thể sửa", Record{"acceptance_id": acceptanceID, "status": existing["status"]})
	}
	projectID := cleanID(existing["project_id"])
	workItemID := req.WorkItemID
	if cleanID(workItemID) == "" {
		workItemID = cleanID(existing["work_item_id"])
	}
	item, err := s.requireAcceptanceWriteAccess(ctx, projectID, workItemID, actorID, []string{"acceptance.create", "operation.report"}, "Bạn không có quyền lưu nghiệm thu đầu ra")
	if err != nil {
		return nil, err
	}
	header, items, err := s.normalizeAcceptanceRequest(ctx, projectID, item, existing, req, false)
	if err != nil {
		return nil, err
	}
	return s.repo.SaveAcceptance(ctx, cleanID(existing["id"]), header, items, actorID, actorName)
}

func (s *Service) SubmitAcceptance(ctx context.Context, acceptanceID string, actorID, actorName string) (Record, error) {
	existing, err := s.repo.GetAcceptanceDetail(ctx, acceptanceID)
	if err != nil {
		return nil, err
	}
	if fmt.Sprint(existing["status"]) != "draft" {
		return nil, validationError("Chỉ hồ sơ nghiệm thu nháp mới được gửi", Record{"acceptance_id": acceptanceID, "status": existing["status"]})
	}
	projectID := cleanID(existing["project_id"])
	item, err := s.requireAcceptanceWriteAccess(ctx, projectID, cleanID(existing["work_item_id"]), actorID, []string{"acceptance.create", "operation.report"}, "Bạn không có quyền gửi nghiệm thu đầu ra")
	if err != nil {
		return nil, err
	}
	req := acceptanceRequestFromRecord(existing)
	if _, _, err := s.normalizeAcceptanceRequest(ctx, projectID, item, existing, req, true); err != nil {
		return nil, err
	}
	return s.repo.SubmitAcceptance(ctx, cleanID(existing["id"]), actorID, actorName)
}

func (s *Service) AcceptanceAction(ctx context.Context, id, result string, req ActionRequest, actorID, actorName string) (Record, error) {
	existing, err := s.repo.GetAcceptanceDetail(ctx, id)
	if err != nil {
		return nil, err
	}
	if workItemID := cleanID(existing["work_item_id"]); workItemID != "" {
		item, err := s.repo.GetWorkItemDetail(ctx, workItemID)
		if err != nil {
			return nil, err
		}
		if err := s.requireActivePipelineWorkItem(ctx, item); err != nil {
			return nil, err
		}
		if fmt.Sprint(existing["status"]) != "submitted" {
			return nil, validationError("Nghiệm thu đầu ra cần gửi trước khi chốt kết quả", Record{"acceptance_id": id, "status": existing["status"]})
		}
		if err := validateAcceptanceActionResult(result, recordValue(existing["summary_json"])); err != nil {
			return nil, err
		}
	}
	return s.repo.AcceptanceAction(ctx, id, result, req, actorID, actorName)
}

func (s *Service) CreateReworkFromAcceptance(ctx context.Context, acceptanceID, actorID, actorName string) (Record, error) {
	existing, err := s.repo.GetAcceptanceDetail(ctx, acceptanceID)
	if err != nil {
		return nil, err
	}
	if workItemID := cleanID(existing["work_item_id"]); workItemID != "" {
		item, err := s.repo.GetWorkItemDetail(ctx, workItemID)
		if err != nil {
			return nil, err
		}
		if err := s.requireActivePipelineWorkItem(ctx, item); err != nil {
			return nil, err
		}
	}
	return s.repo.CreateReworkFromAcceptance(ctx, acceptanceID, actorID, actorName)
}

func (s *Service) requireAcceptanceWriteAccess(ctx context.Context, projectID, workItemID, actorID string, permissions []string, message string) (Record, error) {
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
		return nil, forbiddenError("Bạn không có quyền thao tác công việc liên quan này")
	}
	item, err := s.repo.GetWorkItemDetail(ctx, workItemID)
	if err != nil {
		return nil, err
	}
	if err := s.requireActivePipelineWorkItem(ctx, item); err != nil {
		return nil, err
	}
	if cleanID(item["project_id"]) != projectID {
		return nil, validationError("Công việc liên quan không thuộc công trình nghiệm thu", Record{"work_item_id": workItemID, "project_id": projectID})
	}
	if err := s.requireWorkItemPermission(ctx, item, actorID); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Service) normalizeAcceptanceRequest(ctx context.Context, projectID string, item Record, existing Record, req AcceptanceRequest, submit bool) (Record, []Record, error) {
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
			return nil, nil, validationError("Công việc liên quan không thuộc công trình nghiệm thu", Record{"work_item_id": workItemID, "project_id": projectID})
		}
	}

	acceptanceType := strings.TrimSpace(req.Type)
	if acceptanceType == "" {
		acceptanceType = "partial"
	}
	if !statusIn(acceptanceType, "partial", "final") {
		return nil, nil, validationError("Loại nghiệm thu không hợp lệ", Record{"type": acceptanceType})
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = "Nghiệm thu đầu ra"
	}
	if len([]rune(title)) > 240 {
		return nil, nil, validationError("Tiêu đề nghiệm thu không được vượt quá 240 ký tự")
	}
	acceptanceDate := strings.TrimSpace(req.AcceptanceDate)
	if acceptanceDate != "" {
		if _, err := time.Parse("2006-01-02", acceptanceDate); err != nil {
			return nil, nil, validationError("Ngày nghiệm thu không hợp lệ", Record{"acceptance_date": acceptanceDate})
		}
	} else if submit {
		return nil, nil, validationError("Ngày nghiệm thu là bắt buộc khi gửi")
	}
	location := strings.TrimSpace(req.Location)
	clientRepresentative := strings.TrimSpace(req.ClientRepresentative)
	contractorRepresentative := strings.TrimSpace(req.ContractorRepresentative)
	scopeSummary := strings.TrimSpace(req.ScopeSummary)
	for field, value := range map[string]string{
		"location":                  location,
		"client_representative":     clientRepresentative,
		"contractor_representative": contractorRepresentative,
	} {
		if len([]rune(value)) > 240 {
			return nil, nil, validationError("Thông tin nghiệm thu không được vượt quá 240 ký tự", Record{"field": field})
		}
	}
	if len([]rune(scopeSummary)) > 1000 {
		return nil, nil, validationError("Phạm vi nghiệm thu không được vượt quá 1000 ký tự")
	}
	notes := strings.TrimSpace(req.Notes)
	if len([]rune(notes)) > 2000 {
		return nil, nil, validationError("Ghi chú nghiệm thu không được vượt quá 2000 ký tự")
	}

	formInstanceID := cleanID(req.FormInstanceID)
	formHasSignature := false
	if formInstanceID != "" {
		form, err := s.repo.GetWorkflowFormInstanceDetail(ctx, formInstanceID)
		if err != nil {
			return nil, nil, validationError("Biểu mẫu quy trình liên kết nghiệm thu không hợp lệ", Record{"form_instance_id": formInstanceID})
		}
		instance, _ := form["instance"].(Record)
		if cleanID(instance["project_id"]) != projectID {
			return nil, nil, validationError("Biểu mẫu quy trình không thuộc công trình nghiệm thu", Record{"form_instance_id": formInstanceID})
		}
		if workItemID != "" && cleanID(instance["work_item_id"]) != workItemID {
			return nil, nil, validationError("Biểu mẫu quy trình không thuộc công việc liên quan", Record{"form_instance_id": formInstanceID})
		}
		if submit && fmt.Sprint(instance["status"]) != "submitted" {
			return nil, nil, validationError("Biểu mẫu quy trình cần gửi trước nghiệm thu đầu ra", Record{"form_instance_id": formInstanceID})
		}
		if signatures, ok := form["signatures"].([]Record); ok && len(signatures) > 0 {
			formHasSignature = true
		}
	}
	pdfExportID := cleanID(req.PDFExportID)
	if pdfExportID != "" {
		export, err := s.repo.GetWorkflowFormPDFExportDetail(ctx, pdfExportID)
		if err != nil {
			return nil, nil, validationError("PDF biểu mẫu quy trình liên kết nghiệm thu không hợp lệ", Record{"pdf_export_id": pdfExportID})
		}
		if cleanID(export["project_id"]) != projectID {
			return nil, nil, validationError("PDF biểu mẫu quy trình không thuộc công trình nghiệm thu", Record{"pdf_export_id": pdfExportID})
		}
		if workItemID != "" && cleanID(export["work_item_id"]) != workItemID {
			return nil, nil, validationError("PDF biểu mẫu quy trình không thuộc công việc liên quan", Record{"pdf_export_id": pdfExportID})
		}
		if formInstanceID != "" && cleanID(export["form_instance_id"]) != formInstanceID {
			return nil, nil, validationError("PDF biểu mẫu quy trình không thuộc biểu mẫu quy trình đã chọn", Record{"pdf_export_id": pdfExportID, "form_instance_id": formInstanceID})
		}
		if formInstanceID == "" {
			formInstanceID = cleanID(export["form_instance_id"])
		}
	}
	signatureStatus := strings.TrimSpace(req.SignatureStatus)
	if signatureStatus == "" {
		if formHasSignature {
			signatureStatus = "signed"
		} else {
			signatureStatus = "pending"
		}
	}
	if !statusIn(signatureStatus, "pending", "signed", "waived") {
		return nil, nil, validationError("Trạng thái chữ ký nghiệm thu không hợp lệ", Record{"signature_status": signatureStatus})
	}
	documentStatus := strings.TrimSpace(req.DocumentStatus)
	if documentStatus == "" {
		if pdfExportID != "" {
			documentStatus = "linked"
		} else {
			documentStatus = "pending"
		}
	}
	if !statusIn(documentStatus, "pending", "linked", "waived") {
		return nil, nil, validationError("Trạng thái tài liệu nghiệm thu không hợp lệ", Record{"document_status": documentStatus})
	}
	if submit && signatureStatus == "pending" {
		return nil, nil, validationError("Nghiệm thu đầu ra cần chữ ký hoặc miễn chữ ký trước khi gửi")
	}
	if submit && documentStatus == "pending" {
		return nil, nil, validationError("Nghiệm thu đầu ra cần tài liệu/PDF hoặc miễn tài liệu trước khi gửi")
	}
	if submit && len(req.Checklist) == 0 {
		return nil, nil, validationError("Cần ít nhất một dòng checklist để gửi nghiệm thu đầu ra")
	}

	seen := map[int]bool{}
	normalized := make([]Record, 0, len(req.Checklist))
	for index, itemReq := range req.Checklist {
		itemIndex := itemReq.ItemIndex
		if itemIndex < 0 {
			itemIndex = index
		}
		if itemIndex < 0 {
			return nil, nil, validationError("Dòng checklist nghiệm thu không hợp lệ", Record{"item_index": itemReq.ItemIndex})
		}
		if seen[itemIndex] {
			return nil, nil, validationError("Dòng checklist nghiệm thu bị trùng", Record{"item_index": itemIndex})
		}
		seen[itemIndex] = true
		row, err := s.normalizeAcceptanceChecklistItem(ctx, projectID, workItemID, itemIndex, itemReq, submit)
		if err != nil {
			return nil, nil, err
		}
		normalized = append(normalized, row)
	}
	summary := acceptanceSummary(normalized)
	if submit {
		if pending := int(number(summary["pending"])); pending > 0 {
			return nil, nil, validationError("Không thể gửi nghiệm thu đầu ra khi còn checklist chờ xử lý", Record{"pending": pending})
		}
	}
	header := Record{
		"project_id":                projectID,
		"work_item_id":              workItemID,
		"form_instance_id":          formInstanceID,
		"pdf_export_id":             pdfExportID,
		"type":                      acceptanceType,
		"title":                     title,
		"acceptance_date":           acceptanceDate,
		"location":                  location,
		"client_representative":     clientRepresentative,
		"contractor_representative": contractorRepresentative,
		"scope_summary":             scopeSummary,
		"signature_status":          signatureStatus,
		"document_status":           documentStatus,
		"notes":                     notes,
		"summary_json":              summary,
	}
	return header, normalized, nil
}

func (s *Service) normalizeAcceptanceChecklistItem(ctx context.Context, projectID, workItemID string, itemIndex int, req AcceptanceChecklistItemRequest, submit bool) (Record, error) {
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return nil, validationError("Tên checklist nghiệm thu là bắt buộc", Record{"item_index": itemIndex})
	}
	if len([]rune(title)) > 240 {
		return nil, validationError("Tên checklist nghiệm thu không được vượt quá 240 ký tự", Record{"item_index": itemIndex})
	}
	description := strings.TrimSpace(req.Description)
	if len([]rune(description)) > 1000 {
		return nil, validationError("Mô tả checklist nghiệm thu không được vượt quá 1000 ký tự", Record{"item_index": itemIndex})
	}
	decision := strings.TrimSpace(req.Decision)
	if decision == "" {
		decision = "pending"
	}
	if !statusIn(decision, "pending", "pass", "conditional_pass", "fail") {
		return nil, validationError("Quyết định checklist nghiệm thu không hợp lệ", Record{"item_index": itemIndex, "decision": decision})
	}
	if submit && decision == "pending" {
		return nil, validationError("Checklist nghiệm thu chưa có quyết định", Record{"item_index": itemIndex})
	}
	severity := strings.TrimSpace(req.Severity)
	if severity == "" {
		severity = "medium"
	}
	if !statusIn(severity, "low", "medium", "high", "critical") {
		return nil, validationError("Mức độ checklist nghiệm thu không hợp lệ", Record{"item_index": itemIndex, "severity": severity})
	}
	requiredAction := strings.TrimSpace(req.RequiredAction)
	if len([]rune(requiredAction)) > 1000 {
		return nil, validationError("Yêu cầu xử lý checklist nghiệm thu không được vượt quá 1000 ký tự", Record{"item_index": itemIndex})
	}
	dueDate := strings.TrimSpace(req.DueDate)
	if dueDate != "" {
		if _, err := time.Parse("2006-01-02", dueDate); err != nil {
			return nil, validationError("Ngày hạn xử lý checklist nghiệm thu không hợp lệ", Record{"item_index": itemIndex, "due_date": dueDate})
		}
	}
	note := strings.TrimSpace(req.Note)
	if len([]rune(note)) > 1000 {
		return nil, validationError("Ghi chú checklist nghiệm thu không được vượt quá 1000 ký tự", Record{"item_index": itemIndex})
	}
	photoURL := strings.TrimSpace(req.PhotoURL)
	if len([]rune(photoURL)) > 1000 {
		return nil, validationError("URL ảnh checklist nghiệm thu không được vượt quá 1000 ký tự", Record{"item_index": itemIndex})
	}
	evidenceIDs := normalizeAcceptanceEvidenceIDs(req.EvidenceIDs, req.EvidenceID)
	if err := validateAcceptanceEvidenceLimit(evidenceIDs, itemIndex); err != nil {
		return nil, err
	}
	for _, evidenceID := range evidenceIDs {
		evidence, err := s.repo.GetEvidenceDetail(ctx, evidenceID)
		if err != nil {
			return nil, validationError("Minh chứng checklist nghiệm thu không hợp lệ", Record{"item_index": itemIndex, "evidence_id": evidenceID})
		}
		if cleanID(evidence["project_id"]) != projectID || fmt.Sprint(evidence["status"]) == "rejected" {
			return nil, validationError("Minh chứng checklist nghiệm thu không thuộc công trình", Record{"item_index": itemIndex, "evidence_id": evidenceID})
		}
		if workItemID != "" && cleanID(evidence["work_item_id"]) != workItemID {
			return nil, validationError("Minh chứng checklist nghiệm thu không thuộc công việc liên quan", Record{"item_index": itemIndex, "evidence_id": evidenceID})
		}
	}
	evidenceRequired := req.PhotoRequired || decision == "conditional_pass" || decision == "fail"
	if submit && evidenceRequired && len(evidenceIDs) == 0 && photoURL == "" {
		return nil, validationError("Checklist nghiệm thu cần minh chứng/ảnh", Record{"item_index": itemIndex})
	}
	if submit && statusIn(decision, "conditional_pass", "fail") && requiredAction == "" {
		return nil, validationError("Checklist có điều kiện/không đạt cần yêu cầu xử lý", Record{"item_index": itemIndex})
	}
	return Record{
		"item_index":      itemIndex,
		"title":           title,
		"description":     description,
		"decision":        decision,
		"severity":        severity,
		"required_action": requiredAction,
		"due_date":        dueDate,
		"evidence_id":     firstString(evidenceIDs),
		"evidence_ids":    evidenceIDs,
		"note":            note,
		"photo_required":  req.PhotoRequired,
		"photo_url":       photoURL,
		"payload_json": Record{
			"evidence_required": evidenceRequired,
		},
	}, nil
}

func acceptanceRequestFromRecord(record Record) AcceptanceRequest {
	req := AcceptanceRequest{
		WorkItemID:               cleanID(record["work_item_id"]),
		FormInstanceID:           cleanID(record["form_instance_id"]),
		PDFExportID:              cleanID(record["pdf_export_id"]),
		Type:                     cleanOptionalText(record["type"]),
		Title:                    cleanOptionalText(record["title"]),
		AcceptanceDate:           dateOnly(record["acceptance_date"]),
		Location:                 cleanOptionalText(record["location"]),
		ClientRepresentative:     cleanOptionalText(record["client_representative"]),
		ContractorRepresentative: cleanOptionalText(record["contractor_representative"]),
		ScopeSummary:             cleanOptionalText(record["scope_summary"]),
		SignatureStatus:          cleanOptionalText(record["signature_status"]),
		DocumentStatus:           cleanOptionalText(record["document_status"]),
		Notes:                    cleanOptionalText(record["notes"]),
		Checklist:                []AcceptanceChecklistItemRequest{},
	}
	if items, ok := record["checklist"].([]Record); ok {
		req.Checklist = make([]AcceptanceChecklistItemRequest, 0, len(items))
		for _, item := range items {
			req.Checklist = append(req.Checklist, AcceptanceChecklistItemRequest{
				ItemIndex:      int(number(item["item_index"])),
				Title:          cleanOptionalText(item["title"]),
				Description:    cleanOptionalText(item["description"]),
				Decision:       cleanOptionalText(item["decision"]),
				Severity:       cleanOptionalText(item["severity"]),
				RequiredAction: cleanOptionalText(item["required_action"]),
				DueDate:        dateOnly(item["due_date"]),
				EvidenceID:     cleanID(item["evidence_id"]),
				EvidenceIDs:    normalizeAcceptanceEvidenceIDs(stringSlice(item["evidence_ids"]), cleanID(item["evidence_id"])),
				Note:           cleanOptionalText(item["note"]),
				PhotoRequired:  boolValue(item["photo_required"]),
				PhotoURL:       cleanOptionalText(item["photo_url"]),
			})
		}
	}
	return req
}

func normalizeAcceptanceEvidenceIDs(ids []string, primaryID string) []string {
	seen := map[string]bool{}
	values := make([]string, 0, len(ids)+1)
	add := func(value string) {
		value = cleanID(value)
		if value == "" || seen[value] {
			return
		}
		seen[value] = true
		values = append(values, value)
	}
	add(primaryID)
	for _, id := range ids {
		add(id)
	}
	return values
}

func validateAcceptanceEvidenceLimit(ids []string, itemIndex int) error {
	if len(ids) <= 10 {
		return nil
	}
	return validationError("Mỗi tiêu chí nghiệm thu được chọn tối đa 10 minh chứng", Record{"item_index": itemIndex, "evidence_count": len(ids), "max": 10})
}

func firstString(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func acceptanceSummary(items []Record) Record {
	summary := Record{
		"total":          len(items),
		"pass":           0,
		"conditional":    0,
		"fail":           0,
		"pending":        0,
		"evidence_count": 0,
		"critical_count": 0,
	}
	for _, item := range items {
		switch cleanOptionalText(item["decision"]) {
		case "pass":
			summary["pass"] = int(number(summary["pass"])) + 1
		case "conditional_pass":
			summary["conditional"] = int(number(summary["conditional"])) + 1
		case "fail":
			summary["fail"] = int(number(summary["fail"])) + 1
		default:
			summary["pending"] = int(number(summary["pending"])) + 1
		}
		if cleanID(item["evidence_id"]) != "" || cleanOptionalText(item["photo_url"]) != "" {
			summary["evidence_count"] = int(number(summary["evidence_count"])) + 1
		}
		if cleanOptionalText(item["severity"]) == "critical" {
			summary["critical_count"] = int(number(summary["critical_count"])) + 1
		}
	}
	return summary
}

func validateAcceptanceActionResult(result string, summary Record) error {
	result = strings.TrimSpace(result)
	total := int(number(summary["total"]))
	passed := int(number(summary["pass"]))
	conditional := int(number(summary["conditional"]))
	failed := int(number(summary["fail"]))
	pending := int(number(summary["pending"]))
	if total == 0 {
		return validationError("Cần hạng mục checklist trước khi chốt nghiệm thu đầu ra")
	}
	if pending > 0 {
		return validationError("Không thể chốt nghiệm thu khi còn checklist pending", Record{"pending": pending})
	}
	switch result {
	case "pass":
		if passed != total {
			return validationError("Kết quả đạt chỉ hợp lệ khi mọi checklist đều đạt")
		}
	case "conditional_pass":
		if conditional+failed == 0 {
			return validationError("Kết quả đạt có điều kiện cần checklist có điều kiện hoặc không đạt")
		}
	case "fail":
		if failed == 0 {
			return validationError("Kết quả không đạt cần ít nhất một checklist không đạt")
		}
	default:
		return validationError("Kết quả nghiệm thu không hợp lệ", Record{"result": result})
	}
	return nil
}

func recordValue(value any) Record {
	switch v := value.(type) {
	case Record:
		return v
	case map[string]any:
		return Record(v)
	default:
		return Record{}
	}
}
