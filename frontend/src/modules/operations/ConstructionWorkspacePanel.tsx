import { useEffect, useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { ClipboardCheck, GitPullRequestArrow, Plus, Settings2, Wrench } from 'lucide-react';
import { LoadingSpinner } from '../../components/LoadingSpinner';
import { useToast } from '../../components/ToastProvider';
import { get, post } from '../../lib/api/client';
import { useAuth } from '../../lib/auth/AuthContext';
import type { ConstructionActionItem, ConstructionCheck, ConstructionVariationContext, ConstructionWorkspace } from '../../lib/types/domain';
import { ConstructionActionDialogs, type ActionDialogState } from './ConstructionActionDialogs';
import { ConstructionCheckDialogs, type CheckDialogState } from './ConstructionCheckDialogs';
import { ConstructionActionList, ConstructionCheckList } from './ConstructionWorkspaceLists';

export type SimplifiedConstructionMode = 'checks' | 'actions';

export type ConstructionWorkspaceDialogRequest =
  | { mode: 'checks'; state: Exclude<CheckDialogState, null> }
  | { mode: 'actions'; state: Exclude<ActionDialogState, null> };

export type ConstructionVariationSource = { workItemId: string; action?: ConstructionActionItem };

type Props = {
  projectId: string;
  mode: SimplifiedConstructionMode;
  dialogRequest?: ConstructionWorkspaceDialogRequest | null;
  variationContext?: ConstructionVariationContext;
  variationContextLoading?: boolean;
  variationContextError?: boolean;
  canViewContracts?: boolean;
  onDialogRequestHandled?: () => void;
  onViewReport?: (reportId: string) => void;
  onRetryVariationContext?: () => void;
  onProposeVariation?: (source: ConstructionVariationSource) => void;
};

export function ConstructionWorkspacePanel({ projectId, mode, dialogRequest, variationContext, variationContextLoading = false, variationContextError = false, canViewContracts = false, onDialogRequestHandled, onViewReport, onRetryVariationContext, onProposeVariation }: Props) {
  const [checkDialog, setCheckDialog] = useState<CheckDialogState>(null);
  const [actionDialog, setActionDialog] = useState<ActionDialogState>(null);
  const queryClient = useQueryClient();
  const toast = useToast();
  const { hasAnyPermission } = useAuth();
  const permissions = {
    manageTemplates: hasAnyPermission(['construction.template.manage']),
    requestCheck: hasAnyPermission(['construction.check.request']),
    editCheck: hasAnyPermission(['construction.check.edit']),
    performCheck: hasAnyPermission(['construction.check.perform']),
    createAction: hasAnyPermission(['construction.action.create']),
    editAction: hasAnyPermission(['construction.action.edit']),
    resolveAction: hasAnyPermission(['construction.action.resolve']),
    verifyAction: hasAnyPermission(['construction.action.verify', 'replanning.approve']),
    approveImpact: hasAnyPermission(['replanning.approve', 'construction.action.verify']),
	    proposeVariation: hasAnyPermission(['contract.variation.propose']),
  };
  const workspace = useQuery({
    queryKey: ['construction-workspace', projectId],
    queryFn: () => get<ConstructionWorkspace>(`/projects/${projectId}/construction-workspace`),
    enabled: Boolean(projectId),
    retry: false,
  });

  useEffect(() => {
    if (!dialogRequest || dialogRequest.mode !== mode) return;
    if (dialogRequest.mode === 'checks') {
      setActionDialog(null);
      setCheckDialog(dialogRequest.state);
    } else {
      setCheckDialog(null);
      setActionDialog(dialogRequest.state);
    }
    onDialogRequestHandled?.();
  }, [dialogRequest, mode, onDialogRequestHandled]);

  async function refresh() {
    setCheckDialog(null);
    setActionDialog(null);
    await queryClient.invalidateQueries({ queryKey: ['construction-workspace', projectId] });
  }

  async function approveImpact(action: ConstructionActionItem) {
    try {
      await post(`/construction-actions/${action.id}/approve-impact`);
      await queryClient.invalidateQueries({ queryKey: ['construction-workspace', projectId] });
      setActionDialog(null);
      toast.success('Đã xác nhận ảnh hưởng cần cập nhật');
    } catch (error) {
      toast.error('Không xác nhận được ảnh hưởng', error);
    }
  }

  function viewCheck(check: ConstructionCheck) {
    setActionDialog(null);
    setCheckDialog({ type: 'view', check });
  }

  function viewAction(action: ConstructionActionItem) {
    setCheckDialog(null);
    setActionDialog({ type: 'view', action });
  }

  function viewReport(reportId: string) {
    setCheckDialog(null);
    setActionDialog(null);
    onViewReport?.(reportId);
  }

  if (workspace.isLoading) return <LoadingSpinner label={mode === 'checks' ? 'Đang tải danh sách kiểm tra...' : 'Đang tải việc cần xử lý...'} />;
  if (workspace.isError || !workspace.data) return <div className="flex items-center justify-between rounded-lg border border-rose-200 bg-rose-50 p-4 text-sm text-rose-700"><span>Không tải được dữ liệu thi công.</span><button type="button" className="font-semibold underline" onClick={() => workspace.refetch()}>Thử lại</button></div>;

  const pendingChecks = workspace.data.checks.filter((row) => row.status === 'pending').length;
  const activeActions = workspace.data.actions.filter((row) => row.status === 'open' || row.status === 'waiting_confirmation').length;
  const constructionActive = workspace.data.runtime.construction_active;
  const variationSourceWorkItem = workspace.data.work_items.find((item) => item.report_eligible);
  const canProposeFromActions = Boolean(mode === 'actions' && constructionActive && permissions.proposeVariation && variationContext?.available && variationContext.can_propose && variationSourceWorkItem && onProposeVariation);

  return <>
    <section className="overflow-hidden rounded-lg border border-line bg-white">
      {!constructionActive ? <div className="border-b border-blue-200 bg-blue-50 px-4 py-3 text-xs font-medium text-blue-800">Quy trình chưa tới Node Thi công. Dữ liệu cũ vẫn xem được; thao tác tạo mới đang khóa.</div> : null}
      <div className="flex flex-col gap-3 border-b border-line px-4 py-4 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <div className="flex items-center gap-2">
            {mode === 'checks' ? <ClipboardCheck className="h-5 w-5 text-blue-600" /> : <Wrench className="h-5 w-5 text-amber-600" />}
            <h2 className="font-semibold text-ink">{mode === 'checks' ? 'Kiểm tra' : 'Cần xử lý'}</h2>
            <span className="rounded-full bg-slate-100 px-2 py-0.5 text-xs font-semibold text-slate-600">{mode === 'checks' ? pendingChecks : activeActions}</span>
          </div>
          <p className="mt-1 text-sm text-slate-500">{mode === 'checks' ? 'Lập lịch, ghi minh chứng và kết luận chất lượng tại hiện trường.' : 'Giao việc, theo dõi hạn và xác nhận kết quả khắc phục.'}</p>
        </div>
        <div className="flex flex-wrap gap-2">
          {mode === 'checks' && permissions.manageTemplates ? <button type="button" className="inline-flex items-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-50" onClick={() => setCheckDialog({ type: 'template' })}><Settings2 className="h-4 w-4" /> Mẫu kiểm tra</button> : null}
          {canProposeFromActions ? <button type="button" className="inline-flex items-center gap-2 rounded-md border border-amber-300 bg-amber-50 px-3 py-2 text-sm font-semibold text-amber-800 hover:bg-amber-100" onClick={() => onProposeVariation?.({ workItemId: variationSourceWorkItem!.id })}><GitPullRequestArrow className="h-4 w-4" /> Đề xuất phát sinh</button> : null}
          {mode === 'checks' && permissions.requestCheck ? <button type="button" disabled={!constructionActive} className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-50" onClick={() => constructionActive && setCheckDialog({ type: 'create' })}><Plus className="h-4 w-4" /> Tạo kiểm tra</button> : null}
          {mode === 'actions' && permissions.createAction ? <button type="button" disabled={!constructionActive} className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-50" onClick={() => constructionActive && setActionDialog({ type: 'create' })}><Plus className="h-4 w-4" /> Tạo việc</button> : null}
        </div>
      </div>
      <div className="p-4">
        {mode === 'checks' ? <ConstructionCheckList workspace={workspace.data} canCreate={permissions.requestCheck && constructionActive} canEdit={permissions.editCheck} canPerform={permissions.performCheck} onCreate={() => setCheckDialog({ type: 'create' })} onView={viewCheck} onEdit={(check) => setCheckDialog({ type: 'edit', check })} onComplete={(check) => setCheckDialog({ type: 'complete', check })} /> : <ConstructionActionList projectId={projectId} workspace={workspace.data} canCreate={permissions.createAction && constructionActive} canEdit={permissions.editAction} canResolve={permissions.resolveAction} canVerify={permissions.verifyAction} onCreate={() => setActionDialog({ type: 'create' })} onView={viewAction} onEdit={(action) => setActionDialog({ type: 'edit', action })} onResolve={(action) => setActionDialog({ type: 'resolve', action })} onVerify={(action) => setActionDialog({ type: 'verify', action })} />}
      </div>
    </section>
    <ConstructionCheckDialogs state={checkDialog} projectId={projectId} workspace={workspace.data} canEdit={permissions.editCheck} canPerform={permissions.performCheck} onClose={() => setCheckDialog(null)} onSaved={refresh} onEdit={(check) => setCheckDialog({ type: 'edit', check })} onComplete={(check) => setCheckDialog({ type: 'complete', check })} onViewAction={viewAction} onViewReport={onViewReport ? viewReport : undefined} />
    <ConstructionActionDialogs state={actionDialog} projectId={projectId} workspace={workspace.data} canEdit={permissions.editAction} canResolve={permissions.resolveAction} canVerify={permissions.verifyAction} canApproveImpact={permissions.approveImpact} canProposeVariation={permissions.proposeVariation} variationContext={variationContext} variationContextLoading={variationContextLoading} variationContextError={variationContextError} canViewContracts={canViewContracts} onRetryVariationContext={onRetryVariationContext} onClose={() => setActionDialog(null)} onSaved={refresh} onEdit={(action) => setActionDialog({ type: 'edit', action })} onResolve={(action) => setActionDialog({ type: 'resolve', action })} onVerify={(action) => setActionDialog({ type: 'verify', action })} onApproveImpact={approveImpact} onProposeVariation={(action) => { setActionDialog(null); onProposeVariation?.({ workItemId: action.work_item_id, action }); }} onViewCheck={viewCheck} onViewReport={onViewReport ? viewReport : undefined} />
  </>;
}
