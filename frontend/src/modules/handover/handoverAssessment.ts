import type { ChecklistItem, HandoverDetail } from './handoverTypes';

export function canAssessHandover(status: string, canVerify: boolean) {
  return canVerify && !['handed_over', 'cancelled', 'deleted'].includes(status);
}

export function physicalResultLabel(result?: ChecklistItem['physical_result']) {
  return result === 'pass' ? 'Đạt' : result === 'fail' ? 'Không đạt' : 'Chưa đánh giá';
}

export function handoverConclusionLabel(value?: HandoverDetail['handover_conclusion']) {
  return value === 'eligible' ? 'Đủ điều kiện bàn giao' : value === 'not_eligible' ? 'Chưa đủ điều kiện bàn giao' : 'Chưa kết luận';
}

// Observation updates never carry verification, source-link or readiness fields.
export function checklistAssessmentPayload(values: Record<string, unknown>) {
  return {
    requirement: String(values.requirement ?? '').trim(),
    physical_result: String(values.physical_result ?? 'not_assessed'),
    defect: String(values.defect ?? '').trim(),
    due_date: String(values.due_date ?? '').trim(),
  };
}

export function handoverAssessmentPayload(values: Record<string, unknown>) {
  return { handover_stage: String(values.handover_stage ?? '').trim(), handover_conclusion: String(values.handover_conclusion ?? '') };
}

export function handoverDueDateIssue(value: unknown) {
  const date = String(value ?? '').trim();
  if (!date) return undefined;
  const parsed = new Date(`${date}T00:00:00Z`);
  if (!/^\d{4}-\d{2}-\d{2}$/.test(date) || Number.isNaN(parsed.getTime()) || parsed.toISOString().slice(0, 10) !== date) return 'Hạn xử lý tồn tại cần ngày hợp lệ';
  return undefined;
}
