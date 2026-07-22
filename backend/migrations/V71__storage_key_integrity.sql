ALTER TABLE evidence_records
  ADD CONSTRAINT evidence_records_internal_storage_key_canonical_check CHECK (
    storage_provider NOT IN ('local', 's3')
    OR
    (
      storage_key IS NOT NULL
      AND btrim(storage_key) <> ''
      AND storage_key = btrim(storage_key)
    )
  ) NOT VALID;

ALTER TABLE workflow_form_signatures
  ADD CONSTRAINT workflow_form_signatures_visual_storage_key_canonical_check CHECK (
    visual_storage_provider NOT IN ('local', 's3')
    OR
    (
      visual_storage_key IS NOT NULL
      AND btrim(visual_storage_key) <> ''
      AND visual_storage_key = btrim(visual_storage_key)
    )
  ) NOT VALID;

ALTER TABLE workflow_form_pdf_exports
  ADD CONSTRAINT workflow_form_pdf_exports_storage_key_canonical_check CHECK (
    storage_provider NOT IN ('local', 's3')
    OR
    (
      storage_key IS NOT NULL
      AND btrim(storage_key) <> ''
      AND storage_key = btrim(storage_key)
    )
  ) NOT VALID;

ALTER TABLE input_invoices
  ADD CONSTRAINT input_invoices_storage_key_canonical_check CHECK (
    storage_provider NOT IN ('local', 's3')
    OR
    (
      storage_key IS NOT NULL
      AND btrim(storage_key) <> ''
      AND storage_key = btrim(storage_key)
    )
  ) NOT VALID;

ALTER TABLE output_invoices
  ADD CONSTRAINT output_invoices_storage_key_canonical_check CHECK (
    storage_provider NOT IN ('local', 's3')
    OR
    (
      storage_key IS NOT NULL
      AND btrim(storage_key) <> ''
      AND storage_key = btrim(storage_key)
    )
  ) NOT VALID;
