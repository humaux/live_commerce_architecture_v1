-- 0097 buyer notification locale on checkout.orders (buyer-comms follow-up to 0090). Every buyer mail rendered
-- zh-TW because placement never captured the buyer's language: notify.claim_batch selected k.snapshot->>'locale',
-- a key no writer ever set, so Payload.Locale was always NULL and notify.localeOf fell back to the default.
-- This persists the buyer's storefront locale (zh-CN|zh-TW|en; an empty checkout input means the zh-TW default) on
-- the order at placement and points notify.claim_batch at the real column.
-- Owning package: internal/checkout (writer: checkout.set_order_locale, called in the Begin transaction next to
-- checkout.set_order_buyer_email; reader: internal/notify through the notify.claim_batch payload, never directly).
-- Non-goals: the locale is set once and never updated (an order's mails keep the language it was placed in), it
-- never drives pricing or storefront content, and merchant-created orders (no buyer checkout) stay NULL and
-- render as the zh-TW default.

-- A. Column: same CHECK-twin + column-grant pattern as 0088's buyer_email. NULL only off the buyer checkout path.
ALTER TABLE checkout.orders ADD COLUMN locale text
 CHECK (locale IS NULL OR locale IN ('zh-CN','zh-TW','en'));
COMMENT ON COLUMN checkout.orders.locale IS 'internal/checkout: buyer locale (zh-CN|zh-TW|en) captured at checkout placement for buyer notification template selection (internal/notify localeOf via the notify.claim_batch payload). Set once by checkout.set_order_locale in the Begin transaction; NULL = not placed through buyer checkout (merchant-created orders), rendered as the zh-TW default. Non-goal: never updated after placement, never read by pricing or storefront rendering.';
GRANT UPDATE(locale) ON checkout.orders TO commerce_checkout_writer;

-- B. Placement writer, mirroring checkout.set_order_buyer_email (0088): the buyer capability from the session hash,
-- the order of the creating session inside the placing transaction's minute, set once, never overwritten.
CREATE FUNCTION checkout.set_order_locale(p_hash bytea,p_store uuid,p_order uuid,p_locale text) RETURNS void
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE s record; v_rows integer;
BEGIN
 IF p_hash IS NULL OR octet_length(p_hash)<>32 OR p_store IS NULL OR p_order IS NULL
  OR p_locale IS NULL OR p_locale NOT IN ('zh-CN','zh-TW','en') THEN
  RAISE EXCEPTION 'invalid order locale' USING ERRCODE='PT400'; END IF;
 SELECT * INTO s FROM buyer.resolve_scope(p_hash,p_store);
 IF NOT FOUND THEN RAISE EXCEPTION 'invalid buyer capability' USING ERRCODE='PT401'; END IF;
 PERFORM set_config('app.tenant_id',s.tenant_id::text,true),set_config('app.store_id',p_store::text,true),
  set_config('app.buyer_id',s.owner_id::text,true),set_config('app.buyer_session_id',s.session_id::text,true),set_config('app.principal_id','',true);
 UPDATE checkout.orders SET locale=p_locale WHERE tenant_id=s.tenant_id AND store_id=p_store AND owner_id=s.owner_id AND id=p_order
  AND creator_session_id=s.session_id AND locale IS NULL AND created_at>=clock_timestamp()-interval '1 minute';
 GET DIAGNOSTICS v_rows=ROW_COUNT;
 IF v_rows<>1 THEN RAISE EXCEPTION 'order not found' USING ERRCODE='PT404'; END IF;
END $$;
ALTER FUNCTION checkout.set_order_locale(bytea,uuid,uuid,text) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION checkout.set_order_locale(bytea,uuid,uuid,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION checkout.set_order_locale(bytea,uuid,uuid,text) TO commerce_checkout_runtime;
COMMENT ON FUNCTION checkout.set_order_locale(bytea,uuid,uuid,text) IS 'internal/checkout Begin only (EXECUTE commerce_checkout_runtime): sets checkout.orders.locale once, in the placement transaction, for the creating buyer session. Non-goal: no update after the placing minute, no locale on merchant-created orders (those stay NULL, the renderer zh-TW default).';

-- C. notify.claim_batch feeds the renderer from the real column. Patched in place (0088 section J pattern) instead
--    of re-copying the definer: the needle is the never-written snapshot key, replaced by the column; the patch
--    fails loudly when the live definition is not the expected one. Ownership and grants are re-asserted because
--    re-creating through pg_get_functiondef runs as the migration role.
DO $$
DECLARE v_fn text; v_needle text; v_repl text;
BEGIN
 v_needle:='k.snapshot->>''locale'' AS locale';
 v_repl:='k.locale';
 v_fn:=pg_get_functiondef('notify.claim_batch(integer,integer,integer)'::regprocedure);
 IF (length(v_fn)-length(replace(v_fn,v_needle,'')))<>length(v_needle) THEN
  RAISE EXCEPTION 'notify.claim_batch has an unexpected shape for the locale patch'; END IF;
 EXECUTE replace(v_fn,v_needle,v_repl);
END $$;
ALTER FUNCTION notify.claim_batch(integer,integer,integer) OWNER TO commerce_checkout_writer;
REVOKE ALL ON FUNCTION notify.claim_batch(integer,integer,integer) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION notify.claim_batch(integer,integer,integer) TO commerce_worker;
COMMENT ON FUNCTION notify.claim_batch(integer,integer,integer) IS 'internal/notify worker only; EXECUTE commerce_worker. Housekeeping (UNKNOWN for dead SENDING, stale SKIPPED, 180-day delete), then claims up to p_limit buyer rows (FOR UPDATE SKIP LOCKED) under the daily budget p_daily_cap and p_store_hourly per store, plus at most one merchant batch per store per 5 minutes. Returns the renderer input as jsonb; the recipient address leaves SQL only here, in memory. 0097: the buyer payload locale comes from checkout.orders.locale (set at placement), no longer from the never-written snapshot key.';
