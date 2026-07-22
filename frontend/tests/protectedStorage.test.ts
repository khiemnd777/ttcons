import assert from 'node:assert/strict';
import test from 'node:test';
import { isProtectedStorageProvider } from '../src/lib/utils/protectedStorage.ts';

test('only internal local and s3 providers use protected file endpoints', () => {
  assert.equal(isProtectedStorageProvider('local'), true);
  assert.equal(isProtectedStorageProvider('s3'), true);

  for (const provider of ['external', 'none', 'unknown', 'S3', '', undefined, null]) {
    assert.equal(isProtectedStorageProvider(provider), false, String(provider));
  }
});
