CREATE TABLE IF NOT EXISTS acceptance_checklist_item_evidence (
  checklist_item_id UUID NOT NULL REFERENCES acceptance_checklist_items(id) ON DELETE CASCADE,
  evidence_id UUID NOT NULL REFERENCES evidence_records(id),
  sort_order INTEGER NOT NULL DEFAULT 0,
  created_by UUID REFERENCES users(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT acceptance_checklist_item_evidence_sort_check CHECK (sort_order >= 0),
  CONSTRAINT acceptance_checklist_item_evidence_unique UNIQUE (checklist_item_id,evidence_id),
  CONSTRAINT acceptance_checklist_item_evidence_order_unique UNIQUE (checklist_item_id,sort_order)
);

CREATE INDEX IF NOT EXISTS idx_acceptance_checklist_item_evidence_item
  ON acceptance_checklist_item_evidence(checklist_item_id,sort_order,created_at,evidence_id);

CREATE INDEX IF NOT EXISTS idx_acceptance_checklist_item_evidence_evidence
  ON acceptance_checklist_item_evidence(evidence_id);

INSERT INTO acceptance_checklist_item_evidence (
  checklist_item_id,
  evidence_id,
  sort_order,
  created_by,
  created_at
)
SELECT
  item.id,
  item.evidence_id,
  0,
  acceptance.requested_by,
  item.created_at
FROM acceptance_checklist_items item
JOIN acceptance_records acceptance ON acceptance.id=item.acceptance_id
WHERE item.evidence_id IS NOT NULL
ON CONFLICT (checklist_item_id,evidence_id) DO NOTHING;
