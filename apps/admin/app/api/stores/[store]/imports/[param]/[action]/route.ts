// Purpose: four exact CSV import POSTs and one failed-only CSV result GET; one dynamic parameter vocabulary.
// Depends on: import-proxy trusted auth/query/byte/response boundaries.
// Used by: ImportWizard via import-client; no [kind]/[batch] sibling collision.
import { importProxy, privateImportResponse } from "@/lib/import-proxy";
import { localError } from "@/lib/auth";
type Context={params:Promise<{store:string;param:string;action:string}>};
/** Preview/commit original CSV once using authenticated server scope. */
export async function POST(request:Request,context:Context){const {store,param,action}=await context.params;return importProxy(request,store,param,action,fetch);}
/** Read failed-row results through a strict privacy projection. */
export const GET=POST;
/** HEAD must not implicitly evaluate a sensitive GET. */
export async function HEAD(){return privateImportResponse(localError(405,"method_not_allowed","GET, POST"));}
/** No other write verbs exist for imports. */
export const PUT=HEAD;
/** Reject unsupported mutations without forwarding. */
export const PATCH=HEAD;
/** Reject unsupported deletion without forwarding. */
export const DELETE=HEAD;
/** Override Next's implicit OPTIONS so unsupported replies keep the import-leaf private policy. */
export const OPTIONS=HEAD;
