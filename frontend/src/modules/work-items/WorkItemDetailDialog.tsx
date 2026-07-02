import { ReactNode, useState } from 'react';
import { Link } from 'react-router-dom';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  AlertTriangle,
  CheckCircle2,
  Circle,
  Clock3,
  Loader2,
  MessageSquare,
  PlayCircle,
  ShieldCheck,
  Users,
} from 'lucide-react';
import clsx from 'clsx';
import { Dialog } from '../../components/Dialog';
import { EmptyState } from '../../components/EmptyState';
import { LoadingSpinner } from '../../components/LoadingSpinner';
import { StatusBadge } from '../../components/StatusBadge';
import { get, post } from '../../lib/api/client';
import { dateText } from '../../lib/utils/format';
import { useToast } from '../../components/ToastProvider';
import type { ActivityPayload, EvidenceRecord, WorkItem, WorkItemActivity } from '../../lib/types/domain';
import { ActivityAttachments } from './ActivityAttachments';
import { ActivityComposer, type ActivityAttachmentOption } from './ActivityComposer';
import { EvidencePanel } from './EvidencePanel';
import {
  WorkItemPrimarySurface,
  primarySurfaceUsesEvidence,
  primarySurfaceUsesWorkflowForm,
  resolveWorkItemPrimarySurface,
  type WorkItemPrimarySurfaceKind,
} from './WorkItemPrimarySurface';
import { WorkflowFormPanel } from './WorkflowFormPanel';
import {
  actionDescriptions,
  actionLabels,
  actionRequiresReason,
  activityLabel,
  asArray,
  availableActions,
  compactDateTime,
  departmentLabel,
  hasWorkflowFormSummary,
  isWorkOverdue,
  numberValue,
  priorityLabel,
  visibilityLabel,
  workTypeLabel,
  workflowFormChipText,
  workflowFormNextAction,
  type WorkItemActionKey,
} from './workItemPresentation';

export function WorkItemDetailDialog({
  open,
  item,
  onClose,
  onChanged,
}: {
  open: boolean;
  item: WorkItem | null;
  onClose: () => void;
  onChanged?: (item: WorkItem) => void;
}) {
  const [actionText, setActionText] = useState('');
  const [pendingAction, setPendingAction] = useState<WorkItemActionKey | null>(null);
  const queryClient = useQueryClient();
  const toast = useToast();
  const selectedWorkId = item?.id ?? '';
  const workDetail = useQuery({
    queryKey: ['work-item', selectedWorkId],
    queryFn: () => get<WorkItem>(`/work-items/${selectedWorkId}`),
    enabled: open && Boolean(selectedWorkId),
    retry: false,
  });
  const detailItem = workDetail.data ?? item;

  const workAction = useMutation({
    mutationFn: ({ workItemId, action, comment }: { workItemId: string; action: WorkItemActionKey; comment: string }) =>
      post<WorkItem>(`/work-items/${workItemId}/${action}`, { comment, reason: comment }),
    onSuccess: (updated, variables) => {
      setActionText('');
      setPendingAction(null);
      queryClient.invalidateQueries({ queryKey: ['work-item', variables.workItemId] });
      queryClient.invalidateQueries({ queryKey: ['my-work-items'] });
      queryClient.invalidateQueries({ queryKey: ['activity-attention'] });
      if (updated.project_id) {
        queryClient.invalidateQueries({ queryKey: ['project-operations', updated.project_id] });
      }
      onChanged?.(updated);
      toast.success(`Đã ${actionLabels[variables.action].toLowerCase()}`);
    },
    onError: (error) => toast.error('Không thao tác được work item', error),
  });
  const activityAction = useMutation({
    mutationFn: ({ workItemId, payload }: { workItemId: string; payload: ActivityPayload }) =>
      post<WorkItemActivity>(`/work-items/${workItemId}/activity`, payload),
    onSuccess: (_activity, variables) => {
      queryClient.invalidateQueries({ queryKey: ['work-item', variables.workItemId] });
      queryClient.invalidateQueries({ queryKey: ['my-work-items'] });
      queryClient.invalidateQueries({ queryKey: ['activity-attention'] });
      if (detailItem?.project_id) {
        queryClient.invalidateQueries({ queryKey: ['project-operations', detailItem.project_id] });
      }
      toast.success('Đã gửi activity');
    },
    onError: (error) => toast.error('Không gửi được activity', error),
  });

  function runAction(action: WorkItemActionKey) {
    if (!detailItem?.id) return;
    const comment = actionText.trim();
    if (actionRequiresReason(action) && !comment) {
      setPendingAction(action);
      toast.warn({ title: 'Cần nhập lý do', description: actionDescriptions[action] });
      return;
    }
    workAction.mutate({ workItemId: detailItem.id, action, comment });
  }

  function closeDialog() {
    setActionText('');
    setPendingAction(null);
    onClose();
  }

  const actions = availableActions(detailItem?.status);
  const evidence = asArray<EvidenceRecord>(detailItem?.evidence);
  const activities = asArray<WorkItemActivity>(detailItem?.activities);
  const attachmentOptions = buildActivityAttachmentOptions(evidence);
  const primarySurface = detailItem ? resolveWorkItemPrimarySurface(detailItem) : null;
  const requirementRows = detailItem ? buildRequirementRows(detailItem, evidence) : [];
  const nextAction = detailItem ? resolveNextAction(detailItem, evidence, actions, primarySurface?.kind) : '';
  const latestActivity = activities[0];
  const workContextLabel = detailItem?.run_node_name
    ?? detailItem?.run_step_name
    ?? (detailItem?.work_type ? workTypeLabel(detailItem.work_type) : 'Runtime work');

  return (
    <Dialog
      open={open}
      title={detailItem?.title ?? 'Workflow detail'}
      description={detailItem ? `${detailItem.project_name ?? 'Công trình'} · ${departmentLabel(detailItem.department)} · ${workContextLabel}` : undefined}
      onClose={closeDialog}
      size="workspace"
    >
      {workDetail.isLoading && !detailItem ? <LoadingSpinner /> : null}
      {workDetail.error && !detailItem ? <EmptyState title="Không tải được workflow detail" /> : null}
      {detailItem ? (
        <div className="space-y-4">
          <WorkItemWorkspaceHeader item={detailItem} evidence={evidence} nextAction={nextAction} />

          <div className="grid items-start gap-4 xl:grid-cols-[15rem_minmax(0,1fr)_18rem]">
            <WorkItemProgressRail item={detailItem} />

            <main className="min-w-0 space-y-4">
              <section className="rounded-lg border border-line bg-white shadow-panel">
                <div className="border-b border-line px-4 py-3">
                  <p className="text-xs font-semibold uppercase tracking-wide text-blue-700">Bước đang xử lý</p>
                  <div className="mt-1 flex flex-col gap-2 lg:flex-row lg:items-start lg:justify-between">
                    <div className="min-w-0">
                      <h3 className="text-base font-semibold text-ink">{detailItem.run_step_name ?? detailItem.title}</h3>
                      <p className="mt-1 text-sm leading-5 text-slate-600">
                        {detailItem.description || 'Chưa có mô tả chi tiết cho bước này.'}
                      </p>
                    </div>
                    <div className="flex shrink-0 flex-wrap gap-2 text-xs font-semibold text-slate-600">
                      <MetaPill icon={<Users className="h-3.5 w-3.5" />} label={detailItem.assignee_name ?? firstRoleLabel(detailItem) ?? departmentLabel(detailItem.department)} />
                      <MetaPill icon={<Clock3 className="h-3.5 w-3.5" />} label={dateText(detailItem.due_at)} />
                    </div>
                  </div>
                </div>

                <div className="space-y-4 p-4">
                  <WorkItemRequirementPanel rows={requirementRows} />
                  {primarySurface ? <WorkItemPrimarySurface item={detailItem} evidence={evidence} surface={primarySurface} /> : null}
                </div>
              </section>

              {primarySurface ? (
                <WorkItemSupportingTabs
                  item={detailItem}
                  evidence={evidence}
                  activities={activities}
                  attachmentOptions={attachmentOptions}
                  activitySubmitting={activityAction.isPending}
                  primarySurfaceKind={primarySurface.kind}
                  onSubmitActivity={(payload) => {
                    if (!detailItem.id) return Promise.resolve();
                    return activityAction.mutateAsync({ workItemId: detailItem.id, payload }).then(() => undefined);
                  }}
                />
              ) : null}
            </main>

            <WorkItemActionPanel
              item={detailItem}
              actions={actions}
              actionText={actionText}
              pendingAction={pendingAction}
              submitting={workAction.isPending}
              nextAction={nextAction}
              latestActivity={latestActivity}
              onActionTextChange={setActionText}
              onRunAction={runAction}
              onClose={closeDialog}
            />
          </div>
        </div>
      ) : null}
    </Dialog>
  );
}

type SupportTabKey = 'evidence' | 'activity' | 'form';
type RequirementState = 'done' | 'warning' | 'blocked' | 'neutral';

type RequirementRow = {
  id: string;
  label: string;
  description: string;
  state: RequirementState;
};

function WorkItemWorkspaceHeader({
  item,
  evidence,
  nextAction,
}: {
  item: WorkItem;
  evidence: EvidenceRecord[];
  nextAction: string;
}) {
  const activeEvidence = evidence.filter((record) => record.status !== 'rejected');
  const contextStats = [
    { label: 'Node', value: item.run_node_name ?? '-' },
    { label: 'Step', value: item.run_step_name ?? '-' },
    { label: 'Evidence', value: `${activeEvidence.length} active / ${numberValue(item.evidence_count)} total` },
    { label: 'Biểu mẫu', value: hasWorkflowFormSummary(item) ? `${workflowFormChipText(item)} · ${item.workflow_form_status ?? 'draft'}` : '-' },
  ];

  return (
    <section className="rounded-lg border border-line bg-white shadow-panel">
      <div className="flex flex-col gap-3 border-b border-line px-4 py-3 lg:flex-row lg:items-start lg:justify-between">
        <div className="min-w-0">
          <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">Workflow runtime</p>
          <h3 className="mt-1 text-base font-semibold text-ink">{item.title}</h3>
          <p className="mt-1 text-sm text-slate-600">Trạng thái, điều kiện hoàn thành và hành động tiếp theo.</p>
        </div>
        <div className="flex shrink-0 flex-wrap items-center gap-2">
          <StatusBadge status={isWorkOverdue(item) ? 'overdue' : item.status} />
          {item.project_code ? <span className="rounded-md bg-blue-50 px-2 py-1 text-xs font-semibold text-blue-700">{item.project_code}</span> : null}
          {item.priority ? <span className="rounded-md bg-slate-100 px-2 py-1 text-xs font-semibold text-slate-600">{priorityLabel(item.priority)}</span> : null}
        </div>
      </div>
      <div className="grid gap-px bg-line sm:grid-cols-2 xl:grid-cols-4">
        {contextStats.map((stat) => (
          <div key={stat.label} className="min-w-0 bg-white px-4 py-3">
            <p className="text-[11px] font-semibold uppercase tracking-wide text-slate-500">{stat.label}</p>
            <p className="mt-1 truncate text-sm font-semibold text-ink">{stat.value}</p>
          </div>
        ))}
      </div>
      <div className="flex items-start gap-2 border-t border-line bg-blue-50 px-4 py-3 text-sm">
        <PlayCircle className="mt-0.5 h-4 w-4 shrink-0 text-blue-700" />
        <div className="min-w-0">
          <span className="font-semibold text-blue-900">Việc tiếp theo: </span>
          <span className="text-blue-800">{nextAction}</span>
        </div>
      </div>
    </section>
  );
}

function WorkItemProgressRail({ item }: { item: WorkItem }) {
  const steps = [
    {
      id: 'project',
      label: 'Công trình',
      value: item.project_code ?? item.project_name ?? '-',
      meta: item.project_name && item.project_code ? item.project_name : '',
      state: 'context',
    },
    {
      id: 'node',
      label: 'Node workflow',
      value: item.run_node_name ?? 'Chưa gắn node',
      meta: item.run_node_order_index ? `Thứ tự ${item.run_node_order_index}` : '',
      state: item.run_node_name ? 'active' : 'neutral',
    },
    {
      id: 'step',
      label: 'Step',
      value: item.run_step_name ?? 'Chưa gắn step',
      meta: item.run_step_order_index ? `Thứ tự ${item.run_step_order_index}` : '',
      state: item.run_step_name ? 'current' : 'neutral',
    },
    {
      id: 'work-item',
      label: 'Work item',
      value: item.title,
      meta: workTypeLabel(item.work_type),
      state: 'current',
    },
  ];

  return (
    <aside className="h-fit rounded-lg border border-line bg-white p-3 shadow-panel xl:sticky xl:top-0 xl:self-start">
      <div className="border-b border-line pb-3">
        <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">Vị trí workflow</p>
        <h3 className="mt-1 text-sm font-semibold text-ink">Tiến trình</h3>
      </div>
      <ol className="mt-3 max-h-[calc(100vh-18rem)] space-y-0 overflow-y-auto pr-1">
        {steps.map((step, index) => {
          const current = step.state === 'current';
          const active = step.state === 'active';
          return (
            <li key={step.id} className="relative flex gap-3 pb-4 last:pb-0">
              {index < steps.length - 1 ? <span className="absolute left-[0.5625rem] top-5 h-[calc(100%-1rem)] w-px bg-line" /> : null}
              <span
                className={clsx(
                  'relative z-10 mt-0.5 grid h-5 w-5 shrink-0 place-items-center rounded-full ring-4 ring-white',
                  current
                    ? 'bg-blue-600 text-white'
                    : active
                      ? 'bg-emerald-500 text-white'
                      : 'bg-slate-200 text-slate-500',
                )}
              >
                {current ? <Circle className="h-2.5 w-2.5 fill-current" /> : active ? <CheckCircle2 className="h-3.5 w-3.5" /> : <Circle className="h-2.5 w-2.5" />}
              </span>
              <div className="min-w-0">
                <p className="text-[11px] font-semibold uppercase tracking-wide text-slate-500">{step.label}</p>
                <p className={clsx('mt-0.5 line-clamp-2 text-sm font-semibold', current ? 'text-blue-800' : 'text-ink')}>{step.value}</p>
                {step.meta ? <p className="mt-1 truncate text-xs text-slate-500">{step.meta}</p> : null}
              </div>
            </li>
          );
        })}
      </ol>
    </aside>
  );
}

function WorkItemRequirementPanel({ rows }: { rows: RequirementRow[] }) {
  return (
    <section className="rounded-md border border-line bg-slate-50 p-3">
      <div className="flex items-center justify-between gap-3">
        <div>
          <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">Điều kiện bắt buộc</p>
          <h3 className="mt-0.5 text-sm font-semibold text-ink">Gates trước khi hoàn thành</h3>
        </div>
        <span className="rounded-md bg-white px-2 py-1 text-xs font-semibold text-slate-600 ring-1 ring-inset ring-line">{rows.length} mục</span>
      </div>
      <div className="mt-3 divide-y divide-line rounded-md border border-line bg-white">
        {rows.map((row) => (
          <div key={row.id} className="flex items-start gap-3 px-3 py-2.5">
            <RequirementIcon state={row.state} />
            <div className="min-w-0">
              <p className="text-sm font-semibold text-ink">{row.label}</p>
              <p className="mt-0.5 text-xs leading-5 text-slate-600">{row.description}</p>
            </div>
          </div>
        ))}
      </div>
    </section>
  );
}

function WorkItemActionPanel({
  item,
  actions,
  actionText,
  pendingAction,
  submitting,
  nextAction,
  latestActivity,
  onActionTextChange,
  onRunAction,
  onClose,
}: {
  item: WorkItem;
  actions: WorkItemActionKey[];
  actionText: string;
  pendingAction: WorkItemActionKey | null;
  submitting: boolean;
  nextAction: string;
  latestActivity?: WorkItemActivity;
  onActionTextChange: (value: string) => void;
  onRunAction: (action: WorkItemActionKey) => void;
  onClose: () => void;
}) {
  const primaryAction = actions[0];
  const secondaryActions = actions.slice(1);

  return (
    <aside className="rounded-lg border border-line bg-white p-3 shadow-panel xl:sticky xl:top-0 xl:self-start">
      <div className="border-b border-line pb-3">
        <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">Việc cần làm</p>
        <h3 className="mt-1 text-sm font-semibold text-ink">{nextAction}</h3>
      </div>

      {actions.length ? (
        <div className="mt-3 space-y-3">
          <textarea
            className={clsx(
              'min-h-24 w-full rounded-md border px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-500',
              pendingAction && actionRequiresReason(pendingAction) && !actionText.trim() ? 'border-amber-300 bg-amber-50' : 'border-line bg-white',
            )}
            value={actionText}
            onChange={(event) => onActionTextChange(event.target.value)}
            placeholder="Ghi chú / lý do cho thao tác"
          />

          {primaryAction ? (
            <ActionButton action={primaryAction} primary submitting={submitting} onRunAction={onRunAction} />
          ) : null}

          {secondaryActions.length ? (
            <div className="space-y-2">
              {secondaryActions.map((action) => (
                <ActionButton key={action} action={action} submitting={submitting} onRunAction={onRunAction} />
              ))}
            </div>
          ) : null}
        </div>
      ) : (
        <p className="mt-3 rounded-md bg-slate-50 p-3 text-sm text-slate-600">Không có thao tác trực tiếp cho trạng thái hiện tại.</p>
      )}

      <div className="mt-4 space-y-3 border-t border-line pt-3">
        <ActionFact icon={<ShieldCheck className="h-4 w-4" />} label="Trạng thái" value={<StatusBadge status={isWorkOverdue(item) ? 'overdue' : item.status} />} />
        <ActionFact icon={<Users className="h-4 w-4" />} label="Phụ trách" value={item.assignee_name ?? firstRoleLabel(item) ?? departmentLabel(item.department)} />
        <ActionFact icon={<Clock3 className="h-4 w-4" />} label="Hạn xử lý" value={dateText(item.due_at)} />
        {latestActivity ? (
          <ActionFact
            icon={<MessageSquare className="h-4 w-4" />}
            label="Hoạt động gần nhất"
            value={`${latestActivity.actor_name ?? 'System'} · ${compactDateTime(latestActivity.created_at)}`}
          />
        ) : null}
      </div>

      <div className="mt-4 flex flex-col gap-2 border-t border-line pt-3">
        {item.project_id ? (
          <Link
            className="inline-flex items-center justify-center rounded-md border border-line px-3 py-2 text-sm font-semibold text-blue-700 hover:border-blue-300 hover:bg-blue-50"
            to={`/projects/${item.project_id}`}
          >
            Mở công trình
          </Link>
        ) : null}
        <button className="rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100" onClick={onClose}>
          Đóng
        </button>
      </div>
    </aside>
  );
}

function WorkItemSupportingTabs({
  item,
  evidence,
  activities,
  attachmentOptions,
  activitySubmitting,
  primarySurfaceKind,
  onSubmitActivity,
}: {
  item: WorkItem;
  evidence: EvidenceRecord[];
  activities: WorkItemActivity[];
  attachmentOptions: ActivityAttachmentOption[];
  activitySubmitting: boolean;
  primarySurfaceKind: WorkItemPrimarySurfaceKind;
  onSubmitActivity: (payload: ActivityPayload) => Promise<void>;
}) {
  const [selectedTab, setSelectedTab] = useState<SupportTabKey>('evidence');
  const activeEvidence = evidence.filter((record) => record.status !== 'rejected');
  const pendingEvidence = evidence.filter((record) => ['draft', 'submitted', 'pending'].includes(String(record.status)));
  const tabs: Array<{ id: SupportTabKey; label: string; count?: number; status?: string; description: string }> = [];
  if (!primarySurfaceUsesEvidence(primarySurfaceKind)) {
    tabs.push({
      id: 'evidence',
      label: 'Minh chứng',
      count: activeEvidence.length,
      description: pendingEvidence.length ? `${pendingEvidence.length} pending` : 'Sẵn sàng',
    });
  }
  tabs.push({
    id: 'activity',
    label: 'Hoạt động',
    count: activities.length,
    description: activities[0] ? `Mới nhất ${compactDateTime(activities[0].created_at)}` : 'Chưa có activity',
  });
  if (hasWorkflowFormSummary(item) && !primarySurfaceUsesWorkflowForm(primarySurfaceKind)) {
    tabs.push({
      id: 'form',
      label: 'Biểu mẫu / PDF',
      status: item.workflow_form_status ?? 'draft',
      description: workflowFormChipText(item),
    });
  }
  const activeTab = tabs.some((tab) => tab.id === selectedTab) ? selectedTab : tabs[0]?.id ?? 'activity';
  const activeTabMeta = tabs.find((tab) => tab.id === activeTab);
  const supportAction = resolveSupportTabAction(activeTab, item, evidence, activities);

  return (
    <section className="overflow-hidden rounded-lg border border-line bg-white shadow-panel">
      <div className="border-b border-line bg-white px-4 py-3">
        <div className="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
          <div className="min-w-0">
            <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">Thông tin hỗ trợ</p>
            <h3 className="mt-0.5 text-sm font-semibold text-ink">Minh chứng, trao đổi và biểu mẫu của work item</h3>
            <p className="mt-1 text-xs leading-5 text-slate-500">Theo dõi hồ sơ, trao đổi và artifact phục vụ bước đang xử lý.</p>
          </div>
          <div className="rounded-md border border-blue-100 bg-blue-50 px-3 py-2 text-xs lg:max-w-72">
            <p className="font-semibold uppercase tracking-wide text-blue-700">Việc hỗ trợ tiếp theo</p>
            <p className="mt-1 font-semibold text-blue-950">{supportAction}</p>
          </div>
        </div>

        <div className="mt-3 grid gap-2 md:grid-cols-3">
          {tabs.map((tab) => (
            <button
              key={tab.id}
              type="button"
              className={clsx(
                'min-w-0 rounded-md border px-3 py-2 text-left transition',
                activeTab === tab.id ? 'border-blue-300 bg-blue-50 shadow-sm' : 'border-line bg-slate-50 hover:border-blue-200 hover:bg-white',
              )}
              onClick={() => setSelectedTab(tab.id)}
            >
              <span className="flex min-w-0 items-center justify-between gap-2">
                <span className={clsx('truncate text-xs font-semibold uppercase tracking-wide', activeTab === tab.id ? 'text-blue-700' : 'text-slate-600')}>{tab.label}</span>
                {typeof tab.count === 'number' ? <span className="rounded bg-white px-1.5 py-0.5 text-[11px] font-semibold text-slate-700 ring-1 ring-inset ring-line">{tab.count}</span> : null}
                {tab.status ? <span className="rounded bg-emerald-50 px-1.5 py-0.5 text-[11px] font-semibold text-emerald-700 ring-1 ring-inset ring-emerald-100">{tab.status}</span> : null}
              </span>
              <span className="mt-1 block truncate text-xs text-slate-500">{tab.description}</span>
            </button>
          ))}
        </div>
      </div>

      <div className="border-b border-line bg-slate-50 px-4 py-2">
        <p className="text-xs font-semibold text-slate-600">
          Đang xem: <span className="text-ink">{activeTabMeta?.label ?? 'Thông tin hỗ trợ'}</span>
        </p>
      </div>

      <div className="bg-white p-4">
        {activeTab === 'evidence' ? <EvidencePanel workItemId={item.id} projectId={item.project_id} evidence={evidence} /> : null}
        {activeTab === 'activity' ? (
          <WorkItemActivityPanel
            activities={activities}
            attachmentOptions={attachmentOptions}
            submitting={activitySubmitting}
            onSubmit={onSubmitActivity}
          />
        ) : null}
        {activeTab === 'form' ? <WorkflowFormPanel workItemId={item.id} projectId={item.project_id} evidence={evidence} /> : null}
      </div>
    </section>
  );
}

function resolveSupportTabAction(activeTab: SupportTabKey, item: WorkItem, evidence: EvidenceRecord[], activities: WorkItemActivity[]) {
  if (activeTab === 'evidence') {
    const activeEvidence = evidence.filter((record) => record.status !== 'rejected');
    const pending = evidence.filter((record) => ['draft', 'submitted', 'pending'].includes(String(record.status))).length;
    if (!activeEvidence.length) return 'Bổ sung minh chứng cho work item';
    if (pending) return `Review ${pending} minh chứng đang pending`;
    return 'Minh chứng đã sẵn sàng';
  }
  if (activeTab === 'form') return workflowFormNextAction(item);
  return activities.length ? `Theo dõi ${activities.length} hoạt động gần nhất` : 'Ghi activity đầu tiên';
}

function WorkItemActivityPanel({
  activities,
  attachmentOptions,
  submitting,
  onSubmit,
}: {
  activities: WorkItemActivity[];
  attachmentOptions: ActivityAttachmentOption[];
  submitting: boolean;
  onSubmit: (payload: ActivityPayload) => Promise<void>;
}) {
  const [composerOpen, setComposerOpen] = useState(false);
  const latestActivity = activities[0];
  return (
    <section className="space-y-3">
      <div className="flex flex-col gap-3 rounded-md border border-line bg-slate-50 px-3 py-3 lg:flex-row lg:items-center lg:justify-between">
        <div className="min-w-0">
          <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">Dòng hoạt động</p>
          <h3 className="mt-0.5 text-sm font-semibold text-ink">{activities.length} hoạt động</h3>
          <p className="mt-1 text-xs text-slate-500">
            {latestActivity ? `${latestActivity.actor_name ?? 'System'} · ${compactDateTime(latestActivity.created_at)}` : 'Chưa có trao đổi hoặc sự kiện cho work item này.'}
          </p>
        </div>
        <button
          type="button"
          className="inline-flex items-center justify-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700"
          onClick={() => setComposerOpen((current) => !current)}
        >
          <MessageSquare className="h-4 w-4" />
          {composerOpen ? 'Ẩn composer' : 'Ghi activity'}
        </button>
      </div>

      {composerOpen ? (
        <ActivityComposer
          mode="work_item"
          attachmentOptions={attachmentOptions}
          submitting={submitting}
          onSubmit={(payload) => onSubmit(payload).then(() => setComposerOpen(false))}
        />
      ) : null}

      {activities.length ? (
        <div className="space-y-2">
          {activities.map((activity) => (
            <ActivityTimelineItem key={activity.id} activity={activity} />
          ))}
        </div>
      ) : (
        <EmptyState title="Chưa có activity cho work item này" />
      )}
    </section>
  );
}

function ActivityTimelineItem({ activity }: { activity: WorkItemActivity }) {
  return (
    <article className="rounded-md border border-line bg-white px-3 py-2.5">
      <div className="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
        <div className="min-w-0">
          <p className="text-sm font-semibold text-ink">{activityLabel(activity.activity_type)}</p>
          <p className="mt-1 text-xs text-slate-500">
            {activity.actor_name ?? 'System'} · {compactDateTime(activity.created_at)}
          </p>
        </div>
        {activity.visibility ? <span className="w-fit rounded-md bg-slate-100 px-2 py-0.5 text-xs font-semibold text-slate-600">{visibilityLabel(activity.visibility)}</span> : null}
      </div>
      {activity.comment ? <p className="mt-2 whitespace-pre-wrap break-words text-sm leading-5 text-slate-700">{activity.comment}</p> : null}
      <ActivityAttachments attachments={activity.attachments} />
    </article>
  );
}

function ActionButton({
  action,
  primary = false,
  submitting,
  onRunAction,
}: {
  action: WorkItemActionKey;
  primary?: boolean;
  submitting: boolean;
  onRunAction: (action: WorkItemActionKey) => void;
}) {
  return (
    <button
      type="button"
      className={clsx(
        'inline-flex w-full items-center justify-center gap-2 rounded-md px-3 py-2 text-sm font-semibold disabled:cursor-not-allowed disabled:opacity-60',
        primary
          ? 'bg-blue-600 text-white hover:bg-blue-700'
          : action === 'reject' || action === 'block'
            ? 'border border-rose-200 text-rose-700 hover:bg-rose-50'
            : 'border border-line text-slate-700 hover:bg-slate-100',
      )}
      disabled={submitting}
      onClick={() => onRunAction(action)}
      title={actionDescriptions[action]}
    >
      {submitting ? <Loader2 className="h-4 w-4 animate-spin" /> : null}
      {actionLabels[action]}
    </button>
  );
}

function ActionFact({ icon, label, value }: { icon: ReactNode; label: string; value: ReactNode }) {
  return (
    <div className="flex items-start gap-2">
      <span className="mt-0.5 shrink-0 text-slate-400">{icon}</span>
      <div className="min-w-0">
        <p className="text-[11px] font-semibold uppercase tracking-wide text-slate-500">{label}</p>
        <div className="mt-0.5 break-words text-sm font-semibold text-ink">{value}</div>
      </div>
    </div>
  );
}

function MetaPill({ icon, label }: { icon: ReactNode; label: string }) {
  return (
    <span className="inline-flex max-w-56 items-center gap-1.5 truncate rounded-md bg-slate-100 px-2 py-1 text-slate-700">
      <span className="shrink-0">{icon}</span>
      <span className="truncate">{label}</span>
    </span>
  );
}

function RequirementIcon({ state }: { state: RequirementState }) {
  if (state === 'done') return <CheckCircle2 className="mt-0.5 h-4 w-4 shrink-0 text-emerald-600" />;
  if (state === 'blocked') return <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0 text-rose-600" />;
  if (state === 'warning') return <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0 text-amber-600" />;
  return <Circle className="mt-0.5 h-4 w-4 shrink-0 text-slate-400" />;
}

function buildRequirementRows(item: WorkItem, evidence: EvidenceRecord[]): RequirementRow[] {
  const rows: RequirementRow[] = [];
  const requirement = item.evidence_requirement_json ?? {};
  const activeEvidence = evidence.filter((record) => record.status !== 'rejected');
  const acceptedEvidence = evidence.filter((record) => record.status === 'accepted');
  const requiredEvidence = requiredEvidenceLabels(requirement);

  if (item.blocked_reason) {
    rows.push({
      id: 'blocker',
      label: 'Blocker đang mở',
      description: item.blocked_reason,
      state: 'blocked',
    });
  }

  if (hasWorkflowFormSummary(item)) {
    const submitted = item.workflow_form_status === 'submitted';
    const required = Boolean(item.workflow_form_required);
    rows.push({
      id: 'workflow-form',
      label: required ? 'Biểu mẫu nghiệp vụ bắt buộc' : 'Biểu mẫu nghiệp vụ',
      description: submitted
        ? `${workflowFormChipText(item)} đã được submit.`
        : `${workflowFormChipText(item)} đang ở trạng thái ${item.workflow_form_status ?? 'draft'}.`,
      state: submitted ? 'done' : required ? 'blocked' : 'warning',
    });
  }

  if (requiredEvidence.length) {
    rows.push({
      id: 'evidence',
      label: 'Minh chứng bắt buộc',
      description: activeEvidence.length
        ? `${activeEvidence.length} minh chứng đang hiệu lực, ${acceptedEvidence.length} đã được duyệt.`
        : `Cần bổ sung: ${requiredEvidence.join(', ')}.`,
      state: activeEvidence.length ? 'done' : 'blocked',
    });
  } else {
    rows.push({
      id: 'evidence-optional',
      label: 'Minh chứng',
      description: activeEvidence.length ? `${activeEvidence.length} minh chứng đang hiệu lực.` : 'Không có yêu cầu evidence đặc biệt.',
      state: activeEvidence.length ? 'done' : 'neutral',
    });
  }

  if (item.status === 'waiting_approval' || item.status === 'waiting_review' || item.approval_request_id) {
    rows.push({
      id: 'approval',
      label: item.status === 'waiting_review' ? 'Đang chờ review' : 'Phê duyệt',
      description: item.approval_request_id ? 'Approval request đã được liên kết với work item.' : 'Work item đang ở hàng chờ xử lý.',
      state: item.status === 'waiting_approval' || item.status === 'waiting_review' ? 'warning' : 'done',
    });
  }

  if (requirement.required_permission) {
    rows.push({
      id: 'permission',
      label: 'Quyền xử lý',
      description: `Yêu cầu quyền: ${requirement.required_permission}.`,
      state: 'neutral',
    });
  }

  return rows;
}

function resolveNextAction(
  item: WorkItem,
  evidence: EvidenceRecord[],
  actions: WorkItemActionKey[],
  primarySurfaceKind?: WorkItemPrimarySurfaceKind,
) {
  const requirement = item.evidence_requirement_json ?? {};
  const activeEvidence = evidence.filter((record) => record.status !== 'rejected');
  const requiredEvidence = requiredEvidenceLabels(requirement);
  const missingRequiredEvidence = requiredEvidence.length > 0 && activeEvidence.length === 0;
  const missingRequiredForm = hasWorkflowFormSummary(item) && item.workflow_form_required && item.workflow_form_status !== 'submitted';

  if (item.status === 'blocked') return actionLabels['resolve-blocker'];
  if (missingRequiredForm) return workflowFormNextAction(item);
  if (missingRequiredEvidence) return 'Bổ sung minh chứng';
  if (primarySurfaceKind) {
    const surfaceAction = primarySurfaceNextAction(primarySurfaceKind, actions);
    if (surfaceAction) return surfaceAction;
  }
  if (actions.length) return actionLabels[actions[0]];
  return 'Theo dõi hoạt động';
}

function primarySurfaceNextAction(kind: WorkItemPrimarySurfaceKind, actions: WorkItemActionKey[]) {
  switch (kind) {
    case 'evidence':
      return 'Bổ sung minh chứng';
    case 'incoming_acceptance':
      return 'Ghi nhận nghiệm thu đầu vào';
    case 'material_snapshot':
      return 'Lưu snapshot vật tư';
    case 'surplus_handling':
      return 'Xử lý vật tư dư';
    case 'outgoing_acceptance':
      return 'Lập biên bản nghiệm thu';
    case 'owner_client_reconciliation':
      return 'Đối chiếu chủ đầu tư';
    case 'subcontractor_reconciliation':
      return 'Đối chiếu thầu phụ';
    case 'workflow_form':
      return 'Hoàn tất biểu mẫu nghiệp vụ';
    case 'approval':
      return actions.length ? actionLabels[actions[0]] : 'Theo dõi phê duyệt';
    case 'generic':
      return actions.length ? actionLabels[actions[0]] : 'Theo dõi hoạt động';
    default:
      return null;
  }
}

function requiredEvidenceLabels(requirement: Record<string, unknown>) {
  return [
    requirement.checklist_required ? 'Checklist' : '',
    requirement.document_required ? 'Tài liệu' : '',
    requirement.photo_required ? 'Ảnh' : '',
  ].filter(Boolean);
}

function firstRoleLabel(item: WorkItem) {
  return asArray<string>(item.role_ids).filter(Boolean)[0];
}

function buildActivityAttachmentOptions(evidence: EvidenceRecord[]): ActivityAttachmentOption[] {
  const options: ActivityAttachmentOption[] = [];
  evidence.forEach((record) => {
    const label = record.name ?? record.title ?? record.document_name ?? record.id;
    options.push({
      type: 'evidence',
      id: record.id,
      label,
      status: record.status,
      visibility: record.visibility,
    });
    if (record.document_id) {
      options.push({
        type: 'document',
        id: record.document_id,
        label: record.document_name ?? label,
        status: record.status,
        visibility: record.visibility,
      });
    }
  });
  return options;
}
