-- Operational proposals and imported external observations, never a second ledger.
CREATE TABLE accounting_request_bases (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), project_id uuid NOT NULL REFERENCES projects(id),
 source_type text NOT NULL CHECK(source_type IN ('procurement_request','contract_milestone')),
 source_id uuid NOT NULL, partner_id uuid NOT NULL REFERENCES partners(id),
 direction text NOT NULL CHECK(direction IN ('inbound','outbound')), currency text NOT NULL,
 amount numeric(18,3) NOT NULL CHECK(amount > 0), note text NOT NULL CHECK(btrim(note) <> ''),
 snapshot jsonb NOT NULL, evidence_ids jsonb NOT NULL DEFAULT '[]',
 created_by uuid NOT NULL REFERENCES users(id), created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(project_id,source_type,source_id)
);
CREATE TABLE accounting_requests (
 id uuid PRIMARY KEY, project_id uuid NOT NULL REFERENCES projects(id), basis_id uuid NOT NULL REFERENCES accounting_request_bases(id),
 title text NOT NULL, kind text NOT NULL CHECK(kind IN ('advance','progress','final','retention_release')),
 due_date date NOT NULL, gross_amount numeric(18,3) NOT NULL CHECK(gross_amount > 0),
 recovery_amount numeric(18,3) NOT NULL DEFAULT 0 CHECK(recovery_amount >= 0),
 retention_amount numeric(18,3) NOT NULL DEFAULT 0 CHECK(retention_amount >= 0),
 amount numeric(18,3) GENERATED ALWAYS AS (gross_amount-recovery_amount-retention_amount) STORED,
 advance_request_id uuid REFERENCES accounting_requests(id), release_request_id uuid REFERENCES accounting_requests(id), note text NOT NULL DEFAULT '',
 evidence_ids jsonb NOT NULL DEFAULT '[]', stock_document_ids jsonb NOT NULL DEFAULT '[]',
 status text NOT NULL DEFAULT 'draft' CHECK(status IN ('draft','submitted','returned','acknowledged','cancelled')),
 revision integer NOT NULL DEFAULT 1, reason text NOT NULL DEFAULT '',
 created_by uuid NOT NULL REFERENCES users(id), updated_by uuid NOT NULL REFERENCES users(id),
 created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
 CHECK(gross_amount > recovery_amount + retention_amount),
 CHECK((recovery_amount=0 AND advance_request_id IS NULL) OR (recovery_amount>0 AND advance_request_id IS NOT NULL)),
 CHECK(kind <> 'advance' OR (recovery_amount=0 AND retention_amount=0)),
 CHECK((kind='retention_release' AND release_request_id IS NOT NULL AND recovery_amount=0 AND retention_amount=0) OR (kind<>'retention_release' AND release_request_id IS NULL))
);
CREATE INDEX accounting_requests_project_status ON accounting_requests(project_id,status);
CREATE TABLE accounting_request_events (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), request_id uuid NOT NULL REFERENCES accounting_requests(id),
 revision integer NOT NULL, action text NOT NULL, snapshot jsonb NOT NULL,
 actor_id uuid NOT NULL REFERENCES users(id), created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(request_id,revision)
);
CREATE TABLE accounting_misa_batches (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), project_id uuid NOT NULL REFERENCES projects(id),
 entity_code text NOT NULL, filename text NOT NULL, file_content text NOT NULL, digest text NOT NULL,
 status text NOT NULL DEFAULT 'preview' CHECK(status IN ('preview','confirmed')),
 created_by uuid NOT NULL REFERENCES users(id), created_at timestamptz NOT NULL DEFAULT now(), confirmed_at timestamptz,
 UNIQUE(project_id,entity_code,digest)
);
CREATE TABLE accounting_misa_records (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), batch_id uuid NOT NULL REFERENCES accounting_misa_batches(id),
 project_id uuid NOT NULL REFERENCES projects(id), entity_code text NOT NULL, external_id text NOT NULL, line_key text NOT NULL,
 kind text NOT NULL CHECK(kind IN ('transaction','balance')), direction text NOT NULL CHECK(direction IN ('inbound','outbound')),
 partner_id uuid NOT NULL REFERENCES partners(id), currency text NOT NULL, amount numeric(18,3) NOT NULL CHECK(amount >= 0),
 record_date date NOT NULL, replaces_id uuid REFERENCES accounting_misa_records(id), active boolean NOT NULL DEFAULT true,
 UNIQUE(batch_id,external_id,line_key,kind)
);
CREATE UNIQUE INDEX accounting_misa_current_identity ON accounting_misa_records(project_id,entity_code,external_id,line_key,kind) WHERE active;
CREATE TABLE accounting_misa_allocations (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), project_id uuid NOT NULL REFERENCES projects(id),
 request_id uuid NOT NULL REFERENCES accounting_requests(id), record_id uuid NOT NULL REFERENCES accounting_misa_records(id),
 amount numeric(18,3) NOT NULL CHECK(amount > 0), active boolean NOT NULL DEFAULT true,
 created_by uuid NOT NULL REFERENCES users(id), created_at timestamptz NOT NULL DEFAULT now(),
 revoked_by uuid REFERENCES users(id), revoked_at timestamptz, revoke_reason text,
 CHECK(active OR (revoked_by IS NOT NULL AND revoked_at IS NOT NULL AND btrim(revoke_reason) <> ''))
);
CREATE TABLE accounting_misa_reviews (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), project_id uuid NOT NULL REFERENCES projects(id),
 request_id uuid NOT NULL REFERENCES accounting_requests(id), fingerprint text NOT NULL,
 note text NOT NULL, actor_id uuid NOT NULL REFERENCES users(id), created_at timestamptz NOT NULL DEFAULT now()
);
CREATE FUNCTION accounting_observation_immutable() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'Accounting source snapshots and review history are immutable'; END $$;
CREATE TRIGGER accounting_basis_immutable BEFORE UPDATE OR DELETE ON accounting_request_bases FOR EACH ROW EXECUTE FUNCTION accounting_observation_immutable();
CREATE TRIGGER accounting_request_event_immutable BEFORE UPDATE OR DELETE ON accounting_request_events FOR EACH ROW EXECUTE FUNCTION accounting_observation_immutable();
CREATE TRIGGER accounting_review_immutable BEFORE UPDATE OR DELETE ON accounting_misa_reviews FOR EACH ROW EXECUTE FUNCTION accounting_observation_immutable();
CREATE FUNCTION accounting_misa_history_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP='DELETE' THEN RAISE EXCEPTION 'Imported MISA history cannot be deleted'; END IF;
 IF TG_TABLE_NAME='accounting_misa_batches' THEN
   IF (to_jsonb(NEW)-'status'-'confirmed_at') IS DISTINCT FROM (to_jsonb(OLD)-'status'-'confirmed_at') OR (OLD.status='confirmed' AND NEW IS DISTINCT FROM OLD) THEN RAISE EXCEPTION 'Imported source file is immutable'; END IF;
 ELSIF TG_TABLE_NAME='accounting_misa_records' THEN
   IF (to_jsonb(NEW)-'active') IS DISTINCT FROM (to_jsonb(OLD)-'active') OR NOT OLD.active OR NEW.active THEN RAISE EXCEPTION 'Only superseding an active MISA observation is allowed'; END IF;
 ELSIF TG_TABLE_NAME='accounting_misa_allocations' THEN
   IF (to_jsonb(NEW)-'active'-'revoked_by'-'revoked_at'-'revoke_reason') IS DISTINCT FROM (to_jsonb(OLD)-'active'-'revoked_by'-'revoked_at'-'revoke_reason') OR NOT OLD.active OR NEW.active THEN RAISE EXCEPTION 'Only revoking an active allocation is allowed'; END IF;
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER accounting_misa_batch_guard BEFORE UPDATE OR DELETE ON accounting_misa_batches FOR EACH ROW EXECUTE FUNCTION accounting_misa_history_guard();
CREATE TRIGGER accounting_misa_record_guard BEFORE UPDATE OR DELETE ON accounting_misa_records FOR EACH ROW EXECUTE FUNCTION accounting_misa_history_guard();
CREATE TRIGGER accounting_misa_allocation_guard BEFORE UPDATE OR DELETE ON accounting_misa_allocations FOR EACH ROW EXECUTE FUNCTION accounting_misa_history_guard();
INSERT INTO permissions(id,key,name,description,module)
VALUES(gen_random_uuid(),'accounting_request.view','Xem đề nghị kế toán','Xem đề nghị và kết quả đối soát theo công trình','accounting_handoff'),
 (gen_random_uuid(),'accounting_request.submit','Lập đề nghị kế toán','Lập và gửi đề nghị theo đợt','accounting_handoff'),
 (gen_random_uuid(),'accounting_request.review','Tiếp nhận đề nghị kế toán','Xác nhận căn cứ, tiếp nhận và trả bổ sung đề nghị','accounting_handoff'),
 (gen_random_uuid(),'accounting_misa.reconcile','Đối soát dữ liệu MISA','Nhập kết quả MISA, phân bổ và xác nhận đối soát','accounting_handoff')
ON CONFLICT(key) DO NOTHING;
INSERT INTO role_permissions(role_id,permission_id)
 SELECT r.id,p.id FROM roles r JOIN permissions p ON p.key IN ('accounting_request.view','accounting_request.submit','accounting_request.review','accounting_misa.reconcile')
 WHERE r.name IN ('admin','developer','accounting') OR (r.name IN ('pm','procurement') AND p.key IN ('accounting_request.view','accounting_request.submit')) OR (r.name IN ('director','qs') AND p.key='accounting_request.view')
ON CONFLICT DO NOTHING;
