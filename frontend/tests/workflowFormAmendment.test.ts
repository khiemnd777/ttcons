import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';
import { domainTypesSource } from './domainTypesSource.ts';

async function source(path: string) {
  return readFile(new URL(path, import.meta.url), 'utf8');
}

test('submitted workflow form exposes a server-capability amendment action above project navigation', async () => {
  const detail = await source('../src/modules/work-items/WorkItemDetailDialog.tsx');
  const panel = await source('../src/modules/work-items/WorkflowFormPanel.tsx');

  assert.match(panel, /form\.data\?\.capabilities\?\.can_amend/);
  assert.match(detail, /workflowFormActionState\.canAmend/);
  assert.match(detail, /Chỉnh sửa[\s\S]*Mở công trình/);
  assert.match(detail, /title="Chỉnh sửa hồ sơ đã chốt\?"/);
  assert.match(detail, /Lý do chỉnh sửa/);
  assert.match(detail, /toast\.warn\(\{ title: 'Thiếu lý do chỉnh sửa'/);
});

test('amendment saves a new optimistic revision and keeps confirmation artifacts revision-scoped', async () => {
  const panel = await source('../src/modules/work-items/WorkflowFormPanel.tsx');
  const types = await domainTypesSource();

  assert.match(panel, /\/workflow-form-instances\/\$\{instance\?\.id\}\/amend/);
  assert.match(panel, /expected_revision: baseRevisionRef.current/);
  assert.match(panel, /baseRevisionRef.current = currentRevision/);
  assert.match(panel, /Đã lưu phiên bản chỉnh sửa và ghi lịch sử thay đổi/);
  assert.match(panel, /is_current_revision \?\? signature\.form_revision_no === currentRevision/);
  assert.match(panel, /Xác nhận phiên bản trước/);
  assert.match(panel, /Chữ ký, con dấu và PDF tạm khóa/);
  assert.match(types, /can_amend\?: boolean/);
  assert.match(types, /is_current_revision\?: boolean/);
});

test('workflow history renders amendment reason and exact before-after field values', async () => {
  const panel = await source('../src/modules/work-items/WorkflowFormPanel.tsx');
  const presentation = await source('../src/modules/work-items/workItemPresentation.ts');

  assert.match(presentation, /amended: 'Đã chỉnh sửa hồ sơ'/);
  assert.match(panel, /metadata_json\?\.changed_fields/);
  assert.match(panel, />Lý do:<\/span>/);
  assert.match(panel, />Trước:<\/span>/);
  assert.match(panel, />Sau:<\/span>/);
});
