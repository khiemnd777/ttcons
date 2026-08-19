import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

async function source(path: string) {
  return readFile(new URL(path, import.meta.url), 'utf8');
}

test('partner catalog uses business-facing navigation and page labels', async () => {
  const appShell = await source('../src/layouts/AppShell.tsx');
  const page = await source('../src/modules/core-data/CoreDataPage.tsx');

  assert.match(appShell, /to: '\/core-data', label: 'Đối tác'/);
  assert.doesNotMatch(appShell, /label: 'Dữ liệu lõi'/);
  assert.match(page, /title="Danh mục đối tác"/);
  assert.match(page, /Quản lý khách hàng, nhà cung cấp, thầu phụ và các đơn vị phối hợp\./);
  assert.match(page, />\s*Thêm đối tác\s*<\/button>/);
  assert.match(page, /get<Partner\[]>\(`\/partners\?status=/);
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
