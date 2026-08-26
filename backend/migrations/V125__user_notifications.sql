CREATE TABLE IF NOT EXISTS user_notifications (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  recipient_user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  actor_user_id UUID REFERENCES users(id) ON DELETE SET NULL,
  project_id UUID REFERENCES projects(id) ON DELETE CASCADE,
  notification_type TEXT NOT NULL,
  source_kind TEXT NOT NULL,
  source_id UUID NOT NULL,
  source_event_id UUID NOT NULL,
  title_snapshot TEXT NOT NULL,
  message_snapshot TEXT NOT NULL,
  action_key TEXT NOT NULL,
  action_payload_json JSONB NOT NULL DEFAULT '{}',
  idempotency_key TEXT NOT NULL,
  read_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT user_notifications_type_nonblank CHECK (NULLIF(btrim(notification_type),'') IS NOT NULL),
  CONSTRAINT user_notifications_source_kind_nonblank CHECK (NULLIF(btrim(source_kind),'') IS NOT NULL),
  CONSTRAINT user_notifications_title_nonblank CHECK (NULLIF(btrim(title_snapshot),'') IS NOT NULL),
  CONSTRAINT user_notifications_message_nonblank CHECK (NULLIF(btrim(message_snapshot),'') IS NOT NULL),
  CONSTRAINT user_notifications_action_nonblank CHECK (NULLIF(btrim(action_key),'') IS NOT NULL),
  CONSTRAINT user_notifications_payload_object CHECK (jsonb_typeof(action_payload_json)='object'),
  CONSTRAINT user_notifications_idempotency_nonblank CHECK (NULLIF(btrim(idempotency_key),'') IS NOT NULL),
  UNIQUE (recipient_user_id,idempotency_key),
  UNIQUE (recipient_user_id,notification_type,source_kind,source_event_id)
);

CREATE INDEX IF NOT EXISTS idx_user_notifications_recipient_created
  ON user_notifications(recipient_user_id,created_at DESC,id DESC);

CREATE INDEX IF NOT EXISTS idx_user_notifications_recipient_unread
  ON user_notifications(recipient_user_id,created_at DESC,id DESC)
  WHERE read_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_user_notifications_recipient_type_created
  ON user_notifications(recipient_user_id,notification_type,created_at DESC,id DESC);

CREATE INDEX IF NOT EXISTS idx_user_notifications_project
  ON user_notifications(project_id,created_at DESC,id DESC)
  WHERE project_id IS NOT NULL;
