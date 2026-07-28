CREATE TABLE IF NOT EXISTS accounting_books (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  book_code TEXT NOT NULL UNIQUE,
  name TEXT NOT NULL,
  base_currency TEXT NOT NULL DEFAULT 'VND',
  accounting_regime TEXT NOT NULL DEFAULT 'TT200',
  fiscal_year_start_month SMALLINT NOT NULL DEFAULT 1,
  active BOOLEAN NOT NULL DEFAULT true,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT accounting_books_code_nonblank CHECK (NULLIF(btrim(book_code),'') IS NOT NULL),
  CONSTRAINT accounting_books_name_nonblank CHECK (NULLIF(btrim(name),'') IS NOT NULL),
  CONSTRAINT accounting_books_currency_check CHECK (base_currency='VND'),
  CONSTRAINT accounting_books_regime_check CHECK (accounting_regime IN ('TT200','TT133')),
  CONSTRAINT accounting_books_year_start_check CHECK (fiscal_year_start_month BETWEEN 1 AND 12)
);

INSERT INTO accounting_books (id,book_code,name,base_currency,accounting_regime,fiscal_year_start_month)
VALUES ('73000000-0000-0000-0000-000000000001','CONSFLOW-TT200','Sổ kế toán Thủ Thiêm','VND','TT200',1)
ON CONFLICT (book_code) DO NOTHING;

ALTER TABLE projects
  ADD COLUMN IF NOT EXISTS accounting_book_id UUID;

UPDATE projects
SET accounting_book_id='73000000-0000-0000-0000-000000000001'::uuid
WHERE accounting_book_id IS NULL;

ALTER TABLE projects
  ALTER COLUMN accounting_book_id SET DEFAULT '73000000-0000-0000-0000-000000000001'::uuid,
  ALTER COLUMN accounting_book_id SET NOT NULL;

ALTER TABLE projects
  DROP CONSTRAINT IF EXISTS projects_accounting_book_fk;

ALTER TABLE projects
  ADD CONSTRAINT projects_accounting_book_fk
  FOREIGN KEY (accounting_book_id) REFERENCES accounting_books(id) ON DELETE RESTRICT;

CREATE UNIQUE INDEX IF NOT EXISTS ux_projects_id_accounting_book
  ON projects(id,accounting_book_id);

CREATE TABLE IF NOT EXISTS accounting_accounts (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  book_id UUID NOT NULL REFERENCES accounting_books(id) ON DELETE RESTRICT,
  account_code TEXT NOT NULL,
  name TEXT NOT NULL,
  account_type TEXT NOT NULL,
  normal_balance TEXT NOT NULL,
  parent_id UUID REFERENCES accounting_accounts(id) ON DELETE RESTRICT,
  is_postable BOOLEAN NOT NULL DEFAULT true,
  active BOOLEAN NOT NULL DEFAULT true,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT accounting_accounts_book_code_unique UNIQUE (book_id,account_code),
  CONSTRAINT accounting_accounts_code_nonblank CHECK (NULLIF(btrim(account_code),'') IS NOT NULL),
  CONSTRAINT accounting_accounts_name_nonblank CHECK (NULLIF(btrim(name),'') IS NOT NULL),
  CONSTRAINT accounting_accounts_type_check CHECK (account_type IN ('asset','liability','equity','revenue','expense','contra')),
  CONSTRAINT accounting_accounts_normal_balance_check CHECK (normal_balance IN ('debit','credit'))
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_accounting_accounts_id_book
  ON accounting_accounts(id,book_id);

CREATE INDEX IF NOT EXISTS idx_accounting_accounts_book_active
  ON accounting_accounts(book_id,active,account_code);

INSERT INTO accounting_accounts (id,book_id,account_code,name,account_type,normal_balance,is_postable)
VALUES
  ('73100000-0000-0000-0000-000000001111','73000000-0000-0000-0000-000000000001','1111','Tiền Việt Nam tại quỹ','asset','debit',true),
  ('73100000-0000-0000-0000-000000001121','73000000-0000-0000-0000-000000000001','1121','Tiền Việt Nam gửi ngân hàng','asset','debit',true),
  ('73100000-0000-0000-0000-000000000131','73000000-0000-0000-0000-000000000001','131','Phải thu của khách hàng','asset','debit',true),
  ('73100000-0000-0000-0000-000000001331','73000000-0000-0000-0000-000000000001','1331','Thuế GTGT được khấu trừ của hàng hóa, dịch vụ','asset','debit',true),
  ('73100000-0000-0000-0000-000000000141','73000000-0000-0000-0000-000000000001','141','Tạm ứng','asset','debit',true),
  ('73100000-0000-0000-0000-000000000154','73000000-0000-0000-0000-000000000001','154','Chi phí sản xuất, kinh doanh dở dang','asset','debit',true),
  ('73100000-0000-0000-0000-000000000331','73000000-0000-0000-0000-000000000001','331','Phải trả cho người bán','liability','credit',true),
  ('73100000-0000-0000-0000-000000003331','73000000-0000-0000-0000-000000000001','3331','Thuế GTGT phải nộp','liability','credit',true),
  ('73100000-0000-0000-0000-000000000411','73000000-0000-0000-0000-000000000001','411','Vốn đầu tư của chủ sở hữu','equity','credit',true),
  ('73100000-0000-0000-0000-000000005111','73000000-0000-0000-0000-000000000001','5111','Doanh thu bán hàng hóa và cung cấp dịch vụ','revenue','credit',true),
  ('73100000-0000-0000-0000-000000000621','73000000-0000-0000-0000-000000000001','621','Chi phí nguyên liệu, vật liệu trực tiếp','expense','debit',true),
  ('73100000-0000-0000-0000-000000000622','73000000-0000-0000-0000-000000000001','622','Chi phí nhân công trực tiếp','expense','debit',true),
  ('73100000-0000-0000-0000-000000000623','73000000-0000-0000-0000-000000000001','623','Chi phí sử dụng máy thi công','expense','debit',true),
  ('73100000-0000-0000-0000-000000000627','73000000-0000-0000-0000-000000000001','627','Chi phí sản xuất chung','expense','debit',true),
  ('73100000-0000-0000-0000-000000000632','73000000-0000-0000-0000-000000000001','632','Giá vốn hàng bán','expense','debit',true),
  ('73100000-0000-0000-0000-000000000642','73000000-0000-0000-0000-000000000001','642','Chi phí quản lý doanh nghiệp','expense','debit',true),
  ('73100000-0000-0000-0000-000000000711','73000000-0000-0000-0000-000000000001','711','Thu nhập khác','revenue','credit',true),
  ('73100000-0000-0000-0000-000000000811','73000000-0000-0000-0000-000000000001','811','Chi phí khác','expense','debit',true),
  ('73100000-0000-0000-0000-000000000911','73000000-0000-0000-0000-000000000001','911','Xác định kết quả kinh doanh','equity','credit',true)
ON CONFLICT (book_id,account_code) DO NOTHING;

CREATE TABLE IF NOT EXISTS accounting_periods (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  book_id UUID NOT NULL REFERENCES accounting_books(id) ON DELETE RESTRICT,
  fiscal_year INTEGER NOT NULL,
  period_no SMALLINT NOT NULL,
  date_from DATE NOT NULL,
  date_to DATE NOT NULL,
  status TEXT NOT NULL DEFAULT 'open',
  closed_by UUID REFERENCES users(id),
  closed_at TIMESTAMPTZ,
  close_reason TEXT,
  reopened_by UUID REFERENCES users(id),
  reopened_at TIMESTAMPTZ,
  reopen_reason TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT accounting_periods_book_period_unique UNIQUE (book_id,fiscal_year,period_no),
  CONSTRAINT accounting_periods_year_check CHECK (fiscal_year BETWEEN 2000 AND 2200),
  CONSTRAINT accounting_periods_number_check CHECK (period_no BETWEEN 1 AND 12),
  CONSTRAINT accounting_periods_date_check CHECK (date_to >= date_from),
  CONSTRAINT accounting_periods_status_check CHECK (status IN ('open','closed')),
  CONSTRAINT accounting_periods_close_actor_check CHECK (status <> 'closed' OR (closed_by IS NOT NULL AND closed_at IS NOT NULL))
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_accounting_periods_id_book
  ON accounting_periods(id,book_id);

CREATE INDEX IF NOT EXISTS idx_accounting_periods_book_dates
  ON accounting_periods(book_id,date_from,date_to,status);

INSERT INTO accounting_periods (book_id,fiscal_year,period_no,date_from,date_to,status)
SELECT
  '73000000-0000-0000-0000-000000000001'::uuid,
  EXTRACT(YEAR FROM CURRENT_DATE)::integer,
  month_no,
  make_date(EXTRACT(YEAR FROM CURRENT_DATE)::integer,month_no,1),
  (make_date(EXTRACT(YEAR FROM CURRENT_DATE)::integer,month_no,1) + INTERVAL '1 month - 1 day')::date,
  'open'
FROM generate_series(1,12) AS month_no
ON CONFLICT (book_id,fiscal_year,period_no) DO NOTHING;

CREATE TABLE IF NOT EXISTS journal_entries (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  book_id UUID NOT NULL REFERENCES accounting_books(id) ON DELETE RESTRICT,
  project_id UUID NOT NULL,
  period_id UUID NOT NULL,
  entry_no TEXT NOT NULL,
  entry_date DATE NOT NULL,
  document_date DATE,
  description TEXT NOT NULL,
  source_type TEXT NOT NULL,
  source_id UUID NOT NULL,
  source_event TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'draft',
  reversal_of_id UUID REFERENCES journal_entries(id) ON DELETE RESTRICT,
  created_by UUID REFERENCES users(id),
  posted_by UUID REFERENCES users(id),
  posted_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT journal_entries_project_book_fk
    FOREIGN KEY (project_id,book_id) REFERENCES projects(id,accounting_book_id) ON DELETE RESTRICT,
  CONSTRAINT journal_entries_period_book_fk
    FOREIGN KEY (period_id,book_id) REFERENCES accounting_periods(id,book_id) ON DELETE RESTRICT,
  CONSTRAINT journal_entries_book_number_unique UNIQUE (book_id,entry_no),
  CONSTRAINT journal_entries_status_check CHECK (status IN ('draft','posted')),
  CONSTRAINT journal_entries_description_nonblank CHECK (NULLIF(btrim(description),'') IS NOT NULL),
  CONSTRAINT journal_entries_source_nonblank CHECK (NULLIF(btrim(source_type),'') IS NOT NULL AND NULLIF(btrim(source_event),'') IS NOT NULL),
  CONSTRAINT journal_entries_posted_actor_check CHECK (status <> 'posted' OR (posted_by IS NOT NULL AND posted_at IS NOT NULL)),
  CONSTRAINT journal_entries_reversal_self_check CHECK (reversal_of_id IS NULL OR reversal_of_id <> id)
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_journal_entries_id_book
  ON journal_entries(id,book_id);

CREATE UNIQUE INDEX IF NOT EXISTS ux_journal_entries_source_event
  ON journal_entries(book_id,source_type,source_id,source_event)
  WHERE status='posted';

CREATE UNIQUE INDEX IF NOT EXISTS ux_journal_entries_single_reversal
  ON journal_entries(reversal_of_id)
  WHERE reversal_of_id IS NOT NULL AND status='posted';

CREATE INDEX IF NOT EXISTS idx_journal_entries_project_date
  ON journal_entries(project_id,entry_date DESC,entry_no)
  WHERE status='posted';

CREATE INDEX IF NOT EXISTS idx_journal_entries_period
  ON journal_entries(period_id,status,entry_date,entry_no);

CREATE TABLE IF NOT EXISTS journal_entry_lines (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  entry_id UUID NOT NULL,
  book_id UUID NOT NULL,
  line_no INTEGER NOT NULL,
  account_id UUID NOT NULL,
  partner_id UUID REFERENCES partners(id) ON DELETE RESTRICT,
  cost_code TEXT,
  description TEXT,
  debit_amount NUMERIC(18,2) NOT NULL DEFAULT 0,
  credit_amount NUMERIC(18,2) NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT journal_entry_lines_entry_book_fk
    FOREIGN KEY (entry_id,book_id) REFERENCES journal_entries(id,book_id) ON DELETE CASCADE,
  CONSTRAINT journal_entry_lines_account_book_fk
    FOREIGN KEY (account_id,book_id) REFERENCES accounting_accounts(id,book_id) ON DELETE RESTRICT,
  CONSTRAINT journal_entry_lines_number_unique UNIQUE (entry_id,line_no),
  CONSTRAINT journal_entry_lines_number_positive CHECK (line_no > 0),
  CONSTRAINT journal_entry_lines_amount_check CHECK (
    (debit_amount > 0 AND credit_amount = 0)
    OR (credit_amount > 0 AND debit_amount = 0)
  )
);

CREATE INDEX IF NOT EXISTS idx_journal_entry_lines_account
  ON journal_entry_lines(account_id,entry_id);

CREATE OR REPLACE FUNCTION validate_posted_journal_entry()
RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
  target_entry_id UUID;
  entry_row journal_entries%ROWTYPE;
  debit_total NUMERIC(18,2);
  credit_total NUMERIC(18,2);
  line_count INTEGER;
  period_row accounting_periods%ROWTYPE;
BEGIN
  IF TG_TABLE_NAME='journal_entries' THEN
    target_entry_id=COALESCE(NEW.id,OLD.id);
  ELSE
    target_entry_id=COALESCE(NEW.entry_id,OLD.entry_id);
  END IF;

  SELECT * INTO entry_row FROM journal_entries WHERE id=target_entry_id;
  IF NOT FOUND OR entry_row.status <> 'posted' THEN
    RETURN COALESCE(NEW,OLD);
  END IF;

  SELECT COUNT(*),COALESCE(SUM(debit_amount),0),COALESCE(SUM(credit_amount),0)
  INTO line_count,debit_total,credit_total
  FROM journal_entry_lines
  WHERE entry_id=target_entry_id;

  IF line_count < 2 OR debit_total <= 0 OR debit_total <> credit_total THEN
    RAISE EXCEPTION 'bút toán % chưa cân: Nợ %, Có %, số dòng %', target_entry_id,debit_total,credit_total,line_count;
  END IF;

  SELECT * INTO period_row FROM accounting_periods WHERE id=entry_row.period_id;
  IF NOT FOUND OR period_row.status <> 'open' THEN
    RAISE EXCEPTION 'kỳ kế toán của bút toán % đã khóa hoặc không tồn tại', target_entry_id;
  END IF;
  IF entry_row.entry_date < period_row.date_from OR entry_row.entry_date > period_row.date_to THEN
    RAISE EXCEPTION 'ngày hạch toán của bút toán % không thuộc kỳ kế toán', target_entry_id;
  END IF;
  RETURN COALESCE(NEW,OLD);
END;
$$;

DROP TRIGGER IF EXISTS trg_validate_posted_journal_entry ON journal_entries;
CREATE CONSTRAINT TRIGGER trg_validate_posted_journal_entry
AFTER INSERT OR UPDATE OR DELETE ON journal_entries
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW EXECUTE FUNCTION validate_posted_journal_entry();

DROP TRIGGER IF EXISTS trg_validate_posted_journal_line ON journal_entry_lines;
CREATE CONSTRAINT TRIGGER trg_validate_posted_journal_line
AFTER INSERT OR UPDATE OR DELETE ON journal_entry_lines
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW EXECUTE FUNCTION validate_posted_journal_entry();

CREATE OR REPLACE FUNCTION prevent_posted_journal_entry_mutation()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF OLD.status='posted' THEN
    RAISE EXCEPTION 'bút toán đã ghi sổ là bất biến; hãy lập bút toán đảo hoặc điều chỉnh';
  END IF;
  RETURN COALESCE(NEW,OLD);
END;
$$;

DROP TRIGGER IF EXISTS trg_prevent_posted_journal_entry_mutation ON journal_entries;
CREATE TRIGGER trg_prevent_posted_journal_entry_mutation
BEFORE UPDATE OR DELETE ON journal_entries
FOR EACH ROW EXECUTE FUNCTION prevent_posted_journal_entry_mutation();

CREATE OR REPLACE FUNCTION prevent_posted_journal_line_mutation()
RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
  target_entry_id UUID;
  entry_status TEXT;
BEGIN
  target_entry_id=COALESCE(NEW.entry_id,OLD.entry_id);
  SELECT status INTO entry_status FROM journal_entries WHERE id=target_entry_id;
  IF entry_status='posted' THEN
    RAISE EXCEPTION 'chi tiết bút toán đã ghi sổ là bất biến; hãy lập bút toán đảo hoặc điều chỉnh';
  END IF;
  RETURN COALESCE(NEW,OLD);
END;
$$;

DROP TRIGGER IF EXISTS trg_prevent_posted_journal_line_mutation ON journal_entry_lines;
CREATE TRIGGER trg_prevent_posted_journal_line_mutation
BEFORE INSERT OR UPDATE OR DELETE ON journal_entry_lines
FOR EACH ROW EXECUTE FUNCTION prevent_posted_journal_line_mutation();

CREATE TABLE IF NOT EXISTS accounting_posting_rules (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  book_id UUID NOT NULL REFERENCES accounting_books(id) ON DELETE CASCADE,
  event_key TEXT NOT NULL,
  debit_account_id UUID NOT NULL,
  credit_account_id UUID NOT NULL,
  tax_account_id UUID,
  active BOOLEAN NOT NULL DEFAULT true,
  description TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT accounting_posting_rules_event_unique UNIQUE (book_id,event_key),
  CONSTRAINT accounting_posting_rules_debit_book_fk FOREIGN KEY (debit_account_id,book_id) REFERENCES accounting_accounts(id,book_id),
  CONSTRAINT accounting_posting_rules_credit_book_fk FOREIGN KEY (credit_account_id,book_id) REFERENCES accounting_accounts(id,book_id),
  CONSTRAINT accounting_posting_rules_tax_book_fk FOREIGN KEY (tax_account_id,book_id) REFERENCES accounting_accounts(id,book_id),
  CONSTRAINT accounting_posting_rules_event_nonblank CHECK (NULLIF(btrim(event_key),'') IS NOT NULL)
);

INSERT INTO accounting_posting_rules (book_id,event_key,debit_account_id,credit_account_id,tax_account_id,description)
SELECT b.id,v.event_key,da.id,ca.id,ta.id,v.description
FROM accounting_books b
JOIN (VALUES
  ('input_invoice_verified','154','331','1331','Ghi nhận hóa đơn đầu vào đã xác minh'),
  ('output_invoice_issued','131','5111','3331','Ghi nhận hóa đơn đầu ra đã phát hành'),
  ('payable_payment_approved','331','1121',NULL,'Thanh toán công nợ phải trả'),
  ('receivable_receipt_approved','1121','131',NULL,'Thu công nợ phải thu'),
  ('supplier_advance_paid','331','1121',NULL,'Chi ứng trước nhà cung cấp'),
  ('client_advance_received','1121','131',NULL,'Thu ứng trước khách hàng'),
  ('generic_outbound_approved','642','1121',NULL,'Ghi nhận phiếu chi không qua công nợ'),
  ('generic_inbound_approved','1121','711',NULL,'Ghi nhận phiếu thu khác')
) AS v(event_key,debit_code,credit_code,tax_code,description) ON true
JOIN accounting_accounts da ON da.book_id=b.id AND da.account_code=v.debit_code
JOIN accounting_accounts ca ON ca.book_id=b.id AND ca.account_code=v.credit_code
LEFT JOIN accounting_accounts ta ON ta.book_id=b.id AND ta.account_code=v.tax_code
WHERE b.book_code='CONSFLOW-TT200'
ON CONFLICT (book_id,event_key) DO NOTHING;

CREATE TABLE IF NOT EXISTS budget_movements (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  budget_id UUID NOT NULL REFERENCES accounting_budgets(id) ON DELETE RESTRICT,
  project_id UUID NOT NULL REFERENCES projects(id) ON DELETE RESTRICT,
  movement_type TEXT NOT NULL,
  commitment_delta NUMERIC(18,2) NOT NULL DEFAULT 0,
  actual_delta NUMERIC(18,2) NOT NULL DEFAULT 0,
  paid_delta NUMERIC(18,2) NOT NULL DEFAULT 0,
  source_type TEXT NOT NULL,
  source_id UUID NOT NULL,
  source_event TEXT NOT NULL,
  journal_entry_id UUID REFERENCES journal_entries(id) ON DELETE RESTRICT,
  reversal_of_id UUID REFERENCES budget_movements(id) ON DELETE RESTRICT,
  description TEXT NOT NULL,
  occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_by UUID REFERENCES users(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT budget_movements_type_check CHECK (movement_type IN ('commitment','actual','payment','release','reversal','adjustment','opening')),
  CONSTRAINT budget_movements_nonzero_check CHECK (commitment_delta <> 0 OR actual_delta <> 0 OR paid_delta <> 0),
  CONSTRAINT budget_movements_source_nonblank CHECK (NULLIF(btrim(source_type),'') IS NOT NULL AND NULLIF(btrim(source_event),'') IS NOT NULL),
  CONSTRAINT budget_movements_description_nonblank CHECK (NULLIF(btrim(description),'') IS NOT NULL),
  CONSTRAINT budget_movements_reversal_self_check CHECK (reversal_of_id IS NULL OR reversal_of_id <> id),
  CONSTRAINT budget_movements_source_unique UNIQUE (budget_id,source_type,source_id,source_event)
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_budget_movements_single_reversal
  ON budget_movements(reversal_of_id)
  WHERE reversal_of_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_budget_movements_project_time
  ON budget_movements(project_id,occurred_at DESC);

CREATE OR REPLACE FUNCTION prevent_budget_movement_mutation()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  RAISE EXCEPTION 'biến động ngân sách là bất biến; hãy lập biến động đảo hoặc điều chỉnh';
END;
$$;

DROP TRIGGER IF EXISTS trg_prevent_budget_movement_mutation ON budget_movements;
CREATE TRIGGER trg_prevent_budget_movement_mutation
BEFORE UPDATE OR DELETE ON budget_movements
FOR EACH ROW EXECUTE FUNCTION prevent_budget_movement_mutation();

CREATE OR REPLACE VIEW accounting_budget_balances AS
SELECT
  b.id,
  b.project_id,
  b.budget_code,
  b.name,
  b.planned_amount,
  b.approved_amount,
  COALESCE(SUM(m.commitment_delta),0)::numeric(18,2) AS committed_amount,
  COALESCE(SUM(m.actual_delta),0)::numeric(18,2) AS actual_amount,
  COALESCE(SUM(m.paid_delta),0)::numeric(18,2) AS paid_amount,
  GREATEST(
    b.approved_amount-COALESCE(SUM(m.commitment_delta),0)-COALESCE(SUM(m.actual_delta),0),
    0
  )::numeric(18,2) AS available_amount,
  COALESCE(SUM(m.actual_delta),0)::numeric(18,2) AS used_amount,
  GREATEST(
    b.approved_amount-COALESCE(SUM(m.commitment_delta),0)-COALESCE(SUM(m.actual_delta),0),
    0
  )::numeric(18,2) AS remaining_amount,
  b.status,
  b.approved_by,
  b.created_at,
  b.updated_at
FROM accounting_budgets b
LEFT JOIN budget_movements m ON m.budget_id=b.id
GROUP BY b.id;

CREATE TABLE IF NOT EXISTS tax_codes (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  book_id UUID NOT NULL REFERENCES accounting_books(id) ON DELETE RESTRICT,
  tax_code TEXT NOT NULL,
  name TEXT NOT NULL,
  direction TEXT NOT NULL,
  vat_rate NUMERIC(7,4) NOT NULL,
  deductible_rate NUMERIC(7,4) NOT NULL DEFAULT 100,
  input_account_id UUID,
  output_account_id UUID,
  effective_from DATE NOT NULL DEFAULT CURRENT_DATE,
  effective_to DATE,
  active BOOLEAN NOT NULL DEFAULT true,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT tax_codes_book_code_unique UNIQUE (book_id,tax_code),
  CONSTRAINT tax_codes_direction_check CHECK (direction IN ('input','output','both')),
  CONSTRAINT tax_codes_vat_rate_check CHECK (vat_rate BETWEEN 0 AND 100),
  CONSTRAINT tax_codes_deductible_rate_check CHECK (deductible_rate BETWEEN 0 AND 100),
  CONSTRAINT tax_codes_date_check CHECK (effective_to IS NULL OR effective_to >= effective_from),
  CONSTRAINT tax_codes_input_account_book_fk FOREIGN KEY (input_account_id,book_id) REFERENCES accounting_accounts(id,book_id),
  CONSTRAINT tax_codes_output_account_book_fk FOREIGN KEY (output_account_id,book_id) REFERENCES accounting_accounts(id,book_id)
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_tax_codes_id_book
  ON tax_codes(id,book_id);

INSERT INTO tax_codes (book_id,tax_code,name,direction,vat_rate,deductible_rate,input_account_id,output_account_id,effective_from)
SELECT b.id,v.tax_code,v.name,v.direction,v.vat_rate,v.deductible_rate,ia.id,oa.id,DATE '2020-01-01'
FROM accounting_books b
JOIN (VALUES
  ('VAT-0','Thuế GTGT 0%','both',0::numeric,100::numeric),
  ('VAT-5','Thuế GTGT 5%','both',5::numeric,100::numeric),
  ('VAT-8','Thuế GTGT 8%','both',8::numeric,100::numeric),
  ('VAT-10','Thuế GTGT 10%','both',10::numeric,100::numeric),
  ('VAT-KCT','Không chịu thuế GTGT','both',0::numeric,0::numeric),
  ('VAT-KDK','Thuế GTGT đầu vào không được khấu trừ','input',10::numeric,0::numeric)
) AS v(tax_code,name,direction,vat_rate,deductible_rate) ON true
LEFT JOIN accounting_accounts ia ON ia.book_id=b.id AND ia.account_code='1331'
LEFT JOIN accounting_accounts oa ON oa.book_id=b.id AND oa.account_code='3331'
WHERE b.book_code='CONSFLOW-TT200'
ON CONFLICT (book_id,tax_code) DO NOTHING;

CREATE TABLE IF NOT EXISTS tax_periods (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  book_id UUID NOT NULL REFERENCES accounting_books(id) ON DELETE RESTRICT,
  fiscal_year INTEGER NOT NULL,
  period_no SMALLINT NOT NULL,
  date_from DATE NOT NULL,
  date_to DATE NOT NULL,
  status TEXT NOT NULL DEFAULT 'open',
  prepared_by UUID REFERENCES users(id),
  prepared_at TIMESTAMPTZ,
  filed_by UUID REFERENCES users(id),
  filed_at TIMESTAMPTZ,
  locked_by UUID REFERENCES users(id),
  locked_at TIMESTAMPTZ,
  amendment_no INTEGER NOT NULL DEFAULT 0,
  notes TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT tax_periods_book_period_unique UNIQUE (book_id,fiscal_year,period_no),
  CONSTRAINT tax_periods_number_check CHECK (period_no BETWEEN 1 AND 12),
  CONSTRAINT tax_periods_date_check CHECK (date_to >= date_from),
  CONSTRAINT tax_periods_status_check CHECK (status IN ('open','prepared','locked','filed','amended')),
  CONSTRAINT tax_periods_amendment_check CHECK (amendment_no >= 0)
);

INSERT INTO tax_periods (book_id,fiscal_year,period_no,date_from,date_to,status)
SELECT
  '73000000-0000-0000-0000-000000000001'::uuid,
  EXTRACT(YEAR FROM CURRENT_DATE)::integer,
  month_no,
  make_date(EXTRACT(YEAR FROM CURRENT_DATE)::integer,month_no,1),
  (make_date(EXTRACT(YEAR FROM CURRENT_DATE)::integer,month_no,1) + INTERVAL '1 month - 1 day')::date,
  'open'
FROM generate_series(1,12) AS month_no
ON CONFLICT (book_id,fiscal_year,period_no) DO NOTHING;

CREATE UNIQUE INDEX IF NOT EXISTS ux_input_invoices_id_project_v73
  ON input_invoices(id,project_id);

CREATE UNIQUE INDEX IF NOT EXISTS ux_output_invoices_id_project_v73
  ON output_invoices(id,project_id);

CREATE TABLE IF NOT EXISTS invoice_tax_lines (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  book_id UUID NOT NULL,
  project_id UUID NOT NULL,
  input_invoice_id UUID,
  output_invoice_id UUID,
  line_no INTEGER NOT NULL,
  tax_code_id UUID NOT NULL,
  taxable_amount NUMERIC(18,2) NOT NULL,
  vat_amount NUMERIC(18,2) NOT NULL,
  deductible_amount NUMERIC(18,2) NOT NULL DEFAULT 0,
  nondeductible_amount NUMERIC(18,2) NOT NULL DEFAULT 0,
  description TEXT,
  created_by UUID REFERENCES users(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT invoice_tax_lines_project_book_fk FOREIGN KEY (project_id,book_id) REFERENCES projects(id,accounting_book_id) ON DELETE RESTRICT,
  CONSTRAINT invoice_tax_lines_input_project_fk FOREIGN KEY (input_invoice_id,project_id) REFERENCES input_invoices(id,project_id) ON DELETE CASCADE,
  CONSTRAINT invoice_tax_lines_output_project_fk FOREIGN KEY (output_invoice_id,project_id) REFERENCES output_invoices(id,project_id) ON DELETE CASCADE,
  CONSTRAINT invoice_tax_lines_tax_book_fk FOREIGN KEY (tax_code_id,book_id) REFERENCES tax_codes(id,book_id) ON DELETE RESTRICT,
  CONSTRAINT invoice_tax_lines_invoice_check CHECK ((input_invoice_id IS NOT NULL)::integer + (output_invoice_id IS NOT NULL)::integer = 1),
  CONSTRAINT invoice_tax_lines_number_positive CHECK (line_no > 0),
  CONSTRAINT invoice_tax_lines_amounts_check CHECK (
    taxable_amount >= 0
    AND vat_amount >= 0
    AND deductible_amount >= 0
    AND nondeductible_amount >= 0
    AND deductible_amount+nondeductible_amount=vat_amount
  )
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_invoice_tax_lines_input_line
  ON invoice_tax_lines(input_invoice_id,line_no)
  WHERE input_invoice_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS ux_invoice_tax_lines_output_line
  ON invoice_tax_lines(output_invoice_id,line_no)
  WHERE output_invoice_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_invoice_tax_lines_project_tax
  ON invoice_tax_lines(project_id,tax_code_id,created_at DESC);

CREATE TABLE IF NOT EXISTS tax_adjustments (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  tax_period_id UUID NOT NULL REFERENCES tax_periods(id) ON DELETE RESTRICT,
  project_id UUID REFERENCES projects(id) ON DELETE RESTRICT,
  tax_code_id UUID NOT NULL REFERENCES tax_codes(id) ON DELETE RESTRICT,
  adjustment_type TEXT NOT NULL,
  taxable_delta NUMERIC(18,2) NOT NULL DEFAULT 0,
  vat_delta NUMERIC(18,2) NOT NULL,
  reason TEXT NOT NULL,
  source_document_no TEXT,
  journal_entry_id UUID REFERENCES journal_entries(id) ON DELETE RESTRICT,
  reversal_of_id UUID REFERENCES tax_adjustments(id) ON DELETE RESTRICT,
  created_by UUID REFERENCES users(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT tax_adjustments_type_check CHECK (adjustment_type IN ('increase','decrease','reversal','opening')),
  CONSTRAINT tax_adjustments_nonzero_check CHECK (taxable_delta <> 0 OR vat_delta <> 0),
  CONSTRAINT tax_adjustments_reason_nonblank CHECK (NULLIF(btrim(reason),'') IS NOT NULL),
  CONSTRAINT tax_adjustments_reversal_self_check CHECK (reversal_of_id IS NULL OR reversal_of_id <> id)
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_tax_adjustments_single_reversal
  ON tax_adjustments(reversal_of_id)
  WHERE reversal_of_id IS NOT NULL;

CREATE OR REPLACE FUNCTION prevent_tax_adjustment_mutation()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  RAISE EXCEPTION 'điều chỉnh thuế là bất biến; hãy lập điều chỉnh đảo';
END;
$$;

DROP TRIGGER IF EXISTS trg_prevent_tax_adjustment_mutation ON tax_adjustments;
CREATE TRIGGER trg_prevent_tax_adjustment_mutation
BEFORE UPDATE OR DELETE ON tax_adjustments
FOR EACH ROW EXECUTE FUNCTION prevent_tax_adjustment_mutation();

INSERT INTO permissions (id,key,name,description,module)
SELECT gen_random_uuid(),key,name,description,'accounting'
FROM (VALUES
  ('accounting.journal_view','Xem sổ kế toán','Xem Sổ Nhật ký chung, Sổ Cái và Bảng cân đối số phát sinh'),
  ('accounting.journal_post','Ghi sổ kế toán','Duyệt định khoản và ghi chứng từ vào Sổ Cái'),
  ('accounting.journal_reverse','Lập bút toán đảo','Lập bút toán đảo cho chứng từ đã ghi sổ'),
  ('accounting.period_close','Khóa sổ kỳ kế toán','Khóa kỳ kế toán sau khi hoàn tất đối chiếu'),
  ('accounting.period_reopen','Mở lại kỳ kế toán','Mở lại kỳ đã khóa với lý do kiểm soát bắt buộc'),
  ('accounting.tax_view','Xem sổ thuế','Xem sổ chi tiết và báo cáo thuế GTGT'),
  ('accounting.tax_manage','Quản lý kỳ thuế','Chuẩn bị, khóa, kê khai và điều chỉnh kỳ thuế')
) AS p(key,name,description)
ON CONFLICT (key) DO NOTHING;

INSERT INTO role_permissions (role_id,permission_id)
SELECT r.id,p.id
FROM roles r CROSS JOIN permissions p
WHERE p.key IN (
  'accounting.journal_view','accounting.journal_post','accounting.journal_reverse',
  'accounting.period_close','accounting.period_reopen','accounting.tax_view','accounting.tax_manage'
)
AND (
  r.name='admin'
  OR (r.name='director' AND p.key IN ('accounting.journal_view','accounting.journal_reverse','accounting.period_close','accounting.period_reopen','accounting.tax_view'))
  OR (r.name='accounting')
)
ON CONFLICT DO NOTHING;

ALTER TABLE accounting_budgets
  ADD CONSTRAINT accounting_budgets_amounts_check_v73
  CHECK (
    planned_amount >= 0
    AND approved_amount >= 0
    AND used_amount >= 0
    AND remaining_amount >= 0
    AND approved_amount <= planned_amount
  ) NOT VALID;

ALTER TABLE accounting_budgets
  ADD CONSTRAINT accounting_budgets_status_check_v73
  CHECK (status IN ('draft','submitted','approved','cancelled')) NOT VALID;

ALTER TABLE payables
  ADD CONSTRAINT payables_amounts_check_v73
  CHECK (
    gross_amount >= 0
    AND advance_amount >= 0
    AND retention_amount >= 0
    AND deduction_amount >= 0
    AND paid_amount >= 0
    AND remaining_amount >= 0
  ) NOT VALID;

ALTER TABLE payables
  ADD CONSTRAINT payables_status_check_v73
  CHECK (status IN ('draft','submitted','approved','partially_paid','paid','overdue','cancelled','deleted')) NOT VALID;

ALTER TABLE receivables
  ADD CONSTRAINT receivables_amounts_check_v73
  CHECK (contract_amount >= 0 AND gross_amount >= 0 AND received_amount >= 0 AND remaining_amount >= 0) NOT VALID;

ALTER TABLE receivables
  ADD CONSTRAINT receivables_status_check_v73
  CHECK (status IN ('draft','submitted','approved','partially_received','received','overdue','cancelled','deleted')) NOT VALID;

ALTER TABLE payments
  ADD CONSTRAINT payments_amount_positive_v73
  CHECK (amount > 0) NOT VALID;

ALTER TABLE payments
  ADD CONSTRAINT payments_status_check_v73
  CHECK (status IN ('draft','submitted','pending','approved','cancelled','deleted')) NOT VALID;

ALTER TABLE payments
  ADD CONSTRAINT payments_type_check_v73
  CHECK (payment_type IN ('supplier_payment','subcontractor_payment','client_receipt','outbound','inbound','advance','advance_settlement','other')) NOT VALID;

CREATE INDEX IF NOT EXISTS idx_payments_project_status_paid
  ON payments(project_id,status,paid_at DESC,created_at DESC)
  WHERE status NOT IN ('cancelled','deleted');
