package core

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

type runEdgeConditionEvaluator func(*Repository, context.Context, *sql.Tx, Record, Record) (bool, string, error)

var runEdgeConditionEvaluators = map[string]runEdgeConditionEvaluator{
	"always":                      evaluateAlwaysEdge,
	"order_index":                 evaluateOrderIndexEdge,
	"approval_approved":           evaluateApprovalApprovedEdge,
	"approval_rejected":           evaluateApprovalRejectedEdge,
	"acceptance_passed":           evaluateAcceptancePassedEdge,
	"acceptance_failed":           evaluateAcceptanceFailedEdge,
	"rework_required":             evaluateReworkRequiredEdge,
	"rework_completed":            evaluateReworkCompletedEdge,
	"replanning_required":         evaluateReplanningRequiredEdge,
	"materials_issued":            evaluateMaterialsIssuedEdge,
	"boq_approved":                evaluateBOQApprovedEdge,
	"procurement_approved":        evaluateProcurementApprovedEdge,
	"workflow_form_answer_equals": evaluateWorkflowFormAnswerEqualsEdge,
	"handover_ready":              evaluateHandoverReadyEdge,
	"financial_ready":             evaluateFinancialReadyEdge,
}

// Published runtime snapshots are immutable. This registry only keeps old runs
// executable; legacy conditions are deliberately absent from the authoring catalog.
var legacyRunEdgeConditionEvaluators = map[string]runEdgeConditionEvaluator{
	"project_entry_mode": evaluateLegacyProjectEntryModeEdge,
}

func evaluateRunEdgeCondition(r *Repository, ctx context.Context, tx *sql.Tx, edge, fromNode Record) (bool, string, error) {
	conditionType := strings.TrimSpace(edgeConditionType(edge))
	if conditionType == "" {
		conditionType = "always"
	}
	if evaluator, exists := runEdgeConditionEvaluators[conditionType]; exists {
		return evaluator(r, ctx, tx, edge, fromNode)
	}
	if evaluator, exists := legacyRunEdgeConditionEvaluators[conditionType]; exists {
		return evaluator(r, ctx, tx, edge, fromNode)
	}
	return false, fmt.Sprintf("Loại điều kiện chuyển bước chưa được hỗ trợ: %s", conditionType), nil
}

func evaluateAlwaysEdge(_ *Repository, _ context.Context, _ *sql.Tx, _, _ Record) (bool, string, error) {
	return true, "Điều kiện luôn khớp", nil
}

func evaluateOrderIndexEdge(_ *Repository, _ context.Context, _ *sql.Tx, edge, fromNode Record) (bool, string, error) {
	matched := number(edge["to_order_index"]) > number(fromNode["order_index"])
	return matched, boolReason(matched, "Node đích đứng sau node nguồn", "Node đích không đứng sau node nguồn"), nil
}

func evaluateApprovalApprovedEdge(r *Repository, ctx context.Context, tx *sql.Tx, _, fromNode Record) (bool, string, error) {
	return r.edgeExists(ctx, tx, "Phê duyệt node đã được duyệt", "Phê duyệt node chưa được duyệt", `
		SELECT COUNT(*) AS total FROM approval_requests
		WHERE project_id=$1 AND target_type='pipeline_run_node' AND target_id=$2 AND status='approved'
	`, fromNode["project_id"], fromNode["id"])
}

func evaluateApprovalRejectedEdge(r *Repository, ctx context.Context, tx *sql.Tx, _, fromNode Record) (bool, string, error) {
	return r.edgeExists(ctx, tx, "Phê duyệt node đã bị từ chối", "Phê duyệt node chưa bị từ chối", `
		SELECT COUNT(*) AS total FROM approval_requests
		WHERE project_id=$1 AND target_type='pipeline_run_node' AND target_id=$2 AND status='rejected'
	`, fromNode["project_id"], fromNode["id"])
}

func evaluateAcceptancePassedEdge(r *Repository, ctx context.Context, tx *sql.Tx, _, fromNode Record) (bool, string, error) {
	return r.edgeExists(ctx, tx, "Nghiệm thu đã đạt", "Chưa có nghiệm thu hoàn tất đạt", `
		SELECT COUNT(*) AS total FROM acceptance_records
		WHERE project_id=$1 AND result='pass' AND status='completed'
	`, fromNode["project_id"])
}

func evaluateAcceptanceFailedEdge(r *Repository, ctx context.Context, tx *sql.Tx, _, fromNode Record) (bool, string, error) {
	return r.edgeExists(ctx, tx, "Nghiệm thu không đạt", "Chưa có nghiệm thu không đạt", `
		SELECT COUNT(*) AS total FROM acceptance_records WHERE project_id=$1 AND result='fail'
	`, fromNode["project_id"])
}

func evaluateReworkRequiredEdge(r *Repository, ctx context.Context, tx *sql.Tx, _, fromNode Record) (bool, string, error) {
	return r.edgeExists(ctx, tx, "Cần sửa lỗi", "Không có yêu cầu sửa lỗi đang mở", `
		SELECT COUNT(*) AS total FROM (
			SELECT id FROM rework_requests WHERE project_id=$1 AND status IN ('draft','approved','in_progress')
			UNION ALL
			SELECT id FROM issues WHERE project_id=$1 AND requires_rework AND status IN ('open','in_progress')
		) required_rework
	`, fromNode["project_id"])
}

func evaluateReworkCompletedEdge(r *Repository, ctx context.Context, tx *sql.Tx, _, fromNode Record) (bool, string, error) {
	projectID := fromNode["project_id"]
	total, err := r.scalarNumberTx(ctx, tx, `SELECT COUNT(*) AS total FROM rework_requests WHERE project_id=$1`, projectID)
	if err != nil {
		return false, "", err
	}
	open, err := r.scalarNumberTx(ctx, tx, `SELECT COUNT(*) AS total FROM rework_requests WHERE project_id=$1 AND status IN ('draft','approved','in_progress')`, projectID)
	if err != nil {
		return false, "", err
	}
	matched := total > 0 && open == 0
	return matched, boolReason(matched, "Tất cả yêu cầu sửa lỗi đã hoàn tất", "Yêu cầu sửa lỗi vẫn đang mở hoặc chưa tồn tại"), nil
}

func evaluateReplanningRequiredEdge(r *Repository, ctx context.Context, tx *sql.Tx, _, fromNode Record) (bool, string, error) {
	return r.edgeExists(ctx, tx, "Cần lập kế hoạch lại", "Không có yêu cầu lập kế hoạch lại đang mở", `
		SELECT COUNT(*) AS total FROM (
			SELECT id FROM replanning_requests WHERE project_id=$1 AND status IN ('draft','submitted')
			UNION ALL
			SELECT id FROM issues WHERE project_id=$1 AND requires_replanning AND status IN ('open','in_progress')
		) required_replanning
	`, fromNode["project_id"])
}

func evaluateMaterialsIssuedEdge(r *Repository, ctx context.Context, tx *sql.Tx, _, fromNode Record) (bool, string, error) {
	missing, err := r.scalarNumberTx(ctx, tx, `
		SELECT COUNT(*) AS total FROM site_material_balances
		WHERE project_id=$1 AND issued_quantity < planned_quantity
	`, fromNode["project_id"])
	if err != nil {
		return false, "", err
	}
	matched := missing == 0
	return matched, boolReason(matched, "Vật tư theo kế hoạch đã xuất đủ", "Vật tư theo kế hoạch vẫn còn thiếu"), nil
}

func evaluateBOQApprovedEdge(r *Repository, ctx context.Context, tx *sql.Tx, _, fromNode Record) (bool, string, error) {
	projectID := fromNode["project_id"]
	total, err := r.scalarNumberTx(ctx, tx, `SELECT COUNT(*) AS total FROM boq_items WHERE project_id=$1 AND status <> 'deleted'`, projectID)
	if err != nil {
		return false, "", err
	}
	notApproved, err := r.scalarNumberTx(ctx, tx, `SELECT COUNT(*) AS total FROM boq_items WHERE project_id=$1 AND status <> 'deleted' AND status <> 'approved'`, projectID)
	if err != nil {
		return false, "", err
	}
	matched := total > 0 && notApproved == 0
	return matched, boolReason(matched, "Tất cả dòng BOQ đã được duyệt", "BOQ đang thiếu dữ liệu hoặc còn dòng chưa duyệt"), nil
}

func evaluateProcurementApprovedEdge(r *Repository, ctx context.Context, tx *sql.Tx, _, fromNode Record) (bool, string, error) {
	projectID := fromNode["project_id"]
	total, err := r.scalarNumberTx(ctx, tx, `SELECT COUNT(*) AS total FROM procurement_requests WHERE project_id=$1 AND status <> 'cancelled'`, projectID)
	if err != nil {
		return false, "", err
	}
	notApproved, err := r.scalarNumberTx(ctx, tx, `
		SELECT COUNT(*) AS total FROM procurement_requests
		WHERE project_id=$1 AND status <> 'cancelled' AND status NOT IN ('approved','ordered','partially_received','received')
	`, projectID)
	if err != nil {
		return false, "", err
	}
	matched := total > 0 && notApproved == 0
	return matched, boolReason(matched, "Yêu cầu mua/đầu vào đã được duyệt", "Thiếu yêu cầu mua/đầu vào hoặc còn yêu cầu chưa được duyệt"), nil
}

func evaluateWorkflowFormAnswerEqualsEdge(r *Repository, ctx context.Context, tx *sql.Tx, edge, fromNode Record) (bool, string, error) {
	return r.workflowFormAnswerEqualsCondition(ctx, tx, edge, fmt.Sprint(fromNode["project_id"]), fmt.Sprint(fromNode["id"]))
}

func evaluateHandoverReadyEdge(r *Repository, ctx context.Context, tx *sql.Tx, _, fromNode Record) (bool, string, error) {
	return r.handoverReadyCondition(ctx, tx, fmt.Sprint(fromNode["project_id"]))
}

func evaluateFinancialReadyEdge(r *Repository, ctx context.Context, tx *sql.Tx, _, fromNode Record) (bool, string, error) {
	openDebt, err := r.scalarNumberTx(ctx, tx, `
		SELECT COUNT(*) AS total FROM (
			SELECT id FROM payables WHERE project_id=$1 AND status <> 'cancelled' AND remaining_amount > 0
			UNION ALL
			SELECT id FROM receivables WHERE project_id=$1 AND status <> 'cancelled' AND remaining_amount > 0
		) open_debt
	`, fromNode["project_id"])
	if err != nil {
		return false, "", err
	}
	matched := openDebt == 0
	return matched, boolReason(matched, "Không còn công nợ phải thu hoặc phải trả đang mở", "Công nợ phải thu hoặc phải trả vẫn còn mở"), nil
}

func evaluateLegacyProjectEntryModeEdge(_ *Repository, ctx context.Context, tx *sql.Tx, edge, fromNode Record) (bool, string, error) {
	config := edgeConditionConfig(edge)
	expected := strings.TrimSpace(fmt.Sprint(config["value"]))
	var actual string
	if err := tx.QueryRowContext(ctx, `SELECT workflow_entry_mode FROM projects WHERE id=$1`, fromNode["project_id"]).Scan(&actual); err != nil {
		return false, "", err
	}
	matched := actual == expected
	return matched, boolReason(matched, fmt.Sprintf("Điểm bắt đầu công trình là %s", expected), fmt.Sprintf("Điểm bắt đầu công trình là %s, không phải %s", actual, expected)), nil
}
