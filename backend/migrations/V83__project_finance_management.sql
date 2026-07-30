CREATE TABLE IF NOT EXISTS project_material_costs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  procurement_request_id UUID REFERENCES procurement_requests(id) ON DELETE SET NULL,
  incoming_acceptance_id UUID REFERENCES incoming_acceptance_records(id) ON DELETE SET NULL,
  stock_document_id UUID REFERENCES stock_documents(id) ON DELETE SET NULL,
  document_id UUID REFERENCES documents(id) ON DELETE SET NULL,
  vendor_partner_id UUID REFERENCES partners(id) ON DELETE SET NULL,
  vendor_name TEXT NOT NULL,
  invoice_number TEXT,
  invoice_date DATE,
  description TEXT NOT NULL,
  amount_before_vat NUMERIC(18,2) NOT NULL DEFAULT 0,
  vat_amount NUMERIC(18,2) NOT NULL DEFAULT 0,
  total_amount NUMERIC(18,2) NOT NULL DEFAULT 0,
  recognized_cost_amount NUMERIC(18,2) NOT NULL DEFAULT 0,
  notes TEXT,
  status TEXT NOT NULL DEFAULT 'draft',
  created_by UUID NOT NULL REFERENCES users(id),
  submitted_by UUID REFERENCES users(id),
  submitted_at TIMESTAMPTZ,
  reviewed_by UUID REFERENCES users(id),
  reviewed_at TIMESTAMPTZ,
  review_comment TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT project_material_costs_status_check CHECK (status IN ('draft','submitted','needs_changes','confirmed','cancelled')),
  CONSTRAINT project_material_costs_vendor_nonblank CHECK (NULLIF(btrim(vendor_name),'') IS NOT NULL),
  CONSTRAINT project_material_costs_description_nonblank CHECK (NULLIF(btrim(description),'') IS NOT NULL),
  CONSTRAINT project_material_costs_amounts_nonnegative CHECK (
    amount_before_vat >= 0 AND vat_amount >= 0 AND total_amount > 0 AND recognized_cost_amount >= 0
  ),
  CONSTRAINT project_material_costs_total_consistency CHECK (total_amount = amount_before_vat + vat_amount),
  CONSTRAINT project_material_costs_recognized_within_total CHECK (recognized_cost_amount <= total_amount),
  CONSTRAINT project_material_costs_confirmation_check CHECK (
    status <> 'confirmed' OR (recognized_cost_amount > 0 AND reviewed_by IS NOT NULL AND reviewed_at IS NOT NULL)
  )
);

CREATE INDEX IF NOT EXISTS idx_project_material_costs_project_status
  ON project_material_costs(project_id,status,invoice_date DESC NULLS LAST,created_at DESC);

CREATE UNIQUE INDEX IF NOT EXISTS ux_project_material_costs_invoice_vendor
  ON project_material_costs(project_id,lower(btrim(invoice_number)),lower(btrim(vendor_name)))
  WHERE NULLIF(btrim(invoice_number),'') IS NOT NULL AND status <> 'cancelled';

CREATE TABLE IF NOT EXISTS project_cash_requests (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  request_code TEXT NOT NULL UNIQUE,
  direction TEXT NOT NULL,
  title TEXT NOT NULL,
  counterparty_name TEXT NOT NULL,
  amount NUMERIC(18,2) NOT NULL,
  due_date DATE,
  source_type TEXT,
  source_id UUID,
  document_id UUID REFERENCES documents(id) ON DELETE SET NULL,
  notes TEXT,
  status TEXT NOT NULL DEFAULT 'draft',
  created_by UUID NOT NULL REFERENCES users(id),
  submitted_by UUID REFERENCES users(id),
  submitted_at TIMESTAMPTZ,
  approved_by UUID REFERENCES users(id),
  approved_at TIMESTAMPTZ,
  handed_to_accounting_by UUID REFERENCES users(id),
  handed_to_accounting_at TIMESTAMPTZ,
  processed_by UUID REFERENCES users(id),
  processed_at TIMESTAMPTZ,
  review_comment TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT project_cash_requests_direction_check CHECK (direction IN ('inbound','outbound')),
  CONSTRAINT project_cash_requests_status_check CHECK (status IN ('draft','submitted','approved','handed_to_accounting','needs_changes','processed','cancelled')),
  CONSTRAINT project_cash_requests_code_nonblank CHECK (NULLIF(btrim(request_code),'') IS NOT NULL),
  CONSTRAINT project_cash_requests_title_nonblank CHECK (NULLIF(btrim(title),'') IS NOT NULL),
  CONSTRAINT project_cash_requests_counterparty_nonblank CHECK (NULLIF(btrim(counterparty_name),'') IS NOT NULL),
  CONSTRAINT project_cash_requests_amount_positive CHECK (amount > 0),
  CONSTRAINT project_cash_requests_source_pair_check CHECK (
    (source_type IS NULL AND source_id IS NULL) OR
    (NULLIF(btrim(source_type),'') IS NOT NULL AND source_id IS NOT NULL)
  ),
  CONSTRAINT project_cash_requests_source_type_check CHECK (
    source_type IS NULL OR source_type IN (
      'procurement_request','incoming_acceptance','stock_document','document',
      'acceptance','work_item','contract_milestone'
    )
  ),
  CONSTRAINT project_cash_requests_approved_check CHECK (
    status NOT IN ('approved','handed_to_accounting','processed') OR (approved_by IS NOT NULL AND approved_at IS NOT NULL)
  ),
  CONSTRAINT project_cash_requests_handoff_check CHECK (
    status NOT IN ('handed_to_accounting','processed') OR (handed_to_accounting_by IS NOT NULL AND handed_to_accounting_at IS NOT NULL)
  ),
  CONSTRAINT project_cash_requests_processed_check CHECK (
    status <> 'processed' OR (processed_by IS NOT NULL AND processed_at IS NOT NULL)
  )
);

CREATE INDEX IF NOT EXISTS idx_project_cash_requests_project_status
  ON project_cash_requests(project_id,status,due_date NULLS LAST,created_at DESC);

INSERT INTO permissions (id,key,name,description,module)
SELECT gen_random_uuid(),key,name,description,'project_finance'
FROM (VALUES
  ('project_finance.view','Xem tài chính công trình','Xem hồ sơ và số liệu tài chính quản trị trong phạm vi công trình'),
  ('project_finance.cost_create','Ghi nhận chi phí vật tư','Tạo hồ sơ chi phí mua vật tư của công trình'),
  ('project_finance.cost_review','Xác nhận chi phí vật tư','Yêu cầu sửa hoặc xác nhận chi phí vật tư đủ điều kiện lên báo cáo quản trị'),
  ('project_finance.request_create','Lập đề nghị thu/chi','Lập hồ sơ đề nghị thu hoặc chi để bàn giao cho kế toán xử lý ngoài Consflow'),
  ('project_finance.request_approve','Duyệt đề nghị thu/chi','Duyệt hồ sơ đề nghị thu hoặc chi nội bộ'),
  ('project_finance.process','Xác nhận bàn giao kế toán','Xác nhận hồ sơ đã bàn giao hoặc đã được kế toán xử lý'),
  ('project_finance.report_view','Xem báo cáo quản trị công trình','Xem doanh thu đủ điều kiện, chi phí vật tư và lợi nhuận quản trị tạm tính')
) permission_seed(key,name,description)
ON CONFLICT (key) DO UPDATE
SET name=EXCLUDED.name,description=EXCLUDED.description,module=EXCLUDED.module;

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

WITH finance_steps(id,name,description,permission_key) AS (
  VALUES
    ('42500000-0000-0000-0000-000000000042'::uuid,'Lập hồ sơ đề nghị chi thầu phụ','Tập hợp hồ sơ nguồn và lập đề nghị chi nội bộ để cấp có thẩm quyền duyệt.','project_finance.request_create'),
    ('42500000-0000-0000-0000-000000000043'::uuid,'Duyệt đề nghị chi','Giám đốc kiểm tra hồ sơ nguồn và duyệt đề nghị chi nội bộ.','project_finance.request_approve'),
    ('42500000-0000-0000-0000-000000000044'::uuid,'Kiểm tra hồ sơ đề nghị chi','Kiểm tra đủ hợp đồng, nghiệm thu, giá trị đề nghị và chứng từ nguồn trước khi bàn giao kế toán.','project_finance.view'),
    ('42500000-0000-0000-0000-000000000045'::uuid,'Bàn giao hồ sơ chi cho Kế toán','Bàn giao hồ sơ đã duyệt để Kế toán xử lý trên hệ thống chuyên môn; Consflow chỉ theo dõi trạng thái hồ sơ.','project_finance.process'),
    ('42500000-0000-0000-0000-000000000048'::uuid,'Lập hồ sơ đề nghị thu từ chủ đầu tư','Tập hợp nghiệm thu, mốc hợp đồng và lập hồ sơ đề nghị thu nội bộ.','project_finance.request_create'),
    ('42500000-0000-0000-0000-000000000049'::uuid,'Bàn giao hồ sơ phát hành hóa đơn đầu ra','Bàn giao hồ sơ đủ điều kiện cho Kế toán phát hành hóa đơn trên hệ thống chuyên môn.','project_finance.process'),
    ('42500000-0000-0000-0000-000000000050'::uuid,'Theo dõi hồ sơ thu theo kỳ','Theo dõi tình trạng bàn giao và xử lý hồ sơ thu theo từng mốc của hợp đồng.','project_finance.view'),
    ('42500000-0000-0000-0000-000000000051'::uuid,'Ghi nhận chi phí mua vật tư công trình','Nhập giá trị mua vật tư theo hồ sơ nguồn; chỉ chi phí đã xác nhận mới lên báo cáo quản trị.','project_finance.cost_create'),
    ('42500000-0000-0000-0000-000000000052'::uuid,'Xác nhận hoàn tất hồ sơ tài chính quản trị','Kiểm tra đề nghị thu/chi và chi phí vật tư đã được xử lý trước khi đánh giá đủ điều kiện đóng công trình.','project_finance.report_view')
)
UPDATE pipeline_template_steps step
SET name=updates.name,description=updates.description,required_permission=updates.permission_key
FROM finance_steps updates
WHERE step.id=updates.id;

UPDATE pipeline_template_nodes node
SET name='Tài chính công trình',
    description='Hồ sơ thu/chi nội bộ, chi phí vật tư và số liệu quản trị công trình; không thay thế phần mềm kế toán chuyên môn.',
    required_permission='project_finance.view',
    updated_at=now()
WHERE node.name='Tài chính & Công nợ';

UPDATE pipeline_template_nodes
SET description='Nghiệm thu thầu phụ, tổ đội và vật tư; tập hợp hồ sơ nguồn cho đề nghị chi và ghi nhận chi phí mua vật tư.',
    permissions='["acceptance.view","acceptance.create","acceptance.approve","project_finance.view","project_finance.cost_create","project_finance.request_create"]'::jsonb,
    exit_conditions='{"text":"Nghiệm thu đầu vào và hồ sơ nguồn tài chính công trình đã hoàn tất"}'::jsonb,
    updated_at=now()
WHERE id='42400000-0000-0000-0000-000000000010'
  AND template_id='40000000-0000-0000-0000-000000000021';

UPDATE pipeline_template_nodes
SET description='Nghiệm thu công trình, tập hợp hồ sơ quyết toán và lập đề nghị thu nội bộ theo mốc hợp đồng.',
    permissions='["acceptance.view","acceptance.create","acceptance.approve","project_finance.view","project_finance.request_create"]'::jsonb,
    exit_conditions='{"text":"Nghiệm thu đầu ra và hồ sơ đề nghị thu đã hoàn tất"}'::jsonb,
    updated_at=now()
WHERE id='42400000-0000-0000-0000-000000000011'
  AND template_id='40000000-0000-0000-0000-000000000021';

UPDATE pipeline_template_nodes
SET name='Kết thúc: Bàn giao + Tài chính công trình',
    description='Bàn giao công trình và xác nhận hồ sơ tài chính quản trị đã hoàn tất; nghiệp vụ kế toán chuyên môn thực hiện ngoài Consflow.',
    required_permission='project_finance.view',
    permissions='["handover.view","handover.approve","project_finance.view","project_finance.report_view"]'::jsonb,
    exit_conditions='{"handover_included":true,"financial_closure_included":true,"text":"Bàn giao và hồ sơ tài chính công trình đã hoàn tất"}'::jsonb,
    approval_rule='Kế toán và Giám đốc xác nhận hồ sơ quản trị',
    updated_at=now()
WHERE id='42400000-0000-0000-0000-000000000012'
  AND template_id='40000000-0000-0000-0000-000000000021';

UPDATE pipeline_template_steps
SET name=CASE
      WHEN id='41200000-0000-0000-0000-000000000022' THEN 'Kiểm tra hồ sơ tài chính công trình'
      WHEN id='41200000-0000-0000-0000-000000000023' THEN 'Giám đốc duyệt đóng hồ sơ tài chính công trình'
      ELSE name
    END,
    description=CASE
      WHEN id='41200000-0000-0000-0000-000000000022' THEN 'Kiểm tra chi phí vật tư và trạng thái đề nghị thu/chi đã bàn giao.'
      WHEN id='41200000-0000-0000-0000-000000000023' THEN 'Giám đốc xác nhận hồ sơ quản trị đủ điều kiện đóng công trình.'
      ELSE description
    END,
    required_permission=CASE
      WHEN id='41200000-0000-0000-0000-000000000022' THEN 'project_finance.cost_review'
      WHEN id='41200000-0000-0000-0000-000000000023' THEN 'project_finance.request_approve'
      ELSE required_permission
    END
WHERE id IN (
  '41200000-0000-0000-0000-000000000022',
  '41200000-0000-0000-0000-000000000023'
);

UPDATE pipeline_template_nodes
SET name='Duyệt kế hoạch mua vật tư',
    description='PM và Giám đốc kiểm tra giá trị mua vật tư dự kiến trước khi đặt hàng.',
    responsible_role_ids='["PM","Giám đốc"]'::jsonb,
    required_permission='procurement.approve',
    exit_conditions='{"text":"Kế hoạch mua vật tư được phê duyệt"}'::jsonb,
    approval_rule='PM + Giám đốc duyệt',
    updated_at=now()
WHERE id='41000000-0000-0000-0000-000000000004';

UPDATE pipeline_template_nodes
SET description='QS bóc tách khối lượng và lập dự toán công trình.',
    exit_conditions='{"text":"BOQ và dự toán đã được duyệt"}'::jsonb,
    approval_rule='PM + QS duyệt',
    updated_at=now()
WHERE id='41000000-0000-0000-0000-000000000003';

UPDATE pipeline_template_nodes
SET responsible_role_ids='["Mua hàng","PM"]'::jsonb,
    approval_rule='PM duyệt',
    updated_at=now()
WHERE id='41000000-0000-0000-0000-000000000005';

UPDATE pipeline_template_nodes
SET description='Kiểm tra kế hoạch mua vật tư và nghiệm thu đầu vào.',
    exit_conditions='{"text":"Kế hoạch mua và vật tư đầu vào đã kiểm tra"}'::jsonb,
    approval_rule='PM + Giám đốc duyệt',
    updated_at=now()
WHERE id='41100000-0000-0000-0000-000000000003';

UPDATE pipeline_template_steps
SET name='Duyệt kế hoạch mua vật tư',
    description='Kiểm tra giá trị mua vật tư dự kiến trước khi đặt hàng.',
    required_role_ids='["QS","PM","Giám đốc"]'::jsonb,
    required_permission='procurement.approve'
WHERE id='41200000-0000-0000-0000-000000000011';

UPDATE pipeline_template_nodes
SET name='Tài chính công trình',
    description='Xác nhận chi phí vật tư và đề nghị thu/chi nội bộ đã được xử lý.',
    required_permission='project_finance.view',
    exit_conditions='{"text":"Chi phí vật tư và đề nghị thu/chi đã hoàn tất"}'::jsonb,
    approval_rule='Kế toán + Giám đốc xác nhận',
    updated_at=now()
WHERE id='41000000-0000-0000-0000-000000000010';

UPDATE pipeline_template_steps step
SET name=updates.name,description=updates.description,required_permission=updates.permission_key
FROM (VALUES
  (1,'Xác nhận chi phí mua vật tư','Kiểm tra các khoản mua vật tư đã được ghi nhận và xác nhận.','project_finance.cost_review'),
  (2,'Kiểm tra đề nghị thu/chi đã xử lý','Kiểm tra các hồ sơ nội bộ đã bàn giao Kế toán và được xử lý.','project_finance.process'),
  (3,'Đóng hồ sơ tài chính quản trị','Xác nhận số liệu quản trị đủ điều kiện đóng công trình.','project_finance.report_view')
) updates(order_index,name,description,permission_key)
WHERE step.node_id='41000000-0000-0000-0000-000000000010'
  AND step.order_index=updates.order_index;
