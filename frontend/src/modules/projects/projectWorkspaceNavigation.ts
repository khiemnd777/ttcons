export type ProjectWorkspaceChild = {
  label: string;
  to: string;
  permissions?: string[];
};

export type ProjectWorkspaceGroup = {
  key: string;
  label: string;
  explorerLabel?: string;
  description: string;
  to: string;
  permissions?: string[];
  children: ProjectWorkspaceChild[];
};

export const PROJECT_WORKSPACE_GROUPS: ProjectWorkspaceGroup[] = [
  {
    key: 'overview',
    label: 'Tổng quan',
    description: 'Tình trạng, việc cần xử lý, blocker',
    to: '',
    permissions: ['project.view'],
    children: [],
  },
  {
    key: 'planning',
    label: 'Kế hoạch & Quy trình',
    explorerLabel: 'Kế hoạch & Workflow',
    description: 'Pipeline, planning, BOQ / dự toán',
    to: 'planning',
    permissions: ['planning.view', 'pipeline.view', 'boq.view'],
    children: [
      { label: 'Quy trình', to: 'pipeline', permissions: ['pipeline.view'] },
      { label: 'Kế hoạch', to: 'planning', permissions: ['planning.view', 'pipeline.view'] },
      { label: 'Dự toán khối lượng', to: 'boq', permissions: ['boq.view'] },
    ],
  },
  {
    key: 'materials',
    label: 'Thương mại & Vật tư',
    description: 'Hợp đồng, mua sắm, kho vật tư, cân đối vật tư',
    to: 'procurement',
    permissions: ['procurement.view', 'warehouse.view', 'contract.view'],
    children: [
      { label: 'Hợp đồng', to: 'contracts', permissions: ['contract.view'] },
      { label: 'Mua sắm', to: 'procurement', permissions: ['procurement.view'] },
      { label: 'Kho vật tư', to: 'warehouse', permissions: ['warehouse.view'] },
    ],
  },
  {
    key: 'quality',
    label: 'Thi công & Chất lượng',
    description: 'Nhật ký, kiểm tra, cần xử lý, nghiệm thu',
    to: 'operations',
    permissions: ['operation.view', 'quality.view', 'acceptance.view', 'rework.view'],
    children: [
      { label: 'Thi công', to: 'operations', permissions: ['operation.view', 'quality.view', 'rework.view', 'replanning.view'] },
      { label: 'Nghiệm thu', to: 'acceptance', permissions: ['acceptance.view'] },
    ],
  },
  {
    key: 'finance',
    label: 'Bàn giao & Tài chính',
    description: 'Bàn giao, kế toán, công nợ',
    to: 'handover',
    permissions: ['handover.view', 'accounting.view', 'accounting.payable_view', 'accounting.receivable_view'],
    children: [
      { label: 'Bàn giao', to: 'handover', permissions: ['handover.view'] },
      { label: 'Kế toán', to: 'accounting', permissions: ['accounting.view'] },
      { label: 'Công nợ', to: 'debt', permissions: ['accounting.payable_view', 'accounting.receivable_view'] },
    ],
  },
  {
    key: 'governance',
    label: 'Quản trị công trình',
    description: 'Thành viên, hồ sơ, audit',
    to: '',
    permissions: ['project.view', 'audit.view'],
    children: [
      { label: 'Thành viên', to: '?tab=members', permissions: ['project.view'] },
      { label: 'Hồ sơ', to: '?tab=files', permissions: ['project.view'] },
      { label: 'Nhật ký', to: 'audit', permissions: ['audit.view'] },
    ],
  },
];

export function visibleProjectWorkspaceGroups(
  hasAnyPermission: (permissions?: string[]) => boolean,
): ProjectWorkspaceGroup[] {
  return PROJECT_WORKSPACE_GROUPS
    .map((group) => ({
      ...group,
      children: group.children.filter((child) => hasAnyPermission(child.permissions)),
    }))
    .filter((group) => hasAnyPermission(group.permissions) || group.children.length > 0);
}

export function projectWorkspacePath(projectId: string, target: string) {
  if (!target) return `/projects/${projectId}`;
  if (target.startsWith('?')) return `/projects/${projectId}${target}`;
  return `/projects/${projectId}/${target}`;
}

export function projectWorkspaceGroupPath(projectId: string, group: ProjectWorkspaceGroup) {
  return projectWorkspacePath(projectId, group.children[0]?.to ?? group.to);
}
