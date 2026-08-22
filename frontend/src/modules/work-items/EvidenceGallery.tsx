import { useEffect, useMemo, useState } from 'react';
import { CheckCircle2, ExternalLink, Eye, FileText, Image as ImageIcon, Search } from 'lucide-react';
import clsx from 'clsx';
import { EmptyState } from '../../components/EmptyState';
import { StatusBadge } from '../../components/StatusBadge';
import { identityReferenceText } from '../../components/IdentityReferenceLabel';
import { useToast } from '../../components/ToastProvider';
import { downloadBlob } from '../../lib/api/client';
import type { EvidenceRecord } from '../../lib/types/domain';
import { openProtectedBlob } from '../../lib/utils/openProtectedBlob';
import { isProtectedStorageProvider } from '../../lib/utils/protectedStorage.ts';
import {
  EvidenceRecordViewerDialog,
  evidenceRecordTitle as evidenceTitle,
  formatEvidenceFileSize,
  isImageEvidence,
  isInlinePdfEvidence,
  isInlineSpreadsheetEvidence,
  isInlineWordEvidence,
  isPdfEvidence,
  originalFilename,
  type EvidenceReviewAction,
} from '../_shared/EvidenceRecordViewerDialog';
import { compactDateTime, evidenceLabel, visibilityLabel } from './workItemPresentation';

export { formatEvidenceFileSize, isImageEvidence };
export type { EvidenceReviewAction };

type EvidenceFilter = 'all' | 'photo' | 'document' | 'submitted' | 'accepted' | 'rejected';

const PROJECT_EVIDENCE_LIST_GRID_CLASS = 'xl:grid-cols-[8rem_minmax(0,1.4fr)_minmax(0,0.75fr)_minmax(0,1fr)_9rem_8rem_7rem]';
const DEFAULT_EVIDENCE_LIST_GRID_CLASS = 'lg:grid-cols-[8rem_minmax(0,1.6fr)_8rem_8rem_7rem]';

type EvidenceGalleryProps = {
  evidence: EvidenceRecord[];
  emptyTitle?: string;
  showWorkItem?: boolean;
  showControls?: boolean;
  variant?: 'cards' | 'list';
  listContext?: 'default' | 'project';
  onReview?: (record: EvidenceRecord, action: EvidenceReviewAction, comment: string) => Promise<void>;
};

const filters: Array<{ id: EvidenceFilter; label: string }> = [
  { id: 'all', label: 'Tất cả' },
  { id: 'photo', label: 'Ảnh' },
  { id: 'document', label: 'Tài liệu' },
  { id: 'submitted', label: 'Đang chờ' },
  { id: 'accepted', label: 'Đã duyệt' },
  { id: 'rejected', label: 'Từ chối' },
];

export function EvidenceGallery({
  evidence,
  emptyTitle = 'Chưa có minh chứng',
  showWorkItem = false,
  showControls = true,
  variant = 'cards',
  listContext = 'default',
  onReview,
}: EvidenceGalleryProps) {
  const toast = useToast();
  const [activeFilter, setActiveFilter] = useState<EvidenceFilter>('all');
  const [query, setQuery] = useState('');
  const [selected, setSelected] = useState<EvidenceRecord | null>(null);
  const [reviewComment, setReviewComment] = useState('');
  const [reviewingAction, setReviewingAction] = useState<EvidenceReviewAction | null>(null);
  const [previewRetryKey, setPreviewRetryKey] = useState(0);
  const previews = useEvidencePreviewUrls(evidence, previewRetryKey);
  const previewUrls = previews.urls;
  const filterCounts = useMemo(() => evidenceFilterCounts(evidence), [evidence]);
  const rows = useMemo(
    () => showControls ? filterEvidence(evidence, activeFilter, query) : evidence,
    [activeFilter, evidence, query, showControls],
  );

  async function openEvidence(record: EvidenceRecord) {
    const previewUrl = previewUrls[record.id];
    if (previewUrl) {
      window.open(previewUrl, '_blank', 'noopener,noreferrer');
      return;
    }
    if (isProtectedStorageProvider(record.storage_provider) && record.storage_key) {
      try {
        await openProtectedBlob(() => downloadBlob(`/evidence/${record.id}/file`));
      } catch (error) {
        toast.error('Không mở được tệp minh chứng', error);
      }
      return;
    }
    if (record.file_url) {
      window.open(record.file_url, '_blank', 'noopener,noreferrer');
      return;
    }
    toast.warn({ title: 'Minh chứng chưa có tệp', description: 'Bản ghi này không có tệp nội bộ hoặc URL để mở.' });
  }

  async function submitReview(action: EvidenceReviewAction) {
    if (!selected || !onReview) return;
    const comment = reviewComment.trim();
    if (action === 'reject' && !comment) {
      toast.warn({ title: 'Thiếu lý do từ chối', description: 'Nhập lý do để từ chối minh chứng.' });
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
      {showControls ? <div className="space-y-2">
        <label className="relative block w-full">
          <Search className="pointer-events-none absolute left-2.5 top-2.5 h-4 w-4 text-slate-400" />
          <input
            className="w-full rounded-md border border-line bg-white py-2 pl-8 pr-3 text-sm outline-none focus:ring-2 focus:ring-blue-100"
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder="Tìm minh chứng, đầu việc, mô tả"
          />
        </label>
        <div className="min-w-0 space-y-1">
          <p className="text-[11px] font-semibold uppercase tracking-wide text-slate-500">Lọc minh chứng</p>
          <div className="inline-flex max-w-full flex-wrap gap-1 rounded-lg border border-line bg-slate-50 p-1" role="group" aria-label="Lọc minh chứng">
            {filters.map((filter) => {
              const selected = activeFilter === filter.id;
              return (
                <button
                  key={filter.id}
                  type="button"
                  aria-pressed={selected}
                  className={clsx(
                    'inline-flex min-h-8 items-center gap-1.5 rounded-md border px-2.5 py-1.5 text-xs font-semibold transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500',
                    selected
                      ? 'border-blue-300 bg-white text-blue-700 shadow-sm ring-1 ring-inset ring-blue-100'
                      : 'border-transparent bg-transparent text-slate-600 hover:border-line hover:bg-white hover:text-ink',
                  )}
                  onClick={() => setActiveFilter(filter.id)}
                >
                  {selected ? (
                    <CheckCircle2 className="h-3.5 w-3.5 shrink-0" />
                  ) : (
                    <span className="h-2 w-2 shrink-0 rounded-full border border-slate-300 bg-white" aria-hidden="true" />
                  )}
                  <span>{filter.label}</span>
                  <span
                    className={clsx(
                      'rounded-full px-1.5 py-0.5 text-[11px] tabular-nums',
                      selected ? 'bg-blue-100 text-blue-700' : 'bg-white text-slate-500 ring-1 ring-inset ring-line',
                    )}
                  >
                    {filterCounts[filter.id]}
                  </span>
                </button>
              );
            })}
          </div>
        </div>
      </div> : null}

      {rows.length && variant === 'list' ? (
        <div className="overflow-hidden rounded-md border border-line bg-white">
          {listContext === 'project' ? (
            <div className={clsx('hidden gap-3 border-b border-line bg-slate-50 px-3 py-2 text-[11px] font-semibold uppercase tracking-wide text-slate-500 xl:grid', PROJECT_EVIDENCE_LIST_GRID_CLASS)}>
              <span><span className="sr-only">Thao tác</span></span>
              <span>Hồ sơ</span>
              <span>Giai đoạn</span>
              <span>Đầu việc</span>
              <span>Người gửi</span>
              <span>Thời gian</span>
              <span>Trạng thái</span>
            </div>
          ) : (
            <div className={clsx('hidden gap-3 border-b border-line bg-slate-50 px-3 py-2 text-[11px] font-semibold uppercase tracking-wide text-slate-500 lg:grid', DEFAULT_EVIDENCE_LIST_GRID_CLASS)}>
              <span><span className="sr-only">Thao tác</span></span>
              <span>Minh chứng</span>
              <span>Loại</span>
              <span>Phạm vi hiển thị</span>
              <span>Trạng thái</span>
            </div>
          )}
          <div className="divide-y divide-line">
            {rows.map((record) => (
              <EvidenceListRow
                key={record.id}
                record={record}
                previewUrl={previewUrls[record.id]}
                showWorkItem={showWorkItem}
                listContext={listContext}
                onOpen={() => canPreviewEvidenceInApp(record) ? setSelected(record) : void openEvidence(record)}
                onSelect={() => {
                  setSelected(record);
                  setReviewComment('');
                }}
              />
            ))}
          </div>
        </div>
      ) : rows.length ? (
        <div className="grid gap-3 sm:grid-cols-2 2xl:grid-cols-3">
          {rows.map((record) => (
            <EvidenceCard
              key={record.id}
              record={record}
              previewUrl={previewUrls[record.id]}
              showWorkItem={showWorkItem}
              onOpen={() => canPreviewEvidenceInApp(record) ? setSelected(record) : void openEvidence(record)}
              onSelect={() => {
                setSelected(record);
                setReviewComment('');
              }}
            />
          ))}
        </div>
      ) : (
        <EmptyState title={evidence.length ? 'Không có minh chứng phù hợp bộ lọc' : emptyTitle} />
      )}

      <EvidenceRecordViewerDialog
        record={selected}
        previewUrl={selected ? previewUrls[selected.id] || (!selected.storage_key ? selected.file_url : undefined) : undefined}
        previewLoading={selected ? previews.loadingIds.has(selected.id) : false}
        previewError={selected && previews.errorIds.has(selected.id) ? 'Không tải được ảnh minh chứng.' : undefined}
        showWorkItem={showWorkItem}
        canReview={Boolean(onReview) && (selected?.can_review ?? true)}
        reviewingAction={reviewingAction}
        reviewComment={reviewComment}
        onReviewCommentChange={setReviewComment}
        onOpenFile={() => selected && void openEvidence(selected)}
        onRetry={() => setPreviewRetryKey((current) => current + 1)}
        onClose={() => {
          setSelected(null);
          setReviewComment('');
        }}
        onReview={(action) => void submitReview(action)}
      />
    </div>
  );
}

function EvidenceListRow({
  record,
  previewUrl,
  showWorkItem,
  listContext,
  onOpen,
  onSelect,
}: {
  record: EvidenceRecord;
  previewUrl?: string;
  showWorkItem: boolean;
  listContext: 'default' | 'project';
  onOpen: () => void;
  onSelect: () => void;
}) {
  const title = evidenceTitle(record);
  const fileMeta = [
    evidenceLabel(record.evidence_type),
    visibilityLabel(record.visibility ?? 'project'),
    record.mime_type,
    record.file_size ? formatEvidenceFileSize(record.file_size) : undefined,
  ].filter(Boolean).join(' · ') || compactDateTime(record.created_at);
  const fileCell = (
    <button
      type="button"
      className="flex min-w-0 items-start gap-3 text-left"
      aria-label={`Xem chi tiết hồ sơ ${title}`}
      onClick={onSelect}
    >
      <span className="grid h-12 w-14 shrink-0 place-items-center overflow-hidden rounded-md border border-line bg-slate-100 text-slate-400">
        {previewUrl ? (
          <img className="h-full w-full object-cover" src={previewUrl} alt={title} />
        ) : isImageEvidence(record) ? (
          <ImageIcon className="h-5 w-5" />
        ) : (
          <FileText className="h-5 w-5" />
        )}
      </span>
      <span className="min-w-0">
        <span
          className={clsx('block text-sm font-semibold leading-5 text-ink', listContext === 'project' ? 'truncate' : 'line-clamp-2')}
          title={listContext === 'project' ? title : undefined}
        >
          {title}
        </span>
        <span
          className="mt-1 block truncate text-xs text-slate-500"
          title={listContext === 'project' ? fileMeta : undefined}
        >
          {fileMeta}
        </span>
        {showWorkItem && listContext !== 'project' ? <span className="mt-1 block line-clamp-1 text-xs text-slate-500">{record.run_node_name ? `${record.run_node_name} · ` : ''}{record.work_item_title ?? 'Đầu việc'}</span> : null}
      </span>
    </button>
  );
  return (
    <article className={clsx(
      'grid gap-3 px-3 py-3 lg:items-center',
      listContext === 'project'
        ? PROJECT_EVIDENCE_LIST_GRID_CLASS
        : DEFAULT_EVIDENCE_LIST_GRID_CLASS,
    )}>
      <div className="flex flex-wrap gap-2">
        <button
          type="button"
          className="inline-flex items-center gap-1 rounded-md border border-line px-2 py-1 text-xs font-semibold text-blue-700 hover:border-blue-300 hover:bg-blue-50"
          onClick={onOpen}
        >
          {canPreviewEvidenceInApp(record) ? <Eye className="h-3.5 w-3.5" /> : <ExternalLink className="h-3.5 w-3.5" />}
          {evidenceOpenLabel(record)}
        </button>
      </div>
      {fileCell}
      {listContext === 'project' ? (
        <>
          <ProjectEvidenceTextCell label="Giai đoạn" value={record.run_node_name} />
          <ProjectEvidenceTextCell label="Đầu việc" value={record.work_item_title} />
          <ProjectEvidenceTextCell label="Người gửi" value={identityReferenceText(record.submitted_by_name ?? record.uploaded_by, record.submitted_by_account_deleted)} />
          <div className="grid min-w-0 grid-cols-[6rem_minmax(0,1fr)] items-start gap-2 text-xs text-slate-600 xl:block xl:text-sm">
            <span className="text-[11px] font-semibold uppercase tracking-wide text-slate-400 xl:hidden">Thời gian</span>
            <span>{compactDateTime(record.created_at)}</span>
          </div>
        </>
      ) : (
        <>
          <div className="text-xs font-semibold text-slate-600 lg:text-sm lg:font-medium">
            <span className="mr-2 text-[11px] uppercase tracking-wide text-slate-400 lg:hidden">Loại</span>
            {evidenceLabel(record.evidence_type)}
          </div>
          <div className="text-xs font-semibold text-slate-600 lg:text-sm lg:font-medium">
            <span className="mr-2 text-[11px] uppercase tracking-wide text-slate-400 lg:hidden">Phạm vi</span>
            {visibilityLabel(record.visibility ?? 'project')}
          </div>
        </>
      )}
      <div>
        <StatusBadge status={record.status} />
      </div>
    </article>
  );
}

function ProjectEvidenceTextCell({ label, value }: { label: string; value?: string }) {
  const displayValue = String(value ?? '').trim() || '—';
  return (
    <div className="grid min-w-0 grid-cols-[6rem_minmax(0,1fr)] items-start gap-2 text-xs text-slate-600 xl:block xl:text-sm">
      <span className="text-[11px] font-semibold uppercase tracking-wide text-slate-400 xl:hidden">{label}</span>
      <span
        className="block min-w-0 truncate rounded-sm outline-none focus-visible:ring-2 focus-visible:ring-blue-200"
        title={displayValue}
        aria-label={`${label}: ${displayValue}`}
        tabIndex={0}
      >
        {displayValue}
      </span>
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
          {showWorkItem ? <p className="mt-1 truncate text-xs text-slate-500">{record.work_item_title ?? 'Đầu việc'}</p> : null}
          <div className="mt-2 flex flex-wrap items-center gap-2 text-xs text-slate-500">
            {record.mime_type ? <span>{record.mime_type}</span> : null}
            {record.file_size ? <span>{formatEvidenceFileSize(record.file_size)}</span> : null}
            {record.checksum_sha256 ? <span>sha256:{record.checksum_sha256.slice(0, 12)}</span> : null}
          </div>
        </div>
      </button>
      <div className="mt-3 flex flex-wrap items-center justify-between gap-2">
        <span className={clsx('rounded-md px-2 py-0.5 text-xs font-semibold', isProtectedStorageProvider(record.storage_provider) ? 'bg-emerald-50 text-emerald-700' : 'bg-slate-100 text-slate-600')}>
          {isProtectedStorageProvider(record.storage_provider) ? 'Tệp được bảo vệ' : 'Liên kết ngoài'}
        </span>
        <div className="flex justify-end">
          <button
            type="button"
            className="inline-flex items-center gap-1 rounded-md border border-line px-2 py-1 text-xs font-semibold text-blue-700 hover:border-blue-300 hover:bg-blue-50"
            onClick={onOpen}
          >
            {canPreviewEvidenceInApp(record) ? <Eye className="h-3.5 w-3.5" /> : <ExternalLink className="h-3.5 w-3.5" />}
            {evidenceOpenLabel(record)}
          </button>
        </div>
      </div>
      {record.caption ? <p className="mt-2 line-clamp-2 text-sm leading-5 text-slate-600">{record.caption}</p> : null}
    </article>
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

export function useEvidencePreviewUrls(evidence: EvidenceRecord[], retryKey = 0) {
  const [state, setState] = useState<{
    urls: Record<string, string>;
    loadingIds: Set<string>;
    errorIds: Set<string>;
  }>({ urls: {}, loadingIds: new Set(), errorIds: new Set() });
  const signature = useMemo(
    () => evidence.map((record) => `${record.id}:${record.storage_key ?? ''}:${record.updated_at ?? ''}:${record.mime_type ?? ''}`).join('|'),
    [evidence],
  );

  useEffect(() => {
    let cancelled = false;
    const objectUrls: string[] = [];
    const imageRecords = evidence.filter((record) => isProtectedStorageProvider(record.storage_provider) && record.storage_key && isImageEvidence(record));
    setState({ urls: {}, loadingIds: new Set(imageRecords.map((record) => record.id)), errorIds: new Set() });

    async function loadPreviews() {
      const entries = await Promise.all(imageRecords.map(async (record) => {
        try {
          const blob = await downloadBlob(`/evidence/${record.id}/file`);
          const objectUrl = URL.createObjectURL(blob);
          objectUrls.push(objectUrl);
          return { id: record.id, objectUrl };
        } catch {
          return { id: record.id, objectUrl: '' };
        }
      }));
      if (!cancelled) {
        setState({
          urls: Object.fromEntries(entries.filter((entry) => entry.objectUrl).map((entry) => [entry.id, entry.objectUrl])),
          loadingIds: new Set(),
          errorIds: new Set(entries.filter((entry) => !entry.objectUrl).map((entry) => entry.id)),
        });
      } else {
        objectUrls.forEach((url) => URL.revokeObjectURL(url));
      }
    }

    void loadPreviews();
    return () => {
      cancelled = true;
      objectUrls.forEach((url) => URL.revokeObjectURL(url));
    };
  }, [retryKey, signature]);

  return state;
}

function isDocumentEvidence(record: EvidenceRecord) {
  return record.evidence_type === 'document' || isPdfEvidence(record);
}

function canPreviewEvidenceInApp(record: EvidenceRecord) {
  return isImageEvidence(record)
    || isInlinePdfEvidence(record)
    || isInlineWordEvidence(record)
    || isInlineSpreadsheetEvidence(record);
}

function evidenceOpenLabel(record: EvidenceRecord) {
  if (isImageEvidence(record)) return 'Xem ảnh';
  if (isInlinePdfEvidence(record)) return 'Xem PDF';
  if (isInlineWordEvidence(record)) return 'Xem Word';
  if (isInlineSpreadsheetEvidence(record)) return 'Xem Excel';
  return 'Mở tệp';
}
