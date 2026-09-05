import assert from 'node:assert/strict';
import test from 'node:test';
import { read, utils } from 'xlsx';
import xlsxMini from 'xlsx/dist/xlsx.mini.min.js';
import { downloadStaffImportTemplate } from '../src/modules/staff/staffImport.ts';

test('mini writer preserves the Staff template workbook and eligible Vietnamese role labels', async (t) => {
  const mini = xlsxMini as unknown as typeof import('xlsx');
  let bytes: Uint8Array | undefined;
  // Replace only the platform download; exercise the production workbook builder and real mini encoder.
  const download = t.mock.method(mini, 'writeFile', (workbook, filename, options) => {
    assert.equal(filename, 'mau-import-tai-khoan-nhan-su.xlsx');
    assert.equal(options.compression, true);
    bytes = mini.write(workbook, { ...options, type: 'buffer', bookType: 'xlsx' });
  });

  const roles = [
    { id: '1', name: 'pm', display_name: 'Quản lý dự án' },
    { id: '2', name: 'accounting', display_name: 'Kế toán' },
    { id: '3', name: 'admin', display_name: 'Quản trị' },
    { id: '4', name: 'client', display_name: 'Khách hàng' },
    { id: '5', name: 'developer', display_name: 'Phát triển' },
    { id: '6', name: 'external', display_name: 'Đối tác', external_only: true },
  ];
  const originalRoles = structuredClone(roles);
  await downloadStaffImportTemplate('xlsx', roles);

  assert.equal(download.mock.callCount(), 1);
  assert.deepEqual(roles, originalRoles);
  assert.ok(bytes);
  // Decode with the full reader independently of the mini encoder used by the application.
  const workbook = read(bytes, { type: 'buffer', cellStyles: true });
  assert.deepEqual(workbook.SheetNames, ['accounts', 'roles', 'instructions']);
  assert.deepEqual(utils.sheet_to_json(workbook.Sheets.accounts, { header: 1 }), [[
    'full_name', 'login_email', 'password', 'recovery_email', 'phone', 'department', 'position', 'role_code',
  ]]);
  assert.deepEqual(utils.sheet_to_json(workbook.Sheets.roles, { header: 1 }), [
    ['role_code', 'Tên vai trò'], ['accounting', 'Kế toán'], ['pm', 'Quản lý dự án'],
  ]);
  const instructions = utils.sheet_to_json<string[]>(workbook.Sheets.instructions, { header: 1 });
  assert.equal(instructions.length, 7);
  assert.equal(instructions[0][0], 'Hướng dẫn import tài khoản nhân sự');
  assert.match(instructions[4][1], /100 tài khoản và 1 MB/);
  assert.match(instructions[5][1], /Không import được admin, client hoặc developer/);
  for (const [name, widths] of Object.entries({ accounts: [28, 32, 18, 32, 18, 22, 22, 20], roles: [24, 30], instructions: [8, 110] })) {
    assert.deepEqual(workbook.Sheets[name]['!cols']?.map((column) => column.wch), widths);
  }
});
