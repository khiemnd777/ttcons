package core

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

const workItemMutationIntentStartOnSuccess = "start_on_success"

type workItemDomainMutationPolicy struct {
	Mutation          string
	Workspaces        []string
	ActionKeys        []string
	ActionKeyPrefixes []string
}

var (
	boqReleaseWorkItemPolicy = workItemDomainMutationPolicy{
		Mutation:   "boq_release",
		ActionKeys: []string{"source.boq_release_ready", "source.boq_approved", "source.boq_items_ready"},
	}
	boqProcurementWorkItemPolicy = workItemDomainMutationPolicy{
		Mutation:   "procurement_generate_from_boq",
		ActionKeys: []string{"procurement.generate_from_boq"},
	}
	inputContractProcurementWorkItemPolicy = workItemDomainMutationPolicy{
		Mutation:   "input_contract_procurement_updated",
		ActionKeys: []string{"source.input_contract_package_ready", "source.procurement_contract_ready", "procurement.require_approved"},
	}
	supplierQuotationWorkItemPolicy = workItemDomainMutationPolicy{
		Mutation:   "supplier_quotation_qualified",
		ActionKeys: []string{"source.supplier_quotation_ready"},
	}
	supplierSelectionWorkItemPolicy = workItemDomainMutationPolicy{
		Mutation:   "supplier_quote_selected",
		ActionKeys: []string{"source.supplier_selection_ready"},
	}
	contractPackageWorkItemPolicy = workItemDomainMutationPolicy{
		Mutation: "contract_package_updated",
		ActionKeys: []string{
			"contract.require_execution_ready",
			"source.input_contract_package_ready",
			"source.procurement_contract_ready",
			"source.contract_version_ready",
			"source.contract_review_ready",
			"source.contract_app_signed",
		},
	}
	constructionHandoffWorkItemPolicy = workItemDomainMutationPolicy{
		Mutation:   "construction_handoff_activated",
		ActionKeys: []string{"construction.activate_handoff"},
	}
	warehouseWorkItemPolicy = workItemDomainMutationPolicy{
		Mutation:          "warehouse_work_item_updated",
		Workspaces:        []string{executionWorkspaceWarehouse},
		ActionKeyPrefixes: []string{"warehouse.", "stock."},
	}
	subcontractorAcceptanceWorkItemPolicy = workItemDomainMutationPolicy{
		Mutation: "subcontractor_acceptance_updated",
		ActionKeys: []string{
			"source.subcontractor_acceptance_package_ready",
			"source.subcontractor_reconciliation_ready",
			"subcontractor.reconcile_checklist",
			"subcontractor.prepare_quantity_dossier",
		},
		Workspaces: []string{executionWorkspaceAcceptance},
	}
	incomingAcceptanceWorkItemPolicy = workItemDomainMutationPolicy{
		Mutation:   "incoming_acceptance_updated",
		ActionKeys: []string{"acceptance.incoming_material"},
		Workspaces: []string{executionWorkspaceAcceptance},
	}
	finalAcceptanceWorkItemPolicy = workItemDomainMutationPolicy{
		Mutation: "final_acceptance_updated",
		ActionKeys: []string{
			"source.final_acceptance_package_ready",
			"source.final_acceptance_ready",
			"owner.reconcile_acceptance",
		},
		Workspaces: []string{executionWorkspaceAcceptance},
	}
	operationsWorkItemPolicy = workItemDomainMutationPolicy{
		Mutation:   "construction_operation_updated",
		Workspaces: []string{executionWorkspaceOperations},
	}
	handoverWorkItemPolicy = workItemDomainMutationPolicy{
		Mutation:   "handover_updated",
		ActionKeys: []string{"source.handover_completed"},
		Workspaces: []string{executionWorkspaceHandover},
	}
	projectFinanceWorkItemPolicy = workItemDomainMutationPolicy{
		Mutation:   "project_finance_updated",
		ActionKeys: []string{"source.project_finance_close_ready", "source.accounting_handoff_close_ready"},
		Workspaces: []string{executionWorkspaceProjectFinance},
	}
)

func contractWorkItemPolicyFor(contract Record, mutation string) workItemDomainMutationPolicy {
	policy := contractPackageWorkItemPolicy
	policy.Mutation = mutation
	if fmt.Sprint(contract["direction"]) == "payable" {
		policy.ActionKeys = []string{
			"source.input_contract_package_ready",
			"source.procurement_contract_ready",
		}
		return policy
	}
	policy.ActionKeys = []string{
		"contract.require_execution_ready",
		"source.contract_version_ready",
		"source.contract_review_ready",
		"source.contract_app_signed",
	}
	return policy
}

func normalizedWorkItemMutationContext(value WorkItemMutationContext) WorkItemMutationContext {
	return WorkItemMutationContext{
		WorkItemID: cleanID(value.WorkItemID),
		Intent:     strings.TrimSpace(value.Intent),
	}
}

func linkedWorkItemMutationContext(workItemID string) WorkItemMutationContext {
	workItemID = cleanID(workItemID)
	if workItemID == "" {
		return WorkItemMutationContext{}
	}
	return WorkItemMutationContext{
		WorkItemID: workItemID,
		Intent:     workItemMutationIntentStartOnSuccess,
	}
}

func workItemMutationContextFromRecord(body Record) WorkItemMutationContext {
	if body == nil {
		return WorkItemMutationContext{}
	}
	raw, ok := body["work_item_context"]
	if !ok || raw == nil {
		return WorkItemMutationContext{}
	}
	contextRecord, ok := raw.(map[string]any)
	if !ok {
		if record, recordOK := raw.(Record); recordOK {
			contextRecord = record
		} else {
			return WorkItemMutationContext{}
		}
	}
	return normalizedWorkItemMutationContext(WorkItemMutationContext{
		WorkItemID: cleanID(contextRecord["work_item_id"]),
		Intent:     strings.TrimSpace(fmt.Sprint(contextRecord["intent"])),
	})
}

func validateWorkItemMutationContext(value WorkItemMutationContext) (WorkItemMutationContext, error) {
	value = normalizedWorkItemMutationContext(value)
	if value.WorkItemID == "" && value.Intent == "" {
		return WorkItemMutationContext{}, nil
	}
	if value.WorkItemID == "" {
		return WorkItemMutationContext{}, validationError("Ngữ cảnh đầu việc thiếu work_item_id")
	}
	if value.Intent != workItemMutationIntentStartOnSuccess {
		return WorkItemMutationContext{}, validationError("Ý định ngữ cảnh đầu việc không hợp lệ", Record{
			"intent": value.Intent,
		})
	}
	return value, nil
}

func workItemDomainMutationPolicyAllows(item Record, policy workItemDomainMutationPolicy) bool {
	actionKey := cleanID(item["domain_action_key"])
	for _, expected := range policy.ActionKeys {
		if actionKey == expected {
			return true
		}
	}
	for _, prefix := range policy.ActionKeyPrefixes {
		if strings.HasPrefix(actionKey, prefix) {
			return true
		}
	}
	if actionKey != "" && (len(policy.ActionKeys) > 0 || len(policy.ActionKeyPrefixes) > 0) {
		return false
	}
	workspace := executionWorkspaceForWorkItem(item, actionKey)
	for _, expected := range policy.Workspaces {
		if workspace == expected {
			return true
		}
	}
	return false
}

func (r *Repository) startWorkItemForDomainMutationTx(
	ctx context.Context,
	tx *sql.Tx,
	projectID string,
	mutationContext WorkItemMutationContext,
	policy workItemDomainMutationPolicy,
	sourceType string,
	sourceID string,
	actorID string,
	actorName string,
) (Record, error) {
	mutationContext, err := validateWorkItemMutationContext(mutationContext)
	if err != nil {
		return nil, err
	}
	if mutationContext.WorkItemID == "" {
		return nil, nil
	}
	projectID = cleanID(projectID)
	if projectID == "" {
		return nil, validationError("Công trình của thao tác nghiệp vụ không hợp lệ")
	}
	if cleanID(actorID) == "" {
		return nil, forbiddenError("Không xác định được người thực hiện đầu việc")
	}

	rows, err := queryTx(ctx, tx, `
		SELECT
			wi.*,
			COALESCE(pr.engine_version, 1) AS pipeline_engine_version,
			pr.status AS pipeline_run_status,
			pr.current_node_id AS pipeline_current_node_id,
			current_node.status AS pipeline_current_node_status,
			target_node.status AS run_node_status,
			target_node.type AS run_node_type,
			target_node.business_code AS run_node_business_code,
			rs.business_code AS run_step_business_code,
			rs.surface_kind,
			rs.domain_action_key
		FROM work_items wi
		LEFT JOIN pipeline_runs pr ON pr.id=wi.pipeline_run_id
		LEFT JOIN pipeline_run_nodes current_node ON current_node.id=pr.current_node_id
		LEFT JOIN pipeline_run_nodes target_node ON target_node.id=wi.run_node_id
		LEFT JOIN pipeline_run_steps rs ON rs.id=wi.run_step_id
		WHERE wi.id=$1
		FOR UPDATE OF wi
	`, mutationContext.WorkItemID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, validationError("Không tìm thấy đầu việc trong ngữ cảnh nghiệp vụ", Record{"work_item_id": mutationContext.WorkItemID})
	}
	item := rows[0]
	if cleanID(item["project_id"]) != projectID {
		return nil, validationError("Đầu việc không thuộc công trình của thao tác nghiệp vụ", Record{
			"work_item_id": mutationContext.WorkItemID,
			"project_id":   projectID,
		})
	}
	if !workItemDomainMutationPolicyAllows(item, policy) {
		return nil, validationError("Đầu việc không khớp module hoặc hành động nghiệp vụ", Record{
			"work_item_id":      mutationContext.WorkItemID,
			"domain_action_key": item["domain_action_key"],
			"workspace":         executionWorkspaceForWorkItem(item, cleanID(item["domain_action_key"])),
			"mutation":          policy.Mutation,
		})
	}

	allowed, err := r.CanActOnWorkItem(ctx, mutationContext.WorkItemID, actorID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, forbiddenError("Bạn không có quyền thao tác đầu việc liên kết")
	}
	requirement := recordValue(item["evidence_requirement_json"])
	requiredPermission := strings.TrimSpace(fmt.Sprint(requirement["required_permission"]))
	if requiredPermission != "" && requiredPermission != "<nil>" {
		allowed, permissionErr := r.UserHasAnyProjectPermission(ctx, actorID, projectID, []string{requiredPermission})
		if permissionErr != nil {
			return nil, permissionErr
		}
		if !allowed {
			return nil, forbiddenError("Bạn không có quyền bắt buộc để thao tác đầu việc này", Record{"permission": requiredPermission})
		}
	}

	fromStatus := fmt.Sprint(item["status"])
	if !statusIn(fromStatus, "pending", "ready", "needs_changes", "in_progress", "blocked") {
		return nil, validationError("Trạng thái đầu việc không cho phép thực hiện nghiệp vụ", Record{
			"work_item_id": mutationContext.WorkItemID,
			"status":       fromStatus,
		})
	}
	if !workItemHasStarted(item) && myWorkPlannedStartInFuture(item["planned_start_at"], time.Now()) {
		return nil, validationError("Đầu việc chưa đến thời điểm được phép bắt đầu", Record{
			"work_item_id":     mutationContext.WorkItemID,
			"planned_start_at": item["planned_start_at"],
		})
	}
	if int(number(item["pipeline_engine_version"])) >= 2 && cleanID(item["pipeline_run_id"]) != "" {
		runtimeRows, runtimeErr := queryTx(ctx, tx, `
			SELECT
				pr.status AS pipeline_run_status,
				pr.current_node_id AS pipeline_current_node_id,
				current_node.status AS pipeline_current_node_status
			FROM pipeline_runs pr
			LEFT JOIN pipeline_run_nodes current_node ON current_node.id=pr.current_node_id
			WHERE pr.id=$1
			FOR UPDATE OF pr
		`, cleanID(item["pipeline_run_id"]))
		if runtimeErr != nil {
			return nil, runtimeErr
		}
		if len(runtimeRows) == 0 {
			return nil, validationError("Không tìm thấy quy trình của đầu việc", Record{"work_item_id": mutationContext.WorkItemID})
		}
		runtime := runtimeRows[0]
		currentNodeID := cleanID(runtime["pipeline_current_node_id"])
		if statusIn(fmt.Sprint(runtime["pipeline_run_status"]), "completed", "cancelled") ||
			currentNodeID == "" || currentNodeID != cleanID(item["run_node_id"]) ||
			!statusIn(fmt.Sprint(runtime["pipeline_current_node_status"]), "in_progress", "waiting_approval", "blocked") {
			return nil, validationError("Đầu việc không còn thuộc Node đang thực hiện của quy trình", Record{
				"work_item_id":    mutationContext.WorkItemID,
				"run_node_id":     item["run_node_id"],
				"current_node_id": currentNodeID,
			})
		}
	}

	transition := Record{
		"work_item_id":    mutationContext.WorkItemID,
		"work_item_title": item["title"],
		"project_id":      projectID,
		"from_status":     fromStatus,
		"to_status":       fromStatus,
		"started":         false,
		"auto_started":    false,
		"mutation":        policy.Mutation,
	}
	if statusIn(fromStatus, "pending", "ready", "needs_changes") {
		metadata := Record{
			"source":      "domain_mutation_start_on_success",
			"mutation":    policy.Mutation,
			"source_type": sourceType,
			"source_id":   cleanID(sourceID),
		}
		if err := persistWorkItemStartedTx(ctx, tx, item, actorID, actorName, "Bắt đầu đầu việc khi hoàn tất thao tác nghiệp vụ đầu tiên", metadata); err != nil {
			return nil, err
		}
		transition["started"] = true
		transition["auto_started"] = true
		transition["to_status"] = "in_progress"
	}
	return transition, nil
}

func attachWorkItemTransition(record Record, transition Record) Record {
	if record != nil && transition != nil {
		record["work_item_transition"] = transition
	}
	return record
}
