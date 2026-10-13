package core

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

func (r *Repository) ProjectConstructionWorkspace(ctx context.Context, projectID string) (Record, error) {
	runtimeRows, err := r.Query(ctx, `
		SELECT pr.id AS pipeline_run_id,pr.status AS pipeline_run_status,COALESCE(pr.engine_version,1) AS engine_version,
			pr.current_node_id,rn.name AS current_node_name,rn.type AS current_node_type,
			rn.business_code AS current_node_business_code,rn.status AS current_node_status
		FROM pipeline_runs pr
		LEFT JOIN pipeline_run_nodes rn ON rn.id=pr.current_node_id
		WHERE pr.project_id=$1 AND pr.status <> 'cancelled'
		ORDER BY pr.started_at DESC,pr.id DESC
		LIMIT 1
	`, projectID)
	if err != nil {
		return nil, err
	}
	runtime := Record{
		"has_pipeline_run":    false,
		"construction_active": true,
	}
	if len(runtimeRows) > 0 {
		runtime = runtimeRows[0]
		runtime["has_pipeline_run"] = true
		engineVersion := int(number(runtime["engine_version"]))
		runtime["construction_active"] = engineVersion < 2 ||
			(fmt.Sprint(runtime["pipeline_run_status"]) == "in_progress" &&
				statusIn(fmt.Sprint(runtime["current_node_status"]), "in_progress", "waiting_approval", "blocked") &&
				(fmt.Sprint(runtime["current_node_business_code"]) == "construction_execution" ||
					fmt.Sprint(runtime["current_node_type"]) == "construction"))
	}
	templates, err := r.Query(ctx, `
		SELECT t.*,u.full_name AS created_by_name
		FROM construction_check_templates t
		LEFT JOIN users u ON u.id=t.created_by
		WHERE t.project_id=$1
		ORDER BY CASE WHEN t.status='active' THEN 0 ELSE 1 END,t.updated_at DESC,t.name
	`, projectID)
	if err != nil {
		return nil, err
	}
	checks, err := r.Query(ctx, `
		SELECT c.*,t.name AS template_name,w.title AS work_item_title,
			e.name AS evidence_name,e.evidence_type,
			rq.full_name AS requested_by_name,COALESCE(rq.deleted_at IS NOT NULL,false) AS requested_by_account_deleted,
			ck.full_name AS checked_by_name,COALESCE(ck.deleted_at IS NOT NULL,false) AS checked_by_account_deleted,
			dr.report_date AS source_report_date,dr.weather AS source_report_weather,
			hi.handoff_id,hi.source_field_key AS handoff_source_field_key,hi.source_item_index AS handoff_source_item_index,
			h.title AS handoff_title,h.revision_no AS handoff_revision_no,h.source_run_node_id,h.target_run_node_id,
			source_node.name AS handoff_source_node_name
		FROM construction_checks c
		LEFT JOIN construction_check_templates t ON t.id=c.template_id
		LEFT JOIN work_items w ON w.id=c.work_item_id
		LEFT JOIN evidence_records e ON e.id=c.evidence_id
		LEFT JOIN users rq ON rq.id=c.requested_by
		LEFT JOIN users ck ON ck.id=c.checked_by
		LEFT JOIN operation_daily_reports dr ON dr.id=c.source_daily_report_id
		LEFT JOIN construction_handoff_items hi ON hi.id=c.source_handoff_item_id
		LEFT JOIN construction_handoffs h ON h.id=hi.handoff_id
		LEFT JOIN pipeline_run_nodes source_node ON source_node.id=h.source_run_node_id
		WHERE c.project_id=$1 AND c.status <> 'cancelled'
		ORDER BY CASE WHEN c.status='pending' THEN 0 ELSE 1 END,c.scheduled_at NULLS LAST,c.created_at DESC
	`, projectID)
	if err != nil {
		return nil, err
	}
	evidenceRows, err := r.Query(ctx, `
		SELECT e.*,ce.check_id,ce.evidence_role,ce.sort_order
		FROM construction_check_evidence ce
		JOIN construction_checks c ON c.id=ce.check_id
		JOIN evidence_records e ON e.id=ce.evidence_id
		WHERE c.project_id=$1 AND c.status <> 'cancelled'
		ORDER BY ce.check_id,ce.evidence_role,ce.sort_order,ce.created_at,ce.id
	`, projectID)
	if err != nil {
		return nil, err
	}
	contextEvidenceByCheck := map[string][]Record{}
	resultEvidenceByCheck := map[string][]Record{}
	for _, evidence := range evidenceRows {
		checkID := cleanID(evidence["check_id"])
		if fmt.Sprint(evidence["evidence_role"]) == "context" {
			contextEvidenceByCheck[checkID] = append(contextEvidenceByCheck[checkID], evidence)
		} else {
			resultEvidenceByCheck[checkID] = append(resultEvidenceByCheck[checkID], evidence)
		}
	}
	for _, check := range checks {
		checkID := cleanID(check["id"])
		contextEvidence := contextEvidenceByCheck[checkID]
		if contextEvidence == nil {
			contextEvidence = []Record{}
		}
		resultEvidence := resultEvidenceByCheck[checkID]
		if resultEvidence == nil {
			resultEvidence = []Record{}
		}
		check["context_evidence"] = contextEvidence
		check["result_evidence"] = resultEvidence
	}
	actions, err := r.Query(ctx, `
		SELECT a.*,w.title AS work_item_title,c.title AS source_check_title,
			u.full_name AS assigned_user_name,COALESCE(u.deleted_at IS NOT NULL,false) AS assigned_user_account_deleted,
			b.name AS before_evidence_name,re.name AS resolution_evidence_name,
			rp.status AS impact_status,
			variation.id AS linked_variation_id,variation.variation_code AS linked_variation_code,
			variation.status AS linked_variation_status,variation.priority AS linked_variation_priority,
			COALESCE(a.source_daily_report_id,c.source_daily_report_id) AS source_daily_report_id,
			CASE WHEN a.source_daily_report_id IS NOT NULL THEN 'direct' WHEN c.source_daily_report_id IS NOT NULL THEN 'check' ELSE NULL END AS source_report_link_kind,
			dr.report_date AS source_report_date,dr.weather AS source_report_weather
		FROM construction_action_items a
		LEFT JOIN work_items w ON w.id=a.work_item_id
		LEFT JOIN construction_checks c ON c.id=a.source_check_id
		LEFT JOIN users u ON u.id=a.assigned_user_id
		LEFT JOIN evidence_records b ON b.id=a.before_evidence_id
		LEFT JOIN evidence_records re ON re.id=a.resolution_evidence_id
		LEFT JOIN replanning_requests rp ON rp.id=a.technical_replanning_id
		LEFT JOIN LATERAL (
			SELECT v.id,v.variation_code,v.status,v.priority
			FROM contract_variations v
			WHERE v.source_construction_action_id=a.id AND v.status <> 'rejected'
			ORDER BY v.created_at DESC,v.id DESC
			LIMIT 1
		) variation ON true
		LEFT JOIN operation_daily_reports dr ON dr.id=COALESCE(a.source_daily_report_id,c.source_daily_report_id)
		WHERE a.project_id=$1 AND a.status <> 'cancelled'
		ORDER BY CASE a.status WHEN 'open' THEN 0 WHEN 'waiting_confirmation' THEN 1 ELSE 2 END,
			a.due_at NULLS LAST,a.created_at DESC
	`, projectID)
	if err != nil {
		return nil, err
	}
	for _, action := range actions {
		events, err := r.Query(ctx, `
			SELECT ev.*,u.full_name AS actor_name,COALESCE(u.deleted_at IS NOT NULL, false) AS actor_account_deleted,e.name AS evidence_name
			FROM construction_action_events ev
			LEFT JOIN users u ON u.id=ev.actor_id
			LEFT JOIN evidence_records e ON e.id=ev.evidence_id
			WHERE ev.action_id=$1 ORDER BY ev.created_at,ev.id
		`, action["id"])
		if err != nil {
			return nil, err
		}
		action["events"] = events
	}
	workItems, err := r.Query(ctx, `
		SELECT wi.id,wi.title,wi.status,wi.work_type,wi.source_type,wi.source_id,wi.run_node_id,wi.run_step_id,
			COALESCE(pr.engine_version,1) AS pipeline_engine_version,
			rn.type AS run_node_type,rn.business_code AS run_node_business_code,
			rs.business_code AS run_step_business_code,
			CASE
				WHEN wi.status IN ('completed','done') THEN false
				WHEN wi.pipeline_run_id IS NULL OR COALESCE(pr.engine_version,1) < 2 THEN true
				ELSE pr.current_node_id=wi.run_node_id
					AND rn.status IN ('in_progress','waiting_approval','blocked')
					AND (
						rs.business_code='construction.item_reporting'
						OR rn.business_code='construction_execution'
						OR rn.type='construction'
					)
			END AS report_eligible
		FROM work_items wi
		LEFT JOIN pipeline_runs pr ON pr.id=wi.pipeline_run_id
		LEFT JOIN pipeline_run_nodes rn ON rn.id=wi.run_node_id
		LEFT JOIN pipeline_run_steps rs ON rs.id=wi.run_step_id
		WHERE wi.project_id=$1 AND wi.status <> 'cancelled'
		ORDER BY CASE WHEN wi.status IN ('ready','in_progress','needs_changes','blocked') THEN 0 ELSE 1 END,wi.updated_at DESC
		LIMIT 300
	`, projectID)
	if err != nil {
		return nil, err
	}
	participants, err := r.Query(ctx, `
		SELECT DISTINCT u.id,u.full_name,u.email
		FROM project_members pm
		JOIN staff s ON s.id=pm.staff_id
		JOIN users u ON u.id=s.user_id
		WHERE pm.project_id=$1 AND pm.active AND s.status='active' AND u.status='active'
		ORDER BY u.full_name
	`, projectID)
	if err != nil {
		return nil, err
	}
	return Record{"runtime": runtime, "templates": templates, "checks": checks, "actions": actions, "work_items": workItems, "participants": participants}, nil
}

func (r *Repository) CreateConstructionCheckTemplate(ctx context.Context, projectID, name, description string, criteria []ConstructionCheckCriterion, actorID, actorName string) (Record, error) {
	rawCriteria, _ := json.Marshal(criteria)
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := queryTx(ctx, tx, `
		INSERT INTO construction_check_templates (project_id,name,description,criteria_jsonb,created_by)
		VALUES ($1,$2,NULLIF($3,''),$4::jsonb,NULLIF($5,'')::uuid)
		RETURNING *
	`, projectID, name, description, string(rawCriteria), actorID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	row := rows[0]
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "construction_check_template_created", "construction_check_template", cleanID(row["id"]), "Tạo mẫu kiểm tra "+name, Record{"criteria_count": len(criteria)}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return row, nil
}

func (r *Repository) CreateConstructionCheck(ctx context.Context, projectID, workItemID, sourceDailyReportID, templateID, title, location, scheduledAt string, criteria []ConstructionCheckCriterion, criteriaProvided bool, contextEvidenceIDs []string, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	workItems, err := queryTx(ctx, tx, `SELECT id FROM work_items WHERE id=$1 AND project_id=$2 AND status <> 'cancelled'`, workItemID, projectID)
	if err != nil {
		return nil, err
	}
	if len(workItems) == 0 {
		return nil, validationError("Công việc liên quan không thuộc công trình")
	}
	if err := validateConstructionSourceReportTx(ctx, tx, projectID, workItemID, sourceDailyReportID); err != nil {
		return nil, err
	}
	criteriaJSON, err := constructionCheckCriteriaJSONTx(ctx, tx, projectID, templateID, criteria, criteriaProvided)
	if err != nil {
		return nil, err
	}
	for _, evidenceID := range contextEvidenceIDs {
		if err := validateConstructionEvidenceTx(ctx, tx, projectID, workItemID, evidenceID); err != nil {
			return nil, err
		}
	}
	rows, err := queryTx(ctx, tx, `
		INSERT INTO construction_checks (project_id,work_item_id,source_daily_report_id,template_id,title,location,criteria_jsonb,scheduled_at,requested_by)
		VALUES ($1,$2,NULLIF($3,'')::uuid,NULLIF($4,'')::uuid,$5,NULLIF($6,''),$7::jsonb,NULLIF($8,'')::timestamptz,NULLIF($9,'')::uuid)
		RETURNING *
	`, projectID, workItemID, sourceDailyReportID, templateID, title, location, criteriaJSON, scheduledAt, actorID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	row := rows[0]
	if err := replaceConstructionCheckEvidenceTx(ctx, tx, cleanID(row["id"]), "context", contextEvidenceIDs, actorID); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "construction_check_created", "construction_check", cleanID(row["id"]), "Tạo kiểm tra "+title, Record{"work_item_id": workItemID, "source_daily_report_id": sourceDailyReportID, "template_id": templateID, "scheduled_at": scheduledAt, "criteria_count": len(criteria), "context_evidence_count": len(contextEvidenceIDs)}); err != nil {
		return nil, err
	}
	transition, err := r.startWorkItemForDomainMutationTx(ctx, tx, projectID, linkedWorkItemMutationContext(workItemID), operationsWorkItemPolicy, "construction_check", cleanID(row["id"]), actorID, actorName)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return attachWorkItemTransition(row, transition), nil
}

func (r *Repository) UpdateConstructionCheck(ctx context.Context, checkID, workItemID, templateID, title, location, scheduledAt string, criteria []ConstructionCheckCriterion, criteriaProvided bool, contextEvidenceIDs []string, contextEvidenceProvided bool, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	checks, err := queryTx(ctx, tx, `SELECT * FROM construction_checks WHERE id=$1 FOR UPDATE`, checkID)
	if err != nil {
		return nil, err
	}
	if len(checks) == 0 {
		return nil, sql.ErrNoRows
	}
	check := checks[0]
	if fmt.Sprint(check["status"]) != "pending" {
		return nil, validationError("Chỉ được chỉnh sửa kiểm tra đang chờ", Record{"status": check["status"]})
	}
	if cleanID(check["technical_rfi_id"]) != "" {
		return nil, validationError("Kiểm tra đồng bộ từ RFI chỉ được chỉnh sửa tại hồ sơ QA/QC")
	}
	if cleanID(check["source_handoff_item_id"]) != "" {
		return nil, validationError("Kiểm tra từ kế hoạch chỉ được thay đổi bằng một revision bàn giao mới")
	}
	projectID := cleanID(check["project_id"])
	workItems, err := queryTx(ctx, tx, `SELECT id FROM work_items WHERE id=$1 AND project_id=$2 AND status <> 'cancelled'`, workItemID, projectID)
	if err != nil {
		return nil, err
	}
	if len(workItems) == 0 {
		return nil, validationError("Công việc liên quan không thuộc công trình")
	}
	criteriaJSON, err := constructionCheckCriteriaJSONTx(ctx, tx, projectID, templateID, criteria, criteriaProvided)
	if err != nil {
		return nil, err
	}
	if !criteriaProvided {
		raw, marshalErr := json.Marshal(check["criteria_jsonb"])
		if marshalErr != nil {
			return nil, marshalErr
		}
		criteriaJSON = string(raw)
	}
	if contextEvidenceProvided {
		for _, evidenceID := range contextEvidenceIDs {
			if err := validateConstructionEvidenceTx(ctx, tx, projectID, workItemID, evidenceID); err != nil {
				return nil, err
			}
		}
	} else if cleanID(check["work_item_id"]) != workItemID {
		existingEvidence, err := queryTx(ctx, tx, `SELECT evidence_id FROM construction_check_evidence WHERE check_id=$1 AND evidence_role='context'`, checkID)
		if err != nil {
			return nil, err
		}
		for _, evidence := range existingEvidence {
			if err := validateConstructionEvidenceTx(ctx, tx, projectID, workItemID, cleanID(evidence["evidence_id"])); err != nil {
				return nil, validationError("Khi đổi công việc liên quan, cần cập nhật lại ảnh hiện trạng")
			}
		}
	}
	rows, err := queryTx(ctx, tx, `
		UPDATE construction_checks
		SET work_item_id=$2,template_id=NULLIF($3,'')::uuid,title=$4,location=NULLIF($5,''),
			criteria_jsonb=$6::jsonb,scheduled_at=NULLIF($7,'')::timestamptz,updated_at=now()
		WHERE id=$1 RETURNING *
	`, checkID, workItemID, templateID, title, location, criteriaJSON, scheduledAt)
	if err != nil {
		return nil, err
	}
	if contextEvidenceProvided {
		if err := replaceConstructionCheckEvidenceTx(ctx, tx, checkID, "context", contextEvidenceIDs, actorID); err != nil {
			return nil, err
		}
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "construction_check_updated", "construction_check", checkID, "Cập nhật kiểm tra "+title, Record{
		"before": Record{"work_item_id": check["work_item_id"], "template_id": check["template_id"], "title": check["title"], "location": check["location"], "scheduled_at": check["scheduled_at"], "criteria": check["criteria_jsonb"]},
		"after":  Record{"work_item_id": workItemID, "template_id": templateID, "title": title, "location": location, "scheduled_at": scheduledAt, "criteria": rows[0]["criteria_jsonb"], "context_evidence_updated": contextEvidenceProvided, "context_evidence_count": len(contextEvidenceIDs)},
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return rows[0], nil
}

func (r *Repository) CompleteConstructionCheck(ctx context.Context, checkID, result, note string, evidenceIDs []string, actionTitle, assignedUserID, dueAt, impactScope, priority, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	checks, err := queryTx(ctx, tx, `SELECT * FROM construction_checks WHERE id=$1 FOR UPDATE`, checkID)
	if err != nil {
		return nil, err
	}
	if len(checks) == 0 {
		return nil, sql.ErrNoRows
	}
	check := checks[0]
	if fmt.Sprint(check["status"]) != "pending" {
		return nil, validationError("Kiểm tra này đã có kết quả", Record{"status": check["status"]})
	}
	if cleanID(check["source_handoff_item_id"]) != "" {
		workRows, queryErr := queryTx(ctx, tx, `SELECT status FROM work_items WHERE id=$1 AND project_id=$2`, check["work_item_id"], check["project_id"])
		if queryErr != nil {
			return nil, queryErr
		}
		if len(workRows) == 0 || !statusIn(fmt.Sprint(workRows[0]["status"]), "ready", "in_progress", "blocked", "needs_changes") {
			return nil, validationError("Hạng mục này chưa tới giai đoạn thi công", Record{"work_item_status": firstRecordValue(workRows, "status")})
		}
	}
	for _, evidenceID := range evidenceIDs {
		if err := validateConstructionEvidenceTx(ctx, tx, cleanID(check["project_id"]), cleanID(check["work_item_id"]), evidenceID); err != nil {
			return nil, err
		}
	}
	primaryEvidenceID := evidenceIDs[0]
	updated, err := queryTx(ctx, tx, `
		UPDATE construction_checks
		SET status='completed',result=$2,note=NULLIF($3,''),evidence_id=$4,checked_by=NULLIF($5,'')::uuid,checked_at=now(),updated_at=now()
		WHERE id=$1 RETURNING *
	`, checkID, result, note, primaryEvidenceID, actorID)
	if err != nil {
		return nil, err
	}
	if err := replaceConstructionCheckEvidenceTx(ctx, tx, checkID, "result", evidenceIDs, actorID); err != nil {
		return nil, err
	}
	responseRow := Record{"check": updated[0]}
	if result == "fail" {
		if actionTitle == "" {
			actionTitle = "Khắc phục: " + fmt.Sprint(check["title"])
		}
		action, err := createConstructionActionTx(ctx, tx, cleanID(check["project_id"]), cleanID(check["work_item_id"]), "", checkID, actionTitle, note, priority, assignedUserID, dueAt, impactScope, primaryEvidenceID, actorID, actorName)
		if err != nil {
			return nil, err
		}
		responseRow["action"] = action
	}
	if cleanID(check["source_handoff_item_id"]) != "" {
		workStatus := "completed"
		if result == "fail" {
			workStatus = "blocked"
		}
		if err := handoffWorkItemStatusTx(ctx, tx, cleanID(check["work_item_id"]), workStatus, actorID); err != nil {
			return nil, err
		}
	}
	if err := addAuditTx(ctx, tx, cleanID(check["project_id"]), actorID, actorName, "construction_check_completed", "construction_check", checkID, "Ghi nhận kiểm tra "+result, Record{"result": result, "evidence_id": primaryEvidenceID, "evidence_ids": evidenceIDs, "action_created": result == "fail"}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return responseRow, nil
}

func (r *Repository) CreateConstructionAction(ctx context.Context, projectID, workItemID, sourceDailyReportID, title, description, priority, assignedUserID, dueAt, impactScope, evidenceID, actorID, actorName string, fieldContexts ...Record) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	action, err := createConstructionActionTx(ctx, tx, projectID, workItemID, sourceDailyReportID, "", title, description, priority, assignedUserID, dueAt, impactScope, evidenceID, actorID, actorName)
	if err != nil {
		return nil, err
	}
	if len(fieldContexts) > 0 {
		action, err = saveConstructionActionFieldContextTx(ctx, tx, action, fieldContexts[0], actorID, actorName)
		if err != nil {
			return nil, err
		}
	}
	transition, err := r.startWorkItemForDomainMutationTx(ctx, tx, projectID, linkedWorkItemMutationContext(workItemID), operationsWorkItemPolicy, "construction_action", cleanID(action["id"]), actorID, actorName)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return attachWorkItemTransition(action, transition), nil
}

func (r *Repository) UpdateConstructionAction(ctx context.Context, actionID, title, description, priority, assignedUserID, dueAt, actorID, actorName string, fieldContexts ...Record) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	actions, err := queryTx(ctx, tx, `SELECT * FROM construction_action_items WHERE id=$1 FOR UPDATE`, actionID)
	if err != nil {
		return nil, err
	}
	if len(actions) == 0 {
		return nil, sql.ErrNoRows
	}
	action := actions[0]
	if fmt.Sprint(action["status"]) != "open" {
		return nil, validationError("Chỉ được chỉnh sửa việc đang cần xử lý", Record{"status": action["status"]})
	}
	if !constructionActionEditable(action) {
		return nil, validationError("Việc đồng bộ từ hồ sơ kỹ thuật chỉ được chỉnh sửa tại nguồn QA/QC")
	}
	projectID := cleanID(action["project_id"])
	if len(fieldContexts) > 0 && len(fieldContexts[0]) > 0 {
		allowed, e := r.withReadTransaction(tx).CanAccessProjectPermission(ctx, actorID, projectID, []string{"construction.action.edit"})
		if e != nil {
			return nil, e
		}
		if !allowed {
			return nil, forbiddenError("Bạn không có quyền chỉnh sửa việc cần xử lý")
		}
	}
	staffID, err := constructionProjectParticipantStaffIDTx(ctx, tx, projectID, assignedUserID)
	if err != nil {
		return nil, err
	}
	severity := "medium"
	if priority == "urgent" {
		severity = "high"
	}
	rows, err := queryTx(ctx, tx, `
		UPDATE construction_action_items
		SET title=$2,description=NULLIF($3,''),priority=$4,assigned_user_id=$5,due_at=$6::timestamptz,updated_at=now()
		WHERE id=$1 RETURNING *
	`, actionID, title, description, priority, assignedUserID, dueAt)
	if err != nil {
		return nil, err
	}
	if len(fieldContexts) > 0 {
		updated, e := saveConstructionActionFieldContextTx(ctx, tx, rows[0], fieldContexts[0], actorID, actorName)
		if e != nil {
			return nil, e
		}
		rows[0] = updated
	}
	if issueID := cleanID(action["technical_issue_id"]); issueID != "" {
		if _, err := tx.ExecContext(ctx, `UPDATE issues SET title=$2,description=NULLIF($3,''),severity=$4,assigned_to=$5,updated_at=now() WHERE id=$1`, issueID, title, description, severity, staffID); err != nil {
			return nil, err
		}
	}
	if reworkID := cleanID(action["technical_rework_id"]); reworkID != "" {
		if _, err := tx.ExecContext(ctx, `UPDATE rework_requests SET title=$2,description=NULLIF($3,''),assigned_to=$4,due_date=$5::timestamptz::date,updated_at=now() WHERE id=$1`, reworkID, title, description, staffID, dueAt); err != nil {
			return nil, err
		}
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "construction_action_updated", "construction_action", actionID, "Cập nhật việc cần xử lý "+title, Record{
		"before": Record{"title": action["title"], "description": action["description"], "priority": action["priority"], "assigned_user_id": action["assigned_user_id"], "due_at": action["due_at"]},
		"after":  Record{"title": title, "description": description, "priority": priority, "assigned_user_id": assignedUserID, "due_at": dueAt},
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return rows[0], nil
}

func constructionActionEditable(action Record) bool {
	if cleanID(action["technical_ncr_id"]) != "" {
		return false
	}
	issueID := cleanID(action["technical_issue_id"])
	return issueID == "" || cleanID(action["id"]) != issueID
}

func createConstructionActionTx(ctx context.Context, tx *sql.Tx, projectID, workItemID, sourceDailyReportID, sourceCheckID, title, description, priority, assignedUserID, dueAt, impactScope, evidenceID, actorID, actorName string) (Record, error) {
	workItems, err := queryTx(ctx, tx, `SELECT id,run_node_id FROM work_items WHERE id=$1 AND project_id=$2 AND status <> 'cancelled'`, workItemID, projectID)
	if err != nil {
		return nil, err
	}
	if len(workItems) == 0 {
		return nil, validationError("Công việc liên quan không thuộc công trình")
	}
	if err := validateConstructionSourceReportTx(ctx, tx, projectID, workItemID, sourceDailyReportID); err != nil {
		return nil, err
	}
	if err := validateConstructionEvidenceTx(ctx, tx, projectID, workItemID, evidenceID); err != nil {
		return nil, err
	}
	staffID, err := constructionProjectParticipantStaffIDTx(ctx, tx, projectID, assignedUserID)
	if err != nil {
		return nil, err
	}
	severity := "medium"
	if priority == "urgent" {
		severity = "high"
	}
	issues, err := queryTx(ctx, tx, `
		INSERT INTO issues (project_id,node_id,title,description,severity,status,reported_by,assigned_to,requires_rework,requires_replanning)
		VALUES ($1,NULLIF($2,'')::uuid,$3,NULLIF($4,''),$5,'open',NULLIF($6,'')::uuid,$7,true,$8)
		RETURNING *
	`, projectID, cleanID(workItems[0]["run_node_id"]), title, description, severity, actorID, staffID, impactScope != "none")
	if err != nil {
		return nil, err
	}
	issue := issues[0]
	reworks, err := queryTx(ctx, tx, `
		INSERT INTO rework_requests (project_id,issue_id,node_id,title,description,assigned_to,status,due_date,created_by)
		VALUES ($1,$2,NULLIF($3,'')::uuid,$4,NULLIF($5,''),$6,'pending',$7::timestamptz::date,NULLIF($8,'')::uuid)
		RETURNING *
	`, projectID, issue["id"], cleanID(workItems[0]["run_node_id"]), title, description, staffID, dueAt, actorID)
	if err != nil {
		return nil, err
	}
	replanningID := ""
	if impactScope != "none" {
		replannings, err := queryTx(ctx, tx, `
			INSERT INTO replanning_requests (project_id,issue_id,node_id,reason,impact_scope,budget_impact,timeline_impact,status,created_by)
			VALUES ($1,$2,NULLIF($3,'')::uuid,$4,$5,$6,$7,'pending',NULLIF($8,'')::uuid)
			RETURNING *
		`, projectID, issue["id"], cleanID(workItems[0]["run_node_id"]), title, impactScope, 0, 0, actorID)
		if err != nil {
			return nil, err
		}
		replanningID = cleanID(replannings[0]["id"])
	}
	actions, err := queryTx(ctx, tx, `
		INSERT INTO construction_action_items (
			project_id,work_item_id,source_daily_report_id,source_check_id,title,description,priority,assigned_user_id,due_at,status,
			impact_scope,before_evidence_id,technical_issue_id,technical_rework_id,technical_replanning_id,created_by
		) VALUES ($1,$2,NULLIF($3,'')::uuid,NULLIF($4,'')::uuid,$5,NULLIF($6,''),$7,$8,$9::timestamptz,'open',$10,$11,$12,$13,NULLIF($14,'')::uuid,NULLIF($15,'')::uuid)
		RETURNING *
	`, projectID, workItemID, sourceDailyReportID, sourceCheckID, title, description, priority, assignedUserID, dueAt, impactScope, evidenceID, issue["id"], reworks[0]["id"], replanningID, actorID)
	if err != nil {
		return nil, err
	}
	action := actions[0]
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO construction_action_events (action_id,event_type,comment,evidence_id,actor_id)
		VALUES ($1,'created',NULLIF($2,''),$3,NULLIF($4,'')::uuid)
	`, action["id"], description, evidenceID, actorID); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "construction_action_created", "construction_action", cleanID(action["id"]), "Tạo việc cần xử lý "+title, Record{"work_item_id": workItemID, "source_daily_report_id": sourceDailyReportID, "source_check_id": sourceCheckID, "impact_scope": impactScope, "technical_issue_id": issue["id"], "technical_rework_id": reworks[0]["id"], "technical_replanning_id": replanningID}); err != nil {
		return nil, err
	}
	return action, nil
}

func validateConstructionSourceReportTx(ctx context.Context, tx *sql.Tx, projectID, workItemID, sourceDailyReportID string) error {
	if sourceDailyReportID == "" {
		return nil
	}
	reports, err := queryTx(ctx, tx, `
		SELECT id,work_item_id
		FROM operation_daily_reports
		WHERE id=$1 AND project_id=$2
	`, sourceDailyReportID, projectID)
	if err != nil {
		return err
	}
	if len(reports) == 0 {
		return validationError("Báo cáo nguồn không thuộc công trình")
	}
	reportWorkItemID := cleanID(reports[0]["work_item_id"])
	if reportWorkItemID != "" && reportWorkItemID != workItemID {
		return validationError("Báo cáo nguồn và bản ghi hiện trường phải cùng công việc")
	}
	return nil
}

func constructionProjectParticipantStaffIDTx(ctx context.Context, tx *sql.Tx, projectID, assignedUserID string) (string, error) {
	participants, err := queryTx(ctx, tx, `
		SELECT s.id AS staff_id
		FROM project_members pm JOIN staff s ON s.id=pm.staff_id JOIN users u ON u.id=s.user_id
		WHERE pm.project_id=$1 AND pm.active AND s.status='active' AND u.status='active' AND u.id=$2
		LIMIT 1
	`, projectID, assignedUserID)
	if err != nil {
		return "", err
	}
	if len(participants) == 0 {
		return "", validationError("Người phụ trách không phải thành viên đang hoạt động của công trình")
	}
	return cleanID(participants[0]["staff_id"]), nil
}

func constructionCheckCriteriaJSONTx(ctx context.Context, tx *sql.Tx, projectID, templateID string, criteria []ConstructionCheckCriterion, criteriaProvided bool) (string, error) {
	templateCriteria := any([]any{})
	if templateID != "" {
		templates, err := queryTx(ctx, tx, `SELECT criteria_jsonb FROM construction_check_templates WHERE id=$1 AND project_id=$2 AND status='active'`, templateID, projectID)
		if err != nil {
			return "", err
		}
		if len(templates) == 0 {
			return "", validationError("Mẫu kiểm tra không còn hoạt động trong công trình")
		}
		templateCriteria = templates[0]["criteria_jsonb"]
	}
	value := templateCriteria
	if criteriaProvided {
		value = criteria
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func replaceConstructionCheckEvidenceTx(ctx context.Context, tx *sql.Tx, checkID, role string, evidenceIDs []string, actorID string) error {
	if !statusIn(role, "context", "result") {
		return validationError("Vai trò minh chứng kiểm tra không hợp lệ")
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM construction_check_evidence WHERE check_id=$1 AND evidence_role=$2`, checkID, role); err != nil {
		return err
	}
	for index, evidenceID := range evidenceIDs {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO construction_check_evidence (check_id,evidence_id,evidence_role,sort_order,created_by)
			VALUES ($1,$2,$3,$4,NULLIF($5,'')::uuid)
		`, checkID, evidenceID, role, index, actorID); err != nil {
			return err
		}
	}
	return nil
}

func validateConstructionEvidenceTx(ctx context.Context, tx *sql.Tx, projectID, workItemID, evidenceID string) error {
	rows, err := queryTx(ctx, tx, `
		SELECT id FROM evidence_records
		WHERE id=$1 AND project_id=$2 AND work_item_id=$3 AND status <> 'rejected'
	`, evidenceID, projectID, workItemID)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return validationError("Minh chứng phải thuộc cùng công việc và công trình")
	}
	return nil
}

func (r *Repository) SubmitConstructionActionResolution(ctx context.Context, actionID, evidenceID, note, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	actions, err := queryTx(ctx, tx, `SELECT * FROM construction_action_items WHERE id=$1 FOR UPDATE`, actionID)
	if err != nil {
		return nil, err
	}
	if len(actions) == 0 {
		return nil, sql.ErrNoRows
	}
	action := actions[0]
	if fmt.Sprint(action["status"]) != "open" {
		return nil, validationError("Việc này không ở trạng thái cần xử lý", Record{"status": action["status"]})
	}
	if err := validateConstructionEvidenceTx(ctx, tx, cleanID(action["project_id"]), cleanID(action["work_item_id"]), evidenceID); err != nil {
		return nil, err
	}
	rows, err := queryTx(ctx, tx, `
		UPDATE construction_action_items
		SET status='waiting_confirmation',resolution_evidence_id=$2,resolution_note=NULLIF($3,''),
			submitted_by=NULLIF($4,'')::uuid,submitted_at=now(),updated_at=now()
		WHERE id=$1 RETURNING *
	`, actionID, evidenceID, note, actorID)
	if err != nil {
		return nil, err
	}
	if cleanID(action["technical_issue_id"]) != "" {
		if _, err := tx.ExecContext(ctx, `UPDATE issues SET status='in_progress',updated_at=now() WHERE id=$1`, action["technical_issue_id"]); err != nil {
			return nil, err
		}
	}
	if cleanID(action["technical_rework_id"]) != "" {
		if _, err := tx.ExecContext(ctx, `UPDATE rework_requests SET status='in_progress',updated_at=now() WHERE id=$1`, action["technical_rework_id"]); err != nil {
			return nil, err
		}
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO construction_action_events (action_id,event_type,comment,evidence_id,actor_id) VALUES ($1,'resolution_submitted',NULLIF($2,''),$3,NULLIF($4,'')::uuid)`, actionID, note, evidenceID, actorID); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, cleanID(action["project_id"]), actorID, actorName, "construction_action_resolution_submitted", "construction_action", actionID, "Gửi kết quả khắc phục", Record{"evidence_id": evidenceID}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return rows[0], nil
}

func (r *Repository) VerifyConstructionAction(ctx context.Context, actionID, result, comment, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	actions, err := queryTx(ctx, tx, `SELECT * FROM construction_action_items WHERE id=$1 FOR UPDATE`, actionID)
	if err != nil {
		return nil, err
	}
	if len(actions) == 0 {
		return nil, sql.ErrNoRows
	}
	action := actions[0]
	if fmt.Sprint(action["status"]) != "waiting_confirmation" {
		return nil, validationError("Việc này chưa chờ xác nhận", Record{"status": action["status"]})
	}
	nextStatus := "done"
	eventType := "verified_pass"
	issueStatus := "closed"
	reworkStatus := "completed"
	if result == "fail" {
		nextStatus = "open"
		eventType = "verified_fail"
		issueStatus = "in_progress"
		reworkStatus = "in_progress"
	}
	rows, err := queryTx(ctx, tx, `
		UPDATE construction_action_items
		SET status=$2,verified_by=NULLIF($3,'')::uuid,verified_at=now(),retry_count=retry_count+CASE WHEN $2='open' THEN 1 ELSE 0 END,updated_at=now()
		WHERE id=$1 RETURNING *
	`, actionID, nextStatus, actorID)
	if err != nil {
		return nil, err
	}
	if cleanID(action["technical_issue_id"]) != "" {
		if _, err := tx.ExecContext(ctx, `UPDATE issues SET status=$2,updated_at=now() WHERE id=$1`, action["technical_issue_id"], issueStatus); err != nil {
			return nil, err
		}
	}
	if cleanID(action["technical_rework_id"]) != "" {
		if _, err := tx.ExecContext(ctx, `UPDATE rework_requests SET status=$2,approved_by=COALESCE(approved_by,NULLIF($3,'')::uuid),updated_at=now() WHERE id=$1`, action["technical_rework_id"], reworkStatus, actorID); err != nil {
			return nil, err
		}
	}
	if sourceCheckID := cleanID(action["source_check_id"]); sourceCheckID != "" {
		handoffChecks, queryErr := queryTx(ctx, tx, `SELECT source_handoff_item_id FROM construction_checks WHERE id=$1`, sourceCheckID)
		if queryErr != nil {
			return nil, queryErr
		}
		if len(handoffChecks) > 0 && cleanID(handoffChecks[0]["source_handoff_item_id"]) != "" {
			workStatus := "blocked"
			if result == "pass" {
				workStatus = "completed"
			}
			if err := handoffWorkItemStatusTx(ctx, tx, cleanID(action["work_item_id"]), workStatus, actorID); err != nil {
				return nil, err
			}
		}
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO construction_action_events (action_id,event_type,comment,evidence_id,actor_id) VALUES ($1,$2,NULLIF($3,''),$4,NULLIF($5,'')::uuid)`, actionID, eventType, comment, action["resolution_evidence_id"], actorID); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, cleanID(action["project_id"]), actorID, actorName, "construction_action_"+eventType, "construction_action", actionID, "Xác nhận kết quả khắc phục: "+result, Record{"result": result, "comment": comment, "retry_count": rows[0]["retry_count"]}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return rows[0], nil
}

func (r *Repository) ApproveConstructionActionImpact(ctx context.Context, actionID, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	actions, err := queryTx(ctx, tx, `SELECT * FROM construction_action_items WHERE id=$1 FOR UPDATE`, actionID)
	if err != nil {
		return nil, err
	}
	if len(actions) == 0 {
		return nil, sql.ErrNoRows
	}
	action := actions[0]
	replanningID := cleanID(action["technical_replanning_id"])
	if replanningID == "" {
		return nil, validationError("Việc này không có ảnh hưởng cần duyệt")
	}
	if _, err := tx.ExecContext(ctx, `UPDATE replanning_requests SET status='approved',approved_by=NULLIF($2,'')::uuid,updated_at=now() WHERE id=$1 AND status <> 'approved'`, replanningID, actorID); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO construction_action_events (action_id,event_type,comment,actor_id) VALUES ($1,'impact_approved','Đã xác nhận ảnh hưởng',NULLIF($2,'')::uuid)`, actionID, actorID); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, cleanID(action["project_id"]), actorID, actorName, "construction_action_impact_approved", "construction_action", actionID, "Xác nhận ảnh hưởng của việc cần xử lý", Record{"impact_scope": action["impact_scope"], "technical_replanning_id": replanningID}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.QueryOne(ctx, `SELECT a.*,rp.status AS impact_status FROM construction_action_items a LEFT JOIN replanning_requests rp ON rp.id=a.technical_replanning_id WHERE a.id=$1`, actionID)
}

func (r *Repository) ProjectQualityWorkspace(ctx context.Context, projectID string) (Record, error) {
	itps, err := r.Query(ctx, `
		SELECT p.*,COALESCE(items.item_count,0) AS item_count
		FROM quality_itp_plans p
		LEFT JOIN (SELECT itp_plan_id,COUNT(*) item_count FROM quality_itp_items GROUP BY itp_plan_id) items ON items.itp_plan_id=p.id
		WHERE p.project_id=$1 AND p.status <> 'rejected'
		ORDER BY p.itp_code,p.revision_no DESC
	`, projectID)
	if err != nil {
		return nil, err
	}
	for _, plan := range itps {
		items, err := r.Query(ctx, `SELECT * FROM quality_itp_items WHERE itp_plan_id=$1 ORDER BY sequence_no`, plan["id"])
		if err != nil {
			return nil, err
		}
		plan["items"] = items
	}
	rfis, err := r.Query(ctx, `
		SELECT rfi.*,p.itp_code,p.revision_no,i.sequence_no,i.activity_name,i.control_point,i.evidence_required,
			w.title AS work_item_title
		FROM quality_inspection_requests rfi
		JOIN quality_itp_plans p ON p.id=rfi.itp_plan_id
		JOIN quality_itp_items i ON i.id=rfi.itp_item_id
		LEFT JOIN work_items w ON w.id=rfi.work_item_id
		WHERE rfi.project_id=$1 AND rfi.status <> 'cancelled'
		ORDER BY rfi.requested_inspection_at DESC
	`, projectID)
	if err != nil {
		return nil, err
	}
	ncrs, err := r.Query(ctx, `
		SELECT n.*,rfi.rfi_code,rw.status AS rework_status
		FROM quality_ncrs n
		LEFT JOIN quality_inspection_requests rfi ON rfi.id=n.rfi_id
		LEFT JOIN rework_requests rw ON rw.id=n.rework_request_id
		WHERE n.project_id=$1 AND n.status <> 'cancelled'
		ORDER BY n.created_at DESC
	`, projectID)
	if err != nil {
		return nil, err
	}
	plans, err := r.Query(ctx, `
		SELECT rp.*,u.full_name AS assigned_user_name,
			COALESCE(u.deleted_at IS NOT NULL,false) AS assigned_user_account_deleted,
			COALESCE(stats.pending_count,0) AS pending_count,COALESCE(stats.overdue_count,0) AS overdue_count,COALESCE(stats.completed_count,0) AS completed_count
		FROM construction_recurring_plans rp
		LEFT JOIN users u ON u.id=rp.assigned_user_id
		LEFT JOIN (
			SELECT plan_id,COUNT(*) FILTER (WHERE status IN ('pending','in_progress')) pending_count,
				COUNT(*) FILTER (WHERE status='overdue' OR (status='pending' AND occurrence_date<CURRENT_DATE)) overdue_count,
				COUNT(*) FILTER (WHERE status='completed') completed_count
			FROM construction_recurring_occurrences GROUP BY plan_id
		) stats ON stats.plan_id=rp.id
		WHERE rp.project_id=$1 AND rp.status <> 'cancelled'
		ORDER BY rp.next_due_date,rp.title
	`, projectID)
	if err != nil {
		return nil, err
	}
	occurrences, err := r.Query(ctx, `
		SELECT o.*,p.plan_code,p.title AS plan_title,p.evidence_required,p.responsible_role,u.full_name AS assigned_user_name,
			COALESCE(u.deleted_at IS NOT NULL,false) AS assigned_user_account_deleted,
			CASE WHEN o.status='pending' AND o.occurrence_date<CURRENT_DATE THEN 'overdue' ELSE o.status END AS effective_status
		FROM construction_recurring_occurrences o
		JOIN construction_recurring_plans p ON p.id=o.plan_id
		LEFT JOIN users u ON u.id=o.assigned_user_id
		WHERE o.project_id=$1
		ORDER BY o.occurrence_date DESC,o.created_at DESC
		LIMIT 200
	`, projectID)
	if err != nil {
		return nil, err
	}
	evidenceOptions, err := r.Query(ctx, `
		SELECT e.id,e.work_item_id,e.name,e.evidence_type,e.status,e.created_at,w.title AS work_item_title
		FROM evidence_records e LEFT JOIN work_items w ON w.id=e.work_item_id
		WHERE e.project_id=$1 AND e.status <> 'rejected'
		ORDER BY e.created_at DESC LIMIT 200
	`, projectID)
	if err != nil {
		return nil, err
	}
	workItems, err := r.Query(ctx, `SELECT id,title,status,work_type FROM work_items WHERE project_id=$1 AND status <> 'cancelled' ORDER BY updated_at DESC LIMIT 200`, projectID)
	if err != nil {
		return nil, err
	}
	participants, err := r.Query(ctx, `
		SELECT DISTINCT u.id,u.full_name,u.email
		FROM project_members pm JOIN staff s ON s.id=pm.staff_id JOIN users u ON u.id=s.user_id
		WHERE pm.project_id=$1 AND pm.active AND s.status='active' AND u.status='active'
		ORDER BY u.full_name
	`, projectID)
	if err != nil {
		return nil, err
	}
	return Record{"itp_plans": itps, "inspection_requests": rfis, "ncrs": ncrs, "recurring_plans": plans, "recurring_occurrences": occurrences, "evidence_options": evidenceOptions, "work_items": workItems, "participants": participants}, nil
}

func (r *Repository) CreateQualityITP(ctx context.Context, projectID string, header Record, items []Record, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	hashPayload, _ := json.Marshal(Record{"project_id": projectID, "itp_code": header["itp_code"], "title": header["title"], "discipline": header["discipline"], "work_package": header["work_package"], "items": items})
	hash := sha256.Sum256(hashPayload)
	rows, err := queryTx(ctx, tx, `
		INSERT INTO quality_itp_plans (project_id,family_id,itp_code,title,discipline,work_package,revision_no,definition_hash,status,review_comment,document_id,created_by)
		VALUES ($1,gen_random_uuid(),$2,$3,$4,$5,1,$6,'draft',$7,NULLIF($8,'')::uuid,NULLIF($9,'')::uuid) RETURNING *
	`, projectID, header["itp_code"], header["title"], header["discipline"], header["work_package"], hex.EncodeToString(hash[:]), header["review_comment"], cleanID(header["document_id"]), actorID)
	if err != nil {
		return nil, err
	}
	planID := cleanID(rows[0]["id"])
	for _, item := range items {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO quality_itp_items (itp_plan_id,sequence_no,activity_name,inspection_stage,acceptance_criteria,inspection_method,control_point,frequency_type,frequency_value,responsible_role,evidence_required,client_witness_required,notes)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
		`, planID, item["sequence_no"], item["activity_name"], item["inspection_stage"], item["acceptance_criteria"], item["inspection_method"], item["control_point"], item["frequency_type"], item["frequency_value"], item["responsible_role"], item["evidence_required"], item["client_witness_required"], item["notes"]); err != nil {
			return nil, err
		}
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "quality_itp_created", "quality_itp", planID, "Tạo ITP revision 1", Record{"itp_code": header["itp_code"], "item_count": len(items), "definition_hash": hex.EncodeToString(hash[:])}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.ProjectQualityWorkspace(ctx, projectID)
}

func (r *Repository) ReviseQualityITP(ctx context.Context, itpID, comment, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	plans, err := queryTx(ctx, tx, `SELECT * FROM quality_itp_plans WHERE id=$1 AND status IN ('active','approved') FOR UPDATE`, itpID)
	if err != nil {
		return nil, err
	}
	if len(plans) == 0 {
		return nil, validationError("Chỉ ITP approved/active mới được tạo revision")
	}
	plan := plans[0]
	items, err := queryTx(ctx, tx, `SELECT * FROM quality_itp_items WHERE itp_plan_id=$1 ORDER BY sequence_no`, itpID)
	if err != nil {
		return nil, err
	}
	nextRevision := int(number(plan["revision_no"])) + 1
	hashPayload, _ := json.Marshal(Record{"itp_code": plan["itp_code"], "revision_no": nextRevision, "title": plan["title"], "discipline": plan["discipline"], "work_package": plan["work_package"], "items": items})
	hash := sha256.Sum256(hashPayload)
	rows, err := queryTx(ctx, tx, `
		INSERT INTO quality_itp_plans (project_id,family_id,itp_code,title,discipline,work_package,revision_no,supersedes_id,definition_hash,status,review_comment,document_id,created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,'draft',$10,$11,NULLIF($12,'')::uuid) RETURNING id
	`, plan["project_id"], plan["family_id"], plan["itp_code"], plan["title"], plan["discipline"], plan["work_package"], nextRevision, itpID, hex.EncodeToString(hash[:]), comment, plan["document_id"], actorID)
	if err != nil {
		return nil, err
	}
	newID := cleanID(rows[0]["id"])
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO quality_itp_items (itp_plan_id,sequence_no,activity_name,inspection_stage,acceptance_criteria,inspection_method,control_point,frequency_type,frequency_value,responsible_role,evidence_required,client_witness_required,notes)
		SELECT $2,sequence_no,activity_name,inspection_stage,acceptance_criteria,inspection_method,control_point,frequency_type,frequency_value,responsible_role,evidence_required,client_witness_required,notes FROM quality_itp_items WHERE itp_plan_id=$1
	`, itpID, newID); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, cleanID(plan["project_id"]), actorID, actorName, "quality_itp_revised", "quality_itp", newID, "Tạo revision ITP mới", Record{"supersedes_id": itpID, "revision_no": nextRevision, "comment": comment}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.ProjectQualityWorkspace(ctx, cleanID(plan["project_id"]))
}

func (r *Repository) QualityITPAction(ctx context.Context, itpID, action, comment, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := queryTx(ctx, tx, `SELECT * FROM quality_itp_plans WHERE id=$1 FOR UPDATE`, itpID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	plan := rows[0]
	fromStatus := fmt.Sprint(plan["status"])
	toStatus := ""
	eventType := ""
	switch action {
	case "submit":
		if fromStatus != "draft" {
			return nil, validationError("Chỉ ITP nháp mới được gửi review")
		}
		var itemCount int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM quality_itp_items WHERE itp_plan_id=$1`, itpID).Scan(&itemCount); err != nil {
			return nil, err
		}
		if itemCount == 0 {
			return nil, validationError("ITP cần ít nhất một inspection item")
		}
		toStatus, eventType = "review", "quality_itp_submitted"
	case "approve":
		if fromStatus != "review" {
			return nil, validationError("Chỉ ITP đang review mới được duyệt")
		}
		toStatus, eventType = "approved", "quality_itp_approved"
	case "activate":
		if fromStatus != "approved" {
			return nil, validationError("Chỉ ITP đã duyệt mới được kích hoạt")
		}
		if _, err := tx.ExecContext(ctx, `UPDATE quality_itp_plans SET status='superseded',updated_at=now() WHERE family_id=$1 AND status='active' AND id<>$2`, plan["family_id"], itpID); err != nil {
			return nil, err
		}
		toStatus, eventType = "active", "quality_itp_activated"
	default:
		return nil, validationError("Hành động ITP không hợp lệ", Record{"action": action})
	}
	_, err = tx.ExecContext(ctx, `
		UPDATE quality_itp_plans SET status=$2,review_comment=concat_ws(E'\n',NULLIF(review_comment,''),NULLIF($3,'')),
			submitted_by=CASE WHEN $4='submit' THEN NULLIF($5,'')::uuid ELSE submitted_by END,submitted_at=CASE WHEN $4='submit' THEN now() ELSE submitted_at END,
			approved_by=CASE WHEN $4='approve' THEN NULLIF($5,'')::uuid ELSE approved_by END,approved_at=CASE WHEN $4='approve' THEN now() ELSE approved_at END,
			activated_by=CASE WHEN $4='activate' THEN NULLIF($5,'')::uuid ELSE activated_by END,activated_at=CASE WHEN $4='activate' THEN now() ELSE activated_at END,updated_at=now()
		WHERE id=$1
	`, itpID, toStatus, comment, action, actorID)
	if err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, cleanID(plan["project_id"]), actorID, actorName, eventType, "quality_itp", itpID, fmt.Sprintf("Chuyển ITP từ %s sang %s", fromStatus, toStatus), Record{"revision_no": plan["revision_no"], "definition_hash": plan["definition_hash"], "comment": comment}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.ProjectQualityWorkspace(ctx, cleanID(plan["project_id"]))
}

func (r *Repository) CreateQualityInspectionRequest(ctx context.Context, projectID string, body Record, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := queryTx(ctx, tx, `
		SELECT i.*,p.project_id,p.status AS itp_status,p.id AS itp_plan_id
		FROM quality_itp_items i JOIN quality_itp_plans p ON p.id=i.itp_plan_id
		WHERE i.id=$1
	`, body["itp_item_id"])
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 || cleanID(rows[0]["project_id"]) != projectID || fmt.Sprint(rows[0]["itp_status"]) != "active" {
		return nil, validationError("RFI phải liên kết inspection item của ITP active trong cùng công trình")
	}
	itpItem := rows[0]
	if workItemID := cleanID(body["work_item_id"]); workItemID != "" {
		var count int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM work_items WHERE id=$1 AND project_id=$2 AND status <> 'cancelled'`, workItemID, projectID).Scan(&count); err != nil {
			return nil, err
		}
		if count == 0 {
			return nil, validationError("Work item RFI không thuộc công trình")
		}
	}
	if evidenceID := cleanID(body["request_evidence_id"]); evidenceID != "" {
		if err := requireQualityEvidenceTx(ctx, tx, evidenceID, projectID); err != nil {
			return nil, err
		}
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO quality_inspection_requests (project_id,rfi_code,itp_plan_id,itp_item_id,work_item_id,location,lot_reference,requested_inspection_at,status,request_comment,request_evidence_id,requested_by)
		VALUES ($1,$2,$3,$4,NULLIF($5,'')::uuid,$6,$7,$8::timestamptz,'draft',$9,NULLIF($10,'')::uuid,NULLIF($11,'')::uuid)
	`, projectID, body["rfi_code"], itpItem["itp_plan_id"], body["itp_item_id"], cleanID(body["work_item_id"]), body["location"], body["lot_reference"], body["requested_inspection_at"], body["request_comment"], cleanID(body["request_evidence_id"]), actorID)
	if err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "quality_rfi_created", "quality_rfi", "", "Tạo Request for Inspection", Record{"rfi_code": body["rfi_code"], "itp_item_id": body["itp_item_id"], "location": body["location"]}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.ProjectQualityWorkspace(ctx, projectID)
}

func requireQualityEvidenceTx(ctx context.Context, tx *sql.Tx, evidenceID, projectID string) error {
	var status string
	var evidenceProjectID string
	if err := tx.QueryRowContext(ctx, `SELECT project_id::text,status FROM evidence_records WHERE id=$1`, evidenceID).Scan(&evidenceProjectID, &status); err != nil {
		return validationError("Minh chứng QA/QC không tồn tại", Record{"evidence_id": evidenceID})
	}
	if evidenceProjectID != projectID || status == "rejected" {
		return validationError("Minh chứng QA/QC không thuộc công trình hoặc đã bị từ chối", Record{"evidence_id": evidenceID})
	}
	return nil
}

func (r *Repository) QualityInspectionAction(ctx context.Context, rfiID, action, result, comment, inspectionEvidenceID, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := queryTx(ctx, tx, `
		SELECT rfi.*,i.activity_name,i.control_point,i.evidence_required,p.itp_code
		FROM quality_inspection_requests rfi
		JOIN quality_itp_items i ON i.id=rfi.itp_item_id
		JOIN quality_itp_plans p ON p.id=rfi.itp_plan_id
		WHERE rfi.id=$1 FOR UPDATE OF rfi
	`, rfiID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	rfi := rows[0]
	fromStatus := fmt.Sprint(rfi["status"])
	toStatus := ""
	eventType := ""
	if action == "submit" {
		if fromStatus != "draft" {
			return nil, validationError("Chỉ RFI nháp mới được gửi")
		}
		if boolValue(rfi["evidence_required"]) && cleanID(rfi["request_evidence_id"]) == "" {
			return nil, validationError("Inspection item yêu cầu minh chứng khi gửi RFI")
		}
		toStatus, eventType = "submitted", "quality_rfi_submitted"
		_, err = tx.ExecContext(ctx, `UPDATE quality_inspection_requests SET status='submitted',submitted_at=now(),updated_at=now() WHERE id=$1`, rfiID)
	} else if action == "schedule" {
		if fromStatus != "submitted" {
			return nil, validationError("Chỉ RFI đã gửi mới được xếp lịch")
		}
		toStatus, eventType = "scheduled", "quality_rfi_scheduled"
		_, err = tx.ExecContext(ctx, `UPDATE quality_inspection_requests SET status='scheduled',updated_at=now() WHERE id=$1`, rfiID)
	} else if action == "inspect" {
		if !statusIn(fromStatus, "submitted", "scheduled") {
			return nil, validationError("RFI chưa ở trạng thái sẵn sàng inspection")
		}
		if inspectionEvidenceID != "" {
			if err := requireQualityEvidenceTx(ctx, tx, inspectionEvidenceID, cleanID(rfi["project_id"])); err != nil {
				return nil, err
			}
		}
		if (boolValue(rfi["evidence_required"]) || result != "pass") && inspectionEvidenceID == "" {
			return nil, validationError("Inspection bắt buộc có minh chứng theo ITP hoặc khi kết quả không đạt")
		}
		toStatus = map[string]string{"pass": "accepted", "conditional_pass": "conditional", "fail": "rejected"}[result]
		eventType = "quality_rfi_inspected"
		_, err = tx.ExecContext(ctx, `
			UPDATE quality_inspection_requests SET status=$2,result=$3,inspection_comment=$4,inspection_evidence_id=NULLIF($5,'')::uuid,
				inspector_id=NULLIF($6,'')::uuid,inspected_at=now(),closed_at=now(),updated_at=now()
			WHERE id=$1
		`, rfiID, toStatus, result, comment, inspectionEvidenceID, actorID)
		if err == nil && result == "fail" {
			_, err = r.createQualityNCRTx(ctx, tx, cleanID(rfi["project_id"]), Record{
				"ncr_code": fmt.Sprintf("NCR-%s", fmt.Sprint(rfi["rfi_code"])), "source_type": "rfi", "source_id": rfiID,
				"rfi_id": rfiID, "work_item_id": cleanID(rfi["work_item_id"]), "title": fmt.Sprintf("Không phù hợp từ %s", rfi["rfi_code"]),
				"severity": "high", "nonconformity": comment, "immediate_action": "Tạm dừng hạng mục tại vị trí kiểm tra", "evidence_id": inspectionEvidenceID,
			}, actorID, actorName)
		}
	} else {
		return nil, validationError("Hành động RFI không hợp lệ", Record{"action": action})
	}
	if err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, cleanID(rfi["project_id"]), actorID, actorName, eventType, "quality_rfi", rfiID, fmt.Sprintf("Chuyển RFI từ %s sang %s", fromStatus, toStatus), Record{"result": result, "itp_item_id": rfi["itp_item_id"], "control_point": rfi["control_point"], "comment": comment}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.ProjectQualityWorkspace(ctx, cleanID(rfi["project_id"]))
}

func (r *Repository) CreateQualityNCR(ctx context.Context, projectID string, body Record, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if _, err := r.createQualityNCRTx(ctx, tx, projectID, body, actorID, actorName); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.ProjectQualityWorkspace(ctx, projectID)
}

func (r *Repository) createQualityNCRTx(ctx context.Context, tx *sql.Tx, projectID string, body Record, actorID, actorName string) (string, error) {
	if evidenceID := cleanID(body["evidence_id"]); evidenceID != "" {
		if err := requireQualityEvidenceTx(ctx, tx, evidenceID, projectID); err != nil {
			return "", err
		}
	}
	if rfiID := cleanID(body["rfi_id"]); rfiID != "" {
		var count int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM quality_inspection_requests WHERE id=$1 AND project_id=$2`, rfiID, projectID).Scan(&count); err != nil || count == 0 {
			return "", validationError("RFI nguồn NCR không thuộc công trình")
		}
	}
	if workItemID := cleanID(body["work_item_id"]); workItemID != "" {
		var count int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM work_items WHERE id=$1 AND project_id=$2`, workItemID, projectID).Scan(&count); err != nil || count == 0 {
			return "", validationError("Work item nguồn NCR không thuộc công trình")
		}
	}
	if issueID := cleanID(body["issue_id"]); issueID != "" {
		var count int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM issues WHERE id=$1 AND project_id=$2`, issueID, projectID).Scan(&count); err != nil || count == 0 {
			return "", validationError("Issue nguồn NCR không thuộc công trình")
		}
	}
	reworkRows, err := queryTx(ctx, tx, `
		INSERT INTO rework_requests (project_id,issue_id,title,description,status,due_date,created_by)
		VALUES ($1,NULLIF($2,'')::uuid,$3,$4,'draft',NULLIF($5,'')::date,NULLIF($6,'')::uuid) RETURNING id
	`, projectID, cleanID(body["issue_id"]), fmt.Sprintf("Rework %s", body["ncr_code"]), body["nonconformity"], body["due_date"], actorID)
	if err != nil {
		return "", err
	}
	reworkID := cleanID(reworkRows[0]["id"])
	rows, err := queryTx(ctx, tx, `
		INSERT INTO quality_ncrs (project_id,ncr_code,source_type,source_id,rfi_id,work_item_id,issue_id,rework_request_id,title,severity,nonconformity,immediate_action,due_date,status,evidence_id,owner_id,created_by)
		VALUES ($1,$2,$3,$4,NULLIF($5,'')::uuid,NULLIF($6,'')::uuid,NULLIF($7,'')::uuid,$8,$9,$10,$11,$12,NULLIF($13,'')::date,'open',NULLIF($14,'')::uuid,NULLIF($15,'')::uuid,NULLIF($16,'')::uuid) RETURNING id
	`, projectID, body["ncr_code"], body["source_type"], body["source_id"], cleanID(body["rfi_id"]), cleanID(body["work_item_id"]), cleanID(body["issue_id"]), reworkID, body["title"], body["severity"], body["nonconformity"], body["immediate_action"], body["due_date"], cleanID(body["evidence_id"]), cleanID(body["owner_id"]), actorID)
	if err != nil {
		return "", err
	}
	ncrID := cleanID(rows[0]["id"])
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "quality_ncr_created", "quality_ncr", ncrID, "Tạo NCR và rework", Record{"ncr_code": body["ncr_code"], "source_type": body["source_type"], "source_id": body["source_id"], "rework_request_id": reworkID}); err != nil {
		return "", err
	}
	return ncrID, nil
}

func (r *Repository) QualityNCRAction(ctx context.Context, ncrID, action string, req QualityNCRActionRequest, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := queryTx(ctx, tx, `SELECT * FROM quality_ncrs WHERE id=$1 FOR UPDATE`, ncrID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	ncr := rows[0]
	fromStatus := fmt.Sprint(ncr["status"])
	toStatus := ""
	eventType := ""
	switch action {
	case "root-cause":
		if fromStatus != "open" {
			return nil, validationError("NCR không ở trạng thái nhận root cause")
		}
		toStatus, eventType = "root_cause", "quality_ncr_root_cause_submitted"
		_, err = tx.ExecContext(ctx, `UPDATE quality_ncrs SET status=$2,root_cause=$3,preventive_action=$4,root_cause_submitted_by=NULLIF($5,'')::uuid,updated_at=now() WHERE id=$1`, ncrID, toStatus, strings.TrimSpace(req.RootCause), strings.TrimSpace(req.PreventiveAction), actorID)
	case "corrective":
		if fromStatus != "root_cause" {
			return nil, validationError("NCR cần root cause trước corrective action")
		}
		if err := requireQualityEvidenceTx(ctx, tx, cleanID(req.CorrectiveEvidenceID), cleanID(ncr["project_id"])); err != nil {
			return nil, err
		}
		toStatus, eventType = "corrective_action", "quality_ncr_corrective_submitted"
		_, err = tx.ExecContext(ctx, `UPDATE quality_ncrs SET status=$2,corrective_action=$3,preventive_action=COALESCE(NULLIF($4,''),preventive_action),corrective_evidence_id=$5,corrective_submitted_by=NULLIF($6,'')::uuid,updated_at=now() WHERE id=$1`, ncrID, toStatus, strings.TrimSpace(req.CorrectiveAction), strings.TrimSpace(req.PreventiveAction), cleanID(req.CorrectiveEvidenceID), actorID)
	case "reinspection":
		if fromStatus != "corrective_action" {
			return nil, validationError("NCR cần corrective action trước tái kiểm tra")
		}
		var projectID, rfiStatus, rfiResult string
		if err := tx.QueryRowContext(ctx, `SELECT project_id::text,status,COALESCE(result,'') FROM quality_inspection_requests WHERE id=$1`, cleanID(req.ReinspectionRFIID)).Scan(&projectID, &rfiStatus, &rfiResult); err != nil || projectID != cleanID(ncr["project_id"]) {
			return nil, validationError("RFI tái kiểm tra không thuộc công trình")
		}
		toStatus, eventType = "reinspection", "quality_ncr_reinspection_linked"
		_, err = tx.ExecContext(ctx, `UPDATE quality_ncrs SET status=$2,reinspection_rfi_id=$3,updated_at=now() WHERE id=$1`, ncrID, toStatus, cleanID(req.ReinspectionRFIID))
	case "close":
		if fromStatus != "reinspection" || cleanID(ncr["reinspection_rfi_id"]) == "" {
			return nil, validationError("NCR cần RFI tái kiểm tra trước khi đóng")
		}
		var rfiStatus, rfiResult string
		if err := tx.QueryRowContext(ctx, `SELECT status,COALESCE(result,'') FROM quality_inspection_requests WHERE id=$1`, ncr["reinspection_rfi_id"]).Scan(&rfiStatus, &rfiResult); err != nil {
			return nil, err
		}
		if !statusIn(rfiStatus, "accepted", "conditional") || !statusIn(rfiResult, "pass", "conditional_pass") {
			return nil, validationError("RFI tái kiểm tra chưa đạt", Record{"rfi_status": rfiStatus, "rfi_result": rfiResult})
		}
		if reworkID := cleanID(ncr["rework_request_id"]); reworkID != "" {
			var reworkStatus string
			if err := tx.QueryRowContext(ctx, `SELECT status FROM rework_requests WHERE id=$1`, reworkID).Scan(&reworkStatus); err != nil {
				return nil, err
			}
			if !statusIn(reworkStatus, "done", "completed") {
				return nil, validationError("Rework liên kết chưa hoàn tất", Record{"rework_status": reworkStatus})
			}
		}
		toStatus, eventType = "closed", "quality_ncr_closed"
		_, err = tx.ExecContext(ctx, `UPDATE quality_ncrs SET status='closed',closed_by=NULLIF($2,'')::uuid,closed_at=now(),closure_comment=$3,updated_at=now() WHERE id=$1`, ncrID, actorID, strings.TrimSpace(req.Comment))
	default:
		return nil, validationError("Hành động NCR không hợp lệ", Record{"action": action})
	}
	if err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, cleanID(ncr["project_id"]), actorID, actorName, eventType, "quality_ncr", ncrID, fmt.Sprintf("Chuyển NCR từ %s sang %s", fromStatus, toStatus), Record{"rework_request_id": ncr["rework_request_id"], "reinspection_rfi_id": req.ReinspectionRFIID, "comment": req.Comment}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.ProjectQualityWorkspace(ctx, cleanID(ncr["project_id"]))
}

func (r *Repository) CreateRecurringPlan(ctx context.Context, projectID string, body Record, actorID, actorName string) (Record, error) {
	if sourceWorkItemID := cleanID(body["source_work_item_id"]); sourceWorkItemID != "" {
		var count int
		if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM work_items WHERE id=$1 AND project_id=$2`, sourceWorkItemID, projectID).Scan(&count); err != nil || count == 0 {
			return nil, validationError("Work item nguồn lịch lặp không thuộc công trình")
		}
	}
	rows, err := r.Query(ctx, `
		INSERT INTO construction_recurring_plans (project_id,plan_code,title,description,recurrence_type,interval_value,start_date,end_date,next_due_date,due_time,responsible_role,assigned_user_id,evidence_required,source_work_item_id,status,created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,NULLIF($8,'')::date,$7,NULLIF($9,'')::time,$10,NULLIF($11,'')::uuid,$12,NULLIF($13,'')::uuid,'active',NULLIF($14,'')::uuid) RETURNING *
	`, projectID, body["plan_code"], body["title"], body["description"], body["recurrence_type"], body["interval_value"], body["start_date"], body["end_date"], body["due_time"], body["responsible_role"], cleanID(body["assigned_user_id"]), body["evidence_required"], cleanID(body["source_work_item_id"]), actorID)
	if err != nil {
		return nil, err
	}
	planID := cleanID(rows[0]["id"])
	r.AddAuditBestEffort(ctx, projectID, actorID, actorName, "recurring_plan_created", "recurring_plan", planID, "Tạo lịch QA/QC lặp", Record{"plan_code": body["plan_code"], "recurrence_type": body["recurrence_type"], "interval_value": body["interval_value"]})
	return r.ProjectQualityWorkspace(ctx, projectID)
}

func (r *Repository) GenerateRecurringOccurrences(ctx context.Context, planID, throughDate, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := queryTx(ctx, tx, `SELECT * FROM construction_recurring_plans WHERE id=$1 AND status='active' FOR UPDATE`, planID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, validationError("Lịch lặp không hoạt động")
	}
	plan := rows[0]
	nextDate, _ := time.Parse("2006-01-02", dateOnly(plan["next_due_date"]))
	through, _ := time.Parse("2006-01-02", throughDate)
	if end := dateOnly(plan["end_date"]); end != "" {
		endDate, _ := time.Parse("2006-01-02", end)
		if through.After(endDate) {
			through = endDate
		}
	}
	generated := 0
	for !nextDate.After(through) {
		dueAt := any(nil)
		if dueTime := strings.TrimSpace(fmt.Sprint(plan["due_time"])); dueTime != "" && dueTime != "<nil>" {
			dueAt = nextDate.Format("2006-01-02") + "T" + strings.TrimSuffix(dueTime, ":00+00")
		}
		result, err := tx.ExecContext(ctx, `
			INSERT INTO construction_recurring_occurrences (plan_id,project_id,occurrence_date,due_at,assigned_user_id,status)
			VALUES ($1,$2,$3,$4::timestamptz,NULLIF($5,'')::uuid,'pending') ON CONFLICT (plan_id,occurrence_date) DO NOTHING
		`, planID, plan["project_id"], nextDate.Format("2006-01-02"), dueAt, cleanID(plan["assigned_user_id"]))
		if err != nil {
			return nil, err
		}
		if count, _ := result.RowsAffected(); count > 0 {
			generated++
		}
		nextDate = nextRecurringDate(nextDate, fmt.Sprint(plan["recurrence_type"]), int(number(plan["interval_value"])))
	}
	if _, err := tx.ExecContext(ctx, `UPDATE construction_recurring_occurrences SET status='overdue',updated_at=now() WHERE plan_id=$1 AND status='pending' AND occurrence_date<CURRENT_DATE`, planID); err != nil {
		return nil, err
	}
	newPlanStatus := "active"
	if end := dateOnly(plan["end_date"]); end != "" && nextDate.Format("2006-01-02") > end {
		newPlanStatus = "completed"
	}
	if _, err := tx.ExecContext(ctx, `UPDATE construction_recurring_plans SET next_due_date=$2,status=$3,updated_at=now() WHERE id=$1`, planID, nextDate.Format("2006-01-02"), newPlanStatus); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, cleanID(plan["project_id"]), actorID, actorName, "recurring_occurrences_generated", "recurring_plan", planID, "Sinh occurrence QA/QC lặp", Record{"through_date": throughDate, "generated_count": generated, "next_due_date": nextDate.Format("2006-01-02")}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.ProjectQualityWorkspace(ctx, cleanID(plan["project_id"]))
}

func nextRecurringDate(current time.Time, recurrenceType string, interval int) time.Time {
	switch recurrenceType {
	case "weekly":
		return current.AddDate(0, 0, 7*interval)
	case "monthly":
		return current.AddDate(0, interval, 0)
	default:
		return current.AddDate(0, 0, interval)
	}
}

func (r *Repository) RecurringOccurrenceAction(ctx context.Context, occurrenceID, action, comment, evidenceID, reason, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := queryTx(ctx, tx, `SELECT o.*,p.evidence_required FROM construction_recurring_occurrences o JOIN construction_recurring_plans p ON p.id=o.plan_id WHERE o.id=$1 FOR UPDATE OF o`, occurrenceID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	occurrence := rows[0]
	if !statusIn(fmt.Sprint(occurrence["status"]), "pending", "in_progress", "overdue") {
		return nil, validationError("Occurrence đã kết thúc")
	}
	toStatus := ""
	if action == "complete" {
		if boolValue(occurrence["evidence_required"]) && evidenceID == "" {
			return nil, validationError("Occurrence yêu cầu minh chứng hoàn tất")
		}
		if evidenceID != "" {
			if err := requireQualityEvidenceTx(ctx, tx, evidenceID, cleanID(occurrence["project_id"])); err != nil {
				return nil, err
			}
		}
		toStatus = "completed"
		_, err = tx.ExecContext(ctx, `UPDATE construction_recurring_occurrences SET status='completed',completion_comment=$2,evidence_id=NULLIF($3,'')::uuid,completed_by=NULLIF($4,'')::uuid,completed_at=now(),updated_at=now() WHERE id=$1`, occurrenceID, comment, evidenceID, actorID)
	} else if action == "skip" {
		toStatus = "skipped"
		_, err = tx.ExecContext(ctx, `UPDATE construction_recurring_occurrences SET status='skipped',skip_reason=$2,skipped_by=NULLIF($3,'')::uuid,skipped_at=now(),updated_at=now() WHERE id=$1`, occurrenceID, reason, actorID)
	} else {
		return nil, validationError("Hành động occurrence không hợp lệ")
	}
	if err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, cleanID(occurrence["project_id"]), actorID, actorName, "recurring_occurrence_"+toStatus, "recurring_occurrence", occurrenceID, "Cập nhật occurrence QA/QC", Record{"plan_id": occurrence["plan_id"], "occurrence_date": occurrence["occurrence_date"], "comment": comment, "reason": reason}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.ProjectQualityWorkspace(ctx, cleanID(occurrence["project_id"]))
}

func (r *Repository) ListProjectDailyReports(ctx context.Context, projectID string) ([]Record, error) {
	rows, err := r.Query(ctx, `
		SELECT report.*,
			project.code AS project_code,
			project.name AS project_name,
			project.address AS project_address,
			author.full_name AS author_name,
			work_item.title AS work_item_title,
			delivery.id::text AS work_report_id,
			delivery.status AS delivery_status,
			delivery.recipient_id::text AS delivery_recipient_id,
			recipient.full_name AS delivery_recipient_name,
			delivery.current_revision_no AS delivery_revision_no,
			delivery.review_comment AS delivery_review_comment,
			delivery_revision.snapshot_sha256 AS delivery_snapshot_sha256,
			delivery_revision.snapshot_json AS delivery_snapshot_json,
			(
				report.work_item_id IS NOT NULL
				AND EXISTS (
					SELECT 1
					FROM jsonb_array_elements_text(report.photos) photo(evidence_id)
					JOIN evidence_records evidence ON evidence.id::text=photo.evidence_id
					WHERE evidence.project_id=report.project_id
					  AND evidence.work_item_id=report.work_item_id
					  AND evidence.evidence_type='photo'
					  AND evidence.status <> 'rejected'
				)
			) AS runtime_compliant
		FROM operation_daily_reports report
		JOIN projects project ON project.id=report.project_id
		LEFT JOIN users author ON author.id=report.reported_by
		LEFT JOIN work_items work_item ON work_item.id=report.work_item_id
		LEFT JOIN work_reports delivery ON delivery.source_kind='operation_daily_report' AND delivery.source_id=report.id
		LEFT JOIN work_report_revisions delivery_revision ON delivery_revision.report_id=delivery.id AND delivery_revision.revision_no=delivery.current_revision_no
		LEFT JOIN users recipient ON recipient.id=delivery.recipient_id
		WHERE report.project_id=$1
		ORDER BY report.created_at DESC
	`, projectID)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		if cleanText(row["delivery_status"]) == "needs_changes" {
			if err := freezeNativeWorkReportEvidence(ctx, r.querySource(), row, false); err != nil {
				return nil, err
			}
		}
		if err := annotateNativeWorkReportDeliverySourceChanged(ctx, r.querySource(), row); err != nil {
			return nil, err
		}
	}
	return rows, nil
}

func (r *Repository) CreateDailyReport(ctx context.Context, body Record, delivery WorkReportDeliveryRequest, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	workItemID := cleanID(body["work_item_id"])
	projectID := cleanID(body["project_id"])
	rows, err := queryTx(ctx, tx, `
		SELECT
			wi.*,
			COALESCE(pr.engine_version, 1) AS pipeline_engine_version,
			pr.status AS pipeline_run_status,
			pr.current_node_id AS pipeline_current_node_id,
			target_node.status AS run_node_status,
			target_node.type AS run_node_type,
			target_node.business_code AS run_node_business_code,
			rs.business_code AS run_step_business_code
		FROM work_items wi
		LEFT JOIN pipeline_runs pr ON pr.id=wi.pipeline_run_id
		LEFT JOIN pipeline_run_nodes target_node ON target_node.id=wi.run_node_id
		LEFT JOIN pipeline_run_steps rs ON rs.id=wi.run_step_id
		WHERE wi.id=$1
		FOR UPDATE OF wi
	`, workItemID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, validationError("Không tìm thấy hạng mục thi công", Record{"work_item_id": workItemID})
	}
	item := rows[0]
	if cleanID(item["project_id"]) != projectID {
		return nil, validationError("Hạng mục không thuộc công trình", Record{"work_item_id": workItemID, "project_id": projectID})
	}

	fromStatus := fmt.Sprint(item["status"])
	autoStart := statusIn(fromStatus, "pending", "ready", "needs_changes")
	if !autoStart && !statusIn(fromStatus, "in_progress", "blocked") {
		return nil, validationError("Trạng thái đầu việc không cho phép tạo báo cáo thi công", Record{"work_item_id": workItemID, "status": fromStatus})
	}
	if !workItemHasStarted(item) && myWorkPlannedStartInFuture(item["planned_start_at"], time.Now()) {
		return nil, validationError("Đầu việc chưa đến thời điểm được phép bắt đầu", Record{
			"work_item_id":     workItemID,
			"planned_start_at": item["planned_start_at"],
		})
	}
	if int(number(item["pipeline_engine_version"])) >= 2 && cleanID(item["pipeline_run_id"]) != "" {
		runtimeRows, runtimeErr := queryTx(ctx, tx, `
			SELECT pr.current_node_id, pr.status AS run_status, current_node.status AS node_status
			FROM pipeline_runs pr
			LEFT JOIN pipeline_run_nodes current_node ON current_node.id=pr.current_node_id
			WHERE pr.id=$1
			FOR UPDATE OF pr
		`, item["pipeline_run_id"])
		if runtimeErr != nil {
			return nil, runtimeErr
		}
		if len(runtimeRows) == 0 {
			return nil, validationError("Không tìm thấy pipeline run của đầu việc")
		}
		runtime := runtimeRows[0]
		currentNodeID := cleanID(runtime["current_node_id"])
		if statusIn(fmt.Sprint(runtime["run_status"]), "completed", "cancelled") ||
			currentNodeID == "" || currentNodeID != cleanID(item["run_node_id"]) ||
			!statusIn(fmt.Sprint(runtime["node_status"]), "in_progress", "waiting_approval", "blocked") {
			return nil, validationError("Đầu việc không còn thuộc Node Thi công đang thực hiện", Record{
				"work_item_id":    workItemID,
				"run_node_id":     item["run_node_id"],
				"current_node_id": currentNodeID,
			})
		}
		if fmt.Sprint(item["run_step_business_code"]) != "construction.item_reporting" &&
			fmt.Sprint(item["run_node_business_code"]) != "construction_execution" &&
			fmt.Sprint(item["run_node_type"]) != "construction" {
			return nil, validationError("Hạng mục không thuộc bước Thi công của quy trình", Record{"work_item_id": workItemID})
		}
	}

	report, err := insertRecord(ctx, tx, "operation_daily_reports", dailyReportColumns(), body)
	if err != nil {
		return nil, err
	}
	transition := Record{
		"auto_started":    false,
		"work_item_id":    workItemID,
		"work_item_title": item["title"],
		"from_status":     fromStatus,
		"to_status":       fromStatus,
	}
	if autoStart {
		metadata := Record{
			"source":          "construction_daily_report_created",
			"daily_report_id": report["id"],
		}
		if err := persistWorkItemStartedTx(ctx, tx, item, actorID, actorName, "Bắt đầu đầu việc khi tạo báo cáo thi công", metadata); err != nil {
			return nil, err
		}
		transition["auto_started"] = true
		transition["to_status"] = "in_progress"
	}
	auditMetadata := Record{
		"progress_percent":       report["progress_percent"],
		"work_item_id":           workItemID,
		"evidence_count":         len(stringSlice(report["photos"])),
		"work_item_auto_started": autoStart,
		"captured_rows":          constructionRiskPlanAuditRows(constructionReportRecord(report["structured_json"])),
	}
	if err := addAuditTx(
		ctx,
		tx,
		projectID,
		actorID,
		actorName,
		"daily_report_created",
		"daily_report",
		cleanID(report["id"]),
		fmt.Sprintf("Tạo nhật ký ngày %v", report["report_date"]),
		auditMetadata,
	); err != nil {
		return nil, err
	}
	deliveryTransition := Record{"mode": "draft", "submitted": false, "status": "draft"}
	if delivery.Mode == "submit" {
		deliveryTransition, err = r.submitWorkReportTx(ctx, tx, cleanID(report["id"]), delivery.RecipientID, delivery.Comment, actorID, actorName)
		if err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	report["work_item_transition"] = transition
	applyWorkReportDeliveryTransition(report, deliveryTransition)
	return report, nil
}

func (r *Repository) UpdateDailyReportWithDelivery(ctx context.Context, id string, body Record, delivery WorkReportDeliveryRequest, actorID, actorName string, omittedSections ...string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	current, err := queryOneRecord(ctx, tx, `SELECT * FROM operation_daily_reports WHERE id=$1 FOR UPDATE`, id)
	if err != nil {
		return nil, err
	}
	if !boolValue(current["report_delivery_eligible"]) || cleanID(current["workflow_form_instance_id"]) != "" {
		return nil, conflictError("Nguồn báo cáo không còn cho phép chỉnh sửa tại nhật ký")
	}
	allowed, accessErr := r.withReadTransaction(tx).CanAccessProjectPermission(ctx, actorID, cleanID(current["project_id"]), []string{"operation.report"})
	if accessErr != nil {
		return nil, accessErr
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền cập nhật báo cáo thi công")
	}
	status, err := workReportStatusForSourceTx(ctx, tx, id)
	if err != nil {
		return nil, err
	}
	if status != "" && status != "needs_changes" {
		return nil, validationError("Báo cáo chỉ được chỉnh sửa khi chưa gửi hoặc đang được yêu cầu bổ sung", Record{"report_id": id, "status": status})
	}
	if _, hasStructured := body["structured_json"]; hasStructured {
		structured := constructionReportRecord(body["structured_json"])
		mergeConstructionCapturedRows(structured, constructionReportRecord(current["structured_json"]))
		oldCrews := map[string]Record{}
		for _, row := range workflowAnswerRecordList(constructionReportRecord(current["structured_json"])["crew_items"]) {
			if id := cleanID(row["row_id"]); id != "" {
				oldCrews[id] = row
			}
		}
		for _, row := range workflowAnswerRecordList(structured["crew_items"]) {
			old := oldCrews[cleanID(row["row_id"])]
			for _, key := range []string{"representative", "contact"} {
				if _, provided := row[key]; !provided {
					if value, exists := old[key]; exists {
						row[key] = value
					}
				}
			}
		}
	}
	if len(omittedSections) > 0 {
		structured := constructionReportRecord(body["structured_json"])
		previous := constructionReportRecord(current["structured_json"])
		for _, key := range omittedSections {
			if !statusIn(key, "supervision_assessments", "material_arrival_items") {
				return nil, validationError("Trường giữ nguyên của báo cáo không hợp lệ", nil)
			}
			delete(structured, key)
			if value, exists := previous[key]; exists {
				structured[key] = value
			}
		}
		body["structured_json"] = structured
	}
	updated, err := updateRecord(ctx, tx, "operation_daily_reports", dailyReportColumns(), id, body)
	if err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, cleanID(current["project_id"]), actorID, actorName, "daily_report_updated", "daily_report", id, fmt.Sprintf("Cập nhật nhật ký ngày %s", updated["report_date"]), Record{
		"progress_percent": updated["progress_percent"],
		"work_item_id":     updated["work_item_id"],
		"evidence_count":   len(constructionReportStringList(updated["photos"])),
		"capture_before":   constructionRiskPlanAuditRows(constructionReportRecord(current["structured_json"])),
		"capture_after":    constructionRiskPlanAuditRows(constructionReportRecord(updated["structured_json"])),
	}); err != nil {
		return nil, err
	}
	deliveryTransition := Record{"mode": "draft", "submitted": false, "status": status}
	if status == "" {
		deliveryTransition["status"] = "draft"
	}
	if delivery.Mode == "submit" {
		deliveryTransition, err = r.submitWorkReportTx(ctx, tx, id, delivery.RecipientID, delivery.Comment, actorID, actorName)
		if err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	applyWorkReportDeliveryTransition(updated, deliveryTransition)
	return updated, nil
}

func applyWorkReportDeliveryTransition(report, transition Record) {
	report["delivery_transition"] = transition
	report["delivery_status"] = transition["status"]
	if boolValue(transition["submitted"]) {
		report["work_report_id"] = transition["work_report_id"]
		report["delivery_recipient_id"] = transition["recipient_id"]
		report["delivery_recipient_name"] = transition["recipient_name"]
		report["delivery_revision_no"] = transition["revision_no"]
		report["delivery_review_comment"] = nil
	}
}

func (r *Repository) UpdateDailyReport(ctx context.Context, id string, body Record) (Record, error) {
	return r.Update(ctx, "operation_daily_reports", dailyReportColumns(), id, body)
}

func (r *Repository) ListProjectIssues(ctx context.Context, projectID string) ([]Record, error) {
	return r.Query(ctx, `
		SELECT *
		FROM issues
		WHERE project_id=$1 AND status <> 'deleted'
		ORDER BY created_at DESC
	`, projectID)
}

func (r *Repository) CreateIssue(ctx context.Context, body Record) (Record, error) {
	return r.Insert(ctx, "issues", issueColumns(), body)
}

func (r *Repository) UpdateIssue(ctx context.Context, id string, body Record, actorID, actorName string) (Record, error) {
	eventFor := func(row Record) auditEvent {
		return auditEvent{
			ProjectID: cleanID(row["project_id"]), ActorID: actorID, ActorName: actorName,
			EventType: "issue_updated", TargetType: "issue", TargetID: cleanID(row["id"]),
			Description: fmt.Sprintf("Cập nhật vấn đề %s", row["title"]),
			Metadata:    Record{"status": row["status"], "severity": row["severity"]},
		}
	}
	if _, changesStatus := body["status"]; changesStatus {
		return r.updateWithAudit(ctx, "issues", issueColumns(), id, body, eventFor)
	}
	row, err := r.Update(ctx, "issues", issueColumns(), id, body)
	if err != nil {
		return nil, err
	}
	event := eventFor(row)
	r.AddAuditBestEffort(ctx, event.ProjectID, event.ActorID, event.ActorName, event.EventType, event.TargetType, event.TargetID, event.Description, event.Metadata)
	return row, nil
}

func (r *Repository) ListProjectReworks(ctx context.Context, projectID string) ([]Record, error) {
	return r.Query(ctx, `
		SELECT *
		FROM rework_requests
		WHERE project_id=$1 AND status <> 'deleted'
		ORDER BY created_at DESC
	`, projectID)
}

func (r *Repository) CreateRework(ctx context.Context, body Record) (Record, error) {
	return r.Insert(ctx, "rework_requests", reworkColumns(), body)
}

func (r *Repository) UpdateReworkStatus(ctx context.Context, id, status, actorID, actorName string) (Record, error) {
	eventType := "rework_approved"
	if status == "done" {
		eventType = "rework_completed"
	}
	return r.withAuditTransaction(ctx, func(tx *sql.Tx) (Record, error) {
		rows, err := queryTx(ctx, tx, `
			UPDATE rework_requests
			SET status=$2,
				approved_by=CASE
					WHEN $2='approved' THEN NULLIF($3,'')::uuid
					WHEN $2='done' THEN COALESCE(approved_by, NULLIF($3,'')::uuid)
					ELSE approved_by
				END,
				updated_at=now()
			WHERE id=$1
			RETURNING *
		`, id, status, actorID)
		if err != nil {
			return nil, err
		}
		if len(rows) == 0 {
			return nil, sql.ErrNoRows
		}
		return rows[0], nil
	}, func(row Record) auditEvent {
		return auditEvent{
			ProjectID: cleanID(row["project_id"]), ActorID: actorID, ActorName: actorName,
			EventType: eventType, TargetType: "rework",
			Description: fmt.Sprintf("Cập nhật sửa lỗi sang %s", status), Metadata: Record{"status": status},
		}
	})
}

func (r *Repository) ListProjectReplannings(ctx context.Context, projectID string) ([]Record, error) {
	return r.Query(ctx, `
		SELECT *
		FROM replanning_requests
		WHERE project_id=$1 AND status <> 'deleted'
		ORDER BY created_at DESC
	`, projectID)
}

func (r *Repository) CreateReplanning(ctx context.Context, body Record) (Record, error) {
	return r.Insert(ctx, "replanning_requests", replanningColumns(), body)
}

func (r *Repository) ApproveReplanning(ctx context.Context, id, actorID, actorName string) (Record, error) {
	return r.withAuditTransaction(ctx, func(tx *sql.Tx) (Record, error) {
		rows, err := queryTx(ctx, tx, `
			UPDATE replanning_requests
			SET status='approved', approved_by=NULLIF($2,'')::uuid, updated_at=now()
			WHERE id=$1
			RETURNING *
		`, id, actorID)
		if err != nil {
			return nil, err
		}
		if len(rows) == 0 {
			return nil, sql.ErrNoRows
		}
		return rows[0], nil
	}, func(row Record) auditEvent {
		return auditEvent{
			ProjectID: cleanID(row["project_id"]), ActorID: actorID, ActorName: actorName,
			EventType: "replanning_approved", TargetType: "replanning",
			Description: "Duyệt yêu cầu lập kế hoạch lại", Metadata: Record{"status": row["status"]},
		}
	})
}
