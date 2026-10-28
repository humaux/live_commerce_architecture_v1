// Purpose: OC-04 acceptance - the merchant admin re-auth dialog and 403 reauth_required handling (contracts/merchant-password-auth-v1.md
//   §16, §16.1, §11; contracts/tasks.json OC-04). Playwright suite `oc` (playwright.config.ts), started by tests/foundation/browser_oc_test.go
//   in `--browser-password-auth`: Chromium -> packaged Next admin BFF -> in-process Go (password, staff, admin API) -> real PG; codes over
//   the real SMTP adapter to the loopback fake. The OIDC IdP is MOCK (the Go harness answers the two OIDC re-auth calls).
// Depends on: the harness env (LC_BROWSER_PUBLIC_ORIGIN, LC_BROWSER_INSPECT_ORIGIN, LC_BROWSER_EVIDENCE_DIR) and its inspection
//   listener: GET /mail?to=, POST /age?email= (session authenticated 2 h ago).
// Used by: tests/foundation/browser_oc_test.go only. Written from the contract and the card, driven by data-testid="reauth-dialog"
//   (with data-step), the existing Team page hooks (team-invite-*) and the sign-up form's type/autocomplete attributes.
// Asserts: a stale session gets 403 reauth_required (never 503 retry_later) on every guarded BFF route file (team, catch-all stores
//   proxy, reports) and stays signed in; the dialog opens instead of a sign-out; password -> emailed code -> complete replays the
//   original request exactly once with the same Idempotency-Key and body; wrong password = 403 reauth_failed and still signed in;
//   owner role = 403 owner_role_disabled; the OIDC route returns via /api/auth/callback with no session Set-Cookie, the session cookie
//   unchanged and the originating page as Location; an OIDC account is offered the OIDC route; the mail_unavailable OIDC hint only for
//   an OIDC account (MOCK 503); zh-TW / zh-CN / en; focus trap, focus return, 44 px targets at 390 px.
import { createHmac, randomBytes } from "node:crypto";
import { appendFileSync, mkdirSync } from "node:fs";
import { join } from "node:path";
import { expect, test, type Browser, type BrowserContext, type Page } from "@playwright/test";

function requiredOrigin(name: string) {
  const value = process.env[name];
  if (!value) throw new Error(`${name} is required`);
  const url = new URL(value);
  if (url.origin !== value) throw new Error(`${name} must be an exact origin`);
  return url.origin;
}
const publicOrigin = requiredOrigin("LC_BROWSER_PUBLIC_ORIGIN");
const inspectOrigin = requiredOrigin("LC_BROWSER_INSPECT_ORIGIN");
const evidenceDir = process.env.LC_BROWSER_EVIDENCE_DIR;
if (!evidenceDir) throw new Error("LC_BROWSER_EVIDENCE_DIR is required");
mkdirSync(join(evidenceDir, "screens"), { recursive: true });

test.use({ baseURL: publicOrigin, trace: "off", screenshot: "only-on-failure" });
test.describe.configure({ timeout: 240_000 });

type Locale = "zh-TW" | "zh-CN" | "en";
const viewports = { desktop: { width: 1586, height: 992 }, mobile: { width: 390, height: 844 } } as const;
// Copy the card requires per locale (lib/reauth-copy.ts is the source; the spec pins the visible words).
const copy: Record<Locale, { title: string; oidc: string; useOidc: string; usePassword: string; mailUnavailable: string; oidcHint: string }> = {
  "zh-TW": { title: "確認是你本人", oidc: "透過身分提供者登入", useOidc: "改用身分提供者", usePassword: "改用密碼", mailUnavailable: "暫時無法寄送郵件", oidcHint: "你也可以改用身分提供者確認。" },
  "zh-CN": { title: "确认是你本人", oidc: "通过身份提供方登录", useOidc: "改用身份提供方", usePassword: "改用密码", mailUnavailable: "暂时无法发送邮件", oidcHint: "你也可以改用身份提供方确认。" },
  en: { title: "Confirm it's you", oidc: "Sign in with identity provider", useOidc: "Use identity provider instead", usePassword: "Use password instead", mailUnavailable: "We cannot send email right now", oidcHint: "You can confirm with your identity provider instead." },
};

// ---- sentinels, mailbox, helpers --------------------------------------------------------------------------------------------------
const remember = (value: string) => { appendFileSync(join(evidenceDir!, "canaries.txt"), value + "\n"); return value; };
const letters = (n: number) => Array.from(randomBytes(n), (b: number) => String.fromCharCode(97 + (b % 26))).join("");
const ownerEmail = () => remember(`oc4o.${letters(14)}@example.test`);
const inviteeEmail = () => remember(`oc4i.${letters(14)}@example.test`);
const newPassword = () => remember(`Oc4-${letters(22)}`);
const source = () => `10.${randomBytes(1)[0]}.${randomBytes(1)[0]}.${1 + (randomBytes(1)[0] % 250)}`;
const uuid = () => crypto.randomUUID();
const days = () => { const to = new Date(); const from = new Date(to.getTime() - 7 * 86_400_000); return { from: from.toISOString().slice(0, 10), to: to.toISOString().slice(0, 10) }; };

type Mail = { subject: string; text: string; html: string };
async function nthMail(to: string, n: number): Promise<Mail> {
  const deadline = Date.now() + 20_000;
  for (;;) {
    const all = (await (await fetch(`${inspectOrigin}/mail?to=${encodeURIComponent(to.toLowerCase())}`)).json()) as Mail[];
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
async function age(email: string) {
  const res = await fetch(`${inspectOrigin}/age?email=${encodeURIComponent(email)}`, { method: "POST" });
  expect(((await res.json()) as { sessions: number }).sessions, "sessions aged").toBeGreaterThanOrEqual(1);
}

async function person(browser: Browser, vp: keyof typeof viewports) {
  const context = await browser.newContext({ ignoreHTTPSErrors: true, baseURL: publicOrigin, viewport: viewports[vp] });
  await context.setExtraHTTPHeaders({ "x-forwarded-for": source() });
  return { context, page: await context.newPage() };
}

async function authCookies(context: BrowserContext) {
  // Match password-auth.spec.ts: an http URL filter omits Secure cookies accepted by Chromium on loopback.
  const host = new URL(publicOrigin).hostname;
  return (await context.cookies()).filter((cookie) => cookie.domain === host);
}

async function signUp(page: Page, locale: Locale, email: string, password: string) {
  await page.goto(`/${locale}/signup`);
  await page.locator('input[type="email"]').fill(email);
  await page.locator('input[type="password"]').fill(password);
  await page.locator('form button[type="submit"]').click();
  await expect(page.locator('input[autocomplete="one-time-code"]')).toBeVisible();
  await page.locator('input[autocomplete="one-time-code"]').fill(codeOf(await nthMail(email, 1)));
  const [response] = await Promise.all([
    page.waitForResponse((r) => new URL(r.url()).pathname === "/api/auth/password/verify"),
    page.locator('form button[type="submit"]').click(),
  ]);
  expect(response.status()).toBe(200);
}

// Raw BFF call through XMLHttpRequest: the dialog wraps window.fetch only, so these see the BFF's own answer (status + code).
async function xhr(page: Page, method: string, path: string, options: { body?: unknown; key?: string } = {}) {
  // G-UI8 audit [READ/MEASURE]: BFF protocol conformance (status/code per guarded route), not UI acceptance
  return page.evaluate(({ method, path, options }) => new Promise<{ status: number; code: string | null }>((resolve) => {
    const x = new XMLHttpRequest();
    x.open(method, path);
    const csrf = document.cookie.split(";").map((p) => p.trim()).filter((p) => p.startsWith("__Host-commerce_csrf=")).map((p) => p.slice(21));
    if (csrf.length === 1) x.setRequestHeader("X-CSRF-Token", csrf[0]);
    if (options.key) x.setRequestHeader("Idempotency-Key", options.key);
    if (options.body !== undefined) x.setRequestHeader("Content-Type", "application/json");
    x.onload = () => { let code: string | null = null; try { code = JSON.parse(x.responseText).code ?? null; } catch { /* CSV or empty */ } resolve({ status: x.status, code }); };
    x.onerror = () => resolve({ status: 0, code: null });
    x.send(options.body === undefined ? null : JSON.stringify(options.body));
  }), { method, path, options });
}
const signedIn = async (page: Page) => expect((await xhr(page, "GET", "/api/stores")).status, "still signed in").toBe(200);

async function createStore(page: Page, locale: Locale) {
  await page.goto(`/${locale}/`);
  const created = await page.evaluate(async () => {
    const csrf = document.cookie.split(";").map((p) => p.trim()).find((p) => p.startsWith("__Host-commerce_csrf="))!.slice(21);
    const r = await fetch("/api/onboarding/initial-store", { method: "POST", headers: { "Content-Type": "application/json", "X-CSRF-Token": csrf, "Idempotency-Key": `oc4-${crypto.randomUUID()}` },
      body: JSON.stringify({ tenant_name: "OC04 Merchant", store_name: "OC04 Store", warehouse_name: "OC04 Warehouse", currency: "TWD" }) });
    return { status: r.status, body: await r.json() };
  });
  expect(created.status, JSON.stringify(created.body)).toBe(200);
  return created.body.store_id as string;
}

const dialog = (page: Page) => page.getByTestId("reauth-dialog");
const modal = (page: Page) => page.locator("dialog[open]");

// The re-auth mail is throttled per email: email-mail-login allows 1 per aligned 60 s window (internal/identity/throttle.go winMailLogin).
// A second start for the same owner must therefore begin >= 61 s after the previous one; the production limit is left as is.
const lastStart = new Map<string, number>();

// Password route inside the open dialog: password -> Enter -> emailed code (mail n) -> Enter.
async function passwordReauth(page: Page, email: string, password: string, mailIndex: number) {
  await expect(dialog(page)).toHaveAttribute("data-step", "password");
  const field = dialog(page).locator('input[type="password"]');
  await expect(field).toBeFocused();
  await field.fill(password);
  const wait = (lastStart.get(email) ?? 0) + 61_000 - Date.now();
  if (wait > 0) await page.waitForTimeout(wait);
  lastStart.set(email, Date.now());
  await field.press("Enter");
  await expect(dialog(page)).toHaveAttribute("data-step", "code");
  const codeField = dialog(page).locator('input[autocomplete="one-time-code"]');
  await codeField.fill(codeOf(await nthMail(email, mailIndex)));
  await codeField.press("Enter");
  await expect(modal(page)).toHaveCount(0);
}

// One invite from the Team page while stale: dialog (not a sign-out), password re-auth, replayed exactly once with the same body.
async function inviteThroughDialog(page: Page, locale: Locale, store: string, owner: string, password: string, mailIndex: number) {
  const invitee = inviteeEmail();
  const sent: { body: string | null }[] = [];
  page.on("request", (r) => { if (new URL(r.url()).pathname === "/api/team/invite") sent.push({ body: r.postData() }); });
  await page.goto(`/${locale}/team?store=${store}`);
  await expect(page.getByTestId("team-invite-email")).toBeVisible();
  await age(owner);
  await page.getByTestId("team-invite-email").fill(invitee);
  await page.getByTestId("team-invite-role").selectOption("viewer");
  await page.getByTestId("team-invite-send").click();
  await expect(dialog(page)).toBeVisible();
  await expect(modal(page)).toContainText(copy[locale].title);
  expect(new URL(page.url()).pathname, "no sign-out navigation").toBe(`/${locale}/team`);
  expect(sent).toHaveLength(1);
  await signedIn(page);
  return { invitee, sent, finish: async () => {
    await passwordReauth(page, owner, password, mailIndex);
    await expect(page.getByTestId("team-invites")).toContainText(invitee);
    expect(sent, "the refused invite is replayed exactly once").toHaveLength(2);
    expect(sent[1].body, "the replay sends the original bytes").toBe(sent[0].body);
    await expect(page.getByTestId("team-invite-send"), "focus returns to the control that started the request").toBeFocused();
  } };
}

test("OC-04 zh-TW desktop: guarded BFF codes, password re-auth with exact replay, OIDC re-auth, routing and hints", async ({ browser }) => {
  const locale: Locale = "zh-TW";
  const c = copy[locale];
  const owner = ownerEmail(), password = newPassword();
  const { context, page } = await person(browser, "desktop");
  await signUp(page, locale, owner, password);
  const store = await createStore(page, locale);
  const { from, to } = days();

  // 1. Stale session: every guarded BFF route file answers 403 reauth_required (never 503 retry_later); the session stays.
  await age(owner);
  const base = `/api/stores/${store}`;
  const key = () => `oc4-${letters(16)}`;
  const guarded: [string, string, string, { body?: unknown; key?: string }][] = [
    ["team/[action] invite", "POST", "/api/team/invite", { body: { store_id: store, email: remember(`oc4x.${letters(14)}@example.test`), role: "viewer", locale } }],
    ["stores catch-all orders/unshipped.csv", "GET", `${base}/orders/unshipped.csv`, {}],
    ["stores catch-all finance/summary.csv", "GET", `${base}/finance/summary.csv?from=${from}&to=${to}`, {}],
    ["stores catch-all customers erasure", "POST", `${base}/customers/${uuid()}/erasure`, { key: key(), body: { confirm: "ERASE" } }],
    ["stores catch-all bank-transfer confirm", "POST", `${base}/orders/${uuid()}/bank-transfer/confirm`, { key: key(), body: {} }],
    ["reports/[report] products.csv", "GET", `${base}/reports/products.csv?from=${from}&to=${to}`, {}],
  ];
  for (const [name, method, path, options] of guarded) {
    expect(await xhr(page, method, path, options), name).toEqual({ status: 403, code: "reauth_required" });
  }
  await signedIn(page);

  // 2. Wrong password on the re-auth route: 403 reauth_failed, still signed in (never 401).
  expect(await xhr(page, "POST", "/api/auth/password/reauth/start", { body: { password: remember(`Wrong-${letters(20)}`), locale } }))
    .toEqual({ status: 403, code: "reauth_failed" });
  await signedIn(page);

  // 3. UI: a refused invite opens the dialog; focus is trapped; password -> code -> the invite is replayed once.
  const chain = await inviteThroughDialog(page, locale, store, owner, password, 2);
  for (let i = 0; i < 6; i++) {
    await page.keyboard.press("Tab");
    expect(await page.evaluate(() => !!document.activeElement?.closest("dialog[open]")), "Tab stays inside the dialog").toBe(true);
  }
  await dialog(page).locator('input[type="password"]').focus();
  await chain.finish();

  // 4. Fresh session: the owner role is refused as owner_role_disabled (passed through, not 503).
  expect(await xhr(page, "POST", "/api/team/invite", { body: { store_id: store, email: remember(`oc4x.${letters(14)}@example.test`), role: "owner", locale } }))
    .toEqual({ status: 403, code: "owner_role_disabled" });

  // 5. Keyed command: the replay carries the ORIGINAL Idempotency-Key and body, exactly once.
  await age(owner);
  const erasure = `${base}/customers/${uuid()}/erasure`;
  const idem = key();
  const keyed: { key: string | undefined; body: string | null }[] = [];
  page.on("request", (r) => { if (new URL(r.url()).pathname === erasure) keyed.push({ key: r.headers()["idempotency-key"], body: r.postData() }); });
  await page.evaluate(({ erasure, idem }) => {
    const csrf = document.cookie.split(";").map((p) => p.trim()).find((p) => p.startsWith("__Host-commerce_csrf="))!.slice(21);
    (window as unknown as { oc4: Promise<number> }).oc4 = fetch(erasure, { method: "POST", headers: { "Content-Type": "application/json", "X-CSRF-Token": csrf, "Idempotency-Key": idem },
      body: JSON.stringify({ confirm: "ERASE" }), signal: AbortSignal.timeout(8000) }).then((r) => r.status);
  }, { erasure, idem });
  await expect(dialog(page)).toBeVisible();
  await page.waitForTimeout(8500); // the caller's own 8 s timeout runs out while the person re-authenticates: the replay still goes
  await passwordReauth(page, owner, password, 3);
  const replayed = await page.evaluate(() => (window as unknown as { oc4: Promise<number> }).oc4);
  expect(replayed, "the caller receives the replay's answer, not the 403").not.toBe(403);
  expect(keyed).toHaveLength(2);
  expect(keyed[1]).toEqual({ key: idem, body: keyed[0].body });

  // 6. OIDC re-auth (MOCK IdP) from the password dialog: no session Set-Cookie, cookie unchanged, back to the opening page.
  await age(owner);
  const beforeCookies = await authCookies(context);
  expect(beforeCookies.filter((k) => k.name === "__Host-commerce_session"), "session cookie exists before OIDC re-auth").toHaveLength(1);
  const before = beforeCookies.filter((k) => k.name.startsWith("__Host-commerce_")).map((k) => `${k.name}=${k.value}`).sort();
  const callbacks: { status: number; location: string; cookies: string }[] = [];
  context.on("response", async (r) => {
    if (new URL(r.url()).pathname !== "/api/auth/callback") return;
    callbacks.push({ status: r.status(), location: (await r.headerValue("location")) ?? "", cookies: (await r.headerValue("set-cookie")) ?? "" });
  });
  await page.evaluate(({ path }) => {
    (window as unknown as { oc4csv: Promise<number> }).oc4csv = fetch(path).then((r) => r.status);
  }, { path: `${base}/finance/summary.csv?from=${from}&to=${to}` });
  await expect(dialog(page)).toHaveAttribute("data-step", "password");
  await dialog(page).getByRole("button", { name: c.useOidc }).click();
  await expect(dialog(page)).toHaveAttribute("data-step", "oidc");
  const popupOpened = context.waitForEvent("page");
  await modal(page).getByRole("button", { name: c.oidc }).click();
  await popupOpened;
  await expect(modal(page)).toHaveCount(0, { timeout: 20_000 });
  expect(await page.evaluate(() => (window as unknown as { oc4csv: Promise<number> }).oc4csv), "replayed export after OIDC re-auth").toBe(200);
  expect(callbacks).toHaveLength(1);
  expect(callbacks[0].status).toBe(303);
  expect(callbacks[0].location, "returns to the page that opened the dialog").toBe(`/${locale}/team?store=${store}#reauth-done`);
  expect(callbacks[0].cookies, "a re-auth callback sets no session cookie").not.toContain("__Host-commerce_session");
  expect(callbacks[0].cookies).not.toContain("__Host-commerce_csrf");
  const afterCookies = await authCookies(context);
  expect(afterCookies.filter((k) => k.name === "__Host-commerce_session"), "session cookie exists after OIDC re-auth").toHaveLength(1);
  const after = afterCookies.filter((k) => k.name.startsWith("__Host-commerce_") && k.name !== "__Host-commerce_login").map((k) => `${k.name}=${k.value}`).sort();
  expect(after, "session and CSRF cookies unchanged by the OIDC re-auth").toEqual(before.filter((k) => !k.startsWith("__Host-commerce_login=")));
  await signedIn(page);

  // 7. Routing: a session the OIDC callback issued (marker = HMAC-SHA256(fixed label, session token), lib/auth.ts) gets the OIDC route; any other marker does not.
  const session = afterCookies.find((k) => k.name === "__Host-commerce_session")!.value;
  // FIXTURE/SETUP: use the existing admin specs' HTTPS cookie URL for a Secure __Host- marker, including Chromium's HTTP loopback front.
  const cookieURL = publicOrigin.replace(/^http:/, "https:");
  const mark = async (value: string) => context.addCookies([{ name: "__Host-commerce_oidc_session", value, url: cookieURL, secure: true, httpOnly: true, sameSite: "Lax" }]);
  const method = async () => page.evaluate(async () => (await (await fetch("/api/auth/password/reauth/method")).json()) as { account: string });
  await mark(createHmac("sha256", "commerce-oidc-session-v1").update(session).digest("base64url"));
  expect((await method()).account).toBe("oidc");
  await mark(createHmac("sha256", "commerce-oidc-session-v1").update(session + "x").digest("base64url"));
  expect((await method()).account).toBe("password");

  // 8. mail_unavailable copy (MOCK 503 from the start step): the OIDC hint only for an account with an OIDC identity; then Cancel
  //    hands the caller the original 403 and the person is still signed in.
  await page.route("**/api/auth/password/reauth/start", (route) => route.fulfill({ status: 503, contentType: "application/json", body: JSON.stringify({ code: "mail_unavailable" }) }));
  for (const oidcAccount of [true, false]) {
    await mark(createHmac("sha256", "commerce-oidc-session-v1").update(oidcAccount ? session : session + "x").digest("base64url"));
    await age(owner);
    await page.evaluate(({ path }) => { (window as unknown as { oc4c: Promise<number> }).oc4c = fetch(path).then((r) => r.status); }, { path: `${base}/orders/unshipped.csv` });
    await expect(dialog(page)).toHaveAttribute("data-step", oidcAccount ? "oidc" : "password");
    if (oidcAccount) await dialog(page).getByRole("button", { name: c.usePassword }).click();
    await dialog(page).locator('input[type="password"]').fill(password);
    await dialog(page).locator('input[type="password"]').press("Enter");
    await expect(modal(page)).toContainText(c.mailUnavailable);
    if (oidcAccount) await expect(modal(page)).toContainText(c.oidcHint);
    else await expect(modal(page)).not.toContainText(c.oidcHint);
    await page.keyboard.press("Escape");
    await expect(modal(page)).toHaveCount(0);
    expect(await page.evaluate(() => (window as unknown as { oc4c: Promise<number> }).oc4c), "cancel returns the original 403").toBe(403);
    await signedIn(page);
  }
  await page.unroute("**/api/auth/password/reauth/start");
  await context.close();
});

for (const [locale, vp] of [["zh-CN", "desktop"], ["en", "mobile"]] as const) {
  test(`OC-04 ${locale} ${vp}: the refused invite opens the localized dialog and is replayed once after password re-auth`, async ({ browser }) => {
    const owner = ownerEmail(), password = newPassword();
    const { context, page } = await person(browser, vp);
    await signUp(page, locale, owner, password);
    const store = await createStore(page, locale);
    const chain = await inviteThroughDialog(page, locale, store, owner, password, 2);
    if (vp === "mobile") {
      // 44 px touch targets at 390 px (DS controls switch to --ds-target below 768 px).
      for (const target of await modal(page).locator("button:visible, input:visible").all()) {
        expect((await target.boundingBox())!.height, "touch target height").toBeGreaterThanOrEqual(44);
      }
      expect(await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth), "no horizontal page scroll").toBeLessThanOrEqual(0);
    }
    await page.screenshot({ path: join(evidenceDir!, "screens", `reauth-${locale}-${vp}.png`) });
    await chain.finish();
    await context.close();
  });
}
