-- Official workflow Revision 7: reduce repeated administrative capture and
-- keep business-source revisions explicit. Rev6 and every historical runtime
-- remain immutable.

ALTER TABLE documents
  ADD COLUMN IF NOT EXISTS revision_no INTEGER NOT NULL DEFAULT 1,
  ADD COLUMN IF NOT EXISTS release_status TEXT NOT NULL DEFAULT 'released',
  ADD COLUMN IF NOT EXISTS released_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS released_by UUID REFERENCES users(id);

ALTER TABLE documents
  DROP CONSTRAINT IF EXISTS documents_revision_no_check,
  DROP CONSTRAINT IF EXISTS documents_release_status_check;

ALTER TABLE documents
  ADD CONSTRAINT documents_revision_no_check CHECK (revision_no > 0),
  ADD CONSTRAINT documents_release_status_check
    CHECK (release_status IN ('draft','released','superseded','withdrawn'));

UPDATE documents
SET released_at=COALESCE(released_at,created_at),
    released_by=COALESCE(released_by,uploaded_by)
WHERE release_status='released' AND released_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_documents_business_release
  ON documents(project_id,target_type,target_id,document_type,revision_no DESC)
  WHERE status='active';

CREATE TABLE IF NOT EXISTS boq_releases (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  release_no INTEGER NOT NULL,
  status TEXT NOT NULL DEFAULT 'released',
  item_count INTEGER NOT NULL DEFAULT 0,
  total_amount NUMERIC(18,2) NOT NULL DEFAULT 0,
  definition_hash TEXT NOT NULL,
  items_snapshot_json JSONB NOT NULL DEFAULT '[]',
  released_by UUID REFERENCES users(id),
  released_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(project_id,release_no),
  CHECK (release_no > 0),
  CHECK (status IN ('released','superseded','withdrawn')),
  CHECK (definition_hash ~ '^[a-f0-9]{64}$')
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_boq_releases_current
  ON boq_releases(project_id)
  WHERE status='released';

CREATE TEMP TABLE rev7_removed_field_keys (field_key TEXT PRIMARY KEY) ON COMMIT DROP;
INSERT INTO rev7_removed_field_keys (field_key) VALUES
  ('confirmed'),('progress_percent'),('site_photo_evidence_id'),('output_invoice_no'),
  ('layout_version'),('render_version'),('revision_no'),('boq_version'),('quote_version'),
  ('quote_items'),('management_rate'),('margin_status'),('price_variance_note'),
  ('contract_value'),('payment_terms'),('review_items'),('takeoff_source'),
  ('input_quantity_items'),('procurement_readiness'),('sourcing_items'),
  ('negotiation_items'),('contract_partner'),('advance_amount'),('settlement_amount'),
  ('collection_period'),('received_amount'),('payment_period'),('payment_amount'),
  ('paid_amount'),('review_status'),('invoice_status'),('invoice_no'),
  ('input_invoice_no'),('invoice_amount'),('closure_checks'),('internal_decision'),('signer');

CREATE TEMP TABLE rev7_action_form_codes (form_code TEXT PRIMARY KEY) ON COMMIT DROP;
INSERT INTO rev7_action_form_codes (form_code) VALUES
  ('client_design_owner_approval'),
  ('client_director_management_rate_review'),
  ('client_procurement_input_approval'),
  ('client_director_subcontractor_quantity_approval'),
  ('client_payment_acceptance_approval'),
  ('client_outgoing_acceptance_director_approval');

CREATE TEMP TABLE rev7_source_versions ON COMMIT DROP AS
SELECT DISTINCT
  version.id AS source_version_id,
  version.template_id AS form_template_id,
  template.code AS form_code
FROM pipeline_template_nodes node
JOIN pipeline_template_steps step ON step.node_id=node.id
JOIN workflow_form_versions version ON version.id=step.form_version_id
JOIN workflow_form_templates template ON template.id=version.template_id
WHERE node.template_id='40000000-0000-0000-0000-000000000025'
  AND step.business_code<>'planning.assignment_meeting';

INSERT INTO workflow_form_versions (
  template_id,version_no,status,schema_json,validation_json,published_at,created_by
)
SELECT
  source.form_template_id,
  (SELECT COALESCE(MAX(existing.version_no),0)+1
   FROM workflow_form_versions existing
   WHERE existing.template_id=source.form_template_id),
  'published',
  version.schema_json || jsonb_build_object(
    'variant','operational_capture_v3',
    'source_revision',7,
    'capture_policy','business_input_only'
  ),
  version.validation_json || jsonb_build_object(
    'source_template_id','40000000-0000-0000-0000-000000000026',
    'source_form_version_id',source.source_version_id,
    'optional_section_label','Thông tin bổ sung'
  ),
  now(),
  version.created_by
FROM rev7_source_versions source
JOIN workflow_form_versions version ON version.id=source.source_version_id
WHERE EXISTS (
  SELECT 1
  FROM workflow_form_fields field
  WHERE field.form_version_id=source.source_version_id
    AND NOT EXISTS (SELECT 1 FROM rev7_removed_field_keys removed WHERE removed.field_key=field.field_key)
    AND NOT EXISTS (SELECT 1 FROM rev7_action_form_codes removed WHERE removed.form_code=source.form_code)
)
  AND NOT EXISTS (
    SELECT 1
    FROM workflow_form_versions existing
    WHERE existing.template_id=source.form_template_id
      AND existing.schema_json->>'variant'='operational_capture_v3'
      AND existing.validation_json->>'source_template_id'='40000000-0000-0000-0000-000000000026'
  );

CREATE TEMP TABLE rev7_form_versions ON COMMIT DROP AS
SELECT
  (version.validation_json->>'source_form_version_id')::uuid AS source_version_id,
  version.id AS target_version_id
FROM workflow_form_versions version
WHERE version.schema_json->>'variant'='operational_capture_v3'
  AND version.validation_json->>'source_template_id'='40000000-0000-0000-0000-000000000026';

INSERT INTO workflow_form_fields (
  form_version_id,field_key,label,field_type,required,options_json,
  validation_json,help_text,order_index,visibility_condition_json
)
SELECT
  target.target_version_id,
  field.field_key,
  field.label,
  field.field_type,
  field.required,
  field.options_json,
  field.validation_json
    || CASE
      WHEN NOT field.required
        AND NOT (field.validation_json ? 'required_when')
      THEN '{"capture_mode":"optional"}'::jsonb
      ELSE '{}'::jsonb
    END
    || CASE
      WHEN field.field_key IN ('measurement_date','survey_date','meeting_date','report_date','check_date')
        THEN '{"default_source":"current_date"}'::jsonb
      WHEN field.field_key IN ('reported_by','company_representative')
        THEN '{"default_source":"actor_name"}'::jsonb
      WHEN field.field_key='meeting_title'
        THEN '{"default_source":"step_title"}'::jsonb
      ELSE '{}'::jsonb
    END,
  field.help_text,
  field.order_index,
  field.visibility_condition_json
FROM rev7_source_versions source
JOIN rev7_form_versions target ON target.source_version_id=source.source_version_id
JOIN workflow_form_fields field ON field.form_version_id=source.source_version_id
WHERE NOT EXISTS (SELECT 1 FROM rev7_removed_field_keys removed WHERE removed.field_key=field.field_key)
  AND NOT EXISTS (SELECT 1 FROM rev7_action_form_codes removed WHERE removed.form_code=source.form_code)
ON CONFLICT (form_version_id,field_key) DO NOTHING;

INSERT INTO pipeline_templates (
  id,name,description,project_type,version,status,created_by,template_family_id,revision_no,
  source_template_id,engine_version,definition_hash,published_at,published_by,report_config_json
)
SELECT
  '40000000-0000-0000-0000-000000000026',
  source.name,
  'Revision 7: biểu mẫu vận hành chỉ giữ dữ liệu cần nhập; dữ liệu BOQ, hợp đồng, mua hàng, nghiệm thu và tài chính được đọc từ module nguồn có revision và gate fail-closed.',
  source.project_type,
  7,
  'published',
  source.created_by,
  source.template_family_id,
  7,
  source.id,
  source.engine_version,
  encode(digest('official-client-workflow-rev7-operational-forms','sha256'),'hex'),
  now(),
  NULL,
  source.report_config_json || jsonb_build_object(
    'capture_policy','business_input_only',
    'source_revision_policy','explicit',
    'removed_duplicate_step','planning.assignment_meeting'
  )
FROM pipeline_templates source
WHERE source.id='40000000-0000-0000-0000-000000000025'
  AND source.status='published'
ON CONFLICT (id) DO NOTHING;

INSERT INTO pipeline_template_nodes (
  id,template_id,business_code,name,description,type,position_x,position_y,order_index,
  responsible_role_ids,required_permission,permissions,entry_conditions,exit_conditions,
  approval_rule,approval_policy_json,rework_enabled,replanning_enabled,fail_path,status
)
SELECT
  replace(source.id::text,'46400000','47400000')::uuid,
  '40000000-0000-0000-0000-000000000026',
  source.business_code,source.name,source.description,source.type,source.position_x,source.position_y,
  source.order_index,source.responsible_role_ids,source.required_permission,source.permissions,
  source.entry_conditions,source.exit_conditions,source.approval_rule,source.approval_policy_json,
  source.rework_enabled,source.replanning_enabled,source.fail_path,source.status
FROM pipeline_template_nodes source
WHERE source.template_id='40000000-0000-0000-0000-000000000025'
ON CONFLICT (id) DO NOTHING;

INSERT INTO pipeline_template_steps (
  id,node_id,business_code,name,description,step_type,required_role_ids,required_permission,
  checklist_required,document_required,photo_required,form_version_id,form_required,
  surface_kind,domain_action_key,order_index
)
SELECT
  replace(source.id::text,'46500000','47500000')::uuid,
  replace(source.node_id::text,'46400000','47400000')::uuid,
  source.business_code,source.name,source.description,source.step_type,source.required_role_ids,
  source.required_permission,
  CASE WHEN target.target_version_id IS NULL THEN false ELSE source.checklist_required END,
  CASE WHEN target.target_version_id IS NULL THEN false ELSE source.document_required END,
  source.photo_required,
  target.target_version_id,
  target.target_version_id IS NOT NULL,
  source.surface_kind,
  CASE source.business_code
    WHEN 'design.client_approval' THEN 'source.design_documents_ready'
    WHEN 'boq_without_design.pricing' THEN 'source.boq_items_ready'
    WHEN 'boq_without_design.price_check' THEN 'source.boq_items_ready'
    WHEN 'boq_without_design.management_norm' THEN 'source.boq_approved'
    WHEN 'outbound_contract.prepare' THEN 'source.contract_version_ready'
    WHEN 'outbound_contract.review' THEN 'source.contract_review_ready'
    WHEN 'outbound_contract.app_sign' THEN 'source.contract_app_signed'
    WHEN 'input.takeoff' THEN 'source.boq_release_ready'
    WHEN 'input.price_sourcing' THEN 'source.supplier_quotation_ready'
    WHEN 'price_negotiation.complete' THEN 'source.supplier_selection_ready'
    WHEN 'input.contract_advance' THEN 'source.procurement_contract_ready'
    WHEN 'incoming.quantity_review' THEN 'source.subcontractor_reconciliation_ready'
    WHEN 'incoming.payment_request' THEN 'source.payable_ready'
    WHEN 'incoming.payment_approval' THEN 'source.payable_approved'
    WHEN 'incoming.payment_check' THEN 'source.input_invoice_verified'
    WHEN 'outgoing.file_signoff' THEN 'source.final_acceptance_ready'
    WHEN 'outgoing.collection_request' THEN 'source.receivable_ready'
    WHEN 'outgoing.collection_tracking' THEN 'source.receivable_collected'
    ELSE source.domain_action_key
  END,
  CASE
    WHEN source.business_code='planning.assignment_minutes' THEN 1
    WHEN source.business_code='planning.client_quality_file' THEN 2
    WHEN source.business_code='planning.construction_handoff' THEN 3
    ELSE source.order_index
  END
FROM pipeline_template_steps source
JOIN pipeline_template_nodes node ON node.id=source.node_id
LEFT JOIN rev7_form_versions target ON target.source_version_id=source.form_version_id
WHERE node.template_id='40000000-0000-0000-0000-000000000025'
  AND source.business_code<>'planning.assignment_meeting'
ON CONFLICT (id) DO NOTHING;

INSERT INTO pipeline_template_edges (
  id,template_id,from_node_id,to_node_id,condition_type,condition_config,label,priority
)
SELECT
  replace(source.id::text,'46600000','47600000')::uuid,
  '40000000-0000-0000-0000-000000000026',
  replace(source.from_node_id::text,'46400000','47400000')::uuid,
  replace(source.to_node_id::text,'46400000','47400000')::uuid,
  source.condition_type,source.condition_config,source.label,source.priority
FROM pipeline_template_edges source
WHERE source.template_id='40000000-0000-0000-0000-000000000025'
ON CONFLICT (id) DO NOTHING;

-- Only not-yet-started projects that explicitly selected Rev6 adopt Rev7.
UPDATE projects project
SET pipeline_template_id='40000000-0000-0000-0000-000000000026',updated_at=now()
WHERE project.pipeline_template_id='40000000-0000-0000-0000-000000000025'
  AND NOT EXISTS (SELECT 1 FROM pipeline_runs run WHERE run.project_id=project.id);

DO $$
DECLARE
  actual integer;
BEGIN
  SELECT count(*) INTO actual
  FROM pipeline_template_nodes
  WHERE template_id='40000000-0000-0000-0000-000000000026';
  IF actual<>15 THEN
    RAISE EXCEPTION 'expected Rev7 to contain 15 nodes, found %',actual;
  END IF;

  SELECT count(*) INTO actual
  FROM pipeline_template_steps step
  JOIN pipeline_template_nodes node ON node.id=step.node_id
  WHERE node.template_id='40000000-0000-0000-0000-000000000026';
  IF actual<>50 THEN
    RAISE EXCEPTION 'expected Rev7 to contain 50 steps, found %',actual;
  END IF;

  SELECT count(*) INTO actual
  FROM pipeline_template_nodes node
  JOIN pipeline_template_steps step ON step.node_id=node.id
  JOIN workflow_form_fields field ON field.form_version_id=step.form_version_id
  WHERE node.template_id='40000000-0000-0000-0000-000000000026';
  IF actual<>90 THEN
    RAISE EXCEPTION 'expected Rev7 to expose 90 field occurrences, found %',actual;
  END IF;

  SELECT count(*) INTO actual
  FROM pipeline_template_nodes node
  JOIN pipeline_template_steps step ON step.node_id=node.id
  WHERE node.template_id='40000000-0000-0000-0000-000000000026'
    AND step.business_code='planning.assignment_meeting';
  IF actual<>0 THEN
    RAISE EXCEPTION 'Rev7 still contains duplicate planning.assignment_meeting step';
  END IF;

  SELECT count(*) INTO actual
  FROM (
    SELECT step.id
    FROM pipeline_template_nodes node
    JOIN pipeline_template_steps step ON step.node_id=node.id
    LEFT JOIN workflow_form_fields field ON field.form_version_id=step.form_version_id
    WHERE node.template_id='40000000-0000-0000-0000-000000000026'
      AND step.form_required
    GROUP BY step.id
    HAVING count(field.id)=0
  ) empty_forms;
  IF actual<>0 THEN
    RAISE EXCEPTION 'Rev7 contains % required forms without fields',actual;
  END IF;
END $$;
