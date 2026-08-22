import type { WorkSheet } from 'xlsx';
import xlsxMini from 'xlsx/dist/xlsx.mini.min.js';
import { STAFF_IMPORT_HEADERS, STAFF_IMPORT_MAX_ROWS, type StaffImportRow } from './staffImport';

type SheetJs = typeof import('xlsx');
type WorkerRequest = { type: 'parse'; fileName: string; buffer: ArrayBuffer };
type WorkerScope = { onmessage: ((event: MessageEvent<WorkerRequest>) => void) | null; postMessage: (message: unknown) => void };

const scope = self as unknown as WorkerScope;
const xlsx = xlsxMini as unknown as SheetJs;

scope.onmessage = (event) => {
  try {
    const workbook = xlsx.read(new Uint8Array(event.data.buffer), {
      type: 'array', raw: true, cellFormula: true, cellHTML: false, cellNF: false, cellStyles: false, bookVBA: false,
      sheetRows: STAFF_IMPORT_MAX_ROWS + 2,
    });
    const isCSV = event.data.fileName.toLocaleLowerCase('vi-VN').endsWith('.csv');
    const sheetName = isCSV ? workbook.SheetNames[0] : workbook.SheetNames.find((name) => name.trim().toLocaleLowerCase('vi-VN') === 'accounts');
    if (!sheetName) throw new Error(isCSV ? 'Tệp CSV không có dữ liệu.' : 'Tệp Excel phải có sheet accounts.');
    const worksheet = workbook.Sheets[sheetName];
    if (!worksheet) throw new Error('Không tìm thấy dữ liệu tài khoản.');
    rejectFormulas(worksheet);
    const values = xlsx.utils.sheet_to_json<unknown[]>(worksheet, { header: 1, raw: false, defval: '', blankrows: false });
    if (!values.length) throw new Error('Tệp import chưa có dòng tiêu đề.');
    const headers = values[0].map((value) => String(value ?? '').trim());
    if (headers.length !== STAFF_IMPORT_HEADERS.length || STAFF_IMPORT_HEADERS.some((header, index) => headers[index] !== header)) {
      throw new Error(`Dòng tiêu đề phải đúng thứ tự: ${STAFF_IMPORT_HEADERS.join(', ')}.`);
    }
    const rows = values.slice(1)
      .map((values, index) => ({ values, rowNumber: index + 2 }))
      .filter(({ values }) => values.some((value) => String(value ?? '').trim() !== ''));
    if (!rows.length) throw new Error('Tệp import chưa có tài khoản.');
    if (rows.length > STAFF_IMPORT_MAX_ROWS) throw new Error(`Mỗi lần chỉ được import tối đa ${STAFF_IMPORT_MAX_ROWS} tài khoản.`);
    const parsed: StaffImportRow[] = rows.map(({ values, rowNumber }) => {
      const row = { row_number: rowNumber } as StaffImportRow;
      STAFF_IMPORT_HEADERS.forEach((header, index) => { row[header] = String(values[index] ?? ''); });
      return row;
    });
    scope.postMessage({ type: 'parsed', rows: parsed });
  } catch (error) {
    scope.postMessage({ type: 'error', message: error instanceof Error ? error.message : 'Không thể đọc tệp import.' });
  }
};

function rejectFormulas(worksheet: WorkSheet) {
  for (const [address, cell] of Object.entries(worksheet)) {
    if (address.startsWith('!')) continue;
    if (cell && typeof cell === 'object' && 'f' in cell && cell.f) {
      throw new Error('Tệp import không được chứa công thức. Hãy chuyển các ô thành giá trị trước khi tải lên.');
    }
  }
}
