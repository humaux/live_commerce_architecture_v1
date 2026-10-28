// Purpose: exact private store-profile GET/PUT endpoint for settings v2.
// Depends on: live-settings-proxy and signed session/store/CSRF authority.
// Used by: merchant settings clients and the profile BFF CI gate.
// Invariants: no fixture fallback, caller scope, contact logging or automatic mutation retry.
import { liveSettingsProxy } from "@/lib/live-settings-proxy";

async function handle(request: Request, context: {params: Promise<{store: string}>}) {
  const {store} = await context.params;
  return liveSettingsProxy(request, store, "profile", fetch);
}
export {handle as GET, handle as PUT, handle as POST, handle as PATCH, handle as DELETE, handle as HEAD, handle as OPTIONS};
