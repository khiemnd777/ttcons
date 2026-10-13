import assert from 'node:assert/strict';
import test from 'node:test';
import { readFile } from 'node:fs/promises';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';
import { build } from 'esbuild';
import { constructionReportWeatherLabel } from '../src/modules/operations/constructionWeatherPresentation.ts';

const require = createRequire(import.meta.url);
const React = require('react');
const { renderToStaticMarkup } = require('react-dom/server');
const { QueryClient, QueryClientProvider } = require('@tanstack/react-query');
const { MemoryRouter } = require('react-router-dom');
const bundle = await build({
  stdin: { resolveDir: fileURLToPath(new URL('../src', import.meta.url)), contents: `
    export { DailyTimeline } from './modules/operations/OperationsPage';
    export { ConstructionReportDetailDialog } from './modules/operations/ConstructionReportDetailDialog';
    export { SourceReportBanner as CheckBanner, CheckViewDialog } from './modules/operations/ConstructionCheckDialogs';
    export { SourceReportBanner as ActionBanner } from './modules/operations/ConstructionActionDialogs';
    export { ReportSummary } from './modules/work-reports/WorkReportReviewDialog';
    export { constructionReportViewModel, constructionReportEditModel } from './modules/operations/constructionReportViewModel';
    export { AuthContext } from './lib/auth/AuthContext';
    export { ToastProvider } from './components/ToastProvider';` },
  bundle: true, write: false, platform: 'node', format: 'cjs', packages: 'external', jsx: 'automatic', loader: { '.css': 'empty' },
  define: { 'import.meta.env': '{}', 'import.meta.url': '"file:///non-browser-test"' },
  plugins: [{ name: 'weather-real-component-entrypoints', setup(builder) {
    const exports: Record<string, string> = { 'OperationsPage.tsx': 'DailyTimeline', 'ConstructionCheckDialogs.tsx': 'SourceReportBanner, CheckViewDialog', 'ConstructionActionDialogs.tsx': 'SourceReportBanner', 'WorkReportReviewDialog.tsx': 'ReportSummary', 'AuthContext.tsx': 'AuthContext' };
    builder.onLoad({ filter: /\/(OperationsPage|ConstructionCheckDialogs|ConstructionActionDialogs|WorkReportReviewDialog|AuthContext)\.tsx$/ }, async ({ path }) => ({ contents: await readFile(path, 'utf8') + `\nexport { ${exports[path.split('/').at(-1)!]} };`, loader: 'tsx' }));
  } }],
});
const module = { exports: {} as any };
new Function('require', 'module', 'exports', bundle.outputFiles[0].text)(require, module, module.exports);
const components = module.exports;
const noop = () => {};
function render(component: any, props: object) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  try {
    return renderToStaticMarkup(React.createElement(QueryClientProvider, { client }, React.createElement(MemoryRouter, null,
      React.createElement(components.AuthContext.Provider, { value: { user: null, loading: false, hasPermission: () => false } },
        React.createElement(components.ToastProvider, null, React.createElement(component, props))))));
  } finally { client.clear(); }
}

const cases: [unknown, string][] = [
  ['normal', 'Bình thường'], ['hot', 'Nắng nóng'], ['rain', 'Mưa'], ['wind', 'Gió mạnh'], ['restricted_access', 'Hạn chế mặt bằng'], ['other', 'Khác'],
  ['Nắng', 'Nắng'], ['Mưa nhẹ tại công trường', 'Mưa nhẹ tại công trường'], ['unknown_code', 'unknown_code'], ['toString', 'toString'], ['  Custom weather  ', '  Custom weather  '],
  [undefined, 'Chưa ghi thời tiết'], [null, 'Chưa ghi thời tiết'], ['', 'Chưa ghi thời tiết'], [' ', 'Chưa ghi thời tiết'], [{ value: 'rain' }, 'Mưa'], [{ value: null }, 'Chưa ghi thời tiết'],
];
test('weather presentation agrees with the original six-option V20 contract', async () => {
  const sql = await readFile(new URL('../../backend/migrations/V20__construction_daily_report_form.sql', import.meta.url), 'utf8');
  const options = JSON.parse(sql.match(/'weather',[\s\S]*?'(\[[\s\S]*?\])'::jsonb/)![1]);
  assert.equal(options.length, 6);
  for (const option of options) assert.equal(constructionReportWeatherLabel(option.value), option.label);
});

test('actual report and linked-source components display labels without mutating raw reports', () => {
  for (const [weather, expected] of cases) {
    assert.equal(constructionReportWeatherLabel(weather), expected);
    const row = { id: 'report', weather, report_date: '2026-10-12', progress_percent: 0, structured_json: {}, evidence: [] };
    const before = structuredClone(row);
    const view = components.constructionReportViewModel(row);
    const surfaces: [any, object][] = [
      [components.DailyTimeline, { rows: [row], loading: false, error: null, onView: noop, onRetry: noop, onCreate: noop }],
      [components.ConstructionReportDetailDialog, { open: true, projectId: 'project', record: row, onClose: noop }],
      [components.ReportSummary, { detail: { report_date: row.report_date }, revision: null, view, weather: row.weather }],
      [components.CheckBanner, { report: row }], [components.ActionBanner, { report: row }],
      [components.CheckViewDialog, { check: { id: 'check', title: 'Check', status: 'pending', source_daily_report_id: row.id, source_report_weather: weather, source_report_date: row.report_date, criteria_jsonb: [] }, workspace: { actions: [], work_items: [] }, onClose: noop }],
    ];
    for (const [component, props] of surfaces) assert.ok(render(component, props).includes(expected), `${component.name}: ${JSON.stringify(weather)}`);
    assert.deepEqual(row, before);
  }
});

test('display translation never changes the raw view or edit weather authority', () => {
  for (const weather of ['normal', 'hot', 'rain', 'wind', 'restricted_access', 'other', 'Nắng', 'Mưa nhẹ tại công trường', 'unknown_code']) {
    const row = { weather, structured_json: {} };
    const before = structuredClone(row);
    assert.equal(components.constructionReportViewModel(row).weather, weather);
    assert.equal(components.constructionReportEditModel(row).weather, weather);
    constructionReportWeatherLabel(weather);
    assert.deepEqual(row, before);
  }
});
