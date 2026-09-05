import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { createRequire } from 'node:module';
import test from 'node:test';
import ts from 'typescript';
import { QueryClient, QueryObserver } from '@tanstack/react-query';
import * as updateHelpers from '../src/modules/projects/projectUpdates.ts';

const source = (path: string) => readFile(new URL(path, import.meta.url), 'utf8');

// Exercise the production hook's query options with the real query cache.
// Only the authentication and HTTP boundaries are substituted; no browser runs.
async function updateOptions(actorID: string | undefined, get: (path: string) => Promise<unknown>) {
  const code = await source('../src/modules/projects/useProjectUpdates.ts');
  const output = ts.transpileModule(code, { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 } }).outputText;
  const exports: Record<string, any> = {};
  const require = createRequire(import.meta.url);
  const substitutes: Record<string, unknown> = {
    '@tanstack/react-query': { useQuery: (options: unknown) => options },
    '../../lib/api/client': { get },
    '../../lib/auth/AuthContext': { useAuth: () => ({ user: actorID ? { id: actorID } : null }) },
    './projectUpdates': updateHelpers,
  };
  new Function('exports', 'require', output)(exports, (name: string) => name in substitutes ? substitutes[name] : require(name));
  return exports.useProjectUpdates;
}

test('Projects and Progress refresh read counts together without resetting deep stage pages or another actor', async () => {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: Infinity } } });
  let unread = 3;
  const get = async (path: string) => {
    const url = new URL(path, 'http://local');
    assert.equal(url.pathname, '/projects/updates-summary');
    return { items: url.searchParams.get('project_ids')!.split(',').map((project_id) => ({ project_id, notification_unread: unread, discussion_unread: 2, system_unread: 1, latest: null })) };
  };
  const options = await updateOptions('actor-a', get);
  const otherOptions = await updateOptions('actor-b', get);
  const explorer = new QueryObserver<any>(client, options(['p2', 'p1']));
  const progress = new QueryObserver<any>(client, options(['p1']));
  const otherActor = new QueryObserver<any>(client, otherOptions(['p1']));
  const stop = [explorer.subscribe(() => {}), progress.subscribe(() => {}), otherActor.subscribe(() => {})];
  const stageKey = ['project-progress-board-stage', 'actor-a', '', 'construction'];
  client.setQueryData(stageKey, { pages: [{ items: ['p1'], next_cursor: 'cursor-2' }, { items: ['p2'] }], pageParams: ['', 'cursor-2'] });
  const stageBeforeRead = client.getQueryState(stageKey);
  try {
    await Promise.all([explorer.refetch(), progress.refetch(), otherActor.refetch()]);
    assert.equal(progress.getCurrentResult().data.items[0].notification_unread, 3);
    unread = 0;
    // Same actor-prefix invalidation as the shared dialog's explicit read action.
    await client.invalidateQueries({ queryKey: ['project-updates', 'actor-a'] });
    assert.equal(explorer.getCurrentResult().data.items[0].notification_unread, 0);
    assert.equal(progress.getCurrentResult().data.items[0].notification_unread, 0);
    assert.equal(otherActor.getCurrentResult().data.items[0].notification_unread, 3);
    assert.strictEqual(client.getQueryState(stageKey), stageBeforeRead);
    assert.equal(options([]).enabled, false);
    const anonymous = await updateOptions(undefined, get);
    assert.equal(anonymous(['p1']).enabled, false);
    assert.deepEqual(options(['p2', 'p1', 'p2']).queryKey, options(['p1', 'p2']).queryKey);
  } finally {
    stop.forEach((unsubscribe) => unsubscribe());
    client.clear();
  }
});

test('Progress update actions are outside navigation links and the dialog belongs to the page', async () => {
  const code = await source('../src/modules/projects/ProjectProgressPage.tsx');
  const file = ts.createSourceFile('ProjectProgressPage.tsx', code, ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);
  const owners = new Map<string, ts.FunctionDeclaration>();
  for (const statement of file.statements) {
    if (ts.isFunctionDeclaration(statement) && statement.name) owners.set(statement.name.text, statement);
  }
  const components = (owner: string) => {
    const found: { tag: string; ancestors: string[] }[] = [];
    const visit = (node: ts.Node, ancestors: string[]) => {
      if (ts.isJsxElement(node)) {
        const tag = node.openingElement.tagName.getText(file);
        found.push({ tag, ancestors });
        node.children.forEach((child) => visit(child, [...ancestors, tag]));
      } else if (ts.isJsxSelfClosingElement(node)) {
        found.push({ tag: node.tagName.getText(file), ancestors });
      } else ts.forEachChild(node, (child) => visit(child, ancestors));
    };
    visit(owners.get(owner)!, []);
    return found;
  };
  const card = components('ProgressCard');
  assert.ok(card.some(({ tag }) => tag === 'Link'), 'project detail remains reachable');
  const indicators = card.find(({ tag }) => tag === 'ProjectUpdateIndicators');
  assert.ok(indicators);
  assert.ok(!indicators.ancestors.includes('Link'), 'update buttons must not navigate to project detail');
  assert.equal(components('ProgressCard').filter(({ tag }) => tag === 'ProjectUpdatesDialog').length, 0);
  assert.equal(components('ProgressColumn').filter(({ tag }) => tag === 'ProjectUpdatesDialog').length, 0);
  assert.equal(components('ProjectProgressPage').filter(({ tag }) => tag === 'ProjectUpdatesDialog').length, 1);
});
