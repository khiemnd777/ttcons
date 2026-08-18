import { useEffect, useMemo, useState } from 'react';
import { Check, ChevronDown, ChevronUp, Loader2, RefreshCw, Search } from 'lucide-react';
import clsx from 'clsx';
import { Dialog } from '../../components/Dialog';
import { EmptyState } from '../../components/EmptyState';
import { StatusBadge } from '../../components/StatusBadge';
import type { WorkItem } from '../../lib/types/domain';
import { groupAcceptanceWorkItems } from './acceptanceWorkItemPicker';

export function AcceptanceWorkItemPickerDialog({
  open,
  items,
  selectedId,
  requiredWorkItemId = '',
  loading = false,
  error = false,
  confirmLabel = 'Dùng công việc này',
  canClear = false,
  onClose,
  onRetry,
  onClear,
  onConfirm,
}: {
  open: boolean;
  items: WorkItem[];
  selectedId: string;
  requiredWorkItemId?: string;
  loading?: boolean;
  error?: boolean;
  confirmLabel?: string;
  canClear?: boolean;
  onClose: () => void;
  onRetry?: () => void;
  onClear?: () => void;
  onConfirm: (workItemId: string) => void;
}) {
  const [query, setQuery] = useState('');
  const [selection, setSelection] = useState('');
  const [showAll, setShowAll] = useState(false);

  useEffect(() => {
    if (!open) return;
    setQuery('');
    setSelection(requiredWorkItemId || selectedId);
    setShowAll(false);
  }, [open, requiredWorkItemId, selectedId]);

  const groups = useMemo(() => groupAcceptanceWorkItems(items, query, selection), [items, query, selection]);
  const selectableIDs = useMemo(() => new Set(items.map((item) => item.id)), [items]);
  const showOtherItems = showAll || Boolean(query.trim());
  const canConfirm = Boolean(selection) && selectableIDs.has(selection) && (!requiredWorkItemId || selection === requiredWorkItemId);

  return (
    <Dialog
      open={open}
      title="Chọn nơi lưu minh chứng"
      description="Ảnh và tệp mới cần gắn với một công việc để truy vết và kiểm soát quyền truy cập."
      size="lg"
      onClose={onClose}
      footer={(
        <div className="flex flex-col-reverse gap-2 sm:flex-row sm:items-center sm:justify-between">
          <div>{canClear && onClear ? <button type="button" className="rounded-md px-3 py-2 text-sm font-semibold text-slate-600 hover:bg-white hover:text-rose-700" onClick={onClear}>Bỏ liên kết nguồn</button> : null}</div>
          <div className="flex justify-end gap-2">
            <button type="button" className="rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100" onClick={onClose}>Hủy</button>
            <button type="button" disabled={!canConfirm} className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-4 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-50" onClick={() => onConfirm(selection)}><Check className="h-4 w-4" /> {confirmLabel}</button>
          </div>
        </div>
      )}
    >
      <div className="space-y-4">
        <label className="relative block">
          <span className="sr-only">Tìm công việc</span>
          <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" />
          <input autoFocus className="h-10 w-full rounded-md border border-line bg-white pl-9 pr-3 text-sm outline-none focus:border-blue-300 focus:ring-2 focus:ring-blue-100" value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Tìm tên công việc, giai đoạn, người phụ trách..." />
        </label>

        {requiredWorkItemId ? <p className="rounded-md border border-blue-200 bg-blue-50 px-3 py-2 text-xs leading-5 text-blue-800">Minh chứng đã chọn đang thuộc một công việc cụ thể. Ảnh/tệp mới phải tiếp tục được lưu vào cùng công việc đó.</p> : null}

        {loading ? <div className="grid min-h-52 place-items-center"><span className="inline-flex items-center gap-2 text-sm text-slate-500"><Loader2 className="h-5 w-5 animate-spin text-blue-600" /> Đang tải danh sách công việc...</span></div> : null}
        {!loading && error ? (
          <div className="rounded-md border border-rose-200 bg-rose-50 p-4 text-sm text-rose-800">
            <p className="font-semibold">Không tải được danh sách công việc</p>
            <p className="mt-1 text-xs leading-5">Anh vẫn có thể đóng dialog và lưu nháp nếu chưa cần tải ảnh/tệp mới.</p>
            {onRetry ? <button type="button" className="mt-3 inline-flex items-center gap-2 rounded-md border border-rose-300 bg-white px-3 py-2 text-sm font-semibold hover:bg-rose-100" onClick={onRetry}><RefreshCw className="h-4 w-4" /> Thử lại</button> : null}
          </div>
        ) : null}

        {!loading && !error ? (
          <>
            <WorkItemGroup title="Đề xuất" items={groups.suggested} selection={selection} requiredWorkItemId={requiredWorkItemId} onSelect={setSelection} />

            {groups.others.length ? (
              <section>
                <button type="button" className="flex w-full items-center justify-between rounded-md border border-line bg-slate-50 px-3 py-2 text-left text-sm font-semibold text-slate-700 hover:bg-slate-100" aria-expanded={showOtherItems} onClick={() => setShowAll((current) => !current)}>
                  <span>Công việc khác ({groups.others.length})</span>
                  {showOtherItems ? <ChevronUp className="h-4 w-4" /> : <ChevronDown className="h-4 w-4" />}
                </button>
                {showOtherItems ? <div className="mt-2"><WorkItemList items={groups.others} selection={selection} requiredWorkItemId={requiredWorkItemId} onSelect={setSelection} /></div> : null}
              </section>
            ) : null}

            {!groups.suggested.length && !groups.others.length ? <EmptyState title={query.trim() ? 'Không tìm thấy công việc phù hợp' : 'Chưa có công việc có thể liên kết'} /> : null}
          </>
        ) : null}
      </div>
    </Dialog>
  );
}

function WorkItemGroup({ title, items, selection, requiredWorkItemId, onSelect }: { title: string; items: WorkItem[]; selection: string; requiredWorkItemId: string; onSelect: (id: string) => void }) {
  if (!items.length) return <p className="rounded-md border border-dashed border-line px-3 py-4 text-center text-xs text-slate-500">Chưa có công việc đang xử lý hoặc sẵn sàng.</p>;
  return <section><h3 className="mb-2 text-xs font-semibold uppercase tracking-wide text-slate-500">{title}</h3><WorkItemList items={items} selection={selection} requiredWorkItemId={requiredWorkItemId} onSelect={onSelect} /></section>;
}

function WorkItemList({ items, selection, requiredWorkItemId, onSelect }: { items: WorkItem[]; selection: string; requiredWorkItemId: string; onSelect: (id: string) => void }) {
  return (
    <div className="max-h-80 space-y-2 overflow-y-auto pr-1" role="listbox" aria-label="Danh sách công việc">
      {items.map((item) => {
        const selected = selection === item.id;
        const disabled = Boolean(requiredWorkItemId && requiredWorkItemId !== item.id);
        const context = [item.run_node_name, item.assignee_name ? `Phụ trách: ${item.assignee_name}` : ''].filter(Boolean).join(' · ');
        return (
          <button
            key={item.id}
            type="button"
            role="option"
            aria-selected={selected}
            disabled={disabled}
            className={clsx('flex w-full items-start gap-3 rounded-lg border p-3 text-left transition disabled:cursor-not-allowed disabled:opacity-45', selected ? 'border-blue-300 bg-blue-50 ring-1 ring-blue-200' : 'border-line bg-white hover:border-blue-200 hover:bg-slate-50')}
            onClick={() => onSelect(item.id)}
          >
            <span className={clsx('mt-0.5 grid h-7 w-7 shrink-0 place-items-center rounded-full', selected ? 'bg-blue-600 text-white' : 'bg-slate-100 text-slate-400')}>{selected ? <Check className="h-4 w-4" /> : <span className="h-2 w-2 rounded-full bg-current" />}</span>
            <span className="min-w-0 flex-1">
              <span className="block text-sm font-semibold text-ink">{item.title || item.name}</span>
              {context ? <span className="mt-1 block text-xs leading-5 text-slate-500">{context}</span> : null}
              {disabled ? <span className="mt-1 block text-xs font-medium text-amber-700">Không cùng nguồn với minh chứng đã chọn</span> : null}
            </span>
            <StatusBadge status={item.status} className="shrink-0" />
          </button>
        );
      })}
    </div>
  );
}
