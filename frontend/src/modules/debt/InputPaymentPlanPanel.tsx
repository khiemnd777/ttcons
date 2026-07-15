import { type ReactNode, useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Check, HandCoins, Plus, RefreshCw } from 'lucide-react';
import { CurrencyText } from '../../components/CurrencyText';
import { DataTable } from '../../components/DataTable';
import { Dialog } from '../../components/Dialog';
import { EmptyState } from '../../components/EmptyState';
import { EnumChip } from '../../components/EnumChip';
import { RowActions } from '../../components/RowActions';
import { StatusBadge } from '../../components/StatusBadge';
import { useToast } from '../../components/ToastProvider';
import { get, post } from '../../lib/api/client';
import { useAuth } from '../../lib/auth/AuthContext';
import type {
  InputInvoiceRecord,
  PayableAdvanceRequestRecord,
  PayablePaymentRecord,
  PayablePaymentScheduleRecord,
  PayableRecord,
} from '../../lib/types/domain';
import { dateText } from '../../lib/utils/format';
import { enumOptions } from '../../lib/utils/status';
import { SimpleForm } from '../_shared/SimpleForm';

type InputPaymentPlanPanelProps = {
  payable: PayableRecord;
  onChanged?: () => void;
  view?: 'all' | 'plan' | 'payments';
};

type OpenDialog =
  | { type: 'schedule' }
  | { type: 'advance' }
  | { type: 'payment' }
  | { type: 'settlement'; advance: PayableAdvanceRequestRecord }
  | null;

export function InputPaymentPlanPanel({ payable, onChanged, view = 'all' }: InputPaymentPlanPanelProps) {
  const [openDialog, setOpenDialog] = useState<OpenDialog>(null);
  const queryClient = useQueryClient();
  const toast = useToast();
  const { hasAnyPermission } = useAuth();
  const canCreatePlan = hasAnyPermission(['accounting.payment_create', 'accounting.payable_approve']);
  const canApprovePlan = hasAnyPermission(['accounting.payable_approve', 'accounting.payment_approve']);
  const canCreatePayment = hasAnyPermission(['accounting.payment_create']);
  const canApprovePayment = hasAnyPermission(['accounting.payment_approve']);

  const schedules = useQuery({
    queryKey: ['payable-payment-schedules', payable.id],
    queryFn: () => get<PayablePaymentScheduleRecord[]>(`/payables/${payable.id}/payment-schedules`),
  });
  const advances = useQuery({
    queryKey: ['payable-advance-requests', payable.id],
    queryFn: () => get<PayableAdvanceRequestRecord[]>(`/payables/${payable.id}/advance-requests`),
  });
  const payments = useQuery({
    queryKey: ['payable-payments', payable.id],
    queryFn: () => get<PayablePaymentRecord[]>(`/payables/${payable.id}/payments`),
  });
  const invoices = useQuery({
    queryKey: ['input-invoices', payable.id],
    queryFn: () => get<InputInvoiceRecord[]>(`/payables/${payable.id}/input-invoices`),
  });

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ['payable-payment-schedules', payable.id] });
    queryClient.invalidateQueries({ queryKey: ['payable-advance-requests', payable.id] });
    queryClient.invalidateQueries({ queryKey: ['payable-payments', payable.id] });
    queryClient.invalidateQueries({ queryKey: ['payables'] });
    queryClient.invalidateQueries({ queryKey: ['payments'] });
    onChanged?.();
  };

  const createSchedule = useMutation({
    mutationFn: (values: Record<string, any>) => post<PayablePaymentScheduleRecord>(`/payables/${payable.id}/payment-schedules`, values),
    onSuccess: () => {
      setOpenDialog(null);
      invalidate();
      toast.success('Đã tạo kỳ thanh toán đầu vào');
    },
    onError: (error) => toast.error('Không tạo được kỳ thanh toán', error),
  });
  const approveSchedule = useMutation({
    mutationFn: (scheduleID: string) => post<PayablePaymentScheduleRecord>(`/payment-schedules/${scheduleID}/approve`, {}),
    onSuccess: () => {
      invalidate();
      toast.success('Đã duyệt kỳ thanh toán');
    },
    onError: (error) => toast.error('Không duyệt được kỳ thanh toán', error),
  });
  const createAdvance = useMutation({
    mutationFn: (values: Record<string, any>) => post<PayableAdvanceRequestRecord>(`/payables/${payable.id}/advance-requests`, values),
    onSuccess: () => {
      setOpenDialog(null);
      invalidate();
      toast.success('Đã tạo yêu cầu ứng trước');
    },
    onError: (error) => toast.error('Không tạo được ứng trước', error),
  });
  const approveAdvance = useMutation({
    mutationFn: (advanceID: string) => post<PayableAdvanceRequestRecord>(`/advance-requests/${advanceID}/approve`, {}),
    onSuccess: () => {
      invalidate();
      toast.success('Đã duyệt ứng trước');
    },
    onError: (error) => toast.error('Không duyệt được ứng trước', error),
  });
  const settleAdvance = useMutation({
    mutationFn: ({ advanceID, values }: { advanceID: string; values: Record<string, any> }) => post<PayableAdvanceRequestRecord>(`/advance-requests/${advanceID}/settle`, values),
    onSuccess: () => {
      setOpenDialog(null);
      invalidate();
      toast.success('Đã quyết toán ứng trước');
    },
    onError: (error) => toast.error('Không quyết toán được ứng trước', error),
  });
  const createPayment = useMutation({
    mutationFn: (values: Record<string, any>) => post<PayablePaymentRecord>(`/payables/${payable.id}/payments`, values),
    onSuccess: () => {
      setOpenDialog(null);
      invalidate();
      toast.success('Đã tạo phiếu thanh toán từng phần');
    },
    onError: (error) => toast.error('Không tạo được phiếu thanh toán', error),
  });
  const approvePayment = useMutation({
    mutationFn: (paymentID: string) => post<PayablePaymentRecord>(`/payments/${paymentID}/approve`, {}),
    onSuccess: () => {
      invalidate();
      toast.success('Đã duyệt phiếu thanh toán');
    },
    onError: (error) => toast.error('Không duyệt được phiếu thanh toán', error),
  });

  const scheduleRows = schedules.data ?? [];
  const advanceRows = advances.data ?? [];
  const paymentRows = payments.data ?? [];
  const verifiedInvoiceOptions = (invoices.data ?? [])
    .filter((invoice) => invoice.status === 'verified')
    .map((invoice) => ({ label: `${invoice.invoice_number} · ${dateText(invoice.invoice_date)}`, value: invoice.id }));
  const payablePaymentType = payable.vendor_type === 'subcontractor' || payable.vendor_type === 'labor_team' ? 'subcontractor_payment' : 'supplier_payment';
  const activeScheduleOptions = scheduleRows
    .filter((row) => ['approved', 'partially_paid', 'overdue'].includes(row.status) && Number(row.remaining_amount ?? 0) > 0)
    .map((row) => ({ label: `${row.schedule_code} · ${row.milestone_name} · ${moneyLabel(row.remaining_amount)}`, value: row.id }));
  const payableAdvanceOptions = advanceRows
    .filter((row) => ['approved', 'partially_settled'].includes(row.status) && advanceUnpaid(row) > 0)
    .map((row) => ({ label: `${row.request_code} · còn chi ${moneyLabel(advanceUnpaid(row))}`, value: row.id }));

  const summary = useMemo(() => {
    const planned = scheduleRows.reduce((sum, row) => sum + Number(row.planned_amount ?? 0), 0);
    const scheduleRemaining = scheduleRows.reduce((sum, row) => sum + Number(row.remaining_amount ?? 0), 0);
    const advanceApproved = advanceRows.filter((row) => ['approved', 'partially_settled', 'settled'].includes(row.status)).reduce((sum, row) => sum + Number(row.amount ?? 0), 0);
    const advanceSettled = advanceRows.reduce((sum, row) => sum + Number(row.settled_amount ?? 0), 0);
    return { planned, scheduleRemaining, advanceApproved, advanceSettled };
  }, [advanceRows, scheduleRows]);

  const loading = schedules.isLoading || advances.isLoading || payments.isLoading;
  const error = schedules.error || advances.error || payments.error;

  return (
    <section className="space-y-3">
      {view !== 'payments' ? <div className="grid gap-3 sm:grid-cols-4">
        <MetricTile label="Kế hoạch kỳ" value={<CurrencyText value={summary.planned} />} />
        <MetricTile label="Còn theo kỳ" value={<CurrencyText value={summary.scheduleRemaining} />} />
        <MetricTile label="Ứng trước duyệt" value={<CurrencyText value={summary.advanceApproved} />} />
        <MetricTile label="Đã quyết toán" value={<CurrencyText value={summary.advanceSettled} />} />
      </div> : null}

      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div className="min-w-0">
          <h3 className="text-sm font-semibold text-ink">{view === 'payments' ? 'Phiếu thanh toán đầu vào' : 'Lịch thanh toán và ứng trước'}</h3>
          <p className="mt-1 text-xs text-slate-500">Phiếu chỉ cập nhật phải trả sau khi được duyệt và phân bổ đúng nguồn.</p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <button type="button" className="inline-flex items-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100" onClick={() => void Promise.all([schedules.refetch(), advances.refetch(), payments.refetch()])}>
            <RefreshCw className={loading ? 'h-4 w-4 animate-spin' : 'h-4 w-4'} />
            Tải lại
          </button>
          {view !== 'payments' && canCreatePlan ? <button type="button" className="inline-flex items-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100" onClick={() => setOpenDialog({ type: 'schedule' })}>
            <Plus className="h-4 w-4" />
            Kỳ
          </button> : null}
          {view !== 'payments' && canCreatePlan ? <button type="button" className="inline-flex items-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100" onClick={() => setOpenDialog({ type: 'advance' })}>
            <HandCoins className="h-4 w-4" />
            Ứng trước
          </button> : null}
          {view !== 'plan' && canCreatePayment ? <button type="button" className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700" onClick={() => setOpenDialog({ type: 'payment' })}>
            <Plus className="h-4 w-4" />
            Thanh toán
          </button> : null}
        </div>
      </div>

      {error ? (
        <div className="rounded-md border border-rose-100 bg-rose-50 p-3 text-sm text-rose-700">Không tải được dữ liệu thanh toán đầu vào.</div>
      ) : null}

      {view !== 'payments' ? <div className="grid gap-4 xl:grid-cols-2">
        <div className="space-y-2">
          <h4 className="text-xs font-semibold uppercase text-slate-500">Lịch thanh toán</h4>
          <DataTable
            rows={scheduleRows}
            loading={schedules.isLoading}
            empty="Chưa có kỳ thanh toán"
            pageSize={5}
            defaultSort={{ key: 'due_date', direction: 'asc' }}
            columns={[
              { key: 'schedule_code', header: 'Mã', sortable: true },
              { key: 'schedule_type', header: 'Loại', render: (row) => <EnumChip group="payment_schedule_type" value={row.schedule_type} /> },
              { key: 'milestone_name', header: 'Kỳ', render: (row) => <span className="font-semibold text-ink">{row.milestone_name}</span> },
              { key: 'due_date', header: 'Hạn', sortable: true, render: (row) => dateText(row.due_date) },
              { key: 'planned_amount', header: 'Kế hoạch', align: 'right', render: (row) => <CurrencyText value={row.planned_amount} /> },
              { key: 'remaining_amount', header: 'Còn', align: 'right', render: (row) => <CurrencyText value={row.remaining_amount} /> },
              { key: 'status', header: 'Trạng thái', render: (row) => <StatusBadge status={row.status} /> },
              {
                key: 'actions',
                header: 'Thao tác',
                render: (row) => (
                  <RowActions>
                    {canApprovePlan ? <button className="text-sm font-semibold text-blue-700 disabled:cursor-not-allowed disabled:opacity-40" disabled={row.status !== 'draft' || approveSchedule.isPending} onClick={() => approveSchedule.mutate(row.id)}>
                      <Check className="mr-1 inline h-4 w-4" />
                      Duyệt
                    </button> : null}
                  </RowActions>
                ),
              },
            ]}
          />
        </div>

        <div className="space-y-2">
          <h4 className="text-xs font-semibold uppercase text-slate-500">Ứng trước</h4>
          <DataTable
            rows={advanceRows}
            loading={advances.isLoading}
            empty="Chưa có ứng trước"
            pageSize={5}
            defaultSort={{ key: 'due_date', direction: 'asc' }}
            columns={[
              { key: 'request_code', header: 'Mã', sortable: true },
              { key: 'amount', header: 'Số tiền', align: 'right', render: (row) => <CurrencyText value={row.amount} /> },
              { key: 'paid_amount', header: 'Đã chi', align: 'right', render: (row) => <CurrencyText value={row.paid_amount ?? 0} /> },
              { key: 'settled_amount', header: 'Đã quyết toán', align: 'right', render: (row) => <CurrencyText value={row.settled_amount ?? 0} /> },
              { key: 'due_date', header: 'Hạn', sortable: true, render: (row) => dateText(row.due_date) },
              { key: 'status', header: 'Trạng thái', render: (row) => <StatusBadge status={row.status} /> },
              {
                key: 'actions',
                header: 'Thao tác',
                render: (row) => (
                  <RowActions>
                    {canApprovePlan ? <button className="text-sm font-semibold text-blue-700 disabled:cursor-not-allowed disabled:opacity-40" disabled={row.status !== 'draft' || approveAdvance.isPending} onClick={() => approveAdvance.mutate(row.id)}>
                      Duyệt
                    </button> : null}
                    {canApprovePlan ? <button className="text-sm font-semibold text-teal-700 disabled:cursor-not-allowed disabled:opacity-40" disabled={!['approved', 'partially_settled'].includes(row.status)} onClick={() => setOpenDialog({ type: 'settlement', advance: row })}>
                      Quyết toán
                    </button> : null}
                  </RowActions>
                ),
              },
            ]}
          />
        </div>
      </div> : null}

      {view !== 'plan' ? <div className="space-y-2">
        <h4 className="text-xs font-semibold uppercase text-slate-500">Phiếu thanh toán đầu vào</h4>
        {paymentRows.length || payments.isLoading ? (
          <DataTable
            rows={paymentRows}
            loading={payments.isLoading}
            empty="Chưa có phiếu thanh toán"
            pageSize={5}
            defaultSort={{ key: 'created_at', direction: 'desc' }}
            columns={[
              { key: 'payment_code', header: 'Mã', sortable: true },
              { key: 'payment_type', header: 'Loại', render: (row) => <EnumChip group="payment_type" value={row.payment_type} /> },
              { key: 'allocation_type', header: 'Phân bổ', render: (row) => <EnumChip group="payment_allocation_type" value={row.allocation_type ?? 'payable_payment'} /> },
              { key: 'schedule_code', header: 'Kỳ/ứng trước', render: (row) => row.schedule_code ?? row.advance_request_code ?? '-' },
              { key: 'input_invoice_number', header: 'Hoá đơn', render: (row) => row.input_invoice_number ?? '-' },
              { key: 'amount', header: 'Số tiền', align: 'right', render: (row) => <CurrencyText value={row.amount} /> },
              { key: 'status', header: 'Trạng thái', render: (row) => <StatusBadge status={row.status} /> },
              {
                key: 'actions',
                header: 'Thao tác',
                render: (row) => (
                  <RowActions>
                    {canApprovePayment ? <button className="text-sm font-semibold text-blue-700 disabled:cursor-not-allowed disabled:opacity-40" disabled={!['draft', 'submitted', 'pending'].includes(row.status) || approvePayment.isPending} onClick={() => approvePayment.mutate(row.id)}>
                      <Check className="mr-1 inline h-4 w-4" />
                      Duyệt
                    </button> : null}
                  </RowActions>
                ),
              },
            ]}
          />
        ) : (
          <EmptyState title="Chưa có phiếu thanh toán" />
        )}
      </div> : null}

      <Dialog open={openDialog?.type === 'schedule'} title="Tạo kỳ thanh toán đầu vào" onClose={() => setOpenDialog(null)} size="lg">
        <SimpleForm
          layout="two-column"
          fields={[
            { name: 'schedule_code', label: 'Mã kỳ', defaultValue: `SCH-${Date.now().toString().slice(-5)}`, required: true },
            { name: 'schedule_type', label: 'Loại kỳ', type: 'select', defaultValue: 'progress', required: true, options: enumOptions('payment_schedule_type', ['advance', 'progress', 'final', 'retention', 'other']) },
            { name: 'milestone_name', label: 'Tên kỳ', defaultValue: 'Kỳ thanh toán', required: true, fullWidth: true },
            { name: 'planned_amount', label: 'Số tiền kế hoạch', type: 'number', defaultValue: Number(payable.remaining_amount ?? payable.gross_amount ?? 0), required: true, min: 1 },
            { name: 'due_date', label: 'Hạn thanh toán', type: 'date' },
            { name: 'notes', label: 'Ghi chú', type: 'textarea', fullWidth: true },
          ]}
          submitLabel="Tạo kỳ"
          onSubmit={(values) => createSchedule.mutateAsync(values)}
        />
      </Dialog>

      <Dialog open={openDialog?.type === 'advance'} title="Tạo ứng trước đầu vào" onClose={() => setOpenDialog(null)} size="lg">
        <SimpleForm
          layout="two-column"
          fields={[
            { name: 'request_code', label: 'Mã ứng trước', defaultValue: `ADV-${Date.now().toString().slice(-5)}`, required: true },
            { name: 'amount', label: 'Số tiền', type: 'number', defaultValue: Number(payable.remaining_amount ?? 0), required: true, min: 1 },
            { name: 'due_date', label: 'Hạn thanh toán', type: 'date' },
            { name: 'reason', label: 'Lý do', type: 'textarea', fullWidth: true },
            { name: 'notes', label: 'Ghi chú', type: 'textarea', fullWidth: true },
          ]}
          submitLabel="Tạo ứng trước"
          onSubmit={(values) => createAdvance.mutateAsync(values)}
        />
      </Dialog>

      <Dialog open={openDialog?.type === 'payment'} title="Tạo thanh toán từng phần" onClose={() => setOpenDialog(null)} size="lg">
        <SimpleForm
          layout="two-column"
          fields={[
            { name: 'payment_code', label: 'Mã phiếu', defaultValue: `PMT-${Date.now().toString().slice(-5)}`, required: true },
            { name: 'payment_type', label: 'Loại', type: 'select', defaultValue: payablePaymentType, required: true, options: enumOptions('payment_type', ['supplier_payment', 'subcontractor_payment', 'advance', 'other']) },
            { name: 'amount', label: 'Số tiền', type: 'number', defaultValue: Number(payable.remaining_amount ?? 0), required: true, min: 1 },
            { name: 'payment_method', label: 'Phương thức', type: 'select', defaultValue: 'bank_transfer', required: true, options: enumOptions('payment_method', ['bank_transfer', 'cash']) },
            { name: 'schedule_id', label: 'Kỳ thanh toán', type: 'select', options: activeScheduleOptions },
            {
              name: 'advance_request_id',
              label: 'Ứng trước',
              type: 'select',
              options: payableAdvanceOptions,
              validate: (value, values) => value && values.schedule_id ? 'Chỉ chọn kỳ thanh toán hoặc ứng trước' : undefined,
            },
            { name: 'input_invoice_id', label: 'Hoá đơn verified', type: 'select', options: verifiedInvoiceOptions },
            { name: 'paid_at', label: 'Ngày thanh toán', type: 'date', defaultValue: new Date().toISOString().slice(0, 10) },
            { name: 'notes', label: 'Ghi chú', type: 'textarea', fullWidth: true },
          ]}
          submitLabel="Tạo phiếu"
          onSubmit={(values) => createPayment.mutateAsync(values)}
        />
      </Dialog>

      <Dialog open={openDialog?.type === 'settlement'} title="Quyết toán ứng trước" onClose={() => setOpenDialog(null)} size="lg">
        {openDialog?.type === 'settlement' ? (
          <SimpleForm
            layout="two-column"
            fields={[
              {
                name: 'amount',
                label: 'Số tiền quyết toán',
                type: 'number',
                defaultValue: advanceUnsettled(openDialog.advance),
                required: true,
                min: 1,
                max: advanceUnsettled(openDialog.advance),
              },
              { name: 'input_invoice_id', label: 'Hoá đơn verified', type: 'select', options: verifiedInvoiceOptions },
              { name: 'schedule_id', label: 'Kỳ thanh toán liên quan', type: 'select', options: activeScheduleOptions },
              { name: 'comment', label: 'Ghi chú', type: 'textarea', fullWidth: true },
            ]}
            submitLabel="Quyết toán"
            onSubmit={(values) => settleAdvance.mutateAsync({ advanceID: openDialog.advance.id, values })}
          />
        ) : null}
      </Dialog>
    </section>
  );
}

function MetricTile({ label, value }: { label: string; value: ReactNode }) {
  return (
    <div className="rounded-md border border-line bg-slate-50 px-3 py-2">
      <p className="text-xs font-semibold uppercase text-slate-500">{label}</p>
      <div className="mt-1 text-base font-semibold text-ink">{value}</div>
    </div>
  );
}

function advanceUnpaid(row: PayableAdvanceRequestRecord) {
  return Math.max(0, Number(row.amount ?? 0) - Number(row.paid_amount ?? 0));
}

function advanceUnsettled(row: PayableAdvanceRequestRecord) {
  return Math.max(0, Number(row.amount ?? 0) - Number(row.settled_amount ?? 0));
}

function moneyLabel(value: unknown) {
  return new Intl.NumberFormat('vi-VN', { style: 'currency', currency: 'VND', maximumFractionDigits: 0 }).format(Number(value ?? 0));
}
