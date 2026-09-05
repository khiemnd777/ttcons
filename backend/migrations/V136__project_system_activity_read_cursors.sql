-- A single rollout baseline prevents historical system activity becoming unread.
CREATE TABLE project_activity_tracking_start (
    singleton BOOLEAN PRIMARY KEY DEFAULT true CHECK (singleton),
    enabled_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);
INSERT INTO project_activity_tracking_start (singleton) VALUES (true);

CREATE TABLE project_system_activity_read_cursors (
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    last_read_activity_id UUID REFERENCES work_item_activities(id) ON DELETE SET NULL,
    last_read_created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (project_id, user_id)
);
