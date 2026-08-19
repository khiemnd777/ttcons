import type { WorkBook, WorkSheet } from 'xlsx';
import xlsxMini from 'xlsx/dist/xlsx.mini.min.js';
import {
  MAX_SPREADSHEET_CELL_CHARACTERS,
  MAX_SPREADSHEET_PREVIEW_COLUMNS,
  MAX_SPREADSHEET_PREVIEW_ROWS,
  MAX_SPREADSHEET_PREVIEW_SHEETS,
  type SpreadsheetSheetPreview,
  type SpreadsheetSheetSummary,
  type SpreadsheetWorkerRequest,
  type SpreadsheetWorkerResponse,
} from './evidenceSpreadsheetPreview';

type SheetJsModule = typeof import('xlsx');
type SpreadsheetWorkerScope = {
  onmessage: ((event: MessageEvent<SpreadsheetWorkerRequest>) => void) | null;
  postMessage: (message: SpreadsheetWorkerResponse) => void;
};

const workerScope = self as unknown as SpreadsheetWorkerScope;
let workbook: WorkBook | null = null;
const sheetJs = xlsxMini as unknown as SheetJsModule;

workerScope.onmessage = (event) => {
  void handleRequest(event.data);
};

async function handleRequest(request: SpreadsheetWorkerRequest) {
  try {
    if (request.type === 'load') {
      workbook = sheetJs.read(new Uint8Array(request.buffer), {
        type: 'array',
        cellFormula: false,
        cellHTML: false,
        cellNF: false,
        cellStyles: false,
        bookVBA: false,
        sheetRows: MAX_SPREADSHEET_PREVIEW_ROWS,
      });
      const visibleSheetNames = workbook.SheetNames.slice(0, MAX_SPREADSHEET_PREVIEW_SHEETS);
      const sheets = visibleSheetNames.map((name) => sheetSummary(name, workbook?.Sheets[name]));
      workerScope.postMessage({
        type: 'loaded',
        sheets,
        sheetsTruncated: workbook.SheetNames.length > visibleSheetNames.length,
      });
      return;
    }

    if (!workbook) throw new Error('Workbook chưa được tải.');
    const worksheet = workbook.Sheets[request.sheetName];
    if (!worksheet) throw new Error('Không tìm thấy sheet đã chọn.');
    workerScope.postMessage({
      type: 'sheet',
      requestId: request.requestId,
      preview: buildSheetPreview(request.sheetName, worksheet, sheetJs),
    });
  } catch (error) {
    workerScope.postMessage({
      type: 'error',
      requestId: request.type === 'sheet' ? request.requestId : undefined,
      message: error instanceof Error ? error.message : 'Không thể đọc workbook.',
    });
  }
}

function sheetSummary(name: string, worksheet?: WorkSheet): SpreadsheetSheetSummary {
  const range = worksheet ? fullRange(worksheet) : undefined;
  const rowCount = range ? Math.max(0, range.e.r - range.s.r + 1) : 0;
  const columnCount = range ? Math.max(0, range.e.c - range.s.c + 1) : 0;
  return {
    name,
    rowCount,
    columnCount,
    truncated: rowCount > MAX_SPREADSHEET_PREVIEW_ROWS || columnCount > MAX_SPREADSHEET_PREVIEW_COLUMNS,
  };
}

function buildSheetPreview(name: string, worksheet: WorkSheet, xlsx: SheetJsModule): SpreadsheetSheetPreview {
  const summary = sheetSummary(name, worksheet);
  const range = fullRange(worksheet);
  if (!range || summary.rowCount === 0 || summary.columnCount === 0) {
    return { ...summary, startRow: 0, startColumn: 0, rows: [] };
  }

  const previewEndRow = Math.min(range.e.r, range.s.r + MAX_SPREADSHEET_PREVIEW_ROWS - 1);
  const previewEndColumn = Math.min(range.e.c, range.s.c + MAX_SPREADSHEET_PREVIEW_COLUMNS - 1);
  const values = xlsx.utils.sheet_to_json<unknown[]>(worksheet, {
    header: 1,
    raw: false,
    defval: '',
    blankrows: true,
    range: {
      s: { r: range.s.r, c: range.s.c },
      e: { r: previewEndRow, c: previewEndColumn },
    },
  });
  const columnCount = previewEndColumn - range.s.c + 1;
  const rows = values.map((row) => Array.from({ length: columnCount }, (_, index) => safeCellText(row[index])));
  return {
    ...summary,
    startRow: range.s.r,
    startColumn: range.s.c,
    rows,
  };
}

function fullRange(worksheet: WorkSheet) {
  const ref = (worksheet as WorkSheet & { '!fullref'?: string })['!fullref'] ?? worksheet['!ref'];
  if (!ref) return undefined;
  try {
    return sheetJs.utils.decode_range(ref);
  } catch {
    return undefined;
  }
}

function safeCellText(value: unknown) {
  if (value === null || value === undefined) return '';
  const text = typeof value === 'string' ? value : String(value);
  return text.slice(0, MAX_SPREADSHEET_CELL_CHARACTERS);
}
