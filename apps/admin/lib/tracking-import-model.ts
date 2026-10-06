// Purpose: closed tracking-import DTOs, exact BFF grammar and spreadsheet-to-CSV conversion.
// Depends on: immutable fa25f019 merchanttools tracking API, RFC4180 and Web TextEncoder.
// Used by: TrackingImport, tracking-import-client, tools BFF and independent Node counterexamples.
export const TRACKING_MAX_BYTES = 2 * 1024 * 1024;
export const TRACKING_MAX_ROWS = 500;
export const trackingUUID = /^[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}$/;
export type TrackingRow = { row: number; order_number: string; carrier_code: string; tracking_number: string; outcome: "apply" | "unchanged" | "failed"; code?: string };
export type TrackingPreview = { file_sha256: string; rows_total: number; apply_rows: number; unchanged_rows: number; failed_rows: number; mail_eta_hours: number; rows: TrackingRow[] };
export type TrackingCommit = { batch_id: string; applied: number; unchanged: number; failed: number; replayed: boolean };
export type TrackingRoute = "tracking-preview" | "tracking-commit" | "tracking-result";
const object = (v: unknown): v is Record<string, unknown> => !!v && typeof v === "object" && !Array.isArray(v);
const exact = (v: Record<string, unknown>, keys: string[]) => Object.keys(v).sort().join() === [...keys].sort().join();
const count = (v: unknown, max = TRACKING_MAX_ROWS): v is number => Number.isSafeInteger(v) && (v as number) >= 0 && (v as number) <= max;
const invalid = () => new Error("invalid_response");

/** Validate the whole closed preview, including row identities and consistent totals. No writes. */
export function parseTrackingPreview(v: unknown): TrackingPreview {
  if (!object(v) || !exact(v, ["file_sha256", "rows_total", "apply_rows", "unchanged_rows", "failed_rows", "mail_eta_hours", "rows"]) ||
    typeof v.file_sha256 !== "string" || !/^[0-9a-f]{64}$/.test(v.file_sha256) ||
    !count(v.rows_total) || !count(v.apply_rows) || !count(v.unchanged_rows) || !count(v.failed_rows) ||
    v.rows_total !== v.apply_rows + v.unchanged_rows + v.failed_rows || v.mail_eta_hours !== Math.ceil(v.apply_rows / 30) ||
    !Array.isArray(v.rows) || v.rows.length !== v.rows_total) throw invalid();
  const totals = { apply: 0, unchanged: 0, failed: 0 };
  for (const [i, row] of v.rows.entries()) {
    if (!object(row) || !exact(row, ["row", "order_number", "carrier_code", "tracking_number", ...(Object.hasOwn(row, "code") ? ["code"] : []), "outcome"]) ||
      row.row !== i + 1 || typeof row.order_number !== "string" || typeof row.carrier_code !== "string" || typeof row.tracking_number !== "string" ||
      [row.order_number, row.carrier_code, row.tracking_number].some(s => s.length > TRACKING_MAX_BYTES) ||
      !["apply", "unchanged", "failed"].includes(String(row.outcome))) throw invalid();
    if (row.outcome === "failed") {
      if (typeof row.code !== "string" || !/^[a-z0-9_]{1,64}$/.test(row.code)) throw invalid();
    } else if (Object.hasOwn(row, "code") || !trackingUUID.test(row.order_number) ||
      !["black_cat", "hsinchu", "chunghwa_post", "other"].includes(row.carrier_code) || !/^[A-Za-z0-9][A-Za-z0-9 -]{0,63}$/.test(row.tracking_number)) throw invalid();
    totals[row.outcome as keyof typeof totals]++;
  }
  if (totals.apply !== v.apply_rows || totals.unchanged !== v.unchanged_rows || totals.failed !== v.failed_rows) throw invalid();
  return v as unknown as TrackingPreview;
}
/** Validate a persisted commit receipt; replay keeps the original nonzero applied count. */
export function parseTrackingCommit(v: unknown, expectedApplyRows?: number): TrackingCommit {
  if (!object(v) || !exact(v, ["batch_id", "applied", "unchanged", "failed", "replayed"]) || typeof v.batch_id !== "string" || !trackingUUID.test(v.batch_id) ||
    !count(v.applied) || v.applied === 0 || !count(v.unchanged) || !count(v.failed) || v.applied + v.unchanged + v.failed > TRACKING_MAX_ROWS || typeof v.replayed !== "boolean") throw invalid();
  if (expectedApplyRows !== undefined && (!count(expectedApplyRows) || v.applied !== expectedApplyRows)) throw invalid();
  return v as unknown as TrackingCommit;
}
/** Match only the three tracking resources. The server remains the permission authority. */
export function trackingRoute(method: string, path: string): TrackingRoute | null {
  if (method === "POST" && path === "shipments/tracking-import/preview") return "tracking-preview";
  if (method === "POST" && path === "shipments/tracking-import/commit") return "tracking-commit";
  if (method === "GET" && /^shipments\/tracking-import\/[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}\/result\.csv$/.test(path)) return "tracking-result";
  return null;
}
/** Accept canonical, bounded query values only; duplicate/extra keys are refused. */
export function validTrackingQuery(kind: TrackingRoute, search: string): boolean {
  if (kind === "tracking-preview") return search === "";
  if (kind === "tracking-result") return search === "" || search === "?only=failed";
  return /^\?expected_apply_rows=(0|[1-9][0-9]{0,2})$/.test(search) && Number(search.split("=")[1]) <= TRACKING_MAX_ROWS;
}
/** Put failures first without losing source row numbers or changing the server snapshot. */
export function trackingRows(preview: TrackingPreview): TrackingRow[] {
  return [...preview.rows].sort((a, b) => Number(b.outcome === "failed") - Number(a.outcome === "failed"));
}
/** Render the existing public order number from the canonical UUID; invalid input stays visible. */
export function trackingOrderNumber(value: string): string {
  return trackingUUID.test(value) ? `LC-${value.replaceAll("-", "").toUpperCase()}` : value;
}

const headers = ["order_number", "carrier", "tracking_number", "carrier_name", "tracking_url"];
const aliases: Record<string, string> = {
  order_number: "order_number", order_id: "order_number", 訂單編號: "order_number", 订单编号: "order_number",
  carrier: "carrier", 物流商: "carrier", 承運商: "carrier", 承运商: "carrier",
  tracking_number: "tracking_number", 運單號: "tracking_number", 運單號碼: "tracking_number", 运单号: "tracking_number", 託運單號: "tracking_number",
  carrier_name: "carrier_name", 物流商名稱: "carrier_name", tracking_url: "tracking_url", 追蹤網址: "tracking_url",
};
const cell = (value: string) => `"${value.replaceAll('"', '""')}"`;

/** Convert pasted CSV/TSV to one immutable RFC4180 byte sequence; Go validates business values. */
export function pasteTrackingCSV(text: string): Uint8Array<ArrayBuffer> {
  if (new TextEncoder().encode(text).length > TRACKING_MAX_BYTES) throw new Error("too_large");
  text = text.replace(/^\uFEFF/, "");
  let quoted = false, delimiter = ",";
  for (let i = 0; i < text.length; i++) {
    if (text[i] === '"') { if (quoted && text[i + 1] === '"') i++; else quoted = !quoted; }
    if (!quoted && text[i] === "\t") { delimiter = "\t"; break; }
    if (!quoted && /[\r\n]/.test(text[i])) break;
  }
  const rows: string[][] = [], row: string[] = [];
  let field = "", inQuote = false, afterQuote = false;
  const endCell = () => { row.push(field); field = ""; afterQuote = false; };
  const endRow = () => { endCell(); if (row.some(s => s.trim())) rows.push([...row]); row.length = 0; };
  for (let i = 0; i < text.length; i++) {
    const c = text[i];
    if (inQuote) {
      if (c === '"') { if (text[i + 1] === '"') { field += '"'; i++; } else { inQuote = false; afterQuote = true; } }
      else field += c;
    } else if (c === delimiter) endCell();
    else if (c === "\r" || c === "\n") { endRow(); if (c === "\r" && text[i + 1] === "\n") i++; }
    else if (c === '"' && field === "" && !afterQuote) inQuote = true;
    else { if (afterQuote || c === '"' || c === "\0") throw new Error("invalid_csv"); field += c; }
  }
  if (inQuote) throw new Error("invalid_csv");
  if (field || row.length || afterQuote) endRow();
  if (!rows.length) throw new Error("empty");
  const first = rows[0].map(s => aliases[s.normalize("NFKC").trim().toLowerCase()] ?? s);
  const hasHeader = headers.slice(0, 3).every(name => first.includes(name));
  if (hasHeader) rows[0] = first;
  else {
    if (rows.some(r => r.length < 3 || r.length > 5)) throw new Error("invalid_csv");
    for (const r of rows) while (r.length < 5) r.push("");
    rows.unshift(headers);
  }
  if (rows.length - 1 > TRACKING_MAX_ROWS) throw new Error("too_many_rows");
  const data = new TextEncoder().encode(rows.map(r => r.map(cell).join(",")).join("\r\n") + "\r\n");
  if (data.length > TRACKING_MAX_BYTES) throw new Error("too_large");
  return data;
}

/** Downloadable template uses backend-recognized headers, never real order/customer values. */
export const trackingTemplate = () => new Blob([headers.join(",") + "\r\n"], { type: "text/csv;charset=utf-8" });
