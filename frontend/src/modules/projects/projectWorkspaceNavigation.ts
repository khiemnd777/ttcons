import {
  BadgeCheck,
  Boxes,
  CalendarRange,
  Construction,
  FileCheck2,
  FileSignature,
  Files,
  Handshake,
  HardHat,
  KeyRound,
  LayoutDashboard,
  ListTodo,
  MessageSquare,
  ReceiptText,
  ScrollText,
  Settings2,
  ShoppingCart,
  TableProperties,
  Users,
  Warehouse,
  Workflow,
  type LucideIcon,
} from 'lucide-react';

export type ProjectWorkspaceChild = {
  label: string;
  to: string;
  icon: LucideIcon;
  permissions?: string[];
  excludePermissions?: string[];
};

export type ProjectWorkspaceSurface = 'workspace' | 'explorer';

export type ProjectWorkspaceGroup = {
  key: string;
  label: string;
  explorerLabel?: string;
  description: string;
  to: string;
  icon: LucideIcon;
  permissions?: string[];
  excludePermissions?: string[];
  surfaces?: ProjectWorkspaceSurface[];
  badge?: 'discussion_unread';
  children: ProjectWorkspaceChild[];
};

export const PROJECT_WORKSPACE_GROUPS: ProjectWorkspaceGroup[] = [
  {
    key: 'overview',
    label: 'Tổng quan',
    description: 'Tình trạng, việc cần xử lý, blocker',
    to: '',
    icon: LayoutDashboard,
    permissions: ['project.view'],
    children: [],
  },
  {
    key: 'discussion',
    label: 'Trao đổi',
    description: 'Phối hợp, quyết định và theo dõi vấn đề công trình',
    to: 'discussion',
    icon: MessageSquare,
    permissions: ['project.view'],
    surfaces: ['workspace'],
    badge: 'discussion_unread',
    children: [],
  },
  {
    key: 'planning',
    label: 'Kế hoạch',
    explorerLabel: 'Kế hoạch',
    description: 'Hồ sơ chuẩn bị, kế hoạch công việc và quy trình',
    to: 'planning',
    icon: CalendarRange,
    permissions: ['project.view', 'planning.view', 'pipeline.view'],
    children: [
      { label: 'Hồ sơ chuẩn bị', to: 'preparation', icon: FileCheck2, permissions: ['project.view'] },
      { label: 'Kế hoạch công việc', to: 'planning', icon: ListTodo, permissions: ['planning.view', 'pipeline.view'] },
      { label: 'Quy trình', to: 'pipeline', icon: Workflow, permissions: ['pipeline.view'] },
    ],
  },
  {
    key: 'materials',
    label: 'Thương mại & Vật tư',
    description: 'Hợp đồng, mua sắm, kho vật tư, cân đối vật tư',
    to: 'procurement',
    icon: Boxes,
    permissions: ['procurement.view', 'warehouse.view', 'contract.view'],
    children: [
      { label: 'BOQ', to: 'boq', icon: TableProperties, permissions: ['boq.view'] },
      { label: 'Hợp đồng', to: 'contracts', icon: FileSignature, permissions: ['contract.view'] },
      { label: 'Mua sắm', to: 'procurement', icon: ShoppingCart, permissions: ['procurement.view'] },
      { label: 'Kho vật tư', to: 'warehouse', icon: Warehouse, permissions: ['warehouse.view'] },
    ],
  },
  {
    key: 'quality',
    label: 'Thi công & Chất lượng',
    description: 'Nhật ký, kiểm tra, cần xử lý, nghiệm thu',
    to: 'operations',
    icon: HardHat,
    permissions: ['operation.view', 'quality.view', 'acceptance.view', 'rework.view'],
    children: [
      { label: 'Thi công', to: 'operations', icon: Construction, permissions: ['operation.view', 'quality.view', 'rework.view', 'replanning.view'] },
      { label: 'Nghiệm thu', to: 'acceptance', icon: BadgeCheck, permissions: ['acceptance.view'] },
    ],
  },
  {
    key: 'finance',
    label: 'Bàn giao & Kế toán',
    description: 'Bàn giao công trình và hồ sơ nghiệp vụ cho Kế toán',
    to: 'handover',
    icon: Handshake,
    permissions: ['handover.view', 'accounting_handoff.view'],
    children: [
      { label: 'Bàn giao', to: 'handover', icon: KeyRound, permissions: ['handover.view'] },
      { label: 'Bàn giao kế toán', to: 'accounting', icon: ReceiptText, permissions: ['accounting_handoff.view'] },
    ],
  },
  {
    key: 'governance',
    label: 'Quản trị công trình',
    description: 'Thành viên và hồ sơ công trình',
    to: '',
    icon: Settings2,
    permissions: ['project.view', 'audit.view'],
    children: [
      { label: 'Thành viên', to: '?tab=members', icon: Users, permissions: ['project.view'] },
      { label: 'Hồ sơ', to: '?tab=files', icon: Files, permissions: ['project.view'] },
      { label: 'Nhật ký', to: 'audit', icon: ScrollText, permissions: ['audit.view'], excludePermissions: ['project.reconciliation.view'] },
    ],
  },
];

export function visibleProjectWorkspaceGroups(
  hasAnyPermission: (permissions?: string[]) => boolean,
  surface: ProjectWorkspaceSurface = 'workspace',
): ProjectWorkspaceGroup[] {
  return PROJECT_WORKSPACE_GROUPS
    .map((group) => ({
      ...group,
      children: group.children.filter((child) => isWorkspaceEntryVisible(child, hasAnyPermission)),
    }))
    .filter((group) => (!group.surfaces || group.surfaces.includes(surface))
      && (isWorkspaceEntryVisible(group, hasAnyPermission) || group.children.length > 0));
}

function isWorkspaceEntryVisible(
  entry: { permissions?: string[]; excludePermissions?: string[] },
  hasAnyPermission: (permissions?: string[]) => boolean,
) {
  return hasAnyPermission(entry.permissions)
    && !(entry.excludePermissions?.length && hasAnyPermission(entry.excludePermissions));
}

export function projectWorkspacePath(projectId: string, target: string) {
  if (!target) return `/projects/${projectId}`;
  if (target.startsWith('?')) return `/projects/${projectId}${target}`;
  return `/projects/${projectId}/${target}`;
}

export function projectWorkspaceGroupPath(projectId: string, group: ProjectWorkspaceGroup) {
  return projectWorkspacePath(projectId, group.children[0]?.to ?? group.to);
}
