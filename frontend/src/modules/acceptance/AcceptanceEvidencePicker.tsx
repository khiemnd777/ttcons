import { useDeferredValue, useEffect, useMemo, useState } from 'react';
import { useInfiniteQuery } from '@tanstack/react-query';
import { Check, FileText, Image as ImageIcon, Loader2, Search } from 'lucide-react';
import clsx from 'clsx';
import { Dialog } from '../../components/Dialog';
import { EmptyState } from '../../components/EmptyState';
import { StatusBadge } from '../../components/StatusBadge';
import { get } from '../../lib/api/client';
import type { EvidenceRecord, ProjectEvidencePage } from '../../lib/types/domain';

export function AcceptanceEvidencePicker({
  open,
  projectId,
  workItemId,
  selected,
  maxSelected = 10,
  onClose,
  onConfirm,
}: {
  open: boolean;
  projectId: string;
  workItemId?: string;
  selected: EvidenceRecord[];
  maxSelected?: number;
  onClose: () => void;
  onConfirm: (records: EvidenceRecord[]) => void;
}) {
  const [search, setSearch] = useState('');
  const [evidenceType, setEvidenceType] = useState('');
  const [status, setStatus] = useState('');
  const [selection, setSelection] = useState<Map<string, EvidenceRecord>>(new Map());
  const deferredSearch = useDeferredValue(search.trim());

  useEffect(() => {
    if (!open) return;
    setSelection(new Map(selected.map((record) => [String(record.id), record])));
  }, [open, selected]);

  const evidence = useInfiniteQuery({
    queryKey: ['acceptance-evidence-picker', projectId, workItemId ?? '', evidenceType, status, deferredSearch],
    initialPageParam: '',
    queryFn: ({ pageParam }) => {
      const params = new URLSearchParams();
      if (workItemId) params.set('work_item_id', workItemId);
      if (evidenceType) params.set('evidence_type', evidenceType);
      if (status) params.set('status', status);
      if (deferredSearch) params.set('search', deferredSearch);
      if (pageParam) params.set('cursor', String(pageParam));
      params.set('limit', '25');
      return get<ProjectEvidencePage>(`/projects/${projectId}/evidence?${params.toString()}`);
    },
    getNextPageParam: (page) => page.has_more && page.next_cursor ? page.next_cursor : undefined,
    enabled: open && Boolean(projectId),
    retry: false,
  });

  const rows = useMemo(
    () => (evidence.data?.pages ?? []).flatMap((page) => page.items ?? []).filter((record) => record.status !== 'rejected'),
    [evidence.data?.pages],
  );
  const filteredTotal = Number(evidence.data?.pages[0]?.filtered_total ?? 0);

  function toggle(record: EvidenceRecord) {
    const id = String(record.id);
    setSelection((current) => {
      const next = new Map(current);
      if (next.has(id)) {
        next.delete(id);
        return next;
      }
      if (next.size >= maxSelected) return current;
      next.set(id, record);
      return next;
    });
  }

  return (
    <Dialog
      open={open}
      title="Chọn từ hồ sơ công trình"
      description={workItemId ? 'Chỉ hiển thị minh chứng thuộc Công việc liên quan của biên bản.' : 'Tìm và chọn minh chứng đã có trong công trình.'}
      size="xl"
      onClose={onClose}
      footer={(
        <div className="flex flex-col-reverse gap-2 sm:flex-row sm:items-center sm:justify-between">
          <p className="text-xs font-medium text-slate-500">Đã chọn {selection.size}/{maxSelected} minh chứng</p>
          <div className="flex justify-end gap-2">
            <button type="button" className="rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100" onClick={onClose}>Hủy</button>
            <button type="button" className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-4 py-2 text-sm font-semibold text-white hover:bg-blue-700" onClick={() => onConfirm(Array.from(selection.values()))}><Check className="h-4 w-4" /> Dùng minh chứng đã chọn</button>
          </div>
        </div>
      )}
    >
      <div className="space-y-3">
        <div className="grid gap-2 md:grid-cols-[1fr_180px_180px]">
          <label className="relative block">
            <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" />
            <input className="h-10 w-full rounded-md border border-line bg-white pl-9 pr-3 text-sm outline-none focus:ring-2 focus:ring-blue-100" value={search} onChange={(event) => setSearch(event.target.value)} placeholder="Tìm tên, mô tả, công việc..." />
          </label>
          <select className="h-10 rounded-md border border-line bg-white px-3 text-sm outline-none focus:ring-2 focus:ring-blue-100" value={evidenceType} onChange={(event) => setEvidenceType(event.target.value)}>
            <option value="">Tất cả loại</option>
            <option value="photo">Ảnh</option>
            <option value="document">Tài liệu</option>
            <option value="checklist">Checklist</option>
            <option value="link">Liên kết</option>
            <option value="note">Ghi chú</option>
            <option value="other">Khác</option>
          </select>
          <select className="h-10 rounded-md border border-line bg-white px-3 text-sm outline-none focus:ring-2 focus:ring-blue-100" value={status} onChange={(event) => setStatus(event.target.value)}>
            <option value="">Trạng thái hợp lệ</option>
            <option value="waiting">Cần rà soát</option>
            <option value="accepted">Đã chấp nhận</option>
          </select>
        </div>

        <div className="flex items-center justify-between gap-3 text-xs text-slate-500">
          <span>{filteredTotal} kết quả</span>
          {selection.size >= maxSelected ? <span className="font-semibold text-amber-700">Đã đạt giới hạn {maxSelected} minh chứng</span> : null}
        </div>

        {evidence.isLoading ? <div className="grid min-h-56 place-items-center"><Loader2 className="h-6 w-6 animate-spin text-blue-600" /></div> : null}
        {evidence.error ? <div className="rounded-md border border-red-200 bg-red-50 p-4 text-sm text-red-700">Không tải được hồ sơ công trình. Vui lòng thử lại.</div> : null}
        {!evidence.isLoading && !evidence.error && !rows.length ? <EmptyState title="Không có minh chứng phù hợp" /> : null}
        {rows.length ? (
          <div className="grid gap-2 md:grid-cols-2">
            {rows.map((record) => {
              const id = String(record.id);
              const checked = selection.has(id);
              const disabled = !checked && selection.size >= maxSelected;
              const Icon = record.evidence_type === 'photo' ? ImageIcon : FileText;
              return (
                <button
                  key={id}
                  type="button"
                  disabled={disabled}
                  className={clsx(
                    'flex min-w-0 items-start gap-3 rounded-lg border p-3 text-left transition disabled:cursor-not-allowed disabled:opacity-50',
                    checked ? 'border-blue-300 bg-blue-50 ring-1 ring-blue-200' : 'border-line bg-white hover:border-blue-200 hover:bg-slate-50',
                  )}
                  onClick={() => toggle(record)}
                >
                  <span className={clsx('mt-0.5 grid h-8 w-8 shrink-0 place-items-center rounded-md', checked ? 'bg-blue-600 text-white' : 'bg-slate-100 text-slate-500')}>{checked ? <Check className="h-4 w-4" /> : <Icon className="h-4 w-4" />}</span>
                  <span className="min-w-0 flex-1">
                    <span className="block truncate text-sm font-semibold text-ink">{record.name || record.title || id}</span>
                    <span className="mt-1 block truncate text-xs text-slate-500">{record.work_item_title || 'Công việc không xác định'}</span>
                    {record.caption || record.description ? <span className="mt-1 line-clamp-2 block text-xs leading-5 text-slate-500">{record.caption || record.description}</span> : null}
                  </span>
                  <StatusBadge status={record.status} className="shrink-0" />
                </button>
              );
            })}
          </div>
        ) : null}

        {evidence.hasNextPage ? <div className="flex justify-center"><button type="button" disabled={evidence.isFetchingNextPage} className="inline-flex items-center gap-2 rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-50 disabled:opacity-60" onClick={() => evidence.fetchNextPage()}>{evidence.isFetchingNextPage ? <Loader2 className="h-4 w-4 animate-spin" /> : null} Tải thêm</button></div> : null}
      </div>
    </Dialog>
  );
}
