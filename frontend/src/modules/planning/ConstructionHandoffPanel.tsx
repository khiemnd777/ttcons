import { useEffect, useMemo, useState, type Dispatch, type ReactNode, type SetStateAction } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { ArrowRight, CheckCircle2, ClipboardCheck, Loader2, Plus, ShieldCheck, Trash2 } from 'lucide-react';
import { useLocation, useNavigate } from 'react-router-dom';
import { Dialog } from '../../components/Dialog';
import { identityReferenceText } from '../../components/IdentityReferenceLabel';
import { useToast } from '../../components/ToastProvider';
import { useAuth } from '../../lib/auth/AuthContext';
import { get, post } from '../../lib/api/client';
import type {
  ConstructionCheckCriterion,
  ConstructionHandoff,
  ConstructionHandoffCandidate,
  ConstructionHandoffItem,
  ConstructionHandoffWorkspace,
} from '../../lib/types/domain';
import {
  constructionHandoffIntentMatches,
  consumeConstructionHandoffNavigationState,
} from '../work-items/workItemNavigation';
import {
  syncWorkItemMutationTransition,
  withWorkItemMutationContext,
  workItemAutoStartedNotice,
  type WorkItemMutationResult,
} from '../work-items/workItemMutationContext';

type EditableItem = ConstructionHandoffCandidate & {
  local_id: string;
  assigned_user_name?: string;
  assigned_user_account_deleted?: boolean;
};

export function ConstructionHandoffPanel({ projectId, currentNode }: { projectId: string; currentNode?: Record<string, any> }) {
  const { hasAnyPermission } = useAuth();
  const canManage = hasAnyPermission(['construction.handoff.manage']);
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const location = useLocation();
  const taskId = new URLSearchParams(location.search).get('task')?.trim() ?? '';
  const toast = useToast();
  const [open, setOpen] = useState(false);
  const [readOnly, setReadOnly] = useState(false);
  const [title, setTitle] = useState('');
  const [notes, setNotes] = useState('');
  const [items, setItems] = useState<EditableItem[]>([]);
  const [errors, setErrors] = useState<Record<string, string>>({});

  const handoff = useQuery({
    queryKey: ['construction-handoff', projectId],
    queryFn: () => get<ConstructionHandoffWorkspace>(`/projects/${projectId}/construction-handoff`),
    enabled: Boolean(projectId),
    retry: false,
  });
  const latest = handoff.data?.handoffs?.[0];
  const latestMetrics = useMemo(() => handoffMetrics(latest), [latest]);
  const handoffOpenIntent = constructionHandoffIntentMatches(location.state, projectId);

  useEffect(() => {
    if (!handoffOpenIntent || !handoff.data) return;
    navigate(`${location.pathname}${location.search}${location.hash}`, {
      replace: true,
      state: consumeConstructionHandoffNavigationState(location.state),
    });
    if (!handoff.data.can_activate) {
      toast.warn({ title: 'Chưa thể mở bàn giao thi công', description: handoff.data.reason || 'Hồ sơ chưa có hạng mục đủ điều kiện bàn giao.' });
      return;
    }
    if (!canManage) {
      toast.warn({ title: 'Không có quyền kích hoạt bàn giao', description: 'Cần quyền quản lý bàn giao thi công để thực hiện bước này.' });
      return;
    }
    setReadOnly(false);
    setTitle(handoff.data.suggested_title || 'Bàn giao kế hoạch thi công');
    setNotes('');
    setItems((handoff.data.candidates ?? []).map(editableCandidate));
    setErrors({});
    setOpen(true);
  }, [canManage, handoff.data, handoffOpenIntent, location.hash, location.pathname, location.search, location.state, navigate, projectId, toast]);

  const activate = useMutation({
    mutationFn: () => post<ConstructionHandoff & WorkItemMutationResult>(`/projects/${projectId}/construction-handoff/activate`, withWorkItemMutationContext({
      title: title.trim(),
      notes: notes.trim(),
      source_run_node_id: handoff.data?.source_run_node_id ?? currentNode?.id ?? '',
      source_form_instance_id: handoff.data?.source_form_instance_id ?? '',
      source_form_revision_no: handoff.data?.source_form_revision_no ?? 0,
      items: items.map((item, index) => ({
        source_field_key: item.source_field_key || 'manual_handoff_items',
        source_item_index: Number.isFinite(item.source_item_index) ? item.source_item_index : index,
        title: item.title.trim(),
        location: item.location?.trim() ?? '',
        scheduled_at: toISO(item.scheduled_at),
        responsible_role: item.responsible_role || 'QA/QC',
        assigned_user_id: item.assigned_user_id ?? '',
        evidence_required: true,
        criteria: normalizedCriteria(item),
      })),
    }, taskId)),
    onSuccess: async (result) => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['construction-handoff', projectId] }),
        queryClient.invalidateQueries({ queryKey: ['construction-workspace', projectId] }),
        queryClient.invalidateQueries({ queryKey: ['pipeline-run', projectId] }),
        queryClient.invalidateQueries({ queryKey: ['project-operations', projectId] }),
        queryClient.invalidateQueries({ queryKey: ['work-item'] }),
      ]);
      setOpen(false);
      syncWorkItemMutationTransition(queryClient, result);
      toast.success({
        title: result.idempotent_replay ? 'Bàn giao này đã được kích hoạt trước đó' : 'Đã bàn giao kế hoạch sang Thi công',
        description: `${result.items?.length ?? items.length} kiểm tra đã được tạo trong không gian Thi công.${workItemAutoStartedNotice(result)}`,
      });
    },
    onError: (error) => toast.error('Không kích hoạt được bàn giao thi công', error),
  });

  function openCandidates() {
    const candidates = handoff.data?.candidates ?? [];
    setReadOnly(false);
    setTitle(handoff.data?.suggested_title || 'Bàn giao kế hoạch thi công');
    setNotes('');
    setItems(candidates.map(editableCandidate));
    setErrors({});
    setOpen(true);
  }

  function openLatest() {
    if (!latest) return;
    setReadOnly(true);
    setTitle(latest.title);
    setNotes(latest.notes ?? '');
    setItems((latest.items ?? []).map(editableHandoffItem));
    setErrors({});
    setOpen(true);
  }

  function save() {
    const next: Record<string, string> = {};
    if (!title.trim()) next.title = 'Cần tên đợt bàn giao';
    if (!items.length) next.items = 'Cần ít nhất một hạng mục giám sát';
    items.forEach((item) => {
      if (!item.title.trim()) next[`${item.local_id}.title`] = 'Cần nội dung kiểm tra';
      if (!String(item.criteria?.[0]?.criteria ?? item.criteria?.[0]?.title ?? '').trim()) next[`${item.local_id}.criteria`] = 'Cần tiêu chí đạt';
    });
    setErrors(next);
    if (Object.keys(next).length) {
      toast.warn({ title: 'Chưa đủ thông tin bàn giao', description: 'Kiểm tra tên đợt, nội dung và tiêu chí đạt của từng hạng mục.' });
      return;
    }
    activate.mutate();
  }

  if (handoff.isLoading) return <HandoffLoading />;
  if (handoff.isError || (!handoff.data?.available && !latest)) return null;

  const candidateCount = handoff.data?.can_activate ? handoff.data.candidates?.length ?? 0 : 0;
  const targetLabel = handoff.data?.target_node_name || 'Thi công';
  const sourceRelevant = ['construction_planning', 'construction'].includes(String(currentNode?.type ?? handoff.data?.source_node_type ?? ''));

  return (
    <>
      <section className="overflow-hidden rounded-lg border border-blue-200 bg-white shadow-panel">
        <div className="grid gap-0 xl:grid-cols-[minmax(0,1fr)_24rem]">
          <div className="p-4 sm:p-5">
            <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
              <div className="min-w-0">
                <div className="flex flex-wrap items-center gap-2">
                  <span className="grid h-9 w-9 place-items-center rounded-md bg-blue-50 text-blue-700"><ClipboardCheck className="h-5 w-5" /></span>
                  <div>
                    <p className="text-xs font-semibold uppercase tracking-wide text-blue-700">Planning → Thi công</p>
                    <h2 className="mt-0.5 font-bold text-ink">Bàn giao hạng mục giám sát</h2>
                  </div>
                </div>
                <p className="mt-3 max-w-3xl text-sm leading-6 text-slate-600">
                  Chốt danh sách một lần tại Planning; hệ thống tạo sẵn đầu việc và kiểm tra để giám sát viên chỉ cần kiểm tra, khắc phục và báo cáo.
                </p>
              </div>
              {sourceRelevant && handoff.data?.can_activate && canManage ? (
                <button className="inline-flex shrink-0 items-center justify-center gap-2 rounded-md bg-blue-600 px-4 py-2.5 text-sm font-semibold text-white hover:bg-blue-700" onClick={openCandidates}>
                  Rà soát & bàn giao
                  <ArrowRight className="h-4 w-4" />
                </button>
              ) : null}
            </div>

            <div className="mt-4 grid gap-px overflow-hidden rounded-md border border-line bg-line sm:grid-cols-3">
              <Metric label="Sẵn sàng bàn giao" value={candidateCount} tone="text-blue-700" />
              <Metric label="Đã kích hoạt" value={latest?.items?.length ?? 0} tone="text-violet-700" />
              <Metric label="Đã hoàn tất" value={latestMetrics.completed} tone="text-emerald-700" />
            </div>
          </div>

          <aside className="border-t border-blue-100 bg-blue-50/60 p-4 sm:p-5 xl:border-l xl:border-t-0">
            <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">Trạng thái handoff</p>
            {latest ? (
              <>
                <div className="mt-3 flex items-start gap-3">
                  <CheckCircle2 className="mt-0.5 h-5 w-5 shrink-0 text-emerald-600" />
                  <div>
                    <p className="font-semibold text-ink">Đã kích hoạt revision {latest.revision_no}</p>
                    <p className="mt-1 text-xs leading-5 text-slate-600">{latest.title} · {identityReferenceText(latest.activated_by_name, latest.activated_by_account_deleted, 'Người lập kế hoạch')}</p>
                  </div>
                </div>
                <div className="mt-4 flex flex-wrap gap-2">
                  <button className="rounded-md border border-blue-200 bg-white px-3 py-2 text-sm font-semibold text-blue-700 hover:bg-blue-50" onClick={openLatest}>Xem bàn giao</button>
                  <button className="rounded-md border border-blue-200 bg-white px-3 py-2 text-sm font-semibold text-blue-700 hover:bg-blue-50" onClick={() => navigate(`/projects/${projectId}/operations?tab=checks`)}>Mở {targetLabel}</button>
                </div>
              </>
            ) : candidateCount ? (
              <div className="mt-3">
                <p className="font-semibold text-blue-900">Có {candidateCount} hạng mục chờ rà soát</p>
                <p className="mt-1 text-xs leading-5 text-blue-800">Kích hoạt một lần để tránh tạo từng kiểm tra thủ công.</p>
                {sourceRelevant && handoff.data?.can_activate && canManage ? <button className="mt-4 w-full rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700" onClick={openCandidates}>Bàn giao sang {targetLabel}</button> : null}
              </div>
            ) : (
              <div className="mt-3">
                <p className="font-semibold text-slate-800">Chưa có hạng mục giám sát</p>
                <p className="mt-1 text-xs leading-5 text-slate-600">Hoàn tất bước “Chốt hạng mục giám sát và bàn giao thi công” trong hàng đợi kế hoạch.</p>
              </div>
            )}
          </aside>
        </div>
      </section>

      <Dialog
        open={open}
        title={readOnly ? `Bàn giao revision ${latest?.revision_no ?? ''}` : 'Rà soát bàn giao sang Thi công'}
        description={readOnly ? 'Snapshot kế hoạch đã dùng để sinh đầu việc và kiểm tra.' : 'Mỗi dòng sẽ sinh đúng một đầu việc và một kiểm tra; có thể chỉnh nhanh trước khi kích hoạt.'}
        size="report"
        onClose={() => setOpen(false)}
        footer={(
          <div className="flex flex-col-reverse gap-2 sm:flex-row sm:items-center sm:justify-between">
            <button className="rounded-md border border-line bg-white px-4 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100" onClick={() => setOpen(false)}>{readOnly ? 'Đóng' : 'Hủy'}</button>
            {!readOnly ? <button disabled={activate.isPending} className="inline-flex items-center justify-center gap-2 rounded-md bg-blue-600 px-4 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:opacity-60" onClick={save}>{activate.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <ShieldCheck className="h-4 w-4" />}Kích hoạt {items.length} kiểm tra</button> : null}
          </div>
        )}
      >
        <div className="space-y-5">
          <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_minmax(0,1fr)]">
            <label className="block">
              <span className="text-sm font-semibold text-slate-700">Tên đợt bàn giao <span className="text-rose-600">*</span></span>
              <input disabled={readOnly} className={controlClass(Boolean(errors.title), readOnly)} value={title} onChange={(event) => { setTitle(event.target.value); setErrors((current) => ({ ...current, title: '' })); }} autoFocus={!readOnly} />
              {errors.title ? <span className="mt-1 block text-xs text-rose-600">{errors.title}</span> : null}
            </label>
            <label className="block">
              <span className="text-sm font-semibold text-slate-700">Lưu ý bàn giao</span>
              <input disabled={readOnly} className={controlClass(false, readOnly)} value={notes} onChange={(event) => setNotes(event.target.value)} placeholder="Phạm vi, điều kiện hoặc lưu ý cho giám sát viên" />
            </label>
          </div>

          <div className="flex flex-col gap-2 border-t border-line pt-5 sm:flex-row sm:items-center sm:justify-between">
            <div>
              <h3 className="font-semibold text-ink">Hạng mục giám sát · {items.length}</h3>
              <p className="mt-1 text-xs text-slate-500">Nội dung và tiêu chí đạt là bắt buộc; người giám sát có thể được gán ngay hoặc để theo vai trò.</p>
            </div>
            {!readOnly ? <button type="button" className="inline-flex items-center justify-center gap-2 rounded-md border border-blue-200 bg-white px-3 py-2 text-sm font-semibold text-blue-700 hover:bg-blue-50" onClick={() => setItems((current) => [...current, emptyItem(current)])}><Plus className="h-4 w-4" /> Thêm hạng mục</button> : null}
          </div>

          {errors.items ? <p className="rounded-md border border-rose-200 bg-rose-50 px-3 py-2 text-sm text-rose-700">{errors.items}</p> : null}
          <div className="space-y-3">
            {items.map((item, index) => (
              <article key={item.local_id} className="rounded-lg border border-line bg-slate-50/60 p-4">
                <div className="flex items-start justify-between gap-3">
                  <div className="flex min-w-0 items-center gap-3">
                    <span className="grid h-8 w-8 shrink-0 place-items-center rounded-full bg-blue-100 text-xs font-bold text-blue-700">{index + 1}</span>
                    <div className="min-w-0"><p className="font-semibold text-ink">Kiểm tra dự kiến</p><p className="mt-0.5 text-xs text-slate-500">{item.source_form_code ? 'Từ biểu mẫu kế hoạch' : 'Thêm trực tiếp'}</p></div>
                  </div>
                  {!readOnly ? <button className="rounded-md p-2 text-slate-400 hover:bg-rose-50 hover:text-rose-600" onClick={() => setItems((current) => current.filter((row) => row.local_id !== item.local_id))} aria-label={`Xóa hạng mục ${index + 1}`}><Trash2 className="h-4 w-4" /></button> : null}
                </div>
                <div className="mt-4 grid gap-3 lg:grid-cols-2 xl:grid-cols-4">
                  <Field label="Nội dung kiểm tra" required error={errors[`${item.local_id}.title`]} className="xl:col-span-2">
                    <input disabled={readOnly} className={controlClass(Boolean(errors[`${item.local_id}.title`]), readOnly)} value={item.title} onChange={(event) => updateItem(setItems, item.local_id, { title: event.target.value })} />
                  </Field>
                  <Field label="Vị trí"><input disabled={readOnly} className={controlClass(false, readOnly)} value={item.location ?? ''} onChange={(event) => updateItem(setItems, item.local_id, { location: event.target.value })} placeholder="Khu vực / tầng" /></Field>
                  <Field label="Thời điểm dự kiến"><input disabled={readOnly} type="datetime-local" className={controlClass(false, readOnly)} value={toDateTimeInput(item.scheduled_at)} onChange={(event) => updateItem(setItems, item.local_id, { scheduled_at: event.target.value })} /></Field>
                  <Field label="Tiêu chí đạt" required error={errors[`${item.local_id}.criteria`]} className="xl:col-span-2"><textarea disabled={readOnly} rows={2} className={controlClass(Boolean(errors[`${item.local_id}.criteria`]), readOnly)} value={criterionText(item)} onChange={(event) => updateCriterion(setItems, item.local_id, event.target.value)} /></Field>
                  <Field label="Vai trò giám sát"><select disabled={readOnly} className={controlClass(false, readOnly)} value={item.responsible_role || 'QA/QC'} onChange={(event) => updateItem(setItems, item.local_id, { responsible_role: event.target.value })}>{!['QA/QC', 'Giám sát', 'PM'].includes(item.responsible_role) && item.responsible_role ? <option value={item.responsible_role}>{item.responsible_role}</option> : null}<option value="QA/QC">QA/QC</option><option value="Giám sát">Giám sát</option><option value="PM">PM</option></select></Field>
                  <Field label="Người giám sát"><select disabled={readOnly} className={controlClass(false, readOnly)} value={item.assigned_user_id ?? ''} onChange={(event) => updateItem(setItems, item.local_id, { assigned_user_id: event.target.value })}><option value="">Theo vai trò</option>{item.assigned_user_id && !(handoff.data?.participants ?? []).some((participant) => participant.id === item.assigned_user_id) ? <option value={item.assigned_user_id}>{identityReferenceText(item.assigned_user_name, item.assigned_user_account_deleted, 'Tài khoản không còn hoạt động')}{item.assigned_user_account_deleted ? ' · Cần phân công lại' : ''}</option> : null}{handoff.data?.participants?.map((participant) => <option key={participant.id} value={participant.id}>{participant.full_name}</option>)}</select></Field>
                </div>
              </article>
            ))}
          </div>
          <div className="flex items-start gap-2 rounded-md border border-blue-200 bg-blue-50 px-3 py-2 text-sm text-blue-800">
            <ShieldCheck className="mt-0.5 h-4 w-4 shrink-0" />
            <span>Mọi hạng mục bắt buộc ít nhất 1 ảnh hoặc tài liệu khi ghi kết quả kiểm tra.</span>
          </div>
        </div>
      </Dialog>
    </>
  );
}

function HandoffLoading() {
  return <section className="rounded-lg border border-line bg-white p-5 shadow-panel"><div className="h-5 w-56 animate-pulse rounded bg-slate-100" /><div className="mt-3 h-4 w-3/4 animate-pulse rounded bg-slate-100" /></section>;
}

function Metric({ label, value, tone }: { label: string; value: number; tone: string }) {
  return <div className="bg-white px-4 py-3"><p className="text-xs font-semibold uppercase tracking-wide text-slate-500">{label}</p><p className={`mt-1 text-xl font-black ${tone}`}>{value}</p></div>;
}

function Field({ label, required, error, className = '', children }: { label: string; required?: boolean; error?: string; className?: string; children: ReactNode }) {
  return <label className={`block ${className}`}><span className="text-xs font-semibold text-slate-600">{label}{required ? <span className="text-rose-600"> *</span> : null}</span><div className="mt-1">{children}</div>{error ? <span className="mt-1 block text-xs text-rose-600">{error}</span> : null}</label>;
}

function controlClass(error = false, disabled = false) {
  return `w-full rounded-md border px-3 py-2 text-sm outline-none focus:ring-2 ${error ? 'border-rose-400 bg-rose-50 focus:ring-rose-200' : 'border-line bg-white focus:ring-blue-500'} ${disabled ? 'cursor-default bg-slate-100 text-slate-600' : ''}`;
}

function editableCandidate(candidate: ConstructionHandoffCandidate): EditableItem {
  return {
    ...candidate,
    local_id: `${candidate.source_field_key}-${candidate.source_item_index}-${crypto.randomUUID()}`,
    evidence_required: true,
    criteria: (candidate.criteria ?? []).map((criterion) => ({ ...criterion, evidence_required: true })),
  };
}

function editableHandoffItem(item: ConstructionHandoffItem): EditableItem {
  return {
    local_id: item.id,
    source_field_key: item.source_field_key,
    source_item_index: item.source_item_index,
    title: item.title,
    location: item.location,
    scheduled_at: item.scheduled_at,
    responsible_role: item.responsible_role_ids?.[0] || 'QA/QC',
    assigned_user_id: item.assigned_user_id,
    assigned_user_name: item.assigned_user_name,
    assigned_user_account_deleted: item.assigned_user_account_deleted,
    evidence_required: item.evidence_required,
    criteria: item.criteria_jsonb ?? [],
  };
}

function emptyItem(current: EditableItem[]): EditableItem {
  const nextIndex = current.reduce((max, item) => Math.max(max, Number(item.source_item_index) || 0), -1) + 1;
  return {
    local_id: crypto.randomUUID(), source_field_key: 'manual_handoff_items', source_item_index: nextIndex,
    title: '', location: '', scheduled_at: '', responsible_role: 'QA/QC', assigned_user_id: '', evidence_required: true,
    criteria: [{ title: '', criteria: '', evidence_required: true }],
  };
}

function updateItem(setItems: Dispatch<SetStateAction<EditableItem[]>>, id: string, values: Partial<EditableItem>) {
  setItems((current) => current.map((item) => item.local_id === id ? { ...item, ...values } : item));
}

function updateCriterion(setItems: Dispatch<SetStateAction<EditableItem[]>>, id: string, value: string) {
  setItems((current) => current.map((item) => item.local_id === id ? { ...item, criteria: [{ title: item.title || value, criteria: value, evidence_required: true }] } : item));
}

function normalizedCriteria(item: EditableItem): ConstructionCheckCriterion[] {
  const text = criterionText(item).trim();
  return [{ title: item.title.trim() || text, criteria: text, evidence_required: true }];
}

function criterionText(item: EditableItem) {
  return String(item.criteria?.[0]?.criteria ?? item.criteria?.[0]?.title ?? '');
}

function toDateTimeInput(value?: string) {
  if (!value) return '';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value.slice(0, 16);
  const local = new Date(date.getTime() - date.getTimezoneOffset() * 60_000);
  return local.toISOString().slice(0, 16);
}

function toISO(value?: string) {
  if (!value) return '';
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? '' : date.toISOString();
}

function handoffMetrics(handoff?: ConstructionHandoff) {
  const items = handoff?.items ?? [];
  return { completed: items.filter((item) => item.status === 'completed' || item.work_item_status === 'completed').length };
}
