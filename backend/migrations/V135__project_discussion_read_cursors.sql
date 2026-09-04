CREATE TABLE IF NOT EXISTS project_discussion_read_cursors (
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    last_read_activity_id UUID REFERENCES work_item_activities(id) ON DELETE SET NULL,
    last_read_created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (project_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_project_discussion_read_cursors_user_updated
    ON project_discussion_read_cursors (user_id, updated_at DESC);
