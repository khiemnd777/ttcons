import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

async function source(path: string) {
  return readFile(new URL(path, import.meta.url), 'utf8');
}

test('manual desktop tables keep the blank action header and action cell first', async () => {
  const supplier = await source('../src/modules/boq/SupplierQuotationWorkspace.tsx');
  const acceptance = await source('../src/modules/acceptance/AcceptancePage.tsx');
  const operations = await source('../src/modules/operations/ConstructionWorkspaceLists.tsx');
  const planning = await source('../src/modules/planning/PlanningPage.tsx');
  const handover = await source('../src/modules/handover/HandoverWorkspace.tsx');
  const warehouse = await source('../src/modules/warehouse/WarehousePage.tsx');
  const quality = await source('../src/modules/quality/QualityPage.tsx');

  assert.match(supplier, /<tr><th[^>]*><span className="sr-only">Thao tác<\/span><\/th><th[^>]*>Mã<\/th>/);
  assert.match(supplier, /<tr key=\{quotation\.id\}>\s*<td[^>]*><div className="flex gap-2">/);
  assert.match(acceptance, /<tr><th[^>]*><span className="sr-only">Thao tác<\/span><\/th><th[^>]*>#<\/th>/);
  assert.equal((operations.match(/<tr><Th><span className="sr-only">Thao tác<\/span><\/Th>/g) ?? []).length, 2);
  assert.match(planning, /<tr[^>]*>\s*<th[^>]*><span className="sr-only">Thao tác<\/span><\/th>\s*<th[^>]*>Công việc<\/th>/);
  assert.match(handover, /<tr><th[^>]*><span className="sr-only">Thao tác<\/span><\/th><th[^>]*>Nội dung<\/th>/);
  assert.match(handover, /return <tr><td[^>]*><ChecklistAction/);
  assert.match(warehouse, /<tr><th[^>]*><span className="sr-only">Thao tác<\/span><\/th><th[^>]*>Vật tư<\/th>/);
  assert.match(quality, /<tr><th[^>]*><span className="sr-only">Thao tác<\/span><\/th><th[^>]*>#<\/th>/);
});

test('manual responsive grids keep their action track first', async () => {
  const preparation = await source('../src/modules/planning/PreparationPage.tsx');
  const projectDetail = await source('../src/modules/projects/ProjectDetailPage.tsx');
  const projectWork = await source('../src/modules/projects/ProjectWorkTab.tsx');
  const projects = await source('../src/modules/projects/ProjectsPage.tsx');
  const evidence = await source('../src/modules/work-items/EvidenceGallery.tsx');
  const incoming = await source('../src/modules/work-items/IncomingAcceptancePanel.tsx');
  const materials = await source('../src/modules/work-items/MaterialSnapshotPanel.tsx');
  const surplus = await source('../src/modules/work-items/SurplusHandlingPanel.tsx');

  assert.match(preparation, /PREPARATION_TABLE_GRID = 'grid-cols-\[8rem_/);
  assert.match(preparation, /PREPARATION_TABLE_GRID\)\}>\s*<div><PreparationOpenButton/);
  assert.match(projectDetail, /grid-cols-\[72px_minmax\(260px,1\.6fr\)/);
  assert.match(projectDetail, /grid-cols-\[32px_120px_180px_minmax\(140px,1fr\)_150px\]/);
  assert.match(projectWork, /<tr>\s*<WorkTableHeader label="" \/>\s*<WorkTableHeader label="Trạng thái"/);
  assert.match(projectWork, /<tr[^>]*>\s*<td[^>]*>\s*<OpenWorkButton/);
  assert.match(projects, /PROJECT_EXPLORER_DESKTOP_GRID_CLASS = 'lg:grid-cols-\[112px_/);
  assert.match(evidence, /const PROJECT_EVIDENCE_LIST_GRID_CLASS = 'xl:grid-cols-\[8rem_/);
  assert.match(evidence, /PROJECT_EVIDENCE_LIST_GRID_CLASS\)\}>\s*<span><span className="sr-only">Thao tác<\/span><\/span>/);
  assert.match(evidence, /\)\}>\s*<div className="flex flex-wrap gap-2">/);
  assert.match(incoming, /grid-cols-\[44px_32px_1\.5fr_/);
  assert.match(materials, /xl:grid-cols-\[44px_1\.2fr_1fr_1\.2fr\]/);
  assert.match(surplus, /xl:grid-cols-\[auto_1fr_1fr_1\.2fr\]/);
});

test('manual table headers do not expose a visible action title', async () => {
  const files = [
    '../src/modules/acceptance/AcceptancePage.tsx',
    '../src/modules/boq/SupplierQuotationWorkspace.tsx',
    '../src/modules/contracts/ContractsPage.tsx',
    '../src/modules/operations/ConstructionWorkspaceLists.tsx',
    '../src/modules/planning/PlanningPage.tsx',
    '../src/modules/planning/PreparationPage.tsx',
    '../src/modules/projects/ProjectDetailPage.tsx',
    '../src/modules/projects/ProjectsPage.tsx',
    '../src/modules/work-items/EvidenceGallery.tsx',
  ];

  for (const file of files) {
    const contents = await source(file);
    assert.doesNotMatch(contents, /<th[^>]*>Thao tác<\/th>|<span className="text-right">Thao tác<\/span>|headers=\{\[[^\]]*'Thao tác'/, file);
  }
});
