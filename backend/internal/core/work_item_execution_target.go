package core

import (
	"fmt"
	"net/url"
	"strings"
)

const (
	executionWorkspacePreparation    = "preparation"
	executionWorkspacePlanning       = "planning"
	executionWorkspaceBOQ            = "boq"
	executionWorkspaceContracts      = "contracts"
	executionWorkspaceProcurement    = "procurement"
	executionWorkspaceWarehouse      = "warehouse"
	executionWorkspaceOperations     = "operations"
	executionWorkspaceAcceptance     = "acceptance"
	executionWorkspaceHandover       = "handover"
	executionWorkspaceProjectFinance = "project_finance"
	executionWorkspaceWorkItem       = "work_item"
)

type executionWorkspacePresentation struct {
	Path        string
	ActionLabel string
}

var executionWorkspacePresentations = map[string]executionWorkspacePresentation{
	executionWorkspacePreparation:    {Path: "preparation", ActionLabel: "Mở hồ sơ chuẩn bị"},
	executionWorkspacePlanning:       {Path: "planning", ActionLabel: "Mở kế hoạch"},
	executionWorkspaceBOQ:            {Path: "boq", ActionLabel: "Cập nhật BOQ"},
	executionWorkspaceContracts:      {Path: "contracts", ActionLabel: "Mở hợp đồng"},
	executionWorkspaceProcurement:    {Path: "procurement", ActionLabel: "Mở mua sắm"},
	executionWorkspaceWarehouse:      {Path: "warehouse", ActionLabel: "Mở kho vật tư"},
	executionWorkspaceOperations:     {Path: "operations", ActionLabel: "Mở thi công"},
	executionWorkspaceAcceptance:     {Path: "acceptance", ActionLabel: "Mở nghiệm thu"},
	executionWorkspaceHandover:       {Path: "handover", ActionLabel: "Mở bàn giao"},
	executionWorkspaceProjectFinance: {Path: "accounting", ActionLabel: "Mở bàn giao kế toán"},
	executionWorkspaceWorkItem:       {Path: "", ActionLabel: "Mở hồ sơ công việc"},
}

var executionWorkspaceByActionPrefix = []struct {
	Prefix    string
	Workspace string
}{
	{Prefix: "source.boq", Workspace: executionWorkspaceBOQ},
	{Prefix: "boq.", Workspace: executionWorkspaceBOQ},
	{Prefix: "source.contract", Workspace: executionWorkspaceContracts},
	{Prefix: "source.input_contract", Workspace: executionWorkspaceContracts},
	{Prefix: "contract.", Workspace: executionWorkspaceContracts},
	{Prefix: "source.supplier", Workspace: executionWorkspaceProcurement},
	{Prefix: "source.procurement", Workspace: executionWorkspaceContracts},
	{Prefix: "procurement.", Workspace: executionWorkspaceProcurement},
	{Prefix: "warehouse.", Workspace: executionWorkspaceWarehouse},
	{Prefix: "stock.", Workspace: executionWorkspaceWarehouse},
	{Prefix: "planning.", Workspace: executionWorkspacePlanning},
	{Prefix: "construction.", Workspace: executionWorkspacePlanning},
	{Prefix: "subcontractor.", Workspace: executionWorkspaceAcceptance},
	{Prefix: "source.subcontractor", Workspace: executionWorkspaceAcceptance},
	{Prefix: "owner.", Workspace: executionWorkspaceAcceptance},
	{Prefix: "acceptance.", Workspace: executionWorkspaceAcceptance},
	{Prefix: "source.final_acceptance", Workspace: executionWorkspaceAcceptance},
	{Prefix: "source.payable", Workspace: executionWorkspaceProjectFinance},
	{Prefix: "source.input_invoice", Workspace: executionWorkspaceProjectFinance},
	{Prefix: "payable.", Workspace: executionWorkspaceProjectFinance},
	{Prefix: "source.receivable", Workspace: executionWorkspaceProjectFinance},
	{Prefix: "receivable.", Workspace: executionWorkspaceProjectFinance},
	{Prefix: "finance.", Workspace: executionWorkspaceProjectFinance},
	{Prefix: "source.project_finance", Workspace: executionWorkspaceProjectFinance},
	{Prefix: "source.accounting_handoff", Workspace: executionWorkspaceProjectFinance},
	{Prefix: "source.handover", Workspace: executionWorkspaceHandover},
	{Prefix: "handover.", Workspace: executionWorkspaceHandover},
}

func workItemExecutionTarget(item Record, businessSource Record) Record {
	projectID := cleanID(item["project_id"])
	workItemID := cleanID(item["id"])
	actionKey := cleanID(item["domain_action_key"])
	hasForm := cleanID(item["workflow_form_instance_id"]) != ""
	workspace := executionWorkspaceForWorkItem(item, actionKey)
	kind := executionTargetKind(item, workspace, actionKey, hasForm)
	completionMode := executionCompletionMode(kind)
	presentation := executionWorkspacePresentations[workspace]
	actionLabel := presentation.ActionLabel
	if workspace == executionWorkspaceProjectFinance && !strings.HasPrefix(actionKey, "source.accounting_handoff") {
		actionLabel = "Mở tài chính công trình"
	}
	switch kind {
	case "approval":
		actionLabel = "Mở phê duyệt"
	case "evidence":
		actionLabel = "Nộp minh chứng"
	}
	route := executionWorkspaceRoute(projectID, workItemID, presentation.Path)

	if sourceRoute := cleanID(businessSource["route"]); sourceRoute != "" && workspace != executionWorkspacePreparation && executionSourceRouteAllowed(sourceRoute, projectID) {
		route = executionRouteWithTask(sourceRoute, workItemID)
	}
	fallbackRoute := executionWorkspaceRoute(projectID, workItemID, "")
	if route == "" {
		route = fallbackRoute
	}

	target := Record{
		"kind":            kind,
		"workspace":       workspace,
		"route":           route,
		"fallback_route":  fallbackRoute,
		"action_label":    actionLabel,
		"completion_mode": completionMode,
		"source_status":   cleanID(businessSource["status"]),
		"source_ready":    boolValue(businessSource["ready"]),
		"source_message":  cleanID(businessSource["message"]),
		"target_type":     cleanID(businessSource["target_type"]),
		"target_id":       cleanID(businessSource["target_id"]),
	}
	if actionKey != "" {
		target["action_key"] = actionKey
	}
	return target
}

func executionWorkspaceForWorkItem(item Record, actionKey string) string {
	if actionKey == "source.design_documents_ready" {
		return executionWorkspacePreparation
	}
	if actionKey == "construction.activate_handoff" {
		return executionWorkspacePlanning
	}
	for _, candidate := range executionWorkspaceByActionPrefix {
		if strings.HasPrefix(actionKey, candidate.Prefix) {
			return candidate.Workspace
		}
	}

	nodeCode := strings.ToLower(cleanID(item["run_node_business_code"]))
	nodeType := strings.ToLower(cleanID(item["run_node_type"]))
	surface := strings.ToLower(cleanID(item["surface_kind"]))
	workType := strings.ToLower(cleanID(item["work_type"]))

	switch {
	case strings.Contains(nodeCode, "construction_planning"):
		return executionWorkspacePlanning
	case surface == "material_snapshot", surface == "surplus_handling", workType == "material_check":
		return executionWorkspaceWarehouse
	case strings.Contains(nodeCode, "construction_execution"), nodeType == "construction", surface == "quality":
		return executionWorkspaceOperations
	case strings.Contains(nodeCode, "incoming_acceptance"), strings.Contains(nodeCode, "outgoing_acceptance"):
		return executionWorkspaceAcceptance
	case strings.Contains(nodeCode, "closure"), strings.Contains(nodeCode, "handover"):
		return executionWorkspaceHandover
	case strings.Contains(nodeCode, "boq"):
		return executionWorkspaceBOQ
	case strings.Contains(nodeCode, "contract"):
		return executionWorkspaceContracts
	case surface == "contract":
		return executionWorkspaceContracts
	case surface == "incoming_acceptance", surface == "owner_client_reconciliation", surface == "subcontractor_reconciliation":
		return executionWorkspaceAcceptance
	case surface == "finance":
		return executionWorkspaceProjectFinance
	case surface == "handover":
		return executionWorkspaceHandover
	case workType == "handover_check":
		return executionWorkspaceHandover
	case surface == "approval", workType == "approval":
		return executionWorkspaceWorkItem
	case surface == "evidence", workType == "photo_upload", workType == "evidence_upload", workType == "document_upload":
		return executionWorkspacePreparation
	case cleanID(item["workflow_form_instance_id"]) != "":
		return executionWorkspacePreparation
	default:
		return executionWorkspaceWorkItem
	}
}

func executionTargetKind(item Record, workspace, actionKey string, hasForm bool) string {
	workType := strings.ToLower(cleanID(item["work_type"]))
	surface := strings.ToLower(cleanID(item["surface_kind"]))
	if surface == "approval" || workType == "approval" {
		return "approval"
	}
	if actionKey != "" && hasForm {
		return "mixed"
	}
	if actionKey != "" {
		return "domain"
	}
	if workType == "photo_upload" || workType == "evidence_upload" || surface == "evidence" {
		return "evidence"
	}
	if hasForm {
		return "capture"
	}
	if workspace != executionWorkspaceWorkItem {
		return "domain"
	}
	return "work_item"
}

func executionCompletionMode(kind string) string {
	switch kind {
	case "capture":
		return "workflow_form"
	case "evidence":
		return "evidence"
	case "approval":
		return "approval"
	case "mixed":
		return "domain_then_form"
	case "domain":
		return "domain_gate"
	default:
		return "work_item"
	}
}

func executionWorkspaceRoute(projectID, workItemID, workspacePath string) string {
	if projectID == "" || workItemID == "" {
		return ""
	}
	if workspacePath == "" {
		return fmt.Sprintf("/projects/%s/work-items/%s", projectID, workItemID)
	}
	return executionRouteWithTask(fmt.Sprintf("/projects/%s/%s", projectID, workspacePath), workItemID)
}

func executionRouteWithTask(route, workItemID string) string {
	if route == "" || workItemID == "" {
		return route
	}
	parsed, err := url.Parse(route)
	if err != nil {
		separator := "?"
		if strings.Contains(route, "?") {
			separator = "&"
		}
		return route + separator + "task=" + url.QueryEscape(workItemID)
	}
	query := parsed.Query()
	query.Set("task", workItemID)
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func executionSourceRouteAllowed(route, projectID string) bool {
	if route == "" || projectID == "" || strings.HasPrefix(route, "//") {
		return false
	}
	parsed, err := url.Parse(route)
	if err != nil || parsed.IsAbs() || parsed.Host != "" {
		return false
	}
	expectedProjectPath := "/projects/" + url.PathEscape(projectID)
	return parsed.Path == expectedProjectPath || strings.HasPrefix(parsed.Path, expectedProjectPath+"/")
}

func attachExecutionTargets(items []Record) {
	for _, item := range items {
		item["execution_target"] = workItemExecutionTarget(item, nil)
	}
}
