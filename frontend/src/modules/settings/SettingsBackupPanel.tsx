import { DragEvent, useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { AlertTriangle, CheckCircle2, Download, FileJson, Loader2, RotateCcw, Upload } from 'lucide-react';
import { Dialog } from '../../components/Dialog';
import { useToast } from '../../components/ToastProvider';
import { downloadBlob, postForm } from '../../lib/api/client';

const maxBackupFileSize = 256 * 1024;

type BackupWarning = { code: string; key?: string; message: string };
type BackupSecret = { key: string; configured: boolean };
type BackupChange = {
  key: string;
  label: string;
  category: string;
  old_value: unknown;
  new_value: unknown;
  requires_restart: boolean;
  expected_version: number;
};
type BackupPreview = {
  file_hash: string;
  format_version: number;
  exported_at: string;
  source_environment: string;
  registry_fingerprint: string;
  current_registry_fingerprint: string;
  changed_count: number;
  unchanged_count: number;
  unsupported_count: number;
  preserved_missing_count: number;
  restart_required_count: number;
  immediate_count: number;
  changes: BackupChange[];
  warnings: BackupWarning[];
  expected_versions: Record<string, number>;
  excluded_secrets: BackupSecret[];
};
type RestoreResult = {
  updated: number;
  restart_required_count: number;
  immediate_count: number;
  warnings: BackupWarning[];
};

export function SettingsBackupPanel({ canRestore }: { canRestore: boolean }) {
  const toast = useToast();
  const queryClient = useQueryClient();
  const [open, setOpen] = useState(false);
  const [file, setFile] = useState<File | null>(null);
  const [fileError, setFileError] = useState('');
  const [operationError, setOperationError] = useState('');
  const [preview, setPreview] = useState<BackupPreview | null>(null);
  const [restored, setRestored] = useState<RestoreResult | null>(null);
  const [dragging, setDragging] = useState(false);
  const [inputKey, setInputKey] = useState(0);

  const download = useMutation({
    mutationFn: () => downloadBlob('/settings/backup'),
    onSuccess: (blob) => {
      const objectUrl = URL.createObjectURL(blob);
      const anchor = document.createElement('a');
      anchor.href = objectUrl;
      anchor.download = settingsBackupFilename(new Date());
      anchor.click();
      window.setTimeout(() => URL.revokeObjectURL(objectUrl), 1000);
      toast.success('Đã tải bản sao thiết lập hệ thống');
    },
    onError: (error) => toast.error('Không tải được bản sao thiết lập', error),
  });

  const checkBackup = useMutation({
    mutationFn: async (selectedFile: File) => {
      const body = new FormData();
      body.append('file', selectedFile);
      return postForm<BackupPreview>('/settings/backup/preview', body);
    },
    onSuccess: (result) => {
      setPreview(result);
      setOperationError('');
    },
    onError: (error) => {
      setOperationError(errorMessage(error));
      toast.error('Không kiểm tra được bản sao cấu hình', error);
    },
  });

  const restore = useMutation({
    mutationFn: async ({ selectedFile, checked }: { selectedFile: File; checked: BackupPreview }) => {
      const body = new FormData();
      body.append('file', selectedFile);
      body.append('preview_hash', checked.file_hash);
      body.append('expected_versions', JSON.stringify(checked.expected_versions));
      return postForm<RestoreResult>('/settings/backup/restore', body);
    },
    onSuccess: async (result) => {
      await queryClient.invalidateQueries({ queryKey: ['settings'] });
      setRestored(result);
      setOperationError('');
      toast.success(`Đã khôi phục ${result.updated} thiết lập hệ thống`);
    },
    onError: (error) => {
      setOperationError(errorMessage(error));
      toast.error('Không khôi phục được thiết lập hệ thống', error);
    },
  });

  const busy = checkBackup.isPending || restore.isPending;

  function resetSelection() {
    if (busy) return;
    setFile(null);
    setFileError('');
    setOperationError('');
    setPreview(null);
    setRestored(null);
    setInputKey((value) => value + 1);
  }

  function closeDialog() {
    if (busy) return;
    setOpen(false);
    resetSelection();
  }

  function selectFile(nextFile: File | null) {
    if (!nextFile) return;
    setFile(nextFile);
    setPreview(null);
    setRestored(null);
    setOperationError('');
    setFileError(validateBackupFile(nextFile));
  }

  function previewSelectedFile() {
    if (!file) {
      setFileError('Vui lòng chọn tệp JSON bản sao cấu hình.');
      toast.warn({ title: 'Chưa thể kiểm tra bản sao', description: 'Vui lòng chọn tệp JSON bản sao cấu hình.' });
      return;
    }
    const nextError = validateBackupFile(file);
    setFileError(nextError);
    if (nextError) {
      toast.warn({ title: 'Chưa thể kiểm tra bản sao', description: nextError });
      return;
    }
    checkBackup.mutate(file);
  }

  function handleDrop(event: DragEvent<HTMLDivElement>) {
    event.preventDefault();
    setDragging(false);
    if (!busy) selectFile(event.dataTransfer.files?.[0] ?? null);
  }

  const footer = restored ? (
    <div className="flex justify-end"><button type="button" className="rounded-md bg-blue-600 px-4 py-2 text-sm font-semibold text-white" onClick={closeDialog}>Đóng</button></div>
  ) : preview ? (
    <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
      <button type="button" className="rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-slate-700" disabled={busy} onClick={closeDialog}>Hủy</button>
      <button type="button" className="inline-flex items-center justify-center gap-2 rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-slate-700" disabled={busy} onClick={resetSelection}><RotateCcw className="h-4 w-4" />Chọn file khác</button>
      <button type="button" className="inline-flex items-center justify-center gap-2 rounded-md bg-amber-600 px-4 py-2 text-sm font-semibold text-white disabled:opacity-50" disabled={busy || preview.changed_count === 0 || !file} onClick={() => file && restore.mutate({ selectedFile: file, checked: preview })}>{restore.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <Upload className="h-4 w-4" />}Khôi phục {preview.changed_count} mục</button>
    </div>
  ) : (
    <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
      <button type="button" className="rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-slate-700" disabled={busy} onClick={closeDialog}>Hủy</button>
      <button type="button" className="inline-flex items-center justify-center gap-2 rounded-md bg-blue-600 px-4 py-2 text-sm font-semibold text-white disabled:opacity-50" disabled={busy || !file || Boolean(fileError)} onClick={previewSelectedFile}>{checkBackup.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <FileJson className="h-4 w-4" />}Kiểm tra bản sao</button>
    </div>
  );

  return (
    <>
      <section className="rounded-lg border border-line bg-white p-4">
        <div className="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
          <div className="min-w-0">
            <div className="flex items-center gap-2"><FileJson className="h-5 w-5 text-blue-600" /><h2 className="text-sm font-semibold text-ink">Sao lưu và khôi phục cấu hình</h2></div>
            <p className="mt-1 max-w-3xl text-sm text-slate-600">JSON chỉ gồm thiết lập database/UI. Cấu hình lõi .env, con dấu và giá trị secret không được đưa vào bản sao.</p>
            {!canRestore ? <p className="mt-2 text-xs font-medium text-blue-700">Admin được tải bản sao; chỉ vai trò Nhà phát triển được khôi phục.</p> : null}
          </div>
          <div className="flex shrink-0 flex-col gap-2 sm:flex-row">
            <button type="button" className="inline-flex items-center justify-center gap-2 rounded-md border border-blue-200 bg-white px-3 py-2 text-sm font-semibold text-blue-700 hover:bg-blue-50 disabled:opacity-50" disabled={download.isPending} onClick={() => download.mutate()}>{download.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <Download className="h-4 w-4" />}Tải bản sao JSON</button>
            {canRestore ? <button type="button" className="inline-flex items-center justify-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700" onClick={() => setOpen(true)}><Upload className="h-4 w-4" />Khôi phục từ JSON</button> : null}
          </div>
        </div>
      </section>

      <Dialog open={open} title={restored ? 'Khôi phục cấu hình thành công' : preview ? 'Xác nhận khôi phục cấu hình' : 'Khôi phục thiết lập hệ thống'} description={preview || restored ? undefined : 'Tệp được kiểm tra và so sánh trước; chưa có dữ liệu nào bị ghi ở bước này.'} size={preview ? 'xl' : 'lg'} onClose={closeDialog} footer={footer}>
        {restored ? <RestoreSuccess result={restored} /> : preview ? <BackupPreviewContent file={file} preview={preview} operationError={operationError} /> : (
          <div className="space-y-4">
            <div className={`rounded-lg border-2 border-dashed p-6 text-center transition ${dragging ? 'border-blue-400 bg-blue-50' : fileError ? 'border-rose-300 bg-rose-50' : 'border-slate-300 bg-slate-50'}`} onDragEnter={(event) => { event.preventDefault(); setDragging(true); }} onDragOver={(event) => event.preventDefault()} onDragLeave={() => setDragging(false)} onDrop={handleDrop}>
              <FileJson className="mx-auto h-9 w-9 text-slate-400" />
              <p className="mt-3 text-sm font-semibold text-ink">Kéo thả file JSON vào đây</p>
              <p className="mt-1 text-xs text-slate-500">Chỉ nhận JSON · tối đa 256 KB</p>
              <label className="mt-4 inline-flex cursor-pointer items-center gap-2 rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-blue-700 hover:bg-blue-50"><Upload className="h-4 w-4" />Chọn file<input key={inputKey} className="sr-only" type="file" accept=".json,application/json" disabled={busy} onChange={(event) => selectFile(event.target.files?.[0] ?? null)} /></label>
            </div>
            {file ? <div className="flex items-center justify-between gap-3 rounded-md border border-line bg-white px-3 py-2"><div className="min-w-0"><p className="truncate text-sm font-semibold text-ink">{file.name}</p><p className="text-xs text-slate-500">{formatBytes(file.size)}</p></div><button type="button" className="text-xs font-semibold text-blue-700" disabled={busy} onClick={resetSelection}>Chọn lại</button></div> : null}
            {fileError ? <InlineError message={fileError} /> : null}
            {operationError ? <InlineError message={operationError} /> : null}
            <div className="rounded-md border border-amber-200 bg-amber-50 p-3 text-sm text-amber-900"><span className="font-semibold">Secret không nằm trong file.</span> SMTP_PASSWORD hiện có sẽ được giữ nguyên; hệ thống mới cần nhập lại secret này.</div>
          </div>
        )}
      </Dialog>
    </>
  );
}

function BackupPreviewContent({ file, preview, operationError }: { file: File | null; preview: BackupPreview; operationError: string }) {
  return (
    <div className="space-y-4">
      <div className="rounded-md border border-line bg-slate-50 p-3 text-sm text-slate-700"><p className="font-semibold text-ink">{file?.name}</p><p className="mt-1 text-xs">Xuất lúc {dateTimeText(preview.exported_at)} · Nguồn {preview.source_environment} · Format v{preview.format_version}</p></div>
      <div className="grid grid-cols-2 gap-2 lg:grid-cols-4">
        <PreviewMetric label="Sẽ thay đổi" value={preview.changed_count} tone="blue" />
        <PreviewMetric label="Giữ nguyên" value={preview.unchanged_count + preview.preserved_missing_count} tone="slate" />
        <PreviewMetric label="Không hỗ trợ" value={preview.unsupported_count} tone="amber" />
        <PreviewMetric label="Cần restart" value={preview.restart_required_count} tone="amber" />
      </div>
      {preview.excluded_secrets.length ? <div className="rounded-md border border-amber-200 bg-amber-50 p-3 text-sm text-amber-900"><span className="font-semibold">Secret được bảo toàn:</span> {preview.excluded_secrets.map((secret) => `${secret.key} (${secret.configured ? 'đã cấu hình' : 'chưa cấu hình'})`).join(', ')}. File không chứa giá trị secret.</div> : null}
      {preview.warnings.length ? <section className="rounded-md border border-amber-200 bg-amber-50 p-3"><h3 className="flex items-center gap-2 text-sm font-semibold text-amber-900"><AlertTriangle className="h-4 w-4" />Cảnh báo</h3><ul className="mt-2 space-y-1 text-sm text-amber-900">{preview.warnings.map((warning, index) => <li key={`${warning.code}:${warning.key ?? index}`}>• {warning.key ? <code className="font-semibold">{warning.key}: </code> : null}{warning.message}</li>)}</ul></section> : null}
      {operationError ? <InlineError message={operationError} /> : null}
      {preview.changes.length ? <section><h3 className="text-sm font-semibold text-ink">Thay đổi sẽ áp dụng</h3><div className="mt-2 hidden overflow-x-auto rounded-lg border border-line md:block"><table className="w-full min-w-[760px] text-left text-sm"><thead className="bg-slate-50 text-xs uppercase text-slate-500"><tr><th className="px-3 py-2">Thiết lập</th><th className="px-3 py-2">Hiện tại</th><th className="px-3 py-2">Khôi phục thành</th><th className="px-3 py-2">Hiệu lực</th></tr></thead><tbody className="divide-y divide-line">{preview.changes.map((change) => <tr key={change.key}><td className="px-3 py-3"><p className="font-semibold text-ink">{change.label}</p><code className="text-[11px] text-slate-400">{change.key}</code></td><td className="px-3 py-3"><code className="break-all text-xs">{displayBackupValue(change.old_value)}</code></td><td className="px-3 py-3"><code className="break-all text-xs font-semibold text-blue-700">{displayBackupValue(change.new_value)}</code></td><td className="px-3 py-3 text-xs font-semibold text-slate-600">{change.requires_restart ? 'Cần restart' : 'Ngay lập tức'}</td></tr>)}</tbody></table></div><div className="mt-2 space-y-2 md:hidden">{preview.changes.map((change) => <article key={change.key} className="rounded-lg border border-line bg-white p-3"><p className="font-semibold text-ink">{change.label}</p><code className="text-[11px] text-slate-400">{change.key}</code><div className="mt-3 grid gap-2 text-xs"><p><span className="text-slate-500">Hiện tại:</span> <code>{displayBackupValue(change.old_value)}</code></p><p><span className="text-slate-500">Khôi phục:</span> <code className="font-semibold text-blue-700">{displayBackupValue(change.new_value)}</code></p><p className="font-semibold text-slate-600">{change.requires_restart ? 'Cần restart backend' : 'Hiệu lực ngay'}</p></div></article>)}</div></section> : <div className="rounded-md border border-emerald-200 bg-emerald-50 p-4 text-sm text-emerald-900"><span className="font-semibold">Không có thay đổi cần khôi phục.</span> Các giá trị được hỗ trợ đã giống hệ thống hiện tại.</div>}
    </div>
  );
}

function RestoreSuccess({ result }: { result: RestoreResult }) {
  return <div className="py-5 text-center"><CheckCircle2 className="mx-auto h-12 w-12 text-emerald-600" /><h3 className="mt-3 text-lg font-semibold text-ink">Đã khôi phục {result.updated} thiết lập</h3><p className="mt-2 text-sm text-slate-600">{result.immediate_count} thiết lập có hiệu lực ngay. {result.restart_required_count} thiết lập cần khởi động lại backend.</p>{result.restart_required_count ? <div className="mx-auto mt-4 max-w-xl rounded-md border border-amber-200 bg-amber-50 p-3 text-sm text-amber-900">Hãy khởi động lại backend trong thời gian vận hành phù hợp để áp dụng toàn bộ cấu hình.</div> : null}</div>;
}

function PreviewMetric({ label, value, tone }: { label: string; value: number; tone: 'blue' | 'slate' | 'amber' }) {
  const tones = { blue: 'border-blue-200 bg-blue-50 text-blue-800', slate: 'border-line bg-slate-50 text-slate-700', amber: 'border-amber-200 bg-amber-50 text-amber-800' };
  return <div className={`rounded-md border p-3 ${tones[tone]}`}><p className="text-xl font-semibold">{value}</p><p className="mt-1 text-xs font-semibold">{label}</p></div>;
}

function InlineError({ message }: { message: string }) {
  return <div className="rounded-md border border-rose-200 bg-rose-50 p-3 text-sm font-medium text-rose-700">{message}</div>;
}

function validateBackupFile(file: File) {
  if (!file.name.toLocaleLowerCase().endsWith('.json')) return 'Chỉ chấp nhận tệp có phần mở rộng .json.';
  if (file.size === 0) return 'Tệp bản sao không được rỗng.';
  if (file.size > maxBackupFileSize) return 'Tệp bản sao không được vượt quá 256 KB.';
  return '';
}

function settingsBackupFilename(now: Date) {
  const part = (value: number) => String(value).padStart(2, '0');
  return `consflow-settings-${now.getFullYear()}${part(now.getMonth() + 1)}${part(now.getDate())}-${part(now.getHours())}${part(now.getMinutes())}${part(now.getSeconds())}.json`;
}

function formatBytes(value: number) { return value < 1024 ? `${value} B` : `${(value / 1024).toFixed(1)} KB`; }
function dateTimeText(value: string) { const date = new Date(value); return Number.isNaN(date.getTime()) ? value : date.toLocaleString('vi-VN'); }
function errorMessage(error: unknown) { return error instanceof Error ? error.message : 'Không thực hiện được thao tác.'; }
function displayBackupValue(value: unknown) {
  if (typeof value === 'boolean') return value ? 'Bật' : 'Tắt';
  if (Array.isArray(value)) return value.join(', ');
  if (value === null || value === undefined || value === '') return '—';
  if (typeof value === 'object') return JSON.stringify(value);
  return String(value);
}
