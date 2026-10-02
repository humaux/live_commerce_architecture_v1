// BFF request grammar for the store-level logistics resources (contracts/taiwan-cvs-logistics-v1.md §8, §16.5):
// BFF `/api/stores/{store}/logistics/*` -> Go internal/httpapi/cvs.go. No generic proxying: every resource is
// listed. Order-level CVS resources (cvs-shipment, collection, pay-at-pickup-release) are in orders-request.ts.
// The route handler (`[...resource]/route.ts`, integrator hook) calls logisticsRoute() exactly like it calls
// orderActionRoute(): a null result means 404, otherwise the kind picks the header/body policy:
//   get      bare JSON read, no query/body/key
//   command  JSON body + Idempotency-Key (PUT ecpay, POST ecpay/enabled, PUT cvs-settings)
export type LogisticsKind = "get" | "command";
const routes: [string, RegExp, LogisticsKind][] = [
  ["GET", /^logistics\/ecpay$/, "get"],
  ["PUT", /^logistics\/ecpay$/, "command"],
  ["POST", /^logistics\/ecpay\/enabled$/, "command"],
  ["GET", /^logistics\/cvs-settings$/, "get"],
  ["PUT", /^logistics\/cvs-settings$/, "command"],
  // storefront-v2 §C: the store's bank-transfer settings (Go internal/httpapi/offline.go; integration:read / integration:manage).
  ["GET", /^bank-transfer-settings$/, "get"],
  ["PUT", /^bank-transfer-settings$/, "command"],
  // home-cod R5 (migration 0107): the store's cash-on-delivery settings (Go internal/httpapi/cod.go; integration:read / integration:manage).
  ["GET", /^cash-on-delivery-settings$/, "get"],
  ["PUT", /^cash-on-delivery-settings$/, "command"],
  // storefront-v2 §E6: the new-order mail opt-out (Go internal/httpapi/notify.go; integration:read / integration:manage).
  ["GET", /^notification-settings$/, "get"],
  ["PUT", /^notification-settings$/, "command"],
];
export function logisticsRoute(method: string, path: string): LogisticsKind | null {
  return routes.find(([m, re]) => m === method && re.test(path))?.[2] ?? null;
}
