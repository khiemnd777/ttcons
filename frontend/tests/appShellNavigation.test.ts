import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

async function source(path: string) {
  return readFile(new URL(path, import.meta.url), 'utf8');
}

test('standard shell exposes the permission-filtered navigation on mobile', async () => {
  const appShell = await source('../src/layouts/AppShell.tsx');

  assert.match(appShell, /const \[mobileNavOpen, setMobileNavOpen\] = useState\(false\)/);
  assert.match(appShell, /aria-controls="mobile-navigation"/);
  assert.match(appShell, /aria-expanded=\{mobileNavOpen\}/);
  assert.match(appShell, /id="mobile-navigation"/);
  assert.match(appShell, /aria-modal="true"/);
  assert.match(appShell, /event\.key === 'Escape'/);
  assert.match(appShell, /document\.body\.style\.overflow = 'hidden'/);
  assert.ok((appShell.match(/<AppNavigation/g) ?? []).length >= 2, 'desktop and mobile must share AppNavigation');
  assert.match(appShell, /to: '\/dashboard', label: 'Dashboard'/);
  assert.doesNotMatch(appShell, /to: '\/dashboard', label: 'Điều hành'/);
  assert.match(appShell, /aria-label="Mở hồ sơ cá nhân"/);
  assert.ok((appShell.match(/navigate\('\/profile'\)/g) ?? []).length >= 2, 'desktop and mobile user surfaces must expose Profile');
});

test('focus sidebars reuse the approved home brand link', async () => {
  const brand = await source('../src/components/BrandLogo.tsx');
  const templateSidebar = await source('../src/modules/pipeline-builder/components/TemplateSidebar.tsx');
  const projectWorkspace = await source('../src/modules/projects/ProjectWorkspaceLayout.tsx');

  assert.match(brand, /export function BrandHomeLink/);
  assert.match(brand, /aria-label="Về trang mặc định"/);
  assert.match(templateSidebar, /<BrandHomeLink/);
  assert.match(projectWorkspace, /<BrandHomeLink/);
});
