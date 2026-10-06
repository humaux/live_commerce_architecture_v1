-- 0160_ads_unbind.sql — W6-06B (unit w6-06b-ads-unbind): Meta ad account unbind + catalog feed URL entry.
-- Purpose: two merchant definers and nothing else (no table, no GRANT/POLICY delta, §4.4 privilege list untouched):
--   integration.meta_ads_unbind — refuse while any operation on the store's enabled meta_ads bindings of the asset is
--   DISPATCHING/UNKNOWN/ACKNOWLEDGED (external-operation-v1 rules 4-5: never cancel or blind-retry; "ACKNOWLEDGED is not
--   success"); otherwise destroy the sealed BISU token copies (heads first, then versions — the 0108
--   meta_connect_disconnect pattern) and return the binding versions so the Go caller CAS-disables the bindings in the
--   SAME transaction (the frozen 0074 bindings_ads_disable_guard trigger still refuses PT409 binding_in_use while a
--   draft counts = R2-ADS-PAUSE-1 "pause first, then disconnect"; the rollback keeps the credentials).
--   ads.catalog_feed_url — the absolute public catalog feed URL (origin + /feeds/meta.csv, §7 / catalog-inventory-v1
--   "Meta catalog feed") per ACTIVE storefront domain of a published store, for pasting into Meta Commerce Manager.
-- Depends on: 0008 (integration.bindings/operations, commerce_integration_writer SELECT + UPDATE(id) + lock policies,
--   commerce_runtime UPDATE(enabled,semantic_version,updated_at)), 0064 (meta_page_credentials/heads + writer policies),
--   0095 (DELETE on the credential tables), 0074 (ads.auth/ads.deny, GAP-2 domain+publication reads for
--   commerce_ads_writer, frozen bindings_ads_disable_guard), 0113 (USAGE ads + EXECUTE ads.auth to
--   commerce_integration_writer), 0020 (control.storefront_domains/storefront_publications), identity.resolve_access (0153).
-- Used by: internal/ads/unbind.go (Service.Unbind / Service.CatalogFeed), internal/httpapi/ads.go (POST meta/unbind,
--   GET catalog-feed). Tests: internal/ads/unbind_pg_test.go (TestAdsUnbind*), r2_integration_upgrade_test (count 81->82).
-- Invariants: I02 (both routes ride ops.command_results / audit in the Go caller), AD2 (meta_ads bindings), server-side
--   tenant/store auth only (ads.auth = resolve_access + GUC equality; no caller-supplied principal), no plaintext token
--   ever exists or is logged, history is kept (unbind detaches, never deletes: drafts, remote objects, insights, CAPI
--   facts, connections rows, oauth states and audit all survive; READY ops go terminal STALE_BINDING at their next
--   claim, external-operation-v1 rule 5). Both definers: SECURITY DEFINER SET search_path=pg_catalog, REVOKE ALL FROM
--   PUBLIC, EXECUTE commerce_runtime only; refusal codes are the frozen ads ADnnn vocabulary.

DO $$
BEGIN
 IF to_regclass('integration.meta_page_heads') IS NULL OR to_regclass('control.storefront_domains') IS NULL
  OR to_regprocedure('ads.auth(bytea,uuid,text[])') IS NULL
  OR to_regprocedure('integration.claim_operation(uuid,integer,bytea)') IS NULL THEN
  RAISE EXCEPTION '0160 requires 0008/0020/0064/0074 objects';
 END IF;
END $$;

-- ---------------------------------------------------------------------------------------
-- Unbind, definer half: authenticate, lock, in-flight check, token destruction. The detach
-- (enabled=false, semantic_version+1, CAS on the returned version) is the Go caller's UPDATE
-- as commerce_runtime (0008:115) in the same transaction, so the frozen disable guard sees
-- the whole unbind atomically and a PT409 rolls the credential destruction back with it.
-- ---------------------------------------------------------------------------------------
CREATE FUNCTION integration.meta_ads_unbind(p_hash bytea,p_store uuid,p_ad_account text) RETURNS jsonb
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE a record; b record; v_ids uuid[]:='{}'; v_versions bigint[]:='{}'; v_bindings jsonb:='[]'::jsonb;
 v_ops jsonb; v_total bigint; v_destroyed bigint; i int;
BEGIN
 IF p_ad_account IS NULL OR p_ad_account !~ '^[0-9]{1,40}$' THEN
  RAISE EXCEPTION 'invalid_request' USING ERRCODE='AD422';
 END IF;
 -- ads:manage via resolve_access + GUC scope equality (0113 granted EXECUTE to this owner); ADnnn refusals.
 SELECT * INTO a FROM ads.auth(p_hash,p_store,ARRAY['ads:manage']);
 -- Lock EVERY enabled meta_ads binding of this store's asset (normally one; bindOne re-uses enabled
 -- same-asset bindings). claim_operation locks the binding row first, so after this loop no operation
 -- of these bindings can enter DISPATCHING while the unbind transaction is open. Deterministic order.
 FOR b IN SELECT x.id,x.semantic_version FROM integration.bindings x
   WHERE x.tenant_id=a.out_tenant AND x.store_id=p_store AND x.provider='meta_ads'
     AND x.external_asset_id=p_ad_account AND x.enabled
   ORDER BY x.id FOR UPDATE LOOP
  v_ids:=v_ids||b.id; v_versions:=v_versions||b.semantic_version;
 END LOOP;
 IF cardinality(v_ids)=0 THEN
  -- Idempotent no-op: already unbound (or never bound). Not an error, nothing written.
  RETURN jsonb_build_object('unbound',false,'already_unbound',true);
 END IF;
 -- In-flight refusal: DISPATCHING/UNKNOWN/ACKNOWLEDGED operations are never cancelled here
 -- (external-operation-v1 rules 4-5). A RETURN value, not an exception: it carries the list.
 SELECT count(*) INTO v_total FROM integration.operations o
  WHERE o.tenant_id=a.out_tenant AND o.store_id=p_store AND o.binding_id=ANY(v_ids)
    AND o.state IN ('DISPATCHING','UNKNOWN','ACKNOWLEDGED');
 IF v_total>0 THEN
  SELECT jsonb_agg(jsonb_build_object('operation_id',o.id,'action',o.action,'state',o.state))
   INTO v_ops FROM (SELECT o2.id,o2.action,o2.state FROM integration.operations o2
    WHERE o2.tenant_id=a.out_tenant AND o2.store_id=p_store AND o2.binding_id=ANY(v_ids)
      AND o2.state IN ('DISPATCHING','UNKNOWN','ACKNOWLEDGED')
    ORDER BY o2.created_at,o2.id LIMIT 50) o;
  RETURN jsonb_build_object('refused','operations_in_flight',
   'operations',coalesce(v_ops,'[]'::jsonb),'operations_total',v_total);
 END IF;
 -- Token destruction through the existing credential path (0064/0095): heads first, then every
 -- sealed version; no plaintext ever exists and nothing token-shaped is returned or logged.
 FOR i IN 1..cardinality(v_ids) LOOP
  DELETE FROM integration.meta_page_heads h
   WHERE h.tenant_id=a.out_tenant AND h.store_id=p_store AND h.binding_id=v_ids[i];
  DELETE FROM integration.meta_page_credentials k
   WHERE k.tenant_id=a.out_tenant AND k.store_id=p_store AND k.binding_id=v_ids[i];
  GET DIAGNOSTICS v_destroyed=ROW_COUNT;
  v_bindings:=v_bindings||jsonb_build_object('binding_id',v_ids[i],'binding_version',v_versions[i],
   'credentials_destroyed',v_destroyed);
 END LOOP;
 RETURN jsonb_build_object('unbound',true,'bindings',v_bindings);
END $$;

ALTER FUNCTION integration.meta_ads_unbind(bytea,uuid,text) OWNER TO commerce_integration_writer;
REVOKE ALL ON FUNCTION integration.meta_ads_unbind(bytea,uuid,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION integration.meta_ads_unbind(bytea,uuid,text) TO commerce_runtime;

COMMENT ON FUNCTION integration.meta_ads_unbind(bytea,uuid,text) IS
 'integration owner; only caller internal/ads.Service.Unbind under the ads.meta.unbind command receipt (ads:manage via ads.auth + GUC equality). Locks every enabled meta_ads binding of the store''s ad-account asset FOR UPDATE (serializes with claim_operation); returns {refused:operations_in_flight,operations<=50,operations_total} when any is DISPATCHING/UNKNOWN/ACKNOWLEDGED (never cancels), {unbound:false,already_unbound:true} as the idempotent no-op, else destroys the sealed token heads+versions and returns {unbound:true,bindings:[{binding_id,binding_version,credentials_destroyed}]}. The Go caller CAS-disables those bindings in the SAME transaction; the frozen 0074 disable guard may still refuse PT409 binding_in_use (R2-ADS-PAUSE-1) and the rollback keeps the credentials. History is never deleted.';

-- ---------------------------------------------------------------------------------------
-- Catalog feed URL entry: the absolute public feed URL per ACTIVE domain of the published
-- store. Pure read; the feed itself is public and unsigned (§7), so ads:read suffices and
-- no secret is involved. Serving at fetch time stays governed by buyer.resolve_published_store
-- (the 0074 GAP-2 grant has no valid_until column, so the validity window is not re-checked here).
-- ---------------------------------------------------------------------------------------
CREATE FUNCTION ads.catalog_feed_url(p_hash bytea,p_store uuid) RETURNS jsonb
LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE a record; d record; v_domains jsonb:='[]'::jsonb; v_first text;
BEGIN
 SELECT * INTO a FROM ads.auth(p_hash,p_store,ARRAY['ads:read']);
 IF EXISTS(SELECT 1 FROM control.storefront_publications p
   WHERE p.tenant_id=a.out_tenant AND p.store_id=p_store AND p.published) THEN
  FOR d IN SELECT x.origin FROM control.storefront_domains x
    WHERE x.tenant_id=a.out_tenant AND x.store_id=p_store AND x.state='ACTIVE'
    ORDER BY x.origin COLLATE "C" LOOP
   IF v_first IS NULL THEN v_first:=d.origin||'/feeds/meta.csv'; END IF;
   v_domains:=v_domains||jsonb_build_object('origin',d.origin,'feed_url',d.origin||'/feeds/meta.csv');
  END LOOP;
 END IF;
 RETURN jsonb_build_object('feed_url',v_first,'domains',v_domains,'path','/feeds/meta.csv');
END $$;

ALTER FUNCTION ads.catalog_feed_url(bytea,uuid) OWNER TO commerce_ads_writer;
REVOKE ALL ON FUNCTION ads.catalog_feed_url(bytea,uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION ads.catalog_feed_url(bytea,uuid) TO commerce_runtime;

COMMENT ON FUNCTION ads.catalog_feed_url(bytea,uuid) IS
 'ads owner; only caller GET ads/catalog-feed (commerce_runtime, ads:read via ads.auth). The public Meta Commerce Manager feed entry of the store: feed_url = lexicographically first ACTIVE storefront origin (C collation) + /feeds/meta.csv while the store is published, every ACTIVE domain listed, null/[] when unpublished or no ACTIVE domain. No secret, no token, no provider call.';
