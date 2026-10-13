/** Persisted record identities only. Display codes and names never resolve an ID. */
export const recordKinds = {
  boq_item: { label: 'Dòng BOQ', permissions: ['boq.view'] },
  procurement_request: { label: 'Yêu cầu mua', permissions: ['procurement.view'] },
  work_item: { label: 'Đầu việc', permissions: [] },
  acceptance: { label: 'Nghiệm thu', permissions: ['acceptance.view'] },
  handover: { label: 'Bàn giao', permissions: ['handover.view'] },
  stock_movement: { label: 'Biến động kho', permissions: ['warehouse.view'] },
  rework: { label: 'Yêu cầu sửa lỗi', permissions: ['rework.view'] },
  supplier_quotation: { label: 'Báo giá', permissions: ['boq.view'] },
  stock_document: { label: 'Phiếu kho', permissions: ['warehouse.view'] },
  partner: { label: 'Đối tác', permissions: ['catalog.view'] },
  replanning: { label: 'Lập kế hoạch lại', permissions: ['replanning.view'] },
  planning_daily_report: { label: 'Báo cáo ngày kế hoạch', permissions: ['planning.view', 'pipeline.view'] },
  meeting_minutes: { label: 'Biên bản họp', permissions: ['planning.view', 'pipeline.view'] },
  planning_checklist: { label: 'Checklist kế hoạch', permissions: ['planning.view', 'pipeline.view'] },
  role: { label: 'Vai trò', permissions: ['role.view'] },
  audit: { label: 'Sự kiện hệ thống', permissions: ['audit.view'] },
  incoming_acceptance: { label: 'Nghiệm thu đầu vào', permissions: ['acceptance.view', 'operation.view', 'warehouse.view'] },
  contract: { label: 'Hợp đồng', permissions: ['contract.view'] },
  daily_report: { label: 'Báo cáo thi công', permissions: ['operation.view'] },
  action: { label: 'Việc cần xử lý', permissions: ['operation.view', 'quality.view', 'rework.view'] },
  issue: { label: 'Vấn đề công trình', permissions: ['operation.view'] },
} as const;

export type RecordKind = keyof typeof recordKinds;
export type RecordTarget = { kind: RecordKind; id: string; projectId?: string; label?: string; partnerGroup?: 'input' | 'output'; workItemId?: string };
export const isRecordID = (value: unknown): value is string => typeof value === 'string' && /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(value);
export const isRecordKind = (value: unknown): value is RecordKind => typeof value === 'string' && Object.prototype.hasOwnProperty.call(recordKinds, value);
const globalKinds: RecordKind[] = ['partner', 'role', 'audit', 'stock_document'];

export function validRecordTarget(target: RecordTarget): boolean {
  return isRecordKind(target.kind) && isRecordID(target.id)
    && (target.projectId == null || target.projectId === '' || isRecordID(target.projectId))
    && (globalKinds.includes(target.kind) || isRecordID(target.projectId));
}

export function recordPath(target: RecordTarget): string | null {
  if (!validRecordTarget(target)) return null;
  const project = target.projectId ? `/projects/${target.projectId}` : '';
  const params = new URLSearchParams({ record: target.id, record_type: target.kind });
  if (target.projectId) params.set('project_id', target.projectId);
  const paths: Partial<Record<RecordKind, string>> = {
    boq_item: `${project}/boq`, supplier_quotation: `${project}/boq`, procurement_request: `${project}/procurement`,
    acceptance: `${project}/acceptance`, handover: `${project}/handover`, stock_document: `${project}/warehouse`,
    rework: '/rework', replanning: '/rework',
    partner: `/core-data/${target.partnerGroup ?? 'output'}`, role: '/roles', audit: `${project}/audit`, contract: `${project}/contracts`,
    daily_report: `${project}/operations`, action: `${project}/operations`,
  };
  if (target.kind === 'work_item') return `${project}/work-items/${target.id}`;
  if (['planning_daily_report', 'meeting_minutes', 'planning_checklist'].includes(target.kind)) {
    return isRecordID(target.workItemId) ? `${project}/work-items/${target.workItemId}` : null;
  }
  if (target.kind === 'stock_document') params.set('tab', 'documents');
  if (target.kind === 'supplier_quotation') params.set('tab', 'quotes');
  if (target.kind === 'daily_report') params.set('tab', 'timeline');
  if (target.kind === 'action') params.set('tab', 'actions');
  const path = paths[target.kind];
  return path ? `${path}?${params}` : `/source-records/${target.kind}/${target.id}?project_id=${target.projectId}`;
}

export function canOpenRecord(target: RecordTarget, hasPermission: (permission: string) => boolean): boolean {
  const path = recordPath(target);
  if (!path) return false;
  if (['role', 'audit'].includes(target.kind) && hasPermission('project.reconciliation.view')) return false;
  if (path.startsWith('/projects/') && !hasPermission('project.view')) return false;
  const permissions: readonly string[] = recordKinds[target.kind].permissions;
  return !permissions.length || permissions.some(hasPermission);
}

export class RecordUnavailable extends Error {
  constructor() { super('Không tìm thấy hồ sơ hoặc hồ sơ không còn khả dụng.'); }
}

/** These reads keep the source endpoint's authorization and never execute a mutation. */
export function recordRead(target: RecordTarget): { path: string; collection?: 'array' | 'items' | 'actions' } {
  if (!validRecordTarget(target)) throw new RecordUnavailable();
  const id = target.id;
  const project = target.projectId;
  const direct: Partial<Record<RecordKind, string>> = {
    boq_item: '/boq-items', procurement_request: '/procurement-requests', work_item: '/work-items', acceptance: '/acceptances',
    handover: '/handover', supplier_quotation: '/supplier-quotations', stock_document: '/stock/documents', incoming_acceptance: '/incoming-acceptances', contract: '/contracts',
  };
  if (direct[target.kind]) return { path: `${direct[target.kind]}/${id}` };
  if (target.kind === 'audit') return { path: `/audit-logs?record_id=${id}${project ? `&project_id=${project}` : ''}`, collection: 'items' };
  if (target.kind === 'action') return {path: `/projects/${project}/construction-workspace`, collection: 'actions'};
  const lists: Partial<Record<RecordKind, string>> = {
    partner: '/partners?status=all', role: '/roles', stock_movement: `/stock/movements?project_id=${project}`,
    rework: `/projects/${project}/reworks`, replanning: `/projects/${project}/replannings`, issue: `/projects/${project}/issues`,
    daily_report: `/projects/${project}/daily-reports`,
  };
  if (!lists[target.kind]) throw new RecordUnavailable();
  return { path: lists[target.kind]!, collection: 'array' };
}

export function selectSourceRecord(target: RecordTarget, payload: unknown): Record<string, any> {
  const { collection } = recordRead(target);
  const matches = (row: any) => row && !Array.isArray(row) && typeof row === 'object'
    && String(row.id).toLowerCase() === target.id.toLowerCase()
    && (['partner', 'role'].includes(target.kind) || !target.projectId || String(row.project_id).toLowerCase() === target.projectId.toLowerCase())
    && !['cancelled', 'deleted'].includes(row.status) && !row.deleted_at;
  const rows = collection === 'items' ? (payload as { items?: unknown[] })?.items : collection === 'actions' ? (payload as {actions?: unknown[]})?.actions : payload;
  const row = collection ? Array.isArray(rows) ? rows.find(matches) : undefined : matches(payload) ? payload : undefined;
  if (!row) throw new RecordUnavailable();
  return row as Record<string, any>;
}

export async function loadSourceRecord(target: RecordTarget, read: (path: string) => Promise<unknown>) {
  return selectSourceRecord(target, await read(recordRead(target).path));
}

/** Known persisted source types only; unknown historical types stay readable without a guessed URL. */
export function typedRecordTarget(type: unknown, id: unknown, projectId?: string): RecordTarget | null {
  const aliases: Record<string, RecordKind> = {
    boq: 'boq_item', boq_item: 'boq_item', procurement: 'procurement_request', procurement_request: 'procurement_request',
    acceptance: 'acceptance', acceptance_record: 'acceptance', incoming_acceptance: 'incoming_acceptance', handover: 'handover',
    stock_document: 'stock_document', stock_movement: 'stock_movement', work_item: 'work_item', rework: 'rework', rework_request: 'rework',
    replanning: 'replanning', replanning_request: 'replanning', supplier_quotation: 'supplier_quotation', partner: 'partner', role: 'role',
    contract: 'contract', issue: 'issue',
  };
  const kind = aliases[String(type)];
  return kind && isRecordID(id) ? { kind, id, projectId } : null;
}

export function relatedRecordTargets(kind: RecordKind, row: Record<string, any>): RecordTarget[] {
  const result: RecordTarget[] = [];
  const add = (target: RecordTarget | null) => {
    if (target && validRecordTarget(target) && !(target.kind === kind && target.id === row.id)
      && !result.some(item => item.kind === target.kind && item.id === target.id)) result.push(target);
  };
  const field = (targetKind: RecordKind, key: string, label?: string, partnerGroup?: 'input' | 'output') => {
    if (isRecordID(row[key])) add({ kind: targetKind, id: row[key], projectId: row.project_id, label, partnerGroup });
  };
  field('work_item', 'work_item_id'); field('work_item', 'source_work_item_id');
  field('work_item', 'incoming_acceptance_work_item_id', 'Đầu việc nghiệm thu đầu vào');
  field('supplier_quotation', 'selected_quotation_id', row.selected_quotation_code);
  field('procurement_request', 'procurement_request_id', row.procurement_request_code);
  field('boq_item', 'boq_item_id', row.work_item_name);
  field('partner', 'supplier_partner_id', row.supplier_name, 'output');
  field('partner', 'selected_supplier_partner_id', row.selected_supplier_name, 'output');
  field('partner', 'vendor_partner_id', row.vendor_name, 'output');
  field('partner', 'client_partner_id', row.client_name, 'input');
  field('acceptance', 'final_acceptance_id', row.final_acceptance_title);
  field('acceptance', 'acceptance_id'); field('issue', 'issue_id');
  field('stock_document', 'stock_document_id', row.document_code);
  field('stock_document', 'correction_of_document_id', 'Phiếu trước điều chỉnh');
  field('stock_document', 'corrected_by_document_id', 'Phiếu điều chỉnh thay thế');
  field('contract', 'contract_id');
  add(typedRecordTarget(row.reference_type, row.reference_id, row.project_id));
  add(typedRecordTarget(row.source_type, row.source_id, row.project_id));
  if (kind === 'audit') add(typedRecordTarget(row.target_type, row.target_id, row.project_id));
  return result;
}
