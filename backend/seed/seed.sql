TRUNCATE TABLE
  audit_logs, documents, advance_requests, payments, receivables, payables, accounting_budgets,
  handover_records, replanning_requests, rework_requests, acceptance_checklist_items,
  acceptance_records, issues, operation_daily_reports, site_material_balances, stock_movements,
  procurement_request_items, procurement_requests, boq_items, approval_actions, approval_requests,
  approval_rules, pipeline_run_steps, pipeline_run_nodes, pipeline_runs, project_members, projects,
  pipeline_template_edges, pipeline_template_steps, pipeline_template_nodes, pipeline_templates,
  auth_sessions, user_roles, role_permissions, permissions, roles, staff, users, warehouses, materials, partners,
  system_settings
RESTART IDENTITY CASCADE;

INSERT INTO users (id, email, password_hash, full_name, status) VALUES
('10000000-0000-0000-0000-000000000001','admin@consflow.local','demo:123456','Admin hệ thống','active'),
('10000000-0000-0000-0000-000000000002','pm@consflow.local','demo:123456','Nguyễn Minh PM','active'),
('10000000-0000-0000-0000-000000000003','qs@consflow.local','demo:123456','Trần Hà QS','active'),
('10000000-0000-0000-0000-000000000004','accounting@consflow.local','demo:123456','Lê Mai Kế toán','active'),
('10000000-0000-0000-0000-000000000005','warehouse@consflow.local','demo:123456','Phạm Long Thủ kho','active'),
('10000000-0000-0000-0000-000000000006','supervisor@consflow.local','demo:123456','Đặng Khoa Giám sát','active'),
('10000000-0000-0000-0000-000000000007','director@consflow.local','demo:123456','Võ An Giám đốc','active');

INSERT INTO roles (id, name, display_name, description, system_role) VALUES
('20000000-0000-0000-0000-000000000001','admin','Admin','Quản trị toàn hệ thống',true),
('20000000-0000-0000-0000-000000000002','director','Giám đốc','Duyệt ngân sách và quyết định vượt hạn mức',true),
('20000000-0000-0000-0000-000000000003','pm','PM','Quản lý công trình và pipeline',true),
('20000000-0000-0000-0000-000000000004','qs','QS','Bóc tách khối lượng, BOQ và kiểm soát chi phí',true),
('20000000-0000-0000-0000-000000000005','accounting','Kế toán','Kiểm soát ngân sách, công nợ và thanh toán',true),
('20000000-0000-0000-0000-000000000006','procurement','Mua hàng','Phụ trách yêu cầu mua và nhà cung cấp',true),
('20000000-0000-0000-0000-000000000007','warehouse','Thủ kho','Nhập xuất tồn và vật tư dư',true),
('20000000-0000-0000-0000-000000000008','supervisor','Giám sát','Báo cáo thi công và hiện trường',true),
('20000000-0000-0000-0000-000000000009','qa_qc','QA/QC','Nghiệm thu và kiểm soát chất lượng',true),
('20000000-0000-0000-0000-000000000010','construction_team','Đội thi công','Thực hiện công việc tại công trình',true),
('20000000-0000-0000-0000-000000000011','subcontractor','Thầu phụ','Đơn vị thực hiện theo gói thầu',true),
('20000000-0000-0000-0000-000000000012','client','Chủ đầu tư','Xác nhận thiết kế, nghiệm thu, bàn giao',true),
('20000000-0000-0000-0000-000000000013','viewer','Viewer','Chỉ xem dữ liệu được phân quyền',true);

INSERT INTO permissions (id, key, name, module)
SELECT gen_random_uuid(), key, name, module FROM (VALUES
('project.view','Xem công trình','project'),('project.create','Tạo công trình','project'),('project.update','Cập nhật công trình','project'),('project.delete','Xóa công trình','project'),
('pipeline.view','Xem pipeline','pipeline'),('pipeline.create','Tạo pipeline','pipeline'),('pipeline.edit','Sửa pipeline','pipeline'),('pipeline.publish','Publish pipeline','pipeline'),('pipeline.run','Chạy pipeline','pipeline'),('pipeline.advance_node','Chuyển node','pipeline'),
('planning.view','Xem planning','planning'),('planning.edit','Sửa planning','planning'),
('boq.view','Xem BOQ','boq'),('boq.create','Tạo BOQ','boq'),('boq.update','Sửa BOQ','boq'),('boq.approve','Duyệt BOQ','boq'),
('procurement.view','Xem mua sắm','procurement'),('procurement.create','Tạo yêu cầu mua','procurement'),('procurement.approve','Duyệt mua','procurement'),('procurement.order','Đặt hàng','procurement'),('procurement.receive','Nhận hàng','procurement'),
('warehouse.view','Xem kho','warehouse'),('warehouse.receive','Nhập kho','warehouse'),('warehouse.issue','Xuất kho','warehouse'),('warehouse.transfer','Điều chuyển','warehouse'),('warehouse.adjust','Điều chỉnh kho','warehouse'),('warehouse.surplus_handle','Xử lý vật tư dư','warehouse'),
('operation.view','Xem thi công','operation'),('operation.report','Tạo daily report','operation'),('operation.issue_create','Tạo issue','operation'),('operation.issue_update','Sửa issue','operation'),
('acceptance.view','Xem nghiệm thu','acceptance'),('acceptance.create','Tạo nghiệm thu','acceptance'),('acceptance.approve','Duyệt nghiệm thu','acceptance'),('acceptance.reject','Reject nghiệm thu','acceptance'),
('rework.view','Xem rework','rework'),('rework.create','Tạo rework','rework'),('rework.approve','Duyệt rework','rework'),
('replanning.view','Xem replanning','replanning'),('replanning.create','Tạo replanning','replanning'),('replanning.approve','Duyệt replanning','replanning'),
('handover.view','Xem bàn giao','handover'),('handover.create','Tạo bàn giao','handover'),('handover.approve','Duyệt bàn giao','handover'),
('accounting.view','Xem kế toán','accounting'),('accounting.budget_view','Xem ngân sách','accounting'),('accounting.budget_approve','Duyệt ngân sách','accounting'),('accounting.payable_view','Xem phải trả','accounting'),('accounting.payable_approve','Duyệt phải trả','accounting'),('accounting.receivable_view','Xem phải thu','accounting'),('accounting.receivable_approve','Duyệt phải thu','accounting'),('accounting.payment_create','Tạo thanh toán','accounting'),('accounting.payment_approve','Duyệt thanh toán','accounting'),
('catalog.view','Xem dữ liệu lõi','catalog'),('catalog.manage','Quản lý dữ liệu lõi','catalog'),
('staff.view','Xem nhân sự','staff'),('staff.create','Tạo nhân sự','staff'),('staff.update','Sửa nhân sự','staff'),('role.view','Xem role','rbac'),('role.create','Tạo role','rbac'),('role.update','Sửa role','rbac'),('permission.view','Xem permission','rbac'),('permission.assign','Gán quyền','rbac'),
('audit.view','Xem audit','audit'),('report.view','Xem báo cáo','report'),('report.finance.view','Xem báo cáo tài chính','report'),('settings.view','Xem cấu hình','settings'),('settings.update','Sửa cấu hình','settings')
) AS p(key, name, module);

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r CROSS JOIN permissions p WHERE r.name = 'admin'
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r JOIN permissions p ON p.key IN ('project.view','pipeline.view','pipeline.run','pipeline.advance_node','boq.view','procurement.view','warehouse.view','operation.view','acceptance.view','rework.view','replanning.view','handover.view','accounting.view','audit.view','report.view')
WHERE r.name IN ('director','pm')
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
JOIN permissions p ON (
  (r.name='director' AND p.key IN (
    'project.view','pipeline.view','pipeline.run','pipeline.advance_node',
    'boq.view','boq.approve',
    'procurement.view','procurement.approve',
    'warehouse.view','operation.view',
    'acceptance.view','acceptance.approve','acceptance.reject',
    'rework.view','rework.approve',
    'replanning.view','replanning.approve',
    'handover.view','handover.approve',
    'accounting.view','accounting.budget_view','accounting.budget_approve',
    'accounting.payable_view','accounting.payable_approve',
    'accounting.receivable_view','accounting.receivable_approve',
    'accounting.payment_approve',
    'catalog.view','audit.view','report.view','report.finance.view'
  ))
  OR (r.name='pm' AND p.key IN (
    'project.view','project.update',
    'pipeline.view','pipeline.run','pipeline.advance_node',
    'boq.view','boq.create','boq.update','boq.approve',
    'procurement.view','procurement.create','procurement.approve','procurement.order',
    'warehouse.view',
    'operation.view','operation.report','operation.issue_create','operation.issue_update',
    'acceptance.view','acceptance.create','acceptance.approve','acceptance.reject',
    'rework.view','rework.create','rework.approve',
    'replanning.view','replanning.create','replanning.approve',
    'handover.view','handover.create','handover.approve',
    'accounting.view','accounting.budget_view','accounting.payable_view','accounting.receivable_view',
    'catalog.view','audit.view','report.view'
  ))
  OR (r.name='qs' AND p.key IN (
    'project.view','pipeline.view','boq.view','boq.create','boq.update','boq.approve','procurement.view','accounting.budget_view','catalog.view','report.view'
  ))
  OR (r.name='accounting' AND p.key IN (
    'project.view','pipeline.view','boq.view',
    'procurement.view','procurement.approve',
    'handover.view',
    'accounting.view','accounting.budget_view','accounting.budget_approve',
    'accounting.payable_view','accounting.payable_approve',
    'accounting.receivable_view','accounting.receivable_approve',
    'accounting.payment_create','accounting.payment_approve',
    'catalog.view','catalog.manage','audit.view','report.view','report.finance.view'
  ))
  OR (r.name='procurement' AND p.key IN (
    'project.view','pipeline.view','procurement.view','procurement.create','procurement.approve','procurement.order','procurement.receive','warehouse.view','catalog.view','catalog.manage','report.view'
  ))
  OR (r.name='warehouse' AND p.key IN (
    'project.view','pipeline.view','procurement.view','procurement.receive',
    'warehouse.view','warehouse.receive','warehouse.issue','warehouse.transfer','warehouse.adjust','warehouse.surplus_handle',
    'catalog.view','report.view'
  ))
  OR (r.name='supervisor' AND p.key IN (
    'project.view','pipeline.view','warehouse.view',
    'operation.view','operation.report','operation.issue_create','operation.issue_update','catalog.view',
    'acceptance.view','rework.view','rework.create','report.view'
  ))
  OR (r.name='qa_qc' AND p.key IN (
    'project.view','pipeline.view','operation.view',
    'acceptance.view','acceptance.create','acceptance.approve','acceptance.reject',
    'rework.view','rework.create','catalog.view','report.view'
  ))
  OR (r.name='construction_team' AND p.key IN (
    'project.view','operation.view','operation.report','operation.issue_create','warehouse.view'
  ))
  OR (r.name='subcontractor' AND p.key IN (
    'project.view','operation.view','operation.report','operation.issue_create','warehouse.view'
  ))
  OR (r.name='client' AND p.key IN (
    'project.view','acceptance.view','handover.view'
  ))
  OR (r.name='viewer' AND p.key IN (
    'project.view','pipeline.view','operation.view','acceptance.view','handover.view','report.view'
  ))
)
ON CONFLICT DO NOTHING;

INSERT INTO staff (id, user_id, full_name, email, phone, department, position, status) VALUES
('21000000-0000-0000-0000-000000000001','10000000-0000-0000-0000-000000000001','Admin hệ thống','admin@consflow.local','0900000001','Công nghệ','Admin','active'),
('21000000-0000-0000-0000-000000000002','10000000-0000-0000-0000-000000000002','Nguyễn Minh PM','pm@consflow.local','0900000002','Dự án','Project Manager','active'),
('21000000-0000-0000-0000-000000000003','10000000-0000-0000-0000-000000000003','Trần Hà QS','qs@consflow.local','0900000003','QS','Quantity Surveyor','active'),
('21000000-0000-0000-0000-000000000004','10000000-0000-0000-0000-000000000004','Lê Mai Kế toán','accounting@consflow.local','0900000004','Kế toán','Accountant','active'),
('21000000-0000-0000-0000-000000000005','10000000-0000-0000-0000-000000000005','Phạm Long Thủ kho','warehouse@consflow.local','0900000005','Kho','Warehouse Lead','active'),
('21000000-0000-0000-0000-000000000006','10000000-0000-0000-0000-000000000006','Đặng Khoa Giám sát','supervisor@consflow.local','0900000006','Thi công','Site Supervisor','active'),
('21000000-0000-0000-0000-000000000007','10000000-0000-0000-0000-000000000007','Võ An Giám đốc','director@consflow.local','0900000007','Ban giám đốc','Director','active');

INSERT INTO user_roles (user_id, role_id)
SELECT u.id, r.id FROM users u JOIN roles r ON
  (u.email='admin@consflow.local' AND r.name='admin') OR
  (u.email='pm@consflow.local' AND r.name='pm') OR
  (u.email='qs@consflow.local' AND r.name='qs') OR
  (u.email='accounting@consflow.local' AND r.name='accounting') OR
  (u.email='warehouse@consflow.local' AND r.name='warehouse') OR
  (u.email='supervisor@consflow.local' AND r.name='supervisor') OR
  (u.email='director@consflow.local' AND r.name='director');

INSERT INTO pipeline_templates (id, name, description, project_type, version, status, created_by) VALUES
('40000000-0000-0000-0000-000000000001','Template hoàn thiện nội thất tiêu chuẩn','Khảo sát → Thiết kế → BOQ → Mua sắm → Kho → Thi công → Nghiệm thu → Bàn giao → Financial Closure','Hoàn thiện nội thất',1,'published','10000000-0000-0000-0000-000000000001');

INSERT INTO pipeline_template_nodes (id, template_id, name, description, type, order_index, position_x, position_y, responsible_role_ids, required_permission, exit_conditions, approval_rule, rework_enabled, replanning_enabled, fail_path) VALUES
('41000000-0000-0000-0000-000000000001','40000000-0000-0000-0000-000000000001','Khảo sát','Ghi nhận hiện trạng, kích thước và rủi ro ban đầu.','survey',1,80,120,'["Giám sát"]','planning.edit','{"text":"Có biên bản khảo sát và ảnh hiện trạng"}','PM duyệt',false,true,'previous'),
('41000000-0000-0000-0000-000000000002','40000000-0000-0000-0000-000000000001','Thiết kế','Chuẩn bị bản vẽ, phương án thiết kế và xác nhận với khách hàng.','design',2,320,120,'["PM"]','planning.edit','{"text":"Bản vẽ được khách hàng xác nhận"}','Chủ đầu tư xác nhận',false,true,'replanning'),
('41000000-0000-0000-0000-000000000003','40000000-0000-0000-0000-000000000001','BOQ / Dự toán','QS bóc tách khối lượng và kiểm tra ngân sách.','boq',3,560,120,'["QS"]','boq.update','{"text":"BOQ đã duyệt và ngân sách đã kiểm tra"}','PM + QS + Kế toán duyệt',false,true,'replanning'),
('41000000-0000-0000-0000-000000000004','40000000-0000-0000-0000-000000000001','Duyệt ngân sách','Kế toán kiểm tra ngân sách, giám đốc duyệt khi vượt hạn mức.','budget_approval',4,800,120,'["Kế toán","Giám đốc"]','accounting.budget_approve','{"text":"Ngân sách được phê duyệt"}','PM + Kế toán + Giám đốc duyệt',false,true,'replanning'),
('41000000-0000-0000-0000-000000000005','40000000-0000-0000-0000-000000000001','Mua vật tư','Tạo yêu cầu mua, duyệt mua và đặt hàng.','procurement',5,1040,120,'["Mua hàng","Kế toán"]','procurement.approve','{"text":"Vật tư cần mua đã được duyệt"}','PM + Kế toán duyệt',false,false,'previous'),
('41000000-0000-0000-0000-000000000006','40000000-0000-0000-0000-000000000001','Xuất kho','Kiểm tra tồn kho, xuất vật tư ra công trình.','warehouse_issue',6,1280,120,'["Thủ kho"]','warehouse.issue','{"text":"Vật tư required đã xuất đủ"}','Thủ kho xác nhận',false,false,'previous'),
('41000000-0000-0000-0000-000000000007','40000000-0000-0000-0000-000000000001','Thi công','Daily report, issue, vật tư sử dụng và tiến độ.','construction',7,1520,120,'["Giám sát","Đội thi công"]','operation.report','{"client_payment_min_percent":30,"materials_required":true}','PM duyệt',true,true,'rework'),
('41000000-0000-0000-0000-000000000008','40000000-0000-0000-0000-000000000001','Nghiệm thu','Checklist nghiệm thu, pass/fail, punch list.','acceptance',8,1760,120,'["QA/QC","Chủ đầu tư"]','acceptance.approve','{"text":"Nghiệm thu đạt và punch list đã đóng"}','QA/QC + Chủ đầu tư xác nhận',true,true,'rework'),
('41000000-0000-0000-0000-000000000009','40000000-0000-0000-0000-000000000001','Bàn giao','Closure checker, biên bản bàn giao, chứng từ.','handover',9,2000,120,'["PM","Chủ đầu tư"]','handover.approve','{"text":"Nghiệm thu cuối pass, kho và công nợ đã closure"}','PM + Chủ đầu tư xác nhận',false,false,'previous'),
('41000000-0000-0000-0000-000000000010','40000000-0000-0000-0000-000000000001','Financial Closure','Đóng công nợ, thanh toán và báo cáo lãi/lỗ.','financial_closure',10,2240,120,'["Kế toán","Giám đốc"]','accounting.payment_approve','{"text":"Payables/Receivables đã resolve hoặc duyệt pending"}','Kế toán + Giám đốc duyệt',false,false,'none');

INSERT INTO pipeline_template_edges (template_id, from_node_id, to_node_id, condition_type, label)
SELECT '40000000-0000-0000-0000-000000000001', a.id, b.id, 'always', 'Hoàn tất'
FROM pipeline_template_nodes a
JOIN pipeline_template_nodes b ON b.order_index = a.order_index + 1
WHERE a.template_id='40000000-0000-0000-0000-000000000001' AND b.template_id='40000000-0000-0000-0000-000000000001';

INSERT INTO pipeline_template_steps (node_id, name, description, step_type, order_index)
SELECT id, step_name, 'Checklist mẫu của node ' || n.name, step_type, idx FROM pipeline_template_nodes n
JOIN (VALUES
('Khảo sát',1,'Upload ảnh hiện trạng','photo_upload'),('Khảo sát',2,'Ghi nhận kích thước chính','checklist'),('Khảo sát',3,'Xác định rủi ro mặt bằng','checklist'),
('Thiết kế',1,'Upload bản vẽ','document_upload'),('Thiết kế',2,'Review phương án thiết kế','task'),('Thiết kế',3,'Khách hàng xác nhận','approval'),
('BOQ / Dự toán',1,'Bóc tách khối lượng','task'),('BOQ / Dự toán',2,'Nhập đơn giá dự kiến','task'),('BOQ / Dự toán',3,'Kiểm tra ngân sách','budget_check'),
('Duyệt ngân sách',1,'Tổng hợp ngân sách','budget_check'),('Duyệt ngân sách',2,'Kế toán review','approval'),('Duyệt ngân sách',3,'Giám đốc duyệt nếu vượt hạn mức','approval'),
('Mua vật tư',1,'Sinh nhu cầu vật tư từ BOQ','material_check'),('Mua vật tư',2,'PM duyệt nhu cầu','approval'),('Mua vật tư',3,'Đặt hàng nhà cung cấp','task'),
('Xuất kho',1,'Kiểm tra tồn kho','material_check'),('Xuất kho',2,'Xuất vật tư ra công trình','task'),('Xuất kho',3,'Cập nhật tồn tại site','task'),
('Thi công',1,'Daily report','task'),('Thi công',2,'Ghi nhận vật tư sử dụng','material_check'),('Thi công',3,'Kiểm tra chất lượng trước nghiệm thu','acceptance_check'),
('Nghiệm thu',1,'Tạo checklist nghiệm thu','acceptance_check'),('Nghiệm thu',2,'Upload ảnh minh chứng','photo_upload'),('Nghiệm thu',3,'Pass/Fail nghiệm thu','approval'),
('Bàn giao',1,'Kiểm tra punch list đã đóng','checklist'),('Bàn giao',2,'Chuẩn bị biên bản bàn giao','document_upload'),('Bàn giao',3,'Khách hàng xác nhận','approval'),
('Financial Closure',1,'Ghi nhận phải trả/phải thu','accounting_check'),('Financial Closure',2,'Đối chiếu thanh toán','accounting_check'),('Financial Closure',3,'Đóng tài chính công trình','approval')
) s(node_name, idx, step_name, step_type) ON s.node_name = n.name
WHERE n.template_id = '40000000-0000-0000-0000-000000000001';

INSERT INTO partners (id, partner_code, name, partner_types, tax_code, contact_name, phone, address, payment_terms_days, notes) VALUES
('90000000-0000-0000-0000-000000000001','PT-ANPHAT','Công ty An Phát','["client"]','0312345678','Anh Bình','0909000001','Quận 1, TP.HCM',30,'Chủ đầu tư demo'),
('90000000-0000-0000-0000-000000000002','PT-ANHMINH','Anh Minh','["client"]',NULL,'Anh Minh','0909000002','TP. Thủ Đức, TP.HCM',15,'Khách hàng cá nhân demo'),
('90000000-0000-0000-0000-000000000003','PT-CHIHANH','Chị Hạnh','["client"]',NULL,'Chị Hạnh','0909000003','Masteri An Phú, TP.HCM',15,'Khách hàng cá nhân demo'),
('90000000-0000-0000-0000-000000000101','PT-CADIVI','Cadivi','["supplier"]','0300427407','Sale Cadivi','0909100001','TP.HCM',30,'Nhà cung cấp dây điện'),
('90000000-0000-0000-0000-000000000102','PT-BINHMINH','Bình Minh','["supplier"]','0301464823','Sale Bình Minh','0909100002','TP.HCM',30,'Nhà cung cấp ống nước'),
('90000000-0000-0000-0000-000000000103','PT-DULUX','Dulux','["supplier"]',NULL,'Sale Dulux','0909100003','TP.HCM',30,'Nhà cung cấp sơn'),
('90000000-0000-0000-0000-000000000104','PT-VINHTUONG','Vĩnh Tường','["supplier"]',NULL,'Sale Vĩnh Tường','0909100004','TP.HCM',30,'Nhà cung cấp trần thạch cao'),
('90000000-0000-0000-0000-000000000105','PT-PRIME','Prime','["supplier"]',NULL,'Sale Prime','0909100005','TP.HCM',30,'Nhà cung cấp gạch'),
('90000000-0000-0000-0000-000000000106','PT-RANGDONG','Rạng Đông','["supplier"]',NULL,'Sale Rạng Đông','0909100006','TP.HCM',30,'Nhà cung cấp đèn'),
('90000000-0000-0000-0000-000000000107','PT-TONGHOP','Nhà cung cấp tổng hợp','["supplier","service_provider"]',NULL,'Sale tổng hợp','0909100007','TP.HCM',15,'Nhà cung cấp phụ kiện'),
('90000000-0000-0000-0000-000000000108','PT-APOLLO','Apollo','["supplier"]',NULL,'Sale Apollo','0909100008','TP.HCM',30,'Nhà cung cấp keo'),
('90000000-0000-0000-0000-000000000201','PT-ANTAM','Thầu phụ điện An Tâm','["subcontractor"]',NULL,'Anh Tâm','0909200001','TP.HCM',20,'Thầu phụ điện'),
('90000000-0000-0000-0000-000000000202','PT-MINHLONG','Đội sơn Minh Long','["labor_team","subcontractor"]',NULL,'Anh Long','0909200002','TP.HCM',15,'Đội sơn demo');

INSERT INTO projects (id, name, code, partner_id, client_name, address, planned_start_date, planned_end_date, estimated_budget, contract_amount, status, pipeline_template_id, pm_staff_id, progress_percent) VALUES
('30000000-0000-0000-0000-000000000001','Công trình Fit-out Văn phòng Quận 1','CF-Q1-001','90000000-0000-0000-0000-000000000001','Công ty An Phát','Quận 1, TP.HCM','2026-06-01','2026-08-30',850000000,1200000000,'in_progress','40000000-0000-0000-0000-000000000001','21000000-0000-0000-0000-000000000002',42),
('30000000-0000-0000-0000-000000000002','Cải tạo Nhà phố Thủ Đức','CT-TD-002','90000000-0000-0000-0000-000000000002','Anh Minh','TP. Thủ Đức, TP.HCM','2026-05-15','2026-07-20',420000000,650000000,'blocked','40000000-0000-0000-0000-000000000001','21000000-0000-0000-0000-000000000002',28),
('30000000-0000-0000-0000-000000000003','Hoàn thiện Căn hộ Masteri','HT-MA-003','90000000-0000-0000-0000-000000000003','Chị Hạnh','Masteri An Phú, TP.HCM','2026-06-10','2026-07-30',260000000,390000000,'planning','40000000-0000-0000-0000-000000000001','21000000-0000-0000-0000-000000000002',18);

INSERT INTO project_members (project_id, staff_id, role_ids, assigned_nodes)
SELECT p.id, s.id, jsonb_build_array(r.display_name), '[]'::jsonb
FROM projects p
JOIN staff s ON s.id IN ('21000000-0000-0000-0000-000000000002','21000000-0000-0000-0000-000000000003','21000000-0000-0000-0000-000000000004','21000000-0000-0000-0000-000000000005','21000000-0000-0000-0000-000000000006')
JOIN roles r ON (s.email='pm@consflow.local' AND r.name='pm') OR (s.email='qs@consflow.local' AND r.name='qs') OR (s.email='accounting@consflow.local' AND r.name='accounting') OR (s.email='warehouse@consflow.local' AND r.name='warehouse') OR (s.email='supervisor@consflow.local' AND r.name='supervisor');

INSERT INTO materials (id, code, name, unit, category, default_supplier_partner_id, default_supplier, standard_price) VALUES
('50000000-0000-0000-0000-000000000001','MAT-CADIVI','Dây điện Cadivi','m','Điện','90000000-0000-0000-0000-000000000101','Cadivi',12000),
('50000000-0000-0000-0000-000000000002','MAT-PVC-BM','Ống PVC Bình Minh','m','Nước','90000000-0000-0000-0000-000000000102','Bình Minh',28000),
('50000000-0000-0000-0000-000000000003','MAT-DULUX','Sơn Dulux','thùng','Sơn','90000000-0000-0000-0000-000000000103','Dulux',1450000),
('50000000-0000-0000-0000-000000000004','MAT-GYPSUM','Tấm thạch cao','tấm','Trần','90000000-0000-0000-0000-000000000104','Vĩnh Tường',165000),
('50000000-0000-0000-0000-000000000005','MAT-TILE','Gạch lát nền','m2','Sàn','90000000-0000-0000-0000-000000000105','Prime',320000),
('50000000-0000-0000-0000-000000000006','MAT-LED','Đèn LED âm trần','cái','Điện','90000000-0000-0000-0000-000000000106','Rạng Đông',185000),
('50000000-0000-0000-0000-000000000007','MAT-SCREW','Vít, tắc kê','kg','Phụ kiện','90000000-0000-0000-0000-000000000107','Nhà cung cấp tổng hợp',65000),
('50000000-0000-0000-0000-000000000008','MAT-SILICONE','Keo silicone','chai','Phụ kiện','90000000-0000-0000-0000-000000000108','Apollo',78000);

INSERT INTO warehouses (id, name, location, type) VALUES
('51000000-0000-0000-0000-000000000001','Kho trung tâm Bình Tân','Bình Tân, TP.HCM','central'),
('51000000-0000-0000-0000-000000000002','Kho site Fit-out Q1','Quận 1, TP.HCM','site');

INSERT INTO boq_items (project_id, category, work_item_name, description, unit, quantity, unit_price, material_id, status, created_by, approved_by) VALUES
('30000000-0000-0000-0000-000000000001','Điện','Đi dây điện khu văn phòng','Dây Cadivi âm trần','m',1200,12000,'50000000-0000-0000-0000-000000000001','approved','10000000-0000-0000-0000-000000000003','10000000-0000-0000-0000-000000000002'),
('30000000-0000-0000-0000-000000000001','Nước','Lắp ống nước pantry','Ống PVC Bình Minh','m',260,28000,'50000000-0000-0000-0000-000000000002','approved','10000000-0000-0000-0000-000000000003','10000000-0000-0000-0000-000000000002'),
('30000000-0000-0000-0000-000000000001','Sơn','Sơn hoàn thiện tường','Sơn Dulux nội thất','thùng',36,1450000,'50000000-0000-0000-0000-000000000003','submitted','10000000-0000-0000-0000-000000000003',NULL),
('30000000-0000-0000-0000-000000000001','Trần','Thi công trần thạch cao','Tấm thạch cao','tấm',220,165000,'50000000-0000-0000-0000-000000000004','approved','10000000-0000-0000-0000-000000000003','10000000-0000-0000-0000-000000000002'),
('30000000-0000-0000-0000-000000000001','Sàn','Lát gạch khu pantry','Gạch lát nền','m2',85,320000,'50000000-0000-0000-0000-000000000005','draft','10000000-0000-0000-0000-000000000003',NULL),
('30000000-0000-0000-0000-000000000001','Nội thất','Đèn LED âm trần','Đèn LED văn phòng','cái',96,185000,'50000000-0000-0000-0000-000000000006','approved','10000000-0000-0000-0000-000000000003','10000000-0000-0000-0000-000000000002');

INSERT INTO accounting_budgets (project_id, budget_code, name, planned_amount, approved_amount, used_amount, remaining_amount, status, approved_by) VALUES
('30000000-0000-0000-0000-000000000001','BUD-Q1-001','Ngân sách fit-out Q1',850000000,820000000,315000000,505000000,'approved','10000000-0000-0000-0000-000000000007'),
('30000000-0000-0000-0000-000000000002','BUD-TD-002','Ngân sách cải tạo Thủ Đức',420000000,390000000,210000000,180000000,'approved','10000000-0000-0000-0000-000000000007'),
('30000000-0000-0000-0000-000000000003','BUD-MA-003','Ngân sách căn hộ Masteri',260000000,0,0,260000000,'submitted',NULL);

INSERT INTO procurement_requests (id, project_id, request_code, title, requested_by, status, total_estimated_amount, budget_status) VALUES
('60000000-0000-0000-0000-000000000001','30000000-0000-0000-0000-000000000001','PR-Q1-001','Mua vật tư điện nước đợt 1','10000000-0000-0000-0000-000000000002','waiting_approval',116800000,'within_budget'),
('60000000-0000-0000-0000-000000000002','30000000-0000-0000-0000-000000000002','PR-TD-002','Mua vật tư phát sinh cải tạo','10000000-0000-0000-0000-000000000002','submitted',205000000,'budget_exceeded');

INSERT INTO procurement_request_items (request_id, material_id, quantity_planned, quantity_requested, unit, estimated_unit_price, supplier_partner_id, supplier_name, delivery_status, approval_status) VALUES
('60000000-0000-0000-0000-000000000001','50000000-0000-0000-0000-000000000001',1200,1200,'m',12000,'90000000-0000-0000-0000-000000000101','Cadivi','pending','pending'),
('60000000-0000-0000-0000-000000000001','50000000-0000-0000-0000-000000000002',260,260,'m',28000,'90000000-0000-0000-0000-000000000102','Bình Minh','pending','pending'),
('60000000-0000-0000-0000-000000000001','50000000-0000-0000-0000-000000000004',220,220,'tấm',165000,'90000000-0000-0000-0000-000000000104','Vĩnh Tường','pending','pending');

INSERT INTO stock_movements (movement_code, project_id, warehouse_id, material_id, movement_type, quantity, unit, reason, created_by, approved_by) VALUES
('SM-IN-001',NULL,'51000000-0000-0000-0000-000000000001','50000000-0000-0000-0000-000000000001','receive',2000,'m','Nhập kho đầu kỳ','10000000-0000-0000-0000-000000000005','10000000-0000-0000-0000-000000000005'),
('SM-IN-002',NULL,'51000000-0000-0000-0000-000000000001','50000000-0000-0000-0000-000000000003','receive',80,'thùng','Nhập kho đầu kỳ','10000000-0000-0000-0000-000000000005','10000000-0000-0000-0000-000000000005'),
('SM-OUT-001','30000000-0000-0000-0000-000000000001','51000000-0000-0000-0000-000000000002','50000000-0000-0000-0000-000000000001','issue',800,'m','Xuất ra công trình Q1','10000000-0000-0000-0000-000000000005','10000000-0000-0000-0000-000000000002');

INSERT INTO site_material_balances (project_id, material_id, planned_quantity, issued_quantity, used_quantity, surplus_quantity, surplus_status) VALUES
('30000000-0000-0000-0000-000000000001','50000000-0000-0000-0000-000000000001',1200,800,620,180,'pending'),
('30000000-0000-0000-0000-000000000001','50000000-0000-0000-0000-000000000003',36,30,24,6,'store_pending');

INSERT INTO operation_daily_reports (project_id, report_date, reported_by, weather, work_summary, manpower_summary, material_usage_summary, issues_summary, progress_percent, photos) VALUES
('30000000-0000-0000-0000-000000000001',CURRENT_DATE,'10000000-0000-0000-0000-000000000006','Nắng','Thi công trần và kéo dây khu A','8 thợ điện, 4 thợ trần','Dây điện 120m, tấm thạch cao 18 tấm','Thiếu 12 đèn LED âm trần',42,'[]');

INSERT INTO issues (id, project_id, title, description, severity, status, reported_by, requires_rework, requires_replanning) VALUES
('70000000-0000-0000-0000-000000000001','30000000-0000-0000-0000-000000000002','Hiện trạng tường ẩm khác khảo sát','Cần đổi phương án chống thấm và cập nhật BOQ','high','open','10000000-0000-0000-0000-000000000006',false,true),
('70000000-0000-0000-0000-000000000002','30000000-0000-0000-0000-000000000001','Sơn khu pantry chưa đạt','Màu sơn lệch so với mẫu duyệt','medium','in_progress','10000000-0000-0000-0000-000000000006',true,false);

INSERT INTO acceptance_records (id, project_id, type, title, requested_by, status, result, notes) VALUES
('71000000-0000-0000-0000-000000000001','30000000-0000-0000-0000-000000000001','partial','Nghiệm thu trần khu A','10000000-0000-0000-0000-000000000002','completed','conditional_pass','Cần bổ sung ảnh vị trí kỹ thuật'),
('71000000-0000-0000-0000-000000000002','30000000-0000-0000-0000-000000000002','partial','Nghiệm thu chống thấm tầng 1','10000000-0000-0000-0000-000000000002','completed','fail','Không đạt do phát sinh ẩm tường');

INSERT INTO acceptance_checklist_items (acceptance_id, title, description, is_passed, note, photo_required) VALUES
('71000000-0000-0000-0000-000000000001','Bề mặt trần phẳng','Sai số trong ngưỡng cho phép',true,'Đạt',true),
('71000000-0000-0000-0000-000000000001','Vị trí đèn đúng bản vẽ','Kiểm tra tim đèn',true,'Đạt',true),
('71000000-0000-0000-0000-000000000002','Tường khô trước khi sơn','Đo độ ẩm bề mặt',false,'Ẩm vượt ngưỡng',true);

INSERT INTO rework_requests (project_id, issue_id, acceptance_id, title, description, status, due_date, created_by) VALUES
('30000000-0000-0000-0000-000000000001','70000000-0000-0000-0000-000000000002','71000000-0000-0000-0000-000000000001','Sửa màu sơn pantry','Sơn lại theo mẫu đã duyệt','in_progress',CURRENT_DATE + INTERVAL '3 day','10000000-0000-0000-0000-000000000002');

INSERT INTO replanning_requests (project_id, issue_id, reason, impact_scope, budget_impact, timeline_impact, status, created_by) VALUES
('30000000-0000-0000-0000-000000000002','70000000-0000-0000-0000-000000000001','Hiện trạng khác khảo sát, cần cập nhật chống thấm','boq',45000000,5,'submitted','10000000-0000-0000-0000-000000000002');

INSERT INTO handover_records (project_id, status, client_representative, company_representative, documents_ready, warehouse_closed, payables_recorded, receivables_recorded, notes, created_by) VALUES
('30000000-0000-0000-0000-000000000001','checking','Anh Bình','Nguyễn Minh PM',true,false,true,true,'Đang chờ xử lý vật tư dư','10000000-0000-0000-0000-000000000002');

INSERT INTO payables (project_id, payable_code, vendor_type, vendor_partner_id, vendor_name, source_type, gross_amount, advance_amount, retention_amount, deduction_amount, paid_amount, remaining_amount, due_date, status, created_by) VALUES
('30000000-0000-0000-0000-000000000001','PAY-Q1-001','supplier','90000000-0000-0000-0000-000000000101','Cadivi','procurement',120000000,0,0,0,60000000,60000000,CURRENT_DATE + INTERVAL '10 day','partially_paid','10000000-0000-0000-0000-000000000004'),
('30000000-0000-0000-0000-000000000001','PAY-Q1-002','subcontractor','90000000-0000-0000-0000-000000000201','Thầu phụ điện An Tâm','acceptance',100000000,30000000,5000000,0,0,65000000,CURRENT_DATE + INTERVAL '20 day','approved','10000000-0000-0000-0000-000000000004'),
('30000000-0000-0000-0000-000000000002','PAY-TD-001','labor_team','90000000-0000-0000-0000-000000000202','Đội sơn Minh Long','operation',45000000,10000000,0,0,0,35000000,CURRENT_DATE - INTERVAL '3 day','overdue','10000000-0000-0000-0000-000000000004');

INSERT INTO payable_payment_schedules (project_id, payable_id, schedule_code, milestone_name, schedule_type, planned_amount, paid_amount, remaining_amount, due_date, status, notes, created_by, approved_by)
SELECT project_id, id, 'SCH-Q1-001', 'Vật tư điện nước đợt 1', 'progress', 120000000, 60000000, 60000000, CURRENT_DATE + INTERVAL '10 day', 'partially_paid', 'Đợt còn lại sau giao hàng', created_by, '10000000-0000-0000-0000-000000000007'
FROM payables WHERE payable_code='PAY-Q1-001'
UNION ALL
SELECT project_id, id, 'SCH-Q1-002-ADV', 'Ứng trước thầu phụ điện', 'advance', 30000000, 30000000, 0, CURRENT_DATE - INTERVAL '2 day', 'paid', 'Đã duyệt ứng trước khởi công', created_by, '10000000-0000-0000-0000-000000000007'
FROM payables WHERE payable_code='PAY-Q1-002'
UNION ALL
SELECT project_id, id, 'SCH-Q1-002-PROG', 'Thanh toán nghiệm thu điện', 'progress', 65000000, 0, 65000000, CURRENT_DATE + INTERVAL '20 day', 'approved', 'Chờ hồ sơ nghiệm thu và quyết toán ứng', created_by, '10000000-0000-0000-0000-000000000007'
FROM payables WHERE payable_code='PAY-Q1-002';

INSERT INTO advance_requests (project_id, payable_id, request_code, requested_by, amount, reason, status, approved_by, settled_amount, vendor_name, due_date, paid_amount, paid_at, notes)
SELECT project_id, id, 'ADV-Q1-001', created_by, 30000000, 'Ứng trước vật tư và nhân công điện', 'partially_settled', '10000000-0000-0000-0000-000000000007', 10000000, vendor_name, CURRENT_DATE - INTERVAL '2 day', 30000000, now() - INTERVAL '2 day', 'Còn quyết toán sau nghiệm thu kỳ'
FROM payables WHERE payable_code='PAY-Q1-002';

INSERT INTO receivables (project_id, receivable_code, client_partner_id, client_name, source_type, contract_amount, milestone_name, gross_amount, received_amount, remaining_amount, due_date, status, created_by) VALUES
('30000000-0000-0000-0000-000000000001','REC-Q1-001','90000000-0000-0000-0000-000000000001','Công ty An Phát','milestone',1200000000,'Đợt 1 - Tạm ứng 30%',360000000,360000000,0,CURRENT_DATE - INTERVAL '5 day','received','10000000-0000-0000-0000-000000000004'),
('30000000-0000-0000-0000-000000000001','REC-Q1-002','90000000-0000-0000-0000-000000000001','Công ty An Phát','acceptance',1200000000,'Nghiệm thu bán phần',300000000,120000000,180000000,CURRENT_DATE + INTERVAL '12 day','partially_received','10000000-0000-0000-0000-000000000004'),
('30000000-0000-0000-0000-000000000003','REC-MA-001','90000000-0000-0000-0000-000000000003','Chị Hạnh','handover',390000000,'Sau bàn giao',390000000,0,390000000,CURRENT_DATE + INTERVAL '40 day','recorded','10000000-0000-0000-0000-000000000004');

INSERT INTO payments (project_id, payment_code, payment_type, target_type, amount, payment_method, status, paid_at, created_by, approved_by) VALUES
('30000000-0000-0000-0000-000000000001','PMT-Q1-001','supplier_payment','payable',60000000,'bank_transfer','paid',now() - INTERVAL '2 day','10000000-0000-0000-0000-000000000004','10000000-0000-0000-0000-000000000007'),
('30000000-0000-0000-0000-000000000001','RCV-Q1-001','client_receipt','receivable',360000000,'bank_transfer','received',now() - INTERVAL '8 day','10000000-0000-0000-0000-000000000004','10000000-0000-0000-0000-000000000007');

INSERT INTO approval_requests (id, target_type, target_id, project_id, requested_by, status, reason, required_roles) VALUES
('80000000-0000-0000-0000-000000000001','procurement_request','60000000-0000-0000-0000-000000000001','30000000-0000-0000-0000-000000000001','10000000-0000-0000-0000-000000000002','pending','Duyệt mua vật tư điện nước đợt 1','["PM","Kế toán"]'),
('80000000-0000-0000-0000-000000000002','replanning_request',NULL,'30000000-0000-0000-0000-000000000002','10000000-0000-0000-0000-000000000002','pending','Phát sinh chống thấm vượt BOQ','["PM","QS","Kế toán","Giám đốc"]');

INSERT INTO audit_logs (project_id, actor_id, actor_name, event_type, target_type, target_id, description, metadata_json) VALUES
('30000000-0000-0000-0000-000000000001','10000000-0000-0000-0000-000000000002','Nguyễn Minh PM','pipeline_template_published','pipeline_template','40000000-0000-0000-0000-000000000001','Template hoàn thiện nội thất tiêu chuẩn đã được publish','{}'),
('30000000-0000-0000-0000-000000000001','10000000-0000-0000-0000-000000000006','Đặng Khoa Giám sát','material_requested','procurement_request','60000000-0000-0000-0000-000000000001','Tạo yêu cầu mua vật tư điện nước đợt 1','{"amount":116800000}'),
('30000000-0000-0000-0000-000000000001','10000000-0000-0000-0000-000000000005','Phạm Long Thủ kho','warehouse_issued','stock_movement',NULL,'Xuất 800m dây điện Cadivi ra công trình Q1','{"material":"Dây điện Cadivi","quantity":800}'),
('30000000-0000-0000-0000-000000000002','10000000-0000-0000-0000-000000000006','Đặng Khoa Giám sát','replanning_triggered','issue','70000000-0000-0000-0000-000000000001','Issue lớn yêu cầu replanning vì hiện trạng khác khảo sát','{"impact_scope":"boq","budget_impact":45000000}');

INSERT INTO system_settings (key, value_json, description) VALUES
('client_payment_min_percent','{"construction":30}','Tỷ lệ thu tiền tối thiểu trước khi bắt đầu node thi công'),
('surplus_handling_options','["store","return_to_subcontractor"]','Cách xử lý vật tư dư sau thi công');
