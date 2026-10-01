-- 0091 promotions (contracts/storefront-v2.md §F, R4 unit promotions): merchant discount codes applied INSIDE the server quote, counted
-- atomically at order placement.
-- Owning package: internal/promotions (Go calls only the definers below). Owner role of every definer: commerce_checkout_writer (the role that
-- already writes checkout.* and ops.command_results for the bank-transfer settings, 0088); EXECUTE goes to commerce_runtime (merchant admin),
-- commerce_buyer_runtime (quote-time advisory check) and commerce_checkout_runtime (BeginCheckout's authoritative redeem).
--
-- Design decisions (binding for readers of this file):
--   * Usage is DERIVED, never a counter. A redemption row is written in the BeginCheckout transaction; "used" = redemptions whose order is not
--     CANCELLED. expire_held / refund cancel / CVS cancel already move an order to CANCELLED, so a cancelled or expired order frees its use
--     with no change to any of those frozen functions. A refunded or shipped order keeps counting on purpose (§F).
--   * Atomicity: promotions.redeem locks the code row FOR UPDATE, then counts, then inserts. Every placement with the same code serialises on
--     that row lock; under READ COMMITTED the count after the wait sees every earlier committed redemption, so the limit cannot be exceeded.
--   * redeem reads the code id / version / discount from the ORDER's stored snapshot (written by begin_hold from the verified quote), never from
--     a caller argument, so Go cannot redeem a different code or amount than the one the order was priced with.
--   * No money is computed here. The discount amount is computed once, in Go (internal/pricing CalculateWith); SQL only compares versions and
--     enforces the usage rules (window, minimum, limits, status).
--   * begin_hold (post_river/0018) is NOT redefined: redeem runs after it in the same Go transaction, so any refusal rolls the order, the
--     stock hold, the expiry job and the redemption back together.
-- Non-goals: no shipping discount, no stacking (one code per order), no automatic (codeless) discount, no per-SKU targeting, no refund-time
-- usage release, no buyer-visible list of codes (a buyer only ever presents a code).

CREATE SCHEMA promotions;
COMMENT ON SCHEMA promotions IS '0091 / internal/promotions: merchant discount codes and their redemptions. Written only by the definers of this schema (owner commerce_checkout_writer).';
GRANT USAGE ON SCHEMA promotions TO commerce_checkout_writer, commerce_runtime, commerce_buyer_runtime, commerce_checkout_runtime, commerce_privacy_writer;

CREATE TABLE promotions.codes (
 tenant_id uuid NOT NULL, store_id uuid NOT NULL, id uuid NOT NULL DEFAULT gen_random_uuid(),
 code text NOT NULL CHECK(code ~ '^[A-Z0-9-]{3,24}$'),
 kind text NOT NULL CHECK(kind IN ('percent','fixed')),
 percent integer CHECK(percent BETWEEN 1 AND 90),
 fixed_minor bigint CHECK(fixed_minor BETWEEN 1 AND 1000000000000),
 min_subtotal_minor bigint NOT NULL DEFAULT 0 CHECK(min_subtotal_minor BETWEEN 0 AND 1000000000000),
 starts_at timestamptz, ends_at timestamptz,
 total_limit integer CHECK(total_limit BETWEEN 1 AND 1000000000),
 per_buyer_limit integer CHECK(per_buyer_limit BETWEEN 1 AND 1000000),
 status text NOT NULL DEFAULT 'active' CHECK(status IN ('active','paused')),
 version bigint NOT NULL CHECK(version>0),
 created_at timestamptz NOT NULL, updated_at timestamptz NOT NULL,
 PRIMARY KEY(tenant_id,store_id,id),
 UNIQUE(tenant_id,store_id,code),
 FOREIGN KEY(tenant_id,store_id) REFERENCES control.stores(tenant_id,id),
 CHECK((kind='percent')=(percent IS NOT NULL) AND (kind='fixed')=(fixed_minor IS NOT NULL)),
 CHECK(starts_at IS NULL OR ends_at IS NULL OR ends_at>starts_at)
);
COMMENT ON TABLE promotions.codes IS 'internal/promotions: one merchant discount code per (store, code). Written only by promotions.admin_create / admin_update (merchant, pricing:write); read by quote_check / redeem. FORCE RLS, writer role only. Non-goal: no usage counter (usage is derived from redemptions).';
COMMENT ON COLUMN promotions.codes.code IS 'Upper-case, ^[A-Z0-9-]{3,24}$, immutable after create; buyers type any case (internal/promotions.Normalize).';
COMMENT ON COLUMN promotions.codes.version IS 'CAS version, bumped on every admin_update (also pause). The quote snapshot freezes it; redeem refuses promo_changed when it moved.';
COMMENT ON COLUMN promotions.codes.min_subtotal_minor IS 'Minimum PRE-discount merchandise subtotal (store-currency minor units); 0 = none.';
COMMENT ON COLUMN promotions.codes.starts_at IS 'Optional instant; the admin UI enters Asia/Taipei wall time and sends +08:00.';

CREATE TABLE promotions.redemptions (
 tenant_id uuid NOT NULL, store_id uuid NOT NULL, code_id uuid NOT NULL, order_id uuid NOT NULL, owner_id uuid NOT NULL,
 email_hash bytea CHECK(octet_length(email_hash)=32), phone_hash bytea CHECK(octet_length(phone_hash)=32),
 discount_minor bigint NOT NULL CHECK(discount_minor BETWEEN 0 AND 1000000000000),
 created_at timestamptz NOT NULL,
 PRIMARY KEY(tenant_id,store_id,order_id),
 FOREIGN KEY(tenant_id,store_id,code_id) REFERENCES promotions.codes(tenant_id,store_id,id),
 FOREIGN KEY(tenant_id,store_id,owner_id,order_id) REFERENCES checkout.orders(tenant_id,store_id,owner_id,id)
);
CREATE INDEX redemptions_by_code ON promotions.redemptions(tenant_id,store_id,code_id);
CREATE INDEX redemptions_by_owner ON promotions.redemptions(tenant_id,store_id,code_id,owner_id);
CREATE INDEX redemptions_by_email ON promotions.redemptions(tenant_id,store_id,code_id,email_hash) WHERE email_hash IS NOT NULL;
CREATE INDEX redemptions_by_phone ON promotions.redemptions(tenant_id,store_id,code_id,phone_hash) WHERE phone_hash IS NOT NULL;
COMMENT ON TABLE promotions.redemptions IS 'internal/promotions: one row per order placed with a code, inserted by promotions.redeem in the BeginCheckout transaction. Usage = rows whose checkout.orders.commercial_state is not CANCELLED. email_hash / phone_hash are sha256(store_id || lower(email) | digits(phone)) used only for the per-buyer limit (pseudonymous; cleared by promotions.clear_buyer on erasure). Non-goal: no buyer-visible read.';
COMMENT ON COLUMN promotions.redemptions.owner_id IS 'Buyer capability owner of the order (per-buyer limit identity of last resort).';

DO $$ DECLARE r text; BEGIN
 FOREACH r IN ARRAY ARRAY['promotions.codes','promotions.redemptions'] LOOP
  EXECUTE format('ALTER TABLE %s ENABLE ROW LEVEL SECURITY',r);
  EXECUTE format('ALTER TABLE %s FORCE ROW LEVEL SECURITY',r);
  EXECUTE format('REVOKE ALL ON %s FROM PUBLIC',r);
  -- GUC-scoped like bank_transfer_settings (0088): every definer sets or verifies app.tenant_id/app.store_id first.
  EXECUTE format('CREATE POLICY promotions_writer ON %s TO commerce_checkout_writer
   USING(tenant_id=nullif(current_setting(''app.tenant_id'',true),'''')::uuid AND store_id=nullif(current_setting(''app.store_id'',true),'''')::uuid)
   WITH CHECK(tenant_id=nullif(current_setting(''app.tenant_id'',true),'''')::uuid AND store_id=nullif(current_setting(''app.store_id'',true),'''')::uuid)',r);
 END LOOP;
END $$;
GRANT SELECT,INSERT ON promotions.codes,promotions.redemptions TO commerce_checkout_writer;
GRANT UPDATE(percent,fixed_minor,kind,min_subtotal_minor,starts_at,ends_at,total_limit,per_buyer_limit,status,version,updated_at) ON promotions.codes TO commerce_checkout_writer;
-- Erasure clears the hash columns only (promotions.clear_buyer); no other column of a redemption ever changes.
GRANT UPDATE(email_hash,phone_hash) ON promotions.redemptions TO commerce_checkout_writer;

-- ---------------------------------------------------------------------------------------------------
-- Shared rule check. Internal: only the definers below (all owned by commerce_checkout_writer) call it. Returns the refusal code or NULL.
-- The caller has already set/verified app.tenant_id/app.store_id and, for redeem, holds the code row lock.
-- ---------------------------------------------------------------------------------------------------
CREATE FUNCTION promotions.refusal_for(p_code promotions.codes,p_subtotal bigint,p_owner uuid,p_email bytea,p_phone bytea) RETURNS text
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_now timestamptz:=clock_timestamp(); v_used bigint; v_buyer bigint;
BEGIN
 -- paused is "invalid" to a buyer: nothing to enumerate about a code the merchant switched off.
 IF p_code.status<>'active' THEN RETURN 'promo_invalid'; END IF;
 IF p_code.starts_at IS NOT NULL AND v_now<p_code.starts_at THEN RETURN 'promo_not_started'; END IF;
 IF p_code.ends_at IS NOT NULL AND v_now>=p_code.ends_at THEN RETURN 'promo_expired'; END IF;
 IF p_subtotal<p_code.min_subtotal_minor THEN RETURN 'promo_min_subtotal'; END IF;
 IF p_code.total_limit IS NOT NULL THEN
  SELECT count(*) INTO v_used FROM promotions.redemptions r
   JOIN checkout.orders o ON o.tenant_id=r.tenant_id AND o.store_id=r.store_id AND o.owner_id=r.owner_id AND o.id=r.order_id
   WHERE r.tenant_id=p_code.tenant_id AND r.store_id=p_code.store_id AND r.code_id=p_code.id AND o.commercial_state<>'CANCELLED';
  IF v_used>=p_code.total_limit THEN RETURN 'promo_used_up'; END IF;
 END IF;
 IF p_code.per_buyer_limit IS NOT NULL THEN
  -- Any identity match counts: capability owner, e-mail hash, phone hash (the last two only when known). Weakness documented in §F.
  SELECT count(*) INTO v_buyer FROM promotions.redemptions r
   JOIN checkout.orders o ON o.tenant_id=r.tenant_id AND o.store_id=r.store_id AND o.owner_id=r.owner_id AND o.id=r.order_id
   WHERE r.tenant_id=p_code.tenant_id AND r.store_id=p_code.store_id AND r.code_id=p_code.id AND o.commercial_state<>'CANCELLED'
    AND (r.owner_id=p_owner OR (p_email IS NOT NULL AND r.email_hash=p_email) OR (p_phone IS NOT NULL AND r.phone_hash=p_phone));
  IF v_buyer>=p_code.per_buyer_limit THEN RETURN 'promo_buyer_limit'; END IF;
 END IF;
 RETURN NULL;
END $$;
ALTER FUNCTION promotions.refusal_for(promotions.codes,bigint,uuid,bytea,bytea) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION promotions.refusal_for(promotions.codes,bigint,uuid,bytea,bytea) FROM PUBLIC;
COMMENT ON FUNCTION promotions.refusal_for(promotions.codes,bigint,uuid,bytea,bytea) IS 'internal/promotions: the single usage-rule check (status, window, minimum, total and per-buyer limit) shared by quote_check (advisory) and redeem (authoritative, under the code row lock). No EXECUTE for any login role; only the definers of this schema call it. Non-goal: computes no money.';

-- ---------------------------------------------------------------------------------------------------
-- Buyer, quote time: advisory validation + the frozen effect. Runs in the buyer scoped transaction (GUCs set by buyer.WithScope); verifies
-- them against its arguments and fails closed. No lock: the authoritative check is redeem at BeginCheckout.
-- ---------------------------------------------------------------------------------------------------
CREATE FUNCTION promotions.quote_check(p_tenant uuid,p_store uuid,p_owner uuid,p_code text,p_subtotal bigint) RETURNS jsonb
LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE c promotions.codes; v_refusal text;
BEGIN
 IF p_tenant IS NULL OR p_store IS NULL OR p_owner IS NULL OR p_code IS NULL OR p_subtotal IS NULL OR p_subtotal<0 THEN
  RAISE EXCEPTION 'invalid promotion check' USING ERRCODE='PT400'; END IF;
 IF current_setting('app.tenant_id',true) IS DISTINCT FROM p_tenant::text OR current_setting('app.store_id',true) IS DISTINCT FROM p_store::text
  OR current_setting('app.buyer_id',true) IS DISTINCT FROM p_owner::text THEN
  RAISE EXCEPTION 'promotion scope' USING ERRCODE='PT403'; END IF;
 IF p_code !~ '^[A-Za-z0-9-]{3,24}$' THEN RAISE EXCEPTION 'promo_invalid' USING ERRCODE='PT422'; END IF;
 SELECT * INTO c FROM promotions.codes x WHERE x.tenant_id=p_tenant AND x.store_id=p_store AND x.code=upper(p_code);
 IF NOT FOUND THEN RAISE EXCEPTION 'promo_invalid' USING ERRCODE='PT422'; END IF;
 v_refusal:=promotions.refusal_for(c,p_subtotal,p_owner,NULL,NULL);
 IF v_refusal IS NOT NULL THEN RAISE EXCEPTION '%',v_refusal USING ERRCODE='PT422'; END IF;
 RETURN jsonb_build_object('id',c.id,'code',c.code,'version',c.version,'kind',c.kind,'percent',coalesce(c.percent,0),'fixed_minor',coalesce(c.fixed_minor,0));
END $$;
ALTER FUNCTION promotions.quote_check(uuid,uuid,uuid,text,bigint) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION promotions.quote_check(uuid,uuid,uuid,text,bigint) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION promotions.quote_check(uuid,uuid,uuid,text,bigint) TO commerce_buyer_runtime;
COMMENT ON FUNCTION promotions.quote_check(uuid,uuid,uuid,text,bigint) IS 'internal/storefront CreateQuote (buyer runtime, inside the buyer scoped transaction): advisory check of a typed code against the PRE-discount merchandise subtotal; returns {id,code,version,kind,percent,fixed_minor} or raises PT422 with a promo_* code. Verifies the buyer GUCs equal its arguments. Non-goals: no lock, no write, no per-buyer check by e-mail/phone (unknown at quote time).';

-- ---------------------------------------------------------------------------------------------------
-- Buyer, BeginCheckout: authoritative redeem. Called by internal/checkout.Begin after checkout.begin_hold, same transaction.
-- ---------------------------------------------------------------------------------------------------
CREATE FUNCTION promotions.redeem(p_hash bytea,p_store uuid,p_order uuid,p_email_hash bytea,p_phone_hash bytea) RETURNS void
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE s record; o record; c promotions.codes; v_code uuid; v_version bigint; v_discount bigint; v_subtotal bigint; v_refusal text;
BEGIN
 IF p_hash IS NULL OR octet_length(p_hash)<>32 OR p_store IS NULL OR p_order IS NULL
  OR (p_email_hash IS NOT NULL AND octet_length(p_email_hash)<>32) OR (p_phone_hash IS NOT NULL AND octet_length(p_phone_hash)<>32)
  OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid redemption' USING ERRCODE='PT400'; END IF;
 SELECT * INTO s FROM buyer.resolve_scope(p_hash,p_store);
 IF NOT FOUND THEN RAISE EXCEPTION 'invalid buyer capability' USING ERRCODE='PT401'; END IF;
 PERFORM set_config('app.tenant_id',s.tenant_id::text,true),set_config('app.store_id',p_store::text,true),
  set_config('app.buyer_id',s.owner_id::text,true),set_config('app.buyer_session_id',s.session_id::text,true),set_config('app.principal_id','',true);
 -- The order must be this buyer's, placed by this session in this transaction's minute (same fence as set_order_buyer_email).
 SELECT x.snapshot,x.owner_id INTO o FROM checkout.orders x WHERE x.tenant_id=s.tenant_id AND x.store_id=p_store AND x.owner_id=s.owner_id
  AND x.id=p_order AND x.creator_session_id=s.session_id AND x.created_at>=clock_timestamp()-interval '1 minute' AND x.commercial_state<>'CANCELLED';
 IF NOT FOUND THEN RAISE EXCEPTION 'order not found' USING ERRCODE='PT404'; END IF;
 v_code:=(o.snapshot#>>'{quote,promotion,id}')::uuid;
 v_version:=(o.snapshot#>>'{quote,promotion,version}')::bigint;
 v_discount:=(o.snapshot#>>'{quote,amount,discount_minor}')::bigint;
 v_subtotal:=(o.snapshot#>>'{quote,amount,subtotal_minor}')::bigint;
 IF v_code IS NULL OR v_version IS NULL OR v_discount IS NULL OR v_subtotal IS NULL THEN
  RAISE EXCEPTION 'order has no promotion' USING ERRCODE='PT400'; END IF;
 -- The serialisation point: every placement with this code waits here. The count below (READ COMMITTED: a fresh snapshot per statement)
 -- therefore sees every redemption committed before this one, so total_limit and per_buyer_limit cannot be oversubscribed.
 SELECT * INTO c FROM promotions.codes x WHERE x.tenant_id=s.tenant_id AND x.store_id=p_store AND x.id=v_code FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'promo_invalid' USING ERRCODE='PT422'; END IF;
 IF c.version<>v_version THEN RAISE EXCEPTION 'promo_changed' USING ERRCODE='PT422'; END IF;
 v_refusal:=promotions.refusal_for(c,v_subtotal,s.owner_id,p_email_hash,p_phone_hash);
 IF v_refusal IS NOT NULL THEN RAISE EXCEPTION '%',v_refusal USING ERRCODE='PT422'; END IF;
 INSERT INTO promotions.redemptions(tenant_id,store_id,code_id,order_id,owner_id,email_hash,phone_hash,discount_minor,created_at)
 VALUES(s.tenant_id,p_store,v_code,p_order,s.owner_id,p_email_hash,p_phone_hash,v_discount,clock_timestamp());
END $$;
ALTER FUNCTION promotions.redeem(bytea,uuid,uuid,bytea,bytea) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION promotions.redeem(bytea,uuid,uuid,bytea,bytea) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION promotions.redeem(bytea,uuid,uuid,bytea,bytea) TO commerce_checkout_runtime;
COMMENT ON FUNCTION promotions.redeem(bytea,uuid,uuid,bytea,bytea) IS 'internal/checkout Begin, after checkout.begin_hold in the same transaction: locks the code row FOR UPDATE, re-checks version + usage rules, inserts the redemption. Code id, version and discount are read from the ORDER snapshot, never from arguments. PT422 promo_changed|promo_invalid|promo_expired|promo_not_started|promo_min_subtotal|promo_used_up|promo_buyer_limit roll the whole placement back. Non-goals: no money computation, no release (usage is derived from order state).';

-- Erasure hook: customers.apply_erasure (0078) calls this for the erased owner. Clears only the pseudonymous hashes; the redemption row
-- (and so the usage count) stays, which is exactly what a limit needs.
CREATE FUNCTION promotions.clear_buyer(p_tenant uuid,p_store uuid,p_owner uuid) RETURNS integer
LANGUAGE sql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
 WITH g AS (SELECT set_config('app.tenant_id',p_tenant::text,true),set_config('app.store_id',p_store::text,true)),
 cleared AS (UPDATE promotions.redemptions SET email_hash=NULL,phone_hash=NULL
   WHERE tenant_id=p_tenant AND store_id=p_store AND owner_id=p_owner AND (email_hash IS NOT NULL OR phone_hash IS NOT NULL)
    AND EXISTS(SELECT 1 FROM g) RETURNING 1)
 SELECT count(*)::integer FROM cleared
$$;
ALTER FUNCTION promotions.clear_buyer(uuid,uuid,uuid) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION promotions.clear_buyer(uuid,uuid,uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION promotions.clear_buyer(uuid,uuid,uuid) TO commerce_privacy_writer;
COMMENT ON FUNCTION promotions.clear_buyer(uuid,uuid,uuid) IS 'customers.apply_erasure only (sole EXECUTE: commerce_privacy_writer): NULLs the pseudonymous email/phone hashes of one owner''s redemptions and returns the row count. Non-goal: no authorization of its own, never deletes a redemption.';

-- ---------------------------------------------------------------------------------------------------
-- Merchant admin. Same shape as payments.set_bank_transfer_settings (0088): resolve_access, GUCs, advisory lock, idempotent receipt, audit,
-- final access fence.
-- ---------------------------------------------------------------------------------------------------
CREATE FUNCTION promotions.code_json(p_code promotions.codes,p_used bigint) RETURNS jsonb
LANGUAGE sql STABLE SET search_path=pg_catalog AS $$
 SELECT jsonb_build_object('id',p_code.id,'code',p_code.code,'kind',p_code.kind,'percent',p_code.percent,'fixed_minor',p_code.fixed_minor,
  'min_subtotal_minor',p_code.min_subtotal_minor,'starts_at',p_code.starts_at,'ends_at',p_code.ends_at,'total_limit',p_code.total_limit,
  'per_buyer_limit',p_code.per_buyer_limit,'status',p_code.status,'version',p_code.version,'used',p_used,'created_at',p_code.created_at)
$$;
ALTER FUNCTION promotions.code_json(promotions.codes,bigint) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION promotions.code_json(promotions.codes,bigint) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION promotions.code_json(promotions.codes,bigint) TO commerce_checkout_writer;
COMMENT ON FUNCTION promotions.code_json(promotions.codes,bigint) IS 'internal/promotions: the merchant JSON shape of one code (contracts/storefront-v2.md §F). Pure projection; SECURITY INVOKER, called only from the admin definers.';

CREATE FUNCTION promotions.admin_list(p_hash bytea,p_store uuid) RETURNS jsonb
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE s record; v_final record; v_out jsonb;
BEGIN
 IF p_hash IS NULL OR octet_length(p_hash)<>32 OR p_store IS NULL OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid promotions read' USING ERRCODE='PT400'; END IF;
 SELECT * INTO s FROM identity.resolve_access(p_hash,p_store,'pricing:read');
 IF s.access_status='unauthorized' THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
 IF s.access_status='not_found' THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
 IF s.access_status IS DISTINCT FROM 'ok' THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 PERFORM set_config('app.tenant_id',s.tenant_id::text,true),set_config('app.store_id',p_store::text,true),
  set_config('app.principal_id',s.principal_id::text,true);
 SELECT coalesce(jsonb_agg(promotions.code_json(c,(SELECT count(*) FROM promotions.redemptions r
    JOIN checkout.orders o ON o.tenant_id=r.tenant_id AND o.store_id=r.store_id AND o.owner_id=r.owner_id AND o.id=r.order_id
    WHERE r.tenant_id=c.tenant_id AND r.store_id=c.store_id AND r.code_id=c.id AND o.commercial_state<>'CANCELLED')) ORDER BY c.created_at DESC,c.id),'[]'::jsonb)
  INTO v_out FROM promotions.codes c WHERE c.tenant_id=s.tenant_id AND c.store_id=p_store;
 SELECT * INTO v_final FROM identity.resolve_access(p_hash,p_store,'pricing:read');
 IF v_final.access_status<>'ok' OR v_final.principal_id IS DISTINCT FROM s.principal_id
  OR v_final.authz_revision IS DISTINCT FROM s.authz_revision THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 RETURN jsonb_build_object('promotions',v_out);
END $$;
ALTER FUNCTION promotions.admin_list(bytea,uuid) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION promotions.admin_list(bytea,uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION promotions.admin_list(bytea,uuid) TO commerce_runtime;
COMMENT ON FUNCTION promotions.admin_list(bytea,uuid) IS 'internal/promotions merchant GET promotions (pricing:read): every code of the store with its derived usage count. Re-authorizes with a final access fence. Non-goal: no buyer data, no redemption rows.';

-- Field validation shared by create and update. A bad body is a coded 422, never a constraint-violation 500.
CREATE FUNCTION promotions.check_fields(p_kind text,p_percent integer,p_fixed bigint,p_min bigint,p_starts timestamptz,p_ends timestamptz,
 p_total integer,p_buyer integer,p_status text) RETURNS void
LANGUAGE plpgsql IMMUTABLE SET search_path=pg_catalog AS $$
BEGIN
 IF p_kind IS NULL OR p_min IS NULL OR p_status IS NULL OR p_status NOT IN ('active','paused')
  OR p_kind NOT IN ('percent','fixed')
  OR (p_kind='percent' AND (p_percent IS NULL OR p_percent NOT BETWEEN 1 AND 90 OR p_fixed IS NOT NULL))
  OR (p_kind='fixed' AND (p_fixed IS NULL OR p_fixed NOT BETWEEN 1 AND 1000000000000 OR p_percent IS NOT NULL))
  OR p_min NOT BETWEEN 0 AND 1000000000000
  OR (p_starts IS NOT NULL AND NOT isfinite(p_starts)) OR (p_ends IS NOT NULL AND NOT isfinite(p_ends))
  OR (p_starts IS NOT NULL AND p_ends IS NOT NULL AND p_ends<=p_starts)
  OR (p_total IS NOT NULL AND p_total NOT BETWEEN 1 AND 1000000000)
  OR (p_buyer IS NOT NULL AND p_buyer NOT BETWEEN 1 AND 1000000) THEN
  RAISE EXCEPTION 'invalid_promotion' USING ERRCODE='PT422'; END IF;
END $$;
ALTER FUNCTION promotions.check_fields(text,integer,bigint,bigint,timestamptz,timestamptz,integer,integer,text) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION promotions.check_fields(text,integer,bigint,bigint,timestamptz,timestamptz,integer,integer,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION promotions.check_fields(text,integer,bigint,bigint,timestamptz,timestamptz,integer,integer,text) TO commerce_checkout_writer;
COMMENT ON FUNCTION promotions.check_fields(text,integer,bigint,bigint,timestamptz,timestamptz,integer,integer,text) IS 'internal/promotions: mirrors the promotions.codes CHECKs so a bad merchant body is PT422 invalid_promotion. Called only by admin_create / admin_update.';

CREATE FUNCTION promotions.admin_create(p_hash bytea,p_store uuid,p_key text,p_request_hash bytea,p_code text,p_kind text,p_percent integer,
 p_fixed bigint,p_min bigint,p_starts timestamptz,p_ends timestamptz,p_total integer,p_buyer integer,p_status text) RETURNS jsonb
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE s record; v_final record; v_saved bytea; v_response jsonb; v_now timestamptz; c promotions.codes; v_count bigint;
BEGIN
 IF p_hash IS NULL OR octet_length(p_hash)<>32 OR p_store IS NULL OR p_key IS NULL OR p_key !~ '^[A-Za-z0-9_.:-]{8,128}$'
  OR p_request_hash IS NULL OR octet_length(p_request_hash)<>32 OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid promotion create' USING ERRCODE='PT400'; END IF;
 SELECT * INTO s FROM identity.resolve_access(p_hash,p_store,'pricing:write');
 IF s.access_status='unauthorized' THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
 IF s.access_status='not_found' THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
 IF s.access_status IS DISTINCT FROM 'ok' THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 PERFORM set_config('app.tenant_id',s.tenant_id::text,true),set_config('app.store_id',p_store::text,true),
  set_config('app.principal_id',s.principal_id::text,true);
 PERFORM pg_advisory_xact_lock(hashtextextended('promotions.admin|'||s.tenant_id||'|'||p_store,0));
 SELECT r.request_hash,r.response INTO v_saved,v_response FROM ops.command_results r WHERE r.tenant_id=s.tenant_id
  AND r.store_id=p_store AND r.operation='promotions.create' AND r.idempotency_key=p_key;
 IF FOUND THEN
  IF v_saved<>p_request_hash THEN RAISE EXCEPTION 'idempotency_conflict' USING ERRCODE='PT409'; END IF;
 ELSE
  IF p_code IS NULL OR p_code !~ '^[A-Za-z0-9-]{3,24}$' THEN RAISE EXCEPTION 'invalid_promotion' USING ERRCODE='PT422'; END IF;
  PERFORM promotions.check_fields(p_kind,p_percent,p_fixed,p_min,p_starts,p_ends,p_total,p_buyer,p_status);
  SELECT count(*) INTO v_count FROM promotions.codes x WHERE x.tenant_id=s.tenant_id AND x.store_id=p_store;
  IF v_count>=500 THEN RAISE EXCEPTION 'invalid_promotion' USING ERRCODE='PT422'; END IF;
  v_now:=clock_timestamp();
  BEGIN
   INSERT INTO promotions.codes(tenant_id,store_id,code,kind,percent,fixed_minor,min_subtotal_minor,starts_at,ends_at,total_limit,per_buyer_limit,
    status,version,created_at,updated_at)
   VALUES(s.tenant_id,p_store,upper(p_code),p_kind,p_percent,p_fixed,p_min,p_starts,p_ends,p_total,p_buyer,p_status,1,v_now,v_now) RETURNING * INTO c;
  EXCEPTION WHEN unique_violation THEN RAISE EXCEPTION 'promo_exists' USING ERRCODE='PT409';
  END;
  INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action) VALUES(s.tenant_id,p_store,s.principal_id,'promotions.created');
  v_response:=promotions.code_json(c,0);
  INSERT INTO ops.command_results(tenant_id,store_id,operation,idempotency_key,request_hash,response,principal_id)
   VALUES(s.tenant_id,p_store,'promotions.create',p_key,p_request_hash,v_response,s.principal_id);
 END IF;
 SELECT * INTO v_final FROM identity.resolve_access(p_hash,p_store,'pricing:write');
 IF v_final.access_status='unauthorized' THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
 IF v_final.access_status='not_found' THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
 IF v_final.access_status<>'ok' OR v_final.tenant_id IS DISTINCT FROM s.tenant_id
  OR v_final.principal_id IS DISTINCT FROM s.principal_id OR v_final.authz_revision IS DISTINCT FROM s.authz_revision THEN
  RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 RETURN v_response;
END $$;
ALTER FUNCTION promotions.admin_create(bytea,uuid,text,bytea,text,text,integer,bigint,bigint,timestamptz,timestamptz,integer,integer,text) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION promotions.admin_create(bytea,uuid,text,bytea,text,text,integer,bigint,bigint,timestamptz,timestamptz,integer,integer,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION promotions.admin_create(bytea,uuid,text,bytea,text,text,integer,bigint,bigint,timestamptz,timestamptz,integer,integer,text) TO commerce_runtime;
COMMENT ON FUNCTION promotions.admin_create(bytea,uuid,text,bytea,text,text,integer,bigint,bigint,timestamptz,timestamptz,integer,integer,text) IS 'internal/promotions merchant POST promotions (pricing:write): per-store advisory lock, idempotent receipt (ops.command_results), audit row promotions.created, <=500 codes per store. PT409 promo_exists on a duplicate code, PT422 invalid_promotion on a rule violation. Non-goal: never touches placed orders.';

CREATE FUNCTION promotions.admin_update(p_hash bytea,p_store uuid,p_key text,p_request_hash bytea,p_id uuid,p_expected_version bigint,p_kind text,
 p_percent integer,p_fixed bigint,p_min bigint,p_starts timestamptz,p_ends timestamptz,p_total integer,p_buyer integer,p_status text) RETURNS jsonb
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE s record; v_final record; v_saved bytea; v_response jsonb; v_now timestamptz; c promotions.codes; v_used bigint;
BEGIN
 IF p_hash IS NULL OR octet_length(p_hash)<>32 OR p_store IS NULL OR p_key IS NULL OR p_key !~ '^[A-Za-z0-9_.:-]{8,128}$'
  OR p_request_hash IS NULL OR octet_length(p_request_hash)<>32 OR p_id IS NULL OR p_expected_version IS NULL OR p_expected_version<1
  OR p_expected_version>=9223372036854775807 OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'invalid promotion update' USING ERRCODE='PT400'; END IF;
 SELECT * INTO s FROM identity.resolve_access(p_hash,p_store,'pricing:write');
 IF s.access_status='unauthorized' THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
 IF s.access_status='not_found' THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
 IF s.access_status IS DISTINCT FROM 'ok' THEN RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 PERFORM set_config('app.tenant_id',s.tenant_id::text,true),set_config('app.store_id',p_store::text,true),
  set_config('app.principal_id',s.principal_id::text,true);
 PERFORM pg_advisory_xact_lock(hashtextextended('promotions.admin|'||s.tenant_id||'|'||p_store,0));
 SELECT r.request_hash,r.response INTO v_saved,v_response FROM ops.command_results r WHERE r.tenant_id=s.tenant_id
  AND r.store_id=p_store AND r.operation='promotions.update' AND r.idempotency_key=p_key;
 IF FOUND THEN
  IF v_saved<>p_request_hash THEN RAISE EXCEPTION 'idempotency_conflict' USING ERRCODE='PT409'; END IF;
 ELSE
  PERFORM promotions.check_fields(p_kind,p_percent,p_fixed,p_min,p_starts,p_ends,p_total,p_buyer,p_status);
  -- FOR UPDATE: waits for an in-flight redeem of this code, so the version bump and a placement never interleave.
  SELECT * INTO c FROM promotions.codes x WHERE x.tenant_id=s.tenant_id AND x.store_id=p_store AND x.id=p_id FOR UPDATE;
  IF NOT FOUND THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
  IF c.version<>p_expected_version THEN RAISE EXCEPTION 'version_changed' USING ERRCODE='PT409'; END IF;
  v_now:=clock_timestamp();
  UPDATE promotions.codes SET kind=p_kind,percent=p_percent,fixed_minor=p_fixed,min_subtotal_minor=p_min,starts_at=p_starts,ends_at=p_ends,
   total_limit=p_total,per_buyer_limit=p_buyer,status=p_status,version=c.version+1,updated_at=v_now
   WHERE tenant_id=s.tenant_id AND store_id=p_store AND id=p_id RETURNING * INTO c;
  SELECT count(*) INTO v_used FROM promotions.redemptions r
   JOIN checkout.orders o ON o.tenant_id=r.tenant_id AND o.store_id=r.store_id AND o.owner_id=r.owner_id AND o.id=r.order_id
   WHERE r.tenant_id=c.tenant_id AND r.store_id=c.store_id AND r.code_id=c.id AND o.commercial_state<>'CANCELLED';
  INSERT INTO ops.audit_events(tenant_id,store_id,principal_id,action) VALUES(s.tenant_id,p_store,s.principal_id,'promotions.updated');
  v_response:=promotions.code_json(c,v_used);
  INSERT INTO ops.command_results(tenant_id,store_id,operation,idempotency_key,request_hash,response,principal_id)
   VALUES(s.tenant_id,p_store,'promotions.update',p_key,p_request_hash,v_response,s.principal_id);
 END IF;
 SELECT * INTO v_final FROM identity.resolve_access(p_hash,p_store,'pricing:write');
 IF v_final.access_status='unauthorized' THEN RAISE EXCEPTION 'unauthorized' USING ERRCODE='PT401'; END IF;
 IF v_final.access_status='not_found' THEN RAISE EXCEPTION 'not found' USING ERRCODE='PT404'; END IF;
 IF v_final.access_status<>'ok' OR v_final.tenant_id IS DISTINCT FROM s.tenant_id
  OR v_final.principal_id IS DISTINCT FROM s.principal_id OR v_final.authz_revision IS DISTINCT FROM s.authz_revision THEN
  RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403'; END IF;
 RETURN v_response;
END $$;
ALTER FUNCTION promotions.admin_update(bytea,uuid,text,bytea,uuid,bigint,text,integer,bigint,bigint,timestamptz,timestamptz,integer,integer,text) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION promotions.admin_update(bytea,uuid,text,bytea,uuid,bigint,text,integer,bigint,bigint,timestamptz,timestamptz,integer,integer,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION promotions.admin_update(bytea,uuid,text,bytea,uuid,bigint,text,integer,bigint,bigint,timestamptz,timestamptz,integer,integer,text) TO commerce_runtime;
COMMENT ON FUNCTION promotions.admin_update(bytea,uuid,text,bytea,uuid,bigint,text,integer,bigint,bigint,timestamptz,timestamptz,integer,integer,text) IS 'internal/promotions merchant POST promotions/{id} (pricing:write): CAS on version, locks the code row FOR UPDATE (serialises with redeem), bumps the version (so an in-flight quote answers promo_changed at Begin), audit row promotions.updated, idempotent receipt. The code text is immutable. Non-goal: never touches placed orders or their redemptions.';

-- ---------------------------------------------------------------------------------------------------
-- Erasure: customers.apply_erasure (0078, patched by 0088) also clears the redemption hashes of the erased owner. Same pg_get_functiondef
-- patch pattern as 0088 section J: it fails loudly when the live definition is not the expected one (the needle must occur exactly once).
-- ---------------------------------------------------------------------------------------------------
DO $$
DECLARE v_fn text; v_needle text:=' PERFORM checkout.clear_buyer_email(p_tenant,p_store,p_owner);';
BEGIN
 v_fn:=pg_get_functiondef('customers.apply_erasure(uuid,uuid,uuid)'::regprocedure);
 IF length(v_fn)-length(replace(v_fn,v_needle,''))<>length(v_needle) OR position('promotions.clear_buyer' IN v_fn)>0 THEN
  RAISE EXCEPTION 'customers.apply_erasure has an unexpected shape for the promotions patch'; END IF;
 EXECUTE replace(v_fn,v_needle,v_needle||E'\n -- storefront-v2 §F: the pseudonymous e-mail/phone hashes of this owner''s promotion redemptions (the usage count itself stays).\n PERFORM promotions.clear_buyer(p_tenant,p_store,p_owner);');
END $$;
