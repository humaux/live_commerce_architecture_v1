// Purpose: closed buyer return-request routing, body admission and persisted response validation.
// Depends on: return-request-contract.ts; cookie, capability and transport authority stay in buyer-server.ts.
// Used by: buyer-server.ts and the existing GitHub CI buyer BFF suites.
// Invariants: exact routes/methods, server-issued IDs, closed projections and no automatic mutation retry.
import {
  validReturnRequestInput,
  validReturnRequestWithdraw,
  validReturnRequest,
  validReturnRequestList,
} from "./return-request-contract.ts";
export { RETURN_REQUEST_CODES } from "./return-request-contract.ts";

export type BuyerReturnBody = "returnRequest" | "returnWithdraw";
export type BuyerReturnFields = {
  returnRequest?: "create" | "list" | "detail" | "withdraw";
  requestID?: string;
};
type ReturnRoute = BuyerReturnFields & {
  method: string;
  privatePath: string;
  body?: BuyerReturnBody;
  orderID?: string;
};
type ReturnShape = { [key: string]: "string" | "integer" | [ReturnShape] };
export const buyerReturnBodies: Record<BuyerReturnBody, ReturnShape> = {
  returnRequest: {
    reason: "string",
    note: "string",
    lines: [{ sku_id: "string", warehouse_id: "string", quantity: "integer" }],
  },
  returnWithdraw: { expected_version: "integer" },
};
const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;

export function buyerReturnRoute(
  suffix: string,
  method: string,
): { known: boolean; invalidID?: boolean; route?: ReturnRoute } | undefined {
  if (suffix === "return-requests") {
    return {
      known: true,
      route: method === "GET" ? { method, privatePath: suffix, returnRequest: "list" } : undefined,
    };
  }
  const create = /^orders\/([^/]+)\/return-requests$/.exec(suffix);
  if (create) {
    if (!UUID.test(create[1])) return { known: true, invalidID: true };
    return {
      known: true,
      route: method === "POST" ? {
        method, privatePath: suffix, returnRequest: "create",
        body: "returnRequest", orderID: create[1],
      } : undefined,
    };
  }
  const item = /^return-requests\/([^/]+)(?:\/(withdraw))?$/.exec(suffix);
  if (!item) return undefined;
  if (!UUID.test(item[1])) return { known: true, invalidID: true };
  const withdraw = item[2] === "withdraw";
  return {
    known: true,
    route: method === (withdraw ? "POST" : "GET") ? {
      method, privatePath: suffix, returnRequest: withdraw ? "withdraw" : "detail",
      body: withdraw ? "returnWithdraw" : undefined, requestID: item[1],
    } : undefined,
  };
}

export function isBuyerReturnMutation(request: Request): boolean {
  return request.method === "POST" && /^\/api\/buyer\/(?:orders\/[^/]+\/return-requests|return-requests\/[^/]+\/withdraw)$/.test(new URL(request.url).pathname);
}

export function validBuyerReturnBody(shape: unknown, data: unknown): boolean {
  if (shape === buyerReturnBodies.returnRequest) return validReturnRequestInput(data);
  if (shape === buyerReturnBodies.returnWithdraw) return validReturnRequestWithdraw(data);
  return true;
}

export function validBuyerReturnResponse(target: BuyerReturnFields & { orderID?: string }, status: number, data: unknown): boolean {
  if (!target.returnRequest) return true;
  return status === (target.returnRequest === "create" ? 201 : 200) &&
    (target.returnRequest === "list" ? validReturnRequestList(data) : validReturnRequest(data, target.requestID, target.orderID));
}
