ALTER TABLE projects
  ADD COLUMN IF NOT EXISTS created_by UUID REFERENCES users(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_projects_startup_owner
  ON projects(created_by,created_at DESC,id DESC)
  WHERE created_by IS NOT NULL AND NOT is_sample AND status <> 'cancelled';

INSERT INTO notification_event_types (
  event_key,domain_key,category_key,category_label,intent,severity,icon_key,tone_key,
  audience_policy_key,visibility_policy_key,permissions_any,action_route_template,
  action_label,required_delivery,enabled
) VALUES
  ('project.setup_required','planning','project_startup','Khởi tạo công trình','action_required','warning','workflow','warning',
   'explicit_users','project_setup_actionable','{}','/my-work?setup={{project_id}}',
   'Tiếp tục thiết lập',true,true),
  ('project.setup_handed_over','planning','project_startup','Khởi tạo công trình','outcome','info','users','primary',
   'explicit_users','project_scope','{}','/my-work?setup={{project_id}}',
   'Xem bàn giao',false,true),
  ('project.setup_completed','planning','project_startup','Khởi tạo công trình','outcome','success','check','success',
   'explicit_users','project_scope','{}','/projects/{{project_id}}?tab=workflow',
   'Mở quy trình',false,true),
  ('project.setup_batch_required','planning','project_startup','Khởi tạo công trình','action_required','warning','workflow','warning',
   'explicit_users','actor_only','{}','/my-work?startup=all',
   'Mở danh sách cần thiết lập',true,true),
  ('work_item.assigned','workflow','work','Công việc','action_required','info','task','primary',
   'work_item_responsible','work_item_actionable','{}','/my-work?task={{work_item_id}}',
   'Mở công việc',true,true)
ON CONFLICT (event_key) DO UPDATE SET
  domain_key=EXCLUDED.domain_key,
  category_key=EXCLUDED.category_key,
  category_label=EXCLUDED.category_label,
  intent=EXCLUDED.intent,
  severity=EXCLUDED.severity,
  icon_key=EXCLUDED.icon_key,
  tone_key=EXCLUDED.tone_key,
  audience_policy_key=EXCLUDED.audience_policy_key,
  visibility_policy_key=EXCLUDED.visibility_policy_key,
  permissions_any=EXCLUDED.permissions_any,
  action_route_template=EXCLUDED.action_route_template,
  action_label=EXCLUDED.action_label,
  required_delivery=EXCLUDED.required_delivery,
  enabled=EXCLUDED.enabled,
  updated_at=now();

INSERT INTO notification_templates (
  event_key,locale,version,title_template,body_template,required_payload_keys,active
) VALUES
  ('project.setup_required','vi-VN',1,
   'Công trình mới cần được thiết lập',
   'Anh/chị đang phụ trách thiết lập công trình {{project_code}} · {{project_name}} trước khi bắt đầu quy trình.',
   ARRAY['project_id','project_code','project_name'],true),
  ('project.setup_handed_over','vi-VN',1,
   'Đã bàn giao thiết lập công trình',
   'Trách nhiệm thiết lập công trình {{project_code}} · {{project_name}} đã được bàn giao cho {{owner_name}}.',
   ARRAY['project_id','project_code','project_name','owner_name'],true),
  ('project.setup_completed','vi-VN',1,
   'Công trình đã bắt đầu quy trình',
   '{{actor_name}} đã hoàn tất thiết lập và bắt đầu quy trình cho {{project_code}} · {{project_name}}.',
   ARRAY['actor_name','project_id','project_code','project_name'],true),
  ('project.setup_batch_required','vi-VN',1,
   'Danh sách công trình mới cần thiết lập',
   'Lô nhập vừa tạo {{project_count}} công trình. Anh/chị cần hoàn tất nhân sự và khởi động quy trình cho các công trình này.',
   ARRAY['project_count'],true),
  ('work_item.assigned','vi-VN',1,
   'Công việc mới đã sẵn sàng',
   'Công việc "{{work_item_title}}" tại {{project_name}} đã đến lượt nhóm vai trò của anh/chị.',
   ARRAY['work_item_id','work_item_title','project_id','project_name'],true)
ON CONFLICT (event_key,locale,version) DO UPDATE SET
  title_template=EXCLUDED.title_template,
  body_template=EXCLUDED.body_template,
  required_payload_keys=EXCLUDED.required_payload_keys,
  active=EXCLUDED.active;
