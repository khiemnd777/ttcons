import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';
import {
  constructionHandoffIntentMatches,
  constructionHandoffNavigationState,
  consumeConstructionHandoffNavigationState,
  consumeProjectTaskDetailNavigationState,
  currentWorkspaceReturnTo,
  projectTaskDetailIntentMatches,
  projectTaskDetailNavigationState,
} from '../src/modules/work-items/workItemNavigation.ts';

async function source(path: string) {
  return readFile(new URL(path, import.meta.url), 'utf8');
}

test('Planning return navigation removes stale task context without losing unrelated location state', () => {
  assert.equal(
    currentWorkspaceReturnTo({
      pathname: '/projects/project-1/planning',
      search: '?task=work-1&queue=actionable&returnTo=%2Fprojects%2Fproject-1%2Fplanning%3Ftask%3Dwork-1',
      hash: '#work-queue',
    }),
    '/projects/project-1/planning?queue=actionable#work-queue',
  );
});

test('construction handoff intent is project-scoped, one-time and preserves unrelated router state', () => {
  const state = constructionHandoffNavigationState('project-1', { source: 'work-item' });
  assert.deepEqual(state, { source: 'work-item', constructionHandoffIntent: 'project-1' });
  assert.equal(constructionHandoffIntentMatches(state, 'project-1'), true);
  assert.equal(constructionHandoffIntentMatches(state, 'project-2'), false);
  assert.deepEqual(consumeConstructionHandoffNavigationState(state), { source: 'work-item' });
  assert.equal(consumeConstructionHandoffNavigationState(constructionHandoffNavigationState('project-1')), null);
});

test('project task detail intent is explicit, work-item scoped, one-time and preserves unrelated router state', () => {
  const currentState = { source: 'planning-queue' };
  const navigationState = projectTaskDetailNavigationState('work-1', currentState);

  assert.deepEqual(navigationState, {
    source: 'planning-queue',
    projectTaskDetailIntent: 'work-1',
  });
  assert.deepEqual(currentState, { source: 'planning-queue' });
  assert.equal(projectTaskDetailIntentMatches(navigationState, 'work-1'), true);
  assert.equal(projectTaskDetailIntentMatches(navigationState, 'work-2'), false);
  assert.equal(projectTaskDetailIntentMatches(null, 'work-1'), false);
  assert.deepEqual(consumeProjectTaskDetailNavigationState(navigationState), { source: 'planning-queue' });
  assert.equal(consumeProjectTaskDetailNavigationState(projectTaskDetailNavigationState('work-1')), null);
});

test('Planning embedded targets explicitly open the sole ProjectTaskContext dialog without query-driven auto-open', async () => {
  const planning = await source('../src/modules/planning/PlanningPage.tsx');
  const taskContext = await source('../src/modules/projects/ProjectTaskContext.tsx');

  assert.match(planning, /currentWorkspaceReturnTo\(location\)/);
  assert.match(planning, /executionTargetNeedsEmbeddedWorkItem\(enriched\.execution_target\)/);
  assert.match(planning, /navigate\(targetPath, \{ state: projectTaskDetailNavigationState\(enriched\.id, location\.state\) \}\)/);

  assert.match(taskContext, /const detailOpenIntent = projectTaskDetailIntentMatches\(location\.state, taskId\)/);
  assert.match(taskContext, /if \(!contextEnabled \|\| !detailOpenIntent\) return;/);
  assert.match(taskContext, /setDetailOpen\(true\);/);
  assert.match(taskContext, /consumeProjectTaskDetailNavigationState\(location\.state\)/);
  assert.equal((taskContext.match(/<WorkItemDetailDialog/g) ?? []).length, 1);
  assert.doesNotMatch(taskContext, /useState\(Boolean\(taskId\)\)/);
});
