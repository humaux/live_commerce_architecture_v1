// Purpose: Render signed standalone order, carrier choice, shipment review and persisted result.
// Depends on: react, @live-commerce/i18n, @/lib/orders-client, @/lib/orders-model, @/lib/orders-copy,
//   ./orders/parity-copy, ./OrderDetailPanel; existing scoped command and readback only.
// Used by: MerchantOrders on /[locale]/orders/[order].
// Invariants: No write before confirmation; CAS and identical uncertain retries preserved; no optimistic shipment fact.
"use client";
import {useEffect, useRef, useState} from "react";
import type {Locale} from "@live-commerce/i18n";
import {putShipment, readShipmentHistory} from "@/lib/orders-client";
import {trackingNumber, type CarrierCode, type OrderDetail} from "@/lib/orders-model";
import type {OrdersCopy} from "@/lib/orders-copy";
import {amount, badge, detailPanel, type Sections} from "./OrderDetailPanel";
import {parityCopy, shortOrderNumber} from "./orders/parity-copy";

const signedCarriers:CarrierCode[]=["black_cat","hsinchu","chunghwa_post","sf_express","seven_eleven_cvs","familymart_cvs","hilife_cvs","okmart_cvs","other"];

/** Signed compact flow; other existing order operations remain in its native disclosure. */
export function OrderDetailView({detail,locale,c,sections,onBack}:{detail:OrderDetail;locale:Locale;c:OrdersCopy;sections:Sections;onBack:()=>void}) {
  const p=parityCopy[locale];
  const [stage,setStage]=useState<"record"|"carrier"|"review">("record");
  const [carrier,setCarrier]=useState<CarrierCode>("black_cat");
  const [name,setName]=useState("");
  const [tracking,setTracking]=useState("");
  const [version,setVersion]=useState<number|null>(null);
  const [reviewed,setReviewed]=useState<number|null>(null);
  const [problem,setProblem]=useState("");
  const [busy,setBusy]=useState(false);
  const [tick,setTick]=useState(0);
  const [uncertain,setUncertain]=useState(false);
  const pending=useRef<{key:string;body:string}|null>(null);
  useEffect(()=>{
    const active=new AbortController();
    readShipmentHistory(sections.store,detail.order_id,active.signal).then(history=>{
      if(!active.signal.aborted) setVersion(Math.max(detail.shipment?.version??0,...history.map(row=>row.version)));
    },()=>{if(!active.signal.aborted)setProblem(c.sectionUnavailable);});
    return ()=>active.abort();
  },[sections.store,detail.order_id,detail.shipment?.version,c.sectionUnavailable,tick]);
  const blocked=detail.work_state==="REVIEW_REQUIRED" || detail.payment_state==="REVIEW_REQUIRED";
  const eligible=!blocked && (detail.commercial_state==="CONFIRMED" || detail.commercial_state==="AWAITING_COLLECTION") &&
    detail.fulfillment_state==="MANUAL_UNASSIGNED" && (detail.work_state==="READY" ||
      (detail.payment_mode==="bank_transfer" && detail.commercial_state==="CONFIRMED") ||
      ((detail.payment_mode==="cash_on_delivery"||detail.payment_mode==="pay_at_pickup")&&detail.collection_state==="PENDING")) &&
    detail.refunded_minor+detail.refund_pending_minor<detail.total_minor;
  const parcel=sections.parcelBlockText?.(detail.order_id);
  const valid=trackingNumber.test(tracking.trim()) && (carrier!=="other" || name.trim().length>0) && [...name.trim()].length<=80;
  const head=detail.shipment;
  const carrierName=carrier==="other"?name.trim():p.carriers[carrier];
  const heading=stage==="carrier"?p.carrier:stage==="review"?p.review:head?p.saved:p.detail;
  async function confirm() {
    if(busy || !eligible || parcel || !valid || reviewed!==version || version===null)return;
    const body=JSON.stringify({expected_version:version,status:"SHIPPED",carrier_code:carrier,carrier_name:carrier==="other"?name.trim():null,tracking_number:tracking.trim(),tracking_url:null,note:null,void_reason:null});
    if(pending.current?.body!==body)pending.current={key:`ship-${crypto.randomUUID()}`,body};
    setBusy(true);setProblem("");
    try {
      const result=await putShipment(sections.store,detail.order_id,pending.current.key,body,sections.boundary);
      if(result.ok){pending.current=null;setUncertain(false);setVersion(null);const loaded=await sections.onChanged();if(!loaded)setProblem(c.sectionUnavailable);setStage("record");}
      else if(result.uncertain){setUncertain(true);setProblem(c.shipUncertain);}
      else{pending.current=null;setProblem(c.errors[result.code as keyof typeof c.errors]??c.sectionUnavailable);sections.onShipmentRefused?.(result.code);if(result.code==="version_changed"){setVersion(null);setReviewed(null);await sections.onChanged();setStage("record");}}
    } finally {setBusy(false);}
  }
  return <section data-testid="order-detail" aria-label={`${c.order} ${detail.order_id}`} className="signed-order-detail">
    <header className="signed-orders-heading"><h1>{heading}</h1><button type="button" data-testid="order-detail-back" onClick={()=>{if(stage!=="record"&&!uncertain)setStage("record");else onBack();}}>{p.back}</button></header>
    {stage==="carrier" ? <>
      <div className="signed-order-summary signed-order-carrier-identity"><strong>{shortOrderNumber(detail.order_id)}</strong></div>
      <div className="signed-order-panel signed-carriers">{signedCarriers.map(code=><button type="button" key={code} onClick={()=>{setCarrier(code);setStage("record");}}>{p.carriers[code]}</button>)}<p className="signed-order-note">{p.deliveryUnchanged}</p></div>
    </> : <>
      <div className="signed-order-summary">
        <strong>{shortOrderNumber(detail.order_id)}</strong>
        <p>{detail.items.map(item=>`${item.name} ×${item.quantity}`).join(" · ")} · {amount(locale,detail.currency,detail.total_minor)}</p>
        <div className="signed-order-state">{badge(detail.fulfillment_state,{...c,statuses:{...c.statuses,MANUAL_UNASSIGNED:p.tabs.ready_to_ship}})}{sections.parcelBadge&&<span data-testid={`parcel-badge-${detail.order_id}`}>{sections.parcelBadge}</span>}{blocked&&<span data-state={detail.work_state}>{p.reviewState}</span>}</div>
      </div>
      {blocked ? <p className="signed-order-panel" role="status">{p.blocked}</p> : head ? <div className="signed-order-panel" data-testid="signed-shipment-record">
        <p>{head.carrier_name??p.carriers[head.carrier_code]}</p><p>{p.tracking}{head.tracking_number}</p>
      </div> : eligible && sections.actions.fulfillment_write && !parcel ? stage==="review" ? <div className="signed-order-panel">
        <p>{carrierName}</p><p>{p.tracking}{tracking.trim()}</p><p className="signed-order-note">{p.note}</p>
      </div> : <div className="signed-order-panel">
        <button type="button" className="signed-carrier-trigger" onClick={()=>setStage("carrier")}>{carrierName||p.carriers[carrier]}</button>
        {carrier==="other"&&<label>{p.other}<input value={name} maxLength={80} onChange={event=>setName(event.target.value)} autoComplete="off"/></label>}
        <label>{p.tracking}<input aria-label={p.tracking} placeholder={p.trackingHint} value={tracking} maxLength={64} autoComplete="off" onChange={event=>setTracking(event.target.value)}/></label>
        <p>{c.shipping}{amount(locale,detail.currency,detail.totals.shipping_minor)}</p>
      </div> : <p className="signed-order-panel">{parcel??c.shipNotEligible}</p>}
      {head&&<><p className="signed-order-note">{p.handover}</p><button className="signed-order-return" type="button" onClick={onBack}>{p.returnOrders}</button></>}
      {problem&&<div role="alert"><p>{problem}</p>{!uncertain&&<button type="button" onClick={()=>{setProblem("");setTick(value=>value+1);void sections.onChanged();}}>{c.retry}</button>}</div>}
      <details className="signed-order-advanced" data-testid="order-more-details"><summary>{p.facts}</summary>
        {detailPanel(detail,locale,c,{...sections,orderNumber:shortOrderNumber(detail.order_id),onBack:undefined})}
      </details>
      {eligible && sections.actions.fulfillment_write && !head && !parcel && <footer className="signed-order-footer">
        {stage==="review"?<><button type="button" className="primary" disabled={busy||reviewed!==version} onClick={()=>void confirm()}>{busy?c.shipSending:uncertain?c.shipRetry:p.confirm}</button><button type="button" disabled={busy||uncertain} onClick={()=>setStage("record")}>{p.edit}</button></>:
        <button type="button" className="primary" disabled={!valid||version===null} onClick={()=>{setReviewed(version);setStage("review");}}>{p.record}</button>}
      </footer>}
    </>}
  </section>;
}
