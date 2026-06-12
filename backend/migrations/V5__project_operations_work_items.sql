CREATE UNIQUE INDEX IF NOT EXISTS ux_pipeline_runs_id_project
  ON pipeline_runs(id, project_id);

CREATE UNIQUE INDEX IF NOT EXISTS ux_pipeline_run_nodes_id_project
  ON pipeline_run_nodes(id, project_id);

CREATE UNIQUE INDEX IF NOT EXISTS ux_pipeline_run_nodes_id_run_project
  ON pipeline_run_nodes(id, run_id, project_id);

CREATE UNIQUE INDEX IF NOT EXISTS ux_pipeline_run_steps_id_project
  ON pipeline_run_steps(id, project_id);

CREATE UNIQUE INDEX IF NOT EXISTS ux_pipeline_run_steps_id_node_project
  ON pipeline_run_steps(id, run_node_id, project_id);

CREATE UNIQUE INDEX IF NOT EXISTS ux_approval_requests_id_project
  ON approval_requests(id, project_id);

CREATE UNIQUE INDEX IF NOT EXISTS ux_documents_id_project
  ON documents(id, project_id);

CREATE TABLE IF NOT EXISTS work_items (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  pipeline_run_id UUID,
  run_node_id UUID,
  run_step_id UUID,
  source_type TEXT NOT NULL DEFAULT 'pipeline_run_step',
  source_id UUID NOT NULL,
  title TEXT NOT NULL,
  description TEXT,
  work_type TEXT NOT NULL DEFAULT 'task',
  status TEXT NOT NULL DEFAULT 'pending',
  priority TEXT NOT NULL DEFAULT 'normal',
  department TEXT,
  role_ids JSONB NOT NULL DEFAULT '[]',
  assignee_staff_id UUID REFERENCES staff(id),
  watcher_staff_ids JSONB NOT NULL DEFAULT '[]',
  planned_start_at TIMESTAMPTZ,
  due_at TIMESTAMPTZ,
  started_at TIMESTAMPTZ,
  completed_at TIMESTAMPTZ,
  blocked_at TIMESTAMPTZ,
  blocked_reason TEXT,
  completion_payload_json JSONB NOT NULL DEFAULT '{}',
  evidence_requirement_json JSONB NOT NULL DEFAULT '{}',
  approval_request_id UUID,
  created_by UUID REFERENCES users(id),
  updated_by UUID REFERENCES users(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT work_items_id_project_unique UNIQUE (id, project_id),
  CONSTRAINT work_items_pipeline_run_project_fk
    FOREIGN KEY (pipeline_run_id, project_id) REFERENCES pipeline_runs(id, project_id) ON DELETE CASCADE,
  CONSTRAINT work_items_run_node_project_fk
    FOREIGN KEY (run_node_id, project_id) REFERENCES pipeline_run_nodes(id, project_id) ON DELETE CASCADE,
  CONSTRAINT work_items_run_step_project_fk
    FOREIGN KEY (run_step_id, project_id) REFERENCES pipeline_run_steps(id, project_id) ON DELETE CASCADE,
  CONSTRAINT work_items_run_node_hierarchy_fk
    FOREIGN KEY (run_node_id, pipeline_run_id, project_id) REFERENCES pipeline_run_nodes(id, run_id, project_id) ON DELETE CASCADE,
  CONSTRAINT work_items_run_step_hierarchy_fk
    FOREIGN KEY (run_step_id, run_node_id, project_id) REFERENCES pipeline_run_steps(id, run_node_id, project_id) ON DELETE CASCADE,
  CONSTRAINT work_items_approval_request_project_fk
    FOREIGN KEY (approval_request_id, project_id) REFERENCES approval_requests(id, project_id),
  CONSTRAINT work_items_source_type_check CHECK (source_type IN (
    'pipeline_run_step',
    'boq_item',
    'procurement_request',
    'stock_movement',
    'daily_report',
    'issue',
    'acceptance',
    'rework_request',
    'replanning_request',
    'handover',
    'accounting_budget',
    'payable',
    'receivable',
    'payment'
  )),
  CONSTRAINT work_items_work_type_check CHECK (work_type IN (
    'task',
    'checklist',
    'approval',
    'document_upload',
    'photo_upload',
    'design_review',
    'budget_check',
    'material_check',
    'daily_report',
    'acceptance_check',
    'rework_task',
    'handover_check',
    'accounting_check'
  )),
  CONSTRAINT work_items_status_check CHECK (status IN (
    'pending',
    'ready',
    'in_progress',
    'waiting_evidence',
    'waiting_review',
    'waiting_approval',
    'blocked',
    'needs_changes',
    'completed',
    'cancelled'
  )),
  CONSTRAINT work_items_priority_check CHECK (priority IN ('low', 'normal', 'high', 'urgent'))
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_work_items_run_step
  ON work_items(run_step_id)
  WHERE run_step_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_work_items_project_status
  ON work_items(project_id, status, due_at);

CREATE INDEX IF NOT EXISTS idx_work_items_pipeline_run
  ON work_items(pipeline_run_id, run_node_id, run_step_id);

CREATE INDEX IF NOT EXISTS idx_work_items_assignee
  ON work_items(assignee_staff_id, status)
  WHERE assignee_staff_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_work_items_role_ids
  ON work_items USING GIN(role_ids);

CREATE INDEX IF NOT EXISTS idx_work_items_watcher_staff_ids
  ON work_items USING GIN(watcher_staff_ids);

CREATE TABLE IF NOT EXISTS work_item_assignments (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  work_item_id UUID NOT NULL REFERENCES work_items(id) ON DELETE CASCADE,
  assignment_type TEXT NOT NULL,
  staff_id UUID REFERENCES staff(id),
  department TEXT,
  role_id UUID REFERENCES roles(id),
  role_name TEXT,
  created_by UUID REFERENCES users(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT work_item_assignments_type_check CHECK (assignment_type IN ('assignee', 'watcher', 'department', 'role')),
  CONSTRAINT work_item_assignments_target_check CHECK (
    staff_id IS NOT NULL OR
    NULLIF(btrim(COALESCE(department, '')), '') IS NOT NULL OR
    role_id IS NOT NULL OR
    NULLIF(btrim(COALESCE(role_name, '')), '') IS NOT NULL
  )
);

CREATE INDEX IF NOT EXISTS idx_work_item_assignments_work_item
  ON work_item_assignments(work_item_id);

CREATE INDEX IF NOT EXISTS idx_work_item_assignments_staff
  ON work_item_assignments(staff_id, assignment_type)
  WHERE staff_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_work_item_assignments_department
  ON work_item_assignments(department, assignment_type)
  WHERE department IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_work_item_assignments_role_name
  ON work_item_assignments(role_name, assignment_type)
  WHERE role_name IS NOT NULL;

CREATE TABLE IF NOT EXISTS work_item_activities (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  work_item_id UUID NOT NULL,
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  scope_type TEXT NOT NULL DEFAULT 'work_item',
  scope_id UUID,
  actor_id UUID REFERENCES users(id),
  activity_type TEXT NOT NULL,
  comment TEXT,
  from_status TEXT,
  to_status TEXT,
  metadata_json JSONB NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT work_item_activities_work_item_project_fk
    FOREIGN KEY (work_item_id, project_id) REFERENCES work_items(id, project_id) ON DELETE CASCADE,
  CONSTRAINT work_item_activities_scope_type_check CHECK (scope_type IN (
    'project',
    'pipeline_run_node',
    'pipeline_run_step',
    'work_item',
    'boq_item',
    'procurement_request',
    'stock_movement',
    'daily_report',
    'issue',
    'acceptance',
    'rework',
    'rework_request',
    'replanning',
    'replanning_request',
    'handover',
    'accounting_budget',
    'payable',
    'receivable',
    'payment'
  )),
  CONSTRAINT work_item_activities_type_check CHECK (activity_type IN (
    'created',
    'started',
    'completed',
    'blocked',
    'blocker_resolved',
    'status_changed',
    'comment',
    'evidence_added'
  )),
  CONSTRAINT work_item_activities_from_status_check CHECK (from_status IS NULL OR from_status IN (
    'pending',
    'ready',
    'in_progress',
    'waiting_evidence',
    'waiting_review',
    'waiting_approval',
    'blocked',
    'needs_changes',
    'completed',
    'cancelled'
  )),
  CONSTRAINT work_item_activities_to_status_check CHECK (to_status IS NULL OR to_status IN (
    'pending',
    'ready',
    'in_progress',
    'waiting_evidence',
    'waiting_review',
    'waiting_approval',
    'blocked',
    'needs_changes',
    'completed',
    'cancelled'
  ))
);

CREATE INDEX IF NOT EXISTS idx_work_item_activities_work_item_created
  ON work_item_activities(work_item_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_work_item_activities_project_created
  ON work_item_activities(project_id, created_at DESC);

CREATE TABLE IF NOT EXISTS evidence_records (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  work_item_id UUID NOT NULL,
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  target_type TEXT NOT NULL DEFAULT 'work_item',
  target_id UUID,
  evidence_type TEXT NOT NULL,
  document_type TEXT,
  name TEXT NOT NULL,
  document_id UUID,
  title TEXT,
  description TEXT,
  file_url TEXT,
  mime_type TEXT,
  file_size BIGINT,
  caption TEXT,
  revision_no TEXT,
  visibility TEXT NOT NULL DEFAULT 'internal',
  payload_json JSONB NOT NULL DEFAULT '{}',
  status TEXT NOT NULL DEFAULT 'submitted',
  uploaded_by UUID REFERENCES users(id),
  submitted_by UUID REFERENCES users(id),
  reviewed_by UUID REFERENCES users(id),
  reviewed_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT evidence_records_work_item_project_fk
    FOREIGN KEY (work_item_id, project_id) REFERENCES work_items(id, project_id) ON DELETE CASCADE,
  CONSTRAINT evidence_records_document_project_fk
    FOREIGN KEY (document_id, project_id) REFERENCES documents(id, project_id),
  CONSTRAINT evidence_records_target_type_check CHECK (target_type IN (
    'project',
    'pipeline_run_node',
    'pipeline_run_step',
    'work_item',
    'boq_item',
    'procurement_request',
    'stock_movement',
    'daily_report',
    'issue',
    'acceptance',
    'rework',
    'rework_request',
    'replanning',
    'replanning_request',
    'handover',
    'accounting_budget',
    'payable',
    'receivable',
    'payment'
  )),
  CONSTRAINT evidence_records_type_check CHECK (evidence_type IN ('document', 'photo', 'checklist', 'link', 'note', 'other')),
  CONSTRAINT evidence_records_visibility_check CHECK (visibility IN ('internal', 'project', 'client')),
  CONSTRAINT evidence_records_file_size_check CHECK (file_size IS NULL OR file_size >= 0),
  CONSTRAINT evidence_records_status_check CHECK (status IN ('draft', 'submitted', 'accepted', 'rejected'))
);

CREATE INDEX IF NOT EXISTS idx_evidence_records_work_item
  ON evidence_records(work_item_id, status, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_evidence_records_project
  ON evidence_records(project_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_evidence_records_target
  ON evidence_records(target_type, target_id)
  WHERE target_id IS NOT NULL;

DO $$
BEGIN
  IF EXISTS (
    SELECT 1
    FROM pipeline_run_steps rs
    JOIN pipeline_run_nodes rn ON rn.id=rs.run_node_id
    JOIN pipeline_runs pr ON pr.id=rn.run_id
    WHERE rs.project_id <> rn.project_id
       OR rs.project_id <> pr.project_id
  ) THEN
    RAISE EXCEPTION 'Cannot backfill work_items: pipeline_run_steps, pipeline_run_nodes, and pipeline_runs project_id values are inconsistent';
  END IF;
END $$;

INSERT INTO work_items (
  project_id,
  pipeline_run_id,
  run_node_id,
  run_step_id,
  source_type,
  source_id,
  title,
  description,
  work_type,
  status,
  priority,
  role_ids,
  evidence_requirement_json,
  created_by,
  updated_by,
  created_at,
  updated_at,
  started_at,
  completed_at
)
SELECT
  rs.project_id,
  rn.run_id,
  rs.run_node_id,
  rs.id,
  'pipeline_run_step',
  rs.id,
  rs.name,
  rs.description,
  CASE
    WHEN rs.step_type IN (
      'task',
      'checklist',
      'approval',
      'document_upload',
      'photo_upload',
      'design_review',
      'budget_check',
      'material_check',
      'daily_report',
      'acceptance_check',
      'rework_task',
      'handover_check',
      'accounting_check'
    ) THEN rs.step_type
    ELSE 'task'
  END,
  CASE
    WHEN rs.status='completed' THEN 'completed'
    WHEN rn.status='in_progress' THEN 'ready'
    ELSE 'pending'
  END,
  'normal',
  CASE
    WHEN jsonb_array_length(COALESCE(rs.required_role_ids, '[]'::jsonb)) > 0 THEN rs.required_role_ids
    ELSE rn.responsible_role_ids
  END,
  jsonb_build_object(
    'checklist_required', rs.checklist_required,
    'document_required', rs.document_required,
    'photo_required', rs.photo_required,
    'required_permission', rs.required_permission
  ),
  rs.submitted_by,
  rs.submitted_by,
  COALESCE(rs.submitted_at, now()),
  COALESCE(rs.submitted_at, now()),
  CASE WHEN rs.status='completed' THEN rs.submitted_at ELSE NULL END,
  CASE WHEN rs.status='completed' THEN rs.submitted_at ELSE NULL END
FROM pipeline_run_steps rs
JOIN pipeline_run_nodes rn ON rn.id=rs.run_node_id AND rn.project_id=rs.project_id
JOIN pipeline_runs pr ON pr.id=rn.run_id AND pr.project_id=rs.project_id
WHERE NOT EXISTS (
  SELECT 1
  FROM work_items wi
  WHERE wi.run_step_id=rs.id
);

INSERT INTO work_item_assignments (work_item_id, assignment_type, role_name, created_by)
SELECT wi.id, 'role', role_label.value, wi.created_by
FROM work_items wi
CROSS JOIN LATERAL jsonb_array_elements_text(wi.role_ids) AS role_label(value)
WHERE wi.source_type='pipeline_run_step'
  AND NOT EXISTS (
    SELECT 1
    FROM work_item_assignments wia
    WHERE wia.work_item_id=wi.id
      AND wia.assignment_type='role'
      AND wia.role_name=role_label.value
  );

INSERT INTO work_item_activities (
  work_item_id,
  project_id,
  scope_type,
  scope_id,
  actor_id,
  activity_type,
  to_status,
  comment,
  metadata_json
)
SELECT
  wi.id,
  wi.project_id,
  'work_item',
  wi.id,
  wi.created_by,
  'created',
  wi.status,
  'Work item backfilled from pipeline run step',
  jsonb_build_object('source_type', wi.source_type, 'source_id', wi.source_id)
FROM work_items wi
WHERE wi.source_type='pipeline_run_step'
  AND NOT EXISTS (
    SELECT 1
    FROM work_item_activities wia
    WHERE wia.work_item_id=wi.id
      AND wia.activity_type='created'
  );
