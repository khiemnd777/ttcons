import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

async function source(path: string) {
  return readFile(new URL(path, import.meta.url), 'utf8');
}

const genericPickerPages = [
  '../src/modules/operations/OperationsPage.tsx',
  '../src/modules/rework/ReworkPage.tsx',
  '../src/modules/acceptance/AcceptancePage.tsx',
  '../src/modules/handover/HandoverPage.tsx',
  '../src/modules/warehouse/WarehousePage.tsx',
  '../src/modules/planning-daily-reports/PlanningDailyReportsPage.tsx',
  '../src/modules/planning-checklists/PlanningChecklistsPage.tsx',
  '../src/modules/meeting-minutes/MeetingMinutesPage.tsx',
];

test('shared async picker owns all ordinary project scopes', async () => {
  for (const pagePath of genericPickerPages) {
    const page = await source(pagePath);
    assert.match(page, /import \{ AsyncProjectPicker \} from '..\/_shared\/AsyncProjectPicker'/);
    assert.match(page, /<AsyncProjectPicker[\s\S]*endpoint="\/projects\/options"[\s\S]*queryKey="project-options"/);
  }
});

test('aggregate project scopes preserve their explicit empty state', async () => {
  for (const pagePath of genericPickerPages.slice(-4)) {
    const page = await source(pagePath);
    assert.match(page, /emptyLabel="Tất cả công trình"/);
  }
});
