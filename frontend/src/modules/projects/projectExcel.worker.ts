import type { WorkSheet } from 'xlsx';
import xlsxMini from 'xlsx/dist/xlsx.mini.min.js';
import {
  PROJECT_DATA_PACKAGE_VERSION,
  PROJECT_EXCEL_IMPORT_HEADERS,
  PROJECT_EXCEL_MAXIMUM_ROWS,
  PROJECT_EXCEL_WORKBOOK_VERSION,
  type ProjectExcelImportRow,
  type DataPackageFormat,
} from './projectExcel';
import { assertExactKeys, isJsonRecord, parseStrictJson } from '../_shared/strictJson';

type SheetJs = typeof import('xlsx');
type WorkerRequest = { type: 'parse'; sourceFormat: DataPackageFormat; buffer: ArrayBuffer };
type WorkerScope = { onmessage: ((event: MessageEvent<WorkerRequest>) => void) | null; postMessage: (message: unknown) => void };

const scope = self as unknown as WorkerScope;
const xlsx = xlsxMini as unknown as SheetJs;

scope.onmessage = (event) => {
  try {
	if (event.data.sourceFormat === 'json') {
		parseJSONPackage(event.data.buffer);
		return;
	}
    const workbook = xlsx.read(new Uint8Array(event.data.buffer), {
      type: 'array',
      raw: true,
      cellDates: false,
      cellFormula: true,
      cellHTML: false,
      cellNF: false,
      cellStyles: false,
      bookVBA: false,
      sheetRows: PROJECT_EXCEL_MAXIMUM_ROWS + 2,
    });
    const projectSheet = findSheet(workbook.SheetNames, 'Cong_trinh');
    const guideSheet = findSheet(workbook.SheetNames, 'Huong_dan');
    if (!projectSheet || !guideSheet) {
      throw new Error('Tệp Excel phải có đủ sheet Huong_dan và Cong_trinh. Hãy tải lại file mẫu mới.');
    }

    const guide = workbook.Sheets[guideSheet];
    const worksheet = workbook.Sheets[projectSheet];
    if (!guide || !worksheet) throw new Error('Không đọc được cấu trúc tệp Excel.');
    const workbookVersion = readWorkbookVersion(guide);
    if (workbookVersion !== PROJECT_EXCEL_WORKBOOK_VERSION) {
      throw new Error(`Phiên bản file không được hỗ trợ. Hệ thống yêu cầu workbook_version=${PROJECT_EXCEL_WORKBOOK_VERSION}.`);
    }
    rejectFormulas(worksheet);

    const values = xlsx.utils.sheet_to_json<unknown[]>(worksheet, {
      header: 1,
      raw: false,
      defval: '',
      blankrows: false,
      dateNF: 'yyyy-mm-dd',
    });
    if (!values.length) throw new Error('Sheet Cong_trinh chưa có dòng tiêu đề.');
    const headers = values[0].map((value) => String(value ?? '').trim());
    if (headers.length !== PROJECT_EXCEL_IMPORT_HEADERS.length || PROJECT_EXCEL_IMPORT_HEADERS.some((header, index) => headers[index] !== header)) {
      throw new Error(`Dòng tiêu đề sheet Cong_trinh phải đúng thứ tự: ${PROJECT_EXCEL_IMPORT_HEADERS.join(', ')}.`);
    }
    const rows = values.slice(1)
      .map((rowValues, index) => ({ values: rowValues, rowNumber: index + 2 }))
      .filter(({ values: rowValues }) => rowValues.some((value) => String(value ?? '').trim() !== ''));
    if (!rows.length) throw new Error('Sheet Cong_trinh chưa có dữ liệu công trình.');
    if (rows.length > PROJECT_EXCEL_MAXIMUM_ROWS) {
      throw new Error(`Mỗi lần chỉ được import tối đa ${PROJECT_EXCEL_MAXIMUM_ROWS} công trình.`);
    }
    const parsed = rows.map(({ values: rowValues, rowNumber }) => {
      const row = { row_number: rowNumber } as ProjectExcelImportRow;
      PROJECT_EXCEL_IMPORT_HEADERS.forEach((header, index) => {
        row[header] = String(rowValues[index] ?? '').trim();
      });
      return row;
    });
    scope.postMessage({ type: 'parsed', packageVersion: workbookVersion, sourceFormat: 'xlsx', rows: parsed });
  } catch (error) {
    scope.postMessage({ type: 'error', message: error instanceof Error ? error.message : 'Không thể đọc tệp Excel.' });
  }
};

function parseJSONPackage(buffer: ArrayBuffer) {
  const text = new TextDecoder('utf-8', { fatal: true }).decode(buffer);
  const value = parseStrictJson(text);
  if (!isJsonRecord(value)) throw new Error('JSON Công trình phải là một object.');
  assertExactKeys(value, ['schema', 'schema_version', 'mode', 'generated_at', 'reference_fingerprint', 'instructions', 'readiness', 'projects', 'project_snapshots', 'references'], 'Gói Công trình');
  if (value.schema !== 'consflow.project-package' || value.schema_version !== Number(PROJECT_DATA_PACKAGE_VERSION)) {
    throw new Error(`JSON không đúng schema Công trình v${PROJECT_DATA_PACKAGE_VERSION}.`);
  }
  if (!Array.isArray(value.projects) || !value.projects.length) throw new Error('Mảng projects chưa có dữ liệu công trình.');
  if (value.projects.length > PROJECT_EXCEL_MAXIMUM_ROWS) throw new Error(`Mỗi lần chỉ được import tối đa ${PROJECT_EXCEL_MAXIMUM_ROWS} công trình.`);
  const rows = value.projects.map((item, index) => {
    if (!isJsonRecord(item)) throw new Error(`projects[${index}] phải là object.`);
    assertExactKeys(item, PROJECT_EXCEL_IMPORT_HEADERS, `projects[${index}]`);
    const row = { row_number: index + 2 } as ProjectExcelImportRow;
    PROJECT_EXCEL_IMPORT_HEADERS.forEach((header) => {
      const fieldValue = item[header];
      if (typeof fieldValue !== 'string') throw new Error(`projects[${index}].${header} phải là chuỗi.`);
      row[header] = fieldValue.trim();
    });
    return row;
  });
  scope.postMessage({ type: 'parsed', packageVersion: PROJECT_DATA_PACKAGE_VERSION, sourceFormat: 'json', rows });
}

function findSheet(sheetNames: string[], expected: string) {
  const normalizedExpected = expected.toLocaleLowerCase('vi-VN');
  return sheetNames.find((name) => name.trim().toLocaleLowerCase('vi-VN') === normalizedExpected);
}

function readWorkbookVersion(worksheet: WorkSheet) {
  const rows = xlsx.utils.sheet_to_json<unknown[]>(worksheet, { header: 1, raw: false, defval: '', blankrows: false });
  const versionRow = rows.find((row) => String(row[0] ?? '').trim() === 'workbook_version');
  if (!versionRow) throw new Error('Sheet Huong_dan không có workbook_version. Hãy tải lại file mẫu mới.');
  return String(versionRow[1] ?? '').trim();
}

function rejectFormulas(worksheet: WorkSheet) {
  for (const [address, cell] of Object.entries(worksheet)) {
    if (address.startsWith('!')) continue;
    if (cell && typeof cell === 'object' && 'f' in cell && cell.f) {
      throw new Error('Sheet Cong_trinh không được chứa công thức. Hãy chuyển các ô thành giá trị trước khi tải lên.');
    }
  }
}
