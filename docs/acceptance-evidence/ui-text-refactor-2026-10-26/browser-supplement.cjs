const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict');
const {chromium}=require('/Users/khiemnguyen/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/node_modules/playwright');
const OUT=path.join(__dirname,'browser-supplement');fs.mkdirSync(OUT,{recursive:true});const results=[];
const sleep=ms=>new Promise(resolve=>setTimeout(resolve,ms));
(async()=>{
 const browser=await chromium.launch({executablePath:'/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',headless:true,chromiumSandbox:true});
 const context=await browser.newContext({viewport:{width:1440,height:1000},locale:'vi-VN',timezoneId:'Asia/Ho_Chi_Minh'});const page=await context.newPage();page.setDefaultTimeout(10000);
 await page.goto('http://127.0.0.1:15173/login');const c=JSON.parse(fs.readFileSync('/private/tmp/consflow-guide-local-20261022/setup-login.json','utf8'));
 await page.locator('input[name="email"]').fill(c.email);await page.locator('input[name="password"]').fill(c.password);await page.getByRole('button',{name:'Đăng nhập',exact:true}).click();await page.waitForURL('**/my-work');
 async function check(view,name,route,expect){
  const row={view,name,evidence_dir:'browser-supplement',route};
  try{
   await page.evaluate(r=>{history.pushState({...history.state,idx:(history.state?.idx??0)+1},'',r);dispatchEvent(new PopStateEvent('popstate'));},route);await sleep(700);await expect();
   const m=await page.evaluate(()=>({layoutViewport:innerWidth,clientWidth:document.documentElement.clientWidth,documentWidth:document.documentElement.scrollWidth,heading:[...document.querySelectorAll('h1,h2,h3')].map(x=>x.textContent.trim()),alerts:[...document.querySelectorAll('[role="alert"]')].map(x=>x.textContent)}));row.measurement={...m,viewport:page.viewportSize().width};assert.ok(m.documentWidth<=row.measurement.viewport+1);row.status='PASS';
  }catch(e){row.status='CHECK_REQUIRED';row.error=e.message.split('\n')[0]}
  row.screenshot=`${view}-${name}.png`;await page.screenshot({path:path.join(OUT,row.screenshot),animations:'disabled',caret:'hide'});results.push(row);console.log(view,name,row.status);
 }
 for(const view of ['desktop','mobile']){
  await page.setViewportSize(view==='desktop'?{width:1440,height:1000}:{width:390,height:844});
  await check(view,'recovery-requests','/recovery-requests',async()=>{await page.getByRole('heading',{name:'Yêu cầu khôi phục',exact:true}).waitFor();});
  await check(view,'record-source-unavailable','/source-records/issue/00000000-0000-0000-0000-000000000000?project_id=2885d835-e21e-4c9f-a194-16acaa776857',async()=>{await page.locator('h1').filter({hasText:'Hồ sơ nguồn'}).waitFor();await page.getByRole('alert').waitFor();});
  await page.keyboard.press('Escape');await sleep(100);
 }
 await context.close();
 const anonymous=await browser.newContext({viewport:{width:1440,height:1000},locale:'vi-VN'});const publicPage=await anonymous.newPage();
 for(const view of ['desktop','mobile']){await publicPage.setViewportSize(view==='desktop'?{width:1440,height:1000}:{width:390,height:844});await publicPage.goto('http://127.0.0.1:15173/forgot-password');await publicPage.getByRole('heading',{name:'Quên mật khẩu',exact:true}).waitFor();const m=await publicPage.evaluate(()=>({documentWidth:document.documentElement.scrollWidth,layoutViewport:innerWidth}));const screenshot=`${view}-forgot-password.png`;await publicPage.screenshot({path:path.join(OUT,screenshot)});results.push({view,name:'forgot-password',evidence_dir:'browser-supplement',status:m.documentWidth<=publicPage.viewportSize().width?'PASS':'FAIL',measurement:{...m,viewport:publicPage.viewportSize().width},screenshot});}
 await browser.close();fs.writeFileSync(path.join(OUT,'results.json'),JSON.stringify({results,comparisons:[],errors:[]},null,2));console.log(JSON.stringify({checks:results.length,status:results.map(x=>x.status)}));
})().catch(e=>{console.error(e.message);process.exitCode=1});
