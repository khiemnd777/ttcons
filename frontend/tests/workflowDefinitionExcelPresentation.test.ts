import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

async function source(path: string) {
  return readFile(new URL(path, import.meta.url), 'utf8');
}

test('workflow definition worker enforces versioned Excel and nested JSON contracts', async () => {
  const helper = await source('../src/modules/workflow-definitions/workflowDefinitionExcel.ts');
  const worker = await source('../src/modules/workflow-definitions/workflowDefinitionExcel.worker.ts');
  for (const sheet of ['Pipelines', 'Pipeline_Nodes', 'Pipeline_Steps', 'Pipeline_Edges', 'Form_Templates', 'Form_Versions', 'Form_Fields']) {
    assert.match(helper, new RegExp(`${sheet}:`));
  }
  assert.match(helper, /WORKFLOW_DEFINITION_WORKBOOK_VERSION = '1'/);
  assert.match(helper, /WORKFLOW_DEFINITION_PACKAGE_VERSION = '1'/);
  assert.match(worker, /requiredSheet\(workbook, 'Manifest'\)/);
  assert.match(worker, /không được chứa công thức/);
  assert.match(worker, /Ref_Edge_Conditions|WORKFLOW_DEFINITION_SHEETS/);
  assert.match(worker, /consflow\.workflow-definition-package/);
  assert.match(worker, /assertExactKeys/);
  assert.match(worker, /item\.version\.fields/);
});

test('workflow definition import is preview-bound, idempotent and draft-only in the shared dialog', async () => {
  const dialog = await source('../src/modules/workflow-definitions/WorkflowDefinitionImportDialog.tsx');
  assert.match(dialog, /'\/workflow-definitions\/imports\/preview'/);
  assert.match(dialog, /'\/workflow-definitions\/imports\/execute'/);
  assert.match(dialog, /preview_hash/);
  assert.match(dialog, /idempotency_key/);
  assert.match(dialog, /Tạo các bản nháp/);
  assert.match(dialog, /không phát hành, không chạy Pipeline/);
  assert.match(dialog, /Cần xử lý trước khi import/);
  assert.match(dialog, /sourceFormat/);
  assert.match(dialog, /JSON \(\.json\)/);
});

test('Pipeline and Form screens align data actions with dedicated permissions', async () => {
  const pipeline = await source('../src/modules/pipeline-builder/PipelineBuilderPage.tsx');
  const forms = await source('../src/modules/workflow-forms/WorkflowFormsPage.tsx');
  assert.match(pipeline, /hasPermission\('workflow_definition\.import'\)/);
  assert.match(pipeline, /hasPermission\('workflow_definition\.export'\)/);
  assert.match(pipeline, /hasPermission\('pipeline\.publish'\)/);
  assert.match(pipeline, /canCloneTemplate=\{canCreateTemplate\}/);
  assert.match(forms, /hasPermission\('workflow_definition\.import'\)/);
  assert.match(forms, /hasPermission\('workflow_definition\.export'\)/);
  assert.match(forms, /schema_json: selectedVersion\?\.schema_json \?\? \{\}/);
  assert.match(forms, /validation_json: selectedVersion\?\.validation_json \?\? \{\}/);
  assert.match(forms, /<WorkflowDefinitionImportDialog/);
  assert.match(forms, /<WorkflowDefinitionExportDialog/);
});
