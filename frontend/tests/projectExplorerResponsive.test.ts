import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

async function source(path: string) {
  return readFile(new URL(path, import.meta.url), 'utf8');
}

test('project explorer separates mobile project cards while retaining desktop dividers', async () => {
  const projectsPage = await source('../src/modules/projects/ProjectsPage.tsx');

  assert.match(
    projectsPage,
    /className="space-y-3 bg-slate-50\/80 p-3 lg:space-y-0 lg:bg-transparent lg:p-0 lg:divide-y lg:divide-line"/,
  );
  assert.match(
    projectsPage,
    /className="overflow-hidden rounded-lg border border-line bg-white shadow-sm lg:rounded-none lg:border-0 lg:shadow-none"/,
  );
});

test('project explorer exposes equivalent desktop and mobile sort controls', async () => {
  const projectsPage = await source('../src/modules/projects/ProjectsPage.tsx');

  assert.match(projectsPage, /aria-label="Sắp xếp công trình"/);
  assert.match(projectsPage, /focus-ring lg:hidden/);
  assert.match(projectsPage, /ProjectExplorerSortHeader label="Cây công trình" sortKey="project"/);
  assert.match(projectsPage, /ProjectExplorerSortHeader label="Trạng thái" sortKey="status"/);
  assert.match(projectsPage, /ProjectExplorerSortHeader label="Phụ trách" sortKey="owner"/);
  assert.match(projectsPage, /ProjectExplorerSortHeader label="Ngày tạo" sortKey="created_at"/);
  assert.match(projectsPage, /rounded-sm text-left uppercase hover:text-ink focus-ring/);
});

test('project explorer keeps blank actions first and creation date after progress', async () => {
  const projectsPage = await source('../src/modules/projects/ProjectsPage.tsx');
  const domainTypes = await source('../src/lib/types/domain.ts');

  assert.match(projectsPage, /<span><span className="sr-only">Thao tác<\/span><\/span>\s*<ProjectExplorerSortHeader label="Cây công trình"/);
  assert.match(projectsPage, /<span>Tiến độ<\/span>\s*<ProjectExplorerSortHeader label="Ngày tạo" sortKey="created_at"[^/]+\/>/);
  assert.match(projectsPage, /<span className="font-medium text-slate-500 lg:hidden">Ngày tạo: <\/span>\s*\{dateText\(project\.created_at\)\}/);
  assert.match(projectsPage, /const PROJECT_EXPLORER_DESKTOP_GRID_CLASS = 'lg:grid-cols-\[112px_minmax\(0,1\.6fr\)_135px_100px_112px_100px\]';/);
  assert.equal((projectsPage.match(/\$\{PROJECT_EXPLORER_DESKTOP_GRID_CLASS\}/g) ?? []).length, 2);
  assert.match(projectsPage, /className="flex min-w-0 items-center gap-2"/);
  assert.match(projectsPage, /className="h-1\.5 min-w-0 flex-1 overflow-hidden rounded-full bg-slate-100"/);
  assert.match(projectsPage, /className="shrink-0 text-xs font-semibold tabular-nums text-slate-600"/);
  assert.match(projectsPage, /className="whitespace-nowrap text-sm tabular-nums text-slate-600"/);
  assert.match(domainTypes, /export type Project =[^}]+created_at\?: string;/s);
});
