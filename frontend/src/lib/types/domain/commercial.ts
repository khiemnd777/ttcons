import type { AnyRecord } from './common';

export type ProjectOperationsRecord = AnyRecord & {
  id: string;
  project_id?: string;
  created_at?: string;
  updated_at?: string;
  actor_account_deleted?: boolean;
  created_by_account_deleted?: boolean;
  submitted_by_account_deleted?: boolean;
  updated_by_account_deleted?: boolean;
  requested_by_account_deleted?: boolean;
  reviewed_by_account_deleted?: boolean;
  approved_by_account_deleted?: boolean;
  uploaded_by_account_deleted?: boolean;
  checked_by_account_deleted?: boolean;
  assigned_user_account_deleted?: boolean;
};

export type BOQUnitPriceEvent = ProjectOperationsRecord & {
  boq_item_id: string;
  event_type: 'expected_updated' | 'norm_locked' | 'final_locked' | string;
  price_source: 'expected' | 'norm' | 'final' | string;
  from_unit_price?: number;
  to_unit_price: number;
  revision_no: number;
  comment?: string;
  actor_name?: string;
  actor_account_deleted?: boolean;
};

export type BOQItem = ProjectOperationsRecord & {
  category: string;
  work_item_name: string;
  description?: string;
  unit: string;
  quantity: number;
  unit_price: number;
  expected_unit_price: number;
  norm_unit_price?: number;
  final_unit_price?: number;
  effective_unit_price?: number;
  total_amount?: number;
  effective_total_amount?: number;
  material_id?: string;
  material_name?: string;
  status: string;
  unit_price_source?: 'expected' | 'norm' | 'final' | string;
  price_lifecycle_status?: 'expected' | 'norm_locked' | 'final_locked' | string;
  norm_price_locked?: boolean;
  norm_price_locked_by?: string;
  norm_price_locked_at?: string;
  norm_price_comment?: string;
  final_price_locked?: boolean;
  final_price_locked_by?: string;
  final_price_locked_at?: string;
  final_price_comment?: string;
  price_revision_no?: number;
  price_events?: BOQUnitPriceEvent[];
  price_event_count?: number;
  price_events_truncated?: boolean;
  selected_quotation_item_id?: string;
  selected_quotation_code?: string;
  selected_supplier_partner_id?: string;
  selected_quotation_id?: string;
  selected_supplier_name?: string;
  quoted_unit_price?: number;
  price_waiver_reason?: string;
  procurement_request_id?: string;
  procurement_request_code?: string;
  procurement_request_status?: string;
  has_active_procurement?: boolean;
  active_procurement_count?: number;
};

export type BOQProcurementResult = {
  id?: string;
  request_code?: string;
  status?: string;
  requests?: Array<{ id: string; request_code?: string; status?: string; supplier_name?: string }>;
  created_requests?: Array<{ id: string; request_code?: string; status?: string; supplier_name?: string }>;
};

export type SupplierQuotationItem = ProjectOperationsRecord & {
  quotation_id: string;
  boq_item_id: string;
  material_id?: string;
  description: string;
  unit: string;
  quantity: number;
  unit_price: number;
  discount_amount: number;
  tax_rate: number;
  amount_before_tax: number;
  total_amount: number;
  lead_time_days: number;
  compliance_status: string;
  deviation_note?: string;
  work_item_name?: string;
  variance_to_norm?: number;
};

export type SupplierQuotation = ProjectOperationsRecord & {
  quotation_code: string;
  supplier_partner_id: string;
  supplier_name: string;
  supplier_tax_code?: string;
  quotation_date: string;
  valid_until?: string;
  currency: string;
  payment_terms_days: number;
  lead_time_days: number;
  delivery_terms?: string;
  warranty_terms?: string;
  status: string;
  technical_status: string;
  commercial_status: string;
  notes?: string;
  item_count?: number;
  amount_before_tax?: number;
  total_amount?: number;
  items?: SupplierQuotationItem[];
};

export type SupplierQuoteComparison = {
  boq_item_id: string;
  status: string;
  category: string;
  work_item_name: string;
  unit: string;
  quantity: number;
  expected_unit_price: number;
  norm_unit_price?: number;
  final_unit_price?: number;
  norm_price_locked: boolean;
  final_price_locked: boolean;
  selected_quotation_item_id?: string;
  price_waiver_reason?: string;
  quotations: Array<{
    quotation_item_id: string;
    quotation_id: string;
    quotation_code: string;
    supplier_partner_id: string;
    supplier_name: string;
    quotation_status: string;
    valid_until?: string;
    unit_price: number;
    discount_amount: number;
    tax_rate: number;
    total_amount: number;
    lead_time_days: number;
    compliance_status: string;
    variance_to_norm: number;
    is_selected: boolean;
  }>;
};

export type ContractVersion = ProjectOperationsRecord & {
  contract_id: string;
  version_no: number;
  version_label: string;
  source_variation_id?: string;
  amount: number;
  scope_summary: string;
  commercial_terms_json?: Record<string, any>;
  payment_terms_json?: Record<string, any>;
  planned_start_date?: string;
  planned_end_date?: string;
  document_id?: string;
  definition_hash: string;
  status: string;
  review_comment?: string;
  signature_comment?: string;
};

export type ContractMilestone = ProjectOperationsRecord & {
  contract_id: string;
  contract_version_id: string;
  milestone_code: string;
  milestone_name: string;
  sequence_no: number;
  milestone_type: string;
  amount_basis: string;
  percentage?: number;
  gross_amount: number;
  retention_percent: number;
  retention_amount: number;
  net_amount: number;
  planned_date?: string;
  due_days: number;
  acceptance_required: boolean;
  acceptance_id?: string;
  status: string;
  obligation_type?: string;
  obligation_id?: string;
};

export type ContractVariation = ProjectOperationsRecord & {
  contract_id: string;
  variation_code: string;
  title: string;
  variation_type: string;
  reason: string;
  amount_delta: number;
  days_delta: number;
  scope_delta?: string;
  status: string;
  resulting_version_id?: string;
  priority: 'normal' | 'urgent' | string;
  source_work_item_id?: string;
	  source_work_item_title?: string;
	  source_construction_action_id?: string;
	  source_construction_action_title?: string;
  procurement_request_id?: string;
  qs_comment?: string;
  purchasing_comment?: string;
  events?: AnyRecord[];
};

export type ContractWorkflowSignature = AnyRecord & {
  workflow_form_signature_id: string;
  contract_version_id: string;
  form_instance_id: string;
  signer_name: string;
  signed_at: string;
  payload_hash: string;
};

export type ContractSigningForm = AnyRecord & {
  id: string;
  title: string;
  status: string;
  current_revision?: number;
  work_item_id: string;
  signatures?: Array<AnyRecord & { id: string; purpose: string; signer_user_id?: string; linked_contract_version_id?: string }>;
};

export type ContractDispatch = AnyRecord & {
  id: string;
  contract_id: string;
  contract_version_id: string;
  dispatch_no: number;
  recipient_name: string;
  delivery_method: string;
  tracking_reference?: string;
  status: 'sent' | 'received' | string;
  sent_at: string;
  received_at?: string;
  returned_scan_document_id?: string;
  returned_scan_document_name?: string;
};

export type ContractRetention = ProjectOperationsRecord & {
  contract_id: string;
  source_milestone_id: string;
  milestone_code: string;
  milestone_name: string;
  direction: string;
  held_amount: number;
  released_amount: number;
  outstanding_amount: number;
  warranty_end_date?: string;
  status: string;
  releases?: Array<Record<string, any>>;
};

export type Contract = ProjectOperationsRecord & {
  contract_code: string;
  title: string;
  contract_type: string;
  direction: string;
  counterparty_partner_id?: string;
  counterparty_name: string;
  currency: string;
  original_amount: number;
  current_amount: number;
  advance_percent: number;
  retention_percent: number;
  warranty_days: number;
  signed_date?: string;
  effective_date?: string;
  expiry_date?: string;
  status: string;
  current_version_id?: string;
  current_version_no?: number;
  current_version_label?: string;
  current_version_status?: string;
  milestone_count?: number;
  variation_count?: number;
  signed_scan_received?: boolean;
  workflow_signatures?: ContractWorkflowSignature[];
	signing_forms?: ContractSigningForm[];
	dispatches?: ContractDispatch[];
	execution_readiness?: {
		app_signed: boolean;
		paper_dispatched: boolean;
		returned_scan_received: boolean;
		contract_active: boolean;
	};
  retention_outstanding?: number;
  versions?: ContractVersion[];
  milestones?: ContractMilestone[];
  variations?: ContractVariation[];
  retentions?: ContractRetention[];
};
