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
	scopeSQL, scopeArgs, err := r.ProjectAccessFilter(ctx, actorID, "project", 1)
	if err != nil {
		return nil, err
	}
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
		conditions = append(conditions, "source.reported_by="+addArg(actorID)+"::uuid")
	case "review":
		conditions = append(conditions, "report.recipient_id="+addArg(actorID)+"::uuid", "report.status='submitted'")
	case "all":
	default:
		return nil, validationError("Phạm vi báo cáo không hợp lệ", Record{"scope": filter.Scope})
	}
	if filter.ProjectID != "" {
		conditions = append(conditions, "source.project_id="+addArg(filter.ProjectID)+"::uuid")
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
	return r.Query(ctx, fmt.Sprintf(`
		SELECT
			source.id::text AS source_id,
			'operation_daily_report' AS source_kind,
			report.id::text AS work_report_id,
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
	`, statusExpression, strings.Join(conditions, " AND ")), args...)
}

func (r *Repository) WorkReportSource(ctx context.Context, sourceID string) (Record, error) {
	return r.QueryOne(ctx, `
		SELECT source.*,source.id::text AS source_id,source.project_id::text AS project_id,
			source.reported_by::text AS author_id,project.code AS project_code,project.name AS project_name,
			author.full_name AS author_name,COALESCE(author.deleted_at IS NOT NULL, false) AS author_account_deleted,work_item.title AS work_item_title,
			report.id::text AS work_report_id,report.recipient_id::text AS recipient_id,
			recipient.full_name AS recipient_name,COALESCE(recipient.deleted_at IS NOT NULL, false) AS recipient_account_deleted,report.status,report.current_revision_no,
			report.submitted_at,report.reviewed_by::text AS reviewed_by,reviewer.full_name AS reviewed_by_name,COALESCE(reviewer.deleted_at IS NOT NULL, false) AS reviewed_by_account_deleted,
			report.reviewed_at,report.review_comment
		FROM operation_daily_reports source
		JOIN projects project ON project.id=source.project_id
		LEFT JOIN users author ON author.id=source.reported_by
		LEFT JOIN work_items work_item ON work_item.id=source.work_item_id
		LEFT JOIN work_reports report ON report.source_kind='operation_daily_report' AND report.source_id=source.id
		LEFT JOIN users recipient ON recipient.id=report.recipient_id
		LEFT JOIN users reviewer ON reviewer.id=report.reviewed_by
		WHERE source.id=$1
	`, sourceID)
}

func (r *Repository) WorkReportRecipients(ctx context.Context, projectID, actorID string) ([]Record, error) {
	return r.Query(ctx, `
		SELECT DISTINCT user_account.id::text AS id,user_account.full_name,
			COALESCE(staff.position,role.display_name) AS position
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
		ORDER BY user_account.full_name
	`, projectID, actorID)
}

func (r *Repository) WorkReportStatusForSource(ctx context.Context, sourceID string) (string, error) {
	var status string
	err := r.db.QueryRowContext(ctx, `
		SELECT status
		FROM work_reports
		WHERE source_kind = 'operation_daily_report' AND source_id = $1
	`, sourceID).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return status, err
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
	return r.QueryOne(ctx, `SELECT id::text AS id,source_id::text AS source_id,project_id::text AS project_id,author_id::text AS author_id,recipient_id::text AS recipient_id,status,current_revision_no FROM work_reports WHERE id=$1`, reportID)
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

func (r *Repository) SubmitWorkReport(ctx context.Context, sourceID, recipientID, comment, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	source, err := queryOneRecord(ctx, tx, `
		SELECT source.*,project.code AS project_code,project.name AS project_name,author.full_name AS author_name,
			work_item.title AS work_item_title
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

	existing, existingErr := queryOneRecord(ctx, tx, `SELECT * FROM work_reports WHERE source_kind='operation_daily_report' AND source_id=$1 FOR UPDATE`, sourceID)
	revisionNo := 1
	reportID := ""
	eventType := "submitted"
	if existingErr == nil {
		if fmt.Sprint(existing["status"]) != "needs_changes" {
			return nil, validationError("Báo cáo chỉ được gửi lại sau khi có yêu cầu bổ sung", Record{"status": existing["status"]})
		}
		reportID = cleanID(existing["id"])
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
				(source_kind,source_id,project_id,report_type,report_date,author_id,recipient_id,status,current_revision_no)
			VALUES ('operation_daily_report',$1,$2,'construction_daily',$3,$4,$5,'submitted',1)
			RETURNING id::text
		`, sourceID, source["project_id"], source["report_date"], actorID, recipientID).Scan(&reportID); err != nil {
			return nil, err
		}
	} else {
		return nil, existingErr
	}

	snapshot := workReportSnapshot(source)
	rawSnapshot, err := json.Marshal(snapshot)
	if err != nil {
		return nil, err
	}
	hash := sha256Hex(rawSnapshot)
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO work_report_revisions
			(report_id,revision_no,snapshot_json,snapshot_sha256,submitted_by,comment)
		VALUES ($1,$2,$3::jsonb,$4,$5::uuid,NULLIF($6,''))
	`, reportID, revisionNo, rawSnapshot, hash, actorID, strings.TrimSpace(comment)); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO work_report_events (report_id,revision_no,event_type,actor_id,comment,metadata_json)
		VALUES ($1,$2,$3,$4::uuid,NULLIF($5,''),$6::jsonb)
	`, reportID, revisionNo, eventType, actorID, strings.TrimSpace(comment), `{"recipient_id":"`+recipientID+`"}`); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, cleanID(source["project_id"]), actorID, actorName, "work_report_"+eventType, "work_report", reportID, "Gửi báo cáo thi công", Record{"source_id": sourceID, "revision_no": revisionNo, "recipient_id": recipientID}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.WorkReportSource(ctx, sourceID)
}

func (r *Repository) ReviewWorkReport(ctx context.Context, reportID, action, comment, actorID, actorName string, canManage bool) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	report, err := queryOneRecord(ctx, tx, `SELECT * FROM work_reports WHERE id=$1 FOR UPDATE`, reportID)
	if err != nil {
		return nil, err
	}
	if fmt.Sprint(report["status"]) != "submitted" {
		return nil, validationError("Báo cáo không ở trạng thái chờ xem", Record{"status": report["status"]})
	}
	if cleanID(report["recipient_id"]) != actorID && !canManage {
		return nil, forbiddenError("Báo cáo này không được gửi đến bạn")
	}
	status := map[string]string{"request-changes": "needs_changes", "acknowledge": "acknowledged"}[action]
	if status == "" {
		return nil, validationError("Hành động xem xét báo cáo không hợp lệ", Record{"action": action})
	}
	if action == "request-changes" && strings.TrimSpace(comment) == "" {
		return nil, validationError("Lý do yêu cầu bổ sung là bắt buộc", Record{"comment": "required"})
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE work_reports
		SET status=$2,reviewed_by=$3::uuid,reviewed_at=now(),review_comment=NULLIF($4,''),updated_at=now()
		WHERE id=$1
	`, reportID, status, actorID, strings.TrimSpace(comment)); err != nil {
		return nil, err
	}
	eventType := map[string]string{"request-changes": "needs_changes", "acknowledge": "acknowledged"}[action]
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO work_report_events (report_id,revision_no,event_type,actor_id,comment)
		VALUES ($1,$2,$3,$4::uuid,NULLIF($5,''))
	`, reportID, report["current_revision_no"], eventType, actorID, strings.TrimSpace(comment)); err != nil {
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
	}
}
