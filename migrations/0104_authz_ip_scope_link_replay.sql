-- 0104 authorization hardening (output/kimi-calibration/K3-REVIEW-AUTHZ.md findings F1 and F2, contracts/storefront-v2.md §E5 and §G3).
--   * F1: the guest-lookup and order-link per-IP throttle bucket was global ('ip:'/'olip:' with no store), so one store's
--     flood 429'd every store behind the shared edge IP. The buckets are now store-scoped ('ip:'||store||':', 'olip:'||store||':');
--     the order and store buckets were already scoped. The client IP itself still arrives only via the BFF's X-Commerce-Client-IP
--     (one literal, hashed in Go), fed from the storefront edge's overwritten X-Forwarded-For (deploy/caddy/Caddyfile, preflight P18).
--   * F2: a lost response on POST /v1/buyer/orders/link burned the single-use manual-order link forever. Redemption is now
--     idempotent for a short window: the same link token presented again within 10 minutes by the SAME bearer (the BFF now derives
--     the capability token deterministically from the link token and a browser-bound proof, apps/storefront) re-delivers the SAME
--     capability — no new session, no second capability for a different browser. After the window, or for any other bearer, the
--     refusal is the existing identical empty answer. A merchant "regenerate link" action (fulfillment.regenerate_order_link,
--     audited, invalidates the old link) is added beside mark_order_merchant_manual.
-- Nothing is renamed, dropped or re-granted to a different role; the two replaced functions keep their owner and grants. No grant
-- reaches the retired commerce_worker.
-- Owning packages: internal/buyerhttp (lookup.go, orderlink.go) and internal/merchanttools (manual.go).

-- ---------------------------------------------------------------------------------------------------
-- A. buyer.session_live: the redeem definer's owner (commerce_checkout_writer) cannot read buyer.capability_sessions directly
-- (SELECT is commerce_buyer_writer's, 0006/0078), so the idempotent re-delivery check lives in this buyer-side definer, exactly
-- the session_view_order pattern. Returns whether a live, un-revoked capability session exists for (store, owner, token hash).
-- ---------------------------------------------------------------------------------------------------
CREATE FUNCTION buyer.session_live(p_store uuid, p_owner uuid, p_token bytea) RETURNS boolean
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog AS $$
BEGIN
 IF p_store IS NULL OR p_owner IS NULL OR p_token IS NULL OR octet_length(p_token)<>32 THEN
  RAISE EXCEPTION 'invalid session check' USING ERRCODE='PT400'; END IF;
 RETURN EXISTS(SELECT 1 FROM buyer.capability_sessions c
  WHERE c.token_hash=p_token AND c.store_id=p_store AND c.owner_id=p_owner
    AND c.revoked_at IS NULL AND c.expires_at>clock_timestamp());
END $$;
ALTER FUNCTION buyer.session_live(uuid,uuid,bytea) OWNER TO commerce_buyer_writer;
REVOKE ALL ON FUNCTION buyer.session_live(uuid,uuid,bytea) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION buyer.session_live(uuid,uuid,bytea) TO commerce_checkout_writer;
COMMENT ON FUNCTION buyer.session_live(uuid,uuid,bytea) IS 'checkout.redeem_order_link only (EXECUTE commerce_checkout_writer): whether a live, un-revoked capability session exists for (store, owner, token hash) — the idempotent re-delivery test of K3 F2. Non-goal: no expiry or revocation judgement of anything else.';

-- ---------------------------------------------------------------------------------------------------
-- B. F1: checkout.guest_order_lookup, store-scoped IP throttle. Identical to the active 0098 body (D2 PT409) except the IP bucket key.
-- ---------------------------------------------------------------------------------------------------
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
 -- The IP bucket is store-scoped (K3 F1): one store's flood can no longer throttle another store behind the same edge IP.
 PERFORM checkout.lookup_hit('ip:'||p_store::text||':'||coalesce(encode(p_ip,'hex'),'none'),10);
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
COMMENT ON FUNCTION checkout.guest_order_lookup(uuid,text,text,bytea,bytea,bigint,bytea) IS 'internal/buyerhttp lookup route; EXECUTE commerce_buyer_issuer. Throttles (ip 10 STORE-SCOPED, order ref 5, store 200 per 10 min — the ip bucket is keyed by store since 0104 F1), finds the order by a 12-hex id prefix (or full id) in the store, compares sha256(email) or sha256(normalised phone) with p_contact, and on a match issues a view-only capability for that order (p_token = sha256 of the BFF token). Returns the order id, or no row for every kind of mismatch. PT409 when the bearer already names a capability session (0098, D2).';

-- ---------------------------------------------------------------------------------------------------
-- C. F1 + F2: checkout.redeem_order_link, store-scoped IP throttle and idempotent re-delivery. The single-use exchange is
-- unchanged: the first bearer wins a FULL capability and marks the row used; a later SAME bearer within 10 minutes re-delivers the
-- SAME capability (no new session); every other bearer, or anything after the window, is the identical empty refusal.
-- ---------------------------------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION checkout.redeem_order_link(p_store uuid, p_order uuid, p_link bytea, p_token bytea, p_ttl bigint, p_ip bytea)
RETURNS TABLE(order_id uuid)
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v record;
BEGIN
 IF p_store IS NULL OR p_order IS NULL OR p_link IS NULL OR octet_length(p_link)<>32 OR p_token IS NULL OR octet_length(p_token)<>32
  OR p_ttl IS NULL OR p_ttl<60 OR p_ttl>2592000 OR (p_ip IS NOT NULL AND octet_length(p_ip)<>32)
  OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid order link' USING ERRCODE='PT400'; END IF;
 PERFORM checkout.lookup_hit('olip:'||p_store::text||':'||coalesce(encode(p_ip,'hex'),'none'),10);
 PERFORM checkout.lookup_hit('olord:'||p_store::text||':'||p_order::text,5);
 PERFORM checkout.lookup_hit('olstore:'||p_store::text,200);
 SELECT l.owner_id,l.order_id,l.redeemed_at,l.expires_at INTO v FROM checkout.order_links l
  WHERE l.token_hash=p_link AND l.store_id=p_store AND l.order_id=p_order FOR UPDATE;
 IF NOT FOUND OR v.expires_at<=clock_timestamp() THEN RETURN; END IF;
 IF v.redeemed_at IS NOT NULL THEN
  -- Idempotent re-delivery (K3 F2): the SAME bearer within 10 minutes of redemption re-delivers the SAME capability (no new
  -- session, no new rights — the token hash names the session buyer.issue_owner_capability already registered). Any other
  -- bearer (a different browser-bound proof), or anything after the window, is the identical empty refusal below.
  IF v.redeemed_at > clock_timestamp()-interval '10 minutes' AND buyer.session_live(p_store,v.owner_id,p_token) THEN
   order_id:=v.order_id;
   RETURN NEXT;
  END IF;
  RETURN;
 END IF;
 BEGIN
  PERFORM buyer.issue_owner_capability(p_store,v.owner_id,p_token,p_ttl);
 EXCEPTION WHEN SQLSTATE 'PT401' THEN
  RETURN; -- inactive / erased owner, store or tenant: the same empty answer as a mismatch
 END;
 UPDATE checkout.order_links SET redeemed_at=clock_timestamp() WHERE token_hash=p_link;
 order_id:=v.order_id;
 RETURN NEXT;
END $$;
ALTER FUNCTION checkout.redeem_order_link(uuid,uuid,bytea,bytea,bigint,bytea) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION checkout.redeem_order_link(uuid,uuid,bytea,bytea,bigint,bytea) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION checkout.redeem_order_link(uuid,uuid,bytea,bytea,bigint,bytea) TO commerce_buyer_issuer;
COMMENT ON FUNCTION checkout.redeem_order_link(uuid,uuid,bytea,bytea,bigint,bytea) IS 'internal/buyerhttp orderlink.go (POST /v1/buyer/orders/link); EXECUTE commerce_buyer_issuer. Throttles (ip 10 STORE-SCOPED, order 5, store 200 per 10 min — the ip bucket is keyed by store since 0104 F1), then exchanges a single-use <= 7 day link token for a FULL buyer capability of the order''s owner. The same bearer within 10 minutes of redemption re-delivers the same capability (idempotent, no new session — 0104 F2); every other refusal is one identical empty answer. Non-goals: no plaintext token, no view-only session, no renewal (regenerate_order_link owns re-issuing).';

-- ---------------------------------------------------------------------------------------------------
-- D. F2: the merchant "regenerate link" action. Re-verifies the merchant bearer with inventory:reserve, requires a merchant_manual
-- order of the caller's store, marks every still-unused old link of that order used (invalidating it) and records the new link
-- (sha256, 7 days); a repeat of the same key is a no-op (the new hash already exists, the old links are already dead).
-- ---------------------------------------------------------------------------------------------------
CREATE FUNCTION fulfillment.regenerate_order_link(p_hash bytea, p_store uuid, p_order uuid, p_link bytea) RETURNS void
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE s record; v_source text; v_owner uuid; v_now timestamptz:=clock_timestamp();
BEGIN
 IF p_hash IS NULL OR octet_length(p_hash)<>32 OR p_store IS NULL OR p_order IS NULL OR p_link IS NULL OR octet_length(p_link)<>32
  OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid order link regenerate' USING ERRCODE='PT400'; END IF;
 SELECT * INTO s FROM identity.resolve_access(p_hash,p_store,'inventory:reserve');
 IF s.access_status='unauthorized' THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
 IF s.access_status='not_found' THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
 IF s.access_status IS DISTINCT FROM 'ok' THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 PERFORM set_config('app.tenant_id',s.tenant_id::text,true),set_config('app.store_id',p_store::text,true),
  set_config('app.principal_id',s.principal_id::text,true);
 SELECT o.source,o.owner_id INTO v_source,v_owner FROM checkout.orders o
  WHERE o.tenant_id=s.tenant_id AND o.store_id=p_store AND o.id=p_order FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'order not found' USING ERRCODE='PT404'; END IF;
 IF v_source<>'merchant_manual' THEN RAISE EXCEPTION 'not a manual order' USING ERRCODE='PT409'; END IF;
 -- Invalidate every still-unused old link of this order (a redeemed one is already dead); the caller may not DELETE (no grant),
 -- so the dead mark reuses UPDATE(redeemed_at), the 0094 grant. A repeat of the same key is then a no-op on both statements.
 UPDATE checkout.order_links SET redeemed_at=v_now
  WHERE tenant_id=s.tenant_id AND store_id=p_store AND order_id=p_order AND redeemed_at IS NULL;
 INSERT INTO checkout.order_links(token_hash,tenant_id,store_id,owner_id,order_id,created_at,expires_at)
  VALUES(p_link,s.tenant_id,p_store,v_owner,p_order,v_now,v_now+interval '7 days') ON CONFLICT(token_hash) DO NOTHING;
END $$;
ALTER FUNCTION fulfillment.regenerate_order_link(bytea,uuid,uuid,bytea) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION fulfillment.regenerate_order_link(bytea,uuid,uuid,bytea) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION fulfillment.regenerate_order_link(bytea,uuid,uuid,bytea) TO commerce_runtime;
COMMENT ON FUNCTION fulfillment.regenerate_order_link(bytea,uuid,uuid,bytea) IS 'internal/merchanttools.RegenerateLink only (merchant transaction, commerce_runtime). inventory:reserve via resolve_access; requires a merchant_manual order of the caller''s store, marks every unused old link of it used (invalidates the old link) and records the new link (sha256, 7 days); a repeat is a no-op. Non-goals: no price, stock, state or payment change, no relabel, no plaintext token.';
