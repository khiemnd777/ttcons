import type { RoleRow } from './staffImportTypes';

export const STAFF_IMPORT_HEADERS = [
  'full_name',
  'login_email',
  'password',
  'recovery_email',
  'phone',
  'department',
  'position',
  'role_code',
] as const;

export const STAFF_IMPORT_MAX_FILE_BYTES = 1024 * 1024;
export const STAFF_IMPORT_MAX_ROWS = 100;
export const STAFF_IMPORT_RESTRICTED_ROLES = new Set(['admin', 'client', 'developer']);

export type StaffImportRow = Record<(typeof STAFF_IMPORT_HEADERS)[number], string> & { row_number: number };

type WorkerResponse =
  | { type: 'parsed'; rows: StaffImportRow[] }
  | { type: 'error'; message: string };

export async function parseStaffImportFile(file: File): Promise<StaffImportRow[]> {
  const extension = file.name.toLocaleLowerCase('vi-VN').split('.').pop();
  if (extension !== 'xlsx' && extension !== 'csv') throw new Error('Chỉ chấp nhận tệp .xlsx hoặc .csv.');
  if (file.size <= 0 || file.size > STAFF_IMPORT_MAX_FILE_BYTES) throw new Error('Tệp phải có dung lượng không quá 1 MB.');
  const buffer = await file.arrayBuffer();
  return new Promise<StaffImportRow[]>((resolve, reject) => {
    const worker = new Worker(new URL('./staffImport.worker.ts', import.meta.url), { type: 'module' });
    const finish = () => worker.terminate();
    worker.onmessage = (event: MessageEvent<WorkerResponse>) => {
      finish();
      if (event.data.type === 'parsed') resolve(event.data.rows);
      else reject(new Error(event.data.message));
    };
    worker.onerror = () => {
      finish();
      reject(new Error('Không thể đọc tệp import.'));
    };
    worker.postMessage({ type: 'parse', fileName: file.name, buffer }, [buffer]);
  });
}

export async function downloadStaffImportTemplate(format: 'xlsx' | 'csv', roles: RoleRow[]) {
  if (format === 'csv') {
    downloadBlob(new Blob([`\uFEFF${STAFF_IMPORT_HEADERS.join(',')}\r\n`], { type: 'text/csv;charset=utf-8' }), 'mau-import-tai-khoan-nhan-su.csv');
    return;
  }
  const xlsx = await import('xlsx');
  const workbook = xlsx.utils.book_new();
  const accounts = xlsx.utils.aoa_to_sheet([Array.from(STAFF_IMPORT_HEADERS)]);
  accounts['!cols'] = [{ wch: 28 }, { wch: 32 }, { wch: 18 }, { wch: 32 }, { wch: 18 }, { wch: 22 }, { wch: 22 }, { wch: 20 }];
  xlsx.utils.book_append_sheet(workbook, accounts, 'accounts');

  const roleRows = roles
    .filter((role) => !role.external_only && !STAFF_IMPORT_RESTRICTED_ROLES.has(role.name))
    .sort((left, right) => left.display_name.localeCompare(right.display_name, 'vi'))
    .map((role) => [role.name, role.display_name]);
  const roleSheet = xlsx.utils.aoa_to_sheet([['role_code', 'Tên vai trò'], ...roleRows]);
  roleSheet['!cols'] = [{ wch: 24 }, { wch: 30 }];
  xlsx.utils.book_append_sheet(workbook, roleSheet, 'roles');

  const instructions = xlsx.utils.aoa_to_sheet([
    ['Hướng dẫn import tài khoản nhân sự'],
    ['1', 'Chỉ nhập dữ liệu trong sheet accounts; giữ nguyên đúng 8 tiêu đề cột.'],
    ['2', 'full_name, login_email, password và role_code là bắt buộc.'],
    ['3', 'Mật khẩu: tối thiểu 8 ký tự Unicode, tối đa 72 byte và không chỉ gồm khoảng trắng.'],
    ['4', 'Mỗi tệp tối đa 100 tài khoản và 1 MB. Tài khoản được kích hoạt ngay, sau đó phải đổi mật khẩu lần đầu.'],
    ['5', 'role_code phải lấy từ sheet roles. Không import được admin, client hoặc developer.'],
    ['6', 'Tệp chứa mật khẩu nhạy cảm: chỉ lưu và truyền qua kênh an toàn, xóa tệp sau khi dùng.'],
  ]);
  instructions['!cols'] = [{ wch: 8 }, { wch: 110 }];
  xlsx.utils.book_append_sheet(workbook, instructions, 'instructions');
  xlsx.writeFile(workbook, 'mau-import-tai-khoan-nhan-su.xlsx', { compression: true });
}

function downloadBlob(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement('a');
  anchor.href = url;
  anchor.download = filename;
  anchor.click();
  URL.revokeObjectURL(url);
}
