-- Business updates are authored by the source transaction, never by the chat API.
ALTER TABLE work_item_activities DROP CONSTRAINT work_item_activities_type_check;
ALTER TABLE work_item_activities ADD CONSTRAINT work_item_activities_type_check CHECK (activity_type IN (
  'created','started','completed','blocked','blocker_resolved','status_changed',
  'comment','status','blocker','question','decision','reminder','file_note','system_event',
  'evidence_added','evidence_accepted','evidence_rejected','review_requested','approval_requested',
  'approved','rejected','changes_requested','work_update'
));

CREATE TABLE activity_work_updates (
  activity_id UUID PRIMARY KEY REFERENCES work_item_activities(id) ON DELETE CASCADE,
  source_event_key TEXT NOT NULL UNIQUE,
  source_type TEXT NOT NULL CHECK (source_type IN ('workflow_form','design_release','evidence')),
  source_id UUID NOT NULL,
  source_snapshot_json JSONB NOT NULL,
  CHECK (jsonb_typeof(source_snapshot_json) = 'object')
);

CREATE FUNCTION prevent_activity_work_update_rewrite() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  RAISE EXCEPTION 'Published discussion update snapshots cannot be rewritten';
END;
$$;
CREATE TRIGGER activity_work_updates_immutable
BEFORE UPDATE ON activity_work_updates
FOR EACH ROW EXECUTE FUNCTION prevent_activity_work_update_rewrite();
