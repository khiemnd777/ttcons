/** Activity scope IDs identify source records, never a page, title or stock document. */
export const activityTopicTypes = {
  work_item: { label: 'Đầu việc', endpoint: '/work-items', list: false },
  boq_item: { label: 'Dòng BOQ', endpoint: '/boq-items', list: false },
  procurement_request: { label: 'Yêu cầu mua', endpoint: '/procurement-requests', list: false },
  stock_movement: { label: 'Luân chuyển kho', endpoint: '/stock/movements', list: true },
  acceptance: { label: 'Nghiệm thu', endpoint: '/acceptances', list: false },
  rework: { label: 'Yêu cầu sửa lỗi', endpoint: 'reworks', list: true },
  handover: { label: 'Bàn giao', endpoint: '/handover', list: false },
} as const;

export type ActivityTopicScope = keyof typeof activityTopicTypes;
export type ActivityTopic = { scope: ActivityTopicScope; id: string; projectId: string; label: string };
export type ActivityTopicSource = {
  scope_type?: string;
  scope_id?: string;
  work_item_id?: string;
  target_label?: string;
  work_item_title?: string;
  label?: string;
};

const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

export function activityTopic(source: ActivityTopicSource, projectId: string): ActivityTopic | null {
  const scope = source.scope_type || (source.work_item_id ? 'work_item' : 'project');
  if (!Object.prototype.hasOwnProperty.call(activityTopicTypes, scope) || !uuid.test(projectId)) return null;
  const id = source.scope_id || (scope === 'work_item' ? source.work_item_id : '');
  if (!id || !uuid.test(id)) return null;
  const typedScope = scope as ActivityTopicScope;
  return {
    scope: typedScope, id, projectId,
    label: source.target_label || source.label || (typedScope === 'work_item' ? source.work_item_title : '') || activityTopicTypes[typedScope].label,
  };
}

export function activityTopicPath(topic: ActivityTopic): string {
  const type = activityTopicTypes[topic.scope];
  if (!type.list) return `${type.endpoint}/${encodeURIComponent(topic.id)}`;
  if (topic.scope === 'rework') return `/projects/${encodeURIComponent(topic.projectId)}/reworks`;
  return `${type.endpoint}?project_id=${encodeURIComponent(topic.projectId)}`;
}

export class ActivityTopicUnavailable extends Error {
  constructor() { super('Hồ sơ không còn tồn tại hoặc không còn khả dụng trong công trình này.'); }
}

export function activityTopicRecord(topic: ActivityTopic, payload: unknown): Record<string, any> {
  const matches = (row: unknown): row is Record<string, any> => {
    if (!row || typeof row !== 'object' || Array.isArray(row)) return false;
    const record = row as Record<string, unknown>;
    return String(record.id).toLowerCase() === topic.id.toLowerCase()
      && String(record.project_id).toLowerCase() === topic.projectId.toLowerCase()
      && !['cancelled', 'deleted'].includes(String(record.status))
      && !record.deleted_at;
  };
  const record = activityTopicTypes[topic.scope].list
    ? Array.isArray(payload) ? payload.find(matches) : undefined
    : matches(payload) ? payload : undefined;
  if (!record) throw new ActivityTopicUnavailable();
  return record;
}

export function activityVisibilityLabel(visibility?: string): string | null {
  if (!visibility || visibility === 'project') return null;
  return ({ internal: 'Nội bộ', client: 'Chủ đầu tư' } as Record<string, string>)[visibility] || visibility;
}
