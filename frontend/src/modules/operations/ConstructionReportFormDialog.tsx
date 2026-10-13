import { useEffect, useMemo, useRef, useState, type FormEvent, type ReactNode } from 'react';
import { useQuery } from '@tanstack/react-query';
import { AlertTriangle, ClipboardCheck, Eye, FileText, Image as ImageIcon, Loader2, Package, Plus, RefreshCw, Send, Trash2, Users, Wrench } from 'lucide-react';
import clsx from 'clsx';
import { Dialog } from '../../components/Dialog';
import { emptyBlockerCapture, emptyNextPlan, emptyMaterialArrival, hasMaterialArrival, loadDailyCapture, serializeDailyCapture, serializeDailyWorkRows, supervisionCriteria, validateDailyCapture, validateDailyWorkRows, type BlockerCaptureDraft, type NextPlanDraft, type MaterialArrivalDraft, type SupervisionAssessment } from './constructionDailyCapture';
import { LoadingSpinner } from '../../components/LoadingSpinner';
import { StatusBadge } from '../../components/StatusBadge';
import { useToast } from '../../components/ToastProvider';
import { downloadBlob, get } from '../../lib/api/client';
import type { ConstructionWorkspace, EvidenceRecord, ProjectEvidencePage, ProjectOperationsResponse, WorkItem } from '../../lib/types/domain';
import { openProtectedBlob } from '../../lib/utils/openProtectedBlob';
import { isProtectedStorageProvider } from '../../lib/utils/protectedStorage';
import { statusLabel } from '../../lib/utils/status';
import {
  EvidenceRecordViewerDialog,
  isImageEvidence,
  isInlinePdfEvidence,
  isInlineSpreadsheetEvidence,
  isInlineWordEvidence,
} from '../_shared/EvidenceRecordViewerDialog';
import { useEvidencePreviewUrls } from '../work-items/EvidenceGallery';
import { ConstructionEvidenceCapture, type ConstructionEvidenceUploadDraft } from './ConstructionEvidenceCapture';
import { ConstructionReportLinkedComposer, createReportLinkedDraft, validateReportLinkedDrafts, type ReportLinkedDraft } from './ConstructionReportLinkedComposer';
import { constructionReportEditModel } from './constructionReportViewModel';
import { useWorkReportRecipients } from './useWorkReportRecipients';
import { useValidationNavigation } from '../../lib/utils/formValidationNavigation';
import './constructionReport.css';

type CrewDraft = { row_id: string; crew_name: string; worker_count: string; work_hours: string; note: string; representative: string; contact: string };
type MaterialDraft = { material_name: string; quantity: string; unit: string; note: string };
type BlockerDraft = BlockerCaptureDraft & { id: string };
type ReportEvidenceViewer = { source: 'stored'; record: EvidenceRecord };
export type ConstructionDailyReportCreatePayload = {
  report_date: string;
  weather: string;
  progress_percent: number;
  work_item_id: string;
  done_items: Array<{ title: string }>;
  current_items: Array<{ title: string }>;
  next_items: ReturnType<typeof serializeDailyWorkRows>['next_items'];
  blockers: ReturnType<typeof serializeDailyWorkRows>['blockers'];
  field_notes: string;
  crew_items: Array<{ row_id: string; crew_name: string; worker_count: number; work_hours: number; note: string; representative: string; contact: string }>;
  supervision_assessments: SupervisionAssessment[];
  material_arrival_items: ReturnType<typeof serializeDailyCapture>['material_arrival_items'];
  material_items: Array<{ material_name: string; quantity: number; unit: string; note: string }>;
  evidence_ids: string[];
  primary_evidence_id: string;
  manpower_summary: string;
  material_usage_summary: string;
  delivery: {
    mode: 'draft' | 'submit';
    recipient_id: string;
    comment: string;
  };
};

type ConstructionReportFormDialogProps = {
  open: boolean;
  projectId: string;
  projectName?: string;
  mode?: 'create' | 'edit';
  record?: Record<string, any> | null;
  suggestion?: Record<string, any> | null;
  workspace?: ConstructionWorkspace;
  canCreateCheck?: boolean;
  canCreateAction?: boolean;
  canSubmitWorkReport?: boolean;
  onClose: () => void;
  onSubmit: (payload: ConstructionDailyReportCreatePayload, uploads: ConstructionEvidenceUploadDraft[], linkedDrafts: ReportLinkedDraft[]) => Promise<unknown>;
};

export function ConstructionReportFormDialog({ open, projectId, projectName = '', mode = 'create', record = null, suggestion = null, workspace, canCreateCheck = false, canCreateAction = false, canSubmitWorkReport = false, onClose, onSubmit }: ConstructionReportFormDialogProps) {
  const toast = useToast();
  const [reportDate, setReportDate] = useState(todayText());
  const [weather, setWeather] = useState('Nắng');
  const [progressPercent, setProgressPercent] = useState('0');
  const [workItemId, setWorkItemId] = useState('');
  const [doneItems, setDoneItems] = useState<string[]>(['']);
  const [currentItems, setCurrentItems] = useState<string[]>(['']);
  const [nextItems, setNextItems] = useState<NextPlanDraft[]>([emptyNextPlan()]);
  const [blockers, setBlockers] = useState<BlockerDraft[]>([emptyBlocker()]);
  const [fieldNotes, setFieldNotes] = useState('');
  const [crewItems, setCrewItems] = useState<CrewDraft[]>([emptyCrew()]);
  const [materialItems, setMaterialItems] = useState<MaterialDraft[]>([emptyMaterial()]);
  const [assessments, setAssessments] = useState<SupervisionAssessment[]>(() => loadDailyCapture({}).assessments);
  const [arrivals, setArrivals] = useState<MaterialArrivalDraft[]>([emptyMaterialArrival()]);
  const [selectedEvidenceIds, setSelectedEvidenceIds] = useState<string[]>([]);
  const [primaryEvidenceId, setPrimaryEvidenceId] = useState('');
  const [manpowerSummary, setManpowerSummary] = useState('');
  const [materialSummary, setMaterialSummary] = useState('');
  const [uploads, setUploads] = useState<ConstructionEvidenceUploadDraft[]>([]);
  const [saving, setSaving] = useState(false);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [workGroup, setWorkGroup] = useState<'done' | 'current' | 'next'>('done');
  const [linkedDrafts, setLinkedDrafts] = useState<ReportLinkedDraft[]>([]);
  const [showLinkedErrors, setShowLinkedErrors] = useState(false);
  const [evidencePreviewRetryKey, setEvidencePreviewRetryKey] = useState(0);
  const [evidenceViewer, setEvidenceViewer] = useState<ReportEvidenceViewer | null>(null);
  const [recipientId, setRecipientId] = useState('');
  const [deliveryComment, setDeliveryComment] = useState('');
  const [dirty, setDirty] = useState(false);
  const [discardOpen, setDiscardOpen] = useState(false);
  const [recipientEditing, setRecipientEditing] = useState(false);
  const [validationAttempt, setValidationAttempt] = useState(0);
  const [formSession, setFormSession] = useState(0);
  const initialized = useRef('');
  const formRef = useRef<HTMLFormElement | null>(null);
  const navigateToValidation = useValidationNavigation(formRef);

  useEffect(() => {
    if (!open) { initialized.current = ''; return; }
    const identity = `${projectId}:${mode}:${record?.id ?? 'new'}`;
    if (initialized.current === identity) return;
    initialized.current = identity;
    setFormSession((session) => session + 1);
    const initial = mode === 'edit' && record ? constructionReportEditModel(record) : null;
    const suggested = mode === 'create' && suggestion ? constructionReportEditModel(suggestion) : null;
    setReportDate(initial?.reportDate || todayText());
    setWeather(initial?.weather || suggested?.weather || 'Nắng');
    setProgressPercent(initial?.progressPercent || suggested?.progressPercent || '0');
    setWorkItemId(initial?.workItemId || '');
    setDoneItems(initial?.done ?? ['']);
    setCurrentItems(initial?.current ?? ['']);
    setNextItems(initial?.nextPlanItems.length ? initial.nextPlanItems : [emptyNextPlan()]);
    setBlockers(initial?.blockerItems.length ? initial.blockerItems.map(row => ({ ...row, id: row.row_id })) : [emptyBlocker()]);
    setFieldNotes(initial?.fieldNotes ?? '');
    setCrewItems(initial?.crewItems.length ? initial.crewItems : [emptyCrew()]);
    setMaterialItems(initial?.materialItems.length ? initial.materialItems : [emptyMaterial()]);
    setAssessments(initial?.supervisionAssessments ?? loadDailyCapture({}).assessments);
    setArrivals(initial?.materialArrivalItems.length ? initial.materialArrivalItems : [emptyMaterialArrival()]);
    setSelectedEvidenceIds(initial?.evidenceIds ?? []);
    setPrimaryEvidenceId(initial?.primaryEvidenceId ?? '');
    setManpowerSummary(initial?.manpowerSummary ?? '');
    setMaterialSummary(initial?.materialSummary ?? '');
    setUploads([]);
    setSaving(false);
    setErrors({});
    setWorkGroup('done');
    setLinkedDrafts([]);
    setShowLinkedErrors(false);
    setEvidencePreviewRetryKey(0);
    setEvidenceViewer(null);
    setRecipientId(String(record?.delivery_recipient_id ?? ''));
    setDeliveryComment('');
    setDirty(false);
    setDiscardOpen(false);
    setRecipientEditing(false);
    setValidationAttempt(0);
  }, [mode, open, projectId, record, suggestion]);

  useEffect(() => {
    if (!open || !dirty) return;
    const beforeUnload = (event: BeforeUnloadEvent) => { event.preventDefault(); event.returnValue = ''; };
    window.addEventListener('beforeunload', beforeUnload);
    return () => window.removeEventListener('beforeunload', beforeUnload);
  }, [open, dirty]);

  const operations = useQuery({
    queryKey: ['project-operations', projectId, 'full'],
    queryFn: () => get<ProjectOperationsResponse>(`/projects/${projectId}/operations`),
    enabled: open && Boolean(projectId),
    retry: false,
  });
  const workItems = useMemo(
    () => [...(operations.data?.work_items ?? [])]
      .filter((item) => item.status !== 'cancelled' && item.report_eligible === true)
      .sort((left, right) => workItemRank(left) - workItemRank(right) || left.title.localeCompare(right.title, 'vi')),
    [operations.data?.work_items],
  );
  const selectedWorkItem = workItems.find((item) => item.id === workItemId);
  const willAutoStartWorkItem = mode === 'create' && selectedWorkItem && ['pending', 'ready', 'needs_changes'].includes(selectedWorkItem.status);
  const deliveryStatus = String(record?.delivery_status ?? '');
  const mayDeliver = canSubmitWorkReport && (mode === 'create' || deliveryStatus === '' || deliveryStatus === 'draft' || deliveryStatus === 'needs_changes');
  const recipients = useWorkReportRecipients(projectId, open && mayDeliver);
  useEffect(() => {
    if (!open || !mayDeliver || !recipients.data?.length) return;
    setRecipientId((current) => {
      if (current && recipients.data.some((recipient) => recipient.id === current)) return current;
      return recipients.recommendedRecipient?.id ?? '';
    });
  }, [mayDeliver, open, recipients.data, recipients.recommendedRecipient?.id]);
  const selectedRecipient = recipients.data?.find((recipient) => recipient.id === recipientId);
  const deliverySubmitLabel = deliveryStatus === 'needs_changes'
    ? `Lưu & gửi lại${selectedRecipient ? ` cho ${selectedRecipient.full_name}` : ''}`
    : `Lưu & gửi${selectedRecipient ? ` cho ${selectedRecipient.full_name}` : ''}`;
  useEffect(() => {
    if (!open || mode !== 'create' || workItemId || workItems.length !== 1) return;
    selectWorkItem(workItems[0].id);
  }, [mode, open, workItemId, workItems]);
  const evidence = useQuery({
    queryKey: ['construction-report-create-evidence', projectId, workItemId],
    queryFn: () => get<ProjectEvidencePage>(`/projects/${projectId}/evidence?work_item_id=${encodeURIComponent(workItemId)}&limit=50`),
    enabled: open && Boolean(projectId) && Boolean(workItemId),
    retry: false,
  });
  const evidenceOptions = useMemo(() => evidence.data?.items ?? [], [evidence.data?.items]);
  const selectedEvidence = useMemo(() => evidenceOptions.filter((item) => selectedEvidenceIds.includes(item.id)), [evidenceOptions, selectedEvidenceIds]);
  const previewEvidence = useMemo(() => {
    if (evidenceViewer?.source !== 'stored' || selectedEvidence.some((item) => item.id === evidenceViewer.record.id)) return selectedEvidence;
    return [...selectedEvidence, evidenceViewer.record];
  }, [evidenceViewer, selectedEvidence]);
  const selectedEvidencePreviews = useEvidencePreviewUrls(previewEvidence, evidencePreviewRetryKey);
  const hasPhotoEvidence = selectedEvidence.some((item) => item.evidence_type === 'photo' && item.status !== 'rejected') || uploads.some((item) => item.evidenceType === 'photo');

  function selectWorkItem(value: string) {
    setWorkItemId(value);
    setSelectedEvidenceIds([]);
    setPrimaryEvidenceId('');
    setLinkedDrafts((current) => current.map((draft) => ({ ...draft, work_item_id: value, evidence_id: '', uploads: [] })));
    clearError('work_item_id');
  }

  function toggleEvidence(id: string) {
	    setDirty(true);
	    clearError('evidence_ids');
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

  function setBlockerContent(id: string, content: string) {
    setErrors(current => Object.fromEntries(Object.entries(current).filter(([key]) => key !== `blocker_${id}_content`)));
    const previous = blockers.find((item) => item.id === id)?.content ?? '';
    setBlockers((current) => current.map((item) => item.id === id ? { ...item, content } : item));
    setLinkedDrafts((current) => current.map((draft) => draft.source_blocker_key === id && (draft.auto_title || draft.title === previous) ? { ...draft, title: content, source_label: content, auto_title: true } : draft));
  }

  function setBlockerRelation(blocker: BlockerDraft, kind: ReportLinkedDraft['kind'] | null) {
    setDirty(true);
    setLinkedDrafts((current) => {
      const withoutCurrent = current.filter((draft) => draft.source_blocker_key !== blocker.id);
      if (!kind) return withoutCurrent;
      return [...withoutCurrent, createReportLinkedDraft(kind, { title: blocker.content, workItemId, sourceBlockerKey: blocker.id, sourceLabel: blocker.content })];
    });
  }

  function removeBlocker(id: string) {
    setDirty(true);
    setBlockers((current) => current.length === 1 ? [emptyBlocker()] : current.filter((item) => item.id !== id));
    setLinkedDrafts((current) => current.filter((draft) => draft.source_blocker_key !== id));
  }

  async function openEvidenceFile(record: EvidenceRecord) {
    try {
      if (isProtectedStorageProvider(record.storage_provider) && record.storage_key) {
        await openProtectedBlob(() => downloadBlob(`/evidence/${record.id}/file`));
        return;
      }
      if (record.file_url) {
        window.open(record.file_url, '_blank', 'noopener,noreferrer');
        return;
      }
      toast.warn({ title: 'Minh chứng chưa có tệp', description: 'Bản ghi này không có tệp có thể mở.' });
    } catch (error) {
      toast.error('Không mở được minh chứng', error);
    }
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (saving) return;
    const submitter = (event.nativeEvent as SubmitEvent).submitter as HTMLButtonElement | null;
    const deliveryMode: 'draft' | 'submit' = mayDeliver && submitter?.value === 'submit' ? 'submit' : 'draft';
    const nextErrors = validateReport({ reportDate, weather, progressPercent, doneItems, crewItems, materialItems });
    Object.assign(nextErrors, validateDailyWorkRows(nextItems, blockers));
    const arrivalError = validateDailyCapture(arrivals);
    if (arrivalError) nextErrors.material_arrival_items = arrivalError;
	    if (!workItemId) nextErrors.work_item_id = 'Hạng mục thi công đang thực hiện';
	    if (!hasPhotoEvidence) nextErrors.evidence_ids = 'Ít nhất một ảnh hiện trường';
    if (selectedEvidenceIds.length + uploads.length > 50) nextErrors.evidence_ids = 'Tối đa 50 ảnh/tệp cho mỗi báo cáo';
    if (deliveryMode === 'submit' && !recipientId) nextErrors.delivery_recipient_id = 'Người nhận báo cáo';
    const linkedErrors = mode === 'create' ? validateReportLinkedDrafts(linkedDrafts, workItemId, selectedEvidenceIds.length + uploads.length > 0) : [];
    if (linkedErrors.length) setShowLinkedErrors(true);
    if (Object.keys(nextErrors).length || linkedErrors.length) {
      setErrors(nextErrors);
      setValidationAttempt((attempt) => attempt + 1);
      const linkedSummary = linkedErrors.length ? `${linkedErrors.length} kiểm tra/việc xử lý chưa đủ thông tin` : '';
      toast.warn({ title: 'Kiểm tra báo cáo', description: `Thiếu hoặc sai: ${[...Object.values(nextErrors), linkedSummary].filter(Boolean).join(', ')}` });
      navigateToValidation({ prepare: () => {
        if (nextErrors.done_items) setWorkGroup('done');
        if (Object.keys(nextErrors).some(key => key.startsWith('next_'))) setWorkGroup('next');
      } });
      return;
    }
    const payload: ConstructionDailyReportCreatePayload = {
      report_date: reportDate,
      weather: weather.trim(),
      progress_percent: Number(progressPercent),
      work_item_id: workItemId,
      done_items: textRows(doneItems, 'title'),
      current_items: textRows(currentItems, 'title'),
      ...serializeDailyWorkRows(nextItems, blockers),
      field_notes: fieldNotes.trim(),
      ...serializeDailyCapture(assessments, arrivals),
      crew_items: crewItems.filter(hasCrewValue).map((item) => ({
        row_id: item.row_id,
        crew_name: item.crew_name.trim(),
        worker_count: Number(item.worker_count),
        work_hours: Number(item.work_hours || 0),
        note: item.note.trim(),
        representative: item.representative.trim(),
        contact: item.contact.trim(),
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
      delivery: {
        mode: deliveryMode,
        recipient_id: deliveryMode === 'submit' ? recipientId : '',
        comment: deliveryMode === 'submit' ? deliveryComment.trim() : '',
      },
    };
    setSaving(true);
    try {
      await onSubmit(payload, uploads, mode === 'create' ? linkedDrafts : []);
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

  function closeReportDialog() {
    if (saving) return;
    if (evidenceViewer) {
      setEvidenceViewer(null);
      return;
    }
    if (dirty) { setDiscardOpen(true); return; }
    onClose();
  }

  const viewerRecord = evidenceViewer?.source === 'stored' ? evidenceViewer.record : null;
  const viewerPreviewURL = viewerRecord
      ? selectedEvidencePreviews.urls[viewerRecord.id] || (!viewerRecord.storage_key ? viewerRecord.file_url : undefined)
      : undefined;
  const viewerLoading = Boolean(viewerRecord && selectedEvidencePreviews.loadingIds.has(viewerRecord.id));
  const viewerError = viewerRecord && selectedEvidencePreviews.errorIds.has(viewerRecord.id) ? 'Không tải được ảnh minh chứng.' : undefined;

  return (
    <>
      <Dialog
        open={open}
        title={mode === 'edit' ? 'Chỉnh sửa báo cáo thi công' : 'Báo cáo thi công hôm nay'}
        description={projectName || undefined}
        size="report"
        mobileFullscreen
        manageFocus
        onClose={closeReportDialog}
        footer={(
          <div className={clsx('report-actions', !mayDeliver && 'report-actions-single')}>
            <button type="button" disabled={saving} className="report-cancel rounded-md border border-line bg-white px-4 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100 disabled:opacity-60" onClick={closeReportDialog}>Hủy</button>
            {mayDeliver ? <button type="submit" form="construction-report-form" name="delivery_mode" value="draft" disabled={saving} className="inline-flex items-center justify-center rounded-md border border-blue-200 bg-white px-3 py-2 text-sm font-semibold text-blue-700 hover:bg-blue-50 disabled:opacity-60">Lưu nháp</button> : null}
            <button type="submit" form="construction-report-form" name="delivery_mode" value={mayDeliver ? 'submit' : 'draft'} disabled={saving || (mayDeliver && (recipients.isLoading || !selectedRecipient))} aria-label={mayDeliver ? deliverySubmitLabel : undefined} className="report-primary inline-flex items-center justify-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:opacity-60">
              {saving ? <Loader2 className="h-4 w-4 shrink-0 animate-spin" /> : mayDeliver ? <Send className="h-4 w-4 shrink-0" /> : null}
              <span>{saving ? 'Đang lưu...' : mayDeliver ? (deliveryStatus === 'needs_changes' ? 'Lưu & gửi lại' : 'Lưu & gửi') : mode === 'edit' ? 'Lưu thay đổi' : 'Lưu báo cáo'}</span>
            </button>
          </div>
        )}
      >
        <form key={formSession} ref={formRef} id="construction-report-form" className="construction-report-form min-w-0 space-y-5" noValidate onChangeCapture={() => setDirty(true)} onSubmit={handleSubmit}>
          <fieldset disabled={saving} className="min-w-0 space-y-5">
          <section className="grid min-w-0 gap-4 md:grid-cols-[minmax(0,2fr)_minmax(0,1fr)]">
            <Field label="Hạng mục thi công" required error={errors.work_item_id} helper={willAutoStartWorkItem ? 'Khi lưu báo cáo, hạng mục sẽ chuyển sang “Đang làm”.' : undefined}>
              <select disabled={operations.isLoading || operations.isError || saving} className={inputClass(Boolean(errors.work_item_id))} value={workItemId} onChange={(event) => selectWorkItem(event.target.value)}>
                <option value="">{operations.isLoading ? 'Đang tải hạng mục...' : operations.isError ? 'Không tải được hạng mục' : workItems.length ? 'Chọn hạng mục' : 'Chưa có hạng mục Thi công khả dụng'}</option>
                {workItems.map((item) => <option key={item.id} value={item.id}>{item.title} · {statusLabel(item.status)}</option>)}
              </select>
            </Field>
            <Field label="Ngày báo cáo" required error={errors.report_date}>
              <input type="date" className={inputClass(Boolean(errors.report_date))} value={reportDate} onChange={(event) => { setReportDate(event.target.value); clearError('report_date'); }} />
            </Field>
            {operations.isError ? <InlineError message="Không tải được danh sách công việc." onRetry={() => operations.refetch()} /> : null}
            {operations.isSuccess && !workItems.length ? <p className="text-sm text-amber-800 md:col-span-2">Chưa có hạng mục Thi công khả dụng. Cần hạng mục đang hoạt động để lưu báo cáo.</p> : null}
          </section>

          <section data-validation-error={errors.evidence_ids ? 'true' : undefined} className="min-w-0 scroll-mt-4 rounded-md">
            <SectionTitle title={'Ảnh hiện trường · ' + (selectedEvidenceIds.length + uploads.length)} description="Cần ít nhất một ảnh hiện trường." />
            <div className="mt-3 min-w-0 space-y-3">
              {!workItemId && workItems.length > 1 ? <p className="text-xs text-slate-600">Có thể chụp trước; chọn hạng mục trước khi lưu báo cáo.</p> : null}
              <ConstructionEvidenceCapture uploadScope={`/work-items/${workItemId}/evidence/upload`} drafts={uploads} onChange={(next) => { setUploads(next); setDirty(true); clearError('evidence_ids'); }} hasSource={Boolean(workItemId)} disabled={saving} maxFiles={Math.max(0, 50 - selectedEvidenceIds.length)} hint="Ảnh/tệp sẽ được tải lên khi lưu báo cáo." />
              {workItemId && evidence.isLoading ? <LoadingSpinner label="Đang tải ảnh đã có..." /> : null}
              {workItemId && evidence.isError ? <InlineError message="Không tải được ảnh đã có." onRetry={() => evidence.refetch()} /> : null}
              {workItemId && evidence.isSuccess && evidenceOptions.length ? <details className="min-w-0 rounded-lg border border-line p-3"><summary className="cursor-pointer text-sm font-semibold text-slate-700">Ảnh/tệp đã có · {selectedEvidenceIds.length} đã chọn</summary><div className="mt-3 grid min-w-0 gap-2 md:grid-cols-2">{evidenceOptions.map((record) => <ExistingEvidenceOption key={record.id} record={record} selected={selectedEvidenceIds.includes(record.id)} primary={primaryEvidenceId === record.id} previewURL={selectedEvidencePreviews.urls[record.id]} previewPending={selectedEvidencePreviews.loadingIds.has(record.id)} previewFailed={selectedEvidencePreviews.errorIds.has(record.id)} onToggle={() => toggleEvidence(record.id)} onPrimary={() => { setPrimaryEvidenceId(record.id); setDirty(true); }} onOpen={() => isImageEvidence(record) || isInlinePdfEvidence(record) || isInlineWordEvidence(record) || isInlineSpreadsheetEvidence(record) ? setEvidenceViewer({ source: 'stored', record }) : void openEvidenceFile(record)} onRetry={() => setEvidencePreviewRetryKey((current) => current + 1)} />)}</div></details> : null}
              {errors.evidence_ids ? <p role="alert" className="text-sm font-medium text-rose-700">{errors.evidence_ids}</p> : null}
            </div>
          </section>

          <section data-validation-error={errors.done_items || Object.keys(errors).some(key => key.startsWith('next_')) ? 'true' : undefined} className="min-w-0 scroll-mt-4 rounded-md">
            <SectionTitle title="Công việc trong ngày" />
            <div className="mt-3 grid grid-cols-3 gap-1 rounded-lg border border-line bg-slate-50 p-1" aria-label="Nhóm công việc">
              {([
                ['done', 'Đã làm', doneItems],
                ['current', 'Đang làm', currentItems],
                ['next', 'Sẽ làm', nextItems.map(row => row.title)],
              ] as const).map(([key, label, values]) => <button key={key} type="button" aria-pressed={workGroup === key} className={clsx('min-w-0 rounded-md px-1 py-2 text-sm font-semibold', workGroup === key ? 'bg-white text-blue-700 shadow-sm ring-1 ring-inset ring-line' : 'text-slate-500 hover:text-slate-700')} onClick={() => setWorkGroup(key)}>{label}{key === 'done' ? <span className="text-rose-600"> *</span> : null} <span className="text-xs">{values.filter((item) => item.trim()).length}</span></button>)}
            </div>
            <div className="mt-3">
              {workGroup === 'done' ? <TextListEditor title="Đã làm" required values={doneItems} onChange={(values) => { setDoneItems(values); setDirty(true); clearError('done_items'); }} tone="green" error={errors.done_items} addLabel="Thêm công việc" /> : null}
              {workGroup === 'current' ? <TextListEditor title="Đang làm" values={currentItems} onChange={(values) => { setCurrentItems(values); setDirty(true); }} tone="blue" addLabel="Thêm công việc" /> : null}
              {workGroup === 'next' ? <NextPlanEditor rows={nextItems} errors={errors} onChange={rows => { setNextItems(rows); setDirty(true); setErrors(current => Object.fromEntries(Object.entries(current).filter(([key]) => !key.startsWith('next_')))); }} /> : null}
            </div>
          </section>

          <section className="grid min-w-0 grid-cols-2 gap-3">
            <Field label="Thời tiết" required error={errors.weather}>
              <select className={inputClass(Boolean(errors.weather))} value={weather} onChange={(event) => { setWeather(event.target.value); clearError('weather'); }}>
                {['Nắng', 'Ít mây', 'Nhiều mây', 'Mưa', 'Gió mạnh', 'Hạn chế mặt bằng'].map((item) => <option key={item} value={item}>{item}</option>)}
              </select>
            </Field>
            <Field label="Tiến độ (%)" required error={errors.progress_percent}>
              <input type="number" inputMode="decimal" min={0} max={100} className={inputClass(Boolean(errors.progress_percent))} value={progressPercent} onChange={(event) => { setProgressPercent(event.target.value); clearError('progress_percent'); }} />
            </Field>
          </section>

          <ReportDisclosure title="Vấn đề & xử lý" summary={blockers.filter((item) => item.content.trim()).length + ' vấn đề · ' + linkedDrafts.length + ' việc'} reveal={validationAttempt} forceOpen={showLinkedErrors || blockers.some((item) => Boolean(item.content.trim())) || linkedDrafts.length > 0 || Object.keys(errors).some(key => key.startsWith('blocker'))}>
            <BlockerEditor rows={blockers} errors={errors} onDetails={(id, patch) => { setBlockers(current => current.map(row => row.id === id ? { ...row, ...patch } : row)); setDirty(true); setErrors(current => Object.fromEntries(Object.entries(current).filter(([key]) => !key.startsWith(`blocker_${id}_`)))); }} drafts={linkedDrafts} canCreateCheck={mode === 'create' && canCreateCheck} canCreateAction={mode === 'create' && canCreateAction} disabled={!workspace || saving} onChange={setBlockerContent} onRelation={setBlockerRelation} onRemove={removeBlocker} onAdd={() => { setBlockers((current) => [...current, emptyBlocker()]); setDirty(true); }} />
            {mode === 'create' && (canCreateCheck || canCreateAction) ? <ConstructionReportLinkedComposer embedded mode="batch" drafts={linkedDrafts} onChange={(next) => { setLinkedDrafts(next); setDirty(true); }} workspace={workspace} sourceWorkItemId={workItemId} reportEvidence={selectedEvidence} reportEvidencePending={uploads.length > 0} primaryEvidenceId={primaryEvidenceId} canCreateCheck={canCreateCheck} canCreateAction={canCreateAction} showValidation={showLinkedErrors} /> : null}
          </ReportDisclosure>

          <ReportDisclosure title="Nội dung giám sát" summary={`${assessments.filter(row => row.assessment !== 'not_assessed').length}/12 nội dung đã đánh giá`}>
            <div className="space-y-3">{supervisionCriteria.map(([code, label]) => {
              const row = assessments.find(item => item.code === code)!;
              return <div key={code} className="grid min-w-0 gap-2 md:grid-cols-[1fr_180px_2fr]">
                <label htmlFor={`assessment-${code}`} className="text-sm font-medium">{label}</label>
                <select id={`assessment-${code}`} className={tableInputClass(false)} value={row.assessment} onChange={event => { setAssessments(current => current.map(item => item.code === code ? { ...item, assessment: event.target.value as SupervisionAssessment['assessment'] } : item)); setDirty(true); }}>
                  <option value="not_assessed">Chưa đánh giá</option><option value="pass">Đạt</option><option value="fail">Không đạt</option><option value="not_applicable">Không áp dụng</option>
                </select>
                <input aria-label={`Ghi chú ${label}`} maxLength={1000} className={tableInputClass(false)} value={row.note ?? ''} placeholder="Ghi chú" onChange={event => { setAssessments(current => current.map(item => item.code === code ? { ...item, note: event.target.value } : item)); setDirty(true); }} />
              </div>;
            })}</div>
          </ReportDisclosure>
          <div className="min-w-0 space-y-3">
            <CrewEditor rows={crewItems} onChange={(rows) => { setCrewItems(rows); setDirty(true); clearError('crew_items'); }} error={errors.crew_items} reveal={validationAttempt} />
            <MaterialArrivalEditor rows={arrivals} onChange={rows => { setArrivals(rows); setDirty(true); clearError('material_arrival_items'); }} error={errors.material_arrival_items} reveal={validationAttempt} />
            <MaterialEditor rows={materialItems} onChange={(rows) => { setMaterialItems(rows); setDirty(true); clearError('material_items'); }} error={errors.material_items} reveal={validationAttempt} />
            {manpowerSummary || materialSummary ? <p className="text-xs leading-5 text-slate-600"><strong>Tóm tắt đã ghi:</strong>{manpowerSummary ? ' Nhân công: ' + manpowerSummary + '.' : ''}{materialSummary ? ' Vật tư: ' + materialSummary + '.' : ''}</p> : null}
          </div>
          <ReportDisclosure title="Ghi chú" forceOpen={Boolean(fieldNotes.trim())}>
            <textarea aria-label="Ghi chú hiện trường" className={clsx(inputClass(false), 'min-h-24')} value={fieldNotes} onChange={(event) => setFieldNotes(event.target.value)} placeholder="Thông tin khác cần lưu ý tại hiện trường..." />
          </ReportDisclosure>

          {mayDeliver ? (
            <section data-validation-error={errors.delivery_recipient_id ? 'true' : undefined} className={clsx('min-w-0 scroll-mt-4 border-t pt-4', errors.delivery_recipient_id ? 'border-rose-300' : 'border-line')} aria-label="Gửi báo cáo">
              {deliveryStatus === 'needs_changes' ? <p className="mb-3 rounded-md bg-amber-50 px-3 py-2 text-sm text-amber-800"><strong>Gửi lại sau yêu cầu bổ sung</strong>{record?.delivery_review_comment ? ': ' + String(record.delivery_review_comment) : ''}</p> : null}
              {recipients.isLoading ? <LoadingSpinner label="Đang xác định người nhận báo cáo..." /> : null}
              {recipients.isError ? <InlineError message="Không tải được người nhận báo cáo. Bạn vẫn có thể lưu nháp." onRetry={() => recipients.refetch()} /> : null}
              {recipients.isSuccess && recipients.data.length === 0 ? <p className="text-sm text-amber-800">Chưa có người quản lý đủ quyền nhận báo cáo. Báo cáo chỉ có thể lưu nháp.</p> : null}
              {recipients.isSuccess && recipients.data.length > 0 ? (
                <>
                  {selectedRecipient && !recipientEditing && !errors.delivery_recipient_id ? <div className="flex min-w-0 items-center justify-between gap-3"><p className="min-w-0 break-words text-sm text-slate-700">Gửi đến: <strong>{selectedRecipient.full_name}</strong>{selectedRecipient.position ? ' · ' + selectedRecipient.position : ''}</p><button type="button" className="shrink-0 px-3 py-2 text-sm font-semibold text-blue-700" onClick={() => setRecipientEditing(true)}>Đổi</button></div> : <Field label="Gửi đến" required error={errors.delivery_recipient_id}>
                    <select className={inputClass(Boolean(errors.delivery_recipient_id))} value={recipientId} onChange={(event) => { setRecipientId(event.target.value); clearError('delivery_recipient_id'); }}>
                      <option value="">Chọn người nhận báo cáo</option>
                      {recipients.data.map((recipient) => <option key={recipient.id} value={recipient.id}>{recipient.full_name}{recipient.position ? ' · ' + recipient.position : ''}{recipient.is_recommended ? ' · Đề xuất' : ''}</option>)}
                    </select>
                  </Field>}
                  <details className="mt-2">
                    <summary className="cursor-pointer text-sm text-slate-600">Ghi chú khi gửi (tùy chọn)</summary>
                    <textarea aria-label="Ghi chú khi gửi" className={clsx(inputClass(false), 'min-h-20')} maxLength={2000} value={deliveryComment} onChange={(event) => setDeliveryComment(event.target.value)} placeholder="Nội dung cần người nhận lưu ý..." />
                  </details>
                </>
              ) : null}
            </section>
          ) : null}
          </fieldset>
        </form>
      </Dialog>
      <Dialog open={open && discardOpen} title="Bỏ thay đổi chưa lưu?" description="Nội dung vừa nhập và ảnh chưa tải lên sẽ bị bỏ." size="sm" manageFocus onClose={() => setDiscardOpen(false)} footer={<div className="grid grid-cols-2 gap-2"><button type="button" className="min-h-11 rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold" onClick={() => setDiscardOpen(false)}>Tiếp tục nhập</button><button type="button" className="min-h-11 rounded-md bg-rose-600 px-3 py-2 text-sm font-semibold text-white" onClick={() => { setDiscardOpen(false); setDirty(false); onClose(); }}>Bỏ thay đổi</button></div>}>
        <p className="text-sm text-slate-600">Chọn “Tiếp tục nhập” để quay lại báo cáo.</p>
      </Dialog>
      <EvidenceRecordViewerDialog
        record={viewerRecord}
        previewUrl={viewerPreviewURL}
        previewLoading={viewerLoading}
        previewError={viewerError}
        showWorkItem
        onOpenFile={viewerRecord ? () => void openEvidenceFile(viewerRecord) : undefined}
        onRetry={viewerRecord ? () => setEvidencePreviewRetryKey((current) => current + 1) : undefined}
        onClose={() => setEvidenceViewer(null)}
      />
    </>
  );
}

function ExistingEvidenceOption({ record, selected, primary, previewURL, previewPending, previewFailed, onToggle, onPrimary, onOpen, onRetry }: {
  record: EvidenceRecord;
  selected: boolean;
  primary: boolean;
  previewURL?: string;
  previewPending: boolean;
  previewFailed: boolean;
  onToggle: () => void;
  onPrimary: () => void;
  onOpen: () => void;
  onRetry: () => void;
}) {
  const rejected = record.status === 'rejected';
  const image = isImageEvidence(record);
  const pdf = isInlinePdfEvidence(record);
  const word = isInlineWordEvidence(record);
  const spreadsheet = isInlineSpreadsheetEvidence(record);
  const openLabel = image ? 'Xem ảnh' : pdf ? 'Xem PDF' : word ? 'Xem Word' : spreadsheet ? 'Xem Excel' : 'Mở tệp';
  const typeLabel = image ? 'Ảnh' : pdf ? 'PDF' : word ? 'Word' : spreadsheet ? 'Excel' : 'Tài liệu';
  const title = record.name || record.title || 'Minh chứng';
  return (
    <article className={clsx('min-w-0 rounded-lg border p-3', selected ? 'border-blue-300 bg-blue-50' : 'border-line bg-white')}>
      <div className="flex min-w-0 items-start gap-3">
        <input type="checkbox" className="mt-1 h-4 w-4 shrink-0 rounded border-line" checked={selected} disabled={rejected && !selected} onChange={onToggle} aria-label={`Chọn ${title}`} />
        <button type="button" className="grid h-14 w-16 shrink-0 place-items-center overflow-hidden rounded-md border border-line bg-slate-100 text-slate-400" onClick={onOpen} aria-label={`${openLabel} ${title}`}>
          {previewURL ? <img src={previewURL} alt={title} className="h-full w-full object-cover" /> : previewPending ? <Loader2 className="h-5 w-5 animate-spin text-blue-600" /> : image ? <ImageIcon className={clsx('h-5 w-5', previewFailed && 'text-rose-500')} /> : <FileText className="h-5 w-5" />}
        </button>
        <div className="min-w-0 flex-1">
          <p className="truncate text-sm font-semibold text-ink">{title}</p>
          <div className="mt-1 flex flex-wrap items-center gap-2"><StatusBadge status={record.status} /><span className="text-xs text-slate-500">{typeLabel}</span></div>
          <div className="mt-2 flex flex-wrap items-center gap-3">
            <button type="button" className="inline-flex items-center gap-1 text-xs font-semibold text-blue-700 hover:underline" onClick={onOpen}><Eye className="h-3.5 w-3.5" /> {openLabel}</button>
            {previewFailed ? <button type="button" className="inline-flex items-center gap-1 text-xs font-semibold text-rose-700 hover:underline" onClick={onRetry}><RefreshCw className="h-3.5 w-3.5" /> Thử lại thumbnail</button> : null}
          </div>
        </div>
        {selected ? <label className="flex shrink-0 items-center gap-1 text-xs font-semibold text-blue-700"><input type="radio" name="primary-evidence" checked={primary} onChange={onPrimary} /> Chính</label> : null}
      </div>
    </article>
  );
}

function BlockerEditor({ rows, errors = {}, onDetails, drafts, canCreateCheck, canCreateAction, disabled, onChange, onRelation, onRemove, onAdd }: { rows: BlockerDraft[]; errors?: Record<string, string>; onDetails?: (id: string, patch: Partial<BlockerDraft>) => void; drafts: ReportLinkedDraft[]; canCreateCheck: boolean; canCreateAction: boolean; disabled: boolean; onChange: (id: string, content: string) => void; onRelation: (row: BlockerDraft, kind: ReportLinkedDraft['kind'] | null) => void; onRemove: (id: string) => void; onAdd: () => void }) {
  return <div className="mt-3 space-y-3">{rows.map((row, index) => {
    const linked = drafts.find((draft) => draft.source_blocker_key === row.id);
    return <article key={row.id} className="rounded-lg border border-red-200 bg-red-50/30 p-3"><div className="flex items-start gap-2"><textarea rows={2} className={clsx(inputClass(Boolean(errors[`blocker_${row.row_id}_content`])), 'mt-0 min-h-16 flex-1 resize-y')} value={row.content} aria-invalid={Boolean(errors[`blocker_${row.row_id}_content`]) || undefined} aria-label={`Vấn đề ${index + 1}`} placeholder={`Vấn đề ${index + 1}`} onChange={(event) => onChange(row.id, event.target.value)} /><button type="button" aria-label={`Xóa vấn đề ${index + 1}`} className="rounded-md p-2 text-slate-400 hover:bg-white hover:text-rose-600" onClick={() => onRemove(row.id)}><Trash2 className="h-4 w-4" /></button></div>{errors[`blocker_${row.row_id}_content`] ? <p role="alert" className="text-xs text-rose-700">{errors[`blocker_${row.row_id}_content`]}</p> : null}
      <DailyRowDetails kind="blocker" row={row} rowLabel={`Vấn đề ${index + 1}`} errors={errors} onChange={patch => onDetails?.(row.id, patch)} />
      {canCreateCheck || canCreateAction ? <div className="mt-2 flex flex-wrap items-center gap-1.5"><span className="mr-1 text-[11px] font-semibold uppercase tracking-wide text-slate-500">Bước tiếp theo</span><button type="button" className={clsx('rounded-md border px-2 py-1 text-[11px] font-semibold', !linked ? 'border-slate-300 bg-white text-slate-700' : 'border-transparent text-slate-500')} onClick={() => onRelation(row, null)}>Chỉ ghi nhận</button>{canCreateCheck ? <button type="button" disabled={disabled} className={clsx('inline-flex items-center gap-1 rounded-md border px-2 py-1 text-[11px] font-semibold disabled:opacity-40', linked?.kind === 'check' ? 'border-blue-500 bg-blue-600 text-white' : 'border-blue-200 bg-white text-blue-700')} onClick={() => onRelation(row, 'check')}><ClipboardCheck className="h-3 w-3" /> Tạo kiểm tra</button> : null}{canCreateAction ? <button type="button" disabled={disabled} className={clsx('inline-flex items-center gap-1 rounded-md border px-2 py-1 text-[11px] font-semibold disabled:opacity-40', linked?.kind === 'action' ? 'border-amber-700 bg-amber-700 text-white' : 'border-amber-300 bg-white text-amber-800')} onClick={() => onRelation(row, 'action')}><Wrench className="h-3 w-3" /> Giao xử lý</button> : null}</div> : null}</article>;
  })}{errors.blockers ? <p role="alert" data-validation-error="true" className="text-xs text-rose-700">{errors.blockers}</p> : null}<button type="button" className="inline-flex items-center gap-1.5 text-xs font-semibold text-blue-700" onClick={onAdd}><Plus className="h-3.5 w-3.5" /> Thêm vấn đề</button></div>;
}

function TextListEditor({ title, required = false, values, onChange, tone, error, addLabel }: { title: string; required?: boolean; values: string[]; onChange: (values: string[]) => void; tone: 'green' | 'blue' | 'slate' | 'red'; error?: string; addLabel: string }) {
  const tones = { green: 'border-emerald-200 bg-emerald-50/30', blue: 'border-blue-200 bg-blue-50/30', slate: 'border-line bg-slate-50/40', red: 'border-red-200 bg-red-50/30' };
  return <article className={clsx('rounded-lg border p-4', tones[tone], error && 'ring-1 ring-rose-300')}><div className="flex items-center justify-between gap-3"><h4 className="text-sm font-semibold text-ink">{title}{required ? <span className="ml-1 text-rose-600">*</span> : null}</h4><span className="rounded-full bg-white px-2 py-0.5 text-xs font-semibold text-slate-600 ring-1 ring-inset ring-line">{values.filter((item) => item.trim()).length}</span></div><div className="mt-3 space-y-2">{values.map((value, index) => <div key={index} className="flex items-start gap-2"><textarea rows={2} className={clsx(inputClass(Boolean(error)), 'mt-0 min-h-16 flex-1 resize-y')} value={value} aria-label={`${title} ${index + 1}`} placeholder={`${title} ${index + 1}`} onChange={(event) => onChange(values.map((item, itemIndex) => itemIndex === index ? event.target.value : item))} /><button type="button" aria-label={`Xóa ${title} ${index + 1}`} disabled={values.length === 1} className="rounded-md p-2 text-slate-400 hover:bg-white hover:text-rose-600 disabled:opacity-30" onClick={() => onChange(values.filter((_item, itemIndex) => itemIndex !== index))}><Trash2 className="h-4 w-4" /></button></div>)}</div>{error ? <p className="mt-2 text-xs font-medium text-rose-700">{error}</p> : null}<button type="button" className="mt-3 inline-flex items-center gap-1.5 text-xs font-semibold text-blue-700" onClick={() => onChange([...values, ''])}><Plus className="h-3.5 w-3.5" /> {addLabel}</button></article>;
}

function NextPlanEditor({ rows, errors, onChange }: { rows: NextPlanDraft[]; errors: Record<string, string>; onChange: (rows: NextPlanDraft[]) => void }) {
  return <article className="rounded-lg border border-line p-3"><h4 className="mb-3 text-sm font-semibold">Sẽ làm</h4><div className="space-y-3">{rows.map((row, index) => <div key={row.row_id} className="rounded-md border border-line p-3">
    <div className="flex items-start gap-2"><textarea rows={2} maxLength={180} aria-label={`Sẽ làm ${index + 1}`} aria-invalid={Boolean(errors[`next_${row.row_id}_title`]) || undefined} className={inputClass(Boolean(errors[`next_${row.row_id}_title`]))} value={row.title} onChange={event => onChange(rows.map(item => item.row_id === row.row_id ? { ...item, title: event.target.value } : item))} /><button type="button" aria-label={`Xóa Sẽ làm ${index + 1}`} onClick={() => onChange(rows.filter(item => item.row_id !== row.row_id))} className="p-2 text-slate-400"><Trash2 className="h-4 w-4" /></button></div>
    {errors[`next_${row.row_id}_title`] ? <p role="alert" className="text-xs text-rose-700">{errors[`next_${row.row_id}_title`]}</p> : null}
    <DailyRowDetails kind="next" row={row} rowLabel={`Sẽ làm ${index + 1}`} errors={errors} onChange={patch => onChange(rows.map(item => item.row_id === row.row_id ? { ...item, ...patch } : item))} />
  </div>)}</div>{errors.next_items ? <p role="alert" className="text-xs text-rose-700">{errors.next_items}</p> : null}<button type="button" onClick={() => onChange([...rows, emptyNextPlan()])} className="mt-3 inline-flex items-center gap-1 text-xs font-semibold text-blue-700"><Plus className="h-3.5 w-3.5" /> Thêm kế hoạch</button></article>;
}

function DailyRowDetails({ kind, row, rowLabel, errors, onChange }: { kind: 'next' | 'blocker'; row: NextPlanDraft | BlockerCaptureDraft; rowLabel: string; errors: Record<string, string>; onChange: (patch: Record<string, string>) => void }) {
  const fields: Array<[string, string, string, number]> = kind === 'next' ? [['instruction_code', 'Mã chỉ dẫn', 'text', 80], ['location', 'Vị trí', 'text', 200], ['subcontractor', 'Thầu phụ / tổ đội', 'text', 160], ['receiver', 'Người nhận', 'text', 120], ['contact', 'Liên hệ', 'text', 100], ['issued_date', 'Ngày giao', 'date', 10], ['due_date', 'Hạn thực hiện', 'date', 10], ['progress_percent', 'Tiến độ (%)', 'number', 10], ['state', 'Tình trạng đã ghi nhận', 'text', 80]] : [['risk_code', 'Mã rủi ro', 'text', 80], ['detected_at', 'Ngày phát hiện', 'date', 10], ['cause', 'Nguyên nhân', 'textarea', 1000], ['impact', 'Ảnh hưởng', 'textarea', 1000], ['severity', 'Mức độ rủi ro', 'select', 20], ['proposed_action', 'Biện pháp đề xuất', 'textarea', 1000], ['owner', 'Người phụ trách', 'text', 160], ['target_date', 'Hạn xử lý', 'date', 10], ['status', 'Tình trạng đã ghi nhận', 'text', 80]];
  const prefix = `${kind}_${row.row_id}_`;
  return <ReportDisclosure title={kind === 'next' ? 'Chi tiết kế hoạch' : 'Chi tiết vấn đề'} forceOpen={Object.keys(errors).some(key => key.startsWith(prefix))}><div className="grid min-w-0 gap-3 md:grid-cols-2">{fields.map(([key, label, type, max]) => {
    const error = errors[prefix + key]; const value = (row as Record<string, string>)[key] ?? '';
    const props = { 'aria-label': `${label} · ${rowLabel}`, 'aria-invalid': Boolean(error) || undefined, className: inputClass(Boolean(error)), value, onChange: (event: { target: { value: string } }) => onChange({ [key]: event.target.value }) };
    return <Field key={key} label={label} error={error}>{type === 'select' ? <select {...props}><option value="">Chưa đánh giá</option><option value="low">Thấp</option><option value="medium">Trung bình</option><option value="high">Cao</option><option value="critical">Nghiêm trọng</option></select> : type === 'textarea' ? <textarea {...props} maxLength={max} rows={2} /> : <input {...props} type={type} maxLength={max} min={type === 'number' ? 0 : undefined} max={type === 'number' ? 100 : undefined} step={type === 'number' ? 'any' : undefined} />}</Field>;
  })}</div></ReportDisclosure>;
}

function CrewEditor({ rows, onChange, error, reveal }: { rows: CrewDraft[]; onChange: (rows: CrewDraft[]) => void; error?: string; reveal: number }) {
  return (
    <ResourceEditorFrame icon={<Users className="h-4 w-4" />} title="Nhân công / tổ đội" count={rows.filter(hasCrewValue).length} error={error} reveal={reveal} onAdd={() => onChange([...rows, emptyCrew()])}>
      <div className="report-resource-scroll consflow-table-viewport min-w-0 max-w-full rounded-md border border-line">
        <table role="table" className="report-resource-table w-full min-w-[760px] border-collapse text-sm">
          <thead className="bg-slate-50 text-left text-xs font-semibold text-slate-600">
            <tr>
              <th className="w-12 border-b border-line px-2 py-2"><span className="sr-only">Thao tác</span></th>
              <th className="w-12 border-b border-line px-2 py-2">#</th>
              <th className="min-w-52 border-b border-line px-2 py-2">Tên tổ / đội <span className="text-rose-600">*</span></th>
              <th className="w-32 border-b border-line px-2 py-2">Số người <span className="text-rose-600">*</span></th>
              <th className="w-32 border-b border-line px-2 py-2">Giờ công</th>
              <th className="min-w-60 border-b border-line px-2 py-2">Ghi chú</th>
              <th className="min-w-44 border-b border-line px-2 py-2">Đại diện</th>
              <th className="min-w-44 border-b border-line px-2 py-2">Liên hệ 24/7</th>
            </tr>
          </thead>
          <tbody role="rowgroup" className="divide-y divide-line bg-white">
            {rows.map((row, index) => (
              <tr role="row" key={index}>
                <td role="cell" className="px-2 py-2 align-top"><button type="button" disabled={rows.length === 1} className="rounded-md p-2 text-slate-400 hover:bg-rose-50 hover:text-rose-600 disabled:opacity-30" onClick={() => onChange(rows.filter((_item, itemIndex) => itemIndex !== index))} aria-label={`Xóa dòng nhân công ${index + 1}`}><Trash2 className="h-4 w-4" /></button></td>
                <td role="cell" className="px-2 py-3 align-top text-xs font-semibold text-slate-500"><span className="md:hidden">Dòng </span>{index + 1}</td>
                <td role="cell" data-label="Tên tổ / đội *" className="px-2 py-2 align-top"><input className={tableInputClass(Boolean(error))} value={row.crew_name} placeholder="Tên tổ / đội" onChange={(event) => onChange(updateRow(rows, index, { crew_name: event.target.value }))} aria-label={`Tên tổ đội dòng ${index + 1}`} /></td>
                <td role="cell" data-label="Số người *" className="px-2 py-2 align-top"><input type="number" inputMode="numeric" min={1} className={tableInputClass(Boolean(error))} value={row.worker_count} placeholder="Số người" onChange={(event) => onChange(updateRow(rows, index, { worker_count: event.target.value }))} aria-label={`Số người dòng ${index + 1}`} /></td>
                <td role="cell" data-label="Giờ công" className="px-2 py-2 align-top"><input type="number" inputMode="decimal" min={0} step="0.5" className={tableInputClass(Boolean(error))} value={row.work_hours} placeholder="Giờ công" onChange={(event) => onChange(updateRow(rows, index, { work_hours: event.target.value }))} aria-label={`Giờ công dòng ${index + 1}`} /></td>
                <td role="cell" data-label="Ghi chú" className="px-2 py-2 align-top"><input className={tableInputClass(Boolean(error))} value={row.note} placeholder="Ghi chú" onChange={(event) => onChange(updateRow(rows, index, { note: event.target.value }))} aria-label={`Ghi chú nhân công dòng ${index + 1}`} /></td>
                <td role="cell" data-label="Đại diện" className="px-2 py-2 align-top"><input maxLength={120} className={tableInputClass(false)} value={row.representative ?? ''} onChange={event => onChange(updateRow(rows, index, { representative: event.target.value }))} aria-label={`Đại diện tổ đội dòng ${index + 1}`} /></td>
                <td role="cell" data-label="Liên hệ 24/7" className="px-2 py-2 align-top"><input maxLength={100} className={tableInputClass(false)} value={row.contact ?? ''} onChange={event => onChange(updateRow(rows, index, { contact: event.target.value }))} aria-label={`Liên hệ tổ đội dòng ${index + 1}`} /></td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </ResourceEditorFrame>
  );
}

function MaterialArrivalEditor({ rows, onChange, error, reveal }: { rows: MaterialArrivalDraft[]; onChange: (rows: MaterialArrivalDraft[]) => void; error?: string; reveal: number }) {
  const fields = [['material_name', 'Tên vật tư', 160], ['specification', 'Quy cách', 300], ['origin', 'Xuất xứ', 200], ['quantity', 'Số lượng', 30], ['unit', 'Đơn vị', 30], ['note', 'Ghi chú', 800]] as const;
  return <ResourceEditorFrame icon={<Package className="h-4 w-4" />} title="Vật tư nhập trường" count={rows.filter(hasMaterialArrival).length} error={error} reveal={reveal} onAdd={() => onChange([...rows, emptyMaterialArrival()])}>
    <div className="space-y-3">{rows.map((row, index) => <div key={row.row_id} className="rounded-md border border-line p-3">
      <div className="mb-2 flex items-center justify-between"><span className="text-xs font-semibold">Dòng {index + 1}</span><button type="button" aria-label={`Xóa vật tư nhập trường dòng ${index + 1}`} onClick={() => onChange(rows.filter(item => item.row_id !== row.row_id))}><Trash2 className="h-4 w-4 text-slate-400" /></button></div>
      <div className="grid min-w-0 gap-2 md:grid-cols-3">{fields.map(([key, label, max]) => {
        const unitInvalid = Boolean(error && key === 'unit' && row.quantity.trim() !== '' && !row.unit.trim());
        const unitErrorId = `arrival-unit-error-${row.row_id}`;
        return <label key={key} className="text-xs font-medium">{label}<input aria-label={`${label} nhập trường dòng ${index + 1}`} aria-invalid={unitInvalid || undefined} aria-describedby={unitInvalid ? unitErrorId : undefined} type={key === 'quantity' ? 'number' : 'text'} min={key === 'quantity' ? 0 : undefined} step={key === 'quantity' ? 'any' : undefined} maxLength={max} className={inputClass(Boolean(error))} value={row[key]} onChange={event => onChange(updateRow(rows, index, { [key]: event.target.value }))} />{unitInvalid ? <span id={unitErrorId} role="alert" className="mt-1 block text-rose-700">Nhập đơn vị khi có số lượng.</span> : null}</label>;
      })}</div>
    </div>)}</div>
  </ResourceEditorFrame>;
}

function MaterialEditor({ rows, onChange, error, reveal }: { rows: MaterialDraft[]; onChange: (rows: MaterialDraft[]) => void; error?: string; reveal: number }) {
  return (
    <ResourceEditorFrame icon={<Package className="h-4 w-4" />} title="Vật tư sử dụng" count={rows.filter(hasMaterialValue).length} error={error} reveal={reveal} onAdd={() => onChange([...rows, emptyMaterial()])}>
      <div className="report-resource-scroll consflow-table-viewport min-w-0 max-w-full rounded-md border border-line">
        <table role="table" className="report-resource-table w-full min-w-[760px] border-collapse text-sm">
          <thead className="bg-slate-50 text-left text-xs font-semibold text-slate-600">
            <tr>
              <th className="w-12 border-b border-line px-2 py-2"><span className="sr-only">Thao tác</span></th>
              <th className="w-12 border-b border-line px-2 py-2">#</th>
              <th className="min-w-56 border-b border-line px-2 py-2">Tên vật tư <span className="text-rose-600">*</span></th>
              <th className="w-36 border-b border-line px-2 py-2">Số lượng <span className="text-rose-600">*</span></th>
              <th className="w-28 border-b border-line px-2 py-2">ĐVT <span className="text-rose-600">*</span></th>
              <th className="min-w-60 border-b border-line px-2 py-2">Ghi chú</th>
            </tr>
          </thead>
          <tbody role="rowgroup" className="divide-y divide-line bg-white">
            {rows.map((row, index) => (
              <tr role="row" key={index}>
                <td role="cell" className="px-2 py-2 align-top"><button type="button" disabled={rows.length === 1} className="rounded-md p-2 text-slate-400 hover:bg-rose-50 hover:text-rose-600 disabled:opacity-30" onClick={() => onChange(rows.filter((_item, itemIndex) => itemIndex !== index))} aria-label={`Xóa dòng vật tư ${index + 1}`}><Trash2 className="h-4 w-4" /></button></td>
                <td role="cell" className="px-2 py-3 align-top text-xs font-semibold text-slate-500"><span className="md:hidden">Dòng </span>{index + 1}</td>
                <td role="cell" data-label="Tên vật tư *" className="px-2 py-2 align-top"><input className={tableInputClass(Boolean(error))} value={row.material_name} placeholder="Tên vật tư" onChange={(event) => onChange(updateRow(rows, index, { material_name: event.target.value }))} aria-label={`Tên vật tư dòng ${index + 1}`} /></td>
                <td role="cell" data-label="Số lượng *" className="px-2 py-2 align-top"><input type="number" inputMode="decimal" min={0} step="any" className={tableInputClass(Boolean(error))} value={row.quantity} placeholder="Số lượng" onChange={(event) => onChange(updateRow(rows, index, { quantity: event.target.value }))} aria-label={`Số lượng vật tư dòng ${index + 1}`} /></td>
                <td role="cell" data-label="ĐVT *" className="px-2 py-2 align-top"><input className={tableInputClass(Boolean(error))} value={row.unit} placeholder="ĐVT" onChange={(event) => onChange(updateRow(rows, index, { unit: event.target.value }))} aria-label={`Đơn vị tính dòng ${index + 1}`} /></td>
                <td role="cell" data-label="Ghi chú" className="px-2 py-2 align-top"><input className={tableInputClass(Boolean(error))} value={row.note} placeholder="Ghi chú vật tư" onChange={(event) => onChange(updateRow(rows, index, { note: event.target.value }))} aria-label={`Ghi chú vật tư dòng ${index + 1}`} /></td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </ResourceEditorFrame>
  );
}

function ResourceEditorFrame({ icon, title, count, error, reveal, onAdd, children }: { icon: ReactNode; title: string; count: number; error?: string; reveal: number; onAdd: () => void; children: ReactNode }) {
  const contentRef = useRef<HTMLDivElement>(null);
  function addRow() {
    onAdd();
    requestAnimationFrame(() => {
      const rows = contentRef.current?.querySelectorAll('tbody tr');
      rows?.[rows.length - 1]?.querySelector<HTMLInputElement>('input')?.focus();
    });
  }
  return <ReportDisclosure title={title} icon={icon} summary={count + ' dòng'} forceOpen={count > 0 || Boolean(error)} reveal={reveal}>
    <div ref={contentRef} data-validation-error={error ? 'true' : undefined} className="min-w-0 scroll-mt-4">
      {children}
      {error ? <p role="alert" className="mt-2 text-sm font-medium text-rose-700">{error}</p> : null}
      <button type="button" className="mt-3 inline-flex items-center gap-1.5 rounded-md px-2 py-2 text-sm font-semibold text-blue-700" onClick={addRow}><Plus className="h-4 w-4" /> Thêm dòng</button>
    </div>
  </ReportDisclosure>;
}

function ReportDisclosure({ title, icon, summary, forceOpen = false, reveal = 0, children }: { title: string; icon?: ReactNode; summary?: string; forceOpen?: boolean; reveal?: number; children: ReactNode }) {
  const [expanded, setExpanded] = useState(forceOpen);
  useEffect(() => { if (forceOpen) setExpanded(true); }, [forceOpen, reveal]);
  return <details className="report-disclosure min-w-0 rounded-lg border border-line" open={expanded} onToggle={(event) => setExpanded(event.currentTarget.open)}>
    <summary className="cursor-pointer px-3 py-2 font-semibold text-ink">{icon ? <span className="mr-2 inline-block align-middle text-blue-600">{icon}</span> : null}{title}{summary ? <span className="ml-2 text-xs font-normal text-slate-500">{summary}</span> : null}</summary>
    <div className="min-w-0 space-y-4 border-t border-line p-3">{children}</div>
  </details>;
}

function Field({ label, required = false, error, helper, children }: { label: string; required?: boolean; error?: string; helper?: string; children: ReactNode }) {
  return <label data-validation-error={error ? 'true' : undefined} className="block min-w-0 scroll-mt-4 rounded-md"><span className="text-sm font-semibold text-slate-700">{label}{required ? <span className="ml-1 text-rose-600">*</span> : null}</span>{children}{helper ? <p className="mt-1 text-xs text-slate-500">{helper}</p> : null}{error ? <p role="alert" className="mt-1 text-xs font-medium text-rose-700">{error}</p> : null}</label>;
}

function SectionTitle({ title, description }: { title: string; description?: string }) {
  return <div><h3 className="font-semibold text-ink">{title}</h3>{description ? <p className="mt-0.5 text-xs leading-5 text-slate-500">{description}</p> : null}</div>;
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

function emptyCrew(): CrewDraft { return { row_id: crypto.randomUUID(), crew_name: '', worker_count: '', work_hours: '', note: '', representative: '', contact: '' }; }
function emptyMaterial(): MaterialDraft { return { material_name: '', quantity: '', unit: '', note: '' }; }
function emptyBlocker(content = ''): BlockerDraft { const row = emptyBlockerCapture(content); return { ...row, id: row.row_id }; }
function hasCrewValue(item: CrewDraft) { return Boolean(item.crew_name.trim() || item.worker_count || item.work_hours || item.note.trim() || item.representative?.trim() || item.contact?.trim()); }
function hasMaterialValue(item: MaterialDraft) { return Boolean(item.material_name.trim() || item.quantity || item.unit.trim() || item.note.trim()); }
function todayText() { return new Date().toISOString().slice(0, 10); }
function workItemRank(item: WorkItem) { return ['ready', 'in_progress', 'needs_changes', 'blocked'].includes(item.status) ? 0 : ['waiting_review', 'waiting_approval'].includes(item.status) ? 1 : 2; }
function inputClass(error: boolean) { return clsx('mt-1 min-w-0 max-w-full w-full rounded-md border bg-white px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-100', error ? 'border-rose-300 bg-rose-50' : 'border-line'); }
function tableInputClass(error: boolean) { return clsx('min-w-0 max-w-full w-full rounded-md border bg-white px-2.5 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-100', error ? 'border-rose-300 bg-rose-50' : 'border-line'); }
