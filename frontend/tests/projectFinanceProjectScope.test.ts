import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

async function source(path: string) {
  return readFile(new URL(path, import.meta.url), 'utf8');
}

test('global project finance uses a draft/applied asynchronous project scope', async () => {
  const financePage = await source('../src/modules/project-finance/ProjectFinancePage.tsx');

  assert.doesNotMatch(financePage, /ProjectSelector|\/projects'\)|projects\.data\?\.\[0\]/);
  assert.match(financePage, /const \[draftProjectId, setDraftProjectId\] = useState\(requestedProjectId\)/);
  assert.match(financePage, /const projectId = id \?\? \(global \? requestedProjectId : ''\)/);
  assert.match(financePage, /params\.set\('project', draftProjectId\)/);
  assert.match(financePage, /params\.delete\('project'\)/);
  assert.match(financePage, /endpoint="\/project-finance\/projects"/);
  assert.match(financePage, /allowEmptySelection=\{false\}/);
  assert.match(financePage, /applyDisabled=\{!draftProjectId \|\| draftProjectId === projectId\}/);
  assert.match(financePage, /aria-label="Bộ lọc công trình tài chính"/);
  assert.match(financePage, /queryKey: \['project-finance', projectId\]/);
  assert.match(financePage, /`\/projects\/\$\{projectId\}\/project-finance\/export`/);
});

test('project finance keeps route-scoped behavior and moves create actions into the workspace toolbar', async () => {
  const financePage = await source('../src/modules/project-finance/ProjectFinancePage.tsx');
  const headerEnd = financePage.indexOf('</header>');
  const toolbarStart = financePage.indexOf('aria-label="Nội dung tài chính công trình"');
  const costAction = financePage.indexOf('Ghi nhận chi phí vật tư', headerEnd);
  const requestAction = financePage.indexOf('Lập đề nghị thu/chi', headerEnd);

  assert.ok(headerEnd > 0);
  assert.ok(toolbarStart > headerEnd);
  assert.ok(costAction > toolbarStart);
  assert.ok(requestAction > toolbarStart);
  assert.match(financePage, /\{id \? <ProjectBackLink \/> : null\}/);
  assert.match(financePage, /\{global \? <section/);
  assert.match(financePage, /order-2 flex min-w-0 gap-1 overflow-x-auto sm:order-1/);
  assert.match(financePage, /order-1 flex flex-wrap gap-2 pb-0 sm:order-2/);
});
