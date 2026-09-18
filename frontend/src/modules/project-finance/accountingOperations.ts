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
    snapshot: {
        code: string;
        title: string;
        contract_id?: string;
    };
    evidence_ids: string[];
};
export type AccountingRequest = {
    id: string;
    basis_id: string;
    project_id: string;
    title: string;
    kind: string;
    due_date: string;
    gross_amount: string;
    recovery_amount: string;
    retention_amount: string;
    amount: string;
    advance_request_id: string | null;
    release_request_id: string | null;
    note: string;
    evidence_ids: string[];
    stock_document_ids: string[];
    status: string;
    reason: string;
    revision: number;
    currency: string;
    direction: string;
    partner_name: string;
    partner_id: string;
    allocated_amount: string;
    uncovered_amount: string;
    fingerprint: string;
    reviewed: boolean;
};
export type MISARecord = {
    id: string;
    external_id: string;
    line_key: string;
    kind: string;
    direction: string;
    partner_id: string;
    partner_name: string;
    currency: string;
    amount: string;
    available_amount: string;
    record_date: string;
    active: boolean;
    entity_code: string;
    batch_id: string;
};
export type AccountingOperations = {
    project: {
        id: string;
        code: string;
        name: string;
    };
    bases: AccountingBasis[];
    requests: AccountingRequest[];
    records: MISARecord[];
    sources: {
        id: string;
        source_type: AccountingBasis['source_type'];
        code: string;
        title: string;
        currency: string;
    }[];
    stock_documents: {
        id: string;
        document_code: string;
    }[];
    evidence: {
        id: string;
        name: string;
        work_item_id: string;
    }[];
    batches: {
        id: string;
        entity_code: string;
        filename: string;
        status: string;
        created_at: string;
        confirmed_at: string;
        digest: string;
    }[];
    allocations: {
        id: string;
        request_id: string;
        record_id: string;
        amount: string;
        active: boolean;
        external_id: string;
        record_date: string;
        revoke_reason: string;
    }[];
    events: {
        id: string;
        request_id: string;
        revision: number;
        action: string;
        created_at: string;
        snapshot: AccountingRequest & {
            basis: AccountingBasis;
        };
    }[];
    reviews: {
        id: string;
        request_id: string;
        note: string;
        created_at: string;
        fingerprint: string;
    }[];
    capabilities: Record<string, boolean>;
    upload_targets: {
        id: string;
        title: string;
    }[];
};
export const requestStatuses: Record<string, string> = { draft: 'Nháp', submitted: 'Chờ tiếp nhận', returned: 'Yêu cầu bổ sung', acknowledged: 'Đã tiếp nhận', cancelled: 'Đã hủy' };
export const requestKinds: Record<string, string> = { advance: 'Tạm ứng', progress: 'Thanh toán khối lượng', final: 'Quyết toán', retention_release: 'Giải phóng giữ lại' };
export const misaCSVHeader = 'external_id,line_key,kind,direction,partner_id,currency,amount,record_date,replaces_id';
export function proposalMoneyInput(value:unknown){return String(value??'').replace(/(\.\d*?[1-9])0+$|\.0+$/, '$1')}
export function reconciliationLabel(r: Pick<AccountingRequest, 'reviewed' | 'allocated_amount' | 'uncovered_amount'>) {
    if (r.reviewed)
        return Number(r.uncovered_amount) === 0 ? 'Đã đối soát · đủ phân bổ' : 'Đã đối soát · còn chênh lệch';
    return Number(r.allocated_amount) > 0 ? (Number(r.uncovered_amount) === 0 ? 'Đủ phân bổ · chờ đối soát' : 'Khớp một phần') : 'Cần xử lý';
}
export function eligibleMISATransactions(records: MISARecord[], r: AccountingRequest) { return records.filter(m => m.active && m.kind === 'transaction' && m.partner_id === r.partner_id && m.currency === r.currency && m.direction === r.direction && Number(m.available_amount) > 0); }
