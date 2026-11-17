import assert from 'node:assert/strict';
import test from 'node:test';
import { createBrowserRouter } from 'react-router-dom';
import { safeReturnPath, workItemDetailPath } from '../src/modules/work-items/workItemNavigation.ts';
import { executionTargetPath } from '../src/modules/work-items/workItemExecutionTarget.ts';
import type { WorkItem } from '../src/lib/types/domain.ts';
import { withWordless } from '../src/lib/utils/wordless.ts';

const unsafe = [
  '', 'https://attacker.invalid', '//attacker.invalid', '/\\attacker.invalid',
  '/\t/attacker.invalid', '/\n/attacker.invalid', '/\r/attacker.invalid',
  '/my-work\u0000', '/my-work\u007f', '\n/my-work', '/projects/p\\../q',
  '/a/..//attacker.invalid',
];

test('rejects unsafe direct and once-decoded return destinations before URL normalization', () => {
  for (const value of unsafe) {
    assert.equal(safeReturnPath(value), '', JSON.stringify(value));
    const encoded = new URLSearchParams({ returnTo: value });
    assert.equal(safeReturnPath(new URLSearchParams(encoded.toString()).get('returnTo')), '', encoded.toString());
    assert.equal(workItemDetailPath('w', value), '/work-items/w');
  }
});

test('preserves internal query/hash, Wordless selection and nested encoded data without double decoding', () => {
  const path = '/my-work?next=https%3A%2F%2Fexample.invalid&returnTo=%2F%255Cvalue&wordless=0#section';
  assert.equal(safeReturnPath(path), path);
  assert.equal(safeReturnPath(' /my-work?bucket=mine#top '), '/my-work?bucket=mine#top');
  assert.equal(safeReturnPath('/projects/p/../q/boq'), '/projects/q/boq');
  const target = new URL(workItemDetailPath('w', path, 'p'), 'https://consflow.invalid');
  assert.equal(target.searchParams.get('returnTo'), path);
});

test('execution targets share the guard and check project scope after path normalization', () => {
  const item = (route: string, project_id = 'p') => ({ id: 'w', project_id, execution_target: { route } }) as WorkItem;
  for (const route of [...unsafe, '/projects/p/../q/boq', '/projects/p-other/boq']) {
    assert.equal(executionTargetPath(item(route)), '/projects/p/work-items/w', JSON.stringify(route));
  }
  assert.equal(executionTargetPath(item('/\\attacker.invalid', '')), '/work-items/w');
  assert.equal(executionTargetPath(item('/projects/p/boq?task=w#budget'), '/my-work?bucket=mine'),
    '/projects/p/boq?task=w&returnTo=%2Fmy-work%3Fbucket%3Dmine#budget');
  assert.equal(executionTargetPath(item('/projects/p/boq?task=w'), '/\\attacker.invalid'), '/projects/p/boq?task=w');
  const path = new URL(executionTargetPath(item('/projects/p/planning?task=w'), '/projects/p/planning?task=old&returnTo=%2Fmy-work&wordless=0#plan'), 'https://consflow.invalid');
  assert.equal(path.searchParams.get('returnTo'), '/projects/p/planning?task=old&returnTo=%2Fmy-work&wordless=0#plan');
  assert.equal(path.searchParams.get('wordless'), '0');
  const wordless = new URL(executionTargetPath(item('/projects/p/planning?task=w'), '/projects/p/planning?task=old&returnTo=%2Fmy-work&wordless=1#plan'), 'https://consflow.invalid');
  assert.equal(wordless.searchParams.get('returnTo'), '/projects/p/planning?wordless=1#plan');
  assert.equal(wordless.searchParams.get('wordless'), '1');
});

test('guarded WorkItem close destinations navigate through the public Data Router without external assignment or exception', async () => {
  // Synthetic History only: no browser or network navigation is started.
  let current = new URL('https://consflow.invalid/work-items/w');
  const assigned: string[] = [];
  const location = {
    get href() { return current.href; }, get origin() { return current.origin; },
    get pathname() { return current.pathname; }, get search() { return current.search; }, get hash() { return current.hash; },
    assign(to: string) { assigned.push(to); },
  };
  const history = {
    state: { idx: 0 },
    pushState(state: { idx: number }, _title: string, to: string) {
      const next = new URL(to, current);
      assert.equal(next.origin, current.origin);
      this.state = state; current = next;
    },
    replaceState(state: { idx: number }, _title: string, to?: string) { this.state = state; if (to) current = new URL(to, current); },
    go() {},
  };
  const fakeWindow = { location, history, document: { createElement() { return {}; } }, addEventListener() {}, removeEventListener() {}, sessionStorage: { getItem() { return null; }, setItem() {}, removeItem() {} } };
  const previous = Object.getOwnPropertyDescriptor(globalThis, 'window');
  Object.defineProperty(globalThis, 'window', { value: fakeWindow, configurable: true });
  let router: ReturnType<typeof createBrowserRouter> | undefined;
  try {
    router = createBrowserRouter([{ id: 'detail', path: '*', element: null }], { window: fakeWindow as unknown as Window });
    for (const value of [...unsafe, '/my-work?bucket=mine#top']) {
      const decoded = new URLSearchParams(new URLSearchParams({ returnTo: value }).toString()).get('returnTo');
      const target = withWordless(safeReturnPath(decoded) || '/my-work', '');
      await router.navigate(target, { fromRouteId: 'detail' });
      assert.equal(current.origin, 'https://consflow.invalid');
      assert.equal(`${current.pathname}${current.search}${current.hash}`, unsafe.includes(value) ? '/my-work' : value);
    }
    assert.deepEqual(assigned, []);
  } finally {
    router?.dispose();
    if (previous) Object.defineProperty(globalThis, 'window', previous);
    else Reflect.deleteProperty(globalThis, 'window');
  }
});
