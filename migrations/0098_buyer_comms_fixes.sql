-- 0098 buyer-comms defect fixes (contracts/storefront-v2.md §E, defects output/buyer-comms/tests/DEFECTS.md).
-- Fix-forward of two 0090 defects; nothing is renamed, dropped or re-granted differently:
--   * D1 (§E3): notify.claim_batch applied the per-store hourly cap and the shared daily budget AFTER selecting the
--     oldest p_limit PENDING buyer rows, so one store with >= p_limit rows stuck over its cap filled the whole window
--     and starved every other store's buyer mail. The caps are now applied BEFORE the LIMIT (per-store hourly count in
--     the WHERE, remaining daily budget bounds the LIMIT), so a capped store contributes no row to the window.
--     FOR UPDATE SKIP LOCKED, the in-loop guards (which re-count and stay the exact guards, because this batch's own
--     claims move the hour count) and the exactly-once claim -> record_result protocol are unchanged.
--   * D2 (§E5 amendment 2026-10-01): checkout.guest_order_lookup called with a bearer that already names a capability
--     (a registered buyer's token reused as the lookup bearer; the BFF always mints fresh, so only a buggy/hostile
--     client hits this) surfaced the capability_sessions token-hash unique violation as a retryable 503. It now raises
--     PT409 -> a clear non-retryable 409 conflict; the existing session is untouched (replacing it would silently
--     downgrade a registered buyer's full capability to view-only).
-- Owning packages: internal/notify (claim_batch, commerce_expiry_worker since 0096) and internal/buyerhttp (guest_order_lookup,
-- commerce_buyer_issuer). Non-goals: no schema change, no new roles, no behaviour change for any other caller.

-- Integrator merge note: k.locale (column added by 0097_order_locale) replaces the never-written snapshot key that
-- 0090 read; 0098 re-creates the whole function, so it must carry 0097's patch forward.
CREATE OR REPLACE FUNCTION notify.claim_batch(p_limit integer, p_daily_cap integer, p_store_hourly integer) RETURNS jsonb
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_now timestamptz:=clock_timestamp(); v_day timestamptz; v_used integer; v_h integer; v_out jsonb:='[]'::jsonb; v_batch uuid;
 c record; o record; s record; v_store_name text; v_origin text; v_bank jsonb; v_ship jsonb; v_pick jsonb; v_cvs jsonb; v_emails text[]; v_orders uuid[]; v_count integer;
 v_locale text;
BEGIN
 IF p_limit IS NULL OR p_limit NOT BETWEEN 1 AND 50 OR p_daily_cap IS NULL OR p_daily_cap<0 OR p_store_hourly IS NULL OR p_store_hourly<1
  OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid claim' USING ERRCODE='PT400'; END IF;
 -- UTC+8 day start (same alignment as the login-code caps, identity.auth_throttle_hit)
 v_day:=to_timestamp((floor((extract(epoch FROM v_now)+28800)/86400)*86400-28800)::double precision);
 -- A worker that died between SENDING and the record may or may not have sent: UNKNOWN, never re-sent (I06).
 UPDATE notify.outbox SET state='UNKNOWN',updated_at=v_now WHERE state='SENDING' AND claimed_at<v_now-interval '15 minutes';
 UPDATE notify.outbox SET state='SKIPPED',skip_reason='stale',updated_at=v_now WHERE state='PENDING' AND created_at<v_now-interval '24 hours';
 DELETE FROM notify.outbox WHERE (order_id,kind) IN (SELECT x.order_id,x.kind FROM notify.outbox x WHERE x.state<>'PENDING' AND x.created_at<v_now-interval '180 days' ORDER BY x.created_at LIMIT 200);
 -- §E3: every notify mail of the day (a merchant batch counts once) shares one budget of 60% of the daily cap.
 SELECT count(DISTINCT coalesce(x.batch_id::text,x.order_id::text||x.kind)) INTO v_used FROM notify.outbox x
  WHERE x.state IN ('SENDING','SENT','UNKNOWN') AND x.claimed_at>=v_day;

 -- D1: the caps are applied BEFORE the LIMIT (0090 applied them after it and starved every store behind one capped
 -- store). A store already at its hourly cap contributes no row to the window, and the window never exceeds what is
 -- left of today's shared budget. FOR UPDATE locks only notify.outbox x (the cap subquery is never locked); the in-loop
 -- guards below still re-count per row and remain the exact guards, since this batch's own claims move the counts.
 FOR c IN SELECT * FROM notify.outbox x WHERE x.state='PENDING' AND x.kind<>'merchant_new' AND x.next_attempt_at<=v_now
   AND (SELECT count(*) FROM notify.outbox h WHERE h.store_id=x.store_id AND h.kind<>'merchant_new'
     AND h.state IN ('SENDING','SENT','UNKNOWN') AND h.claimed_at>=v_now-interval '1 hour')<p_store_hourly
   ORDER BY x.created_at LIMIT LEAST(p_limit,GREATEST(p_daily_cap-v_used,0)) FOR UPDATE SKIP LOCKED LOOP
  EXIT WHEN v_used>=p_daily_cap;
  SELECT count(*) INTO v_h FROM notify.outbox x WHERE x.store_id=c.store_id AND x.kind<>'merchant_new'
   AND x.state IN ('SENDING','SENT','UNKNOWN') AND x.claimed_at>=v_now-interval '1 hour';
  CONTINUE WHEN v_h>=p_store_hourly;
  SELECT k.owner_id,k.buyer_email,k.payment_mode,k.commercial_state,k.total_minor,k.currency,k.expires_at,k.destination_id,k.locale
   INTO o FROM checkout.orders k WHERE k.id=c.order_id AND k.tenant_id=c.tenant_id AND k.store_id=c.store_id;
  IF NOT FOUND THEN
   UPDATE notify.outbox SET state='SKIPPED',skip_reason='no_order',updated_at=v_now WHERE order_id=c.order_id AND kind=c.kind; CONTINUE; END IF;
  IF o.buyer_email IS NULL THEN
   UPDATE notify.outbox SET state='SKIPPED',skip_reason='no_recipient',updated_at=v_now WHERE order_id=c.order_id AND kind=c.kind; CONTINUE; END IF;
  -- commerce_checkout_writer reads stores, domains, bank transfers, pickups and destination snapshots only inside a scope (0013 / 0073 / 0088
  -- policies are keyed on these GUCs, exactly as every checkout definer sets them): scope this row before reading them.
  PERFORM set_config('app.tenant_id',c.tenant_id::text,true),set_config('app.store_id',c.store_id::text,true),set_config('app.buyer_id',o.owner_id::text,true);
  SELECT s2.name INTO v_store_name FROM control.stores s2 WHERE s2.tenant_id=c.tenant_id AND s2.id=c.store_id;
  -- link origin: the store's ACTIVE published storefront domain (control.storefront_domains); none = a mail without a link
  SELECT d.origin INTO v_origin FROM control.storefront_domains d JOIN control.storefront_publications p ON p.tenant_id=d.tenant_id AND p.store_id=d.store_id
   WHERE d.tenant_id=c.tenant_id AND d.store_id=c.store_id AND d.state='ACTIVE' AND d.valid_until>v_now AND p.published ORDER BY d.id LIMIT 1;
  v_bank:=NULL; v_ship:=NULL; v_pick:=NULL; v_cvs:=NULL;
  IF c.kind='placed' AND o.payment_mode='bank_transfer' THEN
   -- the order's own snapshot (a later settings change never moves an open order's account)
   SELECT jsonb_build_object('bank_name',t.bank_name,'branch',t.branch,'account_name',t.account_name,'account_number',t.account_number)
    INTO v_bank FROM checkout.bank_transfers t WHERE t.tenant_id=c.tenant_id AND t.store_id=c.store_id AND t.order_id=c.order_id;
  END IF;
  IF c.kind IN ('placed','shipped') THEN
   -- CVS pickup store of the order's destination snapshot (home delivery has none)
   SELECT jsonb_build_object('name',pv.name,'address',pv.address) INTO v_pick
    FROM storefront.destination_snapshots ds JOIN fulfillment.pickup_versions pv ON pv.tenant_id=ds.tenant_id AND pv.store_id=ds.store_id AND pv.id=ds.pickup_id
    WHERE ds.tenant_id=c.tenant_id AND ds.store_id=c.store_id AND ds.owner_id=o.owner_id AND ds.id=o.destination_id;
  END IF;
  IF c.kind='shipped' THEN
   SELECT jsonb_build_object('carrier_code',v.carrier_code,'carrier_name',v.carrier_name,'tracking_number',v.tracking_number,'tracking_url',v.tracking_url)
    INTO v_ship FROM fulfillment.manual_shipment_heads h JOIN fulfillment.manual_shipment_versions v
     ON v.tenant_id=h.tenant_id AND v.store_id=h.store_id AND v.order_id=h.order_id AND v.version=h.current_version AND v.status='SHIPPED'
    WHERE h.tenant_id=c.tenant_id AND h.store_id=c.store_id AND h.order_id=c.order_id;
   SELECT jsonb_build_object('shipping_no',coalesce(z.cvs_payment_no,z.shipment_no)) INTO v_cvs FROM fulfillment.cvs_shipments z
    WHERE z.tenant_id=c.tenant_id AND z.store_id=c.store_id AND z.order_id=c.order_id AND z.state IN ('AT_DC','AT_STORE','PICKED_UP') ORDER BY z.attempt DESC LIMIT 1;
  END IF;
  v_locale:=CASE WHEN o.locale IN ('zh-TW','zh-CN','en') THEN o.locale END;
  v_batch:=gen_random_uuid();
  UPDATE notify.outbox SET state='SENDING',attempts=attempts+1,claimed_at=v_now,batch_id=v_batch,updated_at=v_now WHERE order_id=c.order_id AND kind=c.kind;
  v_used:=v_used+1;
  v_out:=v_out||jsonb_build_array(jsonb_build_object('batch_id',v_batch,'kind',c.kind,'order_id',c.order_id,'to',jsonb_build_array(o.buyer_email),
   'store_name',v_store_name,'locale',v_locale,'origin',v_origin,'total_minor',o.total_minor,'currency',o.currency,'payment_mode',o.payment_mode,
   'expires_at',o.expires_at,'bank',v_bank,'pickup',v_pick,'shipment',v_ship,'cvs',v_cvs));
 END LOOP;

 -- Merchant new-order mail: one batch per store, at most one per 5 minutes, only for stores that did not opt out (§E6).
 FOR s IN SELECT DISTINCT x.tenant_id,x.store_id FROM notify.outbox x WHERE x.kind='merchant_new' AND x.state='PENDING' AND x.next_attempt_at<=v_now LOOP
  EXIT WHEN v_used>=p_daily_cap;
  IF EXISTS(SELECT 1 FROM notify.store_settings z WHERE z.tenant_id=s.tenant_id AND z.store_id=s.store_id AND NOT z.merchant_new_order_email) THEN
   UPDATE notify.outbox SET state='SKIPPED',skip_reason='opted_out',updated_at=v_now WHERE store_id=s.store_id AND kind='merchant_new' AND state='PENDING';
   CONTINUE;
  END IF;
  CONTINUE WHEN EXISTS(SELECT 1 FROM notify.outbox x WHERE x.store_id=s.store_id AND x.kind='merchant_new' AND x.state IN ('SENDING','SENT','UNKNOWN')
   AND x.claimed_at>v_now-interval '5 minutes');
  -- the store's owners: store_staff role owner, active membership, verified password email (identity definer; no direct credentials read)
  SELECT array_agg(DISTINCT e) INTO v_emails FROM (SELECT identity.staff_password_email(f.principal_id) AS e FROM identity.store_staff f
   JOIN identity.memberships m ON m.tenant_id=f.tenant_id AND m.principal_id=f.principal_id AND m.active
   WHERE f.tenant_id=s.tenant_id AND f.store_id=s.store_id AND f.role='owner') q WHERE e IS NOT NULL;
  IF v_emails IS NULL THEN
   UPDATE notify.outbox SET state='SKIPPED',skip_reason='no_owner',updated_at=v_now WHERE store_id=s.store_id AND kind='merchant_new' AND state='PENDING';
   CONTINUE;
  END IF;
  PERFORM set_config('app.tenant_id',s.tenant_id::text,true),set_config('app.store_id',s.store_id::text,true);
  SELECT array_agg(q.order_id), count(*) INTO v_orders, v_count FROM (SELECT x.order_id FROM notify.outbox x WHERE x.store_id=s.store_id AND x.kind='merchant_new'
   AND x.state='PENDING' AND x.next_attempt_at<=v_now ORDER BY x.created_at LIMIT 50 FOR UPDATE SKIP LOCKED) q;
  CONTINUE WHEN v_count=0;
  v_batch:=gen_random_uuid();
  UPDATE notify.outbox SET state='SENDING',attempts=attempts+1,claimed_at=v_now,batch_id=v_batch,updated_at=v_now
   WHERE kind='merchant_new' AND order_id=ANY(v_orders);
  v_used:=v_used+1;
  SELECT s2.name INTO v_store_name FROM control.stores s2 WHERE s2.tenant_id=s.tenant_id AND s2.id=s.store_id;
  v_out:=v_out||jsonb_build_array(jsonb_build_object('batch_id',v_batch,'kind','merchant_new','store_name',v_store_name,'to',to_jsonb(v_emails),
   'count',v_count,'order_ids',to_jsonb(v_orders[1:10])));
 END LOOP;
 RETURN v_out;
END $$;
ALTER FUNCTION notify.claim_batch(integer,integer,integer) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION notify.claim_batch(integer,integer,integer) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION notify.claim_batch(integer,integer,integer) TO commerce_expiry_worker; -- 0096 split: the buyer mail loop runs in expiry-worker
COMMENT ON FUNCTION notify.claim_batch(integer,integer,integer) IS 'internal/notify worker only; EXECUTE commerce_expiry_worker (0096 split). Housekeeping (UNKNOWN for dead SENDING, stale SKIPPED, 180-day delete), then claims up to p_limit buyer rows (FOR UPDATE SKIP LOCKED) under the daily budget p_daily_cap and p_store_hourly per store — both applied BEFORE the LIMIT, so a capped store cannot starve the others (0098, D1) — plus at most one merchant batch per store per 5 minutes. Returns the renderer input as jsonb; the recipient address leaves SQL only here, in memory.';

CREATE OR REPLACE FUNCTION checkout.guest_order_lookup(p_store uuid, p_ref text, p_kind text, p_contact bytea, p_token bytea, p_ttl bigint, p_ip bytea)
RETURNS TABLE(order_id uuid)
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_hex text; v_lo uuid; v_hi uuid; r record; v_hit record; v_cmp bytea; v_ok boolean:=false; v_phone text; v_dummy boolean;
BEGIN
 IF p_store IS NULL OR p_ref IS NULL OR length(p_ref)>64 OR p_kind IS NULL OR p_kind NOT IN ('email','phone')
  OR p_contact IS NULL OR octet_length(p_contact)<>32 OR p_token IS NULL OR octet_length(p_token)<>32 OR p_ttl IS NULL OR p_ttl<60 OR p_ttl>2592000
  OR (p_ip IS NOT NULL AND octet_length(p_ip)<>32) OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid lookup' USING ERRCODE='PT400'; END IF;
 v_hex:=lower(regexp_replace(p_ref,'[^0-9a-fA-F]','','g'));
 -- Counted before anything is read and keyed by what the caller sent, so the 429 says nothing about whether an order exists.
 PERFORM checkout.lookup_hit('ip:'||coalesce(encode(p_ip,'hex'),'none'),10);
 PERFORM checkout.lookup_hit('ref:'||p_store::text||':'||v_hex,5);
 PERFORM checkout.lookup_hit('store:'||p_store::text,200);
 -- an inactive store / tenant / owner is refused by buyer.issue_order_capability below (same empty answer as a mismatch)
 IF length(v_hex) IN (12,32) THEN
  IF length(v_hex)=32 THEN v_lo:=v_hex::uuid; v_hi:=v_lo;
  ELSE v_lo:=(v_hex||repeat('0',20))::uuid; v_hi:=(v_hex||repeat('f',20))::uuid; END IF;
  -- uuid compares bytewise, so a 12-hex prefix is one range scan on checkout.orders(id) UNIQUE; DRAFT orders were never placed.
  FOR r IN SELECT k.tenant_id,k.owner_id,k.id,k.buyer_email,k.destination_id FROM checkout.orders k
    WHERE k.store_id=p_store AND k.id BETWEEN v_lo AND v_hi AND k.commercial_state<>'DRAFT' ORDER BY k.id LIMIT 5 LOOP
   -- storefront.destination_snapshots is GUC-scoped for this role (0013 checkout_writer_read): scope the candidate, read its phone (always, so an
   -- email lookup does the same work as a phone lookup)
   PERFORM set_config('app.tenant_id',r.tenant_id::text,true),set_config('app.store_id',p_store::text,true),set_config('app.buyer_id',r.owner_id::text,true);
   SELECT d.phone INTO v_phone FROM storefront.destination_snapshots d WHERE d.tenant_id=r.tenant_id AND d.store_id=p_store AND d.owner_id=r.owner_id AND d.id=r.destination_id;
   -- digest-to-digest compare: nothing about the stored value is revealed by how long it takes (§E5); phone = digits without 886 / leading 0
   v_cmp:=CASE p_kind WHEN 'email' THEN sha256(convert_to(lower(coalesce(r.buyer_email,'')),'UTF8'))
    ELSE sha256(convert_to(regexp_replace(regexp_replace(regexp_replace(coalesce(v_phone,''),'[^0-9]','','g'),'^886',''),'^0',''),'UTF8')) END;
   IF v_cmp=p_contact AND NOT v_ok AND ((p_kind='phone' AND v_phone IS NOT NULL) OR (p_kind='email' AND r.buyer_email IS NOT NULL)) THEN v_ok:=true; v_hit:=r; END IF;
  END LOOP;
 END IF;
 IF NOT v_ok THEN
  v_dummy:=sha256(convert_to(v_hex,'UTF8'))=p_contact; -- the same hash + compare a real candidate costs
  RETURN;
 END IF;
 BEGIN
  PERFORM buyer.issue_order_capability(p_store,v_hit.owner_id,v_hit.id,p_token,p_ttl);
 EXCEPTION WHEN SQLSTATE 'PT401' THEN
  RETURN; -- erased / inactive owner, store or tenant: the same empty answer as a mismatch
  -- D2 (§E5 amendment 2026-10-01): the bearer already names a capability session (a registered buyer's token reused as
  -- the lookup bearer; the BFF always mints fresh). Not a mismatch (the order matched) and not retryable: a clear PT409,
  -- and the existing session is untouched — replacing it would silently downgrade a full buyer capability to view-only.
  WHEN unique_violation THEN
  RAISE EXCEPTION 'lookup bearer already in use' USING ERRCODE='PT409';
 END;
 order_id:=v_hit.id;
 RETURN NEXT;
END $$;
ALTER FUNCTION checkout.guest_order_lookup(uuid,text,text,bytea,bytea,bigint,bytea) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION checkout.guest_order_lookup(uuid,text,text,bytea,bytea,bigint,bytea) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION checkout.guest_order_lookup(uuid,text,text,bytea,bytea,bigint,bytea) TO commerce_buyer_issuer;
COMMENT ON FUNCTION checkout.guest_order_lookup(uuid,text,text,bytea,bytea,bigint,bytea) IS 'internal/buyerhttp lookup route; EXECUTE commerce_buyer_issuer. Throttles (ip 10, order ref 5, store 200 per 10 min), finds the order by a 12-hex id prefix (or full id) in the store, compares sha256(email) or sha256(normalised phone) with p_contact, and on a match issues a view-only capability for that order (p_token = sha256 of the BFF token). Returns the order id, or no row for every kind of mismatch. PT409 when the bearer already names a capability session (0098, D2).';
