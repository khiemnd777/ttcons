import type { ReactNode } from 'react';
import { AlertTriangle, CheckCircle2, Circle, CreditCard, ShieldCheck } from 'lucide-react';
import { CurrencyText } from '../../components/CurrencyText';
import { Dialog } from '../../components/Dialog';
import { StatusBadge } from '../../components/StatusBadge';
import type { PayableRecord, ReceivableRecord } from '../../lib/types/domain';
import { dateText } from '../../lib/utils/format';
import { InputInvoicePanel } from './InputInvoicePanel';
import { InputPaymentPlanPanel } from './InputPaymentPlanPanel';
import { OutputBillingPlanPanel } from './OutputBillingPlanPanel';
import { OutputInvoicePanel } from './OutputInvoicePanel';
import { PayableReconciliationPanel } from './PayableReconciliationPanel';
import { ReceivableReconciliationPanel } from './ReceivableReconciliationPanel';
import { agingBadgeClass, debtAgingLabel } from './payableDebtUtils';
import { RelatedRecords } from '../_shared/RecordLinks';

export type DebtDetailTab = 'overview' | 'invoices' | 'plan' | 'payments' | 'reconciliation';
export type DebtDetail = { type: 'payable'; record: PayableRecord } | { type: 'receivable'; record: ReceivableRecord };

export function DebtDetailDialog({
  detail,
  tab,
  canApprove,
  approving,
  onTab,
  onClose,
  onChanged,
  onApprove,
}: {
  detail: DebtDetail | null;
  tab: DebtDetailTab;
  canApprove: boolean;
  approving: boolean;
  onTab: (tab: DebtDetailTab) => void;
  onClose: () => void;
  onChanged: () => void;
  onApprove: () => void;
}) {
  if (!detail) return null;
  const payable = detail.type === 'payable';
  const record = detail.record;
  const party = payable ? (record as PayableRecord).vendor_name : (record as ReceivableRecord).client_name;
  const code = payable ? (record as PayableRecord).payable_code : (record as ReceivableRecord).receivable_code;
  const warningCount = Number(record.reconciliation_issue_count ?? 0);
  const planCount = Number(record.schedule_count ?? 0) + Number(payable ? (record as PayableRecord).advance_request_count ?? 0 : (record as ReceivableRecord).advance_receipt_count ?? 0);
  const primary = primaryAction(detail, canApprove, onApprove, onTab);

  return (
    <Dialog manageFocus open title={`${code} · ${payable ? 'Phải trả' : 'Phải thu'}`} onClose={onClose} size="workspace">
      <div className="-mx-5 -mt-5 border-b border-line bg-slate-50 px-5 py-4">
        <div className="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
          <div className="min-w-0">
            <div className="flex flex-wrap items-center gap-2">
              <h3 className="truncate text-lg font-semibold text-ink">{party}</h3>
              <StatusBadge status={record.status} />
              <span className={agingBadgeClass(record.aging_bucket)}>{debtAgingLabel(record.aging_bucket, record.overdue_days)}</span>
            </div>
            <p className="mt-1 text-sm text-slate-600">{record.project_name ? `${record.project_name} · ` : ''}Hạn {dateText(record.effective_due_date ?? record.due_date)}</p>
          </div>
          <div className="flex flex-col items-start gap-2 sm:flex-row sm:items-center">
            <div className="sm:text-right">
              <div className="text-xs font-semibold uppercase tracking-wide text-slate-500">{payable ? 'Còn phải trả' : 'Còn phải thu'}</div>
              <div className="text-xl font-semibold text-ink"><CurrencyText value={record.remaining_amount ?? 0} /></div>
            </div>
            {primary ? <button type="button" disabled={approving} className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-4 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:opacity-50" onClick={primary.onClick}>{primary.icon}{approving ? 'Đang xử lý...' : primary.label}</button> : null}
          </div>
        </div>
      </div>

      <div className="mb-5 flex gap-1 overflow-x-auto border-b border-line pt-2">
        <DetailTabButton active={tab === 'overview'} label="Tổng quan" onClick={() => onTab('overview')} />
        <DetailTabButton active={tab === 'invoices'} label="Hóa đơn" count={Number(record.invoice_count ?? 0)} onClick={() => onTab('invoices')} />
        <DetailTabButton active={tab === 'plan'} label="Lịch & tạm ứng" count={planCount} onClick={() => onTab('plan')} />
        <DetailTabButton active={tab === 'payments'} label="Thu/chi" onClick={() => onTab('payments')} />
        <DetailTabButton active={tab === 'reconciliation'} label="Đối soát" count={warningCount} warning={warningCount > 0} onClick={() => onTab('reconciliation')} />
      </div>

      {payable ? <PayableDetail tab={tab} record={record as PayableRecord} onChanged={onChanged} /> : <ReceivableDetail tab={tab} record={record as ReceivableRecord} onChanged={onChanged} />}
      {tab === 'overview' ? <div className="mt-6"><RelatedRecords kind={detail.type} record={record} /></div> : null}
    </Dialog>
  );
}

function primaryAction(detail: DebtDetail, canApprove: boolean, onApprove: () => void, onTab: (tab: DebtDetailTab) => void) {
  const record = detail.record;
  const warningCount = Number(record.reconciliation_issue_count ?? 0);
  if (canApprove && ['draft', 'submitted', 'recorded'].includes(record.status)) return { label: 'Duyệt công nợ', icon: <ShieldCheck className="h-4 w-4" />, onClick: onApprove };
  if (warningCount > 0) return { label: 'Xử lý đối soát', icon: <AlertTriangle className="h-4 w-4" />, onClick: () => onTab('reconciliation') };
  if (Number(record.remaining_amount ?? 0) > 0 && ['approved', 'partially_paid', 'partially_received', 'overdue'].includes(record.status)) return { label: detail.type === 'payable' ? 'Ghi nhận thanh toán' : 'Ghi nhận thu', icon: <CreditCard className="h-4 w-4" />, onClick: () => onTab('payments') };
  return null;
}

function DetailTabButton({ active, label, count, warning, onClick }: { active: boolean; label: string; count?: number; warning?: boolean; onClick: () => void }) {
  return <button type="button" className={`whitespace-nowrap border-b-2 px-3 py-3 text-sm font-semibold ${active ? 'border-blue-600 text-blue-700' : 'border-transparent text-slate-500 hover:text-ink'}`} onClick={onClick}>{label}{count !== undefined ? <span className={`ml-1.5 rounded-full px-1.5 py-0.5 text-[11px] ${warning ? 'bg-amber-100 text-amber-800' : active ? 'bg-blue-100 text-blue-700' : 'bg-slate-100 text-slate-600'}`}>{count}</span> : null}</button>;
}

function PayableDetail({ tab, record, onChanged }: { tab: DebtDetailTab; record: PayableRecord; onChanged: () => void }) {
  if (tab === 'invoices') return <InputInvoicePanel payable={record} onChanged={onChanged} />;
  if (tab === 'plan') return <InputPaymentPlanPanel payable={record} onChanged={onChanged} view="plan" />;
  if (tab === 'payments') return <InputPaymentPlanPanel payable={record} onChanged={onChanged} view="payments" />;
  if (tab === 'reconciliation') return <PayableReconciliationPanel payable={record} />;
  return <DebtOverview record={record} direction="payable" />;
}

function ReceivableDetail({ tab, record, onChanged }: { tab: DebtDetailTab; record: ReceivableRecord; onChanged: () => void }) {
  if (tab === 'invoices') return <OutputInvoicePanel receivable={record} onChanged={onChanged} />;
  if (tab === 'plan') return <OutputBillingPlanPanel receivable={record} onChanged={onChanged} view="plan" />;
  if (tab === 'payments') return <OutputBillingPlanPanel receivable={record} onChanged={onChanged} view="payments" />;
  if (tab === 'reconciliation') return <ReceivableReconciliationPanel receivable={record} />;
  return <DebtOverview record={record} direction="receivable" />;
}

function DebtOverview({ record, direction }: { record: PayableRecord | ReceivableRecord; direction: 'payable' | 'receivable' }) {
  const payable = direction === 'payable';
  const paid = payable ? (record as PayableRecord).paid_amount : (record as ReceivableRecord).received_amount;
  const advance = record.advance_approved_total ?? 0;
  const warningCount = Number(record.reconciliation_issue_count ?? 0);
  const invoiceReady = Number(record.invoice_count ?? 0) > 0;
  const settled = Number(record.remaining_amount ?? 0) <= 0;
  return <div className="grid gap-5 xl:grid-cols-[minmax(0,1fr)_380px]">
    <section className="rounded-lg border border-line bg-white p-4">
      <h3 className="font-semibold text-ink">Tình trạng xử lý</h3>
      <p className="mt-1 text-xs text-slate-500">Các bước phản ánh dữ liệu nguồn hiện có; thu/chi vẫn được ghi nhận trong tab chuyên biệt.</p>
      <div className="mt-4 divide-y divide-line">
        <ProcessRow done={!['draft', 'submitted', 'recorded'].includes(record.status)} label="Nghĩa vụ đã được duyệt" detail={<StatusBadge status={record.status} />} />
        <ProcessRow done={invoiceReady} label={invoiceReady ? `${record.invoice_count} hóa đơn đã liên kết` : 'Chưa có hóa đơn liên kết'} />
        <ProcessRow done={settled || Number(paid ?? 0) > 0} label={settled ? 'Đã tất toán nghĩa vụ' : `${payable ? 'Đã trả' : 'Đã thu'} một phần hoặc chưa phát sinh`} detail={<CurrencyText value={paid ?? 0} />} />
        <ProcessRow done={warningCount === 0} warning={warningCount > 0} label={warningCount ? `${warningCount} cảnh báo cần đối soát` : 'Đối soát không có cảnh báo'} />
      </div>
    </section>
    <section className="rounded-lg border border-line bg-slate-50 p-4">
      <h3 className="font-semibold text-ink">Cân đối nghĩa vụ</h3>
      <div className="mt-4 space-y-3 text-sm">
        <MoneyRow label="Giá trị ghi nhận" value={record.gross_amount} />
        <MoneyRow label="Ứng trước đã duyệt" value={advance} prefix="−" />
        {payable ? <MoneyRow label="Khấu trừ khác" value={(record as PayableRecord).deduction_amount ?? 0} prefix="−" /> : null}
        <MoneyRow label={payable ? 'Đã trả' : 'Đã thu'} value={paid ?? 0} prefix="−" />
        <div className="border-t border-line pt-3"><MoneyRow label={payable ? 'Còn phải trả' : 'Còn phải thu'} value={record.remaining_amount ?? 0} strong /></div>
      </div>
      {payable && Number((record as PayableRecord).retention_amount ?? 0) > 0 ? <div className="mt-4 rounded-md border border-blue-100 bg-blue-50 p-3 text-xs leading-5 text-blue-800">Giữ lại <CurrencyText value={(record as PayableRecord).retention_amount ?? 0} /> đang được theo dõi riêng.</div> : null}
      <div className="mt-4 grid gap-2 text-xs text-slate-500 sm:grid-cols-2 xl:grid-cols-1">
        <span>Nguồn: {record.source_type || 'manual'}</span>
        <span>Hạn hiệu lực: {dateText(record.effective_due_date ?? record.due_date)}</span>
      </div>
    </section>
  </div>;
}

function ProcessRow({ done, warning, label, detail }: { done: boolean; warning?: boolean; label: string; detail?: ReactNode }) {
  const Icon = warning ? AlertTriangle : done ? CheckCircle2 : Circle;
  const tone = warning ? 'text-amber-700' : done ? 'text-emerald-700' : 'text-slate-400';
  return <div className="flex items-center justify-between gap-3 py-3 first:pt-0 last:pb-0"><div className="flex items-center gap-2"><Icon className={`h-4 w-4 shrink-0 ${tone}`} /><span className="text-sm font-medium text-slate-700">{label}</span></div>{detail ? <div className="shrink-0 text-sm text-ink">{detail}</div> : null}</div>;
}

function MoneyRow({ label, value, prefix, strong }: { label: string; value: unknown; prefix?: string; strong?: boolean }) {
  return <div className={`flex items-center justify-between gap-3 ${strong ? 'text-base font-semibold text-ink' : 'text-slate-600'}`}><span>{label}</span><span>{prefix ? `${prefix} ` : ''}<CurrencyText value={value} /></span></div>;
}
