TRUNCATE TABLE
  audit_logs,
  project_cash_requests, project_material_costs,
  activity_mentions, activity_read_states,
  receipt_allocations, client_advance_receipts, receivable_payment_schedules,
  payment_allocations, payable_payment_schedules,
  output_invoices, input_invoices,
  incoming_acceptance_items, incoming_acceptance_records,
  site_surplus_handlings,
  material_before_after_snapshot_items, material_before_after_snapshots,
  owner_client_checklist_reconciliation_items, owner_client_checklist_reconciliations,
  subcontractor_checklist_reconciliation_items, subcontractor_checklist_reconciliations,
  workflow_form_pdf_exports, workflow_form_seal_applications, workflow_form_signatures, workflow_form_events,
  workflow_form_answers, workflow_form_instances,
  seal_assets,
  evidence_records, work_item_activities, work_item_assignments, work_items,
  documents, advance_requests, payments, receivables, payables, accounting_budgets,
  handover_records, replanning_requests, rework_requests, acceptance_checklist_items,
  acceptance_records, issues, operation_daily_reports, site_material_balances, stock_movements,
  procurement_request_items, procurement_requests, boq_unit_price_events, boq_items,
  approval_actions, approval_requests, approval_rules,
  pipeline_run_edges, pipeline_run_steps, pipeline_run_nodes, pipeline_runs, project_members, projects,
  pipeline_template_edges, pipeline_template_steps, pipeline_template_nodes, pipeline_templates,
  auth_sessions, user_roles, role_permissions, permissions, roles, staff, warehouses, materials, partners,
  system_settings
RESTART IDENTITY CASCADE;

UPDATE workflow_form_templates SET created_by = NULL WHERE created_by IS NOT NULL;
UPDATE workflow_form_versions SET created_by = NULL WHERE created_by IS NOT NULL;
DELETE FROM users;

INSERT INTO users (id, email, password_hash, full_name, status) VALUES
('10000000-0000-0000-0000-000000000001','admin@consflow.local','demo:123456','Admin hệ thống','active'),
('10000000-0000-0000-0000-000000000002','pm@consflow.local','demo:123456','Nguyễn Minh PM','active'),
('10000000-0000-0000-0000-000000000003','qs@consflow.local','demo:123456','Trần Hà QS','active'),
('10000000-0000-0000-0000-000000000004','accounting@consflow.local','demo:123456','Ngân Kế Toán','active'),
('10000000-0000-0000-0000-000000000005','warehouse@consflow.local','demo:123456','Phạm Long Thủ kho','active'),
('10000000-0000-0000-0000-000000000006','supervisor@consflow.local','demo:123456','Đặng Khoa Giám sát','active'),
('10000000-0000-0000-0000-000000000007','director@consflow.local','demo:123456','Hoàng Long','active'),
('10000000-0000-0000-0000-000000000008','procurement@consflow.local','demo:123456','Ngô Lan Mua hàng','active'),
('10000000-0000-0000-0000-000000000009','qaqc@consflow.local','demo:123456','Phan Huy QA/QC','active'),
('10000000-0000-0000-0000-000000000010','construction@consflow.local','demo:123456','Đội trưởng Nguyễn Tín','active'),
('10000000-0000-0000-0000-000000000011','subcontractor@consflow.local','demo:123456','Đại diện Thầu phụ An Tâm','active'),
('10000000-0000-0000-0000-000000000012','client@consflow.local','demo:123456','Anh Bình Chủ đầu tư','inactive'),
('10000000-0000-0000-0000-000000000013','design@consflow.local','demo:123456','Nguyễn Hà Thiết kế','active'),
('10000000-0000-0000-0000-000000000014','sales@consflow.local','demo:123456','Trần Vy Sale','active'),
('10000000-0000-0000-0000-000000000015','legal@consflow.local','demo:123456','Lê Quang Pháp lý','active'),
('10000000-0000-0000-0000-000000000016','contract@consflow.local','demo:123456','Vũ An Hợp đồng','active'),
('79900000-0000-0000-0000-000000000001','accounting.backfill@system.local','system-disabled','Hệ thống chuyển đổi sổ kế toán','inactive');

INSERT INTO roles (id, name, display_name, description, system_role) VALUES
('20000000-0000-0000-0000-000000000001','admin','Admin','Quản trị toàn hệ thống',true),
('20000000-0000-0000-0000-000000000002','director','Giám đốc','Duyệt ngân sách và quyết định vượt hạn mức',true),
('20000000-0000-0000-0000-000000000003','pm','PM','Quản lý công trình và pipeline',true),
('20000000-0000-0000-0000-000000000004','qs','QS','Bóc tách khối lượng, BOQ và kiểm soát chi phí',true),
('20000000-0000-0000-0000-000000000005','accounting','Kế toán','Ghi nhận chi phí vật tư và theo dõi hồ sơ thu/chi công trình',true),
('20000000-0000-0000-0000-000000000006','procurement','Mua hàng','Phụ trách yêu cầu mua và nhà cung cấp',true),
('20000000-0000-0000-0000-000000000007','warehouse','Thủ kho','Nhập xuất tồn và vật tư dư',true),
('20000000-0000-0000-0000-000000000008','supervisor','Giám sát','Báo cáo thi công và hiện trường',true),
('20000000-0000-0000-0000-000000000009','qa_qc','QA/QC','Nghiệm thu và kiểm soát chất lượng',true),
('20000000-0000-0000-0000-000000000010','construction_team','Đội thi công','Thực hiện công việc tại công trình',true),
('20000000-0000-0000-0000-000000000011','subcontractor','Thầu phụ','Đơn vị thực hiện theo gói thầu',true),
('20000000-0000-0000-0000-000000000012','client','Chủ đầu tư','Xác nhận thiết kế, nghiệm thu, bàn giao',true),
('20000000-0000-0000-0000-000000000013','viewer','Viewer','Chỉ xem dữ liệu được phân quyền',true),
('20000000-0000-0000-0000-000000000014','design','Thiết kế','Phụ trách phương án và hồ sơ thiết kế',true),
('20000000-0000-0000-0000-000000000015','sales','Sale','Phụ trách phối hợp khách hàng, BOQ và hồ sơ thương mại',true),
('20000000-0000-0000-0000-000000000016','legal','Pháp lý','Phụ trách rà soát pháp lý hợp đồng',true),
('20000000-0000-0000-0000-000000000017','contract_officer','Bộ phận HĐ','Phụ trách lập, luân chuyển, ký và lưu hồ sơ hợp đồng',true);

UPDATE roles
SET external_only=(name='client');

INSERT INTO permissions (id, key, name, module)
SELECT gen_random_uuid(), key, name, module FROM (VALUES
('project.view','Xem công trình','project'),('project.create','Tạo công trình','project'),('project.update','Cập nhật công trình','project'),('project.delete','Xóa công trình','project'),('project.member.manage','Quản lý thành viên công trình','project'),
('pipeline.view','Xem pipeline','pipeline'),('pipeline.create','Tạo pipeline','pipeline'),('pipeline.edit','Sửa pipeline','pipeline'),('pipeline.publish','Publish pipeline','pipeline'),('pipeline.run','Chạy pipeline','pipeline'),('pipeline.advance_node','Chuyển node','pipeline'),
('planning.view','Xem planning','planning'),('planning.edit','Cập nhật kế hoạch','planning'),
('boq.view','Xem BOQ','boq'),('boq.create','Tạo BOQ','boq'),('boq.update','Sửa BOQ','boq'),('boq.approve','Duyệt BOQ','boq'),
('procurement.view','Xem mua sắm','procurement'),('procurement.create','Tạo yêu cầu mua','procurement'),('procurement.approve','Duyệt mua','procurement'),('procurement.order','Đặt hàng','procurement'),('procurement.receive','Nhận hàng','procurement'),
('warehouse.view','Xem kho','warehouse'),('warehouse.receive','Nhập kho','warehouse'),('warehouse.issue','Xuất kho','warehouse'),('warehouse.transfer','Điều chuyển','warehouse'),('warehouse.adjust','Điều chỉnh kho','warehouse'),('warehouse.surplus_handle','Xử lý vật tư dư','warehouse'),
('operation.view','Xem thi công','operation'),('operation.report','Tạo daily report','operation'),('operation.issue_create','Tạo issue','operation'),('operation.issue_update','Sửa issue','operation'),
('acceptance.view','Xem nghiệm thu','acceptance'),('acceptance.create','Tạo nghiệm thu','acceptance'),('acceptance.approve','Duyệt nghiệm thu','acceptance'),('acceptance.reject','Reject nghiệm thu','acceptance'),
('rework.view','Xem rework','rework'),('rework.create','Tạo rework','rework'),('rework.approve','Duyệt rework','rework'),
('replanning.view','Xem replanning','replanning'),('replanning.create','Tạo replanning','replanning'),('replanning.approve','Duyệt replanning','replanning'),
('handover.view','Xem bàn giao','handover'),('handover.create','Tạo bàn giao','handover'),('handover.approve','Duyệt bàn giao','handover'),
('accounting.view','Xem kế toán','accounting'),('accounting.budget_view','Xem ngân sách','accounting'),('accounting.budget_approve','Duyệt ngân sách','accounting'),('accounting.payable_view','Xem phải trả','accounting'),('accounting.payable_approve','Duyệt phải trả','accounting'),('accounting.receivable_view','Xem phải thu','accounting'),('accounting.receivable_approve','Duyệt phải thu','accounting'),('accounting.payment_create','Tạo thanh toán','accounting'),('accounting.payment_approve','Duyệt thanh toán','accounting'),
('accounting.journal_view','Xem sổ kế toán','accounting'),('accounting.journal_post','Ghi sổ chứng từ kế toán','accounting'),('accounting.journal_reverse','Lập bút toán đảo','accounting'),('accounting.period_close','Khóa kỳ kế toán','accounting'),('accounting.period_reopen','Mở lại kỳ kế toán','accounting'),('accounting.tax_view','Xem sổ thuế','accounting'),('accounting.tax_manage','Quản lý thuế','accounting'),
('project_finance.view','Xem tài chính công trình','project_finance'),('project_finance.cost_create','Ghi nhận chi phí vật tư','project_finance'),('project_finance.cost_review','Xác nhận chi phí vật tư','project_finance'),('project_finance.request_create','Lập đề nghị thu/chi','project_finance'),('project_finance.request_approve','Duyệt đề nghị thu/chi','project_finance'),('project_finance.process','Xác nhận bàn giao kế toán','project_finance'),('project_finance.report_view','Xem báo cáo quản trị công trình','project_finance'),
('contract.view','Xem hợp đồng','contract'),('contract.create','Tạo và sửa hợp đồng','contract'),('contract.review','Review hợp đồng','contract'),('contract.approve','Duyệt hợp đồng','contract'),('contract.sign','Ký và kích hoạt hợp đồng','contract'),('contract.milestone_manage','Quản lý milestone hợp đồng','contract'),('contract.retention_release','Giải phóng giữ lại','contract'),
('contract.dispatch','Gửi hồ sơ hợp đồng giấy','contract'),('contract.scan_manage','Quản lý bản scan hợp đồng','contract'),('contract.variation.propose','Đề xuất phát sinh thi công','contract'),('contract.variation_qs_review','QS rà soát phát sinh','contract'),('contract.variation_purchase','Mua hàng xử lý phát sinh','contract'),
('catalog.view','Xem dữ liệu lõi','catalog'),('catalog.manage','Quản lý dữ liệu lõi','catalog'),
('staff.view','Xem nhân sự','staff'),('staff.create','Tạo nhân sự','staff'),('staff.update','Sửa nhân sự','staff'),('staff.account_security.manage','Quản trị bảo mật tài khoản nhân sự','staff'),('role.view','Xem role','rbac'),('role.create','Tạo role','rbac'),('role.update','Sửa role','rbac'),('permission.view','Xem permission','rbac'),('permission.assign','Gán quyền','rbac'),
('audit.view','Xem audit','audit'),('report.view','Xem báo cáo','report'),('report.finance.view','Xem báo cáo tài chính','report'),
('statistics.view','Xem thống kê','statistics'),('statistics.export','Xuất dữ liệu thống kê','statistics'),
('work_report.view','Xem báo cáo công việc','work_report'),('work_report.create','Lập báo cáo công việc','work_report'),('work_report.submit','Gửi báo cáo công việc','work_report'),('work_report.review','Xem xét báo cáo công việc','work_report'),('work_report.export','Xuất báo cáo công việc','work_report'),('work_report.manage','Quản lý báo cáo công việc','work_report'),
('settings.view','Xem cấu hình','settings'),('settings.update','Sửa cấu hình','settings'),
('seal.use','Sử dụng con dấu','settings'),('seal.manage','Quản lý con dấu','settings')
) AS p(key, name, module);

UPDATE permissions
SET description='Cập nhật nội dung kế hoạch được phân công'
WHERE key='planning.edit';

UPDATE permissions
SET description='Đặt mật khẩu và email khôi phục đã xác minh cho tài khoản liên kết với nhân sự'
WHERE key='staff.account_security.manage';

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r CROSS JOIN permissions p WHERE r.name = 'admin'
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role_id,permission_id)
SELECT role.id,permission.id
FROM roles role
JOIN permissions permission ON
  (role.name='admin' AND permission.key IN (
    'statistics.view','statistics.export','work_report.view','work_report.create',
    'work_report.submit','work_report.review','work_report.export','work_report.manage'
  )) OR
  (role.name='director' AND permission.key IN (
    'work_report.view','work_report.review','work_report.export','work_report.manage'
  )) OR
  (role.name='pm' AND permission.key IN (
    'work_report.view','work_report.create','work_report.submit','work_report.review',
    'work_report.export','work_report.manage'
  ))
ON CONFLICT DO NOTHING;

DELETE FROM role_permissions role_permission
USING roles role,permissions permission
WHERE role_permission.role_id=role.id
  AND role_permission.permission_id=permission.id
  AND role.name <> 'admin'
  AND (permission.key LIKE 'accounting.%' OR permission.key IN ('accounting.view','report.finance.view'));

INSERT INTO role_permissions (role_id,permission_id)
SELECT role.id,permission.id
FROM roles role
JOIN permissions permission ON
  (role.name='admin' AND permission.key LIKE 'project_finance.%') OR
  (role.name='accounting' AND permission.key IN (
    'project_finance.view','project_finance.cost_create','project_finance.cost_review',
    'project_finance.request_create','project_finance.process','project_finance.report_view'
  )) OR
  (role.name='director' AND permission.key IN (
    'project_finance.view','project_finance.request_approve','project_finance.report_view'
  )) OR
  (role.name='pm' AND permission.key IN (
    'project_finance.view','project_finance.cost_create','project_finance.request_create','project_finance.report_view'
  )) OR
  (role.name='procurement' AND permission.key IN (
    'project_finance.view','project_finance.cost_create','project_finance.request_create'
  )) OR
  (role.name='qs' AND permission.key IN ('project_finance.view','project_finance.report_view'))
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
    'project.view','project.member.manage','pipeline.view','pipeline.run','pipeline.advance_node',
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
    'accounting.journal_view','accounting.journal_reverse',
    'accounting.period_close','accounting.period_reopen','accounting.tax_view',
    'catalog.view','audit.view','report.view','report.finance.view'
  ))
  OR (r.name='pm' AND p.key IN (
    'project.view','project.update','project.member.manage',
    'pipeline.view','pipeline.run','pipeline.advance_node','planning.view','planning.edit',
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
    'accounting.journal_view','accounting.journal_post','accounting.journal_reverse',
    'accounting.period_close','accounting.period_reopen','accounting.tax_view','accounting.tax_manage',
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
    'project.view','pipeline.view','planning.view','planning.edit','operation.view','operation.report','operation.issue_create','warehouse.view'
  ))
  OR (r.name='subcontractor' AND p.key IN (
    'project.view','operation.view','operation.report','operation.issue_create','warehouse.view'
  ))
  OR (r.name='viewer' AND p.key IN (
    'project.view','pipeline.view','operation.view','acceptance.view','handover.view','report.view'
  ))
  OR (r.name='design' AND p.key IN (
    'project.view','pipeline.view','planning.view','planning.edit','operation.view','operation.report'
  ))
  OR (r.name='sales' AND p.key IN (
    'project.view','pipeline.view','planning.view','planning.edit','boq.view','boq.update','boq.approve','contract.view','contract.review'
  ))
  OR (r.name='legal' AND p.key IN (
    'project.view','pipeline.view','contract.view','contract.review'
  ))
  OR (r.name='contract_officer' AND p.key IN (
    'project.view','pipeline.view','contract.view','contract.create','contract.review','contract.dispatch','contract.scan_manage','seal.use'
  ))
  OR (r.name='director' AND p.key IN ('contract.view','contract.review','contract.approve','contract.sign','contract.milestone_manage','contract.retention_release','seal.use'))
  OR (r.name='pm' AND p.key IN ('contract.view','contract.create','contract.review','contract.approve','contract.milestone_manage','contract.dispatch','contract.scan_manage','contract.variation.propose'))
  OR (r.name='qs' AND p.key IN ('contract.view','contract.create','contract.review','contract.milestone_manage','contract.variation_qs_review'))
  OR (r.name='accounting' AND p.key IN ('contract.view','contract.review','contract.milestone_manage','contract.retention_release'))
  OR (r.name='procurement' AND p.key IN ('contract.view','contract.variation_purchase','contract.dispatch','contract.scan_manage'))
  OR (r.name='supervisor' AND p.key='contract.variation.propose')
)
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role_id,permission_id)
SELECT DISTINCT existing.role_id,target.id
FROM role_permissions existing
JOIN permissions legacy ON legacy.id=existing.permission_id AND legacy.key='report.view'
JOIN permissions target ON target.key IN ('statistics.view','statistics.export')
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role_id,permission_id)
SELECT DISTINCT existing.role_id,target.id
FROM role_permissions existing
JOIN permissions source_permission ON source_permission.id=existing.permission_id AND source_permission.key='operation.report'
JOIN permissions target ON target.key IN ('work_report.view','work_report.create','work_report.submit','work_report.export')
ON CONFLICT DO NOTHING;

DELETE FROM role_permissions role_permission
USING roles role
WHERE role_permission.role_id=role.id
  AND role.external_only;

-- Professional accounting remains available only to administrators for historical support.
DELETE FROM role_permissions role_permission
USING roles role,permissions permission
WHERE role_permission.role_id=role.id
  AND role_permission.permission_id=permission.id
  AND role.name <> 'admin'
  AND (permission.key LIKE 'accounting.%' OR permission.key IN ('accounting.view','report.finance.view'));

INSERT INTO staff (id, user_id, full_name, email, phone, department, position, status) VALUES
('21000000-0000-0000-0000-000000000001','10000000-0000-0000-0000-000000000001','Admin hệ thống','admin@consflow.local','0900000001','Công nghệ','Admin','active'),
('21000000-0000-0000-0000-000000000002','10000000-0000-0000-0000-000000000002','Nguyễn Minh PM','pm@consflow.local','0900000002','Dự án','Project Manager','active'),
('21000000-0000-0000-0000-000000000003','10000000-0000-0000-0000-000000000003','Trần Hà QS','qs@consflow.local','0900000003','QS','Quantity Surveyor','active'),
('21000000-0000-0000-0000-000000000004','10000000-0000-0000-0000-000000000004','Ngân Kế Toán','accounting@consflow.local','0900000004','Kế toán','Accountant','active'),
('21000000-0000-0000-0000-000000000005','10000000-0000-0000-0000-000000000005','Phạm Long Thủ kho','warehouse@consflow.local','0900000005','Kho','Warehouse Lead','active'),
('21000000-0000-0000-0000-000000000006','10000000-0000-0000-0000-000000000006','Đặng Khoa Giám sát','supervisor@consflow.local','0900000006','Thi công','Site Supervisor','active'),
('21000000-0000-0000-0000-000000000007','10000000-0000-0000-0000-000000000007','Hoàng Long','director@consflow.local','0900000007','Ban giám đốc','Director','active'),
('21000000-0000-0000-0000-000000000008','10000000-0000-0000-0000-000000000008','Ngô Lan Mua hàng','procurement@consflow.local','0900000008','Mua hàng','Procurement Lead','active'),
('21000000-0000-0000-0000-000000000009','10000000-0000-0000-0000-000000000009','Phan Huy QA/QC','qaqc@consflow.local','0900000009','QA/QC','Quality Engineer','active'),
('21000000-0000-0000-0000-000000000010','10000000-0000-0000-0000-000000000010','Đội trưởng Nguyễn Tín','construction@consflow.local','0900000010','Thi công','Construction Team Lead','active'),
('21000000-0000-0000-0000-000000000011','10000000-0000-0000-0000-000000000011','Đại diện Thầu phụ An Tâm','subcontractor@consflow.local','0900000011','Thầu phụ','Subcontractor Representative','active'),
('21000000-0000-0000-0000-000000000012','10000000-0000-0000-0000-000000000012','Anh Bình Chủ đầu tư','client@consflow.local','0900000012','Chủ đầu tư','Client Representative','inactive'),
('21000000-0000-0000-0000-000000000013','10000000-0000-0000-0000-000000000013','Nguyễn Hà Thiết kế','design@consflow.local','0900000013','Thiết kế','Design Lead','active'),
('21000000-0000-0000-0000-000000000014','10000000-0000-0000-0000-000000000014','Trần Vy Sale','sales@consflow.local','0900000014','Kinh doanh','Sales Lead','active'),
('21000000-0000-0000-0000-000000000015','10000000-0000-0000-0000-000000000015','Lê Quang Pháp lý','legal@consflow.local','0900000015','Pháp lý','Legal Reviewer','active'),
('21000000-0000-0000-0000-000000000016','10000000-0000-0000-0000-000000000016','Vũ An Hợp đồng','contract@consflow.local','0900000016','Hợp đồng','Contract Officer','active');

INSERT INTO user_roles (user_id, role_id)
SELECT u.id, r.id FROM users u JOIN roles r ON
  (u.email='admin@consflow.local' AND r.name='admin') OR
  (u.email='pm@consflow.local' AND r.name='pm') OR
  (u.email='qs@consflow.local' AND r.name='qs') OR
  (u.email='accounting@consflow.local' AND r.name='accounting') OR
  (u.email='warehouse@consflow.local' AND r.name='warehouse') OR
  (u.email='supervisor@consflow.local' AND r.name='supervisor') OR
  (u.email='director@consflow.local' AND r.name='director') OR
  (u.email='procurement@consflow.local' AND r.name='procurement') OR
  (u.email='qaqc@consflow.local' AND r.name='qa_qc') OR
  (u.email='construction@consflow.local' AND r.name='construction_team') OR
  (u.email='subcontractor@consflow.local' AND r.name='subcontractor') OR
  (u.email='client@consflow.local' AND r.name='client') OR
  (u.email='design@consflow.local' AND r.name='design') OR
  (u.email='sales@consflow.local' AND r.name='sales') OR
  (u.email='legal@consflow.local' AND r.name='legal') OR
  (u.email='contract@consflow.local' AND r.name='contract_officer');

INSERT INTO pipeline_templates (
  id, name, description, project_type, version, status, created_by,
  template_family_id, revision_no, engine_version, definition_hash, published_at, published_by
) VALUES
('40000000-0000-0000-0000-000000000001','Template hoàn thiện nội thất tiêu chuẩn','Khảo sát → Thiết kế → BOQ → Mua sắm → Kho → Thi công → Nghiệm thu → Bàn giao → Tài chính công trình','Hoàn thiện nội thất',1,'published','10000000-0000-0000-0000-000000000001','40000000-0000-0000-0000-000000000001',1,1,repeat('1',64),now(),'10000000-0000-0000-0000-000000000001');

INSERT INTO pipeline_template_nodes (id, template_id, name, description, type, order_index, position_x, position_y, responsible_role_ids, required_permission, exit_conditions, approval_rule, rework_enabled, replanning_enabled, fail_path) VALUES
('41000000-0000-0000-0000-000000000001','40000000-0000-0000-0000-000000000001','Khảo sát','Ghi nhận hiện trạng, kích thước và rủi ro ban đầu.','survey',1,80,120,'["Giám sát"]','planning.edit','{"text":"Có biên bản khảo sát và ảnh hiện trạng"}','PM duyệt',false,true,'previous'),
('41000000-0000-0000-0000-000000000002','40000000-0000-0000-0000-000000000001','Thiết kế','Chuẩn bị bản vẽ, phương án thiết kế và xác nhận với khách hàng.','design',2,320,120,'["PM"]','planning.edit','{"text":"Bản vẽ được khách hàng xác nhận"}','Chủ đầu tư xác nhận',false,true,'replanning'),
('41000000-0000-0000-0000-000000000003','40000000-0000-0000-0000-000000000001','BOQ / Dự toán','QS bóc tách khối lượng và lập dự toán công trình.','boq',3,560,120,'["QS"]','boq.update','{"text":"BOQ và dự toán đã được duyệt"}','PM + QS duyệt',false,true,'replanning'),
('41000000-0000-0000-0000-000000000004','40000000-0000-0000-0000-000000000001','Duyệt kế hoạch mua vật tư','PM và Giám đốc kiểm tra giá trị mua vật tư dự kiến trước khi đặt hàng.','budget_approval',4,800,120,'["PM","Giám đốc"]','procurement.approve','{"text":"Kế hoạch mua vật tư được phê duyệt"}','PM + Giám đốc duyệt',false,true,'replanning'),
('41000000-0000-0000-0000-000000000005','40000000-0000-0000-0000-000000000001','Mua vật tư','Tạo yêu cầu mua, duyệt mua và đặt hàng.','procurement',5,1040,120,'["Mua hàng","PM"]','procurement.approve','{"text":"Vật tư cần mua đã được duyệt"}','PM duyệt',false,false,'previous'),
('41000000-0000-0000-0000-000000000006','40000000-0000-0000-0000-000000000001','Xuất kho','Kiểm tra tồn kho, xuất vật tư ra công trình.','warehouse_issue',6,1280,120,'["Thủ kho"]','warehouse.issue','{"text":"Vật tư required đã xuất đủ"}','Thủ kho xác nhận',false,false,'previous'),
('41000000-0000-0000-0000-000000000007','40000000-0000-0000-0000-000000000001','Thi công','Daily report, issue, vật tư sử dụng và tiến độ.','construction',7,1520,120,'["Giám sát","Đội thi công"]','operation.report','{"client_payment_min_percent":30,"materials_required":true}','PM duyệt',true,true,'rework'),
('41000000-0000-0000-0000-000000000008','40000000-0000-0000-0000-000000000001','Nghiệm thu','Checklist nghiệm thu, pass/fail, punch list.','acceptance',8,1760,120,'["QA/QC","Chủ đầu tư"]','acceptance.approve','{"text":"Nghiệm thu đạt và punch list đã đóng"}','QA/QC + Chủ đầu tư xác nhận',true,true,'rework'),
('41000000-0000-0000-0000-000000000009','40000000-0000-0000-0000-000000000001','Bàn giao','Kiểm tra nghiệm thu cuối, vật tư dư và hồ sơ bàn giao.','handover',9,2000,120,'["PM","Chủ đầu tư"]','handover.approve','{"text":"Nghiệm thu cuối, kho và hồ sơ quản trị đã hoàn tất"}','PM + Chủ đầu tư xác nhận',false,false,'previous'),
('41000000-0000-0000-0000-000000000010','40000000-0000-0000-0000-000000000001','Tài chính công trình','Xác nhận chi phí vật tư và đề nghị thu/chi nội bộ đã được xử lý.','financial_closure',10,2240,120,'["Kế toán","Giám đốc"]','project_finance.view','{"text":"Chi phí vật tư và đề nghị thu/chi đã hoàn tất"}','Kế toán + Giám đốc xác nhận',false,false,'none');

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
('BOQ / Dự toán',1,'Bóc tách khối lượng','task'),('BOQ / Dự toán',2,'Nhập đơn giá dự kiến','task'),('BOQ / Dự toán',3,'Kiểm tra dự toán','budget_check'),
('Duyệt kế hoạch mua vật tư',1,'Tổng hợp kế hoạch mua vật tư','budget_check'),('Duyệt kế hoạch mua vật tư',2,'PM kiểm tra giá trị dự kiến','approval'),('Duyệt kế hoạch mua vật tư',3,'Giám đốc duyệt nếu vượt hạn mức','approval'),
('Mua vật tư',1,'Sinh nhu cầu vật tư từ BOQ','material_check'),('Mua vật tư',2,'PM duyệt nhu cầu','approval'),('Mua vật tư',3,'Đặt hàng nhà cung cấp','task'),
('Xuất kho',1,'Kiểm tra tồn kho','material_check'),('Xuất kho',2,'Xuất vật tư ra công trình','task'),('Xuất kho',3,'Cập nhật tồn tại site','task'),
('Thi công',1,'Daily report','task'),('Thi công',2,'Ghi nhận vật tư sử dụng','material_check'),('Thi công',3,'Kiểm tra chất lượng trước nghiệm thu','acceptance_check'),
('Nghiệm thu',1,'Tạo checklist nghiệm thu','acceptance_check'),('Nghiệm thu',2,'Upload ảnh minh chứng','photo_upload'),('Nghiệm thu',3,'Pass/Fail nghiệm thu','approval'),
('Bàn giao',1,'Kiểm tra punch list đã đóng','checklist'),('Bàn giao',2,'Chuẩn bị biên bản bàn giao','document_upload'),('Bàn giao',3,'Khách hàng xác nhận','approval'),
('Tài chính công trình',1,'Xác nhận chi phí mua vật tư','accounting_check'),('Tài chính công trình',2,'Kiểm tra đề nghị thu/chi đã xử lý','accounting_check'),('Tài chính công trình',3,'Đóng hồ sơ tài chính quản trị','approval')
) s(node_name, idx, step_name, step_type) ON s.node_name = n.name
WHERE n.template_id = '40000000-0000-0000-0000-000000000001';

WITH template_seed(code, name, description, form_type) AS (
  VALUES (
    'client_design_branch_decision',
    'Quyết định nhánh thiết kế',
    'Form chốt workflow đi nhánh Có thiết kế hoặc Không thiết kế sau họp báo cáo khảo sát.',
    'generic_submission'
  )
)
INSERT INTO workflow_form_templates (id, code, name, description, form_type, status)
SELECT
  '80000000-0000-0000-0000-000000000020'::uuid,
  code,
  name,
  description,
  form_type,
  'active'
FROM template_seed
ON CONFLICT (code) DO UPDATE
SET name=EXCLUDED.name,
    description=EXCLUDED.description,
    form_type=EXCLUDED.form_type,
    status='active',
    updated_at=now();

INSERT INTO workflow_form_versions (id, template_id, version_no, status, schema_json, validation_json, published_at)
SELECT
  '80000000-0000-0000-0000-000000000021'::uuid,
  id,
  1,
  'published',
  '{"source":"client_official_workflow","purpose":"branch_routing"}'::jsonb,
  '{"routing_field":"needs_design"}'::jsonb,
  now()
FROM workflow_form_templates
WHERE code='client_design_branch_decision'
ON CONFLICT (template_id, version_no) DO UPDATE
SET status='published',
    schema_json=EXCLUDED.schema_json,
    validation_json=EXCLUDED.validation_json,
    published_at=COALESCE(workflow_form_versions.published_at, EXCLUDED.published_at),
    updated_at=now();

WITH decision_version AS (
  SELECT v.id
  FROM workflow_form_templates t
  JOIN workflow_form_versions v ON v.template_id=t.id AND v.version_no=1
  WHERE t.code='client_design_branch_decision'
),
field_seed(field_key, label, field_type, required, options_json, validation_json, help_text, order_index) AS (
  VALUES
    ('needs_design', 'Nhánh xử lý sau khảo sát', 'select', true,
      '[{"value":"yes","label":"Có thiết kế"},{"value":"no","label":"Không thiết kế"}]'::jsonb,
      '{}'::jsonb,
      'Chọn đúng nhánh để workflow chuyển tiếp.',
      10),
    ('decision_note', 'Ghi chú quyết định', 'textarea', false,
      '[]'::jsonb,
      '{}'::jsonb,
      'Tóm tắt lý do chọn nhánh, người chốt và căn cứ họp.',
      20)
)
INSERT INTO workflow_form_fields (form_version_id, field_key, label, field_type, required, options_json, validation_json, help_text, order_index)
SELECT v.id, f.field_key, f.label, f.field_type, f.required, f.options_json, f.validation_json, f.help_text, f.order_index
FROM field_seed f
CROSS JOIN decision_version v
ON CONFLICT (form_version_id, field_key) DO UPDATE
SET label=EXCLUDED.label,
    field_type=EXCLUDED.field_type,
    required=EXCLUDED.required,
    options_json=EXCLUDED.options_json,
    validation_json=EXCLUDED.validation_json,
    help_text=EXCLUDED.help_text,
    order_index=EXCLUDED.order_index;

INSERT INTO pipeline_templates (
  id, name, description, project_type, version, status, created_by,
  template_family_id, revision_no, engine_version, definition_hash, published_at, published_by
)
VALUES (
  '40000000-0000-0000-0000-000000000020',
  '[Client] Workflow hoàn thiện công trình chính thức',
  'Workflow chính thức từ vận hành thực tế của client: khảo sát, nhánh thiết kế/không thiết kế, BOQ, đầu vào bắt buộc, thi công, nghiệm thu, bàn giao và tài chính công trình.',
  'Hoàn thiện công trình',
  1,
  'published',
  NULL,
  '40000000-0000-0000-0000-000000000020',
  1,
  2,
  repeat('2',64),
  now(),
  '10000000-0000-0000-0000-000000000001'
)
ON CONFLICT (id) DO UPDATE
SET name=EXCLUDED.name,
    description=EXCLUDED.description,
    project_type=EXCLUDED.project_type,
    version=EXCLUDED.version,
    status=EXCLUDED.status,
    template_family_id=EXCLUDED.template_family_id,
    revision_no=EXCLUDED.revision_no,
    engine_version=EXCLUDED.engine_version,
    definition_hash=EXCLUDED.definition_hash,
    published_at=EXCLUDED.published_at,
    published_by=EXCLUDED.published_by,
    updated_at=now();

WITH node_seed(id, name, description, type, order_index, position_x, position_y, roles, required_permission, permissions, exit_conditions, approval_rule, rework_enabled, replanning_enabled, fail_path) AS (
  VALUES
    ('41400000-0000-0000-0000-000000000001'::uuid, 'Khảo sát', 'Ghi nhận hiện trạng, kích thước và rủi ro ban đầu. Đầu ra là biên bản khảo sát.', 'survey', 1, 80, 260, '["PM","Giám sát","QS"]'::jsonb, 'planning.edit', '["planning.edit"]'::jsonb, '{"text":"Có yêu cầu chủ đầu tư, ảnh hiện trạng, kích thước, rủi ro và biên bản khảo sát"}'::jsonb, 'PM duyệt khảo sát', false, true, 'previous'),
    ('41400000-0000-0000-0000-000000000002'::uuid, 'Họp báo cáo khảo sát', 'Họp thống nhất vấn đề, lên báo giá, phân việc và chốt nhánh có thiết kế hay không thiết kế.', 'survey', 2, 360, 260, '["PM","QS","Giám sát"]'::jsonb, 'planning.edit', '["planning.edit","boq.view"]'::jsonb, '{"text":"Biên bản họp đã chốt nhánh Có thiết kế hoặc Không thiết kế"}'::jsonb, 'PM chốt biên bản họp', false, true, 'previous'),
    ('41400000-0000-0000-0000-000000000003'::uuid, 'Thiết kế', 'Chuẩn bị bản vẽ, phương án thiết kế và xác nhận với khách hàng.', 'design', 3, 640, 420, '["PM","Chủ đầu tư"]'::jsonb, 'planning.edit', '["planning.edit"]'::jsonb, '{"text":"Bản vẽ mặt bằng, 3D, bản vẽ thi công và chốt chủ đầu tư đã hoàn tất"}'::jsonb, 'PM và chủ đầu tư xác nhận thiết kế', false, true, 'replanning'),
    ('41400000-0000-0000-0000-000000000004'::uuid, 'BOQ (Không thiết kế)', 'Bóc tách, lấy giá và gửi BOQ khi công trình không cần thiết kế.', 'boq', 4, 640, 120, '["QS","PM","Giám đốc"]'::jsonb, 'boq.update', '["boq.view","boq.update","boq.approve"]'::jsonb, '{"text":"BOQ không thiết kế đã được kiểm tra và gửi chủ đầu tư"}'::jsonb, 'QS/PM/GĐ duyệt BOQ không thiết kế', false, true, 'previous'),
    ('41400000-0000-0000-0000-000000000005'::uuid, 'BOQ (Có thiết kế)', 'Gửi BOQ qua chủ đầu tư sau khi hoàn tất thiết kế.', 'boq', 5, 920, 420, '["QS","PM"]'::jsonb, 'boq.update', '["boq.view","boq.update"]'::jsonb, '{"text":"BOQ có thiết kế đã gửi chủ đầu tư"}'::jsonb, 'PM/QS xác nhận BOQ có thiết kế', false, true, 'previous'),
    ('41400000-0000-0000-0000-000000000006'::uuid, 'Đầu Vào', 'Bước bắt buộc để chốt khối lượng, mua hàng, hợp đồng/tạm ứng và rà soát đầu vào.', 'procurement', 6, 1200, 260, '["QS","Mua hàng","PM","Giám đốc"]'::jsonb, 'procurement.create', '["boq.view","procurement.view","procurement.create","procurement.approve"]'::jsonb, '{"mandatory_gate":true,"text":"Đầu vào đã được rà soát và kiểm tra"}'::jsonb, 'PM và Giám đốc rà soát đầu vào', false, false, 'previous'),
    ('41400000-0000-0000-0000-000000000007'::uuid, 'Đàm phán giá', 'Đàm phán giá cùng với chủ đầu tư và các bộ phận liên quan.', 'budget_approval', 7, 1480, 260, '["PM","QS","Giám đốc","Kế toán"]'::jsonb, 'boq.approve', '["boq.view","boq.approve","accounting.budget_view"]'::jsonb, '{"text":"Đơn giá và định mức đã chốt"}'::jsonb, 'GĐ chốt báo cáo đơn giá', false, false, 'previous'),
    ('41400000-0000-0000-0000-000000000008'::uuid, 'Chốt Hợp đồng với chủ đầu tư', 'Lên báo giá, hợp đồng, pháp lý, ký duyệt và gửi bản cứng cho chủ đầu tư.', 'contract', 8, 1760, 260, '["PM","QS","Kế toán","Giám đốc","Chủ đầu tư"]'::jsonb, 'project.update', '["project.update","accounting.view"]'::jsonb, '{"text":"Hợp đồng và hồ sơ tạm ứng với chủ đầu tư đã ký/gửi"}'::jsonb, 'PM/GĐ ký duyệt hợp đồng', false, false, 'previous'),
    ('41400000-0000-0000-0000-000000000009'::uuid, 'QA & QC / Thi công', 'Họp phân công, chốt biên bản, đảm bảo chất lượng, kế hoạch thi công và báo cáo liên tục.', 'construction', 9, 2040, 260, '["PM","QS","Giám sát","QA/QC"]'::jsonb, 'operation.report', '["operation.view","operation.report","operation.issue_create"]'::jsonb, '{"text":"QA/QC và báo cáo thi công đã hoàn tất"}'::jsonb, 'PM/QA review thi công', true, true, 'rework'),
    ('41400000-0000-0000-0000-000000000010'::uuid, 'Nghiệm thu Đầu Vào', 'Nghiệm thu thầu phụ, tổ đội và vật tư; tập hợp hồ sơ nguồn cho đề nghị chi và ghi nhận chi phí mua vật tư.', 'incoming_acceptance', 10, 2320, 260, '["QA/QC","QS","Kế toán","Giám đốc"]'::jsonb, 'acceptance.approve', '["acceptance.view","acceptance.create","acceptance.approve","project_finance.view","project_finance.cost_create","project_finance.request_create"]'::jsonb, '{"text":"Nghiệm thu đầu vào và hồ sơ nguồn tài chính công trình đã hoàn tất"}'::jsonb, 'GĐ ký nghiệm thu hồ sơ nguồn', true, true, 'rework'),
    ('41400000-0000-0000-0000-000000000011'::uuid, 'Nghiệm thu Đầu Ra', 'Nghiệm thu công trình, tập hợp hồ sơ quyết toán và lập đề nghị thu nội bộ theo mốc hợp đồng.', 'acceptance', 11, 2600, 260, '["QA/QC","QS","PM","Giám đốc","Kế toán"]'::jsonb, 'acceptance.approve', '["acceptance.view","acceptance.create","acceptance.approve","project_finance.view","project_finance.request_create"]'::jsonb, '{"text":"Nghiệm thu đầu ra và hồ sơ đề nghị thu đã hoàn tất"}'::jsonb, 'QA/QS/PM/GĐ ký nghiệm thu đầu ra', true, true, 'rework'),
    ('41400000-0000-0000-0000-000000000012'::uuid, 'Kết thúc: Bàn giao + Tài chính công trình', 'Bàn giao công trình và xác nhận hồ sơ tài chính quản trị đã hoàn tất; nghiệp vụ kế toán chuyên môn thực hiện ngoài Consflow.', 'financial_closure', 12, 2880, 260, '["Kế toán","PM","Giám đốc"]'::jsonb, 'project_finance.view', '["handover.view","handover.approve","project_finance.view","project_finance.report_view"]'::jsonb, '{"handover_included":true,"financial_closure_included":true,"text":"Bàn giao và hồ sơ tài chính công trình đã hoàn tất"}'::jsonb, 'Kế toán và Giám đốc xác nhận hồ sơ quản trị', false, false, 'none')
)
INSERT INTO pipeline_template_nodes (
  id, template_id, name, description, type, order_index, position_x, position_y,
  responsible_role_ids, required_permission, permissions, exit_conditions, approval_rule,
  rework_enabled, replanning_enabled, fail_path
)
SELECT id, '40000000-0000-0000-0000-000000000020', name, description, type, order_index, position_x, position_y,
  roles, required_permission, permissions, exit_conditions, approval_rule, rework_enabled, replanning_enabled, fail_path
FROM node_seed
ON CONFLICT (id) DO UPDATE
SET name=EXCLUDED.name,
    description=EXCLUDED.description,
    type=EXCLUDED.type,
    order_index=EXCLUDED.order_index,
    position_x=EXCLUDED.position_x,
    position_y=EXCLUDED.position_y,
    responsible_role_ids=EXCLUDED.responsible_role_ids,
    required_permission=EXCLUDED.required_permission,
    permissions=EXCLUDED.permissions,
    exit_conditions=EXCLUDED.exit_conditions,
    approval_rule=EXCLUDED.approval_rule,
    rework_enabled=EXCLUDED.rework_enabled,
    replanning_enabled=EXCLUDED.replanning_enabled,
    fail_path=EXCLUDED.fail_path;

WITH form_versions AS (
  SELECT t.code, v.id AS version_id
  FROM workflow_form_templates t
  JOIN workflow_form_versions v ON v.template_id=t.id AND v.version_no=1
  WHERE t.code IN ('builtin_generic_submission', 'client_design_branch_decision')
),
step_seed(id, node_id, order_index, name, description, step_type, roles, permission_key, checklist_required, document_required, photo_required, form_code, form_required) AS (
  VALUES
    ('41500000-0000-0000-0000-000000000001'::uuid,'41400000-0000-0000-0000-000000000001'::uuid,1,'Khảo sát yêu cầu của chủ đầu tư','Ghi nhận yêu cầu chính của chủ đầu tư và phạm vi khảo sát. Phụ trách chính: PM, Giám đốc. Phối hợp: QS, Sale, Đội thi công.','task','["PM","Giám đốc","QS","Sale","Đội thi công"]'::jsonb,'planning.edit',true,false,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000002'::uuid,'41400000-0000-0000-0000-000000000001'::uuid,2,'Chụp và tải ảnh hiện trạng','Chụp và tải ảnh hiện trạng.','photo_upload','["Giám sát"]'::jsonb,'planning.edit',false,false,true,NULL,false),
    ('41500000-0000-0000-0000-000000000003'::uuid,'41400000-0000-0000-0000-000000000001'::uuid,3,'Nắm phong cách thiết kế','Ghi nhận phong cách thiết kế mong muốn.','task','["Thiết kế"]'::jsonb,'planning.edit',true,false,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000004'::uuid,'41400000-0000-0000-0000-000000000001'::uuid,4,'Ghi nhận kích thước chính','Ghi nhận kích thước chính phục vụ BOQ và thiết kế.','task','["QS","Thiết kế"]'::jsonb,'planning.edit',true,false,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000005'::uuid,'41400000-0000-0000-0000-000000000001'::uuid,5,'Xác định rủi ro mặt bằng','Xác định rủi ro mặt bằng và lưu ý kỹ thuật.','task','["QS","Đội thi công"]'::jsonb,'planning.edit',true,false,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000006'::uuid,'41400000-0000-0000-0000-000000000001'::uuid,6,'Hoàn tất biên bản khảo sát','Hoàn tất biên bản khảo sát.','document_upload','["PM","Giám sát"]'::jsonb,'planning.edit',true,true,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000007'::uuid,'41400000-0000-0000-0000-000000000002'::uuid,1,'Bóc khối lượng cho nhánh không thiết kế','Bóc khối lượng phục vụ nhánh không thiết kế.','task','["QS"]'::jsonb,'boq.update',true,false,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000008'::uuid,'41400000-0000-0000-0000-000000000002'::uuid,2,'Lên bản vẽ bố trí mặt bằng','Lên bản vẽ bố trí mặt bằng sơ bộ nếu cần thiết kế.','task','["Thiết kế"]'::jsonb,'planning.edit',true,false,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000009'::uuid,'41400000-0000-0000-0000-000000000002'::uuid,3,'Lên bản vẽ 3D','Lên bản vẽ 3D sơ bộ nếu cần thiết kế.','task','["Thiết kế"]'::jsonb,'planning.edit',true,false,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000010'::uuid,'41400000-0000-0000-0000-000000000002'::uuid,4,'Lên bản vẽ thi công','Lên bản vẽ thi công sơ bộ nếu cần thiết kế.','task','["Thiết kế"]'::jsonb,'planning.edit',true,false,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000011'::uuid,'41400000-0000-0000-0000-000000000002'::uuid,5,'Chốt biên bản cuộc họp và nhánh xử lý','Chốt biên bản cuộc họp và nhánh xử lý tiếp theo.','meeting_minutes','["PM","QS","Giám sát"]'::jsonb,'planning.edit',true,false,false,'client_design_branch_decision',true),
    ('41500000-0000-0000-0000-000000000012'::uuid,'41400000-0000-0000-0000-000000000003'::uuid,1,'Trao đổi và hoàn thiện bản vẽ bố trí mặt bằng','Trao đổi và hoàn thiện bản vẽ bố trí mặt bằng.','design_review','["PM"]'::jsonb,'planning.edit',true,false,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000013'::uuid,'41400000-0000-0000-0000-000000000003'::uuid,2,'Trao đổi và hoàn thiện bản vẽ 3D','Trao đổi và hoàn thiện bản vẽ 3D.','design_review','["PM"]'::jsonb,'planning.edit',true,false,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000014'::uuid,'41400000-0000-0000-0000-000000000003'::uuid,3,'Trao đổi và hoàn thiện bản vẽ thi công','Trao đổi và hoàn thiện bản vẽ thi công.','document_upload','["PM"]'::jsonb,'planning.edit',true,true,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000015'::uuid,'41400000-0000-0000-0000-000000000003'::uuid,4,'Chốt phương án với Sale và chủ đầu tư','Chốt phương án với Sale và chủ đầu tư.','approval','["PM","Chủ đầu tư"]'::jsonb,'planning.edit',true,false,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000016'::uuid,'41400000-0000-0000-0000-000000000003'::uuid,5,'Bàn giao bản vẽ cho QS','Bàn giao bản vẽ cho QS để làm BOQ.','task','["PM","QS"]'::jsonb,'planning.edit',true,false,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000017'::uuid,'41400000-0000-0000-0000-000000000004'::uuid,1,'Lấy giá đầu vào','Thu thập giá đầu vào.','task','["QS"]'::jsonb,'boq.update',true,false,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000018'::uuid,'41400000-0000-0000-0000-000000000004'::uuid,2,'Kiểm tra đơn giá và tính hợp lý của đầu vào','Kiểm tra đơn giá và tính hợp lý của đầu vào.','budget_check','["QS","PM","Giám đốc"]'::jsonb,'boq.approve',true,false,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000019'::uuid,'41400000-0000-0000-0000-000000000004'::uuid,3,'Review định mức quản lý','Giám đốc review định mức quản lý.','approval','["Giám đốc"]'::jsonb,'boq.approve',true,false,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000020'::uuid,'41400000-0000-0000-0000-000000000004'::uuid,4,'Gửi BOQ cho chủ đầu tư','Gửi BOQ cho chủ đầu tư.','document_upload','["QS","PM","Giám đốc"]'::jsonb,'boq.update',true,true,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000021'::uuid,'41400000-0000-0000-0000-000000000005'::uuid,1,'Gửi BOQ cho chủ đầu tư','Gửi BOQ cho chủ đầu tư sau thiết kế.','document_upload','["QS","Sale","PM"]'::jsonb,'boq.update',true,true,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000022'::uuid,'41400000-0000-0000-0000-000000000006'::uuid,1,'Bóc khối lượng đầu vào','Bóc khối lượng đầu vào.','task','["QS"]'::jsonb,'boq.update',true,false,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000023'::uuid,'41400000-0000-0000-0000-000000000006'::uuid,2,'Mua hàng đầu vào','Thực hiện mua hàng đầu vào.','material_check','["Mua hàng","QS"]'::jsonb,'procurement.create',true,false,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000024'::uuid,'41400000-0000-0000-0000-000000000006'::uuid,3,'Ký hợp đồng và lập hồ sơ tạm ứng đầu vào','Ký hợp đồng và chuẩn bị hồ sơ tạm ứng đầu vào.','document_upload','["Mua hàng","QS","Kế toán"]'::jsonb,'procurement.create',true,true,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000025'::uuid,'41400000-0000-0000-0000-000000000006'::uuid,4,'Rà soát và kiểm tra đầu vào','PM và Giám đốc rà soát, kiểm tra đầu vào.','approval','["PM","Giám đốc"]'::jsonb,'procurement.approve',true,false,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000026'::uuid,'41400000-0000-0000-0000-000000000007'::uuid,1,'Đàm phán với thầu phụ để chốt giá hợp lý','Đàm phán với thầu phụ để chốt giá hợp lý.','budget_check','["Sale","QS","PM","Giám đốc"]'::jsonb,'boq.approve',true,false,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000027'::uuid,'41400000-0000-0000-0000-000000000007'::uuid,2,'Chốt định mức sau đàm phán','Chốt định mức sau đàm phán.','budget_check','["QS","PM"]'::jsonb,'boq.update',true,false,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000028'::uuid,'41400000-0000-0000-0000-000000000007'::uuid,3,'Review và chốt báo cáo đơn giá','Giám đốc review và chốt báo cáo đơn giá.','approval','["Giám đốc"]'::jsonb,'boq.approve',true,false,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000029'::uuid,'41400000-0000-0000-0000-000000000008'::uuid,1,'Lập báo giá và hợp đồng','Lập báo giá và hợp đồng với chủ đầu tư.','document_upload','["QS","PM"]'::jsonb,'project.update',true,true,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000030'::uuid,'41400000-0000-0000-0000-000000000008'::uuid,2,'Kiểm tra hợp đồng','Kế toán và Pháp lý kiểm tra hợp đồng.','task','["Kế toán","Pháp lý"]'::jsonb,'contract.review',true,false,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000031'::uuid,'41400000-0000-0000-0000-000000000008'::uuid,3,'Gửi hồ sơ điện tử cho chủ đầu tư','Gửi hồ sơ điện tử gồm hợp đồng, bảng tiến độ, đề nghị tạm ứng và nội dung ký duyệt hai bên cho chủ đầu tư.','document_upload','["Sale","PM","QS"]'::jsonb,'contract.review',true,true,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000032'::uuid,'41400000-0000-0000-0000-000000000008'::uuid,4,'Rà soát và ký duyệt hợp đồng','PM và Giám đốc rà soát, ký duyệt hợp đồng.','approval','["PM","Giám đốc"]'::jsonb,'project.update',true,false,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000033'::uuid,'41400000-0000-0000-0000-000000000008'::uuid,5,'In và gửi hồ sơ bản cứng cho chủ đầu tư','In và gửi bản cứng hợp đồng, hồ sơ tạm ứng cho chủ đầu tư.','document_upload','["PM","Kế toán"]'::jsonb,'project.update',true,true,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000034'::uuid,'41400000-0000-0000-0000-000000000009'::uuid,1,'Họp và phân công thi công','Họp và phân công thi công cho QS, PM, Thiết kế và QA/QC.','task','["QS","PM","Thiết kế","QA/QC"]'::jsonb,'operation.report',true,false,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000035'::uuid,'41400000-0000-0000-0000-000000000009'::uuid,2,'Chốt biên bản cuộc họp','Chốt biên bản cuộc họp thi công và QA/QC.','document_upload','["PM","QA/QC"]'::jsonb,'operation.report',true,true,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000036'::uuid,'41400000-0000-0000-0000-000000000009'::uuid,3,'Lập kế hoạch thi công và đảm bảo chất lượng','Lập kế hoạch thi công và đảm bảo chất lượng.','daily_report','["QA/QC","Giám sát"]'::jsonb,'operation.report',true,false,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000037'::uuid,'41400000-0000-0000-0000-000000000009'::uuid,4,'Báo cáo liên tục trong quá trình thi công','QA/QC và Giám sát báo cáo liên tục trong quá trình thi công.','daily_report','["QA/QC","Giám sát"]'::jsonb,'operation.report',true,false,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000038'::uuid,'41400000-0000-0000-0000-000000000010'::uuid,1,'Nghiệm thu thầu phụ và tổ đội','Nghiệm thu thầu phụ, tổ đội và đưa số liệu cho QS kiểm tra.','acceptance_check','["QA/QC","QS"]'::jsonb,'acceptance.create',true,false,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000039'::uuid,'41400000-0000-0000-0000-000000000010'::uuid,2,'Nghiệm thu vật tư đầu vào và đầu ra','Nghiệm thu vật tư đầu vào, đầu ra và đưa số liệu cho QS kiểm tra.','acceptance_check','["QA/QC","QS"]'::jsonb,'acceptance.create',true,false,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000040'::uuid,'41400000-0000-0000-0000-000000000010'::uuid,3,'Lập hồ sơ kiểm tra khối lượng thầu phụ','Lập hồ sơ kiểm tra khối lượng thầu phụ và áp giá.','document_upload','["QS"]'::jsonb,'boq.update',true,true,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000041'::uuid,'41400000-0000-0000-0000-000000000010'::uuid,4,'Rà soát và kiểm tra hồ sơ khối lượng thầu phụ','Giám đốc rà soát, kiểm tra hồ sơ khối lượng thầu phụ.','approval','["Giám đốc"]'::jsonb,'boq.approve',true,false,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000042'::uuid,'41400000-0000-0000-0000-000000000010'::uuid,5,'Lập hồ sơ đề nghị chi thầu phụ','Tập hợp hồ sơ nguồn và lập đề nghị chi nội bộ để cấp có thẩm quyền duyệt.','document_upload','["QS","Kế toán"]'::jsonb,'project_finance.request_create',true,true,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000043'::uuid,'41400000-0000-0000-0000-000000000010'::uuid,6,'Duyệt đề nghị chi','Giám đốc kiểm tra hồ sơ nguồn và duyệt đề nghị chi nội bộ.','approval','["Giám đốc"]'::jsonb,'project_finance.request_approve',true,false,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000044'::uuid,'41400000-0000-0000-0000-000000000010'::uuid,7,'Kiểm tra hồ sơ đề nghị chi','Kiểm tra đủ hợp đồng, nghiệm thu, giá trị đề nghị và chứng từ nguồn trước khi bàn giao Kế toán.','accounting_check','["Kế toán"]'::jsonb,'project_finance.view',true,false,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000045'::uuid,'41400000-0000-0000-0000-000000000010'::uuid,8,'Bàn giao hồ sơ chi cho Kế toán','Bàn giao hồ sơ đã duyệt để Kế toán xử lý trên hệ thống chuyên môn; Consflow chỉ theo dõi trạng thái hồ sơ.','accounting_check','["Kế toán"]'::jsonb,'project_finance.process',true,false,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000046'::uuid,'41400000-0000-0000-0000-000000000011'::uuid,1,'Nghiệm thu và ký duyệt công trình','Nghiệm thu và ký duyệt công trình.','acceptance_check','["QA/QC","QS","PM","Giám đốc"]'::jsonb,'acceptance.approve',true,false,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000047'::uuid,'41400000-0000-0000-0000-000000000011'::uuid,2,'Ký duyệt hồ sơ nghiệm thu','Giám đốc ký duyệt hồ sơ nghiệm thu.','approval','["Giám đốc"]'::jsonb,'acceptance.approve',true,false,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000048'::uuid,'41400000-0000-0000-0000-000000000011'::uuid,3,'Lập hồ sơ đề nghị thu từ chủ đầu tư','Tập hợp nghiệm thu, mốc hợp đồng và lập hồ sơ đề nghị thu nội bộ.','document_upload','["Kế toán","QS","PM","Giám đốc"]'::jsonb,'project_finance.request_create',true,true,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000049'::uuid,'41400000-0000-0000-0000-000000000011'::uuid,4,'Bàn giao hồ sơ phát hành hóa đơn đầu ra','Bàn giao hồ sơ đủ điều kiện cho Kế toán phát hành hóa đơn trên hệ thống chuyên môn.','accounting_check','["Kế toán"]'::jsonb,'project_finance.process',true,false,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000050'::uuid,'41400000-0000-0000-0000-000000000011'::uuid,5,'Theo dõi hồ sơ thu theo kỳ','Theo dõi tình trạng bàn giao và xử lý hồ sơ thu theo từng mốc của hợp đồng.','accounting_check','["Kế toán"]'::jsonb,'project_finance.view',true,false,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000051'::uuid,'41400000-0000-0000-0000-000000000012'::uuid,1,'Ghi nhận chi phí mua vật tư công trình','Nhập giá trị mua vật tư theo hồ sơ nguồn; chỉ chi phí đã xác nhận mới lên báo cáo quản trị.','accounting_check','["Kế toán"]'::jsonb,'project_finance.cost_create',true,false,false,'builtin_generic_submission',true),
    ('41500000-0000-0000-0000-000000000052'::uuid,'41400000-0000-0000-0000-000000000012'::uuid,2,'Xác nhận hoàn tất hồ sơ tài chính quản trị','Kiểm tra đề nghị thu/chi và chi phí vật tư đã được xử lý trước khi đánh giá đủ điều kiện đóng công trình.','accounting_check','["Kế toán","Giám đốc"]'::jsonb,'project_finance.report_view',true,false,false,'builtin_generic_submission',true)
)
INSERT INTO pipeline_template_steps (
  id, node_id, name, description, step_type, required_role_ids, required_permission,
  checklist_required, document_required, photo_required, form_version_id, form_required, order_index
)
SELECT s.id, s.node_id, s.name, s.description, s.step_type, s.roles, s.permission_key,
  s.checklist_required, s.document_required, s.photo_required, fv.version_id, s.form_required, s.order_index
FROM step_seed s
LEFT JOIN form_versions fv ON fv.code=s.form_code
ON CONFLICT (id) DO UPDATE
SET name=EXCLUDED.name,
    description=EXCLUDED.description,
    step_type=EXCLUDED.step_type,
    required_role_ids=EXCLUDED.required_role_ids,
    required_permission=EXCLUDED.required_permission,
    checklist_required=EXCLUDED.checklist_required,
    document_required=EXCLUDED.document_required,
    photo_required=EXCLUDED.photo_required,
    form_version_id=EXCLUDED.form_version_id,
    form_required=EXCLUDED.form_required,
    order_index=EXCLUDED.order_index;

WITH edge_seed(id, from_node_id, to_node_id, condition_type, condition_config, label, priority) AS (
  VALUES
    ('41600000-0000-0000-0000-000000000001'::uuid,'41400000-0000-0000-0000-000000000001'::uuid,'41400000-0000-0000-0000-000000000002'::uuid,'always','{}'::jsonb,'Hoàn tất khảo sát',100),
    ('41600000-0000-0000-0000-000000000002'::uuid,'41400000-0000-0000-0000-000000000002'::uuid,'41400000-0000-0000-0000-000000000003'::uuid,'workflow_form_answer_equals','{"field_key":"needs_design","value":"yes"}'::jsonb,'Có thiết kế',10),
    ('41600000-0000-0000-0000-000000000003'::uuid,'41400000-0000-0000-0000-000000000002'::uuid,'41400000-0000-0000-0000-000000000004'::uuid,'workflow_form_answer_equals','{"field_key":"needs_design","value":"no"}'::jsonb,'Không thiết kế',20),
    ('41600000-0000-0000-0000-000000000004'::uuid,'41400000-0000-0000-0000-000000000003'::uuid,'41400000-0000-0000-0000-000000000005'::uuid,'always','{}'::jsonb,'Hoàn tất thiết kế',100),
    ('41600000-0000-0000-0000-000000000005'::uuid,'41400000-0000-0000-0000-000000000004'::uuid,'41400000-0000-0000-0000-000000000006'::uuid,'always','{}'::jsonb,'Qua đầu vào bắt buộc',100),
    ('41600000-0000-0000-0000-000000000006'::uuid,'41400000-0000-0000-0000-000000000005'::uuid,'41400000-0000-0000-0000-000000000006'::uuid,'always','{}'::jsonb,'Qua đầu vào bắt buộc',100),
    ('41600000-0000-0000-0000-000000000007'::uuid,'41400000-0000-0000-0000-000000000006'::uuid,'41400000-0000-0000-0000-000000000007'::uuid,'always','{}'::jsonb,'Đầu vào đã được kiểm tra',100),
    ('41600000-0000-0000-0000-000000000008'::uuid,'41400000-0000-0000-0000-000000000007'::uuid,'41400000-0000-0000-0000-000000000008'::uuid,'always','{}'::jsonb,'Chốt giá',100),
    ('41600000-0000-0000-0000-000000000009'::uuid,'41400000-0000-0000-0000-000000000008'::uuid,'41400000-0000-0000-0000-000000000009'::uuid,'always','{}'::jsonb,'Hợp đồng đã chốt',100),
    ('41600000-0000-0000-0000-000000000010'::uuid,'41400000-0000-0000-0000-000000000009'::uuid,'41400000-0000-0000-0000-000000000010'::uuid,'always','{}'::jsonb,'Thi công xong',100),
    ('41600000-0000-0000-0000-000000000011'::uuid,'41400000-0000-0000-0000-000000000010'::uuid,'41400000-0000-0000-0000-000000000011'::uuid,'always','{}'::jsonb,'Nghiệm thu đầu vào xong',100),
    ('41600000-0000-0000-0000-000000000012'::uuid,'41400000-0000-0000-0000-000000000011'::uuid,'41400000-0000-0000-0000-000000000012'::uuid,'always','{}'::jsonb,'Nghiệm thu đầu ra xong',100)
)
INSERT INTO pipeline_template_edges (id, template_id, from_node_id, to_node_id, condition_type, condition_config, label, priority)
SELECT id, '40000000-0000-0000-0000-000000000020', from_node_id, to_node_id, condition_type, condition_config, label, priority
FROM edge_seed
ON CONFLICT (id) DO UPDATE
SET from_node_id=EXCLUDED.from_node_id,
    to_node_id=EXCLUDED.to_node_id,
    condition_type=EXCLUDED.condition_type,
    condition_config=EXCLUDED.condition_config,
    label=EXCLUDED.label,
    priority=EXCLUDED.priority;

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

INSERT INTO acceptance_checklist_items (acceptance_id, item_index, title, description, is_passed, note, photo_required) VALUES
('71000000-0000-0000-0000-000000000001',0,'Bề mặt trần phẳng','Sai số trong ngưỡng cho phép',true,'Đạt',true),
('71000000-0000-0000-0000-000000000001',1,'Vị trí đèn đúng bản vẽ','Kiểm tra tim đèn',true,'Đạt',true),
('71000000-0000-0000-0000-000000000002',0,'Tường khô trước khi sơn','Đo độ ẩm bề mặt',false,'Ẩm vượt ngưỡng',true);

INSERT INTO rework_requests (project_id, issue_id, acceptance_id, title, description, status, due_date, created_by) VALUES
('30000000-0000-0000-0000-000000000001','70000000-0000-0000-0000-000000000002','71000000-0000-0000-0000-000000000001','Sửa màu sơn pantry','Sơn lại theo mẫu đã duyệt','in_progress',CURRENT_DATE + INTERVAL '3 day','10000000-0000-0000-0000-000000000002');

INSERT INTO replanning_requests (project_id, issue_id, reason, impact_scope, budget_impact, timeline_impact, status, created_by) VALUES
('30000000-0000-0000-0000-000000000002','70000000-0000-0000-0000-000000000001','Hiện trạng khác khảo sát, cần cập nhật chống thấm','boq',45000000,5,'submitted','10000000-0000-0000-0000-000000000002');

INSERT INTO handover_records (project_id, handover_code, status, client_representative, company_representative, documents_ready, warehouse_closed, payables_recorded, receivables_recorded, notes, created_by) VALUES
('30000000-0000-0000-0000-000000000001','HO-CF-Q1-001','submitted','Anh Bình','Nguyễn Minh PM',true,false,true,true,'Đang chờ xử lý vật tư dư','10000000-0000-0000-0000-000000000002');

INSERT INTO payables (project_id, payable_code, vendor_type, vendor_partner_id, vendor_name, source_type, gross_amount, advance_amount, retention_amount, deduction_amount, paid_amount, remaining_amount, due_date, status, created_by) VALUES
('30000000-0000-0000-0000-000000000001','PAY-Q1-001','supplier','90000000-0000-0000-0000-000000000101','Cadivi','procurement',120000000,0,0,0,60000000,60000000,CURRENT_DATE + INTERVAL '10 day','partially_paid','10000000-0000-0000-0000-000000000004'),
('30000000-0000-0000-0000-000000000001','PAY-Q1-002','subcontractor','90000000-0000-0000-0000-000000000201','Thầu phụ điện An Tâm','acceptance',100000000,30000000,5000000,0,0,65000000,CURRENT_DATE + INTERVAL '20 day','approved','10000000-0000-0000-0000-000000000004'),
('30000000-0000-0000-0000-000000000002','PAY-TD-001','labor_team','90000000-0000-0000-0000-000000000202','Đội sơn Minh Long','operation',45000000,10000000,0,0,0,35000000,CURRENT_DATE - INTERVAL '3 day','overdue','10000000-0000-0000-0000-000000000004');

INSERT INTO payable_payment_schedules (project_id, payable_id, schedule_code, milestone_name, schedule_type, planned_amount, paid_amount, remaining_amount, due_date, status, notes, created_by, approved_by)
SELECT project_id, id, 'SCH-Q1-001', 'Vật tư điện nước đợt 1', 'progress', 120000000, 60000000, 60000000, CURRENT_DATE + INTERVAL '10 day', 'partially_paid', 'Đợt còn lại sau giao hàng', created_by, '10000000-0000-0000-0000-000000000007'::uuid
FROM payables WHERE payable_code='PAY-Q1-001'
UNION ALL
SELECT project_id, id, 'SCH-Q1-002-ADV', 'Ứng trước thầu phụ điện', 'advance', 30000000, 30000000, 0, CURRENT_DATE - INTERVAL '2 day', 'paid', 'Đã duyệt ứng trước khởi công', created_by, '10000000-0000-0000-0000-000000000007'::uuid
FROM payables WHERE payable_code='PAY-Q1-002'
UNION ALL
SELECT project_id, id, 'SCH-Q1-002-PROG', 'Thanh toán nghiệm thu điện', 'progress', 65000000, 0, 65000000, CURRENT_DATE + INTERVAL '20 day', 'approved', 'Chờ hồ sơ nghiệm thu và quyết toán ứng', created_by, '10000000-0000-0000-0000-000000000007'::uuid
FROM payables WHERE payable_code='PAY-Q1-002';

INSERT INTO advance_requests (project_id, payable_id, request_code, requested_by, amount, reason, status, approved_by, settled_amount, vendor_name, due_date, paid_amount, paid_at, notes)
SELECT project_id, id, 'ADV-Q1-001', created_by, 30000000, 'Ứng trước vật tư và nhân công điện', 'partially_settled', '10000000-0000-0000-0000-000000000007'::uuid, 10000000, vendor_name, CURRENT_DATE - INTERVAL '2 day', 30000000, now() - INTERVAL '2 day', 'Còn quyết toán sau nghiệm thu kỳ'
FROM payables WHERE payable_code='PAY-Q1-002';

INSERT INTO receivables (project_id, receivable_code, client_partner_id, client_name, source_type, contract_amount, milestone_name, gross_amount, received_amount, remaining_amount, due_date, status, created_by) VALUES
('30000000-0000-0000-0000-000000000001','REC-Q1-001','90000000-0000-0000-0000-000000000001','Công ty An Phát','milestone',1200000000,'Đợt 1 - Tạm ứng 30%',360000000,360000000,0,CURRENT_DATE - INTERVAL '5 day','received','10000000-0000-0000-0000-000000000004'),
('30000000-0000-0000-0000-000000000001','REC-Q1-002','90000000-0000-0000-0000-000000000001','Công ty An Phát','acceptance',1200000000,'Nghiệm thu bán phần',300000000,120000000,180000000,CURRENT_DATE + INTERVAL '12 day','partially_received','10000000-0000-0000-0000-000000000004'),
('30000000-0000-0000-0000-000000000003','REC-MA-001','90000000-0000-0000-0000-000000000003','Chị Hạnh','handover',390000000,'Sau bàn giao',390000000,0,390000000,CURRENT_DATE + INTERVAL '40 day','recorded','10000000-0000-0000-0000-000000000004');

INSERT INTO payments (project_id, payment_code, payment_type, target_type, amount, payment_method, status, paid_at, created_by, approved_by) VALUES
('30000000-0000-0000-0000-000000000001','PMT-Q1-001','supplier_payment',NULL,60000000,'bank_transfer','approved',now() - INTERVAL '2 day','10000000-0000-0000-0000-000000000004','10000000-0000-0000-0000-000000000007'),
('30000000-0000-0000-0000-000000000001','RCV-Q1-001','client_receipt',NULL,360000000,'bank_transfer','approved',now() - INTERVAL '8 day','10000000-0000-0000-0000-000000000004','10000000-0000-0000-0000-000000000007');

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

-- Full workflow/form coverage demo project.
-- This section is intentionally seeded as runtime data, not migrations:
-- Pipeline template shows how a user configures Step types in Pipeline Builder;
-- runtime rows show how one project operates with all current work item types and form templates.

INSERT INTO pipeline_templates (
  id, name, description, project_type, version, status, created_by,
  template_family_id, revision_no, engine_version, definition_hash, published_at, published_by
) VALUES
('40000000-0000-0000-0000-000000000010','[Demo] Full workflow form coverage','Template demo bao phủ toàn bộ work item type, workflow form template và form/panel vận hành hiện có.','Hoàn thiện nội thất',1,'published','10000000-0000-0000-0000-000000000001','40000000-0000-0000-0000-000000000010',1,1,repeat('a',64),now(),'10000000-0000-0000-0000-000000000001');

INSERT INTO pipeline_template_nodes (id, template_id, name, description, type, order_index, position_x, position_y, responsible_role_ids, required_permission, exit_conditions, approval_rule, rework_enabled, replanning_enabled, fail_path, permissions) VALUES
('41100000-0000-0000-0000-000000000001','40000000-0000-0000-0000-000000000010','Khảo sát & Planning','Thu thập hiện trạng, checklist, daily report planning và biên bản họp.','survey',1,80,120,'["Giám sát","PM"]','planning.edit','{"text":"Có ảnh, checklist, daily report và biên bản họp"}','PM duyệt planning',false,true,'previous','["planning.edit"]'),
('41100000-0000-0000-0000-000000000002','40000000-0000-0000-0000-000000000010','Thiết kế & Xác nhận','Upload bản vẽ, review thiết kế và lấy xác nhận khách hàng.','design',2,360,120,'["PM","Chủ đầu tư"]','planning.edit','{"text":"Bản vẽ và xác nhận khách hàng đã đủ"}','Chủ đầu tư xác nhận',false,true,'replanning','["planning.edit"]'),
('41100000-0000-0000-0000-000000000003','40000000-0000-0000-0000-000000000010','BOQ · Mua sắm · Kho','Kiểm tra kế hoạch mua vật tư và nghiệm thu đầu vào.','procurement',3,640,120,'["QS","Mua hàng","Thủ kho"]','procurement.approve','{"text":"Kế hoạch mua và vật tư đầu vào đã kiểm tra"}','PM + Giám đốc duyệt',false,false,'previous','["boq.update","procurement.approve","warehouse.receive"]'),
('41100000-0000-0000-0000-000000000004','40000000-0000-0000-0000-000000000010','Thi công','Thi công, nhật ký, vật tư trước/sau và đối chiếu thầu phụ.','construction',4,920,120,'["Giám sát","Đội thi công","Thầu phụ"]','operation.report','{"text":"Nhật ký, vật tư và đối chiếu thầu phụ đã ghi nhận"}','PM review',true,true,'rework','["operation.report"]'),
('41100000-0000-0000-0000-000000000005','40000000-0000-0000-0000-000000000010','Nghiệm thu & Bàn giao','Nghiệm thu đầu ra, evidence nghiệm thu, vật tư dư, rework và bàn giao.','handover',5,1200,120,'["QA/QC","PM","Chủ đầu tư"]','handover.approve','{"text":"Nghiệm thu và bàn giao đủ hồ sơ"}','Chủ đầu tư xác nhận',true,true,'rework','["acceptance.approve","handover.approve"]'),
('41100000-0000-0000-0000-000000000006','40000000-0000-0000-0000-000000000010','Tài chính công trình','Kiểm tra chi phí vật tư, đề nghị thu/chi và số liệu quản trị trước khi đóng công trình.','financial_closure',6,1480,120,'["Kế toán","Giám đốc"]','project_finance.view','{"text":"Hồ sơ tài chính công trình đã hoàn tất"}','Kế toán + Giám đốc xác nhận',false,false,'none','["project_finance.view","project_finance.report_view"]');

WITH step_seed(id, node_name, order_index, name, step_type, roles, permission_key, checklist_required, document_required, photo_required, form_code, form_version_no, form_required, description) AS (
  VALUES
    ('41200000-0000-0000-0000-000000000001','Khảo sát & Planning',1,'Upload ảnh hiện trạng','photo_upload','["Giám sát"]'::jsonb,'planning.edit',false,false,true,NULL::text,NULL::integer,false,'Upload ảnh hiện trường để làm evidence khảo sát.'),
    ('41200000-0000-0000-0000-000000000002','Khảo sát & Planning',2,'Checklist hiện trạng basic v1','checklist','["Giám sát"]'::jsonb,'planning.edit',true,false,false,'builtin_checklist',1,true,'Checklist v1 basic.'),
    ('41200000-0000-0000-0000-000000000003','Khảo sát & Planning',3,'Checklist planning structured v2','checklist','["PM"]'::jsonb,'planning.edit',true,false,false,'builtin_checklist',2,true,'Checklist v2 structured.'),
    ('41200000-0000-0000-0000-000000000004','Khảo sát & Planning',4,'Daily report planning v1','daily_report','["PM"]'::jsonb,'planning.edit',true,false,false,'builtin_daily_report',1,true,'Daily report v1 basic.'),
    ('41200000-0000-0000-0000-000000000005','Khảo sát & Planning',5,'Daily report planning structured v2','daily_report','["PM"]'::jsonb,'planning.edit',true,false,false,'builtin_daily_report',2,true,'Daily report v2 structured.'),
    ('41200000-0000-0000-0000-000000000006','Khảo sát & Planning',6,'Biên bản họp basic v1','meeting_minutes','["PM"]'::jsonb,'planning.edit',true,false,false,'builtin_meeting_minutes',1,true,'Meeting minutes v1.'),
    ('41200000-0000-0000-0000-000000000007','Khảo sát & Planning',7,'Biên bản họp structured v2','meeting_minutes','["PM"]'::jsonb,'planning.edit',true,false,false,'builtin_meeting_minutes',2,true,'Meeting minutes v2 structured.'),
    ('41200000-0000-0000-0000-000000000008','Thiết kế & Xác nhận',1,'Upload bản vẽ thiết kế','document_upload','["PM"]'::jsonb,'planning.edit',false,true,false,NULL::text,NULL::integer,false,'Upload PDF/bản vẽ thiết kế.'),
    ('41200000-0000-0000-0000-000000000009','Thiết kế & Xác nhận',2,'Review phương án thiết kế','design_review','["PM"]'::jsonb,'planning.edit',true,false,false,'builtin_generic_submission',1,true,'Review phương án bằng generic submission.'),
    ('41200000-0000-0000-0000-000000000010','Thiết kế & Xác nhận',3,'Khách hàng xác nhận thiết kế','approval','["Chủ đầu tư","PM"]'::jsonb,'handover.approve',true,false,false,'builtin_owner_client_checklist',1,true,'Xác nhận bên ngoài bởi chủ đầu tư/khách hàng.'),
    ('41200000-0000-0000-0000-000000000011','BOQ · Mua sắm · Kho',1,'Duyệt kế hoạch mua vật tư','budget_check','["QS","PM","Giám đốc"]'::jsonb,'procurement.approve',true,false,false,'builtin_generic_submission',1,true,'Kiểm tra giá trị mua vật tư dự kiến trước khi đặt hàng.'),
    ('41200000-0000-0000-0000-000000000012','BOQ · Mua sắm · Kho',2,'Kiểm tra nhu cầu vật tư','material_check','["Mua hàng","Thủ kho"]'::jsonb,'warehouse.view',false,false,false,NULL::text,NULL::integer,false,'Material check.'),
    ('41200000-0000-0000-0000-000000000013','BOQ · Mua sắm · Kho',3,'Nghiệm thu đầu vào vật tư','material_check','["Thủ kho","QA/QC"]'::jsonb,'warehouse.receive',false,false,false,NULL::text,NULL::integer,false,'Incoming acceptance panel.'),
    ('41200000-0000-0000-0000-000000000014','Thi công',1,'Nhật ký thi công','daily_report','["Giám sát"]'::jsonb,'operation.report',true,false,false,'builtin_construction_daily_report',1,true,'Construction daily report.'),
    ('41200000-0000-0000-0000-000000000015','Thi công',2,'Vật tư trước/sau','material_check','["Giám sát","Thủ kho"]'::jsonb,'warehouse.view',false,false,false,NULL::text,NULL::integer,false,'Material before/after snapshot.'),
    ('41200000-0000-0000-0000-000000000016','Thi công',3,'Đối chiếu thầu phụ','checklist','["PM","Thầu phụ"]'::jsonb,'operation.report',true,false,false,'builtin_subcontractor_checklist',1,true,'Checklist/reconciliation thầu phụ.'),
    ('41200000-0000-0000-0000-000000000017','Nghiệm thu & Bàn giao',1,'Nghiệm thu đầu ra','acceptance_check','["QA/QC","Chủ đầu tư"]'::jsonb,'acceptance.approve',true,false,false,'builtin_checklist',2,true,'Outgoing acceptance.'),
    ('41200000-0000-0000-0000-000000000018','Nghiệm thu & Bàn giao',2,'Upload ảnh nghiệm thu','photo_upload','["QA/QC"]'::jsonb,'acceptance.create',false,false,true,NULL::text,NULL::integer,false,'Evidence ảnh nghiệm thu.'),
    ('41200000-0000-0000-0000-000000000019','Nghiệm thu & Bàn giao',3,'Xử lý vật tư dư','material_check','["Thủ kho"]'::jsonb,'warehouse.surplus_handle',false,false,false,NULL::text,NULL::integer,false,'Surplus handling.'),
    ('41200000-0000-0000-0000-000000000020','Nghiệm thu & Bàn giao',4,'Đối chiếu chủ đầu tư / Bàn giao','handover_check','["PM","Chủ đầu tư"]'::jsonb,'handover.approve',true,false,false,'builtin_owner_client_checklist',1,true,'Owner/client reconciliation, signature, PDF.'),
    ('41200000-0000-0000-0000-000000000021','Nghiệm thu & Bàn giao',5,'Rework sau nghiệm thu','rework_task','["Đội thi công"]'::jsonb,'rework.create',false,false,false,NULL::text,NULL::integer,false,'Rework task.'),
    ('41200000-0000-0000-0000-000000000022','Tài chính công trình',1,'Kiểm tra hồ sơ tài chính công trình','accounting_check','["Kế toán"]'::jsonb,'project_finance.cost_review',true,false,false,'builtin_generic_submission',1,true,'Kiểm tra chi phí vật tư và trạng thái đề nghị thu/chi đã bàn giao.'),
    ('41200000-0000-0000-0000-000000000023','Tài chính công trình',2,'Giám đốc duyệt đóng hồ sơ tài chính công trình','approval','["Giám đốc"]'::jsonb,'project_finance.request_approve',true,false,false,'builtin_generic_submission',1,true,'Giám đốc xác nhận hồ sơ quản trị đủ điều kiện đóng công trình.'),
    ('41200000-0000-0000-0000-000000000024','Tài chính công trình',3,'Cập nhật tổng hợp công trình','task','["PM"]'::jsonb,'project.update',true,false,false,'builtin_generic_submission',1,true,'Cập nhật báo cáo quản trị công trình.')
),
form_versions AS (
  SELECT t.code, v.version_no, v.id AS version_id
  FROM workflow_form_templates t
  JOIN workflow_form_versions v ON v.template_id=t.id
)
INSERT INTO pipeline_template_steps (
  id, node_id, name, description, step_type, required_role_ids, required_permission,
  checklist_required, document_required, photo_required, form_version_id, form_required, order_index
)
SELECT
  s.id::uuid, n.id, s.name, s.description, s.step_type, s.roles, s.permission_key,
  s.checklist_required, s.document_required, s.photo_required, fv.version_id, s.form_required, s.order_index
FROM step_seed s
JOIN pipeline_template_nodes n ON n.template_id='40000000-0000-0000-0000-000000000010' AND n.name=s.node_name
LEFT JOIN form_versions fv ON fv.code=s.form_code AND fv.version_no=s.form_version_no;

INSERT INTO pipeline_template_edges (template_id, from_node_id, to_node_id, condition_type, label, priority)
SELECT '40000000-0000-0000-0000-000000000010', a.id, b.id, 'always', 'Hoàn tất', 100
FROM pipeline_template_nodes a
JOIN pipeline_template_nodes b ON b.order_index=a.order_index + 1
WHERE a.template_id='40000000-0000-0000-0000-000000000010'
  AND b.template_id='40000000-0000-0000-0000-000000000010';

INSERT INTO projects (id, name, code, partner_id, client_name, address, planned_start_date, planned_end_date, estimated_budget, contract_amount, status, pipeline_template_id, pm_staff_id, progress_percent, current_pipeline_run_id) VALUES
('30000000-0000-0000-0000-000000000010','Demo vận hành đầy đủ form','CF-FULL-001','90000000-0000-0000-0000-000000000001','Công ty An Phát','Quận 1, TP.HCM','2026-06-15','2026-09-15',980000000,1450000000,'in_progress','40000000-0000-0000-0000-000000000010','21000000-0000-0000-0000-000000000002',58,'42000000-0000-0000-0000-000000000010');

INSERT INTO project_members (project_id, staff_id, role_ids, assigned_nodes)
SELECT '30000000-0000-0000-0000-000000000010', s.id, jsonb_build_array(r.display_name), '[]'::jsonb
FROM staff s
JOIN users u ON u.id=s.user_id
JOIN user_roles ur ON ur.user_id=u.id
JOIN roles r ON r.id=ur.role_id
WHERE u.email IN ('admin@consflow.local','pm@consflow.local','qs@consflow.local','accounting@consflow.local','warehouse@consflow.local','supervisor@consflow.local','director@consflow.local');

INSERT INTO pipeline_runs (id, project_id, template_id, current_node_id, status, started_at)
VALUES ('42000000-0000-0000-0000-000000000010','30000000-0000-0000-0000-000000000010','40000000-0000-0000-0000-000000000010','42100000-0000-0000-0000-000000000004','in_progress',now() - INTERVAL '15 day');

WITH run_node_seed(id, template_node_id, status, started_at, completed_at) AS (
  VALUES
    ('42100000-0000-0000-0000-000000000001','41100000-0000-0000-0000-000000000001','completed',now() - INTERVAL '15 day',now() - INTERVAL '12 day'),
    ('42100000-0000-0000-0000-000000000002','41100000-0000-0000-0000-000000000002','completed',now() - INTERVAL '12 day',now() - INTERVAL '8 day'),
    ('42100000-0000-0000-0000-000000000003','41100000-0000-0000-0000-000000000003','completed',now() - INTERVAL '8 day',now() - INTERVAL '4 day'),
    ('42100000-0000-0000-0000-000000000004','41100000-0000-0000-0000-000000000004','in_progress',now() - INTERVAL '4 day',NULL::timestamptz),
    ('42100000-0000-0000-0000-000000000005','41100000-0000-0000-0000-000000000005','pending',NULL::timestamptz,NULL::timestamptz),
    ('42100000-0000-0000-0000-000000000006','41100000-0000-0000-0000-000000000006','pending',NULL::timestamptz,NULL::timestamptz)
)
INSERT INTO pipeline_run_nodes (
  id, run_id, project_id, template_node_id, name, description, type, order_index,
  responsible_role_ids, required_permission, exit_conditions, approval_rule, fail_path,
  status, started_at, completed_at, permissions
)
SELECT
  s.id::uuid, '42000000-0000-0000-0000-000000000010', '30000000-0000-0000-0000-000000000010',
  n.id, n.name, n.description, n.type, n.order_index, n.responsible_role_ids,
  n.required_permission, n.exit_conditions, n.approval_rule, n.fail_path, s.status,
  s.started_at, s.completed_at, n.permissions
FROM run_node_seed s
JOIN pipeline_template_nodes n ON n.id=s.template_node_id::uuid;

WITH run_step_seed(id, template_step_id, run_node_id, status) AS (
  VALUES
    ('42200000-0000-0000-0000-000000000001','41200000-0000-0000-0000-000000000001','42100000-0000-0000-0000-000000000001','submitted'),
    ('42200000-0000-0000-0000-000000000002','41200000-0000-0000-0000-000000000002','42100000-0000-0000-0000-000000000001','submitted'),
    ('42200000-0000-0000-0000-000000000003','41200000-0000-0000-0000-000000000003','42100000-0000-0000-0000-000000000001','submitted'),
    ('42200000-0000-0000-0000-000000000004','41200000-0000-0000-0000-000000000004','42100000-0000-0000-0000-000000000001','submitted'),
    ('42200000-0000-0000-0000-000000000005','41200000-0000-0000-0000-000000000005','42100000-0000-0000-0000-000000000001','submitted'),
    ('42200000-0000-0000-0000-000000000006','41200000-0000-0000-0000-000000000006','42100000-0000-0000-0000-000000000001','submitted'),
    ('42200000-0000-0000-0000-000000000007','41200000-0000-0000-0000-000000000007','42100000-0000-0000-0000-000000000001','submitted'),
    ('42200000-0000-0000-0000-000000000008','41200000-0000-0000-0000-000000000008','42100000-0000-0000-0000-000000000002','submitted'),
    ('42200000-0000-0000-0000-000000000009','41200000-0000-0000-0000-000000000009','42100000-0000-0000-0000-000000000002','submitted'),
    ('42200000-0000-0000-0000-000000000010','41200000-0000-0000-0000-000000000010','42100000-0000-0000-0000-000000000002','submitted'),
    ('42200000-0000-0000-0000-000000000011','41200000-0000-0000-0000-000000000011','42100000-0000-0000-0000-000000000003','submitted'),
    ('42200000-0000-0000-0000-000000000012','41200000-0000-0000-0000-000000000012','42100000-0000-0000-0000-000000000003','submitted'),
    ('42200000-0000-0000-0000-000000000013','41200000-0000-0000-0000-000000000013','42100000-0000-0000-0000-000000000003','submitted'),
    ('42200000-0000-0000-0000-000000000014','41200000-0000-0000-0000-000000000014','42100000-0000-0000-0000-000000000004','submitted'),
    ('42200000-0000-0000-0000-000000000015','41200000-0000-0000-0000-000000000015','42100000-0000-0000-0000-000000000004','submitted'),
    ('42200000-0000-0000-0000-000000000016','41200000-0000-0000-0000-000000000016','42100000-0000-0000-0000-000000000004','submitted'),
    ('42200000-0000-0000-0000-000000000017','41200000-0000-0000-0000-000000000017','42100000-0000-0000-0000-000000000005','pending'),
    ('42200000-0000-0000-0000-000000000018','41200000-0000-0000-0000-000000000018','42100000-0000-0000-0000-000000000005','pending'),
    ('42200000-0000-0000-0000-000000000019','41200000-0000-0000-0000-000000000019','42100000-0000-0000-0000-000000000005','pending'),
    ('42200000-0000-0000-0000-000000000020','41200000-0000-0000-0000-000000000020','42100000-0000-0000-0000-000000000005','pending'),
    ('42200000-0000-0000-0000-000000000021','41200000-0000-0000-0000-000000000021','42100000-0000-0000-0000-000000000005','pending'),
    ('42200000-0000-0000-0000-000000000022','41200000-0000-0000-0000-000000000022','42100000-0000-0000-0000-000000000006','pending'),
    ('42200000-0000-0000-0000-000000000023','41200000-0000-0000-0000-000000000023','42100000-0000-0000-0000-000000000006','pending'),
    ('42200000-0000-0000-0000-000000000024','41200000-0000-0000-0000-000000000024','42100000-0000-0000-0000-000000000006','pending')
)
INSERT INTO pipeline_run_steps (
  id, run_node_id, project_id, template_step_id, name, description, step_type,
  required_role_ids, required_permission, checklist_required, document_required, photo_required,
  order_index, status, submitted_by, submitted_at
)
SELECT
  s.id::uuid, s.run_node_id::uuid, '30000000-0000-0000-0000-000000000010',
  pts.id, pts.name, pts.description, pts.step_type, pts.required_role_ids, pts.required_permission,
  pts.checklist_required, pts.document_required, pts.photo_required, pts.order_index, s.status,
  CASE WHEN s.status='submitted' THEN '10000000-0000-0000-0000-000000000002'::uuid ELSE NULL::uuid END,
  CASE WHEN s.status='submitted' THEN now() - INTERVAL '2 day' ELSE NULL::timestamptz END
FROM run_step_seed s
JOIN pipeline_template_steps pts ON pts.id=s.template_step_id::uuid;

INSERT INTO pipeline_run_edges (run_id, project_id, template_edge_id, from_run_node_id, to_run_node_id, condition_type, condition_config, label, priority)
SELECT
  '42000000-0000-0000-0000-000000000010',
  '30000000-0000-0000-0000-000000000010',
  e.id,
  from_node.id,
  to_node.id,
  e.condition_type,
  e.condition_config,
  e.label,
  e.priority
FROM pipeline_template_edges e
JOIN pipeline_run_nodes from_node ON from_node.template_node_id=e.from_node_id AND from_node.run_id='42000000-0000-0000-0000-000000000010'
JOIN pipeline_run_nodes to_node ON to_node.template_node_id=e.to_node_id AND to_node.run_id='42000000-0000-0000-0000-000000000010'
WHERE e.template_id='40000000-0000-0000-0000-000000000010';

WITH wi_seed(id, run_step_id, work_type, status, priority, assignee_staff_id, evidence_requirement_json) AS (
  VALUES
    ('43000000-0000-0000-0000-000000000001','42200000-0000-0000-0000-000000000001','photo_upload','completed','normal','21000000-0000-0000-0000-000000000006','{"photo_required":true}'::jsonb),
    ('43000000-0000-0000-0000-000000000002','42200000-0000-0000-0000-000000000002','checklist','completed','normal','21000000-0000-0000-0000-000000000006','{"checklist_required":true}'::jsonb),
    ('43000000-0000-0000-0000-000000000003','42200000-0000-0000-0000-000000000003','checklist','completed','normal','21000000-0000-0000-0000-000000000002','{"checklist_required":true}'::jsonb),
    ('43000000-0000-0000-0000-000000000004','42200000-0000-0000-0000-000000000004','daily_report','completed','normal','21000000-0000-0000-0000-000000000002','{"checklist_required":true}'::jsonb),
    ('43000000-0000-0000-0000-000000000005','42200000-0000-0000-0000-000000000005','daily_report','completed','normal','21000000-0000-0000-0000-000000000002','{"checklist_required":true}'::jsonb),
    ('43000000-0000-0000-0000-000000000006','42200000-0000-0000-0000-000000000006','task','completed','normal','21000000-0000-0000-0000-000000000002','{"checklist_required":true}'::jsonb),
    ('43000000-0000-0000-0000-000000000007','42200000-0000-0000-0000-000000000007','task','completed','normal','21000000-0000-0000-0000-000000000002','{"checklist_required":true}'::jsonb),
    ('43000000-0000-0000-0000-000000000008','42200000-0000-0000-0000-000000000008','document_upload','completed','normal','21000000-0000-0000-0000-000000000002','{"document_required":true}'::jsonb),
    ('43000000-0000-0000-0000-000000000009','42200000-0000-0000-0000-000000000009','design_review','completed','normal','21000000-0000-0000-0000-000000000002','{"checklist_required":true}'::jsonb),
    ('43000000-0000-0000-0000-000000000010','42200000-0000-0000-0000-000000000010','approval','completed','high','21000000-0000-0000-0000-000000000002','{"checklist_required":true}'::jsonb),
    ('43000000-0000-0000-0000-000000000011','42200000-0000-0000-0000-000000000011','budget_check','completed','high','21000000-0000-0000-0000-000000000004','{"checklist_required":true}'::jsonb),
    ('43000000-0000-0000-0000-000000000012','42200000-0000-0000-0000-000000000012','material_check','completed','normal','21000000-0000-0000-0000-000000000005','{}'::jsonb),
    ('43000000-0000-0000-0000-000000000013','42200000-0000-0000-0000-000000000013','material_check','completed','high','21000000-0000-0000-0000-000000000005','{}'::jsonb),
    ('43000000-0000-0000-0000-000000000014','42200000-0000-0000-0000-000000000014','daily_report','completed','normal','21000000-0000-0000-0000-000000000006','{"checklist_required":true}'::jsonb),
    ('43000000-0000-0000-0000-000000000015','42200000-0000-0000-0000-000000000015','material_check','completed','normal','21000000-0000-0000-0000-000000000006','{}'::jsonb),
    ('43000000-0000-0000-0000-000000000016','42200000-0000-0000-0000-000000000016','checklist','completed','normal','21000000-0000-0000-0000-000000000002','{"checklist_required":true}'::jsonb),
    ('43000000-0000-0000-0000-000000000017','42200000-0000-0000-0000-000000000017','acceptance_check','ready','high','21000000-0000-0000-0000-000000000002','{"checklist_required":true}'::jsonb),
    ('43000000-0000-0000-0000-000000000018','42200000-0000-0000-0000-000000000018','photo_upload','ready','normal','21000000-0000-0000-0000-000000000006','{"photo_required":true}'::jsonb),
    ('43000000-0000-0000-0000-000000000019','42200000-0000-0000-0000-000000000019','material_check','ready','normal','21000000-0000-0000-0000-000000000005','{}'::jsonb),
    ('43000000-0000-0000-0000-000000000020','42200000-0000-0000-0000-000000000020','handover_check','waiting_review','high','21000000-0000-0000-0000-000000000002','{"checklist_required":true}'::jsonb),
    ('43000000-0000-0000-0000-000000000021','42200000-0000-0000-0000-000000000021','rework_task','ready','normal','21000000-0000-0000-0000-000000000006','{}'::jsonb),
    ('43000000-0000-0000-0000-000000000022','42200000-0000-0000-0000-000000000022','accounting_check','ready','normal','21000000-0000-0000-0000-000000000004','{"checklist_required":true}'::jsonb),
    ('43000000-0000-0000-0000-000000000023','42200000-0000-0000-0000-000000000023','approval','waiting_approval','urgent','21000000-0000-0000-0000-000000000007','{"checklist_required":true}'::jsonb),
    ('43000000-0000-0000-0000-000000000024','42200000-0000-0000-0000-000000000024','task','ready','normal','21000000-0000-0000-0000-000000000002','{"checklist_required":true}'::jsonb)
)
INSERT INTO work_items (
  id, project_id, pipeline_run_id, run_node_id, run_step_id, source_type, source_id,
  title, description, work_type, status, priority, department, role_ids, assignee_staff_id,
  planned_start_at, due_at, started_at, completed_at, completion_payload_json,
  evidence_requirement_json, created_by, updated_by
)
SELECT
  s.id::uuid, '30000000-0000-0000-0000-000000000010', '42000000-0000-0000-0000-000000000010',
  rs.run_node_id, rs.id, 'pipeline_run_step', rs.id,
  rs.name, rs.description, s.work_type, s.status, s.priority,
  rn.name, rs.required_role_ids, s.assignee_staff_id::uuid,
  now() - INTERVAL '10 day', now() + (rs.order_index || ' day')::interval,
  CASE WHEN s.status IN ('in_progress','completed','waiting_review','waiting_approval') THEN now() - INTERVAL '5 day' ELSE NULL::timestamptz END,
  CASE WHEN s.status='completed' THEN now() - INTERVAL '1 day' ELSE NULL::timestamptz END,
  jsonb_build_object('seed_demo', true, 'primary_work_type', s.work_type),
  s.evidence_requirement_json,
  '10000000-0000-0000-0000-000000000002',
  '10000000-0000-0000-0000-000000000002'
FROM wi_seed s
JOIN pipeline_run_steps rs ON rs.id=s.run_step_id::uuid
JOIN pipeline_run_nodes rn ON rn.id=rs.run_node_id;

INSERT INTO work_item_assignments (work_item_id, assignment_type, staff_id, role_name, created_by)
SELECT id, 'assignee', assignee_staff_id, NULL, '10000000-0000-0000-0000-000000000002'
FROM work_items
WHERE project_id='30000000-0000-0000-0000-000000000010'
  AND assignee_staff_id IS NOT NULL;

INSERT INTO documents (id, project_id, target_type, target_id, name, file_url, document_type, status, uploaded_by) VALUES
('46000000-0000-0000-0000-000000000001','30000000-0000-0000-0000-000000000010','work_item','43000000-0000-0000-0000-000000000008','Bản vẽ thiết kế demo','https://example.com/consflow/demo-design.pdf','drawing','active','10000000-0000-0000-0000-000000000002'),
('46000000-0000-0000-0000-000000000002','30000000-0000-0000-0000-000000000010','work_item','43000000-0000-0000-0000-000000000020','Biên bản bàn giao demo','https://example.com/consflow/demo-handover.pdf','handover','active','10000000-0000-0000-0000-000000000002'),
('46000000-0000-0000-0000-000000000003','30000000-0000-0000-0000-000000000010','input_invoice','85000000-0000-0000-0000-000000000001','Hóa đơn đầu vào demo','/input-invoices/85000000-0000-0000-0000-000000000001/file','invoice','active','10000000-0000-0000-0000-000000000004'),
('46000000-0000-0000-0000-000000000004','30000000-0000-0000-0000-000000000010','output_invoice','85100000-0000-0000-0000-000000000001','Hóa đơn đầu ra demo','/output-invoices/85100000-0000-0000-0000-000000000001/file','invoice','active','10000000-0000-0000-0000-000000000004');

INSERT INTO evidence_records (
  id, work_item_id, project_id, target_type, target_id, evidence_type, document_type, name, document_id,
  title, description, file_url, mime_type, file_size, caption, revision_no, visibility, status,
  uploaded_by, submitted_by, reviewed_by, reviewed_at
) VALUES
('45000000-0000-0000-0000-000000000001','43000000-0000-0000-0000-000000000001','30000000-0000-0000-0000-000000000010','work_item','43000000-0000-0000-0000-000000000001','photo','photo','Ảnh hiện trạng sảnh',NULL,'Ảnh sảnh trước thi công','Ảnh demo external','https://example.com/consflow/evidence/site-before.jpg','image/jpeg',240000,'Sảnh trước thi công','1','project','accepted','10000000-0000-0000-0000-000000000006','10000000-0000-0000-0000-000000000006','10000000-0000-0000-0000-000000000002',now() - INTERVAL '9 day'),
('45000000-0000-0000-0000-000000000002','43000000-0000-0000-0000-000000000008','30000000-0000-0000-0000-000000000010','work_item','43000000-0000-0000-0000-000000000008','document','drawing','Bản vẽ thiết kế', '46000000-0000-0000-0000-000000000001','Bản vẽ thiết kế demo','Document demo external','https://example.com/consflow/demo-design.pdf','application/pdf',320000,'Bản vẽ layout đã gửi khách hàng','1','project','accepted','10000000-0000-0000-0000-000000000002','10000000-0000-0000-0000-000000000002','10000000-0000-0000-0000-000000000002',now() - INTERVAL '7 day'),
('45000000-0000-0000-0000-000000000003','43000000-0000-0000-0000-000000000013','30000000-0000-0000-0000-000000000010','work_item','43000000-0000-0000-0000-000000000013','photo','photo','Ảnh vật tư giao tới',NULL,'Ảnh vật tư đầu vào','Ảnh demo incoming acceptance','https://example.com/consflow/evidence/material-delivery.jpg','image/jpeg',180000,'Vật tư giao tại site','1','internal','accepted','10000000-0000-0000-0000-000000000005','10000000-0000-0000-0000-000000000005','10000000-0000-0000-0000-000000000002',now() - INTERVAL '5 day'),
('45000000-0000-0000-0000-000000000004','43000000-0000-0000-0000-000000000015','30000000-0000-0000-0000-000000000010','work_item','43000000-0000-0000-0000-000000000015','photo','photo','Ảnh vật tư trước khi dùng',NULL,'Trước khi dùng','Before material snapshot','https://example.com/consflow/evidence/material-before.jpg','image/jpeg',160000,'Dây điện còn tại site','1','internal','accepted','10000000-0000-0000-0000-000000000006','10000000-0000-0000-0000-000000000006','10000000-0000-0000-0000-000000000002',now() - INTERVAL '4 day'),
('45000000-0000-0000-0000-000000000005','43000000-0000-0000-0000-000000000015','30000000-0000-0000-0000-000000000010','work_item','43000000-0000-0000-0000-000000000015','photo','photo','Ảnh vật tư sau khi dùng',NULL,'Sau khi dùng','After material snapshot','https://example.com/consflow/evidence/material-after.jpg','image/jpeg',165000,'Dây điện sau sử dụng','1','internal','accepted','10000000-0000-0000-0000-000000000006','10000000-0000-0000-0000-000000000006','10000000-0000-0000-0000-000000000002',now() - INTERVAL '3 day'),
('45000000-0000-0000-0000-000000000006','43000000-0000-0000-0000-000000000016','30000000-0000-0000-0000-000000000010','work_item','43000000-0000-0000-0000-000000000016','photo','photo','Ảnh xác nhận thầu phụ',NULL,'Ảnh xác nhận KL điện','Subcontractor evidence','https://example.com/consflow/evidence/subcontractor.jpg','image/jpeg',150000,'Khu vực kéo dây đã hoàn tất','1','internal','accepted','10000000-0000-0000-0000-000000000006','10000000-0000-0000-0000-000000000006','10000000-0000-0000-0000-000000000002',now() - INTERVAL '2 day'),
('45000000-0000-0000-0000-000000000007','43000000-0000-0000-0000-000000000017','30000000-0000-0000-0000-000000000010','work_item','43000000-0000-0000-0000-000000000017','photo','photo','Ảnh nghiệm thu trần',NULL,'Ảnh nghiệm thu đầu ra','Outgoing acceptance evidence','https://example.com/consflow/evidence/outgoing-acceptance.jpg','image/jpeg',190000,'Trần khu A đạt','1','client','submitted','10000000-0000-0000-0000-000000000006','10000000-0000-0000-0000-000000000006',NULL,NULL),
('45000000-0000-0000-0000-000000000008','43000000-0000-0000-0000-000000000019','30000000-0000-0000-0000-000000000010','work_item','43000000-0000-0000-0000-000000000019','photo','photo','Ảnh vật tư dư',NULL,'Ảnh vật tư dư','Surplus handling evidence','https://example.com/consflow/evidence/surplus.jpg','image/jpeg',130000,'Sơn còn dư tại site','1','internal','submitted','10000000-0000-0000-0000-000000000005','10000000-0000-0000-0000-000000000005',NULL,NULL),
('45000000-0000-0000-0000-000000000009','43000000-0000-0000-0000-000000000020','30000000-0000-0000-0000-000000000010','work_item','43000000-0000-0000-0000-000000000020','document','handover','Biên bản bàn giao', '46000000-0000-0000-0000-000000000002','Biên bản bàn giao demo','Handover document','https://example.com/consflow/demo-handover.pdf','application/pdf',280000,'Biên bản bàn giao chuẩn bị ký','1','client','submitted','10000000-0000-0000-0000-000000000002','10000000-0000-0000-0000-000000000002',NULL,NULL),
('45000000-0000-0000-0000-000000000010','43000000-0000-0000-0000-000000000018','30000000-0000-0000-0000-000000000010','work_item','43000000-0000-0000-0000-000000000018','photo','photo','Ảnh nghiệm thu bổ sung',NULL,'Ảnh nghiệm thu bổ sung','Photo upload demo','https://example.com/consflow/evidence/final-photo.jpg','image/jpeg',155000,'Ảnh sau xử lý tồn tại','1','client','submitted','10000000-0000-0000-0000-000000000006','10000000-0000-0000-0000-000000000006',NULL,NULL);

INSERT INTO workflow_form_instances (
  id, project_id, work_item_id, pipeline_run_id, run_node_id, run_step_id, template_step_id,
  form_template_id, form_version_id, form_type, title, status, current_revision,
  answers_snapshot_json, required_role_ids, created_by, submitted_by, submitted_at
)
SELECT seed.instance_id::uuid, '30000000-0000-0000-0000-000000000010', wi.id, wi.pipeline_run_id, wi.run_node_id, wi.run_step_id, rs.template_step_id,
       t.id, v.id, t.form_type, seed.title, seed.status, seed.revision, seed.answers, rs.required_role_ids,
       '10000000-0000-0000-0000-000000000002',
       CASE WHEN seed.status='submitted' THEN '10000000-0000-0000-0000-000000000002'::uuid ELSE NULL::uuid END,
       CASE WHEN seed.status='submitted' THEN now() - INTERVAL '1 day' ELSE NULL::timestamptz END
FROM (
  VALUES
    ('44000000-0000-0000-0000-000000000002','43000000-0000-0000-0000-000000000002','builtin_checklist',1,'Checklist hiện trạng basic v1','submitted',1,'{"items_done":"Đã kiểm tra tường, sàn, trần và cửa hiện hữu.","exceptions":"Không có ngoại lệ lớn.","confirmed":true}'::jsonb),
    ('44000000-0000-0000-0000-000000000003','43000000-0000-0000-0000-000000000003','builtin_checklist',2,'Checklist planning structured v2','submitted',1,'{"check_date":"2026-06-16","checked_by":"Nguyễn Minh PM","planning_scope":"survey","checklist_items":[{"title":"Chốt phạm vi khảo sát","assignee_role":"PM","assignee_name":"Nguyễn Minh PM","status":"done","due_date":"2026-06-16","comment":"Đủ phạm vi"},{"title":"Đối chiếu ảnh hiện trạng","assignee_role":"Giám sát","assignee_name":"Đặng Khoa","status":"done","due_date":"2026-06-16","evidence_id":"45000000-0000-0000-0000-000000000001","comment":"Đã có ảnh"}],"overall_comment":"Planning survey đã đủ dữ liệu.","confirmed":true}'::jsonb),
    ('44000000-0000-0000-0000-000000000004','43000000-0000-0000-0000-000000000004','builtin_daily_report',1,'Daily report planning v1','submitted',1,'{"report_date":"2026-06-17","done_today":"Hoàn tất tổng hợp phạm vi khảo sát.","next_plan":"Chuẩn bị review thiết kế.","risk_note":"Không có blocker lớn."}'::jsonb),
    ('44000000-0000-0000-0000-000000000005','43000000-0000-0000-0000-000000000005','builtin_daily_report',2,'Daily report planning structured v2','submitted',1,'{"report_date":"2026-06-18","reported_by":"Nguyễn Minh PM","planning_scope":"design","progress_percent":35,"completed_items":[{"title":"Chốt layout sơ bộ","owner_role":"PM","result":"done","note":"Đã gửi team thiết kế"}],"next_plan_items":[{"title":"Upload bản vẽ thiết kế","owner_role":"PM","owner_name":"Nguyễn Minh PM","due_date":"2026-06-19","priority":"normal","description":"Upload PDF bản vẽ"}],"blockers":[],"confirmed":true}'::jsonb),
    ('44000000-0000-0000-0000-000000000006','43000000-0000-0000-0000-000000000006','builtin_meeting_minutes',1,'Biên bản họp basic v1','submitted',1,'{"meeting_date":"2026-06-18","attendees":"PM, QS, Giám sát, đại diện khách hàng","agenda":"Kickoff phạm vi thiết kế và vật tư chính.","decisions":"Chốt layout sơ bộ, giữ ngân sách trần.","action_items":"PM upload bản vẽ trước 19/06."}'::jsonb),
    ('44000000-0000-0000-0000-000000000007','43000000-0000-0000-0000-000000000007','builtin_meeting_minutes',2,'Biên bản họp structured v2','submitted',1,'{"meeting_title":"Họp chốt thiết kế demo","meeting_date":"2026-06-19","location":"Văn phòng khách hàng","chair":"Nguyễn Minh PM","attendees":[{"name":"Nguyễn Minh PM","role":"PM","attendance":"present"},{"name":"Anh Bình","role":"Chủ đầu tư","attendance":"present"}],"agenda":"Review bản vẽ và các vật tư chính.","decisions":[{"content":"Chốt phương án trần thạch cao khu A","owner":"PM","target":"Thi công"}],"action_items":[{"title":"Upload bản vẽ đã chỉnh","owner_role":"PM","owner_name":"Nguyễn Minh PM","due_date":"2026-06-20","priority":"normal","description":"Gửi lại bản vẽ PDF"}],"confirmed":true}'::jsonb),
    ('44000000-0000-0000-0000-000000000009','43000000-0000-0000-0000-000000000009','builtin_generic_submission',1,'Review phương án thiết kế','submitted',1,'{"summary":"Phương án thiết kế đã review, chỉ còn ghi chú nhỏ về cao độ trần.","next_action":"Gửi khách hàng xác nhận thiết kế."}'::jsonb),
    ('44000000-0000-0000-0000-000000000010','43000000-0000-0000-0000-000000000010','builtin_owner_client_checklist',1,'Khách hàng xác nhận thiết kế','submitted',1,'{"check_date":"2026-06-20","client_representative":"Anh Bình","company_representative":"Nguyễn Minh PM","handover_scope":"milestone","scope_detail":"Xác nhận thiết kế trước BOQ","checklist_items":[{"title":"Layout tổng thể","output_ref":"DES-01","status":"done","client_decision":"pass","comment":"Khách hàng đồng ý"},{"title":"Vật liệu sơn","output_ref":"MAT-PAINT","status":"done","client_decision":"conditional_pass","severity":"medium","required_action":"Gửi thêm mẫu màu","due_date":"2026-06-21","comment":"Đạt có điều kiện"}],"final_client_note":"Đồng ý triển khai BOQ sau khi gửi mẫu màu.","confirmed":true}'::jsonb),
    ('44000000-0000-0000-0000-000000000011','43000000-0000-0000-0000-000000000011','builtin_generic_submission',1,'Kiểm tra ngân sách','submitted',1,'{"summary":"Ngân sách demo trong ngưỡng đã duyệt.","next_action":"Cho phép tạo yêu cầu mua vật tư."}'::jsonb),
    ('44000000-0000-0000-0000-000000000014','43000000-0000-0000-0000-000000000014','builtin_construction_daily_report',1,'Nhật ký thi công','submitted',1,'{"report_date":"2026-06-27","reported_by":"Đặng Khoa Giám sát","construction_scope":"mep","weather":"normal","progress_percent":58,"site_photo_evidence_id":"45000000-0000-0000-0000-000000000001","completed_items":[{"title":"Kéo dây khu A","work_area":"Khu A","crew_name":"Thầu phụ điện An Tâm","actual_quantity":180,"unit":"m","material_used":"Dây điện Cadivi","result":"done","evidence_id":"45000000-0000-0000-0000-000000000006","note":"Hoàn tất đúng bản vẽ"}],"crew_items":[{"crew_name":"Đội điện An Tâm","trade":"MEP","worker_count":6,"work_hours":8,"note":"Đủ nhân sự"}],"material_usage_items":[{"material_name":"Dây điện Cadivi","material_code":"MAT-CADIVI","quantity":180,"unit":"m","source":"site_balance","note":"Dùng cho khu A"}],"next_plan_items":[{"title":"Kiểm tra chất lượng đầu ra","owner_role":"QA/QC","owner_name":"PM","due_date":"2026-06-28","priority":"high","description":"Chuẩn bị nghiệm thu"}],"blockers":[],"confirmed":true}'::jsonb),
    ('44000000-0000-0000-0000-000000000016','43000000-0000-0000-0000-000000000016','builtin_subcontractor_checklist',1,'Đối chiếu thầu phụ','submitted',1,'{"check_date":"2026-06-28","subcontractor_name":"Thầu phụ điện An Tâm","subcontractor_representative":"Anh Tâm","work_package":"mep","construction_area":"Khu A","checklist_items":[{"title":"Kéo dây khu A","boq_item_ref":"BOQ-FULL-001","claimed_quantity":180,"confirmed_quantity":176,"unit":"m","status":"done","evidence_id":"45000000-0000-0000-0000-000000000006","owner_response":"adjust_quantity","comment":"Điều chỉnh 4m theo kiểm tra thực tế"}],"overall_comment":"Đối chiếu có chênh lệch nhỏ.","confirmed":true}'::jsonb),
    ('44000000-0000-0000-0000-000000000017','43000000-0000-0000-0000-000000000017','builtin_checklist',2,'Nghiệm thu đầu ra','draft',0,'{"check_date":"2026-06-29","checked_by":"QA/QC","planning_scope":"other","checklist_items":[{"title":"Kiểm tra trần khu A","assignee_role":"QA/QC","status":"done","evidence_id":"45000000-0000-0000-0000-000000000007","comment":"Đạt"}],"overall_comment":"Chờ khách hàng review.","confirmed":false}'::jsonb),
    ('44000000-0000-0000-0000-000000000020','43000000-0000-0000-0000-000000000020','builtin_owner_client_checklist',1,'Đối chiếu chủ đầu tư / Bàn giao','submitted',1,'{"check_date":"2026-06-29","client_representative":"Anh Bình","company_representative":"Nguyễn Minh PM","handover_scope":"handover_item","scope_detail":"Bàn giao khu A","checklist_items":[{"title":"Trần khu A","output_ref":"ACC-FULL-001","status":"done","client_decision":"pass","evidence_id":"45000000-0000-0000-0000-000000000007","comment":"Đạt"},{"title":"Hồ sơ bàn giao","output_ref":"DOC-HO","status":"done","client_decision":"conditional_pass","evidence_id":"45000000-0000-0000-0000-000000000009","severity":"low","required_action":"Bổ sung phụ lục ảnh","due_date":"2026-07-01","comment":"Đạt có điều kiện"}],"final_client_note":"Cho phép bàn giao có điều kiện.","confirmed":true}'::jsonb),
    ('44000000-0000-0000-0000-000000000022','43000000-0000-0000-0000-000000000022','builtin_generic_submission',1,'Kiểm tra kế toán','draft',0,'{"summary":"Đã ghi nhận payable/receivable chính, còn chờ hóa đơn bổ sung.","next_action":"Giám đốc duyệt đóng tài chính."}'::jsonb),
    ('44000000-0000-0000-0000-000000000023','43000000-0000-0000-0000-000000000023','builtin_generic_submission',1,'Giám đốc duyệt đóng tài chính','draft',0,'{"summary":"Hồ sơ tài chính chờ duyệt.","next_action":"Duyệt hoặc yêu cầu bổ sung chứng từ."}'::jsonb),
    ('44000000-0000-0000-0000-000000000024','43000000-0000-0000-0000-000000000024','builtin_generic_submission',1,'Cập nhật tổng hợp project','draft',0,'{"summary":"Tổng hợp cuối kỳ đang soạn.","next_action":"Gửi review cho PM."}'::jsonb)
) AS seed(instance_id, work_item_id, form_code, version_no, title, status, revision, answers)
JOIN work_items wi ON wi.id=seed.work_item_id::uuid
JOIN pipeline_run_steps rs ON rs.id=wi.run_step_id
JOIN workflow_form_templates t ON t.code=seed.form_code
JOIN workflow_form_versions v ON v.template_id=t.id AND v.version_no=seed.version_no;

INSERT INTO workflow_form_answers (instance_id, field_key, field_type, value_json, revision_no, updated_by)
SELECT wfi.id, answer.key, COALESCE(wff.field_type, 'text'), answer.value, wfi.current_revision, wfi.created_by
FROM workflow_form_instances wfi
CROSS JOIN LATERAL jsonb_each(wfi.answers_snapshot_json) AS answer(key, value)
LEFT JOIN workflow_form_fields wff ON wff.form_version_id=wfi.form_version_id AND wff.field_key=answer.key
WHERE wfi.project_id='30000000-0000-0000-0000-000000000010';

INSERT INTO workflow_form_events (instance_id, project_id, work_item_id, actor_id, event_type, to_status, comment, metadata_json)
SELECT id, project_id, work_item_id, created_by, 'created', 'draft', 'Seed demo form instance', '{"seed_demo":true}'::jsonb
FROM workflow_form_instances
WHERE project_id='30000000-0000-0000-0000-000000000010';

INSERT INTO workflow_form_events (instance_id, project_id, work_item_id, actor_id, event_type, from_status, to_status, comment, metadata_json)
SELECT id, project_id, work_item_id, submitted_by, 'submitted', 'draft', 'submitted', 'Seed demo submitted form', '{"seed_demo":true}'::jsonb
FROM workflow_form_instances
WHERE project_id='30000000-0000-0000-0000-000000000010'
  AND status='submitted';

INSERT INTO workflow_form_signatures (
  id, project_id, work_item_id, form_instance_id, form_version_id, form_revision_no,
  purpose, signer_user_id, signer_name, signer_roles_json, comment, payload_json, payload_hash,
  visual_storage_provider
)
SELECT
  '44500000-0000-0000-0000-000000000020',
  project_id, work_item_id, id, form_version_id, current_revision,
  'approved_by', '10000000-0000-0000-0000-000000000002', 'Nguyễn Minh PM',
  '["PM"]'::jsonb, 'Ký xác nhận nội bộ cho demo bàn giao.',
  jsonb_build_object('seed_demo', true, 'form_instance_id', id),
  'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
  'none'
FROM workflow_form_instances
WHERE id='44000000-0000-0000-0000-000000000020';

INSERT INTO workflow_form_pdf_exports (
  id, project_id, work_item_id, form_instance_id, form_version_id, form_revision_no,
  title, file_name, storage_provider, storage_key, mime_type, file_size, checksum_sha256,
  payload_hash, metadata_json, generated_by, generated_by_name
)
SELECT
  '44600000-0000-0000-0000-000000000020',
  project_id, work_item_id, id, form_version_id, current_revision,
  'PDF bàn giao demo', 'demo-workflow-form.pdf', 'local',
  'workflow-form-pdfs/30000000-0000-0000-0000-000000000010/44000000-0000-0000-0000-000000000020/demo-workflow-form.pdf',
  'application/pdf', 637,
  '7082569b4e07fb42a1e0f001d618891eac1360d113d9eb954b82f0213cb012ce',
  'bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb',
  '{"seed_demo":true}'::jsonb,
  '10000000-0000-0000-0000-000000000002', 'Nguyễn Minh PM'
FROM workflow_form_instances
WHERE id='44000000-0000-0000-0000-000000000020';

INSERT INTO boq_items (
  id, project_id, category, work_item_name, description, unit, quantity, unit_price,
  material_id, status, created_by, approved_by, expected_unit_price, norm_unit_price,
  final_unit_price, unit_price_source, norm_price_locked, norm_price_locked_by,
  norm_price_locked_at, final_price_locked, final_price_locked_by, final_price_locked_at,
  price_revision_no
) VALUES
('61000000-0000-0000-0000-000000000010','30000000-0000-0000-0000-000000000010','Điện','Kéo dây điện khu A','Dây Cadivi âm trần','m',220,12000,'50000000-0000-0000-0000-000000000001','approved','10000000-0000-0000-0000-000000000003','10000000-0000-0000-0000-000000000002',12000,11800,12000,'final',true,'10000000-0000-0000-0000-000000000003',now() - INTERVAL '7 day',true,'10000000-0000-0000-0000-000000000002',now() - INTERVAL '6 day',2),
('61000000-0000-0000-0000-000000000011','30000000-0000-0000-0000-000000000010','Sơn','Sơn hoàn thiện khu A','Sơn Dulux nội thất','thùng',12,1450000,'50000000-0000-0000-0000-000000000003','approved','10000000-0000-0000-0000-000000000003','10000000-0000-0000-0000-000000000002',1450000,1420000,1450000,'final',true,'10000000-0000-0000-0000-000000000003',now() - INTERVAL '7 day',true,'10000000-0000-0000-0000-000000000002',now() - INTERVAL '6 day',2);

INSERT INTO procurement_requests (id, project_id, request_code, title, requested_by, status, total_estimated_amount, budget_status) VALUES
('60000000-0000-0000-0000-000000000010','30000000-0000-0000-0000-000000000010','PR-FULL-001','Mua vật tư demo full workflow','10000000-0000-0000-0000-000000000002','ordered',40800000,'within_budget');

INSERT INTO procurement_request_items (id, request_id, material_id, boq_item_id, quantity_planned, quantity_requested, unit, estimated_unit_price, supplier_partner_id, supplier_name, delivery_status, approval_status) VALUES
('60100000-0000-0000-0000-000000000010','60000000-0000-0000-0000-000000000010','50000000-0000-0000-0000-000000000001','61000000-0000-0000-0000-000000000010',220,220,'m',12000,'90000000-0000-0000-0000-000000000101','Cadivi','delivered','approved'),
('60100000-0000-0000-0000-000000000011','60000000-0000-0000-0000-000000000010','50000000-0000-0000-0000-000000000003','61000000-0000-0000-0000-000000000011',12,12,'thùng',1450000,'90000000-0000-0000-0000-000000000103','Dulux','delivered','approved');

INSERT INTO stock_movements (id, movement_code, project_id, warehouse_id, material_id, movement_type, quantity, unit, reason, reference_type, reference_id, created_by, approved_by) VALUES
('63000000-0000-0000-0000-000000000010','SM-FULL-ISSUE-001','30000000-0000-0000-0000-000000000010','51000000-0000-0000-0000-000000000002','50000000-0000-0000-0000-000000000001','issue',220,'m','Xuất dây điện cho demo full workflow','procurement_request','60000000-0000-0000-0000-000000000010','10000000-0000-0000-0000-000000000005','10000000-0000-0000-0000-000000000002'),
('63000000-0000-0000-0000-000000000011','SM-FULL-RETURN-001','30000000-0000-0000-0000-000000000010','51000000-0000-0000-0000-000000000001','50000000-0000-0000-0000-000000000003','return',2,'thùng','Lưu kho vật tư dư demo','site_surplus_handling','48100000-0000-0000-0000-000000000001','10000000-0000-0000-0000-000000000005','10000000-0000-0000-0000-000000000002');

INSERT INTO site_material_balances (id, project_id, material_id, planned_quantity, issued_quantity, used_quantity, surplus_quantity, surplus_status) VALUES
('62000000-0000-0000-0000-000000000010','30000000-0000-0000-0000-000000000010','50000000-0000-0000-0000-000000000001',220,220,176,44,'pending'),
('62000000-0000-0000-0000-000000000011','30000000-0000-0000-0000-000000000010','50000000-0000-0000-0000-000000000003',12,12,10,2,'store_pending');

INSERT INTO incoming_acceptance_records (
  id, acceptance_code, project_id, work_item_id, procurement_request_id, warehouse_id,
  source_type, supplier_partner_id, supplier_name, delivery_reference, status, result,
  received_at, summary_json, comment, created_by, submitted_by, submitted_at, updated_by
) VALUES
('48200000-0000-0000-0000-000000000001','IA-FULL-001','30000000-0000-0000-0000-000000000010','43000000-0000-0000-0000-000000000013','60000000-0000-0000-0000-000000000010','51000000-0000-0000-0000-000000000002','procurement','90000000-0000-0000-0000-000000000101','Cadivi','DN-FULL-001','submitted','conditional_pass',CURRENT_DATE - INTERVAL '5 day','{"line_count":2,"accepted":2}'::jsonb,'Có 2 thùng sơn cần kiểm lại màu.','10000000-0000-0000-0000-000000000005','10000000-0000-0000-0000-000000000005',now() - INTERVAL '5 day','10000000-0000-0000-0000-000000000005');

INSERT INTO incoming_acceptance_items (
  incoming_acceptance_id, project_id, work_item_id, item_index, material_id, material_code, material_name,
  procurement_request_item_id, delivered_quantity, accepted_quantity, rejected_quantity, unit, condition,
  defect_note, evidence_id, decision, comment
) VALUES
('48200000-0000-0000-0000-000000000001','30000000-0000-0000-0000-000000000010','43000000-0000-0000-0000-000000000013',0,'50000000-0000-0000-0000-000000000001','MAT-CADIVI','Dây điện Cadivi','60100000-0000-0000-0000-000000000010',220,220,0,'m','usable',NULL,'45000000-0000-0000-0000-000000000003','accepted','Đạt'),
('48200000-0000-0000-0000-000000000001','30000000-0000-0000-0000-000000000010','43000000-0000-0000-0000-000000000013',1,'50000000-0000-0000-0000-000000000003','MAT-DULUX','Sơn Dulux','60100000-0000-0000-0000-000000000011',12,10,0,'thùng','mixed','2 thùng cần kiểm màu trước khi dùng','45000000-0000-0000-0000-000000000003','conditional_acceptance','Chấp nhận có điều kiện');

INSERT INTO material_before_after_snapshots (
  id, project_id, work_item_id, status, snapshot_date, construction_area, summary_json, comment,
  applied_to_site_balance, created_by, submitted_by, submitted_at, updated_by
) VALUES
('48000000-0000-0000-0000-000000000001','30000000-0000-0000-0000-000000000010','43000000-0000-0000-0000-000000000015','submitted',CURRENT_DATE - INTERVAL '3 day','Khu A','{"rows":1,"used_quantity":176,"surplus_quantity":44}'::jsonb,'Snapshot dây điện trước/sau demo.',true,'10000000-0000-0000-0000-000000000006','10000000-0000-0000-0000-000000000006',now() - INTERVAL '3 day','10000000-0000-0000-0000-000000000006');

INSERT INTO material_before_after_snapshot_items (
  id, snapshot_id, project_id, work_item_id, item_index, material_id, material_code, material_name,
  unit, source, before_quantity, added_quantity, after_quantity, waste_quantity, used_quantity,
  affects_site_balance, site_balance_id, before_evidence_id, after_evidence_id, construction_area,
  condition, comment
) VALUES
('48000000-0000-0000-0000-000000000101','48000000-0000-0000-0000-000000000001','30000000-0000-0000-0000-000000000010','43000000-0000-0000-0000-000000000015',0,'50000000-0000-0000-0000-000000000001','MAT-CADIVI','Dây điện Cadivi','m','site_balance',220,0,44,0,176,true,'62000000-0000-0000-0000-000000000010','45000000-0000-0000-0000-000000000004','45000000-0000-0000-0000-000000000005','Khu A','usable','Dùng 176m, còn 44m.');

INSERT INTO site_surplus_handlings (
  id, handling_code, project_id, work_item_id, site_balance_id, material_id,
  material_snapshot_id, material_snapshot_item_id, stock_movement_id, status,
  handling_type, quantity, unit, warehouse_id, custody_location, condition,
  evidence_id, comment, created_by, submitted_by, submitted_at, updated_by
) VALUES
('48100000-0000-0000-0000-000000000001','SH-FULL-001','30000000-0000-0000-0000-000000000010','43000000-0000-0000-0000-000000000019','62000000-0000-0000-0000-000000000011','50000000-0000-0000-0000-000000000003',NULL,NULL,'63000000-0000-0000-0000-000000000011','submitted','return_to_warehouse',2,'thùng','51000000-0000-0000-0000-000000000001','Kệ sơn A2','usable','45000000-0000-0000-0000-000000000008','Lưu kho trung tâm 2 thùng sơn dư.','10000000-0000-0000-0000-000000000005','10000000-0000-0000-0000-000000000005',now() - INTERVAL '1 day','10000000-0000-0000-0000-000000000005');

INSERT INTO subcontractor_checklist_reconciliations (
  id, project_id, work_item_id, form_instance_id, form_revision_no, status, subcontractor_name,
  work_package, construction_area, summary_json, comment, created_by, submitted_by, submitted_at, updated_by
) VALUES
('48300000-0000-0000-0000-000000000001','30000000-0000-0000-0000-000000000010','43000000-0000-0000-0000-000000000016','44000000-0000-0000-0000-000000000016',1,'submitted','Thầu phụ điện An Tâm','MEP','Khu A','{"items":1,"variance_quantity":4}'::jsonb,'Chấp nhận điều chỉnh KL.', '10000000-0000-0000-0000-000000000002','10000000-0000-0000-0000-000000000002',now() - INTERVAL '2 day','10000000-0000-0000-0000-000000000002');

INSERT INTO subcontractor_checklist_reconciliation_items (
  reconciliation_id, project_id, work_item_id, form_instance_id, item_index,
  checklist_title, boq_item_id, boq_item_ref, boq_work_item_name, boq_quantity,
  boq_unit, claimed_quantity, confirmed_quantity, variance_quantity, unit,
  status, evidence_id, owner_response, decision, due_date, comment
) VALUES
('48300000-0000-0000-0000-000000000001','30000000-0000-0000-0000-000000000010','43000000-0000-0000-0000-000000000016','44000000-0000-0000-0000-000000000016',0,'Kéo dây khu A','61000000-0000-0000-0000-000000000010','BOQ-FULL-001','Kéo dây điện khu A',220,'m',180,176,-4,'m','done','45000000-0000-0000-0000-000000000006','adjust_quantity','adjust_quantity',CURRENT_DATE + INTERVAL '2 day','Điều chỉnh theo đo thực tế.');

INSERT INTO acceptance_records (
  id, project_id, work_item_id, form_instance_id, acceptance_code, type, title,
  requested_by, inspected_by, status, result, notes, acceptance_date, location,
  client_representative, contractor_representative, scope_summary,
  signature_status, document_status, summary_json, submitted_by, submitted_at
) VALUES
('71000000-0000-0000-0000-000000000010','30000000-0000-0000-0000-000000000010','43000000-0000-0000-0000-000000000017','44000000-0000-0000-0000-000000000017','OA-FULL-001','partial','Nghiệm thu đầu ra khu A','10000000-0000-0000-0000-000000000002','21000000-0000-0000-0000-000000000006','submitted','conditional_pass','Chờ bổ sung phụ lục ảnh. ',CURRENT_DATE,'Khu A','Anh Bình','Nguyễn Minh PM','Nghiệm thu trần và MEP khu A','pending','linked','{"items":2,"conditional":1}'::jsonb,'10000000-0000-0000-0000-000000000002',now());

INSERT INTO acceptance_checklist_items (
  acceptance_id, item_index, title, description, is_passed, note, photo_required,
  evidence_id, decision, severity, required_action, due_date
) VALUES
('71000000-0000-0000-0000-000000000010',0,'Trần khu A phẳng','Kiểm tra hoàn thiện trần',true,'Đạt',true,'45000000-0000-0000-0000-000000000007','pass','low',NULL,NULL),
('71000000-0000-0000-0000-000000000010',1,'Phụ lục ảnh bàn giao','Cần bổ sung ảnh góc kỹ thuật',false,'Đạt có điều kiện',true,'45000000-0000-0000-0000-000000000010','conditional_pass','medium','Bổ sung phụ lục ảnh',CURRENT_DATE + INTERVAL '2 day');

INSERT INTO owner_client_checklist_reconciliations (
  id, project_id, work_item_id, form_instance_id, form_revision_no, status,
  client_representative, company_representative, handover_scope, scope_detail,
  summary_json, comment, created_by, submitted_by, submitted_at, updated_by
) VALUES
('48400000-0000-0000-0000-000000000001','30000000-0000-0000-0000-000000000010','43000000-0000-0000-0000-000000000020','44000000-0000-0000-0000-000000000020',1,'submitted','Anh Bình','Nguyễn Minh PM','handover_item','Bàn giao khu A','{"items":2,"conditional":1}'::jsonb,'Khách hàng xác nhận bàn giao có điều kiện.','10000000-0000-0000-0000-000000000002','10000000-0000-0000-0000-000000000002',now(),'10000000-0000-0000-0000-000000000002');

INSERT INTO owner_client_checklist_reconciliation_items (
  reconciliation_id, project_id, work_item_id, form_instance_id, item_index,
  checklist_title, output_ref, status, client_decision, severity, required_action,
  due_date, evidence_id, acceptance_id, acceptance_title, acceptance_type,
  acceptance_status, acceptance_result, decision, comment
) VALUES
('48400000-0000-0000-0000-000000000001','30000000-0000-0000-0000-000000000010','43000000-0000-0000-0000-000000000020','44000000-0000-0000-0000-000000000020',0,'Trần khu A','ACC-FULL-001','done','pass','low',NULL,NULL,'45000000-0000-0000-0000-000000000007','71000000-0000-0000-0000-000000000010','Nghiệm thu đầu ra khu A','partial','submitted','conditional_pass','accepted','Đạt'),
('48400000-0000-0000-0000-000000000001','30000000-0000-0000-0000-000000000010','43000000-0000-0000-0000-000000000020','44000000-0000-0000-0000-000000000020',1,'Hồ sơ bàn giao','DOC-HO','done','conditional_pass','medium','Bổ sung phụ lục ảnh',CURRENT_DATE + INTERVAL '2 day','45000000-0000-0000-0000-000000000009','71000000-0000-0000-0000-000000000010','Nghiệm thu đầu ra khu A','partial','submitted','conditional_pass','conditional_acceptance','Đạt có điều kiện');

INSERT INTO issues (id, project_id, title, description, severity, status, reported_by, assigned_to, requires_rework, requires_replanning) VALUES
('70000000-0000-0000-0000-000000000010','30000000-0000-0000-0000-000000000010','Bổ sung phụ lục ảnh nghiệm thu','Khách hàng yêu cầu bổ sung ảnh góc kỹ thuật trước bàn giao cuối.','medium','open','10000000-0000-0000-0000-000000000006','21000000-0000-0000-0000-000000000006',true,false);

INSERT INTO rework_requests (id, project_id, issue_id, acceptance_id, title, description, assigned_to, status, due_date, created_by, approved_by) VALUES
('72000000-0000-0000-0000-000000000010','30000000-0000-0000-0000-000000000010','70000000-0000-0000-0000-000000000010','71000000-0000-0000-0000-000000000010','Bổ sung ảnh nghiệm thu khu A','Chụp bổ sung góc kỹ thuật và attach vào hồ sơ nghiệm thu.','21000000-0000-0000-0000-000000000006','approved',CURRENT_DATE + INTERVAL '2 day','10000000-0000-0000-0000-000000000002','10000000-0000-0000-0000-000000000002');

INSERT INTO handover_records (id, project_id, handover_code, status, final_acceptance_id, handover_date, client_representative, company_representative, documents_ready, warehouse_closed, payables_recorded, receivables_recorded, notes, created_by) VALUES
('73000000-0000-0000-0000-000000000010','30000000-0000-0000-0000-000000000010','HO-CF-FULL-001','submitted','71000000-0000-0000-0000-000000000010',CURRENT_DATE + INTERVAL '3 day','Anh Bình','Nguyễn Minh PM',true,false,true,true,'Chờ đóng vật tư dư và phụ lục ảnh.','10000000-0000-0000-0000-000000000002');

INSERT INTO accounting_budgets (project_id, budget_code, name, planned_amount, approved_amount, used_amount, remaining_amount, status, approved_by) VALUES
('30000000-0000-0000-0000-000000000010','BUD-FULL-001','Ngân sách demo full workflow',980000000,950000000,420000000,530000000,'approved','10000000-0000-0000-0000-000000000007');

INSERT INTO payables (
  id, project_id, payable_code, vendor_type, vendor_partner_id, vendor_name, source_type,
  source_id, gross_amount, advance_amount, retention_amount, deduction_amount, paid_amount,
  remaining_amount, due_date, status, created_by, approved_by
) VALUES
('81000000-0000-0000-0000-000000000010','30000000-0000-0000-0000-000000000010','PAY-FULL-001','supplier','90000000-0000-0000-0000-000000000101','Cadivi','procurement','60000000-0000-0000-0000-000000000010',120000000,30000000,5000000,0,60000000,55000000,CURRENT_DATE + INTERVAL '12 day','partially_paid','10000000-0000-0000-0000-000000000004','10000000-0000-0000-0000-000000000007');

INSERT INTO receivables (
  id, project_id, receivable_code, client_partner_id, client_name, source_type, source_id,
  contract_amount, milestone_name, gross_amount, received_amount, remaining_amount,
  due_date, status, created_by, approved_by
) VALUES
('82000000-0000-0000-0000-000000000010','30000000-0000-0000-0000-000000000010','REC-FULL-001','90000000-0000-0000-0000-000000000001','Công ty An Phát','acceptance','71000000-0000-0000-0000-000000000010',1450000000,'Nghiệm thu khu A',420000000,180000000,240000000,CURRENT_DATE + INTERVAL '14 day','partially_received','10000000-0000-0000-0000-000000000004','10000000-0000-0000-0000-000000000007');

INSERT INTO payments (id, project_id, payment_code, payment_type, target_type, target_id, amount, payment_method, status, paid_at, created_by, approved_by) VALUES
('83000000-0000-0000-0000-000000000010','30000000-0000-0000-0000-000000000010','PMT-FULL-001','supplier_payment','payable','81000000-0000-0000-0000-000000000010',60000000,'bank_transfer','approved',now() - INTERVAL '2 day','10000000-0000-0000-0000-000000000004','10000000-0000-0000-0000-000000000007'),
('83000000-0000-0000-0000-000000000011','30000000-0000-0000-0000-000000000010','RCV-FULL-001','client_receipt','receivable','82000000-0000-0000-0000-000000000010',180000000,'bank_transfer','approved',now() - INTERVAL '1 day','10000000-0000-0000-0000-000000000004','10000000-0000-0000-0000-000000000007');

INSERT INTO payable_payment_schedules (id, project_id, payable_id, schedule_code, milestone_name, schedule_type, planned_amount, paid_amount, remaining_amount, due_date, status, notes, created_by, approved_by) VALUES
('81100000-0000-0000-0000-000000000010','30000000-0000-0000-0000-000000000010','81000000-0000-0000-0000-000000000010','SCH-FULL-PAY-001','Tạm ứng vật tư demo','advance',30000000,30000000,0,CURRENT_DATE - INTERVAL '2 day','paid','Đã thanh toán tạm ứng.','10000000-0000-0000-0000-000000000004','10000000-0000-0000-0000-000000000007'),
('81100000-0000-0000-0000-000000000011','30000000-0000-0000-0000-000000000010','81000000-0000-0000-0000-000000000010','SCH-FULL-PAY-002','Thanh toán sau giao vật tư','progress',90000000,30000000,60000000,CURRENT_DATE + INTERVAL '12 day','partially_paid','Còn lại sau đối soát hóa đơn.','10000000-0000-0000-0000-000000000004','10000000-0000-0000-0000-000000000007');

INSERT INTO advance_requests (id, project_id, payable_id, request_code, requested_by, amount, reason, status, approved_by, settled_amount, vendor_name, due_date, payment_id, paid_amount, paid_at, settled_at, notes) VALUES
('81200000-0000-0000-0000-000000000010','30000000-0000-0000-0000-000000000010','81000000-0000-0000-0000-000000000010','ADV-FULL-001','10000000-0000-0000-0000-000000000004',30000000,'Ứng trước vật tư demo','partially_settled','10000000-0000-0000-0000-000000000007',10000000,'Cadivi',CURRENT_DATE + INTERVAL '10 day','83000000-0000-0000-0000-000000000010',30000000,now() - INTERVAL '2 day',NULL,'Còn quyết toán 20 triệu.');

INSERT INTO input_invoices (
  id, project_id, payable_id, procurement_request_id, document_id, invoice_number, invoice_symbol,
  invoice_date, vendor_partner_id, vendor_name, vendor_tax_code, amount_before_vat, vat_amount,
  total_amount, status, notes, storage_provider, storage_key, mime_type, file_name, file_size,
  checksum_sha256, uploaded_by, verified_by, verified_at
) VALUES
('85000000-0000-0000-0000-000000000001','30000000-0000-0000-0000-000000000010','81000000-0000-0000-0000-000000000010','60000000-0000-0000-0000-000000000010','46000000-0000-0000-0000-000000000003','INV-FULL-IN-001','AA/26E','2026-06-28','90000000-0000-0000-0000-000000000101','Cadivi','0300427407',100000000,10000000,110000000,'verified','PDF fixture demo.','local','input-invoices/30000000-0000-0000-0000-000000000010/81000000-0000-0000-0000-000000000010/demo-input-invoice.pdf','application/pdf','demo-input-invoice.pdf',623,'49caf29994137a9eee81cadcb106c6821ae45d052652bbed56944cd19186af16','10000000-0000-0000-0000-000000000004','10000000-0000-0000-0000-000000000004',now() - INTERVAL '1 day');

INSERT INTO payment_allocations (project_id, payment_id, payable_id, schedule_id, advance_request_id, input_invoice_id, allocation_type, amount, notes, created_by) VALUES
('30000000-0000-0000-0000-000000000010','83000000-0000-0000-0000-000000000010','81000000-0000-0000-0000-000000000010','81100000-0000-0000-0000-000000000010','81200000-0000-0000-0000-000000000010','85000000-0000-0000-0000-000000000001','advance_payment',30000000,'Ứng trước theo hóa đơn demo.','10000000-0000-0000-0000-000000000004'),
('30000000-0000-0000-0000-000000000010','83000000-0000-0000-0000-000000000010','81000000-0000-0000-0000-000000000010','81100000-0000-0000-0000-000000000011',NULL,'85000000-0000-0000-0000-000000000001','payable_payment',30000000,'Thanh toán tiến độ demo.','10000000-0000-0000-0000-000000000004');

INSERT INTO receivable_payment_schedules (id, project_id, receivable_id, schedule_code, milestone_name, schedule_type, planned_amount, received_amount, remaining_amount, due_date, status, notes, created_by, approved_by) VALUES
('82100000-0000-0000-0000-000000000010','30000000-0000-0000-0000-000000000010','82000000-0000-0000-0000-000000000010','SCH-FULL-REC-001','Tạm ứng khách hàng','advance',180000000,180000000,0,CURRENT_DATE - INTERVAL '1 day','received','Đã thu tạm ứng demo.','10000000-0000-0000-0000-000000000004','10000000-0000-0000-0000-000000000007'),
('82100000-0000-0000-0000-000000000011','30000000-0000-0000-0000-000000000010','82000000-0000-0000-0000-000000000010','SCH-FULL-REC-002','Thu sau nghiệm thu khu A','progress',240000000,0,240000000,CURRENT_DATE + INTERVAL '14 day','approved','Chờ nghiệm thu có điều kiện hoàn tất.','10000000-0000-0000-0000-000000000004','10000000-0000-0000-0000-000000000007');

INSERT INTO client_advance_receipts (id, project_id, receivable_id, receipt_code, client_name, requested_by, amount, received_amount, settled_amount, reason, due_date, payment_id, received_at, settled_at, status, notes, approved_by) VALUES
('82200000-0000-0000-0000-000000000010','30000000-0000-0000-0000-000000000010','82000000-0000-0000-0000-000000000010','CAR-FULL-001','Công ty An Phát','10000000-0000-0000-0000-000000000004',180000000,180000000,60000000,'Tạm ứng khách hàng demo',CURRENT_DATE - INTERVAL '1 day','83000000-0000-0000-0000-000000000011',now() - INTERVAL '1 day',NULL,'partially_settled','Còn đối trừ sau bàn giao.','10000000-0000-0000-0000-000000000007');

INSERT INTO output_invoices (
  id, project_id, receivable_id, payment_id, document_id, invoice_number, invoice_symbol,
  invoice_date, client_partner_id, client_name, client_tax_code, amount_before_vat, vat_amount,
  total_amount, status, notes, storage_provider, storage_key, mime_type, file_name, file_size,
  checksum_sha256, uploaded_by, issued_by, issued_at
) VALUES
('85100000-0000-0000-0000-000000000001','30000000-0000-0000-0000-000000000010','82000000-0000-0000-0000-000000000010','83000000-0000-0000-0000-000000000011','46000000-0000-0000-0000-000000000004','INV-FULL-OUT-001','AB/26E','2026-06-29','90000000-0000-0000-0000-000000000001','Công ty An Phát','0312345678',380000000,38000000,418000000,'issued','PDF fixture demo.','local','output-invoices/30000000-0000-0000-0000-000000000010/82000000-0000-0000-0000-000000000010/demo-output-invoice.pdf','application/pdf','demo-output-invoice.pdf',624,'6d9ca61d3f29fdd585c36e812b5827aa80578e56d9902c45356a5a6a19c0f5ec','10000000-0000-0000-0000-000000000004','10000000-0000-0000-0000-000000000004',now());

INSERT INTO receipt_allocations (project_id, payment_id, receivable_id, schedule_id, advance_receipt_id, output_invoice_id, allocation_type, amount, notes, created_by) VALUES
('30000000-0000-0000-0000-000000000010','83000000-0000-0000-0000-000000000011','82000000-0000-0000-0000-000000000010','82100000-0000-0000-0000-000000000010','82200000-0000-0000-0000-000000000010','85100000-0000-0000-0000-000000000001','advance_receipt',180000000,'Thu tạm ứng theo hóa đơn demo.','10000000-0000-0000-0000-000000000004');

INSERT INTO approval_requests (id, target_type, target_id, project_id, requested_by, status, reason, required_roles) VALUES
('47000000-0000-0000-0000-000000000023','work_item','43000000-0000-0000-0000-000000000023','30000000-0000-0000-0000-000000000010','10000000-0000-0000-0000-000000000004','pending','Duyệt đóng tài chính demo full workflow','["Giám đốc"]');

UPDATE work_items
SET approval_request_id='47000000-0000-0000-0000-000000000023'
WHERE id='43000000-0000-0000-0000-000000000023';

INSERT INTO work_item_activities (work_item_id, project_id, scope_type, scope_id, actor_id, activity_type, to_status, comment, metadata_json)
SELECT id, project_id, 'work_item', id, created_by, 'created', status, 'Seeded full workflow demo work item', jsonb_build_object('seed_demo', true, 'work_type', work_type)
FROM work_items
WHERE project_id='30000000-0000-0000-0000-000000000010';

INSERT INTO work_item_activities (work_item_id, project_id, scope_type, scope_id, actor_id, activity_type, comment, metadata_json)
VALUES
('43000000-0000-0000-0000-000000000015','30000000-0000-0000-0000-000000000010','work_item','43000000-0000-0000-0000-000000000015','10000000-0000-0000-0000-000000000006','system_event','Submitted material before/after snapshot','{"status":"submitted","material_snapshot_id":"48000000-0000-0000-0000-000000000001"}'::jsonb),
('43000000-0000-0000-0000-000000000019','30000000-0000-0000-0000-000000000010','work_item','43000000-0000-0000-0000-000000000019','10000000-0000-0000-0000-000000000005','system_event','Submitted surplus handling','{"status":"submitted","surplus_handling_id":"48100000-0000-0000-0000-000000000001"}'::jsonb),
('43000000-0000-0000-0000-000000000013','30000000-0000-0000-0000-000000000010','work_item','43000000-0000-0000-0000-000000000013','10000000-0000-0000-0000-000000000005','system_event','Submitted incoming acceptance','{"status":"submitted","incoming_acceptance_id":"48200000-0000-0000-0000-000000000001"}'::jsonb),
('43000000-0000-0000-0000-000000000017','30000000-0000-0000-0000-000000000010','work_item','43000000-0000-0000-0000-000000000017','10000000-0000-0000-0000-000000000002','system_event','Submitted outgoing acceptance','{"status":"submitted","acceptance_id":"71000000-0000-0000-0000-000000000010"}'::jsonb),
('43000000-0000-0000-0000-000000000020','30000000-0000-0000-0000-000000000010','work_item','43000000-0000-0000-0000-000000000020','10000000-0000-0000-0000-000000000002','system_event','Workflow form signed and PDF exported','{"signature_id":"44500000-0000-0000-0000-000000000020","pdf_export_id":"44600000-0000-0000-0000-000000000020"}'::jsonb),
('43000000-0000-0000-0000-000000000023','30000000-0000-0000-0000-000000000010','work_item','43000000-0000-0000-0000-000000000023','10000000-0000-0000-0000-000000000004','approval_requested','Duyệt đóng tài chính demo full workflow','{"approval_request_id":"47000000-0000-0000-0000-000000000023"}'::jsonb);

INSERT INTO audit_logs (project_id, actor_id, actor_name, event_type, target_type, target_id, description, metadata_json) VALUES
('30000000-0000-0000-0000-000000000010','10000000-0000-0000-0000-000000000001','Admin hệ thống','seed_full_workflow_demo','project','30000000-0000-0000-0000-000000000010','Seed demo project covering all current work item types and forms','{"template_id":"40000000-0000-0000-0000-000000000010","work_items":24,"workflow_form_instances":16}'::jsonb);

INSERT INTO project_material_costs (
  id,project_id,procurement_request_id,document_id,vendor_partner_id,vendor_name,invoice_number,invoice_date,
  description,amount_before_vat,vat_amount,total_amount,recognized_cost_amount,notes,status,created_by,
  submitted_by,submitted_at,reviewed_by,reviewed_at
) VALUES
('86000000-0000-0000-0000-000000000001','30000000-0000-0000-0000-000000000010','60000000-0000-0000-0000-000000000010','46000000-0000-0000-0000-000000000003','90000000-0000-0000-0000-000000000101','Cadivi','CPVT-FULL-001','2026-06-28','Mua dây điện cho khu A',100000000,10000000,110000000,110000000,'Chi phí vật tư quản trị đã xác nhận.','confirmed','10000000-0000-0000-0000-000000000004','10000000-0000-0000-0000-000000000004',now()-INTERVAL '2 day','10000000-0000-0000-0000-000000000004',now()-INTERVAL '1 day'),
('86000000-0000-0000-0000-000000000002','30000000-0000-0000-0000-000000000010','60000000-0000-0000-0000-000000000010',NULL,'90000000-0000-0000-0000-000000000101','Cadivi','CPVT-FULL-002',CURRENT_DATE,'Mua bổ sung thiết bị điện',40000000,4000000,44000000,0,'Chờ xác nhận hồ sơ nguồn.','submitted','10000000-0000-0000-0000-000000000004','10000000-0000-0000-0000-000000000004',now(),NULL,NULL);

INSERT INTO project_cash_requests (
  id,project_id,request_code,direction,title,counterparty_name,amount,due_date,source_type,source_id,notes,status,created_by,
  submitted_by,submitted_at,approved_by,approved_at,handed_to_accounting_by,handed_to_accounting_at
) VALUES
('86100000-0000-0000-0000-000000000001','30000000-0000-0000-0000-000000000010','DNC-FULL-001','outbound','Đề nghị chi tiền mua dây điện','Cadivi',110000000,CURRENT_DATE+INTERVAL '2 day','procurement_request','60000000-0000-0000-0000-000000000010','Đã bàn giao Kế toán xử lý trên MISA.','handed_to_accounting','10000000-0000-0000-0000-000000000004','10000000-0000-0000-0000-000000000004',now()-INTERVAL '2 day','10000000-0000-0000-0000-000000000007',now()-INTERVAL '1 day','10000000-0000-0000-0000-000000000004',now()),
('86100000-0000-0000-0000-000000000002','30000000-0000-0000-0000-000000000010','DNT-FULL-001','inbound','Đề nghị thu theo nghiệm thu khu A','Công ty An Phát',420000000,CURRENT_DATE+INTERVAL '7 day','acceptance','71000000-0000-0000-0000-000000000010','Chờ Giám đốc duyệt trước khi bàn giao.','submitted','10000000-0000-0000-0000-000000000004','10000000-0000-0000-0000-000000000004',now(),NULL,NULL,NULL,NULL);

SELECT backfill_accounting_ledger();
