package core

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

func (r *Repository) DashboardReport(ctx context.Context, userID string) (DashboardProjectSnapshot, error) {
	scopeSQL, scopeArgs, err := r.ProjectAccessFilter(ctx, userID, "p", 1)
	if err != nil {
		return DashboardProjectSnapshot{}, err
	}
	data, err := r.QueryOne(ctx, fmt.Sprintf(`
		WITH scoped_projects AS MATERIALIZED (
			SELECT p.id, p.status, COALESCE(p.progress_percent, 0) AS progress_percent
			FROM projects p
			WHERE p.status <> 'cancelled' AND %s
		)
		SELECT
			(SELECT COUNT(*) FROM scoped_projects) AS total_projects,
			(SELECT COUNT(*) FROM scoped_projects WHERE status IN ('planning','in_progress','acceptance','handover')) AS active_projects,
			(SELECT COUNT(*) FROM scoped_projects WHERE status='completed') AS completed_projects,
			(SELECT COUNT(*) FROM scoped_projects WHERE status='blocked') AS blocked_projects,
			(SELECT COALESCE(AVG(progress_percent), 0) FROM scoped_projects) AS average_progress,
			(SELECT COUNT(*) FROM issues issue JOIN scoped_projects sp ON sp.id=issue.project_id WHERE issue.status IN ('open','in_progress')) AS open_issues,
			(SELECT COUNT(*) FROM approval_requests ar JOIN scoped_projects sp ON sp.id=ar.project_id WHERE ar.status='pending') AS pending_approvals,
			(SELECT COUNT(*) FROM scoped_projects WHERE status='planning') AS planning_projects,
			(SELECT COUNT(*) FROM scoped_projects WHERE status='in_progress') AS in_progress_projects,
			(SELECT COUNT(*) FROM scoped_projects WHERE status='acceptance') AS acceptance_projects,
			(SELECT COUNT(*) FROM scoped_projects WHERE status='handover') AS handover_projects
	`, scopeSQL), scopeArgs...)
	if err != nil {
		return DashboardProjectSnapshot{}, err
	}
	statusRows, err := r.Query(ctx, fmt.Sprintf(`
		SELECT p.status, COUNT(*) AS total
		FROM projects p
		WHERE p.status <> 'cancelled' AND %s
		GROUP BY p.status
		ORDER BY p.status
	`, scopeSQL), scopeArgs...)
	if err != nil {
		return DashboardProjectSnapshot{}, err
	}
	statusCounts := make([]DashboardStatusMetric, 0, len(statusRows))
	for _, row := range statusRows {
		statusCounts = append(statusCounts, DashboardStatusMetric{
			Status: strings.TrimSpace(fmt.Sprint(row["status"])),
			Total:  int(number(row["total"])),
		})
	}
	return DashboardProjectSnapshot{
		TotalProjects:     int(number(data["total_projects"])),
		ActiveProjects:    int(number(data["active_projects"])),
		CompletedProjects: int(number(data["completed_projects"])),
		BlockedProjects:   int(number(data["blocked_projects"])),
		OpenIssues:        int(number(data["open_issues"])),
		PendingApprovals:  int(number(data["pending_approvals"])),
		AverageProgress:   number(data["average_progress"]),
		StatusCounts:      statusCounts,
	}, nil
}

func (r *Repository) DashboardProjects(ctx context.Context, userID string, req DashboardProjectsRequest, meta DashboardPeriodMeta, financeVisible bool, offset int) (DashboardProjectsPage, error) {
	scopeSQL, scopeArgs, err := r.ProjectAccessFilter(ctx, userID, "p", 1)
	if err != nil {
		return DashboardProjectsPage{}, err
	}
	args := append([]any{}, scopeArgs...)
	conditions := []string{"p.status <> 'cancelled'", scopeSQL}
	if req.Search != "" {
		args = append(args, "%"+req.Search+"%")
		conditions = append(conditions, fmt.Sprintf("lower(btrim(p.code) || ' ' || btrim(p.name)) LIKE lower($%d)", len(args)))
	}
	if req.Status != "" {
		args = append(args, req.Status)
		conditions = append(conditions, fmt.Sprintf("p.status=$%d", len(args)))
	}

	financeSelect := `FALSE AS finance_visible,
		NULL::numeric AS inflow_amount,
		NULL::numeric AS outflow_amount,
		NULL::numeric AS net_amount`
	financeJoin := ""
	if financeVisible {
		args = append(args, userID, meta.FromDate, meta.ToDate)
		permissionIdx := len(args) - 2
		fromIdx := len(args) - 1
		toIdx := len(args)
		financeJoin = fmt.Sprintf(`
			LEFT JOIN LATERAL (
				SELECT
					TRUE AS finance_visible,
					COALESCE(SUM(CASE WHEN classified.direction='inflow' THEN classified.amount ELSE 0 END), 0) AS inflow_amount,
					COALESCE(SUM(CASE WHEN classified.direction='outflow' THEN classified.amount ELSE 0 END), 0) AS outflow_amount,
					COALESCE(SUM(CASE
						WHEN classified.direction='inflow' THEN classified.amount
						WHEN classified.direction='outflow' THEN -classified.amount
						ELSE 0
					END), 0) AS net_amount
				FROM (
					SELECT COALESCE(pay.amount, 0) AS amount, %s AS direction
					FROM payments pay
					WHERE pay.project_id=p.id
					  AND pay.status='approved'
					  AND %s >= $%d::date
					  AND %s < ($%d::date + INTERVAL '1 day')
				) classified
			) finance ON %s
		`, cashflowDirectionSQL("pay"), cashflowOccurredAtSQL(), fromIdx, cashflowOccurredAtSQL(), toIdx, reportFinancePermissionSQL("p", permissionIdx))
		financeSelect = `COALESCE(finance.finance_visible, FALSE) AS finance_visible,
		finance.inflow_amount,
		finance.outflow_amount,
		finance.net_amount`
	}

	orderColumn := "lower(project_page.name)"
	switch req.Sort {
	case "status":
		orderColumn = "project_page.status"
	case "progress":
		orderColumn = "project_page.progress_percent"
	case "net":
		orderColumn = "COALESCE(project_page.net_amount, 0)"
	}
	direction := "ASC"
	if req.Direction == "desc" {
		direction = "DESC"
	}
	args = append(args, req.Limit+1, offset)
	limitIdx := len(args) - 1
	offsetIdx := len(args)
	rows, err := r.Query(ctx, fmt.Sprintf(`
		WITH project_page AS MATERIALIZED (
			SELECT
				p.id,
				p.code,
				p.name,
				p.status,
				COALESCE(p.progress_percent, 0) AS progress_percent,
				(SELECT COUNT(*) FROM issues issue WHERE issue.project_id=p.id AND issue.status IN ('open','in_progress')) AS open_issues,
				(SELECT COUNT(*) FROM approval_requests approval WHERE approval.project_id=p.id AND approval.status='pending') AS pending_approvals,
				%s
			FROM projects p
			%s
			WHERE %s
		)
		SELECT project_page.*, COUNT(*) OVER() AS total_count
		FROM project_page
		ORDER BY %s %s, lower(project_page.name), project_page.id
		LIMIT $%d OFFSET $%d
	`, financeSelect, financeJoin, strings.Join(conditions, " AND "), orderColumn, direction, limitIdx, offsetIdx), args...)
	if err != nil {
		return DashboardProjectsPage{}, err
	}
	total := 0
	if len(rows) > 0 {
		total = int(number(rows[0]["total_count"]))
	}
	hasMore := len(rows) > req.Limit
	if hasMore {
		rows = rows[:req.Limit]
	}
	items := make([]DashboardProjectItem, 0, len(rows))
	for _, row := range rows {
		item := DashboardProjectItem{
			ID:               cleanID(row["id"]),
			Code:             strings.TrimSpace(fmt.Sprint(row["code"])),
			Name:             strings.TrimSpace(fmt.Sprint(row["name"])),
			Status:           strings.TrimSpace(fmt.Sprint(row["status"])),
			ProgressPercent:  number(row["progress_percent"]),
			OpenIssues:       int(number(row["open_issues"])),
			PendingApprovals: int(number(row["pending_approvals"])),
			FinanceVisible:   boolValue(row["finance_visible"]),
		}
		if item.FinanceVisible {
			inflow := number(row["inflow_amount"])
			outflow := number(row["outflow_amount"])
			net := number(row["net_amount"])
			item.InflowAmount = &inflow
			item.OutflowAmount = &outflow
			item.NetAmount = &net
		}
		items = append(items, item)
	}
	return DashboardProjectsPage{Items: items, Total: total, HasMore: hasMore}, nil
}

func (r *Repository) ReportProjectOptions(ctx context.Context, userID string, req ReportOptionsRequest, cursor *reportProjectOptionsCursor) ([]Record, Record, error) {
	return r.projectOptions(ctx, userID, req, cursor, "")
}

func (r *Repository) ProjectFinanceProjectOptions(ctx context.Context, userID string, req ReportOptionsRequest, cursor *reportProjectOptionsCursor) ([]Record, Record, error) {
	return r.projectOptions(ctx, userID, req, cursor, "accounting_handoff.view")
}

func (r *Repository) projectOptions(ctx context.Context, userID string, req ReportOptionsRequest, cursor *reportProjectOptionsCursor, requiredPermission string) ([]Record, Record, error) {
	scopeSQL, scopeArgs, err := r.ProjectAccessFilter(ctx, userID, "p", 1)
	if err != nil {
		return nil, nil, err
	}
	args := append([]any{}, scopeArgs...)
	conditions := []string{"p.status <> 'cancelled'", scopeSQL}
	if requiredPermission != "" {
		args = append(args, userID, requiredPermission)
		conditions = append(conditions, fmt.Sprintf(`EXISTS (
			SELECT 1
			FROM user_roles ur
			JOIN role_permissions rp ON rp.role_id=ur.role_id
			JOIN permissions permission ON permission.id=rp.permission_id
			WHERE ur.user_id=NULLIF($%d, '')::uuid
			  AND (ur.project_id IS NULL OR ur.project_id=p.id)
			  AND (permission.key=$%d OR ($%d='accounting_handoff.view' AND permission.key='project_finance.view'))
		)`, len(args)-1, len(args), len(args)))
	}
	if req.Search != "" {
		args = append(args, "%"+req.Search+"%")
		conditions = append(conditions, fmt.Sprintf("lower(btrim(p.code) || ' ' || btrim(p.name)) LIKE lower($%d)", len(args)))
	}
	if cursor != nil {
		args = append(args, cursor.NameKey, cursor.CodeKey, cursor.ID)
		conditions = append(conditions, fmt.Sprintf(`
			(lower(btrim(p.name)), lower(btrim(p.code)), p.id) >
			($%d, $%d, NULLIF($%d, '')::uuid)
		`, len(args)-2, len(args)-1, len(args)))
	}
	args = append(args, req.Limit+1)
	rows, err := r.Query(ctx, fmt.Sprintf(`
		SELECT p.id, p.code, p.name,
			lower(btrim(p.name)) AS name_key,
			lower(btrim(p.code)) AS code_key
		FROM projects p
		WHERE %s
		ORDER BY lower(btrim(p.name)), lower(btrim(p.code)), p.id
		LIMIT $%d
	`, strings.Join(conditions, " AND "), len(args)), args...)
	if err != nil {
		return nil, nil, err
	}
	var selected Record
	if req.SelectedID != "" {
		selectedArgs := append([]any{}, scopeArgs...)
		selectedConditions := []string{"p.status <> 'cancelled'", scopeSQL}
		if requiredPermission != "" {
			selectedArgs = append(selectedArgs, userID, requiredPermission)
			selectedConditions = append(selectedConditions, fmt.Sprintf(`EXISTS (
				SELECT 1
				FROM user_roles ur
				JOIN role_permissions rp ON rp.role_id=ur.role_id
				JOIN permissions permission ON permission.id=rp.permission_id
				WHERE ur.user_id=NULLIF($%d, '')::uuid
				  AND (ur.project_id IS NULL OR ur.project_id=p.id)
				  AND (permission.key=$%d OR ($%d='accounting_handoff.view' AND permission.key='project_finance.view'))
			)`, len(selectedArgs)-1, len(selectedArgs), len(selectedArgs)))
		}
		selectedArgs = append(selectedArgs, req.SelectedID)
		selected, err = r.QueryOne(ctx, fmt.Sprintf(`
			SELECT p.id, p.code, p.name
			FROM projects p
			WHERE %s
			  AND p.id=NULLIF($%d, '')::uuid
		`, strings.Join(selectedConditions, " AND "), len(selectedArgs)), selectedArgs...)
		if errors.Is(err, sql.ErrNoRows) {
			selected = nil
			err = nil
		}
		if err != nil {
			return nil, nil, err
		}
	}
	return rows, selected, nil
}

func (r *Repository) Report(ctx context.Context, kind, userID, projectID string) ([]Record, error) {
	return r.report(ctx, kind, userID, projectID, "")
}

func (r *Repository) StatisticsReport(ctx context.Context, kind, userID, projectID string) ([]Record, error) {
	return r.report(ctx, kind, userID, projectID, "statistics.view")
}

func (r *Repository) report(ctx context.Context, kind, userID, projectID, requiredPermission string) ([]Record, error) {
	scopeSQL, scopeArgs, err := r.ProjectAccessFilter(ctx, userID, "p", 1)
	if err != nil {
		return nil, err
	}
	args := append([]any{}, scopeArgs...)
	globalPermissionSQL := "TRUE"
	if requiredPermission != "" {
		args = append(args, userID, requiredPermission)
		userIndex := len(args) - 1
		permissionIndex := len(args)
		globalPermissionSQL = fmt.Sprintf(`EXISTS (
			SELECT 1
			FROM user_roles ur
			JOIN role_permissions rp ON rp.role_id=ur.role_id
			JOIN permissions permission ON permission.id=rp.permission_id
			WHERE ur.user_id=NULLIF($%d,'')::uuid
			  AND ur.project_id IS NULL
			  AND permission.key=$%d
		)`, userIndex, permissionIndex)
		scopeSQL = fmt.Sprintf(`(%s AND EXISTS (
			SELECT 1
			FROM user_roles ur
			JOIN role_permissions rp ON rp.role_id=ur.role_id
			JOIN permissions permission ON permission.id=rp.permission_id
			WHERE ur.user_id=NULLIF($%d,'')::uuid
			  AND (ur.project_id IS NULL OR ur.project_id=p.id)
			  AND permission.key=$%d
		))`, scopeSQL, userIndex, permissionIndex)
	}
	projectFilter := ""
	if projectID != "" {
		args = append(args, projectID)
		projectFilter = fmt.Sprintf(" AND p.id=$%d", len(args))
	}
	switch kind {
	case "projects":
		return r.Query(ctx, fmt.Sprintf(`
			SELECT p.id AS project_id, p.name, p.status, p.progress_percent, p.is_sample,
				(SELECT COUNT(*) FROM issues i WHERE i.project_id=p.id AND i.status IN ('open','in_progress')) AS open_issues,
				(SELECT COUNT(*) FROM approval_requests ar WHERE ar.project_id=p.id AND ar.status='pending') AS pending_approvals
			FROM projects p
			WHERE p.status <> 'cancelled' AND %s%s
			ORDER BY p.created_at DESC
		`, scopeSQL, projectFilter), args...)
	case "warehouse":
		movementScope := fmt.Sprintf(`(
			(sm.project_id IS NULL AND %s)
			OR EXISTS (
				SELECT 1 FROM projects p
				WHERE p.id=sm.project_id AND p.status <> 'cancelled' AND %s
			)
		)`, globalPermissionSQL, scopeSQL)
		balanceProjectFilter := ""
		if projectID != "" {
			projectPlaceholder := fmt.Sprintf("$%d", len(args))
			movementScope = fmt.Sprintf(`EXISTS (
				SELECT 1 FROM projects p
				WHERE p.id=sm.project_id AND p.id=%s AND p.status <> 'cancelled' AND %s
			)`, projectPlaceholder, scopeSQL)
			balanceProjectFilter = fmt.Sprintf(" AND p.id=%s", projectPlaceholder)
		}
		materialVisibility := "TRUE"
		if requiredPermission != "" {
			materialVisibility = fmt.Sprintf("(%s OR st.material_id IS NOT NULL OR site.material_id IS NOT NULL)", globalPermissionSQL)
		}
		return r.Query(ctx, fmt.Sprintf(`
			WITH stock_totals AS (
				SELECT sm.material_id, COALESCE(SUM(CASE
					WHEN sm.movement_type IN ('receive','return_to_warehouse','adjustment_in') THEN sm.quantity
					WHEN sm.movement_type IN ('issue','loss','adjustment_out') THEN -sm.quantity
					WHEN sm.movement_type='transfer' AND sm.target_warehouse_id IS NULL THEN -sm.quantity
					ELSE 0
				END),0) AS stock_quantity
				FROM stock_movements sm
				WHERE %s
				GROUP BY sm.material_id
			), site_totals AS (
				SELECT smb.material_id, COALESCE(SUM(smb.surplus_quantity),0) AS surplus_quantity
				FROM site_material_balances smb
				JOIN projects p ON p.id=smb.project_id
				WHERE p.status <> 'cancelled' AND %s%s
				GROUP BY smb.material_id
			)
			SELECT m.id AS material_id, m.name, m.category,
				COALESCE(st.stock_quantity,0) AS stock_quantity,
				COALESCE(site.surplus_quantity,0) AS surplus_quantity
			FROM materials m
			LEFT JOIN stock_totals st ON st.material_id=m.id
			LEFT JOIN site_totals site ON site.material_id=m.id
			WHERE m.active AND %s
			ORDER BY m.category, m.name
		`, movementScope, scopeSQL, balanceProjectFilter, materialVisibility), args...)
	case "procurement":
		return r.Query(ctx, fmt.Sprintf(`
			SELECT pr.status, pr.budget_status, COUNT(*) AS total, COALESCE(SUM(pr.total_estimated_amount),0) AS amount
			FROM procurement_requests pr
			JOIN projects p ON p.id=pr.project_id
			WHERE pr.status <> 'cancelled' AND p.status <> 'cancelled' AND %s%s
			GROUP BY pr.status, pr.budget_status
			ORDER BY pr.status
		`, scopeSQL, projectFilter), args...)
	case "accounting":
		return r.Query(ctx, fmt.Sprintf(`
			WITH payable_totals AS (
				SELECT project_id, COALESCE(SUM(remaining_amount),0) AS payable_remaining
				FROM payables WHERE status <> 'cancelled' GROUP BY project_id
			), receivable_totals AS (
				SELECT project_id, COALESCE(SUM(remaining_amount),0) AS receivable_remaining
				FROM receivables WHERE status <> 'cancelled' GROUP BY project_id
			), budget_totals AS (
				SELECT project_id,
					COALESCE(SUM(committed_amount),0) AS committed_budget,
					COALESCE(SUM(actual_amount),0) AS used_budget,
					COALESCE(SUM(paid_amount),0) AS paid_budget,
					COALESCE(SUM(available_amount),0) AS available_budget
				FROM accounting_budget_balances WHERE status <> 'cancelled' GROUP BY project_id
			)
			SELECT p.id AS project_id, p.name,
				COALESCE(pay.payable_remaining,0) AS payable_remaining,
				COALESCE(rec.receivable_remaining,0) AS receivable_remaining,
				COALESCE(b.committed_budget,0) AS committed_budget,
				COALESCE(b.used_budget,0) AS used_budget,
				COALESCE(b.paid_budget,0) AS paid_budget,
				COALESCE(b.available_budget,0) AS available_budget
			FROM projects p
			LEFT JOIN payable_totals pay ON pay.project_id=p.id
			LEFT JOIN receivable_totals rec ON rec.project_id=p.id
			LEFT JOIN budget_totals b ON b.project_id=p.id
			WHERE p.status <> 'cancelled' AND %s%s
			ORDER BY p.name
		`, scopeSQL, projectFilter), args...)
	case "staff":
		return r.Query(ctx, fmt.Sprintf(`
			SELECT s.id AS staff_id, s.full_name, s.department, s.position, COUNT(DISTINCT p.id) AS project_count
			FROM staff s
			JOIN project_members pm ON pm.staff_id=s.id AND pm.active
			JOIN projects p ON p.id=pm.project_id
			WHERE s.status <> 'deleted' AND p.status <> 'cancelled' AND %s%s
			GROUP BY s.id
			ORDER BY s.full_name
		`, scopeSQL, projectFilter), args...)
	default:
		return nil, errors.New("Báo cáo không hợp lệ")
	}
}
