import { ReactNode, useEffect, useMemo, useRef, useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { CheckCircle2, Clock3, FileText, Image as ImageIcon, Upload } from 'lucide-react';
import clsx from 'clsx';
import { useToast } from '../../components/ToastProvider';
import { post, postFormWithProgress } from '../../lib/api/client';
import type { EvidenceRecord } from '../../lib/types/domain';
import { useValidationNavigation } from '../../lib/utils/formValidationNavigation';
import {
  classifyEvidenceFile,
  evidenceSupportedFileHint,
  type EvidenceUploadBusinessType,
} from '../_shared/evidenceFileTypes';
import { EvidenceUploader, type EvidenceUploadRow } from '../_shared/EvidenceUploader';
import { EvidenceDraftViewerDialog } from '../_shared/EvidenceRecordViewerDialog';
import { EvidenceGallery, formatEvidenceFileSize, isImageEvidence, type EvidenceReviewAction } from './EvidenceGallery';
import {
  evidenceUploadBatchLimit,
  evidenceUploadConcurrency,
  failedEvidenceUploadRowIds,
  runEvidenceUploadQueue,
} from './evidenceUploadQueue';
import type { WorkItemRequirementNavigationRequest } from './workItemRequirements';
import { revealWorkItemRequirementTarget } from './workItemRequirementNavigation';

const maxEvidenceUploadBytes = 10 * 1024 * 1024;

type EvidencePanelProps = {
  workItemId: string;
  projectId?: string;
  evidence: EvidenceRecord[];
  readOnly?: boolean;
  navigationRequest?: WorkItemRequirementNavigationRequest | null;
  variant?: 'primary' | 'embedded';
};

let evidenceUploadRowSequence = 0;

export function EvidencePanel({
  workItemId,
  projectId,
  evidence,
  readOnly = false,
  navigationRequest,
  variant = 'embedded',
}: EvidencePanelProps) {
  const [previewRow, setPreviewRow] = useState<EvidenceUploadRow | null>(null);
  const [uploadOpen, setUploadOpen] = useState(false);
  const [uploadRows, setUploadRows] = useState<EvidenceUploadRow[]>(() => [createEvidenceUploadRow()]);
  const [batchUploading, setBatchUploading] = useState(false);
  const [requirementEvidenceType, setRequirementEvidenceType] = useState<EvidenceUploadBusinessType | null>(null);
  const panelRef = useRef<HTMLElement | null>(null);
  const uploadFormRef = useRef<HTMLFormElement | null>(null);
  const fileInputRef = useRef<HTMLInputElement | null>(null);
  const fileInputRefs = useRef(new Map<string, HTMLInputElement>());
  const lastHandledNavigationRef = useRef<number | null>(null);
  const navigateUploadValidation = useValidationNavigation(uploadFormRef);
  const stats = useMemo(() => evidenceStats(evidence), [evidence]);
  const queryClient = useQueryClient();
  const toast = useToast();

  useEffect(() => {
    if (navigationRequest?.target.kind !== 'evidence') return undefined;
    if (lastHandledNavigationRef.current === navigationRequest.requestId) return undefined;
    lastHandledNavigationRef.current = navigationRequest.requestId;

    if (navigationRequest.target.openUpload && !readOnly) {
      setRequirementEvidenceType(navigationRequest.target.evidenceType ?? null);
      setUploadOpen(true);
      let removeHighlight: (() => void) | undefined;
      const timer = window.setTimeout(() => {
        removeHighlight = revealWorkItemRequirementTarget(uploadFormRef.current, fileInputRef.current);
      }, 0);
      return () => {
        window.clearTimeout(timer);
        removeHighlight?.();
      };
    }

    return revealWorkItemRequirementTarget(panelRef.current, panelRef.current);
  }, [navigationRequest?.requestId, navigationRequest?.target.kind, readOnly]);

  const reviewEvidence = useMutation({
    mutationFn: ({ record, action, comment }: { record: EvidenceRecord; action: EvidenceReviewAction; comment: string }) =>
      post<EvidenceRecord>(`/evidence/${record.id}/${action}`, { comment, reason: action === 'reject' ? comment : undefined }),
    onSuccess: (_record, variables) => {
      invalidateEvidenceQueries();
      toast.success(variables.action === 'accept' ? 'Đã chấp nhận minh chứng' : 'Đã từ chối minh chứng');
    },
    onError: (error) => toast.error('Không rà soát được minh chứng', error),
  });

  function invalidateEvidenceQueries() {
    queryClient.invalidateQueries({ queryKey: ['work-item', workItemId] });
    queryClient.invalidateQueries({ queryKey: ['my-work-items'] });
    if (projectId) {
      queryClient.invalidateQueries({ queryKey: ['project-operations', projectId] });
      queryClient.invalidateQueries({ queryKey: ['project-evidence', projectId] });
    }
  }

  function updateUploadRow(id: string, update: (row: EvidenceUploadRow) => EvidenceUploadRow) {
    setUploadRows((current) => current.map((row) => row.id === id ? update(row) : row));
  }

  function onFileChange(rowId: string, file: File | null, input: HTMLInputElement) {
    if (!file) {
      updateUploadRow(rowId, (row) => ({ ...createEvidenceUploadRow(row.id), caption: row.caption }));
      return;
    }

    const classification = classifyEvidenceFile(file);
    if (!classification) {
      const issue = `Chỉ hỗ trợ ${evidenceSupportedFileHint}.`;
      input.value = '';
      updateUploadRow(rowId, (row) => ({ ...row, file: null, classification: null, name: '', fileIssue: issue, progress: 0, status: 'queued', error: '' }));
      toast.warn({ title: 'Tệp không được hỗ trợ', description: `${file.name}: ${issue}` });
      navigateUploadValidation(rowId);
      return;
    }
    if (file.size <= 0 || file.size > maxEvidenceUploadBytes) {
      const issue = file.size <= 0
        ? 'Tệp minh chứng không được rỗng.'
        : `Dung lượng tối đa ${formatEvidenceFileSize(maxEvidenceUploadBytes)}.`;
      input.value = '';
      updateUploadRow(rowId, (row) => ({ ...row, file: null, classification: null, name: '', fileIssue: issue, progress: 0, status: 'queued', error: '' }));
      toast.warn({ title: 'Dung lượng tệp không hợp lệ', description: `${file.name}: ${issue}` });
      navigateUploadValidation(rowId);
      return;
    }

    updateUploadRow(rowId, (row) => ({
      ...row,
      file,
      classification,
      name: file.name,
      fileIssue: '',
      status: 'queued',
      progress: 0,
      error: '',
    }));
    if (requirementEvidenceType === 'photo' && classification.evidenceType === 'document') {
      toast.warn({ title: 'Chưa đáp ứng điều kiện Hình ảnh', description: `${file.name} được nhận diện là ${classification.label}; tệp vẫn có thể tải lên nhưng không được tính là ảnh minh chứng.` });
    }
  }

  function addUploadRow() {
    if (uploadRows.length >= evidenceUploadBatchLimit) {
      toast.warn({ title: 'Đã đạt giới hạn', description: `Mỗi lượt hỗ trợ tối đa ${evidenceUploadBatchLimit} tệp.` });
      return;
    }
    const row = createEvidenceUploadRow();
    setUploadRows((current) => [...current, row]);
    window.setTimeout(() => fileInputRefs.current.get(row.id)?.focus(), 0);
  }

  function removeUploadRow(rowId: string) {
    const input = fileInputRefs.current.get(rowId);
    if (input) fileInputRefs.current.delete(rowId);
    setUploadRows((current) => {
      const next = current.filter((row) => row.id !== rowId);
      return next.length ? next : [createEvidenceUploadRow()];
    });
  }

  async function uploadEvidenceRows(rowIds?: readonly string[]) {
    if (batchUploading) return;
    const included = rowIds ? new Set(rowIds) : null;
    const rows = uploadRows.filter((row) => (!included || included.has(row.id)) && isUploadableRow(row));
    if (!rows.length) {
      toast.warn({ title: 'Chưa có tệp chờ tải', description: `Chọn tối đa ${evidenceUploadBatchLimit} tệp thuộc nhóm ${evidenceSupportedFileHint}.` });
      navigateUploadValidation(uploadRows[0]?.id);
      return;
    }

    setBatchUploading(true);
    const results = await runEvidenceUploadQueue(rows, async (row) => {
      if (!row.file || !row.classification) throw new Error('Tệp chưa sẵn sàng để tải lên.');
      updateUploadRow(row.id, (current) => ({ ...current, status: 'uploading', progress: 0, error: '' }));

      const formData = new FormData();
      formData.append('file', row.file);
      formData.append('evidence_type', row.classification.evidenceType);
      formData.append('visibility', 'internal');
      formData.append('name', row.name.trim() || row.file.name);
      if (row.caption.trim()) formData.append('caption', row.caption.trim());

      try {
        const record = await postFormWithProgress<EvidenceRecord>(
          `/work-items/${workItemId}/evidence/upload`,
          formData,
          ({ percent }) => updateUploadRow(row.id, (current) => ({
            ...current,
            progress: percent,
            status: percent >= 100 ? 'processing' : 'uploading',
          })),
        );
        updateUploadRow(row.id, (current) => ({ ...current, status: 'success', progress: 100, error: '' }));
        return record;
      } catch (error) {
        updateUploadRow(row.id, (current) => ({ ...current, status: 'error', progress: 0, error: uploadErrorMessage(error) }));
        throw error;
      }
    }, evidenceUploadConcurrency);

    setBatchUploading(false);
    const failedRowIds = failedEvidenceUploadRowIds(rows, results);
    const successCount = results.filter((result) => result.status === 'fulfilled').length;
    const failureCount = failedRowIds.size;
    if (successCount) invalidateEvidenceQueries();

    if (!failureCount) {
      setUploadRows([createEvidenceUploadRow()]);
      toast.success({ title: `Đã tải xong ${successCount} tệp` });
      return;
    }

    setUploadRows((current) => {
      const failedRows = current.filter((row) => failedRowIds.has(row.id));
      return failedRows.length ? failedRows : [createEvidenceUploadRow()];
    });
    navigateUploadValidation(failedRowIds.values().next().value);
    if (successCount) {
      toast.warn({
        title: `Đã tải xong ${successCount} tệp · ${failureCount} tệp lỗi`,
        description: 'Các tệp thành công đã được dọn khỏi danh sách. Có thể thử lại từng dòng lỗi.',
      });
    } else {
      toast.error({
        title: 'Không tải được minh chứng',
        description: `${failureCount} tệp lỗi. Có thể thử lại từng dòng.`,
      });
    }
  }

  return (
    <section
      ref={panelRef}
      tabIndex={-1}
      className={clsx(
        'scroll-mt-24 space-y-3 outline-none',
        variant === 'primary'
          ? 'rounded-lg border border-line bg-white p-4 shadow-panel'
          : 'rounded-md',
      )}
    >
      <div className="flex flex-col gap-3 rounded-md border border-line bg-slate-50 px-3 py-3 lg:flex-row lg:items-center lg:justify-between">
        <div className="min-w-0">
          <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">Sổ minh chứng</p>
          <h3 className="mt-0.5 text-sm font-semibold text-ink">{evidence.length} hồ sơ minh chứng</h3>
          <p className="mt-1 text-xs text-slate-500">Ảnh {stats.photos} · Tài liệu {stats.documents} · Đang chờ {stats.pending}</p>
        </div>
        {!readOnly ? <button
          type="button"
          className={clsx(
            'inline-flex items-center justify-center gap-2 rounded-md px-3 py-2 text-sm font-semibold disabled:cursor-not-allowed disabled:opacity-60',
            uploadOpen ? 'border border-line bg-white text-slate-700 hover:bg-slate-100' : 'bg-blue-600 text-white hover:bg-blue-700',
          )}
          disabled={batchUploading}
          onClick={() => setUploadOpen((current) => {
            if (!current) setRequirementEvidenceType(null);
            return !current;
          })}
        >
          <Upload className="h-4 w-4" />
          {uploadOpen ? 'Ẩn tải lên' : 'Tải minh chứng'}
        </button> : <span className="rounded-md bg-amber-50 px-3 py-2 text-xs font-semibold text-amber-700">Chỉ xem</span>}
      </div>

      <div className="grid gap-2 sm:grid-cols-4">
        <EvidenceStat icon={<ImageIcon className="h-3.5 w-3.5" />} label="Ảnh" value={stats.photos} />
        <EvidenceStat icon={<FileText className="h-3.5 w-3.5" />} label="Tài liệu" value={stats.documents} />
        <EvidenceStat icon={<CheckCircle2 className="h-3.5 w-3.5" />} label="Đã chấp nhận" value={stats.accepted} tone="ok" />
        <EvidenceStat icon={<Clock3 className="h-3.5 w-3.5" />} label="Đang chờ" value={stats.pending} tone={stats.pending ? 'warn' : 'neutral'} />
      </div>

      {!readOnly && uploadOpen ? (
        <form
          ref={uploadFormRef}
          tabIndex={-1}
          className="scroll-mt-24 rounded-md border border-blue-100 bg-blue-50/40 p-3 outline-none"
          onSubmit={(event) => {
            event.preventDefault();
            void uploadEvidenceRows();
          }}
          noValidate
        >
          <EvidenceUploader addDisabled={uploadRows.length >= evidenceUploadBatchLimit} uploadRows={uploadRows} batchUploading={batchUploading} updateUploadRow={updateUploadRow} onFileChange={onFileChange} addUploadRow={addUploadRow} removeUploadRow={removeUploadRow} uploadEvidenceRows={uploadEvidenceRows} requirementEvidenceType={requirementEvidenceType} fileInputRef={fileInputRef} fileInputRefs={fileInputRefs} onPreview={setPreviewRow} />
        </form>
      ) : null}

      <div>
        <EvidenceGallery
          evidence={evidence}
          emptyTitle="Chưa có minh chứng cho đầu việc này"
          variant="list"
          onReview={readOnly ? undefined : (record, action, comment) => reviewEvidence.mutateAsync({ record, action, comment }).then(() => undefined)}
        />
      </div>
      <EvidenceDraftViewerDialog open={Boolean(previewRow?.file)} name={previewRow?.name ?? ''} originalName={previewRow?.file?.name ?? ''} file={previewRow?.file ?? undefined} mimeType={previewRow?.file?.type} fileSize={previewRow?.file?.size ?? 0} caption={previewRow?.caption} onClose={() => setPreviewRow(null)} />
    </section>
  );
}

function createEvidenceUploadRow(id = nextEvidenceUploadRowId()): EvidenceUploadRow {
  return {
    id,
    file: null,
    classification: null,
    name: '',
    caption: '',
    fileIssue: '',
    status: 'queued',
    progress: 0,
    error: '',
  };
}

function nextEvidenceUploadRowId() {
  evidenceUploadRowSequence += 1;
  return `evidence-upload-${evidenceUploadRowSequence}`;
}

function isUploadableRow(row: EvidenceUploadRow) {
  return Boolean(row.file && row.classification && !row.fileIssue && (row.status === 'queued' || row.status === 'error'));
}

function uploadErrorMessage(error: unknown) {
  if (error instanceof Error && error.message) return error.message;
  return 'Không tải được tệp. Vui lòng thử lại.';
}

function evidenceStats(evidence: EvidenceRecord[]) {
  return {
    photos: evidence.filter(isImageEvidence).length,
    documents: evidence.filter((record) => record.evidence_type === 'document' || String(record.mime_type ?? '').includes('pdf')).length,
    accepted: evidence.filter((record) => record.status === 'accepted').length,
    pending: evidence.filter((record) => ['draft', 'submitted', 'pending'].includes(record.status)).length,
  };
}

function EvidenceStat({
  icon,
  label,
  value,
  tone = 'neutral',
}: {
  icon: ReactNode;
  label: string;
  value: number;
  tone?: 'neutral' | 'ok' | 'warn';
}) {
  return (
    <div className={clsx(
      'flex min-w-0 items-center gap-2 rounded-md px-2.5 py-2 text-xs font-semibold ring-1 ring-inset',
      tone === 'ok' ? 'bg-emerald-50 text-emerald-700 ring-emerald-200' : tone === 'warn' ? 'bg-amber-50 text-amber-800 ring-amber-200' : 'bg-white text-slate-600 ring-line',
    )}>
      <span className="shrink-0">{icon}</span>
      <span className="min-w-0">
        <span className="block text-[11px] uppercase tracking-wide opacity-75">{label}</span>
        <span className="block">{value}</span>
      </span>
    </div>
  );
}
