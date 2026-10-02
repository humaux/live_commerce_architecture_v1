// Onboarding client: browser -> BFF POST /api/onboarding/handle-suggest -> Go /v1/identity/handle-suggest (R5 store-domains
// Decision 1). Read-only live handle preview for the store-name step, so no CSRF token. The reply is parsed against the closed
// shape; anything else (non-2xx, a body that fails the shape) is "handle_unconfirmed", which Entry shows as an unconfirmed preview.
import { parseHandleSuggestion } from "./storefront-handle";

export async function suggestHandle(
  storeName: string,
  signal: AbortSignal,
): Promise<{ suggested: string; available: boolean }> {
  const response = await fetch("/api/onboarding/handle-suggest", {
    method: "POST",
    credentials: "same-origin",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ store_name: storeName }),
    signal: AbortSignal.any([signal, AbortSignal.timeout(8000)]),
  });
  if (!response.ok) throw new Error("handle_unconfirmed");
  const body: unknown = await response.json();
  const suggestion = parseHandleSuggestion(body);
  if (!suggestion) throw new Error("handle_unconfirmed");
  return suggestion;
}
