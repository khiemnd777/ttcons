import { type ReactNode } from 'react';
import { useQuery } from '@tanstack/react-query';
import { RefreshCw } from 'lucide-react';
import { CurrencyText } from '../../components/CurrencyText';
import { DataTable } from '../../components/DataTable';
import { StatusBadge } from '../../components/StatusBadge';
import { get } from '../../lib/api/client';
import type { PayableLedgerEntry, PayableReconciliationSnapshot, PayableRecord } from '../../lib/types/domain';
import { dateText } from '../../lib/utils/format';
import { agingBadgeClass, debtAgingLabel, warningSeverityClass } from './payableDebtUtils';

type PayableReconciliationPanelProps = {
  payable: PayableRecord;
};

export function PayableReconciliationPanel({ payable }: PayableReconciliationPanelProps) {
  const snapshot = useQuery({
    queryKey: ['payable-reconciliation', payable.id],
    queryFn: () => get<PayableReconciliationSnapshot>(`/payables/${payable.id}/reconciliation`),
  });

  const summary = snapshot.data?.summary;
  const warnings = snapshot.data?.warnings ?? [];
  const ledger = snapshot.data?.ledger ?? [];

  return (
    <section className="space-y-3">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div className="min-w-0">
          <h3 className="text-sm font-semibold text-ink">Đối soát công nợ đầu vào</h3>
          <p className="mt-1 text-xs text-slate-500">Aging, hóa đơn, lịch kỳ, ứng trước và payment allocations theo payable này.</p>
        </div>
        <button
          type="button"
          className="inline-flex items-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100 disabled:cursor-not-allowed disabled:opacity-60"
          onClick={() => void snapshot.refetch()}
          disabled={snapshot.isFetching}
        >
          <RefreshCw className={snapshot.isFetching ? 'h-4 w-4 animate-spin' : 'h-4 w-4'} />
          Tải lại
        </button>
      </div>

      {snapshot.error ? (
        <div className="rounded-md border border-rose-100 bg-rose-50 p-3 text-sm text-rose-700">Không tải được đối soát công nợ đầu vào.</div>
      ) : null}

      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <MetricTile label="Gross" value={<CurrencyText value={summary?.gross_amount ?? payable.gross_amount} />} />
        <MetricTile label="Hoá đơn verified" value={<CurrencyText value={summary?.verified_invoice_total ?? payable.verified_invoice_total ?? 0} />} />
        <MetricTile label="Còn phải trả" value={<CurrencyText value={summary?.remaining_amount ?? payable.remaining_amount ?? 0} />} />
        <MetricTile
          label="Aging"
          value={<span className={agingBadgeClass(summary?.aging_bucket ?? payable.aging_bucket)}>{debtAgingLabel(summary?.aging_bucket ?? payable.aging_bucket, summary?.overdue_days ?? payable.overdue_days)}</span>}
        />
      </div>

      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <MetricTile label="Lịch còn" value={<CurrencyText value={summary?.scheduled_remaining ?? payable.scheduled_remaining ?? 0} />} />
        <MetricTile label="Ứng duyệt" value={<CurrencyText value={summary?.advance_approved_total ?? payable.advance_approved_total ?? 0} />} />
        <MetricTile label="Ứng đã chi" value={<CurrencyText value={summary?.advance_paid_total ?? payable.advance_paid_total ?? 0} />} />
        <MetricTile label="Đã quyết toán ứng" value={<CurrencyText value={summary?.advance_settled_total ?? payable.advance_settled_total ?? 0} />} />
      </div>

      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <MetricTile label="Đã trả" value={<CurrencyText value={summary?.paid_amount ?? payable.paid_amount ?? 0} />} />
        <MetricTile label="Thanh toán approved" value={<CurrencyText value={summary?.approved_payable_payment_total ?? payable.approved_payment_total ?? 0} />} />
        <MetricTile label="Chênh công thức" value={<CurrencyText value={summary?.balance_difference ?? 0} />} />
        <MetricTile label="Pending payment" value={String(Number(summary?.pending_payment_count ?? 0))} />
      </div>

      {warnings.length ? (
        <div className="grid gap-2 lg:grid-cols-2">
          {warnings.map((warning) => (
            <div key={warning.code} className={warningSeverityClass(warning.severity)}>
              <p className="font-semibold">{warning.message}</p>
            </div>
          ))}
        </div>
      ) : snapshot.isLoading ? null : (
        <div className="rounded-md border border-emerald-100 bg-emerald-50 px-3 py-2 text-sm font-semibold text-emerald-700">Đối soát hiện tại không có cảnh báo.</div>
      )}

      <div className="space-y-2">
        <h4 className="text-xs font-semibold uppercase text-slate-500">Sổ công nợ vendor</h4>
        <DataTable<PayableLedgerEntry>
          rows={ledger}
          loading={snapshot.isLoading}
          error={snapshot.error}
          onRetry={() => void snapshot.refetch()}
          empty="Chưa có bút toán công nợ"
          pageSize={8}
          defaultSort={{ key: 'occurred_at', direction: 'desc' }}
          columns={[
            { key: 'occurred_at', header: 'Ngày', sortable: true, render: (row) => dateText(row.occurred_at) },
            {
              key: 'description',
              header: 'Loại',
              render: (row) => (
                <div className="min-w-44">
                  <p className="font-semibold text-ink">{row.description}</p>
                  <p className="mt-1 text-xs text-slate-500">{row.source_code}</p>
                </div>
              ),
            },
            { key: 'status', header: 'Status', render: (row) => <StatusBadge status={row.status} /> },
            { key: 'debit_amount', header: 'Phát sinh nợ', align: 'right', render: (row) => <CurrencyText value={row.debit_amount} /> },
            { key: 'credit_amount', header: 'Giảm nợ', align: 'right', render: (row) => <CurrencyText value={row.credit_amount} /> },
            { key: 'document_amount', header: 'Chứng từ', align: 'right', render: (row) => <CurrencyText value={row.document_amount} /> },
            { key: 'cash_amount', header: 'Cash', align: 'right', render: (row) => <CurrencyText value={row.cash_amount} /> },
            { key: 'running_balance', header: 'Số dư', align: 'right', render: (row) => <CurrencyText value={row.running_balance} /> },
          ]}
        />
      </div>
    </section>
  );
}

function MetricTile({ label, value }: { label: string; value: ReactNode }) {
  return (
    <div className="rounded-md border border-line bg-slate-50 px-3 py-2">
      <p className="text-xs font-semibold uppercase text-slate-500">{label}</p>
      <div className="mt-1 text-sm font-semibold text-ink">{value}</div>
    </div>
  );
}
