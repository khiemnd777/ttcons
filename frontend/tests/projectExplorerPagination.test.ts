import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const source = readFileSync(new URL('../src/modules/projects/ProjectsPage.tsx', import.meta.url), 'utf8');

test('project explorer uses the dedicated paginated contract without changing legacy project consumers', () => {
  assert.match(source, /get<ProjectExplorerPage>\(`\/projects\/explorer\?\$\{explorerParams\}`\)/);
  assert.match(source, /page_size: String\(pageSize\)/);
  assert.match(source, /sort: projectSort\?\.key \?\? 'created_at'/);
  assert.match(source, /\[25, 50, 100\]/);
  assert.match(source, /Hiển thị/);
  assert.doesNotMatch(source, /get<Project\[\]>\('\/projects'\)/);
});

test('project explorer invalidates browser cache locally and across the authenticated stream', () => {
  assert.match(source, /queryClient\.invalidateQueries\(\{ queryKey: \['project-explorer'\] \}\)/);
  assert.match(source, /streamSSE\('\/projects\/explorer\/stream'/);
  assert.match(source, /message\.event !== 'project_explorer_changed'/);
  assert.match(source, /staleTime: 0/);
  assert.match(source, /refetchInterval: streamStatus === 'connected' \? false : 60_000/);
});
