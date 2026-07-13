import { useDeferredValue, useMemo, useState, type ReactNode } from 'react';
import { useInfiniteQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  AlertCircle,
  CheckCircle2,
  Clock3,
  FileText,
  Image as ImageIcon,
  Loader2,
  RotateCcw,
  Search,
  SlidersHorizontal,
  XCircle,
} from 'lucide-react';
import clsx from 'clsx';
import { useToast } from '../../components/ToastProvider';
import { get, post } from '../../lib/api/client';
import type {
  EvidenceRecord,
  ProjectEvidencePage,
  ProjectEvidenceSummary,
} from '../../lib/types/domain';
import { EvidenceGallery, type EvidenceReviewAction } from '../work-items/EvidenceGallery';

type EvidenceStatusFilter = '' | 'waiting' | 'accepted' | 'rejected';

const emptySummary: ProjectEvidenceSummary = {
  total: 0,
  waiting: 0,
  accepted: 0,
  rejected: 0,
  photos: 0,
  documents: 0,
};

export function ProjectEvidencePanel({ projectId }: { projectId: string }) {
  const [status, setStatus] = useState<EvidenceStatusFilter>('');
  const [evidenceType, setEvidenceType] = useState('');
  const [workItemId, setWorkItemId] = useState('');
  const [submittedBy, setSubmittedBy] = useState('');
  const [visibility, setVisibility] = useState('');
  const [dateFrom, setDateFrom] = useState('');
  const [dateTo, setDateTo] = useState('');
  const [search, setSearch] = useState('');
  const [showFilters, setShowFilters] = useState(false);
  const deferredSearch = useDeferredValue(search.trim());
  const queryClient = useQueryClient();
  const toast = useToast();

  const queryKey = [
    'project-evidence',
    projectId,
    status,
    evidenceType,
    workItemId,
    submittedBy,
    visibility,
    dateFrom,
    dateTo,
    deferredSearch,
  ];
  const evidence = useInfiniteQuery({
    queryKey,
    initialPageParam: '',
    queryFn: ({ pageParam }) => {
      const params = new URLSearchParams();
      if (status) params.set('status', status);
      if (evidenceType) params.set('evidence_type', evidenceType);
      if (workItemId) params.set('work_item_id', workItemId);
      if (submittedBy) params.set('submitted_by', submittedBy);
      if (visibility) params.set('visibility', visibility);
      if (dateFrom) params.set('date_from', dateFrom);
      if (dateTo) params.set('date_to', dateTo);
      if (deferredSearch) params.set('search', deferredSearch);
      if (pageParam) params.set('cursor', String(pageParam));
      params.set('limit', '25');
      return get<ProjectEvidencePage>(`/projects/${projectId}/evidence?${params.toString()}`);
    },
    getNextPageParam: (page) => page.has_more && page.next_cursor ? page.next_cursor : undefined,
    enabled: Boolean(projectId),
    retry: false,
  });

  const pages = evidence.data?.pages ?? [];
  const rows = useMemo(() => pages.flatMap((page) => page.items ?? []), [pages]);
  const firstPage = pages[0];
  const summary = firstPage?.summary ?? emptySummary;
  const options = firstPage?.options ?? { work_items: [], submitters: [] };
  const filteredTotal = Number(firstPage?.filtered_total ?? 0);
  const hasActiveFilters = Boolean(status || evidenceType || workItemId || submittedBy || visibility || dateFrom || dateTo || search.trim());

  const reviewEvidence = useMutation({
    mutationFn: ({ record, action, comment }: { record: EvidenceRecord; action: EvidenceReviewAction; comment: string }) =>
      post<EvidenceRecord>(`/evidence/${record.id}/${action}`, { comment, reason: action === 'reject' ? comment : undefined }),
    onSuccess: (_record, variables) => {
      queryClient.invalidateQueries({ queryKey: ['project-evidence', projectId] });
      queryClient.invalidateQueries({ queryKey: ['project-operations', projectId] });
      toast.success(variables.action === 'accept' ? 'Đã chấp nhận minh chứng' : 'Đã từ chối minh chứng');
    },
    onError: (error) => toast.error('Không rà soát được minh chứng', error),
  });

  function resetFilters() {
    setStatus('');
    setEvidenceType('');
    setWorkItemId('');
    setSubmittedBy('');
    setVisibility('');
    setDateFrom('');
    setDateTo('');
    setSearch('');
  }

  return (
    <section className="space-y-4">
      <div className="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
        <div className="min-w-0">
          <div className="flex items-center gap-2">
            <FileText className="h-5 w-5 text-blue-700" />
            <h2 className="font-semibold text-ink">Hồ sơ công trình</h2>
          </div>
          <p className="mt-1 text-sm text-slate-500">Tra cứu và rà soát minh chứng phát sinh từ công việc và workflow.</p>
        </div>
        <span className="w-fit rounded-md bg-slate-100 px-2.5 py-1 text-xs font-semibold tabular-nums text-slate-600">
          Tổng {summary.total}
        </span>
      </div>

      <div className="grid gap-2 sm:grid-cols-2 xl:grid-cols-4">
        <EvidenceMetric
          active={status === 'waiting'}
          icon={<Clock3 className="h-4 w-4" />}
          label="Cần rà soát"
          value={summary.waiting}
          tone="warn"
          onClick={() => setStatus((current) => current === 'waiting' ? '' : 'waiting')}
        />
        <EvidenceMetric
          active={status === 'accepted'}
          icon={<CheckCircle2 className="h-4 w-4" />}
          label="Đã chấp nhận"
          value={summary.accepted}
          tone="ok"
          onClick={() => setStatus((current) => current === 'accepted' ? '' : 'accepted')}
        />
        <EvidenceMetric
          active={status === 'rejected'}
          icon={<XCircle className="h-4 w-4" />}
          label="Từ chối"
          value={summary.rejected}
          tone="danger"
          onClick={() => setStatus((current) => current === 'rejected' ? '' : 'rejected')}
        />
        <EvidenceMetric
          active={!status && !evidenceType}
          icon={<ImageIcon className="h-4 w-4" />}
          label="Ảnh / Tài liệu"
          value={`${summary.photos} / ${summary.documents}`}
          onClick={() => {
            setStatus('');
            setEvidenceType('');
          }}
        />
      </div>

      <div className="rounded-lg border border-line bg-white p-3 shadow-panel">
        <div className="flex flex-col gap-2 lg:flex-row lg:items-center">
          <label className="relative block min-w-0 flex-1">
            <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" />
            <input
              className="w-full rounded-md border border-line bg-white py-2 pl-9 pr-3 text-sm outline-none focus:ring-2 focus:ring-blue-100"
              value={search}
              onChange={(event) => setSearch(event.target.value)}
              placeholder="Tìm tên hồ sơ, đầu việc, người gửi, mã SHA-256..."
            />
          </label>
          <div className="flex flex-wrap items-center gap-2">
            <button
              type="button"
              className={clsx(
                'inline-flex items-center gap-2 rounded-md border px-3 py-2 text-sm font-semibold lg:hidden',
                showFilters ? 'border-blue-200 bg-blue-50 text-blue-700' : 'border-line text-slate-700 hover:bg-slate-50',
              )}
              onClick={() => setShowFilters((current) => !current)}
            >
              <SlidersHorizontal className="h-4 w-4" />
              Bộ lọc
            </button>
            {hasActiveFilters ? (
              <button
                type="button"
                className="inline-flex items-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-600 hover:bg-slate-50"
                onClick={resetFilters}
              >
                <RotateCcw className="h-4 w-4" />
                Xóa lọc
              </button>
            ) : null}
            <span className="rounded-md bg-slate-50 px-2.5 py-2 text-xs font-medium text-slate-500">Cần xử lý trước · mới nhất</span>
          </div>
        </div>

        <div className={clsx('mt-3 gap-2 lg:grid lg:grid-cols-3 xl:grid-cols-6', showFilters ? 'grid' : 'hidden')}>
          <EvidenceSelect label="Loại" value={evidenceType} onChange={setEvidenceType}>
            <option value="">Tất cả loại</option>
            <option value="photo">Ảnh</option>
            <option value="document">Tài liệu</option>
            <option value="checklist">Checklist</option>
            <option value="link">Liên kết</option>
            <option value="note">Ghi chú</option>
            <option value="other">Khác</option>
          </EvidenceSelect>
          <EvidenceSelect label="Trạng thái" value={status} onChange={(value) => setStatus(value as EvidenceStatusFilter)}>
            <option value="">Tất cả trạng thái</option>
            <option value="waiting">Cần rà soát</option>
            <option value="accepted">Đã chấp nhận</option>
            <option value="rejected">Từ chối</option>
          </EvidenceSelect>
          <EvidenceSelect label="Đầu việc" value={workItemId} onChange={setWorkItemId}>
            <option value="">Tất cả đầu việc</option>
            {options.work_items.map((option) => <option key={option.id} value={option.id}>{option.label}</option>)}
          </EvidenceSelect>
          <EvidenceSelect label="Người gửi" value={submittedBy} onChange={setSubmittedBy}>
            <option value="">Tất cả người gửi</option>
            {options.submitters.map((option) => <option key={option.id} value={option.id}>{option.label}</option>)}
          </EvidenceSelect>
          <EvidenceSelect label="Phạm vi" value={visibility} onChange={setVisibility}>
            <option value="">Tất cả phạm vi</option>
            <option value="internal">Nội bộ</option>
            <option value="project">Công trình</option>
            <option value="client">Chủ đầu tư</option>
          </EvidenceSelect>
          <div className="grid grid-cols-2 gap-2">
            <label className="min-w-0">
              <span className="text-[11px] font-semibold uppercase tracking-wide text-slate-500">Từ ngày</span>
              <input
                type="date"
                className="mt-1 w-full rounded-md border border-line bg-white px-2 py-2 text-xs outline-none focus:ring-2 focus:ring-blue-100"
                value={dateFrom}
                max={dateTo || undefined}
                onChange={(event) => setDateFrom(event.target.value)}
              />
            </label>
            <label className="min-w-0">
              <span className="text-[11px] font-semibold uppercase tracking-wide text-slate-500">Đến ngày</span>
              <input
                type="date"
                className="mt-1 w-full rounded-md border border-line bg-white px-2 py-2 text-xs outline-none focus:ring-2 focus:ring-blue-100"
                value={dateTo}
                min={dateFrom || undefined}
                onChange={(event) => setDateTo(event.target.value)}
              />
            </label>
          </div>
        </div>
      </div>

      {evidence.isPending ? (
        <ProjectEvidenceLoading />
      ) : evidence.error ? (
        <div className="flex flex-col gap-3 rounded-lg border border-rose-100 bg-rose-50 p-4 text-sm text-rose-700 sm:flex-row sm:items-center sm:justify-between">
          <div className="flex items-start gap-2">
            <AlertCircle className="mt-0.5 h-4 w-4 shrink-0" />
            <div>
              <p className="font-semibold">Không tải được hồ sơ công trình</p>
              <p className="mt-1 text-xs">{evidence.error instanceof Error ? evidence.error.message : 'Vui lòng thử lại.'}</p>
            </div>
          </div>
          <button type="button" className="rounded-md border border-rose-200 bg-white px-3 py-2 text-xs font-semibold hover:bg-rose-100" onClick={() => void evidence.refetch()}>
            Thử lại
          </button>
        </div>
      ) : (
        <EvidenceGallery
          evidence={rows}
          emptyTitle={hasActiveFilters ? 'Không có hồ sơ phù hợp bộ lọc' : 'Công trình chưa có minh chứng'}
          showWorkItem
          showControls={false}
          variant="list"
          listContext="project"
          onReview={(record, action, comment) => reviewEvidence.mutateAsync({ record, action, comment }).then(() => undefined)}
        />
      )}

      {!evidence.isPending && !evidence.error && filteredTotal > 0 ? (
        <div className="flex flex-col gap-2 rounded-md border border-line bg-slate-50 px-3 py-2 text-xs font-medium text-slate-600 sm:flex-row sm:items-center sm:justify-between">
          <span>Đang hiển thị {rows.length} / {filteredTotal} hồ sơ</span>
          {evidence.hasNextPage ? (
            <button
              type="button"
              className="inline-flex items-center justify-center gap-2 rounded-md border border-line bg-white px-3 py-2 font-semibold text-blue-700 hover:border-blue-300 hover:bg-blue-50 disabled:cursor-not-allowed disabled:opacity-60"
              disabled={evidence.isFetchingNextPage}
              onClick={() => void evidence.fetchNextPage()}
            >
              {evidence.isFetchingNextPage ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : null}
              Tải thêm
            </button>
          ) : null}
        </div>
      ) : null}
    </section>
  );
}

function EvidenceMetric({
  icon,
  label,
  value,
  active,
  tone = 'neutral',
  onClick,
}: {
  icon: ReactNode;
  label: string;
  value: number | string;
  active: boolean;
  tone?: 'neutral' | 'warn' | 'ok' | 'danger';
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      aria-pressed={active}
      className={clsx(
        'flex min-w-0 items-center justify-between gap-3 rounded-lg border px-3 py-3 text-left transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500',
        active ? 'border-blue-300 bg-blue-50 ring-1 ring-inset ring-blue-100' : 'border-line bg-white hover:border-blue-200 hover:bg-slate-50',
      )}
      onClick={onClick}
    >
      <span className="flex min-w-0 items-center gap-2">
        <span className={clsx(
          'grid h-8 w-8 shrink-0 place-items-center rounded-md',
          tone === 'warn' ? 'bg-amber-50 text-amber-700' : tone === 'ok' ? 'bg-emerald-50 text-emerald-700' : tone === 'danger' ? 'bg-rose-50 text-rose-700' : 'bg-slate-100 text-slate-600',
        )}>{icon}</span>
        <span className="truncate text-xs font-semibold text-slate-600">{label}</span>
      </span>
      <strong className="text-lg tabular-nums text-ink">{value}</strong>
    </button>
  );
}

function EvidenceSelect({
  label,
  value,
  onChange,
  children,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  children: ReactNode;
}) {
  return (
    <label className="min-w-0">
      <span className="text-[11px] font-semibold uppercase tracking-wide text-slate-500">{label}</span>
      <select
        className="mt-1 w-full rounded-md border border-line bg-white px-2.5 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-100"
        value={value}
        onChange={(event) => onChange(event.target.value)}
      >
        {children}
      </select>
    </label>
  );
}

function ProjectEvidenceLoading() {
  return (
    <div className="overflow-hidden rounded-md border border-line bg-white">
      <div className="h-9 animate-pulse border-b border-line bg-slate-100" />
      {Array.from({ length: 6 }, (_, index) => (
        <div key={index} className="flex items-center gap-3 border-b border-line px-3 py-3 last:border-b-0">
          <div className="h-12 w-14 animate-pulse rounded-md bg-slate-100" />
          <div className="min-w-0 flex-1 space-y-2">
            <div className="h-3.5 w-2/5 animate-pulse rounded bg-slate-100" />
            <div className="h-3 w-3/5 animate-pulse rounded bg-slate-100" />
          </div>
          <div className="hidden h-7 w-24 animate-pulse rounded bg-slate-100 lg:block" />
        </div>
      ))}
    </div>
  );
}
