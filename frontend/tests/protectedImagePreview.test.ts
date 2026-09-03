import assert from 'node:assert/strict';
import test from 'node:test';
import { protectedImageError, startProtectedImagePreview, type ProtectedImagePreviewState } from '../src/lib/utils/protectedImagePreview.ts';

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason: unknown) => void;
  const promise = new Promise<T>((yes, no) => { resolve = yes; reject = no; });
  return { promise, resolve, reject };
}

function objectURLs() {
  const created: Blob[] = [];
  const revoked: string[] = [];
  return {
    created, revoked,
    api: {
      createObjectURL: (blob: Blob) => { created.push(blob); return `blob:preview-${created.length}`; },
      revokeObjectURL: (url: string) => { revoked.push(url); },
    },
  };
}

test('a protected image stays loading until bytes arrive and releases its URL exactly once', async () => {
  const response = deferred<Blob>();
  const states: ProtectedImagePreviewState[] = [];
  const urls = objectURLs();
  const dispose = startProtectedImagePreview(() => response.promise, (state) => states.push(state), urls.api);
  assert.deepEqual(states, [{ status: 'loading' }]);
  const blob = new Blob(['signature'], { type: 'image/png' });
  response.resolve(blob);
  await response.promise;
  assert.deepEqual(states.at(-1), { status: 'ready', url: 'blob:preview-1' });
  assert.equal(urls.created[0], blob);
  dispose();
  dispose();
  assert.deepEqual(urls.revoked, ['blob:preview-1']);
});

test('closing during download suppresses a late image and creates no orphan URL', async () => {
  const response = deferred<Blob>();
  const states: ProtectedImagePreviewState[] = [];
  const urls = objectURLs();
  const dispose = startProtectedImagePreview(() => response.promise, (state) => states.push(state), urls.api);
  dispose();
  response.resolve(new Blob(['late image']));
  await response.promise;
  assert.deepEqual(states, [{ status: 'loading' }]);
  assert.deepEqual(urls.created, []);
});

test('switching a record or actor cannot replace the next preview with a late response', async () => {
  const previous = deferred<Blob>();
  const current = deferred<Blob>();
  const states: ProtectedImagePreviewState[] = [];
  const urls = objectURLs();
  const disposePrevious = startProtectedImagePreview(() => previous.promise, (state) => states.push(state), urls.api);
  disposePrevious();
  const disposeCurrent = startProtectedImagePreview(() => current.promise, (state) => states.push(state), urls.api);
  const currentBlob = new Blob(['current actor image']);
  current.resolve(currentBlob);
  await current.promise;
  previous.resolve(new Blob(['previous actor image']));
  await previous.promise;
  assert.deepEqual(urls.created, [currentBlob]);
  assert.deepEqual(states.at(-1), { status: 'ready', url: 'blob:preview-1' });
  disposeCurrent();
});

test('late failures after close are ignored', async () => {
  const response = deferred<Blob>();
  const states: ProtectedImagePreviewState[] = [];
  const dispose = startProtectedImagePreview(() => response.promise, (state) => states.push(state));
  dispose();
  response.reject(new Error('private storage path'));
  await response.promise.catch(() => {});
  assert.deepEqual(states, [{ status: 'loading' }]);
});

test('permission and missing-file errors are distinct from retryable transport failures', () => {
  for (const status of [401, 403]) {
    assert.deepEqual(protectedImageError({ status, message: 'sensitive detail' }), {
      status: 'error', message: 'Bạn không có quyền xem dấu ký này.', retryable: false,
    });
  }
  assert.equal(protectedImageError({ status: 404 }).retryable, false);
  assert.match(protectedImageError({ status: 404 }).message, /Không tìm thấy/);
  assert.deepEqual(protectedImageError(new Error('private storage path')), {
    status: 'error', message: 'Không tải được dấu ký.', retryable: true,
  });
});

test('a retry starts clean and can recover from a transport failure', async () => {
  const states: ProtectedImagePreviewState[] = [];
  const urls = objectURLs();
  const failed = deferred<Blob>();
  const disposeFailure = startProtectedImagePreview(() => failed.promise, (state) => states.push(state), urls.api);
  failed.reject(new Error('offline'));
  await failed.promise.catch(() => {});
  assert.equal(states.at(-1)?.status, 'error');
  disposeFailure();
  const retry = deferred<Blob>();
  const disposeRetry = startProtectedImagePreview(() => retry.promise, (state) => states.push(state), urls.api);
  assert.deepEqual(states.at(-1), { status: 'loading' });
  retry.resolve(new Blob(['retried image']));
  await retry.promise;
  assert.equal(states.at(-1)?.status, 'ready');
  disposeRetry();
  assert.equal(urls.revoked.length, 1);
});
