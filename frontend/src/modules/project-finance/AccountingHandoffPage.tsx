import { useEffect, useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { AlertTriangle, CheckCircle2, ExternalLink, Inbox, ListFilter, RotateCcw, Send, Undo2 } from 'lucide-react';
import { Link, useParams, useSearchParams } from 'react-router-dom';
import { CurrencyText } from '../../components/CurrencyText';
import { DataTable, type Column } from '../../components/DataTable';
import { EmptyState } from '../../components/EmptyState';
import { PageHeader } from '../../components/PageHeader';
import { ProjectBackLink } from '../../components/ProjectBackLink';
import { StatusBadge } from '../../components/StatusBadge';
import { useToast } from '../../components/ToastProvider';
import { WorkspaceTabs, type WorkspaceTabItem } from '../../components/WorkspaceTabs';
import { get, post } from '../../lib/api/client';
import { useAuth } from '../../lib/auth/AuthContext';
import type { AccountingHandoffItem, AccountingHandoffWorkspace } from '../../lib/types/domain';
import { dateText } from '../../lib/utils/format';
import { AsyncProjectPicker } from '../_shared/AsyncProjectPicker';
import { ProjectScopeActions } from '../_shared/ProjectScopeActions';
import {
  syncWorkItemMutationTransition,
  withWorkItemMutationContext,
  workItemAutoStartedNotice,
  type WorkItemMutationResult,
} from '../work-items/workItemMutationContext';
import {
  AccountingHandoffActionDialog,
  type AccountingHandoffAction,
  type AccountingHandoffDialog,
} from './AccountingHandoffActionDialog';
import { invalidateAccountingWork } from './accountingWorkActions';

type HandoffFilter = 'all' | 'needs_attention' | 'ready' | 'handed_off' | 'acknowledged' | 'returned';

const filterLabels: Array<WorkspaceTabItem<HandoffFilter> & { countKey?: keyof AccountingHandoffWorkspace['summary'] }> = [
  { id: 'all', label: 'Tất cả', icon: ListFilter },
  { id: 'needs_attention', label: 'Cần bổ sung', icon: AlertTriangle, countKey: 'needs_attention_count' },
  { id: 'ready', label: 'Sẵn sàng', icon: CheckCircle2, countKey: 'ready_count' },
  { id: 'handed_off', label: 'Đã bàn giao', icon: Send, countKey: 'handed_off_count' },
  { id: 'acknowledged', label: 'Đã tiếp nhận', icon: Inbox, countKey: 'acknowledged_count' },
  { id: 'returned', label: 'Bị trả lại', icon: Undo2, countKey: 'returned_count' },
];

export function AccountingHandoffPage({ global = false }: { global?: boolean }) {
  const { id } = useParams();
  const [searchParams, setSearchParams] = useSearchParams();
  const requestedProjectId = searchParams.get('project') ?? '';
  const taskId = searchParams.get('task')?.trim() ?? '';
  const [draftProjectId, setDraftProjectId] = useState(requestedProjectId);
  const [filter, setFilter] = useState<HandoffFilter>('all');
  const [dialog, setDialog] = useState<AccountingHandoffDialog<AccountingHandoffItem>>(null);
  const { hasAnyPermission } = useAuth();
  const toast = useToast();
  const queryClient = useQueryClient();
  const projectId = id ?? (global ? requestedProjectId : '');
  const workspace = useQuery({
    queryKey: ['project-finance', projectId],
    queryFn: () => get<AccountingHandoffWorkspace>(`/projects/${projectId}/project-finance`),
    enabled: Boolean(projectId),
  });
  const canSubmit = hasAnyPermission(['accounting_handoff.submit']);
  const canAcknowledge = hasAnyPermission(['accounting_handoff.acknowledge']);
  const canViewProcurement = hasAnyPermission(['procurement.view']);
  const canViewContracts = hasAnyPermission(['contract.view']);
  const data = workspace.data;
  const rows = useMemo(() => (data?.items ?? []).filter((item) => filter === 'all' || item.status === filter), [data?.items, filter]);

  useEffect(() => {
    if (global) setDraftProjectId(requestedProjectId);
  }, [global, requestedProjectId]);

  const refresh = () => queryClient.invalidateQueries({ queryKey: ['project-finance', projectId] });
  const mutation = useMutation({
    mutationFn: ({ action, item, note, returnReason, externalReference }: { action: AccountingHandoffAction; item: AccountingHandoffItem; note: string; returnReason: string; externalReference: string }) => {
      const body = withWorkItemMutationContext({ note, return_reason: returnReason, external_reference: externalReference }, taskId);
      if (action === 'handoff' && !item.handoff_id) {
        return post<WorkItemMutationResult>(`/projects/${projectId}/accounting-handoffs`, withWorkItemMutationContext({ source_type: item.source_type, source_id: item.source_id, note, external_reference: externalReference }, taskId));
      }
      return post<WorkItemMutationResult>(`/accounting-handoffs/${item.handoff_id}/${action}`, body);
    },
    onSuccess: async (result, variables) => {
      await invalidateAccountingWork(queryClient, projectId);
      syncWorkItemMutationTransition(queryClient, result);
      setDialog(null);
      const message = variables.action === 'acknowledge'
        ? 'Đã xác nhận tiếp nhận hồ sơ'
        : variables.action === 'return'
          ? 'Đã trả hồ sơ để bổ sung'
          : 'Đã bàn giao hồ sơ cho Kế toán';
      toast.success(`${message}.${workItemAutoStartedNotice(result)}`);
    },
    onError: (error) => toast.error('Không cập nhật được bàn giao kế toán', error),
  });

  function applyProjectScope() {
    if (!draftProjectId) return;
    const params = new URLSearchParams(searchParams);
    params.set('project', draftProjectId);
    params.delete('tab');
    setSearchParams(params, { replace: true });
    setFilter('all');
    setDialog(null);
  }

  function resetProjectScope() {
    const params = new URLSearchParams(searchParams);
    params.delete('project');
    params.delete('tab');
    setSearchParams(params, { replace: true });
    setDraftProjectId('');
    setFilter('all');
    setDialog(null);
  }

  const columns: Column<AccountingHandoffItem>[] = [
    {
      key: 'source',
      header: 'Hồ sơ nguồn',
      filterValue: (row) => [row.source_code, row.source_title, row.counterparty_name],
      render: (row) => <div><Link className="font-semibold text-blue-700 hover:underline" to={row.source_route}>{row.source_code}</Link><div className="font-semibold text-ink">{row.source_title}</div><div className="text-xs text-slate-500">{row.counterparty_name || 'Chưa xác định đối tác'}</div></div>,
    },
    {
      key: 'direction',
      header: 'Nhóm hồ sơ',
      render: (row) => <div><span className="font-semibold text-slate-700">{row.direction === 'payable' ? 'Hồ sơ mua' : 'Hồ sơ thu'}</span><div className="text-xs text-slate-500">{row.source_type === 'procurement_request' ? 'Yêu cầu mua' : 'Mốc hợp đồng'}</div></div>,
    },
    {
      key: 'reference_amount',
      header: 'Giá trị tham chiếu',
      align: 'right',
      sortable: true,
      sortValue: (row) => Number(row.reference_amount ?? 0),
      render: (row) => <div><CurrencyText value={row.reference_amount} /><div className="text-xs text-slate-400">{row.currency} · từ hồ sơ nguồn</div></div>,
    },
    {
      key: 'readiness',
      header: 'Điều kiện nguồn',
      render: (row) => <Requirements item={row} />,
    },
    {
      key: 'status',
      header: 'Bàn giao',
      sortable: true,
      render: (row) => <div><StatusBadge status={row.status} />{row.return_reason ? <div className="mt-1 max-w-52 text-xs text-rose-700">{row.return_reason}</div> : null}{row.acknowledged_at ? <div className="mt-1 text-xs text-slate-500">{dateText(row.acknowledged_at)}</div> : null}</div>,
    },
    {
      key: 'actions',
      header: 'Thao tác',
      action: true,
      align: 'right',
      render: (row) => <RowActions item={row} canSubmit={canSubmit} canAcknowledge={canAcknowledge} onOpen={setDialog} />,
    },
  ];

  return (
    <div className="space-y-5">
      <PageHeader
        title="Hồ sơ bàn giao kế toán"
        description="Theo dõi hồ sơ nghiệp vụ từ Mua sắm và Hợp đồng đến lúc Kế toán tiếp nhận. Hạch toán, hóa đơn, thuế, thanh toán và báo cáo tài chính thực hiện trên MISA."
        horizontalFrom={global ? 'xl' : 'md'}
        actions={global ? (
          <section className="w-full min-w-0 border-t border-line pt-4 xl:w-[36rem] xl:border-0 xl:pt-0" aria-label="Bộ lọc công trình bàn giao kế toán">
            <div className="flex flex-col gap-3 sm:flex-row sm:items-end">
              <AsyncProjectPicker value={draftProjectId} onChange={setDraftProjectId} endpoint="/project-finance/projects" queryKey="accounting-handoff-project-options" emptyLabel="Chọn công trình" allowEmptySelection={false} />
              <ProjectScopeActions applying={workspace.isFetching} applyDisabled={!draftProjectId || draftProjectId === projectId} resetDisabled={workspace.isFetching || (!draftProjectId && !projectId)} onApply={applyProjectScope} onReset={resetProjectScope} />
            </div>
          </section>
        ) : id ? <ProjectBackLink /> : undefined}
      />

      {!projectId ? <EmptyState title="Chọn công trình để xem hồ sơ bàn giao kế toán." /> : <>
        <WorkspaceTabs
          tabs={filterLabels.map(({ countKey, ...tab }) => ({ ...tab, count: countKey ? data?.summary[countKey] ?? 0 : undefined }))}
          activeTab={filter}
          onChange={setFilter}
          ariaLabel="Lọc trạng thái bàn giao"
        />
        <div className="rounded-lg border border-blue-100 bg-blue-50 px-4 py-3 text-sm text-blue-900">
          Consflow chỉ kiểm tra hồ sơ nguồn và ghi nhận bàn giao/tiếp nhận. Giá trị hiển thị là tham chiếu từ hồ sơ nghiệp vụ, không phải số liệu kế toán.
        </div>
        <DataTable
          rows={rows}
          columns={columns}
          loading={workspace.isLoading}
          error={workspace.error}
          onRetry={() => void workspace.refetch()}
          searchPlaceholder="Tìm mã hồ sơ, nội dung, đối tác..."
          pageSize={15}
          empty={data?.items.length ? 'Không có hồ sơ phù hợp với bộ lọc hoặc tìm kiếm.' : 'Chưa có hồ sơ nguồn để bàn giao'}
          emptyAction={data?.items.length === 0 ? (
            <AccountingHandoffEmptyActions
              projectId={projectId}
              canViewProcurement={canViewProcurement}
              canViewContracts={canViewContracts}
            />
          ) : undefined}
        />
      </>}

      <AccountingHandoffActionDialog dialog={dialog} pending={mutation.isPending} onClose={() => setDialog(null)} onSubmit={(values) => dialog ? mutation.mutate({ action: dialog.action, item: dialog.item, ...values }) : undefined} />
    </div>
  );
}

function AccountingHandoffEmptyActions({ projectId, canViewProcurement, canViewContracts }: { projectId: string; canViewProcurement: boolean; canViewContracts: boolean }) {
  const hasSourceNavigation = canViewProcurement || canViewContracts;
  const linkClass = 'inline-flex w-full items-center justify-center gap-2 rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:border-blue-200 hover:bg-blue-50 hover:text-blue-700 sm:w-auto';
  return (
    <section className="w-full max-w-2xl text-left" aria-label="Hướng dẫn tạo hồ sơ nguồn bàn giao kế toán">
      <p className="text-sm leading-6 text-slate-600">Hồ sơ sẽ tự xuất hiện tại đây sau khi nghiệp vụ nguồn đạt điều kiện tối thiểu:</p>
      <ul className="mt-2 space-y-1 text-sm leading-6 text-slate-600">
        <li>• Yêu cầu mua đã được duyệt hoặc đặt hàng tại Mua sắm.</li>
        <li>• Mốc hợp đồng chủ đầu tư đã được kích hoạt hoặc đủ điều kiện thu.</li>
      </ul>
      {hasSourceNavigation ? (
        <div className="mt-4 flex flex-col gap-2 sm:flex-row sm:justify-center">
          {canViewProcurement ? <Link className={linkClass} to={`/projects/${projectId}/procurement`}><ExternalLink className="h-4 w-4" />Mở Mua sắm</Link> : null}
          {canViewContracts ? <Link className={linkClass} to={`/projects/${projectId}/contracts`}><ExternalLink className="h-4 w-4" />Mở Hợp đồng</Link> : null}
        </div>
      ) : (
        <p className="mt-4 rounded-md bg-slate-50 px-3 py-2 text-center text-xs text-slate-500">Tài khoản hiện tại chưa có quyền mở Mua sắm hoặc Hợp đồng.</p>
      )}
      <p className="mt-3 text-center text-xs text-slate-500">Không cần tạo hồ sơ kế toán thủ công trên Consflow.</p>
    </section>
  );
}

function Requirements({ item }: { item: AccountingHandoffItem }) {
  const unmet = item.requirements?.filter((requirement) => !requirement.met) ?? [];
  if (!unmet.length) return <div><StatusBadge status="ready" /><div className="mt-1 text-xs text-slate-500">{item.acceptance_count ?? 0} nghiệm thu · {item.stock_document_count ?? 0} phiếu kho</div></div>;
  return <div><StatusBadge status="needs_attention" /><ul className="mt-1 space-y-0.5 text-xs text-amber-800">{unmet.map((requirement) => <li key={requirement.key}>• {requirement.label}</li>)}</ul></div>;
}

function RowActions({ item, canSubmit, canAcknowledge, onOpen }: { item: AccountingHandoffItem; canSubmit: boolean; canAcknowledge: boolean; onOpen: (dialog: AccountingHandoffDialog<AccountingHandoffItem>) => void }) {
  const buttonClass = 'inline-flex items-center gap-1.5 rounded-md px-2.5 py-2 text-xs font-semibold';
  if (item.status === 'ready' && canSubmit) return <button className={`${buttonClass} bg-blue-600 text-white hover:bg-blue-700`} onClick={() => onOpen({ action: 'handoff', item })}><Send className="h-3.5 w-3.5" />Bàn giao</button>;
  if (item.status === 'handed_off' && canAcknowledge) return <div className="flex justify-end gap-1"><button className={`${buttonClass} bg-emerald-600 text-white hover:bg-emerald-700`} onClick={() => onOpen({ action: 'acknowledge', item })}><CheckCircle2 className="h-3.5 w-3.5" />Tiếp nhận</button><button className={`${buttonClass} border border-line bg-white text-slate-700 hover:bg-slate-50`} onClick={() => onOpen({ action: 'return', item })}><Undo2 className="h-3.5 w-3.5" />Trả lại</button></div>;
  if (item.status === 'returned' && canSubmit) return <button className={`${buttonClass} bg-blue-600 text-white hover:bg-blue-700`} onClick={() => onOpen({ action: 'handoff', item })}><RotateCcw className="h-3.5 w-3.5" />Bàn giao lại</button>;
  return <Link className={`${buttonClass} border border-line bg-white text-slate-700 hover:bg-slate-50`} to={item.source_route}><ExternalLink className="h-3.5 w-3.5" />Mở nguồn</Link>;
}
