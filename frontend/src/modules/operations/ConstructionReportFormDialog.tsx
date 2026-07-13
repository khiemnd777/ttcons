import { useEffect, useMemo, useState, type FormEvent, type ReactNode } from 'react';
import { useQuery } from '@tanstack/react-query';
import { AlertTriangle, Camera, Loader2, Package, Plus, RefreshCw, Trash2, Users } from 'lucide-react';
import clsx from 'clsx';
import { Dialog } from '../../components/Dialog';
import { LoadingSpinner } from '../../components/LoadingSpinner';
import { StatusBadge } from '../../components/StatusBadge';
import { useToast } from '../../components/ToastProvider';
import { get } from '../../lib/api/client';
import type { ProjectEvidencePage, ProjectOperationsResponse, WorkItem } from '../../lib/types/domain';
import { statusLabel } from '../../lib/utils/status';
import { ConstructionEvidenceCapture, type ConstructionEvidenceUploadDraft } from './ConstructionEvidenceCapture';
import { constructionReportEditModel } from './constructionReportViewModel';

type CrewDraft = { crew_name: string; worker_count: string; work_hours: string; note: string };
type MaterialDraft = { material_name: string; quantity: string; unit: string; note: string };
export type ConstructionDailyReportCreatePayload = {
  report_date: string;
  weather: string;
  progress_percent: number;
  work_item_id: string;
  done_items: Array<{ title: string }>;
  current_items: Array<{ title: string }>;
  next_items: Array<{ title: string }>;
  blockers: Array<{ content: string }>;
  field_notes: string;
  crew_items: Array<{ crew_name: string; worker_count: number; work_hours: number; note: string }>;
  material_items: Array<{ material_name: string; quantity: number; unit: string; note: string }>;
  evidence_ids: string[];
  primary_evidence_id: string;
  manpower_summary: string;
  material_usage_summary: string;
};

type ConstructionReportFormDialogProps = {
  open: boolean;
  projectId: string;
  mode?: 'create' | 'edit';
  record?: Record<string, any> | null;
  onClose: () => void;
  onSubmit: (payload: ConstructionDailyReportCreatePayload, uploads: ConstructionEvidenceUploadDraft[]) => Promise<unknown>;
};

export function ConstructionReportFormDialog({ open, projectId, mode = 'create', record = null, onClose, onSubmit }: ConstructionReportFormDialogProps) {
  const toast = useToast();
  const [reportDate, setReportDate] = useState(todayText());
  const [weather, setWeather] = useState('Nắng');
  const [progressPercent, setProgressPercent] = useState('0');
  const [workItemId, setWorkItemId] = useState('');
  const [doneItems, setDoneItems] = useState<string[]>(['']);
  const [currentItems, setCurrentItems] = useState<string[]>(['']);
  const [nextItems, setNextItems] = useState<string[]>(['']);
  const [blockers, setBlockers] = useState<string[]>(['']);
  const [fieldNotes, setFieldNotes] = useState('');
  const [crewItems, setCrewItems] = useState<CrewDraft[]>([emptyCrew()]);
  const [materialItems, setMaterialItems] = useState<MaterialDraft[]>([emptyMaterial()]);
  const [selectedEvidenceIds, setSelectedEvidenceIds] = useState<string[]>([]);
  const [primaryEvidenceId, setPrimaryEvidenceId] = useState('');
  const [manpowerSummary, setManpowerSummary] = useState('');
  const [materialSummary, setMaterialSummary] = useState('');
  const [uploads, setUploads] = useState<ConstructionEvidenceUploadDraft[]>([]);
  const [saving, setSaving] = useState(false);
  const [errors, setErrors] = useState<Record<string, string>>({});

  useEffect(() => {
    if (!open) return;
    const initial = mode === 'edit' && record ? constructionReportEditModel(record) : null;
    setReportDate(initial?.reportDate || todayText());
    setWeather(initial?.weather || 'Nắng');
    setProgressPercent(initial?.progressPercent || '0');
    setWorkItemId(initial?.workItemId || '');
    setDoneItems(initial?.done ?? ['']);
    setCurrentItems(initial?.current ?? ['']);
    setNextItems(initial?.next ?? ['']);
    setBlockers(initial?.blockers ?? ['']);
    setFieldNotes(initial?.fieldNotes ?? '');
    setCrewItems(initial?.crewItems.length ? initial.crewItems : [emptyCrew()]);
    setMaterialItems(initial?.materialItems.length ? initial.materialItems : [emptyMaterial()]);
    setSelectedEvidenceIds(initial?.evidenceIds ?? []);
    setPrimaryEvidenceId(initial?.primaryEvidenceId ?? '');
    setManpowerSummary(initial?.manpowerSummary ?? '');
    setMaterialSummary(initial?.materialSummary ?? '');
    setUploads([]);
    setSaving(false);
    setErrors({});
  }, [mode, open, record]);

  const operations = useQuery({
    queryKey: ['project-operations', projectId, 'full'],
    queryFn: () => get<ProjectOperationsResponse>(`/projects/${projectId}/operations`),
    enabled: open && Boolean(projectId),
    retry: false,
  });
  const workItems = useMemo(
    () => [...(operations.data?.work_items ?? [])]
      .filter((item) => item.status !== 'cancelled')
      .sort((left, right) => workItemRank(left) - workItemRank(right) || left.title.localeCompare(right.title, 'vi')),
    [operations.data?.work_items],
  );
  const evidence = useQuery({
    queryKey: ['construction-report-create-evidence', projectId, workItemId],
    queryFn: () => get<ProjectEvidencePage>(`/projects/${projectId}/evidence?work_item_id=${encodeURIComponent(workItemId)}&limit=50`),
    enabled: open && Boolean(projectId) && Boolean(workItemId),
    retry: false,
  });
  const evidenceOptions = useMemo(() => evidence.data?.items ?? [], [evidence.data?.items]);

  function selectWorkItem(value: string) {
    setWorkItemId(value);
    setSelectedEvidenceIds([]);
    setPrimaryEvidenceId('');
    clearError('work_item_id');
  }

  function toggleEvidence(id: string) {
    setSelectedEvidenceIds((current) => {
      if (current.includes(id)) {
        const next = current.filter((item) => item !== id);
        if (primaryEvidenceId === id) setPrimaryEvidenceId(next[0] ?? '');
        return next;
      }
      if (!primaryEvidenceId) setPrimaryEvidenceId(id);
      return [...current, id];
    });
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const nextErrors = validateReport({ reportDate, weather, progressPercent, doneItems, crewItems, materialItems });
    if (uploads.length && !workItemId) nextErrors.work_item_id = 'Công việc liên quan (bắt buộc khi có minh chứng)';
    if (Object.keys(nextErrors).length) {
      setErrors(nextErrors);
      toast.warn({ title: 'Kiểm tra báo cáo', description: `Thiếu hoặc sai: ${Object.values(nextErrors).join(', ')}` });
      return;
    }
    const payload: ConstructionDailyReportCreatePayload = {
      report_date: reportDate,
      weather: weather.trim(),
      progress_percent: Number(progressPercent),
      work_item_id: workItemId,
      done_items: textRows(doneItems, 'title'),
      current_items: textRows(currentItems, 'title'),
      next_items: textRows(nextItems, 'title'),
      blockers: textRows(blockers, 'content'),
      field_notes: fieldNotes.trim(),
      crew_items: crewItems.filter(hasCrewValue).map((item) => ({
        crew_name: item.crew_name.trim(),
        worker_count: Number(item.worker_count),
        work_hours: Number(item.work_hours || 0),
        note: item.note.trim(),
      })),
      material_items: materialItems.filter(hasMaterialValue).map((item) => ({
        material_name: item.material_name.trim(),
        quantity: Number(item.quantity),
        unit: item.unit.trim(),
        note: item.note.trim(),
      })),
      evidence_ids: selectedEvidenceIds,
      primary_evidence_id: primaryEvidenceId,
      manpower_summary: manpowerSummary,
      material_usage_summary: materialSummary,
    };
    setSaving(true);
    try {
      await onSubmit(payload, uploads);
    } catch {
      // Mutation handlers own the user-facing error toast.
    } finally {
      setSaving(false);
    }
  }

  function clearError(key: string) {
    setErrors((current) => {
      if (!current[key]) return current;
      const { [key]: _removed, ...rest } = current;
      return rest;
    });
  }

  return (
    <Dialog
      open={open}
      title={mode === 'edit' ? 'Chỉnh sửa báo cáo thi công' : 'Báo cáo thi công hôm nay'}
      description="Ghi một lần, đọc nhất quán ở timeline và form chi tiết. Minh chứng luôn được lưu theo công việc liên quan đã chọn."
      size="report"
      onClose={onClose}
      footer={(
        <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
          <button type="button" disabled={saving} className="rounded-md border border-line bg-white px-4 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100 disabled:opacity-60" onClick={onClose}>Hủy</button>
          <button type="submit" form="construction-report-form" disabled={saving} className="inline-flex items-center justify-center gap-2 rounded-md bg-blue-600 px-4 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:opacity-60">
            {saving ? <Loader2 className="h-4 w-4 animate-spin" /> : null}{saving ? 'Đang lưu...' : mode === 'edit' ? 'Lưu thay đổi' : 'Lưu báo cáo'}
          </button>
        </div>
      )}
    >
      <form id="construction-report-form" className="space-y-5" onSubmit={handleSubmit}>
        <section className="rounded-lg border border-blue-200 bg-blue-50/40 p-4">
          <div className="grid gap-4 md:grid-cols-3">
            <Field label="Ngày báo cáo" required error={errors.report_date}>
              <input type="date" className={inputClass(Boolean(errors.report_date))} value={reportDate} onChange={(event) => { setReportDate(event.target.value); clearError('report_date'); }} />
            </Field>
            <Field label="Thời tiết" required error={errors.weather}>
              <select className={inputClass(Boolean(errors.weather))} value={weather} onChange={(event) => { setWeather(event.target.value); clearError('weather'); }}>
                {['Nắng', 'Ít mây', 'Nhiều mây', 'Mưa', 'Gió mạnh', 'Hạn chế mặt bằng'].map((item) => <option key={item} value={item}>{item}</option>)}
              </select>
            </Field>
            <Field label="Tiến độ báo cáo (%)" required error={errors.progress_percent}>
              <input type="number" min={0} max={100} className={inputClass(Boolean(errors.progress_percent))} value={progressPercent} onChange={(event) => { setProgressPercent(event.target.value); clearError('progress_percent'); }} />
            </Field>
          </div>
        </section>

        <section>
          <SectionTitle title="Công việc liên quan" description="Chọn công việc khi báo cáo có minh chứng; lựa chọn này cũng giúp truy vết báo cáo." />
          <div className="mt-3">
            {operations.isLoading ? <LoadingSpinner label="Đang tải danh sách công việc..." /> : null}
            {operations.isError ? <InlineError message="Không tải được danh sách công việc." onRetry={() => operations.refetch()} /> : null}
            {operations.isSuccess ? <Field label="Công việc" error={errors.work_item_id} helper="Không bắt buộc nếu báo cáo không có minh chứng.">
              <select className={inputClass(Boolean(errors.work_item_id))} value={workItemId} onChange={(event) => selectWorkItem(event.target.value)}>
                <option value="">Không chọn công việc</option>
                {workItems.map((item) => <option key={item.id} value={item.id}>{item.title} · {statusLabel(item.status)}</option>)}
              </select>
            </Field> : null}
          </div>
        </section>

        <section>
          <SectionTitle title="Công việc trong ngày" description="Các nhóm này được giữ nguyên khi mở form chi tiết." />
          <div className="mt-3 grid gap-3 md:grid-cols-2">
            <TextListEditor title="Đã làm" required values={doneItems} onChange={(values) => { setDoneItems(values); clearError('done_items'); }} tone="green" error={errors.done_items} addLabel="Thêm hạng mục" />
            <TextListEditor title="Đang làm" values={currentItems} onChange={setCurrentItems} tone="blue" addLabel="Thêm hạng mục" />
            <TextListEditor title="Sẽ làm" values={nextItems} onChange={setNextItems} tone="slate" addLabel="Thêm kế hoạch" />
            <TextListEditor title="Blocker cần xử lý" values={blockers} onChange={setBlockers} tone="red" addLabel="Thêm blocker" />
          </div>
        </section>

        <section>
          <SectionTitle title="Nguồn lực sử dụng" description="Nhân công và vật tư được lưu thành danh sách có cấu trúc." />
          <div className="mt-3 grid gap-3 lg:grid-cols-2">
            <CrewEditor rows={crewItems} onChange={(rows) => { setCrewItems(rows); clearError('crew_items'); }} error={errors.crew_items} />
            <MaterialEditor rows={materialItems} onChange={(rows) => { setMaterialItems(rows); clearError('material_items'); }} error={errors.material_items} />
          </div>
          {manpowerSummary || materialSummary ? <div className="mt-3 rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-xs leading-5 text-amber-800"><strong>Dữ liệu tóm tắt cũ được giữ nguyên:</strong>{manpowerSummary ? ` Nhân công: ${manpowerSummary}.` : ''}{materialSummary ? ` Vật tư: ${materialSummary}.` : ''} Thêm dòng có cấu trúc nếu cần chuẩn hóa báo cáo.</div> : null}
        </section>

        <section>
          <SectionTitle title="Ghi chú hiện trường" description="Thông tin bổ sung, không được tự động tính là blocker." />
          <textarea className={clsx(inputClass(false), 'mt-3 min-h-24')} value={fieldNotes} onChange={(event) => setFieldNotes(event.target.value)} placeholder="Ví dụ: Không có sự cố an toàn; khu vực đã được vệ sinh cuối ca." />
        </section>

        <section>
          <SectionTitle title={`Minh chứng · ${selectedEvidenceIds.length + uploads.length}`} description="Chọn minh chứng đã có hoặc giữ ảnh/PDF trong hàng đợi; minh chứng sẽ được lưu theo công việc liên quan đã chọn." />
          <div className="mt-3 space-y-3">
            {!workItemId ? <InfoState title="Chưa chọn công việc liên quan" description="Bạn vẫn có thể chọn tệp hoặc chụp ảnh. Hệ thống sẽ yêu cầu chọn công việc liên quan khi lưu báo cáo có minh chứng." /> : null}
            {workItemId && evidence.isLoading ? <LoadingSpinner label="Đang tải minh chứng của công việc..." /> : null}
            {workItemId && evidence.isError ? <InlineError message="Không tải được minh chứng của công việc." onRetry={() => evidence.refetch()} /> : null}
            {workItemId && evidence.isSuccess ? (
              <div>
                {evidenceOptions.length ? <div className="grid gap-2 sm:grid-cols-2">
                  {evidenceOptions.map((record) => {
                    const selected = selectedEvidenceIds.includes(record.id);
                    const rejected = record.status === 'rejected';
                    return <article key={record.id} className={clsx('rounded-lg border p-3', selected ? 'border-blue-300 bg-blue-50' : 'border-line bg-white')}>
                      <div className="flex items-start gap-3">
                        <input type="checkbox" className="mt-1 h-4 w-4 rounded border-line" checked={selected} disabled={rejected && !selected} onChange={() => toggleEvidence(record.id)} />
                        <div className="min-w-0 flex-1"><p className="truncate text-sm font-semibold text-ink">{record.name || record.title || 'Minh chứng'}</p><div className="mt-1 flex flex-wrap items-center gap-2"><StatusBadge status={record.status} /><span className="text-xs text-slate-500">{record.evidence_type === 'photo' ? 'Ảnh' : 'Tài liệu'}</span></div></div>
                        {selected ? <label className="flex shrink-0 items-center gap-1 text-xs font-semibold text-blue-700"><input type="radio" name="primary-evidence" checked={primaryEvidenceId === record.id} onChange={() => setPrimaryEvidenceId(record.id)} /> Chính</label> : null}
                      </div>
                    </article>;
                  })}
                </div> : <InfoState title="Công việc này chưa có minh chứng" description="Có thể chọn tệp hoặc chụp ảnh mới ngay bên dưới." />}
              </div>
            ) : null}
            <ConstructionEvidenceCapture drafts={uploads} onChange={setUploads} hasSource={Boolean(workItemId)} disabled={saving} />
          </div>
        </section>
      </form>
    </Dialog>
  );
}

function TextListEditor({ title, required = false, values, onChange, tone, error, addLabel }: { title: string; required?: boolean; values: string[]; onChange: (values: string[]) => void; tone: 'green' | 'blue' | 'slate' | 'red'; error?: string; addLabel: string }) {
  const tones = { green: 'border-emerald-200 bg-emerald-50/30', blue: 'border-blue-200 bg-blue-50/30', slate: 'border-line bg-slate-50/40', red: 'border-red-200 bg-red-50/30' };
  return <article className={clsx('rounded-lg border p-4', tones[tone], error && 'ring-1 ring-rose-300')}><div className="flex items-center justify-between gap-3"><h4 className="text-sm font-semibold text-ink">{title}{required ? <span className="ml-1 text-rose-600">*</span> : null}</h4><span className="rounded-full bg-white px-2 py-0.5 text-xs font-semibold text-slate-600 ring-1 ring-inset ring-line">{values.filter((item) => item.trim()).length}</span></div><div className="mt-3 space-y-2">{values.map((value, index) => <div key={index} className="flex items-start gap-2"><textarea rows={2} className={clsx(inputClass(Boolean(error)), 'mt-0 min-h-16 flex-1 resize-y')} value={value} placeholder={`${title} ${index + 1}`} onChange={(event) => onChange(values.map((item, itemIndex) => itemIndex === index ? event.target.value : item))} /><button type="button" aria-label={`Xóa ${title} ${index + 1}`} disabled={values.length === 1} className="rounded-md p-2 text-slate-400 hover:bg-white hover:text-rose-600 disabled:opacity-30" onClick={() => onChange(values.filter((_item, itemIndex) => itemIndex !== index))}><Trash2 className="h-4 w-4" /></button></div>)}</div>{error ? <p className="mt-2 text-xs font-medium text-rose-700">{error}</p> : null}<button type="button" className="mt-3 inline-flex items-center gap-1.5 text-xs font-semibold text-blue-700" onClick={() => onChange([...values, ''])}><Plus className="h-3.5 w-3.5" /> {addLabel}</button></article>;
}

function CrewEditor({ rows, onChange, error }: { rows: CrewDraft[]; onChange: (rows: CrewDraft[]) => void; error?: string }) {
  return <ResourceEditorFrame icon={<Users className="h-4 w-4" />} title="Nhân công / tổ đội" count={rows.filter(hasCrewValue).length} error={error} onAdd={() => onChange([...rows, emptyCrew()])}><div className="space-y-3">{rows.map((row, index) => <div key={index} className="rounded-md border border-line bg-slate-50 p-3"><div className="grid gap-2 sm:grid-cols-2"><input className={inputClass(Boolean(error))} value={row.crew_name} placeholder="Tên tổ / đội" onChange={(event) => onChange(updateRow(rows, index, { crew_name: event.target.value }))} /><input type="number" min={1} className={inputClass(Boolean(error))} value={row.worker_count} placeholder="Số người" onChange={(event) => onChange(updateRow(rows, index, { worker_count: event.target.value }))} /><input type="number" min={0} step="0.5" className={inputClass(Boolean(error))} value={row.work_hours} placeholder="Giờ công" onChange={(event) => onChange(updateRow(rows, index, { work_hours: event.target.value }))} /><div className="flex gap-2"><input className={inputClass(Boolean(error))} value={row.note} placeholder="Ghi chú" onChange={(event) => onChange(updateRow(rows, index, { note: event.target.value }))} /><button type="button" disabled={rows.length === 1} className="rounded-md p-2 text-slate-400 hover:bg-white hover:text-rose-600 disabled:opacity-30" onClick={() => onChange(rows.filter((_item, itemIndex) => itemIndex !== index))}><Trash2 className="h-4 w-4" /></button></div></div></div>)}</div></ResourceEditorFrame>;
}

function MaterialEditor({ rows, onChange, error }: { rows: MaterialDraft[]; onChange: (rows: MaterialDraft[]) => void; error?: string }) {
  return <ResourceEditorFrame icon={<Package className="h-4 w-4" />} title="Vật tư sử dụng" count={rows.filter(hasMaterialValue).length} error={error} onAdd={() => onChange([...rows, emptyMaterial()])}><div className="space-y-3">{rows.map((row, index) => <div key={index} className="rounded-md border border-line bg-slate-50 p-3"><div className="grid gap-2 sm:grid-cols-2"><input className={inputClass(Boolean(error))} value={row.material_name} placeholder="Tên vật tư" onChange={(event) => onChange(updateRow(rows, index, { material_name: event.target.value }))} /><div className="grid grid-cols-[minmax(0,1fr)_6rem] gap-2"><input type="number" min={0} step="any" className={inputClass(Boolean(error))} value={row.quantity} placeholder="Số lượng" onChange={(event) => onChange(updateRow(rows, index, { quantity: event.target.value }))} /><input className={inputClass(Boolean(error))} value={row.unit} placeholder="ĐVT" onChange={(event) => onChange(updateRow(rows, index, { unit: event.target.value }))} /></div><div className="flex gap-2 sm:col-span-2"><input className={inputClass(Boolean(error))} value={row.note} placeholder="Ghi chú vật tư" onChange={(event) => onChange(updateRow(rows, index, { note: event.target.value }))} /><button type="button" disabled={rows.length === 1} className="rounded-md p-2 text-slate-400 hover:bg-white hover:text-rose-600 disabled:opacity-30" onClick={() => onChange(rows.filter((_item, itemIndex) => itemIndex !== index))}><Trash2 className="h-4 w-4" /></button></div></div></div>)}</div></ResourceEditorFrame>;
}

function ResourceEditorFrame({ icon, title, count, error, onAdd, children }: { icon: ReactNode; title: string; count: number; error?: string; onAdd: () => void; children: ReactNode }) {
  return <article className={clsx('rounded-lg border border-line bg-white p-4', error && 'border-rose-300')}><div className="flex items-center justify-between gap-3"><h4 className="flex items-center gap-2 text-sm font-semibold text-ink"><span className="text-blue-600">{icon}</span>{title}</h4><span className="text-xs font-semibold text-slate-500">{count} dòng</span></div><div className="mt-3">{children}</div>{error ? <p className="mt-2 text-xs font-medium text-rose-700">{error}</p> : null}<button type="button" className="mt-3 inline-flex items-center gap-1.5 text-xs font-semibold text-blue-700" onClick={onAdd}><Plus className="h-3.5 w-3.5" /> Thêm dòng</button></article>;
}

function Field({ label, required = false, error, helper, children }: { label: string; required?: boolean; error?: string; helper?: string; children: ReactNode }) {
  return <label className="block"><span className="text-sm font-semibold text-slate-700">{label}{required ? <span className="ml-1 text-rose-600">*</span> : null}</span>{children}{helper ? <p className="mt-1 text-xs text-slate-500">{helper}</p> : null}{error ? <p className="mt-1 text-xs font-medium text-rose-700">{error}</p> : null}</label>;
}

function SectionTitle({ title, description }: { title: string; description: string }) {
  return <div><h3 className="font-semibold text-ink">{title}</h3><p className="mt-0.5 text-xs leading-5 text-slate-500">{description}</p></div>;
}

function InfoState({ title, description }: { title: string; description: string }) {
  return <div className="rounded-lg border border-dashed border-slate-300 bg-slate-50 p-6 text-center"><Camera className="mx-auto h-5 w-5 text-slate-400" /><p className="mt-2 text-sm font-semibold text-slate-700">{title}</p><p className="mt-1 text-xs text-slate-500">{description}</p></div>;
}

function InlineError({ message, onRetry }: { message: string; onRetry: () => void }) {
  return <div className="flex flex-col gap-3 rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-700 sm:flex-row sm:items-center sm:justify-between"><span className="flex items-start gap-2"><AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" />{message}</span><button type="button" className="inline-flex items-center gap-1.5 self-start font-semibold hover:underline" onClick={onRetry}><RefreshCw className="h-3.5 w-3.5" /> Thử lại</button></div>;
}

function validateReport({ reportDate, weather, progressPercent, doneItems, crewItems, materialItems }: { reportDate: string; weather: string; progressPercent: string; doneItems: string[]; crewItems: CrewDraft[]; materialItems: MaterialDraft[] }) {
  const errors: Record<string, string> = {};
  if (!reportDate) errors.report_date = 'Ngày báo cáo';
  if (!weather.trim()) errors.weather = 'Thời tiết';
  const progress = Number(progressPercent);
  if (progressPercent === '' || Number.isNaN(progress) || progress < 0 || progress > 100) errors.progress_percent = 'Tiến độ từ 0 đến 100';
  if (!doneItems.some((item) => item.trim())) errors.done_items = 'Ít nhất một hạng mục đã làm';
  if (crewItems.some((item) => hasCrewValue(item) && (!item.crew_name.trim() || Number(item.worker_count) < 1 || Number(item.work_hours || 0) < 0))) errors.crew_items = 'Dòng nhân công chưa đủ tên tổ đội hoặc số người';
  if (materialItems.some((item) => hasMaterialValue(item) && (!item.material_name.trim() || item.quantity === '' || Number(item.quantity) < 0 || !item.unit.trim()))) errors.material_items = 'Dòng vật tư chưa đủ tên, số lượng hoặc ĐVT';
  return errors;
}

function textRows<Key extends 'title' | 'content'>(values: string[], key: Key): Array<Record<Key, string>> {
  return values.map((value) => value.trim()).filter(Boolean).map((value) => ({ [key]: value }) as Record<Key, string>);
}

function updateRow<Row>(rows: Row[], index: number, patch: Partial<Row>) {
  return rows.map((row, rowIndex) => rowIndex === index ? { ...row, ...patch } : row);
}

function emptyCrew(): CrewDraft { return { crew_name: '', worker_count: '', work_hours: '', note: '' }; }
function emptyMaterial(): MaterialDraft { return { material_name: '', quantity: '', unit: '', note: '' }; }
function hasCrewValue(item: CrewDraft) { return Boolean(item.crew_name.trim() || item.worker_count || item.work_hours || item.note.trim()); }
function hasMaterialValue(item: MaterialDraft) { return Boolean(item.material_name.trim() || item.quantity || item.unit.trim() || item.note.trim()); }
function todayText() { return new Date().toISOString().slice(0, 10); }
function workItemRank(item: WorkItem) { return ['ready', 'in_progress', 'needs_changes', 'blocked'].includes(item.status) ? 0 : ['waiting_review', 'waiting_approval'].includes(item.status) ? 1 : 2; }
function inputClass(error: boolean) { return clsx('mt-1 w-full rounded-md border bg-white px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-100', error ? 'border-rose-300 bg-rose-50' : 'border-line'); }
