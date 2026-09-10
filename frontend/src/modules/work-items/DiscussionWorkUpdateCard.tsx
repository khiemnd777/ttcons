import { lazy, Suspense, useState } from 'react';
import { createPortal } from 'react-dom';
import { useQuery } from '@tanstack/react-query';
import { Dialog } from '../../components/Dialog';
import { useAuth } from '../../lib/auth/AuthContext';
import { get } from '../../lib/api/client';
import type { AnyRecord, EvidenceRecord, WorkItemActivity, WorkflowFormField } from '../../lib/types/domain';
import { EvidenceReferenceViewButton } from '../_shared/EvidenceReferenceViewer';
import { DocumentReferenceViewButton, type DocumentFileReference } from '../_shared/DocumentReferenceViewer';
import { procurementStatusLabels } from '../procurement/procurementWorkspace';
import type { ProcurementUpdateSnapshot } from '../procurement/ProcurementSnapshot';

const WorkflowFormSnapshot = lazy(() => import('./WorkflowFormPanel').then(module => ({ default: module.WorkflowFormSnapshot })));
const ProcurementSnapshot = lazy(() => import('../procurement/ProcurementSnapshot').then(module => ({ default: module.ProcurementSnapshot })));
const ProcurementDialog = lazy(() => import('../procurement/ProcurementDialog').then(module => ({ default: module.ProcurementDialog })));

type WorkUpdate = {
  activity_id: string;
  source_type: 'workflow_form' | 'design_release' | 'evidence' | 'procurement_request';
  source_id: string; project_id: string;
  snapshot: ProcurementUpdateSnapshot & {
    title?: string; revision_label?: string; status?: string; form_code?: string;
    fields?: WorkflowFormField[]; answers?: AnyRecord; evidence_id?: string; caption?: string;
    change_reason?: string; notes?: string;
    files?: Array<DocumentFileReference & Partial<EvidenceRecord> & { evidence_id?: string; reference_type?: 'evidence' | 'document' | string }>;
  };
};

export function DiscussionWorkUpdateCard({ item }: { item: WorkItemActivity }) {
  const [open, setOpen] = useState(false);
  const [currentOpen, setCurrentOpen] = useState(false);
  const { user } = useAuth();
  const source = item.metadata_json?.work_update;
  const designUploadBatch = source?.source_type === 'design_release' && String(source.event_key ?? '').startsWith('files:');
  const detail = useQuery({
    queryKey: ['discussion-work-update', user?.id, item.id],
    queryFn: () => get<WorkUpdate>(`/activities/${item.id}/work-update`),
    enabled: (open || designUploadBatch) && Boolean(user?.id), retry: false, staleTime: 0, gcTime: 0,
  });
  if (item.activity_type !== 'work_update') return null;
  const snapshot = !detail.isFetching && !detail.isError ? detail.data?.snapshot : undefined;
  const label = source?.source_type === 'procurement_request' ? 'Yêu cầu mua' : source?.source_type === 'design_release' ? 'Hồ sơ thiết kế' : source?.source_type === 'evidence' ? 'Minh chứng' : 'Hồ sơ công việc';
  const statusLabel = source?.source_type === 'procurement_request' ? procurementStatusLabels[source.status] ?? 'Cập nhật yêu cầu mua' : source?.status === 'draft' ? 'Nháp · chưa phát hành' : source?.status === 'released' ? 'Đã phát hành' : source?.source_type === 'evidence' ? 'Đã tải lên · chưa duyệt' : 'Đã chốt hồ sơ';
  return <>
    <button type="button" className="my-2 block w-full min-w-0 rounded-lg border border-blue-200 bg-blue-50 p-3 text-left text-slate-800 hover:bg-blue-100" onClick={() => setOpen(true)}>
      <span className="block text-xs font-semibold text-blue-700">{label}{source?.revision_label ? ` · ${source.revision_label}` : ''}</span>
      <span className="mt-1 block break-words text-sm font-semibold">{item.work_item_title || item.target_label || label}</span>
      <span className="mt-1 block text-xs text-slate-600">{statusLabel}</span>
      <span className="mt-2 block text-xs font-semibold text-blue-700">Xem bản cập nhật</span>
    </button>
    {designUploadBatch ? <div className="mt-2 space-y-2">
      <p className="text-sm font-semibold">Đã thêm tệp thiết kế:</p>
      {detail.isFetching ? <p role="status" className="text-xs text-slate-500">Đang tải danh sách tệp…</p> : null}
      {detail.isError ? <p role="alert" className="text-xs text-rose-700">Không tải được danh sách tệp. <button type="button" className="font-semibold underline" onClick={() => void detail.refetch()}>Thử lại</button></p> : null}
      {snapshot?.files?.map((file, index) => <div key={file.evidence_id || file.id} className="flex min-w-0 flex-col gap-2 rounded-md border border-white/50 bg-white/85 px-3 py-2 sm:flex-row sm:items-center sm:justify-between">
        <span className="min-w-0 break-words text-sm text-slate-700">{index + 1}. {file.name}</span>
        <EvidenceReferenceViewButton reference={{ id: file.evidence_id || file.id, name: file.name }} label="Xem hồ sơ" />
      </div>)}
    </div> : null}
    {open ? createPortal(<div className="relative z-[100]"><Dialog open manageFocus mobileFullscreen size="xl" title={snapshot?.title || label} description={`Bản tại thời điểm cập nhật${snapshot?.revision_label ? ` · ${snapshot.revision_label}` : ''}`} onClose={() => { setOpen(false); setCurrentOpen(false); }}>
      {detail.isFetching ? <p role="status">Đang tải bản cập nhật…</p> : null}
      {detail.isError ? <p role="alert" className="text-sm text-rose-700">Không thể xem bản cập nhật. Quyền truy cập có thể đã thay đổi. <button className="underline" onClick={() => void detail.refetch()}>Thử lại</button></p> : null}
      {snapshot ? <div className="space-y-4">
        <p className="text-sm text-slate-600">{item.comment}</p>
        {detail.data?.source_type === 'workflow_form' ? <Suspense fallback={<p>Đang tải biểu mẫu…</p>}><WorkflowFormSnapshot fields={snapshot.fields ?? []} answers={snapshot.answers ?? {}} templateCode={snapshot.form_code} /></Suspense> : null}
        {detail.data?.source_type === 'procurement_request' ? <Suspense fallback={<p>Đang tải yêu cầu mua…</p>}><ProcurementSnapshot snapshot={snapshot} /><button className="mt-4 rounded bg-blue-600 px-3 py-2 text-sm font-semibold text-white" onClick={() => setCurrentOpen(true)}>Mở yêu cầu hiện tại</button></Suspense> : null}
        {snapshot.evidence_id ? <EvidenceReferenceViewButton reference={{ id: snapshot.evidence_id, name: snapshot.title }} label="Xem tệp minh chứng" /> : null}
        {snapshot.caption ? <p className="whitespace-pre-wrap text-sm">{snapshot.caption}</p> : null}
        {snapshot.change_reason ? <p className="whitespace-pre-wrap text-sm"><strong>Lý do cập nhật:</strong> {snapshot.change_reason}</p> : null}
        {snapshot.notes ? <p className="whitespace-pre-wrap text-sm">{snapshot.notes}</p> : null}
        {snapshot.files?.map(file => file.reference_type === 'evidence' || file.evidence_id
          ? <div key={file.evidence_id || file.id} className="rounded border border-line p-3"><span className="mr-3 text-sm text-slate-700">{file.name}</span><EvidenceReferenceViewButton reference={{ id: file.evidence_id || file.id, name: file.name }} label="Xem hồ sơ" /></div>
          : <DocumentReferenceViewButton key={file.id} file={file} label={file.name} sourceLabel={`Hồ sơ thiết kế${snapshot.revision_label ? ` · ${snapshot.revision_label}` : ''} · Bản tại thời điểm cập nhật`} className="block max-w-full break-words rounded border border-line p-3 text-left text-sm text-blue-700" />)}
      </div> : null}
    </Dialog>{currentOpen && detail.data?.source_type === 'procurement_request' ? <Suspense fallback={<Dialog open title="Yêu cầu mua" onClose={() => setCurrentOpen(false)}><p>Đang tải yêu cầu mua…</p></Dialog>}><ProcurementDialog projectId={detail.data.project_id} requestId={detail.data.source_id} onClose={() => setCurrentOpen(false)} /></Suspense> : null}</div>, document.body) : null}
  </>;
}
