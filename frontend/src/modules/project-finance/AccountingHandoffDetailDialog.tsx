import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { ClipboardList, History } from 'lucide-react';
import { Dialog } from '../../components/Dialog';
import { RecordEmpty, RecordSection, RecordTable } from '../../components/RecordDetailLayout';
import { StatusBadge } from '../../components/StatusBadge';
import { WorkspaceTabs } from '../../components/WorkspaceTabs';
import { useToast } from '../../components/ToastProvider';
import { get, post } from '../../lib/api/client';
import { useAuth } from '../../lib/auth/AuthContext';
import type { AccountingHandoffDetail, AccountingHandoffSource, AccountingHandoffTarget } from '../../lib/types/domain';
import { dateText } from '../../lib/utils/format';
import { RecordLink } from '../_shared/RecordLinks';
import { EvidenceReferenceViewer } from '../_shared/EvidenceReferenceViewer';
import { ProtectedDocumentViewerDialog } from '../_shared/EvidenceRecordViewerDialog';
import { DocumentReferenceViewButton } from '../_shared/DocumentReferenceViewer';
import { evidenceReferenceMediaType } from '../_shared/evidenceReferenceMedia';
import { AccountingHandoffActionDialog, type AccountingHandoffAction, type AccountingHandoffDialog } from './AccountingHandoffActionDialog';
import { AccountingOperationsButton } from './AccountingOperationsDialog';
import { invalidateAccountingWork } from './accountingWorkActions';
import { syncWorkItemMutationTransition, withWorkItemMutationContext, type WorkItemMutationResult } from '../work-items/workItemMutationContext';

export function accountingHandoffDetailPath(target: AccountingHandoffTarget) {
  return `/projects/${encodeURIComponent(target.project_id)}/accounting-handoff-detail?${new URLSearchParams({ source_type: target.source_type, source_id: target.source_id })}`;
}

export function AccountingHandoffDetailDialog({ target, taskId = '', showReconciliation = true, onClose }: {
  target: AccountingHandoffTarget; taskId?: string; showReconciliation?: boolean; onClose: () => void;
}) {
  const { user } = useAuth();
  const queryClient = useQueryClient();
  const toast = useToast();
  const [tab, setTab] = useState<'current' | 'snapshot'>('current');
  const [actionDialog, setActionDialog] = useState<AccountingHandoffDialog>(null);
  const query = useQuery({
    queryKey: ['accounting-handoff-detail', user?.id, target.project_id, target.source_type, target.source_id],
    queryFn: () => get<AccountingHandoffDetail>(accountingHandoffDetailPath(target)),
    staleTime: 0, gcTime: 0, refetchOnMount: 'always', retry: false,
  });
  const mutation = useMutation({
    mutationFn: ({ action, values }: { action: AccountingHandoffAction; values: { note: string; returnReason: string; externalReference: string } }) => {
      const handoff = query.data?.handoff;
      const body = withWorkItemMutationContext({ source_type: target.source_type, source_id: target.source_id, note: values.note, return_reason: values.returnReason, external_reference: values.externalReference }, taskId);
      return post<WorkItemMutationResult>(handoff ? `/accounting-handoffs/${handoff.id}/${action}` : `/projects/${target.project_id}/accounting-handoffs`, body);
    },
    onSuccess: async (result, { action }) => {
      setActionDialog(null);
      syncWorkItemMutationTransition(queryClient, result);
      await invalidateAccountingWork(queryClient, target.project_id);
      toast.success(action === 'acknowledge' ? 'Đã xác nhận tiếp nhận hồ sơ.' : action === 'return' ? 'Đã trả hồ sơ để bổ sung.' : 'Đã bàn giao hồ sơ cho Kế toán.');
    },
    onError: async (error) => {
      setActionDialog(null);
      toast.error('Không cập nhật được hồ sơ kế toán', error);
      await invalidateAccountingWork(queryClient, target.project_id);
    },
  });
  const data = query.isFetchedAfterMount && !query.isError && !query.isFetching ? query.data : undefined;
  function openAction(action: AccountingHandoffAction) {
    if (!data?.capabilities[action] || data.handoff_access === false) return;
    const source = data.current ?? data.handoff?.source_snapshot_json;
    setActionDialog({ action, item: { handoff_id: data.handoff?.id, source_code: source?.source_code ?? '', source_title: source?.source_title ?? '', note: data.handoff?.note, external_reference: data.handoff?.external_reference } });
  }
  return <>
    <Dialog open title="Chi tiết hồ sơ kế toán" size="xl" mobileFullscreen manageFocus onClose={() => { if (!mutation.isPending && !actionDialog) onClose(); }}>
      {data && showReconciliation && <AccountingOperationsButton projectId={target.project_id} source={target}/>}
      <AccountingHandoffDetailContent data={data} tab={tab} onTab={setTab} loading={query.isFetching || (!query.isFetchedAfterMount && !query.isError)} error={query.isError} onRetry={() => void query.refetch()} onAction={openAction} />
    </Dialog>
    <AccountingHandoffActionDialog dialog={actionDialog} pending={mutation.isPending} onClose={() => { if (!mutation.isPending) setActionDialog(null); }} onSubmit={(values) => { if (actionDialog) mutation.mutate({ action: actionDialog.action, values }); }} />
  </>;
}

export function AccountingHandoffDetailContent({ data, loading, error, tab, onTab, onRetry, onAction }: {
  data?: AccountingHandoffDetail; loading: boolean; error: boolean; tab: 'current' | 'snapshot';
  onTab: (tab: 'current' | 'snapshot') => void; onRetry: () => void; onAction: (action: AccountingHandoffAction) => void;
}) {
  const [evidence, setEvidence] = useState<AccountingHandoffDetail['files'][number] | null>(null);
  if (error) return <div role="alert">Không tải được hồ sơ hoặc không có quyền truy cập. <button className="text-blue-700 underline" onClick={onRetry}>Thử lại</button></div>;
  if (loading || !data) return <p role="status">Đang tải hồ sơ…</p>;
  const canViewHandoff = data.handoff_access !== false;
  const handoff = canViewHandoff ? data.handoff : null;
  const snapshot = handoff?.source_snapshot_json;
  const title = data.current ?? snapshot;
  return <div className="space-y-4" data-component="accounting-handoff-detail">
    <div><p className="font-semibold">{title?.source_code} · {title?.source_title}</p><p className="text-sm text-slate-600">{data.project.code} · {data.project.name}</p>{canViewHandoff && <StatusBadge status={data.status} />}</div>
    <p className="rounded bg-blue-50 p-3 text-xs text-blue-900">Giá trị tham chiếu từ hồ sơ nguồn. MISA là nguồn công nợ và thu/chi chính thức; mã tham chiếu không xác nhận đã đồng bộ hoặc thanh toán.</p>
    {canViewHandoff && <WorkspaceTabs tabs={[{ id: 'current', label: 'Đối chiếu nguồn', icon: ClipboardList }, { id: 'snapshot', label: 'Thông tin đã bàn giao', icon: History }]} activeTab={tab} onChange={onTab} ariaLabel="Chi tiết hồ sơ kế toán" />}
    {canViewHandoff && tab === 'snapshot' ? <RecordSection title="Thông tin đã bàn giao" description="Bản chụp của lần bàn giao gần nhất được lưu trong hồ sơ; không bổ sung chứng từ hiện tại vào bản chụp.">
      {snapshot ? <SourceFields source={snapshot} snapshot /> : <RecordEmpty>Hồ sơ chưa được bàn giao.</RecordEmpty>}
    </RecordSection> : <>
      <RecordSection title="Nguồn hiện tại">
        {data.current ? <SourceFields source={data.current} /> : <RecordEmpty>{data.source_access ? canViewHandoff ? 'Nguồn không còn khả dụng. Thông tin đã bàn giao vẫn được giữ riêng.' : 'Nguồn không còn khả dụng.' : 'Không thể mở hồ sơ nguồn theo quyền hiện tại.'}</RecordEmpty>}
        {data.current?.requirements?.length ? <ul className="mt-3 space-y-1 text-sm">{data.current.requirements.map((requirement) => <li key={requirement.key}>{requirement.met ? '✓' : '○'} {requirement.label}{!requirement.met ? ` — ${requirement.guidance}` : ''}</li>)}</ul> : null}
      </RecordSection>
      {data.restricted ? <p className="rounded bg-amber-50 p-3 text-sm text-amber-900">Một số nguồn không thể hiển thị theo quyền hoặc trạng thái hiện tại.</p> : null}
      <RecordSection title="Hồ sơ có liên kết">
        <RecordTable rows={data.related} rowKey={(row) => `${row.kind}:${row.id}`} empty="Chưa có hồ sơ liên kết được phép xem." columns={[
          { key: 'record', label: 'Hồ sơ', render: (row) => <RecordLink target={{ kind: row.kind, id: row.id, projectId: data.project.id }}>{row.label}</RecordLink> },
          { key: 'status', label: 'Trạng thái', render: (row) => row.status ? <StatusBadge status={row.status} /> : '—' },
        ]} />
        {data.current && data.source_type === 'procurement_request' && !data.related.some((row) => row.kind === 'stock_document') ? <p className="mt-2 text-xs text-slate-500">Chưa có phiếu kho liên kết được phép xem; điều này không xác nhận vật tư chưa nhập kho.</p> : null}
      </RecordSection>
      <RecordSection title="Chứng từ có liên kết" description="Các tệp dưới đây thuộc nguồn hiện tại, không phải danh mục tệp đã chụp khi bàn giao.">
        {data.files.length ? <ul className="divide-y divide-line">{data.files.map((file) => <li key={`${file.kind}:${file.id}`} className="flex items-center justify-between gap-3 py-2 text-sm"><span className="min-w-0 break-words">{file.name}</span>{file.kind === 'document' ? <DocumentReferenceViewButton file={file} sourceLabel="Hồ sơ hợp đồng" label={fileActionLabel(file)} /> : <button className="shrink-0 rounded border border-line px-3 py-1 text-blue-700" onClick={() => setEvidence(file)}>{fileActionLabel(file)}</button>}</li>)}</ul> : <RecordEmpty>Chưa có tệp liên kết được phép xem.</RecordEmpty>}
      </RecordSection>
    </>}
    {handoff ? <RecordSection title="Thông tin bàn giao">
      <dl className="grid gap-2 text-sm sm:grid-cols-2"><div><dt>Người bàn giao</dt><dd>{handoff.handed_off_by_name || '—'} · {dateText(handoff.handed_off_at)}</dd></div>
        {handoff.acknowledged_at ? <div><dt>Tiếp nhận</dt><dd>{handoff.acknowledged_by_name || '—'} · {dateText(handoff.acknowledged_at)}</dd></div> : null}
        {handoff.returned_at ? <div><dt>Trả bổ sung</dt><dd>{handoff.returned_by_name || '—'} · {dateText(handoff.returned_at)}</dd></div> : null}
        <div><dt>Mã tham chiếu MISA do người dùng ghi nhận</dt><dd>{handoff.external_reference || 'Chưa ghi nhận'}</dd></div><div><dt>Ghi chú</dt><dd className="whitespace-pre-wrap">{handoff.note || '—'}</dd></div>
        {handoff.return_reason ? <div className="text-amber-800"><dt>Nội dung cần bổ sung</dt><dd>{handoff.return_reason}</dd></div> : null}</dl>
    </RecordSection> : null}
    {canViewHandoff && <div className="flex flex-wrap justify-end gap-2 border-t border-line pt-3">
      {data.capabilities.return ? <button className="rounded border border-line px-3 py-2" onClick={() => onAction('return')}>Trả để bổ sung</button> : null}
      {data.capabilities.acknowledge ? <button className="rounded bg-blue-600 px-3 py-2 font-semibold text-white" onClick={() => onAction('acknowledge')}>Xác nhận tiếp nhận hồ sơ</button> : null}
      {data.capabilities.handoff ? <button className="rounded bg-blue-600 px-3 py-2 font-semibold text-white" onClick={() => onAction('handoff')}>{handoff ? 'Bàn giao lại hồ sơ' : 'Bàn giao cho Kế toán'}</button> : null}
    </div>}
    {evidence?.kind === 'workflow_pdf' ? <ProtectedDocumentViewerDialog name={evidence.name} mimeType="application/pdf" protectedFilePath={`/workflow-form-pdf-exports/${encodeURIComponent(evidence.id)}/file`} sourceLabel="Bản PDF được hồ sơ nghiệm thu tham chiếu" onClose={() => setEvidence(null)} /> : <EvidenceReferenceViewer reference={evidence ? { id: evidence.id, name: evidence.name } : null} onClose={() => setEvidence(null)} />}
  </div>;
}

function SourceFields({ source, snapshot = false }: { source: AccountingHandoffSource; snapshot?: boolean }) {
  const amount = source.reference_amount == null ? 'Chưa có giá trị' : new Intl.NumberFormat('vi-VN', { style: 'currency', currency: source.currency || 'VND' }).format(source.reference_amount);
  return <dl className="grid gap-2 text-sm sm:grid-cols-2">
    <div><dt>Hồ sơ</dt><dd>{source.source_code || '—'} · {source.source_title || '—'}</dd></div>
    <div><dt>Đối tác</dt><dd>{source.counterparty_name || 'Chưa xác định đối tác'}</dd>{!snapshot && !source.partner_id ? <dd className="text-amber-700">Chưa liên kết danh mục</dd> : null}</div>
    <div><dt>Giá trị tham chiếu</dt><dd className="font-semibold">{amount}</dd></div>
    <div><dt>Trạng thái nguồn</dt><dd>{source.source_status ? <StatusBadge status={source.source_status} /> : 'Không có trong bản chụp'}</dd></div>
    {source.order_reference ? <div><dt>Số PO</dt><dd>{source.order_reference}</dd></div> : null}
  </dl>;
}

export function fileActionLabel(file: { name: string; mime_type?: string }) {
  const media = evidenceReferenceMediaType(file.name, file.mime_type);
  return media === 'photo' ? 'Xem ảnh' : media === 'pdf' ? 'Xem PDF' : media === 'word' ? 'Xem Word' : media === 'spreadsheet' ? 'Xem Excel' : 'Xem tệp';
}
