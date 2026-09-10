-- Design revisions remain versioned package headers. Their files are canonical
-- evidence records, linked through design_release_items. Legacy document rows
-- remain available for immutable historical discussion snapshots; this
-- migration reuses their existing storage object instead of copying bytes.

CREATE UNIQUE INDEX IF NOT EXISTS ux_evidence_records_id_project
  ON evidence_records(id, project_id);

CREATE TABLE IF NOT EXISTS design_release_items (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  design_release_id UUID NOT NULL,
  evidence_id UUID NOT NULL,
  document_type TEXT NOT NULL,
  position INTEGER NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT design_release_items_release_project_fk
    FOREIGN KEY (design_release_id, project_id)
    REFERENCES design_releases(id, project_id) ON DELETE CASCADE,
  CONSTRAINT design_release_items_evidence_project_fk
    FOREIGN KEY (evidence_id, project_id)
    REFERENCES evidence_records(id, project_id) ON DELETE CASCADE,
  CONSTRAINT design_release_items_document_type_check CHECK (
    document_type IN ('layout','render_3d','construction_drawing','other')
  ),
  CONSTRAINT design_release_items_position_check CHECK (position >= 0),
  CONSTRAINT design_release_items_release_evidence_unique
    UNIQUE (design_release_id, evidence_id),
  CONSTRAINT design_release_items_evidence_unique UNIQUE (evidence_id)
);

CREATE INDEX IF NOT EXISTS idx_design_release_items_release_position
  ON design_release_items(design_release_id, position, created_at, id);

CREATE INDEX IF NOT EXISTS idx_design_release_items_project
  ON design_release_items(project_id, created_at DESC);

DO $$
BEGIN
  IF EXISTS (
    SELECT 1
    FROM documents document
    JOIN design_releases release
      ON release.id=document.target_id
     AND release.project_id=document.project_id
    LEFT JOIN work_items item
      ON item.id=release.work_item_id
     AND item.project_id=release.project_id
     AND item.status <> 'cancelled'
    WHERE document.target_type='design_release'
      AND document.status='active'
      AND item.id IS NULL
  ) THEN
    RAISE EXCEPTION 'Cannot unify design revision evidence: an active legacy design file has no valid project Work Item';
  END IF;

  IF EXISTS (
    SELECT document.storage_provider, document.storage_key
    FROM documents document
    JOIN design_releases release
      ON release.id=document.target_id
     AND release.project_id=document.project_id
    WHERE document.target_type='design_release'
      AND document.status='active'
      AND document.storage_key IS NOT NULL
    GROUP BY document.storage_provider, document.storage_key
    HAVING COUNT(*) > 1
  ) THEN
    RAISE EXCEPTION 'Cannot unify design revision evidence: legacy design files reuse an internal storage key';
  END IF;
END $$;

INSERT INTO evidence_records (
  work_item_id,
  project_id,
  target_type,
  target_id,
  evidence_type,
  document_type,
  name,
  document_id,
  title,
  file_url,
  mime_type,
  file_size,
  storage_provider,
  storage_key,
  checksum_sha256,
  revision_no,
  visibility,
  payload_json,
  status,
  uploaded_by,
  submitted_by,
  reviewed_by,
  reviewed_at,
  created_at,
  updated_at
)
SELECT
  release.work_item_id,
  release.project_id,
  'work_item',
  release.work_item_id,
  CASE WHEN COALESCE(document.mime_type, '') LIKE 'image/%' THEN 'photo' ELSE 'document' END,
  document.document_type,
  document.name,
  document.id,
  document.name,
  document.file_url,
  document.mime_type,
  NULLIF(document.file_size, 0),
  document.storage_provider,
  document.storage_key,
  document.checksum_sha256,
  release.revision_label,
  'internal',
  jsonb_build_object(
    'source_type', 'design_release',
    'design_release_id', release.id,
    'legacy_design_document_id', document.id,
    'original_filename', document.name
  ),
  CASE
    WHEN release.status='draft' THEN 'draft'
    WHEN handoff.id IS NOT NULL THEN 'accepted'
    ELSE 'submitted'
  END,
  document.uploaded_by,
  document.uploaded_by,
  CASE WHEN handoff.id IS NOT NULL THEN handoff.handed_off_by END,
  CASE WHEN handoff.id IS NOT NULL THEN handoff.handed_off_at END,
  document.created_at,
  COALESCE(handoff.handed_off_at, release.released_at, document.created_at)
FROM documents document
JOIN design_releases release
  ON release.id=document.target_id
 AND release.project_id=document.project_id
LEFT JOIN LATERAL (
  SELECT exact_handoff.id, exact_handoff.handed_off_by, exact_handoff.handed_off_at
  FROM design_release_handoffs exact_handoff
  WHERE exact_handoff.design_release_id=release.id
    AND exact_handoff.project_id=release.project_id
  ORDER BY exact_handoff.handed_off_at DESC, exact_handoff.id DESC
  LIMIT 1
) handoff ON TRUE
WHERE document.target_type='design_release'
  AND document.status='active'
  AND NOT EXISTS (
    SELECT 1
    FROM evidence_records evidence
    WHERE evidence.payload_json->>'legacy_design_document_id'=document.id::text
  );

INSERT INTO design_release_items (
  project_id,
  design_release_id,
  evidence_id,
  document_type,
  position,
  created_at
)
SELECT
  release.project_id,
  release.id,
  evidence.id,
  COALESCE(NULLIF(document.document_type, ''), 'other'),
  ROW_NUMBER() OVER (
    PARTITION BY release.id
    ORDER BY document.created_at, document.id
  ) - 1,
  document.created_at
FROM documents document
JOIN design_releases release
  ON release.id=document.target_id
 AND release.project_id=document.project_id
JOIN evidence_records evidence
  ON evidence.payload_json->>'legacy_design_document_id'=document.id::text
WHERE document.target_type='design_release'
  AND document.status='active'
ON CONFLICT (design_release_id, evidence_id) DO NOTHING;

CREATE OR REPLACE FUNCTION enforce_design_release_item_immutability()
RETURNS trigger AS $$
DECLARE
  release_status TEXT;
BEGIN
  SELECT status INTO release_status
  FROM design_releases
  WHERE id=COALESCE(OLD.design_release_id, NEW.design_release_id);

  IF release_status='draft' THEN
    IF TG_OP='DELETE' THEN
      RETURN OLD;
    END IF;
    RETURN NEW;
  END IF;

  RAISE EXCEPTION 'released design revision items are immutable';
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_design_release_item_immutability ON design_release_items;
CREATE TRIGGER trg_design_release_item_immutability
BEFORE UPDATE OR DELETE ON design_release_items
FOR EACH ROW EXECUTE FUNCTION enforce_design_release_item_immutability();
