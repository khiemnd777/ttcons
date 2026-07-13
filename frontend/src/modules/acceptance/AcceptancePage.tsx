import { useEffect, useMemo, useState } from 'react';
import { useParams } from 'react-router-dom';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { CheckCircle2, ClipboardCheck, FileText, Loader2, Plus, Search, Send, Wrench } from 'lucide-react';
import { get, post, put } from '../../lib/api/client';
import { PageHeader } from '../../components/PageHeader';
import { StatusBadge } from '../../components/StatusBadge';
import { useToast } from '../../components/ToastProvider';
import { ProjectBackLink } from '../../components/ProjectBackLink';
import { ProjectSelector } from '../_shared/ProjectSelector';
import { Dialog } from '../../components/Dialog';
import { EmptyState } from '../../components/EmptyState';
import { LoadingSpinner } from '../../components/LoadingSpinner';
import { EnumChip } from '../../components/EnumChip';
import { dateText } from '../../lib/utils/format';
import { useAuth } from '../../lib/auth/AuthContext';
import type { AcceptanceRecord, ProjectOperationsResponse } from '../../lib/types/domain';
import {
  AcceptanceEditor,
  acceptanceDraftFromRecord,
  acceptanceDraftPayload,
  createAcceptanceDraft,
  type AcceptanceDraft,
} from './AcceptanceEditor';

type ConclusionResult = 'pass' | 'conditional_pass' | 'fail';

const resultOptions: Array<{ value: ConclusionResult; label: string; description: string }> = [
  { value: 'pass', label: 'Đạt', description: 'Biên bản hoàn tất và không mở yêu cầu khắc phục.' },
  { value: 'conditional_pass', label: 'Đạt có điều kiện', description: 'Chấp nhận có điều kiện và theo dõi phần việc còn lại.' },
  { value: 'fail', label: 'Không đạt', description: 'Tạo punch list; yêu cầu sửa lỗi được mở bằng bước tiếp theo.' },
];

export function AcceptancePage({ global = false }: { global?: boolean }) {
  const { id } = useParams();
  const [projectId, setProjectId] = useState(id ?? '');
  const [selectedId, setSelectedId] = useState('');
  const [search, setSearch] = useState('');
  const [editorOpen, setEditorOpen] = useState(false);
  const [draft, setDraft] = useState<AcceptanceDraft>(() => createAcceptanceDraft());
  const [conclusionOpen, setConclusionOpen] = useState(false);
  const [conclusionResult, setConclusionResult] = useState<ConclusionResult>('pass');
  const [conclusionComment, setConclusionComment] = useState('');
  const queryClient = useQueryClient();
  const toast = useToast();
  const { hasAnyPermission } = useAuth();
  const canCreate = hasAnyPermission(['acceptance.create', 'operation.report']);
  const canApprove = hasAnyPermission(['acceptance.approve']);
  const canReject = hasAnyPermission(['acceptance.reject']);
  const canCreateRework = hasAnyPermission(['rework.create', 'acceptance.reject']);

  const projects = useQuery({ queryKey: ['projects'], queryFn: () => get<Record<string, any>[]>('/projects') });
  useEffect(() => { if (!projectId && projects.data?.[0]) setProjectId(projects.data[0].id); }, [projects.data, projectId]);

  const acceptances = useQuery({
    queryKey: ['acceptances', projectId],
    queryFn: () => get<AcceptanceRecord[]>(`/projects/${projectId}/acceptances`),
    enabled: Boolean(projectId),
  });
  const operations = useQuery({
    queryKey: ['project-operations', projectId, 'full'],
    queryFn: () => get<ProjectOperationsResponse>(`/projects/${projectId}/operations`),
    enabled: Boolean(projectId) && canCreate,
    retry: false,
  });
  const rows = acceptances.data ?? [];
  useEffect(() => {
    if (!rows.length) {
      setSelectedId('');
      return;
    }
    if (!selectedId || !rows.some((row) => row.id === selectedId)) setSelectedId(String(rows[0].id));
  }, [rows, selectedId]);

  const detail = useQuery({
    queryKey: ['acceptance', selectedId],
    queryFn: () => get<AcceptanceRecord>(`/acceptances/${selectedId}`),
    enabled: Boolean(selectedId),
    retry: false,
  });
  const selected = detail.data ?? rows.find((row) => row.id === selectedId) ?? null;
  const filteredRows = useMemo(() => {
    const needle = search.trim().toLocaleLowerCase('vi');
    if (!needle) return rows;
    return rows.filter((row) => [row.acceptance_code, row.title, row.location, row.work_item_title, row.result, row.status].some((value) => String(value ?? '').toLocaleLowerCase('vi').includes(needle)));
  }, [rows, search]);
  const summary = useMemo(() => ({
    draft: rows.filter((row) => row.status === 'draft').length,
    submitted: rows.filter((row) => row.status === 'submitted').length,
    conditional: rows.filter((row) => row.result === 'conditional_pass').length,
    failed: rows.filter((row) => row.result === 'fail').length,
  }), [rows]);

  const invalidate = async (acceptanceId?: string) => {
    await queryClient.invalidateQueries({ queryKey: ['acceptances', projectId] });
    if (acceptanceId) await queryClient.invalidateQueries({ queryKey: ['acceptance', acceptanceId] });
    await queryClient.invalidateQueries({ queryKey: ['reworks', projectId] });
    await queryClient.invalidateQueries({ queryKey: ['issues', projectId] });
  };

  const save = useMutation({
    mutationFn: async ({ submit }: { submit: boolean }) => {
      const { payload, invalid } = acceptanceDraftPayload(draft, submit);
      if (invalid.length) {
        toast.warn({ title: 'Biên bản nghiệm thu chưa hợp lệ', description: invalid.slice(0, 8).join(', ') });
        return null;
      }
      const record = draft.id
        ? await put<AcceptanceRecord>(`/acceptances/${draft.id}`, payload)
        : await post<AcceptanceRecord>(`/projects/${projectId}/acceptances`, payload);
      if (!submit) return record;
      return post<AcceptanceRecord>(`/acceptances/${record.id}/submit`, {});
    },
    onSuccess: async (record, variables) => {
      if (!record) return;
      setEditorOpen(false);
      setSelectedId(String(record.id));
      await invalidate(String(record.id));
      toast.success(variables.submit ? 'Đã gửi biên bản nghiệm thu' : 'Đã lưu nháp biên bản nghiệm thu');
    },
    onError: (error) => toast.error('Không lưu được biên bản nghiệm thu', error),
  });

  const conclude = useMutation({
    mutationFn: async () => {
      if (!selectedId) return null;
      if (!conclusionComment.trim()) {
        toast.warn({ title: 'Thiếu kết luận', description: 'Nhập nhận xét kết luận nghiệm thu.' });
        return null;
      }
      const action = conclusionResult === 'conditional_pass' ? 'conditional-pass' : conclusionResult;
      return post<AcceptanceRecord>(`/acceptances/${selectedId}/${action}`, { comment: conclusionComment.trim() });
    },
    onSuccess: async (record) => {
      if (!record) return;
      setConclusionOpen(false);
      setConclusionComment('');
      await invalidate(String(record.id));
      toast.success('Đã ghi nhận kết luận nghiệm thu');
    },
    onError: (error) => toast.error('Không kết luận được nghiệm thu', error),
  });

  const createRework = useMutation({
    mutationFn: () => post(`/acceptances/${selectedId}/create-rework`, {}),
    onSuccess: async () => {
      await invalidate(selectedId);
      toast.success('Đã mở yêu cầu khắc phục từ biên bản nghiệm thu');
    },
    onError: (error) => toast.error('Không mở được yêu cầu khắc phục', error),
  });

  function openNewEditor() {
    setDraft(createAcceptanceDraft());
    setEditorOpen(true);
  }

  function openEditEditor() {
    if (!selected || selected.status !== 'draft') return;
    setDraft(acceptanceDraftFromRecord(selected));
    setEditorOpen(true);
  }

  function openConclusion() {
    if (!selected) return;
    setConclusionResult(canApprove ? 'pass' : 'fail');
    setConclusionComment('');
    setConclusionOpen(true);
  }

  return (
    <>
      <PageHeader
        title="Nghiệm thu"
        description="Soạn, kiểm tra và kết luận biên bản nghiệm thu theo checklist; mỗi trạng thái chỉ hiển thị hành động tiếp theo cần làm."
        actions={<>
          {!global ? <ProjectBackLink projectId={projectId} /> : null}
          {global ? <ProjectSelector value={projectId} onChange={(value) => { setProjectId(value); setSelectedId(''); }} /> : null}
          {canCreate ? <button disabled={!projectId} className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:opacity-50" onClick={openNewEditor}><Plus className="h-4 w-4" /> Tạo biên bản</button> : null}
        </>}
      />

      <div className="mb-4 grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
        <MetricCard label="Nháp" value={summary.draft} detail="Cần hoàn thiện checklist" />
        <MetricCard label="Chờ kết luận" value={summary.submitted} detail="Đã gửi biên bản" tone={summary.submitted ? 'blue' : 'slate'} />
        <MetricCard label="Có điều kiện" value={summary.conditional} detail="Theo dõi phần việc còn lại" tone={summary.conditional ? 'amber' : 'slate'} />
        <MetricCard label="Cần khắc phục" value={summary.failed} detail="Biên bản không đạt" tone={summary.failed ? 'red' : 'slate'} />
      </div>

      <div className="grid min-h-[560px] overflow-hidden rounded-lg border border-line bg-white xl:grid-cols-[360px_minmax(0,1fr)]">
        <aside className="border-b border-line xl:border-b-0 xl:border-r">
          <div className="border-b border-line p-3">
            <label className="flex items-center gap-2 rounded-md border border-line bg-white px-3 py-2 text-sm text-slate-500 focus-within:ring-2 focus-within:ring-blue-500">
              <Search className="h-4 w-4" />
              <input className="min-w-0 flex-1 outline-none" value={search} onChange={(event) => setSearch(event.target.value)} placeholder="Tìm mã, tiêu đề, vị trí..." />
            </label>
            <p className="mt-2 text-xs font-medium text-slate-500">{filteredRows.length} / {rows.length} biên bản</p>
          </div>
          <div className="max-h-[660px] overflow-y-auto">
            {acceptances.isLoading ? <div className="p-5"><LoadingSpinner /></div> : null}
            {acceptances.error ? <ErrorPanel message="Không tải được danh sách nghiệm thu." onRetry={() => acceptances.refetch()} /> : null}
            {!acceptances.isLoading && !acceptances.error && !filteredRows.length ? <div className="p-5"><EmptyState title={projectId ? 'Chưa có biên bản nghiệm thu' : 'Chọn công trình để xem nghiệm thu'} /></div> : null}
            {filteredRows.map((row) => (
              <button key={row.id} type="button" className={`w-full border-b border-line px-4 py-3 text-left transition ${selectedId === row.id ? 'bg-blue-50' : 'hover:bg-slate-50'}`} onClick={() => setSelectedId(String(row.id))}>
                <div className="flex items-start justify-between gap-2">
                  <div className="min-w-0">
                    <p className="truncate text-sm font-semibold text-ink">{row.acceptance_code ?? 'Chưa có mã'}</p>
                    <p className="mt-1 line-clamp-2 text-sm text-slate-700">{row.title}</p>
                  </div>
                  <StatusBadge status={row.result ?? row.status} />
                </div>
                <p className="mt-2 truncate text-xs text-slate-500">{row.location || row.work_item_title || 'Chưa ghi vị trí'} · {row.acceptance_date ? dateText(row.acceptance_date) : 'Chưa chốt ngày'}</p>
              </button>
            ))}
          </div>
        </aside>

        <main className="min-w-0 bg-slate-50/50 p-4 lg:p-5">
          {detail.isLoading && selectedId ? <div className="grid min-h-[420px] place-items-center"><LoadingSpinner /></div> : null}
          {detail.error ? <ErrorPanel message="Không tải được chi tiết biên bản nghiệm thu." onRetry={() => detail.refetch()} /> : null}
          {!selectedId && !detail.isLoading ? <div className="grid min-h-[420px] place-items-center"><EmptyState title="Chọn một biên bản để xem checklist" /></div> : null}
          {selected && !detail.isLoading && !detail.error ? (
            <AcceptanceDetail
              record={selected}
              canCreate={canCreate}
              canConclude={(canApprove || canReject) && selected.status === 'submitted'}
              canCreateRework={canCreateRework}
              busy={conclude.isPending || createRework.isPending}
              onEdit={openEditEditor}
              onConclude={openConclusion}
              onCreateRework={() => createRework.mutate()}
            />
          ) : null}
        </main>
      </div>

      <Dialog
        open={editorOpen}
        title={draft.id ? 'Hoàn thiện biên bản nghiệm thu' : 'Tạo biên bản nghiệm thu'}
        description="Checklist là nội dung chính của biên bản. Mục không đạt hoặc đạt có điều kiện phải có minh chứng và yêu cầu xử lý."
        size="workspace"
        onClose={() => setEditorOpen(false)}
        footer={<div className="flex flex-wrap justify-end gap-2">
          <button className="rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100" onClick={() => setEditorOpen(false)}>Đóng</button>
          <button disabled={save.isPending} className="rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100 disabled:opacity-50" onClick={() => save.mutate({ submit: false })}>Lưu nháp</button>
          <button disabled={save.isPending} className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:opacity-50" onClick={() => save.mutate({ submit: true })}>{save.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <Send className="h-4 w-4" />} Gửi nghiệm thu</button>
        </div>}
      >
        <AcceptanceEditor draft={draft} evidence={operations.data?.latest_evidence ?? []} workItems={operations.data?.work_items ?? []} disabled={save.isPending} onChange={setDraft} />
        {operations.error ? <p className="mt-3 rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-800">Không tải được danh sách đầu việc/minh chứng; anh vẫn có thể lưu biên bản không gắn nguồn.</p> : null}
      </Dialog>

      <Dialog
        open={conclusionOpen}
        title="Kết luận nghiệm thu"
        description={selected ? `${selected.acceptance_code ?? ''} · ${selected.title}` : undefined}
        size="lg"
        onClose={() => setConclusionOpen(false)}
        footer={<div className="flex justify-end gap-2">
          <button className="rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold" onClick={() => setConclusionOpen(false)}>Đóng</button>
          <button disabled={conclude.isPending} className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white disabled:opacity-50" onClick={() => conclude.mutate()}>{conclude.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <CheckCircle2 className="h-4 w-4" />} Xác nhận kết luận</button>
        </div>}
      >
        <div className="space-y-3">
          {resultOptions.filter((option) => option.value === 'fail' ? canReject : canApprove).map((option) => (
            <label key={option.value} className={`flex cursor-pointer gap-3 rounded-md border p-3 ${conclusionResult === option.value ? 'border-blue-300 bg-blue-50' : 'border-line bg-white'}`}>
              <input type="radio" name="acceptance-result" className="mt-1" checked={conclusionResult === option.value} onChange={() => setConclusionResult(option.value)} />
              <span><span className="block text-sm font-semibold text-ink">{option.label}</span><span className="mt-1 block text-xs leading-5 text-slate-600">{option.description}</span></span>
            </label>
          ))}
          <label className="block text-sm font-semibold text-slate-700">Nhận xét kết luận *
            <textarea className="mt-1 min-h-24 w-full rounded-md border border-line px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-500" value={conclusionComment} onChange={(event) => setConclusionComment(event.target.value)} placeholder="Ghi rõ căn cứ kết luận và phần việc cần theo dõi nếu có" />
          </label>
        </div>
      </Dialog>
    </>
  );
}

function AcceptanceDetail({ record, canCreate, canConclude, canCreateRework, busy, onEdit, onConclude, onCreateRework }: {
  record: AcceptanceRecord;
  canCreate: boolean;
  canConclude: boolean;
  canCreateRework: boolean;
  busy: boolean;
  onEdit: () => void;
  onConclude: () => void;
  onCreateRework: () => void;
}) {
  const checklist = record.checklist ?? [];
  const passed = checklist.filter((item) => item.decision === 'pass').length;
  const conditional = checklist.filter((item) => item.decision === 'conditional_pass').length;
  const failed = checklist.filter((item) => item.decision === 'fail').length;
  const pending = checklist.length - passed - conditional - failed;
  return (
    <div className="space-y-4">
      <section className="rounded-lg border border-line bg-white p-4">
        <div className="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
          <div className="min-w-0">
            <div className="flex flex-wrap items-center gap-2"><StatusBadge status={record.result ?? record.status} /><span className="text-xs font-semibold text-slate-500">{record.acceptance_code ?? record.id}</span></div>
            <h2 className="mt-2 text-xl font-semibold text-ink">{record.title}</h2>
            <p className="mt-2 text-sm text-slate-600">{record.location || 'Chưa ghi vị trí'} · {record.acceptance_date ? dateText(record.acceptance_date) : 'Chưa chốt ngày nghiệm thu'}</p>
          </div>
          <NextAction record={record} canCreate={canCreate} canConclude={canConclude} canCreateRework={canCreateRework} busy={busy} onEdit={onEdit} onConclude={onConclude} onCreateRework={onCreateRework} />
        </div>
        <ProgressSteps record={record} />
        <div className="mt-4 grid gap-3 text-sm md:grid-cols-2 xl:grid-cols-4">
          <Info label="Đại diện chủ đầu tư" value={record.client_representative} />
          <Info label="Đại diện nhà thầu" value={record.contractor_representative} />
          <Info label="Đầu việc nguồn" value={record.work_item_title} />
          <Info label="Hồ sơ / PDF" value={record.pdf_file_name || record.workflow_form_title || record.document_status} />
        </div>
        {record.scope_summary ? <div className="mt-4 rounded-md bg-slate-50 px-3 py-2 text-sm leading-6 text-slate-700"><span className="font-semibold">Phạm vi: </span>{record.scope_summary}</div> : null}
      </section>

      <section className="overflow-hidden rounded-lg border border-line bg-white">
        <div className="flex flex-wrap items-center justify-between gap-3 border-b border-line px-4 py-3">
          <div><h3 className="flex items-center gap-2 font-semibold text-ink"><ClipboardCheck className="h-4 w-4 text-blue-600" /> Checklist nghiệm thu</h3><p className="mt-1 text-xs text-slate-500">{checklist.length} tiêu chí · Đạt {passed} · Có điều kiện {conditional} · Không đạt {failed} · Chờ {pending}</p></div>
          <div className="flex flex-wrap gap-2"><SummaryChip label="Đạt" value={passed} tone="green" /><SummaryChip label="Có điều kiện" value={conditional} tone="amber" /><SummaryChip label="Không đạt" value={failed} tone="red" /></div>
        </div>
        {checklist.length ? (
          <div className="overflow-x-auto">
            <table className="min-w-[900px] w-full text-sm">
              <thead className="bg-slate-50 text-left text-xs uppercase tracking-wide text-slate-500"><tr><th className="px-4 py-3">#</th><th className="px-4 py-3">Hạng mục / tiêu chí</th><th className="px-4 py-3">Kết quả</th><th className="px-4 py-3">Minh chứng</th><th className="px-4 py-3">Yêu cầu xử lý</th><th className="px-4 py-3">Hạn</th></tr></thead>
              <tbody className="divide-y divide-line">{checklist.map((item, index) => <tr key={item.id ?? index}><td className="px-4 py-3 text-slate-500">{index + 1}</td><td className="px-4 py-3"><p className="font-semibold text-ink">{item.title}</p><p className="mt-1 text-xs text-slate-500">{item.description || 'Không có mô tả bổ sung'}</p></td><td className="px-4 py-3"><EnumChip group="result" value={item.decision} /></td><td className="px-4 py-3">{item.evidence_name ? <div><p className="font-medium text-ink">{item.evidence_name}</p><StatusBadge status={item.evidence_status ?? 'submitted'} /></div> : item.photo_url ? 'Ảnh đính kèm' : '-'}</td><td className="px-4 py-3 text-slate-700">{item.required_action || '-'}</td><td className="px-4 py-3 text-slate-600">{item.due_date ? dateText(item.due_date) : '-'}</td></tr>)}</tbody>
            </table>
          </div>
        ) : <div className="p-5"><EmptyState title="Biên bản chưa có checklist" /></div>}
      </section>

      {record.notes ? <section className="rounded-lg border border-line bg-white p-4"><h3 className="flex items-center gap-2 text-sm font-semibold text-ink"><FileText className="h-4 w-4" /> Ghi chú biên bản</h3><p className="mt-2 whitespace-pre-wrap text-sm leading-6 text-slate-700">{record.notes}</p></section> : null}
    </div>
  );
}

function NextAction({ record, canCreate, canConclude, canCreateRework, busy, onEdit, onConclude, onCreateRework }: {
  record: AcceptanceRecord; canCreate: boolean; canConclude: boolean; canCreateRework: boolean; busy: boolean;
  onEdit: () => void; onConclude: () => void; onCreateRework: () => void;
}) {
  if (record.status === 'draft' && canCreate) return <button className="inline-flex items-center justify-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700" onClick={onEdit}><ClipboardCheck className="h-4 w-4" /> Tiếp tục biên bản</button>;
  if (record.status === 'submitted' && canConclude) return <button disabled={busy} className="inline-flex items-center justify-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:opacity-50" onClick={onConclude}><CheckCircle2 className="h-4 w-4" /> Kết luận nghiệm thu</button>;
  if (record.status === 'completed' && ['fail', 'conditional_pass'].includes(record.result ?? '') && canCreateRework) return <button disabled={busy} className="inline-flex items-center justify-center gap-2 rounded-md bg-amber-600 px-3 py-2 text-sm font-semibold text-white hover:bg-amber-700 disabled:opacity-50" onClick={onCreateRework}><Wrench className="h-4 w-4" /> Mở yêu cầu khắc phục</button>;
  return <span className="inline-flex items-center gap-2 rounded-md border border-line bg-slate-50 px-3 py-2 text-sm font-semibold text-slate-600"><CheckCircle2 className="h-4 w-4" /> Không còn thao tác</span>;
}

function ProgressSteps({ record }: { record: AcceptanceRecord }) {
  const steps = ['Soạn biên bản', 'Đã gửi', 'Kết luận'];
  if (record.result === 'fail' || record.result === 'conditional_pass') steps.push('Khắc phục');
  const activeIndex = record.status === 'draft' ? 0 : record.status === 'submitted' ? 1 : record.status === 'completed' ? 2 : 0;
  return <ol className="mt-5 grid gap-2 sm:grid-cols-3 lg:grid-cols-4">{steps.map((step, index) => <li key={step} className={`flex items-center gap-2 rounded-md border px-3 py-2 text-xs font-semibold ${index <= activeIndex ? 'border-blue-200 bg-blue-50 text-blue-800' : 'border-line bg-white text-slate-500'}`}><span className={`grid h-5 w-5 place-items-center rounded-full text-[10px] ${index <= activeIndex ? 'bg-blue-600 text-white' : 'bg-slate-200 text-slate-600'}`}>{index + 1}</span>{step}</li>)}</ol>;
}

function MetricCard({ label, value, detail, tone = 'slate' }: { label: string; value: number; detail: string; tone?: 'slate' | 'blue' | 'amber' | 'red' }) {
  const tones = { slate: 'border-line bg-white text-ink', blue: 'border-blue-200 bg-blue-50 text-blue-800', amber: 'border-amber-200 bg-amber-50 text-amber-800', red: 'border-red-200 bg-red-50 text-red-800' };
  return <div className={`rounded-lg border px-4 py-3 ${tones[tone]}`}><p className="text-xs font-semibold uppercase tracking-wide opacity-70">{label}</p><div className="mt-1 flex items-end justify-between gap-3"><span className="text-2xl font-semibold">{value}</span><span className="text-right text-xs opacity-75">{detail}</span></div></div>;
}

function SummaryChip({ label, value, tone }: { label: string; value: number; tone: 'green' | 'amber' | 'red' }) {
  const tones = { green: 'bg-emerald-50 text-emerald-700', amber: 'bg-amber-50 text-amber-700', red: 'bg-red-50 text-red-700' };
  return <span className={`rounded-md px-2 py-1 text-xs font-semibold ${tones[tone]}`}>{label}: {value}</span>;
}

function Info({ label, value }: { label: string; value?: string }) { return <div><p className="text-xs font-semibold uppercase tracking-wide text-slate-500">{label}</p><p className="mt-1 font-medium text-ink">{value || '-'}</p></div>; }
function ErrorPanel({ message, onRetry }: { message: string; onRetry: () => void }) { return <div className="m-4 flex items-center justify-between gap-3 rounded-md border border-red-200 bg-red-50 p-3 text-sm text-red-700"><span>{message}</span><button className="font-semibold underline" onClick={onRetry}>Thử lại</button></div>; }
