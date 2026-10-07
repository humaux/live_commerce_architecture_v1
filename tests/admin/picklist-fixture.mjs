// Purpose: isolated W3-U1b MOCK backend with observable selection/export/batch effects.
// Depends on: node HTTP/HTTPS/crypto/fs/os/path/child_process and OpenSSL for ephemeral test TLS; W3-02B DTOs, no PG/provider.
// Used by: picklist.spec.ts real-click CI gate; synthetic IDs and identities only.
import { createServer, request as forwardRequest } from "node:http";
import { createServer as createTLSServer } from "node:https";
import { execFileSync } from "node:child_process";
import { mkdtemp, readFile, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { randomBytes } from "node:crypto";
export const storeID = "11111111-1111-4111-8111-111111111111";
export const sessionID = "22222222-2222-4222-8222-222222222222";
export const orderID = (n) =>
  `33333333-3333-4333-8333-${String(n).padStart(12, "0")}`;
const sku = "44444444-4444-4444-8444-444444444444";
const line = {
  sku_id: sku,
  sku_code: "SYNTHETIC-ONE",
  title: "合成測試商品 Synthetic item",
  option_label: null,
  qty: 2,
};
const stamp = "2026-10-06T06:00:00.000000Z";
const number = (id) => `LC-${id.replaceAll("-", "").toUpperCase()}`;
const columns = {
  black_cat:
    "order_id,recipient_name,phone,region,city,line1,line2,items,collect_minor",
  hsinchu:
    "order_id,order_number,recipient_name,phone,region,city,line1,line2,items,collect_minor",
  chunghwa_post:
    "order_id,recipient_name,phone,country,region,city,postal_code,line1,line2,items,total_minor",
  generic:
    "order_id,order_number,created_at_utc,destination_kind,recipient_name,phone,region,city,line1,line2,pickup_code,items,total_minor,collect_minor",
};

/** Start a disposable TLS front for one loopback Next server; close removes only this listener and its private cert directory. */
export async function picklistTLS(upstreamOrigin) {
  const upstream = new URL(upstreamOrigin);
  if (upstream.protocol !== "http:" || upstream.hostname !== "127.0.0.1" || upstream.pathname !== "/" || upstream.search || upstream.hash || upstream.username || upstream.password)
    throw new Error("picklist TLS target must be the owned loopback server");
  // Same test-only TLS pattern as browserFront/catalog-media: public origin and Secure cookies stay production-like.
  const certificateDirectory = await mkdtemp(join(tmpdir(), "lc-picklist-tls-"));
  let server;
  const close = async () => {
    if (server?.listening) {
      server.closeAllConnections();
      await new Promise(resolve => server.close(resolve));
    }
    await rm(certificateDirectory, { recursive: true, force: true });
  };
  try {
    execFileSync("openssl", ["req", "-x509", "-newkey", "rsa:2048", "-nodes", "-keyout", join(certificateDirectory, "key.pem"),
      "-out", join(certificateDirectory, "cert.pem"), "-days", "1", "-subj", "/CN=localhost", "-addext", "subjectAltName=IP:127.0.0.1"],
    { stdio: "ignore", timeout: 20000 });
    server = createTLSServer({ key: await readFile(join(certificateDirectory, "key.pem")), cert: await readFile(join(certificateDirectory, "cert.pem")) }, (req, res) => {
      const pending = forwardRequest({ hostname: "127.0.0.1", port: upstream.port, path: req.url, method: req.method, agent: false,
        // Preserve Host, Origin and the automatically supplied cookies; never manufacture an auth header.
        headers: { ...req.headers, "x-forwarded-proto": "https" } }, response => {
        res.writeHead(response.statusCode ?? 502, response.headers); response.pipe(res);
      });
      pending.setTimeout(30000, () => pending.destroy(new Error("owned Next proxy deadline")));
      pending.on("error", () => { if (res.destroyed) return; if (!res.headersSent) res.writeHead(502); res.end(); });
      req.on("aborted", () => pending.destroy());
      res.on("close", () => pending.destroy());
      req.pipe(pending);
    });
    await new Promise((resolve, reject) => {
      server.once("error", reject);
      server.listen(0, "127.0.0.1", () => { server.removeListener("error", reject); resolve(); });
    });
    return { origin: `https://127.0.0.1:${server.address().port}`, certificateDirectory, close };
  } catch (error) {
    await close();
    throw error;
  }
}

/** Start one synthetic server with test-owned faults; no production authority accepted. */
export async function pickFixture() {
  const token = randomBytes(32).toString("base64url"),
    key = randomBytes(32).toString("base64url");
  const state = {
    count: 20,
    canShip: true,
    canExport: true,
    unknown: false,
    recovery: "missing",
    reads: [],
    retry: false,
    pickMalformed: process.env.LC_PICKLIST_INJECT_FAULT === "missing_totals",
    requests: [],
    batches: [],
    exports: [],
  };
  const server = createServer(async (req, res) => {
    res.setHeader("Content-Type", "application/json");
    res.setHeader("Cache-Control", "no-store, private");
    const json = (value, status = 200) => {
      res.statusCode = status;
      res.end(JSON.stringify(value));
    };
    const url = new URL(req.url, "http://localhost");
    const path = url.pathname;
    if (
      path.startsWith("/v1/identity/")
        ? req.headers["x-commerce-bff-key"] !== key
        : req.headers.authorization !== `Bearer ${token}`
    )
      return json({ code: "unauthorized" }, 401);
    if (path === "/v1/admin/stores")
      return json({
        items: [
          {
            id: storeID,
            name: "合成撿貨測試店 Synthetic pick store",
            currency: "TWD",
            role: "owner",
            permissions: [
              "orders:read",
              "live:read",
              ...(state.canShip ? ["fulfillment:write"] : []),
              ...(state.canExport ? ["orders:export"] : []),
            ],
          },
        ],
      });
    if (!path.startsWith(`/v1/admin/stores/${storeID}/`))
      return json({ code: "not_found" }, 404);
    const route = path.slice(`/v1/admin/stores/${storeID}/`.length);
    if (route === "order-actions")
      return json({
        refund: false,
        fulfillment_write: state.canShip,
        orders_export: state.canExport,
      });
    if (
      /^orders\/[0-9a-f-]{36}\/cvs-shipment$/.test(route) &&
      req.method === "GET"
    ) {
      state.reads.push(route);
      if (state.recovery === "missing") return json({ code: "not_found" }, 404);
      return json({
        current_attempt: 1,
        expected_version: 2,
        validity_days: 5,
        attempts: [
          {
            attempt: 1,
            state: state.recovery === "unknown" ? "UNKNOWN" : "CREATED",
            subtype: "UNIMARTC2C",
            environment: "SANDBOX",
            receiver_store_id: "131386",
            goods_amount: 25,
            collection_amount: null,
            provider_logistics_id: null,
            code: null,
            print_available: false,
            result_code: null,
            last_status_code: null,
            last_status_at: null,
            alerts: [],
            created_at: stamp,
            updated_at: stamp,
            version: 2,
            events: [],
          },
        ],
      });
    }
    if (/^orders\/[0-9a-f-]{36}$/.test(route) && req.method === "GET") {
      const id = route.slice(7);
      return json({
        order_id: id,
        created_at: stamp,
        updated_at: stamp,
        currency: "TWD",
        total_minor: 2500,
        commercial_state: "CONFIRMED",
        fulfillment_state: "MANUAL_UNASSIGNED",
        payment_state: "NOT_STARTED",
        test_mode: false,
        work_state: "NONE",
        refunded_minor: 0,
        refund_pending_minor: 0,
        source: "storefront",
        pickup_source: "merchant_attested",
        payment_mode: "bank_transfer",
        collection_state: null,
        cod_surcharge_minor: null,
        cod_collect_minor: null,
        shipment: null,
        country: "TW",
        service_code: "cvs_711",
        items: [
          {
            sku_id: sku,
            code: "SYNTHETIC-ONE",
            name: "Synthetic frozen item",
            quantity: 2,
            unit_price_minor: 1250,
            amount: {
              subtotal_minor: 2500,
              discount_minor: 0,
              tax_minor: 0,
              total_minor: 2500,
            },
          },
        ],
        totals: {
          subtotal_minor: 2500,
          discount_minor: 0,
          shipping_minor: 0,
          shipping_tax_minor: 0,
          tax_minor: 0,
          total_minor: 2500,
        },
        destination: {
          kind: "cvs_711",
          country: "TW",
          recipient_name: "Synthetic Recipient",
          phone: "+886900000002",
          home_address: {
            region: "",
            city: "",
            postal_code: "",
            line1: "",
            line2: "",
          },
          pickup: {
            kind: "cvs_711",
            namespace: "fixture.case",
            code: "131386",
            name: "Synthetic pickup",
            address: "Synthetic address",
            verification_kind: "MANUAL_ATTESTED",
          },
        },
      });
    }
    if (
      (route === "orders" && req.method === "GET") ||
      (route === "orders/search" && req.method === "POST")
    ) {
      let search = null;
      if (route === "orders/search") {
        let text = "";
        for await (const chunk of req) text += chunk;
        search = JSON.parse(text).q;
        state.reads.push({ route, q: search });
      }
      const offset = url.searchParams.has("cursor")
        ? Number(
            Buffer.from(url.searchParams.get("cursor"), "base64url").toString(),
          )
        : 0;
      const pageCount = search ? 1 : state.count;
      const items = Array.from(
        { length: search ? 1 : Math.min(10, state.count - offset) },
        (_, i) => {
          const n = search ? Number(search.slice(-12)) : offset + i + 1,
            id = orderID(n);
          return {
            order_id: id,
            order_number: number(id),
            created_at: stamp,
            updated_at: stamp,
            currency: "TWD",
            total_minor: 2500,
            commercial_state: "CONFIRMED",
            fulfillment_state: "MANUAL_UNASSIGNED",
            payment_state: "NOT_STARTED",
            test_mode: false,
            work_state: "NONE",
            refunded_minor: 0,
            refund_pending_minor: 0,
            source: "storefront",
            pickup_source: n % 2 ? "merchant_attested" : null,
            payment_mode: "bank_transfer",
            collection_state: null,
            cod_surcharge_minor: null,
            cod_collect_minor: null,
            recipient_masked: "—",
            delivery_kind: n % 2 ? "cvs_711" : "home",
            live_sessions: [{ id: sessionID, name: "合成場次 Synthetic live" }],
          };
        },
      );
      return json({
        items,
        next_cursor:
          !search && offset + 10 < state.count
            ? Buffer.from(String(offset + 10)).toString("base64url")
            : "",
        total: pageCount,
        counts: {
          all: pageCount,
          unpaid: 0,
          transfer_review: 0,
          ready_to_ship: pageCount,
          ready_to_consign: 0,
          shipped: 0,
          completed: 0,
          cancelled: 0,
        },
        sessions: [{ id: sessionID, name: "合成場次 Synthetic live" }],
      });
    }
    if (
      ["orders/pick-list", "orders/export", "shipments/cvs-batch"].includes(
        route,
      )
    ) {
      let text = "";
      for await (const chunk of req) text += chunk;
      const body = JSON.parse(text);
      state.requests.push({
        route,
        body,
        key: req.headers["idempotency-key"] ?? null,
      });
      const ids = body.order_ids ?? [orderID(1), orderID(2)];
      if (route !== "shipments/cvs-batch" && req.headers["idempotency-key"])
        return json({ code: "invalid_request" }, 422);
      if (route === "orders/pick-list")
        return json(
          state.pickMalformed
            ? { orders: [] }
            : {
                generated_at: "2026-10-06T06:00:00Z",
                orders: ids.slice(0, -1).map((id) => ({
                  order_id: id,
                  order_number: number(id),
                  lines: [line],
                })),
                totals:
                  ids.length > 1
                    ? [{ ...line, qty: 2 * (ids.length - 1) }]
                    : [],
                skipped: [{ order_id: ids.at(-1), code: "not_pickable" }],
              },
        );
      if (route === "orders/export") {
        if (!state.canExport) return json({ code: "forbidden" }, 403);
        const template = url.searchParams.get("template");
        if (!columns[template]) return json({ code: "invalid_request" }, 422);
        const csv =
          "\ufeff" +
          columns[template] +
          "\r\n" +
          ids
            .map((id) =>
              columns[template]
                .split(",")
                .map((col) =>
                  col === "order_id"
                    ? id
                    : col === "order_number"
                      ? number(id)
                      : col === "collect_minor"
                        ? ""
                        : col === "total_minor"
                          ? "25"
                          : "synthetic",
                )
                .join(","),
            )
            .join("\r\n") +
          "\r\n";
        state.exports.push({ template, csv });
        res.setHeader("Content-Type", "text/csv; charset=utf-8");
        res.setHeader(
          "Content-Disposition",
          `attachment; filename="${template}-11111111-202610060600.csv"`,
        );
        return res.end(csv);
      }
      if (!state.canShip) return json({ code: "forbidden" }, 403);
      if (!req.headers["idempotency-key"] || ids.length > 100)
        return json({ code: "invalid_request" }, 422);
      state.batches.push({ ids, key: req.headers["idempotency-key"] });
      if (state.unknown) return json({ code: "unavailable" }, 503);
      return json({
        results: ids.map((id, i) => ({
          order_id: id,
          outcome: i === 0 ? "queued" : i === 1 ? "already" : "failed",
          ...(i > 1 ? { code: state.retry ? "retry" : "not_cvs" } : {}),
        })),
      });
    }
    return json({ code: "not_found" }, 404);
  });
  await new Promise((resolve) => server.listen(0, "127.0.0.1", resolve));
  return {
    state,
    token,
    key,
    origin: `http://127.0.0.1:${server.address().port}`,
    close: () => new Promise((resolve) => server.close(resolve)),
  };
}
