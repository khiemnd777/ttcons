import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

async function source(path: string) {
  return readFile(new URL(path, import.meta.url), 'utf8');
}

test('completion requirement rows expose keyboard actions and explicit instructions', async () => {
  const detail = await source('../src/modules/work-items/WorkItemDetailDialog.tsx');

  assert.match(detail, /<button[\s\S]*onClick=\{\(\) => onNavigate\(navigationTarget\)\}/);
  assert.match(detail, /row\.kind === 'permission' \? 'Khi không thể thao tác:' : 'Cách xử lý:'/);
  assert.match(detail, /focus-visible:ring-2/);
  assert.match(detail, /permissionRequirementPresentation\(item\)/);
  assert.match(detail, /if \(kind === 'permission'\) return <Shield className="[^"]*text-blue-600"/);
  assert.match(detail, /label: 'Quyền thao tác'/);
  assert.match(detail, /kind: 'permission'/);
  assert.match(detail, /Thông tin truy cập/);
  assert.doesNotMatch(detail, /Yêu cầu quyền: \$\{requirement\.required_permission\}/);
});

test('evidence requirement navigation opens and focuses the matching uploader', async () => {
  const evidencePanel = await source('../src/modules/work-items/EvidencePanel.tsx');

  assert.match(evidencePanel, /setRequirementEvidenceType\(navigationRequest\.target\.evidenceType \?\? null\)/);
  assert.match(evidencePanel, /setUploadOpen\(true\)/);
  assert.match(evidencePanel, /revealWorkItemRequirementTarget\(uploadFormRef\.current, fileInputRef\.current\)/);
  assert.match(evidencePanel, /requirementEvidenceType === 'photo' && classification\.evidenceType === 'document'/);
});

test('completion errors dispatch the backend-owned missing requirement target', async () => {
  const detail = await source('../src/modules/work-items/WorkItemDetailDialog.tsx');

  assert.match(
    detail,
    /variables\.action === 'complete'[\s\S]*completionRequirementNavigationFromError\(detailItem, error\)[\s\S]*navigateToRequirement\(target\)/,
  );
});

test('workflow-form navigation targets the first issue, checklist field, or submit action', async () => {
  const formPanel = await source('../src/modules/work-items/WorkflowFormPanel.tsx');

  assert.match(formPanel, /candidate\.field_key === 'checklist_items'/);
  assert.match(formPanel, /activeFields\.find\(\(candidate\) => validateFieldValue/);
  assert.match(formPanel, /data-workflow-field-key=\{field\.field_key\}/);
  assert.match(formPanel, /data-workflow-form-submit/);
});
