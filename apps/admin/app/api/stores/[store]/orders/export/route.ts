// Purpose: exact W3-02B export POST entry point.
// Depends on: picklistProxy authenticated request and response boundary.
// Used by: admin order selection workflow; no generic route forwarding.
import { picklistProxy } from '@/lib/picklist-proxy';
/** Forward a single scoped export request; authority is checked in the shared proxy. */
export async function POST(request:Request, context:{params:Promise<{store:string}>}){
 const {store}=await context.params;return picklistProxy(request,store,'export');
}
