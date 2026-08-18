import { useEffect, useState, type ReactNode } from 'react';
import { FileText, Image as ImageIcon, Loader2, RefreshCw, X } from 'lucide-react';

export type EvidenceViewerDialogProps = {
  open: boolean;
  name: string;
  previewUrl?: string;
  mediaType: 'photo' | 'document';
  loading?: boolean;
  error?: string;
  caption?: string;
  description?: string;
  metadata?: string[];
  details?: ReactNode;
  footer?: ReactNode;
  onRetry?: () => void;
  onClose: () => void;
};

export function EvidenceViewerDialog({
  open,
  name,
  previewUrl,
  mediaType,
  loading = false,
  error,
  caption,
  description = 'Xem minh chứng ngay trong ứng dụng.',
  metadata = [],
  details,
  footer,
  onRetry,
  onClose,
}: EvidenceViewerDialogProps) {
  const [decodeFailed, setDecodeFailed] = useState(false);

  useEffect(() => {
    setDecodeFailed(false);
  }, [open, previewUrl]);

  useEffect(() => {
    if (!open) return undefined;
    const previousOverflow = document.body.style.overflow;
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key !== 'Escape') return;
      event.preventDefault();
      event.stopImmediatePropagation();
      onClose();
    };
    document.body.style.overflow = 'hidden';
    document.addEventListener('keydown', onKeyDown, true);
    return () => {
      document.body.style.overflow = previousOverflow;
      document.removeEventListener('keydown', onKeyDown, true);
    };
  }, [open, onClose]);

  if (!open) return null;

  const visibleError = error || (decodeFailed ? 'Không thể hiển thị định dạng ảnh này.' : '');
  return (
    <div className="fixed inset-0 z-[80] grid place-items-center p-0 sm:p-4">
      <button type="button" className="absolute inset-0 bg-ink/60" aria-label="Đóng trình xem minh chứng" onClick={onClose} />
      <section role="dialog" aria-modal="true" aria-labelledby="evidence-viewer-title" className="relative flex h-full max-h-none w-full max-w-[min(1120px,calc(100vw-2rem))] flex-col overflow-hidden bg-white shadow-2xl sm:h-auto sm:max-h-[calc(100vh-2rem)] sm:rounded-lg sm:border sm:border-line">
        <header className="flex items-start justify-between gap-4 border-b border-line px-4 py-3 sm:px-5 sm:py-4">
          <div className="min-w-0">
            <h2 id="evidence-viewer-title" className="truncate text-lg font-semibold text-ink">Xem minh chứng</h2>
            <p className="mt-1 truncate text-sm font-medium text-slate-700">{name}</p>
            <p className="mt-0.5 text-xs leading-5 text-slate-500">{description}</p>
          </div>
          <button type="button" className="rounded-md p-2 text-slate-500 hover:bg-slate-100" onClick={onClose} aria-label="Đóng"><X className="h-5 w-5" /></button>
        </header>

        <div className="min-h-0 flex-1 overflow-y-auto p-4 sm:p-5">
          <div className={details ? 'grid gap-4 xl:grid-cols-[minmax(0,1fr)_360px]' : 'mx-auto max-w-5xl'}>
            <div className="min-w-0">
              <div className="flex min-h-72 items-center justify-center overflow-hidden rounded-md border border-line bg-slate-950">
                {loading ? <span className="inline-flex items-center gap-2 text-sm font-medium text-white"><Loader2 className="h-5 w-5 animate-spin" /> Đang tải ảnh minh chứng…</span> : null}
                {!loading && visibleError ? <div className="px-6 py-10 text-center"><ImageIcon className="mx-auto h-12 w-12 text-rose-300" /><p className="mt-3 text-sm font-semibold text-white">{visibleError}</p>{onRetry ? <button type="button" className="mt-4 inline-flex items-center gap-2 rounded-md border border-white/30 bg-white/10 px-3 py-2 text-sm font-semibold text-white hover:bg-white/20" onClick={onRetry}><RefreshCw className="h-4 w-4" /> Thử lại</button> : null}</div> : null}
                {!loading && !visibleError && mediaType === 'photo' && previewUrl ? <img className="max-h-[calc(100vh-15rem)] w-full object-contain" src={previewUrl} alt={name} onError={() => setDecodeFailed(true)} /> : null}
                {!loading && !visibleError && mediaType === 'photo' && !previewUrl ? <div className="px-6 py-10 text-center text-slate-300"><ImageIcon className="mx-auto h-12 w-12" /><p className="mt-3 text-sm font-semibold">Chưa có ảnh xem trước</p></div> : null}
                {!loading && !visibleError && mediaType === 'document' ? <div className="px-6 py-10 text-center text-slate-300"><FileText className="mx-auto h-12 w-12" /><p className="mt-3 text-sm font-semibold">Tài liệu được mở bằng hành động Mở tệp</p></div> : null}
              </div>
              {metadata.length ? <div className="mt-3 flex flex-wrap gap-2">{metadata.filter(Boolean).map((item) => <span key={item} className="rounded-full border border-line bg-slate-50 px-2.5 py-1 text-xs font-medium text-slate-600">{item}</span>)}</div> : null}
              {caption ? <p className="mt-3 whitespace-pre-wrap break-words rounded-md border border-line bg-slate-50 p-3 text-sm leading-6 text-slate-700">{caption}</p> : null}
            </div>
            {details ? <aside className="min-w-0">{details}</aside> : null}
          </div>
        </div>

        <footer className="border-t border-line bg-slate-50 px-4 py-3 sm:px-5">
          {footer ?? <div className="flex justify-end"><button type="button" className="rounded-md border border-line bg-white px-4 py-2 text-sm font-semibold text-slate-700 hover:bg-slate-100" onClick={onClose}>Đóng</button></div>}
        </footer>
      </section>
    </div>
  );
}
