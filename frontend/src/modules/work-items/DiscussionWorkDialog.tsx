import { useEffect, useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Link } from 'react-router-dom';
import { Dialog } from '../../components/Dialog';
import { StatusBadge } from '../../components/StatusBadge';
import { get } from '../../lib/api/client';
import { useAuth } from '../../lib/auth/AuthContext';
import type { EvidenceRecord, WorkItem } from '../../lib/types/domain';
import { WorkflowFormPanel } from './WorkflowFormPanel';
import { EvidencePanel } from './EvidencePanel';
import { DesignReleasePanel } from './DesignReleasePanel';
import { WorkItemDetailWorkspace } from './WorkItemDetailDialog';
import { discussionWorkReadOnly, discussionWorkSurface } from './discussionWorkDomain';
import { executionTargetPath, executionTargetWorkspaceLabel } from './workItemExecutionTarget';
import { ProcurementDialog } from '../procurement/ProcurementDialog';

export function DiscussionWorkDialog({ projectId, workItemId, onClose }: { projectId: string; workItemId: string; onClose: () => void }) {
  const { user } = useAuth();
  const client = useQueryClient();
  const [showDetails, setShowDetails] = useState(false);
  const [dirty, setDirty] = useState(false);
  const [evidenceDirty, setEvidenceDirty] = useState(false);
  const [busy, setBusy] = useState(false);
  const [evidenceBusy, setEvidenceBusy] = useState(false);
  const [discardOpen, setDiscardOpen] = useState(false);
  const detail = useQuery({ queryKey: ['work-item', workItemId], queryFn: () => get<WorkItem>(`/work-items/${workItemId}`), enabled: Boolean(user?.id), retry: false, refetchOnWindowFocus: false });
  // Retain mounted draft state after a failed refresh, but hide cached content
  // until access has been successfully rechecked.
  const item = detail.isFetchedAfterMount && detail.data?.project_id === projectId ? detail.data : undefined;
  const surface = item ? discussionWorkSurface(item) : 'external';
  const readOnly = detail.isError || !item || discussionWorkReadOnly(item);
  const changed = () => {
    for (const key of [['work-item', workItemId], ['project-operations', projectId], ['project-activity', projectId], ['utility-project-activity', projectId], ['my-work-items'], ['project-discussions']]) void client.invalidateQueries({ queryKey: key });
  };
  const hasChanges = dirty || evidenceDirty;
  const saving = busy || evidenceBusy;
  const close = () => { if (saving) return; if (hasChanges) setDiscardOpen(true); else onClose(); };
  useEffect(() => {
    if (!hasChanges && !saving) return;
    const preventUnload = (event: BeforeUnloadEvent) => { event.preventDefault(); event.returnValue = ''; };
    window.addEventListener('beforeunload', preventUnload);
    return () => window.removeEventListener('beforeunload', preventUnload);
  }, [hasChanges, saving]);
  if (item && surface === 'procurement' && !showDetails) return <ProcurementDialog projectId={projectId} workItem={item} readOnly={readOnly} onClose={onClose} onShowWorkDetails={() => setShowDetails(true)} />;
  return <><Dialog open manageFocus mobileFullscreen size={showDetails ? 'workspace' : 'xl'} title={(!detail.isError && item?.title) || 'Cập nhật công việc'} description="Cập nhật hồ sơ ngay tại Trao đổi" onClose={close}>
    {!detail.isFetchedAfterMount ? <p role="status">Đang tải công việc…</p> : null}
    {detail.isError || (detail.data && !item) ? <p role="alert" className="text-sm text-rose-700">Không thể mở công việc trong công trình này. <button className="underline" onClick={() => void detail.refetch()}>Thử lại</button></p> : null}
    {item ? <div hidden={detail.isError} className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-3"><StatusBadge status={item.status} />
        <button disabled={hasChanges || saving} className="text-sm font-semibold text-blue-700 underline disabled:opacity-40" onClick={() => setShowDetails(value => !value)}>{showDetails ? 'Quay lại cập nhật' : 'Điều kiện & chốt công việc'}</button>
      </div>
      {readOnly ? <p className="rounded-md bg-amber-50 p-3 text-sm text-amber-900">Chỉ xem tại trạng thái hoặc quyền hiện tại. Kiểm tra điều kiện công việc để biết bước tiếp theo.</p> : null}
      {showDetails ? <WorkItemDetailWorkspace item={item} onChanged={changed} /> : surface === 'design' ? <DesignReleasePanel projectId={projectId} workItems={[item]} readOnly={readOnly} onDirtyChange={setDirty} onBusyChange={setBusy} />
        : surface === 'form' ? <><WorkflowFormPanel workItemId={item.id} projectId={projectId} evidence={(item.evidence ?? []) as EvidenceRecord[]} workItemStatus={item.status} readOnly={readOnly} onChanged={changed} onDirtyChange={setDirty} onBusyChange={setBusy} /><EvidencePanel workItemId={item.id} projectId={projectId} evidence={(item.evidence ?? []) as EvidenceRecord[]} readOnly={readOnly} onDirtyChange={setEvidenceDirty} onBusyChange={setEvidenceBusy} /></>
        : surface === 'evidence' ? <EvidencePanel workItemId={item.id} projectId={projectId} evidence={(item.evidence ?? []) as EvidenceRecord[]} readOnly={readOnly} onDirtyChange={setDirty} onBusyChange={setBusy} />
        : <div className="space-y-3"><p className="text-sm text-slate-600">{item.execution_target?.source_message || 'Thực hiện thao tác này tại không gian nghiệp vụ của công việc.'}</p><Link className="inline-block rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white" to={executionTargetPath(item, `/projects/${projectId}?tab=activity`)}>Mở tại {executionTargetWorkspaceLabel(item)}</Link></div>}
    </div> : null}
  </Dialog><Dialog open={discardOpen} manageFocus title="Bỏ thay đổi chưa lưu?" size="sm" onClose={() => setDiscardOpen(false)} footer={<div className="flex justify-end gap-2"><button className="rounded border px-3 py-2" onClick={() => setDiscardOpen(false)}>Tiếp tục nhập</button><button className="rounded bg-rose-600 px-3 py-2 text-white" onClick={onClose}>Bỏ thay đổi</button></div>}><p>Nội dung chưa lưu sẽ mất khi đóng cửa sổ cập nhật.</p></Dialog></>;
}
