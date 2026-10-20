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
SELECT 'sample_user_count=' || count(*) FROM users WHERE is_sample;
SELECT 'non_sample_user_count=' || count(*) FROM users WHERE NOT is_sample;
SELECT 'protected_admin_developer_identity_count=' || count(*) FROM users account
WHERE account.status='active' AND EXISTS (
  SELECT 1 FROM staff person WHERE person.user_id=account.id AND person.status='active'
) AND EXISTS (
  SELECT 1 FROM user_roles assignment JOIN roles role ON role.id=assignment.role_id
  WHERE assignment.user_id=account.id AND assignment.project_id IS NULL
    AND role.system_role AND role.name IN ('admin','developer')
);
SELECT 'required_grant=' || expected.role_name || ':' || expected.permission_key || ':' || count(grant_row.permission_id)
FROM (VALUES
  ('developer','staff.import'),
  ('developer','sample_data.create'),
  ('developer','workflow_definition.import'),
  ('admin','workflow_definition.import')
) AS expected(role_name,permission_key)
LEFT JOIN roles role ON role.name=expected.role_name AND role.system_role
LEFT JOIN permissions permission ON permission.key=expected.permission_key
LEFT JOIN role_permissions grant_row ON grant_row.role_id=role.id AND grant_row.permission_id=permission.id
GROUP BY expected.role_name,expected.permission_key
ORDER BY expected.role_name,expected.permission_key;
SELECT 'pipeline_status_version=' || status || ':' || version || ':' || count(*)
FROM pipeline_templates GROUP BY status,version ORDER BY status,version;
WITH expected(ref) AS (VALUES
  ('builtin_construction_daily_report@v2'),('builtin_owner_client_checklist@v2'),
  ('client_boq_owner_submission@v3'),('client_construction_execution_plan@v1'),
  ('client_construction_handoff_plan@v2'),('client_design_branch_decision@v3'),
  ('client_design_delivery_package@v1'),('client_design_to_qs_handoff@v3'),
  ('client_hardcopy_dispatch@v3'),('client_material_incoming_acceptance_summary@v3'),
  ('client_project_intake_routing@v2'),('client_site_survey_minutes@v3'),
  ('client_subcontractor_acceptance_package@v3'),('client_survey_work_package@v1')
)
SELECT 'local_v10_form_ref=' || expected.ref || ':' ||
  CASE WHEN EXISTS (
    SELECT 1 FROM workflow_form_templates template
    JOIN workflow_form_versions version ON version.template_id=template.id
    WHERE lower(template.code)||'@v'||version.version_no=expected.ref
      AND template.status='active' AND version.status='published'
  ) THEN 'published' ELSE 'missing' END
FROM expected ORDER BY expected.ref;
WITH expected(key) AS (VALUES
  ('acceptance.approve'),('acceptance.create'),('acceptance.view'),
  ('accounting.budget_view'),('accounting_handoff.acknowledge'),('accounting_handoff.view'),
  ('boq.approve'),('boq.update'),('boq.view'),('construction.handoff.manage'),
  ('contract.approve'),('contract.create'),('contract.dispatch'),('contract.review'),
  ('contract.scan_manage'),('contract.sign'),('contract.view'),('handover.approve'),
  ('handover.view'),('operation.issue_create'),('operation.report'),('operation.view'),
  ('pipeline.view'),('planning.edit'),('procurement.approve'),('procurement.create'),
  ('procurement.view'),('project.update'),('project.view'),
  ('project_finance.cost_create'),('project_finance.request_create'),('project_finance.view')
)
SELECT 'local_v10_permission=' || expected.key || ':' ||
  CASE WHEN permission.id IS NOT NULL THEN 'present' ELSE 'missing' END
FROM expected LEFT JOIN permissions permission ON permission.key=expected.key ORDER BY expected.key;
WITH expected(name) AS (VALUES
  ('accounting'),('contract_officer'),('design'),('pm'),
  ('procurement'),('qa_qc'),('qs'),('supervisor')
)
SELECT 'local_v10_role=' || expected.name || ':' ||
  CASE WHEN role.id IS NULL THEN 'missing'
       WHEN role.external_only THEN 'external_only'
       ELSE 'assignable' END
FROM expected LEFT JOIN roles role ON role.name=expected.name ORDER BY expected.name;
SELECT 'project_approval_count=' || count(*) FROM approval_requests WHERE project_id IS NOT NULL;
SELECT 'global_approval_count=' || count(*) FROM approval_requests WHERE project_id IS NULL;
SELECT 'retired_activity_scope_count=' || count(*) FROM work_item_activities
WHERE scope_type IN ('accounting_budget','payable','receivable','payment');
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
  dependency record;
  target_rows bigint;
BEGIN
  FOR dependency IN
    SELECT fk.conrelid::regclass AS child_table, col.attname AS child_column,
           fk.confrelid::regclass AS parent_table, fk.confdeltype AS delete_action
    FROM pg_constraint fk
    JOIN pg_attribute col ON col.attrelid=fk.conrelid AND col.attnum=fk.conkey[1]
    WHERE fk.contype='f' AND cardinality(fk.conkey)=1
      AND fk.confrelid IN ('public.users'::regclass,'public.staff'::regclass)
    ORDER BY parent_table,child_table,child_column
  LOOP
    IF dependency.parent_table='public.users'::regclass THEN
      EXECUTE format($query$
        SELECT count(*) FROM %s child JOIN users target ON child.%I=target.id
        WHERE NOT (target.status='active'
          AND EXISTS (SELECT 1 FROM staff person WHERE person.user_id=target.id AND person.status='active')
          AND EXISTS (SELECT 1 FROM user_roles assignment JOIN roles role ON role.id=assignment.role_id
            WHERE assignment.user_id=target.id AND assignment.project_id IS NULL
              AND role.system_role AND role.name IN ('admin','developer')))
      $query$,dependency.child_table,dependency.child_column) INTO target_rows;
    ELSE
      EXECUTE format($query$
        SELECT count(*) FROM %s child JOIN staff person ON child.%I=person.id
        LEFT JOIN users target ON target.id=person.user_id
        WHERE target.id IS NULL OR NOT (target.status='active'
          AND person.status='active'
          AND EXISTS (SELECT 1 FROM user_roles assignment JOIN roles role ON role.id=assignment.role_id
            WHERE assignment.user_id=target.id AND assignment.project_id IS NULL
              AND role.system_role AND role.name IN ('admin','developer')))
      $query$,dependency.child_table,dependency.child_column) INTO target_rows;
    END IF;
    IF target_rows>0 OR dependency.delete_action<>'c' THEN
      RAISE NOTICE 'identity_reference=%:%:%:action=%:target_rows=%',
        dependency.parent_table,dependency.child_table,dependency.child_column,
        dependency.delete_action,target_rows;
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

  FOR dependency IN
    SELECT constraint_name.conname, constraint_name.conrelid::regclass AS child_table,
           constraint_name.confrelid::regclass AS parent_table,
           constraint_name.confdeltype AS delete_action,
           pg_get_constraintdef(constraint_name.oid) AS definition
    FROM pg_constraint constraint_name
    WHERE constraint_name.contype='f'
      AND constraint_name.confrelid IN (
        'public.approval_requests'::regclass,
        'public.work_items'::regclass
      )
    ORDER BY parent_table,child_table,constraint_name.conname
  LOOP
    EXECUTE format('SELECT count(*) FROM %s',dependency.child_table) INTO row_count;
    IF dependency.delete_action <> 'c' OR row_count > 0 THEN
      RAISE NOTICE 'cleanup_reference=%:%:%:action=%:child_rows=%:definition=%',
        dependency.parent_table,dependency.child_table,dependency.conname,
        dependency.delete_action,row_count,dependency.definition;
    END IF;
  END LOOP;
END $$;
DO $$
DECLARE
  dependency record;
  row_count bigint;
BEGIN
  FOR dependency IN
    WITH RECURSIVE dependent_tables AS (
      SELECT unnest(ARRAY[
        'public.workflow_form_signatures'::regclass,
        'public.workflow_form_pdf_exports'::regclass
      ]) AS child
      UNION
      SELECT constraint_name.conrelid
      FROM dependent_tables chain
      JOIN pg_constraint constraint_name ON constraint_name.contype='f'
        AND constraint_name.confrelid=chain.child
    )
    SELECT DISTINCT child::regclass AS table_name FROM dependent_tables
    ORDER BY table_name
  LOOP
    EXECUTE format('SELECT count(*) FROM %s',dependency.table_name) INTO row_count;
    RAISE NOTICE 'immutable_export_truncate_dependency=%:rows=%',dependency.table_name,row_count;
  END LOOP;
END $$;
COMMIT;
SQL
