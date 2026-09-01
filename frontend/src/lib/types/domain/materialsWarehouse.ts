import type { ProjectOperationsRecord } from './commercial';
import type { AnyRecord } from './common';
import type { Partner } from './projectsWorkflowDefinitions';

export type MaterialCatalogItem = ProjectOperationsRecord & {
  code: string;
  name: string;
  unit: string;
  category?: string;
  default_supplier_partner_id?: string;
  default_supplier_partner_name?: string;
  default_supplier_tax_code?: string;
  default_supplier?: string;
  standard_price?: number;
  active?: boolean;
};

export type ProjectMaterialBalance = ProjectOperationsRecord & {
  material_id: string;
  code?: string;
  material_name?: string;
  unit?: string;
  category?: string;
  planned_quantity?: number;
  issued_quantity?: number;
  used_quantity?: number;
  surplus_quantity?: number;
  surplus_status?: string;
};

export type MaterialSnapshotItem = ProjectOperationsRecord & {
  snapshot_id?: string;
  work_item_id?: string;
  item_index: number;
  material_id?: string;
  material_code?: string;
  material_name: string;
  unit: string;
  source: 'site_balance' | 'warehouse_issue' | 'subcontractor' | 'other' | string;
  before_quantity: number;
  added_quantity: number;
  after_quantity: number;
  waste_quantity: number;
  used_quantity: number;
  affects_site_balance: boolean;
  site_balance_id?: string;
  before_evidence_id?: string;
  before_evidence_name?: string;
  before_evidence_status?: string;
  after_evidence_id?: string;
  after_evidence_name?: string;
  after_evidence_status?: string;
  construction_area?: string;
  condition: 'usable' | 'damaged' | 'mixed' | 'unknown' | string;
  comment?: string;
  payload_json?: AnyRecord;
};

export type MaterialSnapshotRecord = ProjectOperationsRecord & {
  project_id: string;
  work_item_id: string;
  status: 'draft' | 'submitted' | 'reopened' | string;
  snapshot_date?: string;
  construction_area?: string;
  summary_json?: AnyRecord;
  comment?: string;
  applied_to_site_balance?: boolean;
  submitted_by_name?: string;
  updated_by_name?: string;
  submitted_at?: string;
  items?: MaterialSnapshotItem[];
};

export type MaterialSnapshotResponse = {
  available: boolean;
  reason?: string;
  work_item_id?: string;
  project_id?: string;
  snapshot?: MaterialSnapshotRecord;
  materials?: MaterialCatalogItem[];
  material_balances?: ProjectMaterialBalance[];
};

export type Warehouse = ProjectOperationsRecord & {
  name: string;
  location?: string;
  type?: string;
  active?: boolean;
  submitted_by?: string;
  submitted_by_name?: string;
  submitted_by_account_deleted?: boolean;
  submitted_at?: string;
  updated_at?: string;
};

export type StockPosition = ProjectOperationsRecord & {
  warehouse_id: string;
  warehouse_name: string;
  warehouse_type?: string;
  location?: string;
  material_id: string;
  code: string;
  name: string;
  unit: string;
  category?: string;
  stock_quantity: number;
};

export type StockDocumentItem = ProjectOperationsRecord & {
  document_id?: string;
  material_id: string;
  material_code?: string;
  material_name?: string;
  quantity: number;
  unit: string;
  reason?: string;
  source_line_type?: string;
  source_line_id?: string;
  movement_code?: string;
};

export type StockDocument = ProjectOperationsRecord & {
  document_code: string;
  project_id?: string;
  project_name?: string;
  movement_type: 'receive' | 'issue' | 'transfer' | 'return_to_warehouse' | 'return_to_subcontractor' | string;
  source_warehouse_id?: string;
  source_warehouse_name?: string;
  target_warehouse_id?: string;
  target_warehouse_name?: string;
  reference_type?: string;
  reference_id?: string;
  document_date: string;
  note?: string;
  status: string;
  correction_of_document_id?: string;
  corrected_by_document_id?: string;
  corrected_document_id?: string;
  correction_reason?: string;
  corrected_at?: string;
  item_count?: number;
  total_quantity?: number;
  created_by_name?: string;
  items?: StockDocumentItem[];
};

export type WarehouseReceiptOption = ProjectOperationsRecord & {
  incoming_acceptance_id: string;
  acceptance_code: string;
  procurement_request_id?: string;
  request_code?: string;
  supplier_name?: string;
  received_at?: string;
  result?: string;
  warehouse_id?: string;
  incoming_acceptance_item_id: string;
  material_id: string;
  material_code?: string;
  material_name: string;
  accepted_quantity: number;
  stock_received_quantity: number;
  remaining_quantity: number;
  unit: string;
};

export type SurplusHandlingRecord = ProjectOperationsRecord & {
  handling_code: string;
  work_item_id: string;
  site_balance_id: string;
  material_id: string;
  material_code?: string;
  material_name?: string;
  material_unit?: string;
  material_snapshot_id?: string;
  material_snapshot_item_id?: string;
  stock_movement_id?: string;
  stock_movement_code?: string;
  stock_movement_type?: string;
  status: 'draft' | 'submitted' | 'cancelled' | string;
  handling_type: 'return_to_warehouse' | 'return_to_subcontractor' | string;
  quantity: number;
  unit: string;
  warehouse_id?: string;
  warehouse_name?: string;
  custody_location?: string;
  condition: 'usable' | 'damaged' | 'mixed' | 'unknown' | string;
  evidence_id?: string;
  evidence_name?: string;
  evidence_status?: string;
  comment?: string;
  submitted_by?: string;
  submitted_at?: string;
};

export type SurplusHandlingResponse = {
  available: boolean;
  work_item_id?: string;
  project_id?: string;
  material_balances?: ProjectMaterialBalance[];
  warehouses?: Warehouse[];
  handlings?: SurplusHandlingRecord[];
  handling_options?: Array<'return_to_warehouse' | 'return_to_subcontractor' | string>;
};

export type IncomingAcceptanceItem = ProjectOperationsRecord & {
  incoming_acceptance_id?: string;
  project_id?: string;
  work_item_id?: string;
  item_index: number;
  material_id?: string;
  material_code?: string;
  material_name: string;
  procurement_request_item_id?: string;
  delivered_quantity: number;
  accepted_quantity: number;
  rejected_quantity: number;
  unit: string;
  condition: 'usable' | 'damaged' | 'mixed' | 'unknown' | string;
  defect_note?: string;
  evidence_id?: string;
  evidence_name?: string;
  evidence_status?: string;
  decision: 'pending' | 'accepted' | 'conditional_acceptance' | 'rejected' | string;
  comment?: string;
  procurement_quantity_requested?: number;
  procurement_delivery_status?: string;
  procurement_approval_status?: string;
};

export type IncomingAcceptanceRecord = ProjectOperationsRecord & {
  acceptance_code: string;
  project_id: string;
  work_item_id?: string;
  procurement_request_id?: string;
  procurement_request_code?: string;
  procurement_request_title?: string;
  warehouse_id?: string;
  warehouse_name?: string;
  source_type: 'supplier' | 'subcontractor' | 'procurement' | 'other' | string;
  supplier_partner_id?: string;
  supplier_name?: string;
  subcontractor_partner_id?: string;
  subcontractor_name?: string;
  delivery_reference?: string;
  status: 'draft' | 'submitted' | 'cancelled' | string;
  result?: 'pass' | 'conditional_pass' | 'fail' | string;
  received_at?: string;
  summary_json?: AnyRecord;
  comment?: string;
  item_count?: number;
  submitted_by_name?: string;
  submitted_at?: string;
  items?: IncomingAcceptanceItem[];
};

export type ProcurementRequestOption = ProjectOperationsRecord & {
  project_id: string;
  request_code?: string;
  title?: string;
  status?: string;
  item_count?: number;
};

export type ProcurementRequestItemOption = ProjectOperationsRecord & {
  request_id: string;
  request_code?: string;
  procurement_request_title?: string;
  material_id?: string;
  material_code?: string;
  material_name?: string;
  work_item_name?: string;
  quantity_requested?: number;
  unit?: string;
  delivery_status?: string;
  approval_status?: string;
};

export type IncomingAcceptanceResponse = {
  available: boolean;
  work_item_id?: string;
  project_id?: string;
  records?: IncomingAcceptanceRecord[];
  materials?: MaterialCatalogItem[];
  warehouses?: Warehouse[];
  partners?: Partner[];
  procurement_requests?: ProcurementRequestOption[];
  procurement_request_items?: ProcurementRequestItemOption[];
};
