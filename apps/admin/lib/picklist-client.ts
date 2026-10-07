// Purpose: explicit private pick-list/download/batch requests with session fences and unknown-write safety.
// Depends on: settings CSRF/session helpers, exact BFF routes and picklist-model projections.
// Used by: PickList, CarrierExport and CvsBatch; no automatic retries or PII storage.
import { csrfCookie, sessionBoundary, safeError } from "./settings-client";
import {
  validSelection,
  pickUUID,
  parsePickList,
  parseCvsBatch,
  carrierFilename,
  type PickSelection,
  type PickDocument,
  type CarrierTemplate,
  type BatchResult,
  type PickKind,
} from "./picklist-model";
/** Marks whether a submitted shipment command may have taken effect. */
export class PickError extends Error {
  constructor(
    public code: string,
    public uncertain = false,
  ) {
    super(code);
  }
}
async function send(
  store: string,
  selection: PickSelection,
  boundary: string,
  kind: PickKind,
  key?: string,
  template?: CarrierTemplate,
): Promise<Response> {
  if (
    !pickUUID.test(store) ||
    !validSelection(selection, kind === "cvs-batch" ? 100 : 500)
  )
    throw new PickError("invalid_request");
  const csrf = csrfCookie();
  try {
    if (!csrf || (await sessionBoundary(csrf)) !== boundary) throw new Error();
  } catch {
    throw new PickError("unauthorized");
  }
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    "X-CSRF-Token": csrf,
  };
  if (key) headers["Idempotency-Key"] = key;
  let response: Response;
  try {
    response = await fetch(
      `/api/stores/${store}/${kind === "cvs-batch" ? "shipments/cvs-batch" : `orders/${kind}`}${template ? `?template=${template}` : ""}`,
      {
        method: "POST",
        credentials: "same-origin",
        cache: "no-store",
        redirect: "error",
        headers,
        body: JSON.stringify(selection),
        signal: AbortSignal.timeout(30000),
      },
    );
    if (csrfCookie() !== csrf || (await sessionBoundary(csrf)) !== boundary)
      throw new PickError("unauthorized", kind === "cvs-batch");
  } catch (e) {
    if (e instanceof PickError) throw e;
    throw new PickError("retry_later", kind === "cvs-batch");
  }
  if (!response.ok) {
    const error = safeError(await response.json().catch(() => null));
    throw new PickError(
      error.code,
      kind === "cvs-batch" &&
        (response.status >= 500 || response.status === 408),
    );
  }
  if (
    response.status !== 200 ||
    !response.headers.get("cache-control")?.includes("no-store")
  )
    throw new PickError("invalid_response", kind === "cvs-batch");
  return response;
}
/** Read one PII-free print projection. */
export async function readPickList(
  store: string,
  selection: PickSelection,
  boundary: string,
): Promise<PickDocument> {
  const response = await send(store, selection, boundary, "pick-list");
  return parsePickList(await response.json(), selection);
}
/** Download unmodified carrier bytes with the backend's checked attachment filename. */
export async function exportCarrier(
  store: string,
  selection: PickSelection,
  boundary: string,
  template: CarrierTemplate,
): Promise<{ blob: Blob; filename: string }> {
  const response = await send(
    store,
    selection,
    boundary,
    "export",
    undefined,
    template,
  );
  const filename = carrierFilename(response.headers, template, store);
  const blob = await response.blob();
  if ((await sessionBoundary()) !== boundary)
    throw new PickError("unauthorized");
  return { blob, filename };
}
/** Submit one confirmed batch; malformed replies are uncertain, never permission to resend. */
export async function createCvsBatch(
  store: string,
  ids: string[],
  boundary: string,
  key: string,
): Promise<BatchResult> {
  const response = await send(
    store,
    { order_ids: ids },
    boundary,
    "cvs-batch",
    key,
  );
  try {
    return parseCvsBatch(await response.json(), ids);
  } catch {
    throw new PickError("invalid_response", true);
  }
}
