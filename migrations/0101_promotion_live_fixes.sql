-- 0101 promotion + live-tools defect fixes (adversarial review output/kimi-calibration/K3-REVIEW-PROMOTIONS.md, P2 findings).
-- Nothing is renamed, dropped or re-granted differently; the three functions keep their owner, search_path, grants and COMMENTs.
--   * P2-1 (0091 + 0092): promotions.quote_check (and claims.live_prices / claims.preview_live_prices) were declared STABLE but depend
--     on clock_timestamp() — quote_check through promotions.refusal_for (window + limit counts), the live-price definers for link
--     expiry. STABLE freezes now() per statement, so any future multi-row/multi-call use would silently reuse the first evaluation's
--     clock. They are now VOLATILE; the cost is nil (each is called once per request).
--   * P2-2 (0091): quote_check verified the tenant/store/buyer GUCs but not app.principal_id, unlike its 0092 siblings. Added the same
--     buyer-principal fence: app.principal_id must be '' for the buyer path.
-- Non-goals: no schema change, no new roles, no behaviour change for any caller that already sets app.principal_id='' (buyer.WithScope).

CREATE OR REPLACE FUNCTION promotions.quote_check(p_tenant uuid,p_store uuid,p_owner uuid,p_code text,p_subtotal bigint) RETURNS jsonb
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE c promotions.codes; v_refusal text;
BEGIN
 IF p_tenant IS NULL OR p_store IS NULL OR p_owner IS NULL OR p_code IS NULL OR p_subtotal IS NULL OR p_subtotal<0 THEN
  RAISE EXCEPTION 'invalid promotion check' USING ERRCODE='PT400'; END IF;
 IF current_setting('app.tenant_id',true) IS DISTINCT FROM p_tenant::text OR current_setting('app.store_id',true) IS DISTINCT FROM p_store::text
  OR current_setting('app.buyer_id',true) IS DISTINCT FROM p_owner::text
  OR current_setting('app.principal_id',true) IS DISTINCT FROM '' THEN
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

-- P2-1: the live-price definers read clock_timestamp() for link expiry, so STABLE was a lie; the bodies are unchanged.
ALTER FUNCTION claims.live_prices(uuid[],uuid[],uuid[]) VOLATILE;
ALTER FUNCTION claims.preview_live_prices(bytea) VOLATILE;
