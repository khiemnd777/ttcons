import { useEffect, useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { AlertTriangle, CheckCircle2, Download, ExternalLink, FileText, Loader2, PenLine, Plus, Trash2 } from 'lucide-react';
import clsx from 'clsx';
import { Dialog } from '../../components/Dialog';
import { EmptyState } from '../../components/EmptyState';
import { LoadingSpinner } from '../../components/LoadingSpinner';
import { StatusBadge } from '../../components/StatusBadge';
import { useToast } from '../../components/ToastProvider';
import { downloadBlob, get, post, put } from '../../lib/api/client';
import { useAuth } from '../../lib/auth/AuthContext';
import type { AnyRecord, EvidenceRecord, WorkflowFormDetail, WorkflowFormField, WorkflowFormPDFExport, WorkflowFormSignature } from '../../lib/types/domain';
import { dateText, percent } from '../../lib/utils/format';
import { enumLabel } from '../../lib/utils/status';
import { SignatureCaptureDialog, type SignatureStroke } from './SignatureCaptureDialog';
import { compactDateTime, isGenericWorkflowFormTemplate, workflowFormTypeLabel } from './workItemPresentation';

type FormErrors = Record<string, string>;

export function WorkflowFormPanel({
  workItemId,
  projectId,
  evidence,
  onChanged,
}: {
  workItemId: string;
  projectId?: string;
  evidence: EvidenceRecord[];
  onChanged?: () => void;
}) {
  const queryClient = useQueryClient();
  const toast = useToast();
  const { user } = useAuth();
  const [values, setValues] = useState<AnyRecord>({});
  const [errors, setErrors] = useState<FormErrors>({});
  const [signatureDialogOpen, setSignatureDialogOpen] = useState(false);
  const [signatureDetail, setSignatureDetail] = useState<WorkflowFormSignature | null>(null);
  const form = useQuery({
    queryKey: ['work-item-form', workItemId],
    queryFn: () => get<WorkflowFormDetail>(`/work-items/${workItemId}/form`),
    enabled: Boolean(workItemId),
    retry: false,
  });

  const fields = useMemo(() => form.data?.fields ?? [], [form.data?.fields]);
  const instance = form.data?.instance;
  const submitted = instance?.status === 'submitted';
  const businessFormTitle = workflowFormPanelTitle(
    instance?.title,
    form.data?.template?.code,
    form.data?.template?.name,
    instance?.form_type,
  );
  const signatures = form.data?.signatures ?? [];
  const pdfExports = form.data?.pdf_exports ?? [];
  const signedPurposesByCurrentUser = useMemo(
    () => (user?.id ? signatures.filter((signature) => signature.signer_user_id === user.id).map((signature) => signature.purpose) : []),
    [signatures, user?.id],
  );
  const signedByCurrentUser = signedPurposesByCurrentUser.includes('internal_ack');
  const validationIssues = useMemo(() => fields.map((field) => validateFieldValue(field, values[field.field_key])).filter(Boolean), [fields, values]);
  const checklistField = fields.find((field) => field.field_type === 'structured_list' && field.field_key === 'checklist_items');
  const checklistStats = checklistField ? checklistStatsFromRows(structuredRows(values[checklistField.field_key])) : null;
  const isPlanningDailyReport = instance?.form_type === 'daily_report' && form.data?.template?.code !== 'builtin_construction_daily_report';
  const isMeetingMinutes = instance?.form_type === 'meeting_minutes';
  const isPlanningChecklist = instance?.form_type === 'checklist' && form.data?.template?.code === 'builtin_checklist';
  const activeEvidence = evidence.filter((record) => record.status !== 'rejected');
  const validationIssueLabels = useMemo(
    () => fields.filter((field) => validateFieldValue(field, values[field.field_key])).map((field) => field.label),
    [fields, values],
  );

  useEffect(() => {
    setValues(form.data?.answers ?? {});
    setErrors({});
  }, [form.data?.answers, instance?.id]);

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ['work-item-form', workItemId] });
    queryClient.invalidateQueries({ queryKey: ['work-item', workItemId] });
    queryClient.invalidateQueries({ queryKey: ['my-work-items'] });
    if (projectId) queryClient.invalidateQueries({ queryKey: ['project-operations', projectId] });
    onChanged?.();
  };

  const saveDraft = useMutation({
    mutationFn: () => put<WorkflowFormDetail>(`/workflow-form-instances/${instance?.id}/draft`, { answers: values }),
    onSuccess: () => {
      invalidate();
      toast.success('Đã lưu nháp hồ sơ nghiệp vụ');
    },
    onError: (error) => toast.error('Không lưu được hồ sơ nghiệp vụ', error),
  });

  const submitForm = useMutation({
    mutationFn: () => post<WorkflowFormDetail>(`/workflow-form-instances/${instance?.id}/submit`, { answers: values }),
    onSuccess: () => {
      invalidate();
      toast.success('Đã gửi hồ sơ nghiệp vụ');
    },
    onError: (error) => toast.error('Không gửi được hồ sơ nghiệp vụ', error),
  });

  const signForm = useMutation({
    mutationFn: (payload: { purpose: string; comment: string; signature_image_data_url: string; signature_strokes: SignatureStroke[] }) =>
      post<WorkflowFormDetail>(`/workflow-form-instances/${instance?.id}/sign`, payload),
    onSuccess: () => {
      setSignatureDialogOpen(false);
      invalidate();
      toast.success('Đã ký và lưu dấu ký trực quan');
    },
    onError: (error) => toast.error('Không ký được hồ sơ nghiệp vụ', error),
  });

  const generatePDF = useMutation({
    mutationFn: () => post<WorkflowFormDetail>(`/workflow-form-instances/${instance?.id}/pdf-exports`, {}),
    onSuccess: () => {
      invalidate();
      toast.success('Đã tạo PDF hồ sơ nghiệp vụ');
    },
    onError: (error) => toast.error('Không tạo được PDF', error),
  });

  async function openProtectedBlob(path: string, fallbackTitle: string) {
    const viewer = window.open('', '_blank', 'noopener,noreferrer');
    try {
      const blob = await downloadBlob(path);
      const objectUrl = URL.createObjectURL(blob);
      if (viewer) viewer.location.href = objectUrl;
      else window.open(objectUrl, '_blank', 'noopener,noreferrer');
      window.setTimeout(() => URL.revokeObjectURL(objectUrl), 60_000);
    } catch (error) {
      viewer?.close();
      toast.error(fallbackTitle, error);
    }
  }

  function setFieldValue(field: WorkflowFormField, value: unknown) {
    setValues((current) => ({ ...current, [field.field_key]: value }));
    setErrors((current) => {
      if (!current[field.field_key]) return current;
      const { [field.field_key]: _removed, ...rest } = current;
      return rest;
    });
  }

  function validateBeforeSubmit() {
    const nextErrors: FormErrors = {};
    fields.forEach((field) => {
      const value = values[field.field_key];
      const error = validateFieldValue(field, value);
      if (error) nextErrors[field.field_key] = error;
    });
    setErrors(nextErrors);
    const labels = fields.filter((field) => nextErrors[field.field_key]).map((field) => field.label);
    if (labels.length) {
      toast.warn({ title: 'Thiếu thông tin nghiệp vụ', description: labels.join(', ') });
      return false;
    }
    return true;
  }

  if (form.isLoading) {
    return (
      <section>
        <h3 className="mb-2 text-sm font-semibold text-ink">Hồ sơ nghiệp vụ</h3>
        <div className="rounded-md border border-line bg-white p-4">
          <LoadingSpinner />
        </div>
      </section>
    );
  }

  if (form.isError) {
    return (
      <section>
        <h3 className="mb-2 text-sm font-semibold text-ink">Hồ sơ nghiệp vụ</h3>
        <div className="rounded-md border border-rose-100 bg-rose-50 p-4 text-sm text-rose-700">
          <p className="font-semibold">Không tải được hồ sơ nghiệp vụ</p>
          <p className="mt-1 text-xs">{form.error instanceof Error ? form.error.message : 'Vui lòng thử lại.'}</p>
          <button
            type="button"
            className="mt-3 rounded-md border border-rose-200 bg-white px-3 py-2 text-xs font-semibold text-rose-700 hover:bg-rose-100"
            onClick={() => form.refetch()}
          >
            Thử lại
          </button>
        </div>
      </section>
    );
  }

  if (!form.data?.available) {
    return (
      <section>
        <h3 className="mb-2 text-sm font-semibold text-ink">Hồ sơ nghiệp vụ</h3>
        <EmptyState title="Bước này chưa có hồ sơ nghiệp vụ." />
      </section>
    );
  }

  const formActionPending = saveDraft.isPending || submitForm.isPending;

  return (
    <section className="space-y-3">
      <WorkflowFormActionCenter
        title={businessFormTitle}
        versionNo={Number(form.data.version?.version_no ?? 1)}
        status={instance?.status ?? 'draft'}
        submitted={submitted}
        validationIssueLabels={validationIssueLabels}
        updatedAt={instance?.updated_at}
        submittedAt={instance?.submitted_at}
        saving={saveDraft.isPending}
        submitting={submitForm.isPending}
        onSave={() => saveDraft.mutate()}
        onSubmit={() => {
          if (validateBeforeSubmit()) submitForm.mutate();
        }}
      />

      <div className="space-y-3">
        <WorkflowFormOperationsSummary
          submitted={submitted}
          validationIssueCount={validationIssues.length}
          evidenceCount={activeEvidence.length}
          checklistStats={checklistStats}
          signatures={signatures}
          pdfExports={pdfExports}
        />
        {isPlanningDailyReport ? <PlanningDailyReportSummary values={values} submitted={submitted} /> : null}
        {isMeetingMinutes ? <MeetingMinutesSummary values={values} submitted={submitted} /> : null}
        {isPlanningChecklist ? <PlanningChecklistSummary values={values} submitted={submitted} /> : null}

        <section className="rounded-md border border-line bg-white">
          <div className="flex flex-col gap-2 border-b border-line bg-slate-50 px-3 py-3 sm:flex-row sm:items-center sm:justify-between">
            <div>
              <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">Nội dung cần xử lý</p>
              <h4 className="mt-0.5 text-sm font-semibold text-ink">Thông tin nghiệp vụ</h4>
            </div>
            <span className="rounded-md bg-white px-2 py-1 text-xs font-semibold text-slate-600 ring-1 ring-inset ring-line">{fields.length} thông tin</span>
          </div>
          <div className="grid gap-3 p-3">
            {fields.map((field) => (
              <WorkflowFieldControl
                key={field.field_key}
                field={field}
                value={values[field.field_key]}
                evidence={evidence}
                error={errors[field.field_key]}
                disabled={submitted || formActionPending}
                onChange={(value) => setFieldValue(field, value)}
              />
            ))}
          </div>
          <div className="border-t border-line bg-slate-50 px-3 py-2 text-xs text-slate-500">
            {instance?.submitted_at ? `Đã gửi ${compactDateTime(instance.submitted_at)}` : `Cập nhật ${compactDateTime(instance?.updated_at)}`}
          </div>
        </section>

        <section className="rounded-md border border-line bg-white">
          <div className="border-b border-line bg-slate-50 px-3 py-3">
            <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">Tài liệu nghiệp vụ</p>
            <h4 className="mt-0.5 text-sm font-semibold text-ink">Chữ ký và PDF</h4>
          </div>
          <div className="space-y-3 p-3">
            <WorkflowSignaturePanel
              signatures={signatures}
              submitted={submitted}
              signedByCurrentUser={signedByCurrentUser}
              signing={signForm.isPending}
              onOpenSign={() => setSignatureDialogOpen(true)}
              onViewDetail={(signature) => setSignatureDetail(signature)}
              onOpenVisual={(signature) => void openProtectedBlob(`/workflow-form-signatures/${signature.id}/visual`, 'Không mở được chữ ký')}
            />
            <WorkflowPDFExportPanel
              exports={pdfExports}
              submitted={submitted}
              generating={generatePDF.isPending}
              onGenerate={() => generatePDF.mutate()}
              onOpen={(record) => void openProtectedBlob(`/workflow-form-pdf-exports/${record.id}/file`, 'Không mở được PDF hồ sơ nghiệp vụ')}
            />
          </div>
        </section>

        {form.data.events?.length ? <WorkflowFormEventsPanel events={form.data.events} /> : null}
      </div>
      <SignatureCaptureDialog
        open={signatureDialogOpen}
        signerName={user?.full_name}
        submitting={signForm.isPending}
        signedPurposes={signedPurposesByCurrentUser}
        formRevision={instance?.current_revision}
        onClose={() => setSignatureDialogOpen(false)}
        onSubmit={(payload) => signForm.mutate(payload)}
      />
      <SignatureDetailDialog
        signature={signatureDetail}
        onClose={() => setSignatureDetail(null)}
        onOpenVisual={(signature) => void openProtectedBlob(`/workflow-form-signatures/${signature.id}/visual`, 'Không mở được chữ ký')}
      />
    </section>
  );
}

type ChecklistStats = {
  total: number;
  complete: number;
  needsAction: number;
  withEvidence: number;
};

const planningScopeLabels: Record<string, string> = {
  survey: 'Khảo sát',
  design: 'Thiết kế',
  boq: 'BOQ / Dự toán',
  budget: 'Ngân sách',
  procurement: 'Mua sắm',
  site_preparation: 'Chuẩn bị thi công',
  other: 'Khác',
};

const dailyResultLabels: Record<string, string> = {
  done: 'Hoàn tất',
  partial: 'Một phần',
  blocked: 'Bị chặn',
};

const dailyPriorityLabels: Record<string, string> = {
  low: 'Thấp',
  normal: 'Bình thường',
  high: 'Cao',
  urgent: 'Khẩn cấp',
};

const dailySeverityLabels: Record<string, string> = {
  low: 'Thấp',
  medium: 'Trung bình',
  high: 'Cao',
  critical: 'Nghiêm trọng',
};

const meetingAttendanceLabels: Record<string, string> = {
  present: 'Có mặt',
  remote: 'Từ xa',
  absent: 'Vắng',
};

const checklistItemStatusLabels: Record<string, string> = {
  done: 'Hoàn tất',
  not_applicable: 'Không áp dụng',
  needs_follow_up: 'Cần theo dõi',
  blocked: 'Bị chặn',
};

function workflowFormPanelTitle(instanceTitle?: string, templateCode?: string, templateName?: string, formType?: string) {
  const runtimeTitle = String(instanceTitle ?? '').trim();
  if (runtimeTitle && !isGenericWorkflowFormTemplate(undefined, runtimeTitle)) return runtimeTitle;
  const businessTemplateName = String(templateName ?? '').trim();
  if (businessTemplateName && !isGenericWorkflowFormTemplate(templateCode, businessTemplateName)) return businessTemplateName;
  return workflowFormTypeLabel(formType);
}

function WorkflowFormActionCenter({
  title,
  versionNo,
  status,
  submitted,
  validationIssueLabels,
  updatedAt,
  submittedAt,
  saving,
  submitting,
  onSave,
  onSubmit,
}: {
  title: string;
  versionNo: number;
  status: string;
  submitted: boolean;
  validationIssueLabels: string[];
  updatedAt?: string;
  submittedAt?: string;
  saving: boolean;
  submitting: boolean;
  onSave: () => void;
  onSubmit: () => void;
}) {
  const blocked = !submitted && validationIssueLabels.length > 0;
  const issuePreview = validationIssueLabels.slice(0, 4).join(', ');
  const extraIssues = validationIssueLabels.length > 4 ? ` +${validationIssueLabels.length - 4}` : '';
  return (
    <section className={clsx('rounded-md border p-3', blocked ? 'border-amber-200 bg-amber-50' : 'border-line bg-white')}>
      <div className="flex flex-col gap-3 xl:flex-row xl:items-start xl:justify-between">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <h3 className="text-sm font-semibold text-ink">{title}</h3>
            <StatusBadge status={status} />
          </div>
          <p className="mt-1 text-xs text-slate-500">
            Hồ sơ nghiệp vụ v{versionNo} · {submittedAt ? `Đã gửi ${compactDateTime(submittedAt)}` : `Cập nhật ${compactDateTime(updatedAt)}`}
          </p>
          <div className="mt-3 flex items-start gap-2">
            <span className={clsx('mt-0.5 rounded-md p-1.5', blocked ? 'bg-white text-amber-700' : 'bg-blue-50 text-blue-700')}>
              {blocked ? <AlertTriangle className="h-4 w-4" /> : <CheckCircle2 className="h-4 w-4" />}
            </span>
            <div className="min-w-0">
              <p className="text-sm font-semibold text-ink">
                {blocked ? 'Cần hoàn tất thông tin trước khi gửi' : submitted ? 'Hồ sơ đã gửi, chuyển sang tài liệu' : 'Sẵn sàng lưu nháp hoặc gửi'}
              </p>
              <p className="mt-1 text-xs leading-5 text-slate-600">
                {blocked ? `Thiếu: ${issuePreview}${extraIssues}` : submitted ? 'Nội dung hồ sơ đã khóa; chữ ký và PDF dùng phiên bản đã gửi.' : 'Không còn lỗi bắt buộc ở trạng thái hiện tại.'}
              </p>
            </div>
          </div>
        </div>
        <div className="flex shrink-0 flex-wrap gap-2 xl:justify-end">
          <button
            type="button"
            className="inline-flex items-center gap-2 rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100 disabled:cursor-not-allowed disabled:opacity-60"
            disabled={submitted || saving || submitting}
            onClick={onSave}
          >
            {saving ? <Loader2 className="h-4 w-4 animate-spin" /> : null}
            Lưu nháp
          </button>
          <button
            type="button"
            className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60"
            disabled={submitted || saving || submitting}
            onClick={onSubmit}
          >
            {submitting ? <Loader2 className="h-4 w-4 animate-spin" /> : null}
            Gửi hồ sơ
          </button>
        </div>
      </div>
    </section>
  );
}

function WorkflowFormOperationsSummary({
  submitted,
  validationIssueCount,
  evidenceCount,
  checklistStats,
  signatures,
  pdfExports,
}: {
  submitted: boolean;
  validationIssueCount: number;
  evidenceCount: number;
  checklistStats: ChecklistStats | null;
  signatures: WorkflowFormSignature[];
  pdfExports: WorkflowFormPDFExport[];
}) {
  const nextAction = workflowNextAction(submitted, validationIssueCount, signatures.length, pdfExports.length);
  return (
    <section className="rounded-md border border-line bg-white p-3">
      <div className="flex flex-col gap-3 xl:flex-row xl:items-center xl:justify-between">
        <div className="min-w-0">
          <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">Tóm tắt xử lý</p>
          <h4 className="mt-0.5 text-sm font-semibold text-ink">Việc tiếp theo: {nextAction}</h4>
        </div>
        <div className="grid gap-2 text-xs font-semibold text-slate-600 sm:grid-cols-4 xl:w-[36rem]">
          <WorkflowStepPill label="Checklist" value={checklistStats ? `${checklistStats.complete}/${checklistStats.total} hoàn tất` : 'Không áp dụng'} active={!submitted && Boolean(checklistStats?.needsAction)} />
          <WorkflowStepPill label="Minh chứng" value={`${evidenceCount} đang hiệu lực`} active={!submitted && evidenceCount === 0} />
          <WorkflowStepPill label="Chữ ký" value={`${signatures.length} đã ký`} active={submitted && signatures.length === 0} />
          <WorkflowStepPill label="PDF" value={`${pdfExports.length} tệp`} active={submitted && signatures.length > 0 && pdfExports.length === 0} />
        </div>
      </div>
    </section>
  );
}

function WorkflowFormEventsPanel({ events }: { events: AnyRecord[] }) {
  return (
    <section className="rounded-md border border-line bg-white">
      <div className="border-b border-line bg-slate-50 px-3 py-3">
        <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">Lịch sử hồ sơ</p>
        <h4 className="mt-0.5 text-sm font-semibold text-ink">Sự kiện gần nhất</h4>
      </div>
      <div className="divide-y divide-line">
        {events.slice(0, 5).map((event) => (
          <div key={String(event.id)} className="px-3 py-2">
            <p className="text-xs font-semibold text-ink">{enumLabel('event_type', String(event.event_type))}</p>
            <p className="mt-1 text-xs text-slate-500">{event.actor_name ?? 'Hệ thống'} · {compactDateTime(event.created_at)}</p>
          </div>
        ))}
      </div>
    </section>
  );
}

function PlanningDailyReportSummary({ values, submitted }: { values: AnyRecord; submitted: boolean }) {
  const completedItems = structuredRows(values.completed_items);
  const nextPlanItems = structuredRows(values.next_plan_items);
  const blockers = structuredRows(values.blockers);
  return (
    <section className="mb-4 rounded-md border border-emerald-100 bg-emerald-50 p-3">
      <div className="flex flex-col gap-3 xl:flex-row xl:items-start xl:justify-between">
        <div className="min-w-0">
          <p className="text-sm font-semibold text-ink">Nhật ký kế hoạch</p>
          <p className="mt-1 text-xs leading-5 text-slate-600">
            {submitted ? 'Nhật ký đã gửi; các hạng mục sẽ làm đã được dùng để sinh đầu việc kế hoạch tiếp theo.' : 'Hoàn tất phần Đã làm và Sẽ làm trước khi gửi nhật ký.'}
          </p>
        </div>
        <div className="grid gap-2 text-xs font-semibold text-slate-600 sm:grid-cols-4 xl:w-[40rem]">
          <WorkflowStepPill label="Ngày" value={dateText(values.report_date)} active={!values.report_date} />
          <WorkflowStepPill label="Người báo cáo" value={textOrDash(values.reported_by)} active={!values.reported_by} />
          <WorkflowStepPill label="Phạm vi" value={planningScopeLabel(values.planning_scope)} active={!values.planning_scope} />
          <WorkflowStepPill label="Tiến độ" value={values.progress_percent === undefined ? '-' : percent(values.progress_percent)} active={false} />
        </div>
      </div>
      <div className="mt-3 grid gap-3 xl:grid-cols-3">
        <DailyReportList title="Đã làm" items={completedItems} empty="Chưa có hạng mục đã làm." meta={completedItemMeta} />
        <DailyReportList title="Sẽ làm" items={nextPlanItems} empty="Chưa có việc sẽ làm." meta={nextPlanItemMeta} />
        <DailyReportList title="Điểm chặn / Rủi ro" items={blockers} empty="Chưa có điểm chặn." meta={blockerItemMeta} />
      </div>
    </section>
  );
}

function MeetingMinutesSummary({ values, submitted }: { values: AnyRecord; submitted: boolean }) {
  const attendees = structuredRows(values.attendees);
  const decisions = structuredRows(values.decisions);
  const actionItems = structuredRows(values.action_items);
  const attendeesText = typeof values.attendees === 'string' ? values.attendees.trim() : '';
  const decisionsText = typeof values.decisions === 'string' ? values.decisions.trim() : '';
  const actionItemsText = typeof values.action_items === 'string' ? values.action_items.trim() : '';
  const attendeeCount = attendees.length || (attendeesText ? 1 : 0);
  const presentCount = attendees.filter((item) => String(item.attendance ?? '') === 'present').length;
  const remoteCount = attendees.filter((item) => String(item.attendance ?? '') === 'remote').length;
  const actionCount = actionItems.length || (actionItemsText ? 1 : 0);

  return (
    <section className="mb-4 rounded-md border border-indigo-100 bg-indigo-50 p-3">
      <div className="flex flex-col gap-3 xl:flex-row xl:items-start xl:justify-between">
        <div className="min-w-0">
          <p className="text-sm font-semibold text-ink">{textOrDash(values.meeting_title) === '-' ? 'Biên bản họp' : textOrDash(values.meeting_title)}</p>
          <p className="mt-1 text-xs leading-5 text-slate-600">
            {submitted ? 'Biên bản đã gửi; đầu việc được dùng để sinh đầu việc theo hồ sơ nghiệp vụ.' : 'Hoàn tất người tham dự, quyết định và đầu việc trước khi gửi biên bản.'}
          </p>
        </div>
        <div className="grid gap-2 text-xs font-semibold text-slate-600 sm:grid-cols-4 xl:w-[40rem]">
          <WorkflowStepPill label="Ngày họp" value={dateText(values.meeting_date)} active={!values.meeting_date} />
          <WorkflowStepPill label="Chủ trì" value={textOrDash(values.chair)} active={!values.chair} />
          <WorkflowStepPill label="Tham dự" value={`${attendeeCount} người`} active={!attendeeCount} />
          <WorkflowStepPill label="Đầu việc" value={`${actionCount} mục`} active={!submitted && actionCount > 0} />
        </div>
      </div>
      <div className="mt-3 grid gap-3 xl:grid-cols-3">
        <MeetingSummaryList
          title="Thành phần tham dự"
          items={attendees}
          fallbackText={attendeesText}
          empty="Chưa có người tham dự."
          meta={meetingAttendeeMeta}
          countLabel={attendees.length ? `${presentCount} có mặt · ${remoteCount} từ xa` : undefined}
        />
        <MeetingSummaryList
          title="Quyết định"
          items={decisions}
          fallbackText={decisionsText}
          empty="Chưa có quyết định."
          meta={meetingDecisionMeta}
        />
        <MeetingSummaryList
          title="Đầu việc"
          items={actionItems}
          fallbackText={actionItemsText}
          empty="Chưa có đầu việc."
          meta={meetingActionItemMeta}
        />
      </div>
      <div className="mt-3 rounded-md border border-indigo-100 bg-white/80 px-3 py-2">
        <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">Nội dung họp</p>
        <p className="mt-1 line-clamp-3 whitespace-pre-wrap break-words text-xs leading-5 text-slate-600">{textOrDash(values.agenda)}</p>
      </div>
    </section>
  );
}

function PlanningChecklistSummary({ values, submitted }: { values: AnyRecord; submitted: boolean }) {
  const items = structuredRows(values.checklist_items);
  const itemsDoneText = typeof values.items_done === 'string' ? values.items_done.trim() : '';
  const exceptionsText = typeof values.exceptions === 'string' ? values.exceptions.trim() : '';
  const legacyCount = items.length ? 0 : (itemsDoneText ? 1 : 0);
  const doneCount = items.length
    ? items.filter((item) => ['done', 'not_applicable'].includes(String(item.status ?? ''))).length
    : (itemsDoneText && !exceptionsText ? 1 : 0);
  const needsActionCount = items.length
    ? items.filter((item) => !['done', 'not_applicable'].includes(String(item.status ?? ''))).length
    : (exceptionsText ? 1 : 0);
  const evidenceLinked = items.filter((item) => Boolean(item.evidence_id)).length;
  const evidenceMissing = items.length ? items.length - evidenceLinked : legacyCount;
  const itemCount = items.length || legacyCount;

  return (
    <section className="mb-4 rounded-md border border-cyan-100 bg-cyan-50 p-3">
      <div className="flex flex-col gap-3 xl:flex-row xl:items-start xl:justify-between">
        <div className="min-w-0">
          <p className="text-sm font-semibold text-ink">Checklist kế hoạch</p>
          <p className="mt-1 text-xs leading-5 text-slate-600">
            {submitted ? 'Checklist đã gửi; dữ liệu hoàn tất được lưu lại để kiểm tra bước.' : 'Chỉ được gửi khi mọi dòng ở trạng thái Hoàn tất hoặc Không áp dụng.'}
          </p>
        </div>
        <div className="grid gap-2 text-xs font-semibold text-slate-600 sm:grid-cols-4 xl:w-[40rem]">
          <WorkflowStepPill label="Ngày" value={dateText(values.check_date)} active={!values.check_date} />
          <WorkflowStepPill label="Phạm vi" value={planningScopeLabel(values.planning_scope)} active={!values.planning_scope} />
          <WorkflowStepPill label="Người kiểm tra" value={textOrDash(values.checked_by)} active={!values.checked_by} />
          <WorkflowStepPill label="Minh chứng" value={`${evidenceLinked}/${itemCount} đã liên kết`} active={!submitted && evidenceMissing > 0} />
        </div>
      </div>
      <div className="mt-3 grid gap-3 xl:grid-cols-3">
        <PlanningChecklistList title="Hoàn tất / Không áp dụng" items={items.filter((item) => ['done', 'not_applicable'].includes(String(item.status ?? '')))} fallbackText={items.length ? '' : itemsDoneText} empty="Chưa có dòng hoàn tất/không áp dụng." />
        <PlanningChecklistList title="Cần xử lý" items={items.filter((item) => !['done', 'not_applicable'].includes(String(item.status ?? '')))} fallbackText={items.length ? '' : exceptionsText} empty="Không có dòng cần xử lý." />
        <div className="min-w-0 rounded-md bg-white/80 p-3 ring-1 ring-inset ring-white">
          <div className="mb-2 flex items-center justify-between gap-2">
            <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">Tổng quan</p>
            <span className="rounded-md bg-slate-100 px-2 py-0.5 text-xs font-semibold text-slate-600">{itemCount}</span>
          </div>
          <div className="grid gap-2 text-xs font-semibold text-slate-600">
            <WorkflowStepPill label="Hoàn tất" value={`${doneCount}/${itemCount} hoàn tất`} active={false} />
            <WorkflowStepPill label="Cần xử lý" value={`${needsActionCount} dòng`} active={!submitted && needsActionCount > 0} />
            <WorkflowStepPill label="Thiếu minh chứng" value={`${evidenceMissing} dòng`} active={!submitted && evidenceMissing > 0} />
          </div>
        </div>
      </div>
      {values.overall_comment ? (
        <div className="mt-3 rounded-md border border-cyan-100 bg-white/80 px-3 py-2">
          <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">Ghi chú tổng quan</p>
          <p className="mt-1 line-clamp-3 whitespace-pre-wrap break-words text-xs leading-5 text-slate-600">{textOrDash(values.overall_comment)}</p>
        </div>
      ) : null}
    </section>
  );
}

function DailyReportList({
  title,
  items,
  empty,
  meta,
}: {
  title: string;
  items: AnyRecord[];
  empty: string;
  meta: (item: AnyRecord) => string;
}) {
  return (
    <div className="min-w-0 rounded-md bg-white/80 p-3 ring-1 ring-inset ring-white">
      <div className="mb-2 flex items-center justify-between gap-2">
        <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">{title}</p>
        <span className="rounded-md bg-slate-100 px-2 py-0.5 text-xs font-semibold text-slate-600">{items.length}</span>
      </div>
      {items.length ? (
        <div className="space-y-2">
          {items.slice(0, 3).map((item, index) => (
            <article key={`${title}-${index}`} className="rounded-md border border-line bg-white px-2.5 py-2">
              <p className="break-words text-xs font-semibold text-ink">{dailyItemTitle(item, title)}</p>
              <p className="mt-1 break-words text-xs text-slate-500">{meta(item)}</p>
            </article>
          ))}
          {items.length > 3 ? <p className="text-xs font-medium text-slate-500">+{items.length - 3} dòng khác trong hồ sơ</p> : null}
        </div>
      ) : (
        <p className="rounded-md border border-dashed border-line bg-slate-50 px-3 py-4 text-center text-xs text-slate-500">{empty}</p>
      )}
    </div>
  );
}

function PlanningChecklistList({
  title,
  items,
  fallbackText,
  empty,
}: {
  title: string;
  items: AnyRecord[];
  fallbackText?: string;
  empty: string;
}) {
  const count = items.length || (fallbackText ? 1 : 0);
  return (
    <div className="min-w-0 rounded-md bg-white/80 p-3 ring-1 ring-inset ring-white">
      <div className="mb-2 flex items-center justify-between gap-2">
        <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">{title}</p>
        <span className="rounded-md bg-slate-100 px-2 py-0.5 text-xs font-semibold text-slate-600">{count}</span>
      </div>
      {items.length ? (
        <div className="space-y-2">
          {items.slice(0, 3).map((item, index) => (
            <article key={`${title}-${index}`} className="rounded-md border border-line bg-white px-2.5 py-2">
              <div className="flex flex-wrap items-start justify-between gap-2">
                <p className="break-words text-xs font-semibold text-ink">{planningChecklistItemTitle(item)}</p>
                <StatusBadge status={String(item.status || 'draft')} />
              </div>
              <p className="mt-1 break-words text-xs text-slate-500">{planningChecklistItemMeta(item)}</p>
            </article>
          ))}
          {items.length > 3 ? <p className="text-xs font-medium text-slate-500">+{items.length - 3} dòng khác trong hồ sơ</p> : null}
        </div>
      ) : fallbackText ? (
        <p className="line-clamp-5 whitespace-pre-wrap break-words rounded-md border border-line bg-white px-3 py-2 text-xs leading-5 text-slate-600">{fallbackText}</p>
      ) : (
        <p className="rounded-md border border-dashed border-line bg-slate-50 px-3 py-4 text-center text-xs text-slate-500">{empty}</p>
      )}
    </div>
  );
}

function MeetingSummaryList({
  title,
  items,
  fallbackText,
  empty,
  meta,
  countLabel,
}: {
  title: string;
  items: AnyRecord[];
  fallbackText?: string;
  empty: string;
  meta: (item: AnyRecord) => string;
  countLabel?: string;
}) {
  const count = items.length || (fallbackText ? 1 : 0);
  return (
    <div className="min-w-0 rounded-md bg-white/80 p-3 ring-1 ring-inset ring-white">
      <div className="mb-2 flex items-center justify-between gap-2">
        <div>
          <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">{title}</p>
          {countLabel ? <p className="mt-1 text-[11px] font-medium text-slate-500">{countLabel}</p> : null}
        </div>
        <span className="rounded-md bg-slate-100 px-2 py-0.5 text-xs font-semibold text-slate-600">{count}</span>
      </div>
      {items.length ? (
        <div className="space-y-2">
          {items.slice(0, 3).map((item, index) => (
            <article key={`${title}-${index}`} className="rounded-md border border-line bg-white px-2.5 py-2">
              <p className="break-words text-xs font-semibold text-ink">{meetingItemTitle(item, title)}</p>
              <p className="mt-1 break-words text-xs text-slate-500">{meta(item)}</p>
            </article>
          ))}
          {items.length > 3 ? <p className="text-xs font-medium text-slate-500">+{items.length - 3} dòng khác trong hồ sơ</p> : null}
        </div>
      ) : fallbackText ? (
        <p className="line-clamp-5 whitespace-pre-wrap break-words rounded-md border border-line bg-white px-3 py-2 text-xs leading-5 text-slate-600">{fallbackText}</p>
      ) : (
        <p className="rounded-md border border-dashed border-line bg-slate-50 px-3 py-4 text-center text-xs text-slate-500">{empty}</p>
      )}
    </div>
  );
}

function WorkflowStepPill({ label, value, active }: { label: string; value: string; active: boolean }) {
  return (
    <div className={clsx('min-w-0 rounded-md px-2.5 py-2 ring-1 ring-inset', active ? 'bg-white text-blue-700 ring-blue-100' : 'bg-white/75 text-slate-600 ring-white')}>
      <span className="block text-[11px] uppercase tracking-wide text-slate-500">{label}</span>
      <span className="block truncate">{value}</span>
    </div>
  );
}

function workflowNextAction(submitted: boolean, validationIssueCount: number, signatureCount: number, pdfCount: number) {
  if (!submitted && validationIssueCount > 0) return 'Hoàn tất checklist/hồ sơ';
  if (!submitted) return 'Gửi hồ sơ nghiệp vụ';
  if (signatureCount === 0) return 'Ký / đóng dấu PDF';
  if (pdfCount === 0) return 'Tạo PDF';
  return 'Mở tài liệu hoặc hoàn tất đầu việc';
}

function dailyItemTitle(item: AnyRecord, fallback: string) {
  const title = String(item.title ?? item.content ?? '').trim();
  return title || fallback;
}

function completedItemMeta(item: AnyRecord) {
  return [
    dailyResultLabels[String(item.result ?? '')] ?? item.result,
    item.owner_role ? `Phụ trách: ${item.owner_role}` : '',
  ].filter(Boolean).join(' · ') || '-';
}

function nextPlanItemMeta(item: AnyRecord) {
  return [
    dailyPriorityLabels[String(item.priority ?? '')] ?? item.priority,
    item.owner_role || item.owner_name ? `Phụ trách: ${[item.owner_role, item.owner_name].filter(Boolean).join(' / ')}` : '',
    item.due_date ? `Hạn: ${dateText(item.due_date)}` : '',
  ].filter(Boolean).join(' · ') || '-';
}

function blockerItemMeta(item: AnyRecord) {
  return [
    item.impact ? `Ảnh hưởng: ${item.impact}` : '',
    dailySeverityLabels[String(item.severity ?? '')] ?? item.severity,
    item.owner_role ? `Phụ trách: ${item.owner_role}` : '',
    item.target_date ? `Mốc: ${dateText(item.target_date)}` : '',
  ].filter(Boolean).join(' · ') || '-';
}

function meetingItemTitle(item: AnyRecord, fallback: string) {
  const title = String(item.title ?? item.content ?? item.name ?? '').trim();
  return title || fallback;
}

function meetingAttendeeMeta(item: AnyRecord) {
  return [
    item.role,
    meetingAttendanceLabel(item.attendance),
  ].filter(Boolean).join(' · ') || '-';
}

function meetingDecisionMeta(item: AnyRecord) {
  return [
    item.owner ? `Phụ trách: ${item.owner}` : '',
    item.target ? `Mốc: ${dateText(item.target)}` : '',
  ].filter(Boolean).join(' · ') || '-';
}

function meetingActionItemMeta(item: AnyRecord) {
  return [
    dailyPriorityLabels[String(item.priority ?? '')] ?? item.priority,
    item.owner_role || item.owner_name ? `Phụ trách: ${[item.owner_role, item.owner_name].filter(Boolean).join(' / ')}` : '',
    item.due_date ? `Hạn: ${dateText(item.due_date)}` : '',
  ].filter(Boolean).join(' · ') || '-';
}

function meetingAttendanceLabel(value: unknown) {
  const key = String(value ?? '').trim();
  return meetingAttendanceLabels[key] ?? key;
}

function planningChecklistItemTitle(item: AnyRecord) {
  const title = String(item.title ?? item.content ?? '').trim();
  return title || 'Dòng checklist';
}

function planningChecklistItemMeta(item: AnyRecord) {
  return [
    checklistItemStatusLabel(item.status),
    item.assignee_role || item.assignee_name ? `Người phụ trách: ${[item.assignee_role, item.assignee_name].filter(Boolean).join(' / ')}` : '',
    item.due_date ? `Hạn: ${dateText(item.due_date)}` : '',
    item.evidence_id ? 'Đã liên kết minh chứng' : 'Thiếu minh chứng',
  ].filter(Boolean).join(' · ') || '-';
}

function checklistItemStatusLabel(value: unknown) {
  const key = String(value ?? '').trim();
  return checklistItemStatusLabels[key] ?? key;
}

function planningScopeLabel(value: unknown) {
  const key = String(value ?? '').trim();
  return planningScopeLabels[key] ?? (key || '-');
}

function textOrDash(value: unknown) {
  const text = String(value ?? '').trim();
  return text || '-';
}

function WorkflowSignaturePanel({
  signatures,
  submitted,
  signedByCurrentUser,
  signing,
  onOpenSign,
  onViewDetail,
  onOpenVisual,
}: {
  signatures: WorkflowFormSignature[];
  submitted: boolean;
  signedByCurrentUser: boolean;
  signing: boolean;
  onOpenSign: () => void;
  onViewDetail: (signature: WorkflowFormSignature) => void;
  onOpenVisual: (signature: WorkflowFormSignature) => void;
}) {
  const disabled = !submitted || signing;
  return (
    <section className="rounded-md border border-line bg-slate-50 p-3">
      <div className="mb-2 flex flex-wrap items-center justify-between gap-2">
        <h4 className="text-sm font-semibold text-ink">Chữ ký</h4>
        {signatures.length ? <span className="text-xs font-semibold text-slate-500">{signatures.length} đã ký</span> : null}
      </div>
      {signatures.length ? (
        <div className="space-y-2">
          {signatures.map((signature) => (
            <div key={signature.id} className="rounded-md border border-line bg-slate-50 px-3 py-2">
              <div className="flex flex-wrap items-start justify-between gap-2">
                <div className="min-w-0">
                  <p className="truncate text-sm font-semibold text-ink">{signature.signer_name}</p>
                  <p className="mt-1 text-xs text-slate-500">
                    {signaturePurposeLabel(signature.purpose)} · Phiên bản {signature.form_revision_no} · {compactDateTime(signature.signed_at)}
                  </p>
                  <p className="mt-1 text-xs text-slate-500">
                    Vai trò: {signatureRolesText(signature)} · mã nội dung {shortHash(signature.payload_hash)}
                  </p>
                  {signature.visual_checksum_sha256 ? (
                    <p className="mt-1 text-xs text-slate-500">dấu ký sha256:{signature.visual_checksum_sha256.slice(0, 12)}</p>
                  ) : null}
                </div>
                <div className="flex flex-wrap items-center gap-2">
                  <button
                    type="button"
                    className="inline-flex items-center gap-1 rounded-md border border-line bg-white px-2 py-1 text-xs font-semibold text-slate-700 hover:bg-slate-100"
                    onClick={() => onViewDetail(signature)}
                  >
                    <FileText className="h-3.5 w-3.5" />
                    Chi tiết
                  </button>
                  {signature.visual_storage_key ? (
                    <button
                      type="button"
                      className="inline-flex items-center gap-1 rounded-md border border-line bg-white px-2 py-1 text-xs font-semibold text-blue-700 hover:border-blue-300 hover:bg-blue-50"
                      onClick={() => onOpenVisual(signature)}
                    >
                      <ExternalLink className="h-3.5 w-3.5" />
                      Dấu ký
                    </button>
                  ) : null}
                  <span className="inline-flex items-center gap-1 rounded-md bg-emerald-50 px-2 py-1 text-xs font-semibold text-emerald-700 ring-1 ring-inset ring-emerald-200">
                    <CheckCircle2 className="h-3.5 w-3.5" />
                    Không thể chỉnh sửa
                  </span>
                </div>
              </div>
              {signature.comment ? <p className="mt-2 text-sm text-slate-700">{signature.comment}</p> : null}
            </div>
          ))}
        </div>
      ) : (
        <div className="rounded-md border border-dashed border-line px-3 py-4 text-sm font-medium text-slate-500">
          {submitted ? 'Chưa có chữ ký nội bộ.' : 'Gửi hồ sơ trước khi ký nội bộ.'}
        </div>
      )}
      <div className="mt-3 flex flex-wrap items-center justify-between gap-2">
        <div className="text-xs text-slate-500">
          {!submitted ? 'Gửi hồ sơ trước khi ký.' : signedByCurrentUser ? 'Bạn đã ký xác nhận nội bộ; vẫn có thể ký mục đích khác nếu cần.' : 'Chữ ký canvas sẽ được lưu như tệp được bảo vệ.'}
        </div>
        <button
          type="button"
          className="inline-flex items-center justify-center gap-2 rounded-md bg-emerald-600 px-3 py-2 text-sm font-semibold text-white hover:bg-emerald-700 disabled:cursor-not-allowed disabled:opacity-60"
          disabled={disabled}
          onClick={onOpenSign}
        >
          {signing ? <Loader2 className="h-4 w-4 animate-spin" /> : <PenLine className="h-4 w-4" />}
          Ký / Đóng dấu PDF
        </button>
      </div>
    </section>
  );
}

function SignatureDetailDialog({
  signature,
  onClose,
  onOpenVisual,
}: {
  signature: WorkflowFormSignature | null;
  onClose: () => void;
  onOpenVisual: (signature: WorkflowFormSignature) => void;
}) {
  if (!signature) return null;
  return (
    <Dialog
      open={Boolean(signature)}
      title="Chi tiết chữ ký"
      description="Người thao tác, mục đích, phiên bản và mã kiểm tra được ghi lại cùng bản ghi chữ ký."
      size="lg"
      onClose={onClose}
      footer={
        <div className="flex flex-wrap items-center justify-between gap-2">
          <p className="text-xs text-slate-500">Mã nội dung dùng để đối chiếu nội dung hồ sơ tại thời điểm ký.</p>
          <div className="flex flex-wrap justify-end gap-2">
            {signature.visual_storage_key ? (
              <button
                type="button"
                className="inline-flex items-center gap-2 rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-blue-700 hover:border-blue-300 hover:bg-blue-50"
                onClick={() => onOpenVisual(signature)}
              >
                <ExternalLink className="h-4 w-4" />
                Mở dấu ký
              </button>
            ) : null}
            <button type="button" className="rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100" onClick={onClose}>
              Đóng
            </button>
          </div>
        </div>
      }
    >
      <div className="space-y-4">
        <div className="rounded-md border border-emerald-200 bg-emerald-50 p-3">
          <p className="text-sm font-semibold text-emerald-900">Bản ghi chữ ký không thể chỉnh sửa</p>
          <p className="mt-1 text-xs leading-5 text-emerald-800">
            Chữ ký này được lưu chỉ thêm mới theo hồ sơ, người ký, mục đích và phiên bản. Nếu cần thay đổi nội dung, cần tạo phiên bản/hồ sơ mới thay vì sửa bản ghi ký.
          </p>
        </div>
        <div className="grid gap-3 sm:grid-cols-2">
          <SignatureDetailField label="Người ký" value={signature.signer_name || '-'} />
          <SignatureDetailField label="ID người ký" value={signature.signer_user_id || '-'} mono />
          <SignatureDetailField label="Mục đích" value={signaturePurposeLabel(signature.purpose)} />
          <SignatureDetailField label="Thời điểm ký" value={compactDateTime(signature.signed_at)} />
          <SignatureDetailField label="Phiên bản hồ sơ" value={`Phiên bản ${signature.form_revision_no}`} />
          <SignatureDetailField label="Vai trò lúc ký" value={signatureRolesText(signature)} />
          <SignatureDetailField label="Tệp dấu ký" value={signatureVisualText(signature)} />
          <SignatureDetailField label="Nơi lưu dấu ký" value={signature.visual_storage_provider || '-'} />
        </div>
        <SignatureHashBlock label="Mã nội dung" value={signature.payload_hash} />
        <SignatureHashBlock label="Checksum dấu ký sha256" value={signature.visual_checksum_sha256} />
        {signature.comment ? (
          <div className="rounded-md border border-line bg-slate-50 p-3">
            <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">Ghi chú</p>
            <p className="mt-1 whitespace-pre-wrap break-words text-sm leading-6 text-slate-700">{signature.comment}</p>
          </div>
        ) : null}
      </div>
    </Dialog>
  );
}

function SignatureDetailField({ label, value, mono = false }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="min-w-0 rounded-md border border-line bg-white px-3 py-2">
      <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">{label}</p>
      <p className={clsx('mt-1 break-words text-sm text-ink', mono && 'font-mono text-xs')}>{value}</p>
    </div>
  );
}

function SignatureHashBlock({ label, value }: { label: string; value?: string }) {
  return (
    <div className="rounded-md border border-line bg-slate-50 p-3">
      <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">{label}</p>
      <p className="mt-1 break-all font-mono text-xs leading-5 text-slate-700">{value || '-'}</p>
    </div>
  );
}

function WorkflowPDFExportPanel({
  exports,
  submitted,
  generating,
  onGenerate,
  onOpen,
}: {
  exports: WorkflowFormPDFExport[];
  submitted: boolean;
  generating: boolean;
  onGenerate: () => void;
  onOpen: (record: WorkflowFormPDFExport) => void;
}) {
  return (
    <section className="rounded-md border border-line bg-slate-50 p-3">
      <div className="mb-2 flex flex-wrap items-center justify-between gap-2">
        <h4 className="text-sm font-semibold text-ink">Tệp PDF</h4>
        <button
          type="button"
          className="inline-flex items-center gap-2 rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100 disabled:cursor-not-allowed disabled:opacity-60"
          disabled={!submitted || generating}
          onClick={onGenerate}
        >
          {generating ? <Loader2 className="h-4 w-4 animate-spin" /> : <FileText className="h-4 w-4" />}
          Tạo PDF
        </button>
      </div>
      {exports.length ? (
        <div className="space-y-2">
          {exports.map((record) => (
            <div key={record.id} className="rounded-md border border-line bg-slate-50 px-3 py-2">
              <div className="flex flex-wrap items-start justify-between gap-2">
                <div className="min-w-0">
                  <p className="truncate text-sm font-semibold text-ink">{record.file_name}</p>
                  <p className="mt-1 text-xs text-slate-500">
                    Phiên bản {record.form_revision_no} · {compactDateTime(record.generated_at)} · sha256:{record.checksum_sha256.slice(0, 12)}
                  </p>
                </div>
                <button
                  type="button"
                  className="inline-flex items-center gap-1 rounded-md border border-line bg-white px-2 py-1 text-xs font-semibold text-blue-700 hover:border-blue-300 hover:bg-blue-50"
                  onClick={() => onOpen(record)}
                >
                  <Download className="h-3.5 w-3.5" />
                  Mở
                </button>
              </div>
            </div>
          ))}
        </div>
      ) : (
        <div className="rounded-md border border-dashed border-line px-3 py-4 text-sm font-medium text-slate-500">
          {submitted ? 'Chưa tạo PDF cho hồ sơ này.' : 'Gửi hồ sơ trước khi tạo PDF.'}
        </div>
      )}
    </section>
  );
}

function shortHash(hash?: string) {
  return hash ? hash.slice(0, 8) : '-';
}

function signatureRolesText(signature: WorkflowFormSignature) {
  const roles = signature.signer_roles_json;
  if (!Array.isArray(roles) || !roles.length) return '-';
  return roles
    .map((role) => {
      if (typeof role === 'string') return role;
      if (!role || typeof role !== 'object') return '';
      const label = role.display_name ?? role.role_name ?? role.name ?? role.code ?? role.id;
      return label ? String(label) : JSON.stringify(role);
    })
    .filter(Boolean)
    .join(', ') || '-';
}

function signatureVisualText(signature: WorkflowFormSignature) {
  return [
    signature.visual_mime_type,
    signature.visual_file_size ? fileSizeText(signature.visual_file_size) : undefined,
    signature.visual_storage_key ? 'tệp được bảo vệ' : undefined,
  ].filter(Boolean).join(' · ') || '-';
}

function fileSizeText(value: number) {
  if (value < 1024) return `${value} B`;
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB`;
  return `${(value / 1024 / 1024).toFixed(1)} MB`;
}

function signaturePurposeLabel(purpose?: string) {
  switch (purpose) {
    case 'prepared_by':
      return 'Người lập';
    case 'reviewed_by':
      return 'Người rà soát';
    case 'approved_by':
      return 'Người phê duyệt';
    case 'received_by':
      return 'Người nhận';
    case 'internal_ack':
    default:
      return 'Xác nhận nội bộ';
  }
}

function WorkflowFieldControl({
  field,
  value,
  evidence,
  error,
  disabled,
  onChange,
}: {
  field: WorkflowFormField;
  value: unknown;
  evidence: EvidenceRecord[];
  error?: string;
  disabled: boolean;
  onChange: (value: unknown) => void;
}) {
  const controlClassName = clsx(
    'mt-1 w-full rounded-md border px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-100 disabled:bg-slate-100',
    error ? 'border-rose-300 bg-rose-50' : 'border-line bg-white',
  );
  const fieldContent = renderFieldControl(field, value, evidence, disabled, Boolean(error), controlClassName, onChange);
  return (
    <div className={clsx('block', (field.field_type === 'textarea' || field.field_type === 'structured_list') && 'md:col-span-2')}>
      <span className="text-sm font-semibold text-slate-700">{field.label}</span>
      {field.required ? <span className="ml-1 text-rose-600">*</span> : null}
      {fieldContent}
      {field.help_text ? <p className="mt-1 text-xs text-slate-500">{field.help_text}</p> : null}
      {error ? <p className="mt-1 text-xs font-medium text-rose-700">{error}</p> : null}
    </div>
  );
}

function renderFieldControl(
  field: WorkflowFormField,
  value: unknown,
  evidence: EvidenceRecord[],
  disabled: boolean,
  invalid: boolean,
  controlClassName: string,
  onChange: (value: unknown) => void,
) {
  if (field.field_type === 'textarea') {
    return <textarea className={controlClassName} rows={4} value={textValue(value)} disabled={disabled} onChange={(event) => onChange(event.target.value)} />;
  }
  if (field.field_type === 'select') {
    return (
      <select className={controlClassName} value={textValue(value)} disabled={disabled} onChange={(event) => onChange(event.target.value)}>
        <option value="">Chọn</option>
        {fieldOptions(field).map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
      </select>
    );
  }
  if (field.field_type === 'multi_select') {
    const selected = new Set(arrayValue(value));
    return (
      <select
        multiple
        className={clsx(controlClassName, 'min-h-28')}
        value={Array.from(selected)}
        disabled={disabled}
        onChange={(event) => onChange(Array.from(event.target.selectedOptions).map((option) => option.value))}
      >
        {fieldOptions(field).map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
      </select>
    );
  }
  if (field.field_type === 'checkbox') {
    return (
      <span className={clsx('mt-1 flex items-center gap-2 rounded-md border px-3 py-2', invalid ? 'border-rose-300 bg-rose-50' : 'border-line bg-white')}>
        <input type="checkbox" className="h-4 w-4 rounded border-line" checked={Boolean(value)} disabled={disabled} onChange={(event) => onChange(event.target.checked)} />
        <span className="text-sm font-medium text-slate-700">Xác nhận</span>
      </span>
    );
  }
  if (field.field_type === 'evidence_reference') {
    const selectableEvidence = evidence.filter((record) => record.status !== 'rejected');
    return (
      <select className={controlClassName} value={textValue(value)} disabled={disabled || selectableEvidence.length === 0} onChange={(event) => onChange(event.target.value)}>
        <option value="">{selectableEvidence.length ? 'Chọn minh chứng' : 'Chưa có minh chứng'}</option>
        {selectableEvidence.map((record) => (
          <option key={record.id} value={record.id}>{evidenceOptionLabel(record)}</option>
        ))}
      </select>
    );
  }
  if (field.field_type === 'structured_list') {
    if (field.field_key === 'checklist_items') {
      return <ChecklistStructuredListControl field={field} value={value} evidence={evidence} disabled={disabled} invalid={invalid} onChange={onChange} />;
    }
    return <StructuredListControl field={field} value={value} evidence={evidence} disabled={disabled} invalid={invalid} onChange={onChange} />;
  }
  return (
    <input
      type={field.field_type === 'number' || field.field_type === 'date' ? field.field_type : 'text'}
      className={controlClassName}
      value={textValue(value)}
      disabled={disabled}
      onChange={(event) => onChange(field.field_type === 'number' && event.target.value !== '' ? Number(event.target.value) : event.target.value)}
    />
  );
}

function evidenceOptionLabel(record: EvidenceRecord) {
  const title = record.name ?? record.title ?? record.document_name ?? record.id;
  const kind = enumLabel('evidence_type', record.evidence_type);
  return `${title} · ${kind} · ${record.status}`;
}

function fieldOptions(field: WorkflowFormField) {
  return (field.options_json ?? []).map((option) => {
    if (typeof option === 'string') return { value: option, label: option };
    return { value: option.value, label: option.label ?? option.value };
  }).filter((option) => option.value);
}

function textValue(value: unknown) {
  if (value === null || value === undefined) return '';
  return String(value);
}

function arrayValue(value: unknown) {
  if (Array.isArray(value)) return value.map((item) => String(item));
  return [];
}

function valueEmpty(field: WorkflowFormField, value: unknown) {
  if (value === null || value === undefined) return true;
  if (field.field_type === 'checkbox') return value !== true;
  if (field.field_type === 'structured_list') return structuredRows(value).length === 0;
  if (Array.isArray(value)) return value.length === 0;
  return String(value).trim() === '';
}

function validateFieldValue(field: WorkflowFormField, value: unknown) {
  if (field.required && valueEmpty(field, value)) return `${field.label} là bắt buộc`;
  if (field.field_type !== 'structured_list' || valueEmpty(field, value)) return '';
  const fields = structuredItemFields(field);
  const rows = structuredRows(value);
  for (const [rowIndex, row] of rows.entries()) {
    for (const spec of fields) {
      if (!spec.required) continue;
      const cellValue = row[spec.key];
      if (cellValue === null || cellValue === undefined || String(cellValue).trim() === '') {
        return `${field.label} dòng ${rowIndex + 1}: ${spec.label} là bắt buộc`;
      }
    }
    if (field.field_key === 'checklist_items' && !['done', 'not_applicable'].includes(String(row.status ?? ''))) {
      return `${field.label} dòng ${rowIndex + 1}: trạng thái phải là Hoàn tất hoặc Không áp dụng trước khi gửi`;
    }
  }
  return '';
}

type StructuredFieldSpec = {
  key: string;
  label: string;
  type: 'text' | 'textarea' | 'number' | 'date' | 'select' | 'evidence_reference' | string;
  required?: boolean;
  options?: Array<{ value: string; label?: string }>;
};

function StructuredListControl({
  field,
  value,
  evidence,
  disabled,
  invalid,
  onChange,
}: {
  field: WorkflowFormField;
  value: unknown;
  evidence: EvidenceRecord[];
  disabled: boolean;
  invalid: boolean;
  onChange: (value: unknown) => void;
}) {
  const rows = structuredRows(value);
  const specs = structuredItemFields(field);
  const addRow = () => {
    const nextRow = Object.fromEntries(specs.map((spec) => [spec.key, '']));
    onChange([...rows, nextRow]);
  };
  const updateCell = (rowIndex: number, key: string, nextValue: string) => {
    onChange(rows.map((row, index) => (index === rowIndex ? { ...row, [key]: nextValue } : row)));
  };
  const removeRow = (rowIndex: number) => {
    onChange(rows.filter((_, index) => index !== rowIndex));
  };
  return (
    <div className={clsx('mt-1 rounded-md border p-2', invalid ? 'border-rose-300 bg-rose-50' : 'border-line bg-white')}>
      {rows.length ? (
        <div className="space-y-2">
          {rows.map((row, rowIndex) => (
            <div key={rowIndex} className="rounded-md border border-line bg-slate-50 p-2">
              <div className="grid gap-2 md:grid-cols-12">
                {specs.map((spec) => (
                  <StructuredCell
                    key={spec.key}
                    spec={spec}
                    value={row[spec.key]}
                    evidence={evidence}
                    disabled={disabled}
                    onChange={(nextValue) => updateCell(rowIndex, spec.key, nextValue)}
                  />
                ))}
                <div className="flex items-end justify-end md:col-span-1">
                  <button
                    type="button"
                    title="Xóa dòng"
                    className="rounded-md border border-line bg-white p-2 text-slate-400 hover:bg-rose-50 hover:text-rose-600 disabled:cursor-not-allowed disabled:opacity-40"
                    disabled={disabled}
                    onClick={() => removeRow(rowIndex)}
                  >
                    <Trash2 className="h-4 w-4" />
                  </button>
                </div>
              </div>
            </div>
          ))}
        </div>
      ) : (
        <div className="rounded-md border border-dashed border-line px-3 py-4 text-sm font-medium text-slate-500">Chưa có dòng nào</div>
      )}
      <button
        type="button"
        className="mt-2 inline-flex items-center gap-2 rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100 disabled:cursor-not-allowed disabled:opacity-50"
        disabled={disabled || specs.length === 0}
        onClick={addRow}
      >
        <Plus className="h-4 w-4" />
        Thêm dòng
      </button>
    </div>
  );
}

function ChecklistStructuredListControl({
  field,
  value,
  evidence,
  disabled,
  invalid,
  onChange,
}: {
  field: WorkflowFormField;
  value: unknown;
  evidence: EvidenceRecord[];
  disabled: boolean;
  invalid: boolean;
  onChange: (value: unknown) => void;
}) {
  const rows = structuredRows(value);
  const specs = structuredItemFields(field);
  const stats = checklistStatsFromRows(rows);
  const addRow = () => {
    const nextRow = Object.fromEntries(specs.map((spec) => [spec.key, defaultChecklistCellValue(spec)]));
    onChange([...rows, nextRow]);
  };
  const updateCell = (rowIndex: number, key: string, nextValue: string) => {
    onChange(rows.map((row, index) => (index === rowIndex ? { ...row, [key]: nextValue } : row)));
  };
  const removeRow = (rowIndex: number) => {
    onChange(rows.filter((_, index) => index !== rowIndex));
  };

  return (
    <div className={clsx('mt-1 rounded-md border p-2', invalid ? 'border-rose-300 bg-rose-50' : 'border-line bg-white')}>
      <div className="mb-2 flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex flex-wrap gap-1.5 text-xs font-semibold">
          <ChecklistStat label="Dòng" value={stats.total} />
          <ChecklistStat label="Hoàn tất/Không áp dụng" value={stats.complete} />
          <ChecklistStat label="Cần xử lý" value={stats.needsAction} tone={stats.needsAction ? 'warn' : 'ok'} />
          <ChecklistStat label="Minh chứng" value={stats.withEvidence} />
        </div>
        <button
          type="button"
          className="inline-flex items-center gap-2 rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100 disabled:cursor-not-allowed disabled:opacity-50"
          disabled={disabled || specs.length === 0}
          onClick={addRow}
        >
          <Plus className="h-4 w-4" />
          Thêm dòng
        </button>
      </div>
      {rows.length ? (
        <div className="space-y-2">
          {rows.map((row, rowIndex) => (
            <div key={rowIndex} className="rounded-md border border-line bg-slate-50 p-2">
              <div className="mb-2 flex flex-wrap items-center justify-between gap-2">
                <div className="flex min-w-0 items-center gap-2">
                  <span className="rounded-md bg-white px-2 py-1 text-xs font-semibold text-slate-600 ring-1 ring-inset ring-line">#{rowIndex + 1}</span>
                  <StatusBadge status={textValue(row.status || 'draft')} />
                  {row.evidence_id ? <span className="truncate text-xs font-medium text-slate-500">Đã liên kết minh chứng</span> : null}
                </div>
                <button
                  type="button"
                  title="Xóa dòng"
                  className="rounded-md border border-line bg-white p-2 text-slate-400 hover:bg-rose-50 hover:text-rose-600 disabled:cursor-not-allowed disabled:opacity-40"
                  disabled={disabled}
                  onClick={() => removeRow(rowIndex)}
                >
                  <Trash2 className="h-4 w-4" />
                </button>
              </div>
              <div className="grid gap-2 md:grid-cols-12">
                {specs.map((spec) => (
                  <StructuredCell
                    key={spec.key}
                    spec={spec}
                    value={row[spec.key]}
                    evidence={evidence}
                    disabled={disabled}
                    onChange={(nextValue) => updateCell(rowIndex, spec.key, nextValue)}
                  />
                ))}
              </div>
            </div>
          ))}
        </div>
      ) : (
        <div className="rounded-md border border-dashed border-line px-3 py-4 text-sm font-medium text-slate-500">Chưa có dòng checklist</div>
      )}
    </div>
  );
}

function ChecklistStat({ label, value, tone = 'neutral' }: { label: string; value: number; tone?: 'neutral' | 'ok' | 'warn' }) {
  return (
    <span className={clsx(
      'rounded-md px-2 py-1 ring-1 ring-inset',
      tone === 'warn' ? 'bg-amber-50 text-amber-800 ring-amber-200' : tone === 'ok' ? 'bg-emerald-50 text-emerald-700 ring-emerald-200' : 'bg-white text-slate-600 ring-line',
    )}>
      {label}: {value}
    </span>
  );
}

function defaultChecklistCellValue(spec: StructuredFieldSpec) {
  return '';
}

function checklistStatsFromRows(rows: AnyRecord[]): ChecklistStats {
  const complete = rows.filter((row) => ['done', 'not_applicable'].includes(String(row.status ?? ''))).length;
  return {
    total: rows.length,
    complete,
    needsAction: rows.length - complete,
    withEvidence: rows.filter((row) => Boolean(row.evidence_id)).length,
  };
}

function StructuredCell({
  spec,
  value,
  evidence,
  disabled,
  onChange,
}: {
  spec: StructuredFieldSpec;
  value: unknown;
  evidence: EvidenceRecord[];
  disabled: boolean;
  onChange: (value: string) => void;
}) {
  const className = 'mt-1 w-full rounded-md border border-line bg-white px-2 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-100 disabled:bg-slate-100';
  const colSpan = spec.type === 'textarea' ? 'md:col-span-5' : spec.type === 'number' ? 'md:col-span-2' : 'md:col-span-3';
  const selectableEvidence = evidence.filter((record) => record.status !== 'rejected');
  return (
    <label className={clsx('block min-w-0', colSpan)}>
      <span className="text-xs font-semibold text-slate-600">
        {spec.label}{spec.required ? <span className="ml-1 text-rose-600">*</span> : null}
      </span>
      {spec.type === 'textarea' ? (
        <textarea className={className} rows={2} value={textValue(value)} disabled={disabled} onChange={(event) => onChange(event.target.value)} />
      ) : spec.type === 'select' ? (
        <select className={className} value={textValue(value)} disabled={disabled} onChange={(event) => onChange(event.target.value)}>
          <option value="">Chọn</option>
          {(spec.options ?? []).map((option) => <option key={option.value} value={option.value}>{option.label ?? option.value}</option>)}
        </select>
      ) : spec.type === 'evidence_reference' ? (
        <select className={className} value={textValue(value)} disabled={disabled || selectableEvidence.length === 0} onChange={(event) => onChange(event.target.value)}>
          <option value="">{selectableEvidence.length ? 'Chọn minh chứng' : 'Chưa có minh chứng'}</option>
          {selectableEvidence.map((record) => (
            <option key={record.id} value={record.id}>
              {evidenceOptionLabel(record)}
            </option>
          ))}
        </select>
      ) : (
        <input
          className={className}
          type={spec.type === 'date' || spec.type === 'number' ? spec.type : 'text'}
          value={textValue(value)}
          disabled={disabled}
          onChange={(event) => onChange(event.target.value)}
        />
      )}
    </label>
  );
}

function structuredRows(value: unknown): AnyRecord[] {
  if (!Array.isArray(value)) return [];
  return value.filter((row): row is AnyRecord => Boolean(row) && typeof row === 'object' && !Array.isArray(row));
}

function structuredItemFields(field: WorkflowFormField): StructuredFieldSpec[] {
  const rawFields = Array.isArray(field.validation_json?.item_fields) ? field.validation_json.item_fields : [];
  return rawFields
    .filter((spec): spec is AnyRecord => Boolean(spec) && typeof spec === 'object' && !Array.isArray(spec) && Boolean(spec.key))
    .map((spec) => ({
      key: String(spec.key),
      label: String(spec.label ?? spec.key),
      type: String(spec.type ?? 'text'),
      required: Boolean(spec.required),
      options: Array.isArray(spec.options)
        ? spec.options
            .filter((option): option is AnyRecord => Boolean(option) && typeof option === 'object' && !Array.isArray(option) && Boolean(option.value))
            .map((option) => ({ value: String(option.value), label: option.label ? String(option.label) : undefined }))
        : [],
    }));
}
