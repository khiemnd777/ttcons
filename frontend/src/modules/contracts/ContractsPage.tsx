import { useMemo, useState } from 'react';
import type { ReactNode } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useParams } from 'react-router-dom';
import clsx from 'clsx';
import {
  ArrowLeft,
  CheckCircle2,
  FilePlus2,
  GitPullRequestArrow,
  Landmark,
  Loader2,
  Plus,
  Search,
  Send,
  Stamp,
} from 'lucide-react';
import { CurrencyText } from '../../components/CurrencyText';
import { Dialog } from '../../components/Dialog';
import { EmptyState } from '../../components/EmptyState';
import { PageHeader } from '../../components/PageHeader';
import { ProjectBackLink } from '../../components/ProjectBackLink';
import { StatusBadge } from '../../components/StatusBadge';
import { useToast } from '../../components/ToastProvider';
import { get, post } from '../../lib/api/client';
import { useAuth } from '../../lib/auth/AuthContext';
import type {
  Contract,
  ContractMilestone,
  ContractRetention,
  ContractVariation,
  ContractVersion,
  Partner,
} from '../../lib/types/domain';
import { currency, dateText } from '../../lib/utils/format';
import { statusLabel } from '../../lib/utils/status';
import { SimpleForm } from '../_shared/SimpleForm';
import { partnerOptions } from '../core-data/partnerUtils';

type CreateDialog = 'contract' | 'version' | 'milestone' | 'variation' | null;
type DetailTab = 'versions' | 'milestones' | 'variations' | 'retentions';
type DirectionFilter = 'all' | 'receivable' | 'payable';
type ActionDialog = {
  title: string;
  description: string;
  endpoint: string;
  success: string;
  commentRequired?: boolean;
} | null;
type RetentionDialog = { id: string; outstanding: number; milestone: string } | null;
type ContractPermissions = {
  canCreate: boolean;
  canReview: boolean;
  canApprove: boolean;
  canSign: boolean;
  canManageMilestone: boolean;
  canReleaseRetention: boolean;
};

export function ContractsPage() {
  const { id = '' } = useParams();
  const { hasAnyPermission } = useAuth();
  const toast = useToast();
  const queryClient = useQueryClient();
  const [selectedId, setSelectedId] = useState('');
  const [mobileDetailOpen, setMobileDetailOpen] = useState(false);
  const [activeTab, setActiveTab] = useState<DetailTab>('milestones');
  const [query, setQuery] = useState('');
  const [directionFilter, setDirectionFilter] = useState<DirectionFilter>('all');
  const [statusFilter, setStatusFilter] = useState('all');
  const [createDialog, setCreateDialog] = useState<CreateDialog>(null);
  const [actionDialog, setActionDialog] = useState<ActionDialog>(null);
  const [retentionDialog, setRetentionDialog] = useState<RetentionDialog>(null);

  const contracts = useQuery({
    queryKey: ['contracts', id],
    queryFn: () => get<Contract[]>(`/projects/${id}/contracts`),
    enabled: Boolean(id),
  });
  const detail = useQuery({
    queryKey: ['contract-detail', selectedId],
    queryFn: () => get<Contract>(`/contracts/${selectedId}`),
    enabled: Boolean(selectedId),
  });
  const partners = useQuery({
    queryKey: ['partners', 'contract', 'active'],
    queryFn: () => get<Partner[]>('/partners?status=active'),
  });
  const acceptances = useQuery({
    queryKey: ['acceptances', id, 'contract'],
    queryFn: () => get<Record<string, any>[]>(`/projects/${id}/acceptances`),
    enabled: Boolean(id) && hasAnyPermission(['acceptance.view']),
  });
  const selected = detail.data;

  const permissions = {
    canCreate: hasAnyPermission(['contract.create']),
    canReview: hasAnyPermission(['contract.review']),
    canApprove: hasAnyPermission(['contract.approve']),
    canSign: hasAnyPermission(['contract.sign']),
    canManageMilestone: hasAnyPermission(['contract.milestone_manage']),
    canReleaseRetention: hasAnyPermission(['contract.retention_release']),
  };

  const invalidate = async (contractId?: string) => {
    await queryClient.invalidateQueries({ queryKey: ['contracts', id] });
    if (contractId || selectedId) {
      await queryClient.invalidateQueries({ queryKey: ['contract-detail', contractId || selectedId] });
    }
  };

  const create = useMutation({
    mutationFn: ({ path, values }: { path: string; values: Record<string, any> }) => post<Contract>(path, values),
    onSuccess: async (data, variables) => {
      setSelectedId(data.id);
      setMobileDetailOpen(true);
      if (variables.path.endsWith('/versions')) setActiveTab('versions');
      if (variables.path.endsWith('/milestones')) setActiveTab('milestones');
      if (variables.path.endsWith('/variations')) setActiveTab('variations');
      await invalidate(data.id);
      setCreateDialog(null);
      toast.success('Đã lưu hồ sơ hợp đồng');
    },
    onError: (error) => toast.error('Không lưu được hồ sơ hợp đồng', error),
  });
  const action = useMutation({
    mutationFn: ({ endpoint, body }: { endpoint: string; body?: Record<string, any> }) => post<Contract>(endpoint, body ?? {}),
    onSuccess: async () => {
      await invalidate();
      toast.success(actionDialog?.success ?? 'Đã cập nhật hợp đồng');
      setActionDialog(null);
    },
    onError: (error) => toast.error('Không thực hiện được thao tác hợp đồng', error),
  });
  const release = useMutation({
    mutationFn: (values: Record<string, any>) => post<Contract>(`/contract-retentions/${retentionDialog?.id}/release`, values),
    onSuccess: async () => {
      await invalidate();
      setRetentionDialog(null);
      toast.success('Đã mở công nợ giải phóng khoản giữ lại');
    },
    onError: (error) => toast.error('Không giải phóng được khoản giữ lại', error),
  });

  const summary = useMemo(() => ({
    active: (contracts.data ?? []).filter((item) => item.status === 'active').length,
    value: (contracts.data ?? [])
      .filter((item) => item.status === 'active')
      .reduce((sum, item) => sum + Number(item.current_amount ?? 0), 0),
    retention: (contracts.data ?? []).reduce((sum, item) => sum + Number(item.retention_outstanding ?? 0), 0),
  }), [contracts.data]);

  const statusOptions = useMemo(
    () => [...new Set((contracts.data ?? []).map((item) => item.status).filter(Boolean))],
    [contracts.data],
  );
  const filteredContracts = useMemo(() => {
    const normalizedQuery = query.trim().toLocaleLowerCase('vi-VN');
    return (contracts.data ?? []).filter((contract) => {
      const matchesQuery = !normalizedQuery || [contract.contract_code, contract.title, contract.counterparty_name]
        .some((value) => String(value ?? '').toLocaleLowerCase('vi-VN').includes(normalizedQuery));
      const matchesDirection = directionFilter === 'all' || contract.direction === directionFilter;
      const matchesStatus = statusFilter === 'all' || contract.status === statusFilter;
      return matchesQuery && matchesDirection && matchesStatus;
    });
  }, [contracts.data, directionFilter, query, statusFilter]);

  function openAction(value: NonNullable<ActionDialog>) {
    setActionDialog(value);
  }

  function selectContract(contractId: string) {
    setSelectedId(contractId);
    setMobileDetailOpen(true);
  }

  return (
    <>
      <PageHeader
        title="Hợp đồng"
        description="Theo dõi giá trị, ký duyệt, thanh toán, phụ lục và khoản giữ lại của công trình."
        actions={(
          <>
            <ProjectBackLink projectId={id} />
            {permissions.canCreate ? (
              <button
                className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700"
                onClick={() => setCreateDialog('contract')}
              >
                <Plus className="h-4 w-4" />
                Tạo hợp đồng
              </button>
            ) : null}
          </>
        )}
      />

      <div className="mb-4 grid gap-3 sm:grid-cols-3">
        <Metric label="Hợp đồng đang hiệu lực" value={`${summary.active} hợp đồng`} />
        <Metric label="Tổng giá trị hiện hành" value={<CurrencyText value={summary.value} />} />
        <Metric label="Chưa giải phóng" value={<CurrencyText value={summary.retention} />} />
      </div>

      <div className="grid gap-4 xl:grid-cols-[320px_minmax(0,1fr)]">
        <section className={clsx('overflow-hidden rounded-lg border border-line bg-white shadow-panel', mobileDetailOpen && 'hidden xl:block')}>
          <div className="border-b border-line p-4">
            <h2 className="font-semibold text-ink">Danh sách hợp đồng</h2>
            <p className="mt-1 text-xs text-slate-500">Tìm và chọn hợp đồng cần xử lý.</p>
            <label className="relative mt-3 block">
              <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" />
              <input
                className="w-full rounded-md border border-line py-2 pl-9 pr-3 text-sm focus-ring"
                placeholder="Tìm mã, tên hoặc đối tác"
                value={query}
                onChange={(event) => setQuery(event.target.value)}
              />
            </label>
            <div className="mt-2 grid grid-cols-2 gap-2">
              <select
                className="rounded-md border border-line bg-white px-2 py-2 text-xs font-medium text-slate-700 focus-ring"
                value={directionFilter}
                onChange={(event) => setDirectionFilter(event.target.value as DirectionFilter)}
                aria-label="Lọc theo chiều công nợ"
              >
                <option value="all">Tất cả công nợ</option>
                <option value="receivable">Phải thu</option>
                <option value="payable">Phải trả</option>
              </select>
              <select
                className="rounded-md border border-line bg-white px-2 py-2 text-xs font-medium text-slate-700 focus-ring"
                value={statusFilter}
                onChange={(event) => setStatusFilter(event.target.value)}
                aria-label="Lọc theo trạng thái"
              >
                <option value="all">Tất cả trạng thái</option>
                {statusOptions.map((status) => <option key={status} value={status}>{contractStatusLabel(status)}</option>)}
              </select>
            </div>
          </div>

          {contracts.isLoading ? (
            <Loading label="Đang tải danh sách hợp đồng..." />
          ) : contracts.error ? (
            <ErrorState onRetry={() => contracts.refetch()} />
          ) : !(contracts.data?.length) ? (
            <div className="p-4">
              <EmptyState
                title="Chưa có hợp đồng trong công trình"
                action={permissions.canCreate ? <PrimaryButton label="Tạo hợp đồng" onClick={() => setCreateDialog('contract')} /> : undefined}
              />
            </div>
          ) : !filteredContracts.length ? (
            <div className="p-4"><EmptyState title="Không có hợp đồng phù hợp" /></div>
          ) : (
            <div className="divide-y divide-line">
              {filteredContracts.map((contract) => (
                <button
                  key={contract.id}
                  className={clsx(
                    'w-full border-l-2 border-transparent px-4 py-3 text-left transition hover:bg-slate-50',
                    selectedId === contract.id && 'border-l-blue-600 bg-blue-50',
                  )}
                  onClick={() => selectContract(contract.id)}
                >
                  <div className="flex items-start justify-between gap-3">
                    <div className="min-w-0">
                      <div className="truncate text-sm font-semibold text-ink">{contract.contract_code}</div>
                      <div className="mt-0.5 line-clamp-2 text-xs leading-5 text-slate-600">{contract.title}</div>
                    </div>
                    <ContractStatusBadge status={contract.status} />
                  </div>
                  <div className="mt-2 flex items-center justify-between gap-3 text-xs text-slate-500">
                    <span className="min-w-0 truncate">{contract.counterparty_name}</span>
                    <span className="shrink-0 font-semibold text-slate-700">{directionLabel(contract.direction)}</span>
                  </div>
                  <div className="mt-2 flex items-center justify-between gap-3">
                    <span className="text-sm font-semibold text-ink"><CurrencyText value={contract.current_amount} /></span>
                    <span className="max-w-[48%] truncate text-xs text-slate-500">
                      {contract.current_version_label ? `Phiên bản ${contract.current_version_label}` : 'Chưa có phiên bản'}
                    </span>
                  </div>
                </button>
              ))}
            </div>
          )}
        </section>

        <section className={clsx('min-w-0 overflow-hidden rounded-lg border border-line bg-white shadow-panel', !mobileDetailOpen && 'hidden xl:block')}>
          {!selectedId ? (
            <div className="p-4"><EmptyState title="Chọn một hợp đồng để xem chi tiết" /></div>
          ) : detail.isLoading ? (
            <Loading label="Đang tải chi tiết hợp đồng..." />
          ) : detail.error ? (
            <ErrorState onRetry={() => detail.refetch()} />
          ) : selected ? (
            <div>
              <ContractHeader
                contract={selected}
                permissions={permissions}
                onBack={() => setMobileDetailOpen(false)}
                onCreate={setCreateDialog}
                onAction={openAction}
              />
              <ContractSummary contract={selected} />
              <NextActionBand
                contract={selected}
                permissions={permissions}
                onAction={openAction}
                onRelease={setRetentionDialog}
              />
              <ContractTabs contract={selected} activeTab={activeTab} onChange={setActiveTab} />
              <div className="p-4">
                {activeTab === 'versions' ? (
                  <VersionTable
                    contract={selected}
                    canReview={permissions.canReview}
                    canApprove={permissions.canApprove}
                    canSign={permissions.canSign}
                    onAction={openAction}
                  />
                ) : null}
                {activeTab === 'milestones' ? (
                  <MilestoneTable
                    contract={selected}
                    canManage={permissions.canManageMilestone}
                    onAction={openAction}
                  />
                ) : null}
                {activeTab === 'variations' ? (
                  <VariationTable
                    contract={selected}
                    canReview={permissions.canReview}
                    canApprove={permissions.canApprove}
                    canCreate={permissions.canCreate}
                    onAction={openAction}
                  />
                ) : null}
                {activeTab === 'retentions' ? (
                  <RetentionTable
                    contract={selected}
                    canRelease={permissions.canReleaseRetention}
                    onRelease={setRetentionDialog}
                  />
                ) : null}
              </div>
            </div>
          ) : null}
        </section>
      </div>

      <CreateContractDialog
        open={createDialog === 'contract'}
        partners={partners.data ?? []}
        onClose={() => setCreateDialog(null)}
        onSubmit={(values) => create.mutateAsync({ path: `/projects/${id}/contracts`, values })}
      />
      <CreateVersionDialog
        open={createDialog === 'version'}
        contract={selected}
        onClose={() => setCreateDialog(null)}
        onSubmit={(values) => create.mutateAsync({ path: `/contracts/${selectedId}/versions`, values })}
      />
      <CreateMilestoneDialog
        open={createDialog === 'milestone'}
        contract={selected}
        acceptances={acceptances.data ?? []}
        onClose={() => setCreateDialog(null)}
        onSubmit={(values) => create.mutateAsync({ path: `/contracts/${selectedId}/milestones`, values })}
      />
      <CreateVariationDialog
        open={createDialog === 'variation'}
        onClose={() => setCreateDialog(null)}
        onSubmit={(values) => create.mutateAsync({ path: `/contracts/${selectedId}/variations`, values })}
      />
      <Dialog
        open={Boolean(actionDialog)}
        title={actionDialog?.title ?? ''}
        description={actionDialog?.description}
        onClose={() => setActionDialog(null)}
      >
        <SimpleForm
          submitLabel="Xác nhận"
          cancelLabel="Đóng"
          fields={[{
            name: 'comment',
            label: 'Nhận xét',
            type: 'textarea',
            defaultValue: '',
            required: actionDialog?.commentRequired ? 'Vui lòng nhập nhận xét' : false,
          }]}
          onCancel={() => setActionDialog(null)}
          onSubmit={(values) => actionDialog && action.mutateAsync({ endpoint: actionDialog.endpoint, body: values })}
        />
      </Dialog>
      <Dialog
        open={Boolean(retentionDialog)}
        title="Giải phóng khoản giữ lại"
        description={retentionDialog ? `${retentionDialog.milestone} · còn ${currency(retentionDialog.outstanding)}` : undefined}
        onClose={() => setRetentionDialog(null)}
      >
        <SimpleForm
          submitLabel="Mở công nợ giải phóng"
          cancelLabel="Đóng"
          fields={[
            { name: 'amount', label: 'Số tiền giải phóng', type: 'number', defaultValue: retentionDialog?.outstanding ?? 0, required: true, min: 0 },
            { name: 'due_date', label: 'Ngày đến hạn', type: 'date', defaultValue: new Date().toISOString().slice(0, 10), required: true },
            { name: 'comment', label: 'Căn cứ giải phóng', type: 'textarea', defaultValue: '', required: true },
          ]}
          onCancel={() => setRetentionDialog(null)}
          onSubmit={(values) => release.mutateAsync(values)}
        />
      </Dialog>
    </>
  );
}

function ContractHeader({
  contract,
  permissions,
  onBack,
  onCreate,
  onAction,
}: {
  contract: Contract;
  permissions: ContractPermissions;
  onBack: () => void;
  onCreate: (dialog: CreateDialog) => void;
  onAction: (value: NonNullable<ActionDialog>) => void;
}) {
  return (
    <div className="border-b border-line p-4">
      <button className="mb-3 inline-flex items-center gap-2 text-sm font-semibold text-blue-700 xl:hidden" onClick={onBack}>
        <ArrowLeft className="h-4 w-4" />
        Về danh sách hợp đồng
      </button>
      <div className="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <h2 className="text-lg font-semibold text-ink">{contract.contract_code} · {contract.title}</h2>
            <ContractStatusBadge status={contract.status} />
          </div>
          <p className="mt-1 text-sm text-slate-600">
            {contract.counterparty_name} · {contractTypeLabel(contract.contract_type)} · {directionLabel(contract.direction)}
          </p>
          <p className="mt-1 text-xs text-slate-500">
            {currencyLabel(contract.currency)}
            {contract.effective_date ? ` · Hiệu lực từ ${dateText(contract.effective_date)}` : ''}
            {contract.expiry_date ? ` · Hết hạn ${dateText(contract.expiry_date)}` : ''}
          </p>
        </div>
        <div className="flex flex-wrap gap-2">
          {permissions.canCreate && !['active', 'completed', 'terminated'].includes(contract.status) ? (
            <SmallButton label="Tạo phiên bản" icon={<FilePlus2 className="h-4 w-4" />} onClick={() => onCreate('version')} />
          ) : null}
          {permissions.canManageMilestone && ['signed', 'active'].includes(contract.status) ? (
            <SmallButton label="Thêm mốc thanh toán" icon={<Landmark className="h-4 w-4" />} onClick={() => onCreate('milestone')} />
          ) : null}
          {permissions.canCreate && contract.status === 'active' ? (
            <PrimaryButton label="Tạo phụ lục" icon={<GitPullRequestArrow className="h-4 w-4" />} onClick={() => onCreate('variation')} />
          ) : null}
          {permissions.canSign && contract.status === 'signed' ? (
            <PrimaryButton
              label="Kích hoạt hợp đồng"
              onClick={() => onAction({
                title: 'Kích hoạt hợp đồng',
                description: 'Phiên bản đã ký sẽ trở thành nguồn giá trị hiện hành; các mốc thanh toán nháp được kích hoạt.',
                endpoint: `/contracts/${contract.id}/activate`,
                success: 'Đã kích hoạt hợp đồng',
              })}
            />
          ) : null}
        </div>
      </div>
    </div>
  );
}

function ContractSummary({ contract }: { contract: Contract }) {
  const adjustment = Number(contract.current_amount ?? 0) - Number(contract.original_amount ?? 0);
  return (
    <div className="border-b border-line bg-slate-50 p-4">
      <div className="grid gap-3 sm:grid-cols-3">
        <Metric label="Giá trị gốc" value={<CurrencyText value={contract.original_amount} />} />
        <Metric label="Giá trị điều chỉnh" value={<SignedCurrency value={adjustment} />} />
        <Metric label="Giá trị hiện hành" value={<CurrencyText value={contract.current_amount} />} />
      </div>
      <div className="mt-3 flex flex-wrap gap-x-6 gap-y-2 text-xs font-medium text-slate-600">
        <span>Tạm ứng: <strong className="text-ink">{contract.advance_percent}%</strong></span>
        <span>Giữ lại: <strong className="text-ink">{contract.retention_percent}%</strong></span>
        <span>Bảo hành: <strong className="text-ink">{contract.warranty_days || 0} ngày</strong></span>
      </div>
    </div>
  );
}

function NextActionBand({
  contract,
  permissions,
  onAction,
  onRelease,
}: {
  contract: Contract;
  permissions: ContractPermissions;
  onAction: (value: NonNullable<ActionDialog>) => void;
  onRelease: (value: RetentionDialog) => void;
}) {
  const draftVersion = contract.versions?.find((row) => row.status === 'draft' && permissions.canReview);
  const reviewVersion = contract.versions?.find((row) => row.status === 'review' && permissions.canApprove);
  const approvedVersion = contract.versions?.find((row) => row.status === 'approved' && permissions.canSign);
  const activeMilestone = contract.milestones?.find((row) => row.status === 'active' && permissions.canManageMilestone);
  const eligibleMilestone = contract.milestones?.find((row) => row.status === 'eligible' && permissions.canManageMilestone);
  const draftVariation = contract.variations?.find((row) => row.status === 'draft' && permissions.canReview);
  const reviewVariation = contract.variations?.find((row) => row.status === 'review' && permissions.canApprove);
  const approvedVariation = contract.variations?.find((row) => row.status === 'approved' && permissions.canCreate);
  const outstandingRetention = contract.retentions?.find((row) => Number(row.outstanding_amount) > 0 && permissions.canReleaseRetention);

  let message = 'Không có việc nào thuộc quyền hiện tại đang chờ xử lý.';
  let actionNode: ReactNode = null;

  if (draftVersion) {
    message = `Phiên bản ${draftVersion.version_label} đang chờ gửi rà soát.`;
    actionNode = <PrimaryButton label="Gửi rà soát" onClick={() => onAction(versionSubmitAction(draftVersion))} />;
  } else if (reviewVersion) {
    message = `Phiên bản ${reviewVersion.version_label} đang chờ duyệt.`;
    actionNode = <PrimaryButton label="Duyệt phiên bản" onClick={() => onAction(versionApproveAction(reviewVersion))} />;
  } else if (approvedVersion) {
    message = `Phiên bản ${approvedVersion.version_label} đã duyệt và đang chờ ký.`;
    actionNode = <PrimaryButton label="Ký phiên bản" onClick={() => onAction(versionSignAction(approvedVersion))} />;
  } else if (contract.status === 'signed' && permissions.canSign) {
    message = 'Hợp đồng đã ký và đang chờ kích hoạt.';
    actionNode = (
      <PrimaryButton
        label="Kích hoạt hợp đồng"
        onClick={() => onAction({
          title: 'Kích hoạt hợp đồng',
          description: 'Phiên bản đã ký sẽ trở thành nguồn giá trị hiện hành; các mốc thanh toán nháp được kích hoạt.',
          endpoint: `/contracts/${contract.id}/activate`,
          success: 'Đã kích hoạt hợp đồng',
        })}
      />
    );
  } else if (activeMilestone) {
    message = `Mốc ${activeMilestone.milestone_name} đang chờ xác nhận đủ điều kiện.`;
    actionNode = <PrimaryButton label="Xác nhận đủ điều kiện" onClick={() => onAction(milestoneEligibleAction(activeMilestone))} />;
  } else if (eligibleMilestone) {
    message = `Mốc ${eligibleMilestone.milestone_name} đã đủ điều kiện mở công nợ.`;
    actionNode = <PrimaryButton label="Mở công nợ" onClick={() => onAction(milestoneObligationAction(eligibleMilestone))} />;
  } else if (draftVariation) {
    message = `Phụ lục ${draftVariation.variation_code} đang chờ gửi rà soát.`;
    actionNode = <PrimaryButton label="Gửi rà soát" onClick={() => onAction(variationSubmitAction(draftVariation))} />;
  } else if (reviewVariation) {
    message = `Phụ lục ${reviewVariation.variation_code} đang chờ duyệt.`;
    actionNode = <PrimaryButton label="Duyệt phụ lục" onClick={() => onAction(variationApproveAction(reviewVariation))} />;
  } else if (approvedVariation) {
    message = `Phụ lục ${approvedVariation.variation_code} đã duyệt và đang chờ tạo phiên bản điều chỉnh.`;
    actionNode = <PrimaryButton label="Tạo phiên bản điều chỉnh" onClick={() => onAction(variationIncorporateAction(approvedVariation))} />;
  } else if (outstandingRetention) {
    message = `Khoản giữ lại của mốc ${outstandingRetention.milestone_name} còn ${currency(outstandingRetention.outstanding_amount)}.`;
    actionNode = (
      <PrimaryButton
        label="Giải phóng khoản giữ lại"
        onClick={() => onRelease({
          id: outstandingRetention.id,
          outstanding: Number(outstandingRetention.outstanding_amount),
          milestone: outstandingRetention.milestone_name,
        })}
      />
    );
  }

  return (
    <div className="border-b border-blue-100 bg-blue-50 px-4 py-3">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <p className="text-xs font-semibold uppercase tracking-wide text-blue-700">Việc cần xử lý</p>
          <p className="mt-1 text-sm font-semibold text-blue-950">{message}</p>
        </div>
        {actionNode}
      </div>
    </div>
  );
}

function ContractTabs({ contract, activeTab, onChange }: { contract: Contract; activeTab: DetailTab; onChange: (tab: DetailTab) => void }) {
  const tabs: Array<{ id: DetailTab; label: string; count: number }> = [
    { id: 'versions', label: 'Phiên bản', count: contract.versions?.length ?? 0 },
    { id: 'milestones', label: 'Mốc thanh toán', count: contract.milestones?.length ?? 0 },
    { id: 'variations', label: 'Phụ lục', count: contract.variations?.length ?? 0 },
    { id: 'retentions', label: 'Khoản giữ lại', count: contract.retentions?.length ?? 0 },
  ];
  return (
    <div className="overflow-x-auto border-b border-line bg-white px-2" role="tablist" aria-label="Hồ sơ hợp đồng">
      <div className="flex min-w-max gap-1">
        {tabs.map((tab) => (
          <button
            key={tab.id}
            type="button"
            role="tab"
            aria-selected={activeTab === tab.id}
            className={clsx(
              'inline-flex items-center gap-2 border-b-2 px-3 py-3 text-sm font-semibold transition',
              activeTab === tab.id
                ? 'border-blue-600 text-blue-700'
                : 'border-transparent text-slate-600 hover:border-slate-300 hover:text-ink',
            )}
            onClick={() => onChange(tab.id)}
          >
            {tab.label}
            <span className={clsx(
              'rounded-full px-2 py-0.5 text-[11px]',
              activeTab === tab.id ? 'bg-blue-100 text-blue-700' : 'bg-slate-100 text-slate-600',
            )}
            >
              {tab.count}
            </span>
          </button>
        ))}
      </div>
    </div>
  );
}

function VersionTable({ contract, canReview, canApprove, canSign, onAction }: {
  contract: Contract;
  canReview: boolean;
  canApprove: boolean;
  canSign: boolean;
  onAction: (value: NonNullable<ActionDialog>) => void;
}) {
  const rows = contract.versions ?? [];
  if (!rows.length) return <EmptyState title="Chưa có phiên bản hợp đồng" />;
  const rowAction = (row: ContractVersion) => {
    if (canReview && row.status === 'draft') return <SmallButton label="Gửi rà soát" icon={<Send className="h-4 w-4" />} onClick={() => onAction(versionSubmitAction(row))} />;
    if (canApprove && row.status === 'review') return <SmallButton label="Duyệt" icon={<CheckCircle2 className="h-4 w-4" />} onClick={() => onAction(versionApproveAction(row))} />;
    if (canSign && row.status === 'approved') return <SmallButton label="Ký" icon={<Stamp className="h-4 w-4" />} onClick={() => onAction(versionSignAction(row))} />;
    return null;
  };
  return (
    <ResponsiveRecords
      headers={['Phiên bản', 'Giá trị', 'Phạm vi', 'Mã kiểm chứng', 'Trạng thái', 'Thao tác']}
      rows={rows.map((row) => [
        <span className="font-semibold">V{row.version_no} · {row.version_label}</span>,
        <CurrencyText value={row.amount} />,
        row.scope_summary,
        <span className="font-mono text-xs">{row.definition_hash?.slice(0, 12)}…</span>,
        <StatusBadge status={row.status} />,
        rowAction(row),
      ])}
      cards={rows.map((row) => (
        <RecordCard key={row.id} title={`V${row.version_no} · ${row.version_label}`} status={row.status} action={rowAction(row)}>
          <RecordValue label="Giá trị"><CurrencyText value={row.amount} /></RecordValue>
          <RecordValue label="Phạm vi" full>{row.scope_summary || 'Chưa có mô tả'}</RecordValue>
          <RecordValue label="Mã kiểm chứng"><span className="font-mono text-xs">{row.definition_hash?.slice(0, 12)}…</span></RecordValue>
        </RecordCard>
      ))}
    />
  );
}

function MilestoneTable({ contract, canManage, onAction }: {
  contract: Contract;
  canManage: boolean;
  onAction: (value: NonNullable<ActionDialog>) => void;
}) {
  const rows = contract.milestones ?? [];
  if (!rows.length) return <EmptyState title="Chưa có mốc thanh toán" />;
  const rowAction = (row: ContractMilestone) => {
    if (canManage && row.status === 'active') return <SmallButton label="Đủ điều kiện" onClick={() => onAction(milestoneEligibleAction(row))} />;
    if (canManage && row.status === 'eligible') return <SmallButton label="Mở công nợ" onClick={() => onAction(milestoneObligationAction(row))} />;
    return null;
  };
  return (
    <ResponsiveRecords
      headers={['Mốc', 'Trước giữ lại', 'Giữ lại', 'Sau giữ lại', 'Điều kiện', 'Trạng thái', 'Công nợ']}
      rows={rows.map((row) => [
        <div>
          <div className="font-semibold">{row.milestone_code} · {row.milestone_name}</div>
          <div className="mt-1 text-xs text-slate-500">{milestoneTypeLabel(row.milestone_type)} · {dateText(row.planned_date)}</div>
        </div>,
        <CurrencyText value={row.gross_amount} />,
        <CurrencyText value={row.retention_amount} />,
        <CurrencyText value={row.net_amount} />,
        acceptanceLabel(row),
        <StatusBadge status={row.status} />,
        rowAction(row) ?? obligationLabel(row),
      ])}
      cards={rows.map((row) => (
        <RecordCard
          key={row.id}
          title={`${row.milestone_code} · ${row.milestone_name}`}
          subtitle={`${milestoneTypeLabel(row.milestone_type)} · ${dateText(row.planned_date)}`}
          status={row.status}
          action={rowAction(row)}
        >
          <RecordValue label="Trước giữ lại"><CurrencyText value={row.gross_amount} /></RecordValue>
          <RecordValue label="Giữ lại"><CurrencyText value={row.retention_amount} /></RecordValue>
          <RecordValue label="Sau giữ lại"><CurrencyText value={row.net_amount} /></RecordValue>
          <RecordValue label="Điều kiện">{acceptanceLabel(row)}</RecordValue>
          <RecordValue label="Công nợ" full>{obligationLabel(row) || 'Chưa mở công nợ'}</RecordValue>
        </RecordCard>
      ))}
    />
  );
}

function VariationTable({ contract, canReview, canApprove, canCreate, onAction }: {
  contract: Contract;
  canReview: boolean;
  canApprove: boolean;
  canCreate: boolean;
  onAction: (value: NonNullable<ActionDialog>) => void;
}) {
  const rows = contract.variations ?? [];
  if (!rows.length) return <EmptyState title="Chưa có phụ lục hợp đồng" />;
  const rowAction = (row: ContractVariation) => {
    if (canReview && row.status === 'draft') return <SmallButton label="Gửi rà soát" onClick={() => onAction(variationSubmitAction(row))} />;
    if (canApprove && row.status === 'review') return <SmallButton label="Duyệt" onClick={() => onAction(variationApproveAction(row))} />;
    if (canCreate && row.status === 'approved') return <SmallButton label="Tạo phiên bản điều chỉnh" onClick={() => onAction(variationIncorporateAction(row))} />;
    return null;
  };
  return (
    <ResponsiveRecords
      headers={['Mã', 'Nội dung', 'Thay đổi giá trị', 'Thay đổi ngày', 'Trạng thái', 'Thao tác']}
      rows={rows.map((row) => [
        <span className="font-semibold">{row.variation_code}</span>,
        <div><div>{row.title}</div><div className="mt-1 text-xs text-slate-500">{row.reason}</div></div>,
        <SignedCurrency value={row.amount_delta} />,
        signedDays(row.days_delta),
        <StatusBadge status={row.status} />,
        rowAction(row),
      ])}
      cards={rows.map((row) => (
        <RecordCard key={row.id} title={`${row.variation_code} · ${row.title}`} status={row.status} action={rowAction(row)}>
          <RecordValue label="Lý do" full>{row.reason || 'Chưa có lý do'}</RecordValue>
          <RecordValue label="Thay đổi giá trị"><SignedCurrency value={row.amount_delta} /></RecordValue>
          <RecordValue label="Thay đổi thời gian">{signedDays(row.days_delta)}</RecordValue>
          {row.scope_delta ? <RecordValue label="Thay đổi phạm vi" full>{row.scope_delta}</RecordValue> : null}
        </RecordCard>
      ))}
    />
  );
}

function RetentionTable({ contract, canRelease, onRelease }: {
  contract: Contract;
  canRelease: boolean;
  onRelease: (value: RetentionDialog) => void;
}) {
  const rows = contract.retentions ?? [];
  if (!rows.length) return <EmptyState title="Chưa phát sinh khoản giữ lại" />;
  const rowAction = (row: ContractRetention) => canRelease && Number(row.outstanding_amount) > 0 ? (
    <SmallButton
      label="Giải phóng"
      onClick={() => onRelease({ id: row.id, outstanding: Number(row.outstanding_amount), milestone: row.milestone_name })}
    />
  ) : null;
  return (
    <ResponsiveRecords
      headers={['Nguồn', 'Đã giữ', 'Đã giải phóng', 'Còn lại', 'Hết bảo hành', 'Trạng thái', 'Thao tác']}
      rows={rows.map((row) => [
        <span className="font-semibold">{row.milestone_code} · {row.milestone_name}</span>,
        <CurrencyText value={row.held_amount} />,
        <CurrencyText value={row.released_amount} />,
        <CurrencyText value={row.outstanding_amount} />,
        row.warranty_end_date ? dateText(row.warranty_end_date) : 'Theo xác nhận',
        <StatusBadge status={row.status} />,
        rowAction(row),
      ])}
      cards={rows.map((row) => (
        <RecordCard key={row.id} title={`${row.milestone_code} · ${row.milestone_name}`} status={row.status} action={rowAction(row)}>
          <RecordValue label="Đã giữ"><CurrencyText value={row.held_amount} /></RecordValue>
          <RecordValue label="Đã giải phóng"><CurrencyText value={row.released_amount} /></RecordValue>
          <RecordValue label="Còn lại"><CurrencyText value={row.outstanding_amount} /></RecordValue>
          <RecordValue label="Hết bảo hành">{row.warranty_end_date ? dateText(row.warranty_end_date) : 'Theo xác nhận'}</RecordValue>
        </RecordCard>
      ))}
    />
  );
}

function ResponsiveRecords({ headers, rows, cards }: { headers: string[]; rows: ReactNode[][]; cards: ReactNode[] }) {
  return (
    <>
      <div className="hidden overflow-x-auto rounded-md border border-line 2xl:block">
        <table className="w-full text-sm">
          <thead className="bg-slate-50 text-left text-xs uppercase tracking-wide text-slate-500">
            <tr>{headers.map((header) => <th key={header} className="px-3 py-2">{header}</th>)}</tr>
          </thead>
          <tbody className="divide-y divide-line">
            {rows.map((cells, rowIndex) => (
              <tr key={rowIndex} className="hover:bg-slate-50">
                {cells.map((cell, cellIndex) => <td key={cellIndex} className="max-w-xs px-3 py-2 align-top text-slate-700">{cell}</td>)}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <div className="grid gap-3 2xl:hidden">{cards}</div>
    </>
  );
}

function RecordCard({ title, subtitle, status, action, children }: {
  title: string;
  subtitle?: string;
  status: string;
  action?: ReactNode;
  children: ReactNode;
}) {
  return (
    <article className="rounded-lg border border-line bg-white p-4">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <h4 className="font-semibold text-ink">{title}</h4>
          {subtitle ? <p className="mt-1 text-xs text-slate-500">{subtitle}</p> : null}
        </div>
        <StatusBadge status={status} />
      </div>
      <div className="mt-3 grid gap-3 sm:grid-cols-2">{children}</div>
      {action ? <div className="mt-3 flex justify-end border-t border-line pt-3">{action}</div> : null}
    </article>
  );
}

function RecordValue({ label, full = false, children }: { label: string; full?: boolean; children: ReactNode }) {
  return (
    <div className={clsx(full && 'sm:col-span-2')}>
      <p className="text-[11px] font-semibold uppercase tracking-wide text-slate-500">{label}</p>
      <div className="mt-1 text-sm text-slate-700">{children}</div>
    </div>
  );
}

function CreateContractDialog({ open, partners, onClose, onSubmit }: {
  open: boolean;
  partners: Partner[];
  onClose: () => void;
  onSubmit: (values: Record<string, any>) => Promise<any>;
}) {
  return (
    <Dialog open={open} title="Tạo hợp đồng" description="Hợp đồng mới ở trạng thái nháp; cần tạo phiên bản trước khi rà soát và ký." onClose={onClose} size="lg">
      <SimpleForm
        layout="two-column"
        submitLabel="Tạo hợp đồng"
        onCancel={onClose}
        fields={[
          { name: 'contract_code', label: 'Mã hợp đồng', defaultValue: `HD-${Date.now().toString().slice(-6)}`, required: true },
          { name: 'title', label: 'Tên hợp đồng', defaultValue: 'Hợp đồng thi công', required: true },
          { name: 'contract_type', label: 'Loại hợp đồng', type: 'select', defaultValue: 'main_client', required: true, options: contractTypeOptions },
          { name: 'direction', label: 'Chiều công nợ', type: 'select', defaultValue: 'receivable', required: true, options: directionOptions },
          { name: 'counterparty_partner_id', label: 'Đối tác', type: 'select', defaultValue: '', required: true, options: partnerOptions(partners, ['client', 'supplier', 'subcontractor', 'labor_team', 'service_provider', 'other']) },
          { name: 'currency', label: 'Đơn vị tiền tệ', defaultValue: 'VND', required: true },
          { name: 'original_amount', label: 'Giá trị gốc', type: 'number', defaultValue: 0, required: true, min: 0 },
          { name: 'advance_percent', label: 'Tỷ lệ tạm ứng (%)', type: 'number', defaultValue: 0, min: 0, max: 100 },
          { name: 'retention_percent', label: 'Tỷ lệ giữ lại (%)', type: 'number', defaultValue: 0, min: 0, max: 100 },
          { name: 'warranty_days', label: 'Thời hạn bảo hành (ngày)', type: 'number', defaultValue: 0, min: 0 },
          { name: 'effective_date', label: 'Ngày hiệu lực', type: 'date', defaultValue: '' },
          { name: 'expiry_date', label: 'Ngày hết hạn', type: 'date', defaultValue: '' },
        ]}
        onSubmit={onSubmit}
      />
    </Dialog>
  );
}

function CreateVersionDialog({ open, contract, onClose, onSubmit }: {
  open: boolean;
  contract?: Contract;
  onClose: () => void;
  onSubmit: (values: Record<string, any>) => Promise<any>;
}) {
  return (
    <Dialog open={open} title="Tạo phiên bản hợp đồng" description="Phiên bản sau khi ký sẽ không thể thay đổi." onClose={onClose} size="lg">
      <SimpleForm
        layout="two-column"
        submitLabel="Tạo phiên bản nháp"
        onCancel={onClose}
        fields={[
          { name: 'version_label', label: 'Tên phiên bản', defaultValue: `V${(contract?.versions?.length ?? 0) + 1}`, required: true },
          { name: 'amount', label: 'Giá trị phiên bản', type: 'number', defaultValue: contract?.current_amount ?? contract?.original_amount ?? 0, required: true, min: 0 },
          { name: 'scope_summary', label: 'Phạm vi công việc', type: 'textarea', defaultValue: '', required: true, fullWidth: true },
          { name: 'planned_start_date', label: 'Ngày bắt đầu dự kiến', type: 'date', defaultValue: '' },
          { name: 'planned_end_date', label: 'Ngày kết thúc dự kiến', type: 'date', defaultValue: '' },
          { name: 'comment', label: 'Ghi chú phiên bản', type: 'textarea', defaultValue: '', fullWidth: true },
        ]}
        onSubmit={(values) => onSubmit({ ...values, commercial_terms: {}, payment_terms: {} })}
      />
    </Dialog>
  );
}

function CreateMilestoneDialog({ open, contract, acceptances, onClose, onSubmit }: {
  open: boolean;
  contract?: Contract;
  acceptances: Record<string, any>[];
  onClose: () => void;
  onSubmit: (values: Record<string, any>) => Promise<any>;
}) {
  return (
    <Dialog open={open} title="Tạo mốc thanh toán" description="Tỷ lệ thanh toán được tính trên giá trị phiên bản hiện hành." onClose={onClose} size="lg">
      <SimpleForm
        layout="two-column"
        submitLabel="Tạo mốc thanh toán"
        onCancel={onClose}
        fields={[
          { name: 'milestone_code', label: 'Mã mốc', defaultValue: `M${String((contract?.milestones?.length ?? 0) + 1).padStart(2, '0')}`, required: true },
          { name: 'milestone_name', label: 'Tên mốc thanh toán', defaultValue: 'Thanh toán theo tiến độ', required: true },
          { name: 'sequence_no', label: 'Thứ tự', type: 'number', defaultValue: (contract?.milestones?.length ?? 0) + 1, required: true, min: 1 },
          { name: 'milestone_type', label: 'Loại mốc', type: 'select', defaultValue: 'progress', required: true, options: milestoneTypeOptions },
          { name: 'amount_basis', label: 'Cơ sở giá trị', type: 'select', defaultValue: 'percentage', required: true, options: amountBasisOptions },
          { name: 'percentage', label: 'Tỷ lệ (%)', type: 'number', defaultValue: 10, min: 0, max: 100 },
          { name: 'gross_amount', label: 'Giá trị cố định', type: 'number', defaultValue: 0, min: 0 },
          { name: 'retention_percent', label: 'Tỷ lệ giữ lại (%)', type: 'number', defaultValue: contract?.retention_percent ?? 0, min: 0, max: 100 },
          { name: 'planned_date', label: 'Ngày kế hoạch', type: 'date', defaultValue: '' },
          { name: 'due_days', label: 'Số ngày đến hạn', type: 'number', defaultValue: 7, min: 0 },
          { name: 'acceptance_required', label: 'Bắt buộc nghiệm thu', type: 'checkbox', defaultValue: false },
          {
            name: 'acceptance_id',
            label: 'Hồ sơ nghiệm thu liên kết',
            type: 'select',
            defaultValue: '',
            options: acceptances
              .filter((row) => row.status === 'approved' && ['pass', 'conditional_pass'].includes(row.result))
              .map((row) => ({ value: row.id, label: `${row.title} · ${statusLabel(row.result)}` })),
            helperText: 'Bắt buộc khi mốc thanh toán yêu cầu nghiệm thu.',
          },
        ]}
        onSubmit={(values) => onSubmit({ ...values, acceptance_id: values.acceptance_id || null })}
      />
    </Dialog>
  );
}

function CreateVariationDialog({ open, onClose, onSubmit }: {
  open: boolean;
  onClose: () => void;
  onSubmit: (values: Record<string, any>) => Promise<any>;
}) {
  return (
    <Dialog open={open} title="Tạo phụ lục hợp đồng" description="Phụ lục đã duyệt chỉ tạo phiên bản nháp mới, không tự thay đổi hợp đồng đang hiệu lực." onClose={onClose} size="lg">
      <SimpleForm
        layout="two-column"
        submitLabel="Tạo phụ lục"
        onCancel={onClose}
        fields={[
          { name: 'variation_code', label: 'Mã phụ lục', defaultValue: `PL-${Date.now().toString().slice(-5)}`, required: true },
          { name: 'title', label: 'Tên phụ lục', defaultValue: 'Điều chỉnh phạm vi hợp đồng', required: true },
          { name: 'variation_type', label: 'Loại điều chỉnh', type: 'select', defaultValue: 'mixed', required: true, options: variationTypeOptions },
          { name: 'reason', label: 'Lý do', type: 'textarea', defaultValue: '', required: true, fullWidth: true },
          { name: 'amount_delta', label: 'Thay đổi giá trị (+/-)', type: 'number', defaultValue: 0 },
          { name: 'days_delta', label: 'Thay đổi số ngày (+/-)', type: 'number', defaultValue: 0 },
          { name: 'scope_delta', label: 'Thay đổi phạm vi', type: 'textarea', defaultValue: '', fullWidth: true },
        ]}
        onSubmit={onSubmit}
      />
    </Dialog>
  );
}

function Metric({ label, value }: { label: string; value: ReactNode }) {
  return (
    <div className="rounded-md border border-line bg-white px-3 py-2">
      <div className="text-xs font-semibold uppercase tracking-wide text-slate-500">{label}</div>
      <div className="mt-1 font-semibold text-ink">{value}</div>
    </div>
  );
}

function ContractStatusBadge({ status }: { status?: string }) {
  return (
    <span className={clsx(
      'inline-flex shrink-0 items-center rounded-md px-2 py-1 text-xs font-semibold ring-1 ring-inset',
      status === 'active'
        ? 'bg-emerald-50 text-emerald-700 ring-emerald-200'
        : 'bg-slate-100 text-slate-700 ring-slate-200',
    )}
    >
      {contractStatusLabel(status)}
    </span>
  );
}

function PrimaryButton({ label, icon, onClick }: { label: string; icon?: ReactNode; onClick: () => void }) {
  return (
    <button className="inline-flex items-center justify-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700" onClick={onClick}>
      {icon}{label}
    </button>
  );
}

function SmallButton({ label, icon, onClick }: { label: string; icon?: ReactNode; onClick: () => void }) {
  return (
    <button className="inline-flex items-center gap-1 rounded-md border border-line bg-white px-2 py-1.5 text-xs font-semibold text-slate-700 hover:bg-slate-100" onClick={onClick}>
      {icon}{label}
    </button>
  );
}

function Loading({ label }: { label: string }) {
  return <div className="flex items-center gap-2 p-6 text-sm text-slate-500"><Loader2 className="h-4 w-4 animate-spin" /> {label}</div>;
}

function ErrorState({ onRetry }: { onRetry: () => void }) {
  return (
    <div className="m-4 flex flex-col gap-3 rounded-md border border-red-200 bg-red-50 p-3 text-sm text-red-700 sm:flex-row sm:items-center sm:justify-between">
      <span>Không tải được dữ liệu hợp đồng.</span>
      <button className="font-semibold underline" onClick={onRetry}>Thử lại</button>
    </div>
  );
}

function SignedCurrency({ value }: { value: number }) {
  const numericValue = Number(value ?? 0);
  return <span className={numericValue > 0 ? 'text-emerald-700' : numericValue < 0 ? 'text-rose-700' : undefined}>{numericValue > 0 ? '+' : ''}{currency(numericValue)}</span>;
}

function versionSubmitAction(row: ContractVersion): NonNullable<ActionDialog> {
  return { title: 'Gửi rà soát phiên bản', description: row.version_label, endpoint: `/contract-versions/${row.id}/submit`, success: 'Đã gửi rà soát phiên bản' };
}

function versionApproveAction(row: ContractVersion): NonNullable<ActionDialog> {
  return { title: 'Duyệt phiên bản hợp đồng', description: row.version_label, endpoint: `/contract-versions/${row.id}/approve`, success: 'Đã duyệt phiên bản', commentRequired: true };
}

function versionSignAction(row: ContractVersion): NonNullable<ActionDialog> {
  return { title: 'Xác nhận ký phiên bản', description: row.version_label, endpoint: `/contract-versions/${row.id}/sign`, success: 'Đã xác nhận ký phiên bản', commentRequired: true };
}

function milestoneEligibleAction(row: ContractMilestone): NonNullable<ActionDialog> {
  return { title: 'Xác nhận mốc đủ điều kiện', description: row.milestone_name, endpoint: `/contract-milestones/${row.id}/eligible`, success: 'Đã xác nhận mốc đủ điều kiện' };
}

function milestoneObligationAction(row: ContractMilestone): NonNullable<ActionDialog> {
  return { title: 'Mở công nợ từ mốc thanh toán', description: `${row.milestone_name} · sau giữ lại ${currency(row.net_amount)}`, endpoint: `/contract-milestones/${row.id}/open-obligation`, success: 'Đã mở công nợ từ mốc thanh toán' };
}

function variationSubmitAction(row: ContractVariation): NonNullable<ActionDialog> {
  return { title: 'Gửi rà soát phụ lục', description: row.title, endpoint: `/contract-variations/${row.id}/submit`, success: 'Đã gửi rà soát phụ lục' };
}

function variationApproveAction(row: ContractVariation): NonNullable<ActionDialog> {
  return { title: 'Duyệt phụ lục', description: row.title, endpoint: `/contract-variations/${row.id}/approve`, success: 'Đã duyệt phụ lục', commentRequired: true };
}

function variationIncorporateAction(row: ContractVariation): NonNullable<ActionDialog> {
  return {
    title: 'Tạo phiên bản điều chỉnh từ phụ lục',
    description: 'Tạo phiên bản nháp mới; vẫn phải rà soát, duyệt và ký.',
    endpoint: `/contract-variations/${row.id}/incorporate`,
    success: 'Đã tạo phiên bản điều chỉnh từ phụ lục',
  };
}

function acceptanceLabel(row: ContractMilestone) {
  if (!row.acceptance_required) return 'Không yêu cầu';
  return row.acceptance_id ? 'Có nghiệm thu liên kết' : 'Thiếu hồ sơ nghiệm thu';
}

function obligationLabel(row: ContractMilestone) {
  if (!row.obligation_id) return null;
  return `${directionLabel(row.obligation_type)} · Đã mở công nợ`;
}

function contractStatusLabel(status?: string) {
  if (status === 'active') return 'Đang hiệu lực';
  if (status === 'terminated') return 'Đã chấm dứt';
  return statusLabel(status);
}

function contractTypeLabel(type?: string) {
  return contractTypeOptions.find((option) => option.value === type)?.label ?? 'Loại hợp đồng khác';
}

function directionLabel(direction?: string) {
  if (direction === 'receivable') return 'Phải thu';
  if (direction === 'payable') return 'Phải trả';
  return 'Chưa xác định';
}

function currencyLabel(value?: string) {
  return value === 'VND' ? 'Việt Nam đồng' : value || 'Chưa xác định tiền tệ';
}

function milestoneTypeLabel(type?: string) {
  return milestoneTypeOptions.find((option) => option.value === type)?.label ?? 'Loại mốc khác';
}

function signedDays(value: number) {
  const days = Number(value ?? 0);
  return `${days > 0 ? '+' : ''}${days} ngày`;
}

const contractTypeOptions = [
  { value: 'main_client', label: 'Hợp đồng chủ đầu tư' },
  { value: 'subcontractor', label: 'Hợp đồng thầu phụ' },
  { value: 'supplier', label: 'Hợp đồng nhà cung cấp' },
  { value: 'service', label: 'Hợp đồng dịch vụ' },
  { value: 'other', label: 'Hợp đồng khác' },
];

const directionOptions = [
  { value: 'receivable', label: 'Phải thu · Đầu ra' },
  { value: 'payable', label: 'Phải trả · Đầu vào' },
];

const milestoneTypeOptions = [
  { value: 'advance', label: 'Tạm ứng' },
  { value: 'progress', label: 'Thanh toán tiến độ' },
  { value: 'final', label: 'Quyết toán' },
  { value: 'retention_release', label: 'Giải phóng khoản giữ lại' },
  { value: 'other', label: 'Loại khác' },
];

const amountBasisOptions = [
  { value: 'percentage', label: 'Tỷ lệ hợp đồng' },
  { value: 'fixed', label: 'Giá trị cố định' },
  { value: 'quantity_certificate', label: 'Chứng nhận khối lượng' },
];

const variationTypeOptions = [
  { value: 'scope', label: 'Phạm vi' },
  { value: 'price', label: 'Giá trị' },
  { value: 'time', label: 'Tiến độ' },
  { value: 'mixed', label: 'Kết hợp' },
];
