import { useEffect, useMemo, useState } from 'react';
import { Link, useNavigate, useParams } from 'react-router-dom';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { AlertTriangle, CheckCircle2, DollarSign, Eye, History, Loader2, Lock, Plus, Send, ShoppingCart, ThumbsUp } from 'lucide-react';
import clsx from 'clsx';
import { get, post, put } from '../../lib/api/client';
import type { BOQItem, BOQProcurementResult, BOQUnitPriceEvent } from '../../lib/types/domain';
import { PageHeader } from '../../components/PageHeader';
import { DataTable } from '../../components/DataTable';
import { StatusBadge } from '../../components/StatusBadge';
import { CurrencyText } from '../../components/CurrencyText';
import { CurrencyInput } from '../../components/CurrencyInput';
import { useToast } from '../../components/ToastProvider';
import { RowActions } from '../../components/RowActions';
import { ProjectBackLink } from '../../components/ProjectBackLink';
import { Dialog } from '../../components/Dialog';
import { SimpleForm } from '../_shared/SimpleForm';
import { currency } from '../../lib/utils/format';
import { SupplierQuotationWorkspace } from './SupplierQuotationWorkspace';
import { useAuth } from '../../lib/auth/AuthContext';

type PriceAction = 'expected' | 'norm' | 'final';

type PriceFormState = {
  expected: string;
  norm: string;
  final: string;
  comment: string;
};

type WorkspaceTab = 'estimate' | 'quotes' | 'compare';

export function BOQPage() {
  const { id = '' } = useParams();
  const navigate = useNavigate();
  const { hasPermission } = useAuth();
  const canGenerateProcurement = hasPermission('procurement.create') || hasPermission('procurement.approve') || hasPermission('boq.approve');
  const [activeTab, setActiveTab] = useState<WorkspaceTab>('estimate');
  const [open, setOpen] = useState(false);
  const [dialog, setDialog] = useState<{ mode: 'view' | 'edit'; record: BOQItem } | null>(null);
  const [detailTab, setDetailTab] = useState<'overview' | 'prices' | 'quotes' | 'procurement'>('overview');
  const [reviseDialog, setReviseDialog] = useState<BOQItem | null>(null);
  const [generated, setGenerated] = useState<BOQProcurementResult | null>(null);
  const [priceDialog, setPriceDialog] = useState<BOQItem | null>(null);
  const [priceForm, setPriceForm] = useState<PriceFormState>({ expected: '', norm: '', final: '', comment: '' });
  const queryClient = useQueryClient();
  const toast = useToast();
  const boq = useQuery({ queryKey: ['boq', id], queryFn: () => get<BOQItem[]>(`/projects/${id}/boq`), enabled: Boolean(id) });
  const materials = useQuery({ queryKey: ['materials'], queryFn: () => get<Record<string, any>[]>('/materials'), enabled: hasPermission('catalog.view') });
  const detail = useQuery({
    queryKey: ['boq-detail', dialog?.record.id],
    queryFn: () => get<BOQItem>(`/boq-items/${dialog?.record.id}`),
    enabled: Boolean(dialog),
  });

  const invalidate = () => Promise.all([
    queryClient.invalidateQueries({ queryKey: ['boq', id] }),
    queryClient.invalidateQueries({ queryKey: ['boq-detail'] }),
    queryClient.invalidateQueries({ queryKey: ['supplier-quote-comparison', id] }),
  ]);
  const rows = boq.data ?? [];
  const total = useMemo(() => rows.reduce((sum, item) => sum + Number(item.effective_total_amount ?? item.total_amount ?? 0), 0), [rows]);
  const finalLockedTotal = useMemo(() => rows.filter((item) => item.final_price_locked).reduce((sum, item) => sum + Number(item.effective_total_amount ?? item.total_amount ?? 0), 0), [rows]);
  const pendingFinalTotal = Math.max(total - finalLockedTotal, 0);
  const priceReadiness = useMemo(() => boqPriceReadiness(rows), [rows]);
  const hasEditableRows = rows.some((row) => ['draft', 'revised'].includes(row.status));
  const submittedRows = rows.filter((row) => row.status === 'submitted');
  const canApproveSubmitted = submittedRows.length > 0 && submittedRows.every((row) => row.norm_price_locked);
  const canOpenComparison = !hasEditableRows && submittedRows.length === 0 && priceReadiness.approvedPendingFinal.length > 0;
  const estimatePrimaryAction = hasEditableRows && hasPermission('boq.update')
    ? 'submit'
    : canApproveSubmitted && hasPermission('boq.approve')
      ? 'approve'
      : canOpenComparison
        ? 'compare'
        : null;

  useEffect(() => {
    if (!priceDialog) {
      setPriceForm({ expected: '', norm: '', final: '', comment: '' });
      return;
    }
    setPriceForm({
      expected: priceInputValue(priceDialog.expected_unit_price ?? priceDialog.unit_price),
      norm: priceInputValue(priceDialog.norm_unit_price ?? priceDialog.expected_unit_price ?? priceDialog.unit_price),
      final: priceInputValue(priceDialog.final_unit_price ?? priceDialog.norm_unit_price ?? priceDialog.expected_unit_price ?? priceDialog.unit_price),
      comment: '',
    });
  }, [priceDialog]);

  const create = useMutation({
    mutationFn: (values: Record<string, any>) => post<BOQItem>(`/projects/${id}/boq`, values),
    onSuccess: () => {
      invalidate();
      setOpen(false);
      toast.success('Đã thêm dòng BOQ');
    },
    onError: (error) => toast.error('Không thêm được dòng BOQ', error),
  });

  const update = useMutation({
    mutationFn: ({ itemId, values }: { itemId: string; values: Record<string, any> }) => put<BOQItem>(`/boq-items/${itemId}`, values),
    onSuccess: (item) => {
      invalidate();
      setDialog((current) => (current ? { ...current, record: item } : current));
      queryClient.setQueryData(['boq-detail', item.id], item);
      toast.success('Đã cập nhật dòng BOQ');
    },
    onError: (error) => toast.error('Không cập nhật được dòng BOQ', error),
  });

  const submit = useMutation({
    mutationFn: () => post(`/projects/${id}/boq/submit`, {}),
    onSuccess: () => {
      invalidate();
      toast.success('Đã gửi dự toán để duyệt');
    },
    onError: (error) => toast.error('Không gửi được dự toán', error),
  });

  const approve = useMutation({
    mutationFn: () => post(`/projects/${id}/boq/approve`, {}),
    onSuccess: () => {
      invalidate();
      toast.success('Đã duyệt BOQ');
    },
    onError: (error) => toast.error('Không duyệt được BOQ', error),
  });

  const generate = useMutation({
    mutationFn: () => post<BOQProcurementResult>(`/projects/${id}/boq/generate-procurement`, {}),
    onSuccess: (result) => {
      invalidate();
      queryClient.invalidateQueries({ queryKey: ['procurement'] });
      queryClient.invalidateQueries({ queryKey: ['procurement-detail'] });
      setGenerated(result);
      toast.success('Đã sinh yêu cầu mua sắm');
    },
    onError: (error) => toast.error('Không sinh được yêu cầu mua sắm', error),
  });

  const revise = useMutation({
    mutationFn: ({ itemId, reason }: { itemId: string; reason: string }) => post<BOQItem>(`/boq-items/${itemId}/revise`, { reason }),
    onSuccess: async () => {
      await invalidate();
      setReviseDialog(null);
      toast.success('Đã chuyển dòng dự toán sang chỉnh sửa');
    },
    onError: (error) => toast.error('Không thể yêu cầu chỉnh sửa dòng dự toán', error),
  });

  const updateExpected = useMutation({
    mutationFn: ({ itemId, unitPrice, comment }: { itemId: string; unitPrice: number; comment: string }) =>
      post<BOQItem>(`/boq-items/${itemId}/unit-price/expected`, { unit_price: unitPrice, comment }),
    onSuccess: (item) => {
      invalidate();
      setPriceDialog(item);
      toast.success('Đã cập nhật đơn giá dự kiến');
    },
    onError: (error) => toast.error('Không cập nhật được đơn giá dự kiến', error),
  });

  const lockNorm = useMutation({
    mutationFn: ({ itemId, unitPrice, comment }: { itemId: string; unitPrice: number; comment: string }) =>
      post<BOQItem>(`/boq-items/${itemId}/unit-price/lock-norm`, { unit_price: unitPrice, comment }),
    onSuccess: (item) => {
      invalidate();
      setPriceDialog(item);
      toast.success('Đã khóa đơn giá định mức');
    },
    onError: (error) => toast.error('Không khóa được đơn giá định mức', error),
  });

  const lockFinal = useMutation({
    mutationFn: ({ itemId, unitPrice, comment }: { itemId: string; unitPrice: number; comment: string }) =>
      post<BOQItem>(`/boq-items/${itemId}/unit-price/lock-final`, { unit_price: unitPrice, comment }),
    onSuccess: (item) => {
      invalidate();
      setPriceDialog(item);
      toast.success('Đã chốt đơn giá cuối');
    },
    onError: (error) => toast.error('Không chốt được đơn giá cuối', error),
  });

  const pendingPriceAction = updateExpected.isPending || lockNorm.isPending || lockFinal.isPending;

  const boqFields = (record?: BOQItem) => [
    { name: 'category', label: 'Hạng mục', defaultValue: record?.category ?? 'Điện', required: true },
    { name: 'work_item_name', label: 'Tên công việc', defaultValue: record?.work_item_name ?? 'Hạng mục mới', required: true },
    { name: 'description', label: 'Mô tả', type: 'textarea' as const, defaultValue: record?.description ?? '' },
    { name: 'unit', label: 'Đơn vị', defaultValue: record?.unit ?? 'm2', required: true, disabledWhen: () => Boolean(record?.norm_price_locked), helperText: record?.norm_price_locked ? 'Định mức đã khóa; không thể đổi đơn vị trong lần chỉnh sửa mô tả.' : undefined },
    { name: 'quantity', label: 'Khối lượng', type: 'number' as const, defaultValue: record?.quantity ?? 1, required: true, min: 0, disabledWhen: () => Boolean(record?.norm_price_locked), helperText: record?.norm_price_locked ? 'Định mức đã khóa; không thể đổi khối lượng.' : undefined },
    {
      name: 'expected_unit_price',
      label: 'Đơn giá dự kiến',
      type: 'currency' as const,
      defaultValue: record?.expected_unit_price ?? record?.unit_price ?? 1000000,
      required: true,
      min: 0,
      disabledWhen: () => Boolean(record?.norm_price_locked || record?.final_price_locked),
      helperText: record?.norm_price_locked || record?.final_price_locked ? 'Định mức đã khóa; chỉnh sửa này không mở lại giá.' : 'Giá dự kiến do QS nhập trước khi khóa định mức.',
    },
    { name: 'material_id', label: 'Vật tư liên kết', type: 'select' as const, defaultValue: record?.material_id ?? '', options: (materials.data ?? []).map((m) => ({ label: m.name, value: m.id })), disabledWhen: () => Boolean(record?.norm_price_locked || !hasPermission('catalog.view')), helperText: record?.norm_price_locked ? 'Định mức đã khóa; không thể đổi vật tư liên kết.' : !hasPermission('catalog.view') ? 'Anh/chị không có quyền xem danh mục vật tư; trường này chỉ đọc.' : undefined },
  ];

  function submitPriceAction(action: PriceAction) {
    if (!priceDialog) return;
    const raw = action === 'expected' ? priceForm.expected : action === 'norm' ? priceForm.norm : priceForm.final;
    const unitPrice = Number(raw);
    if (!Number.isFinite(unitPrice) || unitPrice < 0 || (action !== 'expected' && unitPrice <= 0)) {
      toast.warn({ title: 'Đơn giá không hợp lệ', description: action === 'expected' ? 'Đơn giá dự kiến phải lớn hơn hoặc bằng 0.' : 'Giá khóa phải lớn hơn 0.' });
      return;
    }
    if (action === 'final' && !priceForm.comment.trim()) {
      toast.warn({ title: 'Thiếu lý do miễn báo giá', description: 'Khóa giá cuối thủ công chỉ dùng khi có lý do miễn so sánh báo giá.' });
      return;
    }
    const payload = { itemId: priceDialog.id, unitPrice, comment: priceForm.comment };
    if (action === 'expected') updateExpected.mutate(payload);
    if (action === 'norm') lockNorm.mutate(payload);
    if (action === 'final') lockFinal.mutate(payload);
  }

  return (
    <>
      <PageHeader
        title="Dự toán & báo giá"
        description="Quản lý dòng dự toán, thu thập báo giá và chốt giá trước khi chuyển sang mua sắm."
        actions={
          <>
            <ProjectBackLink projectId={id} />
            {activeTab === 'estimate' && hasPermission('boq.create') ? <button className={clsx('inline-flex items-center gap-2 rounded-md px-3 py-2 text-sm font-semibold', rows.length ? 'border border-line bg-white text-slate-700 hover:bg-slate-50' : 'bg-blue-600 text-white hover:bg-blue-700')} onClick={() => setOpen(true)}>
              <Plus className="h-4 w-4" />
              Thêm dòng dự toán
            </button> : null}
          </>
        }
      />

      <div className="mb-4 flex gap-1 overflow-x-auto rounded-lg border border-line bg-white p-1">
        <TabButton active={activeTab === 'estimate'} onClick={() => setActiveTab('estimate')}>Dòng dự toán</TabButton>
        <TabButton active={activeTab === 'quotes'} onClick={() => setActiveTab('quotes')}>Báo giá nhà cung cấp</TabButton>
        <TabButton active={activeTab === 'compare'} onClick={() => setActiveTab('compare')}>So sánh & chốt giá</TabButton>
      </div>

      <div className="mb-4 grid gap-3 md:grid-cols-3">
        <MetricCard label="Tổng dự toán hiệu lực" value={total} tone="blue" />
        <MetricCard label="Đã chốt giá cuối" value={finalLockedTotal} tone="emerald" />
        <MetricCard label="Chờ chốt giá" value={pendingFinalTotal} tone={pendingFinalTotal > 0 ? 'amber' : 'slate'} />
      </div>

      {activeTab === 'estimate' ? <>
        {estimatePrimaryAction ? (
          <div className="mb-4 rounded-lg border border-line bg-white p-3 shadow-panel">
            <div className="flex flex-wrap justify-end gap-2">
              {estimatePrimaryAction === 'submit' ? (
                <button className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white disabled:opacity-50" disabled={submit.isPending} onClick={() => submit.mutate()}>{submit.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <Send className="h-4 w-4" />}Gửi duyệt</button>
              ) : estimatePrimaryAction === 'approve' ? (
                <button className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white disabled:opacity-50" disabled={approve.isPending} onClick={() => approve.mutate()}>{approve.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <ThumbsUp className="h-4 w-4" />}Duyệt dự toán</button>
              ) : (
                <button className="rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white" onClick={() => setActiveTab('compare')}>Mở so sánh giá</button>
              )}
            </div>
          </div>
        ) : null}
        <UnitPriceReadinessPanel readiness={priceReadiness} onOpenItem={setPriceDialog} />
        <DataTable
        rows={rows}
        loading={boq.isLoading}
        error={boq.error}
        onRetry={() => boq.refetch()}
        searchPlaceholder="Tìm hạng mục, công việc, trạng thái"
        pageSize={10}
        defaultSort={{ key: 'work_item_name', direction: 'asc' }}
        columns={[
          { key: 'category', header: 'Hạng mục', sortable: true },
          { key: 'work_item_name', header: 'Công việc', sortable: true, render: (row) => <span className="font-semibold text-ink">{row.work_item_name}</span> },
          { key: 'quantity', header: 'KL', align: 'right', sortable: true, sortValue: (row) => Number(row.quantity ?? 0), render: (row) => <span>{row.quantity} {row.unit}</span> },
          { key: 'price_lifecycle', header: 'Vòng đời đơn giá', sortable: true, sortValue: (row) => Number(row.effective_unit_price ?? row.unit_price ?? 0), render: (row) => <PriceLifecycleCell item={row} /> },
          { key: 'effective_total_amount', header: 'Thành tiền', align: 'right', sortable: true, sortValue: (row) => Number(row.effective_total_amount ?? row.total_amount ?? 0), render: (row) => <CurrencyText value={row.effective_total_amount ?? row.total_amount} /> },
          { key: 'status', header: 'Trạng thái', sortable: true, render: (row) => <StatusBadge status={row.status} /> },
          {
            key: 'action',
            header: 'Thao tác',
            align: 'right',
            render: (row) => (
              <RowActions onView={() => { setDetailTab('overview'); setDialog({ mode: 'view', record: row }); }} onEdit={['draft', 'revised'].includes(row.status) && !row.final_price_locked && hasPermission('boq.update') ? () => setDialog({ mode: 'edit', record: row }) : undefined}>
                <button
                  title="Đơn giá và lịch sử"
                  aria-label="Đơn giá và lịch sử"
                  className="rounded-md border border-line p-1.5 text-blue-700 hover:bg-blue-50"
                  onClick={() => setPriceDialog(row)}
                >
                  <DollarSign className="h-4 w-4" />
                </button>
                {['submitted', 'approved'].includes(row.status) && !row.final_price_locked && !row.has_active_procurement && Number(row.active_procurement_count ?? 0) === 0 && hasPermission('boq.update') ? <button className="text-sm font-semibold text-amber-700" onClick={() => setReviseDialog(row)}>Yêu cầu chỉnh sửa</button> : null}
              </RowActions>
            ),
          },
        ]}
        />
      </> : null}

      {activeTab === 'quotes' ? <SupplierQuotationWorkspace projectId={id} boqItems={rows} mode="quotes" /> : null}
      {activeTab === 'compare' ? <SupplierQuotationWorkspace projectId={id} boqItems={rows} mode="compare" /> : null}
      {activeTab === 'compare' && canGenerateProcurement ? <div className="mt-4 flex justify-end"><button className="inline-flex items-center gap-2 rounded-md bg-emerald-600 px-4 py-2 text-sm font-semibold text-white disabled:opacity-50" disabled={generate.isPending || priceReadiness.generateDisabled} title={priceReadiness.generateDisabledReason} onClick={() => generate.mutate()}>{generate.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <ShoppingCart className="h-4 w-4" />}Tạo yêu cầu mua sắm</button></div> : null}

      <Dialog open={open} title="Thêm dòng dự toán" description="Trạng thái khởi tạo do hệ thống quản lý." onClose={() => setOpen(false)} size="lg">
        <SimpleForm
          submitLabel="Thêm dòng"
          fields={boqFields()}
          layout="two-column"
          onCancel={() => setOpen(false)}
          onSubmit={(values) => create.mutateAsync({ ...values, material_id: values.material_id || null })}
        />
      </Dialog>

      <Dialog open={dialog?.mode === 'view'} title="Chi tiết dòng dự toán" description={dialog?.record.work_item_name} onClose={() => setDialog(null)} size="xl">
        {detail.isLoading ? <LoadingState /> : detail.error ? <ErrorState onRetry={() => detail.refetch()} /> : detail.data ? <BOQDetail item={detail.data} tab={detailTab} onTabChange={setDetailTab} onOpenPrices={() => setPriceDialog(detail.data)} projectId={id} canViewProcurement={hasPermission('procurement.view')} /> : null}
      </Dialog>

      <Dialog open={dialog?.mode === 'edit'} title={dialog?.record.norm_price_locked ? 'Chỉnh sửa mô tả dòng dự toán' : 'Chỉnh sửa dòng dự toán'} description={dialog?.record.norm_price_locked ? 'Định mức đã khóa; lần chỉnh sửa này không mở lại đơn giá, khối lượng, đơn vị hoặc vật tư.' : 'Trạng thái không được thay đổi trong biểu mẫu này.'} onClose={() => setDialog(null)} size="lg">
        {dialog?.record ? (
          <SimpleForm
            submitLabel="Lưu thay đổi"
            fields={boqFields(dialog.record)}
            layout="two-column"
            onCancel={() => setDialog(null)}
            onSubmit={(values) => update.mutateAsync({ itemId: dialog.record.id, values: dialog.record.norm_price_locked ? { category: values.category, work_item_name: values.work_item_name, description: values.description } : { ...values, material_id: values.material_id || null } })}
          />
        ) : null}
      </Dialog>

      <ReviseDialog item={reviseDialog} pending={revise.isPending} onClose={() => setReviseDialog(null)} onSubmit={(reason) => reviseDialog && revise.mutate({ itemId: reviseDialog.id, reason })} />
      <GeneratedProcurementDialog result={generated} canView={hasPermission('procurement.view')} onClose={() => setGenerated(null)} onNavigate={() => navigate(`/projects/${id}/procurement`)} />

      <UnitPriceLifecycleDialog
        item={priceDialog}
        form={priceForm}
        pending={pendingPriceAction}
        onChange={setPriceForm}
        onClose={() => setPriceDialog(null)}
        onSubmit={submitPriceAction}
        canUpdate={hasPermission('boq.update')}
        canApprove={hasPermission('boq.approve')}
      />
    </>
  );
}

function TabButton({ active, onClick, children }: { active: boolean; onClick: () => void; children: React.ReactNode }) {
  return <button type="button" onClick={onClick} className={clsx('whitespace-nowrap rounded-md px-4 py-2 text-sm font-semibold', active ? 'bg-blue-600 text-white' : 'text-slate-600 hover:bg-slate-50')}>{children}</button>;
}

function LoadingState() {
  return <div className="flex items-center gap-2 py-10 text-sm text-slate-500"><Loader2 className="h-4 w-4 animate-spin" />Đang tải chi tiết...</div>;
}

function ErrorState({ onRetry }: { onRetry: () => void }) {
  return <div className="flex items-center justify-between rounded-md border border-rose-200 bg-rose-50 p-3 text-sm text-rose-700"><span>Không tải được chi tiết dòng dự toán.</span><button className="font-semibold underline" onClick={onRetry}>Thử lại</button></div>;
}

function BOQDetail({ item, tab, onTabChange, onOpenPrices, projectId, canViewProcurement }: { item: BOQItem; tab: 'overview' | 'prices' | 'quotes' | 'procurement'; onTabChange: (tab: 'overview' | 'prices' | 'quotes' | 'procurement') => void; onOpenPrices: () => void; projectId: string; canViewProcurement: boolean }) {
  return <div className="space-y-4">
    {Number(item.active_procurement_count ?? 0) > 1 ? <div className="flex items-start gap-2 rounded-md border border-amber-200 bg-amber-50 p-3 text-sm text-amber-800"><AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" /><span>Dòng dự toán đang liên kết với {item.active_procurement_count} yêu cầu mua sắm hoạt động. Mã hiển thị bên dưới là liên kết gần nhất; cần kiểm tra toàn bộ tại màn hình mua sắm.</span></div> : null}
    <div className="flex gap-1 overflow-x-auto border-b border-line pb-2">
      <DetailTab active={tab === 'overview'} onClick={() => onTabChange('overview')}>Tổng quan</DetailTab>
      <DetailTab active={tab === 'prices'} onClick={() => onTabChange('prices')}>Đơn giá & lịch sử</DetailTab>
      <DetailTab active={tab === 'quotes'} onClick={() => onTabChange('quotes')}>Báo giá</DetailTab>
      <DetailTab active={tab === 'procurement'} onClick={() => onTabChange('procurement')}>Mua sắm</DetailTab>
    </div>
    {tab === 'overview' ? <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4"><DetailMetric label="Hạng mục" value={item.category} /><DetailMetric label="Khối lượng" value={`${item.quantity.toLocaleString('vi-VN')} ${item.unit}`} /><DetailMetric label="Thành tiền" value={<CurrencyText value={item.effective_total_amount ?? item.total_amount} />} /><DetailMetric label="Trạng thái" value={<StatusBadge status={item.status} />} /><div className="sm:col-span-2 lg:col-span-4 rounded-md border border-line p-3"><p className="text-xs font-semibold uppercase text-slate-500">Mô tả</p><p className="mt-1 text-sm text-slate-700">{item.description || 'Chưa có mô tả.'}</p></div></div> : null}
    {tab === 'prices' ? <div className="grid gap-4 lg:grid-cols-[1fr_1fr]"><div className="space-y-3"><PriceLifecycleCell item={item} /><button className="rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white" onClick={onOpenPrices}>Mở quản lý đơn giá</button></div><div>{item.price_events_truncated ? <PriceHistoryNotice count={item.price_event_count} /> : null}{item.price_events?.length ? <div className="space-y-2">{item.price_events.map((event) => <PriceEventRow key={event.id} event={event} />)}</div> : <div className="rounded-md border border-dashed border-line p-4 text-sm text-slate-500">Chưa có lịch sử đơn giá.</div>}</div></div> : null}
    {tab === 'quotes' ? <div className="grid gap-3 sm:grid-cols-2"><DetailMetric label="Báo giá đã chọn" value={item.selected_quotation_code || 'Chưa chọn'} /><DetailMetric label="Nhà cung cấp" value={item.selected_supplier_name || 'Chưa chọn'} /><DetailMetric label="Đơn giá báo giá" value={item.quoted_unit_price ? <CurrencyText value={item.quoted_unit_price} /> : '-'} /><DetailMetric label="Căn cứ miễn báo giá" value={item.price_waiver_reason || 'Không có'} /></div> : null}
    {tab === 'procurement' ? item.procurement_request_id || item.has_active_procurement || Number(item.active_procurement_count ?? 0) > 0 ? <div className="rounded-md border border-emerald-200 bg-emerald-50 p-4"><p className="font-semibold text-emerald-800">Đã liên kết yêu cầu mua {item.procurement_request_code || item.procurement_request_id || ''}</p><div className="mt-3 flex items-center gap-3"><StatusBadge status={item.procurement_request_status || 'draft'} />{canViewProcurement ? <Link className="inline-flex items-center gap-1 text-sm font-semibold text-blue-700 hover:underline" to={`/projects/${projectId}/procurement`}><Eye className="h-4 w-4" />Mở mua sắm</Link> : null}</div></div> : <div className="rounded-md border border-dashed border-line p-4 text-sm text-slate-500">Dòng này chưa được liên kết với yêu cầu mua sắm đang hoạt động.</div> : null}
  </div>;
}

function DetailTab({ active, onClick, children }: { active: boolean; onClick: () => void; children: React.ReactNode }) { return <button className={clsx('whitespace-nowrap rounded-md px-3 py-2 text-sm font-semibold', active ? 'bg-slate-900 text-white' : 'text-slate-600 hover:bg-slate-100')} onClick={onClick}>{children}</button>; }
function DetailMetric({ label, value }: { label: string; value: React.ReactNode }) { return <div className="rounded-md border border-line bg-slate-50 p-3"><p className="text-xs font-semibold uppercase text-slate-500">{label}</p><div className="mt-1 font-semibold text-ink">{value}</div></div>; }

function ReviseDialog({ item, pending, onClose, onSubmit }: { item: BOQItem | null; pending: boolean; onClose: () => void; onSubmit: (reason: string) => void }) {
  const [reason, setReason] = useState('');
  useEffect(() => setReason(''), [item]);
  return <Dialog open={Boolean(item)} title="Yêu cầu chỉnh sửa dòng dự toán" description={item?.work_item_name} onClose={onClose} footer={<div className="flex justify-end gap-2"><button className="rounded-md border border-line px-3 py-2 text-sm font-semibold" onClick={onClose}>Hủy</button><button className="rounded-md bg-amber-600 px-3 py-2 text-sm font-semibold text-white disabled:opacity-50" disabled={pending || !reason.trim()} onClick={() => onSubmit(reason.trim())}>Xác nhận chỉnh sửa</button></div>}><label className="block"><span className="text-sm font-semibold text-slate-700">Lý do chỉnh sửa *</span><textarea className="mt-1 min-h-28 w-full rounded-md border border-line px-3 py-2 text-sm" value={reason} onChange={(event) => setReason(event.target.value)} placeholder="Nêu rõ nội dung cần điều chỉnh và căn cứ..." /></label></Dialog>;
}

function GeneratedProcurementDialog({ result, canView, onClose, onNavigate }: { result: BOQProcurementResult | null; canView: boolean; onClose: () => void; onNavigate: () => void }) {
  const requests = result?.created_requests ?? result?.requests ?? (result?.id ? [{ id: result.id, request_code: result.request_code, status: result.status }] : []);
  return <Dialog open={Boolean(result)} title="Đã tạo yêu cầu mua sắm" description="Các dòng đã chốt được nhóm theo nhà cung cấp và giữ nguyên dấu vết báo giá hoặc lý do miễn báo giá." onClose={onClose} footer={<div className="flex justify-end gap-2"><button className="rounded-md border border-line px-3 py-2 text-sm font-semibold" onClick={onClose}>Đóng</button>{canView ? <button className="rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white" onClick={onNavigate}>Mở mua sắm</button> : null}</div>}>
    {requests.length ? <div className="space-y-2">{requests.map((request) => <div key={request.id} className="flex items-center justify-between rounded-md border border-line bg-slate-50 p-3"><div><p className="font-semibold text-ink">{request.request_code || request.id}</p><p className="text-xs text-slate-500">{request.supplier_name || 'Nhà cung cấp theo báo giá đã chọn'}</p></div><StatusBadge status={request.status || 'draft'} /></div>)}</div> : <div className="rounded-md border border-dashed border-line p-4 text-sm text-slate-500">Yêu cầu đã được tạo. Mở màn hình mua sắm để xem mã và trạng thái.</div>}
  </Dialog>;
}

function PriceHistoryNotice({ count }: { count?: number }) {
  return <div className="mb-3 rounded-md border border-blue-200 bg-blue-50 p-3 text-sm text-blue-800">Đang hiển thị các thay đổi đơn giá gần nhất trong tổng số {Number(count ?? 0).toLocaleString('vi-VN')} bản ghi lịch sử.</div>;
}

function MetricCard({ label, value, tone }: { label: string; value: number; tone: 'blue' | 'emerald' | 'amber' | 'slate' }) {
  return (
    <div className={clsx(
      'rounded-lg border bg-white p-4 shadow-panel',
      tone === 'blue' && 'border-blue-100',
      tone === 'emerald' && 'border-emerald-100',
      tone === 'amber' && 'border-amber-100',
      tone === 'slate' && 'border-line',
    )}>
      <p className="text-sm text-slate-500">{label}</p>
      <p className="mt-1 text-2xl font-semibold text-ink"><CurrencyText value={value} /></p>
    </div>
  );
}

function UnitPriceReadinessPanel({ readiness, onOpenItem }: { readiness: BOQPriceReadiness; onOpenItem: (item: BOQItem) => void }) {
  const blocked = !readiness.procurementComplete && (readiness.approvedPendingFinal.length > 0 || readiness.approvedFinalCount === 0);
  return (
    <section className={clsx('mb-4 rounded-md border p-3', blocked ? 'border-amber-200 bg-amber-50' : 'border-emerald-200 bg-emerald-50')}>
      <div className="flex flex-col gap-3 xl:flex-row xl:items-start xl:justify-between">
        <div className="flex min-w-0 items-start gap-3">
          <span className={clsx('mt-0.5 rounded-md bg-white/80 p-2 ring-1 ring-inset', blocked ? 'text-amber-700 ring-amber-100' : 'text-emerald-700 ring-emerald-100')}>
            {blocked ? <AlertTriangle className="h-4 w-4" /> : <CheckCircle2 className="h-4 w-4" />}
          </span>
          <div className="min-w-0">
            <p className="text-sm font-semibold text-ink">
              {readiness.procurementComplete ? 'Trạng thái' : 'Việc tiếp theo'}: {readiness.nextAction}
            </p>
            <p className="mt-1 text-xs leading-5 text-slate-600">
              {readiness.generateDisabledReason || 'Các dòng đã duyệt đã sẵn sàng sinh mua sắm bằng đơn giá cuối.'}
            </p>
          </div>
        </div>
        <div className="grid gap-2 text-xs font-semibold text-slate-600 sm:grid-cols-5 xl:w-[44rem]">
          <ReadinessPill label="Giá dự kiến" value={`${readiness.expectedCount} dòng`} active={readiness.expectedCount > 0} />
          <ReadinessPill label="Đã khóa định mức" value={`${readiness.normLockedCount} dòng`} active={readiness.normLockedCount > 0} />
          <ReadinessPill label="Đã chốt giá cuối" value={`${readiness.finalLockedCount} dòng`} active={false} />
          <ReadinessPill label="Đã duyệt" value={`${readiness.approvedCount} dòng`} active={readiness.approvedCount === 0} />
          <ReadinessPill
            label={readiness.procurementComplete ? 'Đã chuyển mua sắm' : 'Đang chặn'}
            value={`${readiness.procurementComplete ? readiness.procuredCount : readiness.approvedPendingFinal.length} dòng`}
            active={!readiness.procurementComplete && readiness.approvedPendingFinal.length > 0}
          />
        </div>
      </div>
      {readiness.approvedPendingFinal.length ? (
        <div className="mt-3 grid gap-2 md:grid-cols-2 xl:grid-cols-3">
          {readiness.approvedPendingFinal.slice(0, 3).map((item) => (
            <button
              key={item.id}
              type="button"
              className="min-w-0 rounded-md border border-amber-200 bg-white px-3 py-2 text-left hover:bg-amber-50"
              onClick={() => onOpenItem(item)}
            >
              <p className="truncate text-sm font-semibold text-ink">{item.work_item_name}</p>
              <p className="mt-1 text-xs text-slate-500">
                {item.norm_price_locked ? 'Đã khóa định mức, chờ chốt giá cuối' : 'Chưa khóa định mức hoặc giá cuối'} · lần {item.price_revision_no ?? 0}
              </p>
            </button>
          ))}
          {readiness.approvedPendingFinal.length > 3 ? (
            <div className="rounded-md border border-amber-200 bg-white px-3 py-2 text-sm font-semibold text-amber-800">
              +{readiness.approvedPendingFinal.length - 3} dòng đã duyệt khác chưa chốt giá cuối
            </div>
          ) : null}
        </div>
      ) : null}
    </section>
  );
}

function ReadinessPill({ label, value, active }: { label: string; value: string; active: boolean }) {
  return (
    <div className={clsx('min-w-0 rounded-md px-2.5 py-2 ring-1 ring-inset', active ? 'bg-white text-amber-800 ring-amber-100' : 'bg-white/75 text-slate-600 ring-white')}>
      <span className="block text-[11px] uppercase text-slate-500">{label}</span>
      <span className="block truncate">{value}</span>
    </div>
  );
}

function PriceLifecycleCell({ item }: { item: BOQItem }) {
  return (
    <div className="min-w-[220px] space-y-1">
      <div className="flex items-center justify-between gap-3 text-xs">
        <span className="font-semibold text-slate-500">Dự kiến</span>
        <span className="font-semibold text-ink">{currency(item.expected_unit_price ?? item.unit_price)}</span>
      </div>
      <div className="flex items-center justify-between gap-3 text-xs">
        <span className="font-semibold text-slate-500">Định mức</span>
        <span className={clsx('font-semibold', item.norm_price_locked ? 'text-emerald-700' : 'text-slate-400')}>{item.norm_price_locked ? currency(item.norm_unit_price) : 'Chưa khóa'}</span>
      </div>
      <div className="flex items-center justify-between gap-3 text-xs">
        <span className="font-semibold text-slate-500">Giá cuối</span>
        <span className={clsx('font-semibold', item.final_price_locked ? 'text-blue-700' : 'text-slate-400')}>{item.final_price_locked ? currency(item.final_unit_price) : 'Chưa khóa'}</span>
      </div>
      <PriceStatusChip item={item} />
    </div>
  );
}

function PriceStatusChip({ item }: { item: BOQItem }) {
  const status = item.final_price_locked ? 'Đã chốt giá cuối' : item.norm_price_locked ? 'Đã khóa định mức' : 'Giá dự kiến';
  const Icon = item.final_price_locked ? Lock : item.norm_price_locked ? CheckCircle2 : DollarSign;
  return (
    <span className={clsx(
      'inline-flex items-center gap-1 rounded-md px-2 py-1 text-xs font-semibold ring-1 ring-inset',
      item.final_price_locked && 'bg-blue-50 text-blue-700 ring-blue-200',
      item.norm_price_locked && !item.final_price_locked && 'bg-emerald-50 text-emerald-700 ring-emerald-200',
      !item.norm_price_locked && !item.final_price_locked && 'bg-slate-50 text-slate-600 ring-slate-200',
    )}>
      <Icon className="h-3.5 w-3.5" />
      {status} · lần {item.price_revision_no ?? 0}
    </span>
  );
}

function UnitPriceLifecycleDialog({
  item,
  form,
  pending,
  onChange,
  onClose,
  onSubmit,
  canUpdate,
  canApprove,
}: {
  item: BOQItem | null;
  form: PriceFormState;
  pending: boolean;
  onChange: (form: PriceFormState) => void;
  onClose: () => void;
  onSubmit: (action: PriceAction) => void;
  canUpdate: boolean;
  canApprove: boolean;
}) {
  if (!item) return null;
  const setField = (field: keyof PriceFormState, value: string) => onChange({ ...form, [field]: value });
  const nextAction = unitPriceNextAction(item);
  const procurementImpact = unitPriceProcurementImpact(item);
  return (
    <Dialog
      open={Boolean(item)}
      title="Vòng đời đơn giá"
      description={`${item.work_item_name} · ${item.quantity} ${item.unit} · giá hiệu lực ${currency(item.effective_unit_price ?? item.unit_price)}`}
      onClose={onClose}
      size="xl"
      footer={
        <div className="flex justify-end">
          <button className="rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100" onClick={onClose}>Đóng</button>
        </div>
      }
    >
      <div className={clsx('mb-4 rounded-md border p-3', item.final_price_locked ? 'border-emerald-200 bg-emerald-50' : 'border-amber-200 bg-amber-50')}>
        <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
          <div className="min-w-0">
            <p className="text-sm font-semibold text-ink">Việc tiếp theo: {nextAction}</p>
            <p className="mt-1 text-xs leading-5 text-slate-600">{procurementImpact}</p>
          </div>
          <PriceStatusChip item={item} />
        </div>
      </div>
      <div className="grid gap-4 lg:grid-cols-[1.1fr_0.9fr]">
        <div className="space-y-3">
          <PriceStagePanel
            title="Đơn giá dự kiến"
            description="Giá QS dự kiến, chỉ sửa trước khi khóa định mức hoặc giá cuối."
            value={form.expected}
            disabled={pending || !canUpdate || !['draft', 'revised'].includes(item.status) || Boolean(item.norm_price_locked || item.final_price_locked)}
            buttonLabel="Cập nhật giá dự kiến"
            onValueChange={(value) => setField('expected', value)}
            onSubmit={() => onSubmit('expected')}
          />
          <PriceStagePanel
            title="Đơn giá định mức"
            description="Chốt định mức nội bộ. Sau khi khóa, đơn giá dự kiến không còn được sửa."
            value={form.norm}
            disabled={pending || !canApprove || item.status !== 'submitted' || Boolean(item.norm_price_locked || item.final_price_locked)}
            buttonLabel={item.norm_price_locked ? 'Đã khóa định mức' : 'Khóa định mức'}
            onValueChange={(value) => setField('norm', value)}
            onSubmit={() => onSubmit('norm')}
          />
          {item.status === 'approved' ? <PriceStagePanel
            title="Đơn giá cuối"
            description="Ưu tiên chọn tại bảng so sánh báo giá. Khóa thủ công cần ghi rõ lý do miễn báo giá."
            value={form.final}
            disabled={pending || !canApprove || item.status !== 'approved' || Boolean(!item.norm_price_locked || item.final_price_locked)}
            buttonLabel={item.final_price_locked ? 'Đã chốt giá cuối' : 'Chốt giá cuối'}
            onValueChange={(value) => setField('final', value)}
            onSubmit={() => onSubmit('final')}
          /> : null}
          <label className="block">
            <span className="text-sm font-semibold text-slate-700">Ghi chú</span>
            <textarea
              className="mt-1 w-full rounded-md border border-line bg-white px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-100 disabled:bg-slate-100"
              rows={3}
              value={form.comment}
              disabled={pending}
              onChange={(event) => setField('comment', event.target.value)}
              placeholder="Ghi chú lý do cập nhật hoặc khóa giá"
            />
          </label>
        </div>
        <div className="rounded-md border border-line bg-slate-50 p-3">
          <div className="mb-3 flex items-center justify-between gap-2">
            <h3 className="text-sm font-semibold text-ink">Lịch sử đơn giá gần nhất</h3>
            <History className="h-4 w-4 text-slate-400" />
          </div>
          <PriceLockMetadata item={item} />
          {item.price_events_truncated ? <PriceHistoryNotice count={item.price_event_count} /> : null}
          {item.price_events?.length ? (
            <div className="space-y-2">
              {item.price_events.map((event) => <PriceEventRow key={event.id} event={event} />)}
            </div>
          ) : (
            <div className="rounded-md border border-dashed border-line bg-white px-3 py-4 text-sm font-medium text-slate-500">Chưa có lịch sử thay đổi đơn giá.</div>
          )}
        </div>
      </div>
    </Dialog>
  );
}

function PriceStagePanel({
  title,
  description,
  value,
  disabled,
  buttonLabel,
  onValueChange,
  onSubmit,
}: {
  title: string;
  description: string;
  value: string;
  disabled: boolean;
  buttonLabel: string;
  onValueChange: (value: string) => void;
  onSubmit: () => void;
}) {
  return (
    <div className="rounded-md border border-line bg-white p-3">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0">
          <h3 className="text-sm font-semibold text-ink">{title}</h3>
          <p className="mt-1 text-xs leading-5 text-slate-500">{description}</p>
        </div>
        <button
          type="button"
          className="inline-flex items-center gap-2 rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100 disabled:cursor-not-allowed disabled:opacity-50"
          disabled={disabled}
          onClick={onSubmit}
        >
          {disabled ? <Lock className="h-4 w-4" /> : <DollarSign className="h-4 w-4" />}
          {buttonLabel}
        </button>
      </div>
      <CurrencyInput
        className="mt-3"
        inputClassName="text-sm"
        min={0}
        value={value}
        disabled={disabled}
        onValueChange={({ numericString }) => onValueChange(numericString)}
      />
    </div>
  );
}

function PriceLockMetadata({ item }: { item: BOQItem }) {
  return (
    <div className="mb-3 space-y-2">
      <PriceLockRow
        label="Khóa định mức"
        locked={Boolean(item.norm_price_locked)}
        actor={item.norm_price_locked_by}
        at={item.norm_price_locked_at}
        comment={item.norm_price_comment}
      />
      <PriceLockRow
        label="Chốt giá cuối"
        locked={Boolean(item.final_price_locked)}
        actor={item.final_price_locked_by}
        at={item.final_price_locked_at}
        comment={item.final_price_comment}
      />
    </div>
  );
}

function PriceLockRow({
  label,
  locked,
  actor,
  at,
  comment,
}: {
  label: string;
  locked: boolean;
  actor?: string;
  at?: string;
  comment?: string;
}) {
  return (
    <div className="rounded-md border border-line bg-white px-3 py-2">
      <div className="flex flex-wrap items-start justify-between gap-2">
        <p className="text-sm font-semibold text-ink">{label}</p>
        <span className={clsx('rounded-md px-2 py-1 text-xs font-semibold ring-1 ring-inset', locked ? 'bg-emerald-50 text-emerald-700 ring-emerald-200' : 'bg-slate-50 text-slate-500 ring-slate-200')}>
          {locked ? 'Đã khóa' : 'Chưa khóa'}
        </span>
      </div>
      <p className="mt-1 text-xs text-slate-500">{locked ? `${actor ?? 'Hệ thống'} · ${compactDateTime(at)}` : 'Chưa có người thực hiện hoặc thời điểm khóa.'}</p>
      {comment ? <p className="mt-1 text-xs leading-5 text-slate-500">{comment}</p> : null}
    </div>
  );
}

function PriceEventRow({ event }: { event: BOQUnitPriceEvent }) {
  return (
    <div className="rounded-md border border-line bg-white px-3 py-2">
      <div className="flex flex-wrap items-start justify-between gap-2">
        <p className="text-sm font-semibold text-ink">{priceEventLabel(event.event_type)}</p>
        <span className="text-xs font-semibold text-slate-500">lần {event.revision_no}</span>
      </div>
      <p className="mt-1 text-xs text-slate-500">
        {event.actor_name ?? 'Hệ thống'} · {compactDateTime(event.created_at)}
      </p>
      <p className="mt-1 text-sm text-slate-700">
        {event.from_unit_price !== null && event.from_unit_price !== undefined ? `${currency(event.from_unit_price)} → ` : ''}
        {currency(event.to_unit_price)}
      </p>
      {event.comment ? <p className="mt-1 text-xs leading-5 text-slate-500">{event.comment}</p> : null}
    </div>
  );
}

function priceEventLabel(eventType: string) {
  switch (eventType) {
    case 'norm_locked':
      return 'Đã khóa đơn giá định mức';
    case 'final_locked':
      return 'Đã chốt đơn giá cuối';
    case 'expected_updated':
    default:
      return 'Đã cập nhật đơn giá dự kiến';
  }
}

type BOQPriceReadiness = {
  expectedCount: number;
  normLockedCount: number;
  finalLockedCount: number;
  approvedCount: number;
  approvedFinalCount: number;
  approvedPendingFinal: BOQItem[];
  procuredCount: number;
  procurementComplete: boolean;
  generateDisabled: boolean;
  generateDisabledReason: string;
  nextAction: string;
};

function boqPriceReadiness(rows: BOQItem[]): BOQPriceReadiness {
  const expectedCount = rows.filter((item) => !item.norm_price_locked && !item.final_price_locked).length;
  const normLockedCount = rows.filter((item) => item.norm_price_locked && !item.final_price_locked).length;
  const finalLockedCount = rows.filter((item) => item.final_price_locked).length;
  const approved = rows.filter((item) => item.status === 'approved');
  const procuredCount = rows.filter(hasActiveBOQProcurement).length;
  const procurementComplete = rows.length > 0 && rows.every((item) => item.status === 'approved' && item.final_price_locked && hasActiveBOQProcurement(item));
  const eligibleApproved = approved.filter((item) => !hasActiveBOQProcurement(item));
  const approvedFinal = eligibleApproved.filter((item) => item.final_price_locked);
  const approvedPendingFinal = eligibleApproved.filter((item) => !item.final_price_locked || (!item.selected_quotation_item_id && !item.price_waiver_reason));
  const generateDisabledReason = approvedPendingFinal.length
    ? `${approvedPendingFinal.length} dòng đã duyệt chưa khóa giá cuối hoặc chưa có báo giá/lý do miễn báo giá.`
    : procurementComplete
      ? `${procuredCount} dòng dự toán đã liên kết với yêu cầu mua sắm hoạt động.`
      : approvedFinal.length === 0
        ? 'Chưa có dòng đã duyệt và chốt giá cuối để sinh mua sắm.'
        : '';

  return {
    expectedCount,
    normLockedCount,
    finalLockedCount,
    approvedCount: approved.length,
    approvedFinalCount: approvedFinal.length,
    approvedPendingFinal,
    procuredCount,
    procurementComplete,
    generateDisabled: Boolean(generateDisabledReason),
    generateDisabledReason,
    nextAction: boqNextAction(rows, approvedPendingFinal, approvedFinal.length, procurementComplete),
  };
}

function hasActiveBOQProcurement(item: BOQItem) {
  return Boolean(item.has_active_procurement || item.procurement_request_id || Number(item.active_procurement_count ?? 0) > 0);
}

function boqNextAction(rows: BOQItem[], approvedPendingFinal: BOQItem[], approvedFinalCount: number, procurementComplete: boolean) {
  if (!rows.length) return 'Thêm dòng BOQ';
  if (procurementComplete) return 'Đã chuyển sang mua sắm';
  if (approvedPendingFinal.length) return 'Hoàn tất chọn báo giá hoặc ghi lý do miễn báo giá';
  if (approvedFinalCount > 0) return 'Sinh yêu cầu mua sắm';
  if (rows.some((item) => item.status === 'draft' || item.status === 'revised')) return 'Gửi dự toán để duyệt';
  if (rows.some((item) => item.status === 'submitted' && !item.norm_price_locked)) return 'Khóa đơn giá định mức cho các dòng đã gửi';
  if (rows.some((item) => item.status === 'submitted')) return 'Duyệt dự toán';
  if (rows.some((item) => !item.norm_price_locked)) return 'Khóa đơn giá định mức';
  if (rows.some((item) => !item.final_price_locked)) return 'Chốt đơn giá cuối';
  return 'Kiểm tra vòng đời đơn giá';
}

function unitPriceNextAction(item: BOQItem) {
  if (item.final_price_locked) return 'Đơn giá cuối đã được chốt';
  if (item.status === 'draft' || item.status === 'revised') return 'Gửi dự toán để duyệt';
  if (item.status === 'submitted' && !item.norm_price_locked) return 'Khóa đơn giá định mức';
  if (item.status === 'submitted' && item.norm_price_locked) return 'Duyệt dự toán';
  if (item.status === 'approved' && item.norm_price_locked) return 'Chốt đơn giá cuối';
  return 'Kiểm tra trạng thái dòng dự toán';
}

function unitPriceProcurementImpact(item: BOQItem) {
  if (item.has_active_procurement || Number(item.active_procurement_count ?? 0) > 0) return 'Dòng này đã được liên kết với yêu cầu mua sắm đang hoạt động.';
  if (item.final_price_locked) return 'Dòng đã duyệt này sẵn sàng dùng giá cuối để sinh yêu cầu mua sắm.';
  if (item.status === 'draft' || item.status === 'revised') return 'Cần hoàn tất nội dung và gửi dự toán để duyệt trước khi chốt giá cuối.';
  if (item.status === 'submitted' && !item.norm_price_locked) return 'Cần khóa đơn giá định mức trước khi duyệt dự toán.';
  if (item.status === 'submitted') return 'Đơn giá định mức đã khóa; cần duyệt dự toán trước khi chốt giá cuối.';
  if (item.status === 'approved' && item.norm_price_locked) return 'Dòng đã duyệt đang chặn sinh mua sắm cho đến khi giá cuối được chốt.';
  return 'Trạng thái đơn giá chưa đáp ứng điều kiện sinh yêu cầu mua sắm.';
}

function priceInputValue(value: unknown) {
  const numeric = Number(value ?? 0);
  if (!Number.isFinite(numeric)) return '0';
  return String(numeric);
}

function compactDateTime(value?: string) {
  if (!value) return '-';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return new Intl.DateTimeFormat('vi-VN', {
    day: '2-digit',
    month: '2-digit',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  }).format(date);
}
