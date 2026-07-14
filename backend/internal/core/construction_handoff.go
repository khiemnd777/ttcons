package core

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

func (s *Service) ProjectConstructionHandoff(ctx context.Context, projectID string) (Record, error) {
	projectID = cleanID(projectID)
	if !validActivityUUID(projectID) {
		return nil, validationError("Mã công trình không hợp lệ")
	}
	return s.repo.ProjectConstructionHandoff(ctx, projectID)
}

func (s *Service) ActivateConstructionHandoff(ctx context.Context, projectID string, req ConstructionHandoffActivateRequest, actorID, actorName string) (Record, error) {
	projectID = cleanID(projectID)
	if !validActivityUUID(projectID) {
		return nil, validationError("Mã công trình không hợp lệ")
	}
	if req.SourceRunNodeID != "" && !validActivityUUID(cleanID(req.SourceRunNodeID)) {
		return nil, validationError("Giai đoạn kế hoạch nguồn không hợp lệ")
	}
	if req.SourceFormInstanceID != "" && !validActivityUUID(cleanID(req.SourceFormInstanceID)) {
		return nil, validationError("Biểu mẫu kế hoạch nguồn không hợp lệ")
	}
	if req.SourceFormRevisionNo < 0 || (cleanID(req.SourceFormInstanceID) != "" && req.SourceFormRevisionNo == 0) {
		return nil, validationError("Revision biểu mẫu kế hoạch nguồn không hợp lệ")
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = "Bàn giao kế hoạch thi công"
	}
	if len([]rune(title)) > 180 {
		return nil, validationError("Tên đợt bàn giao tối đa 180 ký tự")
	}
	if len(req.Items) == 0 || len(req.Items) > 200 {
		return nil, validationError("Bàn giao cần từ 1 đến 200 hạng mục giám sát")
	}

	items := make([]ConstructionHandoffItemRequest, 0, len(req.Items))
	seenSources := map[string]bool{}
	for index, raw := range req.Items {
		raw.Title = strings.TrimSpace(raw.Title)
		raw.Location = strings.TrimSpace(raw.Location)
		raw.ResponsibleRole = strings.TrimSpace(raw.ResponsibleRole)
		raw.AssignedUserID = cleanID(raw.AssignedUserID)
		raw.SourceFieldKey = strings.TrimSpace(raw.SourceFieldKey)
		if raw.SourceFieldKey == "" {
			raw.SourceFieldKey = "manual_handoff_items"
		}
		if raw.SourceItemIndex < 0 {
			raw.SourceItemIndex = index
		}
		if raw.Title == "" {
			return nil, validationError(fmt.Sprintf("Hạng mục %d cần nội dung kiểm tra", index+1))
		}
		if raw.ResponsibleRole == "" {
			raw.ResponsibleRole = "QA/QC"
		}
		if raw.AssignedUserID != "" && !validActivityUUID(raw.AssignedUserID) {
			return nil, validationError(fmt.Sprintf("Người giám sát của hạng mục %d không hợp lệ", index+1))
		}
		scheduledAt, err := constructionOptionalTime(raw.ScheduledAt, fmt.Sprintf("Thời điểm hạng mục %d", index+1))
		if err != nil {
			return nil, err
		}
		raw.ScheduledAt = scheduledAt
		criteria, err := normalizeConstructionCheckCriteria(raw.Criteria)
		if err != nil {
			return nil, err
		}
		if len(criteria) == 0 {
			criteria = []ConstructionCheckCriterion{{Title: raw.Title, Criteria: "Thực hiện đúng hồ sơ và yêu cầu chất lượng đã duyệt", EvidenceRequired: raw.EvidenceRequired}}
		}
		for criterionIndex := range criteria {
			if raw.EvidenceRequired {
				criteria[criterionIndex].EvidenceRequired = true
			}
		}
		raw.Criteria = criteria
		sourceKey := fmt.Sprintf("%s:%d", raw.SourceFieldKey, raw.SourceItemIndex)
		if seenSources[sourceKey] {
			return nil, validationError("Danh sách bàn giao có hạng mục nguồn bị trùng", Record{"source": sourceKey})
		}
		seenSources[sourceKey] = true
		items = append(items, raw)
	}

	return s.repo.ActivateConstructionHandoff(ctx, projectID, title, strings.TrimSpace(req.Notes), cleanID(req.SourceRunNodeID), cleanID(req.SourceFormInstanceID), req.SourceFormRevisionNo, items, actorID, actorName)
}

func (r *Repository) ProjectConstructionHandoff(ctx context.Context, projectID string) (Record, error) {
	contextRow, err := r.constructionHandoffContext(ctx, projectID)
	if err != nil {
		if err == sql.ErrNoRows {
			return Record{
				"available":  false,
				"reason":     "Công trình chưa có quy trình đang chạy.",
				"candidates": []Record{},
				"handoffs":   []Record{},
			}, nil
		}
		return nil, err
	}

	handoffs, err := r.Query(ctx, `
		SELECT h.*,source_node.name AS source_node_name,target_node.name AS target_node_name,
			u.full_name AS activated_by_name
		FROM construction_handoffs h
		LEFT JOIN pipeline_run_nodes source_node ON source_node.id=h.source_run_node_id
		JOIN pipeline_run_nodes target_node ON target_node.id=h.target_run_node_id
		LEFT JOIN users u ON u.id=h.activated_by
		WHERE h.project_id=$1 AND h.pipeline_run_id=$2 AND h.status <> 'cancelled'
		ORDER BY h.revision_no DESC,h.created_at DESC
	`, projectID, contextRow["pipeline_run_id"])
	if err != nil {
		return nil, err
	}
	for _, handoff := range handoffs {
		items, queryErr := r.Query(ctx, `
			SELECT i.*,u.full_name AS assigned_user_name,w.status AS work_item_status,c.status AS check_status,c.result AS check_result
			FROM construction_handoff_items i
			LEFT JOIN users u ON u.id=i.assigned_user_id
			JOIN work_items w ON w.id=i.generated_work_item_id
			JOIN construction_checks c ON c.id=i.generated_check_id
			WHERE i.handoff_id=$1 AND i.status <> 'cancelled'
			ORDER BY i.source_item_index,i.created_at,i.id
		`, handoff["id"])
		if queryErr != nil {
			return nil, queryErr
		}
		handoff["items"] = items
	}

	candidates, sourceFormID, sourceFormRevision, sourceTitle, err := r.constructionHandoffCandidates(ctx, cleanID(contextRow["pipeline_run_id"]))
	if err != nil {
		return nil, err
	}
	contextRow["available"] = cleanID(contextRow["target_run_node_id"]) != ""
	if !boolValue(contextRow["available"]) {
		contextRow["reason"] = "Quy trình đang chạy chưa có giai đoạn Thi công để nhận bàn giao."
	}
	contextRow["source_form_instance_id"] = sourceFormID
	contextRow["source_form_revision_no"] = sourceFormRevision
	contextRow["suggested_title"] = sourceTitle
	contextRow["candidates"] = candidates
	contextRow["handoffs"] = handoffs
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
	contextRow["participants"] = participants
	alreadyActivated := false
	if len(handoffs) > 0 && sourceFormID != "" {
		alreadyActivated = cleanID(handoffs[0]["source_form_instance_id"]) == sourceFormID && int(number(handoffs[0]["source_form_revision_no"])) == sourceFormRevision
	}
	contextRow["can_activate"] = boolValue(contextRow["available"]) && len(candidates) > 0 && !alreadyActivated
	return contextRow, nil
}

func (r *Repository) constructionHandoffContext(ctx context.Context, projectID string) (Record, error) {
	return r.QueryOne(ctx, `
		SELECT p.current_pipeline_run_id AS pipeline_run_id,
			current_node.id AS source_run_node_id,current_node.name AS source_node_name,
			current_node.type AS source_node_type,current_node.status AS source_node_status,
			target_node.id AS target_run_node_id,target_node.name AS target_node_name,
			target_node.status AS target_node_status
		FROM projects p
		JOIN pipeline_runs pr ON pr.id=p.current_pipeline_run_id AND pr.status='in_progress'
		LEFT JOIN pipeline_run_nodes current_node ON current_node.id=pr.current_node_id
		LEFT JOIN LATERAL (
			SELECT candidate.*
			FROM pipeline_run_nodes candidate
			WHERE candidate.run_id=pr.id
			  AND candidate.type='construction'
			  AND (
				candidate.id=current_node.id
				OR current_node.id IS NULL
				OR candidate.order_index > current_node.order_index
			  )
			ORDER BY CASE WHEN candidate.id=current_node.id THEN 0 ELSE 1 END,candidate.order_index,candidate.id
			LIMIT 1
		) target_node ON true
		WHERE p.id=$1
	`, projectID)
}

func (r *Repository) constructionHandoffCandidates(ctx context.Context, runID string) ([]Record, string, int, string, error) {
	forms, err := r.Query(ctx, `
		SELECT wfi.id,wfi.run_node_id,wfi.current_revision,wfi.answers_snapshot_json,t.code,t.name,wfi.submitted_at
		FROM workflow_form_instances wfi
		JOIN workflow_form_templates t ON t.id=wfi.form_template_id
		WHERE wfi.pipeline_run_id=$1 AND wfi.status='submitted'
		  AND t.code IN ('client_construction_handoff_plan','client_qa_construction_plan_checklist','client_construction_assignment_meeting')
		ORDER BY CASE t.code
			WHEN 'client_construction_handoff_plan' THEN 0
			WHEN 'client_qa_construction_plan_checklist' THEN 1
			ELSE 2 END,
			wfi.submitted_at DESC NULLS LAST,wfi.updated_at DESC,wfi.id DESC
	`, runID)
	if err != nil {
		return nil, "", 0, "", err
	}
	for _, form := range forms {
		answers := recordFromAny(form["answers_snapshot_json"])
		code := strings.TrimSpace(fmt.Sprint(form["code"]))
		fieldKey := "inspection_items"
		if code == "client_qa_construction_plan_checklist" {
			fieldKey = "qa_items"
		} else if code == "client_construction_assignment_meeting" {
			fieldKey = "assignment_items"
		}
		rawItems, ok := answers[fieldKey].([]any)
		if !ok || len(rawItems) == 0 {
			continue
		}
		candidates := make([]Record, 0, len(rawItems))
		for index, raw := range rawItems {
			item := recordFromAny(raw)
			candidate := constructionHandoffCandidate(code, fieldKey, index, item)
			if strings.TrimSpace(fmt.Sprint(candidate["title"])) != "" {
				candidates = append(candidates, candidate)
			}
		}
		if len(candidates) == 0 {
			continue
		}
		title := strings.TrimSpace(fmt.Sprint(answers["handoff_title"]))
		if title == "" || title == "<nil>" {
			title = "Bàn giao kế hoạch thi công"
		}
		return candidates, cleanID(form["id"]), int(number(form["current_revision"])), title, nil
	}
	return []Record{}, "", 0, "Bàn giao kế hoạch thi công", nil
}

func constructionHandoffCandidate(formCode, fieldKey string, index int, item Record) Record {
	title := strings.TrimSpace(fmt.Sprint(item["title"]))
	criteriaText := strings.TrimSpace(fmt.Sprint(item["criteria"]))
	location := strings.TrimSpace(fmt.Sprint(item["location"]))
	role := strings.TrimSpace(fmt.Sprint(item["responsible_role"]))
	scheduledAt := ""
	evidenceRequired := boolValue(item["evidence_required"])
	if formCode == "client_qa_construction_plan_checklist" {
		title = strings.TrimSpace(fmt.Sprint(item["item"]))
		criteriaText = strings.TrimSpace(fmt.Sprint(item["note"]))
		role = "QA/QC"
		evidenceRequired = cleanID(item["evidence_id"]) != ""
	}
	if formCode == "client_construction_assignment_meeting" {
		title = strings.TrimSpace(fmt.Sprint(item["task"]))
		criteriaText = strings.TrimSpace(fmt.Sprint(item["note"]))
		role = strings.TrimSpace(fmt.Sprint(item["owner_role"]))
		if date := strings.TrimSpace(fmt.Sprint(item["due_date"])); date != "" && date != "<nil>" {
			scheduledAt = date + "T09:00:00+07:00"
		}
	}
	if date := strings.TrimSpace(fmt.Sprint(item["scheduled_date"])); date != "" && date != "<nil>" {
		scheduledAt = date + "T09:00:00+07:00"
	}
	if title == "<nil>" {
		title = ""
	}
	if criteriaText == "" || criteriaText == "<nil>" {
		criteriaText = "Thực hiện đúng hồ sơ và yêu cầu chất lượng đã duyệt"
	}
	if location == "<nil>" {
		location = ""
	}
	if role == "" || role == "<nil>" {
		role = "QA/QC"
	}
	criteria := []ConstructionCheckCriterion{{Title: title, Criteria: criteriaText, EvidenceRequired: evidenceRequired}}
	return Record{
		"source_field_key":  fieldKey,
		"source_item_index": index,
		"title":             title,
		"location":          location,
		"scheduled_at":      scheduledAt,
		"responsible_role":  role,
		"assigned_user_id":  "",
		"evidence_required": evidenceRequired,
		"criteria":          criteria,
		"source_form_code":  formCode,
	}
}

func (r *Repository) ActivateConstructionHandoff(ctx context.Context, projectID, title, notes, sourceRunNodeID, sourceFormInstanceID string, expectedSourceFormRevision int, items []ConstructionHandoffItemRequest, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	contextRows, err := queryTx(ctx, tx, `
		SELECT p.current_pipeline_run_id AS pipeline_run_id,
			current_node.id AS current_node_id,current_node.type AS current_node_type,current_node.status AS current_node_status,
			target_node.id AS target_run_node_id,target_node.status AS target_node_status
		FROM projects p
		JOIN pipeline_runs pr ON pr.id=p.current_pipeline_run_id AND pr.status='in_progress'
		LEFT JOIN pipeline_run_nodes current_node ON current_node.id=pr.current_node_id
		LEFT JOIN LATERAL (
			SELECT candidate.* FROM pipeline_run_nodes candidate
			WHERE candidate.run_id=pr.id AND candidate.type='construction'
			  AND (candidate.id=current_node.id OR current_node.id IS NULL OR candidate.order_index > current_node.order_index)
			ORDER BY CASE WHEN candidate.id=current_node.id THEN 0 ELSE 1 END,candidate.order_index,candidate.id LIMIT 1
		) target_node ON true
		WHERE p.id=$1
		FOR UPDATE OF p,pr
	`, projectID)
	if err != nil {
		return nil, err
	}
	if len(contextRows) == 0 || cleanID(contextRows[0]["target_run_node_id"]) == "" {
		return nil, validationError("Quy trình đang chạy chưa có giai đoạn Thi công để nhận bàn giao")
	}
	contextRow := contextRows[0]
	runID := cleanID(contextRow["pipeline_run_id"])
	currentNodeID := cleanID(contextRow["current_node_id"])
	if sourceRunNodeID == "" {
		sourceRunNodeID = currentNodeID
	}
	if sourceRunNodeID != "" {
		sourceRows, queryErr := queryTx(ctx, tx, `SELECT id FROM pipeline_run_nodes WHERE id=$1 AND run_id=$2`, sourceRunNodeID, runID)
		if queryErr != nil {
			return nil, queryErr
		}
		if len(sourceRows) == 0 {
			return nil, validationError("Giai đoạn kế hoạch nguồn không thuộc quy trình đang chạy")
		}
	}
	sourceFormRevision := 0
	if sourceFormInstanceID != "" {
		if expectedSourceFormRevision <= 0 {
			return nil, validationError("Cần revision biểu mẫu kế hoạch nguồn để kích hoạt bàn giao")
		}
		formRows, queryErr := queryTx(ctx, tx, `SELECT id,current_revision FROM workflow_form_instances WHERE id=$1 AND project_id=$2 AND pipeline_run_id=$3 AND status='submitted' FOR UPDATE`, sourceFormInstanceID, projectID, runID)
		if queryErr != nil {
			return nil, queryErr
		}
		if len(formRows) == 0 {
			return nil, validationError("Biểu mẫu nguồn chưa được gửi hoặc không thuộc quy trình đang chạy")
		}
		sourceFormRevision = int(number(formRows[0]["current_revision"]))
		if expectedSourceFormRevision > 0 && expectedSourceFormRevision != sourceFormRevision {
			return nil, validationError("Kế hoạch đã có revision mới; vui lòng tải lại trước khi bàn giao", Record{"expected_revision": expectedSourceFormRevision, "current_revision": sourceFormRevision})
		}
		existingRevision, queryErr := queryTx(ctx, tx, `
			SELECT id FROM construction_handoffs
			WHERE project_id=$1 AND pipeline_run_id=$2 AND source_form_instance_id=$3
			  AND source_form_revision_no=$4 AND status <> 'cancelled'
			LIMIT 1
		`, projectID, runID, sourceFormInstanceID, sourceFormRevision)
		if queryErr != nil {
			return nil, queryErr
		}
		if len(existingRevision) > 0 {
			if err := tx.Commit(); err != nil {
				return nil, err
			}
			return r.GetConstructionHandoff(ctx, cleanID(existingRevision[0]["id"]), true)
		}
	}
	snapshot := Record{
		"title": title, "notes": notes, "items": items,
		"source_run_node_id": sourceRunNodeID, "source_form_instance_id": sourceFormInstanceID, "source_form_revision_no": sourceFormRevision,
	}
	rawSnapshot, err := json.Marshal(snapshot)
	if err != nil {
		return nil, err
	}
	hash := sha256.Sum256(rawSnapshot)
	snapshotHash := hex.EncodeToString(hash[:])

	existing, err := queryTx(ctx, tx, `SELECT id FROM construction_handoffs WHERE project_id=$1 AND pipeline_run_id=$2 AND source_snapshot_hash=$3 AND status <> 'cancelled'`, projectID, runID, snapshotHash)
	if err != nil {
		return nil, err
	}
	if len(existing) > 0 {
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return r.GetConstructionHandoff(ctx, cleanID(existing[0]["id"]), true)
	}
	if sourceFormInstanceID != "" {
		priorItems, queryErr := queryTx(ctx, tx, `
			SELECT i.id,i.generated_work_item_id,i.generated_check_id,w.status AS work_item_status,c.status AS check_status,
				EXISTS (SELECT 1 FROM construction_action_items a WHERE a.source_check_id=c.id AND a.status <> 'cancelled') AS has_action,
				EXISTS (SELECT 1 FROM evidence_records e WHERE e.work_item_id=w.id AND e.status <> 'rejected') AS has_evidence
			FROM construction_handoff_items i
			JOIN construction_handoffs h ON h.id=i.handoff_id
			JOIN work_items w ON w.id=i.generated_work_item_id
			JOIN construction_checks c ON c.id=i.generated_check_id
			WHERE h.project_id=$1 AND h.pipeline_run_id=$2 AND h.source_form_instance_id=$3
			  AND h.status <> 'cancelled' AND i.status <> 'cancelled'
			FOR UPDATE OF i,w,c
		`, projectID, runID, sourceFormInstanceID)
		if queryErr != nil {
			return nil, fmt.Errorf("lock prior construction handoff items: %w", queryErr)
		}
		for _, prior := range priorItems {
			if fmt.Sprint(prior["check_status"]) != "pending" || statusIn(fmt.Sprint(prior["work_item_status"]), "in_progress", "blocked", "needs_changes", "completed") || boolValue(prior["has_action"]) || boolValue(prior["has_evidence"]) {
				return nil, validationError("Không thể thay revision vì hạng mục bàn giao trước đã bắt đầu thi công", Record{"work_item_id": prior["generated_work_item_id"], "check_id": prior["generated_check_id"]})
			}
			if _, queryErr = tx.ExecContext(ctx, `UPDATE construction_handoff_items SET status='cancelled',updated_at=now() WHERE id=$1`, prior["id"]); queryErr != nil {
				return nil, queryErr
			}
			if _, queryErr = tx.ExecContext(ctx, `UPDATE construction_checks SET status='cancelled',updated_at=now() WHERE id=$1`, prior["generated_check_id"]); queryErr != nil {
				return nil, queryErr
			}
			if _, queryErr = tx.ExecContext(ctx, `UPDATE work_items SET status='cancelled',updated_by=NULLIF($2,'')::uuid,updated_at=now() WHERE id=$1`, prior["generated_work_item_id"], actorID); queryErr != nil {
				return nil, queryErr
			}
		}
	}

	var revisionNo int
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(revision_no),0)+1 FROM construction_handoffs WHERE project_id=$1 AND pipeline_run_id=$2`, projectID, runID).Scan(&revisionNo); err != nil {
		return nil, err
	}
	targetNodeID := cleanID(contextRow["target_run_node_id"])
	handoffRows, err := queryTx(ctx, tx, `
		INSERT INTO construction_handoffs (
			project_id,pipeline_run_id,source_run_node_id,target_run_node_id,source_form_instance_id,source_form_revision_no,revision_no,
			title,notes,status,source_snapshot_hash,source_snapshot_jsonb,activated_by,activated_at,created_by
		) VALUES ($1,$2,NULLIF($3,'')::uuid,$4,NULLIF($5,'')::uuid,$6,$7,$8,NULLIF($9,''),'activated',$10,$11::jsonb,NULLIF($12,'')::uuid,now(),NULLIF($12,'')::uuid)
		RETURNING *
	`, projectID, runID, sourceRunNodeID, targetNodeID, sourceFormInstanceID, sourceFormRevision, revisionNo, title, notes, snapshotHash, string(rawSnapshot), actorID)
	if err != nil {
		return nil, err
	}
	handoff := handoffRows[0]
	workStatus := "pending"
	if statusIn(fmt.Sprint(contextRow["target_node_status"]), "in_progress", "blocked") {
		workStatus = "ready"
	}

	for _, item := range items {
		rawCriteria, _ := json.Marshal(item.Criteria)
		rawItem, _ := json.Marshal(item)
		rolesJSON, _ := json.Marshal([]string{item.ResponsibleRole})
		itemRows, queryErr := queryTx(ctx, tx, `
			INSERT INTO construction_handoff_items (
				handoff_id,project_id,source_field_key,source_item_index,title,location,criteria_jsonb,scheduled_at,
				responsible_role_ids,assigned_user_id,evidence_required,source_snapshot_jsonb,status
			) VALUES ($1,$2,$3,$4,$5,NULLIF($6,''),$7::jsonb,NULLIF($8,'')::timestamptz,$9::jsonb,NULLIF($10,'')::uuid,$11,$12::jsonb,'activated')
			RETURNING *
		`, handoff["id"], projectID, item.SourceFieldKey, item.SourceItemIndex, item.Title, item.Location, string(rawCriteria), item.ScheduledAt, string(rolesJSON), item.AssignedUserID, item.EvidenceRequired, string(rawItem))
		if queryErr != nil {
			return nil, queryErr
		}
		handoffItem := itemRows[0]
		assigneeStaffID := ""
		if item.AssignedUserID != "" {
			staffRows, staffErr := queryTx(ctx, tx, `
				SELECT s.id FROM staff s
				JOIN project_members pm ON pm.staff_id=s.id AND pm.project_id=$2 AND pm.active
				WHERE s.user_id=$1 AND s.status='active' LIMIT 1
			`, item.AssignedUserID, projectID)
			if staffErr != nil {
				return nil, staffErr
			}
			if len(staffRows) > 0 {
				assigneeStaffID = cleanID(staffRows[0]["id"])
			} else {
				return nil, validationError("Người giám sát không phải thành viên đang hoạt động của công trình", Record{"assigned_user_id": item.AssignedUserID, "item_title": item.Title})
			}
		}
		evidenceRequirement, _ := json.Marshal(Record{"required": item.EvidenceRequired, "minimum": map[bool]int{true: 1, false: 0}[item.EvidenceRequired], "types": []string{"photo", "document"}})
		workItemID := newID()
		if _, queryErr = tx.ExecContext(ctx, `
			INSERT INTO work_items (
				id,project_id,pipeline_run_id,run_node_id,source_type,source_id,title,description,work_type,status,priority,
				role_ids,assignee_staff_id,planned_start_at,due_at,evidence_requirement_json,created_by,updated_by
			) VALUES ($1,$2,$3,$4,'construction_handoff_item',$5,$6,$7,'checklist',$8,'normal',$9::jsonb,NULLIF($10,'')::uuid,
				NULLIF($11,'')::timestamptz,NULLIF($11,'')::timestamptz,$12::jsonb,NULLIF($13,'')::uuid,NULLIF($13,'')::uuid)
		`, workItemID, projectID, runID, targetNodeID, handoffItem["id"], item.Title, strings.TrimSpace(strings.Join([]string{item.Location, "Từ kế hoạch thi công · " + title}, "\n")), workStatus, string(rolesJSON), assigneeStaffID, item.ScheduledAt, string(evidenceRequirement), actorID); queryErr != nil {
			return nil, queryErr
		}
		checkRows, queryErr := queryTx(ctx, tx, `
			INSERT INTO construction_checks (
				project_id,work_item_id,source_handoff_item_id,title,location,criteria_jsonb,scheduled_at,status,requested_by
			) VALUES ($1,$2,$3,$4,NULLIF($5,''),$6::jsonb,NULLIF($7,'')::timestamptz,'pending',NULLIF($8,'')::uuid)
			RETURNING *
		`, projectID, workItemID, handoffItem["id"], item.Title, item.Location, string(rawCriteria), item.ScheduledAt, actorID)
		if queryErr != nil {
			return nil, queryErr
		}
		if _, queryErr = tx.ExecContext(ctx, `
			UPDATE construction_handoff_items
			SET generated_work_item_id=$2,generated_check_id=$3,updated_at=now()
			WHERE id=$1
		`, handoffItem["id"], workItemID, checkRows[0]["id"]); queryErr != nil {
			return nil, queryErr
		}
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "construction_handoff_activated", "construction_handoff", cleanID(handoff["id"]), "Kích hoạt bàn giao kế hoạch sang thi công", Record{
		"pipeline_run_id": runID, "source_run_node_id": sourceRunNodeID, "target_run_node_id": targetNodeID,
		"source_form_instance_id": sourceFormInstanceID, "source_form_revision_no": sourceFormRevision, "revision_no": revisionNo, "item_count": len(items), "source_snapshot_hash": snapshotHash,
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetConstructionHandoff(ctx, cleanID(handoff["id"]), false)
}

func (r *Repository) GetConstructionHandoff(ctx context.Context, handoffID string, idempotent bool) (Record, error) {
	handoff, err := r.QueryOne(ctx, `
		SELECT h.*,source_node.name AS source_node_name,target_node.name AS target_node_name,u.full_name AS activated_by_name
		FROM construction_handoffs h
		LEFT JOIN pipeline_run_nodes source_node ON source_node.id=h.source_run_node_id
		JOIN pipeline_run_nodes target_node ON target_node.id=h.target_run_node_id
		LEFT JOIN users u ON u.id=h.activated_by
		WHERE h.id=$1
	`, handoffID)
	if err != nil {
		return nil, err
	}
	items, err := r.Query(ctx, `
		SELECT i.*,u.full_name AS assigned_user_name,w.status AS work_item_status,c.status AS check_status,c.result AS check_result
		FROM construction_handoff_items i
		LEFT JOIN users u ON u.id=i.assigned_user_id
		LEFT JOIN work_items w ON w.id=i.generated_work_item_id
		LEFT JOIN construction_checks c ON c.id=i.generated_check_id
		WHERE i.handoff_id=$1 AND i.status <> 'cancelled'
		ORDER BY i.source_item_index,i.created_at,i.id
	`, handoffID)
	if err != nil {
		return nil, err
	}
	handoff["items"] = items
	handoff["idempotent_replay"] = idempotent
	return handoff, nil
}

func handoffWorkItemStatusTx(ctx context.Context, tx *sql.Tx, workItemID, status, actorID string) error {
	if workItemID == "" {
		return nil
	}
	_, err := tx.ExecContext(ctx, `
		UPDATE work_items
		SET status=$2,
			completed_at=CASE WHEN $2='completed' THEN now() ELSE NULL END,
			blocked_at=CASE WHEN $2='blocked' THEN now() ELSE NULL END,
			blocked_reason=CASE WHEN $2='blocked' THEN 'Chờ khắc phục kết quả kiểm tra không đạt' ELSE NULL END,
			updated_by=NULLIF($3,'')::uuid,updated_at=now()
		WHERE id=$1 AND source_type='construction_handoff_item' AND status <> 'cancelled'
	`, workItemID, status, actorID)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
		UPDATE construction_handoff_items
		SET status=CASE WHEN $2='completed' THEN 'completed' ELSE 'activated' END,updated_at=now()
		WHERE generated_work_item_id=$1 AND status <> 'cancelled'
	`, workItemID, status)
	return err
}

func firstRecordValue(rows []Record, key string) any {
	if len(rows) == 0 {
		return nil
	}
	return rows[0][key]
}
