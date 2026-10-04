// Pure display rule for a cart line's unit price (no BFF call of its own; used by lib/cart-details.ts
// and its node test). A live (claim-origin) price — proven by Go GET /v1/buyer/cart as
// live_unit_price_minor, never by a client value — replaces the catalog price and strikes the catalog
// price through; otherwise the catalog price (and its own compare-at) are shown. The Quote remains the
// only charge authority (I05); this decides what the buyer sees, not what they are charged.
export type LinePrice = {
  unitMinor: number;
  liveUnitMinor: number | null;
  compareAtMinor: number | null;
};

export function linePrice(
  catalogMinor: number,
  compareAtMinor: number | null,
  liveUnitMinor: number | null,
): LinePrice {
  if (liveUnitMinor !== null) {
    // The struck-through price is the catalog price (not a variant sale price), so the buyer sees both.
    return { unitMinor: liveUnitMinor, liveUnitMinor, compareAtMinor: catalogMinor };
  }
  return { unitMinor: catalogMinor, liveUnitMinor: null, compareAtMinor };
}
