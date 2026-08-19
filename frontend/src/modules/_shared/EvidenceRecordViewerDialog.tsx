import { useEffect, useState } from 'react';
import { CheckCircle2, ChevronRight, ExternalLink, Loader2, XCircle } from 'lucide-react';
import { StatusBadge } from '../../components/StatusBadge';
import { downloadBlob } from '../../lib/api/client';
import type { EvidenceRecord } from '../../lib/types/domain';
import { isProtectedStorageProvider } from '../../lib/utils/protectedStorage';
import { compactDateTime, evidenceLabel, visibilityLabel } from '../work-items/workItemPresentation';
import {
  evidenceMediaType,
  isImageEvidence,
  isInlinePdfEvidence,
  originalFilename,
} from './evidenceMedia';
import { EvidenceViewerDialog } from './EvidenceViewerDialog';

export { isImageEvidence, isInlinePdfEvidence, isPdfEvidence, originalFilename } from './evidenceMedia';

export type EvidenceReviewAction = 'accept' | 'reject';

type EvidenceRecordViewerDialogProps = {
  record: EvidenceRecord | null;
  previewUrl?: string;
  previewLoading?: boolean;
  previewError?: string;
  showWorkItem?: boolean;
  canReview?: boolean;
  reviewingAction?: EvidenceReviewAction | null;
  reviewComment?: string;
  onReviewCommentChange?: (value: string) => void;
  onOpenFile?: () => void;
  onRetry?: () => void;
  onClose: () => void;
  onReview?: (action: EvidenceReviewAction) => void;
};

export function EvidenceRecordViewerDialog({
  record,
  previewUrl,
  previewLoading = false,
  previewError,
  showWorkItem = false,
  canReview = false,
  reviewingAction = null,
  reviewComment = '',
  onReviewCommentChange,
  onOpenFile,
  onRetry,
  onClose,
  onReview,
}: EvidenceRecordViewerDialogProps) {
  const pdfPreview = useProtectedPdfPreview(record);
  if (!record) return null;
  const reviewable = canReview
    && Boolean(onReview)
    && Boolean(onReviewCommentChange)
    && ['draft', 'submitted', 'pending'].includes(String(record.status));
  const mediaType = evidenceMediaType(record);
  const imageEvidence = mediaType === 'photo';
  const inlinePdf = mediaType === 'pdf' && isInlinePdfEvidence(record);
  const protectedFile = isProtectedStorageProvider(record.storage_provider) && Boolean(record.storage_key);
  const openableDocument = !imageEvidence && Boolean(onOpenFile) && (protectedFile || Boolean(record.file_url));
  const mediaAction = inlinePdf && pdfPreview.previewUrl ? (
    <a
      href={pdfPreview.previewUrl}
      target="_blank"
      rel="noreferrer"
      className="inline-flex items-center gap-2 rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-blue-700 hover:border-blue-300 hover:bg-blue-50"
    >
      <ExternalLink className="h-4 w-4" />
      Mở tab
    </a>
  ) : openableDocument ? (
    <button
      type="button"
      className={inlinePdf
        ? 'inline-flex items-center gap-2 rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-blue-700 hover:border-blue-300 hover:bg-blue-50'
        : 'inline-flex items-center gap-2 rounded-md bg-blue-700 px-4 py-2.5 text-sm font-semibold text-white shadow-sm hover:bg-blue-800'}
      onClick={onOpenFile}
    >
      <ExternalLink className="h-4 w-4" />
      {inlinePdf ? 'Mở tab' : 'Mở tệp'}
    </button>
  ) : undefined;

  return (
    <EvidenceViewerDialog
      open
      name={evidenceRecordTitle(record)}
      previewUrl={inlinePdf ? pdfPreview.previewUrl : previewUrl}
      mediaType={inlinePdf ? 'pdf' : (imageEvidence ? 'photo' : 'document')}
      loading={inlinePdf ? pdfPreview.loading : previewLoading}
      error={inlinePdf ? pdfPreview.error : previewError}
      caption={record.caption}
      metadata={[evidenceLabel(record.evidence_type), record.mime_type ?? '', record.file_size ? formatEvidenceFileSize(record.file_size) : ''].filter(Boolean)}
      headerStatus={<StatusBadge status={record.status} />}
      mediaAction={mediaAction}
      onRetry={inlinePdf ? pdfPreview.retry : onRetry}
      onClose={onClose}
      footer={(
        <div className="flex flex-wrap items-center justify-between gap-2">
          <p className="text-xs text-slate-500">
            {protectedFile ? 'Tệp được bảo vệ được mở qua API có kiểm tra quyền đầu việc.' : 'Thông tin minh chứng được hiển thị theo quyền truy cập hiện tại.'}
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
                <EvidenceInfoRow label="Giai đoạn" value={record.run_node_name ?? '-'} />
                <EvidenceInfoRow label="Đầu việc" value={record.work_item_title ?? '-'} />
              </div>
            </section>
          ) : null}

          <section aria-labelledby="evidence-review-heading">
            <h3 id="evidence-review-heading" className="text-xs font-semibold uppercase tracking-[0.12em] text-slate-500">Gửi và rà soát</h3>
            <div className="mt-3 grid grid-cols-2 gap-x-4 gap-y-4 border-y border-line py-3">
              <EvidenceInfoItem label="Người gửi" value={record.submitted_by_name ?? record.uploaded_by ?? '-'} />
              <EvidenceInfoItem label="Thời điểm gửi" value={compactDateTime(record.created_at)} />
              <EvidenceInfoItem label="Người rà soát" value={record.reviewed_by_name ?? record.reviewed_by ?? '-'} />
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
            <label className="block rounded-md border border-amber-200 bg-amber-50 p-3">
              <span className="block text-sm font-semibold text-amber-900">Rà soát minh chứng</span>
              <span className="mt-1 block text-xs leading-5 text-amber-800">Nhập lý do khi từ chối; ghi chú là tùy chọn khi chấp nhận.</span>
              <textarea
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

type EvidenceDraftViewerDialogProps = {
  open: boolean;
  name: string;
  originalName: string;
  previewUrl?: string;
  mimeType?: string;
  fileSize: number;
  caption?: string;
  onClose: () => void;
};

export function EvidenceDraftViewerDialog({
  open,
  name,
  originalName,
  previewUrl,
  mimeType,
  fileSize,
  caption,
  onClose,
}: EvidenceDraftViewerDialogProps) {
  return (
    <EvidenceViewerDialog
      open={open}
      name={name}
      description="Thông tin tệp đang chờ upload."
      previewUrl={previewUrl}
      mediaType="photo"
      caption={caption}
      metadata={['Ảnh', mimeType ?? '', formatEvidenceFileSize(fileSize)].filter(Boolean)}
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
              <EvidenceInfoItem label="Loại" value="Ảnh" />
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

function CloseButton({ onClose }: { onClose: () => void }) {
  return <button type="button" className="rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100" onClick={onClose}>Đóng</button>;
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

function useProtectedPdfPreview(record: EvidenceRecord | null) {
  const [retryKey, setRetryKey] = useState(0);
  const [state, setState] = useState<{
    recordId?: string;
    previewUrl?: string;
    loading: boolean;
    error?: string;
  }>({ loading: false });
  const eligible = Boolean(record && isInlinePdfEvidence(record));
  const recordId = eligible ? record?.id : undefined;
  const storageKey = eligible ? record?.storage_key : undefined;
  const updatedAt = eligible ? record?.updated_at : undefined;

  useEffect(() => {
    if (!recordId || !storageKey) {
      setState({ loading: false });
      return undefined;
    }

    let cancelled = false;
    let objectUrl = '';
    setState({ recordId, loading: true });

    async function loadPdf() {
      try {
        const blob = await downloadBlob(`/evidence/${recordId}/file`);
        if (cancelled) return;
        const pdfBlob = blob.type.toLowerCase().includes('pdf')
          ? blob
          : blob.slice(0, blob.size, 'application/pdf');
        objectUrl = URL.createObjectURL(pdfBlob);
        setState({ recordId, previewUrl: objectUrl, loading: false });
      } catch (error) {
        if (cancelled) return;
        const message = error instanceof Error ? error.message : 'Vui lòng thử lại.';
        setState({ recordId, loading: false, error: `Không tải được PDF. ${message}` });
      }
    }

    void loadPdf();
    return () => {
      cancelled = true;
      if (objectUrl) URL.revokeObjectURL(objectUrl);
    };
  }, [recordId, retryKey, storageKey, updatedAt]);

  const currentState = state.recordId === recordId ? state : { loading: Boolean(recordId) };
  return {
    ...currentState,
    retry: () => setRetryKey((current) => current + 1),
  };
}
