import type { ProjectOperationsRecord } from './commercial';
import type { AnyRecord } from './common';

export type PayableRecord = ProjectOperationsRecord & {
  project_id: string;
  accounting_book_id?: string;
  budget_id?: string;
  cost_account_id?: string;
  project_name?: string;
  payable_code: string;
  vendor_type: string;
  vendor_partner_id?: string;
  vendor_name: string;
  source_type?: string;
  source_id?: string;
  gross_amount: number;
  advance_amount?: number;
  retention_amount?: number;
  deduction_amount?: number;
  paid_amount?: number;
  remaining_amount?: number;
  due_date?: string;
  status: 'draft' | 'recorded' | 'approved' | 'partially_paid' | 'paid' | 'overdue' | 'cancelled' | string;
  invoice_count?: number;
  invoice_total?: number;
  verified_invoice_total?: number;
  schedule_count?: number;
  scheduled_total?: number;
  scheduled_remaining?: number;
  advance_request_count?: number;
  advance_requested_total?: number;
  advance_approved_total?: number;
  advance_paid_total?: number;
  advance_settled_total?: number;
  approved_payment_total?: number;
  latest_invoice_status?: string;
  latest_invoice_number?: string;
  latest_invoice_date?: string;
  effective_due_date?: string;
  overdue_days?: number;
  aging_bucket?: 'settled' | 'no_due_date' | 'not_due' | '1_30' | '31_60' | '61_90' | '90_plus' | string;
  reconciliation_issue_count?: number;
  reconciliation_status?: 'ok' | 'needs_review' | string;
};

export type PayableReconciliationWarning = AnyRecord & {
  code: string;
  severity: 'info' | 'warning' | 'critical' | string;
  message: string;
  details?: AnyRecord;
};

export type PayableLedgerEntry = AnyRecord & {
  occurred_at: string;
  entry_type: string;
  source_id: string;
  source_code: string;
  status: string;
  description: string;
  debit_amount: number;
  credit_amount: number;
  document_amount: number;
  cash_amount: number;
  running_balance: number;
  actor_name?: string;
};

export type PayableReconciliationSnapshot = AnyRecord & {
  summary: PayableRecord & {
    verified_invoice_count?: number;
    rejected_invoice_count?: number;
    overdue_schedule_count?: number;
    scheduled_paid_total?: number;
    next_schedule_due_date?: string;
    approved_payable_payment_total?: number;
    approved_advance_payment_total?: number;
    payment_allocation_total?: number;
    pending_payment_count?: number;
    computed_remaining?: number;
    balance_difference?: number;
    unverified_invoice_gap?: number;
    advance_unpaid_total?: number;
    advance_unsettled_total?: number;
    warning_count?: number;
  };
  warnings: PayableReconciliationWarning[];
  ledger: PayableLedgerEntry[];
};

export type PayablePaymentScheduleRecord = ProjectOperationsRecord & {
  project_id: string;
  project_name?: string;
  payable_id: string;
  payable_code?: string;
  vendor_name?: string;
  schedule_code: string;
  milestone_name: string;
  schedule_type: 'advance' | 'progress' | 'final' | 'retention' | 'other' | string;
  planned_amount: number;
  paid_amount: number;
  remaining_amount: number;
  due_date?: string;
  status: 'draft' | 'approved' | 'partially_paid' | 'paid' | 'overdue' | 'cancelled' | 'deleted' | string;
  notes?: string;
  created_by_name?: string;
  approved_by_name?: string;
};

export type PayableAdvanceRequestRecord = ProjectOperationsRecord & {
  project_id: string;
  project_name?: string;
  payable_id: string;
  payable_code?: string;
  vendor_name?: string;
  payable_vendor_name?: string;
  request_code: string;
  requested_by?: string;
  requested_by_name?: string;
  amount: number;
  paid_amount?: number;
  settled_amount: number;
  reason?: string;
  due_date?: string;
  paid_at?: string;
  settled_at?: string;
  status: 'draft' | 'submitted' | 'approved' | 'partially_settled' | 'settled' | 'cancelled' | 'deleted' | string;
  payment_id?: string;
  payment_code?: string;
  notes?: string;
  approved_by_name?: string;
};

export type PayablePaymentRecord = ProjectOperationsRecord & {
  project_id: string;
  project_name?: string;
  payable_id: string;
  payable_code?: string;
  payable_vendor_name?: string;
  payment_code: string;
  payment_type: string;
  payment_method: string;
  cash_account_id?: string;
  target_type?: string;
  target_id?: string;
  amount: number;
  allocation_id?: string;
  allocation_type?: 'payable_payment' | 'advance_payment' | 'advance_settlement' | string;
  allocation_amount?: number;
  allocation_notes?: string;
  schedule_id?: string;
  schedule_code?: string;
  milestone_name?: string;
  advance_request_id?: string;
  advance_request_code?: string;
  input_invoice_id?: string;
  input_invoice_number?: string;
  status: 'draft' | 'submitted' | 'pending' | 'approved' | 'paid' | 'cancelled' | string;
  paid_at?: string;
  approved_by_name?: string;
  created_by_name?: string;
};

export type ReceivableRecord = ProjectOperationsRecord & {
  project_id: string;
  accounting_book_id?: string;
  revenue_account_id?: string;
  project_name?: string;
  receivable_code: string;
  client_partner_id?: string;
  client_name: string;
  contract_amount?: number;
  milestone_name?: string;
  gross_amount: number;
  received_amount?: number;
  remaining_amount?: number;
  due_date?: string;
  status: 'draft' | 'recorded' | 'approved' | 'partially_received' | 'received' | 'overdue' | 'cancelled' | string;
  invoice_count?: number;
  invoice_total?: number;
  issued_invoice_count?: number;
  voided_invoice_count?: number;
  issued_invoice_total?: number;
  schedule_count?: number;
  scheduled_total?: number;
  scheduled_received_total?: number;
  scheduled_remaining?: number;
  advance_receipt_count?: number;
  advance_requested_total?: number;
  advance_approved_total?: number;
  advance_received_total?: number;
  advance_settled_total?: number;
  approved_receipt_total?: number;
  approved_advance_receipt_total?: number;
  receipt_allocation_total?: number;
  pending_receipt_count?: number;
  latest_invoice_status?: string;
  latest_invoice_number?: string;
  latest_invoice_date?: string;
  effective_due_date?: string;
  overdue_days?: number;
  aging_bucket?: 'settled' | 'no_due_date' | 'not_due' | '1_30' | '31_60' | '61_90' | '90_plus' | string;
  reconciliation_issue_count?: number;
  reconciliation_status?: 'ok' | 'needs_review' | string;
};

export type ReceivableReconciliationWarning = AnyRecord & {
  code: string;
  severity: 'info' | 'warning' | 'critical' | string;
  message: string;
  details?: AnyRecord;
};

export type ReceivableLedgerEntry = AnyRecord & {
  occurred_at: string;
  entry_type: string;
  source_id: string;
  source_code: string;
  status: string;
  description: string;
  debit_amount: number;
  credit_amount: number;
  document_amount: number;
  cash_amount: number;
  running_balance: number;
  actor_name?: string;
};

export type ReceivableReconciliationSnapshot = AnyRecord & {
  summary: ReceivableRecord & {
    overdue_schedule_count?: number;
    next_schedule_due_date?: string;
    computed_remaining?: number;
    balance_difference?: number;
    unissued_invoice_gap?: number;
    advance_unreceived_total?: number;
    advance_unsettled_total?: number;
    warning_count?: number;
  };
  warnings: ReceivableReconciliationWarning[];
  ledger: ReceivableLedgerEntry[];
};

export type ReceivablePaymentScheduleRecord = ProjectOperationsRecord & {
  project_id: string;
  project_name?: string;
  receivable_id: string;
  receivable_code?: string;
  client_name?: string;
  schedule_code: string;
  milestone_name: string;
  schedule_type: 'advance' | 'progress' | 'final' | 'retention' | 'other' | string;
  planned_amount: number;
  received_amount: number;
  remaining_amount: number;
  due_date?: string;
  status: 'draft' | 'approved' | 'partially_received' | 'received' | 'overdue' | 'cancelled' | 'deleted' | string;
  notes?: string;
  created_by_name?: string;
  approved_by_name?: string;
};

export type ReceivableAdvanceReceiptRecord = ProjectOperationsRecord & {
  project_id: string;
  project_name?: string;
  receivable_id: string;
  receivable_code?: string;
  client_name?: string;
  receivable_client_name?: string;
  receipt_code: string;
  requested_by?: string;
  requested_by_name?: string;
  amount: number;
  received_amount?: number;
  settled_amount: number;
  reason?: string;
  due_date?: string;
  received_at?: string;
  settled_at?: string;
  status: 'draft' | 'submitted' | 'approved' | 'partially_received' | 'received' | 'partially_settled' | 'settled' | 'cancelled' | 'deleted' | string;
  payment_id?: string;
  payment_code?: string;
  notes?: string;
  approved_by_name?: string;
};

export type ReceivablePaymentRecord = ProjectOperationsRecord & {
  project_id: string;
  project_name?: string;
  receivable_id: string;
  receivable_code?: string;
  receivable_client_name?: string;
  payment_code: string;
  payment_type: string;
  payment_method: string;
  cash_account_id?: string;
  target_type?: string;
  target_id?: string;
  amount: number;
  allocation_id?: string;
  allocation_type?: 'receivable_receipt' | 'advance_receipt' | 'advance_settlement' | string;
  allocation_amount?: number;
  allocation_notes?: string;
  schedule_id?: string;
  schedule_code?: string;
  milestone_name?: string;
  advance_receipt_id?: string;
  advance_receipt_code?: string;
  output_invoice_id?: string;
  output_invoice_number?: string;
  status: 'draft' | 'submitted' | 'pending' | 'approved' | 'paid' | 'cancelled' | string;
  paid_at?: string;
  approved_by_name?: string;
  created_by_name?: string;
};

export type InputInvoiceRecord = ProjectOperationsRecord & {
  project_id: string;
  accounting_book_id?: string;
  budget_id?: string;
  cost_account_id?: string;
  tax_code_id?: string;
  project_name?: string;
  payable_id: string;
  payable_code?: string;
  procurement_request_id?: string;
  procurement_request_code?: string;
  document_id?: string;
  document_name?: string;
  invoice_number: string;
  invoice_symbol?: string;
  invoice_date: string;
  vendor_partner_id?: string;
  vendor_name: string;
  vendor_tax_code?: string;
  amount_before_vat: number;
  vat_amount: number;
  total_amount: number;
  currency: 'VND' | string;
  status: 'draft' | 'verified' | 'rejected' | 'cancelled' | 'deleted' | string;
  notes?: string;
  storage_provider?: 'local' | string;
  storage_key?: string;
  mime_type?: string;
  file_name?: string;
  file_size?: number;
  checksum_sha256?: string;
  uploaded_by?: string;
  uploaded_by_name?: string;
  verified_by?: string;
  verified_by_name?: string;
  verified_at?: string;
  rejected_by?: string;
  rejected_by_name?: string;
  rejected_at?: string;
};

export type OutputInvoiceRecord = ProjectOperationsRecord & {
  project_id: string;
  accounting_book_id?: string;
  revenue_account_id?: string;
  tax_code_id?: string;
  project_name?: string;
  receivable_id: string;
  receivable_code?: string;
  payment_id?: string;
  payment_code?: string;
  payment_type?: string;
  document_id?: string;
  document_name?: string;
  invoice_number: string;
  invoice_symbol?: string;
  invoice_date: string;
  client_partner_id?: string;
  client_name: string;
  client_tax_code?: string;
  amount_before_vat: number;
  vat_amount: number;
  total_amount: number;
  currency: 'VND' | string;
  status: 'draft' | 'issued' | 'voided' | 'cancelled' | 'deleted' | string;
  notes?: string;
  storage_provider?: 'local' | string;
  storage_key?: string;
  mime_type?: string;
  file_name?: string;
  file_size?: number;
  checksum_sha256?: string;
  uploaded_by?: string;
  uploaded_by_name?: string;
  issued_by?: string;
  issued_by_name?: string;
  issued_at?: string;
  voided_by?: string;
  voided_by_name?: string;
  voided_at?: string;
};

export type AccountingBudgetRecord = ProjectOperationsRecord & {
  project_id: string;
  accounting_book_id?: string;
  budget_code: string;
  name: string;
  planned_amount: number;
  approved_amount: number;
  committed_amount: number;
  actual_amount: number;
  paid_amount: number;
  available_amount: number;
  variance_amount?: number;
  utilization_percent?: number;
  status: 'draft' | 'submitted' | 'approved' | 'rejected' | 'cancelled' | string;
};

export type AccountingPaymentRecord = ProjectOperationsRecord & {
  project_id: string;
  accounting_book_id?: string;
  budget_id?: string;
  cash_account_id?: string;
  payment_code: string;
  payment_type: string;
  target_type?: string;
  target_id?: string;
  amount: number;
  payment_method: string;
  status: 'draft' | 'submitted' | 'pending' | 'waiting_approval' | 'approved' | 'cancelled' | string;
  paid_at?: string;
};

export type AccountingAccountRecord = AnyRecord & {
  id: string;
  book_id: string;
  account_code: string;
  name: string;
  account_type: string;
  normal_balance: 'debit' | 'credit' | string;
  is_postable: boolean;
};

export type AccountingPeriodRecord = AnyRecord & {
  id: string;
  book_id: string;
  fiscal_year: number;
  period_no: number;
  date_from: string;
  date_to: string;
  status: 'open' | 'closed' | string;
  closed_at?: string;
};

export type VATReturnSnapshotRecord = AnyRecord & {
  id: string;
  tax_period_id: string;
  book_id: string;
  amendment_no: number;
  input_invoice_count: number;
  output_invoice_count: number;
  input_vat_amount: number;
  deductible_input_vat_amount: number;
  output_vat_amount: number;
  input_adjustment_vat_amount: number;
  output_adjustment_vat_amount: number;
  credit_brought_forward: number;
  vat_payable: number;
  credit_carried_forward: number;
  prepared_at: string;
};

export type AccountingTaxPeriodRecord = AnyRecord & {
  id: string;
  book_id: string;
  fiscal_year: number;
  period_type: 'month' | 'quarter';
  period_no: number;
  date_from: string;
  date_to: string;
  status: 'open' | 'prepared' | 'filed' | 'locked' | 'amended' | string;
  amendment_no: number;
};

export type AccountingTaxPeriodDetail = AccountingTaxPeriodRecord & {
  summary?: AnyRecord;
  issues?: AnyRecord[];
  purchases?: AnyRecord[];
  sales?: AnyRecord[];
  adjustments?: AnyRecord[];
  returns?: VATReturnSnapshotRecord[];
  events?: AnyRecord[];
  payments?: AccountingPaymentRecord[];
  latest_return?: VATReturnSnapshotRecord;
};

export type JournalEntryRecord = ProjectOperationsRecord & {
  project_id: string;
  book_id: string;
  period_id: string;
  entry_no: string;
  entry_date: string;
  document_date?: string;
  description: string;
  source_type: string;
  source_event: string;
  debit_total: number;
  credit_total: number;
  status: 'posted' | string;
  reversed?: boolean;
  reversal_entry_id?: string;
};

export type AccountingReconciliationIssue = ProjectOperationsRecord & {
  project_id: string;
  source_type: string;
  source_id: string;
  source_code?: string;
  issue_code: string;
  description: string;
  status: 'open' | 'resolved' | 'accepted' | string;
  resolution_note?: string;
  resolution_journal_entry_id?: string;
};
