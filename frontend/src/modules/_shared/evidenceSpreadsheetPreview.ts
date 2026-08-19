export const MAX_SPREADSHEET_PREVIEW_BYTES = 10 * 1024 * 1024;
export const MAX_SPREADSHEET_PREVIEW_SHEETS = 50;
export const MAX_SPREADSHEET_PREVIEW_ROWS = 5_000;
export const MAX_SPREADSHEET_PREVIEW_COLUMNS = 100;
export const MAX_SPREADSHEET_CELL_CHARACTERS = 2_000;

export type SpreadsheetSheetSummary = {
  name: string;
  rowCount: number;
  columnCount: number;
  truncated: boolean;
};

export type SpreadsheetSheetPreview = SpreadsheetSheetSummary & {
  startRow: number;
  startColumn: number;
  rows: string[][];
};

export type SpreadsheetWorkerRequest =
  | { type: 'load'; buffer: ArrayBuffer }
  | { type: 'sheet'; sheetName: string; requestId: number };

export type SpreadsheetWorkerResponse =
  | { type: 'loaded'; sheets: SpreadsheetSheetSummary[]; sheetsTruncated: boolean }
  | { type: 'sheet'; requestId: number; preview: SpreadsheetSheetPreview }
  | { type: 'error'; requestId?: number; message: string };

export function spreadsheetColumnLabel(columnIndex: number) {
  let value = Math.max(0, Math.trunc(columnIndex)) + 1;
  let label = '';
  while (value > 0) {
    const remainder = (value - 1) % 26;
    label = String.fromCharCode(65 + remainder) + label;
    value = Math.floor((value - 1) / 26);
  }
  return label;
}
