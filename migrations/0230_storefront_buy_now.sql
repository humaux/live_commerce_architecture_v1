-- Purpose: a checkout-only selection under the original buyer capability.
-- Depends on: 0171 browser/attempt cart provenance and the CVS definers in 0093 and 0206.
-- Used by: storefront Buy now and the buyer CVS selection and store-entry commands.
-- Existing carts retain their IDs, versions, lines and creator provenance.
ALTER TABLE storefront.carts
 ADD COLUMN checkout_only boolean NOT NULL DEFAULT false,
 ADD CONSTRAINT carts_checkout_only_identity CHECK (NOT checkout_only OR (cart_kind='storefront' AND version=1));
DROP INDEX storefront.carts_storefront_owner;
CREATE UNIQUE INDEX carts_storefront_owner ON storefront.carts(tenant_id,store_id,owner_id)
 WHERE cart_kind='storefront' AND NOT checkout_only;

ALTER TABLE fulfillment.cvs_selections DROP CONSTRAINT cvs_selections_return_path_check;
ALTER TABLE fulfillment.cvs_selections ADD CONSTRAINT cvs_selections_return_path_check
 CHECK(return_path ~ '^/(zh-TW|zh-CN|en)/(products/[A-Za-z0-9_-]{1,64}|checkout(/buy-now/[0-9a-f]{8}(-[0-9a-f]{4}){3}-[0-9a-f]{12})?)$');

-- These two definers formerly assumed one cart per owner. Keep their ACLs and
-- resolve the authenticated owner first, then select the explicit cart or the regular cart.
DO $do$
DECLARE
 fn regprocedure;
 body text;
 anchor text := 'AND c.owner_id=v_scope.owner_id FOR SHARE;';
 path_anchor text := $path$IF p_return_path !~ '^/(zh-TW|zh-CN|en)/(products/[A-Za-z0-9_-]{1,64}|checkout)$' THEN$path$;
BEGIN
 FOREACH fn IN ARRAY ARRAY[
  'fulfillment.open_cvs_selection(bytea,uuid,text,bytea,bigint,uuid,text,bytea,text,text,text)'::regprocedure,
  'fulfillment.record_buyer_cvs_store(bytea,uuid,text,bytea,bigint,uuid,text,text,text,text)'::regprocedure
 ] LOOP
  body := pg_get_functiondef(fn);
  IF (length(body)-length(replace(body,anchor,'')))/length(anchor) <> 1 THEN
   RAISE EXCEPTION 'unexpected cart selector in %', fn;
  END IF;
  body := replace(body,anchor,$predicate$AND c.owner_id=v_scope.owner_id AND c.cart_kind='storefront'
   AND (CASE WHEN nullif(current_setting('app.buy_now_cart_id',true),'') IS NULL THEN NOT c.checkout_only
        ELSE c.checkout_only AND c.id=nullif(current_setting('app.buy_now_cart_id',true),'')::uuid END) FOR SHARE;$predicate$);
  IF fn='fulfillment.open_cvs_selection(bytea,uuid,text,bytea,bigint,uuid,text,bytea,text,text,text)'::regprocedure THEN
   IF (length(body)-length(replace(body,path_anchor,'')))/length(path_anchor) <> 1 THEN
    RAISE EXCEPTION 'unexpected return path validation in %', fn;
   END IF;
   body := replace(body,path_anchor,$path$IF p_return_path !~ '^/(zh-TW|zh-CN|en)/(products/[A-Za-z0-9_-]{1,64}|checkout(/buy-now/[0-9a-f]{8}(-[0-9a-f]{4}){3}-[0-9a-f]{12})?)$'
    OR coalesce(substring(p_return_path FROM '/buy-now/([0-9a-f-]+)$'),'') <> coalesce(current_setting('app.buy_now_cart_id',true),'') THEN$path$);
  END IF;
  EXECUTE body;
 END LOOP;
END $do$;
