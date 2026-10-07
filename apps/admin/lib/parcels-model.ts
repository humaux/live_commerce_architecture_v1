// Purpose: strict exact-key parsers for the W3-07B parcel-group DTOs (BFF parcel routes -> Go internal/httpapi/parcels.go
//   -> internal/merchantorders/parcels.go; manual-fulfilment-v1 Amendment W3-07B) and the pure reconcile of the session's group
//   panels with the server's OPEN-groups read (W3-U4, migration 0164), plus groupOfOrder (OPEN group before retained history). The server stays the authority for every write and for the
//   recipient mask (suggestions and group members both arrive masked); these parsers only refuse malformed reads, so a drifting
//   Go response fails closed.
// Depends on: ./orders-model.ts (canonicalUUID, parseShipmentVersion, ShipmentVersion).
// Used by: apps/admin/lib/parcels-client.ts; apps/admin/components/ParcelGroup.tsx; tests/admin/parcels-model.test.ts.
// Invariants: a group is 2..20 distinct orders (0146); create answers OPEN with members, dissolve DISSOLVED without members,
//   ship SHIPPED with one shipment version per member; the OPEN-groups read lists OPEN groups only, each order in at most one
//   group, members with masked recipients (never a full name). The browser never receives a full recipient name from either
//   read: both carry recipient_masked (orders-list rule, SQL side) and a value of any other shape is refused. Merging parcels
//   never touches money (I05).
import { canonicalUUID, parseShipmentVersion, type ShipmentVersion } from "./orders-model.ts";

export const parcelGroupStates = ["OPEN", "SHIPPED", "DISSOLVED"] as const;
export type ParcelGroupState = (typeof parcelGroupStates)[number];

// One "same buyer, same address" candidate set the merchant may confirm. Only the MASKED recipient is kept (list rule).
export type MergeSuggestion = { recipient_masked: string; order_ids: string[] };
// One OPEN group of GET parcel-groups: display fields only (masked recipient), newest first.
export type OpenParcelMember = { order_id: string; order_number: string; recipient_masked: string };
export type OpenParcelGroup = { group_id: string; version: number; created_at: string; members: OpenParcelMember[] };
// What a group panel needs: from create/ship answers during the session, from GET parcel-groups after a reload.
export type ParcelGroupView = {
  id: string;
  short: string;
  state: ParcelGroupState;
  version: number;
  orderIDs: string[];
  members?: OpenParcelMember[];
};
// The group projection. order_ids is present on create.
export type ParcelGroup = { id: string; state: ParcelGroupState; version: number; order_ids: string[] };
export type ParcelGroupDissolved = { id: string; state: "DISSOLVED"; version: number };
export type ParcelShipment = { order_id: string; shipment: ShipmentVersion };
export type ParcelGroupShipment = { id: string; state: "SHIPPED"; version: number; shipments: ParcelShipment[] };

// The closed PT409 refusal set of the 0146 definers plus the single-shipment guard (merchantorders/parcels.go parcelCodes
// + ErrInParcelGroup); parcels-copy.ts carries one message per code, pinned by tests/admin/parcels-model.test.ts.
export const parcelRefusalCodes = [
  "cod_not_mergeable",
  "cvs_not_mergeable",
  "not_mergeable",
  "already_in_group",
  "owner_mismatch",
  "destination_mismatch",
  "group_not_open",
  "group_incomplete",
  "in_parcel_group",
] as const;
export type ParcelRefusalCode = (typeof parcelRefusalCodes)[number];

function exact(value: unknown, keys: string[]): Record<string, unknown> {
  if (!value || typeof value !== "object" || Array.isArray(value)) throw new Error("unavailable");
  const result = value as Record<string, unknown>;
  if (Object.keys(result).sort().join(",") !== [...keys].sort().join(",")) throw new Error("unavailable");
  return result;
}

function orderIDs(value: unknown): string[] {
  if (!Array.isArray(value) || value.length < 2 || value.length > 20) throw new Error("unavailable");
  if (!value.every((id) => typeof id === "string" && canonicalUUID.test(id))) throw new Error("unavailable");
  if (new Set(value).size !== value.length) throw new Error("unavailable");
  return value as string[];
}

function version(value: unknown): number {
  if (!Number.isSafeInteger(value) || (value as number) < 1) throw new Error("unavailable");
  return value as number;
}

// recipient_masked as the server sends it (orders-list rule, migrations 0110/0164): "—" or exactly one character + "***". The UI
// only validates the shape; any other value (a full name from a drifting server) fails closed instead of being shown.
function maskedRecipient(value: unknown): string {
  if (typeof value !== "string" || (value !== "—" && (Array.from(value).length !== 4 || !value.endsWith("***"))))
    throw new Error("unavailable");
  return value;
}

/** GET orders/merge-suggestions: {items:[{recipient_masked, order_ids}]}; at most 100 suggestions of 2..20 orders (0146 caps, mask 0164). */
export function parseMergeSuggestions(value: unknown): MergeSuggestion[] {
  const v = exact(value, ["items"]);
  if (!Array.isArray(v.items) || v.items.length > 100) throw new Error("unavailable");
  return v.items.map((raw) => {
    const s = exact(raw, ["recipient_masked", "order_ids"]);
    return { recipient_masked: maskedRecipient(s.recipient_masked), order_ids: orderIDs(s.order_ids) };
  });
}

const isoInstant = /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d{1,9})?(?:Z|[+-]\d{2}:\d{2})$/;

/**
 * GET parcel-groups (migration 0164): {items:[{group_id, version, created_at, members:[{order_id, order_number, recipient_masked}]}]};
 * at most 200 OPEN groups of 2..20 members; a group id or an order id appears once in the whole answer; order_number is LC-<id>
 * and recipient_masked is "—" or one character + "***" (the list rule). Anything else fails closed.
 */
export function parseOpenParcelGroups(value: unknown): OpenParcelGroup[] {
  const v = exact(value, ["items"]);
  if (!Array.isArray(v.items) || v.items.length > 200) throw new Error("unavailable");
  const groupsSeen = new Set<string>();
  const ordersSeen = new Set<string>();
  return v.items.map((raw): OpenParcelGroup => {
    const g = exact(raw, ["group_id", "version", "created_at", "members"]);
    if (typeof g.group_id !== "string" || !canonicalUUID.test(g.group_id) || groupsSeen.has(g.group_id) ||
      typeof g.created_at !== "string" || !isoInstant.test(g.created_at) || Number.isNaN(Date.parse(g.created_at)) ||
      !Array.isArray(g.members) || g.members.length < 2 || g.members.length > 20)
      throw new Error("unavailable");
    groupsSeen.add(g.group_id);
    const members = g.members.map((m): OpenParcelMember => {
      const o = exact(m, ["order_id", "order_number", "recipient_masked"]);
      if (typeof o.order_id !== "string" || !canonicalUUID.test(o.order_id) || ordersSeen.has(o.order_id) ||
        o.order_number !== `LC-${o.order_id.replaceAll("-", "").toUpperCase()}`)
        throw new Error("unavailable");
      ordersSeen.add(o.order_id);
      return { order_id: o.order_id, order_number: o.order_number, recipient_masked: maskedRecipient(o.recipient_masked) };
    });
    return { group_id: g.group_id, version: version(g.version), created_at: g.created_at, members };
  });
}

/**
 * The group an order belongs to for the row badge and the single-order block: the OPEN group wins over retained SHIPPED/DISSOLVED
 * history (a dissolved panel stays until closed while its orders may already sit in a new OPEN group); among history the latest
 * entry wins. Returns undefined for an order in no known group.
 */
export function groupOfOrder(groups: ParcelGroupView[], orderID: string): ParcelGroupView | undefined {
  const mine = groups.filter((g) => g.orderIDs.includes(orderID));
  return mine.find((g) => g.state === "OPEN") ?? mine[mine.length - 1];
}

/**
 * Reconcile the session's group panels with a fresh GET parcel-groups answer (the server is the authority for OPEN groups):
 * a local OPEN group the server no longer lists is gone (shipped, dissolved or cancelled elsewhere) and is dropped; a listed one
 * takes the server's version and members (so the dissolve CAS uses the live version); an OPEN group the session did not know
 * (page reload, another tab) is added after the known ones in the server's newest-first order. A local SHIPPED/DISSOLVED panel is
 * session history until the merchant closes it and is never touched.
 */
export function reconcileGroups(local: ParcelGroupView[], open: OpenParcelGroup[]): ParcelGroupView[] {
  const view = (g: OpenParcelGroup): ParcelGroupView => ({
    id: g.group_id,
    short: parcelShort(g.group_id),
    state: "OPEN",
    version: g.version,
    orderIDs: g.members.map((m) => m.order_id),
    members: g.members,
  });
  const byID = new Map(open.map((g) => [g.group_id, g]));
  const known = new Set(local.map((g) => g.id));
  const kept = local.flatMap((g) => (g.state !== "OPEN" ? [g] : byID.has(g.id) ? [view(byID.get(g.id)!)] : []));
  return [...kept, ...open.filter((g) => !known.has(g.group_id)).map(view)];
}

/** POST parcel-groups 201: exactly {id, state:"OPEN", version, order_ids}. */
export function parseParcelGroupCreated(value: unknown): ParcelGroup {
  const v = exact(value, ["id", "state", "version", "order_ids"]);
  if (typeof v.id !== "string" || !canonicalUUID.test(v.id) || v.state !== "OPEN") throw new Error("unavailable");
  return { id: v.id, state: "OPEN", version: version(v.version), order_ids: orderIDs(v.order_ids) };
}

/** DELETE parcel-groups/{id} 200: exactly {id, state:"DISSOLVED", version} (no members: membership rows are gone). */
export function parseParcelGroupDissolved(value: unknown): ParcelGroupDissolved {
  const v = exact(value, ["id", "state", "version"]);
  if (typeof v.id !== "string" || !canonicalUUID.test(v.id) || v.state !== "DISSOLVED") throw new Error("unavailable");
  return { id: v.id, state: "DISSOLVED", version: version(v.version) };
}

/** PUT parcel-groups/{id}/shipment 200: the SHIPPED group and one SHIPPED merchant-visible shipment version per member. */
export function parseParcelGroupShipped(value: unknown): ParcelGroupShipment {
  const v = exact(value, ["id", "state", "version", "shipments"]);
  if (typeof v.id !== "string" || !canonicalUUID.test(v.id) || v.state !== "SHIPPED" ||
    !Array.isArray(v.shipments) || v.shipments.length < 2 || v.shipments.length > 20)
    throw new Error("unavailable");
  const shipments = v.shipments.map((raw): ParcelShipment => {
    const s = exact(raw, ["order_id", "shipment"]);
    if (typeof s.order_id !== "string" || !canonicalUUID.test(s.order_id)) throw new Error("unavailable");
    const shipment = parseShipmentVersion(s.shipment, true) as ShipmentVersion;
    if (shipment.status !== "SHIPPED") throw new Error("unavailable");
    return { order_id: s.order_id, shipment };
  });
  if (new Set(shipments.map((s) => s.order_id)).size !== shipments.length) throw new Error("unavailable");
  return { id: v.id, state: "SHIPPED", version: version(v.version), shipments };
}

/** The display short code of a group id (first UUID block); never authority, only a label. */
export function parcelShort(id: string): string {
  return id.slice(0, 8);
}
