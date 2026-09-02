import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

async function source(path: string) {
  return readFile(new URL(path, import.meta.url), 'utf8');
}

test('Design receives a read-only material catalog without stock queries', async () => {
  const page = await source('../src/modules/warehouse/WarehousePage.tsx');
  const router = await source('../src/app/router.tsx');
  const shell = await source('../src/layouts/AppShell.tsx');

  assert.match(router, /<WarehousePage \/>.*\['warehouse\.view', 'material\.view'\]/);
  assert.match(shell, /label: 'Vật tư'.*material\.view.*excludePermissions: \['warehouse\.view'\]/);
  assert.match(page, /const canViewWarehouse = hasPermission\('warehouse\.view'\)/);
  assert.match(page, /enabled: canViewWarehouse/);
  assert.match(page, /if \(!canViewWarehouse\) \{[\s\S]*title="Vật tư"[\s\S]*<MaterialsTab/);
  assert.match(page, /Số liệu tồn kho và nghiệp vụ kho không nằm trong phạm vi truy cập/);
});

test('Preparation exposes immutable design package revisions and explicit PM handoff', async () => {
  const preparation = await source('../src/modules/planning/PreparationPage.tsx');
  const detail = await source('../src/modules/work-items/WorkItemDetailDialog.tsx');
  const panel = await source('../src/modules/work-items/DesignReleasePanel.tsx');

  assert.match(preparation, /<DesignReleasePanel projectId=\{id\} workItems=\{allPreparationItems\}/);
  assert.match(detail, /isDesignReleaseWorkItem\(detailItem\)[\s\S]*<DesignReleasePanel/);
  assert.match(panel, /Mỗi lần phát hành là một gói hồ sơ độc lập/);
  assert.match(panel, /Revision mới không tự thay revision PM đã bàn giao cho QS/);
  assert.match(panel, /BOQ và mua hàng giữ nguyên nguồn cũ/);
  assert.match(panel, /Bàn giao \{current\.revision_label\} cho QS/);
  assert.match(panel, /multiple accept="application\/pdf,image\/png,image\/jpeg,image\/webp"/);
  assert.match(panel, /postForm\(`\/design-releases\/\$\{release\.id\}\/files`/);
  assert.match(panel, /post\(`\/design-releases\/\$\{release\.id\}\/release`/);
  assert.match(panel, /Lưu nháp/);
  assert.match(panel, /Phát hành revision/);
});
