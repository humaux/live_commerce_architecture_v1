// Purpose: actual frozen import DTO projection negatives and archive-unit count semantics.
// Depends on: node:test/assert and import-model; every fixture is synthetic.
// Used by: W5-U1 focused Node gate, never browser/PG acceptance.
import { test } from "node:test";
import assert from "node:assert/strict";
import { projectImportPreview, parseImportPreview, parseImportReceipt } from "../../apps/admin/lib/import-model.ts";
const hash = "a".repeat(64);
const base = { file_sha256: hash, headers: ["Customer ID", "Name"], mapping: {external_id:"Customer ID",name:"Name"}, rows_total:2,
 new_rows:1,update_rows:0,apply_rows:1,failed_rows:1,erased_rows:1,consent_ignored_rows:1,
 rows:[{row:1,external_id:"synthetic-1",outcome:"created",consent_ignored:true},{row:2,external_id:"",outcome:"failed",code:"erased"}] };
test("backend privacy projection strips all source IDs on successful and failed rows",()=>{
 const safe=projectImportPreview(base,"customers");
 assert.equal(JSON.stringify(safe).includes("synthetic-1"),false);
 assert.deepEqual(safe.rows,[{row:1,outcome:"created",consent_ignored:true},{row:2,outcome:"failed",code:"erased"}]);
 assert.deepEqual(parseImportPreview(safe,"customers"),safe);
});
test("closed preview parser refuses data-bearing rows, unknown keys and inconsistent counts",()=>{
 assert.throws(()=>parseImportPreview(base,"customers"));
 for(const over of [{new_rows:2},{erased_rows:0},{consent_ignored_rows:0},{rows_total:3},{secret:"rawcell"}]) assert.throws(()=>projectImportPreview({...base,...over},"customers"));
 assert.throws(()=>projectImportPreview({...base,rows:[base.rows[0],{...base.rows[1],external_id:"wrong"}]},"customers"));
 assert.throws(()=>projectImportPreview({...base,rows:[base.rows[0],{...base.rows[1],code:"unknown_code"}]},"customers"));
});
test("order preview counts units while first CSV record numbers can exceed rows_total",()=>{
 const order={file_sha256:hash,headers:["Order ID"],mapping:{order_id:"Order ID"},rows_total:1,new_rows:1,update_rows:0,apply_rows:1,failed_rows:0,erased_rows:0,city_dropped_rows:1,
 rows:[{row:5000,external_id:"synthetic-order",outcome:"created",warning:"city_dropped"}]};
 const safe=projectImportPreview(order,"orders");assert.equal(safe.rows[0].row,5000);assert.equal(safe.city_dropped_rows,1);assert.equal("external_id" in safe.rows[0],false);
});
test("commit receipt has exactly five keys and no extra customer cells",()=>{
 const receipt={batch_id:"11111111-1111-4111-8111-111111111111",created:1,updated:0,failed:1,replayed:false};
 assert.deepEqual(parseImportReceipt(receipt),receipt);assert.throws(()=>parseImportReceipt({...receipt,name:"raw"}));
});

// A reader understands the extension; the actual BFF tests below own request-capability admission.
test("safe customer reader accepts explicit closed error fields and preserves legacy absence without guessing",async(t)=>{
 for(const field of ["external_id","name",undefined])await t.test(String(field),()=>{
  const safe={...base,erased_rows:0,rows:[{row:1,outcome:"created",consent_ignored:true},{row:2,outcome:"failed",code:"required",...(field===undefined?{}:{error_field:field})}]};
  const before=structuredClone(safe),parsed=parseImportPreview(safe,"customers");
  assert.deepEqual(parsed,safe);assert.deepEqual(safe,before);
 });
});
test("safe reader rejects enum/cross-field metadata and never accepts source IDs as field metadata",()=>{
 const safe={...base,erased_rows:0,rows:[{row:1,outcome:"created",consent_ignored:true},{row:2,outcome:"failed",code:"required"}]};
 for(const field of [null,"", "phone","Customer ID","synthetic-source",1,{},["name"]])
  assert.throws(()=>parseImportPreview({...safe,rows:[safe.rows[0],{...safe.rows[1],error_field:field}]},"customers"));
 assert.throws(()=>parseImportPreview({...safe,rows:[{...safe.rows[0],error_field:"name"},safe.rows[1]]},"customers"));
 assert.throws(()=>parseImportPreview({...safe,rows:[safe.rows[0],{...safe.rows[1],code:"invalid_external_id",error_field:"external_id"}]},"customers"));
 const {consent_ignored_rows:_unused,...order}=safe;
 assert.throws(()=>parseImportPreview({...order,mapping:{order_id:"Customer ID"},city_dropped_rows:0,rows:[{row:1,outcome:"created"},{row:2,outcome:"failed",code:"required",error_field:"name"}]},"orders"));
});
test("legacy raw projection is a fresh exact safe row copy and does not infer required fields from redacted IDs",()=>{
 const legacy={...base,erased_rows:0,rows:[base.rows[0],{row:2,external_id:"",outcome:"failed",code:"required"}]};
 const before=structuredClone(legacy),safe=projectImportPreview(legacy,"customers");
 assert.deepEqual(safe.rows,[{row:1,outcome:"created",consent_ignored:true},{row:2,outcome:"failed",code:"required"}]);
 assert.deepEqual(legacy,before);assert.notEqual(safe.rows,legacy.rows);assert.notEqual(safe.rows[1],legacy.rows[1]);
});
