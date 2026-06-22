CREATE TABLE IF NOT EXISTS partners (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  partner_code TEXT NOT NULL UNIQUE,
  name TEXT NOT NULL,
  legal_name TEXT,
  partner_types JSONB NOT NULL DEFAULT '[]'::jsonb,
  tax_code TEXT,
  phone TEXT,
  email TEXT,
  address TEXT,
  contact_name TEXT,
  payment_terms_days INTEGER NOT NULL DEFAULT 0,
  notes TEXT,
  active BOOLEAN NOT NULL DEFAULT true,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT partners_types_array_check CHECK (jsonb_typeof(partner_types) = 'array'),
  CONSTRAINT partners_payment_terms_nonnegative CHECK (payment_terms_days >= 0)
);

CREATE INDEX IF NOT EXISTS idx_partners_active_name
  ON partners(active, lower(name));

CREATE INDEX IF NOT EXISTS idx_partners_tax_code
  ON partners(tax_code)
  WHERE tax_code IS NOT NULL AND tax_code <> '';

CREATE INDEX IF NOT EXISTS idx_partners_types
  ON partners USING GIN (partner_types);

ALTER TABLE projects
  ADD COLUMN IF NOT EXISTS partner_id UUID REFERENCES partners(id) ON DELETE SET NULL;

ALTER TABLE materials
  ADD COLUMN IF NOT EXISTS default_supplier_partner_id UUID REFERENCES partners(id) ON DELETE SET NULL;

ALTER TABLE procurement_request_items
  ADD COLUMN IF NOT EXISTS supplier_partner_id UUID REFERENCES partners(id) ON DELETE SET NULL;

ALTER TABLE payables
  ADD COLUMN IF NOT EXISTS vendor_partner_id UUID REFERENCES partners(id) ON DELETE SET NULL;

ALTER TABLE receivables
  ADD COLUMN IF NOT EXISTS client_partner_id UUID REFERENCES partners(id) ON DELETE SET NULL;

ALTER TABLE input_invoices
  ADD COLUMN IF NOT EXISTS vendor_partner_id UUID REFERENCES partners(id) ON DELETE SET NULL;

ALTER TABLE output_invoices
  ADD COLUMN IF NOT EXISTS client_partner_id UUID REFERENCES partners(id) ON DELETE SET NULL;

ALTER TABLE incoming_acceptances
  ADD COLUMN IF NOT EXISTS supplier_partner_id UUID REFERENCES partners(id) ON DELETE SET NULL,
  ADD COLUMN IF NOT EXISTS subcontractor_partner_id UUID REFERENCES partners(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_projects_partner_id ON projects(partner_id);
CREATE INDEX IF NOT EXISTS idx_materials_default_supplier_partner_id ON materials(default_supplier_partner_id);
CREATE INDEX IF NOT EXISTS idx_procurement_items_supplier_partner_id ON procurement_request_items(supplier_partner_id);
CREATE INDEX IF NOT EXISTS idx_payables_vendor_partner_id ON payables(vendor_partner_id);
CREATE INDEX IF NOT EXISTS idx_receivables_client_partner_id ON receivables(client_partner_id);
CREATE INDEX IF NOT EXISTS idx_input_invoices_vendor_partner_id ON input_invoices(vendor_partner_id);
CREATE INDEX IF NOT EXISTS idx_output_invoices_client_partner_id ON output_invoices(client_partner_id);
CREATE INDEX IF NOT EXISTS idx_incoming_acceptances_supplier_partner_id ON incoming_acceptances(supplier_partner_id);
CREATE INDEX IF NOT EXISTS idx_incoming_acceptances_subcontractor_partner_id ON incoming_acceptances(subcontractor_partner_id);

WITH raw_candidates AS (
  SELECT 'client'::text AS partner_type, client_name AS name, NULL::text AS tax_code FROM projects
  UNION ALL
  SELECT 'client', client_name, NULL::text FROM receivables
  UNION ALL
  SELECT 'client', client_name, client_tax_code FROM output_invoices
  UNION ALL
  SELECT
    CASE
      WHEN vendor_type IN ('supplier','subcontractor','labor_team','transport','equipment_rental','service_provider','other') THEN vendor_type
      ELSE 'supplier'
    END,
    vendor_name,
    NULL::text
  FROM payables
  UNION ALL
  SELECT 'supplier', vendor_name, vendor_tax_code FROM input_invoices
  UNION ALL
  SELECT 'supplier', default_supplier, NULL::text FROM materials
  UNION ALL
  SELECT 'supplier', supplier_name, NULL::text FROM procurement_request_items
  UNION ALL
  SELECT 'supplier', supplier_name, NULL::text FROM incoming_acceptances
  UNION ALL
  SELECT 'subcontractor', subcontractor_name, NULL::text FROM incoming_acceptances
),
normalized AS (
  SELECT
    lower(trim(name)) AS name_key,
    COALESCE(NULLIF(trim(tax_code), ''), '') AS tax_key,
    min(trim(name)) AS name,
    NULLIF(max(NULLIF(trim(tax_code), '')), '') AS tax_code,
    jsonb_agg(DISTINCT partner_type) AS partner_types
  FROM raw_candidates
  WHERE NULLIF(trim(name), '') IS NOT NULL
  GROUP BY lower(trim(name)), COALESCE(NULLIF(trim(tax_code), ''), '')
)
INSERT INTO partners (partner_code, name, partner_types, tax_code, active)
SELECT
  'PT-' || upper(substr(md5(name_key || ':' || tax_key), 1, 10)),
  name,
  partner_types,
  tax_code,
  true
FROM normalized
ON CONFLICT (partner_code) DO NOTHING;

UPDATE projects p
SET partner_id = pr.id
FROM partners pr
WHERE p.partner_id IS NULL
  AND pr.partner_types ? 'client'
  AND lower(trim(p.client_name)) = lower(trim(pr.name));

UPDATE materials m
SET default_supplier_partner_id = pr.id
FROM partners pr
WHERE m.default_supplier_partner_id IS NULL
  AND pr.partner_types ? 'supplier'
  AND lower(trim(m.default_supplier)) = lower(trim(pr.name));

UPDATE procurement_request_items pri
SET supplier_partner_id = pr.id
FROM partners pr
WHERE pri.supplier_partner_id IS NULL
  AND pr.partner_types ? 'supplier'
  AND lower(trim(pri.supplier_name)) = lower(trim(pr.name));

UPDATE payables pay
SET vendor_partner_id = pr.id
FROM partners pr
WHERE pay.vendor_partner_id IS NULL
  AND lower(trim(pay.vendor_name)) = lower(trim(pr.name))
  AND (
    pr.partner_types ? pay.vendor_type
    OR pr.partner_types ? 'supplier'
    OR pr.partner_types ? 'subcontractor'
  );

UPDATE receivables rec
SET client_partner_id = pr.id
FROM partners pr
WHERE rec.client_partner_id IS NULL
  AND pr.partner_types ? 'client'
  AND lower(trim(rec.client_name)) = lower(trim(pr.name));

UPDATE input_invoices ii
SET vendor_partner_id = pr.id
FROM partners pr
WHERE ii.vendor_partner_id IS NULL
  AND lower(trim(ii.vendor_name)) = lower(trim(pr.name))
  AND (
    COALESCE(NULLIF(trim(ii.vendor_tax_code), ''), '') = ''
    OR COALESCE(NULLIF(trim(pr.tax_code), ''), '') = COALESCE(NULLIF(trim(ii.vendor_tax_code), ''), '')
  );

UPDATE output_invoices oi
SET client_partner_id = pr.id
FROM partners pr
WHERE oi.client_partner_id IS NULL
  AND pr.partner_types ? 'client'
  AND lower(trim(oi.client_name)) = lower(trim(pr.name))
  AND (
    COALESCE(NULLIF(trim(oi.client_tax_code), ''), '') = ''
    OR COALESCE(NULLIF(trim(pr.tax_code), ''), '') = COALESCE(NULLIF(trim(oi.client_tax_code), ''), '')
  );

UPDATE incoming_acceptances ia
SET supplier_partner_id = pr.id
FROM partners pr
WHERE ia.supplier_partner_id IS NULL
  AND pr.partner_types ? 'supplier'
  AND lower(trim(ia.supplier_name)) = lower(trim(pr.name));

UPDATE incoming_acceptances ia
SET subcontractor_partner_id = pr.id
FROM partners pr
WHERE ia.subcontractor_partner_id IS NULL
  AND pr.partner_types ? 'subcontractor'
  AND lower(trim(ia.subcontractor_name)) = lower(trim(pr.name));

INSERT INTO permissions (id, key, name, module)
SELECT gen_random_uuid(), key, name, module
FROM (VALUES
  ('catalog.view', 'View master data catalog', 'catalog'),
  ('catalog.manage', 'Manage master data catalog', 'catalog')
) AS p(key, name, module)
ON CONFLICT (key) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
JOIN permissions p ON p.key IN ('catalog.view', 'catalog.manage')
WHERE r.name = 'admin'
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
JOIN permissions p ON p.key = 'catalog.view'
WHERE r.name IN ('director', 'pm', 'qs', 'accounting', 'procurement', 'warehouse', 'supervisor', 'qa_qc')
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
JOIN permissions p ON p.key = 'catalog.manage'
WHERE r.name IN ('accounting', 'procurement')
ON CONFLICT DO NOTHING;
