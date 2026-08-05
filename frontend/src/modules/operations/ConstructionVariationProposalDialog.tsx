import { useEffect, useMemo, useState } from 'react';
import { Loader2 } from 'lucide-react';
import { Dialog } from '../../components/Dialog';
import { useToast } from '../../components/ToastProvider';
import { post } from '../../lib/api/client';
import type { ConstructionActionItem, ConstructionVariationContext, ConstructionWorkspace } from '../../lib/types/domain';
import { FieldBlock, controlClass, errorSummary, requiredErrors, type FormErrors } from './ConstructionFormControls';

type Props = {
  open: boolean;
  projectId: string;
  context?: ConstructionVariationContext;
  workspace?: ConstructionWorkspace;
  sourceWorkItemId?: string;
  sourceAction?: ConstructionActionItem;
  onClose: () => void;
  onSaved: () => Promise<void>;
};

export function ConstructionVariationProposalDialog({ open, projectId, context, workspace, sourceWorkItemId, sourceAction, onClose, onSaved }: Props) {
  const toast = useToast();
  const [values, setValues] = useState(initialValues(sourceWorkItemId, sourceAction));
  const [errors, setErrors] = useState<FormErrors>({});
  const [saving, setSaving] = useState(false);
  const eligibleItems = useMemo(() => (workspace?.work_items ?? []).filter((item) => item.report_eligible), [workspace?.work_items]);

  useEffect(() => {
    if (!open) return;
    setValues(initialValues(sourceWorkItemId, sourceAction));
    setErrors({});
    setSaving(false);
  }, [open, sourceAction, sourceWorkItemId]);

  async function save() {
    const next = requiredErrors(values, [
      { name: 'variation_code', label: 'Mã phát sinh' },
      { name: 'title', label: 'Tên phát sinh' },
      { name: 'reason', label: 'Lý do phát sinh' },
      { name: 'source_work_item_id', label: 'Hạng mục thi công nguồn' },
    ]);
    if (!context?.available) next.contract = 'Cần hợp đồng chủ đầu tư đang hiệu lực';
    if (!context?.can_propose) next.permission = 'Không có quyền đề xuất phát sinh';
    const amount = Number(values.amount_delta || 0);
    const days = Number(values.days_delta || 0);
    if (!Number.isFinite(amount)) next.amount_delta = 'Giá trị ảnh hưởng không hợp lệ';
    if (!Number.isInteger(days)) next.days_delta = 'Số ngày ảnh hưởng phải là số nguyên';
    setErrors(next);
    if (Object.keys(next).length) {
      toast.warn({ title: 'Chưa đủ thông tin', description: errorSummary(next) });
      return;
    }
    setSaving(true);
    try {
      await post(`/projects/${projectId}/construction-variation-proposals`, {
        variation_code: values.variation_code.trim(),
        title: values.title.trim(),
        variation_type: values.variation_type,
        reason: values.reason.trim(),
        amount_delta: amount,
        days_delta: days,
        scope_delta: values.scope_delta.trim(),
        priority: values.priority,
        source_work_item_id: values.source_work_item_id,
        source_construction_action_id: sourceAction?.id ?? '',
        comment: values.comment.trim(),
      });
      toast.success('Đã gửi đề xuất phát sinh cho QS rà soát');
      await onSaved();
    } catch (error) {
      toast.error('Không gửi được đề xuất phát sinh', error);
    } finally {
      setSaving(false);
    }
  }

  const lockedWorkItem = Boolean(sourceWorkItemId || sourceAction?.work_item_id);
  return (
    <Dialog
      open={open}
      title="Đề xuất phát sinh ngoài kế hoạch"
      description="Đề xuất được gửi thẳng tới QS; việc khắc phục kỹ thuật tại hiện trường vẫn tiếp tục độc lập."
      size="xl"
      onClose={onClose}
      footer={<div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end"><button type="button" disabled={saving} className="rounded-md border border-line bg-white px-4 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-50" onClick={onClose}>Hủy</button><button type="button" disabled={saving || !context?.available || !context?.can_propose} className="inline-flex items-center justify-center gap-2 rounded-md bg-amber-800 px-4 py-2 text-sm font-semibold text-white hover:bg-amber-900 disabled:opacity-50" onClick={save}>{saving ? <Loader2 className="h-4 w-4 animate-spin" /> : null}{saving ? 'Đang gửi...' : 'Gửi QS rà soát'}</button></div>}
    >
      <div className="space-y-5">
        <section className="rounded-lg border border-amber-200 bg-amber-50 p-4 text-sm text-amber-950">
          <p className="font-semibold">{context?.contract ? `${context.contract.contract_code} · ${context.contract.title}` : 'Chưa có hợp đồng chủ đầu tư đang hiệu lực'}</p>
          <p className="mt-1 text-xs leading-5 text-amber-800">Luồng xử lý: Đề xuất → QS rà soát → Mua hàng lấy giá → PM/Giám đốc phê duyệt.</p>
          {sourceAction ? <p className="mt-2 rounded-md bg-white/70 px-3 py-2 text-xs"><strong>Nguồn hiện trường:</strong> {sourceAction.title}</p> : null}
          {errors.contract || errors.permission ? <p className="mt-2 text-xs font-semibold text-rose-700">{errors.contract || errors.permission}</p> : null}
        </section>

        <div className="grid gap-4 sm:grid-cols-2">
          <FieldBlock label="Mã phát sinh" required error={errors.variation_code}><input className={controlClass(errors.variation_code)} value={values.variation_code} onChange={(event) => change('variation_code', event.target.value)} /></FieldBlock>
          <FieldBlock label="Mức độ" required><select className={controlClass()} value={values.priority} onChange={(event) => change('priority', event.target.value)}><option value="normal">Không gấp</option><option value="urgent">Gấp</option></select></FieldBlock>
          <FieldBlock className="sm:col-span-2" label="Tên phát sinh" required error={errors.title}><input className={controlClass(errors.title)} value={values.title} onChange={(event) => change('title', event.target.value)} placeholder="Ví dụ: Thay đổi vật liệu hoàn thiện khu A" /></FieldBlock>
          <FieldBlock label="Hạng mục thi công nguồn" required error={errors.source_work_item_id} helper={lockedWorkItem ? 'Được khóa theo nguồn hiện trường.' : undefined}><select disabled={lockedWorkItem} className={controlClass(errors.source_work_item_id, lockedWorkItem)} value={values.source_work_item_id} onChange={(event) => change('source_work_item_id', event.target.value)}><option value="">Chọn hạng mục</option>{eligibleItems.map((item) => <option key={item.id} value={item.id}>{item.title}</option>)}</select></FieldBlock>
          <FieldBlock label="Loại ảnh hưởng" required><select className={controlClass()} value={values.variation_type} onChange={(event) => change('variation_type', event.target.value)}><option value="scope">Phạm vi</option><option value="price">Chi phí</option><option value="time">Tiến độ</option><option value="mixed">Hỗn hợp</option></select></FieldBlock>
          <FieldBlock className="sm:col-span-2" label="Lý do phát sinh" required error={errors.reason}><textarea rows={3} className={controlClass(errors.reason)} value={values.reason} onChange={(event) => change('reason', event.target.value)} /></FieldBlock>
          <FieldBlock className="sm:col-span-2" label="Thay đổi phạm vi"><textarea rows={3} className={controlClass()} value={values.scope_delta} onChange={(event) => change('scope_delta', event.target.value)} placeholder="Mô tả phần việc, vật liệu hoặc thiết kế thay đổi" /></FieldBlock>
          <FieldBlock label="Giá trị ảnh hưởng dự kiến" error={errors.amount_delta}><input type="number" step="any" className={controlClass(errors.amount_delta)} value={values.amount_delta} onChange={(event) => change('amount_delta', event.target.value)} /></FieldBlock>
          <FieldBlock label="Số ngày ảnh hưởng dự kiến" error={errors.days_delta}><input type="number" step="1" className={controlClass(errors.days_delta)} value={values.days_delta} onChange={(event) => change('days_delta', event.target.value)} /></FieldBlock>
          <FieldBlock className="sm:col-span-2" label="Ghi chú gửi QS"><textarea rows={2} className={controlClass()} value={values.comment} onChange={(event) => change('comment', event.target.value)} /></FieldBlock>
        </div>
      </div>
    </Dialog>
  );

  function change(name: keyof typeof values, value: string) {
    setValues((current) => ({ ...current, [name]: value }));
    setErrors((current) => ({ ...current, [name]: '' }));
  }
}

function initialValues(sourceWorkItemId?: string, sourceAction?: ConstructionActionItem) {
  const actionType = sourceAction?.impact_scope === 'timeline' ? 'time' : sourceAction?.impact_scope === 'budget' ? 'price' : sourceAction?.impact_scope === 'design_scope' ? 'scope' : 'mixed';
  return {
    variation_code: `PS-${new Date().toISOString().replace(/\D/g, '').slice(2, 12)}`,
    title: sourceAction?.title ?? '',
    variation_type: actionType,
    reason: sourceAction?.description ?? '',
    amount_delta: '0',
    days_delta: '0',
    scope_delta: '',
    priority: sourceAction?.priority ?? 'normal',
    source_work_item_id: sourceAction?.work_item_id ?? sourceWorkItemId ?? '',
    comment: '',
  };
}
