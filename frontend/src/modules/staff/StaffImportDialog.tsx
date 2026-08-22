import { useRef, useState } from 'react';
import { useMutation } from '@tanstack/react-query';
import { AlertCircle, CheckCircle2, Download, FileSpreadsheet, RefreshCw, Upload } from 'lucide-react';
import { Dialog } from '../../components/Dialog';
import { post } from '../../lib/api/client';
import { downloadStaffImportTemplate, parseStaffImportFile, type StaffImportRow } from './staffImport';
import type { RoleRow, StaffImportPreview, StaffImportResult } from './staffImportTypes';

export function StaffImportDialog({ open, roles, rolesLoading, rolesError, onClose, onSuccess, onError }: {
  open: boolean;
  roles: RoleRow[];
  rolesLoading: boolean;
  rolesError: unknown;
  onClose: () => void;
  onSuccess: (result: StaffImportResult) => void;
  onError: (title: string, error: unknown) => void;
}) {
  const inputRef = useRef<HTMLInputElement>(null);
  const [file, setFile] = useState<File | null>(null);
  const [rows, setRows] = useState<StaffImportRow[]>([]);
  const [preview, setPreview] = useState<StaffImportPreview | null>(null);
  const [parseError, setParseError] = useState('');
  const [parsing, setParsing] = useState(false);
  const [confirmed, setConfirmed] = useState(false);
  const [idempotencyKey, setIdempotencyKey] = useState('');

  const previewMutation = useMutation({
    mutationFn: (payload: { source_filename: string; source_size_bytes: number; rows: StaffImportRow[] }) => post<StaffImportPreview>('/staff/imports/preview', payload),
    onSuccess: (result) => {
      setPreview(result);
      setConfirmed(false);
      setIdempotencyKey(crypto.randomUUID());
    },
    onError: (error) => onError('Không xem trước được danh sách import', error),
  });
  const executeMutation = useMutation({
    mutationFn: () => post<StaffImportResult>('/staff/imports/execute', {
      source_filename: file?.name,
      source_size_bytes: file?.size,
      rows,
      preview_hash: preview?.preview_hash,
      idempotency_key: idempotencyKey,
    }),
    onSuccess: (result) => {
      reset();
      onSuccess(result);
    },
    onError: (error) => onError('Không import được tài khoản nhân sự', error),
  });

  const busy = parsing || previewMutation.isPending || executeMutation.isPending;

  function reset() {
    setFile(null);
    setRows([]);
    setPreview(null);
    setParseError('');
    setConfirmed(false);
    setIdempotencyKey('');
    if (inputRef.current) inputRef.current.value = '';
  }

  function close() {
    if (busy) return;
    reset();
    onClose();
  }

  async function selectFile(selected?: File) {
    reset();
    if (!selected) return;
    setFile(selected);
    setParsing(true);
    try {
      setRows(await parseStaffImportFile(selected));
    } catch (error) {
      setParseError(error instanceof Error ? error.message : 'Không thể đọc tệp import.');
    } finally {
      setParsing(false);
    }
  }

  async function downloadTemplate(format: 'xlsx' | 'csv') {
    try {
      await downloadStaffImportTemplate(format, roles);
    } catch (error) {
      onError('Không tải được tệp mẫu import', error);
    }
  }

  const canExecute = Boolean(preview?.preview_hash) && preview?.invalid_count === 0 && confirmed && !busy;

  return (
    <Dialog
      open={open}
      title="Import tài khoản nhân sự"
      description="Tải Excel/CSV, kiểm tra toàn bộ danh sách rồi tạo nguyên tử nhân sự, tài khoản và một vai trò hệ thống."
      size="xl"
      onClose={close}
      footer={(
        <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
          <p className="text-xs text-slate-500">Tối đa 100 tài khoản · 1 MB · không lưu tệp nguồn</p>
          <div className="flex gap-2">
            <button type="button" className="rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-white disabled:opacity-50" onClick={close} disabled={busy}>Hủy</button>
            {!preview ? (
              <button type="button" className="rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:opacity-50" disabled={!file || !rows.length || busy} onClick={() => file && previewMutation.mutate({ source_filename: file.name, source_size_bytes: file.size, rows })}>
                {previewMutation.isPending ? 'Đang kiểm tra…' : `Xem trước ${rows.length || ''} tài khoản`}
              </button>
            ) : (
              <button type="button" className="rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:opacity-50" disabled={!canExecute} onClick={() => executeMutation.mutate()}>
                {executeMutation.isPending ? 'Đang import…' : `Import & kích hoạt ${preview.valid_count}`}
              </button>
            )}
          </div>
        </div>
      )}
    >
      <div className="space-y-5">
        <div className="rounded-md border border-amber-200 bg-amber-50 p-3 text-sm leading-6 text-amber-900">
          <strong>Tệp chứa mật khẩu nhạy cảm.</strong> Hệ thống chỉ đọc tệp trong trình duyệt và gửi các dòng đã cấu trúc qua kết nối HTTPS; không lưu tệp nguồn hay mật khẩu rõ trong audit.
        </div>

        <section className="rounded-lg border border-line p-4">
          <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
            <div>
              <h3 className="text-sm font-semibold text-ink">1. Chuẩn bị tệp</h3>
              <p className="mt-1 text-xs leading-5 text-slate-500">Giữ nguyên 8 cột mẫu. role_code lấy từ sheet roles hiện hành.</p>
            </div>
            <div className="flex flex-wrap gap-2">
              <button type="button" className="inline-flex items-center gap-2 rounded-md border border-line px-3 py-2 text-xs font-semibold text-slate-700 hover:bg-slate-50 disabled:opacity-50" disabled={rolesLoading || Boolean(rolesError)} onClick={() => void downloadTemplate('xlsx')}><Download className="h-4 w-4" />Mẫu Excel</button>
              <button type="button" className="inline-flex items-center gap-2 rounded-md border border-line px-3 py-2 text-xs font-semibold text-slate-700 hover:bg-slate-50" onClick={() => void downloadTemplate('csv')}><Download className="h-4 w-4" />Mẫu CSV</button>
            </div>
          </div>
          {rolesError ? <InlineMessage tone="warning" message="Chưa tải được danh sách vai trò hiện hành nên tạm thời chưa thể tạo mẫu Excel có sheet roles." /> : null}
          <input ref={inputRef} className="sr-only" type="file" accept=".xlsx,.csv,text/csv,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" onChange={(event) => void selectFile(event.target.files?.[0])} />
          <button type="button" className="mt-4 flex w-full items-center justify-center gap-3 rounded-lg border border-dashed border-blue-300 bg-blue-50/50 px-4 py-6 text-sm font-semibold text-blue-700 hover:bg-blue-50 disabled:opacity-50" disabled={busy} onClick={() => inputRef.current?.click()}>
            {parsing ? <RefreshCw className="h-5 w-5 animate-spin" /> : <Upload className="h-5 w-5" />}
            {parsing ? 'Đang đọc tệp…' : file ? 'Chọn tệp khác' : 'Chọn tệp Excel hoặc CSV'}
          </button>
          {file ? <p className="mt-2 flex items-center gap-2 text-xs text-slate-600"><FileSpreadsheet className="h-4 w-4" />{file.name} · {formatBytes(file.size)} · {rows.length} dòng dữ liệu</p> : null}
          {parseError ? <InlineMessage tone="error" message={parseError} /> : null}
        </section>

        {preview ? (
          <section className="space-y-4">
            <div>
              <h3 className="text-sm font-semibold text-ink">2. Kiểm tra và xác nhận</h3>
              <div className="mt-3 grid grid-cols-2 gap-2 sm:grid-cols-4">
                <Metric label="Tổng" value={preview.total_count} />
                <Metric label="Hợp lệ" value={preview.valid_count} tone="success" />
                <Metric label="Không hợp lệ" value={preview.invalid_count} tone={preview.invalid_count ? 'error' : 'default'} />
                <Metric label="Cảnh báo" value={preview.warning_count} tone={preview.warning_count ? 'warning' : 'default'} />
              </div>
            </div>
            <PreviewRows rows={preview.rows} />
            {preview.invalid_count ? (
              <InlineMessage tone="error" message="Cần sửa toàn bộ dòng lỗi trong tệp rồi chọn lại tệp và xem trước lại. Hệ thống chưa tạo tài khoản nào." />
            ) : (
              <label className="flex items-start gap-3 rounded-md border border-blue-200 bg-blue-50 p-3 text-sm text-blue-900">
                <input type="checkbox" className="mt-1 h-4 w-4 rounded border-line text-blue-600" checked={confirmed} onChange={(event) => setConfirmed(event.target.checked)} />
                <span>Tôi xác nhận tạo và kích hoạt {preview.valid_count} tài khoản, gán vai trò toàn hệ thống như bảng trên và yêu cầu người dùng đổi mật khẩu lần đầu.</span>
              </label>
            )}
          </section>
        ) : null}
      </div>
    </Dialog>
  );
}

function PreviewRows({ rows }: { rows: StaffImportPreview['rows'] }) {
  return (
    <>
      <div className="space-y-2 md:hidden">
        {rows.map((row) => (
          <article key={row.row_number} className={`rounded-lg border p-3 ${row.valid ? 'border-line bg-white' : 'border-rose-200 bg-rose-50/60'}`}>
            <div className="flex items-start justify-between gap-3"><p className="font-semibold text-ink">{row.full_name || '—'}</p><span className="font-mono text-xs text-slate-500">Dòng {row.row_number}</span></div>
            <p className="mt-1 break-all text-xs text-slate-500">{row.login_email || '—'}</p>
            <dl className="mt-3 grid grid-cols-2 gap-2 text-xs"><div><dt className="text-slate-500">Vai trò</dt><dd className="mt-1 font-semibold text-slate-700">{row.role_name || row.role_code || '—'}</dd></div><div><dt className="text-slate-500">Mật khẩu</dt><dd className={`mt-1 font-semibold ${row.password_valid ? 'text-emerald-700' : 'text-rose-700'}`}>{row.password_valid ? 'Hợp lệ' : 'Không hợp lệ'}</dd></div></dl>
            {row.errors.length ? <ul className="mt-3 space-y-1 text-xs text-rose-700">{row.errors.map((issue, index) => <li key={`${issue.field}-${index}`}>{issue.field}: {issue.message}</li>)}</ul> : <p className="mt-3 inline-flex items-center gap-1 text-xs font-semibold text-emerald-700"><CheckCircle2 className="h-4 w-4" />Sẵn sàng</p>}
            {row.warnings.map((issue, index) => <p key={`${issue.field}-${index}`} className="mt-1 text-xs text-amber-700">{issue.message}</p>)}
          </article>
        ))}
      </div>
      <div className="hidden overflow-hidden rounded-lg border border-line md:block">
        <div className="max-h-[42vh] overflow-auto">
        <table className="min-w-full divide-y divide-line text-sm">
          <thead className="sticky top-0 bg-slate-50 text-left text-xs uppercase tracking-wide text-slate-500">
            <tr><th className="px-3 py-2">Dòng</th><th className="px-3 py-2">Nhân sự / tài khoản</th><th className="px-3 py-2">Vai trò</th><th className="px-3 py-2">Mật khẩu</th><th className="px-3 py-2">Kết quả</th></tr>
          </thead>
          <tbody className="divide-y divide-line bg-white">
            {rows.map((row) => (
              <tr key={row.row_number} className={row.valid ? '' : 'bg-rose-50/60'}>
                <td className="px-3 py-3 align-top font-mono text-xs">{row.row_number}</td>
                <td className="px-3 py-3 align-top"><p className="font-semibold text-ink">{row.full_name || '—'}</p><p className="mt-1 break-all text-xs text-slate-500">{row.login_email || '—'}</p>{row.recovery_email_masked ? <p className="mt-1 text-xs text-slate-500">Khôi phục: {row.recovery_email_masked}</p> : null}</td>
                <td className="px-3 py-3 align-top"><p className="font-semibold text-slate-700">{row.role_name || '—'}</p><p className="mt-1 font-mono text-xs text-slate-500">{row.role_code || '—'}</p></td>
                <td className="px-3 py-3 align-top">{row.password_valid ? <span className="text-emerald-700">Hợp lệ</span> : <span className="text-rose-700">Không hợp lệ</span>}</td>
                <td className="px-3 py-3 align-top">
                  {row.valid ? <span className="inline-flex items-center gap-1 text-emerald-700"><CheckCircle2 className="h-4 w-4" />Sẵn sàng</span> : (
                    <ul className="space-y-1 text-xs text-rose-700">{row.errors.map((issue, index) => <li key={`${issue.field}-${index}`}>{issue.field}: {issue.message}</li>)}</ul>
                  )}
                  {row.warnings.map((issue, index) => <p key={`${issue.field}-${index}`} className="mt-1 text-xs text-amber-700">{issue.message}</p>)}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        </div>
      </div>
    </>
  );
}

function Metric({ label, value, tone = 'default' }: { label: string; value: number; tone?: 'default' | 'success' | 'warning' | 'error' }) {
  const colors = { default: 'border-line bg-slate-50 text-slate-700', success: 'border-emerald-200 bg-emerald-50 text-emerald-700', warning: 'border-amber-200 bg-amber-50 text-amber-700', error: 'border-rose-200 bg-rose-50 text-rose-700' };
  return <div className={`rounded-md border px-3 py-2 ${colors[tone]}`}><p className="text-xs font-medium">{label}</p><p className="mt-1 text-lg font-bold">{value}</p></div>;
}

function InlineMessage({ tone, message }: { tone: 'error' | 'warning'; message: string }) {
  return <div className={`mt-3 flex items-start gap-2 rounded-md border p-3 text-sm ${tone === 'error' ? 'border-rose-200 bg-rose-50 text-rose-700' : 'border-amber-200 bg-amber-50 text-amber-800'}`}><AlertCircle className="mt-0.5 h-4 w-4 shrink-0" /><span>{message}</span></div>;
}

function formatBytes(value: number) {
  return value < 1024 ? `${value} B` : `${Math.ceil(value / 1024)} KB`;
}
