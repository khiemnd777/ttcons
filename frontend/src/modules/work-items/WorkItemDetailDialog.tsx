import { ReactNode, useState } from 'react';
import { Link } from 'react-router-dom';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Loader2 } from 'lucide-react';
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
import { IncomingAcceptancePanel } from './IncomingAcceptancePanel';
import { MaterialSnapshotPanel } from './MaterialSnapshotPanel';
import { OwnerClientReconciliationPanel } from './OwnerClientReconciliationPanel';
import { OutgoingAcceptancePanel } from './OutgoingAcceptancePanel';
import { SubcontractorReconciliationPanel } from './SubcontractorReconciliationPanel';
import { SurplusHandlingPanel } from './SurplusHandlingPanel';
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
  priorityLabel,
  isWorkOverdue,
  numberValue,
  visibilityLabel,
  workTypeLabel,
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
  const requirement = detailItem?.evidence_requirement_json ?? {};
  const workContextLabel = detailItem?.run_node_name
    ?? detailItem?.run_step_name
    ?? (detailItem?.work_type ? workTypeLabel(detailItem.work_type) : 'Runtime work');

  return (
    <Dialog
      open={open}
      title={detailItem?.title ?? 'Work item'}
      description={detailItem ? `${detailItem.project_name ?? 'Project'} · ${departmentLabel(detailItem.department)} · ${workContextLabel}` : undefined}
      onClose={closeDialog}
      size="xl"
      footer={
        detailItem ? (
          <div className="space-y-3">
            {actions.length ? (
              <textarea
                className={clsx(
                  'min-h-20 w-full rounded-md border px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-500',
                  pendingAction && actionRequiresReason(pendingAction) && !actionText.trim() ? 'border-amber-300 bg-amber-50' : 'border-line bg-white',
                )}
                value={actionText}
                onChange={(event) => setActionText(event.target.value)}
                placeholder="Ghi chú / lý do cho thao tác"
              />
            ) : null}
            <div className="flex flex-wrap justify-between gap-2">
              {detailItem.project_id ? (
                <Link
                  className="inline-flex items-center rounded-md border border-line px-3 py-2 text-sm font-semibold text-blue-700 hover:border-blue-300 hover:bg-blue-50"
                  to={`/projects/${detailItem.project_id}`}
                >
                  Open project dashboard
                </Link>
              ) : <span />}
              <div className="flex flex-wrap justify-end gap-2">
                <button className="rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100" onClick={closeDialog}>
                  Đóng
                </button>
                {actions.map((action, index) => (
                  <button
                    key={action}
                    className={clsx(
                      'inline-flex items-center gap-2 rounded-md px-3 py-2 text-sm font-semibold disabled:cursor-not-allowed disabled:opacity-60',
                      index === 0
                        ? 'bg-blue-600 text-white hover:bg-blue-700'
                        : action === 'reject' || action === 'block'
                          ? 'border border-rose-200 text-rose-700 hover:bg-rose-50'
                          : 'border border-line text-slate-700 hover:bg-slate-100',
                    )}
                    disabled={workAction.isPending}
                    onClick={() => runAction(action)}
                    title={actionDescriptions[action]}
                  >
                    {workAction.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : null}
                    {actionLabels[action]}
                  </button>
                ))}
              </div>
            </div>
          </div>
        ) : null
      }
    >
      {workDetail.isLoading && !detailItem ? <LoadingSpinner /> : null}
      {detailItem ? (
        <div className="space-y-5">
          <div className="flex flex-wrap items-center gap-2">
            <StatusBadge status={isWorkOverdue(detailItem) ? 'overdue' : detailItem.status} />
            {detailItem.project_code ? <span className="rounded-md bg-blue-50 px-2 py-1 text-xs font-semibold text-blue-700">{detailItem.project_code}</span> : null}
            {detailItem.priority ? <span className="rounded-md bg-slate-100 px-2 py-1 text-xs font-semibold text-slate-600">Priority: {priorityLabel(detailItem.priority)}</span> : null}
            {detailItem.work_type ? <span className="rounded-md bg-slate-100 px-2 py-1 text-xs font-semibold text-slate-600">{workTypeLabel(detailItem.work_type)}</span> : null}
          </div>

          <div className="grid gap-3 text-sm sm:grid-cols-2 xl:grid-cols-4">
            <DetailMetric label="Owner" value={detailItem.assignee_name ?? (asArray<string>(detailItem.role_ids).join(', ') || '-')} />
            <DetailMetric label="Department" value={departmentLabel(detailItem.department)} />
            <DetailMetric label="Due" value={dateText(detailItem.due_at)} />
            <DetailMetric label="Evidence" value={numberValue(detailItem.evidence_count)} />
          </div>

          {detailItem.description ? <p className="rounded-md border border-line bg-slate-50 p-3 text-sm leading-6 text-slate-700">{detailItem.description}</p> : null}
          {detailItem.blocked_reason ? <p className="rounded-md border border-rose-200 bg-rose-50 p-3 text-sm leading-6 text-rose-700">{detailItem.blocked_reason}</p> : null}

          <section>
            <h3 className="mb-2 text-sm font-semibold text-ink">Required Evidence</h3>
            <div className="flex flex-wrap gap-2">
              {requirement.checklist_required ? <RequirementChip label="Checklist" /> : null}
              {requirement.document_required ? <RequirementChip label="Document" /> : null}
              {requirement.photo_required ? <RequirementChip label="Photo" /> : null}
              {requirement.required_permission ? <RequirementChip label={`Permission: ${requirement.required_permission}`} /> : null}
              {!requirement.checklist_required && !requirement.document_required && !requirement.photo_required && !requirement.required_permission ? (
                <span className="text-sm text-slate-500">Không có requirement đặc biệt.</span>
              ) : null}
            </div>
          </section>

          <WorkflowFormPanel workItemId={detailItem.id} projectId={detailItem.project_id} evidence={evidence} />

          <SubcontractorReconciliationPanel workItemId={detailItem.id} projectId={detailItem.project_id} />

          <OwnerClientReconciliationPanel workItemId={detailItem.id} projectId={detailItem.project_id} />

          <MaterialSnapshotPanel workItemId={detailItem.id} projectId={detailItem.project_id} evidence={evidence} />

          <SurplusHandlingPanel workItemId={detailItem.id} projectId={detailItem.project_id} evidence={evidence} />

          <IncomingAcceptancePanel workItemId={detailItem.id} projectId={detailItem.project_id} evidence={evidence} />

          <OutgoingAcceptancePanel workItemId={detailItem.id} projectId={detailItem.project_id} evidence={evidence} />

          <div className="grid gap-4 xl:grid-cols-2">
            <EvidencePanel workItemId={detailItem.id} projectId={detailItem.project_id} evidence={evidence} />
            <section>
              <h3 className="mb-2 text-sm font-semibold text-ink">Activity</h3>
              <ActivityComposer
                mode="work_item"
                attachmentOptions={attachmentOptions}
                submitting={activityAction.isPending}
                onSubmit={(payload) => {
                  if (!detailItem.id) return Promise.resolve();
                  return activityAction.mutateAsync({ workItemId: detailItem.id, payload }).then(() => undefined);
                }}
              />
              {activities.length ? (
                <div className="mt-3 space-y-2">
                  {activities.map((activity) => (
                    <div key={activity.id} className="rounded-md border border-line px-3 py-2">
                      <div className="flex items-start justify-between gap-3">
                        <p className="text-sm font-semibold text-ink">{activityLabel(activity.activity_type)}</p>
                        {activity.visibility ? <span className="rounded-md bg-slate-100 px-2 py-0.5 text-xs font-semibold text-slate-600">{visibilityLabel(activity.visibility)}</span> : null}
                      </div>
                      <p className="mt-1 text-xs text-slate-500">
                        {activity.actor_name ?? 'System'} · {compactDateTime(activity.created_at)}
                      </p>
                      {activity.comment ? <p className="mt-2 text-sm text-slate-700">{activity.comment}</p> : null}
                      <ActivityAttachments attachments={activity.attachments} />
                    </div>
                  ))}
                </div>
              ) : (
                <EmptyState title="Chưa có activity cho work item này" />
              )}
            </section>
          </div>
        </div>
      ) : null}
    </Dialog>
  );
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

function DetailMetric({ label, value }: { label: string; value: ReactNode }) {
  return (
    <div className="rounded-md border border-line bg-white p-3">
      <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">{label}</p>
      <div className="mt-1 text-sm font-semibold text-ink">{value}</div>
    </div>
  );
}

function RequirementChip({ label }: { label: string }) {
  return <span className="rounded-md bg-blue-50 px-2 py-1 text-xs font-semibold text-blue-700 ring-1 ring-inset ring-blue-200">{label}</span>;
}
