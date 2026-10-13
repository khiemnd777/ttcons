import assert from 'node:assert/strict';
import test from 'node:test';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';
import { build } from 'esbuild';
import { createElement } from 'react';
import { renderToStaticMarkup } from 'react-dom/server';

const require = createRequire(import.meta.url);
const bundle = await build({ stdin: { resolveDir: fileURLToPath(new URL('../src/', import.meta.url)), contents: `export * from './modules/_shared/bmgsExport'; export { EvidenceCapture } from './modules/_shared/EvidenceCapture'; export { ToastProvider } from './components/ToastProvider';` }, bundle: true, write: false, platform: 'node', format: 'cjs', packages: 'external', jsx: 'automatic', loader: { '.css': 'empty' }, define: { 'import.meta.env': '{}', 'import.meta.url': '"file:///fixture"' } });
const module = { exports: {} as Record<string, any> };
new Function('require', 'module', 'exports', bundle.outputFiles[0].text)(require, module, module.exports);

test('native export binds exact saved report or immutable revision without selecting photos', () => {
  assert.equal(module.exports.nativeReportExcelPath({dailyReportId: 'daily'}), '/daily-reports/daily/export.xlsx');
  assert.equal(module.exports.nativeReportExcelPath({workReportId: 'formal', revisionNo: 3}), '/work-reports/formal/revisions/3/export.xlsx');
  for (const revisionNo of [0, -1, NaN, 1.5]) assert.throws(() => module.exports.nativeReportExcelPath({workReportId: 'formal', revisionNo}));
  assert.throws(() => module.exports.nativeReportExcelPath({dailyReportId: ''}));
});

test('native list export preserves applied business filters and omits paging and legacy chooser inputs', () => {
  const filters = 'scope=review&project_id=project&status=submitted&date_from=2026-10-01&date_to=2026-10-08&search=steel&supervisor_id=person&read_status=unread';
  assert.equal(module.exports.nativeReportListExcelPath('/work-reports', filters + '&limit=20&offset=40&evidence_ids=photo&template_code=BM-GS-02'), `/work-reports/export.xlsx?${filters}`);
  assert.equal(module.exports.nativeReportListExcelPath('/projects/project/daily-reports', 'date_from=2026-10-01&date_to=2026-10-08&page=2'), '/projects/project/daily-reports/export.xlsx?date_from=2026-10-01&date_to=2026-10-08');
});

test('native GET download uses protected client, downloads exact content and revokes URL even when click fails', async () => {
  const originalFetch = globalThis.fetch;
  const originalDocument = Object.getOwnPropertyDescriptor(globalThis, 'document');
  const originalStorage = Object.getOwnPropertyDescriptor(globalThis, 'localStorage');
  const originalCreate = URL.createObjectURL; const originalRevoke = URL.revokeObjectURL;
  const paths: string[] = []; const revoked: string[] = [];
  Object.defineProperty(globalThis, 'localStorage', { configurable: true, value: { getItem: () => 'access-token' } });
  globalThis.fetch = async (url, init) => { paths.push(String(url)); assert.equal(new Headers(init?.headers).get('Authorization'), 'Bearer access-token'); assert.equal(init?.method ?? 'GET', 'GET'); return new Response('exact-xlsx'); };
  URL.createObjectURL = (blob: Blob) => { assert.equal(blob.size, 10); return 'blob:exact-native'; };
  URL.revokeObjectURL = (url: string) => { revoked.push(url); };
  try {
    for (const clickFails of [false, true]) {
      const anchor = { href: '', download: '', click() { if (clickFails) throw new Error('click failed'); } };
      Object.defineProperty(globalThis, 'document', { configurable: true, value: { createElement: (tag: string) => { assert.equal(tag, 'a'); return anchor; } } });
      const download = module.exports.downloadNativeReportExcel('/work-reports/formal/revisions/3/export.xlsx', 'lan-3.xlsx');
      if (clickFails) await assert.rejects(download, /click failed/); else await download;
      assert.equal(anchor.href, 'blob:exact-native'); assert.equal(anchor.download, 'lan-3.xlsx');
    }
    assert.equal(paths.length, 2); assert.ok(paths.every(path => path.endsWith('/work-reports/formal/revisions/3/export.xlsx')));
    assert.deepEqual(revoked, ['blob:exact-native', 'blob:exact-native']);
  } finally {
    globalThis.fetch = originalFetch; URL.createObjectURL = originalCreate; URL.revokeObjectURL = originalRevoke;
    if (originalDocument) Object.defineProperty(globalThis, 'document', originalDocument); else delete (globalThis as any).document;
    if (originalStorage) Object.defineProperty(globalThis, 'localStorage', originalStorage); else delete (globalThis as any).localStorage;
  }
});

test('shared uploader image-only hint and chooser match, default document support remains available', () => {
  const { QueryClient, QueryClientProvider } = require('@tanstack/react-query');
  function render(allowedFileKinds?: string[]) {
    const client = new QueryClient();
    try { return renderToStaticMarkup(createElement(QueryClientProvider, {client}, createElement(module.exports.ToastProvider, null, createElement(module.exports.EvidenceCapture, {drafts: [], onChange() {}, hasSource: true, uploadScope: '/work-items/item/evidence/upload', allowedFileKinds})))); }
    finally {client.clear();}
  }
  const photo = render(['photo']);
  assert.match(photo, /PNG\/JPG\/JPEG\/WebP\/GIF/);
  assert.doesNotMatch(photo, /PDF|Word|Excel|DOCX|XLSX/);
  for (const input of photo.matchAll(/<input[^>]+type="file"[^>]*>/g)) assert.doesNotMatch(input[0], /pdf|doc|xls/);
  assert.match(render(), /PNG\/JPG\/JPEG\/WebP\/GIF, PDF, Word \(DOC\/DOCX\), Excel \(XLS\/XLSX\)/);
});
