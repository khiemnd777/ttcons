import { useMemo, useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { AlertCircle, MessageSquare, RefreshCw, ShieldAlert } from 'lucide-react';
import { useParams } from 'react-router-dom';
import { EmptyState } from '../../components/EmptyState';
import { ApiError, get } from '../../lib/api/client';
import type {
  EvidenceRecord,
  ProjectOperationsResponse,
  ProjectOperationsTimelineNode,
  WorkItem,
} from '../../lib/types/domain';
import { ActivityThread } from '../work-items/ActivityThread';
import { buildActivityAttachmentOptions } from '../work-items/workItemDetailDomain';
import { asArray, prioritizeWorkItems } from '../work-items/workItemPresentation';
import {
  currentTimelineNode,
  isOpenProjectWorkItem,
  selectProjectPriorityWork,
} from './projectOperationsDomain';
import { ProjectStartupWizard } from './ProjectStartupWizard';

export function ProjectDiscussionPage() {
  const { id = '' } = useParams();
  const queryClient = useQueryClient();
  const [startupWizardOpen, setStartupWizardOpen] = useState(false);
  const operations = useQuery({
    queryKey: ['project-operations', id],
    queryFn: () => get<ProjectOperationsResponse>(`/projects/${id}/operations`),
    enabled: Boolean(id),
    retry: false,
  });
  const data = operations.data;
  const allWorkItems = useMemo(() => asArray<WorkItem>(data?.work_items), [data?.work_items]);
  const priorityWork = useMemo(
    () => prioritizeWorkItems(allWorkItems.filter(isOpenProjectWorkItem)),
    [allWorkItems],
  );
  const blockers = useMemo(
    () => prioritizeWorkItems(asArray<WorkItem>(data?.blockers)),
    [data?.blockers],
  );
  const timeline = useMemo(
    () => asArray<ProjectOperationsTimelineNode>(data?.workflow_timeline),
    [data?.workflow_timeline],
  );
  const currentNode = useMemo(() => currentTimelineNode(timeline), [timeline]);
  const primaryWork = useMemo(
    () => selectProjectPriorityWork(priorityWork, blockers),
    [blockers, priorityWork],
  );
  const activityTargets = useMemo(() => allWorkItems.map((item) => ({
    scope_type: 'work_item',
    scope_id: item.id,
    work_item_id: item.id,
    label: `${item.project_code ? `${item.project_code} · ` : ''}${item.title}`,
  })), [allWorkItems]);
  const attachmentOptions = useMemo(
    () => buildActivityAttachmentOptions(asArray<EvidenceRecord>(data?.latest_evidence)),
    [data?.latest_evidence],
  );

  if (operations.isLoading) return <ProjectDiscussionSkeleton />;
  if (operations.error) {
    return <ProjectDiscussionError error={operations.error} onRetry={() => void operations.refetch()} />;
  }
  if (!data) return <EmptyState title="Không có dữ liệu trao đổi công trình" />;

  const project = data.project_summary ?? data.project;
  const hasPipeline = Boolean(data.pipeline_run) || timeline.length > 0;

  return (
    <>
      <h1 className="sr-only">Trao đổi · {project.code ?? data.project.code} · {project.name ?? data.project.name}</h1>
      <div className="space-y-4">
        <div className="flex items-center gap-2">
          <MessageSquare className="h-5 w-5 text-blue-700" />
          <div>
            <h2 className="font-semibold text-ink">Trao đổi</h2>
            <p className="mt-0.5 text-sm text-slate-500">Phối hợp, quyết định và theo dõi các vấn đề của công trình.</p>
          </div>
        </div>
        <ActivityThread
          projectId={id}
          targetOptions={activityTargets}
          attachmentOptions={attachmentOptions}
          workflowAction={{
            hasPipeline,
            pipelineStatus: String(data.pipeline_run?.status ?? ''),
            currentNode,
            primaryWork,
            onOpenStartup: () => setStartupWizardOpen(true),
          }}
          onChanged={() => {
            void queryClient.invalidateQueries({ queryKey: ['project-operations', id] });
          }}
        />
      </div>
      <ProjectStartupWizard
        open={startupWizardOpen}
        projectId={id}
        onClose={() => setStartupWizardOpen(false)}
        onChanged={() => {
          void queryClient.invalidateQueries({ queryKey: ['project-operations', id] });
          void queryClient.invalidateQueries({ queryKey: ['project-members', id] });
          void queryClient.invalidateQueries({ queryKey: ['project', id] });
        }}
      />
    </>
  );
}

function ProjectDiscussionSkeleton() {
  return (
    <div className="space-y-4" role="status" aria-label="Đang tải trao đổi công trình">
      <div className="h-10 w-64 animate-pulse rounded-md bg-slate-100" />
      <div className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_280px]">
        <div className="h-[620px] animate-pulse rounded-lg border border-line bg-white shadow-panel" />
        <div className="space-y-3">
          <div className="h-44 animate-pulse rounded-lg border border-line bg-white shadow-panel" />
          <div className="h-56 animate-pulse rounded-lg border border-line bg-white shadow-panel" />
        </div>
      </div>
    </div>
  );
}

function ProjectDiscussionError({ error, onRetry }: { error: unknown; onRetry: () => void }) {
  const apiError = error instanceof ApiError ? error : null;
  const forbidden = apiError?.status === 403;
  return (
    <section className="rounded-lg border border-line bg-white p-6 shadow-panel">
      <div className="flex items-start gap-3">
        {forbidden
          ? <ShieldAlert className="mt-0.5 h-6 w-6 text-amber-600" />
          : <AlertCircle className="mt-0.5 h-6 w-6 text-rose-600" />}
        <div className="min-w-0 flex-1">
          <h1 className="text-base font-semibold text-ink">
            {forbidden ? 'Bạn không có quyền xem trao đổi công trình này' : 'Không tải được trao đổi công trình'}
          </h1>
          <p className="mt-1 text-sm leading-6 text-slate-600">
            {error instanceof Error ? error.message : 'Vui lòng thử lại hoặc kiểm tra quyền truy cập.'}
          </p>
          {!forbidden ? (
            <button
              type="button"
              className="mt-4 inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700"
              onClick={onRetry}
            >
              <RefreshCw className="h-4 w-4" />
              Thử lại
            </button>
          ) : null}
        </div>
      </div>
    </section>
  );
}
