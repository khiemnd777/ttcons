import { postFormWithProgress } from '../../lib/api/client';
import type { EvidenceRecord } from '../../lib/types/domain';
import { constructionEvidenceFormData, type EvidenceUploadDraft } from './EvidenceCapture';
import { uploadEvidenceDraft, uploadEvidenceDraftBatch } from './evidenceUploadSession';

export function uploadWorkItemEvidenceDraft(workItemId: string, draft: EvidenceUploadDraft) {
  const path = `/work-items/${workItemId}/evidence/upload`;
  return uploadEvidenceDraft(draft, path, (progress, currentDraft) => sendEvidenceDraft(workItemId, currentDraft, progress));
}

export function uploadWorkItemEvidenceDrafts(workItemId: string, drafts: EvidenceUploadDraft[]) {
  return uploadEvidenceDraftBatch(drafts, `/work-items/${workItemId}/evidence/upload`, (draft, progress) => sendEvidenceDraft(workItemId, draft, progress));
}

async function sendEvidenceDraft(workItemId: string, draft: EvidenceUploadDraft, progress: (percent: number) => void) {
  if (!workItemId) throw new Error('Chọn công việc liên quan trước khi tải minh chứng.');
  const record = await postFormWithProgress<EvidenceRecord>(`/work-items/${workItemId}/evidence/upload`, constructionEvidenceFormData(draft, workItemId), ({ percent }) => progress(percent));
  if (!record.id) throw new Error('Máy chủ chưa trả mã minh chứng. Vui lòng kiểm tra lại hồ sơ.');
  return record;
}
