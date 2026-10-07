// Purpose: BFF request grammar for the W3-07B parcel-group routes (manual-fulfilment-v1 Amendment W3-07B):
//   GET orders/merge-suggestions, POST parcel-groups, DELETE parcel-groups/{id}?expected_version=N,
//   PUT parcel-groups/{id}/shipment -> Go internal/httpapi/parcels.go. No generic proxying: every resource is listed.
// Depends on: nothing (pure grammar).
// Used by: apps/admin/app/api/stores/[store]/[...resource]/route.ts; tests/admin/parcels-request.test.ts.
// Invariants: GET/DELETE carry no Idempotency-Key and no body (Go refuses both on DELETE); the dissolve query is exactly
//   one expected_version (Go's DELETE gate rejects anything else); POST/PUT are keyed JSON commands like order actions.

export type ParcelRouteKind = "get" | "command" | "delete";

const uuid = "[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}";
const parcelRoutes: [string, RegExp, ParcelRouteKind][] = [
  ["GET", /^orders\/merge-suggestions$/, "get"],
  ["POST", /^parcel-groups$/, "command"],
  ["DELETE", new RegExp(`^parcel-groups/${uuid}$`), "delete"],
  ["PUT", new RegExp(`^parcel-groups/${uuid}/shipment$`), "command"],
];

/** Map one method+path to its parcel route kind; null when the pair is not a W3-07B resource. */
export function parcelRoute(method: string, path: string): ParcelRouteKind | null {
  return parcelRoutes.find(([m, re]) => m === method && re.test(path))?.[2] ?? null;
}

// The dissolve CAS token: exactly one expected_version, a bare decimal >= 1 (Go strconv.ParseInt rejects 0/negative;
// the BFF caps at 18 digits so no int64 overflow reaches Go). Inspect the raw URL before Next.js drops a bare '?'.
export function validParcelDeleteQuery(rawURL: string) {
  const at = rawURL.indexOf("?");
  if (at < 0) return false;
  return /^\?expected_version=[1-9][0-9]{0,17}$/.test(rawURL.slice(at));
}
