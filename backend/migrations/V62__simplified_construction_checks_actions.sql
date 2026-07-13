CREATE TABLE IF NOT EXISTS construction_check_templates (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  description TEXT,
  criteria_jsonb JSONB NOT NULL DEFAULT '[]'::jsonb,
  status TEXT NOT NULL DEFAULT 'active',
  technical_itp_id UUID REFERENCES quality_itp_plans(id),
  created_by UUID REFERENCES users(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT construction_check_template_status_check CHECK (status IN ('active','archived')),
  CONSTRAINT construction_check_template_criteria_check CHECK (jsonb_typeof(criteria_jsonb)='array')
);

CREATE INDEX IF NOT EXISTS idx_construction_check_templates_project
  ON construction_check_templates(project_id,status,updated_at DESC);
CREATE UNIQUE INDEX IF NOT EXISTS uq_construction_check_template_technical_itp
  ON construction_check_templates(technical_itp_id) WHERE technical_itp_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS construction_checks (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  work_item_id UUID,
  template_id UUID REFERENCES construction_check_templates(id),
  title TEXT NOT NULL,
  location TEXT,
  criteria_jsonb JSONB NOT NULL DEFAULT '[]'::jsonb,
  scheduled_at TIMESTAMPTZ,
  status TEXT NOT NULL DEFAULT 'pending',
  result TEXT,
  note TEXT,
  evidence_id UUID REFERENCES evidence_records(id),
  requested_by UUID REFERENCES users(id),
  checked_by UUID REFERENCES users(id),
  checked_at TIMESTAMPTZ,
  technical_rfi_id UUID REFERENCES quality_inspection_requests(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT construction_check_work_item_project_fk FOREIGN KEY (work_item_id,project_id) REFERENCES work_items(id,project_id) ON DELETE SET NULL,
  CONSTRAINT construction_check_status_check CHECK (status IN ('pending','completed','cancelled')),
  CONSTRAINT construction_check_result_check CHECK (result IS NULL OR result IN ('pass','fail')),
  CONSTRAINT construction_check_result_consistency CHECK ((status='completed' AND result IS NOT NULL AND checked_at IS NOT NULL) OR status <> 'completed'),
  CONSTRAINT construction_check_criteria_check CHECK (jsonb_typeof(criteria_jsonb)='array')
);

CREATE INDEX IF NOT EXISTS idx_construction_checks_project_queue
  ON construction_checks(project_id,status,scheduled_at,created_at DESC);
CREATE UNIQUE INDEX IF NOT EXISTS uq_construction_check_technical_rfi
  ON construction_checks(technical_rfi_id) WHERE technical_rfi_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS construction_action_items (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  work_item_id UUID,
  source_check_id UUID REFERENCES construction_checks(id) ON DELETE SET NULL,
  title TEXT NOT NULL,
  description TEXT,
  priority TEXT NOT NULL DEFAULT 'normal',
  assigned_user_id UUID REFERENCES users(id),
  due_at TIMESTAMPTZ,
  status TEXT NOT NULL DEFAULT 'open',
  impact_scope TEXT NOT NULL DEFAULT 'none',
  before_evidence_id UUID REFERENCES evidence_records(id),
  resolution_evidence_id UUID REFERENCES evidence_records(id),
  resolution_note TEXT,
  submitted_by UUID REFERENCES users(id),
  submitted_at TIMESTAMPTZ,
  verified_by UUID REFERENCES users(id),
  verified_at TIMESTAMPTZ,
  retry_count INTEGER NOT NULL DEFAULT 0,
  technical_issue_id UUID REFERENCES issues(id),
  technical_ncr_id UUID REFERENCES quality_ncrs(id),
  technical_rework_id UUID REFERENCES rework_requests(id),
  technical_replanning_id UUID REFERENCES replanning_requests(id),
  created_by UUID REFERENCES users(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT construction_action_work_item_project_fk FOREIGN KEY (work_item_id,project_id) REFERENCES work_items(id,project_id) ON DELETE SET NULL,
  CONSTRAINT construction_action_priority_check CHECK (priority IN ('normal','urgent')),
  CONSTRAINT construction_action_status_check CHECK (status IN ('open','waiting_confirmation','done','cancelled')),
  CONSTRAINT construction_action_impact_check CHECK (impact_scope IN ('none','timeline','budget','design_scope')),
  CONSTRAINT construction_action_retry_check CHECK (retry_count >= 0)
);

CREATE INDEX IF NOT EXISTS idx_construction_actions_project_queue
  ON construction_action_items(project_id,status,due_at,created_at DESC);
CREATE UNIQUE INDEX IF NOT EXISTS uq_construction_action_source_check
  ON construction_action_items(source_check_id) WHERE source_check_id IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uq_construction_action_technical_issue
  ON construction_action_items(technical_issue_id) WHERE technical_issue_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS construction_action_events (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  action_id UUID NOT NULL REFERENCES construction_action_items(id) ON DELETE CASCADE,
  event_type TEXT NOT NULL,
  comment TEXT,
  evidence_id UUID REFERENCES evidence_records(id),
  actor_id UUID REFERENCES users(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT construction_action_event_type_check CHECK (event_type IN ('created','resolution_submitted','verified_pass','verified_fail','impact_approved'))
);

CREATE INDEX IF NOT EXISTS idx_construction_action_events_action
  ON construction_action_events(action_id,created_at,id);

INSERT INTO permissions (id,key,name,description,module)
SELECT gen_random_uuid(),key,name,description,'construction'
FROM (VALUES
  ('construction.template.manage','Quản lý mẫu kiểm tra','Tạo và lưu mẫu kiểm tra đơn giản cho công trình'),
  ('construction.check.request','Tạo lịch kiểm tra','Tạo một lần kiểm tra từ mẫu hoặc nội dung ngắn'),
  ('construction.check.perform','Ghi nhận kết quả kiểm tra','Kết luận đạt hoặc không đạt kèm minh chứng'),
  ('construction.action.create','Tạo việc cần xử lý','Ghi nhận một việc cần xử lý từ hiện trường'),
  ('construction.action.resolve','Gửi kết quả khắc phục','Gửi ảnh và ghi chú khắc phục để chờ xác nhận'),
  ('construction.action.verify','Xác nhận kết quả khắc phục','Xác nhận đạt hoặc trả lại việc cần xử lý')
) p(key,name,description)
ON CONFLICT (key) DO UPDATE SET name=EXCLUDED.name,description=EXCLUDED.description,module=EXCLUDED.module;

INSERT INTO role_permissions (role_id,permission_id)
SELECT r.id,p.id FROM roles r CROSS JOIN permissions p
WHERE r.name='admin' AND p.module='construction'
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role_id,permission_id)
SELECT r.id,p.id FROM roles r JOIN permissions p ON p.module='construction'
WHERE
  (r.name IN ('pm','qa_qc') AND p.key IN (
    'construction.template.manage','construction.check.request','construction.check.perform',
    'construction.action.create','construction.action.resolve','construction.action.verify'
  )) OR
  (r.name='director' AND p.key='construction.action.verify') OR
  (r.name='supervisor' AND p.key IN (
    'construction.check.request','construction.check.perform','construction.action.create','construction.action.resolve'
  )) OR
  (r.name='construction_team' AND p.key IN (
    'construction.check.request','construction.action.create','construction.action.resolve'
  ))
ON CONFLICT DO NOTHING;

-- Preserve existing technical quality history behind the simplified read model.
INSERT INTO construction_check_templates (
  id,project_id,name,description,criteria_jsonb,status,technical_itp_id,created_by,created_at,updated_at
)
SELECT p.id,p.project_id,p.title,
       concat_ws(' · ',NULLIF(p.discipline,''),NULLIF(p.work_package,'')),
       COALESCE((
         SELECT jsonb_agg(jsonb_build_object(
           'title',i.activity_name,
           'criteria',i.acceptance_criteria,
           'evidence_required',i.evidence_required
         ) ORDER BY i.sequence_no)
         FROM quality_itp_items i WHERE i.itp_plan_id=p.id
       ),'[]'::jsonb),
       CASE WHEN p.status='active' THEN 'active' ELSE 'archived' END,
       p.id,p.created_by,p.created_at,p.updated_at
FROM quality_itp_plans p
WHERE p.status NOT IN ('draft','rejected')
ON CONFLICT (id) DO NOTHING;

INSERT INTO construction_checks (
  id,project_id,work_item_id,template_id,title,location,criteria_jsonb,scheduled_at,status,result,note,
  evidence_id,requested_by,checked_by,checked_at,technical_rfi_id,created_at,updated_at
)
SELECT r.id,r.project_id,r.work_item_id,r.itp_plan_id,i.activity_name,r.location,
       jsonb_build_array(jsonb_build_object('title',i.activity_name,'criteria',i.acceptance_criteria,'evidence_required',i.evidence_required)),
       r.requested_inspection_at,
       CASE WHEN r.status='cancelled' THEN 'cancelled' WHEN r.result IS NOT NULL THEN 'completed' ELSE 'pending' END,
       CASE WHEN r.result IN ('pass','conditional_pass') THEN 'pass' WHEN r.result='fail' THEN 'fail' ELSE NULL END,
       COALESCE(NULLIF(r.inspection_comment,''),r.request_comment),
       COALESCE(r.inspection_evidence_id,r.request_evidence_id),r.requested_by,r.inspector_id,r.inspected_at,r.id,r.created_at,r.updated_at
FROM quality_inspection_requests r
JOIN quality_itp_items i ON i.id=r.itp_item_id
ON CONFLICT (id) DO NOTHING;

INSERT INTO construction_action_items (
  id,project_id,work_item_id,source_check_id,title,description,priority,assigned_user_id,due_at,status,
  impact_scope,before_evidence_id,resolution_evidence_id,resolution_note,submitted_by,submitted_at,
  verified_by,verified_at,technical_issue_id,technical_ncr_id,technical_rework_id,technical_replanning_id,
  created_by,created_at,updated_at
)
SELECT i.id,i.project_id,n.work_item_id,c.id,i.title,i.description,
       CASE WHEN i.severity IN ('high','critical') THEN 'urgent' ELSE 'normal' END,
       COALESCE(n.owner_id,s.user_id),COALESCE(n.due_date::timestamptz,rw.due_date::timestamptz),
       CASE WHEN i.status='closed' THEN 'done' ELSE 'open' END,
       CASE
         WHEN NOT i.requires_replanning THEN 'none'
         WHEN rp.impact_scope IN ('timeline','budget','design_scope') THEN rp.impact_scope
         ELSE 'timeline'
       END,
       COALESCE(n.evidence_id,c.evidence_id),n.corrective_evidence_id,n.closure_comment,
       n.corrective_submitted_by,n.updated_at,n.closed_by,n.closed_at,
       i.id,n.id,rw.id,rp.id,i.reported_by,i.created_at,i.updated_at
FROM issues i
LEFT JOIN quality_ncrs n ON n.issue_id=i.id
LEFT JOIN construction_checks c ON c.technical_rfi_id=n.rfi_id
LEFT JOIN rework_requests rw ON rw.id=COALESCE(n.rework_request_id,(
  SELECT r2.id FROM rework_requests r2 WHERE r2.issue_id=i.id ORDER BY r2.created_at LIMIT 1
))
LEFT JOIN replanning_requests rp ON rp.id=(
  SELECT rp2.id FROM replanning_requests rp2 WHERE rp2.issue_id=i.id ORDER BY rp2.created_at LIMIT 1
)
LEFT JOIN staff s ON s.id=i.assigned_to
ON CONFLICT (id) DO NOTHING;
