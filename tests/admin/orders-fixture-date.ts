// Purpose: Selects MOU07 calendar bounds from its already admitted live fixture.
// Depends on: packages/format instantToTaipei; no wall clock or filtered results.
// Used by: orders-ui.spec.ts and registered orders-v2.test.ts regressions.
// Invariants: exactly the known fixture with a valid UTC instant; Taipei inclusive day.
import { instantToTaipei } from "../../packages/format/src/index.ts";

export function orderFixtureDates(items: unknown, orderId: string): { day: string; nextDay: string } {
  if (!orderId || !Array.isArray(items) || items.length !== 1) throw new Error("Expected one known live fixture");
  const row = items[0];
  if (!row || row.order_id !== orderId) throw new Error("Wrong live fixture identity");
  const instant = row.created_at;
  if (typeof instant !== "string" || !/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d{1,6})?Z$/.test(instant)) throw new Error("Invalid fixture timestamp");
  const parsed = new Date(instant);
  if (!Number.isFinite(parsed.getTime()) || parsed.toISOString().slice(0, 19) !== instant.slice(0, 19)) throw new Error("Invalid fixture timestamp");
  const day = instantToTaipei(instant).slice(0, 10);
  const nextDay = new Date(Date.parse(`${day}T00:00:00Z`) + 86400000).toISOString().slice(0, 10);
  return { day, nextDay };
}
