// INDEPENDENT browser gate for unit staff-team (contracts/storefront-v2.md §D; docs/delivery/units/staff-team.md), Chromium against the
// packaged Next admin BFF, an in-process Go api (staff routes + the real SMTP adapter against the loopback fake) and real PG, started by
// tests/foundation/browser_staff_team_test.go (`bash scripts/dev/test-local.sh --browser-password-auth`). Password login on, OIDC off.
// Written from the contract and the brief, not from Team.tsx / TeamInvite.tsx: the chain is the one the brief names, driven by
// data-testid hooks the brief lists for the Team page (team-invite-*, member-*, invite-*) and by type/autocomplete attributes of the
// existing sign-up form (tests/admin/password-auth.spec.ts is the template for mailbox + sign-up mechanics).
// One serial chain per (locale x viewport) in {zh-TW, en} x {desktop 1586, mobile 390}; each chain has its OWN owner and invitee:
//   owner signs up and creates a store -> owner invites a FULFILMENT user from the Team page -> the invitation mail is captured
//   (inviter's locale, link = <public origin>/<locale>/invite/<token>, no query) -> a signed-out invitee learns nothing about the
//   store/role from the page, signs up with the invited address, reopens the link, accepts -> the invitee sees orders and exactly the
//   fulfilment order-side powers, direct /team and /billing are refused (403 on the BFF, notice on the page), the BFF refuses every
//   team management action with 403 -> owner changes the role to viewer in the UI -> the invitee loses export/fulfilment on the next
//   request -> owner removes the member in the UI (confirm) -> the invitee's next request is refused and the used link stays dead.
// First chain only: an unrelated signed-in account opening the link gets the SAME generic text as an unknown token.
// Token hygiene in the browser: the token is only ever in the emailed link and the one document navigation to it; never in another
// request URL, referrer, console line or page text. Every token/password/code/address is written to canaries.txt for the Go-side log scan.
// Separate test "known gap": role-aware navigation (a fulfilment user must not be offered Team/Billing); its failure is a recorded
// DEFECT (output/staff-team/tests/DEFECTS.md), not a harness error. LC_STAFF_TEAM_SPEC_ARGS="--grep-invert @defect" isolates it.
import { createHash, randomBytes } from "node:crypto";
import { appendFileSync, mkdirSync } from "node:fs";
import { join } from "node:path";
import { expect, test, type BrowserContext, type Page } from "@playwright/test";

function requiredOrigin(name: string) {
  const value = process.env[name];
  if (!value) throw new Error(`${name} is required`);
  const url = new URL(value);
  if (url.origin !== value || (url.protocol !== "http:" && url.protocol !== "https:")) throw new Error(`${name} must be an exact HTTP(S) origin`);
  return url.origin;
}
const publicOrigin = requiredOrigin("LC_BROWSER_PUBLIC_ORIGIN");
const inspectOrigin = requiredOrigin("LC_BROWSER_INSPECT_ORIGIN");
const evidenceDir = process.env.LC_BROWSER_EVIDENCE_DIR;
if (!evidenceDir) throw new Error("LC_BROWSER_EVIDENCE_DIR is required");
mkdirSync(join(evidenceDir, "screens"), { recursive: true });

test.use({ baseURL: publicOrigin, trace: "off", screenshot: "only-on-failure" });
test.describe.configure({ timeout: 240_000 });

const csrfName = "__Host-commerce_csrf";
const viewports = { desktop: { width: 1586, height: 992 }, mobile: { width: 390, height: 844 } } as const;
type Locale = "zh-TW" | "en";
type VP = keyof typeof viewports;

// ---- sentinels, mailbox, helpers (mechanics shared with password-auth.spec.ts, copied: specs are not importable) -----------------
function remember(value: string) {
  appendFileSync(join(evidenceDir!, "canaries.txt"), value + "\n");
  return value;
}
const letters = (n: number) => Array.from(randomBytes(n), (b: number) => String.fromCharCode(97 + (b % 26))).join("");
const newEmail = () => remember(`stf.${letters(14)}@example.test`);
const newPassword = () => remember(`Stf-${letters(22)}`);
const source = () => `10.${randomBytes(1)[0]}.${randomBytes(1)[0]}.${1 + (randomBytes(1)[0] % 250)}`;

type Mail = { subject: string; text: string; html: string };
async function mails(to: string): Promise<Mail[]> {
  const res = await fetch(`${inspectOrigin}/mail?to=${encodeURIComponent(to.toLowerCase())}`);
  return (await res.json()) as Mail[];
}
async function nthMail(to: string, n: number): Promise<Mail> {
  const deadline = Date.now() + 20_000;
  for (;;) {
    const all = await mails(to);
    if (all.length >= n) return all[n - 1];
    if (Date.now() > deadline) throw new Error(`mail ${n} to the address did not arrive`);
    await new Promise((r) => setTimeout(r, 100));
  }
}
const codeOf = (mail: Mail) => {
  const m = /(?:^|[^0-9A-Za-z])(\d{6})(?:[^0-9A-Za-z]|$)/.exec(mail.text);
  if (!m) throw new Error("no 6-digit code in the mail");
  return remember(m[1]);
};

const emailInput = (page: Page) => page.locator('input[type="email"]');
const passwordInput = (page: Page) => page.locator('input[type="password"]');
const codeInput = (page: Page) => page.locator('input[autocomplete="one-time-code"]');
const submitButton = (page: Page) => page.locator('form button[type="submit"]');

async function person(browser: import("@playwright/test").Browser, vp: VP) {
  const context = await browser.newContext({ ignoreHTTPSErrors: true, baseURL: publicOrigin, viewport: viewports[vp] });
  await context.setExtraHTTPHeaders({ "x-forwarded-for": source() }); // emulates Caddy: one source per person
  const page = await context.newPage();
  return { context, page };
}

async function browserJSON(page: Page, path: string, options: { method?: string; body?: unknown; csrf?: boolean; headers?: Record<string, string> } = {}) {
  return page.evaluate(
    async ({ path, options, csrfName }) => {
      const headers = new Headers(options.headers ?? {});
      if (options.body !== undefined) headers.set("Content-Type", "application/json");
      if (options.csrf) {
        const values = document.cookie.split(";").map((p) => p.trim()).filter((p) => p.startsWith(`${csrfName}=`)).map((p) => p.slice(csrfName.length + 1));
        if (values.length !== 1) throw new Error("exact CSRF cookie is unavailable");
        headers.set("X-CSRF-Token", values[0]);
      }
      const response = await fetch(path, { method: options.method ?? "GET", headers, body: options.body === undefined ? undefined : JSON.stringify(options.body), credentials: "same-origin" });
      const text = await response.text();
      let body: unknown = null;
      try { body = text ? JSON.parse(text) : null; } catch { body = text; }
      return { status: response.status, body: body as any };
    },
    { path, options, csrfName },
  );
}
const team = (page: Page, action: string, body: unknown) => browserJSON(page, `/api/team/${action}`, { method: "POST", body, csrf: true });

async function shot(page: Page, name: string, locale: string, vp: string) {
  const path = join(evidenceDir!, "screens", `${name}-${locale}-${vp}.png`);
  const png = await page.screenshot({ path, fullPage: false });
  appendFileSync(join(evidenceDir!, "screenshots.jsonl"), JSON.stringify({ name, locale, viewport: vp, sha256: createHash("sha256").update(png).digest("hex") }) + "\n");
}
async function noHorizontalScroll(page: Page, what: string) {
  const overflow = await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth);
  expect(overflow, `${what}: no horizontal page scroll`).toBeLessThanOrEqual(0);
}

// Sign-up up to a session; `priorMails` = mails already sent to the address (the sign-up code is the next one).
// `fromHere` signs up on the page the caller already opened (an invite page's own sign-up link) instead of a fresh /signup.
async function signUp(page: Page, locale: string, email: string, password: string, priorMails: number, fromHere = false) {
  if (!fromHere) await page.goto(`/${locale}/signup`);
  await emailInput(page).fill(email);
  await passwordInput(page).fill(password);
  await submitButton(page).click();
  await expect(codeInput(page)).toBeVisible();
  const code = codeOf(await nthMail(email, priorMails + 1));
  await codeInput(page).fill(code);
  const [response] = await Promise.all([
    page.waitForResponse((r) => new URL(r.url()).pathname === "/api/auth/password/verify"),
    submitButton(page).click(),
  ]);
  expect(response.status()).toBe(200);
  await expect(codeInput(page)).toHaveCount(0);
}

async function createStore(page: Page, locale: string): Promise<string> {
  await page.goto(`/${locale}/`);
  const created = await browserJSON(page, "/api/onboarding/initial-store", {
    method: "POST", csrf: true, headers: { "Idempotency-Key": `stf-${letters(16)}` },
    body: { tenant_name: "Staff Merchant", store_name: "Staff Store", warehouse_name: "Staff Warehouse", currency: "TWD" },
  });
  expect(created.status, JSON.stringify(created.body)).toBe(200);
  return created.body.store_id as string;
}

// Records every request URL + referrer the page makes, and every console line, for the token-hygiene check.
type Net = { requests: { url: string; type: string; method: string; referer: string | null }[]; console: string[] };
function watch(page: Page): Net {
  const net: Net = { requests: [], console: [] };
  page.on("request", (r) => {
    void r.headerValue("referer").then((referer) => net.requests.push({ url: r.url(), type: r.resourceType(), method: r.method(), referer }));
  });
  page.on("console", (m) => net.console.push(m.text()));
  page.on("pageerror", (e) => net.console.push(String(e)));
  return net;
}

const navLabel = { team: { en: "Team", "zh-TW": "團隊" }, billing: { en: "Billing", "zh-TW": "帳單" } } as const;
const navButton = (page: Page, label: string) => page.locator("aside nav").getByRole("button", { name: label, exact: true });
const roleWord = { en: "Fulfilment", "zh-TW": "履約" } as const;
const subjectRe = { en: /invited/i, "zh-TW": /邀請/ } as const;
const billingForbidden = { en: /permission to manage billing/, "zh-TW": /無權管理帳單/ } as const;

const chains: { locale: Locale; vp: VP; first: boolean }[] = [
  { locale: "zh-TW", vp: "desktop", first: true },
  { locale: "en", vp: "mobile", first: false },
  { locale: "en", vp: "desktop", first: false },
  { locale: "zh-TW", vp: "mobile", first: false },
];

test.describe("staff chains", () => {
test.describe.configure({ mode: "serial" });
for (const { locale, vp, first } of chains) {
  test(`staff chain ${locale} ${vp}: invite -> mail -> sign up -> accept -> fulfilment powers -> viewer -> revoke`, async ({ browser }) => {
    const owner = await person(browser, vp);
    const invitee = await person(browser, vp);
    const ownerNet = watch(owner.page);
    const inviteeNet = watch(invitee.page);
    const label = `${locale}-${vp}`;
    try {
      // ---- owner: account, store, Team page -------------------------------------------------------------------------------
      const ownerEmail = newEmail();
      await signUp(owner.page, locale, ownerEmail, newPassword(), 0);
      const storeId = await createStore(owner.page, locale);
      await owner.page.goto(`/${locale}/team?store=${storeId}`);
      await expect(owner.page.getByTestId("team-page")).toBeVisible();
      await expect(owner.page.getByTestId("team-invite-email")).toBeVisible();
      await expect(owner.page.getByTestId("team-members")).toBeVisible();
      await noHorizontalScroll(owner.page, "team page (owner)");
      // the owner is offered Team and Billing
      await expect(navButton(owner.page, navLabel.team[locale])).toHaveCount(1);
      await expect(navButton(owner.page, navLabel.billing[locale])).toHaveCount(1);
      await shot(owner.page, "team-owner-empty", locale, vp);

      // ---- owner invites a fulfilment user from the Team page -------------------------------------------------------------
      const inviteeEmail = newEmail();
      await owner.page.getByTestId("team-invite-email").fill(inviteeEmail);
      await owner.page.getByTestId("team-invite-role").selectOption("fulfilment");
      const [inviteResponse] = await Promise.all([
        owner.page.waitForResponse((r) => new URL(r.url()).pathname === "/api/team/invite"),
        owner.page.getByTestId("team-invite-send").click(),
      ]);
      expect(inviteResponse.status()).toBe(201);
      expect(await inviteResponse.text(), "the invite answer never carries the token").not.toMatch(/[A-Za-z0-9_-]{43}/);
      await expect(owner.page.getByTestId("team-notice")).toBeVisible();
      await expect(owner.page.getByTestId("team-invites")).toContainText(inviteeEmail);
      await shot(owner.page, "team-owner-invited", locale, vp);

      // ---- the captured mail ---------------------------------------------------------------------------------------------
      const invitation = await nthMail(inviteeEmail, 1);
      expect(invitation.subject, "mail is in the inviter's locale").toMatch(subjectRe[locale]);
      const link = /https?:\/\/[^\s"<>]+\/invite\/[A-Za-z0-9_-]{43}/.exec(invitation.text)?.[0];
      expect(link, "an invitation link in the text body").toBeTruthy();
      const url = new URL(link!);
      expect(url.origin).toBe(publicOrigin);
      expect(url.pathname).toMatch(new RegExp(`^/${locale}/invite/[A-Za-z0-9_-]{43}$`));
      expect({ search: url.search, hash: url.hash }, "the link is path-only").toEqual({ search: "", hash: "" });
      expect(invitation.html).toContain(link!);
      const token = remember(url.pathname.split("/").pop()!);

      // ---- invitee, signed out: the link tells a stranger nothing ----------------------------------------------------------
      await invitee.page.goto(link!);
      await expect(invitee.page.getByTestId("invite-need-login")).toBeVisible();
      const strangerText = await invitee.page.locator("body").innerText();
      for (const leak of ["Staff Store", "Staff Merchant", roleWord[locale], ownerEmail]) expect(strangerText, `signed-out invite page leaks ${leak}`).not.toContain(leak);
      await noHorizontalScroll(invitee.page, "invite page (signed out)");
      await shot(invitee.page, "invite-signed-out", locale, vp);
      await invitee.page.getByTestId("invite-signup").click();
      // invite-next (a2193e2, fragment carrier): the sign-up link hands its own invite path over in the URL FRAGMENT only. A query
      // string would put the token in a request URL (Referer, access logs), which the hygiene check below forbids.
      await expect(invitee.page).toHaveURL(`${publicOrigin}/${locale}/signup#next=${encodeURIComponent(url.pathname)}`);

      // ---- (first chain) an unrelated signed-in account gets the SAME generic refusal as an unknown token -------------------
      if (first) {
        const intruder = await person(browser, vp);
        try {
          await signUp(intruder.page, locale, newEmail(), newPassword(), 0);
          await intruder.page.goto(link!);
          await intruder.page.getByTestId("invite-accept").click();
          await expect(intruder.page.getByTestId("invite-problem")).toBeVisible();
          const wrongAccount = (await intruder.page.getByTestId("invite-problem").innerText()).trim();
          await intruder.page.goto(`/${locale}/invite/${randomBytes(32).toString("base64url")}`);
          await intruder.page.getByTestId("invite-accept").click();
          await expect(intruder.page.getByTestId("invite-problem")).toBeVisible();
          const unknownToken = (await intruder.page.getByTestId("invite-problem").innerText()).trim();
          expect(wrongAccount.length).toBeGreaterThan(0);
          expect(wrongAccount, "wrong account and unknown token read the same").toBe(unknownToken);
          expect(await mails(inviteeEmail), "the refused attempt sent the invitee nothing").toHaveLength(1);
          const probe = await browserJSON(intruder.page, "/api/stores");
          expect((probe.body as { items: unknown[] }).items, "the intruder gained no store").toEqual([]);
        } finally {
          await intruder.context.close();
        }
      }

      // ---- invitee signs up (on that very sign-up page) with the invited address, is returned to the invite page, accepts -------
      await signUp(invitee.page, locale, inviteeEmail, newPassword(), 1, true);
      await expect(invitee.page, "after sign-up the invitee is back on the invite link, not the dashboard").toHaveURL(link!);
      await expect(invitee.page.getByTestId("invite-accept")).toBeVisible();
      await noHorizontalScroll(invitee.page, "invite page (signed in)");
      await shot(invitee.page, "invite-signed-in", locale, vp);
      const [acceptResponse] = await Promise.all([
        invitee.page.waitForResponse((r) => new URL(r.url()).pathname === "/api/team/accept"),
        invitee.page.getByTestId("invite-accept").click(),
      ]);
      expect(acceptResponse.status()).toBe(200);
      expect(await acceptResponse.json()).toEqual({ store_id: storeId, role: "fulfilment" });
      await expect(invitee.page.getByTestId("invite-done")).toBeVisible();
      await shot(invitee.page, "invite-accepted", locale, vp);

      // ---- fulfilment powers: sees orders; exact order-side bundle; no billing, no team management ----------------------------
      await invitee.page.goto(`/${locale}/orders?store=${storeId}`);
      await expect(invitee.page.getByTestId("orders-refresh")).toBeVisible();
      await expect(invitee.page.getByTestId("orders-export"), "fulfilment holds orders:export").toBeVisible();
      await noHorizontalScroll(invitee.page, "orders page (fulfilment)");
      await shot(invitee.page, "orders-fulfilment", locale, vp);
      if (vp === "desktop") {
        await invitee.page.goto(`/${locale}/`);
        await invitee.page.getByTestId("nav-orders").click();
        await expect(invitee.page).toHaveURL(new RegExp(`/${locale}/orders`));
      }
      const stores = await browserJSON(invitee.page, "/api/stores");
      expect((stores.body as { items: { id: string }[] }).items.map((s) => s.id)).toEqual([storeId]);
      const actions = await browserJSON(invitee.page, `/api/stores/${storeId}/order-actions`);
      expect(actions).toEqual({ status: 200, body: { refund: false, fulfillment_write: true, orders_export: true } });
      expect((await browserJSON(invitee.page, `/api/stores/${storeId}/orders`)).status).toBe(200);
      expect((await browserJSON(invitee.page, `/api/stores/${storeId}/billing`)).status, "billing is owner-only").toBe(403);
      const mine = await team(invitee.page, "list", { store_id: storeId });
      expect(mine.status).toBe(200);
      expect(mine.body).toEqual({ my_role: "fulfilment", members: [], invitations: [] });
      const someone = "00000000-0000-4000-8000-000000000001";
      for (const [action, body] of [
        ["invite", { store_id: storeId, email: newEmail(), role: "owner", locale }],
        ["set-role", { store_id: storeId, principal_id: someone, role: "owner" }],
        ["remove", { store_id: storeId, principal_id: someone }],
        ["revoke-invite", { store_id: storeId, invite_id: someone }],
      ] as const) {
        const r = await team(invitee.page, action, body);
        expect(r.status, `fulfilment ${action} -> ${JSON.stringify(r.body)}`).toBe(403);
        expect(r.body.code).toBe("forbidden");
      }
      // direct URLs: the Team page offers nothing, the Billing page says it is not permitted
      await invitee.page.goto(`/${locale}/team?store=${storeId}`);
      await expect(invitee.page.getByTestId("team-not-owner")).toBeVisible();
      await expect(invitee.page.getByTestId("team-invite-email")).toHaveCount(0);
      await expect(invitee.page.getByTestId("team-members")).toHaveCount(0);
      await shot(invitee.page, "team-direct-url-fulfilment", locale, vp);
      await invitee.page.goto(`/${locale}/billing?store=${storeId}`);
      await expect(invitee.page.getByTestId("billing-page")).toContainText(billingForbidden[locale]);
      await shot(invitee.page, "billing-direct-url-fulfilment", locale, vp);

      // ---- owner changes the role to viewer in the UI; the invitee's next request has the new bundle -------------------------
      await owner.page.goto(`/${locale}/team?store=${storeId}`);
      const listed = await team(owner.page, "list", { store_id: storeId });
      const member = (listed.body.members as { principal_id: string; email: string; role: string }[]).find((m) => m.email === inviteeEmail);
      expect(member?.role).toBe("fulfilment");
      await expect(owner.page.getByTestId(`member-${member!.principal_id}`)).toContainText(inviteeEmail);
      const [roleResponse] = await Promise.all([
        owner.page.waitForResponse((r) => new URL(r.url()).pathname === "/api/team/set-role"),
        owner.page.getByTestId(`member-role-${member!.principal_id}`).selectOption("viewer"),
      ]);
      expect(roleResponse.status()).toBe(204);
      await expect(owner.page.getByTestId(`member-role-${member!.principal_id}`)).toHaveValue("viewer");
      await shot(owner.page, "team-owner-viewer", locale, vp);
      expect(await browserJSON(invitee.page, `/api/stores/${storeId}/order-actions`)).toEqual({
        status: 200, body: { refund: false, fulfillment_write: false, orders_export: false },
      });
      await invitee.page.goto(`/${locale}/orders?store=${storeId}`);
      await expect(invitee.page.getByTestId("orders-refresh")).toBeVisible();
      await expect(invitee.page.getByTestId("orders-export"), "viewer lost orders:export on the next request").toHaveCount(0);
      expect((await browserJSON(invitee.page, `/api/stores/${storeId}/orders`)).status, "viewer keeps orders:read").toBe(200);

      // ---- owner removes the member in the UI (with confirmation); the invitee's next request is refused ----------------------
      await owner.page.getByTestId(`member-remove-${member!.principal_id}`).click();
      await owner.page.getByTestId(`member-remove-yes-${member!.principal_id}`).waitFor();
      const [removeResponse] = await Promise.all([
        owner.page.waitForResponse((r) => new URL(r.url()).pathname === "/api/team/remove"),
        owner.page.getByTestId(`member-remove-yes-${member!.principal_id}`).click(),
      ]);
      expect(removeResponse.status()).toBe(204);
      await expect(owner.page.getByTestId(`member-${member!.principal_id}`)).toHaveCount(0);
      await shot(owner.page, "team-owner-removed", locale, vp);
      const afterStores = await browserJSON(invitee.page, "/api/stores");
      expect(afterStores.status === 401 || (afterStores.status === 200 && (afterStores.body as { items: unknown[] }).items.length === 0), `stores after removal: ${afterStores.status}`).toBe(true);
      for (const path of [`/api/stores/${storeId}/orders`, `/api/stores/${storeId}/order-actions`]) {
        const r = await browserJSON(invitee.page, path);
        expect([401, 403, 404], `${path} after removal -> ${r.status}`).toContain(r.status);
      }
      await invitee.page.goto(`/${locale}/orders?store=${storeId}`);
      await expect(invitee.page.getByTestId("orders-table")).toHaveCount(0);
      await expect(invitee.page.getByTestId("orders-export")).toHaveCount(0);
      await shot(invitee.page, "orders-after-removal", locale, vp);
      // the used link stays dead: same generic refusal, and the removal was not undone by it
      await invitee.page.goto(link!);
      await invitee.page.getByTestId("invite-accept").click();
      await expect(invitee.page.getByTestId("invite-problem")).toBeVisible();
      const afterReuse = await browserJSON(invitee.page, `/api/stores/${storeId}/orders`);
      expect([401, 403, 404], "re-using the link did not re-admit the member").toContain(afterReuse.status);

      // ---- token hygiene ---------------------------------------------------------------------------------------------------
      for (const [who, net] of [["owner", ownerNet], ["invitee", inviteeNet]] as const) {
        for (const r of net.requests) {
          const isTheLink = r.type === "document" && r.method === "GET" && new URL(r.url).pathname === url.pathname;
          if (!isTheLink) expect(r.url, `${who} request URL carries the token`).not.toContain(token);
          // Same-origin Referer on subresources is the separate @defect test below; nothing may ever leave the origin with it.
          if (new URL(r.url).origin !== publicOrigin) expect(r.referer ?? "", `${who} cross-origin referrer carries the token`).not.toContain(token);
        }
        expect(net.console.join("\n"), `${who} console carries the token`).not.toContain(token);
      }
      expect(inviteeNet.requests.some((r) => r.type === "document" && new URL(r.url).pathname === url.pathname), "the link navigation was observed").toBe(true);
    } finally {
      await owner.context.close();
      await invitee.context.close();
    }
  });
}

});

// ---- known gaps (each its own test: a failure here is a recorded DEFECT, not a harness error) ----------------------------------------
test.describe("known gaps", () => {
test.describe.configure({ mode: "default" });
// ---- role-aware navigation (brief: "sees orders but no Team/Billing nav") --------------------------------------------
test("@defect role-aware navigation: a fulfilment user is not offered Team or Billing", async ({ browser }) => {
  const locale: Locale = "zh-TW";
  const owner = await person(browser, "desktop");
  const invitee = await person(browser, "desktop");
  try {
    await signUp(owner.page, locale, newEmail(), newPassword(), 0);
    const storeId = await createStore(owner.page, locale);
    const inviteeEmail = newEmail();
    const r = await team(owner.page, "invite", { store_id: storeId, email: inviteeEmail, role: "fulfilment", locale });
    expect(r.status).toBe(201);
    const link = /https?:\/\/[^\s"<>]+\/invite\/[A-Za-z0-9_-]{43}/.exec((await nthMail(inviteeEmail, 1)).text)![0];
    remember(link.split("/").pop()!);
    await signUp(invitee.page, locale, inviteeEmail, newPassword(), 1);
    await invitee.page.goto(link);
    await invitee.page.getByTestId("invite-accept").click();
    await expect(invitee.page.getByTestId("invite-done")).toBeVisible();
    await invitee.page.goto(`/${locale}/orders?store=${storeId}`);
    await expect(invitee.page.getByTestId("nav-orders")).toBeVisible();
    await shot(invitee.page, "nav-fulfilment", locale, "desktop");
    await expect(navButton(invitee.page, navLabel.team[locale]), "Team nav entry for a non-owner").toHaveCount(0);
    await expect(navButton(invitee.page, navLabel.billing[locale]), "Billing nav entry for a non-owner").toHaveCount(0);
  } finally {
    await owner.context.close();
    await invitee.context.close();
  }
});

// The token in the emailed link is a bearer secret; the page promises Referrer-Policy no-referrer + noindex. A signed-out visit with
// any well-formed token is enough to observe what the page's own subresource requests send.
test("@defect invite link token stays out of Referer headers of the page's own requests", async ({ browser }) => {
  const visitor = await person(browser, "desktop");
  const net = watch(visitor.page);
  const token = remember(randomBytes(32).toString("base64url"));
  try {
    await visitor.page.goto(`/en/invite/${token}`);
    await expect(visitor.page.getByTestId("invite-need-login")).toBeVisible();
    await visitor.page.waitForLoadState("networkidle");
    expect(net.requests.length).toBeGreaterThan(2);
    const leaks = net.requests.filter((r) => (r.referer ?? "").includes(token)).map((r) => `${r.type} ${new URL(r.url).pathname}`);
    expect(leaks, "requests that carry the invitation URL in Referer").toEqual([]);
  } finally {
    await visitor.context.close();
  }
});
});
