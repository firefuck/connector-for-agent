// Local-only browser regression. All identities and credentials are synthetic.
// Requires Playwright and Chromium; run after packaging/build.sh.
const { chromium } = require('playwright');
const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const os = require('node:os');
const path = require('node:path');
const net = require('node:net');
const { spawn } = require('node:child_process');
const { randomBytes } = require('node:crypto');
const { once } = require('node:events');

(async () => {
  const root = await fs.mkdtemp(path.join(os.tmpdir(), 'relaydesk-browser-'));
  const listener = net.createServer(); listener.listen(0, '127.0.0.1'); await once(listener, 'listening');
  const port = listener.address().port; await new Promise(resolve => listener.close(resolve));
  const origin = 'http://127.0.0.1:' + port;
  const admin = randomBytes(32).toString('hex');
  const gateway = spawn(path.resolve(process.env.RELAYDESK_BINARY || 'dist/relaydesk-linux-amd64'), ['gateway', '--listen', '127.0.0.1:' + port, '--state', path.join(root, 'state.json')], {env: {...process.env, RELAYDESK_ADMIN_TOKEN: admin}, stdio: 'ignore'});
  let browser;
  const post = async (route, input, token = admin) => {
    const response = await fetch(origin + route, {method:'POST', headers:{'Content-Type':'application/json', Authorization:'Bearer ' + token}, body:JSON.stringify(input)});
    assert.equal(response.status, 200, route + ' failed'); return response.json();
  };
  try {
    for (let i=0;;i++) { try { if ((await fetch(origin+'/health')).ok) break; } catch {} if(i>=80) throw Error('gateway startup failed'); await new Promise(r=>setTimeout(r,50)); }
    browser = await chromium.launch({executablePath:process.env.CHROMIUM_PATH || '/usr/bin/chromium', headless:true, args:['--no-sandbox']});
    const context = await browser.newContext({viewport:{width:1280,height:900}});
    const page = await context.newPage();
    const errors = []; page.on('pageerror',e=>errors.push(e.message));
    await page.goto(origin);
    await page.locator('#token').fill('invalid'); await page.locator('#login-form button').click();
    await page.waitForFunction(()=>!document.getElementById('error').hidden);
    assert.equal(await page.locator('#main').isVisible(),false);
    await page.locator('#token').fill(admin); await page.locator('#login-form button').click();
    await page.locator('#main').waitFor({state:'visible'});
    assert.equal(await page.locator('#token').inputValue(),'');
    assert.equal(await page.locator('#preset').inputValue(),'view');
    assert.equal(await page.locator('#pair-form').isVisible(), false);
    assert.equal(await page.locator('#agent-form').isVisible(), false);
    // Drawers trap keyboard focus, preserve drafts and restore the menu trigger.
    await page.locator('#menu-pair').click();
    await page.locator('#drawer').waitFor({state:'visible'});
    await page.locator('#pcname').fill('draft PC');
    for(let i=0;i<8;i++) { await page.keyboard.press('Tab'); assert(await page.evaluate(()=>document.getElementById('drawer').contains(document.activeElement)), 'focus escaped modal'); }
    for(let i=0;i<8;i++) { await page.keyboard.press('Shift+Tab'); assert(await page.evaluate(()=>document.getElementById('drawer').contains(document.activeElement)), 'reverse focus escaped modal'); }
    await page.keyboard.press('Escape');
    await page.waitForFunction(()=>!document.getElementById('drawer').open);
    assert.equal(await page.evaluate(()=>document.activeElement.id), 'menu-pair');
    await page.locator('#menu-pair').click();
    assert.equal(await page.locator('#pcname').inputValue(), 'draft PC');
    // Malicious device names must stay literal text under the CSP.
    const hostile='<img src=x onerror="window.injected=true">';
    await page.locator('#pcname').fill(hostile);
    await page.locator('#pair-form button').click(); await page.locator('#result-card').waitFor({state:'visible'});
    const code=await page.getByRole('textbox',{name:'Kode pairing',exact:true}).inputValue();
    const device=await post('/pair',{code},'');
    await post('/poll',{platform:'linux',capabilities:['desktop_x11','files']},device.token);
    await page.locator('#clear-result').click();
    assert.equal(await page.locator('#result').textContent(),'');
    await page.locator('#refresh').click();
    await page.waitForFunction(()=>document.querySelector('#device-options input'));
    assert.equal(await page.locator('#pcs img').count(),0);
    assert.equal(await page.evaluate(()=>window.injected),undefined);
    await page.locator('#menu-devices').click();
    assert.equal(await page.locator('#pcs').isVisible(), true);
    assert.equal(await page.locator('#agent-form').isVisible(), false);
    // Desktop backdrop closes the panel and restores focus.
    await page.mouse.click(20,150);
    await page.waitForFunction(()=>!document.getElementById('drawer').open);
    await page.locator('#menu-agent').click();
    await page.locator('#agentname').fill('agent-vps');
    await page.locator('#agent-form button').click();
    await page.waitForFunction(()=>!document.getElementById('error').hidden);
    assert.equal((await post('/admin/list',{})).agents.length,0);
    await page.locator('#device-options input').check();
    // Hold the first request; a second submission must not create another token.
    let created=0, release;
    const held=new Promise(resolve=>release=resolve);
    await page.route('**/admin/agent',async route=>{created++;await held;await route.continue();});
    await page.locator('#agent-form button').click();
    await page.waitForFunction(()=>document.querySelector('#agent-form button').disabled);
    await page.locator('#agent-form').evaluate(form=>form.dispatchEvent(new Event('submit',{cancelable:true})));
    await page.keyboard.press('Escape');
    assert.equal(await page.evaluate(()=>document.getElementById('drawer').open), true, 'pending mutation drawer closed');
    release(); await page.locator('#result-card').waitFor({state:'visible'});
    await page.waitForFunction(()=>!document.querySelector('#agent-form button').disabled);
    assert.equal(created,1); await page.unroute('**/admin/agent');
    const state=await post('/admin/list',{});
    assert.equal(state.agents.length,1); assert.deepEqual(state.agents[0].devices,[device.id]);
    assert(!state.agents[0].tools.includes('*') && !state.agents[0].tools.includes('system_exec'));
    const agentToken=await page.getByRole('textbox',{name:'Token agent',exact:true}).inputValue();
    assert.equal(await page.getByRole('textbox',{name:'URL MCP',exact:true}).inputValue(),origin+'/mcp');
    assert.equal(await page.evaluate(()=>localStorage.length+sessionStorage.length),0);
    await page.keyboard.press('Escape');
    await page.waitForFunction(()=>!document.getElementById('drawer').open);
    assert.equal(await page.locator('#result').textContent(),'');
    page.on('dialog',dialog=>dialog.accept());
    await page.locator('#menu-agents').click();
    await page.locator('#agents summary').click();
    await page.getByRole('textbox',{name:'Izin tool agent-vps',exact:true}).fill('');
    await page.getByRole('button',{name:'Simpan izin',exact:true}).click();
    await page.waitForFunction(()=>document.getElementById('notice').textContent.includes('Izin diperbarui'));
    assert.deepEqual((await post('/admin/list',{})).agents[0].tools,[]);
    await page.locator('#drawer-close').click();
    // Mobile layout must fit and controls remain reachable.
    await page.setViewportSize({width:390,height:844});
    assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),'mobile overflow');
    const artifact=process.env.RELAYDESK_SCREENSHOT;
    if(artifact) await page.screenshot({path:artifact,fullPage:true});
    await page.locator('#menu-agents').click();
    assert(await page.evaluate(()=>document.getElementById('drawer').scrollWidth<=innerWidth), 'mobile drawer overflow');
    await page.locator('#agents .danger').click();
    await page.waitForFunction(()=>document.getElementById('notice').textContent==='Akses dicabut.');
    assert.equal((await post('/admin/list',{})).agents[0].revoked,true);
    // Lock during a pending response must not reveal a late token or reopen UI.
    let releasePair, receivedPair;
    const pairHeld=new Promise(resolve=>releasePair=resolve);
    const pairStarted=new Promise(resolve=>receivedPair=resolve);
    await page.route('**/admin/pair',async route=>{receivedPair();await pairHeld;await route.fulfill({status:200,contentType:'application/json',body:JSON.stringify({code:'synthetic-late-code',expires_in:600})});});
    await page.locator('#drawer-close').click();
    await page.locator('#menu-pair').click();
    await page.locator('#pcname').fill('pending');await page.locator('#pair-form button').click();await pairStarted;
    await page.locator('#drawer-lock').click();releasePair();
    await page.locator('#login').waitFor({state:'visible'});
    await page.waitForTimeout(100);
    assert.equal(await page.locator('#main').isVisible(),false);
    assert.equal(await page.locator('#result').textContent(),'');
    assert(!(await page.content()).includes(agentToken));
    assert.equal(await page.evaluate(()=>document.getElementById('drawer').open),false);
    // Reduced motion and a narrow viewport retain usable drawer navigation.
    await page.unroute('**/admin/pair');
    await page.emulateMedia({reducedMotion:'reduce'});
    await page.setViewportSize({width:320,height:640});
    await page.locator('#token').fill(admin); await page.locator('#login-form button').click();
    await page.locator('#main').waitFor({state:'visible'});
    assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),'320px page overflow');
    await page.locator('#menu-agent').click();
    assert(await page.evaluate(()=>document.getElementById('drawer').scrollWidth<=innerWidth),'320px drawer overflow');
    assert.equal(await page.locator('#drawer').evaluate(n=>getComputedStyle(n).transitionDuration),'0s');
    await page.keyboard.press('Escape');
    await page.locator('#logout').click();
    assert.deepEqual(errors,[]);
    console.log('PASS dashboard: login, pairing, literal XSS payload, default permissions, device selection, duplicate submit, policy/revoke, mobile layout, no browser token storage, lock during pending request; drawer focus, Escape/backdrop, draft preservation, pending mutation, secret clearing, reduced motion and 320px layout.');
  } finally {
    if(browser) await browser.close();
    const exited=once(gateway,'exit');gateway.kill('SIGTERM');await exited;
    await fs.rm(root,{recursive:true,force:true});
  }
})().catch(err=>{console.error(err.message);process.exitCode=1;});
