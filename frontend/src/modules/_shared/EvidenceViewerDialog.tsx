import { useEffect, useRef, useState, type ReactNode } from 'react';
import { FileText, Image as ImageIcon, Info, Loader2, RefreshCw, X } from 'lucide-react';
import { EvidencePdfViewer } from './EvidencePdfViewer';
import { EvidenceSpreadsheetViewer } from './EvidenceSpreadsheetViewer';
import { EvidenceWordViewer } from './EvidenceWordViewer';
import type { EvidenceMediaType } from './evidenceMedia';

export type EvidenceViewerDialogProps = {
  open: boolean;
  name: string;
  title?: string;
  previewUrl?: string;
  fileBlob?: Blob;
  mediaType: EvidenceMediaType;
  loading?: boolean;
  error?: string;
  caption?: string;
  description?: string;
  metadata?: string[];
  headerStatus?: ReactNode;
  mediaAction?: ReactNode;
  documentUnavailableText?: string;
  details?: ReactNode;
  footer?: ReactNode;
  onRetry?: () => void;
  onClose: () => void;
};

export function EvidenceViewerDialog({
  open,
  name,
  title = 'Xem minh chứng',
  previewUrl,
  fileBlob,
  mediaType,
  loading = false,
  error,
  caption,
  description,
  metadata = [],
  headerStatus,
  mediaAction,
  documentUnavailableText,
  details,
  footer,
  onRetry,
  onClose,
}: EvidenceViewerDialogProps) {
  const [decodeFailed, setDecodeFailed] = useState(false);
  const dialogRef = useRef<HTMLElement | null>(null);

  useEffect(() => {
    setDecodeFailed(false);
  }, [open, previewUrl]);

  useEffect(() => {
    if (!open) return undefined;
    const previousOverflow = document.body.style.overflow;
    const previousFocus = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        event.preventDefault();
        event.stopImmediatePropagation();
        onClose();
        return;
      }
      if (event.key !== 'Tab' || !dialogRef.current) return;
      const focusable = Array.from(dialogRef.current.querySelectorAll<HTMLElement>('a[href], button:not([disabled]), textarea:not([disabled]), input:not([disabled]), select:not([disabled]), [tabindex]:not([tabindex="-1"])'));
      if (!focusable.length) {
        event.preventDefault();
        dialogRef.current.focus();
        return;
      }
      const first = focusable[0];
      const last = focusable[focusable.length - 1];
      if (event.shiftKey && document.activeElement === first) {
        event.preventDefault();
        last.focus();
      } else if (!event.shiftKey && document.activeElement === last) {
        event.preventDefault();
        first.focus();
      }
    };
    document.body.style.overflow = 'hidden';
    document.addEventListener('keydown', onKeyDown, true);
    window.requestAnimationFrame(() => dialogRef.current?.querySelector<HTMLElement>('[data-viewer-autofocus]')?.focus());
    return () => {
      document.body.style.overflow = previousOverflow;
      document.removeEventListener('keydown', onKeyDown, true);
      window.requestAnimationFrame(() => previousFocus?.isConnected && previousFocus.focus());
    };
  }, [open, onClose]);

  if (!open) return null;

  const visibleError = error || (decodeFailed ? 'Không thể hiển thị định dạng ảnh này.' : '');
  return (
    <div className="fixed inset-0 z-[80] grid place-items-center p-0 sm:p-4">
      <button type="button" className="absolute inset-0 bg-ink/60" aria-label={title === 'Xem minh chứng' ? 'Đóng trình xem minh chứng' : 'Đóng trình xem tệp'} onClick={onClose} />
      <section ref={dialogRef} tabIndex={-1} role="dialog" aria-modal="true" aria-labelledby="evidence-viewer-title" className="relative flex h-full max-h-none w-full max-w-[min(1180px,calc(100vw-2rem))] flex-col overflow-hidden bg-white shadow-2xl outline-none sm:h-auto sm:max-h-[calc(100vh-2rem)] sm:rounded-xl sm:border sm:border-line">
        <header className="flex items-start justify-between gap-4 border-b border-line px-4 py-3 sm:px-5 sm:py-4">
          <div className="min-w-0 flex-1">
            <h2 id="evidence-viewer-title" className="text-lg font-semibold text-ink">{title}</h2>
            <p className="mt-1 line-clamp-2 break-words text-sm font-semibold leading-5 text-slate-700">{name}</p>
            {metadata.length ? <div className="mt-2 flex flex-wrap gap-1.5">{metadata.filter(Boolean).map((item) => <span key={item} className="rounded-full border border-line bg-slate-50 px-2.5 py-1 text-xs font-medium text-slate-600">{item}</span>)}</div> : null}
            {description ? <p className="mt-1.5 text-xs leading-5 text-slate-500">{description}</p> : null}
          </div>
          <div className="flex shrink-0 items-center gap-2">
            {mediaAction}
            {headerStatus}
            <button type="button" data-viewer-autofocus className="rounded-md p-2 text-slate-500 hover:bg-slate-100" onClick={onClose} aria-label="Đóng"><X className="h-5 w-5" /></button>
          </div>
        </header>

        <div className="min-h-0 flex-1 overflow-y-auto p-4 sm:p-5">
          <div className={details ? 'grid gap-5 xl:grid-cols-[minmax(0,1.65fr)_minmax(320px,1fr)]' : 'mx-auto max-w-5xl'}>
            <div className="min-w-0">
              {mediaType === 'photo' ? (
                <div className="flex min-h-72 items-center justify-center overflow-hidden rounded-lg border border-line bg-slate-950">
                  {loading ? <span className="inline-flex items-center gap-2 text-sm font-medium text-white"><Loader2 className="h-5 w-5 animate-spin" /> Đang tải ảnh minh chứng…</span> : null}
                  {!loading && visibleError ? <div className="px-6 py-10 text-center"><ImageIcon className="mx-auto h-12 w-12 text-rose-300" /><p className="mt-3 text-sm font-semibold text-white">{visibleError}</p>{onRetry ? <button type="button" className="mt-4 inline-flex items-center gap-2 rounded-md border border-white/30 bg-white/10 px-3 py-2 text-sm font-semibold text-white hover:bg-white/20" onClick={onRetry}><RefreshCw className="h-4 w-4" /> Thử lại</button> : null}</div> : null}
                  {!loading && !visibleError && previewUrl ? <img className="max-h-[calc(100vh-15rem)] w-full object-contain" src={previewUrl} alt={name} onError={() => setDecodeFailed(true)} /> : null}
                  {!loading && !visibleError && !previewUrl ? <div className="px-6 py-10 text-center text-slate-300"><ImageIcon className="mx-auto h-12 w-12" /><p className="mt-3 text-sm font-semibold">Chưa có ảnh xem trước</p></div> : null}
                </div>
              ) : mediaType === 'pdf' ? (
                <EvidencePdfViewer
                  name={name}
                  previewUrl={previewUrl}
                  loading={loading}
                  error={error}
                  onRetry={onRetry}
                />
              ) : mediaType === 'word' ? (
                <EvidenceWordViewer
                  name={name}
                  blob={fileBlob}
                  loading={loading}
                  error={error}
                  onRetry={onRetry}
                />
              ) : mediaType === 'spreadsheet' ? (
                <EvidenceSpreadsheetViewer
                  name={name}
                  blob={fileBlob}
                  loading={loading}
                  error={error}
                  onRetry={onRetry}
                />
              ) : (
                <div className="flex min-h-64 flex-col items-center justify-center rounded-lg border border-line bg-slate-50 px-6 py-8 text-center sm:min-h-[22rem]">
                  {loading ? <span className="mb-4 inline-flex items-center gap-2 text-sm font-medium text-slate-600"><Loader2 className="h-5 w-5 animate-spin" /> Đang tải minh chứng…</span> : null}
                  <span className="grid h-20 w-20 place-items-center rounded-2xl border border-slate-200 bg-white text-slate-500 shadow-sm">
                    <FileText className="h-10 w-10" />
                  </span>
                  <p className="mt-5 max-w-xl break-words text-base font-semibold leading-6 text-ink">{name}</p>
                  {metadata.length ? <p className="mt-2 text-sm text-slate-500">{metadata.filter(Boolean).join(' · ')}</p> : null}
                  {!loading && visibleError ? <div className="mt-4 text-center"><p className="text-sm font-semibold text-rose-700">{visibleError}</p>{onRetry ? <button type="button" className="mt-3 inline-flex items-center gap-2 rounded-md border border-line bg-white px-3 py-2 text-sm font-semibold text-blue-700 hover:bg-blue-50" onClick={onRetry}><RefreshCw className="h-4 w-4" /> Thử lại</button> : null}</div> : null}
                  {!loading && !visibleError && documentUnavailableText ? <p className="mt-4 max-w-xl text-sm font-medium leading-6 text-slate-500">{documentUnavailableText}</p> : null}
                  {!loading && !visibleError && mediaAction ? <div className="mt-5">{mediaAction}</div> : !loading && !visibleError && !documentUnavailableText ? <p className="mt-4 text-sm font-medium text-slate-500">Chưa có tệp có thể mở.</p> : null}
                </div>
              )}
              {caption ? <div className="mt-3 flex items-start gap-2 rounded-md border border-blue-100 bg-blue-50/70 p-3 text-sm leading-6 text-slate-700"><Info className="mt-1 h-4 w-4 shrink-0 text-blue-700" /><p className="whitespace-pre-wrap break-words">{caption}</p></div> : null}
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
