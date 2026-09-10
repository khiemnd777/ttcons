import type { WorkItem } from '../../lib/types/domain';

export type DiscussionWorkDetailLoadState = 'loading' | 'ready' | 'error';

export function discussionWorkDetailLoadState(
  query: {
    isFetchedAfterMount: boolean;
    isFetching: boolean;
    isError: boolean;
    projectId?: string;
  },
  expectedProjectId: string,
): DiscussionWorkDetailLoadState {
  if (query.isFetching) return 'loading';
  if (query.isError) return 'error';
  if (!query.isFetchedAfterMount) return 'loading';
  return query.projectId === expectedProjectId ? 'ready' : 'error';
}

export function discussionWorkSurface(item: WorkItem): 'design' | 'form' | 'evidence' | 'procurement' | 'external' {
  const target = item.execution_target;
  // Canonical domain workspaces remain authoritative until their editor is integrated.
  if (!target) return 'external';
  if (item.run_node_business_code === 'design' || item.run_step_business_code?.startsWith('design.') || item.domain_action_key === 'source.design_documents_ready') return 'design';
  if (['procurement.require_approved', 'procurement.generate_from_boq'].includes(item.domain_action_key ?? '')) return 'procurement';
  if (!['preparation', 'planning', 'work_item'].includes(target.workspace ?? '')) return 'external';
  if (target.kind === 'evidence') return 'evidence';
  if (['capture', 'mixed'].includes(target.kind)) return 'form';
  return 'external';
}

export function discussionWorkActionLabel(item: WorkItem) {
  if (discussionWorkReadOnly(item)) return 'Xem công việc';
  switch (discussionWorkSurface(item)) {
    case 'design': return 'Cập nhật hồ sơ thiết kế';
    case 'form': return 'Cập nhật hồ sơ';
    case 'evidence': return 'Thêm minh chứng';
    case 'procurement': return 'Xử lý yêu cầu mua';
    default: return item.execution_target?.action_label || 'Xem công việc';
  }
}

export function discussionWorkflowActionLabel(item: WorkItem) {
  const canDecide = (item.capabilities?.can_decide ?? item.can_decide) === true;
  if (canDecide && ['waiting_review', 'waiting_approval'].includes(item.status)) return 'Mở yêu cầu duyệt';
  if (discussionWorkReadOnly(item)) return 'Xem công việc';
  if (item.status === 'blocked') return 'Xử lý vướng mắc';
  return 'Xử lý việc ưu tiên';
}

export function discussionWorkReadOnly(item: WorkItem) {
  return !(item.capabilities?.can_act ?? item.can_act) || ['upcoming', 'read_only', 'history'].includes(item.readiness ?? '') || ['completed', 'cancelled', 'waiting_review', 'waiting_approval'].includes(item.status);
}

export function discussionWorkPath(projectId: string, bucket = 'need_action') {
  return `/me/work-items?${new URLSearchParams({ project_id: projectId, queue: 'responsible', bucket, limit: '5', sort: 'priority' })}`;
}
