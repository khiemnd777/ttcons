import { useEffect, useRef, useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { Link } from 'react-router-dom';
import { Dialog } from '../../components/Dialog';
import { useToast } from '../../components/ToastProvider';
import { ApiError, get, post, put } from '../../lib/api/client';
import { useAuth } from '../../lib/auth/AuthContext';
import type { Partner, WorkItem } from '../../lib/types/domain';
import { partnerOptions } from '../core-data/partnerUtils';
import { syncWorkItemMutationTransition, withWorkItemMutationContext, workItemAutoStartedNotice } from '../work-items/workItemMutationContext';
import { workItemDetailPath } from '../work-items/workItemNavigation';
import { ProcurementDetail } from './ProcurementDetail';
import { procurementActionFor, procurementActionLabels, procurementActionSuccess, procurementHeader, procurementMutationWorkItemId, procurementStatusLabels, validateProcurementHeader, type ProcurementAction, type ProcurementHeader, type ProcurementRow } from './procurementWorkspace';

type Props = { projectId: string; requestId?: string; workItem?: WorkItem; taskId?: string; readOnly?: boolean; onClose: () => void; onShowWorkDetails?: () => void };
const secondary = 'rounded-md border border-line px-3 py-2 text-sm font-semibold disabled:opacity-40';
const primary = 'rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white disabled:opacity-40';

export function ProcurementDialog(props: Props) {
  const { user } = useAuth();
  return <ProcurementSession key={`${user?.id}:${props.projectId}:${props.requestId ?? ''}`} {...props} />;
}

function ProcurementSession({ projectId, requestId, workItem, taskId = '', readOnly = false, onClose, onShowWorkDetails }: Props) {
  const { user } = useAuth();
  const client = useQueryClient();
  const toast = useToast();
  const [selectedId, setSelectedId] = useState(requestId ?? '');
  const [base, setBase] = useState<ProcurementRow | null>(null);
  const [values, setValues] = useState<ProcurementHeader | null>(null);
  const [errors, setErrors] = useState<Partial<Record<keyof ProcurementHeader, string>>>({});
  const [busy, setBusy] = useState(false);
  const busyRef = useRef(false);
  const [failure, setFailure] = useState('');
  const [reloadOpen, setReloadOpen] = useState(false);
  const [discardOpen, setDiscardOpen] = useState(false);
  const [search, setSearch] = useState('');
  const task = useQuery({ queryKey: ['work-item', taskId], queryFn: async () => {
    const row = await get<WorkItem>(`/work-items/${taskId}`);
    if (row.id !== taskId || row.project_id !== projectId) throw new Error('Công việc không thuộc công trình này');
    return row;
  }, enabled: Boolean(user?.id && taskId && !workItem), retry: false });
  const list = useQuery({ queryKey: ['procurement', 'discussion', user?.id, projectId], queryFn: () => get<ProcurementRow[]>(`/procurement-requests?project_id=${encodeURIComponent(projectId)}`), enabled: Boolean(user?.id && !requestId), retry: false });
  const detail = useQuery({ queryKey: ['procurement-detail', user?.id, projectId, selectedId], queryFn: async () => {
    const row = await get<ProcurementRow>(`/procurement-requests/${selectedId}`);
    if (row.id !== selectedId || row.project_id !== projectId || ['cancelled', 'deleted'].includes(row.status)) throw new Error('Yêu cầu mua không còn khả dụng trong công trình này');
    return row;
  }, enabled: Boolean(user?.id && selectedId), retry: false, staleTime: 0, refetchOnWindowFocus: true });
  const current = detail.isFetchedAfterMount && !detail.isError ? detail.data : undefined;
  const canEdit = !readOnly && current?.capabilities?.edit === true;
  const partners = useQuery({ queryKey: ['partners', 'supplier', 'active'], queryFn: () => get<Partner[]>('/partners?status=active'), enabled: canEdit, retry: false });
  const dirty = Boolean(base && values && JSON.stringify(procurementHeader(base)) !== JSON.stringify(values));
  const stale = Boolean(base && current && base.edit_version !== current.edit_version);
  const taskUnavailable = Boolean(taskId && !workItem && (!task.isFetchedAfterMount || task.isError));
  const action = current ? procurementActionFor(current, readOnly || taskUnavailable) : null;
  const needsEditor = Boolean(action === 'order' && !canEdit && values && Object.keys(validateProcurementHeader(values, 'order')).length);
  const editableRows = (list.data ?? []).filter(row => row.project_id === projectId && !['cancelled', 'deleted'].includes(row.status));
  const filteredRows = editableRows.filter(row => `${row.request_code} ${row.title} ${row.supplier_name ?? ''}`.toLocaleLowerCase('vi').includes(search.trim().toLocaleLowerCase('vi')));
  useEffect(() => {
    if (!selectedId && !list.isError && list.isFetchedAfterMount && editableRows.length === 1) setSelectedId(editableRows[0].id);
  }, [selectedId, list.data, list.isError, list.isFetchedAfterMount]);
  useEffect(() => {
    if (current && (!base || (!dirty && !busy && current !== base && current.edit_version === base.edit_version))) { setBase(current); setValues(procurementHeader(current)); }
  }, [current, base, dirty, busy]);
  useEffect(() => {
    if (!dirty && !busy) return;
    const prevent = (event: BeforeUnloadEvent) => { event.preventDefault(); event.returnValue = ''; };
    window.addEventListener('beforeunload', prevent);
    return () => window.removeEventListener('beforeunload', prevent);
  }, [dirty, busy]);
  const close = () => { if (busyRef.current) return; if (dirty) setDiscardOpen(true); else onClose(); };
  const invalidate = () => {
    for (const key of [['procurement'], ['procurement-detail'], ['activity-topic-detail'], ['project-activity', projectId], ['utility-project-activity', projectId], ['project-discussions'], ['project-operations', projectId], ['my-work-items'], ['work-item'], ['project-finance', projectId]]) void client.invalidateQueries({ queryKey: key });
  };
  const loadLatest = async () => {
    if (busyRef.current) return;
    busyRef.current = true; setBusy(true);
    try {
      const result = await detail.refetch();
      if (!result.error && result.data) { setBase(result.data); setValues(procurementHeader(result.data)); setErrors({}); setFailure(''); setReloadOpen(false); }
    } finally { busyRef.current = false; setBusy(false); }
  };
  const execute = async (next?: ProcurementAction) => {
    if (busyRef.current || !base || !values || !current || stale || detail.isFetching || readOnly) return;
    if ((next && next !== action) || (dirty && !canEdit) || (!next && !canEdit)) return;
    const invalid = validateProcurementHeader(values, next);
    setErrors(invalid);
    if (Object.keys(invalid).length) { toast.warn({ title: 'Vui lòng kiểm tra thông tin', description: Object.values(invalid).join('. ') }); return; }
    busyRef.current = true; setBusy(true); setFailure('');
    let saved = false;
    try {
      let version = base.edit_version;
      if (dirty) {
        const updated = await put<ProcurementRow>(`/procurement-requests/${base.id}`, { ...values, expected_version: version });
        version = updated.edit_version;
        const savedBase = { ...base, ...updated };
        setBase(savedBase); setValues(procurementHeader(savedBase)); saved = true;
      }
      if (next) {
        const contextId = procurementMutationWorkItemId(workItem ?? task.data);
        const result = await post<ProcurementRow>(`/procurement-requests/${base.id}/${next}`, withWorkItemMutationContext({ expected_version: version }, contextId));
        setBase({ ...base, ...result }); setValues(procurementHeader(result));
        syncWorkItemMutationTransition(client, result);
        toast.success(`${procurementActionSuccess[next]}.${workItemAutoStartedNotice(result)}`);
      } else toast.success('Đã lưu thông tin yêu cầu mua');
      const refreshed = await detail.refetch();
      if (!refreshed.error && refreshed.data) { setBase(refreshed.data); setValues(procurementHeader(refreshed.data)); }
    } catch (error) {
      const message = `${saved ? 'Đã lưu thông tin; chưa thực hiện được bước tiếp theo. ' : ''}${error instanceof Error ? error.message : 'Không cập nhật được yêu cầu mua'}`;
      setFailure(message);
      if (error instanceof ApiError && error.status === 409) void detail.refetch();
      if (error instanceof ApiError && [401, 403, 404].includes(error.status)) void detail.refetch();
      if (error instanceof ApiError && error.details && typeof error.details === 'object' && 'field' in error.details) setErrors({ [String(error.details.field)]: message });
      toast.error(message);
    } finally { invalidate(); busyRef.current = false; setBusy(false); }
  };
  const choose = (id: string) => { setSelectedId(id); setBase(null); setValues(null); setErrors({}); setFailure(''); };
  const visible = Boolean(current && base && values);
  const supplierOptions = partnerOptions(partners.data ?? [], ['supplier', 'service_provider', 'other']);
  const fields: Array<{ key: keyof ProcurementHeader; label: string; type?: string }> = [
    { key: 'title', label: 'Tiêu đề' }, { key: 'supplier_partner_id', label: 'Nhà cung cấp' },
    { key: 'order_reference', label: 'Số PO / Đơn đặt hàng' }, { key: 'expected_delivery_date', label: 'Ngày giao dự kiến', type: 'date' },
  ];
  return <><Dialog open manageFocus mobileFullscreen size="xl" title="Yêu cầu mua" description={current ? `${current.request_code} · ${current.title}` : 'Chọn yêu cầu mua của công trình để xử lý'} onClose={close}
    footer={<div className="flex flex-wrap items-center justify-between gap-2">
      <div className="flex flex-wrap gap-2"><button className={secondary} disabled={busy} onClick={close}>Đóng</button>{onShowWorkDetails ? <button className={secondary} disabled={dirty || busy} onClick={onShowWorkDetails}>Điều kiện & chốt công việc</button> : null}</div>
      {visible ? <div className="flex flex-wrap gap-2">{canEdit ? <button className={secondary} disabled={busy || !dirty || stale || detail.isFetching} onClick={() => void execute()}>Lưu thông tin</button> : null}{action ? <button className={primary} disabled={busy || stale || detail.isFetching || needsEditor || (dirty && !canEdit)} onClick={() => void execute(action)}>{busy ? 'Đang xử lý…' : procurementActionLabels[action]}</button> : null}</div> : null}
    </div>}>
    {!selectedId ? <div className="space-y-3">
      {list.isPending || (list.isFetching && !list.isFetchedAfterMount) ? <p role="status">Đang tải yêu cầu mua…</p> : list.isError ? <p role="alert">Không tải được yêu cầu mua. <button className="text-blue-700 underline" onClick={() => void list.refetch()}>Thử lại</button></p> : <>
        {editableRows.length ? <><label className="block text-sm">Tìm yêu cầu mua<input className="mt-1 w-full rounded border border-line p-2" value={search} onChange={event => setSearch(event.target.value)} placeholder="Mã yêu cầu, tiêu đề, nhà cung cấp…" /></label>
          {filteredRows.map(row => <button className="block w-full rounded border border-line p-3 text-left hover:bg-blue-50" key={row.id} onClick={() => choose(row.id)}><strong className="block text-sm text-blue-700">{row.request_code} · {row.title}</strong><span className="text-xs text-slate-600">{row.supplier_name || 'Chưa có nhà cung cấp'} · {procurementStatusLabels[row.status] ?? row.status}</span></button>)}
          {!filteredRows.length ? <p className="text-sm text-slate-600">Không có yêu cầu phù hợp.</p> : null}
        </> : <p>Chưa có yêu cầu mua trong công trình.</p>}
        <Link target="_blank" rel="noreferrer" className="inline-block text-sm font-semibold text-blue-700 underline" to={`/projects/${projectId}/boq`}>Mở BOQ</Link>
      </>}
    </div> : <div className="space-y-4">
      {!requestId && editableRows.length > 1 ? <button className={secondary} disabled={dirty || busy} onClick={() => choose('')}>Chọn yêu cầu khác</button> : null}
      {!detail.isFetchedAfterMount ? <p role="status">Đang tải yêu cầu mua…</p> : null}
      {detail.isError ? <p role="alert" className="rounded bg-rose-50 p-3 text-sm text-rose-800">Không tải được yêu cầu mua hoặc quyền truy cập đã thay đổi. <button className="underline" onClick={() => void detail.refetch()}>Thử lại</button></p> : null}
      {failure ? <p role="alert" className="rounded bg-rose-50 p-3 text-sm text-rose-800">{failure}</p> : null}
      {taskId && !workItem && task.isError ? <p role="alert" className="text-sm text-rose-700">Không xác minh được công việc liên quan. <button className="underline" onClick={() => void task.refetch()}>Thử lại</button></p> : null}
      {visible ? <>
        {stale ? <p role="alert" className="rounded bg-amber-50 p-3 text-sm text-amber-900">Hồ sơ đã được người khác cập nhật. <button className="font-semibold underline" onClick={() => dirty ? setReloadOpen(true) : void loadLatest()}>Xem bản mới</button></p> : null}
        {readOnly ? <p className="text-sm text-amber-800">Chỉ xem theo trạng thái và quyền công việc hiện tại.</p> : null}
        {needsEditor ? <p className="rounded bg-amber-50 p-3 text-sm text-amber-900">Thông tin đặt hàng chưa đủ. Người có quyền sửa cần bổ sung nhà cung cấp, số PO và ngày giao trước khi đặt hàng.</p> : null}
        {canEdit || dirty || Object.keys(errors).length ? <div className="grid gap-3 sm:grid-cols-2">{fields.map(field => {
          const required = field.key === 'title' || current?.status === 'approved';
          const fieldId = `procurement-${selectedId}-${field.key}`;
          const inputClass = `mt-1 w-full rounded-md border bg-white p-2 text-sm disabled:bg-slate-50 ${errors[field.key] ? 'border-rose-500' : 'border-line'}`;
          const change = (value: string) => { setValues(previous => previous ? { ...previous, [field.key]: value } : previous); setErrors(previous => ({ ...previous, [field.key]: undefined })); };
          return <label key={field.key} className="block text-sm font-medium" htmlFor={fieldId}>{field.label}{required ? ' *' : ''}
            {field.key === 'supplier_partner_id' ? <select id={fieldId} className={inputClass} value={values![field.key]} disabled={!canEdit || busy} aria-invalid={Boolean(errors[field.key])} aria-describedby={errors[field.key] ? `${fieldId}-error` : undefined} onChange={event => change(event.target.value)}>
              <option value="">Chọn nhà cung cấp</option>{values!.supplier_partner_id && !supplierOptions.some(option => option.value === values!.supplier_partner_id) ? <option value={values!.supplier_partner_id}>{base!.supplier_name || 'Nhà cung cấp hiện tại'}</option> : null}{supplierOptions.map(option => <option key={option.value} value={option.value}>{option.label}</option>)}
            </select> : <input id={fieldId} className={inputClass} type={field.type ?? 'text'} value={values![field.key]} disabled={!canEdit || busy} aria-invalid={Boolean(errors[field.key])} aria-describedby={errors[field.key] ? `${fieldId}-error` : undefined} onChange={event => change(event.target.value)} />}
            {errors[field.key] ? <span id={`${fieldId}-error`} className="mt-1 block text-xs text-rose-700">{errors[field.key]}</span> : null}
          </label>;
        })}{partners.isError ? <p className="text-xs text-amber-800">Không tải được danh sách nhà cung cấp. <button className="underline" onClick={() => void partners.refetch()}>Thử lại</button></p> : null}</div> : null}
        <ProcurementDetail record={base!} />
        {['ordered', 'partially_received'].includes(current!.status) ? <Link target="_blank" rel="noreferrer" className="inline-block text-sm font-semibold text-blue-700 underline" to={current!.incoming_acceptance_work_item_id ? workItemDetailPath(current!.incoming_acceptance_work_item_id, `/projects/${projectId}?tab=activity`, projectId) : `/projects/${projectId}/procurement`}>{current!.incoming_acceptance_work_item_id ? 'Mở nghiệm thu liên quan' : 'Mở Mua sắm để theo dõi nghiệm thu'}</Link> : null}
        {!action && !readOnly && !['ordered', 'partially_received', 'received'].includes(current!.status) ? <p className="text-sm text-slate-600">{current!.approval_message || 'Chờ người có quyền thực hiện bước tiếp theo.'}</p> : null}
      </> : null}
    </div>}
  </Dialog>
  <Dialog open={discardOpen || reloadOpen} manageFocus title={reloadOpen ? 'Xem bản mới và bỏ phần chưa lưu?' : 'Bỏ thay đổi chưa lưu?'} size="sm" onClose={() => { setDiscardOpen(false); setReloadOpen(false); }} footer={<div className="flex justify-end gap-2"><button className={secondary} onClick={() => { setDiscardOpen(false); setReloadOpen(false); }}>Tiếp tục nhập</button><button className={primary} onClick={() => reloadOpen ? void loadLatest() : onClose()}>{reloadOpen ? 'Tải bản mới' : 'Bỏ thay đổi'}</button></div>}><p>Nội dung chưa lưu trong yêu cầu mua này sẽ bị bỏ.</p></Dialog>
  </>;
}
