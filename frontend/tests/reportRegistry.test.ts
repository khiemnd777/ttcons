import assert from 'node:assert/strict';
import test from 'node:test';
import { reportPath, visibleReportDefinitions } from '../src/modules/reports/reportRegistry.ts';

function permissionCheck(granted: string[]) {
  const permissions = new Set(granted);
  return (required?: string[]) => !required?.length || required.some((permission) => permissions.has(permission));
}

test('report catalog only exposes operational reports to report viewers', () => {
  assert.deepEqual(
    visibleReportDefinitions(permissionCheck(['report.view'])).map((report) => report.key),
    ['projects', 'procurement', 'warehouse', 'staff'],
  );
});

test('report catalog exposes finance reports to finance or accounting viewers', () => {
  assert.deepEqual(
    visibleReportDefinitions(permissionCheck(['report.finance.view'])).map((report) => report.key),
    ['profit', 'cashflow', 'tax', 'accounting'],
  );
  assert.equal(reportPath('profit'), '/reports/profit');
});
