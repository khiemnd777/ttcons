import { useEffect, useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { AlertTriangle, CheckCircle2, Download, ExternalLink, FileText, Loader2, PenLine, Plus, Trash2 } from 'lucide-react';
import clsx from 'clsx';
import { EmptyState } from '../../components/EmptyState';
import { LoadingSpinner } from '../../components/LoadingSpinner';
import { StatusBadge } from '../../components/StatusBadge';
import { useToast } from '../../components/ToastProvider';
import { downloadBlob, get, post, put } from '../../lib/api/client';
import { useAuth } from '../../lib/auth/AuthContext';
import type { AnyRecord, EvidenceRecord, WorkflowFormDetail, WorkflowFormField, WorkflowFormPDFExport, WorkflowFormSignature } from '../../lib/types/domain';
import { enumLabel } from '../../lib/utils/status';
import { SignatureCaptureDialog, type SignatureStroke } from './SignatureCaptureDialog';
import { compactDateTime } from './workItemPresentation';

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
  const form = useQuery({
    queryKey: ['work-item-form', workItemId],
    queryFn: () => get<WorkflowFormDetail>(`/work-items/${workItemId}/form`),
    enabled: Boolean(workItemId),
    retry: false,
  });

  const fields = useMemo(() => form.data?.fields ?? [], [form.data?.fields]);
  const instance = form.data?.instance;
  const submitted = instance?.status === 'submitted';
  const businessFormTitle = form.data?.template?.name ?? instance?.title ?? 'Biểu mẫu nghiệp vụ';
  const signatures = form.data?.signatures ?? [];
  const pdfExports = form.data?.pdf_exports ?? [];
  const signedByCurrentUser = Boolean(user?.id && signatures.some((signature) => signature.signer_user_id === user.id && signature.purpose === 'internal_ack'));
  const validationIssues = useMemo(() => fields.map((field) => validateFieldValue(field, values[field.field_key])).filter(Boolean), [fields, values]);
  const checklistField = fields.find((field) => field.field_type === 'structured_list' && field.field_key === 'checklist_items');
  const checklistStats = checklistField ? checklistStatsFromRows(structuredRows(values[checklistField.field_key])) : null;
  const activeEvidence = evidence.filter((record) => record.status !== 'rejected');

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
      toast.success('Đã lưu nháp biểu mẫu nghiệp vụ');
    },
    onError: (error) => toast.error('Không lưu được biểu mẫu nghiệp vụ', error),
  });

  const submitForm = useMutation({
    mutationFn: () => post<WorkflowFormDetail>(`/workflow-form-instances/${instance?.id}/submit`, { answers: values }),
    onSuccess: () => {
      invalidate();
      toast.success('Đã submit biểu mẫu nghiệp vụ');
    },
    onError: (error) => toast.error('Không submit được biểu mẫu nghiệp vụ', error),
  });

  const signForm = useMutation({
    mutationFn: (payload: { purpose: string; comment: string; signature_image_data_url: string; signature_strokes: SignatureStroke[] }) =>
      post<WorkflowFormDetail>(`/workflow-form-instances/${instance?.id}/sign`, payload),
    onSuccess: () => {
      setSignatureDialogOpen(false);
      invalidate();
      toast.success('Đã ký và lưu visual stamp');
    },
    onError: (error) => toast.error('Không ký được biểu mẫu nghiệp vụ', error),
  });

  const generatePDF = useMutation({
    mutationFn: () => post<WorkflowFormDetail>(`/workflow-form-instances/${instance?.id}/pdf-exports`, {}),
    onSuccess: () => {
      invalidate();
      toast.success('Đã generate PDF biểu mẫu nghiệp vụ');
    },
    onError: (error) => toast.error('Không generate được PDF', error),
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
      toast.warn({ title: 'Thiếu dữ liệu biểu mẫu nghiệp vụ', description: labels.join(', ') });
      return false;
    }
    return true;
  }

  if (form.isLoading) {
    return (
      <section>
        <h3 className="mb-2 text-sm font-semibold text-ink">Biểu mẫu nghiệp vụ</h3>
        <div className="rounded-md border border-line bg-white p-4">
          <LoadingSpinner />
        </div>
      </section>
    );
  }

  if (form.isError) {
    return (
      <section>
        <h3 className="mb-2 text-sm font-semibold text-ink">Biểu mẫu nghiệp vụ</h3>
        <div className="rounded-md border border-rose-100 bg-rose-50 p-4 text-sm text-rose-700">
          <p className="font-semibold">Không tải được biểu mẫu nghiệp vụ</p>
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
        <h3 className="mb-2 text-sm font-semibold text-ink">Biểu mẫu nghiệp vụ</h3>
        <EmptyState title="Step này chưa có biểu mẫu nghiệp vụ." />
      </section>
    );
  }

  return (
    <section>
      <div className="mb-2 flex flex-wrap items-center justify-between gap-2">
        <div>
          <h3 className="text-sm font-semibold text-ink">{businessFormTitle}</h3>
          <p className="mt-1 text-xs text-slate-500">
            Biểu mẫu nghiệp vụ v{form.data.version?.version_no ?? 1}
          </p>
        </div>
        <StatusBadge status={instance?.status ?? 'draft'} />
      </div>
      <div className="rounded-md border border-line bg-white p-3">
        <WorkflowBusinessSummary
          submitted={submitted}
          validationIssueCount={validationIssues.length}
          evidenceCount={activeEvidence.length}
          checklistStats={checklistStats}
          signatures={signatures}
          pdfExports={pdfExports}
        />
        <div className="grid gap-3 md:grid-cols-2">
          {fields.map((field) => (
            <WorkflowFieldControl
              key={field.field_key}
              field={field}
              value={values[field.field_key]}
              evidence={evidence}
              error={errors[field.field_key]}
              disabled={submitted || saveDraft.isPending || submitForm.isPending}
              onChange={(value) => setFieldValue(field, value)}
            />
          ))}
        </div>
        <div className="mt-4 flex flex-wrap items-center justify-between gap-2 border-t border-line pt-3">
          <div className="text-xs text-slate-500">
            {instance?.submitted_at ? `Submitted ${compactDateTime(instance.submitted_at)}` : `Updated ${compactDateTime(instance?.updated_at)}`}
          </div>
          <div className="flex flex-wrap justify-end gap-2">
            <button
              className="inline-flex items-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100 disabled:cursor-not-allowed disabled:opacity-60"
              disabled={submitted || saveDraft.isPending || submitForm.isPending}
              onClick={() => saveDraft.mutate()}
            >
              {saveDraft.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : null}
              Lưu nháp
            </button>
            <button
              className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60"
              disabled={submitted || saveDraft.isPending || submitForm.isPending}
              onClick={() => {
                if (validateBeforeSubmit()) submitForm.mutate();
              }}
            >
              {submitForm.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : null}
              Submit biểu mẫu
            </button>
          </div>
        </div>
        <div className="mt-3 grid gap-3 xl:grid-cols-2">
          <WorkflowSignaturePanel
            signatures={signatures}
            submitted={submitted}
            signedByCurrentUser={signedByCurrentUser}
            signing={signForm.isPending}
            onOpenSign={() => setSignatureDialogOpen(true)}
            onOpenVisual={(signature) => void openProtectedBlob(`/workflow-form-signatures/${signature.id}/visual`, 'Không mở được chữ ký')}
          />
          <WorkflowPDFExportPanel
            exports={pdfExports}
            submitted={submitted}
            generating={generatePDF.isPending}
            onGenerate={() => generatePDF.mutate()}
            onOpen={(record) => void openProtectedBlob(`/workflow-form-pdf-exports/${record.id}/file`, 'Không mở được PDF biểu mẫu nghiệp vụ')}
          />
        </div>
        {form.data.events?.length ? (
          <div className="mt-3 space-y-1 border-t border-line pt-3">
            {form.data.events.slice(0, 3).map((event) => (
              <p key={event.id} className="text-xs text-slate-500">
                {event.actor_name ?? 'System'} · {enumLabel('event_type', String(event.event_type))} · {compactDateTime(event.created_at)}
              </p>
            ))}
          </div>
        ) : null}
      </div>
      <SignatureCaptureDialog
        open={signatureDialogOpen}
        signerName={user?.full_name}
        submitting={signForm.isPending}
        onClose={() => setSignatureDialogOpen(false)}
        onSubmit={(payload) => signForm.mutate(payload)}
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

function WorkflowBusinessSummary({
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
  const blocked = !submitted && validationIssueCount > 0;
  const tone = blocked ? 'border-amber-200 bg-amber-50' : 'border-blue-100 bg-blue-50';
  return (
    <div className={clsx('mb-4 rounded-md border p-3', tone)}>
      <div className="flex flex-col gap-3 xl:flex-row xl:items-center xl:justify-between">
        <div className="flex min-w-0 items-start gap-3">
          <span className="mt-0.5 rounded-md bg-white/80 p-2 ring-1 ring-inset ring-white">
            {blocked ? <AlertTriangle className="h-4 w-4 text-amber-700" /> : <CheckCircle2 className="h-4 w-4 text-blue-700" />}
          </span>
          <div className="min-w-0">
            <p className="text-sm font-semibold text-ink">Next action: {nextAction}</p>
            <p className="mt-1 text-xs leading-5 text-slate-600">
              {submitted ? 'Form đã khóa nội dung; chữ ký và PDF dùng bản revision đã submit.' : `${validationIssueCount} validation item cần xử lý trước submit.`}
            </p>
          </div>
        </div>
        <div className="grid gap-2 text-xs font-semibold text-slate-600 sm:grid-cols-4 xl:w-[36rem]">
          <WorkflowStepPill label="Checklist" value={checklistStats ? `${checklistStats.complete}/${checklistStats.total} done` : 'N/A'} active={!submitted && Boolean(checklistStats?.needsAction)} />
          <WorkflowStepPill label="Evidence" value={`${evidenceCount} active`} active={!submitted && evidenceCount === 0} />
          <WorkflowStepPill label="Signature" value={`${signatures.length} signed`} active={submitted && signatures.length === 0} />
          <WorkflowStepPill label="PDF" value={`${pdfExports.length} file`} active={submitted && signatures.length > 0 && pdfExports.length === 0} />
        </div>
      </div>
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
  if (!submitted && validationIssueCount > 0) return 'Hoàn tất checklist/form';
  if (!submitted) return 'Submit biểu mẫu nghiệp vụ';
  if (signatureCount === 0) return 'Ký / đóng dấu PDF';
  if (pdfCount === 0) return 'Generate PDF';
  return 'Mở artifact hoặc hoàn tất work item';
}

function WorkflowSignaturePanel({
  signatures,
  submitted,
  signedByCurrentUser,
  signing,
  onOpenSign,
  onOpenVisual,
}: {
  signatures: WorkflowFormSignature[];
  submitted: boolean;
  signedByCurrentUser: boolean;
  signing: boolean;
  onOpenSign: () => void;
  onOpenVisual: (signature: WorkflowFormSignature) => void;
}) {
  const disabled = !submitted || signing;
  return (
    <section className="rounded-md border border-line bg-slate-50 p-3">
      <div className="mb-2 flex flex-wrap items-center justify-between gap-2">
        <h4 className="text-sm font-semibold text-ink">Chữ ký</h4>
        {signatures.length ? <span className="text-xs font-semibold text-slate-500">{signatures.length} signed</span> : null}
      </div>
      {signatures.length ? (
        <div className="space-y-2">
          {signatures.map((signature) => (
            <div key={signature.id} className="rounded-md border border-line bg-slate-50 px-3 py-2">
              <div className="flex flex-wrap items-start justify-between gap-2">
                <div className="min-w-0">
                  <p className="truncate text-sm font-semibold text-ink">{signature.signer_name}</p>
                  <p className="mt-1 text-xs text-slate-500">
                    {signaturePurposeLabel(signature.purpose)} · {compactDateTime(signature.signed_at)} · hash {shortHash(signature.payload_hash)}
                  </p>
                  {signature.visual_checksum_sha256 ? (
                    <p className="mt-1 text-xs text-slate-500">visual sha256:{signature.visual_checksum_sha256.slice(0, 12)}</p>
                  ) : null}
                </div>
                <div className="flex flex-wrap items-center gap-2">
                  {signature.visual_storage_key ? (
                    <button
                      type="button"
                      className="inline-flex items-center gap-1 rounded-md border border-line bg-white px-2 py-1 text-xs font-semibold text-blue-700 hover:border-blue-300 hover:bg-blue-50"
                      onClick={() => onOpenVisual(signature)}
                    >
                      <ExternalLink className="h-3.5 w-3.5" />
                      Visual
                    </button>
                  ) : null}
                  <span className="inline-flex items-center gap-1 rounded-md bg-emerald-50 px-2 py-1 text-xs font-semibold text-emerald-700 ring-1 ring-inset ring-emerald-200">
                    <CheckCircle2 className="h-3.5 w-3.5" />
                    signed
                  </span>
                </div>
              </div>
              {signature.comment ? <p className="mt-2 text-sm text-slate-700">{signature.comment}</p> : null}
            </div>
          ))}
        </div>
      ) : (
        <div className="rounded-md border border-dashed border-line px-3 py-4 text-sm font-medium text-slate-500">
          {submitted ? 'Chưa có chữ ký nội bộ.' : 'Submit biểu mẫu trước khi ký nội bộ.'}
        </div>
      )}
      <div className="mt-3 flex flex-wrap items-center justify-between gap-2">
        <div className="text-xs text-slate-500">
          {!submitted ? 'Submit biểu mẫu trước khi ký.' : signedByCurrentUser ? 'Bạn đã ký Internal ack; vẫn có thể ký purpose khác nếu cần.' : 'Canvas signature sẽ được lưu như protected asset.'}
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
        <h4 className="text-sm font-semibold text-ink">PDF artifacts</h4>
        <button
          type="button"
          className="inline-flex items-center gap-2 rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100 disabled:cursor-not-allowed disabled:opacity-60"
          disabled={!submitted || generating}
          onClick={onGenerate}
        >
          {generating ? <Loader2 className="h-4 w-4 animate-spin" /> : <FileText className="h-4 w-4" />}
          Generate PDF
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
                    Rev {record.form_revision_no} · {compactDateTime(record.generated_at)} · sha256:{record.checksum_sha256.slice(0, 12)}
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
          {submitted ? 'Chưa generate PDF cho biểu mẫu này.' : 'Submit biểu mẫu trước khi generate PDF.'}
        </div>
      )}
    </section>
  );
}

function shortHash(hash?: string) {
  return hash ? hash.slice(0, 8) : '-';
}

function signaturePurposeLabel(purpose?: string) {
  switch (purpose) {
    case 'prepared_by':
      return 'Prepared by';
    case 'reviewed_by':
      return 'Reviewed by';
    case 'approved_by':
      return 'Approved by';
    case 'received_by':
      return 'Received by';
    case 'internal_ack':
    default:
      return 'Internal ack';
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
        <option value="">{selectableEvidence.length ? 'Chọn evidence' : 'Chưa có evidence'}</option>
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
      return `${field.label} dòng ${rowIndex + 1}: status phải là Done hoặc N/A trước khi submit`;
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
          <ChecklistStat label="Rows" value={stats.total} />
          <ChecklistStat label="Done/N/A" value={stats.complete} />
          <ChecklistStat label="Needs action" value={stats.needsAction} tone={stats.needsAction ? 'warn' : 'ok'} />
          <ChecklistStat label="Evidence" value={stats.withEvidence} />
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
                  {row.evidence_id ? <span className="truncate text-xs font-medium text-slate-500">Evidence linked</span> : null}
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
          <option value="">{selectableEvidence.length ? 'Chọn evidence' : 'Chưa có evidence'}</option>
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
