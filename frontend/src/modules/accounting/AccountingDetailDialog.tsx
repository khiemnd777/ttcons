import type { ReactNode } from 'react';
import { Check, ExternalLink } from 'lucide-react';
import { Link } from 'react-router-dom';
import { CurrencyText } from '../../components/CurrencyText';
import { Dialog } from '../../components/Dialog';
import { EnumChip } from '../../components/EnumChip';
import { StatusBadge } from '../../components/StatusBadge';
import { dateText } from '../../lib/utils/format';

export type AccountingDetail = { type: 'budget' | 'payment'; record: Record<string, any> };

export function AccountingDetailDialog({ detail, canApprove, approving, onClose, onApprove }: { detail: AccountingDetail | null; canApprove: boolean; approving: boolean; onClose: () => void; onApprove: () => void }) {
  if (!detail) return null;
  const budget = detail.type === 'budget';
  const record = detail.record;
  const canAct = canApprove && (budget ? ['draft', 'submitted'].includes(record.status) : ['draft', 'submitted', 'pending', 'waiting_approval'].includes(record.status));
  const code = budget ? record.budget_code : record.payment_code;

  return <Dialog open title={`${code} · ${budget ? 'Ngân sách' : 'Phiếu thu/chi'}`} onClose={onClose} size="xl">
    <div className="-mx-5 -mt-5 border-b border-line bg-slate-50 px-5 py-4">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <div className="flex flex-wrap items-center gap-2"><h3 className="text-lg font-semibold text-ink">{budget ? record.name : paymentTypeLabel(record.payment_type)}</h3><StatusBadge status={record.status} /></div>
          <p className="mt-1 text-sm text-slate-600">{record.project_name || 'Công trình hiện tại'}</p>
        </div>
        {canAct ? <button type="button" disabled={approving} className="inline-flex items-center justify-center gap-2 rounded-md bg-blue-600 px-4 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:opacity-50" onClick={onApprove}><Check className="h-4 w-4" />{approving ? 'Đang duyệt...' : budget ? 'Duyệt ngân sách' : 'Duyệt phiếu'}</button> : null}
      </div>
    </div>
    <div className="pt-5">{budget ? <BudgetDetail record={record} /> : <PaymentDetail record={record} />}</div>
  </Dialog>;
}

function BudgetDetail({ record }: { record: Record<string, any> }) {
  const planned = Number(record.planned_amount ?? 0);
  const approved = Number(record.approved_amount ?? 0);
  const used = Number(record.used_amount ?? 0);
  const denominator = approved > 0 ? approved : planned;
  const usedPercent = denominator > 0 ? Math.min((used / denominator) * 100, 100) : 0;
  return <div className="space-y-5">
    <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
      <Metric label="Kế hoạch" value={<CurrencyText value={planned} />} />
      <Metric label="Đã duyệt" value={<CurrencyText value={approved} />} />
      <Metric label="Đã dùng" value={<CurrencyText value={used} />} />
      <Metric label="Còn lại" value={<CurrencyText value={record.remaining_amount ?? 0} />} tone={Number(record.remaining_amount ?? 0) > 0 ? 'green' : 'slate'} />
    </div>
    <section className="rounded-lg border border-line p-4">
      <div className="flex items-center justify-between gap-3 text-sm"><span className="font-semibold text-ink">Mức sử dụng ngân sách</span><span className="text-slate-600">{Math.round(usedPercent)}%</span></div>
      <div className="mt-3 h-3 overflow-hidden rounded-full bg-slate-100"><div className="h-full rounded-full bg-blue-600" style={{ width: `${usedPercent}%` }} /></div>
      <p className="mt-2 text-xs text-slate-500">{approved > 0 ? 'Tỷ lệ tính trên ngân sách đã duyệt.' : 'Ngân sách chưa duyệt; số còn lại tiếp tục do hệ thống quản lý.'}</p>
    </section>
    <RecordMeta record={record} />
  </div>;
}

function PaymentDetail({ record }: { record: Record<string, any> }) {
  const linkedDebt = Boolean(record.target_type && record.target_id && record.project_id && ['payable', 'receivable'].some((value) => String(record.target_type).includes(value)));
  return <div className="grid gap-5 xl:grid-cols-[minmax(0,1fr)_360px]">
    <section className="rounded-lg border border-line p-5">
      <div className="text-xs font-semibold uppercase tracking-wide text-slate-500">Số tiền</div>
      <div className="mt-2 text-3xl font-semibold text-ink"><CurrencyText value={record.amount} /></div>
      <div className="mt-5 grid gap-4 sm:grid-cols-2">
        <Info label="Loại phiếu" value={<EnumChip group="payment_type" value={record.payment_type} />} />
        <Info label="Phương thức" value={<EnumChip group="payment_method" value={record.payment_method} />} />
        <Info label="Công trình" value={record.project_name || '-'} />
        <Info label="Trạng thái" value={<StatusBadge status={record.status} />} />
      </div>
      {linkedDebt ? <Link className="mt-5 inline-flex items-center gap-2 rounded-md border border-blue-200 bg-blue-50 px-3 py-2 text-sm font-semibold text-blue-700" to={`/projects/${record.project_id}/debt`}>Mở công nợ<ExternalLink className="h-4 w-4" /></Link> : <div className="mt-5 rounded-md border border-line bg-slate-50 p-3 text-xs leading-5 text-slate-600">Phiếu không gắn công nợ phải trả/phải thu; không có phân bổ công nợ để xử lý tại đây.</div>}
    </section>
    <RecordMeta record={record} />
  </div>;
}

function RecordMeta({ record }: { record: Record<string, any> }) {
  return <section className="rounded-lg border border-line bg-slate-50 p-4">
    <h3 className="font-semibold text-ink">Dấu thời gian</h3>
    <dl className="mt-4 space-y-3 text-sm">
      <MetaRow label="Ngày tạo" value={dateText(record.created_at)} />
      <MetaRow label="Cập nhật gần nhất" value={dateText(record.updated_at)} />
      {record.paid_at ? <MetaRow label="Ngày ghi nhận thu/chi" value={dateText(record.paid_at)} /> : null}
      {record.target_type ? <MetaRow label="Loại nguồn" value={String(record.target_type)} /> : null}
    </dl>
  </section>;
}

function Metric({ label, value, tone = 'slate' }: { label: string; value: ReactNode; tone?: 'slate' | 'green' }) {
  return <div className={`rounded-lg border p-4 ${tone === 'green' ? 'border-emerald-200 bg-emerald-50' : 'border-line bg-white'}`}><div className="text-xs font-semibold uppercase tracking-wide text-slate-500">{label}</div><div className="mt-2 text-lg font-semibold text-ink">{value}</div></div>;
}

function Info({ label, value }: { label: string; value: ReactNode }) {
  return <div><div className="text-xs font-semibold uppercase tracking-wide text-slate-500">{label}</div><div className="mt-1 text-sm font-medium text-ink">{value}</div></div>;
}

function MetaRow({ label, value }: { label: string; value: string }) {
  return <div className="flex items-center justify-between gap-3"><dt className="text-slate-500">{label}</dt><dd className="text-right font-medium text-ink">{value}</dd></div>;
}

function paymentTypeLabel(value: unknown) {
  return ({ outbound: 'Chi khác', inbound: 'Thu khác', other: 'Điều chỉnh khác', supplier_payment: 'Thanh toán nhà cung cấp', subcontractor_payment: 'Thanh toán thầu phụ', client_receipt: 'Thu từ khách hàng' } as Record<string, string>)[String(value)] ?? String(value ?? '-');
}
