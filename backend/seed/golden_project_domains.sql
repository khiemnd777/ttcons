\set ON_ERROR_STOP on

BEGIN;

-- Business ledgers for GOLDEN-REAL-001. All records are synthetic but follow the
-- same source links, approvals, reconciliations and closure rules as production.

-- Durable project documents used by contract, quality, acceptance and handover.
INSERT INTO documents (id,project_id,target_type,target_id,name,file_url,document_type,status,uploaded_by) VALUES
('46000000-0000-0000-0000-000000000201','30000000-0000-0000-0000-000000000020','project','30000000-0000-0000-0000-000000000020','Bản vẽ thi công được duyệt','https://example.com/golden/as-built-drawing.pdf','drawing','active','10000000-0000-0000-0000-000000000002'),
('46000000-0000-0000-0000-000000000202','30000000-0000-0000-0000-000000000020','project','30000000-0000-0000-0000-000000000020','Hợp đồng chủ đầu tư đã ký','https://example.com/golden/main-contract.pdf','contract','active','10000000-0000-0000-0000-000000000002'),
('46000000-0000-0000-0000-000000000203','30000000-0000-0000-0000-000000000020','project','30000000-0000-0000-0000-000000000020','Hướng dẫn vận hành và bảo trì','https://example.com/golden/operation-manual.pdf','operation_manual','active','10000000-0000-0000-0000-000000000006'),
('46000000-0000-0000-0000-000000000204','30000000-0000-0000-0000-000000000020','project','30000000-0000-0000-0000-000000000020','Cam kết bảo hành 365 ngày','https://example.com/golden/warranty.pdf','warranty','active','10000000-0000-0000-0000-000000000002'),
('46000000-0000-0000-0000-000000000205','30000000-0000-0000-0000-000000000020','project','30000000-0000-0000-0000-000000000020','Biên bản bàn giao hai bên','https://example.com/golden/signed-handover.pdf','handover','active','10000000-0000-0000-0000-000000000002'),
('46000000-0000-0000-0000-000000000206','30000000-0000-0000-0000-000000000020','project','30000000-0000-0000-0000-000000000020','ITP hoàn thiện văn phòng Rev.02','https://example.com/golden/itp-rev02.pdf','quality','active','10000000-0000-0000-0000-000000000006');

-- BOQ with approved norms, immutable final prices and quotation traceability.
INSERT INTO boq_items (
  id,project_id,category,work_item_name,description,unit,quantity,unit_price,material_id,status,
  created_by,approved_by,expected_unit_price,norm_unit_price,final_unit_price,unit_price_source,
  norm_price_locked,norm_price_locked_by,norm_price_locked_at,norm_price_comment,
  final_price_locked,final_price_locked_by,final_price_locked_at,final_price_comment,price_revision_no
) VALUES
('61000000-0000-0000-0000-000000000201','30000000-0000-0000-0000-000000000020','Điện','Kéo dây điện khu làm việc','Dây Cadivi âm trần','m',2400,12500,'50000000-0000-0000-0000-000000000001','approved','10000000-0000-0000-0000-000000000003','10000000-0000-0000-0000-000000000007',13000,12800,12500,'quotation',true,'10000000-0000-0000-0000-000000000003','2026-02-15 09:00+07','Theo định mức MEP',true,'10000000-0000-0000-0000-000000000007','2026-02-20 10:00+07','Chọn giá đạt kỹ thuật và giao hàng 7 ngày',3),
('61000000-0000-0000-0000-000000000202','30000000-0000-0000-0000-000000000020','Nước','Lắp đặt ống PVC pantry','Ống PVC Bình Minh','m',520,29500,'50000000-0000-0000-0000-000000000002','approved','10000000-0000-0000-0000-000000000003','10000000-0000-0000-0000-000000000007',31000,30000,29500,'quotation',true,'10000000-0000-0000-0000-000000000003','2026-02-15 09:00+07','Định mức MEP pantry',true,'10000000-0000-0000-0000-000000000007','2026-02-20 10:00+07','Chọn phương án bảo hành tốt hơn',3),
('61000000-0000-0000-0000-000000000203','30000000-0000-0000-0000-000000000020','Sơn','Sơn hoàn thiện tường','Sơn Dulux nội thất','thùng',86,1420000,'50000000-0000-0000-0000-000000000003','approved','10000000-0000-0000-0000-000000000003','10000000-0000-0000-0000-000000000007',1500000,1450000,1420000,'quotation',true,'10000000-0000-0000-0000-000000000003','2026-02-15 09:00+07','Định mức sơn hai lớp',true,'10000000-0000-0000-0000-000000000007','2026-02-20 10:00+07','Mẫu màu đã được chủ đầu tư duyệt',3),
('61000000-0000-0000-0000-000000000204','30000000-0000-0000-0000-000000000020','Trần','Thi công trần thạch cao','Tấm thạch cao chống ẩm','tấm',680,168000,'50000000-0000-0000-0000-000000000004','approved','10000000-0000-0000-0000-000000000003','10000000-0000-0000-0000-000000000007',175000,170000,168000,'quotation',true,'10000000-0000-0000-0000-000000000003','2026-02-15 09:00+07','Theo bản vẽ trần',true,'10000000-0000-0000-0000-000000000007','2026-02-20 10:00+07','Đủ chứng chỉ chất lượng',3),
('61000000-0000-0000-0000-000000000205','30000000-0000-0000-0000-000000000020','Sàn','Lát gạch pantry và sảnh','Gạch Prime 600x600','m2',410,315000,'50000000-0000-0000-0000-000000000005','approved','10000000-0000-0000-0000-000000000003','10000000-0000-0000-0000-000000000007',330000,320000,315000,'quotation',true,'10000000-0000-0000-0000-000000000003','2026-02-15 09:00+07','Bao gồm hao hụt 3%',true,'10000000-0000-0000-0000-000000000007','2026-02-20 10:00+07','Chốt lô và mã màu',3),
('61000000-0000-0000-0000-000000000206','30000000-0000-0000-0000-000000000020','Chiếu sáng','Lắp đèn LED âm trần','Đèn LED Rạng Đông','cái',188,182000,'50000000-0000-0000-0000-000000000006','approved','10000000-0000-0000-0000-000000000003','10000000-0000-0000-0000-000000000007',190000,185000,182000,'quotation',true,'10000000-0000-0000-0000-000000000003','2026-02-15 09:00+07','Theo lux calculation',true,'10000000-0000-0000-0000-000000000007','2026-02-20 10:00+07','Bảo hành 24 tháng',3);

INSERT INTO supplier_quotations (
  id,project_id,quotation_code,supplier_partner_id,supplier_name,quotation_date,valid_until,
  payment_terms_days,lead_time_days,delivery_terms,warranty_terms,status,technical_status,
  commercial_status,notes,submitted_by,submitted_at,qualified_by,qualified_at,selected_at,created_by
) VALUES
('64000000-0000-0000-0000-000000000201','30000000-0000-0000-0000-000000000020','QT-GOLD-CADIVI','90000000-0000-0000-0000-000000000101','Cadivi','2026-02-16','2026-03-16',30,7,'Giao tại công trình','Bảo hành chính hãng 24 tháng','selected','compliant','compliant','Giá tốt, giao nhanh.','10000000-0000-0000-0000-000000000003','2026-02-17 09:00+07','10000000-0000-0000-0000-000000000007','2026-02-18 09:00+07','2026-02-20 10:00+07','10000000-0000-0000-0000-000000000003'),
('64000000-0000-0000-0000-000000000202','30000000-0000-0000-0000-000000000020','QT-GOLD-DULUX','90000000-0000-0000-0000-000000000103','Dulux','2026-02-16','2026-03-16',30,10,'Giao theo đợt','Bảo hành vật liệu 18 tháng','selected','compliant','compliant','Đạt mẫu màu và hỗ trợ kỹ thuật.','10000000-0000-0000-0000-000000000003','2026-02-17 09:00+07','10000000-0000-0000-0000-000000000007','2026-02-18 09:00+07','2026-02-20 10:00+07','10000000-0000-0000-0000-000000000003'),
('64000000-0000-0000-0000-000000000203','30000000-0000-0000-0000-000000000020','QT-GOLD-TONGHOP','90000000-0000-0000-0000-000000000107','Nhà cung cấp tổng hợp','2026-02-16','2026-03-16',15,12,'Giao tại kho site','Bảo hành 12 tháng','qualified','compliant','compliant','Phương án dự phòng đủ kỹ thuật.','10000000-0000-0000-0000-000000000003','2026-02-17 09:00+07','10000000-0000-0000-0000-000000000007','2026-02-18 09:00+07',NULL,'10000000-0000-0000-0000-000000000003');

INSERT INTO supplier_quotation_items (
  id,quotation_id,boq_item_id,material_id,description,unit,quantity,unit_price,
  discount_amount,tax_rate,lead_time_days,compliance_status,deviation_note
)
SELECT gen_random_uuid(),q.id,b.id,b.material_id,b.work_item_name,b.unit,b.quantity,
       round(b.final_unit_price * CASE q.quotation_code WHEN 'QT-GOLD-CADIVI' THEN 1.00 WHEN 'QT-GOLD-DULUX' THEN 1.015 ELSE 1.035 END,2),
       0,10,q.lead_time_days,'compliant',
       CASE WHEN q.quotation_code='QT-GOLD-TONGHOP' THEN 'Lead time dài hơn phương án được chọn.' END
FROM supplier_quotations q
CROSS JOIN boq_items b
WHERE q.project_id='30000000-0000-0000-0000-000000000020'
  AND b.project_id=q.project_id;

INSERT INTO supplier_quote_selections (
  project_id,boq_item_id,quotation_item_id,status,decision_reason,selected_by,selected_at
)
SELECT b.project_id,b.id,qi.id,'active',
       CASE WHEN b.category IN ('Sơn','Sàn')
            THEN 'Đạt mẫu hoàn thiện, hỗ trợ kỹ thuật và tổng chi phí tối ưu.'
            ELSE 'Đáp ứng kỹ thuật, tiến độ giao hàng và điều kiện bảo hành tốt nhất.' END,
       '10000000-0000-0000-0000-000000000007','2026-02-20 10:00+07'
FROM boq_items b
JOIN supplier_quotation_items qi ON qi.boq_item_id=b.id
JOIN supplier_quotations q ON q.id=qi.quotation_id
WHERE b.project_id='30000000-0000-0000-0000-000000000020'
  AND q.quotation_code=CASE WHEN b.category IN ('Sơn','Sàn') THEN 'QT-GOLD-DULUX' ELSE 'QT-GOLD-CADIVI' END;

UPDATE boq_items b
SET selected_quotation_item_id=s.quotation_item_id
FROM supplier_quote_selections s
WHERE s.boq_item_id=b.id AND s.status='active'
  AND b.project_id='30000000-0000-0000-0000-000000000020';

INSERT INTO boq_unit_price_events (
  project_id,boq_item_id,event_type,price_source,from_unit_price,to_unit_price,revision_no,
  comment,snapshot_json,actor_id,actor_name,created_at
)
SELECT b.project_id,b.id,'final_locked','final',b.norm_unit_price,b.final_unit_price,b.price_revision_no,
       'Khóa giá cuối từ báo giá đã qualify và được phê duyệt.',
       jsonb_build_object('quotation_item_id',b.selected_quotation_item_id,'golden_sample',true),
       '10000000-0000-0000-0000-000000000007','Lê Hoàng Giám đốc','2026-02-20 10:00+07'
FROM boq_items b WHERE b.project_id='30000000-0000-0000-0000-000000000020';

-- Procurement is grouped by the selected supplier and fully received through
-- explicit incoming acceptance. Warehouse movements remain separate.
INSERT INTO procurement_requests (
  id,project_id,request_code,title,requested_by,status,total_estimated_amount,budget_status,
  supplier_partner_id,supplier_name,order_reference,expected_delivery_date,ordered_at
) VALUES
('60000000-0000-0000-0000-000000000201','30000000-0000-0000-0000-000000000020','PR-GOLD-MEP','Mua vật tư MEP và trần','10000000-0000-0000-0000-000000000003','received',193230000,'within_budget','90000000-0000-0000-0000-000000000101','Cadivi','PO-GOLD-001','2026-03-05','2026-02-22 09:00+07'),
('60000000-0000-0000-0000-000000000202','30000000-0000-0000-0000-000000000020','PR-GOLD-FINISH','Mua vật tư hoàn thiện','10000000-0000-0000-0000-000000000003','received',251270000,'within_budget','90000000-0000-0000-0000-000000000103','Dulux','PO-GOLD-002','2026-03-08','2026-02-22 09:30+07');

INSERT INTO procurement_request_items (
  id,request_id,material_id,boq_item_id,quantity_planned,quantity_requested,unit,
  estimated_unit_price,supplier_partner_id,supplier_name,delivery_status,approval_status,
  boq_price_source,boq_price_revision_no,boq_price_event_id,supplier_quotation_item_id,
  quoted_unit_price,price_variance_amount
)
SELECT gen_random_uuid(),
       CASE WHEN b.category IN ('Sơn','Sàn') THEN '60000000-0000-0000-0000-000000000202'::uuid
            ELSE '60000000-0000-0000-0000-000000000201'::uuid END,
       b.material_id,b.id,b.quantity,b.quantity,b.unit,b.final_unit_price,q.supplier_partner_id,q.supplier_name,
       'delivered','approved','final',b.price_revision_no,e.id,b.selected_quotation_item_id,b.final_unit_price,0
FROM boq_items b
JOIN supplier_quotation_items qi ON qi.id=b.selected_quotation_item_id
JOIN supplier_quotations q ON q.id=qi.quotation_id
LEFT JOIN LATERAL (
  SELECT id FROM boq_unit_price_events WHERE boq_item_id=b.id ORDER BY created_at DESC LIMIT 1
) e ON true
WHERE b.project_id='30000000-0000-0000-0000-000000000020';

CREATE TEMP TABLE golden_incoming_work AS
SELECT w.id AS work_item_id
FROM work_items w
JOIN pipeline_run_steps rs ON rs.id=w.run_step_id
WHERE w.project_id='30000000-0000-0000-0000-000000000020'
  AND rs.domain_action_key='acceptance.incoming_material'
LIMIT 1;

INSERT INTO incoming_acceptance_records (
  id,acceptance_code,project_id,work_item_id,procurement_request_id,warehouse_id,source_type,
  supplier_name,delivery_reference,status,result,received_at,summary_json,comment,
  created_by,submitted_by,submitted_at,updated_by,supplier_partner_id
) VALUES
('48200000-0000-0000-0000-000000000201','IA-GOLD-001','30000000-0000-0000-0000-000000000020',(SELECT work_item_id FROM golden_incoming_work),'60000000-0000-0000-0000-000000000201','51000000-0000-0000-0000-000000000002','procurement','Cadivi','DN-GOLD-001','submitted','pass','2026-03-05','{"delivery":"full","quality":"pass"}','Đủ CO/CQ và số lượng; chấp nhận nhập site.','10000000-0000-0000-0000-000000000005','10000000-0000-0000-0000-000000000005','2026-03-05 15:00+07','10000000-0000-0000-0000-000000000005','90000000-0000-0000-0000-000000000101'),
('48200000-0000-0000-0000-000000000202','IA-GOLD-002','30000000-0000-0000-0000-000000000020',(SELECT work_item_id FROM golden_incoming_work),'60000000-0000-0000-0000-000000000202','51000000-0000-0000-0000-000000000002','procurement','Dulux','DN-GOLD-002','submitted','conditional_pass','2026-03-08','{"delivery":"full","quality":"conditional_pass"}','Chấp nhận; mẫu màu lô sơn đã được QA xác nhận tại site.','10000000-0000-0000-0000-000000000005','10000000-0000-0000-0000-000000000005','2026-03-08 15:00+07','10000000-0000-0000-0000-000000000005','90000000-0000-0000-0000-000000000103');

INSERT INTO incoming_acceptance_items (
  incoming_acceptance_id,project_id,work_item_id,item_index,material_id,material_code,
  material_name,procurement_request_item_id,delivered_quantity,accepted_quantity,rejected_quantity,
  unit,condition,evidence_id,decision,comment,payload_json
)
SELECT ia.id,ia.project_id,ia.work_item_id,
       row_number() OVER (PARTITION BY ia.id ORDER BY pri.id)-1,pri.material_id,m.code,m.name,pri.id,
       pri.quantity_requested,pri.quantity_requested,0,pri.unit,'usable',
       (SELECT e.id FROM evidence_records e WHERE e.work_item_id=ia.work_item_id ORDER BY e.created_at LIMIT 1),
       CASE WHEN ia.result='conditional_pass' THEN 'conditional_acceptance' ELSE 'accepted' END,
       'Đối chiếu PO, delivery note, số lượng và tình trạng vật tư đạt.',
       jsonb_build_object('golden_sample',true,'po',pr.order_reference)
FROM incoming_acceptance_records ia
JOIN procurement_requests pr ON pr.id=ia.procurement_request_id
JOIN procurement_request_items pri ON pri.request_id=pr.id
JOIN materials m ON m.id=pri.material_id
WHERE ia.project_id='30000000-0000-0000-0000-000000000020';

-- Explicit warehouse/site ledger with zero residual surplus at closure.
INSERT INTO stock_movements (
  id,movement_code,project_id,warehouse_id,material_id,movement_type,quantity,unit,reason,
  reference_type,reference_id,created_by,approved_by
)
SELECT gen_random_uuid(),'SM-GOLD-RECV-' || right(m.code,6),'30000000-0000-0000-0000-000000000020',
       '51000000-0000-0000-0000-000000000002',m.id,'receive',b.quantity,m.unit,
       'Nhập kho site sau nghiệm thu đầu vào','boq_item',b.id,
       '10000000-0000-0000-0000-000000000005','10000000-0000-0000-0000-000000000002'
FROM boq_items b JOIN materials m ON m.id=b.material_id
WHERE b.project_id='30000000-0000-0000-0000-000000000020';

INSERT INTO stock_movements (
  id,movement_code,project_id,warehouse_id,material_id,movement_type,quantity,unit,reason,
  reference_type,reference_id,created_by,approved_by
)
SELECT gen_random_uuid(),'SM-GOLD-ISSUE-' || right(m.code,6),'30000000-0000-0000-0000-000000000020',
       '51000000-0000-0000-0000-000000000002',m.id,'issue',round(b.quantity*0.97,3),m.unit,
       'Xuất cho thi công theo nhật ký và BOQ','boq_item',b.id,
       '10000000-0000-0000-0000-000000000005','10000000-0000-0000-0000-000000000002'
FROM boq_items b JOIN materials m ON m.id=b.material_id
WHERE b.project_id='30000000-0000-0000-0000-000000000020';

INSERT INTO stock_movements (
  id,movement_code,project_id,warehouse_id,material_id,movement_type,quantity,unit,reason,
  reference_type,reference_id,created_by,approved_by
)
SELECT gen_random_uuid(),'SM-GOLD-RETURN-' || right(m.code,6),'30000000-0000-0000-0000-000000000020',
       '51000000-0000-0000-0000-000000000001',m.id,'return',round(b.quantity*0.03,3),m.unit,
       'Hoàn trả toàn bộ vật tư dư về kho trung tâm','boq_item',b.id,
       '10000000-0000-0000-0000-000000000005','10000000-0000-0000-0000-000000000002'
FROM boq_items b JOIN materials m ON m.id=b.material_id
WHERE b.project_id='30000000-0000-0000-0000-000000000020';

INSERT INTO site_material_balances (
  project_id,material_id,planned_quantity,issued_quantity,used_quantity,surplus_quantity,surplus_status
)
SELECT b.project_id,b.material_id,b.quantity,b.quantity,round(b.quantity*0.97,3),0,'handled'
FROM boq_items b WHERE b.project_id='30000000-0000-0000-0000-000000000020';

-- Construction diary shows a realistic time series rather than one summary row.
INSERT INTO operation_daily_reports (
  id,project_id,report_date,reported_by,weather,work_summary,manpower_summary,
  material_usage_summary,issues_summary,progress_percent,photos,structured_json
) VALUES
('86500000-0000-0000-0000-000000000201','30000000-0000-0000-0000-000000000020','2026-03-15','10000000-0000-0000-0000-000000000006','Nắng','Triển khai MEP âm trần khu A và B.','12 thợ MEP, 4 giám sát','Dây Cadivi 820m; ống PVC 160m','Không có sự cố an toàn.',42,'[]','{"areas":["A","B"],"safety":"pass"}'),
('86500000-0000-0000-0000-000000000202','30000000-0000-0000-0000-000000000020','2026-04-10','10000000-0000-0000-0000-000000000006','Ít mây','Đóng trần, bả và sơn lớp một.','10 thợ trần, 8 thợ sơn','Tấm thạch cao 520 tấm; sơn 42 thùng','Phát hiện một vị trí trần không đạt cao độ, đã mở NCR.',68,'[]','{"areas":["A","B","pantry"],"ncr":"NCR-GOLD-001"}'),
('86500000-0000-0000-0000-000000000203','30000000-0000-0000-0000-000000000020','2026-05-20','10000000-0000-0000-0000-000000000006','Nắng','Hoàn thiện, vệ sinh và nghiệm thu nội bộ.','6 thợ hoàn thiện, 4 QA/QC','Hoàn tất vật tư; vật tư dư chuyển về kho','NCR đã tái kiểm đạt, không còn blocker.',96,'[]','{"areas":["all"],"quality":"ready_for_acceptance"}');

COMMIT;
