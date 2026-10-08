package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
)

type sampleDomainIDs struct {
	clientPartnerID, supplierPartnerID, materialID, warehouseID string
	boqItemID, quotationID, quotationItemID                     string
	procurementID, procurementItemID                            string
	incomingID, receiveDocumentID, issueDocumentID              string
	acceptanceID, contractID, contractVersionID                 string
	milestoneID                                                 string
}

// insertSampleDomainHistoryTx materializes the cross-domain records needed by a
// completed sample. Planning profiles keep their history in the runtime/forms;
// their unfinished business-domain actions remain for assignees to execute.
func insertSampleDomainHistoryTx(ctx context.Context, tx *sql.Tx, plan sampleDataPlan) error {
	if plan.Profile != "completed" {
		return nil
	}
	ids := sampleDomainIDs{
		clientPartnerID: newID(), supplierPartnerID: newID(), materialID: newID(), warehouseID: newID(),
		boqItemID: newID(), quotationID: newID(), quotationItemID: newID(),
		procurementID: newID(), procurementItemID: newID(), incomingID: newID(),
		receiveDocumentID: newID(), issueDocumentID: newID(), acceptanceID: newID(),
		contractID: newID(), contractVersionID: newID(), milestoneID: newID(),
	}
	if err := insertSampleCommercialHistoryTx(ctx, tx, plan, ids); err != nil {
		return fmt.Errorf("insert commercial history: %w", err)
	}
	if err := insertSampleConstructionQualityHistoryTx(ctx, tx, plan, ids); err != nil {
		return fmt.Errorf("insert construction and quality history: %w", err)
	}
	if err := insertSampleContractHandoffHistoryTx(ctx, tx, plan, ids); err != nil {
		return fmt.Errorf("insert contract and handover sources: %w", err)
	}
	return validateSampleCompletedSourcesTx(ctx, tx, plan)
}

func insertSampleCommercialHistoryTx(ctx context.Context, tx *sql.Tx, plan sampleDataPlan, ids sampleDomainIDs) error {
	purchaser := sampleActorForAction(plan, "source.supplier_quotation_ready")
	qs := sampleActorForAction(plan, "source.boq_release_ready")
	if purchaser.UserID == "" || qs.UserID == "" {
		return validationError("Pipeline mẫu hoàn thành thiếu tài khoản mẫu QS hoặc Mua hàng")
	}
	plan.ActorID, plan.ActorName = purchaser.UserID, purchaser.FullName
	suffix := sampleCodeSuffix(plan.ProjectID)
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO partners (id,partner_code,name,legal_name,partner_types,tax_code,address,contact_name,payment_terms_days,notes,active)
		VALUES
			($1,$2,$3,$3,'["client"]'::jsonb,$4,'TP.HCM','Đại diện Chủ đầu tư',30,'Đối tác dữ liệu mẫu',true),
			($5,$6,$7,$7,'["supplier"]'::jsonb,$8,'TP.HCM','Đại diện Nhà cung cấp',30,'Đối tác dữ liệu mẫu',true)
	`, ids.clientPartnerID, "DEMO-CL-"+suffix, "Chủ đầu tư mẫu "+suffix, "03"+suffix[:8],
		ids.supplierPartnerID, "DEMO-SP-"+suffix, "Nhà cung cấp mẫu "+suffix, "04"+suffix[:8]); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO materials (id,code,name,unit,category,default_supplier,standard_price,active,default_supplier_partner_id)
		VALUES ($1,$2,'Vật tư hoàn thiện dữ liệu mẫu','bộ','Hoàn thiện',$3,120000000,true,$4)
	`, ids.materialID, "DEMO-MAT-"+suffix, "Nhà cung cấp mẫu "+suffix, ids.supplierPartnerID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO warehouses (id,name,location,type,active)
		VALUES ($1,$2,'Công trình mẫu · TP.HCM','site',true)
	`, ids.warehouseID, "Kho công trình mẫu "+suffix); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO boq_items (
			id,project_id,category,work_item_name,description,unit,quantity,unit_price,status,
			created_by,approved_by,material_id,expected_unit_price,norm_unit_price,final_unit_price,
			unit_price_source,norm_price_locked,norm_price_locked_by,norm_price_locked_at,norm_price_comment,
			final_price_locked,final_price_locked_by,final_price_locked_at,final_price_comment,price_revision_no
		) VALUES ($1,$2,'Hoàn thiện','Thi công hoàn thiện mẫu','Khối lượng và đơn giá đã được phê duyệt','bộ',10,120000000,'approved',
			$3,$3,$4,125000000,122000000,120000000,'final',true,$3,now()-interval '120 days','Khóa định mức dữ liệu mẫu',
			true,$3,now()-interval '115 days','Chốt theo báo giá được lựa chọn',1)
	`, ids.boqItemID, plan.ProjectID, qs.UserID, ids.materialID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO supplier_quotations (
			id,project_id,quotation_code,supplier_partner_id,supplier_name,quotation_date,valid_until,currency,
			payment_terms_days,lead_time_days,delivery_terms,warranty_terms,status,technical_status,commercial_status,
			notes,submitted_by,submitted_at,qualified_by,qualified_at,selected_at,created_by
		) VALUES ($1,$2,$3,$4,$5,CURRENT_DATE-120,CURRENT_DATE-90,'VND',30,7,'Giao tại công trình','Bảo hành 12 tháng',
			'selected','compliant','compliant','Báo giá lịch sử dữ liệu mẫu',$6,now()-interval '119 days',$6,now()-interval '118 days',now()-interval '115 days',$6)
	`, ids.quotationID, plan.ProjectID, "DEMO-QT-"+suffix, ids.supplierPartnerID, "Nhà cung cấp mẫu "+suffix, plan.ActorID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO supplier_quotation_items (
			id,quotation_id,boq_item_id,material_id,description,unit,quantity,unit_price,
			discount_amount,tax_rate,lead_time_days,compliance_status
		) VALUES ($1,$2,$3,$4,'Vật tư hoàn thiện dữ liệu mẫu','bộ',10,120000000,0,10,7,'compliant')
	`, ids.quotationItemID, ids.quotationID, ids.boqItemID, ids.materialID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO supplier_quote_selections (project_id,boq_item_id,quotation_item_id,status,decision_reason,selected_by,selected_at)
		VALUES ($1,$2,$3,'active','Đáp ứng kỹ thuật, tiến độ và điều kiện thương mại',$4,now()-interval '115 days')
	`, plan.ProjectID, ids.boqItemID, ids.quotationItemID, plan.ActorID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE boq_items SET selected_quotation_item_id=$1 WHERE id=$2`, ids.quotationItemID, ids.boqItemID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO procurement_requests (
			id,project_id,request_code,title,requested_by,status,total_estimated_amount,budget_status,
			supplier_partner_id,supplier_name,order_reference,expected_delivery_date,ordered_at
		) VALUES ($1,$2,$3,'Mua vật tư hoàn thiện dữ liệu mẫu',$4,'received',1200000000,'within_budget',$5,$6,$7,CURRENT_DATE-90,now()-interval '110 days')
	`, ids.procurementID, plan.ProjectID, "DEMO-PR-"+suffix, plan.ActorID, ids.supplierPartnerID,
		"Nhà cung cấp mẫu "+suffix, "DEMO-PO-"+suffix); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO procurement_request_items (
			id,request_id,material_id,boq_item_id,quantity_planned,quantity_requested,unit,estimated_unit_price,
			supplier_partner_id,supplier_name,delivery_status,approval_status,boq_price_source,boq_price_revision_no,
			supplier_quotation_item_id,quoted_unit_price,price_variance_amount
		) VALUES ($1,$2,$3,$4,10,10,'bộ',120000000,$5,$6,'delivered','approved','final',1,$7,120000000,0)
	`, ids.procurementItemID, ids.procurementID, ids.materialID, ids.boqItemID, ids.supplierPartnerID,
		"Nhà cung cấp mẫu "+suffix, ids.quotationItemID)
	return err
}

func insertSampleConstructionQualityHistoryTx(ctx context.Context, tx *sql.Tx, plan sampleDataPlan, ids sampleDomainIDs) error {
	suffix := sampleCodeSuffix(plan.ProjectID)
	incomingStep := sampleCompletedStep(plan, "incoming_acceptance", "acceptance.incoming_material")
	constructionStep := sampleCompletedNodeStep(plan, "construction_execution")
	acceptanceStep := sampleCompletedNodeStep(plan, "outgoing_acceptance")
	if incomingStep == nil || constructionStep == nil || acceptanceStep == nil {
		return fmt.Errorf("sample blueprint is missing completed construction or acceptance steps")
	}
	incomingEvidence := sampleFirstEvidence(*incomingStep)
	incomingLineID := newID()
	acceptanceEvidence := sampleFirstEvidence(*acceptanceStep)
	if incomingEvidence == "" || acceptanceEvidence == "" {
		return fmt.Errorf("sample completed steps are missing evidence")
	}
	plan.ActorID, plan.ActorName = incomingStep.Actor.UserID, incomingStep.Actor.FullName
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO incoming_acceptance_records (
			id,acceptance_code,project_id,work_item_id,procurement_request_id,warehouse_id,source_type,
			supplier_name,delivery_reference,status,result,received_at,summary_json,comment,
			created_by,submitted_by,submitted_at,updated_by,supplier_partner_id
		) VALUES ($1,$2,$3,$4,$5,$6,'procurement',$7,$8,'submitted','pass',CURRENT_DATE-90,
			'{"delivery":"full","quality":"pass","simulated":true}'::jsonb,'Đủ số lượng, chất lượng và minh chứng',$9,$9,now()-interval '90 days',$9,$10)
	`, ids.incomingID, "DEMO-IA-"+suffix, plan.ProjectID, incomingStep.WorkItemID, ids.procurementID, ids.warehouseID,
		"Nhà cung cấp mẫu "+suffix, "DEMO-DN-"+suffix, plan.ActorID, ids.supplierPartnerID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO incoming_acceptance_items (
			incoming_acceptance_id,project_id,work_item_id,item_index,material_id,material_code,material_name,
			procurement_request_item_id,delivered_quantity,accepted_quantity,rejected_quantity,unit,condition,
			evidence_id,decision,comment,payload_json,id
		) VALUES ($1,$2,$3,0,$4,$5,'Vật tư hoàn thiện dữ liệu mẫu',$6,10,10,0,'bộ','usable',$7,'accepted',
			'Đã kiểm nhận đầy đủ','{"simulated":true}'::jsonb,$8)
	`, ids.incomingID, plan.ProjectID, incomingStep.WorkItemID, ids.materialID, "DEMO-MAT-"+suffix,
		ids.procurementItemID, incomingEvidence, incomingLineID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO stock_documents (id,document_code,project_id,movement_type,target_warehouse_id,reference_type,reference_id,document_date,note,status,created_by,posted_by,posted_at)
		VALUES ($1,$2,$3,'receive',$4,'incoming_acceptance',$5,CURRENT_DATE-90,'Nhập kho sau nghiệm thu đầu vào','posted',$6,$6,now()-interval '90 days')
	`, ids.receiveDocumentID, "DEMO-RCV-"+suffix, plan.ProjectID, ids.warehouseID, ids.incomingID, plan.ActorID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO stock_document_items (document_id,material_id,quantity,unit,reason,source_line_type,source_line_id)
		VALUES ($1,$2,10,'bộ','Nhập vật tư đạt nghiệm thu','incoming_acceptance_item',$3)
	`, ids.receiveDocumentID, ids.materialID, incomingLineID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO stock_movements (movement_code,project_id,warehouse_id,material_id,movement_type,quantity,unit,reason,reference_type,reference_id,created_by,approved_by,stock_document_id)
		VALUES ($1,$2,$3,$4,'receive',10,'bộ','Nhập kho dữ liệu mẫu','stock_document',$5,$6,$6,$5)
	`, "DEMO-MV-R-"+suffix, plan.ProjectID, ids.warehouseID, ids.materialID, ids.receiveDocumentID, plan.ActorID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO stock_documents (id,document_code,project_id,movement_type,source_warehouse_id,reference_type,reference_id,document_date,note,status,created_by,posted_by,posted_at)
		VALUES ($1,$2,$3,'issue',$4,'work_item',$5,CURRENT_DATE-60,'Xuất dùng cho thi công','posted',$6,$6,now()-interval '60 days')
	`, ids.issueDocumentID, "DEMO-ISS-"+suffix, plan.ProjectID, ids.warehouseID, constructionStep.WorkItemID, plan.ActorID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO stock_document_items (document_id,material_id,quantity,unit,reason,source_line_type,source_line_id)
		VALUES ($1,$2,10,'bộ','Xuất dùng hoàn tất công trình','work_item',$3)
	`, ids.issueDocumentID, ids.materialID, constructionStep.WorkItemID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO stock_movements (movement_code,project_id,warehouse_id,material_id,movement_type,quantity,unit,reason,reference_type,reference_id,created_by,approved_by,stock_document_id)
		VALUES ($1,$2,$3,$4,'issue',10,'bộ','Xuất thi công dữ liệu mẫu','stock_document',$5,$6,$6,$5)
	`, "DEMO-MV-I-"+suffix, plan.ProjectID, ids.warehouseID, ids.materialID, ids.issueDocumentID, plan.ActorID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO site_material_balances (project_id,material_id,planned_quantity,issued_quantity,used_quantity,surplus_quantity,surplus_status)
		VALUES ($1,$2,10,10,10,0,'closed')
	`, plan.ProjectID, ids.materialID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO operation_daily_reports (
			project_id,report_date,reported_by,weather,work_summary,manpower_summary,material_usage_summary,
			issues_summary,progress_percent,photos,workflow_form_instance_id,work_item_id,structured_json,
			primary_evidence_id,report_delivery_eligible
		) VALUES ($1,CURRENT_DATE-30,$2,'Nắng','Hoàn tất thi công theo kế hoạch','12 nhân sự','Đã sử dụng đủ vật tư',
			'Không có tồn đọng',100,'[]'::jsonb,NULLIF($3,'')::uuid,$4,'{"simulated":true,"safety":"pass"}'::jsonb,$5,true)
	`, plan.ProjectID, constructionStep.Actor.UserID, constructionStep.FormInstanceID, constructionStep.WorkItemID,
		sampleFirstEvidence(*constructionStep)); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO acceptance_records (
			id,project_id,node_id,type,title,requested_by,approved_by,status,result,notes,acceptance_code,
			work_item_id,form_instance_id,pdf_export_id,acceptance_date,location,client_representative,
			contractor_representative,scope_summary,signature_status,document_status,summary_json,
			submitted_by,submitted_at
		) VALUES ($1,$2,$3,'final','Nghiệm thu hoàn thành công trình mẫu',$4,$4,'completed','pass','Đạt yêu cầu bàn giao',$5,
			$6,NULLIF($7,'')::uuid,NULLIF($8,'')::uuid,CURRENT_DATE-10,'Công trình mẫu · TP.HCM','Đại diện Chủ đầu tư',$9,
			'Nghiệm thu toàn bộ phạm vi công trình','signed','linked','{"simulated":true,"quality":"pass"}'::jsonb,$4,now()-interval '10 days')
	`, ids.acceptanceID, plan.ProjectID, acceptanceStep.RunNodeID, acceptanceStep.Actor.UserID, "DEMO-AC-"+suffix,
		acceptanceStep.WorkItemID, acceptanceStep.FormInstanceID, sampleFormPDFID(*acceptanceStep), acceptanceStep.Actor.FullName); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO acceptance_checklist_items (
			acceptance_id,title,description,is_passed,note,photo_required,item_index,evidence_id,decision,severity,payload_json
		) VALUES ($1,'Kiểm tra hoàn thiện và hồ sơ','Đủ điều kiện nghiệm thu',true,'Đã kiểm tra',true,0,$2,'pass','low','{"simulated":true}'::jsonb)
	`, ids.acceptanceID, acceptanceEvidence); err != nil {
		return err
	}
	return nil
}

func insertSampleContractHandoffHistoryTx(ctx context.Context, tx *sql.Tx, plan sampleDataPlan, ids sampleDomainIDs) error {
	contractStep := sampleCompletedStep(plan, "contract", "contract.require_execution_ready")
	if contractStep == nil {
		return validationError("Pipeline mẫu hoàn thành thiếu gói hợp đồng chủ đầu tư")
	}
	plan.ActorID, plan.ActorName = contractStep.Actor.UserID, contractStep.Actor.FullName
	suffix := sampleCodeSuffix(plan.ProjectID)
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO contracts (
			id,project_id,contract_code,title,contract_type,direction,counterparty_partner_id,counterparty_name,
			currency,original_amount,current_amount,advance_percent,retention_percent,warranty_days,signed_date,
			effective_date,expiry_date,status,created_by,approved_by,signed_by,activated_by,activated_at
		) VALUES ($1,$2,$3,'Hợp đồng thi công công trình mẫu','main_client','receivable',$4,$5,'VND',5200000000,5200000000,20,5,365,
			CURRENT_DATE-150,CURRENT_DATE-145,CURRENT_DATE+220,'active',$6,$6,$6,$6,now()-interval '145 days')
	`, ids.contractID, plan.ProjectID, "DEMO-CT-"+suffix, ids.clientPartnerID, "Chủ đầu tư mẫu "+suffix, plan.ActorID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO contract_versions (
			id,contract_id,version_no,version_label,amount,scope_summary,commercial_terms_json,payment_terms_json,
			planned_start_date,planned_end_date,definition_hash,status,review_comment,signature_comment,
			submitted_by,submitted_at,approved_by,approved_at,signed_by,signed_at,created_by
		) VALUES ($1,$2,1,'Bản ký chính thức',5200000000,'Toàn bộ phạm vi công trình mẫu','{"currency":"VND","simulated":true}'::jsonb,
			'{"advance_percent":20,"retention_percent":5}'::jsonb,CURRENT_DATE-145,CURRENT_DATE-5,encode(digest($4 || ':sample-contract','sha256'),'hex'),
			'signed','Đã kiểm tra pháp lý và tài chính','Đã ký dữ liệu mẫu',$3,now()-interval '150 days',$3,now()-interval '148 days',$3,now()-interval '145 days',$3);
	`, ids.contractVersionID, ids.contractID, plan.ActorID, ids.contractID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE contracts SET current_version_id=$1 WHERE id=$2`, ids.contractVersionID, ids.contractID); err != nil {
		return err
	}
	if err := insertSampleCurrentWorkflowSourcesTx(ctx, tx, plan, ids); err != nil {
		return err
	}
	if err := insertSampleHandoverTx(ctx, tx, plan, ids); err != nil {
		return err
	}
	return insertSampleAccountingHandoffsTx(ctx, tx, plan, ids)
}

func insertSampleHandoverTx(ctx context.Context, tx *sql.Tx, plan sampleDataPlan, ids sampleDomainIDs) error {
	handoverStep := sampleCompletedStep(plan, "handover", "source.handover_completed")
	if handoverStep == nil || sampleFirstEvidence(*handoverStep) == "" {
		return fmt.Errorf("sample blueprint is missing completed handover evidence")
	}
	plan.ActorID, plan.ActorName = handoverStep.Actor.UserID, handoverStep.Actor.FullName
	counts, err := queryTx(ctx, tx, `
		SELECT
		(SELECT COUNT(*) FROM acceptance_records WHERE id=$2 AND project_id=$1 AND type='final' AND status='completed' AND result IN ('pass','conditional_pass')) AS final_acceptance,
		(SELECT COUNT(*) FROM issues WHERE project_id=$1 AND status IN ('open','in_progress')) AS open_issues,
		(SELECT COUNT(*) FROM quality_ncrs WHERE project_id=$1 AND status<>'closed') AS open_ncrs,
		(SELECT COUNT(*) FROM rework_requests WHERE project_id=$1 AND status NOT IN ('done','completed','cancelled','deleted')) AS open_reworks,
		(SELECT COUNT(*) FROM construction_action_items WHERE project_id=$1 AND status IN ('open','waiting_confirmation')) AS open_construction_actions,
		(SELECT COUNT(*) FROM site_material_balances WHERE project_id=$1 AND surplus_quantity>0 AND surplus_status<>'handled') AS pending_surplus,
		(SELECT COUNT(*) FROM evidence_records WHERE id=$3 AND project_id=$1 AND status='accepted') AS signed_handover_evidence
	`, plan.ProjectID, ids.acceptanceID, sampleFirstEvidence(*handoverStep))
	if err != nil {
		return err
	}
	if len(counts) != 1 || number(counts[0]["final_acceptance"]) == 0 || number(counts[0]["signed_handover_evidence"]) == 0 {
		return validationError("Nguồn nghiệm thu hoặc minh chứng bàn giao mẫu chưa hợp lệ")
	}
	for _, blocker := range []string{"open_issues", "open_ncrs", "open_reworks", "open_construction_actions", "pending_surplus"} {
		if number(counts[0][blocker]) != 0 {
			return validationError("Nguồn công trình mẫu còn chặn bàn giao", Record{"blocker": blocker, "count": counts[0][blocker]})
		}
	}
	readiness, _ := json.Marshal(Record{"ready": true, "blockers": []string{}, "counts": counts[0], "finance_separated": true, "simulated": true})
	handoverID := newID()
	suffix := sampleCodeSuffix(plan.ProjectID)
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO handover_records (
			id,project_id,status,final_acceptance_id,handover_date,client_representative,company_representative,
			documents_ready,warehouse_closed,notes,created_by,approved_by,
			handover_code,workflow_form_instance_id,readiness_snapshot_json,submitted_by,submitted_at,
			checked_by,checked_at,completed_at
		) VALUES ($1,$2,'handed_over',$3,CURRENT_DATE,$4,$5,true,true,'Bàn giao hoàn tất với đầy đủ hồ sơ mẫu',$6,$6,$7,
			NULLIF($8,'')::uuid,$9::jsonb,$6,now()-interval '2 days',$6,now(),now())
	`, handoverID, plan.ProjectID, ids.acceptanceID, "Đại diện Chủ đầu tư", plan.ActorName, plan.ActorID,
		"DEMO-HO-"+suffix, handoverStep.FormInstanceID, string(readiness)); err != nil {
		return err
	}
	items := []struct{ code, category, title, sourceType, sourceID string }{
		{"final_acceptance", "quality", "Nghiệm thu cuối đạt", "acceptance", ids.acceptanceID},
		{"warehouse_closure", "operation", "Kho công trình đã đóng", "stock_document", ids.issueDocumentID},
		{"signed_handover_minutes", "legal", "Biên bản bàn giao có minh chứng", "evidence", sampleFirstEvidence(*handoverStep)},
		{"as_built_documents", "as_built", "Bản vẽ và hồ sơ hoàn công", "evidence", sampleFirstEvidence(*handoverStep)},
		{"operation_manuals", "operation", "Hướng dẫn vận hành/bảo trì", "evidence", sampleFirstEvidence(*handoverStep)},
		{"warranty_documents", "warranty", "Cam kết và hồ sơ bảo hành", "evidence", sampleFirstEvidence(*handoverStep)},
	}
	for _, item := range items {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO handover_checklist_items (
				handover_id,project_id,item_code,category,title,required,status,source_type,source_id,notes,verified_by,verified_at
			) VALUES ($1,$2,$3,$4,$5,true,'verified',$6,$7,'Đã xác minh từ hồ sơ dữ liệu mẫu',$8,now())
		`, handoverID, plan.ProjectID, item.code, item.category, item.title, item.sourceType, item.sourceID, plan.ActorID); err != nil {
			return err
		}
	}
	return nil
}

func insertSampleCurrentWorkflowSourcesTx(ctx context.Context, tx *sql.Tx, plan sampleDataPlan, ids sampleDomainIDs) error {
	contractStep := sampleCompletedStep(plan, "contract", "contract.require_execution_ready")
	planningStep := sampleCompletedStep(plan, "workflow_form", "construction.activate_handoff")
	constructionStep := sampleCompletedNodeStep(plan, "construction_execution")
	subcontractorStep := sampleCompletedStep(plan, "subcontractor_reconciliation", "source.subcontractor_acceptance_package_ready")
	ownerStep := sampleCompletedStep(plan, "owner_client_reconciliation", "source.final_acceptance_package_ready")
	if contractStep == nil || contractStep.FormInstanceID == "" || len(contractStep.Evidence) == 0 ||
		planningStep == nil || planningStep.FormInstanceID == "" || constructionStep == nil ||
		subcontractorStep == nil || subcontractorStep.FormInstanceID == "" || ownerStep == nil || ownerStep.FormInstanceID == "" {
		return validationError("Pipeline thiếu binding nguồn nghiệp vụ để tạo hồ sơ mẫu hoàn thành")
	}
	designStep := sampleCompletedNodeStep(plan, "design")
	designHandoffStep := sampleCompletedStep(plan, "workflow_form", "source.design_documents_ready")
	if designStep == nil || designHandoffStep == nil || len(designStep.Evidence) == 0 {
		return validationError("Pipeline thiếu bản phát hành thiết kế hoặc bàn giao QS cho hồ sơ mẫu hoàn thành")
	}
	releaseID := newID()
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO design_releases (id,project_id,work_item_id,revision_no,revision_label,change_reason,created_by)
		VALUES ($1,$2,$3,1,'Mẫu R1','Gói thiết kế dữ liệu mẫu',$4)
	`, releaseID, plan.ProjectID, designStep.WorkItemID, designStep.Actor.UserID); err != nil {
		return err
	}
	for position, evidence := range designStep.Evidence {
		if _, err := tx.ExecContext(ctx, `INSERT INTO design_release_items (project_id,design_release_id,evidence_id,document_type,position) VALUES ($1,$2,$3,'construction_drawing',$4)`, plan.ProjectID, releaseID, evidence.ID, position); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE design_releases SET status='released',released_by=$2,released_at=now() WHERE id=$1`, releaseID, designStep.Actor.UserID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO design_release_handoffs (project_id,design_release_id,work_item_id,handed_off_by,comment) VALUES ($1,$2,$3,$4,'Bàn giao đúng revision dữ liệu mẫu cho QS')`, plan.ProjectID, releaseID, designHandoffStep.WorkItemID, designHandoffStep.Actor.UserID); err != nil {
		return err
	}
	// Bind the contract to the actual sample form signature and protected PDF,
	// rather than treating a signed status as execution readiness.
	var signatureID string
	if err := tx.QueryRowContext(ctx, `SELECT id::text FROM workflow_form_signatures WHERE form_instance_id=$1 AND purpose='approved_by' AND signer_user_id=$2 ORDER BY signed_at,id LIMIT 1`, contractStep.FormInstanceID, contractStep.Actor.UserID).Scan(&signatureID); err != nil {
		return fmt.Errorf("load sample contract signature: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO contract_version_signature_links (contract_version_id,workflow_form_signature_id,linked_by) VALUES ($1,$2,$3)`, ids.contractVersionID, signatureID, contractStep.Actor.UserID); err != nil {
		return err
	}
	scanID := newID()
	artifact := contractStep.Evidence[0]
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO documents (id,project_id,target_type,target_id,name,document_type,mime_type,file_size,storage_provider,storage_key,checksum_sha256,uploaded_by,released_by,released_at)
		VALUES ($1,$2,'contract',$3,'Bản scan hợp đồng giấy dữ liệu mẫu','signed_contract',$4,$5,$6,$7,$8,$9,$9,now())
	`, scanID, plan.ProjectID, ids.contractID, artifact.MIMEType, artifact.Size, artifact.Provider, artifact.Key, artifact.Checksum, contractStep.Actor.UserID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO contract_dispatches (contract_id,contract_version_id,dispatch_no,recipient_name,delivery_method,returned_scan_document_id,status,sent_by,received_by,received_at,note)
		VALUES ($1,$2,1,'Đại diện chủ đầu tư mẫu','direct',$3,'received',$4,$4,now(),'Hồ sơ giấy giả lập; không phải giao dịch tài chính')
	`, ids.contractID, ids.contractVersionID, scanID, contractStep.Actor.UserID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO boq_releases (project_id,release_no,item_count,total_amount,definition_hash,items_snapshot_json,released_by)
		SELECT $1,1,COUNT(*),COALESCE(SUM(quantity*final_unit_price),0),encode(digest(jsonb_agg(to_jsonb(item) ORDER BY item.id)::text,'sha256'),'hex'),jsonb_agg(to_jsonb(item) ORDER BY item.id),$2
		FROM boq_items item WHERE project_id=$1 AND status='approved'
	`, plan.ProjectID, sampleActorForAction(plan, "source.boq_release_ready").UserID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO contracts (project_id,contract_code,title,contract_type,direction,counterparty_partner_id,counterparty_name,currency,original_amount,current_amount,status,created_by,approved_by)
		VALUES ($1,$2,'Hợp đồng vật tư dữ liệu mẫu','supplier','payable',$3,$4,'VND',1200000000,1200000000,'approved',$5,$5)
	`, plan.ProjectID, "DEMO-INPUT-"+sampleCodeSuffix(plan.ProjectID), ids.supplierPartnerID, "Nhà cung cấp mẫu "+sampleCodeSuffix(plan.ProjectID), sampleActorForAction(plan, "source.input_contract_package_ready").UserID); err != nil {
		return err
	}
	handoffID := newID()
	snapshot, _ := json.Marshal(Record{"simulated": true, "source_form_instance_id": planningStep.FormInstanceID, "answers": planningStep.Answers})
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO construction_handoffs (id,project_id,pipeline_run_id,source_run_node_id,target_run_node_id,source_form_instance_id,source_form_revision_no,title,status,source_snapshot_hash,source_snapshot_jsonb,activated_by,activated_at,created_by)
		VALUES ($1,$2,$3,$4,$5,$6,1,'Bàn giao thi công dữ liệu mẫu','activated',encode(digest($7,'sha256'),'hex'),$7::jsonb,$8,now(),$8)
	`, handoffID, plan.ProjectID, plan.RunID, planningStep.RunNodeID, constructionStep.RunNodeID, planningStep.FormInstanceID, string(snapshot), planningStep.Actor.UserID); err != nil {
		return err
	}
	if len(plan.ConstructionChecks) == 0 {
		return validationError("Kế hoạch thi công mẫu chưa có hạng mục giám sát để bàn giao")
	}
	for _, check := range plan.ConstructionChecks {
		criteria, _ := json.Marshal(check.Source["criteria"])
		roles, _ := json.Marshal([]string{check.Actor.RoleName})
		itemSnapshot, _ := json.Marshal(check.Source)
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO construction_handoff_items (id,handoff_id,project_id,source_field_key,source_item_index,title,location,criteria_jsonb,scheduled_at,responsible_role_ids,assigned_user_id,evidence_required,source_snapshot_jsonb,status)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8::jsonb,NULLIF($9,'')::timestamptz,$10::jsonb,$11,true,$12::jsonb,'completed')
		`, check.ItemID, handoffID, plan.ProjectID, check.Source["source_field_key"], check.Source["source_item_index"], check.Source["title"], check.Source["location"], string(criteria), cleanOptionalText(check.Source["scheduled_at"]), string(roles), check.Actor.UserID, string(itemSnapshot)); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO work_items (id,project_id,pipeline_run_id,run_node_id,source_type,source_id,title,work_type,status,priority,role_ids,assignee_staff_id,evidence_requirement_json,created_by,updated_by,started_at,completed_at)
			VALUES ($1,$2,$3,$4,'construction_handoff_item',$5,$6,'checklist','completed','normal',$7::jsonb,$8,'{"required":true,"minimum":1,"types":["document"]}'::jsonb,$9,$10,now(),now())
		`, check.WorkItemID, plan.ProjectID, plan.RunID, constructionStep.RunNodeID, check.ItemID, check.Source["title"], string(roles), check.Actor.StaffID, planningStep.Actor.UserID, check.Actor.UserID); err != nil {
			return err
		}
		artifact := check.Evidence
		if artifact.ID == "" {
			return validationError("Hạng mục kiểm tra mẫu chưa có minh chứng kết quả")
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO evidence_records (id,work_item_id,project_id,target_type,target_id,evidence_type,document_type,name,title,mime_type,file_size,revision_no,visibility,status,uploaded_by,submitted_by,reviewed_by,reviewed_at,storage_provider,storage_key,checksum_sha256)
			VALUES ($1,$2,$3,'work_item',$2,'document','record',$4,$4,$5,$6,1,'internal','accepted',$7,$7,$7,now(),$8,$9,$10)
		`, artifact.ID, check.WorkItemID, plan.ProjectID, artifact.Name, artifact.MIMEType, artifact.Size, check.Actor.UserID, artifact.Provider, artifact.Key, artifact.Checksum); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO construction_checks (id,project_id,work_item_id,source_handoff_item_id,title,location,criteria_jsonb,status,result,evidence_id,requested_by,checked_by,checked_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7::jsonb,'completed','pass',$8,$9,$10,now())
		`, check.CheckID, plan.ProjectID, check.WorkItemID, check.ItemID, check.Source["title"], check.Source["location"], string(criteria), artifact.ID, planningStep.Actor.UserID, check.Actor.UserID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO construction_check_evidence (check_id,evidence_id,evidence_role,created_by) VALUES ($1,$2,'result',$3)`, check.CheckID, artifact.ID, check.Actor.UserID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE construction_handoff_items SET generated_work_item_id=$2,generated_check_id=$3 WHERE id=$1`, check.ItemID, check.WorkItemID, check.CheckID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO work_item_activities (work_item_id,project_id,scope_type,scope_id,actor_id,activity_type,to_status,comment,metadata_json) VALUES ($1,$2,'work_item',$1,$3,'completed','completed','Kiểm tra thi công mẫu đạt với minh chứng kết quả','{"simulated":true}'::jsonb)`, check.WorkItemID, plan.ProjectID, check.Actor.UserID); err != nil {
			return err
		}
		if err := addAuditTx(ctx, tx, plan.ProjectID, check.Actor.UserID, check.Actor.FullName, "construction_check_completed", "construction_check", check.CheckID, "Hoàn tất kiểm tra thi công mẫu", Record{"simulated": true, "source_handoff_item_id": check.ItemID, "result": "pass", "evidence_id": artifact.ID}); err != nil {
			return err
		}
	}
	if err := addAuditTx(ctx, tx, plan.ProjectID, planningStep.Actor.UserID, planningStep.Actor.FullName, "construction_handoff_activated", "construction_handoff", handoffID, "Kích hoạt bàn giao thi công mẫu", Record{"simulated": true, "source_form_instance_id": planningStep.FormInstanceID, "source_form_revision_no": 1, "item_count": len(plan.ConstructionChecks)}); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO subcontractor_checklist_reconciliations (project_id,work_item_id,form_instance_id,form_revision_no,status,subcontractor_name,work_package,summary_json,created_by,submitted_by,submitted_at,updated_by)
		VALUES ($1,$2,$3,1,'submitted','Thầu phụ mẫu','Hoàn thiện công trình','{"simulated":true,"accepted":1}'::jsonb,$4,$4,now(),$4)
	`, plan.ProjectID, subcontractorStep.WorkItemID, subcontractorStep.FormInstanceID, subcontractorStep.Actor.UserID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO subcontractor_checklist_reconciliation_items (reconciliation_id,project_id,work_item_id,form_instance_id,item_index,checklist_title,boq_item_id,claimed_quantity,confirmed_quantity,unit,status,evidence_id,decision,payload_json)
		SELECT id,$1,$2,$3,0,'Khối lượng hoàn thiện đã đối chiếu',$4,10,10,'bộ','done',$5,'accepted','{"simulated":true}'::jsonb FROM subcontractor_checklist_reconciliations WHERE work_item_id=$2
	`, plan.ProjectID, subcontractorStep.WorkItemID, subcontractorStep.FormInstanceID, ids.boqItemID, sampleFirstEvidence(*subcontractorStep)); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO owner_client_checklist_reconciliations (project_id,work_item_id,form_instance_id,form_revision_no,status,client_representative,company_representative,summary_json,created_by,submitted_by,submitted_at,updated_by)
		VALUES ($1,$2,$3,1,'submitted','Đại diện chủ đầu tư mẫu',$4,'{"simulated":true,"accepted":1}'::jsonb,$5,$5,now(),$5)
	`, plan.ProjectID, ownerStep.WorkItemID, ownerStep.FormInstanceID, ownerStep.Actor.FullName, ownerStep.Actor.UserID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO owner_client_checklist_reconciliation_items (reconciliation_id,project_id,work_item_id,form_instance_id,item_index,checklist_title,status,client_decision,evidence_id,acceptance_id,decision,payload_json)
		SELECT id,$1,$2,$3,0,'Hoàn thiện công trình','done','pass',$4,$5,'accepted','{"simulated":true}'::jsonb FROM owner_client_checklist_reconciliations WHERE work_item_id=$2
	`, plan.ProjectID, ownerStep.WorkItemID, ownerStep.FormInstanceID, sampleFirstEvidence(*ownerStep), ids.acceptanceID); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO contract_milestones (id,contract_id,contract_version_id,milestone_code,milestone_name,sequence_no,milestone_type,gross_amount,net_amount,acceptance_required,acceptance_id,status,eligible_by,eligible_at,created_by)
		VALUES ($1,$2,$3,'FINAL','Nghiệm thu hoàn thành',1,'final',5200000000,5200000000,true,$4,'eligible',$5,now(),$5)
	`, ids.milestoneID, ids.contractID, ids.contractVersionID, ids.acceptanceID, contractStep.Actor.UserID)
	return err
}

func sampleActorForAction(plan sampleDataPlan, action string) sampleDataStaffActor {
	for _, node := range plan.Nodes {
		for _, step := range node.Steps {
			if step.Selected && cleanOptionalText(step.Definition.Record["domain_action_key"]) == action {
				return step.Actor
			}
		}
	}
	return sampleDataStaffActor{}
}

func insertSampleAccountingHandoffsTx(ctx context.Context, tx *sql.Tx, plan sampleDataPlan, ids sampleDomainIDs) error {
	accountant := sampleActorForAction(plan, "source.accounting_handoff_close_ready")
	if accountant.UserID == "" {
		return validationError("Hồ sơ mẫu hoàn thành thiếu tài khoản mẫu Kế toán")
	}
	for _, source := range []struct{ kind, id string }{{"procurement_request", ids.procurementID}, {"contract_milestone", ids.milestoneID}} {
		sender := sampleActorForAction(plan, "source.supplier_quotation_ready")
		if source.kind == "contract_milestone" {
			step := sampleCompletedStep(plan, "contract", "contract.require_execution_ready")
			if step != nil {
				sender = step.Actor
			}
		}
		if sender.UserID == "" {
			return validationError("Thiếu tài khoản mẫu phụ trách hồ sơ nguồn bàn giao")
		}
		candidate, err := accountingHandoffSourceTx(ctx, tx, plan.ProjectID, source.kind, source.id)
		if err != nil {
			return err
		}
		if !boolValue(candidate["ready"]) {
			return validationError("Nguồn mẫu chưa đủ điều kiện bàn giao kế toán", Record{"source_type": source.kind, "source_id": source.id})
		}
		snapshot, err := json.Marshal(candidate["snapshot"])
		if err != nil {
			return err
		}
		handoffID := newID()
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO accounting_handoffs (id,project_id,direction,source_type,source_id,source_snapshot_json,status,note,handed_off_by,acknowledged_by,acknowledged_at)
			VALUES ($1,$2,$3,$4,$5,$6::jsonb,'acknowledged','Hồ sơ dữ liệu mẫu đã tiếp nhận; không xác nhận thanh toán hoặc hạch toán',$7,$8,now())
		`, handoffID, plan.ProjectID, candidate["direction"], source.kind, source.id, string(snapshot), sender.UserID, accountant.UserID); err != nil {
			return err
		}
		if err := addAuditTx(ctx, tx, plan.ProjectID, sender.UserID, sender.FullName, "accounting_handoff_created", "accounting_handoff", handoffID, "Bàn giao hồ sơ dữ liệu mẫu", Record{"simulated": true, "source_type": source.kind, "source_id": source.id}); err != nil {
			return err
		}
		if err := addAuditTx(ctx, tx, plan.ProjectID, accountant.UserID, accountant.FullName, "accounting_handoff_acknowledged", "accounting_handoff", handoffID, "Tiếp nhận hồ sơ dữ liệu mẫu", Record{"simulated": true}); err != nil {
			return err
		}
	}
	return nil
}

// These predicates mirror the current Rev9 source gates. Unknown bindings fail
// closed; adding a template action requires its source generator and predicate.
func sampleCompletedGateQuery(action string) string {
	var predicate string
	switch action {
	case "source.design_documents_ready":
		predicate = `EXISTS (SELECT 1 FROM design_releases release WHERE release.project_id=$1 AND release.status='released' AND release.id=(SELECT design_release_id FROM design_release_handoffs WHERE project_id=$1 ORDER BY handed_off_at DESC,id DESC LIMIT 1))`
	case "procurement.generate_from_boq":
		predicate = `EXISTS (SELECT 1 FROM boq_items WHERE project_id=$1 AND status<>'deleted') AND NOT EXISTS (SELECT 1 FROM boq_items item WHERE project_id=$1 AND status<>'deleted' AND (status<>'approved' OR NOT final_price_locked OR NOT EXISTS (SELECT 1 FROM procurement_request_items line JOIN procurement_requests request ON request.id=line.request_id WHERE line.boq_item_id=item.id AND request.project_id=$1 AND request.status<>'cancelled')))`
	case "source.boq_release_ready":
		predicate = `EXISTS (SELECT 1 FROM boq_releases WHERE project_id=$1 AND status='released')`
	case "source.supplier_quotation_ready":
		predicate = `EXISTS (SELECT 1 FROM supplier_quotations WHERE project_id=$1 AND status IN ('qualified','selected'))`
	case "source.supplier_selection_ready":
		predicate = `EXISTS (SELECT 1 FROM supplier_quote_selections WHERE project_id=$1 AND status='active')`
	case "source.input_contract_package_ready":
		predicate = `EXISTS (SELECT 1 FROM procurement_requests WHERE project_id=$1 AND status IN ('approved','ordered','partially_received','received')) AND EXISTS (SELECT 1 FROM contracts WHERE project_id=$1 AND direction='payable' AND contract_type IN ('supplier','subcontractor','service') AND status IN ('approved','signed','active'))`
	case "contract.require_execution_ready":
		predicate = `EXISTS (SELECT 1 FROM contracts contract JOIN contract_versions version ON version.id=contract.current_version_id WHERE contract.project_id=$1 AND contract.contract_type='main_client' AND contract.direction='receivable' AND contract.status='active' AND version.status='signed' AND EXISTS (SELECT 1 FROM contract_version_signature_links link JOIN workflow_form_signatures signature ON signature.id=link.workflow_form_signature_id WHERE link.contract_version_id=version.id AND signature.purpose='approved_by' AND signature.project_id=$1) AND EXISTS (SELECT 1 FROM contract_dispatches dispatch JOIN documents scan ON scan.id=dispatch.returned_scan_document_id WHERE dispatch.contract_id=contract.id AND dispatch.contract_version_id=version.id AND dispatch.status='received' AND scan.project_id=$1 AND scan.status='active'))`
	case "construction.activate_handoff":
		predicate = `EXISTS (SELECT 1 FROM construction_handoffs WHERE project_id=$1 AND status='activated')`
	case "source.subcontractor_acceptance_package_ready":
		predicate = `EXISTS (SELECT 1 FROM subcontractor_checklist_reconciliations WHERE project_id=$1 AND status='submitted')`
	case "acceptance.incoming_material":
		predicate = `EXISTS (SELECT 1 FROM incoming_acceptance_records WHERE project_id=$1 AND status='submitted')`
	case "source.final_acceptance_package_ready":
		predicate = `EXISTS (SELECT 1 FROM owner_client_checklist_reconciliations WHERE project_id=$1 AND status='submitted') AND EXISTS (SELECT 1 FROM acceptance_records WHERE project_id=$1 AND type='final' AND status='completed' AND result IN ('pass','conditional_pass'))`
	case "source.handover_completed":
		predicate = `EXISTS (SELECT 1 FROM handover_records WHERE project_id=$1 AND status='handed_over' AND completed_at IS NOT NULL) AND NOT EXISTS (SELECT 1 FROM issues WHERE project_id=$1 AND status IN ('open','in_progress')) AND NOT EXISTS (SELECT 1 FROM quality_ncrs WHERE project_id=$1 AND status<>'closed') AND NOT EXISTS (SELECT 1 FROM rework_requests WHERE project_id=$1 AND status NOT IN ('done','completed','cancelled','deleted')) AND NOT EXISTS (SELECT 1 FROM construction_action_items WHERE project_id=$1 AND status IN ('open','waiting_confirmation')) AND NOT EXISTS (SELECT 1 FROM site_material_balances WHERE project_id=$1 AND surplus_quantity>0 AND surplus_status<>'handled') AND NOT EXISTS (SELECT 1 FROM handover_checklist_items item JOIN handover_records handover ON handover.id=item.handover_id WHERE handover.project_id=$1 AND item.required AND item.status<>'verified')`
	case "source.accounting_handoff_close_ready":
		predicate = `EXISTS (SELECT 1 FROM handover_records WHERE project_id=$1 AND status='handed_over' AND completed_at IS NOT NULL) AND NOT EXISTS (SELECT 1 FROM procurement_requests request WHERE request.project_id=$1 AND request.status IN ('approved','ordered','partially_received','received') AND NOT EXISTS (SELECT 1 FROM accounting_handoffs handoff WHERE handoff.project_id=$1 AND handoff.source_type='procurement_request' AND handoff.source_id=request.id AND handoff.status='acknowledged')) AND NOT EXISTS (SELECT 1 FROM contract_milestones milestone JOIN contracts contract ON contract.id=milestone.contract_id WHERE contract.project_id=$1 AND contract.direction='receivable' AND contract.contract_type='main_client' AND contract.status IN ('active','completed') AND milestone.status IN ('active','eligible') AND NOT EXISTS (SELECT 1 FROM accounting_handoffs handoff WHERE handoff.project_id=$1 AND handoff.source_type='contract_milestone' AND handoff.source_id=milestone.id AND handoff.status='acknowledged'))`
	default:
		return ""
	}
	return "SELECT " + predicate
}

func validateSampleCompletedSourcesTx(ctx context.Context, tx *sql.Tx, plan sampleDataPlan) error {
	for _, node := range plan.Nodes {
		for _, step := range node.Steps {
			if !step.Selected || !step.Completed {
				continue
			}
			action := cleanOptionalText(step.Definition.Record["domain_action_key"])
			if action == "" {
				continue
			}
			query := sampleCompletedGateQuery(action)
			if query == "" {
				return validationError("Chưa có bộ kiểm chứng điều kiện nguồn cho Pipeline mẫu", Record{"domain_action_key": action})
			}
			var ready bool
			if err := tx.QueryRowContext(ctx, query, plan.ProjectID).Scan(&ready); err != nil {
				return fmt.Errorf("validate sample source gate %s: %w", action, err)
			}
			if !ready {
				return validationError("Nguồn nghiệp vụ mẫu chưa vượt qua điều kiện hoàn thành", Record{"domain_action_key": action, "work_item_id": step.WorkItemID})
			}
			if err := addAuditTx(ctx, tx, plan.ProjectID, step.Actor.UserID, step.Actor.FullName, "sample_domain_source_validated", "work_item", step.WorkItemID, "Kiểm chứng nguồn nghiệp vụ dữ liệu mẫu", Record{"simulated": true, "domain_action_key": action}); err != nil {
				return err
			}
		}
	}
	return nil
}

func sampleCompletedStep(plan sampleDataPlan, surfaceKind, actionKey string) *sampleDataPlannedStep {
	for nodeIndex := range plan.Nodes {
		for stepIndex := range plan.Nodes[nodeIndex].Steps {
			step := &plan.Nodes[nodeIndex].Steps[stepIndex]
			if !step.Completed {
				continue
			}
			surface := strings.TrimSpace(fmt.Sprint(step.Definition.Record["surface_kind"]))
			action := strings.TrimSpace(fmt.Sprint(step.Definition.Record["domain_action_key"]))
			if (surfaceKind == "" || surface == surfaceKind) && (actionKey == "" || action == actionKey) {
				return step
			}
		}
	}
	return nil
}

func sampleCompletedNodeStep(plan sampleDataPlan, nodeBusinessCode string) *sampleDataPlannedStep {
	for nodeIndex := range plan.Nodes {
		node := &plan.Nodes[nodeIndex]
		if strings.TrimSpace(fmt.Sprint(node.Definition.Record["business_code"])) != nodeBusinessCode {
			continue
		}
		for stepIndex := range node.Steps {
			if node.Steps[stepIndex].Completed {
				return &node.Steps[stepIndex]
			}
		}
	}
	return nil
}

func sampleFirstEvidence(step sampleDataPlannedStep) string {
	if len(step.Evidence) == 0 {
		return ""
	}
	return step.Evidence[0].ID
}

func sampleFormPDFID(step sampleDataPlannedStep) string {
	if step.FormPDF == nil {
		return ""
	}
	return step.FormPDF.ID
}

func sampleCodeSuffix(projectID string) string {
	value := strings.ToUpper(strings.ReplaceAll(projectID, "-", ""))
	if len(value) > 10 {
		value = value[:10]
	}
	return value
}
