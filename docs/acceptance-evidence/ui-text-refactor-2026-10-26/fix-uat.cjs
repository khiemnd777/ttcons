const fs = require('node:fs'), path = require('node:path'), assert = require('node:assert/strict');
const { chromium } = require('/Users/khiemnguyen/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/node_modules/playwright');
const out = path.join(__dirname, process.env.CONSFLOW_FIX_OUT || 'fix-browser-final'); fs.mkdirSync(out, { recursive: true });
const origin = 'http://127.0.0.1:15173';
const project = '2885d835-e21e-4c9f-a194-16acaa776857';
const results = [], errors = []; const sleep = ms => new Promise(resolve => setTimeout(resolve, ms));
let browser, page, mode = 'real', writes = [], saved = {}, failSecond = false, delayedCatalog;
const fixture = ['Bắt buộc', 'Tùy chọn A', 'Tùy chọn B'].map((label, index) => ({ event_key: `uat.${index}`, domain_key: 'uat', category_key: `uat_${index}`, category_label: label, enabled: true, required_delivery: index === 0, permissions_any: [] }));
const tagline = 'Bắt đầu từ việc ưu tiên, luôn rõ Công trình › Giai đoạn › Gói công việc';
function persist() { fs.writeFileSync(path.join(out, 'results.json'), JSON.stringify({ results, errors, scope: 'Real local catalog/layout; controlled HTTP fixtures explicitly labelled for optional preference writes and failures' }, null, 2)); }
async function check(name, fn, scope = 'real local API') {
  if (process.env.CONSFLOW_FIX_FILTER && !process.env.CONSFLOW_FIX_FILTER.split(',').includes(name)) return;
  const row = { name, viewport: page.viewportSize(), scope };
  try { await fn(row); row.measurement = await geometry(); assert.ok(row.measurement.documentWidth <= row.viewport.width + 1, JSON.stringify(row.measurement)); row.status = 'PASS'; }
  catch (error) { row.status = 'FAIL'; row.error = error.message.split('\n').slice(0, 3).join('\n'); }
  row.screenshot = name + '.png'; await page.screenshot({ path: path.join(out, row.screenshot), animations: 'disabled', caret: 'hide' });
  results.push(row); persist(); console.log(name, row.status, row.error || '');
}
async function geometry() { return page.evaluate(() => ({ documentWidth: document.documentElement.scrollWidth, clientWidth: document.documentElement.clientWidth, layoutViewport: innerWidth })); }
async function navigate(route, heading) {
  for (let i = 0; i < 3 && await page.getByRole('dialog').count(); i++) { await page.keyboard.press('Escape'); await sleep(50); }
  await page.evaluate(route => { history.pushState({ ...history.state, idx: (history.state?.idx ?? 0) + 1 }, '', route); dispatchEvent(new PopStateEvent('popstate')); }, route);
  if (heading) await page.getByRole('heading', { name: heading, exact: true }).first().waitFor();
  await sleep(900); assert.notEqual(new URL(page.url()).pathname, '/login');
}
async function preferencesDialog() { await page.getByRole('button', { name: 'Tùy chọn', exact: true }).click(); await page.getByRole('dialog').waitFor(); }
async function reload(next) {
  mode = next; await sleep(12000);
  await page.goto(origin + '/notifications', { waitUntil: 'domcontentloaded' });
  await page.getByRole('heading', { name: 'Thông báo', exact: true }).waitFor();
  await preferencesDialog();
}
async function intercept(context) {
  const reply = (route, status, data) => route.fulfill({ status, contentType: 'application/json', body: JSON.stringify(status === 200 ? { success: true, data } : { success: false, error: { code: 'UAT_FIXTURE_ERROR', message: 'Lỗi kiểm thử có kiểm soát.' } }) });
  await context.route('**/api/me/notifications/catalog', async route => {
    if (mode === 'real') return route.continue();
    if (mode === 'catalog-error' || mode === 'both-error') return reply(route, 500);
    if (mode === 'loading') await new Promise(resolve => { delayedCatalog = resolve; });
    return reply(route, 200, mode === 'empty' ? [] : fixture);
  });
  await context.route('**/api/me/notifications/preferences', async route => {
    if (mode === 'real') return route.continue();
    if (route.request().method() === 'PUT') {
      const value = route.request().postDataJSON(); writes.push(value);
      await sleep(250);
      if (failSecond && value.category_key === 'uat_2') return reply(route, 500);
      saved[value.category_key] = value.in_app_enabled;
      return reply(route, 200, value);
    }
    if (mode === 'preferences-error' || mode === 'both-error') return reply(route, 500);
    return reply(route, 200, Object.entries(saved).map(([category_key, in_app_enabled]) => ({ category_key, event_key: '*', in_app_enabled })));
  });
}
(async () => {
  browser = await chromium.launch({ executablePath: '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome', headless: true, chromiumSandbox: true });
  const desktop = await browser.newContext({ viewport: { width: 1440, height: 1000 }, locale: 'vi-VN', timezoneId: 'Asia/Ho_Chi_Minh' });
  page = await desktop.newPage(); page.setDefaultTimeout(12000); page.on('pageerror', error => errors.push(error.message));
  await page.goto(origin + '/login');
  const c = JSON.parse(fs.readFileSync('/private/tmp/consflow-guide-local-20261022/operator-login.json', 'utf8'));
  await page.locator('input[name="email"]').fill(c.email); await page.locator('input[name="password"]').fill(c.password);
  await page.getByRole('button', { name: 'Đăng nhập', exact: true }).click(); await page.waitForURL('**/my-work');
  const token = await page.evaluate(() => localStorage.getItem('consflow_token'));
  await intercept(desktop);
  for (const [name, route, heading] of [['audit', '/audit', 'Audit toàn hệ thống'], ['events', '/event-logs', 'Nhật ký sự kiện'], ['discussion', `/projects/${project}/discussion`, undefined]]) await check('desktop-' + name, async () => {
    await navigate(route, heading);
  });
  await check('desktop-real-notification-catalog', async row => {
    await navigate('/notifications', 'Thông báo'); await preferencesDialog();
    await page.getByRole('dialog').getByRole('checkbox').first().waitFor();
    row.groups = await page.getByRole('dialog').getByRole('checkbox').count();
    row.allRequired = await page.getByRole('dialog').getByRole('checkbox').evaluateAll(nodes => nodes.every(node => node.disabled && node.checked));
    assert.equal(row.groups, 6); assert.equal(row.allRequired, true);
    assert.equal(await page.getByRole('button', { name: 'Lưu tùy chọn', exact: true }).isDisabled(), true);
    assert.equal(await page.getByText('Chưa có nhóm thông báo đang hoạt động.', { exact: true }).count(), 0);
  });
  for (const route of ['/workflow-forms', '/workflow-forms/', '/WORKFLOW-FORMS']) await check('desktop-protected-shell-' + (route.endsWith('/') ? 'trailing-slash' : route === '/WORKFLOW-FORMS' ? 'uppercase' : 'canonical'), async row => {
    row.route = route;
    await navigate(route); await page.getByText(tagline, { exact: true }).waitFor(); assert.equal(await page.getByText(tagline, { exact: true }).isVisible(), true);
  });
  await check('desktop-profile-copy', async () => {
    await navigate('/profile', 'Hồ sơ cá nhân');
    assert.equal(await page.getByText('Thông tin liên hệ và hồ sơ công việc hiện tại.', { exact: true }).count(), 0);
    assert.equal(await page.getByText('Kiểm soát mật khẩu và phiên đăng nhập.', { exact: true }).count(), 0);
    assert.equal(await page.getByText(tagline, { exact: true }).count(), 0);
  });
  for (const [next, text] of [['catalog-error', 'Không tải được danh mục thông báo.'], ['preferences-error', 'Không tải được tùy chọn đã lưu.'], ['both-error', 'Không tải được dữ liệu tùy chọn thông báo.']]) {
    await check('desktop-' + next, async () => {
      await reload(next); await page.getByRole('alert').filter({ hasText: text }).waitFor();
      assert.equal(await page.getByRole('button', { name: 'Lưu tùy chọn', exact: true }).isDisabled(), true);
      assert.equal(await page.getByText('Chưa có nhóm thông báo để cấu hình.', { exact: true }).count(), 0);
      mode = 'fixture'; await page.getByRole('button', { name: 'Thử lại', exact: true }).click();
      await page.getByRole('dialog').getByRole('checkbox').first().waitFor();
    }, 'controlled HTTP error/retry fixture');
  }
  await check('desktop-empty', async () => {
    await reload('empty'); await page.getByText('Chưa có nhóm thông báo để cấu hình.', { exact: true }).waitFor();
    assert.equal(await page.getByRole('button', { name: 'Lưu tùy chọn', exact: true }).count(), 0);
  }, 'controlled HTTP empty fixture');
  await check('desktop-loading', async () => {
    await reload('loading'); await page.getByText('Đang tải tùy chọn thông báo…', { exact: true }).waitFor();
    assert.equal(await page.getByRole('button', { name: 'Lưu tùy chọn', exact: true }).isDisabled(), true);
    mode = 'fixture'; delayedCatalog(); await page.getByRole('dialog').getByRole('checkbox').first().waitFor();
  }, 'controlled delayed catalog fixture');
  await check('desktop-partial-save-retry-and-reopen', async row => {
    writes = []; saved = {}; failSecond = true; await reload('fixture');
    const dialog = page.getByRole('dialog');
    await dialog.getByText('Tùy chọn A', { exact: true }).click(); await dialog.getByText('Tùy chọn B', { exact: true }).click();
    const save = page.getByRole('button', { name: 'Lưu tùy chọn', exact: true });
    await save.click();
    await dialog.getByRole('alert').waitFor(); await page.getByRole('button', { name: 'Đóng thông báo', exact: true }).first().click();
    assert.equal(await dialog.count(), 1); assert.equal(writes.length, 2);
    failSecond = false; await save.click(); await dialog.waitFor({ state: 'hidden' });
    assert.deepEqual(writes.map(value => value.category_key), ['uat_1', 'uat_2', 'uat_2']); row.writes = writes.map(value => ({ category: value.category_key, enabled: value.in_app_enabled }));
    await preferencesDialog(); assert.equal(await page.getByRole('dialog').getByRole('alert').count(), 0);
    assert.equal(await page.getByRole('button', { name: 'Lưu tùy chọn', exact: true }).isDisabled(), true);
  }, 'controlled optional-group GET/PUT fixture; no preference DB writes');
  await desktop.close(); mode = 'real'; await sleep(12000);
  const mobile = await browser.newContext({ viewport: { width: 390, height: 844 }, isMobile: true, hasTouch: true, locale: 'vi-VN', timezoneId: 'Asia/Ho_Chi_Minh' });
  await mobile.addInitScript(value => localStorage.setItem('consflow_token', value), token);
  await intercept(mobile); page = await mobile.newPage(); page.setDefaultTimeout(12000); page.on('pageerror', error => errors.push(error.message));
  await page.goto(origin + '/my-work'); await page.locator('h1').first().waitFor();
  for (const width of [360, 390, 430]) {
    await page.setViewportSize({ width, height: 844 });
    for (const [name, route, heading] of [['audit', '/audit', 'Audit toàn hệ thống'], ['events', '/event-logs', 'Nhật ký sự kiện'], ['discussion', `/projects/${project}/discussion`, undefined]]) await check(`mobile-${width}-${name}`, async row => {
      await navigate(route, heading);
      if (name !== 'discussion') {
        const projectSelect = page.locator('select').filter({ has: page.getByRole('option', { name: 'Tất cả công trình', exact: true }) }).first();
        await projectSelect.waitFor();
        const available = await projectSelect.locator('option').evaluateAll(nodes => nodes.filter(node => node.value).map(node => ({ value: node.value, label: node.textContent })).sort((a, b) => b.label.length - a.label.length));
        assert.ok(available.length, 'real project filter options must be loaded');
        await projectSelect.selectOption(available[0].value); await sleep(700); await projectSelect.scrollIntoViewIfNeeded();
        row.selectedProject = await projectSelect.inputValue(); row.optionLabelLength = available[0].label.length;
        assert.equal(row.selectedProject, available[0].value);
      } else {
        await page.getByRole('button', { name: /Việc của tôi/ }).first().tap();
        const strip = page.locator('div.overflow-x-auto').filter({ hasText: 'Chưa đọc' }).first();
        row.strip = await strip.evaluate(el => { el.scrollLeft = el.scrollWidth; return { client: el.clientWidth, scroll: el.scrollWidth, position: el.scrollLeft }; });
        assert.ok(row.strip.position > 0, 'attention strip must scroll locally');
        await page.getByRole('button', { name: /Việc của tôi/ }).first().tap();
      }
    });
    await check(`mobile-${width}-notification-real`, async row => {
      await navigate('/notifications', 'Thông báo'); await preferencesDialog(); await page.getByRole('dialog').getByRole('checkbox').first().waitFor();
      assert.equal(await page.getByRole('dialog').getByRole('checkbox').count(), 6);
      row.dialog = await page.getByRole('dialog').boundingBox();
      assert.ok(row.dialog.x >= 0 && row.dialog.x + row.dialog.width <= width);
      row.openScreenshot = `mobile-${width}-notification-real-open.png`;
      await page.screenshot({ path: path.join(out, row.openScreenshot), animations: 'disabled' });
      await page.keyboard.press('Escape'); await page.getByRole('dialog').waitFor({ state: 'hidden' });
    });
    if (width === 390) await check('mobile-390-profile-copy', async () => {
      await navigate('/profile', 'Hồ sơ cá nhân');
      assert.equal(await page.getByText('Thông tin liên hệ và hồ sơ công việc hiện tại.', { exact: true }).count(), 0);
      assert.equal(await page.getByText('Kiểm soát mật khẩu và phiên đăng nhập.', { exact: true }).count(), 0);
    });
  }
  await check('mobile-430-error-and-retry', async () => {
    await reload('catalog-error'); await page.getByText('Không tải được danh mục thông báo.', { exact: true }).waitFor();
    assert.equal(await page.getByRole('button', { name: 'Lưu tùy chọn', exact: true }).isDisabled(), true);
    mode = 'fixture'; await page.getByRole('button', { name: 'Thử lại', exact: true }).tap(); await page.getByRole('dialog').getByRole('checkbox').first().waitFor();
  }, 'mobile touch + controlled HTTP error/retry fixture');
  await browser.close(); persist(); console.log(JSON.stringify({ total: results.length, pass: results.filter(row => row.status === 'PASS').length, failures: results.filter(row => row.status !== 'PASS'), errors }));
})().catch(async error => { console.error(error.message); errors.push('runner: ' + error.message); persist(); if (browser) await browser.close(); process.exitCode = 1; });
