import { type ReactNode, useMemo, useState } from 'react';
import { useParams } from 'react-router-dom';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Plus } from 'lucide-react';
import { get, post } from '../../lib/api/client';
import { PageHeader } from '../../components/PageHeader';
import { DataTable } from '../../components/DataTable';
import { Dialog } from '../../components/Dialog';
import { CurrencyText } from '../../components/CurrencyText';
import { StatusBadge } from '../../components/StatusBadge';
import { FormDrawer } from '../../components/FormDrawer';
import { useToast } from '../../components/ToastProvider';
import { RowActions } from '../../components/RowActions';
import { ProjectBackLink } from '../../components/ProjectBackLink';
import { SimpleForm } from '../_shared/SimpleForm';
import { InputInvoicePanel } from './InputInvoicePanel';
import { InputPaymentPlanPanel } from './InputPaymentPlanPanel';
import { OutputBillingPlanPanel } from './OutputBillingPlanPanel';
import { OutputInvoicePanel } from './OutputInvoicePanel';
import { PayableReconciliationPanel } from './PayableReconciliationPanel';
import { ReceivableReconciliationPanel } from './ReceivableReconciliationPanel';
import type { Partner, PayableRecord, ReceivableRecord } from '../../lib/types/domain';
import { enumOptions } from '../../lib/utils/status';
import { dateText } from '../../lib/utils/format';
import { agingBadgeClass, debtAgingLabel, reconciliationClass, reconciliationLabel } from './payableDebtUtils';
import { partnerOptions, vendorPartnerTypes } from '../core-data/partnerUtils';

export function DebtPage({ kind }: { kind: 'payables' | 'receivables' | 'both' }) {
  const { id } = useParams();
  const [open, setOpen] = useState<'payable' | 'receivable' | null>(null);
  const [dialog, setDialog] = useState<{ type: 'payable'; record: PayableRecord } | { type: 'receivable'; record: ReceivableRecord } | null>(null);
  const queryClient = useQueryClient();
  const toast = useToast();
  const projects = useQuery({ queryKey: ['projects'], queryFn: () => get<Record<string, any>[]>('/projects') });
  const partners = useQuery({ queryKey: ['partners', 'active'], queryFn: () => get<Partner[]>('/partners?status=active') });
  const payableScope = id ? `?project_id=${encodeURIComponent(id)}` : '';
  const receivableScope = id ? `?project_id=${encodeURIComponent(id)}` : '';
  const payables = useQuery({ queryKey: ['payables', id ?? 'global'], queryFn: () => get<PayableRecord[]>(`/payables${payableScope}`), enabled: kind !== 'receivables' });
  const receivables = useQuery({ queryKey: ['receivables', id ?? 'global'], queryFn: () => get<ReceivableRecord[]>(`/receivables${receivableScope}`), enabled: kind !== 'payables' });
  const payableRows = payables.data ?? [];
  const receivableRows = receivables.data ?? [];
  const payableSummary = useMemo(() => summarizePayables(payableRows), [payableRows]);
  const receivableSummary = useMemo(() => summarizeReceivables(receivableRows), [receivableRows]);
  const invalidatePayables = (payableID?: string) => {
    queryClient.invalidateQueries({ queryKey: ['payables'] });
    if (payableID) queryClient.invalidateQueries({ queryKey: ['payable-reconciliation', payableID] });
  };
  const invalidateReceivables = (receivableID?: string) => {
    queryClient.invalidateQueries({ queryKey: ['receivables'] });
    if (receivableID) queryClient.invalidateQueries({ queryKey: ['receivable-reconciliation', receivableID] });
  };
  const createPayable = useMutation({
    mutationFn: (values: Record<string, any>) => post('/payables', values),
    onSuccess: () => {
      invalidatePayables();
      setOpen(null);
      toast.success('Đã ghi nhận phải trả');
    },
    onError: (error) => toast.error('Không ghi nhận được phải trả', error),
  });
  const createReceivable = useMutation({
    mutationFn: (values: Record<string, any>) => post('/receivables', values),
    onSuccess: () => {
      invalidateReceivables();
      setOpen(null);
      toast.success('Đã ghi nhận phải thu');
    },
    onError: (error) => toast.error('Không ghi nhận được phải thu', error),
  });
  return (
    <>
      <PageHeader
        title={kind === 'payables' ? 'Công nợ đầu vào' : kind === 'receivables' ? 'Công nợ đầu ra' : 'Công nợ'}
        description="Theo dõi phải trả, phải thu, quá hạn, tạm ứng, giữ lại, khấu trừ và các khoản thanh toán/thu tiền."
        actions={
          <>
            {id ? <ProjectBackLink projectId={id} /> : null}
            {kind !== 'receivables' ? <button className="inline-flex items-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold hover:bg-slate-100" onClick={() => setOpen('payable')}><Plus className="h-4 w-4" />Phải trả</button> : null}
            {kind !== 'payables' ? <button className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700" onClick={() => setOpen('receivable')}><Plus className="h-4 w-4" />Phải thu</button> : null}
          </>
        }
      />
      <div className={kind === 'both' ? 'grid gap-4 xl:grid-cols-2' : 'space-y-4'}>
        {kind !== 'receivables' ? (
          <section className="space-y-3">
            <h2 className="mb-3 text-base font-semibold text-ink">Payables</h2>
            <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-6">
              <DebtMetric label="Tổng còn phải trả" value={<CurrencyText value={payableSummary.remaining} />} />
              <DebtMetric label="Chưa đến hạn" value={<CurrencyText value={payableSummary.notDue} />} />
              <DebtMetric label="1-30 ngày" value={<CurrencyText value={payableSummary.bucket1To30} />} />
              <DebtMetric label="31-60 ngày" value={<CurrencyText value={payableSummary.bucket31To60} />} />
              <DebtMetric label=">90 ngày" value={<CurrencyText value={payableSummary.bucket90Plus} />} />
              <DebtMetric label="Cần đối soát" value={String(payableSummary.needsReview)} />
            </div>
            <DataTable
              rows={payableRows}
              loading={payables.isLoading}
              error={payables.error}
              onRetry={() => void payables.refetch()}
              empty="Chưa có phải trả đầu vào"
              searchPlaceholder="Tìm mã, vendor, công trình..."
              pageSize={10}
              defaultSort={{ key: 'effective_due_date', direction: 'asc' }}
              columns={[
              { key: 'payable_code', header: 'Mã', sortable: true },
              { key: 'vendor_name', header: 'Vendor', render: (row) => <span className="font-semibold text-ink">{row.vendor_name}</span> },
              { key: 'project_name', header: 'Công trình' },
              { key: 'gross_amount', header: 'Gross', render: (row) => <CurrencyText value={row.gross_amount} /> },
              { key: 'remaining_amount', header: 'Còn phải trả', render: (row) => <CurrencyText value={row.remaining_amount} /> },
              { key: 'effective_due_date', header: 'Due', sortable: true, render: (row) => dateText(row.effective_due_date ?? row.due_date) },
              {
                key: 'aging_bucket',
                header: 'Aging',
                sortable: true,
                render: (row) => <span className={agingBadgeClass(row.aging_bucket)}>{debtAgingLabel(row.aging_bucket, row.overdue_days)}</span>,
              },
              {
                key: 'invoice_count',
                header: 'Hoá đơn',
                render: (row) => (
                  <div className="min-w-32">
                    <p className="font-semibold text-ink">{Number(row.invoice_count ?? 0)} PDF</p>
                    <p className="mt-1 text-xs text-slate-500"><CurrencyText value={row.verified_invoice_total ?? 0} /> verified</p>
                  </div>
                ),
              },
              {
                key: 'reconciliation_status',
                header: 'Đối soát',
                render: (row) => <span className={reconciliationClass(row.reconciliation_status, row.reconciliation_issue_count)}>{reconciliationLabel(row.reconciliation_status, row.reconciliation_issue_count)}</span>,
              },
              { key: 'status', header: 'Trạng thái', render: (row) => <StatusBadge status={row.status} /> },
              { key: 'action', header: 'Action', render: (row) => <RowActions onView={() => setDialog({ type: 'payable', record: row })} /> },
            ]} />
          </section>
        ) : null}
        {kind !== 'payables' ? (
          <section className="space-y-3">
            <h2 className="mb-3 text-base font-semibold text-ink">Receivables</h2>
            <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-6">
              <DebtMetric label="Tổng còn phải thu" value={<CurrencyText value={receivableSummary.remaining} />} />
              <DebtMetric label="Chưa đến hạn" value={<CurrencyText value={receivableSummary.notDue} />} />
              <DebtMetric label="1-30 ngày" value={<CurrencyText value={receivableSummary.bucket1To30} />} />
              <DebtMetric label="31-60 ngày" value={<CurrencyText value={receivableSummary.bucket31To60} />} />
              <DebtMetric label=">90 ngày" value={<CurrencyText value={receivableSummary.bucket90Plus} />} />
              <DebtMetric label="Cần đối soát" value={String(receivableSummary.needsReview)} />
            </div>
            <DataTable
              rows={receivableRows}
              loading={receivables.isLoading}
              error={receivables.error}
              onRetry={() => void receivables.refetch()}
              empty="Chưa có phải thu đầu ra"
              searchPlaceholder="Tìm mã, khách hàng, công trình..."
              pageSize={10}
              defaultSort={{ key: 'effective_due_date', direction: 'asc' }}
              columns={[
                { key: 'receivable_code', header: 'Mã', sortable: true },
                { key: 'client_name', header: 'Khách hàng', render: (row) => <span className="font-semibold text-ink">{row.client_name}</span> },
                { key: 'project_name', header: 'Công trình' },
                { key: 'milestone_name', header: 'Milestone' },
                { key: 'gross_amount', header: 'Gross', align: 'right', render: (row) => <CurrencyText value={row.gross_amount} /> },
                { key: 'remaining_amount', header: 'Còn phải thu', align: 'right', render: (row) => <CurrencyText value={row.remaining_amount} /> },
                { key: 'effective_due_date', header: 'Due', sortable: true, render: (row) => dateText(row.effective_due_date ?? row.due_date) },
                {
                  key: 'aging_bucket',
                  header: 'Aging',
                  sortable: true,
                  render: (row) => <span className={agingBadgeClass(row.aging_bucket)}>{debtAgingLabel(row.aging_bucket, row.overdue_days)}</span>,
                },
                {
                  key: 'invoice_count',
                  header: 'Hoá đơn',
                  render: (row) => (
                    <div className="min-w-32">
                      <p className="font-semibold text-ink">{Number(row.invoice_count ?? 0)} PDF</p>
                      <p className="mt-1 text-xs text-slate-500"><CurrencyText value={row.issued_invoice_total ?? 0} /> issued</p>
                    </div>
                  ),
                },
                {
                  key: 'received_amount',
                  header: 'Đã thu/Ứng',
                  align: 'right',
                  render: (row) => (
                    <div className="min-w-32 text-right">
                      <p className="font-semibold text-ink"><CurrencyText value={row.received_amount ?? 0} /></p>
                      <p className="mt-1 text-xs text-slate-500">Ứng <CurrencyText value={row.advance_approved_total ?? 0} /></p>
                    </div>
                  ),
                },
                {
                  key: 'reconciliation_status',
                  header: 'Đối soát',
                  render: (row) => <span className={reconciliationClass(row.reconciliation_status, row.reconciliation_issue_count)}>{reconciliationLabel(row.reconciliation_status, row.reconciliation_issue_count)}</span>,
                },
                { key: 'status', header: 'Trạng thái', render: (row) => <StatusBadge status={row.status} /> },
                { key: 'action', header: 'Action', render: (row) => <RowActions onView={() => setDialog({ type: 'receivable', record: row })} /> },
              ]}
            />
          </section>
        ) : null}
      </div>
      <FormDrawer open={open === 'payable'} title="Ghi nhận phải trả" onClose={() => setOpen(null)}>
        <SimpleForm fields={[
          { name: 'project_id', label: 'Công trình', type: 'select', defaultValue: id ?? '', required: true, options: (projects.data ?? []).map((p) => ({ label: p.name, value: p.id })) },
          { name: 'payable_code', label: 'Mã phải trả', defaultValue: `PAY-${Date.now().toString().slice(-5)}`, required: true },
          { name: 'vendor_type', label: 'Loại vendor', type: 'select', defaultValue: 'supplier', required: true, options: enumOptions('vendor_type', ['supplier', 'subcontractor', 'labor_team', 'transport', 'equipment_rental', 'service_provider', 'other']) },
          {
            name: 'vendor_partner_id',
            label: 'Vendor catalog',
            type: 'select',
            options: partnerOptions(partners.data ?? [], vendorPartnerTypes),
            helperText: 'Chọn catalog để snapshot tên vendor vào công nợ.',
          },
          {
            name: 'vendor_name',
            label: 'Vendor',
            defaultValue: 'Nhà cung cấp mới',
            required: true,
            hiddenWhen: (values: Record<string, any>) => Boolean(values.vendor_partner_id),
          },
          { name: 'gross_amount', label: 'Gross', type: 'number', defaultValue: 10000000, required: true, min: 0 },
          { name: 'due_date', label: 'Hạn thanh toán', type: 'date' },
          { name: 'advance_amount', label: 'Tạm ứng', type: 'number', defaultValue: 0, min: 0 },
          { name: 'retention_amount', label: 'Giữ lại', type: 'number', defaultValue: 0, min: 0 },
          { name: 'deduction_amount', label: 'Khấu trừ', type: 'number', defaultValue: 0, min: 0 },
          { name: 'paid_amount', label: 'Đã trả', type: 'number', defaultValue: 0, min: 0 },
          { name: 'remaining_amount', label: 'Còn lại', type: 'number', defaultValue: 10000000, min: 0 },
          { name: 'status', label: 'Trạng thái', type: 'select', defaultValue: 'recorded', required: true, options: enumOptions('status', ['draft', 'recorded', 'approved', 'partially_paid', 'paid']) },
        ]} onSubmit={(values) => createPayable.mutateAsync(values)} />
      </FormDrawer>
      <FormDrawer open={open === 'receivable'} title="Ghi nhận phải thu" onClose={() => setOpen(null)}>
        <SimpleForm fields={[
          { name: 'project_id', label: 'Công trình', type: 'select', required: true, options: (projects.data ?? []).map((p) => ({ label: p.name, value: p.id })) },
          { name: 'receivable_code', label: 'Mã phải thu', defaultValue: `REC-${Date.now().toString().slice(-5)}`, required: true },
          {
            name: 'client_partner_id',
            label: 'Khách hàng catalog',
            type: 'select',
            options: partnerOptions(partners.data ?? [], ['client']),
            helperText: 'Chọn catalog để snapshot tên khách hàng vào công nợ.',
          },
          {
            name: 'client_name',
            label: 'Khách hàng',
            defaultValue: 'Khách hàng',
            required: true,
            hiddenWhen: (values: Record<string, any>) => Boolean(values.client_partner_id),
          },
          { name: 'contract_amount', label: 'Giá trị hợp đồng', type: 'number', defaultValue: 100000000, min: 0 },
          { name: 'milestone_name', label: 'Milestone', defaultValue: 'Đợt thanh toán', required: true },
          { name: 'gross_amount', label: 'Gross', type: 'number', defaultValue: 30000000, required: true, min: 0 },
          { name: 'due_date', label: 'Hạn thu', type: 'date' },
          { name: 'received_amount', label: 'Đã thu', type: 'number', defaultValue: 0, min: 0 },
          { name: 'remaining_amount', label: 'Còn lại', type: 'number', defaultValue: 30000000, min: 0 },
          { name: 'status', label: 'Trạng thái', type: 'select', defaultValue: 'recorded', required: true, options: enumOptions('status', ['draft', 'recorded', 'approved', 'partially_received', 'received']) },
        ]} onSubmit={(values) => createReceivable.mutateAsync(values)} />
      </FormDrawer>
      <Dialog open={dialog?.type === 'payable'} title="Chi tiết phải trả" onClose={() => setDialog(null)} size="xl">
        {dialog?.type === 'payable' ? (
          <div className="space-y-5">
            <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
              <DebtMetric label="Mã" value={dialog.record.payable_code} />
              <DebtMetric label="Vendor" value={dialog.record.vendor_name} />
              <DebtMetric label="Còn phải trả" value={<CurrencyText value={dialog.record.remaining_amount} />} />
              <DebtMetric label="Trạng thái" value={<StatusBadge status={dialog.record.status} />} />
            </div>
            <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
              <DebtMetric label="Công trình" value={dialog.record.project_name ?? '-'} />
              <DebtMetric label="Gross" value={<CurrencyText value={dialog.record.gross_amount} />} />
              <DebtMetric label="Tổng hoá đơn" value={<CurrencyText value={dialog.record.invoice_total ?? 0} />} />
              <DebtMetric label="Lịch kỳ" value={<CurrencyText value={dialog.record.scheduled_remaining ?? 0} />} />
              <DebtMetric label="Ứng trước" value={<CurrencyText value={dialog.record.advance_requested_total ?? 0} />} />
            </div>
            <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
              <DebtMetric label="Đã trả" value={<CurrencyText value={dialog.record.paid_amount ?? 0} />} />
              <DebtMetric label="Đã quyết toán ứng" value={<CurrencyText value={dialog.record.advance_settled_total ?? 0} />} />
              <DebtMetric label="Giữ lại" value={<CurrencyText value={dialog.record.retention_amount ?? 0} />} />
              <DebtMetric
                label="Mới nhất"
                value={dialog.record.latest_invoice_number ? `${dialog.record.latest_invoice_number} · ${dateText(dialog.record.latest_invoice_date)}` : '-'}
              />
            </div>
            <PayableReconciliationPanel payable={dialog.record} />
            <InputInvoicePanel
              payable={dialog.record}
              onChanged={() => {
                invalidatePayables(dialog.record.id);
              }}
            />
            <InputPaymentPlanPanel
              payable={dialog.record}
              onChanged={() => {
                invalidatePayables(dialog.record.id);
              }}
            />
          </div>
        ) : null}
      </Dialog>
      <Dialog open={dialog?.type === 'receivable'} title="Chi tiết phải thu" onClose={() => setDialog(null)} size="xl">
        {dialog?.type === 'receivable' ? (
          <div className="space-y-5">
            <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
              <DebtMetric label="Mã" value={dialog.record.receivable_code} />
              <DebtMetric label="Khách hàng" value={dialog.record.client_name} />
              <DebtMetric label="Còn phải thu" value={<CurrencyText value={dialog.record.remaining_amount} />} />
              <DebtMetric label="Trạng thái" value={<StatusBadge status={dialog.record.status} />} />
            </div>
            <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
              <DebtMetric label="Công trình" value={dialog.record.project_name ?? '-'} />
              <DebtMetric label="Gross" value={<CurrencyText value={dialog.record.gross_amount} />} />
              <DebtMetric label="Tổng hoá đơn" value={<CurrencyText value={dialog.record.invoice_total ?? 0} />} />
              <DebtMetric label="Lịch kỳ" value={<CurrencyText value={dialog.record.scheduled_remaining ?? 0} />} />
              <DebtMetric label="Ứng duyệt" value={<CurrencyText value={dialog.record.advance_approved_total ?? 0} />} />
            </div>
            <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
              <DebtMetric label="Đã thu" value={<CurrencyText value={dialog.record.received_amount ?? 0} />} />
              <DebtMetric label="Đã quyết toán ứng" value={<CurrencyText value={dialog.record.advance_settled_total ?? 0} />} />
              <DebtMetric label="Hạn thu" value={dateText(dialog.record.effective_due_date ?? dialog.record.due_date)} />
              <DebtMetric label="Aging" value={<span className={agingBadgeClass(dialog.record.aging_bucket)}>{debtAgingLabel(dialog.record.aging_bucket, dialog.record.overdue_days)}</span>} />
            </div>
            <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
              <DebtMetric label="Đối soát" value={<span className={reconciliationClass(dialog.record.reconciliation_status, dialog.record.reconciliation_issue_count)}>{reconciliationLabel(dialog.record.reconciliation_status, dialog.record.reconciliation_issue_count)}</span>} />
              <DebtMetric
                label="Mới nhất"
                value={dialog.record.latest_invoice_number ? `${dialog.record.latest_invoice_number} · ${dateText(dialog.record.latest_invoice_date)}` : '-'}
              />
            </div>
            <ReceivableReconciliationPanel receivable={dialog.record} />
            <OutputInvoicePanel
              receivable={dialog.record}
              onChanged={() => {
                invalidateReceivables(dialog.record.id);
              }}
            />
            <OutputBillingPlanPanel
              receivable={dialog.record}
              onChanged={() => {
                invalidateReceivables(dialog.record.id);
              }}
            />
          </div>
        ) : null}
      </Dialog>
    </>
  );
}

function summarizePayables(rows: PayableRecord[]) {
  return rows.reduce(
    (summary, row) => {
      const remaining = Number(row.remaining_amount ?? 0);
      summary.remaining += remaining;
      if (row.aging_bucket === 'not_due') summary.notDue += remaining;
      if (row.aging_bucket === '1_30') summary.bucket1To30 += remaining;
      if (row.aging_bucket === '31_60') summary.bucket31To60 += remaining;
      if (row.aging_bucket === '90_plus') summary.bucket90Plus += remaining;
      if (row.reconciliation_status === 'needs_review' || Number(row.reconciliation_issue_count ?? 0) > 0) summary.needsReview += 1;
      return summary;
    },
    { remaining: 0, notDue: 0, bucket1To30: 0, bucket31To60: 0, bucket90Plus: 0, needsReview: 0 },
  );
}

function summarizeReceivables(rows: ReceivableRecord[]) {
  return rows.reduce(
    (summary, row) => {
      const remaining = Number(row.remaining_amount ?? 0);
      summary.remaining += remaining;
      if (row.aging_bucket === 'not_due') summary.notDue += remaining;
      if (row.aging_bucket === '1_30') summary.bucket1To30 += remaining;
      if (row.aging_bucket === '31_60') summary.bucket31To60 += remaining;
      if (row.aging_bucket === '90_plus') summary.bucket90Plus += remaining;
      if (row.reconciliation_status === 'needs_review' || Number(row.reconciliation_issue_count ?? 0) > 0) summary.needsReview += 1;
      return summary;
    },
    { remaining: 0, notDue: 0, bucket1To30: 0, bucket31To60: 0, bucket90Plus: 0, needsReview: 0 },
  );
}

function DebtMetric({ label, value }: { label: string; value: ReactNode }) {
  return (
    <div className="rounded-md border border-line bg-slate-50 px-3 py-2">
      <p className="text-xs font-semibold uppercase text-slate-500">{label}</p>
      <div className="mt-1 truncate text-sm font-semibold text-ink">{value}</div>
    </div>
  );
}
