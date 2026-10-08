-- Preproduction retirement of internal ledger, AP/AR, payments, invoice/VAT
-- processing and provisional financial reports. Current source dossiers,
-- accounting_handoffs, accounting_requests and imported MISA observations stay.
DO $$
DECLARE
  target_table text;
  has_rows boolean;
  nonempty_tables text[] := ARRAY[]::text[];
BEGIN
  -- Lock every table whose business data would lose meaning or be removed.
  -- No API writer can pass between the data guard and schema changes.
  FOREACH target_table IN ARRAY ARRAY[
    'projects','pipeline_runs','sample_data_generations','accounting_budgets',
    'payables','receivables','payments','input_invoices','output_invoices',
    'payment_allocations','receipt_allocations','payable_payment_schedules',
    'receivable_payment_schedules','advance_requests','client_advance_receipts',
    'budget_movements','payment_reversals','journal_entries','journal_entry_lines',
    'invoice_tax_lines','tax_adjustments','vat_return_snapshots','tax_period_events',
    'accounting_backfill_issues','project_material_costs','project_cash_requests',
    'contract_retentions','contract_retention_releases','contract_milestones'
  ] LOOP
    EXECUTE format('LOCK TABLE %I IN ACCESS EXCLUSIVE MODE',target_table);
    EXECUTE format('SELECT EXISTS (SELECT 1 FROM %I)',target_table) INTO has_rows;
    IF has_rows THEN nonempty_tables := array_append(nonempty_tables,target_table); END IF;
  END LOOP;
  IF cardinality(nonempty_tables)>0 THEN
    RAISE EXCEPTION 'Internal accounting retirement requires empty business tables: %',array_to_string(nonempty_tables,', ');
  END IF;
  LOCK TABLE work_item_activities IN ACCESS EXCLUSIVE MODE;
  IF EXISTS (SELECT 1 FROM work_item_activities WHERE scope_type IN ('accounting_budget','payable','receivable','payment')) THEN
    RAISE EXCEPTION 'Internal accounting retirement requires empty retired activity scopes';
  END IF;
  -- Fresh installs contain migration-owned books/accounts/open periods and
  -- tax classification catalogs, but closed periods are business evidence.
  LOCK TABLE accounting_books,accounting_periods,tax_periods IN ACCESS EXCLUSIVE MODE;
  IF EXISTS (SELECT 1 FROM accounting_books WHERE
    id<>'73000000-0000-0000-0000-000000000001'::uuid OR book_code<>'CONSFLOW-TT200') THEN
    RAISE EXCEPTION 'Internal accounting retirement permits only the migration-owned default book';
  END IF;
  IF EXISTS (SELECT 1 FROM accounting_periods WHERE status<>'open')
    OR EXISTS (SELECT 1 FROM tax_periods WHERE status<>'open') THEN
    RAISE EXCEPTION 'Internal accounting retirement cannot discard closed accounting or tax periods';
  END IF;
END $$;

-- Withdraw exact obsolete definitions; published contents remain untouched.
-- Applied SQL remains checksum-verifiable and creates Rev9 before retirement.
UPDATE pipeline_templates SET status='deleted',updated_at=now()
WHERE id IN (
  '40000000-0000-0000-0000-000000000001','40000000-0000-0000-0000-000000000010',
  '40000000-0000-0000-0000-000000000020','40000000-0000-0000-0000-000000000021',
  '40000000-0000-0000-0000-000000000022','40000000-0000-0000-0000-000000000023',
  '40000000-0000-0000-0000-000000000024','40000000-0000-0000-0000-000000000025',
  '40000000-0000-0000-0000-000000000026','40000000-0000-0000-0000-000000000027'
);

-- Explicit leaf-to-owner order, using RESTRICT (default), never CASCADE.
-- Each dropped table owns its indexes/constraints/triggers; standalone
-- functions and the dependent budget view are removed explicitly below.
DROP VIEW accounting_budget_balances;
DROP FUNCTION backfill_accounting_ledger();
DROP TABLE receipt_allocations;
DROP TABLE payment_allocations;
DROP TABLE tax_period_events;
DROP TABLE vat_return_snapshots;
DROP TABLE tax_adjustments;
DROP TABLE payment_reversals;
DROP TABLE budget_movements;
DROP TABLE accounting_backfill_issues;
DROP TABLE journal_entry_lines;
DROP TABLE journal_entries;
DROP TABLE invoice_tax_lines;
DROP TABLE input_invoices;
DROP TABLE output_invoices;
DROP TABLE client_advance_receipts;
DROP TABLE advance_requests;
DROP TABLE payable_payment_schedules;
DROP TABLE receivable_payment_schedules;
DROP TABLE payments;
DROP TABLE tax_periods;
DROP TABLE payables;
DROP TABLE receivables;
DROP TABLE accounting_budgets;
DROP TABLE accounting_posting_rules;
DROP TABLE accounting_periods;
DROP FUNCTION accounting_account_id_by_code(uuid,text);
DROP FUNCTION accounting_tax_code_id_for_amounts(uuid,numeric,numeric,text);
DROP TABLE tax_codes;
DROP TABLE accounting_accounts;

ALTER TABLE sample_data_generations DROP CONSTRAINT sample_data_generations_accounting_book_fk;
ALTER TABLE sample_data_generations DROP COLUMN accounting_book_id;
ALTER TABLE projects DROP CONSTRAINT projects_accounting_book_fk;
DROP INDEX ux_projects_id_accounting_book;
ALTER TABLE projects DROP COLUMN accounting_book_id;
DROP TABLE accounting_books;
DROP TABLE project_cash_requests;
DROP TABLE project_material_costs;
DROP TABLE contract_retention_releases;
DROP TABLE contract_retentions;
ALTER TABLE contract_milestones DROP CONSTRAINT contract_milestones_obligation_type_check;
ALTER TABLE contract_milestones DROP CONSTRAINT contract_milestones_obligation_consistency;
ALTER TABLE contract_milestones DROP COLUMN obligation_type,DROP COLUMN obligation_id;
ALTER TABLE contract_milestones DROP CONSTRAINT contract_milestones_status_check;
ALTER TABLE contract_milestones ADD CONSTRAINT contract_milestones_status_check_v150
  CHECK (status IN ('draft','active','eligible','cancelled'));
ALTER TABLE handover_records DROP COLUMN payables_recorded,DROP COLUMN receivables_recorded;
ALTER TABLE work_item_activities DROP CONSTRAINT work_item_activities_scope_type_check;
ALTER TABLE work_item_activities ADD CONSTRAINT work_item_activities_scope_type_check_v150 CHECK (scope_type IN (
  'project','pipeline_run_node','pipeline_run_step','work_item','boq_item','procurement_request',
  'stock_movement','daily_report','issue','acceptance','rework','rework_request','replanning','replanning_request','handover'
));

DROP FUNCTION set_accounting_budget_defaults();
DROP FUNCTION set_payable_accounting_defaults();
DROP FUNCTION set_receivable_accounting_defaults();
DROP FUNCTION set_payment_accounting_defaults();
DROP FUNCTION set_input_invoice_accounting_defaults();
DROP FUNCTION set_output_invoice_accounting_defaults();
DROP FUNCTION create_invoice_tax_line();
DROP FUNCTION validate_posted_journal_entry();
DROP FUNCTION prevent_posted_journal_entry_mutation();
DROP FUNCTION prevent_posted_journal_line_mutation();
DROP FUNCTION prevent_budget_movement_mutation();
DROP FUNCTION prevent_payment_reversal_mutation();
DROP FUNCTION prevent_tax_adjustment_mutation();
DROP FUNCTION prevent_vat_return_snapshot_mutation();
DROP FUNCTION prevent_tax_period_event_mutation();

DELETE FROM role_permissions WHERE permission_id IN (
  SELECT id FROM permissions WHERE key LIKE 'accounting.%' OR key LIKE 'project_finance.%'
    OR key IN ('report.finance.view','contract.retention_release')
);
DELETE FROM permissions WHERE key LIKE 'accounting.%' OR key LIKE 'project_finance.%'
  OR key IN ('report.finance.view','contract.retention_release');
DELETE FROM users WHERE id='79900000-0000-0000-0000-000000000001'
  AND email='accounting.backfill@system.local' AND status='inactive';
DELETE FROM system_settings WHERE key='client_payment_min_percent';
