import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';
import {
  firstInvalidFieldKey,
  orderedInvalidFieldKeys,
} from '../src/lib/utils/formValidationNavigation.ts';

async function source(path: string) {
  return readFile(new URL(path, import.meta.url), 'utf8');
}

test('orders invalid fields by visible form order and keeps unknown dynamic keys stable', () => {
  const errors = {
    dynamic_row_2: 'Sai dữ liệu',
    title: 'Bắt buộc',
    amount: '',
    dynamic_row_1: 'Bắt buộc',
  };

  assert.deepEqual(
    orderedInvalidFieldKeys(errors, ['title', 'amount', 'description']),
    ['title', 'dynamic_row_2', 'dynamic_row_1'],
  );
  assert.equal(firstInvalidFieldKey(errors, ['title', 'amount']), 'title');
  assert.equal(firstInvalidFieldKey({}, ['title']), undefined);
});

test('shared navigation stays scoped, reveals conditional content, focuses, and respects reduced motion', async () => {
  const navigation = await source('../src/lib/utils/formValidationNavigation.ts');

  assert.match(navigation, /root\.querySelectorAll<HTMLElement>/);
  assert.match(navigation, /normalized\.prepare\?\.\(\)/);
  assert.match(navigation, /requestAnimationFrame[\s\S]*requestAnimationFrame/);
  assert.match(navigation, /prefers-reduced-motion: reduce/);
  assert.match(navigation, /scrollIntoView\(\{ behavior: reduceMotion \? 'auto' : 'smooth', block: 'center' \}\)/);
  assert.match(navigation, /focus\(\{ preventScroll: true \}\)/);
  assert.match(navigation, /window\.setTimeout[\s\S]*1800/);
  assert.doesNotMatch(navigation, /document\.querySelector/);
});

test('shared and custom form surfaces expose validation keys to the same navigation contract', async () => {
  const [simpleForm, workflowForm, acceptance, pipeline, workItem] = await Promise.all([
    source('../src/modules/_shared/SimpleForm.tsx'),
    source('../src/modules/work-items/WorkflowFormPanel.tsx'),
    source('../src/modules/acceptance/AcceptanceEditor.tsx'),
    source('../src/modules/pipeline-builder/components/NodeInspectorPanel.tsx'),
    source('../src/modules/work-items/WorkItemDetailDialog.tsx'),
  ]);

  for (const formSource of [simpleForm, workflowForm, acceptance, pipeline, workItem]) {
    assert.match(formSource, /useValidationNavigation/);
    assert.match(formSource, /data-validation-key|data-workflow-field-key/);
  }
});
