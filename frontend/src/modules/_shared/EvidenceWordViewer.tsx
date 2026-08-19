import { useEffect, useRef, useState, type CSSProperties, type ReactNode } from 'react';
import { FileText, Loader2, Maximize2, RefreshCw, ZoomIn, ZoomOut } from 'lucide-react';
import { inspectOfficePreviewArchive } from './officePreviewArchive';

const MAX_WORD_PREVIEW_BYTES = 10 * 1024 * 1024;
const MIN_ZOOM = 0.4;
const MAX_ZOOM = 1.6;
const ZOOM_STEP = 0.1;

type EvidenceWordViewerProps = {
  name: string;
  blob?: Blob;
  loading?: boolean;
  error?: string;
  onRetry?: () => void;
};

export function EvidenceWordViewer({
  name,
  blob,
  loading = false,
  error,
  onRetry,
}: EvidenceWordViewerProps) {
  const viewportRef = useRef<HTMLDivElement>(null);
  const documentRef = useRef<HTMLDivElement>(null);
  const [retryKey, setRetryKey] = useState(0);
  const [zoom, setZoom] = useState(1);
  const [renderState, setRenderState] = useState<{ status: 'idle' | 'rendering' | 'ready' | 'error'; error?: string }>({ status: 'idle' });

  useEffect(() => {
    const container = documentRef.current;
    if (!container) return undefined;
    container.replaceChildren();
    setZoom(1);
    if (!blob || loading || error) {
      setRenderState({ status: 'idle' });
      return undefined;
    }
    if (blob.size > MAX_WORD_PREVIEW_BYTES) {
      setRenderState({ status: 'error', error: 'Tệp vượt quá giới hạn 10 MB của trình đọc Word.' });
      return undefined;
    }

    const sourceBlob = blob;
    const documentContainer = container;
    let disposed = false;
    setRenderState({ status: 'rendering' });
    async function renderWord() {
      const buffer = await sourceBlob.arrayBuffer();
      inspectOfficePreviewArchive(buffer);
      if (disposed) return;
      const { renderAsync } = await import('docx-preview');
      if (disposed) return;
      await renderAsync(buffer, documentContainer, documentContainer, {
        inWrapper: true,
        breakPages: true,
        ignoreLastRenderedPageBreak: false,
        renderHeaders: true,
        renderFooters: true,
        renderFootnotes: true,
        renderEndnotes: true,
        renderChanges: false,
        renderComments: false,
        renderAltChunks: false,
        useBase64URL: true,
      });
      if (disposed) return;
      sanitizeRenderedDocument(documentContainer);
      setRenderState({ status: 'ready' });
    }

    void renderWord().catch((reason) => {
      if (disposed) return;
      documentContainer.replaceChildren();
      const message = reason instanceof Error ? reason.message : 'Không phân tích được tài liệu.';
      setRenderState({ status: 'error', error: `Không thể đọc Word. ${message}` });
    });

    return () => {
      disposed = true;
      documentContainer.replaceChildren();
    };
  }, [blob, error, loading, retryKey]);

  const visibleError = error || renderState.error;
  const busy = loading || renderState.status === 'rendering';
  const retry = () => {
    if (error && onRetry) onRetry();
    else setRetryKey((current) => current + 1);
  };
  const zoomStyle = { zoom } as CSSProperties;
  const fitWidth = () => {
    const viewport = viewportRef.current;
    const page = documentRef.current?.querySelector<HTMLElement>('section.docx');
    if (!viewport || !page) return;
    const unscaledPageWidth = page.getBoundingClientRect().width / zoom;
    if (unscaledPageWidth <= 0) return;
    setZoom(clampZoom((viewport.clientWidth - 32) / unscaledPageWidth));
    viewport.scrollTo({ top: 0, left: 0 });
  };

  return (
    <div className="overflow-hidden rounded-lg border border-line bg-slate-200">
      <div className="flex min-h-11 items-center justify-between gap-3 border-b border-line bg-white px-3 py-1.5">
        <p className="min-w-0 truncate text-xs font-medium text-slate-500">Bản xem trước Word</p>
        <div className="flex shrink-0 items-center gap-1" role="group" aria-label="Thu phóng tài liệu Word">
          <button type="button" className="rounded-md border border-line bg-white p-1.5 text-slate-600 hover:bg-slate-50 disabled:opacity-40" aria-label="Thu nhỏ" disabled={zoom <= MIN_ZOOM} onClick={() => setZoom((current) => clampZoom(current - ZOOM_STEP))}><ZoomOut className="h-4 w-4" /></button>
          <span className="min-w-12 text-center text-xs font-semibold tabular-nums text-slate-600">{Math.round(zoom * 100)}%</span>
          <button type="button" className="rounded-md border border-line bg-white p-1.5 text-slate-600 hover:bg-slate-50 disabled:opacity-40" aria-label="Phóng to" disabled={zoom >= MAX_ZOOM} onClick={() => setZoom((current) => clampZoom(current + ZOOM_STEP))}><ZoomIn className="h-4 w-4" /></button>
          <button type="button" className="ml-1 inline-flex items-center gap-1 rounded-md border border-line bg-white px-2 py-1.5 text-xs font-semibold text-slate-600 hover:bg-slate-50 disabled:opacity-40" disabled={renderState.status !== 'ready'} onClick={fitWidth}><Maximize2 className="h-3.5 w-3.5" /> Vừa chiều rộng</button>
        </div>
      </div>
      <div ref={viewportRef} className="relative h-[60vh] min-h-80 overflow-auto bg-slate-200 sm:h-[65vh] xl:h-[calc(100vh-13rem)] xl:min-h-[36rem]">
        {busy ? <WordState icon={<Loader2 className="h-8 w-8 animate-spin text-blue-700" />} text={loading ? 'Đang tải Word…' : 'Đang phân tích tài liệu Word…'} /> : null}
        {!busy && visibleError ? <WordState icon={<FileText className="h-12 w-12 text-rose-400" />} text={visibleError} action={<button type="button" className="inline-flex items-center gap-2 rounded-md border border-rose-200 bg-white px-3 py-2 text-sm font-semibold text-rose-700 hover:bg-rose-50" onClick={retry}><RefreshCw className="h-4 w-4" /> Thử lại</button>} /> : null}
        {!busy && !visibleError && renderState.status === 'idle' ? <WordState icon={<FileText className="h-12 w-12 text-slate-400" />} text="Hồ sơ chưa có tệp Word để đọc." /> : null}
        <div
          ref={documentRef}
          aria-label={`Nội dung Word ${name}`}
          className={renderState.status === 'ready' && !visibleError ? 'min-w-fit origin-top p-4 sm:p-6' : 'hidden'}
          style={zoomStyle}
        />
      </div>
    </div>
  );
}

function WordState({ icon, text, action }: { icon: ReactNode; text: string; action?: ReactNode }) {
  return (
    <div className="absolute inset-0 z-10 flex flex-col items-center justify-center px-6 py-10 text-center text-slate-600">
      {icon}
      <p className="mt-3 max-w-md text-sm font-semibold">{text}</p>
      {action ? <div className="mt-4">{action}</div> : null}
    </div>
  );
}

function clampZoom(value: number) {
  return Math.min(MAX_ZOOM, Math.max(MIN_ZOOM, Math.round(value * 10) / 10));
}

function sanitizeRenderedDocument(container: HTMLElement) {
  container.querySelectorAll('script, iframe, object, embed, form').forEach((element) => element.remove());
  container.querySelectorAll<HTMLElement>('*').forEach((element) => {
    Array.from(element.attributes).forEach((attribute) => {
      if (attribute.name.toLowerCase().startsWith('on')) element.removeAttribute(attribute.name);
    });
  });
  container.querySelectorAll<HTMLAnchorElement>('a[href]').forEach((anchor) => {
    const rawHref = anchor.getAttribute('href')?.trim() ?? '';
    if (!rawHref || rawHref.startsWith('#')) return;
    try {
      const url = new URL(rawHref, window.location.href);
      if (!['http:', 'https:', 'mailto:', 'tel:'].includes(url.protocol)) {
        anchor.removeAttribute('href');
        return;
      }
      anchor.target = '_blank';
      anchor.rel = 'noopener noreferrer';
    } catch {
      anchor.removeAttribute('href');
    }
  });
}
