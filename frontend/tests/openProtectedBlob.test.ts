import assert from 'node:assert/strict';
import test from 'node:test';
import { openProtectedBlob } from '../src/lib/utils/openProtectedBlob.ts';

test('protected blob opener uses exactly one synchronous viewer and reuses it after download', async () => {
  const previousWindow = (globalThis as { window?: unknown }).window;
  const previousCreateObjectURL = URL.createObjectURL;
  const previousRevokeObjectURL = URL.revokeObjectURL;
  let openCalls = 0;
  let closeCalls = 0;
  let revoked = '';
  const viewer = {
    opener: {} as unknown,
    document: { title: '', body: { textContent: '' } },
    location: { href: '' },
    close: () => { closeCalls += 1; },
  };

  Object.defineProperty(globalThis, 'window', {
    configurable: true,
    value: {
      open: () => {
        openCalls += 1;
        return viewer;
      },
      setTimeout: (callback: () => void) => {
        callback();
        return 1;
      },
    },
  });
  URL.createObjectURL = () => 'blob:workflow-form-pdf';
  URL.revokeObjectURL = (value) => { revoked = value; };

  try {
    await openProtectedBlob(async () => new Blob(['%PDF-1.4'], { type: 'application/pdf' }));
    assert.equal(openCalls, 1);
    assert.equal(closeCalls, 0);
    assert.equal(viewer.opener, null);
    assert.equal(viewer.location.href, 'blob:workflow-form-pdf');
    assert.equal(revoked, 'blob:workflow-form-pdf');
  } finally {
    URL.createObjectURL = previousCreateObjectURL;
    URL.revokeObjectURL = previousRevokeObjectURL;
    if (previousWindow === undefined) delete (globalThis as { window?: unknown }).window;
    else Object.defineProperty(globalThis, 'window', { configurable: true, value: previousWindow });
  }
});

test('protected blob opener fails before download when the browser blocks the viewer', async () => {
  const previousWindow = (globalThis as { window?: unknown }).window;
  let loadCalls = 0;
  Object.defineProperty(globalThis, 'window', {
    configurable: true,
    value: { open: () => null },
  });

  try {
    await assert.rejects(
      openProtectedBlob(async () => {
        loadCalls += 1;
        return new Blob();
      }),
      /cho phép pop-up/,
    );
    assert.equal(loadCalls, 0);
  } finally {
    if (previousWindow === undefined) delete (globalThis as { window?: unknown }).window;
    else Object.defineProperty(globalThis, 'window', { configurable: true, value: previousWindow });
  }
});
