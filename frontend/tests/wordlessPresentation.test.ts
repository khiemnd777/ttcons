import assert from 'node:assert/strict';
import test from 'node:test';
import { createRequire } from 'node:module';
import { readFile } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
import { build } from 'esbuild';
import { createElement as h } from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import { MemoryRouter } from 'react-router-dom';

const require = createRequire(import.meta.url);
const bundle = await build({
  stdin: { resolveDir: fileURLToPath(new URL('../src/', import.meta.url)), contents: `
    export { WordlessScope, WordlessExplanation } from './components/Wordless';
    export { Dialog } from './components/Dialog';
    export { ContextHelp } from './components/ContextHelp';
    export { LabelWithHelp } from './components/LabelWithHelp';
    export { WordlessLabel } from './components/Wordless';
    export { WorkflowFieldControl, WorkflowFormActionCenter } from './modules/work-items/WorkflowFormPanel';
    export { WorkItemWorkspaceHeader } from './modules/work-items/WorkItemDetailDialog';
    export { ProjectTaskContext } from './modules/projects/ProjectTaskContext';
    export { WordlessBannerScope, BannerLabel } from './components/Wordless';
    export { SourceReportContextBanner } from './modules/operations/SourceReportContextBanner';
    export { DiscussionWorkflowActionCard } from './modules/work-items/DiscussionWorkflowActionCard';
    export { AcceptanceEditor } from './modules/acceptance/AcceptanceEditor';
    export { ToastProvider } from './components/ToastProvider';
    export { SettingField } from './modules/settings/SettingsPage';
    export { SystemSettingsPanel } from './modules/settings/SettingsPage';
    export { SettingsBackupPanel } from './modules/settings/SettingsBackupPanel';
    export { StatisticsReportHeading } from './modules/reports/ReportsPage';
    export { reportDefinitions } from './modules/reports/reportRegistry';
  ` },
  bundle: true, write: false, platform: 'node', format: 'cjs', packages: 'external', jsx: 'automatic',
  loader: { '.css': 'empty' }, define: { 'import.meta.env': '{}', 'import.meta.url': '"file:///fixture"' },
  plugins: [{ name: 'export-real-presentation', setup(b) {
    b.onLoad({ filter: /SettingsPage\.tsx$/ }, async ({ path }) => ({ contents: `${await readFile(path, 'utf8')}\nexport { SettingField, SystemSettingsPanel };`, loader: 'tsx' }));
    b.onLoad({ filter: /\/ReportsPage\.tsx$/ }, async ({ path }) => ({ contents: `${await readFile(path, 'utf8')}\nexport { StatisticsReportHeading };`, loader: 'tsx' }));
    b.onLoad({ filter: /WorkflowFormPanel\.tsx$/ }, async ({ path }) => ({ contents: `${await readFile(path, 'utf8')}\nexport { WorkflowFieldControl, WorkflowFormActionCenter };`, loader: 'tsx' }));
    b.onLoad({ filter: /WorkItemDetailDialog\.tsx$/ }, async ({ path }) => ({ contents: `${await readFile(path, 'utf8')}\nexport { WorkItemWorkspaceHeader };`, loader: 'tsx' }));
  } }],
});
const mod = { exports: {} as Record<string, any> };
let taskQuery: any = {};
new Function('require', 'module', 'exports', bundle.outputFiles[0].text)((name: string) => name === '@tanstack/react-query'
  ? { ...require(name), useQuery: () => taskQuery, useQueryClient: () => ({ invalidateQueries() {} }) }
  : require(name), mod, mod.exports);
function render(component: string, props: any, path: string) {
  return renderToStaticMarkup(h(MemoryRouter, { initialEntries: [path] }, h(mod.exports.WordlessScope, null, h(mod.exports.WordlessBannerScope, null, h(mod.exports[component], props)))));
}

test('real field controls preserve input, required marker and inline errors in both modes', () => {
  const props = { field: { field_key: 'batch_name', field_type: 'text', label: 'Tên đợt bàn giao', required: true, help_text: 'Ví dụ: Tầng 12.' }, value: 'Tầng 12', formValues: {}, evidence: [], disabled: false, displayOnly: false, error: 'Nhập tên đợt bàn giao.', onChange() {} };
  const old = render('WorkflowFieldControl', props, '/projects/p/planning?wordless=0');
  const next = render('WorkflowFieldControl', props, '/projects/p/planning?wordless=1');
  assert.deepEqual(next.match(/<input[^>]*>/g), old.match(/<input[^>]*>/g));
  for (const markup of [old, next]) {
    assert.match(markup, /Nhập tên đợt bàn giao\./);
    assert.match(markup, /text-rose-600/);
    assert.match(markup, /Tên đợt bàn giao/);
  }
  assert.match(old, /Ví dụ: Tầng 12\./);
  assert.doesNotMatch(next, /Ví dụ: Tầng 12\./);
  assert.match(next, /aria-label="Hướng dẫn: Tên đợt bàn giao"/);
  assert.equal(render('WorkflowFieldControl', props, '/projects/p/planning?wordless=0'), old);
  assert.equal(render('WorkflowFieldControl', props, '/projects/p/operations?wordless=1'), old);
});

test('dialog header removes only duplicate title; standalone workspace retains its heading', () => {
  const props = { item: { id: 'w', title: 'Kích hoạt bàn giao thi công', description: 'PM chốt hạng mục.', status: 'ready', role_ids: ['pm'], run_node_name: 'Kế hoạch thi công', run_step_name: 'Bàn giao', evidence_count: 0 }, evidence: [], titleInDialog: true };
  const old = render('WorkItemWorkspaceHeader', props, '/projects/p/planning?wordless=0');
  const next = render('WorkItemWorkspaceHeader', props, '/projects/p/planning?wordless=1');
  assert.match(old, /<h3[^>]*>Kích hoạt bàn giao thi công<\/h3>/);
  assert.doesNotMatch(next, /<h3/);
  assert.doesNotMatch(next, /aria-label="Mục đích Đầu việc"/);
  assert.match(next, /Sẵn sàng/);
  assert.match(render('WorkItemWorkspaceHeader', { ...props, titleInDialog: false }, '/projects/p/planning?wordless=1'), /<h3/);
});

test('dialog title supports the standard help trigger beside its heading without altering close control', () => {
  const props = { open: true, title: 'Kích hoạt bàn giao thi công', description: 'Kế hoạch thi công', onClose() {}, children: 'Nội dung' };
  const old = render('Dialog', props, '/projects/p/planning?wordless=0');
  const next = render('Dialog', { ...props, titleAside: h(mod.exports.ContextHelp, { tooltip: true, label: 'Mục đích Đầu việc' }, 'PM chốt hạng mục.') }, '/projects/p/planning?wordless=1');
  assert.match(next, /<h2[^>]*>Kích hoạt bàn giao thi <span[^>]*data-label-help-tail[^>]*><span>công<\/span><span[^>]*><button[^>]*aria-label="Mục đích Đầu việc"/);
  const closeButton = (markup: string) => markup.match(/<button[^>]*data-dialog-close[\s\S]*?<\/button>/)?.[0];
  assert.ok(closeButton(old));
  assert.equal(closeButton(next), closeButton(old));
  assert.match(next, /Kế hoạch thi công/);
});

test('long labels leave the prefix wrappable and bind only final word, required marker and help', () => {
  const markup = render('LabelWithHelp', { text: 'Tên đợt bàn giao có nội dung rất dài', required: true, help: h(mod.exports.ContextHelp, { tooltip: true, label: 'Giải thích' }, 'Nội dung') }, '/projects/p/planning?wordless=1');
  assert.match(markup, /^Tên đợt bàn giao có nội dung rất <span class="inline-flex items-center whitespace-nowrap align-baseline" data-label-help-tail="true"><span>dài/);
  assert.match(markup, /dài<span class="ml-1 text-rose-600">\*<\/span><\/span><span[^>]*><button/);
  assert.equal((markup.match(/whitespace-nowrap/g) ?? []).length, 1);
  assert.doesNotMatch(markup, /<div/); // valid phrasing content inside a heading/field label
  const plain = render('WordlessLabel', { text: 'Tên giữ nguyên', label: 'Giải thích', help: 'Nội dung' }, '/projects/p/planning?wordless=0');
  assert.equal(plain, 'Tên giữ nguyên');
});

test('DDL keeps its class, options, selected value, disabled and invalid state across modes', () => {
  const props = { field: { field_key: 'choice', field_type: 'select', label: 'Phương án', required: true, help_text: 'Giải thích phương án.', options_json: [{ value: 'a', label: 'Phương án A' }, { value: 'b', label: 'Phương án B' }] }, value: 'b', formValues: {}, evidence: [], disabled: true, displayOnly: false, error: 'Kiểm tra phương án.', onChange() {} };
  const old = render('WorkflowFieldControl', props, '/projects/p/planning?wordless=0');
  const next = render('WorkflowFieldControl', props, '/projects/p/planning?wordless=1');
  const ddl = (markup: string) => markup.match(/<select[\s\S]*?<\/select>/)?.[0];
  assert.ok(ddl(old));
  assert.equal(ddl(next), ddl(old));
  assert.match(ddl(next)!, /disabled/);
  assert.match(ddl(next)!, /aria-invalid="true"/);
  assert.match(ddl(next)!, /selected=""/);
  assert.match(next, /Kiểm tra phương án\./);
});

test('action center keeps buttons and validation while reducing repeated form heading', () => {
  const props = { title: 'Kích hoạt bàn giao thi công', versionNo: 3, status: 'draft', submitted: false, validationIssueLabels: ['Tên đợt bàn giao'], saving: false, submitting: false, readOnly: false, showActions: true, onSave() {}, onSubmit() {}, saveLabel: 'Bắt đầu & lưu nháp', submitLabel: 'Bắt đầu & chốt hồ sơ' };
  const old = render('WorkflowFormActionCenter', props, '/projects/p/planning?wordless=0');
  const next = render('WorkflowFormActionCenter', props, '/projects/p/planning?wordless=1');
  assert.deepEqual(next.match(/<button[\s\S]*?<\/button>/g), old.match(/<button[\s\S]*?<\/button>/g));
  assert.match(next, /Thiếu: Tên đợt bàn giao/);
  assert.match(next, />Hồ sơ nghiệp vụ<\/h3>/);
});

test('task context reduces explanations only in opted-in scope and preserves actions and source warnings', () => {
  taskQuery = { data: { id: 'w', project_id: 'p', title: 'Kích hoạt bàn giao thi công', status: 'ready', run_node_id: 's', run_node_name: 'Kế hoạch thi công', execution_target: { workspace: 'planning', kind: 'capture', source_ready: true, source_message: 'Hồ sơ chưa chốt.' } } };
  const path = '/projects/p/planning?task=w';
  const old = render('ProjectTaskContext', { projectId: 'p' }, `${path}&wordless=0`);
  const next = render('ProjectTaskContext', { projectId: 'p' }, `${path}&wordless=1`);
  assert.match(old, /Việc đang thực hiện/);
  assert.match(old, /Thao tác nghiệp vụ đầu tiên/);
  assert.doesNotMatch(next, /Việc đang thực hiện|Thao tác nghiệp vụ đầu tiên|Thực hiện tại/);
  assert.match(next, /aria-label="Ngữ cảnh Đầu việc đang thực hiện"/);
  for (const markup of [old, next]) {
    assert.match(markup, /Nguồn đã sẵn sàng/);
    assert.match(markup, /Hồ sơ chưa chốt\./);
    assert.match(markup, /Sẵn sàng/);
    assert.match(markup, /Kiểm tra điều kiện &amp; chốt bước/);
  }
  const buttons = (markup: string) => markup.match(/<button[^>]*(?:aria-label="Đóng ngữ cảnh công việc"|class="inline-flex items-center gap-1 rounded-md bg-blue-600)[\s\S]*?<\/button>/g);
  assert.ok(buttons(old)?.length);
  assert.deepEqual(buttons(next), buttons(old));
  assert.match(next, /pipeline\?wordless=1&amp;stage=s/);
  // The existing pipeline helper preserves the URL's off flag; presentation is identical.
  assert.equal(render('ProjectTaskContext', { projectId: 'p' }, `${path}&wordless=0`), old);
});

test('task context keeps failed reads and cross-project refusal visible in Wordless', () => {
  taskQuery = { error: new Error('unavailable') };
  assert.match(render('ProjectTaskContext', { projectId: 'p' }, '/projects/p/planning?task=w&wordless=1'), /Không tải được ngữ cảnh công việc/);
  taskQuery = { data: { id: 'w', project_id: 'other', title: 'Không được xem' } };
  const mismatch = render('ProjectTaskContext', { projectId: 'p' }, '/projects/p/planning?task=w&wordless=1');
  assert.match(mismatch, /Công việc không thuộc công trình đang mở/);
  assert.doesNotMatch(mismatch, /Không được xem|Ngữ cảnh Đầu việc đang thực hiện/);
});

test('banner scope does not enable full-page or unrelated banner presentation', () => {
  const props = { field: { field_key: 'name', field_type: 'text', label: 'Tên', help_text: 'Hướng dẫn vẫn hiển thị.' }, value: '', formValues: {}, evidence: [], disabled: false, displayOnly: false, onChange() {} };
  for (const surface of ['discussion', 'acceptance', 'operations']) {
    assert.equal(render('WorkflowFieldControl', props, `/projects/p/${surface}?wordless=1`), render('WorkflowFieldControl', props, `/projects/p/${surface}`));
  }
  assert.equal(render('BannerLabel', { surface: 'acceptance', text: 'Nguồn', help: 'Hướng dẫn' }, '/projects/p/operations?wordless=1'), 'Nguồn');
});

test('source report banners retain exact identity and distinct evidence semantics', () => {
  const report: any = { id: 'r', report_date: '2026-11-09', weather: 'sunny', evidence: [{ id: 'e1' }, { id: 'e2' }] };
  for (const variant of ['check', 'action']) {
    const old = render('SourceReportContextBanner', { report, variant }, '/projects/p/operations?wordless=0');
    const next = render('SourceReportContextBanner', { report, variant }, '/projects/p/operations?wordless=1');
    assert.match(old, /Nguồn đã liên kết/);
    assert.doesNotMatch(next, /Nguồn đã liên kết/);
    assert.match(next, /aria-label="Giải thích: Báo cáo ngày/);
    assert.match(next, variant === 'check' ? /Dùng lại 2 minh chứng hiện trạng/ : /Có 2 minh chứng có thể tái sử dụng/);
    assert.equal(render('SourceReportContextBanner', { report, variant }, '/projects/p/operations?wordless=0'), old);
    assert.equal(render('SourceReportContextBanner', { report, variant }, '/projects/p/operations?wordless=1&wordless=1'), old);
    assert.equal(render('SourceReportContextBanner', { report, variant }, '/operations?wordless=1'), old);
    assert.doesNotMatch(render('SourceReportContextBanner', { report: { ...report, evidence: [] }, variant }, '/projects/p/operations?wordless=1'), /0 minh chứng/);
  }
});

test('discussion banner keeps owner, due date, blockers and exact buttons while moving stage into help', () => {
  const item: any = { id: 'w', project_id: 'p', title: 'Kiểm tra bàn giao', status: 'ready', run_node_name: 'Giai đoạn riêng', assigned_user_name: 'Người phụ trách', blocked_reason: 'Cần bổ sung hồ sơ.' };
  taskQuery = { isFetchedAfterMount: true, data: item };
  const props = { projectId: 'p', workflow: { hasPipeline: true, primaryWork: item }, onOpenWork() {}, onDiscuss() {} };
  const old = render('DiscussionWorkflowActionCard', props, '/projects/p/discussion?wordless=0');
  const next = render('DiscussionWorkflowActionCard', props, '/projects/p/discussion?wordless=1');
  assert.match(old, /Giai đoạn riêng/);
  assert.doesNotMatch(next, /Giai đoạn riêng/);
  assert.match(next, /aria-label="Giải thích: Kiểm tra bàn giao"/);
  assert.match(next, /Cần bổ sung hồ sơ\./);
  assert.match(next, /Phụ trách:|Hạn:/);
  const buttons = (s: string) => s.match(/<button[\s\S]*?<\/button>/g)?.filter(b => !b.includes('aria-label="Giải thích:'));
  assert.deepEqual(buttons(next), buttons(old));
  taskQuery = { isError: true };
  assert.match(render('DiscussionWorkflowActionCard', props, '/projects/p/discussion?wordless=1'), /Không tải được bước tiếp theo/);
});

test('acceptance source banner retains source identity, deleted-assignee warning and change control', () => {
  taskQuery = {};
  const draft: any = { id: '', work_item_id: 'w', type: 'partial', title: '', acceptance_date: '2026-11-09', location: '', client_representative: '', contractor_representative: '', scope_summary: '', form_instance_id: '', pdf_export_id: '', signature_status: '', document_status: '', notes: '', checklist: [] };
  const props = { draft, projectId: 'p', evidence: [], workItems: [{ id: 'w', title: 'Đầu việc nguồn chính xác', status: 'ready', run_node_name: 'Giai đoạn nguồn', assignee_name: 'Người cũ', assignee_account_deleted: true }], onChange() {} };
  const renderEditor = (path: string, extra = {}) => renderToStaticMarkup(h(MemoryRouter, { initialEntries: [path] }, h(mod.exports.WordlessScope, null, h(mod.exports.WordlessBannerScope, null, h(mod.exports.ToastProvider, null, h(mod.exports.AcceptanceEditor, { ...props, ...extra }))))));
  const old = renderEditor('/projects/p/acceptance?wordless=0');
  const next = renderEditor('/projects/p/acceptance?wordless=1');
  assert.match(old, /Giai đoạn nguồn/);
  assert.doesNotMatch(next, /Giai đoạn nguồn/);
  assert.match(next, /aria-label="Giải thích: Đầu việc nguồn chính xác"/);
  assert.match(next, /Cần phân công lại/);
  assert.match(next, /Nguồn công việc/);
  const change = (s: string) => s.match(/<button[^>]*>[\s\S]*?Thay đổi<\/button>/)?.[0];
  assert.ok(change(old));
  // Compare the actual action button, excluding preceding help markup.
  const lastButton = (s: string) => s.match(/<button[^>]*>(?:(?!<button)[\s\S])*?Thay đổi<\/button>/)?.[0];
  assert.equal(lastButton(next), lastButton(old));
  assert.doesNotMatch(renderEditor('/projects/p/acceptance?wordless=1', { fixedWorkItem: true }), /> Thay đổi<\/button>/);
  assert.equal(renderEditor('/projects/p/acceptance?wordless=0'), old);
});

test('settings Wordless keeps actual DDL/input controls and inline errors, restart status and secret warning', () => {
  const fixtures = [
    { value_type: 'enum', value: 'b', options: [{ value: 'a', label: 'A' }, { value: 'b', label: 'B' }] },
    { value_type: 'integer', value: 15, min: 1, max: 60, unit: 'phút' },
    { value_type: 'boolean', value: true },
    { value_type: 'secret', value: '', configured: true },
  ];
  for (const fixture of fixtures) {
    const setting = { key: 'TEST', label: 'Cấu hình kiểm tra', description: 'Mô tả cấu hình dài.', version: 1, restart_pending: true, ...fixture };
    const props = { setting, value: fixture.value, clearSecret: fixture.value_type === 'secret', disabled: false, error: 'Giá trị chưa hợp lệ.', onChange() {}, onClearSecret() {} };
    const old = render('SettingField', props, '/settings?wordless=0');
    const next = render('SettingField', props, '/settings?wordless=1');
    assert.match(old, /Mô tả cấu hình dài\./);
    assert.doesNotMatch(next, /Mô tả cấu hình dài\./);
    assert.match(next, /aria-label="Giải thích: Cấu hình kiểm tra"/);
    const inputs = (s: string) => s.match(/<input[^>]*>|<select[\s\S]*?<\/select>/g);
    assert.deepEqual(inputs(next), inputs(old));
    assert.match(next, /Chờ restart/);
    assert.match(next, /Giá trị chưa hợp lệ\./);
    if (fixture.value_type === 'secret') assert.match(next, /Secret sẽ bị xóa khi lưu\./);
    assert.equal(render('SettingField', props, '/settings?wordless=0'), old);
    assert.equal(render('SettingField', props, '/settings?wordless=1&wordless=1'), old);
  }
});

test('statistics keeps one standard trigger in both modes and binds Wordless help to the selected report title', () => {
  for (const report of mod.exports.reportDefinitions) {
    const path = `/statistics/${report.key}`;
    const old = render('StatisticsReportHeading', { report }, `${path}?wordless=0`);
    const next = render('StatisticsReportHeading', { report }, `${path}?wordless=1`);
    assert.equal((old.match(/<button/g) ?? []).length, 1);
    assert.equal((next.match(/<button/g) ?? []).length, 1);
    assert.doesNotMatch(old, /data-label-help-tail/);
    assert.match(next, /data-label-help-tail/);
    assert.equal(next.match(/<button[^>]*class="([^"]*)"/)?.[1], old.match(/<button[^>]*class="([^"]*)"/)?.[1]);
    assert.equal(render('StatisticsReportHeading', { report }, `${path}?wordless=0`), old);
  }
});

test('backup controls and read-only permissions stay visible when explanation is collapsed', () => {
  const query = require('@tanstack/react-query');
  const renderBackup = (path: string) => renderToStaticMarkup(h(MemoryRouter, { initialEntries: [path] }, h(query.QueryClientProvider, { client: new query.QueryClient() }, h(mod.exports.ToastProvider, null, h(mod.exports.WordlessScope, null, h(mod.exports.SettingsBackupPanel, { canRestore: false }))))));
  const old = renderBackup('/settings?wordless=0');
  const next = renderBackup('/settings?wordless=1');
  assert.match(old, /Cấu hình lõi .env, con dấu và giá trị secret/);
  assert.doesNotMatch(next, /Cấu hình lõi .env, con dấu và giá trị secret/);
  assert.match(next, /Bản sao thiết lập database\/UI/);
  assert.match(next, /chỉ vai trò Nhà phát triển được khôi phục/);
  assert.match(next, /Tải bản sao JSON/);
  assert.doesNotMatch(next, />Khôi phục từ JSON/);
});

test('settings panel retains read-only/restart warning and puts core help outside the toggle button', () => {
  const query = require('@tanstack/react-query');
  const props = { data: { capabilities: { can_update: false }, categories: [], settings: [], core_settings: [], restart_required_count: 1 }, loading: false, error: null, onRetry() {} };
  const renderPanel = (path: string) => renderToStaticMarkup(h(MemoryRouter, { initialEntries: [path] }, h(query.QueryClientProvider, { client: new query.QueryClient() }, h(mod.exports.ToastProvider, null, h(mod.exports.WordlessScope, null, h(mod.exports.SystemSettingsPanel, props))))));
  const old = renderPanel('/settings?wordless=0');
  const next = renderPanel('/settings?wordless=1');
  assert.match(next, /Chế độ chỉ xem/);
  assert.match(next, /1 thiết lập đang chờ khởi động lại backend/);
  assert.match(next, /aria-label="Về cấu hình lõi"/);
  assert.match(next, /Xem danh sách/);
  assert.doesNotMatch(next, /<button(?:(?!<\/button>)[\s\S])*?<button/);
  assert.match(old, /Các field bootstrap, kết nối và root secret chỉ đọc/);
  assert.doesNotMatch(next, /Các field bootstrap, kết nối và root secret chỉ đọc/);
  assert.equal(renderPanel('/settings?wordless=0'), old);
});

test('approved scopes use Wordless by default and keep an explicit legacy escape hatch', () => {
  const props = { text: 'Nhãn trợ giúp', label: 'Giải thích', help: 'Nội dung giải thích' };
  for (const path of ['/settings', '/statistics/projects', '/projects/p/planning', '/projects/p/preparation', '/projects/p/pipeline']) {
    assert.equal(render('WordlessLabel', props, path), render('WordlessLabel', props, `${path}?wordless=1`));
    assert.match(render('WordlessLabel', props, path), /aria-label="Giải thích"/);
    assert.equal(render('WordlessLabel', props, `${path}?wordless=0`), 'Nhãn trợ giúp');
  }
  const banner = { surface: 'operations', text: 'Báo cáo nguồn', help: 'Nội dung nguồn' };
  assert.match(render('BannerLabel', banner, '/projects/p/operations'), /Giải thích: Báo cáo nguồn/);
  assert.equal(render('BannerLabel', banner, '/projects/p/operations?wordless=0'), 'Báo cáo nguồn');
  assert.equal(render('WordlessLabel', props, '/dashboard'), 'Nhãn trợ giúp');
});
