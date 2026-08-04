-- Stable workflow identity and the operational records required by the approved
-- client workflow revision. Published templates and existing runtime snapshots
-- are intentionally left unchanged by this migration.

ALTER TABLE projects
  ADD COLUMN IF NOT EXISTS workflow_entry_mode TEXT NOT NULL DEFAULT 'survey_required';

ALTER TABLE projects
  DROP CONSTRAINT IF EXISTS projects_workflow_entry_mode_check;

ALTER TABLE projects
  ADD CONSTRAINT projects_workflow_entry_mode_check
  CHECK (workflow_entry_mode IN ('survey_required','client_boq'));

ALTER TABLE pipeline_template_nodes
  ADD COLUMN IF NOT EXISTS business_code TEXT;

ALTER TABLE pipeline_template_steps
  ADD COLUMN IF NOT EXISTS business_code TEXT;

ALTER TABLE pipeline_run_nodes
  ADD COLUMN IF NOT EXISTS business_code TEXT;

ALTER TABLE pipeline_run_steps
  ADD COLUMN IF NOT EXISTS business_code TEXT;

ALTER TABLE documents
  ADD COLUMN IF NOT EXISTS mime_type TEXT,
  ADD COLUMN IF NOT EXISTS file_size BIGINT NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS storage_provider TEXT NOT NULL DEFAULT 'external',
  ADD COLUMN IF NOT EXISTS storage_key TEXT,
  ADD COLUMN IF NOT EXISTS checksum_sha256 TEXT;

ALTER TABLE documents
  DROP CONSTRAINT IF EXISTS documents_storage_provider_check,
  DROP CONSTRAINT IF EXISTS documents_storage_key_check,
  DROP CONSTRAINT IF EXISTS documents_checksum_check;

ALTER TABLE documents
  ADD CONSTRAINT documents_storage_provider_check CHECK (storage_provider IN ('external','local','s3')),
  ADD CONSTRAINT documents_storage_key_check CHECK (
    (storage_provider='external' AND storage_key IS NULL)
    OR
    (storage_provider IN ('local','s3') AND NULLIF(BTRIM(storage_key),'') IS NOT NULL)
  ),
  ADD CONSTRAINT documents_checksum_check CHECK (
    checksum_sha256 IS NULL OR checksum_sha256 ~ '^[a-f0-9]{64}$'
  );

CREATE INDEX IF NOT EXISTS idx_documents_protected_storage
  ON documents(storage_provider,storage_key)
  WHERE storage_key IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS ux_pipeline_template_nodes_business_code
  ON pipeline_template_nodes(template_id,business_code)
  WHERE NULLIF(BTRIM(business_code),'') IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS ux_pipeline_template_steps_business_code
  ON pipeline_template_steps(node_id,business_code)
  WHERE NULLIF(BTRIM(business_code),'') IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS ux_pipeline_run_nodes_business_code
  ON pipeline_run_nodes(run_id,business_code)
  WHERE NULLIF(BTRIM(business_code),'') IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS ux_pipeline_run_steps_business_code
  ON pipeline_run_steps(run_node_id,business_code)
  WHERE NULLIF(BTRIM(business_code),'') IS NOT NULL;

CREATE TABLE IF NOT EXISTS contract_version_signature_links (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  contract_version_id UUID NOT NULL REFERENCES contract_versions(id) ON DELETE CASCADE,
  workflow_form_signature_id UUID NOT NULL REFERENCES workflow_form_signatures(id),
  purpose TEXT NOT NULL DEFAULT 'approved_by',
  linked_by UUID REFERENCES users(id),
  linked_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT contract_version_signature_links_purpose_check CHECK (purpose='approved_by'),
  CONSTRAINT contract_version_signature_links_version_unique UNIQUE (contract_version_id),
  CONSTRAINT contract_version_signature_links_signature_unique UNIQUE (workflow_form_signature_id)
);

CREATE INDEX IF NOT EXISTS idx_contract_version_signature_links_version
  ON contract_version_signature_links(contract_version_id,linked_at DESC);

CREATE TABLE IF NOT EXISTS contract_dispatches (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  contract_id UUID NOT NULL REFERENCES contracts(id) ON DELETE CASCADE,
  contract_version_id UUID NOT NULL REFERENCES contract_versions(id),
  dispatch_no INTEGER NOT NULL,
  recipient_name TEXT NOT NULL,
  delivery_method TEXT NOT NULL DEFAULT 'courier',
  tracking_reference TEXT,
  sent_document_id UUID REFERENCES documents(id),
  returned_scan_document_id UUID REFERENCES documents(id),
  status TEXT NOT NULL DEFAULT 'sent',
  sent_by UUID REFERENCES users(id),
  sent_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  received_by UUID REFERENCES users(id),
  received_at TIMESTAMPTZ,
  note TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT contract_dispatches_number_unique UNIQUE (contract_id,dispatch_no),
  CONSTRAINT contract_dispatches_number_positive CHECK (dispatch_no > 0),
  CONSTRAINT contract_dispatches_method_check CHECK (delivery_method IN ('courier','direct','postal','other')),
  CONSTRAINT contract_dispatches_status_check CHECK (status IN ('sent','received')),
  CONSTRAINT contract_dispatches_received_consistency CHECK (
    (status='sent' AND received_at IS NULL AND returned_scan_document_id IS NULL)
    OR
    (status='received' AND received_at IS NOT NULL AND returned_scan_document_id IS NOT NULL)
  )
);

CREATE INDEX IF NOT EXISTS idx_contract_dispatches_contract
  ON contract_dispatches(contract_id,status,dispatch_no DESC);

ALTER TABLE contract_variations
  ADD COLUMN IF NOT EXISTS priority TEXT NOT NULL DEFAULT 'normal',
  ADD COLUMN IF NOT EXISTS source_work_item_id UUID REFERENCES work_items(id) ON DELETE SET NULL,
  ADD COLUMN IF NOT EXISTS qs_reviewed_by UUID REFERENCES users(id),
  ADD COLUMN IF NOT EXISTS qs_reviewed_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS qs_comment TEXT,
  ADD COLUMN IF NOT EXISTS purchasing_reviewed_by UUID REFERENCES users(id),
  ADD COLUMN IF NOT EXISTS purchasing_reviewed_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS purchasing_comment TEXT,
  ADD COLUMN IF NOT EXISTS procurement_request_id UUID REFERENCES procurement_requests(id) ON DELETE SET NULL;

ALTER TABLE contract_variations
  DROP CONSTRAINT IF EXISTS contract_variations_status_check,
  DROP CONSTRAINT IF EXISTS contract_variations_priority_check;

ALTER TABLE contract_variations
  ADD CONSTRAINT contract_variations_status_check CHECK (status IN (
    'draft','review','qs_review','purchasing_sourcing','pending_approval',
    'approved','rejected','procurement_created','incorporated'
  )),
  ADD CONSTRAINT contract_variations_priority_check CHECK (priority IN ('normal','urgent'));

CREATE TABLE IF NOT EXISTS contract_variation_events (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  variation_id UUID NOT NULL REFERENCES contract_variations(id) ON DELETE CASCADE,
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  event_type TEXT NOT NULL,
  from_status TEXT,
  to_status TEXT,
  actor_id UUID REFERENCES users(id),
  actor_name TEXT,
  comment TEXT,
  metadata_json JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT contract_variation_events_metadata_object_check CHECK (jsonb_typeof(metadata_json)='object')
);

CREATE INDEX IF NOT EXISTS idx_contract_variation_events_variation
  ON contract_variation_events(variation_id,created_at,id);

INSERT INTO permissions (id,key,name,description,module)
SELECT gen_random_uuid(),key,name,description,'contract'
FROM (VALUES
  ('contract.dispatch','Gửi hồ sơ hợp đồng giấy','Ghi nhận việc gửi hồ sơ hợp đồng giấy cho chủ đầu tư'),
  ('contract.scan_manage','Quản lý bản scan hợp đồng','Ghi nhận bản scan hợp đồng đã ký nhận từ chủ đầu tư'),
  ('contract.variation_qs_review','QS rà soát phát sinh','Rà soát khối lượng và ảnh hưởng thương mại của phát sinh'),
  ('contract.variation_purchase','Mua hàng xử lý phát sinh','Lấy giá và đề xuất phương án mua cho phát sinh')
) permission_seed(key,name,description)
ON CONFLICT (key) DO UPDATE
SET name=EXCLUDED.name,description=EXCLUDED.description,module=EXCLUDED.module;

INSERT INTO roles (id,name,display_name,description,system_role)
VALUES (
  '20000000-0000-0000-0000-000000000017',
  'contract_officer',
  'Bộ phận HĐ',
  'Phụ trách lập, luân chuyển, ký và lưu hồ sơ hợp đồng',
  true
)
ON CONFLICT (name) DO UPDATE
SET display_name=EXCLUDED.display_name,
    description=EXCLUDED.description,
    system_role=EXCLUDED.system_role;

INSERT INTO role_permissions (role_id,permission_id)
SELECT r.id,p.id
FROM roles r
JOIN permissions p ON
  (r.name='admin' AND p.key IN (
    'contract.dispatch','contract.scan_manage','contract.variation_qs_review','contract.variation_purchase'
  )) OR
  (r.name='contract_officer' AND p.key IN (
    'project.view','pipeline.view','contract.view','contract.create','contract.review',
    'contract.dispatch','contract.scan_manage'
  )) OR
  (r.name='qs' AND p.key='contract.variation_qs_review') OR
  (r.name='procurement' AND p.key IN (
    'contract.variation_purchase','contract.dispatch','contract.scan_manage'
  )) OR
  (r.name='pm' AND p.key IN ('contract.dispatch','contract.scan_manage','contract.approve'))
ON CONFLICT DO NOTHING;
