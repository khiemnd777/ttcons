package core

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

type sampleDomainIDs struct {
	clientPartnerID, supplierPartnerID, materialID, warehouseID string
	boqItemID, quotationID, quotationItemID                     string
	procurementID, procurementItemID                            string
	incomingID, receiveDocumentID, issueDocumentID              string
	acceptanceID, contractID, contractVersionID                 string
	payableID, receivableID, payablePaymentID, receiptPaymentID string
	payableScheduleID, receivableScheduleID                     string
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
		contractID: newID(), contractVersionID: newID(), payableID: newID(), receivableID: newID(),
		payablePaymentID: newID(), receiptPaymentID: newID(), payableScheduleID: newID(), receivableScheduleID: newID(),
	}
	if err := insertSampleCommercialHistoryTx(ctx, tx, plan, ids); err != nil {
		return fmt.Errorf("insert commercial history: %w", err)
	}
	if err := insertSampleConstructionQualityHistoryTx(ctx, tx, plan, ids); err != nil {
		return fmt.Errorf("insert construction and quality history: %w", err)
	}
	if err := insertSampleFinanceHistoryTx(ctx, tx, plan, ids); err != nil {
		return fmt.Errorf("insert finance and handover history: %w", err)
	}
	return nil
}

func insertSampleCommercialHistoryTx(ctx context.Context, tx *sql.Tx, plan sampleDataPlan, ids sampleDomainIDs) error {
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
	`, ids.boqItemID, plan.ProjectID, plan.ActorID, ids.materialID); err != nil {
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
	acceptanceEvidence := sampleFirstEvidence(*acceptanceStep)
	if incomingEvidence == "" || acceptanceEvidence == "" {
		return fmt.Errorf("sample completed steps are missing evidence")
	}
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
			evidence_id,decision,comment,payload_json
		) VALUES ($1,$2,$3,0,$4,$5,'Vật tư hoàn thiện dữ liệu mẫu',$6,10,10,0,'bộ','usable',$7,'accepted',
			'Đã kiểm nhận đầy đủ','{"simulated":true}'::jsonb)
	`, ids.incomingID, plan.ProjectID, incomingStep.WorkItemID, ids.materialID, "DEMO-MAT-"+suffix,
		ids.procurementItemID, incomingEvidence); err != nil {
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
	`, ids.receiveDocumentID, ids.materialID, ids.procurementItemID); err != nil {
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
	`, plan.ProjectID, plan.ActorID, constructionStep.FormInstanceID, constructionStep.WorkItemID,
		sampleFirstEvidence(*constructionStep)); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO acceptance_records (
			id,project_id,node_id,type,title,requested_by,approved_by,status,result,notes,acceptance_code,
			work_item_id,form_instance_id,pdf_export_id,acceptance_date,location,client_representative,
			contractor_representative,scope_summary,signature_status,document_status,summary_json,
			submitted_by,submitted_at
		) VALUES ($1,$2,$3,'final','Nghiệm thu hoàn thành công trình mẫu',$4,$4,'approved','pass','Đạt yêu cầu bàn giao',$5,
			$6,NULLIF($7,'')::uuid,NULLIF($8,'')::uuid,CURRENT_DATE-10,'Công trình mẫu · TP.HCM','Đại diện Chủ đầu tư',$9,
			'Nghiệm thu toàn bộ phạm vi công trình','signed','linked','{"simulated":true,"quality":"pass"}'::jsonb,$4,now()-interval '10 days')
	`, ids.acceptanceID, plan.ProjectID, acceptanceStep.RunNodeID, plan.ActorID, "DEMO-AC-"+suffix,
		acceptanceStep.WorkItemID, acceptanceStep.FormInstanceID, sampleFormPDFID(*acceptanceStep), plan.ActorName); err != nil {
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

func insertSampleFinanceHistoryTx(ctx context.Context, tx *sql.Tx, plan sampleDataPlan, ids sampleDomainIDs) error {
	suffix := sampleCodeSuffix(plan.ProjectID)
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO contracts (
			id,project_id,contract_code,title,contract_type,direction,counterparty_partner_id,counterparty_name,
			currency,original_amount,current_amount,advance_percent,retention_percent,warranty_days,signed_date,
			effective_date,expiry_date,status,created_by,approved_by,signed_by,activated_by,activated_at
		) VALUES ($1,$2,$3,'Hợp đồng thi công công trình mẫu','main_client','receivable',$4,$5,'VND',5200000000,5200000000,20,5,365,
			CURRENT_DATE-150,CURRENT_DATE-145,CURRENT_DATE+220,'completed',$6,$6,$6,$6,now()-interval '145 days')
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
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO project_material_costs (
			project_id,procurement_request_id,incoming_acceptance_id,stock_document_id,vendor_partner_id,vendor_name,
			invoice_number,invoice_date,description,amount_before_vat,vat_amount,total_amount,recognized_cost_amount,
			notes,status,created_by,submitted_by,submitted_at,reviewed_by,reviewed_at,review_comment
		) VALUES ($1,$2,$3,$4,$5,$6,$7,CURRENT_DATE-80,'Chi phí vật tư đã nghiệm thu',1090909091,109090909,1200000000,1200000000,
			'Đã đối chiếu mua hàng, nghiệm thu và nhập kho','confirmed',$8,$8,now()-interval '80 days',$8,now()-interval '79 days','Đủ điều kiện lên báo cáo quản trị')
	`, plan.ProjectID, ids.procurementID, ids.incomingID, ids.receiveDocumentID, ids.supplierPartnerID,
		"Nhà cung cấp mẫu "+suffix, "DEMO-INV-"+suffix, plan.ActorID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO project_cash_requests (
			project_id,request_code,direction,title,counterparty_name,amount,due_date,source_type,source_id,notes,status,
			created_by,submitted_by,submitted_at,approved_by,approved_at,handed_to_accounting_by,handed_to_accounting_at,processed_by,processed_at
		) VALUES
			($1,$2,'outbound','Đề nghị chi nhà cung cấp',$3,1200000000,CURRENT_DATE-70,'procurement_request',$4,'Hồ sơ chi đầy đủ','processed',$5,$5,now()-interval '75 days',$5,now()-interval '74 days',$5,now()-interval '73 days',$5,now()-interval '70 days'),
			($1,$6,'inbound','Đề nghị thu Chủ đầu tư',$7,5200000000,CURRENT_DATE-5,'acceptance',$8,'Hồ sơ thu đủ nghiệm thu','processed',$5,$5,now()-interval '9 days',$5,now()-interval '8 days',$5,now()-interval '7 days',$5,now()-interval '5 days')
	`, plan.ProjectID, "DEMO-PC-OUT-"+suffix, "Nhà cung cấp mẫu "+suffix, ids.procurementID, plan.ActorID,
		"DEMO-PC-IN-"+suffix, "Chủ đầu tư mẫu "+suffix, ids.acceptanceID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO payables (
			id,project_id,accounting_book_id,cost_account_id,payable_code,vendor_type,vendor_partner_id,vendor_name,
			source_type,source_id,gross_amount,advance_amount,retention_amount,deduction_amount,paid_amount,remaining_amount,due_date,status,created_by,approved_by
		) VALUES ($1,$2,$3,$4,$5,'supplier',$6,$7,'procurement_request',$8,1200000000,0,0,0,1200000000,0,CURRENT_DATE-70,'paid',$9,$9)
	`, ids.payableID, plan.ProjectID, plan.AccountingBookID, plan.CostAccountID, "DEMO-PAY-"+suffix,
		ids.supplierPartnerID, "Nhà cung cấp mẫu "+suffix, ids.procurementID, plan.ActorID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO receivables (
			id,project_id,accounting_book_id,revenue_account_id,receivable_code,client_partner_id,client_name,source_type,source_id,
			contract_amount,milestone_name,gross_amount,received_amount,remaining_amount,due_date,status,created_by,approved_by
		) VALUES ($1,$2,$3,$4,$5,$6,$7,'acceptance',$8,5200000000,'Nghiệm thu và quyết toán',5200000000,5200000000,0,CURRENT_DATE-5,'received',$9,$9)
	`, ids.receivableID, plan.ProjectID, plan.AccountingBookID, plan.RevenueAccountID, "DEMO-REC-"+suffix,
		ids.clientPartnerID, "Chủ đầu tư mẫu "+suffix, ids.acceptanceID, plan.ActorID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO payable_payment_schedules (
			id,project_id,payable_id,schedule_code,milestone_name,schedule_type,planned_amount,paid_amount,remaining_amount,due_date,status,notes,created_by,approved_by
		) VALUES ($1,$2,$3,$4,'Thanh toán sau nghiệm thu đầu vào','final',1200000000,1200000000,0,CURRENT_DATE-70,'paid','Đã thanh toán và đối soát',$5,$5)
	`, ids.payableScheduleID, plan.ProjectID, ids.payableID, "DEMO-SCH-P-"+suffix, plan.ActorID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO receivable_payment_schedules (
			id,project_id,receivable_id,schedule_code,milestone_name,schedule_type,planned_amount,received_amount,remaining_amount,due_date,status,notes,created_by,approved_by
		) VALUES ($1,$2,$3,$4,'Quyết toán sau nghiệm thu','final',5200000000,5200000000,0,CURRENT_DATE-5,'received','Đã thu và đối soát',$5,$5)
	`, ids.receivableScheduleID, plan.ProjectID, ids.receivableID, "DEMO-SCH-R-"+suffix, plan.ActorID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO payments (
			id,project_id,accounting_book_id,cash_account_id,payment_code,payment_type,target_type,target_id,amount,payment_method,status,paid_at,created_by,approved_by
		) VALUES ($1,$2,$3,$4,$5,'supplier_payment','payable',$6,1200000000,'bank_transfer','approved',now()-interval '70 days',$7,$7)
	`, ids.payablePaymentID, plan.ProjectID, plan.AccountingBookID, plan.BankAccountID, "DEMO-PMT-"+suffix,
		ids.payableID, plan.ActorID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO payments (
			id,project_id,accounting_book_id,cash_account_id,payment_code,payment_type,target_type,target_id,amount,payment_method,status,paid_at,created_by,approved_by
		) VALUES ($1,$2,$3,$4,$5,'client_receipt','receivable',$6,5200000000,'bank_transfer','approved',now()-interval '5 days',$7,$7)
	`, ids.receiptPaymentID, plan.ProjectID, plan.AccountingBookID, plan.BankAccountID, "DEMO-RCV-"+suffix,
		ids.receivableID, plan.ActorID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO payment_allocations (project_id,payment_id,payable_id,schedule_id,allocation_type,amount,notes,created_by)
		VALUES ($1,$2,$3,$4,'payable_payment',1200000000,'Đối soát đủ nghĩa vụ dữ liệu mẫu',$5)
	`, plan.ProjectID, ids.payablePaymentID, ids.payableID, ids.payableScheduleID, plan.ActorID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO receipt_allocations (project_id,payment_id,receivable_id,schedule_id,allocation_type,amount,notes,created_by)
		VALUES ($1,$2,$3,$4,'receivable_receipt',5200000000,'Đối soát đủ khoản thu dữ liệu mẫu',$5)
	`, plan.ProjectID, ids.receiptPaymentID, ids.receivableID, ids.receivableScheduleID, plan.ActorID); err != nil {
		return err
	}
	if err := insertSampleJournalTx(ctx, tx, plan, ids.payablePaymentID, "DEMO-JE-COST-"+suffix,
		"Ghi nhận chi phí và thanh toán dữ liệu mẫu", plan.CostAccountID, plan.BankAccountID, 1200000000); err != nil {
		return err
	}
	if err := insertSampleJournalTx(ctx, tx, plan, ids.receiptPaymentID, "DEMO-JE-REV-"+suffix,
		"Ghi nhận doanh thu và thu tiền dữ liệu mẫu", plan.BankAccountID, plan.RevenueAccountID, 5200000000); err != nil {
		return err
	}
	return insertSampleHandoverTx(ctx, tx, plan, ids)
}

func insertSampleJournalTx(ctx context.Context, tx *sql.Tx, plan sampleDataPlan, sourceID, entryNo, description, debitAccountID, creditAccountID string, amount int64) error {
	entryID := newID()
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO journal_entries (
			id,book_id,project_id,period_id,entry_no,entry_date,document_date,description,source_type,source_id,
			source_event,status,created_by,posted_by,posted_at,source_document_no
		) SELECT $1,$2,$3,period.id,$4,CURRENT_DATE,CURRENT_DATE,$5,'sample_data',$6,'sample_project_completed',
			'draft',$7,NULL,NULL,$4
		FROM accounting_periods period
		WHERE period.book_id=$2 AND CURRENT_DATE BETWEEN period.date_from AND period.date_to AND period.status='open'
		ORDER BY period.created_at DESC LIMIT 1
	`, entryID, plan.AccountingBookID, plan.ProjectID, entryNo, description, sourceID, plan.ActorID); err != nil {
		return err
	}
	var exists bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM journal_entries WHERE id=$1)`, entryID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return validationError("Không có kỳ kế toán đang mở để tạo lịch sử tài chính mẫu", Record{"entry_no": entryNo})
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO journal_entry_lines (entry_id,book_id,line_no,account_id,description,debit_amount,credit_amount)
		VALUES ($1,$2,1,$3,$5,$6,0),($1,$2,2,$4,$5,0,$6)
	`, entryID, plan.AccountingBookID, debitAccountID, creditAccountID, description, amount); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `UPDATE journal_entries SET status='posted',posted_by=$1,posted_at=now(),updated_at=now() WHERE id=$2`, plan.ActorID, entryID)
	return err
}

func insertSampleHandoverTx(ctx context.Context, tx *sql.Tx, plan sampleDataPlan, ids sampleDomainIDs) error {
	handoverStep := sampleCompletedStep(plan, "handover", "source.handover_completed")
	if handoverStep == nil || sampleFirstEvidence(*handoverStep) == "" {
		return fmt.Errorf("sample blueprint is missing completed handover evidence")
	}
	handoverID := newID()
	suffix := sampleCodeSuffix(plan.ProjectID)
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO handover_records (
			id,project_id,status,final_acceptance_id,handover_date,client_representative,company_representative,
			documents_ready,warehouse_closed,payables_recorded,receivables_recorded,notes,created_by,approved_by,
			handover_code,workflow_form_instance_id,readiness_snapshot_json,submitted_by,submitted_at,
			checked_by,checked_at,completed_at
		) VALUES ($1,$2,'handed_over',$3,CURRENT_DATE,$4,$5,true,true,true,true,'Bàn giao hoàn tất với đầy đủ hồ sơ mẫu',$6,$6,$7,
			NULLIF($8,'')::uuid,'{"ready":true,"open_blockers":0,"simulated":true}'::jsonb,$6,now()-interval '2 days',$6,now()-interval '1 day',now())
	`, handoverID, plan.ProjectID, ids.acceptanceID, "Đại diện Chủ đầu tư", plan.ActorName, plan.ActorID,
		"DEMO-HO-"+suffix, handoverStep.FormInstanceID); err != nil {
		return err
	}
	items := []struct{ code, category, title, sourceType, sourceID string }{
		{"final_acceptance", "quality", "Nghiệm thu cuối đạt", "acceptance", ids.acceptanceID},
		{"warehouse_closure", "operation", "Kho công trình đã đóng", "stock_document", ids.issueDocumentID},
		{"ap_reconciliation", "finance", "Công nợ phải trả đã tất toán", "payable", ids.payableID},
		{"ar_reconciliation", "finance", "Công nợ phải thu đã tất toán", "receivable", ids.receivableID},
		{"signed_handover_minutes", "legal", "Biên bản bàn giao có minh chứng", "evidence", sampleFirstEvidence(*handoverStep)},
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
