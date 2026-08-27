import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

async function source(path: string) {
  return readFile(new URL(path, import.meta.url), 'utf8');
}

test('standard shell exposes the permission-filtered navigation on mobile', async () => {
  const appShell = await source('../src/layouts/AppShell.tsx');
  const domainTypes = await source('../src/lib/types/domain.ts');

  assert.match(appShell, /const \[mobileNavOpen, setMobileNavOpen\] = useState\(false\)/);
  assert.match(appShell, /aria-controls="mobile-navigation"/);
  assert.match(appShell, /aria-expanded=\{mobileNavOpen\}/);
  assert.match(appShell, /id="mobile-navigation"/);
  assert.match(appShell, /aria-modal="true"/);
  assert.match(appShell, /event\.key === 'Escape'/);
  assert.match(appShell, /document\.body\.style\.overflow = 'hidden'/);
  assert.ok((appShell.match(/<AppNavigation/g) ?? []).length >= 2, 'desktop and mobile must share AppNavigation');
  assert.match(appShell, /to: '\/dashboard', label: 'Dashboard'/);
  const dashboardIndex = appShell.indexOf("{ to: '/dashboard'");
  const myWorkIndex = appShell.indexOf("{ to: '/my-work'");
  assert.ok(dashboardIndex >= 0 && myWorkIndex >= 0, 'Dashboard and Bàn làm việc must both be registered');
  assert.ok(dashboardIndex < myWorkIndex, 'Dashboard must appear before Bàn làm việc');
  assert.match(appShell, /to: '\/audit', label: 'Audit nghiệp vụ'.*permissions: \['audit\.view'\]/);
  assert.match(appShell, /to: '\/event-logs', label: 'Nhật ký sự kiện'.*permissions: \['event_log\.view'\]/);
  assert.match(appShell, /to: '\/accounting', label: 'Bàn giao kế toán'.*accounting_handoff\.view/);
  assert.match(appShell, /to: '\/core-data', label: 'Đối tác'.*permissions: \['catalog\.view'\]/);
  assert.doesNotMatch(appShell, /to: '\/dashboard', label: 'Điều hành'/);
  assert.match(appShell, /aria-label="Mở hồ sơ cá nhân"/);
  assert.ok((appShell.match(/navigate\('\/profile'\)/g) ?? []).length >= 2, 'desktop and mobile user surfaces must expose Profile');
  assert.match(appShell, /const userRoleSummary = roleSummary\(user\?\.roles\)/);
  assert.doesNotMatch(appShell, /user\?\.roles\?\.join\(', '\)/);
  assert.match(appShell, /<NotificationPanel summary=\{indicators\.notifications\.data\}/);
  assert.match(appShell, /'\/my-work': indicatorBadge\(indicators\.myWork\.data\?\.total/);
  assert.match(appShell, /'\/reports': indicatorBadge\(indicators\.workReports\.data\?\.total_visible/);
  assert.match(appShell, /badges=\{navigationBadges\}/);
  const myWorkNavigationType = domainTypes.slice(domainTypes.indexOf('export type MyWorkNavigationSummary'), domainTypes.indexOf('export type Project'));
  assert.match(myWorkNavigationType, /work_total: number/);
  assert.match(myWorkNavigationType, /approval_total: number/);
  assert.match(myWorkNavigationType, /work_reports_review_pending: number/);
  assert.doesNotMatch(myWorkNavigationType, /attention/);
});

test('focus sidebars reuse the approved home brand link', async () => {
  const brand = await source('../src/components/BrandLogo.tsx');
  const templateSidebar = await source('../src/modules/pipeline-builder/components/TemplateSidebar.tsx');
  const projectWorkspace = await source('../src/modules/projects/ProjectWorkspaceLayout.tsx');

  assert.match(brand, /export function BrandHomeLink/);
  assert.match(brand, /aria-label="Về trang mặc định"/);
  assert.match(templateSidebar, /<BrandHomeLink/);
  assert.match(projectWorkspace, /<BrandHomeLink/);
  assert.match(projectWorkspace, /const userRoleSummary = roleSummary\(user\?\.roles\)/);
  assert.match(projectWorkspace, /<NotificationPanel summary=\{indicators\.notifications\.data\}/);
  assert.doesNotMatch(projectWorkspace, /user\?\.roles\?\.join\(', '\)/);
});
