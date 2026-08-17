import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

async function workflowFormPanelSource() {
  return readFile(new URL('../src/modules/work-items/WorkflowFormPanel.tsx', import.meta.url), 'utf8');
}

async function globalStylesSource() {
  return readFile(new URL('../src/index.css', import.meta.url), 'utf8');
}

function functionSlice(source: string, start: string, end: string) {
  const startIndex = source.indexOf(start);
  const endIndex = source.indexOf(end, startIndex + start.length);
  assert.notEqual(startIndex, -1, `Missing source marker: ${start}`);
  assert.notEqual(endIndex, -1, `Missing source marker: ${end}`);
  return source.slice(startIndex, endIndex);
}

test('generic and checklist structured lists share the schema-driven table shell', async () => {
  const panel = await workflowFormPanelSource();
  const generic = functionSlice(panel, 'function StructuredListControl(', 'function ChecklistStructuredListControl(');
  const checklist = functionSlice(panel, 'function ChecklistStructuredListControl(', 'function StructuredListTable(');
  const schema = panel.slice(panel.indexOf('function structuredItemFields('));

  assert.match(generic, /const specs = structuredItemFields\(field\)/);
  assert.match(generic, /<StructuredListTable/);
  assert.match(checklist, /const specs = structuredItemFields\(field\)/);
  assert.match(checklist, /<StructuredListTable/);
  assert.match(checklist, /<ChecklistStat label="Cần xử lý"/);
  assert.match(schema, /field\.validation_json\?\.item_fields/);
  assert.match(schema, /key: String\(spec\.key\)/);
  assert.match(schema, /label: String\(spec\.label \?\? spec\.key\)/);
  assert.match(schema, /type: String\(spec\.type \?\? 'text'\)/);
  assert.match(schema, /options: Array\.isArray\(spec\.options\)/);
});

test('structured list table handles compact, wide and narrow-screen schemas', async () => {
  const panel = await workflowFormPanelSource();
  const styles = await globalStylesSource();
  const field = functionSlice(panel, 'function WorkflowFieldControl(', 'function renderFieldControl(');
  const table = functionSlice(panel, 'function StructuredListTable(', 'function StructuredDeleteButton(');

  assert.match(field, /className="min-w-0 scroll-mt-24 rounded-md outline-none"/);
  assert.doesNotMatch(field, /md:col-span-2/);
  assert.match(table, /mt-1 min-w-0 w-full max-w-full rounded-md border p-2/);
  assert.match(table, /structured-list-scroll hidden min-w-0 w-full max-w-full overflow-x-auto rounded-md border border-line md:block/);
  assert.match(table, /className="space-y-2 md:hidden"/);
  assert.match(table, /specs\.map\(\(spec\) => <col/);
  assert.match(table, /structuredTableMinWidth\(specs, !displayOnly\)/);
  assert.match(table, /viewport\.scrollWidth > viewport\.clientWidth \+ 1/);
  assert.match(table, /new ResizeObserver\(updateOverflow\)/);
  assert.match(table, /hasHorizontalOverflow \? \(/);
  assert.match(table, /Kéo ngang để xem các cột còn lại/);
  assert.match(styles, /\.structured-list-scroll \{/);
  assert.match(styles, /scrollbar-gutter: stable/);
  assert.match(styles, /scrollbar-width: thin/);
  assert.match(styles, /\.structured-list-scroll::\-webkit-scrollbar/);
  assert.match(styles, /height: 10px/);
  assert.match(panel, /case 'textarea':\s+return 280/);
  assert.match(panel, /case 'evidence_reference':\s+return 260/);
  assert.match(panel, /return 48 \+ specs\.reduce/);
});

test('editable structured list keeps the icon-only actions column first', async () => {
  const panel = await workflowFormPanelSource();
  const table = functionSlice(panel, 'function StructuredListTable(', 'function StructuredDeleteButton(');
  const columnGroup = functionSlice(table, '<colgroup>', '</colgroup>');
  const header = functionSlice(table, '<thead', '</thead>');
  const desktopRow = functionSlice(table, '<tr key={rowIndex}', '{specs.map((spec) => (');

  const actionColumnIndex = columnGroup.indexOf('!displayOnly ? <col style={{ width: 52 }} />');
  const rowNumberColumnIndex = columnGroup.indexOf('<col style={{ width: 48 }} />');
  assert.ok(actionColumnIndex !== -1 && actionColumnIndex < rowNumberColumnIndex);

  const actionHeaderIndex = header.indexOf('<span className="sr-only">Thao tác</span>');
  const rowNumberHeaderIndex = header.indexOf('text-center">#</th>');
  assert.ok(actionHeaderIndex !== -1 && actionHeaderIndex < rowNumberHeaderIndex);
  assert.doesNotMatch(header, />Thao tác<\/th>/);

  const actionCellIndex = desktopRow.indexOf('<StructuredDeleteButton');
  const rowNumberCellIndex = desktopRow.indexOf('<th scope="row"');
  assert.ok(actionCellIndex !== -1 && actionCellIndex < rowNumberCellIndex);
});

test('adding a structured-list row focuses its first inline editor', async () => {
  const panel = await workflowFormPanelSource();
  const table = functionSlice(panel, 'function StructuredListTable(', 'function StructuredDeleteButton(');

  assert.match(table, /focusRowRef\.current = rows\.length;\s+onAddRow\(\)/);
  assert.match(table, /querySelectorAll<HTMLElement>/);
  assert.match(table, /input:not\(:disabled\)/);
  assert.match(table, /select:not\(:disabled\)/);
  assert.match(table, /textarea:not\(:disabled\)/);
  assert.match(table, /target\?\.focus\(\)/);
  assert.match(table, />\s*Thêm dòng\s*</);
});

test('desktop and mobile row deletion require the shared confirmation dialog', async () => {
  const panel = await workflowFormPanelSource();
  const table = functionSlice(panel, 'function StructuredListTable(', 'function StructuredDeleteButton(');

  assert.match(table, /useState<number \| null>\(null\)/);
  assert.equal((table.match(/onRemove=\{setPendingRemoveRowIndex\}/g) ?? []).length, 2);
  assert.match(table, /const confirmRemoveRow = \(\) => \{[\s\S]*onRemoveRow\(pendingRemoveRowIndex\);[\s\S]*setPendingRemoveRowIndex\(null\)/);
  assert.match(table, /<Dialog[\s\S]*open=\{pendingRemoveRowIndex !== null\}/);
  assert.match(table, /title="Xóa dòng này\?"/);
  assert.match(table, /Dòng \$\{pendingRemoveRowIndex \+ 1\} sẽ bị xóa khỏi/);
  assert.match(table, /autoFocus[\s\S]*>\s*Hủy\s*</);
  assert.match(table, /onClick=\{confirmRemoveRow\}[\s\S]*>\s*Xóa dòng\s*</);
  assert.match(table, /structuredRowDeleteSummary\(pendingRemoveRow, specs, evidence\)/);
  assert.match(table, /Dữ liệu chỉ được lưu chính thức khi anh chọn Lưu nháp hoặc Chốt hồ sơ\./);
});

test('structured list table keeps required-cell feedback and read-only presentation', async () => {
  const panel = await workflowFormPanelSource();
  const table = functionSlice(panel, 'function StructuredListTable(', 'function StructuredDeleteButton(');
  const cell = functionSlice(panel, 'function StructuredCell(', 'function structuredCellDisplayValue(');

  assert.match(panel, /displayOnly=\{readOnly \|\| \(submitted && !amending\)\}/);
  assert.match(table, /structuredCellInvalid\(spec, row\[spec\.key\], invalid, checklist\)/);
  assert.match(table, /!displayOnly \? <StructuredDeleteButton/);
  assert.match(table, /!displayOnly \? \(/);
  assert.match(cell, /if \(displayOnly\)/);
  assert.match(cell, /structuredCellDisplayValue\(spec, value, evidence\)/);
  assert.match(cell, /aria-invalid=\{invalid \|\| undefined\}/);
  assert.match(panel, /checklist && spec\.key === 'status'/);
});
