import assert from 'node:assert/strict';
import test from 'node:test';
import { createRequire } from 'node:module';
import { readFile } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
import { build } from 'esbuild';
import { createElement } from 'react';
import { renderToStaticMarkup } from 'react-dom/server';

// Real components with controlled React hooks and event targets; no browser or HTTP.
const require = createRequire(import.meta.url);
const react = require('react');
const bundle = await build({
  stdin: { resolveDir: fileURLToPath(new URL('../src/', import.meta.url)), contents: `
    export { ContextHelp } from './components/ContextHelp';
    export { IconAction } from './components/IconAction';
    export { DiscussionWorkRail } from './modules/work-items/DiscussionWorkRail';
    export { ReviewRail, ReportHistory } from './modules/work-reports/WorkReportReviewDialog';
  ` },
  bundle: true, write: false, platform: 'node', format: 'cjs', packages: 'external', jsx: 'automatic',
  loader: { '.css': 'empty' }, define: { 'import.meta.env': '{}', 'import.meta.url': '"file:///fixture"' },
  plugins: [{ name: 'read-only-boundaries', setup(builder) {
    builder.onLoad({ filter: /WorkReportReviewDialog\.tsx$/ }, async ({ path }) => ({ contents: `${await readFile(path, 'utf8')}\nexport { ReviewRail, ReportHistory };`, loader: 'tsx' }));
    builder.onLoad({ filter: /AuthContext\.tsx$/ }, () => ({ contents: 'export const useAuth=()=>({user:{id:"actor"},hasPermission:()=>false,hasAnyPermission:()=>false});', loader: 'js' }));
  } }],
});

function load(hooks = react, query: any = {}) {
  const module = { exports: {} as Record<string, any> };
  new Function('require', 'module', 'exports', bundle.outputFiles[0].text)((name: string) =>
    name === 'react' ? hooks : name === '@tanstack/react-query'
      ? { ...require(name), useQuery: () => query } : require(name), module, module.exports);
  return module.exports;
}
function nodes(tree: any, result: any[] = []): any[] {
  if (!tree || typeof tree !== 'object') return result;
  result.push(tree);
  for (const child of [tree.props?.children].flat(Infinity)) nodes(child, result);
  return result;
}
function text(tree: any): string {
  if (tree == null || typeof tree === 'boolean') return '';
  return typeof tree === 'object' ? [tree.props?.children].flat(Infinity).map(text).join(' ') : String(tree);
}
function harness(query: any = {}) {
  let cursor = 0;
  const state: any[] = [], deps: any[] = [], cleanups: any[] = [], effects: any[] = [];
  const hooks: any = { ...react,
    useState(initial: any) {
      const at = cursor++;
      if (!(at in state)) state[at] = typeof initial === 'function' ? initial() : initial;
      return [state[at], (value: any) => { state[at] = typeof value === 'function' ? value(state[at]) : value; }];
    },
    useRef(initial: any) { return hooks.useState(() => ({ current: initial }))[0]; },
    useId() { return hooks.useRef('help-test-id').current; },
    useEffect(effect: any, next: any[]) {
      const at = cursor++;
      if (!deps[at] || next.some((value, index) => value !== deps[at][index])) {
        deps[at] = next;
        effects.push(() => { cleanups[at]?.(); cleanups[at] = effect(); });
      }
    },
  };
  const exports = load(hooks, query);
  return {
    exports,
    render(name: string, props: any) {
      cursor = 0;
      const tree = exports[name](props);
      while (effects.length) effects.shift()();
      return tree;
    },
    unmount() { cleanups.forEach(cleanup => cleanup?.()); },
  };
}
class FakeNode {
  inside = new Set<any>();
  contains(node: any) { return this === node || this.inside.has(node); }
}
function withEvents(run: (environment: any) => void) {
  const previous = Object.fromEntries(['window', 'document', 'Node'].map(key => [key, Object.getOwnPropertyDescriptor(globalThis, key)]));
  const listeners = new Map<string, Set<Function>>();
  const dialogs: any[] = [];
  const target = (surface: string) => ({
    addEventListener(type: string, callback: Function) { const key = `${surface}:${type}`; if (!listeners.has(key)) listeners.set(key, new Set()); listeners.get(key)!.add(callback); },
    removeEventListener(type: string, callback: Function) { listeners.get(`${surface}:${type}`)?.delete(callback); },
  });
  Object.defineProperty(globalThis, 'window', { configurable: true, value: target('window') });
  Object.defineProperty(globalThis, 'document', { configurable: true, value: { ...target('document'), querySelectorAll: () => dialogs } });
  Object.defineProperty(globalThis, 'Node', { configurable: true, value: FakeNode });
  try {
    run({ dialogs, listeners, fire(surface: string, type: string, event: any) { for (const callback of listeners.get(`${surface}:${type}`) ?? []) callback(event); } });
  } finally {
    for (const [key, descriptor] of Object.entries(previous)) {
      if (descriptor) Object.defineProperty(globalThis, key, descriptor); else delete (globalThis as any)[key];
    }
  }
}
const helpProps = { label: 'Cách tính tồn kho', children: 'Tồn kho và vật tư dư được tổng hợp độc lập.' };
const helpButton = (tree: any) => nodes(tree).find(node => node.type === 'button');
const helpPanel = (tree: any) => nodes(tree).find(node => node.props?.id === 'help-test-id');

test('help is named, initially closed, toggles by button, and preserves optional explanation', () => withEvents(() => {
  const f = harness();
  let tree = f.render('ContextHelp', helpProps);
  assert.equal(helpButton(tree).props.type, 'button');
  assert.equal(helpButton(tree).props['aria-label'], helpProps.label);
  assert.equal(helpButton(tree).props['aria-controls'], helpPanel(tree).props.id);
  assert.equal(helpPanel(tree).props.hidden, true);
  helpButton(tree).props.onClick(); tree = f.render('ContextHelp', helpProps);
  assert.equal(helpButton(tree).props['aria-expanded'], true);
  assert.equal(helpPanel(tree).props.hidden, false);
  assert.match(text(helpPanel(tree)), /tổng hợp độc lập/);
  helpButton(tree).props.onClick(); tree = f.render('ContextHelp', helpProps);
  assert.equal(helpPanel(tree).props.hidden, true);
  f.unmount();
}));

test('Escape consumes the help dismissal before the enclosing dialog and restores trigger focus', () => withEvents(env => {
  const f = harness(); let tree = f.render('ContextHelp', helpProps); let focused = 0;
  tree.ref.current = new FakeNode(); helpButton(tree).ref.current = { focus: () => focused++ };
  env.dialogs.push({ contains: () => true });
  helpButton(tree).props.onClick(); tree = f.render('ContextHelp', helpProps);
  let prevented = 0, stopped = 0;
  env.fire('window', 'keydown', { key: 'Escape', preventDefault: () => prevented++, stopPropagation: () => stopped++ });
  tree = f.render('ContextHelp', helpProps);
  assert.equal(helpPanel(tree).props.hidden, true);
  assert.equal(prevented, 1); assert.equal(stopped, 1); assert.equal(focused, 1);
  assert.equal(env.listeners.get('window:keydown')?.size, 0);
  f.unmount();
}));

test('help respects a newer modal, outside pointer, keyboard focus leaving, and unmount cleanup', () => withEvents(env => {
  const f = harness(); let tree = f.render('ContextHelp', helpProps);
  const container = new FakeNode(), inside = new FakeNode(); container.inside.add(inside);
  tree.ref.current = container;
  helpButton(tree).props.onClick(); tree = f.render('ContextHelp', helpProps);
  env.dialogs.push({ contains: () => false });
  env.fire('window', 'keydown', { key: 'Escape', preventDefault() { assert.fail('newer modal owns Escape'); } });
  assert.equal(helpPanel(f.render('ContextHelp', helpProps)).props.hidden, false);
  env.dialogs.length = 0;
  env.fire('window', 'keydown', { key: 'Escape', defaultPrevented: true, preventDefault() { assert.fail('handled Escape must be left alone'); } });
  assert.equal(helpPanel(f.render('ContextHelp', helpProps)).props.hidden, false);
  env.fire('document', 'pointerdown', { target: inside });
  assert.equal(helpPanel(f.render('ContextHelp', helpProps)).props.hidden, false);
  env.fire('document', 'pointerdown', { target: new FakeNode() });
  tree = f.render('ContextHelp', helpProps); assert.equal(helpPanel(tree).props.hidden, true);
  helpButton(tree).props.onClick(); tree = f.render('ContextHelp', helpProps);
  tree.props.onBlur({ currentTarget: container, relatedTarget: inside });
  assert.equal(helpPanel(f.render('ContextHelp', helpProps)).props.hidden, false);
  tree.props.onBlur({ currentTarget: container, relatedTarget: new FakeNode() });
  tree = f.render('ContextHelp', helpProps); assert.equal(helpPanel(tree).props.hidden, true);
  helpButton(tree).props.onClick(); f.render('ContextHelp', helpProps);
  assert.equal(env.listeners.get('window:keydown')?.size, 1);
  f.unmount(); assert.equal(env.listeners.get('document:pointerdown')?.size, 0);
  assert.equal(env.listeners.get('window:keydown')?.size, 0);
}));

test('secondary icon action keeps its accessible name, disabled state and original callback', () => {
  let calls = 0;
  const props = { label: 'Làm mới audit', icon: createElement('span'), onClick: () => calls++ };
  const tree = load().IconAction(props);
  assert.equal(tree.props.type, 'button'); assert.equal(tree.props['aria-label'], 'Làm mới audit');
  assert.equal(tree.props.disabled, false); assert.match(tree.props.className, /min-h-11 min-w-11/);
  tree.props.onClick(); assert.equal(calls, 1);
  const disabled = load().IconAction({ ...props, disabled: true });
  assert.equal(disabled.props.disabled, true); // Native button owns disabled input suppression.
});

test('discussion rail preserves work identity, status, due date and distinct primary/discussion callbacks', () => {
  const item = Object.freeze({ id: 'work-1', title: 'Kiểm tra vật tư đầu vào', status: 'in_progress', due_at: '2026-10-27T10:00:00Z' });
  const query: any = { data: { total: 1, items: [item] }, isPending: false, isError: false };
  let opened: any, discussed: any, retried = 0;
  query.refetch = () => retried++;
  const f = harness(query);
  const props = { projectId: 'project-1', onOpen: (value: any) => opened = value, onDiscuss: (value: any) => discussed = value };
  let tree = f.render('DiscussionWorkRail', props);
  assert.match(text(tree), /Kiểm tra vật tư đầu vào/); assert.match(text(tree), /Hạn:/);
  const primary = nodes(tree).find(node => node.type === 'button' && node.props.className.includes('bg-blue-600'));
  assert.ok(text(primary).trim()); primary.props.onClick(); assert.equal(opened, item);
  const secondary = nodes(tree).find(node => node.type?.name === 'IconAction');
  assert.equal(secondary.props.label, `Xem trao đổi: ${item.title}`); secondary.props.onClick(); assert.equal(discussed, item);
  query.data = { total: 0, items: [] }; tree = f.render('DiscussionWorkRail', props); assert.match(text(tree), /Chưa có việc cần xử lý/);
  query.isError = true; tree = f.render('DiscussionWorkRail', props);
  nodes(tree).find(node => node.type === 'button' && text(node).trim() === 'Thử lại').props.onClick(); assert.equal(retried, 1);
  query.isError = false; query.isPending = true; tree = f.render('DiscussionWorkRail', props); assert.ok(nodes(tree).some(node => node.props.role === 'status'));
  f.unmount();
});

test('review rail and history retain revision, actor, decision warning, required feedback and exact comments', () => {
  const components = load();
  let feedback = '';
  const props = { detail: { status: 'submitted', author_name: 'Người lập thử', recipient_name: 'Người nhận thử' }, revision: { revision_no: 2, submitted_at: '2026-10-26T02:15:00Z' }, evidenceCount: 4, isCurrentRevision: true, mayReview: true, reviewComment: '', reviewCommentInvalid: true, onReviewCommentChange: (value: string) => feedback = value };
  const html = renderToStaticMarkup(createElement(components.ReviewRail, props));
  for (const value of ['Bản gửi', 'Lần 2', 'Hiện tại', '4 tệp', 'Người lập thử', 'Người nhận thử', 'không phải nghiệm thu chất lượng', 'Bắt buộc khi yêu cầu bổ sung']) assert.ok(html.includes(value), value);
  assert.match(html, /aria-invalid="true"/);
  const tree = components.ReviewRail(props);
  nodes(tree).find(node => node.type === 'textarea').props.onChange({ target: { value: 'Bổ sung ảnh nguồn' } }); assert.equal(feedback, 'Bổ sung ảnh nguồn');
  const events = Object.freeze([{ id: 'event-1', event_type: 'needs_changes', revision_no: 2, actor_name: 'Người phản hồi', created_at: '2026-10-26T03:30:00Z', comment: '<script>giữ nguyên nội dung</script>' }]);
  const history = renderToStaticMarkup(createElement(components.ReportHistory, { events }));
  assert.match(history, /Lần 2/); assert.match(history, /Người phản hồi/); assert.match(history, /10:30 26-10-2026/);
  assert.match(history, /&lt;script&gt;giữ nguyên nội dung&lt;\/script&gt;/); assert.doesNotMatch(history, /<script>/);
});
