import { useEffect, useMemo, useRef, useState } from 'react';
import { useParams, useSearchParams } from 'react-router-dom';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { CheckCircle2, ClipboardCheck, ExternalLink, Eye, FileText, Loader2, Plus, Search, Send, Wrench } from 'lucide-react';
import { downloadBlob, get, post, put } from '../../lib/api/client';
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
import type { AcceptanceChecklistItem, AcceptanceRecord, EvidenceRecord, ProjectOperationsResponse } from '../../lib/types/domain';
import {
  AcceptanceEditor,
  acceptanceDraftFromRecord,
  acceptanceDraftPayload,
  createAcceptanceDraft,
  uploadAcceptanceDraftEvidence,
  type AcceptanceDraft,
} from './AcceptanceEditor';

type ConclusionResult = 'pass' | 'conditional_pass' | 'fail';
type SelectedChecklistItem = { item: AcceptanceChecklistItem; index: number } | null;

const resultOptions: Array<{ value: ConclusionResult; label: string; description: string }> = [
  { value: 'pass', label: 'Đạt', description: 'Biên bản hoàn tất và không mở yêu cầu khắc phục.' },
  { value: 'conditional_pass', label: 'Đạt có điều kiện', description: 'Chấp nhận có điều kiện và theo dõi phần việc còn lại.' },
  { value: 'fail', label: 'Không đạt', description: 'Tạo punch list; yêu cầu sửa lỗi được mở bằng bước tiếp theo.' },
];

export function AcceptancePage({ global = false }: { global?: boolean }) {
  const { id } = useParams();
  const [searchParams] = useSearchParams();
  const requestedRecordId = searchParams.get('record')?.trim() ?? '';
  const appliedRecordId = useRef('');
  const [projectId, setProjectId] = useState(id ?? '');
  const [selectedId, setSelectedId] = useState('');
  const [search, setSearch] = useState('');
  const [editorOpen, setEditorOpen] = useState(false);
  const [draft, setDraft] = useState<AcceptanceDraft>(() => createAcceptanceDraft());
  const [conclusionOpen, setConclusionOpen] = useState(false);
  const [conclusionResult, setConclusionResult] = useState<ConclusionResult>('pass');
  const [conclusionComment, setConclusionComment] = useState('');
  const [selectedChecklistItem, setSelectedChecklistItem] = useState<SelectedChecklistItem>(null);
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
    if (requestedRecordId && appliedRecordId.current !== requestedRecordId) {
      appliedRecordId.current = requestedRecordId;
      if (rows.some((row) => String(row.id) === requestedRecordId)) {
        setSelectedId(requestedRecordId);
        return;
      }
    }
    if (!selectedId || !rows.some((row) => row.id === selectedId)) setSelectedId(String(rows[0].id));
  }, [requestedRecordId, rows, selectedId]);

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
      const initial = acceptanceDraftPayload(draft, submit);
      const { invalid } = initial;
      if (invalid.length) {
        toast.warn({ title: 'Biên bản nghiệm thu chưa hợp lệ', description: invalid.slice(0, 8).join(', ') });
        return null;
      }
      const readyDraft = await uploadAcceptanceDraftEvidence(draft, setDraft);
      const { payload } = acceptanceDraftPayload(readyDraft, submit);
      const record = readyDraft.id
        ? await put<AcceptanceRecord>(`/acceptances/${readyDraft.id}`, payload)
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
      <div className="mx-auto w-full max-w-[1180px]">
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

        <section className="mb-4 overflow-hidden rounded-lg border border-line bg-white">
          <div className="flex flex-col gap-3 border-b border-line p-3 sm:flex-row sm:items-center">
            <label className="flex min-w-0 flex-1 items-center gap-2 rounded-md border border-line bg-white px-3 py-2 text-sm text-slate-500 focus-within:ring-2 focus-within:ring-blue-500">
              <Search className="h-4 w-4 shrink-0" />
              <input className="min-w-0 flex-1 outline-none" value={search} onChange={(event) => setSearch(event.target.value)} placeholder="Tìm mã, tiêu đề, vị trí..." />
            </label>
            <p className="shrink-0 text-xs font-semibold text-slate-500">{filteredRows.length} / {rows.length} biên bản</p>
          </div>
          <div className="max-h-52 overflow-y-auto divide-y divide-line">
            {acceptances.isLoading ? <div className="p-5"><LoadingSpinner /></div> : null}
            {acceptances.error ? <ErrorPanel message="Không tải được danh sách nghiệm thu." onRetry={() => acceptances.refetch()} /> : null}
            {!acceptances.isLoading && !acceptances.error && !filteredRows.length ? <div className="p-5"><EmptyState title={projectId ? (rows.length ? 'Không tìm thấy biên bản phù hợp' : 'Chưa có biên bản nghiệm thu') : 'Chọn công trình để xem nghiệm thu'} /></div> : null}
            {filteredRows.map((row) => (
              <button key={row.id} type="button" className={`grid w-full gap-2 px-4 py-3 text-left transition sm:grid-cols-[minmax(0,1fr)_auto] sm:items-center ${selectedId === row.id ? 'bg-blue-50' : 'hover:bg-slate-50'}`} onClick={() => setSelectedId(String(row.id))}>
                <div className="min-w-0">
                  <div className="flex flex-wrap items-center gap-2">
                    <p className="truncate text-sm font-semibold text-ink">{row.acceptance_code ?? 'Chưa có mã'}</p>
                    <span className="text-xs text-slate-400">·</span>
                    <p className="min-w-0 truncate text-sm text-slate-700">{row.title}</p>
                  </div>
                  <p className="mt-1 truncate text-xs text-slate-500">{row.location || row.work_item_title || 'Chưa ghi vị trí'} · {row.acceptance_date ? dateText(row.acceptance_date) : 'Chưa chốt ngày'}</p>
                </div>
                <StatusBadge status={row.result ?? row.status} />
              </button>
            ))}
          </div>
        </section>

        <main className="min-w-0">
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
              onViewChecklistItem={(item, index) => setSelectedChecklistItem({ item, index })}
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
        <AcceptanceEditor draft={draft} projectId={projectId} evidence={operations.data?.latest_evidence ?? []} workItems={operations.data?.work_items ?? []} disabled={save.isPending} onChange={setDraft} />
        {operations.error ? <p className="mt-3 rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-800">Không tải được danh sách công việc và minh chứng; anh vẫn có thể lưu biên bản chưa chọn Công việc liên quan.</p> : null}
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

      <ChecklistItemDialog
        selected={selectedChecklistItem}
        record={selected}
        onClose={() => setSelectedChecklistItem(null)}
      />
    </>
  );
}

function AcceptanceDetail({ record, canCreate, canConclude, canCreateRework, busy, onEdit, onConclude, onCreateRework, onViewChecklistItem }: {
  record: AcceptanceRecord;
  canCreate: boolean;
  canConclude: boolean;
  canCreateRework: boolean;
  busy: boolean;
  onEdit: () => void;
  onConclude: () => void;
  onCreateRework: () => void;
  onViewChecklistItem: (item: AcceptanceChecklistItem, index: number) => void;
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
        <AcceptanceStatusSummary record={record} />
        <div className="mt-4 grid gap-3 text-sm md:grid-cols-2 xl:grid-cols-4">
          <Info label="Đại diện chủ đầu tư" value={record.client_representative} />
          <Info label="Đại diện nhà thầu" value={record.contractor_representative} />
          <Info label="Công việc liên quan" value={record.work_item_title} />
          <Info label="Hồ sơ / PDF" value={record.pdf_file_name || record.workflow_form_title || record.document_status} />
        </div>
        {record.scope_summary ? <div className="mt-4 rounded-md bg-slate-50 px-3 py-2 text-sm leading-6 text-slate-700"><span className="font-semibold">Phạm vi: </span>{record.scope_summary}</div> : null}
      </section>

      <section className="overflow-hidden rounded-lg border border-line bg-white">
        <div className="flex flex-wrap items-center justify-between gap-3 border-b border-line px-4 py-3">
          <div>
            <h3 className="flex items-center gap-2 font-semibold text-ink"><ClipboardCheck className="h-4 w-4 text-blue-600" /> Checklist nghiệm thu</h3>
            <p className="mt-1 text-xs text-slate-500">{checklist.length} tiêu chí · Đạt {passed} · Có điều kiện {conditional} · Không đạt {failed} · Chờ {pending}</p>
            <p className="mt-1 max-w-3xl text-xs leading-5 text-slate-500">Nguồn checklist: các tiêu chí được lập trực tiếp trong biên bản này; công việc liên quan, hồ sơ và PDF chỉ là liên kết tham chiếu.</p>
          </div>
          <div className="flex flex-wrap gap-2"><SummaryChip label="Đạt" value={passed} tone="green" /><SummaryChip label="Có điều kiện" value={conditional} tone="amber" /><SummaryChip label="Không đạt" value={failed} tone="red" /></div>
        </div>
        {checklist.length ? (
          <>
            <div className="hidden overflow-x-auto md:block">
              <table className="min-w-[980px] w-full text-sm">
                <thead className="bg-slate-50 text-left text-xs uppercase tracking-wide text-slate-500"><tr><th className="px-4 py-3">#</th><th className="px-4 py-3">Hạng mục / tiêu chí</th><th className="px-4 py-3">Kết quả</th><th className="px-4 py-3">Minh chứng</th><th className="px-4 py-3">Yêu cầu xử lý</th><th className="px-4 py-3">Hạn</th><th className="px-4 py-3 text-right">Thao tác</th></tr></thead>
                <tbody className="divide-y divide-line">{checklist.map((item, index) => <tr key={item.id ?? index} className="hover:bg-slate-50"><td className="px-4 py-3 text-slate-500">{index + 1}</td><td className="px-4 py-3"><p className="font-semibold text-ink">{item.title}</p><p className="mt-1 text-xs text-slate-500">{item.description || 'Không có mô tả bổ sung'}</p></td><td className="px-4 py-3"><EnumChip group="result" value={item.decision} /></td><td className="px-4 py-3">{checklistEvidenceCount(item) ? <div><p className="font-medium text-ink">{checklistEvidenceCount(item)} minh chứng</p>{item.evidence_status ? <StatusBadge status={item.evidence_status} /> : null}</div> : item.photo_url ? 'Ảnh đính kèm' : '-'}</td><td className="px-4 py-3 text-slate-700">{item.required_action || '-'}</td><td className="px-4 py-3 text-slate-600">{item.due_date ? dateText(item.due_date) : '-'}</td><td className="px-4 py-3 text-right"><button type="button" className="inline-flex items-center gap-1.5 rounded-md border border-line bg-white px-2.5 py-1.5 text-xs font-semibold text-slate-700 hover:border-blue-200 hover:bg-blue-50 hover:text-blue-700" onClick={() => onViewChecklistItem(item, index)}><Eye className="h-3.5 w-3.5" /> Xem</button></td></tr>)}</tbody>
              </table>
            </div>
            <div className="divide-y divide-line md:hidden">
              {checklist.map((item, index) => (
                <article key={item.id ?? index} className="space-y-3 p-4">
                  <div className="flex items-start justify-between gap-3">
                    <div className="min-w-0"><p className="text-xs font-semibold text-slate-500">Tiêu chí #{index + 1}</p><h4 className="mt-1 font-semibold text-ink">{item.title}</h4><p className="mt-1 text-xs leading-5 text-slate-500">{item.description || 'Không có mô tả bổ sung'}</p></div>
                    <EnumChip group="result" value={item.decision} />
                  </div>
                  <div className="grid gap-2 text-xs text-slate-600 sm:grid-cols-2">
                    <p><span className="font-semibold text-slate-500">Minh chứng: </span>{checklistEvidenceCount(item) ? `${checklistEvidenceCount(item)} minh chứng` : (item.photo_url ? 'Ảnh đính kèm' : '-')}</p>
                    <p><span className="font-semibold text-slate-500">Hạn xử lý: </span>{item.due_date ? dateText(item.due_date) : '-'}</p>
                  </div>
                  <button type="button" className="inline-flex w-full items-center justify-center gap-2 rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:border-blue-200 hover:bg-blue-50 hover:text-blue-700" onClick={() => onViewChecklistItem(item, index)}><Eye className="h-4 w-4" /> Xem chi tiết</button>
                </article>
              ))}
            </div>
          </>
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

function AcceptanceStatusSummary({ record }: { record: AcceptanceRecord }) {
  const status = acceptanceStatusText(record);
  return (
    <div className={`mt-4 flex flex-col gap-1 rounded-md border px-3 py-2.5 sm:flex-row sm:items-center sm:gap-3 ${status.tone}`}>
      <span className="shrink-0 text-xs font-semibold uppercase tracking-wide opacity-70">Trạng thái xử lý</span>
      <span className="text-sm font-semibold">{status.label}</span>
      <span className="hidden text-slate-300 sm:inline">·</span>
      <span className="text-sm opacity-80">{status.description}</span>
    </div>
  );
}

function acceptanceStatusText(record: AcceptanceRecord) {
  if (record.status === 'draft') {
    return { label: 'Đang soạn', description: 'Hoàn thiện checklist và gửi biên bản để chờ kết luận.', tone: 'border-slate-200 bg-slate-50 text-slate-700' };
  }
  if (record.status === 'submitted') {
    return { label: 'Đã gửi', description: 'Biên bản đang chờ người có quyền ghi nhận kết luận.', tone: 'border-blue-200 bg-blue-50 text-blue-800' };
  }
  if (record.status === 'completed' && record.result === 'pass') {
    return { label: 'Đã kết luận: Đạt', description: 'Biên bản đã hoàn tất và không còn phần việc nghiệm thu cần xử lý.', tone: 'border-emerald-200 bg-emerald-50 text-emerald-800' };
  }
  if (record.status === 'completed' && record.result === 'conditional_pass') {
    return { label: 'Đã kết luận: Đạt có điều kiện', description: 'Theo dõi phần việc còn lại hoặc mở yêu cầu khắc phục.', tone: 'border-amber-200 bg-amber-50 text-amber-800' };
  }
  if (record.status === 'completed' && record.result === 'fail') {
    return { label: 'Đã kết luận: Không đạt', description: 'Cần mở hoặc theo dõi yêu cầu khắc phục.', tone: 'border-red-200 bg-red-50 text-red-800' };
  }
  return { label: `Trạng thái: ${record.status}`, description: 'Biên bản không có thao tác tiếp theo tại màn hình này.', tone: 'border-slate-200 bg-slate-50 text-slate-700' };
}

function ChecklistItemDialog({ selected, record, onClose }: { selected: SelectedChecklistItem; record: AcceptanceRecord | null; onClose: () => void }) {
  const toast = useToast();
  const [openingEvidence, setOpeningEvidence] = useState('');
  const item = selected?.item;
  const evidenceRecords = checklistEvidenceRecords(item);
  const supplemental = Object.entries(item?.payload_json ?? {}).filter(([key]) => key !== 'evidence_required');
  const evidenceRequired = Boolean(item?.photo_required || item?.payload_json?.evidence_required || item?.decision === 'conditional_pass' || item?.decision === 'fail');

  async function openEvidence(evidence?: EvidenceRecord) {
    if (!item) return;
    const evidenceId = String(evidence?.id ?? item.evidence_id ?? '');
    if (!evidenceId && item.photo_url) {
      window.open(item.photo_url, '_blank', 'noopener,noreferrer');
      return;
    }
    if (!evidenceId) return;
    const viewer = window.open('', '_blank', 'noopener,noreferrer');
    setOpeningEvidence(evidenceId);
    try {
      const blob = await downloadBlob(`/evidence/${evidenceId}/file`);
      const url = URL.createObjectURL(blob);
      if (viewer) viewer.location.href = url;
      else window.open(url, '_blank', 'noopener,noreferrer');
      window.setTimeout(() => URL.revokeObjectURL(url), 60_000);
    } catch (error) {
      viewer?.close();
      toast.error('Không mở được minh chứng', error);
    } finally {
      setOpeningEvidence('');
    }
  }

  return (
    <Dialog
      open={Boolean(selected)}
      title={selected ? `Chi tiết tiêu chí #${selected.index + 1}` : 'Chi tiết tiêu chí'}
      description={record ? `${record.acceptance_code ?? record.id} · ${record.title}` : undefined}
      size="lg"
      onClose={onClose}
      footer={<div className="flex justify-end"><button type="button" className="rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100" onClick={onClose}>Đóng</button></div>}
    >
      {item ? (
        <div className="space-y-4">
          <div className="flex flex-col gap-3 rounded-md border border-line bg-slate-50 p-4 sm:flex-row sm:items-start sm:justify-between">
            <div className="min-w-0"><h3 className="text-lg font-semibold text-ink">{item.title}</h3><p className="mt-2 whitespace-pre-wrap text-sm leading-6 text-slate-600">{item.description || 'Không có mô tả bổ sung.'}</p></div>
            <div className="flex shrink-0 flex-wrap gap-2"><EnumChip group="result" value={item.decision} /><EnumChip group="severity" value={item.severity ?? 'medium'} /></div>
          </div>

          <div className="grid gap-4 sm:grid-cols-2">
            <DetailField label="Yêu cầu xử lý" value={item.required_action || '-'} />
            <DetailField label="Hạn xử lý" value={item.due_date ? dateText(item.due_date) : '-'} />
            <DetailField label="Yêu cầu ảnh" value={item.photo_required ? 'Có' : 'Không'} />
            <DetailField label="Minh chứng bắt buộc" value={evidenceRequired ? 'Có' : 'Không'} />
          </div>

          <section className="rounded-md border border-line p-4">
            <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">Minh chứng · {evidenceRecords.length || (item.photo_url ? 1 : 0)}</p>
            {evidenceRecords.length ? <div className="mt-3 grid gap-2 sm:grid-cols-2">{evidenceRecords.map((evidence) => <article key={String(evidence.id)} className="flex min-w-0 items-start gap-3 rounded-md border border-line p-3"><div className="min-w-0 flex-1"><p className="truncate text-sm font-semibold text-ink">{evidence.name || evidence.title || evidence.id}</p><p className="mt-1 truncate text-xs text-slate-500">{evidence.work_item_title || 'Hồ sơ công trình'}</p><div className="mt-2"><StatusBadge status={evidence.status} /></div></div><button type="button" disabled={Boolean(openingEvidence)} className="inline-flex shrink-0 items-center gap-2 rounded-md border border-line bg-white px-2.5 py-2 text-xs font-semibold text-slate-700 hover:border-blue-200 hover:bg-blue-50 hover:text-blue-700 disabled:opacity-60" onClick={() => openEvidence(evidence)}>{openingEvidence === String(evidence.id) ? <Loader2 className="h-4 w-4 animate-spin" /> : <ExternalLink className="h-4 w-4" />} Xem</button></article>)}</div> : item.photo_url ? <button type="button" className="mt-3 inline-flex items-center gap-2 rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-50" onClick={() => openEvidence()}><ExternalLink className="h-4 w-4" /> Xem ảnh dữ liệu cũ</button> : <p className="mt-2 text-sm text-slate-500">Chưa có minh chứng.</p>}
          </section>

          <DetailField label="Ghi chú tiêu chí" value={item.note || '-'} full />

          {supplemental.length ? (
            <section className="rounded-md border border-line p-4"><p className="text-xs font-semibold uppercase tracking-wide text-slate-500">Thông tin bổ sung</p><dl className="mt-3 grid gap-3 sm:grid-cols-2">{supplemental.map(([key, value]) => <div key={key}><dt className="text-xs font-semibold text-slate-500">{supplementalLabel(key)}</dt><dd className="mt-1 break-words text-sm text-ink">{supplementalValue(value)}</dd></div>)}</dl></section>
          ) : null}

          <p className="text-xs leading-5 text-slate-500">Nguồn: tiêu chí được lập và lưu trong biên bản {record?.acceptance_code ?? record?.id ?? ''}; Công việc liên quan, hồ sơ và PDF chỉ là liên kết tham chiếu.</p>
        </div>
      ) : null}
    </Dialog>
  );
}

function DetailField({ label, value, full = false }: { label: string; value: string; full?: boolean }) {
  return <div className={full ? 'rounded-md border border-line p-4' : ''}><p className="text-xs font-semibold uppercase tracking-wide text-slate-500">{label}</p><p className="mt-1 whitespace-pre-wrap text-sm leading-6 text-ink">{value}</p></div>;
}

function supplementalLabel(key: string) {
  const labels: Record<string, string> = { verified: 'Đã xác minh', history: 'Lịch sử', ncr: 'NCR liên quan' };
  return labels[key] ?? key.replace(/_/g, ' ');
}

function supplementalValue(value: unknown) {
  if (typeof value === 'boolean') return value ? 'Có' : 'Không';
  if (typeof value === 'string' || typeof value === 'number') return String(value);
  return JSON.stringify(value);
}

function checklistEvidenceRecords(item?: AcceptanceChecklistItem): EvidenceRecord[] {
  if (!item) return [];
  if (item.evidence_records?.length) return item.evidence_records;
  if (!item.evidence_id) return [];
  return [{
    id: item.evidence_id,
    work_item_id: '',
    evidence_type: 'other',
    name: item.evidence_name || item.evidence_id,
    status: item.evidence_status || 'submitted',
  } as EvidenceRecord];
}

function checklistEvidenceCount(item: AcceptanceChecklistItem) {
  return item.evidence_records?.length || item.evidence_ids?.length || (item.evidence_id ? 1 : 0);
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
