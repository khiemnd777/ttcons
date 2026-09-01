import type { AnyRecord } from './common';

export type CashflowSummary = AnyRecord & {
  from: string;
  to: string;
  group_by: 'day' | 'week' | 'month' | string;
  status: 'approved' | 'all' | string;
  project_id?: string;
  opening_balance: number;
  payment_count: number;
  unclassified_count: number;
  non_cash_count: number;
  total_inflow: number;
  total_outflow: number;
  net_flow: number;
  closing_balance: number;
  pending_payment_count: number;
  pending_payment_total: number;
};

export type CashflowSeriesRow = AnyRecord & {
  period_start: string;
  inflow_amount: number;
  outflow_amount: number;
  net_amount: number;
  running_balance: number;
  payment_count: number;
};

export type CashflowCategoryRow = AnyRecord & {
  category: string;
  direction: 'inflow' | 'outflow' | 'non_cash' | 'unclassified' | string;
  payment_count: number;
  inflow_amount: number;
  outflow_amount: number;
  net_amount: number;
};

export type CashflowLedgerRow = AnyRecord & {
  occurred_at: string;
  id: string;
  project_id: string;
  project_name?: string;
  payment_code: string;
  payment_type: string;
  payment_method: string;
  status: string;
  direction: 'inflow' | 'outflow' | 'non_cash' | 'unclassified' | string;
  category: string;
  amount: number;
  inflow_amount: number;
  outflow_amount: number;
  net_amount: number;
  target_type?: string;
  target_id?: string;
  created_by_name?: string;
  approved_by_name?: string;
};

export type CashflowReport = AnyRecord & {
  summary: CashflowSummary;
  series: CashflowSeriesRow[];
  categories: CashflowCategoryRow[];
  ledger: CashflowLedgerRow[];
};

export type ProjectScopeOption = {
  id: string;
  code: string;
  name: string;
};

export type ProjectScopeOptionsPage = {
  items: ProjectScopeOption[];
  has_more: boolean;
  next_cursor?: string;
  selected: ProjectScopeOption | null;
};

export type ProjectScopeOptions = {
  projects: ProjectScopeOptionsPage;
};

export type ReportProjectOption = ProjectScopeOption;

export type ReportProjectOptionsPage = ProjectScopeOptionsPage;

export type ReportOptions = ProjectScopeOptions;

export type ProjectReportRow = AnyRecord & {
  project_id: string;
  name: string;
  is_sample?: boolean;
  status: string;
  progress_percent: number;
  open_issues: number;
  pending_approvals: number;
};

export type WarehouseReportRow = AnyRecord & {
  material_id: string;
  name: string;
  category?: string;
  stock_quantity: number;
  surplus_quantity: number;
};

export type ProcurementReportRow = AnyRecord & {
  status: string;
  budget_status: string;
  total: number;
  amount: number;
};

export type AccountingReportRow = AnyRecord & {
  project_id: string;
  name: string;
  payable_remaining: number;
  receivable_remaining: number;
  used_budget: number;
};

export type StaffReportRow = AnyRecord & {
  staff_id: string;
  full_name: string;
  department?: string;
  position?: string;
  project_count: number;
};

export type TaxReportSummary = AnyRecord & {
  from: string;
  to: string;
  group_by: 'month' | 'quarter' | string;
  source: 'all' | 'input' | 'output' | string;
  status: 'posted' | 'all' | string;
  project_id?: string;
  invoice_count: number;
  input_invoice_count: number;
  output_invoice_count: number;
  input_taxable_amount: number;
  input_vat_amount: number;
  input_total_amount: number;
  output_taxable_amount: number;
  output_vat_amount: number;
  output_total_amount: number;
  net_vat_amount: number;
  net_vat_status: 'payable' | 'credit' | 'neutral' | string;
  missing_tax_code_count: number;
  vat_math_mismatch_count: number;
  excluded_unposted_count: number;
  excluded_unposted_total: number;
  warning_count: number;
};

export type TaxPeriodRow = AnyRecord & {
  period_start: string;
  invoice_count: number;
  input_invoice_count: number;
  output_invoice_count: number;
  input_taxable_amount: number;
  input_vat_amount: number;
  input_total_amount: number;
  output_taxable_amount: number;
  output_vat_amount: number;
  output_total_amount: number;
  net_vat_amount: number;
};

export type TaxCounterpartyRow = AnyRecord & {
  direction: 'input' | 'output' | string;
  tax_code?: string;
  counterparty_name: string;
  invoice_count: number;
  taxable_amount: number;
  vat_amount: number;
  total_amount: number;
  warning_count: number;
};

export type TaxLedgerRow = AnyRecord & {
  invoice_date: string;
  id: string;
  direction: 'input' | 'output' | string;
  project_id: string;
  project_name?: string;
  invoice_number: string;
  invoice_symbol?: string;
  counterparty_name: string;
  tax_code?: string;
  amount_before_vat: number;
  vat_amount: number;
  total_amount: number;
  currency: 'VND' | string;
  status: string;
  source_type: string;
  source_id?: string;
  source_code?: string;
  secondary_source_type?: string;
  secondary_source_id?: string;
  secondary_source_code?: string;
  document_id?: string;
  document_name?: string;
  missing_tax_code?: boolean;
  vat_math_mismatch?: boolean;
  warning_flags?: string;
};

export type TaxReportWarning = AnyRecord & {
  code: string;
  severity: 'info' | 'warning' | 'critical' | string;
  message: string;
  count: number;
  amount?: number;
};

export type TaxReport = AnyRecord & {
  summary: TaxReportSummary;
  periods: TaxPeriodRow[];
  counterparties: TaxCounterpartyRow[];
  ledger: TaxLedgerRow[];
  warnings: TaxReportWarning[];
};

export type ProfitReportSummary = AnyRecord & {
  from: string;
  to: string;
  group_by: 'month' | 'quarter' | string;
  status: 'posted' | 'all' | string;
  project_id?: string;
  ledger_count: number;
  revenue_invoice_count: number;
  cost_invoice_count: number;
  revenue_amount: number;
  direct_cost_amount: number;
  indirect_cost_amount: number;
  total_cost_amount: number;
  gross_profit: number;
  net_profit: number;
  gross_margin_percent: number;
  net_margin_percent: number;
  unclassified_cost_count: number;
  excluded_unposted_count: number;
  excluded_unposted_total: number;
  warning_count: number;
};

export type ProfitPeriodRow = AnyRecord & {
  period_start: string;
  ledger_count: number;
  revenue_invoice_count: number;
  cost_invoice_count: number;
  revenue_amount: number;
  direct_cost_amount: number;
  indirect_cost_amount: number;
  total_cost_amount: number;
  gross_profit: number;
  net_profit: number;
  gross_margin_percent: number;
  net_margin_percent: number;
};

export type ProfitProjectRow = AnyRecord & {
  project_id: string;
  project_name: string;
  ledger_count: number;
  revenue_invoice_count: number;
  cost_invoice_count: number;
  revenue_amount: number;
  direct_cost_amount: number;
  indirect_cost_amount: number;
  total_cost_amount: number;
  gross_profit: number;
  net_profit: number;
  gross_margin_percent: number;
  net_margin_percent: number;
  warning_count: number;
};

export type ProfitCostBreakdownRow = AnyRecord & {
  cost_type: 'direct' | 'indirect' | string;
  cost_group: string;
  classification_reason: string;
  invoice_count: number;
  cost_amount: number;
  vat_amount: number;
  total_amount: number;
  warning_count: number;
};

export type ProfitLedgerRow = AnyRecord & {
  invoice_date: string;
  id: string;
  direction: 'revenue' | 'cost' | string;
  cost_type: 'revenue' | 'direct' | 'indirect' | string;
  cost_group: string;
  classification_reason: string;
  project_id: string;
  project_name?: string;
  invoice_number: string;
  invoice_symbol?: string;
  counterparty_name: string;
  tax_code?: string;
  amount_before_vat: number;
  vat_amount: number;
  total_amount: number;
  revenue_amount: number;
  direct_cost_amount: number;
  indirect_cost_amount: number;
  currency: 'VND' | string;
  status: string;
  source_type: string;
  source_id?: string;
  source_code?: string;
  secondary_source_type?: string;
  secondary_source_id?: string;
  secondary_source_code?: string;
  document_id?: string;
  document_name?: string;
  unclassified_cost?: boolean;
  warning_flags?: string;
};

export type ProfitReportWarning = AnyRecord & {
  code: string;
  severity: 'info' | 'warning' | 'critical' | string;
  message: string;
  count?: number;
  amount?: number;
};

export type ProfitReport = AnyRecord & {
  summary: ProfitReportSummary;
  periods: ProfitPeriodRow[];
  projects: ProfitProjectRow[];
  cost_breakdown: ProfitCostBreakdownRow[];
  ledger: ProfitLedgerRow[];
  warnings: ProfitReportWarning[];
};
