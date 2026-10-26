import { useEffect, useMemo, useRef, useState } from 'react';
import { dateText, dateTimeText } from '../../lib/utils/format';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useParams } from 'react-router-dom';
import { CheckCircle2, ClipboardCheck, FileWarning, Loader2, Plus, RefreshCw, Repeat2, Send, ShieldCheck } from 'lucide-react';
import { PageHeader } from '../../components/PageHeader';
import { ProjectBackLink } from '../../components/ProjectBackLink';
import { Dialog } from '../../components/Dialog';
import { DataTable } from '../../components/DataTable';
import { StatusBadge } from '../../components/StatusBadge';
import { EnumChip } from '../../components/EnumChip';
import { identityReferenceText } from '../../components/IdentityReferenceLabel';
import { useToast } from '../../components/ToastProvider';
import { get, post } from '../../lib/api/client';
import { useAuth } from '../../lib/auth/AuthContext';
import type { QualityITPPlan, QualityWorkspace, RecurringOccurrence } from '../../lib/types/domain';
import { SimpleForm } from '../_shared/SimpleForm';
import { useValidationNavigation } from '../../lib/utils/formValidationNavigation';

type CreateKind = 'itp' | 'rfi' | 'ncr' | 'recurring' | null;
type ActionState = { title: string; description: string; endpoint: string; fields: Record<string, any>[]; success: string } | null;
type ITPItemDraft = { activity_name: string; inspection_stage: string; acceptance_criteria: string; inspection_method: string; control_point: string; frequency_type: string; frequency_value: string; responsible_role: string; evidence_required: boolean; client_witness_required: boolean };
type ITPDraft = { itp_code: string; title: string; discipline: string; work_package: string; items: ITPItemDraft[] };

const emptyITPItem = (): ITPItemDraft => ({ activity_name: '', inspection_stage: '', acceptance_criteria: '', inspection_method: '', control_point: 'review', frequency_type: 'each_lot', frequency_value: '1', responsible_role: 'QA/QC', evidence_required: true, client_witness_required: false });

function recurringOccurrenceAssigneeLabel(row: RecurringOccurrence) {
  const label = identityReferenceText(row.assigned_user_name, row.assigned_user_account_deleted, row.responsible_role || '-');
  return row.assigned_user_account_deleted ? `${label} · Cần phân công lại` : label;
}

export function QualityPage({ embedded = false, projectId, section = 'all' }: { embedded?: boolean; projectId?: string; section?: 'all' | 'checklist' | 'issues' }) {
  const { id: routeId = '' } = useParams();
  const id = projectId ?? routeId;
  const { hasAnyPermission } = useAuth();
  const toast = useToast();
  const queryClient = useQueryClient();
  const [createKind, setCreateKind] = useState<CreateKind>(null);
  const [actionState, setActionState] = useState<ActionState>(null);
  const [workspaceView, setWorkspaceView] = useState<'field' | 'itp'>('field');
  const [itpDraft, setITPDraft] = useState<ITPDraft>({ itp_code: `ITP-${Date.now().toString().slice(-5)}`, title: '', discipline: '', work_package: '', items: [emptyITPItem()] });
  const [itpValidationRequestId, setITPValidationRequestId] = useState(0);
  const quality = useQuery({ queryKey: ['quality', id], queryFn: () => get<QualityWorkspace>(`/projects/${id}/quality`), enabled: Boolean(id) });
  const data = quality.data;
  const invalidate = () => queryClient.invalidateQueries({ queryKey: ['quality', id] });
  const mutate = useMutation({
    mutationFn: ({ endpoint, values }: { endpoint: string; values?: Record<string, any> }) => post<QualityWorkspace>(endpoint, values ?? {}),
    onSuccess: async () => {
      await invalidate();
      setCreateKind(null);
      setActionState(null);
      toast.success(actionState?.success ?? 'Đã cập nhật QA/QC');
    },
    onError: (error) => toast.error('Không thực hiện được thao tác QA/QC', error),
  });
  const summary = useMemo(() => ({
    activeITP: (data?.itp_plans ?? []).filter((row) => row.status === 'active').length,
    pendingRFI: (data?.inspection_requests ?? []).filter((row) => ['draft', 'submitted', 'scheduled'].includes(row.status)).length,
    openNCR: (data?.ncrs ?? []).filter((row) => row.status !== 'closed').length,
    overdue: (data?.recurring_occurrences ?? []).filter((row) => row.effective_status === 'overdue').length,
  }), [data]);

  function resetITP() {
    setITPDraft({ itp_code: `ITP-${Date.now().toString().slice(-5)}`, title: '', discipline: '', work_package: '', items: [emptyITPItem()] });
    setITPValidationRequestId(0);
    setCreateKind('itp');
  }

  function saveITP() {
    if (!itpDraft.itp_code.trim() || !itpDraft.title.trim() || !itpDraft.discipline.trim() || !itpDraft.work_package.trim() || itpDraft.items.some((item) => !item.activity_name.trim() || !item.acceptance_criteria.trim() || !item.inspection_method.trim())) {
      toast.warn({ title: 'ITP chưa hợp lệ', description: 'Điền thông tin ITP và đầy đủ activity/criteria/method cho mọi inspection item.' });
      setITPValidationRequestId((current) => current + 1);
      return;
    }
    mutate.mutate({ endpoint: `/projects/${id}/quality/itp-plans`, values: { ...itpDraft, items: itpDraft.items.map((item, index) => ({ ...item, sequence_no: index + 1, frequency_value: Number(item.frequency_value || 1) })) } });
  }

  const activeITPItems = (data?.itp_plans ?? []).filter((plan) => plan.status === 'active').flatMap((plan) => plan.items.map((item) => ({ value: item.id, label: `${plan.itp_code} R${plan.revision_no} · ${item.sequence_no}. ${item.activity_name} · ${item.control_point}` })));
  const evidenceOptions = (data?.evidence_options ?? []).map((item) => ({ value: item.id, label: `${item.name} · ${item.work_item_title || item.evidence_type}` }));
  const workItemOptions = (data?.work_items ?? []).map((item) => ({ value: item.id, label: `${item.title} · ${item.status}` }));
  const participantOptions = (data?.participants ?? []).map((item) => ({ value: item.id, label: `${item.full_name} · ${item.email}` }));
  const showITP = section === 'all' || (section === 'checklist' && workspaceView === 'itp');
  const showFieldChecks = section === 'all' || (section === 'checklist' && workspaceView === 'field');
  const showNCR = section === 'all' || section === 'issues';

  return <>
    {!embedded ? <PageHeader title="QA/QC & Thi công" actions={<><ProjectBackLink projectId={id} />{hasAnyPermission(['quality.create']) ? <button className="inline-flex items-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold hover:bg-slate-100" onClick={() => setCreateKind('rfi')}><ClipboardCheck className="h-4 w-4" /> Tạo RFI</button> : null}{hasAnyPermission(['quality.create']) ? <button className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700" onClick={resetITP}><Plus className="h-4 w-4" /> Tạo ITP</button> : null}</>} /> : null}
    {embedded ? <div className="mb-4 flex flex-col gap-3 rounded-lg border border-line bg-white px-4 py-3 sm:flex-row sm:items-center sm:justify-between"><div><h2 className="font-semibold text-ink">{section === 'issues' ? 'Không phù hợp chất lượng (NCR)' : 'Kiểm soát chất lượng hiện trường'}</h2></div><div className="flex flex-wrap items-center gap-2">{section === 'checklist' ? <div className="flex rounded-md border border-line bg-slate-50 p-1"><button className={`rounded px-3 py-1.5 text-xs font-semibold ${workspaceView === 'field' ? 'bg-white text-blue-700 shadow-sm' : 'text-slate-600'}`} onClick={() => setWorkspaceView('field')}>Kiểm tra hiện trường</button><button className={`rounded px-3 py-1.5 text-xs font-semibold ${workspaceView === 'itp' ? 'bg-white text-blue-700 shadow-sm' : 'text-slate-600'}`} onClick={() => setWorkspaceView('itp')}>Cấu hình ITP</button></div> : null}{hasAnyPermission(['quality.create']) && section === 'checklist' && workspaceView === 'field' ? <button className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white" onClick={() => setCreateKind('rfi')}><ClipboardCheck className="h-4 w-4" /> Tạo RFI</button> : null}{hasAnyPermission(['quality.create']) && section === 'checklist' && workspaceView === 'itp' ? <button className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white" onClick={resetITP}><Plus className="h-4 w-4" /> Tạo ITP</button> : null}{hasAnyPermission(['quality.ncr_manage']) && section === 'issues' ? <button className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white" onClick={() => setCreateKind('ncr')}><Plus className="h-4 w-4" /> Tạo NCR</button> : null}</div></div> : null}
    {!embedded ? <div className="mb-4 grid gap-3 sm:grid-cols-4"><Metric label="ITP active" value={summary.activeITP} /><Metric label="RFI chờ xử lý" value={summary.pendingRFI} /><Metric label="NCR đang mở" value={summary.openNCR} tone={summary.openNCR ? 'red' : 'slate'} /><Metric label="Occurrence quá hạn" value={summary.overdue} tone={summary.overdue ? 'amber' : 'slate'} /></div> : null}
    {quality.isLoading ? <Loading /> : quality.error ? <ErrorState onRetry={() => quality.refetch()} /> : data ? <div className="space-y-4">
      <Section hidden={!showITP} title="Bộ checklist chất lượng (ITP)" icon={<ShieldCheck className="h-4 w-4" />} action={hasAnyPermission(['quality.create']) ? <button className="text-sm font-semibold text-blue-700" onClick={resetITP}>Tạo ITP</button> : null}><ITPTable rows={data.itp_plans} permissions={{ review: hasAnyPermission(['quality.review']), approve: hasAnyPermission(['quality.approve']), create: hasAnyPermission(['quality.create']) }} onAction={(value) => setActionState(value)} /></Section>
      <Section hidden={!showFieldChecks} title="Chờ kiểm tra (RFI)" icon={<ClipboardCheck className="h-4 w-4" />} action={hasAnyPermission(['quality.create']) ? <button className="text-sm font-semibold text-blue-700" onClick={() => setCreateKind('rfi')}>Tạo RFI</button> : null}><DataTable rows={data.inspection_requests} searchPlaceholder="Tìm RFI, ITP, vị trí" pageSize={10} defaultSort={{ key: 'requested_inspection_at', direction: 'desc' }} columns={[{ key: 'rfi_code', header: 'RFI', render: (row) => <div><div className="font-semibold">{row.rfi_code}</div><div className="text-xs text-slate-500">{row.itp_code} · {row.control_point}</div></div> }, { key: 'activity_name', header: 'Tiêu chí kiểm tra' }, { key: 'location', header: 'Vị trí' }, { key: 'requested_inspection_at', header: 'Thời điểm', render: (row) => dateTimeText(row.requested_inspection_at) }, { key: 'status', header: 'Trạng thái', render: (row) => <StatusBadge status={row.status} /> }, { key: 'result', header: 'Kết quả', render: (row) => row.result ? <EnumChip group="result" value={row.result} /> : '-' }, { key: 'actions', header: 'Thao tác', render: (row) => <div className="flex gap-1">{hasAnyPermission(['quality.create']) && row.status === 'draft' ? <SmallButton label="Gửi" onClick={() => mutate.mutate({ endpoint: `/quality/rfis/${row.id}/submit` })} /> : null}{hasAnyPermission(['quality.inspect']) && ['submitted', 'scheduled'].includes(row.status) ? <SmallButton label="Kiểm tra" onClick={() => setActionState({ title: `Kiểm tra ${row.rfi_code}`, description: `${row.activity_name} · ${row.location}`, endpoint: `/quality/rfis/${row.id}/inspect`, success: 'Đã ghi nhận kết quả kiểm tra', fields: [{ name: 'result', label: 'Kết quả', type: 'select', defaultValue: 'pass', required: true, options: [{ value: 'pass', label: 'Đạt' }, { value: 'conditional_pass', label: 'Đạt có điều kiện' }, { value: 'fail', label: 'Không đạt' }] }, { name: 'inspection_evidence_id', label: 'Minh chứng kiểm tra', type: 'select', defaultValue: '', required: true, options: evidenceOptions }, { name: 'comment', label: 'Nhận xét', type: 'textarea', defaultValue: '', required: true }] })} /> : null}</div> }]} /></Section>
      <Section hidden={!showNCR} title="Không phù hợp & sửa lỗi (NCR)" icon={<FileWarning className="h-4 w-4" />} action={hasAnyPermission(['quality.ncr_manage']) ? <button className="text-sm font-semibold text-blue-700" onClick={() => setCreateKind('ncr')}>Tạo NCR</button> : null}><DataTable rows={data.ncrs} searchPlaceholder="Tìm NCR, không phù hợp, sửa lỗi" pageSize={10} defaultSort={{ key: 'created_at', direction: 'desc' }} columns={[{ key: 'ncr_code', header: 'NCR', render: (row) => <div><div className="font-semibold">{row.ncr_code}</div><div className="text-xs text-slate-500">{row.rfi_code || row.source_type}</div></div> }, { key: 'title', header: 'Không phù hợp', render: (row) => <div><div>{row.title}</div><div className="text-xs text-slate-500">{row.nonconformity}</div></div> }, { key: 'severity', header: 'Mức độ', render: (row) => <EnumChip group="severity" value={row.severity} /> }, { key: 'rework_status', header: 'Sửa lỗi', render: (row) => row.rework_status ? <StatusBadge status={row.rework_status} /> : '-' }, { key: 'status', header: 'NCR', render: (row) => <StatusBadge status={row.status} /> }, { key: 'actions', header: 'Thao tác', render: (row) => hasAnyPermission(['quality.ncr_manage']) ? <NCRAction row={row} evidenceOptions={evidenceOptions} rfiOptions={data.inspection_requests.filter((rfi) => ['accepted', 'conditional'].includes(rfi.status)).map((rfi) => ({ value: rfi.id, label: `${rfi.rfi_code} · ${rfi.result}` }))} onAction={setActionState} /> : null }]} /></Section>
      <Section hidden={!showFieldChecks} title="Kiểm tra định kỳ" icon={<Repeat2 className="h-4 w-4" />} action={hasAnyPermission(['quality.recurring_manage']) ? <button className="text-sm font-semibold text-blue-700" onClick={() => setCreateKind('recurring')}>Tạo lịch lặp</button> : null}><div className="grid gap-4 xl:grid-cols-[0.8fr_1.2fr]"><DataTable rows={data.recurring_plans} searchPlaceholder="Tìm lịch QA/QC" pageSize={8} columns={[{ key: 'plan_code', header: 'Kế hoạch', render: (row) => <div><div className="font-semibold">{row.plan_code} · {row.title}</div><div className="text-xs text-slate-500">{row.recurrence_type} / {row.interval_value} · {row.responsible_role}</div></div> }, { key: 'next_due_date', header: 'Kỳ tiếp theo', render: (row) => dateText(row.next_due_date) }, { key: 'status', header: 'Trạng thái', render: (row) => <StatusBadge status={row.status} /> }, { key: 'actions', header: '', render: (row) => hasAnyPermission(['quality.recurring_manage']) && row.status === 'active' ? <SmallButton label="Sinh kỳ" icon={<RefreshCw className="h-3.5 w-3.5" />} onClick={() => mutate.mutate({ endpoint: `/quality/recurring-plans/${row.id}/generate`, values: { through_date: new Date(Date.now() + 7 * 86400000).toISOString().slice(0, 10) } })} /> : null }]} /><DataTable rows={data.recurring_occurrences} searchPlaceholder="Tìm lần kiểm tra" pageSize={10} defaultSort={{ key: 'occurrence_date', direction: 'desc' }} columns={[{ key: 'plan_title', header: 'Lần kiểm tra', render: (row) => <div><div className="font-semibold">{row.plan_title}</div><div className="text-xs text-slate-500">{row.occurrence_date} · {recurringOccurrenceAssigneeLabel(row)}</div></div> }, { key: 'effective_status', header: 'Trạng thái', render: (row) => <StatusBadge status={row.effective_status} /> }, { key: 'actions', header: '', render: (row) => hasAnyPermission(['quality.recurring_manage']) && ['pending', 'in_progress', 'overdue'].includes(row.effective_status) ? <SmallButton label="Hoàn tất" icon={<CheckCircle2 className="h-3.5 w-3.5" />} onClick={() => setActionState({ title: 'Hoàn tất kiểm tra định kỳ', description: `${row.plan_title} · ${row.occurrence_date}`, endpoint: `/quality/recurring-occurrences/${row.id}/complete`, success: 'Đã hoàn tất kiểm tra định kỳ', fields: [{ name: 'comment', label: 'Kết quả thực hiện', type: 'textarea', defaultValue: '', required: true }, { name: 'evidence_id', label: 'Minh chứng', type: 'select', defaultValue: '', required: row.evidence_required, options: evidenceOptions }] })} /> : null }]} /></div></Section>
    </div> : null}

    <ValidatedITPDialog open={createKind === 'itp'} draft={itpDraft} pending={mutate.isPending} validationRequestId={itpValidationRequestId} onChange={setITPDraft} onClose={() => setCreateKind(null)} onSave={saveITP} />
    <Dialog open={createKind === 'rfi'} title="Tạo Request for Inspection" description="RFI phải liên kết một inspection item thuộc ITP active." onClose={() => setCreateKind(null)} size="lg"><SimpleForm submitLabel="Tạo RFI nháp" fields={[{ name: 'rfi_code', label: 'Mã RFI', defaultValue: `RFI-${Date.now().toString().slice(-6)}`, required: true }, { name: 'itp_item_id', label: 'ITP inspection item', type: 'select', defaultValue: '', required: true, options: activeITPItems }, { name: 'work_item_id', label: 'Work item hiện trường', type: 'select', defaultValue: '', options: workItemOptions }, { name: 'location', label: 'Vị trí', defaultValue: '', required: true }, { name: 'lot_reference', label: 'Lô / khu vực', defaultValue: '' }, { name: 'requested_inspection_at', label: 'Thời điểm inspection (ISO 8601)', defaultValue: new Date(Date.now() + 3600000).toISOString(), required: true, helperText: 'Ví dụ: 2026-07-10T09:00:00.000Z' }, { name: 'request_evidence_id', label: 'Minh chứng yêu cầu', type: 'select', defaultValue: '', options: evidenceOptions }, { name: 'request_comment', label: 'Nội dung mời kiểm tra', type: 'textarea', defaultValue: '', required: true, fullWidth: true }]} onSubmit={(values) => mutate.mutateAsync({ endpoint: `/projects/${id}/quality/rfis`, values: { ...values, work_item_id: values.work_item_id || null, request_evidence_id: values.request_evidence_id || null, requested_inspection_at: new Date(values.requested_inspection_at).toISOString() } })} /></Dialog>
    <Dialog open={createKind === 'ncr'} title="Tạo NCR thủ công" description="NCR sẽ tự tạo một rework request liên kết." onClose={() => setCreateKind(null)} size="lg"><SimpleForm submitLabel="Tạo NCR" fields={[{ name: 'ncr_code', label: 'Mã NCR', defaultValue: `NCR-${Date.now().toString().slice(-6)}`, required: true }, { name: 'title', label: 'Tiêu đề', defaultValue: 'Không phù hợp hiện trường', required: true }, { name: 'severity', label: 'Mức độ', type: 'select', defaultValue: 'high', required: true, options: ['low', 'medium', 'high', 'critical'].map((value) => ({ value, label: value })) }, { name: 'work_item_id', label: 'Work item nguồn', type: 'select', defaultValue: '', options: workItemOptions }, { name: 'nonconformity', label: 'Mô tả không phù hợp', type: 'textarea', defaultValue: '', required: true, fullWidth: true }, { name: 'immediate_action', label: 'Xử lý tức thời', type: 'textarea', defaultValue: '', fullWidth: true }, { name: 'due_date', label: 'Hạn khắc phục', type: 'date', defaultValue: '' }, { name: 'evidence_id', label: 'Minh chứng NCR', type: 'select', defaultValue: '', options: evidenceOptions }, { name: 'owner_id', label: 'Người phụ trách', type: 'select', defaultValue: '', options: participantOptions }]} onSubmit={(values) => mutate.mutateAsync({ endpoint: `/projects/${id}/quality/ncrs`, values: { ...values, source_type: values.work_item_id ? 'work_item' : 'manual', source_id: values.work_item_id || id } })} /></Dialog>
    <Dialog open={createKind === 'recurring'} title="Tạo lịch QA/QC lặp" onClose={() => setCreateKind(null)} size="lg"><SimpleForm submitLabel="Tạo lịch" fields={[{ name: 'plan_code', label: 'Mã lịch', defaultValue: `QA-${Date.now().toString().slice(-5)}`, required: true }, { name: 'title', label: 'Tên công việc lặp', defaultValue: 'QA walkdown hiện trường', required: true }, { name: 'description', label: 'Mô tả', type: 'textarea', defaultValue: '', fullWidth: true }, { name: 'recurrence_type', label: 'Chu kỳ', type: 'select', defaultValue: 'daily', required: true, options: [{ value: 'daily', label: 'Hằng ngày' }, { value: 'weekly', label: 'Hằng tuần' }, { value: 'monthly', label: 'Hằng tháng' }, { value: 'custom_days', label: 'Theo số ngày' }] }, { name: 'interval_value', label: 'Mỗi N kỳ', type: 'number', defaultValue: 1, required: true, min: 1 }, { name: 'start_date', label: 'Ngày bắt đầu', type: 'date', defaultValue: new Date().toISOString().slice(0, 10), required: true }, { name: 'end_date', label: 'Ngày kết thúc', type: 'date', defaultValue: '' }, { name: 'responsible_role', label: 'Vai trò phụ trách', defaultValue: 'QA/QC', required: true }, { name: 'assigned_user_id', label: 'Người thực hiện', type: 'select', defaultValue: '', options: participantOptions }, { name: 'source_work_item_id', label: 'Work item nguồn', type: 'select', defaultValue: '', options: workItemOptions }, { name: 'evidence_required', label: 'Bắt buộc minh chứng', type: 'checkbox', defaultValue: true }]} onSubmit={(values) => mutate.mutateAsync({ endpoint: `/projects/${id}/quality/recurring-plans`, values })} /></Dialog>
    <Dialog open={Boolean(actionState)} title={actionState?.title ?? ''} description={actionState?.description} onClose={() => setActionState(null)} size="lg">{actionState ? <SimpleForm submitLabel="Xác nhận" fields={actionState.fields as any} onSubmit={(values) => mutate.mutateAsync({ endpoint: actionState.endpoint, values })} /> : null}</Dialog>
  </>;
}

function ITPTable({ rows, permissions, onAction }: { rows: QualityITPPlan[]; permissions: { review: boolean; approve: boolean; create: boolean }; onAction: (value: NonNullable<ActionState>) => void }) { return <DataTable rows={rows} searchPlaceholder="Tìm ITP, bộ môn, gói công việc" pageSize={10} defaultSort={{ key: 'revision_no', direction: 'desc' }} columns={[{ key: 'itp_code', header: 'ITP', render: (row) => <div><div className="font-semibold">{row.itp_code} · R{row.revision_no}</div><div className="text-xs text-slate-500">{row.title}</div></div> }, { key: 'discipline', header: 'Bộ môn' }, { key: 'work_package', header: 'Gói công việc' }, { key: 'item_count', header: 'Inspection items', align: 'right' }, { key: 'definition_hash', header: 'Hash', render: (row) => <span className="font-mono text-xs">{row.definition_hash?.slice(0, 12)}…</span> }, { key: 'status', header: 'Trạng thái', render: (row) => <StatusBadge status={row.status} /> }, { key: 'actions', header: 'Thao tác', render: (row) => <div className="flex gap-1">{permissions.review && row.status === 'draft' ? <SmallButton label="Gửi" icon={<Send className="h-3.5 w-3.5" />} onClick={() => onAction({ title: 'Gửi review ITP', description: `${row.itp_code} R${row.revision_no}`, endpoint: `/quality/itp-plans/${row.id}/submit`, success: 'Đã gửi review ITP', fields: [{ name: 'comment', label: 'Ghi chú', type: 'textarea', defaultValue: '' }] })} /> : null}{permissions.approve && row.status === 'review' ? <SmallButton label="Duyệt" onClick={() => onAction({ title: 'Duyệt ITP', description: `${row.itp_code} R${row.revision_no}`, endpoint: `/quality/itp-plans/${row.id}/approve`, success: 'Đã duyệt ITP', fields: [{ name: 'comment', label: 'Nhận xét duyệt', type: 'textarea', defaultValue: '', required: true }] })} /> : null}{permissions.approve && row.status === 'approved' ? <SmallButton label="Kích hoạt" onClick={() => onAction({ title: 'Kích hoạt ITP', description: 'Revision active trước đó sẽ được supersede.', endpoint: `/quality/itp-plans/${row.id}/activate`, success: 'Đã kích hoạt ITP', fields: [{ name: 'comment', label: 'Căn cứ áp dụng', type: 'textarea', defaultValue: '', required: true }] })} /> : null}{permissions.create && row.status === 'active' ? <SmallButton label="Tạo revision" onClick={() => onAction({ title: 'Tạo revision ITP', description: `${row.itp_code} R${row.revision_no + 1}`, endpoint: `/quality/itp-plans/${row.id}/revise`, success: 'Đã tạo revision ITP', fields: [{ name: 'comment', label: 'Lý do revision', type: 'textarea', defaultValue: '', required: true }] })} /> : null}</div> }]} />; }

function NCRAction({ row, evidenceOptions, rfiOptions, onAction }: { row: any; evidenceOptions: any[]; rfiOptions: any[]; onAction: (value: NonNullable<ActionState>) => void }) { if (row.status === 'open') return <SmallButton label="Root cause" onClick={() => onAction({ title: `Root cause ${row.ncr_code}`, description: row.title, endpoint: `/quality/ncrs/${row.id}/root-cause`, success: 'Đã ghi root cause', fields: [{ name: 'root_cause', label: 'Nguyên nhân gốc', type: 'textarea', defaultValue: '', required: true }, { name: 'preventive_action', label: 'Hành động phòng ngừa', type: 'textarea', defaultValue: '' }] })} />; if (row.status === 'root_cause') return <SmallButton label="Corrective" onClick={() => onAction({ title: `Corrective action ${row.ncr_code}`, description: row.root_cause, endpoint: `/quality/ncrs/${row.id}/corrective`, success: 'Đã ghi corrective action', fields: [{ name: 'corrective_action', label: 'Hành động khắc phục', type: 'textarea', defaultValue: '', required: true }, { name: 'corrective_evidence_id', label: 'Minh chứng khắc phục', type: 'select', defaultValue: '', required: true, options: evidenceOptions }, { name: 'preventive_action', label: 'Phòng ngừa tái diễn', type: 'textarea', defaultValue: '' }] })} />; if (row.status === 'corrective_action') return <SmallButton label="Tái kiểm tra" onClick={() => onAction({ title: `Liên kết RFI tái kiểm tra`, description: row.ncr_code, endpoint: `/quality/ncrs/${row.id}/reinspection`, success: 'Đã liên kết RFI tái kiểm tra', fields: [{ name: 'reinspection_rfi_id', label: 'RFI đạt sau rework', type: 'select', defaultValue: '', required: true, options: rfiOptions }] })} />; if (row.status === 'reinspection') return <SmallButton label="Đóng NCR" onClick={() => onAction({ title: `Đóng ${row.ncr_code}`, description: 'Chỉ đóng khi RFI tái kiểm tra đạt và rework đã hoàn tất.', endpoint: `/quality/ncrs/${row.id}/close`, success: 'Đã đóng NCR', fields: [{ name: 'comment', label: 'Kết luận đóng NCR', type: 'textarea', defaultValue: '', required: true }] })} />; return null; }

function ValidatedITPDialog({ open, draft, pending, validationRequestId, onChange, onClose, onSave }: {
  open: boolean;
  draft: ITPDraft;
  pending: boolean;
  validationRequestId: number;
  onChange: (value: ITPDraft) => void;
  onClose: () => void;
  onSave: () => void;
}) {
  const contentRef = useRef<HTMLDivElement | null>(null);
  const navigateValidation = useValidationNavigation(contentRef);
  const setItem = (index: number, patch: Partial<ITPItemDraft>) => onChange({
    ...draft,
    items: draft.items.map((item, itemIndex) => itemIndex === index ? { ...item, ...patch } : item),
  });

  useEffect(() => {
    if (!open || !validationRequestId) return;
    navigateValidation(firstITPDraftInvalidKey(draft));
  }, [navigateValidation, open, validationRequestId]);

  return (
    <Dialog
      open={open}
      title="Tạo Inspection & Test Plan"
      description="ITP revision sau activation sẽ khóa nội dung và inspection items."
      onClose={onClose}
      size="workspace"
      footer={<div className="flex justify-end gap-2"><button className="rounded-md border border-line px-3 py-2 text-sm font-semibold" onClick={onClose}>Đóng</button><button className="rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white disabled:opacity-50" disabled={pending} onClick={onSave}>Lưu ITP nháp</button></div>}
    >
      <div ref={contentRef}>
        <div className="grid gap-3 md:grid-cols-4">
          <Field fieldKey="itp_code" invalid={Boolean(validationRequestId && !draft.itp_code.trim())} label="Mã ITP" value={draft.itp_code} onChange={(itp_code) => onChange({ ...draft, itp_code })} />
          <Field fieldKey="title" invalid={Boolean(validationRequestId && !draft.title.trim())} label="Tên ITP" value={draft.title} onChange={(title) => onChange({ ...draft, title })} />
          <Field fieldKey="discipline" invalid={Boolean(validationRequestId && !draft.discipline.trim())} label="Bộ môn" value={draft.discipline} onChange={(discipline) => onChange({ ...draft, discipline })} />
          <Field fieldKey="work_package" invalid={Boolean(validationRequestId && !draft.work_package.trim())} label="Gói công việc" value={draft.work_package} onChange={(work_package) => onChange({ ...draft, work_package })} />
        </div>
        <div className="consflow-table-viewport mt-4 rounded-md border border-line">
          <table className="min-w-[1250px] w-full text-sm">
            <thead className="bg-slate-50 text-left text-xs uppercase text-slate-500"><tr><th className="px-2 py-2"><span className="sr-only">Thao tác</span></th><th className="px-2 py-2">#</th><th className="px-2 py-2">Activity</th><th className="px-2 py-2">Stage</th><th className="px-2 py-2">Acceptance criteria</th><th className="px-2 py-2">Method</th><th className="px-2 py-2">Point</th><th className="px-2 py-2">Frequency</th><th className="px-2 py-2">Role</th><th className="px-2 py-2">Evidence</th></tr></thead>
            <tbody className="divide-y divide-line">
              {draft.items.map((item, index) => (
                <tr key={index}>
                  <td className="px-2">{draft.items.length > 1 ? <button className="text-xs font-semibold text-red-600" onClick={() => onChange({ ...draft, items: draft.items.filter((_, itemIndex) => itemIndex !== index) })}>Xóa</button> : null}</td>
                  <td className="px-2">{index + 1}</td>
                  <td className="px-2 py-2"><Mini fieldKey={`items.${index}.activity_name`} invalid={Boolean(validationRequestId && !item.activity_name.trim())} value={item.activity_name} onChange={(activity_name) => setItem(index, { activity_name })} /></td>
                  <td className="px-2"><Mini value={item.inspection_stage} onChange={(inspection_stage) => setItem(index, { inspection_stage })} /></td>
                  <td className="px-2"><Mini fieldKey={`items.${index}.acceptance_criteria`} invalid={Boolean(validationRequestId && !item.acceptance_criteria.trim())} value={item.acceptance_criteria} onChange={(acceptance_criteria) => setItem(index, { acceptance_criteria })} /></td>
                  <td className="px-2"><Mini fieldKey={`items.${index}.inspection_method`} invalid={Boolean(validationRequestId && !item.inspection_method.trim())} value={item.inspection_method} onChange={(inspection_method) => setItem(index, { inspection_method })} /></td>
                  <td className="px-2"><select className="rounded-md border border-line px-2 py-1.5" value={item.control_point} onChange={(event) => setItem(index, { control_point: event.target.value })}>{['review', 'witness', 'hold', 'surveillance'].map((value) => <option key={value}>{value}</option>)}</select></td>
                  <td className="px-2"><div className="flex gap-1"><select className="rounded-md border border-line px-2 py-1.5" value={item.frequency_type} onChange={(event) => setItem(index, { frequency_type: event.target.value })}>{['each_lot', 'daily', 'weekly', 'quantity', 'custom'].map((value) => <option key={value}>{value}</option>)}</select><input className="w-16 rounded-md border border-line px-2" type="number" min={1} value={item.frequency_value} onChange={(event) => setItem(index, { frequency_value: event.target.value })} /></div></td>
                  <td className="px-2"><Mini value={item.responsible_role} onChange={(responsible_role) => setItem(index, { responsible_role })} /></td>
                  <td className="px-2 text-center"><input type="checkbox" checked={item.evidence_required} onChange={(event) => setItem(index, { evidence_required: event.target.checked })} /></td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        <button className="mt-3 inline-flex items-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold" onClick={() => onChange({ ...draft, items: [...draft.items, emptyITPItem()] })}><Plus className="h-4 w-4" /> Inspection item</button>
      </div>
    </Dialog>
  );
}

function firstITPDraftInvalidKey(draft: ITPDraft) {
  if (!draft.itp_code.trim()) return 'itp_code';
  if (!draft.title.trim()) return 'title';
  if (!draft.discipline.trim()) return 'discipline';
  if (!draft.work_package.trim()) return 'work_package';
  for (let index = 0; index < draft.items.length; index += 1) {
    const item = draft.items[index];
    if (!item.activity_name.trim()) return `items.${index}.activity_name`;
    if (!item.acceptance_criteria.trim()) return `items.${index}.acceptance_criteria`;
    if (!item.inspection_method.trim()) return `items.${index}.inspection_method`;
  }
  return undefined;
}

function Section({ title, icon, action, children, hidden = false }: { title: string; icon: React.ReactNode; action?: React.ReactNode; children: React.ReactNode; hidden?: boolean }) { if (hidden) return null; return <section className="overflow-clip rounded-lg border border-line bg-white"><div className="flex items-center justify-between border-b border-line px-4 py-3"><h2 className="flex items-center gap-2 font-semibold text-ink">{icon}{title}</h2>{action}</div><div className="p-4">{children}</div></section>; }
function Metric({ label, value, tone = 'slate' }: { label: string; value: React.ReactNode; tone?: 'slate' | 'red' | 'amber' }) { const toneClass = tone === 'red' ? 'border-red-200 bg-red-50 text-red-700' : tone === 'amber' ? 'border-amber-200 bg-amber-50 text-amber-700' : 'border-line bg-white text-ink'; return <div className={`rounded-md border px-3 py-2 ${toneClass}`}><div className="text-xs font-semibold uppercase tracking-wide opacity-70">{label}</div><div className="mt-1 text-lg font-semibold">{value}</div></div>; }
function SmallButton({ label, icon, onClick }: { label: string; icon?: React.ReactNode; onClick: () => void }) { return <button className="inline-flex items-center gap-1 rounded-md border border-line bg-white px-2 py-1.5 text-xs font-semibold hover:bg-slate-100" onClick={onClick}>{icon}{label}</button>; }
function Field({ fieldKey, label, value, onChange, invalid = false }: { fieldKey?: string; label: string; value: string; onChange: (value: string) => void; invalid?: boolean }) { return <label data-validation-key={fieldKey} data-validation-error={invalid ? 'true' : undefined}><span className="text-sm font-semibold text-slate-700">{label} *</span><input name={fieldKey} className={`mt-1 w-full rounded-md border px-3 py-2 text-sm ${invalid ? 'border-rose-300 bg-rose-50' : 'border-line'}`} aria-invalid={invalid} value={value} onChange={(event) => onChange(event.target.value)} /></label>; }
function Mini({ fieldKey, value, onChange, invalid = false }: { fieldKey?: string; value: string; onChange: (value: string) => void; invalid?: boolean }) { return <div data-validation-key={fieldKey} data-validation-error={invalid ? 'true' : undefined}><input name={fieldKey} className={`w-44 rounded-md border px-2 py-1.5 text-sm ${invalid ? 'border-rose-300 bg-rose-50' : 'border-line'}`} aria-invalid={invalid} value={value} onChange={(event) => onChange(event.target.value)} /></div>; }
function Loading() { return <div className="flex items-center gap-2 rounded-md border border-dashed border-line p-6 text-sm text-slate-500"><Loader2 className="h-4 w-4 animate-spin" /> Đang tải Quality Workspace...</div>; }
function ErrorState({ onRetry }: { onRetry: () => void }) { return <div className="flex items-center justify-between rounded-md border border-red-200 bg-red-50 p-4 text-sm text-red-700"><span>Không tải được Quality Workspace.</span><button className="font-semibold underline" onClick={onRetry}>Thử lại</button></div>; }
