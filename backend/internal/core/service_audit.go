package core

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"
)

func (s *Service) AuditLogs(ctx context.Context, projectID, actorID string, filter AuditLogFilter) (Record, error) {
	filter, err := normalizeAuditLogFilter(filter)
	if err != nil {
		return nil, err
	}

	fromSQL := "FROM audit_logs al JOIN projects p ON p.id=al.project_id LEFT JOIN users audit_actor ON audit_actor.id=al.actor_id"
	scopeConditions := make([]string, 0, 3)
	scopeArgs := make([]any, 0, 3)
	if projectID != "" {
		scopeArgs = append(scopeArgs, projectID)
		scopeConditions = append(scopeConditions, "p.id=$1")
	} else {
		scopeSQL, accessArgs, accessErr := s.repo.ProjectAccessFilter(ctx, actorID, "p", 1)
		if accessErr != nil {
			return nil, accessErr
		}
		scopeArgs = append(scopeArgs, accessArgs...)
		scopeConditions = append(scopeConditions, "p.status <> 'cancelled'", scopeSQL)
	}

	conditions := append([]string(nil), scopeConditions...)
	args := append([]any(nil), scopeArgs...)
	addCondition := func(format string, value any) {
		args = append(args, value)
		conditions = append(conditions, fmt.Sprintf(format, len(args)))
	}
	if projectID == "" && filter.ProjectID != "" {
		addCondition("p.id=$%d", filter.ProjectID)
	}
	if filter.RecordID != "" {
		addCondition("al.id=$%d", filter.RecordID)
	}
	if filter.EventType != "" {
		addCondition("al.event_type=$%d", filter.EventType)
	}
	if filter.TargetType != "" {
		addCondition("al.target_type=$%d", filter.TargetType)
	}
	if filter.ActorName != "" {
		addCondition("LOWER(COALESCE(al.actor_name,''))=LOWER($%d)", filter.ActorName)
	}
	if filter.DateFrom != "" {
		addCondition("al.created_at >= $%d::date", filter.DateFrom)
	}
	if filter.DateTo != "" {
		addCondition("al.created_at < ($%d::date + INTERVAL '1 day')", filter.DateTo)
	}
	if filter.Search != "" {
		args = append(args, "%"+filter.Search+"%")
		placeholder := len(args)
		conditions = append(conditions, fmt.Sprintf(`(
			al.description ILIKE $%[1]d OR al.event_type ILIKE $%[1]d OR
			COALESCE(al.actor_name,'') ILIKE $%[1]d OR COALESCE(al.target_type,'') ILIKE $%[1]d OR
			COALESCE(al.target_id::text,'') ILIKE $%[1]d OR al.id::text ILIKE $%[1]d OR
			p.code ILIKE $%[1]d OR p.name ILIKE $%[1]d
		)`, placeholder))
	}

	whereSQL := strings.Join(conditions, " AND ")
	countRow, err := s.repo.QueryOne(ctx, fmt.Sprintf("SELECT COUNT(*) AS total %s WHERE %s", fromSQL, whereSQL), args...)
	if err != nil {
		return nil, err
	}
	total := int(number(countRow["total"]))
	offset := (filter.Page - 1) * filter.PageSize
	itemArgs := append([]any(nil), args...)
	itemArgs = append(itemArgs, filter.PageSize, offset)
	items, err := s.repo.Query(ctx, fmt.Sprintf(`
		SELECT al.*, p.code AS project_code, p.name AS project_name,
			COALESCE(audit_actor.deleted_at IS NOT NULL, false) AS actor_account_deleted
		%s
		WHERE %s
		ORDER BY al.created_at DESC, al.id DESC
		LIMIT $%d OFFSET $%d
	`, fromSQL, whereSQL, len(itemArgs)-1, len(itemArgs)), itemArgs...)
	if err != nil {
		return nil, err
	}

	options, err := s.auditLogOptions(ctx, fromSQL, scopeConditions, scopeArgs, projectID == "")
	if err != nil {
		return nil, err
	}
	totalPages := 0
	if total > 0 {
		totalPages = int(math.Ceil(float64(total) / float64(filter.PageSize)))
	}
	return Record{
		"items":       items,
		"total":       total,
		"page":        filter.Page,
		"page_size":   filter.PageSize,
		"total_pages": totalPages,
		"options":     options,
	}, nil
}

func (s *Service) auditLogOptions(ctx context.Context, fromSQL string, conditions []string, args []any, includeProjects bool) (Record, error) {
	row, err := s.repo.QueryOne(ctx, fmt.Sprintf(`
		SELECT
			COALESCE(jsonb_agg(DISTINCT al.event_type) FILTER (WHERE al.event_type <> ''), '[]'::jsonb) AS event_types,
			COALESCE(jsonb_agg(DISTINCT al.target_type) FILTER (WHERE COALESCE(al.target_type,'') <> ''), '[]'::jsonb) AS target_types,
			COALESCE(jsonb_agg(DISTINCT al.actor_name) FILTER (WHERE COALESCE(al.actor_name,'') <> ''), '[]'::jsonb) AS actors
		%s
		WHERE %s
	`, fromSQL, strings.Join(conditions, " AND ")), args...)
	if err != nil {
		return nil, err
	}
	options := Record{
		"event_types":  row["event_types"],
		"target_types": row["target_types"],
		"actors":       row["actors"],
		"projects":     []Record{},
	}
	if !includeProjects {
		return options, nil
	}
	projects, err := s.repo.Query(ctx, fmt.Sprintf(`
		SELECT DISTINCT p.id, p.code, p.name
		%s
		WHERE %s
		ORDER BY p.code, p.name
	`, fromSQL, strings.Join(conditions, " AND ")), args...)
	if err != nil {
		return nil, err
	}
	options["projects"] = projects
	return options, nil
}

func normalizeAuditLogFilter(filter AuditLogFilter) (AuditLogFilter, error) {
	filter.RecordID = cleanID(filter.RecordID)
	if filter.RecordID != "" && !validActivityUUID(filter.RecordID) {
		return filter, validationError("ID sự kiện không hợp lệ", Record{"record_id": filter.RecordID})
	}
	filter.ProjectID = cleanID(filter.ProjectID)
	if filter.ProjectID != "" && !validActivityUUID(filter.ProjectID) {
		return filter, validationError("Công trình lọc không hợp lệ", Record{"project_id": filter.ProjectID})
	}
	filter.EventType = strings.ToLower(strings.TrimSpace(filter.EventType))
	filter.TargetType = strings.ToLower(strings.TrimSpace(filter.TargetType))
	filter.ActorName = strings.TrimSpace(filter.ActorName)
	filter.Search = strings.TrimSpace(filter.Search)
	for field, value := range map[string]string{
		"event_type":  filter.EventType,
		"target_type": filter.TargetType,
		"actor_name":  filter.ActorName,
		"search":      filter.Search,
	} {
		if len([]rune(value)) > 200 {
			return filter, validationError("Bộ lọc audit quá dài", Record{"field": field, "max_length": 200})
		}
	}
	filter.DateFrom = strings.TrimSpace(filter.DateFrom)
	filter.DateTo = strings.TrimSpace(filter.DateTo)
	var dateFrom, dateTo time.Time
	if filter.DateFrom != "" {
		parsed, err := time.Parse("2006-01-02", filter.DateFrom)
		if err != nil {
			return filter, validationError("Ngày bắt đầu audit không hợp lệ", Record{"date_from": filter.DateFrom})
		}
		dateFrom = parsed
	}
	if filter.DateTo != "" {
		parsed, err := time.Parse("2006-01-02", filter.DateTo)
		if err != nil {
			return filter, validationError("Ngày kết thúc audit không hợp lệ", Record{"date_to": filter.DateTo})
		}
		dateTo = parsed
	}
	if !dateFrom.IsZero() && !dateTo.IsZero() && dateFrom.After(dateTo) {
		return filter, validationError("Khoảng thời gian audit không hợp lệ", Record{"date_from": filter.DateFrom, "date_to": filter.DateTo})
	}
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 || filter.PageSize > 100 {
		filter.PageSize = 25
	}
	return filter, nil
}
