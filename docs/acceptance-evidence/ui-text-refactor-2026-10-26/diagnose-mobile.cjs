const fs = require('node:fs');
const path = require('node:path');
const { chromium } = require('/Users/khiemnguyen/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/node_modules/playwright');
const out = path.join(__dirname, 'fix-mobile-diagnosis');
fs.mkdirSync(out, { recursive: true });
(async () => {
  const browser = await chromium.launch({ executablePath: '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome', headless: true, chromiumSandbox: true });
  try {
    const context = await browser.newContext({ viewport: { width: 390, height: 844 }, isMobile: true, hasTouch: true, locale: 'vi-VN' });
    const page = await context.newPage();
    await page.goto('http://127.0.0.1:15173/login');
    const credentials = JSON.parse(fs.readFileSync('/private/tmp/consflow-guide-local-20261022/operator-login.json', 'utf8'));
    await page.locator('input[name="email"]').fill(credentials.email);
    await page.locator('input[name="password"]').fill(credentials.password);
    await page.getByRole('button', { name: 'Đăng nhập', exact: true }).click();
    await page.waitForURL('**/my-work');
    const results = [];
    for (const route of ['/audit', '/event-logs', '/projects/2885d835-e21e-4c9f-a194-16acaa776857/discussion']) {
      await page.evaluate(route => { history.pushState({ ...history.state, idx: (history.state?.idx ?? 0) + 1 }, '', route); dispatchEvent(new PopStateEvent('popstate')); }, route);
      await page.locator('h1').first().waitFor();
      await page.waitForTimeout(1800);
      const geometry = await page.evaluate(() => {
        const width = document.documentElement.clientWidth;
        const describe = el => ({ tag: el.tagName, classes: el.className, text: el.textContent?.trim().slice(0, 80), width: el.getBoundingClientRect().width, right: el.getBoundingClientRect().right, scroll: el.scrollWidth, minWidth: getComputedStyle(el).minWidth, display: getComputedStyle(el).display, overflow: getComputedStyle(el).overflowX });
        return { viewport: width, documentWidth: document.documentElement.scrollWidth, elements: [...document.querySelectorAll('main *,section,select')].filter(el => { const r = el.getBoundingClientRect(); return r.width > 0 && (r.right > width + 1 || r.width > width + 1); }).slice(0, 55).map(describe) };
      });
      results.push({ route, before: geometry });
      console.log(JSON.stringify({ route, ...geometry }));
      // Falsifiable, temporary DOM experiments only; navigation restores the source build.
      await page.evaluate(route => {
        if (route === '/audit' || route === '/event-logs') {
          document.querySelectorAll('select').forEach(el => { el.style.minWidth = '0'; el.style.maxWidth = '100%'; });
        } else {
          const grid = document.querySelector('div.grid.gap-4.xl\\:grid-cols-\\[minmax\\(0\\,1fr\\)_280px\\]');
          if (grid) { grid.style.gridTemplateColumns = 'minmax(0, 1fr)'; [...grid.children].forEach(el => el.style.minWidth = '0'); }
        }
      }, route);
      const after = await page.evaluate(() => ({ viewport: document.documentElement.clientWidth, documentWidth: document.documentElement.scrollWidth }));
      results.at(-1).afterExperiment = after;
      console.log(JSON.stringify({ route, afterExperiment: after }));
      await page.screenshot({ path: path.join(out, route === '/audit' ? 'audit-experiment.png' : route === '/event-logs' ? 'events-experiment.png' : 'discussion-experiment.png') });
    }
    fs.writeFileSync(path.join(out, 'results.json'), JSON.stringify(results, null, 2));
  } finally { await browser.close(); }
})().catch(error => { console.error(error.message); process.exitCode = 1; });
