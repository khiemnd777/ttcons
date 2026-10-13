import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';
import { build } from 'esbuild';
import { createElement } from 'react';
import { renderToStaticMarkup } from 'react-dom/server';

test('project-detail tabs pair every project workspace surface with its Lucide icon', async () => {
  const projectDetail = await readFile(new URL('../src/modules/projects/ProjectDetailPage.tsx', import.meta.url), 'utf8');
  const workspaceTabs = await readFile(new URL('../src/components/WorkspaceTabs.tsx', import.meta.url), 'utf8');

  assert.match(projectDetail, /\{ id: 'overview', label: 'Tổng quan', icon: LayoutDashboard \}/);
  assert.match(projectDetail, /\{ id: 'workflow', label: 'Quy trình', icon: GitBranch, count: timeline\.length \}/);
  assert.match(projectDetail, /\{ id: 'work', label: 'Danh sách công việc', icon: ListTodo, count: allWorkItems\.length \}/);
  assert.doesNotMatch(projectDetail, /id: 'activity'/);
  assert.match(projectDetail, /\{ id: 'files', label: 'Hồ sơ', icon: FileText, count: numberValue\(data\.evidence_summary\?\.total \?\? latestEvidence\.length\) \}/);
  assert.match(projectDetail, /\{ id: 'audit', label: 'Nhật ký', icon: ScrollText \}/);
  assert.match(projectDetail, /\{ id: 'members', label: 'Thành viên', icon: Users, count: members\.data\?\.summary\.active \?\? 0 \}/);
  assert.match(projectDetail, /<WorkspaceTabs/);
  assert.match(workspaceTabs, /<Icon className="h-4 w-4 shrink-0" aria-hidden="true" \/>/);
});

test('project overview uses one compact toolbar without repeating project identity or lifecycle actions', async () => {
  const projectDetail = await readFile(new URL('../src/modules/projects/ProjectDetailPage.tsx', import.meta.url), 'utf8');

  assert.match(projectDetail, /<h1 className="sr-only">\{activeTabLabel\} · \{project\.code/);
  assert.match(projectDetail, /data-project-overview-toolbar/);
  assert.match(projectDetail, /ariaLabel="Nội dung công trình"/);
  assert.match(projectDetail, /pathname: `\/projects\/\$\{id\}\/discussion`/);
  assert.match(projectDetail, /nextParams\.delete\('tab'\)/);
  assert.match(projectDetail, /actions=\{<ProjectDetailToolbarActions projectId=\{id\} activeTab=\{activeTab\} \/>\}/);
  assert.match(projectDetail, />\s*Mở kế hoạch\s*<\/Link>/);
  assert.match(projectDetail, /<ProjectOverviewActionBand/);
  assert.doesNotMatch(projectDetail, /function HeaderActions/);
  assert.doesNotMatch(projectDetail, /title=\{`\$\{project\.code/);
});

const require = createRequire(import.meta.url);
const bundle = await build({ stdin: { resolveDir: fileURLToPath(new URL('../src/', import.meta.url)), contents: `export { ProjectDetailToolbarActions } from './modules/projects/ProjectDetailPage';` }, bundle: true, write: false, platform: 'node', format: 'cjs', packages: 'external', jsx: 'automatic', loader: { '.css': 'empty' }, define: { 'import.meta.env': '{}', 'import.meta.url': '"file:///fixture"' } });
const module = { exports: {} as Record<string, any> };
new Function('require', 'module', 'exports', bundle.outputFiles[0].text)(require, module, module.exports);
const { StaticRouter } = require('react-router-dom');

test('project toolbar keeps planning placement and has no report chooser on any project tab', async () => {
  const source = await readFile(new URL('../src/modules/projects/ProjectDetailPage.tsx', import.meta.url), 'utf8');
  assert.doesNotMatch(source, /BMGSExportDialog|bmgsExportOpen|Xuất Excel BM-GS/);
  for (const tab of ['overview', 'workflow', 'work', 'files', 'audit', 'members']) {
    const tree = module.exports.ProjectDetailToolbarActions({ projectId: 'project', activeTab: tab });
    const html = renderToStaticMarkup(createElement(StaticRouter, { location: '/projects/project' }, tree));
    assert.doesNotMatch(html, /Xuất Excel/);
    if (tab === 'workflow') assert.doesNotMatch(html, /Mở kế hoạch/); else assert.match(html, /href="\/projects\/project\/planning"/);
  }
});
