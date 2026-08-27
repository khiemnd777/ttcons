import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

async function source(path: string) {
  return readFile(new URL(path, import.meta.url), 'utf8');
}

test('partner catalog is one role-filtered business directory', async () => {
  const appShell = await source('../src/layouts/AppShell.tsx');
  const page = await source('../src/modules/core-data/CoreDataPage.tsx');
  const router = await source('../src/app/router.tsx');
  const partnerUtils = await source('../src/modules/core-data/partnerUtils.ts');

  assert.match(appShell, /to: '\/core-data', label: 'Đối tác'.*permissions: \['catalog\.view'\]/);
  assert.doesNotMatch(appShell, /to: '\/core-data\/(?:input|output)', label: '(?:Đầu vào|Đầu ra)'/);
  assert.doesNotMatch(appShell, /label: 'Dữ liệu lõi'/);
  assert.match(router, /path: '\/core-data'.*<CoreDataPage key="partners" group="all" \/>/);
  assert.match(router, /path: '\/core-data\/input'.*<Navigate to="\/core-data" replace \/>/);
  assert.match(router, /path: '\/core-data\/output'.*<Navigate to="\/core-data" replace \/>/);
  assert.match(page, /title: 'Đối tác'/);
  assert.match(page, /actionLabel: 'Thêm đối tác'/);
  assert.match(page, /title: 'Danh mục đầu vào'/);
  assert.match(page, /title: 'Danh mục đầu ra'/);
  assert.match(page, /actionLabel: 'Thêm đầu vào'/);
  assert.match(page, /actionLabel: 'Thêm đầu ra'/);
  assert.match(page, /group === 'all' \? '' : `&group=\$\{group\}`/);
  assert.match(page, /group === 'all' \? partnerTypeOptions : outputPartnerTypeOptions/);
  assert.match(partnerUtils, /outputPartnerTypeOptions = partnerTypeOptions\.filter\(\(option\) => option\.value !== 'client'\)/);
  assert.match(partnerUtils, /vendorPartnerTypes = outputPartnerTypeOptions\.map/);
});

test('material catalog presents the linked supplier as a reference source', async () => {
  const warehouse = await source('../src/modules/warehouse/WarehousePage.tsx');
  const simpleForm = await source('../src/modules/_shared/SimpleForm.tsx');

  assert.match(warehouse, /name: 'default_supplier_partner_id', label: 'Nhà cung cấp tham khảo'/);
  assert.match(warehouse, /placeholder: 'Chọn từ Danh mục đối tác'/);
  assert.match(warehouse, /Dùng làm nguồn cung ban đầu; có thể thay đổi khi lấy báo giá\./);
  assert.match(warehouse, /name: 'default_supplier', label: 'Tên nhà cung cấp khác'/);
  assert.match(warehouse, /Chỉ nhập khi nhà cung cấp chưa có trong Danh mục đối tác\./);
  assert.match(warehouse, /key: 'default_supplier_partner_name', header: 'Nhà cung cấp tham khảo'/);
  assert.match(simpleForm, /<option value="">\{field\.placeholder \?\? 'Chọn'\}<\/option>/);
});
