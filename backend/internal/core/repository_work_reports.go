package core

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

func sha256Hex(value []byte) string {
	digest := sha256.Sum256(value)
	return hex.EncodeToString(digest[:])
}

func (r *Repository) WorkReports(ctx context.Context, actorID string, filter WorkReportFilter) ([]Record, error) {
	return r.workReports(ctx, actorID, filter, 0)
}

// exportLimit is zero for the native list, positive for a bounded content export,
// and -1 for the existing full-filter register export.
func (r *Repository) workReports(ctx context.Context, actorID string, filter WorkReportFilter, exportLimit int) ([]Record, error) {
	scopeSQL, scopeArgs, err := r.ProjectAccessFilter(ctx, actorID, "project", 1)
	if err != nil {
		return nil, err
	}
	// Company observation is restricted to classified Supervisor reports even
	// when the actor also carries a separately granted report-management role.
	scopeSQL = "(" + scopeSQL + " OR (report.is_supervisor_report AND NOT project.is_sample AND " + deputyManagerUserSQL("NULLIF($2,'')::uuid") + "))"
	args := append([]any{}, scopeArgs...)
	conditions := []string{"project.status <> 'cancelled'", scopeSQL}
	addArg := func(value any) string {
		args = append(args, value)
		return fmt.Sprintf("$%d", len(args))
	}
	permissionActor := addArg(actorID)
	permissionPredicate := "permission.key='work_report.view'"
	if filter.Scope == "review" {
		permissionPredicate = "permission.key IN ('work_report.review','work_report.manage')"
	} else if filter.Scope == "all" {
		permissionPredicate = "permission.key='work_report.manage'"
	}
	conditions = append(conditions, `EXISTS (
		SELECT 1
		FROM user_roles permission_assignment
		JOIN role_permissions permission_link ON permission_link.role_id=permission_assignment.role_id
		JOIN permissions permission ON permission.id=permission_link.permission_id
		WHERE permission_assignment.user_id=`+permissionActor+`::uuid
		  AND (permission_assignment.project_id IS NULL OR permission_assignment.project_id=project.id)
		  AND `+permissionPredicate+`
	)`)
	statusExpression := `COALESCE(report.status,CASE WHEN source.report_delivery_eligible THEN 'draft' ELSE 'recorded' END)`
	switch filter.Scope {
	case "mine":
		conditions = append(conditions, "source.reported_by="+addArg(actorID)+"::uuid", "report.id IS NOT NULL")
	case "source":
		conditions = append(conditions, "source.reported_by="+addArg(actorID)+"::uuid", "source.report_delivery_eligible", "report.id IS NULL")
	case "review":
		reviewActor := addArg(actorID)
		conditions = append(conditions, "report.recipient_id="+reviewActor+"::uuid", "report.status IN ('submitted','acknowledged')", "NOT EXISTS (SELECT 1 FROM work_report_read_receipts receipt WHERE receipt.report_id=report.id AND receipt.revision_no=report.current_revision_no AND receipt.user_id="+reviewActor+"::uuid)")
	case "all":
		conditions = append(conditions, "report.id IS NOT NULL")
	case "supervision":
		profile, err := r.WorkItemAccessProfile(ctx, actorID)
		if err != nil {
			return nil, err
		}
		if !profile.CanSuperviseAllProjects {
			return nil, forbiddenError("Bạn không có quyền theo dõi báo cáo Giám sát")
		}
		conditions = append(conditions, "report.is_supervisor_report", "NOT project.is_sample")
	default:
		return nil, validationError("Phạm vi báo cáo không hợp lệ", Record{"scope": filter.Scope})
	}
	if filter.ProjectID != "" {
		conditions = append(conditions, "source.project_id="+addArg(filter.ProjectID)+"::uuid")
	}
	if filter.SupervisorID != "" {
		conditions = append(conditions, "source.reported_by="+addArg(filter.SupervisorID)+"::uuid")
	}
	viewerActor := addArg(actorID)
	viewerReceipt := `EXISTS (SELECT 1 FROM work_report_read_receipts receipt WHERE receipt.report_id=report.id AND receipt.revision_no=report.current_revision_no AND receipt.user_id=` + viewerActor + `::uuid)`
	if filter.ReadStatus == "read" {
		conditions = append(conditions, viewerReceipt)
	}
	if filter.ReadStatus == "unread" {
		conditions = append(conditions, "NOT "+viewerReceipt)
	}
	if filter.Status != "" {
		conditions = append(conditions, statusExpression+"="+addArg(filter.Status))
	}
	if filter.DateFrom != "" {
		conditions = append(conditions, "source.report_date >= "+addArg(filter.DateFrom)+"::date")
	}
	if filter.DateTo != "" {
		conditions = append(conditions, "source.report_date <= "+addArg(filter.DateTo)+"::date")
	}
	if filter.Search != "" {
		pattern := "%" + strings.ToLower(filter.Search) + "%"
		conditions = append(conditions, `lower(concat_ws(' ',project.code,project.name,author.full_name,recipient.full_name,source.weather,source.work_summary,source.issues_summary)) LIKE `+addArg(pattern))
	}
	pagination := ""
	if exportLimit > 0 {
		pagination = " LIMIT " + addArg(exportLimit)
	} else if exportLimit == 0 && filter.Scope == "supervision" {
		pagination = " LIMIT " + addArg(filter.Limit) + " OFFSET " + addArg(filter.Offset)
	}
	return r.Query(ctx, fmt.Sprintf(`
		SELECT
			source.id::text AS source_id,
			'operation_daily_report' AS source_kind,
			report.id::text AS work_report_id,
			report.is_supervisor_report,
			%s AS viewer_acknowledged,
			source.project_id::text AS project_id,
			project.code AS project_code,
			project.name AS project_name,
			'construction_daily' AS report_type,
			source.report_date,
			source.reported_by::text AS author_id,
			author.full_name AS author_name,
			COALESCE(author.deleted_at IS NOT NULL, false) AS author_account_deleted,
			report.recipient_id::text AS recipient_id,
			recipient.full_name AS recipient_name,
			COALESCE(recipient.deleted_at IS NOT NULL, false) AS recipient_account_deleted,
			%s AS status,
			COALESCE(report.current_revision_no,0) AS current_revision_no,
			report.submitted_at,
			report.reviewed_by::text AS reviewed_by,
			reviewer.full_name AS reviewed_by_name,
			COALESCE(reviewer.deleted_at IS NOT NULL, false) AS reviewed_by_account_deleted,
			report.reviewed_at,
			report.review_comment,
			source.report_delivery_eligible,
			source.weather,
			source.progress_percent,
			source.work_item_id::text AS work_item_id,
			source.workflow_form_instance_id::text AS workflow_form_instance_id,
			source.structured_json,
			source.work_summary,
			source.manpower_summary,
			source.material_usage_summary,
			source.issues_summary,
			source.photos,
			source.primary_evidence_id::text AS primary_evidence_id,
			source.created_at,
			COALESCE(report.updated_at,source.created_at) AS updated_at
		FROM operation_daily_reports source
		JOIN projects project ON project.id=source.project_id
		LEFT JOIN users author ON author.id=source.reported_by
		LEFT JOIN work_reports report ON report.source_kind='operation_daily_report' AND report.source_id=source.id
		LEFT JOIN users recipient ON recipient.id=report.recipient_id
		LEFT JOIN users reviewer ON reviewer.id=report.reviewed_by
		WHERE %s
		ORDER BY source.report_date DESC,source.created_at DESC,source.id
	%s`, viewerReceipt, statusExpression, strings.Join(conditions, " AND "), pagination), args...)
}

func (r *Repository) WorkReportSummary(ctx context.Context, actorID string, filter WorkReportFilter) (WorkReportSummary, error) {
	profile, err := r.WorkItemAccessProfile(ctx, actorID)
	if err != nil {
		return WorkReportSummary{}, err
	}
	if filter.Scope == "supervision" && !profile.CanSuperviseAllProjects {
		return WorkReportSummary{}, forbiddenError("Bạn không có quyền theo dõi báo cáo Giám sát")
	}
	scopeSQL, scopeArgs, err := r.ProjectAccessFilter(ctx, actorID, "project", 1)
	if err != nil {
		return WorkReportSummary{}, err
	}
	scopeSQL = "(" + scopeSQL + " OR (report.is_supervisor_report AND NOT project.is_sample AND " + deputyManagerUserSQL("NULLIF($2,'')::uuid") + "))"
	args := append([]any{}, scopeArgs...)
	addArg := func(value any) string {
		args = append(args, value)
		return fmt.Sprintf("$%d", len(args))
	}
	permissionActor := addArg(actorID)
	supervisionAllowed := addArg(profile.CanSuperviseAllProjects)
	filterConditions := []string{"TRUE"}
	if filter.SupervisorID != "" {
		filterConditions = append(filterConditions, "base.reported_by="+addArg(filter.SupervisorID)+"::uuid")
	}
	if filter.ReadStatus == "read" {
		filterConditions = append(filterConditions, "base.viewer_acknowledged")
	}
	if filter.ReadStatus == "unread" {
		filterConditions = append(filterConditions, "NOT base.viewer_acknowledged")
	}
	if filter.ProjectID != "" {
		filterConditions = append(filterConditions, "base.project_id="+addArg(filter.ProjectID)+"::uuid")
	}
	if filter.Status != "" {
		filterConditions = append(filterConditions, "base.status="+addArg(filter.Status))
	}
	if filter.DateFrom != "" {
		filterConditions = append(filterConditions, "base.report_date >= "+addArg(filter.DateFrom)+"::date")
	}
	if filter.DateTo != "" {
		filterConditions = append(filterConditions, "base.report_date <= "+addArg(filter.DateTo)+"::date")
	}
	if filter.Search != "" {
		filterConditions = append(filterConditions, "base.search_text LIKE "+addArg("%"+strings.ToLower(filter.Search)+"%"))
	}
	row, err := r.QueryOne(ctx, fmt.Sprintf(`
		WITH base AS (
			SELECT source.id,source.project_id,source.report_date,source.reported_by,
				report.id AS work_report_id,report.recipient_id,report.status AS formal_status,
				(report.is_supervisor_report AND NOT project.is_sample AND %s::boolean) AS supervision_candidate,
				EXISTS (SELECT 1 FROM work_report_read_receipts receipt WHERE receipt.report_id=report.id AND receipt.revision_no=report.current_revision_no AND receipt.user_id=%s::uuid) AS viewer_acknowledged,
				COALESCE(report.status,CASE WHEN source.report_delivery_eligible THEN 'draft' ELSE 'recorded' END) AS status,
				source.report_delivery_eligible,
				lower(concat_ws(' ',project.code,project.name,author.full_name,recipient.full_name,source.weather,source.work_summary,source.issues_summary)) AS search_text,
				EXISTS (
					SELECT 1 FROM user_roles assignment
					JOIN role_permissions link ON link.role_id=assignment.role_id
					JOIN permissions permission ON permission.id=link.permission_id
					WHERE assignment.user_id=%s::uuid
					  AND (assignment.project_id IS NULL OR assignment.project_id=project.id)
					  AND permission.key='work_report.view'
				) AS can_view,
				EXISTS (
					SELECT 1 FROM user_roles assignment
					JOIN role_permissions link ON link.role_id=assignment.role_id
					JOIN permissions permission ON permission.id=link.permission_id
					WHERE assignment.user_id=%s::uuid
					  AND (assignment.project_id IS NULL OR assignment.project_id=project.id)
					  AND permission.key IN ('work_report.review','work_report.manage')
				) AS can_review,
				EXISTS (
					SELECT 1 FROM user_roles assignment
					JOIN role_permissions link ON link.role_id=assignment.role_id
					JOIN permissions permission ON permission.id=link.permission_id
					WHERE assignment.user_id=%s::uuid
					  AND (assignment.project_id IS NULL OR assignment.project_id=project.id)
					  AND permission.key='work_report.manage'
				) AS can_manage
			FROM operation_daily_reports source
			JOIN projects project ON project.id=source.project_id
			LEFT JOIN users author ON author.id=source.reported_by
			LEFT JOIN work_reports report ON report.source_kind='operation_daily_report' AND report.source_id=source.id
			LEFT JOIN users recipient ON recipient.id=report.recipient_id
			WHERE project.status <> 'cancelled' AND %s
		), filtered AS (
			SELECT base.*,
				(can_view AND supervision_candidate) AS supervision_scope,
				(can_view AND reported_by=%s::uuid AND work_report_id IS NOT NULL) AS mine_scope,
				(can_view AND reported_by=%s::uuid AND report_delivery_eligible AND work_report_id IS NULL) AS source_scope,
				(can_review AND recipient_id=%s::uuid AND formal_status IN ('submitted','acknowledged') AND NOT viewer_acknowledged) AS review_scope,
				(can_manage AND work_report_id IS NOT NULL) AS all_scope
			FROM base
			WHERE %s
		)
		SELECT
			COUNT(*) FILTER (WHERE mine_scope) AS mine,
			COUNT(*) FILTER (WHERE source_scope) AS source,
			COUNT(*) FILTER (WHERE review_scope) AS review,
			COUNT(*) FILTER (WHERE all_scope) AS all_scope,
			COUNT(*) FILTER (WHERE supervision_scope AND NOT viewer_acknowledged) AS supervision,
			COUNT(*) FILTER (WHERE mine_scope OR source_scope OR review_scope OR all_scope OR supervision_scope) AS total_visible
		FROM filtered
	`, supervisionAllowed, permissionActor, permissionActor, permissionActor, permissionActor, scopeSQL,
		permissionActor, permissionActor, permissionActor, strings.Join(filterConditions, " AND ")), args...)
	if err != nil {
		return WorkReportSummary{}, err
	}
	return WorkReportSummary{
		Scopes: map[string]int{
			"mine": int(number(row["mine"])), "source": int(number(row["source"])),
			"review": int(number(row["review"])), "all": int(number(row["all_scope"])),
			"supervision": int(number(row["supervision"])),
		},
		TotalVisible: int(number(row["total_visible"])),
	}, nil
}

func (r *Repository) WorkReportSource(ctx context.Context, sourceID string) (Record, error) {
	row, err := r.QueryOne(ctx, `
		SELECT source.*,source.id::text AS source_id,source.project_id::text AS project_id,project.address AS project_address,
			source.reported_by::text AS author_id,project.code AS project_code,project.name AS project_name,
			author.full_name AS author_name,COALESCE(author.deleted_at IS NOT NULL, false) AS author_account_deleted,work_item.title AS work_item_title,
			report.id::text AS work_report_id,report.is_supervisor_report,report.recipient_id::text AS recipient_id,
			recipient.full_name AS recipient_name,COALESCE(recipient.deleted_at IS NOT NULL, false) AS recipient_account_deleted,report.status,report.current_revision_no,
			report.submitted_at,report.reviewed_by::text AS reviewed_by,reviewer.full_name AS reviewed_by_name,COALESCE(reviewer.deleted_at IS NOT NULL, false) AS reviewed_by_account_deleted,
			report.reviewed_at,report.review_comment,revision.snapshot_sha256 AS delivery_snapshot_sha256,revision.snapshot_json AS delivery_snapshot_json
		FROM operation_daily_reports source
		JOIN projects project ON project.id=source.project_id
		LEFT JOIN users author ON author.id=source.reported_by
		LEFT JOIN work_items work_item ON work_item.id=source.work_item_id
		LEFT JOIN work_reports report ON report.source_kind='operation_daily_report' AND report.source_id=source.id
		LEFT JOIN work_report_revisions revision ON revision.report_id=report.id AND revision.revision_no=report.current_revision_no
		LEFT JOIN users recipient ON recipient.id=report.recipient_id
		LEFT JOIN users reviewer ON reviewer.id=report.reviewed_by
		WHERE source.id=$1
	`, sourceID)
	if err != nil {
		return nil, err
	}
	if err := freezeNativeWorkReportEvidence(ctx, r.querySource(), row, false); err != nil {
		return nil, err
	}
	if err := annotateNativeWorkReportDeliverySourceChanged(ctx, r.querySource(), row); err != nil {
		return nil, err
	}
	return row, nil
}

func (r *Repository) WorkReportRecipients(ctx context.Context, projectID, actorID string) ([]Record, error) {
	return r.Query(ctx, `
		WITH eligible AS (
			SELECT user_account.id,user_account.full_name,
				COALESCE(MAX(staff.position),MIN(role.display_name)) AS position,
				EXISTS (
					SELECT 1
					FROM projects project
					JOIN staff project_pm ON project_pm.id=project.pm_staff_id AND project_pm.status <> 'deleted'
					WHERE project.id=$1::uuid AND project_pm.user_id=user_account.id
				) AS is_project_pm
			FROM users user_account
			JOIN user_roles assignment ON assignment.user_id=user_account.id
			JOIN roles role ON role.id=assignment.role_id AND NOT role.external_only
			JOIN role_permissions role_permission ON role_permission.role_id=role.id
			JOIN permissions permission ON permission.id=role_permission.permission_id
			LEFT JOIN staff ON staff.user_id=user_account.id AND staff.status <> 'deleted'
			WHERE user_account.status='active'
			  AND user_account.id <> $2::uuid
			  AND permission.key IN ('work_report.review','work_report.manage')
			  AND (
				role.name IN ('admin','director')
				OR assignment.project_id=$1::uuid
				OR EXISTS (
					SELECT 1
					FROM project_members member
					JOIN staff member_staff ON member_staff.id=member.staff_id
					WHERE member.project_id=$1::uuid AND member_staff.user_id=user_account.id AND member.active
				)
				  )
			GROUP BY user_account.id,user_account.full_name
		), summary AS (
			SELECT COUNT(*) AS recipient_count,COALESCE(bool_or(is_project_pm),false) AS has_project_pm
			FROM eligible
		)
		SELECT eligible.id::text AS id,eligible.full_name,eligible.position,
			(eligible.is_project_pm OR (NOT summary.has_project_pm AND summary.recipient_count=1)) AS is_recommended,
			CASE
				WHEN eligible.is_project_pm THEN 'project_pm'
				WHEN NOT summary.has_project_pm AND summary.recipient_count=1 THEN 'only_eligible_recipient'
				ELSE ''
			END AS recommendation_reason
		FROM eligible
		CROSS JOIN summary
		ORDER BY is_recommended DESC,eligible.full_name
	`, projectID, actorID)
}

func (r *Repository) WorkReportStatusForSource(ctx context.Context, sourceID string) (string, error) {
	return workReportStatusForSourceTx(ctx, r.db, sourceID)
}

func workReportStatusForSourceTx(ctx context.Context, db queryContext, sourceID string) (string, error) {
	row, err := queryOneRecord(ctx, db, `
		SELECT status
		FROM work_reports
		WHERE source_kind = 'operation_daily_report' AND source_id = $1
	`, sourceID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return fmt.Sprint(row["status"]), nil
}

func (r *Repository) WorkReportHistory(ctx context.Context, reportID string) ([]Record, []Record, error) {
	revisions, err := r.Query(ctx, `
		SELECT revision.id::text AS id,revision.revision_no,revision.snapshot_json,revision.snapshot_sha256,
			revision.submitted_by::text AS submitted_by,submitter.full_name AS submitted_by_name,COALESCE(submitter.deleted_at IS NOT NULL, false) AS submitted_by_account_deleted,
			revision.submitted_at,revision.comment
		FROM work_report_revisions revision
		LEFT JOIN users submitter ON submitter.id=revision.submitted_by
		WHERE revision.report_id=$1
		ORDER BY revision.revision_no DESC
	`, reportID)
	if err != nil {
		return nil, nil, err
	}
	events, err := r.Query(ctx, `
		SELECT event.id::text AS id,event.revision_no,event.event_type,event.actor_id::text AS actor_id,
			actor.full_name AS actor_name,COALESCE(actor.deleted_at IS NOT NULL, false) AS actor_account_deleted,event.comment,event.metadata_json,event.created_at
		FROM work_report_events event
		LEFT JOIN users actor ON actor.id=event.actor_id
		WHERE event.report_id=$1
		ORDER BY event.created_at,event.id
	`, reportID)
	return revisions, events, err
}

func (r *Repository) WorkReportHeader(ctx context.Context, reportID string) (Record, error) {
	return r.QueryOne(ctx, `SELECT id::text AS id,source_id::text AS source_id,project_id::text AS project_id,author_id::text AS author_id,recipient_id::text AS recipient_id,status,current_revision_no,is_supervisor_report FROM work_reports WHERE id=$1`, reportID)
}

func (r *Repository) WorkReportReadReceipts(ctx context.Context, reportID string) ([]Record, error) {
	return r.Query(ctx, `SELECT receipt.revision_no,receipt.user_id::text AS user_id,account.full_name AS user_name,COALESCE(account.deleted_at IS NOT NULL,false) AS user_account_deleted,receipt.acknowledged_at,receipt.comment
		FROM work_report_read_receipts receipt JOIN users account ON account.id=receipt.user_id
		WHERE receipt.report_id=$1 ORDER BY receipt.revision_no,receipt.acknowledged_at,receipt.user_id`, reportID)
}

func (r *Repository) WorkReportRevision(ctx context.Context, reportID string, revisionNo int) (Record, error) {
	return r.QueryOne(ctx, `
		SELECT revision.*,revision.report_id::text AS report_id,revision.submitted_by::text AS submitted_by,
			report.recipient_id::text AS recipient_id,recipient.full_name AS recipient_name,
			report.status,report.reviewed_by::text AS reviewed_by,reviewer.full_name AS reviewed_by_name,
			report.reviewed_at,report.review_comment
		FROM work_report_revisions revision
		JOIN work_reports report ON report.id=revision.report_id
		LEFT JOIN users recipient ON recipient.id=report.recipient_id
		LEFT JOIN users reviewer ON reviewer.id=report.reviewed_by
		WHERE revision.report_id=$1 AND revision.revision_no=$2
	`, reportID, revisionNo)
}

func (r *Repository) WorkReportEvidenceRecords(ctx context.Context, projectID, workItemID string, evidenceIDs []string) ([]Record, error) {
	if len(evidenceIDs) == 0 {
		return []Record{}, nil
	}
	rawEvidenceIDs, err := json.Marshal(evidenceIDs)
	if err != nil {
		return nil, err
	}
	return r.Query(ctx, `
		SELECT evidence.*,work_item.title AS work_item_title,work_item.status AS work_item_status,
			run_node.name AS run_node_name,
			submitter.full_name AS submitted_by_name,COALESCE(submitter.deleted_at IS NOT NULL,false) AS submitted_by_account_deleted,
			reviewer.full_name AS reviewed_by_name,COALESCE(reviewer.deleted_at IS NOT NULL,false) AS reviewed_by_account_deleted,
			document.name AS document_name
		FROM jsonb_array_elements_text($1::jsonb) WITH ORDINALITY requested(evidence_id,sort_order)
		JOIN evidence_records evidence ON evidence.id=requested.evidence_id::uuid
		JOIN work_items work_item ON work_item.id=evidence.work_item_id AND work_item.project_id=evidence.project_id
		LEFT JOIN pipeline_run_nodes run_node ON run_node.id=work_item.run_node_id
		LEFT JOIN users submitter ON submitter.id=evidence.submitted_by
		LEFT JOIN users reviewer ON reviewer.id=evidence.reviewed_by
		LEFT JOIN documents document ON document.id=evidence.document_id
		WHERE evidence.project_id=$2::uuid
		  AND (NULLIF($3,'')::uuid IS NULL OR evidence.work_item_id=NULLIF($3,'')::uuid)
		ORDER BY requested.sort_order
	`, string(rawEvidenceIDs), projectID, workItemID)
}

func (r *Repository) SubmitWorkReport(ctx context.Context, sourceID, recipientID, comment, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if _, err := r.submitWorkReportTx(ctx, tx, sourceID, recipientID, comment, actorID, actorName); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.WorkReportSource(ctx, sourceID)
}

func (r *Repository) submitWorkReportTx(ctx context.Context, tx *sql.Tx, sourceID, recipientID, comment, actorID, actorName string) (Record, error) {
	source, err := queryOneRecord(ctx, tx, `
		SELECT source.*,project.code AS project_code,project.name AS project_name,project.address AS project_address,author.full_name AS author_name,
			work_item.title AS work_item_title,`+supervisorUserSQL("source.reported_by", "source.project_id")+` AS is_supervisor_report
		FROM operation_daily_reports source
		JOIN projects project ON project.id=source.project_id AND project.status <> 'cancelled'
		LEFT JOIN users author ON author.id=source.reported_by
		LEFT JOIN work_items work_item ON work_item.id=source.work_item_id
		WHERE source.id=$1
		FOR UPDATE OF source
	`, sourceID)
	if err != nil {
		return nil, err
	}
	if !boolValue(source["report_delivery_eligible"]) {
		return nil, validationError("Báo cáo lịch sử chỉ được xem, không thể gửi như báo cáo mới")
	}
	if cleanID(source["reported_by"]) != actorID {
		return nil, forbiddenError("Chỉ người lập báo cáo mới có thể gửi báo cáo này")
	}
	var recipientAllowed bool
	if err := tx.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM users user_account
			JOIN user_roles assignment ON assignment.user_id=user_account.id
			JOIN roles role ON role.id=assignment.role_id AND NOT role.external_only
			JOIN role_permissions role_permission ON role_permission.role_id=role.id
			JOIN permissions permission ON permission.id=role_permission.permission_id
			WHERE user_account.id=$1::uuid AND user_account.status='active'
			  AND permission.key IN ('work_report.review','work_report.manage')
			  AND (
				role.name IN ('admin','director')
				OR assignment.project_id=$2::uuid
				OR EXISTS (
					SELECT 1
					FROM project_members member
					JOIN staff member_staff ON member_staff.id=member.staff_id
					WHERE member.project_id=$2::uuid AND member_staff.user_id=user_account.id AND member.active
				)
			  )
		)
	`, recipientID, cleanID(source["project_id"])).Scan(&recipientAllowed); err != nil {
		return nil, err
	}
	if !recipientAllowed || recipientID == actorID {
		return nil, validationError("Người nhận báo cáo không hợp lệ", Record{"recipient_id": recipientID})
	}
	var recipientName string
	if err := tx.QueryRowContext(ctx, `SELECT full_name FROM users WHERE id=$1::uuid`, recipientID).Scan(&recipientName); err != nil {
		return nil, err
	}
	if err := freezeNativeWorkReportEvidence(ctx, tx, source, true); err != nil {
		return nil, err
	}
	snapshot := workReportSnapshot(source)
	rawSnapshot, err := json.Marshal(snapshot)
	if err != nil {
		return nil, err
	}
	hash := sha256Hex(rawSnapshot)

	existing, existingErr := queryOneRecord(ctx, tx, `SELECT * FROM work_reports WHERE source_kind='operation_daily_report' AND source_id=$1 FOR UPDATE`, sourceID)
	isSupervisorReport := boolValue(source["is_supervisor_report"])
	revisionNo := 1
	reportID := ""
	eventType := "submitted"
	if existingErr == nil {
		isSupervisorReport = boolValue(existing["is_supervisor_report"])
		if fmt.Sprint(existing["status"]) != "needs_changes" {
			return nil, validationError("Báo cáo chỉ được gửi lại sau khi có yêu cầu bổ sung", Record{"status": existing["status"]})
		}
		reportID = cleanID(existing["id"])
		var previousJSON []byte
		if err := tx.QueryRowContext(ctx, `SELECT snapshot_json FROM work_report_revisions WHERE report_id=$1 AND revision_no=$2`, reportID, existing["current_revision_no"]).Scan(&previousJSON); err != nil {
			return nil, err
		}
		previous := Record{}
		if err = json.Unmarshal(previousJSON, &previous); err != nil {
			return nil, err
		}
		changed, err := workReportNativeSnapshotChanged(snapshot, previous)
		if err != nil {
			return nil, err
		}
		if !changed {
			return nil, validationError("Báo cáo chưa có thay đổi sau yêu cầu bổ sung", Record{"reason": "unchanged_since_change_request"})
		}
		revisionNo = int(number(existing["current_revision_no"])) + 1
		eventType = "resubmitted"
		if _, err := tx.ExecContext(ctx, `
			UPDATE work_reports
			SET recipient_id=$2::uuid,status='submitted',current_revision_no=$3,submitted_at=now(),
				reviewed_by=NULL,reviewed_at=NULL,review_comment=NULL,updated_at=now()
			WHERE id=$1
		`, reportID, recipientID, revisionNo); err != nil {
			return nil, err
		}
	} else if errors.Is(existingErr, sql.ErrNoRows) {
		if err := tx.QueryRowContext(ctx, `
			INSERT INTO work_reports
				(source_kind,source_id,project_id,report_type,report_date,author_id,recipient_id,status,current_revision_no,is_supervisor_report)
			VALUES ('operation_daily_report',$1,$2,'construction_daily',$3,$4,$5,'submitted',1,$6)
			RETURNING id::text
		`, sourceID, source["project_id"], source["report_date"], actorID, recipientID, isSupervisorReport).Scan(&reportID); err != nil {
			return nil, err
		}
	} else {
		return nil, existingErr
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO work_report_revisions
			(report_id,revision_no,snapshot_json,snapshot_sha256,submitted_by,comment)
		VALUES ($1,$2,$3::jsonb,$4,$5::uuid,NULLIF($6,''))
	`, reportID, revisionNo, rawSnapshot, hash, actorID, strings.TrimSpace(comment)); err != nil {
		return nil, err
	}
	var eventID string
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO work_report_events (report_id,revision_no,event_type,actor_id,comment,metadata_json)
		VALUES ($1,$2,$3,$4::uuid,NULLIF($5,''),$6::jsonb)
		RETURNING id::text
	`, reportID, revisionNo, eventType, actorID, strings.TrimSpace(comment), `{"recipient_id":"`+recipientID+`"}`).Scan(&eventID); err != nil {
		return nil, err
	}
	reportDateLabel := workReportDisplayDate(dateOnly(source["report_date"]))
	audienceIDs := []string{recipientID}
	if isSupervisorReport {
		rows, err := tx.QueryContext(ctx, `SELECT account.id::text FROM users account WHERE `+deputyManagerUserSQL("account.id")+` AND `+supervisedProjectSQL("$1::uuid"), cleanID(source["project_id"]))
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return nil, err
			}
			if !workReportContains(audienceIDs, id) {
				audienceIDs = append(audienceIDs, id)
			}
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, err
		}
		rows.Close()
	}
	audience := Record{"user_ids": audienceIDs, "exclude_actor": true}
	if eventType == "resubmitted" {
		audience["resolve_user_ids"] = []string{actorID}
	}
	notificationEventID, err := emitNotificationEventTx(ctx, tx, NotificationDomainEvent{
		Key:           "work_report." + eventType,
		AggregateType: "operation_daily_report",
		AggregateID:   sourceID,
		SourceEventID: eventID,
		ProjectID:     cleanID(source["project_id"]),
		ActorUserID:   actorID,
		Payload: Record{
			"source_id": sourceID, "work_report_id": reportID, "project_id": cleanID(source["project_id"]),
			"actor_name": actorName, "report_date": reportDateLabel, "project_name": fmt.Sprint(source["project_name"]),
		},
		Audience:       audience,
		CorrelationKey: "work_report:" + reportID,
		IdempotencyKey: "work_report_event:" + eventID,
	})
	if err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, cleanID(source["project_id"]), actorID, actorName, "work_report_"+eventType, "work_report", reportID, "Gửi báo cáo thi công", Record{"source_id": sourceID, "revision_no": revisionNo, "recipient_id": recipientID}); err != nil {
		return nil, err
	}
	return Record{
		"mode":                  "submit",
		"submitted":             true,
		"status":                "submitted",
		"work_report_id":        reportID,
		"revision_no":           revisionNo,
		"recipient_id":          recipientID,
		"recipient_name":        recipientName,
		"event_type":            eventType,
		"source_event_id":       eventID,
		"notification_event_id": notificationEventID,
		"project_id":            cleanID(source["project_id"]),
		"author_id":             actorID,
	}, nil
}

func (r *Repository) ReviewWorkReport(ctx context.Context, reportID, action, comment, actorID, actorName string, _ bool, expectedRevision ...int) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	report, err := queryOneRecord(ctx, tx, `SELECT * FROM work_reports WHERE id=$1 FOR UPDATE`, reportID)
	if err != nil {
		return nil, err
	}
	if len(expectedRevision) > 0 && expectedRevision[0] > 0 && expectedRevision[0] != int(number(report["current_revision_no"])) {
		return nil, validationError("Báo cáo đã có lần gửi mới. Vui lòng tải lại", Record{"reason": "stale_revision", "current_revision_no": report["current_revision_no"]})
	}
	var authorized bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS (
		SELECT 1 FROM users account
		JOIN projects project ON project.id=$2::uuid AND project.status <> 'cancelled'
		WHERE account.id=$1::uuid AND account.status='active' AND account.deleted_at IS NULL
		AND NOT EXISTS (SELECT 1 FROM user_roles external_assignment JOIN roles external_role ON external_role.id=external_assignment.role_id WHERE external_assignment.user_id=account.id AND external_role.external_only)
		AND EXISTS (SELECT 1 FROM user_roles assignment
			JOIN roles role ON role.id=assignment.role_id AND NOT role.external_only
			JOIN role_permissions link ON link.role_id=role.id
			JOIN permissions permission ON permission.id=link.permission_id
			WHERE assignment.user_id=account.id AND (assignment.project_id IS NULL OR assignment.project_id=project.id)
			AND permission.key IN ('work_report.review','work_report.manage')
			AND (
				($3::uuid=account.id OR permission.key='work_report.manage')
				AND (project.created_by=account.id OR assignment.project_id=project.id OR (assignment.project_id IS NULL AND role.name IN ('admin','director'))
					OR EXISTS (SELECT 1 FROM project_members member JOIN staff member_staff ON member_staff.id=member.staff_id AND member_staff.status='active' WHERE member.project_id=project.id AND member.active AND member_staff.user_id=account.id))
				OR ($4::boolean AND `+deputyManagerUserSQL("account.id")+` AND `+supervisedProjectSQL("project.id")+`)
			)
		)
	)`, actorID, cleanID(report["project_id"]), cleanID(report["recipient_id"]), boolValue(report["is_supervisor_report"])).Scan(&authorized); err != nil {
		return nil, err
	}
	if !authorized {
		return nil, forbiddenError("Bạn không có quyền xem xét báo cáo này")
	}
	if !workReportContains([]string{"submitted", "acknowledged"}, fmt.Sprint(report["status"])) {
		return nil, validationError("Báo cáo không ở trạng thái chờ xem", Record{"status": report["status"]})
	}
	status := map[string]string{"request-changes": "needs_changes", "acknowledge": "acknowledged"}[action]
	if status == "" {
		return nil, validationError("Hành động xem xét báo cáo không hợp lệ", Record{"action": action})
	}
	if action == "request-changes" && strings.TrimSpace(comment) == "" {
		return nil, validationError("Lý do yêu cầu bổ sung là bắt buộc", Record{"comment": "required"})
	}
	if action == "acknowledge" {
		result, err := tx.ExecContext(ctx, `INSERT INTO work_report_read_receipts (report_id,revision_no,user_id,comment)
			VALUES ($1,$2,$3::uuid,NULLIF($4,'')) ON CONFLICT (report_id,revision_no,user_id) DO NOTHING`, reportID, report["current_revision_no"], actorID, strings.TrimSpace(comment))
		if err != nil {
			return nil, err
		}
		inserted, err := result.RowsAffected()
		if err != nil {
			return nil, err
		}
		if inserted == 0 {
			if err := tx.Commit(); err != nil {
				return nil, err
			}
			return r.WorkReportSource(ctx, cleanID(report["source_id"]))
		}
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE work_reports
		SET status=$2,reviewed_by=$3::uuid,reviewed_at=now(),review_comment=NULLIF($4,''),updated_at=now()
		WHERE id=$1
	`, reportID, status, actorID, strings.TrimSpace(comment)); err != nil {
		return nil, err
	}
	eventType := map[string]string{"request-changes": "needs_changes", "acknowledge": "acknowledged"}[action]
	var eventID string
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO work_report_events (report_id,revision_no,event_type,actor_id,comment)
		VALUES ($1,$2,$3,$4::uuid,NULLIF($5,''))
		RETURNING id::text
	`, reportID, report["current_revision_no"], eventType, actorID, strings.TrimSpace(comment)).Scan(&eventID); err != nil {
		return nil, err
	}
	if _, err := emitNotificationEventTx(ctx, tx, NotificationDomainEvent{
		Key:           "work_report." + eventType,
		AggregateType: "operation_daily_report",
		AggregateID:   cleanID(report["source_id"]),
		SourceEventID: eventID,
		ProjectID:     cleanID(report["project_id"]),
		ActorUserID:   actorID,
		Payload: Record{
			"source_id": cleanID(report["source_id"]), "work_report_id": reportID,
			"project_id": cleanID(report["project_id"]), "actor_name": actorName,
		},
		Audience: Record{
			"user_ids": []string{cleanID(report["author_id"])}, "resolve_user_ids": []string{actorID}, "exclude_actor": true,
		},
		CorrelationKey: "work_report:" + reportID,
		IdempotencyKey: "work_report_event:" + eventID,
	}); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, cleanID(report["project_id"]), actorID, actorName, "work_report_"+eventType, "work_report", reportID, map[string]string{"request-changes": "Yêu cầu bổ sung báo cáo", "acknowledge": "Xác nhận đã xem báo cáo"}[action], Record{"revision_no": report["current_revision_no"], "comment": strings.TrimSpace(comment)}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.WorkReportSource(ctx, cleanID(report["source_id"]))
}

func workReportSnapshot(source Record) Record {
	return Record{
		"source_kind":               "operation_daily_report",
		"source_id":                 cleanID(source["id"]),
		"project_id":                cleanID(source["project_id"]),
		"project_code":              source["project_code"],
		"project_name":              source["project_name"],
		"address":                   source["project_address"],
		"report_type":               "construction_daily",
		"report_date":               source["report_date"],
		"author_id":                 cleanID(source["reported_by"]),
		"author_name":               source["author_name"],
		"weather":                   source["weather"],
		"progress_percent":          source["progress_percent"],
		"work_item_id":              cleanID(source["work_item_id"]),
		"work_item_title":           source["work_item_title"],
		"workflow_form_instance_id": cleanID(source["workflow_form_instance_id"]),
		"work_summary":              source["work_summary"],
		"manpower_summary":          source["manpower_summary"],
		"material_usage_summary":    source["material_usage_summary"],
		"issues_summary":            source["issues_summary"],
		"structured_json":           source["structured_json"],
		"photos":                    source["photos"],
		"primary_evidence_id":       cleanID(source["primary_evidence_id"]),
		"bm_gs_evidence":            source["bm_gs_evidence"],
	}
}

func annotateWorkReportDeliverySourceChanged(source Record) error {
	status := cleanText(source["delivery_status"])
	if status == "" {
		status = cleanText(source["status"])
	}
	source["delivery_source_changed"] = false
	if status == "needs_changes" {
		snapshot := workReportSnapshot(source)
		previous := recordFromAny(source["delivery_snapshot_json"])
		if len(previous) > 0 {
			changed, err := workReportNativeSnapshotChanged(snapshot, previous)
			if err != nil {
				return err
			}
			source["delivery_source_changed"] = changed
			delete(source, "delivery_snapshot_sha256")
			delete(source, "delivery_snapshot_json")
			return nil
		}
		rawSnapshot, err := json.Marshal(snapshot)
		if err != nil {
			return err
		}
		previousHash := strings.TrimSpace(fmt.Sprint(source["delivery_snapshot_sha256"]))
		source["delivery_source_changed"] = previousHash != "" && sha256Hex(rawSnapshot) != previousHash
	}
	delete(source, "delivery_snapshot_sha256")
	delete(source, "delivery_snapshot_json")
	return nil
}

// Compare native business facts without rewriting immutable old snapshots. The
// retired supplemental editor fields cannot constitute a report correction.
func workReportNativeSnapshotChanged(current, previous Record) (bool, error) {
	current = bmgsCopy(current)
	previous = bmgsCopy(previous)
	keys := []string{"bm_gs_json", "bm_gs_version", "bm_gs_updated_by", "bm_gs_updated_at"}
	for _, snapshot := range []Record{current, previous} {
		for _, key := range keys {
			delete(snapshot, key)
		}
		photos := workflowAnswerRecordList(snapshot["bm_gs_evidence"])
		cloned := make([]Record, 0, len(photos))
		for _, original := range photos {
			photo := bmgsCopy(original)
			for _, key := range keys {
				delete(photo, key)
			}
			cloned = append(cloned, photo)
		}
		if len(photos) > 0 {
			snapshot["bm_gs_evidence"] = cloned
		}
	}
	for _, key := range []string{"bm_gs_evidence", "address"} {
		if _, exists := previous[key]; !exists {
			delete(current, key)
		}
	}
	a, err := json.Marshal(current)
	if err != nil {
		return false, err
	}
	b, err := json.Marshal(previous)
	if err != nil {
		return false, err
	}
	return sha256Hex(a) != sha256Hex(b), nil
}
