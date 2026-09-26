import {
  parseOrderDetail,
  parseOrderList,
  type OrderFilter,
} from "./orders-model";

export type OrderReadCode = "signed-out" | "forbidden" | "not-found" | "unavailable";
export class OrderReadError extends Error {
  constructor(readonly code: OrderReadCode) {
    super(code);
  }
}

async function read(path: string, signal: AbortSignal): Promise<unknown> {
  let response: Response;
  try {
    response = await fetch(path, {
      method: "GET",
      cache: "no-store",
      credentials: "same-origin",
      signal,
    });
  } catch {
    throw new OrderReadError("unavailable");
  }
  if (response.status === 401) throw new OrderReadError("signed-out");
  if (response.status === 403) throw new OrderReadError("forbidden");
  if (response.status === 404) throw new OrderReadError("not-found");
  if (!response.ok || response.headers.get("content-type")?.split(";", 1)[0] !== "application/json" ||
    response.headers.get("cache-control") !== "private, no-store")
    throw new OrderReadError("unavailable");
  try {
    return await response.json();
  } catch {
    throw new OrderReadError("unavailable");
  }
}

export async function readOrderList(store: string, state: OrderFilter, cursor: string, signal: AbortSignal) {
  const query = `limit=10&state=${state}${cursor ? `&cursor=${cursor}` : ""}`;
  try {
    return parseOrderList(await read(`/api/stores/${store}/orders?${query}`, signal));
  } catch (error) {
    if (error instanceof OrderReadError) throw error;
    throw new OrderReadError("unavailable");
  }
}

export async function readOrderDetail(store: string, id: string, signal: AbortSignal) {
  try {
    return parseOrderDetail(await read(`/api/stores/${store}/orders/${id}`, signal), id);
  } catch (error) {
    if (error instanceof OrderReadError) throw error;
    throw new OrderReadError("unavailable");
  }
}
