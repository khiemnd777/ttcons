\set ON_ERROR_STOP on

BEGIN;

CREATE TEMP TABLE golden_quality_work AS
SELECT
  (array_agg(w.id) FILTER (WHERE rs.surface_kind='quality'))[1] AS quality_work_item_id,
  (array_agg(w.id) FILTER (WHERE rs.domain_action_key='subcontractor.reconcile_checklist'))[1] AS subcontractor_work_item_id,
  (array_agg(w.id) FILTER (WHERE rs.domain_action_key='owner.reconcile_acceptance'))[1] AS owner_work_item_id,
  (array_agg(w.id) FILTER (WHERE rs.surface_kind='handover'))[1] AS handover_work_item_id
FROM work_items w
JOIN pipeline_run_steps rs ON rs.id=w.run_step_id
WHERE w.project_id='30000000-0000-0000-0000-000000000020';

CREATE TEMP TABLE golden_quality_evidence AS
SELECT
  (SELECT id FROM evidence_records WHERE project_id='30000000-0000-0000-0000-000000000020' AND status='accepted' ORDER BY created_at,id LIMIT 1) AS request_evidence_id,
  (SELECT id FROM evidence_records WHERE project_id='30000000-0000-0000-0000-000000000020' AND status='accepted' ORDER BY created_at,id OFFSET 1 LIMIT 1) AS inspection_evidence_id,
  (SELECT id FROM evidence_records WHERE project_id='30000000-0000-0000-0000-000000000020' AND status='accepted' ORDER BY created_at,id OFFSET 2 LIMIT 1) AS corrective_evidence_id;

-- QA/QC: immutable ITP revisions, pass/fail RFIs, closed NCR and recurring checks.
INSERT INTO quality_itp_plans (
  id,project_id,family_id,itp_code,title,discipline,work_package,revision_no,supersedes_id,
  definition_hash,status,review_comment,document_id,created_by,submitted_by,submitted_at,
  approved_by,approved_at,activated_by,activated_at
) VALUES
('86000000-0000-0000-0000-000000000201','30000000-0000-0000-0000-000000000020','86100000-0000-0000-0000-000000000201','ITP-GOLD-001','ITP hoàn thiện văn phòng','Hoàn thiện','MEP, trần, sơn, sàn',1,NULL,repeat('a',64),'superseded','Rev.01 được thay thế sau review hold point.',NULL,'10000000-0000-0000-0000-000000000006','10000000-0000-0000-0000-000000000006','2026-03-01 09:00+07','10000000-0000-0000-0000-000000000007','2026-03-02 09:00+07','10000000-0000-0000-0000-000000000002','2026-03-02 10:00+07'),
('86000000-0000-0000-0000-000000000202','30000000-0000-0000-0000-000000000020','86100000-0000-0000-0000-000000000201','ITP-GOLD-001','ITP hoàn thiện văn phòng','Hoàn thiện','MEP, trần, sơn, sàn',2,'86000000-0000-0000-0000-000000000201',repeat('b',64),'active','Bổ sung witness point cao độ trần và mẫu màu.','46000000-0000-0000-0000-000000000206','10000000-0000-0000-0000-000000000006','10000000-0000-0000-0000-000000000006','2026-03-03 09:00+07','10000000-0000-0000-0000-000000000007','2026-03-04 09:00+07','10000000-0000-0000-0000-000000000002','2026-03-04 10:00+07');

INSERT INTO quality_itp_items (
  id,itp_plan_id,sequence_no,activity_name,inspection_stage,acceptance_criteria,
  inspection_method,control_point,frequency_type,frequency_value,responsible_role,
  evidence_required,client_witness_required,notes
) VALUES
('86200000-0000-0000-0000-000000000201','86000000-0000-0000-0000-000000000202',1,'Kiểm tra MEP trước đóng trần','Pre-cover','Đúng bản vẽ, đủ test continuity','Quan sát và đo kiểm','hold','each_lot',1,'QA/QC',true,true,'Không đóng trần trước khi RFI đạt.'),
('86200000-0000-0000-0000-000000000202','86000000-0000-0000-0000-000000000202',2,'Kiểm tra cao độ và độ phẳng trần','In-process','Sai số cao độ không quá 3mm','Laser level','witness','each_lot',1,'QA/QC',true,true,'Có đại diện PM witness.'),
('86200000-0000-0000-0000-000000000203','86000000-0000-0000-0000-000000000202',3,'Duyệt mẫu màu sơn','Mockup','Đúng mã màu đã duyệt, bề mặt đều','Mockup và visual','review','each_lot',1,'QA/QC',true,true,'Mẫu lưu tại site.'),
('86200000-0000-0000-0000-000000000204','86000000-0000-0000-0000-000000000202',4,'Nghiệm thu hoàn thiện cuối','Final','Không còn defect ảnh hưởng sử dụng','Checklist và ảnh','hold','each_lot',1,'QA/QC',true,true,'Điều kiện mở nghiệm thu chủ đầu tư.');

INSERT INTO issues (
  id,project_id,title,description,severity,status,reported_by,assigned_to,requires_rework,requires_replanning
) VALUES
('70000000-0000-0000-0000-000000000201','30000000-0000-0000-0000-000000000020','Cao độ trần khu B lệch 8mm','RFI lần một phát hiện khung trần vượt dung sai ITP.','high','closed','10000000-0000-0000-0000-000000000006','21000000-0000-0000-0000-000000000006',true,false);

INSERT INTO rework_requests (
  id,project_id,issue_id,title,description,assigned_to,status,due_date,created_by,approved_by
) VALUES
('72000000-0000-0000-0000-000000000201','30000000-0000-0000-0000-000000000020','70000000-0000-0000-0000-000000000201','Điều chỉnh khung trần khu B','Tháo cục bộ, cân chỉnh ty treo và đo lại toàn bộ lưới trần.','21000000-0000-0000-0000-000000000006','completed','2026-04-15','10000000-0000-0000-0000-000000000006','10000000-0000-0000-0000-000000000002');

INSERT INTO quality_inspection_requests (
  id,project_id,rfi_code,itp_plan_id,itp_item_id,work_item_id,location,lot_reference,
  requested_inspection_at,status,result,request_comment,inspection_comment,
  request_evidence_id,inspection_evidence_id,requested_by,submitted_at,inspector_id,inspected_at,closed_at
) VALUES
('86300000-0000-0000-0000-000000000201','30000000-0000-0000-0000-000000000020','RFI-GOLD-001','86000000-0000-0000-0000-000000000202','86200000-0000-0000-0000-000000000201',(SELECT quality_work_item_id FROM golden_quality_work),'Khu A','MEP-A-01','2026-03-20 09:00+07','accepted','pass','Đề nghị kiểm tra MEP trước đóng trần.','Đạt theo bản vẽ và test continuity.',(SELECT request_evidence_id FROM golden_quality_evidence),(SELECT inspection_evidence_id FROM golden_quality_evidence),'10000000-0000-0000-0000-000000000006','2026-03-19 16:00+07','10000000-0000-0000-0000-000000000002','2026-03-20 10:00+07','2026-03-20 10:00+07'),
('86300000-0000-0000-0000-000000000202','30000000-0000-0000-0000-000000000020','RFI-GOLD-002','86000000-0000-0000-0000-000000000202','86200000-0000-0000-0000-000000000202',(SELECT quality_work_item_id FROM golden_quality_work),'Khu B','CEILING-B-01','2026-04-10 09:00+07','rejected','fail','Đề nghị kiểm tra cao độ trần khu B.','Không đạt: lệch cao độ 8mm.',(SELECT request_evidence_id FROM golden_quality_evidence),(SELECT inspection_evidence_id FROM golden_quality_evidence),'10000000-0000-0000-0000-000000000006','2026-04-09 16:00+07','10000000-0000-0000-0000-000000000002','2026-04-10 10:00+07','2026-04-10 10:00+07'),
('86300000-0000-0000-0000-000000000203','30000000-0000-0000-0000-000000000020','RFI-GOLD-003','86000000-0000-0000-0000-000000000202','86200000-0000-0000-0000-000000000202',(SELECT quality_work_item_id FROM golden_quality_work),'Khu B','CEILING-B-RECHECK','2026-04-16 09:00+07','accepted','pass','Tái kiểm sau khi cân chỉnh khung trần.','Đạt: sai số còn 2mm, trong dung sai.',(SELECT corrective_evidence_id FROM golden_quality_evidence),(SELECT inspection_evidence_id FROM golden_quality_evidence),'10000000-0000-0000-0000-000000000006','2026-04-15 16:00+07','10000000-0000-0000-0000-000000000002','2026-04-16 10:00+07','2026-04-16 10:00+07');

INSERT INTO quality_ncrs (
  id,project_id,ncr_code,source_type,source_id,rfi_id,work_item_id,issue_id,rework_request_id,
  title,severity,nonconformity,immediate_action,root_cause,corrective_action,preventive_action,
  due_date,status,evidence_id,corrective_evidence_id,reinspection_rfi_id,owner_id,created_by,
  root_cause_submitted_by,corrective_submitted_by,closed_by,closed_at,closure_comment
) VALUES
('86400000-0000-0000-0000-000000000201','30000000-0000-0000-0000-000000000020','NCR-GOLD-001','rfi','86300000-0000-0000-0000-000000000202','86300000-0000-0000-0000-000000000202',(SELECT quality_work_item_id FROM golden_quality_work),'70000000-0000-0000-0000-000000000201','72000000-0000-0000-0000-000000000201','Cao độ trần khu B không đạt','high','Cao độ lệch 8mm so với tolerance 3mm.','Dừng đóng tấm khu B và đánh dấu phạm vi.','Ty treo chưa được kiểm tra chéo bằng laser trước khi khóa khung.','Cân chỉnh toàn bộ ty treo, đo lưới 2m và ghi nhận kết quả.','Bổ sung hold point laser checklist trước đóng tấm.','2026-04-16','closed',(SELECT inspection_evidence_id FROM golden_quality_evidence),(SELECT corrective_evidence_id FROM golden_quality_evidence),'86300000-0000-0000-0000-000000000203','10000000-0000-0000-0000-000000000006','10000000-0000-0000-0000-000000000006','10000000-0000-0000-0000-000000000006','10000000-0000-0000-0000-000000000006','10000000-0000-0000-0000-000000000002','2026-04-16 11:00+07','Tái kiểm đạt; rework hoàn tất; cập nhật biện pháp phòng ngừa.');

INSERT INTO construction_recurring_plans (
  id,project_id,plan_code,title,description,recurrence_type,interval_value,weekdays,
  start_date,end_date,next_due_date,due_time,responsible_role,assigned_user_id,
  evidence_required,source_work_item_id,status,created_by
) VALUES
('86500000-0000-0000-0000-000000000201','30000000-0000-0000-0000-000000000020','QA-GOLD-WEEKLY','Kiểm tra QA/QC và an toàn hằng tuần','Kiểm tra ITP, housekeeping, vật tư và công việc che khuất.','weekly',1,'[1]'::jsonb,'2026-03-01','2026-05-31','2026-06-01','16:00','QA/QC','10000000-0000-0000-0000-000000000006',true,(SELECT quality_work_item_id FROM golden_quality_work),'completed','10000000-0000-0000-0000-000000000006');

INSERT INTO construction_recurring_occurrences (
  plan_id,project_id,occurrence_date,due_at,assigned_user_id,status,completion_comment,
  evidence_id,completed_by,completed_at
) VALUES
('86500000-0000-0000-0000-000000000201','30000000-0000-0000-0000-000000000020','2026-03-09','2026-03-09 16:00+07','10000000-0000-0000-0000-000000000006','completed','Đã kiểm tra MEP che khuất và housekeeping.',(SELECT request_evidence_id FROM golden_quality_evidence),'10000000-0000-0000-0000-000000000006','2026-03-09 16:30+07'),
('86500000-0000-0000-0000-000000000201','30000000-0000-0000-0000-000000000020','2026-04-13','2026-04-13 16:00+07','10000000-0000-0000-0000-000000000006','completed','Theo dõi NCR trần và biện pháp sửa chữa.',(SELECT corrective_evidence_id FROM golden_quality_evidence),'10000000-0000-0000-0000-000000000006','2026-04-13 16:30+07'),
('86500000-0000-0000-0000-000000000201','30000000-0000-0000-0000-000000000020','2026-05-18','2026-05-18 16:00+07','10000000-0000-0000-0000-000000000006','completed','Kiểm tra hoàn thiện cuối và hồ sơ nghiệm thu.',(SELECT inspection_evidence_id FROM golden_quality_evidence),'10000000-0000-0000-0000-000000000006','2026-05-18 16:30+07');

-- Final acceptance preserves a resolved conditional item and final pass history.
INSERT INTO acceptance_records (
  id,project_id,type,title,requested_by,inspected_by,approved_by,status,result,notes,
  acceptance_code,work_item_id,form_instance_id,acceptance_date,location,
  client_representative,contractor_representative,scope_summary,signature_status,
  document_status,summary_json,submitted_by,submitted_at
)
SELECT '71000000-0000-0000-0000-000000000201',w.project_id,'final',
       'Nghiệm thu hoàn thành Golden Project','10000000-0000-0000-0000-000000000002',
       '21000000-0000-0000-0000-000000000006','10000000-0000-0000-0000-000000000007',
       'completed','pass','Đã xử lý toàn bộ punch list và tái kiểm đạt.','OA-GOLD-FINAL',w.id,f.id,
       '2026-06-10','Toàn bộ tầng 12','Anh Bình – Công ty An Phát','Nguyễn Minh PM',
       'Nghiệm thu hoàn thành MEP, trần, sơn, sàn và hồ sơ chất lượng.','signed','linked',
       '{"initial_result":"conditional_pass","punch_items":1,"resolved_items":1,"final_result":"pass"}'::jsonb,
       '10000000-0000-0000-0000-000000000002','2026-06-10 16:00+07'
FROM golden_quality_work q
JOIN work_items w ON w.id=q.owner_work_item_id
JOIN workflow_form_instances f ON f.work_item_id=w.id
LIMIT 1;

INSERT INTO acceptance_checklist_items (
  acceptance_id,title,description,is_passed,note,photo_required,item_index,evidence_id,
  decision,severity,required_action,payload_json
) VALUES
('71000000-0000-0000-0000-000000000201','MEP và thiết bị','Test vận hành, nhãn mạch và bản vẽ hoàn công.',true,'Đạt',true,0,(SELECT request_evidence_id FROM golden_quality_evidence),'pass','low',NULL,'{"verified":true}'),
('71000000-0000-0000-0000-000000000201','Cao độ trần khu B','Từng không đạt; đã NCR và tái kiểm.',true,'Đạt sau rework NCR-GOLD-001',true,1,(SELECT corrective_evidence_id FROM golden_quality_evidence),'pass','high','Đã hoàn tất ngày 16/04/2026','{"history":"conditional_pass_to_pass","ncr":"NCR-GOLD-001"}'),
('71000000-0000-0000-0000-000000000201','Sơn và sàn hoàn thiện','Đúng mẫu, không trầy xước ảnh hưởng sử dụng.',true,'Đạt',true,2,(SELECT inspection_evidence_id FROM golden_quality_evidence),'pass','low',NULL,'{"verified":true}');

UPDATE rework_requests
SET acceptance_id='71000000-0000-0000-0000-000000000201'
WHERE id='72000000-0000-0000-0000-000000000201';

-- Checklist reconciliations keep quantity/client decisions separate from acceptance.
INSERT INTO subcontractor_checklist_reconciliations (
  id,project_id,work_item_id,form_instance_id,form_revision_no,status,subcontractor_name,
  work_package,construction_area,summary_json,comment,created_by,submitted_by,submitted_at,updated_by
)
SELECT '48300000-0000-0000-0000-000000000201',w.project_id,w.id,f.id,1,'submitted',
       'Thầu phụ điện An Tâm','MEP','Khu A và B','{"claimed":2400,"confirmed":2388,"variance":-12}',
       'Chấp nhận giảm 12m theo đo hoàn công.','10000000-0000-0000-0000-000000000002',
       '10000000-0000-0000-0000-000000000002','2026-05-25 10:00+07','10000000-0000-0000-0000-000000000002'
FROM golden_quality_work q
JOIN work_items w ON w.id=q.subcontractor_work_item_id
JOIN workflow_form_instances f ON f.work_item_id=w.id LIMIT 1;

INSERT INTO subcontractor_checklist_reconciliation_items (
  reconciliation_id,project_id,work_item_id,form_instance_id,item_index,checklist_title,
  boq_item_id,boq_item_ref,boq_work_item_name,boq_quantity,boq_unit,claimed_quantity,
  confirmed_quantity,variance_quantity,unit,status,evidence_id,owner_response,decision,comment
)
SELECT '48300000-0000-0000-0000-000000000201',r.project_id,r.work_item_id,r.form_instance_id,0,
       'Đối chiếu khối lượng kéo dây hoàn công',b.id,'BOQ-GOLD-ELECTRIC',b.work_item_name,b.quantity,b.unit,
       b.quantity,b.quantity-12,-12,b.unit,'done',(SELECT request_evidence_id FROM golden_quality_evidence),
       'adjust_quantity','adjust_quantity','QS và thầu phụ ký xác nhận khối lượng thực tế.'
FROM subcontractor_checklist_reconciliations r
JOIN boq_items b ON b.id='61000000-0000-0000-0000-000000000201'
WHERE r.id='48300000-0000-0000-0000-000000000201';

INSERT INTO owner_client_checklist_reconciliations (
  id,project_id,work_item_id,form_instance_id,form_revision_no,status,client_representative,
  company_representative,handover_scope,scope_detail,summary_json,comment,
  created_by,submitted_by,submitted_at,updated_by
)
SELECT '48400000-0000-0000-0000-000000000201',w.project_id,w.id,f.id,1,'submitted',
       'Anh Bình – Công ty An Phát','Nguyễn Minh PM','final_project','Toàn bộ công trình',
       '{"items":3,"pass":3,"conditional":0}',
       'Chủ đầu tư xác nhận toàn bộ tồn tại đã xử lý.','10000000-0000-0000-0000-000000000002',
       '10000000-0000-0000-0000-000000000002','2026-06-10 16:30+07','10000000-0000-0000-0000-000000000002'
FROM golden_quality_work q
JOIN work_items w ON w.id=q.owner_work_item_id
JOIN workflow_form_instances f ON f.work_item_id=w.id LIMIT 1;

INSERT INTO owner_client_checklist_reconciliation_items (
  reconciliation_id,project_id,work_item_id,form_instance_id,item_index,checklist_title,
  output_ref,status,client_decision,severity,evidence_id,acceptance_id,acceptance_title,
  acceptance_type,acceptance_status,acceptance_result,decision,comment
)
SELECT '48400000-0000-0000-0000-000000000201',r.project_id,r.work_item_id,r.form_instance_id,x.idx,
       x.title,x.ref,'done','pass','low',x.evidence_id,'71000000-0000-0000-0000-000000000201',
       'Nghiệm thu hoàn thành Golden Project','final','completed','pass','accepted','Đạt và đủ hồ sơ.'
FROM owner_client_checklist_reconciliations r
CROSS JOIN LATERAL (VALUES
  (0,'MEP và thiết bị','OA-GOLD-MEP',(SELECT request_evidence_id FROM golden_quality_evidence)),
  (1,'Hoàn thiện kiến trúc','OA-GOLD-ARCH',(SELECT inspection_evidence_id FROM golden_quality_evidence)),
  (2,'Hồ sơ hoàn công','DOC-GOLD-ASBUILT',(SELECT corrective_evidence_id FROM golden_quality_evidence))
) x(idx,title,ref,evidence_id)
WHERE r.id='48400000-0000-0000-0000-000000000201';

COMMIT;
