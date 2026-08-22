package core

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

func (r *Repository) ProjectFinanceSummary(ctx context.Context, projectID string) (Record, error) {
	return r.QueryOne(ctx, `
		WITH contract_summary AS (
			SELECT COALESCE(MAX(current_amount) FILTER (WHERE contract_type='main_client' AND status='active'),0) AS contract_value
			FROM contracts WHERE project_id=$1
		), revenue_summary AS (
			SELECT COALESCE(SUM(milestone.net_amount),0) AS eligible_revenue
			FROM contract_milestones milestone
			JOIN contracts contract ON contract.id=milestone.contract_id
			WHERE contract.project_id=$1 AND contract.contract_type='main_client'
			  AND contract.status IN ('active','completed')
			  AND milestone.status IN ('eligible','obligation_opened','paid')
		), procurement_summary AS (
			SELECT COALESCE(SUM(total_estimated_amount),0) AS committed_material_cost
			FROM procurement_requests
			WHERE project_id=$1 AND status IN ('ordered','partially_received','received')
		), cost_summary AS (
			SELECT COALESCE(SUM(recognized_cost_amount) FILTER (WHERE status='confirmed'),0) AS confirmed_material_cost,
			       COUNT(*) FILTER (WHERE status IN ('draft','submitted','needs_changes')) AS unconfirmed_cost_count,
			       COALESCE(SUM(total_amount) FILTER (WHERE status IN ('draft','submitted','needs_changes')),0) AS unconfirmed_cost_amount
			FROM project_material_costs WHERE project_id=$1 AND status <> 'cancelled'
		), cash_summary AS (
			SELECT COUNT(*) FILTER (WHERE status NOT IN ('processed','cancelled')) AS open_cash_request_count,
			       COALESCE(SUM(amount) FILTER (WHERE direction='inbound' AND status NOT IN ('processed','cancelled')),0) AS open_inbound_amount,
			       COALESCE(SUM(amount) FILTER (WHERE direction='outbound' AND status NOT IN ('processed','cancelled')),0) AS open_outbound_amount
			FROM project_cash_requests WHERE project_id=$1
		)
		SELECT project.id AS project_id,project.name AS project_name,project.code AS project_code,
		       contract_summary.contract_value,revenue_summary.eligible_revenue,
		       procurement_summary.committed_material_cost,cost_summary.confirmed_material_cost,
		       revenue_summary.eligible_revenue-cost_summary.confirmed_material_cost AS provisional_profit,
		       cost_summary.unconfirmed_cost_count,cost_summary.unconfirmed_cost_amount,
		       cash_summary.open_cash_request_count,cash_summary.open_inbound_amount,cash_summary.open_outbound_amount,
		       'Chỉ phản ánh chi phí mua vật tư đã xác nhận; chưa bao gồm nhân công, thầu phụ, máy thi công, chi phí chung, chi phí quản lý và thuế.'::text AS scope_warning
		FROM projects project
		CROSS JOIN contract_summary CROSS JOIN revenue_summary CROSS JOIN procurement_summary CROSS JOIN cost_summary CROSS JOIN cash_summary
		WHERE project.id=$1 AND project.status <> 'cancelled'
	`, projectID)
}

func (r *Repository) ProjectFinanceTasks(ctx context.Context, projectID string) ([]Record, error) {
	return r.Query(ctx, `
		SELECT * FROM (
			SELECT cost.id,'material_cost'::text AS task_type,cost.description AS title,cost.vendor_name AS counterparty,
			       cost.total_amount AS amount,cost.status,cost.invoice_date AS due_date,cost.updated_at
			FROM project_material_costs cost
			WHERE cost.project_id=$1 AND cost.status IN ('draft','submitted','needs_changes')
			UNION ALL
			SELECT request.id,'cash_request'::text,request.title,request.counterparty_name,
			       request.amount,request.status,request.due_date,request.updated_at
			FROM project_cash_requests request
			WHERE request.project_id=$1 AND request.status NOT IN ('processed','cancelled')
		) task
		ORDER BY task.updated_at DESC,task.id
	`, projectID)
}

func (r *Repository) ProjectMaterialCosts(ctx context.Context, projectID string) ([]Record, error) {
	return r.Query(ctx, `
		SELECT cost.*,procurement.request_code,acceptance.acceptance_code,stock.document_code,
		       creator.full_name AS created_by_name,COALESCE(creator.deleted_at IS NOT NULL,false) AS created_by_account_deleted,
		       submitter.full_name AS submitted_by_name,COALESCE(submitter.deleted_at IS NOT NULL,false) AS submitted_by_account_deleted,
		       reviewer.full_name AS reviewed_by_name,COALESCE(reviewer.deleted_at IS NOT NULL,false) AS reviewed_by_account_deleted
		FROM project_material_costs cost
		LEFT JOIN procurement_requests procurement ON procurement.id=cost.procurement_request_id
		LEFT JOIN incoming_acceptance_records acceptance ON acceptance.id=cost.incoming_acceptance_id
		LEFT JOIN stock_documents stock ON stock.id=cost.stock_document_id
		LEFT JOIN users creator ON creator.id=cost.created_by
		LEFT JOIN users submitter ON submitter.id=cost.submitted_by
		LEFT JOIN users reviewer ON reviewer.id=cost.reviewed_by
		WHERE cost.project_id=$1 AND cost.status <> 'cancelled'
		ORDER BY cost.invoice_date DESC NULLS LAST,cost.created_at DESC,cost.id
	`, projectID)
}

func (r *Repository) ProjectCashRequests(ctx context.Context, projectID string) ([]Record, error) {
	return r.Query(ctx, `
		SELECT request.*,creator.full_name AS created_by_name,COALESCE(creator.deleted_at IS NOT NULL,false) AS created_by_account_deleted,
		       submitter.full_name AS submitted_by_name,COALESCE(submitter.deleted_at IS NOT NULL,false) AS submitted_by_account_deleted,
		       approver.full_name AS approved_by_name,COALESCE(approver.deleted_at IS NOT NULL,false) AS approved_by_account_deleted,
		       processor.full_name AS processed_by_name,COALESCE(processor.deleted_at IS NOT NULL,false) AS processed_by_account_deleted
		FROM project_cash_requests request
		LEFT JOIN users creator ON creator.id=request.created_by
		LEFT JOIN users submitter ON submitter.id=request.submitted_by
		LEFT JOIN users approver ON approver.id=request.approved_by
		LEFT JOIN users processor ON processor.id=request.processed_by
		WHERE request.project_id=$1 AND request.status <> 'cancelled'
		ORDER BY request.due_date DESC NULLS LAST,request.created_at DESC,request.id
	`, projectID)
}

func (r *Repository) ProjectFinanceOptions(ctx context.Context, projectID string) (Record, error) {
	procurements, err := r.Query(ctx, `SELECT id,request_code,title,status,total_estimated_amount FROM procurement_requests WHERE project_id=$1 AND status <> 'cancelled' ORDER BY created_at DESC`, projectID)
	if err != nil {
		return nil, err
	}
	acceptances, err := r.Query(ctx, `SELECT id,acceptance_code,supplier_name,received_at,result FROM incoming_acceptance_records WHERE project_id=$1 AND status='submitted' ORDER BY received_at DESC NULLS LAST,created_at DESC`, projectID)
	if err != nil {
		return nil, err
	}
	stockDocuments, err := r.Query(ctx, `SELECT id,document_code,document_date,movement_type FROM stock_documents WHERE project_id=$1 AND status='posted' ORDER BY document_date DESC,created_at DESC`, projectID)
	if err != nil {
		return nil, err
	}
	partners, err := r.Query(ctx, `SELECT id,name,tax_code FROM partners WHERE active AND partner_types ?| ARRAY['supplier','subcontractor'] ORDER BY name`)
	if err != nil {
		return nil, err
	}
	return Record{"procurements": procurements, "incoming_acceptances": acceptances, "stock_documents": stockDocuments, "partners": partners}, nil
}

func (r *Repository) CreateProjectMaterialCost(ctx context.Context, projectID string, req ProjectMaterialCostRequest, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err := validateProjectFinanceSources(ctx, tx, projectID, req.ProcurementRequestID, req.IncomingAcceptanceID, req.StockDocumentID, req.DocumentID); err != nil {
		return nil, err
	}
	rows, err := queryTx(ctx, tx, `
		INSERT INTO project_material_costs (
			project_id,procurement_request_id,incoming_acceptance_id,stock_document_id,document_id,vendor_partner_id,
			vendor_name,invoice_number,invoice_date,description,amount_before_vat,vat_amount,total_amount,recognized_cost_amount,
			notes,status,created_by
		) VALUES ($1,NULLIF($2,'')::uuid,NULLIF($3,'')::uuid,NULLIF($4,'')::uuid,NULLIF($5,'')::uuid,NULLIF($6,'')::uuid,
			$7,NULLIF($8,''),NULLIF($9,'')::date,$10,$11,$12,$13,0,NULLIF($14,''),'draft',$15)
		RETURNING *
	`, projectID, req.ProcurementRequestID, req.IncomingAcceptanceID, req.StockDocumentID, req.DocumentID, req.VendorPartnerID,
		req.VendorName, req.InvoiceNumber, req.InvoiceDate, req.Description, req.AmountBeforeVAT, req.VATAmount, req.TotalAmount, req.Notes, actorID)
	if err != nil {
		return nil, err
	}
	row := rows[0]
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "project_material_cost_created", "project_material_cost", cleanID(row["id"]), "Ghi nhận chi phí mua vật tư công trình", Record{"total_amount": req.TotalAmount, "vendor_name": req.VendorName}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return row, nil
}

func validateProjectFinanceSources(ctx context.Context, tx *sql.Tx, projectID, procurementID, acceptanceID, stockDocumentID, documentID string) error {
	checks := []struct{ name, table, id string }{
		{"yêu cầu mua", "procurement_requests", procurementID},
		{"nghiệm thu đầu vào", "incoming_acceptance_records", acceptanceID},
		{"phiếu kho", "stock_documents", stockDocumentID},
		{"tài liệu", "documents", documentID},
	}
	for _, check := range checks {
		if strings.TrimSpace(check.id) == "" {
			continue
		}
		query := fmt.Sprintf("SELECT COUNT(*) AS total FROM %s WHERE id=$1 AND project_id=$2", check.table)
		rows, err := queryTx(ctx, tx, query, check.id, projectID)
		if err != nil {
			return err
		}
		if len(rows) == 0 || number(rows[0]["total"]) == 0 {
			return validationError(fmt.Sprintf("%s nguồn không thuộc công trình", check.name), Record{"id": check.id})
		}
	}
	return nil
}

func (r *Repository) CreateProjectCashRequest(ctx context.Context, projectID string, req ProjectCashRequestRequest, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err := validateProjectFinanceSources(ctx, tx, projectID, "", "", "", req.DocumentID); err != nil {
		return nil, err
	}
	if err := validateProjectFinanceCashSource(ctx, tx, projectID, req.SourceType, req.SourceID); err != nil {
		return nil, err
	}
	prefix := "DNT"
	if req.Direction == "outbound" {
		prefix = "DNC"
	}
	requestCode := fmt.Sprintf("%s-%s-%s", prefix, time.Now().UTC().Format("20060102"), strings.ToUpper(strings.ReplaceAll(uuid.NewString()[:8], "-", "")))
	rows, err := queryTx(ctx, tx, `
		INSERT INTO project_cash_requests (
			project_id,request_code,direction,title,counterparty_name,amount,due_date,source_type,source_id,document_id,notes,status,created_by
		) VALUES ($1,$2,$3,$4,$5,$6,NULLIF($7,'')::date,NULLIF($8,''),NULLIF($9,'')::uuid,NULLIF($10,'')::uuid,NULLIF($11,''),'draft',$12)
		RETURNING *
	`, projectID, requestCode, req.Direction, req.Title, req.CounterpartyName, req.Amount, req.DueDate, req.SourceType, req.SourceID, req.DocumentID, req.Notes, actorID)
	if err != nil {
		return nil, err
	}
	row := rows[0]
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "project_cash_request_created", "project_cash_request", cleanID(row["id"]), "Lập đề nghị thu/chi nội bộ", Record{"request_code": requestCode, "direction": req.Direction, "amount": req.Amount}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return row, nil
}

func validateProjectFinanceCashSource(ctx context.Context, tx *sql.Tx, projectID, sourceType, sourceID string) error {
	if sourceType == "" && sourceID == "" {
		return nil
	}
	projectTables := map[string]string{
		"procurement_request": "procurement_requests",
		"incoming_acceptance": "incoming_acceptance_records",
		"stock_document":      "stock_documents",
		"document":            "documents",
		"acceptance":          "acceptance_records",
		"work_item":           "work_items",
	}
	var query string
	if table, ok := projectTables[sourceType]; ok {
		query = fmt.Sprintf("SELECT COUNT(*) AS total FROM %s WHERE id=$1 AND project_id=$2", table)
	} else if sourceType == "contract_milestone" {
		query = `SELECT COUNT(*) AS total FROM contract_milestones milestone JOIN contracts contract ON contract.id=milestone.contract_id WHERE milestone.id=$1 AND contract.project_id=$2`
	} else {
		return validationError("Loại hồ sơ nguồn của đề nghị thu/chi không hợp lệ", Record{"source_type": sourceType})
	}
	rows, err := queryTx(ctx, tx, query, sourceID, projectID)
	if err != nil {
		return err
	}
	if len(rows) == 0 || number(rows[0]["total"]) == 0 {
		return validationError("Hồ sơ nguồn của đề nghị thu/chi không thuộc công trình", Record{"source_type": sourceType, "source_id": sourceID})
	}
	return nil
}

func (r *Repository) ProjectMaterialCostAction(ctx context.Context, id, action string, req ProjectFinanceActionRequest, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := queryTx(ctx, tx, `SELECT * FROM project_material_costs WHERE id=$1 AND status <> 'cancelled' FOR UPDATE`, id)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	current := rows[0]
	fromStatus := fmt.Sprint(current["status"])
	comment := strings.TrimSpace(req.Comment)
	var updateQuery string
	var updateArgs []any
	switch action {
	case "submit":
		if !statusIn(fromStatus, "draft", "needs_changes") {
			return nil, validationError("Chi phí không ở trạng thái có thể gửi", Record{"status": fromStatus})
		}
		updateQuery = `UPDATE project_material_costs SET status='submitted',submitted_by=$2,submitted_at=now(),reviewed_by=NULL,reviewed_at=NULL,review_comment=NULL,updated_at=now() WHERE id=$1 RETURNING *`
		updateArgs = []any{id, actorID}
	case "request_changes":
		if fromStatus != "submitted" {
			return nil, validationError("Chỉ yêu cầu sửa hồ sơ đang chờ xác nhận", Record{"status": fromStatus})
		}
		if comment == "" {
			return nil, validationError("Cần ghi rõ nội dung yêu cầu sửa")
		}
		updateQuery = `UPDATE project_material_costs SET status='needs_changes',reviewed_by=$2,reviewed_at=now(),review_comment=$3,updated_at=now() WHERE id=$1 RETURNING *`
		updateArgs = []any{id, actorID, comment}
	case "confirm":
		if fromStatus != "submitted" {
			return nil, validationError("Chỉ xác nhận hồ sơ đã gửi", Record{"status": fromStatus})
		}
		recognized := req.RecognizedCostAmount
		if recognized <= 0 {
			recognized = number(current["total_amount"])
		}
		if recognized > number(current["total_amount"])+0.001 {
			return nil, validationError("Chi phí được xác nhận không được vượt tổng giá trị hồ sơ")
		}
		req.RecognizedCostAmount = recognized
		updateQuery = `UPDATE project_material_costs SET status='confirmed',recognized_cost_amount=$3,reviewed_by=$2,reviewed_at=now(),review_comment=NULLIF($4,''),updated_at=now() WHERE id=$1 RETURNING *`
		updateArgs = []any{id, actorID, recognized, comment}
	default:
		return nil, validationError("Hành động chi phí vật tư không hợp lệ", Record{"action": action})
	}
	updated, err := queryTx(ctx, tx, updateQuery, updateArgs...)
	if err != nil {
		return nil, err
	}
	row := updated[0]
	if err := addAuditTx(ctx, tx, cleanID(current["project_id"]), actorID, actorName, "project_material_cost_"+action, "project_material_cost", id, "Cập nhật hồ sơ chi phí vật tư", Record{"from_status": fromStatus, "to_status": row["status"], "comment": comment, "recognized_cost_amount": row["recognized_cost_amount"]}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return row, nil
}

func (r *Repository) ProjectCashRequestAction(ctx context.Context, id, action string, req ProjectFinanceActionRequest, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := queryTx(ctx, tx, `SELECT * FROM project_cash_requests WHERE id=$1 AND status <> 'cancelled' FOR UPDATE`, id)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	current := rows[0]
	fromStatus := fmt.Sprint(current["status"])
	comment := strings.TrimSpace(req.Comment)
	var updateQuery, toStatus string
	var updateArgs []any
	switch action {
	case "submit":
		if !statusIn(fromStatus, "draft", "needs_changes") {
			return nil, validationError("Đề nghị không ở trạng thái có thể gửi", Record{"status": fromStatus})
		}
		toStatus = "submitted"
		updateQuery = `UPDATE project_cash_requests SET status='submitted',submitted_by=$2,submitted_at=now(),approved_by=NULL,approved_at=NULL,handed_to_accounting_by=NULL,handed_to_accounting_at=NULL,processed_by=NULL,processed_at=NULL,review_comment=NULL,updated_at=now() WHERE id=$1 RETURNING *`
		updateArgs = []any{id, actorID}
	case "approve":
		if fromStatus != "submitted" {
			return nil, validationError("Chỉ duyệt đề nghị đã gửi", Record{"status": fromStatus})
		}
		toStatus = "approved"
		updateQuery = `UPDATE project_cash_requests SET status='approved',approved_by=$2,approved_at=now(),review_comment=NULLIF($3,''),updated_at=now() WHERE id=$1 RETURNING *`
		updateArgs = []any{id, actorID, comment}
	case "handoff":
		if fromStatus != "approved" {
			return nil, validationError("Đề nghị phải được duyệt trước khi bàn giao Kế toán", Record{"status": fromStatus})
		}
		toStatus = "handed_to_accounting"
		updateQuery = `UPDATE project_cash_requests SET status='handed_to_accounting',handed_to_accounting_by=$2,handed_to_accounting_at=now(),updated_at=now() WHERE id=$1 RETURNING *`
		updateArgs = []any{id, actorID}
	case "request_changes":
		if !statusIn(fromStatus, "submitted", "approved") {
			return nil, validationError("Đề nghị không ở trạng thái có thể yêu cầu sửa", Record{"status": fromStatus})
		}
		if comment == "" {
			return nil, validationError("Cần ghi rõ nội dung yêu cầu sửa")
		}
		toStatus = "needs_changes"
		updateQuery = `UPDATE project_cash_requests SET status='needs_changes',approved_by=NULL,approved_at=NULL,handed_to_accounting_by=NULL,handed_to_accounting_at=NULL,processed_by=NULL,processed_at=NULL,review_comment=$2,updated_at=now() WHERE id=$1 RETURNING *`
		updateArgs = []any{id, comment}
	case "process":
		if fromStatus != "handed_to_accounting" {
			return nil, validationError("Chỉ xác nhận xử lý hồ sơ đã bàn giao Kế toán", Record{"status": fromStatus})
		}
		toStatus = "processed"
		updateQuery = `UPDATE project_cash_requests SET status='processed',processed_by=$2,processed_at=now(),review_comment=NULLIF($3,''),updated_at=now() WHERE id=$1 RETURNING *`
		updateArgs = []any{id, actorID, comment}
	default:
		return nil, validationError("Hành động đề nghị thu/chi không hợp lệ", Record{"action": action})
	}
	updated, err := queryTx(ctx, tx, updateQuery, updateArgs...)
	if err != nil {
		return nil, err
	}
	row := updated[0]
	if err := addAuditTx(ctx, tx, cleanID(current["project_id"]), actorID, actorName, "project_cash_request_"+action, "project_cash_request", id, "Cập nhật đề nghị thu/chi nội bộ", Record{"from_status": fromStatus, "to_status": toStatus, "comment": comment}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return row, nil
}
