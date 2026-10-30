// Purpose: Exact private existing OC-05 quick-start marker GET/POST endpoint.
// Depends on: live-settings-proxy and the signed session/store/CSRF authority.
// Used by: merchant SettingsV2; identity browser CI.
// Invariants: empty keyed completion only, server-derived steps and timestamp, no fixture fallback or inferred scope.
import { liveSettingsProxy } from "@/lib/live-settings-proxy";

async function handle(request: Request, context: { params: Promise<{ store: string }> }) {
  const p = await context.params;
  return liveSettingsProxy(request, p.store, "quickstart", fetch);
}
export { handle as GET, handle as POST, handle as PUT, handle as PATCH, handle as DELETE, handle as HEAD, handle as OPTIONS };
