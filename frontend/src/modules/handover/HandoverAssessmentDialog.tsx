import { useEffect, useMemo, useState } from 'react';
import { Dialog } from '../../components/Dialog';
import { SimpleForm, type FieldDef, type FormValues } from '../_shared/SimpleForm';
import type { ChecklistItem, HandoverDetail } from './handoverTypes';
import { handoverDueDateIssue } from './handoverAssessment';

export type HandoverAssessmentAction = { type: 'assessment'; item: ChecklistItem } | { type: 'header-assessment' } | { type: 'add-item' };

function maxTextLength(limit: number, label: string) {
  return (value: unknown) => Array.from(String(value ?? '').trim()).length > limit ? `${label} tối đa ${limit} ký tự` : undefined;
}

export function HandoverAssessmentDialog({ action, detail, pending, onClose, onSave }: { action: HandoverAssessmentAction | null; detail: HandoverDetail | null; pending: boolean; onClose: () => void; onSave: (values: FormValues) => Promise<unknown> }) {
  const [saveError, setSaveError] = useState('');
  useEffect(() => { setSaveError(''); }, [action]);
  const fields = useMemo<FieldDef[]>(() => {
    if (action?.type === 'header-assessment') return [
      { name: 'handover_stage', label: 'Giai đoạn bàn giao', defaultValue: detail?.handover_stage ?? '', validate: maxTextLength(160, 'Giai đoạn bàn giao') },
      { name: 'handover_conclusion', label: 'Kết luận bàn giao', type: 'select', placeholder: 'Chưa kết luận', defaultValue: detail?.handover_conclusion ?? '', options: [{ value: 'eligible', label: 'Đủ điều kiện bàn giao' }, { value: 'not_eligible', label: 'Chưa đủ điều kiện bàn giao' }], helperText: 'Đánh giá thực tế của người lập hồ sơ. Readiness được kiểm tra riêng.' },
    ];
    const item = action?.type === 'assessment' ? action.item : null;
    return [
      ...(action?.type === 'add-item' ? [{ name: 'title', label: 'Hạng mục bàn giao', required: true, validate: maxTextLength(300, 'Hạng mục bàn giao') } satisfies FieldDef] : []),
      { name: 'requirement', label: 'Yêu cầu bàn giao', type: 'textarea', defaultValue: item?.requirement ?? '', validate: maxTextLength(1000, 'Yêu cầu bàn giao') },
      { name: 'physical_result', label: 'Kết quả kiểm tra thực tế', type: 'select', required: true, defaultValue: item?.physical_result ?? 'not_assessed', options: [{ value: 'not_assessed', label: 'Chưa đánh giá' }, { value: 'pass', label: 'Đạt' }, { value: 'fail', label: 'Không đạt' }] },
      { name: 'defect', label: 'Tồn tại cần xử lý', type: 'textarea', defaultValue: item?.defect ?? '', validate: maxTextLength(1000, 'Tồn tại cần xử lý') },
      { name: 'due_date', label: 'Hạn xử lý tồn tại', type: 'date', defaultValue: item?.due_date?.slice(0, 10) ?? '', validate: handoverDueDateIssue },
    ];
    // Capture values when the editor opens; background query refresh must retain drafts.
  }, [action]);
  return <Dialog open={Boolean(action)} title={action?.type === 'add-item' ? 'Thêm hạng mục bàn giao' : action?.type === 'header-assessment' ? 'Cập nhật đánh giá' : 'Đánh giá hạng mục bàn giao'} description={action?.type === 'assessment' ? action.item.title : undefined} onClose={() => { if (!pending) onClose(); }} size="lg">
    {saveError ? <div role="alert" className="mb-4 rounded-md border border-red-200 bg-red-50 p-3 text-sm text-red-700">{saveError} Kiểm tra dữ liệu rồi chọn Lưu để thử lại.</div> : null}
    {action ? <SimpleForm fields={fields} layout="two-column" submitLabel="Lưu" onCancel={() => { if (!pending) onClose(); }} onSubmit={async (values) => {
      setSaveError('');
      try { await onSave(values); } catch (error) { setSaveError(error instanceof Error ? error.message : 'Không lưu được đánh giá bàn giao.'); }
    }} /> : null}
  </Dialog>;
}
