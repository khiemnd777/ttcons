package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

func (r *Repository) ListProjectHandover(ctx context.Context, projectID string) ([]Record, error) {
	return r.Query(ctx, `
		SELECT h.*,
			COALESCE(items.total_items,0) AS checklist_total,
			COALESCE(items.verified_items,0) AS checklist_verified,
			COALESCE(items.pending_required_items,0) AS checklist_pending_required
		FROM handover_records h
		LEFT JOIN LATERAL (
			SELECT COUNT(*) AS total_items,
				COUNT(*) FILTER (WHERE status IN ('verified','waived')) AS verified_items,
				COUNT(*) FILTER (WHERE required AND status <> 'verified') AS pending_required_items
			FROM handover_checklist_items i WHERE i.handover_id=h.id
		) items ON true
		WHERE h.project_id=$1 AND h.status <> 'deleted'
		ORDER BY h.created_at DESC
	`, projectID)
}

func (r *Repository) HandoverOptions(ctx context.Context, projectID string) (Record, error) {
	acceptances, err := r.Query(ctx, `
		SELECT id,title,result,acceptance_date,client_representative,contractor_representative
		FROM acceptance_records
		WHERE project_id=$1 AND type='final' AND status='completed' AND result IN ('pass','conditional_pass')
		ORDER BY acceptance_date DESC NULLS LAST,updated_at DESC
	`, projectID)
	if err != nil {
		return nil, err
	}
	return Record{"final_acceptances": acceptances}, nil
}

func (r *Repository) CreateHandover(ctx context.Context, body Record) (Record, error) {
	return r.Insert(ctx, "handover_records", handoverColumns(), body)
}

func (r *Repository) CreateHandoverPackage(ctx context.Context, body Record, mutationContext WorkItemMutationContext, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := queryTx(ctx, tx, `
		INSERT INTO handover_records (
			project_id,handover_code,status,final_acceptance_id,workflow_form_instance_id,handover_date,
			client_representative,company_representative,documents_ready,warehouse_closed,
			payables_recorded,receivables_recorded,notes,created_by
		) VALUES ($1,$2,'draft',NULLIF($3,'')::uuid,NULLIF($4,'')::uuid,NULLIF($5,'')::date,$6,$7,false,false,false,false,NULLIF($8,''),NULLIF($9,'')::uuid)
		RETURNING *
	`, body["project_id"], body["handover_code"], cleanID(body["final_acceptance_id"]), cleanID(body["workflow_form_instance_id"]), body["handover_date"], body["client_representative"], body["company_representative"], body["notes"], actorID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	handover := rows[0]
	items := []Record{
		{"item_code": "final_acceptance", "category": "quality", "title": "Biên bản nghiệm thu cuối đạt", "source_type": "acceptance", "source_id": cleanID(body["final_acceptance_id"])},
		{"item_code": "warehouse_closure", "category": "operation", "title": "Kho công trình và vật tư dư đã đóng", "source_type": "system_check", "source_id": handover["id"]},
		{"item_code": "project_material_costs", "category": "finance", "title": "Chi phí vật tư công trình đã được xác nhận", "source_type": "system_check", "source_id": handover["id"]},
		{"item_code": "project_cash_requests", "category": "finance", "title": "Đề nghị thu/chi nội bộ đã được xử lý", "source_type": "system_check", "source_id": handover["id"]},
		{"item_code": "as_built_documents", "category": "as_built", "title": "Bản vẽ và hồ sơ hoàn công"},
		{"item_code": "operation_manuals", "category": "operation", "title": "Hướng dẫn vận hành/bảo trì"},
		{"item_code": "warranty_documents", "category": "warranty", "title": "Cam kết và hồ sơ bảo hành"},
		{"item_code": "signed_handover_minutes", "category": "legal", "title": "Biên bản bàn giao có xác nhận hai bên"},
	}
	splitFinance, err := r.projectUsesSplitHandoverFinanceTx(ctx, tx, cleanID(handover["project_id"]))
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		required := true
		if splitFinance && statusIn(fmt.Sprint(item["item_code"]), "project_material_costs", "project_cash_requests") {
			required = false
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO handover_checklist_items (handover_id,project_id,item_code,category,title,required,status,source_type,source_id)
			VALUES ($1,$2,$3,$4,$5,$6,'pending',NULLIF($7,''),NULLIF($8,'')::uuid)
		`, handover["id"], handover["project_id"], item["item_code"], item["category"], item["title"], required, item["source_type"], cleanID(item["source_id"])); err != nil {
			return nil, err
		}
	}
	if err := addAuditTx(ctx, tx, cleanID(handover["project_id"]), actorID, actorName, "handover_created", "handover", cleanID(handover["id"]), "Tạo hồ sơ bàn giao", Record{"handover_code": handover["handover_code"], "final_acceptance_id": handover["final_acceptance_id"], "checklist_items": len(items)}); err != nil {
		return nil, err
	}
	transition, err := r.startWorkItemForDomainMutationTx(ctx, tx, cleanID(handover["project_id"]), mutationContext, handoverWorkItemPolicy, "handover", cleanID(handover["id"]), actorID, actorName)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	result, err := r.GetHandoverDetail(ctx, cleanID(handover["id"]))
	return attachWorkItemTransition(result, transition), err
}

func (r *Repository) GetHandoverDetail(ctx context.Context, id string) (Record, error) {
	handover, err := r.QueryOne(ctx, `
		SELECT h.*, a.title AS final_acceptance_title, a.status AS final_acceptance_status, a.result AS final_acceptance_result
		FROM handover_records h
		LEFT JOIN acceptance_records a ON a.id=h.final_acceptance_id AND a.project_id=h.project_id
		WHERE h.id=$1 AND h.status <> 'deleted'
	`, id)
	if err != nil {
		return nil, err
	}
	items, err := r.Query(ctx, `
		SELECT i.*,
			e.name AS evidence_name,
			e.status AS evidence_status,
			e.evidence_type,
			e.mime_type AS evidence_mime_type,
			e.file_size AS evidence_file_size,
			e.storage_provider AS evidence_storage_provider,
			d.name AS document_name,
			d.status AS document_status,
			d.document_type,
			d.file_url AS document_file_url
		FROM handover_checklist_items i
		LEFT JOIN evidence_records e ON e.id=i.evidence_id
		LEFT JOIN documents d ON d.id=i.document_id
		WHERE i.handover_id=$1
		ORDER BY CASE i.category WHEN 'quality' THEN 10 WHEN 'as_built' THEN 20 WHEN 'operation' THEN 30 WHEN 'warranty' THEN 40 WHEN 'finance' THEN 50 WHEN 'legal' THEN 60 ELSE 70 END, i.created_at
	`, id)
	if err != nil {
		return nil, err
	}
	evidenceOptions, err := r.Query(ctx, `
		SELECT id,name,evidence_type,status,work_item_id,created_at
		FROM evidence_records
		WHERE project_id=$1 AND status='accepted'
		ORDER BY created_at DESC LIMIT 100
	`, handover["project_id"])
	if err != nil {
		return nil, err
	}
	documentOptions, err := r.Query(ctx, `
		SELECT id,name,document_type,file_url,created_at
		FROM documents
		WHERE project_id=$1 AND status='active'
		ORDER BY created_at DESC LIMIT 100
	`, handover["project_id"])
	if err != nil {
		return nil, err
	}
	handover["checklist_items"] = items
	handover["evidence_options"] = evidenceOptions
	handover["document_options"] = documentOptions
	return handover, nil
}

func (r *Repository) ListProjectBudget(ctx context.Context, projectID string) ([]Record, error) {
	return r.Query(ctx, `
		SELECT *
		FROM accounting_budget_balances
		WHERE project_id=$1 AND status <> 'cancelled'
		ORDER BY created_at DESC
	`, projectID)
}

func (r *Repository) CreateBudget(ctx context.Context, body Record, actorID, actorName string) (Record, error) {
	return r.insertWithAudit(ctx, "accounting_budgets", budgetColumns(), body, func(row Record) auditEvent {
		return auditEvent{
			ProjectID: cleanID(row["project_id"]), ActorID: actorID, ActorName: actorName,
			EventType: "budget_created", TargetType: "budget",
			Description: fmt.Sprintf("Tạo ngân sách %s", row["budget_code"]),
			Metadata:    Record{"status": row["status"], "planned_amount": row["planned_amount"]},
		}
	})
}

func (r *Repository) ApproveBudget(ctx context.Context, id, actorID, actorName string) (Record, error) {
	return r.withAuditTransaction(ctx, func(tx *sql.Tx) (Record, error) {
		rows, err := queryTx(ctx, tx, `
			UPDATE accounting_budgets
			SET status='approved',
				approved_amount=planned_amount,
				remaining_amount=GREATEST(planned_amount-used_amount, 0),
				approved_by=NULLIF($2,'')::uuid,
				updated_at=now()
			WHERE id=$1 AND status IN ('draft','submitted')
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
			EventType: "budget_approved", TargetType: "budget",
			Description: fmt.Sprintf("Duyệt ngân sách %s", row["budget_code"]),
			Metadata:    Record{"status": row["status"], "approved_amount": row["approved_amount"]},
		}
	})
}

func (r *Repository) CreatePayment(ctx context.Context, body Record, actorID, actorName string) (Record, error) {
	return r.insertWithAudit(ctx, "payments", paymentColumns(), body, func(row Record) auditEvent {
		return auditEvent{
			ProjectID: cleanID(row["project_id"]), ActorID: actorID, ActorName: actorName,
			EventType: "payment_created", TargetType: "payment",
			Description: fmt.Sprintf("Tạo phiếu thanh toán %s", row["payment_code"]),
			Metadata:    Record{"status": row["status"], "payment_type": row["payment_type"], "amount": row["amount"]},
		}
	})
}

func (r *Repository) AcceptanceAction(ctx context.Context, id, result string, req ActionRequest, actorID, actorName string) (Record, error) {
	result = strings.TrimSpace(result)
	if !statusIn(result, "pass", "fail", "conditional_pass") {
		return nil, validationError("Kết quả nghiệm thu không hợp lệ", Record{"result": result})
	}
	event := "acceptance_passed"
	if result == "fail" {
		event = "acceptance_failed"
	}
	if result == "conditional_pass" {
		event = "acceptance_conditionally_passed"
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	rows, err := queryTx(ctx, tx, `SELECT * FROM acceptance_records WHERE id=$1 AND status <> 'deleted' FOR UPDATE`, id)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	current := rows[0]
	fromStatus := fmt.Sprint(current["status"])
	if statusIn(fromStatus, "completed", "cancelled") {
		return nil, validationError("Nghiệm thu đã được chốt hoặc không còn hợp lệ", Record{"status": fromStatus})
	}
	updated, err := queryTx(ctx, tx, `
		UPDATE acceptance_records
		SET status='completed', result=$2, approved_by=NULLIF($3,'')::uuid, updated_at=now()
		WHERE id=$1 AND status=$4
		RETURNING *
	`, id, result, actorID, fromStatus)
	if err != nil {
		return nil, err
	}
	if len(updated) == 0 {
		return nil, validationError("Trạng thái nghiệm thu đã thay đổi, vui lòng tải lại trước khi thao tác", Record{"expected_status": fromStatus})
	}
	acc := updated[0]
	projectID := fmt.Sprint(acc["project_id"])
	comment := actionComment(req)
	metadata := Record{"result": result, "from_status": fromStatus, "to_status": "completed"}
	if comment != "" {
		metadata["comment"] = comment
	}
	if result == "fail" {
		issueRows, err := queryTx(ctx, tx, `
			INSERT INTO issues
				(project_id, node_id, title, description, severity, status, reported_by, requires_rework, requires_replanning)
			VALUES ($1,$2,$3,$4,'high','open',NULLIF($5,'')::uuid,true,false)
			RETURNING *
		`, projectID, acc["node_id"], fmt.Sprintf("Punch list từ nghiệm thu: %s", acc["title"]), acc["notes"], actorID)
		if err != nil {
			return nil, err
		}
		if len(issueRows) > 0 {
			if err := addAuditTx(ctx, tx, projectID, actorID, actorName, "rework_requested", "issue", fmt.Sprint(issueRows[0]["id"]), "Nghiệm thu không đạt tạo danh sách lỗi/sửa lỗi", Record{"acceptance_id": id}); err != nil {
				return nil, err
			}
			metadata["issue_id"] = issueRows[0]["id"]
		}
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, event, "acceptance", id, fmt.Sprintf("Nghiệm thu %s: %s", result, acc["title"]), metadata); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return acc, nil
}

func (r *Repository) CreateReworkFromAcceptance(ctx context.Context, acceptanceID, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	accRows, err := queryTx(ctx, tx, `SELECT * FROM acceptance_records WHERE id=$1 AND status <> 'deleted' FOR UPDATE`, acceptanceID)
	if err != nil {
		return nil, err
	}
	if len(accRows) == 0 {
		return nil, sql.ErrNoRows
	}
	acc := accRows[0]
	if fmt.Sprint(acc["status"]) != "completed" || fmt.Sprint(acc["result"]) != "fail" {
		return nil, validationError("Chỉ tạo sửa lỗi từ nghiệm thu không đạt", Record{"status": acc["status"], "result": acc["result"]})
	}
	existing, err := queryTx(ctx, tx, `
		SELECT COUNT(*) AS total
		FROM rework_requests
		WHERE acceptance_id=$1 AND status NOT IN ('cancelled','done','deleted')
	`, acceptanceID)
	if err != nil {
		return nil, err
	}
	if len(existing) > 0 && number(existing[0]["total"]) > 0 {
		return nil, validationError("Nghiệm thu này đã có yêu cầu sửa lỗi đang mở", Record{"acceptance_id": acceptanceID})
	}
	reworkRows, err := queryTx(ctx, tx, `
		INSERT INTO rework_requests
			(project_id, acceptance_id, node_id, title, description, status, created_by, approved_by)
		VALUES ($1,$2,$3,$4,$5,'approved',NULLIF($6,'')::uuid,NULLIF($6,'')::uuid)
		RETURNING *
	`, acc["project_id"], acceptanceID, acc["node_id"], fmt.Sprintf("Sửa lỗi sau nghiệm thu: %s", acc["title"]), acc["notes"], actorID)
	if err != nil {
		return nil, err
	}
	if len(reworkRows) == 0 {
		return nil, sql.ErrNoRows
	}
	rework := reworkRows[0]
	if err := addAuditTx(ctx, tx, fmt.Sprint(acc["project_id"]), actorID, actorName, "rework_requested", "rework_request", fmt.Sprint(rework["id"]), "Tạo yêu cầu sửa lỗi từ nghiệm thu không đạt", Record{"acceptance_id": acceptanceID}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return rework, nil
}

func (r *Repository) SubmitHandover(ctx context.Context, id string, req ActionRequest, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := queryTx(ctx, tx, `SELECT * FROM handover_records WHERE id=$1 AND status <> 'deleted' FOR UPDATE`, id)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	handover := rows[0]
	if !statusIn(fmt.Sprint(handover["status"]), "draft", "blocked") {
		return nil, validationError("Hồ sơ bàn giao không ở trạng thái có thể gửi", Record{"status": handover["status"]})
	}
	if cleanID(handover["final_acceptance_id"]) == "" || strings.TrimSpace(fmt.Sprint(handover["client_representative"])) == "" || strings.TrimSpace(fmt.Sprint(handover["company_representative"])) == "" {
		return nil, validationError("Cần khóa nghiệm thu cuối và đại diện hai bên trước khi gửi bàn giao")
	}
	validAcceptance, err := queryTx(ctx, tx, `
		SELECT COUNT(*) AS total FROM acceptance_records
		WHERE id=$1 AND project_id=$2 AND type='final' AND status='completed' AND result IN ('pass','conditional_pass')
	`, handover["final_acceptance_id"], handover["project_id"])
	if err != nil {
		return nil, err
	}
	if len(validAcceptance) == 0 || number(validAcceptance[0]["total"]) == 0 {
		return nil, validationError("Nghiệm thu cuối được liên kết chưa đạt điều kiện bàn giao")
	}
	updated, err := queryTx(ctx, tx, `
		UPDATE handover_records SET status='submitted',submitted_by=NULLIF($2,'')::uuid,submitted_at=now(),updated_at=now()
		WHERE id=$1 RETURNING *
	`, id, actorID)
	if err != nil {
		return nil, err
	}
	comment := actionComment(req)
	if err := addAuditTx(ctx, tx, cleanID(handover["project_id"]), actorID, actorName, "handover_submitted", "handover", id, "Gửi hồ sơ bàn giao để kiểm tra", Record{"comment": comment, "final_acceptance_id": handover["final_acceptance_id"]}); err != nil {
		return nil, err
	}
	transition, err := r.startWorkItemForDomainMutationTx(ctx, tx, cleanID(handover["project_id"]), req.WorkItemContext, handoverWorkItemPolicy, "handover", id, actorID, actorName)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	result, err := r.GetHandoverDetail(ctx, cleanID(updated[0]["id"]))
	return attachWorkItemTransition(result, transition), err
}

func (r *Repository) UpdateHandoverChecklistItem(ctx context.Context, id string, body Record, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := queryTx(ctx, tx, `
		SELECT i.*,h.status AS handover_status FROM handover_checklist_items i
		JOIN handover_records h ON h.id=i.handover_id AND h.project_id=i.project_id
		WHERE i.id=$1 FOR UPDATE
	`, id)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	item := rows[0]
	if statusIn(fmt.Sprint(item["handover_status"]), "handed_over", "cancelled", "deleted") {
		return nil, validationError("Hồ sơ bàn giao đã khóa", Record{"status": item["handover_status"]})
	}
	if strings.TrimSpace(fmt.Sprint(item["source_type"])) == "system_check" || fmt.Sprint(item["item_code"]) == "final_acceptance" {
		return nil, validationError("Mục checklist này được xác minh tự động từ nguồn nghiệp vụ")
	}
	status := strings.TrimSpace(fmt.Sprint(body["status"]))
	if status == "waived" && item["required"] == true {
		return nil, validationError("Mục bắt buộc không thể miễn")
	}
	evidenceID := cleanID(body["evidence_id"])
	documentID := cleanID(body["document_id"])
	if status == "verified" && evidenceID == "" && documentID == "" {
		return nil, validationError("Mục đã xác minh cần evidence hoặc tài liệu")
	}
	if evidenceID != "" {
		valid, err := queryTx(ctx, tx, `SELECT COUNT(*) AS total FROM evidence_records WHERE id=$1 AND project_id=$2 AND status='accepted'`, evidenceID, item["project_id"])
		if err != nil || len(valid) == 0 || number(valid[0]["total"]) == 0 {
			return nil, validationError("Evidence chưa được chấp nhận hoặc không thuộc công trình", Record{"evidence_id": evidenceID})
		}
	}
	if documentID != "" {
		valid, err := queryTx(ctx, tx, `SELECT COUNT(*) AS total FROM documents WHERE id=$1 AND project_id=$2 AND status='active'`, documentID, item["project_id"])
		if err != nil || len(valid) == 0 || number(valid[0]["total"]) == 0 {
			return nil, validationError("Tài liệu không hoạt động hoặc không thuộc công trình", Record{"document_id": documentID})
		}
	}
	updated, err := queryTx(ctx, tx, `
		UPDATE handover_checklist_items
		SET status=$2,evidence_id=NULLIF($3,'')::uuid,document_id=NULLIF($4,'')::uuid,notes=NULLIF($5,''),
			verified_by=CASE WHEN $2 IN ('verified','waived') THEN NULLIF($6,'')::uuid ELSE NULL END,
			verified_at=CASE WHEN $2 IN ('verified','waived') THEN now() ELSE NULL END,updated_at=now()
		WHERE id=$1 RETURNING *
	`, id, status, evidenceID, documentID, body["notes"], actorID)
	if err != nil {
		return nil, err
	}
	if err := addAuditTx(ctx, tx, cleanID(item["project_id"]), actorID, actorName, "handover_checklist_updated", "handover_checklist_item", id, "Cập nhật checklist bàn giao", Record{"handover_id": item["handover_id"], "item_code": item["item_code"], "from_status": item["status"], "to_status": status, "evidence_id": evidenceID, "document_id": documentID}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return updated[0], nil
}

func (r *Repository) projectUsesSplitHandoverFinanceTx(ctx context.Context, tx *sql.Tx, projectID string) (bool, error) {
	rows, err := queryTx(ctx, tx, `
		SELECT EXISTS (
			SELECT 1
			FROM projects project
			JOIN pipeline_runs run ON run.id=project.current_pipeline_run_id
			JOIN pipeline_templates template ON template.id=run.template_id
			WHERE project.id=$1
			  AND (template.id=$2 OR template.template_family_id=$2)
			  AND template.revision_no>=8
		) AS enabled
	`, projectID, clientOfficialWorkflowTemplateID)
	if err != nil {
		return false, err
	}
	return len(rows) > 0 && boolValue(rows[0]["enabled"]), nil
}

func (r *Repository) HandoverCheck(ctx context.Context, id, actorID, actorName string) (Record, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := queryTx(ctx, tx, `SELECT * FROM handover_records WHERE id=$1 AND status <> 'deleted' FOR UPDATE`, id)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, sql.ErrNoRows
	}
	handover := rows[0]
	if !statusIn(fmt.Sprint(handover["status"]), "submitted", "ready", "blocked") {
		return nil, validationError("Cần gửi hồ sơ bàn giao trước khi kiểm tra readiness", Record{"status": handover["status"]})
	}
	projectID := cleanID(handover["project_id"])
	splitFinance, err := r.projectUsesSplitHandoverFinanceTx(ctx, tx, projectID)
	if err != nil {
		return nil, err
	}
	counts := Record{}
	checkCount := func(key, query string, args ...any) error {
		result, err := queryTx(ctx, tx, query, args...)
		if err != nil {
			return err
		}
		if len(result) > 0 {
			counts[key] = number(result[0]["total"])
			if amount, ok := result[0]["amount"]; ok {
				counts[key+"_amount"] = number(amount)
			}
		}
		return nil
	}
	if err := checkCount("final_acceptance", `SELECT COUNT(*) AS total FROM acceptance_records WHERE id=$1 AND project_id=$2 AND type='final' AND status='completed' AND result IN ('pass','conditional_pass')`, handover["final_acceptance_id"], projectID); err != nil {
		return nil, err
	}
	if err := checkCount("open_issues", `SELECT COUNT(*) AS total FROM issues WHERE project_id=$1 AND status IN ('open','in_progress')`, projectID); err != nil {
		return nil, err
	}
	if err := checkCount("open_ncrs", `SELECT COUNT(*) AS total FROM quality_ncrs WHERE project_id=$1 AND status <> 'closed'`, projectID); err != nil {
		return nil, err
	}
	if err := checkCount("open_reworks", `SELECT COUNT(*) AS total FROM rework_requests WHERE project_id=$1 AND status NOT IN ('done','completed','cancelled','deleted')`, projectID); err != nil {
		return nil, err
	}
	if err := checkCount("open_construction_actions", `SELECT COUNT(*) AS total FROM construction_action_items WHERE project_id=$1 AND status IN ('open','waiting_confirmation')`, projectID); err != nil {
		return nil, err
	}
	if err := checkCount("pending_surplus", `SELECT COUNT(*) AS total FROM site_material_balances WHERE project_id=$1 AND surplus_quantity > 0 AND surplus_status <> 'handled'`, projectID); err != nil {
		return nil, err
	}
	if err := checkCount("unconfirmed_material_costs", `SELECT COUNT(*) AS total,COALESCE(SUM(total_amount),0) AS amount FROM project_material_costs WHERE project_id=$1 AND status IN ('draft','submitted','needs_changes')`, projectID); err != nil {
		return nil, err
	}
	if err := checkCount("pending_cash_requests", `SELECT COUNT(*) AS total,COALESCE(SUM(amount),0) AS amount FROM project_cash_requests WHERE project_id=$1 AND status NOT IN ('processed','cancelled')`, projectID); err != nil {
		return nil, err
	}

	costsReady := number(counts["unconfirmed_material_costs"]) == 0
	cashRequestsReady := number(counts["pending_cash_requests"]) == 0
	systemStatuses := map[string]bool{
		"final_acceptance":       number(counts["final_acceptance"]) > 0,
		"warehouse_closure":      number(counts["pending_surplus"]) == 0,
		"project_material_costs": costsReady,
		"project_cash_requests":  cashRequestsReady,
		// Compatibility for handover packages created before the project-finance boundary changed.
		"ap_reconciliation": costsReady,
		"ar_reconciliation": cashRequestsReady,
	}
	for itemCode, passed := range systemStatuses {
		status := "pending"
		if passed {
			status = "verified"
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE handover_checklist_items SET status=$2,verified_by=CASE WHEN $2='verified' THEN NULLIF($3,'')::uuid ELSE NULL END,
				verified_at=CASE WHEN $2='verified' THEN now() ELSE NULL END,updated_at=now()
			WHERE handover_id=$1 AND item_code=$4
		`, id, status, actorID, itemCode); err != nil {
			return nil, err
		}
	}

	blockers := []string{}
	if number(counts["final_acceptance"]) == 0 {
		blockers = append(blockers, "Nghiệm thu cuối được liên kết chưa đạt")
	}
	if number(counts["open_issues"]) > 0 {
		blockers = append(blockers, "Còn danh sách lỗi/vấn đề chưa đóng")
	}
	if number(counts["open_ncrs"]) > 0 {
		blockers = append(blockers, "Còn NCR chưa đóng")
	}
	if number(counts["open_reworks"]) > 0 {
		blockers = append(blockers, "Còn rework chưa hoàn tất")
	}
	if number(counts["open_construction_actions"]) > 0 {
		blockers = append(blockers, "Còn việc cần xử lý tại công trường chưa hoàn tất")
	}
	if number(counts["pending_surplus"]) > 0 {
		blockers = append(blockers, "Còn vật tư dư chưa xử lý")
	}
	if !splitFinance && !costsReady {
		blockers = append(blockers, "Còn chi phí vật tư chưa được xác nhận")
	}
	if !splitFinance && !cashRequestsReady {
		blockers = append(blockers, "Còn đề nghị thu/chi nội bộ chưa được xử lý")
	}
	pendingItems, err := queryTx(ctx, tx, `
		SELECT item_code,title
		FROM handover_checklist_items
		WHERE handover_id=$1
		  AND required
		  AND status <> 'verified'
		  AND (NOT $2 OR item_code NOT IN ('project_material_costs','project_cash_requests','ap_reconciliation','ar_reconciliation'))
		ORDER BY created_at
	`, id, splitFinance)
	if err != nil {
		return nil, err
	}
	for _, item := range pendingItems {
		blockers = append(blockers, "Thiếu/xác minh chưa đạt: "+fmt.Sprint(item["title"]))
	}
	ready := len(blockers) == 0
	nextStatus := "blocked"
	if ready {
		nextStatus = "ready"
	}
	snapshot := Record{
		"ready": ready, "blockers": blockers, "counts": counts,
		"finance_separated": splitFinance,
		"checked_at":        time.Now().UTC().Format(time.RFC3339),
	}
	rawSnapshot, _ := json.Marshal(snapshot)
	updated, err := queryTx(ctx, tx, `
		UPDATE handover_records SET status=$2,documents_ready=$3,warehouse_closed=$4,payables_recorded=$5,receivables_recorded=$6,
			readiness_snapshot_json=$7::jsonb,checked_by=NULLIF($8,'')::uuid,checked_at=now(),updated_at=now()
		WHERE id=$1 RETURNING *
	`, id, nextStatus, len(pendingItems) == 0, systemStatuses["warehouse_closure"], costsReady, cashRequestsReady, string(rawSnapshot), actorID)
	if err != nil {
		return nil, err
	}
	event := "handover_ready"
	if !ready {
		event = "handover_blocked"
	}
	if err := addAuditTx(ctx, tx, projectID, actorID, actorName, event, "handover", id, "Kiểm tra readiness hồ sơ bàn giao", snapshot); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	result := updated[0]
	result["ready"] = ready
	result["blockers"] = blockers
	result["counts"] = counts
	return result, nil
}

func (r *Repository) CompleteHandover(ctx context.Context, id string, req ActionRequest, actorID, actorName string) (Record, error) {
	check, err := r.HandoverCheck(ctx, id, actorID, actorName)
	if err != nil {
		return nil, err
	}
	blockers, _ := check["blockers"].([]string)
	comment := actionComment(req)
	if len(blockers) > 0 {
		return check, nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	updated, err := queryTx(ctx, tx, `
		UPDATE handover_records
		SET status='handed_over',
			handover_date=COALESCE(handover_date, CURRENT_DATE),
			approved_by=NULLIF($2,'')::uuid,
			completed_at=now(),
			updated_at=now()
		WHERE id=$1 AND status='ready'
		RETURNING *
	`, id, actorID)
	if err != nil {
		return nil, err
	}
	if len(updated) == 0 {
		return nil, validationError("Bàn giao đã hoàn tất hoặc không còn hợp lệ")
	}
	metadata := Record{}
	if comment != "" {
		metadata["comment"] = comment
	}
	done := updated[0]
	if err := addAuditTx(ctx, tx, fmt.Sprint(done["project_id"]), actorID, actorName, "handover_completed", "handover", id, "Hoàn tất bàn giao công trình", metadata); err != nil {
		return nil, err
	}
	transition, err := r.startWorkItemForDomainMutationTx(ctx, tx, cleanID(done["project_id"]), req.WorkItemContext, handoverWorkItemPolicy, "handover", id, actorID, actorName)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return attachWorkItemTransition(done, transition), nil
}
