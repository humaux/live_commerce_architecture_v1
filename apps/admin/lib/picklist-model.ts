// Purpose: bounded selection and closed pick-list/export/CVS wire contracts.
// Depends on: merged W3-02B HTTP DTOs; platform Headers/Request only.
// Used by: PickList, CVS/CSV clients, exact BFF routes and unit gates.
export const pickUUID =
  /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;
export const carrierTemplates = [
  "black_cat",
  "hsinchu",
  "chunghwa_post",
  "generic",
] as const;
export type CarrierTemplate = (typeof carrierTemplates)[number];
export type PickSelection = { order_ids: string[] } | { session_id: string };
export type PickLine = {
  sku_id: string;
  sku_code: string;
  title: string;
  option_label: null;
  qty: number;
};
export type PickDocument = {
  generated_at: string;
  orders: { order_id: string; order_number: string; lines: PickLine[] }[];
  totals: PickLine[];
  skipped: { order_id: string; code: "not_pickable" | "order_not_found" }[];
};
export type BatchResult = {
  results: {
    order_id: string;
    outcome: "queued" | "already" | "failed";
    code?: string;
  }[];
};
export type PickKind = "pick-list" | "export" | "cvs-batch";
const fail = (): never => {
  throw new Error("invalid_response");
};
function record(
  value: unknown,
  required: string[],
  optional: string[] = [],
): Record<string, unknown> {
  if (
    !value ||
    typeof value !== "object" ||
    Array.isArray(value) ||
    required.some((k) => !Object.hasOwn(value, k)) ||
    Object.keys(value).some((k) => ![...required, ...optional].includes(k))
  )
    return fail();
  return value as Record<string, unknown>;
}
const isID = (v: unknown): v is string =>
  typeof v === "string" && pickUUID.test(v);
const text = (v: unknown, max: number): v is string =>
  typeof v === "string" &&
  v.length > 0 &&
  [...v].length <= max &&
  !/[\p{Cc}\p{Cf}\p{Cs}]/u.test(v);
/** Accept exactly one selector; CVS only accepts explicit IDs and its own 100-order cap. */
export function validSelection(
  value: unknown,
  max = 500,
): value is PickSelection {
  try {
    const v = record(value, [], ["order_ids", "session_id"]);
    if (Object.keys(v).length !== 1) return false;
    if ("session_id" in v) return max === 500 && isID(v.session_id);
    return (
      Array.isArray(v.order_ids) &&
      v.order_ids.length > 0 &&
      v.order_ids.length <= max &&
      v.order_ids.every(isID) &&
      new Set(v.order_ids).size === v.order_ids.length
    );
  } catch {
    return false;
  }
}
/** Retain cross-page IDs; refuse an overflowing page atomically instead of silently truncating. */
export function selectOrders(
  current: readonly string[],
  ids: readonly string[],
  checked: boolean,
): string[] {
  const next = checked
    ? [...new Set([...current, ...ids])]
    : current.filter((id) => !ids.includes(id));
  if (next.length > 500 || next.some((id) => !isID(id)))
    throw new Error("too_many");
  return next;
}
function lines(value: unknown, max: number): PickLine[] {
  if (!Array.isArray(value) || value.length > max) return fail();
  return value.map((v) => {
    const r = record(v, ["sku_id", "sku_code", "title", "option_label", "qty"]);
    if (
      !isID(r.sku_id) ||
      !text(r.sku_code, 120) ||
      !text(r.title, 120) ||
      r.option_label !== null ||
      typeof r.qty !== "number" ||
      !Number.isSafeInteger(r.qty) ||
      r.qty < 1
    )
      return fail();
    return r as PickLine;
  });
}
/** Parse private, PII-free print data and bind explicit selections to the response. */
export function parsePickList(
  value: unknown,
  selection: PickSelection,
): PickDocument {
  const v = record(value, ["generated_at", "orders", "totals", "skipped"]);
  if (
    typeof v.generated_at !== "string" ||
    !/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?Z$/.test(v.generated_at) ||
    !Number.isFinite(Date.parse(v.generated_at)) ||
    !Array.isArray(v.orders) ||
    !Array.isArray(v.skipped) ||
    v.orders.length + v.skipped.length > 500
  )
    return fail();
  const orders = v.orders.map((x) => {
    const r = record(x, ["order_id", "order_number", "lines"]);
    if (
      !isID(r.order_id) ||
      r.order_number !== `LC-${r.order_id.replaceAll("-", "").toUpperCase()}`
    )
      return fail();
    const l = lines(r.lines, 50);
    if (!l.length) return fail();
    return {
      order_id: r.order_id,
      order_number: r.order_number as string,
      lines: l,
    };
  });
  const skipped = v.skipped.map((x) => {
    const r = record(x, ["order_id", "code"]);
    if (
      !isID(r.order_id) ||
      (r.code !== "not_pickable" && r.code !== "order_not_found")
    )
      return fail();
    return {
      order_id: r.order_id,
      code: r.code,
    } as PickDocument["skipped"][number];
  });
  const ids = [...orders, ...skipped].map((r) => r.order_id);
  if (
    new Set(ids).size !== ids.length ||
    ("order_ids" in selection &&
      (ids.length !== selection.order_ids.length ||
        ids.some((id) => !selection.order_ids.includes(id))))
  )
    return fail();
  return {
    generated_at: v.generated_at,
    orders,
    totals: lines(v.totals, 25000),
    skipped,
  };
}
/** Refuse missing, duplicated or foreign result rows; never infer success from HTTP status alone. */
export function parseCvsBatch(
  value: unknown,
  ids: readonly string[],
): BatchResult {
  const v = record(value, ["results"]);
  if (!Array.isArray(v.results) || v.results.length !== ids.length)
    return fail();
  const results = v.results.map((x) => {
    const r = record(x, ["order_id", "outcome"], ["code"]);
    if (
      !isID(r.order_id) ||
      !ids.includes(r.order_id) ||
      !["queued", "already", "failed"].includes(String(r.outcome)) ||
      ("code" in r &&
        (typeof r.code !== "string" || !/^[a-z0-9_]{1,64}$/.test(r.code)))
    )
      return fail();
    return r as BatchResult["results"][number];
  });
  if (new Set(results.map((r) => r.order_id)).size !== ids.length)
    return fail();
  return { results };
}
/** Exact request grammar distinguishes read-only POSTs from keyed shipment commands. */
export function validPickRequest(kind: PickKind, request: Request): boolean {
  const query = request.url.includes("?")
    ? request.url.slice(request.url.indexOf("?"))
    : "";
  const queryOK =
    kind === "export"
      ? /^\?template=(black_cat|hsinchu|chunghwa_post|generic)$/.test(query)
      : query === "";
  return (
    request.method === "POST" &&
    queryOK &&
    !request.headers.has("transfer-encoding") &&
    request.headers.get("content-type")?.split(";")[0] === "application/json" &&
    (kind === "cvs-batch"
      ? /^[A-Za-z0-9_.:-]{8,128}$/.test(
          request.headers.get("idempotency-key") ?? "",
        )
      : !request.headers.has("idempotency-key"))
  );
}
/** Use the server filename only after checking its template/store binding and safe attachment shape. */
export function carrierFilename(
  headers: Headers,
  template: CarrierTemplate,
  store: string,
): string {
  const match =
    /^attachment; filename="([a-z_]+-[0-9a-f]{1,8}-[0-9]{12}\.csv)"$/.exec(
      headers.get("content-disposition") ?? "",
    );
  if (
    !match ||
    !match[1].startsWith(`${template}-${store.slice(0, 8)}-`) ||
    headers.get("content-type")?.toLowerCase() !== "text/csv; charset=utf-8" ||
    !headers.get("cache-control")?.includes("no-store")
  )
    return fail();
  return match[1];
}
