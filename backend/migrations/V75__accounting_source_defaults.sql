CREATE OR REPLACE FUNCTION accounting_account_id_by_code(target_book_id UUID,target_code TEXT)
RETURNS UUID LANGUAGE sql STABLE AS $$
  SELECT id
  FROM accounting_accounts
  WHERE book_id=target_book_id AND account_code=target_code AND active AND is_postable
  LIMIT 1
$$;

CREATE OR REPLACE FUNCTION accounting_tax_code_id_for_amounts(
  target_book_id UUID,
  taxable_amount NUMERIC,
  vat_amount NUMERIC,
  target_direction TEXT
)
RETURNS UUID LANGUAGE sql STABLE AS $$
  SELECT id
  FROM tax_codes
  WHERE book_id=target_book_id
    AND active
    AND direction IN (target_direction,'both')
    AND effective_from <= CURRENT_DATE
    AND (effective_to IS NULL OR effective_to >= CURRENT_DATE)
  ORDER BY
    CASE
      WHEN vat_amount=0 AND vat_rate=0 THEN 0
      WHEN taxable_amount > 0 AND abs((vat_amount*100/taxable_amount)-vat_rate) <= 0.1 THEN 0
      ELSE 1
    END,
    CASE WHEN tax_code='VAT-KDK' THEN 1 ELSE 0 END,
    tax_code
  LIMIT 1
$$;

CREATE OR REPLACE FUNCTION set_accounting_budget_defaults()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF NEW.accounting_book_id IS NULL THEN
    SELECT accounting_book_id INTO NEW.accounting_book_id
    FROM projects WHERE id=NEW.project_id;
  END IF;
  RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS trg_set_accounting_budget_defaults ON accounting_budgets;
CREATE TRIGGER trg_set_accounting_budget_defaults
BEFORE INSERT ON accounting_budgets
FOR EACH ROW EXECUTE FUNCTION set_accounting_budget_defaults();

CREATE OR REPLACE FUNCTION set_payable_accounting_defaults()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF NEW.accounting_book_id IS NULL THEN
    SELECT accounting_book_id INTO NEW.accounting_book_id
    FROM projects WHERE id=NEW.project_id;
  END IF;
  IF NEW.cost_account_id IS NULL THEN
    NEW.cost_account_id=accounting_account_id_by_code(NEW.accounting_book_id,'154');
  END IF;
  RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS trg_set_payable_accounting_defaults ON payables;
CREATE TRIGGER trg_set_payable_accounting_defaults
BEFORE INSERT ON payables
FOR EACH ROW EXECUTE FUNCTION set_payable_accounting_defaults();

CREATE OR REPLACE FUNCTION set_receivable_accounting_defaults()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF NEW.accounting_book_id IS NULL THEN
    SELECT accounting_book_id INTO NEW.accounting_book_id
    FROM projects WHERE id=NEW.project_id;
  END IF;
  IF NEW.revenue_account_id IS NULL THEN
    NEW.revenue_account_id=accounting_account_id_by_code(NEW.accounting_book_id,'5111');
  END IF;
  RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS trg_set_receivable_accounting_defaults ON receivables;
CREATE TRIGGER trg_set_receivable_accounting_defaults
BEFORE INSERT ON receivables
FOR EACH ROW EXECUTE FUNCTION set_receivable_accounting_defaults();

CREATE OR REPLACE FUNCTION set_payment_accounting_defaults()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF NEW.accounting_book_id IS NULL THEN
    SELECT accounting_book_id INTO NEW.accounting_book_id
    FROM projects WHERE id=NEW.project_id;
  END IF;
  IF NEW.cash_account_id IS NULL THEN
    NEW.cash_account_id=accounting_account_id_by_code(
      NEW.accounting_book_id,
      CASE WHEN NEW.payment_method='cash' THEN '1111' ELSE '1121' END
    );
  END IF;
  IF NEW.budget_id IS NULL AND NEW.target_type='payable' AND NEW.target_id IS NOT NULL THEN
    SELECT budget_id INTO NEW.budget_id
    FROM payables
    WHERE id=NEW.target_id AND project_id=NEW.project_id;
  END IF;
  RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS trg_set_payment_accounting_defaults ON payments;
CREATE TRIGGER trg_set_payment_accounting_defaults
BEFORE INSERT ON payments
FOR EACH ROW EXECUTE FUNCTION set_payment_accounting_defaults();

CREATE OR REPLACE FUNCTION set_input_invoice_accounting_defaults()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  SELECT
    COALESCE(NEW.accounting_book_id,pay.accounting_book_id),
    COALESCE(NEW.expense_account_id,pay.cost_account_id)
  INTO NEW.accounting_book_id,NEW.expense_account_id
  FROM payables pay
  WHERE pay.id=NEW.payable_id AND pay.project_id=NEW.project_id;

  IF NEW.tax_code_id IS NULL THEN
    NEW.tax_code_id=accounting_tax_code_id_for_amounts(
      NEW.accounting_book_id,NEW.amount_before_vat,NEW.vat_amount,'input'
    );
  END IF;
  RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS trg_set_input_invoice_accounting_defaults ON input_invoices;
CREATE TRIGGER trg_set_input_invoice_accounting_defaults
BEFORE INSERT ON input_invoices
FOR EACH ROW EXECUTE FUNCTION set_input_invoice_accounting_defaults();

CREATE OR REPLACE FUNCTION set_output_invoice_accounting_defaults()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  SELECT
    COALESCE(NEW.accounting_book_id,rec.accounting_book_id),
    COALESCE(NEW.revenue_account_id,rec.revenue_account_id)
  INTO NEW.accounting_book_id,NEW.revenue_account_id
  FROM receivables rec
  WHERE rec.id=NEW.receivable_id AND rec.project_id=NEW.project_id;

  IF NEW.tax_code_id IS NULL THEN
    NEW.tax_code_id=accounting_tax_code_id_for_amounts(
      NEW.accounting_book_id,NEW.amount_before_vat,NEW.vat_amount,'output'
    );
  END IF;
  RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS trg_set_output_invoice_accounting_defaults ON output_invoices;
CREATE TRIGGER trg_set_output_invoice_accounting_defaults
BEFORE INSERT ON output_invoices
FOR EACH ROW EXECUTE FUNCTION set_output_invoice_accounting_defaults();

CREATE OR REPLACE FUNCTION create_invoice_tax_line()
RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
  deductible_rate_value NUMERIC(7,4);
BEGIN
  SELECT deductible_rate INTO deductible_rate_value
  FROM tax_codes WHERE id=NEW.tax_code_id AND book_id=NEW.accounting_book_id;

  IF TG_TABLE_NAME='input_invoices' THEN
    INSERT INTO invoice_tax_lines (
      book_id,project_id,input_invoice_id,line_no,tax_code_id,taxable_amount,vat_amount,
      deductible_amount,nondeductible_amount,description,created_by
    ) VALUES (
      NEW.accounting_book_id,NEW.project_id,NEW.id,1,NEW.tax_code_id,NEW.amount_before_vat,NEW.vat_amount,
      round(NEW.vat_amount*deductible_rate_value/100,2),
      NEW.vat_amount-round(NEW.vat_amount*deductible_rate_value/100,2),
      'Thuế GTGT theo hóa đơn đầu vào',NEW.uploaded_by
    ) ON CONFLICT DO NOTHING;
  ELSE
    INSERT INTO invoice_tax_lines (
      book_id,project_id,output_invoice_id,line_no,tax_code_id,taxable_amount,vat_amount,
      deductible_amount,nondeductible_amount,description,created_by
    ) VALUES (
      NEW.accounting_book_id,NEW.project_id,NEW.id,1,NEW.tax_code_id,NEW.amount_before_vat,NEW.vat_amount,
      0,NEW.vat_amount,'Thuế GTGT theo hóa đơn đầu ra',NEW.uploaded_by
    ) ON CONFLICT DO NOTHING;
  END IF;
  RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS trg_create_input_invoice_tax_line ON input_invoices;
CREATE TRIGGER trg_create_input_invoice_tax_line
AFTER INSERT ON input_invoices
FOR EACH ROW EXECUTE FUNCTION create_invoice_tax_line();

DROP TRIGGER IF EXISTS trg_create_output_invoice_tax_line ON output_invoices;
CREATE TRIGGER trg_create_output_invoice_tax_line
AFTER INSERT ON output_invoices
FOR EACH ROW EXECUTE FUNCTION create_invoice_tax_line();
