// Purpose: BFF request grammar for the nine W6-01B tag/note resources (customers-billing-v1 Amendment) plus the
//   tag-filtered customer list: BFF `/api/stores/{store}/customers/tags*`, `customers/{id}/tags`, `customers/{id}/notes*`
//   and `GET customers?tag=` -> Go internal/httpapi/customer_tags.go + customers.go. No generic proxying: every resource
//   is listed here. Idempotency-Key is required on all seven writes (Go customerRoute demands exactly one on every
//   non-GET, including both DELETEs); the two DELETEs declare an empty body; reads carry no query except the two lists.
// Depends on: ./customer-tags-model.ts (validTagName, validNoteBody), ./customers-model.ts (tagColors).
// Used by: apps/admin/app/api/stores/[store]/[...resource]/route.ts, tests/admin/customer-tags-bff.test.ts
import { validNoteBody, validTagName } from "./customer-tags-model.ts";
import { tagColors } from "./customers-model.ts";

const uuid = "[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}";
const uuidPattern = new RegExp(`^${uuid}$`);

export type CustomerTagsRouteKind =
  | "customer-list"
  | "tag-list"
  | "tag-create"
  | "tag-patch"
  | "tag-delete"
  | "owner-tags"
  | "note-list"
  | "note-create"
  | "note-patch"
  | "note-delete";

const routes: [string, RegExp, CustomerTagsRouteKind][] = [
  ["GET", /^customers\/tags$/, "tag-list"],
  ["POST", /^customers\/tags$/, "tag-create"],
  ["PATCH", new RegExp(`^customers/tags/${uuid}$`), "tag-patch"],
  ["DELETE", new RegExp(`^customers/tags/${uuid}$`), "tag-delete"],
  ["PUT", new RegExp(`^customers/${uuid}/tags$`), "owner-tags"],
  ["GET", new RegExp(`^customers/${uuid}/notes$`), "note-list"],
  ["POST", new RegExp(`^customers/${uuid}/notes$`), "note-create"],
  ["PATCH", new RegExp(`^customers/${uuid}/notes/${uuid}$`), "note-patch"],
  ["DELETE", new RegExp(`^customers/${uuid}/notes/${uuid}$`), "note-delete"],
];

// `search` is the raw URL search string ("" when absent). GET customers is claimed here only when a tag filter is
// present; without one, lib/customers-request.ts stays the owner of the list grammar (one resource, one grammar).
export function customerTagsRoute(method: string, path: string, search?: string): CustomerTagsRouteKind | null {
  const found = routes.find(([m, re]) => m === method && re.test(path))?.[2] ?? null;
  if (found) return found;
  if (method === "GET" && path === "customers" && search && search.startsWith("?") &&
    search.slice(1).split("&").some((segment) => segment.startsWith("tag=")))
    return "customer-list";
  return null;
}

const keyPattern = /^[A-Za-z0-9_.:-]{8,128}$/;
const keyed: readonly CustomerTagsRouteKind[] = ["tag-create", "tag-patch", "tag-delete", "owner-tags", "note-create", "note-patch", "note-delete"];
const withBody: readonly CustomerTagsRouteKind[] = ["tag-create", "tag-patch", "owner-tags", "note-create", "note-patch"];

// Query fence. customer-list: the customers list grammar plus tag=<uuid>; note-list: limit/after only; every other
// resource is exact (no query at all, including a bare trailing '?').
export function validCustomerTagsQuery(kind: CustomerTagsRouteKind, rawURL: string): boolean {
  const at = rawURL.indexOf("?");
  if (at < 0) return kind !== "customer-list"; // customer-list needs its tag
  if (kind !== "customer-list" && kind !== "note-list") return false;
  const seen = new Map<string, string>();
  for (const segment of rawURL.slice(at + 1).split("&")) {
    const match = (kind === "note-list" ? /^(limit|after)=([^=#+]*)$/ : /^(limit|after|q|tag)=([^=#+]*)$/).exec(segment);
    if (!match || seen.has(match[1])) return false;
    seen.set(match[1], match[2]);
  }
  const limit = seen.get("limit");
  const after = seen.get("after");
  if (limit !== undefined && !/^(?:[1-9]|[1-9][0-9]|100)$/.test(limit)) return false;
  if (after !== undefined && !/^[A-Za-z0-9_-]{1,1024}$/.test(after)) return false;
  if (kind === "note-list") return true;
  const tag = seen.get("tag");
  if (tag === undefined || !uuidPattern.test(tag)) return false;
  const q = seen.get("q");
  if (q !== undefined) {
    let text: string;
    try {
      text = decodeURIComponent(q);
    } catch {
      return false;
    }
    if (text.trim() === "" || Array.from(text).length > 40 || /[\p{C}\p{Zl}\p{Zp}]/u.test(text)) return false;
  }
  return true;
}

// Headers/body presence fence. Body *content* is checked by validCustomerTagsBody after the BFF reads it. The DELETEs
// declare an empty body (the shared non-GET gate still requires the JSON content type; route.ts strips both).
export function validCustomerTagsRequest(kind: CustomerTagsRouteKind, request: Request): boolean {
  if (!validCustomerTagsQuery(kind, request.url)) return false;
  const key = request.headers.get("idempotency-key");
  if (keyed.includes(kind) ? key === null || !keyPattern.test(key) : key !== null) return false;
  if (request.headers.has("transfer-encoding")) return false;
  const length = request.headers.get("content-length");
  if (withBody.includes(kind))
    return request.headers.get("content-type")?.split(";", 1)[0].trim().toLowerCase() === "application/json";
  return (request.method === "GET" ? request.body === null : true) && (length === null || length === "0");
}

const isColor = (value: unknown): boolean => (tagColors as readonly unknown[]).includes(value);

// Exact bodies (W6-01B frozen shapes): unknown or missing key = invalid; deletes and reads must carry no body at all.
export function validCustomerTagsBody(kind: CustomerTagsRouteKind, text: string): boolean {
  if (!withBody.includes(kind)) return text === "";
  let value: unknown;
  try {
    value = JSON.parse(text);
  } catch {
    return false;
  }
  if (!value || typeof value !== "object" || Array.isArray(value)) return false;
  const body = value as Record<string, unknown>;
  const keys = Object.keys(body).sort().join(",");
  if (kind === "tag-create") return keys === "color,name" && isColor(body.color) && validTagName(body.name);
  if (kind === "tag-patch") {
    if (keys !== "color" && keys !== "name" && keys !== "color,name") return false;
    return (!("name" in body) || validTagName(body.name)) && (!("color" in body) || isColor(body.color));
  }
  if (kind === "owner-tags")
    return keys === "revision,tag_ids" && typeof body.revision === "string" && /^[0-9a-f]{64}$/.test(body.revision) &&
      Array.isArray(body.tag_ids) && body.tag_ids.length <= 100 &&
      body.tag_ids.every((tagID) => typeof tagID === "string" && uuidPattern.test(tagID));
  if (kind === "note-create") return keys === "body" && validNoteBody(body.body);
  return keys === "body,version" && validNoteBody(body.body) && Number.isSafeInteger(body.version) && (body.version as number) >= 1;
}
