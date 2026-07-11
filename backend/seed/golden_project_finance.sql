\set ON_ERROR_STOP on

BEGIN;

-- Contract headers are the commercial source of truth. Main-client Rev.02
-- incorporates an approved variation instead of rewriting signed Rev.01.
INSERT INTO contracts (
  id,project_id,contract_code,title,contract_type,direction,counterparty_partner_id,
  counterparty_name,currency,original_amount,current_amount,advance_percent,
  retention_percent,warranty_days,signed_date,effective_date,expiry_date,status,
  created_by,approved_by,signed_by,activated_by,activated_at
) VALUES
('65000000-0000-0000-0000-000000000201','30000000-0000-0000-0000-000000000020','HD-GOLD-CDT-001','Hợp đồng hoàn thiện Văn phòng An Phát','main_client','receivable','90000000-0000-0000-0000-000000000001','Công ty An Phát','VND',4500000000,4680000000,30,3.5,365,'2026-02-28','2026-03-01','2027-06-30','active','10000000-0000-0000-0000-000000000002','10000000-0000-0000-0000-000000000007','10000000-0000-0000-0000-000000000007','10000000-0000-0000-0000-000000000007','2026-03-01 08:00+07'),
('65000000-0000-0000-0000-000000000202','30000000-0000-0000-0000-000000000020','HD-GOLD-TP-001','Hợp đồng thầu phụ điện An Tâm','subcontractor','payable','90000000-0000-0000-0000-000000000201','Thầu phụ điện An Tâm','VND',950000000,950000000,20,4,180,'2026-03-01','2026-03-05','2026-12-31','active','10000000-0000-0000-0000-000000000003','10000000-0000-0000-0000-000000000007','10000000-0000-0000-0000-000000000007','10000000-0000-0000-0000-000000000007','2026-03-05 08:00+07'),
('65000000-0000-0000-0000-000000000203','30000000-0000-0000-0000-000000000020','HD-GOLD-NCC-001','Hợp đồng cung cấp vật tư hoàn thiện','supplier','payable','90000000-0000-0000-0000-000000000103','Dulux','VND',444500000,444500000,0,0,365,'2026-02-25','2026-02-25','2026-09-30','active','10000000-0000-0000-0000-000000000003','10000000-0000-0000-0000-000000000007','10000000-0000-0000-0000-000000000007','10000000-0000-0000-0000-000000000007','2026-02-25 08:00+07');

INSERT INTO contract_variations (
  id,contract_id,variation_code,title,variation_type,reason,amount_delta,days_delta,
  scope_delta,document_id,status,submitted_by,submitted_at,approved_by,approved_at,
  incorporated_by,incorporated_at,created_by
) VALUES
('65200000-0000-0000-0000-000000000201','65000000-0000-0000-0000-000000000201','PL-GOLD-001','Bổ sung khu pantry và hệ đèn trang trí','mixed','Chủ đầu tư bổ sung phạm vi sau duyệt mockup.',180000000,10,'Bổ sung pantry, đèn trang trí và kéo dài 10 ngày.','46000000-0000-0000-0000-000000000202','approved','10000000-0000-0000-0000-000000000002','2026-03-20 09:00+07','10000000-0000-0000-0000-000000000007','2026-03-21 09:00+07','10000000-0000-0000-0000-000000000007','2026-03-22 09:00+07','10000000-0000-0000-0000-000000000002');

INSERT INTO contract_versions (
  id,contract_id,version_no,version_label,source_variation_id,amount,scope_summary,
  commercial_terms_json,payment_terms_json,planned_start_date,planned_end_date,document_id,
  definition_hash,status,review_comment,signature_comment,submitted_by,submitted_at,
  approved_by,approved_at,signed_by,signed_at,created_by
) VALUES
('65100000-0000-0000-0000-000000000201','65000000-0000-0000-0000-000000000201',1,'Rev.01 – Hợp đồng gốc',NULL,4500000000,'Hoàn thiện MEP, trần, sơn và sàn tầng 12.','{"vat":10,"warranty_days":365}','{"advance_percent":30,"progress_percent":45,"final_percent":25}','2026-03-01','2026-06-20','46000000-0000-0000-0000-000000000202',repeat('c',64),'signed','Pháp lý và kế toán đã review.','Hai bên ký đầy đủ.','10000000-0000-0000-0000-000000000002','2026-02-26 09:00+07','10000000-0000-0000-0000-000000000007','2026-02-27 09:00+07','10000000-0000-0000-0000-000000000007','2026-02-28 09:00+07','10000000-0000-0000-0000-000000000002'),
('65100000-0000-0000-0000-000000000202','65000000-0000-0000-0000-000000000201',2,'Rev.02 – Phụ lục pantry','65200000-0000-0000-0000-000000000201',4680000000,'Hợp đồng gốc cộng pantry và đèn trang trí.','{"vat":10,"warranty_days":365,"variation":"PL-GOLD-001"}','{"advance_percent":30,"progress_percent":45,"final_percent":25}','2026-03-01','2026-06-30','46000000-0000-0000-0000-000000000202',repeat('d',64),'signed','Đã kiểm tra phát sinh và tiến độ.','Hai bên ký phụ lục Rev.02.','10000000-0000-0000-0000-000000000002','2026-03-22 09:00+07','10000000-0000-0000-0000-000000000007','2026-03-23 09:00+07','10000000-0000-0000-0000-000000000007','2026-03-24 09:00+07','10000000-0000-0000-0000-000000000002'),
('65100000-0000-0000-0000-000000000203','65000000-0000-0000-0000-000000000202',1,'Rev.01 – Thầu phụ MEP',NULL,950000000,'Thi công MEP âm trần và test vận hành.','{"vat":10,"warranty_days":180}','{"advance_percent":20,"retention_percent":4}','2026-03-05','2026-05-31',NULL,repeat('e',64),'signed','Đủ năng lực và biện pháp.','Đã ký giữa công ty và thầu phụ.','10000000-0000-0000-0000-000000000003','2026-02-28 09:00+07','10000000-0000-0000-0000-000000000007','2026-03-01 09:00+07','10000000-0000-0000-0000-000000000007','2026-03-01 10:00+07','10000000-0000-0000-0000-000000000003'),
('65100000-0000-0000-0000-000000000204','65000000-0000-0000-0000-000000000203',1,'Rev.01 – Cung cấp vật tư',NULL,444500000,'Cung cấp vật tư theo PO-GOLD-001 và PO-GOLD-002.','{"vat":10,"delivery":"site"}','{"payment":"after_acceptance"}','2026-02-25','2026-04-15',NULL,repeat('f',64),'signed','Đã review giá và CO/CQ.','Đã ký điện tử.','10000000-0000-0000-0000-000000000003','2026-02-23 09:00+07','10000000-0000-0000-0000-000000000007','2026-02-24 09:00+07','10000000-0000-0000-0000-000000000007','2026-02-25 09:00+07','10000000-0000-0000-0000-000000000003');

UPDATE contracts SET current_version_id=CASE id
  WHEN '65000000-0000-0000-0000-000000000201' THEN '65100000-0000-0000-0000-000000000202'::uuid
  WHEN '65000000-0000-0000-0000-000000000202' THEN '65100000-0000-0000-0000-000000000203'::uuid
  ELSE '65100000-0000-0000-0000-000000000204'::uuid END
WHERE project_id='30000000-0000-0000-0000-000000000020';

UPDATE contract_variations
SET status='incorporated',resulting_version_id='65100000-0000-0000-0000-000000000202'
WHERE id='65200000-0000-0000-0000-000000000201';

-- Fully settled AP/AR ledgers. Net milestone obligations plus released retention
-- reconcile exactly to the active signed contract totals.
INSERT INTO payables (
  id,project_id,payable_code,vendor_type,vendor_partner_id,vendor_name,source_type,source_id,
  gross_amount,advance_amount,retention_amount,deduction_amount,paid_amount,remaining_amount,
  due_date,status,created_by,approved_by
) VALUES
('81000000-0000-0000-0000-000000000201','30000000-0000-0000-0000-000000000020','PAY-GOLD-SUPPLIER','supplier','90000000-0000-0000-0000-000000000103','Dulux','contract_milestone','65300000-0000-0000-0000-000000000207',444500000,0,0,0,444500000,0,'2026-04-30','paid','10000000-0000-0000-0000-000000000004','10000000-0000-0000-0000-000000000007'),
('81000000-0000-0000-0000-000000000202','30000000-0000-0000-0000-000000000020','PAY-GOLD-SUB-ADV','subcontractor','90000000-0000-0000-0000-000000000201','Thầu phụ điện An Tâm','contract_milestone','65300000-0000-0000-0000-000000000205',190000000,0,0,0,190000000,0,'2026-03-10','paid','10000000-0000-0000-0000-000000000004','10000000-0000-0000-0000-000000000007'),
('81000000-0000-0000-0000-000000000203','30000000-0000-0000-0000-000000000020','PAY-GOLD-SUB-PROGRESS','subcontractor','90000000-0000-0000-0000-000000000201','Thầu phụ điện An Tâm','contract_milestone','65300000-0000-0000-0000-000000000206',722000000,0,0,0,722000000,0,'2026-06-05','paid','10000000-0000-0000-0000-000000000004','10000000-0000-0000-0000-000000000007'),
('81000000-0000-0000-0000-000000000204','30000000-0000-0000-0000-000000000020','PAY-GOLD-SUB-RETENTION','subcontractor','90000000-0000-0000-0000-000000000201','Thầu phụ điện An Tâm','contract_retention_release','65600000-0000-0000-0000-000000000203',38000000,0,0,0,38000000,0,'2026-06-25','paid','10000000-0000-0000-0000-000000000004','10000000-0000-0000-0000-000000000007');

INSERT INTO receivables (
  id,project_id,receivable_code,client_partner_id,client_name,source_type,source_id,
  contract_amount,milestone_name,gross_amount,received_amount,remaining_amount,due_date,
  status,created_by,approved_by
) VALUES
('82000000-0000-0000-0000-000000000201','30000000-0000-0000-0000-000000000020','REC-GOLD-ADV','90000000-0000-0000-0000-000000000001','Công ty An Phát','contract_milestone','65300000-0000-0000-0000-000000000201',4680000000,'Tạm ứng 30%',1404000000,1404000000,0,'2026-03-05','received','10000000-0000-0000-0000-000000000004','10000000-0000-0000-0000-000000000007'),
('82000000-0000-0000-0000-000000000202','30000000-0000-0000-0000-000000000020','REC-GOLD-PROGRESS','90000000-0000-0000-0000-000000000001','Công ty An Phát','contract_milestone','65300000-0000-0000-0000-000000000202',4680000000,'Thanh toán tiến độ 45%',2000700000,2000700000,0,'2026-05-10','received','10000000-0000-0000-0000-000000000004','10000000-0000-0000-0000-000000000007'),
('82000000-0000-0000-0000-000000000203','30000000-0000-0000-0000-000000000020','REC-GOLD-FINAL','90000000-0000-0000-0000-000000000001','Công ty An Phát','contract_milestone','65300000-0000-0000-0000-000000000203',4680000000,'Quyết toán sau nghiệm thu',1111500000,1111500000,0,'2026-06-15','received','10000000-0000-0000-0000-000000000004','10000000-0000-0000-0000-000000000007'),
('82000000-0000-0000-0000-000000000204','30000000-0000-0000-0000-000000000020','REC-GOLD-RET-01','90000000-0000-0000-0000-000000000001','Công ty An Phát','contract_retention_release','65600000-0000-0000-0000-000000000201',4680000000,'Giải phóng giữ lại tiến độ',105300000,105300000,0,'2026-06-25','received','10000000-0000-0000-0000-000000000004','10000000-0000-0000-0000-000000000007'),
('82000000-0000-0000-0000-000000000205','30000000-0000-0000-0000-000000000020','REC-GOLD-RET-02','90000000-0000-0000-0000-000000000001','Công ty An Phát','contract_retention_release','65600000-0000-0000-0000-000000000202',4680000000,'Giải phóng giữ lại quyết toán',58500000,58500000,0,'2026-06-25','received','10000000-0000-0000-0000-000000000004','10000000-0000-0000-0000-000000000007');

INSERT INTO contract_milestones (
  id,contract_id,contract_version_id,milestone_code,milestone_name,sequence_no,milestone_type,
  amount_basis,percentage,gross_amount,retention_percent,retention_amount,net_amount,planned_date,
  due_days,acceptance_required,acceptance_id,status,obligation_type,obligation_id,eligible_by,eligible_at,created_by
) VALUES
('65300000-0000-0000-0000-000000000201','65000000-0000-0000-0000-000000000201','65100000-0000-0000-0000-000000000202','MS-CDT-ADV','Tạm ứng 30%',1,'advance','percentage',30,1404000000,0,0,1404000000,'2026-03-05',5,false,NULL,'paid','receivable','82000000-0000-0000-0000-000000000201','10000000-0000-0000-0000-000000000007','2026-03-01 09:00+07','10000000-0000-0000-0000-000000000004'),
('65300000-0000-0000-0000-000000000202','65000000-0000-0000-0000-000000000201','65100000-0000-0000-0000-000000000202','MS-CDT-PROG','Tiến độ 45%',2,'progress','percentage',45,2106000000,5,105300000,2000700000,'2026-05-01',10,true,'71000000-0000-0000-0000-000000000201','paid','receivable','82000000-0000-0000-0000-000000000202','10000000-0000-0000-0000-000000000007','2026-06-10 17:00+07','10000000-0000-0000-0000-000000000004'),
('65300000-0000-0000-0000-000000000203','65000000-0000-0000-0000-000000000201','65100000-0000-0000-0000-000000000202','MS-CDT-FINAL','Quyết toán 25%',3,'final','percentage',25,1170000000,5,58500000,1111500000,'2026-06-15',10,true,'71000000-0000-0000-0000-000000000201','paid','receivable','82000000-0000-0000-0000-000000000203','10000000-0000-0000-0000-000000000007','2026-06-10 17:00+07','10000000-0000-0000-0000-000000000004'),
('65300000-0000-0000-0000-000000000205','65000000-0000-0000-0000-000000000202','65100000-0000-0000-0000-000000000203','MS-TP-ADV','Tạm ứng thầu phụ 20%',1,'advance','fixed',20,190000000,0,0,190000000,'2026-03-10',5,false,NULL,'paid','payable','81000000-0000-0000-0000-000000000202','10000000-0000-0000-0000-000000000007','2026-03-05 09:00+07','10000000-0000-0000-0000-000000000004'),
('65300000-0000-0000-0000-000000000206','65000000-0000-0000-0000-000000000202','65100000-0000-0000-0000-000000000203','MS-TP-PROG','Thanh toán sau nghiệm thu',2,'final','fixed',80,760000000,5,38000000,722000000,'2026-06-05',5,true,'71000000-0000-0000-0000-000000000201','paid','payable','81000000-0000-0000-0000-000000000203','10000000-0000-0000-0000-000000000007','2026-06-10 17:00+07','10000000-0000-0000-0000-000000000004'),
('65300000-0000-0000-0000-000000000207','65000000-0000-0000-0000-000000000203','65100000-0000-0000-0000-000000000204','MS-NCC-FULL','Thanh toán sau giao đủ',1,'final','fixed',100,444500000,0,0,444500000,'2026-04-30',10,false,NULL,'paid','payable','81000000-0000-0000-0000-000000000201','10000000-0000-0000-0000-000000000007','2026-04-15 09:00+07','10000000-0000-0000-0000-000000000004');

INSERT INTO contract_retentions (
  id,contract_id,source_milestone_id,direction,held_amount,released_amount,warranty_end_date,
  status,release_obligation_type,release_obligation_id,released_by,released_at,release_comment
) VALUES
('65400000-0000-0000-0000-000000000201','65000000-0000-0000-0000-000000000201','65300000-0000-0000-0000-000000000202','receivable',105300000,105300000,'2026-06-20','released','receivable','82000000-0000-0000-0000-000000000204','10000000-0000-0000-0000-000000000007','2026-06-25 09:00+07','Giải phóng sau xác nhận bảo hành.'),
('65400000-0000-0000-0000-000000000202','65000000-0000-0000-0000-000000000201','65300000-0000-0000-0000-000000000203','receivable',58500000,58500000,'2026-06-20','released','receivable','82000000-0000-0000-0000-000000000205','10000000-0000-0000-0000-000000000007','2026-06-25 09:00+07','Giải phóng phần giữ lại quyết toán.'),
('65400000-0000-0000-0000-000000000203','65000000-0000-0000-0000-000000000202','65300000-0000-0000-0000-000000000206','payable',38000000,38000000,'2026-06-20','released','payable','81000000-0000-0000-0000-000000000204','10000000-0000-0000-0000-000000000007','2026-06-25 09:00+07','Thầu phụ đã hoàn tất bảo hành và hồ sơ.');

INSERT INTO contract_retention_releases (
  id,retention_id,release_no,amount,due_date,comment,obligation_type,obligation_id,released_by,released_at
) VALUES
('65600000-0000-0000-0000-000000000201','65400000-0000-0000-0000-000000000201',1,105300000,'2026-06-25','Giải phóng đủ giữ lại tiến độ.','receivable','82000000-0000-0000-0000-000000000204','10000000-0000-0000-0000-000000000007','2026-06-25 09:00+07'),
('65600000-0000-0000-0000-000000000202','65400000-0000-0000-0000-000000000202',1,58500000,'2026-06-25','Giải phóng đủ giữ lại quyết toán.','receivable','82000000-0000-0000-0000-000000000205','10000000-0000-0000-0000-000000000007','2026-06-25 09:00+07'),
('65600000-0000-0000-0000-000000000203','65400000-0000-0000-0000-000000000203',1,38000000,'2026-06-25','Giải phóng đủ giữ lại thầu phụ.','payable','81000000-0000-0000-0000-000000000204','10000000-0000-0000-0000-000000000007','2026-06-25 09:00+07');

-- Schedules, advances, invoices, payments and allocations are fully reconciled.
INSERT INTO payable_payment_schedules (
  id,project_id,payable_id,schedule_code,milestone_name,schedule_type,planned_amount,
  paid_amount,remaining_amount,due_date,status,notes,created_by,approved_by
)
SELECT gen_random_uuid(),project_id,id,'SCH-'||payable_code,milestone_name,'final',gross_amount,gross_amount,0,due_date,'paid','Đã thanh toán và đối soát Golden.',created_by,approved_by
FROM payables
LEFT JOIN LATERAL (SELECT COALESCE(source_type,'contract') AS milestone_name) x ON true
WHERE project_id='30000000-0000-0000-0000-000000000020';

INSERT INTO receivable_payment_schedules (
  id,project_id,receivable_id,schedule_code,milestone_name,schedule_type,planned_amount,
  received_amount,remaining_amount,due_date,status,notes,created_by,approved_by
)
SELECT gen_random_uuid(),project_id,id,'SCH-'||receivable_code,COALESCE(milestone_name,'Theo hợp đồng'),
       CASE WHEN receivable_code LIKE '%ADV' THEN 'advance' ELSE 'final' END,
       gross_amount,gross_amount,0,due_date,'received','Đã thu và đối soát Golden.',created_by,approved_by
FROM receivables WHERE project_id='30000000-0000-0000-0000-000000000020';

INSERT INTO payments (
  id,project_id,payment_code,payment_type,target_type,target_id,amount,payment_method,
  status,paid_at,created_by,approved_by
)
SELECT gen_random_uuid(),project_id,'PMT-'||payable_code,'supplier_payment','payable',id,gross_amount,
       'bank_transfer','paid','2026-06-25 10:00+07'::timestamptz,created_by,approved_by
FROM payables WHERE project_id='30000000-0000-0000-0000-000000000020'
UNION ALL
SELECT gen_random_uuid(),project_id,'RCV-'||receivable_code,'client_receipt','receivable',id,gross_amount,
       'bank_transfer','received','2026-06-25 10:30+07'::timestamptz,created_by,approved_by
FROM receivables WHERE project_id='30000000-0000-0000-0000-000000000020';

INSERT INTO advance_requests (
  id,project_id,payable_id,request_code,requested_by,amount,reason,status,approved_by,
  settled_amount,vendor_name,due_date,payment_id,paid_amount,paid_at,settled_at,notes
)
SELECT '81200000-0000-0000-0000-000000000201',p.project_id,p.id,'ADV-GOLD-SUB-001',
       '10000000-0000-0000-0000-000000000004',p.gross_amount,'Tạm ứng mobilization thầu phụ',
       'settled','10000000-0000-0000-0000-000000000007',p.gross_amount,p.vendor_name,p.due_date,
       pay.id,p.gross_amount,pay.paid_at,'2026-06-10 10:00+07','Đã quyết toán theo nghiệm thu khối lượng.'
FROM payables p JOIN payments pay ON pay.target_id=p.id
WHERE p.id='81000000-0000-0000-0000-000000000202';

INSERT INTO client_advance_receipts (
  id,project_id,receivable_id,receipt_code,client_name,requested_by,amount,received_amount,
  settled_amount,reason,due_date,payment_id,received_at,settled_at,status,notes,approved_by
)
SELECT '82200000-0000-0000-0000-000000000201',r.project_id,r.id,'CAR-GOLD-001',r.client_name,
       '10000000-0000-0000-0000-000000000004',r.gross_amount,r.gross_amount,r.gross_amount,
       'Tạm ứng 30% theo hợp đồng',r.due_date,p.id,p.paid_at,'2026-06-15 10:00+07','settled',
       'Đã đối trừ đầy đủ vào khối lượng quyết toán.','10000000-0000-0000-0000-000000000007'
FROM receivables r JOIN payments p ON p.target_id=r.id
WHERE r.id='82000000-0000-0000-0000-000000000201';

INSERT INTO documents (id,project_id,target_type,target_id,name,file_url,document_type,status,uploaded_by) VALUES
('46000000-0000-0000-0000-000000000207','30000000-0000-0000-0000-000000000020','input_invoice','85000000-0000-0000-0000-000000000201','Hóa đơn đầu vào Golden','/input-invoices/85000000-0000-0000-0000-000000000201/file','invoice','active','10000000-0000-0000-0000-000000000004'),
('46000000-0000-0000-0000-000000000208','30000000-0000-0000-0000-000000000020','output_invoice','85100000-0000-0000-0000-000000000201','Hóa đơn đầu ra Golden','/output-invoices/85100000-0000-0000-0000-000000000201/file','invoice','active','10000000-0000-0000-0000-000000000004');

INSERT INTO input_invoices (
  id,project_id,payable_id,procurement_request_id,document_id,invoice_number,invoice_symbol,
  invoice_date,vendor_partner_id,vendor_name,vendor_tax_code,amount_before_vat,vat_amount,total_amount,
  status,notes,storage_provider,storage_key,mime_type,file_name,file_size,checksum_sha256,
  uploaded_by,verified_by,verified_at
) VALUES
('85000000-0000-0000-0000-000000000201','30000000-0000-0000-0000-000000000020','81000000-0000-0000-0000-000000000201','60000000-0000-0000-0000-000000000202','46000000-0000-0000-0000-000000000207','INV-GOLD-IN-001','AA/26E','2026-04-20','90000000-0000-0000-0000-000000000103','Dulux','0300000001',404090909,40409091,444500000,'verified','Đã đối chiếu PO, nhập kho và nghiệm thu.','local','input-invoices/golden-project/demo-input-invoice.pdf','application/pdf','golden-input-invoice.pdf',623,'49caf29994137a9eee81cadcb106c6821ae45d052652bbed56944cd19186af16','10000000-0000-0000-0000-000000000004','10000000-0000-0000-0000-000000000004','2026-04-21 09:00+07');

INSERT INTO output_invoices (
  id,project_id,receivable_id,payment_id,document_id,invoice_number,invoice_symbol,invoice_date,
  client_partner_id,client_name,client_tax_code,amount_before_vat,vat_amount,total_amount,status,
  notes,storage_provider,storage_key,mime_type,file_name,file_size,checksum_sha256,
  uploaded_by,issued_by,issued_at
)
SELECT '85100000-0000-0000-0000-000000000201',r.project_id,r.id,p.id,
       '46000000-0000-0000-0000-000000000208','INV-GOLD-OUT-001','AB/26E','2026-06-11',
       r.client_partner_id,r.client_name,'0312345678',1010454545,101045455,1111500000,'issued',
       'Hóa đơn quyết toán sau nghiệm thu cuối.','local','output-invoices/golden-project/demo-output-invoice.pdf',
       'application/pdf','golden-output-invoice.pdf',624,'6d9ca61d3f29fdd585c36e812b5827aa80578e56d9902c45356a5a6a19c0f5ec',
       '10000000-0000-0000-0000-000000000004','10000000-0000-0000-0000-000000000004','2026-06-11 09:00+07'
FROM receivables r JOIN payments p ON p.target_id=r.id
WHERE r.id='82000000-0000-0000-0000-000000000203';

INSERT INTO payment_allocations (
  project_id,payment_id,payable_id,schedule_id,advance_request_id,input_invoice_id,
  allocation_type,amount,notes,created_by
)
SELECT p.project_id,p.id,ap.id,s.id,
       CASE WHEN ap.id='81000000-0000-0000-0000-000000000202' THEN '81200000-0000-0000-0000-000000000201'::uuid END,
       CASE WHEN ap.id='81000000-0000-0000-0000-000000000201' THEN '85000000-0000-0000-0000-000000000201'::uuid END,
       CASE WHEN ap.id='81000000-0000-0000-0000-000000000202' THEN 'advance_payment' ELSE 'payable_payment' END,
       ap.gross_amount,'Đối soát đủ nghĩa vụ Golden.','10000000-0000-0000-0000-000000000004'
FROM payables ap
JOIN payments p ON p.target_id=ap.id
JOIN payable_payment_schedules s ON s.payable_id=ap.id
WHERE ap.project_id='30000000-0000-0000-0000-000000000020';

INSERT INTO receipt_allocations (
  project_id,payment_id,receivable_id,schedule_id,advance_receipt_id,output_invoice_id,
  allocation_type,amount,notes,created_by
)
SELECT p.project_id,p.id,ar.id,s.id,
       CASE WHEN ar.id='82000000-0000-0000-0000-000000000201' THEN '82200000-0000-0000-0000-000000000201'::uuid END,
       CASE WHEN ar.id='82000000-0000-0000-0000-000000000203' THEN '85100000-0000-0000-0000-000000000201'::uuid END,
       CASE WHEN ar.id='82000000-0000-0000-0000-000000000201' THEN 'advance_receipt' ELSE 'receivable_receipt' END,
       ar.gross_amount,'Đối soát đủ khoản thu Golden.','10000000-0000-0000-0000-000000000004'
FROM receivables ar
JOIN payments p ON p.target_id=ar.id
JOIN receivable_payment_schedules s ON s.receivable_id=ar.id
WHERE ar.project_id='30000000-0000-0000-0000-000000000020';

-- Historical material snapshot and submitted surplus handling show how the zero
-- final balance was reached; they do not fabricate an implicit stock movement.
CREATE TEMP TABLE golden_material_work AS
SELECT w.id AS work_item_id,
       (SELECT e.id FROM evidence_records e WHERE e.work_item_id=w.id ORDER BY e.created_at LIMIT 1) AS evidence_id
FROM work_items w
JOIN pipeline_run_steps rs ON rs.id=w.run_step_id
WHERE w.project_id='30000000-0000-0000-0000-000000000020'
  AND rs.surface_kind='quality'
LIMIT 1;

INSERT INTO material_before_after_snapshots (
  id,project_id,work_item_id,status,snapshot_date,construction_area,summary_json,comment,
  applied_to_site_balance,created_by,submitted_by,submitted_at,updated_by
)
SELECT '48000000-0000-0000-0000-000000000201','30000000-0000-0000-0000-000000000020',work_item_id,
       'submitted','2026-05-20','Toàn bộ công trình','{"materials":6,"surplus_returned":true}',
       'Đối chiếu trước/sau thi công và chốt vật tư dư.',true,
       '10000000-0000-0000-0000-000000000006','10000000-0000-0000-0000-000000000006',
       '2026-05-20 16:00+07','10000000-0000-0000-0000-000000000006'
FROM golden_material_work;

INSERT INTO material_before_after_snapshot_items (
  snapshot_id,project_id,work_item_id,item_index,material_id,material_code,material_name,unit,
  source,before_quantity,added_quantity,after_quantity,waste_quantity,used_quantity,
  affects_site_balance,site_balance_id,before_evidence_id,after_evidence_id,
  construction_area,condition,comment,payload_json
)
SELECT '48000000-0000-0000-0000-000000000201',b.project_id,mw.work_item_id,
       row_number() OVER (ORDER BY b.id)-1,m.id,m.code,m.name,m.unit,'site_balance',b.issued_quantity,0,
       0,0,b.used_quantity,true,b.id,mw.evidence_id,mw.evidence_id,'Toàn bộ công trình','usable',
       'Vật tư dư đã hoàn trả về kho trung tâm.',jsonb_build_object('golden_sample',true)
FROM site_material_balances b
JOIN materials m ON m.id=b.material_id
CROSS JOIN golden_material_work mw
WHERE b.project_id='30000000-0000-0000-0000-000000000020';

INSERT INTO site_surplus_handlings (
  id,handling_code,project_id,work_item_id,site_balance_id,material_id,material_snapshot_id,
  material_snapshot_item_id,stock_movement_id,status,handling_type,quantity,unit,warehouse_id,
  custody_location,condition,evidence_id,comment,payload_json,created_by,submitted_by,submitted_at,updated_by
)
SELECT gen_random_uuid(),'SH-GOLD-'||right(m.code,6),b.project_id,mw.work_item_id,b.id,b.material_id,
       '48000000-0000-0000-0000-000000000201',si.id,sm.id,'submitted','return_to_warehouse',
       round(b.planned_quantity*0.03,3),m.unit,'51000000-0000-0000-0000-000000000001',
       'Kho trung tâm – khu vật tư hoàn trả','usable',mw.evidence_id,'Đã trả đủ vật tư dư và đóng balance.',
       '{"golden_sample":true,"closure":"handled"}',
       '10000000-0000-0000-0000-000000000005','10000000-0000-0000-0000-000000000005','2026-05-21 10:00+07','10000000-0000-0000-0000-000000000005'
FROM site_material_balances b
JOIN materials m ON m.id=b.material_id
JOIN material_before_after_snapshot_items si ON si.snapshot_id='48000000-0000-0000-0000-000000000201' AND si.material_id=b.material_id
JOIN stock_movements sm ON sm.project_id=b.project_id AND sm.material_id=b.material_id AND sm.movement_type='return'
CROSS JOIN golden_material_work mw
WHERE b.project_id='30000000-0000-0000-0000-000000000020';

-- Product-grade handover package: exact final acceptance, verified sources,
-- no open blocker and a final readiness snapshot.
CREATE TEMP TABLE golden_handover_source AS
SELECT w.id AS work_item_id,f.id AS form_instance_id
FROM work_items w
JOIN pipeline_run_steps rs ON rs.id=w.run_step_id
JOIN workflow_form_instances f ON f.work_item_id=w.id
WHERE w.project_id='30000000-0000-0000-0000-000000000020'
  AND rs.surface_kind='handover'
LIMIT 1;

INSERT INTO handover_records (
  id,project_id,status,final_acceptance_id,handover_date,client_representative,
  company_representative,documents_ready,warehouse_closed,payables_recorded,receivables_recorded,
  notes,created_by,approved_by,handover_code,workflow_form_instance_id,readiness_snapshot_json,
  submitted_by,submitted_at,checked_by,checked_at,completed_at
)
SELECT '73000000-0000-0000-0000-000000000201','30000000-0000-0000-0000-000000000020',
       'handed_over','71000000-0000-0000-0000-000000000201','2026-06-30',
       'Anh Bình – Công ty An Phát','Nguyễn Minh PM',true,true,true,true,
       'Đã bàn giao, tất toán AP/AR, đóng kho site và kích hoạt bảo hành.',
       '10000000-0000-0000-0000-000000000002','10000000-0000-0000-0000-000000000007',
       'HO-GOLD-001',form_instance_id,
       '{"ready":true,"acceptance":"pass","open_issues":0,"open_ncrs":0,"open_reworks":0,"material_surplus":0,"ap_remaining":0,"ar_remaining":0,"pending_payments":0,"checklist_pending":0}'::jsonb,
       '10000000-0000-0000-0000-000000000002','2026-06-28 09:00+07',
       '10000000-0000-0000-0000-000000000007','2026-06-29 09:00+07','2026-06-30 16:00+07'
FROM golden_handover_source;

INSERT INTO handover_checklist_items (
  handover_id,project_id,item_code,category,title,required,status,source_type,source_id,
  evidence_id,document_id,notes,verified_by,verified_at
) VALUES
('73000000-0000-0000-0000-000000000201','30000000-0000-0000-0000-000000000020','final_acceptance','quality','Biên bản nghiệm thu cuối đạt',true,'verified','acceptance','71000000-0000-0000-0000-000000000201',NULL,NULL,'Final acceptance pass.','10000000-0000-0000-0000-000000000007','2026-06-29 09:00+07'),
('73000000-0000-0000-0000-000000000201','30000000-0000-0000-0000-000000000020','warehouse_closure','operation','Kho công trình và vật tư dư đã đóng',true,'verified','system_check','73000000-0000-0000-0000-000000000201',NULL,NULL,'Site balances bằng 0, surplus handled.','10000000-0000-0000-0000-000000000007','2026-06-29 09:00+07'),
('73000000-0000-0000-0000-000000000201','30000000-0000-0000-0000-000000000020','ap_reconciliation','finance','Công nợ đầu vào đã tất toán',true,'verified','system_check','73000000-0000-0000-0000-000000000201',NULL,NULL,'AP remaining bằng 0.','10000000-0000-0000-0000-000000000007','2026-06-29 09:00+07'),
('73000000-0000-0000-0000-000000000201','30000000-0000-0000-0000-000000000020','ar_reconciliation','finance','Công nợ đầu ra đã tất toán',true,'verified','system_check','73000000-0000-0000-0000-000000000201',NULL,NULL,'AR remaining bằng 0.','10000000-0000-0000-0000-000000000007','2026-06-29 09:00+07'),
('73000000-0000-0000-0000-000000000201','30000000-0000-0000-0000-000000000020','as_built_documents','as_built','Bản vẽ và hồ sơ hoàn công',true,'verified','document','46000000-0000-0000-0000-000000000201',NULL,'46000000-0000-0000-0000-000000000201','Bản vẽ hoàn công active.','10000000-0000-0000-0000-000000000007','2026-06-29 09:00+07'),
('73000000-0000-0000-0000-000000000201','30000000-0000-0000-0000-000000000020','operation_manuals','operation','Hướng dẫn vận hành/bảo trì',true,'verified','document','46000000-0000-0000-0000-000000000203',NULL,'46000000-0000-0000-0000-000000000203','Đã bàn giao khách hàng.','10000000-0000-0000-0000-000000000007','2026-06-29 09:00+07'),
('73000000-0000-0000-0000-000000000201','30000000-0000-0000-0000-000000000020','warranty_documents','warranty','Cam kết và hồ sơ bảo hành',true,'verified','document','46000000-0000-0000-0000-000000000204',NULL,'46000000-0000-0000-0000-000000000204','Bảo hành 365 ngày.','10000000-0000-0000-0000-000000000007','2026-06-29 09:00+07'),
('73000000-0000-0000-0000-000000000201','30000000-0000-0000-0000-000000000020','signed_handover_minutes','legal','Biên bản bàn giao có xác nhận hai bên',true,'verified','document','46000000-0000-0000-0000-000000000205',NULL,'46000000-0000-0000-0000-000000000205','Biên bản hai bên đã ký.','10000000-0000-0000-0000-000000000007','2026-06-29 09:00+07');

INSERT INTO accounting_budgets (
  project_id,budget_code,name,planned_amount,approved_amount,used_amount,remaining_amount,status,approved_by
) VALUES
('30000000-0000-0000-0000-000000000020','BUD-GOLD-001','Ngân sách Golden Project',3200000000,3200000000,1832500000,1367500000,'approved','10000000-0000-0000-0000-000000000007');

INSERT INTO audit_logs (
  project_id,actor_id,actor_name,event_type,target_type,target_id,description,metadata_json,created_at
) VALUES
('30000000-0000-0000-0000-000000000020','10000000-0000-0000-0000-000000000007','Lê Hoàng Giám đốc','golden_financial_closure','project','30000000-0000-0000-0000-000000000020','Đã đối soát hợp đồng, hóa đơn, AP/AR, retention và phê duyệt đóng tài chính.','{"ap_remaining":0,"ar_remaining":0,"pending_payments":0,"golden_sample":true}','2026-06-29 10:00+07'),
('30000000-0000-0000-0000-000000000020','10000000-0000-0000-0000-000000000007','Lê Hoàng Giám đốc','golden_handover_completed','handover','73000000-0000-0000-0000-000000000201','Golden Project đã bàn giao và hoàn tất.','{"readiness":"ready","checklist_verified":8,"golden_sample":true}','2026-06-30 16:00+07');

COMMIT;
