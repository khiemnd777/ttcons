import assert from 'node:assert/strict';
import test from 'node:test';
import { reportDefinitions, reportPath, visibleReportDefinitions } from '../src/modules/reports/reportRegistry.ts';

function permissionCheck(granted: string[]) {
  const permissions = new Set(granted);
  return (required?: string[]) => !required?.length || required.some((permission) => permissions.has(permission));
}

test('statistics catalog only exposes aggregates to statistics viewers', () => {
  assert.deepEqual(
    visibleReportDefinitions(permissionCheck(['statistics.view'])).map((report) => report.key),
    ['projects', 'procurement', 'warehouse', 'staff'],
  );
  assert.deepEqual(visibleReportDefinitions(permissionCheck(['report.view'])), []);
});

test('professional accounting reports are not exposed in the statistics catalog', () => {
  assert.deepEqual(visibleReportDefinitions(permissionCheck(['report.finance.view'])), []);
  assert.equal(reportPath('projects'), '/statistics/projects');
});

test('every statistics definition has a basis and neutral export file name', () => {
  for (const report of reportDefinitions) {
    assert.ok(report.basis.length > report.description.length);
    assert.match(report.fileName, /^thong-ke-/);
  }
});
