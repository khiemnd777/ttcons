ALTER TABLE handover_records
  ADD COLUMN IF NOT EXISTS handover_code TEXT,
  ADD COLUMN IF NOT EXISTS workflow_form_instance_id UUID REFERENCES workflow_form_instances(id) ON DELETE SET NULL,
  ADD COLUMN IF NOT EXISTS readiness_snapshot_json JSONB NOT NULL DEFAULT '{}',
  ADD COLUMN IF NOT EXISTS submitted_by UUID REFERENCES users(id),
  ADD COLUMN IF NOT EXISTS submitted_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS checked_by UUID REFERENCES users(id),
  ADD COLUMN IF NOT EXISTS checked_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS completed_at TIMESTAMPTZ;

UPDATE handover_records
SET handover_code='HO-' || upper(substr(replace(id::text,'-',''),1,10))
WHERE NULLIF(btrim(handover_code),'') IS NULL;

UPDATE handover_records
SET status='submitted'
WHERE status='checking';

ALTER TABLE handover_records
  ALTER COLUMN handover_code SET NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS ux_handover_project_code
  ON handover_records(project_id,handover_code);

CREATE UNIQUE INDEX IF NOT EXISTS ux_handover_id_project
  ON handover_records(id,project_id);

ALTER TABLE handover_records
  DROP CONSTRAINT IF EXISTS handover_records_status_check;

ALTER TABLE handover_records
  ADD CONSTRAINT handover_records_status_check
  CHECK (status IN ('draft','submitted','ready','blocked','handed_over','cancelled','deleted'));

CREATE TABLE IF NOT EXISTS handover_checklist_items (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  handover_id UUID NOT NULL,
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  item_code TEXT NOT NULL,
  category TEXT NOT NULL,
  title TEXT NOT NULL,
  required BOOLEAN NOT NULL DEFAULT true,
  status TEXT NOT NULL DEFAULT 'pending',
  source_type TEXT,
  source_id UUID,
  evidence_id UUID REFERENCES evidence_records(id) ON DELETE RESTRICT,
  document_id UUID,
  notes TEXT,
  verified_by UUID REFERENCES users(id),
  verified_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT handover_checklist_handover_project_fk
    FOREIGN KEY (handover_id,project_id) REFERENCES handover_records(id,project_id) ON DELETE CASCADE,
  CONSTRAINT handover_checklist_document_project_fk
    FOREIGN KEY (document_id,project_id) REFERENCES documents(id,project_id) ON DELETE RESTRICT,
  CONSTRAINT handover_checklist_unique UNIQUE (handover_id,item_code),
  CONSTRAINT handover_checklist_category_check CHECK (category IN ('quality','as_built','operation','warranty','finance','legal','other')),
  CONSTRAINT handover_checklist_status_check CHECK (status IN ('pending','verified','waived')),
  CONSTRAINT handover_checklist_verified_source_check CHECK (
    status <> 'verified' OR source_id IS NOT NULL OR evidence_id IS NOT NULL OR document_id IS NOT NULL
  ),
  CONSTRAINT handover_checklist_waiver_note_check CHECK (
    status <> 'waived' OR (NOT required AND NULLIF(btrim(COALESCE(notes,'')),'') IS NOT NULL)
  )
);

CREATE INDEX IF NOT EXISTS idx_handover_checklist_status
  ON handover_checklist_items(handover_id,status,category);

INSERT INTO permissions (id,key,name,description,module)
SELECT gen_random_uuid(),'handover.verify','Xác minh hồ sơ bàn giao','Xác minh checklist, chứng từ và readiness trước bàn giao','handover'
ON CONFLICT (key) DO NOTHING;

INSERT INTO role_permissions (role_id,permission_id)
SELECT r.id,p.id
FROM roles r CROSS JOIN permissions p
WHERE p.key='handover.verify'
  AND r.name IN ('admin','director','pm','qa_qc')
ON CONFLICT DO NOTHING;
