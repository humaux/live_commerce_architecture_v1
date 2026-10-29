// Purpose: OC-11 / OC-Q10 admin gate: an uncertain publish shows 「發布結果待確認」, blocks re-publish, survives a reload and resolves only with the original request identity (R1-R5).
// Depends on: @playwright/test, design-copy, design-model (VersionInfo only); the same harness env as tests/admin/design.spec.ts (LC_BROWSER_PUBLIC_ORIGIN, LC_BROWSER_BFF_STORE).
// Used by: tests/foundation/browser_store_design_test.go (same playwright invocation as design.spec.ts; playwright.config.ts store-design testMatch).
// Invariants: transport loss never implies an outcome; only the original key/body POST receipt confirms, with one committed row per attempt.
// Publish answers are cut by interception only (route.fetch then abort = committed but unheard; plain abort = never reached Go); every user step is a
// real click/fill. Tests share one store and run in file order (workers: 1).
import { expect, test, type Page, type Route } from "@playwright/test";
import { designCopy, fill } from "../../../apps/admin/lib/design-copy";
import { entryCopy } from "../../../apps/admin/lib/entry-copy";
import type { VersionInfo } from "../../../apps/admin/lib/design-model";

// Signed navigation is exercised with real clicks; no API/storage/DOM shortcuts.
async function editorTask(page: Page, task: 'profile' | 'home' | 'pages' | 'nav' | 'versions') {
  // Loaded editor exists; its duplicate toolbar is collapsed in review/results. Task clicks below assert the visible destination.
  await expect(page.getByTestId('design-status')).toHaveCount(1);
  if (await page.getByTestId(`design-${task}`).isVisible()) return;
  if (await page.getByTestId('design-back').isVisible()) await page.getByTestId('design-back').click();
  await page.getByTestId(task === 'versions' ? 'design-tab-versions' : `design-task-${task}`).click();
  await expect(page.getByTestId(`design-${task}`)).toBeVisible();
}
async function editorPublish(page: Page) {
  if (await page.getByTestId('design-back').isVisible()) await page.getByTestId('design-back').click();
  await page.getByTestId('design-publish').click();
  await expect(page.getByTestId('design-publish-review')).toBeVisible();
  test.info().annotations.push({ type: 'publication-confirmation', description: await page.getByTestId('design-publish-review').getByRole('heading').innerText() });
  await page.getByTestId('design-publish-confirm').click();
}

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

const sessionRecoveryConfirm = {
  "zh-TW": "確認先前草稿 v{n} 的發布請求？如果尚未完成，這會將該草稿發布給買家。",
  "zh-CN": "确认之前草稿 v{n} 的发布请求？如果尚未完成，这会将该草稿发布给买家。",
  en: "Confirm the earlier publish request for draft v{n}? If it has not completed, this will publish that draft for buyers.",
};

// Integrator ruling: count attempts, not HTTP transmissions. Every replay retains the exact key AND bytes.
function expectAttempts(publishes: { key: string | undefined; body: string | null }[], count: number) {
  const attempts = new Map<string, string | null>();
  for (const request of publishes) {
    expect(request.key, "each publish transmission carries its request identity").toMatch(/^[A-Za-z0-9_.:-]{8,128}$/);
    const key = request.key!;
    if (attempts.has(key)) expect(request.body, "a replay must preserve the exact original body").toBe(attempts.get(key));
    else attempts.set(key, request.body);
  }
  expect(attempts.size, "one key per attempt; retries must never create a replacement attempt").toBe(count);
}

type Cut = "none" | "after-commit" | "before-commit" | "held-before-commit";
async function open(page: Page) {
  const publishes: { key: string | undefined; body: string | null }[] = [];
  const committed: VersionInfo[] = []; // transport-fixture observations, never supplied to the product
  const writes: string[] = [];
  const state = { cut: "none" as Cut, release: () => {} };
  await page.route("**/design/publish", async (route: Route) => {
    const req = route.request();
    publishes.push({ key: req.headers()["idempotency-key"], body: req.postData() });
    if (state.cut === "after-commit") {
      const answer = await route.fetch(); // Go commits (or replays the same receipt)...
      expect(answer.status(), "the lost answer really committed").toBe(200);
      committed.push(await answer.json());
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
  await editorTask(page, "profile");
  await expect(page.getByTestId("design-name"), "initial editor is ready before any fault injection").toBeVisible();
  return { publishes, committed, writes, state };
}
const edit = async (page: Page, tag: string) => {
  await editorTask(page, 'profile');
  await page.getByTestId('design-name').fill(`oc11 ${tag} ${Date.now()}`);
};
async function recheckSettled(page: Page) {
  const button = page.getByTestId("design-publish-recheck");
  await expect(button).toBeEnabled(); // automatic same-key recovery must settle before a merchant click
  const sent = page.waitForRequest((r) => r.url().endsWith("/design/publish") && r.method() === "POST");
  await page.getByTestId("design-publish-recheck").click();
  const response = await (await sent).response(); // null on transport failure; never wait for an unrelated versions GET
  // READ only: one snapshot avoids count -> locator removal -> isEnabled waiting for a vanished button.
  await expect.poll(() => button.evaluateAll(nodes => nodes.length === 0 || !(nodes[0] as HTMLButtonElement).disabled), {
    message: "the request-bound recovery settled and its busy state cleared",
  }).toBe(true);
  return response;
}

for (const locale of ["zh-TW", "zh-CN", "en"] as const) {
  for (const cut of ["after-commit", "before-commit"] as const) {
    test(`OC11 RVFIX P1 ${locale} ${cut}: new login requires explicit recovery of the original request`, async ({ page }) => {
      const { publishes, committed, state } = await open(page);
      await page.goto(`/${locale}/design?store=${store}`);
      await edit(page, `rvfix-${locale}-${cut}`);
      state.cut = cut;
      await editorPublish(page);
      await expect(page.getByTestId("design-publish-unconfirmed")).toBeVisible();
      await expect(page.getByTestId("design-publish-recheck")).toBeEnabled();
      expectAttempts(publishes, 1);
      // Read-only journal measurement: the fixture never writes browser storage or changes session identity.
      const journal = () => page.evaluate((id) => sessionStorage.getItem(`lc.design.publishPending.${id}`), store);
      const original = await journal();
      expect(original).not.toBeNull();
      const attempt = JSON.parse(original!);
      expect(attempt.retries).toBe(3);

      const signOut = page.getByTestId("workspace-sign-out");
      await page.locator("details").filter({ has: signOut }).locator("summary").click();
      await signOut.click(); // real logout/login rotates the production CSRF/session boundary
      await page.getByRole("button", { name: entryCopy[locale].signIn, exact: true }).click();
      await page.getByTestId("nav-group-storefront").waitFor({ state: "attached" });
      const sentBeforeReturn = publishes.length;
      state.cut = "none";
      await page.goto(`/${locale}/design?store=${store}`);
      await editorTask(page, "profile");
      await expect(page.getByTestId("design-name")).toBeVisible();
      await expect(page.getByTestId("design-publish-pending")).toContainText(designCopy[locale].publishPending);
      await expect(page.getByTestId("design-publish")).toBeDisabled();
      // Observe beyond the entire 250+500+1000ms automatic retry schedule: a fresh login must not replay old intent.
      await page.waitForTimeout(2000);
      expect(publishes.length, "RVFIX-P1 no automatic cross-session publish").toBe(sentBeforeReturn);
      expect(await journal(), "RVFIX-P1 login must preserve the original fenced identity").toBe(original);
      await page.reload();
      await expect(page.getByTestId("design-publish-pending")).toBeVisible();
      const recover = page.getByTestId("design-publish-recheck");
      await expect(recover, "RVFIX-P1 a new login must offer a usable explicit recovery path").toBeEnabled();

      page.removeAllListeners("dialog");
      const confirmations: string[] = [];
      let acceptRecovery = false;
      page.on("dialog", async (dialog) => {
        expect(dialog.type()).toBe("confirm");
        confirmations.push(dialog.message());
        if (acceptRecovery) await dialog.accept();
        else await dialog.dismiss();
      });
      await recover.click();
      expect(confirmations, "RVFIX-P1 old intent requires informed confirmation in the current language")
        .toEqual([fill(sessionRecoveryConfirm[locale], { n: attempt.version })]);
      expect(publishes.length, "RVFIX-P1 cancelling recovery sends nothing").toBe(sentBeforeReturn);
      expect(await journal(), "RVFIX-P1 cancelling recovery retains the unresolved original identity").toBe(original);
      await expect(page.getByTestId("design-publish")).toBeDisabled();

      acceptRecovery = true;
      const response = page.waitForResponse((r) => r.url().endsWith("/design/publish") && r.request().method() === "POST");
      await recover.click();
      const receipt = await response;
      expect(receipt.status()).toBe(200);
      expect(receipt.request().headers()["idempotency-key"]).toBe(attempt.key);
      expect(receipt.request().postData()).toBe(publishes[0].body);
      const result: VersionInfo = await receipt.json();
      expect(result.source_version).toBe(attempt.version);
      if (cut === "after-commit") expect(result).toEqual(committed[0]);
      await expect(page.getByTestId("design-note")).toContainText(fill(designCopy[locale].okPublished, { n: result.version }));
      await expect(page.getByTestId("design-publish-pending")).toHaveCount(0);
      expectAttempts(publishes, 1);
      await expect.poll(journal).toBeNull();
      await page.reload();
      await editorTask(page, "profile");
      await expect(page.getByTestId("design-name")).toBeVisible();
      await expect(page.getByTestId("design-publish-pending")).toHaveCount(0);
      await edit(page, "rvfix-recovered");
      await expect(page.getByTestId("design-publish"), "RVFIX-P1 publishing is unlocked after authenticated recovery").toBeEnabled();
      await page.getByTestId("design-save").click();
      await expect(page.getByTestId("design-note")).toContainText(designCopy[locale].okSaved);
    });
  }
}

test("OC11 publish lost after commit: pending, no second publish, recheck finds it", async ({ page }) => {
  expect(designCopy["zh-TW"].publishPending).toBe("發布結果待確認");
  expect(designCopy["zh-CN"].publishPending).toBe("发布结果待确认");
  const { publishes, committed, writes, state } = await open(page);
  state.cut = "after-commit";
  await edit(page, "a");
  await editorPublish(page);

  const pending = page.getByTestId("design-publish-pending");
  await expect(pending).toContainText(c.publishPending);
  await expect(page.getByTestId("design-note")).toHaveCount(0); // never a success note
  await expect(page.getByTestId("design-publish")).toBeDisabled(); // no second publish possible
  await expect(page.getByTestId("design-publish-unconfirmed")).toBeVisible(); // the first failed transport has returned, automatic recovery is armed
  expectAttempts(publishes, 1);
  writes.length = 0; // the save is legitimate; only the original-key recovery may be transmitted now

  await recheckSettled(page); // transport still down: even a stored receipt cannot yet confirm the result
  await expect(pending).toContainText(c.publishPending);
  await expect(page.getByTestId("design-note")).toHaveCount(0);
  await expect(page.getByTestId("design-publish")).toBeDisabled();
  state.cut = "none"; // restore transport; the same key/body POST must return the stored receipt

  const recheckRead = page.waitForResponse((r) => r.url().endsWith("/design/versions") && r.request().method() === "GET");
  const receipt = await recheckSettled(page);
  expect(receipt?.status(), "the original request has a successful recovery receipt").toBe(200);
  expect(receipt!.request().headers()["idempotency-key"]).toBe(publishes[0].key);
  expect(receipt!.request().postData()).toBe(publishes[0].body);
  const returned: VersionInfo = await receipt!.json();
  expect(returned, "R2 same key/body returns the first stored answer").toEqual(committed[0]);
  expect(committed.every((v) => v.version === returned.version)).toBe(true);
  await expect(page.getByTestId("design-note")).toContainText(fill(c.okPublished, { n: returned.version }));
  await expect(pending).toHaveCount(0);
  expectAttempts(publishes, 1);
  expect(writes, "OC11-A3 no other write was sent").toEqual([]);
  const history = await (await recheckRead).json();
  const sentVersion = JSON.parse(publishes[0].body!).expected_draft_version;
  expect(returned.source_version).toBe(sentVersion);
  expect(returned.kind).toBe("publish");
  expect(history.items.filter((v: { kind: string; source_version: number }) => v.kind === "publish" && v.source_version === sentVersion),
    "R2 one committed published row for the single attempt").toHaveLength(1);
  expect(history.items.find((v: VersionInfo) => v.version === returned.version),
    "OC11-P1a the header-bound receipt identifies the exact committed history version").toEqual(returned);
});

test("OC11 publish never reached Go: reload stays pending offline, restored same-key resend commits once", async ({ page }) => {
  const { publishes, state } = await open(page);
  state.cut = "before-commit";
  await edit(page, "b");
  await editorPublish(page);
  await expect(page.getByTestId("design-publish-pending")).toContainText(c.publishPending);
  await expect(page.getByTestId("design-publish-unconfirmed")).toBeVisible();
  await expect(page.getByTestId("design-publish-recheck")).toBeEnabled();
  expectAttempts(publishes, 1);

  await page.reload(); // OC11-A5: UNKNOWN is still UNKNOWN after a refresh
  await expect(page.getByTestId("design-publish-pending"), "OC11-A5 pending survives a reload").toContainText(c.publishPending);
  await expect(page.getByTestId("design-publish"), "OC11-A5 Publish stays disabled after reload").toBeDisabled();

  await page.route("**/design/versions", (route) => route.abort("connectionreset")); // display failures cannot resolve a request
  await recheckSettled(page);
  await expect(page.getByTestId("design-publish-unconfirmed"), "OC11-A6 a failed recovery is 'could not confirm', not 'no record'").toContainText(c.publishUnconfirmed);
  await expect(page.getByTestId("design-publish-notyet")).toHaveCount(0);
  await expect(page.getByTestId("design-note")).toHaveCount(0);
  await page.unroute("**/design/versions");

  await recheckSettled(page); // readable history still cannot prove this request's outcome while POST is down
  await expect(page.getByTestId("design-publish-notyet"), "OC11-A7 no list-derived terminal outcome").toHaveCount(0);
  await expect(page.getByTestId("design-publish-pending")).toBeVisible();
  await expect(page.getByTestId("design-publish")).toBeDisabled();
  await expect(page.getByTestId("design-note")).toHaveCount(0);
  expectAttempts(publishes, 1);

  state.cut = "none";
  const read = page.waitForResponse((r) => r.url().endsWith("/design/versions") && r.request().method() === "GET");
  const receipt = await recheckSettled(page);
  expect(receipt?.status()).toBe(200);
  const returned: VersionInfo = await receipt!.json();
  const sentVersion = JSON.parse(publishes[0].body!).expected_draft_version;
  expect(returned.source_version).toBe(sentVersion);
  await expect(page.getByTestId("design-note")).toContainText(fill(c.okPublished, { n: returned.version }));
  await expect(page.getByTestId("design-publish-pending")).toHaveCount(0);
  const history = await (await read).json();
  expect(history.items.filter((v: VersionInfo) => v.kind === "publish" && v.source_version === sentVersion),
    "R2 resend after no commit creates exactly one published row").toHaveLength(1);
  expectAttempts(publishes, 1);
  await page.reload();
  await editorTask(page, "profile");
  await expect(page.getByTestId("design-name")).toBeVisible();
  await expect(page.getByTestId("design-publish-pending")).toHaveCount(0);
});

test("OC11 old publish row of the same draft version never confirms a new lost publish (publish, rollback, publish again)", async ({ page }) => {
  const { state } = await open(page);
  await edit(page, "c");
  await editorPublish(page); // real publish of this draft version
  await expect(page.getByTestId("design-note")).toContainText(okPublished);
  await editorTask(page, "versions");
  await page.getByTestId("design-rollback").last().click(); // rollback: the newest row is now a rollback, the draft is unchanged
  await expect(page.getByTestId("design-note")).toContainText(/Rolled back/);
  await expect(page.getByTestId("design-publish")).toBeEnabled();

  state.cut = "before-commit"; // publish this same draft version again; the answer is lost and nothing committed
  await editorPublish(page);
  await expect(page.getByTestId("design-publish-pending")).toContainText(c.publishPending);
  await expect(page.getByTestId("design-publish-unconfirmed")).toBeVisible();
  await recheckSettled(page);
  await expect(page.getByTestId("design-publish-notyet"), "OC11-A4 the old publish row (source_version equal) cannot determine the outcome").toHaveCount(0);
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
  await editorPublish(page);
  await expect.poll(() => publishes.length, { message: "the publish request left the page" }).toBe(1);
  // READ/MEASURE only: inspect the real persisted request while the POST cannot complete.
  const armed = await page.evaluate((s) => JSON.parse(sessionStorage.getItem(`lc.design.publishPending.${s}`) ?? "null"), store);
  expect.soft(armed?.key, "OC11-A9 identity is durable while the original POST is held").toBe(publishes[0].key);
  try {
    await page.reload();
    await editorTask(page, "profile");
    await expect(page.getByTestId("design-name")).toBeVisible();
    await expect.soft(page.getByTestId("design-publish-pending"), "OC11-A9 pending armed before the request").toContainText(c.publishPending);
    // The editor is ready: sample the control now. Playwright timeout: 0 disables its deadline,
    // which would strand this soft assertion and hide the independent A10 POST-count failure.
    expect.soft(await page.getByTestId("design-publish").isDisabled(), "OC11-A9 Publish stays locked after reload").toBe(true);
    // Exercise the accidentally enabled control on the unfixed order so A10 has its OWN assertion witness.
    state.cut = "before-commit";
    if (await page.getByTestId("design-publish").isEnabled()) {
      const retryFailed = page.waitForEvent("requestfailed", (r) => r.url().endsWith("/design/publish"));
      await editorPublish(page);
      await retryFailed; // event boundary: the accidental POST has actually passed the route interceptor
    }
    expectAttempts(publishes, 1);
  } finally {
    state.release();
  }
});

test("OC11 stale versions list: confirmed publish, failed refresh, second publish aborted before commit never shows success", async ({ page }) => {
  const { state } = await open(page);
  await edit(page, "e"); // wait for the editor's initial draft/media/versions load before injecting the refresh failure
  await page.route("**/design/versions", (route) => route.abort("connectionreset")); // only subsequent versions reads fail
  await editorPublish(page); // real publish; its answer arrives, the list refresh after it fails
  await expect(page.getByTestId("design-note")).toContainText(okPublished);
  await edit(page, "next-draft-after-failed-list");
  state.cut = "before-commit"; // next draft; the confirmed older row still cannot prove this attempt
  await editorPublish(page);
  await expect(page.getByTestId("design-publish-pending")).toContainText(c.publishPending);
  await page.unroute("**/design/versions");
  await expect(page.getByTestId("design-publish-unconfirmed")).toBeVisible();
  await recheckSettled(page);
  await expect(page.getByTestId("design-publish-notyet"), "OC11-A11 an unrelated confirmed row cannot determine the outcome").toHaveCount(0);
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
  await editorTask(other, "profile");
  await expect(other.getByTestId("design-name")).toBeVisible();
  await editorPublish(other);
  await expect(other.getByTestId("design-note")).toContainText(okPublished);
  state.cut = "before-commit"; // A's stale request never reaches Go; B's publish is the only committed one.
  await editorPublish(page);
  await expect(page.getByTestId("design-publish-pending")).toBeVisible();
  await expect(page.getByTestId("design-publish-unconfirmed")).toBeVisible();
  await recheckSettled(page);
  await expect(page.getByTestId("design-note"), "OC11-P1a another request must never be reported as our success").toHaveCount(0);
  await expect(page.getByTestId("design-publish-pending")).toBeVisible();
  expectAttempts(publishes, 1);
});

test("OC11 stale draft: restoring transport keeps the original version and ends pending with conflict", async ({ page, context }) => {
  const { publishes, state } = await open(page);
  state.cut = "before-commit";
  await edit(page, "stale-request");
  await editorPublish(page);
  await expect(page.getByTestId("design-publish-pending")).toBeVisible();
  await expect(page.getByTestId("design-publish-unconfirmed")).toBeVisible();
  await recheckSettled(page); // retain the original request before another tab changes the draft

  const other = await context.newPage();
  await other.goto(`/en/design?store=${store}`);
  await editorTask(other, "profile");
  await expect(other.getByTestId("design-name")).toBeVisible();
  await edit(other, "newer-draft");
  await other.getByTestId("design-save").click();
  await expect(other.getByTestId("design-note")).toContainText(c.okSaved);
  state.cut = "none";
  const receipt = await recheckSettled(page);
  expect(receipt?.status(), "R4 stale original version is refused, never replaced by the new draft").toBe(409);
  expect(await receipt!.json()).toMatchObject({ code: "conflict" });
  expect(receipt!.request().headers()["idempotency-key"]).toBe(publishes[0].key);
  expect(receipt!.request().postData()).toBe(publishes[0].body);
  await expect(page.getByTestId("design-publish-pending")).toHaveCount(0);
  await expect(page.getByTestId("design-note")).toContainText(c.publishChanged);
  await expect(page.getByTestId("design-note")).not.toContainText(okPublished);
  expectAttempts(publishes, 1);
  await page.reload();
  await editorTask(page, "profile");
  await expect(page.getByTestId("design-name")).toBeVisible();
  await expect(page.getByTestId("design-publish-pending")).toHaveCount(0);
});

test("OC11 P1b pending blocks local rollback, external rollback resolves the committed request as superseded", async ({ page, context }) => {
  const { publishes, state } = await open(page);
  // Establish our own rollback target; do not depend on an earlier test's success.
  await edit(page, "rollback-base");
  await editorPublish(page);
  await expect(page.getByTestId("design-note")).toContainText(okPublished);
  await edit(page, "rollback-pending");
  state.cut = "after-commit";
  await editorPublish(page);
  await expect(page.getByTestId("design-publish-pending")).toBeVisible();
  await editorTask(page, "versions");
  expect.soft(await page.getByTestId("design-rollback").last().isDisabled(), "OC11-P1b local rollback cannot race an unresolved publish").toBe(true);

  const other = await context.newPage();
  other.on("dialog", (d) => void d.accept());
  await other.goto(`/en/design?store=${store}`);
  await editorTask(other, "versions");
  await other.getByTestId("design-rollback").last().click();
  await expect(other.getByTestId("design-note")).toContainText(/Rolled back/);
  await expect(page.getByTestId("design-publish-unconfirmed")).toBeVisible();
  await expect(page.getByTestId("design-publish-recheck")).toBeEnabled();
  state.cut = "none"; // only delivery of our stored POST receipt may resolve, even after external rollback
  await recheckSettled(page);
  await expect.soft(page.getByTestId("design-publish-pending"), "OC11-P1b the original committed receipt resolves despite a newer rollback").toHaveCount(0);
  await expect(page.getByTestId("design-publish-superseded")).toBeVisible(); // allow the post-receipt display refresh to settle
  // recheckSettled already observed completion; take independent snapshots so every oracle runs on RED.
  expect.soft(await page.getByTestId("design-publish-superseded").isVisible(), "OC11-P1b explain that a later version replaced this publication").toBe(true);
  expect.soft(await page.getByTestId("design-publish").isEnabled(), "OC11-P1b terminal resolution unlocks Publish").toBe(true);
  await page.reload();
  await editorTask(page, "profile");
  await expect(page.getByTestId("design-name")).toBeVisible();
  expect.soft(await page.getByTestId("design-publish-pending").count(), "OC11-P1b terminal resolution survives refresh").toBe(0);
  await expect(page.getByTestId("design-publish")).toBeEnabled();
  expectAttempts(publishes, 2);
});

test("OC11 P1b a hung recovery POST has a bounded timeout and releases its busy control", async ({ page }) => {
  const { publishes, state } = await open(page);
  await edit(page, "bounded-read");
  state.cut = "before-commit";
  await editorPublish(page);
  await expect(page.getByTestId("design-publish-pending")).toBeVisible();
  await expect(page.getByTestId("design-publish-unconfirmed")).toBeVisible();
  await expect(page.getByTestId("design-publish-recheck")).toBeEnabled(); // exhaust/settle automatic recovery before holding the merchant's POST
  let release = () => {};
  const held = page.waitForRequest((r) => r.url().endsWith("/design/publish") && r.method() === "POST");
  await page.route("**/design/publish", async (route) => {
    await new Promise<void>((done) => { release = done; });
    await route.abort("connectionreset").catch(() => {});
  });
  try {
    const started = Date.now();
    await page.getByTestId("design-publish-recheck").click();
    const request = await held; // the recovery POST really reached the suspended transport
    expect(request.headers()["idempotency-key"]).toBe(publishes[0].key);
    expect(request.postData()).toBe(publishes[0].body);
    await expect(page.getByTestId("design-publish-recheck")).toBeDisabled();
    await expect(page.getByTestId("design-publish-recheck"), "OC11-P1b a hung POST releases Recheck within five seconds plus scheduling margin").toBeEnabled({ timeout: 7000 });
    expect(Date.now() - started, "the held POST really reached the five-second timeout, not an immediate failed GET").toBeGreaterThanOrEqual(5000);
    await expect(page.getByTestId("design-publish-unconfirmed")).toBeVisible();
    await expect(page.getByTestId("design-note")).toHaveCount(0);
    await expect(page.getByTestId("design-publish-pending")).toBeVisible();
    await expect(page.getByTestId("design-publish")).toBeDisabled();
  } finally {
    release();
  }
});

for (const locale of ["zh-TW", "zh-CN", "en"] as const) {
  test(`OC11 pending and same-key recovery render in ${locale} and survive refresh`, async ({ page }) => {
    const { publishes, state } = await open(page);
    await page.goto(`/${locale}/design?store=${store}`);
    await edit(page, `locale-${locale}`);
    state.cut = "before-commit";
    await editorPublish(page);
    await expect(page.getByTestId("design-publish-pending")).toContainText(designCopy[locale].publishPending);
    await expect(page.getByTestId("design-publish-unconfirmed")).toBeVisible();
    await expect(page.getByTestId("design-publish-recheck")).toBeEnabled();
    await page.reload();
    await expect(page.getByTestId("design-publish-pending")).toContainText(designCopy[locale].publishPending);
    await recheckSettled(page);
    await expect(page.getByTestId("design-publish-notyet")).toHaveCount(0);
    await expect(page.getByTestId("design-publish-pending")).toContainText(designCopy[locale].publishPending);
    await expect(page.getByTestId("design-publish")).toBeDisabled();
    await expect(page.getByTestId("design-note")).toHaveCount(0);
    expectAttempts(publishes, 1);
    state.cut = "none";
    const receipt = await recheckSettled(page);
    expect(receipt?.status()).toBe(200);
    const returned: VersionInfo = await receipt!.json();
    await expect(page.getByTestId("design-note")).toContainText(fill(designCopy[locale].okPublished, { n: returned.version }));
    await expect(page.getByTestId("design-publish-pending")).toHaveCount(0);
    expectAttempts(publishes, 1);
    await page.reload();
    await editorTask(page, "profile");
    await expect(page.getByTestId("design-name")).toBeVisible();
    await expect(page.getByTestId("design-publish-pending")).toHaveCount(0);
  });
}
