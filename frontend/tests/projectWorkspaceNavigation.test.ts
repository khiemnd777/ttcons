import assert from 'node:assert/strict';
import test from 'node:test';
import {
  projectWorkspaceGroupPath,
  visibleProjectWorkspaceGroups,
} from '../src/modules/projects/projectWorkspaceNavigation.ts';

function permissionCheck(granted: string[]) {
  const permissions = new Set(granted);
  return (required?: string[]) => !required?.length || required.some((permission) => permissions.has(permission));
}

function routeMap(granted: string[]) {
  return Object.fromEntries(
    visibleProjectWorkspaceGroups(permissionCheck(granted))
      .map((group) => [group.key, projectWorkspaceGroupPath('project-1', group)]),
  );
}

test('project explorer groups open their canonical first workspace for an admin', () => {
  const allPermissions = [
    'project.view', 'pipeline.view', 'planning.view', 'boq.view',
    'contract.view', 'procurement.view', 'warehouse.view',
    'operation.view', 'quality.view', 'acceptance.view', 'rework.view',
    'handover.view', 'accounting_handoff.view', 'project_finance.view',
    'audit.view',
  ];

  assert.deepEqual(routeMap(allPermissions), {
    overview: '/projects/project-1',
    planning: '/projects/project-1/preparation',
    materials: '/projects/project-1/boq',
    quality: '/projects/project-1/operations',
    finance: '/projects/project-1/handover',
    governance: '/projects/project-1?tab=members',
  });
});

test('project explorer exposes the lean accounting handoff permission without legacy finance rights', () => {
  assert.deepEqual(routeMap([
    'project.view', 'accounting_handoff.view', 'audit.view',
  ]), {
    overview: '/projects/project-1',
    planning: '/projects/project-1/preparation',
    finance: '/projects/project-1/accounting',
    governance: '/projects/project-1?tab=members',
  });
});

test('project explorer groups fall back to the first child the user may view', () => {
  assert.deepEqual(routeMap([
    'project.view', 'boq.view', 'warehouse.view', 'acceptance.view',
    'project_finance.view', 'audit.view',
  ]), {
    overview: '/projects/project-1',
    planning: '/projects/project-1/preparation',
    materials: '/projects/project-1/boq',
    quality: '/projects/project-1/acceptance',
    finance: '/projects/project-1/accounting',
    governance: '/projects/project-1?tab=members',
  });
});
