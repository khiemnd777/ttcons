UPDATE notification_event_types
SET audience_policy_key='explicit_users',
    visibility_policy_key='project_scope',
    permissions_any='{}',
    action_route_template='/projects/{{project_id}}?tab=activity',
    action_label='Mở trao đổi',
    required_delivery=true,
    enabled=true,
    updated_at=now()
WHERE event_key='activity.mentioned';

INSERT INTO notification_templates (
  event_key,locale,version,title_template,body_template,required_payload_keys,active
) VALUES (
  'activity.mentioned','vi-VN',1,
  'Anh/chị được nhắc đến trong trao đổi',
  '{{actor_name}} đã nhắc đến anh/chị trong một trao đổi công việc.',
  ARRAY['actor_name','project_id','activity_id'],true
)
ON CONFLICT (event_key,locale,version) DO UPDATE SET
  title_template=EXCLUDED.title_template,
  body_template=EXCLUDED.body_template,
  required_payload_keys=EXCLUDED.required_payload_keys,
  active=EXCLUDED.active;
