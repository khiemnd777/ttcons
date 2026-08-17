import assert from 'node:assert/strict';
import test from 'node:test';
import type { WorkItem } from '../src/lib/types/domain.ts';
import {
  executionTargetActionLabel,
  executionTargetPath,
  executionTargetWorkspaceLabel,
  workItemMatchesProject,
} from '../src/modules/work-items/workItemExecutionTarget.ts';

function item(values: Partial<WorkItem> = {}): WorkItem {
  return {
    id: 'work-1',
    project_id: 'project-1',
    title: 'Chốt BOQ',
    status: 'ready',
    ...values,
  } as WorkItem;
}

test('opens the backend-owned business workspace and preserves return navigation', () => {
  const work = item({
    execution_target: {
      kind: 'domain',
      workspace: 'boq',
      route: '/projects/project-1/boq?task=work-1',
      action_label: 'Cập nhật BOQ',
      completion_mode: 'domain_gate',
    },
  });

  assert.equal(
    executionTargetPath(work, '/my-work?bucket=need_action'),
    '/projects/project-1/boq?task=work-1&returnTo=%2Fmy-work%3Fbucket%3Dneed_action',
  );
  assert.equal(executionTargetActionLabel(work), 'Cập nhật BOQ');
  assert.equal(executionTargetWorkspaceLabel(work), 'BOQ');
});

test('fails closed to work-item detail when a target is missing or external', () => {
  assert.equal(executionTargetPath(item(), '/my-work'), '/projects/project-1/work-items/work-1?returnTo=%2Fmy-work');
  assert.equal(executionTargetPath(item({
    execution_target: {
      kind: 'domain', workspace: 'boq', route: 'https://example.com', action_label: 'Mở', completion_mode: 'domain_gate',
    },
  })), '/projects/project-1/work-items/work-1');
  assert.equal(executionTargetPath(item({
    execution_target: {
      kind: 'domain', workspace: 'boq', route: '/projects/project-2/boq?task=work-1', action_label: 'Mở', completion_mode: 'domain_gate',
    },
  })), '/projects/project-1/work-items/work-1');
});

test('rejects task context from another project', () => {
  assert.equal(workItemMatchesProject(item(), 'project-1'), true);
  assert.equal(workItemMatchesProject(item(), 'project-2'), false);
});
