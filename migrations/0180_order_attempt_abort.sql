-- Purpose: fenced metadata-only abort and the reusable native owner prefix.
-- Depends on: 0170/0171 identity/provenance, 0175 admission, 0178 execution witness.
-- Used by: actual runtime/claims executor; domain helpers remain coordinator-only.
-- Invariants: current-xid PENDING fence, all indexed owners sorted, exact absence
-- before release/seal, immutable receipt, no scope/principal/subject-barrier locks.

CREATE FUNCTION ops.order_attempt_subject_context(p_attempt uuid,p_generation bigint)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE ctx jsonb; result jsonb;
 prior_tenant text:=current_setting('app.tenant_id',true);
 prior_store text:=current_setting('app.store_id',true);
BEGIN
 ctx:=ops.order_attempt_fence_context(p_attempt,p_generation);
 PERFORM set_config('app.tenant_id',ctx->>'tenant_id',true);
 PERFORM set_config('app.store_id',ctx->>'store_id',true);
 result:=ctx||jsonb_build_object('owner_ids',ARRAY(
  SELECT id FROM (
   SELECT (ctx->>'buyer_owner_id')::uuid AS id
   UNION
   SELECT s.subject_id FROM ops.order_attempt_subjects s
    WHERE s.tenant_id=(ctx->>'tenant_id')::uuid AND s.store_id=(ctx->>'store_id')::uuid
     AND s.attempt_id=p_attempt
  ) owners ORDER BY id));
 PERFORM set_config('app.tenant_id',coalesce(prior_tenant,''),true);
 PERFORM set_config('app.store_id',coalesce(prior_store,''),true);
 RETURN result;
EXCEPTION WHEN OTHERS THEN
 PERFORM set_config('app.tenant_id',coalesce(prior_tenant,''),true);
 PERFORM set_config('app.store_id',coalesce(prior_store,''),true);
 RAISE;
END $$;
ALTER FUNCTION ops.order_attempt_subject_context(uuid,bigint) OWNER TO commerce_order_attempt_owner;
REVOKE ALL ON FUNCTION ops.order_attempt_subject_context(uuid,bigint) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION ops.order_attempt_subject_context(uuid,bigint) TO commerce_buyer_writer;
COMMENT ON FUNCTION ops.order_attempt_subject_context(uuid,bigint) IS
 'Private buyer-owner metadata accessor: exact finished PENDING/current-xid fence and all indexed native owners plus original owner; no plaintext, caller owner list, identity creation or earlier-level locks.';

CREATE FUNCTION buyer.order_attempt_lock_owners(p_attempt uuid,p_generation bigint)
RETURNS text LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE ctx jsonb; owner_id uuid; owner_active boolean;
 missing boolean:=false; erased boolean:=false;
BEGIN
 ctx:=ops.order_attempt_subject_context(p_attempt,p_generation);
 -- Every current subject kind indexes a native buyer owner, including linked
 -- customer/private contact owners. Keep locking after an inactive/missing row.
 FOR owner_id IN SELECT value::uuid FROM jsonb_array_elements_text(ctx->'owner_ids') ORDER BY value::uuid LOOP
  SELECT o.active INTO owner_active FROM buyer.owners o
   WHERE o.tenant_id=(ctx->>'tenant_id')::uuid AND o.store_id=(ctx->>'store_id')::uuid AND o.id=owner_id
   FOR UPDATE;
  IF NOT FOUND THEN missing:=true;
  ELSIF NOT owner_active THEN erased:=true;
  END IF;
 END LOOP;
 IF erased THEN RETURN 'erased'; END IF;
 IF missing THEN RETURN 'missing'; END IF;
 RETURN 'active';
END $$;
ALTER FUNCTION buyer.order_attempt_lock_owners(uuid,bigint) OWNER TO commerce_buyer_writer;
REVOKE ALL ON FUNCTION buyer.order_attempt_lock_owners(uuid,bigint) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION buyer.order_attempt_lock_owners(uuid,bigint) TO commerce_order_attempt_owner;
COMMENT ON FUNCTION buyer.order_attempt_lock_owners(uuid,bigint) IS
 'Private shared C2/abort owner prefix: exact metadata witness, every indexed owner FOR UPDATE in UUID order; returns active/erased/missing, without resolve_scope, principal, scope, subject barriers or placement authentication.';

-- Domain-owned absence inspection needs only provenance columns. These scoped
-- SELECT policies and column grants expose no contacts/prices to the coordinator.
GRANT USAGE ON SCHEMA storefront TO commerce_buyer_writer;
DO $$ DECLARE relation text; BEGIN
 FOREACH relation IN ARRAY ARRAY['storefront.carts','storefront.cart_lines','storefront.quotes',
  'storefront.events','storefront.destination_snapshots','storefront.destination_events','buyer.command_results'] LOOP
  EXECUTE format('GRANT SELECT(tenant_id,store_id,owner_id,create_attempt_id) ON %s TO commerce_buyer_writer',relation);
  EXECUTE format('CREATE POLICY order_attempt_abort_read ON %s FOR SELECT TO commerce_buyer_writer
   USING(tenant_id=nullif(current_setting(''app.tenant_id'',true),'''')::uuid
    AND store_id=nullif(current_setting(''app.store_id'',true),'''')::uuid)',relation);
 END LOOP;
END $$;
GRANT SELECT(create_attempt_id,buyer_owner_id) ON fulfillment.pickup_versions TO commerce_checkout_writer;

CREATE FUNCTION buyer.order_attempt_assert_no_effects(p_attempt uuid,p_generation bigint)
RETURNS void LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE ctx jsonb; tenant uuid; store uuid; has_effect boolean;
 prior text[]:=ARRAY[current_setting('app.tenant_id',true),current_setting('app.store_id',true),
  current_setting('app.principal_id',true),current_setting('app.buyer_id',true),current_setting('app.buyer_session_id',true)];
BEGIN
 ctx:=ops.order_attempt_fence_context(p_attempt,p_generation);
 tenant:=(ctx->>'tenant_id')::uuid; store:=(ctx->>'store_id')::uuid;
 PERFORM set_config('app.tenant_id',tenant::text,true),set_config('app.store_id',store::text,true),
  set_config('app.principal_id',ctx->>'principal_id',true),set_config('app.buyer_id',ctx->>'buyer_owner_id',true),
  set_config('app.buyer_session_id','',true);
 SELECT EXISTS(
  SELECT 1 FROM storefront.carts x WHERE x.tenant_id=tenant AND x.store_id=store AND x.create_attempt_id=p_attempt
  UNION ALL SELECT 1 FROM storefront.cart_lines x WHERE x.tenant_id=tenant AND x.store_id=store AND x.create_attempt_id=p_attempt
  UNION ALL SELECT 1 FROM storefront.quotes x WHERE x.tenant_id=tenant AND x.store_id=store AND x.create_attempt_id=p_attempt
  UNION ALL SELECT 1 FROM storefront.events x WHERE x.tenant_id=tenant AND x.store_id=store AND x.create_attempt_id=p_attempt
  UNION ALL SELECT 1 FROM storefront.destination_snapshots x WHERE x.tenant_id=tenant AND x.store_id=store AND x.create_attempt_id=p_attempt
  UNION ALL SELECT 1 FROM storefront.destination_events x WHERE x.tenant_id=tenant AND x.store_id=store AND x.create_attempt_id=p_attempt
  UNION ALL SELECT 1 FROM buyer.command_results x WHERE x.tenant_id=tenant AND x.store_id=store AND x.create_attempt_id=p_attempt
 ) INTO has_effect;
 IF has_effect THEN RAISE EXCEPTION 'attempt_has_effects' USING ERRCODE='PT409'; END IF;
 PERFORM set_config('app.tenant_id',coalesce(prior[1],''),true),set_config('app.store_id',coalesce(prior[2],''),true),
  set_config('app.principal_id',coalesce(prior[3],''),true),set_config('app.buyer_id',coalesce(prior[4],''),true),
  set_config('app.buyer_session_id',coalesce(prior[5],''),true);
EXCEPTION WHEN OTHERS THEN
 PERFORM set_config('app.tenant_id',coalesce(prior[1],''),true),set_config('app.store_id',coalesce(prior[2],''),true),
  set_config('app.principal_id',coalesce(prior[3],''),true),set_config('app.buyer_id',coalesce(prior[4],''),true),
  set_config('app.buyer_session_id',coalesce(prior[5],''),true);
 RAISE;
END $$;
ALTER FUNCTION buyer.order_attempt_assert_no_effects(uuid,bigint) OWNER TO commerce_buyer_writer;
REVOKE ALL ON FUNCTION buyer.order_attempt_assert_no_effects(uuid,bigint) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION buyer.order_attempt_assert_no_effects(uuid,bigint) TO commerce_order_attempt_owner;
COMMENT ON FUNCTION buyer.order_attempt_assert_no_effects(uuid,bigint) IS
 'Private native cart/line/quote/destination/event/receipt absence proof for this exact fenced creator; rejects same-Tx partials before seal, no business row locks or compensation.';

CREATE FUNCTION checkout.order_attempt_assert_no_effects(p_attempt uuid,p_generation bigint)
RETURNS void LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE ctx jsonb; tenant uuid; store uuid; has_effect boolean;
 prior text[]:=ARRAY[current_setting('app.tenant_id',true),current_setting('app.store_id',true),
  current_setting('app.principal_id',true),current_setting('app.buyer_id',true),current_setting('app.buyer_session_id',true)];
BEGIN
 ctx:=ops.order_attempt_fence_context(p_attempt,p_generation);
 tenant:=(ctx->>'tenant_id')::uuid; store:=(ctx->>'store_id')::uuid;
 PERFORM set_config('app.tenant_id',tenant::text,true),set_config('app.store_id',store::text,true),
  set_config('app.principal_id',ctx->>'principal_id',true),set_config('app.buyer_id',ctx->>'buyer_owner_id',true),
  set_config('app.buyer_session_id','',true);
 -- CANCELLED orders remain effects. No same-buyer/unrelated-order predicate.
 SELECT EXISTS(
  SELECT 1 FROM checkout.orders x WHERE x.tenant_id=tenant AND x.store_id=store AND x.create_attempt_id=p_attempt
  UNION ALL SELECT 1 FROM checkout.command_results x WHERE x.tenant_id=tenant AND x.store_id=store AND x.create_attempt_id=p_attempt
  UNION ALL SELECT 1 FROM checkout.events x WHERE x.tenant_id=tenant AND x.store_id=store AND x.create_attempt_id=p_attempt
  UNION ALL SELECT 1 FROM fulfillment.pickup_versions x WHERE x.tenant_id=tenant AND x.store_id=store AND x.create_attempt_id=p_attempt
  UNION ALL SELECT 1 FROM inventory.reservations x WHERE x.tenant_id=tenant AND x.store_id=store AND x.create_attempt_id=p_attempt
  UNION ALL SELECT 1 FROM inventory.ledger x WHERE x.tenant_id=tenant AND x.store_id=store AND x.create_attempt_id=p_attempt
 ) INTO has_effect;
 IF has_effect THEN RAISE EXCEPTION 'attempt_has_effects' USING ERRCODE='PT409'; END IF;
 PERFORM set_config('app.tenant_id',coalesce(prior[1],''),true),set_config('app.store_id',coalesce(prior[2],''),true),
  set_config('app.principal_id',coalesce(prior[3],''),true),set_config('app.buyer_id',coalesce(prior[4],''),true),
  set_config('app.buyer_session_id',coalesce(prior[5],''),true);
EXCEPTION WHEN OTHERS THEN
 PERFORM set_config('app.tenant_id',coalesce(prior[1],''),true),set_config('app.store_id',coalesce(prior[2],''),true),
  set_config('app.principal_id',coalesce(prior[3],''),true),set_config('app.buyer_id',coalesce(prior[4],''),true),
  set_config('app.buyer_session_id',coalesce(prior[5],''),true);
 RAISE;
END $$;
ALTER FUNCTION checkout.order_attempt_assert_no_effects(uuid,bigint) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION checkout.order_attempt_assert_no_effects(uuid,bigint) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION checkout.order_attempt_assert_no_effects(uuid,bigint) TO commerce_order_attempt_owner;
COMMENT ON FUNCTION checkout.order_attempt_assert_no_effects(uuid,bigint) IS
 'Private exact fenced creator absence proof for all-state orders, checkout receipts/events, local pickup, native stock reservations and ledger; read-only, no unrelated owner-order false block.';

CREATE FUNCTION claims.order_attempt_release_admission(p_attempt uuid,p_generation bigint)
RETURNS integer LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE ctx jsonb; tenant uuid; store uuid; owner_id uuid; principal uuid; request_digest bytea;
 bundle_ids uuid[]; released integer;
 prior text[]:=ARRAY[current_setting('app.tenant_id',true),current_setting('app.store_id',true),
  current_setting('app.principal_id',true),current_setting('app.buyer_id',true),current_setting('app.buyer_session_id',true)];
BEGIN
 ctx:=ops.order_attempt_fence_context(p_attempt,p_generation);
 tenant:=(ctx->>'tenant_id')::uuid; store:=(ctx->>'store_id')::uuid;
 owner_id:=(ctx->>'buyer_owner_id')::uuid; principal:=(ctx->>'principal_id')::uuid;
 -- Same opaque custody binding as C1 and Begin; the canonical request digest
 -- stays in ops retention and is not copied into longer-lived domain audit.
 request_digest:=sha256(convert_to('order-attempt-custody/v1|'||tenant::text||'|'||store::text||'|'||p_attempt::text
  ||'|'||(ctx->>'generation')::bigint::text||'|'||owner_id::text||'|'||principal::text,'UTF8'));
 PERFORM set_config('app.tenant_id',tenant::text,true),set_config('app.store_id',store::text,true),
  set_config('app.principal_id',principal::text,true),set_config('app.buyer_id','',true),set_config('app.buyer_session_id','',true);
 bundle_ids:=ARRAY(SELECT bundle_id FROM (
  SELECT r.bundle_id FROM inbox.order_for_buyer r WHERE r.tenant_id=tenant AND r.store_id=store AND r.create_attempt_id=p_attempt
  UNION SELECT g.bundle_id FROM claims.merchant_origin_grants g WHERE g.tenant_id=tenant AND g.store_id=store AND g.create_attempt_id=p_attempt
 ) admitted ORDER BY bundle_id);
 -- Follow the claim suffix without touching another request's rows. The bundle
 -- fence serializes new admissions/consumption; all own rows are re-read below.
 PERFORM b.id FROM claims.bundles b WHERE b.tenant_id=tenant AND b.store_id=store AND b.id=ANY(bundle_ids) ORDER BY b.id FOR UPDATE;
 PERFORM r.request_id FROM inbox.order_for_buyer r WHERE r.tenant_id=tenant AND r.store_id=store AND r.create_attempt_id=p_attempt
  ORDER BY r.bundle_id,r.request_id FOR UPDATE;
 PERFORM g.request_id FROM claims.merchant_origin_grants g WHERE g.tenant_id=tenant AND g.store_id=store AND g.create_attempt_id=p_attempt
  ORDER BY g.bundle_id,g.request_id FOR UPDATE;
 IF EXISTS(SELECT 1 FROM inbox.order_for_buyer r WHERE r.tenant_id=tenant AND r.store_id=store AND r.create_attempt_id=p_attempt
  AND (r.request_id<>p_attempt OR r.buyer_id<>owner_id OR r.principal_id<>principal
   OR r.request_hash IS DISTINCT FROM request_digest OR r.state<>'pending' OR r.order_id IS NOT NULL))
  OR EXISTS(SELECT 1 FROM claims.merchant_origin_grants g WHERE g.tenant_id=tenant AND g.store_id=store AND g.create_attempt_id=p_attempt
   AND (g.request_id<>p_attempt OR g.buyer_id<>owner_id OR g.principal_id<>principal OR g.quote_id IS NOT NULL OR g.consumed_at IS NOT NULL
    OR NOT EXISTS(SELECT 1 FROM inbox.order_for_buyer r WHERE r.tenant_id=g.tenant_id AND r.store_id=g.store_id
     AND r.create_attempt_id=p_attempt AND r.request_id=g.request_id AND r.bundle_id=g.bundle_id))) THEN
  RAISE EXCEPTION 'attempt_has_effects' USING ERRCODE='PT409';
 END IF;
 UPDATE inbox.order_for_buyer r SET state='released',updated_at=clock_timestamp()
  WHERE r.tenant_id=tenant AND r.store_id=store AND r.create_attempt_id=p_attempt AND r.request_id=p_attempt;
 GET DIAGNOSTICS released=ROW_COUNT;
 UPDATE claims.merchant_origin_grants g SET expires_at=least(g.expires_at,clock_timestamp())
  WHERE g.tenant_id=tenant AND g.store_id=store AND g.create_attempt_id=p_attempt AND g.request_id=p_attempt;
 -- Retain both audit records; never compensate a bound/consumed grant or delete
 -- live-price uses. Any error propagates and rolls back this entire release.
 PERFORM set_config('app.tenant_id',coalesce(prior[1],''),true),set_config('app.store_id',coalesce(prior[2],''),true),
  set_config('app.principal_id',coalesce(prior[3],''),true),set_config('app.buyer_id',coalesce(prior[4],''),true),
  set_config('app.buyer_session_id',coalesce(prior[5],''),true);
 RETURN released;
EXCEPTION WHEN OTHERS THEN
 PERFORM set_config('app.tenant_id',coalesce(prior[1],''),true),set_config('app.store_id',coalesce(prior[2],''),true),
  set_config('app.principal_id',coalesce(prior[3],''),true),set_config('app.buyer_id',coalesce(prior[4],''),true),
  set_config('app.buyer_session_id',coalesce(prior[5],''),true);
 RAISE;
END $$;
ALTER FUNCTION claims.order_attempt_release_admission(uuid,bigint) OWNER TO commerce_claims_writer;
REVOKE ALL ON FUNCTION claims.order_attempt_release_admission(uuid,bigint) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION claims.order_attempt_release_admission(uuid,bigint) TO commerce_order_attempt_owner;
COMMENT ON FUNCTION claims.order_attempt_release_admission(uuid,bigint) IS
 'Private abort admission cleanup: exact fenced attempt/request/hash/original owner/principal; sorted bundle/reservation/grant locks, only pending order-null reservations and unused unbound grants. Retains audit; no unrelated-order gate or compensation.';

CREATE FUNCTION ops.order_attempt_abort_fenced(p_attempt uuid,p_generation bigint,p_code text)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE ctx jsonb; owner_status text; v_receipt jsonb;
 prior_tenant text:=current_setting('app.tenant_id',true);
 prior_store text:=current_setting('app.store_id',true);
BEGIN
 ctx:=ops.order_attempt_fence_context(p_attempt,p_generation);
 IF p_code IS NULL OR p_code<>ALL(ARRAY['EXPIRED','ERASED','PLAN_INVALID','PLAN_UNAVAILABLE',
  'insufficient_inventory','resource_unavailable','bundle_reserved','bundle_order_exists',
  'invalid_destination','payment_unavailable','limit_exceeded']::text[]) THEN
  RAISE EXCEPTION 'invalid_abort_reason' USING ERRCODE='PT400';
 END IF;
 owner_status:=buyer.order_attempt_lock_owners(p_attempt,p_generation);
 IF (p_code='EXPIRED' AND clock_timestamp()<(ctx->>'execute_by')::timestamptz)
  OR (p_code='ERASED' AND owner_status<>'erased') THEN
  RAISE EXCEPTION 'abort_reason_not_proven' USING ERRCODE='PT409';
 END IF;
 -- A locked finished PENDING proves no committed C2. These domain inspections
 -- additionally reject same-Tx partials; executor must roll back its savepoint.
 PERFORM buyer.order_attempt_assert_no_effects(p_attempt,p_generation);
 PERFORM checkout.order_attempt_assert_no_effects(p_attempt,p_generation);
 PERFORM claims.order_attempt_release_admission(p_attempt,p_generation);
 v_receipt:=jsonb_build_object('state','ABORTED','attempt_id',p_attempt,'intent_generation',p_generation,'code',p_code);
 -- The witness is owned by the frozen metadata accessor, independently of the
 -- terminal row's storage layout. Revalidate immediately before the sole write.
 PERFORM ops.order_attempt_fence_context(p_attempt,p_generation);
 PERFORM set_config('app.tenant_id',ctx->>'tenant_id',true),set_config('app.store_id',ctx->>'store_id',true);
 UPDATE ops.order_attempts a SET state='ABORTED',receipt=v_receipt,settled_at=clock_timestamp(),
  plan_ciphertext=NULL,plan_key_version=NULL
  WHERE a.tenant_id=(ctx->>'tenant_id')::uuid AND a.store_id=(ctx->>'store_id')::uuid
   AND a.attempt_id=p_attempt AND a.intent_generation=p_generation
   AND a.state='PENDING' AND a.admission_finished;
 IF NOT FOUND THEN RAISE EXCEPTION 'attempt_not_fenced' USING ERRCODE='PT409'; END IF;
 -- The existing native terminal trigger drops every subject-index row in this Tx.
 PERFORM set_config('app.tenant_id',coalesce(prior_tenant,''),true),set_config('app.store_id',coalesce(prior_store,''),true);
 RETURN jsonb_build_object('state','ABORTED','receipt',v_receipt);
EXCEPTION WHEN OTHERS THEN
 PERFORM set_config('app.tenant_id',coalesce(prior_tenant,''),true),set_config('app.store_id',coalesce(prior_store,''),true);
 RAISE;
END $$;
ALTER FUNCTION ops.order_attempt_abort_fenced(uuid,bigint,text) OWNER TO commerce_order_attempt_owner;
REVOKE ALL ON FUNCTION ops.order_attempt_abort_fenced(uuid,bigint,text) FROM PUBLIC;
COMMENT ON FUNCTION ops.order_attempt_abort_fenced(uuid,bigint,text) IS
 'Private terminal abort: closed definitive codes, current-xid witness, all indexed owner locks and native zero-effects proof before own admission release and immutable metadata receipt. No plan/key access, earlier locks, external effects, commits or transient-error inference.';

CREATE FUNCTION ops.abort_order_attempt(p_hash bytea,p_store uuid,p_attempt uuid,p_generation bigint,p_code text)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE fenced jsonb;
BEGIN
 -- Public attempt entry supplies actual-login authority/current read and locks
 -- FIRST. Terminal replay precedes newly supplied reason validation/decryption.
 fenced:=ops.lock_order_attempt(p_hash,p_store,p_attempt,p_generation);
 IF fenced->>'state' IN ('COMMITTED','ABORTED') THEN RETURN fenced; END IF;
 RETURN ops.order_attempt_abort_fenced(p_attempt,p_generation,p_code);
END $$;
ALTER FUNCTION ops.abort_order_attempt(bytea,uuid,uuid,bigint,text) OWNER TO commerce_order_attempt_owner;
REVOKE ALL ON FUNCTION ops.abort_order_attempt(bytea,uuid,uuid,bigint,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION ops.abort_order_attempt(bytea,uuid,uuid,bigint,text) TO commerce_runtime,commerce_claims_worker;
COMMENT ON FUNCTION ops.abort_order_attempt(bytea,uuid,uuid,bigint,text) IS
 'order-attempt-v1 §§1/3: actual runtime current read or claims worker authority; first take exact attempt fence, replay immutable terminal receipt or metadata-only definitive abort after native absence proof. PLAN_UNAVAILABLE classification belongs to trusted executor; no unknown failure becomes definitive.';
