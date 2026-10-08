// Purpose: one session-fenced raw-Blob preview/commit and safe failure CSV download.
// Depends on: settings-client session/CSRF, import DTO/query grammar and native fetch; migration-import-v1 §§3/4/7.
// Used by: ImportWizard; files remain in memory, no automatic retry or browser storage.
import { csrfCookie, sessionBoundary } from "./settings-client";
import { parseImportPreview, parseImportReceipt, validImportMapping, maxImportBytes, maxImportRows, type ImportKind, type ImportMapping, type ImportPreview, type ImportReceipt } from "./import-model";
import { canonicalUUID } from "./orders-model";
export { readImportHeader } from "./import-header";
export { guessImportMapping } from "./import-request";
export { importFields, requiredImportFields } from "./import-model";

/** Definite preview/stale/receipt or a coded refusal; only post-dispatch commits may be uncertain. */
export type ImportAnswer = {kind:"preview"|"stale";value:ImportPreview}|{kind:"receipt";value:ImportReceipt}|{kind:"terminal";code:"receipt_mismatch";value:ImportReceipt}|{kind:"error";code:string;uncertain:boolean};
/** Frozen original file and mapping parameters; caller owns sticky UNKNOWN intent. */
export type ImportSend = {store:string;kind:ImportKind;action:"preview"|"commit";file:Blob;mapping:ImportMapping;expectedApplyRows?:number;boundary:string;signal:AbortSignal};
const privateResponse = (r:Response) => r.headers.get("cache-control") === "private, no-store";
const fileCodes = ["encoding_not_utf8","too_many_rows","required","invalid_request","nothing_to_apply","idempotency_conflict","retry_later","unauthorized","forbidden","not_found"];
function refusalCode(response:Response,value:unknown):string|null {
  if(!value||typeof value!=="object"||Array.isArray(value))return null;
  const v=value as Record<string,unknown>,keys=["code","message","request_id","retryable","details"];
  if(Object.keys(v).length!==keys.length||keys.some(k=>!Object.hasOwn(v,k))||typeof v.code!=="string"||!fileCodes.includes(v.code)||
    typeof v.message!=="string"||v.message.length>300||typeof v.request_id!=="string"||! /^[a-f0-9]{32}$/.test(v.request_id)||
    v.retryable!==(response.status>=500||response.status===429)||!v.details||typeof v.details!=="object"||Array.isArray(v.details)||Object.keys(v.details).length!==0)return null;
  const statuses:Record<string,readonly number[]>={unauthorized:[401],forbidden:[403],not_found:[404],idempotency_conflict:[409],
    encoding_not_utf8:[422],too_many_rows:[422],required:[422],nothing_to_apply:[422],invalid_request:[413,415,422]};
  return (v.code==="retry_later"?(response.status>=500||response.status===429):statuses[v.code]?.includes(response.status))?v.code:null;
}
async function fenced(boundary:string,csrf:string,signal:AbortSignal) {
  try {return !!boundary && !!csrf && await sessionBoundary(csrf)===boundary && csrfCookie()===csrf && !signal.aborted && document.visibilityState!=="hidden";}
  catch {return false;}
}
const error = (code:string,uncertain=false):ImportAnswer => ({kind:"error",code,uncertain});

/** Send original immutable Blob once; neither errors nor session changes resend the request. */
export async function sendImport(input:ImportSend):Promise<ImportAnswer> {
  const {store,kind,action,file,mapping,expectedApplyRows,boundary,signal}=input;
  if (!canonicalUUID.test(store) || !["customers","orders"].includes(kind) || !["preview","commit"].includes(action) || !file.size || file.size>maxImportBytes || !validImportMapping(mapping,kind) ||
    (action==="commit" && (!Number.isSafeInteger(expectedApplyRows) || expectedApplyRows!<0 || expectedApplyRows!>maxImportRows))) return error("invalid_request");
  const query=new URLSearchParams({mapping:JSON.stringify(mapping)});
  if(action==="commit")query.set("expected_apply_rows",String(expectedApplyRows));
  if(new TextEncoder().encode(query.toString()).length>4096)return error("invalid_request");
  const csrf=csrfCookie();
  const concealed=()=>signal.aborted||document.visibilityState==="hidden";
  if(!await fenced(boundary,csrf,signal)||csrfCookie()!==csrf||concealed())return error("unauthorized");
  const uncertain=action==="commit";
  try {
    // Calls migration-import-v1 §4/§7: file hash is the key; no Idempotency-Key and no CSV decode/rebuild.
    const response=await fetch(`/api/stores/${store}/imports/${kind}/${action}?${query}`,{
      method:"POST",headers:{"Content-Type":"text/csv","X-CSRF-Token":csrf},body:file,cache:"no-store",credentials:"same-origin",
      signal:AbortSignal.any([signal,AbortSignal.timeout(90000)]),
    });
    // The import leaf normalizes real localError as well as successful/private DTOs to one cache policy.
    if(!privateResponse(response)||response.headers.get("content-type")?.split(";",1)[0]!=="application/json")return error("retry_later",uncertain);
    const value:unknown=await response.json();
    if(!await fenced(boundary,csrf,signal)||csrfCookie()!==csrf||concealed())return error("unauthorized",uncertain);
    if(response.status===409 && action==="commit" && value && typeof value==="object" && "file_sha256" in value)
      return {kind:"stale",value:parseImportPreview(value,kind)};
    if(response.status===200){
      if(action==="preview")return {kind:"preview",value:parseImportPreview(value,kind)};
      const receipt=parseImportReceipt(value);
      if(receipt.created+receipt.updated!==expectedApplyRows)return receipt.replayed?{kind:"terminal",code:"receipt_mismatch",value:receipt}:error("retry_later",true);
      return {kind:"receipt",value:receipt};
    }
    const code=refusalCode(response,value);
    return code?error(code,uncertain&&response.status>=500):error("retry_later",uncertain);
  }catch{return error("retry_later",uncertain);}
}

/** Failure CSV outcome; download never writes customer data or retries implicitly. */
export type ImportDownload = "done"|"signed-out"|"forbidden"|"not-found"|"unavailable"|"uncertain";
/** Download the BFF's safe row/code CSV only after session checks before and after awaiting bytes. */
export async function downloadImportFailures({store,batch,boundary,signal}:{store:string;batch:string;boundary:string;signal:AbortSignal}):Promise<ImportDownload> {
  if(!canonicalUUID.test(store)||!canonicalUUID.test(batch))return "unavailable";
  const csrf=csrfCookie();if(!await fenced(boundary,csrf,signal))return "signed-out";
  try {
    const response=await fetch(`/api/stores/${store}/imports/${batch}/results.csv?only=failed`,{
      method:"GET",headers:{"X-CSRF-Token":csrf},cache:"no-store",credentials:"same-origin",signal:AbortSignal.any([signal,AbortSignal.timeout(90000)]),
    });
    if(response.status===401)return "signed-out";if(response.status===403)return "forbidden";if(response.status===404)return "not-found";
    if(response.status!==200 || !privateResponse(response)||response.headers.get("content-type")!=="text/csv; charset=utf-8" ||
      response.headers.get("content-disposition")!=='attachment; filename="customer-import-results.csv"')return "unavailable";
    const blob=await response.blob();if(!await fenced(boundary,csrf,signal))return "uncertain";
    // No await after this final check: cancellation while hashing cannot expose a download.
    if(signal.aborted||document.visibilityState==="hidden"||csrfCookie()!==csrf)return "uncertain";
    const href=URL.createObjectURL(blob);
    try {const link=document.createElement("a");link.href=href;link.download="customer-import-results.csv";document.body.append(link);link.click();link.remove();}
    finally {URL.revokeObjectURL(href);}
    return "done";
  }catch{return "uncertain";}
}
