// Purpose: exact merchant replay GET/PUT BFF; no UI or generic proxy.
// Depends on: liveSettingsProxy signed session, CSRF, closed replay grammar and projection.
// Used by: future merchant replay control (UI pending PARITY-NEWFEAT).
// Invariants: server-issued store grants; no retries; private no-store.
import { liveSettingsProxy } from "@/lib/live-settings-proxy";

async function handle(request: Request, context: { params: Promise<{ store: string; session: string }> }) {
  const p = await context.params;
  return liveSettingsProxy(request, p.store, `live-sessions/${p.session}/replay`, fetch);
}
export { handle as GET, handle as POST, handle as PUT, handle as PATCH, handle as DELETE, handle as HEAD, handle as OPTIONS };
