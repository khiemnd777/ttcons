import { useEffect, useId, useMemo, useRef, useState } from 'react';
import { useInfiniteQuery } from '@tanstack/react-query';
import { Check, ChevronDown, Loader2, Search } from 'lucide-react';
import clsx from 'clsx';
import { get } from '../../lib/api/client';
import type { ReportOptions, ReportProjectOption } from '../../lib/types/domain';

const PROJECT_PAGE_SIZE = 20;

export function ReportProjectPicker({
  value,
  onChange,
  disabled = false,
}: {
  value: string;
  onChange: (value: string) => void;
  disabled?: boolean;
}) {
  const [open, setOpen] = useState(false);
  const [search, setSearch] = useState('');
  const [deferredSearch, setDeferredSearch] = useState('');
  const [activeIndex, setActiveIndex] = useState(0);
  const [selectedCache, setSelectedCache] = useState<ReportProjectOption | null>(null);
  const rootRef = useRef<HTMLDivElement>(null);
  const searchRef = useRef<HTMLInputElement>(null);
  const listboxID = useId();

  useEffect(() => {
    const timeoutID = window.setTimeout(() => setDeferredSearch(search.trim()), 300);
    return () => window.clearTimeout(timeoutID);
  }, [search]);

  const projects = useInfiniteQuery({
    queryKey: ['report-project-options', deferredSearch, value],
    initialPageParam: '',
    queryFn: ({ pageParam }) => {
      const params = new URLSearchParams();
      params.set('limit', String(PROJECT_PAGE_SIZE));
      if (value) params.set('selected_id', value);
      if (deferredSearch) params.set('search', deferredSearch);
      if (pageParam) params.set('cursor', String(pageParam));
      return get<ReportOptions>(`/reports/options?${params.toString()}`);
    },
    getNextPageParam: (page) => page.projects.has_more && page.projects.next_cursor
      ? page.projects.next_cursor
      : undefined,
    enabled: open || Boolean(value),
    staleTime: 5 * 60 * 1000,
    retry: false,
  });

  const rows = useMemo(() => {
    const unique = new Map<string, ReportProjectOption>();
    for (const page of projects.data?.pages ?? []) {
      for (const project of page.projects.items ?? []) unique.set(project.id, project);
    }
    return Array.from(unique.values());
  }, [projects.data?.pages]);

  const serverSelected = projects.data?.pages[0]?.projects.selected ?? null;
  const selectedProject = serverSelected?.id === value
    ? serverSelected
    : selectedCache?.id === value
      ? selectedCache
      : rows.find((project) => project.id === value) ?? null;
  const selectedUnavailable = Boolean(value) && projects.isSuccess && !selectedProject;

  useEffect(() => {
    if (serverSelected?.id === value) setSelectedCache(serverSelected);
  }, [serverSelected, value]);

  useEffect(() => {
    if (!open) return;
    const handlePointerDown = (event: PointerEvent) => {
      if (!rootRef.current?.contains(event.target as Node)) closePicker();
    };
    document.addEventListener('pointerdown', handlePointerDown);
    return () => document.removeEventListener('pointerdown', handlePointerDown);
  }, [open]);

  useEffect(() => {
    if (!open) return;
    window.requestAnimationFrame(() => searchRef.current?.focus());
  }, [open]);

  useEffect(() => {
    if (!open) return;
    setActiveIndex((current) => Math.min(current, rows.length));
  }, [open, rows.length]);

  function closePicker() {
    setOpen(false);
    setSearch('');
    setDeferredSearch('');
    setActiveIndex(0);
  }

  function selectProject(project: ReportProjectOption | null) {
    setSelectedCache(project);
    onChange(project?.id ?? '');
    closePicker();
  }

  function handleSearchKeyDown(event: React.KeyboardEvent<HTMLInputElement>) {
    if (event.key === 'Escape') {
      event.preventDefault();
      closePicker();
      return;
    }
    if (event.key === 'ArrowDown') {
      event.preventDefault();
      setActiveIndex((current) => Math.min(current + 1, rows.length));
      return;
    }
    if (event.key === 'ArrowUp') {
      event.preventDefault();
      setActiveIndex((current) => Math.max(current - 1, 0));
      return;
    }
    if (event.key === 'Enter') {
      event.preventDefault();
      selectProject(activeIndex === 0 ? null : rows[activeIndex - 1] ?? null);
    }
  }

  const displayText = !value
    ? 'Tất cả công trình được phép xem'
    : selectedProject
      ? `${selectedProject.code} · ${selectedProject.name}`
      : projects.isLoading
        ? 'Đang tải công trình đã chọn…'
        : selectedUnavailable
          ? 'Công trình không còn quyền truy cập'
          : 'Không tải được tên công trình';

  return (
    <div ref={rootRef} className="relative block min-w-0 w-full sm:flex-1">
      <div className="block">
        <span className="text-xs font-semibold text-slate-600">Công trình</span>
        <button
          type="button"
          role="combobox"
          aria-label={`Công trình: ${displayText}`}
          aria-expanded={open}
          aria-controls={listboxID}
          aria-haspopup="listbox"
          className={clsx(
            'mt-1 flex h-10 w-full items-center gap-2 rounded-md border bg-white px-3 text-left text-sm outline-none focus:ring-2 focus:ring-blue-100 disabled:cursor-not-allowed disabled:bg-slate-100 disabled:text-slate-500',
            selectedUnavailable ? 'border-amber-300 text-amber-800' : 'border-line text-ink',
          )}
          onClick={() => {
            if (open) closePicker();
            else setOpen(true);
          }}
          onKeyDown={(event) => {
            if (event.key === 'ArrowDown' && !open) {
              event.preventDefault();
              setOpen(true);
            }
            if (event.key === 'Escape' && open) {
              event.preventDefault();
              closePicker();
            }
          }}
          disabled={disabled}
          title={displayText}
        >
          {projects.isLoading && value ? <Loader2 className="h-4 w-4 shrink-0 animate-spin text-blue-600" /> : null}
          {value && selectedProject ? (
            <span className="flex min-w-0 flex-1 items-center gap-2">
              <span className="shrink-0 rounded bg-slate-100 px-1.5 py-0.5 text-[11px] font-semibold uppercase tracking-wide text-slate-600">
                {selectedProject.code}
              </span>
              <span className="min-w-0 flex-1 truncate font-medium text-ink">{selectedProject.name}</span>
            </span>
          ) : (
            <span className="min-w-0 flex-1 truncate">{displayText}</span>
          )}
          <ChevronDown className={clsx('h-4 w-4 shrink-0 text-slate-400 transition-transform', open && 'rotate-180')} />
        </button>
      </div>

      {selectedUnavailable ? <p className="mt-1 text-xs font-medium text-amber-700">Không còn quyền xem công trình trong URL. Hãy chọn công trình khác hoặc Đặt lại.</p> : null}

      {open ? (
        <div className="absolute left-0 top-full z-50 mt-1 w-full overflow-hidden rounded-md border border-line bg-white shadow-lg">
          <div className="border-b border-line p-2">
            <label className="relative block">
              <span className="sr-only">Tìm theo mã hoặc tên công trình</span>
              <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" />
              <input
                ref={searchRef}
                role="combobox"
                aria-autocomplete="list"
                aria-controls={listboxID}
                aria-expanded="true"
                className="h-10 w-full rounded-md border border-line bg-white pl-9 pr-3 text-sm outline-none focus:ring-2 focus:ring-blue-100"
                value={search}
                onChange={(event) => {
                  setSearch(event.target.value);
                  setActiveIndex(0);
                }}
                onKeyDown={handleSearchKeyDown}
                placeholder="Tìm theo mã hoặc tên công trình..."
              />
            </label>
          </div>

          <div id={listboxID} role="listbox" aria-label="Danh sách công trình" className="max-h-80 overflow-y-auto p-1">
            <button
              type="button"
              role="option"
              aria-selected={!value}
              className={clsx(
                'flex w-full items-center gap-2 rounded px-3 py-2 text-left text-sm',
                activeIndex === 0 ? 'bg-blue-50 text-blue-800' : 'text-slate-700 hover:bg-slate-50',
              )}
              onMouseEnter={() => setActiveIndex(0)}
              onClick={() => selectProject(null)}
            >
              <span className="min-w-0 flex-1">Tất cả công trình được phép xem</span>
              {!value ? <Check className="h-4 w-4 shrink-0 text-blue-600" /> : null}
            </button>

            {projects.isLoading ? <div className="grid min-h-28 place-items-center"><Loader2 className="h-5 w-5 animate-spin text-blue-600" /></div> : null}
            {projects.isError ? (
              <div className="p-3 text-center">
                <p className="text-sm text-red-700">Không tải được danh sách công trình.</p>
                <button type="button" className="mt-2 rounded-md border border-line px-3 py-1.5 text-sm font-semibold text-slate-700 hover:bg-slate-50" onClick={() => void projects.refetch()}>Thử lại</button>
              </div>
            ) : null}
            {!projects.isLoading && !projects.isError && rows.length === 0 ? <p className="px-3 py-6 text-center text-sm text-slate-500">Không có công trình phù hợp.</p> : null}

            {rows.map((project, index) => {
              const optionIndex = index + 1;
              const selected = project.id === value;
              return (
                <button
                  key={project.id}
                  type="button"
                  role="option"
                  aria-selected={selected}
                  className={clsx(
                    'flex w-full items-center gap-2 rounded px-3 py-2 text-left',
                    activeIndex === optionIndex ? 'bg-blue-50' : 'hover:bg-slate-50',
                  )}
                  onMouseEnter={() => setActiveIndex(optionIndex)}
                  onClick={() => selectProject(project)}
                >
                  <span className="min-w-0 flex-1">
                    <span className="block truncate text-xs font-semibold uppercase tracking-wide text-slate-500">{project.code}</span>
                    <span className="mt-0.5 block truncate text-sm font-medium text-ink">{project.name}</span>
                  </span>
                  {selected ? <Check className="h-4 w-4 shrink-0 text-blue-600" /> : null}
                </button>
              );
            })}

            {projects.hasNextPage ? (
              <div className="border-t border-line p-2 text-center">
                <button
                  type="button"
                  disabled={projects.isFetchingNextPage}
                  className="inline-flex items-center gap-2 rounded-md border border-line bg-white px-3 py-1.5 text-sm font-semibold text-slate-700 hover:bg-slate-50 disabled:opacity-60"
                  onClick={() => void projects.fetchNextPage()}
                >
                  {projects.isFetchingNextPage ? <Loader2 className="h-4 w-4 animate-spin" /> : null}
                  <span>Tải thêm</span>
                </button>
              </div>
            ) : null}
          </div>
        </div>
      ) : null}
    </div>
  );
}
