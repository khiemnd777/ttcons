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
import { statusLabel } from '../../lib/utils/status';
import { useToast } from '../../components/ToastProvider';
import type { ActivityPayload, EvidenceRecord, ProjectActivityOptions, WorkItem, WorkItemActivity } from '../../lib/types/domain';
import { ActivityAttachments } from './ActivityAttachments';
import { ActivityReactionBar, ActivityReplyQuote } from './ActivityConversationMeta';
import { ActivityComposer, type ActivityAttachmentOption, type ActivityMentionOption } from './ActivityComposer';
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
  const workContextLabel = item?.run_node_name
    ?? item?.run_step_name
    ?? (item?.work_type ? workTypeLabel(item.work_type) : 'Đầu việc đang chạy');

  return (
    <Dialog
      open={open}
      title={item?.title ?? 'Chi tiết đầu việc'}
      description={item ? `${item.project_name ?? 'Công trình'} · ${departmentLabel(item.department)} · ${workContextLabel}` : undefined}
      onClose={onClose}
      size="workspace"
    >
      <WorkItemDetailWorkspace item={item} onClose={onClose} onChanged={onChanged} />
    </Dialog>
  );
}

export function WorkItemDetailWorkspace({
  item,
  workItemId,
  onClose,
  onChanged,
}: {
  item?: WorkItem | null;
  workItemId?: string;
  onClose?: () => void;
  onChanged?: (item: WorkItem) => void;
}) {
  const [actionText, setActionText] = useState('');
  const [pendingAction, setPendingAction] = useState<WorkItemActionKey | null>(null);
  const queryClient = useQueryClient();
  const toast = useToast();
  const selectedWorkId = workItemId ?? item?.id ?? '';
  const workDetail = useQuery({
    queryKey: ['work-item', selectedWorkId],
    queryFn: () => get<WorkItem>(`/work-items/${selectedWorkId}`),
    enabled: Boolean(selectedWorkId),
    retry: false,
  });
  const detailItem = workDetail.data ?? item;
  const activityOptions = useQuery({
    queryKey: ['project-activity-options', detailItem?.project_id],
    queryFn: () => get<ProjectActivityOptions>(`/projects/${detailItem?.project_id}/activity/options`),
    enabled: Boolean(detailItem?.project_id),
    retry: false,
  });
  const activityMentionOptions: ActivityMentionOption[] = [
    ...(activityOptions.data?.members ?? []).map((member) => ({
      type: 'staff' as const,
      id: member.id,
      label: member.label,
      description: [member.position, member.department].filter(Boolean).join(' · '),
    })),
    ...(activityOptions.data?.roles ?? []).map((role) => ({ type: 'role' as const, id: role.id, label: role.label })),
  ];

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
    onError: (error) => toast.error('Không thao tác được đầu việc', error),
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
      toast.success('Đã gửi hoạt động');
    },
    onError: (error) => toast.error('Không gửi được hoạt động', error),
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
    onClose?.();
  }

  const actions = availableActions(detailItem?.status);
  const evidence = asArray<EvidenceRecord>(detailItem?.evidence);
  const activities = asArray<WorkItemActivity>(detailItem?.activities);
  const attachmentOptions = buildActivityAttachmentOptions(evidence);
  const primarySurface = detailItem ? resolveWorkItemPrimarySurface(detailItem) : null;
  const requirementRows = detailItem ? buildRequirementRows(detailItem, evidence) : [];
  const nextAction = detailItem ? resolveNextAction(detailItem, evidence, actions, primarySurface?.kind) : '';
  const latestActivity = activities[0];
  const skippedHistory = detailItem?.status === 'cancelled';
  return (
    <>
      {workDetail.isLoading && !detailItem ? <LoadingSpinner /> : null}
      {workDetail.error && !detailItem ? <EmptyState title="Không tải được chi tiết đầu việc" /> : null}
      {detailItem ? (
        <div className="space-y-4">
          <WorkItemWorkspaceHeader item={detailItem} evidence={evidence} nextAction={nextAction} />

          <div className="grid items-start gap-4 xl:grid-cols-[minmax(0,1fr)_18rem]">
            <main className="min-w-0 space-y-4">
              <section className="rounded-lg border border-line bg-white shadow-panel">
                <div className="border-b border-line px-4 py-3">
                  <p className="text-xs font-semibold uppercase tracking-wide text-blue-700">{skippedHistory ? 'Bước đã bỏ qua' : 'Bước đang xử lý'}</p>
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
                  {skippedHistory ? (
                    <div className="rounded-md border border-slate-200 bg-slate-50 px-3 py-3 text-sm text-slate-700">
                      <p className="font-semibold text-ink">Hồ sơ lịch sử chỉ đọc</p>
                      <p className="mt-1">Work item thuộc nhánh không được lựa chọn. Nội dung được giữ lại để truy vết và không thể thao tác nghiệp vụ.</p>
                    </div>
                  ) : (
                    <>
                      <WorkItemRequirementPanel rows={requirementRows} />
                      {primarySurface ? <WorkItemPrimarySurface item={detailItem} evidence={evidence} surface={primarySurface} /> : null}
                    </>
                  )}
                </div>
              </section>

              {skippedHistory ? (
                <SkippedWorkItemHistory activities={activities} evidence={evidence} />
              ) : primarySurface ? (
                <WorkItemSupportingTabs
                  item={detailItem}
                  evidence={evidence}
                  activities={activities}
                  attachmentOptions={attachmentOptions}
                  mentionOptions={activityMentionOptions}
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
    </>
  );
}

function SkippedWorkItemHistory({ activities, evidence }: { activities: WorkItemActivity[]; evidence: EvidenceRecord[] }) {
  return (
    <section className="rounded-lg border border-line bg-white p-4 shadow-panel">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div>
          <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">Lịch sử workflow</p>
          <h3 className="mt-1 text-sm font-semibold text-ink">Lý do và dấu vết của bước đã bỏ qua</h3>
        </div>
        <span className="rounded-md bg-slate-100 px-2 py-1 text-xs font-semibold text-slate-600">{activities.length} hoạt động · {evidence.length} minh chứng</span>
      </div>
      {activities.length ? (
        <div className="mt-3 divide-y divide-line rounded-md border border-line">
          {activities.map((activity) => (
            <div key={activity.id} className="px-3 py-2.5">
              <div className="flex flex-wrap items-center justify-between gap-2 text-xs text-slate-500">
                <span className="font-semibold text-slate-700">{activity.actor_name ?? 'Hệ thống'}</span>
                <span>{compactDateTime(activity.created_at)}</span>
              </div>
              <p className="mt-1 text-sm text-slate-700">{activity.comment || statusLabel(activity.to_status ?? activity.activity_type ?? 'cancelled')}</p>
            </div>
          ))}
        </div>
      ) : (
        <p className="mt-3 rounded-md bg-slate-50 px-3 py-2 text-sm text-slate-600">Chưa có hoạt động lịch sử.</p>
      )}
    </section>
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
    { label: 'Bước', value: item.run_step_name ?? '-' },
    { label: 'Minh chứng', value: `${activeEvidence.length} hiệu lực / ${numberValue(item.evidence_count)} tổng` },
    { label: 'Hồ sơ', value: hasWorkflowFormSummary(item) ? `${workflowFormChipText(item)} · ${statusLabel(item.workflow_form_status ?? 'draft')}` : '-' },
  ];

  return (
    <section className="rounded-lg border border-line bg-white shadow-panel">
      <div className="flex flex-col gap-3 border-b border-line px-4 py-3 lg:flex-row lg:items-start lg:justify-between">
        <div className="min-w-0">
          <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">Quy trình đang chạy</p>
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

function WorkItemRequirementPanel({ rows }: { rows: RequirementRow[] }) {
  return (
    <section className="rounded-md border border-line bg-slate-50 p-3">
      <div className="flex items-center justify-between gap-3">
        <div>
          <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">Điều kiện bắt buộc</p>
          <h3 className="mt-0.5 text-sm font-semibold text-ink">Điều kiện trước khi hoàn thành</h3>
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
    <aside className="rounded-lg border border-line bg-white p-3 shadow-panel xl:sticky xl:top-[5.5rem] xl:self-start">
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
            value={`${latestActivity.actor_name ?? 'Hệ thống'} · ${compactDateTime(latestActivity.created_at)}`}
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
  mentionOptions,
  activitySubmitting,
  primarySurfaceKind,
  onSubmitActivity,
}: {
  item: WorkItem;
  evidence: EvidenceRecord[];
  activities: WorkItemActivity[];
  attachmentOptions: ActivityAttachmentOption[];
  mentionOptions: ActivityMentionOption[];
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
      description: pendingEvidence.length ? `${pendingEvidence.length} đang chờ` : 'Sẵn sàng',
    });
  }
  tabs.push({
    id: 'activity',
    label: 'Hoạt động',
    count: activities.length,
    description: activities[0] ? `Mới nhất ${compactDateTime(activities[0].created_at)}` : 'Chưa có hoạt động',
  });
  if (hasWorkflowFormSummary(item) && !primarySurfaceUsesWorkflowForm(primarySurfaceKind)) {
    tabs.push({
      id: 'form',
      label: 'Hồ sơ / PDF',
      status: item.workflow_form_status ?? 'draft',
      description: workflowFormChipText(item),
    });
  }
  const activeTab = tabs.some((tab) => tab.id === selectedTab) ? selectedTab : tabs[0]?.id ?? 'activity';
  const supportAction = resolveSupportTabAction(activeTab, item, evidence, activities);

  return (
    <section className="overflow-hidden rounded-lg border border-line bg-white shadow-panel">
      <div className="border-b border-line bg-white px-4 pt-3">
        <div className="space-y-3">
          <div className="min-w-0">
            <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">Thông tin hỗ trợ</p>
            <h3 className="mt-0.5 text-sm font-semibold text-ink">Minh chứng, trao đổi và hồ sơ của đầu việc</h3>
            <p className="mt-1 text-xs leading-5 text-slate-500">Theo dõi hồ sơ, trao đổi và tài liệu phục vụ bước đang xử lý.</p>
          </div>
          <div className="rounded-md border border-blue-100 bg-blue-50 px-3 py-2 text-xs">
            <p className="font-semibold uppercase tracking-wide text-blue-700">Việc hỗ trợ tiếp theo</p>
            <p className="mt-1 font-semibold text-blue-950">{supportAction}</p>
          </div>
        </div>

        <div className="mt-3 flex overflow-x-auto" role="tablist" aria-label="Thông tin hỗ trợ">
          {tabs.map((tab) => (
            <button
              key={tab.id}
              type="button"
              role="tab"
              id={`work-item-support-tab-${tab.id}`}
              aria-selected={activeTab === tab.id}
              aria-controls={`work-item-support-panel-${tab.id}`}
              className={clsx(
                'inline-flex min-h-10 shrink-0 items-center gap-2 border-b-2 px-3 py-2 text-sm font-semibold transition',
                activeTab === tab.id ? 'border-blue-600 text-blue-700' : 'border-transparent text-slate-600 hover:border-slate-300 hover:text-ink',
              )}
              onClick={() => setSelectedTab(tab.id)}
            >
              <span>{tab.label}</span>
              {typeof tab.count === 'number' ? <span className="rounded-full bg-slate-100 px-2 py-0.5 text-[11px] font-semibold text-slate-700 ring-1 ring-inset ring-line">{tab.count}</span> : null}
              {tab.status ? <span className="rounded-full bg-emerald-50 px-2 py-0.5 text-[11px] font-semibold text-emerald-700 ring-1 ring-inset ring-emerald-100">{statusLabel(tab.status)}</span> : null}
              <span className="sr-only">{tab.description}</span>
            </button>
          ))}
        </div>
      </div>

      <div
        role="tabpanel"
        id={`work-item-support-panel-${activeTab}`}
        aria-labelledby={`work-item-support-tab-${activeTab}`}
        className="bg-white p-4"
      >
        {activeTab === 'evidence' ? <EvidencePanel workItemId={item.id} projectId={item.project_id} evidence={evidence} /> : null}
        {activeTab === 'activity' ? (
          <WorkItemActivityPanel
            activities={activities}
            attachmentOptions={attachmentOptions}
            mentionOptions={mentionOptions}
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
    if (!activeEvidence.length) return 'Bổ sung minh chứng cho đầu việc';
    if (pending) return `Rà soát ${pending} minh chứng đang chờ`;
    return 'Minh chứng đã sẵn sàng';
  }
  if (activeTab === 'form') return workflowFormNextAction(item);
  return activities.length ? `Theo dõi ${activities.length} hoạt động gần nhất` : 'Ghi hoạt động đầu tiên';
}

function WorkItemActivityPanel({
  activities,
  attachmentOptions,
  mentionOptions,
  submitting,
  onSubmit,
}: {
  activities: WorkItemActivity[];
  attachmentOptions: ActivityAttachmentOption[];
  mentionOptions: ActivityMentionOption[];
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
            {latestActivity ? `${latestActivity.actor_name ?? 'Hệ thống'} · ${compactDateTime(latestActivity.created_at)}` : 'Chưa có trao đổi hoặc sự kiện cho đầu việc này.'}
          </p>
        </div>
        <button
          type="button"
          className="inline-flex items-center justify-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700"
          onClick={() => setComposerOpen((current) => !current)}
        >
          <MessageSquare className="h-4 w-4" />
          {composerOpen ? 'Ẩn ô nhập' : 'Ghi hoạt động'}
        </button>
      </div>

      {composerOpen ? (
        <ActivityComposer
          mode="work_item"
          attachmentOptions={attachmentOptions}
          mentionOptions={mentionOptions}
          defaultExpanded
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
        <EmptyState title="Chưa có hoạt động cho đầu việc này" />
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
            {activity.actor_name ?? 'Hệ thống'} · {compactDateTime(activity.created_at)}
          </p>
        </div>
        {activity.visibility ? <span className="w-fit rounded-md bg-slate-100 px-2 py-0.5 text-xs font-semibold text-slate-600">{visibilityLabel(activity.visibility)}</span> : null}
      </div>
      <ActivityReplyQuote replyTo={activity.reply_to} />
      {activity.withdrawn_at ? <p className="mt-2 text-sm italic text-slate-500">Tin nhắn đã được thu hồi</p> : activity.comment ? <p className="mt-2 whitespace-pre-wrap break-words text-sm leading-5 text-slate-700">{activity.comment}</p> : null}
      <ActivityAttachments attachments={activity.attachments} />
      {!activity.withdrawn_at && activity.reactions?.length ? <div className="mt-2"><ActivityReactionBar reactions={activity.reactions} /></div> : null}
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
      label: 'Điểm chặn đang mở',
      description: item.blocked_reason,
      state: 'blocked',
    });
  }

  if (item.domain_action_key) {
    const latestAutomation = (item.automation_events ?? []).find((event) => event.action_key === item.domain_action_key);
    const automationStatus = latestAutomation?.status;
    const reason = latestAutomation?.blocker_reason || latestAutomation?.error_message;
    rows.push({
      id: 'required-automation',
      label: 'Tự động hóa nghiệp vụ bắt buộc',
      description: automationStatus === 'succeeded'
        ? `${item.domain_action_key} đã hoàn tất thành công.`
        : automationStatus === 'blocked' || automationStatus === 'failed'
          ? `${item.domain_action_key}: ${reason || 'Đang bị chặn; dùng lại thao tác hoàn tất sau khi khắc phục.'}`
          : `${item.domain_action_key} sẽ được kiểm tra trước khi đầu việc hoàn tất.`,
      state: automationStatus === 'succeeded' || automationStatus === 'skipped'
        ? 'done'
        : automationStatus === 'blocked' || automationStatus === 'failed'
          ? 'blocked'
          : 'neutral',
    });
  }

  if (hasWorkflowFormSummary(item)) {
    const submitted = item.workflow_form_status === 'submitted';
    const required = Boolean(item.workflow_form_required);
    rows.push({
      id: 'workflow-form',
      label: required ? 'Hồ sơ nghiệp vụ bắt buộc' : 'Hồ sơ nghiệp vụ',
      description: submitted
        ? `${workflowFormChipText(item)} đã được gửi.`
        : `${workflowFormChipText(item)} đang ở trạng thái ${statusLabel(item.workflow_form_status ?? 'draft')}.`,
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
      description: activeEvidence.length ? `${activeEvidence.length} minh chứng đang hiệu lực.` : 'Không có yêu cầu minh chứng đặc biệt.',
      state: activeEvidence.length ? 'done' : 'neutral',
    });
  }

  if (item.status === 'waiting_approval' || item.status === 'waiting_review' || item.approval_request_id) {
    rows.push({
      id: 'approval',
      label: item.status === 'waiting_review' ? 'Đang chờ rà soát' : 'Phê duyệt',
      description: item.approval_request_id ? 'Yêu cầu phê duyệt đã được liên kết với đầu việc.' : 'Đầu việc đang ở hàng chờ xử lý.',
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
      return 'Lưu ghi nhận vật tư';
    case 'surplus_handling':
      return 'Xử lý vật tư dư';
    case 'outgoing_acceptance':
      return 'Lập biên bản nghiệm thu';
    case 'owner_client_reconciliation':
      return 'Đối chiếu chủ đầu tư';
    case 'subcontractor_reconciliation':
      return 'Đối chiếu thầu phụ';
    case 'workflow_form':
      return 'Hoàn tất hồ sơ nghiệp vụ';
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
