import type { AnyRecord } from './common';

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

export type AccountingHandoffTarget = {
  project_id: string;
  source_type: 'procurement_request' | 'contract_milestone';
  source_id: string;
};

export type AccountingHandoffSource = {
  source_code?: string;
  source_title?: string;
  counterparty_name?: string;
  partner_id?: string | null;
  reference_amount?: number | null;
  currency?: string;
  source_status?: string;
  order_reference?: string;
  contract_code?: string;
  readiness?: string;
  requirements?: AccountingHandoffRequirement[];
};

export type AccountingHandoffDetail = AccountingHandoffTarget & {
  project: { id: string; code: string; name: string };
  current: AccountingHandoffSource | null;
  handoff: null | {
    id: string; status: string; source_snapshot_json: AccountingHandoffSource;
    note?: string; external_reference?: string; return_reason?: string;
    handed_off_by_name?: string; handed_off_at?: string;
    acknowledged_by_name?: string; acknowledged_at?: string;
    returned_by_name?: string; returned_at?: string;
  };
  related: Array<{ kind: 'procurement_request' | 'contract' | 'incoming_acceptance' | 'acceptance' | 'stock_document'; id: string; label: string; status?: string }>;
  files: Array<{ kind: 'evidence' | 'document' | 'workflow_pdf'; id: string; name: string; mime_type?: string; file_size?: number; created_at?: string }>;
  restricted: boolean;
  source_access: boolean;
  handoff_access?: boolean;
  status: string;
  capabilities: { handoff: boolean; acknowledge: boolean; return: boolean };
};

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
