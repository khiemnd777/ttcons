package core

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

func normalizeProjectDiscussionFilter(filter ProjectDiscussionFilter) (ProjectDiscussionFilter, int, error) {
	filter.ProjectID = cleanID(filter.ProjectID)
	if filter.ProjectID != "" && !validActivityUUID(filter.ProjectID) {
		return filter, 0, validationError("project_id không hợp lệ", Record{"project_id": filter.ProjectID})
	}
	filter.Search = strings.TrimSpace(filter.Search)
	if len([]rune(filter.Search)) > 200 {
		return filter, 0, validationError("Từ khóa tìm kiếm không được vượt quá 200 ký tự", nil)
	}
	if filter.Limit == 0 {
		filter.Limit = 20
	}
	if filter.Limit < 1 || filter.Limit > 50 {
		return filter, 0, validationError("limit phải nằm trong khoảng 1 đến 50", Record{"limit": filter.Limit})
	}
	offset := 0
	if strings.TrimSpace(filter.Cursor) != "" {
		parsed, err := parseProjectDiscussionCursor(filter.Cursor)
		if err != nil {
			return filter, 0, validationError("cursor không hợp lệ", Record{"cursor": filter.Cursor})
		}
		offset = parsed
	}
	return filter, offset, nil
}

func encodeProjectDiscussionCursor(offset int) string {
	if offset < 0 {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf("project-offset:%d", offset)))
}

func parseProjectDiscussionCursor(cursor string) (int, error) {
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(cursor))
	if err != nil {
		return 0, err
	}
	parts := strings.SplitN(string(raw), ":", 2)
	if len(parts) != 2 || parts[0] != "project-offset" {
		return 0, errors.New("invalid cursor")
	}
	offset, err := strconv.Atoi(parts[1])
	if err != nil || offset < 0 {
		return 0, errors.New("invalid cursor")
	}
	return offset, nil
}

func (s *Service) MyProjectDiscussions(ctx context.Context, filter ProjectDiscussionFilter, actorID string) (Record, error) {
	filter, offset, err := normalizeProjectDiscussionFilter(filter)
	if err != nil {
		return nil, err
	}
	profile, err := s.repo.WorkItemAccessProfile(ctx, actorID)
	if err != nil {
		return nil, err
	}
	items, total, unreadTotal, err := s.repo.MyProjectDiscussions(ctx, profile, filter, offset)
	if err != nil {
		return nil, err
	}
	hasMore := offset+len(items) < total
	nextCursor := ""
	if hasMore {
		nextCursor = encodeProjectDiscussionCursor(offset + len(items))
	}
	for _, item := range items {
		delete(item, "filtered_total")
		delete(item, "unread_total")
	}
	return Record{
		"items":        items,
		"unread_total": unreadTotal,
		"total":        total,
		"has_more":     hasMore,
		"next_cursor":  nextCursor,
	}, nil
}

func (s *Service) MarkProjectDiscussionReadThrough(ctx context.Context, projectID string, req ProjectDiscussionReadRequest, actorID string) (Record, error) {
	return s.markProjectActivityReadThrough(ctx, projectID, req, actorID, false)
}

func (s *Service) markProjectActivityReadThrough(ctx context.Context, projectID string, req ProjectDiscussionReadRequest, actorID string, system bool) (Record, error) {
	projectID = cleanID(projectID)
	if !validActivityUUID(projectID) {
		return nil, validationError("Mã công trình không hợp lệ", Record{"project_id": projectID})
	}
	activityID := cleanID(req.ActivityID)
	if !validActivityUUID(activityID) {
		return nil, validationError("Mã hoạt động không hợp lệ", Record{"activity_id": activityID})
	}
	allowed, err := s.repo.CanViewProjectOperations(ctx, projectID, actorID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền cập nhật trao đổi của công trình này")
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
	if cleanID(activity["project_id"]) != projectID {
		return nil, validationError("Hoạt động không thuộc công trình đã chọn", Record{"activity_id": activityID})
	}
	activityType := strings.TrimSpace(fmt.Sprint(activity["activity_type"]))
	isDiscussion := statusIn(activityType, "comment", "status", "blocker", "question", "decision", "reminder", "file_note")
	if isDiscussion == system || activity["withdrawn_at"] != nil {
		return nil, validationError("Hoạt động không thuộc nhóm đã chọn hoặc đã được thu hồi", Record{"activity_id": activityID})
	}
	result, err := s.repo.markProjectActivityReadThrough(ctx, projectID, actorID, activityID, system)
	if err != nil {
		return nil, err
	}
	s.publishActivityEvent(ActivityStreamEvent{Type: "activity_read", ProjectID: projectID, ActivityID: activityID, ActorID: actorID})
	return result, nil
}
