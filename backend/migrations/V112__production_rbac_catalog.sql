-- Production databases run migrations without demo seed data. Ensure the
-- application role and permission catalog exists before the one-shot Admin
-- bootstrap command runs. This migration is additive: it never creates users,
-- removes custom roles, or rewrites existing user assignments.

INSERT INTO roles (id, name, display_name, description, system_role, external_only) VALUES
  ('20000000-0000-0000-0000-000000000001','admin','Admin','Quản trị toàn hệ thống',true,false),
  ('20000000-0000-0000-0000-000000000002','director','Giám đốc','Duyệt ngân sách và quyết định vượt hạn mức',true,false),
  ('20000000-0000-0000-0000-000000000003','pm','PM','Quản lý công trình và pipeline',true,false),
  ('20000000-0000-0000-0000-000000000004','qs','QS','Bóc tách khối lượng, BOQ và kiểm soát chi phí',true,false),
  ('20000000-0000-0000-0000-000000000005','accounting','Kế toán','Ghi nhận chi phí vật tư và theo dõi hồ sơ thu/chi công trình',true,false),
  ('20000000-0000-0000-0000-000000000006','procurement','Mua hàng','Phụ trách yêu cầu mua và nhà cung cấp',true,false),
  ('20000000-0000-0000-0000-000000000007','warehouse','Thủ kho','Nhập xuất tồn và vật tư dư',true,false),
  ('20000000-0000-0000-0000-000000000008','supervisor','Giám sát','Báo cáo thi công và hiện trường',true,false),
  ('20000000-0000-0000-0000-000000000009','qa_qc','QA/QC','Nghiệm thu và kiểm soát chất lượng',true,false),
  ('20000000-0000-0000-0000-000000000010','construction_team','Đội thi công','Thực hiện công việc tại công trình',true,false),
  ('20000000-0000-0000-0000-000000000011','subcontractor','Thầu phụ','Đơn vị thực hiện theo gói thầu',true,false),
  ('20000000-0000-0000-0000-000000000012','client','Chủ đầu tư','Xác nhận thiết kế, nghiệm thu, bàn giao',true,true),
  ('20000000-0000-0000-0000-000000000013','viewer','Viewer','Chỉ xem dữ liệu được phân quyền',true,false),
  ('20000000-0000-0000-0000-000000000014','design','Thiết kế','Phụ trách phương án và hồ sơ thiết kế',true,false),
  ('20000000-0000-0000-0000-000000000015','sales','Sale','Phụ trách phối hợp khách hàng, BOQ và hồ sơ thương mại',true,false),
  ('20000000-0000-0000-0000-000000000016','legal','Pháp lý','Phụ trách rà soát pháp lý hợp đồng',true,false),
  ('20000000-0000-0000-0000-000000000017','contract_officer','Bộ phận HĐ','Phụ trách lập, luân chuyển, ký và lưu hồ sơ hợp đồng',true,false),
  ('20000000-0000-0000-0000-000000000018','developer','Nhà phát triển','Quản trị kỹ thuật và thiết lập hệ thống',true,false)
ON CONFLICT (name) DO UPDATE
SET display_name=EXCLUDED.display_name,
    description=EXCLUDED.description,
    system_role=true,
    external_only=EXCLUDED.external_only;

INSERT INTO permissions (id, key, name, module)
SELECT gen_random_uuid(), seed.key, seed.name, seed.module
FROM (VALUES
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
  ('audit.view','Xem audit','audit'),('event_log.view','Xem nhật ký sự kiện','event_log'),('report.view','Xem báo cáo','report'),('report.finance.view','Xem báo cáo tài chính','report'),
  ('statistics.view','Xem thống kê','statistics'),('statistics.export','Xuất dữ liệu thống kê','statistics'),
  ('work_report.view','Xem báo cáo công việc','work_report'),('work_report.create','Lập báo cáo công việc','work_report'),('work_report.submit','Gửi báo cáo công việc','work_report'),('work_report.review','Xem xét báo cáo công việc','work_report'),('work_report.export','Xuất báo cáo công việc','work_report'),('work_report.manage','Quản lý báo cáo công việc','work_report'),
  ('settings.view','Xem cấu hình','settings'),('settings.update','Sửa cấu hình','settings'),
  ('seal.use','Sử dụng con dấu','settings'),('seal.manage','Quản lý con dấu','settings')
) AS seed(key, name, module)
ON CONFLICT (key) DO UPDATE
SET name=EXCLUDED.name,
    module=EXCLUDED.module;

UPDATE permissions
SET description='Cập nhật nội dung kế hoạch được phân công'
WHERE key='planning.edit';

UPDATE permissions
SET description='Đặt mật khẩu và email khôi phục đã xác minh cho tài khoản liên kết với nhân sự'
WHERE key='staff.account_security.manage';

-- Developer retains every capability. Admin receives every capability except
-- the restricted settings.update permission.
INSERT INTO role_permissions (role_id, permission_id)
SELECT role.id, permission.id
FROM roles role
CROSS JOIN permissions permission
WHERE role.name='developer'
   OR (role.name='admin' AND permission.key <> 'settings.update')
ON CONFLICT DO NOTHING;

-- Base operational role matrix. This mirrors the production-safe portion of
-- seed.sql without creating demo identities or business records.
INSERT INTO role_permissions (role_id, permission_id)
SELECT role.id, permission.id
FROM roles role
JOIN permissions permission ON
  (role.name='director' AND permission.key IN (
    'project.view','project.member.manage','pipeline.view','pipeline.run','pipeline.advance_node',
    'boq.view','boq.approve','procurement.view','procurement.approve','warehouse.view','operation.view',
    'acceptance.view','acceptance.approve','acceptance.reject','rework.view','rework.approve',
    'replanning.view','replanning.approve','handover.view','handover.approve',
    'catalog.view','audit.view','report.view',
    'contract.view','contract.review','contract.approve','contract.sign','contract.milestone_manage','contract.retention_release','seal.use',
    'project_finance.view','project_finance.request_approve','project_finance.report_view',
    'statistics.view','statistics.export','work_report.view','work_report.review','work_report.export','work_report.manage'
  ))
  OR (role.name='pm' AND permission.key IN (
    'project.view','project.update','project.member.manage','pipeline.view','pipeline.run','pipeline.advance_node','planning.view','planning.edit',
    'boq.view','boq.create','boq.update','boq.approve','procurement.view','procurement.create','procurement.approve','procurement.order','warehouse.view',
    'operation.view','operation.report','operation.issue_create','operation.issue_update','acceptance.view','acceptance.create','acceptance.approve','acceptance.reject',
    'rework.view','rework.create','rework.approve','replanning.view','replanning.create','replanning.approve','handover.view','handover.create','handover.approve',
    'catalog.view','audit.view','report.view','contract.view','contract.create','contract.review','contract.approve','contract.milestone_manage','contract.dispatch','contract.scan_manage','contract.variation.propose',
    'project_finance.view','project_finance.cost_create','project_finance.request_create','project_finance.report_view',
    'statistics.view','statistics.export','work_report.view','work_report.create','work_report.submit','work_report.review','work_report.export','work_report.manage'
  ))
  OR (role.name='qs' AND permission.key IN (
    'project.view','pipeline.view','boq.view','boq.create','boq.update','boq.approve','procurement.view','catalog.view','report.view',
    'contract.view','contract.create','contract.review','contract.milestone_manage','contract.variation_qs_review','project_finance.view','project_finance.report_view','statistics.view','statistics.export'
  ))
  OR (role.name='accounting' AND permission.key IN (
    'project.view','pipeline.view','boq.view','procurement.view','procurement.approve','handover.view','catalog.view','catalog.manage','audit.view','report.view',
    'contract.view','contract.review','contract.milestone_manage','contract.retention_release',
    'project_finance.view','project_finance.cost_create','project_finance.cost_review','project_finance.request_create','project_finance.process','project_finance.report_view',
    'statistics.view','statistics.export'
  ))
  OR (role.name='procurement' AND permission.key IN (
    'project.view','pipeline.view','procurement.view','procurement.create','procurement.approve','procurement.order','procurement.receive','warehouse.view','catalog.view','catalog.manage','report.view',
    'contract.view','contract.variation_purchase','contract.dispatch','contract.scan_manage','project_finance.view','project_finance.cost_create','project_finance.request_create','statistics.view','statistics.export'
  ))
  OR (role.name='warehouse' AND permission.key IN (
    'project.view','pipeline.view','procurement.view','procurement.receive','warehouse.view','warehouse.receive','warehouse.issue','warehouse.transfer','warehouse.adjust','warehouse.surplus_handle','catalog.view','report.view','statistics.view','statistics.export'
  ))
  OR (role.name='supervisor' AND permission.key IN (
    'project.view','pipeline.view','warehouse.view','operation.view','operation.report','operation.issue_create','operation.issue_update','catalog.view','acceptance.view','rework.view','rework.create','report.view','contract.variation.propose',
    'statistics.view','statistics.export','work_report.view','work_report.create','work_report.submit','work_report.export'
  ))
  OR (role.name='qa_qc' AND permission.key IN (
    'project.view','pipeline.view','operation.view','acceptance.view','acceptance.create','acceptance.approve','acceptance.reject','rework.view','rework.create','catalog.view','report.view','statistics.view','statistics.export'
  ))
  OR (role.name='construction_team' AND permission.key IN (
    'project.view','pipeline.view','planning.view','planning.edit','operation.view','operation.report','operation.issue_create','warehouse.view',
    'work_report.view','work_report.create','work_report.submit','work_report.export'
  ))
  OR (role.name='subcontractor' AND permission.key IN (
    'project.view','operation.view','operation.report','operation.issue_create','warehouse.view',
    'work_report.view','work_report.create','work_report.submit','work_report.export'
  ))
  OR (role.name='viewer' AND permission.key IN ('project.view','pipeline.view','operation.view','acceptance.view','handover.view','report.view','statistics.view','statistics.export'))
  OR (role.name='design' AND permission.key IN (
    'project.view','pipeline.view','planning.view','planning.edit','operation.view','operation.report',
    'work_report.view','work_report.create','work_report.submit','work_report.export'
  ))
  OR (role.name='sales' AND permission.key IN ('project.view','pipeline.view','planning.view','planning.edit','boq.view','boq.update','boq.approve','contract.view','contract.review'))
  OR (role.name='legal' AND permission.key IN ('project.view','pipeline.view','contract.view','contract.review'))
  OR (role.name='contract_officer' AND permission.key IN ('project.view','pipeline.view','contract.view','contract.create','contract.review','contract.dispatch','contract.scan_manage','seal.use'))
ON CONFLICT DO NOTHING;

-- Reapply grants introduced by migrations that originally ran before the base
-- roles existed on a fresh production database.
INSERT INTO role_permissions (role_id, permission_id)
SELECT role.id, permission.id
FROM roles role
JOIN permissions permission ON
  (role.name='director' AND permission.key IN ('quality.view','quality.approve','construction.action.verify','handover.verify'))
  OR (role.name='pm' AND permission.key IN (
    'quality.view','quality.create','quality.review','quality.approve','quality.ncr_manage','quality.recurring_manage',
    'construction.template.manage','construction.check.request','construction.check.perform','construction.check.edit',
    'construction.action.create','construction.action.resolve','construction.action.verify','construction.action.edit','construction.handoff.manage','handover.verify'
  ))
  OR (role.name='qa_qc' AND permission.key IN (
    'quality.view','quality.create','quality.review','quality.inspect','quality.ncr_manage','quality.recurring_manage',
    'construction.template.manage','construction.check.request','construction.check.perform','construction.check.edit',
    'construction.action.create','construction.action.resolve','construction.action.verify','construction.action.edit','construction.handoff.manage','handover.verify'
  ))
  OR (role.name='supervisor' AND permission.key IN (
    'quality.view','quality.create','quality.recurring_manage','construction.check.request','construction.check.perform','construction.check.edit',
    'construction.action.create','construction.action.resolve','construction.action.edit'
  ))
  OR (role.name='construction_team' AND permission.key IN (
    'quality.view','construction.check.request','construction.check.edit','construction.action.create','construction.action.resolve','construction.action.edit'
  ))
ON CONFLICT DO NOTHING;

-- Reassert reserved permission boundaries after merging an existing catalog.
DELETE FROM role_permissions role_permission
USING roles role, permissions permission
WHERE role_permission.role_id=role.id
  AND role_permission.permission_id=permission.id
  AND (
    role.external_only
    OR (permission.key='settings.view' AND role.name NOT IN ('admin','developer'))
    OR (permission.key='settings.update' AND role.name <> 'developer')
    OR (permission.key='seal.manage' AND role.name NOT IN ('admin','developer'))
    OR (permission.key='event_log.view' AND role.name NOT IN ('admin','developer'))
    OR (role.name NOT IN ('admin','developer') AND (permission.key LIKE 'accounting.%' OR permission.key IN ('accounting.view','report.finance.view')))
  );
