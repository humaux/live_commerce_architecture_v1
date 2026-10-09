// Purpose: header-only UTF-8/RFC4180 reads, exact query grammar and failed CSV data minimization negatives.
// Depends on: node:test/assert, import header/request/BFF pure helpers; synthetic data only.
// Used by: W5-U1 focused Node gate; does not inspect real buyer files.
import {test} from "node:test";
import assert from "node:assert/strict";
import {readImportHeader} from "../../apps/admin/lib/import-header.ts";
import {guessImportMapping,importRoute,validImportRequest} from "../../apps/admin/lib/import-request.ts";
import {projectImportFailures,readImportBytes} from "../../apps/admin/lib/import-bff.ts";
const id="11111111-1111-4111-8111-111111111111";
test("reads quoted/BOM header only, without decoding malformed bytes from data rows",async()=>{
 const file=new Blob(["\ufeff\"Customer ID\",\"Name, quoted\",\"multi\nline\"\r\n",new Uint8Array([0xff,0xff])]);
 assert.deepEqual(await readImportHeader(file),["Customer ID","Name, quoted","multi\nline"]);
 assert.deepEqual(await readImportHeader(new Blob(["\ufeff\r\n\nCustomer ID,Name\nnot decoded"])),["Customer ID","Name"]);
 await assert.rejects(()=>readImportHeader(new Blob([new Uint8Array([0xff]),"\n"])),/encoding_not_utf8/);
 await assert.rejects(()=>readImportHeader(new Blob(["bad\"quote,Name\nrows"])),/invalid_request/);
 await assert.rejects(()=>readImportHeader(new Blob([new Uint8Array(2097153)])),/file_too_large/);
});
test("guess only unique metadata aliases with explicit unmapped values",()=>{
 const map=guessImportMapping("customers",["Ｃｕｓｔｏｍｅｒ ＩＤ","姓名","行動電話","同意行銷"]);
 assert.equal(map.external_id,"Ｃｕｓｔｏｍｅｒ ＩＤ");assert.equal(map.name,"姓名");assert.equal(map.phone,"行動電話");assert.equal(map.email,"");
 assert.equal(guessImportMapping("customers",["Customer ID","customer-id","Name"]).external_id,"");
});
test("shared dynamic leaf admits only exact import methods/actions and bounded mapping/count",()=>{
 const preview=importRoute("POST","customers","preview")!,commit=importRoute("POST","orders","commit")!,result=importRoute("GET",id,"results.csv")!;
 const req=(q:string,method="POST",headers:Record<string,string>={"content-type":"text/csv"})=>new Request(`https://admin.example.invalid/api/stores/${id}/imports/customers/preview${q}`,{method,headers});
 assert.equal(importRoute("GET","customers","preview"),null);assert.equal(importRoute("POST",id,"results.csv"),null);
 assert.equal(validImportRequest(req("?mapping=%7B%22name%22%3A%22Name%22%7D"),preview),true);
 for(const q of ["?tenant_id=x","?mapping=%xx","?mapping={}&mapping={}","?expected_apply_rows=1"] )assert.equal(validImportRequest(req(q),preview),false,q);
 assert.equal(validImportRequest(req("?expected_apply_rows=5000"),commit),true);
 assert.equal(validImportRequest(req("?expected_apply_rows=5001"),commit),false);
 assert.equal(validImportRequest(req("?mapping="+encodeURIComponent(JSON.stringify({unknown:"Name"}))),preview),false);
 assert.equal(validImportRequest(req("?only=failed","GET",{}),result),true);assert.equal(validImportRequest(req("","GET",{}),result),false);
});
test("failed CSV emits only row/outcome/code, refusing any unexpected identifier or raw cell",()=>{
 const enc=new TextEncoder();
 assert.equal(projectImportFailures(enc.encode("\ufeffrow,external_id,outcome,code\n2,,failed,invalid_phone\n")),"\ufeffrow,outcome,code\n2,failed,invalid_phone\n");
 for(const line of ["2,source-id,failed,invalid_phone","2,,created,invalid_phone","2,,failed,raw_cell","2,,failed,=formula"])
  assert.throws(()=>projectImportFailures(enc.encode("row,external_id,outcome,code\n"+line+"\n")));
});
test("raw-byte cap does not decode/rebuild uploads and cancels oversized chunks",async()=>{
 const bytes=new Uint8Array([0xef,0xbb,0xbf,0x41,0x0d,0x0a,0xff]);
 assert.deepEqual(await readImportBytes(new Response(bytes),100),bytes);
 assert.equal(await readImportBytes(new Response(bytes),3),null);
});
