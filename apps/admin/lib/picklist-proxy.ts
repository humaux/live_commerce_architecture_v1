// Purpose: three exact authenticated BFF projections for pick lists, carrier CSV and CVS batches.
// Depends on: auth origin/CSRF/store membership, callBackend and closed W3-02B model.
// Used by: /api/stores/[store]/orders/{pick-list,export} and shipments/cvs-batch routes.
import {
  authConfig,
  authenticatedStores,
  clearAuthCookies,
  localError,
  requireOrigin,
  requireCSRF,
  sessionToken,
  readBody,
  safeError,
  safeJSON,
} from "./auth";
import { callBackend } from "./backend";
import {
  validPickRequest,
  validSelection,
  pickUUID,
  carrierFilename,
  parsePickList,
  parseCvsBatch,
  type PickKind,
  type CarrierTemplate,
} from "./picklist-model";
/** Forward one bounded request with server-selected authority; never retry a shipment write. */
export async function picklistProxy(
  request: Request,
  store: string,
  kind: PickKind,
): Promise<Response> {
  if (!authConfig || !pickUUID.test(store)) return localError(404, "not_found");
  if (!validPickRequest(kind, request))
    return localError(422, "invalid_request");
  const token = sessionToken(request);
  if (!token) {
    const response = localError(401, "unauthorized");
    clearAuthCookies(response.headers);
    return response;
  }
  if (!requireOrigin(request) || !requireCSRF(request))
    return localError(403, "forbidden");
  const listed = await authenticatedStores(token);
  if (!listed.stores) {
    const denied = await safeError(listed.response);
    if (denied.status === 401) clearAuthCookies(denied.headers);
    return denied;
  }
  if (!listed.stores.some((s) => s.id === store))
    return localError(404, "not_found");
  let body: string;
  let selection: unknown;
  try {
    body = await readBody(request, "application/json", 24000);
    selection = JSON.parse(body);
  } catch {
    return localError(422, "invalid_request");
  }
  if (!validSelection(selection, kind === "cvs-batch" ? 100 : 500))
    return localError(422, "invalid_request");
  const headers = new Headers({ "Content-Type": "application/json" });
  if (kind === "cvs-batch")
    headers.set("Idempotency-Key", request.headers.get("idempotency-key")!);
  const template = new URL(request.url).searchParams.get(
    "template",
  ) as CarrierTemplate;
  const path =
    kind === "cvs-batch"
      ? "shipments/cvs-batch"
      : `orders/${kind}${kind === "export" ? `?template=${template}` : ""}`;
  // Calls W3-02B Go HTTP; read POSTs are keyless, CVS key is forwarded unchanged.
  const response = await callBackend(
    path,
    { method: "POST", headers, body },
    token,
    store,
  );
  if (!response.ok) {
    const denied = await safeError(response);
    if (denied.status === 401) clearAuthCookies(denied.headers);
    return denied;
  }
  try {
    if (
      response.status !== 200 ||
      !response.headers.get("cache-control")?.includes("no-store")
    )
      throw new Error("invalid_response");
    if (kind === "export") {
      const filename = carrierFilename(response.headers, template, store);
      return new Response(response.body, {
        headers: {
          "Content-Type": "text/csv; charset=utf-8",
          "Content-Disposition": `attachment; filename="${filename}"`,
          "Cache-Control": "private, no-store",
          "X-Content-Type-Options": "nosniff",
        },
      });
    }
    const value = await safeJSON(response);
    const data =
      kind === "pick-list"
        ? parsePickList(value, selection)
        : parseCvsBatch(
            value,
            "order_ids" in selection ? selection.order_ids : [],
          );
    return Response.json(data, {
      headers: { "Cache-Control": "private, no-store" },
    });
  } catch {
    return localError(502, "invalid_response");
  }
}
