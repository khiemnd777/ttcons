import { useEffect, useState, type Ref } from 'react';
import { CheckCircle2, ChevronRight, ExternalLink, Loader2, XCircle } from 'lucide-react';
import { StatusBadge } from '../../components/StatusBadge';
import { identityReferenceText } from '../../components/IdentityReferenceLabel';
import { downloadBlob } from '../../lib/api/client';
import type { EvidenceRecord } from '../../lib/types/domain';
import { isProtectedStorageProvider } from '../../lib/utils/protectedStorage';
import { compactDateTime, evidenceLabel, visibilityLabel } from '../work-items/workItemPresentation';
import {
  evidenceMediaType,
  isImageEvidence,
  isInlineSpreadsheetEvidence,
  isInlinePdfEvidence,
  isInlineWordEvidence,
  isLegacySpreadsheetEvidence,
  isLegacyWordEvidence,
  originalFilename,
  type EvidenceMediaType,
} from './evidenceMedia';
import { classifyEvidenceFile } from './evidenceFileTypes';
import { evidenceReferenceMediaType, safeEvidenceFileUrl } from './evidenceReferenceMedia';
import { EvidenceViewerDialog } from './EvidenceViewerDialog';

export {
  isImageEvidence,
  isInlinePdfEvidence,
  isInlineSpreadsheetEvidence,
  isInlineWordEvidence,
  isPdfEvidence,
  originalFilename,
} from './evidenceMedia';

export type EvidenceReviewAction = 'accept' | 'reject';

type EvidenceRecordViewerDialogProps = {
  record: EvidenceRecord | null;
  previewUrl?: string;
  previewLoading?: boolean;
  previewError?: string;
  protectedFilePath?: string;
  protectedAccessDescription?: string;
  showWorkItem?: boolean;
  canReview?: boolean;
  reviewingAction?: EvidenceReviewAction | null;
  reviewComment?: string;
  reviewCommentInvalid?: boolean;
  reviewCommentRef?: Ref<HTMLTextAreaElement>;
  onReviewCommentChange?: (value: string) => void;
  onOpenFile?: () => void;
  onRetry?: () => void;
  onClose: () => void;
  onReview?: (action: EvidenceReviewAction) => void;
};

export function EvidenceRecordViewerDialog({
  record,
  previewUrl,
  previewLoading,
  previewError,
  protectedFilePath,
  protectedAccessDescription,
  showWorkItem = false,
  canReview = false,
  reviewingAction = null,
  reviewComment = '',
  reviewCommentInvalid = false,
  reviewCommentRef,
  onReviewCommentChange,
  onOpenFile,
  onRetry,
  onClose,
  onReview,
}: EvidenceRecordViewerDialogProps) {
  const inlineMediaType = record ? inlineEvidenceMediaType(record) : undefined;
  const ownsPhotoPreview = previewUrl === undefined && previewLoading === undefined && previewError === undefined;
  const protectedMediaType = record && (protectedFilePath || (isProtectedStorageProvider(record.storage_provider) && record.storage_key))
    ? inlineMediaType ?? (ownsPhotoPreview ? evidenceMediaType(record) : undefined)
    : undefined;
  const protectedPreview = useProtectedEvidencePreview(record, protectedMediaType, protectedFilePath);
  if (!record) return null;
  const reviewable = canReview
    && Boolean(onReview)
    && Boolean(onReviewCommentChange)
    && ['draft', 'submitted', 'pending'].includes(String(record.status));
  const detectedMediaType = evidenceMediaType(record);
  const imageEvidence = detectedMediaType === 'photo';
  const inlinePdf = inlineMediaType === 'pdf';
  const inlineOffice = inlineMediaType === 'word' || inlineMediaType === 'spreadsheet';
  const protectedFile = Boolean(protectedFilePath) || (isProtectedStorageProvider(record.storage_provider) && Boolean(record.storage_key));
  const openableDocument = !imageEvidence && Boolean(onOpenFile) && (protectedFile || Boolean(record.file_url));
  const openActionLabel = inlinePdf ? 'Mở tab' : inlineOffice ? 'Mở tệp gốc' : 'Mở tệp';
  const mediaAction = protectedMediaType && protectedPreview.previewUrl ? (
    <a
      href={protectedPreview.previewUrl}
      target="_blank"
      rel="noopener noreferrer"
      className="inline-flex items-center gap-2 rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-blue-700 hover:border-blue-300 hover:bg-blue-50"
    >
      <ExternalLink className="h-4 w-4" />
      {openActionLabel}
    </a>
  ) : !protectedFile && safeEvidenceFileUrl(record.file_url) ? <a href={safeEvidenceFileUrl(record.file_url)} target="_blank" rel="noopener noreferrer" className="text-sm font-semibold text-blue-700">Mở tệp gốc</a> : openableDocument ? (
    <button
      type="button"
      className={inlineMediaType
        ? 'inline-flex items-center gap-2 rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-blue-700 hover:border-blue-300 hover:bg-blue-50'
        : 'inline-flex items-center gap-2 rounded-md bg-blue-700 px-4 py-2.5 text-sm font-semibold text-white shadow-sm hover:bg-blue-800'}
      onClick={onOpenFile}
    >
      <ExternalLink className="h-4 w-4" />
      {openActionLabel}
    </button>
  ) : undefined;
  const legacyOfficeMessage = isLegacyWordEvidence(record)
    ? 'Định dạng Word đời cũ (.doc) chưa hỗ trợ đọc trực tiếp. Mở tệp gốc bằng ứng dụng tương thích.'
    : isLegacySpreadsheetEvidence(record)
      ? 'Định dạng Excel đời cũ (.xls) chưa hỗ trợ đọc trực tiếp. Mở tệp gốc bằng ứng dụng tương thích.'
      : undefined;

  return (
    <EvidenceViewerDialog
      open
      name={evidenceRecordTitle(record)}
      previewUrl={protectedMediaType ? protectedPreview.previewUrl : previewUrl ?? (imageEvidence ? safeEvidenceFileUrl(record.file_url) : undefined)}
      fileBlob={inlineOffice ? protectedPreview.blob : undefined}
      mediaType={protectedMediaType ?? inlineMediaType ?? (imageEvidence ? 'photo' : 'document')}
      loading={protectedMediaType ? protectedPreview.loading : previewLoading}
      error={protectedMediaType ? protectedPreview.error : previewError}
      caption={record.caption}
      metadata={[evidenceMediaLabel(record, detectedMediaType), record.mime_type ?? '', record.file_size ? formatEvidenceFileSize(record.file_size) : ''].filter(Boolean)}
      headerStatus={<StatusBadge status={record.status} />}
      mediaAction={mediaAction}
      documentUnavailableText={legacyOfficeMessage}
      onRetry={protectedMediaType ? protectedPreview.retry : onRetry}
      onClose={onClose}
      footer={(
        <div className="flex flex-wrap items-center justify-between gap-2">
          <p className="text-xs text-slate-500">
            {protectedFile ? (protectedAccessDescription ?? 'Tệp được bảo vệ được mở qua API có kiểm tra quyền đầu việc.') : 'Thông tin minh chứng được hiển thị theo quyền truy cập hiện tại.'}
          </p>
          <div className="flex flex-wrap justify-end gap-2">
            <CloseButton onClose={onClose} />
            {reviewable ? (
              <>
                <button
                  type="button"
                  className="inline-flex items-center gap-2 rounded-md border border-rose-200 bg-white px-3 py-2 text-sm font-semibold text-rose-700 hover:bg-rose-50 disabled:cursor-not-allowed disabled:opacity-60"
                  disabled={Boolean(reviewingAction)}
                  onClick={() => onReview?.('reject')}
                >
                  {reviewingAction === 'reject' ? <Loader2 className="h-4 w-4 animate-spin" /> : <XCircle className="h-4 w-4" />}
                  Từ chối
                </button>
                <button
                  type="button"
                  className="inline-flex items-center gap-2 rounded-md border border-emerald-700 bg-emerald-700 px-3 py-2 text-sm font-semibold text-white hover:bg-emerald-800 disabled:cursor-not-allowed disabled:opacity-60"
                  disabled={Boolean(reviewingAction)}
                  onClick={() => onReview?.('accept')}
                >
                  {reviewingAction === 'accept' ? <Loader2 className="h-4 w-4 animate-spin" /> : <CheckCircle2 className="h-4 w-4" />}
                  Chấp nhận
                </button>
              </>
            ) : null}
          </div>
        </div>
      )}
      details={(
        <div className="min-w-0 space-y-5">
          {showWorkItem ? (
            <section aria-labelledby="evidence-source-heading">
              <h3 id="evidence-source-heading" className="text-xs font-semibold uppercase tracking-[0.12em] text-slate-500">Nguồn hồ sơ</h3>
              <div className="mt-3 divide-y divide-line border-y border-line">
                {record.design_release_id ? <EvidenceInfoRow label="Revision thiết kế" value={`${record.design_revision_label || record.revision_no || '-'} · ${designReleaseStatusText(record.design_release_status)}`} /> : null}
                {record.design_change_reason ? <EvidenceInfoRow label="Lý do revision" value={record.design_change_reason} /> : null}
                <EvidenceInfoRow label="Giai đoạn" value={record.run_node_name ?? '-'} />
                <EvidenceInfoRow label="Đầu việc" value={record.work_item_title ?? '-'} />
              </div>
            </section>
          ) : null}

          <section aria-labelledby="evidence-review-heading">
            <h3 id="evidence-review-heading" className="text-xs font-semibold uppercase tracking-[0.12em] text-slate-500">Gửi và rà soát</h3>
            <div className="mt-3 grid grid-cols-2 gap-x-4 gap-y-4 border-y border-line py-3">
              <EvidenceInfoItem label="Người gửi" value={identityReferenceText(record.submitted_by_name ?? record.uploaded_by, record.submitted_by_account_deleted, '-')} />
              <EvidenceInfoItem label="Thời điểm gửi" value={compactDateTime(record.created_at)} />
              <EvidenceInfoItem label="Người rà soát" value={identityReferenceText(record.reviewed_by_name ?? record.reviewed_by, record.reviewed_by_account_deleted, '-')} />
              <EvidenceInfoItem label="Thời điểm rà soát" value={compactDateTime(record.reviewed_at)} />
            </div>
          </section>

          <details className="group overflow-hidden rounded-md border border-line bg-white">
            <summary className="flex cursor-pointer list-none items-center justify-between gap-3 px-3 py-3 marker:hidden hover:bg-slate-50">
              <span className="min-w-0">
                <span className="block text-sm font-semibold text-ink">Chi tiết kỹ thuật</span>
                <span className="mt-0.5 block text-xs leading-5 text-slate-500">Phạm vi, nơi lưu trữ, tên tệp gốc, SHA-256</span>
              </span>
              <ChevronRight className="h-4 w-4 shrink-0 text-slate-500 transition-transform group-open:rotate-90" />
            </summary>
            <div className="space-y-4 border-t border-line bg-slate-50/70 p-3">
              <div className="grid grid-cols-2 gap-4">
                <EvidenceInfoItem label="Loại" value={evidenceLabel(record.evidence_type)} />
                <EvidenceInfoItem label="Phạm vi hiển thị" value={visibilityLabel(record.visibility ?? 'project')} />
                <EvidenceInfoItem label="Nơi lưu trữ" value={protectedFile ? 'Tệp nội bộ được bảo vệ' : (record.file_url ? 'Liên kết ngoài' : '-')} />
                <EvidenceInfoItem label="MIME / Dung lượng" value={[record.mime_type, record.file_size ? formatEvidenceFileSize(record.file_size) : undefined].filter(Boolean).join(' · ') || '-'} />
              </div>
              <EvidenceInfoItem label="Tên tệp gốc" value={originalFilename(record) || record.document_name || '-'} />
              <div>
                <p className="text-[11px] font-semibold uppercase tracking-wide text-slate-500">Mã kiểm tra SHA-256</p>
                <p className="mt-1 break-all font-mono text-xs leading-5 text-slate-700">{record.checksum_sha256 || '-'}</p>
              </div>
            </div>
          </details>

          {reviewable ? (
            <label data-validation-key="review_comment" data-validation-error={reviewCommentInvalid ? 'true' : undefined} className="block scroll-mt-24 rounded-md border border-amber-200 bg-amber-50 p-3">
              <span className="block text-sm font-semibold text-amber-900">Rà soát minh chứng</span>
              <span className="mt-1 block text-xs leading-5 text-amber-800">Nhập lý do khi từ chối; ghi chú là tùy chọn khi chấp nhận.</span>
              <textarea
                ref={reviewCommentRef}
                aria-invalid={reviewCommentInvalid}
                className="mt-2 w-full rounded-md border border-amber-200 bg-white px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-amber-200"
                rows={3}
                value={reviewComment}
                onChange={(event) => onReviewCommentChange?.(event.target.value)}
                placeholder="Ghi chú rà soát"
                disabled={Boolean(reviewingAction)}
              />
            </label>
          ) : null}
        </div>
      )}
    />
  );
}

function designReleaseStatusText(status?: string) {
  if (status === 'draft') return 'Nháp';
  if (status === 'released') return 'Hiện hành';
  if (status === 'superseded') return 'Đã thay thế';
  return status || '-';
}

type EvidenceDraftViewerDialogProps = {
  open: boolean;
  name: string;
  originalName: string;
  previewUrl?: string;
  file?: File;
  mimeType?: string;
  fileSize: number;
  caption?: string;
  onClose: () => void;
};

export function EvidenceDraftViewerDialog({
  open,
  name,
  originalName,
  file,
  previewUrl,
  mimeType,
  fileSize,
  caption,
  onClose,
}: EvidenceDraftViewerDialogProps) {
  const [localPreview, setLocalPreview] = useState<{ file: File; url: string } | null>(null);
  useEffect(() => {
    if (!open || !file) { setLocalPreview(null); return; }
    const url = URL.createObjectURL(file);
    setLocalPreview({ file, url });
    return () => URL.revokeObjectURL(url);
  }, [open, file]);
  const mediaType = evidenceReferenceMediaType(originalName, file?.type || mimeType);
  const label = classifyEvidenceFile({ name: originalName, type: file?.type || mimeType || '' })?.label ?? 'Tài liệu';
  const url = localPreview?.file === file ? localPreview?.url : undefined;
  return (
    <EvidenceViewerDialog
      open={open}
      name={name}
      description="Thông tin tệp đang chờ upload."
      previewUrl={url ?? previewUrl}
      fileBlob={file}
      mediaType={mediaType}
      mediaAction={url ? <a href={url} target="_blank" rel="noopener noreferrer" className="text-sm font-semibold text-blue-700">Mở tệp gốc</a> : undefined}
      documentUnavailableText="Định dạng Office đời cũ chưa hỗ trợ đọc trực tiếp. Mở tệp gốc bằng ứng dụng tương thích."
      caption={caption}
      metadata={[label, mimeType ?? '', formatEvidenceFileSize(fileSize)].filter(Boolean)}
      headerStatus={<span className="rounded-full border border-amber-200 bg-amber-50 px-2.5 py-1 text-xs font-semibold text-amber-700">Chưa upload</span>}
      onClose={onClose}
      footer={(
        <div className="flex flex-wrap items-center justify-between gap-2">
          <p className="text-xs text-slate-500">Thông tin người gửi, nơi lưu trữ và mã kiểm tra sẽ có sau khi upload.</p>
          <CloseButton onClose={onClose} />
        </div>
      )}
      details={(
        <div className="min-w-0 space-y-4">
          <section aria-labelledby="draft-evidence-heading">
            <h3 id="draft-evidence-heading" className="text-xs font-semibold uppercase tracking-[0.12em] text-slate-500">Thông tin tệp</h3>
            <div className="mt-3 grid grid-cols-2 gap-4 border-y border-line py-3">
              <EvidenceInfoItem label="Loại" value={label} />
              <EvidenceInfoItem label="Trạng thái tệp" value="Chưa upload" />
              <EvidenceInfoItem label="MIME / Dung lượng" value={[mimeType, formatEvidenceFileSize(fileSize)].filter(Boolean).join(' · ')} />
              <EvidenceInfoItem label="Tên tệp gốc" value={originalName} />
            </div>
          </section>
          <div className="rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-xs leading-5 text-amber-800">
            Tệp đang nằm trong hàng đợi cục bộ và chưa có dữ liệu lưu trữ, người gửi hoặc mã kiểm tra.
          </div>
        </div>
      )}
    />
  );
}

// Missing metadata and legacy external references use the same dialog layout,
// without inventing persisted IDs, status, people or timestamps.
export function EvidenceReferenceStateDialog({ name, fileUrl, mimeType, evidenceType, loading, error, onRetry, onClose }: { name: string; fileUrl?: string; mimeType?: string; evidenceType?: string; loading: boolean; error?: string; onRetry?: () => void; onClose: () => void }) {
  const url = safeEvidenceFileUrl(fileUrl);
  const mediaType = evidenceReferenceMediaType(name, mimeType, evidenceType);
  return <EvidenceViewerDialog open name={name} loading={loading} error={error} onRetry={onRetry} onClose={onClose}
    mediaType={mediaType === 'photo' && url ? 'photo' : 'document'} previewUrl={mediaType === 'photo' ? url : undefined}
    mediaAction={url ? <a href={url} target="_blank" rel="noopener noreferrer" className="text-sm font-semibold text-blue-700">Mở tệp gốc</a> : undefined}
    details={<section className="space-y-3"><h3 className="text-xs font-semibold uppercase text-slate-500">Nguồn hồ sơ</h3><p role={error ? 'alert' : 'status'} className="text-sm text-slate-600">{loading ? 'Đang tải thông tin minh chứng…' : error || (url ? 'Liên kết ngoài được lưu trong dữ liệu lịch sử. Không có hồ sơ tải lên để hiển thị người gửi và rà soát.' : 'Không có tệp hoặc thông tin minh chứng.')}</p>{error && onRetry ? <button type="button" onClick={onRetry} className="text-sm font-semibold text-blue-700">Thử lại</button> : null}</section>}
  />;
}

function CloseButton({ onClose }: { onClose: () => void }) {
  return <button type="button" className="rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100" onClick={onClose}>Đóng</button>;
}

// Document sources share the existing readers/layout without inventing evidence
// IDs, review state or upload metadata. The caller pins the protected source path.
export function ProtectedDocumentViewerDialog({ name, protectedFilePath, mimeType, fileSize, sourceLabel, uploadedBy, createdAt, onClose }: {
  name: string;
  protectedFilePath: string;
  mimeType?: string;
  fileSize?: number;
  sourceLabel?: string;
  uploadedBy?: string;
  createdAt?: string;
  onClose: () => void;
}) {
  const mediaType = evidenceReferenceMediaType(name, mimeType);
  const preview = useProtectedFilePreview(protectedFilePath, mediaType);
  const label = classifyEvidenceFile({ name, type: mimeType ?? '' })?.label ?? 'Tài liệu';
  return <EvidenceViewerDialog open title="Xem tệp" name={name} description={sourceLabel}
    mediaType={mediaType} previewUrl={preview.previewUrl} fileBlob={preview.blob}
    loading={preview.loading} error={preview.error} onRetry={preview.retry} onClose={onClose}
    metadata={[label, mimeType ?? '', fileSize ? formatEvidenceFileSize(fileSize) : ''].filter(Boolean)}
    mediaAction={preview.previewUrl ? <a href={preview.previewUrl} target="_blank" rel="noopener noreferrer" className="text-sm font-semibold text-blue-700">Mở tệp gốc</a> : undefined}
    documentUnavailableText="Định dạng này chưa hỗ trợ đọc trực tiếp. Mở tệp gốc bằng ứng dụng tương thích."
    details={<section className="min-w-0 space-y-4">
      <h3 className="text-xs font-semibold uppercase tracking-wide text-slate-500">Thông tin tệp</h3>
      <EvidenceInfoItem label="Tên tệp" value={name} />
      {sourceLabel ? <EvidenceInfoItem label="Nguồn hồ sơ" value={sourceLabel} /> : null}
      <EvidenceInfoItem label="Loại" value={label} />
      {fileSize ? <EvidenceInfoItem label="Dung lượng" value={formatEvidenceFileSize(fileSize)} /> : null}
      {uploadedBy ? <EvidenceInfoItem label="Người tải lên" value={uploadedBy} /> : null}
      {createdAt ? <EvidenceInfoItem label="Thời điểm tải lên" value={compactDateTime(createdAt)} /> : null}
    </section>}
    footer={<div className="flex justify-end"><CloseButton onClose={onClose} /></div>}
  />;
}

function EvidenceInfoRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="grid gap-1 py-3 sm:grid-cols-[7rem_minmax(0,1fr)] sm:gap-3">
      <p className="text-xs font-medium text-slate-500">{label}</p>
      <p className="break-words text-sm text-ink">{value}</p>
    </div>
  );
}

function EvidenceInfoItem({ label, value }: { label: string; value: string }) {
  return (
    <div className="min-w-0">
      <p className="text-[11px] font-semibold uppercase tracking-wide text-slate-500">{label}</p>
      <p className="mt-1 break-words text-sm leading-5 text-ink">{value}</p>
    </div>
  );
}

export function evidenceRecordTitle(record: EvidenceRecord) {
  return record.name ?? record.title ?? record.document_name ?? record.id;
}

export function formatEvidenceFileSize(value: number) {
  if (!Number.isFinite(value) || value <= 0) return '-';
  if (value < 1024) return `${value} B`;
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB`;
  return `${(value / 1024 / 1024).toFixed(1)} MB`;
}

export function useProtectedEvidencePreview(record: EvidenceRecord | null, mediaType?: EvidenceMediaType, protectedFilePath?: string) {
  // Server-supplied revision paths identify authorized frozen files whose public
  // metadata deliberately excludes the opaque storage key.
  const eligible = Boolean(record?.id && (protectedFilePath || record.storage_key) && mediaType);
  const path = eligible ? protectedFilePath || `/evidence/${record!.id}/file` : undefined;
  return useProtectedFilePreview(path, mediaType, JSON.stringify([record?.id, record?.storage_key, record?.updated_at]));
}

function useProtectedFilePreview(filePath?: string, mediaType?: EvidenceMediaType, identity?: string) {
  const [retryKey, setRetryKey] = useState(0);
  const [state, setState] = useState<{
    requestKey?: string;
    mediaType?: EvidenceMediaType;
    blob?: Blob;
    previewUrl?: string;
    loading: boolean;
    error?: string;
  }>({ loading: false });
  const requestKey = JSON.stringify([filePath, mediaType, identity]);

  useEffect(() => {
    if (!filePath || !mediaType) {
      setState({ loading: false });
      return undefined;
    }

    let cancelled = false;
    let objectUrl = '';
    setState({ requestKey, mediaType, loading: true });

    async function loadEvidenceFile() {
      try {
        const blob = await downloadBlob(filePath!);
        if (cancelled) return;
        const normalizedBlob = normalizeEvidenceBlob(blob, mediaType);
        objectUrl = URL.createObjectURL(normalizedBlob);
        setState({ requestKey, mediaType, blob: normalizedBlob, previewUrl: objectUrl, loading: false });
      } catch (error) {
        if (cancelled) return;
        const message = error instanceof Error ? error.message : 'Vui lòng thử lại.';
        const label = mediaType === 'pdf' ? 'PDF' : mediaType === 'word' ? 'Word' : mediaType === 'spreadsheet' ? 'Excel' : 'minh chứng';
        setState({ requestKey, mediaType, loading: false, error: `Không tải được ${label}. ${message}` });
      }
    }

    void loadEvidenceFile();
    return () => {
      cancelled = true;
      if (objectUrl) URL.revokeObjectURL(objectUrl);
    };
  }, [requestKey, retryKey]);

  const currentState = state.requestKey === requestKey ? state : { loading: Boolean(filePath && mediaType) };
  return {
    ...currentState,
    retry: () => setRetryKey((current) => current + 1),
  };
}

function inlineEvidenceMediaType(record: EvidenceRecord): 'pdf' | 'word' | 'spreadsheet' | undefined {
  if (isInlinePdfEvidence(record)) return 'pdf';
  if (isInlineWordEvidence(record)) return 'word';
  if (isInlineSpreadsheetEvidence(record)) return 'spreadsheet';
  return undefined;
}

function evidenceMediaLabel(record: EvidenceRecord, mediaType: EvidenceMediaType) {
  if (mediaType === 'pdf') return 'PDF';
  if (mediaType === 'word' || isLegacyWordEvidence(record)) return 'Word';
  if (mediaType === 'spreadsheet' || isLegacySpreadsheetEvidence(record)) return 'Excel';
  return evidenceLabel(record.evidence_type);
}

function normalizeEvidenceBlob(blob: Blob, mediaType?: EvidenceMediaType) {
  const expectedMimeType = mediaType === 'pdf'
    ? 'application/pdf'
    : mediaType === 'word'
      ? 'application/vnd.openxmlformats-officedocument.wordprocessingml.document'
      : mediaType === 'spreadsheet'
        ? 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet'
        : blob.type;
  return blob.type.toLowerCase() === expectedMimeType ? blob : blob.slice(0, blob.size, expectedMimeType);
}
