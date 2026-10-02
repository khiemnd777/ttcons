package core

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"consflow/backend/internal/shared/response"
	"github.com/gofiber/fiber/v2"
)

// Expressions passed to these predicates are repository-owned SQL, never input.
func deputyManagerUserSQL(userExpr string) string {
	return fmt.Sprintf(`EXISTS (
		SELECT 1 FROM users supervision_user
		JOIN user_roles supervision_assignment ON supervision_assignment.user_id=supervision_user.id AND supervision_assignment.project_id IS NULL
		JOIN roles supervision_role ON supervision_role.id=supervision_assignment.role_id AND supervision_role.name='deputy_manager' AND supervision_role.system_role AND NOT supervision_role.external_only
		JOIN role_permissions supervision_grant ON supervision_grant.role_id=supervision_role.id
		JOIN permissions supervision_permission ON supervision_permission.id=supervision_grant.permission_id AND supervision_permission.key='project.supervision.view'
		WHERE supervision_user.id=%s AND supervision_user.status='active' AND supervision_user.deleted_at IS NULL AND NOT supervision_user.is_sample
		AND NOT EXISTS (SELECT 1 FROM user_roles external_assignment JOIN roles external_role ON external_role.id=external_assignment.role_id WHERE external_assignment.user_id=supervision_user.id AND external_role.external_only)
	)`, userExpr)
}

func supervisedProjectSQL(projectExpr string) string {
	return fmt.Sprintf(`EXISTS (SELECT 1 FROM projects supervision_project WHERE supervision_project.id=%s AND supervision_project.status <> 'cancelled' AND NOT supervision_project.is_sample)`, projectExpr)
}

// Classification uses canonical roles and member role IDs/labels, never position
// titles or people's names. Persist report classification on first delivery.
func supervisorUserSQL(userExpr, projectExpr string) string {
	return fmt.Sprintf(`EXISTS (SELECT 1 FROM users supervisor_user WHERE supervisor_user.id=%[1]s AND NOT supervisor_user.is_sample AND (
		EXISTS (SELECT 1 FROM user_roles supervisor_assignment JOIN roles supervisor_role ON supervisor_role.id=supervisor_assignment.role_id
			WHERE supervisor_assignment.user_id=supervisor_user.id AND (supervisor_assignment.project_id IS NULL OR supervisor_assignment.project_id=%[2]s) AND supervisor_role.name='supervisor' AND NOT supervisor_role.external_only)
		OR EXISTS (SELECT 1 FROM staff supervisor_staff JOIN project_members supervisor_member ON supervisor_member.staff_id=supervisor_staff.id AND supervisor_member.active
			WHERE supervisor_staff.user_id=supervisor_user.id AND supervisor_staff.status <> 'deleted' AND supervisor_member.project_id=%[2]s
			AND (supervisor_member.role_ids ?| ARRAY['supervisor','Giám sát'] OR EXISTS (SELECT 1 FROM roles canonical_supervisor WHERE canonical_supervisor.name='supervisor' AND supervisor_member.role_ids ? canonical_supervisor.id::text)))
	))`, userExpr, projectExpr)
}

var projectSupervisionPermissions = map[string]bool{
	"project.supervision.view": true, "project.view": true, "pipeline.view": true,
	"planning.view": true, "operation.view": true,
	"work_report.view": true, "work_report.review": true, "work_report.export": true,
}

func supervisionCompanyScopeAllowed(profile workItemAccessProfile, granted, required []string) bool {
	if !profile.CanSuperviseAllProjects {
		return false
	}
	for _, key := range required {
		if projectSupervisionPermissions[key] && hasAnyString(granted, []string{key}) {
			return true
		}
	}
	return false
}

// Separate from ProjectAccessFilter: financial/warehouse queries must not gain
// company scope merely because another role on the actor grants finance rights.
func (r *Repository) SupervisionProjectAccessFilter(ctx context.Context, userID, alias string, start int) (string, []any, error) {
	scope, args, err := r.ProjectAccessFilter(ctx, userID, alias, start)
	if err != nil {
		return "", nil, err
	}
	return fmt.Sprintf(`(%s OR (%s AND NOT %s.is_sample))`, scope, deputyManagerUserSQL(fmt.Sprintf("NULLIF($%d,'')::uuid", start+1)), alias), args, nil
}

func (r *Repository) CanSuperviseWorkReport(ctx context.Context, userID string, report Record) (bool, error) {
	if !boolValue(report["is_supervisor_report"]) {
		return false, nil
	}
	var allowed bool
	err := r.db.QueryRowContext(ctx, `SELECT `+deputyManagerUserSQL("$1::uuid")+` AND `+supervisedProjectSQL("$2::uuid"), userID, cleanID(report["project_id"])).Scan(&allowed)
	return allowed, err
}

// Commenting is distinct from executing or approving a Work Item.
func (r *Repository) CanCommentOnWorkItem(ctx context.Context, workItemID, userID string) (bool, error) {
	allowed, err := r.CanActOnWorkItem(ctx, workItemID, userID)
	if err != nil || allowed {
		return allowed, err
	}
	var supervised bool
	err = r.db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM work_items wi WHERE wi.id=$1::uuid AND wi.status <> 'cancelled' AND `+supervisedProjectSQL("wi.project_id")+` AND `+deputyManagerUserSQL("$2::uuid")+`)`, workItemID, userID).Scan(&supervised)
	return supervised, err
}

func (s *Service) requireSupervisionCommentScope(ctx context.Context, projectID, actorID string, req WorkItemActivityRequest) error {
	profile, err := s.repo.WorkItemAccessProfile(ctx, actorID)
	if err != nil {
		return err
	}
	if !profile.CanSuperviseAllProjects || profile.IsManager {
		return nil
	}
	inScope, err := s.repo.UserHasProjectScope(ctx, actorID, projectID)
	if err != nil {
		return err
	}
	if inScope {
		return nil
	}
	activityType := strings.TrimSpace(req.ActivityType)
	if activityType != "" && activityType != "comment" {
		return forbiddenError("Phó Quản Lý chỉ được trao đổi trong phạm vi theo dõi Giám sát")
	}
	if len(req.Attachments) > 0 {
		return forbiddenError("Minh chứng công việc phải do người được phân công cập nhật")
	}
	if req.ScopeType != "" && req.ScopeType != "project" && req.ScopeType != "work_item" && req.ScopeType != "node" && req.ScopeType != "step" {
		return forbiddenError("Phạm vi trao đổi không thuộc công việc Giám sát")
	}
	return nil
}

type SupervisionWorkFilter struct {
	ProjectID, SupervisorID, Status, Search string
	Limit, Offset                           int
}

func normalizeSupervisionWorkFilter(filter SupervisionWorkFilter) (SupervisionWorkFilter, error) {
	filter.ProjectID, filter.SupervisorID = cleanID(filter.ProjectID), cleanID(filter.SupervisorID)
	for key, id := range map[string]string{"project_id": filter.ProjectID, "supervisor_id": filter.SupervisorID} {
		if id != "" && !validActivityUUID(id) {
			return filter, validationError(key + " không hợp lệ")
		}
	}
	filter.Status, filter.Search = strings.TrimSpace(filter.Status), strings.TrimSpace(filter.Search)
	if filter.Status != "" && !workReportContains([]string{"pending", "ready", "in_progress", "waiting_evidence", "waiting_review", "waiting_approval", "needs_changes", "blocked", "completed"}, filter.Status) {
		return filter, validationError("Trạng thái công việc không hợp lệ")
	}
	if len([]rune(filter.Search)) > 120 {
		return filter, validationError("Từ khóa tìm kiếm không được vượt quá 120 ký tự")
	}
	if filter.Limit == 0 {
		filter.Limit = 25
	}
	if filter.Limit < 1 || filter.Limit > 50 || filter.Offset < 0 || filter.Offset > 10000 {
		return filter, validationError("Phân trang không hợp lệ")
	}
	return filter, nil
}

func supervisorWorkSQL(alias string) string {
	return fmt.Sprintf(`(%[1]s.role_ids ?| ARRAY['supervisor','Giám sát']
		OR EXISTS (SELECT 1 FROM roles canonical_supervisor WHERE canonical_supervisor.name='supervisor' AND %[1]s.role_ids ? canonical_supervisor.id::text)
		OR EXISTS (SELECT 1 FROM staff supervisor_assignee WHERE supervisor_assignee.id=%[1]s.assignee_staff_id AND supervisor_assignee.status='active' AND %[2]s)
		OR EXISTS (SELECT 1 FROM work_item_assignments supervision_work_assignment LEFT JOIN staff supervisor_assignment_staff ON supervisor_assignment_staff.id=supervision_work_assignment.staff_id AND supervisor_assignment_staff.status='active'
			WHERE supervision_work_assignment.work_item_id=%[1]s.id AND supervision_work_assignment.assignment_type <> 'watcher'
			AND (supervision_work_assignment.role_name IN ('supervisor','Giám sát') OR %[3]s)))`, alias,
		supervisorUserSQL("supervisor_assignee.user_id", alias+".project_id"), supervisorUserSQL("supervisor_assignment_staff.user_id", alias+".project_id"))
}

func (s *Service) SupervisionWork(ctx context.Context, actor string, filter SupervisionWorkFilter) (Record, error) {
	filter, err := normalizeSupervisionWorkFilter(filter)
	if err != nil {
		return nil, err
	}
	profile, err := s.repo.WorkItemAccessProfile(ctx, actor)
	if err != nil {
		return nil, err
	}
	if !profile.CanSuperviseAllProjects {
		return nil, forbiddenError("Bạn không có quyền theo dõi công việc Giám sát toàn công ty")
	}
	return s.repo.SupervisionWork(ctx, actor, filter)
}

func (r *Repository) SupervisionWork(ctx context.Context, actor string, filter SupervisionWorkFilter) (Record, error) {
	rows, err := r.Query(ctx, `SELECT wi.*,p.code AS project_code,p.name AS project_name,assignee.full_name AS assignee_name,
		rn.name AS run_node_name,rs.name AS run_step_name
		FROM work_items wi JOIN projects p ON p.id=wi.project_id
		LEFT JOIN staff assignee ON assignee.id=wi.assignee_staff_id
		LEFT JOIN pipeline_run_nodes rn ON rn.id=wi.run_node_id LEFT JOIN pipeline_run_steps rs ON rs.id=wi.run_step_id
		WHERE p.status <> 'cancelled' AND NOT p.is_sample AND wi.status <> 'cancelled'
		AND `+deputyManagerUserSQL("$1::uuid")+` AND `+supervisorWorkSQL("wi")+`
		AND ($2='' OR wi.project_id=NULLIF($2,'')::uuid)
		AND ($3='' OR assignee.user_id=NULLIF($3,'')::uuid OR EXISTS (SELECT 1 FROM work_item_assignments wa JOIN staff sa ON sa.id=wa.staff_id AND sa.status='active' WHERE wa.work_item_id=wi.id AND wa.assignment_type <> 'watcher' AND sa.user_id=NULLIF($3,'')::uuid))
		AND ($4='' OR wi.status=$4)
		AND ($5='' OR lower(concat_ws(' ',p.code,p.name,wi.title,assignee.full_name)) LIKE $5)
		ORDER BY wi.due_at NULLS LAST,wi.created_at,wi.id LIMIT $6 OFFSET $7`, actor, filter.ProjectID, filter.SupervisorID, filter.Status, supervisionSearchPattern(filter.Search), filter.Limit+1, filter.Offset)
	if err != nil {
		return nil, err
	}
	hasMore := len(rows) > filter.Limit
	if hasMore {
		rows = rows[:filter.Limit]
	}
	return Record{"items": rows, "has_more": hasMore}, nil
}

func supervisionSearchPattern(search string) string {
	if search == "" {
		return ""
	}
	return "%" + strings.ToLower(search) + "%"
}

func supervisionSupervisorOptionsSQL() string {
	// Start with role/member grants instead of testing every user against every
	// project. Keep the same classification as supervisorUserSQL, including the
	// requirement that a global Supervisor has at least one real active project.
	return `WITH canonical_supervisor AS (
		SELECT id,external_only FROM roles WHERE name='supervisor'
	), supervisor_candidates AS (
		SELECT assignment.user_id
		FROM user_roles assignment JOIN canonical_supervisor role ON role.id=assignment.role_id AND NOT role.external_only
		WHERE assignment.project_id IS NULL
		AND EXISTS (SELECT 1 FROM projects WHERE status <> 'cancelled' AND NOT is_sample)
		UNION
		SELECT assignment.user_id
		FROM user_roles assignment JOIN canonical_supervisor role ON role.id=assignment.role_id AND NOT role.external_only
		JOIN projects project ON project.id=assignment.project_id AND project.status <> 'cancelled' AND NOT project.is_sample
		UNION
		SELECT member_staff.user_id
		FROM project_members member JOIN staff member_staff ON member_staff.id=member.staff_id AND member_staff.status <> 'deleted'
		JOIN projects project ON project.id=member.project_id AND project.status <> 'cancelled' AND NOT project.is_sample
		WHERE member.active AND (
			member.role_ids ?| ARRAY['supervisor','Giám sát']
			OR EXISTS (SELECT 1 FROM canonical_supervisor role WHERE member.role_ids ? role.id::text)
		)
	)
	SELECT u.id::text,u.full_name FROM supervisor_candidates candidate JOIN users u ON u.id=candidate.user_id
	WHERE u.status='active' AND u.deleted_at IS NULL AND NOT u.is_sample
	AND ` + deputyManagerUserSQL("$1::uuid") + ` ORDER BY lower(u.full_name),u.id`
}

func (s *Service) SupervisionOptions(ctx context.Context, actor string) (Record, error) {
	profile, err := s.repo.WorkItemAccessProfile(ctx, actor)
	if err != nil {
		return nil, err
	}
	if !profile.CanSuperviseAllProjects {
		return nil, forbiddenError("Bạn không có quyền theo dõi Giám sát toàn công ty")
	}
	projects, err := s.repo.Query(ctx, `SELECT p.id::text,p.code,p.name FROM projects p WHERE p.status <> 'cancelled' AND NOT p.is_sample AND `+deputyManagerUserSQL("$1::uuid")+` ORDER BY lower(p.name),p.id`, actor)
	if err != nil {
		return nil, err
	}
	supervisors, err := s.repo.Query(ctx, supervisionSupervisorOptionsSQL(), actor)
	if err != nil {
		return nil, err
	}
	return Record{"projects": projects, "supervisors": supervisors}, nil
}

func (h *Handler) SupervisionWork(c *fiber.Ctx) error {
	limit, err := strconv.Atoi(c.Query("limit", "25"))
	if err != nil {
		return validationError("limit không hợp lệ")
	}
	offset, err := strconv.Atoi(c.Query("offset", "0"))
	if err != nil {
		return validationError("offset không hợp lệ")
	}
	rows, err := h.service.SupervisionWork(c.Context(), actorID(c), SupervisionWorkFilter{ProjectID: c.Query("project_id"), SupervisorID: c.Query("supervisor_id"), Status: c.Query("status"), Search: c.Query("search"), Limit: limit, Offset: offset})
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}

func (h *Handler) SupervisionOptions(c *fiber.Ctx) error {
	rows, err := h.service.SupervisionOptions(c.Context(), actorID(c))
	if err != nil {
		return err
	}
	return response.OK(c, rows)
}
