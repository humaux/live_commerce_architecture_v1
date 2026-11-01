// Purpose: private merchant contact-list BFF with bounded cursor pagination.
// Depends on: live-settings-proxy's signed session/store authority and closed contact grammar.
// Used by: merchant contact clients; forwards to Go's inbox:read boundary.
// Invariants: private no-store; no fixture fallback, client scope or diagnostic forwarding.
import { liveSettingsProxy } from "@/lib/live-settings-proxy";

async function handle(request: Request, context: {params: Promise<{store: string}>}) {
  const p = await context.params;
  return liveSettingsProxy(request, p.store, "contact-messages", fetch);
}
export {handle as GET, handle as PUT, handle as POST, handle as PATCH, handle as DELETE, handle as HEAD, handle as OPTIONS};
