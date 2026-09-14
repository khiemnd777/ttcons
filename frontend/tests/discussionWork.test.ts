import assert from 'node:assert/strict';
import test from 'node:test';
import { readFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';
import { build } from 'esbuild';
import { createElement } from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import {
  discussionWorkActionLabel,
  discussionWorkDetailLoadState,
  discussionWorkPath,
  discussionWorkReadOnly,
  discussionWorkSurface,
  discussionWorkflowActionLabel,
} from '../src/modules/work-items/discussionWorkDomain.ts';
import type { WorkItem } from '../src/lib/types/domain.ts';

const item = (extra: Partial<WorkItem> = {}) => ({ id: 'task', project_id: 'project', title: 'Checklist thiết kế', status: 'ready', can_act: true, execution_target: { kind: 'capture', workspace: 'planning' }, ...extra } as WorkItem);
const discussionWorkDialogSource = readFileSync(new URL('../src/modules/work-items/DiscussionWorkDialog.tsx', import.meta.url), 'utf8');
const workItemDetailSource = readFileSync(new URL('../src/modules/work-items/WorkItemDetailDialog.tsx', import.meta.url), 'utf8');
const workItemPrimarySurfaceSource = readFileSync(new URL('../src/modules/work-items/WorkItemPrimarySurface.tsx', import.meta.url), 'utf8');

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

test('project workflow action labels separate execution, decision, blocker, and read-only states', () => {
  assert.equal(discussionWorkflowActionLabel(item()), 'Xử lý việc ưu tiên');
  assert.equal(discussionWorkflowActionLabel(item({ status: 'blocked' })), 'Xử lý vướng mắc');
  assert.equal(discussionWorkflowActionLabel(item({ status: 'waiting_approval', can_act: false, can_decide: true })), 'Mở yêu cầu duyệt');
  assert.equal(discussionWorkflowActionLabel(item({ can_act: false })), 'Xem công việc');
  assert.equal(discussionWorkflowActionLabel(item({ status: 'completed' })), 'Xem công việc');
  assert.equal(discussionWorkflowActionLabel(item({ readiness: 'upcoming' })), 'Xem công việc');
});

test('discussion work dialog rechecks warm cache and opens the full workspace directly', () => {
  assert.match(discussionWorkDialogSource, /refetchOnMount:\s*'always'/);
  assert.match(discussionWorkDialogSource, /size="workspace"/);
  assert.match(discussionWorkDialogSource, /<WorkItemDetailWorkspace/);
  assert.match(discussionWorkDialogSource, /onDirtyChange=\{setDirty\}/);
  assert.match(discussionWorkDialogSource, /onBusyChange=\{setBusy\}/);
  assert.doesNotMatch(discussionWorkDialogSource, /Xem toàn bộ & lịch sử/);
  assert.doesNotMatch(discussionWorkDialogSource, /Quay lại hồ sơ/);
  assert.doesNotMatch(discussionWorkDialogSource, /Điều kiện & chốt công việc/);
  assert.equal(discussionWorkDetailLoadState({ isFetchedAfterMount: false, isFetching: false, isError: false, projectId: 'project' }, 'project'), 'loading');
  assert.equal(discussionWorkDetailLoadState({ isFetchedAfterMount: false, isFetching: true, isError: true, projectId: 'project' }, 'project'), 'loading');
  assert.equal(discussionWorkDetailLoadState({ isFetchedAfterMount: true, isFetching: false, isError: false, projectId: 'project' }, 'project'), 'ready');
  assert.equal(discussionWorkDetailLoadState({ isFetchedAfterMount: true, isFetching: false, isError: true, projectId: 'project' }, 'project'), 'error');
  assert.equal(discussionWorkDetailLoadState({ isFetchedAfterMount: true, isFetching: false, isError: false, projectId: 'another-project' }, 'project'), 'error');
});

test('discussion work dialog preserves the procurement source dialog and its explicit work-detail path', () => {
  assert.match(discussionWorkDialogSource, /surface === 'procurement' && !showProcurementWorkDetails/);
  assert.match(discussionWorkDialogSource, /<ProcurementDialog/);
  assert.match(discussionWorkDialogSource, /onShowWorkDetails=\{\(\) => setShowProcurementWorkDetails\(true\)\}/);
  assert.match(discussionWorkDialogSource, /Quay lại yêu cầu mua/);
});

test('full discussion workspace owns requirement navigation and propagates editor guards', () => {
  assert.match(workItemDetailSource, /<WorkItemRequirementPanel rows=\{requirementRows\} onNavigate=\{navigateToRequirement\} \/>/);
  assert.match(workItemDetailSource, /buildRequirementRows\(detailItem, evidence, actions, futureReadOnly\)/);
  assert.match(workItemDetailSource, /onDirtyChange=\{guardEditing \? setPrimaryDirty : undefined\}/);
  assert.match(workItemDetailSource, /onBusyChange=\{guardEditing \? setSupportBusy : undefined\}/);
  assert.match(workItemPrimarySurfaceSource, /onDirtyChange=\{onDirtyChange\}/);
  assert.match(workItemPrimarySurfaceSource, /onBusyChange=\{onBusyChange\}/);
});

const require = createRequire(import.meta.url);
const { QueryClient, QueryClientProvider } = require('@tanstack/react-query');
const bundle = await build({
  stdin: { resolveDir: fileURLToPath(new URL('../src', import.meta.url)), contents: `export { WorkflowFormSnapshot } from './modules/work-items/WorkflowFormPanel'; export { DiscussionWorkUpdateCard } from './modules/work-items/DiscussionWorkUpdateCard'; export { DiscussionWorkflowActionCard } from './modules/work-items/DiscussionWorkflowActionCard'; export { WorkItemRequirementPanel } from './modules/work-items/WorkItemRequirementPanel'; export { workflowFormActivityComment } from './modules/work-items/workItemPresentation'; export { ToastProvider } from './components/ToastProvider';` },
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

test('required-condition panel opens blockers and keeps permission informational', () => {
  const html = render('WorkItemRequirementPanel', {
    rows: [
      {
        id: 'completion-workflow_form',
        label: 'Hồ sơ nghiệp vụ bắt buộc',
        description: 'Hồ sơ nghiệp vụ bắt buộc chưa được chốt.',
        guidance: 'Hoàn tất các trường bắt buộc, sau đó chọn Chốt hồ sơ.',
        actionLabel: 'Hoàn tất hồ sơ',
        target: { kind: 'workflow_form', focus: 'first_issue' },
        state: 'blocked',
      },
      {
        id: 'permission',
        label: 'Quyền thao tác',
        description: 'Cần quyền: Cập nhật kế hoạch.',
        guidance: 'Liên hệ Quản lý dự án hoặc Admin để kiểm tra quyền.',
        kind: 'permission',
        state: 'neutral',
      },
    ],
    onNavigate: () => undefined,
    openWhenBlocked: true,
  });
  assert.match(html, /<details[^>]* open=""/);
  assert.match(html, /Điều kiện bắt buộc/);
  assert.match(html, /Còn 1 điều kiện cần xử lý/);
  assert.match(html, /Hoàn tất hồ sơ/);
  assert.match(html, /Thông tin truy cập/);
  assert.match(html, /mt-2 block sm:hidden/);
  assert.match(html, /hidden self-center sm:block/);

  const clearHtml = render('WorkItemRequirementPanel', {
    rows: [{
      id: 'completion-workflow_form',
      label: 'Hồ sơ nghiệp vụ bắt buộc',
      description: 'Hồ sơ nghiệp vụ đúng phiên bản đã được chốt.',
      guidance: 'Mở hồ sơ để xem phiên bản đã đáp ứng điều kiện.',
      state: 'done',
    }],
    onNavigate: () => undefined,
    openWhenBlocked: true,
  });
  assert.doesNotMatch(clearHtml, /<details[^>]* open=""/);
  assert.match(clearHtml, /Không có điều kiện đang chặn/);
});

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

test('design batch card owns its inline file copy while retaining the persisted activity preview', () => {
  assert.equal(module.exports.workflowFormActivityComment({
    activity_type: 'work_update',
    comment: 'Đã thêm 2 tệp thiết kế',
    metadata_json: { work_update: { event_key: 'files:batch-id' } },
  }), '');
  assert.equal(module.exports.workflowFormActivityComment({
    activity_type: 'work_update',
    comment: 'Đã thêm tệp thiết kế · legacy.pdf',
    metadata_json: { work_update: { event_key: 'file:evidence-id' } },
  }), 'Đã thêm tệp thiết kế · legacy.pdf');
});

test('discussion workflow card renders the startup handoff without executing a transition', () => {
  const html = render('DiscussionWorkflowActionCard', {
    projectId: 'project',
    workflow: { hasPipeline: false, onOpenStartup: () => undefined },
    onOpenWork: () => undefined,
    onDiscuss: () => undefined,
  });
  assert.match(html, /Bước tiếp theo/);
  assert.match(html, /Hoàn tất thiết lập công trình/);
  assert.match(html, /Thông tin → Nhân sự → Thiết lập/);
  assert.match(html, /Hoàn tất thiết lập/);
  assert.doesNotMatch(html, /Bắt đầu quy trình/);
});
