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

test('Rev7 drafts preserve persisted answers over server suggestions and collapse optional capture', async () => {
  const panel = await source('../src/modules/work-items/WorkflowFormPanel.tsx');

  assert.match(panel, /form\.data\?\.suggested_answers/);
  assert.match(panel, /\{ \.\.\.\(form\.data\?\.suggested_answers \?\? \{\}\), \.\.\.\(form\.data\?\.answers \?\? \{\}\) \}/);
  assert.match(panel, /workflowFormFieldIsOptional/);
  assert.match(panel, /validation_json\?\.capture_mode === 'optional'/);
  assert.match(panel, /Thông tin bổ sung/);
  assert.match(panel, /Không bắt buộc/);
  assert.match(panel, /aria-expanded=\{optionalFieldsOpen\}/);
  assert.match(panel, /className="grid grid-cols-1 gap-3 border-t border-line p-3"/);
  assert.match(panel, /className="grid grid-cols-1 gap-3 border-t border-slate-200 bg-white p-3"/);
  assert.match(panel, /className="min-w-0 scroll-mt-24 rounded-md outline-none"/);
  assert.doesNotMatch(panel, /md:col-span-2/);
});

test('Rev7 work item surface renders canonical source readiness and direct source navigation', async () => {
  const surface = await source('../src/modules/work-items/WorkItemPrimarySurface.tsx');

  assert.match(surface, /item\.business_source \? <BusinessSourceCard/);
  assert.match(surface, /Dữ liệu từ module nguồn/);
  assert.match(surface, /source\.ready/);
  assert.match(surface, /to=\{source\.route\}/);
  assert.match(surface, /Mở nguồn/);
});
