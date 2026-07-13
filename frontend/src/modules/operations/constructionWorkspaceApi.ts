import { postForm } from '../../lib/api/client';
import { constructionEvidenceFormData, type ConstructionEvidenceUploadDraft } from './ConstructionEvidenceCapture';

export async function uploadConstructionEvidence(workItemId: string, drafts: ConstructionEvidenceUploadDraft[]) {
	const ids = await uploadConstructionEvidenceItems(workItemId, drafts);
	if (!ids.length) throw new Error('Chưa có ảnh hoặc tệp minh chứng');
	return ids[0];
}

export async function uploadConstructionEvidenceItems(workItemId: string, drafts: ConstructionEvidenceUploadDraft[]) {
	const ids: string[] = [];
	for (const draft of drafts) {
		const evidence = await postForm<Record<string, unknown>>(`/work-items/${workItemId}/evidence/upload`, constructionEvidenceFormData(draft, workItemId));
		const evidenceId = String(evidence.id ?? '');
		if (evidenceId) ids.push(evidenceId);
	}
	return ids;
}
