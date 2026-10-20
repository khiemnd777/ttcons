#!/usr/bin/env bash
set -Eeuo pipefail

# One-time operator action for the inventoried, unused demo database at V145.
release_dir="$(readlink -f /opt/consflow/current)"
[[ "$release_dir" == /opt/consflow/releases/* ]] || exit 1
release_file="$release_dir/release.env"
test -r "$release_file"
release_tag="$(sed -n 's/^RELEASE_TAG=//p' "$release_file")"
backend_image="$(sed -n 's/^BACKEND_IMAGE=//p' "$release_file")"
admin_image="$(sed -n 's/^ADMIN_IMAGE=//p' "$release_file")"
[[ "$release_tag" == v2026-09-25.1 ]] || { echo 'Unexpected production release; cleanup refused' >&2; exit 1; }
image_pattern='^ghcr\.io/[a-z0-9._/-]+@sha256:[a-f0-9]{64}$'
[[ "$backend_image" =~ $image_pattern && "$admin_image" =~ $image_pattern ]] || exit 1

export RELEASE_TAG="$release_tag" BACKEND_IMAGE="$backend_image" ADMIN_IMAGE="$admin_image"
compose=(docker compose --project-name consflow-prod --env-file /opt/consflow/shared/.env.prod -f "$release_dir/docker-compose.prod.yml")
"${compose[@]}" config --quiet

restart_services() {
  result=$?
  trap - EXIT
  if ! "${compose[@]}" start backend admin; then
    echo 'Could not restart the original production services' >&2
    exit 1
  fi
  exit "$result"
}

# Keep a restricted database archive before the destructive step.
umask 077
backup_dir=/opt/consflow/shared/backups
install -d -m 0700 "$backup_dir"
backup_file="$backup_dir/pre-demo-cleanup-v145-$(date -u +%Y%m%dT%H%M%SZ).dump"
echo 'backup_archive=starting'
"${compose[@]}" exec -T postgres sh -ec \
  'exec pg_dump -Fc -U "$POSTGRES_USER" -d "$POSTGRES_DB"' < /dev/null > "$backup_file"
echo 'backup_archive=dump_written'
test -s "$backup_file"
"${compose[@]}" exec -T postgres pg_restore --list < "$backup_file" > /dev/null
echo "backup_archive=catalog_readable:$backup_file"

trap restart_services EXIT
"${compose[@]}" stop -t 30 admin backend

"${compose[@]}" exec -T postgres sh -ec \
  'exec psql -X -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB"' <<'SQL'
BEGIN;
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '120s';
LOCK TABLE projects, approval_requests, sample_data_generations,
  workflow_form_signatures, workflow_form_pdf_exports IN ACCESS EXCLUSIVE MODE;

DO $$
DECLARE
  actual bigint;
  dependency record;
  guarded_table text;
BEGIN
  SELECT count(*) INTO actual FROM schema_migrations;
  IF actual<>145 OR (SELECT max(version::integer) FROM schema_migrations)<>145 THEN
    RAISE EXCEPTION 'Expected exactly 145 applied migrations';
  END IF;
  SELECT count(*) INTO actual FROM projects;
  IF actual<>3 THEN RAISE EXCEPTION 'Project count changed'; END IF;
  SELECT count(*) INTO actual FROM approval_requests WHERE project_id IS NOT NULL;
  IF actual<>9 OR EXISTS (SELECT 1 FROM approval_requests WHERE project_id IS NULL) THEN
    RAISE EXCEPTION 'Approval request inventory changed';
  END IF;
  SELECT count(*) INTO actual FROM sample_data_generations;
  IF actual<>2 THEN RAISE EXCEPTION 'Sample generation count changed'; END IF;
  SELECT count(*) INTO actual FROM workflow_form_signatures;
  IF actual<>10 THEN RAISE EXCEPTION 'Signature count changed'; END IF;
  SELECT count(*) INTO actual FROM workflow_form_pdf_exports;
  IF actual<>9 THEN RAISE EXCEPTION 'PDF export count changed'; END IF;
  SELECT count(*) INTO actual FROM users;
  IF actual<>24 THEN RAISE EXCEPTION 'User inventory changed'; END IF;
  SELECT count(*) INTO actual FROM users WHERE is_sample;
  IF actual<>8 THEN RAISE EXCEPTION 'Sample user inventory changed'; END IF;

  -- TRUNCATE bypasses immutable row-delete triggers for these two demo artifact
  -- tables. Its FK closure must contain no other data before it can run.
  FOR dependency IN
    WITH RECURSIVE closure AS (
      SELECT unnest(ARRAY[
        'public.workflow_form_signatures'::regclass,
        'public.workflow_form_pdf_exports'::regclass
      ]) AS child
      UNION
      SELECT fk.conrelid FROM closure JOIN pg_constraint fk
        ON fk.contype='f' AND fk.confrelid=closure.child
    )
    SELECT DISTINCT child::regclass AS table_name FROM closure
  LOOP
    EXECUTE format('LOCK TABLE %s IN ACCESS EXCLUSIVE MODE',dependency.table_name);
    IF dependency.table_name NOT IN (
      'public.workflow_form_signatures'::regclass,
      'public.workflow_form_pdf_exports'::regclass
    ) THEN
      EXECUTE format('SELECT count(*) FROM %s',dependency.table_name) INTO actual;
      IF actual<>0 THEN
        RAISE EXCEPTION 'Immutable artifact dependency % contains % rows',dependency.table_name,actual;
      END IF;
    END IF;
  END LOOP;

  FOREACH guarded_table IN ARRAY ARRAY[
    'accounting_budgets','payables','receivables','payments','input_invoices',
    'output_invoices','payment_allocations','receipt_allocations',
    'payable_payment_schedules','receivable_payment_schedules','advance_requests',
    'client_advance_receipts','budget_movements','payment_reversals',
    'journal_entries','journal_entry_lines','invoice_tax_lines','tax_adjustments',
    'vat_return_snapshots','tax_period_events','accounting_backfill_issues',
    'project_material_costs','project_cash_requests','contract_retentions',
    'contract_retention_releases','contract_milestones',
    'accounting_request_bases','accounting_requests','accounting_request_events',
    'accounting_misa_batches','accounting_misa_records','accounting_misa_allocations',
    'accounting_misa_reviews'
  ] LOOP
    EXECUTE format('SELECT count(*) FROM public.%I',guarded_table) INTO actual;
    IF actual<>0 THEN RAISE EXCEPTION 'Guarded table % is nonempty',guarded_table; END IF;
  END LOOP;
  IF EXISTS (SELECT 1 FROM work_item_activities WHERE scope_type IN
      ('accounting_budget','payable','receivable','payment'))
    OR EXISTS (SELECT 1 FROM accounting_books WHERE
      id<>'73000000-0000-0000-0000-000000000001'::uuid OR book_code<>'CONSFLOW-TT200')
    OR EXISTS (SELECT 1 FROM accounting_periods WHERE status<>'open')
    OR EXISTS (SELECT 1 FROM tax_periods WHERE status<>'open') THEN
    RAISE EXCEPTION 'V150 reference-data guard is not satisfied';
  END IF;
END $$;

TRUNCATE workflow_form_signatures, workflow_form_pdf_exports CASCADE;
DELETE FROM approval_requests WHERE project_id IS NOT NULL;
DELETE FROM sample_data_generations;
DELETE FROM projects;

DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM projects)
    OR EXISTS (SELECT 1 FROM pipeline_runs)
    OR EXISTS (SELECT 1 FROM sample_data_generations)
    OR EXISTS (SELECT 1 FROM workflow_form_signatures)
    OR EXISTS (SELECT 1 FROM workflow_form_pdf_exports)
    OR EXISTS (SELECT 1 FROM approval_requests) THEN
    RAISE EXCEPTION 'Pre-migration demo cleanup is incomplete';
  END IF;
  IF (SELECT count(*) FROM users)<>24
    OR (SELECT count(*) FROM users WHERE is_sample)<>8 THEN
    RAISE EXCEPTION 'Cleanup changed user identities';
  END IF;
  IF (SELECT count(DISTINCT account.id) FROM users account
      JOIN staff person ON person.user_id=account.id AND person.status='active'
      JOIN user_roles assignment ON assignment.user_id=account.id AND assignment.project_id IS NULL
      JOIN roles role ON role.id=assignment.role_id AND role.name='admin' AND role.system_role
      WHERE account.status='active')<2
    OR (SELECT count(DISTINCT account.id) FROM users account
      JOIN staff person ON person.user_id=account.id AND person.status='active'
      JOIN user_roles assignment ON assignment.user_id=account.id AND assignment.project_id IS NULL
      JOIN roles role ON role.id=assignment.role_id AND role.name='developer' AND role.system_role
      WHERE account.status='active')<1 THEN
    RAISE EXCEPTION 'Protected Admin or Developer access changed';
  END IF;
END $$;
COMMIT;
SQL

echo 'pre_migration_demo_cleanup=committed'
