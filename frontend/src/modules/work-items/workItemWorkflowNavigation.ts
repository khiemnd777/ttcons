import type { WorkItem } from '../../lib/types/domain';

export type WorkItemWorkflowNavigation = {
  stageName: string;
  position: number;
  total: number;
  previous?: WorkItem;
  next?: WorkItem;
  returnTo?: string;
  progressPath?: string;
};

export function buildWorkItemWorkflowNavigation(
  items: WorkItem[],
  currentWorkItemId: string,
): Omit<WorkItemWorkflowNavigation, 'returnTo' | 'progressPath'> | undefined {
  const current = items.find((item) => item.id === currentWorkItemId);
  const stageId = String(current?.run_node_id ?? '').trim();
  if (!current || !stageId) return undefined;

  const stageItems = items
    .filter((item) => (
      item.run_node_id === stageId
      && (!current.pipeline_run_id || item.pipeline_run_id === current.pipeline_run_id)
    ))
    .sort((a, b) => workflowStepOrder(a) - workflowStepOrder(b) || String(a.id).localeCompare(String(b.id)));
  const currentIndex = stageItems.findIndex((item) => item.id === current.id);
  if (currentIndex < 0) return undefined;

  return {
    stageName: String(current.run_node_name ?? '').trim() || 'Chưa xác định',
    position: currentIndex + 1,
    total: stageItems.length,
    previous: currentIndex > 0 ? stageItems[currentIndex - 1] : undefined,
    next: currentIndex < stageItems.length - 1 ? stageItems[currentIndex + 1] : undefined,
  };
}

function workflowStepOrder(item: WorkItem) {
  const value = Number(item.run_step_order_index);
  return Number.isFinite(value) && value > 0 ? value : Number.MAX_SAFE_INTEGER;
}
