import type { DataPackageFormat } from '../_shared/dataPackage';

export const PROJECT_EXCEL_IMPORT_HEADERS = [
  'project_code',
  'project_name',
  'client_partner_code',
  'client_name_unregistered',
  'address',
  'planned_start_date',
  'planned_end_date',
  'estimated_budget_vnd',
  'pipeline_ref',
] as const;

export const PROJECT_EXCEL_WORKBOOK_VERSION = '1';
export const PROJECT_DATA_PACKAGE_VERSION = '1';
export const PROJECT_EXCEL_MAXIMUM_FILE_BYTES = 5 * 1024 * 1024;
export const PROJECT_EXCEL_MAXIMUM_ROWS = 100;

export type ProjectExcelImportRow = Record<(typeof PROJECT_EXCEL_IMPORT_HEADERS)[number], string> & {
  row_number: number;
};

export type ParsedProjectExcelFile = {
  workbookVersion: string;
  packageVersion: string;
  sourceFormat: DataPackageFormat;
  rows: ProjectExcelImportRow[];
};

type WorkerResponse =
  | { type: 'parsed'; packageVersion: string; sourceFormat: DataPackageFormat; rows: ProjectExcelImportRow[] }
  | { type: 'error'; message: string };

export async function parseProjectExcelFile(file: File): Promise<ParsedProjectExcelFile> {
  return parseProjectImportFile(file, 'xlsx');
}

export async function parseProjectImportFile(file: File, sourceFormat: DataPackageFormat): Promise<ParsedProjectExcelFile> {
  const expectedExtension = `.${sourceFormat}`;
  if (!file.name.toLocaleLowerCase('vi-VN').endsWith(expectedExtension)) {
    throw new Error(`Định dạng đã chọn chỉ chấp nhận tệp ${expectedExtension}.`);
  }
  if (file.size <= 0 || file.size > PROJECT_EXCEL_MAXIMUM_FILE_BYTES) {
    throw new Error('Tệp phải có dung lượng không quá 5 MB.');
  }
  const buffer = await file.arrayBuffer();
  return new Promise<ParsedProjectExcelFile>((resolve, reject) => {
    const worker = new Worker(new URL('./projectExcel.worker.ts', import.meta.url), { type: 'module' });
    const finish = () => worker.terminate();
    worker.onmessage = (event: MessageEvent<WorkerResponse>) => {
      finish();
      if (event.data.type === 'parsed') {
        resolve({ workbookVersion: event.data.packageVersion, packageVersion: event.data.packageVersion, sourceFormat: event.data.sourceFormat, rows: event.data.rows });
      } else {
        reject(new Error(event.data.message));
      }
    };
    worker.onerror = () => {
      finish();
      reject(new Error('Không thể đọc tệp dữ liệu Công trình.'));
    };
    worker.postMessage({ type: 'parse', sourceFormat, buffer }, [buffer]);
  });
}

export function saveProjectExcelBlob(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement('a');
  anchor.href = url;
  anchor.download = filename;
  anchor.click();
  URL.revokeObjectURL(url);
}
export type { DataPackageFormat } from '../_shared/dataPackage';
