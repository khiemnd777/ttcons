import type { AnyRecord } from './common';

export type ProjectFinanceSummary = AnyRecord & {
  project_id: string;
  project_name: string;
  project_code: string;
  contract_value: number | null;
  eligible_revenue: number | null;
  committed_material_cost: number | null;
  confirmed_material_cost: number | null;
  provisional_profit: number | null;
  unconfirmed_cost_count: number;
  unconfirmed_cost_amount: number | null;
  open_cash_request_count: number;
  open_inbound_amount: number | null;
  open_outbound_amount: number | null;
  scope_warning: string;
};

export type ProjectMaterialCost = AnyRecord & {
  id: string;
  project_id: string;
  vendor_name: string;
  description: string;
  total_amount: number;
  recognized_cost_amount: number;
  status: 'draft' | 'submitted' | 'needs_changes' | 'confirmed' | 'cancelled' | string;
};

export type ProjectCashRequest = AnyRecord & {
  id: string;
  project_id: string;
  request_code: string;
  direction: 'inbound' | 'outbound';
  title: string;
  counterparty_name: string;
  amount: number;
  status: 'draft' | 'submitted' | 'approved' | 'handed_to_accounting' | 'needs_changes' | 'processed' | 'cancelled' | string;
};

export type ProjectFinanceWorkspace = {
  mode?: 'legacy';
  can_view_report: boolean;
  summary: ProjectFinanceSummary;
  tasks: AnyRecord[];
  material_costs: ProjectMaterialCost[];
  cash_requests: ProjectCashRequest[];
};

export type AccountingHandoffRequirement = {
  key: string;
  label: string;
  met: boolean;
  guidance: string;
};

export type AccountingHandoffItem = AnyRecord & {
  source_id: string;
  source_type: 'procurement_request' | 'contract_milestone';
  direction: 'payable' | 'receivable';
  source_code: string;
  source_title: string;
  counterparty_name: string;
  reference_amount: number;
  currency: string;
  source_route: string;
  readiness: 'needs_attention' | 'ready';
  requirements: AccountingHandoffRequirement[];
  handoff_id?: string | null;
  handoff_status?: 'handed_off' | 'acknowledged' | 'returned' | 'cancelled' | null;
  status: 'needs_attention' | 'ready' | 'handed_off' | 'acknowledged' | 'returned';
  note?: string;
  return_reason?: string;
  external_reference?: string;
};

export type AccountingHandoffWorkspace = {
  mode: 'handoff';
  summary: {
    needs_attention_count: number;
    ready_count: number;
    handed_off_count: number;
    acknowledged_count: number;
    returned_count: number;
  };
  items: AccountingHandoffItem[];
};

export type AccountingWorkStatus = 'handed_off' | 'returned' | 'acknowledged' | 'all';

export type AccountingWorkItem = AnyRecord & {
  id: string;
  handoff_id: string;
  project_id: string;
  project_code: string;
  project_name: string;
  source_id: string;
  source_type: 'procurement_request' | 'contract_milestone';
  direction: 'payable' | 'receivable';
  source_code: string;
  source_title: string;
  counterparty_name: string;
  reference_amount: number;
  currency: string;
  source_route: string;
  status: Exclude<AccountingWorkStatus, 'all'>;
  note?: string;
  return_reason?: string;
  external_reference?: string;
  handed_off_at?: string;
  acknowledged_at?: string;
  returned_at?: string;
  handed_off_by_name?: string;
  acknowledged_by_name?: string;
  returned_by_name?: string;
};

export type AccountingWorkPage = {
  items: AccountingWorkItem[];
  summary: {
    handed_off_count: number;
    returned_count: number;
    acknowledged_count: number;
    total: number;
  };
  total: number;
  has_more: boolean;
  next_cursor?: string;
};

export type ProjectFinanceOptions = {
  procurements: AnyRecord[];
  incoming_acceptances: AnyRecord[];
  stock_documents: AnyRecord[];
  partners: AnyRecord[];
};
