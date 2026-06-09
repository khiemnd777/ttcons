import { Link, useParams } from 'react-router-dom';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { GitBranch, Play } from 'lucide-react';
import { get, post } from '../../lib/api/client';
import { PageHeader } from '../../components/PageHeader';
import { LoadingSpinner } from '../../components/LoadingSpinner';
import { StatusBadge } from '../../components/StatusBadge';
import { StatCard } from '../../components/StatCard';
import { CurrencyText } from '../../components/CurrencyText';
import { DataTable } from '../../components/DataTable';
import type { Project } from '../../lib/types/domain';

const moduleLinks = [
  ['Pipeline', 'pipeline'],
  ['Planning', 'planning'],
  ['BOQ / Dự toán', 'boq'],
  ['Mua sắm', 'procurement'],
  ['Kho', 'warehouse'],
  ['Thi công', 'operations'],
  ['Nghiệm thu', 'acceptance'],
  ['Rework', 'rework'],
  ['Bàn giao', 'handover'],
  ['Kế toán', 'accounting'],
  ['Công nợ', 'debt'],
  ['Audit', 'audit'],
];

export function ProjectDetailPage() {
  const { id = '' } = useParams();
  const queryClient = useQueryClient();
  const project = useQuery({ queryKey: ['project', id], queryFn: () => get<Project>(`/projects/${id}`), enabled: Boolean(id) });
  const run = useQuery({ queryKey: ['pipeline-run', id], queryFn: () => get<Record<string, any>>(`/projects/${id}/pipeline-run`), enabled: Boolean(id), retry: false });
  const start = useMutation({
    mutationFn: () => post(`/projects/${id}/start-pipeline`, {}),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['pipeline-run', id] });
      queryClient.invalidateQueries({ queryKey: ['project', id] });
    },
  });

  if (project.isLoading) return <LoadingSpinner />;
  const data = project.data;
  if (!data) return null;

  return (
    <>
      <PageHeader
        title={data.name}
        description={`${data.code} · ${data.client_name} · ${data.address ?? 'Chưa có địa chỉ'}`}
        actions={
          <button className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700" onClick={() => start.mutate()}>
            <Play className="h-4 w-4" />
            Start pipeline
          </button>
        }
      />
      <div className="grid gap-4 md:grid-cols-4">
        <StatCard label="Trạng thái" value={<StatusBadge status={data.status} />} />
        <StatCard label="Tiến độ" value={`${data.progress_percent ?? 0}%`} />
        <StatCard label="Ngân sách" value={<CurrencyText value={data.estimated_budget} />} />
        <StatCard label="Hợp đồng" value={<CurrencyText value={data.contract_amount} />} />
      </div>

      <div className="mt-5 grid gap-4 lg:grid-cols-[1fr_360px]">
        <section className="rounded-lg border border-line bg-white p-4 shadow-panel">
          <div className="mb-4 flex items-center gap-2">
            <GitBranch className="h-5 w-5 text-blue-700" />
            <h2 className="font-semibold text-ink">Pipeline hiện tại</h2>
          </div>
          {run.data ? (
            <div className="flex flex-wrap gap-2">
              {(run.data.nodes ?? []).map((node: Record<string, any>) => (
                <Link key={node.id} to={`/projects/${id}/planning`} className="min-w-48 rounded-lg border border-line p-3 hover:border-blue-300 hover:bg-blue-50">
                  <p className="text-sm font-semibold text-ink">{node.order_index}. {node.name}</p>
                  <div className="mt-2"><StatusBadge status={node.status} /></div>
                </Link>
              ))}
            </div>
          ) : (
            <div className="rounded-lg border border-dashed border-slate-300 p-6 text-sm text-slate-600">
              Chưa có pipeline run. Bấm Start pipeline để clone template đã publish cho công trình này.
            </div>
          )}
        </section>

        <section className="rounded-lg border border-line bg-white p-4 shadow-panel">
          <h2 className="font-semibold text-ink">Module công trình</h2>
          <div className="mt-3 grid grid-cols-2 gap-2">
            {moduleLinks.map(([label, path]) => (
              <Link key={path} to={`/projects/${id}/${path}`} className="rounded-md border border-line px-3 py-2 text-sm font-medium text-slate-700 hover:border-blue-300 hover:bg-blue-50 hover:text-blue-700">
                {label}
              </Link>
            ))}
          </div>
        </section>
      </div>

      <section className="mt-5">
        <h2 className="mb-3 text-base font-semibold text-ink">Thành viên công trình</h2>
        <DataTable
          rows={data.members ?? []}
          columns={[
            { key: 'full_name', header: 'Nhân sự' },
            { key: 'department', header: 'Phòng ban' },
            { key: 'position', header: 'Chức vụ' },
            { key: 'role_ids', header: 'Role', render: (row) => Array.isArray(row.role_ids) ? row.role_ids.join(', ') : '-' },
          ]}
        />
      </section>
    </>
  );
}
