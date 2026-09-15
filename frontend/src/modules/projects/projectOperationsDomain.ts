import type { ProjectOperationsTimelineNode, WorkItem } from '../../lib/types/domain';

export function currentTimelineNode(nodes: ProjectOperationsTimelineNode[]) {
  return nodes.find((node) => node.status === 'in_progress')
    ?? nodes.find((node) => node.status === 'blocked' || node.status === 'waiting_approval')
    ?? nodes.find((node) => !['completed', 'cancelled', 'skipped'].includes(node.status))
    ?? nodes[nodes.length - 1];
}

export function isOpenProjectWorkItem(item: WorkItem) {
  return item.status !== 'completed' && item.status !== 'cancelled';
}

export function selectProjectPriorityWork(priorityWork: WorkItem[], blockers: WorkItem[]) {
  return blockers[0]
    ?? priorityWork.find((item) => item.status === 'waiting_approval' || item.status === 'waiting_review')
    ?? priorityWork[0];
}
