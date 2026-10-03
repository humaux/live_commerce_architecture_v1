-- Amendment 2: preserve Meta's public-facing refusal wording; no platform regional policy.
-- Provisional number reserved by coord lock migration:live_commerce_architecture_v1:0112.
-- Only ads owns the new projection; no cross-domain table privilege expansion.
CREATE TABLE ads.operation_refusals (
 tenant_id uuid NOT NULL,
 store_id uuid NOT NULL,
 operation_id uuid PRIMARY KEY,
 generation bigint NOT NULL CHECK (generation > 0),
 code text NOT NULL CHECK (code ~ '^graph_[1-9][0-9]{0,5}$'),
 error_user_msg text NOT NULL CHECK (char_length(error_user_msg) BETWEEN 1 AND 300),
 updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 FOREIGN KEY (tenant_id,store_id,operation_id) REFERENCES integration.operations(tenant_id,store_id,id) ON DELETE CASCADE
);
ALTER TABLE ads.operation_refusals ENABLE ROW LEVEL SECURITY;
ALTER TABLE ads.operation_refusals FORCE ROW LEVEL SECURITY;
REVOKE ALL ON ads.operation_refusals FROM PUBLIC;
GRANT SELECT,INSERT,UPDATE,DELETE ON ads.operation_refusals TO commerce_ads_writer;
CREATE POLICY ads_writer_refusal ON ads.operation_refusals TO commerce_ads_writer USING (true) WITH CHECK (true);
CREATE INDEX operation_refusals_store_recent ON ads.operation_refusals(tenant_id,store_id,updated_at DESC);
COMMENT ON TABLE ads.operation_refusals IS 'Ads-owned latest public Meta refusal per operation. Written only by a lease-fenced completion hook; authenticated projections use explicit tenant/store filters. Deleted with operation or on success; not an immutable event history.';
COMMENT ON COLUMN ads.operation_refusals.tenant_id IS 'Server-derived operation tenant; never supplied by browser.';
COMMENT ON COLUMN ads.operation_refusals.store_id IS 'Server-derived operation store; part of scoped operation foreign key.';
COMMENT ON COLUMN ads.operation_refusals.operation_id IS 'Exactly one latest refusal per integration operation; CASCADE with operation retention.';
COMMENT ON COLUMN ads.operation_refusals.generation IS 'Claim generation matched by the completion transaction; stale claims cannot overwrite.';
COMMENT ON COLUMN ads.operation_refusals.code IS 'Generic graph_<numeric code>; no platform regional policy codes.';
COMMENT ON COLUMN ads.operation_refusals.error_user_msg IS 'Sanitized Meta public wording only, at most 300 Unicode characters, rendered as text; no internal error.message or raw envelope.';
COMMENT ON COLUMN ads.operation_refusals.updated_at IS 'Database time of last refusal completion; recent merchant projection is capped to twenty operations.';

CREATE FUNCTION ads.finish_operation_refusal(p_operation uuid,p_generation bigint,p_token bytea,p_mode text,
 p_state text,p_code text,p_message text) RETURNS void
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE o integration.operations%ROWTYPE;
BEGIN
 IF p_generation IS NULL OR p_generation<1 OR p_token IS NULL OR octet_length(p_token)<>32
  OR p_mode IS NULL OR p_mode NOT IN ('dispatch','reconcile')
  OR p_state IS NULL OR p_state NOT IN ('FAILED_FINAL','UNKNOWN','SUCCEEDED') THEN
  RAISE EXCEPTION 'invalid ads finish' USING ERRCODE='22023'; END IF;
 SELECT * INTO o FROM integration.operations x WHERE x.id=p_operation AND x.provider IN ('meta_ads','meta_dataset');
 IF NOT FOUND THEN RAISE EXCEPTION 'ads operation unavailable' USING ERRCODE='P0002'; END IF;
 IF o.generation<>p_generation OR o.lease_until IS NULL OR o.lease_until<=clock_timestamp()
  OR o.lease_token_hash IS DISTINCT FROM sha256(p_token) OR o.lease_mode<>p_mode
  OR o.state NOT IN ('DISPATCHING','UNKNOWN') THEN
  RAISE EXCEPTION 'ads lease conflict' USING ERRCODE='40001'; END IF;
 -- Service.Complete fences the same lease again in this transaction. If it loses
 -- the race the whole transaction rolls back, including this projection write.
 IF p_state='SUCCEEDED' THEN
  DELETE FROM ads.operation_refusals f WHERE f.tenant_id=o.tenant_id AND f.store_id=o.store_id AND f.operation_id=o.id;
 ELSIF p_message IS NOT NULL AND p_message<>'' AND p_code ~ '^graph_[1-9][0-9]{0,5}$' THEN
  INSERT INTO ads.operation_refusals(tenant_id,store_id,operation_id,generation,code,error_user_msg)
   VALUES(o.tenant_id,o.store_id,o.id,p_generation,p_code,left(p_message,300))
  ON CONFLICT(operation_id) DO UPDATE SET generation=EXCLUDED.generation,code=EXCLUDED.code,
   error_user_msg=EXCLUDED.error_user_msg,updated_at=clock_timestamp()
   WHERE operation_refusals.tenant_id=EXCLUDED.tenant_id AND operation_refusals.store_id=EXCLUDED.store_id
    AND operation_refusals.generation<=EXCLUDED.generation;
 END IF;
END $$;
ALTER FUNCTION ads.finish_operation_refusal(uuid,bigint,bytea,text,text,text,text) OWNER TO commerce_ads_writer;
REVOKE ALL ON FUNCTION ads.finish_operation_refusal(uuid,bigint,bytea,text,text,text,text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION ads.finish_operation_refusal(uuid,bigint,bytea,text,text,text,text) TO commerce_ads_worker;
COMMENT ON FUNCTION ads.finish_operation_refusal(uuid,bigint,bytea,text,text,text,text) IS
 'ads-owned projection. Only the ads dispatcher Finish hook; exact claim generation/token/mode/expiry, same completion transaction. Does not change operation state. Sanitized Meta error_user_msg only.';

CREATE OR REPLACE FUNCTION ads.draft_json(p_tenant uuid,p_store uuid,p_draft uuid) RETURNS jsonb
LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE d ads.campaign_drafts; v_remote jsonb:='{}'::jsonb; v_out jsonb; v_ops jsonb; v_acct text; v_cid text; r record;
BEGIN
 SELECT * INTO d FROM ads.campaign_drafts x WHERE x.tenant_id=p_tenant AND x.store_id=p_store AND x.id=p_draft;
 IF NOT FOUND THEN PERFORM ads.deny('not_found'); END IF;
 FOR r IN SELECT r2.kind,r2.remote_id FROM ads.remote_objects r2 WHERE r2.draft_id=d.id AND r2.publish_attempt=d.publish_attempt
   AND r2.kind IN ('campaign','adset','creative','ad') AND r2.remote_id IS NOT NULL LOOP
  v_remote:=v_remote||jsonb_build_object(r.kind||'_id',r.remote_id);
 END LOOP;
 v_out:=jsonb_build_object('id',d.id,'ad_binding_id',d.ad_binding_id,'identity_binding_id',d.identity_binding_id,
  'template',d.template,'source_ref',d.source_ref,'currency',d.currency,'lifetime_budget_minor',d.lifetime_budget_minor,
  'starts_at',ads.ts(d.starts_at),'ends_at',ads.ts(d.ends_at),'countries',to_jsonb(d.countries),
  'age_min',d.age_min,'age_max',d.age_max,'revision',d.revision,'publish_attempt',d.publish_attempt,
  'status',ads.draft_status(d.id),'created_at',ads.ts(d.created_at),'remote',v_remote);
 IF EXISTS(SELECT 1 FROM ads.draft_approvals a WHERE a.draft_id=d.id AND a.revision=d.revision) THEN
  v_out:=v_out||jsonb_build_object('approved_revision',d.revision); END IF;
 IF d.ended_at IS NOT NULL THEN v_out:=v_out||jsonb_build_object('ended_at',ads.ts(d.ended_at)); END IF;
 SELECT coalesce(jsonb_agg(jsonb_build_object('kind',q.kind,'seq',q.seq,'attempt',q.publish_attempt,'state',q.state,
   'updated_at',ads.ts(q.updated_at),'error_user_msg',q.error_user_msg)||CASE WHEN q.result_code<>'' THEN jsonb_build_object('code',q.result_code) ELSE '{}'::jsonb END
   ORDER BY q.publish_attempt,q.ord,q.seq),'[]'::jsonb) INTO v_ops
  FROM (SELECT r3.kind,r3.seq,r3.publish_attempt,o.state,o.updated_at,o.result_code,f.error_user_msg,
         array_position(ARRAY['campaign','adset','creative','ad','preflight','activate','pause'],r3.kind) AS ord
        FROM ads.remote_objects r3 JOIN integration.operations o ON o.id=r3.operation_id
        LEFT JOIN ads.operation_refusals f ON f.tenant_id=o.tenant_id AND f.store_id=o.store_id AND f.operation_id=o.id
          AND f.generation=o.generation AND f.code=o.result_code AND o.state IN ('FAILED_FINAL','UNKNOWN')
        WHERE r3.tenant_id=p_tenant AND r3.store_id=p_store AND r3.draft_id=d.id ORDER BY r3.publish_attempt DESC,ord DESC,r3.seq DESC LIMIT 60) q;
 v_out:=v_out||jsonb_build_object('ops',v_ops);
 v_cid:=ads.draft_campaign(d.id);
 IF v_cid IS NOT NULL THEN
  SELECT b.external_asset_id INTO v_acct FROM integration.bindings b WHERE b.tenant_id=d.tenant_id AND b.store_id=d.store_id AND b.id=d.ad_binding_id;
  -- Ads Manager deep link (UI convention, not an API contract; Meta may change it): campaign list filtered to the id.
  v_out:=v_out||jsonb_build_object('ads_manager_url','https://adsmanager.facebook.com/adsmanager/manage/campaigns?act='||v_acct||'&selected_campaign_ids='||v_cid);
 END IF;
 RETURN v_out;
END $$;

CREATE OR REPLACE FUNCTION ads.get_settings(p_hash bytea,p_store uuid) RETURNS jsonb
LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $$
DECLARE a record; st ads.store_settings; v_conn jsonb; v_ident jsonb; v_out jsonb;
BEGIN
 SELECT * INTO a FROM ads.auth(p_hash,p_store,ARRAY['ads:read']);
 SELECT * INTO st FROM ads.store_settings s WHERE s.tenant_id=a.out_tenant AND s.store_id=p_store;
 IF NOT FOUND THEN
  st.tenant_id:=a.out_tenant; st.store_id:=p_store; st.environment:='SANDBOX'; st.max_active_budget_minor:=0;
  st.allowance_currency:='TWD'; st.capi_enabled:=false;      -- O4: allowance NT$0 (ads off) until the operator sets it
 END IF;
 SELECT coalesce(jsonb_agg(jsonb_build_object('binding_id',c.binding_id,'provider',b.provider,'asset_id',b.external_asset_id,
   'client_business_id',c.client_business_id,'enabled',b.enabled,'connected_at',ads.ts(c.connected_at)) ORDER BY c.connected_at,c.binding_id),'[]'::jsonb)
  INTO v_conn FROM ads.connections c JOIN integration.bindings b ON b.tenant_id=c.tenant_id AND b.store_id=c.store_id AND b.id=c.binding_id
  WHERE c.tenant_id=a.out_tenant AND c.store_id=p_store;
 SELECT coalesce(jsonb_agg(jsonb_build_object('binding_id',b.id,'provider',b.provider,'asset_id',b.external_asset_id) ORDER BY b.created_at,b.id),'[]'::jsonb)
  INTO v_ident FROM integration.bindings b WHERE b.tenant_id=a.out_tenant AND b.store_id=p_store AND b.enabled
   AND b.provider IN ('facebook','instagram');
 v_out:=jsonb_build_object('environment',st.environment,'allowance_currency',st.allowance_currency,
  'max_active_budget_minor',st.max_active_budget_minor,'capi',ads.capi_json(st),'connections',v_conn,'identities',v_ident);
 IF st.sandbox_ad_account IS NOT NULL THEN v_out:=v_out||jsonb_build_object('sandbox_ad_account',st.sandbox_ad_account); END IF;
 v_out:=v_out||jsonb_build_object('recent_refusals',coalesce((
  SELECT jsonb_agg(jsonb_build_object('operation_id',q.operation_id,'action',q.action,'code',q.code,
    'state',q.state,'error_user_msg',q.error_user_msg,'updated_at',ads.ts(q.updated_at)) ORDER BY q.updated_at DESC,q.operation_id)
  FROM (SELECT f.operation_id,o.action,f.code,o.state,f.error_user_msg,f.updated_at
   FROM ads.operation_refusals f JOIN integration.operations o
    ON o.tenant_id=f.tenant_id AND o.store_id=f.store_id AND o.id=f.operation_id
   WHERE f.tenant_id=a.out_tenant AND f.store_id=p_store AND f.generation=o.generation
    AND f.code=o.result_code AND o.state IN ('FAILED_FINAL','UNKNOWN')
   ORDER BY f.updated_at DESC,f.operation_id LIMIT 20) q),'[]'::jsonb));
 RETURN v_out;
END $$;
