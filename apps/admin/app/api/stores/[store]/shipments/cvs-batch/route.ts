// Purpose: exact W3-02B cvs-batch POST entry point.
// Depends on: picklistProxy authenticated request and response boundary.
// Used by: admin order selection workflow; no generic route forwarding.
// BFF POST /api/stores/{store}/shipments/cvs-batch -> Go POST /v1/admin/stores/{store}/shipments/cvs-batch (W3-02B).
import { picklistProxy } from '@/lib/picklist-proxy';
/** Forward a single scoped cvs-batch request; authority is checked in the shared proxy. */
export async function POST(request:Request, context:{params:Promise<{store:string}>}){
 const {store}=await context.params;return picklistProxy(request,store,'cvs-batch');
}
