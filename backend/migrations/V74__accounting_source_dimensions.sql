-- V73 introduced transitional NOT VALID status constraints before the legacy
-- finance rows were dimensioned. NOT VALID still enforces subsequent UPDATEs,
-- so release only those three transitional constraints before the backfill.
-- V77 installs and validates the complete status sets after all dimension
-- updates have finished.
ALTER TABLE payables
  DROP CONSTRAINT IF EXISTS payables_status_check_v73;

ALTER TABLE receivables
  DROP CONSTRAINT IF EXISTS receivables_status_check_v73;

ALTER TABLE payments
  DROP CONSTRAINT IF EXISTS payments_status_check_v73;

ALTER TABLE accounting_budgets
  ADD COLUMN IF NOT EXISTS accounting_book_id UUID;

UPDATE accounting_budgets b
SET accounting_book_id=p.accounting_book_id
FROM projects p
WHERE p.id=b.project_id AND b.accounting_book_id IS NULL;

ALTER TABLE accounting_budgets
  ALTER COLUMN accounting_book_id SET NOT NULL;

ALTER TABLE accounting_budgets
  ADD CONSTRAINT accounting_budgets_project_book_fk
  FOREIGN KEY (project_id,accounting_book_id) REFERENCES projects(id,accounting_book_id) ON DELETE RESTRICT;

CREATE UNIQUE INDEX IF NOT EXISTS ux_accounting_budgets_id_project_book
  ON accounting_budgets(id,project_id,accounting_book_id);

ALTER TABLE payables
  ADD COLUMN IF NOT EXISTS accounting_book_id UUID,
  ADD COLUMN IF NOT EXISTS budget_id UUID,
  ADD COLUMN IF NOT EXISTS cost_account_id UUID;

UPDATE payables pay
SET accounting_book_id=p.accounting_book_id
FROM projects p
WHERE p.id=pay.project_id AND pay.accounting_book_id IS NULL;

UPDATE payables pay
SET cost_account_id=a.id
FROM accounting_accounts a
WHERE a.book_id=pay.accounting_book_id
  AND a.account_code='154'
  AND pay.cost_account_id IS NULL;

UPDATE payables pay
SET budget_id=matched.id
FROM (
  SELECT project_id,MIN(id::text)::uuid AS id
  FROM accounting_budgets
  WHERE status='approved'
  GROUP BY project_id
  HAVING COUNT(*)=1
) matched
WHERE matched.project_id=pay.project_id AND pay.budget_id IS NULL;

ALTER TABLE payables
  ALTER COLUMN accounting_book_id SET NOT NULL,
  ALTER COLUMN cost_account_id SET NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS ux_payables_id_project_book
  ON payables(id,project_id,accounting_book_id);

ALTER TABLE payables
  ADD CONSTRAINT payables_project_book_fk_v74
  FOREIGN KEY (project_id,accounting_book_id) REFERENCES projects(id,accounting_book_id) ON DELETE RESTRICT;

ALTER TABLE payables
  ADD CONSTRAINT payables_cost_account_book_fk
  FOREIGN KEY (cost_account_id,accounting_book_id) REFERENCES accounting_accounts(id,book_id) ON DELETE RESTRICT;

ALTER TABLE payables
  ADD CONSTRAINT payables_budget_project_book_fk
  FOREIGN KEY (budget_id,project_id,accounting_book_id)
  REFERENCES accounting_budgets(id,project_id,accounting_book_id) ON DELETE RESTRICT;

ALTER TABLE receivables
  ADD COLUMN IF NOT EXISTS accounting_book_id UUID,
  ADD COLUMN IF NOT EXISTS revenue_account_id UUID;

UPDATE receivables rec
SET accounting_book_id=p.accounting_book_id
FROM projects p
WHERE p.id=rec.project_id AND rec.accounting_book_id IS NULL;

UPDATE receivables rec
SET revenue_account_id=a.id
FROM accounting_accounts a
WHERE a.book_id=rec.accounting_book_id
  AND a.account_code='5111'
  AND rec.revenue_account_id IS NULL;

ALTER TABLE receivables
  ALTER COLUMN accounting_book_id SET NOT NULL,
  ALTER COLUMN revenue_account_id SET NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS ux_receivables_id_project_book
  ON receivables(id,project_id,accounting_book_id);

ALTER TABLE receivables
  ADD CONSTRAINT receivables_project_book_fk_v74
  FOREIGN KEY (project_id,accounting_book_id) REFERENCES projects(id,accounting_book_id) ON DELETE RESTRICT;

ALTER TABLE receivables
  ADD CONSTRAINT receivables_revenue_account_book_fk
  FOREIGN KEY (revenue_account_id,accounting_book_id) REFERENCES accounting_accounts(id,book_id) ON DELETE RESTRICT;

ALTER TABLE payments
  ADD COLUMN IF NOT EXISTS accounting_book_id UUID,
  ADD COLUMN IF NOT EXISTS budget_id UUID,
  ADD COLUMN IF NOT EXISTS cash_account_id UUID;

UPDATE payments pay
SET accounting_book_id=p.accounting_book_id
FROM projects p
WHERE p.id=pay.project_id AND pay.accounting_book_id IS NULL;

UPDATE payments pay
SET cash_account_id=a.id
FROM accounting_accounts a
WHERE a.book_id=pay.accounting_book_id
  AND a.account_code='1121'
  AND pay.cash_account_id IS NULL;

UPDATE payments pmt
SET budget_id=pay.budget_id
FROM payables pay
WHERE pmt.target_type='payable'
  AND pmt.target_id=pay.id
  AND pmt.project_id=pay.project_id
  AND pmt.budget_id IS NULL;

ALTER TABLE payments
  ALTER COLUMN accounting_book_id SET NOT NULL,
  ALTER COLUMN cash_account_id SET NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS ux_payments_id_project_book
  ON payments(id,project_id,accounting_book_id);

ALTER TABLE payments
  ADD CONSTRAINT payments_project_book_fk_v74
  FOREIGN KEY (project_id,accounting_book_id) REFERENCES projects(id,accounting_book_id) ON DELETE RESTRICT;

ALTER TABLE payments
  ADD CONSTRAINT payments_cash_account_book_fk
  FOREIGN KEY (cash_account_id,accounting_book_id) REFERENCES accounting_accounts(id,book_id) ON DELETE RESTRICT;

ALTER TABLE payments
  ADD CONSTRAINT payments_budget_project_book_fk
  FOREIGN KEY (budget_id,project_id,accounting_book_id)
  REFERENCES accounting_budgets(id,project_id,accounting_book_id) ON DELETE RESTRICT;

ALTER TABLE input_invoices
  ADD COLUMN IF NOT EXISTS accounting_book_id UUID,
  ADD COLUMN IF NOT EXISTS tax_code_id UUID,
  ADD COLUMN IF NOT EXISTS expense_account_id UUID;

UPDATE input_invoices inv
SET accounting_book_id=p.accounting_book_id
FROM projects p
WHERE p.id=inv.project_id AND inv.accounting_book_id IS NULL;

UPDATE input_invoices inv
SET expense_account_id=pay.cost_account_id
FROM payables pay
WHERE pay.id=inv.payable_id AND pay.project_id=inv.project_id AND inv.expense_account_id IS NULL;

UPDATE input_invoices inv
SET tax_code_id=tax.id
FROM tax_codes tax
WHERE tax.book_id=inv.accounting_book_id
  AND tax.tax_code=CASE
    WHEN inv.vat_amount=0 THEN 'VAT-0'
    WHEN inv.amount_before_vat > 0 AND abs((inv.vat_amount*100/inv.amount_before_vat)-5) <= 0.1 THEN 'VAT-5'
    WHEN inv.amount_before_vat > 0 AND abs((inv.vat_amount*100/inv.amount_before_vat)-8) <= 0.1 THEN 'VAT-8'
    WHEN inv.amount_before_vat > 0 AND abs((inv.vat_amount*100/inv.amount_before_vat)-10) <= 0.1 THEN 'VAT-10'
    ELSE 'VAT-KDK'
  END
  AND inv.tax_code_id IS NULL;

ALTER TABLE input_invoices
  ALTER COLUMN accounting_book_id SET NOT NULL,
  ALTER COLUMN tax_code_id SET NOT NULL,
  ALTER COLUMN expense_account_id SET NOT NULL;

ALTER TABLE input_invoices
  ADD CONSTRAINT input_invoices_project_book_fk_v74
  FOREIGN KEY (project_id,accounting_book_id) REFERENCES projects(id,accounting_book_id) ON DELETE RESTRICT;

ALTER TABLE input_invoices
  ADD CONSTRAINT input_invoices_tax_code_book_fk
  FOREIGN KEY (tax_code_id,accounting_book_id) REFERENCES tax_codes(id,book_id) ON DELETE RESTRICT;

ALTER TABLE input_invoices
  ADD CONSTRAINT input_invoices_expense_account_book_fk
  FOREIGN KEY (expense_account_id,accounting_book_id) REFERENCES accounting_accounts(id,book_id) ON DELETE RESTRICT;

ALTER TABLE output_invoices
  ADD COLUMN IF NOT EXISTS accounting_book_id UUID,
  ADD COLUMN IF NOT EXISTS tax_code_id UUID,
  ADD COLUMN IF NOT EXISTS revenue_account_id UUID;

UPDATE output_invoices inv
SET accounting_book_id=p.accounting_book_id
FROM projects p
WHERE p.id=inv.project_id AND inv.accounting_book_id IS NULL;

UPDATE output_invoices inv
SET revenue_account_id=rec.revenue_account_id
FROM receivables rec
WHERE rec.id=inv.receivable_id AND rec.project_id=inv.project_id AND inv.revenue_account_id IS NULL;

UPDATE output_invoices inv
SET tax_code_id=tax.id
FROM tax_codes tax
WHERE tax.book_id=inv.accounting_book_id
  AND tax.tax_code=CASE
    WHEN inv.vat_amount=0 THEN 'VAT-0'
    WHEN inv.amount_before_vat > 0 AND abs((inv.vat_amount*100/inv.amount_before_vat)-5) <= 0.1 THEN 'VAT-5'
    WHEN inv.amount_before_vat > 0 AND abs((inv.vat_amount*100/inv.amount_before_vat)-8) <= 0.1 THEN 'VAT-8'
    WHEN inv.amount_before_vat > 0 AND abs((inv.vat_amount*100/inv.amount_before_vat)-10) <= 0.1 THEN 'VAT-10'
    ELSE 'VAT-KDK'
  END
  AND inv.tax_code_id IS NULL;

ALTER TABLE output_invoices
  ALTER COLUMN accounting_book_id SET NOT NULL,
  ALTER COLUMN tax_code_id SET NOT NULL,
  ALTER COLUMN revenue_account_id SET NOT NULL;

ALTER TABLE output_invoices
  ADD CONSTRAINT output_invoices_project_book_fk_v74
  FOREIGN KEY (project_id,accounting_book_id) REFERENCES projects(id,accounting_book_id) ON DELETE RESTRICT;

ALTER TABLE output_invoices
  ADD CONSTRAINT output_invoices_tax_code_book_fk
  FOREIGN KEY (tax_code_id,accounting_book_id) REFERENCES tax_codes(id,book_id) ON DELETE RESTRICT;

ALTER TABLE output_invoices
  ADD CONSTRAINT output_invoices_revenue_account_book_fk
  FOREIGN KEY (revenue_account_id,accounting_book_id) REFERENCES accounting_accounts(id,book_id) ON DELETE RESTRICT;

INSERT INTO invoice_tax_lines (
  book_id,project_id,input_invoice_id,line_no,tax_code_id,taxable_amount,vat_amount,
  deductible_amount,nondeductible_amount,description,created_by,created_at,updated_at
)
SELECT
  inv.accounting_book_id,inv.project_id,inv.id,1,inv.tax_code_id,inv.amount_before_vat,inv.vat_amount,
  round(inv.vat_amount*tax.deductible_rate/100,2),
  inv.vat_amount-round(inv.vat_amount*tax.deductible_rate/100,2),
  'Thuế GTGT theo hóa đơn đầu vào',inv.uploaded_by,inv.created_at,inv.updated_at
FROM input_invoices inv
JOIN tax_codes tax ON tax.id=inv.tax_code_id
WHERE inv.status <> 'deleted'
ON CONFLICT DO NOTHING;

INSERT INTO invoice_tax_lines (
  book_id,project_id,output_invoice_id,line_no,tax_code_id,taxable_amount,vat_amount,
  deductible_amount,nondeductible_amount,description,created_by,created_at,updated_at
)
SELECT
  inv.accounting_book_id,inv.project_id,inv.id,1,inv.tax_code_id,inv.amount_before_vat,inv.vat_amount,
  0,inv.vat_amount,'Thuế GTGT theo hóa đơn đầu ra',inv.uploaded_by,inv.created_at,inv.updated_at
FROM output_invoices inv
WHERE inv.status <> 'deleted'
ON CONFLICT DO NOTHING;

CREATE INDEX IF NOT EXISTS idx_payables_budget_status
  ON payables(budget_id,status,created_at DESC)
  WHERE budget_id IS NOT NULL AND status NOT IN ('cancelled','deleted');

CREATE INDEX IF NOT EXISTS idx_payments_budget_status
  ON payments(budget_id,status,paid_at DESC)
  WHERE budget_id IS NOT NULL AND status NOT IN ('cancelled','deleted');
