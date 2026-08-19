import { useEffect, useMemo, useRef, useState } from 'react';
import { useVirtualizer } from '@tanstack/react-virtual';
import { AlertTriangle, FileSpreadsheet, Loader2, RefreshCw } from 'lucide-react';
import {
  MAX_SPREADSHEET_PREVIEW_BYTES,
  MAX_SPREADSHEET_PREVIEW_COLUMNS,
  MAX_SPREADSHEET_PREVIEW_ROWS,
  spreadsheetColumnLabel,
  type SpreadsheetSheetPreview,
  type SpreadsheetSheetSummary,
  type SpreadsheetWorkerRequest,
  type SpreadsheetWorkerResponse,
} from './evidenceSpreadsheetPreview';
import { inspectOfficePreviewArchive } from './officePreviewArchive';

type EvidenceSpreadsheetViewerProps = {
  name: string;
  blob?: Blob;
  loading?: boolean;
  error?: string;
  onRetry?: () => void;
};

type WorkbookState = {
  status: 'idle' | 'loading-workbook' | 'loading-sheet' | 'ready' | 'error';
  sheets: SpreadsheetSheetSummary[];
  sheetsTruncated: boolean;
  activeSheet?: string;
  preview?: SpreadsheetSheetPreview;
  error?: string;
};

const initialState: WorkbookState = {
  status: 'idle',
  sheets: [],
  sheetsTruncated: false,
};

export function EvidenceSpreadsheetViewer({
  name,
  blob,
  loading = false,
  error,
  onRetry,
}: EvidenceSpreadsheetViewerProps) {
  const workerRef = useRef<Worker | null>(null);
  const requestIdRef = useRef(0);
  const scrollRef = useRef<HTMLDivElement>(null);
  const [retryKey, setRetryKey] = useState(0);
  const [state, setState] = useState<WorkbookState>(initialState);

  useEffect(() => {
    if (!blob || loading || error) {
      setState(initialState);
      return undefined;
    }
    if (blob.size > MAX_SPREADSHEET_PREVIEW_BYTES) {
      setState({
        ...initialState,
        status: 'error',
        error: 'Tệp vượt quá giới hạn 10 MB của trình đọc Excel.',
      });
      return undefined;
    }

    const worker = new Worker(new URL('./evidenceSpreadsheet.worker.ts', import.meta.url), { type: 'module' });
    workerRef.current = worker;
    let disposed = false;
    setState({ ...initialState, status: 'loading-workbook' });

    const requestSheet = (sheetName: string) => {
      const requestId = requestIdRef.current + 1;
      requestIdRef.current = requestId;
      setState((current) => ({
        ...current,
        status: 'loading-sheet',
        activeSheet: sheetName,
        preview: undefined,
        error: undefined,
      }));
      worker.postMessage({ type: 'sheet', sheetName, requestId } satisfies SpreadsheetWorkerRequest);
    };

    worker.onmessage = (event: MessageEvent<SpreadsheetWorkerResponse>) => {
      if (disposed) return;
      const message = event.data;
      if (message.type === 'loaded') {
        const firstSheet = message.sheets[0]?.name;
        setState({
          status: firstSheet ? 'loading-sheet' : 'ready',
          sheets: message.sheets,
          sheetsTruncated: message.sheetsTruncated,
          activeSheet: firstSheet,
        });
        if (firstSheet) requestSheet(firstSheet);
        return;
      }
      if (message.type === 'sheet') {
        if (message.requestId !== requestIdRef.current) return;
        setState((current) => ({
          ...current,
          status: 'ready',
          activeSheet: message.preview.name,
          preview: message.preview,
          error: undefined,
        }));
        scrollRef.current?.scrollTo({ top: 0, left: 0 });
        return;
      }
      if (message.requestId && message.requestId !== requestIdRef.current) return;
      setState((current) => ({ ...current, status: 'error', error: `Không thể đọc Excel. ${message.message}` }));
    };
    worker.onerror = () => {
      if (!disposed) setState((current) => ({ ...current, status: 'error', error: 'Không thể khởi tạo trình đọc Excel.' }));
    };

    void blob.arrayBuffer()
      .then((buffer) => {
        inspectOfficePreviewArchive(buffer);
        if (!disposed) worker.postMessage({ type: 'load', buffer } satisfies SpreadsheetWorkerRequest, [buffer]);
      })
      .catch((reason) => {
        if (disposed) return;
        const message = reason instanceof Error ? reason.message : 'Không đọc được dữ liệu tệp.';
        setState((current) => ({ ...current, status: 'error', error: `Không thể đọc Excel. ${message}` }));
      });

    return () => {
      disposed = true;
      workerRef.current = null;
      worker.terminate();
    };
  }, [blob, error, loading, retryKey]);

  const openSheet = (sheetName: string) => {
    const worker = workerRef.current;
    if (!worker || sheetName === state.activeSheet) return;
    const requestId = requestIdRef.current + 1;
    requestIdRef.current = requestId;
    setState((current) => ({
      ...current,
      status: 'loading-sheet',
      activeSheet: sheetName,
      preview: undefined,
      error: undefined,
    }));
    worker.postMessage({ type: 'sheet', sheetName, requestId } satisfies SpreadsheetWorkerRequest);
  };

  const retry = () => {
    if (error && onRetry) onRetry();
    else setRetryKey((current) => current + 1);
  };
  const visibleError = error || state.error;
  const busy = loading || state.status === 'loading-workbook' || state.status === 'loading-sheet';

  return (
    <div className="overflow-hidden rounded-lg border border-line bg-white">
      <div className="flex min-h-11 items-center gap-1 overflow-x-auto border-b border-line bg-slate-50 px-2 py-1.5" role="tablist" aria-label={`Sheet trong ${name}`}>
        {state.sheets.map((sheet) => (
          <button
            key={sheet.name}
            type="button"
            role="tab"
            aria-selected={sheet.name === state.activeSheet}
            className={sheet.name === state.activeSheet
              ? 'shrink-0 rounded-md border border-blue-300 bg-white px-3 py-1.5 text-xs font-semibold text-blue-700 shadow-sm'
              : 'shrink-0 rounded-md border border-transparent px-3 py-1.5 text-xs font-semibold text-slate-600 hover:border-line hover:bg-white'}
            onClick={() => openSheet(sheet.name)}
          >
            {sheet.name}
          </button>
        ))}
        {state.sheetsTruncated ? <span className="shrink-0 px-2 text-xs font-medium text-amber-700">Chỉ hiển thị 50 sheet đầu</span> : null}
      </div>

      {busy ? <SpreadsheetState icon={<Loader2 className="h-8 w-8 animate-spin text-blue-700" />} text={loading ? 'Đang tải Excel…' : state.status === 'loading-workbook' ? 'Đang phân tích workbook…' : 'Đang đọc sheet…'} /> : null}
      {!busy && visibleError ? (
        <SpreadsheetState
          icon={<FileSpreadsheet className="h-12 w-12 text-rose-400" />}
          text={visibleError}
          action={<button type="button" className="inline-flex items-center gap-2 rounded-md border border-rose-200 bg-white px-3 py-2 text-sm font-semibold text-rose-700 hover:bg-rose-50" onClick={retry}><RefreshCw className="h-4 w-4" /> Thử lại</button>}
        />
      ) : null}
      {!busy && !visibleError && state.status === 'ready' && !state.sheets.length ? <SpreadsheetState icon={<FileSpreadsheet className="h-12 w-12 text-slate-400" />} text="Workbook không có sheet để hiển thị." /> : null}
      {!busy && !visibleError && state.preview ? <SpreadsheetGrid preview={state.preview} scrollRef={scrollRef} /> : null}
      {!busy && !visibleError && state.status === 'idle' ? <SpreadsheetState icon={<FileSpreadsheet className="h-12 w-12 text-slate-400" />} text="Hồ sơ chưa có tệp Excel để đọc." /> : null}
    </div>
  );
}

function SpreadsheetGrid({ preview, scrollRef }: { preview: SpreadsheetSheetPreview; scrollRef: React.RefObject<HTMLDivElement> }) {
  const rowVirtualizer = useVirtualizer({
    count: preview.rows.length,
    getScrollElement: () => scrollRef.current,
    estimateSize: () => 34,
    overscan: 18,
  });
  const columnCount = useMemo(
    () => Math.max(1, preview.rows.reduce((maximum, row) => Math.max(maximum, row.length), 0)),
    [preview.rows],
  );
  const gridStyle = useMemo(() => ({
    gridTemplateColumns: `3.5rem repeat(${columnCount}, minmax(9rem, 11rem))`,
    minWidth: '100%',
    width: 'max-content',
  }), [columnCount]);

  if (!preview.rows.length) {
    return <SpreadsheetState icon={<FileSpreadsheet className="h-12 w-12 text-slate-400" />} text={`Sheet “${preview.name}” không có ô dữ liệu.`} />;
  }

  return (
    <>
      {preview.truncated ? (
        <div className="flex items-start gap-2 border-b border-amber-200 bg-amber-50 px-3 py-2 text-xs leading-5 text-amber-800">
          <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" />
          <span>Preview giới hạn {MAX_SPREADSHEET_PREVIEW_ROWS.toLocaleString('vi-VN')} dòng và {MAX_SPREADSHEET_PREVIEW_COLUMNS} cột. Dùng “Mở tệp gốc” để xem toàn bộ.</span>
        </div>
      ) : null}
      <div
        ref={scrollRef}
        className="h-[60vh] min-h-80 overflow-auto bg-white sm:h-[65vh] xl:h-[calc(100vh-13rem)] xl:min-h-[36rem]"
        role="grid"
        aria-label={`Nội dung sheet ${preview.name}`}
        aria-rowcount={preview.rowCount}
        aria-colcount={preview.columnCount}
      >
        <div className="sticky top-0 z-20 grid border-b border-line bg-slate-100 text-xs font-semibold text-slate-600" style={gridStyle} role="row">
          <span className="sticky left-0 z-30 border-r border-line bg-slate-200 px-2 py-2 text-center" aria-hidden="true" />
          {Array.from({ length: columnCount }, (_, index) => (
            <span key={index} className="truncate border-r border-line px-2 py-2 text-center" role="columnheader">
              {spreadsheetColumnLabel(preview.startColumn + index)}
            </span>
          ))}
        </div>
        <div className="relative" style={{ height: `${rowVirtualizer.getTotalSize()}px`, minWidth: '100%', width: 'max-content' }} role="rowgroup">
          {rowVirtualizer.getVirtualItems().map((virtualRow) => {
            const row = preview.rows[virtualRow.index] ?? [];
            return (
              <div
                key={virtualRow.key}
                ref={rowVirtualizer.measureElement}
                data-index={virtualRow.index}
                className="absolute left-0 top-0 grid min-h-[34px] border-b border-line text-xs text-slate-700"
                style={{ ...gridStyle, transform: `translateY(${virtualRow.start}px)` }}
                role="row"
              >
                <span className="sticky left-0 z-10 border-r border-line bg-slate-100 px-2 py-2 text-center font-semibold text-slate-500" role="rowheader">
                  {preview.startRow + virtualRow.index + 1}
                </span>
                {Array.from({ length: columnCount }, (_, columnIndex) => {
                  const value = row[columnIndex] ?? '';
                  return <span key={columnIndex} className="min-w-0 whitespace-pre-wrap break-words border-r border-line px-2 py-2" title={value || undefined} role="gridcell">{value || '\u00a0'}</span>;
                })}
              </div>
            );
          })}
        </div>
      </div>
    </>
  );
}

function SpreadsheetState({ icon, text, action }: { icon: React.ReactNode; text: string; action?: React.ReactNode }) {
  return (
    <div className="flex h-[60vh] min-h-80 flex-col items-center justify-center px-6 py-10 text-center text-slate-600 sm:h-[65vh] xl:h-[calc(100vh-13rem)] xl:min-h-[36rem]">
      {icon}
      <p className="mt-3 max-w-md text-sm font-semibold">{text}</p>
      {action ? <div className="mt-4">{action}</div> : null}
    </div>
  );
}
