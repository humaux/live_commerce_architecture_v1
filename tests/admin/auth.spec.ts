import { expect, test } from "@playwright/test";
import {
  createServer,
  request as httpRequest,
  type IncomingMessage,
} from "node:http";

const apiOrigin = "http://127.0.0.1:19111";
const publicOrigin = "http://127.0.0.1:3100";
const issuer = "https://provider.example/realm/";
const bffKey = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA";
const binding = `${"B".repeat(42)}A`;
const session = `${"C".repeat(42)}A`;
const storeID = "11111111-1111-4111-8111-111111111111";
const requests: Array<{
  path: string;
  method: string;
  headers: IncomingMessage["headers"];
  body: string;
}> = [];
let failLogout = true;

const server = createServer(async (request, response) => {
  let body = "";
  for await (const chunk of request) body += chunk;
  requests.push({
    path: request.url ?? "",
    method: request.method ?? "",
    headers: request.headers,
    body,
  });
  response.setHeader("Content-Type", "application/json");
  response.setHeader("Cache-Control", "no-store");
  const identity = request.url?.startsWith("/v1/identity/");
  if (identity) {
    if (
      request.headers["x-commerce-bff-key"] !== bffKey ||
      request.headers.origin ||
      request.headers.cookie
    ) {
      response.writeHead(403).end(JSON.stringify({ code: "forbidden" }));
      return;
    }
    if (request.url === "/v1/identity/login/start") {
      response.end(
        JSON.stringify({
          authorization_url:
            "https://provider.example/authorize?state=state-one",
          binding,
          expires_at: new Date(Date.now() + 300_000).toISOString(),
        }),
      );
      return;
    }
    if (request.url === "/v1/identity/login/complete") {
      const input = JSON.parse(body);
      if (
        input.binding !== binding ||
        input.state !== "state-one" ||
        input.code !== "code-one"
      ) {
        response.writeHead(401).end(JSON.stringify({ code: "unauthorized" }));
        return;
      }
      response.end(
        JSON.stringify({
          token: session,
          expires_at: new Date(Date.now() + 3_600_000).toISOString(),
        }),
      );
      return;
    }
    if (request.url === "/v1/identity/logout") {
      if (failLogout)
        response.writeHead(503).end(JSON.stringify({ code: "retry_later" }));
      else response.writeHead(204).end();
      return;
    }
    if (request.url === "/v1/identity/initial-store") {
      response.end(
        JSON.stringify({
          tenant_id: "22222222-2222-4222-8222-222222222222",
          store_id: storeID,
          warehouse_id: "33333333-3333-4333-8333-333333333333",
          handle: "mock-store",
          storefront_origin: "https://mock-store.xgdwm.com",
        }),
      );
      return;
    }
  }
  if (
    request.headers.authorization !== `Bearer ${session}` ||
    request.headers["x-commerce-bff-key"]
  ) {
    response.writeHead(401).end(JSON.stringify({ code: "unauthorized" }));
    return;
  }
  if (request.url === "/v1/admin/stores") {
    response.end(
      JSON.stringify({
        items: [{ id: storeID, name: "Mock store", currency: "TWD" }],
      }),
    );
    return;
  }
  if (request.url === `/v1/admin/stores/${storeID}/inventory/adjustments`) {
    response.end(JSON.stringify({ ok: true }));
    return;
  }
  if (request.url === `/v1/admin/stores/${storeID}/warehouses`) {
    response.setHeader("Content-Type", "text/plain");
    response.writeHead(503).end("private diagnostic must not escape");
    return;
  }
  if (request.url?.startsWith(`/v1/admin/stores/${storeID}/markets`)) {
    // Transport-only fixture; this response never claims business persistence.
    response.end(JSON.stringify({ transport: "MOCK" }));
    return;
  }
  response.writeHead(404).end(JSON.stringify({ code: "not_found" }));
});

test.beforeAll(async () => {
  await new Promise<void>((resolve, reject) => {
    server.once("error", reject);
    server.listen(19111, "127.0.0.1", resolve);
  });
});

test.afterAll(async () => {
  await new Promise<void>((resolve, reject) =>
    server.close((error) => (error ? reject(error) : resolve())),
  );
});

function cookieValue(headers: Record<string, string>, name: string) {
  return (
    new RegExp(`${name}=([^;,\\n]+)`).exec(headers["set-cookie"] ?? "")?.[1] ??
    ""
  );
}

async function chunkedOverflow() {
  return new Promise<number>((resolve, reject) => {
    const request = httpRequest(
      `${publicOrigin}/api/auth/login`,
      {
        method: "POST",
        headers: {
          Origin: publicOrigin,
          "Content-Type": "application/x-www-form-urlencoded",
        },
      },
      (response) => {
        response.resume();
        response.once("end", () => {
          request.destroy();
          resolve(response.statusCode ?? 0);
        });
      },
    );
    request.once("error", reject);
    request.write(Buffer.alloc(65_537, 97));
    setTimeout(() => {
      request.destroy();
      reject(new Error("server waited for the unbounded request to end"));
    }, 2_000).unref();
  });
}

test("PROVIDER_MOCK browser BFF enforces binding, cookies, CSRF and revocation order", async ({
  page,
}) => {
  const rejected = await page.request.post("/api/auth/login", {
    form: { locale: "zh-TW" },
    maxRedirects: 0,
  });
  expect(rejected.status()).toBe(403);
  expect(requests).toHaveLength(0);

  expect(await chunkedOverflow()).toBe(422);
  expect(requests).toHaveLength(0);

  const started = await page.request.post("/api/auth/login", {
    form: { locale: "zh-TW" },
    headers: { Origin: publicOrigin },
    maxRedirects: 0,
  });
  expect(started.status()).toBe(303);
  expect(started.headers()["location"]).toContain(
    "https://provider.example/authorize",
  );
  expect(started.headers()["set-cookie"]).toContain("__Host-commerce_login=");
  expect(started.headers()["set-cookie"]).toContain(
    "Secure; HttpOnly; SameSite=Lax",
  );
  const loginCookie = cookieValue(started.headers(), "__Host-commerce_login");

  const beforeWrongIssuer = requests.length;
  const wrongIssuer = await page.request.get(
    "/api/auth/callback?state=state-one&code=code-one&iss=https%3A%2F%2Fprovider.example%2Frealm",
    {
      headers: { Cookie: `__Host-commerce_login=${loginCookie}` },
      maxRedirects: 0,
    },
  );
  expect(wrongIssuer.headers()["location"]).toBe("/zh-TW/?auth=failed");
  expect(requests).toHaveLength(beforeWrongIssuer);

  const restarted = await page.request.post("/api/auth/login", {
    form: { locale: "zh-TW" },
    headers: { Origin: publicOrigin },
    maxRedirects: 0,
  });
  const restartedCookie = cookieValue(
    restarted.headers(),
    "__Host-commerce_login",
  );

  const completed = await page.request.get(
    `/api/auth/callback?state=state-one&code=code-one&iss=${encodeURIComponent(issuer)}`,
    {
      headers: { Cookie: `__Host-commerce_login=${restartedCookie}` },
      maxRedirects: 0,
    },
  );
  expect(completed.status()).toBe(303);
  expect(completed.headers()["location"]).toBe("/zh-TW/");
  const completionCookies = completed.headers()["set-cookie"];
  expect(completionCookies).toContain("__Host-commerce_session=");
  expect(completionCookies).toContain("__Host-commerce_csrf=");
  const sessionCookie = cookieValue(
    completed.headers(),
    "__Host-commerce_session",
  );
  const csrf = cookieValue(completed.headers(), "__Host-commerce_csrf");
  const authorityCookie = `__Host-commerce_session=${sessionCookie}; __Host-commerce_csrf=${csrf}`;

  const beforeAmbiguousSSR = requests.length;
  const ambiguousSSR = await page.request.get("/zh-CN/", {
    headers: {
      Cookie: `__Host-commerce_session=${sessionCookie}; __Host-commerce_session=${"D".repeat(42)}A`,
    },
  });
  expect(ambiguousSSR.status()).toBe(200);
  expect(requests).toHaveLength(beforeAmbiguousSSR);

  const safeSSR = await page.request.get("/zh-CN/", {
    headers: { Cookie: authorityCookie },
  });
  expect(safeSSR.status()).toBe(200);
  expect(await safeSSR.text()).not.toContain(
    "private diagnostic must not escape",
  );

  const listed = await page.request.get("/api/stores", {
    headers: { Cookie: authorityCookie },
  });
  expect(listed.status()).toBe(200);
  expect(await listed.json()).toEqual({
    items: [{ id: storeID, name: "Mock store", currency: "TWD" }],
  });
  const storesCall = requests.find((item) => item.path === "/v1/admin/stores")!;
  expect(storesCall.headers.authorization).toBe(`Bearer ${session}`);
  expect(storesCall.headers.cookie).toBeUndefined();
  expect(storesCall.headers["x-commerce-bff-key"]).toBeUndefined();

  const beforeBlockedWrite = requests.length;
  const blockedWrite = await page.request.post(
    `/api/stores/${storeID}/inventory/adjustments`,
    {
      headers: {
        Cookie: authorityCookie,
        Origin: publicOrigin,
        "Content-Type": "application/json",
        "Idempotency-Key": "adjust-key-1",
      },
      data: { delta: 1 },
    },
  );
  expect(blockedWrite.status()).toBe(403);
  expect(requests).toHaveLength(beforeBlockedWrite);

  expect(csrf).toMatch(/^[A-Za-z0-9_-]{43}$/);
  const acceptedWrite = await page.request.post(
    `/api/stores/${storeID}/inventory/adjustments`,
    {
      headers: {
        Origin: publicOrigin,
        Cookie: authorityCookie,
        "Content-Type": "application/json",
        "Idempotency-Key": "adjust-key-1",
        "X-CSRF-Token": csrf!,
        Authorization: "Bearer browser-forgery",
        "X-Commerce-BFF-Key": "browser-forgery",
      },
      data: { delta: 1 },
    },
  );
  expect(acceptedWrite.status()).toBe(200);
  const adjustment = requests.find((item) =>
    item.path.endsWith("/inventory/adjustments"),
  )!;
  expect(adjustment.headers.authorization).toBe(`Bearer ${session}`);
  expect(adjustment.headers.cookie).toBeUndefined();
  expect(adjustment.headers["x-commerce-bff-key"]).toBeUndefined();

  const market = "44444444-4444-4444-8444-444444444444";
  const discoveryBase = `/api/stores/${storeID}/markets`;
  const delivery = `${discoveryBase}/${market}/countries/TW/delivery-services`;
  const policy = `${delivery}/home/policy`;
  const paymentList = `${discoveryBase}/${market}/countries/TW/payment-methods`;
  for (const path of [
    discoveryBase + "?limit=2&cursor=test-cursor",
    delivery + "?limit=1",
    paymentList,
    policy,
  ]) {
    const response = await page.request.get(path, {
      headers: { Cookie: authorityCookie, Authorization: "Bearer forged" },
    });
    expect(response.status()).toBe(200);
    const sent = requests.at(-1)!;
    expect(sent.path).toBe(path.replace("/api/stores/", "/v1/admin/stores/"));
    expect(sent.headers.authorization).toBe(`Bearer ${session}`);
    expect(sent.headers.cookie).toBeUndefined();
  }
  for (const path of [paymentList + "?limit=1", policy + "?version=1"]) {
    const before = requests.length;
    expect(
      (
        await page.request.get(path, { headers: { Cookie: authorityCookie } })
      ).status(),
    ).toBe(422);
    expect(requests).toHaveLength(before);
  }
  for (const [method, path] of [
    ["POST", discoveryBase],
    ["PUT", policy],
  ]) {
    const before = requests.length;
    expect(
      (
        await page.request.fetch(path, {
          method,
          headers: { Cookie: authorityCookie, Origin: publicOrigin },
          data: {},
        })
      ).status(),
    ).toBe(403);
    expect(requests).toHaveLength(before);
    const body = JSON.stringify({ fixture: "transport-only" });
    const response = await page.request.fetch(path, {
      method,
      headers: {
        Cookie: authorityCookie,
        Origin: publicOrigin,
        "X-CSRF-Token": csrf!,
        "Idempotency-Key": "wizard-mock-key",
        "Content-Type": "application/json",
      },
      data: body,
    });
    expect(response.status()).toBe(200);
    expect(requests.at(-1)!.body).toBe(body);
    expect(requests.at(-1)!.headers["idempotency-key"]).toBe("wizard-mock-key");
  }
  const beforeForeignSetup = requests.length;
  expect(
    (
      await page.request.get(discoveryBase.replace(storeID, market), {
        headers: { Cookie: authorityCookie },
      })
    ).status(),
  ).toBe(404);
  expect(
    requests.slice(beforeForeignSetup).map((request) => request.path),
  ).toEqual(["/v1/admin/stores"]);

  const onboarding = await page.request.post("/api/onboarding/initial-store", {
    headers: {
      Origin: publicOrigin,
      Cookie: authorityCookie,
      "Content-Type": "application/json",
      "Idempotency-Key": "onboard-key-1",
      "X-CSRF-Token": csrf!,
    },
    data: {
      tenant_name: "New merchant",
      store_name: "New store",
      warehouse_name: "New warehouse",
      currency: "TWD",
    },
  });
  expect(onboarding.status()).toBe(200);
  const initial = requests.find(
    (item) => item.path === "/v1/identity/initial-store",
  )!;
  expect(initial.headers["idempotency-key"]).toBe("onboard-key-1");
  expect(Object.keys(JSON.parse(initial.body)).sort()).toEqual([
    "currency",
    "store_name",
    "tenant_name",
    "warehouse_name",
  ]);

  const failedLogout = await page.request.post("/api/auth/logout", {
    headers: {
      Cookie: authorityCookie,
      Origin: publicOrigin,
      "Content-Type": "application/json",
      "X-CSRF-Token": csrf!,
    },
    data: {},
  });
  expect(failedLogout.status()).toBe(503);
  expect(failedLogout.headers()["set-cookie"]).toBeUndefined();
  failLogout = false;
  const logout = await page.request.post("/api/auth/logout", {
    headers: {
      Cookie: authorityCookie,
      Origin: publicOrigin,
      "Content-Type": "application/json",
      "X-CSRF-Token": csrf!,
    },
    data: {},
  });
  expect(logout.status()).toBe(204);
  expect(logout.headers()["set-cookie"]).toContain(
    "__Host-commerce_session=; Path=/; Max-Age=0",
  );

  for (const call of requests.filter((item) =>
    item.path.startsWith("/v1/identity/"),
  )) {
    expect(call.headers["x-commerce-bff-key"]).toBe(bffKey);
    expect(call.headers.origin).toBeUndefined();
    expect(call.headers.cookie).toBeUndefined();
  }
});
