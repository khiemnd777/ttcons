import { type FormEvent, type ReactNode, useRef, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Check, ExternalLink, Loader2, Plus, RefreshCw, Upload, X } from 'lucide-react';
import { CurrencyText } from '../../components/CurrencyText';
import { CurrencyInput } from '../../components/CurrencyInput';
import { DataTable } from '../../components/DataTable';
import { Dialog } from '../../components/Dialog';
import { EmptyState } from '../../components/EmptyState';
import { LoadingSpinner } from '../../components/LoadingSpinner';
import { StatusBadge } from '../../components/StatusBadge';
import { useToast } from '../../components/ToastProvider';
import { downloadBlob, get, post, postForm } from '../../lib/api/client';
import { useAuth } from '../../lib/auth/AuthContext';
import type { OutputInvoiceRecord, Partner, ReceivableRecord } from '../../lib/types/domain';
import { dateText } from '../../lib/utils/format';
import { partnerOptions } from '../core-data/partnerUtils';

const maxInvoiceUploadBytes = 10 * 1024 * 1024;

type OutputInvoicePanelProps = {
  receivable: ReceivableRecord;
  onChanged?: () => void;
};

type InvoiceDraft = {
  invoice_number: string;
  invoice_symbol: string;
  invoice_date: string;
  client_partner_id: string;
  client_name: string;
  client_tax_code: string;
  amount_before_vat: string;
  vat_amount: string;
  total_amount: string;
  payment_id: string;
  notes: string;
};

export function OutputInvoicePanel({ receivable, onChanged }: OutputInvoicePanelProps) {
  const [dialogOpen, setDialogOpen] = useState(false);
  const [selectedFile, setSelectedFile] = useState<File | null>(null);
  const [draft, setDraft] = useState<InvoiceDraft>(() => defaultDraft(receivable));
  const fileInputRef = useRef<HTMLInputElement | null>(null);
  const queryClient = useQueryClient();
  const toast = useToast();
  const { hasAnyPermission } = useAuth();
  const canManage = hasAnyPermission(['accounting.receivable_approve']);
  const invoices = useQuery({
    queryKey: ['output-invoices', receivable.id],
    queryFn: () => get<OutputInvoiceRecord[]>(`/receivables/${receivable.id}/output-invoices`),
  });
  const partners = useQuery({ queryKey: ['partners', 'client', 'active'], queryFn: () => get<Partner[]>('/partners?status=active&type=client') });

  const uploadInvoice = useMutation({
    mutationFn: (formData: FormData) => postForm<OutputInvoiceRecord>(`/receivables/${receivable.id}/output-invoices/upload`, formData),
    onSuccess: () => {
      setDialogOpen(false);
      setSelectedFile(null);
      setDraft(defaultDraft(receivable));
      if (fileInputRef.current) fileInputRef.current.value = '';
      invalidateInvoiceData(queryClient, receivable);
      onChanged?.();
      toast.success('Đã upload hoá đơn đầu ra');
    },
    onError: (error) => toast.error('Không upload được hoá đơn đầu ra', error),
  });

  const action = useMutation({
    mutationFn: ({ invoiceID, actionName }: { invoiceID: string; actionName: 'issue' | 'void' }) => post<OutputInvoiceRecord>(`/output-invoices/${invoiceID}/${actionName}`, {}),
    onSuccess: (_row, variables) => {
      invalidateInvoiceData(queryClient, receivable);
      onChanged?.();
      toast.success(variables.actionName === 'issue' ? 'Đã phát hành hoá đơn' : 'Đã huỷ hoá đơn');
    },
    onError: (error) => toast.error('Không cập nhật được hoá đơn đầu ra', error),
  });

  function updateDraft(key: keyof InvoiceDraft, value: string) {
    setDraft((current) => ({ ...current, [key]: value }));
  }

  function selectClientPartner(partnerId: string) {
    const partner = (partners.data ?? []).find((item) => item.id === partnerId);
    setDraft((current) => ({
      ...current,
      client_partner_id: partnerId,
      client_name: partner?.name ?? current.client_name,
      client_tax_code: partner?.tax_code ?? current.client_tax_code,
    }));
  }

  function onFileChange(file: File | null) {
    setSelectedFile(file);
    if (file && !draft.invoice_number.trim()) {
      const name = file.name.replace(/\.pdf$/i, '');
      updateDraft('invoice_number', name.slice(0, 120));
    }
  }

  function submitUpload(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const validation = validateDraft(draft, selectedFile);
    if (validation) {
      toast.warn({ title: 'Thiếu hoặc sai thông tin hoá đơn', description: validation });
      return;
    }
    const formData = new FormData();
    formData.append('file', selectedFile as File);
    formData.append('invoice_number', draft.invoice_number.trim());
    formData.append('invoice_symbol', draft.invoice_symbol.trim());
    formData.append('invoice_date', draft.invoice_date);
    formData.append('client_partner_id', draft.client_partner_id);
    formData.append('client_name', draft.client_name.trim());
    formData.append('client_tax_code', draft.client_tax_code.trim());
    formData.append('amount_before_vat', draft.amount_before_vat.trim() || '0');
    formData.append('vat_amount', draft.vat_amount.trim() || '0');
    formData.append('total_amount', draft.total_amount.trim());
    formData.append('payment_id', draft.payment_id.trim());
    formData.append('notes', draft.notes.trim());
    uploadInvoice.mutate(formData);
  }

  async function openInvoice(record: OutputInvoiceRecord) {
    const viewer = window.open('', '_blank', 'noopener,noreferrer');
    try {
      const blob = await downloadBlob(`/output-invoices/${record.id}/file`);
      const objectUrl = URL.createObjectURL(blob);
      if (viewer) viewer.location.href = objectUrl;
      else window.open(objectUrl, '_blank', 'noopener,noreferrer');
      window.setTimeout(() => URL.revokeObjectURL(objectUrl), 60_000);
    } catch (error) {
      viewer?.close();
      toast.error('Không mở được PDF hoá đơn', error);
    }
  }

  const rows = invoices.data ?? [];
  const invoiceCount = rows.length;
  const invoiceTotal = rows.reduce((sum, row) => sum + Number(row.total_amount ?? 0), 0);
  const issuedTotal = rows.filter((row) => row.status === 'issued').reduce((sum, row) => sum + Number(row.total_amount ?? 0), 0);

  return (
    <section className="space-y-3">
      <div className="grid gap-3 sm:grid-cols-3">
        <MetricTile label="PDF hoá đơn" value={`${invoiceCount}`} />
        <MetricTile label="Tổng upload" value={<CurrencyText value={invoiceTotal} />} />
        <MetricTile label="Đã phát hành" value={<CurrencyText value={issuedTotal} />} />
      </div>

      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div className="min-w-0">
          <h3 className="text-sm font-semibold text-ink">Hoá đơn đầu ra</h3>
          <p className="mt-1 text-xs text-slate-500">File được lưu protected storage và chỉ mở qua quyền kế toán dự án.</p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <button
            type="button"
            className="inline-flex items-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100"
            onClick={() => void invoices.refetch()}
            disabled={invoices.isFetching}
          >
            <RefreshCw className={invoices.isFetching ? 'h-4 w-4 animate-spin' : 'h-4 w-4'} />
            Tải lại
          </button>
          {canManage ? <button
            type="button"
            className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700"
            onClick={() => setDialogOpen(true)}
          >
            <Plus className="h-4 w-4" />
            Upload PDF
          </button> : null}
        </div>
      </div>

      {invoices.isLoading ? (
        <LoadingSpinner />
      ) : invoices.error ? (
        <div className="rounded-md border border-rose-100 bg-rose-50 p-3 text-sm text-rose-700">Không tải được danh sách hoá đơn đầu ra.</div>
      ) : rows.length ? (
        <DataTable
          rows={rows}
          empty="Chưa có hoá đơn đầu ra"
          searchPlaceholder="Tìm số hoá đơn, khách hàng, file..."
          pageSize={5}
          defaultSort={{ key: 'invoice_date', direction: 'desc' }}
          columns={[
            {
              key: 'invoice_number',
              header: 'Số hoá đơn',
              sortable: true,
              render: (row) => (
                <div className="min-w-40">
                  <p className="font-semibold text-ink">{row.invoice_number}</p>
                  <p className="mt-1 text-xs text-slate-500">{row.invoice_symbol || row.file_name || '-'}</p>
                </div>
              ),
            },
            { key: 'invoice_date', header: 'Ngày', sortable: true, render: (row) => dateText(row.invoice_date) },
            { key: 'client_name', header: 'Khách hàng', sortable: true },
            { key: 'total_amount', header: 'Tổng tiền', align: 'right', sortable: true, sortValue: (row) => Number(row.total_amount ?? 0), render: (row) => <CurrencyText value={row.total_amount} /> },
            { key: 'status', header: 'Trạng thái', sortable: true, render: (row) => <StatusBadge status={row.status} /> },
            {
              key: 'actions',
              header: 'Thao tác',
              render: (row) => (
                <div className="flex justify-end gap-1">
                  <IconButton label="Mở PDF" onClick={() => void openInvoice(row)}>
                    <ExternalLink className="h-4 w-4" />
                  </IconButton>
                  {canManage ? <IconButton label="Phát hành" disabled={row.status !== 'draft' || action.isPending} onClick={() => action.mutate({ invoiceID: row.id, actionName: 'issue' })}>
                    <Check className="h-4 w-4" />
                  </IconButton> : null}
                  {canManage ? <IconButton label="Huỷ" disabled={!['draft', 'issued'].includes(row.status) || action.isPending} onClick={() => action.mutate({ invoiceID: row.id, actionName: 'void' })}>
                    <X className="h-4 w-4" />
                  </IconButton> : null}
                </div>
              ),
            },
          ]}
        />
      ) : (
        <EmptyState title="Chưa có hoá đơn đầu ra" />
      )}

      <Dialog open={dialogOpen} title="Upload PDF hoá đơn đầu ra" onClose={() => setDialogOpen(false)} size="lg">
        <form className="space-y-4" onSubmit={submitUpload}>
          <label className="block">
            <span className="text-xs font-semibold text-slate-600">File PDF</span>
            <input
              ref={fileInputRef}
              type="file"
              accept="application/pdf"
              className="mt-1 block w-full rounded-md border border-line bg-white px-3 py-2 text-sm file:mr-3 file:rounded-md file:border-0 file:bg-blue-50 file:px-3 file:py-1.5 file:text-sm file:font-semibold file:text-blue-700 hover:file:bg-blue-100"
              onChange={(event) => onFileChange(event.target.files?.[0] ?? null)}
            />
            <span className="mt-1 block text-xs text-slate-500">{selectedFile ? `${selectedFile.name} · ${formatFileSize(selectedFile.size)}` : 'PDF <= 10MB'}</span>
          </label>

          <div className="grid gap-3 sm:grid-cols-2">
            <TextField label="Số hoá đơn" value={draft.invoice_number} onChange={(value) => updateDraft('invoice_number', value)} required />
            <TextField label="Ký hiệu" value={draft.invoice_symbol} onChange={(value) => updateDraft('invoice_symbol', value)} />
            <TextField label="Ngày hoá đơn" type="date" value={draft.invoice_date} onChange={(value) => updateDraft('invoice_date', value)} required />
            <SelectField label="Khách hàng catalog" value={draft.client_partner_id} onChange={selectClientPartner} options={partnerOptions(partners.data ?? [], ['client'])} />
            {!draft.client_partner_id ? <TextField label="Khách hàng" value={draft.client_name} onChange={(value) => updateDraft('client_name', value)} required /> : <ReadOnlyField label="Khách hàng snapshot" value={draft.client_name} />}
            {!draft.client_partner_id ? <TextField label="Mã số thuế khách hàng" value={draft.client_tax_code} onChange={(value) => updateDraft('client_tax_code', value)} /> : <ReadOnlyField label="MST khách hàng" value={draft.client_tax_code || '-'} />}
            <TextField label="Phiếu thu liên kết" value={draft.payment_id} onChange={(value) => updateDraft('payment_id', value)} placeholder="UUID client receipt nếu có" />
            <TextField label="Trước VAT" type="currency" value={draft.amount_before_vat} onChange={(value) => updateDraft('amount_before_vat', value)} min="0" />
            <TextField label="VAT" type="currency" value={draft.vat_amount} onChange={(value) => updateDraft('vat_amount', value)} min="0" />
            <TextField label="Tổng tiền" type="currency" value={draft.total_amount} onChange={(value) => updateDraft('total_amount', value)} min="0" required />
          </div>

          <label className="block">
            <span className="text-xs font-semibold text-slate-600">Ghi chú</span>
            <textarea
              className="mt-1 h-20 w-full rounded-md border border-line px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-500"
              value={draft.notes}
              onChange={(event) => updateDraft('notes', event.target.value)}
            />
          </label>

          <div className="flex justify-end gap-2 border-t border-line pt-4">
            <button type="button" className="rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100" onClick={() => setDialogOpen(false)}>
              Hủy
            </button>
            <button type="submit" className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60" disabled={uploadInvoice.isPending}>
              {uploadInvoice.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <Upload className="h-4 w-4" />}
              Upload
            </button>
          </div>
        </form>
      </Dialog>
    </section>
  );
}

function defaultDraft(receivable: ReceivableRecord): InvoiceDraft {
  const total = Number(receivable.remaining_amount ?? receivable.gross_amount ?? 0);
  return {
    invoice_number: '',
    invoice_symbol: '',
    invoice_date: new Date().toISOString().slice(0, 10),
    client_partner_id: receivable.client_partner_id ?? '',
    client_name: receivable.client_name ?? '',
    client_tax_code: '',
    amount_before_vat: total > 0 ? String(total) : '',
    vat_amount: '0',
    total_amount: total > 0 ? String(total) : '',
    payment_id: '',
    notes: '',
  };
}

function validateDraft(draft: InvoiceDraft, selectedFile: File | null) {
  if (!selectedFile) return 'Chọn file PDF hoá đơn.';
  if (selectedFile.size > maxInvoiceUploadBytes) return `Dung lượng tối đa ${formatFileSize(maxInvoiceUploadBytes)}.`;
  if (selectedFile.type && selectedFile.type !== 'application/pdf') return 'File upload phải là PDF.';
  if (!draft.invoice_number.trim()) return 'Nhập số hoá đơn.';
  if (!draft.invoice_date) return 'Chọn ngày hoá đơn.';
  if (!draft.client_partner_id && !draft.client_name.trim()) return 'Nhập tên khách hàng trên hoá đơn.';
  const beforeVAT = Number(draft.amount_before_vat || 0);
  const vat = Number(draft.vat_amount || 0);
  const total = Number(draft.total_amount || 0);
  if (!Number.isFinite(beforeVAT) || beforeVAT < 0) return 'Số tiền trước VAT không hợp lệ.';
  if (!Number.isFinite(vat) || vat < 0) return 'VAT không hợp lệ.';
  if (!Number.isFinite(total) || total <= 0) return 'Tổng tiền phải lớn hơn 0.';
  if (beforeVAT + vat > 0 && total + 1 < beforeVAT + vat) return 'Tổng tiền nhỏ hơn trước VAT cộng VAT.';
  return '';
}

function invalidateInvoiceData(queryClient: ReturnType<typeof useQueryClient>, receivable: ReceivableRecord) {
  queryClient.invalidateQueries({ queryKey: ['output-invoices', receivable.id] });
  queryClient.invalidateQueries({ queryKey: ['output-invoices'] });
  queryClient.invalidateQueries({ queryKey: ['receivables'] });
}

function MetricTile({ label, value }: { label: string; value: ReactNode }) {
  return (
    <div className="rounded-md border border-line bg-slate-50 px-3 py-2">
      <p className="text-xs font-semibold uppercase text-slate-500">{label}</p>
      <div className="mt-1 text-base font-semibold text-ink">{value}</div>
    </div>
  );
}

function IconButton({ label, disabled, onClick, children }: { label: string; disabled?: boolean; onClick: () => void; children: ReactNode }) {
  return (
    <button
      type="button"
      className="rounded-md border border-line p-2 text-slate-600 hover:border-blue-300 hover:bg-blue-50 hover:text-blue-700 disabled:cursor-not-allowed disabled:opacity-40"
      title={label}
      aria-label={label}
      disabled={disabled}
      onClick={onClick}
    >
      {children}
    </button>
  );
}

function SelectField({
  label,
  value,
  onChange,
  options,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  options: Array<{ label: string; value: string }>;
}) {
  return (
    <label className="block">
      <span className="text-xs font-semibold text-slate-600">{label}</span>
      <select
        className="mt-1 w-full rounded-md border border-line px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-500"
        value={value}
        onChange={(event) => onChange(event.target.value)}
      >
        <option value="">Nhập tay</option>
        {options.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
      </select>
    </label>
  );
}

function ReadOnlyField({ label, value }: { label: string; value: string }) {
  return (
    <label className="block">
      <span className="text-xs font-semibold text-slate-600">{label}</span>
      <span className="mt-1 block min-h-10 rounded-md border border-line bg-slate-50 px-3 py-2 text-sm text-slate-700">{value || '-'}</span>
    </label>
  );
}

function TextField({
  label,
  value,
  onChange,
  type = 'text',
  required,
  min,
  placeholder,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  type?: string;
  required?: boolean;
  min?: string;
  placeholder?: string;
}) {
  return (
    <label className="block">
      <span className="text-xs font-semibold text-slate-600">{label}{required ? ' *' : ''}</span>
      {type === 'currency' ? <CurrencyInput
        className="mt-1"
        inputClassName="text-sm focus:ring-blue-500"
        value={value}
        min={min}
        placeholder={placeholder}
        onValueChange={({ numericString }) => onChange(numericString)}
      /> : <input
        className="mt-1 w-full rounded-md border border-line px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-500"
        type={type}
        value={value}
        min={min}
        placeholder={placeholder}
        onChange={(event) => onChange(event.target.value)}
      />}
    </label>
  );
}

function formatFileSize(value: number) {
  if (!Number.isFinite(value) || value <= 0) return '-';
  if (value < 1024) return `${value} B`;
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB`;
  return `${(value / 1024 / 1024).toFixed(1)} MB`;
}
