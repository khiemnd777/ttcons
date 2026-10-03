import { useEffect, useMemo, useRef, useState } from 'react';
import { Link } from 'react-router-dom';
import { Loader2 } from 'lucide-react';
import { Dialog } from '../../components/Dialog';
import { CurrencyInput } from '../../components/CurrencyInput';
import { useToast } from '../../components/ToastProvider';
import { post } from '../../lib/api/client';
import type { ConstructionActionItem, ConstructionVariationContext, ConstructionWorkspace } from '../../lib/types/domain';
import { FieldBlock, controlClass, errorSummary, requiredErrors, type FormErrors } from './ConstructionFormControls';
import { useValidationNavigation } from '../../lib/utils/formValidationNavigation';

type Props = {
  open: boolean;
  projectId: string;
  context?: ConstructionVariationContext;
  contextLoading: boolean;
  contextError: boolean;
  canViewContracts: boolean;
  workspace?: ConstructionWorkspace;
  sourceWorkItemId?: string;
  sourceAction?: ConstructionActionItem;
  onRetryContext: () => void;
  onClose: () => void;
  onSaved: () => Promise<void>;
};

export function ConstructionVariationProposalDialog({ open, projectId, context, contextLoading, contextError, canViewContracts, workspace, sourceWorkItemId, sourceAction, onRetryContext, onClose, onSaved }: Props) {
  const toast = useToast();
  const [values, setValues] = useState(initialValues(sourceWorkItemId, sourceAction));
  const [errors, setErrors] = useState<FormErrors>({});
  const [saving, setSaving] = useState(false);
  const formRef = useRef<HTMLDivElement | null>(null);
  const navigateToValidation = useValidationNavigation(formRef);
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
      navigateToValidation({});
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
  const checkingContext = contextLoading || (!context && !contextError);
  if (checkingContext || contextError || !context?.available || !context.can_propose) {
    const unavailable = Boolean(context && !context.available);
    const missingPermission = Boolean(context?.available && !context.can_propose);
    return (
      <Dialog
        open={open}
        title={checkingContext ? 'Đang kiểm tra điều kiện đề xuất' : contextError ? 'Không tải được điều kiện đề xuất' : missingPermission ? 'Không có quyền đề xuất' : 'Chưa thể đề xuất'}
        description={unavailable ? 'Công trình chưa có hợp đồng chủ đầu tư đang hiệu lực.' : undefined}
        size="sm"
        onClose={onClose}
        footer={<div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end"><button type="button" className="rounded-md border border-line bg-white px-4 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-50" onClick={onClose}>Đóng</button>{contextError ? <button type="button" className="rounded-md bg-blue-600 px-4 py-2 text-sm font-semibold text-white hover:bg-blue-700" onClick={onRetryContext}>Thử lại</button> : canViewContracts && unavailable ? <Link className="inline-flex items-center justify-center rounded-md bg-blue-600 px-4 py-2 text-sm font-semibold text-white hover:bg-blue-700" to={`/projects/${projectId}/contracts`} onClick={onClose}>Mở hợp đồng</Link> : null}</div>}
      >
        {checkingContext ? <div className="flex items-center gap-3 rounded-lg border border-slate-200 bg-slate-50 p-4 text-sm text-slate-600"><Loader2 className="h-5 w-5 animate-spin text-blue-600" /> Đang kiểm tra điều kiện đề xuất…</div> : contextError ? <p className="rounded-lg border border-slate-200 bg-slate-50 p-4 text-sm leading-6 text-slate-600">Không thể xác nhận hợp đồng và quyền đề xuất lúc này. Hãy thử tải lại.</p> : missingPermission ? <p className="rounded-lg border border-slate-200 bg-slate-50 p-4 text-sm leading-6 text-slate-600">Tài khoản hiện tại không có quyền gửi đề xuất phát sinh thương mại.</p> : <div className="rounded-lg border border-slate-200 bg-slate-50 p-4 text-sm leading-6 text-slate-600"><p>Hợp đồng cần được Quản lý dự án/Thương mại hoàn tất trước khi gửi đề xuất.</p><p className="mt-2 font-medium text-slate-700">Báo cáo, kiểm tra và xử lý hiện trường vẫn hoạt động bình thường.</p></div>}
      </Dialog>
    );
  }

  return (
    <Dialog
      open={open}
      title="Đề xuất phát sinh ngoài kế hoạch"
      description="Đề xuất được gửi thẳng tới QS; việc khắc phục kỹ thuật tại hiện trường vẫn tiếp tục độc lập."
      size="xl"
      onClose={onClose}
      footer={<div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end"><button type="button" disabled={saving} className="rounded-md border border-line bg-white px-4 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-50" onClick={onClose}>Hủy</button><button type="button" disabled={saving || !context?.available || !context?.can_propose} className="inline-flex items-center justify-center gap-2 rounded-md bg-amber-800 px-4 py-2 text-sm font-semibold text-white hover:bg-amber-900 disabled:opacity-50" onClick={save}>{saving ? <Loader2 className="h-4 w-4 animate-spin" /> : null}{saving ? 'Đang gửi...' : 'Gửi QS rà soát'}</button></div>}
    >
      <div ref={formRef} className="space-y-5">
        <section className="rounded-lg border border-amber-200 bg-amber-50 p-4 text-sm text-amber-950">
          <p className="font-semibold">{context?.contract ? `${context.contract.contract_code} · ${context.contract.title}` : 'Chưa có hợp đồng chủ đầu tư đang hiệu lực'}</p>
          <p className="mt-1 text-xs leading-5 text-amber-800">Luồng xử lý: Đề xuất → QS rà soát → Mua hàng lấy giá → Quản lý dự án/Giám đốc phê duyệt.</p>
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
          <FieldBlock label="Giá trị ảnh hưởng dự kiến" error={errors.amount_delta}><CurrencyInput allowNegative inputClassName={controlClass(errors.amount_delta)} invalid={Boolean(errors.amount_delta)} value={values.amount_delta} onValueChange={({ numericString }) => change('amount_delta', numericString)} /></FieldBlock>
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
