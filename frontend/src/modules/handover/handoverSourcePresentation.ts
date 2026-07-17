import type { ChecklistItem, HandoverDetail } from './handoverTypes';

export type ChecklistSourceMode = 'automatic' | 'linked' | 'manual';
export type SourceResult = 'passed' | 'failed' | 'unchecked';

export type SourceMetric = {
  label: string;
  value?: number;
  unit: 'count' | 'currency';
  suffix?: string;
};

export type LinkedSourcePresentation = {
  kind: 'evidence' | 'document';
  id: string;
  name: string;
  typeLabel: string;
  status?: string;
  openMode: 'protected_evidence' | 'external_document' | 'none';
  targetUrl?: string;
  placeholderUrl: boolean;
  mimeType?: string;
  fileSize?: number;
};

export type AutomaticSourcePresentation = {
  itemCode: string;
  sourceLabel: string;
  moduleLabel: string;
  criterion: string;
  targetPath: string;
  targetLabel: string;
  permissions: string[];
  checkedAt?: string;
  result: SourceResult;
  metrics: SourceMetric[];
  linkedRecord?: {
    label: string;
    title: string;
    status?: string;
    result?: string;
  };
};

type AutomaticSourceDefinition = Omit<AutomaticSourcePresentation, 'checkedAt' | 'result' | 'metrics' | 'linkedRecord'> & {
  countKey: string;
  amountKey?: string;
  metricLabel: string;
  metricSuffix: string;
};

const checklistItemCodeLabels: Record<string, string> = {
  final_acceptance: 'Nghiệm thu cuối',
  as_built_documents: 'Hồ sơ hoàn công',
  warehouse_closure: 'Đóng kho và xử lý vật tư dư',
  operation_manuals: 'Tài liệu vận hành và bảo trì',
  warranty_documents: 'Hồ sơ bảo hành',
  ap_reconciliation: 'Đối soát công nợ phải trả',
  ar_reconciliation: 'Đối soát công nợ phải thu',
  signed_handover_minutes: 'Biên bản bàn giao hai bên',
};

export function checklistItemCodeLabel(itemCode: string) {
  return checklistItemCodeLabels[itemCode] ?? 'Nội dung khác';
}

export function checklistItemCodeTooltip(itemCode: string) {
  return `Mã kỹ thuật: ${itemCode}`;
}

export function checklistSourceMode(item: ChecklistItem): ChecklistSourceMode {
  if (isAutomaticChecklistItem(item)) return 'automatic';
  if (item.evidence_id || item.document_id || ['document', 'evidence'].includes(String(item.source_type ?? ''))) return 'linked';
  return 'manual';
}

export function isAutomaticChecklistItem(item: ChecklistItem) {
  return item.item_code === 'final_acceptance' || item.source_type === 'system_check';
}

export function checklistSourceLabel(item: ChecklistItem, projectId: string) {
  if (isAutomaticChecklistItem(item)) return automaticSourceDefinition(item, projectId)?.sourceLabel ?? 'Kiểm tra hệ thống';
  if (item.evidence_name) return item.evidence_name;
  if (item.document_name) return item.document_name;
  if (item.evidence_id || item.source_type === 'evidence') return 'Minh chứng liên kết';
  if (item.document_id || item.source_type === 'document') return 'Tài liệu liên kết';
  return '-';
}

export function linkedSourcePresentations(item: ChecklistItem): LinkedSourcePresentation[] {
  const sources: LinkedSourcePresentation[] = [];
  if (item.evidence_id) {
    sources.push({
      kind: 'evidence',
      id: item.evidence_id,
      name: item.evidence_name || 'Minh chứng liên kết',
      typeLabel: evidenceTypeLabel(item.evidence_type),
      status: item.evidence_status,
      openMode: item.evidence_storage_provider === 'local' ? 'protected_evidence' : 'none',
      placeholderUrl: false,
      mimeType: item.evidence_mime_type,
      fileSize: item.evidence_file_size,
    });
  }
  if (item.document_id) {
    const targetUrl = item.document_status === 'active' ? safeExternalUrl(item.document_file_url) : undefined;
    sources.push({
      kind: 'document',
      id: item.document_id,
      name: item.document_name || 'Tài liệu liên kết',
      typeLabel: documentTypeLabel(item.document_type),
      status: item.document_status,
      openMode: targetUrl ? 'external_document' : 'none',
      targetUrl,
      placeholderUrl: isPlaceholderUrl(targetUrl),
    });
  }
  return sources;
}

export function automaticSourcePresentation(item: ChecklistItem, detail: HandoverDetail): AutomaticSourcePresentation | null {
  const definition = automaticSourceDefinition(item, detail.project_id);
  if (!definition) return null;

  const counts = detail.readiness_snapshot_json?.counts;
  const checkedAt = detail.readiness_snapshot_json?.checked_at || detail.checked_at || item.verified_at;
  const countValue = numericSnapshotValue(counts?.[definition.countKey]);
  const amountValue = definition.amountKey ? numericSnapshotValue(counts?.[definition.amountKey]) : undefined;
  const metrics: SourceMetric[] = [{ label: definition.metricLabel, value: countValue, unit: 'count', suffix: definition.metricSuffix }];
  if (definition.amountKey) metrics.push({ label: definition.itemCode === 'ap_reconciliation' ? 'Tổng còn phải trả' : 'Tổng còn phải thu', value: amountValue, unit: 'currency' });

  const result: SourceResult = item.status === 'verified' ? 'passed' : checkedAt ? 'failed' : 'unchecked';
  const linkedRecord = definition.itemCode === 'final_acceptance'
    ? {
        label: 'Biên bản liên kết hiện tại',
        title: detail.final_acceptance_title || 'Nghiệm thu cuối',
        status: detail.final_acceptance_status,
        result: detail.final_acceptance_result,
      }
    : undefined;

  return { ...definition, checkedAt, result, metrics, linkedRecord };
}

function automaticSourceDefinition(item: ChecklistItem, projectId: string): AutomaticSourceDefinition | null {
  const sourceId = item.source_id ? `?record=${encodeURIComponent(item.source_id)}` : '';
  const definitions: Record<string, AutomaticSourceDefinition> = {
    final_acceptance: {
      itemCode: 'final_acceptance',
      sourceLabel: 'Nghiệm thu cuối',
      moduleLabel: 'Nghiệm thu',
      criterion: 'Biên bản được liên kết phải là nghiệm thu cuối, đã hoàn tất và có kết quả Đạt hoặc Đạt có điều kiện.',
      targetPath: `/projects/${projectId}/acceptance${sourceId}`,
      targetLabel: 'Mở Nghiệm thu',
      permissions: ['acceptance.view'],
      countKey: 'final_acceptance',
      metricLabel: 'Biên bản đạt điều kiện',
      metricSuffix: 'biên bản',
    },
    warehouse_closure: {
      itemCode: 'warehouse_closure',
      sourceLabel: 'Kho và vật tư dư',
      moduleLabel: 'Kho vật tư',
      criterion: 'Không còn vật tư tại công trình có số lượng dư lớn hơn 0 và trạng thái chưa xử lý xong.',
      targetPath: `/projects/${projectId}/warehouse?tab=site`,
      targetLabel: 'Mở Kho vật tư',
      permissions: ['warehouse.view'],
      countKey: 'pending_surplus',
      metricLabel: 'Vật tư dư chưa xử lý',
      metricSuffix: 'dòng',
    },
    ap_reconciliation: {
      itemCode: 'ap_reconciliation',
      sourceLabel: 'Công nợ phải trả',
      moduleLabel: 'Công nợ · Phải trả',
      criterion: 'Không còn khoản công nợ phải trả chưa hủy có số dư còn phải trả lớn hơn 0.',
      targetPath: `/projects/${projectId}/debt?kind=payables&queue=all`,
      targetLabel: 'Mở Công nợ phải trả',
      permissions: ['accounting.payable_view'],
      countKey: 'open_payables',
      amountKey: 'open_payables_amount',
      metricLabel: 'Khoản phải trả còn số dư',
      metricSuffix: 'khoản',
    },
    ar_reconciliation: {
      itemCode: 'ar_reconciliation',
      sourceLabel: 'Công nợ phải thu',
      moduleLabel: 'Công nợ · Phải thu',
      criterion: 'Không còn khoản công nợ phải thu chưa hủy có số dư còn phải thu lớn hơn 0.',
      targetPath: `/projects/${projectId}/debt?kind=receivables&queue=all`,
      targetLabel: 'Mở Công nợ phải thu',
      permissions: ['accounting.receivable_view'],
      countKey: 'open_receivables',
      amountKey: 'open_receivables_amount',
      metricLabel: 'Khoản phải thu còn số dư',
      metricSuffix: 'khoản',
    },
  };
  return definitions[item.item_code] ?? null;
}

function numericSnapshotValue(value: unknown) {
  if (value === undefined || value === null || value === '') return undefined;
  const number = Number(value);
  return Number.isFinite(number) ? number : undefined;
}

function evidenceTypeLabel(value?: string) {
  return ({ photo: 'Ảnh', document: 'Tài liệu', checklist: 'Checklist', link: 'Liên kết', note: 'Ghi chú', other: 'Khác' } as Record<string, string>)[value ?? ''] ?? value ?? 'Minh chứng';
}

function documentTypeLabel(value?: string) {
  return ({ drawing: 'Bản vẽ', as_built: 'Hồ sơ hoàn công', manual: 'Tài liệu vận hành', warranty: 'Hồ sơ bảo hành', handover: 'Biên bản bàn giao' } as Record<string, string>)[value ?? ''] ?? value ?? 'Tài liệu hồ sơ';
}

function safeExternalUrl(value?: string) {
  const candidate = value?.trim();
  if (!candidate) return undefined;
  try {
    const url = new URL(candidate, 'http://localhost');
    return ['http:', 'https:'].includes(url.protocol) ? candidate : undefined;
  } catch {
    return undefined;
  }
}

function isPlaceholderUrl(value?: string) {
  if (!value) return false;
  try {
    const hostname = new URL(value, 'http://localhost').hostname.toLowerCase();
    return hostname === 'example.com' || hostname.endsWith('.example.com');
  } catch {
    return false;
  }
}
