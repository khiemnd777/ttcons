import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

async function source(path: string) {
  return readFile(new URL(path, import.meta.url), 'utf8');
}

test('mobile and primary-touch form fields prevent focus zoom without disabling user zoom', async () => {
  const [styles, html, constructionReportStyles, evidenceCaptureStyles] = await Promise.all([
    source('../src/index.css'),
    source('../index.html'),
    source('../src/modules/operations/constructionReport.css'),
    source('../src/modules/_shared/evidenceCapture.css'),
  ]);

  assert.match(styles, /@media \(max-width: 767px\), \(hover: none\) and \(pointer: coarse\)/);
  assert.match(
    styles,
    /input:not\(\[type='checkbox'\]\):not\(\[type='radio'\]\):not\(\[type='range'\]\):not\(\[type='button'\]\):not\(\[type='submit'\]\):not\(\[type='reset'\]\):not\(\[type='image'\]\):not\(\[type='hidden'\]\),\s*select,\s*textarea\s*\{\s*font-size: 16px !important;/s,
  );
  assert.doesNotMatch(html, /maximum-scale|user-scalable/);
  assert.doesNotMatch(constructionReportStyles, /font-size:\s*16px/);
  assert.doesNotMatch(evidenceCaptureStyles, /font-size:\s*16px/);
});
