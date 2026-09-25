import test from "node:test";
import assert from "node:assert/strict";
import {payOrder,pendingOrderPayment,readOrderPayment,openPaymentDestination} from "../lib/order-payment.ts";

const id=n=>`00000000-0000-0000-0000-${String(n).padStart(12,"0")}`;
const context="a".repeat(43),orderID=id(6),expiry=new Date(Date.now()+3600000).toISOString();
const method={code:"payuni_credit",version:1,name_hans:"测试",name_hant:"測試",name_en:"Mock"};
const form={action:"https://sandbox-api.payuni.com.tw/api/upp",fields:{Version:"2.0",MerID:"synthetic_merchant",EncryptInfo:"ab".repeat(8),HashInfo:"A".repeat(64)}};
const fresh={order_id:orderID,currency:"TWD",total_minor:2500,commercial_state:"DRAFT",test_mode:true,payment_state:"NOT_STARTED",handoff_state:"NONE",handoff_expires_at:null,methods:[method]};
const pending={...fresh,commercial_state:"AWAITING_PAYMENT",payment_state:"PENDING",handoff_state:"PREPARED",handoff_expires_at:expiry,methods:[]};
const order={order_id:orderID,cart_id:id(1),cart_version:2,commercial_state:"DRAFT",fulfillment_state:"MANUAL_UNASSIGNED",hold_expires_at:expiry,snapshot:{quote:{currency:"TWD",lines:[{sku_id:id(2),name:"Synthetic item",code:"SYNTH",quantity:1,unit_price_minor:2500}],amount:{subtotal_minor:2500,discount_minor:0,shipping_minor:0,shipping_tax_minor:0,tax_minor:0,total_minor:2500}},destination:{kind:"home",country:"TW",recipient_name:"Synthetic Recipient",phone:"+886900000001",home_address:{region:"",city:"Synthetic City",postal_code:"",line1:"Synthetic Street",line2:""}},service:{code:"home",name_hans:"测试",name_hant:"測試",name_en:"Synthetic",delivery_kind:"home",mode:"MANUAL"}}};

async function fixture(run){
  const old={fetch:globalThis.fetch,storage:globalThis.localStorage,window:globalThis.window,locks:Object.getOwnPropertyDescriptor(navigator,"locks")};
  const data=new Map(),writes=[],calls=[],locks=[];
  let failWrite=false,active=context,view=fresh,route=async req=>{
    if(req.path===`orders/${orderID}/payment`&&req.method==="GET")return Response.json(view);
    if(req.path.endsWith("/prepare")){view=pending;return Response.json({order_id:orderID,state:"PAYMENT_PENDING",currency:"TWD",amount_minor:2500});}
    if(req.path.endsWith("/handoff"))return Response.json({order_id:orderID,disposition:"ISSUED",expires_at:expiry,form});
    throw Error("unexpected request");
  };
  const store={getItem:k=>data.get(k)??null,setItem:(k,v)=>{if(failWrite)throw Error("synthetic storage denial");writes.push([k,String(v)]);data.set(k,String(v));},removeItem:k=>data.delete(k)};
  globalThis.localStorage=store;globalThis.window={localStorage:store};
  const tails=new Map();Object.defineProperty(navigator,"locks",{configurable:true,value:{request(name,...args){locks.push(name);const next=(tails.get(name)??Promise.resolve()).then(args.at(-1));tails.set(name,next.catch(()=>{}));return next;}}});
  globalThis.fetch=async(url,init)=>{
    if(url.endsWith("/session"))return Response.json({state:"active",context:active,expires_at:expiry});
    const req={path:url.replace("/api/buyer/",""),method:init.method,key:new Headers(init.headers).get("Idempotency-Key"),body:init.body?JSON.parse(init.body):undefined};
    calls.push(req);return route(req);
  };
  const submitted=[];let ready=true,closed=0;
  const destination={ready:()=>ready,submit:x=>submitted.push(x),close:()=>{closed++;}};
  const gate={calls,writes,data,locks,submitted,destination,get closed(){return closed;},get view(){return view;},set view(x){view=x;},get route(){return route;},set route(x){route=x;},set active(x){active=x;},set ready(x){ready=x;},set failWrite(x){failWrite=x;},pay:(extra={})=>payOrder({context,order,locale:"en",method,destination,isCurrent:()=>true,...extra})};
  try{return await run(gate);}finally{globalThis.fetch=old.fetch;globalThis.localStorage=old.storage;globalThis.window=old.window;if(old.locks)Object.defineProperty(navigator,"locks",old.locks);else delete navigator.locks;}
}
const posts=(gate,suffix)=>gate.calls.filter(x=>x.path===`orders/${orderID}/payment/${suffix}`&&x.method==="POST");

test("BPU01 fresh payment writes exact metadata before prepare, one bodyless Take, no persistent form",async()=>fixture(async g=>{
  await g.pay();
  const marker=pendingOrderPayment(context,orderID);
  assert.deepEqual(Object.keys(marker).sort(),["body","context","key","order_id","stage","v"]);
  assert.deepEqual(marker.body,{method_code:"payuni_credit",method_version:1,locale:"en"});assert.equal(marker.stage,"handoff_started");
  assert.equal(posts(g,"prepare").length,1);assert.equal(posts(g,"handoff").length,1);
  assert.deepEqual(posts(g,"prepare")[0].body,marker.body);assert.equal(posts(g,"prepare")[0].key,marker.key);
  assert.equal(posts(g,"handoff")[0].body,undefined);assert.equal(posts(g,"handoff")[0].key,null);
  assert.deepEqual(g.submitted,[form]);assert(g.locks.includes("commerce-purchase-write-v1"));
  assert.deepEqual(g.writes.map(([,value])=>JSON.parse(value).stage),["prepare","handoff_started"]);
  assert(!JSON.stringify(g.writes).includes("EncryptInfo"));assert(!JSON.stringify(g.writes).includes("Synthetic Recipient"));
  await assert.rejects(g.pay(),{code:"uncertain"});assert.equal(posts(g,"handoff").length,1);
}));

test("BPU01 lost prepare response replays original key/body only on explicit next click",async()=>fixture(async g=>{
  let count=0;const normal=g.route;
  g.route=async req=>{if(req.path.endsWith("/prepare")&&++count===1){g.view=pending;throw Error("synthetic response loss");}return normal(req);};
  await assert.rejects(g.pay(),{code:"uncertain"});const first=pendingOrderPayment(context,orderID);assert.equal(first.stage,"prepare");
  assert.equal(posts(g,"handoff").length,0);assert.equal(g.submitted.length,0);
  assert.equal((await readOrderPayment(context,orderID)).payment_state,"PENDING");assert.equal(posts(g,"prepare").length,1);
  await g.pay({locale:"zh-TW",method:undefined});
  assert.equal(posts(g,"prepare").length,2);assert.equal(posts(g,"prepare")[0].key,posts(g,"prepare")[1].key);
  assert.deepEqual(posts(g,"prepare")[0].body,posts(g,"prepare")[1].body);assert.equal(posts(g,"prepare")[1].body.locale,"en");
  assert.equal(posts(g,"handoff").length,1);assert.equal(g.submitted.length,1);
}));

test("BPU01 lost handoff is permanently GET-only, even after a second click",async()=>fixture(async g=>{
  const normal=g.route;g.route=req=>req.path.endsWith("/handoff")?Promise.reject(Error("synthetic lost handoff")):normal(req);
  await assert.rejects(g.pay(),{code:"uncertain"});assert.equal(pendingOrderPayment(context,orderID).stage,"handoff_started");
  assert.equal(posts(g,"handoff").length,1);assert.equal(g.submitted.length,0);
  await readOrderPayment(context,orderID);await assert.rejects(g.pay(),{code:"uncertain"});
  assert.equal(posts(g,"prepare").length,1);assert.equal(posts(g,"handoff").length,1);assert.equal(g.submitted.length,0);
}));

test("BPU01 storage, amount, scope and child/screen fences reject before irreversible writes",async()=>{
  for(const change of [
    g=>{g.failWrite=true;},
    g=>{g.view={...fresh,total_minor:2501};},
    g=>{g.active="b".repeat(43);},
    g=>{g.ready=false;},
    g=>{g.data.set(`commerce-order-payment-v1:${context}:${orderID}`,"{}");},
  ])await fixture(async g=>{change(g);await assert.rejects(g.pay());assert.equal(posts(g,"prepare").length,0);assert.equal(posts(g,"handoff").length,0);});
  await fixture(async g=>{await assert.rejects(g.pay({isCurrent:()=>false}),{code:"uncertain"});assert.equal(posts(g,"prepare").length,0);});
});

test("BPU01 document/child loss after prepare but before Take leaves exact replay marker",async()=>fixture(async g=>{
  const normal=g.route;g.route=async req=>{const result=await normal(req);if(req.path.endsWith("/prepare"))g.ready=false;return result;};
  await assert.rejects(g.pay(),{code:"uncertain"});assert.equal(pendingOrderPayment(context,orderID).stage,"prepare");
  assert.equal(posts(g,"prepare").length,1);assert.equal(posts(g,"handoff").length,0);assert.equal(g.submitted.length,0);
}));

test("BPU01 malformed server handoff never submits and Take remains nonretryable",async()=>fixture(async g=>{
  const normal=g.route;g.route=req=>req.path.endsWith("/handoff")?Response.json({order_id:orderID,disposition:"ISSUED",expires_at:expiry,form:{...form,action:"https://evil.example/charge"}}):normal(req);
  await assert.rejects(g.pay(),{code:"uncertain"});assert.equal(g.submitted.length,0);
  assert.equal(pendingOrderPayment(context,orderID).stage,"handoff_started");await assert.rejects(g.pay());assert.equal(posts(g,"handoff").length,1);
}));

test("BPU01 marker replacement before Take and after Take blocks release/form reuse",async()=>{
  await fixture(async g=>{
    const original=localStorage.getItem.bind(localStorage);let changed=false;
    localStorage.getItem=key=>{
      const value=original(key);
      if(!changed&&key.startsWith("commerce-order-payment-v1:")&&value?.includes('"handoff_started"')){
        changed=true;g.data.set(key,JSON.stringify({...JSON.parse(value),key:id(99)}));
      }
      return value;
    };
    await assert.rejects(g.pay(),{code:"uncertain"});assert(changed);
    assert.equal(posts(g,"prepare").length,1);assert.equal(posts(g,"handoff").length,0);assert.equal(g.submitted.length,0);
  });
  await fixture(async g=>{
    const normal=g.route;g.route=async req=>{
      const response=await normal(req);
      if(req.path.endsWith("/handoff")){
        const key=`commerce-order-payment-v1:${context}:${orderID}`;
        g.data.set(key,JSON.stringify({...JSON.parse(g.data.get(key)),key:id(98)}));
      }
      return response;
    };
    await assert.rejects(g.pay(),{code:"uncertain"});
    assert.equal(posts(g,"handoff").length,1);assert.equal(g.submitted.length,0);
  });
});

test("BPU01 context replacement after prepare and after Take closes destination without form",async()=>{
  await fixture(async g=>{
    const normal=g.route;g.route=async req=>{const response=await normal(req);if(req.path.endsWith("/prepare"))g.active="b".repeat(43);return response;};
    await assert.rejects(g.pay(),{code:"context_changed"});assert.equal(posts(g,"prepare").length,1);
    assert.equal(posts(g,"handoff").length,0);assert.equal(g.submitted.length,0);assert(g.closed>0);
  });
  await fixture(async g=>{
    const normal=g.route;g.route=async req=>{const response=await normal(req);if(req.path.endsWith("/handoff"))g.active="b".repeat(43);return response;};
    await assert.rejects(g.pay(),{code:"context_changed"});assert.equal(posts(g,"handoff").length,1);
    assert.equal(g.submitted.length,0);assert(g.closed>0);
  });
});

test("BPU01 synchronous destination refuses invalid locale before opening a window",()=>{
  assert.throws(()=>openPaymentDestination("fr"),{code:"request_failed"});
});
