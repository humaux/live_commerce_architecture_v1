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
