export type AccountingBasis = {
    id: string;
    source_type: 'procurement_request' | 'contract_milestone';
    source_id: string;
    amount: string;
    currency: string;
    direction: string;
    partner_id: string;
    partner_name: string;
    note: string;
    snapshot: { code: string; title: string; contract_id?: string; reference_amount?: string };
    evidence_ids: string[];
    allocated_amount: string;
    difference_amount: string;
    fingerprint: string;
    reviewed: boolean;
    source_available?: boolean;
    source_unavailable_reason?: string;
};

export type MISARecord = {
    id: string; external_id: string; line_key: string; kind: string;
    direction: string; partner_id: string; partner_name: string; currency: string;
    amount: string; available_amount: string; record_date: string; active: boolean;
    entity_code: string; batch_id: string;
};

export type AccountingOperations = {
    project: { id: string; code: string; name: string };
    bases: AccountingBasis[];
    records: MISARecord[];
    sources: { id: string; source_type: AccountingBasis['source_type']; code: string; title: string; currency: string; reference_amount: string; direction: string }[];
    stock_documents: { id: string; document_code: string }[];
    evidence: { id: string; name: string; work_item_id: string }[];
    batches: { id: string; entity_code: string; filename: string; status: string; created_at: string; confirmed_at: string; digest: string }[];
    allocations: { id: string; basis_id: string; record_id: string; amount: string; active: boolean; external_id: string; record_date: string; revoke_reason: string }[];
    events: { id: string; basis_id: string | null; action: string; created_at: string; snapshot: Record<string, unknown> }[];
    reviews: { id: string; basis_id: string; note: string; created_at: string; fingerprint: string }[];
    capabilities: Record<string, boolean>;
    upload_targets: { id: string; title: string }[];
};

export const misaCSVHeader = 'external_id,line_key,kind,direction,partner_id,currency,amount,record_date,replaces_id';
export function reconciliationLabel(basis: Pick<AccountingBasis, 'reviewed' | 'allocated_amount' | 'difference_amount'>) {
    const difference = Number(basis.difference_amount);
    if (basis.reviewed) return difference === 0 ? 'Đã đối soát · khớp giá trị' : 'Đã đối soát · có chênh lệch';
    if (Number(basis.allocated_amount) === 0) return 'Chưa ghép giao dịch';
    return difference === 0 ? 'Khớp giá trị · chờ đối soát' : difference < 0 ? 'Khớp một phần' : 'Ghép vượt giá trị căn cứ';
}
export function eligibleMISATransactions(records: MISARecord[], basis: AccountingBasis) {
    return records.filter(record => record.active && record.kind === 'transaction'
        && record.partner_id === basis.partner_id && record.currency === basis.currency
        && record.direction === basis.direction && Number(record.available_amount) > 0);
}
