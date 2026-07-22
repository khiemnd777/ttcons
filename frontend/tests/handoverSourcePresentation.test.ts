import assert from 'node:assert/strict';
import test from 'node:test';
import {
  automaticSourcePresentation,
  checklistItemCodeLabel,
  checklistItemCodeTooltip,
  checklistSourceLabel,
  checklistSourceMode,
  linkedSourcePresentations,
} from '../src/modules/handover/handoverSourcePresentation.ts';
import type { ChecklistItem, HandoverDetail } from '../src/modules/handover/handoverTypes.ts';

const projectId = 'project-1';

function checklistItem(values: Partial<ChecklistItem>): ChecklistItem {
  return {
    id: values.item_code ?? 'item-1',
    item_code: 'manual_item',
    category: 'other',
    title: 'Mục checklist',
    required: true,
    status: 'pending',
    ...values,
  };
}

function detail(items: ChecklistItem[], counts?: Record<string, number>): HandoverDetail {
  return {
    id: 'handover-1',
    project_id: projectId,
    handover_code: 'HO-001',
    status: counts ? 'blocked' : 'draft',
    client_representative: 'Chủ đầu tư',
    company_representative: 'Nhà thầu',
    checklist_items: items,
    evidence_options: [],
    document_options: [],
    readiness_snapshot_json: counts ? { ready: false, blockers: [], counts, checked_at: '2026-07-16T04:00:00Z' } : undefined,
  };
}

test('presents every current checklist item code with its Vietnamese business label', () => {
  const labels = {
    final_acceptance: 'Nghiệm thu cuối',
    as_built_documents: 'Hồ sơ hoàn công',
    warehouse_closure: 'Đóng kho và xử lý vật tư dư',
    operation_manuals: 'Tài liệu vận hành và bảo trì',
    warranty_documents: 'Hồ sơ bảo hành',
    ap_reconciliation: 'Đối soát công nợ phải trả',
    ar_reconciliation: 'Đối soát công nợ phải thu',
    signed_handover_minutes: 'Biên bản bàn giao hai bên',
  };

  for (const [itemCode, label] of Object.entries(labels)) {
    assert.equal(checklistItemCodeLabel(itemCode), label);
    assert.equal(checklistItemCodeTooltip(itemCode), `Mã kỹ thuật: ${itemCode}`);
  }
  assert.equal(checklistItemCodeLabel('future_item'), 'Nội dung khác');
});

test('classifies only readiness-owned checks as automatic', () => {
  assert.equal(checklistSourceMode(checklistItem({ item_code: 'final_acceptance', source_type: 'acceptance' })), 'automatic');
  assert.equal(checklistSourceMode(checklistItem({ item_code: 'warehouse_closure', source_type: 'system_check' })), 'automatic');
  assert.equal(checklistSourceMode(checklistItem({ item_code: 'operation_manuals' })), 'manual');
});

test('linked evidence and documents are presented as user-linked sources, not automatic checks', () => {
  const documentItem = checklistItem({ document_id: 'document-1', document_name: 'Hướng dẫn vận hành' });
  const evidenceItem = checklistItem({ evidence_id: 'evidence-1', evidence_name: 'Ảnh hoàn công' });

  assert.equal(checklistSourceMode(documentItem), 'linked');
  assert.equal(checklistSourceLabel(documentItem, projectId), 'Hướng dẫn vận hành');
  assert.equal(checklistSourceMode(evidenceItem), 'linked');
  assert.equal(checklistSourceLabel(evidenceItem, projectId), 'Ảnh hoàn công');
});

test('builds an openable presentation for a linked document including placeholder URL warning', () => {
  const item = checklistItem({
    item_code: 'as_built_documents',
    document_id: 'document-1',
    document_name: 'Bản vẽ thi công được duyệt',
    document_type: 'drawing',
    document_status: 'active',
    document_file_url: 'https://example.com/golden/as-built.pdf',
  });

  assert.deepEqual(linkedSourcePresentations(item), [{
    kind: 'document',
    id: 'document-1',
    name: 'Bản vẽ thi công được duyệt',
    typeLabel: 'Bản vẽ',
    status: 'active',
    openMode: 'external_document',
    targetUrl: 'https://example.com/golden/as-built.pdf',
    placeholderUrl: true,
  }]);

  assert.equal(linkedSourcePresentations({ ...item, document_status: 'deleted' })[0].openMode, 'none');
});

test('keeps local and s3 linked evidence behind the protected evidence file endpoint', () => {
  const item = checklistItem({
    evidence_id: 'evidence-1',
    evidence_name: 'Ảnh nghiệm thu hoàn công',
    evidence_type: 'photo',
    evidence_status: 'accepted',
    evidence_storage_provider: 'local',
    evidence_mime_type: 'image/jpeg',
    evidence_file_size: 2048,
  });

  assert.deepEqual(linkedSourcePresentations(item), [{
    kind: 'evidence',
    id: 'evidence-1',
    name: 'Ảnh nghiệm thu hoàn công',
    typeLabel: 'Ảnh',
    status: 'accepted',
    openMode: 'protected_evidence',
    placeholderUrl: false,
    mimeType: 'image/jpeg',
    fileSize: 2048,
  }]);

  assert.equal(linkedSourcePresentations({ ...item, evidence_storage_provider: 's3' })[0].openMode, 'protected_evidence');
  assert.equal(linkedSourcePresentations({ ...item, evidence_storage_provider: 'external' })[0].openMode, 'none');
  assert.equal(linkedSourcePresentations({ ...item, evidence_storage_provider: 'none' })[0].openMode, 'none');
  assert.equal(linkedSourcePresentations({ ...item, evidence_storage_provider: 'future-provider' })[0].openMode, 'none');
});

test('builds precise source targets for acceptance, warehouse and both debt kinds', () => {
  const items = [
    checklistItem({ item_code: 'final_acceptance', source_type: 'acceptance', source_id: 'acceptance-1' }),
    checklistItem({ item_code: 'warehouse_closure', source_type: 'system_check' }),
    checklistItem({ item_code: 'ap_reconciliation', source_type: 'system_check' }),
    checklistItem({ item_code: 'ar_reconciliation', source_type: 'system_check' }),
  ];
  const handover = detail(items, {});

  assert.equal(automaticSourcePresentation(items[0], handover)?.targetPath, '/projects/project-1/acceptance?record=acceptance-1');
  assert.equal(automaticSourcePresentation(items[1], handover)?.targetPath, '/projects/project-1/warehouse?tab=site');
  assert.equal(automaticSourcePresentation(items[2], handover)?.targetPath, '/projects/project-1/debt?kind=payables&queue=all');
  assert.equal(automaticSourcePresentation(items[3], handover)?.targetPath, '/projects/project-1/debt?kind=receivables&queue=all');
});

test('presents the stored readiness snapshot without inventing live detail rows', () => {
  const payable = checklistItem({ item_code: 'ap_reconciliation', source_type: 'system_check', status: 'pending' });
  const handover = detail([payable], { open_payables: 3, open_payables_amount: 450000000 });
  const source = automaticSourcePresentation(payable, handover);

  assert.equal(source?.result, 'failed');
  assert.equal(source?.checkedAt, '2026-07-16T04:00:00Z');
  assert.deepEqual(source?.metrics.map((metric) => [metric.label, metric.value]), [
    ['Khoản phải trả còn số dư', 3],
    ['Tổng còn phải trả', 450000000],
  ]);

  const unchecked = automaticSourcePresentation(checklistItem({ item_code: 'ar_reconciliation', source_type: 'system_check' }), detail([]));
  assert.equal(unchecked?.result, 'unchecked');
  assert.equal(unchecked?.metrics[0].value, undefined);
});
