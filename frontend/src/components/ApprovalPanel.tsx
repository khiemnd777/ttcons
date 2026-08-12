import { useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { Check, MessageSquareWarning, X } from 'lucide-react';
import clsx from 'clsx';
import type { ApprovalRequest } from '../lib/types/domain';
import { post } from '../lib/api/client';
import { Dialog } from './Dialog';
import { StatusBadge } from './StatusBadge';
import { useToast } from './ToastProvider';
import {
  approvalActionLabel,
  approvalCanDecide,
  approvalDecisionLabel,
  approvalRequiresComment,
  type ApprovalDecisionAction,
} from './approvalPresentation';

export function ApprovalPanel({ request, onChanged }: { request?: ApprovalRequest; onChanged?: (request: ApprovalRequest) => void }) {
  if (!request) return null;
  return (
    <section className="rounded-lg border border-amber-200 bg-amber-50 p-4">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <h3 className="text-sm font-semibold text-ink">Yêu cầu phê duyệt</h3>
            <StatusBadge status={request.status} />
          </div>
          <p className="mt-1 text-sm text-slate-700">{request.target_name || request.reason || 'Yêu cầu phê duyệt'}</p>
          <p className="mt-1 text-xs font-medium text-slate-600">{approvalDecisionLabel(request)}</p>
        </div>
        {approvalCanDecide(request) ? <ApprovalDecisionActions request={request} onChanged={onChanged} /> : <span className="text-xs font-semibold text-slate-500">Chỉ theo dõi</span>}
      </div>
    </section>
  );
}

export function ApprovalDecisionActions({ request, onChanged, compact = false }: { request: ApprovalRequest; onChanged?: (request: ApprovalRequest) => void; compact?: boolean }) {
  const actions = (request.available_actions ?? []).filter(isApprovalDecisionAction);
  if (!request.capabilities?.can_decide || !actions.length) return null;
  return (
    <div className="flex flex-wrap gap-2">
      {actions.map((action) => <ApprovalDecisionButton key={action} request={request} action={action} compact={compact} onChanged={onChanged} />)}
    </div>
  );
}

export function ApprovalDecisionButton({ request, action, onChanged, compact = false }: {
  request: ApprovalRequest;
  action: ApprovalDecisionAction;
  onChanged?: (request: ApprovalRequest) => void;
  compact?: boolean;
}) {
  const [open, setOpen] = useState(false);
  const [comment, setComment] = useState('');
  const [invalid, setInvalid] = useState(false);
  const queryClient = useQueryClient();
  const toast = useToast();
  const required = approvalRequiresComment(request);
  const mutation = useMutation({
    mutationFn: () => post<ApprovalRequest>(`/approvals/${request.id}/${action}`, { comment: comment.trim() }),
    onSuccess: (updated) => {
      setOpen(false);
      setComment('');
      setInvalid(false);
      queryClient.invalidateQueries({ queryKey: ['approvals'] });
      queryClient.invalidateQueries({ queryKey: ['dashboard'] });
      queryClient.invalidateQueries({ queryKey: ['my-work-items'] });
      queryClient.invalidateQueries({ queryKey: ['pipeline-run', request.project_id] });
      queryClient.invalidateQueries({ queryKey: ['project-operations', request.project_id] });
      queryClient.invalidateQueries({ queryKey: ['work-item'] });
      toast.success(`${approvalActionLabel(action)} yêu cầu thành công`);
      onChanged?.(updated);
    },
    onError: (error) => toast.error(`Không thể ${approvalActionLabel(action).toLowerCase()} yêu cầu`, error),
  });
  const Icon = action === 'approve' ? Check : action === 'request-change' ? MessageSquareWarning : X;
  const tone = action === 'approve'
    ? 'border-emerald-600 bg-emerald-600 text-white hover:bg-emerald-700'
    : action === 'request-change'
      ? 'border-amber-300 bg-white text-amber-800 hover:bg-amber-100'
      : 'border-rose-300 bg-white text-rose-700 hover:bg-rose-50';

  function submit() {
    if (required && !comment.trim()) {
      setInvalid(true);
      toast.warn({ title: 'Thiếu ý kiến phê duyệt', description: 'Vui lòng nhập ý kiến trước khi xác nhận.' });
      return;
    }
    mutation.mutate();
  }

  return (
    <>
      <button type="button" className={clsx('inline-flex items-center justify-center gap-1.5 rounded-md border font-semibold disabled:opacity-60', compact ? 'px-2.5 py-1.5 text-xs' : 'px-3 py-2 text-sm', tone)} onClick={() => setOpen(true)}>
        <Icon className="h-4 w-4" /> {approvalActionLabel(action)}
      </button>
      <Dialog
        open={open}
        title={`${approvalActionLabel(action)} yêu cầu`}
        description={`${request.project_name || 'Công trình'} · ${request.target_name || request.reason || 'Yêu cầu phê duyệt'}`}
        size="sm"
        onClose={() => !mutation.isPending && setOpen(false)}
        footer={<div className="flex justify-end gap-2"><button type="button" className="rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-slate-700" disabled={mutation.isPending} onClick={() => setOpen(false)}>Hủy</button><button type="button" className={clsx('rounded-md border px-3 py-2 text-sm font-semibold disabled:opacity-60', tone)} disabled={mutation.isPending} onClick={submit}>{mutation.isPending ? 'Đang lưu…' : 'Xác nhận'}</button></div>}
      >
        <label className="block text-sm font-semibold text-slate-700">
          Ý kiến{required ? <span className="text-rose-600"> *</span> : null}
          <textarea className={clsx('mt-2 min-h-28 w-full rounded-md border px-3 py-2 text-sm font-normal outline-none focus:border-blue-400', invalid ? 'border-rose-400 bg-rose-50' : 'border-line')} value={comment} onChange={(event) => { setComment(event.target.value); setInvalid(false); }} placeholder="Nêu rõ kết quả rà soát hoặc nội dung cần chỉnh sửa" />
        </label>
        {invalid ? <p className="mt-1 text-xs font-semibold text-rose-600">Ý kiến là bắt buộc.</p> : null}
        <p className="mt-3 rounded-md bg-slate-50 p-3 text-xs leading-5 text-slate-600">{approvalDecisionLabel(request)}</p>
      </Dialog>
    </>
  );
}

function isApprovalDecisionAction(value: string): value is ApprovalDecisionAction {
  return value === 'approve' || value === 'request-change' || value === 'reject';
}
