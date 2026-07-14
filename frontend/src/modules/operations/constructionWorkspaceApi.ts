import { post, postForm } from '../../lib/api/client';
import type { ConstructionActionItem, ConstructionCheck, ConstructionCheckCriterion } from '../../lib/types/domain';
import { constructionEvidenceFormData, type ConstructionEvidenceUploadDraft } from './ConstructionEvidenceCapture';

export type ConstructionCheckCreateInput = {
	work_item_id: string;
	source_daily_report_id?: string;
	template_id?: string;
	title: string;
	location?: string;
	scheduled_at?: string;
	criteria?: ConstructionCheckCriterion[];
	context_evidence_ids?: string[];
};

export type ConstructionActionCreateInput = {
	work_item_id: string;
	source_daily_report_id?: string;
	title: string;
	description?: string;
	priority: 'normal' | 'urgent';
	assigned_user_id: string;
	due_at: string;
	impact_scope: 'none' | 'timeline' | 'budget' | 'design_scope';
	evidence_id: string;
};

export function createConstructionCheck(projectId: string, input: ConstructionCheckCreateInput) {
	return post<ConstructionCheck>(`/projects/${projectId}/construction-checks`, input);
}

export function createConstructionAction(projectId: string, input: ConstructionActionCreateInput) {
	return post<ConstructionActionItem>(`/projects/${projectId}/construction-actions`, input);
}

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
