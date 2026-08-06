import assert from 'node:assert/strict';
import test from 'node:test';
import { reportDefinitions, reportPath, visibleReportDefinitions } from '../src/modules/reports/reportRegistry.ts';

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

test('professional accounting reports are not exposed in the ordinary report catalog', () => {
  assert.deepEqual(visibleReportDefinitions(permissionCheck(['report.finance.view'])), []);
  assert.equal(reportPath('projects'), '/reports/projects');
});

test('every catalog report defines one active-report basis and CSV file name', () => {
  for (const report of reportDefinitions) {
    assert.ok(report.basis.length > report.description.length);
    assert.match(report.fileName, /^bao-cao-/);
  }
});
