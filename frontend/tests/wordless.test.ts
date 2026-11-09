import assert from 'node:assert/strict';
import test from 'node:test';
import { wordlessEnabled, wordlessScope, wordlessDetailScope, withWordless } from '../src/lib/utils/wordless.ts';
import { executionTargetPath } from '../src/modules/work-items/workItemExecutionTarget.ts';
import { projectPipelinePath, currentWorkspaceReturnTo } from '../src/modules/work-items/workItemNavigation.ts';

test('Wordless is default and exact zero explicitly selects legacy presentation', () => {
  assert.equal(wordlessEnabled(''), true);
  for (const query of ['?wordless', '?wordless=0', '?wordless=true', '?wordless=01', '?wordless=1&wordless=1', '?wordless=0&wordless=1']) assert.equal(wordlessEnabled(query), false, query);
  assert.equal(wordlessEnabled('?task=a&wordless=1'), true);
  for (const route of ['/projects/p/preparation', '/projects/p/planning', '/projects/p/pipeline/']) assert.equal(wordlessScope(route), true);
  for (const route of ['/my-work', '/projects/p', '/projects/p/operations', '/pipeline-builder', '/projects/p/planning-other']) assert.equal(wordlessScope(route), false);
  for (const route of ['/settings', '/statistics', '/statistics/projects', '/statistics/procurement', '/statistics/warehouse', '/statistics/staff']) assert.equal(wordlessScope(route), true);
  assert.equal(wordlessScope('/statistics/not-a-report'), false);
  assert.equal(wordlessDetailScope('/projects/projects/work-items/w', '?returnTo=%2Fstatistics%2Fprojects'), false);
});

test('navigation carries presentation only and preserves destination query/hash', () => {
  const path = withWordless('/projects/p/pipeline?stage=s#work', '?task=old&filter=old&wordless=1');
  assert.equal(path, '/projects/p/pipeline?stage=s&wordless=1#work');
  assert.equal(withWordless(path, ''), '/projects/p/pipeline?stage=s#work');
  assert.equal(withWordless(path, '?wordless=0'), '/projects/p/pipeline?stage=s&wordless=0#work');
  assert.equal(withWordless(path, '?wordless=1&wordless=1'), '/projects/p/pipeline?stage=s&wordless=0#work');
  assert.equal(withWordless('', '?wordless=1'), '');
  assert.equal(withWordless('//external.test/projects/p/planning', '?wordless=1'), '//external.test/projects/p/planning');
  assert.equal(withWordless('/projects/p/operations', '?wordless=1'), '/projects/p/operations?wordless=1');
});

test('task execution inherits mode without nested return context and can roll back', () => {
  const source = currentWorkspaceReturnTo({ pathname: '/projects/p/pipeline', search: '?wordless=1&stage=s&task=old&returnTo=%2Fold' });
  assert.equal(source, '/projects/p/pipeline?wordless=1&stage=s');
  const item: any = { id: 'w', project_id: 'p', execution_target: { route: '/projects/p/planning?task=w', workspace: 'planning', kind: 'capture' } };
  const target = new URL(executionTargetPath(item, source), 'http://local');
  assert.equal(target.searchParams.get('task'), 'w');
  assert.equal(target.searchParams.get('wordless'), '1');
  assert.equal(target.searchParams.get('returnTo'), source);
  const rolledBack = withWordless(target.searchParams.get('returnTo')!, '?task=w&wordless=0');
  assert.equal(rolledBack, '/projects/p/pipeline?stage=s&wordless=0');
  const stagePath = projectPipelinePath('p', 'next', { search: '?wordless=1&task=w&returnTo=old', hash: '#work' });
  assert.equal(stagePath, '/projects/p/pipeline?wordless=1&stage=next#work');
});

test('standalone details require same-project provenance and outer URL controls rollback', () => {
  const path = '/projects/p/work-items/w';
  const query = '?returnTo=%2Fprojects%2Fp%2Fpipeline%3Fwordless%3D1';
  assert.equal(wordlessDetailScope(path, query), true);
  assert.equal(wordlessEnabled(query), true);
  assert.equal(wordlessEnabled(query + '&wordless=0'), false); // stale returnTo cannot override explicit rollback
  assert.equal(wordlessDetailScope(path, '?returnTo=%2Fprojects%2Fother%2Fpipeline'), false);
  assert.equal(wordlessDetailScope(path, '?returnTo=%2Fmy-work'), false);
  assert.equal(wordlessDetailScope(path, '?returnTo=%2Fprojects%2Fp%2Fpipeline&returnTo=%2Fprojects%2Fp%2Fplanning'), false);
  const item: any = { id: 'w', project_id: 'p' };
  const target = new URL(executionTargetPath(item, '/projects/p/pipeline?wordless=1'), 'http://local');
  assert.equal(target.pathname, path);
  assert.equal(target.searchParams.get('wordless'), '1');
  assert.equal(withWordless(target.pathname + target.search, '').includes('&wordless=1'), false);
});
