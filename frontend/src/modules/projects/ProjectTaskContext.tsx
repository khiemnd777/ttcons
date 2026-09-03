import { useEffect, useState } from 'react';
import { Link, useLocation, useNavigate } from 'react-router-dom';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { ArrowLeft, CheckCircle2, ExternalLink, GitBranch, X } from 'lucide-react';
import { LoadingSpinner } from '../../components/LoadingSpinner';
import { StatusBadge } from '../../components/StatusBadge';
import { get } from '../../lib/api/client';
import type { WorkItem } from '../../lib/types/domain';
import { WorkItemDetailDialog } from '../work-items/WorkItemDetailDialog';
import {
  consumeProjectTaskDetailNavigationState,
  projectPipelinePath,
  projectPipelineReturnTo,
  projectTaskDetailIntentMatches,
} from '../work-items/workItemNavigation';
import { executionTargetNeedsEmbeddedWorkItem, executionTargetWorkspaceLabel, isDomainExecutionTarget, workItemMatchesProject } from '../work-items/workItemExecutionTarget';

export function ProjectTaskContext({ projectId }: { projectId: string }) {
  const location = useLocation();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const params = new URLSearchParams(location.search);
  const taskId = params.get('task') ?? '';
  const contextEnabled = Boolean(taskId) && !location.pathname.endsWith('/preparation') && !location.pathname.includes('/work-items/');
  const [detailOpen, setDetailOpen] = useState(false);
  const detailOpenIntent = projectTaskDetailIntentMatches(location.state, taskId);
  const task = useQuery({
    queryKey: ['work-item', taskId],
    queryFn: () => get<WorkItem>(`/work-items/${taskId}`),
    enabled: contextEnabled,
    retry: false,
  });

  useEffect(() => {
    if (!contextEnabled || !detailOpenIntent) return;
    setDetailOpen(true);
    navigate(`${location.pathname}${location.search}${location.hash}`, {
      replace: true,
      state: consumeProjectTaskDetailNavigationState(location.state),
    });
  }, [contextEnabled, detailOpenIntent, location.hash, location.pathname, location.search, location.state, navigate]);

  if (!contextEnabled) return null;

  function closeContext() {
    const next = new URLSearchParams(location.search);
    next.delete('task');
    next.delete('returnTo');
    navigate(`${location.pathname}${next.size ? `?${next.toString()}` : ''}`, { replace: true });
  }

  const item = task.data;
  const returnTo = projectPipelineReturnTo(params.get('returnTo'), projectId, item?.run_node_id);
  const contextMismatch = Boolean(item && !workItemMatchesProject(item, projectId));
  return (
    <section className="mb-4 rounded-lg border border-blue-200 bg-blue-50/70 px-4 py-3 shadow-panel" aria-label="Công việc đang thực hiện">
      {task.isLoading ? <LoadingSpinner /> : null}
      {task.error ? (
        <div className="flex items-center justify-between gap-3 text-sm text-rose-700">
          <span>Không tải được ngữ cảnh công việc. Trang nghiệp vụ vẫn ở chế độ xem thông thường.</span>
          <button className="font-semibold" onClick={closeContext}>Đóng</button>
        </div>
      ) : null}
      {contextMismatch ? (
        <div className="flex items-center justify-between gap-3 text-sm text-rose-700">
          <span>Công việc không thuộc công trình đang mở. Ngữ cảnh đã bị từ chối để tránh thao tác nhầm hồ sơ.</span>
          <button className="font-semibold" onClick={closeContext}>Đóng</button>
        </div>
      ) : null}
      {item && !contextMismatch ? (
        <>
          <div className="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
            <div className="min-w-0">
              <div className="flex flex-wrap items-center gap-2">
                <span className="text-xs font-semibold uppercase tracking-wide text-blue-700">Việc đang thực hiện</span>
                <StatusBadge status={item.status} />
                {item.execution_target?.source_ready ? <span className="inline-flex items-center gap-1 rounded-md bg-emerald-100 px-2 py-1 text-xs font-semibold text-emerald-700"><CheckCircle2 className="h-3.5 w-3.5" />Nguồn đã sẵn sàng</span> : null}
              </div>
              <h2 className="mt-2 text-base font-semibold text-ink">{item.title}</h2>
              <p className="mt-1 text-sm text-slate-600">Thực hiện tại <span className="font-semibold text-slate-800">{executionTargetWorkspaceLabel(item)}</span>{item.run_node_name ? ` · Giai đoạn: ${item.run_node_name}` : ''}</p>
              {item.execution_target?.source_message ? <p className="mt-2 text-sm text-slate-700">{item.execution_target.source_message}</p> : null}
              {isDomainExecutionTarget(item.execution_target) && ['pending', 'ready', 'needs_changes'].includes(item.status) ? (
                <p className="mt-2 text-sm font-medium text-blue-800">Thao tác nghiệp vụ đầu tiên lưu thành công sẽ tự chuyển đầu việc sang Đang làm. Không cần bấm Bắt đầu riêng.</p>
              ) : null}
            </div>
            <div className="flex flex-wrap items-center gap-2">
              {returnTo ? <Link className="inline-flex items-center gap-1 rounded-md border border-blue-200 bg-white px-3 py-2 text-sm font-semibold text-blue-700 hover:bg-blue-50" to={returnTo}><ArrowLeft className="h-4 w-4" />Quay lại</Link> : null}
              <Link className="inline-flex items-center gap-1 rounded-md border border-blue-200 bg-white px-3 py-2 text-sm font-semibold text-blue-700 hover:bg-blue-50" to={projectPipelinePath(projectId, item.run_node_id)}><GitBranch className="h-4 w-4" />Tiến độ</Link>
              {executionTargetNeedsEmbeddedWorkItem(item.execution_target) || item.status !== 'completed' ? (
                <button className="inline-flex items-center gap-1 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700" onClick={() => setDetailOpen(true)}>
                  Kiểm tra điều kiện & chốt bước <ExternalLink className="h-4 w-4" />
                </button>
              ) : null}
              <button aria-label="Đóng ngữ cảnh công việc" className="grid h-9 w-9 place-items-center rounded-md text-slate-500 hover:bg-white hover:text-slate-800" onClick={closeContext}><X className="h-4 w-4" /></button>
            </div>
          </div>
          <WorkItemDetailDialog
            open={detailOpen}
            item={item}
            onClose={() => setDetailOpen(false)}
            onChanged={() => {
              void queryClient.invalidateQueries({ queryKey: ['work-item', taskId] });
              void queryClient.invalidateQueries({ queryKey: ['project-operations', projectId] });
              void queryClient.invalidateQueries({ queryKey: ['my-work-items'] });
            }}
          />
        </>
      ) : null}
    </section>
  );
}
