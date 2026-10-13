export type HandoverRow = {
  id: string;
  project_id: string;
  handover_code: string;
  status: string;
  final_acceptance_id?: string;
  final_acceptance_title?: string;
  handover_date?: string;
  handover_stage?: string;
  handover_conclusion?: 'eligible' | 'not_eligible' | '';
  handover_conclusion_by?: string;
  handover_conclusion_at?: string;
  client_representative: string;
  company_representative: string;
  notes?: string;
  checklist_total?: number;
  checklist_verified?: number;
  checklist_pending_required?: number;
  readiness_snapshot_json?: { ready?: boolean; blockers?: string[]; counts?: Record<string, number>; checked_at?: string };
  checked_at?: string;
  completed_at?: string;
  created_at?: string;
  updated_at?: string;
};

export type ChecklistItem = {
  requirement?: string;
  physical_result?: 'pass' | 'fail' | 'not_assessed';
  defect?: string;
  due_date?: string;
  id: string;
  item_code: string;
  category: string;
  title: string;
  required: boolean;
  status: string;
  source_type?: string;
  source_id?: string;
  evidence_id?: string;
  evidence_name?: string;
  evidence_status?: string;
  evidence_type?: string;
  evidence_mime_type?: string;
  evidence_file_size?: number;
  evidence_storage_provider?: string;
  document_id?: string;
  document_name?: string;
  document_status?: string;
  document_type?: string;
  document_file_url?: string;
  notes?: string;
  verified_by?: string;
  verified_at?: string;
  created_at?: string;
  updated_at?: string;
};

export type HandoverDetail = HandoverRow & {
  final_acceptance_status?: string;
  final_acceptance_result?: string;
  checklist_items: ChecklistItem[];
  evidence_options: { id: string; name: string; evidence_type: string }[];
  document_options: { id: string; name: string; document_type?: string }[];
};

export type FinalAcceptanceOption = {
  id: string;
  title: string;
  result: string;
  acceptance_date?: string;
  client_representative?: string;
  contractor_representative?: string;
};
