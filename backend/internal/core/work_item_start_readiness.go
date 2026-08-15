package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	workItemStartReady             = "ready"
	workItemStartRequirementsUnmet = "requirements_unmet"
	workItemStartRouteUnresolved   = "route_unresolved"
	workItemStartNotAllowed        = "not_allowed"
)

func workItemHasStarted(item Record) bool {
	value := strings.TrimSpace(fmt.Sprint(item["started_at"]))
	return value != "" && value != "<nil>"
}

func (s *Service) workItemStartReadiness(
	ctx context.Context,
	item Record,
	actorID string,
	profile workItemAccessProfile,
	canActInScope bool,
) (Record, error) {
	readiness := Record{
		"state":          workItemStartNotAllowed,
		"can_start":      false,
		"can_bypass":     false,
		"unmet_count":    0,
		"requirements":   []Record{},
		"snapshot_token": "",
	}
	if _, err := workItemTransitionForAction("start", item); err != nil || !canActInScope {
		return readiness, nil
	}
	workRequirement := recordFromAny(item["evidence_requirement_json"])
	requiredPermission := strings.TrimSpace(fmt.Sprint(workRequirement["required_permission"]))
	if requiredPermission != "" && requiredPermission != "<nil>" {
		allowed, err := s.repo.UserHasAnyProjectPermission(ctx, actorID, cleanID(item["project_id"]), []string{requiredPermission})
		if err != nil {
			return nil, err
		}
		if !allowed {
			return readiness, nil
		}
	}

	if !boolValue(item["is_future"]) {
		readiness["state"] = workItemStartReady
		readiness["can_start"] = true
		return readiness, nil
	}
	if int(number(item["pipeline_engine_version"])) < 2 || cleanID(item["pipeline_run_id"]) == "" {
		return readiness, nil
	}

	runID := cleanID(item["pipeline_run_id"])
	currentNodeID := cleanID(item["pipeline_current_node_id"])
	targetNodeID := cleanID(item["run_node_id"])
	if currentNodeID == "" || targetNodeID == "" {
		return readiness, nil
	}

	nodes, err := s.repo.Query(ctx, `
		SELECT id, run_id, project_id, name, type, order_index, status, block_reason,
			approval_rule, approval_policy_json
		FROM pipeline_run_nodes
		WHERE run_id=$1
		ORDER BY order_index, id
	`, runID)
	if err != nil {
		return nil, err
	}
	nodeByID := map[string]Record{}
	for _, node := range nodes {
		nodeByID[cleanID(node["id"])] = node
	}
	targetNode := nodeByID[targetNodeID]
	if len(targetNode) == 0 || statusIn(fmt.Sprint(targetNode["status"]), "skipped", "completed", "cancelled") {
		return readiness, nil
	}
	if currentNodeID == targetNodeID {
		requirements := []Record{{
			"id":               "planned_start:" + cleanID(item["id"]),
			"type":             "planned_start",
			"node_id":          targetNodeID,
			"node_label":       targetNode["name"],
			"label":            "Đến thời điểm bắt đầu theo kế hoạch",
			"status":           "waiting",
			"satisfied":        false,
			"can_open":         false,
			"planned_start_at": item["planned_start_at"],
		}}
		automationRequirements, automationErr := s.repo.workItemStartAutomationRequirements(ctx, []string{targetNodeID}, nodeByID)
		if automationErr != nil {
			return nil, automationErr
		}
		requirements = append(requirements, automationRequirements...)
		requirements, hardBlocked := s.appendTargetStartBlockers(ctx, requirements, targetNode)
		hardBlocked = hardBlocked || len(automationRequirements) > 0
		readiness["state"] = workItemStartRequirementsUnmet
		readiness["can_start"] = !hardBlocked
		readiness["can_bypass"] = !hardBlocked
		readiness["unmet_count"] = len(requirements)
		readiness["requirements"] = requirements
		readiness["snapshot_token"] = workItemStartSnapshotToken(item, currentNodeID, requirements)
		readiness["current_node_id"] = currentNodeID
		readiness["current_node_name"] = targetNode["name"]
		readiness["target_node_id"] = targetNodeID
		readiness["target_node_name"] = targetNode["name"]
		return readiness, nil
	}

	edges, err := s.repo.Query(ctx, `
		SELECT id, from_run_node_id, to_run_node_id, condition_type, priority
		FROM pipeline_run_edges
		WHERE run_id=$1
		ORDER BY priority, created_at, id
	`, runID)
	if err != nil {
		return nil, err
	}
	pathNodeIDs, routeResolved, routeReason := deterministicStartPath(nodes, edges, currentNodeID, targetNodeID)
	if !routeResolved {
		requirements := []Record{{
			"id":          "route:" + currentNodeID + ":" + targetNodeID,
			"type":        "route",
			"node_id":     targetNodeID,
			"node_label":  targetNode["name"],
			"label":       "Chưa xác định được nhánh workflow dẫn đến công việc này",
			"status":      "unresolved",
			"satisfied":   false,
			"can_open":    false,
			"description": routeReason,
		}}
		readiness["state"] = workItemStartRouteUnresolved
		readiness["unmet_count"] = 1
		readiness["requirements"] = requirements
		readiness["snapshot_token"] = workItemStartSnapshotToken(item, currentNodeID, requirements)
		return readiness, nil
	}

	requirements := make([]Record, 0, len(pathNodeIDs)+2)
	currentNode := nodeByID[currentNodeID]
	requirements = append(requirements, Record{
		"id":         "transition:" + currentNodeID + ":" + targetNodeID,
		"type":       "transition",
		"node_id":    currentNodeID,
		"node_label": currentNode["name"],
		"label":      fmt.Sprintf("Workflow chuyển từ %s đến %s", displayStartNodeName(currentNode), displayStartNodeName(targetNode)),
		"status":     "waiting",
		"satisfied":  false,
		"can_open":   false,
	})

	workItemRequirements, err := s.repo.workItemStartPathRequirements(ctx, pathNodeIDs, profile)
	if err != nil {
		return nil, err
	}
	requirements = append(requirements, workItemRequirements...)

	approvalRequirements, err := s.repo.workItemStartApprovalRequirements(ctx, pathNodeIDs, nodeByID)
	if err != nil {
		return nil, err
	}
	requirements = append(requirements, approvalRequirements...)

	automationNodeIDs := append(append([]string{}, pathNodeIDs...), targetNodeID)
	automationRequirements, err := s.repo.workItemStartAutomationRequirements(ctx, automationNodeIDs, nodeByID)
	if err != nil {
		return nil, err
	}
	requirements = append(requirements, automationRequirements...)

	if myWorkPlannedStartInFuture(item["planned_start_at"], time.Now()) {
		requirements = append(requirements, Record{
			"id":               "planned_start:" + cleanID(item["id"]),
			"type":             "planned_start",
			"node_id":          targetNodeID,
			"node_label":       targetNode["name"],
			"label":            "Đến thời điểm bắt đầu theo kế hoạch",
			"status":           "waiting",
			"satisfied":        false,
			"can_open":         false,
			"planned_start_at": item["planned_start_at"],
		})
	}

	requirements, hardBlocked := s.appendTargetStartBlockers(ctx, requirements, targetNode)
	for _, requirement := range automationRequirements {
		if !boolValue(requirement["satisfied"]) {
			hardBlocked = true
		}
	}

	unmetCount := 0
	for _, requirement := range requirements {
		if !boolValue(requirement["satisfied"]) {
			unmetCount++
		}
	}
	readiness["state"] = workItemStartRequirementsUnmet
	readiness["can_start"] = !hardBlocked
	readiness["can_bypass"] = !hardBlocked
	readiness["unmet_count"] = unmetCount
	readiness["requirements"] = requirements
	readiness["snapshot_token"] = workItemStartSnapshotToken(item, currentNodeID, requirements)
	readiness["current_node_id"] = currentNodeID
	readiness["current_node_name"] = currentNode["name"]
	readiness["target_node_id"] = targetNodeID
	readiness["target_node_name"] = targetNode["name"]
	return readiness, nil
}

func (s *Service) appendTargetStartBlockers(ctx context.Context, requirements []Record, targetNode Record) ([]Record, bool) {
	targetNodeID := cleanID(targetNode["id"])
	hardBlocked := false
	for _, blocker := range s.repo.nodeBlockers(ctx, targetNode) {
		hardBlocked = true
		requirements = append(requirements, Record{
			"id":         fmt.Sprintf("target_blocker:%s:%d", targetNodeID, len(requirements)),
			"type":       "blocker",
			"node_id":    targetNodeID,
			"node_label": targetNode["name"],
			"label":      blocker,
			"status":     "blocked",
			"satisfied":  false,
			"can_open":   false,
		})
	}
	if fmt.Sprint(targetNode["status"]) == "blocked" {
		hardBlocked = true
		blockReason := strings.TrimSpace(fmt.Sprint(targetNode["block_reason"]))
		if blockReason == "" || blockReason == "<nil>" {
			blockReason = "Node đích đang bị chặn"
		}
		requirements = append(requirements, Record{
			"id":         "target_node_blocked:" + targetNodeID,
			"type":       "blocker",
			"node_id":    targetNodeID,
			"node_label": targetNode["name"],
			"label":      blockReason,
			"status":     "blocked",
			"satisfied":  false,
			"can_open":   false,
		})
	}
	return requirements, hardBlocked
}

func deterministicStartPath(nodes, edges []Record, currentNodeID, targetNodeID string) ([]string, bool, string) {
	nodeByID := map[string]Record{}
	orderedNodes := make([]Record, 0, len(nodes))
	for _, node := range nodes {
		id := cleanID(node["id"])
		if id == "" || fmt.Sprint(node["status"]) == "skipped" {
			continue
		}
		nodeByID[id] = node
		orderedNodes = append(orderedNodes, node)
	}
	sort.SliceStable(orderedNodes, func(i, j int) bool {
		left := number(orderedNodes[i]["order_index"])
		right := number(orderedNodes[j]["order_index"])
		if left == right {
			return cleanID(orderedNodes[i]["id"]) < cleanID(orderedNodes[j]["id"])
		}
		return left < right
	})
	if len(nodeByID[currentNodeID]) == 0 || len(nodeByID[targetNodeID]) == 0 {
		return nil, false, "Node hiện tại hoặc node đích không còn trong runtime workflow"
	}

	outgoing := map[string][]Record{}
	for _, edge := range edges {
		fromID := cleanID(edge["from_run_node_id"])
		toID := cleanID(edge["to_run_node_id"])
		if len(nodeByID[fromID]) == 0 || len(nodeByID[toID]) == 0 {
			continue
		}
		outgoing[fromID] = append(outgoing[fromID], edge)
	}

	path := []string{}
	visited := map[string]bool{}
	cursor := currentNodeID
	for len(visited) <= len(nodeByID) {
		if cursor == targetNodeID {
			return path, true, ""
		}
		if visited[cursor] {
			return path, false, "Workflow có vòng lặp trước node đích"
		}
		visited[cursor] = true
		path = append(path, cursor)

		nextID := ""
		candidates := outgoing[cursor]
		if len(candidates) > 0 {
			if len(candidates) != 1 || edgeConditionType(candidates[0]) != "always" {
				return path, false, "Nhánh tiếp theo phụ thuộc điều kiện chưa được chốt"
			}
			nextID = cleanID(candidates[0]["to_run_node_id"])
		} else {
			currentOrder := number(nodeByID[cursor]["order_index"])
			for _, node := range orderedNodes {
				if number(node["order_index"]) > currentOrder {
					nextID = cleanID(node["id"])
					break
				}
			}
		}
		if nextID == "" {
			return path, false, "Node đích không nằm trên tuyến workflow có thể xác định"
		}
		cursor = nextID
	}
	return path, false, "Không thể xác định tuyến workflow đến node đích"
}

func (r *Repository) workItemStartPathRequirements(ctx context.Context, nodeIDs []string, profile workItemAccessProfile) ([]Record, error) {
	if len(nodeIDs) == 0 {
		return []Record{}, nil
	}
	rawNodeIDs, _ := json.Marshal(nodeIDs)
	args := append([]any{string(rawNodeIDs)}, workItemScopeArgs(profile, profile.IsManager, true)...)
	rows, err := r.Query(ctx, fmt.Sprintf(`
		SELECT wi.id AS work_item_id, wi.title, wi.status,
			rn.id AS node_id, rn.name AS node_label, rn.order_index AS node_order,
			rs.id AS step_id, rs.name AS step_label, rs.order_index AS step_order,
			wfi.status AS workflow_form_status,
			(%s) AS can_open
		FROM work_items wi
		JOIN pipeline_run_nodes rn ON rn.id=wi.run_node_id
		LEFT JOIN pipeline_run_steps rs ON rs.id=wi.run_step_id
		LEFT JOIN workflow_form_instances wfi ON wfi.work_item_id=wi.id AND wfi.status <> 'cancelled'
		WHERE wi.run_node_id IN (
			SELECT value::uuid FROM jsonb_array_elements_text($1::jsonb)
		)
		  AND wi.status <> 'cancelled'
		ORDER BY rn.order_index, rs.order_index NULLS LAST, wi.created_at, wi.id
	`, workItemScopeSQL("wi", 2)), args...)
	if err != nil {
		return nil, err
	}
	requirements := make([]Record, 0, len(rows))
	for _, row := range rows {
		status := fmt.Sprint(row["status"])
		requirement := Record{
			"id":           "work_item:" + cleanID(row["work_item_id"]),
			"type":         "work_item",
			"node_id":      row["node_id"],
			"node_label":   row["node_label"],
			"step_id":      row["step_id"],
			"step_label":   row["step_label"],
			"work_item_id": row["work_item_id"],
			"label":        row["title"],
			"status":       status,
			"satisfied":    statusIn(status, "completed", "done"),
			"can_open":     boolValue(row["can_open"]),
		}
		if formStatus := strings.TrimSpace(fmt.Sprint(row["workflow_form_status"])); formStatus != "" && formStatus != "<nil>" {
			requirement["workflow_form_status"] = formStatus
		}
		requirements = append(requirements, requirement)
	}
	return requirements, nil
}

func (r *Repository) workItemStartApprovalRequirements(ctx context.Context, nodeIDs []string, nodeByID map[string]Record) ([]Record, error) {
	requiredNodeIDs := []string{}
	for _, nodeID := range nodeIDs {
		node := nodeByID[nodeID]
		policy := approvalPolicyFromValue(node["approval_policy_json"])
		legacyRule := strings.TrimSpace(fmt.Sprint(node["approval_rule"]))
		if policy.enabled() || (legacyRule != "" && legacyRule != "<nil>") {
			requiredNodeIDs = append(requiredNodeIDs, nodeID)
		}
	}
	if len(requiredNodeIDs) == 0 {
		return []Record{}, nil
	}
	rawNodeIDs, _ := json.Marshal(requiredNodeIDs)
	approvedRows, err := r.Query(ctx, `
		SELECT DISTINCT target_id::text AS node_id
		FROM approval_requests
		WHERE target_type='pipeline_run_node'
		  AND status='approved'
		  AND target_id IN (SELECT value::uuid FROM jsonb_array_elements_text($1::jsonb))
	`, string(rawNodeIDs))
	if err != nil {
		return nil, err
	}
	approved := map[string]bool{}
	for _, row := range approvedRows {
		approved[cleanID(row["node_id"])] = true
	}
	requirements := make([]Record, 0, len(requiredNodeIDs))
	for _, nodeID := range requiredNodeIDs {
		node := nodeByID[nodeID]
		isApproved := approved[nodeID]
		requirements = append(requirements, Record{
			"id":         "node_approval:" + nodeID,
			"type":       "node_approval",
			"node_id":    nodeID,
			"node_label": node["name"],
			"label":      fmt.Sprintf("Phê duyệt node %s", displayStartNodeName(node)),
			"status":     map[bool]string{true: "approved", false: "pending"}[isApproved],
			"satisfied":  isApproved,
			"can_open":   false,
		})
	}
	return requirements, nil
}

func (r *Repository) workItemStartAutomationRequirements(ctx context.Context, nodeIDs []string, nodeByID map[string]Record) ([]Record, error) {
	if len(nodeIDs) == 0 {
		return []Record{}, nil
	}
	rawNodeIDs, _ := json.Marshal(nodeIDs)
	rows, err := r.Query(ctx, `
		SELECT id, run_node_id, action_key, status,
			COALESCE(NULLIF(blocker_reason,''), NULLIF(error_message,''), 'Tự động hóa bắt buộc chưa thành công') AS reason
		FROM workflow_automation_events
		WHERE run_node_id IN (SELECT value::uuid FROM jsonb_array_elements_text($1::jsonb))
		  AND status IN ('blocked','failed')
		ORDER BY updated_at DESC, id
	`, string(rawNodeIDs))
	if err != nil {
		return nil, err
	}
	requirements := make([]Record, 0, len(rows))
	for _, row := range rows {
		nodeID := cleanID(row["run_node_id"])
		requirements = append(requirements, Record{
			"id":         "automation:" + cleanID(row["id"]),
			"type":       "automation",
			"node_id":    nodeID,
			"node_label": nodeByID[nodeID]["name"],
			"label":      row["reason"],
			"status":     row["status"],
			"satisfied":  false,
			"can_open":   false,
			"action_key": row["action_key"],
		})
	}
	return requirements, nil
}

func displayStartNodeName(node Record) string {
	name := strings.TrimSpace(fmt.Sprint(node["name"]))
	if name == "" || name == "<nil>" {
		return "node workflow"
	}
	return name
}

func workItemStartSnapshotToken(item Record, currentNodeID string, requirements []Record) string {
	payload := Record{
		"work_item_id":     cleanID(item["id"]),
		"work_item_status": fmt.Sprint(item["status"]),
		"current_node_id":  currentNodeID,
		"target_node_id":   cleanID(item["run_node_id"]),
		"planned_start_at": item["planned_start_at"],
		"requirements":     requirements,
	}
	raw, _ := json.Marshal(payload)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
