// Diagnostic only. Same native tel anchor and unchanged click-sweep monitor as baseline.
// This does not replace, waive, or change the full browser gate's failed verdict.
import assert from 'node:assert/strict';
import http from 'node:http';
import { once } from 'node:events';
import { execFileSync } from 'node:child_process';
import { chromium } from '@playwright/test';
import { INIT_SCRIPT, monitor, listControls, sweepControl, settle } from '../../tests/ui/click-sweep-lib.mjs';
for (const file of ['apps/storefront/components/ShopChrome.tsx', 'tests/ui/click-sweep-lib.mjs', 'tests/ui/click-sweep.mjs']) {
  const base = execFileSync('git', ['rev-parse', `e44e58a2:${file}`], {encoding:'utf8'}).trim();
  const head = execFileSync('git', ['rev-parse', `HEAD:${file}`], {encoding:'utf8'}).trim();
  assert.equal(head, base);
  console.log(`${file}: baseline=head ${base}`);
}
const server = http.createServer((_, res) => {
  res.setHeader('Content-Type', 'text/html');
  res.end('<!doctype html><html lang="en"><body><footer><a href="tel:+886223456789">+886 2 2345 6789</a></footer></body></html>');
});
server.listen(0,'127.0.0.1'); await once(server,'listening');
const url = `http://127.0.0.1:${server.address().port}`;
const browser = await chromium.launch({headless:true});
try {
  const context = await browser.newContext({viewport:{width:1586,height:992}});
  await context.addInitScript(INIT_SCRIPT);
  const mon = monitor(context,{allowedHosts:new Set([new URL(url).host])});
  const page = await context.newPage();
  await page.goto(url); await settle(page,mon);
  const controls = await listControls(page,'footer','chrome');
  assert.equal(controls.length,1);
  const row = await sweepControl({page,mon,rootSelector:'footer',unit:{app:'storefront',route:'/',viewport:'desktop',locale:'en'},window:3000},controls[0],1);
  console.log(JSON.stringify(row,null,2));
  assert.equal(row.failure,'no-effect');
  console.log('DIAGNOSTIC REPRODUCED: unchanged baseline monitor reports no-effect for a native tel link in headless Chromium. Full gate remains FAIL; no assertion changed.');
} finally { await browser.close(); await new Promise(resolve=>server.close(resolve)); }
