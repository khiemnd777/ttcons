package core

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"consflow/backend/internal/shared/response"

	"github.com/gofiber/fiber/v2"
)

const (
	projectProgressDefaultPageSize = 25
	projectProgressMaxPageSize     = 50
	projectProgressMaxSearchLength = 120
)

var projectProgressStageKeys = []string{
	"not_started",
	"survey",
	"design",
	"commercial",
	"planning",
	"construction",
	"acceptance",
	"handover_finance",
	"completed",
	"other",
}

type ProjectProgressFilter struct {
	Search    string
	PMStaffID string
	Status    string
}

type ProjectProgressStageRequest struct {
	ProjectProgressFilter
	StageKey string
	Cursor   string
	Limit    int
}

type ProjectProgressPMOption struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	AccountDeleted bool   `json:"account_deleted"`
}

type ProjectProgressSummaryResponse struct {
	FilteredTotal int                       `json:"filtered_total"`
	StageCounts   map[string]int            `json:"stage_counts"`
	PMOptions     []ProjectProgressPMOption `json:"pm_options"`
	StatusOptions []string                  `json:"status_options"`
	GeneratedAt   string                    `json:"generated_at"`
}

type ProjectProgressStagePage struct {
	Items       []Record `json:"items"`
	Total       int      `json:"total"`
	HasMore     bool     `json:"has_more"`
	NextCursor  string   `json:"next_cursor,omitempty"`
	GeneratedAt string   `json:"generated_at"`
}

type projectProgressCursor struct {
	Version    int    `json:"v"`
	StageKey   string `json:"stage"`
	CreatedAt  string `json:"created_at"`
	ID         string `json:"id"`
	FilterHash string `json:"filter_hash"`
}

func (h *Handler) ProjectProgressBoardSummary(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"project.view"}); err != nil {
		return err
	}
	summary, err := h.service.ProjectProgressBoardSummary(c.Context(), actorID(c), projectProgressFilterFromContext(c))
	if err != nil {
		return err
	}
	return response.OK(c, summary)
}

func (h *Handler) ProjectProgressBoardStage(c *fiber.Ctx) error {
	if err := h.requirePermission(c, []string{"project.view"}); err != nil {
		return err
	}
	limit := 0
	if rawLimit := strings.TrimSpace(c.Query("limit")); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil {
			return validationError("limit không hợp lệ", Record{"limit": rawLimit})
		}
		limit = parsed
	}
	page, err := h.service.ProjectProgressBoardStage(c.Context(), actorID(c), ProjectProgressStageRequest{
		ProjectProgressFilter: projectProgressFilterFromContext(c),
		StageKey:              c.Params("stageKey"),
		Cursor:                c.Query("cursor"),
		Limit:                 limit,
	})
	if err != nil {
		return err
	}
	return response.OK(c, page)
}

func projectProgressFilterFromContext(c *fiber.Ctx) ProjectProgressFilter {
	return ProjectProgressFilter{
		Search:    c.Query("search"),
		PMStaffID: c.Query("pm_staff_id"),
		Status:    c.Query("status"),
	}
}

func (s *Service) ProjectProgressBoardSummary(ctx context.Context, actorID string, filter ProjectProgressFilter) (ProjectProgressSummaryResponse, error) {
	normalized, err := normalizeProjectProgressFilter(filter)
	if err != nil {
		return ProjectProgressSummaryResponse{}, err
	}
	summary, err := s.repo.ProjectProgressBoardSummary(ctx, actorID, normalized)
	if err != nil {
		return ProjectProgressSummaryResponse{}, err
	}
	summary.GeneratedAt = time.Now().UTC().Format(time.RFC3339)
	return summary, nil
}

func (s *Service) ProjectProgressBoardStage(ctx context.Context, actorID string, request ProjectProgressStageRequest) (ProjectProgressStagePage, error) {
	normalized, cursor, err := normalizeProjectProgressStageRequest(request)
	if err != nil {
		return ProjectProgressStagePage{}, err
	}
	items, total, err := s.repo.ProjectProgressBoardStage(ctx, actorID, normalized, cursor)
	if err != nil {
		return ProjectProgressStagePage{}, err
	}
	hasMore := len(items) > normalized.Limit
	if hasMore {
		items = items[:normalized.Limit]
	}
	nextCursor := ""
	if hasMore && len(items) > 0 {
		last := items[len(items)-1]
		nextCursor = encodeProjectProgressCursor(projectProgressCursor{
			Version:    1,
			StageKey:   normalized.StageKey,
			CreatedAt:  strings.TrimSpace(fmt.Sprint(last["cursor_created_at"])),
			ID:         cleanID(last["id"]),
			FilterHash: normalized.ProjectProgressFilter.hash(),
		})
	}
	for _, item := range items {
		delete(item, "cursor_created_at")
		delete(item, "cursor_id")
	}
	return ProjectProgressStagePage{
		Items:       items,
		Total:       total,
		HasMore:     hasMore,
		NextCursor:  nextCursor,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
	}, nil
}

func normalizeProjectProgressFilter(filter ProjectProgressFilter) (ProjectProgressFilter, error) {
	filter.Search = strings.TrimSpace(filter.Search)
	filter.PMStaffID = cleanID(filter.PMStaffID)
	filter.Status = strings.TrimSpace(filter.Status)
	if len([]rune(filter.Search)) > projectProgressMaxSearchLength {
		return filter, validationError("Từ khoá tìm kiếm không được vượt quá 120 ký tự", Record{"search": filter.Search})
	}
	if filter.PMStaffID != "" && !validActivityUUID(filter.PMStaffID) {
		return filter, validationError("pm_staff_id không hợp lệ", Record{"pm_staff_id": filter.PMStaffID})
	}
	if len([]rune(filter.Status)) > 64 {
		return filter, validationError("Trạng thái không hợp lệ", Record{"status": filter.Status})
	}
	return filter, nil
}

func normalizeProjectProgressStageRequest(request ProjectProgressStageRequest) (ProjectProgressStageRequest, *projectProgressCursor, error) {
	filter, err := normalizeProjectProgressFilter(request.ProjectProgressFilter)
	if err != nil {
		return request, nil, err
	}
	request.ProjectProgressFilter = filter
	request.StageKey = strings.TrimSpace(request.StageKey)
	if !validProjectProgressStageKey(request.StageKey) {
		return request, nil, validationError("Giai đoạn tiến trình không hợp lệ", Record{"stage": request.StageKey})
	}
	if request.Limit == 0 {
		request.Limit = projectProgressDefaultPageSize
	}
	if request.Limit < 1 || request.Limit > projectProgressMaxPageSize {
		return request, nil, validationError("limit phải nằm trong khoảng 1 đến 50", Record{"limit": request.Limit})
	}
	request.Cursor = strings.TrimSpace(request.Cursor)
	if request.Cursor == "" {
		return request, nil, nil
	}
	cursor, err := parseProjectProgressCursor(request.Cursor)
	if err != nil || cursor.StageKey != request.StageKey || cursor.FilterHash != request.ProjectProgressFilter.hash() {
		return request, nil, validationError("cursor không hợp lệ hoặc không còn khớp bộ lọc", Record{"cursor": request.Cursor})
	}
	return request, &cursor, nil
}

func validProjectProgressStageKey(stageKey string) bool {
	for _, candidate := range projectProgressStageKeys {
		if stageKey == candidate {
			return true
		}
	}
	return false
}

func (filter ProjectProgressFilter) hash() string {
	payload, _ := json.Marshal(struct {
		Search    string `json:"search"`
		PMStaffID string `json:"pm_staff_id"`
		Status    string `json:"status"`
	}{
		Search:    strings.ToLower(filter.Search),
		PMStaffID: filter.PMStaffID,
		Status:    filter.Status,
	})
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func encodeProjectProgressCursor(cursor projectProgressCursor) string {
	raw, _ := json.Marshal(cursor)
	return base64.RawURLEncoding.EncodeToString(raw)
}

func parseProjectProgressCursor(value string) (projectProgressCursor, error) {
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return projectProgressCursor{}, err
	}
	cursor := projectProgressCursor{}
	if err := json.Unmarshal(raw, &cursor); err != nil {
		return projectProgressCursor{}, err
	}
	cursor.StageKey = strings.TrimSpace(cursor.StageKey)
	cursor.CreatedAt = strings.TrimSpace(cursor.CreatedAt)
	cursor.ID = cleanID(cursor.ID)
	cursor.FilterHash = strings.TrimSpace(cursor.FilterHash)
	if cursor.Version != 1 || !validProjectProgressStageKey(cursor.StageKey) || !validActivityUUID(cursor.ID) {
		return projectProgressCursor{}, errors.New("invalid project progress cursor")
	}
	if _, err := time.Parse(time.RFC3339Nano, cursor.CreatedAt); err != nil {
		return projectProgressCursor{}, errors.New("invalid project progress cursor timestamp")
	}
	if len(cursor.FilterHash) != sha256.Size*2 {
		return projectProgressCursor{}, errors.New("invalid project progress cursor filter hash")
	}
	if _, err := hex.DecodeString(cursor.FilterHash); err != nil {
		return projectProgressCursor{}, errors.New("invalid project progress cursor filter hash")
	}
	return cursor, nil
}

func (r *Repository) ProjectProgressBoardSummary(ctx context.Context, actorID string, filter ProjectProgressFilter) (ProjectProgressSummaryResponse, error) {
	cte, args, err := r.projectProgressFilteredCTE(ctx, actorID, filter)
	if err != nil {
		return ProjectProgressSummaryResponse{}, err
	}
	row, err := r.QueryOne(ctx, cte+`
		SELECT
			(SELECT COUNT(*) FROM filtered_projects) AS filtered_total,
			COALESCE((
				SELECT jsonb_object_agg(stage_key, total)
				FROM (
					SELECT stage_key, COUNT(*) AS total
					FROM filtered_projects
					GROUP BY stage_key
				) stage_totals
			), '{}'::jsonb) AS stage_counts,
			COALESCE((
				SELECT jsonb_agg(
					jsonb_build_object('id', option.pm_staff_id::text, 'name', option.pm_name, 'account_deleted', option.pm_account_deleted)
					ORDER BY lower(option.pm_name), option.pm_staff_id
				)
				FROM (
					SELECT DISTINCT pm_staff_id, pm_name, pm_account_deleted
					FROM scoped_projects
					WHERE pm_staff_id IS NOT NULL
				) option
			), '[]'::jsonb) AS pm_options,
			COALESCE((
				SELECT jsonb_agg(option.project_status ORDER BY option.project_status)
				FROM (
					SELECT DISTINCT project_status
					FROM scoped_projects
				) option
			), '[]'::jsonb) AS status_options
	`, args...)
	if err != nil {
		return ProjectProgressSummaryResponse{}, err
	}
	return projectProgressSummaryFromRecord(row), nil
}

func (r *Repository) ProjectProgressBoardStage(ctx context.Context, actorID string, request ProjectProgressStageRequest, cursor *projectProgressCursor) ([]Record, int, error) {
	cte, args, err := r.projectProgressFilteredCTE(ctx, actorID, request.ProjectProgressFilter)
	if err != nil {
		return nil, 0, err
	}
	args = append(args, request.StageKey)
	stagePlaceholder := len(args)
	pageConditions := []string{"TRUE"}
	if cursor != nil {
		args = append(args, cursor.CreatedAt, cursor.ID)
		pageConditions = append(pageConditions, fmt.Sprintf(
			"(stage_project.created_at, stage_project.id) < ($%d::timestamptz, $%d::uuid)", len(args)-1, len(args),
		))
	}
	args = append(args, request.Limit+1)
	limitPlaceholder := len(args)
	row, err := r.QueryOne(ctx, cte+fmt.Sprintf(`,
		stage_projects AS MATERIALIZED (
			SELECT *
			FROM filtered_projects
			WHERE stage_key=$%d
		)
		SELECT
			(SELECT COUNT(*) FROM stage_projects) AS total,
			COALESCE((
				SELECT jsonb_agg(
					to_jsonb(page_row)
					ORDER BY page_row.cursor_created_at DESC, page_row.cursor_id DESC
				)
				FROM (
					SELECT
						stage_project.id AS cursor_id,
						stage_project.created_at AS cursor_created_at,
						stage_project.id::text AS id,
						stage_project.code,
						stage_project.name,
						stage_project.client_name,
						stage_project.project_status,
						stage_project.progress_percent,
						stage_project.planned_start_date,
						stage_project.planned_end_date,
						stage_project.pm_name,
						stage_project.pm_account_deleted,
						stage_project.pipeline_run_id,
						stage_project.pipeline_run_status,
						stage_project.current_node_id,
						stage_project.current_node_name,
						stage_project.current_node_type,
						stage_project.current_node_status,
						stage_project.current_node_order,
						stage_project.current_node_block_reason,
						stage_project.stage_key,
						COALESCE((
							SELECT COUNT(*) FROM issues issue
							WHERE issue.project_id=stage_project.id AND issue.status IN ('open','in_progress')
						), 0) AS open_issues,
						COALESCE((
							SELECT COUNT(*) FROM approval_requests approval
							WHERE approval.project_id=stage_project.id AND approval.status='pending'
						), 0) AS pending_approvals,
						COALESCE((
							SELECT COUNT(*) FROM work_items work_item
							WHERE work_item.project_id=stage_project.id AND work_item.status='blocked'
						), 0) AS blocked_items,
						COALESCE((
							SELECT COUNT(*) FROM work_items work_item
							WHERE work_item.project_id=stage_project.id
							  AND work_item.status NOT IN ('completed','cancelled')
							  AND work_item.due_at IS NOT NULL
							  AND work_item.due_at < now()
						), 0) AS overdue_items
					FROM stage_projects stage_project
					WHERE %s
					ORDER BY stage_project.created_at DESC, stage_project.id DESC
					LIMIT $%d
				) page_row
			), '[]'::jsonb) AS items
	`, stagePlaceholder, strings.Join(pageConditions, " AND "), limitPlaceholder), args...)
	if err != nil {
		return nil, 0, err
	}
	return projectProgressRecords(row["items"]), int(number(row["total"])), nil
}

func (r *Repository) projectProgressFilteredCTE(ctx context.Context, actorID string, filter ProjectProgressFilter) (string, []any, error) {
	scopeSQL, scopeArgs, err := r.ProjectAccessFilter(ctx, actorID, "p", 1)
	if err != nil {
		return "", nil, err
	}
	args := append([]any{}, scopeArgs...)
	conditions := []string{"TRUE"}
	if filter.Search != "" {
		args = append(args, "%"+escapeLikePattern(filter.Search)+"%")
		placeholder := len(args)
		conditions = append(conditions, fmt.Sprintf(`(
			LOWER(scoped_project.code) LIKE LOWER($%[1]d) ESCAPE E'\\' OR
			LOWER(scoped_project.name) LIKE LOWER($%[1]d) ESCAPE E'\\' OR
			LOWER(scoped_project.client_name) LIKE LOWER($%[1]d) ESCAPE E'\\' OR
			LOWER(scoped_project.pm_name) LIKE LOWER($%[1]d) ESCAPE E'\\' OR
			LOWER(scoped_project.current_node_name) LIKE LOWER($%[1]d) ESCAPE E'\\'
		)`, placeholder))
	}
	if filter.PMStaffID != "" {
		args = append(args, filter.PMStaffID)
		conditions = append(conditions, fmt.Sprintf("scoped_project.pm_staff_id=$%d::uuid", len(args)))
	}
	if filter.Status != "" {
		args = append(args, filter.Status)
		conditions = append(conditions, fmt.Sprintf("scoped_project.project_status=$%d", len(args)))
	}
	return fmt.Sprintf(`
		WITH scoped_projects AS MATERIALIZED (
			SELECT
				p.id,
				p.code,
				p.name,
				COALESCE(p.client_name, '') AS client_name,
				p.status AS project_status,
				p.progress_percent,
				p.planned_start_date,
				p.planned_end_date,
				p.created_at,
				p.pm_staff_id,
				COALESCE(staff.full_name, '') AS pm_name,
				COALESCE(staff.status='deleted', false) AS pm_account_deleted,
				COALESCE(run.id::text, '') AS pipeline_run_id,
				COALESCE(run.status, '') AS pipeline_run_status,
				COALESCE(node.id::text, '') AS current_node_id,
				COALESCE(node.name, '') AS current_node_name,
				COALESCE(node.type, '') AS current_node_type,
				COALESCE(node.status, '') AS current_node_status,
				COALESCE(node.order_index, 0) AS current_node_order,
				COALESCE(node.block_reason, '') AS current_node_block_reason,
				%s AS stage_key
			FROM projects p
			LEFT JOIN staff ON staff.id=p.pm_staff_id
			LEFT JOIN pipeline_runs run ON run.id=p.current_pipeline_run_id AND run.project_id=p.id
			LEFT JOIN pipeline_run_nodes node ON node.id=run.current_node_id AND node.project_id=p.id
			WHERE p.status <> 'cancelled' AND %s
		),
		filtered_projects AS MATERIALIZED (
			SELECT *
			FROM scoped_projects scoped_project
			WHERE %s
		)
	`, projectProgressStageSQL("p", "run", "node"), scopeSQL, strings.Join(conditions, " AND ")), args, nil
}

func projectProgressStageSQL(projectAlias, runAlias, nodeAlias string) string {
	return fmt.Sprintf(`CASE
		WHEN %[1]s.status IN ('completed','complete') OR %[2]s.status='completed' THEN 'completed'
		WHEN %[2]s.id IS NULL THEN 'not_started'
		WHEN %[3]s.type='survey' THEN 'survey'
		WHEN %[3]s.type='design' THEN 'design'
		WHEN %[3]s.type IN ('boq','budget_approval','procurement','warehouse_issue','contract') THEN 'commercial'
		WHEN %[3]s.type='construction_planning' THEN 'planning'
		WHEN %[3]s.type='construction' THEN 'construction'
		WHEN %[3]s.type IN ('incoming_acceptance','acceptance','rework','replanning') THEN 'acceptance'
		WHEN %[3]s.type IN ('handover','financial_closure') THEN 'handover_finance'
		ELSE 'other'
	END`, projectAlias, runAlias, nodeAlias)
}

func projectProgressSummaryFromRecord(row Record) ProjectProgressSummaryResponse {
	stageCounts := make(map[string]int, len(projectProgressStageKeys))
	for _, stageKey := range projectProgressStageKeys {
		stageCounts[stageKey] = 0
	}
	if values, ok := row["stage_counts"].(map[string]any); ok {
		for stageKey, value := range values {
			if validProjectProgressStageKey(stageKey) {
				stageCounts[stageKey] = int(number(value))
			}
		}
	}
	pmOptions := make([]ProjectProgressPMOption, 0)
	if values, ok := row["pm_options"].([]any); ok {
		for _, value := range values {
			option, ok := value.(map[string]any)
			if !ok {
				continue
			}
			id := cleanID(option["id"])
			if id == "" {
				continue
			}
			pmOptions = append(pmOptions, ProjectProgressPMOption{ID: id, Name: strings.TrimSpace(fmt.Sprint(option["name"])), AccountDeleted: boolValue(option["account_deleted"])})
		}
	}
	statusOptions := make([]string, 0)
	if values, ok := row["status_options"].([]any); ok {
		for _, value := range values {
			status := strings.TrimSpace(fmt.Sprint(value))
			if status != "" {
				statusOptions = append(statusOptions, status)
			}
		}
	}
	return ProjectProgressSummaryResponse{
		FilteredTotal: int(number(row["filtered_total"])),
		StageCounts:   stageCounts,
		PMOptions:     pmOptions,
		StatusOptions: statusOptions,
	}
}

func projectProgressRecords(value any) []Record {
	values, ok := value.([]any)
	if !ok {
		return []Record{}
	}
	items := make([]Record, 0, len(values))
	for _, value := range values {
		if item, ok := value.(map[string]any); ok {
			items = append(items, Record(item))
		}
	}
	return items
}
