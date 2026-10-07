// Purpose: INU09 real claim-link issuance, native clipboard and persisted receipt without credential-bearing traces.
// Depends on: real Go/PG/M6/M7 fixture and shared inbox navigation; credential remains in browser memory/clipboard.
// Used by: --browser-inbox; worker-scoped artifact settings are isolated from ordinary inbox diagnostics.
import { expect, type Page } from "@playwright/test";
import { test } from "./inbox-private-evidence";
import { mkdir } from "node:fs/promises";
import { createHash } from "node:crypto";
import { origin, api, evidence, store, ids, buyerOrigin, sentinels,
  createInboxLedger, writeInboxLedger, login, privateBoundary } from "./inbox-browser-support";
const { ledger, record } = createInboxLedger();
// These are worker-scoped options: isolate this credential-bearing scenario in its own spec.
test.use({ baseURL: origin, headless: false, trace: "off", video: "off", screenshot: "off" });
test.beforeAll(async () => { await mkdir(evidence, { recursive: true }); });
test.afterEach(async ({}, info) => {
  if (info.status !== info.expectedStatus) ledger.push({ page: "Messages", control: info.title,
    action: "scenario", expected: "all named assertions pass", actual: info.status ?? "UNKNOWN", status: "FAIL" });
});
test.afterAll(async () => { await writeInboxLedger(ledger); });

// I11: the one-time credential stays inside this browser evaluation; the Node driver sees only booleans/digest.
async function bundleClipboardProof(page: Page, expectedOrigin: string) {
  return page.evaluate(async (publishedOrigin) => {
    const copied = await navigator.clipboard.readText();
    let target: URL;
    try { target = new URL(copied); }
    catch { return { origin_valid: false, path_valid: false, fragment_valid: false, private_boundary: false, credentialSHA256: "" }; }
    const token = target.hash.startsWith("#t=") ? target.hash.slice(3) : "";
    const fragmentValid = /^[A-Za-z0-9_-]{43}$/.test(token) && target.search === "";
    const digest = await crypto.subtle.digest("SHA-256", new TextEncoder().encode(token));
    const privateState = JSON.stringify({
      url: location.href, html: document.documentElement.outerHTML,
      local: Object.entries(localStorage), session: Object.entries(sessionStorage),
      resources: performance.getEntriesByType("resource").map((entry) => entry.name),
    });
    return {
      origin_valid: target.origin === publishedOrigin && target.protocol === "https:",
      path_valid: target.pathname === "/en/claim",
      fragment_valid: fragmentValid,
      private_boundary: fragmentValid && !privateState.includes(token) &&
        (await indexedDB.databases()).length === 0 && (await caches.keys()).length === 0,
      credentialSHA256: Array.from(new Uint8Array(digest), (byte) => byte.toString(16).padStart(2, "0")).join(""),
    };
  }, expectedOrigin);
}

async function bundleFacts(page: Page, credentialSHA256 = "", idempotencyKey = "") {
  // POST carries the digest in a bounded body, never a URL/log; this test endpoint only SELECTs PG.
  const response = await page.request.post(`${api}/__test/inbox-bundle-facts`, { data: { credentialSHA256, idempotencyKey } });
  expect(response.status(), "credential-safe PG observation").toBe(200);
  return await response.json() as {
    links: number; generation: number; hash_matches: boolean; principal_matches: boolean;
    ttl_valid: boolean; receipt_count: number; receipt_valid: boolean; pending_manual: boolean; dm_operations: number;
  };
}

// M7 contains a one-time token: retain-on-failure tracing would retain the response even when an assertion fails.
test.afterEach(async ({ page }) => {
  // CI native clipboard is task-owned; clear the synthetic capability even after a failed assertion.
  await page.evaluate(() => navigator.clipboard.writeText(""));
});
test("INU09 flagged bundle copies actual M7 link through native clipboard and persists one token-free receipt", async ({ page, context }) => {
  await context.grantPermissions(["clipboard-read", "clipboard-write"], { origin });
  // The generic diagnostic collector stores text. This sensitive case retains only local digests, never log bodies.
  const consoleDigests: string[] = [];
  let sentinelLogged = false;
  const measureConsole = (text: string) => {
    sentinelLogged ||= sentinels.some((sentinel) => text.includes(sentinel));
    for (const candidate of text.match(/[A-Za-z0-9_-]{43}/g) ?? [])
      consoleDigests.push(createHash("sha256").update(candidate).digest("hex"));
  };
  page.removeAllListeners("console");
  page.removeAllListeners("pageerror");
  page.on("console", (message) => measureConsole(message.text()));
  page.on("pageerror", (error) => measureConsole(error.message));
  await login(page);
  const before = await bundleFacts(page);
  expect(before.links).toBe(0);
  expect(before.generation).toBe(0);
  expect(before.receipt_count).toBe(0);
  expect(before.pending_manual).toBe(true);
  await page.getByTestId(`conversation-${ids.bundle}`).click();
  await expect(page.getByTestId("bundle-recovery")).toBeVisible();
  await expect(page.getByTestId("buyer-panel")).toBeVisible();
  await expect(page.getByTestId("bundle-copy-link")).toBeEnabled();
  await expect(page.getByTestId("reply-send")).toHaveCount(0);
  const endpoint = `/api/stores/${store}/live-sessions/${ids.bundle_session}/claims/bundles/${ids.bundle}/link`;
  let linkPosts = 0;
  page.on("request", (request) => {
    if (request.method() === "POST" && new URL(request.url()).pathname === endpoint) linkPosts++;
  });
  const bundlesRead = page.waitForResponse((response) => response.request().method() === "GET" &&
    new URL(response.url()).pathname === `/api/stores/${store}/live-sessions/${ids.bundle_session}/claims/bundles`);
  const issued = page.waitForResponse((response) => response.request().method() === "POST" &&
    new URL(response.url()).pathname === endpoint);
  await page.getByTestId("bundle-copy-link").click();
  expect((await bundlesRead).status(), "real M6 generation read").toBe(200);
  const response = await issued;
  expect(response.status(), "real BFF M7 issuance").toBe(200);
  expect(response.headers()["cache-control"]).toContain("no-store");
  expect(response.headers()["referrer-policy"]).toBe("no-referrer");
  const key = response.request().headers()["idempotency-key"];
  expect(/^[A-Za-z0-9_-]{8,128}$/.test(key), "M7 uses a real command key").toBe(true);
  expect(response.request().postDataJSON()).toEqual({ expected_generation: 0, release_binding: false });
  // Never read, attach or log the credential-bearing M7 response body.
  await expect(page.getByTestId("bundle-recovery").getByRole("status")).toHaveText("Link copied");
  if (process.env.LC_INBOX_BUNDLE_CALIBRATION === "retain-credential") {
    // MOCK-only injected DOM retention: the privacy assertion must fail without retaining the credential in evidence.
    await page.evaluate(async () => {
      const leaked = document.createElement("p");
      leaked.textContent = await navigator.clipboard.readText();
      document.body.append(leaked);
    });
  }
  const copied = await bundleClipboardProof(page, buyerOrigin);
  expect(copied.origin_valid).toBe(true);
  expect(copied.path_valid).toBe(true);
  expect(copied.fragment_valid).toBe(true);
  expect(copied.private_boundary).toBe(true);
  expect(/^[0-9a-f]{64}$/.test(copied.credentialSHA256)).toBe(true);
  const after = await bundleFacts(page, copied.credentialSHA256, key);
  expect(after.links).toBe(1);
  expect(after.generation).toBe(1);
  expect(after.hash_matches).toBe(true);
  expect(after.principal_matches).toBe(true);
  expect(after.ttl_valid).toBe(true);
  expect(after.receipt_count).toBe(1);
  expect(after.receipt_valid).toBe(true);
  expect(after.pending_manual).toBe(false);
  expect(after.dm_operations).toBe(before.dm_operations);
  await page.getByTestId("bundle-copy-link").click();
  await expect(page.getByTestId("bundle-recovery").getByRole("status")).toHaveText("Link copied");
  const second = await bundleClipboardProof(page, buyerOrigin);
  expect(second.credentialSHA256 === copied.credentialSHA256, "native recopy uses the cached credential").toBe(true);
  expect(second.private_boundary).toBe(true);
  expect(consoleDigests.includes(copied.credentialSHA256), "credential absent from console/errors").toBe(false);
  const recopied = await bundleFacts(page, second.credentialSHA256, key);
  expect(recopied.links).toBe(1);
  expect(recopied.generation).toBe(1);
  expect(recopied.hash_matches).toBe(true);
  expect(recopied.receipt_count).toBe(1);
  expect(recopied.receipt_valid).toBe(true);
  expect(recopied.dm_operations).toBe(before.dm_operations);
  expect(linkPosts).toBe(1);
  const refreshed = page.waitForResponse((r) => r.request().method() === "GET" &&
    new URL(r.url()).pathname === `/api/stores/${store}/inbox/conversations`);
  await page.getByTestId("inbox-page").getByRole("button", { name: "Refresh", exact: true }).click();
  expect((await refreshed).status()).toBe(200);
  await expect(page.getByTestId(`conversation-${ids.bundle}`)).toHaveCount(0);
  await privateBoundary(page, []);
  expect(sentinelLogged, "private inbox sentinel absent from safe console observer").toBe(false);
  record("Copy claim link", "select flagged bundle; click copy twice; refresh", "native clipboard matches one persisted M7 link; token-free receipt; no DM; manual flag clears");
});
