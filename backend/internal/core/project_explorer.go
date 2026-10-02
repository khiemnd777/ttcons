package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
)

const (
	projectExplorerDefaultPageSize = 25
	projectExplorerMaxSearchLength = 120
)

var projectExplorerPageSizes = map[int]struct{}{25: {}, 50: {}, 100: {}}

type ProjectExplorerRequest struct {
	Search    string
	Status    string
	Sort      string
	Direction string
	Page      int
	PageSize  int
}

type ProjectExplorerOptions struct {
	Statuses []string `json:"statuses"`
}

type ProjectExplorerPage struct {
	Items      []Record               `json:"items"`
	Total      int                    `json:"total"`
	Page       int                    `json:"page"`
	PageSize   int                    `json:"page_size"`
	TotalPages int                    `json:"total_pages"`
	Options    ProjectExplorerOptions `json:"options"`
}

type projectExplorerIndex struct {
	IDs      []string `json:"ids"`
	Total    int      `json:"total"`
	Statuses []string `json:"statuses"`
}

type projectExplorerRevisionSnapshot struct {
	CatalogVersion   int64
	ActorVersion     int64
	BypassAccess     bool
	SupervisionScope bool
	StaffID          string
}

func normalizeProjectExplorerRequest(input ProjectExplorerRequest) (ProjectExplorerRequest, error) {
	input.Search = strings.TrimSpace(input.Search)
	input.Status = strings.TrimSpace(input.Status)
	input.Sort = strings.ToLower(strings.TrimSpace(input.Sort))
	input.Direction = strings.ToLower(strings.TrimSpace(input.Direction))
	if input.Page == 0 {
		input.Page = 1
	}
	if input.PageSize == 0 {
		input.PageSize = projectExplorerDefaultPageSize
	}
	if input.Sort == "" {
		input.Sort = "created_at"
	}
	if input.Direction == "" {
		input.Direction = "desc"
	}
	if input.Page < 1 {
		return input, validationError("Trang phải lớn hơn hoặc bằng 1", Record{"page": input.Page})
	}
	if _, ok := projectExplorerPageSizes[input.PageSize]; !ok {
		return input, validationError("Số dòng mỗi trang phải là 25, 50 hoặc 100", Record{"page_size": input.PageSize})
	}
	if len([]rune(input.Search)) > projectExplorerMaxSearchLength {
		return input, validationError("Từ khoá tìm kiếm không được vượt quá 120 ký tự", Record{"search": input.Search})
	}
	if len([]rune(input.Status)) > 64 {
		return input, validationError("Trạng thái không hợp lệ", Record{"status": input.Status})
	}
	switch input.Sort {
	case "project", "status", "owner", "created_at", "updated_at":
	default:
		return input, validationError("Trường sắp xếp không hợp lệ", Record{"sort": input.Sort})
	}
	if input.Direction != "asc" && input.Direction != "desc" {
		return input, validationError("Chiều sắp xếp phải là asc hoặc desc", Record{"direction": input.Direction})
	}
	return input, nil
}

func (q ProjectExplorerRequest) cacheHash() string {
	payload, _ := json.Marshal(struct {
		Search    string `json:"search"`
		Status    string `json:"status"`
		Sort      string `json:"sort"`
		Direction string `json:"direction"`
		Page      int    `json:"page"`
		PageSize  int    `json:"page_size"`
	}{q.Search, q.Status, q.Sort, q.Direction, q.Page, q.PageSize})
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func (r *Repository) ProjectExplorerIndex(ctx context.Context, actorID string, request ProjectExplorerRequest) (projectExplorerIndex, error) {
	scopeSQL, args, err := r.SupervisionProjectAccessFilter(ctx, actorID, "p", 1)
	if err != nil {
		return projectExplorerIndex{}, err
	}
	baseWhere := []string{"p.status <> 'cancelled'", scopeSQL}
	filteredWhere := append([]string(nil), baseWhere...)
	if request.Search != "" {
		args = append(args, "%"+escapeLikePattern(request.Search)+"%")
		placeholder := len(args)
		filteredWhere = append(filteredWhere, fmt.Sprintf(`(
			LOWER(p.name) LIKE LOWER($%[1]d) ESCAPE E'\\' OR
			LOWER(p.code) LIKE LOWER($%[1]d) ESCAPE E'\\' OR
			LOWER(p.client_name) LIKE LOWER($%[1]d) ESCAPE E'\\' OR
			LOWER(COALESCE(s.full_name, '')) LIKE LOWER($%[1]d) ESCAPE E'\\'
		)`, placeholder))
	}
	if request.Status != "" {
		args = append(args, request.Status)
		filteredWhere = append(filteredWhere, fmt.Sprintf("p.status=$%d", len(args)))
	}

	countRow, err := r.QueryOne(ctx, fmt.Sprintf(`
		SELECT COUNT(*) AS total
		FROM projects p
		LEFT JOIN staff s ON s.id=p.pm_staff_id
		WHERE %s
	`, strings.Join(filteredWhere, " AND ")), args...)
	if err != nil {
		return projectExplorerIndex{}, err
	}

	itemArgs := append([]any(nil), args...)
	itemArgs = append(itemArgs, request.PageSize, (request.Page-1)*request.PageSize)
	rows, err := r.Query(ctx, fmt.Sprintf(`
		SELECT p.id::text AS id
		FROM projects p
		LEFT JOIN staff s ON s.id=p.pm_staff_id
		WHERE %s
		ORDER BY %s
		LIMIT $%d OFFSET $%d
	`, strings.Join(filteredWhere, " AND "), projectExplorerOrderSQL(request), len(itemArgs)-1, len(itemArgs)), itemArgs...)
	if err != nil {
		return projectExplorerIndex{}, err
	}
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		if id := cleanID(row["id"]); id != "" {
			ids = append(ids, id)
		}
	}

	optionRow, err := r.QueryOne(ctx, fmt.Sprintf(`
		SELECT COALESCE(jsonb_agg(status ORDER BY status), '[]'::jsonb) AS statuses
		FROM (
			SELECT DISTINCT p.status
			FROM projects p
			WHERE %s
		) scoped_statuses
	`, strings.Join(baseWhere, " AND ")), args[:3]...)
	if err != nil {
		return projectExplorerIndex{}, err
	}
	return projectExplorerIndex{
		IDs:      ids,
		Total:    int(number(countRow["total"])),
		Statuses: projectExplorerStringSlice(optionRow["statuses"]),
	}, nil
}

func (r *Repository) ProjectExplorerRows(ctx context.Context, projectIDs []string) (map[string]Record, error) {
	result := make(map[string]Record, len(projectIDs))
	if len(projectIDs) == 0 {
		return result, nil
	}
	placeholders := make([]string, 0, len(projectIDs))
	args := make([]any, 0, len(projectIDs))
	for _, projectID := range projectIDs {
		args = append(args, projectID)
		placeholders = append(placeholders, fmt.Sprintf("$%d", len(args)))
	}
	rows, err := r.Query(ctx, fmt.Sprintf(`
		SELECT
			p.id::text AS id,
			p.name,
			p.code,
			COALESCE(p.partner_id::text, '') AS partner_id,
			p.client_name,
			p.address,
			p.planned_start_date,
			p.planned_end_date,
			p.estimated_budget,
			p.contract_amount,
			p.status,
			COALESCE(p.pipeline_template_id::text, '') AS pipeline_template_id,
			COALESCE(p.current_pipeline_run_id::text, '') AS current_pipeline_run_id,
			COALESCE(p.pm_staff_id::text, '') AS pm_staff_id,
			p.progress_percent,
			p.is_sample,
			p.sample_profile,
			p.sample_blueprint_version,
			p.workflow_entry_mode,
			p.created_at,
			p.updated_at,
			COALESCE(s.full_name, '') AS pm_name,
			COALESCE(s.status='deleted', false) AS pm_account_deleted,
			COALESCE(t.name, '') AS pipeline_template_name,
			COALESCE(pt.name, '') AS partner_name,
			COALESCE(pt.tax_code, '') AS partner_tax_code,
			COALESCE((SELECT COUNT(*) FROM issues i WHERE i.project_id=p.id AND i.status IN ('open','in_progress')), 0) AS open_issues,
			COALESCE((SELECT COUNT(*) FROM approval_requests ar WHERE ar.project_id=p.id AND ar.status='pending'), 0) AS pending_approvals
		FROM projects p
		LEFT JOIN staff s ON s.id=p.pm_staff_id
		LEFT JOIN pipeline_templates t ON t.id=p.pipeline_template_id
		LEFT JOIN partners pt ON pt.id=p.partner_id
		WHERE p.id::text IN (%s)
	`, strings.Join(placeholders, ",")), args...)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		if id := cleanID(row["id"]); id != "" {
			result[id] = row
		}
	}
	return result, nil
}

func (r *Repository) ProjectExplorerRevisionSnapshot(ctx context.Context, actorID string) (projectExplorerRevisionSnapshot, error) {
	row, err := r.QueryOne(ctx, `
		SELECT state.catalog_version,
			COALESCE(actor.version, 1) AS actor_version,
			EXISTS (
				SELECT 1
				FROM user_roles ur
				JOIN roles role ON role.id=ur.role_id
				WHERE ur.user_id=$1 AND ur.project_id IS NULL AND role.name IN ('admin','director')
			) AS bypass_access,
			`+deputyManagerUserSQL("$1::uuid")+` AS supervision_scope,
			COALESCE((
				SELECT staff.id::text
				FROM staff
				WHERE staff.user_id=$1 AND staff.status='active'
				ORDER BY staff.id
				LIMIT 1
			), '') AS staff_id
		FROM project_explorer_cache_state state
		LEFT JOIN project_explorer_actor_versions actor ON actor.user_id=$1
		WHERE state.id=1
	`, actorID)
	if err != nil {
		return projectExplorerRevisionSnapshot{}, err
	}
	return projectExplorerRevisionSnapshot{
		CatalogVersion:   int64(number(row["catalog_version"])),
		ActorVersion:     int64(number(row["actor_version"])),
		BypassAccess:     boolValue(row["bypass_access"]),
		SupervisionScope: boolValue(row["supervision_scope"]),
		StaffID:          cleanID(row["staff_id"]),
	}, nil
}

func (r *Repository) ProjectExplorerProjectVersions(ctx context.Context, actorID string, revision projectExplorerRevisionSnapshot, projectIDs []string) (map[string]int64, error) {
	versions := make(map[string]int64, len(projectIDs))
	if len(projectIDs) == 0 {
		return versions, nil
	}
	scopeSQL, scopeArgs, err := r.SupervisionProjectAccessFilter(ctx, actorID, "p", 1)
	if err != nil {
		return nil, err
	}
	placeholders := make([]string, 0, len(projectIDs))
	args := append([]any(nil), scopeArgs...)
	for _, projectID := range projectIDs {
		args = append(args, projectID)
		placeholders = append(placeholders, fmt.Sprintf("$%d", len(args)))
	}
	rows, err := r.Query(ctx, fmt.Sprintf(`
		SELECT revision.project_id::text AS project_id, revision.version
		FROM project_explorer_project_versions revision
		JOIN projects p ON p.id=revision.project_id
		WHERE p.status <> 'cancelled'
		  AND %s
		  AND p.id::text IN (%s)
	`, scopeSQL, strings.Join(placeholders, ",")), args...)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		versions[cleanID(row["project_id"])] = int64(number(row["version"]))
	}
	return versions, nil
}

func (s *Service) ProjectExplorer(ctx context.Context, actorID string, request ProjectExplorerRequest) (ProjectExplorerPage, error) {
	normalized, err := normalizeProjectExplorerRequest(request)
	if err != nil {
		return ProjectExplorerPage{}, err
	}
	return s.projectExplorerPage(ctx, actorID, normalized)
}

func (s *Service) projectExplorerDatabasePage(ctx context.Context, actorID string, request ProjectExplorerRequest) (ProjectExplorerPage, error) {
	index, err := s.repo.ProjectExplorerIndex(ctx, actorID, request)
	if err != nil {
		return ProjectExplorerPage{}, err
	}
	rows, err := s.repo.ProjectExplorerRows(ctx, index.IDs)
	if err != nil {
		return ProjectExplorerPage{}, err
	}
	return assembleProjectExplorerPage(index, rows, request), nil
}

func assembleProjectExplorerPage(index projectExplorerIndex, rows map[string]Record, request ProjectExplorerRequest) ProjectExplorerPage {
	items := make([]Record, 0, len(index.IDs))
	for _, projectID := range index.IDs {
		if row, ok := rows[projectID]; ok {
			items = append(items, row)
		}
	}
	totalPages := 0
	if index.Total > 0 {
		totalPages = int(math.Ceil(float64(index.Total) / float64(request.PageSize)))
	}
	return ProjectExplorerPage{
		Items:      items,
		Total:      index.Total,
		Page:       request.Page,
		PageSize:   request.PageSize,
		TotalPages: totalPages,
		Options:    ProjectExplorerOptions{Statuses: append([]string(nil), index.Statuses...)},
	}
}

func projectExplorerOrderSQL(request ProjectExplorerRequest) string {
	direction := "ASC"
	if request.Direction == "desc" {
		direction = "DESC"
	}
	tieBreak := "LOWER(p.code) ASC, LOWER(p.name) ASC, p.id ASC"
	switch request.Sort {
	case "project":
		return fmt.Sprintf("LOWER(p.code) %s, LOWER(p.name) %s, p.id ASC", direction, direction)
	case "status":
		return fmt.Sprintf("LOWER(%s) %s, %s", projectStatusLabelSQL("p.status"), direction, tieBreak)
	case "owner":
		return fmt.Sprintf("(NULLIF(BTRIM(s.full_name), '') IS NULL) ASC, LOWER(s.full_name) %s, %s", direction, tieBreak)
	case "updated_at":
		return fmt.Sprintf("p.updated_at %s, %s", direction, tieBreak)
	default:
		return fmt.Sprintf("p.created_at %s, %s", direction, tieBreak)
	}
}

func projectStatusLabelSQL(column string) string {
	return fmt.Sprintf(`CASE %s
		WHEN 'draft' THEN 'Nháp'
		WHEN 'planning' THEN 'Đang lập kế hoạch'
		WHEN 'in_progress' THEN 'Đang xử lý'
		WHEN 'blocked' THEN 'Bị chặn'
		WHEN 'acceptance' THEN 'Đang nghiệm thu'
		WHEN 'handover' THEN 'Đang bàn giao'
		WHEN 'completed' THEN 'Hoàn tất'
		ELSE %s END`, column, column)
}

func escapeLikePattern(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `%`, `\%`)
	return strings.ReplaceAll(value, `_`, `\_`)
}

func projectExplorerStringSlice(value any) []string {
	values := make([]string, 0)
	switch typed := value.(type) {
	case []any:
		for _, item := range typed {
			if text := strings.TrimSpace(fmt.Sprint(item)); text != "" {
				values = append(values, text)
			}
		}
	case []string:
		values = append(values, typed...)
	}
	sort.Strings(values)
	return values
}
