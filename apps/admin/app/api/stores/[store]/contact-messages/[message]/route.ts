// Purpose: private merchant contact-detail BFF.
// Depends on: live-settings-proxy and its closed contact response projection.
// Used by: merchant contact clients; Go rechecks the signed inbox:read grant.
// Invariants: store comes only from the signed session; PII response is private no-store.
import { liveSettingsProxy } from "@/lib/live-settings-proxy";

async function handle(request: Request, context: {params: Promise<{store: string; message: string}>}) {
  const p = await context.params;
  return liveSettingsProxy(request, p.store, `contact-messages/${p.message}`, fetch);
}
export {handle as GET, handle as PUT, handle as POST, handle as PATCH, handle as DELETE, handle as HEAD, handle as OPTIONS};
