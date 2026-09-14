package core

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

func (s *Service) ListProjects(ctx context.Context, actorID string) ([]Record, error) {
	return s.repo.ListProjects(ctx, actorID)
}

func (s *Service) ProjectOptions(ctx context.Context, req ReportOptionsRequest, actorID string) (ReportOptionsResponse, error) {
	return s.projectOptions(ctx, req, actorID, "")
}

func (s *Service) ProjectProgressBoard(ctx context.Context, actorID string) (Record, error) {
	items, err := s.repo.ProjectProgressBoard(ctx, actorID)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		item["stage_key"] = projectProgressStageKey(
			strings.TrimSpace(fmt.Sprint(item["project_status"])),
			cleanID(item["pipeline_run_id"]),
			strings.TrimSpace(fmt.Sprint(item["pipeline_run_status"])),
			strings.TrimSpace(fmt.Sprint(item["current_node_type"])),
		)
	}
	return Record{
		"items":        items,
		"generated_at": time.Now().UTC().Format(time.RFC3339),
	}, nil
}

func projectProgressStageKey(projectStatus, runID, runStatus, nodeType string) string {
	if statusIn(projectStatus, "completed", "complete") || runStatus == "completed" {
		return "completed"
	}
	if runID == "" {
		return "not_started"
	}
	switch nodeType {
	case "survey":
		return "survey"
	case "design":
		return "design"
	case "boq", "budget_approval", "procurement", "warehouse_issue", "contract":
		return "commercial"
	case "construction_planning":
		return "planning"
	case "construction":
		return "construction"
	case "incoming_acceptance", "acceptance", "rework", "replanning":
		return "acceptance"
	case "handover", "financial_closure":
		return "handover_finance"
	default:
		return "other"
	}
}

func (s *Service) CreateProject(ctx context.Context, body Record, actorID, actorName string) (Record, error) {
	if body["code"] == nil || body["code"] == "" {
		body["code"] = fmt.Sprintf("PRJ-%d", time.Now().Unix())
	}
	if body["status"] == nil {
		body["status"] = "draft"
	}
	// The workflow form and connector configuration own runtime routing.
	// Ignore this legacy project field for newly created projects.
	delete(body, "workflow_entry_mode")
	if err := s.applyPartnerSnapshot(ctx, body, "partner_id", "client_name", "", []string{"client"}); err != nil {
		return nil, err
	}
	project, err := s.repo.CreateProjectWithStartup(ctx, body, actorID, actorName)
	if err != nil {
		return nil, err
	}
	s.publishProjectProgressChanged("")
	s.publishWorkQueueChanged(cleanID(project["id"]))
	return project, nil
}

func (s *Service) UpdateProject(ctx context.Context, id string, body Record) (Record, error) {
	delete(body, "workflow_entry_mode")
	// PM changes must use the dedicated audited handover endpoint.
	delete(body, "pm_staff_id")
	delete(body, "created_by")
	if err := s.applyPartnerSnapshot(ctx, body, "partner_id", "client_name", "", []string{"client"}); err != nil {
		return nil, err
	}
	project, err := s.repo.Update(ctx, "projects", projectColumns(), id, body)
	if err != nil {
		return nil, err
	}
	s.publishProjectProgressChanged(cleanID(project["id"]))
	return project, nil
}

func (s *Service) ProjectDetail(ctx context.Context, id string) (Record, error) {
	return s.repo.GetProjectDetail(ctx, id)
}

func (s *Service) ProjectMembers(ctx context.Context, projectID, actorID string, includeInactive bool) (Record, error) {
	canManage, err := s.repo.CanAccessProjectPermission(ctx, actorID, projectID, []string{"project.member.manage"})
	if err != nil {
		return nil, err
	}
	includeInactive = includeInactive && canManage
	items, err := s.repo.ListProjectMembers(ctx, projectID, includeInactive)
	if err != nil {
		return nil, err
	}
	options := Record{"staff": []Record{}, "access_roles": []Record{}, "role_labels": []string{}}
	if canManage {
		options, err = s.repo.ProjectMemberOptions(ctx, projectID)
		if err != nil {
			return nil, err
		}
	}
	return Record{
		"items":   items,
		"summary": projectMemberSummary(items),
		"options": options,
		"capabilities": Record{
			"can_manage":        canManage,
			"can_view_inactive": canManage,
		},
	}, nil
}

func (s *Service) SaveProjectMember(ctx context.Context, projectID, memberID string, req ProjectMemberMutationRequest, actorID, actorName string) (Record, error) {
	normalized, err := normalizeProjectMemberMutationRequest(projectID, memberID, req)
	if err != nil {
		return nil, err
	}
	member, err := s.repo.SaveProjectMember(ctx, projectID, memberID, normalized, actorID, actorName)
	if err != nil {
		return nil, err
	}
	s.publishWorkQueueChanged(projectID)
	return member, nil
}

func (s *Service) SaveProjectMembersBulk(ctx context.Context, projectID string, req ProjectMemberBulkMutationRequest, actorID, actorName string) (Record, error) {
	if len(req.Members) == 0 {
		return nil, validationError("Chọn ít nhất một nhân sự để thêm vào công trình")
	}
	if len(req.Members) > 50 {
		return nil, validationError("Mỗi lần chỉ có thể thêm tối đa 50 thành viên")
	}
	normalized := make([]ProjectMemberMutationRequest, 0, len(req.Members))
	seenStaff := map[string]bool{}
	for index, member := range req.Members {
		item, err := normalizeProjectMemberMutationRequest(projectID, "", member)
		if err != nil {
			return nil, validationError("Thành viên trong danh sách không hợp lệ", Record{"index": index, "cause": err.Error()})
		}
		if seenStaff[item.StaffID] {
			return nil, validationError("Danh sách có nhân sự bị trùng", Record{"index": index, "staff_id": item.StaffID})
		}
		seenStaff[item.StaffID] = true
		normalized = append(normalized, item)
	}
	members, err := s.repo.SaveProjectMembersBulk(ctx, strings.TrimSpace(projectID), normalized, actorID, actorName)
	if err != nil {
		return nil, err
	}
	s.publishWorkQueueChanged(projectID)
	return Record{"items": members, "total": len(members)}, nil
}

func (s *Service) DeactivateProjectMember(ctx context.Context, projectID, memberID, actorID, actorName string) (Record, error) {
	if _, err := uuid.Parse(strings.TrimSpace(projectID)); err != nil {
		return nil, validationError("project_id không hợp lệ", Record{"project_id": projectID})
	}
	if _, err := uuid.Parse(strings.TrimSpace(memberID)); err != nil {
		return nil, validationError("member_id không hợp lệ", Record{"member_id": memberID})
	}
	member, err := s.repo.DeactivateProjectMember(ctx, strings.TrimSpace(projectID), strings.TrimSpace(memberID), actorID, actorName)
	if err != nil {
		return nil, err
	}
	s.publishWorkQueueChanged(projectID)
	return member, nil
}

func normalizeProjectMemberMutationRequest(projectID, memberID string, req ProjectMemberMutationRequest) (ProjectMemberMutationRequest, error) {
	projectID = strings.TrimSpace(projectID)
	memberID = strings.TrimSpace(memberID)
	req.StaffID = strings.TrimSpace(req.StaffID)
	req.Notes = strings.TrimSpace(req.Notes)
	if _, err := uuid.Parse(projectID); err != nil {
		return req, validationError("project_id không hợp lệ", Record{"project_id": projectID})
	}
	if memberID != "" {
		if _, err := uuid.Parse(memberID); err != nil {
			return req, validationError("member_id không hợp lệ", Record{"member_id": memberID})
		}
	}
	if _, err := uuid.Parse(req.StaffID); err != nil {
		return req, validationError("Phải chọn nhân sự hợp lệ", Record{"staff_id": req.StaffID})
	}
	req.RoleIDs = normalizedRoleList(req.RoleIDs)
	if len(req.RoleIDs) == 0 {
		return req, validationError("Chọn ít nhất một vai trò vận hành", Record{"field": "role_ids"})
	}
	if len(req.RoleIDs) > 12 {
		return req, validationError("Một thành viên không thể có quá 12 vai trò vận hành")
	}
	for _, role := range req.RoleIDs {
		if len([]rune(role)) > 100 {
			return req, validationError("Tên vai trò vận hành không được vượt quá 100 ký tự", Record{"role": role})
		}
	}
	if len(req.Notes) > 1000 {
		return req, validationError("Ghi chú phân công không được vượt quá 1000 ký tự")
	}
	seen := map[string]bool{}
	projectRoleIDs := make([]string, 0, len(req.ProjectRoleIDs))
	for _, roleID := range req.ProjectRoleIDs {
		roleID = strings.TrimSpace(roleID)
		if roleID == "" || seen[roleID] {
			continue
		}
		if _, err := uuid.Parse(roleID); err != nil {
			return req, validationError("Role phân quyền không hợp lệ", Record{"role_id": roleID})
		}
		seen[roleID] = true
		projectRoleIDs = append(projectRoleIDs, roleID)
	}
	req.ProjectRoleIDs = projectRoleIDs
	return req, nil
}

func projectMemberSummary(items []Record) Record {
	departments := map[string]bool{}
	roles := map[string]bool{}
	active := 0
	inactive := 0
	for _, item := range items {
		if item["active"] == true {
			active++
			department := strings.TrimSpace(fmt.Sprint(item["department"]))
			if department != "" && department != "<nil>" {
				departments[department] = true
			}
			for _, role := range stringSlice(item["role_ids"]) {
				roles[role] = true
			}
		} else {
			inactive++
		}
	}
	return Record{
		"active":      active,
		"inactive":    inactive,
		"departments": len(departments),
		"roles":       len(roles),
	}
}
