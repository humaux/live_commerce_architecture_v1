// Purpose: Preserve the existing automatic-rule metadata when reading signed collection rows.
// Depends on: scoped customers-client get and catalog-v2-model's existing collection page parser.
// Used by: CollectionManager list; no new endpoint, transaction or write contract.
// Invariants: only the current created_at/within_days contract is accepted; raw conditions are never rendered.
import { get } from "../../lib/customers-client";
import { parseCollectionPage } from "../../lib/catalog-v2-model";

/** Read the frozen collection page plus bounded, non-PII rule metadata already returned by Go. */
export function readCollectionRows(store: string, signal: AbortSignal) {
  return get(`/api/stores/${store}/collections?limit=100`, value => {
    const page = parseCollectionPage(value);
    const raw = (value as { items: { rules?: unknown }[] }).items;
    return { ...page, items: page.items.map((item, index) => {
      const rules = raw[index].rules;
      if (rules === null || rules === undefined) return { ...item, ruleDays: [] as number[] };
      if (typeof rules !== "object" || Array.isArray(rules)) throw new Error("unavailable");
      const r = rules as Record<string, unknown>;
      if (!["all", "any"].includes(String(r.match)) || !Array.isArray(r.conditions) || r.conditions.length < 1 || r.conditions.length > 20) throw new Error("unavailable");
      const ruleDays = r.conditions.map((condition: unknown) => {
        if (!condition || typeof condition !== "object" || Array.isArray(condition)) throw new Error("unavailable");
        const c = condition as Record<string, unknown>;
        if (c.field !== "created_at" || c.operator !== "within_days" || typeof c.days !== "number" || !Number.isSafeInteger(c.days) || c.days < 1 || c.days > 3650) throw new Error("unavailable");
        return c.days;
      });
      return { ...item, ruleDays: [r.match === "all" ? Math.min(...ruleDays) : Math.max(...ruleDays)] };
    }) };
  }, signal);
}
