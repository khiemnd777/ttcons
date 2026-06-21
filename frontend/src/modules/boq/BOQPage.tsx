import { useEffect, useMemo, useState } from 'react';
import { useParams } from 'react-router-dom';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { CheckCircle2, DollarSign, History, Loader2, Lock, Plus, Send, ShoppingCart, ThumbsUp } from 'lucide-react';
import clsx from 'clsx';
import { get, post, put } from '../../lib/api/client';
import type { BOQItem, BOQUnitPriceEvent } from '../../lib/types/domain';
import { PageHeader } from '../../components/PageHeader';
import { DataTable } from '../../components/DataTable';
import { StatusBadge } from '../../components/StatusBadge';
import { FormDrawer } from '../../components/FormDrawer';
import { CurrencyText } from '../../components/CurrencyText';
import { useToast } from '../../components/ToastProvider';
import { RecordDetailDialog } from '../../components/RecordDetailDialog';
import { RowActions } from '../../components/RowActions';
import { ProjectBackLink } from '../../components/ProjectBackLink';
import { Dialog } from '../../components/Dialog';
import { SimpleForm } from '../_shared/SimpleForm';
import { currency } from '../../lib/utils/format';
import { enumOptions } from '../../lib/utils/status';

type PriceAction = 'expected' | 'norm' | 'final';

type PriceFormState = {
  expected: string;
  norm: string;
  final: string;
  comment: string;
};

export function BOQPage() {
  const { id = '' } = useParams();
  const [open, setOpen] = useState(false);
  const [dialog, setDialog] = useState<{ mode: 'view' | 'edit'; record: BOQItem } | null>(null);
  const [priceDialog, setPriceDialog] = useState<BOQItem | null>(null);
  const [priceForm, setPriceForm] = useState<PriceFormState>({ expected: '', norm: '', final: '', comment: '' });
  const queryClient = useQueryClient();
  const toast = useToast();
  const boq = useQuery({ queryKey: ['boq', id], queryFn: () => get<BOQItem[]>(`/projects/${id}/boq`), enabled: Boolean(id) });
  const materials = useQuery({ queryKey: ['materials'], queryFn: () => get<Record<string, any>[]>('/materials') });

  const invalidate = () => queryClient.invalidateQueries({ queryKey: ['boq', id] });
  const rows = boq.data ?? [];
  const total = useMemo(() => rows.reduce((sum, item) => sum + Number(item.effective_total_amount ?? item.total_amount ?? 0), 0), [rows]);
  const finalLockedTotal = useMemo(() => rows.filter((item) => item.final_price_locked).reduce((sum, item) => sum + Number(item.effective_total_amount ?? item.total_amount ?? 0), 0), [rows]);
  const pendingFinalTotal = Math.max(total - finalLockedTotal, 0);

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
      toast.success('Đã cập nhật dòng BOQ');
    },
    onError: (error) => toast.error('Không cập nhật được dòng BOQ', error),
  });

  const submit = useMutation({
    mutationFn: () => post(`/projects/${id}/boq/submit`, {}),
    onSuccess: () => {
      invalidate();
      toast.success('Đã submit BOQ');
    },
    onError: (error) => toast.error('Không submit được BOQ', error),
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
    mutationFn: () => post(`/projects/${id}/boq/generate-procurement`, {}),
    onSuccess: () => {
      invalidate();
      toast.success('Đã sinh yêu cầu mua sắm');
    },
    onError: (error) => toast.error('Không sinh được yêu cầu mua sắm', error),
  });

  const updateExpected = useMutation({
    mutationFn: ({ itemId, unitPrice, comment }: { itemId: string; unitPrice: number; comment: string }) =>
      post<BOQItem>(`/boq-items/${itemId}/unit-price/expected`, { unit_price: unitPrice, comment }),
    onSuccess: (item) => {
      invalidate();
      setPriceDialog(item);
      toast.success('Đã cập nhật expected price');
    },
    onError: (error) => toast.error('Không cập nhật được expected price', error),
  });

  const lockNorm = useMutation({
    mutationFn: ({ itemId, unitPrice, comment }: { itemId: string; unitPrice: number; comment: string }) =>
      post<BOQItem>(`/boq-items/${itemId}/unit-price/lock-norm`, { unit_price: unitPrice, comment }),
    onSuccess: (item) => {
      invalidate();
      setPriceDialog(item);
      toast.success('Đã lock norm price');
    },
    onError: (error) => toast.error('Không lock được norm price', error),
  });

  const lockFinal = useMutation({
    mutationFn: ({ itemId, unitPrice, comment }: { itemId: string; unitPrice: number; comment: string }) =>
      post<BOQItem>(`/boq-items/${itemId}/unit-price/lock-final`, { unit_price: unitPrice, comment }),
    onSuccess: (item) => {
      invalidate();
      setPriceDialog(item);
      toast.success('Đã lock final price');
    },
    onError: (error) => toast.error('Không lock được final price', error),
  });

  const pendingPriceAction = updateExpected.isPending || lockNorm.isPending || lockFinal.isPending;

  const boqFields = (record?: BOQItem) => [
    { name: 'category', label: 'Hạng mục', defaultValue: record?.category ?? 'Điện', required: true },
    { name: 'work_item_name', label: 'Tên công việc', defaultValue: record?.work_item_name ?? 'Hạng mục mới', required: true },
    { name: 'description', label: 'Mô tả', type: 'textarea' as const, defaultValue: record?.description ?? '' },
    { name: 'unit', label: 'Đơn vị', defaultValue: record?.unit ?? 'm2', required: true },
    { name: 'quantity', label: 'Khối lượng', type: 'number' as const, defaultValue: record?.quantity ?? 1, required: true, min: 0 },
    {
      name: 'expected_unit_price',
      label: 'Expected unit price',
      type: 'number' as const,
      defaultValue: record?.expected_unit_price ?? record?.unit_price ?? 1000000,
      required: true,
      min: 0,
      helperText: 'Giá QS dự kiến. Sau khi lock norm/final, hãy dùng dialog Unit Price Lifecycle.',
    },
    { name: 'material_id', label: 'Vật tư liên kết', type: 'select' as const, defaultValue: record?.material_id ?? '', options: (materials.data ?? []).map((m) => ({ label: m.name, value: m.id })) },
    { name: 'status', label: 'Trạng thái', type: 'select' as const, defaultValue: record?.status ?? 'draft', required: true, options: enumOptions('status', ['draft', 'submitted', 'approved', 'revised']) },
  ];

  function submitPriceAction(action: PriceAction) {
    if (!priceDialog) return;
    const raw = action === 'expected' ? priceForm.expected : action === 'norm' ? priceForm.norm : priceForm.final;
    const unitPrice = Number(raw);
    if (!Number.isFinite(unitPrice) || unitPrice < 0 || (action !== 'expected' && unitPrice <= 0)) {
      toast.warn({ title: 'Đơn giá không hợp lệ', description: action === 'expected' ? 'Expected price phải lớn hơn hoặc bằng 0.' : 'Giá lock phải lớn hơn 0.' });
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
        title="BOQ / Dự toán"
        description="Bóc tách khối lượng, kiểm soát expected/norm/final unit price, submit/approve và sinh nhu cầu mua vật tư từ BOQ."
        actions={
          <>
            <ProjectBackLink projectId={id} />
            <button className="inline-flex items-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold hover:bg-slate-100 disabled:cursor-not-allowed disabled:opacity-60" disabled={submit.isPending} onClick={() => submit.mutate()}>
              {submit.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <Send className="h-4 w-4" />}
              Submit
            </button>
            <button className="inline-flex items-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold hover:bg-slate-100 disabled:cursor-not-allowed disabled:opacity-60" disabled={approve.isPending} onClick={() => approve.mutate()}>
              {approve.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <ThumbsUp className="h-4 w-4" />}
              Approve
            </button>
            <button className="inline-flex items-center gap-2 rounded-md bg-emerald-600 px-3 py-2 text-sm font-semibold text-white hover:bg-emerald-700 disabled:cursor-not-allowed disabled:opacity-60" disabled={generate.isPending} onClick={() => generate.mutate()}>
              {generate.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <ShoppingCart className="h-4 w-4" />}
              Sinh mua sắm
            </button>
            <button className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700" onClick={() => setOpen(true)}>
              <Plus className="h-4 w-4" />
              Dòng BOQ
            </button>
          </>
        }
      />

      <div className="mb-4 grid gap-3 md:grid-cols-3">
        <MetricCard label="Tổng dự toán hiệu lực" value={total} tone="blue" />
        <MetricCard label="Đã final lock" value={finalLockedTotal} tone="emerald" />
        <MetricCard label="Chờ final lock" value={pendingFinalTotal} tone={pendingFinalTotal > 0 ? 'amber' : 'slate'} />
      </div>

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
          { key: 'price_lifecycle', header: 'Unit price lifecycle', sortable: true, sortValue: (row) => Number(row.effective_unit_price ?? row.unit_price ?? 0), render: (row) => <PriceLifecycleCell item={row} /> },
          { key: 'effective_total_amount', header: 'Thành tiền', align: 'right', sortable: true, sortValue: (row) => Number(row.effective_total_amount ?? row.total_amount ?? 0), render: (row) => <CurrencyText value={row.effective_total_amount ?? row.total_amount} /> },
          { key: 'status', header: 'Trạng thái', sortable: true, render: (row) => <StatusBadge status={row.status} /> },
          {
            key: 'action',
            header: 'Action',
            align: 'right',
            render: (row) => (
              <RowActions onView={() => setDialog({ mode: 'view', record: row })} onEdit={row.final_price_locked ? undefined : () => setDialog({ mode: 'edit', record: row })}>
                <button
                  title="Unit price lifecycle"
                  aria-label="Unit price lifecycle"
                  className="rounded-md border border-line p-1.5 text-blue-700 hover:bg-blue-50"
                  onClick={() => setPriceDialog(row)}
                >
                  <DollarSign className="h-4 w-4" />
                </button>
                {!row.final_price_locked ? (
                  <button className="text-sm font-semibold text-blue-700" onClick={() => update.mutate({ itemId: row.id, values: { status: 'revised' } })}>Revised</button>
                ) : null}
              </RowActions>
            ),
          },
        ]}
      />

      <FormDrawer open={open} title="Thêm dòng BOQ" onClose={() => setOpen(false)}>
        <SimpleForm
          submitLabel="Thêm BOQ"
          fields={boqFields()}
          onSubmit={(values) => create.mutateAsync({ ...values, material_id: values.material_id || null })}
        />
      </FormDrawer>

      <RecordDetailDialog
        open={dialog?.mode === 'view'}
        title="Chi tiết dòng BOQ"
        record={dialog?.record ?? null}
        onClose={() => setDialog(null)}
        fields={[
          { key: 'category', label: 'Hạng mục' },
          { key: 'work_item_name', label: 'Công việc' },
          { key: 'description', label: 'Mô tả' },
          { key: 'quantity', label: 'Khối lượng' },
          { key: 'unit', label: 'ĐVT' },
          { key: 'expected_unit_price', label: 'Expected price', render: (row) => <CurrencyText value={row.expected_unit_price} /> },
          { key: 'norm_unit_price', label: 'Norm price', render: (row) => row.norm_unit_price ? <CurrencyText value={row.norm_unit_price} /> : '-' },
          { key: 'final_unit_price', label: 'Final price', render: (row) => row.final_unit_price ? <CurrencyText value={row.final_unit_price} /> : '-' },
          { key: 'effective_total_amount', label: 'Thành tiền hiệu lực', render: (row) => <CurrencyText value={row.effective_total_amount ?? row.total_amount} /> },
          { key: 'price_lifecycle_status', label: 'Lifecycle', render: (row) => <PriceStatusChip item={row as BOQItem} /> },
          { key: 'status', label: 'Trạng thái', render: (row) => <StatusBadge status={row.status} /> },
        ]}
      />

      <FormDrawer open={dialog?.mode === 'edit'} title="Chỉnh sửa dòng BOQ" onClose={() => setDialog(null)}>
        {dialog?.record ? (
          <SimpleForm
            submitLabel="Lưu thay đổi"
            fields={boqFields(dialog.record)}
            onSubmit={(values) => update.mutateAsync({ itemId: dialog.record.id, values: { ...values, material_id: values.material_id || null } })}
          />
        ) : null}
      </FormDrawer>

      <UnitPriceLifecycleDialog
        item={priceDialog}
        form={priceForm}
        pending={pendingPriceAction}
        onChange={setPriceForm}
        onClose={() => setPriceDialog(null)}
        onSubmit={submitPriceAction}
      />
    </>
  );
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

function PriceLifecycleCell({ item }: { item: BOQItem }) {
  return (
    <div className="min-w-[220px] space-y-1">
      <div className="flex items-center justify-between gap-3 text-xs">
        <span className="font-semibold text-slate-500">Expected</span>
        <span className="font-semibold text-ink">{currency(item.expected_unit_price ?? item.unit_price)}</span>
      </div>
      <div className="flex items-center justify-between gap-3 text-xs">
        <span className="font-semibold text-slate-500">Norm</span>
        <span className={clsx('font-semibold', item.norm_price_locked ? 'text-emerald-700' : 'text-slate-400')}>{item.norm_price_locked ? currency(item.norm_unit_price) : 'Chưa lock'}</span>
      </div>
      <div className="flex items-center justify-between gap-3 text-xs">
        <span className="font-semibold text-slate-500">Final</span>
        <span className={clsx('font-semibold', item.final_price_locked ? 'text-blue-700' : 'text-slate-400')}>{item.final_price_locked ? currency(item.final_unit_price) : 'Chưa lock'}</span>
      </div>
      <PriceStatusChip item={item} />
    </div>
  );
}

function PriceStatusChip({ item }: { item: BOQItem }) {
  const status = item.final_price_locked ? 'Final locked' : item.norm_price_locked ? 'Norm locked' : 'Expected';
  const Icon = item.final_price_locked ? Lock : item.norm_price_locked ? CheckCircle2 : DollarSign;
  return (
    <span className={clsx(
      'inline-flex items-center gap-1 rounded-md px-2 py-1 text-xs font-semibold ring-1 ring-inset',
      item.final_price_locked && 'bg-blue-50 text-blue-700 ring-blue-200',
      item.norm_price_locked && !item.final_price_locked && 'bg-emerald-50 text-emerald-700 ring-emerald-200',
      !item.norm_price_locked && !item.final_price_locked && 'bg-slate-50 text-slate-600 ring-slate-200',
    )}>
      <Icon className="h-3.5 w-3.5" />
      {status} · rev {item.price_revision_no ?? 0}
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
}: {
  item: BOQItem | null;
  form: PriceFormState;
  pending: boolean;
  onChange: (form: PriceFormState) => void;
  onClose: () => void;
  onSubmit: (action: PriceAction) => void;
}) {
  if (!item) return null;
  const setField = (field: keyof PriceFormState, value: string) => onChange({ ...form, [field]: value });
  return (
    <Dialog
      open={Boolean(item)}
      title="Unit Price Lifecycle"
      description={`${item.work_item_name} · ${item.quantity} ${item.unit} · effective ${currency(item.effective_unit_price ?? item.unit_price)}`}
      onClose={onClose}
      size="xl"
      footer={
        <div className="flex justify-end">
          <button className="rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100" onClick={onClose}>Đóng</button>
        </div>
      }
    >
      <div className="grid gap-4 lg:grid-cols-[1.1fr_0.9fr]">
        <div className="space-y-3">
          <PriceStagePanel
            title="Expected price"
            description="Giá QS dự kiến, chỉ sửa trước khi lock norm/final."
            value={form.expected}
            disabled={pending || Boolean(item.norm_price_locked || item.final_price_locked)}
            buttonLabel="Cập nhật expected"
            onValueChange={(value) => setField('expected', value)}
            onSubmit={() => onSubmit('expected')}
          />
          <PriceStagePanel
            title="Norm price"
            description="Chốt định mức nội bộ. Sau khi lock norm, expected không còn được sửa."
            value={form.norm}
            disabled={pending || Boolean(item.norm_price_locked || item.final_price_locked)}
            buttonLabel={item.norm_price_locked ? 'Đã lock norm' : 'Lock norm'}
            onValueChange={(value) => setField('norm', value)}
            onSubmit={() => onSubmit('norm')}
          />
          <PriceStagePanel
            title="Final price"
            description="Giá cuối dùng cho procurement. Cần lock norm trước khi lock final."
            value={form.final}
            disabled={pending || Boolean(!item.norm_price_locked || item.final_price_locked)}
            buttonLabel={item.final_price_locked ? 'Đã lock final' : 'Lock final'}
            onValueChange={(value) => setField('final', value)}
            onSubmit={() => onSubmit('final')}
          />
          <label className="block">
            <span className="text-sm font-semibold text-slate-700">Comment</span>
            <textarea
              className="mt-1 w-full rounded-md border border-line bg-white px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-100 disabled:bg-slate-100"
              rows={3}
              value={form.comment}
              disabled={pending}
              onChange={(event) => setField('comment', event.target.value)}
              placeholder="Ghi chú lý do cập nhật hoặc lock giá"
            />
          </label>
        </div>
        <div className="rounded-md border border-line bg-slate-50 p-3">
          <div className="mb-3 flex items-center justify-between gap-2">
            <h3 className="text-sm font-semibold text-ink">History</h3>
            <History className="h-4 w-4 text-slate-400" />
          </div>
          {item.price_events?.length ? (
            <div className="space-y-2">
              {item.price_events.map((event) => <PriceEventRow key={event.id} event={event} />)}
            </div>
          ) : (
            <div className="rounded-md border border-dashed border-line bg-white px-3 py-4 text-sm font-medium text-slate-500">Chưa có price event.</div>
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
      <input
        type="number"
        min={0}
        className="mt-3 w-full rounded-md border border-line bg-white px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-100 disabled:bg-slate-100"
        value={value}
        disabled={disabled}
        onChange={(event) => onValueChange(event.target.value)}
      />
    </div>
  );
}

function PriceEventRow({ event }: { event: BOQUnitPriceEvent }) {
  return (
    <div className="rounded-md border border-line bg-white px-3 py-2">
      <div className="flex flex-wrap items-start justify-between gap-2">
        <p className="text-sm font-semibold text-ink">{priceEventLabel(event.event_type)}</p>
        <span className="text-xs font-semibold text-slate-500">rev {event.revision_no}</span>
      </div>
      <p className="mt-1 text-xs text-slate-500">
        {event.actor_name ?? 'System'} · {compactDateTime(event.created_at)}
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
      return 'Locked norm price';
    case 'final_locked':
      return 'Locked final price';
    case 'expected_updated':
    default:
      return 'Updated expected price';
  }
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
