import { CheckCircle2, ExternalLink, Loader2, XCircle } from 'lucide-react';
import { StatusBadge } from '../../components/StatusBadge';
import type { EvidenceRecord } from '../../lib/types/domain';
import { isProtectedStorageProvider } from '../../lib/utils/protectedStorage';
import { compactDateTime, evidenceLabel, visibilityLabel } from '../work-items/workItemPresentation';
import { EvidenceViewerDialog } from './EvidenceViewerDialog';

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
  if (!record) return null;
  const reviewable = canReview
    && Boolean(onReview)
    && Boolean(onReviewCommentChange)
    && ['draft', 'submitted', 'pending'].includes(String(record.status));
  const protectedFile = isProtectedStorageProvider(record.storage_provider) && Boolean(record.storage_key);

  return (
    <EvidenceViewerDialog
      open
      name={evidenceRecordTitle(record)}
      description="Thông tin tệp, nơi lưu trữ và trạng thái rà soát của minh chứng."
      previewUrl={previewUrl}
      mediaType={isImageEvidence(record) ? 'photo' : 'document'}
      loading={previewLoading}
      error={previewError}
      caption={record.caption}
      metadata={[evidenceLabel(record.evidence_type), record.mime_type ?? '', record.file_size ? formatEvidenceFileSize(record.file_size) : ''].filter(Boolean)}
      onRetry={onRetry}
      onClose={onClose}
      footer={(
        <div className="flex flex-wrap items-center justify-between gap-2">
          <p className="text-xs text-slate-500">
            {protectedFile ? 'Tệp được bảo vệ được mở qua API có kiểm tra quyền đầu việc.' : 'Thông tin minh chứng được hiển thị theo quyền truy cập hiện tại.'}
          </p>
          <div className="flex flex-wrap justify-end gap-2">
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
                  className="inline-flex items-center gap-2 rounded-md border border-emerald-200 bg-white px-3 py-2 text-sm font-semibold text-emerald-700 hover:bg-emerald-50 disabled:cursor-not-allowed disabled:opacity-60"
                  disabled={Boolean(reviewingAction)}
                  onClick={() => onReview?.('accept')}
                >
                  {reviewingAction === 'accept' ? <Loader2 className="h-4 w-4 animate-spin" /> : <CheckCircle2 className="h-4 w-4" />}
                  Duyệt
                </button>
              </>
            ) : null}
            {!isImageEvidence(record) && onOpenFile ? <button type="button" className="inline-flex items-center gap-2 rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-blue-700 hover:border-blue-300 hover:bg-blue-50" onClick={onOpenFile}><ExternalLink className="h-4 w-4" /> Mở tệp</button> : null}
            <CloseButton onClose={onClose} />
          </div>
        </div>
      )}
      details={(
        <div className="min-w-0 space-y-3">
          <div className="flex flex-wrap items-start justify-between gap-2">
            <div className="min-w-0">
              <p className="break-words text-sm font-semibold text-ink">{evidenceRecordTitle(record)}</p>
              <p className="mt-1 break-words text-xs text-slate-500">{record.document_name ?? originalFilename(record) ?? record.id}</p>
            </div>
            <StatusBadge status={record.status} />
          </div>
          <div className="grid gap-2">
            <EvidenceMeta label="Loại" value={evidenceLabel(record.evidence_type)} />
            <EvidenceMeta label="Phạm vi hiển thị" value={visibilityLabel(record.visibility ?? 'project')} />
            {showWorkItem ? <EvidenceMeta label="Đầu việc" value={record.work_item_title ?? '-'} /> : null}
            {showWorkItem && record.run_node_name ? <EvidenceMeta label="Giai đoạn" value={record.run_node_name} /> : null}
            <EvidenceMeta label="Người gửi" value={record.submitted_by_name ?? record.uploaded_by ?? '-'} />
            <EvidenceMeta label="Thời điểm gửi" value={compactDateTime(record.created_at)} />
            <EvidenceMeta label="Người rà soát" value={record.reviewed_by_name ?? record.reviewed_by ?? '-'} />
            <EvidenceMeta label="Thời điểm rà soát" value={compactDateTime(record.reviewed_at)} />
            <EvidenceMeta label="Nơi lưu trữ" value={protectedFile ? 'Tệp nội bộ được bảo vệ' : (record.file_url ? 'Liên kết ngoài' : '-')} />
            <EvidenceMeta label="MIME / Dung lượng" value={[record.mime_type, record.file_size ? formatEvidenceFileSize(record.file_size) : undefined].filter(Boolean).join(' · ') || '-'} />
            <EvidenceMeta label="Tên tệp gốc" value={originalFilename(record) ?? '-'} />
          </div>
          <div className="rounded-md border border-line bg-slate-50 p-3">
            <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">Mã kiểm tra SHA-256</p>
            <p className="mt-1 break-all font-mono text-xs leading-5 text-slate-700">{record.checksum_sha256 || '-'}</p>
          </div>
          {reviewable ? (
            <label className="block">
              <span className="text-xs font-semibold text-slate-600">Ghi chú rà soát</span>
              <textarea
                className="mt-1 w-full rounded-md border border-line bg-white px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-100"
                rows={3}
                value={reviewComment}
                onChange={(event) => onReviewCommentChange?.(event.target.value)}
                placeholder="Lý do duyệt/từ chối"
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
      onClose={onClose}
      footer={(
        <div className="flex flex-wrap items-center justify-between gap-2">
          <p className="text-xs text-slate-500">Thông tin người gửi, nơi lưu trữ và mã kiểm tra sẽ có sau khi upload.</p>
          <CloseButton onClose={onClose} />
        </div>
      )}
      details={(
        <div className="min-w-0 space-y-3">
          <div className="flex flex-wrap items-start justify-between gap-2">
            <div className="min-w-0">
              <p className="break-words text-sm font-semibold text-ink">{name}</p>
              <p className="mt-1 break-words text-xs text-slate-500">{originalName}</p>
            </div>
            <span className="rounded-full border border-amber-200 bg-amber-50 px-2.5 py-1 text-xs font-semibold text-amber-700">Chưa upload</span>
          </div>
          <div className="grid gap-2">
            <EvidenceMeta label="Loại" value="Ảnh" />
            <EvidenceMeta label="Trạng thái tệp" value="Chưa upload" />
            <EvidenceMeta label="MIME / Dung lượng" value={[mimeType, formatEvidenceFileSize(fileSize)].filter(Boolean).join(' · ')} />
            <EvidenceMeta label="Tên tệp gốc" value={originalName} />
          </div>
          <p className="rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-xs leading-5 text-amber-800">Tệp đang nằm trong hàng đợi cục bộ và chưa có dữ liệu lưu trữ, người gửi hoặc mã kiểm tra.</p>
        </div>
      )}
    />
  );
}

function CloseButton({ onClose }: { onClose: () => void }) {
  return <button type="button" className="rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100" onClick={onClose}>Đóng</button>;
}

function EvidenceMeta({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-md border border-line bg-white px-3 py-2">
      <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">{label}</p>
      <p className="mt-1 break-words text-sm text-ink">{value}</p>
    </div>
  );
}

export function isImageEvidence(record: EvidenceRecord) {
  return record.evidence_type === 'photo' || String(record.mime_type ?? '').startsWith('image/');
}

export function evidenceRecordTitle(record: EvidenceRecord) {
  return record.name ?? record.title ?? record.document_name ?? record.id;
}

export function originalFilename(record: EvidenceRecord) {
  const payload = record.payload_json;
  if (!payload || typeof payload !== 'object') return '';
  return String(payload.original_filename ?? '').trim();
}

export function formatEvidenceFileSize(value: number) {
  if (!Number.isFinite(value) || value <= 0) return '-';
  if (value < 1024) return `${value} B`;
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB`;
  return `${(value / 1024 / 1024).toFixed(1)} MB`;
}
