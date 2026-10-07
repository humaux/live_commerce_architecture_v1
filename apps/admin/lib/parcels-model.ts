// Purpose: strict exact-key parsers for the W3-07B parcel-group DTOs (BFF parcel routes -> Go internal/httpapi/parcels.go
//   -> internal/merchantorders/parcels.go; manual-fulfilment-v1 Amendment W3-07B). The server stays the authority for every
//   write; these parsers only refuse malformed reads, so a drifting Go response fails closed instead of painting bad state.
// Depends on: ./orders-model.ts (canonicalUUID, parseShipmentVersion, ShipmentVersion).
// Used by: apps/admin/lib/parcels-client.ts; tests/admin/parcels-model.test.ts.
// Invariants: a group is 2..20 distinct orders (0146); create answers OPEN with members, dissolve DISSOLVED without members,
//   ship SHIPPED with one shipment version per member. Merging parcels never touches money (I05).
import { canonicalUUID, parseShipmentVersion, type ShipmentVersion } from "./orders-model.ts";

export const parcelGroupStates = ["OPEN", "SHIPPED", "DISSOLVED"] as const;
export type ParcelGroupState = (typeof parcelGroupStates)[number];

// One "same buyer, same address" candidate set the merchant may confirm.
export type MergeSuggestion = { recipient_name: string; order_ids: string[] };
// The group projection. order_ids is present on create (the UI learns membership only here: no group list route exists).
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

function recipient(value: unknown): string {
  if (
    typeof value !== "string" || value.length === 0 || Array.from(value).length > 120 ||
    /[\p{C}\p{Zl}\p{Zp}]/u.test(value) ||
    /[\ud800-\udbff](?![\udc00-\udfff])|(?<![\ud800-\udbff])[\udc00-\udfff]/.test(value)
  )
    throw new Error("unavailable");
  return value;
}

/** GET orders/merge-suggestions: {items:[{recipient_name, order_ids}]}; at most 100 suggestions of 2..20 orders (0146 caps). */
export function parseMergeSuggestions(value: unknown): MergeSuggestion[] {
  const v = exact(value, ["items"]);
  if (!Array.isArray(v.items) || v.items.length > 100) throw new Error("unavailable");
  return v.items.map((raw) => {
    const s = exact(raw, ["recipient_name", "order_ids"]);
    return { recipient_name: recipient(s.recipient_name), order_ids: orderIDs(s.order_ids) };
  });
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
