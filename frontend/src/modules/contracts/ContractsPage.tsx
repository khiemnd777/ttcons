import { useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useParams } from 'react-router-dom';
import { CheckCircle2, FilePlus2, GitPullRequestArrow, Landmark, Loader2, Plus, Send, Stamp } from 'lucide-react';
import { PageHeader } from '../../components/PageHeader';
import { ProjectBackLink } from '../../components/ProjectBackLink';
import { Dialog } from '../../components/Dialog';
import { StatusBadge } from '../../components/StatusBadge';
import { CurrencyText } from '../../components/CurrencyText';
import { useToast } from '../../components/ToastProvider';
import { get, post } from '../../lib/api/client';
import { useAuth } from '../../lib/auth/AuthContext';
import type { Contract, Partner } from '../../lib/types/domain';
import { SimpleForm } from '../_shared/SimpleForm';
import { partnerOptions } from '../core-data/partnerUtils';

type CreateDialog = 'contract' | 'version' | 'milestone' | 'variation' | null;
type ActionDialog = { title: string; description: string; endpoint: string; success: string; commentRequired?: boolean } | null;
type RetentionDialog = { id: string; outstanding: number; milestone: string } | null;

export function ContractsPage() {
  const { id = '' } = useParams();
  const { hasAnyPermission } = useAuth();
  const toast = useToast();
  const queryClient = useQueryClient();
  const [selectedId, setSelectedId] = useState('');
  const [createDialog, setCreateDialog] = useState<CreateDialog>(null);
  const [actionDialog, setActionDialog] = useState<ActionDialog>(null);
  const [actionComment, setActionComment] = useState('');
  const [retentionDialog, setRetentionDialog] = useState<RetentionDialog>(null);

  const contracts = useQuery({ queryKey: ['contracts', id], queryFn: () => get<Contract[]>(`/projects/${id}/contracts`), enabled: Boolean(id) });
  const detail = useQuery({ queryKey: ['contract-detail', selectedId], queryFn: () => get<Contract>(`/contracts/${selectedId}`), enabled: Boolean(selectedId) });
  const partners = useQuery({ queryKey: ['partners', 'contract', 'active'], queryFn: () => get<Partner[]>('/partners?status=active') });
  const acceptances = useQuery({ queryKey: ['acceptances', id, 'contract'], queryFn: () => get<Record<string, any>[]>(`/projects/${id}/acceptances`), enabled: Boolean(id) && hasAnyPermission(['acceptance.view']) });
  const selected = detail.data;

  const invalidate = async (contractId?: string) => {
    await queryClient.invalidateQueries({ queryKey: ['contracts', id] });
    if (contractId || selectedId) await queryClient.invalidateQueries({ queryKey: ['contract-detail', contractId || selectedId] });
  };
  const create = useMutation({
    mutationFn: ({ path, values }: { path: string; values: Record<string, any> }) => post<Contract>(path, values),
    onSuccess: async (data) => {
      setSelectedId(data.id);
      await invalidate(data.id);
      setCreateDialog(null);
      toast.success('Đã lưu hồ sơ hợp đồng');
    },
    onError: (error) => toast.error('Không lưu được hồ sơ hợp đồng', error),
  });
  const action = useMutation({
    mutationFn: ({ endpoint, body }: { endpoint: string; body?: Record<string, any> }) => post<Contract>(endpoint, body ?? {}),
    onSuccess: async (_data, variables) => {
      await invalidate();
      toast.success(actionDialog?.success ?? (variables.endpoint.includes('open-obligation') ? 'Đã mở công nợ từ milestone' : 'Đã cập nhật hợp đồng'));
      setActionDialog(null);
      setActionComment('');
    },
    onError: (error) => toast.error('Không thực hiện được thao tác hợp đồng', error),
  });
  const release = useMutation({
    mutationFn: (values: Record<string, any>) => post<Contract>(`/contract-retentions/${retentionDialog?.id}/release`, values),
    onSuccess: async () => {
      await invalidate();
      setRetentionDialog(null);
      toast.success('Đã mở công nợ giải phóng giữ lại');
    },
    onError: (error) => toast.error('Không giải phóng được khoản giữ lại', error),
  });

  const summary = useMemo(() => ({
    active: (contracts.data ?? []).filter((item) => item.status === 'active').length,
    value: (contracts.data ?? []).filter((item) => item.status === 'active').reduce((sum, item) => sum + Number(item.current_amount ?? 0), 0),
    retention: (contracts.data ?? []).reduce((sum, item) => sum + Number(item.retention_outstanding ?? 0), 0),
  }), [contracts.data]);

  function openAction(value: NonNullable<ActionDialog>) {
    setActionComment('');
    setActionDialog(value);
  }

  return <>
    <PageHeader title="Hợp đồng" description="Quản lý version ký duyệt, milestone thanh toán, phụ lục/variation và khoản giữ lại theo hợp đồng." actions={<><ProjectBackLink projectId={id} />{hasAnyPermission(['contract.create']) ? <button className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700" onClick={() => setCreateDialog('contract')}><Plus className="h-4 w-4" /> Tạo hợp đồng</button> : null}</>} />
    <div className="mb-4 grid gap-3 sm:grid-cols-3"><Metric label="Hợp đồng đang hiệu lực" value={summary.active} /><Metric label="Giá trị hợp đồng active" value={<CurrencyText value={summary.value} />} /><Metric label="Giữ lại chưa giải phóng" value={<CurrencyText value={summary.retention} />} /></div>
    <div className="grid gap-4 xl:grid-cols-[340px_minmax(0,1fr)]">
      <section className="overflow-hidden rounded-lg border border-line bg-white">
        <div className="border-b border-line px-4 py-3"><h2 className="font-semibold text-ink">Danh sách hợp đồng</h2><p className="mt-1 text-xs text-slate-500">Chọn hợp đồng để xử lý version và nghĩa vụ.</p></div>
        {contracts.isLoading ? <Loading /> : contracts.error ? <ErrorState onRetry={() => contracts.refetch()} /> : !(contracts.data?.length) ? <Empty text="Chưa có hợp đồng trong công trình." /> : <div className="divide-y divide-line">{contracts.data.map((contract) => <button key={contract.id} className={`w-full px-4 py-3 text-left hover:bg-slate-50 ${selectedId === contract.id ? 'bg-blue-50' : ''}`} onClick={() => setSelectedId(contract.id)}><div className="flex items-start justify-between gap-3"><div className="min-w-0"><div className="truncate font-semibold text-ink">{contract.contract_code} · {contract.title}</div><div className="mt-1 truncate text-xs text-slate-500">{contract.counterparty_name} · {contract.direction === 'receivable' ? 'Đầu ra' : 'Đầu vào'}</div></div><StatusBadge status={contract.status} /></div><div className="mt-2 flex items-center justify-between text-sm"><CurrencyText value={contract.current_amount} /><span className="text-xs text-slate-500">{contract.current_version_label || 'Chưa có version'}</span></div></button>)}</div>}
      </section>

      <section className="min-w-0 overflow-hidden rounded-lg border border-line bg-white">
        {!selectedId ? <Empty text="Chọn một hợp đồng để mở workspace." /> : detail.isLoading ? <Loading /> : detail.error ? <ErrorState onRetry={() => detail.refetch()} /> : selected ? <div>
          <div className="flex flex-col gap-3 border-b border-line px-4 py-4 lg:flex-row lg:items-start lg:justify-between"><div><div className="flex flex-wrap items-center gap-2"><h2 className="text-lg font-semibold text-ink">{selected.contract_code} · {selected.title}</h2><StatusBadge status={selected.status} /></div><p className="mt-1 text-sm text-slate-600">{selected.counterparty_name} · {selected.contract_type} · {selected.currency}</p></div><div className="flex flex-wrap gap-2">{hasAnyPermission(['contract.create']) && !['active', 'completed', 'terminated'].includes(selected.status) ? <SmallButton label="Tạo version" icon={<FilePlus2 className="h-4 w-4" />} onClick={() => setCreateDialog('version')} /> : null}{hasAnyPermission(['contract.milestone_manage']) && ['signed', 'active'].includes(selected.status) ? <SmallButton label="Milestone" icon={<Landmark className="h-4 w-4" />} onClick={() => setCreateDialog('milestone')} /> : null}{hasAnyPermission(['contract.create']) && selected.status === 'active' ? <SmallButton label="Variation" icon={<GitPullRequestArrow className="h-4 w-4" />} onClick={() => setCreateDialog('variation')} /> : null}{hasAnyPermission(['contract.sign']) && selected.status === 'signed' ? <button className="rounded-md bg-emerald-600 px-3 py-2 text-sm font-semibold text-white" onClick={() => openAction({ title: 'Kích hoạt hợp đồng', description: 'Version đã ký sẽ trở thành nguồn giá trị hiện hành; các milestone nháp được kích hoạt.', endpoint: `/contracts/${selected.id}/activate`, success: 'Đã kích hoạt hợp đồng' })}>Kích hoạt</button> : null}</div></div>
          <div className="grid gap-3 border-b border-line bg-slate-50 px-4 py-3 sm:grid-cols-4"><Metric label="Giá trị gốc" value={<CurrencyText value={selected.original_amount} />} /><Metric label="Giá trị hiện hành" value={<CurrencyText value={selected.current_amount} />} /><Metric label="Tạm ứng" value={`${selected.advance_percent}%`} /><Metric label="Giữ lại" value={`${selected.retention_percent}%`} /></div>
          <div className="space-y-6 p-4">
            <WorkspaceSection title="Version & ký duyệt"><VersionTable contract={selected} canReview={hasAnyPermission(['contract.review'])} canApprove={hasAnyPermission(['contract.approve'])} canSign={hasAnyPermission(['contract.sign'])} onAction={openAction} /></WorkspaceSection>
            <WorkspaceSection title="Milestone thanh toán"><MilestoneTable contract={selected} canManage={hasAnyPermission(['contract.milestone_manage'])} onAction={openAction} /></WorkspaceSection>
            <WorkspaceSection title="Variation / Phụ lục"><VariationTable contract={selected} canReview={hasAnyPermission(['contract.review'])} canApprove={hasAnyPermission(['contract.approve'])} canCreate={hasAnyPermission(['contract.create'])} onAction={openAction} /></WorkspaceSection>
            <WorkspaceSection title="Retention ledger"><RetentionTable contract={selected} canRelease={hasAnyPermission(['contract.retention_release'])} onRelease={setRetentionDialog} /></WorkspaceSection>
          </div>
        </div> : null}
      </section>
    </div>

    <CreateContractDialog open={createDialog === 'contract'} partners={partners.data ?? []} pending={create.isPending} onClose={() => setCreateDialog(null)} onSubmit={(values) => create.mutateAsync({ path: `/projects/${id}/contracts`, values })} />
    <CreateVersionDialog open={createDialog === 'version'} contract={selected} pending={create.isPending} onClose={() => setCreateDialog(null)} onSubmit={(values) => create.mutateAsync({ path: `/contracts/${selectedId}/versions`, values })} />
    <CreateMilestoneDialog open={createDialog === 'milestone'} contract={selected} acceptances={acceptances.data ?? []} pending={create.isPending} onClose={() => setCreateDialog(null)} onSubmit={(values) => create.mutateAsync({ path: `/contracts/${selectedId}/milestones`, values })} />
    <CreateVariationDialog open={createDialog === 'variation'} pending={create.isPending} onClose={() => setCreateDialog(null)} onSubmit={(values) => create.mutateAsync({ path: `/contracts/${selectedId}/variations`, values })} />
    <Dialog open={Boolean(actionDialog)} title={actionDialog?.title ?? ''} description={actionDialog?.description} onClose={() => setActionDialog(null)} footer={<div className="flex justify-end gap-2"><button className="rounded-md border border-line px-3 py-2 text-sm font-semibold" onClick={() => setActionDialog(null)}>Đóng</button><button className="rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white disabled:opacity-50" disabled={action.isPending || Boolean(actionDialog?.commentRequired && !actionComment.trim())} onClick={() => actionDialog && action.mutate({ endpoint: actionDialog.endpoint, body: { comment: actionComment } })}>Xác nhận</button></div>}><label className="block"><span className="text-sm font-semibold text-slate-700">Nhận xét {actionDialog?.commentRequired ? '*' : ''}</span><textarea className="mt-1 w-full rounded-md border border-line px-3 py-2 text-sm" rows={4} value={actionComment} onChange={(event) => setActionComment(event.target.value)} /></label></Dialog>
    <Dialog open={Boolean(retentionDialog)} title="Giải phóng khoản giữ lại" description={retentionDialog ? `${retentionDialog.milestone} · còn ${retentionDialog.outstanding.toLocaleString('vi-VN')} VND` : undefined} onClose={() => setRetentionDialog(null)}><SimpleForm submitLabel="Mở công nợ giải phóng" fields={[{ name: 'amount', label: 'Số tiền giải phóng', type: 'number', defaultValue: retentionDialog?.outstanding ?? 0, required: true, min: 0 }, { name: 'due_date', label: 'Ngày đến hạn', type: 'date', defaultValue: new Date().toISOString().slice(0, 10), required: true }, { name: 'comment', label: 'Căn cứ giải phóng', type: 'textarea', defaultValue: '', required: true }]} onSubmit={(values) => release.mutateAsync(values)} /></Dialog>
  </>;
}

function VersionTable({ contract, canReview, canApprove, canSign, onAction }: { contract: Contract; canReview: boolean; canApprove: boolean; canSign: boolean; onAction: (value: NonNullable<ActionDialog>) => void }) {
  const rows = contract.versions ?? [];
  if (!rows.length) return <Empty text="Chưa có version hợp đồng." />;
  return <Table headers={['Version', 'Giá trị', 'Phạm vi', 'Hash', 'Trạng thái', 'Thao tác']} rows={rows.map((row) => [<div className="font-semibold">V{row.version_no} · {row.version_label}</div>, <CurrencyText value={row.amount} />, row.scope_summary, <span className="font-mono text-xs">{row.definition_hash?.slice(0, 12)}…</span>, <StatusBadge status={row.status} />, <div className="flex gap-1">{canReview && row.status === 'draft' ? <SmallButton label="Gửi review" icon={<Send className="h-4 w-4" />} onClick={() => onAction({ title: 'Gửi review version', description: row.version_label, endpoint: `/contract-versions/${row.id}/submit`, success: 'Đã gửi review version' })} /> : null}{canApprove && row.status === 'review' ? <SmallButton label="Duyệt" icon={<CheckCircle2 className="h-4 w-4" />} onClick={() => onAction({ title: 'Duyệt version hợp đồng', description: row.version_label, endpoint: `/contract-versions/${row.id}/approve`, success: 'Đã duyệt version', commentRequired: true })} /> : null}{canSign && row.status === 'approved' ? <SmallButton label="Ký" icon={<Stamp className="h-4 w-4" />} onClick={() => onAction({ title: 'Xác nhận ký version', description: row.version_label, endpoint: `/contract-versions/${row.id}/sign`, success: 'Đã xác nhận ký version', commentRequired: true })} /> : null}</div>])} />;
}

function MilestoneTable({ contract, canManage, onAction }: { contract: Contract; canManage: boolean; onAction: (value: NonNullable<ActionDialog>) => void }) {
  const rows = contract.milestones ?? [];
  if (!rows.length) return <Empty text="Chưa có milestone thanh toán." />;
  return <Table headers={['Mốc', 'Gross', 'Giữ lại', 'Net', 'Điều kiện', 'Trạng thái', 'Công nợ']} rows={rows.map((row) => [<div><div className="font-semibold">{row.milestone_code} · {row.milestone_name}</div><div className="text-xs text-slate-500">{row.milestone_type} · {row.planned_date?.slice?.(0, 10) || 'chưa có ngày'}</div></div>, <CurrencyText value={row.gross_amount} />, <CurrencyText value={row.retention_amount} />, <CurrencyText value={row.net_amount} />, row.acceptance_required ? (row.acceptance_id ? 'Có nghiệm thu liên kết' : 'Thiếu nghiệm thu') : 'Không yêu cầu', <StatusBadge status={row.status} />, <div className="flex gap-1">{canManage && row.status === 'active' ? <SmallButton label="Đủ điều kiện" onClick={() => onAction({ title: 'Xác nhận milestone đủ điều kiện', description: row.milestone_name, endpoint: `/contract-milestones/${row.id}/eligible`, success: 'Đã xác nhận milestone đủ điều kiện' })} /> : null}{canManage && row.status === 'eligible' ? <SmallButton label="Mở AP/AR" onClick={() => onAction({ title: 'Mở công nợ từ milestone', description: `${row.milestone_name} · net ${row.net_amount.toLocaleString('vi-VN')}`, endpoint: `/contract-milestones/${row.id}/open-obligation`, success: 'Đã mở công nợ từ milestone' })} /> : row.obligation_id ? <span className="text-xs font-semibold text-emerald-700">{row.obligation_type} · {row.obligation_id.slice(0, 8)}</span> : null}</div>])} />;
}

function VariationTable({ contract, canReview, canApprove, canCreate, onAction }: { contract: Contract; canReview: boolean; canApprove: boolean; canCreate: boolean; onAction: (value: NonNullable<ActionDialog>) => void }) {
  const rows = contract.variations ?? [];
  if (!rows.length) return <Empty text="Chưa có phụ lục/variation." />;
  return <Table headers={['Mã', 'Nội dung', 'Giá trị ±', 'Ngày ±', 'Trạng thái', 'Thao tác']} rows={rows.map((row) => [<span className="font-semibold">{row.variation_code}</span>, <div><div>{row.title}</div><div className="text-xs text-slate-500">{row.reason}</div></div>, <CurrencyText value={row.amount_delta} />, row.days_delta, <StatusBadge status={row.status} />, <div className="flex gap-1">{canReview && row.status === 'draft' ? <SmallButton label="Gửi" onClick={() => onAction({ title: 'Gửi review variation', description: row.title, endpoint: `/contract-variations/${row.id}/submit`, success: 'Đã gửi review variation' })} /> : null}{canApprove && row.status === 'review' ? <SmallButton label="Duyệt" onClick={() => onAction({ title: 'Duyệt variation', description: row.title, endpoint: `/contract-variations/${row.id}/approve`, success: 'Đã duyệt variation', commentRequired: true })} /> : null}{canCreate && row.status === 'approved' ? <SmallButton label="Tạo revision" onClick={() => onAction({ title: 'Tạo revision từ variation', description: 'Tạo version nháp mới; vẫn phải review, duyệt và ký.', endpoint: `/contract-variations/${row.id}/incorporate`, success: 'Đã tạo revision từ variation' })} /> : null}</div>])} />;
}

function RetentionTable({ contract, canRelease, onRelease }: { contract: Contract; canRelease: boolean; onRelease: (value: RetentionDialog) => void }) {
  const rows = contract.retentions ?? [];
  if (!rows.length) return <Empty text="Chưa phát sinh khoản giữ lại từ milestone." />;
  return <Table headers={['Nguồn', 'Đã giữ', 'Đã giải phóng', 'Còn lại', 'Hết bảo hành', 'Trạng thái', 'Thao tác']} rows={rows.map((row) => [<span className="font-semibold">{row.milestone_code} · {row.milestone_name}</span>, <CurrencyText value={row.held_amount} />, <CurrencyText value={row.released_amount} />, <CurrencyText value={row.outstanding_amount} />, row.warranty_end_date?.slice?.(0, 10) || 'Theo xác nhận', <StatusBadge status={row.status} />, canRelease && Number(row.outstanding_amount) > 0 ? <SmallButton label="Giải phóng" onClick={() => onRelease({ id: row.id, outstanding: Number(row.outstanding_amount), milestone: row.milestone_name })} /> : null])} />;
}

function CreateContractDialog({ open, partners, onClose, onSubmit }: { open: boolean; partners: Partner[]; pending: boolean; onClose: () => void; onSubmit: (values: Record<string, any>) => Promise<any> }) { return <Dialog open={open} title="Tạo hợp đồng" description="Hợp đồng mới ở trạng thái nháp; cần tạo version trước khi review và ký." onClose={onClose} size="lg"><SimpleForm submitLabel="Tạo hợp đồng" fields={[{ name: 'contract_code', label: 'Mã hợp đồng', defaultValue: `HD-${Date.now().toString().slice(-6)}`, required: true }, { name: 'title', label: 'Tên hợp đồng', defaultValue: 'Hợp đồng thi công', required: true }, { name: 'contract_type', label: 'Loại', type: 'select', defaultValue: 'main_client', required: true, options: [{ value: 'main_client', label: 'Hợp đồng Chủ đầu tư' }, { value: 'subcontractor', label: 'Hợp đồng thầu phụ' }, { value: 'supplier', label: 'Hợp đồng nhà cung cấp' }, { value: 'service', label: 'Hợp đồng dịch vụ' }, { value: 'other', label: 'Khác' }] }, { name: 'direction', label: 'Chiều công nợ', type: 'select', defaultValue: 'receivable', required: true, options: [{ value: 'receivable', label: 'Phải thu / Đầu ra' }, { value: 'payable', label: 'Phải trả / Đầu vào' }] }, { name: 'counterparty_partner_id', label: 'Đối tác', type: 'select', defaultValue: '', required: true, options: partnerOptions(partners, ['client', 'supplier', 'subcontractor', 'labor_team', 'service_provider', 'other']) }, { name: 'currency', label: 'Tiền tệ', defaultValue: 'VND', required: true }, { name: 'original_amount', label: 'Giá trị gốc', type: 'number', defaultValue: 0, required: true, min: 0 }, { name: 'advance_percent', label: 'Tạm ứng %', type: 'number', defaultValue: 0, min: 0, max: 100 }, { name: 'retention_percent', label: 'Giữ lại %', type: 'number', defaultValue: 0, min: 0, max: 100 }, { name: 'warranty_days', label: 'Bảo hành (ngày)', type: 'number', defaultValue: 0, min: 0 }, { name: 'effective_date', label: 'Ngày hiệu lực', type: 'date', defaultValue: '' }, { name: 'expiry_date', label: 'Ngày hết hạn', type: 'date', defaultValue: '' }]} onSubmit={onSubmit} /></Dialog>; }
function CreateVersionDialog({ open, contract, onClose, onSubmit }: { open: boolean; contract?: Contract; pending: boolean; onClose: () => void; onSubmit: (values: Record<string, any>) => Promise<any> }) { return <Dialog open={open} title="Tạo version hợp đồng" description="Version sau khi ký sẽ bất biến." onClose={onClose} size="lg"><SimpleForm submitLabel="Tạo version nháp" fields={[{ name: 'version_label', label: 'Tên version', defaultValue: `V${(contract?.versions?.length ?? 0) + 1}`, required: true }, { name: 'amount', label: 'Giá trị version', type: 'number', defaultValue: contract?.current_amount ?? contract?.original_amount ?? 0, required: true, min: 0 }, { name: 'scope_summary', label: 'Phạm vi công việc', type: 'textarea', defaultValue: '', required: true, fullWidth: true }, { name: 'planned_start_date', label: 'Bắt đầu dự kiến', type: 'date', defaultValue: '' }, { name: 'planned_end_date', label: 'Kết thúc dự kiến', type: 'date', defaultValue: '' }, { name: 'comment', label: 'Ghi chú version', type: 'textarea', defaultValue: '', fullWidth: true }]} onSubmit={(values) => onSubmit({ ...values, commercial_terms: {}, payment_terms: {} })} /></Dialog>; }
function CreateMilestoneDialog({ open, contract, acceptances, onClose, onSubmit }: { open: boolean; contract?: Contract; acceptances: Record<string, any>[]; pending: boolean; onClose: () => void; onSubmit: (values: Record<string, any>) => Promise<any> }) { return <Dialog open={open} title="Tạo milestone thanh toán" description="Milestone percentage được tính trên giá trị version hiện hành." onClose={onClose} size="lg"><SimpleForm submitLabel="Tạo milestone" fields={[{ name: 'milestone_code', label: 'Mã mốc', defaultValue: `M${String((contract?.milestones?.length ?? 0) + 1).padStart(2, '0')}`, required: true }, { name: 'milestone_name', label: 'Tên milestone', defaultValue: 'Thanh toán theo tiến độ', required: true }, { name: 'sequence_no', label: 'Thứ tự', type: 'number', defaultValue: (contract?.milestones?.length ?? 0) + 1, required: true, min: 1 }, { name: 'milestone_type', label: 'Loại', type: 'select', defaultValue: 'progress', required: true, options: [{ value: 'advance', label: 'Tạm ứng' }, { value: 'progress', label: 'Tiến độ' }, { value: 'final', label: 'Quyết toán' }, { value: 'retention_release', label: 'Giải phóng giữ lại' }, { value: 'other', label: 'Khác' }] }, { name: 'amount_basis', label: 'Cơ sở giá trị', type: 'select', defaultValue: 'percentage', required: true, options: [{ value: 'percentage', label: 'Tỷ lệ hợp đồng' }, { value: 'fixed', label: 'Giá trị cố định' }, { value: 'quantity_certificate', label: 'Chứng nhận khối lượng' }] }, { name: 'percentage', label: 'Tỷ lệ %', type: 'number', defaultValue: 10, min: 0, max: 100 }, { name: 'gross_amount', label: 'Giá trị cố định', type: 'number', defaultValue: 0, min: 0 }, { name: 'retention_percent', label: 'Giữ lại %', type: 'number', defaultValue: contract?.retention_percent ?? 0, min: 0, max: 100 }, { name: 'planned_date', label: 'Ngày kế hoạch', type: 'date', defaultValue: '' }, { name: 'due_days', label: 'Số ngày đến hạn', type: 'number', defaultValue: 7, min: 0 }, { name: 'acceptance_required', label: 'Bắt buộc nghiệm thu', type: 'checkbox', defaultValue: false }, { name: 'acceptance_id', label: 'Hồ sơ nghiệm thu liên kết', type: 'select', defaultValue: '', options: acceptances.filter((row) => row.status === 'approved' && ['pass', 'conditional_pass'].includes(row.result)).map((row) => ({ value: row.id, label: `${row.title} · ${row.result}` })), helperText: 'Bắt buộc khi milestone yêu cầu nghiệm thu.' }]} onSubmit={(values) => onSubmit({ ...values, acceptance_id: values.acceptance_id || null })} /></Dialog>; }
function CreateVariationDialog({ open, onClose, onSubmit }: { open: boolean; pending: boolean; onClose: () => void; onSubmit: (values: Record<string, any>) => Promise<any> }) { return <Dialog open={open} title="Tạo phụ lục / Variation" description="Variation đã duyệt chỉ tạo version nháp mới, không tự thay đổi hợp đồng đang hiệu lực." onClose={onClose} size="lg"><SimpleForm submitLabel="Tạo variation" fields={[{ name: 'variation_code', label: 'Mã phụ lục', defaultValue: `PL-${Date.now().toString().slice(-5)}`, required: true }, { name: 'title', label: 'Tên phụ lục', defaultValue: 'Điều chỉnh phạm vi hợp đồng', required: true }, { name: 'variation_type', label: 'Loại', type: 'select', defaultValue: 'mixed', required: true, options: [{ value: 'scope', label: 'Phạm vi' }, { value: 'price', label: 'Giá trị' }, { value: 'time', label: 'Tiến độ' }, { value: 'mixed', label: 'Hỗn hợp' }] }, { name: 'reason', label: 'Lý do', type: 'textarea', defaultValue: '', required: true, fullWidth: true }, { name: 'amount_delta', label: 'Thay đổi giá trị (+/-)', type: 'number', defaultValue: 0 }, { name: 'days_delta', label: 'Thay đổi số ngày (+/-)', type: 'number', defaultValue: 0 }, { name: 'scope_delta', label: 'Thay đổi phạm vi', type: 'textarea', defaultValue: '', fullWidth: true }]} onSubmit={onSubmit} /></Dialog>; }

function WorkspaceSection({ title, children }: { title: string; children: React.ReactNode }) { return <section><h3 className="mb-2 text-sm font-semibold text-ink">{title}</h3>{children}</section>; }
function Table({ headers, rows }: { headers: string[]; rows: React.ReactNode[][] }) { return <div className="overflow-x-auto rounded-md border border-line"><table className="min-w-[900px] w-full text-sm"><thead className="bg-slate-50 text-left text-xs uppercase tracking-wide text-slate-500"><tr>{headers.map((header) => <th key={header} className="px-3 py-2">{header}</th>)}</tr></thead><tbody className="divide-y divide-line">{rows.map((cells, index) => <tr key={index}>{cells.map((cell, cellIndex) => <td key={cellIndex} className="max-w-xs px-3 py-2 align-top">{cell}</td>)}</tr>)}</tbody></table></div>; }
function Metric({ label, value }: { label: string; value: React.ReactNode }) { return <div className="rounded-md border border-line bg-white px-3 py-2"><div className="text-xs font-semibold uppercase tracking-wide text-slate-500">{label}</div><div className="mt-1 font-semibold text-ink">{value}</div></div>; }
function SmallButton({ label, icon, onClick }: { label: string; icon?: React.ReactNode; onClick: () => void }) { return <button className="inline-flex items-center gap-1 rounded-md border border-line bg-white px-2 py-1.5 text-xs font-semibold text-slate-700 hover:bg-slate-100" onClick={onClick}>{icon}{label}</button>; }
function Loading() { return <div className="flex items-center gap-2 p-6 text-sm text-slate-500"><Loader2 className="h-4 w-4 animate-spin" /> Đang tải hợp đồng...</div>; }
function Empty({ text }: { text: string }) { return <div className="m-4 rounded-md border border-dashed border-line p-5 text-sm text-slate-500">{text}</div>; }
function ErrorState({ onRetry }: { onRetry: () => void }) { return <div className="m-4 flex items-center justify-between rounded-md border border-red-200 bg-red-50 p-3 text-sm text-red-700"><span>Không tải được dữ liệu hợp đồng.</span><button className="font-semibold underline" onClick={onRetry}>Thử lại</button></div>; }
