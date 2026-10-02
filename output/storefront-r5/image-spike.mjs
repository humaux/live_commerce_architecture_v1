// G1 option-A probe. MOCK API, production Next, real Chromium; never contacts a merchant or provider.
// Run from the worktree root: node output/storefront-r5/image-spike.mjs before|after
// Existing fixture is unchanged between runs. A loopback TLS edge forwards the browser's exact Host.
import assert from 'node:assert/strict';
import http from 'node:http';
import https from 'node:https';
import net from 'node:net';
import { once } from 'node:events';
import { spawn, execFileSync } from 'node:child_process';
import { createHash, randomBytes } from 'node:crypto';
import { createWriteStream } from 'node:fs';
import { readFile, writeFile, mkdtemp, rm } from 'node:fs/promises';
import path from 'node:path';
import { chromium } from '@playwright/test';
import { createFakeApi } from '../../tests/storefront/shop-fake-api.mjs';

const phase = process.argv[2];
assert(['before', 'after'].includes(phase));
const root = process.cwd(), out = path.join(root, 'output/storefront-r5');
const host = 'shop.example', foreignHost = 'other-shop.example';
const key = () => randomBytes(32).toString('base64url');
const bff = key(), api = createFakeApi({ bffKey: bff });
let next, edge, proxy, browser, log, certDir;
const sockets = new Set();
const result = { phase, tier: 'MOCK', viewport: { width: 390, height: 844 }, dpr: 2, path: '/zh-TW/products', runs: [] };
async function listen(server) {
  server.listen(0, '127.0.0.1'); await once(server, 'listening'); return server.address().port;
}
function request(port, pathname, requestHost = host, accept = 'image/webp,image/*,*/*;q=0.8', incoming = {}) {
  return new Promise((resolve, reject) => {
    const headers = { ...incoming, host: requestHost, accept };
    delete headers.connection; delete headers['transfer-encoding'];
    const req = http.get({ hostname: '127.0.0.1', port, path: pathname, headers }, res => {
      const chunks = []; res.on('data', c => chunks.push(c));
      res.on('end', () => resolve({ status: res.statusCode, headers: res.headers, body: Buffer.concat(chunks) }));
      res.on('error', reject);
    });
    req.setTimeout(60000, () => req.destroy(new Error('request timeout'))); req.on('error', reject);
  });
}
try {
  result.fixtureSHA256 = createHash('sha256').update(await readFile('tests/storefront/shop-fake-api.mjs')).digest('hex');
  const apiPort = await api.listen();
  const temporary = http.createServer(); const port = await listen(temporary); await new Promise(r => temporary.close(r));
  log = createWriteStream(path.join(out, `${phase}-next.log`), { flags: 'w', mode: 0o600 });
  await once(log, 'open');
  next = spawn(process.execPath, ['node_modules/next/dist/bin/next', 'start', '--hostname', '127.0.0.1', '--port', String(port)], {
    cwd: path.join(root, 'apps/storefront'), stdio: ['ignore', log, log],
    env: { ...process.env, NODE_ENV: 'production', NEXT_TELEMETRY_DISABLED: '1', COMMERCE_BUYER_WEB_ENABLED: '1', COMMERCE_BUYER_API_ORIGIN: `http://127.0.0.1:${apiPort}`, COMMERCE_BUYER_BFF_KEY: bff, COMMERCE_BUYER_COOKIE_KEY: key(), COMMERCE_BUYER_SESSION_TTL: '3600' },
  });
  let ready = false;
  for (let i = 0; i < 300; i++) {
    assert(next.exitCode === null, 'Next exited before readiness');
    try { if ((await request(port, '/robots.txt')).status === 200) { ready = true; break; } } catch { /* starting */ }
    await new Promise(r => setTimeout(r, 100));
  }
  assert(ready, 'Next readiness timed out');
  const pagePreflight = await request(port, result.path, host, 'text/html');
  result.pagePreflight = { status: pagePreflight.status, bytes: pagePreflight.body.length };
  console.log('page preflight', JSON.stringify(result.pagePreflight));
  if (phase === 'after') {
    const product = api.products[0];
    const raw = `/media/p/${product.id}/${product.images[0].id}`;
    const optimized = `/_next/image?url=${encodeURIComponent(raw)}&w=640&q=75`;
    const rawOwner = await request(port, raw), rawForeign = await request(port, raw, foreignHost);
    const optOwner = await request(port, optimized), optForeign = await request(port, optimized, foreignHost);
    result.isolation = {
      rawOwnerStatus: rawOwner.status, rawForeignStatus: rawForeign.status,
      optimizedOwnerStatus: optOwner.status, optimizedForeignStatus: optForeign.status,
      optimizedOwnerContentType: optOwner.headers['content-type'],
      optimizedOwnerBody: optOwner.status === 200 ? null : optOwner.body.toString().slice(0, 200),
      warmCacheTest: optOwner.status === 200 ? 'available' : 'NOT_RUN: owner optimization failed',
    };
    assert.equal(rawOwner.status, 200); assert.equal(rawForeign.status, 404);
  }
  // Matches the existing shop-gate TLS/CONNECT harness; no hosts-file mutation or response mocking.
  certDir = await mkdtemp(path.join(out, 'tls-fixture-'));
  execFileSync('openssl', ['req', '-x509', '-newkey', 'rsa:2048', '-nodes', '-keyout', path.join(certDir, 'key.pem'), '-out', path.join(certDir, 'cert.pem'), '-days', '1', '-subj', `/CN=${host}`], { stdio: 'ignore' });
  edge = https.createServer({ key: await readFile(path.join(certDir, 'key.pem')), cert: await readFile(path.join(certDir, 'cert.pem')) }, async (req, res) => {
    try {
      const url = new URL(req.url, `https://${req.headers.host}`);
      if (url.hostname !== host || req.method !== 'GET') { res.writeHead(403); res.end(); return; }
      const response = await request(port, url.pathname + url.search, req.headers.host, req.headers.accept, req.headers);
      const headers = { ...response.headers }; delete headers['transfer-encoding']; delete headers.connection;
      res.writeHead(response.status, headers); res.end(response.body);
    } catch { res.writeHead(502); res.end(); }
  });
  const edgePort = await listen(edge);
  proxy = http.createServer((_, res) => { res.writeHead(403); res.end(); });
  proxy.on('connect', (req, socket, head) => {
    if (req.url !== `${host}:443`) { socket.destroy(); return; }
    const upstream = net.connect(edgePort, '127.0.0.1', () => {
      socket.write('HTTP/1.1 200 Connection Established\r\n\r\n');
      if (head.length) upstream.write(head);
      socket.pipe(upstream).pipe(socket);
    });
    for (const s of [socket, upstream]) {
      sockets.add(s); s.on('close', () => sockets.delete(s));
      s.on('error', () => { socket.destroy(); upstream.destroy(); });
    }
  });
  const proxyPort = await listen(proxy);
  browser = await chromium.launch({ headless: true, proxy: { server: `http://127.0.0.1:${proxyPort}` } });
  for (let sample = 0; sample < 3; sample++) {
    const context = await browser.newContext({ viewport: result.viewport, deviceScaleFactor: 2, isMobile: true, hasTouch: true, locale: 'zh-TW', ignoreHTTPSErrors: true, serviceWorkers: 'block' });
    const page = await context.newPage();
    const images = new Map(), reads = [];
    page.on('response', res => {
      if (res.request().resourceType() === 'image') reads.push(res.body().then(body => images.set(res.url(), { bytes: body.length, status: res.status(), sha256: createHash('sha256').update(body).digest('hex') })));
    });
    await page.addInitScript(() => {
      window.__lcp = 0;
      new PerformanceObserver(list => { for (const entry of list.getEntries()) window.__lcp = entry.startTime; }).observe({ type: 'largest-contentful-paint', buffered: true });
    });
    try { await page.goto(`https://${host}${result.path}`, { waitUntil: 'networkidle' }); }
    catch (error) {
      result.browserDiagnostic = { url: page.url(), title: await page.title(), text: (await page.locator('body').innerText()).slice(0, 300), upstreamRequests: api.state.requests.slice(-10) };
      throw error;
    }
    await page.locator('[data-testid="product-card"]').first().waitFor();
    await page.evaluate(async () => { await Promise.all([...document.images].filter(i => i.getBoundingClientRect().top < innerHeight).map(i => i.decode().catch(() => {}))); });
    await Promise.all(reads);
    const visible = await page.evaluate(() => [...document.images].filter(i => { const r = i.getBoundingClientRect(); return r.width > 0 && r.height > 0 && r.top < innerHeight && r.bottom > 0; }).map(i => ({ url: i.currentSrc, loaded: i.complete && i.naturalWidth > 0, width: i.naturalWidth })));
    const urls = [...new Set(visible.map(i => i.url))];
    const resources = urls.map(url => ({ url: new URL(url).pathname + new URL(url).search, ...images.get(url) }));
    assert.equal(resources.some(r => r.url.startsWith('/_next/image?')), phase === 'after', 'phase must match the built UI, not merely the probe argument');
    const rendered = visible.length > 0 && visible.every(i => i.loaded);
    result.runs.push({ sample, rendered, visibleImages: visible.length, loadedImages: visible.filter(i => i.loaded).length, imageBytes: resources.filter(r => r.status === 200).reduce((sum, r) => sum + r.bytes, 0), errorBodyBytes: resources.filter(r => r.status !== 200).reduce((sum, r) => sum + (r.bytes || 0), 0), lcpMs: await page.evaluate(() => window.__lcp), resources, overflow: await page.evaluate(() => document.documentElement.scrollWidth > innerWidth) });
    if (sample === 0) await page.screenshot({ path: path.join(out, `${phase}-products-zh-TW-390x844.png`), scale: 'css' });
    if (phase === 'before') assert(rendered && resources.every(r => r.status === 200 && r.bytes > 0), 'baseline images must really render');
    await context.close();
  }
  if (phase === 'after') {
    const before = JSON.parse(await readFile(path.join(out, 'before.json'), 'utf8'));
    assert.equal(before.fixtureSHA256, result.fixtureSHA256);
    result.imageReduction = result.runs.every(r => r.rendered) ? 1 - result.runs[0].imageBytes / before.runs[0].imageBytes : null;
    const medianLCP = runs => runs.map(r => r.lcpMs).sort((a, b) => a - b)[1];
    result.lcpNonRegression = result.imageReduction === null ? null : medianLCP(result.runs) <= medianLCP(before.runs);
    result.accepted = result.isolation.optimizedOwnerStatus === 200 && result.isolation.optimizedForeignStatus !== 200 && result.imageReduction !== null && result.imageReduction >= 0.7 && result.lcpNonRegression === true;
    if (!result.accepted) process.exitCode = 1;
    // One bounded diagnostic capture batch, not visual acceptance of an unshipped feature.
    for (const [locale, width, height] of [['en', 390, 844], ['zh-TW', 1586, 992], ['en', 1586, 992]]) {
      const context = await browser.newContext({ viewport: { width, height }, deviceScaleFactor: width === 390 ? 2 : 1, locale, ignoreHTTPSErrors: true });
      const page = await context.newPage();
      await page.goto(`https://${host}/${locale}/products`, { waitUntil: 'networkidle' });
      await page.screenshot({ path: path.join(out, `${phase}-products-${locale}-${width}x${height}.png`), scale: 'css' });
      await context.close();
    }
  }
} catch (error) {
  result.error = error.message; process.exitCode = 1;
} finally {
  await browser?.close();
  for (const socket of sockets) socket.destroy();
  if (proxy) { proxy.closeAllConnections(); await new Promise(r => proxy.close(r)); }
  if (edge) { edge.closeAllConnections(); await new Promise(r => edge.close(r)); }
  if (next?.exitCode === null) { next.kill('SIGTERM'); await once(next, 'exit'); }
  log?.end(); await api.close();
  if (certDir) await rm(certDir, { recursive: true });
  await writeFile(path.join(out, `${phase}.json`), JSON.stringify(result, null, 2) + '\n');
  console.log(JSON.stringify({ phase, exitCode: process.exitCode || 0, samples: result.runs.map(r => ({ bytes: r.imageBytes, lcpMs: r.lcpMs })), isolation: result.isolation, imageReduction: result.imageReduction, error: result.error }));
}
