-- Approved current product: compare imported MISA transactions directly with
-- immutable evidence-backed source bases, without payment proposal lifecycle.
DO $$
DECLARE
  target_table text;
  has_rows boolean;
  nonempty_tables text[] := ARRAY[]::text[];
BEGIN
  FOREACH target_table IN ARRAY ARRAY[
    'accounting_request_bases','accounting_requests','accounting_request_events',
    'accounting_misa_batches','accounting_misa_records','accounting_misa_allocations','accounting_misa_reviews'
  ] LOOP
    EXECUTE format('LOCK TABLE %I IN ACCESS EXCLUSIVE MODE',target_table);
    EXECUTE format('SELECT EXISTS (SELECT 1 FROM %I)',target_table) INTO has_rows;
    IF has_rows THEN nonempty_tables := array_append(nonempty_tables,target_table); END IF;
  END LOOP;
  IF cardinality(nonempty_tables)>0 THEN
    RAISE EXCEPTION 'Direct source reconciliation requires empty accounting business tables: %',array_to_string(nonempty_tables,', ');
  END IF;
END $$;

ALTER TABLE accounting_misa_allocations DROP CONSTRAINT accounting_misa_allocations_request_id_fkey;
ALTER TABLE accounting_misa_reviews DROP CONSTRAINT accounting_misa_reviews_request_id_fkey;
DROP TABLE accounting_request_events;
DROP TABLE accounting_requests;
ALTER TABLE accounting_request_bases RENAME TO accounting_reconciliation_bases;
ALTER TABLE accounting_reconciliation_bases RENAME CONSTRAINT accounting_request_bases_pkey TO accounting_reconciliation_bases_pkey;
ALTER TABLE accounting_reconciliation_bases RENAME CONSTRAINT accounting_request_bases_project_id_fkey TO accounting_reconciliation_bases_project_id_fkey;
ALTER TABLE accounting_reconciliation_bases RENAME CONSTRAINT accounting_request_bases_partner_id_fkey TO accounting_reconciliation_bases_partner_id_fkey;
ALTER TABLE accounting_reconciliation_bases RENAME CONSTRAINT accounting_request_bases_created_by_fkey TO accounting_reconciliation_bases_created_by_fkey;
ALTER TABLE accounting_reconciliation_bases RENAME CONSTRAINT accounting_request_bases_source_type_check TO accounting_reconciliation_bases_source_type_check;
ALTER TABLE accounting_reconciliation_bases RENAME CONSTRAINT accounting_request_bases_direction_check TO accounting_reconciliation_bases_direction_check;
ALTER TABLE accounting_reconciliation_bases RENAME CONSTRAINT accounting_request_bases_amount_check TO accounting_reconciliation_bases_amount_check;
ALTER TABLE accounting_reconciliation_bases RENAME CONSTRAINT accounting_request_bases_note_check TO accounting_reconciliation_bases_note_check;
ALTER TABLE accounting_reconciliation_bases RENAME CONSTRAINT accounting_request_bases_project_id_source_type_source_id_key TO accounting_reconciliation_bases_source_unique;
ALTER TABLE accounting_reconciliation_bases ADD CONSTRAINT accounting_reconciliation_bases_id_project_unique UNIQUE (id,project_id);
ALTER TABLE accounting_misa_records ADD CONSTRAINT accounting_misa_records_id_project_unique UNIQUE (id,project_id);
ALTER TABLE accounting_misa_allocations RENAME COLUMN request_id TO basis_id;
ALTER TABLE accounting_misa_reviews RENAME COLUMN request_id TO basis_id;
ALTER TABLE accounting_misa_allocations DROP CONSTRAINT accounting_misa_allocations_check;
ALTER TABLE accounting_misa_allocations ADD CONSTRAINT accounting_misa_allocations_revocation_check CHECK (
  active OR (revoked_by IS NOT NULL AND revoked_at IS NOT NULL AND NULLIF(btrim(revoke_reason),'') IS NOT NULL)
);
ALTER TABLE accounting_misa_allocations ADD CONSTRAINT accounting_misa_allocations_basis_project_fk
  FOREIGN KEY (basis_id,project_id) REFERENCES accounting_reconciliation_bases(id,project_id);
ALTER TABLE accounting_misa_allocations DROP CONSTRAINT accounting_misa_allocations_record_id_fkey;
ALTER TABLE accounting_misa_allocations ADD CONSTRAINT accounting_misa_allocations_record_project_fk
  FOREIGN KEY (record_id,project_id) REFERENCES accounting_misa_records(id,project_id);
ALTER TABLE accounting_misa_reviews ADD CONSTRAINT accounting_misa_reviews_basis_project_fk
  FOREIGN KEY (basis_id,project_id) REFERENCES accounting_reconciliation_bases(id,project_id);
CREATE INDEX accounting_misa_allocations_active_basis ON accounting_misa_allocations(basis_id,created_at,id) WHERE active;
CREATE INDEX accounting_misa_allocations_active_record ON accounting_misa_allocations(record_id) WHERE active;
CREATE INDEX accounting_misa_reviews_basis_history ON accounting_misa_reviews(basis_id,created_at DESC,id);

-- Table rename preserves accounting_basis_immutable and review immutability.
-- A base is comparison context, never a limit on how much MISA can observe.
-- Serialize all allocation writers for the same imported record. The record
-- cap and canonical dimensions are database invariants even for direct SQL.
CREATE FUNCTION accounting_misa_basis_allocation_guard() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
  imported accounting_misa_records%ROWTYPE;
  source_basis accounting_reconciliation_bases%ROWTYPE;
  allocated numeric(18,3);
BEGIN
  IF NOT NEW.active THEN RAISE EXCEPTION 'New MISA allocations must be active'; END IF;
  SELECT * INTO imported FROM accounting_misa_records WHERE id=NEW.record_id FOR UPDATE;
  IF NOT FOUND OR NOT imported.active OR imported.kind<>'transaction' THEN
    RAISE EXCEPTION 'MISA allocation requires an active transaction';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM accounting_misa_batches WHERE id=imported.batch_id AND status='confirmed') THEN
    RAISE EXCEPTION 'MISA allocation requires a confirmed imported batch';
  END IF;
  SELECT * INTO source_basis FROM accounting_reconciliation_bases WHERE id=NEW.basis_id;
  IF NOT FOUND OR NEW.project_id<>source_basis.project_id OR NEW.project_id<>imported.project_id
    OR source_basis.partner_id<>imported.partner_id OR source_basis.direction<>imported.direction
    OR source_basis.currency<>imported.currency THEN
    RAISE EXCEPTION 'MISA allocation must match source project, partner, direction and currency';
  END IF;
  SELECT COALESCE(sum(amount),0) INTO allocated FROM accounting_misa_allocations WHERE record_id=NEW.record_id AND active;
  IF allocated+NEW.amount>imported.amount THEN
    RAISE EXCEPTION 'MISA allocation exceeds imported transaction amount';
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER accounting_misa_basis_allocation_guard BEFORE INSERT ON accounting_misa_allocations
FOR EACH ROW EXECUTE FUNCTION accounting_misa_basis_allocation_guard();

CREATE OR REPLACE FUNCTION accounting_misa_history_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF TG_OP='DELETE' THEN RAISE EXCEPTION 'Imported MISA history cannot be deleted'; END IF;
  IF TG_TABLE_NAME='accounting_misa_batches' THEN
    IF (to_jsonb(NEW)-'status'-'confirmed_at') IS DISTINCT FROM (to_jsonb(OLD)-'status'-'confirmed_at')
      OR (OLD.status='confirmed' AND NEW IS DISTINCT FROM OLD) THEN RAISE EXCEPTION 'Imported source file is immutable'; END IF;
  ELSIF TG_TABLE_NAME='accounting_misa_records' THEN
    IF (to_jsonb(NEW)-'active') IS DISTINCT FROM (to_jsonb(OLD)-'active') OR NOT OLD.active OR NEW.active THEN
      RAISE EXCEPTION 'Only superseding an active MISA observation is allowed';
    END IF;
    IF EXISTS (SELECT 1 FROM accounting_misa_allocations WHERE record_id=OLD.id AND active) THEN
      RAISE EXCEPTION 'Revoke active MISA allocations before replacing an observation';
    END IF;
  ELSIF TG_TABLE_NAME='accounting_misa_allocations' THEN
    IF (to_jsonb(NEW)-'active'-'revoked_by'-'revoked_at'-'revoke_reason') IS DISTINCT FROM
      (to_jsonb(OLD)-'active'-'revoked_by'-'revoked_at'-'revoke_reason') OR NOT OLD.active OR NEW.active THEN
      RAISE EXCEPTION 'Only revoking an active allocation is allowed';
    END IF;
  END IF;
  RETURN NEW;
END $$;

INSERT INTO permissions(id,key,name,description,module)
VALUES (gen_random_uuid(),'accounting_reconciliation.view','Xem đối soát MISA','Xem căn cứ hồ sơ và kết quả đối soát MISA theo công trình','accounting_handoff'),
  (gen_random_uuid(),'accounting_reconciliation.manage','Xác nhận căn cứ đối soát','Xác nhận căn cứ hồ sơ có nguồn và minh chứng để đối chiếu MISA','accounting_handoff')
ON CONFLICT(key) DO NOTHING;
-- Derive grants from the actual previous permission assignments, never names.
INSERT INTO role_permissions(role_id,permission_id)
SELECT old_grant.role_id,new_permission.id
FROM role_permissions old_grant JOIN permissions old_permission ON old_permission.id=old_grant.permission_id
JOIN permissions new_permission ON new_permission.key=CASE old_permission.key
  WHEN 'accounting_request.view' THEN 'accounting_reconciliation.view'
  WHEN 'accounting_request.review' THEN 'accounting_reconciliation.manage' END
WHERE old_permission.key IN ('accounting_request.view','accounting_request.review')
ON CONFLICT DO NOTHING;
DELETE FROM role_permissions WHERE permission_id IN (SELECT id FROM permissions WHERE key LIKE 'accounting_request.%');
DELETE FROM permissions WHERE key LIKE 'accounting_request.%';
