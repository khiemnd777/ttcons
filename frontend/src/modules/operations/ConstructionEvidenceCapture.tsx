import { useEffect, useMemo, useRef, useState } from 'react';
import { Camera, CameraIcon, FileText, Image, Loader2, RefreshCw, SwitchCamera, Trash2, Upload, X } from 'lucide-react';
import { useToast } from '../../components/ToastProvider';
import { formatEvidenceFileSize } from '../work-items/EvidenceGallery';

export const constructionEvidenceMaxBytes = 10 * 1024 * 1024;

export type ConstructionEvidenceUploadDraft = {
  id: string;
  file: File;
  evidenceType: 'photo' | 'document';
  name: string;
  caption: string;
};

type ConstructionEvidenceCaptureProps = {
  drafts: ConstructionEvidenceUploadDraft[];
  onChange: (drafts: ConstructionEvidenceUploadDraft[]) => void;
  hasSource: boolean;
  disabled?: boolean;
  compact?: boolean;
};

export function ConstructionEvidenceCapture({ drafts, onChange, hasSource, disabled = false, compact = false }: ConstructionEvidenceCaptureProps) {
  const toast = useToast();
  const fileInputRef = useRef<HTMLInputElement | null>(null);
  const fallbackCameraInputRef = useRef<HTMLInputElement | null>(null);
  const [cameraOpen, setCameraOpen] = useState(false);

  function addFiles(files: File[]) {
    const valid: ConstructionEvidenceUploadDraft[] = [];
    files.forEach((file) => {
      const evidenceType = evidenceTypeForFile(file);
      if (!evidenceType) {
        toast.warn({ title: 'Tệp không được hỗ trợ', description: `${file.name}: chỉ nhận PNG, JPG, WebP hoặc PDF.` });
        return;
      }
      if (file.size <= 0 || file.size > constructionEvidenceMaxBytes) {
        toast.warn({ title: 'Dung lượng tệp không hợp lệ', description: `${file.name}: tối đa ${formatEvidenceFileSize(constructionEvidenceMaxBytes)}.` });
        return;
      }
      valid.push({ id: newDraftID(), file, evidenceType, name: file.name, caption: '' });
    });
    if (valid.length) onChange([...drafts, ...valid]);
  }

  function updateDraft(id: string, patch: Partial<ConstructionEvidenceUploadDraft>) {
    onChange(drafts.map((draft) => draft.id === id ? { ...draft, ...patch } : draft));
  }

  return (
    <div className="rounded-lg border border-dashed border-blue-300 bg-blue-50/40 p-4">
      <input ref={fileInputRef} type="file" multiple accept="image/png,image/jpeg,image/webp,application/pdf" className="hidden" onChange={(event) => { addFiles(Array.from(event.target.files ?? [])); event.target.value = ''; }} />
      <input ref={fallbackCameraInputRef} type="file" accept="image/*" capture="environment" className="hidden" onChange={(event) => { addFiles(Array.from(event.target.files ?? [])); event.target.value = ''; }} />
      <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <div className="flex items-center gap-2"><Upload className="h-4 w-4 text-blue-600" /><h4 className="text-sm font-semibold text-ink">Tệp và ảnh hiện trường</h4></div>
          <p className="mt-1 text-xs leading-5 text-slate-500">Tệp chỉ được tải lên sau khi lưu báo cáo và được lưu theo công việc liên quan đã chọn.</p>
        </div>
        <div className="flex flex-wrap gap-2">
          <button type="button" disabled={disabled} className="inline-flex items-center gap-2 rounded-md border border-blue-200 bg-white px-3 py-2 text-sm font-semibold text-blue-700 hover:bg-blue-50 disabled:opacity-60" onClick={() => fileInputRef.current?.click()}><Upload className="h-4 w-4" /> Chọn tệp</button>
          <button type="button" disabled={disabled} className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-3 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:opacity-60" onClick={() => setCameraOpen(true)}><Camera className="h-4 w-4" /> Mở camera</button>
        </div>
      </div>

      {!hasSource && drafts.length ? <p className="mt-3 rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-xs font-medium text-amber-800">Đã giữ {drafts.length} tệp trong hàng đợi. Chọn công việc liên quan trước khi lưu để tải minh chứng.</p> : null}
      {!drafts.length ? <div className="mt-3 rounded-md border border-line bg-white px-3 py-4 text-center text-xs text-slate-500">Chưa chọn tệp. PNG/JPG/WebP/PDF, tối đa 10 MB mỗi tệp.</div> : null}
      {drafts.length ? <div className={`mt-3 grid gap-3 ${compact ? 'sm:grid-cols-2' : 'md:grid-cols-2'}`}>{drafts.map((draft) => <EvidenceDraftCard key={draft.id} draft={draft} onChange={(patch) => updateDraft(draft.id, patch)} onRemove={() => onChange(drafts.filter((item) => item.id !== draft.id))} />)}</div> : null}

      <CameraCaptureModal
        open={cameraOpen}
        onClose={() => setCameraOpen(false)}
        onUse={(file) => { addFiles([file]); setCameraOpen(false); }}
        onFallback={() => { setCameraOpen(false); fallbackCameraInputRef.current?.click(); }}
      />
    </div>
  );
}

export function constructionEvidenceFormData(draft: ConstructionEvidenceUploadDraft, workItemId: string) {
  const formData = new FormData();
  formData.append('file', draft.file);
  formData.append('work_item_id', workItemId);
  formData.append('evidence_type', draft.evidenceType);
  formData.append('visibility', 'project');
  formData.append('name', draft.name.trim() || draft.file.name);
  if (draft.caption.trim()) formData.append('caption', draft.caption.trim());
  return formData;
}

function EvidenceDraftCard({ draft, onChange, onRemove }: { draft: ConstructionEvidenceUploadDraft; onChange: (patch: Partial<ConstructionEvidenceUploadDraft>) => void; onRemove: () => void }) {
  const previewURL = useMemo(() => draft.evidenceType === 'photo' ? URL.createObjectURL(draft.file) : '', [draft.evidenceType, draft.file]);
  useEffect(() => () => { if (previewURL) URL.revokeObjectURL(previewURL); }, [previewURL]);
  return <article className="overflow-hidden rounded-lg border border-line bg-white">
    <div className="flex min-h-28 items-center justify-center bg-slate-100">
      {previewURL ? <img src={previewURL} alt={draft.name} className="h-32 w-full object-cover" /> : <FileText className="h-9 w-9 text-slate-400" />}
    </div>
    <div className="space-y-2 p-3">
      <div className="flex items-start gap-2"><span className="mt-2 text-blue-600">{draft.evidenceType === 'photo' ? <Image className="h-4 w-4" /> : <FileText className="h-4 w-4" />}</span><div className="min-w-0 flex-1"><input className="w-full rounded-md border border-line px-2.5 py-1.5 text-sm font-semibold text-ink outline-none focus:border-blue-300" value={draft.name} onChange={(event) => onChange({ name: event.target.value })} aria-label="Tên minh chứng" /><p className="mt-1 text-xs text-slate-500">{formatEvidenceFileSize(draft.file.size)}</p></div><button type="button" className="rounded-md p-1.5 text-slate-400 hover:bg-rose-50 hover:text-rose-600" onClick={onRemove} aria-label={`Xóa ${draft.name}`}><Trash2 className="h-4 w-4" /></button></div>
      <input className="w-full rounded-md border border-line px-2.5 py-1.5 text-sm outline-none focus:border-blue-300" value={draft.caption} onChange={(event) => onChange({ caption: event.target.value })} placeholder="Ghi chú vị trí / hạng mục" />
    </div>
  </article>;
}

function CameraCaptureModal({ open, onClose, onUse, onFallback }: { open: boolean; onClose: () => void; onUse: (file: File) => void; onFallback: () => void }) {
  const videoRef = useRef<HTMLVideoElement | null>(null);
  const streamRef = useRef<MediaStream | null>(null);
  const [facingMode, setFacingMode] = useState<'environment' | 'user'>('environment');
  const [status, setStatus] = useState<'loading' | 'ready' | 'error'>('loading');
  const [error, setError] = useState('');
  const [captured, setCaptured] = useState<{ file: File; url: string } | null>(null);

  useEffect(() => {
    if (!open) return undefined;
    let disposed = false;
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
      stopCamera(streamRef);
    };
  }, [facingMode, open]);

  useEffect(() => () => { if (captured) URL.revokeObjectURL(captured.url); }, [captured]);

  if (!open) return null;

  function close() {
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
    context.drawImage(video, 0, 0, canvas.width, canvas.height);
    canvas.toBlob((blob) => {
      if (!blob) return;
      const file = new File([blob], `hien-truong-${new Date().toISOString().replace(/[:.]/g, '-')}.jpg`, { type: 'image/jpeg' });
      setCaptured((current) => { if (current) URL.revokeObjectURL(current.url); return { file, url: URL.createObjectURL(file) }; });
    }, 'image/jpeg', 0.9);
  }

  return <div className="fixed inset-0 z-[70] grid place-items-center p-4">
    <button type="button" className="absolute inset-0 bg-ink/60" aria-label="Đóng camera" onClick={close} />
    <section role="dialog" aria-modal="true" aria-labelledby="camera-title" className="relative flex max-h-[calc(100vh-2rem)] w-full max-w-3xl flex-col overflow-hidden rounded-lg border border-line bg-white shadow-2xl">
      <div className="flex items-start justify-between gap-4 border-b border-line px-5 py-4"><div><h2 id="camera-title" className="text-lg font-semibold text-ink">Chụp ảnh hiện trường</h2><p className="mt-1 text-sm text-slate-600">Kiểm tra khung hình trước khi dùng làm minh chứng.</p></div><button type="button" className="rounded-md p-2 text-slate-500 hover:bg-slate-100" onClick={close} aria-label="Đóng"><X className="h-5 w-5" /></button></div>
      <div className="min-h-0 overflow-y-auto bg-slate-950 p-4">
        <div className="relative mx-auto aspect-video max-h-[60vh] overflow-hidden rounded-lg bg-black">
          <video ref={videoRef} playsInline muted className={`h-full w-full object-contain ${captured ? 'hidden' : ''}`} />
          {captured ? <img src={captured.url} alt="Ảnh vừa chụp" className="h-full w-full object-contain" /> : null}
          {status === 'loading' && !captured ? <div className="absolute inset-0 grid place-items-center text-sm text-white"><span className="inline-flex items-center gap-2"><Loader2 className="h-5 w-5 animate-spin" /> Đang mở camera...</span></div> : null}
          {status === 'error' && !captured ? <div className="absolute inset-0 grid place-items-center p-6 text-center"><div><CameraIcon className="mx-auto h-9 w-9 text-slate-400" /><p className="mt-3 text-sm font-semibold text-white">Không dùng được camera trực tiếp</p><p className="mt-1 max-w-lg text-sm text-slate-300">{error}</p></div></div> : null}
        </div>
      </div>
      <div className="flex flex-col-reverse gap-2 border-t border-line bg-white px-5 py-3 sm:flex-row sm:items-center sm:justify-between">
        <button type="button" className="inline-flex items-center justify-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-50" onClick={onFallback}><Camera className="h-4 w-4" /> Dùng camera hệ thống</button>
        <div className="flex flex-wrap justify-end gap-2">
          {!captured ? <button type="button" disabled={status !== 'ready'} className="inline-flex items-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-50 disabled:opacity-50" onClick={() => setFacingMode((current) => current === 'environment' ? 'user' : 'environment')}><SwitchCamera className="h-4 w-4" /> Đổi camera</button> : null}
          {!captured ? <button type="button" disabled={status !== 'ready'} className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-4 py-2 text-sm font-semibold text-white hover:bg-blue-700 disabled:opacity-50" onClick={capture}><Camera className="h-4 w-4" /> Chụp ảnh</button> : null}
          {captured ? <button type="button" className="inline-flex items-center gap-2 rounded-md border border-line px-3 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-50" onClick={() => setCaptured((current) => { if (current) URL.revokeObjectURL(current.url); return null; })}><RefreshCw className="h-4 w-4" /> Chụp lại</button> : null}
          {captured ? <button type="button" className="inline-flex items-center gap-2 rounded-md bg-blue-600 px-4 py-2 text-sm font-semibold text-white hover:bg-blue-700" onClick={() => onUse(captured.file)}><Upload className="h-4 w-4" /> Dùng ảnh</button> : null}
        </div>
      </div>
    </section>
  </div>;
}

function evidenceTypeForFile(file: File): ConstructionEvidenceUploadDraft['evidenceType'] | null {
  if (file.type === 'application/pdf' || file.name.toLocaleLowerCase().endsWith('.pdf')) return 'document';
  if (['image/png', 'image/jpeg', 'image/webp'].includes(file.type)) return 'photo';
  return null;
}

function stopCamera(ref: React.MutableRefObject<MediaStream | null>) {
  ref.current?.getTracks().forEach((track) => track.stop());
  ref.current = null;
}

function newDraftID() {
  return typeof crypto !== 'undefined' && 'randomUUID' in crypto ? crypto.randomUUID() : `evidence-${Date.now()}-${Math.random().toString(16).slice(2)}`;
}
