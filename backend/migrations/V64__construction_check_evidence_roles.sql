CREATE TABLE IF NOT EXISTS construction_check_evidence (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  check_id UUID NOT NULL REFERENCES construction_checks(id) ON DELETE CASCADE,
  evidence_id UUID NOT NULL REFERENCES evidence_records(id),
  evidence_role TEXT NOT NULL,
  sort_order INTEGER NOT NULL DEFAULT 0,
  created_by UUID REFERENCES users(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT construction_check_evidence_role_check CHECK (evidence_role IN ('context','result')),
  CONSTRAINT construction_check_evidence_sort_check CHECK (sort_order >= 0),
  CONSTRAINT uq_construction_check_evidence_role UNIQUE (check_id,evidence_id,evidence_role)
);

CREATE INDEX IF NOT EXISTS idx_construction_check_evidence_check_role
  ON construction_check_evidence(check_id,evidence_role,sort_order,created_at,id);

INSERT INTO construction_check_evidence (check_id,evidence_id,evidence_role,sort_order,created_by,created_at)
SELECT c.id,c.evidence_id,
       CASE WHEN c.status='pending' THEN 'context' ELSE 'result' END,
       0,COALESCE(c.checked_by,c.requested_by),COALESCE(c.checked_at,c.created_at)
FROM construction_checks c
WHERE c.evidence_id IS NOT NULL
ON CONFLICT (check_id,evidence_id,evidence_role) DO NOTHING;
