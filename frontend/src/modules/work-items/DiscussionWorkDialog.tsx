import { useEffect, useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Dialog } from '../../components/Dialog';
import { get } from '../../lib/api/client';
import { useAuth } from '../../lib/auth/AuthContext';
import type { WorkItem } from '../../lib/types/domain';
import { WorkItemDetailWorkspace } from './WorkItemDetailDialog';
import { discussionWorkDetailLoadState, discussionWorkReadOnly, discussionWorkSurface } from './discussionWorkDomain';
import { ProcurementDialog } from '../procurement/ProcurementDialog';

export function DiscussionWorkDialog({ projectId, workItemId, onClose }: { projectId: string; workItemId: string; onClose: () => void }) {
  const { user } = useAuth();
  const client = useQueryClient();
  const [showProcurementWorkDetails, setShowProcurementWorkDetails] = useState(false);
  const [dirty, setDirty] = useState(false);
  const [busy, setBusy] = useState(false);
  const [discardOpen, setDiscardOpen] = useState(false);
  const detail = useQuery({ queryKey: ['work-item', workItemId], queryFn: () => get<WorkItem>(`/work-items/${workItemId}`), enabled: Boolean(user?.id), retry: false, refetchOnMount: 'always', refetchOnWindowFocus: false });
  // Retain mounted draft state after a failed refresh, but hide cached content
  // until access has been successfully rechecked.
  const detailLoadState = discussionWorkDetailLoadState({
    isFetchedAfterMount: detail.isFetchedAfterMount,
    isFetching: detail.isFetching,
    isError: detail.isError,
    projectId: detail.data?.project_id,
  }, projectId);
  const item = detailLoadState === 'ready' ? detail.data : undefined;
  const surface = item ? discussionWorkSurface(item) : 'external';
  const readOnly = detail.isError || !item || discussionWorkReadOnly(item);

  const changed = () => {
    for (const key of [['work-item', workItemId], ['project-operations', projectId], ['project-activity', projectId], ['utility-project-activity', projectId], ['my-work-items'], ['project-discussions']]) void client.invalidateQueries({ queryKey: key });
  };
  const close = () => {
    if (busy) return;
    if (dirty) setDiscardOpen(true);
    else onClose();
  };

  useEffect(() => {
    if (!dirty && !busy) return;
    const preventUnload = (event: BeforeUnloadEvent) => { event.preventDefault(); event.returnValue = ''; };
    window.addEventListener('beforeunload', preventUnload);
    return () => window.removeEventListener('beforeunload', preventUnload);
  }, [dirty, busy]);

  useEffect(() => {
    setShowProcurementWorkDetails(false);
    setDirty(false);
    setBusy(false);
    setDiscardOpen(false);
  }, [workItemId]);

  if (item && surface === 'procurement' && !showProcurementWorkDetails) {
    return (
      <ProcurementDialog
        projectId={projectId}
        workItem={item}
        readOnly={readOnly}
        onClose={onClose}
        onShowWorkDetails={() => setShowProcurementWorkDetails(true)}
      />
    );
  }

  return (
    <>
      <Dialog
        open
        manageFocus
        mobileFullscreen
        size="workspace"
        title={(!detail.isError && item?.title) || 'Cập nhật công việc'}
        description="Cập nhật hồ sơ ngay tại Trao đổi"
        onClose={close}
      >
        {detailLoadState === 'loading' ? <p role="status">Đang tải công việc…</p> : null}
        {detailLoadState === 'error' ? <p role="alert" className="text-sm text-rose-700">Không thể mở công việc trong công trình này. <button type="button" className="underline" onClick={() => void detail.refetch()}>Thử lại</button></p> : null}
        {item ? (
          <div className="space-y-4">
            {surface === 'procurement' ? (
              <div className="flex justify-end">
                <button
                  type="button"
                  disabled={dirty || busy}
                  className="text-sm font-semibold text-blue-700 underline disabled:opacity-40"
                  onClick={() => setShowProcurementWorkDetails(false)}
                >
                  Quay lại yêu cầu mua
                </button>
              </div>
            ) : null}
            <WorkItemDetailWorkspace
              item={item}
              onClose={close}
              onChanged={changed}
              onDirtyChange={setDirty}
              onBusyChange={setBusy}
            />
          </div>
        ) : null}
      </Dialog>
      <Dialog
        open={discardOpen}
        manageFocus
        title="Bỏ thay đổi chưa lưu?"
        size="sm"
        onClose={() => setDiscardOpen(false)}
        footer={(
          <div className="flex justify-end gap-2">
            <button className="rounded border px-3 py-2" onClick={() => setDiscardOpen(false)}>Tiếp tục nhập</button>
            <button className="rounded bg-rose-600 px-3 py-2 text-white" onClick={onClose}>Bỏ thay đổi</button>
          </div>
        )}
      >
        <p>Nội dung chưa lưu sẽ mất khi đóng cửa sổ cập nhật.</p>
      </Dialog>
    </>
  );
}
