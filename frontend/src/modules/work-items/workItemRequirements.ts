import type { WorkItem } from '../../lib/types/domain';

export function missingCompletionRequirementLabels(item: WorkItem) {
  return (item.completion_requirements ?? [])
    .filter((requirement) => requirement.required && requirement.status === 'missing')
    .map((requirement) => requirement.label);
}
