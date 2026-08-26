INSERT INTO notification_event_types (
  event_key,domain_key,category_key,category_label,intent,severity,icon_key,tone_key,
  audience_policy_key,visibility_policy_key,permissions_any,action_route_template,
  action_label,required_delivery,enabled
) VALUES
  ('work_item.approval_requested','workflow','work','Công việc','action_required','warning','approval','warning',
   'approval_deciders','project_scope','{}','/my-work?mode=approval&task={{work_item_id}}',
   'Mở hàng chờ duyệt',true,true),
  ('work_item.changes_requested','workflow','work','Công việc','action_required','warning','alert','warning',
   'explicit_users','project_scope','{}','/my-work?bucket=all&task={{work_item_id}}',
   'Mở công việc',true,true),
  ('work_item.approved','workflow','work','Công việc','outcome','success','check','success',
   'explicit_users','project_scope','{}','/my-work?bucket=all&task={{work_item_id}}',
   'Mở công việc',false,true),
  ('work_item.rejected','workflow','work','Công việc','outcome','critical','x','critical',
   'explicit_users','project_scope','{}','/my-work?bucket=all&task={{work_item_id}}',
   'Mở công việc',false,true)
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
  ('work_item.approval_requested','vi-VN',1,
   '{{actor_name}} yêu cầu duyệt công việc',
   'Công việc "{{work_item_title}}" đang chờ anh/chị xử lý.',
   ARRAY['actor_name','work_item_title','work_item_id','project_id','approval_request_id'],true),
  ('work_item.changes_requested','vi-VN',1,
   'Công việc cần bổ sung',
   '{{actor_name}} yêu cầu bổ sung cho công việc "{{work_item_title}}".',
   ARRAY['actor_name','work_item_title','work_item_id','project_id'],true),
  ('work_item.approved','vi-VN',1,
   'Công việc đã được duyệt',
   '{{actor_name}} đã duyệt công việc "{{work_item_title}}".',
   ARRAY['actor_name','work_item_title','work_item_id','project_id'],true),
  ('work_item.rejected','vi-VN',1,
   'Công việc bị từ chối',
   '{{actor_name}} đã từ chối công việc "{{work_item_title}}".',
   ARRAY['actor_name','work_item_title','work_item_id','project_id'],true)
ON CONFLICT (event_key,locale,version) DO UPDATE SET
  title_template=EXCLUDED.title_template,
  body_template=EXCLUDED.body_template,
  required_payload_keys=EXCLUDED.required_payload_keys,
  active=EXCLUDED.active;
