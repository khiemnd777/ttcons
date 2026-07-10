CREATE TABLE IF NOT EXISTS quality_itp_plans (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  family_id UUID NOT NULL,
  itp_code TEXT NOT NULL,
  title TEXT NOT NULL,
  discipline TEXT NOT NULL,
  work_package TEXT NOT NULL,
  revision_no INTEGER NOT NULL DEFAULT 1,
  supersedes_id UUID REFERENCES quality_itp_plans(id),
  definition_hash TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'draft',
  review_comment TEXT,
  document_id UUID REFERENCES documents(id),
  created_by UUID REFERENCES users(id),
  submitted_by UUID REFERENCES users(id),
  submitted_at TIMESTAMPTZ,
  approved_by UUID REFERENCES users(id),
  approved_at TIMESTAMPTZ,
  activated_by UUID REFERENCES users(id),
  activated_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT quality_itp_revision_unique UNIQUE (project_id, itp_code, revision_no),
  CONSTRAINT quality_itp_revision_positive CHECK (revision_no > 0),
  CONSTRAINT quality_itp_status_check CHECK (status IN ('draft','review','approved','active','superseded','rejected'))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_quality_itp_one_active_family ON quality_itp_plans(family_id) WHERE status='active';
CREATE INDEX IF NOT EXISTS idx_quality_itp_project_status ON quality_itp_plans(project_id,status,updated_at DESC);

CREATE TABLE IF NOT EXISTS quality_itp_items (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  itp_plan_id UUID NOT NULL REFERENCES quality_itp_plans(id) ON DELETE CASCADE,
  sequence_no INTEGER NOT NULL,
  activity_name TEXT NOT NULL,
  inspection_stage TEXT NOT NULL,
  acceptance_criteria TEXT NOT NULL,
  inspection_method TEXT NOT NULL,
  control_point TEXT NOT NULL DEFAULT 'review',
  frequency_type TEXT NOT NULL DEFAULT 'each_lot',
  frequency_value INTEGER NOT NULL DEFAULT 1,
  responsible_role TEXT NOT NULL,
  evidence_required BOOLEAN NOT NULL DEFAULT true,
  client_witness_required BOOLEAN NOT NULL DEFAULT false,
  notes TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT quality_itp_items_sequence_unique UNIQUE (itp_plan_id,sequence_no),
  CONSTRAINT quality_itp_items_sequence_positive CHECK (sequence_no > 0),
  CONSTRAINT quality_itp_items_control_check CHECK (control_point IN ('review','witness','hold','surveillance')),
  CONSTRAINT quality_itp_items_frequency_check CHECK (frequency_type IN ('each_lot','daily','weekly','quantity','custom') AND frequency_value > 0)
);

CREATE INDEX IF NOT EXISTS idx_quality_itp_items_plan ON quality_itp_items(itp_plan_id,sequence_no);

CREATE TABLE IF NOT EXISTS quality_inspection_requests (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  rfi_code TEXT NOT NULL,
  itp_plan_id UUID NOT NULL REFERENCES quality_itp_plans(id),
  itp_item_id UUID NOT NULL REFERENCES quality_itp_items(id),
  work_item_id UUID,
  location TEXT NOT NULL,
  lot_reference TEXT,
  requested_inspection_at TIMESTAMPTZ NOT NULL,
  status TEXT NOT NULL DEFAULT 'draft',
  result TEXT,
  request_comment TEXT,
  inspection_comment TEXT,
  request_evidence_id UUID REFERENCES evidence_records(id),
  inspection_evidence_id UUID REFERENCES evidence_records(id),
  requested_by UUID REFERENCES users(id),
  submitted_at TIMESTAMPTZ,
  inspector_id UUID REFERENCES users(id),
  inspected_at TIMESTAMPTZ,
  closed_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT quality_rfi_project_code_unique UNIQUE (project_id,rfi_code),
  CONSTRAINT quality_rfi_work_item_project_fk FOREIGN KEY (work_item_id,project_id) REFERENCES work_items(id,project_id) ON DELETE SET NULL,
  CONSTRAINT quality_rfi_status_check CHECK (status IN ('draft','submitted','scheduled','inspected','accepted','conditional','rejected','cancelled')),
  CONSTRAINT quality_rfi_result_check CHECK (result IS NULL OR result IN ('pass','conditional_pass','fail')),
  CONSTRAINT quality_rfi_result_status_consistency CHECK (
    (status IN ('accepted','conditional','rejected') AND result IS NOT NULL) OR
    (status NOT IN ('accepted','conditional','rejected'))
  )
);

CREATE INDEX IF NOT EXISTS idx_quality_rfi_project_status ON quality_inspection_requests(project_id,status,requested_inspection_at);
CREATE INDEX IF NOT EXISTS idx_quality_rfi_itp_item ON quality_inspection_requests(itp_item_id,status);
CREATE INDEX IF NOT EXISTS idx_quality_rfi_work_item ON quality_inspection_requests(work_item_id,status) WHERE work_item_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS quality_ncrs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  ncr_code TEXT NOT NULL,
  source_type TEXT NOT NULL,
  source_id UUID NOT NULL,
  rfi_id UUID REFERENCES quality_inspection_requests(id),
  work_item_id UUID,
  issue_id UUID REFERENCES issues(id),
  rework_request_id UUID REFERENCES rework_requests(id),
  title TEXT NOT NULL,
  severity TEXT NOT NULL,
  nonconformity TEXT NOT NULL,
  immediate_action TEXT,
  root_cause TEXT,
  corrective_action TEXT,
  preventive_action TEXT,
  due_date DATE,
  status TEXT NOT NULL DEFAULT 'open',
  evidence_id UUID REFERENCES evidence_records(id),
  corrective_evidence_id UUID REFERENCES evidence_records(id),
  reinspection_rfi_id UUID REFERENCES quality_inspection_requests(id),
  owner_id UUID REFERENCES users(id),
  created_by UUID REFERENCES users(id),
  root_cause_submitted_by UUID REFERENCES users(id),
  corrective_submitted_by UUID REFERENCES users(id),
  closed_by UUID REFERENCES users(id),
  closed_at TIMESTAMPTZ,
  closure_comment TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT quality_ncr_project_code_unique UNIQUE (project_id,ncr_code),
  CONSTRAINT quality_ncr_work_item_project_fk FOREIGN KEY (work_item_id,project_id) REFERENCES work_items(id,project_id) ON DELETE SET NULL,
  CONSTRAINT quality_ncr_source_check CHECK (source_type IN ('rfi','issue','work_item','manual')),
  CONSTRAINT quality_ncr_severity_check CHECK (severity IN ('low','medium','high','critical')),
  CONSTRAINT quality_ncr_status_check CHECK (status IN ('open','root_cause','corrective_action','reinspection','closed','cancelled'))
);

CREATE INDEX IF NOT EXISTS idx_quality_ncr_project_status ON quality_ncrs(project_id,status,due_date);
CREATE INDEX IF NOT EXISTS idx_quality_ncr_rfi ON quality_ncrs(rfi_id) WHERE rfi_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_quality_ncr_rework ON quality_ncrs(rework_request_id) WHERE rework_request_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS construction_recurring_plans (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  plan_code TEXT NOT NULL,
  title TEXT NOT NULL,
  description TEXT,
  recurrence_type TEXT NOT NULL,
  interval_value INTEGER NOT NULL DEFAULT 1,
  weekdays JSONB NOT NULL DEFAULT '[]',
  start_date DATE NOT NULL,
  end_date DATE,
  next_due_date DATE NOT NULL,
  due_time TIME,
  responsible_role TEXT NOT NULL,
  assigned_user_id UUID REFERENCES users(id),
  evidence_required BOOLEAN NOT NULL DEFAULT false,
  source_work_item_id UUID,
  status TEXT NOT NULL DEFAULT 'active',
  created_by UUID REFERENCES users(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT recurring_plan_project_code_unique UNIQUE (project_id,plan_code),
  CONSTRAINT recurring_plan_work_item_project_fk FOREIGN KEY (source_work_item_id,project_id) REFERENCES work_items(id,project_id) ON DELETE SET NULL,
  CONSTRAINT recurring_plan_type_check CHECK (recurrence_type IN ('daily','weekly','monthly','custom_days')),
  CONSTRAINT recurring_plan_interval_positive CHECK (interval_value > 0),
  CONSTRAINT recurring_plan_weekdays_array CHECK (jsonb_typeof(weekdays)='array'),
  CONSTRAINT recurring_plan_status_check CHECK (status IN ('active','paused','completed','cancelled')),
  CONSTRAINT recurring_plan_date_order_check CHECK (end_date IS NULL OR end_date >= start_date)
);

CREATE INDEX IF NOT EXISTS idx_recurring_plans_project_due ON construction_recurring_plans(project_id,status,next_due_date);

CREATE TABLE IF NOT EXISTS construction_recurring_occurrences (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  plan_id UUID NOT NULL REFERENCES construction_recurring_plans(id) ON DELETE CASCADE,
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  occurrence_date DATE NOT NULL,
  due_at TIMESTAMPTZ,
  assigned_user_id UUID REFERENCES users(id),
  status TEXT NOT NULL DEFAULT 'pending',
  completion_comment TEXT,
  evidence_id UUID REFERENCES evidence_records(id),
  completed_by UUID REFERENCES users(id),
  completed_at TIMESTAMPTZ,
  skipped_by UUID REFERENCES users(id),
  skipped_at TIMESTAMPTZ,
  skip_reason TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT recurring_occurrence_unique UNIQUE (plan_id,occurrence_date),
  CONSTRAINT recurring_occurrence_status_check CHECK (status IN ('pending','in_progress','completed','overdue','skipped'))
);

CREATE INDEX IF NOT EXISTS idx_recurring_occurrences_project_status ON construction_recurring_occurrences(project_id,status,occurrence_date);

CREATE OR REPLACE FUNCTION prevent_active_itp_mutation()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF OLD.status='superseded' THEN
    RAISE EXCEPTION 'active or superseded ITP revision is immutable';
  END IF;
  IF OLD.status='active' AND NOT (
    NEW.status='superseded'
    AND NEW.itp_code IS NOT DISTINCT FROM OLD.itp_code
    AND NEW.title IS NOT DISTINCT FROM OLD.title
    AND NEW.discipline IS NOT DISTINCT FROM OLD.discipline
    AND NEW.work_package IS NOT DISTINCT FROM OLD.work_package
    AND NEW.revision_no IS NOT DISTINCT FROM OLD.revision_no
    AND NEW.definition_hash IS NOT DISTINCT FROM OLD.definition_hash
    AND NEW.document_id IS NOT DISTINCT FROM OLD.document_id
  ) THEN
    RAISE EXCEPTION 'active or superseded ITP revision is immutable';
  END IF;
  RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS trg_quality_itp_immutable ON quality_itp_plans;
CREATE TRIGGER trg_quality_itp_immutable BEFORE UPDATE ON quality_itp_plans FOR EACH ROW EXECUTE FUNCTION prevent_active_itp_mutation();

CREATE OR REPLACE FUNCTION prevent_active_itp_item_mutation()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF EXISTS (SELECT 1 FROM quality_itp_plans p WHERE p.id=OLD.itp_plan_id AND p.status IN ('active','superseded')) THEN
    RAISE EXCEPTION 'active ITP items are immutable';
  END IF;
  RETURN COALESCE(NEW,OLD);
END;
$$;

DROP TRIGGER IF EXISTS trg_quality_itp_item_immutable ON quality_itp_items;
CREATE TRIGGER trg_quality_itp_item_immutable BEFORE UPDATE ON quality_itp_items FOR EACH ROW EXECUTE FUNCTION prevent_active_itp_item_mutation();

INSERT INTO permissions (id,key,name,description,module)
SELECT gen_random_uuid(),key,name,description,'quality'
FROM (VALUES
  ('quality.view','Xem QA/QC','Xem ITP, RFI, NCR và công việc QA/QC lặp'),
  ('quality.create','Lập hồ sơ QA/QC','Tạo/sửa ITP, RFI và NCR'),
  ('quality.review','Review ITP','Gửi và review revision ITP'),
  ('quality.approve','Duyệt/kích hoạt ITP','Duyệt và kích hoạt ITP revision'),
  ('quality.inspect','Kiểm tra RFI','Thực hiện inspection và quyết định RFI'),
  ('quality.ncr_manage','Quản lý NCR','Root cause, corrective action, reinspection và đóng NCR'),
  ('quality.recurring_manage','Quản lý QA/QC lặp','Tạo lịch, sinh occurrence và hoàn tất occurrence')
) p(key,name,description)
ON CONFLICT (key) DO NOTHING;

INSERT INTO role_permissions (role_id,permission_id)
SELECT r.id,p.id FROM roles r CROSS JOIN permissions p WHERE r.name='admin' AND p.module='quality'
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role_id,permission_id)
SELECT r.id,p.id FROM roles r JOIN permissions p ON p.module='quality'
WHERE
  (r.name='director' AND p.key IN ('quality.view','quality.approve')) OR
  (r.name='pm' AND p.key IN ('quality.view','quality.create','quality.review','quality.approve','quality.ncr_manage','quality.recurring_manage')) OR
  (r.name='qa_qc' AND p.key IN ('quality.view','quality.create','quality.review','quality.inspect','quality.ncr_manage','quality.recurring_manage')) OR
  (r.name='supervisor' AND p.key IN ('quality.view','quality.create','quality.recurring_manage')) OR
  (r.name='construction_team' AND p.key='quality.view') OR
  (r.name='client' AND p.key='quality.view')
ON CONFLICT DO NOTHING;
