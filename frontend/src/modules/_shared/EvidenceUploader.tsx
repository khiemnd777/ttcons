import { type MutableRefObject, type ReactNode, useRef } from 'react';
import { Loader2, Plus, RotateCcw, Trash2, Upload } from 'lucide-react';
import clsx from 'clsx';
import './evidenceUploader.css';
import { evidenceFileAccept, evidenceSupportedFileHint, type EvidenceFileClassification, type EvidenceUploadBusinessType } from './evidenceFileTypes';
import { formatEvidenceFileSize } from './EvidenceRecordViewerDialog';
import { evidenceUploadBatchLimit, evidenceUploadConcurrency, evidenceUploadOverallProgress } from '../work-items/evidenceUploadQueue';
import type { EvidenceUploadStatus } from './evidenceUploadSession';

export type EvidenceUploadRow = {
  id: string;
  file: File | null;
  classification: EvidenceFileClassification | null;
  name: string;
  caption: string;
  fileIssue: string;
  status: EvidenceUploadStatus;
  progress: number;
  error: string;
};

type EvidenceUploaderProps = {
  uploadRows: EvidenceUploadRow[];
  batchUploading: boolean;
  updateUploadRow: (id: string, update: (row: EvidenceUploadRow) => EvidenceUploadRow) => void;
  onFileChange: (id: string, file: File | null, input: HTMLInputElement) => void;
  addUploadRow: () => void;
  removeUploadRow: (id: string) => void;
  canRemoveRow?: (row: EvidenceUploadRow) => boolean;
  uploadEvidenceRows?: (ids?: readonly string[]) => Promise<void>;
  retryUploadRow?: (id: string) => void;
  onPreview?: (row: EvidenceUploadRow) => void;
  actions?: ReactNode;
  maxFiles?: number;
  addDisabled?: boolean;
  requirementEvidenceType?: EvidenceUploadBusinessType | null;
  fileInputRef?: MutableRefObject<HTMLInputElement | null>;
  fileInputRefs?: MutableRefObject<Map<string, HTMLInputElement>>;
  extraColumn?: {
    label: string;
    render: (row: EvidenceUploadRow, index: number) => ReactNode;
  };
};

// The WorkItem upload table is the single upload presentation owner. Adapters
// control persistence timing; this component never submits its enclosing form.
export function EvidenceUploader({ uploadRows, batchUploading, updateUploadRow, onFileChange, addUploadRow, removeUploadRow, canRemoveRow, uploadEvidenceRows, retryUploadRow, onPreview, actions, maxFiles = evidenceUploadBatchLimit, addDisabled = false, requirementEvidenceType, fileInputRef: suppliedInputRef, fileInputRefs: suppliedInputRefs, extraColumn }: EvidenceUploaderProps) {
  const localInputRef = useRef<HTMLInputElement | null>(null);
  const localInputRefs = useRef(new Map<string, HTMLInputElement>());
  const fileInputRef = suppliedInputRef ?? localInputRef;
  const fileInputRefs = suppliedInputRefs ?? localInputRefs;
  const selectedRows = uploadRows.filter((row) => row.file);
  const overallProgress = evidenceUploadOverallProgress(selectedRows.map((row) => ({ fileSize: row.file?.size ?? 0, progress: row.progress })));
  const pendingUploadCount = uploadRows.filter((row) => row.file && row.classification && !row.fileIssue && (row.status === 'queued' || row.status === 'error')).length;
  return <div className={clsx('evidence-uploader min-w-0', extraColumn && 'evidence-uploader--extra-column')} data-component="evidence-uploader">
          <div className="evidence-upload-header px-2 pb-1 text-[11px] font-semibold uppercase tracking-wide text-slate-500">
            <span>#</span>
            <span>Tệp</span>
            <span>Loại</span>
            {extraColumn ? <span>{extraColumn.label}</span> : null}
            <span>Tên minh chứng</span>
            <span>Ghi chú</span>
            <span>Tiến độ</span>
          </div>

          <div className="space-y-2">
            {uploadRows.map((row, index) => {
              const rowBusy = row.status === 'uploading' || row.status === 'processing';
              const photoMismatch = requirementEvidenceType === 'photo' && row.classification?.evidenceType === 'document';
              return (
                <article data-validation-key={row.id} data-validation-error={row.fileIssue || row.status === 'error' ? 'true' : undefined} key={row.id} className={clsx('scroll-mt-24 rounded-md border bg-white p-2', row.status === 'error' ? 'border-rose-300' : row.status === 'success' ? 'border-emerald-300' : 'border-line')}>
                  <div className="evidence-upload-row">
                    <span className="evidence-upload-index pt-2 text-center text-sm font-semibold text-slate-500">{index + 1}</span>
                    <div className="min-w-0">
                      <span className="evidence-upload-field-label text-xs font-semibold text-slate-600">Tệp {index + 1}</span>
                      <input
                        ref={(node) => {
                          if (node) fileInputRefs.current.set(row.id, node);
                          else fileInputRefs.current.delete(row.id);
                          if (index === 0) fileInputRef.current = node;
                        }}
                        type="file"
                        aria-label={`Tệp ${index + 1}`}
                        name={row.id}
                        aria-invalid={Boolean(row.fileIssue || row.status === 'error')}
                        accept={evidenceFileAccept}
                        disabled={batchUploading || row.status === 'success'}
                        className={clsx('block w-full rounded-md border bg-white px-2 py-1.5 text-sm file:mr-2 file:rounded-md file:border-0 file:bg-blue-50 file:px-2 file:py-1 file:text-xs file:font-semibold file:text-blue-700 hover:file:bg-blue-100 disabled:bg-slate-100', row.fileIssue ? 'border-rose-400' : 'border-line')}
                        onChange={(event) => onFileChange(row.id, event.target.files?.[0] ?? null, event.currentTarget)}
                      />
                      {row.file ? <span className="mt-1 block break-all text-[11px] text-slate-500">{row.file.name} · {formatEvidenceFileSize(row.file.size)}</span> : null}
                      {row.file && onPreview ? <button type="button" className="mt-1 text-xs font-semibold text-blue-700 hover:underline" onClick={() => onPreview(row)} aria-label={`Xem tệp ${row.name || row.file?.name}`}>Xem</button> : null}
                      {row.fileIssue ? <span className="mt-1 block text-xs font-medium text-rose-700">{row.fileIssue}</span> : null}
                    </div>
                    <div>
                      <span className="evidence-upload-field-label text-xs font-semibold text-slate-600">Loại tệp</span>
                      <div className="flex min-h-[34px] items-center rounded-md border border-line bg-slate-100 px-2 py-1.5 text-sm font-semibold text-slate-700" aria-live="polite">
                        {row.classification?.label ?? '—'}
                      </div>
                    </div>
                    {extraColumn ? extraColumn.render(row, index) : null}
                    <label>
                      <span className="evidence-upload-field-label text-xs font-semibold text-slate-600">Tên minh chứng</span>
                      <input
                        className="w-full rounded-md border border-line bg-white px-2 py-1.5 text-sm outline-none focus:ring-2 focus:ring-blue-500 disabled:bg-slate-100"
                        aria-label={`Tên minh chứng ${index + 1}`}
                        value={row.name}
                        disabled={batchUploading || rowBusy || row.status === 'success'}
                        onChange={(event) => updateUploadRow(row.id, (current) => ({ ...current, name: event.target.value }))}
                        placeholder="Tên file / hạng mục"
                      />
                    </label>
                    <label>
                      <span className="evidence-upload-field-label text-xs font-semibold text-slate-600">Ghi chú</span>
                      <input
                        className="w-full rounded-md border border-line bg-white px-2 py-1.5 text-sm outline-none focus:ring-2 focus:ring-blue-500 disabled:bg-slate-100"
                        aria-label={`Ghi chú ${index + 1}`}
                        value={row.caption}
                        disabled={batchUploading || rowBusy || row.status === 'success'}
                        onChange={(event) => updateUploadRow(row.id, (current) => ({ ...current, caption: event.target.value }))}
                        placeholder="Ghi chú hiện trường"
                      />
                    </label>
                    <div className="min-w-0">
                      <div className="flex items-center justify-between gap-1">
                        <UploadStatus status={row.status} progress={row.progress} hasFile={Boolean(row.file)} />
                        <div className="flex shrink-0 items-center gap-1">
                          {row.status === 'error' && (uploadEvidenceRows || retryUploadRow) ? <button
                            type="button"
                            className="rounded-md p-1.5 text-blue-700 hover:bg-blue-50 disabled:opacity-50"
                            disabled={batchUploading}
                            onClick={() => retryUploadRow ? retryUploadRow(row.id) : void uploadEvidenceRows?.([row.id])}
                            aria-label={`Thử lại tệp ${index + 1}`}
                            title="Thử lại"
                          ><RotateCcw className="h-3.5 w-3.5" /></button> : null}
                          <button
                            type="button"
                            className="rounded-md p-1.5 text-slate-500 hover:bg-rose-50 hover:text-rose-700 disabled:opacity-40"
                            disabled={batchUploading || rowBusy || (canRemoveRow ? !canRemoveRow(row) : false)}
                            onClick={() => removeUploadRow(row.id)}
                            aria-label={`Bỏ dòng tệp ${index + 1}`}
                            title="Bỏ dòng"
                          ><Trash2 className="h-3.5 w-3.5" /></button>
                        </div>
                      </div>
                      {row.file ? <ProgressBar progress={row.progress} tone={row.status === 'error' ? 'error' : row.status === 'success' ? 'success' : 'default'} label={`Tiến độ tệp ${index + 1}`} /> : null}
                    </div>
                  </div>
                  {photoMismatch ? <p className="mt-2 rounded-md border border-amber-200 bg-amber-50 px-2 py-1.5 text-xs font-medium text-amber-800">{row.classification?.label} không hoàn tất điều kiện Hình ảnh. Hãy chọn thêm PNG, JPG, JPEG, WebP hoặc GIF.</p> : null}
                  {row.error ? <p className="mt-2 text-xs font-medium text-rose-700">{row.error}</p> : null}
                </article>
              );
            })}
          </div>

          <div className="mt-3 flex flex-col gap-3 border-t border-blue-100 pt-3 lg:flex-row lg:items-end lg:justify-between">
            <div className="min-w-0 flex-1">
              <div className="flex flex-wrap items-center justify-between gap-2 text-xs text-slate-600">
                <span>{selectedRows.length}/{maxFiles} tệp · tối đa 10 MB/tệp · {evidenceUploadConcurrency} tệp đồng thời</span>
                {selectedRows.length ? <span className="font-semibold text-slate-700">Tổng tiến độ {overallProgress}%</span> : null}
              </div>
              {selectedRows.length ? <ProgressBar progress={overallProgress} label="Tổng tiến độ tải minh chứng" /> : null}
              <p className="mt-1 text-[11px] text-slate-500">{evidenceSupportedFileHint}</p>
            </div>
            <div className="flex flex-wrap items-center justify-end gap-2">
              {actions}
              <button
                type="button"
                className="inline-flex items-center gap-2 rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100 disabled:cursor-not-allowed disabled:opacity-50"
                disabled={batchUploading || addDisabled || selectedRows.length >= maxFiles}
                onClick={addUploadRow}
              >
                <Plus className="h-4 w-4" />
                Thêm
              </button>
              {uploadEvidenceRows ? <button
                type="button"
                onClick={() => void uploadEvidenceRows()}
                className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60"
                disabled={batchUploading || pendingUploadCount === 0}
              >
                {batchUploading ? <Loader2 className="h-4 w-4 animate-spin" /> : <Upload className="h-4 w-4" />}
                {batchUploading ? 'Đang tải…' : `Tải lên ${pendingUploadCount || ''}`.trim()}
              </button> : null}
            </div>
          </div>
  </div>;
}

function UploadStatus({ status, progress, hasFile }: { status: EvidenceUploadStatus; progress: number; hasFile: boolean }) {
  const label = !hasFile
    ? 'Chưa chọn'
    : status === 'uploading'
      ? `${progress}%`
      : status === 'processing'
        ? '100% · Đang xử lý'
        : status === 'success'
          ? '100%'
          : status === 'error'
            ? 'Lỗi'
            : 'Chờ tải';
  return <span className={clsx(
    'truncate text-xs font-semibold',
    status === 'success' ? 'text-emerald-700' : status === 'error' ? 'text-rose-700' : status === 'processing' ? 'text-blue-700' : 'text-slate-600',
  )}>{label}</span>;
}

function ProgressBar({ progress, label, tone = 'default' }: { progress: number; label: string; tone?: 'default' | 'success' | 'error' }) {
  const value = Math.min(100, Math.max(0, progress));
  return (
    <div
      className="mt-1 h-1.5 overflow-hidden rounded-full bg-slate-200"
      role="progressbar"
      aria-label={label}
      aria-valuemin={0}
      aria-valuemax={100}
      aria-valuenow={value}
    >
      <div
        className={clsx('h-full rounded-full transition-[width] duration-150', tone === 'success' ? 'bg-emerald-500' : tone === 'error' ? 'bg-rose-500' : 'bg-blue-600')}
        style={{ width: `${value}%` }}
      />
    </div>
  );
}
