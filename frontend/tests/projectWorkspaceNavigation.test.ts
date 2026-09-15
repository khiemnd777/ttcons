import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';
import {
  PROJECT_WORKSPACE_GROUPS,
  projectWorkspaceGroupPath,
  visibleProjectWorkspaceGroups,
  type ProjectWorkspaceSurface,
} from '../src/modules/projects/projectWorkspaceNavigation.ts';

function iconName(icon: { displayName?: string }) {
  return icon.displayName ?? '';
}

function permissionCheck(granted: string[]) {
  const permissions = new Set(granted);
  return (required?: string[]) => !required?.length || required.some((permission) => permissions.has(permission));
}

function routeMap(granted: string[], surface: ProjectWorkspaceSurface = 'workspace') {
  return Object.fromEntries(
    visibleProjectWorkspaceGroups(permissionCheck(granted), surface)
      .map((group) => [group.key, projectWorkspaceGroupPath('project-1', group)]),
  );
}

test('project workspace groups open their canonical entrypoints for an admin', () => {
  const allPermissions = [
    'project.view', 'pipeline.view', 'planning.view', 'boq.view',
    'contract.view', 'procurement.view', 'warehouse.view',
    'operation.view', 'quality.view', 'acceptance.view', 'rework.view',
    'handover.view', 'accounting_handoff.view', 'project_finance.view',
    'audit.view',
  ];

  assert.deepEqual(routeMap(allPermissions), {
    overview: '/projects/project-1',
    discussion: '/projects/project-1/discussion',
    planning: '/projects/project-1/preparation',
    materials: '/projects/project-1/boq',
    quality: '/projects/project-1/operations',
    finance: '/projects/project-1/handover',
    governance: '/projects/project-1?tab=members',
  });

  assert.deepEqual(routeMap(allPermissions, 'explorer'), {
    overview: '/projects/project-1',
    planning: '/projects/project-1/preparation',
    materials: '/projects/project-1/boq',
    quality: '/projects/project-1/operations',
    finance: '/projects/project-1/handover',
    governance: '/projects/project-1?tab=members',
  });
});

test('project workspace exposes the lean accounting handoff permission without legacy finance rights', () => {
  assert.deepEqual(routeMap([
    'project.view', 'accounting_handoff.view', 'audit.view',
  ]), {
    overview: '/projects/project-1',
    discussion: '/projects/project-1/discussion',
    planning: '/projects/project-1/preparation',
    finance: '/projects/project-1/accounting',
    governance: '/projects/project-1?tab=members',
  });
});

test('project workspace groups fall back to the first child the user may view', () => {
  assert.deepEqual(routeMap([
    'project.view', 'boq.view', 'warehouse.view', 'acceptance.view',
    'project_finance.view', 'audit.view',
  ]), {
    overview: '/projects/project-1',
    discussion: '/projects/project-1/discussion',
    planning: '/projects/project-1/preparation',
    materials: '/projects/project-1/boq',
    quality: '/projects/project-1/acceptance',
    finance: '/projects/project-1/accounting',
    governance: '/projects/project-1?tab=members',
  });
});

test('accounting reconciliation observer sees the business tree but not project audit', () => {
  const groups = visibleProjectWorkspaceGroups(permissionCheck([
    'project.reconciliation.view', 'project.view', 'pipeline.view', 'planning.view',
    'boq.view', 'contract.view', 'procurement.view', 'warehouse.view',
    'operation.view', 'quality.view', 'acceptance.view', 'rework.view',
    'handover.view', 'accounting_handoff.view', 'project_finance.view', 'audit.view',
  ]));
  const children = groups.flatMap((group) => group.children.map((child) => child.label));

  assert.ok(groups.some((group) => group.key === 'planning'));
  assert.ok(groups.some((group) => group.key === 'discussion'));
  assert.ok(groups.some((group) => group.key === 'materials'));
  assert.ok(groups.some((group) => group.key === 'quality'));
  assert.ok(groups.some((group) => group.key === 'finance'));
  assert.ok(children.includes('Thành viên'));
  assert.ok(children.includes('Hồ sơ'));
  assert.ok(!children.includes('Nhật ký'));
});

test('project workspace navigation assigns a dedicated semantic icon to every item', () => {
  const configuredIcons = Object.fromEntries(PROJECT_WORKSPACE_GROUPS.flatMap((group) => [
    [group.label, iconName(group.icon)],
    ...group.children.map((child) => [child.label, iconName(child.icon)]),
  ]));

  assert.deepEqual(configuredIcons, {
    'Tổng quan': 'LayoutDashboard',
    'Trao đổi': 'MessageSquare',
    'Kế hoạch': 'CalendarRange',
    'Hồ sơ chuẩn bị': 'FileCheck2',
    'Kế hoạch công việc': 'ListTodo',
    'Quy trình': 'Workflow',
    'Thương mại & Vật tư': 'Boxes',
    BOQ: 'TableProperties',
    // Lucide 0.468 exposes FileSignature as the FilePenLine icon alias.
    'Hợp đồng': 'FilePenLine',
    'Mua sắm': 'ShoppingCart',
    'Kho vật tư': 'Warehouse',
    'Thi công & Chất lượng': 'HardHat',
    'Thi công': 'Construction',
    'Nghiệm thu': 'BadgeCheck',
    'Bàn giao & Kế toán': 'Handshake',
    'Bàn giao': 'KeyRound',
    'Bàn giao kế toán': 'ReceiptText',
    'Quản trị công trình': 'Settings2',
    'Thành viên': 'Users',
    'Hồ sơ': 'Files',
    'Nhật ký': 'ScrollText',
  });
  assert.equal(new Set(Object.values(configuredIcons)).size, Object.keys(configuredIcons).length);
});

test('project workspace sidebar renders configured icons instead of generic folders', async () => {
  const source = await readFile(new URL('../src/modules/projects/ProjectWorkspaceLayout.tsx', import.meta.url), 'utf8');
  const projectSidebar = source.slice(source.indexOf('function ProjectNavigationSidebar'), source.indexOf('function WorkflowSidebar'));
  const workspaceNode = source.slice(source.indexOf('function WorkspaceGroupNode'), source.indexOf('function isProjectWorkItemDetailPath'));

  assert.doesNotMatch(projectSidebar, /<Building2 aria-hidden="true"/);
  assert.doesNotMatch(projectSidebar, /\bprojectName\b/);
  assert.doesNotMatch(projectSidebar, /\bprojectCode\b/);
  assert.doesNotMatch(projectSidebar, /<Folder(?:Open)?\b/);
  assert.match(workspaceNode, /const GroupIcon = group\.icon/);
  assert.match(workspaceNode, /<GroupIcon aria-hidden="true"/);
  assert.match(workspaceNode, /const ChildIcon = child\.icon/);
  assert.match(workspaceNode, /<ChildIcon aria-hidden="true"/);
  assert.doesNotMatch(workspaceNode, /<Folder\b/);
  assert.match(workspaceNode, /\{group\.children\.length \? <ChevronDown className="h-4 w-4 shrink-0"\s*\/> : null\}/);
  assert.match(workspaceNode, /ml-8 flex w-\[calc\(100%-2rem\)\]/);
  assert.doesNotMatch(workspaceNode, /border-l border-slate-200/);
});
