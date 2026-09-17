import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

async function source(path: string) {
  return readFile(new URL(path, import.meta.url), 'utf8');
}

test('warehouse tabs expose permission-aware correction actions', async () => {
  const page = await source('../src/modules/warehouse/WarehousePage.tsx');

  assert.match(page, /put\(`\/materials\/\$\{materialId\}`/);
  assert.match(page, /post<StockDocument>\('\/stock\/adjustments'/);
  assert.match(page, /put<StockDocument & WorkItemMutationResult>\(`\/stock\/documents\/\$\{stockDocumentId\}`/);
  assert.match(page, /post\(`\/projects\/\$\{projectId\}\/material-balances\/\$\{materialId\}\/adjustments`/);
  assert.match(page, /identity_locked.*mã, ĐVT và chế độ theo dõi được khóa để bảo toàn lịch sử/);
  assert.match(page, /title="Điều chỉnh tồn kho"/);
  assert.match(page, /title="Điều chỉnh vật tư đã dùng"/);
  assert.match(page, /Lý do điều chỉnh dữ liệu tồn kho/);
  assert.match(page, /canEditStockDocument\(row, hasPermission\)/);
  assert.match(page, /row\.status !== 'posted'/);
});

test('stock document and site adjustment dialogs keep units derived from material master', async () => {
  const page = await source('../src/modules/warehouse/WarehousePage.tsx');

  assert.match(page, /function selectMaterial\([^}]+unit: material\?\.unit \?\? ''/s);
  assert.match(page, /value=\{line\.unit\} readOnly/);
  assert.match(page, /target > usableLimit \? 'Khối lượng đã dùng cộng hao hụt không được vượt khối lượng đã xuất công trình\.'/);
  assert.match(page, /Hệ thống lập phiếu điều chỉnh tăng\/giảm; không ghi đè số dư hiện tại\./);
});
