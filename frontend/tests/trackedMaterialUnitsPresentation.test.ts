import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

const source = (path: string) => readFile(new URL(path, import.meta.url), 'utf8');

test('warehouse exposes the reusable physical-unit subledger and exact stock allocations', async () => {
  const page = await source('../src/modules/warehouse/WarehousePage.tsx');

  assert.match(page, /id: 'units', label: 'Phần dư tái sử dụng'/);
  assert.match(page, /get<MaterialStockUnit\[]>\('\/stock\/units'\)/);
  assert.match(page, /post\(`\/materials\/\$\{materialId\}\/stock-units\/initialize`/);
  assert.match(page, /function PhysicalReceiveEditor\(/);
  assert.match(page, /function PhysicalAllocationEditor\(/);
  assert.match(page, /row\.tracking_mode === 'physical_unit' \? null/);
  assert.match(page, /remaining_width_mm/);
  assert.match(page, /remaining_thickness_mm/);
  assert.match(page, /Phiếu đã ghi sổ giữ nguyên lịch sử đơn vị vật lý/);
});

test('work-item material use and surplus return require physical-unit allocations', async () => {
  const snapshot = await source('../src/modules/work-items/MaterialSnapshotPanel.tsx');
  const surplus = await source('../src/modules/work-items/SurplusHandlingPanel.tsx');

  assert.match(snapshot, /get<MaterialStockUnit\[]>\(`\/stock\/units\?project_id=/);
  assert.match(snapshot, /material\?\.tracking_mode === 'physical_unit'/);
  assert.match(snapshot, /stock_unit_usage: stockUnitUsage/);
  assert.match(snapshot, /remaining_length_mm/);
  assert.match(snapshot, /remaining_width_mm/);
  assert.match(snapshot, /remaining_thickness_mm/);
  assert.match(surplus, /balance\.tracking_mode === 'physical_unit'/);
  assert.match(surplus, /stock_unit_allocations: draft\.stock_unit_allocations/);
  assert.match(surplus, /Phân bổ Đoạn\/Cuộn\/Cây\/Tấm/);
});
