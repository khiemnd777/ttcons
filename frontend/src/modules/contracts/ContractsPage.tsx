import { useEffect, useMemo, useRef, useState } from 'react';
import { useRecordDeepLink } from '../_shared/useRecordDeepLink';
import type { ReactNode } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useParams, useSearchParams } from 'react-router-dom';
import clsx from 'clsx';
import {
  ArrowLeft,
  CheckCircle2,
  FilePlus2,
  GitPullRequestArrow,
  Landmark,
  Loader2,
  PackageCheck,
  Paperclip,
  Plus,
  Search,
  Send,
  Stamp,
  Upload,
} from 'lucide-react';
import { CurrencyText } from '../../components/CurrencyText';
import { Dialog } from '../../components/Dialog';
import { EmptyState } from '../../components/EmptyState';
import { PageHeader } from '../../components/PageHeader';
import { ProjectBackLink } from '../../components/ProjectBackLink';
import { StatusBadge } from '../../components/StatusBadge';
import { identityReferenceText } from '../../components/IdentityReferenceLabel';
import { useToast } from '../../components/ToastProvider';
import { get, post, postForm } from '../../lib/api/client';
import { useAuth } from '../../lib/auth/AuthContext';
import type {
  Contract,
  ContractMilestone,
  ContractDispatch,
  ContractSigningForm,
  ContractVariation,
  ContractVersion,
  Partner,
  WorkflowFormDetail,
} from '../../lib/types/domain';
import { currency, dateText } from '../../lib/utils/format';
import { statusLabel } from '../../lib/utils/status';
import { useValidationNavigation } from '../../lib/utils/formValidationNavigation';
import { SimpleForm } from '../_shared/SimpleForm';
import { partnerOptions } from '../core-data/partnerUtils';
import {
  syncWorkItemMutationTransition,
  withWorkItemMutationContext,
  workItemAutoStartedNotice,
  type WorkItemMutationResult,
} from '../work-items/workItemMutationContext';
import { SignatureCaptureDialog, type SignatureCapturePayload } from '../work-items/SignatureCaptureDialog';
import {
  PriorityBadge,
  SignedCurrency,
  acceptanceLabel,
  amountBasisOptions,
  contractStatusLabel,
  contractTypeLabel,
  contractTypeOptions,
  currencyLabel,
  directionLabel,
  directionOptions,
  dispatchMethodLabel,
  dispatchMethodOptions,
  milestoneEligibleAction,
  milestoneTypeLabel,
  milestoneTypeOptions,
  readinessCount,
  signedDays,
  variationApproveAction,
  variationIncorporateAction,
  variationPriorityOptions,
  variationProcurementAction,
  variationPurchaseAction,
  variationQSAction,
  variationSubmitAction,
  variationTypeOptions,
  versionApproveAction,
  versionSubmitAction,
  type ActionDialog,
} from './contractsPresentation';

type CreateDialog = 'contract' | 'version' | 'milestone' | 'variation' | null;
type DetailTab = 'execution' | 'versions' | 'milestones' | 'variations';
type DirectionFilter = 'all' | 'receivable' | 'payable';
type VersionSignTarget = { version: ContractVersion; form: ContractSigningForm } | null;
type DispatchReceiveTarget = ContractDispatch | null;
type ContractPermissions = {
  canCreate: boolean;
  canReview: boolean;
  canApprove: boolean;
  canSign: boolean;
  canManageMilestone: boolean;
  canDispatch: boolean;
  canScan: boolean;
  canVariationQS: boolean;
  canVariationPurchase: boolean;
  canVariationProcurement: boolean;
};

export function ContractsPage() {
  const { id = '' } = useParams();
  const [searchParams, setSearchParams] = useSearchParams();
  const taskId = searchParams.get('task')?.trim() ?? '';
  const { hasAnyPermission, user } = useAuth();
  const toast = useToast();
  const queryClient = useQueryClient();
  const [selectedId, setSelectedId] = useState('');
  const sourceLink = useRecordDeepLink({ kind: 'contract', projectId: id, onOpen: record => { setSelectedId(record.id); setMobileDetailOpen(true); } });
  const [mobileDetailOpen, setMobileDetailOpen] = useState(false);
  const [activeTab, setActiveTab] = useState<DetailTab>('execution');
  const [query, setQuery] = useState('');
  const [directionFilter, setDirectionFilter] = useState<DirectionFilter>('all');
  const [statusFilter, setStatusFilter] = useState('all');
  const [createDialog, setCreateDialog] = useState<CreateDialog>(null);
  const [actionDialog, setActionDialog] = useState<ActionDialog>(null);
  const [versionSignTarget, setVersionSignTarget] = useState<VersionSignTarget>(null);
  const [dispatchDialogOpen, setDispatchDialogOpen] = useState(false);
  const [dispatchReceiveTarget, setDispatchReceiveTarget] = useState<DispatchReceiveTarget>(null);

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
    canDispatch: hasAnyPermission(['contract.dispatch']),
    canScan: hasAnyPermission(['contract.scan_manage']),
    canVariationQS: hasAnyPermission(['contract.variation_qs_review']),
    canVariationPurchase: hasAnyPermission(['contract.variation_purchase']),
    canVariationProcurement: hasAnyPermission(['procurement.create', 'contract.variation_purchase']),
  };

  useEffect(() => {
    const requestedTab = searchParams.get('tab');
    if (requestedTab && ['execution', 'versions', 'milestones', 'variations'].includes(requestedTab)) {
      setActiveTab(requestedTab as DetailTab);
    }
    if (!sourceLink.active && !selectedId && contracts.data?.length) {
      const preferred = contracts.data.find((contract) => contract.contract_type === 'main_client' && contract.direction === 'receivable') ?? contracts.data[0];
      setSelectedId(preferred.id);
      if (searchParams.get('action') === 'new-variation' && preferred.status === 'active' && permissions.canCreate) {
        setCreateDialog('variation');
        setMobileDetailOpen(true);
        setSearchParams((current) => {
          const next = new URLSearchParams(current);
          next.delete('action');
          return next;
        }, { replace: true });
      }
    }
  }, [contracts.data, permissions.canCreate, searchParams, selectedId, setSearchParams, sourceLink.active]);

  const invalidate = async (contractId?: string) => {
    await queryClient.invalidateQueries({ queryKey: ['contracts', id] });
    if (contractId || selectedId) {
      await queryClient.invalidateQueries({ queryKey: ['contract-detail', contractId || selectedId] });
    }
  };

  const create = useMutation({
    mutationFn: ({ path, values }: { path: string; values: Record<string, any> }) => post<Contract & WorkItemMutationResult>(path, withWorkItemMutationContext(values, taskId)),
    onSuccess: async (data, variables) => {
      setSelectedId(data.id);
      setMobileDetailOpen(true);
      if (variables.path.endsWith('/versions')) setActiveTab('versions');
      if (variables.path.endsWith('/milestones')) setActiveTab('milestones');
      if (variables.path.endsWith('/variations')) setActiveTab('variations');
      await invalidate(data.id);
      syncWorkItemMutationTransition(queryClient, data);
      setCreateDialog(null);
      toast.success(`Đã lưu hồ sơ hợp đồng.${workItemAutoStartedNotice(data)}`);
    },
    onError: (error) => toast.error('Không lưu được hồ sơ hợp đồng', error),
  });
  const action = useMutation({
    mutationFn: ({ endpoint, body }: { endpoint: string; body?: Record<string, any> }) => post<Contract & WorkItemMutationResult>(endpoint, withWorkItemMutationContext(body ?? {}, taskId)),
    onSuccess: async (result) => {
      await invalidate();
      syncWorkItemMutationTransition(queryClient, result);
      toast.success(`${actionDialog?.success ?? 'Đã cập nhật hợp đồng'}.${workItemAutoStartedNotice(result)}`);
      setActionDialog(null);
    },
    onError: (error) => toast.error('Không thực hiện được thao tác hợp đồng', error),
  });
  const signVersion = useMutation({
    mutationFn: async ({ target, payload }: { target: NonNullable<VersionSignTarget>; payload: SignatureCapturePayload }) => {
      const signedForm = await post<WorkflowFormDetail>(`/workflow-form-instances/${target.form.id}/sign`, payload);
      const signature = [...(signedForm.signatures ?? [])]
        .reverse()
        .find((row) => row.purpose === 'approved_by' && row.signer_user_id === user?.id);
      if (!signature) throw new Error('Không tìm thấy chữ ký approved_by vừa tạo để liên kết với phiên bản hợp đồng.');
      return post<Contract & WorkItemMutationResult>(`/contract-versions/${target.version.id}/sign`, withWorkItemMutationContext({
        workflow_form_signature_id: signature.id,
        comment: payload.comment,
      }, taskId));
    },
    onSuccess: async (result) => {
      await invalidate();
      syncWorkItemMutationTransition(queryClient, result);
      setVersionSignTarget(null);
      setActiveTab('execution');
      toast.success(`Đã ký phiên bản hợp đồng trên App.${workItemAutoStartedNotice(result)}`);
    },
    onError: (error) => toast.error('Không ký được phiên bản hợp đồng', error),
  });
  const dispatch = useMutation({
    mutationFn: (values: Record<string, any>) => post<Contract & WorkItemMutationResult>(`/contracts/${selectedId}/dispatches`, withWorkItemMutationContext(values, taskId)),
    onSuccess: async (result) => {
      await invalidate();
      syncWorkItemMutationTransition(queryClient, result);
      setDispatchDialogOpen(false);
      setActiveTab('execution');
      toast.success(`Đã ghi nhận gửi hồ sơ giấy.${workItemAutoStartedNotice(result)}`);
    },
    onError: (error) => toast.error('Không ghi nhận được việc gửi hồ sơ giấy', error),
  });
  const receiveDispatch = useMutation({
    mutationFn: async ({ target, file, note }: { target: ContractDispatch; file: File; note: string }) => {
      const form = new FormData();
      form.append('file', file);
      form.append('document_type', 'returned_signed_contract_scan');
      form.append('name', file.name);
      const document = await postForm<Record<string, any>>(`/contract-dispatches/${target.id}/documents/upload`, form);
      return post<Contract & WorkItemMutationResult>(`/contract-dispatches/${target.id}/receive`, withWorkItemMutationContext({
        returned_scan_document_id: document.id,
        note,
      }, taskId));
    },
    onSuccess: async (result) => {
      await invalidate();
      syncWorkItemMutationTransition(queryClient, result);
      setDispatchReceiveTarget(null);
      setActiveTab('execution');
      toast.success(`Đã lưu bản scan hợp đồng giấy có chữ ký.${workItemAutoStartedNotice(result)}`);
    },
    onError: (error) => toast.error('Không nhận được bản scan hợp đồng giấy', error),
  });

  const summary = useMemo(() => ({
    active: (contracts.data ?? []).filter((item) => item.status === 'active').length,
    value: (contracts.data ?? [])
      .filter((item) => item.status === 'active')
      .reduce((sum, item) => sum + Number(item.current_amount ?? 0), 0),
  }), [contracts.data]);

  const scanReminders = useMemo(() => {
    const rows = contracts.data ?? [];
    return [
      {
        label: 'hợp đồng đầu ra',
        missing: !rows.some((item) => item.direction === 'receivable' && item.signed_scan_received),
      },
      {
        label: 'hợp đồng đầu vào',
        missing: !rows.some((item) => item.direction === 'payable' && item.signed_scan_received),
      },
    ].filter((item) => item.missing);
  }, [contracts.data]);

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

  function openVersionSignature(version: ContractVersion) {
    const form = selected?.signing_forms?.find((candidate) => candidate.status === 'submitted');
    if (!form) {
      setActiveTab('execution');
      toast.warn('Chưa có biểu mẫu đã gửi tại bước “Ký hợp đồng trên App”. Hãy hoàn tất biểu mẫu workflow trước khi ký phiên bản.');
      return;
    }
    setVersionSignTarget({ version, form });
  }

  return (
    <>
      {sourceLink.feedback}
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
      </div>

      {!contracts.isLoading && scanReminders.length ? (
        <div className="mb-4 rounded-lg border border-amber-300 bg-amber-50 px-4 py-3 text-sm text-amber-950" role="status">
          <div className="flex items-start gap-3">
            <Paperclip className="mt-0.5 h-4 w-4 shrink-0" />
            <div>
              <p className="font-semibold">Hồ sơ hợp đồng của dự án chưa đủ bản scan đã ký</p>
              <p className="mt-1 text-amber-800">
                Còn thiếu {scanReminders.map((item) => item.label).join(' và ')}. Hãy vào hồ sơ tương ứng để gửi bản giấy và tải bản scan có chữ ký về hệ thống.
              </p>
            </div>
          </div>
        </div>
      ) : null}

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
                onSign={openVersionSignature}
              />
              <ContractTabs contract={selected} activeTab={activeTab} onChange={setActiveTab} />
              <div className="p-4">
                {activeTab === 'execution' ? (
                  <ContractExecutionPanel
                    contract={selected}
                    permissions={permissions}
                    onDispatch={() => setDispatchDialogOpen(true)}
                    onReceive={setDispatchReceiveTarget}
                    onActivate={() => openAction({
                      title: 'Kích hoạt hợp đồng',
                      description: 'Kích hoạt sau khi đủ chữ ký App, đã gửi giấy và đã nhận bản scan giấy có chữ ký.',
                      endpoint: `/contracts/${selected.id}/activate`,
                      success: 'Đã kích hoạt hợp đồng',
                    })}
                  />
                ) : null}
                {activeTab === 'versions' ? (
                  <VersionTable
                    contract={selected}
                    canReview={permissions.canReview}
                    canApprove={permissions.canApprove}
                    canSign={permissions.canSign}
                    onAction={openAction}
                    onSign={openVersionSignature}
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
                    canQS={permissions.canVariationQS}
                    canPurchase={permissions.canVariationPurchase}
                    canProcurement={permissions.canVariationProcurement}
                    onAction={openAction}
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
        currencyCode={selected?.currency}
        onClose={() => setCreateDialog(null)}
        onSubmit={(values) => create.mutateAsync({
          path: `/contracts/${selectedId}/variations`,
          values: { ...values, source_work_item_id: searchParams.get('source_work_item_id') || null },
        })}
      />
      <SignatureCaptureDialog
        open={Boolean(versionSignTarget)}
        signerName={user?.full_name}
        submitting={signVersion.isPending}
        allowedKinds={['signature']}
        signedPurposes={(versionSignTarget?.form.signatures ?? [])
          .filter((signature) => signature.signer_user_id === user?.id)
          .map((signature) => signature.purpose)}
        allowedPurposes={['approved_by']}
        defaultPurpose="approved_by"
        purposeLocked
        formRevision={versionSignTarget?.form.current_revision}
        onClose={() => setVersionSignTarget(null)}
        onSubmit={(payload) => versionSignTarget && signVersion.mutate({ target: versionSignTarget, payload })}
      />
      <Dialog
        open={dispatchDialogOpen}
        title="Gửi hồ sơ hợp đồng giấy"
        description="Ghi nhận lần gửi hồ sơ giấy cho chủ đầu tư sau khi phiên bản đã ký trên App."
        onClose={() => setDispatchDialogOpen(false)}
      >
        <SimpleForm
          submitLabel="Xác nhận đã gửi"
          cancelLabel="Đóng"
          fields={[
            { name: 'recipient_name', label: 'Người/đơn vị nhận', defaultValue: selected?.counterparty_name ?? '', required: true },
            { name: 'delivery_method', label: 'Hình thức gửi', type: 'select', defaultValue: 'direct', required: true, options: dispatchMethodOptions },
            { name: 'tracking_reference', label: 'Mã vận đơn/biên nhận', defaultValue: '' },
            { name: 'note', label: 'Ghi chú bàn giao', type: 'textarea', defaultValue: '' },
          ]}
          onCancel={() => setDispatchDialogOpen(false)}
          onSubmit={(values) => dispatch.mutateAsync(values)}
        />
      </Dialog>
      <ReceiveContractScanDialog
        target={dispatchReceiveTarget}
        submitting={receiveDispatch.isPending}
        onClose={() => setDispatchReceiveTarget(null)}
        onSubmit={(file, note) => dispatchReceiveTarget && receiveDispatch.mutateAsync({ target: dispatchReceiveTarget, file, note })}
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
          {permissions.canSign && contract.status === 'signed' && contract.execution_readiness?.returned_scan_received ? (
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
  onSign,
}: {
  contract: Contract;
  permissions: ContractPermissions;
  onAction: (value: NonNullable<ActionDialog>) => void;
  onSign: (version: ContractVersion) => void;
}) {
  const draftVersion = contract.versions?.find((row) => row.status === 'draft' && permissions.canReview);
  const reviewVersion = contract.versions?.find((row) => row.status === 'review' && permissions.canApprove);
  const approvedVersion = contract.versions?.find((row) => row.status === 'approved' && permissions.canSign);
  const activeMilestone = contract.milestones?.find((row) => row.status === 'active' && permissions.canManageMilestone);
  const draftVariation = contract.variations?.find((row) => row.status === 'draft' && permissions.canCreate);
  const qsVariation = contract.variations?.find((row) => row.status === 'qs_review' && permissions.canVariationQS);
  const purchaseVariation = contract.variations?.find((row) => row.status === 'purchasing_sourcing' && permissions.canVariationPurchase);
  const reviewVariation = contract.variations?.find((row) => ['pending_approval', 'review'].includes(row.status) && permissions.canApprove);
  const approvedVariation = contract.variations?.find((row) => row.status === 'approved' && permissions.canCreate);

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
    actionNode = <PrimaryButton label="Ký phiên bản" onClick={() => onSign(approvedVersion)} />;
  } else if (contract.status === 'signed' && permissions.canSign) {
    message = contract.execution_readiness?.returned_scan_received
      ? 'Hợp đồng đã có bản scan giấy ký và đang chờ kích hoạt.'
      : 'Hợp đồng đã ký trên App; cần gửi hồ sơ giấy và nhận lại bản scan có chữ ký.';
    actionNode = contract.execution_readiness?.returned_scan_received ? (
      <PrimaryButton
        label="Kích hoạt hợp đồng"
        onClick={() => onAction({
          title: 'Kích hoạt hợp đồng',
          description: 'Phiên bản đã ký và bản scan giấy sẽ trở thành căn cứ thực hiện hợp đồng.',
          endpoint: `/contracts/${contract.id}/activate`,
          success: 'Đã kích hoạt hợp đồng',
        })}
      />
    ) : null;
  } else if (activeMilestone) {
    message = `Mốc ${activeMilestone.milestone_name} đang chờ xác nhận đủ điều kiện.`;
    actionNode = <PrimaryButton label="Xác nhận đủ điều kiện" onClick={() => onAction(milestoneEligibleAction(activeMilestone))} />;
  } else if (draftVariation) {
    message = `Phát sinh ${draftVariation.variation_code} đang chờ gửi QS.`;
    actionNode = <PrimaryButton label="Gửi QS" onClick={() => onAction(variationSubmitAction(draftVariation))} />;
  } else if (qsVariation) {
    message = `Phát sinh ${qsVariation.variation_code} đang chờ QS rà soát.`;
    actionNode = <PrimaryButton label="QS xác nhận" onClick={() => onAction(variationQSAction(qsVariation))} />;
  } else if (purchaseVariation) {
    message = `Phát sinh ${purchaseVariation.variation_code} đang chờ Mua hàng lấy giá.`;
    actionNode = <PrimaryButton label="Xác nhận đã lấy giá" onClick={() => onAction(variationPurchaseAction(purchaseVariation))} />;
  } else if (reviewVariation) {
    message = `Phát sinh ${reviewVariation.variation_code} đang chờ Quản lý dự án/Giám đốc duyệt.`;
    actionNode = <PrimaryButton label="Duyệt phát sinh" onClick={() => onAction(variationApproveAction(reviewVariation))} />;
  } else if (approvedVariation) {
    message = `Phụ lục ${approvedVariation.variation_code} đã duyệt và đang chờ tạo phiên bản điều chỉnh.`;
    actionNode = <PrimaryButton label="Tạo phiên bản điều chỉnh" onClick={() => onAction(variationIncorporateAction(approvedVariation))} />;
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
    { id: 'execution', label: 'Thực hiện HĐ', count: readinessCount(contract) },
    { id: 'versions', label: 'Phiên bản', count: contract.versions?.length ?? 0 },
    { id: 'milestones', label: 'Mốc thanh toán', count: contract.milestones?.length ?? 0 },
    { id: 'variations', label: 'Phụ lục', count: contract.variations?.length ?? 0 },
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

function VersionTable({ contract, canReview, canApprove, canSign, onAction, onSign }: {
  contract: Contract;
  canReview: boolean;
  canApprove: boolean;
  canSign: boolean;
  onAction: (value: NonNullable<ActionDialog>) => void;
  onSign: (version: ContractVersion) => void;
}) {
  const rows = contract.versions ?? [];
  if (!rows.length) return <EmptyState title="Chưa có phiên bản hợp đồng" />;
  const rowAction = (row: ContractVersion) => {
    if (canReview && row.status === 'draft') return <SmallButton label="Gửi rà soát" icon={<Send className="h-4 w-4" />} onClick={() => onAction(versionSubmitAction(row))} />;
    if (canApprove && row.status === 'review') return <SmallButton label="Duyệt" icon={<CheckCircle2 className="h-4 w-4" />} onClick={() => onAction(versionApproveAction(row))} />;
    if (canSign && row.status === 'approved') return <SmallButton label="Ký trên App" icon={<Stamp className="h-4 w-4" />} onClick={() => onSign(row)} />;
    return null;
  };
  return (
    <ResponsiveRecords
      headers={['', 'Phiên bản', 'Giá trị', 'Phạm vi', 'Mã kiểm chứng', 'Trạng thái']}
      rows={rows.map((row) => [
        rowAction(row),
        <span className="font-semibold">V{row.version_no} · {row.version_label}</span>,
        <CurrencyText value={row.amount} />,
        row.scope_summary,
        <span className="font-mono text-xs">{row.definition_hash?.slice(0, 12)}…</span>,
        <StatusBadge status={row.status} />,
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

function ContractExecutionPanel({ contract, permissions, onDispatch, onReceive, onActivate }: {
  contract: Contract;
  permissions: ContractPermissions;
  onDispatch: () => void;
  onReceive: (dispatch: ContractDispatch) => void;
  onActivate: () => void;
}) {
  const readiness = contract.execution_readiness;
  const executionReady = Boolean(
    readiness?.contract_active
    && readiness.app_signed
    && readiness.paper_dispatched
    && readiness.returned_scan_received,
  );
  const submittedForm = contract.signing_forms?.find((form) => form.status === 'submitted');
  const sentDispatch = contract.dispatches?.find((dispatch) => dispatch.status === 'sent');
  const checkpoints = [
    { label: 'Ký trên App', done: Boolean(readiness?.app_signed), detail: readiness?.app_signed ? 'Đã liên kết chữ ký approved_by' : submittedForm ? 'Biểu mẫu đã sẵn sàng ký' : 'Chưa có biểu mẫu workflow đã gửi' },
    { label: 'Gửi hồ sơ giấy', done: Boolean(readiness?.paper_dispatched), detail: readiness?.paper_dispatched ? 'Đã ghi nhận bàn giao cho chủ đầu tư' : 'Chưa ghi nhận lần gửi' },
    { label: 'Scan giấy có ký', done: Boolean(readiness?.returned_scan_received), detail: readiness?.returned_scan_received ? 'Đã lưu trong kho tài liệu bảo vệ' : 'Chưa nhận lại bản scan' },
    {
      label: 'Hiệu lực thực hiện',
      done: executionReady,
      detail: executionReady
        ? 'Được phép chuyển sang triển khai'
        : readiness?.contract_active
          ? 'Hợp đồng lịch sử đang hiệu lực nhưng chưa đủ hồ sơ thực thi Rev3'
          : 'Chưa kích hoạt hợp đồng',
    },
  ];

  return (
    <div className="space-y-4">
      <div>
        <h3 className="font-semibold text-ink">Điều kiện thực hiện hợp đồng</h3>
        <p className="mt-1 text-sm text-slate-500">Các mục triển khai chỉ được mở sau khi đủ chữ ký App, hồ sơ giấy và bản scan giấy đã ký.</p>
      </div>
      <div className="grid gap-3 md:grid-cols-2 2xl:grid-cols-4">
        {checkpoints.map((checkpoint, index) => (
          <div key={checkpoint.label} className={clsx('rounded-lg border p-3', checkpoint.done ? 'border-emerald-200 bg-emerald-50' : 'border-amber-200 bg-amber-50')}>
            <div className="flex items-start gap-2">
              {checkpoint.done ? <CheckCircle2 className="mt-0.5 h-5 w-5 shrink-0 text-emerald-700" /> : <span className="inline-flex h-5 w-5 shrink-0 items-center justify-center rounded-full bg-amber-200 text-xs font-bold text-amber-900">{index + 1}</span>}
              <div>
                <p className="text-sm font-semibold text-ink">{checkpoint.label}</p>
                <p className="mt-1 text-xs leading-5 text-slate-600">{checkpoint.detail}</p>
              </div>
            </div>
          </div>
        ))}
      </div>
      <div className="flex flex-wrap gap-2">
        {permissions.canDispatch && readiness?.app_signed && !readiness.paper_dispatched ? (
          <PrimaryButton label="Gửi hồ sơ giấy" icon={<Send className="h-4 w-4" />} onClick={onDispatch} />
        ) : null}
        {permissions.canScan && sentDispatch ? (
          <PrimaryButton label="Nhận scan giấy đã ký" icon={<Upload className="h-4 w-4" />} onClick={() => onReceive(sentDispatch)} />
        ) : null}
        {permissions.canSign && contract.status === 'signed' && readiness?.returned_scan_received ? (
          <PrimaryButton label="Kích hoạt hợp đồng" icon={<PackageCheck className="h-4 w-4" />} onClick={onActivate} />
        ) : null}
      </div>
      <div className="grid gap-4 2xl:grid-cols-2">
        <section className="rounded-lg border border-line">
          <div className="border-b border-line bg-slate-50 px-3 py-2">
            <h4 className="text-sm font-semibold text-ink">Chữ ký App</h4>
          </div>
          <div className="space-y-2 p-3">
            {(contract.workflow_signatures ?? []).length ? (contract.workflow_signatures ?? []).map((signature) => (
              <div key={signature.workflow_form_signature_id} className="rounded-md border border-line p-3 text-sm">
                <div className="flex items-center justify-between gap-2">
                  <span className="font-semibold text-ink">{signature.signer_name}</span>
                  <span className="text-xs text-slate-500">{dateText(signature.signed_at)}</span>
                </div>
                <p className="mt-1 font-mono text-xs text-slate-500">Hash {signature.payload_hash?.slice(0, 16)}…</p>
              </div>
            )) : <EmptyState title={submittedForm ? 'Biểu mẫu đã gửi, đang chờ ký trên App' : 'Chưa có biểu mẫu ký hợp đồng đã gửi'} />}
          </div>
        </section>
        <section className="rounded-lg border border-line">
          <div className="border-b border-line bg-slate-50 px-3 py-2">
            <h4 className="text-sm font-semibold text-ink">Lịch sử hồ sơ giấy</h4>
          </div>
          <div className="space-y-2 p-3">
            {(contract.dispatches ?? []).length ? (contract.dispatches ?? []).map((dispatch) => (
              <div key={dispatch.id} className="rounded-md border border-line p-3 text-sm">
                <div className="flex flex-wrap items-start justify-between gap-2">
                  <div>
                    <p className="font-semibold text-ink">Lần {dispatch.dispatch_no} · {dispatch.recipient_name}</p>
                    <p className="mt-1 text-xs text-slate-500">{dispatchMethodLabel(dispatch.delivery_method)}{dispatch.tracking_reference ? ` · ${dispatch.tracking_reference}` : ''}</p>
                  </div>
                  <StatusBadge status={dispatch.status} />
                </div>
                {dispatch.returned_scan_document_name ? <p className="mt-2 flex items-center gap-1 text-xs text-emerald-700"><Paperclip className="h-3.5 w-3.5" /> {dispatch.returned_scan_document_name}</p> : null}
                {permissions.canScan && dispatch.status === 'sent' ? <div className="mt-2"><SmallButton label="Tải scan giấy đã ký" icon={<Upload className="h-4 w-4" />} onClick={() => onReceive(dispatch)} /></div> : null}
              </div>
            )) : <EmptyState title="Chưa có lần gửi hồ sơ giấy" />}
          </div>
        </section>
      </div>
    </div>
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
    return null;
  };
  return (
    <ResponsiveRecords
      headers={['Mốc', 'Trước giữ lại', 'Giữ lại', 'Sau giữ lại', 'Điều kiện', 'Trạng thái', '']}
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
        rowAction(row),
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
        </RecordCard>
      ))}
    />
  );
}

function VariationTable({ contract, canReview, canApprove, canCreate, canQS, canPurchase, canProcurement, onAction }: {
  contract: Contract;
  canReview: boolean;
  canApprove: boolean;
  canCreate: boolean;
  canQS: boolean;
  canPurchase: boolean;
  canProcurement: boolean;
  onAction: (value: NonNullable<ActionDialog>) => void;
}) {
  const rows = contract.variations ?? [];
  if (!rows.length) return <EmptyState title="Chưa có phụ lục hợp đồng" />;
  const rowAction = (row: ContractVariation) => {
    if ((canCreate || canReview) && row.status === 'draft') return <SmallButton label="Gửi QS" onClick={() => onAction(variationSubmitAction(row))} />;
    if (canQS && row.status === 'qs_review') return <SmallButton label="QS xác nhận" onClick={() => onAction(variationQSAction(row))} />;
    if (canPurchase && row.status === 'purchasing_sourcing') return <SmallButton label="Đã lấy giá" onClick={() => onAction(variationPurchaseAction(row))} />;
    if (canApprove && ['pending_approval', 'review'].includes(row.status)) return <SmallButton label="Duyệt" onClick={() => onAction(variationApproveAction(row))} />;
    if (canProcurement && row.status === 'approved' && !row.procurement_request_id) return <SmallButton label="Tạo yêu cầu mua" onClick={() => onAction(variationProcurementAction(row))} />;
    if (canCreate && ['approved', 'procurement_created'].includes(row.status)) return <SmallButton label="Tạo phiên bản HĐ" onClick={() => onAction(variationIncorporateAction(row))} />;
    return null;
  };
  return (
    <ResponsiveRecords
      headers={['', 'Mức độ', 'Mã', 'Nội dung', 'Thay đổi giá trị', 'Thay đổi ngày', 'Trạng thái']}
      rows={rows.map((row) => [
        rowAction(row),
        <PriorityBadge priority={row.priority} />,
        <span className="font-semibold">{row.variation_code}</span>,
        <div><div>{row.title}</div><div className="mt-1 text-xs text-slate-500">{row.reason}</div></div>,
        <SignedCurrency value={row.amount_delta} />,
        signedDays(row.days_delta),
        <StatusBadge status={row.status} />,
      ])}
      cards={rows.map((row) => (
        <RecordCard key={row.id} title={`${row.variation_code} · ${row.title}`} status={row.status} action={rowAction(row)}>
          <RecordValue label="Mức độ"><PriorityBadge priority={row.priority} /></RecordValue>
          <RecordValue label="Lý do" full>{row.reason || 'Chưa có lý do'}</RecordValue>
          <RecordValue label="Thay đổi giá trị"><SignedCurrency value={row.amount_delta} /></RecordValue>
          <RecordValue label="Thay đổi thời gian">{signedDays(row.days_delta)}</RecordValue>
          {row.scope_delta ? <RecordValue label="Thay đổi phạm vi" full>{row.scope_delta}</RecordValue> : null}
          {row.procurement_request_id ? <RecordValue label="Mua hàng" full>Đã tạo yêu cầu mua #{row.procurement_request_id.slice(0, 8)}</RecordValue> : null}
          {row.events?.length ? (
            <RecordValue label="Lịch sử chuyển bước" full>
              <div className="space-y-1">
                {row.events.slice(-4).map((event) => (
                  <div key={event.id} className="text-xs text-slate-600">
                    {identityReferenceText(event.actor_name, event.actor_account_deleted, 'Hệ thống')} · {statusLabel(event.to_status || event.event_type)}{event.comment ? ` · ${event.comment}` : ''}
                  </div>
                ))}
              </div>
            </RecordValue>
          ) : null}
        </RecordCard>
      ))}
    />
  );
}

function ResponsiveRecords({ headers, rows, cards }: { headers: string[]; rows: ReactNode[][]; cards: ReactNode[] }) {
  return (
    <>
      <div className="consflow-table-viewport hidden rounded-md border border-line 2xl:block">
        <table className="w-full text-sm">
          <thead className="bg-slate-50 text-left text-xs uppercase tracking-wide text-slate-500">
            <tr>{headers.map((header, index) => <th key={index} className="px-3 py-2">{header || <span className="sr-only">Thao tác</span>}</th>)}</tr>
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
          { name: 'direction', label: 'Chiều hợp đồng', type: 'select', defaultValue: 'receivable', required: true, options: directionOptions },
          { name: 'counterparty_partner_id', label: 'Đối tác', type: 'select', defaultValue: '', required: true, options: partnerOptions(partners, ['client', 'supplier', 'subcontractor', 'labor_team', 'service_provider', 'other']) },
          { name: 'currency', label: 'Đơn vị tiền tệ', defaultValue: 'VND', required: true },
          { name: 'original_amount', label: 'Giá trị gốc', type: 'currency', currencyCode: (values) => String(values.currency || 'VND'), defaultValue: 0, required: true, min: 0 },
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
          { name: 'amount', label: 'Giá trị phiên bản', type: 'currency', currencyCode: contract?.currency, defaultValue: contract?.current_amount ?? contract?.original_amount ?? 0, required: true, min: 0 },
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
          { name: 'gross_amount', label: 'Giá trị cố định', type: 'currency', currencyCode: contract?.currency, defaultValue: 0, min: 0 },
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

function CreateVariationDialog({ open, currencyCode, onClose, onSubmit }: {
  open: boolean;
  currencyCode?: string;
  onClose: () => void;
  onSubmit: (values: Record<string, any>) => Promise<any>;
}) {
  return (
    <Dialog open={open} title="Đề xuất phát sinh ngoài kế hoạch" description="Luồng bắt buộc: Đề xuất → QS → Mua hàng lấy giá → Quản lý dự án/Giám đốc duyệt → mua hàng/điều chỉnh hợp đồng." onClose={onClose} size="lg">
      <SimpleForm
        layout="two-column"
        submitLabel="Tạo đề xuất phát sinh"
        onCancel={onClose}
        fields={[
          { name: 'variation_code', label: 'Mã phụ lục', defaultValue: `PL-${Date.now().toString().slice(-5)}`, required: true },
          { name: 'title', label: 'Tên phụ lục', defaultValue: 'Điều chỉnh phạm vi hợp đồng', required: true },
          { name: 'priority', label: 'Mức độ', type: 'select', defaultValue: 'normal', required: true, options: variationPriorityOptions },
          { name: 'variation_type', label: 'Loại điều chỉnh', type: 'select', defaultValue: 'mixed', required: true, options: variationTypeOptions },
          { name: 'reason', label: 'Lý do', type: 'textarea', defaultValue: '', required: true, fullWidth: true },
          { name: 'amount_delta', label: 'Thay đổi giá trị (+/-)', type: 'currency', currencyCode, defaultValue: 0, allowNegative: true },
          { name: 'days_delta', label: 'Thay đổi số ngày (+/-)', type: 'number', defaultValue: 0 },
          { name: 'scope_delta', label: 'Thay đổi phạm vi', type: 'textarea', defaultValue: '', fullWidth: true },
        ]}
        onSubmit={onSubmit}
      />
    </Dialog>
  );
}

function ReceiveContractScanDialog({ target, submitting, onClose, onSubmit }: {
  target: ContractDispatch | null;
  submitting: boolean;
  onClose: () => void;
  onSubmit: (file: File, note: string) => Promise<any> | false | null;
}) {
  const toast = useToast();
  const [file, setFile] = useState<File | null>(null);
  const [note, setNote] = useState('');
  const [fileError, setFileError] = useState(false);
  const contentRef = useRef<HTMLDivElement | null>(null);
  const navigateValidation = useValidationNavigation(contentRef);

  useEffect(() => {
    if (!target) {
      setFile(null);
      setNote('');
      setFileError(false);
    }
  }, [target]);

  async function submit() {
    if (!file) {
      setFileError(true);
      toast.warn('Vui lòng chọn bản scan PDF hoặc ảnh hợp đồng giấy đã ký.');
      navigateValidation('signed_scan');
      return;
    }
    await onSubmit(file, note);
  }

  return (
    <Dialog
      open={Boolean(target)}
      title="Nhận bản scan hợp đồng giấy đã ký"
      description={target ? `Lần gửi ${target.dispatch_no} · ${target.recipient_name}` : undefined}
      onClose={onClose}
      footer={(
        <div className="flex justify-end gap-2">
          <button className="rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700" onClick={onClose} disabled={submitting}>Đóng</button>
          <button className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white disabled:opacity-60" onClick={() => void submit()} disabled={submitting}>
            {submitting ? <Loader2 className="h-4 w-4 animate-spin" /> : <Upload className="h-4 w-4" />}
            Lưu bản scan
          </button>
        </div>
      )}
    >
      <div ref={contentRef} className="space-y-3">
        <label
          className={clsx('block rounded-md border border-dashed bg-slate-50 p-4', fileError ? 'border-rose-300' : 'border-line')}
          data-validation-key="signed_scan"
          data-validation-error={fileError ? 'true' : undefined}
        >
          <span className="text-sm font-semibold text-ink">Tệp PDF/ảnh có chữ ký hai bên</span>
          <input
            name="signed_scan"
            className="mt-2 block w-full text-sm text-slate-600"
            type="file"
            accept="application/pdf,image/png,image/jpeg,image/webp"
            disabled={submitting}
            aria-invalid={fileError}
            aria-describedby={fileError ? 'signed-scan-error' : undefined}
            onChange={(event) => {
              const nextFile = event.target.files?.[0] ?? null;
              setFile(nextFile);
              if (nextFile) setFileError(false);
            }}
          />
          {fileError ? <p id="signed-scan-error" role="alert" className="mt-2 text-xs font-medium text-rose-700">Vui lòng chọn tệp PDF hoặc ảnh hợp đồng đã ký.</p> : null}
        </label>
        <label className="block">
          <span className="text-xs font-semibold text-slate-600">Ghi chú tiếp nhận</span>
          <textarea className="mt-1 w-full rounded-md border border-line px-3 py-2 text-sm focus-ring" rows={3} value={note} disabled={submitting} onChange={(event) => setNote(event.target.value)} />
        </label>
      </div>
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
