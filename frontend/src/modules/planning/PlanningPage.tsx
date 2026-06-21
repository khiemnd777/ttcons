import { useState } from 'react';
import { useParams } from 'react-router-dom';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Ban, CheckCircle2, ClipboardList, FileCheck2, Play, Send } from 'lucide-react';
import { get, post } from '../../lib/api/client';
import { PageHeader } from '../../components/PageHeader';
import { LoadingSpinner } from '../../components/LoadingSpinner';
import { EmptyState } from '../../components/EmptyState';
import { StatusBadge } from '../../components/StatusBadge';
import { Timeline } from '../../components/Timeline';
import { useToast } from '../../components/ToastProvider';
import { EnumChip } from '../../components/EnumChip';
import { ProjectBackLink } from '../../components/ProjectBackLink';
import type { WorkItem } from '../../lib/types/domain';
import { WorkItemDetailDialog } from '../work-items/WorkItemDetailDialog';
import { numberValue } from '../work-items/workItemPresentation';

export function PlanningPage({ mode = 'planning' }: { mode?: 'planning' | 'pipeline' }) {
  const { id = '' } = useParams();
  const queryClient = useQueryClient();
  const toast = useToast();
  const [selectedNodeId, setSelectedNodeId] = useState('');
  const [selectedWorkItem, setSelectedWorkItem] = useState<WorkItem | null>(null);
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
    queryClient.invalidateQueries({ queryKey: ['work-item', selectedWorkItem?.id ?? ''] });
    queryClient.invalidateQueries({ queryKey: ['work-item-form', selectedWorkItem?.id ?? ''] });
  };
  const start = useMutation({
    mutationFn: () => post(`/projects/${id}/start-pipeline`, {}),
    onSuccess: () => {
      invalidate();
      toast.success('Đã start pipeline run');
    },
    onError: (error) => toast.error('Không start được pipeline run', error),
  });
  const submitStep = useMutation({
    mutationFn: (stepId: string) => post(`/pipeline-run-steps/${stepId}/submit`, {}),
    onSuccess: () => {
      invalidate();
      toast.success('Đã submit step');
    },
    onError: (error) => toast.error('Không submit được step', error),
  });
  const requestApproval = useMutation({
    mutationFn: (nodeId: string) => post(`/pipeline-run-nodes/${nodeId}/request-approval`, { reason: 'Yêu cầu review/approval để chuyển node' }),
    onSuccess: () => {
      invalidate();
      toast.success('Đã gửi yêu cầu duyệt');
    },
    onError: (error) => toast.error('Không gửi được yêu cầu duyệt', error),
  });
  const advance = useMutation({
    mutationFn: (nodeId: string) => post(`/pipeline-run-nodes/${nodeId}/advance`, {}),
    onSuccess: () => {
      invalidate();
      toast.success('Đã chuyển node');
    },
    onError: (error) => toast.error('Không chuyển được node', error),
  });
  const block = useMutation({
    mutationFn: (nodeId: string) => post(`/pipeline-run-nodes/${nodeId}/block`, { reason: blockReason || 'Bị chặn bởi điều kiện vận hành' }),
    onSuccess: () => {
      invalidate();
      toast.success('Đã block node');
    },
    onError: (error) => toast.error('Không block được node', error),
  });

  const nodes = run.data?.nodes ?? [];
  const selectedNode = nodes.find((node: Record<string, any>) => node.id === selectedNodeId) ?? nodes.find((node: Record<string, any>) => node.status === 'in_progress') ?? nodes[0];
  const completed = nodes.filter((node: Record<string, any>) => node.status === 'completed').length;
  const percent = nodes.length ? Math.round((completed / nodes.length) * 100) : 0;

  function openStepWorkItem(step: Record<string, any>) {
    const workItem = stepToWorkItem(step, run.data);
    if (!workItem) {
      toast.warn({ title: 'Step chưa có work item', description: 'Pipeline runtime chưa tạo work item cho step này.' });
      return;
    }
    setSelectedWorkItem(workItem);
  }

  if (run.isLoading) return <LoadingSpinner />;

  return (
    <>
      <PageHeader
        title={mode === 'pipeline' ? 'Pipeline công trình' : 'Planning Pipeline Run'}
        description={run.data ? `${run.data.project_name} · Template: ${run.data.template_name}` : 'Chưa có pipeline run cho công trình này.'}
        actions={
          <>
            <ProjectBackLink projectId={id} />
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
                <div className="flex justify-between gap-3"><dt className="text-slate-500">Fail path</dt><dd className="font-medium text-ink"><EnumChip group="fail_path" value={selectedNode?.fail_path} /></dd></div>
              </dl>
              <div className="space-y-2">
                {(selectedNode?.steps ?? []).map((step: Record<string, any>) => {
                  const hasForm = Boolean(step.workflow_form_instance_id);
                  const formRequired = Boolean(step.workflow_form_required);
                  const formSubmitted = step.workflow_form_status === 'submitted';
                  const completedStep = step.status === 'completed';
                  const canCompleteStep = !completedStep && (!formRequired || formSubmitted);
                  return (
                    <div key={step.id} className="rounded-md border border-line px-3 py-2">
                      <div className="flex items-start justify-between gap-3">
                        <div className="min-w-0">
                          <p className="text-sm font-medium text-ink">{step.name}</p>
                          <div className="mt-2 flex flex-wrap gap-1.5">
                            <EnumChip group="step_type" value={step.step_type} />
                            <StatusBadge status={step.status} />
                            {hasForm ? <StatusBadge status={step.workflow_form_status ?? 'draft'} /> : null}
                            <span className="rounded-md bg-slate-100 px-2 py-1 text-xs font-semibold text-slate-600">Evidence: {numberValue(step.evidence_count)}</span>
                          </div>
                        </div>
                        {completedStep ? <CheckCircle2 className="mt-1 h-5 w-5 shrink-0 text-emerald-600" /> : null}
                      </div>
                      {!completedStep ? (
                        <div className="mt-3 flex flex-wrap justify-end gap-2">
                          {hasForm ? (
                            <button
                              className="inline-flex items-center gap-1.5 rounded-md bg-blue-600 px-3 py-1.5 text-xs font-semibold text-white hover:bg-blue-700"
                              onClick={() => openStepWorkItem(step)}
                            >
                              <ClipboardList className="h-3.5 w-3.5" />
                              Mở form
                            </button>
                          ) : null}
                          {canCompleteStep ? (
                            <button
                              className="inline-flex items-center gap-1.5 rounded-md border border-line px-3 py-1.5 text-xs font-semibold text-slate-700 hover:bg-slate-100"
                              onClick={() => submitStep.mutate(step.id)}
                            >
                              <FileCheck2 className="h-3.5 w-3.5" />
                              Hoàn tất step
                            </button>
                          ) : null}
                          {formRequired && hasForm && !formSubmitted ? (
                            <span className="inline-flex items-center rounded-md bg-amber-50 px-2 py-1 text-xs font-semibold text-amber-800 ring-1 ring-inset ring-amber-200">
                              Cần submit form
                            </span>
                          ) : null}
                          {formRequired && !hasForm ? (
                            <span className="inline-flex items-center rounded-md bg-rose-50 px-2 py-1 text-xs font-semibold text-rose-700 ring-1 ring-inset ring-rose-200">
                              Thiếu form instance
                            </span>
                          ) : null}
                        </div>
                      ) : null}
                    </div>
                  );
                })}
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
      <WorkItemDetailDialog
        open={Boolean(selectedWorkItem)}
        item={selectedWorkItem}
        onClose={() => setSelectedWorkItem(null)}
        onChanged={(item) => {
          setSelectedWorkItem(item);
          invalidate();
        }}
      />
    </>
  );
}

function stepToWorkItem(step: Record<string, any>, run?: Record<string, any>): WorkItem | null {
  const workItemId = String(step.work_item_id ?? '').trim();
  if (!workItemId) return null;
  const node = (run?.nodes ?? []).find((candidate: Record<string, any>) =>
    (candidate.steps ?? []).some((candidateStep: Record<string, any>) => candidateStep.id === step.id),
  );
  return {
    id: workItemId,
    project_id: run?.project_id,
    project_name: run?.project_name,
    title: step.work_item_title ?? step.name,
    description: step.description,
    status: step.work_item_status ?? step.status,
    work_type: step.step_type,
    source_type: 'pipeline_run_step',
    source_id: step.id,
    role_ids: Array.isArray(step.work_item_role_ids) ? step.work_item_role_ids : step.required_role_ids,
    evidence_count: Number(step.evidence_count ?? 0),
    evidence_requirement_json: step.evidence_requirement_json,
    pipeline_run_id: run?.id,
    run_node_id: step.run_node_id,
    run_node_name: node?.name,
    run_step_id: step.id,
    run_step_name: step.name,
  };
}
