import { expect, test } from "@playwright/test";

function requiredOrigin(name: string) {
  const value = process.env[name];
  if (!value) throw new Error(`${name} is required`);
  const url = new URL(value);
  if (
    url.origin !== value ||
    (url.protocol !== "http:" && url.protocol !== "https:")
  )
    throw new Error(`${name} must be an exact HTTP(S) origin`);
  return url.origin;
}

const publicOrigin = requiredOrigin("LC_BROWSER_PUBLIC_ORIGIN");
const apiOrigin = requiredOrigin("LC_BROWSER_API_ORIGIN");
const issuerValue = process.env.LC_BROWSER_ISSUER;
if (!issuerValue) throw new Error("LC_BROWSER_ISSUER is required");
const issuerURL = new URL(issuerValue);
if (
  (issuerURL.protocol !== "http:" && issuerURL.protocol !== "https:") ||
  issuerURL.username ||
  issuerURL.password ||
  issuerURL.search ||
  issuerURL.hash
)
  throw new Error("LC_BROWSER_ISSUER must be a safe HTTP(S) URL");
const issuerOrigin = issuerURL.origin;

test.use({
  baseURL: publicOrigin,
  trace: "off",
  screenshot: "only-on-failure",
});
test.describe.configure({ timeout: 60_000 });

const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;
const sessionName = "__Host-commerce_session";
const csrfName = "__Host-commerce_csrf";

test("REAL_PG signed IdP login, first store, authorization and logout", async ({
  browser,
  context,
  page,
}) => {
  let callbackURL = "";
  let sawIssuer = false;
  page.on("request", (request) => {
    const url = new URL(request.url());
    if (url.origin === issuerOrigin) sawIssuer = true;
    if (url.origin === publicOrigin && url.pathname === "/api/auth/callback")
      callbackURL = url.toString();
  });

  await page.goto(`${publicOrigin}/en/`);
  await Promise.all([
    page.waitForURL(
      (url) => url.origin === publicOrigin && /^\/zh-TW\/?$/.test(url.pathname),
    ),
    page.evaluate(() => {
      const form = document.createElement("form");
      form.method = "post";
      form.action = "/api/auth/login";
      const locale = document.createElement("input");
      locale.name = "locale";
      locale.value = "zh-TW";
      form.append(locale);
      document.body.append(form);
      form.submit();
    }),
  ]);
  if (!sawIssuer || !callbackURL)
    throw new Error("OIDC browser redirects were not observed");
  expect(new URL(page.url()).pathname).toMatch(/^\/zh-TW\/?$/);

  const authority = (await context.cookies(publicOrigin)).filter((cookie) =>
    [sessionName, csrfName].includes(cookie.name),
  );
  const metadata = authority
    .map(({ name, httpOnly, secure, sameSite, path }) => ({
      name,
      httpOnly,
      secure,
      sameSite,
      path,
    }))
    .sort((left, right) => left.name.localeCompare(right.name));
  expect(metadata).toEqual([
    {
      name: csrfName,
      httpOnly: false,
      secure: true,
      sameSite: "Lax",
      path: "/",
    },
    {
      name: sessionName,
      httpOnly: true,
      secure: true,
      sameSite: "Lax",
      path: "/",
    },
  ]);
  expect(
    (await context.cookies(publicOrigin)).some(
      (cookie) => cookie.name === "__Host-commerce_login",
    ),
  ).toBe(false);
  const session = authority.find((cookie) => cookie.name === sessionName);
  const csrf = authority.find((cookie) => cookie.name === csrfName);
  if (!session?.value || !csrf?.value)
    throw new Error("merchant authority cookies were not issued");
  const readableCookieNames = await page.evaluate(() =>
    document.cookie
      .split(";")
      .map((part) => part.trim().split("=", 1)[0])
      .filter(Boolean),
  );
  expect(readableCookieNames).toContain(csrfName);
  expect(readableCookieNames).not.toContain(sessionName);
  expect(session.expires).toBeGreaterThan(Date.now() / 1000);
  expect(session.expires).toBeLessThanOrEqual(Date.now() / 1000 + 86_400);
  expect(Math.abs(session.expires - csrf.expires)).toBeLessThanOrEqual(1);

  const replayContext = await browser.newContext({ baseURL: publicOrigin });
  const replayPage = await replayContext.newPage();
  await replayPage.goto(callbackURL);
  const replayURL = new URL(replayPage.url());
  expect(replayURL.origin).toBe(publicOrigin);
  expect(replayURL.pathname).toMatch(/^\/zh-CN\/?$/);
  expect(replayURL.searchParams.get("auth")).toBe("failed");
  expect(
    (await replayContext.cookies(publicOrigin)).some(
      (cookie) => cookie.name === sessionName,
    ),
  ).toBe(false);
  await replayContext.close();

  const emptyStores = await page.request.get(`${publicOrigin}/api/stores`);
  expect(emptyStores.status()).toBe(200);
  expect(await emptyStores.json()).toEqual({ items: [] });

  const idempotencyKey = "browser-onboard-0001";
  const initialBody = {
    tenant_name: "Browser Merchant",
    store_name: "Browser Store",
    warehouse_name: "Browser Warehouse",
    currency: "TWD",
  };
  const writeHeaders = {
    Origin: publicOrigin,
    "Content-Type": "application/json",
    "Idempotency-Key": idempotencyKey,
    "X-CSRF-Token": csrf.value,
  };
  const createdResponse = await page.request.post(
    `${publicOrigin}/api/onboarding/initial-store`,
    { headers: writeHeaders, data: initialBody },
  );
  expect(createdResponse.status()).toBe(200);
  const created = (await createdResponse.json()) as {
    tenant_id: string;
    store_id: string;
    warehouse_id: string;
  };
  expect(created.tenant_id).toMatch(uuid);
  expect(created.store_id).toMatch(uuid);
  expect(created.warehouse_id).toMatch(uuid);

  const replayed = await page.request.post(
    `${publicOrigin}/api/onboarding/initial-store`,
    { headers: writeHeaders, data: initialBody },
  );
  expect(replayed.status()).toBe(200);
  expect(await replayed.json()).toEqual(created);

  const changed = await page.request.post(
    `${publicOrigin}/api/onboarding/initial-store`,
    {
      headers: writeHeaders,
      data: { ...initialBody, store_name: "Changed Store" },
    },
  );
  expect(changed.status()).toBe(409);

  const stores = await page.request.get(`${publicOrigin}/api/stores`);
  expect(stores.status()).toBe(200);
  expect(await stores.json()).toEqual({
    items: [
      { id: created.store_id, name: initialBody.store_name, currency: "TWD" },
    ],
  });

  const warehouses = await page.request.get(
    `${publicOrigin}/api/stores/${created.store_id}/warehouses`,
  );
  expect(warehouses.status()).toBe(200);
  const warehousePage = (await warehouses.json()) as {
    items: Array<{ id: string; name: string }>;
    next_cursor: string;
  };
  expect(warehousePage).toEqual({
    items: [{ id: created.warehouse_id, name: initialBody.warehouse_name }],
    next_cursor: "",
  });

  const ledgerQuery = new URLSearchParams({
    warehouse_id: created.warehouse_id,
    q: "",
    status: "all",
    cursor: "",
  });
  const ledger = await page.request.get(
    `${publicOrigin}/api/stores/${created.store_id}/catalog-ledger?${ledgerQuery}`,
  );
  expect(ledger.status()).toBe(200);
  expect(await ledger.json()).toEqual({ items: [], next_cursor: "" });

  const rejectedCSRF = await page.request.post(
    `${publicOrigin}/api/stores/${created.store_id}/warehouses`,
    {
      headers: {
        Origin: publicOrigin,
        "Content-Type": "application/json",
        "Idempotency-Key": "browser-warehouse-0001",
      },
      data: { name: "Must Not Exist" },
    },
  );
  expect(rejectedCSRF.status()).toBe(403);
  const unchangedWarehouses = await page.request.get(
    `${publicOrigin}/api/stores/${created.store_id}/warehouses`,
  );
  expect(await unchangedWarehouses.json()).toEqual(warehousePage);

  const foreign = await page.request.get(
    `${publicOrigin}/api/stores/00000000-0000-4000-8000-000000000000/warehouses`,
  );
  expect(foreign.status()).toBe(404);

  const logout = await page.request.post(`${publicOrigin}/api/auth/logout`, {
    headers: {
      Origin: publicOrigin,
      "Content-Type": "application/json",
      "X-CSRF-Token": csrf.value,
    },
    data: {},
  });
  expect(logout.status()).toBe(204);
  expect(
    (await context.cookies(publicOrigin)).some((cookie) =>
      [sessionName, csrfName].includes(cookie.name),
    ),
  ).toBe(false);

  const stale = await page.request.get(`${publicOrigin}/api/stores`, {
    headers: { Cookie: `${sessionName}=${session.value}` },
  });
  expect(stale.status()).toBe(401);

  // Required above so the harness cannot omit its real Go endpoint while a
  // stale Next process happens to be listening; no API secret reaches browser code.
  void apiOrigin;
});
