-- Accounting reconciles company-wide project finance from operational source
-- records. This capability broadens read scope only; mutation permissions stay
-- on their existing keys and continue through their normal backend gates.

INSERT INTO permissions (id, key, name, module, description)
VALUES (
  gen_random_uuid(),
  'project.reconciliation.view',
  'Đối soát tài chính toàn công ty',
  'project',
  'Quan sát toàn bộ công trình và dữ liệu nguồn phục vụ đối soát kế toán; không tự cấp quyền thao tác nghiệp vụ'
)
ON CONFLICT (key) DO UPDATE
SET name=EXCLUDED.name,
    module=EXCLUDED.module,
    description=EXCLUDED.description;

INSERT INTO role_permissions (role_id, permission_id)
SELECT role.id, permission.id
FROM roles role
JOIN permissions permission ON permission.key IN (
  'project.reconciliation.view',
  'project.view',
  'pipeline.view',
  'planning.view',
  'boq.view',
  'procurement.view',
  'contract.view',
  'warehouse.view',
  'operation.view',
  'quality.view',
  'acceptance.view',
  'rework.view',
  'replanning.view',
  'handover.view',
  'accounting_handoff.view',
  'project_finance.view',
  'project_finance.report_view',
  'report.view',
  'statistics.view'
)
WHERE role.name='accounting'
ON CONFLICT DO NOTHING;

UPDATE roles
SET description='Đối soát tài chính toàn công ty từ dữ liệu nguồn và xử lý nghiệp vụ Kế toán được phân quyền'
WHERE name='accounting';
