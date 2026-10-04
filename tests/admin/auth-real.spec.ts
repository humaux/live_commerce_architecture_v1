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
  // G-UI8 audit [FIXTURE/SETUP]: API-contract helper (BFF CSRF/idempotency/shape): the subject is the HTTP contract, not a UI behaviour; UI flows of these routes are click-driven in entry/settings specs
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
  await page.getByLabel("Language", { exact: true }).selectOption("zh-TW");
  await expect(page).toHaveURL(/\/zh-TW\/?$/);
  await Promise.all([
    page.waitForURL(
      (url) => url.origin === publicOrigin && /^\/zh-TW\/?$/.test(url.pathname),
    ),
    page.getByRole("button", { name: "透過身分服務登入" }).click(),
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
  // G-UI8 audit [READ/MEASURE]: reads which cookie names are script-readable (HttpOnly check)
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

  await page.getByLabel("商戶名稱").fill("Browser Merchant");
  await page.getByRole("button", { name: "下一步：商店設定" }).click();
  await page.getByLabel("商店名稱").fill("Browser Store");
  await page.getByLabel("交易幣別").selectOption("TWD");
  await page.getByRole("button", { name: "下一步：庫存倉" }).click();
  await page.getByLabel("初始庫存倉名稱").fill("Browser Warehouse");
  const createdResponsePromise = page.waitForResponse(
    (response) =>
      new URL(response.url()).pathname === "/api/onboarding/initial-store",
  );
  await page.getByRole("button", { name: "建立內部工作區" }).click();
  const createdResponse = await createdResponsePromise;
  expect(createdResponse.status()).toBe(200);
  const originalReceipt = await createdResponse.json();
  await expect(page.getByRole("status")).toContainText("內部工作區已建立");
  const idempotencyKey =
    createdResponse.request().headers()["idempotency-key"] ?? "";
  expect(idempotencyKey).toMatch(/^[A-Za-z0-9_.:-]{8,128}$/);
  const initialBody = JSON.parse(
    createdResponse.request().postData() ?? "null",
  ) as {
    tenant_name: string;
    store_name: string;
    warehouse_name: string;
    currency: string;
  };
  expect(initialBody).toEqual({
    tenant_name: "Browser Merchant",
    store_name: "Browser Store",
    warehouse_name: "Browser Warehouse",
    currency: "TWD",
  });
  const writeOptions = {
    method: "POST",
    body: initialBody,
    csrf: true,
    headers: { "Idempotency-Key": idempotencyKey },
  };
  const replayed = await browserJSON(
    page,
    "/api/onboarding/initial-store",
    writeOptions,
  );
  expect(replayed.status).toBe(200);
  expect(replayed.body).toEqual(originalReceipt);
  const created = replayed.body as {
    tenant_id: string;
    store_id: string;
    warehouse_id: string;
  };
  expect(created.tenant_id).toMatch(uuid);
  expect(created.store_id).toMatch(uuid);
  expect(created.warehouse_id).toMatch(uuid);

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
          // staff-team (migration 0089, storefront-v2 §D): each item carries the caller's role and effective permissions; the
          // initial-store creator is a full owner = the whole store_grants_permission_check catalogue, sorted by the SQL definer;
          // pinned so a catalogue change is a deliberate edit here.
          role: "owner",
          permissions: [
            "ads:approve", "ads:manage", "ads:read", "audit:read", "audit:write", "billing:manage", "catalog:read",
            "catalog:write", "customers:privacy", "customers:read", "fulfillment:write", "integration:execute",
            "integration:manage", "integration:read", "inventory:read", "inventory:reserve", "inventory:write", "live:manage",
            "live:read", "orders:export", "orders:read", "payments:refund", "pricing:read", "pricing:write", "store:read",
          ],
        },
      ],
    },
  });
  await page.getByRole("button", { name: "進入工作區" }).click();
  // merchant-tools G1 (migration 0094): the workspace landing is the dashboard; the stock ledger moved to /inventory.
  await expect(page.getByRole("heading", { name: "總覽", level: 1 })).toBeVisible();

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

  // Real Chromium -> Next BFF -> Go -> PG. Synthetic credentials only. This
  // exercises transport, not the still separately gated settings UI/provider.
  const accountPath = `/api/stores/${created.store_id}/provider-accounts`;
  const accountBody = {
    provider: "payuni",
    environment: "SANDBOX",
    account_id: "browser_fixture_account",
    credentials: { hash_key: "K".repeat(32), hash_iv: "V".repeat(16) },
  };
  const accountOptions = {
    method: "POST",
    csrf: true,
    headers: { "Idempotency-Key": "browser-account-create-0001" },
    body: accountBody,
  };
  expect((await browserJSON(page, accountPath)).body).toEqual({
    items: [],
    next_cursor: "",
  });
  expect(
    (await browserJSON(page, accountPath, { ...accountOptions, csrf: false }))
      .status,
  ).toBe(403);
  expect(
    (await browserJSON(page, accountPath + "?unexpected=1", accountOptions))
      .status,
  ).toBe(422);
  expect(
    (await browserJSON(page, accountPath, { ...accountOptions, headers: {} }))
      .status,
  ).toBe(422);
  const account = await browserJSON(page, accountPath, {
    ...accountOptions,
    headers: {
      ...accountOptions.headers,
      Authorization: "Bearer attacker",
      "X-Tenant-ID": "attacker",
    },
  });
  expect(account.status).toBe(200);
  expect(account.body).toMatchObject({
    id: expect.stringMatching(uuid),
    provider: "payuni",
    environment: "SANDBOX",
    account_id: accountBody.account_id,
    credential_version: 1,
    state: "CONFIGURED_UNVERIFIED",
  });
  const safeAccount = (body: unknown) => {
    const text = JSON.stringify(body);
    for (const forbidden of [
      "key_id",
      "ciphertext",
      "nonce",
      accountBody.credentials.hash_key,
      accountBody.credentials.hash_iv,
      "N".repeat(32),
      "W".repeat(16),
    ]) {
      expect(text.includes(forbidden)).toBe(false);
    }
  };
  safeAccount(account.body);
  expect((await browserJSON(page, accountPath, accountOptions)).body).toEqual(
    account.body,
  );
  expect(
    (
      await browserJSON(page, accountPath, {
        ...accountOptions,
        body: {
          ...accountBody,
          credentials: { ...accountBody.credentials, hash_key: "Z".repeat(32) },
        },
      })
    ).status,
  ).toBe(409);
  const detailPath = `${accountPath}/${account.body.id}`;
  expect((await browserJSON(page, detailPath)).body).toEqual(account.body);
  const rotationOptions = {
    method: "POST",
    csrf: true,
    headers: { "Idempotency-Key": "browser-account-rotate-0001" },
    body: {
      expected_version: 1,
      credentials: { hash_key: "N".repeat(32), hash_iv: "W".repeat(16) },
    },
  };
  const rotated = await browserJSON(
    page,
    `${detailPath}/rotate`,
    rotationOptions,
  );
  expect(rotated.status).toBe(200);
  expect(rotated.body.credential_version).toBe(2);
  safeAccount(rotated.body);
  expect(
    (await browserJSON(page, `${detailPath}/rotate`, rotationOptions)).body,
  ).toEqual(rotated.body);
  expect(
    (
      await browserJSON(page, `${detailPath}/rotate`, {
        ...rotationOptions,
        headers: { "Idempotency-Key": "browser-account-stale-0001" },
      })
    ).status,
  ).toBe(409);
  expect((await browserJSON(page, accountPath + "?limit=1")).body).toEqual({
    items: [rotated.body],
    next_cursor: "",
  });
  expect((await browserJSON(page, accountPath + "?limit=101")).status).toBe(
    422,
  );
  expect(
    (
      await browserJSON(
        page,
        `/api/stores/00000000-0000-4000-8000-000000000000/provider-accounts`,
      )
    ).status,
  ).toBe(404);
  expect((await browserJSON(page, `${detailPath}/decrypt`)).status).toBe(404);
  expect(
    (await browserJSON(page, detailPath, { method: "DELETE", csrf: true }))
      .status,
  ).toBe(405);

  // Chromium accepts Secure cookies on explicit test loopback; Playwright's
  // HTTP client does not auto-send them there. Forward these exact issued test
  // cookies, first prove same-origin authority, then vary ONLY Origin.
  const issuedCookies = `${sessionName}=${sessionCookie.value}; ${csrfName}=${csrfCookie.value}`;
  // G-UI8 audit [FIXTURE/SETUP]: negative probe: a forged/hostile request no UI can send; the server, not the UI, must refuse (UI click paths of the same route are covered elsewhere) (forged Origin)
  const originControl = await context.request.post(accountPath, {
    headers: {
      Cookie: issuedCookies,
      Origin: publicOrigin,
      "X-CSRF-Token": csrfCookie.value,
      "Idempotency-Key": accountOptions.headers["Idempotency-Key"],
    },
    data: accountBody,
  });
  expect(originControl.status()).toBe(200);
  // G-UI8 audit [FIXTURE/SETUP]: negative probe: a forged/hostile request no UI can send; the server, not the UI, must refuse (UI click paths of the same route are covered elsewhere) (forged Origin)
  const badOrigin = await context.request.post(accountPath, {
    headers: {
      Cookie: issuedCookies,
      Origin: "https://attacker.invalid",
      "X-CSRF-Token": csrfCookie.value,
      "Idempotency-Key": "browser-account-origin-0001",
    },
    data: accountBody,
  });
  expect(badOrigin.status()).toBe(403);
  // G-UI8 audit [READ/MEASURE]: scans client storage for secrets/PII (read only)
  const storage = await page.evaluate(() => [
    JSON.stringify(localStorage),
    JSON.stringify(sessionStorage),
  ]);
  storage.forEach(safeAccount);

  const marketID = process.env.LC_BROWSER_MARKET_ID;
  expect(marketID).toMatch(uuid);
  const methodPath = `/api/stores/${created.store_id}/markets/${marketID}/countries/TW/payment-methods/payuni_credit`;
  const methodBody = {
    market_id: marketID,
    country: "TW",
    code: "payuni_credit",
    environment: "SANDBOX",
    connection_id: rotated.body.id,
    binding_version: rotated.body.binding_version,
    expected_version: 0,
    name_hans: "信用卡",
    name_hant: "信用卡",
    name_en: "Credit card",
    enabled: false,
    visible: true,
    sort_order: 10,
    min_amount_minor: 100,
    max_amount_minor: 100000,
  };
  const methodOptions = {
    method: "PUT",
    csrf: true,
    body: methodBody,
    headers: { "Idempotency-Key": "browser-method-create-0001" },
  };
  const savedMethod = await browserJSON(page, methodPath, methodOptions);
  expect(savedMethod.status).toBe(200);
  expect(savedMethod.body).toMatchObject({
    version: 1,
    enabled: false,
    visible: true,
    connection_id: rotated.body.id,
  });
  expect((await browserJSON(page, methodPath)).body).toEqual(savedMethod.body);
  expect((await browserJSON(page, methodPath, methodOptions)).body).toEqual(
    savedMethod.body,
  );
  const inspectOptions = {
    method: "POST",
    csrf: true,
    body: {
      market_id: marketID,
      country: "TW",
      code: "payuni_credit",
      environment: "SANDBOX",
      expected_version: 1,
      currency: "TWD",
      amount_minor: 1000,
    },
  };
  const inspected = await browserJSON(
    page,
    `${methodPath}/inspect`,
    inspectOptions,
  );
  expect(inspected.status).toBe(200);
  expect(inspected.body.available).toBe(false);
  expect(inspected.body.reasons.length).toBeGreaterThan(0);
  // Inspect is a read-only POST: CSRF required, mutation idempotency key absent.
  expect(
    (await browserJSON(page, `${methodPath}/inspect`, inspectOptions)).body,
  ).toEqual(inspected.body);
  expect(
    (
      await browserJSON(page, `${methodPath}/inspect`, {
        ...inspectOptions,
        csrf: false,
      })
    ).status,
  ).toBe(403);
  expect(
    (await browserJSON(page, `${methodPath}?unexpected=1`, methodOptions))
      .status,
  ).toBe(422);
  expect(
    (await browserJSON(page, methodPath, { ...methodOptions, headers: {} }))
      .status,
  ).toBe(422);
  const changedMethod = await browserJSON(page, methodPath, {
    ...methodOptions,
    body: { ...methodBody, expected_version: 1, visible: false },
    headers: { "Idempotency-Key": "browser-method-hide-0001" },
  });
  expect(changedMethod.status).toBe(200);
  expect(changedMethod.body).toMatchObject({
    version: 2,
    visible: false,
    enabled: false,
  });
  expect((await browserJSON(page, methodPath)).body).toEqual(
    changedMethod.body,
  );
  expect(
    (
      await browserJSON(page, methodPath, {
        ...methodOptions,
        body: { ...methodBody, expected_version: 2, enabled: true },
        headers: { "Idempotency-Key": "browser-method-enable-0001" },
      })
    ).status,
  ).toBe(409);

  const rateResponse = page.waitForResponse(
    (response) =>
      new URL(response.url()).pathname === accountPath &&
      response.status() === 429,
  );
  let limited = false;
  for (let attempt = 0; attempt < 61; attempt++) {
    const replay = await browserJSON(page, accountPath, accountOptions);
    if (replay.status === 429) {
      expect(replay.body).toMatchObject({
        code: "rate_limited",
        retryable: true,
      });
      limited = true;
      break;
    }
    expect(replay.status).toBe(200);
  }
  expect(limited).toBe(true);
  expect((await rateResponse).headers()["retry-after"]).toBe("60");
  expect((await browserJSON(page, detailPath)).body).toEqual(rotated.body);

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
