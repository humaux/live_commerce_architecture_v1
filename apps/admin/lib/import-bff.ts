// Purpose: bounded raw-byte reads and safe failure CSV projection for migration import.
// Depends on: native stream/TextDecoder, import-model closed row codes; no auth or storage.
// Used by: import-proxy and focused wire tests; no uploaded data row decoding.
import { importRowCodes, maxImportRows } from "./import-model.ts";

/** Read raw bytes once within a cap, aborting stream readers on cancellation; never decode uploads. */
export async function readImportBytes(input:Request|Response,limit:number,signal?:AbortSignal):Promise<Uint8Array<ArrayBuffer>|null> {
  const declared=input.headers.get("content-length");
  if(declared!==null && (!/^\d+$/.test(declared)||Number(declared)>limit))return null;
  const reader=input.body?.getReader();if(!reader)return new Uint8Array(0);
  const abort=()=>{void reader.cancel().catch(()=>{});};
  signal?.addEventListener("abort",abort,{once:true});
  const chunks:Uint8Array[]=[];let size=0;
  try {
    while(!signal?.aborted){const part=await reader.read();if(part.done)break;size+=part.value.byteLength;if(size>limit){await reader.cancel();return null;}chunks.push(part.value);}
    if(signal?.aborted)return null;
    const result=new Uint8Array(size);let at=0;for(const chunk of chunks){result.set(chunk,at);at+=chunk.length;}return result;
  }catch{return null;}finally{signal?.removeEventListener("abort",abort);reader.releaseLock();}
}

/** Validate the failed-only backend CSV, then emit just row/outcome/code with no source-ID column. */
export function projectImportFailures(data:Uint8Array):string {
  const text=new TextDecoder("utf-8",{fatal:true}).decode(data).replace(/^\uFEFF/,"").replaceAll("\r\n","\n");
  const lines=text.split("\n");if(lines.pop()!=="" || lines.shift()!=="row,external_id,outcome,code" || lines.length>maxImportRows)throw Error("invalid_response");
  let previous=0;const safe:string[]=[];
  for(const line of lines){const match=/^([1-9][0-9]*),,failed,([a-z_]+)$/.exec(line);if(!match || Number(match[1])<=previous || Number(match[1])>maxImportRows || !(importRowCodes as readonly string[]).includes(match[2]))throw Error("invalid_response");
    previous=Number(match[1]);safe.push(`${match[1]},failed,${match[2]}`);}
  return `\uFEFFrow,outcome,code\n${safe.length?safe.join("\n")+"\n":""}`;
}
