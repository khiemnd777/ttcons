import { type FormEvent, useEffect, useState } from 'react';
import { ArrowRight } from 'lucide-react';
import { Dialog } from '../../components/Dialog';

export type AccountingHandoffAction = 'handoff' | 'acknowledge' | 'return';

export type AccountingHandoffActionTarget = {
  handoff_id?: string | null;
  source_code: string;
  source_title: string;
  note?: string;
  external_reference?: string;
};

export type AccountingHandoffDialog<T extends AccountingHandoffActionTarget = AccountingHandoffActionTarget> = {
  action: AccountingHandoffAction;
  item: T;
} | null;

export function AccountingHandoffActionDialog<T extends AccountingHandoffActionTarget>({
  dialog,
  pending,
  onClose,
  onSubmit,
}: {
  dialog: AccountingHandoffDialog<T>;
  pending: boolean;
  onClose: () => void;
  onSubmit: (values: { note: string; returnReason: string; externalReference: string }) => void;
}) {
  const [note, setNote] = useState('');
  const [returnReason, setReturnReason] = useState('');
  const [externalReference, setExternalReference] = useState('');
  const [warning, setWarning] = useState('');

  useEffect(() => {
    setNote(dialog?.item.note ?? '');
    setReturnReason('');
    setExternalReference(dialog?.item.external_reference ?? '');
    setWarning('');
  }, [dialog]);

  const title = dialog?.action === 'acknowledge'
    ? 'Xác nhận tiếp nhận hồ sơ'
    : dialog?.action === 'return'
      ? 'Trả hồ sơ để bổ sung'
      : dialog?.item.handoff_id
        ? 'Bàn giao lại hồ sơ'
        : 'Bàn giao hồ sơ cho Kế toán';

  function submit(event: FormEvent) {
    event.preventDefault();
    if (dialog?.action === 'return' && !returnReason.trim()) {
      setWarning('Cần ghi rõ nội dung hồ sơ phải bổ sung.');
      return;
    }
    onSubmit({
      note: note.trim(),
      returnReason: returnReason.trim(),
      externalReference: externalReference.trim(),
    });
  }

  return (
    <Dialog
      open={Boolean(dialog)}
      title={title}
      description={dialog ? `${dialog.item.source_code} · ${dialog.item.source_title}` : undefined}
      onClose={onClose}
      size="md"
    >
      <form className="space-y-4" onSubmit={submit}>
        {dialog?.action === 'return' ? (
          <label className="block text-sm font-semibold text-slate-700">
            Nội dung cần bổ sung <span className="text-rose-600">*</span>
            <textarea
              className="mt-1 min-h-24 w-full rounded-md border border-line px-3 py-2 text-sm"
              value={returnReason}
              onChange={(event) => setReturnReason(event.target.value)}
            />
          </label>
        ) : null}
        {dialog?.action !== 'return' ? (
          <label className="block text-sm font-semibold text-slate-700">
            Mã tham chiếu MISA (không bắt buộc)
            <input
              className="mt-1 w-full rounded-md border border-line px-3 py-2 text-sm"
              value={externalReference}
              onChange={(event) => setExternalReference(event.target.value)}
              placeholder="Ví dụ: MISA-CT-001"
            />
          </label>
        ) : null}
        <label className="block text-sm font-semibold text-slate-700">
          Ghi chú
          <textarea
            className="mt-1 min-h-20 w-full rounded-md border border-line px-3 py-2 text-sm"
            value={note}
            onChange={(event) => setNote(event.target.value)}
          />
        </label>
        {warning ? <p className="rounded-md border border-amber-200 bg-amber-50 p-3 text-sm text-amber-800">{warning}</p> : null}
        <div className="flex justify-end gap-2">
          <button type="button" className="rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-slate-700" onClick={onClose}>Hủy</button>
          <button type="submit" disabled={pending} className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:opacity-60">
            <ArrowRight className="h-4 w-4" />
            {pending ? 'Đang xử lý...' : 'Xác nhận'}
          </button>
        </div>
      </form>
    </Dialog>
  );
}
