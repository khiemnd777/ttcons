import assert from 'node:assert/strict';
import test from 'node:test';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';
import { build } from 'esbuild';
const require = createRequire(import.meta.url); const react = require('react');
const bundle = await build({ stdin: { resolveDir: fileURLToPath(new URL('../src/', import.meta.url)), contents: `export {UtilityDock} from './modules/utilities/UtilityDock'; export {UtilityDockLauncherProvider,UtilityDockLauncherHost} from './modules/utilities/UtilityDockLauncherHost'; export {AppShell} from './layouts/AppShell'; export {ProjectWorkspaceLayout} from './modules/projects/ProjectWorkspaceLayout';` }, bundle: true, write: false, platform: 'node', format: 'cjs', packages: 'external', jsx: 'automatic', loader: { '.css': 'empty' }, define: { 'import.meta.env': '{}', 'import.meta.url': '"file:///fixture"' }, plugins: [{ name: 'controlled-app-boundaries', setup(builder) {
  builder.onLoad({ filter: /\/lib\/api\/client\.ts$/ }, () => ({ contents: `export class ApiError extends Error {}; export const get=async()=>({}); export const api=get; export const post=get; export const put=get; export const del=get; export const postForm=get; export const postFormWithProgress=get; export const downloadBlob=get; export const postDownloadBlob=get; export function setToken(){}; export function setAuthFailureHandler(){}; export function logoutRequest(){};`, loader: 'js' }));
  builder.onLoad({ filter: /AuthContext\.tsx$/ }, () => ({ contents: `export const useAuth=()=>({user:{id:'actor',roles:[]},logout:async()=>{},hasAnyPermission:()=>true,hasPermission:()=>true});`, loader: 'js' }));
  builder.onLoad({ filter: /ToastProvider\.tsx$/ }, () => ({ contents: `export const useToast=()=>({success(){},error(){},warn(){}});`, loader: 'js' }));
} }] });
function nodes(tree: any, result: any[] = []): any[] { if (!tree || typeof tree !== 'object') return result; result.push(tree); for (const child of [tree.props?.children].flat(Infinity)) nodes(child, result); if (tree.portal) nodes(tree.child, result); return result; }
function text(tree: any): string { if (tree === null || tree === undefined || typeof tree === 'boolean') return ''; if (typeof tree !== 'object') return String(tree); return [tree.props?.children].flat(Infinity).map(text).join(' '); }
function named(tree: any, name: string) { return nodes(tree).filter(node => node.type?.name === name); }
function launchers(tree: any) { return nodes(tree).filter(node => node.type === 'button' && String(node.props['aria-label']).startsWith('Mở tiện ích,')); }
function fixture() {
  let pathname = '/projects/project/operations'; let owner = ''; let cursor = 0; let unread = 2; let work = 3;
  const stores = new Map<string, any[]>(); const memoStores = new Map<string, any[]>();
  const hooks = { ...react,
    createContext(value: any) { const context = react.createContext(value); context.current = value; context.Provider.context = context; return context; }, useContext: (context: any) => context.current,
    useState(value: any) { const values = stores.get(owner)!; const at = cursor++; if (!(at in values)) values[at] = typeof value === 'function' ? value() : value; return [values[at], (next: any) => { values[at] = typeof next === 'function' ? next(values[at]) : next; }]; },
    useRef(value: any) { return hooks.useState(() => ({ current: value }))[0]; }, useEffect() {}, useDeferredValue: (value: any) => value,
    useMemo(fn: any, deps: any[]) { const at = cursor++; const values = memoStores.get(owner)!; if (!values[at] || deps.some((value, i) => value !== values[at].deps[i])) values[at] = { deps, value: fn() }; return values[at].value; },
    useCallback(fn: any, deps: any[]) { return hooks.useMemo(() => fn, deps); },
  };
  const project = { project_id: 'project', project_code: 'P1', project_name: 'Project', unread_count: 2, can_post: true };
  const module = { exports: {} as any };
  new Function('require', 'module', 'exports', bundle.outputFiles[0].text)((id: string) => id === 'react' ? hooks : id === 'react-dom' ? { ...require(id), createPortal: (child: any, host: any) => ({ portal: true, child, host }) } : id === 'react-router-dom' ? { ...require(id), useLocation: () => ({ pathname, search: '', hash: '' }), useNavigate: () => () => {}, useParams: () => ({ id: 'project' }) } : id === '@tanstack/react-query' ? {
    useQuery(config: any) { return { data: config.queryKey[0] === 'my-work-navigation-summary' ? { work_total: work } : config.queryKey[0] === 'project-discussions' ? { unread_total: unread, items: [project] } : {}, isLoading: false, isFetching: false, isError: false }; }, useQueryClient: () => ({ invalidateQueries() {} }), useMutation: () => ({ isPending: false, mutate() {} }),
  } : require(id), module, module.exports);
  function render(nameOrFunction: any, props: any = {}, identity?: string) { owner = identity ?? (typeof nameOrFunction === 'string' ? nameOrFunction : nameOrFunction.name); if (!stores.has(owner)) { stores.set(owner, []); memoStores.set(owner, []); } cursor = 0; return (typeof nameOrFunction === 'string' ? module.exports[nameOrFunction] : nameOrFunction)(props); }
  function provider() { const tree = render('UtilityDockLauncherProvider', { children: null }); tree.type.context.current = tree.props.value; return tree.props.value; }
  provider(); return { render, provider, setPath(next: string) { pathname = next; }, counts(nextWork: number, nextUnread: number) { work = nextWork; unread = nextUnread; } };
}

test('a delayed project host receives exactly the existing labelled launcher with dynamic count and no floating duplicate', () => {
  const f = fixture(); let tree = f.render('UtilityDock'); assert.equal(launchers(tree).length, 1); assert.equal(nodes(tree).filter(node => node.portal).length, 0);
  const host = f.render('UtilityDockLauncherHost', { presentation: 'label' }, 'project-host'); const element = { id: 'project-header' }; host.ref(element); f.provider(); tree = f.render('UtilityDock');
  const portal = nodes(tree).find(node => node.portal); assert.equal(portal.host, element); assert.equal(launchers(tree).length, 1); assert.match(text(portal.child), /Tiện ích/); assert.equal(named(portal.child, 'CountBadge')[0].props.value, 5);
  f.counts(7, 4); tree = f.render('UtilityDock'); assert.equal(named(nodes(tree).find(node => node.portal).child, 'CountBadge')[0].props.value, 11);
});

test('host replacement, stale cleanup and StrictMode detach/reattach preserve the newest live host; unmount restores Pipeline fallback', () => {
  const f = fixture(); const project = f.render('UtilityDockLauncherHost', { presentation: 'label' }, 'project-host'); const standard = f.render('UtilityDockLauncherHost', { presentation: 'icon' }, 'standard-host'); const first = { id: 'project' }; const second = { id: 'standard' };
  project.ref(first); f.provider(); standard.ref(second); project.ref(null); f.provider(); let tree = f.render('UtilityDock'); assert.equal(nodes(tree).find(node => node.portal).host, second); assert.doesNotMatch(text(launchers(tree)[0]), /Tiện ích/); assert.equal(named(launchers(tree)[0], 'CountBadge').length, 0); assert.match(text(launchers(tree)[0]), /5/);
  standard.ref(null); standard.ref(second); f.provider(); tree = f.render('UtilityDock'); assert.equal(nodes(tree).find(node => node.portal).host, second); assert.equal(launchers(tree).length, 1);
  standard.ref(null); f.provider(); f.setPath('/pipeline-builder'); tree = f.render('UtilityDock'); assert.equal(nodes(tree).filter(node => node.portal).length, 0); assert.equal(launchers(tree).length, 1); assert.match(text(launchers(tree)[0]), /Tiện ích/);
  f.setPath('/projects'); tree = f.render('UtilityDock'); assert.equal(launchers(tree).length, 0);
});

test('moving project to standard host preserves conversation component/key and minimizing keeps its session lane', () => {
  const f = fixture(); const host = f.render('UtilityDockLauncherHost', { presentation: 'label' }, 'project-host'); host.ref({ id: 'project' }); f.provider(); let tree = f.render('UtilityDock'); launchers(tree)[0].props.onClick(); tree = f.render('UtilityDock');
  const before = named(tree, 'ProjectConversation')[0]; const lane = nodes(tree).find(node => node.props?.['data-project-dock-lane']); assert.ok(before); assert.equal(lane.key, 'project'); assert.equal(launchers(tree)[0].props['aria-expanded'], true);
  const standard = f.render('UtilityDockLauncherHost', { presentation: 'icon' }, 'standard-host'); standard.ref({ id: 'standard' }); host.ref(null); f.provider(); f.setPath('/projects'); tree = f.render('UtilityDock');
  assert.equal(named(tree, 'ProjectConversation')[0].type, before.type); assert.equal(named(tree, 'ProjectConversation')[0].props.project.project_id, 'project'); assert.equal(nodes(tree).find(node => node.props?.['data-project-dock-lane']).key, lane.key); assert.equal(launchers(tree)[0].props['aria-expanded'], true);
  launchers(tree)[0].props.onClick(); tree = f.render('UtilityDock'); assert.equal(launchers(tree)[0].props['aria-expanded'], false); assert.equal(named(tree, 'ProjectConversation').length, 1);
});

test('project header owns the labelled host row; both AppShell branches keep the same provider and keyed Dock with an icon standard host', () => {
  const f = fixture(); const workspace = f.render('ProjectWorkspaceLayout'); const header = nodes(workspace).find(node => node.type === 'header'); const host = named(header, 'UtilityDockLauncherHost')[0]; assert.equal(host.props.presentation, 'label');
  const focus = f.render('AppShell'); f.setPath('/projects'); const standard = f.render('AppShell'); assert.equal(focus.type, standard.type); assert.equal(focus.type.name, 'UtilityDockLauncherProvider');
  assert.equal(named(focus, 'UtilityDock').length, 1); assert.equal(named(standard, 'UtilityDock').length, 1); assert.equal(named(focus, 'UtilityDock')[0].key, named(standard, 'UtilityDock')[0].key); assert.equal(named(standard, 'UtilityDockLauncherHost')[0].props.presentation, 'icon');
});
