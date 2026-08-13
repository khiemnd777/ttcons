import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

async function source(path: string) {
  return readFile(new URL(path, import.meta.url), 'utf8');
}

test('signature pointer movement snapshots coordinates before the deferred state update', async () => {
  const dialog = await source('../src/modules/work-items/SignatureCaptureDialog.tsx');

  assert.match(
    dialog,
    /const nextPoint = pointFromEvent\(event\);\s*setActiveStroke\(\(current\) => \(current \? \[\.\.\.current, nextPoint\] : current\)\);/,
  );
  assert.doesNotMatch(
    dialog,
    /setActiveStroke\(\(current\)[\s\S]{0,160}pointFromEvent\(event\)/,
    'React pointer events must not be dereferenced from a deferred state updater',
  );
});
