import {
  expect,
  request as playwrightRequest,
  test,
  type Page,
} from "@playwright/test";

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
const publicHostname = new URL(publicOrigin).hostname;
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

async function browserJSON(
  page: Page,
  path: string,
  options: {
    method?: string;
    body?: unknown;
    csrf?: boolean;
    headers?: Record<string, string>;
  } = {},
) {
  return page.evaluate(
    async ({ path, options, csrfName }) => {
      const headers = new Headers(options.headers);
      if (options.body !== undefined)
        headers.set("Content-Type", "application/json");
      if (options.csrf) {
        const values = document.cookie
          .split(";")
          .map((part) => part.trim())
          .filter((part) => part.startsWith(`${csrfName}=`))
          .map((part) => part.slice(csrfName.length + 1));
        if (values.length !== 1 || !/^[A-Za-z0-9_-]{43}$/.test(values[0]))
          throw new Error("exact CSRF cookie is unavailable");
        headers.set("X-CSRF-Token", values[0]);
      }
      const response = await fetch(path, {
        method: options.method ?? "GET",
        headers,
        body:
          options.body === undefined ? undefined : JSON.stringify(options.body),
        credentials: "same-origin",
      });
      const text = await response.text();
      return { status: response.status, body: text ? JSON.parse(text) : null };
    },
    { path, options, csrfName },
  );
}

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

  const authority = (await context.cookies()).filter(
    (cookie) =>
      cookie.domain === publicHostname &&
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
    (await context.cookies()).some(
      (cookie) =>
        cookie.domain === publicHostname &&
        cookie.name === "__Host-commerce_login",
    ),
  ).toBe(false);
  const sessionCookie = authority.find((cookie) => cookie.name === sessionName);
  const csrfCookie = authority.find((cookie) => cookie.name === csrfName);
  if (!sessionCookie?.value || !csrfCookie)
    throw new Error("merchant authority cookies were not issued");
  const readableCookieNames = await page.evaluate(() =>
    document.cookie
      .split(";")
      .map((part) => part.trim().split("=", 1)[0])
      .filter(Boolean),
  );
  expect(readableCookieNames).toContain(csrfName);
  expect(readableCookieNames).not.toContain(sessionName);
  expect(sessionCookie.expires).toBeGreaterThan(Date.now() / 1000);
  expect(sessionCookie.expires).toBeLessThanOrEqual(Date.now() / 1000 + 86_400);
  expect(
    Math.abs(sessionCookie.expires - csrfCookie.expires),
  ).toBeLessThanOrEqual(1);

  const replayContext = await browser.newContext({ baseURL: publicOrigin });
  const replayPage = await replayContext.newPage();
  await replayPage.goto(callbackURL);
  const replayURL = new URL(replayPage.url());
  expect(replayURL.origin).toBe(publicOrigin);
  expect(replayURL.pathname).toMatch(/^\/zh-CN\/?$/);
  expect(replayURL.searchParams.get("auth")).toBe("failed");
  expect(
    (await replayContext.cookies()).some(
      (cookie) =>
        cookie.domain === publicHostname && cookie.name === sessionName,
    ),
  ).toBe(false);
  await replayContext.close();

  const emptyStores = await browserJSON(page, "/api/stores");
  expect(emptyStores).toEqual({ status: 200, body: { items: [] } });

  const idempotencyKey = "browser-onboard-0001";
  const initialBody = {
    tenant_name: "Browser Merchant",
    store_name: "Browser Store",
    warehouse_name: "Browser Warehouse",
    currency: "TWD",
  };
  const writeOptions = {
    method: "POST",
    body: initialBody,
    csrf: true,
    headers: { "Idempotency-Key": idempotencyKey },
  };
  const createdResponse = await browserJSON(
    page,
    "/api/onboarding/initial-store",
    writeOptions,
  );
  expect(createdResponse.status).toBe(200);
  const created = createdResponse.body as {
    tenant_id: string;
    store_id: string;
    warehouse_id: string;
  };
  expect(created.tenant_id).toMatch(uuid);
  expect(created.store_id).toMatch(uuid);
  expect(created.warehouse_id).toMatch(uuid);

  const replayed = await browserJSON(
    page,
    "/api/onboarding/initial-store",
    writeOptions,
  );
  expect(replayed).toEqual({ status: 200, body: created });

  const changed = await browserJSON(page, "/api/onboarding/initial-store", {
    ...writeOptions,
    body: { ...initialBody, store_name: "Changed Store" },
  });
  expect(changed.status).toBe(409);

  const stores = await browserJSON(page, "/api/stores");
  expect(stores).toEqual({
    status: 200,
    body: {
      items: [
        {
          id: created.store_id,
          name: initialBody.store_name,
          currency: "TWD",
        },
      ],
    },
  });

  const warehouses = await browserJSON(
    page,
    `/api/stores/${created.store_id}/warehouses`,
  );
  expect(warehouses.status).toBe(200);
  const warehousePage = warehouses.body as {
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
  const ledger = await browserJSON(
    page,
    `/api/stores/${created.store_id}/catalog-ledger?${ledgerQuery}`,
  );
  expect(ledger).toEqual({
    status: 200,
    body: { items: [], next_cursor: "" },
  });

  const rejectedCSRF = await browserJSON(
    page,
    `/api/stores/${created.store_id}/warehouses`,
    {
      method: "POST",
      headers: {
        "Idempotency-Key": "browser-warehouse-0001",
      },
      body: { name: "Must Not Exist" },
    },
  );
  expect(rejectedCSRF.status).toBe(403);
  const unchangedWarehouses = await browserJSON(
    page,
    `/api/stores/${created.store_id}/warehouses`,
  );
  expect(unchangedWarehouses.body).toEqual(warehousePage);

  const foreign = await browserJSON(
    page,
    "/api/stores/00000000-0000-4000-8000-000000000000/warehouses",
  );
  expect(foreign.status).toBe(404);

  const logout = await browserJSON(page, "/api/auth/logout", {
    method: "POST",
    body: {},
    csrf: true,
  });
  expect(logout).toEqual({ status: 204, body: null });
  expect(
    (await context.cookies()).some(
      (cookie) =>
        cookie.domain === publicHostname &&
        [sessionName, csrfName].includes(cookie.name),
    ),
  ).toBe(false);

  const staleContext = await playwrightRequest.newContext({
    baseURL: publicOrigin,
    extraHTTPHeaders: { Cookie: `${sessionName}=${sessionCookie.value}` },
  });
  const stale = await staleContext.get("/api/stores");
  expect(stale.status()).toBe(401);
  await staleContext.dispose();

  // Required above so the harness cannot omit its real Go endpoint while a
  // stale Next process happens to be listening; no API secret reaches browser code.
  void apiOrigin;
});
