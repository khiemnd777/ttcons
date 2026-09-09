import assert from 'node:assert/strict';
import test from 'node:test';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';
import { build } from 'esbuild';
import { createElement } from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import { discussionWorkActionLabel, discussionWorkPath, discussionWorkReadOnly, discussionWorkSurface } from '../src/modules/work-items/discussionWorkDomain.ts';
import type { WorkItem } from '../src/lib/types/domain.ts';

const item = (extra: Partial<WorkItem> = {}) => ({ id: 'task', project_id: 'project', title: 'Checklist thiết kế', status: 'ready', can_act: true, execution_target: { kind: 'capture', workspace: 'planning' }, ...extra } as WorkItem);

test('discussion routes only supported source editors and preserves domain boundaries', () => {
  assert.equal(discussionWorkSurface(item()), 'form');
  assert.equal(discussionWorkSurface(item({ run_node_business_code: 'design' })), 'design');
  assert.equal(discussionWorkSurface(item({ execution_target: { kind: 'evidence', workspace: 'preparation' } })), 'evidence');
  for (const workspace of ['warehouse', 'boq', 'contracts', 'procurement', 'acceptance', 'project_finance']) {
    assert.equal(discussionWorkSurface(item({ execution_target: { kind: 'mixed', workspace } })), 'external');
  }
  assert.equal(discussionWorkSurface(item({ execution_target: { kind: 'approval', workspace: 'work_item' } })), 'external');
});

test('permission and current workflow readiness control the call to action', () => {
  assert.equal(discussionWorkReadOnly(item()), false);
  assert.equal(discussionWorkActionLabel(item()), 'Cập nhật hồ sơ');
  for (const extra of [{ can_act: false }, { capabilities: { can_act: false } }, { readiness: 'upcoming' }, { status: 'completed' }, { status: 'waiting_approval' }]) {
    assert.equal(discussionWorkReadOnly(item(extra)), true);
    assert.equal(discussionWorkActionLabel(item(extra)), 'Xem công việc');
  }
  const params = new URL('https://local' + discussionWorkPath('project & query')).searchParams;
  assert.equal(params.get('project_id'), 'project & query');
  assert.equal(params.get('queue'), 'responsible');
  assert.equal(params.get('bucket'), 'need_action');
});

const require = createRequire(import.meta.url);
const { QueryClient, QueryClientProvider } = require('@tanstack/react-query');
const bundle = await build({
  stdin: { resolveDir: fileURLToPath(new URL('../src', import.meta.url)), contents: `export { WorkflowFormSnapshot } from './modules/work-items/WorkflowFormPanel'; export { DiscussionWorkUpdateCard } from './modules/work-items/DiscussionWorkUpdateCard'; export { ToastProvider } from './components/ToastProvider';` },
  bundle: true, write: false, platform: 'node', format: 'cjs', packages: 'external', jsx: 'automatic',
  loader: { '.css': 'empty' }, define: { 'import.meta.env': '{}', 'import.meta.url': '"file:///fixture"' },
  plugins: [{ name: 'test-actor', setup(builder) {
    builder.onLoad({ filter: /AuthContext\.tsx$/ }, () => ({ contents: `export const useAuth = () => ({ user: { id: 'actor' }, hasPermission: () => false });`, loader: 'tsx' }));
  } }],
});
const module = { exports: {} as Record<string, any> };
new Function('require', 'module', 'exports', bundle.outputFiles[0].text)(require, module, module.exports);
function render(name: string, props: object) {
  const client = new QueryClient();
  try { return renderToStaticMarkup(createElement(QueryClientProvider, { client }, createElement(module.exports.ToastProvider, null, createElement(module.exports[name], props)))); }
  finally { client.clear(); }
}

test('historical forms render pinned fields and answers through the existing field controls', () => {
  const html = render('WorkflowFormSnapshot', { fields: [
    { id: 'a', field_key: 'note', field_type: 'textarea', label: 'Nội dung bản cũ' },
    { id: 'b', field_key: 'file', field_type: 'evidence_reference', label: 'Tệp của bản cũ' },
  ], answers: { note: 'Ghi chú <b>ban đầu</b>', file: 'evidence-original' } });
  assert.match(html, /Nội dung bản cũ/);
  assert.match(html, /Ghi chú &lt;b&gt;ban đầu&lt;\/b&gt;/);
  assert.match(html, /textarea[^>]*disabled/);
  assert.match(html, /Xem minh chứng Tệp của bản cũ/);
  assert.doesNotMatch(html, /Chốt hồ sơ|Lưu nháp|<b>ban đầu/);
});

test('update cards distinguish uploaded drafts from released design packages', () => {
  const base = { id: 'update', activity_type: 'work_update', work_item_title: 'Thiết kế tầng 1' };
  const card = (status: string) => render('DiscussionWorkUpdateCard', { item: { ...base, metadata_json: { work_update: { source_type: 'design_release', revision_label: 'R01', status } } } });
  assert.match(card('draft'), /Nháp · chưa phát hành/);
  assert.match(card('released'), /Đã phát hành/);
  assert.match(card('released'), /Hồ sơ thiết kế · R01/);
  assert.match(card('released'), /Xem bản cập nhật/);
  assert.doesNotMatch(card('released'), /Phê duyệt|Hoàn thành công việc/);
});
