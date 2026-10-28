// Purpose: OC-11 / OC-Q10 admin gate: an uncertain publish shows 「發布結果待確認」, blocks re-publish, survives a reload and rechecks by READ only.
// Depends on: @playwright/test, design-copy; the same harness env as tests/admin/design.spec.ts (LC_BROWSER_PUBLIC_ORIGIN, LC_BROWSER_BFF_STORE).
// Used by: tests/foundation/browser_store_design_test.go (same playwright invocation as design.spec.ts; playwright.config.ts store-design testMatch).
// Publish answers are cut by interception only (route.fetch then abort = committed but unheard; plain abort = never reached Go); every user step is a
// real click/fill. Tests share one store and run in file order (workers: 1).
import { expect, test, type Page, type Route } from "@playwright/test";
import { designCopy, fill } from "../../../apps/admin/lib/design-copy";

const required = (name: string) => {
  const value = process.env[name];
  if (!value) throw new Error(`${name} is required`);
  return value;
};
const origin = required("LC_BROWSER_PUBLIC_ORIGIN");
// The BFF-probe store: its own merchant, and design.spec.ts's PG-fact journeys (exact version counts) stay untouched.
const store = required("LC_BROWSER_BFF_STORE");
const c = designCopy.en;
const okPublished = new RegExp(c.okPublished.replace("{n}", "\\d+"));
test.use({ baseURL: origin });

type Cut = "none" | "after-commit" | "before-commit" | "held-before-commit";
async function open(page: Page) {
  const publishes: { key: string | undefined; body: string | null }[] = [];
  const writes: string[] = [];
  const state = { cut: "none" as Cut, release: () => {} };
  await page.route("**/design/publish", async (route: Route) => {
    const req = route.request();
    publishes.push({ key: req.headers()["idempotency-key"], body: req.postData() });
    if (state.cut === "after-commit") {
      await route.fetch(); // Go commits...
      await route.abort("connectionreset"); // ...but the browser never hears
    } else if (state.cut === "held-before-commit") {
      // Install the release BEFORE doing any I/O: request arrival is the exact in-flight boundary.
      await new Promise<void>((resolve) => (state.release = resolve));
      await route.abort("connectionreset").catch(() => {}); // the page that asked is gone; no Go write
    } else if (state.cut === "before-commit") await route.abort("connectionreset"); // never reached Go
    else await route.continue();
  });
  page.on("request", (r) => {
    if (r.method() !== "GET" && !r.url().includes("/design/publish")) writes.push(`${r.method()} ${r.url()}`);
  });
  page.on("dialog", (d) => void d.accept());
  await page.goto(new URL("/en/", origin).toString());
  await page.getByRole("button", { name: "Sign in with identity service" }).click();
  await page.getByTestId("nav-group-storefront").waitFor({ state: "attached" });
  await page.goto(`/en/design?store=${store}`);
  await expect(page.getByTestId("design-name"), "initial editor is ready before any fault injection").toBeVisible();
  return { publishes, writes, state };
}
const edit = (page: Page, tag: string) => page.getByTestId("design-name").fill(`oc11 ${tag} ${Date.now()}`);
async function recheckSettled(page: Page) {
  const read = page.waitForResponse((r) => r.url().endsWith("/design/versions") && r.request().method() === "GET");
  await page.getByTestId("design-publish-recheck").click();
  await read;
  const button = page.getByTestId("design-publish-recheck");
  await expect.poll(async () => (await button.count()) === 0 || await button.isEnabled(), {
    message: "the recheck read settled and its busy state cleared",
  }).toBe(true);
}

test("OC11 publish lost after commit: pending, no second publish, recheck finds it", async ({ page }) => {
  expect(designCopy["zh-TW"].publishPending).toBe("發布結果待確認");
  expect(designCopy["zh-CN"].publishPending).toBe("发布结果待确认");
  const { publishes, writes, state } = await open(page);
  state.cut = "after-commit";
  await edit(page, "a");
  await page.getByTestId("design-publish").click();

  const pending = page.getByTestId("design-publish-pending");
  await expect(pending).toContainText(c.publishPending);
  await expect(page.getByTestId("design-note")).toHaveCount(0); // never a success note
  await expect(page.getByTestId("design-publish")).toBeDisabled(); // no second publish possible
  expect(publishes, "OC11-A1 exactly one publish before recheck").toHaveLength(1);
  writes.length = 0; // the save before publish is legitimate; from here on nothing may be written

  const recheckRead = page.waitForResponse((r) => r.url().endsWith("/design/versions") && r.request().method() === "GET");
  await page.getByTestId("design-publish-recheck").click();
  await expect(page.getByTestId("design-note")).toContainText(okPublished);
  await expect(pending).toHaveCount(0);
  expect(publishes, "OC11-A2 recheck is a read: still exactly one publish request, one key").toHaveLength(1);
  expect(writes, "OC11-A3 no other write was sent").toEqual([]);
  const history = await (await recheckRead).json();
  expect(history.items.find((v: { idempotency_key?: string }) => v.idempotency_key === publishes[0].key),
    "OC11-P1a versions expose the committed original request key, not just a draft version").toBeTruthy();
});

test("OC11 publish never reached Go: survives reload, failed read is unconfirmed, empty read is not-yet, never success", async ({ page }) => {
  const { publishes, state } = await open(page);
  state.cut = "before-commit";
  await edit(page, "b");
  await page.getByTestId("design-publish").click();
  await expect(page.getByTestId("design-publish-pending")).toContainText(c.publishPending);
  expect(publishes).toHaveLength(1);

  await page.reload(); // OC11-A5: UNKNOWN is still UNKNOWN after a refresh
  await expect(page.getByTestId("design-publish-pending"), "OC11-A5 pending survives a reload").toContainText(c.publishPending);
  await expect(page.getByTestId("design-publish"), "OC11-A5 Publish stays disabled after reload").toBeDisabled();

  await page.route("**/design/versions", (route) => route.abort("connectionreset")); // the recheck read itself fails
  await page.getByTestId("design-publish-recheck").click();
  await expect(page.getByTestId("design-publish-unconfirmed"), "OC11-A6 a failed read is 'could not confirm', not 'no record'").toContainText(c.publishUnconfirmed);
  await expect(page.getByTestId("design-publish-notyet")).toHaveCount(0);
  await expect(page.getByTestId("design-note")).toHaveCount(0);
  await page.unroute("**/design/versions");

  await page.getByTestId("design-publish-recheck").click();
  await expect(page.getByTestId("design-publish-notyet"), "OC11-A7 empty read: not recorded yet").toBeVisible();
  await expect(page.getByTestId("design-publish-pending")).toBeVisible();
  await expect(page.getByTestId("design-publish")).toBeDisabled();
  await expect(page.getByTestId("design-note")).toHaveCount(0);
  expect(publishes, "OC11-A8 still one publish request").toHaveLength(1);
});

test("OC11 old publish row of the same draft version never confirms a new lost publish (publish, rollback, publish again)", async ({ page }) => {
  const { state } = await open(page);
  await edit(page, "c");
  await page.getByTestId("design-publish").click(); // real publish of this draft version
  await expect(page.getByTestId("design-note")).toContainText(okPublished);
  await page.getByTestId("design-tab-versions").click();
  await page.getByTestId("design-rollback").last().click(); // rollback: the newest row is now a rollback, the draft is unchanged
  await expect(page.getByTestId("design-note")).toContainText(/Rolled back/);
  await expect(page.getByTestId("design-publish")).toBeEnabled();

  state.cut = "before-commit"; // publish this same draft version again; the answer is lost and nothing committed
  await page.getByTestId("design-publish").click();
  await expect(page.getByTestId("design-publish-pending")).toContainText(c.publishPending);
  await page.getByTestId("design-publish-recheck").click();
  await expect(page.getByTestId("design-publish-notyet"), "OC11-A4 the old publish row (source_version equal) must not confirm").toBeVisible();
  await expect(page.getByTestId("design-publish-pending")).toBeVisible();
  await expect(page.getByTestId("design-publish")).toBeDisabled();
  await expect(page.getByTestId("design-note"), "no success note of any kind").toHaveCount(0);
});

test("OC11 reload while the publish request is in flight: pending is armed before the request, no second publish", async ({ page }) => {
  // Transport fixture only: do not deliver a cancellation result to the departing document.
  // An ordinary reload can reject fetch before teardown, letting the broken arm-after-response
  // order persist pending just in time and mask A10. Model the response remaining unheard until
  // the document is gone; native requests, route holding, reload and every user click stay real.
  await page.addInitScript(() => {
    const nativeFetch = window.fetch.bind(window);
    window.fetch = async (...args: Parameters<typeof fetch>) => {
      const [input, init] = args;
      const url = typeof input === "string" ? input : input instanceof URL ? input.href : input.url;
      if (init?.method === "POST" && url.endsWith("/design/publish")) {
        try { return await nativeFetch(...args); }
        catch { return new Promise<Response>(() => {}); }
      }
      return nativeFetch(...args);
    };
  });
  const { publishes, state } = await open(page);
  state.cut = "held-before-commit";
  await edit(page, "d");
  await page.getByTestId("design-publish").click();
  await expect.poll(() => publishes.length, { message: "the publish request left the page" }).toBe(1);
  // READ/MEASURE only: inspect the real persisted request while the POST cannot complete.
  const armed = await page.evaluate((s) => JSON.parse(sessionStorage.getItem(`lc.design.publishPending.${s}`) ?? "null"), store);
  expect.soft(armed?.key, "OC11-A9 identity is durable while the original POST is held").toBe(publishes[0].key);
  try {
    await page.reload();
    await expect(page.getByTestId("design-name")).toBeVisible();
    await expect.soft(page.getByTestId("design-publish-pending"), "OC11-A9 pending armed before the request").toContainText(c.publishPending);
    // The editor is ready: sample the control now. Playwright timeout: 0 disables its deadline,
    // which would strand this soft assertion and hide the independent A10 POST-count failure.
    expect.soft(await page.getByTestId("design-publish").isDisabled(), "OC11-A9 Publish stays locked after reload").toBe(true);
    // Exercise the accidentally enabled control on the unfixed order so A10 has its OWN assertion witness.
    state.cut = "before-commit";
    if (await page.getByTestId("design-publish").isEnabled()) {
      const retryFailed = page.waitForEvent("requestfailed", (r) => r.url().endsWith("/design/publish"));
      await page.getByTestId("design-publish").click();
      await retryFailed; // event boundary: the accidental POST has actually passed the route interceptor
    }
    expect(publishes, "OC11-A10 the publish POST count stays 1 after reload and an available Publish click").toHaveLength(1);
  } finally {
    state.release();
  }
});

test("OC11 stale versions list: confirmed publish, failed refresh, second publish aborted before commit never shows success", async ({ page }) => {
  const { state } = await open(page);
  await edit(page, "e"); // wait for the editor's initial draft/media/versions load before injecting the refresh failure
  await page.route("**/design/versions", (route) => route.abort("connectionreset")); // only subsequent versions reads fail
  await page.getByTestId("design-publish").click(); // real publish; its answer arrives, the list refresh after it fails
  await expect(page.getByTestId("design-note")).toContainText(okPublished);
  state.cut = "before-commit"; // the same draft again; nothing commits
  await page.getByTestId("design-publish").click();
  await expect(page.getByTestId("design-publish-pending")).toContainText(c.publishPending);
  await page.unroute("**/design/versions");
  await page.getByTestId("design-publish-recheck").click();
  await expect(page.getByTestId("design-publish-notyet"), "OC11-A11 the first (confirmed) publish row must not confirm the second").toBeVisible();
  await expect(page.getByTestId("design-publish-pending")).toBeVisible();
  await expect(page.getByTestId("design-note")).toHaveCount(0);
});

test("OC11 P1a another tab publishes the same draft: only its own key can confirm a pending request", async ({ page, context }) => {
  const { publishes, state } = await open(page);
  await edit(page, "two-tabs");
  await page.getByTestId("design-save").click();
  await expect(page.getByTestId("design-note")).toContainText(c.okSaved);
  const other = await context.newPage();
  other.on("dialog", (d) => void d.accept());
  await other.goto(`/en/design?store=${store}`); // second tab, same authenticated merchant; no second login
  await expect(other.getByTestId("design-name")).toBeVisible();
  await other.getByTestId("design-publish").click();
  await expect(other.getByTestId("design-note")).toContainText(okPublished);
  state.cut = "before-commit"; // A's stale request never reaches Go; B's publish is the only committed one.
  await page.getByTestId("design-publish").click();
  await expect(page.getByTestId("design-publish-pending")).toBeVisible();
  await recheckSettled(page);
  await expect(page.getByTestId("design-note"), "OC11-P1a another request must never be reported as our success").toHaveCount(0);
  await expect(page.getByTestId("design-publish-pending")).toBeVisible();
  expect(publishes).toHaveLength(1);
});

test("OC11 P1b pending blocks local rollback, external rollback resolves the committed request as superseded", async ({ page, context }) => {
  const { publishes, state } = await open(page);
  // Establish our own rollback target; do not depend on an earlier test's success.
  await edit(page, "rollback-base");
  await page.getByTestId("design-publish").click();
  await expect(page.getByTestId("design-note")).toContainText(okPublished);
  await edit(page, "rollback-pending");
  state.cut = "after-commit";
  await page.getByTestId("design-publish").click();
  await expect(page.getByTestId("design-publish-pending")).toBeVisible();
  await page.getByTestId("design-tab-versions").click();
  expect.soft(await page.getByTestId("design-rollback").last().isDisabled(), "OC11-P1b local rollback cannot race an unresolved publish").toBe(true);

  const other = await context.newPage();
  other.on("dialog", (d) => void d.accept());
  await other.goto(`/en/design?store=${store}`);
  await other.getByTestId("design-tab-versions").click();
  await other.getByTestId("design-rollback").last().click();
  await expect(other.getByTestId("design-note")).toContainText(/Rolled back/);
  await recheckSettled(page);
  await expect.soft(page.getByTestId("design-publish-pending"), "OC11-P1b the original committed receipt resolves despite a newer rollback").toHaveCount(0);
  // recheckSettled already observed completion; take independent snapshots so every oracle runs on RED.
  expect.soft(await page.getByTestId("design-publish-superseded").isVisible(), "OC11-P1b explain that a later version replaced this publication").toBe(true);
  expect.soft(await page.getByTestId("design-publish").isEnabled(), "OC11-P1b terminal resolution unlocks Publish").toBe(true);
  await page.reload();
  await expect(page.getByTestId("design-name")).toBeVisible();
  expect.soft(await page.getByTestId("design-publish-pending").count(), "OC11-P1b terminal resolution survives refresh").toBe(0);
  await expect(page.getByTestId("design-publish")).toBeEnabled();
  expect(publishes, "resolution never sends a replacement publish").toHaveLength(2);
});

test("OC11 P1b a hung recheck has a bounded read and releases its busy control", async ({ page }) => {
  const { state } = await open(page);
  await edit(page, "bounded-read");
  state.cut = "before-commit";
  await page.getByTestId("design-publish").click();
  await expect(page.getByTestId("design-publish-pending")).toBeVisible();
  let release = () => {};
  await page.route("**/design/versions", async (route) => {
    await new Promise<void>((done) => { release = done; });
    await route.abort("connectionreset").catch(() => {});
  });
  try {
    await page.getByTestId("design-publish-recheck").click();
    await expect(page.getByTestId("design-publish-recheck")).toBeDisabled();
    await expect(page.getByTestId("design-publish-recheck"), "OC11-P1b a hung read releases Recheck within five seconds plus scheduling margin").toBeEnabled({ timeout: 7000 });
    await expect(page.getByTestId("design-publish-unconfirmed")).toBeVisible();
    await expect(page.getByTestId("design-note")).toHaveCount(0);
  } finally {
    release();
  }
});

for (const locale of ["zh-TW", "zh-CN", "en"] as const) {
  test(`OC11 pending and read-only recheck render in ${locale} and survive refresh`, async ({ page }) => {
    const { publishes, state } = await open(page);
    await page.goto(`/${locale}/design?store=${store}`);
    await edit(page, `locale-${locale}`);
    state.cut = "before-commit";
    await page.getByTestId("design-publish").click();
    await expect(page.getByTestId("design-publish-pending")).toContainText(designCopy[locale].publishPending);
    await page.reload();
    await expect(page.getByTestId("design-publish-pending")).toContainText(designCopy[locale].publishPending);
    await page.getByTestId("design-publish-recheck").click();
    await expect(page.getByTestId("design-publish-notyet")).toBeVisible();
    await expect(page.getByTestId("design-note")).toHaveCount(0);
    expect(publishes, "locale recovery never resends the original publish").toHaveLength(1);
  });
}
