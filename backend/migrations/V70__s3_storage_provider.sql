ALTER TABLE evidence_records
  DROP CONSTRAINT IF EXISTS evidence_records_storage_provider_check,
  ADD CONSTRAINT evidence_records_storage_provider_check
    CHECK (storage_provider IN ('external', 'local', 's3'));

ALTER TABLE evidence_records
  DROP CONSTRAINT IF EXISTS evidence_records_storage_key_provider_check,
  ADD CONSTRAINT evidence_records_storage_key_provider_check CHECK (
    storage_key IS NULL OR storage_provider IN ('local', 's3')
  );

ALTER TABLE workflow_form_signatures
  DROP CONSTRAINT IF EXISTS workflow_form_signatures_visual_storage_provider_check,
  ADD CONSTRAINT workflow_form_signatures_visual_storage_provider_check
    CHECK (visual_storage_provider IN ('none', 'local', 's3'));

ALTER TABLE workflow_form_signatures
  DROP CONSTRAINT IF EXISTS workflow_form_signatures_visual_storage_key_check,
  ADD CONSTRAINT workflow_form_signatures_visual_storage_key_check CHECK (
    (visual_storage_provider = 'none' AND visual_storage_key IS NULL)
    OR
    (visual_storage_provider IN ('local', 's3') AND visual_storage_key IS NOT NULL)
  );

ALTER TABLE workflow_form_pdf_exports
  DROP CONSTRAINT IF EXISTS workflow_form_pdf_exports_provider_check,
  ADD CONSTRAINT workflow_form_pdf_exports_provider_check
    CHECK (storage_provider IN ('local', 's3'));

ALTER TABLE input_invoices
  DROP CONSTRAINT IF EXISTS input_invoices_storage_provider_check,
  ADD CONSTRAINT input_invoices_storage_provider_check
    CHECK (storage_provider IN ('local', 's3'));

ALTER TABLE output_invoices
  DROP CONSTRAINT IF EXISTS output_invoices_storage_provider_check,
  ADD CONSTRAINT output_invoices_storage_provider_check
    CHECK (storage_provider IN ('local', 's3'));

CREATE OR REPLACE FUNCTION prevent_workflow_form_signature_mutation()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  IF TG_OP = 'UPDATE'
    AND OLD.visual_storage_provider = 'local'
    AND NEW.visual_storage_provider = 's3'
    AND (to_jsonb(NEW) - 'visual_storage_provider') =
        (to_jsonb(OLD) - 'visual_storage_provider')
  THEN
    RETURN NEW;
  END IF;

  RAISE EXCEPTION 'workflow_form_signatures are immutable';
END;
$$;

DROP TRIGGER IF EXISTS trg_workflow_form_signatures_immutable ON workflow_form_signatures;

CREATE TRIGGER trg_workflow_form_signatures_immutable
BEFORE UPDATE OR DELETE ON workflow_form_signatures
FOR EACH ROW
EXECUTE FUNCTION prevent_workflow_form_signature_mutation();

CREATE OR REPLACE FUNCTION prevent_workflow_form_pdf_export_mutation()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
  IF TG_OP = 'UPDATE'
    AND OLD.storage_provider = 'local'
    AND NEW.storage_provider = 's3'
    AND (to_jsonb(NEW) - 'storage_provider') =
        (to_jsonb(OLD) - 'storage_provider')
  THEN
    RETURN NEW;
  END IF;

  RAISE EXCEPTION 'workflow_form_pdf_exports are immutable';
END;
$$;

DROP TRIGGER IF EXISTS trg_workflow_form_pdf_exports_immutable ON workflow_form_pdf_exports;

CREATE TRIGGER trg_workflow_form_pdf_exports_immutable
BEFORE UPDATE OR DELETE ON workflow_form_pdf_exports
FOR EACH ROW
EXECUTE FUNCTION prevent_workflow_form_pdf_export_mutation();
