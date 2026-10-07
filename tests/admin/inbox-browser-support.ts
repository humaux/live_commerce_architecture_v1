// Purpose: shared real-store navigation, privacy checks and serial ledger output for inbox browser specs.
// Depends on: Playwright, Node fs/path and Go-owned LC_BROWSER_* fixture environment.
// Used by: ordinary inbox and credential-only bundle specs; never installs a test or mocks an API.
import { expect, type Page } from "@playwright/test";
import { readFile, writeFile } from "node:fs/promises";
import { resolve } from "node:path";

const required = (name: string) => {
  const value = process.env[name];
  if (!value) throw new Error(`${name} is required`);
  return value;
};
const origin = required("LC_BROWSER_PUBLIC_ORIGIN");
const api = required("LC_BROWSER_API_ORIGIN");
const evidence = required("LC_BROWSER_EVIDENCE");
const store = required("LC_BROWSER_INBOX_STORE");
const otherStore = required("LC_BROWSER_INBOX_OTHER_STORE");
const buyerOrigin = required("LC_BROWSER_INBOX_BUYER_ORIGIN");
const ids = JSON.parse(required("LC_BROWSER_INBOX_IDS")) as Record<string, string>;
const sentinels = [
  "SYNTHETIC-INBOX-DM-PRIVATE-7c32",
  "SYNTHETIC-INBOX-NAME-7c32",
  "900007320001",
  "SYNTHETIC-INBOX-REPLY-7c32",
  "SYNTHETIC-INBOX-ACK-LOST-7c32",
];
const dm = sentinels[0];
/** Own one spec's evidence rows without sharing mutable ledger state across workers. */
function createInboxLedger() {
  const ledger: Array<{
    page: string;
    control: string;
    action: string;
    expected: string;
    actual: string;
    status: string;
  }> = [];
  const record = (control: string, action: string, actual: string) =>
    ledger.push({
      page: "Messages",
      control,
      action,
      expected: actual,
      actual,
      status: "PASS",
    });
  return { ledger, record };
}

/** Append this spec's fixed ledger rows; the inbox project runs workers serially. */
async function writeInboxLedger(ledger: ReturnType<typeof createInboxLedger>["ledger"]) {
  const file = resolve(evidence, "click-ledger.json");
  let previous: unknown = [];
  try { previous = JSON.parse(await readFile(file, "utf8")); }
  catch (error) { if ((error as NodeJS.ErrnoException).code !== "ENOENT") throw error; }
  if (!Array.isArray(previous)) throw new Error("Invalid inbox evidence ledger");
  await writeFile(file, JSON.stringify([...previous, ...ledger], null, 2), { mode: 0o600 });
}
/** Switch through the real shell control and await its completed store navigation. */
async function selectFixtureStore(page: Page, next: string) {
  // The shell performs a full overview navigation; selector state alone can be a pre-navigation snapshot.
  const overview = new URL(`/en?store=${next}`, origin).href;
  const selector = page.getByTestId("shell-store-selector");
  await expect(selector).toBeVisible();
  if ((await selector.inputValue()) !== next) await selector.selectOption(next);
  await expect(page).toHaveURL(overview);
  await expect(page.getByTestId("inbox-page")).toHaveCount(0);
  await expect(page.getByTestId("shell-store-selector")).toHaveValue(next);
  await expect(page.getByTestId("nav-group-messages")).toBeVisible();
}

/** Sign into the MOCK IdP and enter Messages using the exact fixture store. */
async function login(page: Page) {
  await page.goto(new URL("/en/", origin).href);
  await page.getByRole("button", { name: "Sign in with identity service" }).click();
  // Wait for the signed landing's default-store navigation before driving another real store change.
  await expect(page).toHaveURL((url) => url.origin === origin && url.pathname === "/en" &&
    /^[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}$/.test(url.searchParams.get("store") ?? ""));
  await expect(page.getByTestId("nav-group-messages")).toBeVisible();
  await selectFixtureStore(page, store);
  await page.getByTestId("nav-group-messages").click();
  await expect(page).toHaveURL(new URL(`/en/messages?store=${store}`, origin).href);
  await expect(page.getByTestId("inbox-page")).toBeVisible();
  await expect(page.getByTestId(`conversation-${ids.open}`)).toBeVisible();
  await expect(page.getByTestId(`conversation-${ids.open}`)).toContainText(sentinels[1]);
}

/** Read privacy-sensitive browser channels and assert that fixture sentinels were not retained. */
async function privateBoundary(page: Page, consoleMessages: string[]) {
  for (const sentinel of sentinels) expect(page.url(), "I11 page URL").not.toContain(sentinel);
  // G-UI8 audit [READ/MEASURE]: reads browser storage and resource URLs; never changes product state.
  const state = await page.evaluate(async () => ({
    local: JSON.stringify(Object.entries(localStorage)),
    session: JSON.stringify(Object.entries(sessionStorage)),
    databases: await indexedDB.databases(),
    caches: await caches.keys(),
    urls: performance.getEntriesByType("resource").map((entry) => entry.name),
  }));
  const serialized = JSON.stringify(state);
  for (const sentinel of sentinels) expect(serialized, "I11 URL/storage sentinel leak").not.toContain(sentinel);
  // Inbox may not create a persistence channel for customer text. Existing shell uses localStorage only.
  expect(state.databases, "I11 IndexedDB must not persist inbox data").toEqual([]);
  expect(state.caches, "I11 Cache Storage must not persist inbox data").toEqual([]);
  for (const text of consoleMessages)
    for (const sentinel of sentinels) expect(text, "I11 browser log leak").not.toContain(sentinel);
}

export { required, origin, api, evidence, store, otherStore, buyerOrigin, ids, sentinels, dm,
  createInboxLedger, writeInboxLedger, selectFixtureStore, login, privateBoundary };
