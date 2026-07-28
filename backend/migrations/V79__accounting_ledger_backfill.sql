INSERT INTO users (id,email,password_hash,full_name,status)
VALUES (
  '79900000-0000-0000-0000-000000000001',
  'accounting.backfill@system.local',
  'disabled',
  'Hệ thống chuyển đổi sổ kế toán',
  'inactive'
)
ON CONFLICT (email) DO NOTHING;

CREATE TABLE IF NOT EXISTS accounting_backfill_issues (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id UUID REFERENCES projects(id) ON DELETE RESTRICT,
  source_type TEXT NOT NULL,
  source_id UUID NOT NULL,
  source_code TEXT,
  issue_code TEXT NOT NULL,
  description TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'open',
  resolved_by UUID REFERENCES users(id),
  resolved_at TIMESTAMPTZ,
  resolution_note TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT accounting_backfill_issues_source_unique UNIQUE (source_type,source_id,issue_code),
  CONSTRAINT accounting_backfill_issues_status_check CHECK (status IN ('open','resolved','accepted'))
);

CREATE INDEX IF NOT EXISTS idx_accounting_backfill_issues_project_status
  ON accounting_backfill_issues(project_id,status,created_at DESC);

CREATE OR REPLACE FUNCTION backfill_accounting_ledger()
RETURNS VOID LANGUAGE plpgsql AS $$
DECLARE
  system_actor_id UUID := '79900000-0000-0000-0000-000000000001'::uuid;
BEGIN
  INSERT INTO accounting_periods (book_id,fiscal_year,period_no,date_from,date_to,status)
  SELECT DISTINCT
    project.accounting_book_id,
    year_value.fiscal_year,
    month_value.period_no,
    make_date(year_value.fiscal_year,month_value.period_no,1),
    (make_date(year_value.fiscal_year,month_value.period_no,1)+INTERVAL '1 month - 1 day')::date,
    'open'
  FROM projects project
  CROSS JOIN LATERAL (
    SELECT DISTINCT EXTRACT(YEAR FROM source_date)::integer AS fiscal_year
    FROM (
      SELECT invoice_date AS source_date FROM input_invoices WHERE project_id=project.id
      UNION ALL
      SELECT invoice_date FROM output_invoices WHERE project_id=project.id
      UNION ALL
      SELECT COALESCE(paid_at::date,created_at::date) FROM payments WHERE project_id=project.id
      UNION ALL
      SELECT CURRENT_DATE
    ) source_dates
  ) year_value
  CROSS JOIN LATERAL (SELECT generate_series(1,12)::integer AS period_no) month_value
  ON CONFLICT (book_id,fiscal_year,period_no) DO NOTHING;

  INSERT INTO journal_entries (
    book_id,project_id,period_id,entry_no,entry_date,document_date,description,
    source_type,source_id,source_event,status,created_by
  )
  SELECT
    invoice.accounting_book_id,invoice.project_id,period.id,
    'BF-IN-' || replace(invoice.id::text,'-',''),invoice.invoice_date,invoice.invoice_date,
    'Chuyển đổi hóa đơn đầu vào ' || invoice.invoice_number,
    'input_invoice',invoice.id,'input_invoice_verified','draft',COALESCE(invoice.verified_by,invoice.uploaded_by,system_actor_id)
  FROM input_invoices invoice
  JOIN accounting_periods period
    ON period.book_id=invoice.accounting_book_id
   AND invoice.invoice_date BETWEEN period.date_from AND period.date_to
   AND period.status='open'
  WHERE invoice.status='verified'
  ON CONFLICT DO NOTHING;

  INSERT INTO journal_entry_lines (entry_id,book_id,line_no,account_id,partner_id,description,debit_amount,credit_amount)
  SELECT entry.id,entry.book_id,1,invoice.expense_account_id,invoice.vendor_partner_id,
    'Giá trị mua vào và thuế GTGT không được khấu trừ',
    invoice.total_amount-COALESCE(tax.deductible_amount,0),0
  FROM journal_entries entry
  JOIN input_invoices invoice ON invoice.id=entry.source_id
  LEFT JOIN LATERAL (
    SELECT SUM(deductible_amount) AS deductible_amount FROM invoice_tax_lines WHERE input_invoice_id=invoice.id
  ) tax ON true
  WHERE entry.source_type='input_invoice' AND entry.source_event='input_invoice_verified' AND entry.status='draft'
    AND invoice.total_amount-COALESCE(tax.deductible_amount,0) > 0
  ON CONFLICT DO NOTHING;

  INSERT INTO journal_entry_lines (entry_id,book_id,line_no,account_id,partner_id,description,debit_amount,credit_amount)
  SELECT entry.id,entry.book_id,2,account.id,invoice.vendor_partner_id,
    'Thuế GTGT đầu vào được khấu trừ',COALESCE(tax.deductible_amount,0),0
  FROM journal_entries entry
  JOIN input_invoices invoice ON invoice.id=entry.source_id
  JOIN accounting_accounts account ON account.book_id=entry.book_id AND account.account_code='1331'
  LEFT JOIN LATERAL (
    SELECT SUM(deductible_amount) AS deductible_amount FROM invoice_tax_lines WHERE input_invoice_id=invoice.id
  ) tax ON true
  WHERE entry.source_type='input_invoice' AND entry.source_event='input_invoice_verified' AND entry.status='draft'
    AND COALESCE(tax.deductible_amount,0) > 0
  ON CONFLICT DO NOTHING;

  INSERT INTO journal_entry_lines (entry_id,book_id,line_no,account_id,partner_id,description,debit_amount,credit_amount)
  SELECT entry.id,entry.book_id,3,account.id,invoice.vendor_partner_id,
    'Công nợ phải trả theo hóa đơn đầu vào',0,invoice.total_amount
  FROM journal_entries entry
  JOIN input_invoices invoice ON invoice.id=entry.source_id
  JOIN accounting_accounts account ON account.book_id=entry.book_id AND account.account_code='331'
  WHERE entry.source_type='input_invoice' AND entry.source_event='input_invoice_verified' AND entry.status='draft'
  ON CONFLICT DO NOTHING;

  UPDATE journal_entries entry
  SET status='posted',posted_by=COALESCE(invoice.verified_by,invoice.uploaded_by,system_actor_id),posted_at=COALESCE(invoice.verified_at,invoice.updated_at),updated_at=now()
  FROM input_invoices invoice
  WHERE entry.source_type='input_invoice' AND entry.source_id=invoice.id
    AND entry.source_event='input_invoice_verified' AND entry.status='draft';

  INSERT INTO journal_entries (
    book_id,project_id,period_id,entry_no,entry_date,document_date,description,
    source_type,source_id,source_event,status,created_by
  )
  SELECT
    invoice.accounting_book_id,invoice.project_id,period.id,
    'BF-OUT-' || replace(invoice.id::text,'-',''),invoice.invoice_date,invoice.invoice_date,
    'Chuyển đổi hóa đơn đầu ra ' || invoice.invoice_number,
    'output_invoice',invoice.id,'output_invoice_issued','draft',COALESCE(invoice.issued_by,invoice.uploaded_by,system_actor_id)
  FROM output_invoices invoice
  JOIN accounting_periods period
    ON period.book_id=invoice.accounting_book_id
   AND invoice.invoice_date BETWEEN period.date_from AND period.date_to
   AND period.status='open'
  WHERE invoice.status='issued'
  ON CONFLICT DO NOTHING;

  INSERT INTO journal_entry_lines (entry_id,book_id,line_no,account_id,partner_id,description,debit_amount,credit_amount)
  SELECT entry.id,entry.book_id,1,account.id,invoice.client_partner_id,
    'Công nợ phải thu theo hóa đơn đầu ra',invoice.total_amount,0
  FROM journal_entries entry
  JOIN output_invoices invoice ON invoice.id=entry.source_id
  JOIN accounting_accounts account ON account.book_id=entry.book_id AND account.account_code='131'
  WHERE entry.source_type='output_invoice' AND entry.source_event='output_invoice_issued' AND entry.status='draft'
  ON CONFLICT DO NOTHING;

  INSERT INTO journal_entry_lines (entry_id,book_id,line_no,account_id,partner_id,description,debit_amount,credit_amount)
  SELECT entry.id,entry.book_id,2,invoice.revenue_account_id,invoice.client_partner_id,
    'Doanh thu chưa có thuế GTGT',0,invoice.total_amount-invoice.vat_amount
  FROM journal_entries entry
  JOIN output_invoices invoice ON invoice.id=entry.source_id
  WHERE entry.source_type='output_invoice' AND entry.source_event='output_invoice_issued' AND entry.status='draft'
    AND invoice.total_amount-invoice.vat_amount > 0
  ON CONFLICT DO NOTHING;

  INSERT INTO journal_entry_lines (entry_id,book_id,line_no,account_id,partner_id,description,debit_amount,credit_amount)
  SELECT entry.id,entry.book_id,3,account.id,invoice.client_partner_id,
    'Thuế GTGT đầu ra phải nộp',0,invoice.vat_amount
  FROM journal_entries entry
  JOIN output_invoices invoice ON invoice.id=entry.source_id
  JOIN accounting_accounts account ON account.book_id=entry.book_id AND account.account_code='3331'
  WHERE entry.source_type='output_invoice' AND entry.source_event='output_invoice_issued' AND entry.status='draft'
    AND invoice.vat_amount > 0
  ON CONFLICT DO NOTHING;

  UPDATE journal_entries entry
  SET status='posted',posted_by=COALESCE(invoice.issued_by,invoice.uploaded_by,system_actor_id),posted_at=COALESCE(invoice.issued_at,invoice.updated_at),updated_at=now()
  FROM output_invoices invoice
  WHERE entry.source_type='output_invoice' AND entry.source_id=invoice.id
    AND entry.source_event='output_invoice_issued' AND entry.status='draft';

  INSERT INTO journal_entries (
    book_id,project_id,period_id,entry_no,entry_date,document_date,description,
    source_type,source_id,source_event,status,created_by
  )
  SELECT
    payment.accounting_book_id,payment.project_id,period.id,
    'BF-PAY-' || replace(payment.id::text,'-',''),COALESCE(payment.paid_at::date,payment.created_at::date),COALESCE(payment.paid_at::date,payment.created_at::date),
    'Chuyển đổi Phiếu thu/Phiếu chi ' || payment.payment_code,
    'payment',payment.id,
    CASE
      WHEN COALESCE(pay_alloc.amount,0)>0 THEN 'payable_payment_approved'
      WHEN COALESCE(rec_alloc.amount,0)>0 THEN 'receivable_receipt_approved'
      WHEN payment.payment_type='outbound' THEN 'generic_outbound_approved'
      WHEN payment.payment_type='inbound' THEN 'generic_inbound_approved'
    END,
    'draft',COALESCE(payment.approved_by,payment.created_by,system_actor_id)
  FROM payments payment
  JOIN accounting_periods period
    ON period.book_id=payment.accounting_book_id
   AND COALESCE(payment.paid_at::date,payment.created_at::date) BETWEEN period.date_from AND period.date_to
   AND period.status='open'
  LEFT JOIN LATERAL (
    SELECT SUM(amount) AS amount FROM payment_allocations
    WHERE payment_id=payment.id AND allocation_type <> 'advance_settlement'
  ) pay_alloc ON true
  LEFT JOIN LATERAL (
    SELECT SUM(amount) AS amount FROM receipt_allocations
    WHERE payment_id=payment.id AND allocation_type <> 'advance_settlement'
  ) rec_alloc ON true
  WHERE payment.status='approved' AND payment.payment_type <> 'advance_settlement'
    AND (
      COALESCE(pay_alloc.amount,0)>0 OR COALESCE(rec_alloc.amount,0)>0
      OR payment.payment_type IN ('outbound','inbound')
    )
  ON CONFLICT DO NOTHING;

  INSERT INTO journal_entry_lines (entry_id,book_id,line_no,account_id,description,debit_amount,credit_amount)
  SELECT entry.id,entry.book_id,1,
    CASE
      WHEN entry.source_event='payable_payment_approved' THEN account_331.id
      WHEN entry.source_event IN ('receivable_receipt_approved','generic_inbound_approved') THEN payment.cash_account_id
      ELSE account_642.id
    END,
    'Định khoản chuyển đổi ' || payment.payment_code,payment.amount,0
  FROM journal_entries entry
  JOIN payments payment ON payment.id=entry.source_id
  JOIN accounting_accounts account_331 ON account_331.book_id=entry.book_id AND account_331.account_code='331'
  JOIN accounting_accounts account_642 ON account_642.book_id=entry.book_id AND account_642.account_code='642'
  WHERE entry.source_type='payment' AND entry.status='draft'
  ON CONFLICT DO NOTHING;

  INSERT INTO journal_entry_lines (entry_id,book_id,line_no,account_id,description,debit_amount,credit_amount)
  SELECT entry.id,entry.book_id,2,
    CASE
      WHEN entry.source_event IN ('payable_payment_approved','generic_outbound_approved') THEN payment.cash_account_id
      WHEN entry.source_event='receivable_receipt_approved' THEN account_131.id
      ELSE account_711.id
    END,
    'Định khoản chuyển đổi ' || payment.payment_code,0,payment.amount
  FROM journal_entries entry
  JOIN payments payment ON payment.id=entry.source_id
  JOIN accounting_accounts account_131 ON account_131.book_id=entry.book_id AND account_131.account_code='131'
  JOIN accounting_accounts account_711 ON account_711.book_id=entry.book_id AND account_711.account_code='711'
  WHERE entry.source_type='payment' AND entry.status='draft'
  ON CONFLICT DO NOTHING;

  UPDATE journal_entries entry
  SET status='posted',posted_by=COALESCE(payment.approved_by,payment.created_by,system_actor_id),posted_at=COALESCE(payment.paid_at,payment.updated_at),updated_at=now()
  FROM payments payment
  WHERE entry.source_type='payment' AND entry.source_id=payment.id AND entry.status='draft';

  INSERT INTO budget_movements (
    budget_id,project_id,movement_type,commitment_delta,actual_delta,paid_delta,
    source_type,source_id,source_event,description,occurred_at,created_by
  )
  SELECT payable.budget_id,payable.project_id,'opening',
    GREATEST(payable.gross_amount-COALESCE(verified.total_amount,0),0),0,0,
    'payable',payable.id,'payable_open_commitment_backfill',
    'Chuyển đổi cam kết chi còn mở',payable.created_at,COALESCE(payable.approved_by,payable.created_by,system_actor_id)
  FROM payables payable
  LEFT JOIN LATERAL (
    SELECT SUM(total_amount) AS total_amount FROM input_invoices
    WHERE payable_id=payable.id AND status='verified'
  ) verified ON true
  WHERE payable.budget_id IS NOT NULL
    AND payable.status IN ('approved','partially_paid','paid','overdue')
    AND GREATEST(payable.gross_amount-COALESCE(verified.total_amount,0),0)>0
  ON CONFLICT (budget_id,source_type,source_id,source_event) DO NOTHING;

  INSERT INTO budget_movements (
    budget_id,project_id,movement_type,commitment_delta,actual_delta,paid_delta,
    source_type,source_id,source_event,journal_entry_id,description,occurred_at,created_by
  )
  SELECT payable.budget_id,invoice.project_id,'actual',0,
    invoice.total_amount-COALESCE(tax.deductible_amount,0),0,
    'input_invoice',invoice.id,'input_invoice_verified',entry.id,
    'Chuyển đổi thực chi theo hóa đơn đầu vào',invoice.invoice_date,
    COALESCE(invoice.verified_by,invoice.uploaded_by,system_actor_id)
  FROM input_invoices invoice
  JOIN payables payable ON payable.id=invoice.payable_id AND payable.budget_id IS NOT NULL
  JOIN journal_entries entry ON entry.source_type='input_invoice' AND entry.source_id=invoice.id AND entry.source_event='input_invoice_verified' AND entry.status='posted'
  LEFT JOIN LATERAL (
    SELECT SUM(deductible_amount) AS deductible_amount FROM invoice_tax_lines WHERE input_invoice_id=invoice.id
  ) tax ON true
  WHERE invoice.status='verified'
  ON CONFLICT (budget_id,source_type,source_id,source_event) DO NOTHING;

  INSERT INTO budget_movements (
    budget_id,project_id,movement_type,commitment_delta,actual_delta,paid_delta,
    source_type,source_id,source_event,journal_entry_id,description,occurred_at,created_by
  )
  SELECT payment.budget_id,payment.project_id,'payment',0,
    CASE WHEN entry.source_event='generic_outbound_approved' THEN payment.amount ELSE 0 END,
    payment.amount,'payment',payment.id,entry.source_event,entry.id,
    'Chuyển đổi số đã chi theo Phiếu chi',COALESCE(payment.paid_at,payment.created_at),
    COALESCE(payment.approved_by,payment.created_by,system_actor_id)
  FROM payments payment
  JOIN journal_entries entry ON entry.source_type='payment' AND entry.source_id=payment.id AND entry.status='posted'
  WHERE payment.status='approved' AND payment.budget_id IS NOT NULL
    AND entry.source_event IN ('payable_payment_approved','generic_outbound_approved')
  ON CONFLICT (budget_id,source_type,source_id,source_event) DO NOTHING;

  INSERT INTO budget_movements (
    budget_id,project_id,movement_type,commitment_delta,actual_delta,paid_delta,
    source_type,source_id,source_event,description,occurred_at,created_by
  )
  SELECT budget.id,budget.project_id,'opening',0,
    GREATEST(budget.used_amount-COALESCE(movement.actual_amount,0),0),0,
    'accounting_budget',budget.id,'legacy_used_amount_opening',
    'Số thực chi đầu kỳ chuyển từ dữ liệu ngân sách cũ',budget.created_at,system_actor_id
  FROM accounting_budgets budget
  LEFT JOIN LATERAL (
    SELECT SUM(actual_delta) AS actual_amount FROM budget_movements WHERE budget_id=budget.id
  ) movement ON true
  WHERE budget.used_amount>COALESCE(movement.actual_amount,0)
  ON CONFLICT (budget_id,source_type,source_id,source_event) DO NOTHING;

  INSERT INTO accounting_backfill_issues (project_id,source_type,source_id,source_code,issue_code,description)
  SELECT payment.project_id,'payment',payment.id,payment.payment_code,'ambiguous_payment_direction',
    'Phiếu đã duyệt chưa đủ căn cứ xác định định khoản Nợ/Có; cần kế toán đối chiếu và lập bút toán bổ sung.'
  FROM payments payment
  WHERE payment.status='approved'
    AND NOT EXISTS (SELECT 1 FROM journal_entries entry WHERE entry.source_type='payment' AND entry.source_id=payment.id AND entry.status='posted')
  ON CONFLICT DO NOTHING;

  INSERT INTO accounting_backfill_issues (project_id,source_type,source_id,source_code,issue_code,description)
  SELECT invoice.project_id,'input_invoice',invoice.id,invoice.invoice_number,'closed_or_missing_period',
    'Hóa đơn đã xác thực chưa thể chuyển đổi vì kỳ kế toán thiếu hoặc đã khóa.'
  FROM input_invoices invoice
  WHERE invoice.status='verified'
    AND NOT EXISTS (SELECT 1 FROM journal_entries entry WHERE entry.source_type='input_invoice' AND entry.source_id=invoice.id AND entry.status='posted')
  ON CONFLICT DO NOTHING;

  INSERT INTO accounting_backfill_issues (project_id,source_type,source_id,source_code,issue_code,description)
  SELECT invoice.project_id,'output_invoice',invoice.id,invoice.invoice_number,'closed_or_missing_period',
    'Hóa đơn đã phát hành chưa thể chuyển đổi vì kỳ kế toán thiếu hoặc đã khóa.'
  FROM output_invoices invoice
  WHERE invoice.status='issued'
    AND NOT EXISTS (SELECT 1 FROM journal_entries entry WHERE entry.source_type='output_invoice' AND entry.source_id=invoice.id AND entry.status='posted')
  ON CONFLICT DO NOTHING;
END;
$$;

SELECT backfill_accounting_ledger();
