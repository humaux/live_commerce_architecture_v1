// Purpose: read-only, session-fenced SHOPLINE customer archive transport.
// Depends on: customers-client.get/ReadError, settings-client session fence, frozen import-history-model -> exact BFF -> Go customer_historical.go.
// Used by: CustomerHistoricalOrders; only GET, no archive storage, totals or live order actions.
import { get, ReadError } from "./customers-client";
import { csrfCookie, sessionBoundary } from "./settings-client";
import { historicalOrdersURL, parseHistoricalOrdersPage, type HistoricalOrdersPage } from "./import-history-model.ts";

/** Read one private 50-row archive page under the same session as the parent customer detail. */
export async function readHistoricalOrders(store: string, customer: string, after: string, boundary: string, signal: AbortSignal): Promise<HistoricalOrdersPage> {
  try {
    if (!boundary || await sessionBoundary() !== boundary) throw new ReadError("signed-out");
    // Calls GET historical-orders -> Go customer_historical.go, migration-import-v1 section 7; no writes or retries.
    const value = await get(historicalOrdersURL(store, customer, after), (v) => parseHistoricalOrdersPage(v, 50), signal);
    if (!csrfCookie() || await sessionBoundary() !== boundary) throw new ReadError("signed-out");
    return value;
  } catch (error) {
    if (error instanceof ReadError) throw error;
    throw new ReadError(error instanceof Error && error.message === "session_changed" ? "signed-out" : "unavailable");
  }
}
