import { ReactNode, useEffect, useMemo, useState } from 'react';
import clsx from 'clsx';
import { AlertCircle, ChevronDown, ChevronLeft, ChevronRight, ChevronUp, ChevronsUpDown, Search } from 'lucide-react';
import { EmptyState } from './EmptyState';

type SortDirection = 'asc' | 'desc';
type SortValue = string | number | boolean | Date | null | undefined;

export type Column<T> = {
  key: string;
  header: string;
  render?: (row: T) => ReactNode;
  sortable?: boolean;
  sortValue?: (row: T) => SortValue;
  filterValue?: (row: T) => unknown;
  align?: 'left' | 'center' | 'right';
  className?: string;
  headerClassName?: string;
};

export function DataTable<T extends Record<string, any>>({
  rows,
  columns,
  empty,
  loading = false,
  error,
  onRetry,
  searchPlaceholder,
  pageSize,
  pageSizeOptions,
  defaultSort,
}: {
  rows: T[];
  columns: Column<T>[];
  empty?: string;
  loading?: boolean;
  error?: unknown;
  onRetry?: () => void;
  searchPlaceholder?: string;
  pageSize?: number;
  pageSizeOptions?: number[];
  defaultSort?: { key: string; direction?: SortDirection };
}) {
  const [query, setQuery] = useState('');
  const [sort, setSort] = useState<{ key: string; direction: SortDirection } | null>(
    defaultSort ? { key: defaultSort.key, direction: defaultSort.direction ?? 'asc' } : null,
  );
  const [currentPage, setCurrentPage] = useState(1);
  const [currentPageSize, setCurrentPageSize] = useState(pageSize ?? 0);
  const enhanced = Boolean(searchPlaceholder || loading || error || onRetry || pageSize || defaultSort);

  useEffect(() => {
    setCurrentPageSize(pageSize ?? 0);
  }, [pageSize]);

  useEffect(() => {
    setSort(defaultSort ? { key: defaultSort.key, direction: defaultSort.direction ?? 'asc' } : null);
  }, [defaultSort?.key, defaultSort?.direction]);

  const filteredRows = useMemo(() => {
    const normalizedQuery = normalizeSearchText(query);
    if (!normalizedQuery) return rows;
    return rows.filter((row) => columns.some((column) => normalizeSearchText(searchValueForColumn(row, column)).includes(normalizedQuery)));
  }, [columns, query, rows]);

  const sortedRows = useMemo(() => {
    if (!sort) return filteredRows;
    const column = columns.find((item) => item.key === sort.key);
    if (!column) return filteredRows;
    return [...filteredRows].sort((left, right) => compareSortValues(sortValueForColumn(left, column), sortValueForColumn(right, column), sort.direction));
  }, [columns, filteredRows, sort]);

  const isPaginated = currentPageSize > 0;
  const pageCount = isPaginated ? Math.max(1, Math.ceil(sortedRows.length / currentPageSize)) : 1;
  const safePage = Math.min(currentPage, pageCount);
  const visibleRows = isPaginated ? sortedRows.slice((safePage - 1) * currentPageSize, safePage * currentPageSize) : sortedRows;
  const rangeStart = sortedRows.length ? (isPaginated ? (safePage - 1) * currentPageSize + 1 : 1) : 0;
  const rangeEnd = isPaginated ? Math.min(safePage * currentPageSize, sortedRows.length) : sortedRows.length;
  const hasControls = Boolean(searchPlaceholder || isPaginated);
  const resolvedPageSizeOptions = normalizedPageSizeOptions(pageSizeOptions, currentPageSize);

  useEffect(() => {
    setCurrentPage(1);
  }, [query, currentPageSize, rows.length, sort?.key, sort?.direction]);

  if (!enhanced && !rows.length) return <EmptyState title={empty} />;

  return (
    <div className="overflow-hidden rounded-lg border border-line bg-white shadow-panel">
      {hasControls ? (
        <div className="flex flex-col gap-3 border-b border-line bg-white px-4 py-3 sm:flex-row sm:items-center sm:justify-between">
          {searchPlaceholder ? (
            <label className="relative block w-full max-w-sm">
              <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" />
              <input
                className="w-full rounded-md border border-line py-2 pl-9 pr-3 text-sm outline-none focus:ring-2 focus:ring-blue-100"
                placeholder={searchPlaceholder}
                value={query}
                onChange={(event) => setQuery(event.target.value)}
              />
            </label>
          ) : <span />}
          <p className="whitespace-nowrap text-xs font-medium text-slate-500">
            {rangeStart}-{rangeEnd} / {sortedRows.length} dòng
          </p>
        </div>
      ) : null}

      {loading ? (
        <div className="flex items-center gap-3 px-4 py-6 text-sm text-slate-600">
          <div className="h-4 w-4 animate-spin rounded-full border-2 border-blue-600 border-t-transparent" />
          Đang tải dữ liệu...
        </div>
      ) : error ? (
        <div className="px-4 py-6">
          <div className="flex flex-col gap-3 rounded-md border border-rose-100 bg-rose-50 p-4 text-sm text-rose-700 sm:flex-row sm:items-center sm:justify-between">
            <div className="flex min-w-0 items-start gap-2">
              <AlertCircle className="mt-0.5 h-4 w-4 shrink-0" />
              <div>
                <p className="font-semibold">Không tải được dữ liệu</p>
                <p className="mt-1 text-xs">{errorMessage(error)}</p>
              </div>
            </div>
            {onRetry ? (
              <button className="rounded-md border border-rose-200 bg-white px-3 py-2 text-xs font-semibold text-rose-700 hover:bg-rose-100" onClick={onRetry}>
                Thử lại
              </button>
            ) : null}
          </div>
        </div>
      ) : visibleRows.length ? (
      <div className="overflow-x-auto">
        <table className="min-w-full divide-y divide-line">
          <thead className="bg-slate-50">
            <tr>
              {columns.map((column) => {
                const canSort = Boolean(column.sortable || column.sortValue);
                const activeSort = sort?.key === column.key ? sort.direction : null;
                return (
                <th
                  key={column.key}
                  aria-sort={activeSort === 'asc' ? 'ascending' : activeSort === 'desc' ? 'descending' : undefined}
                  className={clsx('px-4 py-3 text-xs font-semibold uppercase tracking-wide text-slate-500', alignClass(column.align), column.headerClassName)}
                >
                  {canSort ? (
                    <button
                      className={clsx('inline-flex items-center gap-1 rounded-sm hover:text-ink focus:outline-none focus:ring-2 focus:ring-blue-100', justifyClass(column.align))}
                      onClick={() => setSort(nextSort(column.key, sort))}
                      type="button"
                    >
                      <span>{column.header}</span>
                      {activeSort === 'asc' ? <ChevronUp className="h-3.5 w-3.5" /> : activeSort === 'desc' ? <ChevronDown className="h-3.5 w-3.5" /> : <ChevronsUpDown className="h-3.5 w-3.5 text-slate-400" />}
                    </button>
                  ) : column.header}
                </th>
                );
              })}
            </tr>
          </thead>
          <tbody className="divide-y divide-line">
            {visibleRows.map((row, index) => (
              <tr key={row.id ?? index} className="hover:bg-slate-50">
                {columns.map((column) => (
                  <td key={column.key} className={clsx('px-4 py-3 text-sm text-slate-700', alignClass(column.align), column.className)}>
                    {column.render ? column.render(row) : row[column.key] ?? '-'}
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      ) : (
        <div className="p-4">
          <EmptyState title={query.trim() ? 'Không có dòng phù hợp' : empty} />
        </div>
      )}

      {isPaginated && !loading && !error ? (
        <div className="flex flex-col gap-3 border-t border-line bg-slate-50 px-4 py-3 text-xs font-medium text-slate-600 sm:flex-row sm:items-center sm:justify-between">
          <div className="flex items-center gap-2">
            <span>Dòng/trang</span>
            <select
              className="rounded-md border border-line bg-white px-2 py-1 outline-none focus:ring-2 focus:ring-blue-100"
              value={currentPageSize}
              onChange={(event) => setCurrentPageSize(Number(event.target.value))}
            >
              {resolvedPageSizeOptions.map((option) => <option key={option} value={option}>{option}</option>)}
            </select>
          </div>
          <div className="flex items-center justify-end gap-2">
            <span>Trang {safePage} / {pageCount}</span>
            <button
              className="rounded-md border border-line bg-white p-1.5 text-slate-600 hover:bg-slate-100 disabled:cursor-not-allowed disabled:opacity-50"
              disabled={safePage <= 1}
              title="Trang trước"
              onClick={() => setCurrentPage((page) => Math.max(1, page - 1))}
            >
              <ChevronLeft className="h-4 w-4" />
            </button>
            <button
              className="rounded-md border border-line bg-white p-1.5 text-slate-600 hover:bg-slate-100 disabled:cursor-not-allowed disabled:opacity-50"
              disabled={safePage >= pageCount}
              title="Trang sau"
              onClick={() => setCurrentPage((page) => Math.min(pageCount, page + 1))}
            >
              <ChevronRight className="h-4 w-4" />
            </button>
          </div>
        </div>
      ) : null}
    </div>
  );
}

function nextSort(key: string, current: { key: string; direction: SortDirection } | null): { key: string; direction: SortDirection } {
  if (current?.key !== key) return { key, direction: 'asc' as SortDirection };
  return { key, direction: current.direction === 'asc' ? 'desc' : 'asc' };
}

function searchValueForColumn<T extends Record<string, any>>(row: T, column: Column<T>) {
  return column.filterValue ? column.filterValue(row) : row[column.key];
}

function sortValueForColumn<T extends Record<string, any>>(row: T, column: Column<T>) {
  return column.sortValue ? column.sortValue(row) : row[column.key];
}

function compareSortValues(left: SortValue, right: SortValue, direction: SortDirection) {
  const leftEmpty = left === null || left === undefined || left === '';
  const rightEmpty = right === null || right === undefined || right === '';
  if (leftEmpty && rightEmpty) return 0;
  if (leftEmpty) return 1;
  if (rightEmpty) return -1;

  const normalizedLeft = normalizeSortValue(left);
  const normalizedRight = normalizeSortValue(right);
  const result = typeof normalizedLeft === 'number' && typeof normalizedRight === 'number'
    ? normalizedLeft - normalizedRight
    : String(normalizedLeft).localeCompare(String(normalizedRight), 'vi');

  return direction === 'asc' ? result : -result;
}

function normalizeSortValue(value: SortValue) {
  if (value instanceof Date) return value.getTime();
  if (typeof value === 'boolean') return value ? 1 : 0;
  if (typeof value === 'number') return value;
  return String(value ?? '').toLocaleLowerCase('vi-VN');
}

function normalizeSearchText(value: unknown): string {
  if (Array.isArray(value)) return value.map(normalizeSearchText).join(' ');
  return String(value ?? '').toLocaleLowerCase('vi-VN');
}

function alignClass(align: Column<Record<string, any>>['align']) {
  if (align === 'right') return 'text-right';
  if (align === 'center') return 'text-center';
  return 'text-left';
}

function justifyClass(align: Column<Record<string, any>>['align']) {
  if (align === 'right') return 'justify-end';
  if (align === 'center') return 'justify-center';
  return 'justify-start';
}

function normalizedPageSizeOptions(options: number[] | undefined, currentPageSize: number) {
  const defaults = options?.length ? options : [10, 25, 50];
  return [...new Set([...defaults, currentPageSize].filter((option) => option > 0))].sort((left, right) => left - right);
}

function errorMessage(error: unknown) {
  if (error instanceof Error) return error.message;
  if (typeof error === 'string') return error;
  return 'Vui lòng thử lại hoặc kiểm tra kết nối API.';
}
