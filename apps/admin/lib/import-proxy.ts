// Purpose: exact authenticated customer/order CSV preview/commit and failed-result BFF.
// Depends on: trusted auth session/store/CSRF/Origin config, import grammar/DTO projection, bounded byte reader.
// Used by: imports/[param]/[action]/route.ts only; its native75s transport avoids generic6s helper.
// Invariants: I01 store authority, I06 no retry, I11 raw CSV/IDs never logged or shown, I15 private no-store.
import { authConfig,authenticatedStores,sessionToken,clearAuthCookies,localError,requireOrigin,requireCSRF } from "./auth";
import { canonicalUUID } from "./orders-model";
import { importRoute,validImportRequest } from "./import-request";
import { maxImportBytes,projectImportPreview,parseImportReceipt } from "./import-model";
import { readImportBytes,projectImportFailures } from "./import-bff";
const headers={"Cache-Control":"private, no-store","X-Content-Type-Options":"nosniff"};
const codes=["encoding_not_utf8","too_many_rows","required","invalid_request","nothing_to_apply","idempotency_conflict","retry_later","unauthorized","forbidden","not_found"];

/** Normalize every import-leaf response while preserving real shared error/cookie/Allow semantics. */
export function privateImportResponse(response:Response):Response {
  response.headers.set("Cache-Control",headers["Cache-Control"]);
  response.headers.set("X-Content-Type-Options",headers["X-Content-Type-Options"]);
  return response;
}

/** Authorize exactly one import operation; commit may write, preview rolls back, results are private read-only. */
export async function importProxy(request:Request,store:string,param:string,action:string,transport:typeof fetch):Promise<Response> {
  let response:Response;
  try {response=await importReply(request,store,param,action,transport);}
  catch {response=localError(503,"retry_later");}
  // Import policy is local to this leaf; shared auth.localError intentionally remains unchanged.
  return privateImportResponse(response);
}

async function importReply(request:Request,store:string,param:string,action:string,transport:typeof fetch):Promise<Response> {
  const route=importRoute(request.method,param,action);
  if(!authConfig||!canonicalUUID.test(store)||!route)return localError(404,"not_found");
  if(!validImportRequest(request,route))return localError(422,"invalid_request");
  const token=sessionToken(request);if(!token){const denied=localError(401,"unauthorized");clearAuthCookies(denied.headers);return denied;}
  const origin=request.headers.has("origin")?requireOrigin(request):request.method==="GET"&&request.headers.get("sec-fetch-site")==="same-origin";
  if(!origin||!requireCSRF(request))return localError(403,"forbidden");
  const listed=await authenticatedStores(token);
  if(!listed.stores){const denied=localError(listed.response.status===401?401:listed.response.status===403?403:503,listed.response.status===401?"unauthorized":listed.response.status===403?"forbidden":"retry_later");if(denied.status===401)clearAuthCookies(denied.headers);return denied;}
  const current=listed.stores.find((s)=>s.id===store);if(!current)return localError(404,"not_found");
  if(current.permissions&&!current.permissions.includes("customers:privacy"))return localError(403,"forbidden");
  const active=AbortSignal.any([request.signal,AbortSignal.timeout(75000)]);
  let data:Uint8Array<ArrayBuffer>|null=null;
  if(request.method==="POST"){
    data=await readImportBytes(request,maxImportBytes,active);if(!data?.length)return localError(413,"invalid_request");
  }
  if(active.aborted)return localError(503,"retry_later");
  let upstream:Response;
  try {
    // Calls migration-import-v1 §4/§7 using ONLY server session and trusted origin; file hash is idempotency key.
    upstream=await transport(`${authConfig.apiOrigin}/v1/admin/stores/${store}/imports/${param}/${action}${new URL(request.url).search}`,{
      method:request.method,headers:{Authorization:`Bearer ${token}`,Accept:request.method==="GET"?"text/csv":"application/json",...(data?{"Content-Type":"text/csv"}:{})},
      ...(data?{body:data}:{}),cache:"no-store",redirect:"error",signal:active,
    });
  }catch{return localError(503,"retry_later");}
  try {
    const parts=upstream.headers.get("cache-control")?.split(",").map((p)=>p.trim())??[];
    const attachment=route.action==="results.csv"&&upstream.status===200;
    // Go writeAttachment overwrites studioRoute's cache header with no-store; normalize the safe BFF to private.
    const bytes=await readImportBytes(upstream,8*1024*1024,active);if(!bytes||active.aborted||!parts.includes("no-store")||parts.includes("public")||(!attachment&&!parts.includes("private")))throw Error("invalid_response");
    if(route.action==="results.csv"&&upstream.status===200){
      if(upstream.headers.get("content-type")!=="text/csv; charset=utf-8"||upstream.headers.get("content-disposition")!=='attachment; filename="customer-import-results.csv"')throw Error("invalid_response");
      return new Response(projectImportFailures(bytes),{headers:{...headers,"Content-Type":"text/csv; charset=utf-8","Content-Disposition":'attachment; filename="customer-import-results.csv"'}});
    }
    if(upstream.headers.get("content-type")?.split(";",1)[0]!=="application/json")throw Error("invalid_response");
    const value:unknown=JSON.parse(new TextDecoder("utf-8",{fatal:true}).decode(bytes));
    if(route.action!=="results.csv"&&(upstream.status===200||upstream.status===409)){
      if(upstream.status===409&&value&&typeof value==="object"&&"file_sha256"in value)return Response.json(projectImportPreview(value,route.kind),{status:409,headers});
      if(upstream.status===200){
        if(route.action==="preview")return Response.json(projectImportPreview(value,route.kind),{headers});
        const receipt=parseImportReceipt(value);
        // Fresh mismatches remain UNKNOWN. A validated replay is authoritative stored evidence;
        // the client surfaces it as terminal instead of sending the identical commit a third time.
        if(!receipt.replayed&&receipt.created+receipt.updated!==Number(new URL(request.url).searchParams.get("expected_apply_rows")))throw Error("invalid_response");
        return Response.json(receipt,{headers});
      }
    }
    const code=value&&typeof value==="object"&&"code"in value?(value as {code:unknown}).code:null;
    const known=typeof code==="string"&&codes.includes(code);
    if(upstream.status<400||upstream.status>599||!known)throw Error("invalid_response");
    const denied=localError(upstream.status,code as string);if(upstream.status===401)clearAuthCookies(denied.headers);return denied;
  }catch{return localError(503,"retry_later");}
}
