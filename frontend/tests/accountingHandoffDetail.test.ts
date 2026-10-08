import assert from 'node:assert/strict';
import test from 'node:test';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';
import { readFile } from 'node:fs/promises';
import { build } from 'esbuild';
import { createElement } from 'react';
import { renderToStaticMarkup } from 'react-dom/server';

const require = createRequire(import.meta.url);
const react = require('react');
const { QueryClient, QueryClientProvider } = require('@tanstack/react-query');
const code = await build({
  stdin: { resolveDir: fileURLToPath(new URL('../src/', import.meta.url)), contents: `
    export * from './modules/project-finance/AccountingHandoffDetailDialog';
    export { AccountingHandoffPage } from './modules/project-finance/AccountingHandoffPage';
    export { AccountingWorkDetails } from './modules/my-work/AccountingWorkCard';
    export { SourceCell } from './modules/my-work/AccountingWorkLane';` },
  bundle: true, write: false, platform: 'node', format: 'cjs', packages: 'external', jsx: 'automatic',
  loader: { '.css': 'empty' }, define: { 'import.meta.env': '{}', 'import.meta.url': '"file:///fixture"' },
  plugins: [{ name: 'test-context', setup(builder) {
    builder.onLoad({ filter: /AuthContext\.tsx$/ }, () => ({ contents: 'export function useAuth() { return { user: {id:"actor"}, hasPermission: () => true, hasAnyPermission: () => true }; }', loader: 'js' }));
    builder.onLoad({ filter: /ToastProvider\.tsx$/ }, () => ({ contents: 'export function useToast() { return { success() {}, warn() {}, error() {} }; }', loader: 'js' }));
    builder.onLoad({ filter: /AccountingWorkLane\.tsx$/ }, async ({ path }) => ({ contents: await readFile(path, 'utf8') + '\nexport { SourceCell };', loader: 'tsx' }));
  } }],
});
function load(customRequire = require) {
  const module = { exports: {} as Record<string, any> };
  new Function('require', 'module', 'exports', code.outputFiles[0].text)(customRequire, module, module.exports);
  return module.exports;
}
const components = load();
const project = '10000000-0000-4000-8000-000000000001';
const source = '10000000-0000-4000-8000-000000000002';
const stock = '10000000-0000-4000-8000-000000000003';
const data = {
  project: { id: project, code: 'CT-001', name: 'Công trình thật' }, project_id: project,
  source_type: 'procurement_request', source_id: source,
  current: { source_code: 'PO-1', source_title: 'Nguồn hiện tại', counterparty_name: 'Đối tác', partner_id: null, reference_amount: 150, currency: 'USD', source_status: 'ordered' },
  handoff: { id: 'handoff', status: 'handed_off', source_snapshot_json: { source_title: 'Bản chụp cũ', reference_amount: 100, currency: 'USD' } },
  related: [{ kind: 'stock_document', id: stock, label: 'PNK-1', status: 'posted' }],
  files: [], restricted: false, source_access: true, status: 'handed_off',
  capabilities: { handoff: false, acknowledge: true, return: true },
};
const props = { data, loading: false, error: false, tab: 'current', onTab() {}, onRetry() {}, onAction() {} };
function render(overrides: object = {}) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  try { return renderToStaticMarkup(createElement(QueryClientProvider, { client }, createElement(components.AccountingHandoffDetailContent, { ...props, ...overrides }))); }
  finally { client.clear(); }
}

test('current records link to exact project/record and distinguish unlinked partner and currency', () => {
  const html = render();
  assert.match(html, /Chưa liên kết danh mục/);
  assert.match(html, new RegExp(`/projects/${project}/warehouse\\?record=${stock}`));
  assert.match(html, /record_type=stock_document/);
  assert.match(html, /150,00/);
  assert.doesNotMatch(html, /Bản chụp cũ/);
  assert.match(html, /Xác nhận tiếp nhận hồ sơ/);
});

test('snapshot tab never substitutes current fields or attachments for historical values', () => {
  const html = render({ tab: 'snapshot' });
  assert.match(html, /Bản chụp cũ/);
  assert.match(html, /100,00/);
  assert.doesNotMatch(html, /150,00|PNK-1|Chứng từ có liên kết/);
  assert.match(render({ tab: 'snapshot', data: { ...data, handoff: null } }), /Hồ sơ chưa được bàn giao/);
});

test('source-only access shows current source even on stale snapshot selection and hides stored handoff and actions', () => {
  const html = render({ tab: 'snapshot', data: { ...data, handoff_access: false } });
  assert.match(html, /Nguồn hiện tại|150,00|PNK-1/);
  assert.doesNotMatch(html, /Bản chụp cũ|Thông tin đã bàn giao|Hồ sơ chưa được bàn giao|Thông tin bàn giao|Xác nhận tiếp nhận hồ sơ|Trả để bổ sung/);
});

test('loading/error/restricted states do not expose stale records or actions', () => {
  assert.match(render({ loading: true }), /Đang tải hồ sơ/);
  assert.doesNotMatch(render({ loading: true }), /PO-1|Xác nhận tiếp nhận/);
  assert.match(render({ error: true }), /Thử lại/);
  assert.doesNotMatch(render({ error: true }), /PO-1|Xác nhận tiếp nhận/);
  const html = render({ data: { ...data, current: null, related: [], files: [], restricted: true, source_access: false, capabilities: {} } });
  assert.match(html, /Không thể mở hồ sơ nguồn theo quyền hiện tại/);
  assert.doesNotMatch(html, /Xác nhận tiếp nhận hồ sơ|PNK-1/);
});

// Shallow hook harness exercises actual click callbacks and selected identities.
// It intentionally makes no browser/DOM acceptance claim.
function harness(queryData: any = data) {
  const states: any[] = []; let cursor = 0;
  const module = load((name: string) => {
    if (name === 'react') return { ...react, useState(initial: any) { const i = cursor++; if (!(i in states)) states[i] = typeof initial === 'function' ? initial() : initial; return [states[i], (value: any) => { states[i] = typeof value === 'function' ? value(states[i]) : value; }]; }, useMemo: (fn: any) => fn(), useEffect() {} };
    if (name === '@tanstack/react-query') return { ...require(name), useQuery: () => ({ data: queryData }), useQueryClient: () => ({}), useMutation: () => ({}) };
    if (name === 'react-router-dom') return { ...require(name), useParams: () => ({ id: project }), useSearchParams: () => [new URLSearchParams(), () => {}] };
    return require(name);
  });
  return { render(name: string, value: any) { cursor = 0; return module[name](value); } };
}
function find(node: any, predicate: (node: any) => boolean): any {
  if (!node || typeof node !== 'object') return undefined;
  if (predicate(node)) return node;
  for (const child of react.Children.toArray(node.props?.children)) { const found = find(child, predicate); if (found) return found; }
}

test('every accounting entrypoint opens the shared dialog for the selected source identity', () => {
  const item = { ...data, ...data.current, handoff_id: 'handoff', project_id: project, source_type: 'procurement_request', source_id: source };
  for (const name of ['SourceCell', 'AccountingWorkDetails']) {
    const fixture = harness(); const args = { item, canAcknowledge: true, onAction() {} };
    find(fixture.render(name, args), (node) => node.type === 'button').props.onClick();
    const dialog = find(fixture.render(name, args), (node) => node.type?.name === 'AccountingHandoffDetailDialog');
    assert.equal(dialog.props.target.source_id, source, name);
    assert.equal(dialog.props.target.project_id, project, name);
  }
  const fixture = harness({ items: [item], summary: {} });
  const table = find(fixture.render('AccountingHandoffPage', {}), (node) => node.type?.name === 'DataTable');
  table.props.columns.find((column: any) => column.key === 'source').render(item).props.onClick();
  const dialog = find(fixture.render('AccountingHandoffPage', {}), (node) => node.type?.name === 'AccountingHandoffDetailDialog');
  assert.equal(dialog.props.target.source_id, source);
  assert.equal(dialog.props.target.project_id, project);
});

test('file actions preserve evidence IDs, document IDs and exact workflow PDF exports', () => {
  for (const [name, label] of [['ảnh.png', 'Xem ảnh'], ['file.pdf', 'Xem PDF'], ['file.docx', 'Xem Word'], ['file.xlsx', 'Xem Excel']]) {
    const fixture = harness(); const file = { kind: 'evidence', id: 'exact-evidence', name };
    const args = { ...props, data: { ...data, handoff_access: false, files: [file] } };
    const button = find(fixture.render('AccountingHandoffDetailContent', args), (node) => node.type === 'button' && node.props.children === label);
    assert.ok(button, label); button.props.onClick();
    const viewer = find(fixture.render('AccountingHandoffDetailContent', args), (node) => node.type?.name === 'EvidenceReferenceViewer');
    assert.equal(viewer.props.reference.id, 'exact-evidence');
    assert.equal(viewer.props.reference.record, undefined, 'do not fabricate evidence metadata');
  }
  const fixture = harness();
  const args = { ...props, data: { ...data, handoff_access: false, files: [{ kind: 'workflow_pdf', id: 'export-revision-2', name: 'Nghiệm thu.pdf' }, { kind: 'document', id: 'document-id', name: 'Hợp đồng.pdf' }] } };
  const tree = fixture.render('AccountingHandoffDetailContent', args);
  assert.equal(find(tree, (node) => node.type?.name === 'DocumentReferenceViewButton').props.file.id, 'document-id');
  find(tree, (node) => node.type === 'button' && node.props.children === 'Xem PDF').props.onClick();
  const pdf = find(fixture.render('AccountingHandoffDetailContent', args), (node) => node.type?.name === 'ProtectedDocumentViewerDialog');
  assert.equal(pdf.props.protectedFilePath, '/workflow-form-pdf-exports/export-revision-2/file');
});

test('action and retry callbacks use the displayed operation', () => {
  let action = ''; let retried = false;
  const fixture = harness();
  const tree = fixture.render('AccountingHandoffDetailContent', { ...props, onAction(value: string) { action = value; } });
  find(tree, (node) => node.type === 'button' && node.props.children === 'Trả để bổ sung').props.onClick();
  assert.equal(action, 'return');
  const error = fixture.render('AccountingHandoffDetailContent', { ...props, error: true, onRetry() { retried = true; } });
  find(error, (node) => node.type === 'button').props.onClick(); assert.equal(retried, true);
});
