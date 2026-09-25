import { expect, test } from "@playwright/test";
import * as http from "node:http";

const required = (name: string) => {
  const value = process.env[name];
  if (!value) throw new Error(`${name} is required`);
  return value;
};
const publicOrigin = required("LC_BROWSER_PUBLIC_ORIGIN");
const apiOrigin = required("LC_BROWSER_API_ORIGIN");
const store = required("LC_BROWSER_ORDER_STORE");
const order = required("LC_BROWSER_ORDER_ID");
const foreignStore = required("LC_BROWSER_FOREIGN_STORE");
const foreignOrder = required("LC_BROWSER_FOREIGN_ORDER_ID");
const unlistedStore = required("LC_BROWSER_UNLISTED_STORE");
const noOrdersToken = required("LC_BROWSER_NO_ORDERS_TOKEN");
const expiredToken = required("LC_BROWSER_EXPIRED_TOKEN");
const revokedToken = required("LC_BROWSER_REVOKED_TOKEN");
const cookieName = "__Host-commerce_session";
const base = `/api/stores/${store}/orders`;
const foreignBase = `/api/stores/${foreignStore}/orders`;

test.use({
  baseURL: publicOrigin,
  trace: "off",
  screenshot: "only-on-failure",
});
test.describe.configure({ timeout: 90_000 });

type RawResponse = {
  status: number;
  headers: http.IncomingHttpHeaders;
  body: string;
};
function raw(
  path: string,
  method = "GET",
  headers: Record<string, string | string[]> = {},
  body?: string,
): Promise<RawResponse> {
  const origin = new URL(publicOrigin);
  return new Promise((resolve, reject) => {
    const req = http.request(
      {
        hostname: origin.hostname,
        port: origin.port,
        method,
        path,
        headers,
        timeout: 5_000,
      },
      (res) => {
        const chunks: Buffer[] = [];
        res.on("data", (chunk: Buffer) => chunks.push(chunk));
        res.on("end", () =>
          resolve({
            status: res.statusCode ?? 0,
            headers: res.headers,
            body: Buffer.concat(chunks).toString("utf8"),
          }),
        );
      },
    );
    req.on("timeout", () => req.destroy(new Error("Next request timed out")));
    req.on("error", reject);
    req.end(body);
  });
}

async function observation() {
  const response = await fetch(`${apiOrigin}/__test/order-observation`);
  expect(response.status).toBe(200);
  return (await response.json()) as { count: number; last_uri: string };
}

function expectSafe(response: RawResponse, status: number) {
  expect(response.status).toBe(status);
  expect(response.headers["cache-control"]).toBe("no-store");
  expect(response.body).not.toMatch(
    /SQLSTATE|backend-secret|browser-bogus-token/i,
  );
  const parsed = JSON.parse(response.body) as Record<string, unknown>;
  expect(parsed.details).toEqual({});
  expect(typeof parsed.code).toBe("string");
}

test("MBT01-04 signed cookie, raw route policy and real Go/PG order reads", async ({
  context,
  page,
}) => {
  await page.goto(`${publicOrigin}/en/`);
  await page
    .getByRole("button", { name: "Sign in with identity service" })
    .click();
  await expect
    .poll(async () =>
      (await context.cookies(publicOrigin)).some(
        (item) => item.name === cookieName,
      ),
    )
    .toBe(true);
  const cookies = (await context.cookies(publicOrigin)).filter(
    (item) => item.name === cookieName,
  );
  expect(cookies).toHaveLength(1);
  expect(cookies[0].httpOnly).toBe(true);
  const cookie = `${cookieName}=${cookies[0].value}`;
  const authorized = { Cookie: cookie };

  const first = await raw(`${base}?limit=1&state=all`, "GET", {
    Cookie: `${cookie}; browser_bogus=header`,
    Authorization: "Bearer browser-bogus-token",
    "X-Tenant-ID": "browser-bogus-tenant",
    "X-Forwarded-Host": "attacker.invalid",
    "X-BFF-Test": "browser-bogus-header",
  });
  expect(first.status).toBe(200);
  expect(first.headers["cache-control"]).toBe("private, no-store");
  const pageBody = JSON.parse(first.body) as {
    items: Array<{ order_id: string }>;
    next_cursor: string;
  };
  expect(pageBody.items.map((item) => item.order_id)).toContain(order);
  expect((await observation()).last_uri).toBe(
    `/v1/admin/stores/${store}/orders?limit=1&state=all`,
  );

  const detail = await raw(`${base}/${order}`, "GET", authorized);
  expect(detail.status).toBe(200);
  expect(detail.headers["cache-control"]).toBe("private, no-store");
  expect((JSON.parse(detail.body) as { order_id: string }).order_id).toBe(
    order,
  );
  expect((await observation()).last_uri).toBe(
    `/v1/admin/stores/${store}/orders/${order}`,
  );

  const other = await raw(`${foreignBase}/${foreignOrder}`, "GET", authorized);
  expect(other.status).toBe(200);
  expect((JSON.parse(other.body) as { order_id: string }).order_id).toBe(
    foreignOrder,
  );
  const missing = await raw(`${base}/${foreignOrder}`, "GET", authorized);
  const crossStore = await raw(`${foreignBase}/${order}`, "GET", authorized);
  expectSafe(missing, 404);
  expectSafe(crossStore, 404);
  expect(JSON.parse(crossStore.body).code).toBe(JSON.parse(missing.body).code);
  expectSafe(
    await raw(`/api/stores/${unlistedStore}/orders`, "GET", authorized),
    404,
  );

  for (const [path, method, extra, payload] of [
    [`${base}?`, "GET", {}, undefined],
    [`${base}?limit=1&limit=2`, "GET", {}, undefined],
    [`${base}?buyer_id=x`, "GET", {}, undefined],
    [`${base}?limit=`, "GET", {}, undefined],
    [`${base}?=1`, "GET", {}, undefined],
    [`${base}?limit=1&`, "GET", {}, undefined],
    [`${base}?limit=1&&state=all`, "GET", {}, undefined],
    [`${base}?limit=%31`, "GET", {}, undefined],
    [`${base}?%6cimit=1`, "GET", {}, undefined],
    [`${base}?limit=%2531`, "GET", {}, undefined],
    [`${base}?limit=1+`, "GET", {}, undefined],
    [`${base}?limit=%`, "GET", {}, undefined],
    [`${base}?limit=0`, "GET", {}, undefined],
    [`${base}?limit=01`, "GET", {}, undefined],
    [`${base}?limit=101`, "GET", {}, undefined],
    [`${base}?cursor=${"a".repeat(1025)}`, "GET", {}, undefined],
    [`${base}?state=draft`, "GET", {}, undefined],
    [`${base}/${order}?`, "GET", {}, undefined],
    [`${base}/${order}?state=all`, "GET", {}, undefined],
    [base, "GET", { "Content-Length": "1" }, "x"],
    [base, "GET", { "Transfer-Encoding": "chunked" }, "x"],
    [base, "GET", { "Idempotency-Key": "read-only-key" }, undefined],
  ] as Array<[string, string, Record<string, string>, string | undefined]>) {
    const before = (await observation()).count;
    const response = await raw(
      path,
      method,
      { ...authorized, ...extra },
      payload,
    );
    expectSafe(response, 422);
    expect((await observation()).count).toBe(before);
  }
  for (const path of [`/api/stores/not-a-uuid/orders`, `${base}/not-a-uuid`]) {
    const before = (await observation()).count;
    expectSafe(await raw(path, "GET", authorized), 404);
    expect((await observation()).count).toBe(before);
  }
  for (const method of ["POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"]) {
    const before = (await observation()).count;
    const response = await raw(base, method, authorized);
    expect([404, 405]).toContain(response.status);
    expect(response.headers["cache-control"]).toBe("no-store");
    expect((await observation()).count).toBe(before);
  }

  const noCookie = await raw(base);
  const wrong = await raw(base, "GET", {
    Cookie: `${cookieName}=not-a-token`,
  });
  const ambiguous = await raw(base, "GET", {
    Cookie: `${cookie}; ${cookie}`,
  });
  for (const response of [noCookie, wrong, ambiguous]) {
    expectSafe(response, 401);
    expect(String(response.headers["set-cookie"])).toContain(`${cookieName}=`);
    expect(String(response.headers["set-cookie"])).toContain("Max-Age=0");
  }
  for (const token of [expiredToken, revokedToken]) {
    const response = await raw(base, "GET", {
      Cookie: `${cookieName}=${token}`,
    });
    expectSafe(response, 401);
    expect(String(response.headers["set-cookie"])).toContain("Max-Age=0");
  }
  expectSafe(
    await raw(base, "GET", {
      Cookie: `${cookieName}=${noOrdersToken}`,
    }),
    403,
  );
  expectSafe(
    await raw(`${base}/${crypto.randomUUID()}`, "GET", authorized),
    404,
  );
  const beforeUnavailable = (await observation()).count;
  const unavailable = await raw(`${base}?state=CANCELLED`, "GET", authorized);
  expectSafe(unavailable, 503);
  expect((await observation()).count).toBe(beforeUnavailable + 1);
  expect(unavailable.headers["x-backend-secret"]).toBeUndefined();
  expect(unavailable.headers["set-cookie"]).toBeUndefined();
});
