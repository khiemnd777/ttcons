import { ReactNode } from 'react';
import { EmptyState } from './EmptyState';

export type Column<T> = {
  key: string;
  header: string;
  render?: (row: T) => ReactNode;
  className?: string;
};

export function DataTable<T extends Record<string, any>>({
  rows,
  columns,
  empty,
}: {
  rows: T[];
  columns: Column<T>[];
  empty?: string;
}) {
  if (!rows.length) return <EmptyState title={empty} />;
  return (
    <div className="overflow-hidden rounded-lg border border-line bg-white shadow-panel">
      <div className="overflow-x-auto">
        <table className="min-w-full divide-y divide-line">
          <thead className="bg-slate-50">
            <tr>
              {columns.map((column) => (
                <th key={column.key} className="px-4 py-3 text-left text-xs font-semibold uppercase tracking-wide text-slate-500">
                  {column.header}
                </th>
              ))}
            </tr>
          </thead>
          <tbody className="divide-y divide-line">
            {rows.map((row, index) => (
              <tr key={row.id ?? index} className="hover:bg-slate-50">
                {columns.map((column) => (
                  <td key={column.key} className={`px-4 py-3 text-sm text-slate-700 ${column.className ?? ''}`}>
                    {column.render ? column.render(row) : row[column.key] ?? '-'}
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
