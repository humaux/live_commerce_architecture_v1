// Purpose: The one admin BFF catch-all: proxies exactly the allowlisted per-store resources to Go, nothing generic
//   (including Meta health B1/B2, the W3-U5 returns/cancel grammar, the W6-05B operations ledger and the W6-06B ads unbind/feed).
// Depends on: @/lib/backend, @/lib/auth, server session/store/CSRF authority and the per-domain request grammars in
//   @/lib/*-request (orders, customers, logistics, promotions, studio, claims, design, meta-connect, ads, returns).
// Used by: every admin client module under apps/admin/lib (browser fetch -> this route -> Go /v1/admin/stores/...);
//   health responses are closed and private/no-store.
import { validMediaQuery } from "@/lib/product-media-model";
import { inboxResource, inboxRoute, validInboxRequest, validInboxBody, inboxErrorCode } from "@/lib/inbox-bff";
import { callBackend, fixtureSession } from "@/lib/backend";
import {
  orderActionRoute, validCSVHeaders, validKeylessCommandRequest, validKeylessRequest, validOrdersQuery, validReturnsQuery,
} from "@/lib/orders-request";
import { customersRoute, validCustomersBody, validCustomersRequest } from "@/lib/customers-request";
import { cardPaymentsRoute, validCardPaymentsBody, validCardPaymentsRequest } from "@/lib/card-payments-request";
import { logisticsRoute } from "@/lib/logistics-request";
import { promotionsRoute } from "@/lib/promotions-request";
import { validStudioInputToken, validStudioQuery } from "@/lib/studio-request";
import {
  claimLinkRoute, claimsCollection, claimsRoutes, claimsSubpath, validClaimLink,
} from "@/lib/claims-request";
import {
  DESIGN_MAX_JSON, designDetails, designGetPaths, designPostPaths, designPutPaths, isDesignImageBytes, isDesignJsonPut, isDesignPath, isDesignUpload, validDesignRequest,
} from "@/lib/design-request";
import { metaHealthRoute, validMetaHealthRequest, validRecheckBody } from "@/lib/meta-health-request";
import { parseMetaHealth, parseRecheck } from "@/lib/meta-health-model";
import { metaConnectAny, metaConnectRoutes, validMetaConnectRequest } from "@/lib/meta-connect-request";
import { adsAny, adsBodyless, adsKeyless, adsRoutes, validAdsQuery, validIfMatch, validAdsUnbindBody } from "@/lib/ads-request";
import { parseAdsUnbind, parseCatalogFeed, parseAdsInFlight } from "@/lib/ads-model";
import { operationRoute, validOperationsRequest, validOperationBody, operationRetryAfter } from "@/lib/operations-request";
import { parseOperationList, parseOperationDetail, parseOperationResult } from "@/lib/operations-model";
import { parseStudioInput, parseStudioInputPrepared } from "@/lib/studio-model";
import {
  authConfig,
  authenticatedStores,
  clearAuthCookies,
  localError,
  requireCSRF,
  requireOrigin,
  readBody,
  safeError,
  safeJSON,
  sessionToken,
} from "@/lib/auth";

const uuid = "[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}";
const account = `provider-accounts(?:/${uuid})?`;
const setting = `markets/${uuid}/countries/[A-Z]{2}/(?:delivery-services|payment-methods)/[a-z][a-z0-9_-]{0,39}`;
const inspect = `markets/${uuid}/countries/[A-Z]{2}/payment-methods/[a-z][a-z0-9_-]{0,39}/inspect`;
const deliveryCollection = `markets/${uuid}/countries/[A-Z]{2}/delivery-services`;
const paymentCollection = `markets/${uuid}/countries/TW/payment-methods`;
const policy = `${deliveryCollection}/[a-z][a-z0-9_-]{0,39}/policy`;
const purchaseEntry = `products/${uuid}/purchase-entry`;
// catalog-media CM3: product photos -> Go internal/httpapi/images.go. Exactly these five resources, nothing generic.
const imagesRoot = `products/${uuid}/images`;
const imageItem = `${imagesRoot}/${uuid}`;
const imageWrites = `${imagesRoot}|${imageItem}/delete|${imagesRoot}/order|${imageItem}/move|products/${uuid}/(?:option-images|image-axis)`;
const MAX_UPLOAD = 2.5 * 1024 * 1024; // CM3: BFF body cap; Go re-checks 2 MiB for the file itself and is the authority
// catalog-core (storefront-v2 A, unit catalog-core): product list/detail + collections -> Go internal/httpapi/collections.go.
// Exactly these resources: GET catalog-products (q,status,cursor,limit), GET products/{id}, collections CRUD, ordered
// membership (PUT), one collection image (multipart upload, bytes preview, delete).
const catalogProducts = "catalog-products";
const collectionsRoot = "collections";
const collectionItem = `collections/${uuid}`;
const collectionImage = `${collectionItem}/image`;
const catalogV2Writes = `${collectionsRoot}|${collectionItem}/delete|${collectionImage}|${collectionImage}/delete`;
// W4-U1 platform card payments + settlements (card-payments-request.ts): GET/PUT payments/card (keyless CAS PUT),
// GET settlements, GET settlements/{id} -> Go internal/httpapi/payment_card.go + settlement reads.
// Product-core: exact document/bulk/copy commands, with the same authenticated, keyed JSON policy.
const productCommands = `products/document|products/bulk-status|products/${uuid}/copy`;
const orders = `orders(?:/${uuid})?`;
// R3 storefront-publish: GET storefront (state + bound origins), POST storefront/publication {published, expected_version}
// -> Go internal/httpapi/storefront.go.
// R5 store-domains (Decision 3): the merchant self-service domain routes on the same card -> the same Go file.
const storefrontRead = "storefront";
const storefrontWrite = "storefront/publication";
const storefrontDomains = "storefront/domains";
const storefrontDomainMove = "storefront/domains/(?:suspend|detach)";
const studioDetail = `live-sessions/${uuid}`;
const studioInput = `${studioDetail}/input(?:/(?:start|token|prepared))?`;
const studioInputRead = `${studioDetail}/input(?:/prepared)?`;
const studioInputReadRoute = new RegExp(`^${studioInputRead}$`);
const studioInputRoute = new RegExp(`^${studioInput}$`);
const studioAction = `${studioDetail}/(?:rehearsal/(?:start|stop)|input/(?:start|token))`;
const studioAny = new RegExp(`^(?:live-sessions|${studioDetail}|${studioAction}|${studioInputRead}|${studioDetail}/${claimsSubpath})$`);
const routes: Record<string, RegExp> = {
  GET: new RegExp(
    `^(catalog-ledger|${catalogProducts}|products|products/${uuid}|${collectionsRoot}|${collectionItem}|${collectionImage}|warehouses|inventory|products/${uuid}/skus|${purchaseEntry}|${storefrontRead}|${storefrontDomains}|${imagesRoot}|${imageItem}|${designGetPaths}|${account}|${setting}|markets|${deliveryCollection}|${paymentCollection}|${policy}|${orders}|live-sessions|${studioDetail}|${studioInputRead}|${claimsRoutes.GET}|${adsRoutes.GET}|${metaConnectRoutes.GET}|payments/card|settlements|settlements/${uuid})$`,
  ),
  POST: new RegExp(
    `^(orders/search|products|${productCommands}|skus|warehouses|inventory/adjustments|products/${uuid}/archive|${storefrontWrite}|${storefrontDomains}|${storefrontDomainMove}|${imageWrites}|${designPostPaths}|${catalogV2Writes}|skus/${uuid}/(archive|price)|provider-accounts|provider-accounts/${uuid}/rotate|${inspect}|markets|live-sessions|${studioAction}|${claimsRoutes.POST}|${adsRoutes.POST}|${metaConnectRoutes.POST})$`,
  ),
  PATCH: new RegExp(`^(products/${uuid}|${collectionItem}|skus/${uuid}|${studioDetail}|${claimsRoutes.PATCH})$`),
  // Studio PUT is only the comment-source bind (claims-request.ts); settings PUTs are the rest.
  PUT: new RegExp(`^(products/${uuid}/document|${setting}|${policy}|${designPutPaths}|${collectionItem}/products|${claimsRoutes.PUT}|${adsRoutes.PUT}|payments/card)$`),
};
const exactStore = new RegExp(`^${uuid}$`);
const inspectRoute = new RegExp(`^${inspect}$`);
const discoveryRoute = new RegExp(
  `^(markets|${deliveryCollection}|${paymentCollection}|${policy})$`,
);
const pagedSettingsRoute = new RegExp(`^(markets|${deliveryCollection})$`);
const purchaseEntryRoute = new RegExp(`^${purchaseEntry}$`);
const orderRoute = new RegExp(`^${orders}$`);
const imagesRootRoute = new RegExp(`^${imagesRoot}$`);
const imageItemRoute = new RegExp(`^${imageItem}$`);
const collectionImageRoute = new RegExp(`^${collectionImage}$`);
// catalog-core: only these two GETs carry a query (Go is the grammar authority; the BFF caps the key set).
const catalogQueryRoute = new RegExp(`^(?:${catalogProducts}|${collectionsRoot})$`);
const catalogQueryKeys = new Set(["q", "status", "cursor", "limit"]);
const catalogV2Any = new RegExp(`^(?:${catalogProducts}|${productCommands}|products/${uuid}/document|products/${uuid}|${collectionsRoot}|${collectionItem}|${collectionItem}/(?:delete|products|image|image/delete))$`);
const imagesAny = new RegExp(`^(?:${imagesRoot}|${imageItem}|${imageItem}/(?:delete|move)|${imagesRoot}/order|products/${uuid}/(?:option-images|image-axis))$`);
const IMAGE_TYPES = new Set(["image/jpeg", "image/png", "image/webp"]);
type Context = { params: Promise<{ store: string; resource: string[] }> };

async function route(request: Request, context: Context) {
  const error = localError;
  const { store, resource } = await context.params;
  const path = resource.join("/");
  const inbox = inboxResource(path);
  if (inbox && !authConfig) return error(404, "not_found");
  if (inbox && !inboxRoute(request.method, path)) return error(405, "method_not_allowed", path.endsWith("/messages") ? "GET, POST" : inboxRoute("GET", path) ? "GET" : "POST");
  if (inbox && !validInboxRequest(request, path)) {
    const filters = new URL(request.url).searchParams.getAll("filter");
    if (path === "inbox/conversations" && filters.length === 1 && filters[0] && !["all", "unreplied", "messenger", "instagram", "live_comment"].includes(filters[0])) return error(400, "invalid_filter");
    return error(path === "inbox/buyer-panel" && request.method === "GET" ? 400 : 422, "invalid_request");
  }
  const operation = operationRoute(request.method, path);
  if (operation && !validOperationsRequest(operation, request)) return error(422,"invalid_request");
  const health = metaHealthRoute(request.method, path);
  if (health && !validMetaHealthRequest(request)) return error(422, "invalid_request");
  // Refund/shipment/export/permission resources (orders-request.ts grammar) -> Go refunds.go/shipments.go.
  const action = orderActionRoute(request.method, path);
  // customers-billing-ui: customers/finance/billing resources (lib/customers-request.ts grammar) -> Go customers.go/finance.go/billing.go.
  const customers = customersRoute(request.method, path);
  // W4-U1: platform card payments + settlements (lib/card-payments-request.ts grammar; PUT is keyless CAS).
  const cardPayments = cardPaymentsRoute(request.method, path);
  // taiwan-cvs-logistics-v1 §8/§16.5: GET|PUT logistics/ecpay, POST logistics/ecpay/enabled, GET|PUT logistics/cvs-settings.
  // storefront-v2 §F (unit promotions): GET|POST promotions, POST promotions/{id} share the logistics exact-resource policy (no query, keyed JSON).
  const logistic = logisticsRoute(request.method, path) ?? promotionsRoute(request.method, path);
  const studio = path.startsWith("live-sessions");
  if (studio && !authConfig) return error(404, "not_found");
  const input = studioInputRoute.test(path);
  // Trusted deployment origin, not forwarded headers or fixture auth, controls
  // the browser secret boundary. The Go runtime stays explicitly injected/off.
  if (input && !authConfig?.publicOrigin.startsWith("https://")) return error(404, "not_found");
  if (exactStore.test(store) && studio && studioAny.test(path) && !routes[request.method]?.test(path))
    return error(405, "method_not_allowed", "GET, POST, PATCH, PUT");
  if (!exactStore.test(store) || (!routes[request.method]?.test(path) && !action && !customers && !cardPayments && !logistic && !health && !inbox && !operation))
    return error(404, "not_found");
  const order = request.method === "GET" && orderRoute.test(path);
  const orderSearch = request.method === "POST" && path === "orders/search";
  if (orderSearch && (!validOrdersQuery(request.url, false) || !new URL(request.url).searchParams.has("view") || request.headers.has("idempotency-key") || request.headers.has("transfer-encoding") || Number(request.headers.get("content-length") ?? "0") > 1024)) return error(422,"invalid_request");
  const accountRoute = path.startsWith("provider-accounts");
  const inspection = request.method === "POST" && inspectRoute.test(path);
  // Ads (adsRoutes): session/store authority only, and only `ads/report` may carry a query (from,to).
  const ads = adsAny.test(path);
  if (path === "ads/catalog-feed" && (request.body !== null || request.headers.has("transfer-encoding") || request.headers.has("idempotency-key") || (request.headers.has("content-length") && request.headers.get("content-length") !== "0"))) return error(422,"invalid_request");
  if (ads && !validAdsQuery(request.url, path)) return error(422, "invalid_request");
  // Merchant Page/Instagram connect (meta-connect-request.ts): exact resources, no query; a read carries no body or key.
  if (metaConnectAny.test(path) && !validMetaConnectRequest(request)) return error(422, "invalid_request");
  // New setup routes require actual session/store authority, never a shared fixture.
  if ((operation || health || studio || order || orderSearch || action || customers || cardPayments || logistic || accountRoute || ads || discoveryRoute.test(path)) && !authConfig)
    return error(404, "not_found");
  // Query grammar, Idempotency-Key presence and empty/JSON body declaration (keyless: billing POSTs; bodyless: export, portal).
  if (customers && !validCustomersRequest(customers, request)) return error(422, "invalid_request");
  // Card payments/settlements grammar: exact query rules, keyless CAS PUT, bodyless GETs.
  if (cardPayments && !validCardPaymentsRequest(cardPayments, request)) return error(422, "invalid_request");
  // Exact resources: no query at all, including a bare trailing '?'. The one exception is the returns-v1 §6
  // RMA list: GET returns may carry exactly `?state=<RMA state>` (validReturnsQuery mirrors the Go grammar).
  if ((action || logistic) && request.url.includes("?") &&
    !(request.method === "GET" && path === "returns" && validReturnsQuery(request.url)))
    return error(422, "invalid_request");
  // Reads and the keyless refresh carry no body and no key; only commands do.
  // (keyless-command = print-form: a JSON body but no Idempotency-Key.)
  if (action && action !== "command" && action !== "keyless-command" && !validKeylessRequest(action, request))
    return error(422, "invalid_request");
  if (action === "keyless-command" && !validKeylessCommandRequest(request)) return error(422, "invalid_request");
  // Logistics reads carry no body/key/transfer-encoding, like every other exact BFF read.
  if (logistic === "get" && (request.body !== null || request.headers.has("transfer-encoding") ||
    request.headers.has("idempotency-key") || (request.headers.has("content-length") && request.headers.get("content-length") !== "0")))
    return error(422, "invalid_request");
  // URL.search drops an empty trailing '?'. Exact resources must reject that too;
  // Only collection GETs inherit the bounded pagination parser in Go.
  const storefrontDomainMoveRoute = new RegExp(`^${storefrontDomainMove}$`);
  const storefront =
    path === storefrontRead ||
    path === storefrontWrite ||
    path === storefrontDomains ||
    storefrontDomainMoveRoute.test(path);
  const exactResource =
    storefront ||
    ((path === "markets" || path.startsWith("markets/")) &&
      !(request.method === "GET" && pagedSettingsRoute.test(path))) ||
    (accountRoute &&
      !(request.method === "GET" && path === "provider-accounts"));
  if (exactResource && request.url.includes("?"))
    return error(422, "invalid_request");
  const url = new URL(request.url);
  // catalog-core: a read carries no body or key; only the two list resources may carry a query, with known keys only.
  if (catalogV2Any.test(path)) {
    // Exactly one cache-buster is allowed: GET collections/{id}/image?v=<image id> (the id changes per upload).
    const cacheBuster = request.method === "GET" && collectionImageRoute.test(path) && new RegExp(`^\\?v=${uuid}$`).test(url.search);
    if (!catalogQueryRoute.test(path) || request.method !== "GET") {
      if (request.url.includes("?") && !cacheBuster) return error(422, "invalid_request");
    } else if (
      request.url.endsWith("?") ||
      [...url.searchParams.keys()].some((key) => !catalogQueryKeys.has(key)) ||
      (path === collectionsRoot && (url.searchParams.has("q") || url.searchParams.has("status")))
    )
      return error(422, "invalid_request");
    if (
      request.method === "GET" &&
      (request.body !== null || request.headers.has("transfer-encoding") || request.headers.has("idempotency-key") ||
        (request.headers.has("content-length") && request.headers.get("content-length") !== "0"))
    )
      return error(422, "invalid_request");
  }
  // Photo routes take no query; a read carries no body or key; the upload alone is multipart (checked below).
  // store-design: design/media is the same multipart upload / raw-bytes read, with its own query/body rules (design-request.ts).
  if (!validDesignRequest(request, path)) return error(422, "invalid_request");
  const designJson = isDesignJsonPut(request.method, path);
  const imageUpload = (request.method === "POST" && (imagesRootRoute.test(path) || collectionImageRoute.test(path))) || isDesignUpload(request.method, path);
  const imageBytes = (request.method === "GET" && (imageItemRoute.test(path) || collectionImageRoute.test(path))) || isDesignImageBytes(request.method, path);
  if (imagesAny.test(path)) {
    const search = request.url.includes("?") ? request.url.slice(request.url.indexOf("?")) : "";
    if (!validMediaQuery(search, request.method === "POST" && imagesRootRoute.test(path))) return error(422, "invalid_request");
    if (
      request.method === "GET" &&
      (request.body !== null || request.headers.has("transfer-encoding") || request.headers.has("idempotency-key") ||
        (request.headers.has("content-length") && request.headers.get("content-length") !== "0"))
    )
      return error(422, "invalid_request");
    if (imageUpload && Number(request.headers.get("content-length") ?? "0") > MAX_UPLOAD)
      return error(413, "invalid_request");
  }
  if (imageUpload && Number(request.headers.get("content-length") ?? "0") > MAX_UPLOAD) return error(413, "invalid_request");
  if (studio) {
    if (!validStudioQuery(request.url, request.method === "GET" && (path === "live-sessions" || claimsCollection(path))))
      return error(422, "invalid_request");
    if (
      request.method === "GET" &&
      (request.body !== null || request.headers.has("transfer-encoding") ||
        request.headers.has("idempotency-key") ||
        (request.headers.has("content-length") && request.headers.get("content-length") !== "0"))
    ) return error(422, "invalid_request");
  }
  // The storefront GET (state and domains) carries no body, key or transfer-encoding (no query: exactResource above).
  if (
    request.method === "GET" && (path === storefrontRead || path === storefrontDomains) &&
    (request.body !== null || request.headers.has("transfer-encoding") || request.headers.has("idempotency-key") ||
      (request.headers.has("content-length") && request.headers.get("content-length") !== "0"))
  )
    return error(422, "invalid_request");
  if (order) {
    if (
      request.body !== null ||
      request.headers.has("transfer-encoding") ||
      request.headers.has("idempotency-key") ||
      (request.headers.has("content-length") &&
        request.headers.get("content-length") !== "0")
    )
      return error(422, "invalid_request");
    if (!validOrdersQuery(request.url, path !== "orders"))
      return error(422, "invalid_request");
  }
  if (
    request.method === "GET" &&
    purchaseEntryRoute.test(path) &&
    (!/^\?locale=(?:en|zh-CN|zh-TW)$/.test(url.search) ||
      request.body !== null ||
      request.headers.has("idempotency-key") ||
      (request.headers.has("content-length") &&
        request.headers.get("content-length") !== "0") ||
      request.headers.has("transfer-encoding"))
  )
    return error(422, "invalid_request");
  let token: string | undefined;
  if (authConfig) {
    token = sessionToken(request) ?? undefined;
    if (!token) {
      const denied = error(401, "unauthorized");
      if (order || orderSearch || action || customers || cardPayments || logistic || inbox) clearAuthCookies(denied.headers);
      return denied;
    }
    if (
      request.method !== "GET" &&
      (!requireOrigin(request) || !requireCSRF(request))
    )
      return error(403, "forbidden");
    const listed = await authenticatedStores(token);
    if (!listed.stores) {
      const denied = await safeError(listed.response);
      if (denied.status === 401) clearAuthCookies(denied.headers);
      return denied;
    }
    if (!listed.stores.some((item) => item.id === store))
      return error(404, "not_found");
  } else {
    const session = fixtureSession();
    if (!session) return error(401, "unauthorized");
    if (store !== session.storeID) return error(404, "not_found");
    if (url.hostname !== "127.0.0.1" && url.hostname !== "localhost")
      return error(403, "forbidden");
    const host = request.headers.get("host");
    if (host !== "127.0.0.1:3100" && host !== "localhost:3100")
      return error(403, "forbidden");
  }
  const init: RequestInit = { method: request.method };
  if (request.method !== "GET" && action !== "refresh") {
    if (!authConfig) {
      const host = request.headers.get("host");
      if (request.headers.get("origin") !== `http://${host}`)
        return error(403, "forbidden");
    }
    if (
      request.headers.get("content-type")?.split(";")[0] !== (imageUpload ? "multipart/form-data" : "application/json")
    )
      return error(415, "json_required");
    const key = request.headers.get("idempotency-key") ?? "";
    // Billing POSTs are keyless by contract (§6), print-form is a keyless command, and the payments/card PUT
    // is CAS-guarded by expected_version (Go's cvsRoute keyed=false refuses a key); every other command needs its key.
    const keyless = health === "recheck" || orderSearch || customers === "checkout" || customers === "portal" || action === "keyless-command" || cardPayments === "card-write";
    if (!inspection && !keyless && !/^[A-Za-z0-9_.:-]{8,128}$/.test(key))
      return error(422, "invalid_request");
    const customersBodyless = customers === "export" || customers === "portal";
    if (customersBodyless) {
      init.body = undefined; // validCustomersRequest already required an empty declared body
    } else if (studio) {
      try {
        if (!request.body) return error(400, "invalid_json");
        init.body = await readBody(request, "application/json");
      } catch {
        return error(400, "invalid_json");
      }
    } else {
      const reader = request.body?.getReader();
      if (!reader) return error(400, "invalid_json");
      const chunks: Uint8Array[] = [];
      let size = 0;
      while (true) {
        const part = await reader.read();
        if (part.done) break;
        size += part.value.byteLength;
        if (size > (health || orderSearch ? 1024 : imageUpload ? MAX_UPLOAD : designJson ? DESIGN_MAX_JSON : 65536)) {
          await reader.cancel();
          return imageUpload ? error(413, "invalid_request") : error(400, "invalid_json");
        }
        chunks.push(part.value);
      }
      const data = new Uint8Array(size);
      let at = 0;
      for (const chunk of chunks) {
        data.set(chunk, at);
        at += chunk.length;
      }
      // The photo bytes go to Go untouched (Go sniffs the type and owns the 2 MiB rule); everything else is JSON text.
      init.body = imageUpload ? new Blob([data]) : new TextDecoder().decode(data);
    }
    if (operation === "command" && !validOperationBody(typeof init.body === "string" ? init.body : "")) return error(422,"invalid_request");
    if (path === "ads/meta/unbind" && !validAdsUnbindBody(typeof init.body === "string" ? init.body : "")) return error(422,"invalid_request");
    if (health === "recheck" && !validRecheckBody(typeof init.body === "string" ? init.body : "")) return error(400, "invalid_json");
    if (inbox && !validInboxBody(path, typeof init.body === "string" ? init.body : "")) return error(422, "invalid_request");
    // Exact bodies for the customers/billing commands (closed keys, ERASE word, consent pairs, price id).
    if (customers && !validCustomersBody(customers, typeof init.body === "string" ? init.body : "")) return error(400, "invalid_json");
    // The payments/card PUT body is the exact frozen CAS shape (card-payments-request.ts).
    if (cardPayments === "card-write" && !validCardPaymentsBody(typeof init.body === "string" ? init.body : "")) return error(400, "invalid_json");
    // PUT ads/drafts/{id} is revision-guarded: forward exactly one bare-decimal If-Match, never anything else.
    const ifMatch = request.headers.get("if-match");
    const draftPut = request.method === "PUT" && /^ads\/drafts\//.test(path);
    if (draftPut && !validIfMatch(ifMatch)) return error(422, "invalid_request");
    // ads pause/end/audience-read: the browser sends "{}", Go takes no body.
    const adsNoBody = adsBodyless.test(path);
    if (adsNoBody) {
      if (init.body !== "{}") return error(422, "invalid_request");
      delete init.body;
    }
    init.headers = {
      ...(customersBodyless || adsNoBody ? {} : { "Content-Type": imageUpload ? (request.headers.get("content-type") ?? "") : "application/json" }),
      ...(!inspection && !keyless && !adsKeyless.test(path) ? { "Idempotency-Key": key } : {}),
      ...(draftPut ? { "If-Match": ifMatch as string } : {}),
    };
  }
  const response = await callBackend(path + url.search, init, token, store);
  if (inbox) {
    // Authentication denial revokes cookies even when the upstream body is non-JSON or truncated.
    if (response.status === 401) {
      const denied = error(401, "unauthorized");
      clearAuthCookies(denied.headers);
      try { await response.body?.cancel(); } catch { /* Cookie revocation is independent of body cleanup. */ }
      return denied;
    }
    // Calls Go frozen A8-A14/template reads; body text is never logged and diagnostic payloads never escape.
    let body: string;
    let value: unknown;
    // A9 permits 50 Unicode messages; a 256 KiB ceiling rejects valid 2000-rune pages.
    try { body = await readBody(response, "application/json", 1 << 20); value = JSON.parse(body); }
    catch { return error(503, "retry_later"); }
    if (!response.ok) {
      const code = inboxErrorCode(response.status, value);
      const denied = error(code ? response.status : 503, code ?? "retry_later");
      if (response.status === 401) clearAuthCookies(denied.headers);
      const backoff = response.headers.get("retry-after") ?? "";
      if (response.status === 429 && /^(?:[1-9][0-9]{0,2}|[12][0-9]{3}|3[0-5][0-9]{2}|3600)$/.test(backoff)) denied.headers.set("Retry-After", backoff);
      return denied;
    }
    return new Response(body, {status: response.status, headers: {"Content-Type":"application/json", "Cache-Control":"private, no-store"}});
  }
  if (studio) {
    let body: string;
    try {
      const tokenResponse = input && path.endsWith("/token");
      const inputRead = studioInputReadRoute.test(path);
      const claimLink = claimLinkRoute(path);
      body = await readBody(response, "application/json", tokenResponse || inputRead || claimLink ? 8192 : 256 << 10);
      const parsed: unknown = JSON.parse(body);
      if (tokenResponse && response.ok && !validStudioInputToken(parsed)) return error(503, "retry_later");
      // The claim link token leaves the BFF only in this closed shape (§7.1 M7).
      if (claimLink && response.ok && !validClaimLink(parsed)) return error(503, "retry_later");
      if (inputRead && response.ok) {
        if (path.endsWith("/prepared")) parseStudioInputPrepared(parsed);
        else parseStudioInput(parsed);
      }
    } catch {
      return error(503, "retry_later");
    }
    if (!response.ok) {
      const denied = await safeError(new Response(body, {
        status: response.status,
        headers: { "Content-Type": "application/json", "Retry-After": response.headers.get("retry-after") ?? "" },
      }));
      if (response.status === 401) clearAuthCookies(denied.headers);
      return denied;
    }
    return new Response(body, {
      status: response.status,
      headers: { "Content-Type": "application/json", "X-Request-ID": response.headers.get("x-request-id") ?? "" },
    });
  }
  if (authConfig && response.status === 401) {
    const denied = await safeError(response);
    clearAuthCookies(denied.headers);
    return denied;
  }
  if (health && response.ok) {
    // Calls Go meta-health B1/B2 (§9): project known fields and never forward provider/debug payloads.
    try {
      if (response.status !== (health === "status" ? 200 : 202)) return error(503, "retry_later");
      const raw: unknown = JSON.parse(await readBody(response, "application/json", 128 << 10));
      return Response.json(health === "status" ? parseMetaHealth(raw) : parseRecheck(raw), { status: response.status, headers: { "Cache-Control": "private, no-store", "X-Request-ID": response.headers.get("x-request-id") ?? "" } });
    } catch { return error(503, "retry_later"); }
  }
  // W6-05B / W6-06B closed projections: Go is authoritative; only the reviewed public keys cross the BFF.
  const closedAds = path === "ads/meta/unbind" || path === "ads/catalog-feed";
  if (operation || closedAds) {
    const cache = { "Cache-Control":"private, no-store", "X-Request-ID":response.headers.get("x-request-id")??"" };
    let value: unknown;
    try { value = JSON.parse(await readBody(response,"application/json",192 << 10)); } catch { return error(503,"retry_later"); }
    if (!response.ok) {
      if (path === "ads/meta/unbind" && response.status === 409 && value && typeof value === "object" && (value as Record<string,unknown>).code === "operations_in_flight") {
        try { const details = parseAdsInFlight((value as Record<string,unknown>).details); const denied = localError(409,"operations_in_flight"); return Response.json({...await denied.json(),details},{status:409,headers:denied.headers}); } catch { return error(503,"retry_later"); }
      }
      const denied = await safeError(new Response(JSON.stringify(value),{status:response.status,headers:{"Content-Type":"application/json"}}));
      const wait = operationRetryAfter(response.headers.get("retry-after"));
      if (operation && response.status === 429 && wait) denied.headers.set("Retry-After",String(wait));
      denied.headers.set("Cache-Control","private, no-store");
      return denied;
    }
    if (response.status !== 200) return error(503,"retry_later");
    try {
      const parsed = operation === "list" ? parseOperationList(value) : operation === "detail" ? parseOperationDetail(value) : operation === "command" ? parseOperationResult(value) : path === "ads/meta/unbind" ? parseAdsUnbind(value) : parseCatalogFeed(value);
      return Response.json(parsed,{headers:cache});
    } catch { return error(503,"retry_later"); }
  }
  if (!response.ok) {
    // store-design D3: a refused document (422) keeps only the closed {path, reason} pair so the editor can mark the field.
    if (response.status === 422 && isDesignPath(path))
      return localError(422, "invalid_request", undefined, designDetails((await safeJSON<{ details?: unknown }>(response))?.details));
    return safeError(response);
  }
  if (imageBytes) {
    // Preview bytes stream straight through, only as a validated raster type, never as a document.
    const type = (response.headers.get("content-type") ?? "").split(";", 1)[0].trim().toLowerCase();
    if (response.status !== 200 || !response.body || !IMAGE_TYPES.has(type)) return error(503, "retry_later");
    return new Response(response.body, {
      status: 200,
      headers: {
        "Content-Type": type,
        "Cache-Control": "private, max-age=300",
        "Content-Security-Policy": "default-src 'none'; sandbox",
        "X-Content-Type-Options": "nosniff",
        "X-Request-ID": response.headers.get("x-request-id") ?? "",
      },
    });
  }
  if (customers === "export" || customers === "finance-csv") {
    // PII exports: streamed straight through, never buffered or stored here; only the exact attachment shape passes.
    const json = customers === "export";
    const disposition = response.headers.get("content-disposition") ?? "";
    if (
      response.status !== 200 || !response.body ||
      response.headers.get("content-type")?.toLowerCase() !== (json ? "application/json" : "text/csv; charset=utf-8") ||
      !(json ? /^attachment; filename="customer-[0-9a-f-]{36}\.json"$/ : /^attachment; filename="finance-[0-9-]{10}-[0-9-]{10}\.csv"$/).test(disposition)
    )
      return error(503, "retry_later");
    return new Response(response.body, {
      status: 200,
      headers: {
        "Content-Type": json ? "application/json" : "text/csv; charset=utf-8",
        "Content-Disposition": disposition,
        "Cache-Control": "no-store, private",
        "X-Content-Type-Options": "nosniff",
        "X-Request-ID": response.headers.get("x-request-id") ?? "",
      },
    });
  }
  if (action === "csv") {
    // PII export: streamed straight through, never buffered or stored here; refuse anything but the exact attachment.
    if (response.status !== 200 || !response.body || !validCSVHeaders(response.headers))
      return error(503, "retry_later");
    return new Response(response.body, {
      status: 200,
      headers: {
        "Content-Type": "text/csv; charset=utf-8",
        "Content-Disposition": response.headers.get("content-disposition") ?? "",
        "Cache-Control": "no-store, private",
        "X-Export-Truncated": response.headers.get("x-export-truncated") ?? "",
        "X-Content-Type-Options": "nosniff",
        "X-Request-ID": response.headers.get("x-request-id") ?? "",
      },
    });
  }
  return new Response(response.body, {
    status: response.status,
    headers: {
      "Content-Type": "application/json",
      "Cache-Control": order || orderSearch || action || customers || cardPayments || logistic || storefront || metaConnectAny.test(path) ? "private, no-store" : "no-store",
      "X-Request-ID": response.headers.get("x-request-id") ?? "",
    },
  });
}
async function proxy(request: Request, context: Context) {
  const path = (await context.params).resource.join("/");
  const response = await route(request, context);
  if (path.startsWith("live-sessions") || path.startsWith("inbox/") || path === "message-templates" || path.startsWith("operations") || path === "ads/catalog-feed" || path === "ads/meta/unbind") response.headers.set("Cache-Control", "private, no-store");
  // Every M7 answer (success or not) forbids a Referer, like the Go route (§7.1).
  if (claimLinkRoute(path)) response.headers.set("Referrer-Policy", "no-referrer");
  return response;
}
export const GET = proxy;
export const POST = proxy;
export const PATCH = proxy;
export const PUT = proxy;
const unsupported = async (_request: Request, context: Context) => {
  const path = (await context.params).resource.join("/");
  const response = (path.startsWith("live-sessions") && !authConfig) ||
    (studioInputRoute.test(path) && !authConfig?.publicOrigin.startsWith("https://"))
    ? localError(404, "not_found")
    : path.startsWith("live-sessions") && !studioAny.test(path)
    ? localError(404, "not_found")
    : localError(405, "method_not_allowed", "GET, POST, PATCH, PUT");
  if (path.startsWith("live-sessions") || path.startsWith("inbox/") || path === "message-templates")
    response.headers.set("Cache-Control", "private, no-store");
  if (claimLinkRoute(path)) response.headers.set("Referrer-Policy", "no-referrer");
  return response;
};
export const DELETE = unsupported;
export const OPTIONS = unsupported;
export const HEAD = unsupported;
