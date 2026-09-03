import { useEffect, useRef, useState } from 'react';
import { ImageOff, Loader2, Maximize2, Minus, Plus, RefreshCw } from 'lucide-react';
import clsx from 'clsx';
import { downloadBlob } from '../../lib/api/client';
import { useAuth } from '../../lib/auth/AuthContext';
import type { WorkflowFormSignature } from '../../lib/types/domain';
import { startProtectedImagePreview, type ProtectedImagePreviewState } from '../../lib/utils/protectedImagePreview';

type SignaturePreviewState = ProtectedImagePreviewState | { status: 'missing' };
type SignaturePreview = { state: SignaturePreviewState; retry: () => void };

export function useWorkflowSignaturePreview(signature: WorkflowFormSignature): SignaturePreview {
  const { user } = useAuth();
  const [attempt, setAttempt] = useState(0);
  const [result, setResult] = useState<{ key: string; state: ProtectedImagePreviewState }>();
  const path = signature.visual_storage_key ? `/workflow-form-signatures/${signature.id}/visual` : '';
  const key = JSON.stringify([user?.id, signature.id, signature.visual_storage_key, signature.visual_checksum_sha256, attempt]);
  useEffect(() => {
    if (!path || !user?.id) return;
    return startProtectedImagePreview(
      () => downloadBlob(path),
      (state) => setResult({ key, state }),
    );
  }, [key, path, user?.id]);

  const state: SignaturePreviewState = !path ? { status: 'missing' }
    : !user?.id ? { status: 'error', message: 'Bạn không có quyền xem dấu ký này.', retryable: false }
      : result?.key === key ? result.state : { status: 'loading' };
  return { state, retry: () => setAttempt((value) => value + 1) };
}

export function WorkflowSignatureVisual({
  preview, signerName, variant = 'detail', onEnlarge,
}: {
  preview: SignaturePreview;
  signerName: string;
  variant?: 'thumbnail' | 'detail' | 'full';
  onEnlarge?: () => void;
}) {
  const { state, retry } = preview;
  const source = state.status === 'ready' ? state.url : '';
  const [failedSource, setFailedSource] = useState('');
  const [loadedSource, setLoadedSource] = useState('');
  const [naturalSize, setNaturalSize] = useState({ width: 0, height: 0 });
  const [viewportSize, setViewportSize] = useState({ width: 0, height: 0 });
  const [zoom, setZoom] = useState<number | 'fit'>('fit');
  const viewportRef = useRef<HTMLDivElement>(null);
  const full = variant === 'full';
  const thumbnail = variant === 'thumbnail';
  const decoded = Boolean(source && loadedSource === source);
  const decodeFailed = Boolean(source && failedSource === source);
  const ready = state.status === 'ready' && decoded && !decodeFailed;

  useEffect(() => {
    setZoom('fit');
  }, [source, variant]);

  useEffect(() => {
    const viewport = viewportRef.current;
    if (!viewport || !full) return;
    const measure = () => setViewportSize({ width: Math.max(1, viewport.clientWidth - 32), height: Math.max(1, viewport.clientHeight - 32) });
    measure();
    const observer = new ResizeObserver(measure);
    observer.observe(viewport);
    return () => observer.disconnect();
  }, [full]);

  const fitScale = naturalSize.width && naturalSize.height && viewportSize.width
    ? Math.min(viewportSize.width / naturalSize.width, viewportSize.height / naturalSize.height, 1)
    : 1;
  const scale = zoom === 'fit' ? fitScale : zoom;
  const errorMessage = state.status === 'error' ? state.message : decodeFailed ? 'Không hiển thị được ảnh dấu ký.' : '';
  const canRetry = state.status === 'error' ? state.retryable : decodeFailed;
  const loading = state.status === 'loading' || (state.status === 'ready' && !decoded && !decodeFailed);

  return (
    <div className="min-w-0">
      <div
        ref={viewportRef}
        className={clsx(
          'relative rounded-md border border-line bg-white',
          full ? 'h-[min(58dvh,520px)] min-h-60 overflow-auto' : 'overflow-hidden',
          thumbnail ? 'h-24' : !full && 'h-52 sm:h-60',
        )}
        aria-busy={loading}
      >
        {source && !decodeFailed ? (
          <div className={clsx(full ? 'flex min-h-full min-w-full p-4' : 'flex h-full w-full items-center justify-center p-3', !decoded && 'invisible')}>
            <img
              key={source}
              src={source}
              alt={`Dấu ký của ${signerName || 'người ký'}`}
              draggable={false}
              className={full ? 'm-auto block max-w-none shrink-0' : 'max-h-full max-w-full object-contain'}
              style={full && naturalSize.width ? { width: naturalSize.width * scale, height: naturalSize.height * scale } : undefined}
              onLoad={(event) => {
                const image = event.currentTarget;
                setNaturalSize({ width: image.naturalWidth, height: image.naturalHeight });
                setLoadedSource(source);
              }}
              onError={() => setFailedSource(source)}
            />
          </div>
        ) : null}
        {loading || errorMessage || state.status === 'missing' ? (
          <div className="absolute inset-0 flex flex-col items-center justify-center gap-2 px-3 py-2 text-center text-xs leading-5 text-slate-500" role="status">
            {loading ? <><Loader2 className="h-4 w-4 animate-spin" /><span>Đang tải dấu ký…</span></> : <>
              {!thumbnail ? <ImageOff className="h-5 w-5 text-slate-400" /> : null}
              <span className={errorMessage ? 'text-rose-700' : ''}>{errorMessage || 'Bản ghi này không có ảnh dấu ký.'}</span>
              {canRetry ? <button type="button" className="inline-flex items-center gap-1 rounded px-2 py-1 font-semibold text-blue-700 hover:bg-blue-50 focus-visible:outline focus-visible:outline-2 focus-visible:outline-blue-500" onClick={retry}><RefreshCw className="h-3.5 w-3.5" />Thử lại</button> : null}
            </>}
          </div>
        ) : null}
      </div>
      {full ? (
        <div className="mt-3 flex flex-wrap items-center justify-center gap-2" role="group" aria-label="Thu phóng dấu ký">
          <button type="button" aria-label="Thu nhỏ dấu ký" disabled={!ready || scale <= 0.1} onClick={() => setZoom(Math.max(0.1, scale - 0.25))} className="rounded-md border border-line p-2 text-slate-700 hover:bg-slate-50 disabled:opacity-40"><Minus className="h-4 w-4" /></button>
          <output className="min-w-14 text-center text-sm tabular-nums text-slate-600" aria-live="polite">{ready ? `${Math.round(scale * 100)}%` : '—'}</output>
          <button type="button" aria-label="Phóng to dấu ký" disabled={!ready || scale >= 4} onClick={() => setZoom(Math.min(4, scale + 0.25))} className="rounded-md border border-line p-2 text-slate-700 hover:bg-slate-50 disabled:opacity-40"><Plus className="h-4 w-4" /></button>
          <button type="button" disabled={!ready} className="rounded-md border border-line px-3 py-2 text-xs font-semibold text-slate-700 hover:bg-slate-50 disabled:opacity-40" onClick={() => { setZoom('fit'); viewportRef.current?.scrollTo({ top: 0, left: 0 }); }}>Vừa khung</button>
        </div>
      ) : onEnlarge && state.status !== 'missing' ? (
        <div className={clsx('mt-1 flex', thumbnail ? 'justify-center' : 'justify-end')}>
          <button type="button" data-signature-mode-focus aria-label={`Phóng to dấu ký của ${signerName || 'người ký'}`} disabled={!ready} onClick={onEnlarge} className="inline-flex items-center gap-1.5 rounded px-2 py-1.5 text-xs font-semibold text-blue-700 hover:bg-blue-50 disabled:cursor-not-allowed disabled:text-slate-400"><Maximize2 className="h-3.5 w-3.5" />Phóng to</button>
        </div>
      ) : null}
    </div>
  );
}
