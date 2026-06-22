import { type ReactNode } from 'react';
import { useQuery } from '@tanstack/react-query';
import { RefreshCw } from 'lucide-react';
import { CurrencyText } from '../../components/CurrencyText';
import { DataTable } from '../../components/DataTable';
import { StatusBadge } from '../../components/StatusBadge';
import { get } from '../../lib/api/client';
import type { ReceivableLedgerEntry, ReceivableReconciliationSnapshot, ReceivableRecord } from '../../lib/types/domain';
import { dateText } from '../../lib/utils/format';
import { agingBadgeClass, debtAgingLabel, warningSeverityClass } from './payableDebtUtils';

type ReceivableReconciliationPanelProps = {
  receivable: ReceivableRecord;
};

export function ReceivableReconciliationPanel({ receivable }: ReceivableReconciliationPanelProps) {
  const snapshot = useQuery({
    queryKey: ['receivable-reconciliation', receivable.id],
    queryFn: () => get<ReceivableReconciliationSnapshot>(`/receivables/${receivable.id}/reconciliation`),
  });

  const summary = snapshot.data?.summary;
  const warnings = snapshot.data?.warnings ?? [];
  const ledger = snapshot.data?.ledger ?? [];

  return (
    <section className="space-y-3">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div className="min-w-0">
          <h3 className="text-sm font-semibold text-ink">Đối soát công nợ đầu ra</h3>
          <p className="mt-1 text-xs text-slate-500">Aging, hoá đơn, lịch kỳ, ứng trước và receipt allocations theo receivable này.</p>
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
        <div className="rounded-md border border-rose-100 bg-rose-50 p-3 text-sm text-rose-700">Không tải được đối soát công nợ đầu ra.</div>
      ) : null}

      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <MetricTile label="Gross" value={<CurrencyText value={summary?.gross_amount ?? receivable.gross_amount} />} />
        <MetricTile label="Hoá đơn issued" value={<CurrencyText value={summary?.issued_invoice_total ?? receivable.issued_invoice_total ?? 0} />} />
        <MetricTile label="Còn phải thu" value={<CurrencyText value={summary?.remaining_amount ?? receivable.remaining_amount ?? 0} />} />
        <MetricTile
          label="Aging"
          value={<span className={agingBadgeClass(summary?.aging_bucket ?? receivable.aging_bucket)}>{debtAgingLabel(summary?.aging_bucket ?? receivable.aging_bucket, summary?.overdue_days ?? receivable.overdue_days)}</span>}
        />
      </div>

      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <MetricTile label="Lịch còn" value={<CurrencyText value={summary?.scheduled_remaining ?? receivable.scheduled_remaining ?? 0} />} />
        <MetricTile label="Ứng duyệt" value={<CurrencyText value={summary?.advance_approved_total ?? receivable.advance_approved_total ?? 0} />} />
        <MetricTile label="Ứng đã thu" value={<CurrencyText value={summary?.advance_received_total ?? receivable.advance_received_total ?? 0} />} />
        <MetricTile label="Đã quyết toán ứng" value={<CurrencyText value={summary?.advance_settled_total ?? receivable.advance_settled_total ?? 0} />} />
      </div>

      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <MetricTile label="Đã thu" value={<CurrencyText value={summary?.received_amount ?? receivable.received_amount ?? 0} />} />
        <MetricTile label="Phiếu thu approved" value={<CurrencyText value={summary?.approved_receipt_total ?? receivable.approved_receipt_total ?? 0} />} />
        <MetricTile label="Chênh công thức" value={<CurrencyText value={summary?.balance_difference ?? 0} />} />
        <MetricTile label="Pending receipt" value={String(Number(summary?.pending_receipt_count ?? 0))} />
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
        <h4 className="text-xs font-semibold uppercase text-slate-500">Sổ công nợ khách hàng</h4>
        <DataTable<ReceivableLedgerEntry>
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
            { key: 'debit_amount', header: 'Phát sinh phải thu', align: 'right', render: (row) => <CurrencyText value={row.debit_amount} /> },
            { key: 'credit_amount', header: 'Giảm phải thu', align: 'right', render: (row) => <CurrencyText value={row.credit_amount} /> },
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
