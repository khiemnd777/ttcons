import { useEffect, useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useParams } from 'react-router-dom';
import { CheckCircle2, FileCheck2, Loader2, Plus, RefreshCw, Send } from 'lucide-react';
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
import { RecordFormDialog } from '../_shared/RecordFormDialog';
import { SimpleForm } from '../_shared/SimpleForm';
import { ProjectSelector } from '../_shared/ProjectSelector';

type HandoverRow = {
  id: string;
  project_id: string;
  handover_code: string;
  status: string;
  final_acceptance_id?: string;
  final_acceptance_title?: string;
  handover_date?: string;
  client_representative: string;
  company_representative: string;
  checklist_total?: number;
  checklist_verified?: number;
  checklist_pending_required?: number;
  readiness_snapshot_json?: { ready?: boolean; blockers?: string[]; counts?: Record<string, number> };
  completed_at?: string;
};

type ChecklistItem = {
  id: string;
  item_code: string;
  category: string;
  title: string;
  required: boolean;
  status: string;
  source_type?: string;
  evidence_id?: string;
  evidence_name?: string;
  document_id?: string;
  document_name?: string;
  notes?: string;
};

type HandoverDetail = HandoverRow & {
  final_acceptance_status?: string;
  final_acceptance_result?: string;
  checklist_items: ChecklistItem[];
  evidence_options: { id: string; name: string; evidence_type: string }[];
  document_options: { id: string; name: string; document_type?: string }[];
};

type HandoverAction = { type: 'checklist'; item: ChecklistItem } | { type: 'complete'; record: HandoverDetail } | null;

export function HandoverPage() {
  const { id } = useParams();
  const [projectId, setProjectId] = useState(id ?? '');
  const [statusFilter, setStatusFilter] = useState('all');
  const [createOpen, setCreateOpen] = useState(false);
  const [selectedID, setSelectedID] = useState('');
  const [action, setAction] = useState<HandoverAction>(null);
  const queryClient = useQueryClient();
  const toast = useToast();
  const { hasAnyPermission } = useAuth();
  const projects = useQuery({ queryKey: ['projects'], queryFn: () => get<Record<string, any>[]>('/projects') });
  useEffect(() => { if (!projectId && projects.data?.[0]) setProjectId(projects.data[0].id); }, [projects.data, projectId]);
  const handovers = useQuery({ queryKey: ['handover', projectId], queryFn: () => get<HandoverRow[]>(`/projects/${projectId}/handover`), enabled: Boolean(projectId) });
  const options = useQuery({ queryKey: ['handover-options', projectId], queryFn: () => get<{ final_acceptances: Record<string, any>[] }>(`/projects/${projectId}/handover/options`), enabled: Boolean(projectId) });
  const detail = useQuery({ queryKey: ['handover-detail', selectedID], queryFn: () => get<HandoverDetail>(`/handover/${selectedID}`), enabled: Boolean(selectedID) });
  const invalidate = async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: ['handover', projectId] }),
      queryClient.invalidateQueries({ queryKey: ['handover-detail', selectedID] }),
    ]);
  };
  const create = useMutation({
    mutationFn: (values: Record<string, any>) => post<HandoverDetail>(`/projects/${projectId}/handover`, values),
    onSuccess: async (row) => { setCreateOpen(false); setSelectedID(row.id); await invalidate(); toast.success('Đã tạo gói hồ sơ bàn giao'); },
    onError: (error) => toast.error('Không tạo được gói hồ sơ bàn giao', error),
  });
  const transition = useMutation({
    mutationFn: ({ endpoint, values }: { endpoint: string; values?: Record<string, any> }) => post<HandoverDetail>(endpoint, values ?? {}),
    onSuccess: async (row) => { if (row.id) setSelectedID(row.id); setAction(null); await invalidate(); toast.success('Đã cập nhật trạng thái bàn giao'); },
    onError: (error) => toast.error('Không cập nhật được hồ sơ bàn giao', error),
  });
  const checklist = useMutation({
    mutationFn: ({ itemID, values }: { itemID: string; values: Record<string, any> }) => put(`/handover-checklist-items/${itemID}`, values),
    onSuccess: async () => { setAction(null); await invalidate(); toast.success('Đã xác minh checklist bàn giao'); },
    onError: (error) => toast.error('Không cập nhật được checklist', error),
  });

  const rows = handovers.data ?? [];
  const filteredRows = statusFilter === 'all' ? rows : rows.filter((row) => row.status === statusFilter);
  const statusOptions = useMemo<FilterOption[]>(() => [
    { value: 'all', label: 'Tất cả', count: rows.length },
    ...['draft', 'submitted', 'ready', 'blocked', 'handed_over'].map((value) => ({ value, label: value, count: rows.filter((row) => row.status === value).length })),
  ], [rows]);
  const finalAcceptanceOptions = (options.data?.final_acceptances ?? [])
    .map((row) => ({ value: row.id, label: `${row.title} · ${row.result}` }));
  const summary = useMemo(() => ({
    total: rows.length,
    submitted: rows.filter((row) => ['submitted', 'ready', 'blocked'].includes(row.status)).length,
    blocked: rows.filter((row) => row.status === 'blocked').length,
    completed: rows.filter((row) => row.status === 'handed_over').length,
  }), [rows]);

  return <>
    <PageHeader
      title="Đóng tài chính & Bàn giao"
      description="Gói bàn giao khóa đúng nghiệm thu cuối, checklist chứng từ, trạng thái kho, QA/NCR, AP/AR và phiếu thu/chi trước khi hoàn tất."
      actions={<>{id ? <ProjectBackLink projectId={projectId} /> : <ProjectSelector value={projectId} onChange={setProjectId} />}{hasAnyPermission(['handover.create']) ? <button disabled={!projectId} className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white disabled:opacity-50" onClick={() => setCreateOpen(true)}><Plus className="h-4 w-4" /> Gói bàn giao</button> : null}</>}
    />
    <div className="mb-4 grid gap-3 sm:grid-cols-4"><Metric label="Tổng hồ sơ" value={summary.total} /><Metric label="Đã gửi kiểm tra" value={summary.submitted} /><Metric label="Đang blocked" value={summary.blocked} tone={summary.blocked ? 'amber' : 'slate'} /><Metric label="Đã bàn giao" value={summary.completed} tone="green" /></div>
    <div className="mb-4"><FilterBar label="Trạng thái bàn giao" value={statusFilter} options={statusOptions} onChange={setStatusFilter} disabled={!projectId} /></div>
    <ListSection title="Gói hồ sơ bàn giao">
      <DataTable
        rows={filteredRows}
        loading={handovers.isLoading}
        error={handovers.error}
        onRetry={() => handovers.refetch()}
        searchPlaceholder="Tìm mã, đại diện, trạng thái..."
        pageSize={10}
        defaultSort={{ key: 'created_at', direction: 'desc' }}
        columns={[
          { key: 'handover_code', header: 'Hồ sơ', render: (row) => <div><div className="font-semibold text-ink">{row.handover_code}</div><div className="text-xs text-slate-500">{row.client_representative} ↔ {row.company_representative}</div></div> },
          { key: 'final_acceptance_id', header: 'Nghiệm thu cuối', render: (row) => row.final_acceptance_id ? 'Đã khóa nguồn' : <span className="text-red-600">Chưa liên kết</span> },
          { key: 'checklist', header: 'Checklist', render: (row) => <span className={Number(row.checklist_pending_required ?? 0) ? 'font-semibold text-amber-700' : 'font-semibold text-emerald-700'}>{Number(row.checklist_verified ?? 0)}/{Number(row.checklist_total ?? 0)}</span> },
          { key: 'handover_date', header: 'Ngày bàn giao', render: (row) => dateText(row.handover_date) },
          { key: 'status', header: 'Trạng thái', render: (row) => <StatusBadge status={row.status} /> },
          { key: 'action', header: 'Thao tác', render: (row) => <RowActions onView={() => setSelectedID(row.id)}>{hasAnyPermission(['handover.create', 'handover.verify']) && ['draft', 'blocked'].includes(row.status) ? <SmallButton label="Gửi" icon={<Send className="h-3.5 w-3.5" />} onClick={() => transition.mutate({ endpoint: `/handover/${row.id}/submit`, values: { comment: 'Gửi kiểm tra readiness' } })} /> : null}</RowActions> },
        ]}
        empty={projectId ? 'Chưa có gói hồ sơ bàn giao' : 'Chọn công trình để xem bàn giao'}
      />
    </ListSection>

    <RecordFormDialog
      open={createOpen}
      title="Tạo gói hồ sơ bàn giao"
      submitLabel="Tạo hồ sơ nháp"
      onClose={() => setCreateOpen(false)}
      fields={[
        { name: 'handover_code', label: 'Mã bàn giao', defaultValue: `HO-${Date.now().toString().slice(-6)}` },
        { name: 'final_acceptance_id', label: 'Nghiệm thu cuối đạt', type: 'select', required: true, options: finalAcceptanceOptions, helperText: 'Chỉ chọn nghiệm thu final đã pass/conditional pass.' },
        { name: 'handover_date', label: 'Ngày dự kiến bàn giao', type: 'date' },
        { name: 'client_representative', label: 'Đại diện chủ đầu tư', required: true },
        { name: 'company_representative', label: 'Đại diện nhà thầu', required: true },
        { name: 'notes', label: 'Ghi chú', type: 'textarea' },
      ]}
      onSubmit={(values) => create.mutateAsync(values)}
    />

    <Dialog open={Boolean(selectedID)} title={detail.data ? `${detail.data.handover_code} · Gói hồ sơ bàn giao` : 'Gói hồ sơ bàn giao'} description="Checklist chứng từ và readiness được xác minh từ nguồn nghiệp vụ, không dùng checkbox tự khai." onClose={() => { setSelectedID(''); setAction(null); }} size="workspace">
      {detail.isLoading ? <Loading /> : detail.error ? <ErrorState onRetry={() => detail.refetch()} /> : detail.data ? <HandoverWorkspace
        detail={detail.data}
        canVerify={hasAnyPermission(['handover.verify'])}
        canApprove={hasAnyPermission(['handover.approve'])}
        pending={transition.isPending}
        onChecklist={(item) => setAction({ type: 'checklist', item })}
        onSubmit={() => transition.mutate({ endpoint: `/handover/${detail.data.id}/submit`, values: { comment: 'Gửi gói hồ sơ bàn giao' } })}
        onCheck={() => transition.mutate({ endpoint: `/handover/${detail.data.id}/check` })}
        onComplete={() => setAction({ type: 'complete', record: detail.data })}
      /> : null}
    </Dialog>

    <Dialog open={action?.type === 'checklist'} title="Xác minh mục checklist" description={action?.type === 'checklist' ? action.item.title : undefined} onClose={() => setAction(null)} size="lg">
      {action?.type === 'checklist' && detail.data ? <SimpleForm submitLabel="Lưu xác minh" fields={[
        { name: 'status', label: 'Trạng thái', type: 'select', defaultValue: action.item.status === 'verified' ? 'verified' : 'pending', required: true, options: [{ value: 'pending', label: 'Chưa xác minh' }, { value: 'verified', label: 'Đã xác minh' }] },
        { name: 'evidence_id', label: 'Evidence đã accepted', type: 'select', defaultValue: action.item.evidence_id ?? '', options: detail.data.evidence_options.map((item) => ({ value: item.id, label: `${item.name} · ${item.evidence_type}` })) },
        { name: 'document_id', label: 'Tài liệu hồ sơ', type: 'select', defaultValue: action.item.document_id ?? '', options: detail.data.document_options.map((item) => ({ value: item.id, label: `${item.name} · ${item.document_type || 'document'}` })) },
        { name: 'notes', label: 'Ghi chú xác minh', type: 'textarea', defaultValue: action.item.notes ?? '' },
      ]} onSubmit={(values) => checklist.mutateAsync({ itemID: action.item.id, values })} /> : null}
    </Dialog>

    <Dialog open={action?.type === 'complete'} title="Xác nhận hoàn tất bàn giao" description="Hệ thống sẽ kiểm tra lại toàn bộ readiness ngay trước khi khóa hồ sơ." onClose={() => setAction(null)} size="lg">
      {action?.type === 'complete' ? <SimpleForm submitLabel="Hoàn tất bàn giao" fields={[{ name: 'comment', label: 'Căn cứ/xác nhận hai bên', type: 'textarea', required: true, defaultValue: '' }]} onSubmit={(values) => transition.mutateAsync({ endpoint: `/handover/${action.record.id}/complete`, values })} /> : null}
    </Dialog>
  </>;
}

function HandoverWorkspace({ detail, canVerify, canApprove, pending, onChecklist, onSubmit, onCheck, onComplete }: { detail: HandoverDetail; canVerify: boolean; canApprove: boolean; pending: boolean; onChecklist: (item: ChecklistItem) => void; onSubmit: () => void; onCheck: () => void; onComplete: () => void }) {
  const snapshot = detail.readiness_snapshot_json ?? {};
  const blockers = snapshot.blockers ?? [];
  return <div className="space-y-4">
    <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-5"><Metric label="Trạng thái" value={<StatusBadge status={detail.status} />} /><Metric label="Nghiệm thu cuối" value={detail.final_acceptance_result || '-'} /><Metric label="Checklist" value={`${detail.checklist_items.filter((item) => item.status === 'verified').length}/${detail.checklist_items.length}`} /><Metric label="Readiness" value={snapshot.ready ? 'Đạt' : detail.status === 'draft' ? 'Chưa gửi' : 'Chưa đạt'} tone={snapshot.ready ? 'green' : blockers.length ? 'amber' : 'slate'} /><Metric label="Ngày bàn giao" value={dateText(detail.handover_date)} /></div>
    {blockers.length ? <div className="rounded-md border border-amber-200 bg-amber-50 p-4"><div className="font-semibold text-amber-900">Điểm chặn hiện tại</div><ul className="mt-2 list-disc space-y-1 pl-5 text-sm text-amber-900">{blockers.map((item) => <li key={item}>{item}</li>)}</ul></div> : null}
    <div className="flex flex-wrap justify-end gap-2">{canVerify && ['draft', 'blocked'].includes(detail.status) ? <SmallButton label="Gửi kiểm tra" icon={<Send className="h-3.5 w-3.5" />} onClick={onSubmit} /> : null}{canVerify && ['submitted', 'ready', 'blocked'].includes(detail.status) ? <SmallButton label="Kiểm tra lại" icon={<RefreshCw className="h-3.5 w-3.5" />} onClick={onCheck} /> : null}{canApprove && detail.status === 'ready' ? <button disabled={pending} className="inline-flex items-center gap-2 rounded-md bg-emerald-600 px-3 py-2 text-sm font-semibold text-white disabled:opacity-50" onClick={onComplete}><CheckCircle2 className="h-4 w-4" /> Hoàn tất bàn giao</button> : null}</div>
    <div className="overflow-x-auto rounded-md border border-line"><table className="min-w-[900px] w-full text-sm"><thead className="bg-slate-50 text-left text-xs uppercase text-slate-500"><tr><th className="px-3 py-2">Nhóm</th><th className="px-3 py-2">Nội dung</th><th className="px-3 py-2">Nguồn xác minh</th><th className="px-3 py-2">Trạng thái</th><th className="px-3 py-2"></th></tr></thead><tbody className="divide-y divide-line">{detail.checklist_items.map((item) => <tr key={item.id}><td className="px-3 py-2"><span className="rounded bg-slate-100 px-2 py-1 text-xs font-semibold">{item.category}</span></td><td className="px-3 py-2"><div className="font-semibold text-ink">{item.title}</div><div className="text-xs text-slate-500">{item.item_code}{item.required ? ' · bắt buộc' : ''}</div></td><td className="px-3 py-2 text-slate-600">{item.evidence_name || item.document_name || item.source_type || '-'}</td><td className="px-3 py-2"><StatusBadge status={item.status} /></td><td className="px-3 py-2 text-right">{canVerify && !item.source_type ? <SmallButton label="Xác minh" icon={<FileCheck2 className="h-3.5 w-3.5" />} onClick={() => onChecklist(item)} /> : <span className="text-xs text-slate-400">Tự động</span>}</td></tr>)}</tbody></table></div>
  </div>;
}

function Metric({ label, value, tone = 'slate' }: { label: string; value: React.ReactNode; tone?: 'slate' | 'amber' | 'green' }) { const color = tone === 'amber' ? 'border-amber-200 bg-amber-50 text-amber-800' : tone === 'green' ? 'border-emerald-200 bg-emerald-50 text-emerald-800' : 'border-line bg-white text-ink'; return <div className={`rounded-md border px-3 py-2 ${color}`}><div className="text-xs font-semibold uppercase tracking-wide opacity-70">{label}</div><div className="mt-1 text-lg font-semibold">{value}</div></div>; }
function SmallButton({ label, icon, onClick }: { label: string; icon?: React.ReactNode; onClick: () => void }) { return <button type="button" className="inline-flex items-center gap-1.5 rounded-md border border-line bg-white px-2.5 py-1.5 text-xs font-semibold hover:bg-slate-100" onClick={onClick}>{icon}{label}</button>; }
function Loading() { return <div className="flex items-center gap-2 rounded-md border border-dashed border-line p-6 text-sm text-slate-500"><Loader2 className="h-4 w-4 animate-spin" /> Đang tải gói hồ sơ bàn giao...</div>; }
function ErrorState({ onRetry }: { onRetry: () => void }) { return <div className="flex items-center justify-between rounded-md border border-red-200 bg-red-50 p-4 text-sm text-red-700"><span>Không tải được gói hồ sơ bàn giao.</span><button className="inline-flex items-center gap-1 font-semibold underline" onClick={onRetry}><RefreshCw className="h-3.5 w-3.5" /> Thử lại</button></div>; }
