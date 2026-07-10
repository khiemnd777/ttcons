CREATE TABLE IF NOT EXISTS contracts (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  contract_code TEXT NOT NULL,
  title TEXT NOT NULL,
  contract_type TEXT NOT NULL,
  direction TEXT NOT NULL,
  counterparty_partner_id UUID REFERENCES partners(id),
  counterparty_name TEXT NOT NULL,
  currency TEXT NOT NULL DEFAULT 'VND',
  original_amount NUMERIC(18,2) NOT NULL DEFAULT 0,
  current_amount NUMERIC(18,2) NOT NULL DEFAULT 0,
  advance_percent NUMERIC(5,2) NOT NULL DEFAULT 0,
  retention_percent NUMERIC(5,2) NOT NULL DEFAULT 0,
  warranty_days INTEGER NOT NULL DEFAULT 0,
  signed_date DATE,
  effective_date DATE,
  expiry_date DATE,
  status TEXT NOT NULL DEFAULT 'draft',
  current_version_id UUID,
  approval_request_id UUID REFERENCES approval_requests(id),
  created_by UUID REFERENCES users(id),
  approved_by UUID REFERENCES users(id),
  signed_by UUID REFERENCES users(id),
  activated_by UUID REFERENCES users(id),
  activated_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT contracts_project_code_unique UNIQUE (project_id, contract_code),
  CONSTRAINT contracts_type_check CHECK (contract_type IN ('main_client','subcontractor','supplier','service','other')),
  CONSTRAINT contracts_direction_check CHECK (direction IN ('receivable','payable')),
  CONSTRAINT contracts_status_check CHECK (status IN ('draft','review','approved','signed','active','suspended','completed','terminated')),
  CONSTRAINT contracts_currency_check CHECK (currency ~ '^[A-Z]{3}$'),
  CONSTRAINT contracts_amount_nonnegative CHECK (original_amount >= 0 AND current_amount >= 0),
  CONSTRAINT contracts_advance_percent_check CHECK (advance_percent >= 0 AND advance_percent <= 100),
  CONSTRAINT contracts_retention_percent_check CHECK (retention_percent >= 0 AND retention_percent <= 100),
  CONSTRAINT contracts_warranty_nonnegative CHECK (warranty_days >= 0),
  CONSTRAINT contracts_date_order_check CHECK (expiry_date IS NULL OR effective_date IS NULL OR expiry_date >= effective_date)
);

CREATE INDEX IF NOT EXISTS idx_contracts_project_status ON contracts(project_id, status, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_contracts_counterparty ON contracts(counterparty_partner_id, project_id);

CREATE UNIQUE INDEX IF NOT EXISTS idx_contracts_one_active_main_client
  ON contracts(project_id)
  WHERE contract_type='main_client' AND status='active';

CREATE TABLE IF NOT EXISTS contract_versions (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  contract_id UUID NOT NULL REFERENCES contracts(id) ON DELETE CASCADE,
  version_no INTEGER NOT NULL,
  version_label TEXT NOT NULL,
  source_variation_id UUID,
  amount NUMERIC(18,2) NOT NULL,
  scope_summary TEXT NOT NULL,
  commercial_terms_json JSONB NOT NULL DEFAULT '{}',
  payment_terms_json JSONB NOT NULL DEFAULT '{}',
  planned_start_date DATE,
  planned_end_date DATE,
  document_id UUID REFERENCES documents(id),
  definition_hash TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'draft',
  review_comment TEXT,
  signature_comment TEXT,
  submitted_by UUID REFERENCES users(id),
  submitted_at TIMESTAMPTZ,
  approved_by UUID REFERENCES users(id),
  approved_at TIMESTAMPTZ,
  signed_by UUID REFERENCES users(id),
  signed_at TIMESTAMPTZ,
  created_by UUID REFERENCES users(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT contract_versions_number_unique UNIQUE (contract_id, version_no),
  CONSTRAINT contract_versions_number_positive CHECK (version_no > 0),
  CONSTRAINT contract_versions_amount_nonnegative CHECK (amount >= 0),
  CONSTRAINT contract_versions_status_check CHECK (status IN ('draft','review','approved','signed','rejected')),
  CONSTRAINT contract_versions_terms_objects CHECK (jsonb_typeof(commercial_terms_json)='object' AND jsonb_typeof(payment_terms_json)='object'),
  CONSTRAINT contract_versions_date_order_check CHECK (planned_end_date IS NULL OR planned_start_date IS NULL OR planned_end_date >= planned_start_date)
);

ALTER TABLE contracts
  ADD CONSTRAINT contracts_current_version_fk FOREIGN KEY (current_version_id) REFERENCES contract_versions(id);

CREATE INDEX IF NOT EXISTS idx_contract_versions_contract_status ON contract_versions(contract_id, status, version_no DESC);

CREATE TABLE IF NOT EXISTS contract_milestones (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  contract_id UUID NOT NULL REFERENCES contracts(id) ON DELETE CASCADE,
  contract_version_id UUID NOT NULL REFERENCES contract_versions(id),
  milestone_code TEXT NOT NULL,
  milestone_name TEXT NOT NULL,
  sequence_no INTEGER NOT NULL,
  milestone_type TEXT NOT NULL,
  amount_basis TEXT NOT NULL DEFAULT 'fixed',
  percentage NUMERIC(7,4),
  gross_amount NUMERIC(18,2) NOT NULL,
  retention_percent NUMERIC(5,2) NOT NULL DEFAULT 0,
  retention_amount NUMERIC(18,2) NOT NULL DEFAULT 0,
  net_amount NUMERIC(18,2) NOT NULL,
  planned_date DATE,
  due_days INTEGER NOT NULL DEFAULT 0,
  acceptance_required BOOLEAN NOT NULL DEFAULT false,
  acceptance_id UUID REFERENCES acceptance_records(id),
  status TEXT NOT NULL DEFAULT 'draft',
  obligation_type TEXT,
  obligation_id UUID,
  eligible_by UUID REFERENCES users(id),
  eligible_at TIMESTAMPTZ,
  created_by UUID REFERENCES users(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT contract_milestones_code_unique UNIQUE (contract_id, milestone_code),
  CONSTRAINT contract_milestones_sequence_unique UNIQUE (contract_id, sequence_no),
  CONSTRAINT contract_milestones_type_check CHECK (milestone_type IN ('advance','progress','final','retention_release','other')),
  CONSTRAINT contract_milestones_basis_check CHECK (amount_basis IN ('fixed','percentage','quantity_certificate')),
  CONSTRAINT contract_milestones_status_check CHECK (status IN ('draft','active','eligible','obligation_opened','paid','cancelled')),
  CONSTRAINT contract_milestones_obligation_type_check CHECK (obligation_type IS NULL OR obligation_type IN ('payable','receivable')),
  CONSTRAINT contract_milestones_sequence_positive CHECK (sequence_no > 0),
  CONSTRAINT contract_milestones_amount_nonnegative CHECK (gross_amount >= 0 AND retention_amount >= 0 AND net_amount >= 0),
  CONSTRAINT contract_milestones_net_consistency CHECK (net_amount = gross_amount - retention_amount),
  CONSTRAINT contract_milestones_retention_check CHECK (retention_percent >= 0 AND retention_percent <= 100 AND retention_amount <= gross_amount),
  CONSTRAINT contract_milestones_percentage_check CHECK (percentage IS NULL OR (percentage > 0 AND percentage <= 100)),
  CONSTRAINT contract_milestones_due_days_nonnegative CHECK (due_days >= 0),
  CONSTRAINT contract_milestones_obligation_consistency CHECK ((obligation_type IS NULL AND obligation_id IS NULL) OR (obligation_type IS NOT NULL AND obligation_id IS NOT NULL))
);

CREATE INDEX IF NOT EXISTS idx_contract_milestones_contract_status ON contract_milestones(contract_id, status, sequence_no);
CREATE UNIQUE INDEX IF NOT EXISTS idx_contract_milestones_obligation_unique ON contract_milestones(obligation_type, obligation_id) WHERE obligation_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS contract_variations (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  contract_id UUID NOT NULL REFERENCES contracts(id) ON DELETE CASCADE,
  variation_code TEXT NOT NULL,
  title TEXT NOT NULL,
  variation_type TEXT NOT NULL,
  reason TEXT NOT NULL,
  amount_delta NUMERIC(18,2) NOT NULL DEFAULT 0,
  days_delta INTEGER NOT NULL DEFAULT 0,
  scope_delta TEXT,
  document_id UUID REFERENCES documents(id),
  status TEXT NOT NULL DEFAULT 'draft',
  approval_request_id UUID REFERENCES approval_requests(id),
  resulting_version_id UUID REFERENCES contract_versions(id),
  submitted_by UUID REFERENCES users(id),
  submitted_at TIMESTAMPTZ,
  approved_by UUID REFERENCES users(id),
  approved_at TIMESTAMPTZ,
  rejected_by UUID REFERENCES users(id),
  rejected_at TIMESTAMPTZ,
  incorporated_by UUID REFERENCES users(id),
  incorporated_at TIMESTAMPTZ,
  created_by UUID REFERENCES users(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT contract_variations_code_unique UNIQUE (contract_id, variation_code),
  CONSTRAINT contract_variations_type_check CHECK (variation_type IN ('scope','price','time','mixed')),
  CONSTRAINT contract_variations_status_check CHECK (status IN ('draft','review','approved','rejected','incorporated')),
  CONSTRAINT contract_variations_result_consistency CHECK ((status='incorporated' AND resulting_version_id IS NOT NULL) OR status <> 'incorporated')
);

ALTER TABLE contract_versions
  ADD CONSTRAINT contract_versions_source_variation_fk FOREIGN KEY (source_variation_id) REFERENCES contract_variations(id);

CREATE INDEX IF NOT EXISTS idx_contract_variations_contract_status ON contract_variations(contract_id, status, updated_at DESC);

CREATE TABLE IF NOT EXISTS contract_retentions (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  contract_id UUID NOT NULL REFERENCES contracts(id) ON DELETE CASCADE,
  source_milestone_id UUID NOT NULL REFERENCES contract_milestones(id),
  direction TEXT NOT NULL,
  held_amount NUMERIC(18,2) NOT NULL,
  released_amount NUMERIC(18,2) NOT NULL DEFAULT 0,
  outstanding_amount NUMERIC(18,2) GENERATED ALWAYS AS (held_amount - released_amount) STORED,
  warranty_end_date DATE,
  status TEXT NOT NULL DEFAULT 'held',
  release_obligation_type TEXT,
  release_obligation_id UUID,
  released_by UUID REFERENCES users(id),
  released_at TIMESTAMPTZ,
  release_comment TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT contract_retentions_source_unique UNIQUE (source_milestone_id),
  CONSTRAINT contract_retentions_direction_check CHECK (direction IN ('payable','receivable')),
  CONSTRAINT contract_retentions_status_check CHECK (status IN ('held','partially_released','released','forfeited')),
  CONSTRAINT contract_retentions_amount_check CHECK (held_amount > 0 AND released_amount >= 0 AND released_amount <= held_amount),
  CONSTRAINT contract_retentions_release_type_check CHECK (release_obligation_type IS NULL OR release_obligation_type IN ('payable','receivable')),
  CONSTRAINT contract_retentions_release_consistency CHECK ((release_obligation_type IS NULL AND release_obligation_id IS NULL) OR (release_obligation_type IS NOT NULL AND release_obligation_id IS NOT NULL))
);

CREATE INDEX IF NOT EXISTS idx_contract_retentions_contract_status ON contract_retentions(contract_id, status, warranty_end_date);

CREATE TABLE IF NOT EXISTS contract_retention_releases (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  retention_id UUID NOT NULL REFERENCES contract_retentions(id) ON DELETE CASCADE,
  release_no INTEGER NOT NULL,
  amount NUMERIC(18,2) NOT NULL,
  due_date DATE,
  comment TEXT NOT NULL,
  obligation_type TEXT NOT NULL,
  obligation_id UUID NOT NULL,
  released_by UUID REFERENCES users(id),
  released_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT contract_retention_releases_number_unique UNIQUE (retention_id, release_no),
  CONSTRAINT contract_retention_releases_amount_positive CHECK (amount > 0),
  CONSTRAINT contract_retention_releases_obligation_type_check CHECK (obligation_type IN ('payable','receivable')),
  CONSTRAINT contract_retention_releases_obligation_unique UNIQUE (obligation_type, obligation_id)
);

CREATE INDEX IF NOT EXISTS idx_contract_retention_releases_retention ON contract_retention_releases(retention_id, release_no);

CREATE OR REPLACE FUNCTION prevent_final_contract_version_mutation()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF OLD.status = 'signed' THEN
    RAISE EXCEPTION 'signed contract version is immutable';
  END IF;
  RETURN COALESCE(NEW, OLD);
END;
$$;

DROP TRIGGER IF EXISTS trg_contract_version_immutable ON contract_versions;
CREATE TRIGGER trg_contract_version_immutable
BEFORE UPDATE OR DELETE ON contract_versions
FOR EACH ROW EXECUTE FUNCTION prevent_final_contract_version_mutation();

CREATE OR REPLACE FUNCTION prevent_final_contract_variation_mutation()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF OLD.status IN ('rejected','incorporated') THEN
    RAISE EXCEPTION 'final contract variation is immutable';
  END IF;
  RETURN COALESCE(NEW, OLD);
END;
$$;

DROP TRIGGER IF EXISTS trg_contract_variation_immutable ON contract_variations;
CREATE TRIGGER trg_contract_variation_immutable
BEFORE UPDATE OR DELETE ON contract_variations
FOR EACH ROW EXECUTE FUNCTION prevent_final_contract_variation_mutation();

INSERT INTO permissions (id, key, name, description, module)
SELECT gen_random_uuid(), key, name, description, 'contract'
FROM (VALUES
  ('contract.view','Xem hợp đồng','Xem hợp đồng, phiên bản, milestone, phụ lục và retention'),
  ('contract.create','Tạo và sửa hợp đồng','Tạo hợp đồng và chỉnh sửa phiên bản nháp'),
  ('contract.review','Review hợp đồng','Gửi và review phiên bản hợp đồng/phụ lục'),
  ('contract.approve','Duyệt hợp đồng','Duyệt phiên bản, phụ lục và milestone'),
  ('contract.sign','Ký và kích hoạt hợp đồng','Xác nhận ký và kích hoạt hợp đồng'),
  ('contract.milestone_manage','Quản lý milestone hợp đồng','Quản lý điều kiện và mở nghĩa vụ AP/AR từ milestone'),
  ('contract.retention_release','Giải phóng giữ lại','Mở nghĩa vụ thanh toán/thu tiền phần giữ lại')
) p(key,name,description)
ON CONFLICT (key) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r CROSS JOIN permissions p
WHERE r.name='admin' AND p.module='contract'
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r JOIN permissions p ON p.module='contract'
WHERE
  (r.name='director' AND p.key IN ('contract.view','contract.review','contract.approve','contract.sign','contract.milestone_manage','contract.retention_release')) OR
  (r.name='pm' AND p.key IN ('contract.view','contract.create','contract.review','contract.milestone_manage')) OR
  (r.name='qs' AND p.key IN ('contract.view','contract.create','contract.review','contract.milestone_manage')) OR
  (r.name='accounting' AND p.key IN ('contract.view','contract.review','contract.milestone_manage','contract.retention_release')) OR
  (r.name='client' AND p.key='contract.view')
ON CONFLICT DO NOTHING;
