CREATE TABLE IF NOT EXISTS notification_event_types (
  event_key TEXT PRIMARY KEY,
  domain_key TEXT NOT NULL,
  category_key TEXT NOT NULL,
  category_label TEXT NOT NULL,
  intent TEXT NOT NULL DEFAULT 'awareness',
  severity TEXT NOT NULL DEFAULT 'info',
  icon_key TEXT NOT NULL DEFAULT 'bell',
  tone_key TEXT NOT NULL DEFAULT 'neutral',
  audience_policy_key TEXT NOT NULL,
  visibility_policy_key TEXT NOT NULL DEFAULT 'project_permissions_any',
  permissions_any TEXT[] NOT NULL DEFAULT '{}',
  action_route_template TEXT NOT NULL,
  action_label TEXT NOT NULL DEFAULT 'Mở nội dung',
  required_delivery BOOLEAN NOT NULL DEFAULT false,
  enabled BOOLEAN NOT NULL DEFAULT false,
  catalog_version INTEGER NOT NULL DEFAULT 1,
  default_locale TEXT NOT NULL DEFAULT 'vi-VN',
  retention_days INTEGER NOT NULL DEFAULT 180,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT notification_event_types_key_nonblank CHECK (NULLIF(btrim(event_key),'') IS NOT NULL),
  CONSTRAINT notification_event_types_intent CHECK (intent IN ('action_required','awareness','outcome','security')),
  CONSTRAINT notification_event_types_severity CHECK (severity IN ('info','success','warning','critical')),
  CONSTRAINT notification_event_types_retention CHECK (retention_days BETWEEN 1 AND 3650)
);

CREATE TABLE IF NOT EXISTS notification_templates (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  event_key TEXT NOT NULL REFERENCES notification_event_types(event_key) ON DELETE CASCADE,
  locale TEXT NOT NULL DEFAULT 'vi-VN',
  version INTEGER NOT NULL,
  title_template TEXT NOT NULL,
  body_template TEXT NOT NULL,
  required_payload_keys TEXT[] NOT NULL DEFAULT '{}',
  active BOOLEAN NOT NULL DEFAULT true,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT notification_templates_title_nonblank CHECK (NULLIF(btrim(title_template),'') IS NOT NULL),
  CONSTRAINT notification_templates_body_nonblank CHECK (NULLIF(btrim(body_template),'') IS NOT NULL),
  UNIQUE (event_key,locale,version)
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_notification_templates_active_locale
  ON notification_templates(event_key,locale)
  WHERE active;

CREATE TABLE IF NOT EXISTS notification_events (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  event_key TEXT NOT NULL REFERENCES notification_event_types(event_key),
  aggregate_type TEXT NOT NULL,
  aggregate_id UUID NOT NULL,
  source_event_id UUID,
  project_id UUID REFERENCES projects(id) ON DELETE CASCADE,
  actor_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
  payload_json JSONB NOT NULL DEFAULT '{}',
  audience_json JSONB NOT NULL DEFAULT '{}',
  correlation_key TEXT,
  idempotency_key TEXT NOT NULL,
  occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  processed_at TIMESTAMPTZ,
  last_error TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT notification_events_aggregate_nonblank CHECK (NULLIF(btrim(aggregate_type),'') IS NOT NULL),
  CONSTRAINT notification_events_payload_object CHECK (jsonb_typeof(payload_json)='object'),
  CONSTRAINT notification_events_audience_object CHECK (jsonb_typeof(audience_json)='object'),
  CONSTRAINT notification_events_idempotency_nonblank CHECK (NULLIF(btrim(idempotency_key),'') IS NOT NULL),
  UNIQUE (idempotency_key)
);

CREATE INDEX IF NOT EXISTS idx_notification_events_project_occurred
  ON notification_events(project_id,occurred_at DESC,id DESC)
  WHERE project_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS notification_outbox (
  id BIGSERIAL PRIMARY KEY,
  event_id UUID NOT NULL UNIQUE REFERENCES notification_events(id) ON DELETE CASCADE,
  status TEXT NOT NULL DEFAULT 'pending',
  attempt_count INTEGER NOT NULL DEFAULT 0,
  next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  locked_at TIMESTAMPTZ,
  completed_at TIMESTAMPTZ,
  dead_at TIMESTAMPTZ,
  last_error TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT notification_outbox_status CHECK (status IN ('pending','processing','retry','completed','dead')),
  CONSTRAINT notification_outbox_attempts CHECK (attempt_count >= 0)
);

CREATE INDEX IF NOT EXISTS idx_notification_outbox_dispatch
  ON notification_outbox(next_attempt_at,id)
  WHERE status IN ('pending','retry');

ALTER TABLE user_notifications
  ADD COLUMN IF NOT EXISTS event_id UUID REFERENCES notification_events(id) ON DELETE SET NULL,
  ADD COLUMN IF NOT EXISTS event_key TEXT,
  ADD COLUMN IF NOT EXISTS category_key TEXT NOT NULL DEFAULT 'general',
  ADD COLUMN IF NOT EXISTS category_label TEXT NOT NULL DEFAULT 'Cập nhật',
  ADD COLUMN IF NOT EXISTS intent TEXT NOT NULL DEFAULT 'awareness',
  ADD COLUMN IF NOT EXISTS severity TEXT NOT NULL DEFAULT 'info',
  ADD COLUMN IF NOT EXISTS icon_key TEXT NOT NULL DEFAULT 'bell',
  ADD COLUMN IF NOT EXISTS tone_key TEXT NOT NULL DEFAULT 'neutral',
  ADD COLUMN IF NOT EXISTS action_href TEXT,
  ADD COLUMN IF NOT EXISTS action_label TEXT NOT NULL DEFAULT 'Mở nội dung',
  ADD COLUMN IF NOT EXISTS permissions_any_snapshot TEXT[] NOT NULL DEFAULT '{}',
  ADD COLUMN IF NOT EXISTS visibility_policy_key_snapshot TEXT NOT NULL DEFAULT 'project_permissions_any',
  ADD COLUMN IF NOT EXISTS seen_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS archived_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS resolved_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS expires_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS thread_key TEXT;

UPDATE user_notifications
SET event_key=CASE notification_type
      WHEN 'work_report_submitted' THEN 'work_report.submitted'
      WHEN 'work_report_resubmitted' THEN 'work_report.resubmitted'
      WHEN 'work_report_needs_changes' THEN 'work_report.needs_changes'
      WHEN 'work_report_acknowledged' THEN 'work_report.acknowledged'
      ELSE notification_type
    END,
    category_key='reports',
    category_label='Báo cáo',
    intent=CASE WHEN notification_type IN ('work_report_submitted','work_report_resubmitted','work_report_needs_changes') THEN 'action_required' ELSE 'outcome' END,
    severity=CASE WHEN notification_type='work_report_needs_changes' THEN 'warning' WHEN notification_type='work_report_acknowledged' THEN 'success' ELSE 'info' END,
    icon_key=CASE WHEN notification_type='work_report_needs_changes' THEN 'alert' WHEN notification_type='work_report_acknowledged' THEN 'check' ELSE 'report' END,
    tone_key=CASE WHEN notification_type='work_report_needs_changes' THEN 'warning' WHEN notification_type='work_report_acknowledged' THEN 'success' ELSE 'primary' END,
    action_href=CASE
      WHEN action_key='open_work_report_source' THEN '/projects/' || project_id::text || '/operations?tab=timeline&report=' || source_id::text
      WHEN action_key='open_work_report_review' THEN '/reports?scope=review&report=' || source_id::text
      ELSE '/reports?scope=mine&report=' || source_id::text
    END,
    permissions_any_snapshot=CASE
      WHEN notification_type IN ('work_report_submitted','work_report_resubmitted') THEN ARRAY['work_report.review','work_report.manage']::TEXT[]
      ELSE ARRAY['work_report.view','work_report.submit']::TEXT[]
    END
WHERE event_key IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS uq_user_notifications_recipient_event
  ON user_notifications(recipient_user_id,event_id)
  WHERE event_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_user_notifications_recipient_category_created
  ON user_notifications(recipient_user_id,category_key,created_at DESC,id DESC)
  WHERE archived_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_user_notifications_recipient_action_required
  ON user_notifications(recipient_user_id,created_at DESC,id DESC)
  WHERE archived_at IS NULL AND resolved_at IS NULL AND intent='action_required';

CREATE TABLE IF NOT EXISTS notification_preferences (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  category_key TEXT NOT NULL DEFAULT '*',
  event_key TEXT NOT NULL DEFAULT '*',
  in_app_enabled BOOLEAN NOT NULL DEFAULT true,
  email_enabled BOOLEAN NOT NULL DEFAULT false,
  push_enabled BOOLEAN NOT NULL DEFAULT false,
  digest_mode TEXT NOT NULL DEFAULT 'immediate',
  quiet_hours_json JSONB NOT NULL DEFAULT '{}',
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT notification_preferences_digest CHECK (digest_mode IN ('immediate','daily','weekly','off')),
  CONSTRAINT notification_preferences_quiet_object CHECK (jsonb_typeof(quiet_hours_json)='object'),
  UNIQUE (user_id,category_key,event_key)
);

CREATE TABLE IF NOT EXISTS notification_delivery_attempts (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  notification_id UUID NOT NULL REFERENCES user_notifications(id) ON DELETE CASCADE,
  channel TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'pending',
  attempt_count INTEGER NOT NULL DEFAULT 0,
  provider_message_id TEXT,
  next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  delivered_at TIMESTAMPTZ,
  last_error TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT notification_delivery_channel CHECK (channel IN ('in_app','email','push')),
  CONSTRAINT notification_delivery_status CHECK (status IN ('pending','retry','delivered','suppressed','dead')),
  UNIQUE (notification_id,channel)
);

CREATE TABLE IF NOT EXISTS notification_actor_revisions (
  user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  revision BIGINT NOT NULL DEFAULT 0,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO permissions (id,key,name,description,module)
VALUES (gen_random_uuid(),'notification.manage','Quản trị thông báo','Xem chẩn đoán và xử lý lỗi phát thông báo','notifications')
ON CONFLICT (key) DO UPDATE
SET name=EXCLUDED.name,description=EXCLUDED.description,module=EXCLUDED.module;

INSERT INTO role_permissions (role_id,permission_id)
SELECT role.id,permission.id
FROM roles role
JOIN permissions permission ON permission.key='notification.manage'
WHERE role.name IN ('admin','developer')
ON CONFLICT DO NOTHING;

INSERT INTO notification_event_types (
  event_key,domain_key,category_key,category_label,intent,severity,icon_key,tone_key,
  audience_policy_key,permissions_any,action_route_template,action_label,required_delivery,enabled
) VALUES
  ('work_report.submitted','reporting','reports','Báo cáo','action_required','info','report','primary','explicit_users',ARRAY['work_report.review','work_report.manage'],'/reports?scope=review&report={{source_id}}','Mở báo cáo',true,true),
  ('work_report.resubmitted','reporting','reports','Báo cáo','action_required','info','refresh','primary','explicit_users',ARRAY['work_report.review','work_report.manage'],'/reports?scope=review&report={{source_id}}','Mở báo cáo',true,true),
  ('work_report.needs_changes','reporting','reports','Báo cáo','action_required','warning','alert','warning','explicit_users',ARRAY['work_report.view','work_report.submit'],'/projects/{{project_id}}/operations?tab=timeline&report={{source_id}}','Mở báo cáo nguồn',true,true),
  ('work_report.acknowledged','reporting','reports','Báo cáo','outcome','success','check','success','explicit_users',ARRAY['work_report.view','work_report.submit'],'/reports?scope=mine&report={{source_id}}','Mở lịch sử',false,true)
ON CONFLICT (event_key) DO UPDATE SET
  domain_key=EXCLUDED.domain_key,category_key=EXCLUDED.category_key,category_label=EXCLUDED.category_label,
  intent=EXCLUDED.intent,severity=EXCLUDED.severity,icon_key=EXCLUDED.icon_key,tone_key=EXCLUDED.tone_key,
  audience_policy_key=EXCLUDED.audience_policy_key,permissions_any=EXCLUDED.permissions_any,
  action_route_template=EXCLUDED.action_route_template,action_label=EXCLUDED.action_label,
  required_delivery=EXCLUDED.required_delivery,enabled=EXCLUDED.enabled,updated_at=now();

INSERT INTO notification_event_types (event_key,domain_key,category_key,category_label,intent,severity,icon_key,tone_key,audience_policy_key,action_route_template)
SELECT event_key,domain_key,category_key,category_label,intent,severity,icon_key,tone_key,audience_policy_key,action_route_template
FROM (VALUES
  ('work_item.assigned','workflow','work','Công việc','action_required','info','task','primary','assignee','/my-work?task={{aggregate_id}}'),
  ('work_item.reassigned','workflow','work','Công việc','action_required','info','task','primary','assignee','/my-work?task={{aggregate_id}}'),
  ('work_item.review_requested','workflow','work','Công việc','action_required','warning','review','warning','approval_deciders','/my-work?task={{aggregate_id}}'),
  ('work_item.approval_requested','workflow','work','Công việc','action_required','warning','approval','warning','approval_deciders','/my-work?task={{aggregate_id}}'),
  ('work_item.changes_requested','workflow','work','Công việc','action_required','warning','alert','warning','source_owner','/my-work?task={{aggregate_id}}'),
  ('work_item.approved','workflow','work','Công việc','outcome','success','check','success','source_owner','/my-work?task={{aggregate_id}}'),
  ('work_item.rejected','workflow','work','Công việc','outcome','critical','x','critical','source_owner','/my-work?task={{aggregate_id}}'),
  ('work_item.blocked','workflow','work','Công việc','action_required','critical','block','critical','watchers','/my-work?task={{aggregate_id}}'),
  ('work_item.due_soon','workflow','work','Công việc','action_required','warning','clock','warning','assignee','/my-work?task={{aggregate_id}}'),
  ('work_item.overdue','workflow','work','Công việc','action_required','critical','clock','critical','assignee_and_project_pm','/my-work?task={{aggregate_id}}'),
  ('workflow.node_approval_requested','workflow','workflow','Quy trình','action_required','warning','approval','warning','approval_deciders','/my-work'),
  ('workflow.node_advanced','workflow','workflow','Quy trình','awareness','info','workflow','primary','next_step_responsible','/my-work'),
  ('workflow.automation_failed','workflow','workflow','Quy trình','action_required','critical','alert','critical','project_permission','/projects/{{project_id}}/planning'),
  ('workflow_form.submitted','workflow','workflow','Quy trình','awareness','info','form','primary','next_step_responsible','/my-work?task={{work_item_id}}'),
  ('workflow_form.amended','workflow','workflow','Quy trình','awareness','warning','form','warning','reviewers','/my-work?task={{work_item_id}}'),
  ('workflow_form.signature_required','workflow','workflow','Quy trình','action_required','warning','signature','warning','explicit_users','/my-work?task={{work_item_id}}'),
  ('activity.mentioned','planning','discussion','Trao đổi','action_required','info','mention','primary','mentioned_users','/projects/{{project_id}}/activity'),
  ('activity.replied','planning','discussion','Trao đổi','awareness','info','message','primary','parent_author','/projects/{{project_id}}/activity'),
  ('evidence.review_requested','construction','evidence','Minh chứng','action_required','warning','evidence','warning','reviewers','/my-work?task={{work_item_id}}'),
  ('evidence.accepted','construction','evidence','Minh chứng','outcome','success','check','success','source_owner','/my-work?task={{work_item_id}}'),
  ('evidence.rejected','construction','evidence','Minh chứng','action_required','warning','alert','warning','source_owner','/my-work?task={{work_item_id}}'),
  ('construction.handoff_activated','construction','construction','Thi công','awareness','info','construction','primary','project_roles','/projects/{{project_id}}/operations'),
  ('construction.check_assigned','construction','construction','Thi công','action_required','info','checklist','primary','assignee','/projects/{{project_id}}/operations'),
  ('construction.check_failed','construction','construction','Thi công','action_required','critical','alert','critical','assignee_and_project_pm','/projects/{{project_id}}/operations'),
  ('construction.action_assigned','construction','construction','Thi công','action_required','warning','task','warning','assignee','/projects/{{project_id}}/operations'),
  ('construction.action_resolution_submitted','construction','construction','Thi công','action_required','info','review','primary','reviewers','/projects/{{project_id}}/operations'),
  ('construction.action_verified','construction','construction','Thi công','outcome','success','check','success','source_owner','/projects/{{project_id}}/operations'),
  ('construction.action_rejected','construction','construction','Thi công','action_required','warning','alert','warning','source_owner','/projects/{{project_id}}/operations'),
  ('construction.issue_escalated','construction','construction','Thi công','action_required','critical','alert','critical','project_pm','/projects/{{project_id}}/operations'),
  ('boq.submitted','commercial','commercial','Thương mại','action_required','warning','commercial','warning','approval_deciders','/projects/{{project_id}}/boq'),
  ('boq.approved','commercial','commercial','Thương mại','outcome','success','check','success','source_owner','/projects/{{project_id}}/boq'),
  ('boq.rejected','commercial','commercial','Thương mại','action_required','warning','alert','warning','source_owner','/projects/{{project_id}}/boq'),
  ('supplier_quote.submitted','commercial','commercial','Thương mại','action_required','info','commercial','primary','reviewers','/projects/{{project_id}}/procurement'),
  ('supplier_quote.selected','commercial','commercial','Thương mại','outcome','success','check','success','source_owner','/projects/{{project_id}}/procurement'),
  ('procurement.submitted','commercial','procurement','Mua hàng','action_required','warning','procurement','warning','approval_deciders','/projects/{{project_id}}/procurement'),
  ('procurement.approved','commercial','procurement','Mua hàng','outcome','success','check','success','source_owner','/projects/{{project_id}}/procurement'),
  ('procurement.rejected','commercial','procurement','Mua hàng','action_required','warning','alert','warning','source_owner','/projects/{{project_id}}/procurement'),
  ('procurement.ready_to_receive','commercial','procurement','Mua hàng','action_required','info','warehouse','primary','warehouse_roles','/projects/{{project_id}}/warehouse'),
  ('contract.approval_requested','commercial','contracts','Hợp đồng','action_required','warning','approval','warning','approval_deciders','/projects/{{project_id}}/contracts'),
  ('contract.signature_required','commercial','contracts','Hợp đồng','action_required','warning','signature','warning','explicit_users','/projects/{{project_id}}/contracts'),
  ('contract.milestone_due','commercial','contracts','Hợp đồng','action_required','warning','clock','warning','project_roles','/projects/{{project_id}}/contracts'),
  ('contract.variation_stage_changed','commercial','contracts','Hợp đồng','action_required','warning','commercial','warning','next_step_responsible','/projects/{{project_id}}/contracts'),
  ('stock.document_completed','materials','warehouse','Kho & vật tư','awareness','info','warehouse','primary','requester_and_custodian','/projects/{{project_id}}/warehouse'),
  ('material_snapshot.submitted','materials','warehouse','Kho & vật tư','action_required','warning','warehouse','warning','reviewers','/projects/{{project_id}}/operations'),
  ('surplus.assigned','materials','warehouse','Kho & vật tư','action_required','warning','warehouse','warning','assignee','/projects/{{project_id}}/operations'),
  ('incoming_acceptance.submitted','quality','quality','Chất lượng','action_required','warning','quality','warning','reviewers','/projects/{{project_id}}/acceptance'),
  ('quality.itp_submitted','quality','quality','Chất lượng','action_required','warning','quality','warning','approval_deciders','/projects/{{project_id}}/quality'),
  ('quality.rfi_scheduled','quality','quality','Chất lượng','action_required','info','calendar','primary','explicit_users','/projects/{{project_id}}/quality'),
  ('quality.ncr_created','quality','quality','Chất lượng','action_required','critical','alert','critical','assignee_and_project_pm','/projects/{{project_id}}/quality'),
  ('quality.ncr_corrective_submitted','quality','quality','Chất lượng','action_required','warning','review','warning','reviewers','/projects/{{project_id}}/quality'),
  ('quality.recurring_due','quality','quality','Chất lượng','action_required','warning','clock','warning','assignee','/projects/{{project_id}}/quality'),
  ('acceptance.failed','quality','quality','Chất lượng','action_required','critical','alert','critical','source_owner_and_project_pm','/projects/{{project_id}}/acceptance'),
  ('rework.assigned','quality','quality','Chất lượng','action_required','warning','task','warning','assignee','/projects/{{project_id}}/acceptance'),
  ('handover.ready','quality','quality','Chất lượng','action_required','info','handover','primary','project_roles','/projects/{{project_id}}/handover'),
  ('handover.blocked','quality','quality','Chất lượng','action_required','critical','block','critical','project_pm','/projects/{{project_id}}/handover'),
  ('budget.approval_requested','finance','finance','Tài chính','action_required','warning','finance','warning','approval_deciders','/projects/{{project_id}}/accounting'),
  ('payable.due','finance','finance','Tài chính','action_required','warning','clock','warning','finance_roles','/projects/{{project_id}}/accounting'),
  ('receivable.due','finance','finance','Tài chính','action_required','warning','clock','warning','finance_roles','/projects/{{project_id}}/accounting'),
  ('invoice.rejected','finance','finance','Tài chính','action_required','warning','alert','warning','source_owner','/projects/{{project_id}}/accounting'),
  ('payment.reversed','finance','finance','Tài chính','action_required','critical','alert','critical','finance_roles','/projects/{{project_id}}/accounting'),
  ('cash_request.submitted','finance','finance','Tài chính','action_required','warning','finance','warning','finance_roles','/projects/{{project_id}}/accounting'),
  ('tax_period.action_required','finance','finance','Tài chính','action_required','critical','calendar','critical','finance_roles','/accounting'),
  ('project.member_added','admin','governance','Quản trị','awareness','info','users','primary','explicit_users','/projects/{{project_id}}'),
  ('project.member_removed','admin','governance','Quản trị','security','warning','users','warning','explicit_users','/projects'),
  ('project.role_changed','admin','governance','Quản trị','security','warning','shield','warning','explicit_users','/projects/{{project_id}}'),
  ('project.pm_changed','admin','governance','Quản trị','action_required','warning','users','warning','explicit_users','/projects/{{project_id}}'),
  ('account.role_granted','admin','security','Bảo mật','security','info','shield','primary','explicit_users','/profile'),
  ('account.role_revoked','admin','security','Bảo mật','security','warning','shield','warning','explicit_users','/profile'),
  ('account.deactivated','admin','security','Bảo mật','security','critical','shield','critical','explicit_users','/profile'),
  ('account.password_changed','admin','security','Bảo mật','security','info','shield','primary','explicit_users','/profile')
) AS catalog(event_key,domain_key,category_key,category_label,intent,severity,icon_key,tone_key,audience_policy_key,action_route_template)
ON CONFLICT (event_key) DO NOTHING;

INSERT INTO notification_templates (event_key,locale,version,title_template,body_template,required_payload_keys,active)
VALUES
  ('work_report.submitted','vi-VN',1,'Báo cáo chờ anh/chị xác nhận','{{actor_name}} đã gửi báo cáo thi công ngày {{report_date}} tại {{project_name}}.',ARRAY['actor_name','report_date','project_name','project_id','source_id'],true),
  ('work_report.resubmitted','vi-VN',1,'Báo cáo đã được gửi lại','{{actor_name}} đã bổ sung và gửi lại báo cáo thi công ngày {{report_date}} tại {{project_name}}.',ARRAY['actor_name','report_date','project_name','project_id','source_id'],true),
  ('work_report.needs_changes','vi-VN',1,'Báo cáo cần bổ sung','{{actor_name}} yêu cầu bổ sung báo cáo của anh/chị.',ARRAY['actor_name','project_id','source_id'],true),
  ('work_report.acknowledged','vi-VN',1,'Báo cáo đã được xác nhận','{{actor_name}} đã xác nhận báo cáo của anh/chị.',ARRAY['actor_name','project_id','source_id'],true)
ON CONFLICT (event_key,locale,version) DO UPDATE SET
  title_template=EXCLUDED.title_template,body_template=EXCLUDED.body_template,
  required_payload_keys=EXCLUDED.required_payload_keys,active=EXCLUDED.active;
