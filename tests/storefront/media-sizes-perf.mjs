// REAL-stack S1 measurement; called inside the existing storefront browser harness.
// No credentials, altered routes, fake latency, failed-image savings or warm browser caches.
import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { mkdir, readFile, writeFile } from "node:fs/promises";
import path from "node:path";

export async function measureMediaSizes({ browser, origin, evidence, ctxOpts }) {
  const phase = process.env.LC_MEDIA_SIZES_PHASE;
  if (!phase) return;
  assert(["before", "after"].includes(phase));
  const directory = path.resolve("output/media-sizes");
  await mkdir(directory, { recursive: true });
  const result = { phase, fixture: "cmiJPEG-1440x1800-v1", viewport: { width: 390, height: 844 }, dpr: 2,
    network: "Chromium CDP 1.6Mbps down / 750Kbps up / 150ms RTT; CPU 1x", samples: [] };
  for (let sample = 0; sample < 7; sample++) {
    const context = await browser.newContext(ctxOpts({ ignoreHTTPSErrors: true, viewport: result.viewport, deviceScaleFactor: 2, isMobile: true, hasTouch: true, locale: "zh-TW" }));
    try {
      const page = await context.newPage(), reads = [], bodies = new Map();
      const cdp = await context.newCDPSession(page);
      await cdp.send("Network.enable");
      await cdp.send("Network.setCacheDisabled", { cacheDisabled: true });
      await cdp.send("Network.emulateNetworkConditions", { offline: false, latency: 150, downloadThroughput: 1600000 / 8, uploadThroughput: 750000 / 8 });
      await page.addInitScript(() => {
        window.__mediaLCP = null;
        new PerformanceObserver(list => { const entry = list.getEntries().at(-1); window.__mediaLCP = { time: entry.startTime, tag: entry.element?.tagName, url: entry.url }; }).observe({ type: "largest-contentful-paint", buffered: true });
      });
      page.on("response", response => {
        if (response.request().resourceType() === "image") reads.push(response.body().then(bytes => bodies.set(response.url(), { status: response.status(), bytes: bytes.length })));
      });
      await page.goto(`${origin}/zh-TW/products`, { waitUntil: "networkidle" });
      await page.evaluate(async () => { await Promise.all([...document.images].filter(i => i.getBoundingClientRect().top < innerHeight).map(i => i.decode())); });
      await page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
      await Promise.all(reads);
      const visible = await page.evaluate(() => [...document.images].filter(i => { const r = i.getBoundingClientRect(); return r.width > 0 && r.height > 0 && r.top < innerHeight && r.bottom > 0; }).map(i => ({ url: i.currentSrc, loaded: i.complete && i.naturalWidth > 0 })));
      assert(visible.length >= 2 && visible.every(i => i.loaded), "visible images must load, not disappear");
      const images = [];
      for (const url of new Set(visible.map(i => i.url))) {
        const body = bodies.get(url); assert(body?.status === 200 && body.bytes > 0);
        assert.equal(new URL(url).search !== "", phase === "after", "phase must match actual built source");
        if (phase === "after") {
          assert.match(new URL(url).search, /^\?w=(360|720|1080)$/);
          const selectedWidth = await page.evaluate(async url => {
            const bitmap = await createImageBitmap(await (await fetch(url)).blob());
            const width = bitmap.width; bitmap.close(); return width;
          }, url);
          assert.equal(selectedWidth, Number(new URL(url).searchParams.get("w")), "real decoded derivative width, not a query ignored upstream");
        }
        const original = await context.request.get(url.split("?")[0]); assert.equal(original.status(), 200);
        images.push({ ...body, width: new URL(url).searchParams.get("w"), originalSHA256: createHash("sha256").update(await original.body()).digest("hex") });
      }
      const lcp = await page.evaluate(() => window.__mediaLCP); assert(lcp?.time > 0);
      assert.equal(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), true);
      result.samples.push({ sample, visible: visible.length, imageBytes: images.reduce((sum, i) => sum + i.bytes, 0), images, lcp });
      if (sample === 0) await page.screenshot({ path: path.join(directory, `${phase}-products-zh-TW-390.png`), scale: "css" });
    } finally { await context.close(); }
  }
  const median = values => [...values].sort((a, b) => a - b)[3];
  result.medianLCP = median(result.samples.map(s => s.lcp.time));
  await writeFile(path.join(directory, `${phase}.json`), JSON.stringify(result, null, 2) + "\n");
  if (phase === "after") {
    const before = JSON.parse(await readFile(path.join(directory, "before.json"), "utf8"));
    assert.equal(before.fixture, result.fixture);
    for (const sample of result.samples) {
      assert.deepEqual(sample.images.map(i => i.originalSHA256), before.samples[0].images.map(i => i.originalSHA256), "same originals and order");
      assert(sample.imageBytes <= before.samples[0].imageBytes * 0.30, ">=70% image body reduction");
    }
    assert(result.medianLCP <= before.medianLCP, `LCP regressed ${before.medianLCP} -> ${result.medianLCP}`);
  }
  console.log(`MEDIA-SIZES ${phase}: bytes=${result.samples[0].imageBytes} medianLCP=${result.medianLCP}; evidence=${evidence}`);
}
