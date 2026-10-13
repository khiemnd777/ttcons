package core

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func (s *Service) ProjectHandover(ctx context.Context, projectID string) ([]Record, error) {
	return s.repo.ListProjectHandover(ctx, cleanID(projectID))
}

func (s *Service) HandoverOptions(ctx context.Context, projectID string) (Record, error) {
	return s.repo.HandoverOptions(ctx, cleanID(projectID))
}

func (s *Service) CreateHandover(ctx context.Context, projectID string, req HandoverCreateRequest, actorID, actorName string) (Record, error) {
	projectID = cleanID(projectID)
	if !validActivityUUID(projectID) {
		return nil, validationError("project_id không hợp lệ", Record{"project_id": projectID})
	}
	clientRepresentative := strings.TrimSpace(req.ClientRepresentative)
	companyRepresentative := strings.TrimSpace(req.CompanyRepresentative)
	if clientRepresentative == "" || companyRepresentative == "" {
		return nil, validationError("Hồ sơ bàn giao cần đại diện hai bên", Record{"client_representative": clientRepresentative, "company_representative": companyRepresentative})
	}
	handoverCode := strings.TrimSpace(req.HandoverCode)
	if handoverCode == "" {
		randomCode := strings.ToUpper(strings.ReplaceAll(newID(), "-", ""))
		handoverCode = "HO-" + randomCode[:10]
	}
	if len([]rune(handoverCode)) > 80 {
		return nil, validationError("Mã bàn giao không được vượt quá 80 ký tự")
	}
	handoverDate, err := normalizeOptionalDate(req.HandoverDate, "handover_date")
	if err != nil {
		return nil, err
	}
	finalAcceptanceID := cleanID(req.FinalAcceptanceID)
	if finalAcceptanceID != "" {
		acceptance, err := s.repo.GetByID(ctx, "acceptance_records", finalAcceptanceID)
		if err != nil || cleanID(acceptance["project_id"]) != projectID || fmt.Sprint(acceptance["type"]) != "final" {
			return nil, validationError("Nghiệm thu cuối không thuộc công trình", Record{"final_acceptance_id": finalAcceptanceID})
		}
	}
	formInstanceID := cleanID(req.WorkflowFormInstanceID)
	if formInstanceID != "" {
		instance, err := s.repo.GetByID(ctx, "workflow_form_instances", formInstanceID)
		if err != nil || cleanID(instance["project_id"]) != projectID {
			return nil, validationError("Biểu mẫu bàn giao không thuộc công trình", Record{"workflow_form_instance_id": formInstanceID})
		}
	}
	notes := strings.TrimSpace(req.Notes)
	handoverStage, err := nativeCaptureText(req.HandoverStage, "handover_stage", 160)
	if err != nil {
		return nil, err
	}
	if len([]rune(notes)) > 4000 {
		return nil, validationError("Ghi chú bàn giao không được vượt quá 4000 ký tự")
	}
	return s.repo.CreateHandoverPackage(ctx, Record{
		"project_id": projectID, "handover_code": handoverCode, "status": "draft",
		"final_acceptance_id": finalAcceptanceID, "workflow_form_instance_id": formInstanceID,
		"handover_date": handoverDate, "client_representative": clientRepresentative,
		"handover_stage":         handoverStage,
		"company_representative": companyRepresentative, "documents_ready": false,
		"warehouse_closed": false,
		"notes":            notes, "created_by": actorID,
	}, req.WorkItemContext, actorID, actorName)
}

func (s *Service) HandoverDetail(ctx context.Context, id string) (Record, error) {
	return s.repo.GetHandoverDetail(ctx, cleanID(id))
}

func (s *Service) SubmitHandover(ctx context.Context, id string, req ActionRequest, actorID, actorName string) (Record, error) {
	return s.repo.SubmitHandover(ctx, cleanID(id), req, actorID, actorName)
}

func (s *Service) HandoverCheck(ctx context.Context, id, actorID, actorName string) (Record, error) {
	return s.repo.HandoverCheck(ctx, cleanID(id), actorID, actorName)
}

func (s *Service) UpdateHandoverChecklistItem(ctx context.Context, id string, req HandoverChecklistItemRequest, actorID, actorName string) (Record, error) {
	status := strings.TrimSpace(req.Status)
	if !statusIn(status, "pending", "verified", "waived") {
		return nil, validationError("Trạng thái checklist bàn giao không hợp lệ", Record{"status": status})
	}
	evidenceID := cleanID(req.EvidenceID)
	documentID := cleanID(req.DocumentID)
	notes := strings.TrimSpace(req.Notes)
	if len([]rune(notes)) > 2000 {
		return nil, validationError("Ghi chú checklist không được vượt quá 2000 ký tự")
	}
	body := Record{"status": status, "evidence_id": evidenceID, "document_id": documentID, "notes": notes}
	for _, field := range []struct {
		key   string
		value *string
		limit int
	}{{"requirement", req.Requirement, 1000}, {"physical_result", req.PhysicalResult, 20}, {"defect", req.Defect, 1000}, {"due_date", req.DueDate, 10}} {
		if field.value == nil {
			continue
		}
		value, err := nativeCaptureText(*field.value, field.key, field.limit)
		if err != nil {
			return nil, err
		}
		body[field.key] = value
	}
	physicalResult := cleanText(body["physical_result"])
	if physicalResult != "" && !statusIn(physicalResult, "pass", "fail", "not_assessed") {
		return nil, validationError("Đánh giá thực tế bàn giao không hợp lệ", nil)
	}
	dueDate := cleanText(body["due_date"])
	if dueDate != "" {
		if _, err := time.Parse("2006-01-02", dueDate); err != nil {
			return nil, validationError("Thời hạn xử lý cần ngày hợp lệ", nil)
		}
	}
	return s.repo.UpdateHandoverChecklistItem(ctx, cleanID(id), body, actorID, actorName)
}

func (s *Service) CompleteHandover(ctx context.Context, id string, req ActionRequest, actorID, actorName string) (Record, error) {
	return s.repo.CompleteHandover(ctx, id, req, actorID, actorName)
}
