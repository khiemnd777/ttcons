import type { WorkItem } from '../../lib/types/domain';

function positiveIndex(value: unknown) {
  const parsed = Number(value);
  return Number.isFinite(parsed) && parsed > 0 ? parsed : 0;
}

export function workItemWorkflowContext(item: WorkItem) {
  const project = [item.project_code, item.project_name].filter(Boolean).join(' · ') || 'Dự án chưa xác định';
  const nodeNumber = positiveIndex(item.run_node_order_index);
  const stepNumber = positiveIndex(item.run_step_order_index);
  const node = item.run_node_name
    ? `Node${nodeNumber ? ` ${nodeNumber}` : ''}: ${item.run_node_name}`
    : 'Node: Ngoài workflow';
  const step = item.run_step_name
    ? `Bước${stepNumber ? ` ${stepNumber}` : ''}: ${item.run_step_name}`
    : `Bước: ${item.title || item.work_type || 'Công việc ngoài workflow'}`;

  return { project, node, step, path: `${project} › ${node} › ${step}` };
}

export function workItemIsUpcoming(item: WorkItem) {
  return item.readiness === 'upcoming';
}
