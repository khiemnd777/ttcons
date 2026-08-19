import { useEffect, useImperativeHandle, useMemo, useRef, useState, type ReactNode, type Ref } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { AlertTriangle, CheckCircle2, ChevronDown, ChevronRight, Download, ExternalLink, FileText, Loader2, LockKeyhole, PenLine, Plus, Stamp, Trash2 } from 'lucide-react';
import clsx from 'clsx';
import { Dialog } from '../../components/Dialog';
import { CurrencyInput } from '../../components/CurrencyInput';
import { EmptyState } from '../../components/EmptyState';
import { LoadingSpinner } from '../../components/LoadingSpinner';
import { StatusBadge } from '../../components/StatusBadge';
import { useToast } from '../../components/ToastProvider';
import { ApiError, del, downloadBlob, get, post, postForm, put } from '../../lib/api/client';
import { useAuth } from '../../lib/auth/AuthContext';
import type { AnyRecord, EvidenceRecord, SealAsset, WorkflowFormDetail, WorkflowFormField, WorkflowFormPDFConfirmationPreference, WorkflowFormPDFExport, WorkflowFormSealApplication, WorkflowFormSignature } from '../../lib/types/domain';
import { dateText, percent } from '../../lib/utils/format';
import { openProtectedBlob } from '../../lib/utils/openProtectedBlob';
import { enumLabel } from '../../lib/utils/status';
import { SignatureCaptureDialog, type SignatureStroke, type WorkflowSealApplicationPayload } from './SignatureCaptureDialog';
import type { WorkItemRequirementNavigationRequest } from './workItemRequirements';
import { firstFocusableRequirementControl, revealWorkItemRequirementTarget } from './workItemRequirementNavigation';
import { compactDateTime, isGenericWorkflowFormTemplate, workflowFormEventLabel, workflowFormStatusLabel, workflowFormTypeLabel } from './workItemPresentation';
import { effectiveWorkflowCurrencyFields } from './workflowCurrencyFields';
import { effectiveWorkflowFormFields } from './workflowFormRules';
import {
  workflowStructuredOtherDetailRequired,
  workflowTopLevelOtherDetailDependentKeys,
  workflowTopLevelOtherDetailRequired,
} from './workflowOtherDetailRules';
import {
  workflowSelectOptionChangeValue,
  workflowSelectOptionValue,
  workflowSelectOtherTextChangeValue,
  workflowSelectOtherTextConfig,
  workflowSelectOtherTextIssue,
  workflowSelectOtherTextValue,
  workflowSelectSupportsOtherText,
} from './workflowSelectOtherText';
import { effectiveWorkflowStructuredListFields } from './workflowStructuredListFields';

type FormErrors = Record<string, string>;

type PDFConfirmationPreferenceInput = {
  purpose: string;
  include_signatures: boolean;
  include_seals: boolean;
};

type WorkflowFormInstanceMutationDetail = Omit<WorkflowFormDetail, 'available' | 'auto_advance_when_ready'>;

type ConfirmationPurposeGroup = {
  purpose: string;
  signatures: WorkflowFormSignature[];
  sealApplications: WorkflowFormSealApplication[];
  hasBothKinds: boolean;
  includeSignatures: boolean;
  includeSeals: boolean;
};

export type WorkflowFormActionState = {
  available: boolean;
  canAmend: boolean;
  amending: boolean;
  amendmentSaving: boolean;
  submitted: boolean;
  readOnly: boolean;
  saving: boolean;
  submitting: boolean;
  submitLabel: string;
  validationIssueLabels: string[];
  title: string;
  status: string;
  versionNo: number;
  currentRevision: number;
  updatedAt?: string;
  submittedAt?: string;
};

export type WorkflowFormActionHandle = {
  save: () => void;
  submit: () => void;
  beginAmendment: (reason: string) => void;
  saveAmendment: () => void;
  cancelAmendment: () => void;
};

export function WorkflowFormPanel({
  workItemId,
  projectId,
  evidence,
  onChanged,
  readOnly = false,
  navigationRequest,
  externalActions = false,
  actionRef,
  onActionStateChange,
}: {
  workItemId: string;
  projectId?: string;
  evidence: EvidenceRecord[];
  onChanged?: () => void;
  readOnly?: boolean;
  navigationRequest?: WorkItemRequirementNavigationRequest | null;
  externalActions?: boolean;
  actionRef?: Ref<WorkflowFormActionHandle>;
  onActionStateChange?: (state: WorkflowFormActionState | null) => void;
}) {
  const queryClient = useQueryClient();
  const toast = useToast();
  const { user, hasPermission } = useAuth();
  const [values, setValues] = useState<AnyRecord>({});
  const [errors, setErrors] = useState<FormErrors>({});
  const [confirmationDialogOpen, setConfirmationDialogOpen] = useState(false);
  const [signatureDetail, setSignatureDetail] = useState<WorkflowFormSignature | null>(null);
  const [sealApplicationDetail, setSealApplicationDetail] = useState<WorkflowFormSealApplication | null>(null);
  const [pdfDeleteCandidate, setPDFDeleteCandidate] = useState<WorkflowFormPDFExport | null>(null);
  const [amending, setAmending] = useState(false);
  const [amendmentReason, setAmendmentReason] = useState('');
  const [fieldsOpen, setFieldsOpen] = useState(true);
  const [optionalFieldsOpen, setOptionalFieldsOpen] = useState(false);
  const [documentsOpen, setDocumentsOpen] = useState(false);
  const panelRef = useRef<HTMLElement | null>(null);
  const actionCenterRef = useRef<HTMLDivElement | null>(null);
  const fieldsSectionRef = useRef<HTMLElement | null>(null);
  const lastHandledNavigationRef = useRef<number | null>(null);
  const form = useQuery({
    queryKey: ['work-item-form', workItemId],
    queryFn: () => get<WorkflowFormDetail>(`/work-items/${workItemId}/form`),
    enabled: Boolean(workItemId),
    retry: false,
  });
  const canUseSeals = hasPermission('seal.use');
  const canManageSeals = hasPermission('seal.manage');
  const sealAssets = useQuery({
    queryKey: ['seal-assets', 'active'],
    queryFn: () => get<SealAsset[]>('/seal-assets'),
    enabled: canUseSeals && confirmationDialogOpen,
    retry: false,
  });

  const templateCode = form.data?.template?.code;
  const fields = useMemo(
    () => effectiveWorkflowStructuredListFields(
      effectiveWorkflowCurrencyFields(form.data?.fields ?? [], templateCode),
      templateCode,
    ),
    [form.data?.fields, templateCode],
  );
  const instance = form.data?.instance;
  const submitted = instance?.status === 'submitted';
  const currentRevision = Number(instance?.current_revision ?? 0);
  const canAmend = Boolean(form.data?.capabilities?.can_amend) && submitted && !readOnly;
  const businessFormTitle = workflowFormPanelTitle(
    instance?.title,
    form.data?.template?.code,
    form.data?.template?.name,
    instance?.form_type,
  );
  const allSignatures = form.data?.signatures ?? [];
  const allSealApplications = form.data?.seal_applications ?? [];
  const signatures = allSignatures.filter((signature) => signature.is_current_revision ?? signature.form_revision_no === currentRevision);
  const sealApplications = allSealApplications.filter((application) => application.is_current_revision ?? application.form_revision_no === currentRevision);
  const historicalSignatures = allSignatures.filter((signature) => !(signature.is_current_revision ?? signature.form_revision_no === currentRevision));
  const historicalSealApplications = allSealApplications.filter((application) => !(application.is_current_revision ?? application.form_revision_no === currentRevision));
  const pdfConfirmationPreferences = form.data?.pdf_confirmation_preferences ?? [];
  const pdfExports = form.data?.pdf_exports ?? [];
  const confirmationGroups = useMemo(
    () => buildConfirmationPurposeGroups(signatures, sealApplications, pdfConfirmationPreferences),
    [pdfConfirmationPreferences, sealApplications, signatures],
  );
  const pdfConfirmationCounts = useMemo(
    () => selectedPDFConfirmationCounts(confirmationGroups),
    [confirmationGroups],
  );
  const signedPurposesByCurrentUser = useMemo(
    () => (user?.id ? signatures.filter((signature) => signature.signer_user_id === user.id).map((signature) => signature.purpose) : []),
    [signatures, user?.id],
  );
  const signedByCurrentUser = signedPurposesByCurrentUser.includes('internal_ack');
  const activeFields = useMemo(() => effectiveWorkflowFormFields(fields, values), [fields, values]);
  const coreFields = useMemo(() => activeFields.filter((field) => !workflowFormFieldIsOptional(field)), [activeFields]);
  const optionalFields = useMemo(() => activeFields.filter(workflowFormFieldIsOptional), [activeFields]);
  const validationIssues = useMemo(
    () => activeFields.map((field) => validateFieldValue(templateCode, field, values[field.field_key], values)).filter(Boolean),
    [activeFields, templateCode, values],
  );
  const checklistField = activeFields.find((field) => field.field_type === 'structured_list' && field.field_key === 'checklist_items');
  const checklistStats = checklistField ? checklistStatsFromRows(structuredRows(values[checklistField.field_key])) : null;
  const isPlanningDailyReport = instance?.form_type === 'daily_report' && form.data?.template?.code !== 'builtin_construction_daily_report';
  const isMeetingMinutes = instance?.form_type === 'meeting_minutes';
  const isPlanningChecklist = instance?.form_type === 'checklist' && form.data?.template?.code === 'builtin_checklist';
  const activeEvidence = evidence.filter((record) => record.status !== 'rejected');
  const validationIssueLabels = useMemo(
    () => activeFields.filter((field) => validateFieldValue(templateCode, field, values[field.field_key], values)).map((field) => field.label),
    [activeFields, templateCode, values],
  );
  const amendmentHasChanges = useMemo(
    () => !workflowFormAnswersEqual(values, form.data?.answers ?? {}),
    [form.data?.answers, values],
  );

  useEffect(() => {
    if (amending) return;
    setValues(submitted
      ? (form.data?.answers ?? {})
      : { ...(form.data?.suggested_answers ?? {}), ...(form.data?.answers ?? {}) });
    setErrors({});
  }, [amending, form.data?.answers, form.data?.suggested_answers, instance?.id, submitted]);

  useEffect(() => {
    setAmending(false);
    setAmendmentReason('');
  }, [instance?.id]);

  useEffect(() => {
    setFieldsOpen(!submitted);
    setDocumentsOpen(submitted);
    setOptionalFieldsOpen(false);
  }, [instance?.id, submitted]);

  useEffect(() => {
    if (navigationRequest?.target.kind !== 'workflow_form') return undefined;
    if (lastHandledNavigationRef.current === navigationRequest.requestId) return undefined;
    if (form.isLoading) return undefined;
    const navigationTarget = navigationRequest.target;

    let removeHighlight: (() => void) | undefined;
    const timer = window.setTimeout(() => {
      if (!form.data?.available || form.isError) {
        lastHandledNavigationRef.current = navigationRequest.requestId;
        removeHighlight = revealWorkItemRequirementTarget(panelRef.current, panelRef.current);
        return;
      }

      const focusMode = navigationTarget.focus;
      const field = focusMode === 'checklist'
        ? activeFields.find((candidate) => candidate.field_key === 'checklist_items')
        : focusMode === 'first_issue'
          ? activeFields.find((candidate) => validateFieldValue(templateCode, candidate, values[candidate.field_key], values))
          : undefined;

      if (field) {
        if (!fieldsOpen) {
          setFieldsOpen(true);
          return;
        }
        lastHandledNavigationRef.current = navigationRequest.requestId;
        const error = validateFieldValue(templateCode, field, values[field.field_key], values);
        if (error) setErrors((current) => ({ ...current, [field.field_key]: error }));
        const fieldContainer = Array.from(
          panelRef.current?.querySelectorAll<HTMLElement>('[data-workflow-field-key]') ?? [],
        ).find((element) => element.dataset.workflowFieldKey === field.field_key) ?? null;
        removeHighlight = revealWorkItemRequirementTarget(
          fieldContainer,
          firstFocusableRequirementControl(fieldContainer),
        );
        return;
      }

      if (focusMode !== 'source' && !submitted && !readOnly) {
        if (externalActions) {
          lastHandledNavigationRef.current = navigationRequest.requestId;
          return;
        }
        lastHandledNavigationRef.current = navigationRequest.requestId;
        const submitButton = actionCenterRef.current?.querySelector<HTMLElement>('[data-workflow-form-submit]') ?? null;
        removeHighlight = revealWorkItemRequirementTarget(actionCenterRef.current, submitButton);
        return;
      }

      lastHandledNavigationRef.current = navigationRequest.requestId;
      const sourceContainer = actionCenterRef.current ?? fieldsSectionRef.current ?? panelRef.current;
      removeHighlight = revealWorkItemRequirementTarget(sourceContainer, sourceContainer);
    }, 0);

    return () => {
      window.clearTimeout(timer);
      removeHighlight?.();
    };
  }, [activeFields, externalActions, fieldsOpen, form.data?.available, form.isError, form.isLoading, navigationRequest?.requestId, navigationRequest?.target.kind, readOnly, submitted, templateCode, values]);

  const invalidate = () => {
    queryClient.invalidateQueries({ queryKey: ['work-item-form', workItemId] });
    queryClient.invalidateQueries({ queryKey: ['work-item', workItemId] });
    queryClient.invalidateQueries({ queryKey: ['my-work-items'] });
    queryClient.invalidateQueries({ queryKey: ['outgoing-acceptance', workItemId] });
    if (projectId) queryClient.invalidateQueries({ queryKey: ['project-operations', projectId] });
    onChanged?.();
  };

  function handleWorkflowFormMutationError(error: unknown, fallbackTitle: string) {
    const fieldErrors = workflowFormApiFieldErrors(error);
    if (!Object.keys(fieldErrors).length) {
      toast.error(fallbackTitle, error);
      return;
    }
    setErrors(fieldErrors);
    setFieldsOpen(true);
    if (optionalFields.some((field) => fieldErrors[field.field_key])) setOptionalFieldsOpen(true);
    toast.warn({
      title: 'Dữ liệu hồ sơ chưa hợp lệ',
      description: [...new Set(Object.values(fieldErrors))].slice(0, 4).join('; '),
    });
  }

  const saveDraft = useMutation({
    mutationFn: () => put<WorkflowFormDetail>(`/workflow-form-instances/${instance?.id}/draft`, { answers: values }),
    onSuccess: () => {
      invalidate();
      toast.success('Đã lưu nháp hồ sơ nghiệp vụ');
    },
    onError: (error) => handleWorkflowFormMutationError(error, 'Không lưu được hồ sơ nghiệp vụ'),
  });

  const submitForm = useMutation({
    mutationFn: () => post<WorkflowFormDetail>(`/workflow-form-instances/${instance?.id}/submit`, { answers: values }),
    onSuccess: (result) => {
      invalidate();
      if (result.auto_transition?.status === 'advanced') {
        toast.success('Đã xác nhận và chuyển sang bước tiếp theo');
      } else if (result.auto_transition?.status === 'blocked') {
        toast.warn({ title: 'Đã lưu lựa chọn nhưng chưa thể chuyển bước', description: result.auto_transition.message ?? 'Kiểm tra điều kiện connector và thử lại.' });
      } else {
        toast.success('Đã chốt hồ sơ nghiệp vụ');
      }
    },
    onError: (error) => handleWorkflowFormMutationError(error, 'Không chốt được hồ sơ nghiệp vụ'),
  });

  const amendForm = useMutation({
    mutationFn: () => post<WorkflowFormDetail>(`/workflow-form-instances/${instance?.id}/amend`, {
      answers: values,
      reason: amendmentReason,
      expected_revision: currentRevision,
    }),
    onSuccess: (result) => {
      queryClient.setQueryData<WorkflowFormDetail>(['work-item-form', workItemId], (current) => ({
        ...result,
        available: true,
        capabilities: current?.capabilities,
      }));
      setAmending(false);
      setAmendmentReason('');
      invalidate();
      toast.success('Đã lưu phiên bản chỉnh sửa và ghi lịch sử thay đổi');
    },
    onError: (error) => {
      if (error instanceof ApiError && error.status === 409) {
        toast.warn({ title: 'Hồ sơ đã có phiên bản mới', description: 'Nội dung đang nhập vẫn được giữ. Hãy sao chép phần cần thiết, huỷ chỉnh sửa rồi mở lại phiên bản mới nhất.' });
        return;
      }
      handleWorkflowFormMutationError(error, 'Không lưu được nội dung chỉnh sửa');
    },
  });

  const signForm = useMutation({
    mutationFn: (payload: { purpose: string; comment: string; signature_image_data_url: string; signature_strokes: SignatureStroke[] }) =>
      post<WorkflowFormDetail>(`/workflow-form-instances/${instance?.id}/sign`, payload),
    onSuccess: () => {
      setConfirmationDialogOpen(false);
      invalidate();
      toast.success('Đã lưu chữ ký cá nhân');
    },
    onError: (error) => toast.error('Không ký được hồ sơ nghiệp vụ', error),
  });

  const applySeal = useMutation({
    mutationFn: (payload: WorkflowSealApplicationPayload) => post<WorkflowFormDetail>(`/workflow-form-instances/${instance?.id}/seal-applications`, payload),
    onSuccess: () => {
      setConfirmationDialogOpen(false);
      invalidate();
      toast.success('Đã đóng dấu công ty vào hồ sơ');
    },
    onError: (error) => toast.error('Không đóng dấu được hồ sơ nghiệp vụ', error),
  });

  const uploadSeal = useMutation({
    mutationFn: async ({ name, file }: { name: string; file: File }) => {
      const body = new FormData();
      body.append('name', name);
      body.append('file', file);
      return postForm<SealAsset>('/seal-assets/upload', body);
    },
    onSuccess: (asset) => {
      queryClient.setQueryData<SealAsset[]>(['seal-assets', 'active'], (current = []) => [asset, ...current.filter((item) => item.id !== asset.id)]);
      void queryClient.invalidateQueries({ queryKey: ['seal-assets'] });
      toast.success('Đã lưu con dấu vào tài nguyên dùng chung');
    },
    onError: (error) => toast.error('Không tải được ảnh con dấu', error),
  });

  const savePDFConfirmationPreference = useMutation({
    mutationFn: (payload: PDFConfirmationPreferenceInput) =>
      put<WorkflowFormInstanceMutationDetail>(`/workflow-form-instances/${instance?.id}/pdf-confirmation-preference`, payload),
    onMutate: async (payload) => {
      const queryKey = ['work-item-form', workItemId] as const;
      await queryClient.cancelQueries({ queryKey });
      const previous = queryClient.getQueryData<WorkflowFormDetail>(queryKey);
      queryClient.setQueryData<WorkflowFormDetail>(queryKey, (current) => optimisticPDFConfirmationPreference(current, payload, user?.id, user?.full_name));
      return { previous, queryKey };
    },
    onSuccess: (_result, _payload, context) => queryClient.invalidateQueries({ queryKey: context.queryKey }),
    onError: (error, _payload, context) => {
      if (context?.previous) queryClient.setQueryData(context.queryKey, context.previous);
      toast.error('Không lưu được lựa chọn hiển thị trong PDF', error);
    },
  });

  const generatePDF = useMutation({
    mutationFn: () => post<WorkflowFormDetail>(`/workflow-form-instances/${instance?.id}/pdf-exports`, {}),
    onSuccess: () => {
      invalidate();
      toast.success('Đã tạo PDF hồ sơ nghiệp vụ');
    },
    onError: (error) => toast.error('Không tạo được PDF', error),
  });

  const deletePDF = useMutation({
    mutationFn: (exportId: string) => del<WorkflowFormDetail>(`/workflow-form-pdf-exports/${exportId}`),
    onSuccess: () => {
      setPDFDeleteCandidate(null);
      invalidate();
      toast.success('Đã xoá tệp PDF hồ sơ nghiệp vụ');
    },
    onError: (error) => toast.error('Không xoá được PDF', error),
  });

  async function openWorkflowBlob(path: string, fallbackTitle: string) {
    try {
      await openProtectedBlob(() => downloadBlob(path));
    } catch (error) {
      toast.error(fallbackTitle, error);
    }
  }

  function setFieldValue(field: WorkflowFormField, value: unknown) {
    setValues((current) => ({ ...current, [field.field_key]: value }));
    setErrors((current) => {
      const clearedKeys = new Set([
        field.field_key,
        ...workflowTopLevelOtherDetailDependentKeys(templateCode, field.field_key),
      ]);
      if (![...clearedKeys].some((key) => current[key])) return current;
      return Object.fromEntries(Object.entries(current).filter(([key]) => !clearedKeys.has(key)));
    });
  }

  function validateBeforeSubmit() {
    const nextErrors: FormErrors = {};
    activeFields.forEach((field) => {
      const value = values[field.field_key];
      const error = validateFieldValue(templateCode, field, value, values);
      if (error) nextErrors[field.field_key] = error;
    });
    setErrors(nextErrors);
    const issues = [...new Set(Object.values(nextErrors))];
    if (issues.length) {
      toast.warn({ title: 'Dữ liệu hồ sơ chưa hợp lệ', description: issues.slice(0, 4).join('; ') });
      return false;
    }
    return true;
  }

  function beginAmendment(reason: string) {
    const normalizedReason = reason.trim();
    if (!canAmend || !normalizedReason) return;
    setValues(form.data?.answers ?? {});
    setErrors({});
    setAmendmentReason(normalizedReason);
    setAmending(true);
    setFieldsOpen(true);
    setDocumentsOpen(false);
  }

  function cancelAmendment() {
    if (amendForm.isPending) return;
    setValues(form.data?.answers ?? {});
    setErrors({});
    setAmending(false);
    setAmendmentReason('');
    setFieldsOpen(false);
    setDocumentsOpen(true);
  }

  function saveAmendment() {
    if (!canAmend || !amending || amendForm.isPending) return;
    if (!amendmentHasChanges) {
      toast.warn({ title: 'Chưa có nội dung thay đổi', description: 'Hãy cập nhật ít nhất một thông tin trước khi lưu phiên bản mới.' });
      return;
    }
    if (validateBeforeSubmit()) amendForm.mutate();
  }

  const submitLabel = form.data?.auto_advance_when_ready ? 'Xác nhận và tiếp tục' : 'Chốt hồ sơ';
  const actionState = useMemo<WorkflowFormActionState>(() => ({
    available: Boolean(form.data?.available) && !form.isLoading && !form.isError,
    canAmend,
    amending,
    amendmentSaving: amendForm.isPending,
    submitted,
    readOnly,
    saving: saveDraft.isPending,
    submitting: submitForm.isPending,
    submitLabel,
    validationIssueLabels,
    title: businessFormTitle,
    status: instance?.status ?? 'draft',
    versionNo: Number(form.data?.version?.version_no ?? 1),
    currentRevision,
    updatedAt: instance?.updated_at,
    submittedAt: instance?.submitted_at,
  }), [amendForm.isPending, amending, businessFormTitle, canAmend, currentRevision, form.data?.available, form.data?.version?.version_no, form.isError, form.isLoading, instance?.status, instance?.submitted_at, instance?.updated_at, readOnly, saveDraft.isPending, submitForm.isPending, submitLabel, submitted, validationIssueLabels]);

  useImperativeHandle(actionRef, () => ({
    save: () => {
      if (!readOnly && !submitted && form.data?.available) saveDraft.mutate();
    },
    submit: () => {
      if (!readOnly && !submitted && form.data?.available && validateBeforeSubmit()) submitForm.mutate();
    },
    beginAmendment,
    saveAmendment,
    cancelAmendment,
  }), [activeFields, amendForm.isPending, amendmentHasChanges, amendmentReason, amending, canAmend, currentRevision, form.data?.answers, form.data?.available, readOnly, submitted, values]);

  useEffect(() => {
    if (externalActions) onActionStateChange?.(actionState);
  }, [actionState, externalActions, onActionStateChange]);

  useEffect(() => () => {
    if (externalActions) onActionStateChange?.(null);
  }, [externalActions, onActionStateChange, workItemId]);

  if (form.isLoading) {
    return (
      <section ref={panelRef} tabIndex={-1} className="scroll-mt-24 rounded-md outline-none">
        <h3 className="mb-2 text-sm font-semibold text-ink">Hồ sơ nghiệp vụ</h3>
        <div className="rounded-md border border-line bg-white p-4">
          <LoadingSpinner />
        </div>
      </section>
    );
  }

  if (form.isError) {
    return (
      <section ref={panelRef} tabIndex={-1} className="scroll-mt-24 rounded-md outline-none">
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
      <section ref={panelRef} tabIndex={-1} className="scroll-mt-24 rounded-md outline-none">
        <h3 className="mb-2 text-sm font-semibold text-ink">Hồ sơ nghiệp vụ</h3>
        <EmptyState title="Bước này chưa có hồ sơ nghiệp vụ." />
      </section>
    );
  }

  const formActionPending = saveDraft.isPending || submitForm.isPending || amendForm.isPending;

  return (
    <section ref={panelRef} tabIndex={-1} className="scroll-mt-24 space-y-3 rounded-md outline-none">
      <div ref={actionCenterRef} tabIndex={-1} className="scroll-mt-24 rounded-md outline-none">
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
          readOnly={readOnly}
          showActions={!externalActions}
          onSave={() => { if (!readOnly) saveDraft.mutate(); }}
          onSubmit={() => {
            if (!readOnly && validateBeforeSubmit()) submitForm.mutate();
          }}
          submitLabel={submitLabel}
        />
      </div>

      {amending ? (
        <div className="rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-sm text-amber-900">
          <p className="font-semibold">Đang chỉnh sửa · dự kiến phiên bản {currentRevision + 1}</p>
          <p className="mt-1 text-xs leading-5">Lý do: {amendmentReason}. Chữ ký, con dấu và PDF tạm khóa cho đến khi lưu hoặc huỷ chỉnh sửa.</p>
        </div>
      ) : null}

      <div className="space-y-3">
        {!externalActions ? <WorkflowFormOperationsSummary
          submitted={submitted}
          validationIssueCount={validationIssues.length}
          evidenceCount={activeEvidence.length}
          checklistStats={checklistStats}
          signatures={signatures}
          sealApplications={sealApplications}
          pdfExports={pdfExports}
        /> : null}
        {isPlanningDailyReport ? <PlanningDailyReportSummary values={values} submitted={submitted} /> : null}
        {isMeetingMinutes ? <MeetingMinutesSummary values={values} submitted={submitted} /> : null}
        {isPlanningChecklist ? <PlanningChecklistSummary values={values} submitted={submitted} /> : null}

        <ol
          aria-label="Trình tự xử lý hồ sơ"
          data-workflow-process="two-step"
          className="overflow-hidden rounded-lg border border-line bg-white shadow-sm"
        >
          <li
            data-workflow-step="content"
            aria-current={!submitted || amending ? 'step' : undefined}
            className="grid grid-cols-[2.25rem_minmax(0,1fr)] sm:grid-cols-[2.75rem_minmax(0,1fr)]"
          >
            <div data-workflow-step-rail aria-hidden="true" className="relative flex justify-center bg-slate-50/80 pt-3">
              <span className={clsx(
                'relative z-10 grid h-6 w-6 shrink-0 place-items-center rounded-full text-xs font-bold sm:h-7 sm:w-7',
                submitted && !amending
                  ? 'bg-emerald-100 text-emerald-700 ring-1 ring-inset ring-emerald-200'
                  : amending
                    ? 'bg-amber-500 text-white shadow-sm'
                  : 'bg-blue-600 text-white shadow-sm',
              )} aria-hidden="true">1</span>
              <span
                data-workflow-process-line="content-to-documents"
                className="absolute bottom-0 left-1/2 top-9 w-px -translate-x-1/2 bg-slate-300 sm:top-10"
              />
            </div>
            <section ref={fieldsSectionRef} tabIndex={-1} className="min-w-0 scroll-mt-24 border-l border-line bg-white outline-none">
              <button
                type="button"
                className={clsx(
                  'flex w-full flex-col gap-2 px-3 py-3 text-left transition-colors sm:flex-row sm:items-center sm:justify-between',
                  amending
                    ? 'bg-amber-50/60 hover:bg-amber-50'
                    : submitted
                    ? 'bg-emerald-50/40 hover:bg-emerald-50/70'
                    : 'bg-blue-50/60 hover:bg-blue-50',
                )}
                aria-expanded={fieldsOpen}
                onClick={() => setFieldsOpen((current) => !current)}
              >
                <div className="min-w-0">
                  <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">Nội dung cần xử lý</p>
                  <h4 className="mt-0.5 text-sm font-semibold text-ink">Thông tin nghiệp vụ</h4>
                </div>
                <span className="flex w-full flex-wrap items-center gap-2 sm:w-auto sm:flex-nowrap sm:justify-end">
                  {amending ? <span className="rounded-full bg-amber-100 px-2 py-1 text-xs font-semibold text-amber-800 ring-1 ring-inset ring-amber-200">Đang chỉnh sửa · v{currentRevision + 1}</span> : null}
                  {submitted && !amending ? <span className="rounded-full bg-emerald-50 px-2 py-1 text-xs font-semibold text-emerald-700 ring-1 ring-inset ring-emerald-200">Đã chốt</span> : null}
                  {!submitted && validationIssues.length ? <span className="rounded-full bg-amber-50 px-2 py-1 text-xs font-semibold text-amber-700 ring-1 ring-inset ring-amber-200">{validationIssues.length} cần xử lý</span> : null}
                  {!submitted && !validationIssues.length ? <span className="rounded-full bg-blue-50 px-2 py-1 text-xs font-semibold text-blue-700 ring-1 ring-inset ring-blue-200">Sẵn sàng chốt</span> : null}
                  <span className="rounded-full bg-white px-2 py-1 text-xs font-semibold text-slate-600 ring-1 ring-inset ring-line">{coreFields.length} chính{optionalFields.length ? ` · ${optionalFields.length} bổ sung` : ''}</span>
                  {fieldsOpen ? <ChevronDown className="ml-auto h-4 w-4 text-slate-500 sm:ml-0" /> : <ChevronRight className="ml-auto h-4 w-4 text-slate-500 sm:ml-0" />}
                </span>
              </button>
              {fieldsOpen ? <div className="grid grid-cols-1 gap-3 border-t border-line p-3">
                {coreFields.map((field) => (
                  <WorkflowFieldControl
                    key={field.field_key}
                    templateCode={templateCode}
                    field={field}
                    value={values[field.field_key]}
                    formValues={values}
                    evidence={evidence}
                    error={errors[field.field_key]}
                    disabled={readOnly || (submitted && !amending) || formActionPending}
                    displayOnly={readOnly || (submitted && !amending)}
                    onChange={(value) => setFieldValue(field, value)}
                  />
                ))}
                {optionalFields.length ? (
                  <section className="min-w-0 overflow-hidden rounded-md border border-dashed border-slate-300 bg-slate-50/70">
                    <button
                      type="button"
                      className="flex w-full items-center justify-between gap-3 px-3 py-2 text-left text-sm font-semibold text-slate-700 hover:bg-slate-100"
                      aria-expanded={optionalFieldsOpen}
                      onClick={() => setOptionalFieldsOpen((current) => !current)}
                    >
                      <span>
                        Thông tin bổ sung
                        <span className="ml-2 text-xs font-normal text-slate-500">Không bắt buộc · {optionalFields.length} mục</span>
                      </span>
                      {optionalFieldsOpen ? <ChevronDown className="h-4 w-4" /> : <ChevronRight className="h-4 w-4" />}
                    </button>
                    {optionalFieldsOpen ? <div className="grid grid-cols-1 gap-3 border-t border-slate-200 bg-white p-3">
                      {optionalFields.map((field) => (
                        <WorkflowFieldControl
                          key={field.field_key}
                          templateCode={templateCode}
                          field={field}
                          value={values[field.field_key]}
                          formValues={values}
                          evidence={evidence}
                          error={errors[field.field_key]}
                          disabled={readOnly || (submitted && !amending) || formActionPending}
                          displayOnly={readOnly || (submitted && !amending)}
                          onChange={(value) => setFieldValue(field, value)}
                        />
                      ))}
                    </div> : null}
                  </section>
                ) : null}
              </div> : null}
              {fieldsOpen ? <div className="border-t border-line bg-slate-50 px-3 py-2 text-xs text-slate-500">
                {amending ? `Lý do chỉnh sửa: ${amendmentReason}` : instance?.submitted_at ? `Chốt lúc ${compactDateTime(instance.submitted_at)}` : `Cập nhật ${compactDateTime(instance?.updated_at)}`}
              </div> : null}
            </section>
          </li>

          <li
            data-workflow-step="documents"
            aria-current={submitted && !amending ? 'step' : undefined}
            className="grid grid-cols-[2.25rem_minmax(0,1fr)] sm:grid-cols-[2.75rem_minmax(0,1fr)]"
          >
            <div data-workflow-step-rail aria-hidden="true" className="relative flex justify-center bg-slate-50/80 pt-3">
              <span className="absolute left-1/2 top-0 h-3 w-px -translate-x-1/2 bg-slate-300" />
              <span className={clsx(
                'relative z-10 grid h-6 w-6 shrink-0 place-items-center rounded-full text-xs font-bold sm:h-7 sm:w-7',
                submitted && !amending
                  ? 'bg-blue-600 text-white shadow-sm'
                  : 'bg-slate-200 text-slate-500 ring-1 ring-inset ring-slate-300',
              )} aria-hidden="true">2</span>
            </div>
            <section className="min-w-0 border-l border-t border-line bg-white">
              <button
                type="button"
                className={clsx(
                  'flex w-full flex-col gap-2 px-3 py-3 text-left transition-colors sm:flex-row sm:items-center sm:justify-between',
                  submitted && !amending ? 'bg-blue-50/60 hover:bg-blue-50' : 'bg-slate-50 text-slate-500',
                  'disabled:cursor-not-allowed',
                )}
                aria-expanded={submitted && !amending && documentsOpen}
                disabled={!submitted || amending}
                onClick={() => { if (submitted && !amending) setDocumentsOpen((current) => !current); }}
              >
                <div className="min-w-0">
                  <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">Tài liệu nghiệp vụ</p>
                  <h4 className="mt-0.5 text-sm font-semibold text-ink">Xác nhận hồ sơ và PDF</h4>
                  {!submitted ? <p className="mt-1 text-xs text-slate-500">Hoàn tất bước 1 để ký, đóng dấu hoặc tạo PDF.</p> : null}
                  {amending ? <p className="mt-1 text-xs text-amber-700">Tạm khóa khi đang chỉnh sửa nội dung hồ sơ.</p> : null}
                </div>
                <span className="flex w-full shrink-0 items-center gap-2 sm:w-auto sm:justify-end">
                  {!submitted || amending ? <span className="inline-flex items-center gap-1 rounded-full bg-slate-200 px-2 py-1 text-xs font-semibold text-slate-600"><LockKeyhole className="h-3.5 w-3.5" /> {amending ? 'Tạm khóa' : 'Chưa mở'}</span> : null}
                  {submitted && !amending ? documentsOpen ? <ChevronDown className="ml-auto h-4 w-4 text-slate-500 sm:ml-0" /> : <ChevronRight className="ml-auto h-4 w-4 text-slate-500 sm:ml-0" /> : null}
                </span>
              </button>
              {documentsOpen ? <div className="space-y-3 border-t border-line p-3">
                <WorkflowConfirmationPanel
                  groups={confirmationGroups}
                  signatureCount={signatures.length}
                  sealCount={sealApplications.length}
                  submitted={submitted}
                  signedByCurrentUser={signedByCurrentUser}
                  saving={signForm.isPending || applySeal.isPending}
                  canUseSeals={canUseSeals}
                  readOnly={readOnly}
                  preferenceSavingPurpose={savePDFConfirmationPreference.isPending ? savePDFConfirmationPreference.variables?.purpose : undefined}
                  onPreferenceChange={(payload) => {
                    if (!payload.include_signatures && !payload.include_seals) {
                      toast.warn({ title: 'Cần ít nhất một loại xác nhận', description: 'Giữ lại chữ ký cá nhân, con dấu công ty hoặc cả hai trong PDF.' });
                      return;
                    }
                    if (!readOnly) savePDFConfirmationPreference.mutate(payload);
                  }}
                  onOpenConfirmation={() => { if (!readOnly) setConfirmationDialogOpen(true); }}
                  onViewSignatureDetail={(signature) => setSignatureDetail(signature)}
                  onOpenSignatureVisual={(signature) => void openWorkflowBlob(`/workflow-form-signatures/${signature.id}/visual`, 'Không mở được chữ ký')}
                  onViewSealDetail={(application) => setSealApplicationDetail(application)}
                  onOpenSealVisual={(application) => void openWorkflowBlob(`/workflow-form-seal-applications/${application.id}/visual`, 'Không mở được ảnh con dấu')}
                />
                <WorkflowPDFExportPanel
                  exports={pdfExports}
                  submitted={submitted}
                  generating={generatePDF.isPending}
                  preferenceSaving={savePDFConfirmationPreference.isPending}
                  selectedSignatureCount={pdfConfirmationCounts.signatureCount}
                  selectedSealCount={pdfConfirmationCounts.sealCount}
                  deletingId={deletePDF.isPending ? pdfDeleteCandidate?.id : undefined}
                  readOnly={readOnly}
                  onGenerate={() => { if (!readOnly) generatePDF.mutate(); }}
                  onOpen={(record) => void openWorkflowBlob(`/workflow-form-pdf-exports/${record.id}/file`, 'Không mở được PDF hồ sơ nghiệp vụ')}
                  onRequestDelete={(record) => { if (!readOnly && record.can_delete) setPDFDeleteCandidate(record); }}
                />
                <WorkflowHistoricalConfirmations
                  signatures={historicalSignatures}
                  sealApplications={historicalSealApplications}
                  onViewSignatureDetail={(signature) => setSignatureDetail(signature)}
                  onOpenSignatureVisual={(signature) => void openWorkflowBlob(`/workflow-form-signatures/${signature.id}/visual`, 'Không mở được chữ ký')}
                  onViewSealDetail={(application) => setSealApplicationDetail(application)}
                  onOpenSealVisual={(application) => void openWorkflowBlob(`/workflow-form-seal-applications/${application.id}/visual`, 'Không mở được ảnh con dấu')}
                />
              </div> : null}
            </section>
          </li>
        </ol>

      </div>
      <SignatureCaptureDialog
        open={!readOnly && confirmationDialogOpen}
        signerName={user?.full_name}
        submitting={signForm.isPending || applySeal.isPending}
        signedPurposes={signedPurposesByCurrentUser}
        formRevision={instance?.current_revision}
        allowedKinds={canUseSeals ? ['signature', 'seal'] : ['signature']}
        sealAssets={sealAssets.data ?? []}
        sealAssetsLoading={sealAssets.isLoading}
        appliedSealKeys={sealApplications.map((application) => `${application.purpose}:${application.seal_asset_id}`)}
        canManageSeals={canManageSeals}
        uploadingSeal={uploadSeal.isPending}
        onClose={() => setConfirmationDialogOpen(false)}
        onSubmit={(payload) => signForm.mutate(payload)}
        onApplySeal={(payload) => applySeal.mutate(payload)}
        onUploadSeal={(name, file) => uploadSeal.mutateAsync({ name, file })}
        onValidationWarning={(message) => toast.warn({ title: 'Chưa thể tải con dấu', description: message })}
      />
      <SignatureDetailDialog
        signature={signatureDetail}
        onClose={() => setSignatureDetail(null)}
        onOpenVisual={(signature) => void openWorkflowBlob(`/workflow-form-signatures/${signature.id}/visual`, 'Không mở được chữ ký')}
      />
      <SealApplicationDetailDialog
        application={sealApplicationDetail}
        onClose={() => setSealApplicationDetail(null)}
        onOpenVisual={(application) => void openWorkflowBlob(`/workflow-form-seal-applications/${application.id}/visual`, 'Không mở được ảnh con dấu')}
      />
      <Dialog
        open={Boolean(pdfDeleteCandidate)}
        title="Xoá tệp PDF?"
        description="Tệp sẽ bị xoá khỏi kho lưu trữ. Metadata export và lịch sử audit vẫn được giữ lại."
        onClose={() => { if (!deletePDF.isPending) setPDFDeleteCandidate(null); }}
        size="sm"
        footer={(
          <div className="flex justify-end gap-2">
            <button
              type="button"
              className="rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100 disabled:opacity-60"
              disabled={deletePDF.isPending}
              onClick={() => setPDFDeleteCandidate(null)}
            >
              Huỷ
            </button>
            <button
              type="button"
              className="inline-flex items-center gap-2 rounded-md bg-rose-600 px-3 py-2 text-sm font-semibold text-white hover:bg-rose-700 disabled:cursor-not-allowed disabled:opacity-60"
              disabled={deletePDF.isPending || !pdfDeleteCandidate}
              onClick={() => { if (pdfDeleteCandidate) deletePDF.mutate(pdfDeleteCandidate.id); }}
            >
              {deletePDF.isPending ? <Loader2 className="h-4 w-4 animate-spin" /> : <Trash2 className="h-4 w-4" />}
              Xoá PDF
            </button>
          </div>
        )}
      >
        <div className="rounded-md border border-rose-100 bg-rose-50 p-3 text-sm text-rose-800">
          <p className="font-semibold">{pdfDeleteCandidate?.file_name}</p>
          <p className="mt-1 text-xs leading-5">Không thể hoàn tác việc xoá nội dung tệp khỏi kho lưu trữ.</p>
        </div>
      </Dialog>
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
  readOnly,
  showActions,
  onSave,
  onSubmit,
  submitLabel,
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
  readOnly: boolean;
  showActions: boolean;
  onSave: () => void;
  onSubmit: () => void;
  submitLabel: string;
}) {
  const blocked = !readOnly && !submitted && validationIssueLabels.length > 0;
  const issuePreview = validationIssueLabels.slice(0, 4).join(', ');
  const extraIssues = validationIssueLabels.length > 4 ? ` +${validationIssueLabels.length - 4}` : '';
  return (
    <section className={clsx('rounded-md border p-3', showActions && blocked ? 'border-amber-200 bg-amber-50' : 'border-line bg-white')}>
      <div className="flex flex-col gap-3 xl:flex-row xl:items-start xl:justify-between">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <h3 className="text-sm font-semibold text-ink">{title}</h3>
            <StatusBadge status={status} label={workflowFormStatusLabel(status)} />
          </div>
          <p className="mt-1 text-xs text-slate-500">
            Hồ sơ nghiệp vụ v{versionNo} · {submittedAt ? `Chốt lúc ${compactDateTime(submittedAt)}` : `Cập nhật ${compactDateTime(updatedAt)}`}
          </p>
          {showActions ? <div className="mt-3 flex items-start gap-2">
            <span className={clsx('mt-0.5 rounded-md p-1.5', blocked ? 'bg-white text-amber-700' : 'bg-blue-50 text-blue-700')}>
              {blocked ? <AlertTriangle className="h-4 w-4" /> : <CheckCircle2 className="h-4 w-4" />}
            </span>
            <div className="min-w-0">
              <p className="text-sm font-semibold text-ink">
                {readOnly ? 'Hồ sơ xem trước của bước sắp tới' : blocked ? 'Cần hoàn tất thông tin trước khi chốt' : submitted ? 'Hồ sơ đã được chốt' : 'Sẵn sàng lưu nháp hoặc chốt hồ sơ'}
              </p>
              <p className="mt-1 text-xs leading-5 text-slate-600">
                {readOnly ? 'Chỉ có thể cập nhật khi workflow chuyển đến node này.' : blocked ? `Thiếu: ${issuePreview}${extraIssues}` : submitted ? 'Nội dung đã khóa và được lưu vào đầu việc; chưa gửi cho người duyệt. Xác nhận và PDF dùng phiên bản đã chốt.' : 'Không còn lỗi bắt buộc ở trạng thái hiện tại.'}
              </p>
            </div>
          </div> : null}
        </div>
        {showActions ? <div className="flex shrink-0 flex-wrap gap-2 xl:justify-end">
          <button
            type="button"
            className="inline-flex items-center gap-2 rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100 disabled:cursor-not-allowed disabled:opacity-60"
            disabled={readOnly || submitted || saving || submitting}
            onClick={onSave}
          >
            {saving ? <Loader2 className="h-4 w-4 animate-spin" /> : null}
            Lưu nháp
          </button>
          <button
            type="button"
            data-workflow-form-submit
            className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:cursor-not-allowed disabled:opacity-60"
            disabled={readOnly || submitted || saving || submitting}
            onClick={onSubmit}
          >
            {submitting ? <Loader2 className="h-4 w-4 animate-spin" /> : null}
            {submitLabel}
          </button>
        </div> : null}
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
  sealApplications,
  pdfExports,
}: {
  submitted: boolean;
  validationIssueCount: number;
  evidenceCount: number;
  checklistStats: ChecklistStats | null;
  signatures: WorkflowFormSignature[];
  sealApplications: WorkflowFormSealApplication[];
  pdfExports: WorkflowFormPDFExport[];
}) {
  const confirmationCount = signatures.length + sealApplications.length;
  const nextAction = workflowNextAction(submitted, validationIssueCount, confirmationCount, pdfExports.length);
  return (
    <section className="rounded-md border border-line bg-white p-3">
      <div className="flex flex-col gap-3 xl:flex-row xl:items-center xl:justify-between">
        <div className="min-w-0">
          <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">Tóm tắt xử lý</p>
          <h4 className="mt-0.5 text-sm font-semibold text-ink">Việc tiếp theo: {nextAction}</h4>
        </div>
        <div className="grid gap-2 text-xs font-semibold text-slate-600 sm:grid-cols-4 xl:w-[36rem]">
          {checklistStats ? <WorkflowStepPill label="Checklist" value={`${checklistStats.complete}/${checklistStats.total} hoàn tất`} active={!submitted && Boolean(checklistStats.needsAction)} /> : null}
          <WorkflowStepPill label="Minh chứng" value={`${evidenceCount} đang hiệu lực`} active={!submitted && evidenceCount === 0} />
          <WorkflowStepPill label="Xác nhận" value={`${signatures.length} ký · ${sealApplications.length} dấu`} active={submitted && confirmationCount === 0} />
          <WorkflowStepPill label="PDF" value={`${pdfExports.length} tệp`} active={submitted && confirmationCount > 0 && pdfExports.length === 0} />
        </div>
      </div>
    </section>
  );
}

export function WorkflowFormEventsPanel({ events }: { events: AnyRecord[] }) {
  return (
    <section className="space-y-3">
      <div className="rounded-md border border-line bg-slate-50 px-3 py-3">
        <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">Lịch sử hồ sơ</p>
        <h3 className="mt-0.5 text-sm font-semibold text-ink">{events.length} sự kiện hồ sơ</h3>
        <p className="mt-1 text-xs text-slate-500">Các lần tạo, lưu, chốt, xác nhận và xuất PDF của hồ sơ nghiệp vụ.</p>
      </div>
      <div className="divide-y divide-line overflow-hidden rounded-md border border-line bg-white">
        {events.slice(0, 5).map((event) => {
          const changes = workflowFormAmendmentChanges(event);
          return (
            <article key={String(event.id)} className="px-3 py-2.5">
              <p className="text-xs font-semibold text-ink">{workflowFormEventLabel(event.event_type)}</p>
              <p className="mt-1 text-xs text-slate-500">{event.actor_name ?? 'Hệ thống'} · {compactDateTime(event.created_at)}</p>
              {event.event_type === 'amended' ? (
                <div className="mt-2 rounded-md border border-amber-100 bg-amber-50 p-2 text-xs text-slate-700">
                  <p className="font-semibold text-amber-900">Phiên bản {String(event.metadata_json?.from_revision ?? '?')} → {String(event.metadata_json?.to_revision ?? '?')}</p>
                  <p className="mt-1"><span className="font-semibold">Lý do:</span> {String(event.metadata_json?.reason ?? event.comment ?? '-')}</p>
                  {changes.length ? (
                    <div className="mt-2 space-y-2">
                      {changes.map((change) => (
                        <div key={String(change.field_key)} className="rounded border border-amber-100 bg-white p-2">
                          <p className="font-semibold text-ink">{String(change.field_label || change.field_key)}</p>
                          <p className="mt-1 whitespace-pre-wrap break-words"><span className="font-semibold text-slate-500">Trước:</span> {workflowFormHistoryValue(change.before, change.before_present)}</p>
                          <p className="mt-1 whitespace-pre-wrap break-words"><span className="font-semibold text-slate-500">Sau:</span> {workflowFormHistoryValue(change.after, change.after_present)}</p>
                        </div>
                      ))}
                    </div>
                  ) : null}
                </div>
              ) : null}
            </article>
          );
        })}
      </div>
      {events.length > 5 ? <p className="text-xs text-slate-500">Đang hiển thị 5 sự kiện gần nhất.</p> : null}
    </section>
  );
}

function workflowFormAmendmentChanges(event: AnyRecord) {
  const changes = event.metadata_json?.changed_fields;
  return Array.isArray(changes) ? changes.filter((change): change is AnyRecord => Boolean(change && typeof change === 'object')) : [];
}

function workflowFormHistoryValue(value: unknown, present: unknown) {
  if (present === false || value === undefined || value === null || value === '') return '—';
  if (typeof value === 'string') return value;
  if (typeof value === 'boolean') return value ? 'Có' : 'Không';
  if (typeof value === 'number') return String(value);
  return JSON.stringify(value, null, 2);
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
            {submitted ? 'Nhật ký đã chốt; các hạng mục sẽ làm đã được dùng để sinh đầu việc kế hoạch tiếp theo.' : 'Hoàn tất phần Đã làm và Sẽ làm trước khi chốt nhật ký.'}
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
            {submitted ? 'Biên bản đã chốt; đầu việc được dùng để sinh đầu việc theo hồ sơ nghiệp vụ.' : 'Hoàn tất người tham dự, quyết định và đầu việc trước khi chốt biên bản.'}
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
            {submitted ? 'Checklist đã chốt; dữ liệu hoàn tất được lưu lại để kiểm tra bước.' : 'Chỉ được chốt khi mọi dòng ở trạng thái Hoàn tất hoặc Không áp dụng.'}
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

function workflowNextAction(submitted: boolean, validationIssueCount: number, confirmationCount: number, pdfCount: number) {
  if (!submitted && validationIssueCount > 0) return 'Hoàn tất checklist/hồ sơ';
  if (!submitted) return 'Chốt hồ sơ nghiệp vụ';
  if (confirmationCount === 0) return 'Thêm xác nhận hồ sơ';
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

function workflowFormAnswersEqual(left: AnyRecord, right: AnyRecord) {
  return JSON.stringify(left) === JSON.stringify(right);
}

const confirmationPurposeOrder = ['prepared_by', 'reviewed_by', 'approved_by', 'received_by', 'internal_ack'];

function buildConfirmationPurposeGroups(
  signatures: WorkflowFormSignature[],
  sealApplications: WorkflowFormSealApplication[],
  preferences: WorkflowFormPDFConfirmationPreference[],
): ConfirmationPurposeGroup[] {
  const purposes = new Set([
    ...signatures.map((record) => record.purpose),
    ...sealApplications.map((record) => record.purpose),
  ]);
  const preferenceByPurpose = new Map(preferences.map((preference) => [preference.purpose, preference]));
  return [...purposes]
    .sort((left, right) => {
      const leftIndex = confirmationPurposeOrder.indexOf(left);
      const rightIndex = confirmationPurposeOrder.indexOf(right);
      if (leftIndex === -1 && rightIndex === -1) return left.localeCompare(right);
      if (leftIndex === -1) return 1;
      if (rightIndex === -1) return -1;
      return leftIndex - rightIndex;
    })
    .map((purpose) => {
      const purposeSignatures = signatures.filter((record) => record.purpose === purpose);
      const purposeSeals = sealApplications.filter((record) => record.purpose === purpose);
      const preference = preferenceByPurpose.get(purpose);
      const hasBothKinds = purposeSignatures.length > 0 && purposeSeals.length > 0;
      return {
        purpose,
        signatures: purposeSignatures,
        sealApplications: purposeSeals,
        hasBothKinds,
        includeSignatures: !hasBothKinds || preference?.include_signatures !== false,
        includeSeals: !hasBothKinds || preference?.include_seals !== false,
      };
    });
}

function selectedPDFConfirmationCounts(groups: ConfirmationPurposeGroup[]) {
  return groups.reduce((counts, group) => ({
    signatureCount: counts.signatureCount + (group.includeSignatures ? group.signatures.length : 0),
    sealCount: counts.sealCount + (group.includeSeals ? group.sealApplications.length : 0),
  }), { signatureCount: 0, sealCount: 0 });
}

function optimisticPDFConfirmationPreference(
  current: WorkflowFormDetail | undefined,
  payload: PDFConfirmationPreferenceInput,
  actorID?: string,
  actorName?: string,
) {
  if (!current) return current;
  const preferences = current.pdf_confirmation_preferences ?? [];
  const existing = preferences.find((preference) => preference.purpose === payload.purpose);
  const nextPreference: WorkflowFormPDFConfirmationPreference = {
    ...(existing ?? {
      id: `optimistic:${payload.purpose}`,
      form_instance_id: current.instance?.id ?? '',
      purpose: payload.purpose,
    }),
    include_signatures: payload.include_signatures,
    include_seals: payload.include_seals,
    updated_by: actorID,
    updated_by_name: actorName,
    updated_at: new Date().toISOString(),
  };
  return {
    ...current,
    pdf_confirmation_preferences: [
      nextPreference,
      ...preferences.filter((preference) => preference.purpose !== payload.purpose),
    ],
  };
}

function WorkflowConfirmationPanel({
  groups,
  signatureCount,
  sealCount,
  submitted,
  signedByCurrentUser,
  saving,
  canUseSeals,
  readOnly,
  preferenceSavingPurpose,
  onPreferenceChange,
  onOpenConfirmation,
  onViewSignatureDetail,
  onOpenSignatureVisual,
  onViewSealDetail,
  onOpenSealVisual,
}: {
  groups: ConfirmationPurposeGroup[];
  signatureCount: number;
  sealCount: number;
  submitted: boolean;
  signedByCurrentUser: boolean;
  saving: boolean;
  canUseSeals: boolean;
  readOnly: boolean;
  preferenceSavingPurpose?: string;
  onPreferenceChange: (payload: PDFConfirmationPreferenceInput) => void;
  onOpenConfirmation: () => void;
  onViewSignatureDetail: (signature: WorkflowFormSignature) => void;
  onOpenSignatureVisual: (signature: WorkflowFormSignature) => void;
  onViewSealDetail: (application: WorkflowFormSealApplication) => void;
  onOpenSealVisual: (application: WorkflowFormSealApplication) => void;
}) {
  const disabled = readOnly || !submitted || saving;
  const anyPreferenceSaving = Boolean(preferenceSavingPurpose);
  return (
    <section className="rounded-md border border-line bg-slate-50 p-3">
      <div className="mb-2 flex flex-wrap items-center justify-between gap-2">
        <div>
          <h4 className="text-sm font-semibold text-ink">Xác nhận hồ sơ</h4>
          <p className="mt-1 text-xs text-slate-500">Xác nhận được nhóm theo mục đích. Khi một mục đích có đủ hai loại, chọn chữ ký, con dấu hoặc cả hai để đưa vào PDF.</p>
        </div>
        {signatureCount || sealCount ? <span className="text-xs font-semibold text-slate-500">{signatureCount} chữ ký · {sealCount} con dấu</span> : null}
      </div>
      {groups.length ? (
        <div className="space-y-3">
          {groups.map((group) => {
            const savingThisPreference = preferenceSavingPurpose === group.purpose;
            return (
              <section key={group.purpose} className="overflow-hidden rounded-md border border-line bg-white">
                <div className="flex flex-wrap items-center justify-between gap-2 border-b border-line bg-slate-100 px-3 py-2">
                  <div>
                    <p className="text-sm font-semibold text-ink">{signaturePurposeLabel(group.purpose)}</p>
                    <p className="mt-0.5 text-xs text-slate-500">{group.signatures.length} chữ ký · {group.sealApplications.length} con dấu</p>
                  </div>
                  {group.hasBothKinds ? (
                    <span className="inline-flex items-center gap-1 text-xs font-medium text-slate-500">
                      {savingThisPreference ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <CheckCircle2 className="h-3.5 w-3.5 text-emerald-600" />}
                      {savingThisPreference ? 'Đang lưu lựa chọn PDF…' : 'Áp dụng cho PDF mới'}
                    </span>
                  ) : (
                    <span className="text-xs font-medium text-slate-500">Tự động đưa loại hiện có vào PDF</span>
                  )}
                </div>
                <div className={clsx('grid gap-3 p-3', group.signatures.length && group.sealApplications.length && 'lg:grid-cols-2')}>
                  {group.signatures.length ? (
                    <div className={clsx('space-y-2 rounded-md border border-line bg-slate-50 p-2', !group.includeSignatures && 'opacity-60')}>
                      <div className="flex flex-wrap items-center justify-between gap-2">
                        <p className="flex items-center gap-2 text-xs font-semibold uppercase tracking-wide text-slate-600"><PenLine className="h-3.5 w-3.5" />Chữ ký cá nhân</p>
                        {group.hasBothKinds ? (
                          <label className="inline-flex cursor-pointer items-center gap-2 text-xs font-semibold text-slate-700">
                            <input
                              type="checkbox"
                              className="h-4 w-4 rounded border-slate-300 text-blue-600 focus:ring-blue-500"
                              checked={group.includeSignatures}
                              disabled={readOnly || anyPreferenceSaving}
                              onChange={() => onPreferenceChange({
                                purpose: group.purpose,
                                include_signatures: !group.includeSignatures,
                                include_seals: group.includeSeals,
                              })}
                            />
                            Hiển thị trong PDF
                          </label>
                        ) : null}
                      </div>
                      {group.signatures.map((signature) => (
                        <ConfirmationRecord
                          key={signature.id}
                          title={signature.signer_name}
                          statusLabel="Đã ký"
                          meta={`Phiên bản ${signature.form_revision_no} · ${compactDateTime(signature.signed_at)}`}
                          audit={`Vai trò: ${signatureRolesText(signature)} · mã nội dung ${shortHash(signature.payload_hash)}`}
                          checksum={signature.visual_checksum_sha256 ? `chữ ký sha256:${signature.visual_checksum_sha256.slice(0, 12)}` : undefined}
                          comment={signature.comment}
                          visualLabel="Mở chữ ký"
                          onViewDetail={() => onViewSignatureDetail(signature)}
                          onOpenVisual={signature.visual_storage_key ? () => onOpenSignatureVisual(signature) : undefined}
                        />
                      ))}
                    </div>
                  ) : null}
                  {group.sealApplications.length ? (
                    <div className={clsx('space-y-2 rounded-md border border-line bg-slate-50 p-2', !group.includeSeals && 'opacity-60')}>
                      <div className="flex flex-wrap items-center justify-between gap-2">
                        <p className="flex items-center gap-2 text-xs font-semibold uppercase tracking-wide text-slate-600"><Stamp className="h-3.5 w-3.5" />Con dấu công ty</p>
                        {group.hasBothKinds ? (
                          <label className="inline-flex cursor-pointer items-center gap-2 text-xs font-semibold text-slate-700">
                            <input
                              type="checkbox"
                              className="h-4 w-4 rounded border-slate-300 text-blue-600 focus:ring-blue-500"
                              checked={group.includeSeals}
                              disabled={readOnly || anyPreferenceSaving}
                              onChange={() => onPreferenceChange({
                                purpose: group.purpose,
                                include_signatures: group.includeSignatures,
                                include_seals: !group.includeSeals,
                              })}
                            />
                            Hiển thị trong PDF
                          </label>
                        ) : null}
                      </div>
                      {group.sealApplications.map((application) => (
                        <ConfirmationRecord
                          key={application.id}
                          title={application.seal_asset_name}
                          statusLabel="Đã đóng dấu"
                          meta={`Phiên bản ${application.form_revision_no} · ${compactDateTime(application.applied_at)}`}
                          audit={`Đóng dấu bởi: ${application.applied_by_name} · mã nội dung ${shortHash(application.payload_hash)}`}
                          checksum={`ảnh dấu sha256:${application.checksum_sha256.slice(0, 12)}`}
                          comment={application.comment}
                          visualLabel="Mở con dấu"
                          onViewDetail={() => onViewSealDetail(application)}
                          onOpenVisual={() => onOpenSealVisual(application)}
                        />
                      ))}
                    </div>
                  ) : null}
                </div>
              </section>
            );
          })}
        </div>
      ) : <ConfirmationEmpty text={submitted ? 'Chưa có chữ ký cá nhân hoặc con dấu công ty.' : 'Chốt hồ sơ trước khi thêm xác nhận.'} />}
      <div className="mt-3 flex flex-wrap items-center justify-between gap-2">
        <div className="text-xs text-slate-500">
          {!submitted
            ? 'Chốt hồ sơ trước khi thêm xác nhận.'
            : signedByCurrentUser
              ? `Bạn đã ký xác nhận nội bộ.${canUseSeals ? ' Có thể chọn con dấu được cấp quyền.' : ''}`
              : canUseSeals ? 'Chọn chữ ký cá nhân hoặc con dấu công ty.' : 'Chữ ký cá nhân được lưu như tệp được bảo vệ.'}
        </div>
        <button
          type="button"
          className="inline-flex items-center justify-center gap-2 rounded-md bg-emerald-600 px-3 py-2 text-sm font-semibold text-white hover:bg-emerald-700 disabled:cursor-not-allowed disabled:opacity-60"
          disabled={disabled}
          onClick={onOpenConfirmation}
        >
          {saving ? <Loader2 className="h-4 w-4 animate-spin" /> : <Plus className="h-4 w-4" />}
          Thêm xác nhận
        </button>
      </div>
    </section>
  );
}

function ConfirmationRecord({ title, statusLabel, meta, audit, checksum, comment, visualLabel, onViewDetail, onOpenVisual }: {
  title: string;
  statusLabel: string;
  meta: string;
  audit: string;
  checksum?: string;
  comment?: string;
  visualLabel: string;
  onViewDetail: () => void;
  onOpenVisual?: () => void;
}) {
  return (
    <div className="rounded-md border border-line bg-white px-3 py-2">
      <div className="flex flex-wrap items-start justify-between gap-2">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <p className="truncate text-sm font-semibold text-ink">{title}</p>
            <span className="rounded-md bg-emerald-50 px-2 py-0.5 text-xs font-semibold text-emerald-700 ring-1 ring-inset ring-emerald-200">{statusLabel}</span>
          </div>
          <p className="mt-1 text-xs text-slate-500">{meta}</p>
          <p className="mt-1 text-xs text-slate-500">{audit}</p>
          {checksum ? <p className="mt-1 text-xs text-slate-500">{checksum}</p> : null}
        </div>
        <div className="flex flex-wrap items-center gap-1.5">
          <button type="button" className="inline-flex items-center gap-1 rounded-md border border-line bg-white px-2 py-1 text-xs font-semibold text-slate-700 hover:bg-slate-100" onClick={onViewDetail}>
            <FileText className="h-3.5 w-3.5" />Chi tiết
          </button>
          {onOpenVisual ? (
            <button type="button" className="inline-flex items-center gap-1 rounded-md border border-line bg-white px-2 py-1 text-xs font-semibold text-blue-700 hover:border-blue-300 hover:bg-blue-50" onClick={onOpenVisual}>
              <ExternalLink className="h-3.5 w-3.5" />{visualLabel}
            </button>
          ) : null}
          <span className="inline-flex items-center gap-1 rounded-md bg-emerald-50 px-2 py-1 text-xs font-semibold text-emerald-700 ring-1 ring-inset ring-emerald-200">
            <CheckCircle2 className="h-3.5 w-3.5" />Bất biến
          </span>
        </div>
      </div>
      {comment ? <p className="mt-2 text-sm text-slate-700">{comment}</p> : null}
    </div>
  );
}

function ConfirmationEmpty({ text }: { text: string }) {
  return <div className="rounded-md border border-dashed border-line bg-white px-3 py-4 text-sm font-medium text-slate-500">{text}</div>;
}

function WorkflowHistoricalConfirmations({
  signatures,
  sealApplications,
  onViewSignatureDetail,
  onOpenSignatureVisual,
  onViewSealDetail,
  onOpenSealVisual,
}: {
  signatures: WorkflowFormSignature[];
  sealApplications: WorkflowFormSealApplication[];
  onViewSignatureDetail: (signature: WorkflowFormSignature) => void;
  onOpenSignatureVisual: (signature: WorkflowFormSignature) => void;
  onViewSealDetail: (application: WorkflowFormSealApplication) => void;
  onOpenSealVisual: (application: WorkflowFormSealApplication) => void;
}) {
  if (!signatures.length && !sealApplications.length) return null;
  return (
    <details className="group rounded-md border border-line bg-slate-50">
      <summary className="flex cursor-pointer list-none items-center justify-between gap-3 px-3 py-2.5 text-sm font-semibold text-slate-700">
        <span>Xác nhận phiên bản trước · {signatures.length} chữ ký · {sealApplications.length} con dấu</span>
        <ChevronDown className="h-4 w-4 transition group-open:rotate-180" />
      </summary>
      <div className="space-y-2 border-t border-line bg-white p-3">
        {signatures.map((signature) => (
          <div key={signature.id} className="flex flex-col gap-2 rounded-md border border-line bg-slate-50 p-2 sm:flex-row sm:items-center sm:justify-between">
            <div className="min-w-0 text-xs text-slate-600">
              <p className="font-semibold text-ink">Chữ ký · {signaturePurposeLabel(signature.purpose)}</p>
              <p className="mt-1">Phiên bản {signature.form_revision_no} · {signature.signer_name} · {compactDateTime(signature.signed_at)}</p>
            </div>
            <div className="flex shrink-0 gap-2">
              <button type="button" className="rounded-md border border-line bg-white px-2 py-1 text-xs font-semibold text-slate-700 hover:bg-slate-100" onClick={() => onViewSignatureDetail(signature)}>Chi tiết</button>
              {signature.visual_storage_key ? <button type="button" className="rounded-md border border-line bg-white px-2 py-1 text-xs font-semibold text-blue-700 hover:bg-blue-50" onClick={() => onOpenSignatureVisual(signature)}>Mở chữ ký</button> : null}
            </div>
          </div>
        ))}
        {sealApplications.map((application) => (
          <div key={application.id} className="flex flex-col gap-2 rounded-md border border-line bg-slate-50 p-2 sm:flex-row sm:items-center sm:justify-between">
            <div className="min-w-0 text-xs text-slate-600">
              <p className="font-semibold text-ink">Con dấu · {signaturePurposeLabel(application.purpose)}</p>
              <p className="mt-1">Phiên bản {application.form_revision_no} · {application.seal_asset_name} · {compactDateTime(application.applied_at)}</p>
            </div>
            <div className="flex shrink-0 gap-2">
              <button type="button" className="rounded-md border border-line bg-white px-2 py-1 text-xs font-semibold text-slate-700 hover:bg-slate-100" onClick={() => onViewSealDetail(application)}>Chi tiết</button>
              <button type="button" className="rounded-md border border-line bg-white px-2 py-1 text-xs font-semibold text-blue-700 hover:bg-blue-50" onClick={() => onOpenSealVisual(application)}>Mở con dấu</button>
            </div>
          </div>
        ))}
      </div>
    </details>
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

function SealApplicationDetailDialog({
  application,
  onClose,
  onOpenVisual,
}: {
  application: WorkflowFormSealApplication | null;
  onClose: () => void;
  onOpenVisual: (application: WorkflowFormSealApplication) => void;
}) {
  if (!application) return null;
  return (
    <Dialog
      open={Boolean(application)}
      title="Chi tiết lần đóng dấu"
      description="Con dấu là xác nhận tổ chức riêng; không thay thế chữ ký cá nhân hoặc chữ ký hợp đồng trên App."
      size="lg"
      onClose={onClose}
      footer={(
        <div className="flex flex-wrap items-center justify-between gap-2">
          <p className="text-xs text-slate-500">Ảnh dấu và bản ghi áp dụng được bảo vệ để phục vụ đối chiếu.</p>
          <div className="flex gap-2">
            <button type="button" className="inline-flex items-center gap-2 rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-blue-700 hover:bg-blue-50" onClick={() => onOpenVisual(application)}>
              <ExternalLink className="h-4 w-4" />Mở con dấu
            </button>
            <button type="button" className="rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100" onClick={onClose}>Đóng</button>
          </div>
        </div>
      )}
    >
      <div className="space-y-4">
        <div className="rounded-md border border-emerald-200 bg-emerald-50 p-3">
          <p className="text-sm font-semibold text-emerald-900">Bản ghi đóng dấu không thể chỉnh sửa</p>
          <p className="mt-1 text-xs leading-5 text-emerald-800">Việc lưu trữ hoặc ngừng sử dụng tài nguyên con dấu không làm thay đổi lần đóng dấu đã ghi nhận.</p>
        </div>
        <div className="grid gap-3 sm:grid-cols-2">
          <SignatureDetailField label="Con dấu" value={application.seal_asset_name || '-'} />
          <SignatureDetailField label="Trạng thái tài nguyên" value={application.seal_asset_status === 'archived' ? 'Ngừng sử dụng' : 'Đang hoạt động'} />
          <SignatureDetailField label="Người đóng dấu" value={application.applied_by_name || '-'} />
          <SignatureDetailField label="ID người thao tác" value={application.applied_by || '-'} mono />
          <SignatureDetailField label="Mục đích" value={signaturePurposeLabel(application.purpose)} />
          <SignatureDetailField label="Thời điểm" value={compactDateTime(application.applied_at)} />
          <SignatureDetailField label="Phiên bản hồ sơ" value={`Phiên bản ${application.form_revision_no}`} />
          <SignatureDetailField label="Vai trò lúc đóng dấu" value={confirmationRolesText(application.applied_by_roles_json)} />
          <SignatureDetailField label="Tệp ảnh" value={`${application.mime_type} · ${fileSizeText(application.file_size)} · ${application.width_px}×${application.height_px}px`} />
          <SignatureDetailField label="Nơi lưu" value={application.storage_provider || '-'} />
        </div>
        <SignatureHashBlock label="Mã nội dung" value={application.payload_hash} />
        <SignatureHashBlock label="Checksum ảnh dấu sha256" value={application.checksum_sha256} />
        {application.comment ? (
          <div className="rounded-md border border-line bg-slate-50 p-3">
            <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">Ghi chú</p>
            <p className="mt-1 whitespace-pre-wrap break-words text-sm leading-6 text-slate-700">{application.comment}</p>
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
  preferenceSaving,
  selectedSignatureCount,
  selectedSealCount,
  deletingId,
  readOnly,
  onGenerate,
  onOpen,
  onRequestDelete,
}: {
  exports: WorkflowFormPDFExport[];
  submitted: boolean;
  generating: boolean;
  preferenceSaving: boolean;
  selectedSignatureCount: number;
  selectedSealCount: number;
  deletingId?: string;
  readOnly: boolean;
  onGenerate: () => void;
  onOpen: (record: WorkflowFormPDFExport) => void;
  onRequestDelete: (record: WorkflowFormPDFExport) => void;
}) {
  return (
    <section className="rounded-md border border-line bg-slate-50 p-3">
      <div className="mb-2 flex flex-wrap items-center justify-between gap-2">
        <div>
          <h4 className="text-sm font-semibold text-ink">Tệp PDF</h4>
          <p className="mt-1 text-xs text-slate-500">PDF mới sẽ gồm {selectedSignatureCount} chữ ký · {selectedSealCount} con dấu theo lựa chọn phía trên.</p>
        </div>
        <button
          type="button"
          className="inline-flex items-center gap-2 rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100 disabled:cursor-not-allowed disabled:opacity-60"
          disabled={readOnly || !submitted || generating || preferenceSaving}
          onClick={onGenerate}
        >
          {generating || preferenceSaving ? <Loader2 className="h-4 w-4 animate-spin" /> : <FileText className="h-4 w-4" />}
          {preferenceSaving ? 'Đang lưu lựa chọn…' : 'Tạo PDF'}
        </button>
      </div>
      {exports.length ? (
        <div className="space-y-2">
          {exports.map((record) => (
            <div key={record.id} className="rounded-md border border-line bg-slate-50 px-3 py-2">
              <div className="flex flex-wrap items-start justify-between gap-2">
                <div className="min-w-0">
                  <div className="flex flex-wrap items-center gap-2">
                    <p className="truncate text-sm font-semibold text-ink">{record.file_name}</p>
                    {record.is_current_revision === false ? <span className="rounded-full bg-slate-200 px-2 py-0.5 text-[11px] font-semibold text-slate-600">Phiên bản trước</span> : null}
                  </div>
                  <p className="mt-1 text-xs text-slate-500">
                    Phiên bản {record.form_revision_no} · {compactDateTime(record.generated_at)} · sha256:{record.checksum_sha256.slice(0, 12)}
                  </p>
                  {workflowPDFExportConfirmationSummary(record) ? (
                    <p className="mt-1 text-xs font-medium text-slate-600">{workflowPDFExportConfirmationSummary(record)}</p>
                  ) : null}
                </div>
                <div className="flex flex-wrap items-center justify-end gap-1.5">
                  <button
                    type="button"
                    className="inline-flex items-center gap-1 rounded-md border border-line bg-white px-2 py-1 text-xs font-semibold text-blue-700 hover:border-blue-300 hover:bg-blue-50 disabled:cursor-not-allowed disabled:opacity-50"
                    disabled={record.deletion_status === 'pending'}
                    title={record.deletion_status === 'pending' ? 'PDF đang chờ hoàn tất xoá; hãy thử xoá lại' : 'Mở tệp PDF'}
                    onClick={() => onOpen(record)}
                  >
                    <Download className="h-3.5 w-3.5" />
                    Mở
                  </button>
                  <button
                    type="button"
                    className="inline-flex items-center gap-1 rounded-md border border-rose-200 bg-white px-2 py-1 text-xs font-semibold text-rose-700 hover:bg-rose-50 disabled:cursor-not-allowed disabled:opacity-50"
                    disabled={readOnly || deletingId === record.id || !record.can_delete}
                    title={record.can_delete ? 'Xoá tệp PDF' : 'PDF đã được liên kết với hồ sơ nghiệm thu đầu ra'}
                    onClick={() => onRequestDelete(record)}
                  >
                    {deletingId === record.id ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Trash2 className="h-3.5 w-3.5" />}
                    {record.deletion_status === 'failed' || record.deletion_status === 'pending' ? 'Thử xoá lại' : 'Xoá'}
                  </button>
                </div>
              </div>
            </div>
          ))}
        </div>
      ) : (
        <div className="rounded-md border border-dashed border-line px-3 py-4 text-sm font-medium text-slate-500">
          {submitted ? 'Chưa tạo PDF cho hồ sơ này.' : 'Chốt hồ sơ trước khi tạo PDF.'}
        </div>
      )}
    </section>
  );
}

function shortHash(hash?: string) {
  return hash ? hash.slice(0, 8) : '-';
}

function workflowPDFExportConfirmationSummary(record: WorkflowFormPDFExport) {
  const selection = record.metadata_json?.confirmation_selection;
  if (!selection) return '';
  const signatureCount = Number(selection.signature_count ?? 0);
  const sealCount = Number(selection.seal_application_count ?? 0);
  if (!Number.isFinite(signatureCount) || !Number.isFinite(sealCount)) return '';
  return `Đã xuất: ${signatureCount} chữ ký · ${sealCount} con dấu`;
}

function signatureRolesText(signature: WorkflowFormSignature) {
  return confirmationRolesText(signature.signer_roles_json);
}

function confirmationRolesText(roles: WorkflowFormSignature['signer_roles_json'] | WorkflowFormSealApplication['applied_by_roles_json']) {
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
  templateCode,
  field,
  value,
  formValues,
  evidence,
  error,
  disabled,
  displayOnly,
  onChange,
}: {
  templateCode?: string;
  field: WorkflowFormField;
  value: unknown;
  formValues: AnyRecord;
  evidence: EvidenceRecord[];
  error?: string;
  disabled: boolean;
  displayOnly: boolean;
  onChange: (value: unknown) => void;
}) {
  const required = field.required || workflowTopLevelOtherDetailRequired(templateCode, field.field_key, formValues);
  const controlClassName = clsx(
    'mt-1 w-full rounded-md border px-3 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-100 disabled:bg-slate-100',
    error ? 'border-rose-300 bg-rose-50' : 'border-line bg-white',
  );
  const fieldContent = renderFieldControl(templateCode, field, value, evidence, disabled, displayOnly, Boolean(error), controlClassName, onChange);
  return (
    <div
      data-workflow-field-key={field.field_key}
      tabIndex={-1}
      className="min-w-0 scroll-mt-24 rounded-md outline-none"
    >
      <span className="text-sm font-semibold text-slate-700">{field.label}</span>
      {required ? <span className="ml-1 text-rose-600">*</span> : null}
      {fieldContent}
      {field.help_text ? <p className="mt-1 text-xs text-slate-500">{field.help_text}</p> : null}
      {error ? <p className="mt-1 text-xs font-medium text-rose-700">{error}</p> : null}
    </div>
  );
}

function renderFieldControl(
  templateCode: string | undefined,
  field: WorkflowFormField,
  value: unknown,
  evidence: EvidenceRecord[],
  disabled: boolean,
  displayOnly: boolean,
  invalid: boolean,
  controlClassName: string,
  onChange: (value: unknown) => void,
) {
  if (field.field_type === 'textarea') {
    return <textarea className={controlClassName} rows={4} value={textValue(value)} disabled={disabled} onChange={(event) => onChange(event.target.value)} />;
  }
  if (field.field_type === 'select') {
    const otherTextConfig = workflowSelectOtherTextConfig(templateCode, field);
    const supportsOtherText = workflowSelectSupportsOtherText(templateCode, field);
    const selectValue = supportsOtherText ? workflowSelectOptionValue(templateCode, field, value) : textValue(value);
    const showOtherText = supportsOtherText && selectValue === 'other';
    return (
      <>
        <select
          className={controlClassName}
          value={selectValue}
          disabled={disabled}
          aria-invalid={invalid || undefined}
          onChange={(event) => onChange(supportsOtherText
            ? workflowSelectOptionChangeValue(templateCode, field, value, event.target.value)
            : event.target.value)}
        >
          <option value="">Chọn</option>
          {fieldOptions(field).map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
        </select>
        {showOtherText ? (
          <label className="mt-2 block min-w-0">
            <span className="text-xs font-semibold text-slate-600">
              {otherTextConfig?.inputLabel}<span className="ml-1 text-rose-600">*</span>
            </span>
            <input
              type="text"
              className={controlClassName}
              value={workflowSelectOtherTextValue(templateCode, field, value)}
              disabled={disabled}
              maxLength={otherTextConfig?.maxLength}
              placeholder={otherTextConfig?.placeholder}
              aria-invalid={invalid || undefined}
              onChange={(event) => onChange(workflowSelectOtherTextChangeValue(templateCode, field, event.target.value))}
            />
          </label>
        ) : null}
      </>
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
      return <ChecklistStructuredListControl templateCode={templateCode} field={field} value={value} evidence={evidence} disabled={disabled} displayOnly={displayOnly} invalid={invalid} onChange={onChange} />;
    }
    return <StructuredListControl templateCode={templateCode} field={field} value={value} evidence={evidence} disabled={disabled} displayOnly={displayOnly} invalid={invalid} onChange={onChange} />;
  }
  if (field.field_type === 'currency') {
    return (
      <CurrencyInput
        className="mt-1"
        inputClassName="text-sm"
        value={textValue(value)}
        currencyCode={String(field.validation_json?.currency_code ?? 'VND')}
        allowNegative={Boolean(field.validation_json?.allow_negative)}
        maximumFractionDigits={field.validation_json?.maximum_fraction_digits === undefined ? undefined : Number(field.validation_json.maximum_fraction_digits)}
        min={typeof field.validation_json?.min === 'number' ? field.validation_json.min : undefined}
        max={typeof field.validation_json?.max === 'number' ? field.validation_json.max : undefined}
        invalid={invalid}
        disabled={disabled}
        onValueChange={({ numericString }) => onChange(numericString)}
      />
    );
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

function workflowFormApiFieldErrors(error: unknown): FormErrors {
  if (!(error instanceof ApiError) || !error.details || typeof error.details !== 'object' || Array.isArray(error.details)) return {};
  const rawFields = (error.details as AnyRecord).fields;
  if (!rawFields || typeof rawFields !== 'object' || Array.isArray(rawFields)) return {};
  return Object.fromEntries(
    Object.entries(rawFields as AnyRecord)
      .map(([key, value]) => [key, String(value ?? '').trim()] as const)
      .filter(([, value]) => Boolean(value)),
  );
}

export function workflowFormFieldIsOptional(field: WorkflowFormField) {
  return field.validation_json?.capture_mode === 'optional';
}

function valueEmpty(field: WorkflowFormField, value: unknown) {
  if (value === null || value === undefined) return true;
  if (field.field_type === 'checkbox') return value !== true;
  if (field.field_type === 'structured_list') return structuredRows(value).length === 0;
  if (Array.isArray(value)) return value.length === 0;
  return String(value).trim() === '';
}

function validateFieldValue(templateCode: string | undefined, field: WorkflowFormField, value: unknown, answers: AnyRecord) {
  const required = field.required || workflowTopLevelOtherDetailRequired(templateCode, field.field_key, answers);
  if (required && valueEmpty(field, value)) return `${field.label} là bắt buộc`;
  const otherTextIssue = workflowSelectOtherTextIssue(templateCode, field, value);
  if (otherTextIssue) return otherTextIssue;
  if (field.field_type !== 'structured_list' || valueEmpty(field, value)) return '';
  const fields = structuredItemFields(field);
  const rows = structuredRows(value);
  for (const [rowIndex, row] of rows.entries()) {
    for (const spec of fields) {
      const issue = structuredCellIssue(templateCode, field.field_key, spec, row, field.field_key === 'checklist_items');
      if (issue) return `${field.label} dòng ${rowIndex + 1}: ${issue}`;
    }
  }
  return '';
}

type StructuredFieldSpec = {
  key: string;
  label: string;
  type: 'text' | 'textarea' | 'number' | 'currency' | 'date' | 'select' | 'evidence_reference' | string;
  required?: boolean;
  options?: Array<{ value: string; label?: string }>;
  currencyCode?: string;
  allowNegative?: boolean;
  maximumFractionDigits?: number;
  minLength?: number;
  maxLength?: number;
  min?: number;
  max?: number;
};

function StructuredListControl({
  templateCode,
  field,
  value,
  evidence,
  disabled,
  displayOnly,
  invalid,
  onChange,
}: {
  templateCode?: string;
  field: WorkflowFormField;
  value: unknown;
  evidence: EvidenceRecord[];
  disabled: boolean;
  displayOnly: boolean;
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
    <StructuredListTable
      templateCode={templateCode}
      fieldKey={field.field_key}
      label={field.label}
      rows={rows}
      specs={specs}
      evidence={evidence}
      disabled={disabled}
      displayOnly={displayOnly}
      invalid={invalid}
      emptyLabel="Chưa có dòng nào"
      onAddRow={addRow}
      onRemoveRow={removeRow}
      onUpdateCell={updateCell}
    />
  );
}

function ChecklistStructuredListControl({
  templateCode,
  field,
  value,
  evidence,
  disabled,
  displayOnly,
  invalid,
  onChange,
}: {
  templateCode?: string;
  field: WorkflowFormField;
  value: unknown;
  evidence: EvidenceRecord[];
  disabled: boolean;
  displayOnly: boolean;
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
    <StructuredListTable
      templateCode={templateCode}
      fieldKey={field.field_key}
      label={field.label}
      rows={rows}
      specs={specs}
      evidence={evidence}
      disabled={disabled}
      displayOnly={displayOnly}
      invalid={invalid}
      checklist
      emptyLabel="Chưa có dòng checklist"
      summary={(
        <div className="flex flex-wrap gap-1.5 text-xs font-semibold">
          <ChecklistStat label="Dòng" value={stats.total} />
          <ChecklistStat label="Hoàn tất/Không áp dụng" value={stats.complete} />
          <ChecklistStat label="Cần xử lý" value={stats.needsAction} tone={stats.needsAction ? 'warn' : 'ok'} />
          <ChecklistStat label="Minh chứng" value={stats.withEvidence} />
        </div>
      )}
      onAddRow={addRow}
      onRemoveRow={removeRow}
      onUpdateCell={updateCell}
    />
  );
}

function StructuredListTable({
  templateCode,
  fieldKey,
  label,
  rows,
  specs,
  evidence,
  disabled,
  displayOnly,
  invalid,
  checklist = false,
  emptyLabel,
  summary,
  onAddRow,
  onRemoveRow,
  onUpdateCell,
}: {
  templateCode?: string;
  fieldKey: string;
  label: string;
  rows: AnyRecord[];
  specs: StructuredFieldSpec[];
  evidence: EvidenceRecord[];
  disabled: boolean;
  displayOnly: boolean;
  invalid: boolean;
  checklist?: boolean;
  emptyLabel: string;
  summary?: ReactNode;
  onAddRow: () => void;
  onRemoveRow: (rowIndex: number) => void;
  onUpdateCell: (rowIndex: number, key: string, value: string) => void;
}) {
  const containerRef = useRef<HTMLDivElement | null>(null);
  const scrollViewportRef = useRef<HTMLDivElement | null>(null);
  const focusRowRef = useRef<number | null>(null);
  const [hasHorizontalOverflow, setHasHorizontalOverflow] = useState(false);
  const [pendingRemoveRowIndex, setPendingRemoveRowIndex] = useState<number | null>(null);
  const minWidth = structuredTableMinWidth(specs, !displayOnly);
  const pendingRemoveRow = pendingRemoveRowIndex === null ? undefined : rows[pendingRemoveRowIndex];
  const pendingRemoveSummary = pendingRemoveRow ? structuredRowDeleteSummary(pendingRemoveRow, specs, evidence) : '';

  useEffect(() => {
    const rowIndex = focusRowRef.current;
    if (rowIndex === null) return undefined;
    const frame = window.requestAnimationFrame(() => {
      const candidates = Array.from(
        containerRef.current?.querySelectorAll<HTMLElement>(
          `[data-structured-row-index="${rowIndex}"] input:not(:disabled), [data-structured-row-index="${rowIndex}"] select:not(:disabled), [data-structured-row-index="${rowIndex}"] textarea:not(:disabled)`,
        ) ?? [],
      );
      const target = candidates.find((candidate) => candidate.getClientRects().length > 0) ?? candidates[0];
      target?.focus();
      focusRowRef.current = null;
    });
    return () => window.cancelAnimationFrame(frame);
  }, [rows.length]);

  useEffect(() => {
    const viewport = scrollViewportRef.current;
    if (!viewport) return undefined;
    const updateOverflow = () => {
      setHasHorizontalOverflow(viewport.scrollWidth > viewport.clientWidth + 1);
    };
    const frame = window.requestAnimationFrame(updateOverflow);
    const resizeObserver = typeof ResizeObserver === 'undefined' ? null : new ResizeObserver(updateOverflow);
    resizeObserver?.observe(viewport);
    const table = viewport.querySelector('table');
    if (table) resizeObserver?.observe(table);
    window.addEventListener('resize', updateOverflow);
    return () => {
      window.cancelAnimationFrame(frame);
      resizeObserver?.disconnect();
      window.removeEventListener('resize', updateOverflow);
    };
  }, [displayOnly, minWidth, rows.length]);

  const addAndFocusRow = () => {
    focusRowRef.current = rows.length;
    onAddRow();
  };
  const confirmRemoveRow = () => {
    if (pendingRemoveRowIndex === null) return;
    onRemoveRow(pendingRemoveRowIndex);
    setPendingRemoveRowIndex(null);
  };

  return (
    <div
      ref={containerRef}
      data-structured-list-table
      className={clsx('mt-1 min-w-0 w-full max-w-full rounded-md border p-2', invalid ? 'border-rose-300 bg-rose-50/40' : 'border-line bg-white')}
    >
      {summary ? <div className="mb-2">{summary}</div> : null}
      {rows.length ? (
        <>
          <div
            ref={scrollViewportRef}
            className="structured-list-scroll hidden min-w-0 w-full max-w-full overflow-x-auto rounded-md border border-line md:block"
            role="region"
            aria-label={`${label}: bảng có thể cuộn ngang`}
            tabIndex={hasHorizontalOverflow ? 0 : undefined}
          >
            <table className="w-full table-fixed border-collapse text-left text-sm" style={{ minWidth }}>
              <caption className="sr-only">{label}</caption>
              <colgroup>
                {!displayOnly ? <col style={{ width: 52 }} /> : null}
                <col style={{ width: 48 }} />
                {specs.map((spec) => <col key={spec.key} style={{ width: structuredColumnWidth(spec) }} />)}
              </colgroup>
              <thead className="bg-slate-100 text-xs font-semibold text-slate-600">
                <tr>
                  {!displayOnly ? <th scope="col" className="border-b border-line px-2 py-2"><span className="sr-only">Thao tác</span></th> : null}
                  <th scope="col" className="border-b border-line px-2 py-2 text-center">#</th>
                  {specs.map((spec) => (
                    <th key={spec.key} scope="col" className="border-b border-l border-line px-2 py-2 align-bottom">
                      {spec.label}{structuredSpecRequired(templateCode, fieldKey, rows, spec) ? <span className="ml-1 text-rose-600">*</span> : null}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody className="divide-y divide-line bg-white">
                {rows.map((row, rowIndex) => {
                  const rowInvalid = structuredRowInvalid(templateCode, fieldKey, specs, row, invalid, checklist);
                  return (
                    <tr key={rowIndex} data-structured-row-index={rowIndex} className={rowInvalid ? 'bg-rose-50/50' : undefined}>
                      {!displayOnly ? (
                        <td className="p-2 text-center align-top">
                          <StructuredDeleteButton disabled={disabled} rowIndex={rowIndex} onRemove={setPendingRemoveRowIndex} />
                        </td>
                      ) : null}
                      <th scope="row" className="px-2 py-3 text-center align-top text-xs font-semibold text-slate-500">{rowIndex + 1}</th>
                      {specs.map((spec) => (
                        <td key={spec.key} className="border-l border-line p-2 align-top">
                          <StructuredCell
                            required={spec.required || workflowStructuredOtherDetailRequired(templateCode, fieldKey, row, spec.key)}
                            spec={spec}
                            value={row[spec.key]}
                            evidence={evidence}
                            disabled={disabled}
                            displayOnly={displayOnly}
                            invalid={structuredCellInvalid(templateCode, fieldKey, spec, row, invalid, checklist)}
                            issue={invalid ? structuredCellIssue(templateCode, fieldKey, spec, row, checklist) : ''}
                            showLabel={false}
                            onChange={(nextValue) => onUpdateCell(rowIndex, spec.key, nextValue)}
                          />
                        </td>
                      ))}
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
          {hasHorizontalOverflow ? (
            <p className="mt-1 hidden items-center justify-between gap-3 px-1 text-[11px] font-medium text-slate-500 md:flex">
              <span aria-hidden="true">←</span>
              <span>Kéo ngang để xem các cột còn lại</span>
              <span aria-hidden="true">→</span>
            </p>
          ) : null}

          <div className="space-y-2 md:hidden">
            {rows.map((row, rowIndex) => {
              const rowInvalid = structuredRowInvalid(templateCode, fieldKey, specs, row, invalid, checklist);
              return (
                <section
                  key={rowIndex}
                  data-structured-row-index={rowIndex}
                  aria-label={`${label}, dòng ${rowIndex + 1}`}
                  className={clsx('rounded-md border bg-white p-2', rowInvalid ? 'border-rose-300 bg-rose-50/40' : 'border-line')}
                >
                  <div className="mb-2 flex flex-wrap items-center justify-between gap-2">
                    <div className="flex min-w-0 items-center gap-2">
                      <span className="rounded-md bg-slate-100 px-2 py-1 text-xs font-semibold text-slate-600">Dòng {rowIndex + 1}</span>
                      {checklist ? <StatusBadge status={textValue(row.status || 'draft')} /> : null}
                      {checklist && row.evidence_id ? <span className="truncate text-xs font-medium text-slate-500">Đã liên kết minh chứng</span> : null}
                    </div>
                    {!displayOnly ? <StructuredDeleteButton disabled={disabled} rowIndex={rowIndex} onRemove={setPendingRemoveRowIndex} /> : null}
                  </div>
                  <div className="grid gap-2 sm:grid-cols-2">
                    {specs.map((spec) => (
                      <StructuredCell
                        key={spec.key}
                        required={spec.required || workflowStructuredOtherDetailRequired(templateCode, fieldKey, row, spec.key)}
                        spec={spec}
                        value={row[spec.key]}
                        evidence={evidence}
                        disabled={disabled}
                        displayOnly={displayOnly}
                        invalid={structuredCellInvalid(templateCode, fieldKey, spec, row, invalid, checklist)}
                        issue={invalid ? structuredCellIssue(templateCode, fieldKey, spec, row, checklist) : ''}
                        showLabel
                        onChange={(nextValue) => onUpdateCell(rowIndex, spec.key, nextValue)}
                      />
                    ))}
                  </div>
                </section>
              );
            })}
          </div>
        </>
      ) : (
        <div className="rounded-md border border-dashed border-line bg-white px-3 py-5 text-center text-sm font-medium text-slate-500">{emptyLabel}</div>
      )}
      {!displayOnly ? (
        <button
          type="button"
          className="mt-2 inline-flex items-center gap-2 rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100 focus:outline-none focus:ring-2 focus:ring-blue-100 disabled:cursor-not-allowed disabled:opacity-50"
          disabled={disabled || specs.length === 0}
          onClick={addAndFocusRow}
        >
          <Plus className="h-4 w-4" />
          Thêm dòng
        </button>
      ) : null}
      <Dialog
        open={pendingRemoveRowIndex !== null}
        title="Xóa dòng này?"
        description={pendingRemoveRowIndex === null ? undefined : `Dòng ${pendingRemoveRowIndex + 1} sẽ bị xóa khỏi “${label}”.`}
        size="sm"
        onClose={() => setPendingRemoveRowIndex(null)}
        footer={(
          <div className="flex items-center justify-end gap-2">
            <button
              type="button"
              autoFocus
              className="rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100 focus:outline-none focus:ring-2 focus:ring-blue-100"
              onClick={() => setPendingRemoveRowIndex(null)}
            >
              Hủy
            </button>
            <button
              type="button"
              className="rounded-md bg-rose-600 px-3 py-2 text-sm font-semibold text-white hover:bg-rose-700 focus:outline-none focus:ring-2 focus:ring-rose-200"
              onClick={confirmRemoveRow}
            >
              Xóa dòng
            </button>
          </div>
        )}
      >
        <div className="space-y-3 text-sm text-slate-600">
          {pendingRemoveSummary ? (
            <div className="rounded-md border border-line bg-slate-50 px-3 py-2">
              <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">Dòng đã chọn</p>
              <p className="mt-1 font-medium text-slate-700">{pendingRemoveSummary}</p>
            </div>
          ) : null}
          <p>Dữ liệu chỉ được lưu chính thức khi anh chọn Lưu nháp hoặc Chốt hồ sơ.</p>
        </div>
      </Dialog>
    </div>
  );
}

function StructuredDeleteButton({ disabled, rowIndex, onRemove }: { disabled: boolean; rowIndex: number; onRemove: (rowIndex: number) => void }) {
  return (
    <button
      type="button"
      aria-label={`Xóa dòng ${rowIndex + 1}`}
      title="Xóa dòng"
      className="rounded-md border border-line bg-white p-2 text-slate-400 hover:bg-rose-50 hover:text-rose-600 focus:outline-none focus:ring-2 focus:ring-rose-100 disabled:cursor-not-allowed disabled:opacity-40"
      disabled={disabled}
      onClick={() => onRemove(rowIndex)}
    >
      <Trash2 className="h-4 w-4" />
    </button>
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
  required,
  spec,
  value,
  evidence,
  disabled,
  displayOnly,
  invalid,
  issue,
  showLabel,
  onChange,
}: {
  required: boolean;
  spec: StructuredFieldSpec;
  value: unknown;
  evidence: EvidenceRecord[];
  disabled: boolean;
  displayOnly: boolean;
  invalid: boolean;
  issue: string;
  showLabel: boolean;
  onChange: (value: string) => void;
}) {
  const className = clsx(
    showLabel && 'mt-1',
    'w-full rounded-md border bg-white px-2 py-2 text-sm outline-none focus:ring-2 focus:ring-blue-100 disabled:bg-slate-100',
    invalid ? 'border-rose-300 bg-rose-50' : 'border-line',
  );
  const selectableEvidence = evidence.filter((record) => record.status !== 'rejected');
  if (displayOnly) {
    return (
      <div className="min-w-0">
        <span className={clsx('text-xs font-semibold text-slate-600', !showLabel && 'sr-only')}>
          {spec.label}{required ? <span className="ml-1 text-rose-600">*</span> : null}
        </span>
        <p className={clsx('whitespace-pre-wrap break-words text-sm text-slate-700', showLabel && 'mt-1')}>
          {structuredCellDisplayValue(spec, value, evidence)}
        </p>
      </div>
    );
  }
  return (
    <label className="block min-w-0">
      <span className={clsx('text-xs font-semibold text-slate-600', !showLabel && 'sr-only')}>
        {spec.label}{required ? <span className="ml-1 text-rose-600">*</span> : null}
      </span>
      {spec.type === 'textarea' ? (
        <textarea className={className} rows={2} value={textValue(value)} disabled={disabled} aria-invalid={invalid || undefined} onChange={(event) => onChange(event.target.value)} />
      ) : spec.type === 'currency' ? (
        <CurrencyInput
          className={showLabel ? 'mt-1' : undefined}
          inputClassName="px-2 py-2 text-sm"
          value={textValue(value)}
          currencyCode={spec.currencyCode}
          allowNegative={spec.allowNegative}
          maximumFractionDigits={spec.maximumFractionDigits}
          invalid={invalid}
          disabled={disabled}
          onValueChange={({ numericString }) => onChange(numericString)}
        />
      ) : spec.type === 'select' ? (
        <select className={className} value={textValue(value)} disabled={disabled} aria-invalid={invalid || undefined} onChange={(event) => onChange(event.target.value)}>
          <option value="">Chọn</option>
          {(spec.options ?? []).map((option) => <option key={option.value} value={option.value}>{option.label ?? option.value}</option>)}
        </select>
      ) : spec.type === 'evidence_reference' ? (
        <select className={className} value={textValue(value)} disabled={disabled || selectableEvidence.length === 0} aria-invalid={invalid || undefined} onChange={(event) => onChange(event.target.value)}>
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
          aria-invalid={invalid || undefined}
          onChange={(event) => onChange(event.target.value)}
        />
      )}
      {issue ? <p className="mt-1 text-xs font-medium text-rose-700">{issue}</p> : null}
    </label>
  );
}

function structuredCellDisplayValue(spec: StructuredFieldSpec, value: unknown, evidence: EvidenceRecord[]) {
  const rawValue = textValue(value);
  if (!rawValue) return '—';
  if (spec.type === 'select') {
    return spec.options?.find((option) => option.value === rawValue)?.label ?? rawValue;
  }
  if (spec.type === 'evidence_reference') {
    const record = evidence.find((item) => item.id === rawValue);
    return record ? evidenceOptionLabel(record) : rawValue;
  }
  if (spec.type === 'currency') {
    return `${rawValue} ${spec.currencyCode ?? 'VND'}`;
  }
  return rawValue;
}

function structuredRowDeleteSummary(row: AnyRecord, specs: StructuredFieldSpec[], evidence: EvidenceRecord[]) {
  for (const spec of specs) {
    if (!textValue(row[spec.key]).trim()) continue;
    return `${spec.label}: ${structuredCellDisplayValue(spec, row[spec.key], evidence)}`;
  }
  return '';
}

function structuredCellIssue(
  templateCode: string | undefined,
  fieldKey: string,
  spec: StructuredFieldSpec,
  row: AnyRecord,
  checklist: boolean,
) {
  const rawValue = row[spec.key];
  const text = textValue(rawValue).trim();
  const required = spec.required || workflowStructuredOtherDetailRequired(templateCode, fieldKey, row, spec.key);
  if (!text) return required ? `${spec.label} là bắt buộc` : '';

  if (spec.type === 'text' || spec.type === 'textarea') {
    const length = Array.from(text).length;
    if (spec.minLength !== undefined && length < spec.minLength) return `${spec.label} phải có ít nhất ${spec.minLength} ký tự`;
    if (spec.maxLength !== undefined && length > spec.maxLength) return `${spec.label} không được vượt quá ${spec.maxLength} ký tự`;
  }
  if (spec.type === 'number' || spec.type === 'currency') {
    const numeric = Number(text);
    if (!Number.isFinite(numeric)) return `${spec.label} phải là số`;
    if (spec.min !== undefined && numeric < spec.min) return `${spec.label} phải lớn hơn hoặc bằng ${spec.min}`;
    if (spec.max !== undefined && numeric > spec.max) return `${spec.label} phải nhỏ hơn hoặc bằng ${spec.max}`;
  }
  if (spec.type === 'date') {
    const date = /^\d{4}-\d{2}-\d{2}$/.test(text) ? new Date(`${text}T00:00:00Z`) : null;
    if (!date || Number.isNaN(date.getTime()) || date.toISOString().slice(0, 10) !== text) return `${spec.label} không hợp lệ`;
  }
  if (spec.type === 'select' && spec.options?.length && !spec.options.some((option) => option.value === text)) {
    return `${spec.label} không nằm trong schema`;
  }
  if (checklist && spec.key === 'status' && !['done', 'not_applicable'].includes(text)) {
    return 'Trạng thái phải là Hoàn tất hoặc Không áp dụng trước khi chốt';
  }
  return '';
}

function structuredSpecRequired(
  templateCode: string | undefined,
  fieldKey: string,
  rows: AnyRecord[],
  spec: StructuredFieldSpec,
) {
  return Boolean(spec.required)
    || rows.some((row) => workflowStructuredOtherDetailRequired(templateCode, fieldKey, row, spec.key));
}

function structuredCellInvalid(
  templateCode: string | undefined,
  fieldKey: string,
  spec: StructuredFieldSpec,
  row: AnyRecord,
  listInvalid: boolean,
  checklist: boolean,
) {
  if (!listInvalid) return false;
  return Boolean(structuredCellIssue(templateCode, fieldKey, spec, row, checklist));
}

function structuredRowInvalid(
  templateCode: string | undefined,
  fieldKey: string,
  specs: StructuredFieldSpec[],
  row: AnyRecord,
  listInvalid: boolean,
  checklist: boolean,
) {
  return specs.some((spec) => structuredCellInvalid(templateCode, fieldKey, spec, row, listInvalid, checklist));
}

function structuredColumnWidth(spec: StructuredFieldSpec) {
  switch (spec.type) {
    case 'textarea':
      return 280;
    case 'evidence_reference':
      return 260;
    case 'select':
      return 190;
    case 'date':
      return 160;
    case 'number':
    case 'currency':
      return 150;
    default:
      return 190;
  }
}

function structuredTableMinWidth(specs: StructuredFieldSpec[], withActions: boolean) {
  return 48 + specs.reduce((total, spec) => total + structuredColumnWidth(spec), 0) + (withActions ? 52 : 0);
}

function structuredRows(value: unknown): AnyRecord[] {
  if (!Array.isArray(value)) return [];
  return value.filter((row): row is AnyRecord => Boolean(row) && typeof row === 'object' && !Array.isArray(row));
}

function optionalFiniteNumber(value: unknown) {
  if (value === null || value === undefined || String(value).trim() === '') return undefined;
  const numeric = Number(value);
  return Number.isFinite(numeric) ? numeric : undefined;
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
      currencyCode: spec.currency_code ? String(spec.currency_code) : undefined,
      allowNegative: Boolean(spec.allow_negative),
      maximumFractionDigits: spec.maximum_fraction_digits === undefined ? undefined : Number(spec.maximum_fraction_digits),
      minLength: optionalFiniteNumber(spec.min_length),
      maxLength: optionalFiniteNumber(spec.max_length),
      min: optionalFiniteNumber(spec.min),
      max: optionalFiniteNumber(spec.max),
      options: Array.isArray(spec.options)
        ? spec.options
            .filter((option): option is AnyRecord => Boolean(option) && typeof option === 'object' && !Array.isArray(option) && Boolean(option.value))
            .map((option) => ({ value: String(option.value), label: option.label ? String(option.label) : undefined }))
        : [],
    }));
}
