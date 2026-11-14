// Purpose: native contact-only acceptance extracted from PR440 1182b744; no C7/browse parity import.
// Depends on: production-built storefront via shop-real-gate, its browser/virtual origin, and synthetic Go/PG fixture.
// Used by: shop-real-gate.mjs (--browser-storefront); Go independently requires the complete marker and every ledger case.
// Invariants: real clicks/fills, no autoPOST, exact-key/body recovery, memory-only private input; MOCK != SANDBOX.
import assert from "node:assert/strict";
import { writeFile } from "node:fs/promises";
import path from "node:path";
import { expect, type Browser, type BrowserContext, type Page } from "@playwright/test";
import { ctxOpts } from "./browser-engine.mjs";

const T = 10_000;
const namespace = "M8-CONTACT-NATIVE/v1";
const mockEnvironment = "MOCK";
type Row = { namespace: string; case_id: string; frame: string; locale: string; viewport: string; route: string; control: string; operation: string; expected: string; actual: string; environment: string; status: "PASS" | "FAIL" };

// Contact regression: terminal attempts cannot acquire recovery authority from another tab's pending marker.
async function checkContactAttemptLifecycle(createContext: (options: Parameters<Browser["newContext"]>[0]) => Promise<BrowserContext>, options: Parameters<Browser["newContext"]>[0], origin: string, timeout = 10000, record: (result: unknown) => void = () => {}) {
  const results: { status: number; edited: boolean; recoverOffered: boolean; summary: string | null; unknownText: string; calls: unknown[] }[] = [];
  for (const status of [400, 413, 415, 422, 429]) for (const edited of [false, true]) {
    const context = await createContext(options);
    const first = await context.newPage(), other = await context.newPage();
    const calls: { tab: string; key: string | undefined; body: string | null }[] = [];
    const A = { name: "Synthetic Alpha", email: "alpha@example.invalid", message: "Synthetic message A original refused" };
    const B = { name: "Synthetic Beta", email: "beta@example.invalid", message: "Synthetic message B edited draft" };
    const C = { name: "Synthetic Gamma", email: "gamma@example.invalid", message: "Synthetic message C other tab uncertain" };
    const errors: string[] = [];
    try {
      for (const [tab, page] of [["A", first], ["C", other]] as const) {
        page.on("pageerror", e => errors.push(e.message));
        await page.route("**/api/buyer/contact-messages", async route => {
          const request = route.request();
          assert.equal(request.method(), "POST");
          calls.push({ tab, key: request.headers()["idempotency-key"], body: request.postData() });
          await route.fulfill({ status: tab === "A" ? status : 503, json: { code: tab === "A" ? status === 429 ? "rate_limited" : "invalid_request" : "unavailable" } });
        });
      }
      const fill = async (page: Page, values: typeof A) => {
        for (const [field, value] of Object.entries(values)) {
          await page.getByTestId(`contact-${field}`).fill(value);
          await expect(page.getByTestId(`contact-${field}`)).toHaveValue(value);
        }
      };
      await first.goto(`${origin}/en/contact`);
      await fill(first, A);
      await first.getByTestId("contact-next").click();
      await expect(first.getByTestId("contact-confirm")).toBeVisible({ timeout });
      assert.equal(calls.length, 0);
      await first.getByTestId("contact-send").click();
      await expect(first.locator("main").getByRole("alert")).toBeVisible({ timeout });
      if (status === 429) await expect(first.getByTestId("contact-summary")).toContainText(A.message);
      assert.equal(calls.length, 1);
      if (edited) {
        await first.locator("main").getByRole("button").click();
        await expect(first.getByTestId("contact-form")).toBeVisible({ timeout });
        await fill(first, B);
      }
      await other.goto(`${origin}/en/contact`);
      await fill(other, C);
      await other.getByTestId("contact-next").click();
      assert.equal(calls.length, 1);
      await other.getByTestId("contact-send").click();
      await expect(other.getByTestId("contact-unknown")).toBeVisible({ timeout });
      await expect(other.getByTestId("contact-summary")).toContainText(C.message);
      // Read-only lock measurement ensures the request has settled before inspecting cross-tab state.
      await expect.poll(() => other.evaluate(async () => {
        const snapshot = await navigator.locks.query();
        if (!Array.isArray(snapshot.held)) throw new Error("Lock snapshot held array missing");
        return snapshot.held.length;
      }), { timeout }).toBe(0);
      await expect(first.getByTestId("contact-unknown")).toBeVisible({ timeout });
      const recoverOffered = await first.getByTestId("contact-recover").isVisible();
      const summary = await first.getByTestId("contact-summary").count() ? await first.getByTestId("contact-summary").innerText() : null;
      assert.equal(calls.length, 2, "storage event must not POST");
      assert.deepEqual(errors, [], "strict zero pageerrors");
      const result = { status, edited, recoverOffered, summary, unknownText: await first.getByTestId("contact-unknown").innerText(), calls: [...calls] };
      results.push(result); record(result);
      // Recovery in the genuinely ambiguous tab must still use its original bytes/key.
      await other.getByTestId("contact-recover").click();
      await expect.poll(() => calls.length, { timeout }).toBe(3);
      assert.deepEqual(calls[2], calls[1]);
      const storage = await first.evaluate(() => JSON.stringify({ local: { ...localStorage }, session: { ...sessionStorage } }));
      for (const value of [...Object.values(A), ...Object.values(B), ...Object.values(C)]) assert(!storage.includes(value));
    } finally { await context.close(); }
  }
  for (const result of results) {
    assert.equal(result.recoverOffered, false, `${result.status} edit=${result.edited}: refused attempt cannot recover after another tab pending`);
    assert.equal(result.summary, null, `${result.status} edit=${result.edited}: bounded unknown has no original summary`);
    for (const value of ["Synthetic Alpha", "Synthetic Beta", "Synthetic Gamma", "alpha@example.invalid", "beta@example.invalid", "gamma@example.invalid"]) assert(!result.unknownText.includes(value), "bounded unknown must expose no buyer content");
  }
  for (const outcome of ["malformed201", "transport"]) {
    const context = await createContext(options), page = await context.newPage();
    const calls: { key: string | undefined; body: string | null }[] = [], errors: string[] = [];
    page.on("pageerror", e => errors.push(e.message));
    try {
      await page.route("**/api/buyer/contact-messages", async route => {
        calls.push({ key: route.request().headers()["idempotency-key"], body: route.request().postData() });
        if (calls.length > 1) return route.fulfill({ status: 201, json: { message_id: "00000000-0000-4000-8000-000000000001", status: "received" } });
        if (outcome === "transport") return route.abort();
        return route.fulfill({ status: 201, json: { status: "received" } });
      });
      await page.goto(`${origin}/en/contact`);
      for (const [field, value] of Object.entries({ name: "Synthetic Ambiguous", email: "ambiguous@example.invalid", message: "Synthetic ambiguous original" })) await page.getByTestId(`contact-${field}`).fill(value);
      await page.getByTestId("contact-next").click();
      assert.equal(calls.length, 0);
      await page.getByTestId("contact-send").click();
      await expect(page.getByTestId("contact-unknown")).toBeVisible({ timeout });
      await expect(page.getByTestId("contact-summary")).toContainText("Synthetic ambiguous original");
      assert.equal(calls.length, 1, "ambiguous response cannot auto POST");
      await page.getByTestId("contact-recover").click();
      await expect(page.getByTestId("contact-sent")).toBeVisible({ timeout });
      assert.equal(calls.length, 2); assert.deepEqual(calls[1], calls[0]); assert.deepEqual(errors, []);
      record({ outcome, calls, recovered: true });
    } finally { await context.close(); }
  }
  return results;
}

export async function checkContactNative({ browser, origin, evidence }: { browser: Browser; origin: string; evidence: string }) {
  const ledger: Row[] = [];
  const pageErrors: string[] = [];
  let contexts = 0;
  // Contact-scoped context accounting and uncaught-error observation; no browser fixture changes.
  const createContext = async (options: Parameters<Browser["newContext"]>[0]) => {
    const context = await browser.newContext(options);
    contexts++;
    context.on("page", page => page.on("pageerror", error => pageErrors.push(error.message)));
    return context;
  };
  try {
    for (const locale of ["zh-TW", "zh-CN", "en"]) for (const width of [1440, 390]) {
      const height = width === 1440 ? 900 : 844;
      const viewport = `${width}x${height}`;
      const isMobile = width === 390;
      const f = (desktop: string, mobile: string) => isMobile ? mobile : desktop;
      const context = await createContext(ctxOpts({ ignoreHTTPSErrors: true, viewport: { width, height }, locale }));
      const step = async (frame: string, route: string, control: string, operation: string, expected: string, perform: () => Promise<void>) => {
        const row: Row = { namespace, case_id: `${locale}/${viewport}/${control}`, frame, locale, viewport, route, control, operation, expected, actual: "", environment: control === "contact real producer receipt" ? "SANDBOX" : mockEnvironment, status: "FAIL" };
        ledger.push(row);
        try { await perform(); row.actual = "assertions passed"; row.status = "PASS"; }
        catch (error) { row.actual = error instanceof Error ? error.message : String(error); throw error; }
      };
      try {
        // Protocol candidate: the route is wholly intercepted and never sends a buyer message.
        // The three synthetic field values stay in memory; evaluate only reads browser storage for privacy assertions.
        {
          const frame = f("26:850", "26:1117");
          const route = `/${locale}/contact`;
          const contactPage = await context.newPage();
          const attempts: { key: string | undefined; body: string | null }[] = [];
          const synthetic = { name: "Fixture Contact", email: "fixture@example.invalid", message: "Fixture inquiry only" };
          let reply: "uncertain" | "received" = "uncertain";
          const intercept = async (r: import("@playwright/test").Route) => {
            const request = r.request();
            if (request.method() !== "POST") return r.abort();
            attempts.push({ key: request.headers()["idempotency-key"], body: request.postData() });
            if (reply === "uncertain") return r.fulfill({ status: 503, json: { code: "unavailable" } });
            return r.fulfill({ status: 201, json: { message_id: "00000000-0000-4000-8000-000000000001", status: "received" } });
          };
          try {
            await contactPage.route("**/api/buyer/contact-messages", intercept);
            await contactPage.goto(`${origin}${route}`);
            await step(frame, route, "contact protocol entry", "open contact page", "required contact form is visible", async () => {
              await expect(contactPage.getByTestId("contact-form")).toBeVisible({ timeout: 2000 });
            });
              await step(frame, route, "contact uncertainty recovery", "fill and submit; explicitly recover the original request; refresh", "no automatic retry, original key/body retained, confirmed receipt survives refresh, no field values in URL or web storage", async () => {
                await contactPage.getByTestId("contact-name").fill(synthetic.name);
                await contactPage.getByTestId("contact-email").fill(synthetic.email);
                await contactPage.getByTestId("contact-message").fill(synthetic.message);
                await contactPage.getByTestId("contact-next").click();
                await expect(contactPage.getByTestId("contact-confirm")).toBeVisible({ timeout: T });
                assert.equal(attempts.length, 0, "next must not send the message");
                for (const value of Object.values(synthetic)) await expect(contactPage.getByTestId("contact-confirm")).toContainText(value);
                const otherTab = await context.newPage();
                await otherTab.goto(`${origin}${route}`);
                await contactPage.getByTestId("contact-send").click();
                await expect(contactPage.getByTestId("contact-unknown")).toBeVisible({ timeout: T });
                await expect(otherTab.getByTestId("contact-unknown")).toBeVisible({ timeout: T });
                await expect(otherTab.getByTestId("contact-recover")).toHaveCount(0);
                await expect(otherTab.getByTestId("contact-summary")).toHaveCount(0);
                for (const value of Object.values(synthetic)) await expect(contactPage.getByTestId("contact-summary")).toContainText(value);
                await otherTab.close();
                assert.equal(attempts.length, 1, "uncertain result must not be retried automatically");
                assert(attempts[0].key && /^[A-Za-z0-9_.:-]{8,128}$/.test(attempts[0].key), "valid idempotency key required");
                assert.deepEqual(JSON.parse(attempts[0].body ?? "null"), synthetic);
                const storage = await contactPage.evaluate(() => JSON.stringify({ local: { ...localStorage }, session: { ...sessionStorage } }));
                for (const value of Object.values(synthetic)) {
                  assert(!contactPage.url().includes(value), "contact value leaked into URL");
                  assert(!storage.includes(value), "contact value leaked into browser storage");
                }
                reply = "received";
                await contactPage.getByTestId("contact-recover").click();
                await expect(contactPage.getByTestId("contact-sent")).toBeVisible({ timeout: T });
                assert.equal(attempts.length, 2, "one explicit recovery request required");
                assert.deepEqual(attempts[1], attempts[0], "recovery must use the same key and exact body");
                await contactPage.reload();
                await expect(contactPage.getByTestId("contact-sent")).toBeVisible({ timeout: T });
                assert.equal(attempts.length, 2, "refresh must not resend");
              });
              await step(frame, route, "contact uncertainty refresh", "start a separate uncertain attempt and refresh", "unknown remains visible, no automatic resend or false success, and fields are absent from browser storage", async () => {
                const fresh = await context.newPage();
                reply = "uncertain";
                const start = attempts.length;
                try {
                  await fresh.route("**/api/buyer/contact-messages", intercept);
                  await fresh.goto(`${origin}${route}`);
                  await fresh.getByTestId("contact-name").fill(synthetic.name);
                  await fresh.getByTestId("contact-email").fill(synthetic.email);
                  await fresh.getByTestId("contact-message").fill(synthetic.message);
                  await fresh.getByTestId("contact-next").click();
                  await expect(fresh.getByTestId("contact-confirm")).toBeVisible({ timeout: T });
                  await fresh.getByTestId("contact-send").click();
                  await expect(fresh.getByTestId("contact-unknown")).toBeVisible({ timeout: T });
                  assert.equal(attempts.length, start + 1);
                  await fresh.reload();
                  await expect(fresh.getByTestId("contact-summary")).toHaveCount(0);
                  await expect(fresh.getByTestId("contact-recover")).toHaveCount(0);
                  for (const value of Object.values(synthetic)) await expect(fresh.getByTestId("contact-unknown")).not.toContainText(value);
                  await expect(fresh.getByTestId("contact-unknown")).toBeVisible({ timeout: T });
                  await expect(fresh.getByTestId("contact-sent")).toHaveCount(0);
                  assert.equal(attempts.length, start + 1, "refresh must not resend an uncertain submission");
                  const storage = await fresh.evaluate(() => JSON.stringify({ local: { ...localStorage }, session: { ...sessionStorage } }));
                  for (const value of Object.values(synthetic)) assert(!storage.includes(value), "contact value leaked into browser storage");
                } finally {
                  await fresh.unroute("**/api/buyer/contact-messages", intercept);
                  await fresh.close();
                }
              });
          } finally {
            await contactPage.unroute("**/api/buyer/contact-messages", intercept);
            await contactPage.close();
          }
        }

        // Contract-shaped failure injections: no POST reaches the real contact producer in these cases.
        {
          const frame = f("26:850", "26:1117"), route = `/${locale}/contact`;
          for (const status of [429, 409, 422]) {
            await step(frame, route, `contact ${status} refusal`, "fill, review and submit once", "visible refusal, no sent state and no automatic retry (MOCK)", async () => {
              const isolated = await createContext(ctxOpts({ ignoreHTTPSErrors: true, viewport: { width, height }, locale }));
              const buyer = await isolated.newPage(); let posts = 0;
              try {
                await isolated.route("**/api/buyer/contact-messages", async r => { posts++; await r.fulfill({ status, json: { code: status === 429 ? "rate_limited" : status === 409 ? "conflict" : "invalid_request" } }); });
                await buyer.goto(`${origin}${route}`);
                await buyer.getByTestId("contact-name").fill("Fixture Contact");
                await buyer.getByTestId("contact-email").fill("fixture@example.invalid");
                await buyer.getByTestId("contact-message").fill("Fixture inquiry only");
                await buyer.getByTestId("contact-next").click();
                await buyer.getByTestId("contact-send").click();
                await expect(buyer.locator("main").getByRole("alert")).toBeVisible({ timeout: T });
                await expect(buyer.getByTestId("contact-sent")).toHaveCount(0);
                if (status === 429) {
                  for (const value of ["Fixture Contact", "fixture@example.invalid", "Fixture inquiry only"]) await expect(buyer.getByTestId("contact-summary")).toContainText(value);
                }
                assert.equal(posts, 1, "a refusal must not trigger another POST");
              } finally { await isolated.close(); }
            });
          }
          await step(frame, route, "contact concurrent tabs", "submit from two already-open tabs concurrently", "one intercepted POST, both tabs visibly uncertain (MOCK)", async () => {
            const isolated = await createContext(ctxOpts({ ignoreHTTPSErrors: true, viewport: { width, height }, locale }));
            const first = await isolated.newPage(), second = await isolated.newPage(); let posts = 0;
            try {
              await isolated.route("**/api/buyer/contact-messages", async r => { posts++; await r.fulfill({ status: 503, json: { code: "unavailable" } }); });
              for (const buyer of [first, second]) {
                await buyer.goto(`${origin}${route}`);
                await buyer.getByTestId("contact-name").fill("Fixture Contact");
                await buyer.getByTestId("contact-email").fill("fixture@example.invalid");
                await buyer.getByTestId("contact-message").fill("Fixture inquiry only");
                await buyer.getByTestId("contact-next").click();
              }
              await Promise.allSettled([first.getByTestId("contact-send").click({ timeout: 1500 }), second.getByTestId("contact-send").click({ timeout: 1500 })]);
              await expect.poll(() => posts, { timeout: T }).toBe(1);
              await expect(first.getByTestId("contact-unknown")).toBeVisible({ timeout: T });
              await expect(second.getByTestId("contact-unknown")).toBeVisible({ timeout: T });
              assert.equal(posts, 1, "competing tabs must not submit twice");
            } finally { await isolated.close(); }
          });
          await step(frame, route, "contact unavailable locks", "submit in a browser without Web Locks", "visible refusal and no POST (MOCK)", async () => {
            const isolated = await createContext(ctxOpts({ ignoreHTTPSErrors: true, viewport: { width, height }, locale }));
            await isolated.addInitScript(() => Object.defineProperty(navigator, "locks", { value: undefined }));
            const buyer = await isolated.newPage(); let posts = 0;
            try {
              await isolated.route("**/api/buyer/contact-messages", async r => { posts++; await r.abort(); });
              await buyer.goto(`${origin}${route}`);
              await buyer.getByTestId("contact-name").fill("Fixture Contact");
              await buyer.getByTestId("contact-email").fill("fixture@example.invalid");
              await buyer.getByTestId("contact-message").fill("Fixture inquiry only");
              await buyer.getByTestId("contact-next").click();
              await buyer.getByTestId("contact-send").click();
              await expect(buyer.locator("main").getByRole("alert")).toBeVisible({ timeout: T });
              assert.equal(posts, 0, "unsupported locking must fail closed");
            } finally { await isolated.close(); }
          });
        }

        // One synthetic message traverses the real Next BFF, Go handler and isolated PG fixture.
        // The fixture owns random encryption material and no mail worker; failure-injection cases above are MOCK.
        if (locale === "en" && isMobile) {
          const frame = "26:1117";
          const route = `/${locale}/contact`;
          await step(frame, route, "contact real producer receipt", "fill, review, submit once and reload", "real BFF/Go returns 201, receipt survives refresh without another POST", async () => {
            const isolated = await createContext(ctxOpts({ ignoreHTTPSErrors: true, viewport: { width, height }, locale }));
            const buyer = await isolated.newPage();
            const posts: import("@playwright/test").Request[] = [];
            buyer.on("request", request => {
              if (new URL(request.url()).pathname === "/api/buyer/contact-messages" && request.method() === "POST") posts.push(request);
            });
            try {
              await buyer.goto(`${origin}${route}`);
              await buyer.getByTestId("contact-name").fill("Synthetic Test Buyer");
              await buyer.getByTestId("contact-email").fill("sfr-buyer@example.invalid");
              await buyer.getByTestId("contact-message").fill("Synthetic storefront browser fixture");
              await buyer.getByTestId("contact-next").click();
              await expect(buyer.getByTestId("contact-confirm")).toBeVisible({ timeout: T });
              assert.equal(posts.length, 0, "review must not send");
              const receipt = buyer.waitForResponse(response => new URL(response.url()).pathname === "/api/buyer/contact-messages" && response.request().method() === "POST", { timeout: T });
              await buyer.getByTestId("contact-send").click();
              assert.equal((await receipt).status(), 201, "real producer must return a committed receipt");
              await expect(buyer.getByTestId("contact-sent")).toBeVisible({ timeout: T });
              assert.equal(posts.length, 1);
              await buyer.reload();
              await expect(buyer.getByTestId("contact-sent")).toBeVisible({ timeout: T });
              assert.equal(posts.length, 1, "reload must not resend the contact message");
            } finally { await isolated.close(); }
          });
        }

      } finally { await context.close(); }
    }
    // Emit lifecycle PASS rows only after the entire extracted function, including recovery/privacy assertions, returns.
    const lifecycle: unknown[] = [];
    await checkContactAttemptLifecycle(createContext, ctxOpts({ ignoreHTTPSErrors: true, viewport: { width: 1440, height: 900 }, locale: "en" }), origin, T, value => lifecycle.push(value));
    assert.equal(lifecycle.length, 12);
    for (const value of lifecycle) {
      const result = value as { status?: number; edited?: boolean; outcome?: string };
      const id = result.outcome ? `lifecycle/${result.outcome}` : `lifecycle/${result.status}/edited=${result.edited}`;
      ledger.push({ namespace, case_id: id, frame: "26:850", locale: "en", viewport: "1440x900", route: "/en/contact", control: id, operation: "real fill/review/send/refuse/cross-tab/recover", expected: "terminal attempt cannot recover; ambiguous attempt retains exact key/body; no PII storage or automatic POST", actual: "assertions passed", environment: mockEnvironment, status: "PASS" });
    }
    assert.deepEqual(pageErrors, [], "strict zero pageerrors across contact contexts");
    assert.equal(contexts, 49);
    assert.equal(ledger.length, 61);
    assert.equal(new Set(ledger.map(row => row.case_id)).size, 61);
    assert.equal(ledger.filter(row => row.environment === "MOCK").length, 60);
    assert.equal(ledger.filter(row => row.environment === "SANDBOX").length, 1);
    assert(ledger.every(row => row.status === "PASS"));
  } finally {
    // Partial failure ledger survives; no complete marker is emitted on an exception.
    await writeFile(path.join(evidence, "contact-click-ledger.json"), JSON.stringify({ namespace, contexts, skips: 0, rows: ledger }, null, 2) + "\n");
  }
  console.log("M8_CONTACT_NATIVE_COMPLETE namespace=M8-CONTACT-NATIVE/v1 mock=60 sandbox=1 contexts=49 skips=0");
}
