-- Purpose: OA1 C1 explicit PII subject discovery and exact private-owner creation.
-- Depends on: native buyer.owners/customer identity, inbox A14 links, identity grants,
-- and ops.order_attempt_admission_context (0172, including stored private_owner).
-- Used by: the order-attempt admission coordinator before/after sorted subject barriers,
-- then after the new attempt fence and before claims/domain locks in the same C1 Tx.
-- Invariants: I01/I02/I09/I11; no contact-derived identity, no discovery row locks,
-- no runtime internal-helper EXECUTE, no coordinator business DML or new privacy table grants.

-- The inbox domain owns its relation. The privacy reader receives only a boolean
-- projection, not table access or contact/social identifiers. NULL customer means
-- validate the conversation, not resolve its linked customer implicitly.
GRANT USAGE ON SCHEMA inbox TO commerce_privacy_writer;
CREATE FUNCTION inbox.order_attempt_customer_link(p_conversation uuid,p_customer uuid)
RETURNS boolean LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_tenant uuid; v_store uuid;
BEGIN
 IF NOT inbox.principal_holds(ARRAY['inbox:read']::text[]) THEN
  RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403';
 END IF;
 v_tenant:=nullif(current_setting('app.tenant_id',true),'')::uuid;
 v_store:=nullif(current_setting('app.store_id',true),'')::uuid;
 RETURN EXISTS(SELECT 1 FROM social.conversations c
  JOIN inbox.conversation_state s ON s.tenant_id=c.tenant_id AND s.store_id=c.store_id AND s.conversation_id=c.id
  WHERE c.tenant_id=v_tenant AND c.store_id=v_store AND c.id=p_conversation
   AND (p_customer IS NULL OR s.customer_id=p_customer));
END $$;
ALTER FUNCTION inbox.order_attempt_customer_link(uuid,uuid) OWNER TO commerce_inbox_writer;
REVOKE ALL ON FUNCTION inbox.order_attempt_customer_link(uuid,uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION inbox.order_attempt_customer_link(uuid,uuid) TO commerce_privacy_writer;
COMMENT ON FUNCTION inbox.order_attempt_customer_link(uuid,uuid) IS
 'Internal C1 privacy projection only: scoped existing conversation and exact explicit A14 customer link; null customer never derives identity. Unlocked, no PII or state returned.';

GRANT USAGE ON SCHEMA customers,buyer TO commerce_order_attempt_owner;
GRANT EXECUTE ON FUNCTION identity.principal_holds(uuid,uuid,uuid,text[]) TO commerce_privacy_writer;
CREATE FUNCTION customers.order_attempt_sources(p_selection jsonb)
RETURNS jsonb LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE
 v_tenant uuid; v_store uuid; v_principal uuid; v_owner uuid; v_conversation uuid; v_active boolean;
 v_items jsonb; v_item jsonb; v_delivery jsonb; v_customer jsonb; v_for jsonb; v_home boolean;
 v_uuid_pattern constant text:='^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$';
 v_ordinals integer[]:='{}'; v_keys text[]:='{}'; v_key text; v_field text; v_value text;
BEGIN
 -- This seam runs before an attempt exists; it cannot use the candidate accessor.
 -- The coordinator alone calls it, with authenticated GUCs. Validate every selection
 -- block before discovery so unindexed PII/source fields cannot enter the frozen plan.
 -- Keep these exact normalized-shape bounds aligned with 0173 admission facts.
 IF jsonb_typeof(p_selection) IS DISTINCT FROM 'object' OR octet_length(p_selection::text)>65536 THEN
  RAISE EXCEPTION 'invalid_selection' USING ERRCODE='PT400';
 END IF;
 IF EXISTS(SELECT 1 FROM jsonb_object_keys(p_selection) k
  WHERE k NOT IN ('schema_version','items','delivery','customer','payment_mode','locale','for','send_payment_link'))
  OR p_selection->'schema_version' IS DISTINCT FROM '1'::jsonb
  OR jsonb_typeof(p_selection->'items') IS DISTINCT FROM 'array'
  OR jsonb_typeof(p_selection->'delivery') IS DISTINCT FROM 'object'
  OR jsonb_typeof(p_selection->'customer') IS DISTINCT FROM 'object'
  OR jsonb_typeof(p_selection->'for') IS DISTINCT FROM 'object'
  OR jsonb_typeof(p_selection->'payment_mode') IS DISTINCT FROM 'string'
  OR jsonb_typeof(p_selection->'locale') IS DISTINCT FROM 'string'
  OR p_selection->>'locale' NOT IN ('zh-CN','zh-TW','en')
  OR jsonb_typeof(p_selection->'send_payment_link') IS DISTINCT FROM 'boolean' THEN
  RAISE EXCEPTION 'invalid_selection' USING ERRCODE='PT400';
 END IF;
 v_items:=p_selection->'items'; v_delivery:=p_selection->'delivery';
 v_customer:=p_selection->'customer'; v_for:=p_selection->'for';
 IF jsonb_array_length(v_items) NOT BETWEEN 1 AND 50 THEN
  RAISE EXCEPTION 'invalid_selection' USING ERRCODE='PT400';
 END IF;
 FOR v_item IN SELECT value FROM jsonb_array_elements(v_items) LOOP
  IF jsonb_typeof(v_item) IS DISTINCT FROM 'object' THEN
   RAISE EXCEPTION 'invalid_selection' USING ERRCODE='PT400';
  END IF;
  IF EXISTS(SELECT 1 FROM jsonb_object_keys(v_item) k
    WHERE k NOT IN ('sku_id','quantity','ordinal','origin_id','bundle_id','offer_id'))
   OR jsonb_typeof(v_item->'sku_id') IS DISTINCT FROM 'string'
   OR coalesce(v_item->>'sku_id','') !~ v_uuid_pattern
   OR jsonb_typeof(v_item->'quantity') IS DISTINCT FROM 'number'
   OR coalesce(v_item->>'quantity','') !~ '^[0-9]{1,4}$'
   OR jsonb_typeof(v_item->'ordinal') IS DISTINCT FROM 'number'
   OR coalesce(v_item->>'ordinal','') !~ '^[0-9]{1,9}$' THEN
   RAISE EXCEPTION 'invalid_selection' USING ERRCODE='PT400';
  END IF;
  IF (v_item->>'quantity')::integer NOT BETWEEN 1 AND 1000 OR (v_item->>'ordinal')::integer<1
   OR (v_item->>'ordinal')::integer=ANY(v_ordinals) THEN
   RAISE EXCEPTION 'invalid_selection' USING ERRCODE='PT400';
  END IF;
  v_ordinals:=array_append(v_ordinals,(v_item->>'ordinal')::integer);
  FOREACH v_field IN ARRAY ARRAY['origin_id','bundle_id','offer_id'] LOOP
   IF v_item ? v_field AND v_item->v_field<>'null'::jsonb AND
    (jsonb_typeof(v_item->v_field)<>'string' OR coalesce(v_item->>v_field,'') !~ v_uuid_pattern) THEN
    RAISE EXCEPTION 'invalid_selection' USING ERRCODE='PT400';
   END IF;
  END LOOP;
  IF ((v_item->>'bundle_id') IS NULL)<>((v_item->>'offer_id') IS NULL)
   OR (v_item->>'bundle_id' IS NOT NULL AND v_item->>'origin_id' IS NULL) THEN
   RAISE EXCEPTION 'invalid_selection' USING ERRCODE='PT400';
  END IF;
  v_key:=(v_item->>'sku_id')||'|'||coalesce(v_item->>'origin_id','');
  IF v_key=ANY(v_keys) THEN RAISE EXCEPTION 'invalid_selection' USING ERRCODE='PT400'; END IF;
  v_keys:=array_append(v_keys,v_key);
 END LOOP;
 IF EXISTS(SELECT 1 FROM jsonb_object_keys(v_delivery) k
   WHERE k NOT IN ('market_id','country','service_code','home_address','cvs'))
  OR jsonb_typeof(v_delivery->'market_id') IS DISTINCT FROM 'string'
  OR coalesce(v_delivery->>'market_id','') !~ v_uuid_pattern
  OR jsonb_typeof(v_delivery->'country') IS DISTINCT FROM 'string'
  OR coalesce(v_delivery->>'country','') !~ '^[A-Z]{2}$'
  OR jsonb_typeof(v_delivery->'service_code') IS DISTINCT FROM 'string'
  OR coalesce(v_delivery->>'service_code','') !~ '^[a-z][a-z0-9_-]{0,39}$'
  OR ((coalesce(v_delivery->'home_address','null')='null')=(coalesce(v_delivery->'cvs','null')='null')) THEN
  RAISE EXCEPTION 'invalid_selection' USING ERRCODE='PT400';
 END IF;
 v_home:=coalesce(v_delivery->'home_address','null')<>'null';
 v_item:=CASE WHEN v_home THEN v_delivery->'home_address' ELSE v_delivery->'cvs' END;
 IF jsonb_typeof(v_item) IS DISTINCT FROM 'object' THEN
  RAISE EXCEPTION 'invalid_selection' USING ERRCODE='PT400';
 END IF;
 IF EXISTS(SELECT 1 FROM jsonb_object_keys(v_item) k WHERE
  (v_home AND k NOT IN ('region','city','postal_code','line1','line2')) OR
  (NOT v_home AND k NOT IN ('store_code','store_name','store_address'))) THEN
  RAISE EXCEPTION 'invalid_selection' USING ERRCODE='PT400';
 END IF;
 FOREACH v_field IN ARRAY (CASE WHEN v_home THEN ARRAY['city','line1'] ELSE ARRAY['store_code','store_name','store_address'] END) LOOP
  v_value:=v_item->>v_field;
  IF jsonb_typeof(v_item->v_field) IS DISTINCT FROM 'string' OR v_value<>btrim(v_value)
   OR v_value ~ '[[:cntrl:]]' OR char_length(v_value)<(CASE WHEN v_field='store_address' THEN 5 ELSE 1 END)
   OR char_length(v_value)>(CASE v_field WHEN 'line1' THEN 200 WHEN 'city' THEN 100
    WHEN 'store_code' THEN 32 WHEN 'store_name' THEN 40 ELSE 120 END) THEN
   RAISE EXCEPTION 'invalid_selection' USING ERRCODE='PT400';
  END IF;
 END LOOP;
 IF v_home THEN
  FOREACH v_field IN ARRAY ARRAY['region','postal_code','line2'] LOOP
   IF v_item ? v_field THEN
    v_value:=v_item->>v_field;
    IF jsonb_typeof(v_item->v_field) IS DISTINCT FROM 'string' OR v_value<>btrim(v_value)
     OR v_value ~ '[[:cntrl:]]' OR char_length(v_value)>(CASE v_field WHEN 'region' THEN 100 WHEN 'postal_code' THEN 20 ELSE 200 END) THEN
     RAISE EXCEPTION 'invalid_selection' USING ERRCODE='PT400';
    END IF;
   END IF;
  END LOOP;
 END IF;
 IF EXISTS(SELECT 1 FROM jsonb_object_keys(v_customer) k WHERE k NOT IN ('name','phone','email')) THEN
  RAISE EXCEPTION 'invalid_selection' USING ERRCODE='PT400';
 END IF;
 FOREACH v_field IN ARRAY ARRAY['name','phone','email'] LOOP
  v_value:=v_customer->>v_field;
  IF jsonb_typeof(v_customer->v_field) IS DISTINCT FROM 'string' OR v_value<>btrim(v_value)
   OR v_value ~ '[[:cntrl:]]' OR char_length(v_value)>(CASE v_field WHEN 'name' THEN 120 WHEN 'phone' THEN 32 ELSE 254 END) THEN
   RAISE EXCEPTION 'invalid_selection' USING ERRCODE='PT400';
  END IF;
 END LOOP;
 IF v_customer->>'name'='' OR v_customer->>'phone' !~ '^[0-9+() -]{6,32}$'
  OR char_length(regexp_replace(v_customer->>'phone','[^0-9]','','g')) NOT BETWEEN 6 AND 20
  OR (v_customer->>'email'<>'' AND v_customer->>'email' !~ '^[^[:space:]@]+@[^[:space:]@]+$') THEN
  RAISE EXCEPTION 'invalid_selection' USING ERRCODE='PT400';
 END IF;
 IF EXISTS(SELECT 1 FROM jsonb_object_keys(v_for) k WHERE k NOT IN ('bundle_ids','conversation_id','customer_id'))
  OR jsonb_typeof(v_for->'bundle_ids') IS DISTINCT FROM 'array' THEN
  RAISE EXCEPTION 'invalid_selection' USING ERRCODE='PT400';
 END IF;
 IF jsonb_array_length(v_for->'bundle_ids')>5 OR EXISTS(SELECT 1 FROM jsonb_array_elements(v_for->'bundle_ids') x
  WHERE jsonb_typeof(x)<>'string' OR trim(both '"' FROM x::text) !~ v_uuid_pattern) THEN
  RAISE EXCEPTION 'invalid_selection' USING ERRCODE='PT400';
 END IF;
 FOREACH v_field IN ARRAY ARRAY['conversation_id','customer_id'] LOOP
  IF v_for ? v_field AND v_for->v_field<>'null'::jsonb AND
   (jsonb_typeof(v_for->v_field)<>'string' OR coalesce(v_for->>v_field,'') !~ v_uuid_pattern) THEN
   RAISE EXCEPTION 'invalid_selection' USING ERRCODE='PT400';
  END IF;
 END LOOP;
 IF p_selection->>'payment_mode' NOT IN ('bank_transfer','cash_on_delivery','pay_at_pickup') THEN
  RAISE EXCEPTION 'payment_mode_unavailable' USING ERRCODE='PT422';
 END IF;
 BEGIN
  v_tenant:=nullif(current_setting('app.tenant_id',true),'')::uuid;
  v_store:=nullif(current_setting('app.store_id',true),'')::uuid;
  v_principal:=nullif(current_setting('app.principal_id',true),'')::uuid;
 EXCEPTION WHEN invalid_text_representation THEN
  RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403';
 END;
 IF NOT identity.principal_holds(v_tenant,v_store,v_principal,ARRAY['store:read']) THEN
  RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403';
 END IF;
 v_owner:=(v_for->>'customer_id')::uuid;
 v_conversation:=(v_for->>'conversation_id')::uuid;
 IF v_owner IS NOT NULL THEN
  IF NOT identity.principal_holds(v_tenant,v_store,v_principal,ARRAY['customers:read']) THEN
   RAISE EXCEPTION 'forbidden' USING ERRCODE='PT403';
  END IF;
  SELECT o.active INTO v_active FROM buyer.owners o
   WHERE o.tenant_id=v_tenant AND o.store_id=v_store AND o.id=v_owner;
  IF NOT FOUND THEN RAISE EXCEPTION 'not_found' USING ERRCODE='PT404'; END IF;
 END IF;
 IF v_conversation IS NOT NULL AND NOT inbox.order_attempt_customer_link(v_conversation,v_owner) THEN
  RAISE EXCEPTION 'not_found' USING ERRCODE='PT404';
 END IF;
 IF v_owner IS NULL THEN
  RETURN jsonb_build_object('buyer_owner_id',NULL,'subjects','[]'::jsonb);
 END IF;
 IF NOT v_active THEN RAISE EXCEPTION 'ERASED' USING ERRCODE='PT422'; END IF;
 -- Native customers are buyer owners (0078); the same UUID has both erasure
 -- roles. Neither bundles nor social labels/contact values establish this link.
 RETURN jsonb_build_object('buyer_owner_id',v_owner,'subjects',jsonb_build_array(
  jsonb_build_object('kind','buyer_owner','id',v_owner),jsonb_build_object('kind','customer','id',v_owner)));
END $$;
ALTER FUNCTION customers.order_attempt_sources(jsonb) OWNER TO commerce_privacy_writer;
REVOKE ALL ON FUNCTION customers.order_attempt_sources(jsonb) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION customers.order_attempt_sources(jsonb) TO commerce_order_attempt_owner;
COMMENT ON FUNCTION customers.order_attempt_sources(jsonb) IS
 'customers internal C1 coordinator only: validate exact normalized selection, recheck merchant scope and explicit native customer/A14 link, return sorted opaque buyer_owner/customer subjects. No row locks or GUC mutation. Coordinator calls again under complete sorted subject barriers and rejects changed bindings; unlinked contact requires a new private owner.';

CREATE FUNCTION buyer.order_attempt_bind_owner(p_attempt uuid,p_selection jsonb)
RETURNS void LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE v_context jsonb; v_tenant uuid; v_store uuid; v_owner uuid; v_active boolean;
BEGIN
 -- Accessor checks the exact stored candidate/request hash, unfinished C1 and
 -- principal/scope. The private flag is stored provenance, never a call argument.
 v_context:=ops.order_attempt_admission_context(p_attempt,p_selection);
 IF jsonb_typeof(v_context->'private_owner') IS DISTINCT FROM 'boolean' THEN
  RAISE EXCEPTION 'invalid_admission_provenance' USING ERRCODE='PT409';
 END IF;
 v_tenant:=(v_context->>'tenant_id')::uuid;
 v_store:=(v_context->>'store_id')::uuid;
 v_owner:=(v_context->>'buyer_owner_id')::uuid;
 IF v_tenant IS NULL OR v_store IS NULL OR v_owner IS NULL THEN
  RAISE EXCEPTION 'attempt_not_found' USING ERRCODE='PT404';
 END IF;
 IF (v_context->>'private_owner')::boolean THEN
  BEGIN
   INSERT INTO buyer.owners(id,tenant_id,store_id) VALUES(v_owner,v_tenant,v_store);
  EXCEPTION WHEN unique_violation THEN
   -- An existing UUID, even in another scope, must never be adopted as private.
   RAISE EXCEPTION 'owner_conflict' USING ERRCODE='PT409';
  END;
 ELSE
  SELECT o.active INTO v_active FROM buyer.owners o
   WHERE o.tenant_id=v_tenant AND o.store_id=v_store AND o.id=v_owner FOR UPDATE;
  IF NOT FOUND THEN RAISE EXCEPTION 'not_found' USING ERRCODE='PT404'; END IF;
  IF NOT v_active THEN RAISE EXCEPTION 'ERASED' USING ERRCODE='PT422'; END IF;
 END IF;
 -- No GUCs changed: the caller keeps its merchant context on success or error.
END $$;
ALTER FUNCTION buyer.order_attempt_bind_owner(uuid,jsonb) OWNER TO commerce_buyer_writer;
REVOKE ALL ON FUNCTION buyer.order_attempt_bind_owner(uuid,jsonb) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION buyer.order_attempt_bind_owner(uuid,jsonb) TO commerce_order_attempt_owner;
COMMENT ON FUNCTION buyer.order_attempt_bind_owner(uuid,jsonb) IS
 'Internal C1 coordinator only after attempt fence, before claims locks: accessor-bound private_owner=true inserts exactly the new persisted UUID, false locks the existing active exact scoped owner. Missing/nonboolean provenance, body mismatch and UUID collisions fail closed; no contact identity inference or capability issuance.';
