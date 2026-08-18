import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';
import { actionColumnsFirst, isActionColumn } from '../src/components/tableActionColumns.ts';

test('action column recognition supports conventional keys and explicit semantic columns', () => {
  assert.equal(isActionColumn({ key: 'action' }), true);
  assert.equal(isActionColumn({ key: 'actions' }), true);
  assert.equal(isActionColumn({ key: ' Actions ' }), true);
  assert.equal(isActionColumn({ key: 'next', action: true }), true);
  assert.equal(isActionColumn({ key: 'status' }), false);
});

test('action columns are stably moved to the beginning without mutating the input', () => {
  const columns = [
    { key: 'name' },
    { key: 'actions' },
    { key: 'status' },
    { key: 'next', action: true },
  ];

  assert.deepEqual(actionColumnsFirst(columns).map((column) => column.key), ['actions', 'next', 'name', 'status']);
  assert.deepEqual(columns.map((column) => column.key), ['name', 'actions', 'status', 'next']);
});

test('DataTable renders ordered action columns with a visually blank accessible header', async () => {
  const source = await readFile(new URL('../src/components/DataTable.tsx', import.meta.url), 'utf8');

  assert.equal((source.match(/orderedColumns\.map\(\(column\)/g) ?? []).length, 2);
  assert.match(source, /const orderedColumns = useMemo\(\(\) => actionColumnsFirst\(columns\), \[columns\]\)/);
  assert.match(source, /actionColumn \? \(\s*<span className="sr-only">Thao tác<\/span>/);
  assert.doesNotMatch(source, /actionColumn \? column\.header/);
});
