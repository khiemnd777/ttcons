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
