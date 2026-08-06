import { useEffect, useMemo, useState, type ReactNode } from 'react';
import { Plus, Trash2 } from 'lucide-react';
import { CurrencyText } from '../../components/CurrencyText';
import { CurrencyInput } from '../../components/CurrencyInput';
import { Dialog } from '../../components/Dialog';
import { useToast } from '../../components/ToastProvider';

type JournalLine = {
  key: string;
  account_id: string;
  description: string;
  debit_amount: string;
  credit_amount: string;
};

const emptyLine = (): JournalLine => ({ key: crypto.randomUUID(), account_id: '', description: '', debit_amount: '', credit_amount: '' });

export function ManualJournalDialog({
  open,
  accounts,
  saving,
  onClose,
  onSubmit,
}: {
  open: boolean;
  accounts: Record<string, any>[];
  saving: boolean;
  onClose: () => void;
  onSubmit: (values: Record<string, any>) => Promise<unknown>;
}) {
  const today = new Date().toISOString().slice(0, 10);
  const toast = useToast();
  const [entryDate, setEntryDate] = useState(today);
  const [documentDate, setDocumentDate] = useState(today);
  const [documentNo, setDocumentNo] = useState('');
  const [description, setDescription] = useState('');
  const [lines, setLines] = useState<JournalLine[]>([emptyLine(), emptyLine()]);
  const [invalid, setInvalid] = useState<string[]>([]);

  useEffect(() => {
    if (!open) return;
    setEntryDate(today);
    setDocumentDate(today);
    setDocumentNo('');
    setDescription('');
    setLines([emptyLine(), emptyLine()]);
    setInvalid([]);
  }, [open, today]);

  const totals = useMemo(() => lines.reduce((result, line) => ({
    debit: result.debit + amount(line.debit_amount),
    credit: result.credit + amount(line.credit_amount),
  }), { debit: 0, credit: 0 }), [lines]);
  const balanced = totals.debit > 0 && Math.abs(totals.debit - totals.credit) < 0.005;

  const clearInvalid = () => setInvalid((current) => current.length ? [] : current);

  const updateLine = (key: string, field: keyof JournalLine, value: string) => {
    setLines((current) => current.map((line) => line.key === key ? { ...line, [field]: value } : line));
    clearInvalid();
  };

  const submit = async () => {
    const errors: string[] = [];
    if (!entryDate) errors.push('Ngày hạch toán');
    if (!description.trim()) errors.push('Diễn giải chung');
    if (lines.length < 2) errors.push('Ít nhất hai dòng định khoản');
    lines.forEach((line, index) => {
      const debit = amount(line.debit_amount);
      const credit = amount(line.credit_amount);
      if (!line.account_id) errors.push(`Tài khoản dòng ${index + 1}`);
      if ((debit <= 0 && credit <= 0) || (debit > 0 && credit > 0)) errors.push(`Số tiền Nợ/Có dòng ${index + 1}`);
    });
    if (!balanced) errors.push('Tổng phát sinh Nợ phải bằng tổng phát sinh Có');
    setInvalid(errors);
    if (errors.length) {
      toast.warn({ title: 'Chứng từ chưa đủ điều kiện ghi sổ', description: errors.join(' · ') });
      return;
    }
    await onSubmit({
      entry_date: entryDate,
      document_date: documentDate || undefined,
      source_document_no: documentNo.trim() || undefined,
      description: description.trim(),
      lines: lines.map((line) => ({
        account_id: line.account_id,
        description: line.description.trim(),
        debit_amount: amount(line.debit_amount),
        credit_amount: amount(line.credit_amount),
      })),
    });
  };

  const fieldClass = (isInvalid = false) => `mt-1 w-full rounded-md border px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-100 ${isInvalid ? 'border-rose-400 bg-rose-50' : 'border-line bg-white'}`;
  return <Dialog open={open} title="Lập Chứng từ ghi sổ" description="Hạch toán nghiệp vụ khác theo nguyên tắc tổng phát sinh Nợ bằng tổng phát sinh Có. Chứng từ đã ghi sổ chỉ được sửa bằng bút toán đảo." onClose={onClose} size="xl" footer={<div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between"><div className={`text-sm font-semibold ${balanced ? 'text-emerald-700' : 'text-rose-700'}`}>{balanced ? 'Định khoản đã cân' : 'Định khoản chưa cân'}</div><div className="flex justify-end gap-2"><button type="button" className="rounded-md border border-line bg-white px-4 py-2 text-sm font-semibold" onClick={onClose}>Hủy</button><button type="button" className="rounded-md bg-blue-600 px-4 py-2 text-sm font-semibold text-white disabled:opacity-50" disabled={saving} onClick={submit}>{saving ? 'Đang ghi sổ...' : 'Ghi sổ'}</button></div></div>}>
    <div className="space-y-5">
      <div className="grid gap-3 md:grid-cols-4">
        <Field label="Ngày hạch toán *"><input type="date" value={entryDate} onChange={(event) => { setEntryDate(event.target.value); clearInvalid(); }} className={fieldClass(!entryDate)} /></Field>
        <Field label="Ngày chứng từ"><input type="date" value={documentDate} onChange={(event) => { setDocumentDate(event.target.value); clearInvalid(); }} className={fieldClass()} /></Field>
        <Field label="Số chứng từ gốc"><input value={documentNo} onChange={(event) => { setDocumentNo(event.target.value); clearInvalid(); }} placeholder="HĐ, BB, UNC..." className={fieldClass()} /></Field>
        <Field label="Diễn giải chung *"><input value={description} onChange={(event) => { setDescription(event.target.value); clearInvalid(); }} placeholder="Nội dung nghiệp vụ kinh tế" className={fieldClass(!description.trim())} /></Field>
      </div>

      <div className="space-y-3 md:hidden">
        {lines.map((line, index) => {
          const debit = amount(line.debit_amount);
          const credit = amount(line.credit_amount);
          const amountInvalid = (debit <= 0 && credit <= 0) || (debit > 0 && credit > 0);
          return <section key={line.key} className="rounded-lg border border-line bg-slate-50 p-3">
            <div className="mb-3 flex items-center justify-between"><p className="text-sm font-semibold text-ink">Dòng định khoản {index + 1}</p><button type="button" disabled={lines.length <= 2} className="rounded-md p-2 text-slate-400 hover:bg-rose-50 hover:text-rose-700 disabled:opacity-30" aria-label={`Xóa dòng ${index + 1}`} onClick={() => { setLines((current) => current.filter((item) => item.key !== line.key)); clearInvalid(); }}><Trash2 className="h-4 w-4" /></button></div>
            <div className="space-y-3"><Field label="Tài khoản *"><select value={line.account_id} onChange={(event) => updateLine(line.key, 'account_id', event.target.value)} className={fieldClass(!line.account_id)}><option value="">Chọn tài khoản</option>{accounts.filter((account) => account.is_postable !== false).map((account) => <option key={account.id} value={account.id}>{account.account_code} · {account.name}</option>)}</select></Field><Field label="Diễn giải dòng"><input value={line.description} onChange={(event) => updateLine(line.key, 'description', event.target.value)} className={fieldClass()} /></Field><div className="grid grid-cols-2 gap-2"><Field label="Phát sinh Nợ"><CurrencyInput min={0} value={line.debit_amount} invalid={amountInvalid} onValueChange={({ numericString }) => updateLine(line.key, 'debit_amount', numericString)} /></Field><Field label="Phát sinh Có"><CurrencyInput min={0} value={line.credit_amount} invalid={amountInvalid} onValueChange={({ numericString }) => updateLine(line.key, 'credit_amount', numericString)} /></Field></div></div>
          </section>;
        })}
        <div className="grid grid-cols-2 gap-2 rounded-lg border border-line bg-white p-3 text-sm font-semibold"><span>Tổng Nợ<br /><CurrencyText value={totals.debit} /></span><span className="text-right">Tổng Có<br /><CurrencyText value={totals.credit} /></span></div>
      </div>

      <div className="hidden overflow-x-auto rounded-lg border border-line md:block">
        <table className="min-w-[920px] w-full text-sm">
          <thead className="bg-slate-50 text-left text-xs font-semibold uppercase tracking-wide text-slate-500"><tr><th className="px-3 py-2">Tài khoản *</th><th className="px-3 py-2">Diễn giải dòng</th><th className="px-3 py-2 text-right">Phát sinh Nợ</th><th className="px-3 py-2 text-right">Phát sinh Có</th><th className="w-12 px-2 py-2" /></tr></thead>
          <tbody>
            {lines.map((line, index) => {
              const debit = amount(line.debit_amount);
              const credit = amount(line.credit_amount);
              const amountInvalid = (debit <= 0 && credit <= 0) || (debit > 0 && credit > 0);
              return <tr key={line.key} className="border-t border-line align-top">
                <td className="px-3 py-2"><select value={line.account_id} onChange={(event) => updateLine(line.key, 'account_id', event.target.value)} className={fieldClass(!line.account_id)}><option value="">Chọn tài khoản</option>{accounts.filter((account) => account.is_postable !== false).map((account) => <option key={account.id} value={account.id}>{account.account_code} · {account.name}</option>)}</select></td>
                <td className="px-3 py-2"><input value={line.description} onChange={(event) => updateLine(line.key, 'description', event.target.value)} placeholder={`Diễn giải dòng ${index + 1}`} className={fieldClass()} /></td>
                <td className="px-3 py-2"><CurrencyInput min={0} value={line.debit_amount} invalid={amountInvalid} onValueChange={({ numericString }) => updateLine(line.key, 'debit_amount', numericString)} /></td>
                <td className="px-3 py-2"><CurrencyInput min={0} value={line.credit_amount} invalid={amountInvalid} onValueChange={({ numericString }) => updateLine(line.key, 'credit_amount', numericString)} /></td>
                <td className="px-2 py-3"><button type="button" disabled={lines.length <= 2} className="rounded-md p-2 text-slate-400 hover:bg-rose-50 hover:text-rose-700 disabled:opacity-30" aria-label={`Xóa dòng ${index + 1}`} onClick={() => { setLines((current) => current.filter((item) => item.key !== line.key)); clearInvalid(); }}><Trash2 className="h-4 w-4" /></button></td>
              </tr>;
            })}
          </tbody>
          <tfoot className="border-t border-line bg-slate-50 font-semibold"><tr><td colSpan={2} className="px-3 py-3 text-right">Cộng phát sinh</td><td className="px-3 py-3 text-right"><CurrencyText value={totals.debit} /></td><td className="px-3 py-3 text-right"><CurrencyText value={totals.credit} /></td><td /></tr></tfoot>
        </table>
      </div>
      <button type="button" className="inline-flex items-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700" onClick={() => { setLines((current) => [...current, emptyLine()]); clearInvalid(); }}><Plus className="h-4 w-4" />Thêm dòng định khoản</button>
      {invalid.length ? <div className="rounded-md border border-rose-200 bg-rose-50 p-3 text-sm text-rose-700"><span className="font-semibold">Cần bổ sung:</span> {invalid.join(' · ')}</div> : null}
    </div>
  </Dialog>;
}

function Field({ label, children }: { label: string; children: ReactNode }) {
  return <label className="block"><span className="text-xs font-semibold uppercase tracking-wide text-slate-500">{label}</span>{children}</label>;
}

function amount(value: string) {
  const parsed = Number(value);
  return Number.isFinite(parsed) && parsed > 0 ? parsed : 0;
}
