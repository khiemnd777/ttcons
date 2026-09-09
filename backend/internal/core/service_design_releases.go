package core

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"mime/multipart"
	"strings"
)

var designReleaseDocumentTypes = map[string]bool{
	"layout":               true,
	"render_3d":            true,
	"construction_drawing": true,
	"other":                true,
}

type designReleaseCapabilities struct {
	CanView    bool
	CanManage  bool
	CanHandoff bool
}

func (s *Service) DesignReleaseWorkspace(ctx context.Context, projectID, actorID string) (Record, error) {
	projectID = cleanID(projectID)
	capabilities, err := s.designReleaseCapabilities(ctx, projectID, actorID)
	if err != nil {
		return nil, err
	}
	if !capabilities.CanView {
		return nil, forbiddenError("Bạn không có quyền xem revision hồ sơ thiết kế của công trình")
	}
	releases, err := s.repo.ListDesignReleases(ctx, projectID)
	if err != nil {
		return nil, err
	}
	handoff, err := s.repo.LatestDesignReleaseHandoff(ctx, projectID)
	if err != nil {
		return nil, err
	}
	current := designReleaseFromRows(releases, "released")
	draft := designReleaseFromRows(releases, "draft")
	requiresRehandoff := current != nil && (handoff == nil || cleanID(handoff["design_release_id"]) != cleanID(current["id"]))
	return Record{
		"project_id":         projectID,
		"releases":           releases,
		"current_release":    current,
		"draft_release":      draft,
		"latest_handoff":     handoff,
		"requires_rehandoff": requiresRehandoff,
		"capabilities": Record{
			"can_manage":  capabilities.CanManage,
			"can_handoff": capabilities.CanHandoff,
		},
	}, nil
}

func (s *Service) CreateDesignRelease(ctx context.Context, projectID string, body Record, actorID, actorName string) (Record, error) {
	projectID = cleanID(projectID)
	if err := s.requireDesignReleaseManager(ctx, projectID, actorID); err != nil {
		return nil, err
	}
	revisionLabel, changeReason, notes, err := validateDesignReleaseDraft(body)
	if err != nil {
		return nil, err
	}
	workItemID := cleanID(body["work_item_id"])
	if err := s.validateDesignReleaseWorkItem(ctx, projectID, workItemID, false); err != nil {
		return nil, err
	}
	return s.repo.CreateDesignRelease(ctx, projectID, workItemID, revisionLabel, changeReason, notes, actorID, actorName)
}

func (s *Service) UpdateDesignReleaseDraft(ctx context.Context, releaseID string, body Record, actorID, actorName string) (Record, error) {
	release, err := s.repo.DesignReleaseByID(ctx, cleanID(releaseID))
	if err != nil {
		return nil, err
	}
	if err := s.requireDesignReleaseManager(ctx, cleanID(release["project_id"]), actorID); err != nil {
		return nil, err
	}
	revisionLabel, changeReason, notes, err := validateDesignReleaseDraft(body)
	if err != nil {
		return nil, err
	}
	if revisionLabel == "" {
		revisionLabel = strings.TrimSpace(fmt.Sprint(release["revision_label"]))
	}
	return s.repo.UpdateDesignReleaseDraft(ctx, cleanID(releaseID), revisionLabel, changeReason, notes, actorID, actorName, recordFromAny(body["expected_values"]))
}

func (s *Service) UploadDesignReleaseFile(ctx context.Context, releaseID, documentType, name string, fileHeader *multipart.FileHeader, actorID, actorName string) (Record, error) {
	releaseID = cleanID(releaseID)
	release, err := s.repo.DesignReleaseByID(ctx, releaseID)
	if err != nil {
		return nil, err
	}
	projectID := cleanID(release["project_id"])
	if err := s.requireDesignReleaseManager(ctx, projectID, actorID); err != nil {
		return nil, err
	}
	documentType = strings.TrimSpace(documentType)
	if !designReleaseDocumentTypes[documentType] {
		return nil, validationError("Nhóm tệp thiết kế không hợp lệ", Record{"document_type": documentType})
	}
	file, storedObject, originalName, err := s.storeProjectDocumentUpload(ctx, projectID, "design_release", releaseID, fileHeader)
	if err != nil {
		return nil, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = originalName
	}
	if len([]rune(name)) > 240 {
		return nil, s.cleanupStoredObjectAfterError(storedObject, validationError("Tên tệp thiết kế không được vượt quá 240 ký tự"))
	}
	document, err := s.repo.AddDesignReleaseFile(ctx, releaseID, documentType, name, originalName, file, actorID, actorName)
	if err != nil {
		return nil, s.cleanupStoredObjectAfterError(storedObject, err)
	}
	s.publishDiscussionWorkUpdate(projectID, cleanID(release["work_item_id"]), actorID)
	return document, nil
}

func (s *Service) ReleaseDesignRelease(ctx context.Context, releaseID, actorID, actorName string) (Record, error) {
	release, err := s.repo.DesignReleaseByID(ctx, cleanID(releaseID))
	if err != nil {
		return nil, err
	}
	if err := s.requireDesignReleaseManager(ctx, cleanID(release["project_id"]), actorID); err != nil {
		return nil, err
	}
	result, err := s.repo.ReleaseDesignRelease(ctx, cleanID(releaseID), actorID, actorName)
	if err == nil {
		s.publishDiscussionWorkUpdate(cleanID(release["project_id"]), cleanID(release["work_item_id"]), actorID)
	}
	return result, err
}

func (s *Service) HandoffDesignRelease(ctx context.Context, releaseID string, body Record, actorID, actorName string) (Record, error) {
	release, err := s.repo.DesignReleaseByID(ctx, cleanID(releaseID))
	if err != nil {
		return nil, err
	}
	projectID := cleanID(release["project_id"])
	capabilities, err := s.designReleaseCapabilities(ctx, projectID, actorID)
	if err != nil {
		return nil, err
	}
	if !capabilities.CanHandoff {
		return nil, forbiddenError("Chỉ PM của công trình mới có thể bàn giao revision thiết kế cho QS")
	}
	workItemID := cleanID(body["work_item_id"])
	if err := s.validateDesignReleaseWorkItem(ctx, projectID, workItemID, true); err != nil {
		return nil, err
	}
	comment := strings.TrimSpace(fmt.Sprint(body["comment"]))
	if comment == "<nil>" {
		comment = ""
	}
	if len([]rune(comment)) > 1000 {
		return nil, validationError("Ghi chú bàn giao không được vượt quá 1000 ký tự")
	}
	result, err := s.repo.HandoffDesignRelease(ctx, cleanID(releaseID), workItemID, comment, actorID, actorName)
	if err == nil {
		s.publishDiscussionWorkUpdate(cleanID(release["project_id"]), workItemID, actorID)
	}
	return result, err
}

func (s *Service) designReleaseCapabilities(ctx context.Context, projectID, actorID string) (designReleaseCapabilities, error) {
	canView, err := s.repo.CanAccessProjectPermission(ctx, actorID, projectID, []string{"project.view"})
	if err != nil || !canView {
		return designReleaseCapabilities{CanView: canView}, err
	}
	canEdit, err := s.repo.CanAccessProjectPermission(ctx, actorID, projectID, []string{"planning.edit"})
	if err != nil {
		return designReleaseCapabilities{}, err
	}
	profile, err := s.repo.WorkItemAccessProfile(ctx, actorID)
	if err != nil {
		return designReleaseCapabilities{}, err
	}
	projectRoles, err := s.repo.UserProjectRoleLabels(ctx, actorID, projectID)
	if err != nil {
		return designReleaseCapabilities{}, err
	}
	roleLabels := append(append([]string{}, profile.RoleLabels...), projectRoles...)
	canManage := canEdit && (profile.IsAdmin || roleLabelsIntersect(roleLabels, []string{"design", "Thiết kế"}))
	isProjectManager, err := s.repo.UserIsProjectManager(ctx, actorID, projectID)
	if err != nil {
		return designReleaseCapabilities{}, err
	}
	isPMRole := roleLabelsIntersect(roleLabels, []string{"pm", "project manager"})
	return designReleaseCapabilities{
		CanView:    true,
		CanManage:  canManage,
		CanHandoff: canEdit && (profile.IsAdmin || (isProjectManager && isPMRole)),
	}, nil
}

func (s *Service) requireDesignReleaseManager(ctx context.Context, projectID, actorID string) error {
	capabilities, err := s.designReleaseCapabilities(ctx, projectID, actorID)
	if err != nil {
		return err
	}
	if !capabilities.CanManage {
		return forbiddenError("Chỉ vai trò Thiết kế của công trình mới có thể chỉnh sửa revision hồ sơ thiết kế")
	}
	return nil
}

func (s *Service) validateDesignReleaseWorkItem(ctx context.Context, projectID, workItemID string, handoff bool) error {
	if workItemID == "" {
		return nil
	}
	row, err := s.repo.QueryOne(ctx, `
		SELECT wi.project_id,
			COALESCE(node.business_code,'') AS node_business_code,
			COALESCE(step.business_code,'') AS step_business_code,
			COALESCE(step.domain_action_key,'') AS domain_action_key
		FROM work_items wi
		LEFT JOIN pipeline_run_nodes node ON node.id=wi.run_node_id
		LEFT JOIN pipeline_run_steps step ON step.id=wi.run_step_id
		WHERE wi.id=$1 AND wi.status <> 'cancelled'
	`, workItemID)
	if errors.Is(err, sql.ErrNoRows) {
		return validationError("Công việc liên kết revision thiết kế không tồn tại", Record{"work_item_id": workItemID})
	}
	if err != nil {
		return err
	}
	if cleanID(row["project_id"]) != projectID {
		return validationError("Công việc liên kết không thuộc công trình", Record{"work_item_id": workItemID})
	}
	nodeCode := strings.ToLower(strings.TrimSpace(fmt.Sprint(row["node_business_code"])))
	actionKey := strings.TrimSpace(fmt.Sprint(row["domain_action_key"]))
	if handoff {
		if actionKey != "source.design_documents_ready" {
			return validationError("Công việc liên kết không phải bước PM bàn giao hồ sơ thiết kế", Record{"work_item_id": workItemID})
		}
		return nil
	}
	if nodeCode != "design" && !strings.HasPrefix(strings.ToLower(strings.TrimSpace(fmt.Sprint(row["step_business_code"]))), "design.") {
		return validationError("Công việc liên kết không thuộc node Thiết kế", Record{"work_item_id": workItemID})
	}
	return nil
}

func validateDesignReleaseDraft(body Record) (string, string, string, error) {
	revisionLabel := strings.TrimSpace(fmt.Sprint(body["revision_label"]))
	if revisionLabel == "<nil>" {
		revisionLabel = ""
	}
	changeReason := strings.TrimSpace(fmt.Sprint(body["change_reason"]))
	if changeReason == "" || changeReason == "<nil>" {
		return "", "", "", validationError("Lý do cập nhật revision thiết kế là bắt buộc")
	}
	notes := strings.TrimSpace(fmt.Sprint(body["notes"]))
	if notes == "<nil>" {
		notes = ""
	}
	if len([]rune(revisionLabel)) > 40 {
		return "", "", "", validationError("Mã revision không được vượt quá 40 ký tự")
	}
	if revisionLabel != "" && strings.ContainsAny(revisionLabel, "\r\n\t") {
		return "", "", "", validationError("Mã revision không hợp lệ")
	}
	if len([]rune(changeReason)) > 1000 {
		return "", "", "", validationError("Lý do cập nhật không được vượt quá 1000 ký tự")
	}
	if len([]rune(notes)) > 4000 {
		return "", "", "", validationError("Ghi chú revision không được vượt quá 4000 ký tự")
	}
	return revisionLabel, changeReason, notes, nil
}
