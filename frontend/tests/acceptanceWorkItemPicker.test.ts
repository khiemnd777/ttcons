import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';
import type { WorkItem } from '../src/lib/types/domain.ts';
import {
  acceptanceWorkItemMatchesSearch,
  groupAcceptanceWorkItems,
} from '../src/modules/acceptance/acceptanceWorkItemPicker.ts';

function workItem(id: string, values: Partial<WorkItem>): WorkItem {
  return {
    id,
    name: values.title ?? `Công việc ${id}`,
    title: values.title ?? `Công việc ${id}`,
    status: 'pending',
    ...values,
  } as WorkItem;
}

async function source(path: string) {
  return readFile(new URL(path, import.meta.url), 'utf8');
}

test('acceptance work item picker prioritizes active work and hides cancelled history', () => {
  const groups = groupAcceptanceWorkItems([
    workItem('pending', { title: 'Việc tương lai', status: 'pending', run_node_order_index: 4 }),
    workItem('ready', { title: 'Sẵn sàng nghiệm thu', status: 'ready', run_node_order_index: 2 }),
    workItem('active', { title: 'Đang nghiệm thu', status: 'in_progress', run_node_order_index: 3 }),
    workItem('done', { title: 'Đã hoàn tất', status: 'completed', run_node_order_index: 1 }),
    workItem('cancelled', { title: 'Đã hủy', status: 'cancelled' }),
  ]);

  assert.deepEqual(groups.suggested.map((item) => item.id), ['active', 'ready']);
  assert.deepEqual(groups.others.map((item) => item.id), ['pending', 'done']);
});

test('acceptance work item search ignores Vietnamese accents and includes workflow context', () => {
  const item = workItem('acceptance', {
    title: 'Nghiệm thu vật tư đầu vào và đầu ra',
    status: 'ready',
    run_node_name: 'Thi công & Chất lượng',
    assignee_name: 'Nguyễn Minh',
  });

  assert.equal(acceptanceWorkItemMatchesSearch(item, 'nghiem thu vat tu'), true);
  assert.equal(acceptanceWorkItemMatchesSearch(item, 'chat luong'), true);
  assert.equal(acceptanceWorkItemMatchesSearch(item, 'nguyen minh'), true);
  assert.equal(acceptanceWorkItemMatchesSearch(item, 'ke toan'), false);
});

test('the acceptance editor defers source selection until new capture and task context is server-targeted', async () => {
  const editor = await source('../src/modules/acceptance/AcceptanceEditor.tsx');
  const page = await source('../src/modules/acceptance/AcceptancePage.tsx');
  const capture = await source('../src/modules/_shared/EvidenceCapture.tsx');

  assert.doesNotMatch(editor, /<select[^>]*value=\{draft\.work_item_id\}/);
  assert.match(editor, /onSourceRequired=\{/);
  assert.match(editor, /<AcceptanceWorkItemPickerDialog/);
  assert.match(page, /item\.execution_target\?\.workspace === 'acceptance'/);
  assert.match(page, /createAcceptanceDraft\(\{ workItemId: taskWorkItem\?\.id \}\)/);
  assert.match(capture, /onSourceRequired\?: \(intent: EvidenceCaptureIntent, resume: \(\) => void\)/);
  assert.match(capture, /requestCapture\('file'/);
  assert.match(capture, /requestCapture\('camera'/);
});
