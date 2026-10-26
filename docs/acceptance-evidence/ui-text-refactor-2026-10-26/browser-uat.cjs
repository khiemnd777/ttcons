// Browser checks against the local API and existing synthetic guide records. No business submit.
const fs = require('node:fs');
const path = require('node:path');
const assert = require('node:assert/strict');
const { chromium } = require('/Users/khiemnguyen/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/node_modules/playwright');
const ROOT = path.resolve(__dirname, '../../..');
const OUT = process.env.CONSFLOW_UAT_OUT || path.join(__dirname, 'browser');
fs.mkdirSync(OUT, { recursive: true });
const CURRENT = 'http://127.0.0.1:15173', BASELINE = 'http://127.0.0.1:15174';
const API = 'http://127.0.0.1:18081/api';
const PROJECT = '2885d835-e21e-4c9f-a194-16acaa776857';
const DONE = 'f3d6a673-78f1-4fff-9830-9831478c99e1';
const results = [], errors = [], comparisons = [];
let page, browser, token;
const sleep = ms => new Promise(resolve => setTimeout(resolve, ms));
const comparisonDelay = Number(process.env.CONSFLOW_UAT_COMPARISON_DELAY || 6500);
async function api(route) {
  const response = await fetch(API + route, { headers: { Authorization: 'Bearer ' + token } });
  const body = await response.json();
  if (!response.ok) throw new Error(`API ${route}: ${response.status}`);
  return body.data;
}
async function ready() {
  await page.locator('h1,h2,h3,[role="dialog"]').first().waitFor({ timeout: 15000 });
  await sleep(500);
  await page.evaluate(() => document.fonts.ready);
}
async function navigate(route) {
  for (let attempt = 0; attempt < 4 && await page.getByRole('dialog').count(); attempt++) {
    await page.keyboard.press('Escape'); await sleep(100);
  }
  if (page.url().startsWith(CURRENT) && new URL(page.url()).pathname !== '/login' && route !== '/login') {
    const link = page.locator(`a[href="${route}"]`).first();
    if (await link.count() && await link.isVisible()) await link.click();
    else await page.evaluate(value => { history.pushState({ ...history.state, idx: (history.state?.idx ?? 0) + 1 }, '', value); dispatchEvent(new PopStateEvent('popstate')); }, route);
  } else await page.goto(CURRENT + route, { waitUntil: 'domcontentloaded' });
  await ready();
  if (route !== '/login') assert.notEqual(new URL(page.url()).pathname, '/login', 'Session lost; this is not a valid page acceptance');
}
async function measure() {
  const measurement = await page.evaluate(() => {
    const root = document.documentElement;
    const dialog = [...document.querySelectorAll('[role="dialog"]')].at(-1);
    const bounds = dialog?.getBoundingClientRect();
    return { layoutViewport: innerWidth, clientWidth: root.clientWidth, documentWidth: root.scrollWidth,
      dialog: bounds ? { left: bounds.left, right: bounds.right, top: bounds.top, bottom: bounds.bottom, width: bounds.width } : null,
      heading: [...document.querySelectorAll('h1,h2,h3')].map(x => x.textContent.trim()).slice(0,18),
      alerts: [...document.querySelectorAll('[role="alert"]')].map(x => x.textContent.trim()).filter(Boolean).slice(0,5) };
  });
  return { ...measurement, viewport: page.viewportSize().width };
}
async function baselineGeometry(context,route) {
  await sleep(6500);const previous=await context.newPage();previous.setDefaultTimeout(10000);
  await previous.addInitScript(value=>localStorage.setItem('consflow_token',value),token);
  await previous.goto(BASELINE+route,{waitUntil:'domcontentloaded'});await previous.locator('h1,h2,h3').first().waitFor();await sleep(1000);
  const result=await previous.evaluate(()=>({layoutViewport:innerWidth,clientWidth:document.documentElement.clientWidth,documentWidth:document.documentElement.scrollWidth}));
  await previous.close();return {...result,viewport:390};
}
async function capture(name, fullPage = false) {
  const file = `${name}.png`;
  await page.screenshot({ path: path.join(OUT, file), fullPage, animations: 'disabled', caret: 'hide' });
  return file;
}
async function scenario(view, name, run) {
  const filters = process.env.CONSFLOW_UAT_FILTER?.split(',');
  if (filters && name !== 'login' && !filters.includes(name) && !filters.includes(`${view}.${name}`)) return;
  const row = { view, name, evidence_dir: path.relative(__dirname, OUT) };
  try {
    await run(row);
    row.measurement = await measure();
    row.screenshot = await capture(`${view}-${name}`);
    if(row.measurement.documentWidth > row.measurement.viewport + 1)throw new Error(`LAYOUT_FAIL: document overflow ${row.measurement.documentWidth}/${row.measurement.viewport}`);
    if (row.measurement.dialog) {
      const d = row.measurement.dialog;
      assert.ok(d.left >= -1 && d.right <= row.measurement.viewport + 1, `dialog outside viewport ${d.left}..${d.right}`);
    }
    row.status = 'PASS';
  } catch (error) {
    row.status = error.message.startsWith('NOT_RUN:') ? 'NOT_RUN' : error.message.startsWith('API_BLOCKED:') ? 'BLOCKED' : error.message.startsWith('LAYOUT_FAIL:') ? 'FAIL' : 'CHECK_REQUIRED'; row.error = error.message.split('\n').slice(0,4).join('\n');
    try { row.measurement = await measure(); row.screenshot = await capture(`${view}-${name}-check`); } catch {}
  }
  results.push(row); console.log(`${view} ${name}: ${row.status}`);
  fs.writeFileSync(path.join(OUT, 'results.json'), JSON.stringify({ results, errors, comparisons }, null, 2));
}
async function closeDialog() {
  const dialog = page.getByRole('dialog').last();
  await dialog.getByRole('button', { name: 'Đóng', exact: true }).first().click();
  await sleep(100);
}
async function login(credentials) {
  for(let attempt=0;attempt<4;attempt++) {
    await navigate('/login');
    await page.locator('input[name="email"]').fill(credentials.email);
    await page.locator('input[name="password"]').fill(credentials.password);
    const responsePromise=page.waitForResponse(response=>response.url().endsWith('/auth/login'));
    await page.getByRole('button', { name: 'Đăng nhập', exact: true }).click();
    const response=await responsePromise;
    if(response.status()===429) { const seconds=Number(response.headers()['retry-after'])||10;console.log(`Respecting login Retry-After: ${seconds}s`);for(let left=seconds;left>0;left-=5)await sleep(Math.min(5,left)*1000);continue; }
    assert.ok(response.ok(),`Login HTTP ${response.status()}`);
    await page.waitForURL('**/my-work'); await ready();
    token = await page.evaluate(() => localStorage.getItem('consflow_token'));return;
  }
  throw new Error('NOT_RUN: authentication rate limit still active after respecting Retry-After');
}
async function helpTest(row, name, inDialog = false, touch = false) {
  const button = page.getByRole('button', { name, exact: true });
  await button.scrollIntoViewIfNeeded();
  if (touch) await button.tap(); else { await button.focus(); await page.keyboard.press('Enter'); }
  await assert.equal(await button.getAttribute('aria-expanded'), 'true');
  const target = button.locator('..').locator('div[id]');
  await target.waitFor({ state: 'visible' });
  row.helpText = await target.textContent();
  row.openScreenshot = await capture(`${row.view}-help-${name.replace(/[^a-zA-Z0-9]/g, '').slice(0,35)}`);
  await page.keyboard.press('Escape');
  assert.equal(await button.getAttribute('aria-expanded'), 'false');
  assert.equal(await button.evaluate(element => document.activeElement === element), true);
  if (inDialog) assert.ok(await page.getByRole('dialog').count(), 'Escape must leave parent dialog open');
  if (touch) await button.tap(); else await button.click();
  await (inDialog ? page.getByRole('dialog').last().locator('h2').first() : page.locator('h1,h2').first()).click({ force: true });
  assert.equal(await button.getAttribute('aria-expanded'), 'false');
  await button.focus(); await page.keyboard.press('Space');
  assert.equal(await button.getAttribute('aria-expanded'), 'true');
  await page.keyboard.press('Tab');
  assert.equal(await button.getAttribute('aria-expanded'), 'false');
}
const routes = [
  ['dashboard','/dashboard'],['my-work','/my-work'],['progress','/progress'],['projects','/projects'],
  ['project-overview',`/projects/${PROJECT}`],['preparation',`/projects/${PROJECT}/preparation`],
  ['planning',`/projects/${PROJECT}/planning`],['planning-daily','/planning/daily-reports'],['meeting-minutes','/planning/meeting-minutes'],['planning-checklists','/planning/checklists'],
  ['boq',`/projects/${PROJECT}/boq`],['procurement','/procurement'],['procurement-scoped',`/projects/${PROJECT}/procurement`],['contracts',`/projects/${PROJECT}/contracts`],
  ['warehouse','/warehouse'],['operations',`/projects/${PROJECT}/operations`],['checks',`/projects/${PROJECT}/operations?tab=checks`],['actions',`/projects/${PROJECT}/operations?tab=actions`],
  ['rework','/rework'],['acceptance',`/projects/${DONE}/acceptance`],['handover',`/projects/${DONE}/handover`],
  ['accounting',`/projects/${DONE}/accounting`],['partners-input','/core-data/input'],['partners-output','/core-data/output'],
  ['staff','/staff'],['roles','/roles'],['permissions','/permissions'],['audit','/audit'],['event-logs','/event-logs'],
  ['statistics','/statistics'],['reports','/reports'],['notifications','/notifications'],['profile','/profile'],['settings','/settings'],
];
(async () => {
  browser = await chromium.launch({ executablePath: '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome', headless: true, chromiumSandbox: true });
  const credentials = JSON.parse(fs.readFileSync(`/private/tmp/consflow-guide-local-20261022/${process.env.CONSFLOW_UAT_ACTOR === 'developer' ? 'setup' : 'operator'}-login.json`, 'utf8'));
  const seeds = {};
  for (const view of process.env.CONSFLOW_UAT_VIEW ? [process.env.CONSFLOW_UAT_VIEW] : ['desktop','mobile']) {
    const context = await browser.newContext({ viewport: view === 'desktop' ? { width:1440,height:1000 } : { width:390,height:844 }, isMobile:view==='mobile',hasTouch:view==='mobile',deviceScaleFactor:1,locale:'vi-VN',timezoneId:'Asia/Ho_Chi_Minh' });
    page = await context.newPage();
    page.setDefaultTimeout(8000);
    page.on('pageerror', error => errors.push({ view, route:new URL(page.url()).pathname, message:error.message }));
    await scenario(view,'login',async () => { await login(credentials); });
    seeds[view] = token;
    for (const [name,route] of routes) await scenario(view,name,async row => {
      await navigate(route);
      row.route = new URL(page.url()).pathname;
      assert.equal(await page.getByText('Không có quyền truy cập', { exact:true }).count(),0);
      row.leftoverCopy = await page.getByText('Bắt đầu từ việc ưu tiên, luôn rõ Công trình › Giai đoạn › Gói công việc', {exact:true}).count();
      if(view==='mobile'&&['audit','event-logs'].includes(name))row.baselineGeometry=await baselineGeometry(context,route);
    });
    await scenario(view,'profile-edit-dialog',async () => { await navigate('/profile');await page.getByRole('button',{name:'Chỉnh sửa thông tin',exact:true}).click();await page.getByRole('dialog').waitFor();assert.equal(await page.getByText('Cập nhật thông tin tài khoản và liên hệ cá nhân.',{exact:true}).count(),0);await page.getByRole('dialog').getByLabel('Họ và tên',{exact:false}).waitFor(); });
    await scenario(view,'profile-avatar-help',async row => { await navigate('/profile');await page.getByRole('button',{name:'Thay ảnh đại diện',exact:true}).click();await page.getByRole('dialog').waitFor();await helpTest(row,'Về cách lưu ảnh đại diện',true,view==='mobile');assert.ok(await page.getByText(/tối đa 2 MB/).isVisible()); });
    await scenario(view,'profile-password-dialog',async () => { await navigate('/profile');await page.getByRole('button',{name:'Đổi mật khẩu',exact:true}).click();await page.getByRole('dialog').waitFor(); });
    await scenario(view,'statistics-help',async row => { await navigate('/statistics');const name=await page.locator('button[aria-controls][aria-label^="Cách tính và phạm vi:"]').getAttribute('aria-label');await helpTest(row,name,false,view==='mobile'); });
    await scenario(view,'accounting-help',async row => { await navigate(`/projects/${DONE}/accounting`);await helpTest(row,'Phạm vi hồ sơ bàn giao kế toán',false,view==='mobile'); });
    await scenario(view,'rework-help',async row => { await navigate('/rework');await helpTest(row,'Phân biệt sửa lỗi và lập kế hoạch lại',false,view==='mobile'); });
    await scenario(view,'notifications-dialog',async () => { await navigate('/notifications');await page.getByRole('button',{name:'Tùy chọn',exact:true}).click();await page.getByRole('dialog').waitFor(); });
    await scenario(view,'notifications-loaded',async row => {
      await navigate('/notifications');
      await page.getByRole('button',{name:'Tùy chọn',exact:true}).click();await sleep(1200);
      const response=await fetch(API+'/me/notifications/catalog',{headers:{Authorization:'Bearer '+token}});
      row.catalogStatus=response.status;row.catalogRequestId=response.headers.get('x-request-id');
      row.dialogText=await page.getByRole('dialog').innerText();
      if(!response.ok)throw new Error(`API_BLOCKED: notification catalog HTTP ${response.status}; unchanged local backend image`);
    });
    await scenario(view,'staff-import-dialog',async row => {
      await navigate('/staff');
      row.actor = process.env.CONSFLOW_UAT_ACTOR || 'operator';
      if (!await page.getByRole('button',{name:'Import',exact:true}).count()) throw new Error('NOT_RUN: existing local roles do not expose staff.import; no permission grant made');
      await page.getByRole('button',{name:'Import',exact:true}).click();await page.getByRole('dialog').waitFor();
    });
    await scenario(view,'staff-create-dialog',async () => { await navigate('/staff');await page.getByRole('button',{name:'Nhân sự',exact:true}).click();await page.getByRole('dialog').waitFor(); });
    await scenario(view,'settings-seal-dialog',async () => { await navigate('/settings');await page.getByRole('button',{name:'Thêm con dấu',exact:true}).click();await page.getByRole('dialog').waitFor(); });
    await scenario(view,'project-import-dialog',async () => { await navigate('/projects');await page.getByRole('button',{name:'Nhập dữ liệu',exact:true}).click();await page.getByRole('dialog').waitFor(); });
    await scenario(view,'project-export-dialog',async () => { await navigate('/projects');await page.getByRole('button',{name:'Xuất dữ liệu',exact:true}).click();await page.getByRole('dialog').waitFor(); });
    await scenario(view,'boq-create-dialog',async () => { await navigate(`/projects/${PROJECT}/boq`);await page.getByRole('button',{name:'Thêm dòng dự toán',exact:true}).click();await page.getByRole('dialog').waitFor(); });
    await scenario(view,'acceptance-create-dialog',async () => { await navigate(`/projects/${DONE}/acceptance`);await page.getByRole('button',{name:'Tạo biên bản',exact:true}).click();await page.getByRole('dialog').waitFor(); });
    await scenario(view,'rework-create-dialog',async () => { await navigate('/rework');await page.locator('button[role="combobox"]').first().click();const picker=page.locator('input[role="combobox"]').first();await picker.fill('DEMO-PLAN');await sleep(400);await page.getByRole('option').first().click();await page.getByRole('button',{name:'Sửa lỗi',exact:true}).first().click();await page.getByRole('dialog').waitFor();assert.ok(await page.getByText('Dùng khi xử lý trong phạm vi hiện tại.',{exact:true}).isVisible()); });
    await scenario(view,'report-review-rail',async row => { await navigate('/reports?scope=all&report=4721d3dd-054c-43fa-9bf5-b75bda75f439');await page.getByRole('dialog').waitFor();await page.getByRole('heading',{name:'Bản gửi',exact:true}).waitFor();assert.equal(await page.getByText('Ưu tiên đọc kết quả trong ngày, đối chiếu minh chứng hiện trường, sau đó xác nhận đã xem hoặc trả lại với nội dung cần bổ sung.',{exact:true}).count(),0);row.businessWarningPresent=await page.getByText(/chỉ xác nhận đã tiếp nhận báo cáo, không phải nghiệm thu chất lượng/).isVisible();assert.ok(row.businessWarningPresent); });
    await scenario(view,'report-detail-rail',async row => { await navigate(`/projects/0c99140a-ba85-46b8-8e00-dd0c7934c2b4/operations?report=4721d3dd-054c-43fa-9bf5-b75bda75f439`);await page.getByRole('dialog').waitFor();await page.getByRole('heading',{name:'Chuỗi liên quan',exact:true}).waitFor();assert.equal(await page.getByText('Nhịp thi công trong ngày được tách rõ thành kết quả, việc đang tiếp diễn, kế hoạch tiếp theo và blocker cần xử lý.',{exact:true}).count(),0);row.redundantCopyAbsent=true; });
    await scenario(view,'project-discussion-rail',async row => { await navigate(`/projects/${PROJECT}/discussion`);const rail=page.getByRole('region',{name:'Việc của tôi trong công trình'});row.railCount=await page.locator('[aria-label="Việc của tôi trong công trình"]').count();assert.ok(row.railCount>0);if(view==='mobile'){await page.getByRole('button',{name:/Việc của tôi/}).click();row.baselineGeometry=await baselineGeometry(context,`/projects/${PROJECT}/discussion`);} });
    for (const [name,route,selector] of [
      ['mywork-startup','/my-work','article:has-text("TH-GOV-20261023")'],
      ['discussion-workflow',`/projects/${PROJECT}/discussion`,'section[aria-label="Bước tiếp theo của quy trình"]'],
    ]) await scenario(view,`protected-${name}`,async row => {
      await navigate(route);const region=page.locator(selector).first();await region.waitFor();await sleep(500);
      const currentText=await region.innerText();row.currentRegion=`${view}-protected-${name}-current-region.png`;
      await region.screenshot({path:path.join(OUT,row.currentRegion),animations:'disabled',caret:'hide'});
      await sleep(comparisonDelay);
      const beforePage=await context.newPage();await beforePage.addInitScript(value=>localStorage.setItem('consflow_token',value),token);
      await beforePage.goto(BASELINE+route,{waitUntil:'domcontentloaded'});const previous=beforePage.locator(selector).first();await previous.waitFor();await sleep(700);
      assert.notEqual(new URL(beforePage.url()).pathname, '/login', 'Baseline session lost; comparison is invalid');
      const previousText=await previous.innerText();row.baselineRegion=`${view}-protected-${name}-baseline-region.png`;
      row.baselineGeometry=await beforePage.evaluate(()=>({layoutViewport:innerWidth,clientWidth:document.documentElement.clientWidth,documentWidth:document.documentElement.scrollWidth}));
      await previous.screenshot({path:path.join(OUT,row.baselineRegion),animations:'disabled',caret:'hide'});
      row.sameVisibleText=currentText.replace(/\s+/g,' ').trim()===previousText.replace(/\s+/g,' ').trim();
      comparisons.push({view,name,region_only:true,sameVisibleText:row.sameVisibleText,current:row.currentRegion,baseline:row.baselineRegion});
      await beforePage.close();assert.ok(row.sameVisibleText,'embedded Pipeline component differs from baseline');
    });
    for (const [name,route] of [
      ['pipeline-builder','/pipeline-builder/40000000-0000-0000-0000-000000000028'],
      ['workflow-forms','/workflow-forms'],
      ['project-pipeline',`/projects/${PROJECT}/pipeline`],
      ['project-workflow-tab',`/projects/${PROJECT}?tab=workflow`],
      ['embedded-form',`/work-items/3d04c4d3-088e-4d60-87e2-ed71b3ea5031`],
    ]) await scenario(view,`protected-${name}`,async row => {
      await navigate(route);await sleep(1000);
      await page.evaluate(() => { for (const element of document.querySelectorAll('*')) { if(element.scrollTop)element.scrollTop=0;if(element.scrollLeft)element.scrollLeft=0; } window.scrollTo(0,0); });
      const surface = target => name === 'embedded-form' ? target.locator('section.scroll-mt-24').filter({ has: target.locator('[data-workflow-process="two-step"]') }).first() : target.locator('body');
      await surface(page).waitFor();
      const currentText=await surface(page).innerText();
      row.currentScreenshot=await capture(`${view}-protected-${name}-current`);
      if (name === 'embedded-form') {
        row.currentRegion = `${view}-protected-${name}-current-region.png`;
        await surface(page).screenshot({path:path.join(OUT,row.currentRegion),animations:'disabled',caret:'hide'});
      }
      await sleep(comparisonDelay);
      const beforePage=await context.newPage();
      await beforePage.addInitScript(value=>localStorage.setItem('consflow_token',value),token);
      await beforePage.goto(BASELINE+route,{waitUntil:'domcontentloaded'});await beforePage.locator('h1,h2,h3,[role="dialog"]').first().waitFor();await sleep(1500);await beforePage.evaluate(()=>document.fonts.ready);
      assert.notEqual(new URL(beforePage.url()).pathname, '/login', 'Baseline session lost; comparison is invalid');
      await beforePage.evaluate(() => { for (const element of document.querySelectorAll('*')) { if(element.scrollTop)element.scrollTop=0;if(element.scrollLeft)element.scrollLeft=0; } window.scrollTo(0,0); });
      await surface(beforePage).waitFor();
      const baselineText=await surface(beforePage).innerText();
      row.baselineScreenshot=`${view}-protected-${name}-baseline.png`;
      await beforePage.screenshot({path:path.join(OUT,row.baselineScreenshot),animations:'disabled',caret:'hide'});
      if (name === 'embedded-form') {
        row.baselineRegion = `${view}-protected-${name}-baseline-region.png`;
        await surface(beforePage).screenshot({path:path.join(OUT,row.baselineRegion),animations:'disabled',caret:'hide'});
        row.comparisonScope = 'Complete WorkflowFormPanel; approved outer AppShell tagline change excluded';
      }
      row.sameVisibleText=currentText.replace(/\s+/g,' ').trim()===baselineText.replace(/\s+/g,' ').trim();
      row.baselineHeading=await beforePage.locator('h1,h2,h3').allTextContents();
      comparisons.push({view,name,sameVisibleText:row.sameVisibleText,current:row.currentScreenshot,baseline:row.baselineScreenshot});
      await beforePage.close();assert.ok(row.sameVisibleText,'protected visible text differs from baseline; inspect screenshots');
    });
    await context.close();await sleep(6500);
  }
  await browser.close();
  const summary={created_at:new Date().toISOString(),browser:'Chrome156, Chromium sandbox enabled',environment:'local API18081, database consflow_guide_20261022, migrations160, current15173 / baseline15174',mobile:'390x844 emulation; not physical device',status:results.some(x=>x.status!=='PASS')?'REVIEW_REQUIRED':'PASS',results,errors,comparisons};
  fs.writeFileSync(path.join(OUT,'results.json'),JSON.stringify(summary,null,2));
  console.log(JSON.stringify({total:results.length,pass:results.filter(x=>x.status==='PASS').length,check:results.filter(x=>x.status!=='PASS').map(x=>({view:x.view,name:x.name,error:x.error})),errors}));
})().catch(async error => { console.error(error.message);fs.writeFileSync(path.join(OUT,'results.json'),JSON.stringify({results,errors,comparisons,runner_error:error.message},null,2));if(browser)await browser.close();process.exitCode=1; });
