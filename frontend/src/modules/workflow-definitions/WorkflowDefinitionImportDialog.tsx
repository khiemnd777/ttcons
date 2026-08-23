import { useRef, useState } from 'react';
import { useMutation } from '@tanstack/react-query';
import { AlertCircle, Download, FileJson, FileSpreadsheet, RefreshCw, Upload } from 'lucide-react';
import { Dialog } from '../../components/Dialog';
import { downloadBlob, post } from '../../lib/api/client';
import {
  parseWorkflowDefinitionImportFile,
  saveWorkflowDefinitionBlob,
  type ParsedWorkflowDefinitionFile,
} from './workflowDefinitionExcel';
import type {
  WorkflowDefinitionImportPreview,
  WorkflowDefinitionImportResult,
  WorkflowDefinitionScope,
} from './workflowDefinitionExcelTypes';
import type { DataPackageFormat } from '../_shared/dataPackage';

export function WorkflowDefinitionImportDialog({ open, templateScope, allowedScopes, onClose, onSuccess, onError }: {
  open: boolean;
  templateScope: WorkflowDefinitionScope;
  allowedScopes: WorkflowDefinitionScope[];
  onClose: () => void;
  onSuccess: (result: WorkflowDefinitionImportResult) => void;
  onError: (title: string, error: unknown) => void;
}) {
  const inputRef = useRef<HTMLInputElement>(null);
  const [file, setFile] = useState<File | null>(null);
  const [sourceFormat, setSourceFormat] = useState<DataPackageFormat>('xlsx');
  const [parsed, setParsed] = useState<ParsedWorkflowDefinitionFile | null>(null);
  const [preview, setPreview] = useState<WorkflowDefinitionImportPreview | null>(null);
  const [parseError, setParseError] = useState('');
  const [parsing, setParsing] = useState(false);
  const [confirmed, setConfirmed] = useState(false);
  const [idempotencyKey, setIdempotencyKey] = useState('');

  const templateMutation = useMutation({
    mutationFn: () => downloadBlob(`/workflow-definitions/imports/template?scope=${templateScope}&format=${sourceFormat}`),
    onSuccess: (blob) => saveWorkflowDefinitionBlob(blob, templateScope, 'template', sourceFormat),
    onError: (error) => onError('Không tải được file mẫu định nghĩa workflow', error),
  });
  const previewMutation = useMutation({
    mutationFn: () => post<WorkflowDefinitionImportPreview>('/workflow-definitions/imports/preview', buildPayload()),
    onSuccess: (result) => {
      setPreview(result);
      setConfirmed(false);
      setIdempotencyKey(crypto.randomUUID());
    },
    onError: (error) => onError('Không xem trước được gói định nghĩa workflow', error),
  });
  const executeMutation = useMutation({
    mutationFn: () => post<WorkflowDefinitionImportResult>('/workflow-definitions/imports/execute', {
      ...buildPayload(), preview_hash: preview?.preview_hash, idempotency_key: idempotencyKey,
    }),
    onSuccess: (result) => {
      reset();
      onSuccess(result);
    },
    onError: (error) => onError('Không import được định nghĩa workflow', error),
  });

  const busy = parsing || templateMutation.isPending || previewMutation.isPending || executeMutation.isPending;
  const canExecute = Boolean(preview?.preview_hash) && preview?.blocker_count === 0 && confirmed && !busy;

  function buildPayload() {
    if (!file || !parsed) throw new Error('Chưa chọn file định nghĩa workflow.');
    return { ...parsed, source_filename: file.name, source_size_bytes: file.size };
  }

  function reset() {
    setFile(null);
    setParsed(null);
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
      const next = await parseWorkflowDefinitionImportFile(selected, sourceFormat);
      if (!allowedScopes.includes(next.scope)) {
        throw new Error(`Màn hình này không nhận package scope=${next.scope}. Scope cho phép: ${allowedScopes.join(', ')}.`);
      }
      setParsed(next);
    } catch (error) {
      setParseError(error instanceof Error ? error.message : 'Không thể đọc Workflow Definition package.');
    } finally {
      setParsing(false);
    }
  }

  return (
    <Dialog
      open={open}
      title="Import dữ liệu định nghĩa workflow"
      description="Đối chiếu Form, Fields, Pipeline, role và permission trước khi tạo bản nháp đích."
      size="xl"
      onClose={close}
      footer={(
        <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
          <p className="text-xs text-slate-500">Tối đa 25 Pipeline · 100 Form · 10 MB · không lưu file nguồn</p>
          <div className="flex gap-2">
            <button type="button" className="rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-white disabled:opacity-50" onClick={close} disabled={busy}>Hủy</button>
            {!preview ? (
              <button type="button" className="rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:opacity-50" disabled={!file || !parsed || busy} onClick={() => previewMutation.mutate()}>
                {previewMutation.isPending ? 'Đang đối chiếu…' : 'Xem trước và đối chiếu'}
              </button>
            ) : (
              <button type="button" className="rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:opacity-50" disabled={!canExecute} onClick={() => executeMutation.mutate()}>
                {executeMutation.isPending ? 'Đang import…' : 'Tạo các bản nháp'}
              </button>
            )}
          </div>
        </div>
      )}
    >
      <div className="space-y-5">
        <fieldset>
          <legend className="mb-2 text-sm font-semibold text-ink">Định dạng dữ liệu</legend>
          <div className="grid grid-cols-2 gap-2 sm:max-w-md">
            {(['xlsx', 'json'] as const).map((format) => <label key={format} className={`flex cursor-pointer items-center gap-2 rounded-md border px-3 py-2 text-sm font-semibold ${sourceFormat === format ? 'border-blue-300 bg-blue-50 text-blue-800' : 'border-line bg-white text-slate-700'} ${busy ? 'cursor-not-allowed opacity-60' : ''}`}><input type="radio" name="workflow-import-format" className="h-4 w-4 text-blue-600" checked={sourceFormat === format} disabled={busy} onChange={() => { reset(); setSourceFormat(format); }} />{format === 'xlsx' ? 'Excel (.xlsx)' : 'JSON (.json)'}</label>)}
          </div>
        </fieldset>
        <section className="rounded-lg border border-line p-4">
          <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
            <div>
              <h3 className="text-sm font-semibold text-ink">1. Chọn Workflow Definition package</h3>
              <p className="mt-1 text-xs leading-5 text-slate-500">{sourceFormat === 'xlsx' ? 'Tải mẫu mới để có đúng 12 sheet, headers và catalog tham chiếu.' : 'JSON dùng topology nested, package keys và đầy đủ role/permission/Form/Pipeline references.'}</p>
            </div>
            <button type="button" className="inline-flex items-center gap-2 rounded-md border border-line bg-white px-3 py-2 text-xs font-semibold text-slate-700 hover:bg-slate-50 disabled:opacity-50" disabled={busy} onClick={() => templateMutation.mutate()}>
              {templateMutation.isPending ? <RefreshCw className="h-4 w-4 animate-spin" /> : <Download className="h-4 w-4" />}
              Tải mẫu {sourceFormat === 'xlsx' ? 'Excel' : 'JSON'} + references
            </button>
          </div>
          <input ref={inputRef} className="sr-only" type="file" accept={sourceFormat === 'xlsx' ? '.xlsx,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' : '.json,application/json'} onChange={(event) => void selectFile(event.target.files?.[0])} />
          <button type="button" className="mt-4 flex w-full items-center justify-center gap-3 rounded-lg border border-dashed border-blue-300 bg-blue-50/50 px-4 py-6 text-sm font-semibold text-blue-700 hover:bg-blue-50 disabled:opacity-50" disabled={busy} onClick={() => inputRef.current?.click()}>
            {parsing ? <RefreshCw className="h-5 w-5 animate-spin" /> : <Upload className="h-5 w-5" />}
            {parsing ? 'Đang đọc package…' : file ? 'Chọn file khác' : `Chọn file Workflow Definition .${sourceFormat}`}
          </button>
          {file ? <p className="mt-2 flex items-center gap-2 text-xs text-slate-600">{sourceFormat === 'xlsx' ? <FileSpreadsheet className="h-4 w-4" /> : <FileJson className="h-4 w-4" />}{file.name} · {formatBytes(file.size)} · scope {parsed?.scope ?? '—'} · v{parsed?.package_version ?? '—'}</p> : null}
          {parseError ? <InlineIssue message={parseError} /> : null}
        </section>

        {preview ? (
          <section className="space-y-4">
            <div>
              <h3 className="text-sm font-semibold text-ink">2. Đối chiếu dependency và hành động đích</h3>
              <div className="mt-3 grid grid-cols-2 gap-2 sm:grid-cols-4">
                <Metric label="Pipeline" value={preview.pipeline_count} />
                <Metric label="Form" value={preview.form_count} />
                <Metric label="Điểm chặn" value={preview.blocker_count} tone={preview.blocker_count ? 'error' : 'success'} />
                <Metric label="Cảnh báo" value={preview.warning_count} tone={preview.warning_count ? 'warning' : 'default'} />
              </div>
            </div>
            <EntityMappings entities={preview.entities} />
            {preview.blockers.length ? <IssueList title="Cần xử lý trước khi import" issues={preview.blockers} tone="error" /> : null}
            {preview.warnings.length ? <IssueList title="Cảnh báo" issues={preview.warnings} tone="warning" /> : null}
            {!preview.blocker_count ? (
              <label className="flex items-start gap-3 rounded-md border border-blue-200 bg-blue-50 p-3 text-sm leading-6 text-blue-900">
                <input type="checkbox" className="mt-1 h-4 w-4 rounded border-line text-blue-600" checked={confirmed} onChange={(event) => setConfirmed(event.target.checked)} />
                <span>{preview.confirmation || 'Import chỉ tạo bản nháp; không phát hành, không chạy Pipeline và không tạo dữ liệu runtime.'}</span>
              </label>
            ) : null}
          </section>
        ) : null}
      </div>
    </Dialog>
  );
}

function EntityMappings({ entities }: { entities: WorkflowDefinitionImportPreview['entities'] }) {
  const labels: Record<string, string> = { create_template: 'Tạo Form template + draft v1', create_version: 'Tạo draft version kế tiếp', create_family: 'Tạo Pipeline family mới', create_revision: 'Tạo draft revision kế tiếp' };
  return (
    <div className="overflow-hidden rounded-lg border border-line">
      <div className="max-h-64 overflow-auto">
        <table className="min-w-full divide-y divide-line text-sm">
          <thead className="sticky top-0 bg-slate-50 text-left text-xs uppercase tracking-wide text-slate-500"><tr><th className="px-3 py-2">Loại</th><th className="px-3 py-2">Định nghĩa</th><th className="px-3 py-2">Hành động</th></tr></thead>
          <tbody className="divide-y divide-line bg-white">{entities.map((entity) => <tr key={`${entity.kind}-${entity.key}`}><td className="px-3 py-3 font-semibold text-slate-600">{entity.kind === 'form' ? 'Form' : 'Pipeline'}</td><td className="px-3 py-3"><p className="font-semibold text-ink">{entity.name}</p><p className="font-mono text-xs text-slate-500">{entity.key}</p></td><td className="px-3 py-3 text-slate-700">{labels[entity.action] ?? entity.action}</td></tr>)}</tbody>
        </table>
      </div>
    </div>
  );
}

function IssueList({ title, issues, tone }: { title: string; issues: WorkflowDefinitionImportPreview['blockers']; tone: 'error' | 'warning' }) {
  const style = tone === 'error' ? 'border-rose-200 bg-rose-50 text-rose-800' : 'border-amber-200 bg-amber-50 text-amber-800';
  return <div className={`rounded-lg border p-4 ${style}`}><p className="font-semibold">{title}</p><ul className="mt-2 space-y-1 text-xs">{issues.map((issue, index) => <li key={`${issue.code}-${issue.entity_key}-${index}`}>{issue.sheet}{issue.row_number ? ` dòng ${issue.row_number}` : ''} · {issue.entity_key || 'package'}: {issue.message}</li>)}</ul></div>;
}

function Metric({ label, value, tone = 'default' }: { label: string; value: number; tone?: 'default' | 'success' | 'warning' | 'error' }) {
  const colors = { default: 'border-line bg-slate-50 text-slate-700', success: 'border-emerald-200 bg-emerald-50 text-emerald-700', warning: 'border-amber-200 bg-amber-50 text-amber-700', error: 'border-rose-200 bg-rose-50 text-rose-700' };
  return <div className={`rounded-md border px-3 py-2 ${colors[tone]}`}><p className="text-xs font-medium">{label}</p><p className="mt-1 text-lg font-bold">{value}</p></div>;
}

function InlineIssue({ message }: { message: string }) {
  return <div className="mt-3 flex items-start gap-2 rounded-md border border-rose-200 bg-rose-50 p-3 text-sm text-rose-700"><AlertCircle className="mt-0.5 h-4 w-4 shrink-0" /><span>{message}</span></div>;
}

function formatBytes(value: number) {
  if (value < 1024) return `${value} B`;
  if (value < 1024 * 1024) return `${Math.ceil(value / 1024)} KB`;
  return `${(value / (1024 * 1024)).toFixed(1)} MB`;
}
