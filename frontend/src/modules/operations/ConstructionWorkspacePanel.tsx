import { useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { ClipboardCheck, Plus, Settings2, Wrench } from 'lucide-react';
import { LoadingSpinner } from '../../components/LoadingSpinner';
import { useToast } from '../../components/ToastProvider';
import { get, post } from '../../lib/api/client';
import { useAuth } from '../../lib/auth/AuthContext';
import type { ConstructionActionItem, ConstructionCheck, ConstructionWorkspace } from '../../lib/types/domain';
import { ConstructionActionDialogs, type ActionDialogState } from './ConstructionActionDialogs';
import { ConstructionCheckDialogs, type CheckDialogState } from './ConstructionCheckDialogs';
import { ConstructionActionList, ConstructionCheckList } from './ConstructionWorkspaceLists';

export type SimplifiedConstructionMode = 'checks' | 'actions';

export function ConstructionWorkspacePanel({ projectId, mode }: { projectId: string; mode: SimplifiedConstructionMode }) {
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
  };
  const workspace = useQuery({
    queryKey: ['construction-workspace', projectId],
    queryFn: () => get<ConstructionWorkspace>(`/projects/${projectId}/construction-workspace`),
    enabled: Boolean(projectId),
    retry: false,
  });

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

  if (workspace.isLoading) return <LoadingSpinner label={mode === 'checks' ? 'Đang tải danh sách kiểm tra...' : 'Đang tải việc cần xử lý...'} />;
  if (workspace.isError || !workspace.data) return <div className="flex items-center justify-between rounded-lg border border-rose-200 bg-rose-50 p-4 text-sm text-rose-700"><span>Không tải được dữ liệu thi công.</span><button type="button" className="font-semibold underline" onClick={() => workspace.refetch()}>Thử lại</button></div>;

  const pendingChecks = workspace.data.checks.filter((row) => row.status === 'pending').length;
  const activeActions = workspace.data.actions.filter((row) => row.status === 'open' || row.status === 'waiting_confirmation').length;

  return <>
    <section className="overflow-hidden rounded-lg border border-line bg-white">
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
          {mode === 'checks' && permissions.requestCheck ? <button type="button" className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700" onClick={() => setCheckDialog({ type: 'create' })}><Plus className="h-4 w-4" /> Tạo kiểm tra</button> : null}
          {mode === 'actions' && permissions.createAction ? <button type="button" className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700" onClick={() => setActionDialog({ type: 'create' })}><Plus className="h-4 w-4" /> Tạo việc</button> : null}
        </div>
      </div>
      <div className="p-4">
        {mode === 'checks' ? <ConstructionCheckList workspace={workspace.data} canCreate={permissions.requestCheck} canEdit={permissions.editCheck} canPerform={permissions.performCheck} onCreate={() => setCheckDialog({ type: 'create' })} onView={viewCheck} onEdit={(check) => setCheckDialog({ type: 'edit', check })} onComplete={(check) => setCheckDialog({ type: 'complete', check })} /> : <ConstructionActionList workspace={workspace.data} canCreate={permissions.createAction} canEdit={permissions.editAction} canResolve={permissions.resolveAction} canVerify={permissions.verifyAction} onCreate={() => setActionDialog({ type: 'create' })} onView={viewAction} onEdit={(action) => setActionDialog({ type: 'edit', action })} onResolve={(action) => setActionDialog({ type: 'resolve', action })} onVerify={(action) => setActionDialog({ type: 'verify', action })} />}
      </div>
    </section>
    <ConstructionCheckDialogs state={checkDialog} projectId={projectId} workspace={workspace.data} canEdit={permissions.editCheck} canPerform={permissions.performCheck} onClose={() => setCheckDialog(null)} onSaved={refresh} onEdit={(check) => setCheckDialog({ type: 'edit', check })} onComplete={(check) => setCheckDialog({ type: 'complete', check })} onViewAction={viewAction} />
    <ConstructionActionDialogs state={actionDialog} projectId={projectId} workspace={workspace.data} canEdit={permissions.editAction} canResolve={permissions.resolveAction} canVerify={permissions.verifyAction} canApproveImpact={permissions.approveImpact} onClose={() => setActionDialog(null)} onSaved={refresh} onEdit={(action) => setActionDialog({ type: 'edit', action })} onResolve={(action) => setActionDialog({ type: 'resolve', action })} onVerify={(action) => setActionDialog({ type: 'verify', action })} onApproveImpact={approveImpact} onViewCheck={viewCheck} />
  </>;
}
