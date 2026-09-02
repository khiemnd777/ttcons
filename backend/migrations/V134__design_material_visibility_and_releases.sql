-- Design needs a read-only material catalog without warehouse/stock authority.
INSERT INTO permissions (id, key, name, module, description)
VALUES (
  gen_random_uuid(),
  'material.view',
  'Xem danh mục vật tư',
  'warehouse',
  'Chỉ xem danh mục vật tư; không cấp quyền xem tồn kho hoặc thao tác kho'
)
ON CONFLICT (key) DO UPDATE
SET name=EXCLUDED.name,
    module=EXCLUDED.module,
    description=EXCLUDED.description;

INSERT INTO role_permissions (role_id, permission_id)
SELECT role.id, permission.id
FROM roles role
JOIN permissions permission ON permission.key='material.view'
WHERE role.name IN ('design','admin','developer')
ON CONFLICT DO NOTHING;

CREATE TABLE IF NOT EXISTS design_releases (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  work_item_id UUID,
  revision_no INTEGER NOT NULL,
  revision_label TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'draft',
  change_reason TEXT NOT NULL,
  notes TEXT,
  created_by UUID REFERENCES users(id),
  released_by UUID REFERENCES users(id),
  released_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT design_releases_work_item_project_fk
    FOREIGN KEY (work_item_id, project_id) REFERENCES work_items(id, project_id) ON DELETE SET NULL (work_item_id),
  CONSTRAINT design_releases_revision_no_check CHECK (revision_no > 0),
  CONSTRAINT design_releases_revision_label_not_blank CHECK (btrim(revision_label) <> ''),
  CONSTRAINT design_releases_change_reason_not_blank CHECK (btrim(change_reason) <> ''),
  CONSTRAINT design_releases_status_check CHECK (status IN ('draft','released','superseded')),
  CONSTRAINT design_releases_release_actor_check CHECK (
    (status='draft' AND released_by IS NULL AND released_at IS NULL)
    OR (status IN ('released','superseded') AND released_by IS NOT NULL AND released_at IS NOT NULL)
  ),
  CONSTRAINT design_releases_id_project_unique UNIQUE (id, project_id),
  UNIQUE (project_id, revision_no),
  UNIQUE (project_id, revision_label)
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_design_releases_project_draft
  ON design_releases(project_id)
  WHERE status='draft';

CREATE UNIQUE INDEX IF NOT EXISTS ux_design_releases_project_current
  ON design_releases(project_id)
  WHERE status='released';

CREATE INDEX IF NOT EXISTS idx_design_releases_project_history
  ON design_releases(project_id, revision_no DESC);

CREATE UNIQUE INDEX IF NOT EXISTS ux_design_releases_project_label_ci
  ON design_releases(project_id, lower(revision_label));

CREATE TABLE IF NOT EXISTS design_release_handoffs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  design_release_id UUID NOT NULL,
  work_item_id UUID,
  comment TEXT,
  handed_off_by UUID REFERENCES users(id),
  handed_off_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT design_release_handoffs_work_item_project_fk
    FOREIGN KEY (work_item_id, project_id) REFERENCES work_items(id, project_id) ON DELETE SET NULL (work_item_id),
  CONSTRAINT design_release_handoffs_release_project_fk
    FOREIGN KEY (design_release_id, project_id) REFERENCES design_releases(id, project_id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_design_release_handoffs_project_latest
  ON design_release_handoffs(project_id, handed_off_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_design_release_handoffs_work_item
  ON design_release_handoffs(work_item_id, handed_off_at DESC)
  WHERE work_item_id IS NOT NULL;

CREATE OR REPLACE FUNCTION enforce_design_release_immutability()
RETURNS trigger AS $$
BEGIN
  IF OLD.status='draft' AND NEW.status IN ('draft','released') THEN
    RETURN NEW;
  END IF;

  IF OLD.status='released'
     AND NEW.status='superseded'
     AND NEW.project_id=OLD.project_id
     AND NEW.work_item_id IS NOT DISTINCT FROM OLD.work_item_id
     AND NEW.revision_no=OLD.revision_no
     AND NEW.revision_label=OLD.revision_label
     AND NEW.change_reason=OLD.change_reason
     AND NEW.notes IS NOT DISTINCT FROM OLD.notes
     AND NEW.created_by IS NOT DISTINCT FROM OLD.created_by
     AND NEW.released_by IS NOT DISTINCT FROM OLD.released_by
     AND NEW.released_at IS NOT DISTINCT FROM OLD.released_at
     AND NEW.created_at=OLD.created_at THEN
    RETURN NEW;
  END IF;

  RAISE EXCEPTION 'released design revisions are immutable';
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_design_release_immutability ON design_releases;
CREATE TRIGGER trg_design_release_immutability
BEFORE UPDATE ON design_releases
FOR EACH ROW EXECUTE FUNCTION enforce_design_release_immutability();

CREATE OR REPLACE FUNCTION enforce_design_release_handoff_immutability()
RETURNS trigger AS $$
BEGIN
  RAISE EXCEPTION 'design release handoffs are append-only';
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_design_release_handoff_immutability ON design_release_handoffs;
CREATE TRIGGER trg_design_release_handoff_immutability
BEFORE UPDATE ON design_release_handoffs
FOR EACH ROW EXECUTE FUNCTION enforce_design_release_handoff_immutability();
