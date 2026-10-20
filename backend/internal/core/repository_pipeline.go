package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/url"
	"strconv"
	"strings"
)

func (r *Repository) GetTemplateDetail(ctx context.Context, id string) (Record, error) {
	template, err := r.GetByID(ctx, "pipeline_templates", id)
	if err != nil {
		return nil, err
	}
	nodes, err := r.Query(ctx, `
		SELECT * FROM pipeline_template_nodes
		WHERE template_id=$1
		ORDER BY order_index, created_at
	`, id)
	if err != nil {
		return nil, err
	}
	edges, _ := r.Query(ctx, `
		SELECT * FROM pipeline_template_edges
		WHERE template_id=$1
		ORDER BY priority, created_at, id
	`, id)
	for _, node := range nodes {
		steps, _ := r.Query(ctx, `
			SELECT * FROM pipeline_template_steps
			WHERE node_id=$1
			ORDER BY order_index, created_at
		`, node["id"])
		node["steps"] = steps
	}
	// Existing pins can be valid even when hidden from the catalog for new selections.
	pinnedForms, err := r.Query(ctx, workflowFormVersionDetailSQL(`
		WHERE v.id IN (
			SELECT DISTINCT pts.form_version_id
			FROM pipeline_template_steps pts
			JOIN pipeline_template_nodes n ON n.id=pts.node_id
			WHERE n.template_id=$1 AND pts.form_version_id IS NOT NULL
		)
		AND v.status='published' AND t.status='active'
	`), id)
	if err != nil {
		return nil, err
	}
	template["nodes"] = nodes
	template["edges"] = edges
	template["pinned_workflow_forms"] = pinnedForms
	return template, nil
}

func (r *Repository) CreateTemplateWithNodes(ctx context.Context, body Record, userID string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	templateID := newID()
	body["id"] = templateID
	body["template_family_id"] = templateID
	body["revision_no"] = 1
	body["engine_version"] = 2
	body["status"] = "draft"
	if body["created_by"] == nil && userID != "" {
		body["created_by"] = userID
	}
	nodes, _ := body["nodes"].([]any)
	delete(body, "nodes")
	_, err = insertRecord(ctx, tx, "pipeline_templates", templateColumns(), body)
	if err != nil {
		return nil, err
	}
	for index, raw := range nodes {
		nodeBody, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		nodeBody["template_id"] = templateID
		if nodeBody["order_index"] == nil {
			nodeBody["order_index"] = index + 1
		}
		steps, _ := nodeBody["steps"].([]any)
		delete(nodeBody, "steps")
		node, err := insertRecord(ctx, tx, "pipeline_template_nodes", templateNodeColumns(), nodeBody)
		if err != nil {
			return nil, err
		}
		nodeID := fmt.Sprint(node["id"])
		for stepIndex, stepRaw := range steps {
			stepBody := Record{}
			if text, ok := stepRaw.(string); ok {
				stepBody["name"] = text
			}
			if obj, ok := stepRaw.(map[string]any); ok {
				stepBody = obj
			}
			stepBody["node_id"] = nodeID
			if stepBody["order_index"] == nil {
				stepBody["order_index"] = stepIndex + 1
			}
			if stepBody["step_type"] == nil {
				stepBody["step_type"] = "task"
			}
			formRequired := workflowFormRequiredForStep(fmt.Sprint(stepBody["step_type"]), stepBody["checklist_required"] == nil || boolValue(stepBody["checklist_required"]))
			if raw, ok := stepBody["form_required"]; ok {
				formRequired = boolValue(raw)
			} else {
				stepBody["form_required"] = workflowFormRequiredForStep(fmt.Sprint(stepBody["step_type"]), stepBody["checklist_required"] == nil || boolValue(stepBody["checklist_required"]))
			}
			if _, provided := stepBody["surface_kind"]; !provided {
				stepBody["surface_kind"] = defaultStepSurfaceKind(fmt.Sprint(stepBody["step_type"]), formRequired)
			}
			if formRequired && cleanID(stepBody["form_version_id"]) == "" {
				formVersionID, err := r.DefaultWorkflowFormVersionID(ctx, fmt.Sprint(stepBody["step_type"]), stepBody["checklist_required"] == nil || boolValue(stepBody["checklist_required"]), fmt.Sprint(stepBody["name"]))
				if err != nil {
					return nil, err
				}
				if formVersionID != "" {
					stepBody["form_version_id"] = formVersionID
				}
			}
			if _, err := insertRecord(ctx, tx, "pipeline_template_steps", templateStepColumns(), stepBody); err != nil {
				return nil, err
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetTemplateDetail(ctx, templateID)
}

func (r *Repository) CloneTemplate(ctx context.Context, id, userID string) (Record, error) {
	detail, err := r.GetTemplateDetail(ctx, id)
	if err != nil {
		return nil, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	familyID := cleanID(detail["template_family_id"])
	if familyID == "" {
		familyID = id
	}
	if _, err := queryTx(ctx, tx, `SELECT id FROM pipeline_templates WHERE id=$1 FOR UPDATE`, familyID); err != nil {
		return nil, err
	}
	var revisionNo int
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(revision_no),0)+1 FROM pipeline_templates WHERE template_family_id=$1 AND status <> 'deleted'`, familyID).Scan(&revisionNo); err != nil {
		return nil, err
	}
	cloneID := newID()
	body := Record{
		"id":                 cloneID,
		"name":               detail["name"],
		"description":        detail["description"],
		"project_type":       detail["project_type"],
		"report_config_json": detail["report_config_json"],
		"version":            revisionNo,
		"status":             "draft",
		"created_by":         userID,
		"template_family_id": familyID,
		"revision_no":        revisionNo,
		"source_template_id": id,
		"engine_version":     detail["engine_version"],
	}
	_, err = insertRecord(ctx, tx, "pipeline_templates", templateColumns(), body)
	if err != nil {
		return nil, err
	}
	nodeIDMap := map[string]string{}
	nodes, _ := detail["nodes"].([]Record)
	for _, node := range nodes {
		oldID := fmt.Sprint(node["id"])
		nodeBody := Record{
			"template_id":          cloneID,
			"business_code":        node["business_code"],
			"name":                 node["name"],
			"description":          node["description"],
			"type":                 node["type"],
			"position_x":           node["position_x"],
			"position_y":           node["position_y"],
			"order_index":          node["order_index"],
			"responsible_role_ids": node["responsible_role_ids"],
			"required_permission":  node["required_permission"],
			"permissions":          nodePermissions(node),
			"entry_conditions":     node["entry_conditions"],
			"exit_conditions":      node["exit_conditions"],
			"approval_rule":        node["approval_rule"],
			"approval_policy_json": node["approval_policy_json"],
			"rework_enabled":       node["rework_enabled"],
			"replanning_enabled":   node["replanning_enabled"],
			"fail_path":            node["fail_path"],
			"status":               node["status"],
		}
		newNode, err := insertRecord(ctx, tx, "pipeline_template_nodes", templateNodeColumns(), nodeBody)
		if err != nil {
			return nil, err
		}
		newNodeID := fmt.Sprint(newNode["id"])
		nodeIDMap[oldID] = newNodeID
		steps, _ := node["steps"].([]Record)
		for _, step := range steps {
			stepBody := Record{
				"node_id":             newNodeID,
				"business_code":       step["business_code"],
				"name":                step["name"],
				"description":         step["description"],
				"step_type":           step["step_type"],
				"required_role_ids":   step["required_role_ids"],
				"required_permission": step["required_permission"],
				"checklist_required":  step["checklist_required"],
				"document_required":   step["document_required"],
				"photo_required":      step["photo_required"],
				"form_version_id":     step["form_version_id"],
				"form_required":       step["form_required"],
				"surface_kind":        step["surface_kind"],
				"domain_action_key":   step["domain_action_key"],
				"order_index":         step["order_index"],
			}
			if _, err := insertRecord(ctx, tx, "pipeline_template_steps", templateStepColumns(), stepBody); err != nil {
				return nil, err
			}
		}
	}
	edges, _ := detail["edges"].([]Record)
	for _, edge := range edges {
		from := nodeIDMap[fmt.Sprint(edge["from_node_id"])]
		to := nodeIDMap[fmt.Sprint(edge["to_node_id"])]
		if from == "" || to == "" {
			continue
		}
		if _, err := insertRecord(ctx, tx, "pipeline_template_edges", edgeColumns(), Record{
			"template_id":      cloneID,
			"from_node_id":     from,
			"to_node_id":       to,
			"condition_type":   edge["condition_type"],
			"condition_config": edge["condition_config"],
			"label":            edge["label"],
			"priority":         edgePriority(edge),
		}); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetTemplateDetail(ctx, cloneID)
}

func (r *Repository) StartPipeline(ctx context.Context, projectID, templateID, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	projectRows, err := queryTx(ctx, tx, `
		SELECT id::text,code,name,COALESCE(created_by::text,'') AS created_by,is_sample,
			COALESCE(pm_staff_id::text,'') AS pm_staff_id
		FROM projects WHERE id=$1 AND status <> 'cancelled' FOR UPDATE
	`, projectID)
	if err != nil {
		return nil, err
	}
	if len(projectRows) == 0 {
		return nil, sql.ErrNoRows
	}
	activeRuns, err := queryTx(ctx, tx, `
		SELECT id
		FROM pipeline_runs
		WHERE project_id=$1 AND status NOT IN ('completed','cancelled')
		ORDER BY started_at DESC
		LIMIT 1
	`, projectID)
	if err != nil {
		return nil, err
	}
	if len(activeRuns) > 0 {
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return r.GetPipelineRun(ctx, projectID)
	}

	if templateID == "" {
		if err := tx.QueryRowContext(ctx, `SELECT COALESCE(pipeline_template_id, (SELECT id FROM pipeline_templates WHERE status='published' ORDER BY revision_no DESC, published_at DESC NULLS LAST, created_at DESC LIMIT 1)) FROM projects WHERE id=$1`, projectID).Scan(&templateID); err != nil {
			return nil, err
		}
	}
	if templateID == "" {
		return nil, errors.New("Công trình chưa có mẫu quy trình")
	}
	templateRows, err := queryTx(ctx, tx, `
		SELECT id, status, engine_version, definition_hash
		FROM pipeline_templates
		WHERE id=$1
		FOR SHARE
	`, templateID)
	if err != nil {
		return nil, err
	}
	if len(templateRows) == 0 {
		return nil, sql.ErrNoRows
	}
	if strings.TrimSpace(fmt.Sprint(templateRows[0]["status"])) != "published" {
		return nil, validationError("Chỉ phiên bản quy trình đã phát hành mới được áp dụng cho công trình", Record{
			"template_id": templateID,
			"status":      templateRows[0]["status"],
		})
	}
	if cleanID(projectRows[0]["created_by"]) != "" && !boolValue(projectRows[0]["is_sample"]) {
		readiness, readinessErr := projectStartupReadiness(ctx, tx, projectID)
		if readinessErr != nil {
			return nil, readinessErr
		}
		if cleanID(readiness["template_id"]) != templateID {
			return nil, validationError("Mẫu quy trình được chọn không khớp cấu hình khởi tạo đã kiểm tra", Record{
				"expected_template_id": readiness["template_id"], "template_id": templateID,
			})
		}
		if !boolValue(readiness["ready_to_start"]) {
			return nil, validationError("Chưa thể bắt đầu quy trình vì thiết lập công trình chưa hoàn tất", Record{
				"blockers": readiness["blockers"], "readiness": readiness,
			})
		}
	}
	if err := validateLegacyProjectEntryPrerequisites(ctx, tx, projectID, templateID); err != nil {
		return nil, err
	}

	formIssues, err := queryTx(ctx, tx, templatePublishFormIssuesSQL(), templateID)
	if err != nil {
		return nil, err
	}
	if len(formIssues) > 0 {
		return nil, validationError("Mẫu quy trình có bước bắt buộc biểu mẫu quy trình nhưng chưa gắn phiên bản biểu mẫu đã phát hành", Record{"issues": formIssues})
	}

	runID := newID()
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO pipeline_runs (id, project_id, template_id, engine_version, template_definition_hash, status, started_at)
		VALUES ($1,$2,$3,$4,$5,'in_progress',now())
	`, runID, projectID, templateID, templateRows[0]["engine_version"], templateRows[0]["definition_hash"]); err != nil {
		return nil, err
	}

	nodes, err := queryTx(ctx, tx, `
		SELECT * FROM pipeline_template_nodes
		WHERE template_id=$1
		ORDER BY order_index, created_at
	`, templateID)
	if err != nil {
		return nil, err
	}
	if len(nodes) == 0 {
		return nil, errors.New("Mẫu quy trình chưa có node")
	}

	currentNodeID := ""
	readyWorkItems := make([]Record, 0)
	runNodeIDByTemplateNodeID := map[string]string{}
	for index, node := range nodes {
		runNodeID := newID()
		runNodeIDByTemplateNodeID[fmt.Sprint(node["id"])] = runNodeID
		status := "pending"
		var startedAt any = nil
		if index == 0 {
			status = "in_progress"
			startedAt = "now"
			currentNodeID = runNodeID
		}
		startedSQL := "NULL"
		if startedAt != nil {
			startedSQL = "now()"
		}
		rawRoles, _ := json.Marshal(node["responsible_role_ids"])
		rawPermissions, _ := json.Marshal(nodePermissions(node))
		rawExit, _ := json.Marshal(node["exit_conditions"])
		rawApprovalPolicy, _ := json.Marshal(node["approval_policy_json"])
		if _, err := tx.ExecContext(ctx, fmt.Sprintf(`
				INSERT INTO pipeline_run_nodes
					(id, run_id, project_id, template_node_id, business_code, name, description, type, order_index, responsible_role_ids, required_permission, permissions, exit_conditions, approval_rule, approval_policy_json, fail_path, status, started_at)
				VALUES ($1,$2,$3,$4,NULLIF($5,''),$6,$7,$8,$9,$10::jsonb,$11,$12::jsonb,$13::jsonb,$14,$15::jsonb,$16,$17,%s)
			`, startedSQL), runNodeID, runID, projectID, node["id"], cleanOptionalText(node["business_code"]), node["name"], node["description"], node["type"], node["order_index"], string(rawRoles), node["required_permission"], string(rawPermissions), string(rawExit), node["approval_rule"], string(rawApprovalPolicy), node["fail_path"], status); err != nil {
			return nil, err
		}
		steps, err := queryTx(ctx, tx, `
			SELECT * FROM pipeline_template_steps
			WHERE node_id=$1
			ORDER BY order_index, created_at
		`, node["id"])
		if err != nil {
			return nil, err
		}
		for _, step := range steps {
			runStepID := newID()
			workItemStatus := "pending"
			if index == 0 {
				workItemStatus = "ready"
			}
			rawStepRoles, _ := json.Marshal(step["required_role_ids"])
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO pipeline_run_steps
					(id, run_node_id, project_id, template_step_id, business_code, name, description, step_type, required_role_ids, required_permission, checklist_required, document_required, photo_required, form_required, form_version_id, surface_kind, domain_action_key, order_index, status)
				VALUES ($1,$2,$3,$4,NULLIF($5,''),$6,$7,$8,$9::jsonb,$10,$11,$12,$13,$14,NULLIF($15,'')::uuid,NULLIF($16,''),NULLIF($17,''),$18,'pending')
			`, runStepID, runNodeID, projectID, step["id"], cleanOptionalText(step["business_code"]), step["name"], step["description"], step["step_type"], string(rawStepRoles), step["required_permission"], step["checklist_required"], step["document_required"], step["photo_required"], step["form_required"], cleanID(step["form_version_id"]), cleanOptionalText(step["surface_kind"]), cleanOptionalText(step["domain_action_key"]), step["order_index"]); err != nil {
				return nil, err
			}
			roleLabels := step["required_role_ids"]
			if len(stringSlice(roleLabels)) == 0 {
				roleLabels = node["responsible_role_ids"]
			}
			rawWorkItemRoles, _ := json.Marshal(roleLabels)
			rawEvidenceRequirement, _ := json.Marshal(Record{
				"checklist_required":  step["checklist_required"],
				"document_required":   step["document_required"],
				"photo_required":      step["photo_required"],
				"form_required":       step["form_required"],
				"form_version_id":     cleanID(step["form_version_id"]),
				"required_permission": step["required_permission"],
			})
			workItemID := newID()
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO work_items
					(id, project_id, pipeline_run_id, run_node_id, run_step_id, source_type, source_id, title, description, work_type, status, priority, role_ids, evidence_requirement_json, created_by, updated_by)
				VALUES ($1,$2,$3,$4,$5,'pipeline_run_step',$5,$6,$7,$8,$9,'normal',$10::jsonb,$11::jsonb,NULLIF($12,'')::uuid,NULLIF($12,'')::uuid)
			`, workItemID, projectID, runID, runNodeID, runStepID, step["name"], step["description"], workItemType(step["step_type"]), workItemStatus, string(rawWorkItemRoles), string(rawEvidenceRequirement), actorID); err != nil {
				return nil, err
			}
			if workItemStatus == "ready" {
				readyWorkItems = append(readyWorkItems, Record{"id": workItemID, "title": step["name"]})
			}
			for _, roleLabel := range stringSlice(roleLabels) {
				if _, err := tx.ExecContext(ctx, `
					INSERT INTO work_item_assignments (work_item_id, assignment_type, role_name, created_by)
					VALUES ($1, 'role', $2, NULLIF($3,'')::uuid)
				`, workItemID, roleLabel, actorID); err != nil {
					return nil, err
				}
			}
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO work_item_activities (work_item_id, project_id, scope_type, scope_id, actor_id, activity_type, to_status, comment, metadata_json)
				VALUES ($1, $2, 'work_item', $1, NULLIF($3,'')::uuid, 'created', $4, 'Đầu việc được tạo từ bước chạy quy trình', $5::jsonb)
			`, workItemID, projectID, actorID, workItemStatus, `{"source_type":"pipeline_run_step"}`); err != nil {
				return nil, err
			}
			if formVersionID := cleanID(step["form_version_id"]); formVersionID != "" {
				formRows, err := queryTx(ctx, tx, `
					INSERT INTO workflow_form_instances
						(project_id, work_item_id, pipeline_run_id, run_node_id, run_step_id, template_step_id, form_template_id, form_version_id, form_type, title, required_role_ids, created_by)
					SELECT $1, $2, $3, $4, $5, $6, v.template_id, v.id, t.form_type, $7, $8::jsonb, NULLIF($9,'')::uuid
					FROM workflow_form_versions v
					JOIN workflow_form_templates t ON t.id=v.template_id
					WHERE v.id=$10
					ON CONFLICT DO NOTHING
					RETURNING *
				`, projectID, workItemID, runID, runNodeID, runStepID, step["id"], step["name"], string(rawWorkItemRoles), actorID, formVersionID)
				if err != nil {
					return nil, err
				}
				if len(formRows) > 0 {
					if _, err := tx.ExecContext(ctx, `
						INSERT INTO workflow_form_events (instance_id, project_id, work_item_id, actor_id, event_type, to_status, comment, metadata_json)
						VALUES ($1, $2, $3, NULLIF($4,'')::uuid, 'created', 'draft', 'Biểu mẫu quy trình được tạo khi khởi động quy trình', $5::jsonb)
					`, formRows[0]["id"], projectID, workItemID, actorID, `{"source":"start_pipeline"}`); err != nil {
						return nil, err
					}
				}
			}
		}
	}
	edges, err := queryTx(ctx, tx, `
		SELECT * FROM pipeline_template_edges
		WHERE template_id=$1
		ORDER BY priority, created_at, id
	`, templateID)
	if err != nil {
		return nil, err
	}
	for _, edge := range edges {
		fromRunNodeID := runNodeIDByTemplateNodeID[fmt.Sprint(edge["from_node_id"])]
		toRunNodeID := runNodeIDByTemplateNodeID[fmt.Sprint(edge["to_node_id"])]
		if fromRunNodeID == "" || toRunNodeID == "" {
			continue
		}
		rawConfig, _ := json.Marshal(edge["condition_config"])
		if _, err := tx.ExecContext(ctx, `
		INSERT INTO pipeline_run_edges
			(id, run_id, project_id, template_edge_id, from_run_node_id, to_run_node_id, condition_type, condition_config, label, priority)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8::jsonb,$9,$10)
	`, newID(), runID, projectID, edge["id"], fromRunNodeID, toRunNodeID, edgeConditionType(edge), string(rawConfig), edge["label"], edgePriority(edge)); err != nil {
			return nil, err
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE pipeline_runs SET current_node_id=$1 WHERE id=$2`, currentNodeID, runID); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE projects SET current_pipeline_run_id=$1, status='planning', progress_percent=0, updated_at=now() WHERE id=$2`, runID, projectID); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "node_started", "pipeline_run", runID, "Tạo Pipeline Run và mở node đầu tiên", Record{}); err != nil {
		return nil, err
	}
	if cleanID(projectRows[0]["created_by"]) != "" && !boolValue(projectRows[0]["is_sample"]) {
		recipients := []string{cleanID(projectRows[0]["created_by"])}
		if pmUserRows, pmErr := queryTx(ctx, tx, `
			SELECT account.id::text AS user_id
			FROM projects project
			JOIN staff person ON person.id=project.pm_staff_id
			JOIN users account ON account.id=person.user_id
			WHERE project.id=$1::uuid
		`, projectID); pmErr != nil {
			return nil, pmErr
		} else if len(pmUserRows) > 0 && cleanID(pmUserRows[0]["user_id"]) != "" && cleanID(pmUserRows[0]["user_id"]) != recipients[0] {
			recipients = append(recipients, cleanID(pmUserRows[0]["user_id"]))
		}
		if _, err := emitNotificationEventTx(ctx, tx, NotificationDomainEvent{
			Key: "project.setup_completed", AggregateType: "project", AggregateID: projectID, ProjectID: projectID, ActorUserID: actorID,
			Payload:  Record{"actor_name": actorName, "project_id": projectID, "project_code": projectRows[0]["code"], "project_name": projectRows[0]["name"]},
			Audience: Record{"user_ids": recipients, "resolve_thread_for_all": true}, CorrelationKey: projectStartupThreadPrefix + projectID,
			IdempotencyKey: "project_setup:" + projectID + ":completed",
		}); err != nil {
			return nil, err
		}
	}
	for _, item := range readyWorkItems {
		if _, err := emitNotificationEventTx(ctx, tx, workItemAssignedNotificationEvent(item, projectRows[0], actorID, actorName, "start")); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetPipelineRun(ctx, projectID)
}

func (r *Repository) GetPipelineRun(ctx context.Context, projectID string) (Record, error) {
	run, err := r.QueryOne(ctx, `
		SELECT pr.*, p.name AS project_name, pt.name AS template_name
		FROM pipeline_runs pr
		JOIN projects p ON p.id=pr.project_id
		JOIN pipeline_templates pt ON pt.id=pr.template_id
		WHERE pr.project_id=$1
		ORDER BY pr.started_at DESC
		LIMIT 1
	`, projectID)
	if err != nil {
		return nil, err
	}
	nodes, err := r.Query(ctx, `
		SELECT * FROM pipeline_run_nodes
		WHERE run_id=$1
		ORDER BY order_index, started_at NULLS LAST
	`, run["id"])
	if err != nil {
		return nil, err
	}
	stepRows, err := r.Query(ctx, `
		SELECT
			rs.*,
			wi.id AS work_item_id,
			wi.status AS work_item_status,
			wi.title AS work_item_title,
			wi.role_ids AS work_item_role_ids,
			wi.evidence_requirement_json,
			COALESCE((
				SELECT COUNT(*)
				FROM evidence_records er
				WHERE er.work_item_id=wi.id
				  AND er.status <> 'rejected'
			), 0) AS evidence_count,
			wfi.id AS workflow_form_instance_id,
			wfi.status AS workflow_form_status,
			wfi.form_type AS workflow_form_type,
			wfi.current_revision AS workflow_form_revision,
			wfi.submitted_at AS workflow_form_submitted_at,
			wfi.form_template_id AS workflow_form_template_id,
			COALESCE(wfi.form_version_id, rs.form_version_id) AS workflow_form_version_id,
			wfv.version_no AS workflow_form_version_no,
			wfv.status AS workflow_form_version_status,
			wft.code AS workflow_form_template_code,
			wft.name AS workflow_form_template_name,
			COALESCE(rs.form_required, false) AS workflow_form_required,
			rs.form_version_id AS required_workflow_form_version_id,
			rs.surface_kind,
			rs.domain_action_key
		FROM pipeline_run_steps rs
		JOIN pipeline_run_nodes rn ON rn.id=rs.run_node_id
		LEFT JOIN work_items wi ON wi.run_step_id=rs.id AND wi.status <> 'cancelled'
		LEFT JOIN workflow_form_instances wfi ON wfi.run_step_id=rs.id AND wfi.status <> 'cancelled'
		LEFT JOIN workflow_form_versions wfv ON wfv.id=COALESCE(wfi.form_version_id, rs.form_version_id)
		LEFT JOIN workflow_form_templates wft ON wft.id=COALESCE(wfi.form_template_id, wfv.template_id)
		WHERE rn.run_id=$1
		ORDER BY rn.order_index, rs.order_index, rs.id
	`, run["id"])
	if err != nil {
		return nil, err
	}
	stepsByNodeID := map[string][]Record{}
	for _, step := range stepRows {
		nodeID := cleanID(step["run_node_id"])
		stepsByNodeID[nodeID] = append(stepsByNodeID[nodeID], step)
	}
	for _, node := range nodes {
		nodeID := cleanID(node["id"])
		node["steps"] = stepsByNodeID[nodeID]
		if nodeID == cleanID(run["current_node_id"]) {
			issues, issueErr := r.NodeCompletionIssues(ctx, node)
			if issueErr != nil {
				return nil, issueErr
			}
			node["completion_issues"] = issues
			run["completion_issues"] = issues
		}
	}
	run["nodes"] = nodes
	return run, nil
}

func (r *Repository) SubmitRunStep(ctx context.Context, stepID string, req WorkItemActionRequest, actorID, actorName string) (Record, error) {
	payload := req.CompletionPayload
	if payload == nil {
		payload = Record{}
	}
	rawPayload, _ := json.Marshal(payload)
	comment := strings.TrimSpace(req.Comment)
	if comment == "" {
		comment = "Hoàn tất qua thao tác gửi bước quy trình cũ"
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	steps, err := queryTx(ctx, tx, `
		UPDATE pipeline_run_steps
		SET status='completed', submitted_by=NULLIF($2,'')::uuid, submitted_at=now()
		WHERE id=$1
		RETURNING *
	`, stepID, actorID)
	if err != nil {
		return nil, err
	}
	if len(steps) == 0 {
		return nil, sql.ErrNoRows
	}
	step := steps[0]
	workItems, err := queryTx(ctx, tx, `
		WITH existing AS (
			SELECT id, status AS previous_status
			FROM work_items
			WHERE run_step_id=$1 AND status <> 'cancelled'
		)
			UPDATE work_items
			SET status='completed',
				completed_at=now(),
				completion_payload_json=$3::jsonb,
				updated_by=NULLIF($2,'')::uuid,
				updated_at=now()
			FROM existing
			WHERE work_items.id=existing.id
			RETURNING work_items.*, existing.previous_status
		`, stepID, actorID, string(rawPayload))
	if err != nil {
		return nil, err
	}
	if len(workItems) > 0 {
		item := workItems[0]
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO work_item_activities (work_item_id, project_id, scope_type, scope_id, actor_id, activity_type, comment, from_status, to_status, metadata_json)
			VALUES ($1, $2, 'work_item', $1, NULLIF($3,'')::uuid, 'completed', $4, $5, 'completed', $6::jsonb)
		`, item["id"], item["project_id"], actorID, comment, item["previous_status"], `{"source":"pipeline_run_step_submit"}`); err != nil {
			return nil, err
		}
	}
	if err := ensureRunStepWorkItemCompletionSyncTx(ctx, tx, stepID); err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, cleanID(step["project_id"]), actorID, actorName, "step_completed", "pipeline_run_step", stepID, fmt.Sprintf("Hoàn tất step %s", step["name"]), Record{"step": step["name"]}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return step, nil
}

func (r *Repository) RunClientWorkflowAutomationForWorkItem(ctx context.Context, workItemID, actorID, actorName string) error {
	workItemID = cleanID(workItemID)
	if workItemID == "" {
		return nil
	}
	contextRow, err := r.clientWorkflowAutomationContext(ctx, workItemID)
	if err != nil {
		return err
	}
	if !isClientOfficialWorkflowContext(contextRow) {
		return nil
	}
	if int(number(contextRow["engine_version"])) >= 2 {
		return nil
	}
	if fmt.Sprint(contextRow["status"]) != "completed" {
		return nil
	}
	actionKey := clientWorkflowAutomationAction(contextRow)
	if actionKey == "" {
		return nil
	}
	outcome, err := r.executeClientWorkflowAutomation(ctx, contextRow, actionKey, actorID, actorName)
	if err != nil {
		outcome = workflowAutomationOutcome{
			ActionKey:    actionKey,
			Status:       "failed",
			Message:      "Tự động hóa quy trình lỗi",
			ErrorMessage: err.Error(),
			Metadata:     Record{"error": err.Error()},
		}
	}
	return r.recordClientWorkflowAutomationOutcome(ctx, contextRow, outcome, actorID, actorName)
}

func (r *Repository) RunRequiredWorkflowAutomationForWorkItem(ctx context.Context, workItemID, actorID, actorName string) error {
	workItemID = cleanID(workItemID)
	if workItemID == "" {
		return nil
	}
	contextRow, err := r.clientWorkflowAutomationContext(ctx, workItemID)
	if err != nil {
		return err
	}
	if int(number(contextRow["engine_version"])) < 2 || !isClientOfficialWorkflowContext(contextRow) {
		return nil
	}
	actionKey := clientWorkflowAutomationAction(contextRow)
	if actionKey == "" {
		return nil
	}
	outcome, executeErr := r.executeClientWorkflowAutomation(ctx, contextRow, actionKey, actorID, actorName)
	if executeErr != nil {
		outcome = workflowAutomationOutcome{
			ActionKey:    actionKey,
			Status:       "failed",
			Message:      "Tự động hóa bắt buộc bị lỗi",
			ErrorMessage: executeErr.Error(),
			Metadata:     Record{"error": executeErr.Error(), "hard_gate": true},
		}
	}
	if outcome.ActionKey == "" {
		outcome.ActionKey = actionKey
	}
	if outcome.Metadata == nil {
		outcome.Metadata = Record{}
	}
	outcome.Metadata["hard_gate"] = true
	if err := r.recordClientWorkflowAutomationOutcome(ctx, contextRow, outcome, actorID, actorName); err != nil {
		return err
	}
	if outcome.Status != "succeeded" && outcome.Status != "skipped" {
		message := outcome.BlockerReason
		if message == "" {
			message = outcome.ErrorMessage
		}
		if message == "" {
			message = outcome.Message
		}
		return validationError("Tự động hóa bắt buộc chưa hoàn tất", Record{
			"action_key":  actionKey,
			"status":      outcome.Status,
			"reason":      message,
			"target_type": outcome.TargetType,
			"target_id":   outcome.TargetID,
		})
	}
	return nil
}

func (r *Repository) clientWorkflowAutomationContext(ctx context.Context, workItemID string) (Record, error) {
	return r.QueryOne(ctx, `
		SELECT
			wi.*,
			pr.id AS pipeline_run_id,
			pr.template_id,
			COALESCE(pr.engine_version, 1) AS engine_version,
			pt.name AS template_name,
			pt.template_family_id,
			pt.revision_no AS template_revision_no,
			rn.id AS context_run_node_id,
			rn.name AS run_node_name,
			rn.type AS run_node_type,
			rn.order_index AS run_node_order_index,
			rs.id AS context_run_step_id,
			rs.name AS run_step_name,
			rs.step_type AS run_step_type,
			rs.domain_action_key
		FROM work_items wi
		LEFT JOIN pipeline_runs pr ON pr.id=wi.pipeline_run_id
		LEFT JOIN pipeline_templates pt ON pt.id=pr.template_id
		LEFT JOIN pipeline_run_nodes rn ON rn.id=wi.run_node_id
		LEFT JOIN pipeline_run_steps rs ON rs.id=wi.run_step_id
		WHERE wi.id=$1
	`, workItemID)
}

func clientWorkflowAutomationAction(contextRow Record) string {
	domainActionKey := cleanID(contextRow["domain_action_key"])
	if domainActionKey != "" {
		// Preserve unknown keys so the required automation gate rejects them.
		return domainActionKey
	}
	if int(number(contextRow["engine_version"])) >= 2 {
		return ""
	}
	nodeName := strings.TrimSpace(fmt.Sprint(contextRow["run_node_name"]))
	switch {
	case strings.HasPrefix(nodeName, "BOQ ("):
		return "generate_procurement_from_boq"
	case strings.HasPrefix(nodeName, "Đầu Vào:"):
		return "input_procurement_gate"
	case strings.HasPrefix(nodeName, "Kết thúc"):
		return "handover_financial_closure_check"
	default:
		return ""
	}
}

func (r *Repository) executeClientWorkflowAutomation(ctx context.Context, contextRow Record, actionKey, actorID, actorName string) (workflowAutomationOutcome, error) {
	var outcome workflowAutomationOutcome
	var err error
	switch actionKey {
	case "generate_procurement_from_boq", "procurement.generate_from_boq":
		outcome, err = r.automateClientBOQProcurement(ctx, contextRow, actorID, actorName)
	case "input_procurement_gate", "procurement.require_approved":
		outcome, err = r.automateClientInputProcurementGate(ctx, contextRow)
	case "contract.require_execution_ready":
		outcome, err = r.automateClientContractExecutionGate(ctx, contextRow)
	case "handover_financial_closure_check", "handover.evaluate_readiness":
		outcome, err = r.automateClientHandoverClosure(ctx, contextRow, actorID, actorName)
	case "construction.activate_handoff", "subcontractor.reconcile_checklist", "acceptance.incoming_material", "subcontractor.prepare_quantity_dossier", "owner.reconcile_acceptance":
		outcome, err = r.automateClientOperationalSourceGate(ctx, contextRow, actionKey)
	default:
		if strings.HasPrefix(actionKey, "source.") {
			outcome, err = r.automateClientOperationalSourceGate(ctx, contextRow, actionKey)
		} else {
			return automationBlocked(actionKey, "Hành động tự động hóa không được hỗ trợ", Record{"action_key": actionKey}), nil
		}
	}
	if outcome.ActionKey != "" {
		outcome.ActionKey = actionKey
	}
	return outcome, err
}

func isClientOfficialWorkflowContext(contextRow Record) bool {
	return cleanID(contextRow["template_id"]) == clientOfficialWorkflowTemplateID ||
		cleanID(contextRow["template_family_id"]) == clientOfficialWorkflowTemplateID
}

func (r *Repository) WorkflowBusinessSourceForWorkItem(ctx context.Context, workItemID string) (Record, error) {
	contextRow, err := r.clientWorkflowAutomationContext(ctx, cleanID(workItemID))
	if err != nil {
		return nil, err
	}
	if !isClientOfficialWorkflowContext(contextRow) {
		return nil, nil
	}
	actionKey := clientWorkflowAutomationAction(contextRow)
	if actionKey == "" {
		return nil, nil
	}
	if int(number(contextRow["template_revision_no"])) < 7 &&
		(int(number(contextRow["engine_version"])) < 2 || actionKey != "procurement.require_approved") {
		return nil, nil
	}
	inspectionKey := actionKey
	switch actionKey {
	case "procurement.generate_from_boq":
		inspectionKey = "source.boq_release_ready"
	case "procurement.require_approved":
		outcome, inspectErr := r.automateClientInputProcurementGate(ctx, contextRow)
		return workflowBusinessSourceRecord(actionKey, outcome), inspectErr
	case "contract.require_execution_ready":
		outcome, inspectErr := r.automateClientContractExecutionGate(ctx, contextRow)
		return workflowBusinessSourceRecord(actionKey, outcome), inspectErr
	case "handover.evaluate_readiness":
		return Record{
			"action_key": actionKey,
			"label":      "Hồ sơ bàn giao và đóng tài chính",
			"ready":      false,
			"status":     "pending_check",
			"message":    "Readiness được kiểm tra fail-closed khi hoàn tất bước.",
			"route":      fmt.Sprintf("/projects/%s/handover", cleanID(contextRow["project_id"])),
		}, nil
	}
	outcome, err := r.automateClientOperationalSourceGate(ctx, contextRow, inspectionKey)
	return workflowBusinessSourceRecord(actionKey, outcome), err
}

func workflowBusinessSourceRecord(actionKey string, outcome workflowAutomationOutcome) Record {
	if outcome.ActionKey == "" && outcome.Status == "" {
		return nil
	}
	metadata := outcome.Metadata
	if metadata == nil {
		metadata = Record{}
	}
	message := outcome.Message
	if outcome.BlockerReason != "" {
		message = outcome.BlockerReason
	}
	return Record{
		"action_key":  actionKey,
		"label":       metadata["label"],
		"ready":       outcome.Status == "succeeded" || outcome.Status == "skipped",
		"status":      outcome.Status,
		"message":     message,
		"route":       metadata["route"],
		"target_type": outcome.TargetType,
		"target_id":   outcome.TargetID,
		"checks":      metadata["checks"],
		"facts":       metadata,
	}
}

func workflowSourceReadinessCheck(key, label string, ready bool, message, route string) Record {
	return Record{
		"key":     key,
		"label":   label,
		"ready":   ready,
		"status":  map[bool]string{true: "ready", false: "blocked"}[ready],
		"message": message,
		"route":   route,
	}
}

func (r *Repository) automateClientOperationalSourceGate(ctx context.Context, contextRow Record, actionKey string) (workflowAutomationOutcome, error) {
	projectID := cleanID(contextRow["project_id"])
	workItemID := cleanID(contextRow["id"])
	metadata := Record{"project_id": projectID}
	var row Record
	var err error
	ready := false
	reason := "Nguồn nghiệp vụ chưa sẵn sàng"
	targetType := "project"
	targetID := projectID

	switch actionKey {
	case "source.design_documents_ready":
		metadata["label"] = "Hồ sơ thiết kế phát hành"
		metadata["route"] = fmt.Sprintf("/projects/%s", projectID)
		releaseState, releaseErr := r.QueryOne(ctx, `
			SELECT
				(SELECT COUNT(*) FROM design_releases WHERE project_id=$1) AS total_releases,
				COALESCE((SELECT id::text FROM design_releases WHERE project_id=$1 AND status='released' LIMIT 1), '') AS current_release_id,
				COALESCE((SELECT revision_label FROM design_releases WHERE project_id=$1 AND status='released' LIMIT 1), '') AS revision_label,
				COALESCE((SELECT design_release_id::text FROM design_release_handoffs WHERE project_id=$1 ORDER BY handed_off_at DESC,id DESC LIMIT 1), '') AS handed_off_release_id,
				COALESCE((SELECT id::text FROM design_release_handoffs WHERE project_id=$1 ORDER BY handed_off_at DESC,id DESC LIMIT 1), '') AS handoff_id
		`, projectID)
		if releaseErr != nil {
			err = releaseErr
			break
		}
		if number(releaseState["total_releases"]) > 0 {
			currentReleaseID := cleanID(releaseState["current_release_id"])
			handedOffReleaseID := cleanID(releaseState["handed_off_release_id"])
			ready = currentReleaseID != "" && currentReleaseID == handedOffReleaseID
			reason = "Revision thiết kế hiện hành chưa được Quản lý dự án bàn giao cho QS"
			if currentReleaseID == "" {
				reason = "Chưa có revision hồ sơ thiết kế đã phát hành"
			}
			targetType, targetID = "design_release", currentReleaseID
			metadata["revision_label"] = cleanOptionalText(releaseState["revision_label"])
			metadata["handoff_id"] = cleanID(releaseState["handoff_id"])
			metadata["handoff_required"] = true
			break
		}
		// Historical runtime compatibility: projects created before design package
		// revisions keep their accepted/submitted evidence gate unchanged.
		row, err = r.QueryOne(ctx, `
			SELECT COUNT(*) AS total,MAX(er.id::text) AS target_id
			FROM evidence_records er
			JOIN work_items wi ON wi.id=er.work_item_id
			LEFT JOIN pipeline_run_nodes node ON node.id=wi.run_node_id
			WHERE er.project_id=$1 AND er.status IN ('submitted','accepted')
			  AND (wi.id=$2 OR node.business_code='design')
		`, projectID, workItemID)
		ready = number(row["total"]) > 0
		reason = "Chưa có hồ sơ/minh chứng thiết kế đã nộp"
		targetType, targetID = "evidence", cleanID(row["target_id"])
	case "source.boq_items_ready":
		metadata["label"] = "BOQ công trình"
		metadata["route"] = fmt.Sprintf("/projects/%s/boq", projectID)
		row, err = r.QueryOne(ctx, `SELECT COUNT(*) AS total,MAX(id::text) AS target_id FROM boq_items WHERE project_id=$1 AND status<>'deleted'`, projectID)
		ready = number(row["total"]) > 0
		reason = "Chưa có dòng BOQ trong module BOQ"
		targetType, targetID = "boq_item", cleanID(row["target_id"])
	case "source.boq_approved", "source.boq_release_ready":
		metadata["label"] = "Bản phát hành BOQ"
		metadata["route"] = fmt.Sprintf("/projects/%s/boq", projectID)
		row, err = r.QueryOne(ctx, `SELECT COUNT(*) AS total,MAX(id::text) AS target_id,MAX(release_no) AS release_no FROM boq_releases WHERE project_id=$1 AND status='released'`, projectID)
		ready = number(row["total"]) > 0
		reason = "BOQ chưa được duyệt để tạo bản phát hành"
		targetType, targetID = "boq_release", cleanID(row["target_id"])
	case "source.contract_version_ready", "source.contract_review_ready", "source.contract_app_signed":
		metadata["label"] = "Hợp đồng chủ đầu tư"
		metadata["route"] = fmt.Sprintf("/projects/%s/contracts", projectID)
		row, err = r.QueryOne(ctx, `
			SELECT c.id::text AS target_id,c.status,cv.status AS version_status,cv.version_no,
			  EXISTS (SELECT 1 FROM contract_version_signature_links link WHERE link.contract_version_id=cv.id) AS app_signed
			FROM contracts c
			LEFT JOIN contract_versions cv ON cv.id=c.current_version_id
			WHERE c.project_id=$1 AND c.contract_type='main_client' AND c.direction='receivable' AND c.status<>'terminated'
			ORDER BY c.updated_at DESC LIMIT 1
		`, projectID)
		targetType, targetID = "contract", cleanID(row["target_id"])
		switch actionKey {
		case "source.contract_version_ready":
			ready = targetID != "" && cleanID(row["version_no"]) != ""
			reason = "Chưa có version hợp đồng chủ đầu tư"
		case "source.contract_review_ready":
			ready = statusIn(fmt.Sprint(row["version_status"]), "approved", "signed")
			reason = "Version hợp đồng chưa được duyệt"
		case "source.contract_app_signed":
			ready = fmt.Sprint(row["version_status"]) == "signed" && boolValue(row["app_signed"])
			reason = "Version hợp đồng chưa được ký trên App"
		}
	case "source.supplier_quotation_ready":
		metadata["label"] = "Báo giá nhà cung cấp"
		metadata["route"] = fmt.Sprintf("/projects/%s/procurement", projectID)
		row, err = r.QueryOne(ctx, `SELECT COUNT(*) AS total,MAX(id::text) AS target_id FROM supplier_quotations WHERE project_id=$1 AND status IN ('qualified','selected')`, projectID)
		ready = number(row["total"]) > 0
		reason = "Chưa có báo giá nhà cung cấp đạt điều kiện"
		targetType, targetID = "supplier_quotation", cleanID(row["target_id"])
	case "source.supplier_selection_ready":
		metadata["label"] = "Lựa chọn báo giá"
		metadata["route"] = fmt.Sprintf("/projects/%s/procurement", projectID)
		row, err = r.QueryOne(ctx, `SELECT COUNT(*) AS total,MAX(id::text) AS target_id FROM supplier_quote_selections WHERE project_id=$1 AND status='active'`, projectID)
		ready = number(row["total"]) > 0
		reason = "Chưa chốt lựa chọn báo giá đầu vào"
		targetType, targetID = "supplier_quote_selection", cleanID(row["target_id"])
	case "source.procurement_contract_ready":
		metadata["label"] = "Hợp đồng đầu vào"
		metadata["route"] = fmt.Sprintf("/projects/%s/contracts", projectID)
		row, err = r.QueryOne(ctx, `SELECT COUNT(*) AS total,MAX(id::text) AS target_id FROM contracts WHERE project_id=$1 AND direction='payable' AND contract_type IN ('supplier','subcontractor','service') AND status IN ('approved','signed','active')`, projectID)
		ready = number(row["total"]) > 0
		reason = "Chưa có hợp đồng đầu vào đã duyệt"
		targetType, targetID = "contract", cleanID(row["target_id"])
	case "source.input_contract_package_ready":
		metadata["label"] = "Gói hợp đồng đầu vào"
		metadata["route"] = fmt.Sprintf("/projects/%s/contracts", projectID)
		row, err = r.QueryOne(ctx, `
			SELECT
			  (SELECT COUNT(*) FROM procurement_requests
			   WHERE project_id=$1 AND status IN ('approved','ordered','partially_received','received')) AS approved_procurements,
			  (SELECT COUNT(*) FROM contracts
			   WHERE project_id=$1 AND direction='payable'
			     AND contract_type IN ('supplier','subcontractor','service')
			     AND status IN ('approved','signed','active')) AS ready_contracts,
			  (SELECT MAX(id::text) FROM contracts
			   WHERE project_id=$1 AND direction='payable'
			     AND contract_type IN ('supplier','subcontractor','service')
			     AND status IN ('approved','signed','active')) AS target_id
		`, projectID)
		procurementReady := number(row["approved_procurements"]) > 0
		contractReady := number(row["ready_contracts"]) > 0
		ready = procurementReady && contractReady
		reason = "Procurement hoặc hợp đồng đầu vào chưa đủ điều kiện"
		metadata["checks"] = []Record{
			workflowSourceReadinessCheck("approved_procurement", "Procurement đã duyệt", procurementReady, "Cần ít nhất một yêu cầu mua đã duyệt.", fmt.Sprintf("/projects/%s/procurement", projectID)),
			workflowSourceReadinessCheck("ready_input_contract", "Hợp đồng đầu vào đủ điều kiện", contractReady, "Hợp đồng nhà cung cấp/thầu phụ cần ở trạng thái đã duyệt, đã ký hoặc đang hiệu lực.", fmt.Sprintf("/projects/%s/contracts", projectID)),
		}
		targetType, targetID = "contract", cleanID(row["target_id"])
	case "subcontractor.reconcile_checklist", "subcontractor.prepare_quantity_dossier", "source.subcontractor_reconciliation_ready":
		metadata["label"] = "Đối chiếu thầu phụ"
		metadata["route"] = fmt.Sprintf("/projects/%s/acceptance", projectID)
		row, err = r.QueryOne(ctx, `SELECT COUNT(*) AS total,MAX(id::text) AS target_id FROM subcontractor_checklist_reconciliations WHERE project_id=$1 AND status='submitted'`, projectID)
		ready = number(row["total"]) > 0
		reason = "Đối chiếu checklist/khối lượng thầu phụ chưa được chốt"
		targetType, targetID = "subcontractor_reconciliation", cleanID(row["target_id"])
	case "source.subcontractor_acceptance_package_ready":
		metadata["label"] = "Gói nghiệm thu thầu phụ"
		metadata["route"] = fmt.Sprintf("/work-items/%s#subcontractor-reconciliation", workItemID)
		row, err = r.QueryOne(ctx, `
			SELECT COUNT(*) AS submitted_reconciliations,MAX(scr.id::text) AS target_id
			FROM subcontractor_checklist_reconciliations scr
			JOIN workflow_form_instances wfi ON wfi.id=scr.form_instance_id AND wfi.project_id=scr.project_id AND wfi.work_item_id=scr.work_item_id
			JOIN work_items wi ON wi.id=scr.work_item_id AND wi.project_id=scr.project_id
			JOIN workflow_form_templates wft ON wft.id=wfi.form_template_id
			WHERE scr.project_id=$1 AND scr.work_item_id=$2 AND scr.status='submitted'
			AND wfi.status='submitted' AND scr.form_revision_no=wfi.current_revision
			AND wft.code='client_subcontractor_acceptance_package'
			AND wfi.id=(SELECT id FROM workflow_form_instances WHERE work_item_id=wi.id AND project_id=wi.project_id AND status<>'cancelled' ORDER BY created_at DESC LIMIT 1)
		`, projectID, workItemID)
		reconciliationReady := number(row["submitted_reconciliations"]) > 0
		ready = reconciliationReady
		reason = "Đối chiếu checklist/khối lượng thầu phụ chưa được chốt"
		metadata["checks"] = []Record{
			workflowSourceReadinessCheck("subcontractor_reconciliation", "Đối chiếu thầu phụ đã chốt", reconciliationReady, "Chốt đối soát của đúng phiên bản biểu mẫu tại đầu việc này.", fmt.Sprintf("/work-items/%s#subcontractor-reconciliation", workItemID)),
		}
		targetType, targetID = "subcontractor_reconciliation", cleanID(row["target_id"])
	case "acceptance.incoming_material":
		metadata["label"] = "Nghiệm thu vật tư đầu vào"
		metadata["route"] = fmt.Sprintf("/projects/%s/acceptance", projectID)
		row, err = r.QueryOne(ctx, `SELECT COUNT(*) AS total,MAX(id::text) AS target_id FROM incoming_acceptance_records WHERE project_id=$1 AND status='submitted'`, projectID)
		ready = number(row["total"]) > 0
		reason = "Hồ sơ nghiệm thu vật tư đầu vào chưa được chốt"
		targetType, targetID = "incoming_acceptance", cleanID(row["target_id"])
	case "construction.activate_handoff":
		metadata["label"] = "Bàn giao kế hoạch thi công"
		metadata["route"] = fmt.Sprintf("/projects/%s/planning", projectID)
		row, err = r.QueryOne(ctx, `SELECT COUNT(*) AS total,MAX(id::text) AS target_id,MAX(revision_no) AS revision_no FROM construction_handoffs WHERE project_id=$1 AND status='activated'`, projectID)
		ready = number(row["total"]) > 0
		reason = "Kế hoạch thi công chưa được kích hoạt bàn giao"
		targetType, targetID = "construction_handoff", cleanID(row["target_id"])
	case "source.final_acceptance_ready":
		metadata["label"] = "Nghiệm thu đầu ra"
		metadata["route"] = fmt.Sprintf("/projects/%s/acceptance", projectID)
		row, err = r.QueryOne(ctx, `SELECT COUNT(*) AS total,MAX(id::text) AS target_id FROM acceptance_records WHERE project_id=$1 AND type='final' AND status='completed' AND result IN ('pass','conditional_pass')`, projectID)
		ready = number(row["total"]) > 0
		reason = "Nghiệm thu đầu ra chưa hoàn tất đạt"
		targetType, targetID = "acceptance", cleanID(row["target_id"])
	case "source.final_acceptance_package_ready":
		metadata["label"] = "Gói nghiệm thu cuối"
		metadata["route"] = fmt.Sprintf("/projects/%s/acceptance", projectID)
		row, err = r.QueryOne(ctx, `
			SELECT
			  (SELECT COUNT(*) FROM owner_client_checklist_reconciliations
			   WHERE project_id=$1 AND status='submitted') AS owner_reconciliations,
			  (SELECT COUNT(*) FROM acceptance_records
			   WHERE project_id=$1 AND type='final' AND status='completed'
			     AND result IN ('pass','conditional_pass')) AS final_acceptances,
			  (SELECT MAX(id::text) FROM acceptance_records
			   WHERE project_id=$1 AND type='final' AND status='completed'
			     AND result IN ('pass','conditional_pass')) AS target_id
		`, projectID)
		ownerReady := number(row["owner_reconciliations"]) > 0
		acceptanceReady := number(row["final_acceptances"]) > 0
		ready = ownerReady && acceptanceReady
		reason = "Đối chiếu chủ đầu tư hoặc nghiệm thu cuối chưa hoàn tất đạt"
		metadata["checks"] = []Record{
			workflowSourceReadinessCheck("owner_reconciliation", "Đối chiếu chủ đầu tư đã chốt", ownerReady, "Gửi bản đối chiếu checklist/khối lượng với chủ đầu tư.", fmt.Sprintf("/projects/%s/acceptance", projectID)),
			workflowSourceReadinessCheck("final_acceptance", "Nghiệm thu cuối đạt", acceptanceReady, "Nghiệm thu cuối phải hoàn tất với kết quả đạt hoặc đạt có điều kiện.", fmt.Sprintf("/projects/%s/acceptance", projectID)),
		}
		targetType, targetID = "acceptance", cleanID(row["target_id"])
	case "owner.reconcile_acceptance":
		metadata["label"] = "Đối chiếu chủ đầu tư"
		metadata["route"] = fmt.Sprintf("/projects/%s/acceptance", projectID)
		row, err = r.QueryOne(ctx, `SELECT COUNT(*) AS total,MAX(id::text) AS target_id FROM owner_client_checklist_reconciliations WHERE project_id=$1 AND status='submitted'`, projectID)
		ready = number(row["total"]) > 0
		reason = "Đối chiếu chủ đầu tư chưa được chốt"
		targetType, targetID = "owner_client_reconciliation", cleanID(row["target_id"])
	case "source.handover_completed":
		metadata["label"] = "Bàn giao vật lý công trình"
		metadata["route"] = fmt.Sprintf("/projects/%s/handover", projectID)
		row, err = r.QueryOne(ctx, `
			SELECT COUNT(*) AS handed_over,MAX(id::text) AS target_id
			FROM handover_records
			WHERE project_id=$1 AND status='handed_over' AND completed_at IS NOT NULL
		`, projectID)
		ready = number(row["handed_over"]) > 0
		reason = "Bàn giao vật lý công trình chưa hoàn tất"
		metadata["checks"] = []Record{
			workflowSourceReadinessCheck("physical_handover", "Hồ sơ bàn giao đã hoàn tất", ready, "Hoàn tất kiểm tra readiness và bàn giao trong workspace Bàn giao.", fmt.Sprintf("/projects/%s/handover", projectID)),
		}
		targetType, targetID = "handover", cleanID(row["target_id"])
	case "source.accounting_handoff_close_ready":
		metadata["label"] = "Bàn giao hồ sơ kế toán"
		metadata["route"] = fmt.Sprintf("/projects/%s/accounting", projectID)
		readiness, readinessErr := r.AccountingHandoffCloseReadiness(ctx, projectID)
		if readinessErr != nil {
			return workflowAutomationOutcome{}, readinessErr
		}
		handoverReady := boolValue(readiness["physical_handover"])
		handoffsReady := number(readiness["pending_handoffs"]) == 0
		ready = handoverReady && handoffsReady
		reason = "Bàn giao vật lý hoặc hồ sơ kế toán còn điều kiện chưa kết thúc"
		metadata["checks"] = []Record{
			workflowSourceReadinessCheck("physical_handover", "Bàn giao vật lý đã hoàn tất", handoverReady, "Hoàn tất bàn giao vật lý trước khi đóng công trình.", fmt.Sprintf("/projects/%s/handover", projectID)),
			workflowSourceReadinessCheck("accounting_handoffs", "Hồ sơ kế toán đã được tiếp nhận", handoffsReady, "Bàn giao và để Kế toán xác nhận mọi hồ sơ mua sắm/mốc thu đủ điều kiện.", fmt.Sprintf("/projects/%s/accounting", projectID)),
		}
		row = readiness
		targetType, targetID = "handover", cleanID(readiness["target_id"])
	default:
		return automationBlocked(actionKey, "Điều kiện nguồn nghiệp vụ không được hỗ trợ", Record{"action_key": actionKey}), nil
	}
	if err != nil {
		if err == sql.ErrNoRows {
			row = Record{}
		} else {
			return workflowAutomationOutcome{}, err
		}
	}
	for key, value := range row {
		if key != "target_id" {
			metadata[key] = value
		}
	}
	if !ready {
		return automationBlockedWithTarget(actionKey, reason, targetType, targetID, metadata), nil
	}
	return automationSucceeded(actionKey, "Nguồn nghiệp vụ đã sẵn sàng", targetType, targetID, metadata), nil
}

func (r *Repository) automateClientContractExecutionGate(ctx context.Context, contextRow Record) (workflowAutomationOutcome, error) {
	projectID := cleanID(contextRow["project_id"])
	rows, err := r.Query(ctx, `
		SELECT c.id AS contract_id,c.status AS contract_status,cv.id AS contract_version_id,cv.status AS version_status,
			EXISTS (
				SELECT 1 FROM contract_version_signature_links link
				WHERE link.contract_version_id=cv.id
			) AS app_signed,
			EXISTS (
				SELECT 1 FROM contract_dispatches dispatch
				WHERE dispatch.contract_id=c.id
				  AND dispatch.contract_version_id=cv.id
				  AND dispatch.status='received'
				  AND dispatch.returned_scan_document_id IS NOT NULL
			) AS returned_scan_received
		FROM contracts c
		LEFT JOIN contract_versions cv ON cv.id=c.current_version_id
		WHERE c.project_id=$1
		  AND c.contract_type='main_client'
		  AND c.direction='receivable'
		  AND c.status <> 'terminated'
		ORDER BY (c.status='active') DESC,c.updated_at DESC,c.id
		LIMIT 1
	`, projectID)
	if err != nil {
		return workflowAutomationOutcome{}, err
	}
	metadata := Record{
		"project_id": projectID,
		"label":      "Gói hợp đồng chủ đầu tư",
		"route":      fmt.Sprintf("/projects/%s/contracts", projectID),
	}
	if len(rows) == 0 {
		metadata["checks"] = []Record{
			workflowSourceReadinessCheck("current_version", "Version hợp đồng hiện hành", false, "Chưa có hợp đồng đầu ra với chủ đầu tư.", fmt.Sprintf("/projects/%s/contracts", projectID)),
			workflowSourceReadinessCheck("app_signature", "Ký workflow trên App", false, "Cần ký version hợp đồng trên App.", fmt.Sprintf("/projects/%s/contracts", projectID)),
			workflowSourceReadinessCheck("returned_scan", "Bản scan giấy đã nhận", false, "Cần liên kết bản scan giấy đã ký.", fmt.Sprintf("/projects/%s/contracts", projectID)),
			workflowSourceReadinessCheck("active_contract", "Hợp đồng đã kích hoạt", false, "Kích hoạt hợp đồng sau khi đủ chữ ký và hồ sơ giấy.", fmt.Sprintf("/projects/%s/contracts", projectID)),
		}
		return automationBlocked("contract.require_execution_ready", "Chưa có hợp đồng đầu ra với chủ đầu tư", metadata), nil
	}
	contract := rows[0]
	metadata["contract_id"] = contract["contract_id"]
	metadata["contract_status"] = contract["contract_status"]
	metadata["contract_version_id"] = contract["contract_version_id"]
	metadata["version_status"] = contract["version_status"]
	metadata["app_signed"] = contract["app_signed"]
	metadata["returned_scan_received"] = contract["returned_scan_received"]
	versionReady := cleanID(contract["contract_version_id"]) != "" && statusIn(fmt.Sprint(contract["version_status"]), "approved", "signed")
	appSigned := fmt.Sprint(contract["version_status"]) == "signed" && boolValue(contract["app_signed"])
	returnedScanReady := boolValue(contract["returned_scan_received"])
	contractActive := fmt.Sprint(contract["contract_status"]) == "active"
	metadata["checks"] = []Record{
		workflowSourceReadinessCheck("current_version", "Version hợp đồng hiện hành", versionReady, "Version hợp đồng cần được duyệt hoặc ký.", fmt.Sprintf("/projects/%s/contracts", projectID)),
		workflowSourceReadinessCheck("app_signature", "Ký workflow trên App", appSigned, "Version hiện hành cần được ký trên App.", fmt.Sprintf("/projects/%s/contracts", projectID)),
		workflowSourceReadinessCheck("returned_scan", "Bản scan giấy đã nhận", returnedScanReady, "Liên kết bản scan hợp đồng giấy đã ký tại lần gửi hợp đồng.", fmt.Sprintf("/projects/%s/contracts", projectID)),
		workflowSourceReadinessCheck("active_contract", "Hợp đồng đã kích hoạt", contractActive, "Kích hoạt hợp đồng sau khi đủ hồ sơ ký.", fmt.Sprintf("/projects/%s/contracts", projectID)),
	}
	if !appSigned {
		return automationBlockedWithTarget("contract.require_execution_ready", "Version hợp đồng chưa được ký trên App bằng chữ ký workflow", "contract", cleanID(contract["contract_id"]), metadata), nil
	}
	if !returnedScanReady {
		return automationBlockedWithTarget("contract.require_execution_ready", "Chưa nhận lại và liên kết bản scan hợp đồng giấy đã ký", "contract", cleanID(contract["contract_id"]), metadata), nil
	}
	if !contractActive {
		return automationBlockedWithTarget("contract.require_execution_ready", "Hợp đồng chưa được kích hoạt sau khi hoàn tất ký và hồ sơ giấy", "contract", cleanID(contract["contract_id"]), metadata), nil
	}
	return automationSucceeded("contract.require_execution_ready", "Hợp đồng đầu ra đã đủ điều kiện mở triển khai", "contract", cleanID(contract["contract_id"]), metadata), nil
}

func (r *Repository) automateClientBOQProcurement(ctx context.Context, contextRow Record, actorID, actorName string) (workflowAutomationOutcome, error) {
	projectID := cleanID(contextRow["project_id"])
	summary, err := r.QueryOne(ctx, `
		SELECT
			COUNT(*) AS total,
			COUNT(*) FILTER (WHERE status <> 'approved' OR NOT final_price_locked) AS not_ready,
			COUNT(*) FILTER (
				WHERE status='approved'
				  AND final_price_locked
				  AND NOT EXISTS (
					SELECT 1
					FROM procurement_request_items pri
					JOIN procurement_requests pr ON pr.id=pri.request_id
					WHERE pri.boq_item_id=boq_items.id
					  AND pr.status <> 'cancelled'
				  )
			) AS ready_without_procurement
		FROM boq_items
		WHERE project_id=$1 AND status <> 'deleted'
	`, projectID)
	if err != nil {
		return workflowAutomationOutcome{}, err
	}
	metadata := Record{
		"node":                      contextRow["run_node_name"],
		"boq_total":                 number(summary["total"]),
		"boq_not_ready":             number(summary["not_ready"]),
		"ready_without_procurement": number(summary["ready_without_procurement"]),
	}
	if number(summary["total"]) == 0 {
		return automationBlocked("generate_procurement_from_boq", "Chưa có dòng BOQ để sinh yêu cầu mua", metadata), nil
	}
	if number(summary["not_ready"]) > 0 {
		return automationBlocked("generate_procurement_from_boq", "BOQ còn dòng chưa duyệt hoặc chưa khóa đơn giá cuối", metadata), nil
	}
	if number(summary["ready_without_procurement"]) > 0 {
		procurement, err := r.GenerateProcurementFromBOQ(ctx, projectID, WorkItemMutationContext{}, actorID, actorName)
		if err != nil {
			var statusErr StatusError
			if errors.As(err, &statusErr) {
				metadata["validation"] = statusErr.Message
				if statusErr.Details != nil {
					metadata["details"] = statusErr.Details
				}
				return automationBlocked("generate_procurement_from_boq", statusErr.Message, metadata), nil
			}
			return workflowAutomationOutcome{}, err
		}
		return automationSucceeded("generate_procurement_from_boq", "Đã sinh yêu cầu mua từ BOQ", "procurement_request", cleanID(procurement["id"]), metadata), nil
	}
	procurement, err := r.firstProjectProcurement(ctx, projectID)
	if err != nil {
		return workflowAutomationOutcome{}, err
	}
	if procurement == nil {
		return automationBlocked("generate_procurement_from_boq", "Không còn dòng BOQ nào cần sinh mua sắm nhưng cũng chưa có yêu cầu mua", metadata), nil
	}
	return automationSucceeded("generate_procurement_from_boq", "Yêu cầu mua từ BOQ đã tồn tại", "procurement_request", cleanID(procurement["id"]), metadata), nil
}

func (r *Repository) automateClientInputProcurementGate(ctx context.Context, contextRow Record) (workflowAutomationOutcome, error) {
	projectID := cleanID(contextRow["project_id"])
	summary, err := r.QueryOne(ctx, `
		SELECT
			COUNT(*) AS total,
			COUNT(*) FILTER (WHERE status NOT IN ('approved','ordered','partially_received','received')) AS not_approved,
			COALESCE((ARRAY_AGG(id::text ORDER BY created_at,id) FILTER (WHERE status NOT IN ('approved','ordered','partially_received','received')))[1], '') AS blocking_request_id,
			COALESCE((ARRAY_AGG(request_code ORDER BY created_at,id) FILTER (WHERE status NOT IN ('approved','ordered','partially_received','received')))[1], '') AS blocking_request_code,
			COALESCE((ARRAY_AGG(title ORDER BY created_at,id) FILTER (WHERE status NOT IN ('approved','ordered','partially_received','received')))[1], '') AS blocking_request_title,
			COALESCE((ARRAY_AGG(status ORDER BY created_at,id) FILTER (WHERE status NOT IN ('approved','ordered','partially_received','received')))[1], '') AS blocking_request_status,
			COALESCE(
				JSONB_AGG(
					JSONB_BUILD_OBJECT('id',id,'request_code',request_code,'title',title,'status',status)
					ORDER BY created_at,id
				) FILTER (WHERE status NOT IN ('approved','ordered','partially_received','received')),
				'[]'::jsonb
			) AS blocking_requests
		FROM procurement_requests
		WHERE project_id=$1 AND status <> 'cancelled'
	`, projectID)
	if err != nil {
		return workflowAutomationOutcome{}, err
	}
	metadata := Record{
		"node":                     contextRow["run_node_name"],
		"label":                    "Yêu cầu mua cần duyệt",
		"route":                    fmt.Sprintf("/projects/%s/procurement", projectID),
		"procurement_total":        number(summary["total"]),
		"procurement_not_approved": number(summary["not_approved"]),
	}
	if number(summary["total"]) == 0 {
		return automationBlockedWithTarget("input_procurement_gate", "Chưa có yêu cầu mua/đầu vào bắt buộc", "project", projectID, metadata), nil
	}
	if number(summary["not_approved"]) > 0 {
		blockingID := cleanID(summary["blocking_request_id"])
		blockingCode := strings.TrimSpace(fmt.Sprint(summary["blocking_request_code"]))
		blockingTitle := strings.TrimSpace(fmt.Sprint(summary["blocking_request_title"]))
		blockingStatus := cleanID(summary["blocking_request_status"])
		blockingLabel := strings.Trim(strings.Join([]string{blockingCode, blockingTitle}, " · "), " ·")
		reason := fmt.Sprintf("Còn %d yêu cầu mua/đầu vào chưa được duyệt", int(number(summary["not_approved"])))
		if blockingLabel != "" {
			reason += ": " + blockingLabel
		}
		if blockingID != "" {
			query := url.Values{}
			query.Set("record", blockingID)
			query.Set("record_type", "procurement_request")
			metadata["route"] = fmt.Sprintf("/projects/%s/procurement?%s", projectID, query.Encode())
			metadata["blocking_request"] = Record{
				"id":           blockingID,
				"request_code": blockingCode,
				"title":        blockingTitle,
				"status":       blockingStatus,
			}
			metadata["blocking_requests"] = summary["blocking_requests"]
			checks := make([]Record, 0, int(number(summary["not_approved"])))
			for _, raw := range anySlice(summary["blocking_requests"]) {
				request := recordFromAny(raw)
				requestID := cleanID(request["id"])
				requestCode := strings.TrimSpace(fmt.Sprint(request["request_code"]))
				requestTitle := strings.TrimSpace(fmt.Sprint(request["title"]))
				requestLabel := strings.Trim(strings.Join([]string{requestCode, requestTitle}, " · "), " ·")
				requestQuery := url.Values{}
				requestQuery.Set("record", requestID)
				requestQuery.Set("record_type", "procurement_request")
				checks = append(checks, Record{
					"key":     "procurement_request:" + requestID,
					"label":   requestLabel,
					"ready":   false,
					"status":  cleanID(request["status"]),
					"message": "Yêu cầu mua này cần được duyệt trước khi hoàn tất bước.",
					"route":   fmt.Sprintf("/projects/%s/procurement?%s", projectID, requestQuery.Encode()),
				})
			}
			metadata["checks"] = checks
		}
		return automationBlockedWithTarget("input_procurement_gate", reason, "procurement_request", blockingID, metadata), nil
	}
	procurement, err := r.firstProjectProcurement(ctx, projectID)
	if err != nil {
		return workflowAutomationOutcome{}, err
	}
	targetID := ""
	if procurement != nil {
		targetID = cleanID(procurement["id"])
	}
	return automationSucceeded("input_procurement_gate", "Đầu vào bắt buộc đã có procurement được duyệt", "procurement_request", targetID, metadata), nil
}

func (r *Repository) automateClientHandoverClosure(ctx context.Context, contextRow Record, actorID, actorName string) (workflowAutomationOutcome, error) {
	projectID := cleanID(contextRow["project_id"])
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return workflowAutomationOutcome{}, err
	}
	defer tx.Rollback()
	handoverID, metadata, err := r.ensureHandoverScaffoldTx(ctx, tx, contextRow, actorID, actorName)
	if err != nil {
		return workflowAutomationOutcome{}, err
	}
	if err := tx.Commit(); err != nil {
		return workflowAutomationOutcome{}, err
	}
	if handoverID == "" {
		return automationBlocked("handover_financial_closure_check", "Chưa có hồ sơ nghiệm thu đầu ra để mở bàn giao", metadata), nil
	}
	handover, err := r.GetHandoverDetail(ctx, handoverID)
	if err != nil {
		return workflowAutomationOutcome{}, err
	}
	if fmt.Sprint(handover["status"]) == "draft" {
		metadata["handover_blockers"] = []string{"Cần hoàn thiện và gửi hồ sơ bàn giao trước khi kiểm tra readiness"}
		return automationBlockedWithTarget("handover_financial_closure_check", "Hồ sơ bàn giao chưa được gửi kiểm tra", "handover", handoverID, metadata), nil
	}
	check, err := r.HandoverCheck(ctx, handoverID, actorID, actorName)
	if err != nil {
		return workflowAutomationOutcome{}, err
	}
	blockers, _ := check["blockers"].([]string)
	if len(blockers) > 0 {
		metadata["handover_blockers"] = blockers
		return automationBlockedWithTarget("handover_financial_closure_check", "Bàn giao/đóng tài chính còn điều kiện chưa hoàn tất", "handover", handoverID, metadata), nil
	}
	metadata["project_id"] = projectID
	return automationSucceeded("handover_financial_closure_check", "Bàn giao và đóng tài chính đã sẵn sàng", "handover", handoverID, metadata), nil
}

func (r *Repository) ensureHandoverScaffoldTx(ctx context.Context, tx *sql.Tx, contextRow Record, actorID, actorName string) (string, Record, error) {
	projectID := cleanID(contextRow["project_id"])
	metadata := Record{"node": contextRow["run_node_name"]}
	acceptanceRows, err := queryTx(ctx, tx, `
		SELECT id, status, result
		FROM acceptance_records
		WHERE project_id=$1 AND type='final' AND status <> 'cancelled'
		ORDER BY updated_at DESC, created_at DESC
		LIMIT 1
	`, projectID)
	if err != nil {
		return "", nil, err
	}
	if len(acceptanceRows) == 0 {
		metadata["handover_blocker"] = "missing_final_acceptance"
		return "", metadata, nil
	}
	finalAcceptanceID := cleanID(acceptanceRows[0]["id"])
	metadata["final_acceptance_id"] = finalAcceptanceID
	handoverRows, err := queryTx(ctx, tx, `
		SELECT id
		FROM handover_records
		WHERE project_id=$1 AND status <> 'deleted'
		ORDER BY created_at DESC
		LIMIT 1
	`, projectID)
	if err != nil {
		return "", nil, err
	}
	if len(handoverRows) > 0 {
		metadata["handover_existing"] = true
		return cleanID(handoverRows[0]["id"]), metadata, nil
	}
	handoverCode := "HO-" + strings.ToUpper(strings.ReplaceAll(newID(), "-", ""))[:10]
	rows, err := queryTx(ctx, tx, `
		INSERT INTO handover_records (project_id,handover_code,status,final_acceptance_id,client_representative,company_representative,notes,created_by)
		SELECT $1,$2,'draft',$3,COALESCE(NULLIF(p.client_name,''),'Chủ đầu tư'),'Quản lý dự án phụ trách','Tự động mở từ quy trình chính thức',NULLIF($4,'')::uuid
		FROM projects p WHERE p.id=$1
		RETURNING id
	`, projectID, handoverCode, finalAcceptanceID, actorID)
	if err != nil {
		return "", nil, err
	}
	if len(rows) == 0 {
		return "", nil, sql.ErrNoRows
	}
	handoverID := cleanID(rows[0]["id"])
	items := []Record{
		{"item_code": "final_acceptance", "category": "quality", "title": "Biên bản nghiệm thu cuối đạt", "source_type": "acceptance", "source_id": finalAcceptanceID},
		{"item_code": "warehouse_closure", "category": "operation", "title": "Kho công trình và vật tư dư đã đóng", "source_type": "system_check", "source_id": handoverID},
		{"item_code": "as_built_documents", "category": "as_built", "title": "Bản vẽ và hồ sơ hoàn công"},
		{"item_code": "operation_manuals", "category": "operation", "title": "Hướng dẫn vận hành/bảo trì"},
		{"item_code": "warranty_documents", "category": "warranty", "title": "Cam kết và hồ sơ bảo hành"},
		{"item_code": "signed_handover_minutes", "category": "legal", "title": "Biên bản bàn giao có xác nhận hai bên"},
	}
	for _, item := range items {
		required := true
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO handover_checklist_items (handover_id,project_id,item_code,category,title,required,status,source_type,source_id)
			VALUES ($1,$2,$3,$4,$5,$6,'pending',NULLIF($7,''),NULLIF($8,'')::uuid)
		`, handoverID, projectID, item["item_code"], item["category"], item["title"], required, item["source_type"], cleanID(item["source_id"])); err != nil {
			return "", nil, err
		}
	}
	metadata["handover_created"] = true
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "handover_created", "handover", handoverID, "Tự động mở hồ sơ bàn giao dạng nháp", metadata); err != nil {
		return "", nil, err
	}
	return handoverID, metadata, nil
}

func (r *Repository) firstProjectProcurement(ctx context.Context, projectID string) (Record, error) {
	rows, err := r.Query(ctx, `
		SELECT pr.*
		FROM procurement_requests pr
		WHERE pr.project_id=$1 AND pr.status <> 'cancelled'
		ORDER BY pr.created_at DESC
		LIMIT 1
	`, projectID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return rows[0], nil
}

func automationSucceeded(actionKey, message, targetType, targetID string, metadata Record) workflowAutomationOutcome {
	return workflowAutomationOutcome{
		ActionKey:  actionKey,
		Status:     "succeeded",
		Message:    message,
		TargetType: targetType,
		TargetID:   cleanID(targetID),
		Metadata:   metadata,
	}
}

func automationBlocked(actionKey, reason string, metadata Record) workflowAutomationOutcome {
	return automationBlockedWithTarget(actionKey, reason, "", "", metadata)
}

func automationBlockedWithTarget(actionKey, reason, targetType, targetID string, metadata Record) workflowAutomationOutcome {
	return workflowAutomationOutcome{
		ActionKey:     actionKey,
		Status:        "blocked",
		Message:       reason,
		TargetType:    targetType,
		TargetID:      cleanID(targetID),
		BlockerReason: reason,
		Metadata:      metadata,
	}
}

func clientWorkflowAutomationSource(contextRow Record, actionKey string) (string, string) {
	workItemID := cleanID(contextRow["id"])
	switch actionKey {
	case "handover_financial_closure_check", "handover.evaluate_readiness":
		if runNodeID := cleanID(contextRow["run_node_id"]); runNodeID != "" {
			return "pipeline_run_node", runNodeID
		}
	}
	return "work_item", workItemID
}

func (r *Repository) recordClientWorkflowAutomationOutcome(ctx context.Context, contextRow Record, outcome workflowAutomationOutcome, actorID, actorName string) error {
	if outcome.ActionKey == "" || outcome.Status == "" {
		return nil
	}
	projectID := cleanID(contextRow["project_id"])
	workItemID := cleanID(contextRow["id"])
	sourceType, sourceID := clientWorkflowAutomationSource(contextRow, outcome.ActionKey)
	idempotencyKey := strings.Join([]string{cleanID(contextRow["template_id"]), sourceType, sourceID, outcome.ActionKey}, ":")
	metadata := outcome.Metadata
	if metadata == nil {
		metadata = Record{}
	}
	metadata["template_id"] = cleanID(contextRow["template_id"])
	metadata["run_node_id"] = cleanID(contextRow["run_node_id"])
	metadata["run_step_id"] = cleanID(contextRow["run_step_id"])
	metadata["work_item_id"] = workItemID
	metadata["automation_source_type"] = sourceType
	metadata["automation_source_id"] = sourceID
	rawMetadata, _ := json.Marshal(metadata)

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	existing, err := queryTx(ctx, tx, `
		SELECT id, status, target_type, target_id, blocker_reason, error_message
		FROM workflow_automation_events
		WHERE idempotency_key=$1
		LIMIT 1
	`, idempotencyKey)
	if err != nil {
		return err
	}
	changed := len(existing) == 0
	if len(existing) > 0 {
		prev := existing[0]
		changed = fmt.Sprint(prev["status"]) != outcome.Status ||
			cleanOptionalText(prev["target_type"]) != outcome.TargetType ||
			cleanID(prev["target_id"]) != cleanID(outcome.TargetID) ||
			cleanOptionalText(prev["blocker_reason"]) != outcome.BlockerReason ||
			cleanOptionalText(prev["error_message"]) != outcome.ErrorMessage
	}
	rows, err := queryTx(ctx, tx, `
		INSERT INTO workflow_automation_events (
			project_id, pipeline_run_id, run_node_id, run_step_id, work_item_id,
			source_type, source_id, action_key, idempotency_key, status,
			target_type, target_id, blocker_reason, error_message, metadata_json,
			actor_id, actor_name
		)
		VALUES (
			$1, NULLIF($2,'')::uuid, NULLIF($3,'')::uuid, NULLIF($4,'')::uuid, NULLIF($5,'')::uuid,
			$6, $7, $8, $9, $10,
			NULLIF($11,''), NULLIF($12,'')::uuid, NULLIF($13,''), NULLIF($14,''), $15::jsonb,
			NULLIF($16,'')::uuid, NULLIF($17,'')
		)
		ON CONFLICT (idempotency_key)
		DO UPDATE SET
			status=EXCLUDED.status,
			target_type=EXCLUDED.target_type,
			target_id=EXCLUDED.target_id,
			blocker_reason=EXCLUDED.blocker_reason,
			error_message=EXCLUDED.error_message,
			metadata_json=EXCLUDED.metadata_json,
			actor_id=EXCLUDED.actor_id,
			actor_name=EXCLUDED.actor_name,
			run_count=workflow_automation_events.run_count + 1,
			updated_at=now()
		RETURNING id
	`, projectID, cleanID(contextRow["pipeline_run_id"]), cleanID(contextRow["run_node_id"]), cleanID(contextRow["run_step_id"]), workItemID,
		sourceType, sourceID, outcome.ActionKey, idempotencyKey, outcome.Status,
		outcome.TargetType, cleanID(outcome.TargetID), outcome.BlockerReason, outcome.ErrorMessage, string(rawMetadata),
		actorID, actorName)
	if err != nil {
		return err
	}
	eventID := ""
	if len(rows) > 0 {
		eventID = cleanID(rows[0]["id"])
	}
	if changed && workItemID != "" {
		activityType := "system_event"
		if outcome.Status == "blocked" {
			activityType = "blocker"
		}
		activityMetadata := Record{
			"automation_event_id": eventID,
			"automation_action":   outcome.ActionKey,
			"automation_status":   outcome.Status,
			"target_type":         outcome.TargetType,
			"target_id":           cleanID(outcome.TargetID),
			"metadata":            metadata,
		}
		if outcome.BlockerReason != "" {
			activityMetadata["blocker_reason"] = outcome.BlockerReason
		}
		if outcome.ErrorMessage != "" {
			activityMetadata["error_message"] = outcome.ErrorMessage
		}
		rawActivityMetadata, _ := json.Marshal(activityMetadata)
		comment := outcome.Message
		if comment == "" {
			comment = "Tự động hóa quy trình đã cập nhật trạng thái"
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO work_item_activities (work_item_id, project_id, scope_type, scope_id, actor_id, activity_type, comment, visibility, metadata_json)
			VALUES ($1, $2, 'work_item', $1, NULLIF($3,'')::uuid, $4, $5, 'project', $6::jsonb)
		`, workItemID, projectID, actorID, activityType, comment, string(rawActivityMetadata)); err != nil {
			return err
		}
		if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "workflow_automation_"+outcome.Status, "work_item", workItemID, comment, activityMetadata); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *Repository) RequestNodeApproval(ctx context.Context, nodeID, actorID, actorName, reason string) (Record, error) {
	node, err := r.GetByID(ctx, "pipeline_run_nodes", nodeID)
	if err != nil {
		return nil, err
	}
	if existing, err := r.Query(ctx, `
		SELECT * FROM approval_requests
		WHERE target_type='pipeline_run_node' AND target_id=$1 AND status='pending'
		ORDER BY created_at DESC LIMIT 1
	`, nodeID); err != nil {
		return nil, err
	} else if len(existing) > 0 {
		return existing[0], nil
	}
	policyRecord, err := normalizeApprovalPolicyValue(node["approval_policy_json"])
	if err != nil {
		return nil, err
	}
	policy := approvalPolicyFromValue(policyRecord)
	runRows, err := r.Query(ctx, `SELECT engine_version FROM pipeline_runs WHERE id=$1`, node["run_id"])
	if err != nil {
		return nil, err
	}
	engineVersion := 1
	if len(runRows) > 0 {
		engineVersion = int(number(runRows[0]["engine_version"]))
	}
	if engineVersion < 2 {
		policyRecord = Record{}
		policy = ApprovalPolicy{Roles: stringSlice(node["responsible_role_ids"])}
	} else if !policy.enabled() {
		roles := stringSlice(node["responsible_role_ids"])
		policyRecord = Record{"mode": "any", "roles": roles, "quorum": 1, "requester_cannot_approve": true, "comment_required": true, "signature_required": false}
		policy = approvalPolicyFromValue(policyRecord)
	}
	request, err := r.withAuditTransaction(ctx, func(tx *sql.Tx) (Record, error) {
		created, mutationErr := insertRecord(ctx, tx, "approval_requests", []ColumnSpec{
			col("target_type"), col("target_id"), col("project_id"), col("requested_by"), col("status"), col("reason"), jsonCol("required_roles"), jsonCol("policy_json"), col("current_stage"),
		}, Record{
			"target_type":    "pipeline_run_node",
			"target_id":      nodeID,
			"project_id":     node["project_id"],
			"requested_by":   actorID,
			"status":         "pending",
			"reason":         reason,
			"required_roles": policy.Roles,
			"policy_json":    policyRecord,
			"current_stage":  0,
		})
		if mutationErr != nil {
			return nil, mutationErr
		}
		result, mutationErr := tx.ExecContext(ctx, `UPDATE pipeline_run_nodes SET status='waiting_approval' WHERE id=$1`, nodeID)
		if mutationErr != nil {
			return nil, mutationErr
		}
		if affected, rowsErr := result.RowsAffected(); rowsErr != nil {
			return nil, rowsErr
		} else if affected != 1 {
			return nil, sql.ErrNoRows
		}
		if err := emitPipelineNodeApprovalRequestedTx(ctx, tx, created, node, actorID, actorName, "", 0, false); err != nil {
			return nil, err
		}
		return created, nil
	}, func(request Record) auditEvent {
		return auditEvent{
			ProjectID: fmt.Sprint(node["project_id"]), ActorID: actorID, ActorName: actorName,
			EventType: "review_requested", TargetType: "pipeline_run_node", TargetID: nodeID,
			Description: fmt.Sprintf("Yêu cầu duyệt node %s", node["name"]), Metadata: Record{"approval_request_id": request["id"]},
		}
	})
	if err != nil {
		return nil, err
	}
	return request, nil
}

func (r *Repository) AdvanceRunNode(ctx context.Context, nodeID, actorID, actorName string) (Record, error) {
	node, err := r.GetByID(ctx, "pipeline_run_nodes", nodeID)
	if err != nil {
		return nil, err
	}
	completionIssues, err := r.NodeCompletionIssues(ctx, node)
	if err != nil {
		return nil, err
	}
	if len(completionIssues) > 0 {
		return nil, validationError("Node chưa đạt điều kiện hoàn tất", Record{"issues": completionIssues})
	}
	if blockers := r.nodeBlockers(ctx, node); len(blockers) > 0 {
		reason := stringsJoin(blockers)
		blocked, err := r.withAuditTransaction(ctx, func(tx *sql.Tx) (Record, error) {
			rows, mutationErr := queryTx(ctx, tx, `UPDATE pipeline_run_nodes SET status='blocked', block_reason=$2 WHERE id=$1 RETURNING *`, nodeID, reason)
			if mutationErr != nil {
				return nil, mutationErr
			}
			if len(rows) == 0 {
				return nil, sql.ErrNoRows
			}
			return rows[0], nil
		}, func(Record) auditEvent {
			return auditEvent{
				ProjectID: fmt.Sprint(node["project_id"]), ActorID: actorID, ActorName: actorName,
				EventType: "node_blocked", TargetType: "pipeline_run_node", TargetID: nodeID,
				Description: reason, Metadata: Record{"blockers": blockers},
			}
		})
		if err != nil {
			return nil, err
		}
		return blocked, nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	nextNode, transition, transitionBlockers, err := r.nextRunNode(ctx, tx, node)
	if err != nil {
		return nil, err
	}
	if len(transitionBlockers) > 0 {
		reason := stringsJoin(transitionBlockers)
		blockedRows, err := queryTx(ctx, tx, `UPDATE pipeline_run_nodes SET status='blocked', block_reason=$2 WHERE id=$1 RETURNING *`, nodeID, reason)
		if err != nil {
			return nil, err
		}
		metadata := Record{"blockers": transitionBlockers}
		if transition != nil {
			metadata["transition"] = transition
		}
		if err := addAuditTx(ctx, tx, fmt.Sprint(node["project_id"]), actorID, actorName, "node_blocked", "pipeline_run_node", nodeID, reason, metadata); err != nil {
			return nil, err
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		if len(blockedRows) > 0 {
			return blockedRows[0], nil
		}
		return node, nil
	}
	if _, err := tx.ExecContext(ctx, `UPDATE pipeline_run_nodes SET status='completed', completed_at=now(), block_reason=NULL WHERE id=$1`, nodeID); err != nil {
		return nil, err
	}
	if nextNode != nil {
		nextID := fmt.Sprint(nextNode["id"])
		if err := skipUnselectedBranchExclusiveNodesTx(ctx, tx, node, nextID); err != nil {
			return nil, err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE pipeline_run_nodes SET status='in_progress', started_at=COALESCE(started_at, now()) WHERE id=$1`, nextID); err != nil {
			return nil, err
		}
		readyItems, err := queryTx(ctx, tx, `UPDATE work_items SET status='ready', updated_at=now() WHERE run_node_id=$1 AND status='pending' RETURNING id::text,title`, nextID)
		if err != nil {
			return nil, err
		}
		projectRows, err := queryTx(ctx, tx, `SELECT id::text,code,name FROM projects WHERE id=$1::uuid`, node["project_id"])
		if err != nil {
			return nil, err
		}
		if len(projectRows) > 0 {
			for _, item := range readyItems {
				if _, err := emitNotificationEventTx(ctx, tx, workItemAssignedNotificationEvent(item, projectRows[0], actorID, actorName, "advance-"+nextID)); err != nil {
					return nil, err
				}
			}
		}
		if _, err := tx.ExecContext(ctx, `UPDATE pipeline_runs SET current_node_id=$1 WHERE id=$2`, nextID, node["run_id"]); err != nil {
			return nil, err
		}
	} else {
		if _, err := tx.ExecContext(ctx, `UPDATE pipeline_runs SET status='completed', completed_at=now(), current_node_id=NULL WHERE id=$1`, node["run_id"]); err != nil {
			return nil, err
		}
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE projects p
		SET progress_percent = COALESCE((
			SELECT ROUND(100.0 * COUNT(*) FILTER (WHERE status='completed') / NULLIF(COUNT(*),0))::int
			FROM pipeline_run_nodes rn
			WHERE rn.run_id=$2 AND rn.status <> 'skipped'
		),0),
		status = CASE WHEN EXISTS (SELECT 1 FROM pipeline_runs pr WHERE pr.id=$2 AND pr.status='completed') THEN 'financial_closure' ELSE p.status END,
		updated_at=now()
		WHERE p.id=$1
	`, node["project_id"], node["run_id"]); err != nil {
		return nil, err
	}
	metadata := Record{"node": node["name"]}
	description := fmt.Sprintf("Hoàn tất node %s", node["name"])
	if transition != nil {
		metadata["transition"] = transition
		if targetName := fmt.Sprint(transition["to_node_name"]); targetName != "" {
			description = fmt.Sprintf("Hoàn tất node %s -> %s", node["name"], targetName)
		}
	}
	if err := addAuditTx(ctx, tx, fmt.Sprint(node["project_id"]), actorID, actorName, "node_completed", "pipeline_run_node", nodeID, description, metadata); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetPipelineRun(ctx, fmt.Sprint(node["project_id"]))
}

func skipUnselectedBranchExclusiveNodesTx(ctx context.Context, tx *sql.Tx, fromNode Record, selectedNodeID string) error {
	skippedRows, err := queryTx(ctx, tx, `
		WITH RECURSIVE selected_reachable(id) AS (
			SELECT $3::uuid
			UNION
			SELECT e.to_run_node_id
			FROM pipeline_run_edges e
			JOIN selected_reachable selected ON selected.id=e.from_run_node_id
			WHERE e.run_id=$1
		), alternative_roots(id) AS (
			SELECT DISTINCT e.to_run_node_id AS id
			FROM pipeline_run_edges e
			WHERE e.run_id=$1
			  AND e.from_run_node_id=$2
			  AND e.to_run_node_id <> $3
		), alternative_reachable(id) AS (
			SELECT id FROM alternative_roots
			UNION
			SELECT e.to_run_node_id
			FROM pipeline_run_edges e
			JOIN alternative_reachable alternative ON alternative.id=e.from_run_node_id
			WHERE e.run_id=$1
		), branch_exclusive(id) AS (
			SELECT id FROM alternative_reachable
			EXCEPT
			SELECT id FROM selected_reachable
		)
		UPDATE pipeline_run_nodes rn
		SET status='skipped', block_reason='Nhánh không được chọn bởi điều kiện chuyển bước'
		FROM branch_exclusive exclusive
		WHERE rn.id=exclusive.id AND rn.status='pending'
		RETURNING rn.id
	`, fromNode["run_id"], fromNode["id"], selectedNodeID)
	if err != nil {
		return err
	}
	for _, skipped := range skippedRows {
		skippedNodeID := skipped["id"]
		if _, err := tx.ExecContext(ctx, `UPDATE pipeline_run_steps SET status='skipped' WHERE run_node_id=$1 AND status='pending'`, skippedNodeID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE work_items SET status='cancelled', updated_at=now() WHERE run_node_id=$1 AND status='pending'`, skippedNodeID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE workflow_form_instances SET status='cancelled', updated_at=now() WHERE run_node_id=$1 AND status='draft'`, skippedNodeID); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) nextRunNode(ctx context.Context, tx *sql.Tx, node Record) (Record, Record, []string, error) {
	edges, err := queryTx(ctx, tx, `
		SELECT e.*, rn.name AS to_node_name, rn.order_index AS to_order_index, rn.status AS to_status
		FROM pipeline_run_edges e
		JOIN pipeline_run_nodes rn ON rn.id=e.to_run_node_id
		WHERE e.run_id=$1 AND e.from_run_node_id=$2 AND rn.status <> 'skipped'
		ORDER BY e.priority, e.created_at, e.id
	`, node["run_id"], node["id"])
	if err != nil {
		return nil, nil, nil, err
	}
	evaluations := []Record{}
	blockers := []string{}
	for _, edge := range edges {
		matched, reason, err := r.runEdgeConditionMet(ctx, tx, edge, node)
		if err != nil {
			return nil, nil, nil, err
		}
		evaluations = append(evaluations, Record{
			"edge_id":        edge["id"],
			"label":          edge["label"],
			"condition_type": edgeConditionType(edge),
			"matched":        matched,
			"reason":         reason,
		})
		if !matched {
			blockers = append(blockers, fmt.Sprintf("%s: %s", edgeDisplayLabel(edge), reason))
			continue
		}
		nextRows, err := queryTx(ctx, tx, `SELECT * FROM pipeline_run_nodes WHERE id=$1 LIMIT 1`, edge["to_run_node_id"])
		if err != nil {
			return nil, nil, nil, err
		}
		if len(nextRows) == 0 {
			continue
		}
		transition := Record{
			"mode":             "edge",
			"edge_id":          edge["id"],
			"template_edge_id": edge["template_edge_id"],
			"condition_type":   edgeConditionType(edge),
			"label":            edge["label"],
			"priority":         edgePriority(edge),
			"reason":           reason,
			"to_node_id":       edge["to_run_node_id"],
			"to_node_name":     edge["to_node_name"],
		}
		return nextRows[0], transition, nil, nil
	}
	if len(edges) > 0 {
		return nil, Record{"mode": "edge", "evaluations": evaluations}, blockers, nil
	}
	return r.nextRunNodeByOrder(ctx, tx, node)
}

func (r *Repository) nextRunNodeByOrder(ctx context.Context, tx *sql.Tx, node Record) (Record, Record, []string, error) {
	nextRows, err := queryTx(ctx, tx, `
		SELECT * FROM pipeline_run_nodes
		WHERE run_id=$1 AND order_index > $2 AND status <> 'skipped'
		ORDER BY order_index
		LIMIT 1
	`, node["run_id"], node["order_index"])
	if err != nil {
		return nil, nil, nil, err
	}
	if len(nextRows) == 0 {
		return nil, Record{"mode": "completed"}, nil, nil
	}
	return nextRows[0], Record{
		"mode":         "fallback_order_index",
		"to_node_id":   nextRows[0]["id"],
		"to_node_name": nextRows[0]["name"],
	}, nil, nil
}

func (r *Repository) runEdgeConditionMet(ctx context.Context, tx *sql.Tx, edge, fromNode Record) (bool, string, error) {
	return evaluateRunEdgeCondition(r, ctx, tx, edge, fromNode)
}

func (r *Repository) workflowFormAnswerEqualsCondition(ctx context.Context, tx *sql.Tx, edge Record, projectID, runNodeID string) (bool, string, error) {
	config := edgeConditionConfig(edge)
	fieldKey := strings.TrimSpace(fmt.Sprint(config["field_key"]))
	if fieldKey == "" || fieldKey == "<nil>" {
		return false, "Điều kiện biểu mẫu quy trình thiếu field_key", nil
	}
	expected, hasExpected := config["value"]
	if !hasExpected {
		expected, hasExpected = config["expected_value"]
	}
	if !hasExpected {
		return false, fmt.Sprintf("Điều kiện biểu mẫu quy trình %s thiếu giá trị mong đợi", fieldKey), nil
	}
	rows, err := queryTx(ctx, tx, `
		SELECT answers_snapshot_json
		FROM workflow_form_instances
		WHERE project_id=$1
			AND run_node_id=$2
			AND status='submitted'
			AND answers_snapshot_json ? $3
		ORDER BY submitted_at DESC NULLS LAST, updated_at DESC, id DESC
		LIMIT 1
	`, projectID, runNodeID, fieldKey)
	if err != nil {
		return false, "", err
	}
	if len(rows) == 0 {
		return false, fmt.Sprintf("Câu trả lời biểu mẫu quy trình %s chưa được gửi", fieldKey), nil
	}
	answers := recordFromAny(rows[0]["answers_snapshot_json"])
	actual, ok := answers[fieldKey]
	if !ok {
		return false, fmt.Sprintf("Câu trả lời biểu mẫu quy trình %s đang thiếu", fieldKey), nil
	}
	matched := workflowFormAnswerValueMatches(actual, expected)
	return matched, boolReason(
		matched,
		fmt.Sprintf("Câu trả lời biểu mẫu quy trình %s khớp %s", fieldKey, edgeExpectedDisplay(expected)),
		fmt.Sprintf("Câu trả lời biểu mẫu quy trình %s không khớp %s", fieldKey, edgeExpectedDisplay(expected)),
	), nil
}

func edgeConditionConfig(edge Record) Record {
	return recordFromAny(edge["condition_config"])
}

func recordFromAny(value any) Record {
	switch typed := value.(type) {
	case Record:
		return typed
	case map[string]any:
		return Record(typed)
	default:
		return Record{}
	}
}

func workflowFormAnswerValueMatches(actual any, expected any) bool {
	switch expectedValues := expected.(type) {
	case []any:
		for _, expectedValue := range expectedValues {
			if workflowFormAnswerValueMatches(actual, expectedValue) {
				return true
			}
		}
		return false
	case []string:
		for _, expectedValue := range expectedValues {
			if workflowFormAnswerValueMatches(actual, expectedValue) {
				return true
			}
		}
		return false
	}
	switch actualValues := actual.(type) {
	case []any:
		for _, actualValue := range actualValues {
			if workflowFormAnswerValueMatches(actualValue, expected) {
				return true
			}
		}
		return false
	case []string:
		for _, actualValue := range actualValues {
			if workflowFormAnswerValueMatches(actualValue, expected) {
				return true
			}
		}
		return false
	}
	if actualBool, ok := actual.(bool); ok {
		expectedBool, err := strconv.ParseBool(strings.TrimSpace(fmt.Sprint(expected)))
		return err == nil && actualBool == expectedBool
	}
	if expectedBool, ok := expected.(bool); ok {
		actualBool, err := strconv.ParseBool(strings.TrimSpace(fmt.Sprint(actual)))
		return err == nil && actualBool == expectedBool
	}
	if actualNumber, ok := strictFloat(actual); ok {
		expectedNumber, expectedOK := strictFloat(expected)
		return expectedOK && actualNumber == expectedNumber
	}
	if expectedNumber, ok := strictFloat(expected); ok {
		actualNumber, actualOK := strictFloat(actual)
		return actualOK && actualNumber == expectedNumber
	}
	return strings.EqualFold(strings.TrimSpace(fmt.Sprint(actual)), strings.TrimSpace(fmt.Sprint(expected)))
}

func strictFloat(value any) (float64, bool) {
	switch typed := value.(type) {
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case string:
		text := strings.TrimSpace(typed)
		if text == "" {
			return 0, false
		}
		parsed, err := strconv.ParseFloat(text, 64)
		return parsed, err == nil && !math.IsNaN(parsed)
	default:
		return 0, false
	}
}

func edgeExpectedDisplay(value any) string {
	raw, err := json.Marshal(value)
	if err != nil {
		return strings.TrimSpace(fmt.Sprint(value))
	}
	return string(raw)
}

func (r *Repository) edgeExists(ctx context.Context, tx *sql.Tx, passReason, failReason, query string, args ...any) (bool, string, error) {
	total, err := r.scalarNumberTx(ctx, tx, query, args...)
	if err != nil {
		return false, "", err
	}
	matched := total > 0
	return matched, boolReason(matched, passReason, failReason), nil
}

func (r *Repository) handoverReadyCondition(ctx context.Context, tx *sql.Tx, projectID string) (bool, string, error) {
	handoverReady, err := r.scalarNumberTx(ctx, tx, `
		SELECT COUNT(*) AS total
		FROM handover_records
		WHERE project_id=$1
			AND documents_ready
			AND warehouse_closed
			AND status <> 'blocked'
	`, projectID)
	if err != nil {
		return false, "", err
	}
	failedAcceptance, err := r.scalarNumberTx(ctx, tx, `
		SELECT COUNT(*) AS total
		FROM acceptance_records
		WHERE project_id=$1 AND result='fail' AND status <> 'cancelled'
	`, projectID)
	if err != nil {
		return false, "", err
	}
	openIssues, err := r.scalarNumberTx(ctx, tx, `
		SELECT COUNT(*) AS total
		FROM issues
		WHERE project_id=$1 AND status IN ('open','in_progress')
	`, projectID)
	if err != nil {
		return false, "", err
	}
	pendingSurplus, err := r.scalarNumberTx(ctx, tx, `
		SELECT COUNT(*) AS total
		FROM site_material_balances
		WHERE project_id=$1 AND surplus_quantity > 0 AND surplus_status <> 'handled'
	`, projectID)
	if err != nil {
		return false, "", err
	}
	matched := handoverReady > 0 && failedAcceptance == 0 && openIssues == 0 && pendingSurplus == 0
	return matched, boolReason(matched, "Hồ sơ bàn giao không còn điểm chặn", "Hồ sơ bàn giao còn điểm chặn"), nil
}

func (r *Repository) scalarNumberTx(ctx context.Context, tx *sql.Tx, query string, args ...any) (float64, error) {
	rows, err := queryTx(ctx, tx, query, args...)
	if err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, nil
	}
	return number(rows[0]["total"]), nil
}

func boolReason(matched bool, passReason, failReason string) string {
	if matched {
		return passReason
	}
	return failReason
}

func edgeDisplayLabel(edge Record) string {
	label := strings.TrimSpace(fmt.Sprint(edge["label"]))
	if label != "" && label != "<nil>" {
		return label
	}
	return edgeConditionType(edge)
}

func nodePermissions(node Record) []string {
	permissions := stringSlice(node["permissions"])
	if len(permissions) > 0 {
		return permissions
	}
	legacy := strings.TrimSpace(fmt.Sprint(node["required_permission"]))
	if legacy != "" && legacy != "<nil>" {
		return []string{legacy}
	}
	return []string{}
}

func stringSlice(value any) []string {
	out := []string{}
	switch v := value.(type) {
	case []string:
		out = append(out, v...)
	case []any:
		for _, item := range v {
			text := strings.TrimSpace(fmt.Sprint(item))
			if text != "" && text != "<nil>" {
				out = append(out, text)
			}
		}
	}
	return out
}

func normalizedRoleList(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		text := strings.TrimSpace(value)
		if text == "" || text == "<nil>" || seen[text] {
			continue
		}
		seen[text] = true
		out = append(out, text)
	}
	return out
}

func edgeConditionType(edge Record) string {
	conditionType := strings.TrimSpace(fmt.Sprint(edge["condition_type"]))
	if conditionType == "" || conditionType == "<nil>" {
		return "always"
	}
	return conditionType
}

func edgePriority(edge Record) int {
	priority, err := parseEdgePriority(edge["priority"])
	if err != nil {
		return 100
	}
	return priority
}

func workItemType(value any) string {
	workType := strings.TrimSpace(fmt.Sprint(value))
	switch workType {
	case "task", "checklist", "approval", "document_upload", "photo_upload", "design_review", "budget_check", "material_check", "daily_report", "acceptance_check", "rework_task", "handover_check", "accounting_check":
		return workType
	default:
		return "task"
	}
}

func (r *Repository) nodeBlockers(ctx context.Context, node Record) []string {
	blockers := []string{}
	nodeType := fmt.Sprint(node["type"])
	projectID := fmt.Sprint(node["project_id"])
	if nodeType == "construction" {
		missing, _ := r.Query(ctx, `
			SELECT COUNT(*) AS total
			FROM site_material_balances
			WHERE project_id=$1 AND issued_quantity < planned_quantity
		`, projectID)
		if len(missing) > 0 && number(missing[0]["total"]) > 0 {
			blockers = append(blockers, "Vật tư bắt buộc chưa được xuất đủ ra công trình")
		}
	}
	return blockers
}

func (r *Repository) BlockRunNode(ctx context.Context, nodeID, reason, actorID, actorName string) (Record, error) {
	node, err := r.withAuditTransaction(ctx, func(tx *sql.Tx) (Record, error) {
		rows, mutationErr := queryTx(ctx, tx, `UPDATE pipeline_run_nodes SET status='blocked', block_reason=$2 WHERE id=$1 RETURNING *`, nodeID, reason)
		if mutationErr != nil {
			return nil, mutationErr
		}
		if len(rows) == 0 {
			return nil, sql.ErrNoRows
		}
		return rows[0], nil
	}, func(node Record) auditEvent {
		return auditEvent{
			ProjectID: fmt.Sprint(node["project_id"]), ActorID: actorID, ActorName: actorName,
			EventType: "node_blocked", TargetType: "pipeline_run_node", TargetID: nodeID,
			Description: reason, Metadata: Record{},
		}
	})
	if err != nil {
		return nil, err
	}
	return node, nil
}

func (r *Repository) ApprovalAction(ctx context.Context, approvalID, action, comment, actorID, actorName string) (Record, error) {
	request, err := r.GetByID(ctx, "approval_requests", approvalID)
	if err != nil {
		return nil, err
	}
	status, ok := approvalStatusForAction(action)
	if !ok {
		return nil, validationError("Hành động phê duyệt không hợp lệ", Record{"action": action})
	}
	event := "approval_granted"
	if action == "reject" {
		event = "approval_rejected"
	} else if action == "request-change" {
		event = "approval_changes_requested"
	}
	projectID := cleanID(request["project_id"])
	targetID := cleanID(request["target_id"])
	targetType := strings.TrimSpace(fmt.Sprint(request["target_type"]))
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var approvalActionID string
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO approval_actions (approval_request_id, user_id, action, comment)
		VALUES ($1, NULLIF($2,'')::uuid, $3, $4)
		RETURNING id::text
	`, approvalID, actorID, action, comment).Scan(&approvalActionID); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE approval_requests SET status=$2, resolved_at=now() WHERE id=$1`, approvalID, status); err != nil {
		return nil, err
	}
	actionLabel := approvalAuditActionLabel(action)
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, event, targetType, targetID, fmt.Sprintf("%s phê duyệt: %s", actionLabel, comment), Record{"approval_request_id": approvalID, "action": action}); err != nil {
		return nil, err
	}
	currentStage := int(number(request["current_stage"]))
	if err := emitPipelineNodeApprovalActionTx(ctx, tx, request, approvalActionID, action, comment, actorID, actorName, currentStage, currentStage, status); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return r.GetByID(ctx, "approval_requests", approvalID)
}
