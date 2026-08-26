package core

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"
)

var notificationKeyPattern = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)+$`)
var notificationCategoryPattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

func normalizeNotificationFilter(filter NotificationFilter) (NotificationFilter, error) {
	filter.State = strings.ToLower(strings.TrimSpace(filter.State))
	if filter.State == "" {
		filter.State = "all"
	}
	if filter.State != "all" && filter.State != "unread" && filter.State != "action_required" && filter.State != "archived" {
		return filter, validationError("Trạng thái thông báo không hợp lệ", Record{"state": filter.State})
	}
	filter.EventKey = strings.ToLower(strings.TrimSpace(filter.EventKey))
	if filter.EventKey != "" && !notificationKeyPattern.MatchString(filter.EventKey) {
		return filter, validationError("Loại thông báo không hợp lệ", Record{"event_key": filter.EventKey})
	}
	filter.Category = strings.ToLower(strings.TrimSpace(filter.Category))
	if filter.Category != "" && !notificationCategoryPattern.MatchString(filter.Category) {
		return filter, validationError("Nhóm thông báo không hợp lệ", Record{"category": filter.Category})
	}
	filter.ProjectID = cleanID(filter.ProjectID)
	if filter.ProjectID != "" && !validActivityUUID(filter.ProjectID) {
		return filter, validationError("Công trình lọc thông báo không hợp lệ", Record{"project_id": filter.ProjectID})
	}
	filter.Search = strings.TrimSpace(filter.Search)
	if len([]rune(filter.Search)) > 200 {
		return filter, validationError("Từ khóa thông báo không được vượt quá 200 ký tự", Record{"search": "too_long"})
	}
	filter.FromDate = strings.TrimSpace(filter.FromDate)
	filter.ToDate = strings.TrimSpace(filter.ToDate)
	for key, value := range map[string]string{"from_date": filter.FromDate, "to_date": filter.ToDate} {
		if value != "" {
			if _, err := time.Parse("2006-01-02", value); err != nil {
				return filter, validationError("Ngày lọc thông báo không hợp lệ", Record{key: value})
			}
		}
	}
	if filter.FromDate != "" && filter.ToDate != "" && filter.FromDate > filter.ToDate {
		return filter, validationError("Khoảng ngày lọc thông báo không hợp lệ", Record{"from_date": filter.FromDate, "to_date": filter.ToDate})
	}
	filter.Cursor = strings.TrimSpace(filter.Cursor)
	if filter.Limit == 0 {
		filter.Limit = 20
	}
	if filter.Limit < 1 || filter.Limit > 50 {
		return filter, validationError("limit thông báo phải từ 1 đến 50", Record{"limit": filter.Limit})
	}
	return filter, nil
}

func (s *Service) UserNotifications(ctx context.Context, filter NotificationFilter, actorID string) (NotificationPage, error) {
	normalized, err := normalizeNotificationFilter(filter)
	if err != nil {
		return NotificationPage{}, err
	}
	return s.repo.UserNotifications(ctx, actorID, normalized)
}

func (s *Service) UserNotificationSummary(ctx context.Context, actorID string) (NotificationSummary, error) {
	return s.repo.UserNotificationSummary(ctx, actorID)
}

func (s *Service) MarkUserNotificationRead(ctx context.Context, notificationID, actorID string) (Record, error) {
	notificationID = cleanID(notificationID)
	if !validActivityUUID(notificationID) {
		return nil, validationError("Thông báo không hợp lệ", Record{"notification_id": notificationID})
	}
	row, err := s.repo.MarkUserNotificationRead(ctx, notificationID, actorID)
	if err != nil {
		return nil, err
	}
	s.publishPersonalInvalidation("notification_changed", actorID, "")
	return row, nil
}

func (s *Service) MarkAllUserNotificationsRead(ctx context.Context, actorID string) (Record, error) {
	row, err := s.repo.MarkAllUserNotificationsRead(ctx, actorID)
	if err != nil {
		return nil, err
	}
	s.publishPersonalInvalidation("notification_changed", actorID, "")
	return row, nil
}

func (s *Service) ArchiveUserNotification(ctx context.Context, notificationID, actorID string) (Record, error) {
	notificationID = cleanID(notificationID)
	if !validActivityUUID(notificationID) {
		return nil, validationError("Thông báo không hợp lệ", Record{"notification_id": notificationID})
	}
	row, err := s.repo.ArchiveUserNotification(ctx, notificationID, actorID)
	if err != nil {
		return nil, err
	}
	s.publishPersonalInvalidation("notification_changed", actorID, "")
	return row, nil
}

func (s *Service) NotificationCatalog(ctx context.Context) ([]NotificationCatalogItem, error) {
	return s.repo.NotificationCatalog(ctx)
}

func (s *Service) NotificationProjects(ctx context.Context, actorID string) ([]Record, error) {
	return s.repo.NotificationProjects(ctx, actorID)
}

func (s *Service) NotificationPreferences(ctx context.Context, actorID string) ([]NotificationPreference, error) {
	return s.repo.NotificationPreferences(ctx, actorID)
}

func (s *Service) SaveNotificationPreference(ctx context.Context, actorID string, input NotificationPreference) (NotificationPreference, error) {
	input.CategoryKey = strings.ToLower(strings.TrimSpace(input.CategoryKey))
	input.EventKey = strings.ToLower(strings.TrimSpace(input.EventKey))
	input.DigestMode = strings.ToLower(strings.TrimSpace(input.DigestMode))
	if input.CategoryKey == "" {
		input.CategoryKey = "*"
	}
	if input.EventKey == "" {
		input.EventKey = "*"
	}
	if input.CategoryKey != "*" && !notificationCategoryPattern.MatchString(input.CategoryKey) {
		return input, validationError("Nhóm thông báo không hợp lệ", Record{"category_key": input.CategoryKey})
	}
	if input.EventKey != "*" && !notificationKeyPattern.MatchString(input.EventKey) {
		return input, validationError("Loại thông báo không hợp lệ", Record{"event_key": input.EventKey})
	}
	if !statusIn(input.DigestMode, "immediate", "daily", "weekly", "off") {
		return input, validationError("Chế độ tổng hợp thông báo không hợp lệ", Record{"digest_mode": input.DigestMode})
	}
	required, err := s.repo.NotificationPreferenceTargetsRequiredEvent(ctx, input.CategoryKey, input.EventKey)
	if err != nil {
		return input, err
	}
	if required && (!input.InApp || input.DigestMode == "off") {
		return input, validationError("Thông báo bắt buộc phải được giữ trên App", Record{"event_key": input.EventKey, "category_key": input.CategoryKey})
	}
	if input.QuietHours == nil {
		input.QuietHours = map[string]any{}
	}
	row, err := s.repo.SaveNotificationPreference(ctx, actorID, input)
	if err != nil {
		return input, fmt.Errorf("save notification preference: %w", err)
	}
	return row, nil
}
