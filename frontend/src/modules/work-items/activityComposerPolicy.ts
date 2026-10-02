import type { ActivityPayload } from '../../lib/types/domain';

export function companyDiscussionPayload(payload: ActivityPayload, commentOnly: boolean): ActivityPayload {
  if (!commentOnly) return payload;
  if (payload.scope_type && !['project', 'work_item'].includes(payload.scope_type)) {
    throw new Error('Chỉ trao đổi trong công trình hoặc đầu việc.');
  }
  return { ...payload, activity_type: 'comment', visibility: 'project', attachments: [], metadata: {} };
}
