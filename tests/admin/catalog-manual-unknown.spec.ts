// Purpose: Independently exercise four manual collection UNKNOWN escape paths on the real catalog browser gate.
// Depends on: Playwright, catalog-v2-copy, signed catalog-core Go/PG/browser fixture and runner-only shopper reads.
// Used by: TestBrowserCatalogCore's existing generated Playwright spec registration (CC12 mode only).
// Invariants: real enabled clicks only; a genuine committed POST loses only its ACK; original key/body survive recovery;
// same-origin authenticated reads expose only matching synthetic IDs/status, never cookies or unrelated customer data.
import { expect, test, type Page } from "@playwright/test";
import { catalogCopy } from "../../apps/admin/lib/catalog-v2-copy";

const required = (name: string) => {
  const value = process.env[name];
  if (!value) throw new Error(`${name} is required`);
  return value;
};
const origin = required("LC_BROWSER_PUBLIC_ORIGIN");
const store = required("LC_BROWSER_STORE");
const tag = required("LC_BROWSER_TAG");
const control = required("LC_BROWSER_CONTROL");
const controlKey = required("LC_BROWSER_CONTROL_KEY");
const endpoint = `/api/stores/${store}/collections`;
const copy = catalogCopy.en;

test.use({ baseURL: origin, trace: "retain-on-failure", screenshot: "only-on-failure" });
// Deliberately independent, not serial: one product RED must not skip the other three paths.
test.describe.configure({ mode: "default" });

async function signedLogin(page: Page) {
  await page.goto(new URL("/en/", origin).toString());
  await page.getByRole("button", { name: "Sign in with identity service" }).click();
  await expect(page.getByTestId("nav-orders")).toBeAttached();
  await page.goto(`/en/collections?store=${store}`);
  await expect(page.getByTestId("collections-list")).toBeVisible();
}

// READ/MEASURE only: the browser retains the authenticated session; no API writes or cookie export.
async function matching(page: Page, title: string) {
  return page.evaluate(async ({ url, title }) => {
    const response = await fetch(url, { method: "GET", credentials: "same-origin", cache: "no-store" });
    if (response.status !== 200) return { status: response.status, ids: [] as string[] };
    const body = await response.json();
    return { status: response.status, ids: body.items.filter((item: { title: string }) => item.title === title).map((item: { id: string }) => item.id) };
  }, { url: `${endpoint}?limit=100`, title });
}

// Same existing runner-owned shopper HTTP boundary as catalog-core; retain only this synthetic title's IDs.
async function shopperMatching(title: string) {
  const response = await fetch(`${control}/?path=${encodeURIComponent("/v1/buyer/catalog/v2/collections")}`, { headers: { "X-Gate-Key": controlKey } });
  expect(response.status, "runner-only shopper read").toBe(200);
  const receipt = await response.json();
  const body = receipt.body ? JSON.parse(receipt.body) : null;
  return { status: receipt.status as number, ids: (body?.collections ?? []).filter((item: { title: string }) => item.title === title).map((item: { id: string }) => item.id) as string[] };
}

if (process.env.PRODUCT_EDITOR_ACCEPTANCE !== "1") for (const entry of ["new", "row", "dismiss", "reload"] as const) {
  test(`manual collection UNKNOWN fences ${entry} and preserves the original creation`, async ({ page }, testInfo) => {
    const title = `${tag} manual unknown ${entry}`;
    expect(title, "ASCII synthetic title exercises server-generated suffixes, not explicit-slug conflicts").toMatch(/^[A-Za-z0-9 ]{1,80}$/);
    const commands: Array<{ key: string | null; body: string | null }> = [];
    const receipts: Array<{ status: number; id: string | null }> = [];
    const reads: Array<{ phase: string; status: number; ids: string[] }> = [];
    const clicks: Array<{ control: string; actual: "clicked" | "disabled" | "absent" }> = [];
    const extraWrites: Array<{ method: string; path: string }> = [];
    const retry = page.getByRole("button", { name: copy.edit.retry, exact: true });
    const canRetry = async () => await retry.isVisible() && await retry.isEnabled();
    let pendingPosts = 0;
    let settledPosts = 0;
    let committedID = "";
    let rowID = "";
    const isCreation = (response: { url(): string; request(): { method(): string } }) => new URL(response.url()).pathname === endpoint && response.request().method() === "POST";
    const clickEnabled = async (id: string) => {
      const button = page.getByTestId(id);
      await expect(button).toBeVisible();
      const enabled = await button.isEnabled();
      if (enabled) await button.click();
      clicks.push({ control: id, actual: enabled ? "clicked" : "disabled" });
      return enabled;
    };
    const openManual = async () => {
      await page.getByTestId("collection-manage-new").click();
      await expect(page.getByTestId("collection-title")).toHaveValue("");
      await expect(page.getByTestId("collection-slug")).toHaveValue("");
    };
    const observe = async (phase: string) => {
      const admin = await matching(page, title);
      const shopper = await shopperMatching(title);
      reads.push({ phase: `${phase}:admin`, ...admin }, { phase: `${phase}:shopper`, ...shopper });
      expect.soft(admin.status, `${phase}: authenticated same-origin collection read`).toBe(200);
      expect.soft(admin.ids, `${phase}: one actual object, including suffixed duplicate titles`).toEqual([committedID]);
      expect.soft(shopper.status, `${phase}: genuine shopper catalog read`).toBe(200);
      expect.soft(shopper.ids, `${phase}: one shopper-visible collection, including suffix variants`).toEqual([committedID]);
    };
    const settleProbe = async () => {
      // READ/MEASURE: observe the next rendered state after the click, not an old alert from before React's event commit.
      await page.evaluate(() => new Promise<void>(resolve => requestAnimationFrame(() => resolve())));
      await expect.poll(async () => {
        const manualBusy = await page.locator("#collection-save-reason").count() > 0;
        const terminal = await page.getByTestId("collection-message").isVisible()
          || await page.getByTestId("collection-members").isVisible()
          || await page.locator("#collections-quick-create").getByRole("alert").isVisible();
        return !manualBusy && pendingPosts === 0 && settledPosts === commands.length && terminal;
      }, { message: "clicked writer is guarded or all genuinely dispatched POSTs and rendered results have settled" }).toBe(true);
    };
    // Probe only an actually accessible writer. A pending guard may safely decline the click without sending a POST.
    const probeManual = async () => {
      const input = page.getByTestId("collection-title");
      if (await input.isVisible() && await input.isEnabled()) await input.fill(title);
      const save = page.getByTestId("collection-save");
      if (await save.isVisible() && await save.isEnabled()) {
        await save.click();
        clicks.push({ control: "collection-save", actual: "clicked" });
        await settleProbe();
      } else clicks.push({ control: "collection-save", actual: await save.isVisible() ? "disabled" : "absent" });
    };
    const probeQuick = async () => {
      const quick = page.locator("#collections-quick-create");
      if (!await quick.isVisible()) return;
      const input = quick.getByRole("textbox", { name: "Name", exact: true });
      const create = page.getByTestId("collection-new");
      if (await input.isEnabled() && await create.isEnabled()) {
        await input.fill(title);
        await create.click();
        clicks.push({ control: "collection-new:quick-submit", actual: "clicked" });
        await settleProbe();
      } else clicks.push({ control: "collection-new:quick-submit", actual: "disabled" });
    };

    try {
      await signedLogin(page);
      if (entry === "row") {
        // Prepare the required existing row through genuine merchant clicks, before installing the ACK-loss fault.
        await openManual();
        await page.getByTestId("collection-title").fill(`${tag} manual row target`);
        const [created] = await Promise.all([page.waitForResponse(isCreation), page.getByTestId("collection-save").click()]);
        expect(created.status()).toBe(200);
        rowID = (await created.json()).id;
        expect(rowID).toMatch(/^[0-9a-f-]{36}$/);
        await expect(page.getByTestId(`collection-item-${rowID}`)).toBeVisible();
        await expect(page.getByTestId("collection-members")).toBeVisible();
      }
      await openManual();
      await page.getByTestId("collection-title").fill(title);
      page.on("request", request => {
        const url = new URL(request.url());
        if (url.origin === origin && url.pathname.startsWith(`/api/stores/${store}/`) && !["GET", "HEAD"].includes(request.method()) && url.pathname !== endpoint)
          extraWrites.push({ method: request.method(), path: url.pathname });
      });
      await page.route(`**${endpoint}`, async route => {
        if (route.request().method() !== "POST") return route.continue();
        commands.push({ key: route.request().headers()["idempotency-key"] ?? null, body: route.request().postData() });
        const first = commands.length === 1;
        pendingPosts++;
        try {
          const response = await route.fetch();
          const receipt = await response.json();
          receipts.push({ status: response.status(), id: typeof receipt.id === "string" ? receipt.id : null });
          if (first) {
            expect(response.status(), "genuine manual POST committed before losing only its ACK").toBe(200);
            expect(receipt.id).toMatch(/^[0-9a-f-]{36}$/);
            committedID = receipt.id;
            await route.abort("connectionreset");
          } else await route.fulfill({ response }); // Forward the genuine server receipt; never synthesize success.
          settledPosts++;
        } finally { pendingPosts--; }
      });
      await page.getByTestId("collection-save").click();
      clicks.push({ control: "collection-save:initial-manual", actual: "clicked" });
      await expect(page.getByTestId("collection-message")).toContainText(copy.collections.uncertain);
      expect(commands).toHaveLength(1);
      expect(commands[0].key).toMatch(/^[0-9a-f-]{36}$/);
      expect(JSON.parse(commands[0].body!)).toMatchObject({ title, description: "", sort_mode: "manual", status: "active" });
      expect(JSON.parse(commands[0].body!)).not.toHaveProperty("slug");
      await expect(retry).toBeVisible();
      await observe("committed-ACK-lost");

      if (entry === "new") {
        await clickEnabled("collection-new");
        await expect(page.locator("#collections-quick-create").or(retry)).toBeVisible();
        await probeQuick();
      } else if (entry === "row") {
        await clickEnabled(`collection-item-${rowID}`);
        await expect(page.getByTestId("collection-editor").or(retry).first()).toBeVisible();
        if (!await canRetry() && await clickEnabled("collection-manage-new")) {
          await expect(page.getByTestId("collection-title")).toHaveValue("");
          await probeManual();
        }
      } else if (entry === "dismiss") {
        const dismiss = page.getByRole("button", { name: copy.collections.dismiss, exact: true });
        if (await dismiss.isVisible() && await dismiss.isEnabled()) {
          await dismiss.click();
          clicks.push({ control: "Dismiss", actual: "clicked" });
          await probeManual();
        } else clicks.push({ control: "Dismiss", actual: await dismiss.isVisible() ? "disabled" : "absent" });
      } else {
        await page.reload();
        await expect(page.getByTestId("collections-list")).toBeVisible();
        await probeQuick();
        if (commands.length === 1 && !await canRetry() && await clickEnabled("collection-manage-new")) {
          await expect(page.getByTestId("collection-title")).toBeVisible();
          await probeManual();
        }
      }

      // Preserve causal evidence before a missing original Retry fails: the real second POST and GET can prove two objects.
      await settleProbe();
      await observe(`after-${entry}`);
      expect.soft(commands, "UNKNOWN entry must not send an additional creation before explicit original Retry").toHaveLength(1);
      expect.soft(new Set(commands.map(command => command.key)).size, "UNKNOWN must not replace the unresolved idempotency key").toBe(1);
      for (const command of commands) expect.soft(command, "UNKNOWN must retain original request bytes as well as its key").toEqual(commands[0]);
      expect.soft(extraWrites, "navigation/recovery must not cause any other store mutation").toEqual([]);
      for (const receipt of receipts) expect.soft(receipt.status, "all observed creation receipts come from the genuine server").toBe(200);

      if (entry === "reload" && !await canRetry()) {
        // The existing recovery contract permits explicit fail-closed reconciliation; it does not promise automatic command recovery.
        const recovery = page.getByRole("alert");
        expect.soft(await recovery.count(), "reload must visibly retain the unconfirmed/reconciliation state").toBeGreaterThan(0);
        expect.soft(await recovery.allTextContents(), "reload must explain UNKNOWN/recovery rather than silently reset it").toEqual(expect.arrayContaining([expect.stringMatching(/not confirmed|unconfirmed|reconcil|recovery/i)]));
        expect(commands, "fail-closed reload sends no extra POST and creates no new key").toHaveLength(1);
        return;
      }

      expect(await retry.isVisible(), "the original same-command Retry was lost; do not replace it with a fresh writer").toBe(true);
      await expect(retry).toBeEnabled();
      const [retried] = await Promise.all([page.waitForResponse(isCreation), retry.click()]);
      clicks.push({ control: "original Retry", actual: "clicked" });
      expect(retried.status(), "explicit original-command retry returns the real stored receipt").toBe(200);
      expect(commands, "exactly the original submission plus its explicit retry").toHaveLength(2);
      for (const command of commands) expect(command, "every creation attempt retains original key and exact body bytes").toEqual(commands[0]);
      expect(receipts.map(receipt => receipt.id), "both server receipts identify the original committed object").toEqual([committedID, committedID]);
      await expect(page.getByTestId("collection-title")).toHaveValue(title);
      await observe("original-retry-resolved");
      await page.reload();
      await expect(page.getByTestId(`collection-item-${committedID}`)).toBeVisible();
      await observe("resolved-reload");
      expect(extraWrites, "retry/reload never mutates another catalog resource").toEqual([]);
    } finally {
      // Synthetic command/receipt evidence only. No cookies, auth headers, control key or unrelated response bodies.
      await testInfo.attach(`manual-unknown-${entry}`, { contentType: "application/json", body: Buffer.from(JSON.stringify({ entry, commands, receipts, reads, clicks, extraWrites }, null, 2)) });
    }
  });
}

// New recovery cases are separate from the frozen four entry probes above. In particular, a
// disabled fresh writer after reload is insufficient: the original command must be recoverable.
if (process.env.PRODUCT_EDITOR_ACCEPTANCE !== "1") {
  const journalKey = `catalog-collection-create:${store}`;
  type Captured = { key: string | null; body: string | null };
  const isCreate = (url: string, method: string) => new URL(url).pathname === endpoint && method === "POST";

  async function committedUnknown(page: Page, title: string, description: string, second: "real" | "forbidden" = "real") {
    const commands: Captured[] = [];
    const receipts: Array<{ status: number; id: string | null }> = [];
    const otherWrites: string[] = [];
    let originalID = "";
    page.on("request", request => {
      const path = new URL(request.url()).pathname;
      if (new URL(request.url()).origin === origin && path.startsWith(`/api/stores/${store}/`) &&
        !["GET", "HEAD"].includes(request.method()) && !isCreate(request.url(), request.method()))
        otherWrites.push(`${request.method()} ${path}`);
    });
    await page.route(`**${endpoint}`, async route => {
      if (route.request().method() !== "POST") return route.continue();
      commands.push({ key: route.request().headers()["idempotency-key"] ?? null, body: route.request().postData() });
      if (commands.length > 1 && second === "forbidden") {
        await route.fulfill({ status: 403, contentType: "application/json", body: JSON.stringify({ code: "forbidden", message: "Forbidden" }) });
        receipts.push({ status: 403, id: null });
        return;
      }
      const response = await route.fetch();
      const body = await response.json();
      receipts.push({ status: response.status(), id: typeof body.id === "string" ? body.id : null });
      if (commands.length === 1) {
        expect(response.status(), "first POST must really commit before ACK loss").toBe(200);
        expect(body.id).toMatch(/^[0-9a-f-]{36}$/);
        originalID = body.id;
        await route.abort("connectionreset");
      } else await route.fulfill({ response });
    });
    await signedLogin(page);
    await page.getByTestId("collection-manage-new").click();
    await expect(page.getByTestId("collection-title")).toBeVisible();
    await page.getByTestId("collection-title").fill(title);
    await page.getByTestId("collection-description").fill(description);
    await page.getByTestId("collection-save").click();
    await expect(page.getByTestId("collection-message")).toContainText(copy.collections.uncertain);
    expect(commands, "one genuinely committed creation before recovery probes").toHaveLength(1);
    expect(commands[0].key).toMatch(/^[0-9a-f-]{36}$/);
    expect(JSON.parse(commands[0].body!)).toMatchObject({ title, description, sort_mode: "manual", status: "active" });
    expect(otherWrites).toEqual([]);
    const originalJournal = await page.evaluate(key => sessionStorage.getItem(key), journalKey);
    expect(originalJournal, "durable record exists after lost ACK").not.toBeNull();
    const record = JSON.parse(originalJournal!);
    expect(record).toMatchObject({ version: 1, store, command: { key: commands[0].key, method: "POST", resource: "collections", body: commands[0].body } });
    expect(record.boundary).toMatch(/^[0-9a-f]{64}$/);
    return { commands, receipts, otherWrites, originalID, originalJournal: originalJournal! };
  }

  test("valid manual journal reload restores original Retry and real Go/PG receipt", async ({ page }, testInfo) => {
    const title = `${tag} durable manual reload`;
    const description = `${tag} durable description`;
    const evidence = await committedUnknown(page, title, description);
    try {
      await page.reload();
      await expect(page.getByTestId("collections-list")).toBeVisible();
      await expect(page.getByTestId("collection-title"), "valid manual payload must reappear in its editor").toHaveValue(title);
      await expect(page.getByTestId("collection-description")).toHaveValue(description);
      const retry = page.getByRole("button", { name: copy.edit.retry, exact: true });
      await expect(retry, "fail-closed alert alone does not satisfy durable recovery").toBeVisible();
      await expect(retry).toBeEnabled();
      expect(evidence.commands).toHaveLength(1);
      const [response] = await Promise.all([page.waitForResponse(r => isCreate(r.url(), r.request().method())), retry.click()]);
      expect(response.status(), "explicit retry reaches the real BFF/Go/PG path").toBe(200);
      expect(evidence.commands).toHaveLength(2);
      expect(evidence.commands[1], "original key and body bytes survive reload").toEqual(evidence.commands[0]);
      expect(evidence.receipts).toEqual([{ status: 200, id: evidence.originalID }, { status: 200, id: evidence.originalID }]);
      expect(await matching(page, title)).toEqual({ status: 200, ids: [evidence.originalID] });
      expect(await shopperMatching(title)).toEqual({ status: 200, ids: [evidence.originalID] });
      expect(evidence.otherWrites).toEqual([]);
      await expect.poll(() => page.evaluate(key => sessionStorage.getItem(key), journalKey)).toBeNull();
    } finally {
      await testInfo.attach("durable-manual-reload", { contentType: "application/json", body: Buffer.from(JSON.stringify({ commands: evidence.commands, receipts: evidence.receipts, originalID: evidence.originalID, otherWrites: evidence.otherWrites })) });
    }
  });

  for (const bad of ["malformed-body", "foreign-boundary"] as const) {
    test(`${bad} journal reload never renders private draft or dispatches a writer`, async ({ page }) => {
      const title = `${tag} bad journal ${bad.replace("-", " ")}`;
      const description = `${tag} private draft ${bad}`;
      const evidence = await committedUnknown(page, title, description);
      const altered = JSON.parse(evidence.originalJournal);
      if (bad === "malformed-body") altered.command.body = JSON.stringify({ title, description, sort_mode: "automatic", status: "active", rules: { match: "all", conditions: [] } });
      else altered.boundary = "0".repeat(64) === altered.boundary ? "1".repeat(64) : "0".repeat(64);
      const alteredBytes = JSON.stringify(altered);
      await page.evaluate(({ key, value }) => sessionStorage.setItem(key, value), { key: journalKey, value: alteredBytes });
      await page.reload();
      await expect(page.getByTestId("collections-list")).toBeVisible();
      await expect(page.getByTestId("collections-page").getByRole("alert")).toContainText(copy.collections.uncertain);
      expect(await page.locator("body").innerText(), "invalid or foreign journal must not expose its draft description").not.toContain(description);
      await expect(page.getByTestId("collection-manage-new")).toBeDisabled();
      await expect(page.getByTestId("collection-new")).toBeDisabled();
      expect(evidence.commands, "reload must not auto-replay or mint a fresh key").toHaveLength(1);
      expect(await page.evaluate(key => sessionStorage.getItem(key), journalKey), "bad record remains for reconciliation").toBe(alteredBytes);
      expect(await matching(page, title)).toEqual({ status: 200, ids: [evidence.originalID] });
      expect(evidence.otherWrites).toEqual([]);
    });
  }

  test("session storage write denial before send leaves zero creation POSTs", async ({ page }) => {
    const title = `${tag} storage denial`;
    const creates: Captured[] = [];
    await signedLogin(page);
    await page.route(`**${endpoint}`, async route => {
      if (route.request().method() === "POST") {
        creates.push({ key: route.request().headers()["idempotency-key"] ?? null, body: route.request().postData() });
        await route.abort("blockedbyclient");
        return;
      }
      await route.continue();
    });
    await page.getByTestId("collection-manage-new").click();
    await page.getByTestId("collection-title").fill(title);
    await page.evaluate(key => {
      const original = Storage.prototype.setItem;
      Storage.prototype.setItem = function (name, value) {
        if (this === sessionStorage && name === key) throw new DOMException("storage denied", "QuotaExceededError");
        return original.call(this, name, value);
      };
    }, journalKey);
    await page.getByTestId("collection-save").click();
    await expect(page.getByTestId("collection-message")).toContainText(copy.collections.uncertain);
    expect(creates, "failure to persist before send must never create remotely").toEqual([]);
    expect(await page.evaluate(key => sessionStorage.getItem(key), journalKey)).toBeNull();
    await expect(page.getByTestId("collection-save")).toBeDisabled();
    expect(await matching(page, title)).toEqual({ status: 200, ids: [] });
  });

  test("retry auth denial retains UNKNOWN journal and prevents a fresh creation", async ({ page }) => {
    const title = `${tag} retry forbidden`;
    const evidence = await committedUnknown(page, title, `${tag} forbidden draft`, "forbidden");
    const retry = page.getByRole("button", { name: copy.edit.retry, exact: true });
    await expect(retry).toBeEnabled();
    await retry.click();
    await expect.poll(() => evidence.commands.length).toBe(2);
    await expect(page.getByTestId("collection-message")).toContainText(copy.collections.uncertain);
    expect(evidence.commands[1]).toEqual(evidence.commands[0]);
    expect(evidence.receipts).toEqual([{ status: 200, id: evidence.originalID }, { status: 403, id: null }]);
    expect(await page.evaluate(key => sessionStorage.getItem(key), journalKey)).toBe(evidence.originalJournal);
    await expect(page.getByTestId("collection-manage-new")).toBeDisabled();
    await expect(page.getByTestId("collection-new")).toBeDisabled();
    await page.reload();
    await expect(page.getByTestId("collections-list")).toBeVisible();
    expect(evidence.commands, "reload must not issue a new command after 403").toHaveLength(2);
    expect(await matching(page, title)).toEqual({ status: 200, ids: [evidence.originalID] });
    expect(evidence.otherWrites).toEqual([]);
  });

  test("rapid manual doubleclick cannot mint a second creation", async ({ page }) => {
    const title = `${tag} rapid doubleclick`;
    const commands: Captured[] = [];
    const receipts: string[] = [];
    await signedLogin(page);
    await page.route(`**${endpoint}`, async route => {
      if (route.request().method() !== "POST") return route.continue();
      commands.push({ key: route.request().headers()["idempotency-key"] ?? null, body: route.request().postData() });
      const response = await route.fetch();
      const body = await response.json();
      expect(response.status()).toBe(200);
      receipts.push(body.id);
      if (commands.length === 1) await route.abort("connectionreset");
      else await route.fulfill({ response });
    });
    await page.getByTestId("collection-manage-new").click();
    await page.getByTestId("collection-title").fill(title);
    // Use the actionable user control; Playwright emits a real browser doubleclick without force.
    const save = page.getByTestId("collection-save");
    await expect(save).toBeEnabled();
    await save.dblclick();
    await expect(page.getByTestId("collection-message")).toBeVisible();
    expect(commands.length, "both click events may produce at most one duplicate replay").toBeGreaterThanOrEqual(1);
    expect(commands.length).toBeLessThanOrEqual(2);
    expect(commands[0].key).toMatch(/^[0-9a-f-]{36}$/);
    expect(JSON.parse(commands[0].body!)).toMatchObject({ title, sort_mode: "manual" });
    for (const command of commands) expect(command, "duplicate click must retain one command").toEqual(commands[0]);
    expect(receipts).toHaveLength(commands.length);
    expect(new Set(receipts).size).toBe(1);
    expect(await matching(page, title)).toEqual({ status: 200, ids: [receipts[0]] });
    expect(await shopperMatching(title)).toEqual({ status: 200, ids: [receipts[0]] });
  });

  test("late real retry receipt after session change cannot clear journal or expose prior draft", async ({ page }) => {
    const title = `${tag} late session result`;
    const description = `${tag} prior session private draft`;
    const evidence = await committedUnknown(page, title, description);
    let release!: () => void;
    let markStarted!: () => void;
    const permit = new Promise<void>(resolve => { release = resolve; });
    const started = new Promise<void>(resolve => { markStarted = resolve; });
    // The newer route holds only the retry ACK; the first request has already really committed.
    await page.route(`**${endpoint}`, async route => {
      if (route.request().method() !== "POST") return route.continue();
      evidence.commands.push({ key: route.request().headers()["idempotency-key"] ?? null, body: route.request().postData() });
      const response = await route.fetch();
      const body = await response.json();
      evidence.receipts.push({ status: response.status(), id: body.id });
      markStarted();
      await permit;
      await route.fulfill({ response });
    });
    const retry = page.getByRole("button", { name: copy.edit.retry, exact: true });
    await expect(retry).toBeEnabled();
    const retryResponse = page.waitForResponse(r => isCreate(r.url(), r.request().method()) && r.status() === 200);
    try {
      await retry.click();
      await started;
      expect(evidence.receipts[1], "server replay must be a real receipt before delaying its ACK").toEqual({ status: 200, id: evidence.originalID });
      expect(evidence.commands[1]).toEqual(evidence.commands[0]);
      const changed = await page.evaluate(() => {
        const original = document.cookie.split(";").map(part => part.trim()).find(part => part.startsWith("__Host-commerce_csrf="));
        const replacement = original === `__Host-commerce_csrf=${"A".repeat(43)}` ? "B".repeat(43) : "A".repeat(43);
        document.cookie = `__Host-commerce_csrf=${replacement}; Path=/; Secure; SameSite=Lax`;
        const current = document.cookie.split(";").map(part => part.trim()).find(part => part.startsWith("__Host-commerce_csrf="));
        return !!original && current === `__Host-commerce_csrf=${replacement}` && current !== original;
      });
      expect(changed, "fixture browser must allow an actual CSRF session-boundary change").toBe(true);
    } finally { release(); }
    await retryResponse;
    await expect(page.getByTestId("collections-page").getByRole("alert")).toContainText(copy.collections.uncertain);
    await expect.poll(() => page.evaluate(key => sessionStorage.getItem(key), journalKey)).toBe(evidence.originalJournal);
    await expect(page.getByTestId("collection-manage-new")).toBeDisabled();
    expect(await page.locator("body").innerText()).not.toContain(description);
    expect(await page.getByTestId("collection-description").count()).toBe(0);
    expect(evidence.commands).toHaveLength(2);
    expect(evidence.otherWrites).toEqual([]);
  });
}
