// Purpose: independent W5-U1 real-click customer/history import acceptance and failure/privacy oracles from migration-import-v1.
// Depends on: Playwright, localized labels only, genuine signed HTTPS Go/PG fixture and LC_MIUI_* controls.
// Used by: TestBrowserMigrationImport / --browser-migration-import; no page.route/DOM injection or API writes as UI evidence.
// Invariants: I01/I02/I05/I06/I09/I11/I18; all names/cells are synthetic and never dumped to console/assertion output.
import { expect, test, type Browser, type Locator, type Page } from "@playwright/test";
import { createHash } from "node:crypto";
import { readFile, writeFile } from "node:fs/promises";
import path from "node:path";
import { shellCopy } from "../../apps/admin/src/shell-copy";
import { importCopy } from "../../apps/admin/lib/import-copy";
import { importViewCopy } from "../../apps/admin/lib/import-view-copy";
import { importHistoryCopy } from "../../apps/admin/lib/import-history-copy";

const required = (name: string) => { const value = process.env[name]; if (!value) throw new Error(`${name} required`); return value; };
const origin = required("LC_BROWSER_PUBLIC_ORIGIN"), evidence = required("LC_BROWSER_EVIDENCE"), store = required("LC_MIUI_STORE");
const control = required("LC_MIUI_CONTROL_URL"), key = required("LC_MIUI_CONTROL_KEY");
type Locale = "en" | "zh-TW";
type RequestMeta = { SHA: string; Mapping: string; Expected: string; Status: number; Path: string; Bytes: number; HasKey: boolean; ContentType: string };
type State = { counts: Record<string, number>; owners: Record<string, string>; batches: { SHA: string; Kind: string; ID: string; Created: number; Updated: number; Failed: number }[]; requests: RequestMeta[] };
type FileCase = { buffer: Buffer; mapping: Record<string, string>; markers: string[]; prefix: string };
type Ledger = { page: string; control: string; operation: string; expected: string; actual: string; result: "pass" | "fail" };
const ledger: Ledger[] = [];
const sha = (buffer: Buffer) => createHash("sha256").update(buffer).digest("hex");
const wizard = (page: Page) => page.getByTestId("import-wizard");
test.describe.configure({ mode: "serial" });
test.afterEach(async () => { await writeFile(path.join(evidence, "miui-click-ledger.json"), JSON.stringify(ledger, null, 2)); });
async function step(page: Page, name: string, expected: string, action: () => Promise<void>, operation = "click") {
  const row: Ledger = { page: new URL(page.url()).pathname, control: name, operation, expected, actual: "", result: "fail" }; ledger.push(row);
  try { await action(); row.result = "pass"; row.actual = expected; } catch (error) { row.actual = error instanceof Error ? error.message.slice(0, 200) : "assertion failed"; throw error; }
}
async function ctl(route: string, mode = "") {
  // Only fixture setup/fault arming/auth fences. Import writes under acceptance always originate from file input + clicks.
  const response = await fetch(`${control}/${route}`, { method: "POST", headers: { "X-MIUI-Gate-Key": key, "Content-Type": "application/json" }, body: JSON.stringify({ mode }) });
  expect(response.status, `fixture ${route}`).toBe(204);
}
async function state(): Promise<State> {
  const response = await fetch(`${control}/state`, { method: "POST", headers: { "X-MIUI-Gate-Key": key, "Content-Type": "application/json" }, body: JSON.stringify({ mode: "" }) });
  expect(response.status).toBe(200); return response.json() as Promise<State>;
}
async function signed(browser: Browser, locale: Locale, width: number, actor = "owner") {
  await ctl("actor", actor);
  const context = await browser.newContext({ ignoreHTTPSErrors: true, viewport: { width, height: width === 390 ? 844 : 900 } });
  const page = await context.newPage(), consoleMessages: string[] = [];
  page.on("console", (message) => consoleMessages.push(message.text()));
  await page.goto(`${origin}/en/`);
  await step(page, "Sign in with identity service", "signed MOCK OIDC callback establishes Secure merchant cookies", async () => {
    await page.getByRole("button", { name: "Sign in with identity service" }).click(); await page.getByTestId("shell-store-selector").waitFor();
    expect((await context.cookies()).some((c) => c.name.startsWith("__Host-") && c.secure && c.httpOnly)).toBe(true);
  });
  await page.goto(`${origin}/${locale}/customers/import?store=${store}`);
  await expect(actor === "reader" ? page.getByTestId("route-forbidden") : wizard(page)).toBeVisible();
  return { page, context, consoleMessages };
}
function customers(prefix: string, partial = false): FileCase {
  const rows = partial ? [
    `${prefix}-A,MIUI-NAME-${prefix}-A,0900000001,miui-${prefix}-a@invalid.test,yes,MIUI-NOTE-${prefix}`,
    `${prefix}-B,MIUI-NAME-${prefix}-B,0212345678,,no,`, `${prefix}-C,,,,yes,`,
    `${prefix}-D,MIUI-NAME-${prefix}-D,,miui-${prefix}-d@invalid.test,no,`,
  ] : [`${prefix}-A,MIUI-NAME-${prefix}-A,0900000001,,yes,MIUI-NOTE-${prefix}`];
  return { prefix, buffer: Buffer.from(`\ufeffref,who,tel,mail,opt,staff_note\r\n${rows.join("\r\n")}\r\n`), mapping: { external_id: "ref", name: "who", phone: "tel", email: "mail", consent: "opt" }, markers: [`${prefix}-A`, `${prefix}-D`, `MIUI-NAME-${prefix}`, `MIUI-NOTE-${prefix}`, `miui-${prefix}`, "0900000001", "0212345678"] };
}
function orders(prefix: string, customerPrefix: string): FileCase {
  return { prefix, buffer: Buffer.from(`oid,cid,when,state,money,item,qty,place\n${prefix}-H1,${customerPrefix}-A,2026-03-05 14:30:00,paid,"1,280",Tea,2,台北市\n${prefix}-H1,${customerPrefix}-A,2026-03-05 14:30:00,paid,"1,280",Socks,1,台北市\n${prefix}-H2,${customerPrefix}-D,2026-04-01 09:00:00,paid,300,Sticker,3,MIUI-CITY-DROP-${prefix}\n${prefix}-H3,MIUI-MISSING,2026-04-02,paid,400,Cup,1,臺中市\n`), mapping: { order_id: "oid", customer_id: "cid", ordered_at: "when", status: "state", total: "money", item_name: "item", item_qty: "qty", city: "place" }, markers: [`${prefix}-H1`, `${prefix}-H2`, `${customerPrefix}-A`, `${customerPrefix}-D`, "MIUI-CITY-DROP", "Tea", "Socks", "Sticker"] };
}
async function upload(page: Page, file: FileCase, kind: "customers" | "orders" = "customers") {
  await step(page, `${kind} type`, "type selection exposes genuine file input", async () => { await page.getByTestId(`import-type-${kind}`).click(); await expect(page.getByTestId("import-file")).toBeVisible(); });
  await step(page, "CSV file picker", "unknown source headers become header-only mapping options", async () => {
    await page.getByTestId("import-file").setInputFiles({ name: `${file.prefix}.csv`, mimeType: "text/csv", buffer: file.buffer });
    await expect(page.getByTestId(`import-map-${kind === "customers" ? "external_id" : "order_id"}`)).toBeVisible();
  }, "setInputFiles");
  for (const [field, header] of Object.entries(file.mapping)) await step(page, `map ${field}`, "requested header choice is visible", async () => { await page.getByTestId(`import-map-${field}`).selectOption(header); await expect(page.getByTestId(`import-map-${field}`)).toHaveValue(header); }, "selectOption");
}
async function preview(page: Page, file: FileCase, expected: { new: number; update: number; failed: number }, locale: Locale = "en") {
  const c = importCopy[locale], before = await state();
  await step(page, "Preview", "MIUI-RED-PREVIEW-DATA: actual server preview renders independent counts", async () => {
    await page.getByTestId("import-preview").click(); await expect(page.getByTestId("import-rows"), "MIUI-RED-PREVIEW-DATA").toBeVisible();
    const counts = wizard(page).locator("dl.import-counts").first();
    for (const [label, n] of [[c.newRows, expected.new], [c.updateRows, expected.update], [c.failedRows, expected.failed]] as const) await expect(counts.locator("div").filter({ has: page.getByText(label, { exact: true }) }).locator("dd")).toHaveText(String(n));
  });
  const after = await state();
  for (const key of ["owners", "profiles", "history", "batches", "consents", "receipts", "audits"]) expect(after.counts[key], `preview rolls back ${key}`).toBe(before.counts[key]);
  const request = after.requests.at(-1)!; expect(request.SHA).toBe(sha(file.buffer)); expect(request.Bytes).toBe(file.buffer.length); expect(request.HasKey).toBe(false); expect(request.ContentType.startsWith("text/csv")).toBe(true);
}
async function confirm(page: Page, counts: { created: number; updated: number; failed: number }, locale: Locale = "en") {
  const c = importCopy[locale];
  await step(page, "Review confirmation", "confirmation is a separate deliberate step before mutation", async () => { await page.getByTestId("import-confirm-next").click(); await expect(page.getByTestId("import-confirm")).toBeVisible(); });
  await step(page, "Confirm import", "MIUI-RED-COMMIT-RECEIPT: actual committed receipt appears", async () => {
    await page.getByTestId("import-confirm").click(); await expect(page.getByTestId("import-receipt"), "MIUI-RED-COMMIT-RECEIPT").toBeVisible();
    const receipt = page.getByTestId("import-receipt").locator(":scope > dl").first();
    for (const [label, n] of [[c.created, counts.created], [c.updated, counts.updated], [c.failed, counts.failed]] as const) await expect(receipt.locator("div").filter({ has: page.getByText(label, { exact: true }) }).locator("dd")).toHaveText(String(n));
  });
}
async function privacy(page: Page, markers: string[], messages: string[], original?: Buffer) {
  const html = await wizard(page).textContent() ?? "";
  // READ/MEASURE only, including existing IndexedDB via readonly transactions. No database is created by inspection.
  const stored = await page.evaluate(async () => {
    const values: string[] = [JSON.stringify({ ...localStorage }), JSON.stringify({ ...sessionStorage })];
    for (const info of await indexedDB.databases()) if (info.name) {
      const db = await new Promise<IDBDatabase>((resolve, reject) => { const request = indexedDB.open(info.name!); request.onsuccess = () => resolve(request.result); request.onerror = () => reject(request.error); request.onupgradeneeded = () => { request.transaction?.abort(); reject(new Error("unexpected database creation")); }; });
      try { for (const storeName of Array.from(db.objectStoreNames)) {
        const data = await new Promise<unknown[]>((resolve, reject) => { const request = db.transaction(storeName, "readonly").objectStore(storeName).getAll(); request.onsuccess = () => resolve(request.result as unknown[]); request.onerror = () => reject(request.error); });
        async function readable(value: unknown): Promise<string> {
          if (value instanceof Blob) return value.text();
          if (value instanceof ArrayBuffer) return new TextDecoder().decode(value);
          if (ArrayBuffer.isView(value)) return new TextDecoder().decode(new Uint8Array(value.buffer, value.byteOffset, value.byteLength));
          if (value && typeof value === "object") return JSON.stringify(value) + (await Promise.all(Object.values(value).map(readable))).join("\n");
          return String(value);
        }
        for (const value of data) values.push(await readable(value));
      } } finally { db.close(); }
    }
    return values.join("\n");
  });
  for (const marker of markers) {
    expect(html.includes(marker), "raw cell/source ID appeared in wizard").toBe(false); expect(stored.includes(marker), "raw cell persisted in storage").toBe(false);
    expect(messages.some((message) => message.includes(marker)), "raw cell printed to console").toBe(false);
  }
  if (original) for (const representation of [original.toString("utf8"), original.toString("base64"), original.toString("base64url")]) expect(stored.includes(representation), "whole uploaded file persisted in storage").toBe(false);
  expect(await page.evaluate(() => document.documentElement.scrollWidth - innerWidth)).toBeLessThanOrEqual(1);
}
async function failureCSV(page: Page, server: boolean) {
  let text = "";
  await step(page, "Download failed rows", "native click downloads safe metadata CSV", async () => {
    const event = page.waitForEvent("download"); await page.getByTestId("import-download-failed").click(); const download = await event;
    expect(download.suggestedFilename()).toBe(server ? "customer-import-results.csv" : "import-failed-rows.csv"); const location = await download.path(); expect(location).not.toBeNull(); text = await readFile(location!, "utf8");
    expect(text.charCodeAt(0)).toBe(0xfeff); expect(text.includes("MIUI-NAME-"), "failure CSV leaked cells").toBe(false); expect(text.includes("@invalid.test"), "failure CSV leaked email").toBe(false);
  });
  return text.replace(/^\ufeff/, "").trim().split(/\r?\n/).map((line) => line.split(","));
}
async function bootstrap(page: Page, prefix: string) { const file = customers(prefix); await upload(page, file); await preview(page, file, { new: 1, update: 0, failed: 0 }); await confirm(page, { created: 1, updated: 0, failed: 0 }); return file; }

test("MIUI calibration-sensitive genuine preview/commit", async ({ browser }) => {
  const { page, context } = await signed(browser, "en", 1440), file = customers("CALIBRATION");
  try {
    await expect(page.getByTestId("import-type-orders")).toBeDisabled(); await upload(page, file);
    if (process.env.LC_MIUI_CALIBRATION === "truncate-preview") await ctl("fault", "truncate-preview");
    await preview(page, file, { new: 1, update: 0, failed: 0 });
    if (process.env.LC_MIUI_CALIBRATION === "drop-customer-commit") await ctl("fault", "drop-customers");
    await confirm(page, { created: 1, updated: 0, failed: 0 });
  } finally { await context.close(); }
});

for (const locale of ["en", "zh-TW"] as const) for (const width of [1440, 390]) {
  test(`MIUI two-file flow, mapping/back/replay/CSV/storage ${locale}/${width}`, async ({ browser }) => {
    const { page, context, consoleMessages } = await signed(browser, locale, width), c = importCopy[locale], file = customers(`FLOW-${locale}-${width}`, true), order = orders(`ORDER-${locale}-${width}`, file.prefix);
    try {
      await expect(wizard(page)).toContainText(c.consentNotice); await expect(wizard(page)).toContainText(c.historyNotice); await expect(page.getByTestId("import-type-orders")).toBeDisabled();
      await upload(page, file);
      await step(page, "Guess mapping", "guess does not silently import unknown columns", async () => { await page.getByTestId("import-guess").click(); await expect(page.getByTestId("import-preview")).toBeDisabled(); });
      for (const [field, header] of Object.entries(file.mapping)) await page.getByTestId(`import-map-${field}`).selectOption(header);
      await preview(page, file, { new: 2, update: 0, failed: 2 }, locale); await expect(page.getByTestId("import-consent-count").locator("dd")).toHaveText("4");
      // Go numbers nonempty data rows from 1, excluding the header; failed rows keep those original numbers.
      await expect(page.getByTestId("import-rows").locator("tbody tr").first()).toHaveAttribute("data-testid", "import-row-2");
      await privacy(page, file.markers, consoleMessages, file.buffer);
      await step(page, "Preview Back", "back to mapping discards previous confirmation; explicit new preview required", async () => { await page.getByTestId("import-back").click(); await expect(page.getByTestId("import-map-phone")).toBeVisible(); await expect(page.getByTestId("import-confirm-next")).toHaveCount(0); });
      await preview(page, file, { new: 2, update: 0, failed: 2 }, locale);
      await step(page, "Confirm Back", "confirmation can be cancelled before any write", async () => { await page.getByTestId("import-confirm-next").click(); await page.getByTestId("import-back").click(); await expect(page.getByTestId("import-confirm-next")).toBeVisible(); });
      const before = await state(); await confirm(page, { created: 2, updated: 0, failed: 2 }, locale); const committed = await state();
      for (const key of ["owners", "profiles", "external"]) expect(committed.counts[key] - before.counts[key]).toBe(2);
      for (const key of ["batches", "receipts", "audits"]) expect(committed.counts[key] - before.counts[key]).toBe(1);
      expect(committed.counts.consents).toBe(0);
      const csv = await failureCSV(page, true); expect(csv[0]).toEqual(["row", "outcome", "code"]); expect(csv).toHaveLength(3);
      expect(csv.slice(1).every((row) => row.length === 3 && row[1] === "failed")).toBe(true);
      expect(csv.slice(1).map((row) => row[2]).sort()).toEqual(["invalid_phone", "required"]);
      await privacy(page, file.markers, consoleMessages, file.buffer);
      await step(page, "Restart/replay", "same original bytes/settings/count replay without additional rows", async () => {
        await page.getByTestId("import-restart").click(); await upload(page, file); await preview(page, file, { new: 0, update: 2, failed: 2 }, locale);
        await confirm(page, { created: 2, updated: 0, failed: 2 }, locale); await expect(page.getByTestId("import-replayed")).toHaveText(c.replayed);
        const after = await state(); for (const key of ["profiles", "owners", "batches", "receipts", "audits"]) expect(after.counts[key]).toBe(committed.counts[key]);
      });
      await expect(page.getByTestId("import-type-orders")).toBeEnabled(); await upload(page, order, "orders"); await preview(page, order, { new: 2, update: 0, failed: 1 }, locale);
      await expect(page.getByTestId("import-city-count").locator("dd")).toHaveText("1"); await privacy(page, order.markers, consoleMessages, order.buffer);
      await confirm(page, { created: 2, updated: 0, failed: 1 }, locale); const orderCSV = await failureCSV(page, true);
      expect(orderCSV[0]).toEqual(["row", "outcome", "code"]); expect(orderCSV.slice(1)).toEqual([["4", "failed", "customer_not_imported"]]);
      expect((await state()).counts.city_dropped).toBeGreaterThanOrEqual(1);
      const owner = (await state()).owners[`${file.prefix}-A`]; expect(owner).toBeTruthy();
      await page.goto(`${origin}/${locale}/customers?store=${store}`);
      await step(page, "Imported customer link", "saved customer/history is visible and survives reload", async () => {
        await page.getByTestId(`customer-open-${owner}`).click(); const history = page.getByTestId("customer-historical-orders");
        await expect(history.locator("tbody tr")).toHaveCount(1); await expect(history).toContainText("1,280"); await expect(history).toContainText("臺北市"); await expect(history).toContainText("Tea×2、Socks×1");
        await page.reload(); await expect(page.getByTestId("customer-historical-orders").locator("tbody tr")).toHaveCount(1);
      });
      await page.goto(`${origin}/${locale}/customers/import?store=${store}`); await expect(page.getByTestId("import-type-orders")).toBeDisabled();
      await privacy(page, [...file.markers, ...order.markers], consoleMessages);
    } finally { await context.close(); }
  });
}

test("MIUI all-invalid local CSV, duplicate mapping, Unicode/size/line limits and safe verdict paging", async ({ browser }) => {
  const { page, context } = await signed(browser, "en", 1440), c = importCopy.en;
  try {
    const invalid: FileCase = { prefix: "ALL-INVALID", buffer: Buffer.from("ref,who,tel\nBAD-A,MIUI-NAME-BAD,0212345678\nBAD-B,,\n"), mapping: { external_id: "ref", name: "who", phone: "tel" }, markers: [] };
    await upload(page, invalid); await preview(page, invalid, { new: 0, update: 0, failed: 2 }); await expect(page.getByTestId("import-confirm-next")).toBeDisabled();
    const local = await failureCSV(page, false); expect(local[0]).toEqual(["row", "outcome", "code"]);
    expect(local.slice(1).every((row) => row.length === 3 && row[1] === "failed")).toBe(true);
    expect(local.slice(1).map((row) => row[2]).sort()).toEqual(["invalid_phone", "required"]);
    await page.getByTestId("import-restart").click(); await page.getByTestId("import-type-customers").click();
    for (const [name, buffer, expected] of [
      ["big5.csv", Buffer.from([0xb4, 0xfa, 0xb8, 0xd5, 0x2c, 0x6e, 0x61, 0x6d, 0x65, 0x0a]), c.errors.encoding_not_utf8],
      ["oversize.csv", Buffer.alloc((2 << 20) + 1, 65), c.fileTooLarge],
    ] as const) await step(page, name, "local header/size refusal gives usable guidance without a backend request", async () => {
      const before = await state(); await page.getByTestId("import-file").setInputFiles({ name, mimeType: "text/csv", buffer });
      await expect(wizard(page).getByRole("alert")).toHaveText(expected); await expect(page.getByTestId("import-retry-same")).toHaveCount(0);
      expect((await state()).requests.length).toBe(before.requests.length);
    });
    const lines = (n: number) => Buffer.from(`customer_id,name\n${Array.from({ length: n }, () => "DUPLICATE,MIUI-NAME-LINE").join("\n")}\n`);
    const many: FileCase = { prefix: "LINES5000", buffer: lines(5000), mapping: { external_id: "customer_id", name: "name" }, markers: [] };
    await upload(page, many); await preview(page, many, { new: 0, update: 0, failed: 5000 });
    await step(page, "Verdict Next/Previous", "safe fifty-row pages change without exposing raw source IDs", async () => {
      const first = page.getByTestId("import-rows").locator("tbody tr").first();
      await page.getByTestId("import-row-next").click(); await expect(first).toHaveAttribute("data-testid", "import-row-51");
      await page.getByTestId("import-row-previous").click(); await expect(first).toHaveAttribute("data-testid", "import-row-1");
    });
    const overLines = { ...many, prefix: "LINES5001", buffer: lines(5001) };
    expect(overLines.buffer.length).toBeLessThan(2 << 20); await upload(page, overLines); const beforeRefusal = await state();
    await step(page, "5001-line Preview", "actual Go422 too_many_rows stays a confirmed plain-language refusal, with no UNKNOWN or mutation", async () => {
      await expect(page.getByTestId("import-preview")).toBeEnabled(); await page.getByTestId("import-preview").click();
      await expect(wizard(page).getByRole("alert")).toHaveText(c.errors.too_many_rows); await expect(page.getByTestId("import-retry-same")).toHaveCount(0);
      const after = await state(); expect(after.requests.length).toBe(beforeRefusal.requests.length + 1);
      const request = after.requests.at(-1)!; expect(request.Status).toBe(422); expect(request.SHA).toBe(sha(overLines.buffer)); expect(request.Path.endsWith("/imports/customers/preview")).toBe(true);
      for (const key of ["owners", "profiles", "history", "batches", "receipts", "audits"]) expect(after.counts[key]).toBe(beforeRefusal.counts[key]);
    });
    const head = Buffer.from("customer_id,name,ignored\nMIUI-BYTE,MIUI-NAME-BYTE,"); const exact: FileCase = { prefix: "EXACT2MIB", buffer: Buffer.concat([head, Buffer.alloc((2 << 20) - head.length - 1, 120), Buffer.from("\n")]), mapping: { external_id: "customer_id", name: "name" }, markers: [] };
    await upload(page, exact); await preview(page, exact, { new: 1, update: 0, failed: 0 });
    await page.getByTestId("import-back").click(); await page.getByTestId("import-map-name").selectOption("customer_id"); await expect(page.getByTestId("import-preview")).toBeDisabled();
  } finally { await context.close(); }
});

test("MIUI real erasure decreases apply count: direct fresh409/no partial mutation then deliberate fresh commit", async ({ browser }) => {
  const { page, context } = await signed(browser, "en", 1440), c = importCopy.en;
  const file: FileCase = { prefix: "STALE", buffer: Buffer.from("ref,who\nMIUI-STALE-A,MIUI-NAME-STALE-A\nMIUI-STALE-B,MIUI-NAME-STALE-B-CHANGED\n"), mapping: { external_id: "ref", name: "who" }, markers: [] };
  try {
    await upload(page, file); await preview(page, file, { new: 1, update: 1, failed: 0 }); await page.getByTestId("import-confirm-next").click();
    await ctl("erase-stale"); const erased = await state();
    await step(page, "stale Confirm", "actual409 presents fresh one-applicable preview and rolls back new customer", async () => {
      await page.getByTestId("import-confirm").click(); await expect(wizard(page)).toContainText(c.staleNotice); await expect(page.getByTestId("import-erased-count").locator("dd")).toHaveText("1"); await expect(page.getByTestId("import-receipt")).toHaveCount(0);
      const after = await state(); for (const key of ["profiles", "owners", "batches", "audits", "receipts"]) expect(after.counts[key]).toBe(erased.counts[key]); expect(after.owners["MIUI-STALE-A"]).toBeUndefined(); expect(after.requests.at(-1)?.Status).toBe(409);
    });
    await confirm(page, { created: 1, updated: 0, failed: 1 });
  } finally { await context.close(); }
});

test("MIUI changed requested mapping on identical committed bytes is refused", async ({ browser }) => {
  const { page, context } = await signed(browser, "en", 1440), file = customers("MAPPING-CONFLICT"), c = importCopy.en;
  try {
    await upload(page, file); await preview(page, file, { new: 1, update: 0, failed: 0 }); await confirm(page, { created: 1, updated: 0, failed: 0 }); const before = await state();
    await upload(page, file); await page.getByTestId("import-map-consent").selectOption(""); await preview(page, { ...file, mapping: { ...file.mapping, consent: "" } }, { new: 0, update: 1, failed: 0 });
    await step(page, "conflicting Confirm", "coded409 advises changing file, no second batch/profile mutation or UNKNOWN", async () => {
      await page.getByTestId("import-confirm-next").click(); await page.getByTestId("import-confirm").click();
      await expect(wizard(page).getByRole("alert")).toHaveText(c.idempotencyConflict); await expect(page.getByTestId("import-retry-same")).toHaveCount(0);
      const after = await state(); expect(after.requests.at(-1)?.Status).toBe(409);
      for (const key of ["batches", "profiles", "receipts", "audits"]) expect(after.counts[key]).toBe(before.counts[key]);
    });
  } finally { await context.close(); }
});

test("MIUI order stale erasure returns zero-apply fresh preview and local failure CSV", async ({ browser }) => {
  const { page, context } = await signed(browser, "en", 1440), c = importCopy.en;
  try {
    await bootstrap(page, "ORDER-STALE");
    const file: FileCase = { prefix: "ORDER-STALE-FILE", buffer: Buffer.from("order_id,customer_id,ordered_at,status,total\nMIUI-ORDER-STALE-H,ORDER-STALE-A,2026-04-01,paid,100\n"), mapping: { order_id: "order_id", customer_id: "customer_id", ordered_at: "ordered_at", status: "status", total: "total" }, markers: [] };
    await upload(page, file, "orders"); await preview(page, file, { new: 1, update: 0, failed: 0 }); await page.getByTestId("import-confirm-next").click(); await ctl("erase-stale", "orders"); const erased = await state();
    await step(page, "stale order Confirm", "real409 reduces apply to zero without storing archive or batch", async () => {
      await page.getByTestId("import-confirm").click(); await expect(wizard(page)).toContainText(c.staleNotice); await expect(page.getByTestId("import-erased-count").locator("dd")).toHaveText("1"); await expect(page.getByTestId("import-confirm-next")).toBeDisabled();
      const after = await state(); for (const key of ["history", "profiles", "batches", "receipts", "audits"]) expect(after.counts[key]).toBe(erased.counts[key]); expect(after.requests.at(-1)?.Status).toBe(409);
    });
    const rows = await failureCSV(page, false); expect(rows[1][2]).toBe("erased");
  } finally { await context.close(); }
});

test("MIUI changed-byte updates preserve unmapped fields and clear mapped empty optional fields", async ({ browser }) => {
  const { page, context } = await signed(browser, "en", 1440);
  try {
    const base = customers("UPDATE"), original = { ...base, buffer: Buffer.from(base.buffer.toString("utf8").replace("0900000001,,yes", "0900000001,miui-update@invalid.test,yes")) };
    await upload(page, original); await preview(page, original, { new: 1, update: 0, failed: 0 }); await confirm(page, { created: 1, updated: 0, failed: 0 }); const owner = (await state()).owners["UPDATE-A"];
    const omitted: FileCase = { prefix: "UPDATE-OMIT", buffer: Buffer.from("customer_id,name\nUPDATE-A,MIUI-NAME-UPDATE-OMIT\n"), mapping: { external_id: "customer_id", name: "name" }, markers: [] };
    await upload(page, omitted); await preview(page, omitted, { new: 0, update: 1, failed: 0 }); await confirm(page, { created: 0, updated: 1, failed: 0 }); const kept = await state();
    expect(kept.owners["UPDATE-A"]).toBe(owner); expect(kept.counts.update_phone_e164_present).toBe(1); expect(kept.counts.update_email_present).toBe(1);
    const cleared: FileCase = { prefix: "UPDATE-CLEAR", buffer: Buffer.from("customer_id,name,phone,email\nUPDATE-A,MIUI-NAME-UPDATE-CLEAR,,\n"), mapping: { external_id: "customer_id", name: "name", phone: "phone", email: "email" }, markers: [] };
    await upload(page, cleared); await preview(page, cleared, { new: 0, update: 1, failed: 0 }); await confirm(page, { created: 0, updated: 1, failed: 0 }); const after = await state();
    expect(after.owners["UPDATE-A"]).toBe(owner); expect(after.counts.update_phone_e164_present).toBe(0); expect(after.counts.update_email_present).toBe(0);
  } finally { await context.close(); }
});

test("MIUI UNKNOWN both kinds explicitly replay immutable bytes/map/count, no automatic retry", async ({ browser }) => {
  const { page, context } = await signed(browser, "en", 1440), file = customers("UNKNOWN"), c = importCopy.en;
  try {
    for (const kind of ["customers", "orders"] as const) {
      const current = kind === "customers" ? file : { prefix: "UNKNOWN-O", buffer: Buffer.from("oid,cid,when,state,money\nUNKNOWN-H,UNKNOWN-A,2026-04-01,paid,100\n"), mapping: { order_id: "oid", customer_id: "cid", ordered_at: "when", status: "state", total: "money" }, markers: [] };
      await upload(page, current, kind); await preview(page, current, { new: 1, update: 0, failed: 0 }); await page.getByTestId("import-confirm-next").click(); await ctl("fault", `drop-${kind}`); const before = await state();
      await step(page, `${kind} UNKNOWN Retry`, "UNKNOWN locks old intent; explicit same-buffer retry replays exactly once", async () => {
        await page.getByTestId("import-confirm").click(); await expect(wizard(page)).toContainText(c.unknownCommit); await expect(page.getByTestId("import-retry-same")).toBeVisible(); await expect(page.getByTestId("import-restart")).toBeDisabled();
        const committed = await state(); expect(committed.counts.batches - before.counts.batches).toBe(1); await page.waitForTimeout(500); expect((await state()).requests.length).toBe(committed.requests.length);
        await page.getByTestId("import-retry-same").click(); await expect(page.getByTestId("import-replayed")).toHaveText(c.replayed);
        const after = await state(); for (const key of ["batches", "profiles", "history", "receipts", "audits"]) expect(after.counts[key]).toBe(committed.counts[key]);
        const pair = after.requests.filter((r) => r.Path.endsWith(`/imports/${kind}/commit`) && r.SHA === sha(current.buffer)); expect(pair).toHaveLength(2); expect(pair[1].Mapping).toBe(pair[0].Mapping); expect(pair[1].Expected).toBe(pair[0].Expected); expect(pair.every((r) => !r.HasKey && r.Bytes === current.buffer.length)).toBe(true);
      });
    }
  } finally { await context.close(); }
});

test("MIUI history50+1 readonly paging, retry, persistence and source-safe city", async ({ browser }) => {
  const { page, context } = await signed(browser, "en", 390), h = importHistoryCopy.en;
  try {
    await bootstrap(page, "HISTORY");
    const file: FileCase = { prefix: "HISTORY-ORDERS", buffer: Buffer.from(`order_id,customer_id,ordered_at,status,total,item_name,item_qty,city\n${Array.from({ length: 51 }, (_, i) => `MIUI-HIST-${String(i + 1).padStart(4, "0")},HISTORY-A,2026-04-01 00:00:${String(i).padStart(2, "0")},paid,${100 + i},Item,1,臺中市`).join("\n")}\n`), mapping: { order_id: "order_id", customer_id: "customer_id", ordered_at: "ordered_at", status: "status", total: "total", item_name: "item_name", item_qty: "item_qty", city: "city" }, markers: [] };
    await upload(page, file, "orders"); await preview(page, file, { new: 51, update: 0, failed: 0 }); await confirm(page, { created: 51, updated: 0, failed: 0 });
    const owner = (await state()).owners["HISTORY-A"]; await page.goto(`${origin}/en/customers/${owner}?store=${store}`); const history = page.getByTestId("customer-historical-orders");
    await expect(history.locator("tbody tr")).toHaveCount(50); await expect(history.getByRole("button", { name: h.previous, exact: true })).toBeDisabled();
    await step(page, "history Next/Previous/Refresh", "real opaque cursor pages50+1 without duplicate rows, refresh restores first page", async () => {
      await history.getByRole("button", { name: h.next, exact: true }).click(); await expect(history.locator("tbody tr")).toHaveCount(1); await expect(history).toContainText("MIUI-HIST-0001");
      await history.getByRole("button", { name: h.previous, exact: true }).click(); await expect(history.locator("tbody tr")).toHaveCount(50);
      await history.getByRole("button", { name: h.refresh, exact: true }).click(); await expect(history.locator("tbody tr")).toHaveCount(50); await expect(page.getByTestId("historical-orders-count")).toHaveText(h.count(50, 51));
      await page.reload(); await expect(page.getByTestId("customer-historical-orders").locator("tbody tr")).toHaveCount(50);
    });
    await expect(history).toContainText(h.basis); await expect(history.locator("tbody").getByRole("link")).toHaveCount(0); await expect(history.locator("tbody").getByRole("button")).toHaveCount(0);
    await ctl("fault", "history-503"); await history.getByRole("button", { name: h.refresh, exact: true }).click(); await expect(history.getByRole("alert")).toContainText(h.unavailable);
    await step(page, "history Retry", "actual failed read retries saved archive", async () => { await history.getByRole("button", { name: h.retry, exact: true }).click(); await expect(history.locator("tbody tr")).toHaveCount(50); });
  } finally { await context.close(); }
});

test("MIUI imported-only erasure removes history and tombstone blocks same source ID", async ({ browser }) => {
  const { page, context } = await signed(browser, "en", 1440), c = importCopy.en;
  try {
    const file = await bootstrap(page, "ERASE");
    const order: FileCase = { prefix: "ERASE-ORDER", buffer: Buffer.from("order_id,customer_id,ordered_at,status,total\nMIUI-ERASE-H,ERASE-A,2026-04-01,paid,100\n"), mapping: { order_id: "order_id", customer_id: "customer_id", ordered_at: "ordered_at", status: "status", total: "total" }, markers: [] };
    await upload(page, order, "orders"); await preview(page, order, { new: 1, update: 0, failed: 0 }); await confirm(page, { created: 1, updated: 0, failed: 0 }); const before = await state(), owner = before.owners["ERASE-A"];
    await page.goto(`${origin}/en/customers/${owner}?store=${store}`);
    await step(page, "customer erasure", "typed deliberate erasure deletes only owned profile/archive", async () => { await page.getByTestId("customer-erase").click(); await page.getByTestId("erase-typed").fill("ERASE"); await page.getByTestId("erase-submit").click(); await expect(page.getByTestId("customer-erased")).toBeVisible(); });
    const erased = await state(); expect(erased.owners["ERASE-A"]).toBeUndefined(); expect(erased.counts.history).toBe(before.counts.history - 1); expect(erased.counts.profiles).toBe(before.counts.profiles - 1);
    await page.goto(`${origin}/en/customers/import?store=${store}`);
    const changed = { ...file, buffer: Buffer.from(file.buffer.toString("utf8").replace("MIUI-NAME-ERASE-A", "MIUI-NAME-ERASE-CHANGED")) };
    await upload(page, changed); await preview(page, changed, { new: 0, update: 0, failed: 1 }); await expect(page.getByTestId("import-erased-count").locator("dd")).toHaveText("1"); await expect(page.getByTestId("import-confirm-next")).toBeDisabled();
    const csv = await failureCSV(page, false); expect(csv[1][2]).toBe("erased"); expect((await state()).owners["ERASE-A"]).toBeUndefined(); await expect(wizard(page)).toContainText(c.onlyRowsHint);
  } finally { await context.close(); }
});

test("MIUI read-only identity sees history but import permission refusal", async ({ browser }) => {
  const { page, context } = await signed(browser, "en", 390, "reader");
  try {
    await expect(page.getByTestId("route-forbidden")).toContainText(shellCopy.en.forbidden);
    await expect(wizard(page)).toHaveCount(0); await expect(page.getByTestId("import-file")).toHaveCount(0);
    const owner = (await state()).owners["HISTORY-A"]; await page.goto(`${origin}/en/customers/${owner}?store=${store}`);
    await expect(page.getByTestId("customer-historical-orders").locator("tbody tr")).toHaveCount(50);
  } finally { await context.close(); }
});

test("MIUI pagehide drops raw file, UNKNOWN revoked session cannot submit under new boundary", async ({ browser }) => {
  const { page, context, consoleMessages } = await signed(browser, "en", 1440), file = customers("AUTH-FENCE"), c = importCopy.en;
  try {
    await upload(page, file); await preview(page, file, { new: 1, update: 0, failed: 0 });
    await step(page, "navigate away/Back", "pagehide drops in-memory CSV; fresh file choice required", async () => { await page.goto(`${origin}/en/customers?store=${store}`); await page.goBack(); await expect(page.getByTestId("import-confirm-next")).toHaveCount(0); await expect(page.getByTestId("import-type-orders")).toBeDisabled(); });
    await upload(page, file); await preview(page, file, { new: 1, update: 0, failed: 0 }); await page.getByTestId("import-confirm-next").click(); await ctl("fault", "drop-customers"); await page.getByTestId("import-confirm").click(); await expect(wizard(page)).toContainText(c.unknownCommit);
    const pending = await state(); await ctl("revoke");
    await step(page, "Retry after revocation", "signed-out clears raw payload and preserves honest unconfirmed message", async () => {
      await page.getByTestId("import-retry-same").click(); await expect(wizard(page)).toContainText(importViewCopy.en.sessionUnknown); await expect(page.getByTestId("import-retry-same")).toHaveCount(0);
      expect((await state()).requests.length).toBe(pending.requests.length); await privacy(page, file.markers, consoleMessages);
    });
  } finally { await context.close(); }
});
