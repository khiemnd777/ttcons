import assert from 'node:assert/strict';
import test from 'node:test';
import { build } from 'esbuild';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';

// Exercise the actual dialog's handlers with controlled query/HTTP boundaries.
// Browser coverage separately verifies DOM, focus and real React Query behavior.
const require = createRequire(import.meta.url);
const bundle = await build({
  entryPoints: [fileURLToPath(new URL('../src/modules/notifications/NotificationPreferencesDialog.tsx', import.meta.url))],
  bundle: true, write: false, platform: 'node', format: 'cjs', packages: 'external', jsx: 'automatic',
  plugins: [{ name: 'dialog-boundaries', setup(builder) {
    builder.onResolve({ filter: /\/lib\/api\/client$/ }, () => ({ path: 'fixture:api', external: true }));
    builder.onResolve({ filter: /\/components\/ToastProvider$/ }, () => ({ path: 'fixture:toast', external: true }));
    builder.onResolve({ filter: /\/components\/Dialog$/ }, () => ({ path: 'fixture:dialog', external: true }));
  } }],
});
function descendants(node: any): any[] {
  if (!node || typeof node !== 'object') return [];
  return [node, ...[node.props?.children, node.props?.footer].flat(Infinity).flatMap(descendants)];
}
function content(node: any): string {
  if (node == null || typeof node === 'boolean') return '';
  return typeof node === 'object' ? [node.props?.children].flat(Infinity).map(content).join(' ') : String(node);
}
const event = (key: string, required = false) => ({ event_key: key, category_key: key, category_label: key, enabled: true, required_delivery: required });
function fixture(catalogData = [event('required', true), event('one'), event('two')]) {
  const states: any[] = [], effects: any[] = [];
  let cursor = 0, pending = false, mutationError = false, mutation: any, running: Promise<any> = Promise.resolve(), failKey = '';
  let closed = 0, catalogRetries = 0, preferenceRetries = 0;
  const writes: any[] = [], success: string[] = [], errors: string[] = [];
  const server: Record<string, boolean> = {};
  const flags = { isPending: false, isError: false, isSuccess: true, isFetching: false };
  const preferences: any = { ...flags, data: [], refetch: async () => {
    preferenceRetries++;
    preferences.data = Object.entries(server).map(([category_key, in_app_enabled]) => ({ category_key, event_key: '*', in_app_enabled }));
    return { data: preferences.data };
  } };
  const catalog: any = { ...flags, data: catalogData, refetch: async () => { catalogRetries++; } };
  const hooks = { ...require('react'),
    useState(initial: any) { const at = cursor++; if (!(at in states)) states[at] = typeof initial === 'function' ? initial() : initial; return [states[at], (value: any) => { states[at] = typeof value === 'function' ? value(states[at]) : value; }]; },
    useRef(initial: any) { return hooks.useState(() => ({ current: initial }))[0]; },
    useMemo: (fn: any) => fn(),
    useEffect(fn: any, deps: any[]) { const at = cursor++; if (!states[at] || deps.some((value, index) => value !== states[at][index])) { states[at] = deps; effects.push(fn); } },
  };
  const module = { exports: {} as any };
  new Function('require', 'module', 'exports', bundle.outputFiles[0].text)((name: string) => {
    if (name === 'react') return hooks;
    if (name === 'fixture:dialog') return { Dialog: 'dialog-fixture' };
    if (name === 'fixture:toast') return { useToast: () => ({ success: (text: string) => success.push(text), error: (text: string) => errors.push(text) }) };
    if (name === 'fixture:api') return { get: async () => preferences.data, put: async (_path: string, value: any) => {
      writes.push(value);
      if (value.category_key === failKey) throw new Error('simulated write failure');
      server[value.category_key] = value.in_app_enabled;
    } };
    if (name === '@tanstack/react-query') return {
      useQuery: () => preferences,
      useQueryClient: () => ({ invalidateQueries: () => preferences.refetch() }),
      useMutation: (options: any) => {
        mutation = options;
        return { isPending: pending, isError: mutationError, reset: () => { mutationError = false; }, mutate: () => {
          pending = true;
          running = Promise.resolve().then(options.mutationFn).then(options.onSuccess, async (error: Error) => { mutationError = true; await options.onError(error); }).finally(() => { pending = false; options.onSettled(); });
        } };
      },
    };
    return require(name);
  }, module, module.exports);
  const api = {
    catalog, preferences, writes, success, errors,
    render(open = true) { cursor = 0; const tree = module.exports.NotificationPreferencesDialog({ open, catalog, onClose: () => closed++ }); effects.splice(0).forEach(fn => fn()); return tree; },
    button(tree: any, label = 'Lưu tùy chọn') { return descendants(tree).find(node => node.type === 'button' && content(node) === label); },
    toggle(key: string, checked: boolean) { const label = descendants(api.render()).find(node => node.type === 'label' && content(node).includes(key)); descendants(label).find(node => node.type === 'input').props.onChange({ target: { checked } }); },
    flush: () => running,
    rawMutation: () => mutation.mutationFn(),
    fail: (key: string) => { failKey = key; },
    counters: () => ({ closed, catalogRetries, preferenceRetries }),
  };
  return api;
}

test('catalog and preference failures never masquerade as an empty catalog; retry only the failed source', () => {
  for (const source of ['catalog', 'preferences'] as const) {
    const f = fixture(); f[source].isError = true; f[source].isSuccess = false;
    const tree = f.render();
    assert.match(content(tree), source === 'catalog' ? /Không tải được danh mục/ : /Không tải được tùy chọn đã lưu/);
    assert.doesNotMatch(content(tree), /Chưa có nhóm/);
    assert.equal(f.button(tree).props.disabled, true);
    f.button(tree, 'Thử lại').props.onClick();
    assert.equal(f.counters().catalogRetries, source === 'catalog' ? 1 : 0);
    assert.equal(f.counters().preferenceRetries, source === 'preferences' ? 1 : 0);
  }
});

test('loading and a successful empty catalog cannot issue writes, including direct mutation invocation', async () => {
  for (const loading of [false, true]) {
    const f = fixture([]);
    if (loading) { f.catalog.isPending = true; f.catalog.isSuccess = false; }
    const tree = f.render();
    if (loading) assert.equal(f.button(tree).props.disabled, true);
    else { assert.match(content(tree), /Chưa có nhóm thông báo để cấu hình/); assert.equal(f.button(tree), undefined); }
    await assert.rejects(f.rawMutation(), /chưa sẵn sàng/);
    assert.equal(f.writes.length, 0); assert.equal(f.success.length, 0);
  }
});

test('required groups stay locked; save writes only actual changes and prevents repeated submission', async () => {
  const f = fixture(); let tree = f.render();
  assert.equal(f.button(tree).props.disabled, true);
  const required = descendants(tree).find(node => node.type === 'label' && content(node).includes('required'));
  assert.equal(descendants(required).find(node => node.type === 'input').props.disabled, true);
  f.toggle('one', false); tree = f.render();
  const save = f.button(tree); assert.equal(save.props.disabled, false);
  save.props.onClick(); save.props.onClick(); await f.flush();
  assert.deepEqual(f.writes.map(value => [value.category_key, value.in_app_enabled]), [['one', false]]);
  assert.equal(f.success.length, 1); assert.equal(f.counters().closed, 1);
});

test('partial failure reconciles saved groups while retaining unsaved choices for retry', async () => {
  const f = fixture(); f.toggle('one', false); f.toggle('two', false); f.fail('two');
  f.button(f.render()).props.onClick(); await f.flush();
  assert.equal(f.success.length, 0); assert.equal(f.counters().closed, 0);
  assert.match(f.errors[0], /Một số thay đổi có thể đã được lưu/);
  assert.equal(f.counters().preferenceRetries, 1);
  f.fail(''); f.button(f.render()).props.onClick(); await f.flush();
  assert.deepEqual(f.writes.map(value => value.category_key), ['one', 'two', 'two']);
  assert.equal(f.success.length, 1); assert.equal(f.counters().closed, 1);
});

test('closing after a failed save starts the next dialog session without stale error or choices', async () => {
  const f = fixture(); f.toggle('one', false); f.fail('one');
  f.button(f.render()).props.onClick(); await f.flush();
  assert.match(content(f.render()), /Chưa lưu được đầy đủ/);
  f.render(false);
  const reopened = f.render(true);
  assert.doesNotMatch(content(reopened), /Chưa lưu được đầy đủ/);
  assert.equal(f.button(reopened).props.disabled, true);
});
