import assert from 'node:assert/strict';
import test from 'node:test';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';
import { build } from 'esbuild';
const require = createRequire(import.meta.url);
const react = require('react');
const bundle = await build({ stdin: { resolveDir: fileURLToPath(new URL('../src/', import.meta.url)), contents: `export { ConstructionActionList, compareConstructionActions } from './modules/operations/ConstructionWorkspaceLists'; export { NativeExcelExportButton } from './modules/_shared/NativeExcelExportButton';` }, bundle: true, write: false, platform: 'node', format: 'cjs', packages: 'external', jsx: 'automatic', define: { 'import.meta.env': '{}' }, plugins: [{ name: 'controlled-access-transport', setup(builder) {
  builder.onLoad({ filter: /\/lib\/api\/client\.ts$/ }, () => ({ contents: 'export const downloadBlob=(path)=>globalThis.__actionDownload(path);', loader: 'js' }));
  builder.onLoad({ filter: /AuthContext\.tsx$/ }, () => ({ contents: 'export const useAuth=()=>({hasPermission:(permission)=>permission==="operation.view"&&globalThis.__actionAllowed});', loader: 'js' }));
  builder.onLoad({ filter: /ToastProvider\.tsx$/ }, () => ({ contents: 'export const useToast=()=>globalThis.__actionToast;', loader: 'js' }));
} }] });
function fixture(name: string) {
  const state: any[] = []; let cursor = 0;
  const hooks = { ...react, useMemo: (fn: any) => fn(), useState(initial: any) { const index = cursor++; if (!(index in state)) state[index] = typeof initial === 'function' ? initial() : initial; return [state[index], (next: any) => { state[index] = typeof next === 'function' ? next(state[index]) : next; }]; } };
  const module = { exports: {} as any };
  new Function('require', 'module', 'exports', bundle.outputFiles[0].text)((id: string) => id === 'react' ? hooks : require(id), module, module.exports);
  return (props: any) => { cursor = 0; return module.exports[name](props); };
}
function nodes(tree: any, matches: any[] = []): any[] { if (!tree || typeof tree !== 'object') return matches; matches.push(tree); for (const child of [tree.props?.children].flat(Infinity)) nodes(child, matches); return matches; }
function component(tree: any, name: string, label?: string) { const node = nodes(tree).find(node => node.type?.name === name && (!label || node.props.ariaLabel === label)); assert.ok(node, `${name} ${label ?? ''}`); return node; }
const globals = globalThis as any; globals.__actionAllowed = true;
const props = { projectId: 'project', workspace: { participants: [{ id: 'actor', full_name: 'Actor' }], actions: ['open', 'waiting_confirmation', 'done'].flatMap(status => Array.from({ length: 25 }, (_, index) => ({ id: `${status}-${index}`, status, title: 'BÊ TÔNG', description: 'Column', assigned_user_id: 'actor', assigned_user_name: 'Actor', priority: 'urgent', impact_scope: 'budget' }))) }, canCreate: true, canEdit: true, canResolve: true, canVerify: true, onCreate() {}, onView() {}, onEdit() {}, onResolve() {}, onVerify() {} };

test('native action export uses every active exact filter and all matching records, with canonical queue/reset values', () => {
  const render = fixture('ConstructionActionList');
  let tree = render(props);
  component(tree, 'SearchInput').props.onChange('  BÊ TÔNG  ');
  component(tree, 'FilterSelect', 'Lọc người xử lý').props.onChange('actor');
  component(tree, 'FilterSelect', 'Lọc mức ưu tiên').props.onChange('urgent');
  component(tree, 'FilterSelect', 'Lọc ảnh hưởng').props.onChange('budget');
  for (const queue of ['open', 'waiting_confirmation', 'done', 'all']) {
    component(render(props), 'QueueTabs').props.onChange(queue);
    tree = render(props); const action = component(tree, 'NativeExcelExportButton');
    const url = new URL(action.props.path, 'https://test.invalid');
    assert.equal(url.pathname, '/projects/project/construction-actions/export.xlsx');
    assert.deepEqual(Object.fromEntries(url.searchParams), { queue, search: 'bê tông', assignee_id: 'actor', priority: 'urgent', impact_scope: 'budget' });
    assert.equal(action.props.disabled, false); assert.equal(action.props.label, 'Xuất báo cáo');
    assert.ok(!/limit|offset|page|ids|template/.test(action.props.path));
  }
  component(tree, 'FilterToolbar').props.onReset();
  assert.equal(component(render(props), 'NativeExcelExportButton').props.path, '/projects/project/construction-actions/export.xlsx?queue=open');
});

test('native action export hides without operation view; unmatched filters and empty scope disable export', () => {
  const render = fixture('ConstructionActionList');
  component(render(props), 'SearchInput').props.onChange('unmatched');
  assert.equal(component(render(props), 'NativeExcelExportButton').props.disabled, true);
  assert.equal(component(fixture('ConstructionActionList')({ ...props, workspace: { ...props.workspace, actions: [] } }), 'NativeExcelExportButton').props.disabled, true);
  globals.__actionAllowed = false;
  assert.equal(nodes(render(props)).some(node => node.type?.name === 'NativeExcelExportButton'), false);
  globals.__actionAllowed = true;
});

test('action ordering keeps status and priority ahead of missing due dates, with stable time and ID ties', () => {
  // The comparator has two arguments, so evaluate it directly from the bundle.
  const module = { exports: {} as any };
  new Function('require', 'module', 'exports', bundle.outputFiles[0].text)(require, module, module.exports);
  const make = (id: string, values: any = {}) => ({ id, status: 'open', priority: 'urgent', created_at: '2026-10-08T00:00:00Z', ...values });
  const rows = [make('done', {status: 'done', due_at: '2026-10-01'}), make('waiting', {status: 'waiting_confirmation'}), make('open-normal', {priority: 'normal',due_at:'2026-10-01'}), make('open-no-due'), make('open-with-due', {due_at:'2026-10-08'})];
  assert.deepEqual(rows.sort(module.exports.compareConstructionActions).map(row=>row.id), ['open-with-due','open-no-due','open-normal','waiting','done']);
  const ties = [make('b'), make('a'), make('newer', {created_at:'2026-10-09T00:00:00Z'})];
  assert.deepEqual(ties.sort(module.exports.compareConstructionActions).map(row=>row.id), ['newer','a','b']);
  assert.equal(module.exports.compareConstructionActions(make('same'),make('same')),0);
  const completed = [make('done-a',{status:'done',priority:'normal',due_at:'2026-10-01'}),make('done-b',{status:'done',priority:'urgent',due_at:'2026-10-02'})];
  assert.deepEqual(completed.sort(module.exports.compareConstructionActions).map(row=>row.id),['done-a','done-b']);
});

test('actual shared export button sends exact filtered GET, blocks repeated pending clicks, surfaces error and retries', async () => {
  const render = fixture('NativeExcelExportButton'); const paths: string[] = []; const errors: string[] = []; let reject: any;
  globals.__actionToast = { success() {}, error(message: string) { errors.push(message); } };
  globals.__actionDownload = (path: string) => { paths.push(path); return new Promise((_resolve, no) => { reject = no; }); };
  const exportProps = { path: '/projects/project/construction-actions/export.xlsx?queue=done&impact_scope=budget', filename: 'BM04.xlsx', label: 'Xuất báo cáo' };
  const request = render(exportProps).props.onClick();
  const pending = render(exportProps); assert.equal(pending.props.disabled, true); await pending.props.onClick(); assert.equal(paths.length, 1);
  reject(new Error('Limit exceeded')); await request;
  assert.deepEqual(errors, ['Không xuất được biểu mẫu Excel']); assert.equal(render(exportProps).props.disabled, false);
  const retry = render(exportProps).props.onClick(); reject(new Error('Still unavailable')); await retry;
  assert.deepEqual(paths, [exportProps.path, exportProps.path]);
});
