import { useEffect, useMemo, useState } from 'react';
import { CheckCircle2, ExternalLink, FileText, Image as ImageIcon, Loader2, Search, XCircle } from 'lucide-react';
import clsx from 'clsx';
import { Dialog } from '../../components/Dialog';
import { EmptyState } from '../../components/EmptyState';
import { StatusBadge } from '../../components/StatusBadge';
import { useToast } from '../../components/ToastProvider';
import { downloadBlob } from '../../lib/api/client';
import type { EvidenceRecord } from '../../lib/types/domain';
import { compactDateTime, evidenceLabel, visibilityLabel } from './workItemPresentation';

export type EvidenceReviewAction = 'accept' | 'reject';

type EvidenceFilter = 'all' | 'photo' | 'document' | 'submitted' | 'accepted' | 'rejected';

type EvidenceGalleryProps = {
  evidence: EvidenceRecord[];
  emptyTitle?: string;
  showWorkItem?: boolean;
  onReview?: (record: EvidenceRecord, action: EvidenceReviewAction, comment: string) => Promise<void>;
};

const filters: Array<{ id: EvidenceFilter; label: string }> = [
  { id: 'all', label: 'All' },
  { id: 'photo', label: 'Photo' },
  { id: 'document', label: 'Document' },
  { id: 'submitted', label: 'Submitted' },
  { id: 'accepted', label: 'Accepted' },
  { id: 'rejected', label: 'Rejected' },
];

export function EvidenceGallery({
  evidence,
  emptyTitle = 'Chưa có evidence',
  showWorkItem = false,
  onReview,
}: EvidenceGalleryProps) {
  const toast = useToast();
  const [activeFilter, setActiveFilter] = useState<EvidenceFilter>('all');
  const [query, setQuery] = useState('');
  const [selected, setSelected] = useState<EvidenceRecord | null>(null);
  const [reviewComment, setReviewComment] = useState('');
  const [reviewingAction, setReviewingAction] = useState<EvidenceReviewAction | null>(null);
  const previewUrls = useEvidencePreviewUrls(evidence);
  const filterCounts = useMemo(() => evidenceFilterCounts(evidence), [evidence]);
  const rows = useMemo(() => filterEvidence(evidence, activeFilter, query), [activeFilter, evidence, query]);

  async function openEvidence(record: EvidenceRecord) {
    const previewUrl = previewUrls[record.id];
    if (previewUrl) {
      window.open(previewUrl, '_blank', 'noopener,noreferrer');
      return;
    }
    if (record.storage_provider === 'local' && record.storage_key) {
      const viewer = window.open('', '_blank', 'noopener,noreferrer');
      try {
        const blob = await downloadBlob(`/evidence/${record.id}/file`);
        const objectUrl = URL.createObjectURL(blob);
        if (viewer) viewer.location.href = objectUrl;
        else window.open(objectUrl, '_blank', 'noopener,noreferrer');
        window.setTimeout(() => URL.revokeObjectURL(objectUrl), 60_000);
      } catch (error) {
        viewer?.close();
        toast.error('Không mở được file evidence', error);
      }
      return;
    }
    if (record.file_url) {
      window.open(record.file_url, '_blank', 'noopener,noreferrer');
      return;
    }
    toast.warn({ title: 'Evidence chưa có file', description: 'Record này không có file nội bộ hoặc URL để mở.' });
  }

  async function submitReview(action: EvidenceReviewAction) {
    if (!selected || !onReview) return;
    const comment = reviewComment.trim();
    if (action === 'reject' && !comment) {
      toast.warn({ title: 'Thiếu lý do reject', description: 'Nhập lý do để từ chối evidence.' });
      return;
    }
    setReviewingAction(action);
    try {
      await onReview(selected, action, comment);
      setSelected(null);
      setReviewComment('');
    } catch {
      // Parent mutation owns the error toast.
    } finally {
      setReviewingAction(null);
    }
  }

  return (
    <div className="space-y-3">
      <div className="flex flex-col gap-2 lg:flex-row lg:items-center lg:justify-between">
        <div className="flex flex-wrap gap-2">
          {filters.map((filter) => (
            <button
              key={filter.id}
              type="button"
              className={clsx(
                'rounded-md border px-2.5 py-1.5 text-xs font-semibold',
                activeFilter === filter.id ? 'border-blue-300 bg-blue-50 text-blue-700' : 'border-line bg-white text-slate-600 hover:bg-slate-50',
              )}
              onClick={() => setActiveFilter(filter.id)}
            >
              {filter.label} {filterCounts[filter.id]}
            </button>
          ))}
        </div>
        <label className="relative min-w-0 lg:w-72">
          <Search className="pointer-events-none absolute left-2.5 top-2.5 h-4 w-4 text-slate-400" />
          <input
            className="w-full rounded-md border border-line bg-white py-2 pl-8 pr-3 text-sm outline-none focus:ring-2 focus:ring-blue-100"
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder="Tìm evidence, work item, caption"
          />
        </label>
      </div>

      {rows.length ? (
        <div className="grid gap-3 sm:grid-cols-2 2xl:grid-cols-3">
          {rows.map((record) => (
            <EvidenceCard
              key={record.id}
              record={record}
              previewUrl={previewUrls[record.id]}
              showWorkItem={showWorkItem}
              onOpen={() => void openEvidence(record)}
              onSelect={() => {
                setSelected(record);
                setReviewComment('');
              }}
            />
          ))}
        </div>
      ) : (
        <EmptyState title={evidence.length ? 'Không có evidence phù hợp bộ lọc' : emptyTitle} />
      )}

      <EvidenceDetailDialog
        record={selected}
        previewUrl={selected ? previewUrls[selected.id] : undefined}
        showWorkItem={showWorkItem}
        canReview={Boolean(onReview)}
        reviewingAction={reviewingAction}
        reviewComment={reviewComment}
        onReviewCommentChange={setReviewComment}
        onOpen={() => selected && void openEvidence(selected)}
        onClose={() => {
          setSelected(null);
          setReviewComment('');
        }}
        onReview={(action) => void submitReview(action)}
      />
    </div>
  );
}

function EvidenceCard({
  record,
  previewUrl,
  showWorkItem,
  onOpen,
  onSelect,
}: {
  record: EvidenceRecord;
  previewUrl?: string;
  showWorkItem: boolean;
  onOpen: () => void;
  onSelect: () => void;
}) {
  const title = evidenceTitle(record);
  return (
    <article className="min-w-0 rounded-md border border-line bg-white p-3">
      <button type="button" className="block w-full text-left" onClick={onSelect}>
        <div className="aspect-[4/3] overflow-hidden rounded-md border border-line bg-slate-100">
          {previewUrl ? (
            <img className="h-full w-full object-cover" src={previewUrl} alt={title} />
          ) : (
            <div className="flex h-full w-full items-center justify-center text-slate-400">
              {isImageEvidence(record) ? <ImageIcon className="h-8 w-8" /> : <FileText className="h-8 w-8" />}
            </div>
          )}
        </div>
        <div className="mt-3 min-w-0">
          <div className="flex items-start justify-between gap-2">
            <p className="min-w-0 truncate text-sm font-semibold text-ink">{title}</p>
            <StatusBadge status={record.status} />
          </div>
          <p className="mt-1 truncate text-xs text-slate-500">
            {evidenceLabel(record.evidence_type)} · {visibilityLabel(record.visibility ?? 'project')} · {compactDateTime(record.created_at)}
          </p>
          {showWorkItem ? <p className="mt-1 truncate text-xs text-slate-500">{record.work_item_title ?? 'Work item'}</p> : null}
          <div className="mt-2 flex flex-wrap items-center gap-2 text-xs text-slate-500">
            {record.mime_type ? <span>{record.mime_type}</span> : null}
            {record.file_size ? <span>{formatEvidenceFileSize(record.file_size)}</span> : null}
            {record.checksum_sha256 ? <span>sha256:{record.checksum_sha256.slice(0, 12)}</span> : null}
          </div>
        </div>
      </button>
      <div className="mt-3 flex flex-wrap items-center justify-between gap-2">
        <span className={clsx('rounded-md px-2 py-0.5 text-xs font-semibold', record.storage_provider === 'local' ? 'bg-emerald-50 text-emerald-700' : 'bg-slate-100 text-slate-600')}>
          {record.storage_provider === 'local' ? 'Protected file' : 'External link'}
        </span>
        <div className="flex flex-wrap justify-end gap-2">
          <button type="button" className="rounded-md border border-line px-2 py-1 text-xs font-semibold text-slate-700 hover:bg-slate-100" onClick={onSelect}>
            Chi tiết
          </button>
          <button
            type="button"
            className="inline-flex items-center gap-1 rounded-md border border-line px-2 py-1 text-xs font-semibold text-blue-700 hover:border-blue-300 hover:bg-blue-50"
            onClick={onOpen}
          >
            <ExternalLink className="h-3.5 w-3.5" />
            Mở
          </button>
        </div>
      </div>
      {record.caption ? <p className="mt-2 line-clamp-2 text-sm leading-5 text-slate-600">{record.caption}</p> : null}
    </article>
  );
}

function EvidenceDetailDialog({
  record,
  previewUrl,
  showWorkItem,
  canReview,
  reviewingAction,
  reviewComment,
  onReviewCommentChange,
  onOpen,
  onClose,
  onReview,
}: {
  record: EvidenceRecord | null;
  previewUrl?: string;
  showWorkItem: boolean;
  canReview: boolean;
  reviewingAction: EvidenceReviewAction | null;
  reviewComment: string;
  onReviewCommentChange: (value: string) => void;
  onOpen: () => void;
  onClose: () => void;
  onReview: (action: EvidenceReviewAction) => void;
}) {
  if (!record) return null;
  const reviewable = canReview && ['draft', 'submitted', 'pending'].includes(String(record.status));
  return (
    <Dialog
      open={Boolean(record)}
      title="Chi tiết evidence"
      description="Metadata, storage và trạng thái review của file evidence."
      size="xl"
      onClose={onClose}
      footer={
        <div className="flex flex-wrap items-center justify-between gap-2">
          <p className="text-xs text-slate-500">Protected files được mở qua API có kiểm tra quyền work item.</p>
          <div className="flex flex-wrap justify-end gap-2">
            {reviewable ? (
              <>
                <button
                  type="button"
                  className="inline-flex items-center gap-2 rounded-md border border-rose-200 bg-white px-3 py-2 text-sm font-semibold text-rose-700 hover:bg-rose-50 disabled:cursor-not-allowed disabled:opacity-60"
                  disabled={Boolean(reviewingAction)}
                  onClick={() => onReview('reject')}
                >
                  {reviewingAction === 'reject' ? <Loader2 className="h-4 w-4 animate-spin" /> : <XCircle className="h-4 w-4" />}
                  Reject
                </button>
                <button
                  type="button"
                  className="inline-flex items-center gap-2 rounded-md border border-emerald-200 bg-white px-3 py-2 text-sm font-semibold text-emerald-700 hover:bg-emerald-50 disabled:cursor-not-allowed disabled:opacity-60"
                  disabled={Boolean(reviewingAction)}
                  onClick={() => onReview('accept')}
                >
                  {reviewingAction === 'accept' ? <Loader2 className="h-4 w-4 animate-spin" /> : <CheckCircle2 className="h-4 w-4" />}
                  Accept
                </button>
              </>
            ) : null}
            <button
              type="button"
              className="inline-flex items-center gap-2 rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-blue-700 hover:border-blue-300 hover:bg-blue-50"
              onClick={onOpen}
            >
              <ExternalLink className="h-4 w-4" />
              Mở file
            </button>
            <button type="button" className="rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100" onClick={onClose}>
              Đóng
            </button>
          </div>
        </div>
      }
    >
      <div className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_360px]">
        <div className="min-w-0">
          <div className="overflow-hidden rounded-md border border-line bg-slate-100">
            {previewUrl ? (
              <img className="max-h-[520px] w-full object-contain" src={previewUrl} alt={evidenceTitle(record)} />
            ) : (
              <div className="flex min-h-72 items-center justify-center text-slate-400">
                {isImageEvidence(record) ? <ImageIcon className="h-12 w-12" /> : <FileText className="h-12 w-12" />}
              </div>
            )}
          </div>
          {record.caption ? <p className="mt-3 whitespace-pre-wrap break-words rounded-md border border-line bg-slate-50 p-3 text-sm leading-6 text-slate-700">{record.caption}</p> : null}
        </div>
        <div className="min-w-0 space-y-3">
          <div className="flex flex-wrap items-start justify-between gap-2">
            <div className="min-w-0">
              <p className="break-words text-sm font-semibold text-ink">{evidenceTitle(record)}</p>
              <p className="mt-1 text-xs text-slate-500">{record.document_name ?? originalFilename(record) ?? record.id}</p>
            </div>
            <StatusBadge status={record.status} />
          </div>
          <div className="grid gap-2">
            <EvidenceMeta label="Type" value={evidenceLabel(record.evidence_type)} />
            <EvidenceMeta label="Visibility" value={visibilityLabel(record.visibility ?? 'project')} />
            {showWorkItem ? <EvidenceMeta label="Work item" value={record.work_item_title ?? '-'} /> : null}
            <EvidenceMeta label="Submitted by" value={record.submitted_by_name ?? record.uploaded_by ?? '-'} />
            <EvidenceMeta label="Submitted at" value={compactDateTime(record.created_at)} />
            <EvidenceMeta label="Reviewed by" value={record.reviewed_by_name ?? record.reviewed_by ?? '-'} />
            <EvidenceMeta label="Reviewed at" value={compactDateTime(record.reviewed_at)} />
            <EvidenceMeta label="Storage" value={record.storage_provider === 'local' ? 'Protected local file' : (record.file_url ? 'External link' : '-')} />
            <EvidenceMeta label="MIME / Size" value={[record.mime_type, record.file_size ? formatEvidenceFileSize(record.file_size) : undefined].filter(Boolean).join(' · ') || '-'} />
            <EvidenceMeta label="Original filename" value={originalFilename(record) ?? '-'} />
          </div>
          <div className="rounded-md border border-line bg-slate-50 p-3">
            <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">Checksum sha256</p>
            <p className="mt-1 break-all font-mono text-xs leading-5 text-slate-700">{record.checksum_sha256 || '-'}</p>
          </div>
          {reviewable ? (
            <label className="block">
              <span className="text-xs font-semibold text-slate-600">Review comment</span>
              <textarea
                className="mt-1 w-full rounded-md border border-line bg-white px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-100"
                rows={3}
                value={reviewComment}
                onChange={(event) => onReviewCommentChange(event.target.value)}
                placeholder="Lý do accept/reject"
                disabled={Boolean(reviewingAction)}
              />
            </label>
          ) : null}
        </div>
      </div>
    </Dialog>
  );
}

function EvidenceMeta({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-md border border-line bg-white px-3 py-2">
      <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">{label}</p>
      <p className="mt-1 break-words text-sm text-ink">{value}</p>
    </div>
  );
}

function evidenceFilterCounts(evidence: EvidenceRecord[]): Record<EvidenceFilter, number> {
  return {
    all: evidence.length,
    photo: evidence.filter(isImageEvidence).length,
    document: evidence.filter(isDocumentEvidence).length,
    submitted: evidence.filter((record) => ['draft', 'submitted', 'pending'].includes(String(record.status))).length,
    accepted: evidence.filter((record) => record.status === 'accepted').length,
    rejected: evidence.filter((record) => record.status === 'rejected').length,
  };
}

function filterEvidence(evidence: EvidenceRecord[], filter: EvidenceFilter, query: string) {
  const term = query.trim().toLowerCase();
  return evidence.filter((record) => {
    const matchesFilter =
      filter === 'all'
      || (filter === 'photo' && isImageEvidence(record))
      || (filter === 'document' && isDocumentEvidence(record))
      || (filter === 'submitted' && ['draft', 'submitted', 'pending'].includes(String(record.status)))
      || record.status === filter;
    if (!matchesFilter) return false;
    if (!term) return true;
    return [
      record.name,
      record.title,
      record.document_name,
      record.caption,
      record.description,
      record.work_item_title,
      record.file_url,
      record.checksum_sha256,
      originalFilename(record),
    ].some((value) => String(value ?? '').toLowerCase().includes(term));
  });
}

function useEvidencePreviewUrls(evidence: EvidenceRecord[]) {
  const [urls, setUrls] = useState<Record<string, string>>({});
  const signature = useMemo(
    () => evidence.map((record) => `${record.id}:${record.storage_key ?? ''}:${record.updated_at ?? ''}:${record.mime_type ?? ''}`).join('|'),
    [evidence],
  );

  useEffect(() => {
    let cancelled = false;
    const objectUrls: string[] = [];
    setUrls({});

    async function loadPreviews() {
      const imageRecords = evidence.filter((record) => record.storage_provider === 'local' && record.storage_key && isImageEvidence(record));
      const entries = await Promise.all(imageRecords.map(async (record) => {
        try {
          const blob = await downloadBlob(`/evidence/${record.id}/file`);
          const objectUrl = URL.createObjectURL(blob);
          objectUrls.push(objectUrl);
          return [record.id, objectUrl] as const;
        } catch {
          return null;
        }
      }));
      if (!cancelled) {
        setUrls(Object.fromEntries(entries.filter(Boolean) as Array<readonly [string, string]>));
      } else {
        objectUrls.forEach((url) => URL.revokeObjectURL(url));
      }
    }

    void loadPreviews();
    return () => {
      cancelled = true;
      objectUrls.forEach((url) => URL.revokeObjectURL(url));
    };
  }, [signature]);

  return urls;
}

export function isImageEvidence(record: EvidenceRecord) {
  return record.evidence_type === 'photo' || String(record.mime_type ?? '').startsWith('image/');
}

function isDocumentEvidence(record: EvidenceRecord) {
  return record.evidence_type === 'document' || String(record.mime_type ?? '').includes('pdf');
}

function evidenceTitle(record: EvidenceRecord) {
  return record.name ?? record.title ?? record.document_name ?? record.id;
}

function originalFilename(record: EvidenceRecord) {
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
