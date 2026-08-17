import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

async function source(path: string) {
  return readFile(new URL(path, import.meta.url), 'utf8');
}

test('editable workflow forms keep the primary actions available at the mobile viewport edge', async () => {
  const detail = await source('../src/modules/work-items/WorkItemDetailDialog.tsx');

  assert.match(detail, /const showMobileWorkflowFormActions = Boolean\(/);
  assert.match(detail, /showMobileWorkflowFormActions && 'pb-20 xl:pb-0'/);
  assert.match(detail, /data-workflow-form-mobile-actions/);
  assert.match(detail, /fixed inset-x-3 bottom-3[\s\S]*xl:hidden/);
  assert.match(detail, /paddingBottom: 'max\(0\.5rem, env\(safe-area-inset-bottom\)\)'/);
  assert.match(detail, /function WorkflowFormPrimaryActions\(/);
  assert.match(detail, /variant="desktop"/);
  assert.match(detail, /variant="mobile"/);
  assert.match(detail, /data-workflow-form-submit=\{variant\}/);
  assert.match(detail, /<div className="hidden xl:block">[\s\S]*variant="desktop"/);

  const mobileLayout = detail.match(/return variant === 'mobile' \? \(([\s\S]*?)\n  \) : \(/)?.[1] ?? '';
  assert.ok(mobileLayout.indexOf('{saveButton}') < mobileLayout.indexOf('{submitButton}'));
});

test('mobile and desktop workflow actions share the same save and submit callbacks', async () => {
  const detail = await source('../src/modules/work-items/WorkItemDetailDialog.tsx');

  const saveBindings = detail.match(/onSave=\{onSaveWorkflowForm\}/g) ?? [];
  const submitBindings = detail.match(/onSubmit=\{onSubmitWorkflowForm\}/g) ?? [];
  assert.equal(saveBindings.length, 2);
  assert.equal(submitBindings.length, 2);
  assert.match(detail, /onSaveWorkflowForm=\{\(\) => workflowFormActionRef\.current\?\.save\(\)\}/);
  assert.match(detail, /onSubmitWorkflowForm=\{\(\) => workflowFormActionRef\.current\?\.submit\(\)\}/);
});
