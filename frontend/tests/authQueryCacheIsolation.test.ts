import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

async function source(path: string) {
  return readFile(new URL(path, import.meta.url), 'utf8');
}

test('auth identity boundaries clear all actor-owned React Query data', async () => {
  const auth = await source('../src/lib/auth/AuthContext.tsx');

  assert.match(auth, /const queryClient = useQueryClient\(\)/);
  assert.match(auth, /const replaceAuthenticatedIdentity = useCallback\(\(nextUser: User \| null\) => \{\s*queryClient\.clear\(\);\s*setUser\(nextUser\);/);
  assert.match(auth, /setAuthFailureHandler\(\(\) => \{\s*replaceAuthenticatedIdentity\(null\);/);
  assert.match(auth, /api<User>\('\/auth\/me'\)\s*\.then\(replaceAuthenticatedIdentity\)/);
  assert.match(auth, /setToken\(data\.token\);\s*replaceAuthenticatedIdentity\(data\.user\);/);
  assert.match(auth, /await logoutRequest\(\);\s*replaceAuthenticatedIdentity\(null\);/);
  assert.match(auth, /\.catch\(\(\) => \{\s*setToken\(null\);\s*replaceAuthenticatedIdentity\(null\);/);
});
