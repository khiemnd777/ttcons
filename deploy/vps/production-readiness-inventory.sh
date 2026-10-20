#!/usr/bin/env bash
set -Eeuo pipefail

release_dir="$(readlink -f /opt/consflow/current)"
[[ "$release_dir" == /opt/consflow/releases/* ]] || {
  echo 'Current production release is not a managed release directory' >&2
  exit 1
}
release_file="$release_dir/release.env"
test -r "$release_file"
release_tag="$(sed -n 's/^RELEASE_TAG=//p' "$release_file")"
backend_image="$(sed -n 's/^BACKEND_IMAGE=//p' "$release_file")"
admin_image="$(sed -n 's/^ADMIN_IMAGE=//p' "$release_file")"
[[ "$release_tag" =~ ^v[0-9]{4}-[0-9]{2}-[0-9]{2}\.[1-9][0-9]*$ ]] || exit 1
image_pattern='^ghcr\.io/[a-z0-9._/-]+@sha256:[a-f0-9]{64}$'
[[ "$backend_image" =~ $image_pattern && "$admin_image" =~ $image_pattern ]] || exit 1

export RELEASE_TAG="$release_tag" BACKEND_IMAGE="$backend_image" ADMIN_IMAGE="$admin_image"
compose=(docker compose --project-name consflow-prod --env-file /opt/consflow/shared/.env.prod -f "$release_dir/docker-compose.prod.yml")
printf 'release_tag=%s\n' "$release_tag"

# Queries return counts and migration fingerprints only. They never export records or credentials.
"${compose[@]}" exec -T postgres sh -ec 'exec psql -X -A -t -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB"' <<'SQL'
BEGIN TRANSACTION READ ONLY;
SET LOCAL statement_timeout = '60s';
SET LOCAL lock_timeout = '2s';
SELECT 'database_bytes=' || pg_database_size(current_database());
SELECT 'migration_count=' || count(*) FROM schema_migrations;
SELECT 'latest_migration=' || COALESCE(max(version::integer),0) FROM schema_migrations;
SELECT 'migration=' || version || ':' || checksum_sha256 FROM schema_migrations ORDER BY version::integer;
SELECT 'active_admin_count=' || count(DISTINCT account.id)
FROM users account
JOIN staff person ON person.user_id=account.id AND person.status='active'
JOIN user_roles assignment ON assignment.user_id=account.id AND assignment.project_id IS NULL
JOIN roles role ON role.id=assignment.role_id AND role.name='admin' AND role.system_role
WHERE account.status='active';
SELECT 'active_developer_count=' || count(DISTINCT account.id)
FROM users account
JOIN staff person ON person.user_id=account.id AND person.status='active'
JOIN user_roles assignment ON assignment.user_id=account.id AND assignment.project_id IS NULL
JOIN roles role ON role.id=assignment.role_id AND role.name='developer' AND role.system_role
WHERE account.status='active';
DO $$
DECLARE
  table_name text;
  row_count bigint;
BEGIN
  FOREACH table_name IN ARRAY ARRAY[
    'users','staff','projects','project_members','pipeline_templates','pipeline_runs',
    'workflow_form_templates','workflow_form_versions','workflow_form_instances',
    'work_items','work_item_activities','activity_mentions','activity_read_states',
    'activity_message_revisions','activity_reactions','project_discussion_read_cursors',
    'audit_logs','auth_sessions','auth_email_tokens','evidence_records','documents',
    'user_notifications','notification_events','notification_outbox',
    'notification_delivery_attempts','notification_preferences',
    'sample_data_generations','event_logs','staff_account_imports',
    'workflow_definition_imports','bulk_deletion_batches','staff_deletion_batches',
    'accounting_budgets','payables','receivables','payments','input_invoices',
    'output_invoices','payment_allocations','receipt_allocations',
    'payable_payment_schedules','receivable_payment_schedules','advance_requests',
    'client_advance_receipts','budget_movements','payment_reversals',
    'journal_entries','journal_entry_lines','invoice_tax_lines','tax_adjustments',
    'vat_return_snapshots','tax_period_events','accounting_backfill_issues',
    'project_material_costs','project_cash_requests','contract_retentions',
    'contract_retention_releases','contract_milestones','accounting_request_bases',
    'accounting_requests','accounting_request_events','accounting_misa_batches',
    'accounting_misa_records','accounting_misa_allocations','accounting_misa_reviews'
  ] LOOP
    IF to_regclass(format('public.%I',table_name)) IS NULL THEN
      RAISE NOTICE 'table_missing=%',table_name;
    ELSE
      EXECUTE format('SELECT count(*) FROM public.%I',table_name) INTO row_count;
      RAISE NOTICE 'table_count=%:%',table_name,row_count;
    END IF;
  END LOOP;
END $$;
DO $$
DECLARE
  table_name text;
  row_count bigint;
  dependency record;
BEGIN
  FOR table_name IN
    SELECT tablename FROM pg_tables WHERE schemaname='public' ORDER BY tablename
  LOOP
    EXECUTE format('SELECT count(*) FROM public.%I',table_name) INTO row_count;
    IF row_count > 0 THEN
      RAISE NOTICE 'nonempty_table=%:%',table_name,row_count;
    END IF;
  END LOOP;

  FOR dependency IN
    SELECT constraint_name.conname, constraint_name.conrelid::regclass AS child_table,
           constraint_name.confdeltype AS delete_action,
           pg_get_constraintdef(constraint_name.oid) AS definition
    FROM pg_constraint constraint_name
    WHERE constraint_name.contype='f'
      AND constraint_name.confrelid='public.projects'::regclass
    ORDER BY constraint_name.conrelid::regclass::text,constraint_name.conname
  LOOP
    EXECUTE format('SELECT count(*) FROM %s',dependency.child_table) INTO row_count;
    IF dependency.delete_action <> 'c' OR row_count > 0 THEN
      RAISE NOTICE 'project_reference=%:%:action=%:child_rows=%:definition=%',
        dependency.child_table,dependency.conname,dependency.delete_action,
        row_count,dependency.definition;
    END IF;
  END LOOP;
END $$;
COMMIT;
SQL
