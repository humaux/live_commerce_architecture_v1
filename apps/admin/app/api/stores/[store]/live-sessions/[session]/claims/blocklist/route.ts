// Purpose: W3-U2 exact private live-sessions/[session]/claims/blocklist route.
// Depends on: live-settings-proxy and real signed-session authority.
// Used by: LiveSettings/BuyerPanel; proxies the matching Go store endpoint.
import { liveSettingsProxy } from "@/lib/live-settings-proxy";
/** Forward one method through the closed resource grammar; unsupported methods remain private. */
async function handle(request:Request,context:{params:Promise<{store:string;session?:string;entry?:string}>}) {
 const p=await context.params; return liveSettingsProxy(request,p.store,`live-sessions/${p.session}/claims/blocklist`,fetch);
}
export {handle as GET,handle as POST,handle as PUT,handle as PATCH,handle as DELETE,handle as HEAD,handle as OPTIONS};
