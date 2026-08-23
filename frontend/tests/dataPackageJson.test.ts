import assert from 'node:assert/strict';
import test from 'node:test';
import { assertExactKeys, parseStrictJson } from '../src/modules/_shared/strictJson.ts';

test('strict JSON accepts one ordinary document', () => {
  assert.deepEqual(parseStrictJson('{"schema":"consflow.project-package","projects":[]}'), {
    schema: 'consflow.project-package',
    projects: [],
  });
});

test('strict JSON rejects duplicate, dangerous and trailing content', () => {
  assert.throws(() => parseStrictJson('{"schema":"one","schema":"two"}'), /key bị trùng/);
  assert.throws(() => parseStrictJson('{"__proto__":{}}'), /key không được phép/);
  assert.throws(() => parseStrictJson('{} {}'), /một document/);
});

test('exact-key validation rejects fields outside the versioned schema', () => {
  assert.throws(() => assertExactKeys({ schema: 'v1', unexpected: true }, ['schema'], 'package'), /unexpected/);
});
