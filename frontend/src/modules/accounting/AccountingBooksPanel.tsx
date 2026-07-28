import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Eye, LockKeyhole, Plus, RotateCcw, UnlockKeyhole } from 'lucide-react';
import { CurrencyText } from '../../components/CurrencyText';
import { DataTable } from '../../components/DataTable';
import { Dialog } from '../../components/Dialog';
import { EmptyState } from '../../components/EmptyState';
import { ListSection } from '../../components/ListSection';
import { StatusBadge } from '../../components/StatusBadge';
import { useToast } from '../../components/ToastProvider';
import { get, post } from '../../lib/api/client';
import { useAuth } from '../../lib/auth/AuthContext';
import { dateText } from '../../lib/utils/format';
import { SimpleForm } from '../_shared/SimpleForm';
import { JournalEntryNumber } from './JournalEntryNumber';
import { ManualJournalDialog } from './ManualJournalDialog';
import { TaxWorkspace } from './TaxWorkspace';

export type AccountingBookView = 'journal' | 'ledger' | 'tax';

type AccountingReport = { summary: Record<string, any>; accounts?: Record<string, any>[]; lines?: Record<string, any>[]; ledger?: Record<string, any>[] };

export function AccountingBooksPanel({ view, projectId, enabled, accounts }: { view: AccountingBookView; projectId: string; enabled: boolean; accounts: Record<string, any>[] }) {
  const today = new Date().toISOString().slice(0, 10);
  const { hasAnyPermission } = useAuth();
  const toast = useToast();
  const queryClient = useQueryClient();
  const [dateFrom, setDateFrom] = useState(`${today.slice(0, 4)}-01-01`);
  const [dateTo, setDateTo] = useState(today);
  const [manualJournalOpen, setManualJournalOpen] = useState(false);
  const [periodAction, setPeriodAction] = useState<{ row: Record<string, any>; action: 'close' | 'reopen' } | null>(null);
  const [selectedJournalId, setSelectedJournalId] = useState('');
  const [reversingJournal, setReversingJournal] = useState<Record<string, any> | null>(null);
  const canPost = hasAnyPermission(['accounting.journal_post']);
  const canReverseJournal = hasAnyPermission(['accounting.journal_reverse']);
  const canClosePeriod = hasAnyPermission(['accounting.period_close']);
  const canReopenPeriod = hasAnyPermission(['accounting.period_reopen']);
  const query = `project_id=${encodeURIComponent(projectId)}&date_from=${dateFrom}&date_to=${dateTo}`;
  const journal = useQuery({ queryKey: ['accounting-journal', projectId, dateFrom, dateTo], queryFn: () => get<Record<string, any>[]>(`/accounting/journal-entries?${query}`), enabled: enabled && view === 'journal' && Boolean(projectId) });
  const journalDetail = useQuery({ queryKey: ['accounting-journal-detail', selectedJournalId], queryFn: () => get<Record<string, any>>(`/accounting/journal-entries/${selectedJournalId}`), enabled: Boolean(selectedJournalId) });
  const periods = useQuery({ queryKey: ['accounting-periods'], queryFn: () => get<Record<string, any>[]>('/accounting/periods'), enabled: enabled && view === 'journal' });
  const trialBalance = useQuery({ queryKey: ['accounting-trial-balance', projectId, dateFrom, dateTo], queryFn: () => get<AccountingReport>(`/accounting/reports/trial-balance?${query}`), enabled: enabled && view === 'ledger' && Boolean(projectId) });
  const generalLedger = useQuery({ queryKey: ['accounting-general-ledger', projectId, dateFrom, dateTo], queryFn: () => get<AccountingReport>(`/accounting/reports/general-ledger?${query}`), enabled: enabled && view === 'ledger' && Boolean(projectId) });
  const profitLoss = useQuery({ queryKey: ['accounting-profit-loss', projectId, dateFrom, dateTo], queryFn: () => get<AccountingReport>(`/accounting/reports/profit-loss?${query}`), enabled: enabled && view === 'ledger' && Boolean(projectId) });
  const createJournal = useMutation({
    mutationFn: (values: Record<string, any>) => post(`/projects/${projectId}/accounting/journal-entries`, values),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['accounting-journal'] });
      queryClient.invalidateQueries({ queryKey: ['accounting-trial-balance'] });
      queryClient.invalidateQueries({ queryKey: ['accounting-profit-loss'] });
      setManualJournalOpen(false);
      toast.success('Đã ghi sổ Chứng từ nghiệp vụ khác');
    },
    onError: (error) => toast.error('Không ghi sổ được chứng từ', error),
  });
  const updatePeriod = useMutation({
    mutationFn: ({ id, action, reason }: { id: string; action: 'close' | 'reopen'; reason: string }) => post(`/accounting/periods/${id}/${action}`, { reason }),
    onSuccess: (_, variables) => {
      queryClient.invalidateQueries({ queryKey: ['accounting-periods'] });
      setPeriodAction(null);
      toast.success(variables.action === 'close' ? 'Đã khóa kỳ kế toán' : 'Đã mở lại kỳ kế toán');
    },
    onError: (error) => toast.error('Không cập nhật được kỳ kế toán', error),
  });
  const reverseJournal = useMutation({
    mutationFn: ({ id, values }: { id: string; values: Record<string, any> }) => post(`/accounting/journal-entries/${id}/reverse`, values),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['accounting-journal'] });
      queryClient.invalidateQueries({ queryKey: ['accounting-journal-detail'] });
      queryClient.invalidateQueries({ queryKey: ['accounting-trial-balance'] });
      queryClient.invalidateQueries({ queryKey: ['accounting-general-ledger'] });
      setReversingJournal(null);
      toast.success('Đã lập và liên kết bút toán đảo');
    },
    onError: (error) => toast.error('Không lập được bút toán đảo', error),
  });
  const bookID = String(accounts[0]?.book_id ?? '');
  const bookPeriods = (periods.data ?? []).filter((period) => !bookID || period.book_id === bookID);

  if (!enabled) return <EmptyState title="Chưa có quyền xem sổ kế toán hoặc sổ thuế" />;

  return <div className="space-y-4">
    <div className="flex flex-col gap-3 rounded-lg border border-line bg-white p-3 sm:flex-row sm:items-end">
      <DateField label="Từ ngày" value={dateFrom} onChange={setDateFrom} />
      <DateField label="Đến ngày" value={dateTo} onChange={setDateTo} />
      <div className="text-xs leading-5 text-slate-500">Số liệu chỉ gồm chứng từ đã ghi sổ; bút toán đảo được trình bày độc lập để bảo toàn dấu vết kiểm toán.</div>
    </div>
    {view === 'journal' ? <div className="space-y-4">
      <div className="flex justify-end">{canPost ? <button type="button" className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white" onClick={() => setManualJournalOpen(true)}><Plus className="h-4 w-4" />Lập Chứng từ ghi sổ</button> : null}</div>
      <JournalTable rows={journal.data ?? []} loading={journal.isLoading} error={journal.error} canReverse={canReverseJournal} onRetry={() => journal.refetch()} onView={(row) => setSelectedJournalId(String(row.id))} onReverse={setReversingJournal} />
      <AccountingPeriodsTable rows={bookPeriods} loading={periods.isLoading} error={periods.error} canClose={canClosePeriod} canReopen={canReopenPeriod} onRetry={() => periods.refetch()} onAction={(row, action) => setPeriodAction({ row, action })} />
    </div> : null}
    {view === 'ledger' ? <LedgerReport trial={trialBalance.data} general={generalLedger.data} profit={profitLoss.data} loading={trialBalance.isLoading || generalLedger.isLoading || profitLoss.isLoading} error={trialBalance.error || generalLedger.error || profitLoss.error} onRetry={() => { trialBalance.refetch(); generalLedger.refetch(); profitLoss.refetch(); }} /> : null}
    {view === 'tax' ? <TaxWorkspace projectId={projectId} enabled={enabled} accounts={accounts} /> : null}
    <ManualJournalDialog open={manualJournalOpen} accounts={accounts} saving={createJournal.isPending} onClose={() => setManualJournalOpen(false)} onSubmit={(values) => createJournal.mutateAsync(values)} />
    <Dialog open={Boolean(periodAction)} title={`${periodAction?.action === 'close' ? 'Khóa' : 'Mở lại'} kỳ kế toán ${periodAction?.row.period_no ?? ''}/${periodAction?.row.fiscal_year ?? ''}`} description={periodAction?.action === 'close' ? 'Sau khi khóa kỳ, hệ thống không cho ghi sổ hoặc lập bút toán đảo có ngày hạch toán thuộc kỳ này.' : 'Mở lại kỳ là nghiệp vụ kiểm soát đặc biệt và được lưu trong nhật ký kiểm toán.'} onClose={() => setPeriodAction(null)}>
      <SimpleForm key={`${periodAction?.row.id ?? 'period'}-${periodAction?.action ?? ''}`} fields={[{ name: 'reason', label: 'Căn cứ khóa/mở kỳ', type: 'textarea', required: true, fullWidth: true, placeholder: 'Nêu rõ căn cứ và người phê duyệt' }]} submitLabel={updatePeriod.isPending ? 'Đang xử lý...' : periodAction?.action === 'close' ? 'Xác nhận khóa kỳ' : 'Xác nhận mở lại kỳ'} onCancel={() => setPeriodAction(null)} onSubmit={(values) => updatePeriod.mutateAsync({ id: periodAction!.row.id, action: periodAction!.action, reason: String(values.reason ?? '').trim() })} />
    </Dialog>
    <Dialog open={Boolean(selectedJournalId)} title={`Chứng từ ghi sổ ${journalDetail.data?.entry_no ?? ''}`} description="Chi tiết định khoản, chứng từ nguồn và liên kết bút toán đảo được giữ nguyên để phục vụ kiểm tra." size="xl" onClose={() => setSelectedJournalId('')}>
      {journalDetail.isLoading ? <p className="py-8 text-sm text-slate-500">Đang tải chi tiết chứng từ...</p> : journalDetail.error ? <div className="rounded-md border border-rose-200 bg-rose-50 p-3 text-sm text-rose-700">Không tải được chi tiết chứng từ.</div> : journalDetail.data ? <JournalDetail record={journalDetail.data} canReverse={canReverseJournal} onReverse={() => { setReversingJournal(journalDetail.data!); setSelectedJournalId(''); }} /> : null}
    </Dialog>
    <Dialog open={Boolean(reversingJournal)} title={`Lập bút toán đảo ${reversingJournal?.entry_no ?? ''}`} description="Chứng từ gốc không bị sửa hoặc xóa. Hệ thống tạo một CTGS mới có định khoản ngược và liên kết hai chứng từ." onClose={() => setReversingJournal(null)}>
      <SimpleForm key={`reverse-journal-${reversingJournal?.id ?? ''}`} fields={[{ name: 'entry_date', label: 'Ngày hạch toán bút toán đảo', type: 'date', defaultValue: new Date().toISOString().slice(0, 10), required: true }, { name: 'reason', label: 'Lý do sửa sai', type: 'textarea', required: true, placeholder: 'Nêu sai sót, chứng từ căn cứ và người phê duyệt' }]} submitLabel={reverseJournal.isPending ? 'Đang lập bút toán đảo...' : 'Lập bút toán đảo'} onCancel={() => setReversingJournal(null)} onSubmit={(values) => reverseJournal.mutateAsync({ id: String(reversingJournal?.id ?? ''), values })} />
    </Dialog>
  </div>;
}

function DateField({ label, value, onChange }: { label: string; value: string; onChange: (value: string) => void }) {
  return <label className="block min-w-44"><span className="text-xs font-semibold uppercase tracking-wide text-slate-500">{label}</span><input type="date" className="mt-1 w-full rounded-md border border-line px-3 py-2 text-sm" value={value} onChange={(event) => onChange(event.target.value)} /></label>;
}

function JournalTable({ rows, loading, error, canReverse, onRetry, onView, onReverse }: { rows: Record<string, any>[]; loading: boolean; error: unknown; canReverse: boolean; onRetry: () => void; onView: (row: Record<string, any>) => void; onReverse: (row: Record<string, any>) => void }) {
  return <ListSection title="Sổ Nhật ký chung"><div className="space-y-2 md:hidden">{loading ? <p className="rounded-lg border border-line bg-white p-4 text-sm text-slate-500">Đang tải Sổ Nhật ký chung...</p> : error ? <button className="w-full rounded-lg border border-rose-200 bg-rose-50 p-4 text-left text-sm font-semibold text-rose-700" onClick={onRetry}>Không tải được dữ liệu · Thử lại</button> : rows.length ? rows.map((row) => <div key={row.id} className="rounded-lg border border-line bg-white p-3"><div className="flex items-start justify-between gap-3"><div className="min-w-0 overflow-hidden"><p className="overflow-hidden"><JournalEntryNumber value={row.entry_no} /></p><p className="mt-1 text-xs text-slate-500">{dateText(row.entry_date)} · {sourceEventLabel(row.source_event)}</p></div><StatusBadge status={row.reversed ? 'cancelled' : row.status} /></div><p className="mt-2 text-sm text-slate-600">{row.description}</p><div className="mt-3 flex items-center justify-between border-t border-line pt-2 text-sm"><span>Nợ/Có: <b><CurrencyText value={row.debit_total} /></b></span><div className="flex gap-2"><button className="inline-flex items-center gap-1 font-semibold text-blue-700" onClick={() => onView(row)}><Eye className="h-4 w-4" />Xem</button>{canReverse && !row.reversed && !row.reversal_of_id ? <button className="inline-flex items-center gap-1 font-semibold text-rose-700" onClick={() => onReverse(row)}><RotateCcw className="h-4 w-4" />Đảo</button> : null}</div></div></div>) : <EmptyState title="Chưa có chứng từ ghi sổ trong kỳ đã chọn" />}</div><div className="hidden md:block"><DataTable rows={rows} loading={loading} error={error} onRetry={onRetry} pageSize={15} searchPlaceholder="Tìm số chứng từ, diễn giải, nguồn..." defaultSort={{ key: 'entry_date', direction: 'desc' }} columns={[
    { key: 'entry_date', header: 'Ngày hạch toán', sortable: true, render: (row) => dateText(row.entry_date) },
    { key: 'entry_no', header: 'Số CTGS', sortable: true, render: (row) => <JournalEntryNumber value={row.entry_no} /> },
    { key: 'description', header: 'Diễn giải' },
    { key: 'source_event', header: 'Nghiệp vụ', render: (row) => sourceEventLabel(row.source_event) },
    { key: 'debit_total', header: 'Tổng Nợ', align: 'right', render: (row) => <CurrencyText value={row.debit_total} /> },
    { key: 'credit_total', header: 'Tổng Có', align: 'right', render: (row) => <CurrencyText value={row.credit_total} /> },
    { key: 'status', header: 'Trạng thái', render: (row) => <StatusBadge status={row.status} /> },
    { key: 'action', header: 'Thao tác', render: (row) => <div className="flex items-center gap-2"><button className="inline-flex items-center gap-1 text-xs font-semibold text-blue-700" onClick={() => onView(row)}><Eye className="h-3.5 w-3.5" />Xem</button>{canReverse && !row.reversed && !row.reversal_of_id ? <button className="inline-flex items-center gap-1 text-xs font-semibold text-rose-700" onClick={() => onReverse(row)}><RotateCcw className="h-3.5 w-3.5" />Đảo</button> : null}</div> },
  ]} empty="Chưa có chứng từ ghi sổ trong kỳ đã chọn" /></div></ListSection>;
}

function JournalDetail({ record, canReverse, onReverse }: { record: Record<string, any>; canReverse: boolean; onReverse: () => void }) {
  const lines = record.lines ?? [];
  return <div className="space-y-4"><div className="grid gap-3 rounded-lg border border-line bg-slate-50 p-3 sm:grid-cols-2 lg:grid-cols-4"><DetailValue label="Ngày hạch toán" value={dateText(record.entry_date)} /><DetailValue label="Nghiệp vụ" value={sourceEventLabel(record.source_event)} /><DetailValue label="Chứng từ nguồn" value={record.source_document_no || sourceDocumentLabel(record.source_type)} /><DetailValue label="Trạng thái" value={record.reversed ? 'Đã có bút toán đảo' : record.reversal_of_id ? 'Bút toán đảo' : 'Đã ghi sổ'} /></div><p className="text-sm text-slate-700"><span className="font-semibold">Diễn giải:</span> {record.description}</p>{record.original_entry_no || record.reversal_entry_no ? <div className="rounded-md border border-blue-100 bg-blue-50 p-3 text-sm text-blue-800">{record.original_entry_no ? `Bút toán gốc: ${record.original_entry_no}` : `Bút toán đảo liên kết: ${record.reversal_entry_no}`}</div> : null}<DataTable rows={lines} columns={[{ key: 'account_code', header: 'Số hiệu TK', render: (row) => <span className="font-semibold text-ink">{row.account_code}</span> }, { key: 'account_name', header: 'Tên tài khoản' }, { key: 'description', header: 'Diễn giải dòng' }, { key: 'debit_amount', header: 'Phát sinh Nợ', align: 'right', render: (row) => <CurrencyText value={row.debit_amount} /> }, { key: 'credit_amount', header: 'Phát sinh Có', align: 'right', render: (row) => <CurrencyText value={row.credit_amount} /> }]} empty="Chứng từ chưa có dòng định khoản" />{canReverse && !record.reversed && !record.reversal_of_id ? <div className="flex justify-end"><button className="inline-flex items-center gap-2 rounded-md border border-rose-200 bg-white px-3 py-2 text-sm font-semibold text-rose-700" onClick={onReverse}><RotateCcw className="h-4 w-4" />Lập bút toán đảo</button></div> : null}</div>;
}

function DetailValue({ label, value }: { label: string; value: unknown }) { return <div><p className="text-xs font-semibold uppercase text-slate-500">{label}</p><p className="mt-1 text-sm font-semibold text-ink">{String(value ?? '-')}</p></div>; }

function AccountingPeriodsTable({ rows, loading, error, canClose, canReopen, onRetry, onAction }: { rows: Record<string, any>[]; loading: boolean; error: unknown; canClose: boolean; canReopen: boolean; onRetry: () => void; onAction: (row: Record<string, any>, action: 'close' | 'reopen') => void }) {
  return <ListSection title="Kỳ kế toán" description="Khóa kỳ sau khi hoàn tất đối chiếu sổ chi tiết, công nợ, ngân sách và thuế."><DataTable rows={rows} loading={loading} error={error} onRetry={onRetry} pageSize={12} defaultSort={{ key: 'date_from', direction: 'desc' }} columns={[
    { key: 'period_no', header: 'Kỳ', sortable: true, render: (row) => <span className="font-semibold text-ink">Tháng {row.period_no}/{row.fiscal_year}</span> },
    { key: 'date_from', header: 'Từ ngày', render: (row) => dateText(row.date_from) },
    { key: 'date_to', header: 'Đến ngày', render: (row) => dateText(row.date_to) },
    { key: 'status', header: 'Trạng thái', render: (row) => <StatusBadge status={row.status} /> },
    { key: 'closed_at', header: 'Ngày khóa', render: (row) => dateText(row.closed_at) },
    { key: 'action', header: 'Thao tác', render: (row) => row.status === 'open' && canClose ? <button type="button" className="inline-flex items-center gap-1 text-sm font-semibold text-amber-700" onClick={() => onAction(row, 'close')}><LockKeyhole className="h-4 w-4" />Khóa kỳ</button> : row.status === 'closed' && canReopen ? <button type="button" className="inline-flex items-center gap-1 text-sm font-semibold text-blue-700" onClick={() => onAction(row, 'reopen')}><UnlockKeyhole className="h-4 w-4" />Mở lại</button> : null },
  ]} empty="Chưa có kỳ kế toán" /></ListSection>;
}

function LedgerReport({ trial, general, profit, loading, error, onRetry }: { trial?: AccountingReport; general?: AccountingReport; profit?: AccountingReport; loading: boolean; error: unknown; onRetry: () => void }) {
  const summary = trial?.summary ?? {};
  const profitSummary = profit?.summary ?? {};
  return <div className="space-y-4">
    <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
      <Metric label="Phát sinh Nợ" value={summary.period_debit} />
      <Metric label="Phát sinh Có" value={summary.period_credit} />
      <Metric label="Doanh thu" value={profitSummary.revenue_amount} />
      <Metric label="Kết quả kỳ" value={profitSummary.profit_amount} tone={Number(profitSummary.profit_amount ?? 0) < 0 ? 'rose' : 'green'} />
    </div>
    <ListSection title="Bảng cân đối số phát sinh"><DataTable rows={trial?.accounts ?? []} loading={loading} error={error} onRetry={onRetry} pageSize={20} searchPlaceholder="Tìm số hiệu hoặc tên tài khoản..." columns={[
      { key: 'account_code', header: 'Số hiệu TK', render: (row) => <span className="font-semibold text-ink">{row.account_code}</span> },
      { key: 'name', header: 'Tên tài khoản' },
      { key: 'opening_debit', header: 'Dư Nợ đầu kỳ', align: 'right', render: (row) => <CurrencyText value={row.opening_debit} /> },
      { key: 'opening_credit', header: 'Dư Có đầu kỳ', align: 'right', render: (row) => <CurrencyText value={row.opening_credit} /> },
      { key: 'period_debit', header: 'Phát sinh Nợ', align: 'right', render: (row) => <CurrencyText value={row.period_debit} /> },
      { key: 'period_credit', header: 'Phát sinh Có', align: 'right', render: (row) => <CurrencyText value={row.period_credit} /> },
      { key: 'closing_debit', header: 'Dư Nợ cuối kỳ', align: 'right', render: (row) => <CurrencyText value={row.closing_debit} /> },
      { key: 'closing_credit', header: 'Dư Có cuối kỳ', align: 'right', render: (row) => <CurrencyText value={row.closing_credit} /> },
    ]} empty="Chưa có phát sinh kế toán trong kỳ đã chọn" /></ListSection>
    <ListSection title="Sổ Cái" description="Chi tiết phát sinh theo tài khoản từ các Chứng từ ghi sổ đã hạch toán."><DataTable rows={general?.lines ?? []} loading={loading} error={error} onRetry={onRetry} pageSize={20} searchPlaceholder="Tìm tài khoản, số CTGS hoặc diễn giải..." columns={[
      { key: 'account_code', header: 'Số hiệu TK', render: (row) => <span className="font-semibold text-ink">{row.account_code}</span> },
      { key: 'account_name', header: 'Tên tài khoản' },
      { key: 'entry_date', header: 'Ngày hạch toán', sortable: true, render: (row) => dateText(row.entry_date) },
      { key: 'entry_no', header: 'Số CTGS', render: (row) => <JournalEntryNumber value={row.entry_no} /> },
      { key: 'description', header: 'Diễn giải', render: (row) => row.description || row.entry_description },
      { key: 'debit_amount', header: 'Phát sinh Nợ', align: 'right', render: (row) => <CurrencyText value={row.debit_amount} /> },
      { key: 'credit_amount', header: 'Phát sinh Có', align: 'right', render: (row) => <CurrencyText value={row.credit_amount} /> },
      { key: 'period_running_net', header: 'Số dư thuần trong kỳ', align: 'right', render: (row) => <CurrencyText value={row.period_running_net} /> },
    ]} empty="Chưa có dòng Sổ Cái trong kỳ đã chọn" /></ListSection>
    <ListSection title="Báo cáo kết quả hoạt động kinh doanh"><DataTable rows={profit?.accounts ?? []} loading={loading} error={error} onRetry={onRetry} pageSize={15} columns={[
      { key: 'account_code', header: 'Số hiệu TK', render: (row) => <span className="font-semibold text-ink">{row.account_code}</span> },
      { key: 'name', header: 'Chỉ tiêu' },
      { key: 'revenue_amount', header: 'Doanh thu', align: 'right', render: (row) => <CurrencyText value={row.revenue_amount} /> },
      { key: 'expense_amount', header: 'Chi phí', align: 'right', render: (row) => <CurrencyText value={row.expense_amount} /> },
    ]} empty="Chưa có doanh thu hoặc chi phí đã ghi sổ" /></ListSection>
  </div>;
}

function Metric({ label, value, tone = 'slate' }: { label: string; value: unknown; tone?: 'slate' | 'green' | 'amber' | 'rose' }) {
  const colors = { slate: 'border-line bg-slate-50', green: 'border-emerald-200 bg-emerald-50', amber: 'border-amber-200 bg-amber-50', rose: 'border-rose-200 bg-rose-50' };
  return <div className={`rounded-lg border p-3 ${colors[tone]}`}><div className="text-xs font-semibold uppercase tracking-wide text-slate-500">{label}</div><div className="mt-1 text-lg font-semibold text-ink"><CurrencyText value={Number(value ?? 0)} /></div></div>;
}

function sourceEventLabel(value: unknown) {
  return ({ input_invoice_verified: 'Ghi nhận hóa đơn đầu vào', input_invoice_rejected: 'Đảo hóa đơn đầu vào', output_invoice_issued: 'Ghi nhận hóa đơn đầu ra', output_invoice_voided: 'Đảo hóa đơn đầu ra', payable_payment_approved: 'Chi trả công nợ', receivable_receipt_approved: 'Thu công nợ', generic_outbound_approved: 'Phiếu chi khác', generic_inbound_approved: 'Phiếu thu khác', tax_adjustment_posted: 'Điều chỉnh thuế GTGT', tax_adjustment_reversed: 'Đảo điều chỉnh thuế GTGT', vat_payment_approved: 'Nộp thuế GTGT', payment_reversed: 'Đảo Phiếu thu/chi', manual_post: 'Chứng từ nghiệp vụ khác', reversal: 'Bút toán đảo' } as Record<string, string>)[String(value)] ?? String(value ?? '-');
}

function sourceDocumentLabel(value: unknown) {
  return ({ payment: 'Phiếu thu/chi', input_invoice: 'Hóa đơn đầu vào', output_invoice: 'Hóa đơn đầu ra', manual_adjustment: 'Chứng từ nghiệp vụ khác', tax_adjustment: 'Điều chỉnh thuế GTGT', journal_entry: 'Chứng từ ghi sổ gốc' } as Record<string, string>)[String(value)] ?? '-';
}
