import { useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { CheckCircle2, ClipboardCheck, FilePlus2, Loader2, Pencil, Send } from 'lucide-react';
import { Dialog } from '../../components/Dialog';
import { StatusBadge } from '../../components/StatusBadge';
import { CurrencyText } from '../../components/CurrencyText';
import { useToast } from '../../components/ToastProvider';
import { get, post, put } from '../../lib/api/client';
import type { BOQItem, Partner, SupplierQuoteComparison, SupplierQuotation } from '../../lib/types/domain';
import { partnerOptions } from '../core-data/partnerUtils';

type QuotationItemDraft = {
  enabled: boolean;
  boq_item_id: string;
  description: string;
  unit: string;
  quantity: string;
  unit_price: string;
  discount_amount: string;
  tax_rate: string;
  lead_time_days: string;
  compliance_status: string;
  deviation_note: string;
};

type QuotationDraft = {
  id?: string;
  quotation_code: string;
  supplier_partner_id: string;
  quotation_date: string;
  valid_until: string;
  currency: string;
  payment_terms_days: string;
  lead_time_days: string;
  delivery_terms: string;
  warranty_terms: string;
  notes: string;
  items: QuotationItemDraft[];
};

type QualificationState = {
  quotation: SupplierQuotation;
  technical_status: string;
  commercial_status: string;
  comment: string;
};

type SelectionState = {
  row: SupplierQuoteComparison;
  quote: SupplierQuoteComparison['quotations'][number];
  reason: string;
};

const today = () => new Date().toISOString().slice(0, 10);

export function SupplierQuotationWorkspace({ projectId, boqItems }: { projectId: string; boqItems: BOQItem[] }) {
  const queryClient = useQueryClient();
  const toast = useToast();
  const [editor, setEditor] = useState<QuotationDraft | null>(null);
  const [qualification, setQualification] = useState<QualificationState | null>(null);
  const [selection, setSelection] = useState<SelectionState | null>(null);

  const quotations = useQuery({
    queryKey: ['supplier-quotations', projectId],
    queryFn: () => get<SupplierQuotation[]>(`/projects/${projectId}/supplier-quotations`),
    enabled: Boolean(projectId),
  });
  const comparison = useQuery({
    queryKey: ['supplier-quote-comparison', projectId],
    queryFn: () => get<SupplierQuoteComparison[]>(`/projects/${projectId}/supplier-quote-comparison`),
    enabled: Boolean(projectId),
  });
  const partners = useQuery({
    queryKey: ['partners', 'supplier', 'active'],
    queryFn: () => get<Partner[]>('/partners?status=active'),
  });
  const supplierOptions = partnerOptions(partners.data ?? [], ['supplier', 'service_provider', 'other']);

  const invalidate = () => Promise.all([
    queryClient.invalidateQueries({ queryKey: ['supplier-quotations', projectId] }),
    queryClient.invalidateQueries({ queryKey: ['supplier-quote-comparison', projectId] }),
    queryClient.invalidateQueries({ queryKey: ['boq', projectId] }),
  ]);

  const save = useMutation({
    mutationFn: (draft: QuotationDraft) => {
      const payload = quotationPayload(draft);
      return draft.id
        ? put<SupplierQuotation>(`/supplier-quotations/${draft.id}`, payload)
        : post<SupplierQuotation>(`/projects/${projectId}/supplier-quotations`, payload);
    },
    onSuccess: async () => {
      await invalidate();
      setEditor(null);
      toast.success('Đã lưu báo giá nhà cung cấp');
    },
    onError: (error) => toast.error('Không lưu được báo giá', error),
  });
  const submit = useMutation({
    mutationFn: (quotationId: string) => post(`/supplier-quotations/${quotationId}/submit`, {}),
    onSuccess: async () => {
      await invalidate();
      toast.success('Đã gửi báo giá để đánh giá');
    },
    onError: (error) => toast.error('Không gửi được báo giá', error),
  });
  const qualify = useMutation({
    mutationFn: (value: NonNullable<typeof qualification>) => post(`/supplier-quotations/${value.quotation.id}/qualify`, {
      technical_status: value.technical_status,
      commercial_status: value.commercial_status,
      comment: value.comment,
    }),
    onSuccess: async () => {
      await invalidate();
      setQualification(null);
      toast.success('Đã hoàn tất đánh giá báo giá');
    },
    onError: (error) => toast.error('Không đánh giá được báo giá', error),
  });
  const selectQuote = useMutation({
    mutationFn: (value: NonNullable<typeof selection>) => post(`/supplier-quotation-items/${value.quote.quotation_item_id}/select`, { decision_reason: value.reason }),
    onSuccess: async () => {
      await invalidate();
      setSelection(null);
      toast.success('Đã chọn báo giá và khóa giá cuối BOQ');
    },
    onError: (error) => toast.error('Không chọn được báo giá', error),
  });

  const counts = useMemo(() => ({
    draft: (quotations.data ?? []).filter((row) => row.status === 'draft').length,
    pending: (quotations.data ?? []).filter((row) => row.status === 'submitted').length,
    qualified: (quotations.data ?? []).filter((row) => ['qualified', 'selected'].includes(row.status)).length,
  }), [quotations.data]);

  async function openEdit(quotationId: string) {
    try {
      const quotation = await get<SupplierQuotation>(`/supplier-quotations/${quotationId}`);
      setEditor(quotationDraftFromRecord(quotation, boqItems));
    } catch (error) {
      toast.error('Không tải được chi tiết báo giá', error);
    }
  }

  function saveEditor() {
    if (!editor) return;
    const error = quotationDraftError(editor);
    if (error) {
      toast.warn({ title: 'Báo giá chưa hợp lệ', description: error });
      return;
    }
    save.mutate(editor);
  }

  return (
    <section className="mb-4 overflow-hidden rounded-lg border border-line bg-white">
      <div className="flex flex-col gap-3 border-b border-line px-4 py-3 md:flex-row md:items-center md:justify-between">
        <div>
          <h2 className="text-base font-semibold text-ink">Báo giá nhà cung cấp & so sánh thương mại</h2>
          <p className="mt-1 text-sm text-slate-600">Chỉ báo giá đạt kỹ thuật và thương mại mới được chọn để khóa giá cuối BOQ.</p>
        </div>
        <button className="inline-flex items-center justify-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700" onClick={() => setEditor(emptyQuotationDraft(boqItems))}>
          <FilePlus2 className="h-4 w-4" /> Tạo báo giá
        </button>
      </div>
      <div className="grid gap-3 border-b border-line bg-slate-50 px-4 py-3 sm:grid-cols-3">
        <Summary label="Báo giá nháp" value={counts.draft} />
        <Summary label="Chờ đánh giá" value={counts.pending} />
        <Summary label="Đạt / đã chọn" value={counts.qualified} />
      </div>

      <div className="p-4">
        <h3 className="mb-2 text-sm font-semibold text-ink">Hồ sơ báo giá</h3>
        {quotations.isLoading ? <LoadingText /> : quotations.error ? <ErrorText onRetry={() => quotations.refetch()} /> : !(quotations.data?.length) ? (
          <EmptyText text="Chưa có báo giá. Tạo báo giá đầu tiên để bắt đầu so sánh." />
        ) : (
          <div className="overflow-x-auto rounded-md border border-line">
            <table className="min-w-[900px] w-full text-sm">
              <thead className="bg-slate-50 text-left text-xs uppercase tracking-wide text-slate-500">
                <tr><th className="px-3 py-2">Mã</th><th className="px-3 py-2">Nhà cung cấp</th><th className="px-3 py-2">Hiệu lực</th><th className="px-3 py-2 text-right">Giá trị</th><th className="px-3 py-2">Đánh giá</th><th className="px-3 py-2">Trạng thái</th><th className="px-3 py-2 text-right">Thao tác</th></tr>
              </thead>
              <tbody className="divide-y divide-line">
                {quotations.data.map((quotation) => (
                  <tr key={quotation.id}>
                    <td className="px-3 py-2 font-semibold text-ink">{quotation.quotation_code}</td>
                    <td className="px-3 py-2">{quotation.supplier_name}<div className="text-xs text-slate-500">{quotation.item_count ?? 0} dòng</div></td>
                    <td className="px-3 py-2">{quotation.quotation_date}<div className="text-xs text-slate-500">đến {quotation.valid_until || 'không giới hạn'}</div></td>
                    <td className="px-3 py-2 text-right"><CurrencyText value={quotation.total_amount ?? 0} /></td>
                    <td className="px-3 py-2 text-xs">KT: {quotation.technical_status}<br />TM: {quotation.commercial_status}</td>
                    <td className="px-3 py-2"><StatusBadge status={quotation.status} /></td>
                    <td className="px-3 py-2"><div className="flex justify-end gap-2">
                      {quotation.status === 'draft' ? <button className="rounded-md border border-line p-2 hover:bg-slate-100" title="Sửa báo giá" onClick={() => void openEdit(quotation.id)}><Pencil className="h-4 w-4" /></button> : null}
                      {quotation.status === 'draft' ? <button className="inline-flex items-center gap-1 rounded-md border border-blue-200 px-2 py-1.5 font-semibold text-blue-700 hover:bg-blue-50" disabled={submit.isPending} onClick={() => submit.mutate(quotation.id)}><Send className="h-4 w-4" /> Gửi</button> : null}
                      {quotation.status === 'submitted' ? <button className="inline-flex items-center gap-1 rounded-md border border-emerald-200 px-2 py-1.5 font-semibold text-emerald-700 hover:bg-emerald-50" onClick={() => setQualification({ quotation, technical_status: 'compliant', commercial_status: 'compliant', comment: '' })}><ClipboardCheck className="h-4 w-4" /> Đánh giá</button> : null}
                    </div></td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}

        <h3 className="mb-2 mt-5 text-sm font-semibold text-ink">So sánh theo dòng BOQ</h3>
        {comparison.isLoading ? <LoadingText /> : comparison.error ? <ErrorText onRetry={() => comparison.refetch()} /> : !(comparison.data?.length) ? <EmptyText text="Chưa có dòng BOQ để so sánh." /> : (
          <div className="overflow-x-auto rounded-md border border-line">
            <table className="min-w-[980px] w-full text-sm">
              <thead className="bg-slate-50 text-left text-xs uppercase tracking-wide text-slate-500">
                <tr><th className="px-3 py-2">Công việc</th><th className="px-3 py-2 text-right">Giá định mức</th><th className="px-3 py-2">Các báo giá đủ điều kiện</th><th className="px-3 py-2">Quyết định</th></tr>
              </thead>
              <tbody className="divide-y divide-line">
                {comparison.data.map((row) => {
                  const qualifiedQuotes = row.quotations.filter((quote) => ['qualified', 'selected'].includes(quote.quotation_status) && ['compliant', 'conditional'].includes(quote.compliance_status));
                  return <tr key={row.boq_item_id}>
                    <td className="px-3 py-3"><div className="font-semibold text-ink">{row.work_item_name}</div><div className="text-xs text-slate-500">{row.quantity} {row.unit} · {row.category}</div></td>
                    <td className="px-3 py-3 text-right"><CurrencyText value={row.norm_unit_price ?? row.expected_unit_price} /></td>
                    <td className="px-3 py-3">
                      {qualifiedQuotes.length ? <div className="space-y-2">{qualifiedQuotes.map((quote) => <div key={quote.quotation_item_id} className="flex items-center justify-between gap-3 rounded-md border border-line px-2 py-1.5">
                        <div><span className="font-semibold">{quote.supplier_name}</span><span className="ml-2 text-xs text-slate-500">{quote.quotation_code} · {quote.lead_time_days} ngày</span></div>
                        <div className="flex items-center gap-3"><span className="font-semibold"><CurrencyText value={quote.unit_price} /></span>{!row.final_price_locked ? <button className="rounded-md bg-emerald-600 px-2 py-1 text-xs font-semibold text-white hover:bg-emerald-700" onClick={() => setSelection({ row, quote, reason: '' })}>Chọn</button> : null}</div>
                      </div>)}</div> : <span className="text-slate-500">Chưa có báo giá đạt.</span>}
                    </td>
                    <td className="px-3 py-3">{row.selected_quotation_item_id ? <span className="inline-flex items-center gap-1 font-semibold text-emerald-700"><CheckCircle2 className="h-4 w-4" /> Đã chọn & khóa giá</span> : row.price_waiver_reason ? <span className="text-amber-700">Miễn báo giá: {row.price_waiver_reason}</span> : <span className="text-slate-500">Chưa quyết định</span>}</td>
                  </tr>;
                })}
              </tbody>
            </table>
          </div>
        )}
      </div>

      <QuotationEditor draft={editor} supplierOptions={supplierOptions} pending={save.isPending} onChange={setEditor} onClose={() => setEditor(null)} onSave={saveEditor} />
      <QualificationDialog value={qualification} pending={qualify.isPending} onChange={setQualification} onClose={() => setQualification(null)} onSubmit={() => qualification && qualify.mutate(qualification)} />
      <SelectionDialog value={selection} pending={selectQuote.isPending} onChange={setSelection} onClose={() => setSelection(null)} onSubmit={() => {
        if (!selection?.reason.trim()) {
          toast.warn({ title: 'Thiếu lý do chọn', description: 'Nhập căn cứ quyết định trước khi khóa giá cuối BOQ.' });
          return;
        }
        selectQuote.mutate(selection);
      }} />
    </section>
  );
}

function QuotationEditor({ draft, supplierOptions, pending, onChange, onClose, onSave }: { draft: QuotationDraft | null; supplierOptions: Array<{ label: string; value: string }>; pending: boolean; onChange: (value: QuotationDraft | null) => void; onClose: () => void; onSave: () => void }) {
  if (!draft) return null;
  const set = <K extends keyof QuotationDraft>(key: K, value: QuotationDraft[K]) => onChange({ ...draft, [key]: value });
  const updateItem = (index: number, patch: Partial<QuotationItemDraft>) => set('items', draft.items.map((item, itemIndex) => itemIndex === index ? { ...item, ...patch } : item));
  return <Dialog open title={draft.id ? 'Chỉnh sửa báo giá' : 'Tạo báo giá nhà cung cấp'} description="Lưu nháp trước khi gửi đánh giá. Sau khi gửi, nội dung báo giá không thể sửa." onClose={onClose} size="workspace" footer={<div className="flex justify-end gap-2"><button className="rounded-md border border-line px-3 py-2 text-sm font-semibold" onClick={onClose}>Đóng</button><button className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white disabled:opacity-50" disabled={pending} onClick={onSave}>{pending ? <Loader2 className="h-4 w-4 animate-spin" /> : null} Lưu nháp</button></div>}>
    <div className="grid gap-3 md:grid-cols-4">
      <Field label="Mã báo giá" value={draft.quotation_code} onChange={(value) => set('quotation_code', value)} required />
      <SelectField label="Nhà cung cấp" value={draft.supplier_partner_id} onChange={(value) => set('supplier_partner_id', value)} options={supplierOptions} required />
      <Field label="Ngày báo giá" type="date" value={draft.quotation_date} onChange={(value) => set('quotation_date', value)} required />
      <Field label="Hiệu lực đến" type="date" value={draft.valid_until} onChange={(value) => set('valid_until', value)} />
      <Field label="Tiền tệ" value={draft.currency} onChange={(value) => set('currency', value.toUpperCase())} required />
      <Field label="Công nợ (ngày)" type="number" value={draft.payment_terms_days} onChange={(value) => set('payment_terms_days', value)} />
      <Field label="Lead time (ngày)" type="number" value={draft.lead_time_days} onChange={(value) => set('lead_time_days', value)} />
      <Field label="Điều kiện giao hàng" value={draft.delivery_terms} onChange={(value) => set('delivery_terms', value)} />
      <Field label="Bảo hành" value={draft.warranty_terms} onChange={(value) => set('warranty_terms', value)} />
      <label className="md:col-span-3"><span className="text-sm font-semibold text-slate-700">Ghi chú</span><textarea className="mt-1 w-full rounded-md border border-line px-3 py-2 text-sm" rows={2} value={draft.notes} onChange={(event) => set('notes', event.target.value)} /></label>
    </div>
    <div className="mt-5 overflow-x-auto rounded-md border border-line">
      <table className="min-w-[1100px] w-full text-sm"><thead className="bg-slate-50 text-left text-xs uppercase text-slate-500"><tr><th className="px-2 py-2">Chọn</th><th className="px-2 py-2">Dòng BOQ</th><th className="px-2 py-2">KL</th><th className="px-2 py-2">Đơn giá</th><th className="px-2 py-2">Chiết khấu</th><th className="px-2 py-2">VAT %</th><th className="px-2 py-2">Lead time</th><th className="px-2 py-2">Đáp ứng</th><th className="px-2 py-2">Sai khác</th></tr></thead>
        <tbody className="divide-y divide-line">{draft.items.map((item, index) => <tr key={item.boq_item_id} className={!item.enabled ? 'opacity-50' : ''}>
          <td className="px-2 py-2"><input type="checkbox" checked={item.enabled} onChange={(event) => updateItem(index, { enabled: event.target.checked })} /></td>
          <td className="px-2 py-2"><div className="font-semibold">{item.description}</div><div className="text-xs text-slate-500">{item.unit}</div></td>
          <td className="px-2 py-2"><MiniInput type="number" value={item.quantity} disabled={!item.enabled} onChange={(value) => updateItem(index, { quantity: value })} /></td>
          <td className="px-2 py-2"><MiniInput type="number" value={item.unit_price} disabled={!item.enabled} onChange={(value) => updateItem(index, { unit_price: value })} /></td>
          <td className="px-2 py-2"><MiniInput type="number" value={item.discount_amount} disabled={!item.enabled} onChange={(value) => updateItem(index, { discount_amount: value })} /></td>
          <td className="px-2 py-2"><MiniInput type="number" value={item.tax_rate} disabled={!item.enabled} onChange={(value) => updateItem(index, { tax_rate: value })} /></td>
          <td className="px-2 py-2"><MiniInput type="number" value={item.lead_time_days} disabled={!item.enabled} onChange={(value) => updateItem(index, { lead_time_days: value })} /></td>
          <td className="px-2 py-2"><select className="rounded-md border border-line px-2 py-1.5" value={item.compliance_status} disabled={!item.enabled} onChange={(event) => updateItem(index, { compliance_status: event.target.value })}><option value="pending">Chờ đánh giá</option><option value="compliant">Đạt</option><option value="conditional">Có điều kiện</option><option value="non_compliant">Không đạt</option></select></td>
          <td className="px-2 py-2"><MiniInput value={item.deviation_note} disabled={!item.enabled} onChange={(value) => updateItem(index, { deviation_note: value })} /></td>
        </tr>)}</tbody>
      </table>
    </div>
  </Dialog>;
}

function QualificationDialog({ value, pending, onChange, onClose, onSubmit }: { value: QualificationState | null; pending: boolean; onChange: (value: QualificationState | null) => void; onClose: () => void; onSubmit: () => void }) {
  if (!value) return null;
  return <Dialog open title={`Đánh giá ${value.quotation.quotation_code}`} description={value.quotation.supplier_name} onClose={onClose} footer={<div className="flex justify-end gap-2"><button className="rounded-md border border-line px-3 py-2 text-sm font-semibold" onClick={onClose}>Đóng</button><button className="rounded-md bg-emerald-600 px-3 py-2 text-sm font-semibold text-white disabled:opacity-50" disabled={pending || !value.comment.trim()} onClick={onSubmit}>Hoàn tất đánh giá</button></div>}>
    <div className="grid gap-3 sm:grid-cols-2"><SelectField label="Kỹ thuật" value={value.technical_status} onChange={(technical_status) => onChange({ ...value, technical_status })} options={[{ value: 'compliant', label: 'Đạt' }, { value: 'conditional', label: 'Đạt có điều kiện' }, { value: 'non_compliant', label: 'Không đạt' }]} required /><SelectField label="Thương mại" value={value.commercial_status} onChange={(commercial_status) => onChange({ ...value, commercial_status })} options={[{ value: 'compliant', label: 'Đạt' }, { value: 'non_compliant', label: 'Không đạt' }]} required /></div>
    <label className="mt-3 block"><span className="text-sm font-semibold text-slate-700">Nhận xét đánh giá *</span><textarea className="mt-1 w-full rounded-md border border-line px-3 py-2 text-sm" rows={4} value={value.comment} onChange={(event) => onChange({ ...value, comment: event.target.value })} /></label>
  </Dialog>;
}

function SelectionDialog({ value, pending, onChange, onClose, onSubmit }: { value: SelectionState | null; pending: boolean; onChange: (value: SelectionState | null) => void; onClose: () => void; onSubmit: () => void }) {
  if (!value) return null;
  return <Dialog open title="Xác nhận chọn báo giá" description={`${value.row.work_item_name} · ${value.quote.supplier_name}`} onClose={onClose} footer={<div className="flex justify-end gap-2"><button className="rounded-md border border-line px-3 py-2 text-sm font-semibold" onClick={onClose}>Đóng</button><button className="rounded-md bg-emerald-600 px-3 py-2 text-sm font-semibold text-white disabled:opacity-50" disabled={pending} onClick={onSubmit}>Chọn & khóa giá cuối</button></div>}>
    <div className="grid gap-3 rounded-md border border-line bg-slate-50 p-3 sm:grid-cols-3"><Summary label="Giá định mức" value={<CurrencyText value={value.row.norm_unit_price ?? value.row.expected_unit_price} />} /><Summary label="Giá được chọn" value={<CurrencyText value={value.quote.unit_price} />} /><Summary label="Lead time" value={`${value.quote.lead_time_days} ngày`} /></div>
    <label className="mt-3 block"><span className="text-sm font-semibold text-slate-700">Căn cứ quyết định *</span><textarea className="mt-1 w-full rounded-md border border-line px-3 py-2 text-sm" rows={4} value={value.reason} onChange={(event) => onChange({ ...value, reason: event.target.value })} placeholder="Nêu căn cứ giá, kỹ thuật, tiến độ, điều khoản thanh toán..." /></label>
  </Dialog>;
}

function emptyQuotationDraft(boqItems: BOQItem[]): QuotationDraft {
  return { quotation_code: `BG-${Date.now().toString().slice(-6)}`, supplier_partner_id: '', quotation_date: today(), valid_until: '', currency: 'VND', payment_terms_days: '0', lead_time_days: '0', delivery_terms: '', warranty_terms: '', notes: '', items: boqItems.filter((item) => !item.final_price_locked && item.status !== 'deleted').map((item) => ({ enabled: false, boq_item_id: item.id, description: item.work_item_name, unit: item.unit, quantity: String(item.quantity), unit_price: '', discount_amount: '0', tax_rate: '0', lead_time_days: '0', compliance_status: 'pending', deviation_note: '' })) };
}

function quotationDraftFromRecord(record: SupplierQuotation, boqItems: BOQItem[]): QuotationDraft {
  const existing = new Map((record.items ?? []).map((item) => [item.boq_item_id, item]));
  const draft = emptyQuotationDraft(boqItems);
  return { ...draft, id: record.id, quotation_code: record.quotation_code, supplier_partner_id: record.supplier_partner_id, quotation_date: record.quotation_date?.slice(0, 10), valid_until: record.valid_until?.slice(0, 10) ?? '', currency: record.currency, payment_terms_days: String(record.payment_terms_days ?? 0), lead_time_days: String(record.lead_time_days ?? 0), delivery_terms: record.delivery_terms ?? '', warranty_terms: record.warranty_terms ?? '', notes: record.notes ?? '', items: draft.items.map((item) => { const found = existing.get(item.boq_item_id); return found ? { ...item, enabled: true, description: found.description, unit: found.unit, quantity: String(found.quantity), unit_price: String(found.unit_price), discount_amount: String(found.discount_amount), tax_rate: String(found.tax_rate), lead_time_days: String(found.lead_time_days), compliance_status: found.compliance_status, deviation_note: found.deviation_note ?? '' } : item; }) };
}

function quotationPayload(draft: QuotationDraft) {
  return { ...draft, id: undefined, payment_terms_days: Number(draft.payment_terms_days || 0), lead_time_days: Number(draft.lead_time_days || 0), items: draft.items.filter((item) => item.enabled).map((item) => ({ ...item, enabled: undefined, quantity: Number(item.quantity), unit_price: Number(item.unit_price), discount_amount: Number(item.discount_amount || 0), tax_rate: Number(item.tax_rate || 0), lead_time_days: Number(item.lead_time_days || 0) })) };
}

function quotationDraftError(draft: QuotationDraft) {
  if (!draft.quotation_code.trim()) return 'Nhập mã báo giá.';
  if (!draft.supplier_partner_id) return 'Chọn nhà cung cấp từ catalog.';
  if (!draft.quotation_date) return 'Nhập ngày báo giá.';
  const items = draft.items.filter((item) => item.enabled);
  if (!items.length) return 'Chọn ít nhất một dòng BOQ.';
  if (items.some((item) => Number(item.quantity) <= 0 || Number(item.unit_price) <= 0)) return 'Khối lượng và đơn giá các dòng đã chọn phải lớn hơn 0.';
  return '';
}

function Summary({ label, value }: { label: string; value: React.ReactNode }) { return <div><div className="text-xs font-semibold uppercase tracking-wide text-slate-500">{label}</div><div className="mt-1 text-base font-semibold text-ink">{value}</div></div>; }
function LoadingText() { return <div className="flex items-center gap-2 rounded-md border border-dashed border-line p-4 text-sm text-slate-500"><Loader2 className="h-4 w-4 animate-spin" /> Đang tải dữ liệu...</div>; }
function EmptyText({ text }: { text: string }) { return <div className="rounded-md border border-dashed border-line p-4 text-sm text-slate-500">{text}</div>; }
function ErrorText({ onRetry }: { onRetry: () => void }) { return <div className="flex items-center justify-between rounded-md border border-red-200 bg-red-50 p-3 text-sm text-red-700"><span>Không tải được dữ liệu báo giá.</span><button className="font-semibold underline" onClick={onRetry}>Thử lại</button></div>; }
function Field({ label, value, onChange, type = 'text', required = false }: { label: string; value: string; onChange: (value: string) => void; type?: string; required?: boolean }) { return <label><span className="text-sm font-semibold text-slate-700">{label}{required ? ' *' : ''}</span><input className="mt-1 w-full rounded-md border border-line px-3 py-2 text-sm" type={type} value={value} onChange={(event) => onChange(event.target.value)} /></label>; }
function SelectField({ label, value, onChange, options, required = false }: { label: string; value: string; onChange: (value: string) => void; options: Array<{ label: string; value: string }>; required?: boolean }) { return <label><span className="text-sm font-semibold text-slate-700">{label}{required ? ' *' : ''}</span><select className="mt-1 w-full rounded-md border border-line bg-white px-3 py-2 text-sm" value={value} onChange={(event) => onChange(event.target.value)}><option value="">-- Chọn --</option>{options.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}</select></label>; }
function MiniInput({ value, onChange, type = 'text', disabled = false }: { value: string; onChange: (value: string) => void; type?: string; disabled?: boolean }) { return <input className="w-28 rounded-md border border-line px-2 py-1.5 text-sm disabled:bg-slate-100" type={type} value={value} disabled={disabled} onChange={(event) => onChange(event.target.value)} />; }
