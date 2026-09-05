package core

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

func (s *Service) AddWorkItemActivity(ctx context.Context, workItemID string, req WorkItemActivityRequest, actorID, actorName string) (Record, error) {
	allowed, err := s.repo.CanActOnWorkItem(ctx, workItemID, actorID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền thêm hoạt động cho đầu việc này")
	}
	normalized, err := validateActivityRequest(req)
	if err != nil {
		return nil, err
	}
	if normalized.ParentActivityID != "" {
		return nil, validationError("Phản hồi trực tiếp chỉ được tạo từ thảo luận công trình")
	}
	if err := s.requireActivityVisibility(ctx, normalized, actorID); err != nil {
		return nil, err
	}
	item, err := s.repo.GetWorkItemDetail(ctx, workItemID)
	if err != nil {
		return nil, err
	}
	if err := s.requireActivePipelineWorkItem(ctx, item); err != nil {
		return nil, err
	}
	normalized, err = s.validateActivityAttachments(ctx, cleanID(item["project_id"]), workItemID, normalized)
	if err != nil {
		return nil, err
	}
	activity, err := s.repo.AddWorkItemActivity(ctx, item, normalized, actorID, actorName)
	if err != nil {
		return nil, err
	}
	s.publishActivityEvent(activityEventFromRecord("activity_changed", activity, actorID))
	return activity, nil
}

func (s *Service) MarkActivityRead(ctx context.Context, activityID, actorID string) (Record, error) {
	activityID = cleanID(activityID)
	if !validActivityUUID(activityID) {
		return nil, validationError("Mã hoạt động không hợp lệ", Record{"activity_id": activityID})
	}
	profile, err := s.repo.WorkItemAccessProfile(ctx, actorID)
	if err != nil {
		return nil, err
	}
	visible, err := s.repo.ActivityVisibleToUser(ctx, activityID, profile)
	if err != nil {
		return nil, err
	}
	if !visible {
		return nil, forbiddenError("Bạn không có quyền xem hoạt động này")
	}
	readState, err := s.repo.MarkActivityRead(ctx, activityID, actorID)
	if err != nil {
		return nil, err
	}
	s.publishActivityEvent(ActivityStreamEvent{
		Type:       "activity_read",
		ProjectID:  cleanID(readState["project_id"]),
		ActivityID: activityID,
		ActorID:    actorID,
	})
	return readState, nil
}

func (s *Service) MarkProjectActivityRead(ctx context.Context, projectID, actorID string) (Record, error) {
	allowed, err := s.repo.CanViewProjectOperations(ctx, projectID, actorID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền cập nhật hoạt động của công trình này")
	}
	result, err := s.repo.MarkProjectActivityRead(ctx, projectID, actorID)
	if err != nil {
		return nil, err
	}
	s.publishActivityEvent(ActivityStreamEvent{Type: "activity_read", ProjectID: projectID, ActorID: actorID})
	return result, nil
}

func (s *Service) ResolveActivity(ctx context.Context, activityID string, req ActivityResolutionRequest, actorID, actorName string) (Record, error) {
	activityID = cleanID(activityID)
	if !validActivityUUID(activityID) {
		return nil, validationError("Mã hoạt động không hợp lệ", Record{"activity_id": activityID})
	}
	profile, err := s.repo.WorkItemAccessProfile(ctx, actorID)
	if err != nil {
		return nil, err
	}
	visible, err := s.repo.ActivityVisibleToUser(ctx, activityID, profile)
	if err != nil {
		return nil, err
	}
	if !visible {
		return nil, forbiddenError("Bạn không có quyền xem hoạt động này")
	}
	activity, err := s.repo.ActivityRecord(ctx, activityID)
	if err != nil {
		return nil, err
	}
	projectID := cleanID(activity["project_id"])
	workItemID := cleanID(activity["work_item_id"])
	if workItemID != "" {
		allowed, err := s.repo.CanActOnWorkItem(ctx, workItemID, actorID)
		if err != nil {
			return nil, err
		}
		if !allowed {
			return nil, forbiddenError("Bạn không có quyền xử lý hoạt động của đầu việc này")
		}
	} else {
		allowed, err := s.repo.CanViewProjectOperations(ctx, projectID, actorID)
		if err != nil {
			return nil, err
		}
		if !allowed {
			return nil, forbiddenError("Bạn không có quyền xử lý hoạt động của công trình này")
		}
		if !profile.IsManager && cleanID(activity["actor_id"]) != actorID {
			mentioned, err := s.repo.ActivityMentionedUser(ctx, activityID, actorID)
			if err != nil {
				return nil, err
			}
			if !mentioned {
				return nil, forbiddenError("Chỉ người tạo, người được nhắc hoặc quản lý công trình có thể xử lý hoạt động này")
			}
		}
	}
	activityType := strings.TrimSpace(fmt.Sprint(activity["activity_type"]))
	if activityType != "question" && activityType != "blocker" && activityType != "reminder" {
		return nil, validationError("Loại hoạt động này không có hành động hoàn tất", Record{"activity_type": activityType})
	}
	comment := strings.TrimSpace(req.Comment)
	resolved, err := s.repo.ResolveActivity(ctx, activity, comment, actorID, actorName)
	if err != nil {
		return nil, err
	}
	s.publishActivityEvent(activityEventFromRecord("activity_changed", resolved, actorID))
	return resolved, nil
}

func (s *Service) SetActivityReaction(ctx context.Context, activityID string, req ActivityReactionRequest, actorID string) (Record, error) {
	activityID = cleanID(activityID)
	if !validActivityUUID(activityID) {
		return nil, validationError("Mã hoạt động không hợp lệ", Record{"activity_id": activityID})
	}
	reaction := strings.TrimSpace(req.Reaction)
	if !activityReactions[reaction] {
		return nil, validationError("Phản ứng không hợp lệ", Record{"reaction": reaction})
	}
	profile, err := s.repo.WorkItemAccessProfile(ctx, actorID)
	if err != nil {
		return nil, err
	}
	visible, err := s.repo.ActivityVisibleToUser(ctx, activityID, profile)
	if err != nil {
		return nil, err
	}
	if !visible {
		return nil, forbiddenError("Bạn không có quyền xem hoạt động này")
	}
	activity, err := s.repo.ActivityRecord(ctx, activityID)
	if err != nil {
		return nil, err
	}
	if !userActivityTypes[strings.TrimSpace(fmt.Sprint(activity["activity_type"]))] {
		return nil, validationError("Không thể phản ứng với sự kiện hệ thống")
	}
	if cleanID(activity["withdrawn_at"]) != "" {
		return nil, validationError("Không thể phản ứng với tin nhắn đã được thu hồi")
	}
	result, err := s.repo.SetActivityReaction(ctx, activity, reaction, req.Active, actorID)
	if err != nil {
		return nil, err
	}
	s.publishActivityEvent(ActivityStreamEvent{
		Type:       "activity_changed",
		ProjectID:  cleanID(activity["project_id"]),
		ActivityID: activityID,
		ActorID:    actorID,
	})
	return result, nil
}

func (s *Service) UpdateActivityMessage(ctx context.Context, activityID string, req ActivityMessageUpdateRequest, actorID, actorName string) (Record, error) {
	activity, err := s.activityMessageForMutation(ctx, activityID, actorID)
	if err != nil {
		return nil, err
	}
	comment := strings.TrimSpace(req.Comment)
	if comment == "" {
		return nil, validationError("Nội dung tin nhắn là bắt buộc")
	}
	if comment == strings.TrimSpace(fmt.Sprint(activity["comment"])) {
		return nil, validationError("Nội dung tin nhắn chưa thay đổi")
	}
	updated, err := s.repo.UpdateActivityMessage(ctx, activity, comment, actorID, actorName)
	if err != nil {
		return nil, err
	}
	s.publishActivityEvent(activityEventFromRecord("activity_changed", updated, actorID))
	return updated, nil
}

func (s *Service) WithdrawActivityMessage(ctx context.Context, activityID, actorID, actorName string) (Record, error) {
	activity, err := s.activityMessageForMutation(ctx, activityID, actorID)
	if err != nil {
		return nil, err
	}
	withdrawn, err := s.repo.WithdrawActivityMessage(ctx, activity, actorID, actorName)
	if err != nil {
		return nil, err
	}
	s.publishActivityEvent(activityEventFromRecord("activity_changed", withdrawn, actorID))
	return withdrawn, nil
}

func (s *Service) activityMessageForMutation(ctx context.Context, activityID, actorID string) (Record, error) {
	activityID = cleanID(activityID)
	if !validActivityUUID(activityID) {
		return nil, validationError("Mã hoạt động không hợp lệ", Record{"activity_id": activityID})
	}
	profile, err := s.repo.WorkItemAccessProfile(ctx, actorID)
	if err != nil {
		return nil, err
	}
	visible, err := s.repo.ActivityVisibleToUser(ctx, activityID, profile)
	if err != nil {
		return nil, err
	}
	if !visible {
		return nil, forbiddenError("Bạn không có quyền xem hoạt động này")
	}
	activity, err := s.repo.ActivityRecord(ctx, activityID)
	if err != nil {
		return nil, err
	}
	if cleanID(activity["actor_id"]) != cleanID(actorID) {
		return nil, forbiddenError("Chỉ tác giả được chỉnh sửa hoặc thu hồi tin nhắn")
	}
	if strings.TrimSpace(fmt.Sprint(activity["activity_type"])) != "comment" {
		return nil, validationError("Chỉ tin nhắn ghi chú mới có thể chỉnh sửa hoặc thu hồi")
	}
	if cleanID(activity["withdrawn_at"]) != "" {
		return nil, validationError("Tin nhắn đã được thu hồi")
	}
	createdAt, err := time.Parse(time.RFC3339, cleanID(activity["created_at"]))
	if err != nil {
		return nil, err
	}
	if time.Since(createdAt) > activityMessageEditWindow {
		return nil, validationError("Thời hạn chỉnh sửa hoặc thu hồi tin nhắn đã kết thúc")
	}
	return activity, nil
}

func (s *Service) AddProjectActivity(ctx context.Context, projectID string, req WorkItemActivityRequest, actorID, actorName string) (Record, error) {
	allowed, err := s.repo.CanViewProjectOperations(ctx, projectID, actorID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền thêm hoạt động cho công trình này")
	}
	canMutate, err := s.repo.CanMutateProjectActivity(ctx, actorID, projectID)
	if err != nil {
		return nil, err
	}
	if !canMutate {
		return nil, forbiddenError("Chế độ đối soát Kế toán chỉ được xem hoạt động của công trình")
	}
	if strings.TrimSpace(req.ParentActivityID) != "" {
		req.ActivityType = "comment"
		req.Visibility = "project"
		req.Attachments = nil
		req.Metadata = Record{}
	}
	normalized, err := validateActivityRequest(req)
	if err != nil {
		return nil, err
	}
	normalized, err = s.normalizeActivityReply(ctx, projectID, normalized, actorID)
	if err != nil {
		return nil, err
	}
	if err := s.requireActivityVisibility(ctx, normalized, actorID); err != nil {
		return nil, err
	}
	normalized, err = s.normalizeProjectActivityTarget(ctx, projectID, normalized, actorID)
	if err != nil {
		return nil, err
	}
	attachmentWorkItemID := ""
	if normalized.ScopeType == "work_item" {
		attachmentWorkItemID = normalized.ScopeID
	} else if normalized.WorkItemID != "" {
		attachmentWorkItemID = normalized.WorkItemID
	}
	normalized, err = s.validateActivityAttachments(ctx, projectID, attachmentWorkItemID, normalized)
	if err != nil {
		return nil, err
	}
	activity, err := s.repo.AddProjectActivity(ctx, projectID, normalized, actorID, actorName)
	if err != nil {
		return nil, err
	}
	s.publishActivityEvent(activityEventFromRecord("activity_changed", activity, actorID))
	return activity, nil
}

func (s *Service) normalizeActivityReply(ctx context.Context, projectID string, req WorkItemActivityRequest, actorID string) (WorkItemActivityRequest, error) {
	parentActivityID := cleanID(req.ParentActivityID)
	if parentActivityID == "" {
		req.ParentActivityID = ""
		return req, nil
	}
	if !validActivityUUID(parentActivityID) {
		return req, validationError("Mã trao đổi được trả lời không hợp lệ", Record{"parent_activity_id": parentActivityID})
	}
	profile, err := s.repo.WorkItemAccessProfile(ctx, actorID)
	if err != nil {
		return req, err
	}
	visible, err := s.repo.ActivityVisibleToUser(ctx, parentActivityID, profile)
	if err != nil {
		return req, err
	}
	if !visible {
		return req, forbiddenError("Bạn không có quyền trả lời trao đổi này")
	}
	parent, err := s.repo.ActivityRecord(ctx, parentActivityID)
	if err != nil {
		return req, err
	}
	if cleanID(parent["project_id"]) != cleanID(projectID) {
		return req, validationError("Trao đổi được trả lời không thuộc công trình này")
	}
	if !userActivityTypes[strings.TrimSpace(fmt.Sprint(parent["activity_type"]))] {
		return req, validationError("Không thể trả lời sự kiện hệ thống")
	}
	if cleanID(parent["withdrawn_at"]) != "" {
		return req, validationError("Không thể trả lời tin nhắn đã được thu hồi")
	}
	req.ParentActivityID = parentActivityID
	req.ActivityType = "comment"
	req.Visibility = strings.TrimSpace(fmt.Sprint(parent["visibility"]))
	req.ScopeType = strings.TrimSpace(fmt.Sprint(parent["scope_type"]))
	req.ScopeID = cleanID(parent["scope_id"])
	req.WorkItemID = cleanID(parent["work_item_id"])
	req.Attachments = nil
	req.Metadata = Record{}
	parentActorID := cleanID(parent["actor_id"])
	if parentActorID != "" && parentActorID != actorID {
		req.Mentions = append(req.Mentions, ActivityMentionRequest{Type: "user", ID: parentActorID})
	}
	mentions, err := normalizeActivityMentions(req.Mentions)
	if err != nil {
		return req, err
	}
	req.Mentions = mentions
	if len(mentions) > 0 {
		req.Metadata["mentions"] = mentions
	}
	return req, nil
}

var userActivityTypes = map[string]bool{
	"comment":   true,
	"status":    true,
	"blocker":   true,
	"question":  true,
	"decision":  true,
	"reminder":  true,
	"file_note": true,
}

var activityFilterTypes = map[string]bool{
	"created":            true,
	"started":            true,
	"completed":          true,
	"blocked":            true,
	"blocker_resolved":   true,
	"status_changed":     true,
	"comment":            true,
	"status":             true,
	"blocker":            true,
	"question":           true,
	"decision":           true,
	"reminder":           true,
	"file_note":          true,
	"system_event":       true,
	"evidence_added":     true,
	"evidence_accepted":  true,
	"evidence_rejected":  true,
	"review_requested":   true,
	"approval_requested": true,
	"approved":           true,
	"rejected":           true,
	"changes_requested":  true,
}

var activityVisibilities = map[string]bool{
	"internal": true,
	"project":  true,
	"client":   true,
}

var activityTargetScopes = map[string]bool{
	"project":             true,
	"work_item":           true,
	"boq_item":            true,
	"procurement_request": true,
	"stock_movement":      true,
	"acceptance":          true,
	"rework":              true,
	"handover":            true,
	"payable":             true,
	"receivable":          true,
	"payment":             true,
}

var activityAttentionQueues = map[string]bool{
	"all":       true,
	"mentions":  true,
	"questions": true,
	"blockers":  true,
	"reminders": true,
	"unread":    true,
}

var activityAttachmentTypes = map[string]bool{
	"evidence": true,
	"document": true,
}

var activityReactions = map[string]bool{
	"👍": true,
	"👀": true,
	"🙏": true,
	"🎉": true,
}

func validateActivityRequest(req WorkItemActivityRequest) (WorkItemActivityRequest, error) {
	activityType := strings.TrimSpace(req.ActivityType)
	if activityType == "" {
		activityType = "comment"
	}
	if activityType == "system_event" {
		return req, validationError("Sự kiện hệ thống chỉ được tạo bởi quy trình")
	}
	if !userActivityTypes[activityType] {
		return req, validationError("Loại hoạt động không hợp lệ", Record{"activity_type": activityType})
	}

	comment := strings.TrimSpace(req.Comment)
	if comment == "" {
		return req, validationError("Nội dung hoạt động là bắt buộc")
	}

	visibility := strings.TrimSpace(req.Visibility)
	if visibility == "" {
		visibility = "project"
	}
	if !activityVisibilities[visibility] {
		return req, validationError("Phạm vi hiển thị hoạt động không hợp lệ", Record{"visibility": visibility})
	}

	metadata := req.Metadata
	if metadata == nil {
		metadata = Record{}
	}

	if activityType == "reminder" {
		dueAt := activityMetadataText(metadata, "due_at")
		if dueAt == "" {
			return req, validationError("Nhắc việc cần ngày nhắc lại", Record{"field": "metadata.due_at"})
		}
		if !validActivityDueDate(dueAt) {
			return req, validationError("Ngày nhắc lại không hợp lệ", Record{"field": "metadata.due_at"})
		}
		metadata["due_at"] = dueAt
	}
	if activityType == "blocker" {
		if dueAt := activityMetadataText(metadata, "due_at"); dueAt != "" && !validActivityDueDate(dueAt) {
			return req, validationError("Hạn xử lý điểm chặn không hợp lệ", Record{"field": "metadata.due_at"})
		}
	}
	req.ActivityType = activityType
	req.Comment = comment
	req.Visibility = visibility
	req.ScopeType = strings.TrimSpace(req.ScopeType)
	req.ScopeID = cleanID(req.ScopeID)
	req.WorkItemID = cleanID(req.WorkItemID)
	req.ParentActivityID = cleanID(req.ParentActivityID)
	attachments, err := normalizeActivityAttachments(req.Attachments, metadata)
	if err != nil {
		return req, err
	}
	req.Attachments = attachments
	if len(attachments) > 0 {
		metadata["attachments"] = attachments
		for _, attachment := range attachments {
			if attachment.Type == "evidence" && activityMetadataText(metadata, "evidence_id") == "" {
				metadata["evidence_id"] = attachment.ID
			}
			if attachment.Type == "document" && activityMetadataText(metadata, "document_id") == "" {
				metadata["document_id"] = attachment.ID
			}
		}
	}
	if activityType == "file_note" && len(attachments) == 0 {
		return req, validationError("Ghi chú tệp cần minh chứng hoặc tài liệu", Record{"fields": []string{"attachments", "metadata.evidence_id", "metadata.document_id"}})
	}
	mentions, err := normalizeActivityMentions(req.Mentions)
	if err != nil {
		return req, err
	}
	req.Mentions = mentions
	if len(mentions) > 0 {
		metadata["mentions"] = mentions
	}
	req.Metadata = metadata
	return req, nil
}

func normalizeActivityAttachments(input []ActivityAttachmentRequest, metadata Record) ([]ActivityAttachmentRequest, error) {
	out := make([]ActivityAttachmentRequest, 0, len(input))
	seen := map[string]bool{}
	add := func(attachmentType, attachmentID, label string) error {
		attachmentType = strings.TrimSpace(strings.ToLower(attachmentType))
		attachmentID = cleanID(attachmentID)
		label = strings.TrimSpace(label)
		if attachmentType == "" && attachmentID == "" {
			return nil
		}
		if !activityAttachmentTypes[attachmentType] {
			return validationError("Loại tệp đính kèm không hợp lệ", Record{"attachment_type": attachmentType})
		}
		if attachmentID == "" || !validActivityUUID(attachmentID) {
			return validationError("ID tệp đính kèm không hợp lệ", Record{"attachment_type": attachmentType, "id": attachmentID})
		}
		key := attachmentType + ":" + strings.ToLower(attachmentID)
		if seen[key] {
			return nil
		}
		seen[key] = true
		out = append(out, ActivityAttachmentRequest{Type: attachmentType, ID: attachmentID, Label: label})
		return nil
	}
	for _, attachment := range input {
		if err := add(attachment.Type, attachment.ID, attachment.Label); err != nil {
			return nil, err
		}
	}
	if rawAttachments, ok := metadata["attachments"]; ok {
		for _, rawAttachment := range anySlice(rawAttachments) {
			if record, ok := rawAttachment.(map[string]any); ok {
				if err := add(fmt.Sprint(record["type"]), fmt.Sprint(record["id"]), fmt.Sprint(record["label"])); err != nil {
					return nil, err
				}
				continue
			}
			if record, ok := rawAttachment.(Record); ok {
				if err := add(fmt.Sprint(record["type"]), fmt.Sprint(record["id"]), fmt.Sprint(record["label"])); err != nil {
					return nil, err
				}
			}
		}
	}
	if evidenceID := activityMetadataText(metadata, "evidence_id"); evidenceID != "" {
		if err := add("evidence", evidenceID, ""); err != nil {
			return nil, err
		}
	}
	if documentID := activityMetadataText(metadata, "document_id"); documentID != "" {
		if err := add("document", documentID, ""); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (s *Service) validateActivityAttachments(ctx context.Context, projectID, workItemID string, req WorkItemActivityRequest) (WorkItemActivityRequest, error) {
	if len(req.Attachments) == 0 {
		return req, nil
	}
	for _, attachment := range req.Attachments {
		switch attachment.Type {
		case "evidence":
			evidence, err := s.repo.GetEvidenceDetail(ctx, attachment.ID)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return req, validationError("Minh chứng đính kèm không tồn tại", Record{"evidence_id": attachment.ID})
				}
				return req, err
			}
			if cleanID(evidence["project_id"]) != projectID {
				return req, validationError("Minh chứng đính kèm không thuộc công trình này", Record{"evidence_id": attachment.ID})
			}
			if workItemID != "" && cleanID(evidence["work_item_id"]) != workItemID {
				return req, validationError("Minh chứng đính kèm không thuộc đầu việc này", Record{"evidence_id": attachment.ID})
			}
			if req.Visibility == "client" && fmt.Sprint(evidence["visibility"]) != "client" {
				return req, validationError("Hoạt động hiển thị cho chủ đầu tư chỉ được gắn minh chứng cùng phạm vi hiển thị", Record{"evidence_id": attachment.ID})
			}
		case "document":
			document, err := s.repo.GetDocumentAttachment(ctx, attachment.ID)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return req, validationError("Tài liệu đính kèm không tồn tại", Record{"document_id": attachment.ID})
				}
				return req, err
			}
			if cleanID(document["project_id"]) != projectID {
				return req, validationError("Tài liệu đính kèm không thuộc công trình này", Record{"document_id": attachment.ID})
			}
			if req.Visibility == "client" {
				return req, validationError("Tài liệu đính kèm chưa có phạm vi hiển thị; hãy gắn minh chứng hiển thị cho chủ đầu tư thay thế", Record{"document_id": attachment.ID})
			}
		}
	}
	return req, nil
}

func normalizeActivityMentions(input []ActivityMentionRequest) ([]ActivityMentionRequest, error) {
	out := make([]ActivityMentionRequest, 0, len(input))
	seen := map[string]bool{}
	for _, mention := range input {
		mentionType := strings.TrimSpace(strings.ToLower(mention.Type))
		mentionID := cleanID(mention.ID)
		label := strings.TrimSpace(mention.Label)
		if mentionType == "" && mentionID == "" {
			continue
		}
		if mentionType == "all" {
			if !strings.EqualFold(mentionID, "all") {
				return nil, validationError("Nhắc toàn bộ thành viên cần mã all", Record{"mention_type": mentionType})
			}
			mentionID, label = "all", "all"
		} else if mentionType != "user" && mentionType != "staff" && mentionType != "role" {
			return nil, validationError("Loại nhắc đến không hợp lệ", Record{"mention_type": mention.Type})
		}
		if mentionID == "" {
			return nil, validationError("Nhắc đến cần ID hoặc nhãn vai trò", Record{"mention_type": mentionType})
		}
		if mentionType != "role" && mentionType != "all" && !validActivityUUID(mentionID) {
			return nil, validationError("Người dùng/nhân sự được nhắc đến không hợp lệ", Record{"mention_type": mentionType, "id": mentionID})
		}
		key := mentionType + ":" + strings.ToLower(mentionID)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, ActivityMentionRequest{Type: mentionType, ID: mentionID, Label: label})
	}
	return out, nil
}

func (s *Service) normalizeProjectActivityTarget(ctx context.Context, projectID string, req WorkItemActivityRequest, actorID string) (WorkItemActivityRequest, error) {
	scopeType := strings.TrimSpace(req.ScopeType)
	if scopeType == "" {
		scopeType = "project"
	}
	if !activityTargetScopes[scopeType] {
		return req, validationError("Phạm vi hoạt động không hợp lệ", Record{"scope_type": scopeType})
	}
	if scopeType == "project" {
		req.ScopeType = "project"
		req.ScopeID = projectID
		req.WorkItemID = ""
		return req, nil
	}

	scopeID := cleanID(req.ScopeID)
	if scopeID == "" {
		return req, validationError("Phạm vi hoạt động cần bản ghi đích", Record{"scope_type": scopeType})
	}
	if !validActivityUUID(scopeID) {
		return req, validationError("scope_id không hợp lệ", Record{"scope_id": scopeID})
	}
	if scopeType == "work_item" {
		allowed, err := s.repo.CanActOnWorkItem(ctx, scopeID, actorID)
		if err != nil {
			return req, err
		}
		if !allowed {
			return req, forbiddenError("Bạn không có quyền thêm hoạt động cho đầu việc này")
		}
		item, err := s.repo.GetWorkItemDetail(ctx, scopeID)
		if err != nil {
			return req, err
		}
		if cleanID(item["project_id"]) != projectID {
			return req, validationError("Đầu việc không thuộc công trình này", Record{"work_item_id": scopeID})
		}
		req.ScopeType = "work_item"
		req.ScopeID = scopeID
		req.WorkItemID = scopeID
		return req, nil
	}

	exists, err := s.repo.ProjectActivityTargetExists(ctx, projectID, scopeType, scopeID)
	if err != nil {
		return req, err
	}
	if !exists {
		return req, validationError("Bản ghi hoạt động không thuộc công trình này", Record{"scope_type": scopeType, "scope_id": scopeID})
	}
	if req.WorkItemID != "" {
		if !validActivityUUID(req.WorkItemID) {
			return req, validationError("Mã đầu việc không hợp lệ", Record{"work_item_id": req.WorkItemID})
		}
		allowed, err := s.repo.CanActOnWorkItem(ctx, req.WorkItemID, actorID)
		if err != nil {
			return req, err
		}
		if !allowed {
			return req, forbiddenError("Bạn không có quyền liên kết hoạt động với đầu việc này")
		}
		item, err := s.repo.GetWorkItemDetail(ctx, req.WorkItemID)
		if err != nil {
			return req, err
		}
		if cleanID(item["project_id"]) != projectID {
			return req, validationError("Đầu việc liên kết không thuộc công trình này", Record{"work_item_id": req.WorkItemID})
		}
	}
	req.ScopeType = scopeType
	req.ScopeID = scopeID
	return req, nil
}

func validActivityUUID(value string) bool {
	_, err := uuid.Parse(value)
	return err == nil
}

func activityMetadataText(metadata Record, keys ...string) string {
	for _, key := range keys {
		text := strings.TrimSpace(fmt.Sprint(metadata[key]))
		if text != "" && text != "<nil>" {
			return text
		}
	}
	return ""
}

func validActivityDueDate(value string) bool {
	for _, layout := range []string{time.RFC3339, "2006-01-02"} {
		if _, err := time.Parse(layout, value); err == nil {
			return true
		}
	}
	return false
}

func (s *Service) requireActivityVisibility(ctx context.Context, req WorkItemActivityRequest, actorID string) error {
	profile, err := s.repo.WorkItemAccessProfile(ctx, actorID)
	if err != nil {
		return err
	}
	if profileIsClient(profile) && !profile.IsManager && req.Visibility != "client" {
		return validationError("Chủ đầu tư chỉ được tạo hoạt động hiển thị cho chủ đầu tư", Record{"visibility": req.Visibility})
	}
	return nil
}
