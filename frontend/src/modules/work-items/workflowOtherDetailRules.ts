import type { AnyRecord } from '../../lib/types/domain';

export function workflowTopLevelOtherDetailRequired(
  templateCode: string | undefined,
  fieldKey: string,
  answers: AnyRecord,
) {
  return templateCode === 'builtin_owner_client_checklist'
    && fieldKey === 'scope_detail'
    && workflowRuleText(answers.handover_scope) === 'other';
}

export function workflowTopLevelOtherDetailDependentKeys(
  templateCode: string | undefined,
  changedFieldKey: string,
) {
  if (templateCode === 'builtin_owner_client_checklist' && changedFieldKey === 'handover_scope') {
    return ['scope_detail'];
  }
  return [];
}

export function workflowStructuredOtherDetailRequired(
  templateCode: string | undefined,
  listFieldKey: string,
  row: AnyRecord,
  cellKey: string,
) {
  return templateCode === 'builtin_construction_daily_report'
    && listFieldKey === 'material_usage_items'
    && cellKey === 'note'
    && workflowRuleText(row.source) === 'other';
}

function workflowRuleText(value: unknown) {
  if (value === null || value === undefined) return '';
  return String(value).trim();
}
