// Purpose: closed A2-A5 request grammar and safe refusal codes; no private data in query strings.
// Depends on: live-console-v1 §§2.6/3/11; native URL and JSON.
// Used by: inbox BFF and private browser transport; Go owns authorisation and send rules.
const id = "[0-9a-f]{8}(?:-[0-9a-f]{4}){3}-[0-9a-f]{12}";
const root = `live-sessions/${id}/comments`;
const read = new RegExp(`^${root}$`);
const write = new RegExp(`^${root}/[0-9_]{1,80}/(?:print|private-reply|public-reply)$`);
/** Identifies only frozen comment resources, including invalid-method attempts. */
export const commentResource = (path: string) => read.test(path) || write.test(path);
/** The A2 read and A3-A5 commands are deliberately separate. */
export const commentRoute = (method: string, path: string) => method === "GET" ? read.test(path) : method === "POST" && write.test(path);
/** Rejects unknown/private query keys, duplicate cursors and read-side idempotency keys. */
export function validCommentRequest(request: Request, path: string): boolean {
  if (!commentRoute(request.method,path) || request.headers.has("transfer-encoding")) return false;
  const marker = request.url.indexOf("?"), raw = marker < 0 ? "" : request.url.slice(marker+1);
  if ((marker>=0 && !raw) || raw.length>4096 || /%(?![0-9a-f]{2})/i.test(raw)) return false;
  if(request.method === "POST") return marker<0 && /^[A-Za-z0-9_.:-]{8,128}$/.test(request.headers.get("idempotency-key")??"");
  if(request.body || request.headers.has("idempotency-key") || ![null,"0"].includes(request.headers.get("content-length"))) return false;
  const q = new URL(request.url).searchParams;
  for(const [k,v] of q) {
    if(q.getAll(k).length!==1) return false;
    if(k==="before_cursor" && /^[A-Za-z0-9_-]{1,2048}$/.test(v)) continue;
    if(["after_epoch","after_seq","limit"].includes(k) && /^(0|[1-9][0-9]*)$/.test(v) && Number.isSafeInteger(Number(v))) {
      if(k==="limit" && (Number(v)<1 || Number(v)>100)) return false;
      if(k==="after_epoch" && Number(v)<1) return false;
      continue;
    }
    return false;
  }
  return q.has("after_epoch") === q.has("after_seq") && !(q.has("before_cursor") && q.has("after_epoch"));
}
/** Text/template exclusivity and explicit private-only automatic-reply preemption. */
export function validCommentBody(path: string, raw: string): boolean {
  if(!write.test(path)) return false;
  let b: Record<string,unknown>;
  try { b=JSON.parse(raw); } catch { return false; }
  if(!b || typeof b!=="object" || Array.isArray(b)) return false;
  const keys=Object.keys(b);
  if(path.endsWith("/print")) return keys.length===0;
  const confirm=Object.hasOwn(b,"confirm_preempt_auto");
  if(confirm && (!path.endsWith("/private-reply") || b.confirm_preempt_auto!==true)) return false;
  const fields=keys.filter(k=>k!=="confirm_preempt_auto");
  if(fields.length===1 && fields[0]==="text") return typeof b.text==="string" && b.text.trim().length>0 && b.text.length<=8000 && !/[\u0000-\u0008\u000b\u000c\u000e-\u001f\u007f]/.test(b.text);
  return fields.length===2 && fields.includes("template_id") && fields.includes("template_version") && typeof b.template_id==="string" && /^[a-z0-9][a-z0-9/_-]{0,63}$/.test(b.template_id) && Number.isSafeInteger(b.template_version) && Number(b.template_version)>0;
}
const errors: Record<number, readonly string[]>={
  400:["invalid_cursor","invalid_json","invalid_request"],401:["unauthorized"],403:["forbidden"],404:["not_found"],405:["method_not_allowed"],
  409:["no_source","used","auto_pending","auto_pending_confirm","expired_7d","ig_live_ended","capability","duplicate_recent","comment_facts_unavailable","comment_unknown","page_comment","reply_comment_unsupported","ig_live_unsupported","window_closed"],
  415:["json_required"],422:["invalid_request","invalid_text","public_reply_forbidden_content","ig_live_unsupported"],429:["rate_limited"],503:["stream_unavailable","retry_later","unavailable"],
};
/** Only documented codes cross the privacy boundary, never provider diagnostics. */
export function commentErrorCode(status:number,value:unknown):string|null {
  const code=value && typeof value==="object" && "code" in value ? value.code : null;
  return typeof code==="string" && errors[status]?.includes(code) ? code : null;
}
