import { useRef, useState } from 'react';
import { useMutation, useQuery } from '@tanstack/react-query';
import { AlertCircle, CheckCircle2, Download, FileJson, FileSpreadsheet, RefreshCw, Upload } from 'lucide-react';
import { Link } from 'react-router-dom';
import { Dialog } from '../../components/Dialog';
import { downloadBlob, get, post } from '../../lib/api/client';
import {
  parseProjectImportFile,
  saveProjectExcelBlob,
  type DataPackageFormat,
  type ProjectExcelImportRow,
} from './projectExcel';
import type {
  ProjectExcelImportPreview,
  ProjectExcelImportResult,
  ProjectExcelReadiness,
} from './projectExcelTypes';

export function ProjectImportDialog({ open, onClose, onSuccess, onError }: {
  open: boolean;
  onClose: () => void;
  onSuccess: (result: ProjectExcelImportResult) => void;
  onError: (title: string, error: unknown) => void;
}) {
  const inputRef = useRef<HTMLInputElement>(null);
  const [file, setFile] = useState<File | null>(null);
  const [rows, setRows] = useState<ProjectExcelImportRow[]>([]);
  const [packageVersion, setPackageVersion] = useState('');
  const [sourceFormat, setSourceFormat] = useState<DataPackageFormat>('xlsx');
  const [preview, setPreview] = useState<ProjectExcelImportPreview | null>(null);
  const [parseError, setParseError] = useState('');
  const [parsing, setParsing] = useState(false);
  const [confirmed, setConfirmed] = useState(false);
  const [idempotencyKey, setIdempotencyKey] = useState('');

  const readiness = useQuery({
    queryKey: ['project-excel-readiness'],
    queryFn: () => get<ProjectExcelReadiness>('/projects/imports/readiness'),
    enabled: open,
    staleTime: 0,
  });
  const templateMutation = useMutation({
    mutationFn: () => downloadBlob(`/projects/imports/template?format=${sourceFormat}`),
    onSuccess: (blob) => saveProjectExcelBlob(blob, `mau-import-cong-trinh.${sourceFormat}`),
    onError: (error) => onError('Không tải được file mẫu công trình', error),
  });
  const previewMutation = useMutation({
    mutationFn: () => post<ProjectExcelImportPreview>('/projects/imports/preview', {
      workbook_version: sourceFormat === 'xlsx' ? packageVersion : undefined,
      package_version: packageVersion,
      source_format: sourceFormat,
      source_filename: file?.name,
      source_size_bytes: file?.size,
      rows,
    }),
    onSuccess: (result) => {
      setPreview(result);
      setConfirmed(false);
      setIdempotencyKey(crypto.randomUUID());
    },
    onError: (error) => onError('Không xem trước được danh sách công trình', error),
  });
  const executeMutation = useMutation({
    mutationFn: () => post<ProjectExcelImportResult>('/projects/imports/execute', {
      workbook_version: sourceFormat === 'xlsx' ? packageVersion : undefined,
      package_version: packageVersion,
      source_format: sourceFormat,
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
    onError: (error) => onError('Không import được công trình', error),
  });

  const busy = parsing || templateMutation.isPending || previewMutation.isPending || executeMutation.isPending;
  const canPrepareFile = readiness.data?.ready === true;
  const canExecute = Boolean(preview?.preview_hash) && preview?.invalid_count === 0 && confirmed && !busy;

  function reset() {
    setFile(null);
    setRows([]);
    setPackageVersion('');
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
      const parsed = await parseProjectImportFile(selected, sourceFormat);
      setPackageVersion(parsed.packageVersion);
      setRows(parsed.rows);
    } catch (error) {
      setParseError(error instanceof Error ? error.message : 'Không thể đọc tệp dữ liệu.');
    } finally {
      setParsing(false);
    }
  }

  return (
    <Dialog
      open={open}
      title="Import công trình"
      description="Kiểm tra dữ liệu trước khi tạo công trình."
      size="xl"
      onClose={close}
      footer={(
        <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
          <p className="text-xs text-slate-500">Tối đa 100 công trình · 5 MB · .xlsx hoặc .json · không lưu file nguồn</p>
          <div className="flex gap-2">
            <button type="button" className="rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-white disabled:opacity-50" onClick={close} disabled={busy}>Hủy</button>
            {!preview ? (
              <button type="button" className="rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:opacity-50" disabled={!canPrepareFile || !file || !rows.length || busy} onClick={() => previewMutation.mutate()}>
                {previewMutation.isPending ? 'Đang kiểm tra…' : `Xem trước ${rows.length || ''} công trình`}
              </button>
            ) : (
              <button type="button" className="rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:opacity-50" disabled={!canExecute} onClick={() => executeMutation.mutate()}>
                {executeMutation.isPending ? 'Đang import…' : `Tạo ${preview.valid_count} công trình`}
              </button>
            )}
          </div>
        </div>
      )}
    >
      <div className="space-y-5">
        <FormatSelector value={sourceFormat} disabled={busy} onChange={(next) => { reset(); setSourceFormat(next); }} />
        <ReadinessPanel readiness={readiness.data} loading={readiness.isLoading} error={readiness.error} onRetry={() => void readiness.refetch()} />

        <section className={`rounded-lg border p-4 ${canPrepareFile ? 'border-line' : 'border-slate-200 bg-slate-50'}`}>
          <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
            <div>
              <h3 className="text-sm font-semibold text-ink">1. Chuẩn bị file từ mẫu hiện hành</h3>
              <p className="mt-1 text-xs leading-5 text-slate-500">{sourceFormat === 'xlsx' ? 'Chỉ chỉnh sheet Cong_trinh; các sheet Ref là dữ liệu chỉ đọc.' : 'Chỉ chỉnh mảng projects; project_snapshots và references là dữ liệu chỉ đọc.'}</p>
            </div>
            <button
              type="button"
              className="inline-flex items-center gap-2 rounded-md border border-line bg-white px-3 py-2 text-xs font-semibold text-slate-700 hover:bg-slate-50 disabled:opacity-50"
              disabled={!canPrepareFile || busy}
              onClick={() => templateMutation.mutate()}
            >
              {templateMutation.isPending ? <RefreshCw className="h-4 w-4 animate-spin" /> : <Download className="h-4 w-4" />}
              Tải mẫu {sourceFormat === 'xlsx' ? 'Excel' : 'JSON'} + references
            </button>
          </div>
          <input ref={inputRef} className="sr-only" type="file" accept={sourceFormat === 'xlsx' ? '.xlsx,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' : '.json,application/json'} onChange={(event) => void selectFile(event.target.files?.[0])} />
          <button type="button" className="mt-4 flex w-full items-center justify-center gap-3 rounded-lg border border-dashed border-blue-300 bg-blue-50/50 px-4 py-6 text-sm font-semibold text-blue-700 hover:bg-blue-50 disabled:opacity-50" disabled={!canPrepareFile || busy} onClick={() => inputRef.current?.click()}>
            {parsing ? <RefreshCw className="h-5 w-5 animate-spin" /> : <Upload className="h-5 w-5" />}
            {parsing ? 'Đang đọc file…' : file ? 'Chọn file khác' : `Chọn file Công trình .${sourceFormat}`}
          </button>
          {file ? <p className="mt-2 flex items-center gap-2 text-xs text-slate-600">{sourceFormat === 'xlsx' ? <FileSpreadsheet className="h-4 w-4" /> : <FileJson className="h-4 w-4" />}{file.name} · {formatBytes(file.size)} · {rows.length} dòng · phiên bản {packageVersion || '—'}</p> : null}
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
              <InlineMessage tone="error" message="Cần sửa toàn bộ dòng lỗi trong file, chọn lại file và xem trước lại. Hệ thống chưa tạo công trình nào." />
            ) : (
              <label className="flex items-start gap-3 rounded-md border border-blue-200 bg-blue-50 p-3 text-sm leading-6 text-blue-900">
                <input type="checkbox" className="mt-1 h-4 w-4 rounded border-line text-blue-600" checked={confirmed} onChange={(event) => setConfirmed(event.target.checked)} />
                <span>Tôi xác nhận tạo {preview.valid_count} master Công trình ở trạng thái Nháp. Import không khởi chạy Pipeline, không tạo runtime, Form answers, thành viên hay dữ liệu nghiệp vụ.</span>
              </label>
            )}
          </section>
        ) : null}
      </div>
    </Dialog>
  );
}

function FormatSelector({ value, disabled, onChange }: { value: DataPackageFormat; disabled: boolean; onChange: (value: DataPackageFormat) => void }) {
  return (
    <fieldset>
      <legend className="mb-2 text-sm font-semibold text-ink">Định dạng dữ liệu</legend>
      <div className="grid grid-cols-2 gap-2 sm:max-w-md">
        {(['xlsx', 'json'] as const).map((format) => (
          <label key={format} className={`flex cursor-pointer items-center gap-2 rounded-md border px-3 py-2 text-sm font-semibold ${value === format ? 'border-blue-300 bg-blue-50 text-blue-800' : 'border-line bg-white text-slate-700'} ${disabled ? 'cursor-not-allowed opacity-60' : ''}`}>
            <input type="radio" name="project-import-format" className="h-4 w-4 text-blue-600" checked={value === format} disabled={disabled} onChange={() => onChange(format)} />
            {format === 'xlsx' ? 'Excel (.xlsx)' : 'JSON (.json)'}
          </label>
        ))}
      </div>
    </fieldset>
  );
}

function ReadinessPanel({ readiness, loading, error, onRetry }: {
  readiness?: ProjectExcelReadiness;
  loading: boolean;
  error: unknown;
  onRetry: () => void;
}) {
  if (loading) {
    return <div className="flex items-center gap-2 rounded-lg border border-line bg-slate-50 p-4 text-sm text-slate-600"><RefreshCw className="h-4 w-4 animate-spin" />Đang kiểm tra Pipeline và Biểu mẫu quy trình…</div>;
  }
  if (error || !readiness) {
    return (
      <div className="rounded-lg border border-rose-200 bg-rose-50 p-4 text-sm text-rose-800">
        <p className="font-semibold">Không kiểm tra được dữ liệu nền</p>
        <button type="button" className="mt-2 rounded-md border border-rose-200 bg-white px-3 py-1.5 font-semibold" onClick={onRetry}>Kiểm tra lại</button>
      </div>
    );
  }
  return (
    <section className={`rounded-lg border p-4 ${readiness.ready ? 'border-emerald-200 bg-emerald-50' : 'border-amber-200 bg-amber-50'}`}>
      <div className="flex items-start gap-3">
        {readiness.ready ? <CheckCircle2 className="mt-0.5 h-5 w-5 shrink-0 text-emerald-700" /> : <AlertCircle className="mt-0.5 h-5 w-5 shrink-0 text-amber-700" />}
        <div className="min-w-0 flex-1">
          <h3 className={`text-sm font-semibold ${readiness.ready ? 'text-emerald-900' : 'text-amber-900'}`}>{readiness.ready ? 'Dữ liệu nền đã sẵn sàng' : 'Cần cập nhật dữ liệu nền trước khi import'}</h3>
          <p className="mt-1 text-xs leading-5 text-slate-700">{readiness.ready_pipeline_count} Pipeline sẵn sàng · {readiness.published_form_version_count} phiên bản Form · {readiness.field_count} Fields · {readiness.active_client_partner_count} chủ đầu tư active</p>
          {readiness.blockers.length ? (
            <div className="mt-3 flex flex-wrap gap-2">
              {readiness.blockers.map((blocker) => (
                <Link key={blocker.code} to={blocker.route} className="rounded-md border border-amber-300 bg-white px-3 py-2 text-xs font-semibold text-amber-900 hover:bg-amber-100">
                  {blocker.message} — Cập nhật
                </Link>
              ))}
            </div>
          ) : null}
          {readiness.warnings.map((warning) => <p key={warning.code} className="mt-2 text-xs text-amber-800">{warning.message}{warning.count ? ` (${warning.count})` : ''}</p>)}
        </div>
      </div>
    </section>
  );
}

function PreviewRows({ rows }: { rows: ProjectExcelImportPreview['rows'] }) {
  return (
    <>
      <div className="space-y-2 md:hidden">
        {rows.map((row) => (
          <article key={row.row_number} className={`rounded-lg border p-3 ${row.valid ? 'border-line bg-white' : 'border-rose-200 bg-rose-50/60'}`}>
            <div className="flex items-start justify-between gap-3"><p className="font-semibold text-ink">{row.project_code || '—'} · {row.project_name || '—'}</p><span className="font-mono text-xs text-slate-500">Dòng {row.row_number}</span></div>
            <p className="mt-2 text-xs text-slate-600">Chủ đầu tư: {row.client_name || '—'}</p>
            <p className="mt-1 text-xs text-slate-600">Pipeline: {row.pipeline_name ? `${row.pipeline_name} · r${row.pipeline_revision}` : '—'}</p>
            <RowIssues row={row} />
          </article>
        ))}
      </div>
      <div className="hidden overflow-hidden rounded-lg border border-line md:block">
        <div className="consflow-table-viewport max-h-[42vh]">
          <table className="min-w-full divide-y divide-line text-sm">
            <thead className="bg-slate-50 text-left text-xs uppercase tracking-wide text-slate-500">
              <tr><th className="px-3 py-2">Dòng</th><th className="px-3 py-2">Công trình</th><th className="px-3 py-2">Chủ đầu tư</th><th className="px-3 py-2">Pipeline</th><th className="px-3 py-2">Ngân sách</th><th className="px-3 py-2">Kết quả</th></tr>
            </thead>
            <tbody className="divide-y divide-line bg-white">
              {rows.map((row) => (
                <tr key={row.row_number} className={row.valid ? '' : 'bg-rose-50/60'}>
                  <td className="px-3 py-3 align-top font-mono text-xs">{row.row_number}</td>
                  <td className="px-3 py-3 align-top"><p className="font-mono text-xs text-slate-500">{row.project_code || '—'}</p><p className="mt-1 font-semibold text-ink">{row.project_name || '—'}</p></td>
                  <td className="px-3 py-3 align-top"><p className="font-semibold text-slate-700">{row.client_name || '—'}</p><p className="mt-1 font-mono text-xs text-slate-500">{row.client_partner_code || 'Chưa liên kết danh mục'}</p></td>
                  <td className="px-3 py-3 align-top"><p className="font-semibold text-slate-700">{row.pipeline_name || '—'}</p><p className="mt-1 text-xs text-slate-500">{row.pipeline_revision ? `Revision ${row.pipeline_revision}` : '—'}</p></td>
                  <td className="whitespace-nowrap px-3 py-3 align-top text-right tabular-nums">{formatVND(row.estimated_budget_vnd)}</td>
                  <td className="px-3 py-3 align-top"><RowIssues row={row} /></td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </>
  );
}

function RowIssues({ row }: { row: ProjectExcelImportPreview['rows'][number] }) {
  return (
    <div className="mt-3 md:mt-0">
      {row.valid ? <span className="inline-flex items-center gap-1 text-xs font-semibold text-emerald-700"><CheckCircle2 className="h-4 w-4" />Sẵn sàng</span> : (
        <ul className="space-y-1 text-xs text-rose-700">{row.errors.map((issue, index) => <li key={`${issue.field}-${index}`}>{issue.field}: {issue.message}</li>)}</ul>
      )}
      {row.warnings.map((issue, index) => <p key={`${issue.field}-${index}`} className="mt-1 text-xs text-amber-700">{issue.message}</p>)}
    </div>
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
  if (value < 1024) return `${value} B`;
  if (value < 1024 * 1024) return `${Math.ceil(value / 1024)} KB`;
  return `${(value / (1024 * 1024)).toFixed(1)} MB`;
}

function formatVND(value: number) {
  return new Intl.NumberFormat('vi-VN', { style: 'currency', currency: 'VND', maximumFractionDigits: 0 }).format(value || 0);
}
