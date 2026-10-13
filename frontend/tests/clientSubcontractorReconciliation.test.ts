import assert from 'node:assert/strict';
import test from 'node:test';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';
import { build } from 'esbuild';

const require = createRequire(import.meta.url);
const react = require('react');
const bundle = await build({ stdin: { resolveDir: fileURLToPath(new URL('../src/', import.meta.url)), contents: `export {ApiError} from './lib/api/client'; export {WorkItemPrimarySurface} from './modules/work-items/WorkItemPrimarySurface'; export {SubcontractorReconciliationPanel} from './modules/work-items/SubcontractorReconciliationPanel';` }, bundle: true, write: false, platform: 'node', format: 'cjs', packages: 'external', jsx: 'automatic', loader: { '.css': 'empty' }, define: { 'import.meta.env': '{}', 'import.meta.url': '"file:///fixture"' }, plugins: [{ name: 'controlled-boundaries', setup(builder) {
  builder.onLoad({ filter: /\/lib\/api\/client\.ts$/ }, () => ({ contents: `export class ApiError extends Error { constructor(message,code,status){super(message);this.code=code;this.status=status;} }; export const api=(path,init)=>globalThis.__reconciliationRequest(path,init); export const get=(path)=>api(path); export const post=api; export const put=api; export const del=api; export const postForm=api; export const postFormWithProgress=api; export const downloadBlob=api; export const postDownloadBlob=api; export function setToken(){}; export function setAuthFailureHandler(){}; export function logoutRequest(){};`, loader: 'js' }));
  builder.onLoad({ filter: /AuthContext\.tsx$/ }, () => ({ contents: `export const useAuth=()=>({user:{id:'actor'},hasPermission:()=>false});`, loader: 'js' }));
  builder.onLoad({ filter: /ToastProvider\.tsx$/ }, () => ({ contents: `export const useToast=()=>({success(){},error(){},warn(){}});`, loader: 'js' }));
} }] });

function nodes(tree: any, result: any[] = []): any[] { if (!tree || typeof tree !== 'object') return result; result.push(tree); for (const child of [tree.props?.children].flat(Infinity)) nodes(child, result); return result; }
function text(tree: any): string { if (tree === null || tree === undefined || typeof tree === 'boolean') return ''; if (typeof tree !== 'object') return String(tree); return [tree.props?.children].flat(Infinity).map(text).join(' '); }
function named(tree: any, name: string) { return nodes(tree).filter(node => node.type?.name === name); }
function button(tree: any, label: string) { return nodes(tree).find(node => node.type === 'button' && text(node).trim() === label); }
function fixture(initial: any) {
  let data = initial; let status: any = {}; let cursor = 0; const state: any[] = []; const effects: any[] = []; const deps: any[] = []; let mutations: any[] = []; let hash = ''; let panelTarget: any;
  const queries: any[] = []; const invalidated: any[] = [];
  const hooks = { ...react,
    useState(value: any) { const at = cursor++; if (!(at in state)) state[at] = typeof value === 'function' ? value() : value; return [state[at], (next: any) => { state[at] = typeof next === 'function' ? next(state[at]) : next; }]; },
    useRef(value: any) { return hooks.useState(() => ({ current: value }))[0]; },
    useEffect(fn: any, next: any[]) { const at = cursor++; if (!deps[at] || next.some((value, index) => value !== deps[at][index])) { deps[at] = next; effects.push(fn); } },
    useMemo(fn: any) { return fn(); }, useCallback(fn: any) { return fn; },
  };
  const module = { exports: {} as any };
  new Function('require', 'module', 'exports', bundle.outputFiles[0].text)((id: string) => id === 'react' ? hooks : id === 'react-router-dom' ? { ...require(id), useLocation: () => ({ hash }) } : id === '@tanstack/react-query' ? {
    useQuery(config: any) { queries.push(config); return { data, isLoading: false, isFetching: false, ...status, refetch: async () => ({ data, error: status.error }) }; },
    useQueryClient: () => ({ invalidateQueries: (key: any) => invalidated.push(key), setQueryData() {} }),
    useMutation(config: any) { mutations.push(config); return { isPending: false, mutate: async () => { try { const result = await config.mutationFn(); config.onSuccess?.(result); } catch (error) { config.onError?.(error); } } }; },
  } : require(id), module, module.exports);
  return { queries, invalidated, exports: module.exports, setData(next: any) { data = next; }, setStatus(next: any) { status = next; }, setHash(next: string) { hash = next; }, setPanelTarget(next: any) { panelTarget = next; }, render(nameOrFunction: any, props: any) { cursor = 0; mutations = []; const tree = (typeof nameOrFunction === 'string' ? module.exports[nameOrFunction] : nameOrFunction)(props); if (panelTarget && tree.ref) tree.ref.current = panelTarget; while (effects.length) effects.shift()(); return tree; }, mutations: () => mutations };
}
const item = { id: 'work', project_id: 'project', title: 'Checklist', status: 'in_progress', work_type: 'checklist', workflow_form_template_code: 'client_subcontractor_acceptance_package', workflow_form_instance_id: 'form', workflow_form_type: 'checklist' };
const response = () => ({ available: true, precondition: '"baseline"', capabilities: { can_write: true }, reconciliation: { id: 'ledger', form_instance_id: 'form', form_revision_no: 2, status: 'draft', comment: 'Saved', items: [{ item_index: 0, checklist_title: 'Source row', claimed_quantity: 5, confirmed_quantity: 4, decision: 'accepted' }] }, history: [{ id: 'old-ledger', form_instance_id: 'form', form_revision_no: 1, status: 'submitted' }] });
const props = { workItemId: 'work', projectId: 'project', clientContext: true };

test('exact client context composes canonical form first and existing ledger second; unrelated builtin remains form only', () => {
  const f = fixture(response());
  const primary = f.render('WorkItemPrimarySurface', { item, evidence: [] });
  const wrapper = named(primary, 'ClientSubcontractorFormSurface')[0]; assert.ok(wrapper);
  const tree = f.render(wrapper.type, wrapper.props);
  assert.deepEqual(nodes(tree).filter(node => ['WorkflowFormPanel', 'SubcontractorReconciliationPanel'].includes(node.type?.name)).map(node => node.type.name), ['WorkflowFormPanel', 'SubcontractorReconciliationPanel']);
  const builtin = f.render('WorkItemPrimarySurface', { item: { ...item, workflow_form_template_code: 'builtin_subcontractor_checklist' }, evidence: [] });
  assert.equal(named(builtin, 'ClientSubcontractorFormSurface').length, 0); assert.equal(named(builtin, 'WorkflowFormPanel').length, 1);
});

test('client unavailable and request errors have visible retry; loading keeps a named panel', () => {
  const f = fixture({ available: false, reason: 'Chốt checklist trước khi đối soát.' });
  let tree = f.render('SubcontractorReconciliationPanel', props); assert.match(text(tree), /Đối soát thầu phụ/); assert.ok(button(tree, 'Tải lại'));
  f.setStatus({ error: new Error('Unavailable') }); tree = f.render('SubcontractorReconciliationPanel', props); assert.match(text(tree), /Không tải được dữ liệu đối soát/); assert.ok(button(tree, 'Tải lại'));
  f.setStatus({ isLoading: true }); tree = f.render('SubcontractorReconciliationPanel', props); assert.equal(named(tree, 'LoadingSpinner').length, 1);
});

test('client composition aggregates form and ledger dirty/busy flags and refreshes ledger after a form revision change', () => {
  const f = fixture(response()); const dirty: boolean[] = []; const busy: boolean[] = [];
  const primary = f.render('WorkItemPrimarySurface', { item, evidence: [], observerReadOnly: true, onDirtyChange: (value: boolean) => dirty.push(value), onBusyChange: (value: boolean) => busy.push(value) });
  const wrapper = named(primary, 'ClientSubcontractorFormSurface')[0]; let tree = f.render(wrapper.type, wrapper.props);
  let form = named(tree, 'WorkflowFormPanel')[0]; let ledger = named(tree, 'SubcontractorReconciliationPanel')[0];
  assert.equal(form.props.artifactReadOnly, true); assert.equal(form.props.externalActions, true); assert.equal(ledger.props.readOnly, true);
  ledger.props.onDirtyChange(true); form.props.onDirtyChange(false); assert.equal(dirty.at(-1), true);
  ledger.props.onBusyChange(true); form.props.onBusyChange(false); assert.equal(busy.at(-1), true);
  tree = f.render(wrapper.type, wrapper.props); form = named(tree, 'WorkflowFormPanel')[0]; ledger = named(tree, 'SubcontractorReconciliationPanel')[0]; assert.equal(form.props.readOnly, true);
  form.props.onActionStateChange({ currentRevision: 1, status: 'submitted' }); form.props.onActionStateChange({ currentRevision: 2, status: 'submitted' });
  assert.ok(f.invalidated.some(entry => entry.queryKey[0] === 'subcontractor-reconciliation'));
});

test('history Xem mounts the exact readonly revision ledger, including before current form submission', () => {
  const f = fixture({ ...response(), available: false, reason: 'Chốt checklist trước khi đối soát.' }); let tree = f.render('SubcontractorReconciliationPanel', props);
  button(tree, 'Xem').props.onClick(); tree = f.render('SubcontractorReconciliationPanel', props);
  const history = named(tree, 'SubcontractorReconciliationPanel')[0]; assert.ok(history); assert.equal(history.props.revision, 1); assert.equal(history.props.readOnly, true);
});

test('current source anchor focuses after matching-hash availability, with no automatic or historical scroll', () => {
  const f = fixture({ available: false }); const calls: any[] = []; f.setPanelTarget({ scrollIntoView: (options: any) => calls.push(['scroll', options]), focus: (options: any) => calls.push(['focus', options]) });
  f.setHash('#subcontractor-reconciliation'); let tree = f.render('SubcontractorReconciliationPanel', props); assert.equal(tree.props.id, 'subcontractor-reconciliation'); assert.deepEqual(calls, []);
  f.setData(response()); f.render('SubcontractorReconciliationPanel', props); assert.deepEqual(calls, [['scroll', { block: 'start', behavior: 'smooth' }], ['focus', { preventScroll: true }]]);
  f.render('SubcontractorReconciliationPanel', props); assert.equal(calls.length, 2);
  const noHash = fixture(response()); noHash.setPanelTarget({ scrollIntoView: () => calls.push('unexpected'), focus: () => calls.push('unexpected') }); noHash.render('SubcontractorReconciliationPanel', props); assert.equal(calls.length, 2);
  const historical = fixture(response()); historical.setHash('#subcontractor-reconciliation'); historical.setPanelTarget({ scrollIntoView: () => calls.push('unexpected'), focus: () => calls.push('unexpected') }); tree = historical.render('SubcontractorReconciliationPanel', { ...props, revision: 1 }); assert.equal(tree.props.id, undefined); assert.equal(calls.length, 2);
});

test('closed virtual client ledger explicitly shows unsaved readonly state and denies edits without changing builtin copy', () => {
  const virtual = { ...response(), readonly: true, capabilities: { can_write: false }, reconciliation: { ...response().reconciliation, id: undefined } };
  const f = fixture(virtual); f.render('SubcontractorReconciliationPanel', props); const tree = f.render('SubcontractorReconciliationPanel', props);
  assert.match(text(tree), /Chưa lưu đối soát/); assert.match(text(tree), /Đối soát đang ở chế độ chỉ xem\./);
  assert.equal(button(tree, 'Lưu nháp').props.disabled, true); assert.equal(button(tree, 'Gửi đối soát').props.disabled, true); assert.equal(nodes(tree).find(node => node.type === 'textarea').props.disabled, true);
  const builtin = fixture(virtual).render('SubcontractorReconciliationPanel', { ...props, clientContext: false }); assert.doesNotMatch(text(builtin), /Chưa lưu đối soát|Đối soát đang ở chế độ chỉ xem/);
  const submitted = fixture({ ...virtual, reconciliation: { ...virtual.reconciliation, status: 'submitted', submitted_by_name: 'Actor' } }).render('SubcontractorReconciliationPanel', props); assert.match(text(submitted), /Đã gửi bởi/); assert.doesNotMatch(text(submitted), /Đối soát đang ở chế độ chỉ xem/);
});

test('history fetch uses exact revision and its own rows, with disabled writes and no current row fallback', async () => {
  const historical = { ...response(), readonly: true, capabilities: { can_write: false }, reconciliation: { ...response().reconciliation, comment: 'Old decision', items: [{ item_index: 3, checklist_title: 'Frozen old row', decision: 'rejected' }] } };
  const f = fixture(historical); f.render('SubcontractorReconciliationPanel', { ...props, revision: 1 });
  const tree = f.render('SubcontractorReconciliationPanel', { ...props, revision: 1 });
  const row = nodes(tree).find(node => node.props?.item?.checklist_title === 'Frozen old row'); assert.ok(row); assert.equal(row.props.disabled, true); assert.equal(button(tree, 'Lưu nháp').props.disabled, true);
  const paths: string[] = []; (globalThis as any).__reconciliationRequest = async (path: string) => { paths.push(path); return historical; }; await f.queries[0].queryFn(); assert.deepEqual(paths, ['/work-items/work/subcontractor-reconciliation?revision=1']);
  f.setData({ available: false, readonly: true, reason: 'Không có phiên bản này.' }); const missing = f.render('SubcontractorReconciliationPanel', { ...props, revision: 1 }); assert.equal(nodes(missing).filter(node => node.props?.item?.checklist_title).length, 0);
});

test('draft writes send baseline If-Match, preserve edits during refetch and fail closed on stale conflict', async () => {
  const f = fixture(response()); f.render('SubcontractorReconciliationPanel', props); let tree = f.render('SubcontractorReconciliationPanel', props);
  nodes(tree).find(node => node.type === 'textarea').props.onChange({ target: { value: 'Unsaved draft' } });
  f.setData({ ...response(), precondition: '"newer"', reconciliation: { ...response().reconciliation, items: [{ item_index: 0, checklist_title: 'New source row', confirmed_quantity: 99, decision: 'accepted' }] } }); tree = f.render('SubcontractorReconciliationPanel', props); assert.equal(nodes(tree).find(node => node.type === 'textarea').props.value, 'Unsaved draft'); assert.equal(nodes(tree).find(node => node.props?.item?.checklist_title).props.item.checklist_title, 'Source row');
  const requests: any[] = []; (globalThis as any).__reconciliationRequest = async (path: string, init: any) => { requests.push({ path, init }); throw new f.exports.ApiError('Stale', 'PRECONDITION_FAILED', 412); };
  await button(tree, 'Lưu nháp').props.onClick();
  assert.equal(requests[0].init.headers['If-Match'], '"baseline"'); assert.equal(JSON.parse(requests[0].init.body).comment, 'Unsaved draft'); assert.equal(JSON.parse(requests[0].init.body).form_revision_no, undefined);
  tree = f.render('SubcontractorReconciliationPanel', props); assert.equal(nodes(tree).find(node => node.type === 'textarea').props.value, 'Unsaved draft'); assert.equal(button(tree, 'Lưu nháp').props.disabled, true); assert.ok(button(tree, 'Tải lại dữ liệu đối soát'));
  await button(tree, 'Lưu nháp').props.onClick(); assert.equal(requests.length, 1);
  await button(tree, 'Tải lại dữ liệu đối soát').props.onClick(); tree = f.render('SubcontractorReconciliationPanel', props); assert.equal(nodes(tree).find(node => node.type === 'textarea').props.value, 'Saved'); assert.equal(button(tree, 'Lưu nháp').props.disabled, false);
  // Missing write capability denies requests even when ledger is still draft.
  f.setData({ ...response(), capabilities: { can_write: false } }); tree = f.render('SubcontractorReconciliationPanel', props); assert.equal(button(tree, 'Gửi đối soát').props.disabled, true);
});
