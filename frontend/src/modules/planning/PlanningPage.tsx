import { useState } from 'react';
import { Link, useParams } from 'react-router-dom';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Ban, CheckCircle2, ClipboardCheck, Play, Send } from 'lucide-react';
import { get, post } from '../../lib/api/client';
import { PageHeader } from '../../components/PageHeader';
import { LoadingSpinner } from '../../components/LoadingSpinner';
import { EmptyState } from '../../components/EmptyState';
import { StatusBadge } from '../../components/StatusBadge';
import { Timeline } from '../../components/Timeline';

export function PlanningPage({ mode = 'planning' }: { mode?: 'planning' | 'pipeline' }) {
  const { id = '' } = useParams();
  const queryClient = useQueryClient();
  const [selectedNodeId, setSelectedNodeId] = useState('');
  const [blockReason, setBlockReason] = useState('');
  const run = useQuery({
    queryKey: ['pipeline-run', id],
    queryFn: () => get<Record<string, any>>(`/projects/${id}/pipeline-run`),
    enabled: Boolean(id),
    retry: false,
  });
  const audit = useQuery({
    queryKey: ['project-audit', id],
    queryFn: () => get<Record<string, any>[]>(`/projects/${id}/audit-logs`),
    enabled: Boolean(id),
  });
  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ['pipeline-run', id] });
    queryClient.invalidateQueries({ queryKey: ['project-audit', id] });
  };
  const start = useMutation({ mutationFn: () => post(`/projects/${id}/start-pipeline`, {}), onSuccess: invalidate });
  const submitStep = useMutation({ mutationFn: (stepId: string) => post(`/pipeline-run-steps/${stepId}/submit`, {}), onSuccess: invalidate });
  const requestApproval = useMutation({ mutationFn: (nodeId: string) => post(`/pipeline-run-nodes/${nodeId}/request-approval`, { reason: 'Yêu cầu review/approval để chuyển node' }), onSuccess: invalidate });
  const advance = useMutation({ mutationFn: (nodeId: string) => post(`/pipeline-run-nodes/${nodeId}/advance`, {}), onSuccess: invalidate });
  const block = useMutation({ mutationFn: (nodeId: string) => post(`/pipeline-run-nodes/${nodeId}/block`, { reason: blockReason || 'Bị chặn bởi điều kiện vận hành' }), onSuccess: invalidate });

  const nodes = run.data?.nodes ?? [];
  const selectedNode = nodes.find((node: Record<string, any>) => node.id === selectedNodeId) ?? nodes.find((node: Record<string, any>) => node.status === 'in_progress') ?? nodes[0];
  const completed = nodes.filter((node: Record<string, any>) => node.status === 'completed').length;
  const percent = nodes.length ? Math.round((completed / nodes.length) * 100) : 0;

  if (run.isLoading) return <LoadingSpinner />;

  return (
    <>
      <PageHeader
        title={mode === 'pipeline' ? 'Pipeline công trình' : 'Planning Pipeline Run'}
        description={run.data ? `${run.data.project_name} · Template: ${run.data.template_name}` : 'Chưa có pipeline run cho công trình này.'}
        actions={
          <>
            <Link to={`/projects/${id}`} className="rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100">Về công trình</Link>
            <button className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700" onClick={() => start.mutate()}>
              <Play className="h-4 w-4" />
              Start / Reset run
            </button>
          </>
        }
      />
      {!run.data ? (
        <EmptyState title="Chưa có pipeline run. Start pipeline để clone template thành workflow thật." />
      ) : (
        <div className="grid gap-4 xl:grid-cols-[1fr_420px]">
          <section className="space-y-4">
            <div className="rounded-lg border border-line bg-white p-4 shadow-panel">
              <div className="mb-2 flex items-center justify-between text-sm">
                <span className="font-semibold text-ink">Tiến độ pipeline</span>
                <span className="font-semibold text-blue-700">{percent}% · {completed}/{nodes.length} node</span>
              </div>
              <div className="h-2 rounded-full bg-slate-100">
                <div className="h-2 rounded-full bg-blue-600" style={{ width: `${percent}%` }} />
              </div>
            </div>

            <div className="flex flex-wrap gap-3 rounded-lg border border-line bg-white p-4 shadow-panel">
              {nodes.map((node: Record<string, any>, index: number) => (
                <button
                  key={node.id}
                  className={`w-56 rounded-lg border p-4 text-left hover:border-blue-300 hover:bg-blue-50 ${selectedNode?.id === node.id ? 'border-blue-400 bg-blue-50' : 'border-line'}`}
                  onClick={() => setSelectedNodeId(node.id)}
                >
                  <div className="mb-3 grid h-8 w-8 place-items-center rounded-md bg-slate-100 text-xs font-black text-slate-700">{index + 1}</div>
                  <p className="text-sm font-semibold text-ink">{node.name}</p>
                  <p className="mt-2 line-clamp-2 text-xs text-slate-500">{node.description}</p>
                  <div className="mt-3 flex flex-wrap gap-1">
                    <StatusBadge status={node.status} />
                    <span className="rounded-md bg-slate-100 px-2 py-1 text-xs font-semibold text-slate-600">{node.steps?.filter((s: Record<string, any>) => s.status === 'completed').length ?? 0}/{node.steps?.length ?? 0} step</span>
                  </div>
                </button>
              ))}
            </div>
          </section>

          <aside className="space-y-4">
            <div className="rounded-lg border border-line bg-white p-4 shadow-panel">
              <div className="mb-4 flex items-start justify-between gap-3">
                <div>
                  <h2 className="text-base font-semibold text-ink">{selectedNode?.name}</h2>
                  <p className="mt-1 text-sm leading-6 text-slate-600">{selectedNode?.description}</p>
                </div>
                <StatusBadge status={selectedNode?.status} />
              </div>
              <dl className="mb-4 grid gap-2 rounded-lg bg-slate-50 p-3 text-sm">
                <div className="flex justify-between gap-3"><dt className="text-slate-500">Role</dt><dd className="font-medium text-ink">{selectedNode?.responsible_role_ids?.join?.(', ') ?? '-'}</dd></div>
                <div className="flex justify-between gap-3"><dt className="text-slate-500">Approval</dt><dd className="font-medium text-ink">{selectedNode?.approval_rule ?? '-'}</dd></div>
                <div className="flex justify-between gap-3"><dt className="text-slate-500">Fail path</dt><dd className="font-medium text-ink">{selectedNode?.fail_path ?? '-'}</dd></div>
              </dl>
              <div className="space-y-2">
                {(selectedNode?.steps ?? []).map((step: Record<string, any>) => (
                  <div key={step.id} className="flex items-center justify-between gap-3 rounded-md border border-line px-3 py-2">
                    <div>
                      <p className="text-sm font-medium text-ink">{step.name}</p>
                      <p className="text-xs text-slate-500">{step.step_type}</p>
                    </div>
                    {step.status === 'completed' ? (
                      <CheckCircle2 className="h-5 w-5 text-emerald-600" />
                    ) : (
                      <button className="rounded-md border border-line px-3 py-1.5 text-xs font-semibold hover:bg-slate-100" onClick={() => submitStep.mutate(step.id)}>
                        Submit
                      </button>
                    )}
                  </div>
                ))}
              </div>
              <div className="mt-4 grid gap-2">
                <button className="inline-flex items-center justify-center gap-2 rounded-md bg-emerald-600 px-3 py-2 text-sm font-semibold text-white hover:bg-emerald-700" onClick={() => advance.mutate(selectedNode.id)}>
                  <CheckCircle2 className="h-4 w-4" />
                  Advance node
                </button>
                <button className="inline-flex items-center justify-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100" onClick={() => requestApproval.mutate(selectedNode.id)}>
                  <Send className="h-4 w-4" />
                  Request approval
                </button>
                <div className="flex gap-2">
                  <input className="min-w-0 flex-1 rounded-md border border-line px-3 py-2 text-sm focus-ring" placeholder="Lý do block" value={blockReason} onChange={(event) => setBlockReason(event.target.value)} />
                  <button className="rounded-md bg-rose-600 px-3 py-2 text-white hover:bg-rose-700" onClick={() => block.mutate(selectedNode.id)} title="Block node">
                    <Ban className="h-4 w-4" />
                  </button>
                </div>
              </div>
            </div>
            <div>
              <h2 className="mb-3 text-base font-semibold text-ink">Audit / Playback</h2>
              <Timeline events={audit.data?.slice(0, 8) ?? []} />
            </div>
          </aside>
        </div>
      )}
    </>
  );
}
