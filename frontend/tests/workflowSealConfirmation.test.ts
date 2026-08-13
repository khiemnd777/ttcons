import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

async function source(path: string) {
  return readFile(new URL(path, import.meta.url), 'utf8');
}

test('workflow confirmation UI separates personal signatures from company seals', async () => {
  const dialog = await source('../src/modules/work-items/SignatureCaptureDialog.tsx');
  const panel = await source('../src/modules/work-items/WorkflowFormPanel.tsx');
  const presentation = await source('../src/modules/work-items/workItemPresentation.ts');

  assert.match(dialog, /title="Thêm xác nhận hồ sơ"/);
  assert.match(dialog, /label="Chữ ký cá nhân"/);
  assert.match(dialog, /label="Con dấu công ty"/);
  assert.match(dialog, /'Ký và lưu' : 'Đóng dấu và lưu'/);
  assert.match(dialog, /image\/png,image\/jpeg,image\/webp/);
  assert.match(dialog, /không hỗ trợ SVG/);
  assert.match(panel, /Xác nhận hồ sơ và PDF/);
  assert.match(panel, /\/workflow-form-instances\/\$\{instance\?\.id\}\/seal-applications/);
  assert.match(panel, /\/workflow-form-seal-applications\/\$\{application\.id\}\/visual/);
  assert.match(panel, /buildConfirmationPurposeGroups/);
  assert.match(panel, /\/pdf-confirmation-preference/);
  assert.match(panel, /Hiển thị trong PDF/);
  assert.match(panel, /statusLabel="Đã ký"/);
  assert.match(panel, /statusLabel="Đã đóng dấu"/);
  assert.match(panel, /Cần ít nhất một loại xác nhận/);
  assert.match(panel, /post<WorkflowFormDetail>\(`\/workflow-form-instances\/\$\{instance\?\.id\}\/pdf-exports`, \{\}\)/);
  assert.doesNotMatch(panel, /Chọn xác nhận để tạo PDF/);
  assert.doesNotMatch(panel, /Ký \/ Đóng dấu PDF/);
  assert.match(presentation, /seal_applied: 'Đã đóng dấu hồ sơ'/);
  assert.match(presentation, /Xác nhận \/ PDF \/ hoàn tất bước/);
});

test('PDF preference save preserves the canonical work-item-form query envelope', async () => {
  const panel = await source('../src/modules/work-items/WorkflowFormPanel.tsx');

  assert.match(panel, /type WorkflowFormInstanceMutationDetail = Omit<WorkflowFormDetail, 'available' \| 'auto_advance_when_ready'>/);
  assert.match(panel, /put<WorkflowFormInstanceMutationDetail>\(`\/workflow-form-instances\/\$\{instance\?\.id\}\/pdf-confirmation-preference`/);
  assert.match(panel, /onSuccess: \(_result, _payload, context\) => queryClient\.invalidateQueries\(\{ queryKey: context\.queryKey \}\)/);
  assert.doesNotMatch(panel, /onSuccess: \(result, _payload, context\) => \{\s*queryClient\.setQueryData<WorkflowFormDetail>\(context\.queryKey, result\)/);
});

test('seal catalog is reusable and management-gated', async () => {
  const settings = await source('../src/modules/settings/SettingsPage.tsx');
  const panel = await source('../src/modules/work-items/WorkflowFormPanel.tsx');

  assert.match(settings, /hasPermission\('seal\.manage'\)/);
  assert.match(settings, /Tài nguyên con dấu/);
  assert.match(settings, /\/seal-assets\?include_archived=true/);
  assert.match(settings, /\/seal-assets\/\$\{assetId\}\/archive/);
  assert.match(panel, /hasPermission\('seal\.use'\)/);
  assert.match(panel, /postForm<SealAsset>\('\/seal-assets\/upload'/);
});

test('contract App signing remains personal-signature only', async () => {
  const contracts = await source('../src/modules/contracts/ContractsPage.tsx');

  assert.match(contracts, /allowedKinds=\{\['signature'\]\}/);
  assert.match(contracts, /find\(\(row\) => row\.purpose === 'approved_by'/);
  assert.doesNotMatch(contracts, /seal-applications/);
});
