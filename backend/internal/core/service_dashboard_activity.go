package core

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type dashboardActivityCursor struct {
	Version    int    `json:"v"`
	FilterHash string `json:"filter"`
	AsOf       string `json:"as_of"`
	CreatedAt  string `json:"created_at"`
	ID         string `json:"id"`
}

type dashboardActivityQuery struct {
	DashboardActivitiesRequest
	From       time.Time
	AsOf       time.Time
	Before     time.Time
	BeforeID   string
	FilterHash string
}

func normalizeDashboardActivities(req DashboardActivitiesRequest, actorID string, now time.Time) (dashboardActivityQuery, error) {
	req.Range = strings.TrimSpace(req.Range)
	if req.Range == "" {
		req.Range = "7d"
	}
	if !statusIn(req.Range, "today", "7d", "30d", "90d", "all") {
		return dashboardActivityQuery{}, validationError("Khoảng thời gian hoạt động không hợp lệ")
	}
	req.Search = strings.TrimSpace(req.Search)
	if len([]rune(req.Search)) > 200 {
		return dashboardActivityQuery{}, validationError("Từ khóa không được vượt quá 200 ký tự")
	}
	req.ActivityType = strings.TrimSpace(req.ActivityType)
	if req.ActivityType != "" && !statusIn(req.ActivityType, "created", "started", "completed", "blocked", "blocker_resolved", "status_changed", "system_event", "evidence_added", "evidence_accepted", "evidence_rejected", "review_requested", "approval_requested", "approved", "rejected", "changes_requested") {
		return dashboardActivityQuery{}, validationError("Loại hoạt động không hợp lệ")
	}
	if req.Limit == 0 {
		req.Limit = 20
	}
	if req.Limit < 1 || req.Limit > 50 {
		return dashboardActivityQuery{}, validationError("limit phải từ 1 đến 50")
	}
	filterJSON, _ := json.Marshal([]any{actorID, req.Range, req.Search, req.ActivityType, req.Limit})
	q := dashboardActivityQuery{DashboardActivitiesRequest: req, AsOf: now.UTC().Truncate(time.Microsecond), FilterHash: fmt.Sprintf("%x", sha256.Sum256(filterJSON))}
	if req.Cursor != "" {
		invalid := func() (dashboardActivityQuery, error) {
			return q, validationError("Trang hoạt động không còn hợp lệ. Vui lòng làm mới danh sách.")
		}
		if len(req.Cursor) > 2048 {
			return invalid()
		}
		raw, err := base64.RawURLEncoding.DecodeString(req.Cursor)
		if err != nil {
			return invalid()
		}
		var cursor dashboardActivityCursor
		if err := json.Unmarshal(raw, &cursor); err != nil || cursor.Version != 1 || cursor.FilterHash != q.FilterHash || !validActivityUUID(cursor.ID) {
			return invalid()
		}
		asOf, err := time.Parse(time.RFC3339Nano, cursor.AsOf)
		if err != nil || asOf.Year() < 1 || asOf.After(now) {
			return invalid()
		}
		before, err := time.Parse(time.RFC3339Nano, cursor.CreatedAt)
		if err != nil || before.Year() < 1 || before.After(asOf) {
			return invalid()
		}
		q.AsOf, q.Before, q.BeforeID = asOf, before, cursor.ID
	}
	if req.Range != "all" {
		days := map[string]int{"today": 1, "7d": 7, "30d": 30, "90d": 90}[req.Range]
		local := q.AsOf.In(dashboardLocation)
		q.From = time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, dashboardLocation).AddDate(0, 0, 1-days)
		if !q.Before.IsZero() && q.Before.Before(q.From) {
			return q, validationError("Hoạt động nằm ngoài khoảng thời gian đã chọn")
		}
	}
	return q, nil
}

func encodeDashboardActivityCursor(q dashboardActivityQuery, last DashboardActivityItem) string {
	raw, _ := json.Marshal(dashboardActivityCursor{Version: 1, FilterHash: q.FilterHash, AsOf: q.AsOf.Format(time.RFC3339Nano), CreatedAt: last.CreatedAt, ID: last.ID})
	return base64.RawURLEncoding.EncodeToString(raw)
}

func (s *Service) DashboardActivities(ctx context.Context, req DashboardActivitiesRequest, actorID string) (DashboardActivitiesPage, error) {
	q, err := normalizeDashboardActivities(req, actorID, time.Now())
	if err != nil {
		return DashboardActivitiesPage{}, err
	}
	profile, err := s.repo.WorkItemAccessProfile(ctx, actorID)
	if err != nil {
		return DashboardActivitiesPage{}, err
	}
	canView := profile.IsAdmin || profile.CanReconcileAllProjects
	if !canView {
		canView, err = s.repo.UserHasAnyPermission(ctx, actorID, projectOperationsViewPermissions)
		if err != nil {
			return DashboardActivitiesPage{}, err
		}
	}
	if !canView {
		return DashboardActivitiesPage{Items: []DashboardActivityItem{}, AsOf: q.AsOf.Format(time.RFC3339Nano)}, nil
	}
	page, err := s.repo.DashboardActivities(ctx, profile, q)
	if err != nil {
		return page, err
	}
	page.AsOf = q.AsOf.Format(time.RFC3339Nano)
	page.HasMore = len(page.Items) > q.Limit
	if page.HasMore {
		page.Items = page.Items[:q.Limit]
		page.NextCursor = encodeDashboardActivityCursor(q, page.Items[len(page.Items)-1])
	}
	return page, nil
}
