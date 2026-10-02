import assert from 'node:assert/strict';
import test from 'node:test';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';
import { build } from 'esbuild';
import { createElement } from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import { companyDiscussionPayload } from '../src/modules/work-items/activityComposerPolicy.ts';
import { workflowFormArtifactAccess } from '../src/modules/work-items/workflowFormArtifactAccess.ts';
import { workItemObserverReadOnly } from '../src/modules/work-items/workItemObserverAccess.ts';
import type { ActivityPayload } from '../src/lib/types/domain.ts';

const require = createRequire(import.meta.url);
const { QueryClient, QueryClientProvider } = require('@tanstack/react-query');
const { MemoryRouter } = require('react-router-dom');
const bundle = await build({
  stdin: { resolveDir: fileURLToPath(new URL('../src', import.meta.url)), contents: `export { WorkItemDetailWorkspace } from './modules/work-items/WorkItemDetailDialog'; export { ActivityComposer } from './modules/work-items/ActivityComposer'; export { ToastProvider } from './components/ToastProvider';` },
  bundle: true, write: false, platform: 'node', format: 'cjs', packages: 'external', jsx: 'automatic',
  loader: { '.css': 'empty' }, define: { 'import.meta.env': '{}', 'import.meta.url': '"file:///fixture"' },
  plugins: [{ name: 'test-actor', setup(builder) {
    builder.onLoad({ filter: /AuthContext\.tsx$/ }, () => ({ contents: `export const useAuth = () => ({ user: { id: 'deputy' }, hasPermission: (key) => key === 'project.supervision.view' ? globalThis.__supervisionTestActor !== false : true });`, loader: 'tsx' }));
  } }],
});
const module = { exports: {} as Record<string, any> };
new Function('require', 'module', 'exports', bundle.outputFiles[0].text)(require, module, module.exports);
function render(name: string, props: object, seed?: (client: any) => void) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  seed?.(client);
  try { return renderToStaticMarkup(createElement(QueryClientProvider, { client }, createElement(MemoryRouter, null, createElement(module.exports.ToastProvider, null, createElement(module.exports[name], props))))); }
  finally { client.clear(); }
}

const item = { id: 'task', project_id: 'project', title: 'Hồ sơ Giám sát', status: 'in_progress', work_type: 'document_upload', can_act: false, available_actions: ['complete'], evidence: [], activities: [] };

test('active oversight work keeps evidence reading while hiding uploads, composer and actions', () => {
  const html = render('WorkItemDetailWorkspace', { item, readOnly: true }, (client) => client.setQueryData(['work-item', 'task'], item));
  assert.match(html, /Sổ minh chứng/);
  assert.match(html, /Theo dõi công việc · Chỉ xem/);
  assert.doesNotMatch(html, /Tải minh chứng|Gửi hoạt động|Thêm ghi chú \/ lý do|data-workflow-form-mobile-actions/);
  (globalThis as any).__supervisionTestActor = false;
  const ordinary = render('WorkItemDetailWorkspace', { item: { ...item, can_act: true }, readOnly: false });
  delete (globalThis as any).__supervisionTestActor;
  assert.match(ordinary, /Tải minh chứng/);
});

test('canonical workspace automatically keeps a company observer read-only before a fresh detail check', () => {
  const html = render('WorkItemDetailWorkspace', { item }, (client) => client.setQueryData(['work-item', 'task'], item));
  assert.match(html, /Sổ minh chứng/);
  assert.doesNotMatch(html, /Tải minh chứng|Thêm ghi chú \/ lý do/);
  const stalePrivileged = render('WorkItemDetailWorkspace', { item: { ...item, can_act: true } }, (client) => client.setQueryData(['work-item', 'task'], { ...item, can_act: true }));
  assert.doesNotMatch(stalePrivileged, /Tải minh chứng/);
});

test('fresh action or decision capabilities preserve ordinary assigned Deputy and manager behavior', () => {
  const context = { explicitReadOnly: false, hasCompanySupervision: true, detailFresh: true };
  assert.equal(workItemObserverReadOnly({ ...context, canAct: false, canDecide: false }), true);
  assert.equal(workItemObserverReadOnly({ ...context, canAct: true, canDecide: false }), false);
  assert.equal(workItemObserverReadOnly({ ...context, canAct: false, canDecide: true }), false);
  assert.equal(workItemObserverReadOnly({ ...context, detailFresh: false, canAct: true }), true);
  assert.equal(workItemObserverReadOnly({ ...context, explicitReadOnly: true, canAct: true, canDecide: true }), true);
  assert.equal(workItemObserverReadOnly({ ...context, hasCompanySupervision: false, detailFresh: false }), false);
});

test('observer form capability override suppresses signatures and generated/deleted PDFs', () => {
  assert.deepEqual(workflowFormArtifactAccess({ available: true, instance: { id: 'form', status: 'submitted' } as any, capabilities: { can_add_confirmation: true, can_generate_pdf: true, can_delete_pdf: true } }, true), { canAddConfirmation: false, canGeneratePDF: false, canDeletePDF: false });
});

test('oversight workflow and specialized surfaces render their shared readers without editors', () => {
  const formItem = { ...item, surface_kind: 'workflow_form', workflow_form_template_code: 'site-check', workflow_form_template_name: 'Checklist công trình' };
  const form = { available: true, template: { code: 'site-check', name: 'Checklist công trình' }, version: { version_no: 1 }, instance: { id: 'form', title: 'Checklist công trình', status: 'draft', current_revision: 1 }, fields: [{ id: 'field', field_key: 'notes', label: 'Nội dung hiện trường', field_type: 'text', required: true }], answers: { notes: 'Nội dung đã ghi' }, capabilities: { can_amend: true, can_add_confirmation: true, can_generate_pdf: true, can_delete_pdf: true } };
  const html = render('WorkItemDetailWorkspace', { item: formItem, readOnly: true }, (client) => { client.setQueryData(['work-item', 'task'], formItem); client.setQueryData(['work-item-form', 'task'], form); });
  assert.match(html, /Nội dung hiện trường/);
  assert.doesNotMatch(html, /data-workflow-form-submit|data-workflow-form-mobile-actions|Chỉnh sửa hồ sơ đã chốt|Tải minh chứng/);
  const specialized = { ...item, surface_kind: 'incoming_acceptance', title: 'Nghiệm thu vật tư Giám sát' };
  const specializedHTML = render('WorkItemDetailWorkspace', { item: specialized, readOnly: true }, (client) => client.setQueryData(['work-item', 'task'], specialized));
  assert.match(specializedHTML, /Sổ minh chứng/);
  assert.doesNotMatch(specializedHTML, /Tải minh chứng|Tạo biên bản|Lưu biên bản/);
});

test('company discussion renders only comment and project/work-item targets in both densities', () => {
  for (const density of ['comfortable', 'compact']) {
    const props = { mode: 'project', variant: 'chat', density, allowTarget: true, onSubmit: async () => {}, targetOptions: [ { scope_type: 'work_item', scope_id: 'task', label: 'Công việc được xem' }, { scope_type: 'boq_item', scope_id: 'boq', label: 'BOQ không được viết' } ] };
    const html = render('ActivityComposer', { ...props, commentOnly: true });
    assert.match(html, /Gửi trao đổi/);
    assert.match(html, /value="work_item"/);
    assert.doesNotMatch(html, /value="boq_item"|value="decision"|value="reminder"|Loại tệp đính kèm|Ngày nhắc|Hạn điểm chặn/);
    const ordinary = render('ActivityComposer', props);
    assert.match(ordinary, /value="boq_item"/);
  }
});

test('comment-only payload clears stale advanced input and rejects another business target', () => {
  const payload: ActivityPayload = { activity_type: 'decision', comment: 'Trao đổi', visibility: 'internal', scope_type: 'work_item', scope_id: 'task', attachments: [{ type: 'evidence', id: 'file' }], metadata: { owner_id: 'other', due_at: '2026-10-03' } };
  assert.deepEqual(companyDiscussionPayload(payload, true), { ...payload, activity_type: 'comment', visibility: 'project', attachments: [], metadata: {} });
  assert.throws(() => companyDiscussionPayload({ ...payload, scope_type: 'boq_item' }, true));
  assert.equal(companyDiscussionPayload(payload, false), payload);
});
