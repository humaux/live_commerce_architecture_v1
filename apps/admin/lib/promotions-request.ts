// BFF request grammar for the discount-code resources (contracts/storefront-v2.md §F): BFF `/api/stores/{store}/promotions[/{id}]` -> Go
// internal/httpapi/promotions.go (pricing:read / pricing:write). No generic proxying: exactly these three resources. The catch-all route
// (`app/api/stores/[store]/[...resource]/route.ts`) calls promotionsRoute() next to logisticsRoute() and applies the same exact-resource policy:
//   get      bare JSON read, no query/body/key
//   command  JSON body + Idempotency-Key (POST create, POST update-or-pause)
// A null result means "not a promotions resource".
export type PromotionsKind = "get" | "command";
const UUID = "[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}";
const routes: [string, RegExp, PromotionsKind][] = [
  ["GET", /^promotions$/, "get"],
  ["POST", /^promotions$/, "command"],
  ["POST", new RegExp(`^promotions/${UUID}$`), "command"],
];
export function promotionsRoute(method: string, path: string): PromotionsKind | null {
  return routes.find(([m, re]) => m === method && re.test(path))?.[2] ?? null;
}
