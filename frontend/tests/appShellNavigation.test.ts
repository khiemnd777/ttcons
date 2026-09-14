import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';
import { domainTypesSource } from './domainTypesSource.ts';

async function source(path: string) {
  return readFile(new URL(path, import.meta.url), 'utf8');
}

test('standard shell exposes the permission-filtered navigation on mobile', async () => {
  const appShell = await source('../src/layouts/AppShell.tsx');
  const domainTypes = await domainTypesSource();

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
  assert.match(appShell, /to: '\/procurement', label: 'Mua sắm vật tư'.*permissions: \['procurement\.view'\]/);
  const projectIndex = appShell.indexOf("{ to: '/projects', label: 'Công trình'");
  const procurementIndex = appShell.indexOf("{ to: '/procurement', label: 'Mua sắm vật tư'");
  const reportIndex = appShell.indexOf("{ to: '/reports', label: 'Báo cáo'");
  assert.ok(projectIndex < procurementIndex && procurementIndex < reportIndex, 'Mua sắm vật tư must sit below Công trình');
  assert.match(appShell, /to: '\/audit', label: 'Audit nghiệp vụ'.*permissions: \['audit\.view'\]/);
  assert.match(appShell, /to: '\/event-logs', label: 'Nhật ký sự kiện'.*permissions: \['event_log\.view'\]/);
  assert.match(appShell, /to: '\/accounting', label: 'Bàn giao kế toán'.*accounting_handoff\.view/);
  assert.match(appShell, /to: '\/core-data', label: 'Đối tác'.*permissions: \['catalog\.view'\]/);
  assert.match(appShell, /to: '\/warehouse\?tab=materials', label: 'Vật tư'.*permissions: \['material\.view'\].*excludePermissions: \['warehouse\.view'\]/);
  assert.match(appShell, /to: '\/warehouse', label: 'Kho vật tư'.*permissions: \['warehouse\.view'\]/);
  const partnerIndex = appShell.indexOf("{ to: '/core-data', label: 'Đối tác'");
  const materialIndex = appShell.indexOf("{ to: '/warehouse?tab=materials', label: 'Vật tư'");
  const warehouseIndex = appShell.indexOf("{ to: '/warehouse', label: 'Kho vật tư'");
  const staffIndex = appShell.indexOf("{ to: '/staff', label: 'Nhân sự'");
  assert.ok(partnerIndex >= 0 && materialIndex >= 0 && warehouseIndex >= 0 && staffIndex >= 0, 'Đối tác, Vật tư, Kho vật tư and Nhân sự must all be registered');
  assert.ok(partnerIndex < materialIndex && materialIndex < warehouseIndex && warehouseIndex < staffIndex, 'Material-only navigation must sit between Đối tác and Kho vật tư');
  assert.match(appShell, /to: '\/core-data', label: 'Đối tác'[^\n]*\n\s*\{ to: '\/warehouse\?tab=materials', label: 'Vật tư'[^\n]*\n\s*\{ to: '\/warehouse', label: 'Kho vật tư'/);
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
  assert.match(myWorkNavigationType, /accounting_handoffs_pending: number/);
  assert.match(myWorkNavigationType, /accounting_total: number/);
  assert.doesNotMatch(myWorkNavigationType, /attention/);
});

test('accounting reconciliation mode excludes technical and administration navigation', async () => {
  const appShell = await source('../src/layouts/AppShell.tsx');
  const router = await source('../src/app/router.tsx');

  assert.match(appShell, /type NavItem = \{[\s\S]*excludePermissions\?: string\[\]/);
  for (const path of ['/pipeline-builder', '/workflow-forms', '/staff', '/roles', '/audit', '/event-logs', '/settings']) {
    const escapedPath = path.replace('/', '\\/');
    assert.match(appShell, new RegExp(`to: '${escapedPath}'[^\\n]*excludePermissions: \\['project\\.reconciliation\\.view'\\]`));
  }
  assert.match(appShell, /hasAnyPermission\(item\.permissions\)[\s\S]*item\.excludePermissions/);
  assert.match(router, /excludePermissions\?\.length && hasAnyPermission\(excludePermissions\)/);
  assert.match(router, /path: 'audit'.*project\.reconciliation\.view/);
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
