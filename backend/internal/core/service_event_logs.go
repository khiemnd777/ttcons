package core

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"
)

func (s *Service) EventLogs(ctx context.Context, filter EventLogFilter) (Record, error) {
	filter, err := normalizeEventLogFilter(filter)
	if err != nil {
		return nil, err
	}
	fromSQL := "FROM event_logs el LEFT JOIN projects p ON p.id=el.project_id"
	conditions := []string{"1=1"}
	args := make([]any, 0, 10)
	addCondition := func(format string, value any) {
		args = append(args, value)
		conditions = append(conditions, fmt.Sprintf(format, len(args)))
	}
	if filter.Category != "" {
		addCondition("el.category=$%d", filter.Category)
	}
	if filter.HTTPStatus != 0 {
		addCondition("el.http_status=$%d", filter.HTTPStatus)
	}
	if filter.Method != "" {
		addCondition("el.method=$%d", filter.Method)
	}
	if filter.Route != "" {
		addCondition("el.route_template=$%d", filter.Route)
	}
	if filter.ActorEmail != "" {
		addCondition("LOWER(COALESCE(el.actor_email,''))=LOWER($%d)", filter.ActorEmail)
	}
	if filter.ProjectID != "" {
		addCondition("el.project_id=$%d", filter.ProjectID)
	}
	if filter.DateFrom != "" {
		addCondition("el.created_at >= $%d::date", filter.DateFrom)
	}
	if filter.DateTo != "" {
		addCondition("el.created_at < ($%d::date + INTERVAL '1 day')", filter.DateTo)
	}
	if filter.Search != "" {
		args = append(args, "%"+filter.Search+"%")
		placeholder := len(args)
		conditions = append(conditions, fmt.Sprintf(`(
			el.request_id ILIKE $%[1]d OR el.error_fingerprint ILIKE $%[1]d OR
			el.error_code ILIKE $%[1]d OR el.route_template ILIKE $%[1]d OR
			COALESCE(el.actor_email,'') ILIKE $%[1]d OR
			COALESCE(el.internal_message,'') ILIKE $%[1]d OR
			COALESCE(p.code,'') ILIKE $%[1]d OR COALESCE(p.name,'') ILIKE $%[1]d
		)`, placeholder))
	}
	whereSQL := strings.Join(conditions, " AND ")
	countRow, err := s.repo.QueryOne(ctx, fmt.Sprintf(`
		SELECT
			COUNT(*) AS total,
			COUNT(*) FILTER (WHERE el.category='unexpected_5xx') AS unexpected_5xx,
			COUNT(*) FILTER (WHERE el.category='panic') AS panic,
			COUNT(*) FILTER (WHERE el.category='controlled_503') AS controlled_503
		%s
		WHERE %s
	`, fromSQL, whereSQL), args...)
	if err != nil {
		return nil, err
	}
	total := int(number(countRow["total"]))
	offset := (filter.Page - 1) * filter.PageSize
	itemArgs := append([]any(nil), args...)
	itemArgs = append(itemArgs, filter.PageSize, offset)
	items, err := s.repo.Query(ctx, fmt.Sprintf(`
		SELECT
			el.id,el.category,el.http_status,el.error_code,el.method,el.route_template,
			el.request_class,el.request_id,el.actor_id,el.actor_email,el.project_id,
			p.code AS project_code,p.name AS project_name,el.duration_ms,el.error_type,
			el.error_fingerprint,LEFT(COALESCE(el.internal_message,''),240) AS message_preview,
			el.created_at
		%s
		WHERE %s
		ORDER BY el.created_at DESC,el.id DESC
		LIMIT $%d OFFSET $%d
	`, fromSQL, whereSQL, len(itemArgs)-1, len(itemArgs)), itemArgs...)
	if err != nil {
		return nil, err
	}
	options, err := s.eventLogOptions(ctx)
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
		"summary": Record{
			"unexpected_5xx": int(number(countRow["unexpected_5xx"])),
			"panic":          int(number(countRow["panic"])),
			"controlled_503": int(number(countRow["controlled_503"])),
		},
		"options": options,
	}, nil
}

func (s *Service) EventLogDetail(ctx context.Context, id string) (Record, error) {
	id = cleanID(id)
	if !validActivityUUID(id) {
		return nil, validationError("ID nhật ký sự kiện không hợp lệ", Record{"id": id})
	}
	return s.repo.QueryOne(ctx, `
		SELECT
			el.id,el.category,el.http_status,el.error_code,el.method,el.route_template,
			el.request_class,el.request_id,el.actor_id,el.actor_email,el.project_id,
			p.code AS project_code,p.name AS project_name,el.duration_ms,el.error_type,
			el.error_fingerprint,el.internal_message,el.stack_trace,el.created_at
		FROM event_logs el
		LEFT JOIN projects p ON p.id=el.project_id
		WHERE el.id=$1
	`, id)
}

func (s *Service) eventLogOptions(ctx context.Context) (Record, error) {
	row, err := s.repo.QueryOne(ctx, `
		SELECT
			COALESCE(jsonb_agg(DISTINCT category) FILTER (WHERE category <> ''),'[]'::jsonb) AS categories,
			COALESCE(jsonb_agg(DISTINCT http_status),'[]'::jsonb) AS statuses,
			COALESCE(jsonb_agg(DISTINCT method) FILTER (WHERE method <> ''),'[]'::jsonb) AS methods,
			COALESCE(jsonb_agg(DISTINCT route_template) FILTER (WHERE route_template <> ''),'[]'::jsonb) AS routes,
			COALESCE(jsonb_agg(DISTINCT actor_email) FILTER (WHERE COALESCE(actor_email,'') <> ''),'[]'::jsonb) AS actors
		FROM event_logs
	`)
	if err != nil {
		return nil, err
	}
	projects, err := s.repo.Query(ctx, `
		SELECT DISTINCT p.id,p.code,p.name
		FROM event_logs el
		JOIN projects p ON p.id=el.project_id
		ORDER BY p.code,p.name
	`)
	if err != nil {
		return nil, err
	}
	return Record{
		"categories": row["categories"],
		"statuses":   row["statuses"],
		"methods":    row["methods"],
		"routes":     row["routes"],
		"actors":     row["actors"],
		"projects":   projects,
	}, nil
}

func normalizeEventLogFilter(filter EventLogFilter) (EventLogFilter, error) {
	filter.Category = strings.ToLower(strings.TrimSpace(filter.Category))
	if filter.Category != "" && !statusIn(filter.Category, "unexpected_5xx", "panic", "controlled_503") {
		return filter, validationError("Loại lỗi không hợp lệ", Record{"category": filter.Category})
	}
	if filter.HTTPStatus != 0 && (filter.HTTPStatus < 500 || filter.HTTPStatus > 599) {
		return filter, validationError("HTTP status không hợp lệ", Record{"http_status": filter.HTTPStatus})
	}
	filter.Method = strings.ToUpper(strings.TrimSpace(filter.Method))
	if filter.Method != "" && !statusIn(filter.Method, "GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS", "OTHER") {
		return filter, validationError("HTTP method không hợp lệ", Record{"method": filter.Method})
	}
	filter.Route = strings.TrimSpace(filter.Route)
	filter.ActorEmail = strings.TrimSpace(filter.ActorEmail)
	filter.Search = strings.TrimSpace(filter.Search)
	for field, value := range map[string]string{"route": filter.Route, "actor_email": filter.ActorEmail, "search": filter.Search} {
		if len([]rune(value)) > 200 {
			return filter, validationError("Bộ lọc nhật ký sự kiện quá dài", Record{"field": field, "max_length": 200})
		}
	}
	filter.ProjectID = cleanID(filter.ProjectID)
	if filter.ProjectID != "" && !validActivityUUID(filter.ProjectID) {
		return filter, validationError("Công trình lọc không hợp lệ", Record{"project_id": filter.ProjectID})
	}
	filter.DateFrom = strings.TrimSpace(filter.DateFrom)
	filter.DateTo = strings.TrimSpace(filter.DateTo)
	var dateFrom, dateTo time.Time
	if filter.DateFrom != "" {
		parsed, err := time.Parse("2006-01-02", filter.DateFrom)
		if err != nil {
			return filter, validationError("Ngày bắt đầu không hợp lệ", Record{"date_from": filter.DateFrom})
		}
		dateFrom = parsed
	}
	if filter.DateTo != "" {
		parsed, err := time.Parse("2006-01-02", filter.DateTo)
		if err != nil {
			return filter, validationError("Ngày kết thúc không hợp lệ", Record{"date_to": filter.DateTo})
		}
		dateTo = parsed
	}
	if !dateFrom.IsZero() && !dateTo.IsZero() && dateFrom.After(dateTo) {
		return filter, validationError("Khoảng thời gian không hợp lệ", Record{"date_from": filter.DateFrom, "date_to": filter.DateTo})
	}
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 || filter.PageSize > 100 {
		filter.PageSize = 25
	}
	return filter, nil
}
