// W0 isolated MOCK transport. No production credentials, provider writes or PG.
import { createServer } from "node:http";
import { randomBytes } from "node:crypto";
export const storeID = "11111111-1111-4111-8111-111111111111";
export const otherID = "22222222-2222-4222-8222-222222222222";
export const entityID = "33333333-3333-4333-8333-333333333333";
export async function fixture() {
  const key = randomBytes(32).toString("base64url"),
    token = randomBytes(32).toString("base64url");
  const permissions = [
    "catalog:read",
    "inventory:read",
    "orders:read",
    "live:read",
    "customers:read",
    "billing:manage",
    "ads:read",
    "integration:read",
    "pricing:read",
  ];
  const state = { role: "owner", permissions, requests: [] };
  const server = createServer(async (req, res) => {
    const path = new URL(req.url, "http://localhost").pathname;
    state.requests.push(path);
    res.setHeader("Content-Type", "application/json");
    res.setHeader("Cache-Control", "no-store");
    const json = (data) => res.end(JSON.stringify(data));
    if (
      path.startsWith("/v1/identity/")
        ? req.headers["x-commerce-bff-key"] !== key
        : req.headers.authorization !== `Bearer ${token}`
    ) {
      res.statusCode = 403;
      return json({ code: "forbidden" });
    }
    if (path === "/v1/identity/logout") {
      res.statusCode = 204;
      return res.end();
    }
    if (path === "/v1/admin/stores")
      return json({
        items: [
          {
            id: storeID,
            name: "W0 合成測試店 · Synthetic baseline",
            currency: "TWD",
            role: state.role,
            permissions: state.permissions,
          },
          {
            id: otherID,
            name: "A very long English store name for accessible workspace switching without overlap",
            currency: "TWD",
            role: state.role,
            permissions: state.permissions,
          },
        ],
      });
    if (path.endsWith("/warehouses"))
      return json({
        items: [{ id: entityID, name: "Synthetic warehouse" }],
        next_cursor: "",
      });
    if (
      /\/(catalog-products|collections|catalog-ledger|customers|orders|live-sessions)$/.test(
        path,
      )
    )
      return json({ items: [], next_cursor: "" });
    // Explicit unavailable is a real legacy state, not fabricated business data.
    res.statusCode = 503;
    return json({
      code: "retry_later",
      message: "MOCK endpoint intentionally unavailable",
      retryable: true,
      details: {},
      request_id: "w0-mock",
    });
  });
  await new Promise((r) => server.listen(0, "127.0.0.1", r));
  return {
    state,
    key,
    token,
    origin: `http://127.0.0.1:${server.address().port}`,
    close: () => new Promise((r) => server.close(r)),
  };
}
