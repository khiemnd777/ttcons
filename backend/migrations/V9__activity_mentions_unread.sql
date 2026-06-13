CREATE TABLE IF NOT EXISTS activity_mentions (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  activity_id UUID NOT NULL REFERENCES work_item_activities(id) ON DELETE CASCADE,
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  mention_type TEXT NOT NULL CHECK (mention_type IN ('user', 'staff', 'role')),
  mention_key TEXT NOT NULL,
  recipient_user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  recipient_staff_id UUID REFERENCES staff(id) ON DELETE SET NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (activity_id, mention_type, mention_key, recipient_user_id)
);

CREATE TABLE IF NOT EXISTS activity_read_states (
  activity_id UUID NOT NULL REFERENCES work_item_activities(id) ON DELETE CASCADE,
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  read_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (activity_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_activity_mentions_recipient_created
  ON activity_mentions(recipient_user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_activity_mentions_activity
  ON activity_mentions(activity_id);

CREATE INDEX IF NOT EXISTS idx_activity_read_states_user_unread
  ON activity_read_states(user_id, created_at DESC)
  WHERE read_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_activity_read_states_project_user
  ON activity_read_states(project_id, user_id);
