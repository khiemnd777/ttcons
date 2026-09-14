import { type ReactNode, useEffect, useMemo, useRef, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useParams, useSearchParams } from 'react-router-dom';
import { Loader2, Plus, RefreshCw, Send } from 'lucide-react';
import { DataTable } from '../../components/DataTable';
import { Dialog } from '../../components/Dialog';
import { FilterBar, type FilterOption } from '../../components/FilterBar';
import { ListSection } from '../../components/ListSection';
import { PageHeader } from '../../components/PageHeader';
import { ProjectBackLink } from '../../components/ProjectBackLink';
import { RowActions } from '../../components/RowActions';
import { StatusBadge } from '../../components/StatusBadge';
import { useToast } from '../../components/ToastProvider';
import { get, post, put } from '../../lib/api/client';
import { useAuth } from '../../lib/auth/AuthContext';
import { dateText } from '../../lib/utils/format';
import { AsyncProjectPicker } from '../_shared/AsyncProjectPicker';
import { SimpleForm } from '../_shared/SimpleForm';
import { HandoverChecklistSourceDialog } from './HandoverChecklistSourceDialog';
import { HandoverCreateDialog } from './HandoverCreateDialog';
import { HandoverWorkspace } from './HandoverWorkspace';
import { useRecordDeepLink } from '../_shared/useRecordDeepLink';
import type { ChecklistItem, FinalAcceptanceOption, HandoverDetail, HandoverRow } from './handoverTypes';
import {
  syncWorkItemMutationTransition,
  withWorkItemMutationContext,
  workItemAutoStartedNotice,
  type WorkItemMutationResult,
} from '../work-items/workItemMutationContext';

type HandoverAction = { type: 'checklist'; item: ChecklistItem } | { type: 'source'; item: ChecklistItem } | { type: 'complete'; record: HandoverDetail } | null;

export function HandoverPage() {
  const { id } = useParams();
  const [searchParams] = useSearchParams();
  const taskId = searchParams.get('task')?.trim() ?? '';
  const [projectId, setProjectId] = useState(id ?? '');
  const [statusFilter, setStatusFilter] = useState('all');
  const [createOpen, setCreateOpen] = useState(false);
  const [selectedID, setSelectedID] = useState('');
  const sourceLink = useRecordDeepLink({ kind: 'handover', projectId: id, onOpen: record => { setProjectId(record.project_id); setSelectedID(record.id); } });
  const [action, setAction] = useState<HandoverAction>(null);
  const workspaceRef = useRef<HTMLElement>(null);
  const revealedSource = useRef('');
  const queryClient = useQueryClient();
  const toast = useToast();
  const { hasAnyPermission } = useAuth();
  const canCreate = hasAnyPermission(['handover.create']);
  const canVerify = hasAnyPermission(['handover.verify']);
  const canApprove = hasAnyPermission(['handover.approve']);
  const projects = useQuery({ queryKey: ['projects'], queryFn: () => get<Record<string, any>[]>('/projects') });
  useEffect(() => { if (!projectId && projects.data?.[0]) setProjectId(projects.data[0].id); }, [projects.data, projectId]);
  const handovers = useQuery({ queryKey: ['handover', projectId], queryFn: () => get<HandoverRow[]>(`/projects/${projectId}/handover`), enabled: Boolean(projectId) });
  const options = useQuery({ queryKey: ['handover-options', projectId], queryFn: () => get<{ final_acceptances: FinalAcceptanceOption[] }>(`/projects/${projectId}/handover/options`), enabled: Boolean(projectId) });
  const rows = handovers.data ?? [];
  useEffect(() => {
    if (sourceLink.active) return;
    if (!rows.length) { setSelectedID(''); return; }
    if (!selectedID || !rows.some((row) => row.id === selectedID)) setSelectedID(rows[0].id);
  }, [rows, selectedID, sourceLink.active]);
  const detail = useQuery({ queryKey: ['handover-detail', selectedID], queryFn: () => get<HandoverDetail>(`/handover/${selectedID}`), enabled: Boolean(selectedID) });
  useEffect(() => {
    if (!sourceLink.active) { revealedSource.current = ''; return; }
    if (!detail.data || detail.isFetching || detail.data.id !== sourceLink.data?.id || revealedSource.current === detail.data.id) return;
    const frame = window.requestAnimationFrame(() => {
      workspaceRef.current?.scrollIntoView({ block: 'start' });
      workspaceRef.current?.focus({ preventScroll: true });
      revealedSource.current = detail.data.id;
    });
    return () => window.cancelAnimationFrame(frame);
  }, [sourceLink.active, sourceLink.data?.id, detail.data, detail.isFetching]);
  const invalidate = async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: ['handover', projectId] }),
      queryClient.invalidateQueries({ queryKey: ['handover-detail', selectedID] }),
    ]);
  };
  const create = useMutation({
    mutationFn: (values: Record<string, any>) => post<HandoverDetail & WorkItemMutationResult>(`/projects/${projectId}/handover`, withWorkItemMutationContext(values, taskId)),
    onSuccess: async (row) => { setCreateOpen(false); setSelectedID(row.id); await invalidate(); syncWorkItemMutationTransition(queryClient, row); toast.success(`Đã tạo hồ sơ bàn giao.${workItemAutoStartedNotice(row)}`); },
    onError: (error) => toast.error('Không tạo được hồ sơ bàn giao', error),
  });
  const transition = useMutation({
    mutationFn: ({ endpoint, values }: { endpoint: string; values?: Record<string, any> }) => post<HandoverDetail & WorkItemMutationResult>(endpoint, withWorkItemMutationContext(values ?? {}, taskId)),
    onSuccess: async (row) => { if (row.id) setSelectedID(row.id); setAction(null); await invalidate(); syncWorkItemMutationTransition(queryClient, row); toast.success(`Đã cập nhật trạng thái bàn giao.${workItemAutoStartedNotice(row)}`); },
    onError: (error) => toast.error('Không cập nhật được hồ sơ bàn giao', error),
  });
  const checklist = useMutation({
    mutationFn: ({ itemID, values }: { itemID: string; values: Record<string, any> }) => put(`/handover-checklist-items/${itemID}`, values),
    onSuccess: async () => { setAction(null); await invalidate(); toast.success('Đã xác minh checklist bàn giao'); },
    onError: (error) => toast.error('Không cập nhật được checklist', error),
  });

  const filteredRows = statusFilter === 'all' ? rows : rows.filter((row) => row.status === statusFilter);
  const statusOptions = useMemo<FilterOption[]>(() => [
    { value: 'all', label: 'Tất cả', count: rows.length },
    { value: 'draft', label: 'Nháp', count: countStatus(rows, 'draft') },
    { value: 'submitted', label: 'Đã gửi', count: countStatus(rows, 'submitted') },
    { value: 'ready', label: 'Sẵn sàng', count: countStatus(rows, 'ready') },
    { value: 'blocked', label: 'Đang chặn', count: countStatus(rows, 'blocked') },
    { value: 'handed_over', label: 'Đã bàn giao', count: countStatus(rows, 'handed_over') },
  ], [rows]);
  const summary = useMemo(() => ({
    total: rows.length,
    pending: rows.filter((row) => ['draft', 'submitted', 'ready', 'blocked'].includes(row.status)).length,
    blocked: countStatus(rows, 'blocked'),
    completed: countStatus(rows, 'handed_over'),
  }), [rows]);
  const openHandoverDetail = (handoverID: string) => {
    setSelectedID(handoverID);
    window.requestAnimationFrame(() => {
      workspaceRef.current?.scrollIntoView({ behavior: 'smooth', block: 'start' });
      workspaceRef.current?.focus({ preventScroll: true });
    });
  };

  return <>
    {sourceLink.feedback}
    <PageHeader
      title="Bàn giao"
      description="Hoàn tất hồ sơ bàn giao bằng readiness thật; mỗi điểm chặn dẫn về đúng module nguồn để xử lý."
      actions={<>{id ? <ProjectBackLink projectId={projectId} /> : <div className="w-full sm:w-80"><AsyncProjectPicker value={projectId} onChange={(value) => { setProjectId(value); setSelectedID(''); }} endpoint="/projects/options" queryKey="project-options" emptyLabel="Chọn công trình" allowEmptySelection={false} /></div>}{canCreate ? <button disabled={!projectId} className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white disabled:opacity-50" onClick={() => setCreateOpen(true)}><Plus className="h-4 w-4" />Tạo hồ sơ</button> : null}</>}
    />

    <div className="mb-4 grid gap-3 sm:grid-cols-2 lg:grid-cols-4"><Metric label="Tổng hồ sơ" value={summary.total} /><Metric label="Đang xử lý" value={summary.pending} /><Metric label="Đang bị chặn" value={summary.blocked} tone={summary.blocked ? 'amber' : 'slate'} /><Metric label="Đã bàn giao" value={summary.completed} tone="green" /></div>

    <div className="mb-3"><FilterBar label="Lịch sử bàn giao" value={statusFilter} options={statusOptions} onChange={setStatusFilter} disabled={!projectId} /></div>
    <ListSection title="Danh sách hồ sơ" className="mb-6">
      <DataTable rows={filteredRows} loading={handovers.isLoading} error={handovers.error} onRetry={() => handovers.refetch()} searchPlaceholder="Tìm mã, đại diện, trạng thái..." pageSize={10} defaultSort={{ key: 'created_at', direction: 'desc' }} columns={[
        { key: 'handover_code', header: 'Mã hồ sơ', render: (row) => <div><div className="font-semibold text-ink">{row.handover_code}</div><div className="text-xs text-slate-500">{row.client_representative} ↔ {row.company_representative}</div></div> },
        { key: 'final_acceptance_id', header: 'Nghiệm thu cuối', render: (row) => row.final_acceptance_id ? 'Đã khóa nguồn' : <span className="text-red-600">Chưa liên kết</span> },
        { key: 'checklist', header: 'Checklist', render: (row) => <span className={Number(row.checklist_pending_required ?? 0) ? 'font-semibold text-amber-700' : 'font-semibold text-emerald-700'}>{Number(row.checklist_verified ?? 0)}/{Number(row.checklist_total ?? 0)}</span> },
        { key: 'handover_date', header: 'Ngày bàn giao', render: (row) => dateText(row.handover_date) },
        { key: 'status', header: 'Trạng thái', render: (row) => <StatusBadge status={row.status} /> },
        { key: 'action', header: 'Thao tác', render: (row) => <RowActions onView={() => openHandoverDetail(row.id)}>{canVerify && ['draft', 'blocked'].includes(row.status) ? <SmallButton label="Gửi" icon={<Send className="h-3.5 w-3.5" />} onClick={() => transition.mutate({ endpoint: `/handover/${row.id}/submit`, values: { comment: 'Gửi kiểm tra readiness' } })} /> : null}</RowActions> },
      ]} empty={projectId ? 'Chưa có hồ sơ bàn giao' : 'Chọn công trình để xem bàn giao'} />
    </ListSection>

    {rows.length > 1 ? <label className="mb-4 block max-w-xl"><span className="text-xs font-semibold uppercase text-slate-500">Hồ sơ đang xem</span><select className="mt-1 w-full rounded-md border border-line bg-white px-3 py-2 text-sm" value={selectedID} onChange={(event) => openHandoverDetail(event.target.value)}>{rows.map((row) => <option key={row.id} value={row.id}>{row.handover_code} · {handoverStatusLabel(row.status)}</option>)}</select></label> : null}

    <section ref={workspaceRef} tabIndex={-1} aria-label="Chi tiết hồ sơ bàn giao" className="mb-6 scroll-mt-20 focus:outline-none">
      {handovers.isLoading || detail.isLoading ? <Loading /> : handovers.error || detail.error ? <ErrorState onRetry={() => { void handovers.refetch(); void detail.refetch(); }} /> : detail.data ? <HandoverWorkspace
        detail={detail.data}
        canVerify={canVerify}
        canApprove={canApprove}
        pending={transition.isPending}
        onChecklist={(item) => setAction({ type: 'checklist', item })}
        onViewSource={(item) => setAction({ type: 'source', item })}
        onSubmit={() => transition.mutate({ endpoint: `/handover/${detail.data.id}/submit`, values: { comment: 'Gửi hồ sơ bàn giao' } })}
        onCheck={() => transition.mutate({ endpoint: `/handover/${detail.data.id}/check` })}
        onComplete={() => setAction({ type: 'complete', record: detail.data })}
      /> : <div className="rounded-lg border border-dashed border-line p-8 text-center text-sm text-slate-500">Chưa có hồ sơ bàn giao cho công trình này.</div>}
    </section>

    <HandoverCreateDialog
      open={createOpen}
      projectId={projectId}
      acceptances={options.data?.final_acceptances ?? []}
      loading={options.isLoading}
      error={options.error}
      onRetry={() => void options.refetch()}
      onClose={() => setCreateOpen(false)}
      onCreate={(values) => create.mutateAsync(values)}
    />

    <HandoverChecklistSourceDialog
      open={action?.type === 'source'}
      item={action?.type === 'source' ? action.item : null}
      detail={detail.data ?? null}
      canEdit={Boolean(canVerify && detail.data && !['handed_over', 'cancelled', 'deleted'].includes(detail.data.status))}
      onEdit={action?.type === 'source' ? () => setAction({ type: 'checklist', item: action.item }) : undefined}
      onClose={() => setAction(null)}
    />

    <Dialog open={action?.type === 'checklist'} title="Xác minh mục checklist" description={action?.type === 'checklist' ? action.item.title : undefined} onClose={() => setAction(null)} size="lg">
      {action?.type === 'checklist' && detail.data ? <SimpleForm submitLabel="Lưu xác minh" fields={[
        { name: 'status', label: 'Trạng thái', type: 'select', defaultValue: action.item.status === 'verified' ? 'verified' : 'pending', required: true, options: [{ value: 'pending', label: 'Chưa xác minh' }, { value: 'verified', label: 'Đã xác minh' }] },
        { name: 'evidence_id', label: 'Minh chứng đã chấp nhận', type: 'select', defaultValue: action.item.evidence_id ?? '', options: detail.data.evidence_options.map((item) => ({ value: item.id, label: `${item.name} · ${item.evidence_type}` })) },
        { name: 'document_id', label: 'Tài liệu hồ sơ', type: 'select', defaultValue: action.item.document_id ?? '', options: detail.data.document_options.map((item) => ({ value: item.id, label: `${item.name} · ${item.document_type || 'tài liệu'}` })) },
        { name: 'notes', label: 'Ghi chú xác minh', type: 'textarea', defaultValue: action.item.notes ?? '' },
      ]} onSubmit={(values) => checklist.mutateAsync({ itemID: action.item.id, values })} /> : null}
    </Dialog>

    <Dialog open={action?.type === 'complete'} title="Xác nhận hoàn tất bàn giao" description="Hệ thống kiểm tra lại toàn bộ readiness ngay trước khi khóa hồ sơ." onClose={() => setAction(null)} size="lg">
      {action?.type === 'complete' ? <SimpleForm submitLabel="Hoàn tất bàn giao" fields={[{ name: 'comment', label: 'Căn cứ/xác nhận hai bên', type: 'textarea', required: true }]} onSubmit={(values) => transition.mutateAsync({ endpoint: `/handover/${action.record.id}/complete`, values })} /> : null}
    </Dialog>
  </>;
}

function handoverStatusLabel(status: string) { return ({ draft: 'Nháp', submitted: 'Đã gửi', ready: 'Sẵn sàng', blocked: 'Đang chặn', handed_over: 'Đã bàn giao' } as Record<string, string>)[status] ?? status; }
function countStatus(rows: HandoverRow[], status: string) { return rows.filter((row) => row.status === status).length; }
function Metric({ label, value, tone = 'slate' }: { label: string; value: ReactNode; tone?: 'slate' | 'amber' | 'green' }) { const color = tone === 'amber' ? 'border-amber-200 bg-amber-50 text-amber-800' : tone === 'green' ? 'border-emerald-200 bg-emerald-50 text-emerald-800' : 'border-line bg-white text-ink'; return <div className={`rounded-md border px-3 py-2 ${color}`}><div className="text-xs font-semibold uppercase tracking-wide opacity-70">{label}</div><div className="mt-1 text-lg font-semibold">{value}</div></div>; }
function SmallButton({ label, icon, onClick }: { label: string; icon?: ReactNode; onClick: () => void }) { return <button type="button" className="inline-flex items-center gap-1.5 rounded-md border border-line bg-white px-2.5 py-1.5 text-xs font-semibold hover:bg-slate-100" onClick={onClick}>{icon}{label}</button>; }
function Loading() { return <div className="flex items-center gap-2 rounded-md border border-dashed border-line p-6 text-sm text-slate-500"><Loader2 className="h-4 w-4 animate-spin" />Đang tải hồ sơ bàn giao...</div>; }
function ErrorState({ onRetry }: { onRetry: () => void }) { return <div className="flex items-center justify-between rounded-md border border-red-200 bg-red-50 p-4 text-sm text-red-700"><span>Không tải được hồ sơ bàn giao.</span><button className="inline-flex items-center gap-1 font-semibold underline" onClick={onRetry}><RefreshCw className="h-3.5 w-3.5" />Thử lại</button></div>; }
