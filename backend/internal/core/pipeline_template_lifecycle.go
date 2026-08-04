package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

func (s *Service) ensureDraftTemplate(ctx context.Context, resourceType, resourceID string) (Record, error) {
	query := `SELECT * FROM pipeline_templates WHERE id=$1`
	switch resourceType {
	case "node":
		query = `SELECT t.* FROM pipeline_template_nodes n JOIN pipeline_templates t ON t.id=n.template_id WHERE n.id=$1`
	case "step":
		query = `SELECT t.* FROM pipeline_template_steps s JOIN pipeline_template_nodes n ON n.id=s.node_id JOIN pipeline_templates t ON t.id=n.template_id WHERE s.id=$1`
	case "edge":
		query = `SELECT t.* FROM pipeline_template_edges e JOIN pipeline_templates t ON t.id=e.template_id WHERE e.id=$1`
	}
	template, err := s.repo.QueryOne(ctx, query, resourceID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(fmt.Sprint(template["status"])) != "draft" {
		return nil, validationError("Phiên bản quy trình đã phát hành là bất biến; hãy tạo revision nháp mới để chỉnh sửa", Record{
			"template_id": template["id"],
			"status":      template["status"],
		})
	}
	return template, nil
}

func (s *Service) CreateTemplateNode(ctx context.Context, templateID string, body Record) (Record, error) {
	template, err := s.ensureDraftTemplate(ctx, "template", templateID)
	if err != nil {
		return nil, err
	}
	if err := prepareBusinessCode(body, int(number(template["engine_version"])) >= 3); err != nil {
		return nil, err
	}
	if raw, ok := body["approval_policy_json"]; ok {
		normalized, err := normalizeApprovalPolicyValue(raw)
		if err != nil {
			return nil, err
		}
		body["approval_policy_json"] = normalized
	}
	body["template_id"] = templateID
	return s.repo.Insert(ctx, "pipeline_template_nodes", templateNodeColumns(), body)
}

func (s *Service) UpdateTemplateNode(ctx context.Context, nodeID string, body Record) (Record, error) {
	template, err := s.ensureDraftTemplate(ctx, "node", nodeID)
	if err != nil {
		return nil, err
	}
	if err := prepareBusinessCode(body, int(number(template["engine_version"])) >= 3); err != nil {
		return nil, err
	}
	if raw, ok := body["approval_policy_json"]; ok {
		normalized, err := normalizeApprovalPolicyValue(raw)
		if err != nil {
			return nil, err
		}
		body["approval_policy_json"] = normalized
	}
	return s.repo.Update(ctx, "pipeline_template_nodes", templateNodeColumns(), nodeID, body)
}

func prepareBusinessCode(body Record, required bool) error {
	raw, provided := body["business_code"]
	if !provided {
		if required {
			return validationError("Business code là bắt buộc với workflow engine v3")
		}
		return nil
	}
	code := strings.TrimSpace(fmt.Sprint(raw))
	if code == "<nil>" {
		code = ""
	}
	if code == "" {
		if required {
			return validationError("Business code là bắt buộc với workflow engine v3")
		}
		body["business_code"] = nil
		return nil
	}
	if len(code) > 120 || !validBusinessCode(code) {
		return validationError("Business code không hợp lệ; dùng chữ thường, số và dấu chấm/gạch nối/gạch dưới", Record{"business_code": code})
	}
	body["business_code"] = code
	return nil
}

func validBusinessCode(code string) bool {
	if code == "" || code[0] < 'a' || code[0] > 'z' {
		return false
	}
	separator := false
	for _, character := range code {
		if character >= 'a' && character <= 'z' || character >= '0' && character <= '9' {
			separator = false
			continue
		}
		if character == '.' || character == '_' || character == '-' {
			if separator {
				return false
			}
			separator = true
			continue
		}
		return false
	}
	return !separator
}

func (s *Service) DeleteTemplateNode(ctx context.Context, nodeID string) error {
	if _, err := s.ensureDraftTemplate(ctx, "node", nodeID); err != nil {
		return err
	}
	return s.repo.Exec(ctx, `DELETE FROM pipeline_template_nodes WHERE id=$1`, nodeID)
}

func (s *Service) CreateTemplateEdge(ctx context.Context, templateID string, body Record) (Record, error) {
	if _, err := s.ensureDraftTemplate(ctx, "template", templateID); err != nil {
		return nil, err
	}
	body["template_id"] = templateID
	return s.repo.Insert(ctx, "pipeline_template_edges", edgeColumns(), body)
}

func (s *Service) UpdateTemplateEdge(ctx context.Context, edgeID string, body Record) (Record, error) {
	if _, err := s.ensureDraftTemplate(ctx, "edge", edgeID); err != nil {
		return nil, err
	}
	return s.repo.Update(ctx, "pipeline_template_edges", edgeColumns(), edgeID, body)
}

func (s *Service) DeleteTemplateEdge(ctx context.Context, edgeID string) error {
	if _, err := s.ensureDraftTemplate(ctx, "edge", edgeID); err != nil {
		return err
	}
	return s.repo.Exec(ctx, `DELETE FROM pipeline_template_edges WHERE id=$1`, edgeID)
}

func (r *Repository) TemplatePublishIssues(ctx context.Context, templateID string) ([]Record, error) {
	issues, err := r.TemplatePublishFormIssues(ctx, templateID)
	if err != nil {
		return nil, err
	}
	for _, issue := range issues {
		issue["code"] = "FORM_VERSION_NOT_PUBLISHED"
		issue["message"] = "Bước bắt buộc chưa gắn phiên bản biểu mẫu đã phát hành"
	}

	configIssues, err := r.Query(ctx, `
		SELECT e.id AS edge_id, e.from_node_id, e.to_node_id,
			'EDGE_CONDITION_CONFIG_INVALID' AS code,
			'Điều kiện câu trả lời biểu mẫu cần field_key, value và field phải thuộc biểu mẫu của node nguồn' AS message
		FROM pipeline_template_edges e
		WHERE e.template_id=$1
		  AND e.condition_type='workflow_form_answer_equals'
		  AND (
			NOT (e.condition_config ? 'field_key')
			OR NULLIF(BTRIM(e.condition_config->>'field_key'),'') IS NULL
			OR NOT (e.condition_config ? 'value')
			OR NOT EXISTS (
				SELECT 1
				FROM pipeline_template_steps s
				JOIN workflow_form_fields f ON f.form_version_id=s.form_version_id
				WHERE s.node_id=e.from_node_id
				  AND f.field_key=e.condition_config->>'field_key'
			)
		  )
	`, templateID)
	if err != nil {
		return nil, err
	}
	issues = append(issues, configIssues...)

	permissionIssues, err := r.Query(ctx, `
		SELECT n.id AS node_id, NULL::uuid AS step_id, n.required_permission AS permission_key,
			'PERMISSION_NOT_FOUND' AS code, 'Quyền bắt buộc của node không tồn tại' AS message
		FROM pipeline_template_nodes n
		LEFT JOIN permissions p ON p.key=n.required_permission
		WHERE n.template_id=$1 AND NULLIF(n.required_permission,'') IS NOT NULL AND p.id IS NULL
		UNION ALL
		SELECT n.id, s.id, s.required_permission,
			'PERMISSION_NOT_FOUND', 'Quyền bắt buộc của bước không tồn tại'
		FROM pipeline_template_steps s
		JOIN pipeline_template_nodes n ON n.id=s.node_id
		LEFT JOIN permissions p ON p.key=s.required_permission
		WHERE n.template_id=$1 AND NULLIF(s.required_permission,'') IS NOT NULL AND p.id IS NULL
	`, templateID)
	if err != nil {
		return nil, err
	}
	issues = append(issues, permissionIssues...)

	surfaceIssues, err := r.Query(ctx, `
		SELECT n.id AS node_id, s.id AS step_id, s.name AS step_name,
			'WORK_ITEM_SURFACE_MISSING' AS code,
			'Bước engine v2 cần surface_kind ổn định trước khi phát hành' AS message
		FROM pipeline_template_steps s
		JOIN pipeline_template_nodes n ON n.id=s.node_id
		JOIN pipeline_templates t ON t.id=n.template_id
		WHERE t.id=$1 AND t.engine_version >= 2 AND NULLIF(BTRIM(s.surface_kind),'') IS NULL
		ORDER BY n.order_index, s.order_index, s.id
	`, templateID)
	if err != nil {
		return nil, err
	}
	issues = append(issues, surfaceIssues...)

	businessCodeIssues, err := r.Query(ctx, `
		SELECT n.id AS node_id,NULL::uuid AS step_id,'BUSINESS_CODE_REQUIRED' AS code,
			'Node của engine v2 cần business_code ổn định trước khi phát hành' AS message
		FROM pipeline_template_nodes n
		JOIN pipeline_templates t ON t.id=n.template_id
		WHERE t.id=$1 AND t.engine_version >= 3 AND NULLIF(BTRIM(n.business_code),'') IS NULL
		UNION ALL
		SELECT n.id,s.id,'BUSINESS_CODE_REQUIRED',
			'Bước của engine v2 cần business_code ổn định trước khi phát hành'
		FROM pipeline_template_steps s
		JOIN pipeline_template_nodes n ON n.id=s.node_id
		JOIN pipeline_templates t ON t.id=n.template_id
		WHERE t.id=$1 AND t.engine_version >= 3 AND NULLIF(BTRIM(s.business_code),'') IS NULL
		ORDER BY node_id,step_id NULLS FIRST
	`, templateID)
	if err != nil {
		return nil, err
	}
	issues = append(issues, businessCodeIssues...)

	detail, err := r.GetTemplateDetail(ctx, templateID)
	if err != nil {
		return nil, err
	}
	nodes, _ := detail["nodes"].([]Record)
	edges, _ := detail["edges"].([]Record)
	for _, node := range nodes {
		policy, policyErr := normalizeApprovalPolicyValue(node["approval_policy_json"])
		if policyErr != nil {
			issues = append(issues, Record{"code": "APPROVAL_POLICY_INVALID", "message": policyErr.Error(), "node_id": node["id"], "node_name": node["name"]})
			continue
		}
		if int(number(detail["engine_version"])) >= 2 && strings.TrimSpace(fmt.Sprint(node["approval_rule"])) != "" && fmt.Sprint(node["approval_rule"]) != "<nil>" && !approvalPolicyFromValue(policy).enabled() {
			issues = append(issues, Record{"code": "APPROVAL_POLICY_REQUIRED", "message": "Node có yêu cầu phê duyệt cần approval_policy_json có cấu trúc", "node_id": node["id"], "node_name": node["name"]})
		}
	}
	issues = append(issues, validateTemplateGraph(nodes, edges)...)
	return issues, nil
}

func validateTemplateGraph(nodes, edges []Record) []Record {
	issues := []Record{}
	if len(nodes) == 0 {
		return []Record{{"code": "GRAPH_EMPTY", "message": "Mẫu quy trình cần ít nhất một node"}}
	}
	known := map[string]bool{}
	indegree := map[string]int{}
	outdegree := map[string]int{}
	adjacency := map[string][]string{}
	for _, node := range nodes {
		id := cleanID(node["id"])
		known[id] = true
		indegree[id] = 0
		outdegree[id] = 0
	}
	for _, edge := range edges {
		from, to := cleanID(edge["from_node_id"]), cleanID(edge["to_node_id"])
		if !known[from] || !known[to] {
			issues = append(issues, Record{"code": "EDGE_NODE_OUTSIDE_TEMPLATE", "message": "Connector tham chiếu node ngoài template", "edge_id": edge["id"]})
			continue
		}
		adjacency[from] = append(adjacency[from], to)
		outdegree[from]++
		indegree[to]++
	}
	roots := []string{}
	sinks := []string{}
	for id := range known {
		if indegree[id] == 0 {
			roots = append(roots, id)
		}
		if outdegree[id] == 0 {
			sinks = append(sinks, id)
		}
	}
	if len(roots) != 1 {
		issues = append(issues, Record{"code": "GRAPH_ROOT_COUNT", "message": "Mẫu quy trình phải có đúng một node bắt đầu", "root_count": len(roots)})
	}
	if len(sinks) != 1 {
		issues = append(issues, Record{"code": "GRAPH_SINK_COUNT", "message": "Mẫu quy trình phải có đúng một node kết thúc", "sink_count": len(sinks)})
	}
	if len(roots) == 1 {
		visited := map[string]bool{}
		queue := []string{roots[0]}
		for len(queue) > 0 {
			id := queue[0]
			queue = queue[1:]
			if visited[id] {
				continue
			}
			visited[id] = true
			queue = append(queue, adjacency[id]...)
		}
		if len(visited) != len(nodes) {
			issues = append(issues, Record{"code": "GRAPH_UNREACHABLE", "message": "Mẫu quy trình có node không thể đi tới từ node bắt đầu", "unreachable_count": len(nodes) - len(visited)})
		}
	}
	degrees := map[string]int{}
	queue := []string{}
	for id, degree := range indegree {
		degrees[id] = degree
		if degree == 0 {
			queue = append(queue, id)
		}
	}
	processed := 0
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		processed++
		for _, next := range adjacency[id] {
			degrees[next]--
			if degrees[next] == 0 {
				queue = append(queue, next)
			}
		}
	}
	if processed != len(nodes) {
		issues = append(issues, Record{"code": "GRAPH_CYCLE", "message": "Mẫu quy trình có chu trình; rework phải dùng transition được kiểm soát thay vì connector vòng"})
	}
	return issues
}

func templateDefinitionHash(detail Record) (string, error) {
	definition := Record{}
	for key, value := range detail {
		if key == "nodes" {
			definition[key] = stripTemplateNodePositions(value)
			continue
		}
		definition[key] = value
	}
	canonical := stripTemplateVolatileFields(definition)
	raw, err := json.Marshal(canonical)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

func (r *Repository) NodeCompletionIssues(ctx context.Context, node Record) ([]Record, error) {
	run, err := r.QueryOne(ctx, `SELECT id, current_node_id, status, engine_version FROM pipeline_runs WHERE id=$1`, node["run_id"])
	if err != nil {
		return nil, err
	}
	if int(number(run["engine_version"])) < 2 {
		return []Record{}, nil
	}
	issues := []Record{}
	if cleanID(run["current_node_id"]) != cleanID(node["id"]) {
		issues = append(issues, Record{"code": "NODE_NOT_CURRENT", "message": "Chỉ node hiện tại của quy trình mới được hoàn tất"})
	}
	if !statusIn(fmt.Sprint(node["status"]), "in_progress", "waiting_approval", "blocked") {
		issues = append(issues, Record{"code": "NODE_STATUS_INVALID", "message": "Trạng thái node không cho phép hoàn tất", "status": node["status"]})
	}

	stepIssues, err := r.Query(ctx, `
		SELECT rs.id AS run_step_id, rs.name AS step_name, rs.status,
			'RUN_STEP_INCOMPLETE' AS code, 'Bước quy trình chưa hoàn tất' AS message
		FROM pipeline_run_steps rs
		WHERE rs.run_node_id=$1 AND rs.status <> 'completed'
		ORDER BY rs.order_index, rs.id
	`, node["id"])
	if err != nil {
		return nil, err
	}
	issues = append(issues, stepIssues...)

	workItemIssues, err := r.Query(ctx, `
		SELECT wi.id AS work_item_id, wi.title, wi.status,
			'WORK_ITEM_INCOMPLETE' AS code, 'Đầu việc của bước quy trình chưa hoàn tất' AS message
		FROM work_items wi
		WHERE wi.run_node_id=$1 AND wi.status NOT IN ('completed','cancelled')
		ORDER BY wi.created_at, wi.id
	`, node["id"])
	if err != nil {
		return nil, err
	}
	issues = append(issues, workItemIssues...)

	if strings.TrimSpace(fmt.Sprint(node["type"])) == "construction_planning" {
		handoffIssues, err := r.Query(ctx, `
			SELECT 'CONSTRUCTION_HANDOFF_REQUIRED' AS code,
				'Kế hoạch giám sát chưa được kích hoạt và bàn giao sang Thi công' AS message
			WHERE NOT EXISTS (
				SELECT 1
				FROM construction_handoffs h
				WHERE h.pipeline_run_id=$1 AND h.source_run_node_id=$2 AND h.status='activated'
			)
		`, node["run_id"], node["id"])
		if err != nil {
			return nil, err
		}
		issues = append(issues, handoffIssues...)
	}

	formIssues, err := r.Query(ctx, `
		SELECT rs.id AS run_step_id, rs.name AS step_name, rs.form_version_id,
			wfi.id AS workflow_form_instance_id, wfi.status AS workflow_form_status,
			'REQUIRED_FORM_INCOMPLETE' AS code,
			'Biểu mẫu bắt buộc đúng phiên bản chưa được gửi' AS message
		FROM pipeline_run_steps rs
		LEFT JOIN workflow_form_instances wfi
		  ON wfi.run_step_id=rs.id
		 AND wfi.form_version_id=rs.form_version_id
		 AND wfi.status <> 'cancelled'
		WHERE rs.run_node_id=$1
		  AND rs.form_required
		  AND (wfi.id IS NULL OR wfi.status <> 'submitted')
		ORDER BY rs.order_index, rs.id
	`, node["id"])
	if err != nil {
		return nil, err
	}
	issues = append(issues, formIssues...)

	approvalPolicy := approvalPolicyFromValue(node["approval_policy_json"])
	requiresApproval := approvalPolicy.enabled()
	if !requiresApproval && strings.TrimSpace(fmt.Sprint(node["approval_rule"])) != "" && fmt.Sprint(node["approval_rule"]) != "<nil>" {
		requiresApproval = true
	}
	if requiresApproval {
		approvalIssues, err := r.Query(ctx, `
			SELECT 'NODE_APPROVAL_REQUIRED' AS code, 'Node chưa có phê duyệt đạt yêu cầu' AS message
			WHERE NOT EXISTS (
				SELECT 1 FROM approval_requests
				WHERE target_type='pipeline_run_node' AND target_id=$1 AND status='approved'
			)
		`, node["id"])
		if err != nil {
			return nil, err
		}
		issues = append(issues, approvalIssues...)
	}

	automationIssues, err := r.Query(ctx, `
		SELECT id AS automation_event_id, action_key, status, blocker_reason, error_message,
			'AUTOMATION_BLOCKED' AS code,
			COALESCE(NULLIF(blocker_reason,''), NULLIF(error_message,''), 'Tự động hóa bắt buộc chưa thành công') AS message
		FROM workflow_automation_events
		WHERE run_node_id=$1 AND status IN ('blocked','failed')
		ORDER BY updated_at DESC, id
	`, node["id"])
	if err != nil {
		return nil, err
	}
	issues = append(issues, automationIssues...)
	return issues, nil
}

func stripTemplateVolatileFields(value any) any {
	volatile := map[string]bool{
		"status": true, "definition_hash": true, "published_at": true, "published_by": true,
		"created_at": true, "updated_at": true, "node_count": true,
	}
	switch typed := value.(type) {
	case Record:
		out := Record{}
		for key, item := range typed {
			if volatile[key] {
				continue
			}
			if key == "condition_config" {
				out[key] = stripTemplateDiagramMetadata(item)
				continue
			}
			out[key] = stripTemplateVolatileFields(item)
		}
		return out
	case map[string]any:
		out := map[string]any{}
		for key, item := range typed {
			if volatile[key] {
				continue
			}
			if key == "condition_config" {
				out[key] = stripTemplateDiagramMetadata(item)
				continue
			}
			out[key] = stripTemplateVolatileFields(item)
		}
		return out
	case []Record:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, stripTemplateVolatileFields(item))
		}
		return out
	case []any:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, stripTemplateVolatileFields(item))
		}
		return out
	default:
		return value
	}
}

func stripTemplateNodePositions(value any) any {
	switch typed := value.(type) {
	case []Record:
		out := make([]Record, 0, len(typed))
		for _, node := range typed {
			out = append(out, stripTemplateNodePosition(node))
		}
		return out
	case []any:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			switch node := item.(type) {
			case Record:
				out = append(out, stripTemplateNodePosition(node))
			case map[string]any:
				out = append(out, stripTemplateNodePosition(node))
			default:
				out = append(out, item)
			}
		}
		return out
	default:
		return value
	}
}

func stripTemplateNodePosition(node map[string]any) map[string]any {
	out := map[string]any{}
	for key, value := range node {
		if key != "position_x" && key != "position_y" {
			out[key] = value
		}
	}
	return out
}

func stripTemplateDiagramMetadata(value any) any {
	switch typed := value.(type) {
	case Record:
		out := Record{}
		for key, item := range typed {
			if key != "diagram" {
				out[key] = stripTemplateVolatileFields(item)
			}
		}
		return out
	case map[string]any:
		out := map[string]any{}
		for key, item := range typed {
			if key != "diagram" {
				out[key] = stripTemplateVolatileFields(item)
			}
		}
		return out
	default:
		return stripTemplateVolatileFields(value)
	}
}
