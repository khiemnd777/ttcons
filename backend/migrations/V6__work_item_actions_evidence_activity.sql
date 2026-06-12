ALTER TABLE work_item_activities
  DROP CONSTRAINT IF EXISTS work_item_activities_type_check;

ALTER TABLE work_item_activities
  ADD CONSTRAINT work_item_activities_type_check CHECK (activity_type IN (
    'created',
    'started',
    'completed',
    'blocked',
    'blocker_resolved',
    'status_changed',
    'comment',
    'evidence_added',
    'evidence_accepted',
    'evidence_rejected',
    'review_requested',
    'approval_requested',
    'approved',
    'rejected',
    'changes_requested'
  ));

CREATE INDEX IF NOT EXISTS idx_evidence_records_status_created
  ON evidence_records(status, created_at DESC);
