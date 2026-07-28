import { useEffect, useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { AlertTriangle, CheckCircle2, Download, FileCheck2, Plus, RotateCcw } from 'lucide-react';
import { CurrencyText } from '../../components/CurrencyText';
import { DataTable, type Column } from '../../components/DataTable';
import { Dialog } from '../../components/Dialog';
import { EmptyState } from '../../components/EmptyState';
import { StatusBadge } from '../../components/StatusBadge';
import { useToast } from '../../components/ToastProvider';
import { downloadBlob, get, post } from '../../lib/api/client';
import { useAuth } from '../../lib/auth/AuthContext';
import type { AccountingTaxPeriodDetail } from '../../lib/types/domain';
import { dateText } from '../../lib/utils/format';
import { SimpleForm } from '../_shared/SimpleForm';
import { JournalEntryNumber } from './JournalEntryNumber';

type TaxTab = 'declaration' | 'purchases' | 'sales' | 'adjustments' | 'history';
type TaxDialog =
  | { type: 'action'; action: 'prepare' | 'file' | 'lock' | 'start-amendment' }
  | { type: 'adjustment' }
  | { type: 'reverse-adjustment'; row: Record<string, any> }
  | { type: 'payment' }
  | null;

const tabs: Array<{ value: TaxTab; label: string }> = [
  { value: 'declaration', label: 'Tổng hợp tờ khai' },
  { value: 'purchases', label: 'Mua vào' },
  { value: 'sales', label: 'Bán ra' },
  { value: 'adjustments', label: 'Điều chỉnh' },
  { value: 'history', label: 'Lịch sử kỳ' },
];

export function TaxWorkspace({ projectId, enabled, accounts }: { projectId: string; enabled: boolean; accounts: Record<string, any>[] }) {
  const { hasAnyPermission } = useAuth();
  const canManage = hasAnyPermission(['accounting.tax_manage']);
  const canApprovePayment = hasAnyPermission(['accounting.payment_approve']);
  const toast = useToast();
  const queryClient = useQueryClient();
  const [periodType, setPeriodType] = useState<'month' | 'quarter'>('month');
  const [periodId, setPeriodId] = useState('');
  const [tab, setTab] = useState<TaxTab>('declaration');
  const [dialog, setDialog] = useState<TaxDialog>(null);
  const [exporting, setExporting] = useState('');
  const bookId = String(accounts[0]?.book_id ?? '');

  const periods = useQuery({
    queryKey: ['accounting-tax-periods'],
    queryFn: () => get<Record<string, any>[]>('/accounting/tax-periods'),
    enabled,
  });
  const taxCodes = useQuery({
    queryKey: ['accounting-tax-codes', projectId],
    queryFn: () => get<Record<string, any>[]>(`/accounting/tax-codes?project_id=${encodeURIComponent(projectId)}`),
    enabled: enabled && Boolean(projectId),
  });
  const matchingPeriods = useMemo(
    () => (periods.data ?? []).filter((period) => (!bookId || period.book_id === bookId) && (period.period_type ?? 'month') === periodType),
    [bookId, periodType, periods.data],
  );
  useEffect(() => {
    if (matchingPeriods.some((period) => period.id === periodId)) return;
    const today = new Date().toISOString().slice(0, 10);
    const current = matchingPeriods.find((period) => String(period.date_from).slice(0, 10) <= today && String(period.date_to).slice(0, 10) >= today);
    setPeriodId(String(current?.id ?? matchingPeriods[0]?.id ?? ''));
  }, [matchingPeriods, periodId]);
  const detail = useQuery({
    queryKey: ['accounting-tax-period', periodId],
    queryFn: () => get<AccountingTaxPeriodDetail>(`/accounting/tax-periods/${periodId}`),
    enabled: enabled && Boolean(periodId),
  });

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ['accounting-tax-periods'] });
    queryClient.invalidateQueries({ queryKey: ['accounting-tax-period'] });
    queryClient.invalidateQueries({ queryKey: ['accounting-journal'] });
    queryClient.invalidateQueries({ queryKey: ['accounting-general-ledger'] });
    queryClient.invalidateQueries({ queryKey: ['accounting-vat-ledger'] });
    queryClient.invalidateQueries({ queryKey: ['payments'] });
    queryClient.invalidateQueries({ queryKey: ['cashflow-report'] });
  };
  const action = useMutation({
    mutationFn: ({ actionName, values }: { actionName: string; values: Record<string, any> }) => post(`/accounting/tax-periods/${periodId}/${actionName}`, values),
    onSuccess: (_, variables) => {
      invalidate();
      setDialog(null);
      toast.success(taxActionSuccess(variables.actionName));
    },
    onError: (error) => toast.error('Không cập nhật được kỳ thuế GTGT', error),
  });
  const createAdjustment = useMutation({
    mutationFn: (values: Record<string, any>) => post(`/accounting/tax-periods/${periodId}/adjustments`, { ...values, project_id: projectId }),
    onSuccess: () => { invalidate(); setDialog(null); toast.success('Đã ghi sổ điều chỉnh thuế GTGT'); },
    onError: (error) => toast.error('Không ghi sổ được điều chỉnh thuế', error),
  });
  const reverseAdjustment = useMutation({
    mutationFn: ({ id, values }: { id: string; values: Record<string, any> }) => post(`/accounting/tax-adjustments/${id}/reverse`, values),
    onSuccess: () => { invalidate(); setDialog(null); toast.success('Đã lập bút toán đảo điều chỉnh thuế'); },
    onError: (error) => toast.error('Không đảo được điều chỉnh thuế', error),
  });
  const createPayment = useMutation({
    mutationFn: (values: Record<string, any>) => post(`/accounting/tax-periods/${periodId}/payments`, { ...values, project_id: projectId }),
    onSuccess: () => { invalidate(); setDialog(null); toast.success('Đã lập Phiếu nộp thuế GTGT nháp'); },
    onError: (error) => toast.error('Không lập được Phiếu nộp thuế', error),
  });
  const approvePayment = useMutation({
    mutationFn: (id: string) => post(`/payments/${id}/approve`, {}),
    onSuccess: () => { invalidate(); toast.success('Đã duyệt và ghi sổ Phiếu nộp thuế GTGT'); },
    onError: (error) => toast.error('Không duyệt được Phiếu nộp thuế', error),
  });

  if (!enabled) return <EmptyState title="Chưa có quyền xem Thuế GTGT" />;
  const period = detail.data;
  const summary = period?.summary ?? {};
  const latestReturn: Record<string, any> = period?.latest_return ?? {};
  const issues = period?.issues ?? [];
  const payments = period?.payments ?? [];
  const reservedOrPaid = payments.filter((payment) => !['cancelled', 'deleted'].includes(payment.status)).reduce((sum, payment) => sum + Number(payment.amount ?? 0), 0);
  const previewMode = ['open', 'amended'].includes(String(period?.status ?? ''));
  const payable = Number(previewMode ? summary.vat_payable ?? 0 : latestReturn.vat_payable ?? summary.vat_payable ?? 0);
  const outstanding = Math.max(payable - reservedOrPaid, 0);
  const periodLabel = period ? taxPeriodLabel(period) : 'Kỳ thuế GTGT';

  async function exportCSV(kind: 'purchases' | 'sales' | 'declaration') {
    if (!periodId) return;
    setExporting(kind);
    try {
      const blob = await downloadBlob(`/accounting/tax-periods/${periodId}/export?kind=${kind}`);
      const url = URL.createObjectURL(blob);
      const anchor = document.createElement('a');
      anchor.href = url;
      anchor.download = `thue-gtgt-${kind}-${period?.fiscal_year ?? ''}-${period?.period_no ?? ''}.csv`;
      anchor.click();
      window.setTimeout(() => URL.revokeObjectURL(url), 1000);
      toast.success('Đã xuất sổ thuế từ cùng nguồn số liệu tờ khai');
    } catch (error) {
      toast.error('Không xuất được sổ thuế', error);
    } finally {
      setExporting('');
    }
  }

  const primary = taxPrimaryAction(period?.status);
  const adjustmentDate = defaultPeriodDate(period);
  const taxCodeOptions = (taxCodes.data ?? []).map((item) => ({ value: String(item.id), label: `${item.tax_code} · ${item.name}` }));
  const accountOptions = accounts.filter((account) => account.is_postable !== false && !['1331', '3331'].includes(String(account.account_code))).map((account) => ({ value: String(account.id), label: `${account.account_code} · ${account.name}` }));

  return <section className="space-y-4">
    <div className="rounded-lg border border-line bg-white p-4 shadow-panel">
      <div className="flex flex-col gap-4 xl:flex-row xl:items-end xl:justify-between">
        <div>
          <p className="text-xs font-semibold uppercase tracking-wide text-blue-700">Phân hệ Thuế GTGT</p>
          <h2 className="mt-1 text-lg font-semibold text-ink">Lập, nộp và khóa tờ khai</h2>
          <p className="mt-1 max-w-3xl text-sm text-slate-500">Số liệu lấy từ hóa đơn đã ghi sổ, bút toán đảo, điều chỉnh thuế và TK 1331/3331. Phân hệ này chưa bao gồm thuế TNDN hoặc TNCN.</p>
        </div>
        <div className="grid gap-2 sm:grid-cols-2 xl:flex xl:items-end">
          <label className="block"><span className="text-xs font-semibold uppercase text-slate-500">Chu kỳ kê khai</span><select className="mt-1 w-full rounded-md border border-line px-3 py-2 text-sm" value={periodType} onChange={(event) => setPeriodType(event.target.value as 'month' | 'quarter')}><option value="month">Theo tháng</option><option value="quarter">Theo quý</option></select></label>
          <label className="block min-w-56"><span className="text-xs font-semibold uppercase text-slate-500">Kỳ kê khai</span><select className="mt-1 w-full rounded-md border border-line px-3 py-2 text-sm" value={periodId} onChange={(event) => setPeriodId(event.target.value)}><option value="">Chọn kỳ</option>{matchingPeriods.map((item) => <option key={item.id} value={item.id}>{taxPeriodLabel(item)}</option>)}</select></label>
          <div className="flex h-10 items-center self-end"><StatusBadge status={String(period?.status ?? 'open')} /></div>
        </div>
      </div>
      <div className="mt-4 flex flex-wrap gap-2 border-t border-line pt-3">
        <ExportButton label="Sổ mua vào" loading={exporting === 'purchases'} onClick={() => void exportCSV('purchases')} />
        <ExportButton label="Sổ bán ra" loading={exporting === 'sales'} onClick={() => void exportCSV('sales')} />
        <ExportButton label="Tổng hợp tờ khai" loading={exporting === 'declaration'} onClick={() => void exportCSV('declaration')} />
      </div>
    </div>

    {detail.isLoading ? <div className="rounded-lg border border-line bg-white p-8 text-sm text-slate-500">Đang tập hợp số liệu kỳ thuế...</div> : detail.error ? <div className="rounded-lg border border-rose-200 bg-rose-50 p-4 text-sm text-rose-700"><p className="font-semibold">Không tải được kỳ thuế GTGT</p><button className="mt-3 rounded-md border border-rose-200 bg-white px-3 py-2 font-semibold" onClick={() => detail.refetch()}>Thử lại</button></div> : !period ? <EmptyState title="Chưa có kỳ thuế GTGT phù hợp" /> : <>
      <section className={`rounded-lg border p-4 ${issues.length ? 'border-amber-200 bg-amber-50' : 'border-emerald-200 bg-emerald-50'}`}>
        <div className="flex items-start gap-3">
          {issues.length ? <AlertTriangle className="mt-0.5 h-5 w-5 shrink-0 text-amber-700" /> : <CheckCircle2 className="mt-0.5 h-5 w-5 shrink-0 text-emerald-700" />}
          <div className="min-w-0"><h3 className="font-semibold text-ink">{issues.length ? `${issues.length} vấn đề phải xử lý trước khi chốt số liệu` : 'Số liệu đủ điều kiện chốt tờ khai'}</h3><p className="mt-1 text-xs leading-5 text-slate-600">{issues.length ? 'Không thể chốt kỳ khi còn hóa đơn chưa ghi sổ, thiếu dòng thuế hoặc lệch TK 1331/3331.' : 'Hóa đơn, sổ thuế và tài khoản thuế đang khớp trong phạm vi kỳ đã chọn.'}</p></div>
        </div>
        {issues.length ? <div className="mt-3 grid gap-2 lg:grid-cols-3">{issues.map((issue) => <div key={issue.code} className="rounded-md border border-amber-200 bg-white px-3 py-2"><p className="text-sm font-semibold text-amber-800">{issue.title}</p><p className="mt-1 text-xs text-slate-500">{taxIssueDetail(issue)}</p></div>)}</div> : null}
      </section>

      <div className="grid grid-cols-2 gap-3 xl:grid-cols-5">
        <TaxMetric label="Thuế GTGT đầu vào" value={summary.input_vat_amount} />
        <TaxMetric label="Được khấu trừ" value={summary.deductible_input_vat_amount} />
        <TaxMetric label="Thuế GTGT đầu ra" value={summary.output_vat_amount} />
        <TaxMetric label="Điều chỉnh thuần" value={summary.adjustment_vat_amount} />
        <TaxMetric label={previewMode ? 'Dự kiến còn phải nộp' : 'Còn phải nộp'} value={payable} tone={payable > 0 ? 'amber' : 'green'} />
      </div>

      <div className="flex gap-1 overflow-x-auto border-b border-line">{tabs.map((item) => <button key={item.value} type="button" className={`whitespace-nowrap border-b-2 px-3 py-2.5 text-sm font-semibold ${tab === item.value ? 'border-blue-600 text-blue-700' : 'border-transparent text-slate-500'}`} onClick={() => setTab(item.value)}>{item.label}</button>)}</div>
      {tab === 'declaration' ? <DeclarationSummary period={period} summary={summary} latestReturn={latestReturn} payments={payments} canApprovePayment={canApprovePayment} approving={approvePayment.isPending} onApprove={(id) => approvePayment.mutate(id)} /> : null}
      {tab === 'purchases' ? <TaxLedger title="Sổ mua vào" direction="input" rows={period.purchases ?? []} /> : null}
      {tab === 'sales' ? <TaxLedger title="Sổ bán ra" direction="output" rows={period.sales ?? []} /> : null}
      {tab === 'adjustments' ? <AdjustmentLedger rows={period.adjustments ?? []} canReverse={canManage && ['open', 'amended'].includes(period.status)} onReverse={(row) => setDialog({ type: 'reverse-adjustment', row })} /> : null}
      {tab === 'history' ? <TaxHistory period={period} /> : null}

      {canManage ? <div className="sticky bottom-3 z-10 flex flex-wrap justify-end gap-2 rounded-lg border border-line bg-white/95 p-3 shadow-lg backdrop-blur">
        {['open', 'amended'].includes(period.status) ? <button type="button" className="inline-flex items-center gap-2 rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-blue-700" onClick={() => setDialog({ type: 'adjustment' })}><Plus className="h-4 w-4" />Lập điều chỉnh</button> : null}
        {['filed', 'locked'].includes(period.status) && outstanding > 0 ? <button type="button" className="inline-flex items-center gap-2 rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-blue-700" onClick={() => setDialog({ type: 'payment' })}><FileCheck2 className="h-4 w-4" />Lập Phiếu nộp thuế</button> : null}
        {primary ? <button type="button" disabled={primary.action === 'prepare' && issues.length > 0} className="rounded-md bg-blue-600 px-4 py-2 text-sm font-semibold text-white disabled:cursor-not-allowed disabled:opacity-50" onClick={() => setDialog({ type: 'action', action: primary.action })}>{primary.label}</button> : null}
      </div> : null}
    </>}

    <Dialog open={dialog?.type === 'action'} title={taxActionTitle(dialog?.type === 'action' ? dialog.action : 'prepare', periodLabel)} description={taxActionDescription(dialog?.type === 'action' ? dialog.action : 'prepare')} onClose={() => setDialog(null)}>
      <SimpleForm key={`${periodId}-${dialog?.type === 'action' ? dialog.action : ''}`} fields={[
        { name: 'filing_reference', label: 'Mã giao dịch nộp tờ khai', required: dialog?.type === 'action' && dialog.action === 'file', hiddenWhen: () => !(dialog?.type === 'action' && dialog.action === 'file'), placeholder: 'Mã tiếp nhận của cơ quan thuế' },
        { name: 'reason', label: 'Căn cứ thực hiện', type: 'textarea', required: true, fullWidth: true, placeholder: 'Nêu căn cứ đối chiếu, phê duyệt hoặc lý do lập bổ sung' },
      ]} submitLabel={action.isPending ? 'Đang xử lý...' : taxActionButton(dialog?.type === 'action' ? dialog.action : 'prepare')} onCancel={() => setDialog(null)} onSubmit={(values) => action.mutateAsync({ actionName: dialog?.type === 'action' ? dialog.action : 'prepare', values })} />
    </Dialog>

    <Dialog open={dialog?.type === 'adjustment'} title="Lập điều chỉnh thuế GTGT" description="Điều chỉnh tạo Chứng từ ghi sổ cân Nợ/Có ngay trong cùng giao dịch và không sửa số liệu hóa đơn gốc." size="xl" onClose={() => setDialog(null)}>
      <SimpleForm key={`adjustment-${periodId}`} layout="two-column" fields={[
        { name: 'entry_date', label: 'Ngày hạch toán', type: 'date', defaultValue: adjustmentDate, required: true },
        { name: 'direction', label: 'Nhóm sổ thuế', type: 'select', defaultValue: 'input', required: true, options: [{ value: 'input', label: 'Thuế GTGT đầu vào' }, { value: 'output', label: 'Thuế GTGT đầu ra' }] },
        { name: 'tax_code_id', label: 'Mã thuế GTGT', type: 'select', required: true, options: taxCodeOptions },
        { name: 'offset_account_id', label: 'Tài khoản đối ứng', type: 'select', required: true, options: accountOptions },
        { name: 'taxable_delta', label: 'Điều chỉnh giá trị chưa thuế', type: 'number', defaultValue: 0 },
        { name: 'vat_delta', label: 'Điều chỉnh tiền thuế', type: 'number', required: true, validate: (value) => Math.abs(Number(value ?? 0)) < 0.005 ? 'Tiền thuế điều chỉnh phải khác 0' : undefined },
        { name: 'source_document_no', label: 'Số chứng từ căn cứ', placeholder: 'Biên bản, hóa đơn điều chỉnh...' },
        { name: 'reason', label: 'Lý do điều chỉnh', type: 'textarea', required: true, fullWidth: true },
      ]} renderAside={(values) => <TaxAdjustmentPreview values={values} accounts={accounts} taxCodes={taxCodes.data ?? []} />} submitLabel={createAdjustment.isPending ? 'Đang ghi sổ...' : 'Ghi sổ điều chỉnh'} onCancel={() => setDialog(null)} onSubmit={(values) => createAdjustment.mutateAsync(values)} />
    </Dialog>

    <Dialog open={dialog?.type === 'reverse-adjustment'} title="Đảo điều chỉnh thuế GTGT" description="Hệ thống giữ nguyên điều chỉnh gốc và lập điều chỉnh đảo cùng bút toán đảo liên kết." onClose={() => setDialog(null)}>
      <SimpleForm key={`reverse-${dialog?.type === 'reverse-adjustment' ? dialog.row.id : ''}`} fields={[
        { name: 'entry_date', label: 'Ngày hạch toán bút toán đảo', type: 'date', defaultValue: adjustmentDate, required: true },
        { name: 'reason', label: 'Lý do đảo điều chỉnh', type: 'textarea', required: true },
      ]} submitLabel={reverseAdjustment.isPending ? 'Đang lập bút toán đảo...' : 'Xác nhận đảo'} onCancel={() => setDialog(null)} onSubmit={(values) => reverseAdjustment.mutateAsync({ id: String(dialog?.type === 'reverse-adjustment' ? dialog.row.id : ''), values })} />
    </Dialog>

    <Dialog open={dialog?.type === 'payment'} title="Lập Phiếu nộp thuế GTGT" description="Phiếu được lập ở trạng thái nháp. Khi duyệt, hệ thống ghi Nợ TK 3331/Có TK tiền và không ghi nhận chi phí ngân sách." onClose={() => setDialog(null)}>
      <SimpleForm key={`tax-payment-${periodId}-${outstanding}`} fields={[
        { name: 'payment_code', label: 'Số Phiếu nộp thuế', defaultValue: taxPaymentCode(period), required: true },
        { name: 'amount', label: 'Số tiền nộp', type: 'number', defaultValue: outstanding, min: 0.01, max: outstanding, required: true },
        { name: 'payment_method', label: 'Phương thức nộp', type: 'select', defaultValue: 'bank_transfer', required: true, options: [{ value: 'bank_transfer', label: 'Chuyển khoản' }, { value: 'cash', label: 'Tiền mặt' }] },
        { name: 'cash_account_id', label: 'Tài khoản tiền', type: 'select', options: accounts.filter((account) => ['1111', '1121'].includes(String(account.account_code))).map((account) => ({ value: String(account.id), label: `${account.account_code} · ${account.name}` })), helperText: 'Để trống để hệ thống chọn TK 1121 hoặc 1111 theo phương thức nộp.' },
      ]} submitLabel={createPayment.isPending ? 'Đang lập phiếu...' : 'Lập Phiếu nộp thuế'} onCancel={() => setDialog(null)} onSubmit={(values) => createPayment.mutateAsync(values)} />
    </Dialog>
  </section>;
}

function DeclarationSummary({ period, summary, latestReturn, payments, canApprovePayment, approving, onApprove }: { period: AccountingTaxPeriodDetail; summary: Record<string, any>; latestReturn: Record<string, any>; payments: Record<string, any>[]; canApprovePayment: boolean; approving: boolean; onApprove: (id: string) => void }) {
  const previewMode = ['open', 'amended'].includes(period.status);
  const declaration = previewMode ? summary : latestReturn;
  const values = [
    ['Thuế GTGT đầu vào', summary.input_vat_amount], ['Thuế GTGT đầu vào được khấu trừ', summary.deductible_input_vat_amount],
    ['Điều chỉnh thuế đầu vào', summary.input_adjustment_vat_amount], ['Thuế GTGT đầu ra', summary.output_vat_amount],
    ['Điều chỉnh thuế đầu ra', summary.output_adjustment_vat_amount], ['Còn được khấu trừ kỳ trước', declaration.credit_brought_forward],
    [previewMode ? 'Dự kiến thuế GTGT phải nộp' : 'Thuế GTGT phải nộp', declaration.vat_payable], [previewMode ? 'Dự kiến khấu trừ chuyển kỳ sau' : 'Còn được khấu trừ chuyển kỳ sau', declaration.credit_carried_forward],
  ];
  return <div className="space-y-4">
    <div className="grid gap-2 md:grid-cols-2">{values.map(([label, value]) => <div key={String(label)} className="flex items-center justify-between gap-4 rounded-md border border-line bg-white px-3 py-2 text-sm"><span className="text-slate-600">{label}</span><span className="font-semibold text-ink"><CurrencyText value={Number(value ?? 0)} /></span></div>)}</div>
    <div><h3 className="mb-2 text-sm font-semibold text-ink">Phiếu nộp thuế của kỳ</h3>{payments.length ? <div className="space-y-2">{payments.map((payment) => <div key={payment.id} className="flex flex-col gap-2 rounded-md border border-line bg-white p-3 sm:flex-row sm:items-center sm:justify-between"><div><p className="font-semibold text-ink">{payment.payment_code}</p><p className="mt-1 text-xs text-slate-500"><CurrencyText value={payment.amount} /> · {payment.project_name}</p></div><div className="flex items-center gap-2"><StatusBadge status={payment.status} />{canApprovePayment && ['draft', 'submitted', 'pending', 'waiting_approval'].includes(payment.status) ? <button disabled={approving} className="rounded-md bg-blue-600 px-3 py-2 text-xs font-semibold text-white disabled:opacity-50" onClick={() => onApprove(payment.id)}>Duyệt & ghi sổ</button> : null}</div></div>)}</div> : <EmptyState title="Chưa có Phiếu nộp thuế trong kỳ" />}</div>
    <p className="text-xs text-slate-500">Trạng thái kỳ: {taxPeriodLabel(period)} · lần kê khai {Number(period.amendment_no ?? 0) === 0 ? 'chính thức' : `bổ sung ${period.amendment_no}`}.</p>
  </div>;
}

function TaxLedger({ title, direction, rows }: { title: string; direction: 'input' | 'output'; rows: Record<string, any>[] }) {
  const columns: Column<Record<string, any>>[] = [
    { key: 'invoice_date', header: 'Ngày hóa đơn', render: (row) => dateText(row.invoice_date) }, { key: 'invoice_number', header: 'Số hóa đơn', render: (row) => <span className="font-semibold text-ink">{row.invoice_number}</span> },
    { key: 'counterparty_name', header: 'Đối tượng' }, { key: 'counterparty_tax_code', header: 'Mã số thuế' }, { key: 'tax_code', header: 'Mã thuế GTGT' },
    { key: 'taxable_amount', header: 'Giá trị chưa thuế', align: 'right', render: (row) => <CurrencyText value={row.taxable_amount} /> }, { key: 'vat_amount', header: 'Tiền thuế', align: 'right', render: (row) => <CurrencyText value={row.vat_amount} /> },
  ];
  if (direction === 'input') columns.push({ key: 'deductible_amount', header: 'Được khấu trừ', align: 'right', render: (row) => <CurrencyText value={row.deductible_amount} /> });
  columns.push({ key: 'journal_entry_no', header: 'Số CTGS', render: (row) => <JournalEntryNumber value={row.journal_entry_no} /> });
  return <section className="space-y-3"><h3 className="text-sm font-semibold text-ink">{title}</h3><div className="space-y-2 md:hidden">{rows.length ? rows.map((row) => <div key={row.id} className="rounded-lg border border-line bg-white p-3"><div className="flex items-start justify-between gap-3"><div><p className="font-semibold text-ink">{row.invoice_number}</p><p className="mt-1 text-xs text-slate-500">{dateText(row.invoice_date)} · {row.counterparty_name}</p></div><span className="text-sm font-semibold text-blue-700"><CurrencyText value={row.vat_amount} /></span></div><div className="mt-3 grid grid-cols-2 gap-2 text-xs"><span>Mã thuế: <b>{row.tax_code}</b></span><span className="min-w-0 overflow-hidden">CTGS: <JournalEntryNumber value={row.journal_entry_no} className="font-semibold text-ink" /></span><span className="col-span-2">Giá trị chưa thuế: <b><CurrencyText value={row.taxable_amount} /></b></span>{direction === 'input' ? <span className="col-span-2">Được khấu trừ: <b><CurrencyText value={row.deductible_amount} /></b></span> : null}</div></div>) : <EmptyState title="Chưa có hóa đơn đã ghi sổ trong kỳ" />}</div><div className="hidden md:block"><DataTable rows={rows} pageSize={12} searchPlaceholder="Tìm số hóa đơn, đối tượng, mã thuế..." columns={columns} empty="Chưa có hóa đơn đã ghi sổ trong kỳ" /></div></section>;
}

function AdjustmentLedger({ rows, canReverse, onReverse }: { rows: Record<string, any>[]; canReverse: boolean; onReverse: (row: Record<string, any>) => void }) {
  return <div className="space-y-2">{rows.length ? rows.map((row) => <div key={row.id} className="rounded-lg border border-line bg-white p-3"><div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between"><div><div className="flex flex-wrap items-center gap-2"><span className="font-semibold text-ink">{row.source_document_no || `Điều chỉnh ${String(row.id).slice(0, 8)}`}</span><StatusBadge status={row.adjustment_type === 'reversal' ? 'cancelled' : 'posted'} /></div><p className="mt-1 text-sm text-slate-600">{row.reason}</p><p className="mt-1 text-xs text-slate-500">{row.direction === 'input' ? 'Thuế đầu vào' : 'Thuế đầu ra'} · {row.tax_code} · đối ứng {row.offset_account_code} · CTGS <JournalEntryNumber value={row.journal_entry_no} className="font-semibold text-slate-500" /></p></div><div className="flex items-center gap-3"><span className={`font-semibold ${Number(row.vat_delta) < 0 ? 'text-rose-700' : 'text-blue-700'}`}><CurrencyText value={row.vat_delta} /></span>{canReverse && !row.reversal_of_id && !row.reversal_journal_entry_no ? <button className="inline-flex items-center gap-1 rounded-md border border-line px-3 py-2 text-xs font-semibold text-rose-700" onClick={() => onReverse(row)}><RotateCcw className="h-3.5 w-3.5" />Đảo</button> : null}</div></div></div>) : <EmptyState title="Chưa có điều chỉnh thuế trong kỳ" />}</div>;
}

function TaxHistory({ period }: { period: AccountingTaxPeriodDetail }) {
  const returns = period.returns ?? [];
  const events = period.events ?? [];
  return <div className="grid gap-4 xl:grid-cols-2"><section><h3 className="mb-2 text-sm font-semibold text-ink">Các lần chốt số liệu</h3><div className="space-y-2">{returns.length ? returns.map((row) => <div key={row.id} className="rounded-md border border-line bg-white p-3"><div className="flex items-center justify-between"><span className="font-semibold text-ink">{Number(row.amendment_no) === 0 ? 'Tờ khai chính thức' : `Tờ khai bổ sung lần ${row.amendment_no}`}</span><span className="text-sm font-semibold text-amber-700"><CurrencyText value={row.vat_payable} /></span></div><p className="mt-1 text-xs text-slate-500">Chốt bởi {row.prepared_by_name || '-'} · {dateText(row.prepared_at)}</p></div>) : <EmptyState title="Chưa chốt số liệu lần nào" />}</div></section><section><h3 className="mb-2 text-sm font-semibold text-ink">Dấu vết xử lý kỳ</h3><div className="space-y-2">{events.length ? events.map((row) => <div key={row.id} className="rounded-md border border-line bg-white p-3"><p className="font-semibold text-ink">{taxEventLabel(row.event_type)}</p><p className="mt-1 text-xs text-slate-500">{row.actor_name || '-'} · {dateText(row.created_at)} · {row.reason}</p>{row.filing_reference ? <p className="mt-1 text-xs font-medium text-blue-700">Mã giao dịch: {row.filing_reference}</p> : null}</div>) : <EmptyState title="Chưa có lịch sử xử lý kỳ" />}</div></section></div>;
}

function TaxAdjustmentPreview({ values, accounts, taxCodes }: { values: Record<string, any>; accounts: Record<string, any>[]; taxCodes: Record<string, any>[] }) {
  const vat = Number(values.vat_delta ?? 0);
  const direction = values.direction === 'output' ? 'output' : 'input';
  const taxCode = taxCodes.find((item) => item.id === values.tax_code_id);
  const offset = accounts.find((item) => item.id === values.offset_account_id);
  const taxAccount = direction === 'input' ? taxCode?.input_account_code ?? '1331' : taxCode?.output_account_code ?? '3331';
  const taxDebit = (direction === 'input' && vat > 0) || (direction === 'output' && vat < 0);
  return <div><p className="text-xs font-semibold uppercase text-slate-500">Định khoản dự kiến</p><div className="mt-3 space-y-2 text-sm"><div className="flex justify-between gap-3"><span>Nợ {taxDebit ? taxAccount : offset?.account_code ?? 'TK đối ứng'}</span><CurrencyText value={Math.abs(vat)} /></div><div className="flex justify-between gap-3"><span>Có {taxDebit ? offset?.account_code ?? 'TK đối ứng' : taxAccount}</span><CurrencyText value={Math.abs(vat)} /></div></div><div className={`mt-3 rounded-md p-2 text-xs font-semibold ${Math.abs(vat) > 0 ? 'bg-emerald-50 text-emerald-700' : 'bg-amber-50 text-amber-700'}`}>{Math.abs(vat) > 0 ? 'Bút toán cân Nợ/Có' : 'Nhập tiền thuế điều chỉnh để xem định khoản'}</div></div>;
}

function TaxMetric({ label, value, tone = 'slate' }: { label: string; value: unknown; tone?: 'slate' | 'amber' | 'green' }) {
  const colors = tone === 'amber' ? 'border-amber-200 bg-amber-50' : tone === 'green' ? 'border-emerald-200 bg-emerald-50' : 'border-line bg-slate-50';
  return <div className={`rounded-lg border p-3 ${colors}`}><p className="text-xs font-semibold uppercase tracking-wide text-slate-500">{label}</p><p className="mt-1 text-base font-semibold text-ink sm:text-lg"><CurrencyText value={Number(value ?? 0)} /></p></div>;
}

function ExportButton({ label, loading, onClick }: { label: string; loading: boolean; onClick: () => void }) {
  return <button type="button" disabled={loading} className="inline-flex items-center gap-2 rounded-md border border-line bg-white px-3 py-2 text-xs font-semibold text-slate-700 disabled:opacity-50" onClick={onClick}><Download className="h-3.5 w-3.5" />{loading ? 'Đang xuất...' : label}</button>;
}

function taxPeriodLabel(period: Record<string, any>) {
  return `${(period.period_type ?? 'month') === 'quarter' ? 'Quý' : 'Tháng'} ${period.period_no}/${period.fiscal_year}`;
}

function taxPrimaryAction(status?: string): { action: 'prepare' | 'file' | 'lock' | 'start-amendment'; label: string } | null {
  if (status === 'open' || status === 'amended') return { action: 'prepare', label: 'Chốt số liệu' };
  if (status === 'prepared') return { action: 'file', label: 'Ghi nhận đã nộp tờ khai' };
  if (status === 'filed') return { action: 'lock', label: 'Khóa kỳ thuế' };
  if (status === 'locked') return { action: 'start-amendment', label: 'Lập tờ khai bổ sung' };
  return null;
}

function taxActionTitle(action: string, periodLabel: string) {
  return ({ prepare: `Chốt số liệu ${periodLabel}`, file: `Ghi nhận nộp tờ khai ${periodLabel}`, lock: `Khóa ${periodLabel}`, 'start-amendment': `Lập tờ khai bổ sung ${periodLabel}` } as Record<string, string>)[action];
}
function taxActionDescription(action: string) {
  return ({ prepare: 'Hệ thống kiểm tra hóa đơn, dòng thuế và TK 1331/3331 rồi lưu ảnh chụp số liệu bất biến.', file: 'Ghi nhận mã giao dịch tiếp nhận từ cơ quan thuế và người thực hiện.', lock: 'Sau khi khóa, muốn sửa số liệu phải mở một lần kê khai bổ sung.', 'start-amendment': 'Tạo lần kê khai bổ sung mới; tờ khai và dấu vết trước đó được giữ nguyên.' } as Record<string, string>)[action];
}
function taxActionButton(action: string) { return ({ prepare: 'Xác nhận chốt số liệu', file: 'Xác nhận đã nộp', lock: 'Xác nhận khóa kỳ', 'start-amendment': 'Mở lần kê khai bổ sung' } as Record<string, string>)[action]; }
function taxActionSuccess(action: string) { return ({ prepare: 'Đã chốt số liệu tờ khai', file: 'Đã ghi nhận nộp tờ khai', lock: 'Đã khóa kỳ thuế GTGT', 'start-amendment': 'Đã mở lần kê khai bổ sung' } as Record<string, string>)[action] ?? 'Đã cập nhật kỳ thuế'; }
function taxEventLabel(event: string) { return ({ prepared: 'Chốt số liệu', filed: 'Nộp tờ khai', locked: 'Khóa kỳ thuế', amendment_started: 'Mở tờ khai bổ sung' } as Record<string, string>)[event] ?? event; }
function taxIssueDetail(issue: Record<string, any>) {
  if (issue.code === 'unposted_invoices') return `${Number(issue.count ?? 0)} hóa đơn chưa ghi sổ`;
  if (issue.code === 'tax_dimension_gap') return `${Number(issue.tax_line_gap_count ?? 0)} hóa đơn thiếu dòng thuế · ${Number(issue.vat_math_mismatch_count ?? 0)} hóa đơn lệch phép tính`;
  return `Lệch TK 1331: ${Number(issue.input_difference ?? 0).toLocaleString('vi-VN')} · TK 3331: ${Number(issue.output_difference ?? 0).toLocaleString('vi-VN')}`;
}
function defaultPeriodDate(period?: Record<string, any>) { const today = new Date().toISOString().slice(0, 10); if (!period) return today; const from = String(period.date_from).slice(0, 10); const to = String(period.date_to).slice(0, 10); return today >= from && today <= to ? today : to; }
function taxPaymentCode(period?: Record<string, any>) { if (!period) return ''; return `NTGTGT-${period.fiscal_year}-${period.period_type === 'quarter' ? `Q${period.period_no}` : String(period.period_no).padStart(2, '0')}`; }
