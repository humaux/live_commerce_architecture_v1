// Purpose: private keyed mark-handled BFF for a store contact message.
// Depends on: live-settings-proxy's signed session, store, origin and CSRF boundary.
// Used by: merchant contact clients; Go owns CAS, audit and replay under inbox:reply.
// Invariants: one explicit request, no automatic write retry or caller-selected tenant.
import { liveSettingsProxy } from "@/lib/live-settings-proxy";

async function handle(request: Request, context: {params: Promise<{store: string; message: string}>}) {
  const p = await context.params;
  return liveSettingsProxy(request, p.store, `contact-messages/${p.message}/handled`, fetch);
}
export {handle as GET, handle as PUT, handle as POST, handle as PATCH, handle as DELETE, handle as HEAD, handle as OPTIONS};
