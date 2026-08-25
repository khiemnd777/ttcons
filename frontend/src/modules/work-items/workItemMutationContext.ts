import type { QueryClient } from '@tanstack/react-query';

export type WorkItemMutationTransition = {
  work_item_id?: string;
  work_item_title?: string;
  project_id?: string;
  from_status?: string;
  to_status?: string;
  mutation?: string;
  started?: boolean;
  auto_started?: boolean;
};

export type WorkItemMutationResult = {
  work_item_transition?: WorkItemMutationTransition;
};

export function withWorkItemMutationContext<T extends Record<string, unknown>>(payload: T, workItemId: string): T & {
  work_item_context?: { work_item_id: string; intent: 'start_on_success' };
} {
  const normalizedId = workItemId.trim();
  if (!normalizedId) return payload;
  return {
    ...payload,
    work_item_context: {
      work_item_id: normalizedId,
      intent: 'start_on_success',
    },
  };
}

export function workItemAutoStartedNotice(result: unknown): string {
  const transition = workItemMutationTransition(result);
  if (!transition?.auto_started) return '';
  const title = transition.work_item_title?.trim();
  return title ? ` Đầu việc “${title}” đã chuyển sang Đang làm.` : ' Đầu việc liên quan đã chuyển sang Đang làm.';
}

export function syncWorkItemMutationTransition(queryClient: QueryClient, result: unknown): WorkItemMutationTransition | null {
  const transition = workItemMutationTransition(result);
  if (!transition?.work_item_id) return transition;
  void queryClient.invalidateQueries({ queryKey: ['work-item', transition.work_item_id] });
  void queryClient.invalidateQueries({ queryKey: ['project-operations', transition.project_id] });
  void queryClient.invalidateQueries({ queryKey: ['my-work-items'] });
  void queryClient.invalidateQueries({ queryKey: ['my-workflow-actions'] });
  return transition;
}

function workItemMutationTransition(result: unknown): WorkItemMutationTransition | null {
  if (!result || typeof result !== 'object') return null;
  const transition = (result as WorkItemMutationResult).work_item_transition;
  return transition && typeof transition === 'object' ? transition : null;
}
