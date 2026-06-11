package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

func (r *Repository) FindUserByEmail(ctx context.Context, email string) (Record, error) {
	return r.QueryOne(ctx, `
		SELECT id, email, password_hash, full_name, status, created_at, updated_at
		FROM users
		WHERE email=$1 AND status='active'
	`, email)
}

func (r *Repository) UserRoles(ctx context.Context, userID string) ([]string, error) {
	rows, err := r.Query(ctx, `
		SELECT r.display_name
		FROM user_roles ur
		JOIN roles r ON r.id=ur.role_id
		WHERE ur.user_id=$1
		ORDER BY r.display_name
	`, userID)
	if err != nil {
		return nil, err
	}
	roles := make([]string, 0, len(rows))
	for _, row := range rows {
		roles = append(roles, fmt.Sprint(row["display_name"]))
	}
	return roles, nil
}

func (r *Repository) UserPermissions(ctx context.Context, userID string) ([]string, error) {
	rows, err := r.Query(ctx, `
		SELECT DISTINCT p.key
		FROM user_roles ur
		JOIN role_permissions rp ON rp.role_id=ur.role_id
		JOIN permissions p ON p.id=rp.permission_id
		WHERE ur.user_id=$1
		ORDER BY p.key
	`, userID)
	if err != nil {
		return nil, err
	}
	permissions := make([]string, 0, len(rows))
	for _, row := range rows {
		permissions = append(permissions, fmt.Sprint(row["key"]))
	}
	return permissions, nil
}

func (r *Repository) UserHasAnyPermission(ctx context.Context, userID string, required []string) (bool, error) {
	if len(required) == 0 {
		return true, nil
	}
	permissions, err := r.UserPermissions(ctx, userID)
	if err != nil {
		return false, err
	}
	granted := map[string]bool{}
	for _, permission := range permissions {
		granted[permission] = true
	}
	for _, permission := range required {
		if granted[permission] {
			return true, nil
		}
	}
	return false, nil
}

func (r *Repository) RunNodePermissionGate(ctx context.Context, userID, nodeID string) (bool, []string, error) {
	node, err := r.GetByID(ctx, "pipeline_run_nodes", nodeID)
	if err != nil {
		return false, nil, err
	}
	permissions := nodePermissions(node)
	allowed, err := r.UserHasAnyPermission(ctx, userID, permissions)
	return allowed, permissions, err
}

func (r *Repository) RunStepPermissionGate(ctx context.Context, userID, stepID string) (bool, []string, error) {
	step, err := r.GetByID(ctx, "pipeline_run_steps", stepID)
	if err != nil {
		return false, nil, err
	}
	return r.RunNodePermissionGate(ctx, userID, fmt.Sprint(step["run_node_id"]))
}

func (r *Repository) AddAudit(ctx context.Context, projectID, actorID, actorName, eventType, targetType, targetID, description string, metadata any) error {
	raw, _ := json.Marshal(metadata)
	var p any = nil
	var t any = nil
	if projectID != "" {
		p = projectID
	}
	if targetID != "" {
		t = targetID
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO audit_logs (project_id, actor_id, actor_name, event_type, target_type, target_id, description, metadata_json)
		VALUES ($1, NULLIF($2,'')::uuid, $3, $4, $5, $6, $7, $8::jsonb)
	`, p, actorID, actorName, eventType, targetType, t, description, string(raw))
	return err
}

func (r *Repository) ListProjects(ctx context.Context) ([]Record, error) {
	return r.Query(ctx, `
		SELECT p.*, s.full_name AS pm_name, t.name AS pipeline_template_name,
			COALESCE((SELECT COUNT(*) FROM issues i WHERE i.project_id=p.id AND i.status IN ('open','in_progress')),0) AS open_issues,
			COALESCE((SELECT COUNT(*) FROM approval_requests ar WHERE ar.project_id=p.id AND ar.status='pending'),0) AS pending_approvals
		FROM projects p
		LEFT JOIN staff s ON s.id=p.pm_staff_id
		LEFT JOIN pipeline_templates t ON t.id=p.pipeline_template_id
		WHERE p.status <> 'cancelled'
		ORDER BY p.created_at DESC
	`)
}

func (r *Repository) GetProjectDetail(ctx context.Context, id string) (Record, error) {
	project, err := r.QueryOne(ctx, `
		SELECT p.*, s.full_name AS pm_name, t.name AS pipeline_template_name
		FROM projects p
		LEFT JOIN staff s ON s.id=p.pm_staff_id
		LEFT JOIN pipeline_templates t ON t.id=p.pipeline_template_id
		WHERE p.id=$1 AND p.status <> 'cancelled'
	`, id)
	if err != nil {
		return nil, err
	}
	members, _ := r.Query(ctx, `
		SELECT pm.*, s.full_name, s.department, s.position
		FROM project_members pm
		JOIN staff s ON s.id=pm.staff_id
		WHERE pm.project_id=$1
		ORDER BY s.full_name
	`, id)
	project["members"] = members
	return project, nil
}

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
	template["nodes"] = nodes
	template["edges"] = edges
	return template, nil
}

func (r *Repository) CreateTemplateWithNodes(ctx context.Context, body Record, userID string) (Record, error) {
	if body["created_by"] == nil && userID != "" {
		body["created_by"] = userID
	}
	nodes, _ := body["nodes"].([]any)
	delete(body, "nodes")
	template, err := r.Insert(ctx, "pipeline_templates", templateColumns(), body)
	if err != nil {
		return nil, err
	}
	templateID := fmt.Sprint(template["id"])
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
		node, err := r.Insert(ctx, "pipeline_template_nodes", templateNodeColumns(), nodeBody)
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
			if _, err := r.Insert(ctx, "pipeline_template_steps", templateStepColumns(), stepBody); err != nil {
				return nil, err
			}
		}
	}
	return r.GetTemplateDetail(ctx, templateID)
}

func (r *Repository) CloneTemplate(ctx context.Context, id, userID string) (Record, error) {
	detail, err := r.GetTemplateDetail(ctx, id)
	if err != nil {
		return nil, err
	}
	body := Record{
		"name":         fmt.Sprintf("%s (copy)", detail["name"]),
		"description":  detail["description"],
		"project_type": detail["project_type"],
		"version":      1,
		"status":       "draft",
		"created_by":   userID,
	}
	clone, err := r.Insert(ctx, "pipeline_templates", templateColumns(), body)
	if err != nil {
		return nil, err
	}
	cloneID := fmt.Sprint(clone["id"])
	nodeIDMap := map[string]string{}
	nodes, _ := detail["nodes"].([]Record)
	for _, node := range nodes {
		oldID := fmt.Sprint(node["id"])
		nodeBody := Record{
			"template_id":          cloneID,
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
			"rework_enabled":       node["rework_enabled"],
			"replanning_enabled":   node["replanning_enabled"],
			"fail_path":            node["fail_path"],
			"status":               node["status"],
		}
		newNode, err := r.Insert(ctx, "pipeline_template_nodes", templateNodeColumns(), nodeBody)
		if err != nil {
			return nil, err
		}
		newNodeID := fmt.Sprint(newNode["id"])
		nodeIDMap[oldID] = newNodeID
		steps, _ := node["steps"].([]Record)
		for _, step := range steps {
			stepBody := Record{
				"node_id":             newNodeID,
				"name":                step["name"],
				"description":         step["description"],
				"step_type":           step["step_type"],
				"required_role_ids":   step["required_role_ids"],
				"required_permission": step["required_permission"],
				"checklist_required":  step["checklist_required"],
				"document_required":   step["document_required"],
				"photo_required":      step["photo_required"],
				"order_index":         step["order_index"],
			}
			if _, err := r.Insert(ctx, "pipeline_template_steps", templateStepColumns(), stepBody); err != nil {
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
		_, _ = r.Insert(ctx, "pipeline_template_edges", edgeColumns(), Record{
			"template_id":      cloneID,
			"from_node_id":     from,
			"to_node_id":       to,
			"condition_type":   edge["condition_type"],
			"condition_config": edge["condition_config"],
			"label":            edge["label"],
			"priority":         edgePriority(edge),
		})
	}
	return r.GetTemplateDetail(ctx, cloneID)
}

func (r *Repository) StartPipeline(ctx context.Context, projectID, templateID, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if templateID == "" {
		if err := tx.QueryRowContext(ctx, `SELECT COALESCE(pipeline_template_id, (SELECT id FROM pipeline_templates WHERE status='published' ORDER BY created_at LIMIT 1)) FROM projects WHERE id=$1`, projectID).Scan(&templateID); err != nil {
			return nil, err
		}
	}
	if templateID == "" {
		return nil, errors.New("project has no pipeline template")
	}

	runID := newID()
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO pipeline_runs (id, project_id, template_id, status, started_at)
		VALUES ($1,$2,$3,'in_progress',now())
	`, runID, projectID, templateID); err != nil {
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
		return nil, errors.New("template has no nodes")
	}

	currentNodeID := ""
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
		if _, err := tx.ExecContext(ctx, fmt.Sprintf(`
				INSERT INTO pipeline_run_nodes
					(id, run_id, project_id, template_node_id, name, description, type, order_index, responsible_role_ids, required_permission, permissions, exit_conditions, approval_rule, fail_path, status, started_at)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9::jsonb,$10,$11::jsonb,$12::jsonb,$13,$14,$15,%s)
			`, startedSQL), runNodeID, runID, projectID, node["id"], node["name"], node["description"], node["type"], node["order_index"], string(rawRoles), node["required_permission"], string(rawPermissions), string(rawExit), node["approval_rule"], node["fail_path"], status); err != nil {
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
			rawStepRoles, _ := json.Marshal(step["required_role_ids"])
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO pipeline_run_steps
					(id, run_node_id, project_id, template_step_id, name, description, step_type, required_role_ids, required_permission, checklist_required, document_required, photo_required, order_index, status)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8::jsonb,$9,$10,$11,$12,$13,'pending')
			`, newID(), runNodeID, projectID, step["id"], step["name"], step["description"], step["step_type"], string(rawStepRoles), step["required_permission"], step["checklist_required"], step["document_required"], step["photo_required"], step["order_index"]); err != nil {
				return nil, err
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
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO audit_logs (project_id, actor_id, actor_name, event_type, target_type, target_id, description)
		VALUES ($1, NULLIF($2,'')::uuid, $3, 'node_started', 'pipeline_run', $4, 'Tạo Pipeline Run và mở node đầu tiên')
	`, projectID, actorID, actorName, runID); err != nil {
		return nil, err
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
	for _, node := range nodes {
		steps, _ := r.Query(ctx, `
			SELECT * FROM pipeline_run_steps
			WHERE run_node_id=$1
			ORDER BY order_index, created_at
		`, node["id"])
		node["steps"] = steps
	}
	run["nodes"] = nodes
	return run, nil
}

func (r *Repository) SubmitRunStep(ctx context.Context, stepID, actorID, actorName string) (Record, error) {
	step, err := r.QueryOne(ctx, `
		UPDATE pipeline_run_steps
		SET status='completed', submitted_by=NULLIF($2,'')::uuid, submitted_at=now()
		WHERE id=$1
		RETURNING *
	`, stepID, actorID)
	if err != nil {
		return nil, err
	}
	_ = r.AddAudit(ctx, fmt.Sprint(step["project_id"]), actorID, actorName, "step_completed", "pipeline_run_step", stepID, fmt.Sprintf("Hoàn tất step %s", step["name"]), Record{"step": step["name"]})
	return step, nil
}

func (r *Repository) RequestNodeApproval(ctx context.Context, nodeID, actorID, actorName, reason string) (Record, error) {
	node, err := r.GetByID(ctx, "pipeline_run_nodes", nodeID)
	if err != nil {
		return nil, err
	}
	request, err := r.Insert(ctx, "approval_requests", []ColumnSpec{
		col("target_type"), col("target_id"), col("project_id"), col("requested_by"), col("status"), col("reason"), jsonCol("required_roles"),
	}, Record{
		"target_type":    "pipeline_run_node",
		"target_id":      nodeID,
		"project_id":     node["project_id"],
		"requested_by":   actorID,
		"status":         "pending",
		"reason":         reason,
		"required_roles": node["responsible_role_ids"],
	})
	if err != nil {
		return nil, err
	}
	_, _ = r.QueryOne(ctx, `UPDATE pipeline_run_nodes SET status='waiting_approval' WHERE id=$1 RETURNING *`, nodeID)
	_ = r.AddAudit(ctx, fmt.Sprint(node["project_id"]), actorID, actorName, "review_requested", "pipeline_run_node", nodeID, fmt.Sprintf("Yêu cầu duyệt node %s", node["name"]), Record{"approval_request_id": request["id"]})
	return request, nil
}

func (r *Repository) AdvanceRunNode(ctx context.Context, nodeID, actorID, actorName string) (Record, error) {
	node, err := r.GetByID(ctx, "pipeline_run_nodes", nodeID)
	if err != nil {
		return nil, err
	}
	if blockers := r.nodeBlockers(ctx, node); len(blockers) > 0 {
		reason := stringsJoin(blockers)
		blocked, err := r.QueryOne(ctx, `UPDATE pipeline_run_nodes SET status='blocked', block_reason=$2 WHERE id=$1 RETURNING *`, nodeID, reason)
		if err != nil {
			return nil, err
		}
		_ = r.AddAudit(ctx, fmt.Sprint(node["project_id"]), actorID, actorName, "node_blocked", "pipeline_run_node", nodeID, reason, Record{"blockers": blockers})
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
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		metadata := Record{"blockers": transitionBlockers}
		if transition != nil {
			metadata["transition"] = transition
		}
		_ = r.AddAudit(ctx, fmt.Sprint(node["project_id"]), actorID, actorName, "node_blocked", "pipeline_run_node", nodeID, reason, metadata)
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
		if _, err := tx.ExecContext(ctx, `UPDATE pipeline_run_nodes SET status='in_progress', started_at=COALESCE(started_at, now()) WHERE id=$1`, nextID); err != nil {
			return nil, err
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
			WHERE rn.run_id=$2
		),0),
		status = CASE WHEN EXISTS (SELECT 1 FROM pipeline_runs pr WHERE pr.id=$2 AND pr.status='completed') THEN 'financial_closure' ELSE p.status END,
		updated_at=now()
		WHERE p.id=$1
	`, node["project_id"], node["run_id"]); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
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
	_ = r.AddAudit(ctx, fmt.Sprint(node["project_id"]), actorID, actorName, "node_completed", "pipeline_run_node", nodeID, description, metadata)
	return r.GetPipelineRun(ctx, fmt.Sprint(node["project_id"]))
}

func (r *Repository) nextRunNode(ctx context.Context, tx *sql.Tx, node Record) (Record, Record, []string, error) {
	edges, err := queryTx(ctx, tx, `
		SELECT e.*, rn.name AS to_node_name, rn.order_index AS to_order_index, rn.status AS to_status
		FROM pipeline_run_edges e
		JOIN pipeline_run_nodes rn ON rn.id=e.to_run_node_id
		WHERE e.run_id=$1 AND e.from_run_node_id=$2
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
		WHERE run_id=$1 AND order_index > $2
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
	projectID := fmt.Sprint(fromNode["project_id"])
	nodeID := fmt.Sprint(fromNode["id"])

	switch edgeConditionType(edge) {
	case "", "always":
		return true, "condition always matched", nil
	case "order_index":
		matched := number(edge["to_order_index"]) > number(fromNode["order_index"])
		return matched, boolReason(matched, "target node is after source node", "target node is not after source node"), nil
	case "approval_approved":
		return r.edgeExists(ctx, tx, "node approval has been approved", "node approval is not approved", `
			SELECT COUNT(*) AS total
			FROM approval_requests
			WHERE project_id=$1 AND target_type='pipeline_run_node' AND target_id=$2 AND status='approved'
		`, projectID, nodeID)
	case "approval_rejected":
		return r.edgeExists(ctx, tx, "node approval has been rejected", "node approval is not rejected", `
			SELECT COUNT(*) AS total
			FROM approval_requests
			WHERE project_id=$1 AND target_type='pipeline_run_node' AND target_id=$2 AND status='rejected'
		`, projectID, nodeID)
	case "acceptance_passed":
		return r.edgeExists(ctx, tx, "acceptance passed", "no completed passed acceptance", `
			SELECT COUNT(*) AS total
			FROM acceptance_records
			WHERE project_id=$1 AND result='pass' AND status='completed'
		`, projectID)
	case "acceptance_failed":
		return r.edgeExists(ctx, tx, "acceptance failed", "no failed acceptance", `
			SELECT COUNT(*) AS total
			FROM acceptance_records
			WHERE project_id=$1 AND result='fail'
		`, projectID)
	case "rework_required":
		return r.edgeExists(ctx, tx, "rework is required", "no open rework requirement", `
			SELECT COUNT(*) AS total
			FROM (
				SELECT id FROM rework_requests WHERE project_id=$1 AND status IN ('draft','approved','in_progress')
				UNION ALL
				SELECT id FROM issues WHERE project_id=$1 AND requires_rework AND status IN ('open','in_progress')
			) required_rework
		`, projectID)
	case "rework_completed":
		total, err := r.scalarNumberTx(ctx, tx, `SELECT COUNT(*) AS total FROM rework_requests WHERE project_id=$1`, projectID)
		if err != nil {
			return false, "", err
		}
		open, err := r.scalarNumberTx(ctx, tx, `SELECT COUNT(*) AS total FROM rework_requests WHERE project_id=$1 AND status IN ('draft','approved','in_progress')`, projectID)
		if err != nil {
			return false, "", err
		}
		matched := total > 0 && open == 0
		return matched, boolReason(matched, "all rework requests are done", "rework requests are still open or do not exist"), nil
	case "replanning_required":
		return r.edgeExists(ctx, tx, "replanning is required", "no open replanning requirement", `
			SELECT COUNT(*) AS total
			FROM (
				SELECT id FROM replanning_requests WHERE project_id=$1 AND status IN ('draft','submitted')
				UNION ALL
				SELECT id FROM issues WHERE project_id=$1 AND requires_replanning AND status IN ('open','in_progress')
			) required_replanning
		`, projectID)
	case "materials_issued":
		missing, err := r.scalarNumberTx(ctx, tx, `
			SELECT COUNT(*) AS total
			FROM site_material_balances
			WHERE project_id=$1 AND issued_quantity < planned_quantity
		`, projectID)
		if err != nil {
			return false, "", err
		}
		matched := missing == 0
		return matched, boolReason(matched, "all planned materials have been issued", "planned materials are still missing"), nil
	case "boq_approved":
		total, err := r.scalarNumberTx(ctx, tx, `SELECT COUNT(*) AS total FROM boq_items WHERE project_id=$1 AND status <> 'deleted'`, projectID)
		if err != nil {
			return false, "", err
		}
		notApproved, err := r.scalarNumberTx(ctx, tx, `SELECT COUNT(*) AS total FROM boq_items WHERE project_id=$1 AND status <> 'deleted' AND status <> 'approved'`, projectID)
		if err != nil {
			return false, "", err
		}
		matched := total > 0 && notApproved == 0
		return matched, boolReason(matched, "all BOQ items are approved", "BOQ is missing or has unapproved items"), nil
	case "procurement_approved":
		total, err := r.scalarNumberTx(ctx, tx, `SELECT COUNT(*) AS total FROM procurement_requests WHERE project_id=$1 AND status <> 'cancelled'`, projectID)
		if err != nil {
			return false, "", err
		}
		notApproved, err := r.scalarNumberTx(ctx, tx, `
			SELECT COUNT(*) AS total
			FROM procurement_requests
			WHERE project_id=$1 AND status <> 'cancelled' AND status NOT IN ('approved','ordered','partially_received','received')
		`, projectID)
		if err != nil {
			return false, "", err
		}
		matched := total > 0 && notApproved == 0
		return matched, boolReason(matched, "procurement requests are approved", "procurement is missing or has unapproved requests"), nil
	case "handover_ready":
		return r.handoverReadyCondition(ctx, tx, projectID)
	case "financial_ready":
		openDebt, err := r.scalarNumberTx(ctx, tx, `
			SELECT COUNT(*) AS total
			FROM (
				SELECT id FROM payables WHERE project_id=$1 AND status <> 'cancelled' AND remaining_amount > 0
				UNION ALL
				SELECT id FROM receivables WHERE project_id=$1 AND status <> 'cancelled' AND remaining_amount > 0
			) open_debt
		`, projectID)
		if err != nil {
			return false, "", err
		}
		matched := openDebt == 0
		return matched, boolReason(matched, "no open payable or receivable balance", "payable or receivable balances are still open"), nil
	default:
		return false, fmt.Sprintf("unsupported condition_type %s", edgeConditionType(edge)), nil
	}
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
			AND payables_recorded
			AND receivables_recorded
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
	return matched, boolReason(matched, "handover checklist has no blockers", "handover still has blockers"), nil
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

func (r *Repository) nodeBlockers(ctx context.Context, node Record) []string {
	blockers := []string{}
	nodeType := fmt.Sprint(node["type"])
	projectID := fmt.Sprint(node["project_id"])
	if nodeType == "construction" {
		project, err := r.GetByID(ctx, "projects", projectID)
		if err == nil {
			contractAmount := number(project["contract_amount"])
			rows, _ := r.Query(ctx, `
				SELECT COALESCE(SUM(received_amount),0) AS received
				FROM receivables
				WHERE project_id=$1
			`, projectID)
			received := 0.0
			if len(rows) > 0 {
				received = number(rows[0]["received"])
			}
			if contractAmount > 0 && received/contractAmount < 0.30 {
				blockers = append(blockers, "Chưa thu đủ tối thiểu 30% giá trị hợp đồng để bắt đầu thi công")
			}
		}
		missing, _ := r.Query(ctx, `
			SELECT COUNT(*) AS total
			FROM site_material_balances
			WHERE project_id=$1 AND issued_quantity < planned_quantity
		`, projectID)
		if len(missing) > 0 && number(missing[0]["total"]) > 0 {
			blockers = append(blockers, "Vật tư required chưa được xuất đủ ra công trình")
		}
	}
	return blockers
}

func (r *Repository) BlockRunNode(ctx context.Context, nodeID, reason, actorID, actorName string) (Record, error) {
	node, err := r.QueryOne(ctx, `UPDATE pipeline_run_nodes SET status='blocked', block_reason=$2 WHERE id=$1 RETURNING *`, nodeID, reason)
	if err != nil {
		return nil, err
	}
	_ = r.AddAudit(ctx, fmt.Sprint(node["project_id"]), actorID, actorName, "node_blocked", "pipeline_run_node", nodeID, reason, Record{})
	return node, nil
}

func (r *Repository) ApprovalAction(ctx context.Context, approvalID, action, comment, actorID, actorName string) (Record, error) {
	request, err := r.GetByID(ctx, "approval_requests", approvalID)
	if err != nil {
		return nil, err
	}
	status := "pending"
	if action == "approve" {
		status = "approved"
	} else if action == "reject" {
		status = "rejected"
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO approval_actions (approval_request_id, user_id, action, comment)
		VALUES ($1, NULLIF($2,'')::uuid, $3, $4)
	`, approvalID, actorID, action, comment); err != nil {
		return nil, err
	}
	if status != "pending" {
		if _, err := tx.ExecContext(ctx, `UPDATE approval_requests SET status=$2, resolved_at=now() WHERE id=$1`, approvalID, status); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	event := "approval_granted"
	if action == "reject" {
		event = "approval_rejected"
	}
	_ = r.AddAudit(ctx, fmt.Sprint(request["project_id"]), actorID, actorName, event, fmt.Sprint(request["target_type"]), fmt.Sprint(request["target_id"]), fmt.Sprintf("%s approval: %s", action, comment), Record{"approval_request_id": approvalID})
	return r.GetByID(ctx, "approval_requests", approvalID)
}

func (r *Repository) GenerateProcurementFromBOQ(ctx context.Context, projectID, actorID, actorName string) (Record, error) {
	rows, err := r.Query(ctx, `
		SELECT b.*, m.name AS material_name, m.standard_price
		FROM boq_items b
		LEFT JOIN materials m ON m.id=b.material_id
		WHERE b.project_id=$1 AND b.status IN ('approved','submitted')
	`, projectID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, errors.New("no BOQ items available to generate procurement")
	}
	total := 0.0
	for _, row := range rows {
		price := number(row["unit_price"])
		if price == 0 {
			price = number(row["standard_price"])
		}
		total += number(row["quantity"]) * price
	}
	requestCode := fmt.Sprintf("PR-%s-%d", projectID[:8], timeNowUnix())
	req, err := r.Insert(ctx, "procurement_requests", procurementColumns(), Record{
		"project_id":             projectID,
		"request_code":           requestCode,
		"title":                  "Yêu cầu mua vật tư sinh từ BOQ",
		"requested_by":           actorID,
		"status":                 "submitted",
		"total_estimated_amount": total,
		"budget_status":          "within_budget",
	})
	if err != nil {
		return nil, err
	}
	requestID := fmt.Sprint(req["id"])
	for _, row := range rows {
		price := number(row["unit_price"])
		if price == 0 {
			price = number(row["standard_price"])
		}
		_, _ = r.Insert(ctx, "procurement_request_items", procurementItemColumns(), Record{
			"request_id":           requestID,
			"material_id":          row["material_id"],
			"boq_item_id":          row["id"],
			"quantity_planned":     row["quantity"],
			"quantity_requested":   row["quantity"],
			"unit":                 row["unit"],
			"estimated_unit_price": price,
			"supplier_name":        row["default_supplier"],
			"delivery_status":      "pending",
			"approval_status":      "pending",
		})
	}
	_ = r.AddAudit(ctx, projectID, actorID, actorName, "material_requested", "procurement_request", requestID, "Sinh yêu cầu mua vật tư từ BOQ", Record{"total": total})
	return r.GetProcurementDetail(ctx, requestID)
}

func (r *Repository) GetProcurementDetail(ctx context.Context, requestID string) (Record, error) {
	req, err := r.GetByID(ctx, "procurement_requests", requestID)
	if err != nil {
		return nil, err
	}
	items, _ := r.Query(ctx, `
		SELECT pri.*, m.name AS material_name, b.work_item_name
		FROM procurement_request_items pri
		LEFT JOIN materials m ON m.id=pri.material_id
		LEFT JOIN boq_items b ON b.id=pri.boq_item_id
		WHERE pri.request_id=$1
		ORDER BY pri.created_at
	`, requestID)
	req["items"] = items
	return req, nil
}

func (r *Repository) ProcurementAction(ctx context.Context, id, action, actorID, actorName string) (Record, error) {
	status := map[string]string{
		"submit":  "waiting_approval",
		"approve": "approved",
		"order":   "ordered",
		"receive": "received",
	}[action]
	if status == "" {
		return nil, errors.New("invalid procurement action")
	}
	req, err := r.QueryOne(ctx, `UPDATE procurement_requests SET status=$2, updated_at=now() WHERE id=$1 RETURNING *`, id, status)
	if err != nil {
		return nil, err
	}
	event := map[string]string{"submit": "review_requested", "approve": "material_approved", "order": "material_ordered", "receive": "warehouse_received"}[action]
	_ = r.AddAudit(ctx, fmt.Sprint(req["project_id"]), actorID, actorName, event, "procurement_request", id, fmt.Sprintf("Cập nhật yêu cầu mua sang trạng thái %s", status), Record{})
	return r.GetProcurementDetail(ctx, id)
}

func (r *Repository) CreateStockMovement(ctx context.Context, req StockActionRequest, actorID, actorName string) (Record, error) {
	if req.MovementType == "" {
		req.MovementType = "receive"
	}
	code := fmt.Sprintf("SM-%s-%d", req.MovementType, timeNowUnix())
	body := Record{
		"movement_code":  code,
		"project_id":     req.ProjectID,
		"warehouse_id":   req.WarehouseID,
		"material_id":    req.MaterialID,
		"movement_type":  req.MovementType,
		"quantity":       req.Quantity,
		"unit":           req.Unit,
		"reason":         req.Reason,
		"reference_type": req.ReferenceType,
		"reference_id":   req.ReferenceID,
		"created_by":     actorID,
		"approved_by":    actorID,
	}
	movement, err := r.Insert(ctx, "stock_movements", []ColumnSpec{
		col("movement_code"), col("project_id"), col("warehouse_id"), col("material_id"), col("movement_type"), col("quantity"), col("unit"), col("reason"), col("reference_type"), col("reference_id"), col("created_by"), col("approved_by"),
	}, body)
	if err != nil {
		return nil, err
	}
	if req.ProjectID != "" && (req.MovementType == "issue" || req.MovementType == "return_to_warehouse" || req.MovementType == "return_to_subcontractor") {
		issuedDelta := req.Quantity
		if req.MovementType != "issue" {
			issuedDelta = -req.Quantity
		}
		_, _ = r.db.ExecContext(ctx, `
			INSERT INTO site_material_balances (project_id, material_id, issued_quantity, surplus_quantity, surplus_status)
			VALUES ($1,$2,$3,GREATEST($3,0),'pending')
			ON CONFLICT (project_id, material_id)
			DO UPDATE SET issued_quantity = site_material_balances.issued_quantity + EXCLUDED.issued_quantity,
				surplus_quantity = GREATEST(site_material_balances.issued_quantity + EXCLUDED.issued_quantity - site_material_balances.used_quantity, 0),
				surplus_status = CASE WHEN $4 IN ('return_to_warehouse','return_to_subcontractor') THEN 'handled' ELSE site_material_balances.surplus_status END,
				updated_at=now()
		`, req.ProjectID, req.MaterialID, issuedDelta, req.MovementType)
	}
	event := map[string]string{
		"receive":                 "warehouse_received",
		"issue":                   "warehouse_issued",
		"transfer":                "warehouse_transferred",
		"return_to_warehouse":     "surplus_stored",
		"return_to_subcontractor": "surplus_returned_to_subcontractor",
	}[req.MovementType]
	if event == "" {
		event = "stock_movement_created"
	}
	_ = r.AddAudit(ctx, req.ProjectID, actorID, actorName, event, "stock_movement", fmt.Sprint(movement["id"]), req.Reason, Record{"quantity": req.Quantity, "material_id": req.MaterialID})
	return movement, nil
}

func (r *Repository) ProjectMaterialBalances(ctx context.Context, projectID string) ([]Record, error) {
	return r.Query(ctx, `
		SELECT smb.*, m.code, m.name AS material_name, m.unit, m.category
		FROM site_material_balances smb
		JOIN materials m ON m.id=smb.material_id
		WHERE smb.project_id=$1 AND m.active
		ORDER BY m.category, m.name
	`, projectID)
}

func (r *Repository) AcceptanceAction(ctx context.Context, id, result, actorID, actorName string) (Record, error) {
	status := "completed"
	event := "acceptance_passed"
	if result == "fail" {
		event = "acceptance_failed"
	}
	acc, err := r.QueryOne(ctx, `UPDATE acceptance_records SET status=$2, result=$3, approved_by=NULLIF($4,'')::uuid, updated_at=now() WHERE id=$1 RETURNING *`, id, status, result, actorID)
	if err != nil {
		return nil, err
	}
	projectID := fmt.Sprint(acc["project_id"])
	if result == "fail" {
		issue, _ := r.Insert(ctx, "issues", issueColumns(), Record{
			"project_id":          projectID,
			"node_id":             acc["node_id"],
			"title":               fmt.Sprintf("Punch list từ nghiệm thu: %s", acc["title"]),
			"description":         acc["notes"],
			"severity":            "high",
			"status":              "open",
			"reported_by":         actorID,
			"requires_rework":     true,
			"requires_replanning": false,
		})
		_ = r.AddAudit(ctx, projectID, actorID, actorName, "rework_requested", "issue", fmt.Sprint(issue["id"]), "Nghiệm thu fail tạo punch list/rework", Record{"acceptance_id": id})
	}
	_ = r.AddAudit(ctx, projectID, actorID, actorName, event, "acceptance", id, fmt.Sprintf("Nghiệm thu %s: %s", result, acc["title"]), Record{})
	return acc, nil
}

func (r *Repository) CreateReworkFromAcceptance(ctx context.Context, acceptanceID, actorID, actorName string) (Record, error) {
	acc, err := r.GetByID(ctx, "acceptance_records", acceptanceID)
	if err != nil {
		return nil, err
	}
	rework, err := r.Insert(ctx, "rework_requests", reworkColumns(), Record{
		"project_id":    acc["project_id"],
		"acceptance_id": acceptanceID,
		"node_id":       acc["node_id"],
		"title":         fmt.Sprintf("Rework sau nghiệm thu: %s", acc["title"]),
		"description":   acc["notes"],
		"status":        "approved",
		"created_by":    actorID,
		"approved_by":   actorID,
	})
	if err != nil {
		return nil, err
	}
	_ = r.AddAudit(ctx, fmt.Sprint(acc["project_id"]), actorID, actorName, "rework_requested", "rework_request", fmt.Sprint(rework["id"]), "Tạo rework request từ nghiệm thu fail", Record{"acceptance_id": acceptanceID})
	return rework, nil
}

func (r *Repository) HandoverCheck(ctx context.Context, id string) (Record, error) {
	handover, err := r.GetByID(ctx, "handover_records", id)
	if err != nil {
		return nil, err
	}
	projectID := fmt.Sprint(handover["project_id"])
	blockers := []string{}
	finalPass, _ := r.Query(ctx, `
		SELECT COUNT(*) AS total FROM acceptance_records
		WHERE project_id=$1 AND type='final' AND result IN ('pass','conditional_pass')
	`, projectID)
	if len(finalPass) == 0 || number(finalPass[0]["total"]) == 0 {
		blockers = append(blockers, "Chưa có nghiệm thu toàn phần đạt")
	}
	openIssues, _ := r.Query(ctx, `SELECT COUNT(*) AS total FROM issues WHERE project_id=$1 AND status IN ('open','in_progress')`, projectID)
	if len(openIssues) > 0 && number(openIssues[0]["total"]) > 0 {
		blockers = append(blockers, "Còn punch list/issue chưa đóng")
	}
	pendingSurplus, _ := r.Query(ctx, `SELECT COUNT(*) AS total FROM site_material_balances WHERE project_id=$1 AND surplus_quantity > 0 AND surplus_status <> 'handled'`, projectID)
	if len(pendingSurplus) > 0 && number(pendingSurplus[0]["total"]) > 0 {
		blockers = append(blockers, "Còn vật tư dư chưa xử lý")
	}
	if handover["documents_ready"] != true {
		blockers = append(blockers, "Chứng từ bàn giao chưa đủ")
	}
	if handover["warehouse_closed"] != true {
		blockers = append(blockers, "Kho công trình chưa closure")
	}
	if handover["payables_recorded"] != true {
		blockers = append(blockers, "Công nợ đầu vào chưa ghi nhận")
	}
	if handover["receivables_recorded"] != true {
		blockers = append(blockers, "Công nợ đầu ra chưa ghi nhận")
	}
	handover["blockers"] = blockers
	handover["ready"] = len(blockers) == 0
	return handover, nil
}

func (r *Repository) CompleteHandover(ctx context.Context, id, actorID, actorName string) (Record, error) {
	check, err := r.HandoverCheck(ctx, id)
	if err != nil {
		return nil, err
	}
	blockers, _ := check["blockers"].([]string)
	if len(blockers) > 0 {
		_, _ = r.QueryOne(ctx, `UPDATE handover_records SET status='blocked', updated_at=now() WHERE id=$1 RETURNING *`, id)
		return check, nil
	}
	done, err := r.QueryOne(ctx, `UPDATE handover_records SET status='handed_over', handover_date=COALESCE(handover_date, CURRENT_DATE), approved_by=NULLIF($2,'')::uuid, updated_at=now() WHERE id=$1 RETURNING *`, id, actorID)
	if err != nil {
		return nil, err
	}
	_ = r.AddAudit(ctx, fmt.Sprint(done["project_id"]), actorID, actorName, "handover_completed", "handover", id, "Hoàn tất bàn giao công trình", Record{})
	return done, nil
}

func (r *Repository) DashboardReport(ctx context.Context) (Record, error) {
	data := Record{}
	scalars := map[string]string{
		"total_projects":       `SELECT COUNT(*) FROM projects WHERE status <> 'cancelled'`,
		"active_projects":      `SELECT COUNT(*) FROM projects WHERE status IN ('planning','in_progress','acceptance','handover')`,
		"blocked_projects":     `SELECT COUNT(*) FROM projects WHERE status='blocked'`,
		"pending_approvals":    `SELECT COUNT(*) FROM approval_requests ar LEFT JOIN projects p ON p.id=ar.project_id WHERE ar.status='pending' AND (p.id IS NULL OR p.status <> 'cancelled')`,
		"receivable_remaining": `SELECT COALESCE(SUM(remaining_amount),0) FROM receivables WHERE status <> 'cancelled'`,
		"payable_remaining":    `SELECT COALESCE(SUM(remaining_amount),0) FROM payables WHERE status <> 'cancelled'`,
		"missing_materials":    `SELECT COUNT(*) FROM site_material_balances smb JOIN materials m ON m.id=smb.material_id WHERE smb.issued_quantity < smb.planned_quantity AND m.active`,
		"failed_acceptances":   `SELECT COUNT(*) FROM acceptance_records ar JOIN projects p ON p.id=ar.project_id WHERE ar.result='fail' AND p.status <> 'cancelled'`,
		"pending_reworks":      `SELECT COUNT(*) FROM rework_requests rw JOIN projects p ON p.id=rw.project_id WHERE rw.status IN ('draft','approved','in_progress') AND p.status <> 'cancelled'`,
		"cashflow_month":       `SELECT COALESCE(SUM(CASE WHEN payment_type='client_receipt' THEN amount ELSE -amount END),0) FROM payments WHERE status <> 'cancelled' AND date_trunc('month', created_at)=date_trunc('month', now())`,
	}
	for key, query := range scalars {
		var value any
		if err := r.db.QueryRowContext(ctx, query).Scan(&value); err != nil {
			return nil, err
		}
		data[key] = normalizeValue(value)
	}
	charts := Record{}
	statusRows, _ := r.Query(ctx, `SELECT status, COUNT(*) AS total FROM projects WHERE status <> 'cancelled' GROUP BY status ORDER BY status`)
	debtRows, _ := r.Query(ctx, `
		SELECT 'Phải thu' AS type, COALESCE(SUM(remaining_amount),0) AS amount FROM receivables
		WHERE status <> 'cancelled'
		UNION ALL
		SELECT 'Phải trả' AS type, COALESCE(SUM(remaining_amount),0) AS amount FROM payables
		WHERE status <> 'cancelled'
	`)
	costRows, _ := r.Query(ctx, `
		SELECT p.name, COALESCE(b.used_amount,0) AS used_amount, COALESCE(b.remaining_amount,0) AS remaining_amount
		FROM projects p
		LEFT JOIN accounting_budgets b ON b.project_id=p.id
		WHERE p.status <> 'cancelled'
		ORDER BY p.created_at
	`)
	approvalRows, _ := r.Query(ctx, `SELECT target_type, COUNT(*) AS total FROM approval_requests GROUP BY target_type ORDER BY target_type`)
	charts["project_status"] = statusRows
	charts["debt"] = debtRows
	charts["cost_by_project"] = costRows
	charts["approvals_by_type"] = approvalRows
	data["charts"] = charts
	return data, nil
}

func (r *Repository) Report(ctx context.Context, kind string) ([]Record, error) {
	switch kind {
	case "projects":
		return r.Query(ctx, `
			SELECT p.name, p.status, p.progress_percent, COUNT(i.id) FILTER (WHERE i.status IN ('open','in_progress')) AS open_issues,
				COUNT(ar.id) FILTER (WHERE ar.status='pending') AS pending_approvals
			FROM projects p
			LEFT JOIN issues i ON i.project_id=p.id
			LEFT JOIN approval_requests ar ON ar.project_id=p.id
			WHERE p.status <> 'cancelled'
			GROUP BY p.id
			ORDER BY p.created_at DESC
		`)
	case "warehouse":
		return r.Query(ctx, `
			SELECT m.name, m.category,
				COALESCE(SUM(CASE WHEN sm.movement_type='receive' THEN sm.quantity WHEN sm.movement_type IN ('issue','loss') THEN -sm.quantity ELSE 0 END),0) AS stock_quantity,
				COALESCE(SUM(smb.surplus_quantity),0) AS surplus_quantity
			FROM materials m
			LEFT JOIN stock_movements sm ON sm.material_id=m.id
			LEFT JOIN site_material_balances smb ON smb.material_id=m.id
			WHERE m.active
			GROUP BY m.id
			ORDER BY m.category, m.name
		`)
	case "procurement":
		return r.Query(ctx, `SELECT status, budget_status, COUNT(*) AS total, COALESCE(SUM(total_estimated_amount),0) AS amount FROM procurement_requests WHERE status <> 'cancelled' GROUP BY status, budget_status ORDER BY status`)
	case "accounting":
		return r.Query(ctx, `
			SELECT p.name,
				COALESCE(SUM(DISTINCT pay.remaining_amount),0) AS payable_remaining,
				COALESCE(SUM(DISTINCT rec.remaining_amount),0) AS receivable_remaining,
				COALESCE(MAX(b.used_amount),0) AS used_budget
			FROM projects p
			LEFT JOIN payables pay ON pay.project_id=p.id AND pay.status <> 'cancelled'
			LEFT JOIN receivables rec ON rec.project_id=p.id AND rec.status <> 'cancelled'
			LEFT JOIN accounting_budgets b ON b.project_id=p.id AND b.status <> 'cancelled'
			WHERE p.status <> 'cancelled'
			GROUP BY p.id
			ORDER BY p.name
		`)
	case "staff":
		return r.Query(ctx, `
			SELECT s.full_name, s.department, s.position, COUNT(pm.project_id) AS project_count
			FROM staff s
			LEFT JOIN project_members pm ON pm.staff_id=s.id AND pm.active
			WHERE s.status <> 'deleted'
			GROUP BY s.id
			ORDER BY s.full_name
		`)
	default:
		return nil, errors.New("unknown report")
	}
}

func queryTx(ctx context.Context, tx *sql.Tx, query string, args ...any) ([]Record, error) {
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRows(rows)
}

func number(value any) float64 {
	switch v := value.(type) {
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case float64:
		return v
	case float32:
		return float64(v)
	case string:
		out, _ := strconv.ParseFloat(v, 64)
		if math.IsNaN(out) {
			return 0
		}
		return out
	default:
		return 0
	}
}

func stringsJoin(values []string) string {
	return strings.Join(values, "; ")
}

func timeNowUnix() int64 {
	return time.Now().Unix()
}
