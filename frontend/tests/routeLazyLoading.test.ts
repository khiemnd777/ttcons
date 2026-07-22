import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

test('authenticated pages stay lazy and below the permission boundary', async () => {
  const source = await readFile(new URL('../src/app/router.tsx', import.meta.url), 'utf8');
  const lazyImports = source.match(/lazy\(\(\) => import\(/g) ?? [];
  const guardedFunction = source.match(/function guarded[\s\S]*?\n}\n\nexport const router/)?.[0] ?? '';

  assert.ok(lazyImports.length >= 20, `expected at least 20 lazy page imports, received ${lazyImports.length}`);
  assert.match(source, /import \{ LoginPage \} from '\.\.\/modules\/auth\/LoginPage';/);
  assert.doesNotMatch(source, /lazy\(\(\) => import\('\.\.\/modules\/auth\/LoginPage'\)/);
  assert.match(
    guardedFunction,
    /<RequirePermission permissions=\{permissions\}>[\s\S]*<RouteModuleBoundary focus=\{focus\}>\{element\}<\/RouteModuleBoundary>[\s\S]*<\/RequirePermission>/,
  );
});
