import { useRef, useState, useSyncExternalStore } from 'react';
import { Camera, CameraIcon, Loader2, SwitchCamera } from 'lucide-react';
import { useToast } from '../../components/ToastProvider';
import { Dialog } from '../../components/Dialog';
import './evidenceCapture.css';
import { createCameraCaptureSession } from './cameraCaptureSession';
import { EvidenceDraftViewerDialog, formatEvidenceFileSize } from './EvidenceRecordViewerDialog';
import { EvidenceUploader, type EvidenceUploadRow } from './EvidenceUploader';
import { evidenceUploadSnapshot, retryEvidenceUpload, subscribeEvidenceUpload } from './evidenceUploadSession';
import { classifyEvidenceFile, evidenceAcceptForKinds, evidenceFileAllowed, evidenceSupportedFileHint, type EvidenceFileKind } from './evidenceFileTypes';
import { useValidationNavigation } from '../../lib/utils/formValidationNavigation';
import { useEffect } from 'react';
import { appendEvidenceCaptureMetadata, type EvidenceCaptureMetadata } from './evidenceCaptureMetadata';

export const constructionEvidenceMaxBytes = 10 * 1024 * 1024;
export type ConstructionEvidenceUploadDraft = {
  id: string; file: File; evidenceType: 'photo' | 'document';
  fileKind: EvidenceFileKind; name: string; caption: string;
  captureMetadata?: EvidenceCaptureMetadata;
};
export type EvidenceCaptureIntent = 'file' | 'camera';
type ConstructionEvidenceCaptureProps = {
  allowedFileKinds?: readonly EvidenceFileKind[];
  drafts: ConstructionEvidenceUploadDraft[];
  onChange: (drafts: ConstructionEvidenceUploadDraft[]) => void;
  hasSource: boolean;
  uploadScope: string;
  linkOnUpload?: boolean;
  disabled?: boolean;
  hint?: string;
  maxFiles?: number;
  onSourceRequired?: (intent: EvidenceCaptureIntent, resume: () => void) => void;
};

export function ConstructionEvidenceCapture({ allowedFileKinds, drafts, onChange, hasSource, uploadScope, linkOnUpload = false, disabled = false, hint, maxFiles = 10, onSourceRequired }: ConstructionEvidenceCaptureProps) {
  const toast = useToast();
  const fileInputRef = useRef<HTMLInputElement | null>(null);
  const fallbackCameraInputRef = useRef<HTMLInputElement | null>(null);
  const captureRef = useRef<HTMLDivElement | null>(null);
  const navigateValidation = useValidationNavigation(captureRef);
  const [cameraOpen, setCameraOpen] = useState(false);
  const [preview, setPreview] = useState<ConstructionEvidenceUploadDraft | null>(null);
  const [fileIssue, setFileIssue] = useState('');
  const draftsRef = useRef(drafts);
  draftsRef.current = drafts;
  useSyncExternalStore(
    (listener) => { const removers = drafts.map((draft) => subscribeEvidenceUpload(draft, uploadScope, listener)); return () => removers.forEach((remove) => remove()); },
    () => JSON.stringify(drafts.map((draft) => evidenceUploadSnapshot(draft, uploadScope))),
    () => '',
  );
  const uploadRows: EvidenceUploadRow[] = drafts.map((draft) => ({ ...draft, classification: classifyEvidenceFile(draft.file), fileIssue: '', ...evidenceUploadSnapshot(draft, uploadScope) }));
  const uploading = disabled || uploadRows.some((row) => row.status === 'uploading' || row.status === 'processing');

  function addFiles(files: File[], replaceId?: string, captureMetadata?: EvidenceCaptureMetadata) {
    if (uploading) return false;
    let issue = '';
    const valid: ConstructionEvidenceUploadDraft[] = [];
    files.forEach((file) => {
      const classification = classifyEvidenceFile(file);
      if (!classification || !evidenceFileAllowed(file, allowedFileKinds)) { issue = `${file.name}: chỉ nhận ${allowedFileKinds?.length === 1 && allowedFileKinds[0] === 'photo' ? 'PNG/JPG/JPEG/WebP/GIF' : evidenceSupportedFileHint}.`; return; }
      if (file.size <= 0 || file.size > constructionEvidenceMaxBytes) { issue = `${file.name}: tệp phải có nội dung, tối đa ${formatEvidenceFileSize(constructionEvidenceMaxBytes)}.`; return; }
      valid.push({ id: newDraftID(), file, evidenceType: classification.evidenceType, fileKind: classification.kind, name: file.name, caption: '', captureMetadata: classification.kind === 'photo' ? { ...captureMetadata } : undefined });
    });
    const previous = draftsRef.current;
    const replacing = previous.some((draft) => draft.id === replaceId);
    const available = Math.max(0, maxFiles - previous.length + Number(replacing));
    if (valid.length > available) issue = `Mỗi nhóm được đính kèm tối đa ${maxFiles} ảnh hoặc tệp.`;
    setFileIssue(issue);
    if (issue) { toast.warn({ title: 'Kiểm tra tệp minh chứng', description: issue }); navigateValidation('evidence_files'); }
    const accepted = valid.slice(0, available);
    if (!accepted.length) return false;
    const next = replacing ? previous.map((draft) => draft.id === replaceId ? { ...accepted[0], caption: draft.caption } : draft) : [...previous, ...accepted];
    draftsRef.current = next;
    onChange(next);
    return true;
  }
  function requestCapture(intent: EvidenceCaptureIntent, resume: () => void) {
    if (uploading) return;
    if (!hasSource && onSourceRequired) { onSourceRequired(intent, resume); return; }
    resume();
  }
  function updateUploadRow(id: string, update: (row: EvidenceUploadRow) => EvidenceUploadRow) {
    const row = uploadRows.find((item) => item.id === id);
    if (!row || uploading || row.status === 'success') return;
    const next = update(row);
    onChange(drafts.map((draft) => draft.id === id ? { ...draft, name: next.name, caption: next.caption, captureMetadata: next.captureMetadata } : draft));
  }
  const rows = uploadRows.length ? uploadRows : [{ id: 'evidence_files', file: null, classification: null, name: '', caption: '', fileIssue, status: 'queued' as const, progress: 0, error: '' }];
  return <div ref={captureRef} className="min-w-0 rounded-md border border-blue-100 bg-blue-50/40 p-3" data-validation-key="evidence_files" data-validation-error={fileIssue ? 'true' : undefined}>
    <input ref={fileInputRef} type="file" multiple accept={evidenceAcceptForKinds(allowedFileKinds)} className="hidden" onChange={(event) => { addFiles(Array.from(event.target.files ?? [])); event.target.value = ''; }} />
    <input ref={fallbackCameraInputRef} type="file" accept="image/*" capture="environment" className="hidden" onChange={(event) => { addFiles(Array.from(event.target.files ?? [])); event.target.value = ''; }} />
    <EvidenceUploader allowedFileKinds={allowedFileKinds} uploadRows={rows} batchUploading={uploading} maxFiles={maxFiles}
      updateUploadRow={updateUploadRow}
      onFileChange={(id, file, input) => { if (file) requestCapture('file', () => addFiles([file], id)); input.value = ''; }}
      addUploadRow={() => requestCapture('file', () => fileInputRef.current?.click())}
      removeUploadRow={(id) => { if (!uploading) onChange(drafts.filter((draft) => draft.id !== id)); }}
      canRemoveRow={(row) => !linkOnUpload || row.status !== 'success'}
      retryUploadRow={(id) => { const draft = drafts.find((item) => item.id === id); if (draft) void retryEvidenceUpload(draft, uploadScope)?.catch((error: unknown) => toast.error('Không tải được minh chứng', error)); }}
      onPreview={(row) => setPreview(drafts.find((draft) => draft.id === row.id) ?? null)}
      actions={<button type="button" disabled={uploading || drafts.length >= maxFiles} className="inline-flex items-center gap-2 rounded-md border border-blue-200 bg-white px-3 py-2 text-sm font-semibold text-blue-700 disabled:opacity-50" onClick={() => requestCapture('camera', () => setCameraOpen(true))}><Camera className="h-4 w-4" /> Mở camera</button>}
    />
    {fileIssue && drafts.length ? <p role="alert" className="mt-2 text-xs text-rose-700">{fileIssue}</p> : null}
    <p className="mt-2 text-xs text-slate-500">{hint ?? 'Tệp được tải lên khi lưu cùng công việc liên quan đã chọn.'}</p>
    {!hasSource && drafts.length ? <p className="mt-2 text-xs text-amber-800">Đã giữ {drafts.length} tệp trong hàng đợi. Chọn công việc liên quan trước khi lưu.</p> : null}
    {uploadRows.some((row) => row.status === 'success') ? <p className="mt-2 text-xs text-emerald-700">{linkOnUpload ? 'Tệp tải thành công đã được đính kèm vào báo cáo.' : 'Đã giữ tệp tải thành công. Lưu để hoàn tất đính kèm.'}</p> : null}
    <CameraCaptureModal open={cameraOpen && !uploading} selectedCount={drafts.filter((draft) => draft.evidenceType === 'photo').length} atCapacity={drafts.length >= maxFiles} onClose={() => setCameraOpen(false)} onUse={(file, capturedAt) => addFiles([file], undefined, { captured_at: capturedAt })} onFallback={() => { setCameraOpen(false); fallbackCameraInputRef.current?.click(); }} />
    <EvidenceDraftViewerDialog open={Boolean(preview)} name={preview?.name ?? 'Minh chứng'} originalName={preview?.file.name ?? ''} file={preview?.file} mimeType={preview?.file.type} fileSize={preview?.file.size ?? 0} caption={preview?.caption} onClose={() => setPreview(null)} />
  </div>;
}

export function constructionEvidenceFormData(draft: ConstructionEvidenceUploadDraft, workItemId: string) {
  const classification = classifyEvidenceFile(draft.file);
  const formData = new FormData();
  formData.append('file', draft.file);
  formData.append('work_item_id', workItemId);
  formData.append('evidence_type', classification?.evidenceType ?? draft.evidenceType);
  formData.append('visibility', 'internal');
  formData.append('name', draft.name.trim() || draft.file.name);
  if (draft.caption.trim()) formData.append('caption', draft.caption.trim());
  if (classification?.kind === 'photo') appendEvidenceCaptureMetadata(formData, draft.captureMetadata);
  return formData;
}

function CameraCaptureModal({ open, selectedCount = 0, atCapacity = false, onClose, onUse, onFallback }: { open: boolean; selectedCount?: number; atCapacity?: boolean; onClose: () => void; onUse: (file: File, capturedAt: string) => boolean; onFallback: () => void }) {
  const videoRef = useRef<HTMLVideoElement | null>(null);
  const streamRef = useRef<MediaStream | null>(null);
  const [facingMode, setFacingMode] = useState<'environment' | 'user'>('environment');
  const [status, setStatus] = useState<'loading' | 'ready' | 'error'>('loading');
  const [error, setError] = useState('');
  const [captured, setCaptured] = useState<{ file: File; url: string; capturedAt: string } | null>(null);
  const [captureBusy, setCaptureBusy] = useState(false);
  const captureSession = useRef(createCameraCaptureSession());
  const usedFile = useRef<File | null>(null);

  useEffect(() => {
    if (!open) return undefined;
    let disposed = false;
    captureSession.current.invalidate();
    setCaptureBusy(false);
    setCaptured((current) => { if (current) URL.revokeObjectURL(current.url); return null; });
    setStatus('loading');
    setError('');
    stopCamera(streamRef);
    if (!navigator.mediaDevices?.getUserMedia) {
      setStatus('error');
      setError('Trình duyệt không hỗ trợ camera trực tiếp hoặc trang chưa chạy trong ngữ cảnh an toàn.');
      return undefined;
    }
    navigator.mediaDevices.getUserMedia({ audio: false, video: { facingMode: { ideal: facingMode } } })
      .then(async (stream) => {
        if (disposed) { stream.getTracks().forEach((track) => track.stop()); return; }
        streamRef.current = stream;
        if (videoRef.current) {
          videoRef.current.srcObject = stream;
          await videoRef.current.play().catch(() => undefined);
        }
        setStatus('ready');
      })
      .catch((cameraError: unknown) => {
        if (disposed) return;
        setStatus('error');
        const name = cameraError instanceof DOMException ? cameraError.name : '';
        setError(name === 'NotAllowedError' ? 'Quyền camera đã bị từ chối. Có thể cấp quyền trong trình duyệt hoặc dùng camera hệ thống.' : 'Không mở được camera trên thiết bị này.');
      });
    return () => {
      disposed = true;
      captureSession.current.invalidate();
      stopCamera(streamRef);
    };
  }, [facingMode, open]);

  useEffect(() => () => { if (captured) URL.revokeObjectURL(captured.url); }, [captured]);

  if (!open) return null;

  function close() {
    captureSession.current.invalidate();
    stopCamera(streamRef);
    onClose();
  }

  function capture() {
    const video = videoRef.current;
    if (!video || !video.videoWidth || !video.videoHeight) return;
    const canvas = document.createElement('canvas');
    canvas.width = video.videoWidth;
    canvas.height = video.videoHeight;
    const context = canvas.getContext('2d');
    if (!context) return;
    const ticket = captureSession.current.begin();
    if (ticket === null) return;
    setCaptureBusy(true);
    setError('');
    try {
      // Keep the shutter instant with its frame across encoding and user review.
      const capturedAt = new Date().toISOString();
      context.drawImage(video, 0, 0, canvas.width, canvas.height);
      canvas.toBlob((blob) => {
        if (!captureSession.current.finish(ticket)) return;
        setCaptureBusy(false);
        if (!blob) { setError('Không tạo được ảnh. Vui lòng chụp lại.'); return; }
        const file = new File([blob], `hien-truong-${capturedAt.replace(/[:.]/g, '-')}.jpg`, { type: 'image/jpeg' });
        setCaptured((current) => { if (current) URL.revokeObjectURL(current.url); return { file, url: URL.createObjectURL(file), capturedAt }; });
      }, 'image/jpeg', 0.9);
    } catch {
      if (captureSession.current.finish(ticket)) {
        setCaptureBusy(false);
        setError('Không tạo được ảnh. Vui lòng chụp lại.');
      }
    }
  }

  function useCaptured(keepOpen = false) {
    if (!captured || usedFile.current === captured.file) return;
    if (!onUse(captured.file, captured.capturedAt)) return;
    usedFile.current = captured.file;
    if (keepOpen) setCaptured(null);
    else close();
  }

  return <Dialog open title="Chụp ảnh hiện trường" description={'Đã chọn: ' + selectedCount + ' ảnh'} size="lg" mobileFullscreen manageFocus onClose={close} footer={<div className="evidence-camera-actions space-y-2">
    {atCapacity ? <p role="status" className="text-sm text-amber-800">Đã đạt giới hạn ảnh/tệp đã chọn.</p> : null}
    <div className="grid grid-cols-2 gap-2">
      {captured ? <><button type="button" className="min-h-11 rounded-md border border-line px-3 py-2 text-sm font-semibold" onClick={() => setCaptured(null)}>Chụp lại</button><button type="button" disabled={atCapacity} className="min-h-11 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white disabled:opacity-50" onClick={() => useCaptured(true)}>Dùng & chụp tiếp</button></> : <><button type="button" disabled={status !== 'ready' || captureBusy} className="inline-flex min-h-11 items-center justify-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold disabled:opacity-50" onClick={() => setFacingMode((current) => current === 'environment' ? 'user' : 'environment')}><SwitchCamera className="h-4 w-4" /> Đổi camera</button><button type="button" disabled={status !== 'ready' || captureBusy || atCapacity} className="inline-flex min-h-11 items-center justify-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white disabled:opacity-50" onClick={capture}>{captureBusy ? <Loader2 className="h-4 w-4 animate-spin" /> : <Camera className="h-4 w-4" />} Chụp ảnh</button></>}
    </div>
    <div className="flex items-center justify-between gap-2"><button type="button" disabled={captureBusy || atCapacity} className="min-h-11 px-2 py-2 text-sm font-semibold text-blue-700 disabled:opacity-50" onClick={onFallback}>Camera hệ thống</button><button type="button" disabled={captureBusy} className="min-h-11 rounded-md border border-line px-5 py-2 text-sm font-semibold disabled:opacity-50" onClick={() => captured ? useCaptured() : close()}>Xong</button></div>
  </div>}>
    <div className="evidence-camera-content">
      <div className="evidence-camera-frame relative mx-auto aspect-[4/3] overflow-hidden rounded-lg bg-black">
        <video ref={videoRef} playsInline muted className={'h-full w-full object-contain ' + (captured ? 'hidden' : '')} />
        {captured ? <img src={captured.url} alt="Ảnh vừa chụp" className="h-full w-full object-contain" /> : null}
        {status === 'loading' ? <div className="absolute inset-0 grid place-items-center text-white"><span className="inline-flex items-center gap-2 text-sm"><Loader2 className="h-5 w-5 animate-spin" /> Đang mở camera...</span></div> : null}
        {status === 'error' ? <div role="alert" className="absolute inset-0 grid place-items-center p-5 text-center text-sm text-white"><div><CameraIcon className="mx-auto mb-3 h-8 w-8" /><p>{error}</p><p className="mt-3">Chọn “Camera hệ thống” để thử cách khác.</p></div></div> : null}
      </div>
      {error && status !== 'error' ? <p role="alert" className="mt-2 text-sm text-rose-700">{error}</p> : null}
    </div>
  </Dialog>;

}

function stopCamera(ref: React.MutableRefObject<MediaStream | null>) {
  ref.current?.getTracks().forEach((track) => track.stop());
  ref.current = null;
}

function newDraftID() {
  return typeof crypto !== 'undefined' && 'randomUUID' in crypto ? crypto.randomUUID() : `evidence-${Date.now()}-${Math.random().toString(16).slice(2)}`;
}

export const EvidenceCapture = ConstructionEvidenceCapture;
export const evidenceUploadFormData = constructionEvidenceFormData;
export const evidenceMaxBytes = constructionEvidenceMaxBytes;
export type EvidenceUploadDraft = ConstructionEvidenceUploadDraft;
