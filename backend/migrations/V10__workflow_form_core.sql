ALTER TABLE pipeline_template_steps
  ADD COLUMN IF NOT EXISTS form_version_id UUID,
  ADD COLUMN IF NOT EXISTS form_required BOOLEAN NOT NULL DEFAULT false;

CREATE TABLE IF NOT EXISTS workflow_form_templates (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  code TEXT NOT NULL,
  name TEXT NOT NULL,
  description TEXT,
  form_type TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'active',
  created_by UUID REFERENCES users(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT workflow_form_templates_code_unique UNIQUE (code),
  CONSTRAINT workflow_form_templates_type_check CHECK (form_type IN (
    'checklist',
    'daily_report',
    'meeting_minutes',
    'generic_submission'
  )),
  CONSTRAINT workflow_form_templates_status_check CHECK (status IN ('active', 'archived'))
);

CREATE TABLE IF NOT EXISTS workflow_form_versions (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  template_id UUID NOT NULL REFERENCES workflow_form_templates(id) ON DELETE CASCADE,
  version_no INTEGER NOT NULL,
  status TEXT NOT NULL DEFAULT 'draft',
  schema_json JSONB NOT NULL DEFAULT '{}',
  validation_json JSONB NOT NULL DEFAULT '{}',
  published_at TIMESTAMPTZ,
  created_by UUID REFERENCES users(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT workflow_form_versions_template_version_unique UNIQUE (template_id, version_no),
  CONSTRAINT workflow_form_versions_id_template_unique UNIQUE (id, template_id),
  CONSTRAINT workflow_form_versions_version_positive CHECK (version_no > 0),
  CONSTRAINT workflow_form_versions_status_check CHECK (status IN ('draft', 'published', 'archived'))
);

CREATE TABLE IF NOT EXISTS workflow_form_fields (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  form_version_id UUID NOT NULL REFERENCES workflow_form_versions(id) ON DELETE CASCADE,
  field_key TEXT NOT NULL,
  label TEXT NOT NULL,
  field_type TEXT NOT NULL,
  required BOOLEAN NOT NULL DEFAULT false,
  options_json JSONB NOT NULL DEFAULT '[]',
  validation_json JSONB NOT NULL DEFAULT '{}',
  help_text TEXT,
  order_index INTEGER NOT NULL DEFAULT 0,
  visibility_condition_json JSONB NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT workflow_form_fields_version_key_unique UNIQUE (form_version_id, field_key),
  CONSTRAINT workflow_form_fields_key_format CHECK (field_key ~ '^[a-z][a-z0-9_]*$'),
  CONSTRAINT workflow_form_fields_type_check CHECK (field_type IN (
    'text',
    'number',
    'date',
    'textarea',
    'select',
    'multi_select',
    'checkbox',
    'evidence_reference'
  ))
);

ALTER TABLE pipeline_template_steps
  DROP CONSTRAINT IF EXISTS pipeline_template_steps_form_version_fk;

ALTER TABLE pipeline_template_steps
  ADD CONSTRAINT pipeline_template_steps_form_version_fk
  FOREIGN KEY (form_version_id) REFERENCES workflow_form_versions(id);

CREATE TABLE IF NOT EXISTS workflow_form_instances (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  work_item_id UUID REFERENCES work_items(id) ON DELETE CASCADE,
  pipeline_run_id UUID,
  run_node_id UUID,
  run_step_id UUID,
  template_step_id UUID REFERENCES pipeline_template_steps(id),
  form_template_id UUID NOT NULL,
  form_version_id UUID NOT NULL,
  form_type TEXT NOT NULL,
  title TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'draft',
  current_revision INTEGER NOT NULL DEFAULT 0,
  answers_snapshot_json JSONB NOT NULL DEFAULT '{}',
  required_role_ids JSONB NOT NULL DEFAULT '[]',
  created_by UUID REFERENCES users(id),
  submitted_by UUID REFERENCES users(id),
  submitted_at TIMESTAMPTZ,
  reopened_by UUID REFERENCES users(id),
  reopened_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT workflow_form_instances_template_fk
    FOREIGN KEY (form_version_id, form_template_id) REFERENCES workflow_form_versions(id, template_id),
  CONSTRAINT workflow_form_instances_work_item_project_fk
    FOREIGN KEY (work_item_id, project_id) REFERENCES work_items(id, project_id) ON DELETE CASCADE,
  CONSTRAINT workflow_form_instances_pipeline_run_project_fk
    FOREIGN KEY (pipeline_run_id, project_id) REFERENCES pipeline_runs(id, project_id) ON DELETE CASCADE,
  CONSTRAINT workflow_form_instances_run_node_project_fk
    FOREIGN KEY (run_node_id, project_id) REFERENCES pipeline_run_nodes(id, project_id) ON DELETE CASCADE,
  CONSTRAINT workflow_form_instances_run_step_project_fk
    FOREIGN KEY (run_step_id, project_id) REFERENCES pipeline_run_steps(id, project_id) ON DELETE CASCADE,
  CONSTRAINT workflow_form_instances_type_check CHECK (form_type IN (
    'checklist',
    'daily_report',
    'meeting_minutes',
    'generic_submission'
  )),
  CONSTRAINT workflow_form_instances_status_check CHECK (status IN ('draft', 'submitted', 'reopened', 'cancelled')),
  CONSTRAINT workflow_form_instances_revision_nonnegative CHECK (current_revision >= 0)
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_workflow_form_instances_work_item
  ON workflow_form_instances(work_item_id)
  WHERE work_item_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS ux_workflow_form_instances_run_step
  ON workflow_form_instances(run_step_id)
  WHERE run_step_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_workflow_form_instances_project_status
  ON workflow_form_instances(project_id, status, updated_at DESC);

CREATE TABLE IF NOT EXISTS workflow_form_answers (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  instance_id UUID NOT NULL REFERENCES workflow_form_instances(id) ON DELETE CASCADE,
  field_key TEXT NOT NULL,
  field_type TEXT NOT NULL,
  value_json JSONB NOT NULL DEFAULT 'null',
  revision_no INTEGER NOT NULL DEFAULT 0,
  updated_by UUID REFERENCES users(id),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT workflow_form_answers_instance_key_unique UNIQUE (instance_id, field_key),
  CONSTRAINT workflow_form_answers_key_format CHECK (field_key ~ '^[a-z][a-z0-9_]*$'),
  CONSTRAINT workflow_form_answers_revision_nonnegative CHECK (revision_no >= 0)
);

CREATE INDEX IF NOT EXISTS idx_workflow_form_answers_instance
  ON workflow_form_answers(instance_id, field_key);

CREATE TABLE IF NOT EXISTS workflow_form_events (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  instance_id UUID NOT NULL REFERENCES workflow_form_instances(id) ON DELETE CASCADE,
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  work_item_id UUID REFERENCES work_items(id) ON DELETE CASCADE,
  actor_id UUID REFERENCES users(id),
  event_type TEXT NOT NULL,
  from_status TEXT,
  to_status TEXT,
  comment TEXT,
  metadata_json JSONB NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT workflow_form_events_type_check CHECK (event_type IN (
    'created',
    'draft_saved',
    'submitted',
    'reopened',
    'cancelled'
  )),
  CONSTRAINT workflow_form_events_status_check CHECK (
    (from_status IS NULL OR from_status IN ('draft', 'submitted', 'reopened', 'cancelled')) AND
    (to_status IS NULL OR to_status IN ('draft', 'submitted', 'reopened', 'cancelled'))
  )
);

CREATE INDEX IF NOT EXISTS idx_workflow_form_events_instance_created
  ON workflow_form_events(instance_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_workflow_form_events_project_created
  ON workflow_form_events(project_id, created_at DESC);

WITH template_seed(code, name, description, form_type) AS (
  VALUES
    ('builtin_checklist', 'Checklist', 'Checklist nghiệp vụ dùng cho step cần xác nhận hạng mục.', 'checklist'),
    ('builtin_daily_report', 'Daily report', 'Báo cáo ngày có phần đã làm, sẽ làm và rủi ro.', 'daily_report'),
    ('builtin_meeting_minutes', 'Biên bản họp', 'Biên bản họp gồm nội dung, quyết định và action items.', 'meeting_minutes'),
    ('builtin_generic_submission', 'Generic submission', 'Biểu mẫu gửi kết quả cho step tổng quát.', 'generic_submission')
)
INSERT INTO workflow_form_templates (code, name, description, form_type, status)
SELECT code, name, description, form_type, 'active'
FROM template_seed
ON CONFLICT (code) DO UPDATE
SET name=EXCLUDED.name,
    description=EXCLUDED.description,
    form_type=EXCLUDED.form_type,
    status='active',
    updated_at=now();

INSERT INTO workflow_form_versions (template_id, version_no, status, schema_json, published_at)
SELECT id, 1, 'published', jsonb_build_object('source', 'builtin', 'code', code), now()
FROM workflow_form_templates
WHERE code IN ('builtin_checklist', 'builtin_daily_report', 'builtin_meeting_minutes', 'builtin_generic_submission')
ON CONFLICT (template_id, version_no) DO UPDATE
SET status='published',
    schema_json=EXCLUDED.schema_json,
    published_at=COALESCE(workflow_form_versions.published_at, EXCLUDED.published_at),
    updated_at=now();

WITH field_seed(code, field_key, label, field_type, required, options_json, validation_json, help_text, order_index) AS (
  VALUES
    ('builtin_checklist', 'items_done', 'Hạng mục đã hoàn tất', 'textarea', true, '[]'::jsonb, '{"min_length": 3}'::jsonb, 'Ghi rõ các checklist item đã kiểm tra và kết quả.', 10),
    ('builtin_checklist', 'exceptions', 'Ngoại lệ / tồn tại', 'textarea', false, '[]'::jsonb, '{}'::jsonb, 'Ghi tồn tại nếu có.', 20),
    ('builtin_checklist', 'confirmed', 'Xác nhận hoàn tất', 'checkbox', true, '[]'::jsonb, '{}'::jsonb, 'Bắt buộc tick khi checklist đã được kiểm tra.', 30),

    ('builtin_daily_report', 'report_date', 'Ngày báo cáo', 'date', true, '[]'::jsonb, '{}'::jsonb, NULL, 10),
    ('builtin_daily_report', 'done_today', 'Đã làm', 'textarea', true, '[]'::jsonb, '{"min_length": 3}'::jsonb, 'Tóm tắt khối lượng hoặc công việc đã thực hiện.', 20),
    ('builtin_daily_report', 'next_plan', 'Sẽ làm', 'textarea', false, '[]'::jsonb, '{}'::jsonb, 'Kế hoạch tiếp theo hoặc blocker cần xử lý.', 30),
    ('builtin_daily_report', 'risk_note', 'Rủi ro / blocker', 'textarea', false, '[]'::jsonb, '{}'::jsonb, NULL, 40),

    ('builtin_meeting_minutes', 'meeting_date', 'Ngày họp', 'date', true, '[]'::jsonb, '{}'::jsonb, NULL, 10),
    ('builtin_meeting_minutes', 'attendees', 'Thành phần tham dự', 'textarea', true, '[]'::jsonb, '{"min_length": 3}'::jsonb, 'Ghi tên/role của người tham dự.', 20),
    ('builtin_meeting_minutes', 'agenda', 'Nội dung chính', 'textarea', true, '[]'::jsonb, '{"min_length": 3}'::jsonb, NULL, 30),
    ('builtin_meeting_minutes', 'decisions', 'Quyết định', 'textarea', false, '[]'::jsonb, '{}'::jsonb, NULL, 40),
    ('builtin_meeting_minutes', 'action_items', 'Action items', 'textarea', false, '[]'::jsonb, '{}'::jsonb, 'Ghi owner và deadline nếu có.', 50),

    ('builtin_generic_submission', 'summary', 'Kết quả thực hiện', 'textarea', true, '[]'::jsonb, '{"min_length": 3}'::jsonb, 'Tóm tắt kết quả đủ rõ để audit lại.', 10),
    ('builtin_generic_submission', 'next_action', 'Kế tiếp', 'textarea', false, '[]'::jsonb, '{}'::jsonb, NULL, 20)
)
INSERT INTO workflow_form_fields (form_version_id, field_key, label, field_type, required, options_json, validation_json, help_text, order_index)
SELECT v.id, fs.field_key, fs.label, fs.field_type, fs.required, fs.options_json, fs.validation_json, fs.help_text, fs.order_index
FROM field_seed fs
JOIN workflow_form_templates t ON t.code=fs.code
JOIN workflow_form_versions v ON v.template_id=t.id AND v.version_no=1
ON CONFLICT (form_version_id, field_key) DO UPDATE
SET label=EXCLUDED.label,
    field_type=EXCLUDED.field_type,
    required=EXCLUDED.required,
    options_json=EXCLUDED.options_json,
    validation_json=EXCLUDED.validation_json,
    help_text=EXCLUDED.help_text,
    order_index=EXCLUDED.order_index;

WITH builtins AS (
  SELECT t.code, v.id AS version_id
  FROM workflow_form_templates t
  JOIN workflow_form_versions v ON v.template_id=t.id AND v.version_no=1
)
UPDATE pipeline_template_steps pts
SET form_version_id = CASE
      WHEN pts.step_type='daily_report' THEN (SELECT version_id FROM builtins WHERE code='builtin_daily_report')
      WHEN pts.step_type='meeting_minutes' THEN (SELECT version_id FROM builtins WHERE code='builtin_meeting_minutes')
      WHEN pts.step_type IN ('checklist', 'acceptance_check', 'handover_check') OR pts.checklist_required THEN (SELECT version_id FROM builtins WHERE code='builtin_checklist')
      ELSE (SELECT version_id FROM builtins WHERE code='builtin_generic_submission')
    END,
    form_required = pts.step_type IN ('daily_report', 'meeting_minutes', 'checklist', 'acceptance_check', 'handover_check') OR pts.checklist_required
WHERE pts.form_version_id IS NULL;

INSERT INTO workflow_form_instances (
  project_id,
  work_item_id,
  pipeline_run_id,
  run_node_id,
  run_step_id,
  template_step_id,
  form_template_id,
  form_version_id,
  form_type,
  title,
  required_role_ids,
  created_by
)
SELECT
  wi.project_id,
  wi.id,
  wi.pipeline_run_id,
  wi.run_node_id,
  wi.run_step_id,
  prs.template_step_id,
  v.template_id,
  v.id,
  t.form_type,
  wi.title,
  wi.role_ids,
  wi.created_by
FROM work_items wi
JOIN pipeline_run_steps prs ON prs.id=wi.run_step_id AND prs.project_id=wi.project_id
JOIN pipeline_template_steps pts ON pts.id=prs.template_step_id
JOIN workflow_form_versions v ON v.id=pts.form_version_id
JOIN workflow_form_templates t ON t.id=v.template_id
WHERE wi.source_type='pipeline_run_step'
  AND wi.status <> 'cancelled'
  AND NOT EXISTS (
    SELECT 1
    FROM workflow_form_instances wfi
    WHERE wfi.work_item_id=wi.id
  );

INSERT INTO workflow_form_events (instance_id, project_id, work_item_id, actor_id, event_type, to_status, comment, metadata_json)
SELECT id, project_id, work_item_id, created_by, 'created', status, 'Workflow form instance created from migration backfill', '{"source":"migration_v10"}'::jsonb
FROM workflow_form_instances wfi
WHERE NOT EXISTS (
  SELECT 1
  FROM workflow_form_events wfe
  WHERE wfe.instance_id=wfi.id AND wfe.event_type='created'
);
