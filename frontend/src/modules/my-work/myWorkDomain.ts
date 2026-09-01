import type { AccountingWorkStatus, MyWorkflowAction, WorkItem } from '../../lib/types/domain';
import { workItemIsUpcoming } from '../work-items/workItemPresentation';

export type ApprovalLaneKey = 'work' | 'node' | 'report';
export type WorkScopeKey = 'responsible' | 'mine' | 'department' | 'watching' | 'all';
export type SummaryBucket = 'all' | 'need_action' | 'in_progress' | 'overdue' | 'blocked' | 'waiting_decision' | 'upcoming' | 'completed' | 'done_today';

export function workReportDateText(value: string) {
  const parts = value.slice(0, 10).split('-');
  return parts.length === 3 ? parts.reverse().join('/') : value;
}

export function buildAccountingWorkPath(filters: { project: string; status: AccountingWorkStatus; search: string }, cursor: string) {
  const params = new URLSearchParams({ status: filters.status, limit: '30' });
  if (filters.project) params.set('project_id', filters.project);
  if (filters.search) params.set('search', filters.search);
  if (cursor) params.set('cursor', cursor);
  return `/me/accounting-handoffs?${params.toString()}`;
}

export function buildWorkPath(filters: {
  queue: WorkScopeKey;
  bucket: SummaryBucket;
  project: string;
  status: string;
  deadline: string;
  workType: string;
  priority: string;
  search: string;
  sort: string;
}, cursor: string) {
  const params = new URLSearchParams({ queue: filters.queue, sort: filters.sort, limit: '30' });
  params.set('bucket', filters.bucket);
  if (filters.project) params.set('project_id', filters.project);
  if (filters.status) params.set('status', filters.status);
  if (filters.deadline) params.set('deadline', filters.deadline);
  if (filters.workType) params.set('work_type', filters.workType);
  if (filters.priority) params.set('priority', filters.priority);
  if (filters.search) params.set('search', filters.search);
  if (cursor) params.set('cursor', cursor);
  return `/me/work-items?${params.toString()}`;
}

export function buildWorkScopeSummaryPath(project: string) {
  return project
    ? `/me/work-items/scope-summary?project_id=${encodeURIComponent(project)}`
    : '/me/work-items/scope-summary';
}

export function buildWorkflowActionsPath(filters: { queue: WorkScopeKey; project: string; search: string }) {
  const params = new URLSearchParams({ queue: filters.queue });
  if (filters.project) params.set('project_id', filters.project);
  if (filters.search) params.set('search', filters.search);
  return `/me/workflow-actions?${params.toString()}`;
}

export function buildProjectStartupActionsPath(filters: { project: string; search: string }) {
  const params = new URLSearchParams();
  if (filters.project) params.set('project_id', filters.project);
  if (filters.search) params.set('search', filters.search);
  const query = params.toString();
  return `/me/project-startup-actions${query ? `?${query}` : ''}`;
}

export function workflowActionSuccessMessage(action: MyWorkflowAction) {
  if (action.terminal) return 'Đã hoàn tất quy trình';
  const nextType = action.next_run_node_type ?? '';
  if (['construction_execution', 'construction'].includes(nextType)) return 'Đã bắt đầu giai đoạn Thi công';
  if (['handover_finance', 'handover'].includes(nextType)) return 'Đã mở giai đoạn Bàn giao';
  return `Đã chuyển sang ${action.next_run_node_name || 'giai đoạn tiếp theo'}`;
}

export function dedupeWorkItems(rows: WorkItem[]) {
  return Array.from(new Map(rows.map((item) => [item.id, item])).values());
}

export function validValue<T extends string>(value: string | null, values: readonly T[], fallback: T): T {
  return value && values.includes(value as T) ? value as T : fallback;
}

export function firstPopulatedApprovalLane(workCount: number, nodeCount: number, reportCount: number, reportVisible: boolean): ApprovalLaneKey {
  if (workCount > 0) return 'work';
  if (nodeCount > 0) return 'node';
  if (reportVisible && reportCount > 0) return 'report';
  return 'work';
}


export function nextActionCopy(item: WorkItem) {
  if (workItemIsUpcoming(item)) return 'Chờ quy trình chuyển đến node này.';
  if (item.status === 'waiting_approval' || item.status === 'waiting_review') return (item.capabilities?.can_decide ?? item.can_decide) ? 'Rà soát hồ sơ và đưa ra quyết định.' : 'Theo dõi phản hồi từ người có thẩm quyền.';
  if (item.status === 'waiting_evidence') return 'Bổ sung đủ minh chứng trước khi hoàn tất.';
  if (item.status === 'completed') return 'Công việc đã hoàn tất. Có thể xem lại hồ sơ và lịch sử.';
  return 'Mở không gian xử lý để thực hiện bước tiếp theo.';
}
