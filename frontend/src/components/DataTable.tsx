import { ReactNode, useEffect, useMemo, useState } from 'react';
import clsx from 'clsx';
import { AlertCircle, ChevronDown, ChevronLeft, ChevronRight, ChevronUp, ChevronsUpDown, Search } from 'lucide-react';
import { EmptyState } from './EmptyState';
import { actionColumnsFirst, isActionColumn } from './tableActionColumns';
import { SelectionCheckbox } from './SelectionCheckbox';

export { actionColumnsFirst, isActionColumn } from './tableActionColumns';

export type SortDirection = 'asc' | 'desc';
export type DataTableSort = { key: string; direction: SortDirection };
export type DataTableRowKey = string | number;
export type DataTableSelection = {
  selectedKeys: ReadonlySet<DataTableRowKey>;
  onSelectedKeysChange: (keys: Set<DataTableRowKey>) => void;
  ariaLabel?: string;
};
type SortValue = string | number | boolean | Date | null | undefined;

export type Column<T> = {
  key: string;
  header: string;
  action?: boolean;
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
  emptyAction,
  loading = false,
  error,
  onRetry,
  searchPlaceholder,
  searchQuery,
  onSearchQueryChange,
  pageSize,
  pageSizeOptions,
  defaultSort,
  sortState,
  onSortStateChange,
  getRowKey,
  renderMobileCard,
  selection,
}: {
  rows: T[];
  columns: Column<T>[];
  empty?: string;
  emptyAction?: ReactNode;
  loading?: boolean;
  error?: unknown;
  onRetry?: () => void;
  searchPlaceholder?: string;
  searchQuery?: string;
  onSearchQueryChange?: (value: string) => void;
  pageSize?: number;
  pageSizeOptions?: number[];
  defaultSort?: { key: string; direction?: SortDirection };
  sortState?: DataTableSort | null;
  onSortStateChange?: (sort: DataTableSort | null) => void;
  getRowKey?: (row: T, index: number) => string | number;
  renderMobileCard?: (row: T) => ReactNode;
  selection?: DataTableSelection;
}) {
  const [internalQuery, setInternalQuery] = useState('');
  const query = searchQuery ?? internalQuery;
  const [internalSort, setInternalSort] = useState<DataTableSort | null>(
    defaultSort ? { key: defaultSort.key, direction: defaultSort.direction ?? 'asc' } : null,
  );
  const sort = sortState === undefined ? internalSort : sortState;
  const updateSort = (next: DataTableSort | null) => {
    if (onSortStateChange) onSortStateChange(next);
    else setInternalSort(next);
  };
  const [currentPage, setCurrentPage] = useState(1);
  const [currentPageSize, setCurrentPageSize] = useState(pageSize ?? 0);
  const enhanced = Boolean(searchPlaceholder || loading || error || onRetry || pageSize || defaultSort || emptyAction);

  useEffect(() => {
    setCurrentPageSize(pageSize ?? 0);
  }, [pageSize]);

  useEffect(() => {
    if (sortState === undefined) setInternalSort(defaultSort ? { key: defaultSort.key, direction: defaultSort.direction ?? 'asc' } : null);
  }, [defaultSort?.key, defaultSort?.direction, sortState]);

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
  const sortableColumns = columns.filter((column) => column.sortable || column.sortValue);
  const mobileSortValue = sort ? `${sort.key}:${sort.direction}` : '';
  const orderedColumns = useMemo(() => actionColumnsFirst(columns), [columns]);
  const rowKey = (row: T, index: number): DataTableRowKey => getRowKey?.(row, index) ?? row.id ?? index;
  const visibleRowKeys = visibleRows.map(rowKey);
  const selectedVisibleCount = selection ? visibleRowKeys.filter((key) => selection.selectedKeys.has(key)).length : 0;
  const allVisibleSelected = Boolean(selection && visibleRowKeys.length && selectedVisibleCount === visibleRowKeys.length);
  const someVisibleSelected = Boolean(selection && selectedVisibleCount > 0 && !allVisibleSelected);

  useEffect(() => {
    setCurrentPage(1);
  }, [query, currentPageSize, rows.length, sort?.key, sort?.direction]);

  useEffect(() => {
    if (selection?.selectedKeys.size) selection.onSelectedKeysChange(new Set());
  }, [query, currentPageSize, currentPage, rows.length, sort?.key, sort?.direction]);

  if (!enhanced && !rows.length) return <EmptyState title={empty} action={emptyAction} />;

  const clearSearch = () => {
    if (onSearchQueryChange) onSearchQueryChange('');
    else setInternalQuery('');
  };

  const toggleVisibleRows = () => {
    if (!selection) return;
    const next = new Set(selection.selectedKeys);
    if (allVisibleSelected) visibleRowKeys.forEach((key) => next.delete(key));
    else visibleRowKeys.forEach((key) => next.add(key));
    selection.onSelectedKeysChange(next);
  };

  const toggleRow = (key: DataTableRowKey) => {
    if (!selection) return;
    const next = new Set(selection.selectedKeys);
    if (next.has(key)) next.delete(key);
    else next.add(key);
    selection.onSelectedKeysChange(next);
  };

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
                onChange={(event) => {
                  const value = event.target.value;
                  if (onSearchQueryChange) onSearchQueryChange(value);
                  else setInternalQuery(value);
                }}
              />
            </label>
          ) : <span />}
          <p className={clsx('whitespace-nowrap text-xs font-medium text-slate-500', renderMobileCard && 'hidden md:block')}>
            {rangeStart}-{rangeEnd} / {sortedRows.length} dòng
          </p>
        </div>
      ) : null}

      {renderMobileCard && hasControls ? (
        <div className="flex items-center justify-between gap-3 border-b border-line bg-white px-4 pb-3 text-xs font-medium text-slate-500 md:hidden">
          <span className="whitespace-nowrap">{sortedRows.length} kết quả</span>
          {sortableColumns.length ? (
            <label className="flex min-w-0 items-center gap-2">
              <span className="sr-only">Sắp xếp báo cáo</span>
              <select
                className="min-w-0 max-w-52 rounded-md border border-line bg-white px-2 py-1.5 text-xs font-semibold text-slate-700 outline-none focus:ring-2 focus:ring-blue-100"
                aria-label="Sắp xếp báo cáo"
                value={mobileSortValue}
                onChange={(event) => updateSort(sortFromSelectValue(event.target.value))}
              >
                {!sort ? <option value="">Thứ tự mặc định</option> : null}
                {sortableColumns.flatMap((column) => [
                  <option key={`${column.key}:asc`} value={`${column.key}:asc`}>{column.header}: tăng dần</option>,
                  <option key={`${column.key}:desc`} value={`${column.key}:desc`}>{column.header}: giảm dần</option>,
                ])}
              </select>
            </label>
          ) : null}
        </div>
      ) : null}

      {loading ? (
        renderMobileCard ? (
          <>
            <div className="space-y-3 bg-slate-50/70 p-3 md:hidden" role="status" aria-label="Đang tải dữ liệu">
              {[0, 1, 2].map((item) => (
                <div key={item} className="animate-pulse rounded-lg border border-line bg-white p-4">
                  <div className="h-4 w-2/3 rounded bg-slate-200" />
                  <div className="mt-3 h-5 w-1/2 rounded bg-slate-100" />
                  <div className="mt-4 grid grid-cols-2 gap-3">
                    <div className="h-12 rounded bg-slate-100" />
                    <div className="h-12 rounded bg-slate-100" />
                  </div>
                </div>
              ))}
              <span className="sr-only">Đang tải dữ liệu...</span>
            </div>
            <div className="hidden items-center gap-3 px-4 py-6 text-sm text-slate-600 md:flex">
              <div className="h-4 w-4 animate-spin rounded-full border-2 border-blue-600 border-t-transparent" />
              Đang tải dữ liệu...
            </div>
          </>
        ) : (
          <div className="flex items-center gap-3 px-4 py-6 text-sm text-slate-600">
            <div className="h-4 w-4 animate-spin rounded-full border-2 border-blue-600 border-t-transparent" />
            Đang tải dữ liệu...
          </div>
        )
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
      <>
      <div className={clsx('overflow-x-auto', renderMobileCard && 'hidden md:block')}>
        <table className="min-w-full divide-y divide-line">
          <thead className="bg-slate-50">
            <tr>
              {selection ? (
                <th className="w-12 px-3 py-3 text-center">
                  <SelectionCheckbox
                    checked={allVisibleSelected}
                    indeterminate={someVisibleSelected}
                    ariaLabel={selection.ariaLabel ?? 'Chọn tất cả dòng trên trang hiện tại'}
                    onChange={toggleVisibleRows}
                  />
                </th>
              ) : null}
              {orderedColumns.map((column) => {
                const actionColumn = isActionColumn(column);
                const canSort = !actionColumn && Boolean(column.sortable || column.sortValue);
                const activeSort = sort?.key === column.key ? sort.direction : null;
                return (
                <th
                  key={column.key}
                  aria-sort={activeSort === 'asc' ? 'ascending' : activeSort === 'desc' ? 'descending' : undefined}
                  className={clsx('px-4 py-3 text-xs font-semibold uppercase tracking-wide text-slate-500', alignClass(column.align), column.headerClassName)}
                >
                  {actionColumn ? (
                    <span className="sr-only">Thao tác</span>
                  ) : canSort ? (
                    <button
                      className={clsx('inline-flex items-center gap-1 rounded-sm hover:text-ink focus:outline-none focus:ring-2 focus:ring-blue-100', justifyClass(column.align))}
                      onClick={() => updateSort(nextSort(column.key, sort))}
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
              <tr key={rowKey(row, index)} className={clsx('hover:bg-slate-50', selection?.selectedKeys.has(rowKey(row, index)) && 'bg-blue-50/60')}>
                {selection ? (
                  <td className="w-12 px-3 py-3 text-center">
                    <SelectionCheckbox
                      checked={selection.selectedKeys.has(rowKey(row, index))}
                      ariaLabel={`Chọn dòng ${String(row.full_name ?? row.name ?? row.email ?? index + 1)}`}
                      onChange={() => toggleRow(rowKey(row, index))}
                    />
                  </td>
                ) : null}
                {orderedColumns.map((column) => (
                  <td key={column.key} className={clsx('px-4 py-3 text-sm text-slate-700', alignClass(column.align), column.className)}>
                    {column.render ? column.render(row) : row[column.key] ?? '-'}
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {renderMobileCard ? (
        <div className="space-y-3 bg-slate-50/70 p-3 md:hidden">
          {visibleRows.map((row, index) => (
            <div key={rowKey(row, index)} className={clsx(selection && 'flex items-start gap-3')}>
              {selection ? (
                <div className="pt-4">
                  <SelectionCheckbox
                    checked={selection.selectedKeys.has(rowKey(row, index))}
                    ariaLabel={`Chọn dòng ${String(row.full_name ?? row.name ?? row.email ?? index + 1)}`}
                    onChange={() => toggleRow(rowKey(row, index))}
                  />
                </div>
              ) : null}
              <div className="min-w-0 flex-1">{renderMobileCard(row)}</div>
            </div>
          ))}
        </div>
      ) : null}
      </>
      ) : (
        <div className="p-4">
          <EmptyState
            title={query.trim() ? 'Không có dòng phù hợp' : empty}
            action={query.trim() && renderMobileCard ? (
              <button type="button" className="rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-blue-700 hover:bg-blue-50" onClick={clearSearch}>
                Xóa nội dung tìm kiếm
              </button>
            ) : emptyAction}
          />
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

function sortFromSelectValue(value: string): { key: string; direction: SortDirection } | null {
  const separatorIndex = value.lastIndexOf(':');
  if (separatorIndex <= 0) return null;
  const direction = value.slice(separatorIndex + 1);
  if (direction !== 'asc' && direction !== 'desc') return null;
  return { key: value.slice(0, separatorIndex), direction };
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
